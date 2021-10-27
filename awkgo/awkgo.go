// AWKGo: AWK to Go compiler

/*
TODO:
- figure out how or whether to handle numStr types
- support functions
- make print statement output more compact
- pre-compile regex literals
- AugAssign of field not yet supported
- Incr of field not yet supported
- print redirection?
- non-literal [s]printf format strings?
- getline?
- a way to report line/col info in error messages? (parser doesn't record these)

NOT SUPPORTED:
- dynamic typing
- assigning numStr values (but using $0 in conditionals works)
- null values (unset number variable should output "", we output "0")
- "next" in functions
*/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	. "github.com/benhoyt/goawk/internal/ast"
	. "github.com/benhoyt/goawk/lexer"
	. "github.com/benhoyt/goawk/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: awkgo 'prog'")
		os.Exit(1)
	}

	prog, err := ParseProgram([]byte(os.Args[1]), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	err = compile(prog, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compile error: %v\n", err)
		os.Exit(1)
	}
}

func compile(prog *Program, writer io.Writer) (err error) {
	// Both typer and compiler signal errors internally with
	// panic(&errorExit{...}), so recover and return an error.
	defer func() {
		r := recover()
		switch r := r.(type) {
		case nil:
		case *errorExit:
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
		typer:  t,
		writer: writer,
	}
	c.program(prog)

	return nil
}

type errorExit struct {
	message string
}

func (e *errorExit) Error() string {
	return e.message
}

func errorf(format string, args ...interface{}) error {
	return &errorExit{fmt.Sprintf(format, args...)}
}

type compiler struct {
	prog   *Program
	typer  *typer
	writer io.Writer
}

func (c *compiler) output(s string) {
	fmt.Fprint(c.writer, s)
}

func (c *compiler) program(prog *Program) {
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
		c.output(fmt.Sprintf("%s %s\n", name, c.goType(typ)))
	}

	c.output(`)

func main() {
	_output = bufio.NewWriter(os.Stdout)
	defer _output.Flush()

	_scanner = bufio.NewScanner(os.Stdin)

	OFS = " "
	ORS = "\n"
	OFMT = "%.6g"
	CONVFMT = "%.6g"
	SUBSEP = "\x1c"
	_seed = 1.0
	_rand = rand.New(rand.NewSource(int64(math.Float64bits(_seed))))

`)

	for i, action := range prog.Actions {
		if len(action.Pattern) == 2 {
			// Booleans to keep track of range pattern state
			c.output(fmt.Sprintf("_inRange%d := false\n", i))
		}
	}

	for _, name := range globals {
		typ := c.typer.globals[name]
		switch typ {
		case typeArrayStr, typeArrayNum:
			c.output(fmt.Sprintf("%s = make(%s)\n", name, c.goType(typ)))
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
		_fields = strings.Fields(_line) // TODO: use FS or call _split or similar
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

	for _, f := range prog.Functions {
		c.function(f)
	}

	c.outputHelpers()
}

func (c *compiler) actions(actions []Action) {
	for i, action := range actions {
		c.output("\n")
		switch len(action.Pattern) {
		case 0:
			// No pattern is equivalent to pattern evaluating to true
		case 1:
			// Single boolean pattern
			c.output("if ")
			c.output(c.cond(action.Pattern[0]))
			c.output(" {\n")
		case 2:
			// Range pattern (matches between start and stop lines)
			c.output(fmt.Sprintf("if !_inRange%d { _inRange%d = %s }\n",
				i, i, c.cond(action.Pattern[0])))
			c.output(fmt.Sprintf("if _inRange%d {\n", i))
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
			c.output(fmt.Sprintf("\n_inRange%d = !(%s)\n", i, c.cond(action.Pattern[1])))
			c.output("}\n")
		}
	}
}

func (c *compiler) stmts(stmts Stmts) {
	for _, stmt := range stmts {
		c.stmt(stmt)
	}
}

func (c *compiler) assign(left, right Expr) string {
	switch left := left.(type) {
	case *VarExpr:
		// TODO: handle ScopeSpecial
		return fmt.Sprintf("%s = %s", left.Name, c.expr(right))
	case *IndexExpr:
		return fmt.Sprintf("%s[%s] = %s", left.Array.Name, c.index(left.Index), c.expr(right))
	case *FieldExpr:
		// TODO: simplify to _fields[n-1] if n is int constant?
		return fmt.Sprintf("_setField(%s, %s)", c.intExpr(left.Index), c.strExpr(right))
	default:
		panic(errorf("expected lvalue, not %s", left))
	}
}

func (c *compiler) stmtNoNewline(stmt Stmt) {
	switch s := stmt.(type) {
	case *ExprStmt:
		switch e := s.Expr.(type) {
		case *AssignExpr:
			c.output(c.assign(e.Left, e.Right))

		case *AugAssignExpr:
			switch left := e.Left.(type) {
			case *VarExpr:
				// TODO: handle ScopeSpecial
				switch e.Op {
				case MOD, POW:
					c.output(left.Name)
					if e.Op == MOD {
						c.output(" = math.Mod(")
					} else {
						c.output(" = math.Pow(")
					}
					c.output(left.Name)
					c.output(", ")
					c.output(c.numExpr(e.Right))
					c.output(")")
				default:
					c.output(left.Name)
					c.output(" " + e.Op.String() + "= ")
					c.output(c.numExpr(e.Right))
				}
			case *IndexExpr:
				switch e.Op {
				case MOD, POW:
					c.output(left.Array.Name)
					c.output("[")
					c.output(c.index(left.Index))
					if e.Op == MOD {
						c.output("] = math.Mod(")
					} else {
						c.output("] = math.Pow(")
					}
					c.output(left.Array.Name)
					c.output("[")
					c.output(c.index(left.Index))
					c.output("], ")
					c.output(c.numExpr(e.Right))
					c.output(")")
				default:
					c.output(left.Array.Name)
					c.output("[")
					c.output(c.index(left.Index))
					c.output("] " + e.Op.String() + "= ")
					c.output(c.numExpr(e.Right))
				}
			case *FieldExpr:
				panic(errorf("AugAssign of field not yet supported"))
			}

		case *IncrExpr:
			switch left := e.Expr.(type) {
			case *VarExpr:
				// TODO: handle ScopeSpecial
				c.output(left.Name)
				c.output(e.Op.String())
			case *IndexExpr:
				c.output(left.Array.Name)
				c.output("[")
				c.output(c.index(left.Index))
				c.output("]" + e.Op.String())
			case *FieldExpr:
				panic(errorf("Incr of field not yet supported"))
			}

		default:
			c.output("_ = ")
			str := c.expr(s.Expr)
			c.output(str)
		}

	case *PrintStmt:
		// TODO: if OFS and ORS are never changed, use fmt.Fprintln() instead
		if s.Dest != nil {
			panic(errorf("print redirection not yet supported"))
		}
		c.output("fmt.Fprint(_output, ")
		if len(s.Args) > 0 {
			for i, arg := range s.Args {
				if i > 0 {
					c.output(", OFS, ")
				}
				str := c.expr(arg)
				if c.typer.exprs[arg] == typeNum {
					str = fmt.Sprintf("_numToStrFormat(OFMT, %s)", str)
				}
				c.output(str)
			}
		} else {
			// "print" with no args is equivalent to "print $0"
			c.output("_line")
		}
		c.output(", ORS)")

	case *PrintfStmt:
		if s.Dest != nil {
			panic(errorf("printf redirection not yet supported"))
		}
		formatExpr, ok := s.Args[0].(*StrExpr)
		if !ok {
			panic(errorf("printf currently only supports literal format strings"))
		}
		args := c.printfArgs(formatExpr.Value, s.Args[1:])
		c.output(fmt.Sprintf("fmt.Fprintf(_output, %q", formatExpr.Value))
		for _, arg := range args {
			c.output(", ")
			c.output(arg)
		}
		c.output(")")

	case *IfStmt:
		c.output("if ")
		switch cond := s.Cond.(type) {
		case *InExpr:
			// if _, _ok := a[k]; ok { ... }
			c.output(fmt.Sprintf("_, _ok := %s[%s]; _ok ",
				cond.Array.Name, c.index(cond.Index)))
		default:
			c.output(c.cond(s.Cond))
		}
		c.output(" {\n")
		c.stmts(s.Body)
		c.output("}")
		if len(s.Else) > 0 {
			// TODO: handle "else if"
			c.output(" else {\n")
			c.stmts(s.Else)
			c.output("}")
		}

	case *ForStmt:
		c.output("for ")
		if s.Pre != nil {
			_, ok := s.Pre.(*ExprStmt)
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
			_, ok := s.Post.(*ExprStmt)
			if !ok {
				panic(errorf(`only expressions are allowed in "for" post expression`))
			}
			c.stmtNoNewline(s.Post)
		}
		c.output(" {\n")
		c.stmts(s.Body)
		c.output("}")

	case *ForInStmt:
		c.output("for ")
		c.output(s.Var.Name)
		c.output(" = range ")
		c.output(s.Array.Name)
		c.output(" {\n")
		c.stmts(s.Body)
		c.output("}")

	case *ReturnStmt:
		if s.Value != nil {
			c.output("return ")
			str := c.expr(s.Value)
			c.output(str)
		} else {
			c.output("return")
		}

	case *WhileStmt:
		c.output("for ")
		c.output(c.cond(s.Cond))
		c.output(" {\n")
		c.stmts(s.Body)
		c.output("}")

	case *DoWhileStmt:
		c.output("for {\n")
		c.stmts(s.Body)
		c.output("if !(")
		c.output(c.cond(s.Cond))
		c.output(") {\nbreak\n}\n")
		c.output("}")

	case *BreakStmt:
		c.output("break")

	case *ContinueStmt:
		c.output("continue")

	case *NextStmt:
		// typer ensures "next" is not used inside a function
		c.output("goto _nextLine")

	case *ExitStmt:
		if s.Status != nil {
			c.output("os.Exit(")
			c.output(c.intExpr(s.Status))
			c.output(")")
		} else {
			c.output("os.Exit(0)")
		}

	case *DeleteStmt:
		if len(s.Index) > 0 {
			// Delete single key from array
			c.output("delete(")
			c.output(s.Array.Name)
			c.output(", ")
			c.output(c.index(s.Index))
			c.output(")")
		} else {
			// Delete every element in array
			c.output("for k := range ")
			c.output(s.Array.Name)
			c.output(" {\ndelete(")
			c.output(s.Array.Name)
			c.output(", k)\n}")
		}

	case *BlockStmt:
		c.output("{\n")
		c.stmts(s.Body)
		c.output("}")

	default:
		panic(errorf("%T not yet supported", s))
	}
}

func (c *compiler) stmt(stmt Stmt) {
	c.stmtNoNewline(stmt)
	c.output("\n")
}

type valueType int

const (
	typeUnknown valueType = iota
	typeStr
	typeNum
	typeNumStr // TODO: don't support this for now?
	typeArrayStr
	typeArrayNum
)

func (t valueType) String() string {
	switch t {
	case typeStr:
		return "str"
	case typeNum:
		return "num"
	case typeNumStr:
		return "numeric string"
	case typeArrayStr:
		return "array of str"
	case typeArrayNum:
		return "array of num"
	default:
		return "unknown"
	}
}

func (c *compiler) expr(expr Expr) string {
	switch e := expr.(type) {
	case *NumExpr:
		if e.Value == float64(int(e.Value)) {
			return fmt.Sprintf("%d.0", int(e.Value))
		}
		if math.IsInf(e.Value, 0) {
			panic(errorf("number literal out of range"))
		}
		return fmt.Sprintf("%g", e.Value)

	case *StrExpr:
		return strconv.Quote(e.Value)

	case *FieldExpr:
		return "_getField(" + c.intExpr(e.Index) + ")"

	case *VarExpr:
		switch e.Scope {
		case ScopeSpecial:
			return c.special(e.Name, e.Index)
		case ScopeGlobal:
			return e.Name
		default:
			panic(errorf("unexpected scope %v", e.Scope))
		}

	case *RegExpr:
		// TODO: pre-compile regex literal as global
		return fmt.Sprintf("_boolToNum(_regexMatches(_line, %q))", e.Regex)

	case *BinaryExpr:
		return c.binaryExpr(e.Op, e.Left, e.Right)

	case *IncrExpr:
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

	case *AssignExpr:
		right := c.expr(e.Right)
		switch l := e.Left.(type) {
		case *VarExpr:
			return fmt.Sprintf("func () %s { %s = %s; return %s }()",
				c.goType(c.typer.exprs[e.Right]), l.Name, right, l.Name)

		//TODO: case *IndexExpr:

		//TODO: case *FieldExpr:

		default:
			panic(errorf("unexpected lvalue type: %T", l))
		}

	//case *AugAssignExpr:
	//	return "TODO", 0

	case *CondExpr:
		return fmt.Sprintf("func() %s { if %s { return %s }; return %s }()",
			c.goType(c.typer.exprs[e]), c.cond(e.Cond), c.expr(e.True), c.expr(e.False))

	case *IndexExpr:
		// TODO: see interp.go getArrayValue
		// Strangely, per the POSIX spec, "Any other reference to a
		// nonexistent array element [apart from "in" expressions]
		// shall automatically create it."
		switch e.Array.Scope {
		case ScopeSpecial:
			panic(errorf("special variable %s not yet supported", e.Array.Name))
		case ScopeGlobal:
			return e.Array.Name + "[" + c.index(e.Index) + "]"
		default:
			panic(errorf("unexpected scope %v", e.Array.Scope))
		}

	case *CallExpr:
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
				case *StrExpr:
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
			return "_match(" + c.strExpr(e.Args[0]) + ", " + c.strExpr(e.Args[1]) + ")"

		case F_RAND:
			return "_rand.Float64()"

		case F_SIN:
			return "math.Sin(" + c.numExpr(e.Args[0]) + ")"

		case F_SPLIT:
			arrayArg := e.Args[1].(*ArrayExpr)
			str := fmt.Sprintf("_split(%s, %s, ", c.strExpr(e.Args[0]), arrayArg.Name)
			if len(e.Args) == 3 {
				str += c.strExpr(e.Args[2])
			} else {
				str += `" "` // TODO: use FS
			}
			str += ")"
			return str

		case F_SPRINTF:
			formatExpr, ok := e.Args[0].(*StrExpr)
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
			str := fmt.Sprintf("func() float64 { out, n := _sub(%s, %s, ", c.strExpr(e.Args[0]), c.strExpr(e.Args[1]))
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
				// TODO: hmm, passing VarExpr here is weird
				str += c.assign(e.Args[2], &VarExpr{Name: "out", Scope: ScopeGlobal})
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

	case *UnaryExpr:
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

	case *InExpr:
		return fmt.Sprintf("func() float64 { _, ok := %s[%s]; if ok { return 1 }; return 0 }()",
			e.Array.Name, c.index(e.Index))

	//case *UserCallExpr:
	//	return "TODO", 0

	//case *GetlineExpr:
	//	return "TODO", 0

	default:
		panic(errorf("%T not yet supported", expr))
	}
}

func (c *compiler) binaryExpr(op Token, l, r Expr) (str string) {
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

func (c *compiler) boolExpr(op Token, l, r Expr) (string, bool) {
	switch op {
	case EQUALS, LESS, LTE, GREATER, GTE, NOT_EQUALS:
		ls := c.expr(l)
		rs := c.expr(r)
		lt := c.typer.exprs[l]
		rt := c.typer.exprs[r]
		switch lt {
		case typeNum:
			switch rt {
			case typeNum:
				return ls + " " + op.String() + " " + rs, true
			case typeStr:
				return "_numToStr(" + ls + ") " + op.String() + " " + rs, true
			case typeNumStr:
				return ls + " " + op.String() + " _strToNum(" + rs + ")", true
			}
		case typeStr:
			switch rt {
			case typeNum:
				return ls + " " + op.String() + " _numToStr(" + rs + ")", true
			case typeStr, typeNumStr:
				return ls + " " + op.String() + " " + rs, true
			}
		case typeNumStr:
			switch rt {
			case typeNum:
				return "_strToNum(" + ls + ") " + op.String() + " " + rs, true
			case typeStr:
				return ls + " " + op.String() + " " + rs, true
			case typeNumStr:
				panic(errorf("type on one side of %s comparison must be known", op))
			}
		}
		panic(errorf("unexpected types in %s (%s) %s %s (%s)", ls, lt, op.String(), rs, rt))
	case MATCH, NOT_MATCH:
		// TODO: pre-compile regex literals if r is string literal
		return "_regexMatches(" + c.strExpr(l) + ", " + c.strExpr(r) + ")", true
	case AND, OR:
		// TODO: what to do about precedence / parentheses?
		return c.cond(l) + " " + op.String() + " " + c.cond(r), true
	default:
		return "", false
	}
}

func (c *compiler) cond(expr Expr) string {
	// If possible, simplify conditional expression to avoid "_boolToNum(b) != 0"
	switch e := expr.(type) {
	case *BinaryExpr:
		str, ok := c.boolExpr(e.Op, e.Left, e.Right)
		if ok {
			return str
		}
	case *RegExpr:
		return fmt.Sprintf("_regexMatches(_line, %q)", e.Regex)
	case *FieldExpr:
		return fmt.Sprintf("_isNumStrTrue(%s)", c.expr(e))
	case *UnaryExpr:
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

func (c *compiler) numExpr(expr Expr) string {
	str := c.expr(expr)
	if c.typer.exprs[expr] == typeStr {
		str = "_strToNum(" + str + ")"
	}
	return str
}

func (c *compiler) intExpr(expr Expr) string {
	switch e := expr.(type) {
	case *NumExpr:
		return strconv.Itoa(int(e.Value))
	case *UnaryExpr:
		ne, ok := e.Value.(*NumExpr)
		if ok && e.Op == SUB {
			return "-" + strconv.Itoa(int(ne.Value))
		}
	}
	return "int(" + c.numExpr(expr) + ")"
}

func (c *compiler) strExpr(expr Expr) string {
	str := c.expr(expr)
	if c.typer.exprs[expr] == typeNum {
		str = "_numToStr(" + str + ")"
	}
	return str
}

func (c *compiler) index(index []Expr) string {
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

func (c *compiler) function(f Function) {
	// TODO: handle param types and return type (and use f.Arrays)
	c.output("\nfunc ")
	c.output(f.Name)
	c.output("(")
	if len(f.Params) > 0 {
		c.output(strings.Join(f.Params, ", "))
		c.output(" string")
	}
	c.output(") {\n")
	c.stmts(f.Body)
	c.output("}\n")
}

func (c *compiler) special(name string, index int) string {
	switch index {
	case V_NF:
		return "float64(len(_fields))"
	case V_NR:
		return "float64(_lineNum)"
	case V_RLENGTH:
		return "RLENGTH"
	case V_RSTART:
		return "RSTART"
	//case V_FNR:
	//case V_ARGC:
	case V_CONVFMT:
		return "CONVFMT"
	//case V_FILENAME:
	//case V_FS:
	case V_OFMT:
		return "OFMT"
	case V_OFS:
		return "OFS"
	case V_ORS:
		return "ORS"
	//case V_RS:
	case V_SUBSEP:
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

func (c *compiler) printfArgs(format string, args []Expr) []string {
	argIndex := 0
	nextArg := func() Expr {
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
				// TODO: could avoid float64() in many cases
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
