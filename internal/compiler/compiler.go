package compiler

import (
	"fmt"
	"math"
	"regexp"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/lexer"
)

/*
TODO:
- refactor, simplify, reduce copy-n-pasta
- other TODOs
- test disassembler in compiler package, for each instruction that it outputs instructions name - can we do that generically?
- check overflow everywhere we output a number in an opcode
- look at code coverage and get closer to 100%
  + add decrement tests under "Incr/decr expressions", for example
  + use the following to see how much of the internal/compiler package is covered:
    $ go test -coverpkg=./... ./interp -v -awk="" -coverprofile=cover.out
    $ go tool cover -html cover.out
- optimize! probably on new branch
  + any super-instructions to add?
  + any instructions to remove?
  + specializations
  + optimize CONCAT(a,CONCAT(b,c)) etc to CONCAT(a, b, c) to avoid allocs/copying
- fuzz testing
*/

type Program struct {
	Begin     []Opcode
	Actions   []Action
	End       []Opcode
	Functions []Function
	Nums      []float64
	Strs      []string
	Regexes   []*regexp.Regexp

	numIndexes   map[float64]int
	strIndexes   map[string]int
	regexIndexes map[string]int

	// For disassembly
	scalarNames     []string
	arrayNames      []string
	nativeFuncNames []string
}

type Action struct {
	Pattern [][]Opcode
	Body    []Opcode
}

type Function struct {
	Name       string
	Params     []string
	Arrays     []bool
	NumScalars int
	NumArrays  int
	Body       []Opcode
}

// compileError is the internal error type raised in the rare cases when
// compilation can't succeed, such as program too large (jump offsets greater
// than 2GB). Most actual problems are caught as parse time.
type compileError struct {
	message string
}

func (e *compileError) Error() string {
	return e.message
}

