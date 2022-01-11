// AWKGo: the main compiler

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"

	"github.com/benhoyt/goawk/internal/ast"
	. "github.com/benhoyt/goawk/lexer"
)

// compile compiles the parsed AWK program to Go and outputs to writer.
func compile(prog *ast.Program, writer io.Writer) (err error) {
	// Both typer and compiler signal errors internally with
	// panic(&errorExit{...}), so recover and return an error.
	defer func() {
		r := recover()
		switch r := r.(type) {
		case nil:
		case *exitError:
			err = errors.New(r.message)
		default:
			panic(r) // another type, re-panic
		}
	}()

	// Determine the types of variables and expressions.
	t := newTyper()
	t.program(prog)

	// Do a second typing pass over the program to ensure we detect the types
	// of variables assigned after they're used, for example:
	// BEGIN { while (i<5) { i++; print i } }
	t.program(prog)

	c := &compiler{
		typer:   t,
		writer:  writer,
		regexen: make(map[string]int),
	}
	c.program(prog)

	return nil
}

type exitError struct {
	message string
}

func (e *exitError) Error() string {
	return e.message
}

func errorf(format string, args ...interface{}) error {
	return &exitError{fmt.Sprintf(format, args...)}
}

type compiler struct {
	typer   *typer
	writer  io.Writer
	regexen map[string]int
}

func (c *compiler) output(s string) {
	fmt.Fprint(c.writer, s)
}

func (c *compiler) outputf(format string, args ...interface{}) {
	fmt.Fprintf(c.writer, format, args...)
}

func (c *compiler) program(prog *ast.Program) {
	c.output(`package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	_output  *bufio.Writer
	_scanner *bufio.Scanner
	_line    string
	_fields  []string
	_lineNum int
	_seed    float64
	_rand    *rand.Rand

`)

	globals := make([]string, 0, len(c.typer.globals))
	for name := range c.typer.globals {
		globals = append(globals, name)
	}
	sort.Strings(globals)
	for _, name := range globals {
		typ := c.typer.globals[name]
		c.outputf("%s %s\n", name, c.goType(typ))
	}

	c.output(`)

func main() {
	_output = bufio.NewWriter(os.Stdout)
	defer _output.Flush()

	_scanner = bufio.NewScanner(os.Stdin)
	_seed = 1.0
	_rand = rand.New(rand.NewSource(int64(math.Float64bits(_seed))))

	FS = " "
	OFS = " "
	ORS = "\n"
	OFMT = "%.6g"
	CONVFMT = "%.6g"
	SUBSEP = "\x1c"

`)

	for i, action := range prog.Actions {
		if len(action.Pattern) == 2 {
			// Booleans to keep track of range pattern state
			c.outputf("_inRange%d := false\n", i)
		}
	}

	for _, name := range globals {
		typ := c.typer.globals[name]
		switch typ {
		case typeArrayStr, typeArrayNum:
			c.outputf("%s = make(%s)\n", name, c.goType(typ))
		}
	}

	for _, stmts := range prog.Begin {
		c.output("\n")
		c.stmts(stmts)
	}

	if c.typer.nextUsed {
		c.output("_nextLine:\n")
	}

	if len(prog.Actions) > 0 {
		c.output(`
	for _scanner.Scan() {
		_lineNum++
		_line = _scanner.Text()
		_fields = _splitHelper(_line, FS)
`)
		c.actions(prog.Actions)
		c.output(`	}

	if _scanner.Err() != nil {
		fmt.Fprintln(os.Stderr, _scanner.Err())
		os.Exit(1)
	}
`)
	}

	for _, stmts := range prog.End {
		c.output("\n")
		c.stmts(stmts)
	}

	c.output("}\n")

	type regex struct {
		pattern string
		n       int
	}
	var regexen []regex
	for pattern, n := range c.regexen {
		regexen = append(regexen, regex{pattern, n})
	}
	sort.Slice(regexen, func(i, j int) bool {
		return regexen[i].n < regexen[j].n
	})
	if len(regexen) > 0 {
		c.output("\nvar (\n")
		for _, r := range regexen {
			c.outputf("_re%d = regexp.MustCompile(%q)\n", r.n, r.pattern)
		}
		c.output(")\n")
	}

	c.outputHelpers()
}

