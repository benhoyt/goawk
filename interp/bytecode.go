package interp

import (
	"fmt"
	"strings"
)

const (
	opNum = iota
	opGetg
	opSetg
	opAdd
	opLess
	opJz
	opJump
	opPrint
)

func opString(opcode uint8) string {
	switch opcode {
	case opNum:
		return "NUM"
	case opGetg:
		return "GETG"
	case opSetg:
		return "SETG"
	case opAdd:
		return "ADD"
	case opLess:
		return "LESS"
	case opJz:
		return "JZ"
	case opJump:
		return "JUMP"
	case opPrint:
		return "PRINT"
	default:
		return fmt.Sprintf("UNKNOWN OPCODE %d", opcode)
	}
}

type code struct {
	opcodes []uint8
	nums    []float64
}

func (p *interp) execBytecode(chunk code) error {
	opcodes := chunk.opcodes
	for pc := 0; pc < len(opcodes); {
		opcode := opcodes[pc]
		//fmt.Printf("%d: %s %v\n", pc, opString(opcode), p.stack)
		pc++
		switch opcode {
		case opNum:
			index := opcodes[pc]
			pc++
			p.push(num(chunk.nums[index]))
		case opGetg:
			index := opcodes[pc]
			pc++
			p.push(p.globals[index])
		case opSetg:
			index := opcodes[pc]
			pc++
			p.globals[index] = p.pop()
		case opAdd:
			r := p.pop()
			l := p.pop()
			p.push(num(l.num() + r.num()))
		case opLess:
			r := p.pop()
			l := p.pop()
			if l.isTrueStr() || r.isTrueStr() {
				p.push(boolean(p.toString(l) < p.toString(r)))
			} else {
				p.push(boolean(l.n < r.n))
			}
		case opJz:
			offset := int(int8(opcodes[pc]))
			pc++
			if p.pop().n == 0 {
				pc += offset
			}
		case opJump:
			offset := int(int8(opcodes[pc]))
			pc += 1 + offset
		case opPrint:
			numArgs := int(opcodes[pc])
			pc++
			// Print OFS-separated args followed by ORS (usually newline)
			var line string
			if numArgs > 0 {
				strs := make([]string, numArgs)
				args := p.stack[len(p.stack)-numArgs:]
				for i, v := range args {
					strs[i] = v.str(p.outputFormat)
				}
				line = strings.Join(strs, p.outputFieldSep)
			} else {
				// "print" with no args is equivalent to "print $0"
				line = p.line
			}
			// TODO
			//output, err := p.getOutputStream(s.Redirect, s.Dest)
			//if err != nil {
			//	return err
			//}
			err := p.printLine(p.output, line)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *interp) push(v value) {
	p.stack = append(p.stack, v)
}

func (p *interp) pop() value {
	last := len(p.stack) - 1
	v := p.stack[last]
	p.stack = p.stack[:last]
	return v
}
