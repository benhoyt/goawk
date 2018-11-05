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
- fix crash with: BEGIN { print |"1"; getline <"1" }  # also print >"1"
- performance testing: I/O, allocations, CPU
  + PoC: is interpreting via a "heavy AST" faster?
    i.e., with execute functions on the AST elements instead of the switch/case
  + buffer > and >> output (see TODO in getOutputStream)
  + other "escapes to heap" uses of make() in interp.go
  + benchmark against awk/gawk with some real awk scripts
  + why does writing output take 180ms with script '$0', but 630ms with script '/.$/'?
  + optimize parser
- think about length() and substr() chars vs bytes:
  https://github.com/benhoyt/goawk/issues/2#issuecomment-415314000
- get goawk_test.go working in TravisCI

ISSUE - discrepancy against gawk on Windows:
   --- FAIL: TestAWK/t.printf2 (0.05s)
        goawk_test.go:128: output differs, run: git diff testdata\output\t.printf2; expected:
            %:  ... /dev/rrp3:         0            0 0 0 / <00>
            %:  ...          0            0 0 0 <00> <00>
            %: mel ... 17379     17379        mel 0 0 0 � m
            %: bwk ... 16693     16693        bwk 0 0 0 5 b
            %: ken ... 16116     16116        ken 0 0 0 � k
            %: srb ... 15713     15713        srb 0 0 0 a s
            %: lem ... 11895     11895        lem 0 0 0 w l
            %: scj ... 10409     10409        scj 0 0 0 � s
            %: rhm ... 10252     10252        rhm 0 0 0  r
    --- got:
            %:  ... /dev/rrp3:         0            0 0 0 / <00>
            %:  ...          0            0 0 0 <00> <00>
            %: mel ... 17379     17379        mel 0 0 0 䏣 m
            %: bwk ... 16693     16693        bwk 0 0 0 䄵 b
            %: ken ... 16116     16116        ken 0 0 0 㻴 k
            %: srb ... 15713     15713        srb 0 0 0 㵡 s
            %: lem ... 11895     11895        lem 0 0 0 ⹷ l
            %: scj ... 10409     10409        scj 0 0 0 ⢩ s
            %: rhm ... 10252     10252        rhm 0 0 0 ⠌ r

*/// This comment intentionally left blank

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
	debugTypes := flag.Bool("dt", false, "show variable types debug info")
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
