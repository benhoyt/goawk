// Package compiler compiles an AST to virtual machine instructions.
package compiler

import (
	"fmt"
	"math"
	"regexp"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/lexer"
)

// Program holds an entire compiled program.
type Program struct {
	Begin     []Opcode
	Actions   []Action
	End       []Opcode
	Functions []Function
	Nums      []float64
	Strs      []string
	Regexes   []*regexp.Regexp

	// For disassembly
	scalarNames     []string
	arrayNames      []string
	nativeFuncNames []string
}

// Action holds a compiled pattern-action block.
type Action struct {
	Pattern [][]Opcode
	Body    []Opcode
}

// Function holds a compiled function.
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

// Compile compiles an AST (parsed program) into virtual machine instructions.
func Compile(prog *ast.Program) (compiledProg *Program, err error) {
	defer func() {
		// The compiler uses panic with a *compileError to signal compile
		// errors internally, and they're caught here. This avoids the
		// need to check errors everywhere.
		if r := recover(); r != nil {
			// Convert to compileError or re-panic
			err = r.(*compileError)
		}
	}()

	p := &Program{}

	// Reuse identical constants across entire program.
	indexes := constantIndexes{
		nums:    make(map[float64]int),
		strs:    make(map[string]int),
		regexes: make(map[string]int),
	}

	// Compile functions. For functions called before they're defined or
	// recursive functions, we have to set most p.Functions data first, then
	// compile Body afterward.
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
		c := &compiler{program: p, indexes: indexes}
		c.stmts(astFunc.Body)
		p.Functions[i].Body = c.finish()
	}

	// Compile BEGIN blocks.
	for _, stmts := range prog.Begin {
		c := &compiler{program: p, indexes: indexes}
		c.stmts(stmts)
		p.Begin = append(p.Begin, c.finish()...)
	}

	// Compile pattern-action blocks.
	for _, action := range prog.Actions {
		var pattern [][]Opcode
		switch len(action.Pattern) {
		case 0:
			// Always considered a match
		case 1:
			c := &compiler{program: p, indexes: indexes}
			c.expr(action.Pattern[0])
			pattern = [][]Opcode{c.finish()}
		case 2:
			c := &compiler{program: p, indexes: indexes}
			c.expr(action.Pattern[0])
			pattern = append(pattern, c.finish())
			c = &compiler{program: p, indexes: indexes}
			c.expr(action.Pattern[1])
			pattern = append(pattern, c.finish())
		}
		var body []Opcode
		if len(action.Stmts) > 0 {
			c := &compiler{program: p, indexes: indexes}
			c.stmts(action.Stmts)
			body = c.finish()
		}
		p.Actions = append(p.Actions, Action{
			Pattern: pattern,
			Body:    body,
		})
	}

	// Compile END blocks.
	for _, stmts := range prog.End {
		c := &compiler{program: p, indexes: indexes}
		c.stmts(stmts)
		p.End = append(p.End, c.finish()...)
	}

	// These are only used for disassembly, but set them up here.
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

// So we can look up the indexes of constants that have been used before.
type constantIndexes struct {
	nums    map[float64]int
	strs    map[string]int
	regexes map[string]int
}