func (c *compiler) actions(actions []ast.Action) {
	for i, action := range actions {
		c.output("\n")
		switch len(action.Pattern) {
		case 0:
			// No pattern is equivalent to pattern evaluating to true
		case 1:
			// Single boolean pattern
			c.outputf("if %s {\n", c.cond(action.Pattern[0]))
		case 2:
			// Range pattern (matches between start and stop lines)
			c.outputf("if !_inRange%d { _inRange%d = %s }\n", i, i, c.cond(action.Pattern[0]))
			c.outputf("if _inRange%d {\n", i)
		}

		if len(action.Stmts) == 0 {
			// No action is equivalent to { print $0 }
			c.output("fmt.Fprint(_output, _line, ORS)\n")
		} else {
			c.stmts(action.Stmts)
		}

		switch len(action.Pattern) {
		case 1:
			c.output("}\n")
		case 2:
			c.outputf("\n_inRange%d = !(%s)\n", i, c.cond(action.Pattern[1]))
			c.output("}\n")
		}
	}
}

func (c *compiler) stmts(stmts ast.Stmts) {
	for _, stmt := range stmts {
		c.stmt(stmt)
	}
}

func (c *compiler) assign(left, right ast.Expr) string {
	switch left := left.(type) {
	case *ast.VarExpr:
		if left.Scope == ast.ScopeSpecial {
			switch left.Index {
			case ast.V_NF, ast.V_NR, ast.V_FNR:
				panic(errorf("can't assign to special variable %s", left.Name))
			}
		}
		return fmt.Sprintf("%s = %s", left.Name, c.expr(right))
	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s] = %s", left.Array.Name, c.index(left.Index), c.expr(right))
	case *ast.FieldExpr:
		return fmt.Sprintf("_setField(%s, %s)", c.intExpr(left.Index), c.strExpr(right))
	default:
		panic(errorf("expected lvalue, not %s", left))
	}
}

