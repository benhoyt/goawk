package bytecode

import (
	"fmt"
	"io"
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
		op := code[i]
		i++

		switch op {
		case Nop:
			writeOpcodef(w, "Nop")
		case Num:
			index := int(code[i])
			i++
			writeOpcodef(w, "Num %.6g", p.Nums[index])
		case Str:
			index := int(code[i])
			i++
			writeOpcodef(w, "Str %q", p.Strs[index])
		case Regex:
			// TODO
		case Drop:
			writeOpcodef(w, "Drop")
		default:
			panic(fmt.Sprintf("unexpected opcode %d", op))
		}
	}
}

func writef(w io.Writer, format string, args ...interface{}) {
	fmt.Fprintf(w, format, args...)
}

func writeOpcodef(w io.Writer, format string, args ...interface{}) {
	fmt.Fprintf(w, "\t"+format+"\n", args...)
}