// Holds the compilation state.
type compiler struct {
	program   *Program
	indexes   constantIndexes
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
		// Optimize assignment expressions to avoid the extra Dupe and Drop
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
					c.add(IncrGlobal, incrAmount(expr.Op), opcodeInt(target.Index))
				case ast.ScopeLocal:
					c.add(IncrLocal, incrAmount(expr.Op), opcodeInt(target.Index))
				default: // ScopeSpecial
					c.add(IncrSpecial, incrAmount(expr.Op), opcodeInt(target.Index))
				}
			case *ast.FieldExpr:
				c.expr(target.Index)
				c.add(IncrField, incrAmount(expr.Op))
			case *ast.IndexExpr:
				c.index(target.Index)
				switch target.Array.Scope {
				case ast.ScopeGlobal:
					c.add(IncrArrayGlobal, incrAmount(expr.Op), opcodeInt(target.Array.Index))
				default: // ScopeLocal
					c.add(IncrArrayLocal, incrAmount(expr.Op), opcodeInt(target.Array.Index))
				}
			}
			return

		case *ast.AugAssignExpr:
			c.expr(expr.Right)

			var augOp AugOp
			switch expr.Op {
			case lexer.ADD:
				augOp = AugOpAdd
			case lexer.SUB:
				augOp = AugOpSub
			case lexer.MUL:
				augOp = AugOpMul
			case lexer.DIV:
				augOp = AugOpDiv
			case lexer.POW:
				augOp = AugOpPow
			default: // MOD
				augOp = AugOpMod
			}

			switch target := expr.Left.(type) {
			case *ast.VarExpr:
				switch target.Scope {
				case ast.ScopeGlobal:
					c.add(AugAssignGlobal, Opcode(augOp), opcodeInt(target.Index))
				case ast.ScopeLocal:
					c.add(AugAssignLocal, Opcode(augOp), opcodeInt(target.Index))
				default: // ScopeSpecial
					c.add(AugAssignSpecial, Opcode(augOp), opcodeInt(target.Index))
				}
			case *ast.FieldExpr:
				c.expr(target.Index)
				c.add(AugAssignField, Opcode(augOp))
			case *ast.IndexExpr:
				c.index(target.Index)
				switch target.Array.Scope {
				case ast.ScopeGlobal:
					c.add(AugAssignArrayGlobal, Opcode(augOp), opcodeInt(target.Array.Index))
				default: // ScopeLocal
					c.add(AugAssignArrayLocal, Opcode(augOp), opcodeInt(target.Array.Index))
				}
			}
			return
		}

		// Non-optimized ExprStmt: push value and then drop it
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
			jumpOp := c.condition(s.Cond, true)
			ifMark := c.jumpForward(jumpOp)
			c.stmts(s.Body)
			c.patchForward(ifMark)
		} else {
			jumpOp := c.condition(s.Cond, true)
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

		// Optimization: include condition once before loop and at the end.
		// This avoids one jump (a conditional jump at the top and an
		// unconditional one at the end). This idea was stolen from an
		// optimization CPython did recently in its "while" loop.
		var mark int
		if s.Cond != nil {
			jumpOp := c.condition(s.Cond, true)
			mark = c.jumpForward(jumpOp)
		}

		loopStart := c.labelBackward()
		c.stmts(s.Body)
		c.patchContinues()
		if s.Post != nil {
			c.stmt(s.Post)
		}

		if s.Cond != nil {
			jumpOp := c.condition(s.Cond, false)
			c.jumpBackward(loopStart, jumpOp)
			c.patchForward(mark)
		} else {
			c.jumpBackward(loopStart, Jump)
		}

		c.patchBreaks()

	case *ast.ForInStmt:
		// ForIn is handled a bit differently from the other loops, because we
		// want to use Go's "for range" construct directly in the interpreter.
		// Otherwise we'd need to build a slice of all keys rather than
		// iterating, or write our own hash table that has a more flexible
		// iterator.
		mark := c.jumpForward(ForIn, opcodeInt(int(s.Var.Scope)), opcodeInt(s.Var.Index),
			Opcode(s.Array.Scope), opcodeInt(s.Array.Index))

		c.breaks = append(c.breaks, nil) // nil tells BreakStmt it's a for-in loop
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

		// Optimization: include condition once before loop and at the end.
		// See ForStmt for more details.
		jumpOp := c.condition(s.Cond, true)
		mark := c.jumpForward(jumpOp)

		loopStart := c.labelBackward()
		c.stmts(s.Body)
		c.patchContinues()

		jumpOp = c.condition(s.Cond, false)
		c.jumpBackward(loopStart, jumpOp)
		c.patchForward(mark)

		c.patchBreaks()

	case *ast.DoWhileStmt:
		c.breaks = append(c.breaks, []int{})
		c.continues = append(c.continues, []int{})

		loopStart := c.labelBackward()
		c.stmts(s.Body)
		c.patchContinues()

		jumpOp := c.condition(s.Cond, false)
		c.jumpBackward(loopStart, jumpOp)

		c.patchBreaks()

	case *ast.BreakStmt:
		i := len(c.breaks) - 1
		if c.breaks[i] == nil {
			// Break in for-in loop is executed differently, use errBreak to exit
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
			c.add(Delete, Opcode(s.Array.Scope), opcodeInt(s.Array.Index))
		} else {
			c.add(DeleteAll, Opcode(s.Array.Scope), opcodeInt(s.Array.Index))
		}

	case *ast.BlockStmt:
		c.stmts(s.Body)

	default:
		// Should never happen
		panic(fmt.Sprintf("unexpected stmt type: %T", stmt))
	}
}

