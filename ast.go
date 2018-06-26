package main

import (
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

func (e *FieldExpr) expr()  {}
func (e *UnaryExpr) expr()  {}
func (e *BinaryExpr) expr() {}
func (e *CondExpr) expr()   {}
func (e *ConstExpr) expr()  {}
func (e *VarExpr) expr()    {}
func (e *ArrayExpr) expr()  {}
func (e *AssignExpr) expr() {}
func (e *IncrExpr) expr()   {}
func (e *CallExpr) expr()   {}

type FieldExpr struct {
	Index Expr
}

func (e *FieldExpr) String() string {
	return "$" + e.Index.String()
}

type UnaryExpr struct {
	Op    string
	Value Expr
}

func (e *UnaryExpr) String() string {
	return e.Op + e.Value.String()
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

type CondExpr struct {
	Cond  Expr
	True  Expr
	False Expr
}

func (e *CondExpr) String() string {
	return e.Cond.String() + " ? " + e.True.String() + " : " + e.False.String()
}

type ConstExpr struct {
	Value Value
}

func (e *ConstExpr) String() string {
	return e.Value.AWKString()
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
	Op    string
	Right Expr
}

func (e *AssignExpr) String() string {
	return e.Left.String() + " " + e.Op + "= " + e.Right.String()
}

type IncrExpr struct {
	Left Expr
	Op   string
	Pre  bool
}

func (e *IncrExpr) String() string {
	if e.Pre {
		return e.Op + e.Left.String()
	} else {
		return e.Left.String() + e.Op
	}
}

type CallExpr struct {
	Name string
	Args []Expr
}

func (e *CallExpr) String() string {
	args := make([]string, len(e.Args))
	for i, a := range e.Args {
		args[i] = a.String()
	}
	return e.Name + "(" + strings.Join(args, ", ") + ")"
}

type Stmt interface {
	stmt()
	String() string
}

func (s *PrintStmt) stmt()   {}
func (s *ExprStmt) stmt()    {}
func (s *IfStmt) stmt()      {}
func (s *ForStmt) stmt()     {}
func (s *WhileStmt) stmt()   {}
func (s *DoWhileStmt) stmt() {}

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

type IfStmt struct {
	Cond Expr
	Body Stmts
	Else Stmts
}

func (s *IfStmt) String() string {
	str := "if (" + s.Cond.String() + ") {\n" + indent(s.Body.String()) + "\n}"
	if len(s.Else) > 0 {
		str += " else {\n" + indent(s.Else.String()) + "\n}"
	}
	return str
}

type ForStmt struct {
	Pre  Stmt
	Cond Expr
	Post Stmt
	Body Stmts
}

func (s *ForStmt) String() string {
	return "for (" + s.Pre.String() +
		"; " + s.Cond.String() +
		"; " + s.Post.String() + ") {\n" +
		indent(s.Body.String()) + "\n}"
}

type WhileStmt struct {
	Cond Expr
	Body Stmts
}

func (s *WhileStmt) String() string {
	return "while (" + s.Cond.String() + ") {\n" + indent(s.Body.String()) + "\n}"
}

type DoWhileStmt struct {
	Body Stmts
	Cond Expr
}

func (s *DoWhileStmt) String() string {
	return "do {\n" + indent(s.Body.String()) + "\n} while (" + s.Cond.String() + ")"
}
