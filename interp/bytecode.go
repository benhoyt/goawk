package interp

import (
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/bytecode"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

// TODO: rename "bytecode" to "wordcode" or some such

// ExecBytecode... TODO
func ExecBytecode(program *parser.Program, config *Config, byteProg *bytecode.Program) (int, error) {
	p, err := execInit(program, config)
	if err != nil {
		return 0, err
	}
	defer p.closeAll()

	// Execute the program! BEGIN, then pattern/actions, then END
	err = p.execBytecode(byteProg, byteProg.Begin)
	if err != nil && err != errExit {
		return 0, err
	}
	if program.Actions == nil && program.End == nil {
		return p.exitStatus, nil
	}
	if err != errExit {
		err = p.execBytecodeActions(byteProg, byteProg.Actions)
		if err != nil && err != errExit {
			return 0, err
		}
	}
	err = p.execBytecode(byteProg, byteProg.End)
	if err != nil && err != errExit {
		return 0, err
	}
	return p.exitStatus, nil
}

func (p *interp) execBytecode(byteProg *bytecode.Program, code []bytecode.Op) error {
	for i := 0; i < len(code); {
		op := code[i]
		i++

		switch op {
		case bytecode.Nop:

		case bytecode.Num:
			index := code[i]
			i++
			p.push(num(byteProg.Nums[index]))

		case bytecode.Str:
			index := code[i]
			i++
			p.push(str(byteProg.Strs[index]))

		case bytecode.Dupe:
			p.push(p.st[len(p.st)-1])

		case bytecode.Drop:
			p.pop()

		case bytecode.Field:
			index := p.pop()
			v, err := p.getField(int(index.num()))
			if err != nil {
				return err
			}
			p.push(v)

		case bytecode.Global:
			index := code[i]
			i++
			p.push(p.globals[index])

		case bytecode.Special:
			index := code[i]
			i++
			p.push(p.getVar(ast.ScopeSpecial, int(index))) // TODO: extract getVar to getSpecial function

		case bytecode.ArrayGlobal:
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

		case bytecode.AssignField:
			index := p.pop()
			right := p.pop()
			err := p.setField(int(index.num()), p.toString(right))
			if err != nil {
				return err
			}

		case bytecode.AssignGlobal:
			index := code[i]
			i++
			p.globals[index] = p.pop()

		case bytecode.AssignLocal:
			index := code[i]
			i++
			p.frame[index] = p.pop()

		case bytecode.AssignSpecial:
			index := code[i]
			i++
			err := p.setVar(ast.ScopeSpecial, int(index), p.pop()) // TODO: extract setVar to setSpecial function
			if err != nil {
				return err
			}

		case bytecode.AssignArrayGlobal:
			arrayIndex := code[i]
			i++
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			array[index] = p.pop()

		case bytecode.AssignArrayLocal:
			arrayIndex := code[i]
			i++
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			index := p.toString(p.pop())
			array[index] = p.pop()

		case bytecode.PostIncrGlobal:
			index := code[i]
			i++
			p.globals[index] = num(p.globals[index].num() + 1)

		case bytecode.PostIncrArrayGlobal:
			arrayIndex := code[i]
			i++
			array := p.arrays[arrayIndex]
			index := p.toString(p.pop())
			array[index] = num(array[index].num() + 1)

		case bytecode.PostIncrArrayLocal:
			arrayIndex := code[i]
			i++
			array := p.arrays[p.localArrays[len(p.localArrays)-1][arrayIndex]]
			index := p.toString(p.pop())
			array[index] = num(array[index].num() + 1)

		case bytecode.AugAssignGlobal:
			operation := lexer.Token(code[i])
			index := code[i+1]
			i += 2
			switch operation {
			case lexer.ADD:
				p.globals[index] = num(p.globals[index].num() + p.pop().num())
			default:
				panic(fmt.Sprintf("unexpected operation %s", operation))
			}

		case bytecode.Regex:
			// Stand-alone /regex/ is equivalent to: $0 ~ /regex/
			index := code[i]
			i++
			re := byteProg.Regexes[index]
			p.push(boolean(re.MatchString(p.line)))

		case bytecode.Add:
			r := p.pop()
			l := p.pop()
			p.push(num(l.num() + r.num()))

		case bytecode.Subtract:
			r := p.pop()
			l := p.pop()
			p.push(num(l.num() - r.num()))

		case bytecode.Multiply:
			r := p.pop()
			l := p.pop()
			p.push(num(l.num() * r.num()))

		case bytecode.Divide:
			r := p.pop()
			l := p.pop()
			rf := r.num()
			if rf == 0.0 {
				return newError("division by zero")
			}
			p.push(num(l.num() / rf))

		case bytecode.Power:
			r := p.pop()
			l := p.pop()
			p.push(num(math.Pow(l.num(), r.num())))

		case bytecode.Modulo:
			r := p.pop()
			l := p.pop()
			rf := r.num()
			if rf == 0.0 {
				return newError("division by zero in mod")
			}
			p.push(num(math.Mod(l.num(), rf)))

		case bytecode.Equals:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.push(boolean(p.toString(l) == p.toString(r)))
			} else {
				p.push(boolean(ln == rn))
			}

		case bytecode.NotEquals:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.push(boolean(p.toString(l) != p.toString(r)))
			} else {
				p.push(boolean(ln != rn))
			}

		case bytecode.Less:
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

		case bytecode.Greater:
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

		case bytecode.LessOrEqual:
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

		case bytecode.GreaterOrEqual:
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

		case bytecode.Concat:
			r := p.pop()
			l := p.pop()
			p.push(str(p.toString(l) + p.toString(r)))

		case bytecode.Match:
			r := p.pop()
			l := p.pop()
			re, err := p.compileRegex(p.toString(r))
			if err != nil {
				return err
			}
			matched := re.MatchString(p.toString(l))
			p.push(boolean(matched))

		case bytecode.NotMatch:
			r := p.pop()
			l := p.pop()
			re, err := p.compileRegex(p.toString(r))
			if err != nil {
				return err
			}
			matched := re.MatchString(p.toString(l))
			p.push(boolean(!matched))

		case bytecode.Not:
			p.push(boolean(!p.pop().boolean()))

		case bytecode.UnaryMinus:
			p.push(num(-p.pop().num()))

		case bytecode.UnaryPlus:
			p.push(num(p.pop().num()))

		case bytecode.Jump:
			offset := int32(code[i])
			i += 1 + int(offset)

		case bytecode.JumpFalse:
			offset := int32(code[i])
			v := p.pop()
			if !v.boolean() {
				i += 1 + int(offset)
			} else {
				i++
			}

		case bytecode.JumpTrue:
			offset := int32(code[i])
			v := p.pop()
			if v.boolean() {
				i += 1 + int(offset)
			} else {
				i++
			}

		case bytecode.JumpNumLess:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			if l.num() < r.num() {
				i += 1 + int(offset)
			} else {
				i++
			}

		case bytecode.JumpNumGreater:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			if l.num() > r.num() {
				i += 1 + int(offset)
			} else {
				i++
			}

		case bytecode.JumpNumLessOrEqual:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			if l.num() <= r.num() {
				i += 1 + int(offset)
			} else {
				i++
			}

		case bytecode.JumpNumGreaterOrEqual:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			if l.num() >= r.num() {
				i += 1 + int(offset)
			} else {
				i++
			}

		case bytecode.Next:
			return errNext

		case bytecode.ForGlobalInGlobal:
			varIndex := code[i]
			arrayIndex := code[i+1]
			offset := code[i+2]
			i += 3
			array := p.arrays[arrayIndex]
			loopCode := code[i : i+int(offset)]
			for index := range array {
				p.globals[varIndex] = str(index)
				err := p.execBytecode(byteProg, loopCode)
				if err == errBreak {
					break
				}
				// TODO: handle continue with jump to end of loopCode block?
				if err != nil {
					return err
				}
			}
			i += int(offset)

		case bytecode.BreakForIn:
			return errBreak

		case bytecode.Print:
			numArgs := code[i]
			redirect := lexer.Token(code[i+1])
			i += 2

			// Print OFS-separated args followed by ORS (usually newline)
			var line string
			if numArgs > 0 {
				sp := len(p.st) - int(numArgs)
				args := p.st[sp:]
				strs := make([]string, len(args))
				for i, a := range args {
					strs[i] = a.str(p.outputFormat)
				}
				p.st = p.st[:sp]
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

		case bytecode.Printf:
			numArgs := code[i]
			redirect := lexer.Token(code[i+1])
			i += 2

			sp := len(p.st) - int(numArgs)
			s, err := p.sprintf(p.toString(p.st[sp]), p.st[sp+1:])
			p.st = p.st[:sp]
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

		case bytecode.CallAtan2:
			// TODO: optimize stack operations for all of these if it improves performance
			y := p.pop()
			x := p.pop()
			p.push(num(math.Atan2(y.num(), x.num())))

		case bytecode.CallClose:
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

		case bytecode.CallCos:
			p.push(num(math.Cos(p.pop().num())))

		case bytecode.CallExp:
			p.push(num(math.Exp(p.pop().num())))

		case bytecode.CallFflush:
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

		case bytecode.CallFflushAll:
			ok := p.flushAll()
			if !ok {
				p.push(num(-1))
			} else {
				p.push(num(0))
			}

		//case bytecode.CallGsub:
		//case bytecode.CallGsubField:
		//case bytecode.CallGsubGlobal:
		//case bytecode.CallGsubLocal:
		//case bytecode.CallGsubSpecial:
		//case bytecode.CallGsubArrayGlobal:
		//case bytecode.CallGsubArrayLocal:

		case bytecode.CallIndex:
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

		case bytecode.CallInt:
			p.push(num(float64(int(p.pop().num()))))

		case bytecode.CallLength:
			s := p.line
			var n int
			if p.bytes {
				n = len(s)
			} else {
				n = utf8.RuneCountInString(s)
			}
			p.push(num(float64(n)))

		case bytecode.CallLengthArg:
			s := p.toString(p.pop())
			var n int
			if p.bytes {
				n = len(s)
			} else {
				n = utf8.RuneCountInString(s)
			}
			p.push(num(float64(n)))

		case bytecode.CallLog:
			p.push(num(math.Log(p.pop().num())))

		case bytecode.CallMatch:
			s := p.toString(p.pop())
			regex := p.toString(p.pop())
			// TODO: could optimize literal regexes to avoid map lookup? but probably not worth it
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

		case bytecode.CallRand:
			p.push(num(p.random.Float64()))

		case bytecode.CallSin:
			p.push(num(math.Sin(p.pop().num())))

		case bytecode.CallSplitGlobal:
			arrayIndex := code[i]
			i++
			s := p.toString(p.pop())
			n, err := p.split(s, ast.ScopeGlobal, int(arrayIndex), p.fieldSep)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))

		case bytecode.CallSplitLocal:
			arrayIndex := code[i]
			i++
			s := p.toString(p.pop())
			n, err := p.split(s, ast.ScopeLocal, int(arrayIndex), p.fieldSep)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))

		case bytecode.CallSplitSepGlobal:
			arrayIndex := code[i]
			i++
			fieldSep := p.toString(p.pop())
			s := p.toString(p.pop())
			n, err := p.split(s, ast.ScopeGlobal, int(arrayIndex), fieldSep)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))

		case bytecode.CallSplitSepLocal:
			arrayIndex := code[i]
			i++
			fieldSep := p.toString(p.pop())
			s := p.toString(p.pop())
			n, err := p.split(s, ast.ScopeGlobal, int(arrayIndex), fieldSep)
			if err != nil {
				return err
			}
			p.push(num(float64(n)))

		case bytecode.CallSprintf:
			numArgs := code[i]
			i++

			sp := len(p.st) - int(numArgs)
			s, err := p.sprintf(p.toString(p.st[sp]), p.st[sp+1:])
			p.st = p.st[:sp]
			if err != nil {
				return err
			}
			p.push(str(s))

		case bytecode.CallSqrt:
			p.push(num(math.Sqrt(p.pop().num())))

		case bytecode.CallSrand:
			prevSeed := p.randSeed
			p.random.Seed(time.Now().UnixNano())
			p.push(num(prevSeed))

		case bytecode.CallSrandSeed:
			prevSeed := p.randSeed
			p.randSeed = p.pop().num()
			p.random.Seed(int64(math.Float64bits(p.randSeed)))
			p.push(num(prevSeed))

		//case bytecode.CallSub:
		//case bytecode.CallSubField:
		//case bytecode.CallSubGlobal:
		//case bytecode.CallSubLocal:
		//case bytecode.CallSubSpecial:
		//case bytecode.CallSubArrayGlobal:
		//case bytecode.CallSubArrayLocal:

		case bytecode.CallSubstr:
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

		case bytecode.CallSubstrLength:
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

		case bytecode.CallSystem:
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

		case bytecode.CallTolower:
			p.push(str(strings.ToLower(p.toString(p.pop()))))

		case bytecode.CallToupper:
			p.push(str(strings.ToUpper(p.toString(p.pop()))))

		default:
			panic(fmt.Sprintf("unexpected opcode %s", op))
		}
	}
	return nil
}

