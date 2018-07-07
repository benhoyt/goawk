// GoAWK: an implementation of (some of) AWK written in Go.
package main

/*
TODO:
- parsing
- testing (against other implementations?)
- performance testing: I/O, allocations, CPU

NICE TO HAVE:
- parser: ensure vars aren't used in array context and vice-versa
- regex caching for user regexes
- implement printf / sprintf (probably have to do this by hand)
- multi-dimensional "in", multi-dimensional IndexExpr and SUBSEP
- range patterns
- I don't think interp.SetArray is concurrency-safe

*/

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "usage: goawk src [filename] ...\n")
		os.Exit(4)
	}

	src := []byte(os.Args[1])
	prog, err := parser.ParseProgram(src)
	if err != nil {
		errMsg := fmt.Sprintf("%s", err)
		if err, ok := err.(*parser.ParseError); ok {
			showSourceLine(src, err.Position, len(errMsg))
		}
		fmt.Fprintln(os.Stderr, errMsg)
		os.Exit(3)
	}
	fmt.Println(prog)
	fmt.Println("-----") // TODO

	p := interp.New(os.Stdout)
	err = p.ExecBegin(prog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
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
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	err = p.ExecEnd(prog)
	if err == interp.ErrExit {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func showSourceLine(src []byte, pos lexer.Position, dividerLen int) {
	divider := strings.Repeat("-", dividerLen)
	if divider != "" {
		fmt.Println(divider)
	}
	lines := bytes.Split(src, []byte{'\n'})
	srcLine := string(lines[pos.Line-1])
	numTabs := strings.Count(srcLine[:pos.Column-1], "\t")
	fmt.Println(strings.Replace(srcLine, "\t", "    ", -1))
	fmt.Println(strings.Repeat(" ", pos.Column-1) + strings.Repeat("   ", numTabs) + "^")
	if divider != "" {
		fmt.Println(divider)
	}
}
