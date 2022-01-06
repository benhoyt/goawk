package bytecode

import (
	"fmt"
	"io"

	"github.com/benhoyt/goawk/lexer"
)

func (p *Program) Disassemble(w io.Writer) error {
	if p.Begin != nil {
		writef(w, "BEGIN:\n")
		p.disassembleCode(w, p.Begin)
	}
	if p.End != nil {
		writef(w, "END:\n")
		p.disassembleCode(w, p.End)
	}
	return nil
}

func (p *Program) disassembleCode(w io.Writer, code []Opcode) {
	for i := 0; i < len(code); {
		addr := i
		op := code[i]
		i++

		switch op {
		case Nop:
			writeOpcodef(w, addr, "Nop")

		case Num:
			index := code[i]
			i++
			num := p.Nums[index]
			if num == float64(int(num)) {
				writeOpcodef(w, addr, "Num %d", int(num))
			} else {
				writeOpcodef(w, addr, "Num %.6g", num)
			}

		case Str:
			index := code[i]
			i++
			writeOpcodef(w, addr, "Str %q", p.Strs[index])

		case Drop:
			writeOpcodef(w, addr, "Drop")

		case Dupe:
			writeOpcodef(w, addr, "Dupe")

		case Global:
			index := code[i]
			i++
			writeOpcodef(w, addr, "Global %s", p.ScalarNames[index])

		case AssignGlobal:
			index := code[i]
			i++
			writeOpcodef(w, addr, "AssignGlobal %s", p.ScalarNames[index])

		case PostIncrGlobal:
			index := code[i]
			i++
			writeOpcodef(w, addr, "PostIncrGlobal %s", p.ScalarNames[index])

		case AugAssignGlobal:
			operation := lexer.Token(code[i])
			index := code[i+1]
			i += 2
			writeOpcodef(w, addr, "AugAssignGlobal %s %s", operation, p.ScalarNames[index])

		case Less:
			writeOpcodef(w, addr, "Less")

		case Jump:
			offset := int32(code[i])
			i++
			writeOpcodef(w, addr, "Jump %04x", i+int(offset))

		case JumpFalse:
			offset := int32(code[i])
			i++
			writeOpcodef(w, addr, "JumpFalse %04x", i+int(offset))

		case JumpTrue:
			offset := int32(code[i])
			i++
			writeOpcodef(w, addr, "JumpTrue %04x", i+int(offset))

		case JumpNumLess:
			offset := int32(code[i])
			i++
			writeOpcodef(w, addr, "JumpNumLess %04x", i+int(offset))

		case Print:
			numArgs := code[i]
			i++
			writeOpcodef(w, addr, "Print %d", numArgs)

		default:
			panic(fmt.Sprintf("unexpected opcode %d", op))
		}
	}
}

func writef(w io.Writer, format string, args ...interface{}) {
	fmt.Fprintf(w, format, args...)
}

func writeOpcodef(w io.Writer, addr int, format string, args ...interface{}) {
	addrStr := fmt.Sprintf("%04x", addr)
	fmt.Fprintf(w, addrStr+"    "+format+"\n", args...)
}
