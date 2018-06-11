package main

import (
    "fmt"
)

type Value interface{}

type binaryFunc func(l, r Value) Value

var binaryFuncs = map[string]binaryFunc{
    "!=": notEqual,
}

func notEqual(l, r Value) Value {
    switch l := l.(type) {
    case string:
        if r, ok := r.(string); ok {
            if l != r {
                return 1.0
            }
            return 0.0
        }
        return 0.0
    default:
        return 0.0
    }
}

func Evaluate(expr Expr) Value {
    switch e := expr.(type) {
    case *BinaryExpr:
        left := Evaluate(e.Left)
        right := Evaluate(e.Right)
        return binaryFuncs[e.Op](left, right)
    case *NumberExpr:
        return e.Value
    case *StringExpr:
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

func Truthy(value Value) bool {
    switch v := value.(type) {
    case float64:
        return v != 0
    case string:
        return v != ""
    default:
        panic(fmt.Sprintf("unexpected type: %T", value))
    }
}

func Execute(stmt Stmt) {
    switch s := stmt.(type) {
    case *PrintStmt:
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