func Compile(prog *ast.Program) (compiledProg *Program, err error) {
	defer func() {
		// The compiler uses panic with a *CompileError to signal compile
		// errors internally, and they're caught here. This avoids the
		// need to check errors everywhere.
		if r := recover(); r != nil {
			// Convert to ParseError or re-panic
			err = r.(*compileError)
		}
	}()

	p := &Program{}

	// For functions called before they're defined or recursive functions, we
	// have to set most p.Functions data first, then compile Body after.
	p.Functions = make([]Function, len(prog.Functions))
	for i, astFunc := range prog.Functions {
		numArrays := 0
		for _, a := range astFunc.Arrays {
			if a {
				numArrays++
			}
		}
		compiledFunc := Function{
			Name:       astFunc.Name,
			Params:     astFunc.Params,
			Arrays:     astFunc.Arrays,
			NumScalars: len(astFunc.Arrays) - numArrays,
			NumArrays:  numArrays,
		}
		p.Functions[i] = compiledFunc
	}
	for i, astFunc := range prog.Functions {
		c := &compiler{program: p}
		c.stmts(astFunc.Body)
		p.Functions[i].Body = c.finish()
	}

	for _, stmts := range prog.Begin {
		c := &compiler{program: p}
		c.stmts(stmts)
		p.Begin = append(p.Begin, c.finish()...)
	}

	for _, action := range prog.Actions {
		var pattern [][]Opcode
		switch len(action.Pattern) {
		case 0:
			// Always considered a match
		case 1:
			c := &compiler{program: p}
			c.expr(action.Pattern[0])
			pattern = [][]Opcode{c.finish()}
		case 2:
			c := &compiler{program: p}
			c.expr(action.Pattern[0])
			pattern = append(pattern, c.finish())
			c = &compiler{program: p}
			c.expr(action.Pattern[1])
			pattern = append(pattern, c.finish())
		}
		var body []Opcode
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

	p.scalarNames = make([]string, len(prog.Scalars))
	for name, index := range prog.Scalars {
		p.scalarNames[index] = name
	}
	p.arrayNames = make([]string, len(prog.Arrays))
	for name, index := range prog.Arrays {
		p.arrayNames[index] = name
	}

	return p, nil
}

type compiler struct {
	program   *Program
	code      []Opcode
	breaks    [][]int
	continues [][]int
}

func (c *compiler) add(ops ...Opcode) {
	c.code = append(c.code, ops...)
}

func (c *compiler) finish() []Opcode {
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
			c.expr(expr.Right)
			c.assign(expr.Left)
			return

		case *ast.IncrExpr:
			// Pre or post doesn't matter for an assignment expression
			switch target := expr.Expr.(type) {
			case *ast.VarExpr:
				switch target.Scope {
				case ast.ScopeGlobal:
					if expr.Op == lexer.INCR {
						c.add(IncrGlobal, 1, opcodeInt(target.Index))
					} else {
						c.add(IncrGlobal, -1, opcodeInt(target.Index))
					}
					return
				case ast.ScopeLocal:
					if expr.Op == lexer.INCR {
						c.add(IncrLocal, 1, opcodeInt(target.Index))
					} else {
						c.add(IncrLocal, -1, opcodeInt(target.Index))
					}
					return
				case ast.ScopeSpecial:
					if expr.Op == lexer.INCR {
						c.add(IncrSpecial, 1, opcodeInt(target.Index))
					} else {
						c.add(IncrSpecial, -1, opcodeInt(target.Index))
					}
					return
				}
			case *ast.FieldExpr:
				c.expr(target.Index)
				if expr.Op == lexer.INCR {
					c.add(IncrField, 1)
				} else {
					c.add(IncrField, -1)
				}
				return
			case *ast.IndexExpr:
				c.index(target.Index)
				switch target.Array.Scope {
				case ast.ScopeGlobal:
					if expr.Op == lexer.INCR {
						c.add(IncrArrayGlobal, 1, opcodeInt(target.Array.Index))
					} else {
						c.add(IncrArrayGlobal, -1, opcodeInt(target.Array.Index))
					}
					return
				case ast.ScopeLocal:
					if expr.Op == lexer.INCR {
						c.add(IncrArrayLocal, 1, opcodeInt(target.Array.Index))
					} else {
						c.add(IncrArrayLocal, -1, opcodeInt(target.Array.Index))
					}
					return
				}
			}

		case *ast.AugAssignExpr:
			c.expr(expr.Right)
			switch target := expr.Left.(type) {
			case *ast.VarExpr:
				switch target.Scope {
				case ast.ScopeGlobal:
					c.add(AugAssignGlobal, Opcode(expr.Op), opcodeInt(target.Index))
				case ast.ScopeLocal:
					c.add(AugAssignLocal, Opcode(expr.Op), opcodeInt(target.Index))
				case ast.ScopeSpecial:
					c.add(AugAssignSpecial, Opcode(expr.Op), opcodeInt(target.Index))
				}
			case *ast.FieldExpr:
				c.expr(target.Index)
				c.add(AugAssignField, Opcode(expr.Op))
			case *ast.IndexExpr:
				c.index(target.Index)
				switch target.Array.Scope {
				case ast.ScopeGlobal:
					c.add(AugAssignArrayGlobal, Opcode(expr.Op), opcodeInt(target.Array.Index))
				case ast.ScopeLocal:
					c.add(AugAssignArrayLocal, Opcode(expr.Op), opcodeInt(target.Array.Index))
				}
			}
			return
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
		c.add(Print, opcodeInt(len(s.Args)), Opcode(s.Redirect))

	case *ast.PrintfStmt:
		if s.Redirect != lexer.ILLEGAL {
			c.expr(s.Dest) // redirect destination
		}
		for _, a := range s.Args {
			c.expr(a)
		}
		c.add(Printf, opcodeInt(len(s.Args)), Opcode(s.Redirect))

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
			jumpOp := c.cond(s.Cond, false)
			c.jumpBackward(loopStart, jumpOp)
			c.patchForward(mark)
		} else {
			c.jumpBackward(loopStart, Jump)
		}

		c.patchBreaks()

	case *ast.ForInStmt:
		var op Opcode
		switch s.Var.Scope {
		case ast.ScopeGlobal:
			op = ForInGlobal
		case ast.ScopeLocal:
			op = ForInLocal
		default: // ScopeSpecial
			op = ForInSpecial
		}
		mark := c.jumpForward(op, opcodeInt(s.Var.Index), Opcode(s.Array.Scope), opcodeInt(s.Array.Index))

		c.breaks = append(c.breaks, nil) // nil tells BreakStmt it's a for..in loop
		c.continues = append(c.continues, []int{})

		c.stmts(s.Body)

		c.patchForward(mark)
		c.patchContinues()
		c.breaks = c.breaks[:len(c.breaks)-1]

	case *ast.ReturnStmt:
		if s.Value != nil {
			c.expr(s.Value)
			c.add(Return)
		} else {
			c.add(ReturnNull)
		}

	case *ast.WhileStmt:
		c.breaks = append(c.breaks, []int{})
		c.continues = append(c.continues, []int{})

		jumpOp := c.cond(s.Cond, true)
		mark := c.jumpForward(jumpOp)

		loopStart := c.labelBackward()
		c.stmts(s.Body)
		c.patchContinues()

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

	case *ast.ExitStmt:
		if s.Status != nil {
			c.expr(s.Status)
		} else {
			c.expr(&ast.NumExpr{0})
		}
		c.add(Exit)

	case *ast.DeleteStmt:
		if len(s.Index) > 0 {
			c.index(s.Index)
			switch s.Array.Scope {
			case ast.ScopeGlobal:
				c.add(DeleteGlobal, opcodeInt(s.Array.Index))
			case ast.ScopeLocal:
				c.add(DeleteLocal, opcodeInt(s.Array.Index))
			}
		} else {
			switch s.Array.Scope {
			case ast.ScopeGlobal:
				c.add(DeleteAllGlobal, opcodeInt(s.Array.Index))
			case ast.ScopeLocal:
				c.add(DeleteAllLocal, opcodeInt(s.Array.Index))
			}
		}

	case *ast.BlockStmt:
		c.stmts(s.Body)

	default:
		// Should never happen
		panic(fmt.Sprintf("unexpected stmt type: %T", stmt))
	}
}

