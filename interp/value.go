// GoAWK interpreter

package interp

import (
    "fmt"
    "strconv"
)

type Type int

const (
    TypeNil Type = iota
    TypeStr
    TypeNum
)

type Value struct {
    typ      Type
    isNumStr bool
    str      string
    num      float64
}

func Num(n float64) Value {
    return Value{typ: TypeNum, num: n}
}

func Str(s string) Value {
    return Value{typ: TypeStr, str: s}
}

func NumStr(s string) Value {
    // TODO: should use same logic as Value.Float()?
    n, err := strconv.ParseFloat(s, 64)
    return Value{typ: TypeStr, isNumStr: err == nil, str: s, num: n}
}

func Bool(b bool) Value {
    if b {
        return Num(1)
    }
    return Num(0)
}

func (v Value) isTrueStr() bool {
    return v.typ == TypeStr && !v.isNumStr
}

func (v Value) Bool() bool {
    if v.isTrueStr() {
        return v.str != ""
    } else {
        return v.num != 0
    }
}

func (v Value) String(floatFormat string) string {
    switch v.typ {
    case TypeNum:
        if v.num == float64(int(v.num)) {
            return strconv.Itoa(int(v.num))
        } else {
            return fmt.Sprintf(floatFormat, v.num)
        }
    case TypeStr:
        return v.str
    default:
        return ""
    }
}

func (v Value) Float() float64 {
    switch v.typ {
    case TypeNum:
        return v.num
    case TypeStr:
        // TODO: handle cases like "3x"
        f, _ := strconv.ParseFloat(v.str, 64)
        return f
    default:
        return 0
    }
}
