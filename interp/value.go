// GoAWK interpreter value type (not exported).

package interp

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/scanner"
)

const (
	typeNil = iota
	typeStr
	typeNum
)

type value struct {
	typ      uint8
	isNumStr bool
	s        string
	n        float64
}

func num(n float64) value {
	return value{typ: typeNum, n: n}
}

func str(s string) value {
	return value{typ: typeStr, s: s}
}

func numStr(s string) value {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return value{typ: typeStr, isNumStr: err == nil, s: s, n: f}
}

func boolean(b bool) value {
	if b {
		return num(1)
	}
	return num(0)
}

func (v value) isTrueStr() bool {
	return v.typ == typeStr && !v.isNumStr
}

func (v value) boolean() bool {
	if v.isTrueStr() {
		return v.s != ""
	} else {
		return v.n != 0
	}
}

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

func (v value) num() float64 {
	f, _ := v.numChecked()
	return f
}

func (v value) numChecked() (float64, error) {
	switch v.typ {
	case typeNum:
		return v.n, nil
	case typeStr:
		if v.isNumStr {
			// If it's a numeric string, we already have the float
			// value from the numStr() call
			return v.n, nil
		}
		// TODO: scanner is relatively slow and allocates a bunch, do this by hand
		// Note that converting to number directly (in constrast to
		// "numeric strings") allows things like "1.5foo"
		var scan scanner.Scanner
		scan.Init(strings.NewReader(v.s))
		scan.Error = func(*scanner.Scanner, string) {}
		tok := scan.Scan()
		negative := tok == '-'
		if tok == '-' || tok == '+' {
			tok = scan.Scan()
		}
		if scan.ErrorCount != 0 || (tok != scanner.Float && tok != scanner.Int) {
			return 0, fmt.Errorf("invalid number %q", v.s)
		}
		// Scanner allows trailing 'e', ParseFloat doesn't
		text := scan.TokenText()
		text = strings.TrimRight(text, "eE")
		f, _ := strconv.ParseFloat(text, 64)
		if negative {
			f = -f
		}
		return f, nil
	default:
		return 0, nil
	}
}
