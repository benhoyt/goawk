package compiler

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestDisassembler(t *testing.T) {
	// Note: this doesn't really test the disassembly, just that each opcode
	// disassembly includes the opcode name, to help catch silly typos.
	for op := Nop; op < EndOpcode; op++ {
		t.Run(op.String(), func(t *testing.T) {
			p := Program{
				Begin: []Opcode{op, 0, 0, 0, 0, 0, 0, 0},
				Functions: []Function{
					{
						Name:       "f",
						Params:     []string{"a", "k"},
						Arrays:     []bool{true, false},
						NumScalars: 1,
						NumArrays:  1,
					},
				},
				Nums:            []float64{0},
				Strs:            []string{""},
				Regexes:         []*regexp.Regexp{regexp.MustCompile("")},
				scalarNames:     []string{"s"},
				arrayNames:      []string{"a"},
				nativeFuncNames: []string{"n"},
			}
			var buf bytes.Buffer
			err := p.Disassemble(&buf)
			if err != nil {
				t.Fatalf("error disassembling opcode %s: %v", op, err)
			}
			lines := strings.Split(buf.String(), "\n")
			if strings.TrimSpace(lines[0]) != "// BEGIN" {
				t.Fatalf("first line should be \"// BEGIN\", not %q", lines[0])
			}
			fields := strings.Fields(lines[1])
			if fields[0] != "0000" {
				t.Fatalf("address should be \"0000\", not %q", fields[0])
			}
			if fields[1] != op.String() {
				t.Fatalf("opcode name should be %q, not %q", op.String(), fields[1])
			}
		})
	}
}
