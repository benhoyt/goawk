// Package goawk is an implementation of AWK written in Go.
//
// You can use the command-line "goawk" command or run AWK from your
// Go programs using the "interp" package. The command-line program
// has the same interface as regular awk:
//
//     goawk [-F fs] [-v var=value] [-f progfile | 'prog'] [file ...]
//
// The -F flag specifies the field separator (the default is to split
// on whitespace). The -v flag allows you to set a variable to a
// given value (multiple -v flags allowed). The -f flag allows you to
// read AWK source from a file instead of the 'prog' command-line
// argument. The rest of the arguments are input filenames (default
// is to read from stdin).
//
// A simple example (prints the sum of the numbers in the file's
// second column):
//
//     $ echo 'foo 12
//     > bar 34
//     > baz 56' >file.txt
//     $ goawk '{ sum += $2 } END { print sum }' file.txt
//     102
//
// To use GoAWK in your Go programs, see README.md or the "interp"
// docs.
//
package main

/*

TODO:
- performance testing: I/O, allocations, CPU
  + resolve array variables at parse time (by index instead of name)
  + resolve array parameters to functions at parse time and clean up userCall
  + buffer > and >> output (see TODO in getOutputStream)
  + other "escapes to heap" uses of make() in interp.go
  + writeOutput crlfNewline handling is probably slow
  + try using a typ and index for getVarIndex instead of just int?
  + benchmark against awk/gawk with some real awk scripts
  + why does writing output take 180ms with script '$0', but 630ms with script '/.$/'?
  + optimize parser
- convert benchmark_awks.py to Python 3
- break up interp.go? structure it better and add comments
- think about length() and substr() chars vs bytes:
  https://github.com/benhoyt/goawk/issues/2#issuecomment-415314000
- get goawk_test.go working in TravisCI
- consider adding non-POSIX behaviour: length(array)
- error if array is used as scalar and vice-versa
- try out Go 2 error handling proposal with the GoAWK codebase:
  https://go.googlesource.com/proposal/+/master/design/go2draft-error-handling.md

NICE TO HAVE:
- fix broken (commented-out) interp tests due to syntax handling
- think about proper CSV support: https://news.ycombinator.com/item?id=17788471
- think about linear time string concat: https://news.ycombinator.com/item?id=17788028
- support for calling Go functions: https://news.ycombinator.com/item?id=17788915
- parser: ensure vars aren't used in array context and vice-versa
- interp: flag "unexpected comma-separated expression" at parse time

*/

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"unicode/utf8"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

func main() {
	var progFiles multiString
	flag.Var(&progFiles, "f", "load AWK source from `progfile` (multiple allowed)")
	fieldSep := flag.String("F", " ", "field separator")
	var vars multiString
	flag.Var(&vars, "v", "name=value variable `assignment` (multiple allowed)")

	debug := flag.Bool("d", false, "debug mode (print parsed AST to stderr)")
	cpuprofile := flag.String("cpuprofile", "", "write CPU profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")

	flag.Parse()
	args := flag.Args()

	var src []byte
	if len(progFiles) > 0 {
		buf := &bytes.Buffer{}
		for _, progFile := range progFiles {
			if progFile == "-" {
				_, err := buf.ReadFrom(os.Stdin)
				if err != nil {
					errorExit("%s", err)
				}
			} else {
				f, err := os.Open(progFile)
				if err != nil {
					errorExit("%s", err)
				}
				_, err = buf.ReadFrom(f)
				if err != nil {
					f.Close()
					errorExit("%s", err)
				}
				f.Close()
			}
			buf.WriteByte('\n')
		}
		src = buf.Bytes()
	} else {
		if len(args) < 1 {
			errorExit("usage: goawk [-F fs] [-v var=value] [-f progfile | 'prog'] [file ...]")
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

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			errorExit("could not create CPU profile: %v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			errorExit("could not start CPU profile: %v", err)
		}
	}

	config := &interp.Config{
		Argv0: filepath.Base(os.Args[0]),
		Args:  args,
		Vars:  []string{"FS", *fieldSep},
	}
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			errorExit("-v flag must be in format name=value")
		}
		config.Vars = append(config.Vars, parts[0], parts[1])
	}

	status, err := interp.ExecProgram(prog, config)
	if err != nil {
		errorExit("%s", err)
	}

	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			errorExit("could not create memory profile: %v", err)
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			errorExit("could not write memory profile: %v", err)
		}
		f.Close()
	}

	os.Exit(status)
}

func showSourceLine(src []byte, pos lexer.Position, dividerLen int) {
	divider := strings.Repeat("-", dividerLen)
	if divider != "" {
		fmt.Fprintln(os.Stderr, divider)
	}
	lines := bytes.Split(src, []byte{'\n'})
	srcLine := string(lines[pos.Line-1])
	numTabs := strings.Count(srcLine[:pos.Column-1], "\t")
	runeColumn := utf8.RuneCountInString(srcLine[:pos.Column-1])
	fmt.Fprintln(os.Stderr, strings.Replace(srcLine, "\t", "    ", -1))
	fmt.Fprintln(os.Stderr, strings.Repeat(" ", runeColumn)+strings.Repeat("   ", numTabs)+"^")
	if divider != "" {
		fmt.Fprintln(os.Stderr, divider)
	}
}

func errorExit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

type multiString []string

func (m *multiString) String() string {
	return fmt.Sprintf("%v", []string(*m))
}

func (m *multiString) Set(value string) error {
	*m = append(*m, value)
	return nil
}
