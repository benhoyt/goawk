package interp

import (
	"fmt"

	"github.com/benhoyt/goawk/internal/bytecode"
)

func (p *interp) executeCode(prog *bytecode.Program, code []bytecode.Opcode) error {
	for i := 0; i < len(code); {
		op := code[i]
		i++

		switch op {
		case bytecode.Nop:
		case bytecode.Num:
			index := int(code[i])
			i++
			p.push(num(prog.Nums[index]))
		case bytecode.Str:
			index := int(code[i])
			i++
			p.push(str(prog.Strs[index]))
		case bytecode.Regex:
			// TODO
		case bytecode.Drop:
			v := p.pop()
			fmt.Printf("Dropping %s\n", p.toString(v))
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
