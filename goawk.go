// GoAWK: an implementation of (some of) AWK written in Go.
package main

/*
TODO:
- testing (against other implementations?)
    - support bare "length" without parens
    - parsing of comments
    - proper parsing of div (instead of regex), eg: k/n (p.48b)
    - \ line continuation (p.26, p.26a)
    - fix regex parsing
    - implement printf / sprintf (probably have to do this by hand)
    - range patterns
- performance testing: I/O, allocations, CPU

NICE TO HAVE:
- parser: ensure vars aren't used in array context and vice-versa
- regex caching for user regexes
- multi-dimensional "in", multi-dimensional IndexExpr and SUBSEP
- print redirection, eg: { print >"tempbig" }
- print piping, eg: print c ":" pop[c] | "sort"
- user-defined functions
- I don't think interp.SetArray is concurrency-safe

*/

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

func main() {
	progFile := flag.String("f", "", "load AWK source from `filename`")
	flag.Parse()
	args := flag.Args()

	var src []byte
	if *progFile != "" {
		var err error
		src, err = ioutil.ReadFile(*progFile)
		if err != nil {
			errorExit("%s", err)
		}
	} else {
		if len(args) < 1 {
			errorExit("usage: goawk (src | -f path) [filename] ...")
		}
		src = []byte(args[0])
		args = args[1:]
	}

	prog, err := parser.ParseProgram(src)
	if err != nil {
		errMsg := fmt.Sprintf("%s", err)
		if err, ok := err.(*parser.ParseError); ok {
			showSourceLine(src, err.Position, len(errMsg))
		}
		errorExit(errMsg)
	}
	fmt.Println(prog)
	fmt.Println("-----") // TODO

	p := interp.New(os.Stdout)
	err = p.ExecBegin(prog)
	if err != nil {
		errorExit("%s", err)
	}

	if len(args) < 1 {
		err = p.ExecFile(prog, "", os.Stdin)
	} else {
		for _, filename := range args {
			f, errOpen := os.Open(filename)
			if errOpen != nil {
				errorExit("%s", errOpen)
			}
			err = p.ExecFile(prog, filename, f)
			f.Close()
			if err != nil {
				break
			}
		}
	}
	if err != nil && err != interp.ErrExit {
		errorExit("%s", err)
	}

	err = p.ExecEnd(prog)
	if err == interp.ErrExit {
		return
	}
	if err != nil {
		errorExit("%s", err)
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

func errorExit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
