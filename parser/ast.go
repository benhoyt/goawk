// GoAWK parser - abstract syntax tree structs

package parser

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	. "github.com/benhoyt/goawk/lexer"
)

type Program struct {
	Begin     []Stmts
	Actions   []Action
	End       []Stmts
	Functions map[string]Function
}

func (p *Program) String() string {
	parts := []string{}
	for _, ss := range p.Begin {
		parts = append(parts, "BEGIN {\n"+ss.String()+"}")
	}
	for _, a := range p.Actions {
		parts = append(parts, a.String())
	}
	for _, ss := range p.End {
		parts = append(parts, "END {\n"+ss.String()+"}")
	}
	funcNames := make([]string, 0, len(p.Functions))
	for name := range p.Functions {
		funcNames = append(funcNames, name)
	}
	sort.Strings(funcNames)
	for _, name := range funcNames {
		function := p.Functions[name]
		parts = append(parts, function.String())
	}
	return strings.Join(parts, "\n\n")
}

type Stmts []Stmt

func (ss Stmts) String() string {
	lines := []string{}
	for _, s := range ss {
		subLines := strings.Split(s.String(), "\n")
		for _, sl := range subLines {
			lines = append(lines, "    "+sl+"\n")
		}
	}
	return strings.Join(lines, "")
}

type Action struct {
	Pattern []Expr
	Stmts   Stmts
}

func (a *Action) String() string {
	patterns := make([]string, len(a.Pattern))
	for i, p := range a.Pattern {
		patterns[i] = p.String()
	}
	sep := ""
	if len(patterns) > 0 {
		sep = " "
	}
	stmtsStr := ""
	if a.Stmts != nil {
		stmtsStr = "{\n" + a.Stmts.String() + "}"
	}
	return strings.Join(patterns, ", ") + sep + stmtsStr
}

type Expr interface {
	expr()
	String() string
}

func (e *FieldExpr) expr()    {}
func (e *UnaryExpr) expr()    {}
func (e *BinaryExpr) expr()   {}
func (e *InExpr) expr()       {}
func (e *CondExpr) expr()     {}
func (e *NumExpr) expr()      {}
func (e *StrExpr) expr()      {}
func (e *RegExpr) expr()      {}
func (e *VarExpr) expr()      {}
func (e *IndexExpr) expr()    {}
func (e *AssignExpr) expr()   {}
func (e *IncrExpr) expr()     {}
func (e *CallExpr) expr()     {}
func (e *UserCallExpr) expr() {}

type FieldExpr struct {
	Index Expr
}

func (e *FieldExpr) String() string {
	return "$" + e.Index.String()
}

type UnaryExpr struct {
	Op    Token
	Value Expr
}

func (e *UnaryExpr) String() string {
	return e.Op.String() + e.Value.String()
}

type BinaryExpr struct {
	Left  Expr
	Op    Token
	Right Expr
}

func (e *BinaryExpr) String() string {
	var opStr string
	if e.Op == CONCAT {
		opStr = " "
	} else {
		opStr = " " + e.Op.String() + " "
	}
	return "(" + e.Left.String() + opStr + e.Right.String() + ")"
}

type InExpr struct {
	Index []Expr
	Array string
}

func (e *InExpr) String() string {
	if len(e.Index) == 1 {
		return "(" + e.Index[0].String() + " in " + e.Array + ")"
	}
	indices := make([]string, len(e.Index))
	for i, index := range e.Index {
		indices[i] = index.String()
	}
	return "((" + strings.Join(indices, ", ") + ") in " + e.Array + ")"
}

type CondExpr struct {
	Cond  Expr
	True  Expr
	False Expr
}

func (e *CondExpr) String() string {
	return "(" + e.Cond.String() + " ? " + e.True.String() + " : " + e.False.String() + ")"
}

type NumExpr struct {
	Value float64
}

func (e *NumExpr) String() string {
	return fmt.Sprintf("%.6g", e.Value)
}

type StrExpr struct {
	Value string
}

func (e *StrExpr) String() string {
	return strconv.Quote(e.Value)
}

type RegExpr struct {
	Regex string
}

func (e *RegExpr) String() string {
	escaped := strings.Replace(e.Regex, "/", `\/`, -1)
	return "/" + escaped + "/"
}

type VarExpr struct {
	Name string
}

func (e *VarExpr) String() string {
	return e.Name
}

type IndexExpr struct {
	Name  string
	Index []Expr
}

func (e *IndexExpr) String() string {
	indices := make([]string, len(e.Index))
	for i, index := range e.Index {
		indices[i] = index.String()
	}
	return e.Name + "[" + strings.Join(indices, ", ") + "]"
}

type AssignExpr struct {
	Left  Expr // can be one of: var, array[x], $n
	Op    Token
	Right Expr
}

