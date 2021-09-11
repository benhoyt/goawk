package main

import (
	"fmt"
	"os"
	"strconv"

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
	prog    *Program
	imports map[string]bool
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
	"strconv"
	"strings"
)

func main() {
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

func _getField(line string, fields []string, i int) string {
	if i == 0 {
		return line
	}
	if i >= 1 && i <= len(fields) {
        return fields[i-1]
    }
    return ""
}

func _boolToNum(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func _numToStr(n float64) string {
	switch {
	case math.IsNaN(n):
		return "nan"
	case math.IsInf(n, 0):
		if n < 0 {
			return "-inf"
		} else {
			return "inf"
		}
	case n == float64(int(n)):
		return strconv.Itoa(int(n))
	default:
		return fmt.Sprintf("%.6g", n)
	}
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// Like strconv.ParseFloat, but parses at the start of string and
// allows things like "1.5foo".
func _strToNum(s string) float64 {
	// Skip whitespace at start
	i := 0
	for i < len(s) && asciiSpace[s[i]] != 0 {
		i++
	}
	start := i

	// Parse mantissa: optional sign, initial digit(s), optional '.',
	// then more digits
	gotDigit := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		gotDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		gotDigit = true
		i++
	}
	if !gotDigit {
		return 0
	}

	// Parse exponent ("1e" and similar are allowed, but ParseFloat
	// rejects them)
	end := i
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			end = i
		}
	}

	floatStr := s[start:end]
	f, _ := strconv.ParseFloat(floatStr, 64)
	return f // Returns infinity in case of "value out of range" error
}
`)
}

func (c *compiler) addImport(path string) {
	c.imports[path] = true
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
		s, _ := c.expr(stmt.Expr)
		c.output(s)

	case *PrintStmt:
		if stmt.Dest != nil {
			panic(errorf("print redirection not yet supported"))
		}
		c.output("fmt.Println(")
		for i, arg := range stmt.Args {
			if i > 0 {
				c.output(", ")
			}
			s, _ := c.expr(arg) // TODO: handle type
			c.output(s)
		}
		c.output(")")

	case *IfStmt:
		c.output("if ")
		s, t := c.expr(stmt.Cond)
		c.output(s)
		if t == typeStr {
			c.output(` != ""`)
		} else {
			c.output(" != 0")
		}
		c.output(" {\n")
		c.stmts(stmt.Body)
		c.output("}")
		if len(stmt.Else) > 0 {
			// TODO: handle "else if"
			c.output(" else {\n")
			c.stmts(stmt.Else)
			c.output("}")
		}

	default:
		panic(errorf("%T not yet supported", stmt))
	}
	c.output("\n")
}

type valueType int

const (
	typeStr valueType = iota + 1
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

	case *BinaryExpr:
		return c.binaryExpr(e.Op, e.Left, e.Right)

	default:
		panic(errorf("%T not yet supported", expr))
	}
}

func (c *compiler) binaryExpr(op Token, l, r Expr) (string, valueType) {
	switch op {
	case ADD, SUB, MUL, DIV, MOD:
		return c.numExpr(l) + " " + op.String() + " " + c.numExpr(r), typeNum
	case POW:
		c.addImport("math")
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
	default:
		panic(errorf("unexpected binary operator %s", op))
	}
}

func (c *compiler) numExpr(expr Expr) string {
	s, t := c.expr(expr)
	if t == typeStr {
		s = "_strToNum(" + s + ")"
	}
	return s
}

func (c *compiler) intExpr(expr Expr) string {
	// TODO: if integer NumExpr can avoid int(...)
	return "int(" + c.numExpr(expr) + ")"
}

func (c *compiler) strExpr(expr Expr) string {
	s, t := c.expr(expr)
	if t == typeNum {
		s = "_numToStr(" + s + ")"
	}
	return s
}
