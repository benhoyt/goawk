// GoAWK interpreter

package interp

import (
	"fmt"
	"strconv"
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
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
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
		var f float64
		_, _ = fmt.Sscanf(v.s, "%f", &f)
		return f
	default:
		return 0
	}
}
