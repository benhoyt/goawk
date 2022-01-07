package bytecode

import (
	"fmt"
	"regexp"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

type Program struct {
	Begin       []Op
	Actions     []Action
	End         []Op
	Functions   []Function
	ScalarNames []string
	ArrayNames  []string
	Nums        []float64
	Strs        []string
	Regexes     []*regexp.Regexp
}

type Action struct {
	Pattern []Op
	Body    []Op
}

type Function struct {
	Name   string
	Params []string
	Arrays []bool
	Body   []Op
}

func Compile(prog *parser.Program) *Program {
	p := &Program{}

	for _, stmts := range prog.Begin {
		c := &compiler{program: p}
		c.stmts(stmts)
		p.Begin = append(p.Begin, c.finish()...)
	}
	for _, action := range prog.Actions {
		var pattern, body []Op
		if len(action.Pattern) > 0 {
			switch len(action.Pattern) {
			case 1:
				c := &compiler{program: p}
				c.expr(action.Pattern[0])
				pattern = c.finish()
			default:
				panic("TODO: range patterns not yet supported")
			}
		}
		if len(action.Stmts) > 0 {
			c := &compiler{program: p}
			c.stmts(action.Stmts)
			body = c.finish()
		}
		p.Actions = append(p.Actions, Action{
			Pattern: pattern,
			Body:    body,
		})
	}
	//for _, stmts := range prog.End {
	//	c := &compiler{program: p}
	//	p.End = append(p.End, c.stmts(stmts)...)
	//	p.update(c)
	//}

	p.ScalarNames = make([]string, len(prog.Scalars))
	for name, index := range prog.Scalars {
		p.ScalarNames[index] = name
	}
	p.ArrayNames = make([]string, len(prog.Arrays))
	for name, index := range prog.Arrays {
		p.ArrayNames[index] = name
	}

	return p
}

type compiler struct {
	program *Program
	code    []Op
}

func (c *compiler) add(ops ...Op) {
	c.code = append(c.code, ops...)
}

func (c *compiler) finish() []Op {
	return c.code
}

func (c *compiler) stmts(stmts []ast.Stmt) {
	for _, stmt := range stmts {
		c.stmt(stmt)
	}
}

func (c *compiler) stmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		// Optimize assignment expressions to avoid Dupe and Drop
		switch expr := s.Expr.(type) {
		case *ast.AssignExpr:
			switch left := expr.Left.(type) {
			case *ast.VarExpr:
				if left.Scope == ast.ScopeGlobal {
					c.expr(expr.Right)
					c.add(AssignGlobal, Op(left.Index))
					return
				}
			}
		case *ast.IncrExpr:
			if !expr.Pre {
				switch target := expr.Expr.(type) {
				case *ast.VarExpr:
					if target.Scope == ast.ScopeGlobal {
						c.add(PostIncrGlobal, Op(target.Index))
						return
					}
				case *ast.IndexExpr:
					if len(target.Index) > 1 {
						panic("TODO multi indexes not yet supported")
					}
					if target.Array.Scope == ast.ScopeGlobal {
						c.expr(target.Index[0])
						c.add(PostIncrArrayGlobal, Op(target.Array.Index))
						return
					}
				}
			}
		case *ast.AugAssignExpr:
			switch left := expr.Left.(type) {
			case *ast.VarExpr:
				if left.Scope == ast.ScopeGlobal {
					c.expr(expr.Right)
					c.add(AugAssignGlobal, Op(expr.Op), Op(left.Index))
					return
				}
			}
		}
		c.expr(s.Expr)
		c.add(Drop)

	case *ast.PrintStmt:
		for _, a := range s.Args {
			c.expr(a)
		}
		if s.Redirect == lexer.ILLEGAL {
			c.add(Print, Op(len(s.Args)))
		} else {
			c.expr(s.Dest)
			c.add(PrintRedirect, Op(len(s.Args)), Op(s.Redirect))
		}

	//case *ast.PrintfStmt:
	//
	//case *ast.IfStmt:

	case *ast.ForStmt:
		if s.Pre != nil {
			c.stmt(s.Pre)
		}
		// Optimization: include condition once before loop and at the end
		var forwardMark int
		if s.Cond != nil {
			// TODO: could do the BinaryExpr optimization below here as well
			c.expr(s.Cond)
			forwardMark = len(c.code)
			c.add(JumpFalse, 0)
		}

		loopStart := len(c.code)
		c.stmts(s.Body)
		if s.Post != nil {
			c.stmt(s.Post)
		}

		if s.Cond != nil {
			// TODO: if s.Cond is BinaryExpr num == != < > <= >= or str == != then use JumpLess and similar optimizations

			done := false
			switch cond := s.Cond.(type) {
			case *ast.BinaryExpr:
				switch cond.Op {
				case lexer.LESS:
					if _, ok := cond.Right.(*ast.NumExpr); ok {
						done = true
						c.expr(cond.Left)
						c.expr(cond.Right)
						offset := loopStart - (len(c.code) + 2)
						c.add(JumpNumLess, Op(int32(offset)))
					}
				case lexer.LTE:
					//if _, ok := cond.Right.(*ast.NumExpr); ok { // TODO: or number special variable like NF
					done = true
					c.expr(cond.Left)
					c.expr(cond.Right)
					offset := loopStart - (len(c.code) + 2)
					c.add(JumpNumLessOrEqual, Op(int32(offset)))
					//}
				}
			}
			if !done {
				c.expr(s.Cond)
				offset := loopStart - (len(c.code) + 2)
				c.add(JumpTrue, Op(int32(offset)))
			}

			offset := len(c.code) - (forwardMark + 2)
			c.code[forwardMark+1] = Op(int32(offset))
		} else {
			offset := loopStart - (len(c.code) + 2)
			c.add(Jump, Op(int32(offset)))
		}

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
}

