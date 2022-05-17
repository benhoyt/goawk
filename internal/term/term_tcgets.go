//go:build (linux || aix || zos) && !appengine
// +build linux aix zos
// +build !appengine

package term

import (
	"golang.org/x/sys/unix"
)

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}
