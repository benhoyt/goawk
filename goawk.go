// Package goawk is an implementation of AWK with CSV support
//
// You can use the command-line "goawk" command or run AWK from your
// Go programs using the "interp" package. The command-line program
// has the same interface as regular awk:
//
//	goawk [-F fs] [-v var=value] [-f progfile | 'prog'] [file ...]
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
//	$ echo 'foo 12
//	> bar 34
//	> baz 56' >file.txt
//	$ goawk '{ sum += $2 } END { print sum }' file.txt
//	102
//
// To use GoAWK in your Go programs, see README.md or the "interp"
// package docs.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"unicode/utf8"

	"github.com/benhoyt/goawk/internal/compiler"
	"github.com/benhoyt/goawk/internal/cover"
	"github.com/benhoyt/goawk/internal/parseutil"
	"github.com/benhoyt/goawk/internal/resolver"
	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

const (
	version    = "v1.20.0"
	copyright  = "GoAWK " + version + " - Copyright (c) 2022 Ben Hoyt"
	shortUsage = "usage: goawk [-F fs] [-v var=value] [-f progfile | 'prog'] [file ...]"
	longUsage  = `Standard AWK arguments:
  -F separator      field separator (default " ")
  -f progfile       load AWK source from progfile (multiple allowed)
  -v var=value      variable assignment (multiple allowed)

Additional GoAWK features:
  -E progfile       load program, treat as last option, disable var=value args
  -H                parse header row and enable @"field" in CSV input mode
  -h, --help        show this help message
  -i mode           parse input into fields using CSV format (ignore FS and RS)
                    'csv|tsv [separator=<char>] [comment=<char>] [header]'
  -o mode           use CSV output for print with args (ignore OFS and ORS)
                    'csv|tsv [separator=<char>]'
  -version          show GoAWK version and exit

GoAWK debugging arguments:
  -coverappend      append to coverage profile instead of overwriting
  -covermode mode   set coverage mode: set, count (default "set")
  -coverprofile fn  write coverage profile to file
  -cpuprofile fn    write CPU profile to file
  -d                print parsed syntax tree to stdout and exit
  -da               print VM assembly instructions to stdout and exit
  -dt               print variable type information to stdout and exit
  -memprofile fn    write memory profile to file
`
)