func (c *compiler) assign(target ast.Expr) {
	switch target := target.(type) {
	case *ast.VarExpr:
		switch target.Scope {
		case ast.ScopeGlobal:
			c.add(AssignGlobal, opcodeInt(target.Index))
		case ast.ScopeLocal:
			c.add(AssignLocal, opcodeInt(target.Index))
		case ast.ScopeSpecial:
			c.add(AssignSpecial, opcodeInt(target.Index))
		}
	case *ast.FieldExpr:
		c.expr(target.Index)
		c.add(AssignField)
	case *ast.IndexExpr:
		c.index(target.Index)
		switch target.Array.Scope {
		case ast.ScopeGlobal:
			c.add(AssignArrayGlobal, opcodeInt(target.Array.Index))
		case ast.ScopeLocal:
			c.add(AssignArrayLocal, opcodeInt(target.Array.Index))
		}
	}
}

// opcodeInt converts int to Opcode, ensuring it fits.
func opcodeInt(n int) Opcode {
	if n > math.MaxInt32 || n < math.MinInt32 {
		// Two billion should be enough for anybody.
		panic(&compileError{message: fmt.Sprintf("program too large (constant index or jump offset %d doesn't fit in int32)", n)})
	}
	return Opcode(n)
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

func (c *compiler) jumpForward(jumpOp Opcode, args ...Opcode) int {
	c.add(jumpOp)
	c.add(args...)
	c.add(0)
	return len(c.code)
}

func (c *compiler) patchForward(mark int) {
	offset := len(c.code) - mark
	c.code[mark-1] = opcodeInt(offset)
}

func (c *compiler) labelBackward() int {
	return len(c.code)
}

func (c *compiler) jumpBackward(label int, jumpOp Opcode, args ...Opcode) {
	offset := label - (len(c.code) + len(args) + 2)
	c.add(jumpOp)
	c.add(args...)
	c.add(opcodeInt(offset))
}

// TODO: better performance to have JumpNumLess and so on with number as opcode?
func (c *compiler) cond(expr ast.Expr, invert bool) Opcode {
	switch cond := expr.(type) {
	case *ast.BinaryExpr:
		switch cond.Op {
		case lexer.EQUALS:
			c.expr(cond.Left)
			c.expr(cond.Right)
			if invert {
				return JumpNotEquals
			} else {
				return JumpEquals
			}

		case lexer.NOT_EQUALS:
			c.expr(cond.Left)
			c.expr(cond.Right)
			if invert {
				return JumpEquals
			} else {
				return JumpNotEquals
			}

		case lexer.LESS:
			c.expr(cond.Left)
			c.expr(cond.Right)
			if invert {
				return JumpGreaterOrEqual
			} else {
				return JumpLess
			}

		case lexer.LTE:
			c.expr(cond.Left)
			c.expr(cond.Right)
			if invert {
				return JumpGreater
			} else {
				return JumpLessOrEqual
			}

		case lexer.GREATER:
			c.expr(cond.Left)
			c.expr(cond.Right)
			if invert {
				return JumpLessOrEqual
			} else {
				return JumpGreater
			}

		case lexer.GTE:
			c.expr(cond.Left)
			c.expr(cond.Right)
			if invert {
				return JumpLess
			} else {
				return JumpGreaterOrEqual
			}
		}
	}

	c.expr(expr)
	if invert {
		return JumpFalse
	} else {
		return JumpTrue
	}
}

func (c *compiler) expr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.NumExpr:
		c.add(Num, opcodeInt(c.numIndex(e.Value)))

	case *ast.StrExpr:
		c.add(Str, opcodeInt(c.strIndex(e.Value)))

	case *ast.FieldExpr:
		switch index := e.Index.(type) {
		case *ast.NumExpr:
			if index.Value == float64(int32(index.Value)) {
				c.add(FieldNum, opcodeInt(int(index.Value)))
				return
			}
		}
		c.expr(e.Index)
		c.add(Field)

	case *ast.VarExpr:
		switch e.Scope {
		case ast.ScopeGlobal:
			c.add(Global, opcodeInt(e.Index))
		case ast.ScopeLocal:
			c.add(Local, opcodeInt(e.Index))
		case ast.ScopeSpecial:
			c.add(Special, opcodeInt(e.Index))
		}

	case *ast.RegExpr:
		c.add(Regex, opcodeInt(c.regexIndex(e.Regex)))

	case *ast.BinaryExpr:
		// && and || are special cases as they're short-circuit operators.
		switch e.Op {
		case lexer.AND:
			c.expr(e.Left)
			c.add(Dupe)
			mark := c.jumpForward(JumpFalse)
			c.add(Drop)
			c.expr(e.Right)
			c.patchForward(mark)
			c.add(Boolean)
		case lexer.OR:
			c.expr(e.Left)
			c.add(Dupe)
			mark := c.jumpForward(JumpTrue)
			c.add(Drop)
			c.expr(e.Right)
			c.patchForward(mark)
			c.add(Boolean)
		default:
			c.expr(e.Left)
			c.expr(e.Right)
			c.binaryOp(e.Op)
		}

	case *ast.IncrExpr:
		op := Add
		if e.Op == lexer.DECR {
			op = Subtract
		}
		if e.Pre {
			c.expr(e.Expr)
			c.expr(&ast.NumExpr{1})
			c.add(op)
			c.add(Dupe)
		} else {
			c.expr(e.Expr)
			c.expr(&ast.NumExpr{0})
			c.add(Add)
			c.add(Dupe)
			c.expr(&ast.NumExpr{1})
			c.add(op)
		}
		c.assign(e.Expr)

	case *ast.AssignExpr:
		c.expr(e.Right)
		c.add(Dupe)
		c.assign(e.Left)

	case *ast.AugAssignExpr:
		c.expr(e.Right)
		c.expr(e.Left)
		c.add(Swap)
		c.binaryOp(e.Op)
		c.add(Dupe)
		c.assign(e.Left)

	case *ast.CondExpr:
		jumpOp := c.cond(e.Cond, true)
		ifMark := c.jumpForward(jumpOp)
		c.expr(e.True)
		elseMark := c.jumpForward(Jump)
		c.patchForward(ifMark)
		c.expr(e.False)
		c.patchForward(elseMark)

	case *ast.IndexExpr:
		c.index(e.Index)
		switch e.Array.Scope {
		case ast.ScopeGlobal:
			c.add(ArrayGlobal, opcodeInt(e.Array.Index))
		case ast.ScopeLocal:
			c.add(ArrayLocal, opcodeInt(e.Array.Index))
		}

	case *ast.CallExpr:
		switch e.Func {
		case lexer.F_SPLIT:
			c.expr(e.Args[0])
			arrayExpr := e.Args[1].(*ast.ArrayExpr)
			switch {
			case arrayExpr.Scope == ast.ScopeGlobal && len(e.Args) > 2:
				c.expr(e.Args[2])
				c.add(CallSplitSepGlobal, opcodeInt(arrayExpr.Index))
			case arrayExpr.Scope == ast.ScopeGlobal:
				c.add(CallSplitGlobal, opcodeInt(arrayExpr.Index))
			case arrayExpr.Scope == ast.ScopeLocal && len(e.Args) > 2:
				c.expr(e.Args[2])
				c.add(CallSplitSepLocal, opcodeInt(arrayExpr.Index))
			case arrayExpr.Scope == ast.ScopeLocal:
				c.add(CallSplitLocal, opcodeInt(arrayExpr.Index))
			default:
				panic(fmt.Sprintf("unexpected array scope %d or num args %d", arrayExpr.Scope, len(e.Args)))
			}
			return
		case lexer.F_SUB, lexer.F_GSUB:
			op := CallSub
			if e.Func == lexer.F_GSUB {
				op = CallGsub
			}
			var target ast.Expr = &ast.FieldExpr{&ast.NumExpr{0}} // default value and target is $0
			if len(e.Args) == 3 {
				target = e.Args[2]
			}
			c.expr(e.Args[0])
			c.expr(e.Args[1])
			c.expr(target)
			c.add(op)
			c.assign(target)
			return
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
			c.add(CallSprintf, opcodeInt(len(e.Args)))
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
			panic(fmt.Sprintf("unexpected function: %s", e.Func))
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
		c.index(e.Index)
		switch e.Array.Scope {
		case ast.ScopeGlobal:
			c.add(InGlobal, opcodeInt(e.Array.Index))
		case ast.ScopeLocal:
			c.add(InLocal, opcodeInt(e.Array.Index))
		}

	case *ast.UserCallExpr:
		if e.Native {
			for _, arg := range e.Args {
				c.expr(arg)
			}
			c.add(CallNative, opcodeInt(e.Index), opcodeInt(len(e.Args)))
			for len(c.program.nativeFuncNames) <= e.Index {
				c.program.nativeFuncNames = append(c.program.nativeFuncNames, "")
			}
			c.program.nativeFuncNames[e.Index] = e.Name
		} else {
			f := c.program.Functions[e.Index]
			var arrayOpcodes []Opcode
			numScalarArgs := 0
			for i, arg := range e.Args {
				if f.Arrays[i] {
					a := arg.(*ast.VarExpr)
					arrayOpcodes = append(arrayOpcodes, Opcode(a.Scope), opcodeInt(a.Index))
				} else {
					c.expr(arg)
					numScalarArgs++
				}
			}
			if numScalarArgs < f.NumScalars {
				c.add(Nulls, opcodeInt(f.NumScalars-numScalarArgs))
			}
			c.add(CallUser, opcodeInt(e.Index), opcodeInt(len(arrayOpcodes)/2))
			c.add(arrayOpcodes...)
		}

	case *ast.GetlineExpr:
		redirect := func() Opcode {
			switch {
			case e.Command != nil:
				c.expr(e.Command)
				return Opcode(lexer.PIPE)
			case e.File != nil:
				c.expr(e.File)
				return Opcode(lexer.LESS)
			default:
				return Opcode(lexer.ILLEGAL)
			}
		}
		switch target := e.Target.(type) {
		case *ast.VarExpr:
			switch target.Scope {
			case ast.ScopeGlobal:
				c.add(GetlineGlobal, redirect(), opcodeInt(target.Index))
			case ast.ScopeLocal:
				c.add(GetlineLocal, redirect(), opcodeInt(target.Index))
			case ast.ScopeSpecial:
				c.add(GetlineSpecial, redirect(), opcodeInt(target.Index))
			}
		case *ast.FieldExpr:
			c.expr(target.Index)
			c.add(GetlineField, redirect())
		case *ast.IndexExpr:
			c.index(target.Index)
			switch target.Array.Scope {
			case ast.ScopeGlobal:
				c.add(GetlineArrayGlobal, redirect(), opcodeInt(target.Array.Index))
			case ast.ScopeLocal:
				c.add(GetlineArrayLocal, redirect(), opcodeInt(target.Array.Index))
			}
		default:
			c.add(Getline, redirect())
		}

	default:
		// Should never happen
		panic(fmt.Sprintf("unexpected expr type: %T", expr))
	}
}

