// Implementation of (some of) AWK written in Go
package main

/*
TODO:
- error handling: InterpError and catch in top-level Evaluate and Execute
- lexing
- parsing
- testing (against other implementations?)
- performance testing: I/O, allocations, CPU

NICE TO HAVE:
- regex caching for user regexes
- implement printf / sprintf (probably have to do this by hand)
- multi-dimensional "in", multi-dimensional IndexExpr and SUBSEP

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
	fmt.Println(prog)
	fmt.Println("-----") // TODO

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
	if err != nil && err != ErrExit {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}

	err = interp.ExecuteEnd()
	if err == ErrExit {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}
}

func NumExpr(n float64) *ConstExpr {
	return &ConstExpr{Num(n)}
}

func StrExpr(s string) *ConstExpr {
	return &ConstExpr{Str(s)}
}

func Parse(src string) (*Program, error) {
	program := &Program{
		Begin: []Stmts{
			{
				// &ExprStmt{
				// 	&AssignExpr{&VarExpr{"OFS"}, "", StrExpr("|")},
				// },
				&ExprStmt{
					&CallExpr{"srand", []Expr{NumExpr(1.2)}},
				},
			},
		},
		Actions: []Action{
			{
				Pattern: &BinaryExpr{
					Left:  &FieldExpr{NumExpr(0)},
					Op:    "!=",
					Right: StrExpr(""),
				},
				Stmts: []Stmt{
					&PrintStmt{
						Args: []Expr{
							&CallSubExpr{StrExpr(`\.`), StrExpr(","), &FieldExpr{NumExpr(0)}, true},
							&FieldExpr{NumExpr(0)},
						},
					},
					// &ForInStmt{
					// 	Var:   "x",
					// 	Array: "a",
					// 	Body: []Stmt{
					// 		&PrintStmt{
					// 			Args: []Expr{
					// 				&VarExpr{"x"},
					// 				&IndexExpr{"a", &VarExpr{"x"}},
					// 			},
					// 		},
					// 	},
					// },
				},
			},
		},
	}
	return program, nil
}
