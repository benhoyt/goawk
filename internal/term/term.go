// Package term provides the IsTerminal function.
package term

// Code is taken from the github.com/mattn/go-isatty and
// golang.org/x/term packages (which have compatible licenses), but including
// it here avoids pulling in those dependencies. We still need to pull in
// golang.org/x/sys/unix, but that should be very stable.

// IsTerminal reports whether the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	return isTerminal(fd)
}
