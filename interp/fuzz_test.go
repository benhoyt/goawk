// Fuzz tests for use with the Go 1.18 fuzzer.

//go:build go1.18
// +build go1.18

package interp_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func isFuzzTest(test interpTest) bool {
	return test.err == "" && test.awkErr == "" && !strings.Contains(test.src, "!fuzz")
}

func FuzzSource(f *testing.F) {
	for _, test := range interpTests {
		if isFuzzTest(test) {
			f.Add(test.src)
		}
	}

	f.Fuzz(func(t *testing.T, src string) {
		prog, err := parser.ParseProgram([]byte(src), nil)
		if err != nil {
			return
		}
		interpreter, err := interp.New(prog)
		if err != nil {
			f.Fatalf("interp.New error: %v", err)
		}
		config := interp.Config{
			Stdin:        strings.NewReader("foo bar\nbazz\n"),
			Output:       ioutil.Discard,
			Error:        ioutil.Discard,
			NoExec:       true,
			NoFileWrites: true,
			NoFileReads:  true,
			Environ:      []string{},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, _ = interpreter.ExecuteContext(ctx, &config)
	})
}

func FuzzInput(f *testing.F) {
	f.Add("")
	added := make(map[string]bool)
	for _, test := range interpTests {
		if test.in != "" && !added[test.in] {
			f.Add(test.in)
			added[test.in] = true
		}
	}

	prog, err := parser.ParseProgram([]byte(`{ print $0, $3, $1, $10 }`), nil)
	if err != nil {
		f.Fatalf("parse error: %v", err)
	}

	interpreter, err := interp.New(prog)
	if err != nil {
		f.Fatalf("interp.New error: %v", err)
	}

	var vars = [][]string{
		{"FS", " ", "RS", "\n"},
		{"FS", ",", "RS", "\n"},
		{"FS", "\t", "RS", "\n"},
		{"FS", "@+", "RS", "\n"},
		{"FS", "\n", "RS", ""},
		{"FS", " ", "RS", "X+"},
	}

	f.Fuzz(func(t *testing.T, in string) {
		for _, v := range vars {
			t.Run(fmt.Sprintf("Vars=%q", v), func(t *testing.T) {
				interpreter.ResetVars()
				config := interp.Config{
					Stdin:        strings.NewReader(in),
					Output:       ioutil.Discard,
					Error:        ioutil.Discard,
					Vars:         v,
					NoExec:       true,
					NoFileWrites: true,
					NoFileReads:  true,
					Environ:      []string{},
				}
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
				_, err := interpreter.ExecuteContext(ctx, &config)
				if err != nil {
					t.Fatalf("execute error: %v", err)
				}
			})
		}
	})
}
