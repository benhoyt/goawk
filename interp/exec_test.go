package interp_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func TestExecFunc(t *testing.T) {
	isCommand := func(t *testing.T, command string, args []string) bool {
		t.Helper()
		if len(args) == 0 {
			t.Fatalf("isCommand: args empty")
		} else if len(args) == 1 && strings.HasPrefix(args[0], command) {
			return true
		} else if len(args) >= 2 && strings.HasPrefix(args[1], command) {
			return true
		}
		return false
	}

	type want struct {
		name   string
		args   []string
		input  string
		output string
	}
	tests := []struct {
		name         string
		program      string
		shellCommand []string
		execFunc     func(context.Context, string, ...string) *cmd
		wants        []want
	}{
		{
			name:         "system zero exit",
			program:      `BEGIN { print system("echo real-os-exec") }`,
			shellCommand: []string{"/bin/sh", "-c"},
			execFunc: func(ctx context.Context, name string, args ...string) *cmd {
				return newCmd(ctx, name, args...).
					OnStdout("mocked goawk.Cmd")
			},
			wants: []want{
				{
					name:   "/bin/sh",
					args:   []string{"-c", "echo real-os-exec"},
					output: "mocked goawk.Cmd0\n",
				},
			},
		},
		{
			name:         "system nonzero exit surfaces code",
			program:      `BEGIN { print system("false") }`,
			shellCommand: []string{"/bin/sh", "-c"},
			execFunc: func(ctx context.Context, name string, args ...string) *cmd {
				return newCmd(ctx, name, args...).
					OnWaitExitCode(3, nil)
			},
			wants: []want{
				{
					name:   "/bin/sh",
					args:   []string{"-c", "false"},
					output: "3\n",
				},
			},
		},
		{
			name:         "write to pipe",
			program:      `BEGIN { "printf 'pipe\n'" | getline x; print x}`,
			shellCommand: []string{"/bin/sh", "-c"},
			execFunc: func(ctx context.Context, name string, args ...string) *cmd {
				switch {
				case isCommand(t, "printf", args):
					stdout := regexp.MustCompile(`^printf '([^']*)'$`).FindStringSubmatch(args[1])[1]
					return newCmd(ctx, name, args...).
						OnStdout(stdout)
				default:
					panic("execFunc: unsupported command for arguments: " + strings.Join(args, " "))
				}
			},
			wants: []want{
				{
					name:   "/bin/sh",
					args:   []string{"-c", "printf 'pipe\n'"},
					output: "pipe\n",
				},
			},
		},
		{
			name:         "print to command pipe",
			program:      `BEGIN { print "print-from-awk" | "cat" }`,
			shellCommand: []string{"/bin/sh", "-c"},
			execFunc: func(ctx context.Context, name string, args ...string) *cmd {
				switch {
				case isCommand(t, "cat", args):
					return newCmd(ctx, name, args...).
						OnStdout("print-from-cat")
				default:
					panic("execFunc: unsupported command for arguments: " + strings.Join(args, " "))
				}
			},
			wants: []want{
				{
					name:   "/bin/sh",
					args:   []string{"-c", "cat"},
					input:  "print-from-awk\n",
					output: "print-from-cat",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			prog, err := parser.ParseProgram([]byte(tt.program), nil)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			var cmds []*cmd
			var out bytes.Buffer
			config := &interp.Config{
				Output:       &out,
				Error:        &out,
				ShellCommand: tt.shellCommand,
				Command: func(ctx context.Context, name string, args ...string) interp.Cmd {
					cmd := tt.execFunc(ctx, name, args...)
					cmds = append(cmds, cmd)
					return cmd
				},
			}

			if _, err := interp.ExecProgram(prog, config); err != nil {
				t.Fatalf("exec: %v", err)
			}

			if len(cmds) != len(tt.wants) {
				t.Fatalf("unexpected number of called commands: got %d, want %d", len(cmds), len(tt.wants))
			}

			for i, gotCmd := range cmds {
				want := tt.wants[i]
				if gotCmd.name != want.name {
					t.Fatalf("name: got %v, want %v", gotCmd.name, want.name)
				}

				if !reflect.DeepEqual(gotCmd.args, want.args) {
					t.Fatalf("args: got %q, want %q", gotCmd.args, want.args)
				}

				if want.input != "" {
					<-gotCmd.stdinDone
					normalized := normalizeNewlines(gotCmd.stdinBuf.String())
					if normalized != want.input {
						t.Fatalf("input: got %q, want %q", normalized, want.input)
					}
				}

				if want.output != "" {
					normalized := normalizeNewlines(out.String())
					if normalized != want.output {
						t.Fatalf("output: got %q, want %q", normalized, want.output)
					}
				}
			}
		})
	}
}

type cmd struct {
	// captured params
	name string
	args []string

	// mock: Start()
	onStartError error
	startCalled  bool

	// mock: SetStdin()
	stdin     io.Reader
	stdinPipe *io.PipeReader
	stdinBuf  *bytes.Buffer
	stdinDone chan struct{}

	// mock: SetStdout()
	onStdout   string
	stdoutPipe *io.PipeWriter
	stdout     io.Writer

	// mock: SetStderr()
	stderr io.Writer

	// mock: WaitExitCode()
	onWaitExitCodeExitCode int
	onWaitExitCodeErr      error
}

func newCmd(_ context.Context, name string, args ...string) *cmd {
	return &cmd{
		name: name,
		args: args,
	}
}

func (c *cmd) OnStart(err error) *cmd {
	c.onStartError = err
	return c
}

func (c *cmd) Start() error {
	if c.startCalled {
		panic("start: called twice")
	}
	c.startCalled = true

	if c.stdinPipe != nil {
		c.stdinBuf = new(bytes.Buffer)
		c.stdinDone = make(chan struct{})
		go func() {
			defer close(c.stdinDone)
			_, _ = io.Copy(c.stdinBuf, c.stdinPipe)
		}()
	}

	if c.onStdout != "" {
		if c.stdoutPipe != nil {
			go func() {
				defer func() {
					err := c.stdoutPipe.Close()
					if err != nil {
						panic("start: closing stdout pipe: " + err.Error())
					}
				}()
				_, err := io.WriteString(c.stdoutPipe, c.onStdout)
				if err != nil && !errors.Is(err, io.ErrClosedPipe) {
					panic("start: writing to stdout: " + err.Error())
				}
			}()
		} else if c.stdout != nil {
			_, err := io.WriteString(c.stdout, c.onStdout)
			if err != nil {
				panic("start: writing to stdout: " + err.Error())
			}
		} else {
			panic("start: OnStdout() called without SetStdout() or StdoutPipe()")
		}
	}

	return c.onStartError
}

func (c *cmd) StdinPipe() (io.WriteCloser, error) {
	if c.startCalled {
		panic("StdinPipe: Must be called before Start")
	}
	if c.stdinPipe != nil {
		panic("StdinPipe: Called twice")
	}
	if c.stdin != nil {
		panic("StdinPipe: SetStdin() called")
	}
	r, w := io.Pipe()
	c.stdinPipe = r
	return w, nil
}

func (c *cmd) SetStdin(r io.Reader) {
	if c.startCalled {
		panic("SetStdin: Must be called before Start")
	}
	c.stdin = r
}

func (c *cmd) StdoutPipe() (io.ReadCloser, error) {
	if c.startCalled {
		panic("StdoutPipe: Must be called before Start")
	}
	if c.stdoutPipe != nil {
		panic("StdoutPipe: Called twice")
	}
	r, w := io.Pipe()
	c.stdoutPipe = w
	return r, nil
}

func (c *cmd) OnStdout(stdout string) *cmd {
	c.onStdout = stdout
	return c
}

func (c *cmd) SetStdout(w io.Writer) {
	if c.startCalled {
		panic("SetStdout: Must be called before Start")
	}
	c.stdout = w
}
func (c *cmd) SetStderr(w io.Writer) {
	if c.startCalled {
		panic("SetStderr: Must be called before Start")
	}
	c.stderr = w
}

func (c *cmd) OnWaitExitCode(exitCode int, err error) *cmd {
	c.onWaitExitCodeExitCode = exitCode
	c.onWaitExitCodeErr = err
	return c
}
func (c *cmd) WaitExitCode() (int, error) {
	return c.onWaitExitCodeExitCode, c.onWaitExitCodeErr
}
