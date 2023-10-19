// Virtual machine: interpret GoAWK compiled opcodes

package interp

import (
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/benhoyt/goawk/internal/compiler"
	"github.com/benhoyt/goawk/internal/resolver"
	"github.com/benhoyt/goawk/lexer"
)

// Execute a block of virtual machine instructions.
//
// A big switch seems to be the best way of doing this for now. I also tried
// an array of functions (https://github.com/benhoyt/goawk/commit/8e04b069b621ff9b9456de57a35ff2fe335cf201)
// and it was ever so slightly faster, but the code was harder to work with
// and it won't be improved when Go gets faster switches via jump tables
// (https://go-review.googlesource.com/c/go/+/357330/).
//
// Additionally, I've made this version faster since the above test by
// reducing the number of opcodes (replacing a couple dozen Call* opcodes with
// a single CallBuiltin -- that probably pushed it below a switch binary tree
// branch threshold).
func (p *interp) execute(code []compiler.Opcode) error {
	for ip := 0; ip < len(code); {
		op := code[ip]
		ip++

		if p.checkCtx {
			err := p.checkContext()
			if err != nil {
				return err
			}
		}

		switch op {
		case compiler.Num:
			index := code[ip]
			ip++
			p.push(num(p.nums[index]))

		case compiler.Str:
			index := code[ip]
			ip++
			p.push(str(p.strs[index]))

		case compiler.Dupe:
			v := p.peekTop()
			p.push(v)

		case compiler.Drop:
			p.pop()

		case compiler.Swap:
			l, r := p.peekTwo()
			p.replaceTwo(r, l)

		case compiler.Rote:
			s := p.peekSlice(3)
			v0, v1, v2 := s[0], s[1], s[2]
			s[0], s[1], s[2] = v1, v2, v0

		case compiler.Field:
			index := p.peekTop()
			v := p.getField(int(index.num()))
			p.replaceTop(v)

		case compiler.FieldInt:
			index := code[ip]
			ip++
			v := p.getField(int(index))
			p.push(v)

		case compiler.FieldByName:
			fieldName := p.peekTop()
			field, err := p.getFieldByName(p.toString(fieldName))
			if err != nil {
				return err
			}
			p.replaceTop(field)

		case compiler.FieldByNameStr:
			index := code[ip]
			fieldName := p.strs[index]
			ip++
			field, err := p.getFieldByName(fieldName)
			if err != nil {
				return err
			}
			p.push(field)

		case compiler.Global:
			index := code[ip]
			ip++
			p.push(p.globals[index])

		case compiler.Local:
			index := code[ip]
			ip++
			p.push(p.frame[index])

		case compiler.Special:
			index := code[ip]
			ip++
			p.push(p.getSpecial(int(index)))

		case compiler.ArrayGlobal:
			arrayIndex := code[ip]
			ip++
			array := p.arrays[arrayIndex]
			index := p.toString(p.peekTop())
			v := arrayGet(array, index)
			p.replaceTop(v)

		case compiler.ArrayLocal:
			arrayIndex := code[ip]
			ip++
			array := p.localArray(int(arrayIndex))
			index := p.toString(p.peekTop())
			v := arrayGet(array, index)
			p.replaceTop(v)

		case compiler.InGlobal:
			arrayIndex := code[ip]
			ip++
			array := p.arrays[arrayIndex]
			index := p.toString(p.peekTop())
			_, ok := array[index]
			p.replaceTop(boolean(ok))

		case compiler.InLocal:
			arrayIndex := code[ip]
			ip++
			array := p.localArray(int(arrayIndex))
			index := p.toString(p.peekTop())
			_, ok := array[index]
			p.replaceTop(boolean(ok))

		case compiler.AssignField:
			right, index := p.popTwo()
			err := p.setField(int(index.num()), p.toString(right))
			if err != nil {
				return err
			}

		case compiler.AssignGlobal:
			index := code[ip]
			ip++
			p.globals[index] = p.pop()

		case compiler.AssignLocal:
			index := code[ip]
			ip++
			p.frame[index] = p.pop()

		case compiler.AssignSpecial:
			index := code[ip]
			ip++
			err := p.setSpecial(int(index), p.pop())
			if err != nil {
				return err
			}

		case compiler.AssignArrayGlobal:
			arrayIndex := code[ip]
			ip++
			array := p.arrays[arrayIndex]
			v, index := p.popTwo()
			array[p.toString(index)] = v

		case compiler.AssignArrayLocal:
			arrayIndex := code[ip]
			ip++
			array := p.localArray(int(arrayIndex))
			v, index := p.popTwo()
			array[p.toString(index)] = v

		case compiler.Delete:
			arrayScope := code[ip]
			arrayIndex := code[ip+1]
			ip += 2
			array := p.array(resolver.Scope(arrayScope), int(arrayIndex))
			index := p.toString(p.pop())
			delete(array, index)

		case compiler.DeleteAll:
			arrayScope := code[ip]
			arrayIndex := code[ip+1]
			ip += 2
			array := p.array(resolver.Scope(arrayScope), int(arrayIndex))
			for k := range array {
				delete(array, k)
			}

		case compiler.IncrField:
			amount := code[ip]
			ip++
			index := int(p.pop().num())
			v := p.getField(index)
			err := p.setField(index, p.toString(num(v.num()+float64(amount))))
			if err != nil {
				return err
			}

		case compiler.IncrGlobal:
			amount := code[ip]
			index := code[ip+1]
			ip += 2
			p.globals[index] = num(p.globals[index].num() + float64(amount))

		case compiler.IncrLocal:
			amount := code[ip]
			index := code[ip+1]
			ip += 2
			p.frame[index] = num(p.frame[index].num() + float64(amount))

		case compiler.IncrSpecial:
			amount := code[ip]
			index := int(code[ip+1])
			ip += 2
			v := p.getSpecial(index)
			err := p.setSpecial(index, num(v.num()+float64(amount)))
			if err != nil {
				return err
			}

		case compiler.IncrArrayGlobal:
			amount := code[ip]
			arrayIndex := code[ip+1]
			ip += 2
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			array[index] = num(array[index].num() + float64(amount))

		case compiler.IncrArrayLocal:
			amount := code[ip]
			arrayIndex := code[ip+1]
			ip += 2
			array := p.localArray(int(arrayIndex))
			index := p.toString(p.pop())
			array[index] = num(array[index].num() + float64(amount))

		case compiler.AugAssignField:
			operation := compiler.AugOp(code[ip])
			ip++
			right, indexVal := p.popTwo()
			index := int(indexVal.num())
			field := p.getField(index)
			v, err := p.augAssignOp(operation, field, right)
			if err != nil {
				return err
			}
			err = p.setField(index, p.toString(v))
			if err != nil {
				return err
			}

		case compiler.AugAssignGlobal:
			operation := compiler.AugOp(code[ip])
			index := code[ip+1]
			ip += 2
			v, err := p.augAssignOp(operation, p.globals[index], p.pop())
			if err != nil {
				return err
			}
			p.globals[index] = v

		case compiler.AugAssignLocal:
			operation := compiler.AugOp(code[ip])
			index := code[ip+1]
			ip += 2
			v, err := p.augAssignOp(operation, p.frame[index], p.pop())
			if err != nil {
				return err
			}
			p.frame[index] = v

		case compiler.AugAssignSpecial:
			operation := compiler.AugOp(code[ip])
			index := int(code[ip+1])
			ip += 2
			v, err := p.augAssignOp(operation, p.getSpecial(index), p.pop())
			if err != nil {
				return err
			}
			err = p.setSpecial(index, v)
			if err != nil {
				return err
			}

		case compiler.AugAssignArrayGlobal:
			operation := compiler.AugOp(code[ip])
			arrayIndex := code[ip+1]
			ip += 2
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			v, err := p.augAssignOp(operation, array[index], p.pop())
			if err != nil {
				return err
			}
			array[index] = v

		case compiler.AugAssignArrayLocal:
			operation := compiler.AugOp(code[ip])
			arrayIndex := code[ip+1]
			ip += 2
			array := p.localArray(int(arrayIndex))
			right, indexVal := p.popTwo()
			index := p.toString(indexVal)
			v, err := p.augAssignOp(operation, array[index], right)
			if err != nil {
				return err
			}
			array[index] = v

		case compiler.Regex:
			// Stand-alone /regex/ is equivalent to: $0 ~ /regex/
			index := code[ip]
			ip++
			re := p.regexes[index]
			p.push(boolean(re.MatchString(p.line)))

		case compiler.IndexMulti:
			numValues := int(code[ip])
			ip++
			values := p.popSlice(numValues)
			indices := make([]string, 0, 3) // up to 3-dimensional indices won't require heap allocation
			for _, v := range values {
				indices = append(indices, p.toString(v))
			}
			p.push(str(strings.Join(indices, p.subscriptSep)))

		case compiler.Add:
			l, r := p.peekPop()
			p.replaceTop(num(l.num() + r.num()))

		case compiler.Subtract:
			l, r := p.peekPop()
			p.replaceTop(num(l.num() - r.num()))

		case compiler.Multiply:
			l, r := p.peekPop()
			p.replaceTop(num(l.num() * r.num()))

		case compiler.Divide:
			l, r := p.peekPop()
			rf := r.num()
			if rf == 0.0 {
				return newError("division by zero")
			}
			p.replaceTop(num(l.num() / rf))

		case compiler.Power:
			l, r := p.peekPop()
			p.replaceTop(num(math.Pow(l.num(), r.num())))

		case compiler.Modulo:
			l, r := p.peekPop()
			rf := r.num()
			if rf == 0.0 {
				return newError("division by zero in mod")
			}
			p.replaceTop(num(math.Mod(l.num(), rf)))

		case compiler.Equals:
			l, r := p.peekPop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.replaceTop(boolean(p.toString(l) == p.toString(r)))
			} else {
				p.replaceTop(boolean(ln == rn))
			}

		case compiler.NotEquals:
			l, r := p.peekPop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.replaceTop(boolean(p.toString(l) != p.toString(r)))
			} else {
				p.replaceTop(boolean(ln != rn))
			}

		case compiler.Less:
			l, r := p.peekPop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.replaceTop(boolean(p.toString(l) < p.toString(r)))
			} else {
				p.replaceTop(boolean(ln < rn))
			}

		case compiler.Greater:
			l, r := p.peekPop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.replaceTop(boolean(p.toString(l) > p.toString(r)))
			} else {
				p.replaceTop(boolean(ln > rn))
			}

		case compiler.LessOrEqual:
			l, r := p.peekPop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.replaceTop(boolean(p.toString(l) <= p.toString(r)))
			} else {
				p.replaceTop(boolean(ln <= rn))
			}

		case compiler.GreaterOrEqual:
			l, r := p.peekPop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.replaceTop(boolean(p.toString(l) >= p.toString(r)))
			} else {
				p.replaceTop(boolean(ln >= rn))
			}

		case compiler.Concat:
			l, r := p.peekPop()
			p.replaceTop(str(p.toString(l) + p.toString(r)))

		case compiler.ConcatMulti:
			numValues := int(code[ip])
			ip++
			values := p.popSlice(numValues)
			var sb strings.Builder

			for _, v := range values {
				sb.WriteString(p.toString(v))
			}
			p.push(str(sb.String()))

		case compiler.Match:
			l, r := p.peekPop()
			re, err := p.compileRegex(p.toString(r))
			if err != nil {
				return err
			}
			matched := re.MatchString(p.toString(l))
			p.replaceTop(boolean(matched))

		case compiler.NotMatch:
			l, r := p.peekPop()
			re, err := p.compileRegex(p.toString(r))
			if err != nil {
				return err
			}
			matched := re.MatchString(p.toString(l))
			p.replaceTop(boolean(!matched))

		case compiler.Not:
			p.replaceTop(boolean(!p.peekTop().boolean()))

		case compiler.UnaryMinus:
			p.replaceTop(num(-p.peekTop().num()))

		case compiler.UnaryPlus:
			p.replaceTop(num(p.peekTop().num()))

		case compiler.Boolean:
			p.replaceTop(boolean(p.peekTop().boolean()))

		case compiler.Jump:
			offset := code[ip]
			ip += 1 + int(offset)

		case compiler.JumpFalse:
			offset := code[ip]
			ip++
			v := p.pop()
			if !v.boolean() {
				ip += int(offset)
			}

		case compiler.JumpTrue:
			offset := code[ip]
			ip++
			v := p.pop()
			if v.boolean() {
				ip += int(offset)
			}

		case compiler.JumpEquals:
			offset := code[ip]
			ip++
			l, r := p.popTwo()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) == p.toString(r)
			} else {
				b = ln == rn
			}
			if b {
				ip += int(offset)
			}

		case compiler.JumpNotEquals:
			offset := code[ip]
			ip++
			l, r := p.popTwo()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) != p.toString(r)
			} else {
				b = ln != rn
			}
			if b {
				ip += int(offset)
			}

		case compiler.JumpLess:
			offset := code[ip]
			ip++
			l, r := p.popTwo()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) < p.toString(r)
			} else {
				b = ln < rn
			}
			if b {
				ip += int(offset)
			}

		case compiler.JumpGreater:
			offset := code[ip]
			ip++
			l, r := p.popTwo()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) > p.toString(r)
			} else {
				b = ln > rn
			}
			if b {
				ip += int(offset)
			}

		case compiler.JumpLessOrEqual:
			offset := code[ip]
			ip++
			l, r := p.popTwo()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) <= p.toString(r)
			} else {
				b = ln <= rn
			}
			if b {
				ip += int(offset)
			}

		case compiler.JumpGreaterOrEqual:
			offset := code[ip]
			ip++
			l, r := p.popTwo()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) >= p.toString(r)
			} else {
				b = ln >= rn
			}
			if b {
				ip += int(offset)
			}

		case compiler.Next:
			return errNext

		case compiler.Nextfile:
			return errNextfile

		case compiler.Exit:
			// Return special errExit value "caught" by top-level executor
			return errExit

		case compiler.ExitStatus:
			p.exitStatus = int(p.pop().num())
			return errExit

		case compiler.ForIn:
			varScope := code[ip]
			varIndex := code[ip+1]
			arrayScope := code[ip+2]
			arrayIndex := code[ip+3]
			offset := code[ip+4]
			ip += 5
			array := p.array(resolver.Scope(arrayScope), int(arrayIndex))
			loopCode := code[ip : ip+int(offset)]
			for index := range array {
				switch resolver.Scope(varScope) {
				case resolver.Global:
					p.globals[varIndex] = str(index)
				case resolver.Local:
					p.frame[varIndex] = str(index)
				default: // resolver.Special
					err := p.setSpecial(int(varIndex), str(index))
					if err != nil {
						return err
					}
				}
				err := p.execute(loopCode)
				if err == errBreak {
					break
				}
				if err != nil {
					return err
				}
			}
			ip += int(offset)

		case compiler.BreakForIn:
			return errBreak

		case compiler.CallBuiltin:
			builtinOp := compiler.BuiltinOp(code[ip])
			ip++
			err := p.callBuiltin(builtinOp)
			if err != nil {
				return err
			}

		case compiler.CallLengthArray:
			arrayScope := code[ip]
			arrayIndex := code[ip+1]
			ip += 2
			array := p.array(resolver.Scope(arrayScope), int(arrayIndex))
			p.push(num(float64(len(array))))

		case compiler.CallSplit:
			arrayScope := code[ip]
			arrayIndex := code[ip+1]
			ip += 2
			s := p.toString(p.peekTop())
			n, err := p.split(s, resolver.Scope(arrayScope), int(arrayIndex), p.fieldSep, p.inputMode)
			if err != nil {
				return err
			}
			p.replaceTop(num(float64(n)))

		case compiler.CallSplitSep:
			arrayScope := code[ip]
			arrayIndex := code[ip+1]
			ip += 2
			s, fieldSep := p.peekPop()
			// 3-argument form of split() ignores input mode
			n, err := p.split(p.toString(s), resolver.Scope(arrayScope), int(arrayIndex), p.toString(fieldSep), DefaultMode)
			if err != nil {
				return err
			}
			p.replaceTop(num(float64(n)))

		case compiler.CallSprintf:
			numArgs := code[ip]
			ip++
			args := p.popSlice(int(numArgs))
			s, err := p.sprintf(p.toString(args[0]), args[1:])
			if err != nil {
				return err
			}
			p.push(str(s))

		case compiler.CallUser:
			funcIndex := code[ip]
			numArrayArgs := int(code[ip+1])
			ip += 2

			f := p.program.Compiled.Functions[funcIndex]
			if p.callDepth >= maxCallDepth {
				return newError("calling %q exceeded maximum call depth of %d", f.Name, maxCallDepth)
			}

			// Set up frame for scalar arguments
			oldFrame := p.frame
			p.frame = p.peekSlice(f.NumScalars)

			// Handle array arguments
			var arrays []int
			for j := 0; j < numArrayArgs; j++ {
				arrayScope := resolver.Scope(code[ip])
				arrayIndex := int(code[ip+1])
				ip += 2
				arrays = append(arrays, p.arrayIndex(arrayScope, arrayIndex))
			}
			oldArraysLen := len(p.arrays)
			for j := numArrayArgs; j < f.NumArrays; j++ {
				arrays = append(arrays, len(p.arrays))
				p.arrays = append(p.arrays, make(map[string]value))
			}
			p.localArrays = append(p.localArrays, arrays)

			// Execute the function!
			p.callDepth++
			err := p.execute(f.Body)
			p.callDepth--

			// Pop the locals off the stack
			p.popSlice(f.NumScalars)
			p.frame = oldFrame
			p.localArrays = p.localArrays[:len(p.localArrays)-1]
			p.arrays = p.arrays[:oldArraysLen]

			if r, ok := err.(returnValue); ok {
				p.push(r.Value)
			} else if err != nil {
				return err
			} else {
				p.push(null())
			}

		case compiler.CallNative:
			funcIndex := int(code[ip])
			numArgs := int(code[ip+1])
			ip += 2

			args := p.popSlice(numArgs)
			r, err := p.callNative(funcIndex, args)
			if err != nil {
				return err
			}
			p.push(r)

		case compiler.Return:
			v := p.pop()
			return returnValue{v}

		case compiler.ReturnNull:
			return returnValue{null()}

		case compiler.Nulls:
			numNulls := int(code[ip])
			ip++
			p.pushNulls(numNulls)

		case compiler.Print:
			numArgs := code[ip]
			redirect := lexer.Token(code[ip+1])
			ip += 2

			args := p.popSlice(int(numArgs))

			// Determine what output stream to write to.
			output := p.output
			if redirect != lexer.ILLEGAL {
				var err error
				dest := p.pop()
				output, err = p.getOutputStream(redirect, dest)
				if err != nil {
					return err
				}
			}

			if numArgs > 0 {
				err := p.printArgs(output, args)
				if err != nil {
					return err
				}
			} else {
				// "print" with no arguments prints the raw value of $0,
				// regardless of output mode.
				err := p.printLine(output, p.line)
				if err != nil {
					return err
				}
			}

		case compiler.Printf:
			numArgs := code[ip]
			redirect := lexer.Token(code[ip+1])
			ip += 2

			args := p.popSlice(int(numArgs))
			s, err := p.sprintf(p.toString(args[0]), args[1:])
			if err != nil {
				return err
			}

			output := p.output
			if redirect != lexer.ILLEGAL {
				dest := p.pop()
				output, err = p.getOutputStream(redirect, dest)
				if err != nil {
					return err
				}
			}
			err = writeOutput(output, s)
			if err != nil {
				return err
			}

		case compiler.Getline:
			redirect := lexer.Token(code[ip])
			ip++

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				p.setLine(line, false)
			}
			p.push(num(ret))

		case compiler.GetlineField:
			redirect := lexer.Token(code[ip])
			ip++

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				err := p.setField(0, line)
				if err != nil {
					return err
				}
			}
			p.push(num(ret))

		case compiler.GetlineGlobal:
			redirect := lexer.Token(code[ip])
			index := code[ip+1]
			ip += 2

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				p.globals[index] = numStr(line)
			}
			p.push(num(ret))

		case compiler.GetlineLocal:
			redirect := lexer.Token(code[ip])
			index := code[ip+1]
			ip += 2

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				p.frame[index] = numStr(line)
			}
			p.push(num(ret))

		case compiler.GetlineSpecial:
			redirect := lexer.Token(code[ip])
			index := code[ip+1]
			ip += 2

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				err := p.setSpecial(int(index), numStr(line))
				if err != nil {
					return err
				}
			}
			p.push(num(ret))

		case compiler.GetlineArray:
			redirect := lexer.Token(code[ip])
			arrayScope := code[ip+1]
			arrayIndex := code[ip+2]
			ip += 3

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			index := p.toString(p.peekTop())
			if ret == 1 {
				array := p.array(resolver.Scope(arrayScope), int(arrayIndex))
				array[index] = numStr(line)
			}
			p.replaceTop(num(ret))
		}
	}

	return nil
}

