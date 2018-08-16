// Package goawk is an implementation of AWK written in Go.
package main

/*

TODO:
- other interp tests
  - tests for recursion, locals vs globals, array params, etc
  - fix broken interp tests due to syntax handling
- performance testing: I/O, allocations, CPU
  + add "go test" benchmarks for various common workloads
  + faster to do switch+case for binary funcs instead of map of funcs?
  + getVar/setVar overhead -- can resolve stuff at compile-time
  + defer in eval/exec -- will this help?

NICE TO HAVE:
- parser: ensure vars aren't used in array context and vice-versa
- interp: flag "unexpected comma-separated expression" at parse time

*/

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

func main() {
	progFile := flag.String("f", "", "load AWK source from `progfile`")
	fieldSep := flag.String("F", " ", "field separator")
	var vars varFlags
	flag.Var(&vars, "v", "variable `assignment` (name=value)")

	debug := flag.Bool("d", false, "debug mode (print parsed AST to stderr)")
	cpuprofile := flag.String("cpuprofile", "", "write CPU profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")

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

	p := interp.New(nil, nil)
	p.SetVar("FS", *fieldSep)
	for _, v := range vars {
		p.SetVar(v.name, v.value)
	}

	interpArgs := []string{filepath.Base(os.Args[0])}
	interpArgs = append(interpArgs, args...)
	p.SetArgs(interpArgs)
	err = p.Exec(prog, os.Stdin, args)
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

type varFlag struct {
	name  string
	value string
}

type varFlags []varFlag

func (v *varFlags) String() string {
	return ""
}

func (v *varFlags) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return errors.New("must be name=value")
	}
	*v = append(*v, varFlag{parts[0], parts[1]})
	return nil
}