func main() {
	// Parse command line arguments manually rather than using the
	// "flag" package, so we can support flags with no space between
	// flag and argument, like '-F:' (allowed by POSIX)
	var progFiles []string
	var vars []string
	fieldSep := " "
	cpuProfile := ""
	debug := false
	debugAsm := false
	debugTypes := false
	memProfile := ""
	inputMode := ""
	outputMode := ""
	header := false
	noArgVars := false
	coverMode := cover.ModeUnspecified
	coverProfile := ""
	coverAppend := false

	var i int
argsLoop:
	for i = 1; i < len(os.Args); i++ {
		// Stop on explicit end of args or first arg not prefixed with "-"
		arg := os.Args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}

		switch arg {
		case "-covermode":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -covermode")
			}
			i++
			coverMode = coverModeFromString(os.Args[i])
		case "-coverprofile":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -coverprofile")
			}
			i++
			coverProfile = os.Args[i]
		case "-coverappend":
			coverAppend = true
		case "-E":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -E")
			}
			i++
			progFiles = append(progFiles, os.Args[i])
			noArgVars = true
			i++
			break argsLoop
		case "-F":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -F")
			}
			i++
			fieldSep = os.Args[i]
		case "-f":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -f")
			}
			i++
			progFiles = append(progFiles, os.Args[i])
		case "-v":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -v")
			}
			i++
			vars = append(vars, os.Args[i])
		case "-cpuprofile":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -cpuprofile")
			}
			i++
			cpuProfile = os.Args[i]
		case "-d":
			debug = true
		case "-da":
			debugAsm = true
		case "-dt":
			debugTypes = true
		case "-H":
			header = true
		case "-h", "--help":
			fmt.Printf("%s\n\n%s\n\n%s", copyright, shortUsage, longUsage)
			os.Exit(0)
		case "-i":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -i")
			}
			i++
			inputMode = os.Args[i]
		case "-memprofile":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -memprofile")
			}
			i++
			memProfile = os.Args[i]
		case "-o":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -o")
			}
			i++
			outputMode = os.Args[i]
		case "-version", "--version":
			fmt.Println(version)
			os.Exit(0)
		default:
			switch {
			case strings.HasPrefix(arg, "-E"):
				progFiles = append(progFiles, arg[2:])
				noArgVars = true
				i++
				break argsLoop
			case strings.HasPrefix(arg, "-F"):
				fieldSep = arg[2:]
			case strings.HasPrefix(arg, "-f"):
				progFiles = append(progFiles, arg[2:])
			case strings.HasPrefix(arg, "-i"):
				inputMode = arg[2:]
			case strings.HasPrefix(arg, "-o"):
				outputMode = arg[2:]
			case strings.HasPrefix(arg, "-v"):
				vars = append(vars, arg[2:])
			case strings.HasPrefix(arg, "-cpuprofile="):
				cpuProfile = arg[len("-cpuprofile="):]
			case strings.HasPrefix(arg, "-memprofile="):
				memProfile = arg[len("-memprofile="):]
			case strings.HasPrefix(arg, "-covermode="):
				coverMode = coverModeFromString(arg[len("-covermode="):])
			case strings.HasPrefix(arg, "-coverprofile="):
				coverProfile = arg[len("-coverprofile="):]
			default:
				errorExitf("flag provided but not defined: %s", arg)
			}
		}
	}
	if coverProfile != "" && coverMode == cover.ModeUnspecified {
		coverMode = cover.ModeSet
	}

	// Any remaining args are program and input files
	args := os.Args[i:]

	fileReader := &parseutil.FileReader{}
	if len(progFiles) > 0 {
		// Read source: the concatenation of all source files specified
		progFiles = expandWildcardsOnWindows(progFiles)
		for _, progFile := range progFiles {
			if progFile == "-" {
				err := fileReader.AddFile("<stdin>", os.Stdin)
				if err != nil {
					errorExit(err)
				}
			} else {
				f, err := os.Open(progFile)
				if err != nil {
					errorExit(err)
				}
				err = fileReader.AddFile(progFile, f)
				if err != nil {
					_ = f.Close()
					errorExit(err)
				}
				_ = f.Close()
			}
		}
	} else {
		if len(args) < 1 {
			errorExitf(shortUsage)
		}
		err := fileReader.AddFile("<cmdline>", strings.NewReader(args[0]))
		if err != nil {
			errorExit(err)
		}
		args = args[1:]
	}

	// Parse source code and setup interpreter
	parserConfig := &parser.ParserConfig{
		DebugTypes:  debugTypes,
		DebugWriter: os.Stdout,
	}
	prog, err := parser.ParseProgram(fileReader.Source(), parserConfig)
	if err != nil {
		if err, ok := err.(*parser.ParseError); ok {
			name, line := fileReader.FileLine(err.Position.Line)
			fmt.Fprintf(os.Stderr, "%s:%d:%d: %s\n",
				name, line, err.Position.Column, err.Message)
			showSourceLine(fileReader.Source(), err.Position)
			os.Exit(1)
		}
		errorExitf("%s", err)
	}

	coverage := cover.New(coverMode, coverAppend, fileReader)

	if coverMode != cover.ModeUnspecified {
		astProgram := &prog.ResolvedProgram.Program
		coverage.Annotate(astProgram)

		// re-resolve annotated program
		prog.ResolvedProgram = *resolver.Resolve(astProgram, &resolver.Config{
			DebugTypes:  parserConfig.DebugTypes,
			DebugWriter: parserConfig.DebugWriter})

		// re-compile it
		prog.Compiled, err = compiler.Compile(&prog.ResolvedProgram)
		if err != nil {
			errorExitf("%s", err)
		}
	}

	if debug {
		fmt.Fprintln(os.Stdout, prog)
	}

	if debugAsm {
		err := prog.Disassemble(os.Stdout)
		if err != nil {
			errorExitf("could not disassemble program: %v", err)
		}
	}

	if debug || debugAsm || debugTypes {
		os.Exit(0)
	}

	if header {
		if inputMode == "" {
			errorExitf("-H only allowed together with -i")
		}
		inputMode += " header"
	}

	// Don't buffer output if stdout is a terminal (default output writer when
	// Config.Output is nil is a buffered version of os.Stdout).
	var stdout io.Writer
	stdoutInfo, err := os.Stdout.Stat()
	if err == nil && stdoutInfo.Mode()&os.ModeCharDevice != 0 {
		stdout = os.Stdout
	}

	config := &interp.Config{
		Argv0:     filepath.Base(os.Args[0]),
		Args:      expandWildcardsOnWindows(args),
		NoArgVars: noArgVars,
		Output:    stdout,
		Vars: []string{
			"FS", fieldSep,
			"INPUTMODE", inputMode,
			"OUTPUTMODE", outputMode,
		},
	}
	for _, v := range vars {
		equals := strings.IndexByte(v, '=')
		if equals < 0 {
			errorExitf("-v flag must be in format name=value")
		}
		name, value := v[:equals], v[equals+1:]
		// Oddly, -v must interpret escapes (issue #129)
		unescaped, err := lexer.Unescape(value)
		if err == nil {
			value = unescaped
		}
		config.Vars = append(config.Vars, name, value)
	}

	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			errorExitf("could not create CPU profile: %v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			errorExitf("could not start CPU profile: %v", err)
		}
	}

	// Run the program!
	interpreter, err := interp.New(prog)
	status, err := interpreter.Execute(config)

	if err != nil {
		errorExit(err)
	}

	if coverProfile != "" {
		err := coverage.WriteProfile(coverProfile, interpreter.Array(cover.ArrayName))
		if err != nil {
			errorExitf("unable to write coverage profile: %v", err)
		}
	}

	if cpuProfile != "" {
		pprof.StopCPUProfile()
	}
	if memProfile != "" {
		f, err := os.Create(memProfile)
		if err != nil {
			errorExitf("could not create memory profile: %v", err)
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			errorExitf("could not write memory profile: %v", err)
		}
		_ = f.Close()
	}

	os.Exit(status)
}

