package main

import (
	"errors"
	"fmt"
	"io"
	"os"
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
	defer func() {
		r := recover()
		switch r := r.(type) {
		case nil:
			break
		case *errorExit:
			err = errors.New(r.message)
		default:
			panic(r)
		}
	}()

	t := newTyper()
	t.program(prog)
	t.program(prog)
	//t.dump()

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
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	_output  *bufio.Writer
	_scanner *bufio.Scanner
	_line    string
	_fields  []string
	_lineNum int

`)

	for name, typ := range c.typer.globals {
		c.output(fmt.Sprintf("%s %s\n", name, c.goType(typ)))
	}

	c.output(`)

func main() {
	_output = bufio.NewWriter(os.Stdout)
	defer _output.Flush()

	_scanner = bufio.NewScanner(os.Stdin)

	OFS = " "
	ORS = "\n"
`)

	for i, action := range prog.Actions {
		if len(action.Pattern) == 2 {
			// Booleans to keep track of range pattern state
			c.output(fmt.Sprintf("_inRange%d := false\n", i))
		}
	}

	for name, typ := range c.typer.globals {
		switch typ {
		case typeArrayStr, typeArrayNum:
			c.output(fmt.Sprintf("%s = make(%s)\n", name, c.goType(typ)))
		}
	}

	for _, stmts := range prog.Begin {
		c.output("\n")
		c.stmts(stmts)
	}

	if len(prog.Actions) > 0 {
		c.output(`
	for _scanner.Scan() {
		_lineNum++
		_line = _scanner.Text()
		_fields = strings.Fields(_line)
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

func (c *compiler) stmtNoNewline(stmt Stmt) {
	switch s := stmt.(type) {
	case *ExprStmt:
		switch e := s.Expr.(type) {
		case *AssignExpr:
			switch left := e.Left.(type) {
			case *VarExpr:
				// TODO: handle ScopeSpecial
				c.output(left.Name)
				c.output(" = ")
				c.output(c.expr(e.Right))
			case *IndexExpr:
				c.output(left.Array.Name)
				c.output("[")
				c.output(c.index(left.Index))
				c.output("] = ")
				c.output(c.expr(e.Right))
			case *FieldExpr:
				// TODO: simplify to _fields[n-1] if n is int constant?
				c.output("_setField(")
				c.output(c.intExpr(left.Index))
				c.output(", ")
				c.output(c.strExpr(e.Right))
				c.output(")")
			}

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
				str := c.expr(arg) // TODO: need to handle type?
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
		c.output("fmt.Fprintf(_output, ")
		c.output(c.strExpr(s.Args[0]))
		for _, a := range s.Args[1:] {
			// TODO: hmm, need special handling for the types to avoid "%!d(string=1234)"
			c.output(", ")
			str := c.expr(a)
			c.output(str)
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
		// TODO: scoping of loop variable
		c.output("for ")
		c.output(s.Var.Name)
		c.output(" := range ")
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
		panic(errorf(`"next" statement not supported`))

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
		return fmt.Sprintf("_boolToNum(_regexMatch(_line, %q))", e.Regex)

	case *BinaryExpr:
		return c.binaryExpr(e.Op, e.Left, e.Right)

	case *IncrExpr:
		// TODO: hmm, this isn't going to handle m[k]++
		switch {
		case e.Op == INCR && e.Pre:
			// ++x
			return "_preIncr(&" + c.expr(e.Expr) + ")"
		case e.Op == INCR && !e.Pre:
			// x++
			return "_postIncr(&" + c.expr(e.Expr) + ")"
		case e.Op == DECR && e.Pre:
			// --x
			return "_preDecr(&" + c.expr(e.Expr) + ")"
		case e.Op == DECR && !e.Pre:
			// x--
			return "_postDecr(&" + c.expr(e.Expr) + ")"
		default:
			panic(errorf("unexpected increment type %s (pre=%v)", e.Op, e.Pre))
		}

	case *AssignExpr:
		right := c.expr(e.Right)
		switch l := e.Left.(type) {
		case *VarExpr:
			if c.typer.exprs[e.Right] == typeNum {
				return "_assignNum(&" + l.Name + ", " + right + ")"
			}
			return "_assignStr(&" + l.Name + ", " + right + ")"

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
		//case F_CLOSE:
		case F_COS:
			return "math.Cos(" + c.numExpr(e.Args[0]) + ")"
		case F_EXP:
			return "math.Exp(" + c.numExpr(e.Args[0]) + ")"
		//case F_FFLUSH
		//case F_GSUB
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
		//case F_MATCH
		//case F_RAND
		case F_SIN:
			return "math.Sin(" + c.numExpr(e.Args[0]) + ")"
		//case F_SPLIT
		//case F_SPRINTF
		case F_SQRT:
			return "math.Sqrt(" + c.numExpr(e.Args[0]) + ")"
		//case F_SRAND
		//case F_SUB
		//case F_SUBSTR
		//case F_SYSTEM
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
			if typ == typeStr {
				str = str + ` == ""`
			} else {
				str = str + " == 0 "
			}
			return "_boolToNum(" + str + ")"

		case ADD:
			if typ == typeStr {
				str = "_strToNum(" + str + ")"
			}
			return "(+" + str + ")"

		default:
			panic(errorf("unexpected unary operation: %s", e.Op))
		}

	case *InExpr:
		if c.typer.globals[e.Array.Name] == typeArrayNum {
			return "_boolToNum(_containsNum(" + e.Array.Name + ", " + c.index(e.Index) + "))"
		}
		return "_boolToNum(_containsStr(" + e.Array.Name + ", " + c.index(e.Index) + "))"

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
		return "_regexMatch(" + c.strExpr(l) + ", " + c.strExpr(r) + ")", true
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
		return fmt.Sprintf("_regexMatch(_line, %q)", e.Regex)
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
			indexStr += ` + "\x1c" + `
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
	//TODO:
	//case V_RLENGTH:
	//case V_RSTART:
	//case V_FNR:
	//case V_ARGC:
	//case V_CONVFMT:
	//case V_FILENAME:
	//case V_FS:
	//case V_OFMT:
	case V_OFS:
		return "OFS"
	case V_ORS:
		return "ORS"
	//case V_RS:
	//case V_SUBSEP:
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
