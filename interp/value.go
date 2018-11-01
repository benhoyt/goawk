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
	typeNil valueType = iota
	typeStr
	typeNum
)

// An AWK value (these are passed around by value)
type value struct {
	typ      valueType // Value type
	isNumStr bool      // An AWK "numeric string" from user input
	s        string    // String value (for typeStr)
	n        float64   // Numeric value (for typeNum and numeric strings)
}

// Create a new number value
func num(n float64) value {
	return value{typ: typeNum, n: n}
}

// Create a new string value
func str(s string) value {
	return value{typ: typeStr, s: s}
}

// Create a new value for a "numeric string" context, converting the
// string to a number if possible.
func numStr(s string) value {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return value{typ: typeStr, isNumStr: err == nil, s: s, n: f}
}

// Create a numeric value from a Go bool
func boolean(b bool) value {
	if b {
		return num(1)
	}
	return num(0)
}

// Return true if value is a "true string" (string but not a "numeric
// string")
func (v value) isTrueStr() bool {
	return v.typ == typeStr && !v.isNumStr
}

// Return Go bool value of AWK value. For numbers or numeric strings,
// zero is false and everything else is true. For strings, empty
// string is false and everything else is true.
func (v value) boolean() bool {
	if v.isTrueStr() {
		return v.s != ""
	} else {
		return v.n != 0
	}
}

// Return value's string value, or convert to a string using given
// format if a number value. Integers are a special case and don't
// use floatFormat.
func (v value) str(floatFormat string) string {
	switch v.typ {
	case typeNum:
		if math.IsNaN(v.n) {
			return "nan"
		} else if math.IsInf(v.n, 0) {
			if v.n < 0 {
				return "-inf"
			} else {
				return "inf"
			}
		} else if v.n == float64(int(v.n)) {
			return strconv.Itoa(int(v.n))
		} else {
			return fmt.Sprintf(floatFormat, v.n)
		}
	case typeStr:
		return v.s
	default:
		return ""
	}
}

// Return value's number value, converting from string if necessary
func (v value) num() float64 {
	f, _ := v.numChecked()
	return f
}

// Return value's number value and a success flag, converting from a
// string if necessary
func (v value) numChecked() (float64, bool) {
	switch v.typ {
	case typeNum:
		return v.n, true
	case typeStr:
		if v.isNumStr {
			// If it's a numeric string, we already have the float
			// value from the numStr() call
			return v.n, true
		}
		// Otherwise ensure string starts with a float and convert it
		return parseFloatPrefix(v.s)
	default:
		return 0, true
	}
}

// Like strconv.ParseFloat, but parses at the start of string and
// allows things like "1.5foo"
func parseFloatPrefix(s string) (float64, bool) {
	// Skip whitespace at start
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	start := i

	// Parse mantissa: optional sign, initial digit(s), optional '.',
	// then more digits
	gotDigit := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		gotDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		gotDigit = true
		i++
	}
	if !gotDigit {
		return 0, false
	}

	// Parse exponent ("1e" and similar are allowed, but ParseFloat
	// rejects them)
	end := i
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			end = i
		}
	}

	floatStr := s[start:end]
	f, err := strconv.ParseFloat(floatStr, 64)
	return f, err == nil // May be "value out of range" error
}
