//go:build plan9
// +build plan9

package term

import (
	"syscall"
)

func isTerminal(fd uintptr) bool {
	path, err := syscall.Fd2path(int(fd))
	if err != nil {
		return false
	}
	return path == "/dev/cons" || path == "/mnt/term/dev/cons"
}
