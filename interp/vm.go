package interp

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/compiler"
	"github.com/benhoyt/goawk/lexer"
)

func (p *interp) execute(compiled *compiler.Program, code []compiler.Opcode) error {
	for i := 0; i < len(code); {
		op := code[i]
		//fmt.Printf("TODO %04x %s %v\n", i, op, p.vmStack[:p.vmSp])
		i++

		switch op {
		case compiler.Num:
			index := code[i]
			i++
			p.push(num(compiled.Nums[index]))

		case compiler.Str:
			index := code[i]
			i++
			p.push(str(compiled.Strs[index]))

		case compiler.Dupe:
			v := p.pop()
			p.push(v)
			p.push(v)

		case compiler.Drop:
			p.pop()

		case compiler.Field:
			index := p.pop()
			v, err := p.getField(int(index.num()))
			if err != nil {
				return err
			}
			p.push(v)

		case compiler.FieldNum:
			index := code[i]
			i++
			v, err := p.getField(int(index))
			if err != nil {
				return err
			}
			p.push(v)

		case compiler.Global:
			index := code[i]
			i++
			p.push(p.globals[index])

		case compiler.Local:
			index := code[i]
			i++
			p.push(p.frame[index])

		case compiler.Special:
			index := code[i]
			i++
			p.push(p.getVar(ast.ScopeSpecial, int(index))) // TODO: extract getVar to getSpecial function

		case compiler.ArrayGlobal:
			arrayIndex := code[i]
			i++
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			v, ok := array[index]
			if !ok {
				// Strangely, per the POSIX spec, "Any other reference to a
				// nonexistent array element [apart from "in" expressions]
				// shall automatically create it."
				array[index] = v
			}
			p.push(v)

		case compiler.ArrayLocal:
			arrayIndex := code[i]
			i++
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			index := p.toString(p.pop())
			v, ok := array[index]
			if !ok {
				// Strangely, per the POSIX spec, "Any other reference to a
				// nonexistent array element [apart from "in" expressions]
				// shall automatically create it."
				array[index] = v
			}
			p.push(v)

		case compiler.InGlobal:
			arrayIndex := code[i]
			i++
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			_, ok := array[index]
			p.push(boolean(ok))

		case compiler.InLocal:
			arrayIndex := code[i]
			i++
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			index := p.toString(p.pop())
			_, ok := array[index]
			p.push(boolean(ok))

		case compiler.AssignField:
			index := p.pop()
			right := p.pop()
			err := p.setField(int(index.num()), p.toString(right))
			if err != nil {
				return err
			}

		case compiler.AssignGlobal:
			index := code[i]
			i++
			p.globals[index] = p.pop()

		case compiler.AssignLocal:
			index := code[i]
			i++
			p.frame[index] = p.pop()

		case compiler.AssignSpecial:
			index := code[i]
			i++
			err := p.setVar(ast.ScopeSpecial, int(index), p.pop()) // TODO: extract setVar to setSpecial function
			if err != nil {
				return err
			}

		case compiler.AssignArrayGlobal:
			arrayIndex := code[i]
			i++
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			array[index] = p.pop()

		case compiler.AssignArrayLocal:
			arrayIndex := code[i]
			i++
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			index := p.toString(p.pop())
			array[index] = p.pop()

		case compiler.DeleteGlobal:
			arrayIndex := code[i]
			i++
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			delete(array, index)

		case compiler.DeleteLocal:
			arrayIndex := code[i]
			i++
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			index := p.toString(p.pop())
			delete(array, index)

		case compiler.DeleteAllGlobal:
			arrayIndex := code[i]
			i++
			array := p.arrays[arrayIndex]
			for k := range array {
				delete(array, k)
			}

		case compiler.DeleteAllLocal:
			arrayIndex := code[i]
			i++
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			for k := range array {
				delete(array, k)
			}

		case compiler.IncrField:
			amount := int32(code[i])
			i++
			index := int(p.pop().num())
			v, err := p.getField(index)
			if err != nil {
				return err
			}
			err = p.setField(index, p.toString(num(v.num()+float64(amount))))
			if err != nil {
				return err
			}

		case compiler.IncrGlobal:
			amount := int32(code[i])
			index := code[i+1]
			i += 2
			p.globals[index] = num(p.globals[index].num() + float64(amount))

		case compiler.IncrLocal:
			amount := int32(code[i])
			index := code[i+1]
			i += 2
			p.frame[index] = num(p.frame[index].num() + float64(amount))

		case compiler.IncrSpecial:
			amount := int32(code[i])
			index := int(code[i+1])
			i += 2
			v := p.getVar(ast.ScopeSpecial, index)
			err := p.setVar(ast.ScopeSpecial, index, num(v.num()+float64(amount)))
			if err != nil {
				return err
			}

		case compiler.IncrArrayGlobal:
			amount := int32(code[i])
			arrayIndex := code[i+1]
			i += 2
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			array[index] = num(array[index].num() + float64(amount))

		case compiler.IncrArrayLocal:
			amount := int32(code[i])
			arrayIndex := code[i+1]
			i += 2
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			index := p.toString(p.pop())
			array[index] = num(array[index].num() + float64(amount))

		case compiler.AugAssignField:
			operation := lexer.Token(code[i])
			i++
			index := int(p.pop().num())
			field, err := p.getField(index)
			if err != nil {
				return err
			}
			v, err := p.evalBinary(operation, field, p.pop())
			if err != nil {
				return err
			}
			err = p.setField(index, p.toString(v))
			if err != nil {
				return err
			}

		case compiler.AugAssignGlobal:
			operation := lexer.Token(code[i])
			index := code[i+1]
			i += 2
			v, err := p.evalBinary(operation, p.globals[index], p.pop())
			if err != nil {
				return err
			}
			p.globals[index] = v

		case compiler.AugAssignLocal:
			operation := lexer.Token(code[i])
			index := code[i+1]
			i += 2
			v, err := p.evalBinary(operation, p.frame[index], p.pop())
			if err != nil {
				return err
			}
			p.frame[index] = v

		case compiler.AugAssignSpecial:
			operation := lexer.Token(code[i])
			index := int(code[i+1])
			i += 2
			v, err := p.evalBinary(operation, p.getVar(ast.ScopeSpecial, index), p.pop())
			if err != nil {
				return err
			}
			err = p.setVar(ast.ScopeSpecial, index, v)
			if err != nil {
				return err
			}

		case compiler.AugAssignArrayGlobal:
			operation := lexer.Token(code[i])
			arrayIndex := code[i+1]
			i += 2
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			v, err := p.evalBinary(operation, array[index], p.pop())
			if err != nil {
				return err
			}
			array[index] = v

		case compiler.AugAssignArrayLocal:
			operation := lexer.Token(code[i])
			arrayIndex := code[i+1]
			i += 2
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			index := p.toString(p.pop())
			v, err := p.evalBinary(operation, array[index], p.pop())
			if err != nil {
				return err
			}
			array[index] = v

		case compiler.Regex:
			// Stand-alone /regex/ is equivalent to: $0 ~ /regex/
			index := code[i]
			i++
			re := compiled.Regexes[index]
			p.push(boolean(re.MatchString(p.line)))

		case compiler.MultiIndex:
			numValues := int(code[i])
			i++
			values := p.popSlice(numValues)
			indices := make([]string, 0, 3) // up to 3-dimensional indices won't require heap allocation
			for _, v := range values {
				indices = append(indices, p.toString(v))
			}
			p.push(str(strings.Join(indices, p.subscriptSep)))

		case compiler.Add:
			r := p.pop()
			l := p.pop()
			p.push(num(l.num() + r.num()))

		case compiler.Subtract:
			r := p.pop()
			l := p.pop()
			p.push(num(l.num() - r.num()))

		case compiler.Multiply:
			r := p.pop()
			l := p.pop()
			p.push(num(l.num() * r.num()))

		case compiler.Divide:
			r := p.pop()
			l := p.pop()
			rf := r.num()
			if rf == 0.0 {
				return newError("division by zero")
			}
			p.push(num(l.num() / rf))

		case compiler.Power:
			r := p.pop()
			l := p.pop()
			p.push(num(math.Pow(l.num(), r.num())))

		case compiler.Modulo:
			r := p.pop()
			l := p.pop()
			rf := r.num()
			if rf == 0.0 {
				return newError("division by zero in mod")
			}
			p.push(num(math.Mod(l.num(), rf)))

		case compiler.Equals:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.push(boolean(p.toString(l) == p.toString(r)))
			} else {
				p.push(boolean(ln == rn))
			}

		case compiler.NotEquals:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.push(boolean(p.toString(l) != p.toString(r)))
			} else {
				p.push(boolean(ln != rn))
			}

		case compiler.Less:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var v value
			if lIsStr || rIsStr {
				v = boolean(p.toString(l) < p.toString(r))
			} else {
				v = boolean(ln < rn)
			}
			p.push(v)

		case compiler.Greater:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var v value
			if lIsStr || rIsStr {
				v = boolean(p.toString(l) > p.toString(r))
			} else {
				v = boolean(ln > rn)
			}
			p.push(v)

		case compiler.LessOrEqual:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var v value
			if lIsStr || rIsStr {
				v = boolean(p.toString(l) <= p.toString(r))
			} else {
				v = boolean(ln <= rn)
			}
			p.push(v)

		case compiler.GreaterOrEqual:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var v value
			if lIsStr || rIsStr {
				v = boolean(p.toString(l) >= p.toString(r))
			} else {
				v = boolean(ln >= rn)
			}
			p.push(v)

		case compiler.Concat:
			r := p.pop()
			l := p.pop()
			p.push(str(p.toString(l) + p.toString(r)))

		case compiler.Match:
			r := p.pop()
			l := p.pop()
			re, err := p.compileRegex(p.toString(r))
			if err != nil {
				return err
			}
			matched := re.MatchString(p.toString(l))
			p.push(boolean(matched))

		case compiler.NotMatch:
			r := p.pop()
			l := p.pop()
			re, err := p.compileRegex(p.toString(r))
			if err != nil {
				return err
			}
			matched := re.MatchString(p.toString(l))
			p.push(boolean(!matched))

		case compiler.Not:
			p.push(boolean(!p.pop().boolean()))

		case compiler.UnaryMinus:
			p.push(num(-p.pop().num()))

		case compiler.UnaryPlus:
			p.push(num(p.pop().num()))

		case compiler.Boolean:
			p.push(boolean(p.pop().boolean()))

		case compiler.Jump:
			offset := int32(code[i])
			i += 1 + int(offset)

		case compiler.JumpFalse:
			offset := int32(code[i])
			v := p.pop()
			if !v.boolean() {
				i += 1 + int(offset)
			} else {
				i++
			}

		case compiler.JumpTrue:
			offset := int32(code[i])
			v := p.pop()
			if v.boolean() {
				i += 1 + int(offset)
			} else {
				i++
			}

		case compiler.JumpEquals:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) == p.toString(r)
			} else {
				b = ln == rn
			}
			if b {
				i += 1 + int(offset)
			} else {
				i++
			}

		case compiler.JumpNotEquals:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) != p.toString(r)
			} else {
				b = ln != rn
			}
			if b {
				i += 1 + int(offset)
			} else {
				i++
			}

		case compiler.JumpLess:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) < p.toString(r)
			} else {
				b = ln < rn
			}
			if b {
				i += 1 + int(offset)
			} else {
				i++
			}

		case compiler.JumpGreater:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) > p.toString(r)
			} else {
				b = ln > rn
			}
			if b {
				i += 1 + int(offset)
			} else {
				i++
			}

		case compiler.JumpLessOrEqual:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) <= p.toString(r)
			} else {
				b = ln <= rn
			}
			if b {
				i += 1 + int(offset)
			} else {
				i++
			}

		case compiler.JumpGreaterOrEqual:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			var b bool
			if lIsStr || rIsStr {
				b = p.toString(l) >= p.toString(r)
			} else {
				b = ln >= rn
			}
			if b {
				i += 1 + int(offset)
			} else {
				i++
			}

		case compiler.Next:
			return errNext

		case compiler.Exit:
			p.exitStatus = int(p.pop().num())
			// Return special errExit value "caught" by top-level executor
			return errExit

		case compiler.ForInGlobal:
			// TODO: can we reduce the copy-pasta here and below?
			varIndex := code[i]
			arrayScope := code[i+1]
			arrayIndex := code[i+2]
			offset := code[i+3]
			i += 4
			array := p.arrays[p.getArrayIndex(ast.VarScope(arrayScope), int(arrayIndex))]
			loopCode := code[i : i+int(offset)]
			for index := range array {
				p.globals[varIndex] = str(index)
				err := p.execute(compiled, loopCode)
				if err == errBreak {
					break
				}
				if err != nil {
					return err
				}
			}
			i += int(offset)

		case compiler.ForInLocal:
			varIndex := code[i]
			arrayScope := code[i+1]
			arrayIndex := code[i+2]
			offset := code[i+3]
			i += 4
			array := p.arrays[p.getArrayIndex(ast.VarScope(arrayScope), int(arrayIndex))]
			loopCode := code[i : i+int(offset)]
			for index := range array {
				p.frame[varIndex] = str(index)
				err := p.execute(compiled, loopCode)
				if err == errBreak {
					break
				}
				if err != nil {
					return err
				}
			}
			i += int(offset)

		case compiler.ForInSpecial:
			varIndex := code[i]
			arrayScope := code[i+1]
			arrayIndex := code[i+2]
			offset := code[i+3]
			i += 4
			array := p.arrays[p.getArrayIndex(ast.VarScope(arrayScope), int(arrayIndex))]
			loopCode := code[i : i+int(offset)]
			for index := range array {
				err := p.setVar(ast.ScopeSpecial, int(varIndex), str(index))
				if err != nil {
					return err
				}
				p.frame[varIndex] = str(index)
				err = p.execute(compiled, loopCode)
				if err == errBreak {
					break
				}
				if err != nil {
					return err
				}
			}
			i += int(offset)

		case compiler.BreakForIn:
			return errBreak

		case compiler.CallUser:
			funcIndex := code[i]
			numArrayArgs := int(code[i+1])
			i += 2

			f := p.program.Compiled.Functions[funcIndex]
			if p.callDepth >= maxCallDepth {
				return newError("calling %q exceeded maximum call depth of %d", f.Name, maxCallDepth)
			}

			// Set up frame for scalar arguments
			oldFrame := p.frame
			p.frame = p.vmStack[p.vmSp-f.NumScalars:] // TODO: replace with stackSlice() call or similar

			// Handle array arguments
			var arrays []int
			for j := 0; j < numArrayArgs; j++ {
				arrayScope := ast.VarScope(code[i])
				arrayIndex := int(code[i+1])
				i += 2
				arrays = append(arrays, p.getArrayIndex(arrayScope, arrayIndex))
			}
			oldArraysLen := len(p.arrays)
			for j := numArrayArgs; j < f.NumArrays; j++ {
				arrays = append(arrays, len(p.arrays))
				p.arrays = append(p.arrays, make(map[string]value))
			}
			p.localArrays = append(p.localArrays, arrays)

			// Execute the function!
			p.callDepth++
			err := p.execute(compiled, f.Body)
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
			funcIndex := int(code[i])
			numArgs := int(code[i+1])
			i += 2

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
			numNulls := int(code[i])
			i++
			for n := 0; n < numNulls; n++ {
				p.push(null())
			}

		case compiler.CallAtan2:
			// TODO: optimize stack operations for all of these (and binary ops) if it improves performance
			x := p.pop()
			y := p.pop()
			p.push(num(math.Atan2(y.num(), x.num())))

		case compiler.CallClose:
			name := p.toString(p.pop())
			var c io.Closer = p.inputStreams[name]
			if c != nil {
				// Close input stream
				delete(p.inputStreams, name)
				err := c.Close()
				if err != nil {
					p.push(num(-1))
				} else {
					p.push(num(0))
				}
			} else {
				c = p.outputStreams[name]
				if c != nil {
					// Close output stream
					delete(p.outputStreams, name)
					err := c.Close()
					if err != nil {
						p.push(num(-1))
					} else {
						p.push(num(0))
					}
				} else {
					// Nothing to close
					p.push(num(-1))
				}
			}

		case compiler.CallCos:
			p.push(num(math.Cos(p.pop().num())))

		case compiler.CallExp:
			p.push(num(math.Exp(p.pop().num())))

		case compiler.CallFflush:
			name := p.toString(p.pop())
			var ok bool
			if name != "" {
				// Flush a single, named output stream
				ok = p.flushStream(name)
			} else {
				// fflush() or fflush("") flushes all output streams
				ok = p.flushAll()
			}
			if !ok {
				p.push(num(-1))
			} else {
				p.push(num(0))
			}

		case compiler.CallFflushAll:
			ok := p.flushAll()
			if !ok {
				p.push(num(-1))
			} else {
				p.push(num(0))
			}

		case compiler.CallGsub:
			in := p.toString(p.pop())
			repl := p.toString(p.pop())
			regex := p.toString(p.pop())
			out, n, err := p.sub(regex, repl, in, true)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))
			p.push(str(out))

		case compiler.CallIndex:
			substr := p.toString(p.pop())
			s := p.toString(p.pop())
			index := strings.Index(s, substr)
			if p.bytes {
				p.push(num(float64(index + 1)))
			} else {
				if index < 0 {
					p.push(num(float64(0)))
				} else {
					index = utf8.RuneCountInString(s[:index])
					p.push(num(float64(index + 1)))
				}
			}

		case compiler.CallInt:
			p.push(num(float64(int(p.pop().num()))))

		case compiler.CallLength:
			s := p.line
			var n int
			if p.bytes {
				n = len(s)
			} else {
				n = utf8.RuneCountInString(s)
			}
			p.push(num(float64(n)))

		case compiler.CallLengthArg:
			s := p.toString(p.pop())
			var n int
			if p.bytes {
				n = len(s)
			} else {
				n = utf8.RuneCountInString(s)
			}
			p.push(num(float64(n)))

		case compiler.CallLog:
			p.push(num(math.Log(p.pop().num())))

		case compiler.CallMatch:
			regex := p.toString(p.pop())
			s := p.toString(p.pop())
			re, err := p.compileRegex(regex)
			if err != nil {
				return err
			}
			loc := re.FindStringIndex(s)
			if loc == nil {
				p.matchStart = 0
				p.matchLength = -1
				p.push(num(0))
			} else {
				if p.bytes {
					p.matchStart = loc[0] + 1
					p.matchLength = loc[1] - loc[0]
				} else {
					p.matchStart = utf8.RuneCountInString(s[:loc[0]]) + 1
					p.matchLength = utf8.RuneCountInString(s[loc[0]:loc[1]])
				}
				p.push(num(float64(p.matchStart)))
			}

		case compiler.CallRand:
			p.push(num(p.random.Float64()))

		case compiler.CallSin:
			p.push(num(math.Sin(p.pop().num())))

		case compiler.CallSplitGlobal:
			arrayIndex := code[i]
			i++
			s := p.toString(p.pop())
			n, err := p.split(s, ast.ScopeGlobal, int(arrayIndex), p.fieldSep)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))

		case compiler.CallSplitLocal:
			arrayIndex := code[i]
			i++
			s := p.toString(p.pop())
			n, err := p.split(s, ast.ScopeLocal, int(arrayIndex), p.fieldSep)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))

		case compiler.CallSplitSepGlobal:
			arrayIndex := code[i]
			i++
			fieldSep := p.toString(p.pop())
			s := p.toString(p.pop())
			n, err := p.split(s, ast.ScopeGlobal, int(arrayIndex), fieldSep)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))

		case compiler.CallSplitSepLocal:
			arrayIndex := code[i]
			i++
			fieldSep := p.toString(p.pop())
			s := p.toString(p.pop())
			n, err := p.split(s, ast.ScopeLocal, int(arrayIndex), fieldSep)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))

		case compiler.CallSprintf:
			numArgs := code[i]
			i++
			args := p.popSlice(int(numArgs))
			s, err := p.sprintf(p.toString(args[0]), args[1:])
			if err != nil {
				return err
			}
			p.push(str(s))

		case compiler.CallSqrt:
			p.push(num(math.Sqrt(p.pop().num())))

		case compiler.CallSrand:
			prevSeed := p.randSeed
			p.random.Seed(time.Now().UnixNano())
			p.push(num(prevSeed))

		case compiler.CallSrandSeed:
			prevSeed := p.randSeed
			p.randSeed = p.pop().num()
			p.random.Seed(int64(math.Float64bits(p.randSeed)))
			p.push(num(prevSeed))

		case compiler.CallSub:
			in := p.toString(p.pop())
			repl := p.toString(p.pop())
			regex := p.toString(p.pop())
			out, n, err := p.sub(regex, repl, in, false)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))
			p.push(str(out))

		case compiler.CallSubstr:
			// TODO: avoid duplication in function.go if we're keeping that
			pos := int(p.pop().num())
			s := p.toString(p.pop())
			if p.bytes {
				if pos > len(s) {
					pos = len(s) + 1
				}
				if pos < 1 {
					pos = 1
				}
				length := len(s) - pos + 1
				p.push(str(s[pos-1 : pos-1+length]))
			} else {
				// Count characters till we get to pos.
				chars := 1
				start := 0
				for start = range s {
					chars++
					if chars > pos {
						break
					}
				}
				if pos >= chars {
					start = len(s)
				}

				// Count characters from start till we reach length.
				end := len(s)
				p.push(str(s[start:end]))
			}

		case compiler.CallSubstrLength:
			// TODO: avoid duplication in function.go if we're keeping that
			length := int(p.pop().num())
			pos := int(p.pop().num())
			s := p.toString(p.pop())
			if p.bytes {
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
				p.push(str(s[pos-1 : pos-1+length]))
			} else {
				// Count characters till we get to pos.
				chars := 1
				start := 0
				for start = range s {
					chars++
					if chars > pos {
						break
					}
				}
				if pos >= chars {
					start = len(s)
				}

				// Count characters from start till we reach length.
				var end int
				chars = 0
				for end = range s[start:] {
					chars++
					if chars > length {
						break
					}
				}
				if length >= chars {
					end = len(s)
				} else {
					end += start
				}
				p.push(str(s[start:end]))
			}

		case compiler.CallSystem:
			if p.noExec {
				return newError("can't call system() due to NoExec")
			}
			cmdline := p.toString(p.pop())
			cmd := p.execShell(cmdline)
			cmd.Stdout = p.output
			cmd.Stderr = p.errorOutput
			_ = p.flushAll() // ensure synchronization
			err := cmd.Start()
			var ret float64
			if err != nil {
				p.printErrorf("%s\n", err)
				ret = -1
			} else {
				err = cmd.Wait()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						ret = float64(exitErr.ProcessState.ExitCode())
					} else {
						p.printErrorf("unexpected error running command %q: %v\n", cmdline, err)
						ret = -1
					}
				} else {
					ret = 0
				}
			}
			p.push(num(ret))

		case compiler.CallTolower:
			p.push(str(strings.ToLower(p.toString(p.pop()))))

		case compiler.CallToupper:
			p.push(str(strings.ToUpper(p.toString(p.pop()))))

		case compiler.Print:
			numArgs := code[i]
			redirect := lexer.Token(code[i+1])
			i += 2

			// Print OFS-separated args followed by ORS (usually newline)
			var line string
			if numArgs > 0 {
				args := p.popSlice(int(numArgs))
				strs := make([]string, len(args))
				for i, a := range args {
					strs[i] = a.str(p.outputFormat)
				}
				line = strings.Join(strs, p.outputFieldSep)
			} else {
				// "print" with no args is equivalent to "print $0"
				line = p.line
			}

			output := p.output
			if redirect != lexer.ILLEGAL {
				var err error
				dest := p.pop()
				output, err = p.getOutputStream(redirect, dest)
				if err != nil {
					return err
				}
			}
			err := p.printLine(output, line)
			if err != nil {
				return err
			}

		case compiler.Printf:
			numArgs := code[i]
			redirect := lexer.Token(code[i+1])
			i += 2

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
			redirect := lexer.Token(code[i])
			i++

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				p.setLine(line, false)
			}
			p.push(num(ret))

		case compiler.GetlineField:
			redirect := lexer.Token(code[i])
			i++

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
			redirect := lexer.Token(code[i])
			index := code[i+1]
			i += 2

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				p.globals[index] = numStr(line)
			}
			p.push(num(ret))

		case compiler.GetlineLocal:
			redirect := lexer.Token(code[i])
			index := code[i+1]
			i += 2

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				p.frame[index] = numStr(line)
			}
			p.push(num(ret))

		case compiler.GetlineSpecial:
			redirect := lexer.Token(code[i])
			index := code[i+1]
			i += 2

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				err := p.setVar(ast.ScopeSpecial, int(index), numStr(line))
				if err != nil {
					return err
				}
			}
			p.push(num(ret))

		case compiler.GetlineArrayGlobal:
			redirect := lexer.Token(code[i])
			arrayIndex := code[i+1]
			i += 2

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				array := p.arrays[arrayIndex]
				index := p.toString(p.pop())
				array[index] = numStr(line)
			}
			p.push(num(ret))

		case compiler.GetlineArrayLocal:
			redirect := lexer.Token(code[i])
			arrayIndex := code[i+1]
			i += 2

			ret, line, err := p.getline(redirect)
			if err != nil {
				return err
			}
			if ret == 1 {
				array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
				index := p.toString(p.pop())
				array[index] = numStr(line)
			}
			p.push(num(ret))

		default:
			panic(fmt.Sprintf("TODO remove: unsupported opcode %s", op))
		}
	}
	return nil
}

// TODO: maybe add popTwo() for binary ops and pushTwo() if it helps? or are these all inlined anyway?

func (p *interp) push(v value) {
	if p.vmSp >= len(p.vmStack) {
		p.vmStack = append(p.vmStack, null())
	}
	p.vmStack[p.vmSp] = v
	p.vmSp++
}

func (p *interp) pop() value {
	p.vmSp--
	return p.vmStack[p.vmSp]
}

func (p *interp) popSlice(n int) []value {
	p.vmSp -= n
	return p.vmStack[p.vmSp : p.vmSp+n]
}

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
