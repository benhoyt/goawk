// +build gofuzz

package fuzz

import (
	"bytes"
	"github.com/benhoyt/goawk/interp"
)

func Fuzz(data []byte) int {
	input := bytes.NewReader([]byte("foo bar\nbaz buz\n"))
	err := interp.Exec(string(data), " ", input, &bytes.Buffer{})
	if err != nil {
		return 0
	}
	return 1
}
