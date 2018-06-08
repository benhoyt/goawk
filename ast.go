package main

import (
    "fmt"
    "strings"
)

type Program struct {
    Begin Stmts
    Actions []Action
    End Stmts
}

func (p *Program) String() string {
    parts := []string{}
    if len(p.Begin) > 0 {
        parts = append(parts, "BEGIN {\n" + indent(p.Begin.String()) + "\n}")
    }
    for _, a := range p.Actions {
        parts = append(parts, a.String())
    }
    if len(p.End) > 0 {
        parts = append(parts, "END {\n" + indent(p.End.String()) + "\n}")
    }
    return strings.Join(parts, "\n\n")
}

func indent(s string) string {
    input := strings.Split(s, "\n")
    output := make([]string, len(input))
    for i, s := range input {
        output[i] = "    " + s
    }
    return strings.Join(output, "\n")
}

type Stmts []Stmt

func (ss Stmts) String() string {
    lines := make([]string, len(ss))
    for i, s := range ss {
        lines[i] = s.String()
    }
    return strings.Join(lines, "\n")
}

type Action struct {
    Pattern Expr
    Stmts Stmts
}

func (a *Action) String() string {
    return a.Pattern.String() + " {\n" + indent(a.Stmts.String()) + "\n}"
}

type Expr interface {
    expr()
    String() string
}

type DollarExpr struct {
    Index Expr
}

func (e *DollarExpr) expr() {}

func (e *DollarExpr) String() string {
    return "$" + e.Index.String()
}

type BinaryExpr struct {
    Left Expr
    Op string
    Right Expr
}

func (e *BinaryExpr) expr() {}

func (e *BinaryExpr) String() string {
    return "(" + e.Left.String() + " " + e.Op + " " + e.Right.String() + ")"
}

type NumberExpr struct {
    Value float64
}

func (e *NumberExpr) expr() {}

func (e *NumberExpr) String() string {
    return fmt.Sprintf("%v", e.Value)
}

type StringExpr struct {
    Value string
}

func (e *StringExpr) expr() {}

func (e *StringExpr) String() string {
    return fmt.Sprintf("%q", e.Value)
}

type Stmt interface {
    stmt()
    String() string
}

type PrintStmt struct {
    Args []Expr
}

func (s *PrintStmt) stmt() {}

func (s *PrintStmt) String() string {
    parts := make([]string, len(s.Args))
    for i, a := range s.Args {
        parts[i] = a.String()
    }
    return "print " + strings.Join(parts, ", ")
}