// Return the amount (+1 or -1) to add for an increment expression.
func incrAmount(op lexer.Token) Opcode {
	if op == lexer.INCR {
		return 1
	} else {
		return -1 // DECR
	}
}

// Generate opcodes for an assignment.
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

// Convert int to Opcode, raising a *compileError if it doesn't fit.
func opcodeInt(n int) Opcode {
	if n > math.MaxInt32 || n < math.MinInt32 {
		// Two billion should be enough for anybody.
		panic(&compileError{message: fmt.Sprintf("program too large (constant index or jump offset %d doesn't fit in int32)", n)})
	}
	return Opcode(n)
}

// Patch jump addresses for break statements in a loop.
func (c *compiler) patchBreaks() {
	breaks := c.breaks[len(c.breaks)-1]
	for _, mark := range breaks {
		c.patchForward(mark)
	}
	c.breaks = c.breaks[:len(c.breaks)-1]
}

// Patch jump addresses for continue statements in a loop
func (c *compiler) patchContinues() {
	continues := c.continues[len(c.continues)-1]
	for _, mark := range continues {
		c.patchForward(mark)
	}
	c.continues = c.continues[:len(c.continues)-1]
}

// Generate a forward jump (patched later) and return a "mark".
func (c *compiler) jumpForward(jumpOp Opcode, args ...Opcode) int {
	c.add(jumpOp)
	c.add(args...)
	c.add(0)
	return len(c.code)
}

// Patch a previously-generated forward jump.
func (c *compiler) patchForward(mark int) {
	offset := len(c.code) - mark
	c.code[mark-1] = opcodeInt(offset)
}

// Return a "label" for a subsequent backward jump.
func (c *compiler) labelBackward() int {
	return len(c.code)
}

// Jump to a previously-created label.
func (c *compiler) jumpBackward(label int, jumpOp Opcode, args ...Opcode) {
	offset := label - (len(c.code) + len(args) + 2)
	c.add(jumpOp)
	c.add(args...)
	c.add(opcodeInt(offset))
}