func (p *interp) callBuiltin(builtinOp compiler.BuiltinOp) error {
	switch builtinOp {
	case compiler.BuiltinAtan2:
		y, x := p.peekPop()
		p.replaceTop(num(math.Atan2(y.num(), x.num())))

	case compiler.BuiltinClose:
		var err error
		code := -1
		name := p.toString(p.peekTop())
		if stream := p.inputStreams[name]; stream != nil {
			// Close input stream
			delete(p.inputStreams, name)
			delete(p.scanners, name)
			err = stream.Close()
			code = stream.ExitCode()
		} else if stream := p.outputStreams[name]; stream != nil {
			// Close output stream
			delete(p.outputStreams, name)
			err = stream.Close()
			code = stream.ExitCode()
		}
		if err != nil {
			p.printErrorf("error closing %q: %v\n", name, err)
		}
		p.replaceTop(num(float64(code)))

	case compiler.BuiltinCos:
		p.replaceTop(num(math.Cos(p.peekTop().num())))

	case compiler.BuiltinExp:
		p.replaceTop(num(math.Exp(p.peekTop().num())))

	case compiler.BuiltinFflush:
		name := p.toString(p.peekTop())
		var ok bool
		if name != "" {
			// Flush a single, named output stream
			ok = p.flushStream(name)
		} else {
			// fflush() or fflush("") flushes all output streams
			ok = p.flushAll()
		}
		if !ok {
			p.replaceTop(num(-1))
		} else {
			p.replaceTop(num(0))
		}

	case compiler.BuiltinFflushAll:
		ok := p.flushAll()
		if !ok {
			p.push(num(-1))
		} else {
			p.push(num(0))
		}

	case compiler.BuiltinGsub:
		regex, repl, in := p.peekPeekPop()
		out, n, err := p.sub(p.toString(regex), p.toString(repl), p.toString(in), true)
		if err != nil {
			return err
		}
		p.replaceTwo(num(float64(n)), str(out))

	case compiler.BuiltinIndex:
		sValue, substr := p.peekPop()
		s := p.toString(sValue)
		index := strings.Index(s, p.toString(substr))
		p.replaceTop(num(float64(index + 1)))

	case compiler.BuiltinInt:
		p.replaceTop(num(float64(int(p.peekTop().num()))))

	case compiler.BuiltinLength:
		p.push(num(float64(len(p.line))))

	case compiler.BuiltinLengthArg:
		s := p.toString(p.peekTop())
		p.replaceTop(num(float64(len(s))))

	case compiler.BuiltinLog:
		p.replaceTop(num(math.Log(p.peekTop().num())))

	case compiler.BuiltinMatch:
		sValue, regex := p.peekPop()
		s := p.toString(sValue)
		re, err := p.compileRegex(p.toString(regex))
		if err != nil {
			return err
		}
		loc := re.FindStringIndex(s)
		if loc == nil {
			p.matchStart = 0
			p.matchLength = -1
			p.replaceTop(num(0))
		} else {
			p.matchStart = loc[0] + 1
			p.matchLength = loc[1] - loc[0]
			p.replaceTop(num(float64(p.matchStart)))
		}

	case compiler.BuiltinRand:
		p.push(num(p.random.Float64()))

	case compiler.BuiltinSin:
		p.replaceTop(num(math.Sin(p.peekTop().num())))

	case compiler.BuiltinSqrt:
		p.replaceTop(num(math.Sqrt(p.peekTop().num())))

	case compiler.BuiltinSrand:
		prevSeed := p.randSeed
		p.random.Seed(time.Now().UnixNano())
		p.push(num(prevSeed))

	case compiler.BuiltinSrandSeed:
		prevSeed := p.randSeed
		p.randSeed = p.peekTop().num()
		p.random.Seed(int64(math.Float64bits(p.randSeed)))
		p.replaceTop(num(prevSeed))

	case compiler.BuiltinSub:
		regex, repl, in := p.peekPeekPop()
		out, n, err := p.sub(p.toString(regex), p.toString(repl), p.toString(in), false)
		if err != nil {
			return err
		}
		p.replaceTwo(num(float64(n)), str(out))

	case compiler.BuiltinSubstr:
		sValue, posValue := p.peekPop()
		pos := int(posValue.num())
		s := p.toString(sValue)
		if pos > len(s) {
			pos = len(s) + 1
		}
		if pos < 1 {
			pos = 1
		}
		length := len(s) - pos + 1
		p.replaceTop(str(s[pos-1 : pos-1+length]))

	case compiler.BuiltinSubstrLength:
		posValue, lengthValue := p.popTwo()
		length := int(lengthValue.num())
		pos := int(posValue.num())
		s := p.toString(p.peekTop())
		if pos > len(s) {
			pos = len(s) + 1
		}
		if pos < 1 {
			pos = 1
		}
		maxLength := len(s) - pos + 1
		if length < 0 {
			length = 0
		}
		if length > maxLength {
			length = maxLength
		}
		p.replaceTop(str(s[pos-1 : pos-1+length]))

	case compiler.BuiltinSystem:
		if p.noExec {
			return newError("can't call system() due to NoExec")
		}
		cmdline := p.toString(p.peekTop())
		cmd := p.execShell(cmdline)
		cmd.Stdin = p.stdin
		cmd.Stdout = p.output
		cmd.Stderr = p.errorOutput
		_ = p.flushAll() // ensure synchronization
		err := cmd.Start()
		if err != nil {
			// Could not start the shell so skip waiting on it.
			p.printErrorf("%v\n", err)
			p.replaceTop(num(-1.0))
			return nil
		}
		exitCode, err := waitExitCode(cmd)
		if err != nil {
			if p.checkCtx && p.ctx.Err() != nil {
				return p.ctx.Err()
			}
			p.printErrorf("%v\n", err)
		}
		p.replaceTop(num(float64(exitCode)))

	case compiler.BuiltinTolower:
		p.replaceTop(str(strings.ToLower(p.toString(p.peekTop()))))

	case compiler.BuiltinToupper:
		p.replaceTop(str(strings.ToUpper(p.toString(p.peekTop()))))
	}

	return nil
}

