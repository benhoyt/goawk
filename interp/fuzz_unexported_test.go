// Fuzz tests for unexported functions for use with the Go 1.18 fuzzer.

//go:build go1.18
// +build go1.18

package interp

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func FuzzParseFloatPrefix(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add("The quick.")
	f.Add("0")
	f.Add("9")
	f.Add("1.3e4")
	f.Add("1.3E0")
	f.Add("1.3e+5")
	f.Add("1.3e-5")
	f.Add("1E1000")
	f.Add("    1234    ")
	f.Add("1234xyz")
	f.Add("-1234567890")
	f.Add("0x0")
	f.Add("0X10")
	f.Add("0x1234567890")
	f.Add("0xabcdef")
	f.Add("0xABCDEF")
	f.Add("-0xa")
	f.Add("+0XA")
	f.Add("0xf.f")
	f.Add("0xf.fp10")
	f.Add("0xf.fp-10")
	f.Add("0x.f")
	f.Add("0xf.")
	f.Add("0x.")
	f.Add("nan")
	f.Add("+nan")
	f.Add("-nan")
	f.Add("NAN")
	f.Add("inf")
	f.Add("+inf")
	f.Add("-inf")
	f.Add("INF")

	f.Fuzz(func(t *testing.T, in string) {
		nPrefix := parseFloatPrefix(in)
		if nPrefix != 0 {
			for i := 1; i <= len(in); i++ {
				n, _ := parseFloatHelper(in[:i])
				if n == nPrefix || math.IsNaN(n) && math.IsNaN(nPrefix) {
					return
				}
			}
			t.Fatalf("no ParseFloat match: %q", in)
		}
	})
}

func parseFloatHelper(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	if s == "+nan" || s == "-nan" {
		return math.NaN(), nil
	}
	if strings.Contains(s, "0x") && strings.IndexAny(s, "pP") < 0 {
		s += "p0"
	}
	return strconv.ParseFloat(s, 64)
}
