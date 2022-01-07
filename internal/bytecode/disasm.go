package bytecode

import (
	"fmt"
	"io"

	"github.com/benhoyt/goawk/lexer"
)

func (p *Program) Disassemble(writer io.Writer) error {
	if p.Begin != nil {
		d := &disassembler{
			program: p,
			writer:  writer,
			code:    p.Begin,
		}
		err := d.disassemble("BEGIN")
		if err != nil {
			return err
		}
	}

	for _, action := range p.Actions {
		switch len(action.Pattern) {
		case 0:
		case 1:
			d := &disassembler{
				program: p,
				writer:  writer,
				code:    action.Pattern[0],
			}
			err := d.disassemble("match pattern")
			if err != nil {
				return err
			}
		case 2:
			d := &disassembler{
				program: p,
				writer:  writer,
				code:    action.Pattern[0],
			}
			err := d.disassemble("start pattern")
			if err != nil {
				return err
			}
			d = &disassembler{
				program: p,
				writer:  writer,
				code:    action.Pattern[1],
			}
			err = d.disassemble("stop pattern")
			if err != nil {
				return err
			}
		}
		if len(action.Body) > 0 {
			d := &disassembler{
				program: p,
				writer:  writer,
				code:    action.Body,
			}
			err := d.disassemble("{ body }")
			if err != nil {
				return err
			}
		}
	}

	if p.End != nil {
		d := &disassembler{
			program: p,
			writer:  writer,
			code:    p.End,
		}
		err := d.disassemble("END")
		if err != nil {
			return err
		}
	}
	return nil
}

type disassembler struct {
	program *Program
	writer  io.Writer
	code    []Op
	ip      int
	opAddr  int
	err     error
}

func (d *disassembler) disassemble(prefix string) error {
	if prefix != "" {
		d.writef("        // %s\n", prefix)
	}

	for d.ip < len(d.code) && d.err == nil {
		d.opAddr = d.ip
		op := d.fetch()

		switch op {
		case Nop:
			d.writeOpf("Nop")

		case Num:
			index := d.fetch()
			num := d.program.Nums[index]
			if num == float64(int(num)) {
				d.writeOpf("Num %d", int(num))
			} else {
				d.writeOpf("Num %.6g", num)
			}

		case Str:
			index := d.fetch()
			d.writeOpf("Str %q", d.program.Strs[index])

		case Dupe:
			d.writeOpf("Dupe")

		case Drop:
			d.writeOpf("Drop")

		case Field:
			d.writeOpf("Field")

		case Global:
			index := d.fetch()
			d.writeOpf("Global %s", d.program.ScalarNames[index])

		case Special:
			index := d.fetch()
			d.writeOpf("Special %d", index) // TODO: show name instead

		case AssignGlobal:
			index := d.fetch()
			d.writeOpf("AssignGlobal %s", d.program.ScalarNames[index])

		case PostIncrGlobal:
			index := d.fetch()
			d.writeOpf("PostIncrGlobal %s", d.program.ScalarNames[index])

		case AugAssignGlobal:
			operation := lexer.Token(d.fetch())
			index := d.fetch()
			d.writeOpf("AugAssignGlobal %s %s", operation, d.program.ScalarNames[index])

		case PostIncrArrayGlobal:
			arrayIndex := d.fetch()
			d.writeOpf("PostIncrArrayGlobal %s", d.program.ArrayNames[arrayIndex])

		case Less:
			d.writeOpf("Less")

		case LessOrEqual:
			d.writeOpf("LessOrEqual")

		case Jump:
			offset := int32(d.fetch())
			d.writeOpf("Jump %04x", d.ip+int(offset))

		case JumpFalse:
			offset := int32(d.fetch())
			d.writeOpf("JumpFalse %04x", d.ip+int(offset))

		case JumpTrue:
			offset := int32(d.fetch())
			d.writeOpf("JumpTrue %04x", d.ip+int(offset))

		case JumpNumLess:
			offset := int32(d.fetch())
			d.writeOpf("JumpNumLess %04x", d.ip+int(offset))

		case JumpNumLessOrEqual:
			offset := int32(d.fetch())
			d.writeOpf("JumpNumLessOrEqual %04x", d.ip+int(offset))

		case CallBuiltin:
			function := lexer.Token(d.fetch())
			switch function {
			case lexer.F_TOLOWER:
				d.writeOpf("CallBuiltin tolower")
			}

		case Print:
			numArgs := d.fetch()
			d.writeOpf("Print %d", numArgs)

		default:
			panic(fmt.Sprintf("unexpected opcode %d", op))
		}
	}

	d.writef("\n")
	return d.err
}

func (d *disassembler) fetch() Op {
	op := d.code[d.ip]
	d.ip++
	return op
}

func (d *disassembler) writef(format string, args ...interface{}) {
	if d.err != nil {
		return
	}
	_, d.err = fmt.Fprintf(d.writer, format, args...)
}

func (d *disassembler) writeOpf(format string, args ...interface{}) {
	if d.err != nil {
		return
	}
	addrStr := fmt.Sprintf("%04x", d.opAddr)
	_, d.err = fmt.Fprintf(d.writer, addrStr+"    "+format+"\n", args...)
}
