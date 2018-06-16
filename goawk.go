// Implementation of (some of) AWK written in Go
package main

/*
TODO:
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

*/

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "usage: goawk src [filename] ...\n")
		os.Exit(4)
	}

	src := os.Args[1]
	prog, err := Parse(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		os.Exit(3)
	}

	interp := NewInterp(prog, os.Stdout)
	err = interp.ExecuteBegin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}

	if len(os.Args) <= 2 {
		err = interp.ExecuteFile("", os.Stdin)
	} else {
		for _, filename := range os.Args[2:] {
			f, errOpen := os.Open(filename)
			if errOpen != nil {
				fmt.Fprintf(os.Stderr, "can't open file %q\n", filename)
				os.Exit(2)
			}
			err = interp.ExecuteFile(filename, f)
			f.Close()
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}

	err = interp.ExecuteEnd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}
}

func Parse(src string) (*Program, error) {
	program := &Program{
		Begin: []Stmts{
			{
				&ExprStmt{
					&AssignExpr{&VarExpr{"OFS"}, &ConstExpr{"|"}},
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
					// &ExprStmt{
					// 	&AssignExpr{&FieldExpr{&ConstExpr{0.0}}, &ConstExpr{"HELLO 2 3"}},
					// },
					&PrintStmt{
						Args: []Expr{
							&VarExpr{"FILENAME"},
							&VarExpr{"NR"},
							&VarExpr{"FNR"},
							&VarExpr{"NF"},
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