func (p *interp) push(v value) {
	p.st = append(p.st, v)
}

func (p *interp) pop() value {
	last := len(p.st) - 1
	v := p.st[last]
	p.st = p.st[:last]
	return v
}

// Execute pattern-action blocks (may be multiple)
func (p *interp) execBytecodeActions(byteProg *bytecode.Program, actions []bytecode.Action) error {
	inRange := make([]bool, len(actions))
lineLoop:
	for {
		// Read and setup next line of input
		line, err := p.nextLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		p.setLine(line, false)

		// Execute all the pattern-action blocks for each line
		for i, action := range actions {
			// First determine whether the pattern matches
			matched := false
			switch len(action.Pattern) {
			case 0:
				// No pattern is equivalent to pattern evaluating to true
				matched = true
			case 1:
				// Single boolean pattern
				err := p.execBytecode(byteProg, action.Pattern[0])
				if err != nil {
					return err
				}
				matched = p.pop().boolean()
			case 2:
				// Range pattern (matches between start and stop lines)
				if !inRange[i] {
					err := p.execBytecode(byteProg, action.Pattern[0])
					if err != nil {
						return err
					}
					inRange[i] = p.pop().boolean()
				}
				matched = inRange[i]
				if inRange[i] {
					err := p.execBytecode(byteProg, action.Pattern[1])
					if err != nil {
						return err
					}
					inRange[i] = !p.pop().boolean()
				}
			}
			if !matched {
				continue
			}

			// No action is equivalent to { print $0 }
			if len(action.Body) == 0 {
				err := p.printLine(p.output, p.line)
				if err != nil {
					return err
				}
				continue
			}

			// Execute the body statements
			err := p.execBytecode(byteProg, action.Body)
			if err == errNext {
				// "next" statement skips straight to next line
				continue lineLoop
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
