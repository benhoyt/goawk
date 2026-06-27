//go:build go1.24

package interp_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
)

func TestOpenFileRoot(t *testing.T) {
	source := `BEGIN { print "Hello, GoAWK!" > "output.txt" }`
	interpreter := newInterp(t, source)

	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("error opening an *os.Root: %v", err)
	}
	t.Cleanup(func() {
		err := root.Close()
		if err != nil {
			t.Fatalf("error closing root: %v", err)
		}
	})

	var output bytes.Buffer
	status, err := interpreter.Execute(&interp.Config{
		Stdin:  strings.NewReader(""),
		Output: &output,
		OpenFileFunc: func(name string, flag int, perm os.FileMode) (*os.File, error) {
			return root.OpenFile(name, flag, perm)
		},
	})
	if err != nil {
		t.Fatalf("error executing: %v", err)
	}
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if output.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", output.String())
	}

	f, err := root.Open("output.txt")
	if err != nil {
		t.Fatalf("error opening file in root: %v", err)
	}
	t.Cleanup(func() {
		err := f.Close()
		if err != nil {
			t.Fatalf("error closing file in root: %v", err)
		}
	})
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("error reading file in root: %v", err)
	}
	const expected = "Hello, GoAWK!\n"
	normalized := normalizeNewlines(string(data))
	if normalized != expected {
		t.Fatalf("expected file content %q, got %q", expected, normalized)
	}
}

func TestOpenFileCustom(t *testing.T) {
	source := `BEGIN { print "Hello, GoAWK!" > "output.txt" }`
	interpreter := newInterp(t, source)

	var output bytes.Buffer
	status, err := interpreter.Execute(&interp.Config{
		Stdin:  strings.NewReader(""),
		Output: &output,
		OpenFileFunc: func(name string, _ int, _ os.FileMode) (*os.File, error) {
			return nil, fmt.Errorf("can't open %s for writing: read only filesystem", name)
		},
	})

	const expectedErr = `output redirection error: can't open output.txt for writing: read only filesystem`
	if err == nil || err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if output.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", output.String())
	}
}