func (e *AssignExpr) String() string {
	return e.Left.String() + " " + e.Op.String() + " " + e.Right.String()
}

type IncrExpr struct {
	Left Expr
	Op   Token
	Pre  bool
}

func (e *IncrExpr) String() string {
	if e.Pre {
		return e.Op.String() + e.Left.String()
	} else {
		return e.Left.String() + e.Op.String()
	}
}

type CallExpr struct {
	Func Token
	Args []Expr
}

func (e *CallExpr) String() string {
	args := make([]string, len(e.Args))
	for i, a := range e.Args {
		args[i] = a.String()
	}
	return e.Func.String() + "(" + strings.Join(args, ", ") + ")"
}

type UserCallExpr struct {
	Name string
	Args []Expr
}

func (e *UserCallExpr) String() string {
	args := make([]string, len(e.Args))
	for i, a := range e.Args {
		args[i] = a.String()
	}
	return e.Name + "(" + strings.Join(args, ", ") + ")"
}

func IsLValue(expr Expr) bool {
	switch expr.(type) {
	case *VarExpr, *IndexExpr, *FieldExpr:
		return true
	default:
		return false
	}
}

type Stmt interface {
	stmt()
	String() string
}

func (s *PrintStmt) stmt()    {}
func (s *PrintfStmt) stmt()   {}
func (s *ExprStmt) stmt()     {}
func (s *IfStmt) stmt()       {}
func (s *ForStmt) stmt()      {}
func (s *ForInStmt) stmt()    {}
func (s *WhileStmt) stmt()    {}
func (s *DoWhileStmt) stmt()  {}
func (s *BreakStmt) stmt()    {}
func (s *ContinueStmt) stmt() {}
func (s *NextStmt) stmt()     {}
func (s *ExitStmt) stmt()     {}
func (s *DeleteStmt) stmt()   {}
func (s *ReturnStmt) stmt()   {}

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

type PrintfStmt struct {
	Args []Expr
}

func (s *PrintfStmt) String() string {
	parts := make([]string, len(s.Args))
	for i, a := range s.Args {
		parts[i] = a.String()
	}
	return "printf " + strings.Join(parts, ", ")
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
	str := "if (" + trimParens(s.Cond.String()) + ") {\n" + s.Body.String() + "}"
	if len(s.Else) > 0 {
		str += " else {\n" + s.Else.String() + "}"
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
	preStr := ""
	if s.Pre != nil {
		preStr = s.Pre.String()
	}
	condStr := ""
	if s.Cond != nil {
		condStr = " " + trimParens(s.Cond.String())
	}
	postStr := ""
	if s.Post != nil {
		postStr = " " + s.Post.String()
	}
	return "for (" + preStr + ";" + condStr + ";" + postStr + ") {\n" + s.Body.String() + "}"
}

type ForInStmt struct {
	Var   string
	Array string
	Body  Stmts
}

func (s *ForInStmt) String() string {
	return "for (" + s.Var + " in " + s.Array + ") {\n" + s.Body.String() + "}"
}

type WhileStmt struct {
	Cond Expr
	Body Stmts
}

func (s *WhileStmt) String() string {
	return "while (" + trimParens(s.Cond.String()) + ") {\n" + s.Body.String() + "}"
}

type DoWhileStmt struct {
	Body Stmts
	Cond Expr
}

func (s *DoWhileStmt) String() string {
	return "do {\n" + s.Body.String() + "} while (" + trimParens(s.Cond.String()) + ")"
}

type BreakStmt struct{}

func (s *BreakStmt) String() string {
	return "break"
}

type ContinueStmt struct{}

func (s *ContinueStmt) String() string {
	return "continue"
}

type NextStmt struct{}

func (s *NextStmt) String() string {
	return "next"
}

type ExitStmt struct {
	Status Expr
}

func (s *ExitStmt) String() string {
	var statusStr string
	if s.Status != nil {
		statusStr = " " + s.Status.String()
	}
	return "exit" + statusStr
}

type DeleteStmt struct {
	Array string
	Index []Expr
}

func (s *DeleteStmt) String() string {
	indices := make([]string, len(s.Index))
	for i, index := range s.Index {
		indices[i] = index.String()
	}
	return "delete " + s.Array + "[" + strings.Join(indices, ", ") + "]"
}

type ReturnStmt struct {
	Value Expr
}

func (s *ReturnStmt) String() string {
	var valueStr string
	if s.Value != nil {
		valueStr = " " + s.Value.String()
	}
	return "return" + valueStr
}

type Function struct {
	Name   string
	Params []string
	Arrays []bool
	Body   Stmts
}

func (f *Function) String() string {
	return "function " + f.Name + "(" + strings.Join(f.Params, ", ") + ") {\n" +
		f.Body.String() + "}"
}

func trimParens(s string) string {
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = s[1 : len(s)-1]
	}
	return s
}
