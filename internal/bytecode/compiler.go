package bytecode

import (
	"fmt"
	"math"
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
	Pattern [][]Op
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
		var pattern [][]Op
		switch len(action.Pattern) {
		case 0:
			// TODO: can we somehow do more at compile time here?
		case 1:
			c := &compiler{program: p}
			c.expr(action.Pattern[0])
			pattern = [][]Op{c.finish()}
		case 2:
			c := &compiler{program: p}
			c.expr(action.Pattern[0])
			pattern = append(pattern, c.finish())
			c = &compiler{program: p}
			c.expr(action.Pattern[1])
			pattern = append(pattern, c.finish())
		}
		var body []Op
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

	for _, stmts := range prog.End {
		c := &compiler{program: p}
		c.stmts(stmts)
		p.End = append(p.End, c.finish()...)
	}

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
	program   *Program
	code      []Op
	breaks    [][]int
	continues [][]int
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
			switch target := expr.Left.(type) {
			case *ast.VarExpr:
				switch target.Scope {
				case ast.ScopeGlobal:
					c.expr(expr.Right)
					c.add(AssignGlobal, Op(target.Index))
					return
				case ast.ScopeLocal:
					panic("TODO assign scope local")
				case ast.ScopeSpecial:
					c.expr(expr.Right)
					c.add(AssignSpecial, Op(target.Index))
					return
				}
			case *ast.FieldExpr:
				c.expr(expr.Right)
				c.expr(target.Index)
				c.add(AssignField)
				return
			case *ast.IndexExpr:
				if len(target.Index) > 1 {
					panic("TODO multi indexes not yet supported")
				}
				switch target.Array.Scope {
				case ast.ScopeGlobal:
					c.expr(expr.Right)
					c.expr(target.Index[0])
					c.add(AssignArrayGlobal, Op(target.Array.Index))
					return
				case ast.ScopeLocal:
					c.expr(expr.Right)
					c.expr(target.Index[0])
					c.add(AssignArrayLocal, Op(target.Array.Index))
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
				// TODO: case *ast.FieldExpr:
				case *ast.IndexExpr:
					if len(target.Index) > 1 {
						panic("TODO multi indexes not yet supported")
					}
					switch target.Array.Scope {
					case ast.ScopeGlobal:
						c.expr(target.Index[0])
						c.add(PostIncrArrayGlobal, Op(target.Array.Index))
						return
					case ast.ScopeLocal:
						c.expr(target.Index[0])
						c.add(PostIncrArrayLocal, Op(target.Array.Index))
						return
					}
				}
			}

		case *ast.AugAssignExpr:
			switch target := expr.Left.(type) {
			case *ast.VarExpr:
				if target.Scope == ast.ScopeGlobal {
					c.expr(expr.Right)
					c.add(AugAssignGlobal, Op(expr.Op), Op(target.Index))
					return
				}
				// TODO: case *ast.IndexExpr
				// TODO: case *ast.ArrayExpr
			}
		}

		// Non-optimized expression: push it and then drop
		c.expr(s.Expr)
		c.add(Drop)

	case *ast.PrintStmt:
		if s.Redirect != lexer.ILLEGAL {
			c.expr(s.Dest) // redirect destination
		}
		for _, a := range s.Args {
			c.expr(a)
		}
		c.add(Print, Op(len(s.Args)), Op(s.Redirect))

	case *ast.PrintfStmt:
		if s.Redirect != lexer.ILLEGAL {
			c.expr(s.Dest) // redirect destination
		}
		for _, a := range s.Args {
			c.expr(a)
		}
		c.add(Printf, Op(len(s.Args)), Op(s.Redirect))

	case *ast.IfStmt:
		if len(s.Else) == 0 {
			jumpOp := c.cond(s.Cond, true)
			ifMark := c.jumpForward(jumpOp)
			c.stmts(s.Body)
			c.patchForward(ifMark)
		} else {
			jumpOp := c.cond(s.Cond, true)
			ifMark := c.jumpForward(jumpOp)
			c.stmts(s.Body)
			elseMark := c.jumpForward(Jump)
			c.patchForward(ifMark)
			c.stmts(s.Else)
			c.patchForward(elseMark)
		}

	case *ast.ForStmt:
		if s.Pre != nil {
			c.stmt(s.Pre)
		}
		c.breaks = append(c.breaks, []int{})
		c.continues = append(c.continues, []int{})

		// Optimization: include condition once before loop and at the end
		var mark int
		if s.Cond != nil {
			jumpOp := c.cond(s.Cond, true)
			mark = c.jumpForward(jumpOp)
		}

		loopStart := c.labelBackward()
		c.stmts(s.Body)
		c.patchContinues()
		if s.Post != nil {
			c.stmt(s.Post)
		}

		if s.Cond != nil {
			// TODO: if s.Cond is BinaryExpr num == != < > <= >= or str == != then use JumpLess and similar optimizations
			jumpOp := c.cond(s.Cond, false)
			c.jumpBackward(loopStart, jumpOp)
			c.patchForward(mark)
		} else {
			c.jumpBackward(loopStart, Jump)
		}

		c.patchBreaks()

	case *ast.ForInStmt:
		var op Op
		switch {
		case s.Var.Scope == ast.ScopeGlobal && s.Array.Scope == ast.ScopeGlobal:
			op = ForGlobalInGlobal
		default:
			panic("TODO: for in with local/special not yet supported")
		}
		mark := c.jumpForward(op, Op(s.Var.Index), Op(s.Array.Index))

		c.breaks = append(c.breaks, nil) // nil tells BreakStmt it's a for..in loop
		c.continues = append(c.continues, []int{})

		c.stmts(s.Body)

		c.patchForward(mark)
		c.patchContinues()
		c.breaks = c.breaks[:len(c.breaks)-1]

	//case *ast.ReturnStmt:
	//

	case *ast.WhileStmt:
		c.breaks = append(c.breaks, []int{})
		c.continues = append(c.continues, []int{})

		jumpOp := c.cond(s.Cond, true)
		mark := c.jumpForward(jumpOp)

		loopStart := c.labelBackward()
		c.stmts(s.Body)
		c.patchContinues()

		// TODO: if s.Cond is BinaryExpr num == != < > <= >= or str == != then use JumpLess and similar optimizations
		jumpOp = c.cond(s.Cond, false)
		c.jumpBackward(loopStart, jumpOp)
		c.patchForward(mark)

		c.patchBreaks()

	case *ast.DoWhileStmt:
		c.breaks = append(c.breaks, []int{})
		c.continues = append(c.continues, []int{})

		loopStart := c.labelBackward()
		c.stmts(s.Body)
		c.patchContinues()

		jumpOp := c.cond(s.Cond, false)
		c.jumpBackward(loopStart, jumpOp)

		c.patchBreaks()

	case *ast.BreakStmt:
		i := len(c.breaks) - 1
		if c.breaks[i] == nil {
			// break in for..in loop is executed differently, use errBreak to exit
			c.add(BreakForIn)
		} else {
			mark := c.jumpForward(Jump)
			c.breaks[i] = append(c.breaks[i], mark)
		}

	case *ast.ContinueStmt:
		i := len(c.continues) - 1
		mark := c.jumpForward(Jump)
		c.continues[i] = append(c.continues[i], mark)

	case *ast.NextStmt:
		c.add(Next)

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