func (c *compiler) stmtNoNewline(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		switch e := s.Expr.(type) {
		case *ast.AssignExpr:
			c.output(c.assign(e.Left, e.Right))

		case *ast.AugAssignExpr:
			switch left := e.Left.(type) {
			case *ast.VarExpr:
				switch e.Op {
				case MOD, POW:
					c.output(left.Name)
					if e.Op == MOD {
						c.output(" = math.Mod(")
					} else {
						c.output(" = math.Pow(")
					}
					c.outputf("%s, %s)", left.Name, c.numExpr(e.Right))
				default:
					c.outputf("%s %s= %s", left.Name, e.Op, c.numExpr(e.Right))
				}

			case *ast.IndexExpr:
				switch e.Op {
				case MOD, POW:
					c.outputf("%s[%s", left.Array.Name, c.index(left.Index))
					if e.Op == MOD {
						c.output("] = math.Mod(")
					} else {
						c.output("] = math.Pow(")
					}
					c.outputf("%s[%s], %s)", left.Array.Name, c.index(left.Index), c.numExpr(e.Right))
				default:
					c.outputf("%s[%s] %s= %s", left.Array.Name, c.index(left.Index), e.Op, c.numExpr(e.Right))
				}

			case *ast.FieldExpr:
				// We have to be careful not to evaluate left.Index twice, in
				// case it has side effects, as in: $(y++) += 10
				switch e.Op {
				case MOD, POW:
					c.outputf("func() { _t := %s; _setField(_t, _numToStr(", c.intExpr(left.Index))
					if e.Op == MOD {
						c.output("math.Mod(")
					} else {
						c.output("math.Pow(")
					}
					c.outputf("_strToNum(_getField(_t)), %s))) }()", c.numExpr(e.Right))
				default:
					c.outputf("func() { _t := %s; _setField(_t, _numToStr(_strToNum(_getField(_t)) %s %s)) }()",
						c.intExpr(left.Index), e.Op, c.numExpr(e.Right))
				}
			}

		case *ast.IncrExpr:
			switch left := e.Expr.(type) {
			case *ast.VarExpr:
				c.outputf("%s%s", left.Name, e.Op)
			case *ast.IndexExpr:
				c.outputf("%s[%s]%s", left.Array.Name, c.index(left.Index), e.Op)
			case *ast.FieldExpr:
				// We have to be careful not to evaluate left.Index twice, in
				// case it has side effects, as in: $(y++)++
				op := "+"
				if e.Op == DECR {
					op = "-"
				}
				c.outputf("func() { _t := %s; _setField(_t, _numToStr(_strToNum(_getField(_t)) %s 1)) }()",
					c.intExpr(left.Index), op)
			}

		default:
			c.outputf("_ = %s", c.expr(s.Expr))
		}

	case *ast.PrintStmt:
		if s.Dest != nil {
			panic(errorf("print redirection not yet supported"))
		}
		if c.typer.oFSRSChanged {
			c.output("fmt.Fprint(_output, ")
		} else {
			c.output("fmt.Fprintln(_output, ")
		}
		if len(s.Args) > 0 {
			for i, arg := range s.Args {
				if i > 0 {
					c.output(", ")
					if c.typer.oFSRSChanged {
						c.output("OFS, ")
					}
				}
				str := c.expr(arg)
				if c.typer.exprs[arg] == typeNum {
					str = fmt.Sprintf("_formatNum(%s)", str)
				}
				c.output(str)
			}
		} else {
			// "print" with no args is equivalent to "print $0"
			c.output("_line")
		}
		if c.typer.oFSRSChanged {
			c.output(", ORS")
		}
		c.output(")")

	case *ast.PrintfStmt:
		if s.Dest != nil {
			panic(errorf("printf redirection not yet supported"))
		}
		formatExpr, ok := s.Args[0].(*ast.StrExpr)
		if !ok {
			panic(errorf("printf currently only supports literal format strings"))
		}
		args := c.printfArgs(formatExpr.Value, s.Args[1:])
		c.outputf("fmt.Fprintf(_output, %q", formatExpr.Value)
		for _, arg := range args {
			c.outputf(", %s", arg)
		}
		c.output(")")

	case *ast.IfStmt:
		c.output("if ")
		switch cond := s.Cond.(type) {
		case *ast.InExpr:
			// if _, _ok := a[k]; ok { ... }
			c.outputf("_, _ok := %s[%s]; _ok ", cond.Array.Name, c.index(cond.Index))
		default:
			c.output(c.cond(s.Cond))
		}
		c.output(" {\n")
		c.stmts(s.Body)
		c.output("}")
		if len(s.Else) > 0 {
			if _, isIf := s.Else[0].(*ast.IfStmt); isIf && len(s.Else) == 1 {
				// Simplify runs if-else if
				c.output(" else ")
				c.stmt(s.Else[0])
			} else {
				c.output(" else {\n")
				c.stmts(s.Else)
				c.output("}")
			}
		}

	case *ast.ForStmt:
		c.output("for ")
		if s.Pre != nil {
			_, ok := s.Pre.(*ast.ExprStmt)
			if !ok {
				panic(errorf(`only expressions are allowed in "for" initializer`))
			}
			c.stmtNoNewline(s.Pre)
		}
		c.output("; ")
		if s.Cond != nil {
			c.output(c.cond(s.Cond))
		}
		c.output("; ")
		if s.Post != nil {
			_, ok := s.Post.(*ast.ExprStmt)
			if !ok {
				panic(errorf(`only expressions are allowed in "for" post expression`))
			}
			c.stmtNoNewline(s.Post)
		}
		c.output(" {\n")
		c.stmts(s.Body)
		c.output("}")

	case *ast.ForInStmt:
		c.outputf("for %s = range %s {\n", s.Var.Name, s.Array.Name)
		c.stmts(s.Body)
		c.output("}")

	case *ast.WhileStmt:
		c.outputf("for %s {\n", c.cond(s.Cond))
		c.stmts(s.Body)
		c.output("}")

	case *ast.DoWhileStmt:
		c.output("for {\n")
		c.stmts(s.Body)
		c.outputf("if !(%s) {\nbreak\n}\n}", c.cond(s.Cond))

	case *ast.BreakStmt:
		c.output("break")

	case *ast.ContinueStmt:
		c.output("continue")

	case *ast.NextStmt:
		c.output("goto _nextLine")

	case *ast.ExitStmt:
		if s.Status != nil {
			c.outputf("os.Exit(%s)", c.intExpr(s.Status))
		} else {
			c.output("os.Exit(0)")
		}

	case *ast.DeleteStmt:
		if len(s.Index) > 0 {
			// Delete single key from array
			c.outputf("delete(%s, %s)", s.Array.Name, c.index(s.Index))
		} else {
			// Delete every element in array
			c.outputf("for _k := range %s {\ndelete(%s, _k)\n}", s.Array.Name, s.Array.Name)
		}

	case *ast.BlockStmt:
		c.output("{\n")
		c.stmts(s.Body)
		c.output("}")

	default:
		panic(errorf("%T not yet supported", s))
	}
}

