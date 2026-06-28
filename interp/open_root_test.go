//go:build go1.24

package interp_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
)

func TestOpenFileRoot(t *testing.T) {
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

	openFile := func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return root.OpenFile(name, flag, perm)
	}

	t.Run("success", func(t *testing.T) {
		var output bytes.Buffer
		source := `BEGIN { print "Hello, GoAWK!" > "output.txt" }`
		interpreter := newInterp(t, source)
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
		if output.Len() != 0 {
			t.Fatalf("expected empty stdout, got %q", output.String())
		}

		data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
		if err != nil {
			t.Fatalf("error reading file in root: %v", err)
		}
		const expected = "Hello, GoAWK!\n"
		normalized := normalizeNewlines(string(data))
		if normalized != expected {
			t.Fatalf("expected file content %q, got %q", expected, normalized)
		}
	})

	t.Run("path traversal", func(t *testing.T) {
		var output bytes.Buffer
		source := `BEGIN { print "Hello, GoAWK!" > "../../etc/passwd" }`
		interpreter := newInterp(t, source)
		status, err := interpreter.Execute(&interp.Config{
			Stdin:    strings.NewReader(""),
			Output:   &output,
			OpenFile: openFile,
		})
		const expectedErr = `path escapes from parent`
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
}
