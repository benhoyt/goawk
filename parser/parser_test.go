// Test parser package

package parser_test

import (
	"fmt"

	"github.com/benhoyt/goawk/parser"
)

// NOTE: parser doesn't have its own tests, as the idea is to test
// the parser in the interp tests.

func Example_valid() {
	prog, err := parser.ParseProgram([]byte("$0 { print $1 }"))
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(prog)
	}
	// Output:
	// $0 {
	//     print $1
	// }
}

func Example_error() {
	prog, err := parser.ParseProgram([]byte("{ for if }"))
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(prog)
	}
	// Output:
	// parse error at 1:7: expected ( instead of if
}
