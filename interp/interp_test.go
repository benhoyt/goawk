// Tests for GoAWK interpreter.
package interp_test

import (
	"bytes"
	"flag"
	"fmt"
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

// Note: a lot of these are really parser tests too.
func TestInterp(t *testing.T) {
	tests := []struct {
		src    string
		in     string
		out    string
		err    string
		awkErr string
	}{
		// BEGIN and END work correctly
		{`BEGIN { print "b" }`, "", "b\n", "", ""},
		{`BEGIN { print "b" }`, "foo", "b\n", "", ""},
		{`END { print "e" }`, "", "e\n", "", ""},
		{`END { print "e" }`, "foo", "e\n", "", ""},
		{`BEGIN { print "b"} END { print "e" }`, "", "b\ne\n", "", ""},
		{`BEGIN { print "b"} END { print "e" }`, "foo", "b\ne\n", "", ""},
		{`BEGIN { print "b"} $0 { print NR } END { print "e" }`, "foo", "b\n1\ne\n", "", ""},

		// Patterns
		{`$0`, "foo\n\nbar", "foo\nbar\n", "", ""},
		{`{ print $0 }`, "foo\n\nbar", "foo\n\nbar\n", "", ""},
		{`$1=="foo"`, "foo\n\nbar", "foo\n", "", ""},
		{`$1==42`, "foo\n42\nbar", "42\n", "", ""},
		{`$1=="42"`, "foo\n42\nbar", "42\n", "", ""},
		{`/foo/`, "foo\nx\nfood\nxfooz\nbar", "foo\nfood\nxfooz\n", "", ""},
		{`NR==2, NR==4`, "1\n2\n3\n4\n5\n6\n", "2\n3\n4\n", "", ""},
		{`
NR==2, NR==4 { print $0 }
NR==3, NR==5 { print NR }
`, "a\nb\nc\nd\ne\nf\ng", "b\nc\n3\nd\n4\n5\n", "", ""},

		// print and printf statements
		{`BEGIN { print "x", "y" }`, "", "x y\n", "", ""},
		{`BEGIN { OFS = ","; print "x", "y" }`, "", "x,y\n", "", ""},
		{`BEGIN { ORS = "."; print "x", "y" }`, "", "x y.", "", ""},
		{`BEGIN { ORS = ""; print "x", "y" }`, "", "x y", "", ""},
		{`{ print; print }`, "foo", "foo\nfoo\n", "", ""},
		{`BEGIN { print; print }`, "", "\n\n", "", ""},
		{`BEGIN { printf "%% %d %x %c %f %s", 42, 42, 42, 42, 42 }`, "", "% 42 2a * 42.000000 42", "", ""},
		{`BEGIN { printf "%3d", 42 }`, "", " 42", "", ""},
		{`BEGIN { printf "%3s", "x" }`, "", "  x", "", ""},
		{`BEGIN { printf "%.1g", 42 }`, "", "4e+01", "", ""},

		// if and loop statements
		{`BEGIN { if (1) print "t"; }`, "", "t\n", "", ""},
		{`BEGIN { if (0) print "t"; }`, "", "", "", ""},
		{`BEGIN { if (1) print "t"; else print "f" }`, "", "t\n", "", ""},
		{`BEGIN { if (0) print "t"; else print "f" }`, "", "f\n", "", ""},

		// Builtin functions
		{`{ print tolower($1 $2) }`, "Fo o\nB aR", "foo\nbar\n", "", ""},
		{`{ print toupper($1 $2) }`, "Fo o\nB aR", "FOO\nBAR\n", "", ""},

		// Conditional expressions parse and work correctly
		{`BEGIN { print 0?"t":"f" }`, "", "f\n", "", ""},
		{`BEGIN { print 1?"t":"f" }`, "", "t\n", "", ""},
		{`BEGIN { print (1+2)?"t":"f" }`, "", "t\n", "", ""},
		{`BEGIN { print (1+2?"t":"f") }`, "", "t\n", "", ""},
		{`BEGIN { print(1 ? x="t" : "f"); print x; }`, "", "t\nt\n", "", ""},

		// Locals vs globals, array params, and recursion
		{`
function f(loc) {
	glob += 1
	loc += 1
	print glob, loc
}
BEGIN {
	glob = 1
	loc = 42
	f(3)
	print loc
	f(4)
	print loc
}
`, "", "2 4\n42\n3 5\n42\n", "", ""},
		{`
function set(a, x, v) {
	a[x] = v
}
function get(a, x) {
	return a[x]
}
BEGIN {
	a["x"] = 1
	set(b, "y", 2)
	for (k in a) print k, a[k]
	print "---"
	for (k in b) print k, b[k]
	print "---"
	print get(a, "x"), get(b, "y")
}
`, "", "x 1\n---\ny 2\n---\n1 2\n", "", ""},
		{`
function fib(n) {
	return n < 3 ? 1 : fib(n-2) + fib(n-1)
}
BEGIN {
	for (i = 1; i <= 7; i++) {
		printf "%d ", fib(i)
	}
}
`, "", "1 1 2 3 5 8 13 ", "", ""},

		// Greater than operator requires parentheses in print statement,
		// otherwise it's a redirection directive
		{`BEGIN { print "x" > "out" }`, "", "", "", ""},
		{`BEGIN { printf "x" > "out" }`, "", "", "", ""},
		{`BEGIN { print("x" > "out") }`, "", "1\n", "", ""},
		{`BEGIN { printf("x" > "out") }`, "", "1", "", ""},

		// Grammar should allow blocks wherever statements are allowed
		{`BEGIN { if (1) printf "x"; else printf "y" }`, "", "x", "", ""},
		// {`BEGIN { printf "x"; { printf "y"; printf "z" } }`, "", "xyz", "", ""},

		// Ensure certain odd syntax matches awk behaviour
		// {`BEGIN { printf "x" }; BEGIN { printf "y" }`, "", "xy", "", ""},
		// {`BEGIN { printf "x" };; BEGIN { printf "y" }`, "", "xy", "", ""},

		// Ensure syntax errors result in errors
		{`BEGIN { }'`, "", "", `parse error at 1:10: unexpected '\''`, "syntax error"},
		// {`{ $1 = substr($1, 1, 3) print $1 }`, "", "", "ERROR", "syntax error"},
	}
	for _, test := range tests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}

		if awkExe != "" {
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
		}

		// Then test it in GoAWK
		t.Run(testName, func(t *testing.T) {
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
			p := interp.New(outBuf, errBuf)
			err = p.Exec(prog, strings.NewReader(test.in), nil)
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

func benchmarkProgram(b *testing.B, n int, input, expected, srcFormat string, args ...interface{}) {
	b.StopTimer()
	src := fmt.Sprintf(srcFormat, args...)
	prog, err := parser.ParseProgram([]byte(src))
	if err != nil {
		b.Fatalf("error parsing %s: %v", b.Name(), err)
	}
	outBuf := &bytes.Buffer{}
	p := interp.New(outBuf, ioutil.Discard)
	if expected != "" {
		expected += "\n"
	}
	for i := 0; i < n; i++ {
		outBuf.Reset()
		b.StartTimer()
		err := p.Exec(prog, strings.NewReader(input), nil)
		b.StopTimer()
		if err != nil {
			b.Fatalf("error interpreting %s: %v", b.Name(), err)
		}
		if outBuf.String() != expected {
			b.Fatalf("expected %q, got %q", expected, outBuf.String())
		}
	}
}

func BenchmarkRecursiveFunc(b *testing.B) {
	benchmarkProgram(b, b.N, "", "13", `
function fib(n) {
  if (n <= 2) {
    return 1
  }
  return fib(n-1) + fib(n-2)
}

BEGIN {
  print fib(7)
}
`)
}

func BenchmarkFuncCall(b *testing.B) {
	b.StopTimer()
	sum := 0
	for i := 0; i < b.N; i++ {
		sum += i
	}
	benchmarkProgram(b, 1, "", fmt.Sprintf("%d", sum), `
function add(a, b) {
  return a + b
}

BEGIN {
  for (i = 0; i < %d; i++) {
    sum = add(sum, i)
  }
  print sum
}
`, b.N)
}

func BenchmarkForLoop(b *testing.B) {
	b.StopTimer()
	sum := 0
	for i := 0; i < b.N; i++ {
		sum += i
	}
	benchmarkProgram(b, 1, "", fmt.Sprintf("%d", sum), `
BEGIN {
  for (i = 0; i < %d; i++) {
    sum += i
  }
  print sum
}
`, b.N)
}

func BenchmarkSimplePattern(b *testing.B) {
	b.StopTimer()
	inputLines := []string{}
	expectedLines := []string{}
	for i := 0; i < b.N; i++ {
		if i != 0 && i%2 == 0 {
			line := fmt.Sprintf("%d", i)
			inputLines = append(inputLines, line)
			expectedLines = append(expectedLines, line)
		} else {
			inputLines = append(inputLines, "")
		}
	}
	input := strings.Join(inputLines, "\n")
	expected := strings.Join(expectedLines, "\n")
	benchmarkProgram(b, 1, input, expected, "$0")
}

func BenchmarkFields(b *testing.B) {
	b.StopTimer()
	inputLines := []string{}
	expectedLines := []string{}
	for i := 1; i < b.N+1; i++ {
		inputLines = append(inputLines, fmt.Sprintf("%d %d %d", i, i*2, i*3))
		expectedLines = append(expectedLines, fmt.Sprintf("%d %d", i, i*3))
	}
	input := strings.Join(inputLines, "\n")
	expected := strings.Join(expectedLines, "\n")
	benchmarkProgram(b, 1, input, expected, "{ print $1, $3 }")
}

func Example_simple() {
	input := bytes.NewReader([]byte("foo bar\n\nbaz buz"))
	err := interp.Exec("$0 { print $1 }", " ", input, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Output:
	// foo
	// baz
}

func Example_program() {
	src := "{ print $1+$2 }"
	input := "1,2\n3,4\n5,6"

	prog, err := parser.ParseProgram([]byte(src))
	if err != nil {
		fmt.Println(err)
		return
	}
	p := interp.New(nil, nil)
	p.SetVar("FS", ",")
	err = p.Exec(prog, bytes.NewReader([]byte(input)), nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Output:
	// 3
	// 7
	// 11
}

func Example_expression() {
	src := "1 + 2 * 3 / 4"

	expr, err := parser.ParseExpr([]byte(src))
	if err != nil {
		fmt.Println(err)
		return
	}
	p := interp.New(nil, nil)
	n, err := p.EvalNum(expr)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(n)
	// Output:
	// 2.5
}