func (c *compiler) stmt(stmt ast.Stmt) {
	c.stmtNoNewline(stmt)
	c.output("\n")
}

type valueType int

const (
	typeUnknown valueType = iota
	typeStr
	typeNum
	typeArrayStr
	typeArrayNum
)

func (t valueType) String() string {
	switch t {
	case typeStr:
		return "str"
	case typeNum:
		return "num"
	case typeArrayStr:
		return "array of str"
	case typeArrayNum:
		return "array of num"
	default:
		return "unknown"
	}
}

func (c *compiler) expr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.NumExpr:
		if e.Value == float64(int(e.Value)) {
			return fmt.Sprintf("%d.0", int(e.Value))
		}
		if math.IsInf(e.Value, 0) {
			panic(errorf("number literal out of range"))
		}
		return fmt.Sprintf("%g", e.Value)

	case *ast.StrExpr:
		return strconv.Quote(e.Value)

	case *ast.FieldExpr:
		return "_getField(" + c.intExpr(e.Index) + ")"

	case *ast.VarExpr:
		switch e.Scope {
		case ast.ScopeSpecial:
			return c.special(e.Name, e.Index)
		case ast.ScopeGlobal:
			return e.Name
		default:
			panic(errorf("unexpected scope %v", e.Scope))
		}

	case *ast.RegExpr:
		return fmt.Sprintf("_boolToNum(%s.MatchString(_line))", c.regexLiteral(e.Regex))

	case *ast.BinaryExpr:
		return c.binaryExpr(e.Op, e.Left, e.Right)

	case *ast.IncrExpr:
		exprStr := c.expr(e.Expr) // will be an lvalue (VarExpr, IndexExpr, FieldExpr)
		if e.Pre {
			// Change ++x expression to:
			// func() float64 { x++; return x }()
			return fmt.Sprintf("func() float64 { %s%s; return %s }()",
				exprStr, e.Op, exprStr)
		} else {
			// Change x++ expression to:
			// func() float64 { _t := x; x++; return _t }()
			return fmt.Sprintf("func() float64 { _t := %s; %s%s; return _t }()",
				exprStr, exprStr, e.Op)
		}

	case *ast.AssignExpr:
		right := c.expr(e.Right)
		switch l := e.Left.(type) {
		case *ast.VarExpr:
			return fmt.Sprintf("func () %s { %s = %s; return %s }()",
				c.goType(c.typer.exprs[e.Right]), l.Name, right, l.Name)
		default:
			panic(errorf("lvalue type %T not yet supported", l))
		}

	case *ast.CondExpr:
		return fmt.Sprintf("func() %s { if %s { return %s }; return %s }()",
			c.goType(c.typer.exprs[e]), c.cond(e.Cond), c.expr(e.True), c.expr(e.False))

	case *ast.IndexExpr:
		switch e.Array.Scope {
		case ast.ScopeSpecial:
			panic(errorf("special variable %s not yet supported", e.Array.Name))
		case ast.ScopeGlobal:
			return e.Array.Name + "[" + c.index(e.Index) + "]"
		default:
			panic(errorf("unexpected scope %v", e.Array.Scope))
		}

	case *ast.CallExpr:
		switch e.Func {
		case F_ATAN2:
			return "math.Atan2(" + c.numExpr(e.Args[0]) + ", " + c.numExpr(e.Args[1]) + ")"

		case F_COS:
			return "math.Cos(" + c.numExpr(e.Args[0]) + ")"

		case F_EXP:
			return "math.Exp(" + c.numExpr(e.Args[0]) + ")"

		case F_FFLUSH:
			var arg string
			if len(e.Args) > 0 {
				switch argExpr := e.Args[0].(type) {
				case *ast.StrExpr:
					arg = argExpr.Value
				default:
					arg = "not supported"
				}
			}
			if arg != "" {
				panic(errorf(`fflush() currently only supports no args or "" arg`))
			}
			return "_fflush()"

		case F_INDEX:
			return "float64(strings.Index(" + c.strExpr(e.Args[0]) + ", " + c.strExpr(e.Args[1]) + ") + 1)"

		case F_INT:
			return "float64(" + c.intExpr(e.Args[0]) + ")"

		case F_LENGTH:
			switch len(e.Args) {
			case 0:
				return "float64(len(_line))"
			default:
				return "float64(len(" + c.strExpr(e.Args[0]) + "))"
			}

		case F_LOG:
			return "math.Log(" + c.numExpr(e.Args[0]) + ")"

		case F_MATCH:
			if strExpr, ok := e.Args[1].(*ast.StrExpr); ok {
				return fmt.Sprintf("_match(%s, %s)", c.strExpr(e.Args[0]), c.regexLiteral(strExpr.Value))
			}
			return fmt.Sprintf("_match(%s, _reCompile(%s))", c.strExpr(e.Args[0]), c.strExpr(e.Args[1]))

		case F_RAND:
			return "_rand.Float64()"

		case F_SIN:
			return "math.Sin(" + c.numExpr(e.Args[0]) + ")"

		case F_SPLIT:
			arrayArg := e.Args[1].(*ast.ArrayExpr)
			str := fmt.Sprintf("_split(%s, %s, ", c.strExpr(e.Args[0]), arrayArg.Name)
			if len(e.Args) == 3 {
				str += c.strExpr(e.Args[2])
			} else {
				str += "FS"
			}
			str += ")"
			return str

		case F_SPRINTF:
			formatExpr, ok := e.Args[0].(*ast.StrExpr)
			if !ok {
				panic(errorf("sprintf currently only supports literal format strings"))
			}
			args := c.printfArgs(formatExpr.Value, e.Args[1:])
			str := fmt.Sprintf("fmt.Sprintf(%q", formatExpr.Value)
			for _, arg := range args {
				str += ", " + arg
			}
			str += ")"
			return str

		case F_SQRT:
			return "math.Sqrt(" + c.numExpr(e.Args[0]) + ")"

		case F_SRAND:
			if len(e.Args) == 0 {
				return "_srandNow()"
			}
			return "_srand(" + c.numExpr(e.Args[0]) + ")"

		case F_SUB, F_GSUB:
			// sub() is actually an assignment to "in" (an lvalue) or $0:
			// n = sub(re, repl[, in])
			var reArg string
			if strExpr, ok := e.Args[0].(*ast.StrExpr); ok {
				reArg = c.regexLiteral(strExpr.Value)
			} else {
				reArg = fmt.Sprintf("_reCompile(%s)", c.strExpr(e.Args[0]))
			}
			str := fmt.Sprintf("func() float64 { out, n := _sub(%s, %s, ", reArg, c.strExpr(e.Args[1]))
			if len(e.Args) == 3 {
				str += c.expr(e.Args[2])
			} else {
				str += "_line"
			}
			if e.Func == F_GSUB {
				str += ", true); "
			} else {
				str += ", false); "
			}
			if len(e.Args) == 3 {
				str += c.assign(e.Args[2], &ast.VarExpr{Name: "out", Scope: ast.ScopeGlobal})
			} else {
				str += "_setField(0, out)"
			}
			str += "; return float64(n) }()"
			return str

		case F_SUBSTR:
			if len(e.Args) == 2 {
				return "_substr(" + c.strExpr(e.Args[0]) + ", " + c.intExpr(e.Args[1]) + ")"
			}
			return "_substrLength(" + c.strExpr(e.Args[0]) + ", " + c.intExpr(e.Args[1]) + ", " + c.intExpr(e.Args[2]) + ")"

		case F_SYSTEM:
			return "_system(" + c.strExpr(e.Args[0]) + ")"

		case F_TOLOWER:
			return "strings.ToLower(" + c.expr(e.Args[0]) + ")"

		case F_TOUPPER:
			return "strings.ToUpper(" + c.expr(e.Args[0]) + ")"

		default:
			panic(errorf("%s() not yet supported", e.Func))
		}

	case *ast.UnaryExpr:
		str := c.expr(e.Value)
		typ := c.typer.exprs[e.Value]
		switch e.Op {
		case SUB:
			if typ == typeStr {
				str = "_strToNum(" + str + ")"
			}
			return "(-" + str + ")"

		case NOT:
			return "_boolToNum(!(" + c.cond(e.Value) + "))"

		case ADD:
			if typ == typeStr {
				str = "_strToNum(" + str + ")"
			}
			return "(+" + str + ")"

		default:
			panic(errorf("unexpected unary operation: %s", e.Op))
		}

	case *ast.InExpr:
		return fmt.Sprintf("func() float64 { _, ok := %s[%s]; if ok { return 1 }; return 0 }()",
			e.Array.Name, c.index(e.Index))

	default:
		panic(errorf("%T not yet supported", expr))
	}
}

