// Tests for the New...Execute API.

package interp_test

import (
	"bytes"
	"context"
	"errors"
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
	interpreter := newInterp(t, `BEGIN { print system("sleep 4") }`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err := interpreter.ExecuteContext(ctx, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded error, got: %v", err)
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
