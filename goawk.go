// GoAWK: an implementation of (some of) AWK written in Go.
package main

/*

TODO:
- testing (against other implementations?)
    - t.null0 not working due to handling of uninitialized values:
		- StackOverflow question: https://stackoverflow.com/questions/51632945
    - t.NF not working; in awk, this program
      '{ OFS = "|"; print NF; NF = 2; print NF; print; }'
      produces different output when run as a sub-process (eg: os/exec)
      vs when run from the command line -- why? which is correct?

    - shouldn't allow syntax: { $1 = substr($1, 1, 3) print $1 }
    - should allow: NR==1, NR==2 { print "A", $0 };  NR==4, NR==6 { print "B", $0 }
      needs to look for semicolon after statement block?
- other lexer, parser, and interpreter tests
  - test user-defined functions: recursion, locals vs globals, array params, etc
  - does awk treat for-in variable as local?
- other TODOs:
  + interp: TODOs about parse checking in userCall
  + interp: srand(): truncating fraction, return previous seed
- performance testing: I/O, allocations, CPU

NICE TO HAVE:
- parser: ensure vars aren't used in array context and vice-versa
- interp: flag "unexpected comma-separated expression" at parse time
- I don't think interp.SetArray is concurrency-safe

*/

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

	p := interp.New(prog, nil, nil)
	interpArgs := []string{filepath.Base(os.Args[0])}
	interpArgs = append(interpArgs, args...)
	p.SetArgs(interpArgs)
	err = p.Exec(os.Stdin, args)
	if err != nil {
		errorExit("%s", err)
	}
	os.Exit(p.ExitStatus())
}

func showSourceLine(src []byte, pos lexer.Position, dividerLen int) {
	divider := strings.Repeat("-", dividerLen)
	if divider != "" {
		fmt.Fprintln(os.Stderr, divider)
	}
	lines := bytes.Split(src, []byte{'\n'})
	srcLine := string(lines[pos.Line-1])
	numTabs := strings.Count(srcLine[:pos.Column-1], "\t")
	fmt.Fprintln(os.Stderr, strings.Replace(srcLine, "\t", "    ", -1))
	fmt.Fprintln(os.Stderr, strings.Repeat(" ", pos.Column-1)+strings.Repeat("   ", numTabs)+"^")
	if divider != "" {
		fmt.Fprintln(os.Stderr, divider)
	}
}

func errorExit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