// Generate opcodes for a boolean condition.
func (c *compiler) condition(expr ast.Expr, invert bool) Opcode {
	jumpOp := func(normal, inverted Opcode) Opcode {
		if invert {
			return inverted
		}
		return normal
	}

	switch cond := expr.(type) {
	case *ast.BinaryExpr:
		// Optimize binary comparison expressions like "x < 10" into just
		// JumpLess instead of two instructions (Less and JumpTrue).
		switch cond.Op {
		case lexer.EQUALS:
			c.expr(cond.Left)
			c.expr(cond.Right)
			return jumpOp(JumpEquals, JumpNotEquals)

		case lexer.NOT_EQUALS:
			c.expr(cond.Left)
			c.expr(cond.Right)
			return jumpOp(JumpNotEquals, JumpEquals)

		case lexer.LESS:
			c.expr(cond.Left)
			c.expr(cond.Right)
			return jumpOp(JumpLess, JumpGreaterOrEqual)

		case lexer.LTE:
			c.expr(cond.Left)
			c.expr(cond.Right)
			return jumpOp(JumpLessOrEqual, JumpGreater)

		case lexer.GREATER:
			c.expr(cond.Left)
			c.expr(cond.Right)
			return jumpOp(JumpGreater, JumpLessOrEqual)

		case lexer.GTE:
			c.expr(cond.Left)
			c.expr(cond.Right)
			return jumpOp(JumpGreaterOrEqual, JumpLess)
		}
	}

	// Fall back to evaluating the expression normally, followed by JumpTrue
	// or JumpFalse.
	c.expr(expr)
	return jumpOp(JumpTrue, JumpFalse)
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
			if index.Value == float64(Opcode(index.Value)) {
				// Optimize $i to FieldInt opcode with integer argument
				c.add(FieldInt, opcodeInt(int(index.Value)))
				return
			}
		}
		c.expr(e.Index)
		c.add(Field)

	case *ast.NamedFieldExpr:
		switch index := e.Field.(type) {
		case *ast.StrExpr:
			c.add(FieldByNameStr, opcodeInt(c.strIndex(index.Value)))
			return
		}
		c.expr(e.Field)
		c.add(FieldByName)

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
		case lexer.CONCAT:
			c.concatOp(e)
		default:
			// All other binary expressions
			c.expr(e.Left)
			c.expr(e.Right)
			c.binaryOp(e.Op)
		}

	case *ast.IncrExpr:
		// Most IncrExpr (standalone) will be handled by the ExprStmt special case
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
		// Most AssignExpr (standalone) will be handled by the ExprStmt special case
		c.expr(e.Right)
		c.add(Dupe)
		c.assign(e.Left)

	case *ast.AugAssignExpr:
		// Most AugAssignExpr (standalone) will be handled by the ExprStmt special case
		c.expr(e.Right)
		c.expr(e.Left)
		c.add(Swap)
		c.binaryOp(e.Op)
		c.add(Dupe)
		c.assign(e.Left)

	case *ast.CondExpr:
		jump := c.condition(e.Cond, true)
		ifMark := c.jumpForward(jump)
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
		// split and sub/gsub require special cases as they have lvalue arguments
		switch e.Func {
		case lexer.F_SPLIT:
			c.expr(e.Args[0])
			arrayExpr := e.Args[1].(*ast.ArrayExpr)
			if len(e.Args) > 2 {
				c.expr(e.Args[2])
				c.add(CallSplitSep, Opcode(arrayExpr.Scope), opcodeInt(arrayExpr.Index))
			} else {
				c.add(CallSplit, Opcode(arrayExpr.Scope), opcodeInt(arrayExpr.Index))
			}
			return
		case lexer.F_SUB, lexer.F_GSUB:
			op := BuiltinSub
			if e.Func == lexer.F_GSUB {
				op = BuiltinGsub
			}
			var target ast.Expr = &ast.FieldExpr{&ast.NumExpr{0}} // default value and target is $0
			if len(e.Args) == 3 {
				target = e.Args[2]
			}
			c.expr(e.Args[0])
			c.expr(e.Args[1])
			c.expr(target)
			c.add(CallBuiltin, Opcode(op))
			c.assign(target)
			return
		}

		for _, arg := range e.Args {
			c.expr(arg)
		}
		switch e.Func {
		case lexer.F_ATAN2:
			c.add(CallBuiltin, Opcode(BuiltinAtan2))
		case lexer.F_CLOSE:
			c.add(CallBuiltin, Opcode(BuiltinClose))
		case lexer.F_COS:
			c.add(CallBuiltin, Opcode(BuiltinCos))
		case lexer.F_EXP:
			c.add(CallBuiltin, Opcode(BuiltinExp))
		case lexer.F_FFLUSH:
			if len(e.Args) > 0 {
				c.add(CallBuiltin, Opcode(BuiltinFflush))
			} else {
				c.add(CallBuiltin, Opcode(BuiltinFflushAll))
			}
		case lexer.F_INDEX:
			c.add(CallBuiltin, Opcode(BuiltinIndex))
		case lexer.F_INT:
			c.add(CallBuiltin, Opcode(BuiltinInt))
		case lexer.F_LENGTH:
			if len(e.Args) > 0 {
				c.add(CallBuiltin, Opcode(BuiltinLengthArg))
			} else {
				c.add(CallBuiltin, Opcode(BuiltinLength))
			}
		case lexer.F_LOG:
			c.add(CallBuiltin, Opcode(BuiltinLog))
		case lexer.F_MATCH:
			c.add(CallBuiltin, Opcode(BuiltinMatch))
		case lexer.F_RAND:
			c.add(CallBuiltin, Opcode(BuiltinRand))
		case lexer.F_SIN:
			c.add(CallBuiltin, Opcode(BuiltinSin))
		case lexer.F_SPRINTF:
			c.add(CallSprintf, opcodeInt(len(e.Args)))
		case lexer.F_SQRT:
			c.add(CallBuiltin, Opcode(BuiltinSqrt))
		case lexer.F_SRAND:
			if len(e.Args) > 0 {
				c.add(CallBuiltin, Opcode(BuiltinSrandSeed))
			} else {
				c.add(CallBuiltin, Opcode(BuiltinSrand))
			}
		case lexer.F_SUBSTR:
			if len(e.Args) > 2 {
				c.add(CallBuiltin, Opcode(BuiltinSubstrLength))
			} else {
				c.add(CallBuiltin, Opcode(BuiltinSubstr))
			}
		case lexer.F_SYSTEM:
			c.add(CallBuiltin, Opcode(BuiltinSystem))
		case lexer.F_TOLOWER:
			c.add(CallBuiltin, Opcode(BuiltinTolower))
		case lexer.F_TOUPPER:
			c.add(CallBuiltin, Opcode(BuiltinToupper))
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
		default: // ADD
			c.add(UnaryPlus)
		}

	case *ast.InExpr:
		c.index(e.Index)
		switch e.Array.Scope {
		case ast.ScopeGlobal:
			c.add(InGlobal, opcodeInt(e.Array.Index))
		default: // ScopeLocal
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
			c.add(GetlineArray, redirect(), Opcode(target.Array.Scope), opcodeInt(target.Array.Index))
		default:
			c.add(Getline, redirect())
		}

	default:
		// Should never happen
		panic(fmt.Sprintf("unexpected expr type: %T", expr))
	}
}

