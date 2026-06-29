package interp

// I/O streams are interfaces which allow file redirects and command pipelines to be treated
// equivalently.

import (
	"bufio"
	"errors"
	"io"
)

const (
	notClosedExitCode = -127
)

var (
	errDoubleClose = errors.New("close: stream already closed")
)

// firstError returns the first non-nil error or nil if all errors are nil.
func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

type inputStream interface {
	io.ReadCloser
	ExitCode() int
}

type outputStream interface {
	io.WriteCloser
	Flush() error
	ExitCode() int
}

type outFileStream struct {
	*bufio.Writer
	closer   io.Closer
	exitCode int
	closed   bool
}

func newOutFileStream(wc io.WriteCloser, size int) outputStream {
	b := bufio.NewWriterSize(wc, size)
	return &outFileStream{b, wc, notClosedExitCode, false}
}

func (s *outFileStream) Close() error {
	if s.closed {
		return errDoubleClose
	}
	s.closed = true
	flushErr := s.Writer.Flush()
	closeErr := s.closer.Close()
	if err := firstError(flushErr, closeErr); err != nil {
		s.exitCode = -1
		return err
	}
	s.exitCode = 0
	return nil
}

func (s *outFileStream) ExitCode() int {
	return s.exitCode
}

type outCmdStream struct {
	*bufio.Writer
	closer   io.Closer
	cmd      Cmd
	exitCode int
	closed   bool
}

func newOutCmdStream(cmd Cmd) (outputStream, error) {
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, newError("error connecting to stdin pipe: %v", err)
	}
	err = cmd.Start()
	if err != nil {
		_ = w.Close()
		return nil, err
	}
	out := &outCmdStream{bufio.NewWriterSize(w, outputBufSize), w, cmd, notClosedExitCode, false}
	return out, nil
}

func (s *outCmdStream) Close() error {
	if s.closed {
		return errDoubleClose
	}
	s.closed = true
	flushErr := s.Writer.Flush()
	closeErr := s.closer.Close()
	var waitErr error
	s.exitCode, waitErr = s.cmd.WaitExitCode()
	return firstError(waitErr, flushErr, closeErr)
}

func (s *outCmdStream) ExitCode() int {
	return s.exitCode
}

// An outNullStream allows writes to not do anything while fulfilling the outputStream interface.
type outNullStream struct {
	io.Writer
	closed bool
}

func newOutNullStream() outputStream { return &outNullStream{io.Discard, false} }
func (s outNullStream) Flush() error { return nil }
func (s *outNullStream) Close() error {
	if s.closed {
		return errDoubleClose
	}
	s.closed = true
	return nil
}
func (s outNullStream) ExitCode() int { return -1 }

type inFileStream struct {
	io.ReadCloser
	exitCode int
	closed   bool
}

func newInFileStream(rc io.ReadCloser) inputStream {
	return &inFileStream{rc, notClosedExitCode, false}
}

func (s *inFileStream) Close() error {
	if s.closed {
		return errDoubleClose
	}
	s.closed = true
	if err := s.ReadCloser.Close(); err != nil {
		s.exitCode = -1
		return err
	}
	s.exitCode = 0
	return nil
}

func (s *inFileStream) ExitCode() int {
	return s.exitCode
}

type inCmdStream struct {
	io.ReadCloser
	cmd      Cmd
	exitCode int
	closed   bool
}

func newInCmdStream(cmd Cmd) (inputStream, error) {
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, newError("error connecting to stdout pipe: %v", err)
	}
	err = cmd.Start()
	if err != nil {
		_ = r.Close()
		return nil, err
	}
	return &inCmdStream{r, cmd, notClosedExitCode, false}, nil
}

func (s *inCmdStream) Close() error {
	if s.closed {
		return errDoubleClose
	}
	s.closed = true
	closeErr := s.ReadCloser.Close()
	var waitErr error
	s.exitCode, waitErr = s.cmd.WaitExitCode()
	return firstError(waitErr, closeErr)
}

func (s *inCmdStream) ExitCode() int {
	return s.exitCode
}
