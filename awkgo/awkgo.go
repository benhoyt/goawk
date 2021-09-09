package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/parser"
)

func main() {
	prog, err := parser.ParseProgram([]byte(os.Args[1]), nil)
	if err != nil {
		panic(err)
	}
	c := &compiler{}
	c.program(prog)
}

func error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\nERROR: "+format+"\n", args...)
	os.Exit(1)
}

type compiler struct {
	prog *parser.Program
}

func (c *compiler) output(s string) {
	fmt.Print(s)
}

func (c *compiler) program(prog *parser.Program) {
	c.output(`package main

import (
	"bufio"
	"fmt"
	"os"
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
`)
}

func (c *compiler) actions(actions []ast.Action) {
	for _, action := range actions {
		if len(action.Pattern) > 0 {
			error("patterns not yet supported")
		}
		if len(action.Stmts) == 0 {
			error("must have an action")
		}
		c.stmts(action.Stmts)
	}
}

func (c *compiler) stmts(stmts ast.Stmts) {
	for _, stmt := range stmts {
		c.stmt(stmt)
	}
}

func (c *compiler) stmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.PrintStmt:
		if s.Dest != nil {
			error("print redirection not yet supported")
		}
		c.output("fmt.Println(")
		for i, arg := range s.Args {
			if i > 0 {
				c.output(", ")
			}
			c.expr(arg)
		}
		c.output(")")
	default:
		error("%T not yet supported", stmt)
	}
	c.output("\n")
}

func (c *compiler) expr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.NumExpr:
		c.output(fmt.Sprintf("%g", e.Value))

	case *ast.StrExpr:
		c.output(strconv.Quote(e.Value))

	case *ast.FieldExpr:
		c.output("_getField(_line, _fields, ")
		c.expr(e.Index) // TODO: what if this is a string or float?
		c.output(")")
	default:
		error("%T not yet supported", expr)
	}
}