func (c *compiler) patchBreaks() {
	breaks := c.breaks[len(c.breaks)-1]
	for _, mark := range breaks {
		c.patchForward(mark)
	}
	c.breaks = c.breaks[:len(c.breaks)-1]
}

func (c *compiler) patchContinues() {
	continues := c.continues[len(c.continues)-1]
	for _, mark := range continues {
		c.patchForward(mark)
	}
	c.continues = c.continues[:len(c.continues)-1]
}

func (c *compiler) jumpForward(jumpOp Op, args ...Op) int {
	c.add(jumpOp)
	c.add(args...)
	c.add(0)
	return len(c.code)
}

func (c *compiler) patchForward(mark int) {
	offset := len(c.code) - mark
	if offset > math.MaxInt32 || offset < math.MinInt32 {
		panic("forward jump offset too large") // TODO: handle more gracefully?
	}
	c.code[mark-1] = Op(int32(offset))
}

func (c *compiler) labelBackward() int {
	return len(c.code)
}

func (c *compiler) jumpBackward(label int, jumpOp Op, args ...Op) {
	offset := label - (len(c.code) + len(args) + 2)
	if offset > math.MaxInt32 || offset < math.MinInt32 {
		panic("backward jump offset too large") // TODO: handle more gracefully?
	}
	c.add(jumpOp)
	c.add(args...)
	c.add(Op(int32(offset)))
}

