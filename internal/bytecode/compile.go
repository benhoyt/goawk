package bytecode

import (
	"fmt"
	"regexp"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/parser"
)

type Program struct {
	Begin       []Opcode
	Actions     []Action
	End         []Opcode
	Functions   []Function
	ScalarNames []string
	ArrayNames  []string
	Nums        []float64
	Strs        []string
	Regexes     []*regexp.Regexp
}

type Action struct {
	Pattern []Opcode
	Body    []Opcode
}

type Function struct {
	Name   string
	Params []string
	Arrays []bool
	Body   []Opcode
}

func Compile(prog *parser.Program) *Program {
	p := &Program{}
	c := &compiler{}

	for _, stmts := range prog.Begin {
		p.Begin = append(p.Begin, c.stmts(stmts)...)
	}
	//for _, action := range prog.Actions {
	//}
	for _, stmts := range prog.End {
		p.End = append(p.End, c.stmts(stmts)...)
	}

	p.ScalarNames = make([]string, len(prog.Scalars))
	for name, index := range prog.Scalars {
		p.ScalarNames[index] = name
	}
	p.ArrayNames = make([]string, len(prog.Arrays))
	for name, index := range prog.Arrays {
		p.ArrayNames[index] = name
	}
	p.Nums = c.nums
	p.Strs = c.strs
	p.Regexes = c.regexes
	return p
}

type compiler struct {
	nums    []float64
	strs    []string
	regexes []*regexp.Regexp
}

func (c *compiler) stmts(stmts []ast.Stmt) []Opcode {
	var code []Opcode
	for _, stmt := range stmts {
		code = append(code, c.stmt(stmt)...)
	}
	return code
}

func (c *compiler) stmt(stmt ast.Stmt) []Opcode {
	var code []Opcode
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		code = append(code, c.expr(s.Expr)...)
		code = append(code, Drop)

	//case *ast.PrintStmt:
	//
	//case *ast.PrintfStmt:
	//
	//case *ast.IfStmt:
	//
	//case *ast.ForStmt:
	//
	//case *ast.ForInStmt:
	//
	//case *ast.ReturnStmt:
	//
	//case *ast.WhileStmt:
	//
	//case *ast.DoWhileStmt:
	//
	//case *ast.BreakStmt:
	//case *ast.ContinueStmt:
	//case *ast.NextStmt:
	//case *ast.ExitStmt:
	//
	//case *ast.DeleteStmt:
	//
	//case *ast.BlockStmt:

	default:
		// Should never happen
		panic(fmt.Sprintf("unexpected stmt type: %T", stmt))
	}
	return code
}

func (c *compiler) expr(expr ast.Expr) []Opcode {
	var code []Opcode
	switch e := expr.(type) {
	case *ast.NumExpr:
		if len(c.nums) >= 256 {
			panic("TODO: too many nums!")
		}
		code = append(code, Num, Opcode(len(c.nums)))
		c.nums = append(c.nums, e.Value)

	case *ast.StrExpr:
		if len(c.strs) >= 256 {
			panic("TODO: too many strs!")
		}
		code = append(code, Str, Opcode(len(c.strs)))
		c.strs = append(c.strs, e.Value)

	//case *ast.FieldExpr:
	//
	//case *ast.VarExpr:
	//
	//case *ast.RegExpr:
	//
	//case *ast.BinaryExpr:
	//	switch e.Op {
	//	case lexer.AND:
	//	case lexer.OR:
	//	default:
	//	}
	//
	//case *ast.IncrExpr:
	//
	//case *ast.AssignExpr:
	//
	//case *ast.AugAssignExpr:
	//
	//case *ast.CondExpr:
	//
	//case *ast.IndexExpr:
	//
	//case *ast.CallExpr:
	//
	//case *ast.UnaryExpr:
	//
	//case *ast.InExpr:
	//
	//case *ast.UserCallExpr:
	//
	//case *ast.GetlineExpr:

	default:
		// Should never happen
		panic(fmt.Sprintf("unexpected expr type: %T", expr))
	}
	return code
}
