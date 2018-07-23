// Tests for GoAWK interpreter.
package interp_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func TestInterp(t *testing.T) {
	tests := []struct {
		src string
		in  string
		out string
	}{
		{`{ print toupper($1 $2) }`, "Fo o\nB aR", "FOO\nBAR\n"},

		// Grammar should allow blocks wherever statements are allowed
		{`BEGIN { if (1) printf "x"; else printf "y" }`, "", "x"},
		{`BEGIN { printf "x"; { printf "y"; printf "z" } }`, "", "xyz"},
	}
	for _, test := range tests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}
		t.Run(testName, func(t *testing.T) {
			// TODO: test against awk executable too

			prog, err := parser.ParseProgram([]byte(test.src))
			if err != nil {
				t.Fatal(err)
			}
			outBuf := &bytes.Buffer{}
			p := interp.New(prog, outBuf)
			err = p.ExecStream(strings.NewReader(test.in))
			if err != nil {
				t.Fatal(err)
			}
			outStr := outBuf.String()
			if test.out != outStr {
				t.Fatalf("expected %q, got %q", test.out, outStr)
			}
		})
	}
}
