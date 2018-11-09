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
  + PoC: is interpreting via a "heavy AST" faster?
    i.e., with execute functions on the AST elements instead of the switch/case
  + benchmark against awk/gawk with some real awk scripts
  + do a bit more cpu and mem profiling
- do some more fuzz testing
- release 1.0.0 (do anything for Go modules?)
- remove notice about "beta" from README.md

*/ // This comment intentionally left blank

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
	// Main AWK arguments
	var progFiles multiString
	flag.Var(&progFiles, "f", "load AWK source from `progfile` (multiple allowed)")
	fieldSep := flag.String("F", " ", "field separator")
	var vars multiString
	flag.Var(&vars, "v", "name=value variable `assignment` (multiple allowed)")

	// Debugging and profiling arguments
	debug := flag.Bool("d", false, "debug mode (print parsed AST to stderr)")
	debugTypes := flag.Bool("dt", false, "show variable types debug info")
	cpuprofile := flag.String("cpuprofile", "", "write CPU profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")

	flag.Parse()
	args := flag.Args()

	var src []byte
	if len(progFiles) > 0 {
		// Read source: the concatenation of all source files specified
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
			// Append newline to file in case it doesn't end with one
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

	// Parse source code and setup interpreter
	parserConfig := &parser.ParserConfig{
		DebugTypes:  *debugTypes,
		DebugWriter: os.Stderr,
	}
	prog, err := parser.ParseProgram(src, parserConfig)
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

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			errorExit("could not create CPU profile: %v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			errorExit("could not start CPU profile: %v", err)
		}
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

// For parse errors, show source line and position of error, eg:
//
// -----------------------------------------------------
// BEGIN { x*; }
//           ^
// -----------------------------------------------------
// parse error at 1:11: expected expression instead of ;
//
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

// Helper type for flag parsing to allow multiple -f and -v arguments
type multiString []string

func (m *multiString) String() string {
	return fmt.Sprintf("%v", []string(*m))
}

func (m *multiString) Set(value string) error {
	*m = append(*m, value)
	return nil
}
