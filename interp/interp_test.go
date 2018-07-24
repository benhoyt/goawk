// Tests for GoAWK interpreter.
package interp_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

var (
	awkExe string
)

func TestMain(m *testing.M) {
	flag.StringVar(&awkExe, "awk", "", "awk executable name")
	flag.Parse()
	os.Exit(m.Run())
}

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
			if awkExe != "" {
				srcFile, err := ioutil.TempFile("", "goawktest_")
				if err != nil {
					t.Fatalf("error creating temp file: %v", err)
				}
				defer os.Remove(srcFile.Name())
				_, err = srcFile.Write([]byte(test.src))
				if err != nil {
					t.Fatalf("error writing temp file: %v", err)
				}
				cmd := exec.Command(awkExe, "-f", srcFile.Name(), "-")
				if test.in != "" {
					stdin, err := cmd.StdinPipe()
					if err != nil {
						t.Fatalf("error fetching stdin pipe: %v", err)
					}
					go func() {
						defer stdin.Close()
						stdin.Write([]byte(test.in))
					}()
				}
				expected, err := cmd.Output()
				if err != nil {
					t.Fatalf("error running %s: %v", awkExe, err)
				}
				if test.out != string(expected) {
					t.Fatalf("expected %q, got %q", test.out, expected)
				}
			} else {
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
			}
		})
	}
}