func (c *compiler) expr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.NumExpr:
		c.add(Num, Op(len(c.program.Nums)))
		c.program.Nums = append(c.program.Nums, e.Value)

	case *ast.StrExpr:
		c.add(Str, Op(len(c.program.Strs)))
		c.program.Strs = append(c.program.Strs, e.Value)

	case *ast.FieldExpr:
		c.expr(e.Index)
		c.add(Field)

	case *ast.VarExpr:
		switch e.Scope {
		case ast.ScopeGlobal:
			c.add(Global, Op(e.Index))
		case ast.ScopeLocal:
		case ast.ScopeSpecial:
			c.add(Special, Op(e.Index))
		}

	//case *ast.RegExpr:
	//

	case *ast.BinaryExpr:
		switch e.Op {
		case lexer.AND:
			panic("TODO: &&")
		case lexer.OR:
			panic("TODO: ||")
		}
		c.expr(e.Left)
		c.expr(e.Right)
		var opcode Op
		switch e.Op {
		case lexer.ADD:
			opcode = Add
		case lexer.SUB:
			opcode = Subtract
		case lexer.EQUALS:
			opcode = Equals
		case lexer.LESS:
			opcode = Less
		case lexer.LTE:
			opcode = LessOrEqual
		case lexer.CONCAT:
			opcode = Concat
		case lexer.MUL:
			opcode = Multiply
		case lexer.DIV:
			opcode = Divide
		case lexer.GREATER:
			opcode = Greater
		case lexer.GTE:
			opcode = GreaterOrEqual
		case lexer.NOT_EQUALS:
			opcode = NotEquals
		case lexer.MATCH:
			opcode = Match
		case lexer.NOT_MATCH:
			opcode = NotMatch
		case lexer.POW:
			opcode = Power
		case lexer.MOD:
			opcode = Modulo
		default:
			panic(fmt.Sprintf("unexpected binary operation: %s", e.Op))
		}
		c.add(opcode)

	//case *ast.IncrExpr:

	case *ast.AssignExpr:
		c.expr(e.Right)
		c.add(Dupe)
		switch left := e.Left.(type) {
		case *ast.VarExpr:
			switch left.Scope {
			case ast.ScopeGlobal:
				c.add(AssignGlobal, Op(left.Index))
			case ast.ScopeLocal:
			default: // ast.ScopeSpecial
			}
		case *ast.IndexExpr:
		default: // *ast.FieldExpr
		}

	//case *ast.AugAssignExpr:
	//
	//case *ast.CondExpr:
	//
	//case *ast.IndexExpr:
	//

	case *ast.CallExpr:
		switch e.Func {
		case lexer.F_TOLOWER:
			c.expr(e.Args[0])
			c.add(CallBuiltin, Op(lexer.F_TOLOWER))
		default:
			panic(fmt.Sprintf("TODO: func %s not yet supported", e.Func))
		}

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
}
