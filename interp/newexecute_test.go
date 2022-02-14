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
	source := `{ print NR, x, y, a["k"], $1, $3; x++; y++; a["k"]++ }`
	program, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		t.Fatalf("error parsing: %v", err)
	}

	interpreter, err := interp.New(program)
	if err != nil {
		t.Fatalf("error creating interpreter: %v", err)
	}
	var output bytes.Buffer

	tests := []struct {
		input  string
		vars   []string
		output string
	}{
		{"one two three\nfour five six\n", nil, "1    one three\n2 1 1 1 four six\n"},
		{"ONE TWO THREE\nFOUR FIVE SIX\n", []string{"x", "10"}, "1 10   ONE THREE\n2 11 1 1 FOUR SIX\n"},
		{"1 2 3\n4 5 6\n", []string{"x", "100"}, "1 100   1 3\n2 101 1 1 4 6\n"},
	}
	for i, test := range tests {
		output.Reset()
		status, err := interpreter.Execute(&interp.Config{
			Stdin:  strings.NewReader(test.input),
			Output: &output,
			Vars:   test.vars,
		})
		if err != nil {
			t.Fatalf("%d: error executing: %v", i, err)
		}
		if status != 0 {
			t.Fatalf("%d: expected status 0, got %d", i, status)
		}
		normalized := normalizeNewlines(output.String())
		if normalized != test.output {
			t.Fatalf("%d: expected %q, got %q", i, test.output, normalized)
		}
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