func coverModeFromString(mode string) cover.Mode {
	switch mode {
	case "set":
		return cover.ModeSet
	case "count":
		return cover.ModeCount
	default:
		errorExitf("-covermode can only be one of: set, count")
		return cover.ModeUnspecified
	}
}

// Show source line and position of error, for example:
//
//	BEGIN { x*; }
//	          ^
func showSourceLine(src []byte, pos lexer.Position) {
	lines := bytes.Split(src, []byte{'\n'})
	srcLine := string(lines[pos.Line-1])
	numTabs := strings.Count(srcLine[:pos.Column-1], "\t")
	runeColumn := utf8.RuneCountInString(srcLine[:pos.Column-1])
	fmt.Fprintln(os.Stderr, strings.Replace(srcLine, "\t", "    ", -1))
	fmt.Fprintln(os.Stderr, strings.Repeat(" ", runeColumn)+strings.Repeat("   ", numTabs)+"^")
}

func errorExit(err error) {
	pathErr, ok := err.(*os.PathError)
	if ok && os.IsNotExist(err) {
		errorExitf("file %q not found", pathErr.Path)
	}
	errorExitf("%s", err)
}

func errorExitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func expandWildcardsOnWindows(args []string) []string {
	if runtime.GOOS != "windows" {
		return args
	}
	return expandWildcards(args)
}

// Originally from https://github.com/mattn/getwild (compatible LICENSE).
func expandWildcards(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		matches, err := filepath.Glob(arg)
		if err == nil && len(matches) > 0 {
			result = append(result, matches...)
		} else {
			result = append(result, arg)
		}
	}
	return result
}