func (c *compiler) binaryExpr(op Token, l, r ast.Expr) (str string) {
	switch op {
	case ADD, SUB, MUL, DIV:
		return "(" + c.numExpr(l) + " " + op.String() + " " + c.numExpr(r) + ")"
	case MOD:
		return "math.Mod(" + c.numExpr(l) + ", " + c.numExpr(r) + ")"
	case POW:
		return "math.Pow(" + c.numExpr(l) + ", " + c.numExpr(r) + ")"
	case CONCAT:
		return "(" + c.strExpr(l) + " + " + c.strExpr(r) + ")"
	default:
		s, ok := c.boolExpr(op, l, r)
		if ok {
			return "_boolToNum(" + s + ")"
		}
		panic(errorf("unexpected binary operator %s", op))
	}
}

func (c *compiler) boolExpr(op Token, l, r ast.Expr) (string, bool) {
	switch op {
	case EQUALS, LESS, LTE, GREATER, GTE, NOT_EQUALS:
		_, leftIsField := l.(*ast.FieldExpr)
		_, rightIsField := r.(*ast.FieldExpr)
		if leftIsField && rightIsField {
			panic(errorf("can't compare two fields directly (%s %s %s); convert one to string or number", l, op, r))
		}
		ls := c.expr(l)
		rs := c.expr(r)
		lt := c.typer.exprs[l]
		rt := c.typer.exprs[r]
		switch lt {
		case typeNum:
			switch rt {
			case typeNum:
				return fmt.Sprintf("(%s %s %s)", ls, op, rs), true
			case typeStr:
				return fmt.Sprintf("(_numToStr(%s) %s %s)", ls, op, rs), true
			}
		case typeStr:
			switch rt {
			case typeNum:
				return fmt.Sprintf("(_strToNum(%s) %s %s)", ls, op, rs), true
			case typeStr:
				return fmt.Sprintf("(%s %s %s)", ls, op, rs), true
			}
		}
		panic(errorf("unexpected types in %s (%s) %s %s (%s)", ls, lt, op, rs, rt))
	case MATCH:
		if strExpr, ok := r.(*ast.StrExpr); ok {
			return fmt.Sprintf("%s.MatchString(%s)", c.regexLiteral(strExpr.Value), c.strExpr(l)), true
		}
		return fmt.Sprintf("_reCompile(%s).MatchString(%s)", c.strExpr(l), c.strExpr(r)), true
	case NOT_MATCH:
		if strExpr, ok := r.(*ast.StrExpr); ok {
			return fmt.Sprintf("(!%s.MatchString(%s))", c.regexLiteral(strExpr.Value), c.strExpr(l)), true
		}
		return fmt.Sprintf("(!_reCompile(%s).MatchString(%s))", c.strExpr(l), c.strExpr(r)), true
	case AND, OR:
		return fmt.Sprintf("(%s %s %s)", c.cond(l), op, c.cond(r)), true
	default:
		return "", false
	}
}