// numIndex adds (or reuses) a number constant and returns its index.
func (c *compiler) numIndex(n float64) int {
	if index, ok := c.program.numIndexes[n]; ok {
		return index // reuse existing constant
	}
	index := len(c.program.Nums)
	c.program.Nums = append(c.program.Nums, n)
	if c.program.numIndexes == nil {
		c.program.numIndexes = make(map[float64]int)
	}
	c.program.numIndexes[n] = index
	return index
}

// strIndex adds (or reuses) a string constant and returns its index.
func (c *compiler) strIndex(s string) int {
	if index, ok := c.program.strIndexes[s]; ok {
		return index // reuse existing constant
	}
	index := len(c.program.Strs)
	c.program.Strs = append(c.program.Strs, s)
	if c.program.strIndexes == nil {
		c.program.strIndexes = make(map[string]int)
	}
	c.program.strIndexes[s] = index
	return index
}

// regexIndex adds (or reuses) a regex constant and returns its index.
func (c *compiler) regexIndex(r string) int {
	if index, ok := c.program.regexIndexes[r]; ok {
		return index // reuse existing constant
	}
	index := len(c.program.Regexes)
	c.program.Regexes = append(c.program.Regexes, regexp.MustCompile(r))
	if c.program.regexIndexes == nil {
		c.program.regexIndexes = make(map[string]int)
	}
	c.program.regexIndexes[r] = index
	return index
}

func (c *compiler) binaryOp(op lexer.Token) {
	var opcode Opcode
	switch op {
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
		panic(fmt.Sprintf("unexpected binary operation: %s", op))
	}
	c.add(opcode)
}

func (c *compiler) index(index []ast.Expr) {
	for _, expr := range index {
		c.expr(expr)
	}
	if len(index) > 1 {
		c.add(MultiIndex, opcodeInt(len(index)))
	}
}