// Fetch the value at the given index from array. This handles the strange
// POSIX behavior of creating a null entry for non-existent array elements.
// Per the POSIX spec, "Any other reference to a nonexistent array element
// [apart from "in" expressions] shall automatically create it."
func arrayGet(array map[string]value, index string) value {
	v, ok := array[index]
	if !ok {
		array[index] = v
	}
	return v
}

// Stack operations follow. These should be inlined. Instead of just push and
// pop, for efficiency we have custom operations for when we're replacing the
// top of stack without changing the stack pointer. Primarily this avoids the
// check for append in push.
func (p *interp) push(v value) {
	sp := p.sp
	if sp >= len(p.stack) {
		p.stack = append(p.stack, null())
	}
	p.stack[sp] = v
	sp++
	p.sp = sp
}

func (p *interp) pushNulls(num int) {
	sp := p.sp
	for p.sp+num-1 >= len(p.stack) {
		p.stack = append(p.stack, null())
	}
	for i := 0; i < num; i++ {
		p.stack[sp] = null()
		sp++
	}
	p.sp = sp
}

func (p *interp) pop() value {
	p.sp--
	return p.stack[p.sp]
}

func (p *interp) popTwo() (value, value) {
	p.sp -= 2
	return p.stack[p.sp], p.stack[p.sp+1]
}