func (c *compiler) cond(expr ast.Expr) string {
	// If possible, simplify conditional expression to avoid "_boolToNum(b) != 0"
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		str, ok := c.boolExpr(e.Op, e.Left, e.Right)
		if ok {
			return str
		}
	case *ast.RegExpr:
		return fmt.Sprintf("%s.MatchString(_line)", c.regexLiteral(e.Regex))
	case *ast.FieldExpr:
		return fmt.Sprintf("_isFieldTrue(%s)", c.expr(e))
	case *ast.UnaryExpr:
		if e.Op == NOT {
			return "(!(" + c.cond(e.Value) + "))"
		}
	}

	str := c.expr(expr)
	if c.typer.exprs[expr] == typeStr {
		str += ` != ""`
	} else {
		str += " != 0"
	}
	return str
}

func (c *compiler) numExpr(expr ast.Expr) string {
	str := c.expr(expr)
	if c.typer.exprs[expr] == typeStr {
		str = "_strToNum(" + str + ")"
	}
	return str
}

func (c *compiler) intExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.NumExpr:
		return strconv.Itoa(int(e.Value))
	case *ast.UnaryExpr:
		ne, ok := e.Value.(*ast.NumExpr)
		if ok && e.Op == SUB {
			return "-" + strconv.Itoa(int(ne.Value))
		}
	}
	return "int(" + c.numExpr(expr) + ")"
}