func (c *compiler) cond(expr ast.Expr, invert bool) Op {
	var jumpOp Op
	switch cond := expr.(type) {
	case *ast.BinaryExpr:
		switch cond.Op {
		case lexer.LESS:
			if _, ok := cond.Right.(*ast.NumExpr); ok {
				c.expr(cond.Left)
				c.expr(cond.Right)
				if invert {
					jumpOp = JumpNumGreaterOrEqual
				} else {
					jumpOp = JumpNumLess
				}
			}
		case lexer.LTE:
			//if _, ok := cond.Right.(*ast.NumExpr); ok { // TODO: or number special variable like NF
			c.expr(cond.Left)
			c.expr(cond.Right)
			if invert {
				jumpOp = JumpNumGreater
			} else {
				jumpOp = JumpNumLessOrEqual
			}
			//}
		}
	}
	if jumpOp == Nop {
		c.expr(expr)
		if invert {
			jumpOp = JumpFalse
		} else {
			jumpOp = JumpTrue
		}
	}
	return jumpOp
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

	case *ast.RegExpr:
		c.add(Regex, Op(len(c.program.Regexes)))
		c.program.Regexes = append(c.program.Regexes, regexp.MustCompile(e.Regex))

	case *ast.BinaryExpr:
		switch e.Op {
		case lexer.AND:
			panic("TODO: &&")
		case lexer.OR:
			panic("TODO: ||")
		}
		c.expr(e.Left)
		c.expr(e.Right)
		var op Op
		switch e.Op {
		case lexer.ADD:
			op = Add
		case lexer.SUB:
			op = Subtract
		case lexer.EQUALS:
			op = Equals
		case lexer.LESS:
			op = Less
		case lexer.LTE:
			op = LessOrEqual
		case lexer.CONCAT:
			op = Concat
		case lexer.MUL:
			op = Multiply
		case lexer.DIV:
			op = Divide
		case lexer.GREATER:
			op = Greater
		case lexer.GTE:
			op = GreaterOrEqual
		case lexer.NOT_EQUALS:
			op = NotEquals
		case lexer.MATCH:
			op = Match
		case lexer.NOT_MATCH:
			op = NotMatch
		case lexer.POW:
			op = Power
		case lexer.MOD:
			op = Modulo
		default:
			panic(fmt.Sprintf("unexpected binary operation: %s", e.Op))
		}
		c.add(op)

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
	case *ast.IndexExpr:
		if len(e.Index) > 1 {
			panic("TODO multi indexes not yet supported")
		}
		switch e.Array.Scope {
		case ast.ScopeGlobal:
			c.expr(e.Index[0])
			c.add(ArrayGlobal, Op(e.Array.Index))
		case ast.ScopeLocal:
			panic("TODO IndexExpr local array not yet supported")
		}

	case *ast.CallExpr:
		switch e.Func {
		case lexer.F_SPLIT:
			c.expr(e.Args[0])
			arrayExpr := e.Args[1].(*ast.ArrayExpr)
			switch {
			case arrayExpr.Scope == ast.ScopeGlobal && len(e.Args) > 2:
				c.expr(e.Args[2])
				c.add(CallSplitSepGlobal, Op(arrayExpr.Index))
			case arrayExpr.Scope == ast.ScopeGlobal:
				c.add(CallSplitGlobal, Op(arrayExpr.Index))
			case arrayExpr.Scope == ast.ScopeLocal && len(e.Args) > 2:
				c.expr(e.Args[2])
				c.add(CallSplitSepLocal, Op(arrayExpr.Index))
			case arrayExpr.Scope == ast.ScopeLocal:
				c.add(CallSplitLocal, Op(arrayExpr.Index))
			default:
				panic(fmt.Sprintf("unexpected array scope %s or num args %d", arrayExpr.Scope, len(e.Args)))
			}
			return
			//case lexer.F_SUB, lexer.F_GSUB:
		}

		for _, arg := range e.Args {
			c.expr(arg)
		}
		switch e.Func {
		case lexer.F_ATAN2:
			c.add(CallAtan2)
		case lexer.F_CLOSE:
			c.add(CallClose)
		case lexer.F_COS:
			c.add(CallCos)
		case lexer.F_EXP:
			c.add(CallExp)
		case lexer.F_FFLUSH:
			if len(e.Args) > 0 {
				c.add(CallFflush)
			} else {
				c.add(CallFflushAll)
			}
		case lexer.F_INDEX:
			c.add(CallIndex)
		case lexer.F_INT:
			c.add(CallInt)
		case lexer.F_LENGTH:
			if len(e.Args) > 0 {
				c.add(CallLengthArg)
			} else {
				c.add(CallLength)
			}
		case lexer.F_LOG:
			c.add(CallLog)
		case lexer.F_MATCH:
			c.add(CallMatch)
		case lexer.F_RAND:
			c.add(CallRand)
		case lexer.F_SIN:
			c.add(CallSin)
		case lexer.F_SPRINTF:
			c.add(Op(len(e.Args)))
			c.add(CallSprintf)
		case lexer.F_SQRT:
			c.add(CallSqrt)
		case lexer.F_SRAND:
			if len(e.Args) > 0 {
				c.add(CallSrandSeed)
			} else {
				c.add(CallSrand)
			}
		case lexer.F_SUBSTR:
			if len(e.Args) > 2 {
				c.add(CallSubstrLength)
			} else {
				c.add(CallSubstr)
			}
		case lexer.F_SYSTEM:
			c.add(CallSystem)
		case lexer.F_TOLOWER:
			c.add(CallTolower)
		case lexer.F_TOUPPER:
			c.add(CallToupper)
		default:
			panic(fmt.Sprintf("TODO: func %s not yet supported", e.Func))
		}

	case *ast.UnaryExpr:
		c.expr(e.Value)
		switch e.Op {
		case lexer.SUB:
			c.add(UnaryMinus)
		case lexer.NOT:
			c.add(Not)
		case lexer.ADD:
			c.add(UnaryPlus)
		default:
			panic(fmt.Sprintf("unexpected unary operation: %s", e.Op))
		}

	case *ast.InExpr:
		if len(e.Index) > 1 {
			panic("TODO multi indexes not yet supported")
		}
		switch e.Array.Scope {
		case ast.ScopeGlobal:
			c.expr(e.Index[0])
			c.add(InGlobal, Op(e.Array.Index))
		case ast.ScopeLocal:
			c.expr(e.Index[0])
			c.add(InLocal, Op(e.Array.Index))
		}

	//case *ast.UserCallExpr:
	//
	//case *ast.GetlineExpr:

	default:
		// Should never happen
		panic(fmt.Sprintf("unexpected expr type: %T", expr))
	}
}
