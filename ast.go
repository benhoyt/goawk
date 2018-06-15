package main

import (
	"fmt"
	"strings"
)

type Program struct {
	Begin   []Stmts
	Actions []Action
	End     []Stmts
}

func (p *Program) String() string {
	parts := []string{}
	for _, ss := range p.Begin {
		parts = append(parts, "BEGIN {\n"+indent(ss.String())+"\n}")
	}
	for _, a := range p.Actions {
		parts = append(parts, a.String())
	}
	for _, ss := range p.End {
		parts = append(parts, "END {\n"+indent(ss.String())+"\n}")
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
	Stmts   Stmts
}

func (a *Action) String() string {
	return a.Pattern.String() + " {\n" + indent(a.Stmts.String()) + "\n}"
}

type Expr interface {
	expr()
	String() string
}

func (e *BinaryExpr) expr() {}
func (e *FieldExpr) expr()  {}
func (e *ConstExpr) expr()  {}
func (e *VarExpr) expr()    {}
func (e *ArrayExpr) expr()  {}
func (e *AssignExpr) expr() {}

type FieldExpr struct {
	Index Expr
}

func (e *FieldExpr) String() string {
	return "$" + e.Index.String()
}

type BinaryExpr struct {
	Left  Expr
	Op    string
	Right Expr
}

func (e *BinaryExpr) String() string {
	var opStr string
	if e.Op == "" {
		opStr = " "
	} else {
		opStr = " " + e.Op + " "
	}
	return "(" + e.Left.String() + opStr + e.Right.String() + ")"
}

type ConstExpr struct {
	Value Value
}

func (e *ConstExpr) String() string {
	switch v := e.Value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case nil:
		return "<undefined>"
	}
	panic(fmt.Sprintf("unexpected type: %T", e.Value))
}

type VarExpr struct {
	Name string
}

func (e *VarExpr) String() string {
	return e.Name
}

type ArrayExpr struct {
	Name  string
	Index Expr
}

func (e *ArrayExpr) String() string {
	return e.Name + "[" + e.Index.String() + "]"
}

type AssignExpr struct {
	Left  Expr // can be one of: var, array[x], $n
	Right Expr
}

func (e *AssignExpr) String() string {
	return e.Left.String() + " = " + e.Right.String()
}

type Stmt interface {
	stmt()
	String() string
}

func (s *PrintStmt) stmt() {}
func (s *ExprStmt) stmt()  {}

type PrintStmt struct {
	Args []Expr
}

func (s *PrintStmt) String() string {
	parts := make([]string, len(s.Args))
	for i, a := range s.Args {
		parts[i] = a.String()
	}
	return "print " + strings.Join(parts, ", ")
}

type ExprStmt struct {
	Expr Expr
}

func (s *ExprStmt) String() string {
	return s.Expr.String()
}