func (c *compiler) strExpr(expr ast.Expr) string {
	if fieldExpr, ok := expr.(*ast.FieldExpr); ok {
		if numExpr, ok := fieldExpr.Index.(*ast.NumExpr); ok && numExpr.Value == 0 {
			// Optimize _getField(0) to just _line
			return "_line"
		}
	}
	str := c.expr(expr)
	if c.typer.exprs[expr] == typeNum {
		str = "_numToStr(" + str + ")"
	}
	return str
}

func (c *compiler) index(index []ast.Expr) string {
	indexStr := ""
	for i, e := range index {
		if i > 0 {
			indexStr += ` + SUBSEP + `
		}
		str := c.expr(e)
		if c.typer.exprs[e] == typeNum {
			str = "_numToStr(" + str + ")"
		}
		indexStr += str
	}
	return indexStr
}

func (c *compiler) special(name string, index int) string {
	switch index {
	case ast.V_NF:
		return "float64(len(_fields))"
	case ast.V_NR, ast.V_FNR:
		return "float64(_lineNum)"
	case ast.V_RLENGTH:
		return "RLENGTH"
	case ast.V_RSTART:
		return "RSTART"
	case ast.V_CONVFMT:
		return "CONVFMT"
	case ast.V_FS:
		return "FS"
	case ast.V_OFMT:
		return "OFMT"
	case ast.V_OFS:
		return "OFS"
	case ast.V_ORS:
		return "ORS"
	case ast.V_SUBSEP:
		return "SUBSEP"
	default:
		panic(errorf("special variable %s not yet supported", name))
	}
}

func (c *compiler) goType(typ valueType) string {
	switch typ {
	case typeNum:
		return "float64"
	case typeStr:
		return "string"
	case typeArrayNum:
		return "map[string]float64"
	case typeArrayStr:
		return "map[string]string"
	default:
		panic(errorf("can't convert type %s to Go type", typ))
	}
}

func (c *compiler) printfArgs(format string, args []ast.Expr) []string {
	argIndex := 0
	nextArg := func() ast.Expr {
		if argIndex >= len(args) {
			panic(errorf("not enough arguments (%d) for format string %q", len(args), format))
		}
		arg := args[argIndex]
		argIndex++
		return arg
	}

	var argStrs []string
	for i := 0; i < len(format); i++ {
		if format[i] == '%' {
			i++
			if i >= len(format) {
				panic(errorf("expected type specifier after %%"))
			}
			if format[i] == '%' {
				continue
			}
			for i < len(format) && bytes.IndexByte([]byte(" .-+*#0123456789"), format[i]) >= 0 {
				if format[i] == '*' {
					argStrs = append(argStrs, c.intExpr(nextArg()))
				}
				i++
			}
			if i >= len(format) {
				panic(errorf("expected type specifier after %%"))
			}
			var argStr string
			switch format[i] {
			case 's':
				argStr = c.strExpr(nextArg())
			case 'd', 'i', 'o', 'x', 'X', 'u':
				argStr = c.intExpr(nextArg())
			case 'f', 'e', 'E', 'g', 'G':
				argStr = "float64(" + c.numExpr(nextArg()) + ")"
			case 'c':
				arg := nextArg()
				if c.typer.exprs[arg] == typeStr {
					argStr = fmt.Sprintf("_firstRune(%s)", arg)
				} else {
					argStr = c.intExpr(arg)
				}
			default:
				panic(errorf("invalid format type %q", format[i]))
			}
			argStrs = append(argStrs, argStr)
		}
	}
	return argStrs
}

func (c *compiler) regexLiteral(pattern string) string {
	n, ok := c.regexen[pattern]
	if !ok {
		n = len(c.regexen) + 1
		c.regexen[pattern] = n
	}
	varName := fmt.Sprintf("_re%d", n)
	return varName
}
