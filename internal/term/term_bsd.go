//go:build (darwin || freebsd || openbsd || netbsd || dragonfly) && !appengine
// +build darwin freebsd openbsd netbsd dragonfly
// +build !appengine

package term

import "golang.org/x/sys/unix"

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	return err == nil
}
