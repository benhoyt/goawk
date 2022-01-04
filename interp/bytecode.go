/*
To test this, use:

go build .
time ./goawk '{ for (i = 1; i <= NF; i++) counts[tolower($i)]++ } END { for (k in counts) print k, counts[k] }' < ../countwords/kjvbible_x10.txt >/dev/null
*/

package interp

import (
	"fmt"
	"strings"

	"github.com/benhoyt/goawk/internal/ast"
)

const (
	opNum0 = iota
	opNum1
	opNum2
	opNum3
	opGetg0
	opGetg1
	opGetg2
	opGetg3
	opSetg0
	opSetg1
	opSetg2
	opSetg3
	opIncrg0
	opIncrg1
	opIncrg2
	opIncrg3
	opAddAssign0
	opAddAssign1
	opAddAssign2
	opAddAssign3
	opAdd
	opLess
	opJz
	opJg
	opJge
	opJump
	opPrint0
	opPrint1
	opPrint2
	opPrint3
	opGetSpecial
	opGetField
	opTolower
	opMapIncr0
)

func opString(opcode uint8) string {
	switch opcode {
	case opNum0:
		return "NUM0"
	case opNum1:
		return "NUM1"
	case opNum2:
		return "NUM2"
	case opNum3:
		return "NUM3"
	case opGetg0:
		return "GETG0"
	case opGetg1:
		return "GETG1"
	case opGetg2:
		return "GETG2"
	case opGetg3:
		return "GETG3"
	case opSetg0:
		return "SETG0"
	case opSetg1:
		return "SETG1"
	case opSetg2:
		return "SETG2"
	case opSetg3:
		return "SETG3"
	case opIncrg0:
		return "INCRG0"
	case opIncrg1:
		return "INCRG1"
	case opIncrg2:
		return "INCRG2"
	case opIncrg3:
		return "INCRG3"
	case opAddAssign0:
		return "ADDASSIGN0"
	case opAddAssign1:
		return "ADDASSIGN1"
	case opAddAssign2:
		return "ADDASSIGN2"
	case opAddAssign3:
		return "ADDASSIGN3"
	case opAdd:
		return "ADD"
	case opLess:
		return "LESS"
	case opJz:
		return "JZ"
	case opJg:
		return "JG"
	case opJge:
		return "JGE"
	case opJump:
		return "JUMP"
	case opPrint0:
		return "PRINT0"
	case opPrint1:
		return "PRINT1"
	case opPrint2:
		return "PRINT2"
	case opPrint3:
		return "PRINT3"
	case opGetSpecial:
		return "GETSPECIAL"
	case opGetField:
		return "GETFIELD"
	case opTolower:
		return "TOLOWER"
	case opMapIncr0:
		return "MAPINCR0"
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
		//fmt.Printf("%d: %s %v counts=%v\n", pc, opString(opcode), p.stack, p.arrays[0])
		pc++
		switch opcode {
		case opNum0, opNum1, opNum2, opNum3:
			p.push(num(chunk.nums[opcode-opNum0]))
		case opGetg0, opGetg1, opGetg2, opGetg3:
			p.push(p.globals[opcode-opGetg0])
		case opSetg0, opSetg1, opSetg2, opSetg3:
			p.globals[opcode-opSetg0] = p.pop()
		case opIncrg0, opIncrg1, opIncrg2, opIncrg3:
			index := opcode - opIncrg0
			p.globals[index] = num(p.globals[index].num() + 1)
		case opAddAssign0, opAddAssign1, opAddAssign2, opAddAssign3:
			index := opcode - opAddAssign0
			r := p.pop()
			p.globals[index] = num(p.globals[index].num() + r.num())
		case opGetSpecial:
			index := int(opcodes[pc])
			pc++
			p.push(p.getVar(ast.ScopeSpecial, index))
		case opGetField:
			index := p.pop()
			field, _ := p.getField(int(index.num()))
			p.push(field)
		case opTolower:
			s := p.pop()
			lower := strings.ToLower(p.toString(s))
			p.push(str(lower))
		case opMapIncr0:
			index := p.pop()
			indexStr := p.toString(index)
			v := p.arrays[0][indexStr]
			p.arrays[0][indexStr] = num(v.num() + 1)
		case opAdd:
			r := p.pop()
			l := p.pop()
			p.push(num(l.num() + r.num()))
		case opLess:
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				p.push(boolean(p.toString(l) < p.toString(r)))
			} else {
				p.push(boolean(ln < rn))
			}
		case opJz:
			offset := int(int8(opcodes[pc]))
			pc++
			if p.pop().n == 0 {
				pc += offset
			}
		case opJg:
			offset := int(int8(opcodes[pc]))
			pc++
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				if p.toString(l) > p.toString(r) {
					pc += offset
				}
			} else {
				if ln > rn {
					pc += offset
				}
			}
		case opJge:
			offset := int(int8(opcodes[pc]))
			pc++
			r := p.pop()
			l := p.pop()
			ln, lIsStr := l.isTrueStr()
			rn, rIsStr := r.isTrueStr()
			if lIsStr || rIsStr {
				if p.toString(l) >= p.toString(r) {
					pc += offset
				}
			} else {
				if ln >= rn {
					pc += offset
				}
			}
		case opJump:
			offset := int(int8(opcodes[pc]))
			pc += 1 + offset
		case opPrint0, opPrint1, opPrint2, opPrint3:
			numArgs := int(opcode - opPrint0)
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
