package ast

import (
	"testing"
)

func TestNameIndex(t *testing.T) {
	tests := []struct {
		name  string
		index int
	}{
		{"ILLEGAL", V_ILLEGAL},
		{"ARGC", V_ARGC},
		{"CONVFMT", V_CONVFMT},
		{"FILENAME", V_FILENAME},
		{"FNR", V_FNR},
		{"FS", V_FS},
		{"INPUTMODE", V_INPUTMODE},
		{"NF", V_NF},
		{"NR", V_NR},
		{"OFMT", V_OFMT},
		{"OFS", V_OFS},
		{"ORS", V_ORS},
		{"OUTPUTMODE", V_OUTPUTMODE},
		{"RLENGTH", V_RLENGTH},
		{"RS", V_RS},
		{"RSTART", V_RSTART},
		{"RT", V_RT},
		{"SUBSEP", V_SUBSEP},
		{"<unknown special var 42>", 42},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name := SpecialVarName(test.index)
			if name != test.name {
				t.Errorf("got %q, want %q", name, test.name)
			}
			if test.index <= V_LAST {
				index := SpecialVarIndex(test.name)
				if index != test.index {
					t.Errorf("got %d, want %d", index, test.index)
				}
			}
		})
	}
}
