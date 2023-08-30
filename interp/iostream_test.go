package interp

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	outputStreamBufferSize = 1024
)

func TestStreamDoubleClose(t *testing.T) {
	dir := t.TempDir()
	t.Run("InFile", func(t *testing.T) {
		f, err := os.Create(filepath.Join(dir, "infile"))
		if err != nil {
			t.Fatal(err)
		}
		in := newInFileStream(f)
		checkDoubleClose(t, in)
	})
	t.Run("OutFile", func(t *testing.T) {
		f, err := os.Create(filepath.Join(dir, "outfile"))
		if err != nil {
			t.Fatal(err)
		}
		out := newOutFileStream(f, outputStreamBufferSize)
		checkDoubleClose(t, out)
	})
	t.Run("InCmd", func(t *testing.T) {
		cmd := execDefaultShell("echo close me")
		in, err := newInCmdStream(cmd)
		if err != nil {
			t.Fatal(err)
		}
		checkDoubleClose(t, in)
	})
	t.Run("OutCmd", func(t *testing.T) {
		cmd := execDefaultShell("echo close me")
		out, err := newOutCmdStream(cmd)
		if err != nil {
			t.Fatal(err)
		}
		checkDoubleClose(t, out)
	})
}

func execDefaultShell(scriptlet string) *exec.Cmd {
	cmdline := append(defaultShellCommand, scriptlet)
	return exec.Command(cmdline[0], cmdline[1:]...)
}

type streamCloser interface {
	ExitCode() int
	Close() error
}

func checkDoubleClose(t *testing.T, sc streamCloser) {
	t.Helper()
	if err := sc.Close(); err != nil {
		t.Fatal(err)
	}
	exitCode := sc.ExitCode()
	if err := sc.Close(); !errors.Is(err, doubleCloseError) {
		t.Error("expected stream.Close() to return error on double close")
	}
	if sc.ExitCode() != exitCode {
		t.Error("expected stream.ExitCode() to stay the same")
	}
}
