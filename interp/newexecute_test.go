// Tests for the New...Execute API.

package interp_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

// This definitely doesn't test that everything was reset, but it's a good start.
func TestNewExecute(t *testing.T) {
	source := `{ print NR, OFMT, x, y, a["k"], $1, $3; OFMT="%g"; x++; y++; a["k"]++ }`
	program, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		t.Fatalf("error parsing: %v", err)
	}
	interpreter, err := interp.New(program)
	if err != nil {
		t.Fatalf("error creating interpreter: %v", err)
	}

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
	program, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		t.Fatalf("error parsing: %v", err)
	}

	interpreter, err := interp.New(program)
	if err != nil {
		t.Fatalf("error creating interpreter: %v", err)
	}
	var output bytes.Buffer

	_, err = interpreter.Execute(&interp.Config{Output: &output})
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
