//go:build appengine || js || nacl || wasm
// +build appengine js nacl wasm

package term

func isTerminal(fd uintptr) bool {
	return false
}
