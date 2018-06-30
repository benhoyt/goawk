// GoAWK: an implementation of (some of) AWK written in Go.
package main

/*
TODO:
- lexing
    Lexer
    NewLexer(input []byte) *Lexer
    .Scan() (Token, string)
    PLUS, MINUS, DOT, ...
    // can be imported into parser with "."
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
	prog, err := parser.ParseProgram([]byte(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		os.Exit(3)
	}
	fmt.Println(prog)
	fmt.Println("-----") // TODO

	p := interp.New(os.Stdout)
	err = p.ExecBegin(prog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}

	if len(os.Args) <= 2 {
		err = p.ExecFile(prog, "", os.Stdin)
	} else {
		for _, filename := range os.Args[2:] {
			f, errOpen := os.Open(filename)
			if errOpen != nil {
				fmt.Fprintf(os.Stderr, "can't open %q: %v\n", filename, errOpen)
				os.Exit(2)
			}
			err = p.ExecFile(prog, filename, f)
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

	err = p.ExecEnd(prog)
	if err == interp.ErrExit {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute error: %s\n", err)
		os.Exit(1)
	}
}
