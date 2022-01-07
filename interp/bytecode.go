package interp

import (
	"fmt"
	"io"
	"strings"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/bytecode"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

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
			p.push(p.getVar(ast.ScopeSpecial, int(index)))

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

		case bytecode.AssignGlobal:
			index := code[i]
			i++
			p.globals[index] = p.pop()

		case bytecode.AssignField:
			index := p.pop()
			right := p.pop()
			err := p.setField(int(index.num()), p.toString(right))
			if err != nil {
				return err
			}

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

		case bytecode.Print:
			numArgs := code[i]
			i++

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
			err := p.printLine(p.output, line)
			if err != nil {
				return err
			}

		case bytecode.CallBuiltin:
			function := code[i]
			i++
			switch lexer.Token(function) {
			case lexer.F_TOLOWER:
				v := p.pop()
				v = str(strings.ToLower(p.toString(v)))
				p.push(v)
			}

		default:
			panic(fmt.Sprintf("unexpected opcode %d", op))
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
