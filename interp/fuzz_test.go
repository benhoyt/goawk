// Fuzz tests for the "go test -fuzz" beta: https://go.dev/blog/fuzz-beta
//
// These are extremely simple right now, and really only test for parser and
// interpreter panics.

//go:build gofuzzbeta
// +build gofuzzbeta

package interp_test

import (
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func isFuzzTest(test interpTest) bool {
	return test.err == "" && test.awkErr == "" && !strings.Contains(test.src, "!fuzz")
}

func execForFuzz(src, in string) {
	prog, err := parser.ParseProgram([]byte(src), nil)
	if err != nil {
		return
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
	_, _ = interp.ExecProgram(prog, config)
}

func FuzzSource(f *testing.F) {
	for _, test := range interpTests {
		if isFuzzTest(test) {
			f.Add(test.src)
		}
	}
	f.Fuzz(func(t *testing.T, src string) {
		execForFuzz(src, "")
	})
}

func FuzzBoth(f *testing.F) {
	for _, test := range interpTests {
		if isFuzzTest(test) {
			f.Add(test.src, test.in)
		}
	}
	f.Fuzz(func(t *testing.T, src, in string) {
		execForFuzz(src, in)
	})
}
