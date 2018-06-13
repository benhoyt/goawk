package main

import (
    "fmt"
    "math"
    "strconv"
)

type Value interface{}

func BoolValue(b bool) Value {
    if b {
        return 1.0
    }
    return 0.0
}

func ToBool(v Value) bool {
    switch v := v.(type) {
    case float64:
        return v != 0
    case string:
        return v != ""
    case nil:
        return false
    default:
        panic(fmt.Sprintf("unexpected type converting to bool: %T", v))
    }
}

func ToFloat(v Value) float64 {
    switch v := v.(type) {
    case float64:
        return v
    case string:
        // TODO: handle cases like "3x"
        f, _ := strconv.ParseFloat(v, 64)
        return f
    case nil:
        return 0.0
    default:
        panic(fmt.Sprintf("unexpected type converting to float: %T", v))
    }
}

func ToString(v Value) string {
    switch v := v.(type) {
    case float64:
        // TODO: take output format into account
        return fmt.Sprintf("%v", v)
    case string:
        return v
    case nil:
        return ""
    default:
        panic(fmt.Sprintf("unexpected type converting to string: %T", v))
    }
}

type binaryFunc func(l, r Value) Value

var binaryFuncs = map[string]binaryFunc{
    "==": equal,
    "!=": func(l, r Value) Value {
        return not(equal(l, r))
    },
    "+": func(l, r Value) Value {
        return ToFloat(l) + ToFloat(r)
    },
    "-": func(l, r Value) Value {
        return ToFloat(l) + ToFloat(r)
    },
    "*": func(l, r Value) Value {
        return ToFloat(l) * ToFloat(r)
    },
    "^": func(l, r Value) Value {
        return math.Pow(ToFloat(l), ToFloat(r))
    },
    "/": func(l, r Value) Value {
        rf := ToFloat(r)
        if rf == 0.0 {
            panic("division by zero")
        }
        return ToFloat(l) / rf
    },
    "%": func(l, r Value) Value {
        rf := ToFloat(r)
        if rf == 0.0 {
            panic("division by zero in mod")
        }
        // TODO: integer/float handling?
        return int(ToFloat(l)) % int(rf)
    },
    "": func(l, r Value) Value {
        return ToString(l) + ToString(r)
    },
}

func equal(l, r Value) Value {
    switch l := l.(type) {
    case float64:
        switch r := r.(type) {
        case float64:
            return BoolValue(l == r)
        case string:
            return BoolValue(l == ToFloat(r))
        }
    case string:
        switch r := r.(type) {
        case string:
            return BoolValue(l == r)
        case float64:
            return BoolValue(ToFloat(l) == r)
        }
    }
    // TODO: uninitialized value (nil)
    return 0.0
}

func not(v Value) Value {
    return BoolValue(!ToBool(v))
}

func Evaluate(expr Expr) Value {
    switch e := expr.(type) {
    case *BinaryExpr:
        left := Evaluate(e.Left)
        right := Evaluate(e.Right)
        return binaryFuncs[e.Op](left, right)
    case *ConstExpr:
        return e.Value
    case *FieldExpr:
        index := Evaluate(e.Index)
        if f, ok := index.(float64); ok {
            i := int(f)
            if float64(i) != f {
                panic(fmt.Sprintf("field index not an integer: %v", f))
            }
            if i == 0 {
                return LINE
            }
            if i < 0 || i > len(FIELDS) {
                panic(fmt.Sprintf("field index out of range: %d (%d)", i, len(FIELDS)))
            }
            return FIELDS[i-1]
        }
        panic(fmt.Sprintf("field index not a number: %q", index))
    default:
        panic(fmt.Sprintf("unexpected expr type: %T", expr))
    }
}

func Execute(stmt Stmt) {
    switch s := stmt.(type) {
    case *PrintStmt:
        // TODO: convert to string properly, respecting output format
        // TODO: handle nil (undefined)
        args := make([]interface{}, len(s.Args))
        for i, a := range s.Args {
            args[i] = Evaluate(a)
        }
        fmt.Println(args...)
    default:
        panic(fmt.Sprintf("unexpected stmt type: %T", stmt))
    }
}

func Executes(stmts Stmts) {
    for _, s := range stmts {
        Execute(s)
    }
}