// Generate a Concat opcode or, if possible, compact multiple Concats into one
// ConcatMulti opcode.
func (c *compiler) concatOp(expr *ast.BinaryExpr) {
	var values []ast.Expr
	for {
		values = append(values, expr.Right)
		left, isBinary := expr.Left.(*ast.BinaryExpr)
		if !isBinary || left.Op != lexer.CONCAT {
			break
		}
		expr = left
	}
	values = append(values, expr.Left)

	// values are appended right to left
	// but need to pushed left to right

	if len(values) == 2 {
		c.expr(values[1])
		c.expr(values[0])
		c.add(Concat)
		return
	}

	for i := len(values) - 1; i >= 0; i-- {
		c.expr(values[i])
	}

	c.add(ConcatMulti, opcodeInt(len(values)))
}

// Add (or reuse) a number constant and returns its index.
func (c *compiler) numIndex(n float64) int {
	if index, ok := c.indexes.nums[n]; ok {
		return index // reuse existing constant
	}
	index := len(c.program.Nums)
	c.program.Nums = append(c.program.Nums, n)
	c.indexes.nums[n] = index
	return index
}

// Add (or reuse) a string constant and returns its index.
func (c *compiler) strIndex(s string) int {
	if index, ok := c.indexes.strs[s]; ok {
		return index // reuse existing constant
	}
	index := len(c.program.Strs)
	c.program.Strs = append(c.program.Strs, s)
	c.indexes.strs[s] = index
	return index
}

// Add (or reuse) a regex constant and returns its index.
func (c *compiler) regexIndex(r string) int {
	if index, ok := c.indexes.regexes[r]; ok {
		return index // reuse existing constant
	}
	index := len(c.program.Regexes)
	c.program.Regexes = append(c.program.Regexes, regexp.MustCompile(AddRegexFlags(r)))
	c.indexes.regexes[r] = index
	return index
}

// AddRegexFlags add the necessary flags to regex to make it work like other
// AWKs (exported so we can also use this in the interpreter).
func AddRegexFlags(regex string) string {
	// "s" flag lets . match \n (multi-line matching like other AWKs)
	return "(?s:" + regex + ")"
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

// Generate an array index, handling multi-indexes properly.
func (c *compiler) index(index []ast.Expr) {
	for _, expr := range index {
		c.expr(expr)
	}
	if len(index) > 1 {
		c.add(IndexMulti, opcodeInt(len(index)))
	}
}
