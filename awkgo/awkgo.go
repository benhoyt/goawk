// AWKGo: AWK to Go compiler

/*
NOT SUPPORTED:
- functions
- dynamic typing
- non-literal [s]printf format strings?
- assigning numStr values (but using $0 in conditionals works)
- null values (unset number variable should output "", we output "0")
- print redirection
- getline
- reference to nonexistent array element should create it (POSIX, but yuck)
- some forms of augmented assignment (no reason we can't, just TODO)
*/

package main

import (
	"bytes"
	"fmt"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

const (
	version    = "v1.0.0"
	copyright  = "AWKGo " + version + " - Copyright (c) 2021 Ben Hoyt"
	shortUsage = "usage: awkgo [-f progfile | 'prog']"
	longUsage  = `AWKGo arguments:
  -f progfile
        load AWK source from progfile (multiple allowed)
  -h    show this usage message
  -version
        show AWKGo version and exit
`
)

func main() {
	var progFiles []string

	var i int
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
		case "-f":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -f")
			}
			i++
			progFiles = append(progFiles, os.Args[i])
		case "-h", "--help":
			fmt.Printf("%s\n\n%s\n\n%s", copyright, shortUsage, longUsage)
			os.Exit(0)
		case "-version", "--version":
			fmt.Println(version)
			os.Exit(0)
		default:
			switch {
			case strings.HasPrefix(arg, "-f"):
				progFiles = append(progFiles, arg[2:])
			default:
				errorExitf("flag provided but not defined: %s", arg)
			}
		}
	}

	// Any remaining arg is the program source
	args := os.Args[i:]

	var src []byte
	if len(progFiles) > 0 {
		// Read source: the concatenation of all source files specified
		buf := &bytes.Buffer{}
		progFiles = expandWildcardsOnWindows(progFiles)
		for _, progFile := range progFiles {
			if progFile == "-" {
				_, err := buf.ReadFrom(os.Stdin)
				if err != nil {
					errorExit(err)
				}
			} else {
				f, err := os.Open(progFile)
				if err != nil {
					errorExit(err)
				}
				_, err = buf.ReadFrom(f)
				if err != nil {
					_ = f.Close()
					errorExit(err)
				}
				_ = f.Close()
			}
			// Append newline to file in case it doesn't end with one
			_ = buf.WriteByte('\n')
		}
		src = buf.Bytes()
	} else {
		if len(args) < 1 {
			errorExitf(shortUsage)
		}
		src = []byte(args[0])
		args = args[1:]
	}

	if len(args) > 0 {
		errorExitf("additional arguments not allowed: %v\n%s", strings.Join(args, " "), shortUsage)
	}

	prog, err := parser.ParseProgram(src, nil)
	if err != nil {
		errMsg := fmt.Sprintf("%s", err)
		if err, ok := err.(*parser.ParseError); ok {
			showSourceLine(src, err.Position, len(errMsg))
		}
		errorExitf("%s", errMsg)
	}

	err = compile(prog, os.Stdout)
	if err != nil {
		errorExitf("%v", err)
	}
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