func (p *interp) peekTop() value {
	return p.stack[p.sp-1]
}

func (p *interp) peekTwo() (value, value) {
	return p.stack[p.sp-2], p.stack[p.sp-1]
}

func (p *interp) peekPop() (value, value) {
	p.sp--
	return p.stack[p.sp-1], p.stack[p.sp]
}

func (p *interp) peekPeekPop() (value, value, value) {
	p.sp--
	return p.stack[p.sp-2], p.stack[p.sp-1], p.stack[p.sp]
}

func (p *interp) replaceTop(v value) {
	p.stack[p.sp-1] = v
}

func (p *interp) replaceTwo(l, r value) {
	p.stack[p.sp-2] = l
	p.stack[p.sp-1] = r
}

func (p *interp) popSlice(n int) []value {
	p.sp -= n
	return p.stack[p.sp : p.sp+n]
}

func (p *interp) peekSlice(n int) []value {
	return p.stack[p.sp-n:]
}

// Helper for getline operations. This performs the (possibly redirected) read
// of a line, and returns the result. If the result is 1 (success in AWK), the
// caller will set the target to the returned string.
func (p *interp) getline(redirect lexer.Token) (float64, string, error) {
	switch redirect {
	case lexer.PIPE: // redirect from command
		name := p.toString(p.pop())
		scanner, err := p.getInputScannerPipe(name)
		if err != nil {
			return 0, "", err
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return -1, "", nil
			}
			return 0, "", nil
		}
		return 1, scanner.Text(), nil

	case lexer.LESS: // redirect from file
		name := p.toString(p.pop())
		scanner, err := p.getInputScannerFile(name)
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				// File not found is not a hard error, getline just returns -1.
				// See: https://github.com/benhoyt/goawk/issues/41
				return -1, "", nil
			}
			return 0, "", err
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return -1, "", nil
			}
			return 0, "", nil
		}
		return 1, scanner.Text(), nil

	default: // no redirect
		p.flushOutputAndError() // Flush output in case they've written a prompt
		var err error
		line, err := p.nextLine()
		if err == io.EOF {
			return 0, "", nil
		}
		if err != nil {
			return -1, "", nil
		}
		return 1, line, nil
	}
}

// Perform augmented assignment operation.
func (p *interp) augAssignOp(op compiler.AugOp, l, r value) (value, error) {
	switch op {
	case compiler.AugOpAdd:
		return num(l.num() + r.num()), nil
	case compiler.AugOpSub:
		return num(l.num() - r.num()), nil
	case compiler.AugOpMul:
		return num(l.num() * r.num()), nil
	case compiler.AugOpDiv:
		rf := r.num()
		if rf == 0.0 {
			return null(), newError("division by zero")
		}
		return num(l.num() / rf), nil
	case compiler.AugOpPow:
		return num(math.Pow(l.num(), r.num())), nil
	default: // AugOpMod
		rf := r.num()
		if rf == 0.0 {
			return null(), newError("division by zero in mod")
		}
		return num(math.Mod(l.num(), rf)), nil
	}
}
