// GoAWK tests

package main_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

// TODO: don't hard-code these paths
func TestP(t *testing.T) {
	infos, err := ioutil.ReadDir("/Users/ben.hoyt/Home/awk/regdir")
	if err != nil {
		t.Fatalf("couldn't read test files: %v", err)
	}
	for _, info := range infos {
		if !strings.HasPrefix(info.Name(), "p.") {
			continue
		}
		srcPath := "/Users/ben.hoyt/Home/awk/regdir/" + info.Name()
		inputPath := "/Users/ben.hoyt/Home/awk/regdir/test.countries"
		t.Run(info.Name(), func(t *testing.T) {
			cmd := exec.Command("awk", "-f", srcPath, inputPath)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Error(err)
			}
			err = cmd.Start()
			if err != nil {
				t.Error(err)
			}
			expected, err := ioutil.ReadAll(stdout)
			if err != nil {
				t.Error(err)
			}
			err = cmd.Wait()
			if err != nil {
				t.Error(err)
			}

			output, err := execute(srcPath, inputPath)
			if err != nil {
				t.Error(err)
			} else if string(output) != string(expected) {
				t.Errorf("got:\n%s--- expected:\n%s", output, expected)
			}
		})
	}
}

func execute(srcPath, inputPath string) ([]byte, error) {
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
	err = p.ExecFile(prog, srcPath, f)
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
