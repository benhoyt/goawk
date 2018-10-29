// Don't run these on Windows, because newline handling means they
// don't pass (TODO: report a Go bug?)

// +build !windows

package interp_test

import (
	"bytes"
	"fmt"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func Example_simple() {
	input := bytes.NewReader([]byte("foo bar\n\nbaz buz"))
	err := interp.Exec("$0 { print $1 }", " ", input, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Output:
	// foo
	// baz
}

func Example_fieldsep() {
	// Use ',' as the field separator
	input := bytes.NewReader([]byte("1,2\n3,4"))
	err := interp.Exec("{ print $1, $2 }", ",", input, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Output:
	// 1 2
	// 3 4
}

func Example_program() {
	src := "{ print NR, tolower($0) }"
	input := "A\naB\nAbC"

	prog, err := parser.ParseProgram([]byte(src), nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	config := &interp.Config{
		Stdin: bytes.NewReader([]byte(input)),
		Vars:  []string{"OFS", ":"},
	}
	_, err = interp.ExecProgram(prog, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Output:
	// 1:a
	// 2:ab
	// 3:abc
}
