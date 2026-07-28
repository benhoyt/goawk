//go:build go1.24

package interp_test

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

// rootFS adapts an [*os.Root] into an [fs.FS] that also implements
// [interp.WriteFS], confining all access to the root tree.
type rootFS struct {
	Root *os.Root
}

func (r rootFS) Open(name string) (fs.File, error) {
	return r.Root.Open(name)
}

func (r rootFS) Create(name string) (io.WriteCloser, error) {
	return r.Root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
}

func (r rootFS) Append(name string) (io.WriteCloser, error) {
	return r.Root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
}

func TestFileSystemRoot(t *testing.T) {
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

	runProgram := func(source string) (output *bytes.Buffer, err error) {
		prog, err := parser.ParseProgram([]byte(source), nil)
		if err != nil {
			t.Fatalf("error parsing: %v", err)
		}
		output = new(bytes.Buffer)
		config := interp.Config{
			Stdin:      strings.NewReader(""),
			Output:     output,
			Error:      io.Discard,
			FileSystem: rootFS{Root: root},
		}
		status, err := interp.ExecProgram(prog, &config)
		if status != 0 {
			t.Fatalf("expected status 0, got %d", status)
		}
		return output, err
	}

	t.Run("success", func(t *testing.T) {
		output, err := runProgram(`BEGIN { print "Hello, GoAWK!" > "output.txt" }`)
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
		output, err := runProgram(`BEGIN { print "Hello, GoAWK!" > "../../etc/passwd" }`)
		const expectedErr = `path escapes from parent`
		if err == nil {
			t.Fatalf("expected error contains %q, got <nil>", expectedErr)
		} else if !strings.Contains(err.Error(), expectedErr) {
			t.Fatalf("expected error contains %q, got %q", expectedErr, err.Error())
		}
		if output.Len() != 0 {
			t.Fatalf("expected empty stdout, got %q", output.String())
		}
	})
}
