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
	flag.StringVar(&awkExe, "awk", "awk", "awk executable name")
	flag.Parse()
	os.Exit(m.Run())
}

// Note: a lot of these are really parser tests too.
func TestInterp(t *testing.T) {
	tests := []struct {
		src    string
		in     string
		out    string
		err    string
		awkErr string
	}{
		{`{ print toupper($1 $2) }`, "Fo o\nB aR", "FOO\nBAR\n", "", ""},

		// Greater than operator requires parentheses in print statement,
		// otherwise it's a redirection directive
		{`BEGIN { print "x" > "out" }`, "", "", "", ""},
		{`BEGIN { printf "x" > "out" }`, "", "", "", ""},
		{`BEGIN { print("x" > "out") }`, "", "1\n", "", ""},
		{`BEGIN { printf("x" > "out") }`, "", "1", "", ""},

		// Grammar should allow blocks wherever statements are allowed
		{`BEGIN { if (1) printf "x"; else printf "y" }`, "", "x", "", ""},
		{`BEGIN { printf "x"; { printf "y"; printf "z" } }`, "", "xyz", "", ""},

		// Conditional expressions parse and work correctly
		{`BEGIN { print 0?"t":"f" }`, "", "f\n", "", ""},
		{`BEGIN { print 1?"t":"f" }`, "", "t\n", "", ""},
		{`BEGIN { print (1+2)?"t":"f" }`, "", "t\n", "", ""},
		{`BEGIN { print (1+2?"t":"f") }`, "", "t\n", "", ""},

		// Ensure certain odd syntax matches awk behaviour
		{`BEGIN { printf "x" }; BEGIN { printf "y" }`, "", "xy", "", ""},

		// Ensure syntax errors result in errors
		{`BEGIN { }'`, "", "", `parse error at 1:10: unexpected '\''`, `invalid char ''' in expression`},
		{`{ $1 = substr($1, 1, 3) print $1 }`, "", "", "ERROR", "syntax error"},
		{`BEGIN { printf "x" };; BEGIN { printf "y" }`, "", "xy", `parse error at 1:21: expected expression instead of ;`, "each rule must have a pattern or an action part"},
	}
	for _, test := range tests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}
		// Run it through external awk program first
		t.Run("awk_"+testName, func(t *testing.T) {
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
			out, err := cmd.CombinedOutput()
			if err != nil {
				if test.awkErr != "" {
					if strings.Contains(string(out), test.awkErr) {
						return
					}
					t.Fatalf("expected error %q, got:\n%s", test.awkErr, out)
				} else {
					t.Fatalf("error running %s: %v:\n%s", awkExe, err, out)
				}
			}
			if test.awkErr != "" {
				t.Fatalf(`expected error %q, got ""`, test.awkErr)
			}
			if string(out) != test.out {
				t.Fatalf("expected %q, got %q", test.out, out)
			}
		})

		// Then test it in GoAWK
		t.Run("goawk_"+testName, func(t *testing.T) {
			prog, err := parser.ParseProgram([]byte(test.src))
			if err != nil {
				if test.err != "" {
					if err.Error() == test.err {
						return
					}
					t.Fatalf("expected error %q, got %q", test.err, err.Error())
				}
				t.Fatal(err)
			}
			outBuf := &bytes.Buffer{}
			errBuf := &bytes.Buffer{}
			p := interp.New(prog, outBuf, errBuf)
			err = p.ExecStream(strings.NewReader(test.in))
			if err != nil {
				if test.err != "" {
					if err.Error() == test.err {
						return
					}
					t.Fatalf("expected error %q, got %q", test.err, err.Error())
				}
				t.Fatal(err)
			}
			if test.err != "" {
				t.Fatalf(`expected error %q, got ""`, test.err)
			}
			out := outBuf.String() + errBuf.String()
			if out != test.out {
				t.Fatalf("expected %q, got %q", test.out, out)
			}
		})
	}
	_ = os.Remove("out")
}
