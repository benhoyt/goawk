// GoAWK tests

package main_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

var (
	testsDir   string
	outputDir  string
	awkExe     string
	goAWKExe   string
	writeAWK   bool
	writeGoAWK bool
)

func TestMain(m *testing.M) {
	flag.StringVar(&testsDir, "testsdir", "./testdata", "directory with one-true-awk tests")
	flag.StringVar(&outputDir, "outputdir", "./testdata/output", "directory for test output")
	flag.StringVar(&awkExe, "awk", "gawk", "awk executable name")
	flag.StringVar(&goAWKExe, "goawk", "./goawk", "goawk executable name")
	flag.BoolVar(&writeAWK, "writeawk", false, "write expected output")
	flag.BoolVar(&writeGoAWK, "writegoawk", true, "write Go AWK output")
	flag.Parse()
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
		"t.in2":     true,
		"t.intest2": true,
	}
	dontRunOnWindows := map[string]bool{
		"p.50":      true, // because this pipes to Unix sort "sort -t: +0 -1 +2nr"
		"t.printf2": true, // TODO: until we fix discrepancies here
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
		Error:  errBuf,
		Args:   []string{inputPath},
	}
	_, err := interp.ExecProgram(prog, config)
	result := outBuf.Bytes()
	result = append(result, errBuf.Bytes()...)
	return result, err
}

func sortedLines(data []byte) []byte {
	trimmed := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(trimmed, "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

func TestCommandLine(t *testing.T) {
	tests := []struct {
		args   []string
		stdin  string
		output string
	}{
		// Load source from stdin
		{[]string{"-f", "-"}, `BEGIN { print "b" }`, "b\n"},
		{[]string{"-f", "-", "-f", "-"}, `BEGIN { print "b" }`, "b\n"},

		// Program with no input
		{[]string{`BEGIN { print "a" }`}, "", "a\n"},

		// Read input from stdin
		{[]string{`$0`}, "one\n\nthree", "one\nthree\n"},
		{[]string{`$0`, "-"}, "one\n\nthree", "one\nthree\n"},
		{[]string{`$0`, "-", "-"}, "one\n\nthree", "one\nthree\n"},

		// Read input from file(s)
		{[]string{`$0`, "testdata/g.1"}, "", "ONE\n"},
		{[]string{`$0`, "testdata/g.1", "testdata/g.2"}, "", "ONE\nTWO\n"},
		{[]string{`{ print FILENAME ":" FNR "/" NR ": " $0 }`, "testdata/g.1", "testdata/g.4"}, "",
			"testdata/g.1:1/1: ONE\ntestdata/g.4:1/2: FOUR a\ntestdata/g.4:2/3: FOUR b\n"},
		{[]string{`$0`, "testdata/g.1", "-", "testdata/g.2"}, "STDIN", "ONE\nSTDIN\nTWO\n"},
		{[]string{`$0`, "testdata/g.1", "-", "testdata/g.2", "-"}, "STDIN", "ONE\nSTDIN\nTWO\n"},

		// Specifying field separator with -F
		{[]string{`{ print $1, $3 }`}, "1 2 3\n4 5 6", "1 3\n4 6\n"},
		{[]string{"-F", ",", `{ print $1, $3 }`}, "1 2 3\n4 5 6", "1 2 3 \n4 5 6 \n"},
		{[]string{"-F", ",", `{ print $1, $3 }`}, "1,2,3\n4,5,6", "1 3\n4 6\n"},
		{[]string{"-F", ",", `{ print $1, $3 }`}, "1,2,3\n4,5,6", "1 3\n4 6\n"},

		// Assigning other variables with -v
		{[]string{"-v", "OFS=.", `{ print $1, $3 }`}, "1 2 3\n4 5 6", "1.3\n4.6\n"},
		{[]string{"-v", "OFS=.", "-v", "ORS=", `{ print $1, $3 }`}, "1 2 3\n4 5 6", "1.34.6"},
		{[]string{"-v", "x=42", "-v", "y=foo", `BEGIN { print x, y }`}, "", "42 foo\n"},
		{[]string{"-v", "RS=;", `$0`}, "a b;c\nd;e", "a b\nc\nd\ne\n"},

		// ARGV/ARGC handling
		{[]string{`
			BEGIN {
				for (i=1; i<ARGC; i++) {
					print i, ARGV[i]
				}
			}`, "a", "b"}, "", "1 a\n2 b\n"},
		{[]string{`
			BEGIN {
				for (i=1; i<ARGC; i++) {
					print i, ARGV[i]
					delete ARGV[i]
				}
			}
			$0`, "a", "b"}, "c\nd", "1 a\n2 b\nc\nd\n"},
		{[]string{`
			BEGIN {
				ARGV[1] = ""
			}
			$0`, "testdata/g.1", "-", "testdata/g.2"}, "c\nd", "c\nd\nTWO\n"},
		{[]string{`
			BEGIN {
				ARGC = 3
			}
			$0`, "testdata/g.1", "-", "testdata/g.2"}, "c\nd", "ONE\nc\nd\n"},
		{[]string{"-v", "A=1", "-f", "testdata/g.3", "B=2", "testdata/test.countries"}, "",
			"A=1, B=0\n\tARGV[1] = B=2\n\tARGV[2] = testdata/test.countries\nA=1, B=2\n"},
		{[]string{`END { print (x==42) }`, "x=42.0"}, "", "1\n"},
		{[]string{"-v", "x=42.0", `BEGIN { print (x==42) }`}, "", "1\n"},
	}
	for _, test := range tests {
		testName := strings.Join(test.args, " ")
		t.Run(testName, func(t *testing.T) {
			cmd := exec.Command(awkExe, test.args...)
			if test.stdin != "" {
				cmd.Stdin = bytes.NewReader([]byte(test.stdin))
			}
			stderr := &bytes.Buffer{}
			cmd.Stderr = stderr
			output, err := cmd.Output()
			if err != nil {
				t.Fatalf("error running %s: %v: %s", awkExe, err, stderr.String())
			}
			output = normalizeNewlines(output)
			if string(output) != test.output {
				t.Fatalf("expected AWK to give %q, got %q", test.output, output)
			}

			cmd = exec.Command(goAWKExe, test.args...)
			if test.stdin != "" {
				cmd.Stdin = bytes.NewReader([]byte(test.stdin))
			}
			stderr = &bytes.Buffer{}
			cmd.Stderr = stderr
			output, err = cmd.Output()
			if err != nil {
				t.Fatalf("error running %s: %v: %s", goAWKExe, err, stderr.String())
			}
			output = normalizeNewlines(output)
			if string(output) != test.output {
				t.Fatalf("expected GoAWK to give %q, got %q", test.output, output)
			}
		})
	}
}

func normalizeNewlines(b []byte) []byte {
	return bytes.Replace(b, []byte("\r\n"), []byte{'\n'}, -1)
}
