// Tests for the New...Execute API.

package interp_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

// This definitely doesn't test that everything was reset, but it's a good start.
func TestNewExecute(t *testing.T) {
	source := `{ print NR, OFMT, x, y, a["k"], $1, $3; OFMT="%g"; x++; y++; a["k"]++ }`
	interpreter := newInterp(t, source)

	// First execution.
	var output bytes.Buffer
	status, err := interpreter.Execute(&interp.Config{
		Stdin:  strings.NewReader("one two three\nfour five six\n"),
		Output: &output,
	})
	if err != nil {
		t.Fatalf("error executing: %v", err)
	}
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	normalized := normalizeNewlines(output.String())
	expected := "1 %.6g    one three\n2 %g 1 1 1 four six\n"
	if normalized != expected {
		t.Fatalf("expected %q, got %q", expected, normalized)
	}

	// Second execution, with ResetVars.
	output.Reset()
	interpreter.ResetVars()
	status, err = interpreter.Execute(&interp.Config{
		Stdin:  strings.NewReader("ONE TWO THREE\nFOUR FIVE SIX\n"),
		Output: &output,
		Vars:   []string{"x", "10"},
	})
	if err != nil {
		t.Fatalf("error executing: %v", err)
	}
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	normalized = normalizeNewlines(output.String())
	expected = "1 %.6g 10   ONE THREE\n2 %g 11 1 1 FOUR SIX\n"
	if normalized != expected {
		t.Fatalf("expected %q, got %q", expected, normalized)
	}

	// Third execution, without ResetVars.
	output.Reset()
	status, err = interpreter.Execute(&interp.Config{
		Stdin:  strings.NewReader("1 2 3\n4 5 6\n"),
		Output: &output,
		Vars:   []string{"x", "100"},
	})
	if err != nil {
		t.Fatalf("error executing: %v", err)
	}
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	normalized = normalizeNewlines(output.String())
	expected = "1 %g 100 2 2 1 3\n2 %g 101 3 3 4 6\n"
	if normalized != expected {
		t.Fatalf("expected %q, got %q", expected, normalized)
	}
}

func TestResetRand(t *testing.T) {
	source := `BEGIN { print rand(), rand(), rand() }`
	interpreter := newInterp(t, source)
	var output bytes.Buffer

	_, err := interpreter.Execute(&interp.Config{Output: &output})
	if err != nil {
		t.Fatalf("error executing: %v", err)
	}
	original := output.String()

	output.Reset()
	_, err = interpreter.Execute(&interp.Config{Output: &output})
	if err != nil {
		t.Fatalf("error executing: %v", err)
	}
	noResetRand := output.String()
	if original == noResetRand {
		t.Fatalf("expected different random numbers, got %q both times", original)
	}

	output.Reset()
	interpreter.ResetRand()
	_, err = interpreter.Execute(&interp.Config{Output: &output})
	if err != nil {
		t.Fatalf("error executing: %v", err)
	}
	withResetRand := output.String()
	if original != withResetRand {
		t.Fatalf("expected same random numbers (%q) as original (%q)", withResetRand, original)
	}
}

func TestGetArrayValue(t *testing.T) {
	interpreter := newInterp(t, `
BEGIN { Arr["key"]; f(); g(Arr) }
function f() { Arr["hello"]="world" }
function g(arr) { arr["a"]=1.23 }`)
	_, err := interpreter.Execute(nil)
	if err != nil {
		t.Fatalf("error executing: %v", err)
	}
	arr := interpreter.Array("Arr")
	if len(arr) != 3 {
		t.Errorf("expected length 3, got %d", len(arr))
	}
	if arr["key"] != "" {
		t.Errorf("expected value \"\", got %q", arr["key"])
	}
	if arr["hello"] != "world" {
		t.Errorf("expected value \"world\", got %q", arr["hello"])
	}
	if math.Abs(arr["a"].(float64)-1.23) > 1e-9 {
		t.Errorf("expected value 1.23, got %f", arr["a"])
	}
	if interpreter.Array("NonExistent") != nil {
		t.Errorf("non existent name must resolve to nil")
	}
}

func TestExecuteContextNoError(t *testing.T) {
	interpreter := newInterp(t, `BEGIN {}`)
	_, err := interpreter.ExecuteContext(context.Background(), nil)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
}

