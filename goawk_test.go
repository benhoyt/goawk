// GoAWK tests

package main_test

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

var (
	goExe      string
	testsDir   string
	outputDir  string
	awkExe     string
	goAWKExe   string
	writeAWK   bool
	writeGoAWK bool
)

func TestMain(m *testing.M) {
	flag.StringVar(&goExe, "goexe", "go", "set to override Go executable used to build goawk")
	flag.StringVar(&testsDir, "testsdir", "./testdata", "directory with one-true-awk tests")
	flag.StringVar(&outputDir, "outputdir", "./testdata/output", "directory for test output")
	flag.StringVar(&awkExe, "awk", "gawk", "awk executable name")
	flag.StringVar(&goAWKExe, "goawk", "./goawk", "goawk executable name")
	flag.BoolVar(&writeAWK, "writeawk", false, "write expected output")
	flag.BoolVar(&writeGoAWK, "writegoawk", true, "write Go AWK output")
	flag.Parse()

	cmd := exec.Command(goExe, "build")
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building goawk: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestAWK(t *testing.T) {
	inputByPrefix := map[string]string{
		"t": "test.data",
		"p": "test.countries",
	}
	// These programs exit with non-zero status code
	errorExits := map[string]bool{
		"t.exit":   true,
		"t.exit1":  true,
		"t.gsub4":  true,
		"t.split3": true,
	}
	// These programs have known different output
	knownDifferent := map[string]bool{
		"t.printf2": true, // because awk is weird here (our behavior is like mawk)
	}
	// Can't really diff test rand() tests as we're using a totally
	// different algorithm for random numbers
	randTests := map[string]bool{
		"p.48b":   true,
		"t.randk": true,
	}
	// These tests use "for (x in a)", which iterates in an undefined
	// order (according to the spec), so sort lines before comparing.
	sortLines := map[string]bool{
		"p.43":      true,
		"t.in1":     true, // because "sort" is locale-dependent
		"t.in2":     true,
		"t.intest2": true,
	}
	dontRunOnWindows := map[string]bool{
		"p.50": true, // because this pipes to Unix sort "sort -t: +0 -1 +2nr"
	}

	infos, err := ioutil.ReadDir(testsDir)
	if err != nil {
		t.Fatalf("couldn't read test files: %v", err)
	}
	for _, info := range infos {
		if !strings.HasPrefix(info.Name(), "t.") && !strings.HasPrefix(info.Name(), "p.") {
			continue
		}
		if runtime.GOOS == "windows" && dontRunOnWindows[info.Name()] {
			continue
		}
		t.Run(info.Name(), func(t *testing.T) {
			srcPath := filepath.Join(testsDir, info.Name())
			inputPath := filepath.Join(testsDir, inputByPrefix[info.Name()[:1]])
			outputPath := filepath.Join(outputDir, info.Name())

			cmd := exec.Command(awkExe, "-f", srcPath, inputPath)
			expected, err := cmd.Output()
			if err != nil && !errorExits[info.Name()] {
				t.Fatalf("error running %s: %v", awkExe, err)
			}
			expected = bytes.Replace(expected, []byte{0}, []byte("<00>"), -1)
			expected = normalizeNewlines(expected)
			if sortLines[info.Name()] {
				expected = sortedLines(expected)
			}
			if writeAWK {
				err := ioutil.WriteFile(outputPath, expected, 0644)
				if err != nil {
					t.Fatalf("error writing awk output: %v", err)
				}
			}

			prog, err := parseGoAWK(srcPath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := interpGoAWK(prog, inputPath)
			if err != nil && !errorExits[info.Name()] {
				t.Fatal(err)
			}
			output = bytes.Replace(output, []byte{0}, []byte("<00>"), -1)
			output = normalizeNewlines(output)
			if randTests[info.Name()] || knownDifferent[info.Name()] {
				// For tests that use rand(), run them to ensure they
				// parse and interpret, but can't compare the output,
				// so stop now
				return
			}
			if sortLines[info.Name()] {
				output = sortedLines(output)
			}
			if writeGoAWK {
				err := ioutil.WriteFile(outputPath, output, 0644)
				if err != nil {
					t.Fatalf("error writing goawk output: %v", err)
				}
			}
			if string(output) != string(expected) {
				t.Fatalf("output differs, run: git diff %s", outputPath)
			}
		})
	}

	_ = os.Remove("tempbig")
	_ = os.Remove("tempsmall")
}

func parseGoAWK(srcPath string) (*parser.Program, error) {
	src, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	prog, err := parser.ParseProgram(src, nil)
	if err != nil {
		return nil, err
	}
	return prog, nil
}

func interpGoAWK(prog *parser.Program, inputPath string) ([]byte, error) {
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	config := &interp.Config{
		Output: outBuf,
		Error:  &concurrentWriter{w: errBuf},
		Args:   []string{inputPath},
	}
	_, err := interp.ExecProgram(prog, config)
	result := outBuf.Bytes()
	result = append(result, errBuf.Bytes()...)
	return result, err
}

func interpGoAWKStdin(prog *parser.Program, inputPath string) ([]byte, error) {
	input, _ := ioutil.ReadFile(inputPath)
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	config := &interp.Config{
		Stdin:  &concurrentReader{r: bytes.NewReader(input)},
		Output: outBuf,
		Error:  &concurrentWriter{w: errBuf},
		// srcdir is for "redfilnm.awk"
		Vars: []string{"srcdir", filepath.Dir(inputPath)},
	}
	_, err := interp.ExecProgram(prog, config)
	result := outBuf.Bytes()
	result = append(result, errBuf.Bytes()...)
	return result, err
}

// Wraps a Writer but makes Write calls safe for concurrent use.
type concurrentWriter struct {
	w  io.Writer
	mu sync.Mutex
}

func (w *concurrentWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

// Wraps a Reader but makes Read calls safe for concurrent use.
type concurrentReader struct {
	r  io.Reader
	mu sync.Mutex
}

func (r *concurrentReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.r.Read(p)
}

func sortedLines(data []byte) []byte {
	trimmed := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(trimmed, "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

func TestGAWK(t *testing.T) {
	skip := map[string]bool{ // TODO: fix these
		"getline":  true, // getline syntax issues (may be okay, see grammar notes at http://pubs.opengroup.org/onlinepubs/007904975/utilities/awk.html#tag_04_06_13_14)
		"getline3": true, // getline syntax issues (similar to above)
		"getline5": true, // getline syntax issues (similar to above)
		"inputred": true, // getline syntax issues (similar to above)

		"gsubtst7":     true, // something wrong with gsub or field split/join
		"splitwht":     true, // other awks handle split(s, a, " ") differently from split(s, a, / /)
		"status-close": true, // hmmm, not sure what's up here
		"sigpipe1":     true, // probable race condition: sometimes fails, sometimes passes

		"parse1": true, // incorrect parsing of $$a++++ (see TODOs in interp_test.go too)

		"rscompat": true, // GoAWK allows multi-char RS by default
		"rsstart2": true, // GoAWK ^ and $ anchors match beginning and end of line, not file (unlike Gawk)
	}

	dontRunOnWindows := map[string]bool{
		"delargv":  true, // reads from /dev/null
		"eofsplit": true, // reads from /etc/passwd
		"iobug1":   true, // reads from /dev/null
	}

	sortLines := map[string]bool{
		"arryref2": true,
		"delargv":  true,
		"delarpm2": true,
		"forref":   true,
	}

	gawkDir := filepath.Join(testsDir, "gawk")
	infos, err := ioutil.ReadDir(gawkDir)
	if err != nil {
		t.Fatalf("couldn't read test files: %v", err)
	}
	for _, info := range infos {
		if !strings.HasSuffix(info.Name(), ".awk") {
			continue
		}
		testName := info.Name()[:len(info.Name())-4]
		if skip[testName] {
			continue
		}
		if runtime.GOOS == "windows" && dontRunOnWindows[testName] {
			continue
		}
		t.Run(testName, func(t *testing.T) {
			srcPath := filepath.Join(gawkDir, info.Name())
			inputPath := filepath.Join(gawkDir, testName+".in")
			okPath := filepath.Join(gawkDir, testName+".ok")

			expected, err := ioutil.ReadFile(okPath)
			if err != nil {
				t.Fatal(err)
			}
			expected = normalizeNewlines(expected)

			prog, err := parseGoAWK(srcPath)
			if err != nil {
				if err.Error() != string(expected) {
					t.Fatalf("parser error differs, got:\n%s\nexpected:\n%s", err.Error(), expected)
				}
				return
			}
			output, err := interpGoAWKStdin(prog, inputPath)
			output = normalizeNewlines(output)
			if err != nil {
				errStr := string(output) + err.Error()
				if errStr != string(expected) {
					t.Fatalf("interp error differs, got:\n%s\nexpected:\n%s", errStr, expected)
				}
				return
			}

			if sortLines[testName] {
				output = sortedLines(output)
				expected = sortedLines(expected)
			}

			if string(output) != string(expected) {
				t.Fatalf("output differs, got:\n%s\nexpected:\n%s", output, expected)
			}
		})
	}

	_ = os.Remove("seq")
}

func TestCommandLine(t *testing.T) {
	tests := []struct {
		args   []string
		stdin  string
		output string
		error  string
	}{
		// Load source from stdin
		{[]string{"-f", "-"}, `BEGIN { print "b" }`, "b\n", ""},
		{[]string{"-f", "-", "-f", "-"}, `BEGIN { print "b" }`, "b\n", ""},
		{[]string{"-f-", "-f", "-"}, `BEGIN { print "b" }`, "b\n", ""},

		// Program with no input
		{[]string{`BEGIN { print "a" }`}, "", "a\n", ""},

		// Read input from stdin
		{[]string{`$0`}, "one\n\nthree", "one\nthree\n", ""},
		{[]string{`$0`, "-"}, "one\n\nthree", "one\nthree\n", ""},
		{[]string{`$0`, "-", "-"}, "one\n\nthree", "one\nthree\n", ""},
		{[]string{"-f", "testdata/t.0", "-"}, "one\ntwo\n", "one\ntwo\n", ""},

		// Read input from file(s)
		{[]string{`$0`, "testdata/g.1"}, "", "ONE\n", ""},
		{[]string{`$0`, "testdata/g.1", "testdata/g.2"}, "", "ONE\nTWO\n", ""},
		{[]string{`{ print FILENAME ":" FNR "/" NR ": " $0 }`, "testdata/g.1", "testdata/g.4"}, "",
			"testdata/g.1:1/1: ONE\ntestdata/g.4:1/2: FOUR a\ntestdata/g.4:2/3: FOUR b\n", ""},
		{[]string{`$0`, "testdata/g.1", "-", "testdata/g.2"}, "STDIN", "ONE\nSTDIN\nTWO\n", ""},
		{[]string{`$0`, "testdata/g.1", "-", "testdata/g.2", "-"}, "STDIN", "ONE\nSTDIN\nTWO\n", ""},
		{[]string{"-F", " ", "--", "$0", "testdata/g.1"}, "", "ONE\n", ""},
		// I've deleted the "-ftest" file for now as it was causing problems with "go install" zip files
		// {[]string{"--", "$0", "-ftest"}, "", "used in tests; do not delete\n", ""}, // Issue #53
		// {[]string{"$0", "-ftest"}, "", "used in tests; do not delete\n", ""},

		// Specifying field separator with -F
		{[]string{`{ print $1, $3 }`}, "1 2 3\n4 5 6", "1 3\n4 6\n", ""},
		{[]string{"-F", ",", `{ print $1, $3 }`}, "1 2 3\n4 5 6", "1 2 3 \n4 5 6 \n", ""},
		{[]string{"-F", ",", `{ print $1, $3 }`}, "1,2,3\n4,5,6", "1 3\n4 6\n", ""},
		{[]string{"-F", ",", `{ print $1, $3 }`}, "1,2,3\n4,5,6", "1 3\n4 6\n", ""},
		{[]string{"-F,", `{ print $1, $3 }`}, "1,2,3\n4,5,6", "1 3\n4 6\n", ""},

		// Assigning other variables with -v
		{[]string{"-v", "OFS=.", `{ print $1, $3 }`}, "1 2 3\n4 5 6", "1.3\n4.6\n", ""},
		{[]string{"-v", "OFS=.", "-v", "ORS=", `{ print $1, $3 }`}, "1 2 3\n4 5 6", "1.34.6", ""},
		{[]string{"-v", "x=42", "-v", "y=foo", `BEGIN { print x, y }`}, "", "42 foo\n", ""},
		{[]string{"-v", "RS=;", `$0`}, "a b;c\nd;e", "a b\nc\nd\ne\n", ""},
		{[]string{"-vRS=;", `$0`}, "a b;c\nd;e", "a b\nc\nd\ne\n", ""},

		// ARGV/ARGC handling
		{[]string{`
			BEGIN {
				for (i=1; i<ARGC; i++) {
					print i, ARGV[i]
				}
			}`, "a", "b"}, "", "1 a\n2 b\n", ""},
		{[]string{`
			BEGIN {
				for (i=1; i<ARGC; i++) {
					print i, ARGV[i]
					delete ARGV[i]
				}
			}
			$0`, "a", "b"}, "c\nd", "1 a\n2 b\nc\nd\n", ""},
		{[]string{`
			BEGIN {
				ARGV[1] = ""
			}
			$0`, "testdata/g.1", "-", "testdata/g.2"}, "c\nd", "c\nd\nTWO\n", ""},
		{[]string{`
			BEGIN {
				ARGC = 3
			}
			$0`, "testdata/g.1", "-", "testdata/g.2"}, "c\nd", "ONE\nc\nd\n", ""},
		{[]string{"-v", "A=1", "-f", "testdata/g.3", "B=2", "testdata/test.countries"}, "",
			"A=1, B=0\n\tARGV[1] = B=2\n\tARGV[2] = testdata/test.countries\nA=1, B=2\n", ""},
		{[]string{`END { print (x==42) }`, "x=42.0"}, "", "1\n", ""},
		{[]string{"-v", "x=42.0", `BEGIN { print (x==42) }`}, "", "1\n", ""},
		{[]string{`BEGIN { print(ARGV[1]<2, ARGV[2]<2); ARGV[1]="10"; ARGV[2]="10x"; print(ARGV[1]<2, ARGV[2]<2) }`,
			"10", "10x"}, "", "0 1\n1 1\n", ""},

		// Error handling
		{[]string{}, "", "", "usage: goawk [-F fs] [-v var=value] [-f progfile | 'prog'] [file ...]"},
		{[]string{"-F"}, "", "", "flag needs an argument: -F"},
		{[]string{"-f"}, "", "", "flag needs an argument: -f"},
		{[]string{"-v"}, "", "", "flag needs an argument: -v"},
		{[]string{"-z"}, "", "", "flag provided but not defined: -z"},
		{[]string{"{ print }", "notexist"}, "", "", `file "notexist" not found`},
		{[]string{"BEGIN { print 1/0 }"}, "", "", "division by zero"},
		{[]string{"-v", "foo", "BEGIN {}"}, "", "", "-v flag must be in format name=value"},
		{[]string{"--", "{ print $1 }", "-file"}, "", "", `file "-file" not found`},
		{[]string{"{ print $1 }", "-file"}, "", "", `file "-file" not found`},

		// Output synchronization
		{[]string{`BEGIN { print "1"; print "2"|"cat" }`}, "", "1\n2\n", ""},
		{[]string{`BEGIN { print "1"; "echo 2" | getline x; print x }`}, "", "1\n2\n", ""},

		// Parse error formatting
		{[]string{"`"}, "", "", "<cmdline>:1:1: unexpected char\n`\n^"},
		{[]string{"BEGIN {\n\tx*;\n}"}, "", "", "<cmdline>:2:4: expected expression instead of ;\n    x*;\n      ^"},
		{[]string{"BEGIN {\n\tx*\r\n}"}, "", "", "<cmdline>:2:4: expected expression instead of <newline>\n    x*\n      ^"},
		{[]string{"-f", "-"}, "\n ++", "", "<stdin>:2:4: expected expression instead of <newline>\n ++\n   ^"},
		{[]string{"-f", "testdata/parseerror/good.awk", "-f", "testdata/parseerror/bad.awk"},
			"", "", "testdata/parseerror/bad.awk:2:3: expected expression instead of <newline>\nx*\n  ^"},
		{[]string{"-f", "testdata/parseerror/bad.awk", "-f", "testdata/parseerror/good.awk"},
			"", "", "testdata/parseerror/bad.awk:2:3: expected expression instead of <newline>\nx*\n  ^"},
		{[]string{"-f", "testdata/parseerror/good.awk", "-f", "-", "-f", "testdata/parseerror/bad.awk"},
			"`", "", "<stdin>:1:1: unexpected char\n`\n^"},
	}
	for _, test := range tests {
		testName := strings.Join(test.args, " ")
		t.Run(testName, func(t *testing.T) {
			runAWKs(t, test.args, test.stdin, test.output, test.error)
		})
	}
}

func TestDevStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/stdout not presnt on Windows")
	}
	runAWKs(t, []string{`BEGIN { print "1"; print "2">"/dev/stdout" }`}, "", "1\n2\n", "")
}

func runGoAWK(args []string, stdin string) (stdout, stderr string, err error) {
	cmd := exec.Command(goAWKExe, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	errBuf := &bytes.Buffer{}
	cmd.Stderr = errBuf
	output, err := cmd.Output()
	stdout = string(normalizeNewlines(output))
	stderr = string(normalizeNewlines(errBuf.Bytes()))
	return stdout, stderr, err
}

func runAWKs(t *testing.T, testArgs []string, testStdin, testOutput, testError string) {
	cmd := exec.Command(awkExe, testArgs...)
	if testStdin != "" {
		cmd.Stdin = strings.NewReader(testStdin)
	}
	errBuf := &bytes.Buffer{}
	cmd.Stderr = errBuf
	output, err := cmd.Output()
	if err != nil {
		if testError == "" {
			t.Fatalf("expected no error, got AWK error: %v (%s)", err, errBuf.String())
		}
	} else {
		if testError != "" {
			t.Fatalf("expected AWK error, got none")
		}
	}
	stdout := string(normalizeNewlines(output))
	if stdout != testOutput {
		t.Fatalf("expected AWK to give %q, got %q", testOutput, stdout)
	}

	stdout, stderr, err := runGoAWK(testArgs, testStdin)
	if err != nil {
		stderr = strings.TrimSpace(stderr)
		if stderr != testError {
			t.Fatalf("expected GoAWK error %q, got %q", testError, stderr)
		}
	} else {
		if testError != "" {
			t.Fatalf("expected GoAWK error %q, got none", testError)
		}
	}
	if stdout != testOutput {
		t.Fatalf("expected GoAWK to give %q, got %q", testOutput, stdout)
	}
}

func TestWildcards(t *testing.T) {
	if runtime.GOOS != "windows" {
		// Wildcards shouldn't be expanded on non-Windows systems, and a file
		// literally named "*.go" doesn't exist, so expect a failure.
		_, stderr, err := runGoAWK([]string{"FNR==1 { print FILENAME }", "testdata/wildcards/*.txt"}, "")
		if err == nil {
			t.Fatal("expected error using wildcards on non-Windows system")
		}
		expected := "file \"testdata/wildcards/*.txt\" not found\n"
		if stderr != expected {
			t.Fatalf("expected %q, got %q", expected, stderr)
		}
		return
	}

	tests := []struct {
		args   []string
		output string
	}{
		{
			[]string{"FNR==1 { print FILENAME }", "testdata/wildcards/*.txt"},
			"testdata/wildcards/one.txt\ntestdata/wildcards/two.txt\n",
		},
		{
			[]string{"-f", "testdata/wildcards/*.awk", "testdata/wildcards/one.txt"},
			"testdata/wildcards/one.txt\nbee\n",
		},
		{
			[]string{"-f", "testdata/wildcards/*.awk", "testdata/wildcards/*.txt"},
			"testdata/wildcards/one.txt\nbee\ntestdata/wildcards/two.txt\nbee\n",
		},
	}

	for _, test := range tests {
		testName := strings.Join(test.args, " ")
		t.Run(testName, func(t *testing.T) {
			stdout, stderr, err := runGoAWK(test.args, "")
			if err != nil {
				t.Fatalf("expected no error, got %v (%q)", err, stderr)
			}
			stdout = strings.Replace(stdout, "\\", "/", -1)
			if stdout != test.output {
				t.Fatalf("expected %q, got %q", test.output, stdout)
			}
		})
	}
}

func TestFILENAME(t *testing.T) {
	origGoAWKExe := goAWKExe
	goAWKExe = "../../" + goAWKExe
	defer func() { goAWKExe = origGoAWKExe }()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chdir("testdata/filename")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	src := `
BEGIN { FILENAME = "10"; print(FILENAME, FILENAME<2) }
BEGIN { FILENAME = 10; print(FILENAME, FILENAME<2) }
{ print(FILENAME, FILENAME<2) }
`
	runAWKs(t, []string{src, "10", "10x"}, "", "10 1\n10 0\n10 0\n10x 1\n", "")
}

func normalizeNewlines(b []byte) []byte {
	return bytes.Replace(b, []byte("\r\n"), []byte{'\n'}, -1)
}

func TestInputOutputMode(t *testing.T) {
	tests := []struct {
		args   []string
		input  string
		output string
		error  string
	}{
		{[]string{"-icsv", "-H", `{ print @"age", @"name" }`}, "name,age\nBob,42\nJane,37", "42 Bob\n37 Jane\n", ""},
		{[]string{"-i", "csv", "-H", `{ print @"age", @"name" }`}, "name,age\nBob,42\nJane,37", "42 Bob\n37 Jane\n", ""},
		{[]string{"-icsv", `{ print $2, $1 }`}, "Bob,42\nJane,37", "42 Bob\n37 Jane\n", ""},
		{[]string{"-i", "csv", `{ print $2, $1 }`}, "Bob,42\nJane,37", "42 Bob\n37 Jane\n", ""},
		{[]string{"-icsv", "-H", "-ocsv", `{ print @"age", @"name" }`}, "name,age\n\"Bo,ba\",42\nJane,37", "42,\"Bo,ba\"\n37,Jane\n", ""},
		{[]string{"-o", "csv", `BEGIN { print "foo,bar", 3.14, "baz" }`}, "", "\"foo,bar\",3.14,baz\n", ""},
		{[]string{"-iabc", `{}`}, "", "", "invalid input mode \"abc\"\n"},
		{[]string{"-oxyz", `{}`}, "", "", "invalid output mode \"xyz\"\n"},
		{[]string{"-H", `{}`}, "", "", "-H only allowed together with -i\n"},
	}

	for _, test := range tests {
		testName := strings.Join(test.args, " ")
		t.Run(testName, func(t *testing.T) {
			stdout, stderr, err := runGoAWK(test.args, test.input)
			if err != nil {
				if test.error == "" {
					t.Fatalf("expected no error, got %v (%q)", err, stderr)
				} else if stderr != test.error {
					t.Fatalf("expected error message %q, got %q", test.error, stderr)
				}
			}
			if stdout != test.output {
				t.Fatalf("expected %q, got %q", test.output, stdout)
			}
		})
	}
}

func TestMultipleCSVFiles(t *testing.T) {
	// Ensure CSV handling works across multiple files with different headers (field names).
	src := `
{
    for (i=1; i in FIELDS; i++) {
        if (i>1)
            printf ",";
        printf "%s", FIELDS[i]
    }
    printf " "
}
{ print @"name", @"age" }
`
	stdout, stderr, err := runGoAWK([]string{"-i", "csv", "-H", src, "testdata/csv/1.csv", "testdata/csv/2.csv"}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v (%q)", err, stderr)
	}
	expected := `
name,age Bob 42
name,age Jill 37
age,email,name Sarah 25
`[1:]
	if stdout != expected {
		t.Fatalf("expected %q, got %q", expected, stdout)
	}
}

func TestCSVDocExamples(t *testing.T) {
	f, err := os.Open("csv.md")
	if err != nil {
		t.Fatalf("error opening examples file: %v", err)
	}
	defer f.Close()

	var (
		command   string
		output    string
		truncated bool
		n         = 1
	)
	runTest := func() {
		t.Run(fmt.Sprintf("Example%d", n), func(t *testing.T) {
			shell := "/bin/sh"
			if runtime.GOOS == "windows" {
				shell = "sh"
			}
			cmd := exec.Command(shell, "-c", command)
			gotBytes, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("error running %q: %v\n%s", command, err, gotBytes)
			}
			got := string(gotBytes)
			if truncated {
				numLines := strings.Count(output, "\n")
				got = strings.Join(strings.Split(got, "\n")[:numLines], "\n") + "\n"
			}
			got = string(normalizeNewlines([]byte(got)))
			if got != output {
				t.Fatalf("error running %q\ngot:\n%s\nexpected:\n%s", command, got, output)
			}
		})
		n++
	}

	scanner := bufio.NewScanner(f)
	inTest := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "$ goawk") {
			if inTest {
				runTest()
			}
			inTest = true
			command = "./" + line[2:]
			output = ""
			truncated = false
		} else if inTest {
			switch line {
			case "```":
				runTest()
				inTest = false
			case "...":
				truncated = true
				runTest()
				inTest = false
			default:
				output += line + "\n"
			}
		}
	}
	if scanner.Err() != nil {
		t.Errorf("error reading input: %v", scanner.Err())
	}
	if inTest {
		t.Error("unexpectedly in test at end of file")
	}
}
