// +build gofuzzbeta

package interp_test

import (
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func FuzzInterp(f *testing.F) {
	for _, test := range interpTests {
		if test.err == "" && test.awkErr == "" && !strings.Contains(test.src, "!fuzz") {
			f.Add(test.src, test.in)
		}
	}
	f.Fuzz(func(t *testing.T, src, in string) {
		prog, err := parser.ParseProgram([]byte(src), nil)
		if err != nil {
			return
			//t.Fatalf("%s:\nerror parsing: %v", src, err)
		}
		outBuf := &concurrentBuffer{}
		config := &interp.Config{
			Stdin:        strings.NewReader(in),
			Output:       outBuf,
			Error:        outBuf,
			NoExec:       true,
			NoFileWrites: true,
			NoFileReads:  true,
		}
		_, err = interp.ExecProgram(prog, config)
		if err != nil {
			return
			//t.Fatalf("%s:\nerror interpreting: %v", src, err)
		}
	})
}
