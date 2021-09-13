package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	. "github.com/benhoyt/goawk/internal/ast"
	. "github.com/benhoyt/goawk/lexer"
	. "github.com/benhoyt/goawk/parser"
)

func main() {
	defer func() {
		r := recover()
		switch r := r.(type) {
		case *errorExit:
			fmt.Fprintln(os.Stderr, r.message)
			os.Exit(1)
		case nil:
			break
		default:
			panic(r)
		}
	}()

	prog, err := ParseProgram([]byte(os.Args[1]), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	c := &compiler{}
	c.globalTypes = make(map[string]valueType)
	c.program(prog)
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
	prog        *Program
	globalTypes map[string]valueType
}

func (c *compiler) globalType(name string) valueType {
	return c.globalTypes[name]
}

func (c *compiler) output(s string) {
	fmt.Print(s)
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

func main() {
	_output := bufio.NewWriter(os.Stdout)
	defer _output.Flush()

	_scanner := bufio.NewScanner(os.Stdin)
	for _scanner.Scan() {
		_line := _scanner.Text()
		_fields := strings.Fields(_line)
`)
	c.actions(prog.Actions)
	c.output(`	}
	if _scanner.Err() != nil {
		fmt.Fprintln(os.Stderr, _scanner.Err())
		os.Exit(1)
	}
}
`)

	// TODO: handle BEGIN and END

	for _, f := range prog.Functions {
		c.function(f)
	}

	c.outputHelpers()
}

func (c *compiler) actions(actions []Action) {
	for _, action := range actions {
		if len(action.Pattern) > 0 {
			panic(errorf("patterns not yet supported"))
		}
		if len(action.Stmts) == 0 {
			panic(errorf("must have an action"))
		}
		c.stmts(action.Stmts)
	}
}

func (c *compiler) stmts(stmts Stmts) {
	for _, stmt := range stmts {
		c.stmt(stmt)
	}
}

func (c *compiler) stmt(stmt Stmt) {
	switch stmt := stmt.(type) {
	case *ExprStmt:
		// TODO: some expressions like "1+2" won't be valid Go statements
		// TODO: if it's an assign expr, simplify _assignNum(&x, 3) to "x = 3"
		s, _ := c.expr(stmt.Expr)
		c.output(s)

	case *PrintStmt:
		// TODO: handle modified OFS and ORS
		if stmt.Dest != nil {
			panic(errorf("print redirection not yet supported"))
		}
		c.output("fmt.Fprintln(_output, ")
		for i, arg := range stmt.Args {
			if i > 0 {
				c.output(", ")
			}
			s, _ := c.expr(arg) // TODO: need to handle type?
			c.output(s)
		}
		c.output(")")

	case *PrintfStmt:
		if stmt.Dest != nil {
			panic(errorf("printf redirection not yet supported"))
		}
		c.output("fmt.Fprintf(_output, ")
		c.output(c.strExpr(stmt.Args[0]))
		for _, a := range stmt.Args[1:] {
			// TODO: hmm, need special handling for the types to avoid "%!d(string=1234)"
			c.output(", ")
			s, _ := c.expr(a)
			c.output(s)
		}
		c.output(")")

	case *IfStmt:
		c.output("if ")
		c.output(c.cond(stmt.Cond))
		c.output(" {\n")
		c.stmts(stmt.Body)
		c.output("}")
		if len(stmt.Else) > 0 {
			// TODO: handle "else if"
			c.output(" else {\n")
			c.stmts(stmt.Else)
			c.output("}")
		}

	case *ForStmt:
		c.output("for ")
		if stmt.Pre != nil {
			exprStmt, ok := stmt.Pre.(*ExprStmt)
			if !ok {
				panic(errorf(`only expressions are allowed in "for" initializer`))
			}
			s, _ := c.expr(exprStmt.Expr)
			c.output(s)
		}
		c.output("; ")
		if stmt.Cond != nil {
			c.cond(stmt.Cond)
		}
		c.output("; ")
		if stmt.Post != nil {
			exprStmt, ok := stmt.Pre.(*ExprStmt)
			if !ok {
				panic(errorf(`only expressions are allowed in "for" post expression`))
			}
			s, _ := c.expr(exprStmt.Expr)
			c.output(s)
		}
		c.output(" {\n")
		c.stmts(stmt.Body)
		c.output("}")

	case *ForInStmt:
		// TODO: scoping of loop variable
		c.output("for ")
		c.output(stmt.Var.Name)
		c.globalTypes[stmt.Var.Name] = typeStr
		c.output(" := range ")
		c.output(stmt.Array.Name)
		c.output(" {\n")
		c.stmts(stmt.Body)
		c.output("}")

	case *ReturnStmt:
		if stmt.Value != nil {
			c.output("return ")
			s, _ := c.expr(stmt.Value)
			c.output(s)
		} else {
			c.output("return")
		}

	case *WhileStmt:
		c.output("for ")
		c.output(c.cond(stmt.Cond))
		c.output(" {\n")
		c.stmts(stmt.Body)
		c.output("}")

	case *DoWhileStmt:
		c.output("for {\n")
		c.stmts(stmt.Body)
		c.output("if !(")
		c.output(c.cond(stmt.Cond))
		c.output(") {\nbreak\n}\n")
		c.output("}")

	case *BreakStmt:
		c.output("break")

	case *ContinueStmt:
		c.output("continue")

	case *NextStmt:
		panic(errorf(`"next" statement not supported`))

	case *ExitStmt:
		if stmt.Status != nil {
			c.output("os.Exit(")
			c.output(c.intExpr(stmt.Status))
			c.output(")")
		} else {
			c.output("os.Exit(0)")
		}

	case *DeleteStmt:
		if len(stmt.Index) > 0 {
			// Delete single key from array
			c.output("delete(")
			c.output(stmt.Array.Name)
			c.output(", ")
			c.output(c.index(stmt.Index))
			c.output(")")
		} else {
			// Delete every element in array
			c.output("for k := range ")
			c.output(stmt.Array.Name)
			c.output(" {\ndelete(")
			c.output(stmt.Array.Name)
			c.output(", k)\n}")
		}

	case *BlockStmt:
		c.output("{\n")
		c.stmts(stmt.Body)
		c.output("}")

	default:
		panic(errorf("%T not yet supported", stmt))
	}
	c.output("\n")
}

type valueType int

const (
	typeUnknown valueType = iota
	typeStr
	typeNum
	typeNumStr
)

func (c *compiler) expr(expr Expr) (string, valueType) {
	switch e := expr.(type) {
	case *NumExpr:
		return fmt.Sprintf("%g", e.Value), typeNum

	case *StrExpr:
		return strconv.Quote(e.Value), typeStr

	case *FieldExpr:
		return "_getField(_line, _fields, " + c.intExpr(e.Index) + ")", typeNumStr

	case *VarExpr:
		if e.Scope != ScopeGlobal {
			panic(errorf("scope %v not yet supported", e.Scope))
		}
		// TODO: ideally would do a pass to determine types ahead of time...
		t := c.globalType(e.Name)
		if t == typeUnknown {
			panic(errorf("%q not yet assigned to; type not known", e.Name))
		}
		return e.Name, t

	case *RegExpr:
		// TODO: pre-compile regex literal as global
		return fmt.Sprintf("_regexMatch(%q, _line)", e.Regex), typeNum

	case *BinaryExpr:
		return c.binaryExpr(e.Op, e.Left, e.Right)

	//case *IncrExpr:
	//	return "TODO", 0

	case *AssignExpr:
		r, t := c.expr(e.Right)
		switch l := e.Left.(type) {
		case *VarExpr:
			// TODO: check scope
			c.globalTypes[l.Name] = t
			if t == typeNum {
				return "_assignNum(&" + l.Name + ", " + r + ")", typeNum
			}
			return "_assignStr(&" + l.Name + ", " + r + ")", t

		//TODO: case *IndexExpr:

		//TODO: case *FieldExpr:

		default:
			panic(errorf("unexpected lvalue type: %T", l))
		}

	//case *AugAssignExpr:
	//	return "TODO", 0

	case *CondExpr:
		// TODO: should only evaluate True or False, not both
		ts, tt := c.expr(e.True)
		fs, ft := c.expr(e.False)
		if tt != ft {
			panic(errorf("true branch of ?: must be same type as false branch"))
		}
		if tt == typeNum {
			return "_condNum(" + c.cond(e.Cond) + ", " + ts + ", " + fs + ")", typeNum
		}
		return "_condStr(" + c.cond(e.Cond) + ", " + ts + ", " + fs + ")", tt

	case *IndexExpr:
		if e.Array.Scope != ScopeGlobal {
			panic(errorf("scope %v not yet supported", e.Array.Scope))
		}
		arrayType := c.globalType(e.Array.Name)
		if arrayType == typeUnknown {
			panic(errorf("%q not yet assigned to; type not known", e.Array.Name))
		}
		return e.Array.Name + "[" + c.index(e.Index) + "]", arrayType

	//case *CallExpr:
	//	return "TODO", 0

	case *UnaryExpr:
		s, t := c.expr(e.Value)
		switch e.Op {
		case SUB:
			if t == typeStr {
				s = "_strToNum(" + s + ")"
			}
			return "-" + s, typeNum

		case NOT:
			if t == typeStr {
				s = s + ` == ""`
			} else {
				s = s + " == 0 "
			}
			return "_boolToNum(" + s + ")", typeNum

		case ADD:
			if t == typeStr {
				s = "_strToNum(" + s + ")"
			}
			return "+" + s, typeNum

		default:
			panic(errorf("unexpected unary operation: %s", e.Op))
		}

	case *InExpr:
		if e.Array.Scope != ScopeGlobal {
			panic(errorf("scope %v not yet supported", e.Array.Scope))
		}
		arrayType := c.globalType(e.Array.Name)
		if arrayType == typeUnknown {
			panic(errorf("%q not yet assigned to; type not known", e.Array.Name))
		}
		if arrayType == typeNum {
			return "_containsNum(" + e.Array.Name + ", " + c.index(e.Index) + ")", typeNum
		}
		return "_containsStr(" + e.Array.Name + ", " + c.index(e.Index) + ")", arrayType

	//case *UserCallExpr:
	//	return "TODO", 0

	//case *GetlineExpr:
	//	return "TODO", 0

	default:
		panic(errorf("%T not yet supported", expr))
	}
}

func (c *compiler) binaryExpr(op Token, l, r Expr) (string, valueType) {
	switch op {
	case ADD, SUB, MUL, DIV, MOD:
		return c.numExpr(l) + " " + op.String() + " " + c.numExpr(r), typeNum
	case POW:
		return "math.Pow(" + c.numExpr(l) + ", " + c.numExpr(r) + ")", typeNum
	case CONCAT:
		return c.strExpr(l) + " + " + c.strExpr(r), typeStr
	case EQUALS, LESS, LTE, GREATER, GTE, NOT_EQUALS:
		ls, lt := c.expr(l)
		rs, rt := c.expr(r)
		switch lt {
		case typeNum:
			switch rt {
			case typeNum:
				return "_boolToNum(" + ls + " " + op.String() + " " + rs + ")", typeNum
			case typeStr:
				return "_boolToNum(_numToStr(" + ls + ") " + op.String() + " " + rs + ")", typeNum
			case typeNumStr:
				return "_boolToNum(" + ls + " " + op.String() + " _strToNum(" + rs + "))", typeNum
			}
		case typeStr:
			switch rt {
			case typeNum:
				return "_boolToNum(" + ls + " " + op.String() + " _numToStr(" + rs + "))", typeNum
			case typeStr, typeNumStr:
				return "_boolToNum(" + ls + " " + op.String() + " " + rs + ")", typeNum
			}
		case typeNumStr:
			switch rt {
			case typeNum:
				return "_boolToNum(_strToNum(" + ls + ") " + op.String() + " " + rs + ")", typeNum
			case typeStr:
				return "_boolToNum(" + ls + " " + op.String() + " " + rs + ")", typeNum
			case typeNumStr:
				panic(errorf("type on one side of %s comparison must be known", op))
			}
		}
		panic(errorf("unexpected types in %s %s %s", ls, op.String(), rs))
	case MATCH, NOT_MATCH:
		// TODO: pre-compile regex literals if r is string literal
		return "_regexMatch(" + c.strExpr(r) + ", " + c.strExpr(l) + ")", typeNum
	case AND, OR:
		// TODO: what to do about precedence / parentheses?
		return "_boolToNum(" + c.cond(l) + " " + op.String() + " " + c.cond(r) + ")", typeNum
	default:
		panic(errorf("unexpected binary operator %s", op))
	}
}

func (c *compiler) cond(expr Expr) string {
	s, t := c.expr(expr)
	if t == typeStr {
		s += ` != ""`
	} else {
		s += " != 0"
	}
	return s
}

func (c *compiler) numExpr(expr Expr) string {
	s, t := c.expr(expr)
	if t == typeStr {
		s = "_strToNum(" + s + ")"
	}
	return s
}

func (c *compiler) intExpr(expr Expr) string {
	e, ok := expr.(*NumExpr)
	if ok && e.Value == float64(int(e.Value)) {
		return strconv.Itoa(int(e.Value))
	}
	return "int(" + c.numExpr(expr) + ")"
}

func (c *compiler) strExpr(expr Expr) string {
	s, t := c.expr(expr)
	if t == typeNum {
		s = "_numToStr(" + s + ")"
	}
	return s
}

func (c *compiler) index(index []Expr) string {
	indexStr := ""
	for i, e := range index {
		if i > 0 {
			indexStr += ` + "\x1c" + `
		}
		s, t := c.expr(e)
		if t == typeNum {
			s = "_numToStr(" + s + ")"
		}
		indexStr += s
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
