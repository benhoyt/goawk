//go:build solaris && !appengine
// +build solaris,!appengine

package term

import (
	"golang.org/x/sys/unix"
)

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermio(int(fd), unix.TCGETA)
	return err == nil
}
