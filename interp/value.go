// GoAWK interpreter value type (not exported).

package interp

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type valueType uint8

const (
	typeNull valueType = iota
	typeStr
	typeNum
	typeNumStr
)

// An AWK value (these are passed around by value)
type value struct {
	typ valueType // Type of value
	s   string    // String value (for typeStr and typeNumStr)
	n   float64   // Numeric value (for typeNum)
}

// Create a new null value
func null() value {
	return value{}
}

// Create a new number value
func num(n float64) value {
	return value{typ: typeNum, n: n}
}

// Create a new string value
func str(s string) value {
	return value{typ: typeStr, s: s}
}

// Create a new value to represent a "numeric string" from an input field
func numStr(s string) value {
	return value{typ: typeNumStr, s: s}
}

// Create a numeric value from a Go bool
func boolean(b bool) value {
	if b {
		return num(1)
	}
	return num(0)
}

// String returns a string representation of v for debugging.
func (v value) String() string {
	switch v.typ {
	case typeStr:
		return fmt.Sprintf("str(%q)", v.s)
	case typeNum:
		return fmt.Sprintf("num(%s)", v.str("%.6g"))
	case typeNumStr:
		return fmt.Sprintf("numStr(%q)", v.s)
	default:
		return "null()"
	}
}

// Return true if value is a "true string" (a string or a "numeric string"
// from an input field that can't be converted to a number). If false,
// also return the (possibly converted) number.
func (v value) isTrueStr() (float64, bool) {
	switch v.typ {
	case typeStr:
		return 0, true
	case typeNumStr:
		f, err := strconv.ParseFloat(strings.TrimSpace(v.s), 64)
		if err != nil {
			return 0, true
		}
		return f, false
	default: // typeNum, typeNull
		return v.n, false
	}
}

// Return Go bool value of AWK value. For numbers or numeric strings,
// zero is false and everything else is true. For strings, empty
// string is false and everything else is true.
func (v value) boolean() bool {
	switch v.typ {
	case typeStr:
		return v.s != ""
	case typeNumStr:
		f, err := strconv.ParseFloat(strings.TrimSpace(v.s), 64)
		if err != nil {
			return v.s != ""
		}
		return f != 0
	default: // typeNum, typeNull
		return v.n != 0
	}
}

// Return value's string value, or convert to a string using given
// format if a number value. Integers are a special case and don't
// use floatFormat.
func (v value) str(floatFormat string) string {
	if v.typ == typeNum {
		switch {
		case math.IsNaN(v.n):
			return "nan"
		case math.IsInf(v.n, 0):
			if v.n < 0 {
				return "-inf"
			} else {
				return "inf"
			}
		case v.n == float64(int(v.n)):
			return strconv.Itoa(int(v.n))
		default:
			if floatFormat == "%.6g" {
				return strconv.FormatFloat(v.n, 'g', 6, 64)
			}
			return fmt.Sprintf(floatFormat, v.n)
		}
	}
	// For typeStr and typeNumStr we already have the string, for
	// typeNull v.s == "".
	return v.s
}

// Return value's number value, converting from string if necessary
func (v value) num() float64 {
	switch v.typ {
	case typeStr, typeNumStr:
		// Ensure string starts with a float and convert it
		return parseFloatPrefix(v.s)
	default: // typeNum, typeNull
		return v.n
	}
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// Like strconv.ParseFloat, but parses at the start of string and
// allows things like "1.5foo"
func parseFloatPrefix(s string) float64 {
	// Skip whitespace at start
	i := 0
	for i < len(s) && asciiSpace[s[i]] != 0 {
		i++
	}
	start := i

	// Parse optional sign and check for NaN and Inf.
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if i+3 <= len(s) {
		if (s[i] == 'n' || s[i] == 'N') && (s[i+1] == 'a' || s[i+1] == 'A') && (s[i+2] == 'n' || s[i+2] == 'N') {
			return math.NaN()
		}
		if (s[i] == 'i' || s[i] == 'I') && (s[i+1] == 'n' || s[i+1] == 'N') && (s[i+2] == 'f' || s[i+2] == 'F') {
			if s[start] == '-' {
				return math.Inf(-1)
			}
			return math.Inf(1)
		}
	}

	// Parse mantissa: initial digit(s), optional '.', then more digits
	if i+2 < len(s) && s[i] == '0' && (s[i+1] == 'x' || s[i+1] == 'X') {
		return parseHexFloatPrefix(s, start, i+2)
	}
	gotDigit := false
	for i < len(s) && isDigit(s[i]) {
		gotDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
	}
	for i < len(s) && isDigit(s[i]) {
		gotDigit = true
		i++
	}
	if !gotDigit {
		return 0
	}

	// Parse exponent ("1e" and similar are allowed, but ParseFloat
	// rejects them)
	end := i
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		for i < len(s) && isDigit(s[i]) {
			i++
			end = i
		}
	}

	floatStr := s[start:end]
	f, _ := strconv.ParseFloat(floatStr, 64)
	return f // Returns infinity in case of "value out of range" error
}

// Helper used by parseFloatPrefix to handle hexadecimal floating point.
func parseHexFloatPrefix(s string, start, i int) float64 {
	gotDigit := false
	for i < len(s) && isHexDigit(s[i]) {
		gotDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
	}
	for i < len(s) && isHexDigit(s[i]) {
		gotDigit = true
		i++
	}
	if !gotDigit {
		return 0
	}

	gotExponent := false
	end := i
	if i < len(s) && (s[i] == 'p' || s[i] == 'P') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		for i < len(s) && isDigit(s[i]) {
			gotExponent = true
			i++
			end = i
		}
	}

	floatStr := s[start:end]
	if !gotExponent {
		floatStr = floatStr + "p0" // AWK allows "0x12", ParseFloat requires "0x12p0"
	}
	f, _ := strconv.ParseFloat(floatStr, 64)
	return f // Returns infinity in case of "value out of range" error
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
