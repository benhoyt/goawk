package interp

import (
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// Cmd defines a subset of interface compatible with [exec.Cmd] making it
// possible to mock and replace the implementation. All methods should mimic
// those of [exec.Cmd], while [Cmd.WaitExitCode] should mimic GO AWK conventions.
type Cmd interface {
	// Start begins execution and returns without waiting for completion.
	// It returns an error if the command fails to start.
	Start() error

	// StdinPipe returns a pipe connected to the command's standard input.
	// goawk closes it to signal EOF (`print ... | cmd`). It must be called
	// before Start, and is mutually exclusive with SetStdin.
	StdinPipe() (io.WriteCloser, error)

	// StdoutPipe returns a pipe connected to the command's standard output,
	// which goawk reads (`cmd | getline`). It must be called before Start,
	// and is mutually exclusive with SetStdout. WaitExitCode MUST NOT be
	// called until the pipe has been fully read.
	StdoutPipe() (io.ReadCloser, error)

	// SetStdin sets the command's standard input. Must be called before Start.
	SetStdin(io.Reader)

	// SetStdout sets the command's standard output. Must be called before Start.
	SetStdout(io.Writer)

	// SetStderr sets the command's standard error. Must be called before Start.
	SetStderr(io.Writer)

	// WaitExitCode waits for the command to finish and reports its result,
	// mimicking GNU awk status convention:
	//   - exit status (including non-zero) with a nil error on normal exit;
	//   - 256+signal on an unhandled signal, 512+signal if it dumped core;
	//   - -1 and a non-nil error for any other (I/O or internal) failure.
	// Backends without OS process semantics (no signals/core dumps) just
	// return the command's exit status, or -1 and the error that stopped it.
	WaitExitCode() (int, error)
}

type execCmd struct {
	cmd *exec.Cmd
}

func (e execCmd) Start() error {
	return e.cmd.Start()
}

func (e execCmd) StdinPipe() (io.WriteCloser, error) {
	return e.cmd.StdinPipe()
}

func (e execCmd) StdoutPipe() (io.ReadCloser, error) {
	return e.cmd.StdoutPipe()
}

func (e execCmd) SetStdin(r io.Reader) {
	e.cmd.Stdin = r
}

func (e execCmd) SetStdout(w io.Writer) {
	e.cmd.Stdout = w
}

func (e execCmd) SetStderr(w io.Writer) {
	e.cmd.Stderr = w
}

// WaitExitCode closes the cmd and convert the error result into the result returned from goawk builtin functions.
// A nil error is returned if that error describes a non-zero exit status or an unhandled signal.
// Any other type of error returns -1 and err.
//
// The result mimicks gawk for expected child process errors:
// 1. Returns the exit status of the child process and nil error on normal process exit.
// 2. Returns 256 + signal on unhandled signal exit.
// 3. Returns 512 + signal on unhandled signal exit which caused a core dump.
func (e execCmd) WaitExitCode() (int, error) {
	err := e.cmd.Wait()
	if err == nil {
		return 0, nil
	}
	ee, ok := err.(*exec.ExitError)
	if !ok {
		// Wait() returned an io error.
		return -1, err
	}
	status, ok := ee.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		// Maybe not all platforms support WaitStatus?
		return -1, err
	}
	switch {
	case status.CoreDump():
		return 512 + int(status.Signal()), nil
	case status.Signaled():
		return 256 + int(status.Signal()), nil
	case status.Exited():
		return status.ExitStatus(), nil
	default:
		return -1, err
	}
}

func execCommand(ctx context.Context, name string, args ...string) Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	// Ensure stdout/stderr pipes being held open don't keep process running.
	cmd.WaitDelay = 250 * time.Millisecond
	return execCmd{cmd: cmd}
}
