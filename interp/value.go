// GoAWK interpreter

package interp

import (
	"fmt"
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
		if v.n == float64(int(v.n)) {
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
	switch v.typ {
	case typeNum:
		return v.n
	case typeStr:
		// Note that converting to number directly (in constrast to
		// "numeric strings") allows things like "1.5foo"
		var scan scanner.Scanner
		scan.Init(strings.NewReader(v.s))
		scan.Error = func(*scanner.Scanner, string) {}
		tok := scan.Scan()
		if tok != scanner.Float && tok != scanner.Int {
			return 0
		}
		text := scan.TokenText()
		// Scanner allows trailing 'e', ParseFloat doesn't
		text = strings.TrimRight(text, "eE")
		f, _ := strconv.ParseFloat(text, 64)
		return f
	default:
		return 0
	}
}
