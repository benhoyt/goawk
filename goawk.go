// Implementation of (some of) AWK written in Go
package main

/*
TODO:
- built-in variables (and assignments)
- built-in function calls
- add other expressions:
    post inc/dec
    pre inc/dec
    unary not, plus, minus
    equality and inequality
    regex and regex not
    "in" and multi-dimensional "in"
    logical and
    logical or
    cond ?:
- error handling: InterpError and catch in Evaluate and Execute
- lexing
- parsing

OTHER:
* support for assigning $0 and $1...
* support for assigning RS (instead of newline)
* ENVIRON built-in variable

*/

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "usage: goawk src [filename] ...\n")
		os.Exit(3)
	}
	src := os.Args[1]

	var err error
	if len(os.Args) <= 2 {
		err = Run("", src, os.Stdin)
	} else {
		filename := os.Args[2]
		f, errOpen := os.Open(filename)
		if errOpen != nil {
			fmt.Fprintf(os.Stderr, "can't open file %q\n", filename)
			os.Exit(2)
		}
		defer f.Close()
		err = Run(filename, src, f)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}
}

func Run(filename, src string, input io.Reader) error {
	prog, err := Parse(src)
	if err != nil {
		return err
	}
	fmt.Println(prog)
	fmt.Println("-----")

	interp := NewInterp()

	for _, ss := range prog.Begin {
		interp.Executes(ss)
	}

	interp.SetFile(filename)

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		interp.NextLine(scanner.Text())
		for _, a := range prog.Actions {
			pattern := interp.Evaluate(a.Pattern)
			if ToBool(pattern) {
				interp.Executes(a.Stmts)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading lines from input: %s", err)
	}

	for _, ss := range prog.End {
		interp.Executes(ss)
	}

	return nil
}

func Parse(src string) (*Program, error) {
	program := &Program{
		Begin: []Stmts{
			{
				&ExprStmt{
					&AssignExpr{&VarExpr{"FILENAME"}, &ConstExpr{"FooFile"}},
				},
			},
		},
		Actions: []Action{
			{
				Pattern: &BinaryExpr{
					Left:  &FieldExpr{&ConstExpr{0.0}},
					Op:    "!=",
					Right: &ConstExpr{""},
				},
				Stmts: []Stmt{
					&ExprStmt{
						&AssignExpr{&FieldExpr{&ConstExpr{0.0}}, &ConstExpr{"HELLO 2 3"}},
					},
					&PrintStmt{
						Args: []Expr{
							&VarExpr{"FILENAME"},
							&FieldExpr{&ConstExpr{1.0}},
							&BinaryExpr{
								Left:  &FieldExpr{&ConstExpr{2.0}},
								Op:    "*",
								Right: &FieldExpr{&ConstExpr{3.0}},
							},
						},
					},
				},
			},
		},
	}
	return program, nil
}
