package bytecode

import (
	"fmt"
	"io"

	"github.com/benhoyt/goawk/internal/ast"
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
			err := d.disassemble("pattern")
			if err != nil {
				return err
			}
		case 2:
			d := &disassembler{
				program: p,
				writer:  writer,
				code:    action.Pattern[0],
			}
			err := d.disassemble("start")
			if err != nil {
				return err
			}
			d = &disassembler{
				program: p,
				writer:  writer,
				code:    action.Pattern[1],
			}
			err = d.disassemble("stop")
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

		case Global:
			index := d.fetch()
			d.writeOpf("Global %s", d.program.ScalarNames[index])

		case Special:
			index := d.fetch()
			d.writeOpf("Special %s", ast.SpecialVarName(int(index)))

		case ArrayGlobal:
			arrayIndex := d.fetch()
			d.writeOpf("ArrayGlobal %s", d.program.ArrayNames[arrayIndex])

		case AssignGlobal:
			index := d.fetch()
			d.writeOpf("AssignGlobal %s", d.program.ScalarNames[index])

		case AssignLocal:
			index := d.fetch()
			d.writeOpf("AssignLocal %d", index) // TODO: local name

		case AssignSpecial:
			index := d.fetch()
			d.writeOpf("AssignSpecial %s", ast.SpecialVarName(int(index)))

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

		case Regex:
			regexIndex := d.fetch()
			d.writeOpf("Regex %q", d.program.Regexes[regexIndex])

		case Jump:
			offset := int32(d.fetch())
			d.writeOpf("Jump 0x%04x", d.ip+int(offset))

		case JumpFalse:
			offset := int32(d.fetch())
			d.writeOpf("JumpFalse 0x%04x", d.ip+int(offset))

		case JumpTrue:
			offset := int32(d.fetch())
			d.writeOpf("JumpTrue 0x%04x", d.ip+int(offset))

		case JumpNumLess:
			offset := int32(d.fetch())
			d.writeOpf("JumpNumLess 0x%04x", d.ip+int(offset))

		case JumpNumGreater:
			offset := int32(d.fetch())
			d.writeOpf("JumpNumGreater 0x%04x", d.ip+int(offset))

		case JumpNumLessOrEqual:
			offset := int32(d.fetch())
			d.writeOpf("JumpNumLessOrEqual 0x%04x", d.ip+int(offset))

		case ForGlobalInGlobal:
			varIndex := d.fetch()
			arrayIndex := d.fetch()
			offset := d.fetch()
			d.writeOpf("ForGlobalInGlobal %s %s 0x%04x", d.program.ScalarNames[varIndex], d.program.ArrayNames[arrayIndex], d.ip+int(offset))

		//case CallGsub:
		//	d.writeOpf("CallGsub")
		//case CallGsubField:
		//	d.writeOpf("CallGsubField")
		//case CallGsubGlobal:
		//	d.writeOpf("CallGsubGlobal")
		//case CallGsubLocal:
		//	d.writeOpf("CallGsubLocal")
		//case CallGsubSpecial:
		//	d.writeOpf("CallGsubSpecial")
		//case CallGsubArrayGlobal:
		//	d.writeOpf("CallGsubArrayGlobal")
		//case CallGsubArrayLocal:
		//	d.writeOpf("CallGsubArrayLocal")

		case CallSplitGlobal:
			arrayIndex := d.fetch()
			d.writeOpf("CallSplitGlobal %s", d.program.ArrayNames[arrayIndex])

		case CallSplitLocal:
			arrayIndex := d.fetch()
			d.writeOpf("CallSplitLocal %s", d.program.ArrayNames[arrayIndex])

		case CallSplitSepGlobal:
			arrayIndex := d.fetch()
			d.writeOpf("CallSplitSepGlobal %s", d.program.ArrayNames[arrayIndex])

		case CallSplitSepLocal:
			arrayIndex := d.fetch()
			d.writeOpf("CallSplitSepLocal %s", d.program.ArrayNames[arrayIndex])

		case CallSprintf:
			numArgs := d.fetch()
			d.writeOpf("CallSprintf %d", numArgs)

		//case CallSub:
		//	d.writeOpf("CallSub")
		//case CallSubField:
		//	d.writeOpf("CallSubField")
		//case CallSubGlobal:
		//	d.writeOpf("CallSubGlobal")
		//case CallSubLocal:
		//	d.writeOpf("CallSubLocal")
		//case CallSubSpecial:
		//	d.writeOpf("CallSubSpecial")
		//case CallSubArrayGlobal:
		//	d.writeOpf("CallSubArrayGlobal")
		//case CallSubArrayLocal:
		//	d.writeOpf("CallSubArrayLocal")

		case Print:
			numArgs := d.fetch()
			redirect := lexer.Token(d.fetch())
			if redirect == lexer.ILLEGAL {
				d.writeOpf("Print %d", numArgs)
			} else {
				d.writeOpf("Print %d %s", numArgs, redirect)
			}

		case Printf:
			numArgs := d.fetch()
			redirect := lexer.Token(d.fetch())
			if redirect == lexer.ILLEGAL {
				d.writeOpf("Printf %d", numArgs)
			} else {
				d.writeOpf("Printf %d %s", numArgs, redirect)
			}

		default:
			d.writeOpf("%s", op)
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
