package interp

import (
	"fmt"
	"strings"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/bytecode"
	"github.com/benhoyt/goawk/lexer"
)

func (p *interp) executeCode(prog *bytecode.Program, code []bytecode.Opcode) error {
	for i := 0; i < len(code); {
		op := code[i]
		i++

		switch op {
		case bytecode.Nop:

		case bytecode.Num:
			index := code[i]
			i++
			p.push(num(prog.Nums[index]))

		case bytecode.Str:
			index := code[i]
			i++
			p.push(str(prog.Strs[index]))

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

		case bytecode.AssignGlobal:
			index := code[i]
			i++
			p.globals[index] = p.pop()

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

		case bytecode.JumpNumLessOrEqual:
			offset := int32(code[i])
			r := p.pop()
			l := p.pop()
			if l.num() <= r.num() {
				i += 1 + int(offset)
			} else {
				i++
			}

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
			return p.printLine(p.output, line)

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