func TestExecuteContextTimeout(t *testing.T) {
	interpreter := newInterp(t, `BEGIN { for (i=0; i<100000000; i++) s+=i }`) // would take about 4s
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err := interpreter.ExecuteContext(ctx, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded error, got: %v", err)
	}
}

func TestExecuteContextCancel(t *testing.T) {
	interpreter := newInterp(t, `BEGIN { for (i=0; i<100000000; i++) s+=i }`) // would take about 4s
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel it right away
	_, err := interpreter.ExecuteContext(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Canceled error, got: %v", err)
	}
}

func TestExecuteContextSystemTimeout(t *testing.T) {
	start := time.Now()
	interpreter := newInterp(t, `BEGIN { print system("sleep 1") }`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	interpreter.ExecuteContext(ctx, nil)

	// The reason we don't take the err from ExecuteContext is the following:
	//  - os/exec: CommandContext does not forward the context's error on timeout
	//    see: https://github.com/golang/go/issues/21880
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded error, got: %v", ctx.Err())
	}

	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("should have taken ~5ms, took %v", elapsed)
	}
}

func newInterp(t *testing.T, src string) *interp.Interpreter {
	t.Helper()
	prog, err := parser.ParseProgram([]byte(src), nil)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	interpreter, err := interp.New(prog)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}
	return interpreter
}

func TestOpenFileCustom(t *testing.T) {
	openFile := func(name string, flags int, mode os.FileMode) (*os.File, error) {
		accMode := flags & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR)
		if accMode == os.O_WRONLY || accMode == os.O_RDWR {
			return nil, fmt.Errorf("can't open %s for writing: read only filesystem", name)
		}
		f, err := os.OpenFile(name, flags, mode)
		return f, err
	}

	t.Run("cannot write", func(t *testing.T) {
		source := `BEGIN {
		print "Hello, GoAWK!" > "output.txt"
		close("output.txt")
		}`
		interpreter := newInterp(t, source)

		var output bytes.Buffer
		status, err := interpreter.Execute(&interp.Config{
			Stdin:    strings.NewReader(""),
			Output:   &output,
			OpenFile: openFile,
		})

		const expectedErr = `output redirection error: can't open output.txt for writing: read only filesystem`
		if err == nil {
			t.Fatalf("expected error contains %q, got <nil>", expectedErr)
		} else if !strings.Contains(err.Error(), expectedErr) {
			t.Fatalf("expected error contains %q, got %q", expectedErr, err.Error())
		}
		if status != 0 {
			t.Fatalf("expected status 0, got %d", status)
		}
		if output.Len() != 0 {
			t.Fatalf("expected empty stdout, got %q", output.String())
		}
	})

	t.Run("can read", func(t *testing.T) {
		source := `BEGIN {
		n = 0
		while ((getline < "newexecute_test.go") > 0) n++
		close("newexecute_test.go")
		print n
		}`
		interpreter := newInterp(t, source)

		var output bytes.Buffer
		status, err := interpreter.Execute(&interp.Config{
			Stdin:    strings.NewReader(""),
			Output:   &output,
			OpenFile: openFile,
		})
		if err != nil {
			t.Fatalf("error executing: %v", err)
		}
		if status != 0 {
			t.Fatalf("expected status 0, got %d", status)
		}

		const pattern = `^[0-9]+\n`
		normalized := normalizeNewlines(output.String())
		matched, err := regexp.MatchString(pattern, normalized)
		if err != nil {
			t.Fatalf("error with pattern: %v", err)
		}

		if !matched {
			t.Fatalf("expected output matching %q: got %q", pattern, normalized)
		}
	})

	t.Run("getline not not found", func(t *testing.T) {
		source := `BEGIN {
		n = 0
		while ((getline < "does_not_exists.go") > 0) n++
		close("does_not_exists.go")
		print n
		}`
		interpreter := newInterp(t, source)

		var output bytes.Buffer
		status, err := interpreter.Execute(&interp.Config{
			Stdin:    strings.NewReader(""),
			Output:   &output,
			OpenFile: openFile,
		})

		if err != nil {
			t.Fatalf("error executing: %v", err)
		}
		if status != 0 {
			t.Fatalf("expected status 0, got %d", status)
		}
		normalized := normalizeNewlines(output.String())
		const expected = "0\n"
		if normalized != expected {
			t.Fatalf("expected stdout %q, got %q", expected, output.String())
		}
	})
}
