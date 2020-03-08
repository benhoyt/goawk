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

import (
	"bytes"
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

const (
	version    = "v1.6.0"
	copyright  = "GoAWK " + version + " - Copyright (c) 2019 Ben Hoyt"
	shortUsage = "usage: goawk [-F fs] [-v var=value] [-f progfile | 'prog'] [file ...]"
	longUsage  = `Standard AWK arguments:
  -F separator
        field separator (default " ")
  -v assignment
        name=value variable assignment (multiple allowed)
  -f progfile
        load AWK source from progfile (multiple allowed)

Additional GoAWK arguments:
  -cpuprofile file
        write CPU profile to file
  -d    debug mode (print parsed AST to stderr)
  -dt   show variable types debug info
  -h    show this usage message
  -version
        show GoAWK version and exit
`
)

func main() {
	// Parse command line arguments manually rather than using the
	// "flag" package so we can support flags with no space between
	// flag and argument, like '-F:' (allowed by POSIX)
	args := make([]string, 0)
	fieldSep := " "
	progFiles := make([]string, 0)
	vars := make([]string, 0)
	cpuprofile := ""
	debug := false
	debugTypes := false
	memprofile := ""
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-F":
			if i+1 >= len(os.Args) {
				errorExit("flag needs an argument: -F")
			}
			i++
			fieldSep = os.Args[i]
		case "-f":
			if i+1 >= len(os.Args) {
				errorExit("flag needs an argument: -f")
			}
			i++
			progFiles = append(progFiles, os.Args[i])
		case "-v":
			if i+1 >= len(os.Args) {
				errorExit("flag needs an argument: -v")
			}
			i++
			vars = append(vars, os.Args[i])
		case "-cpuprofile":
			if i+1 >= len(os.Args) {
				errorExit("flag needs an argument: -cpuprofile")
			}
			i++
			cpuprofile = os.Args[i]
		case "-d":
			debug = true
		case "-dt":
			debugTypes = true
		case "-h", "--help":
			fmt.Printf("%s\n\n%s\n\n%s", copyright, shortUsage, longUsage)
			os.Exit(0)
		case "-memprofile":
			if i+1 >= len(os.Args) {
				errorExit("flag needs an argument: -memprofile")
			}
			i++
			memprofile = os.Args[i]
		case "-version", "--version":
			fmt.Println(version)
			os.Exit(0)
		default:
			arg := os.Args[i]
			switch {
			case strings.HasPrefix(arg, "-F"):
				fieldSep = arg[2:]
			case strings.HasPrefix(arg, "-f"):
				progFiles = append(progFiles, arg[2:])
			case strings.HasPrefix(arg, "-v"):
				vars = append(vars, arg[2:])
			case strings.HasPrefix(arg, "-cpuprofile="):
				cpuprofile = arg[12:]
			case strings.HasPrefix(arg, "-memprofile="):
				memprofile = arg[12:]
			case len(arg) > 1 && arg[0] == '-':
				errorExit("flag provided but not defined: %s", arg)
			default:
				args = append(args, arg)
			}
		}
	}

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
			errorExit(shortUsage)
		}
		src = []byte(args[0])
		args = args[1:]
	}

	// Parse source code and setup interpreter
	parserConfig := &parser.ParserConfig{
		DebugTypes:  debugTypes,
		DebugWriter: os.Stderr,
	}
	prog, err := parser.ParseProgram(src, parserConfig)
	if err != nil {
		errMsg := fmt.Sprintf("%s", err)
		if err, ok := err.(*parser.ParseError); ok {
			showSourceLine(src, err.Position, len(errMsg))
		}
		errorExit("%s", errMsg)
	}
	if debug {
		fmt.Fprintln(os.Stderr, prog)
	}
	config := &interp.Config{
		Argv0: filepath.Base(os.Args[0]),
		Args:  args,
		Vars:  []string{"FS", fieldSep},
	}
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			errorExit("-v flag must be in format name=value")
		}
		config.Vars = append(config.Vars, parts[0], parts[1])
	}

	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
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

	if cpuprofile != "" {
		pprof.StopCPUProfile()
	}

	if memprofile != "" {
		f, err := os.Create(memprofile)
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
