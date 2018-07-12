// GoAWK: an implementation of (some of) AWK written in Go.
package main

/*
TODO:

- testing (against other implementations?)
    - need the 17EZ / 30p float parsing behaviour in value.numStr() as well?
    - implement printf / sprintf (scan to determine types, then call Sprintf?)
    - shouldn't allow syntax: { $1 = substr($1, 1, 3) print $1 }
    - should allow: NR==1, NR==2 { print "A", $0 };  NR==4, NR==6 { print "B", $0 }
      needs to look for semicolon after statement block?
    - proper parsing of div (instead of regex)
		"In some contexts, a <slash> ( '/' ) that is used to surround an ERE
		could also be the division operator. This shall be resolved in such a
		way that wherever the division operator could appear, a <slash> is
		assumed to be the division operator. (There is no unary division operator.)"
- performance testing: I/O, allocations, CPU

NICE TO HAVE:
- ampersand handling in sub/gsub
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
	debug := flag.Bool("d", false, "debug mode (print parsed AST on stderr)")
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
	if *debug {
		fmt.Fprintln(os.Stderr, prog)
	}

	p := interp.New(os.Stdout)
	if len(args) < 1 {
		err = p.ExecStream(prog, os.Stdin)
	} else {
		err = p.ExecFiles(prog, args)
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
