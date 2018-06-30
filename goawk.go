// Implementation of (some of) AWK written in Go
package main

/*
TODO:
- API surface: what should be exported?
  lexer:
    Lexer
    NewLexer(input []byte) *Lexer
    .Scan() (Token, string)
    PLUS, MINUS, DOT, ...
    // can be imported into parser with "."
  parser:
    ParseProgram(input []byte) (*Program, error)
    ParseExpr(input []byte) (Expr, error)
    Expr, BinaryExpr, Stmt, PrintStmt, ...
    // can be imported into interp with "."
  interp:
    Interp
    New(output io.Writer) *Interp
    .SetVar(name, value string) error
    .SetField(index int, value string)
    .ExecBegin(p *Program) error
    .ExecFile(p *Program, filename string, input io.Reader) error
    .ExecEnd(p *Program) error
    .ExecExpr(expr Expr) (string, error) // need to return more than string?
    ExecExpr(expr string) (string, error)
    ExecExprLine(expr, line string) (string, error) // maybe not?
- lexing
- parsing
- testing (against other implementations?)
- performance testing: I/O, allocations, CPU

NICE TO HAVE:
- regex caching for user regexes
- implement printf / sprintf (probably have to do this by hand)
- multi-dimensional "in", multi-dimensional IndexExpr and SUBSEP
- I don't think interp.SetArray is concurrency-safe

*/

import (
	"fmt"
	"os"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "usage: goawk src [filename] ...\n")
		os.Exit(4)
	}

	src := os.Args[1]
	prog, err := parser.Parse(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		os.Exit(3)
	}
	fmt.Println(prog)
	fmt.Println("-----") // TODO

	p := interp.NewInterp(prog, os.Stdout)
	err = p.ExecuteBegin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}

	if len(os.Args) <= 2 {
		err = p.ExecuteFile("", os.Stdin)
	} else {
		for _, filename := range os.Args[2:] {
			f, errOpen := os.Open(filename)
			if errOpen != nil {
				fmt.Fprintf(os.Stderr, "can't open %q: %v\n", filename, errOpen)
				os.Exit(2)
			}
			err = p.ExecuteFile(filename, f)
			f.Close()
			if err != nil {
				break
			}
		}
	}
	if err != nil && err != interp.ErrExit {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}

	err = p.ExecuteEnd()
	if err == interp.ErrExit {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}
}
