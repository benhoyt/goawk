// GoAWK tests

package main_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

var (
	testsDir  string
	outputDir string
)

func TestMain(m *testing.M) {
	flag.StringVar(&testsDir, "testsdir", "./tests", "directory with one-true-awk tests")
	flag.StringVar(&outputDir, "outputdir", "./tests/output", "directory for test output")
	flag.Parse()
	os.Exit(m.Run())
}

func TestAgainstOneTrueAWK(t *testing.T) {
	inputByPrefix := map[string]string{
		"t": "test.data",
		"p": "test.countries",
	}
	nonzeroExits := map[string]bool{
		"t.exit":  true,
		"t.exit1": true,
	}

	infos, err := ioutil.ReadDir(testsDir)
	if err != nil {
		t.Fatalf("couldn't read test files: %v", err)
	}
	for _, info := range infos {
		if !strings.HasPrefix(info.Name(), "t.") && !strings.HasPrefix(info.Name(), "p.") {
			continue
		}
		t.Run(info.Name(), func(t *testing.T) {
			srcPath := filepath.Join(testsDir, info.Name())
			inputPath := filepath.Join(testsDir, inputByPrefix[info.Name()[:1]])
			outputPath := filepath.Join(outputDir, info.Name())
			cmd := exec.Command("awk", "-f", srcPath, inputPath)
			expected, err := cmd.Output()
			if err != nil && !nonzeroExits[info.Name()] {
				t.Fatalf("error running awk: %v", err)
			}
			err = ioutil.WriteFile(outputPath, expected, 0644)
			if err != nil {
				t.Fatalf("error writing awk output: %v", err)
			}
			// output, err := executeGoAWK(srcPath, inputPath)
			// if err != nil {
			// 	t.Fatal(err)
			// } else if string(output) != string(expected) {
			// 	t.Fatalf("got first block instead of second (expected):\n%s---\n%s", output, expected)
			// }
		})
	}

	_ = os.Remove("tempbig")
	_ = os.Remove("tempsmall")
}

func executeGoAWK(srcPath, inputPath string) ([]byte, error) {
	src, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	prog, err := parser.ParseProgram(src)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	p := interp.New(buf)
	err = p.ExecBegin(prog)
	if err != nil {
		return nil, err
	}
	f, errOpen := os.Open(inputPath)
	if errOpen != nil {
		return nil, errOpen
	}
	err = p.ExecFile(prog, inputPath, f)
	f.Close()
	if err != nil && err != interp.ErrExit {
		return nil, err
	}
	err = p.ExecEnd(prog)
	if err != nil && err != interp.ErrExit {
		return nil, err
	}

	return buf.Bytes(), nil
}
