// Tests for ExecuteContext (only present if "goawk_context" build tag is set)

//go:build goawk_context
// +build goawk_context

package interp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func TestExecuteContextNoError(t *testing.T) {
	prog, err := parser.ParseProgram([]byte(`BEGIN {}`), nil)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	interpreter, err := interp.New(prog)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}
	_, err = interpreter.ExecuteContext(context.Background(), nil)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
}

func TestExecuteContextTimeout(t *testing.T) {
	src := `BEGIN { for (i=0; i<1000000000; i++) s+=i }` // would take about 40s on my machine
	prog, err := parser.ParseProgram([]byte(src), nil)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	interpreter, err := interp.New(prog)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err = interpreter.ExecuteContext(ctx, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded error, got: %v", err)
	}
}

func TestExecuteContextCancel(t *testing.T) {
	src := `BEGIN { for (i=0; i<1000000000; i++) s+=i }` // would take about 40s on my machine
	prog, err := parser.ParseProgram([]byte(src), nil)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	interpreter, err := interp.New(prog)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel it right away
	_, err = interpreter.ExecuteContext(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Canceled error, got: %v", err)
	}
}
