// Test parser package

package parser_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/parser"
)

// NOTE: apart from TestParseAndString, the parser doesn't have
// extensive tests of its own; the idea is to test the parser in the
// interp tests.

func TestParseAndString(t *testing.T) {
	// This program should have one of every AST element to ensure
	// we can parse and String()ify each.
	source := strings.TrimSpace(`
BEGIN {
    print "begin one"
}

BEGIN {
    print "begin two"
}

{
    print "empty pattern"
}

$0 {
    print "normal pattern"
    print 1, 2, 3
    printf "%.3f", 3.14159
    print "x" >"file"
    print "x" >>"append"
    print "y" |"prog"
    delete a
    delete a[k]
    if (c) {
        get(a, k)
    }
    if (1 + 2) {
        get(a, k)
    } else {
        set(a, k, v)
    }
    for (i = 0; i < 10; i++) {
        print i
        continue
    }
    for (k in a) {
        break
    }
    while (0) {
        print "x"
    }
    do {
        print "y"
        exit status
    } while (x)
    next
    nextfile
    "cmd" |getline
    "cmd" |getline x
    "cmd" |getline a[1]
    "cmd" |getline $1
    getline
    getline x
    (getline x + 1)
    getline $1
    getline a[1]
    getline <"file"
    getline x <"file"
    (getline x <"file" "x")
    getline $1 <"file"
    getline a[1] <"file"
    x = 0
    y = z = 0
    b += 1
    c -= 2
    d *= 3
    e /= 4
    g ^= 5
    h %= 6
    (x ? "t" : "f")
    ((b && c) || d)
    (k in a)
    ((x, y, z) in a)
    (s ~ "foo")
    (b < 1)
    (c <= 2)
    (d > 3)
    (e >= 4)
    (g == 5)
    (h != 6)
    ((x y) z)
    ((b + c) + d)
    ((b * c) * d)
    ((b - c) - d)
    ((b / c) / d)
    (b ^ (c ^ d))
    x++
    x--
    ++y
    --y
    1234
    1.5
    "This is a string"
    if (/a.b/) {
        print "match"
    }
    $1
    $(1 + 2)
    !x
    +x
    -x
    var
    a[key]
    a[x, y, z]
    f()
    set(a, k, v)
    sub(regex, repl)
    sub(regex, repl, s)
    gsub(regex, repl)
    gsub(regex, repl, s)
    split(s, a)
    split(s, a, regex)
    match(s, regex)
    rand()
    srand()
    srand(1)
    length()
    length($1)
    sprintf("")
    sprintf("%.3f", 3.14159)
    sprintf("%.3f %d", 3.14159, 42)
    cos(1)
    sin(1)
    exp(1)
    log(1)
    sqrt(1)
    int("42")
    tolower("FOO")
    toupper("foo")
    system("ls")
    close("file")
    atan2(x, y)
    index(haystack, needle)
    {
        print "block statement"
        f()
    }
}

(NR == 1), (NR == 2) {
    print "range pattern"
}

($1 == "foo")

END {
    print "end one"
}

END {
    print "end two"
}

function f() {
}

function get(a, k) {
    return a[k]
}

function set(a, k, v) {
    a[k] = v
    return
}
`)
	prog, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		t.Fatalf("error parsing program: %v", err)
	}
	progStr := prog.String()
	if progStr != source {
		t.Fatalf("expected first, got second:\n%s\n----------\n%s", source, progStr)
	}
}

func TestResolveLargeCallGraph(t *testing.T) {
	const numCalls = 10000

	var buf bytes.Buffer
	var i int
	for i = 0; i < numCalls; i++ {
		fmt.Fprintf(&buf, "function f%d(a) { return f%d(a) }\n", i, i+1)
	}
	fmt.Fprintf(&buf, "function f%d(a) { return a }\n", i)
	fmt.Fprint(&buf, "BEGIN { printf f0(42) }\n")
	_, err := parser.ParseProgram(buf.Bytes(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf.Reset()
	fmt.Fprint(&buf, "BEGIN { printf f0(42) }\n")
	fmt.Fprintf(&buf, "function f%d(a) { return a }\n", numCalls)
	for i = numCalls - 1; i >= 0; i-- {
		fmt.Fprintf(&buf, "function f%d(a) { return f%d(a) }\n", i, i+1)
	}
	_, err = parser.ParseProgram(buf.Bytes(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPositions(t *testing.T) {
	source := strings.TrimSpace(`
function AddNums(n,   sum,i) {
  sum = 0
  if (n%2 == 0) {
    for (i = 0; i < n; i++) {
      sum += i * 2
    }
  } else if (n%3 == 0) {
    print "Divides by 3"
  } else {
    sum = -1
  }
  while(1) { do { print 123 } while(1) }
  return sum
}`)
	expectedAst := strings.TrimSpace(`
*ast.Program
 *ast.Function
  *ast.ExprStmt [2:3-2:10)
   *ast.AssignExpr
    *ast.VarExpr
    *ast.NumExpr
  *ast.IfStmt [3:3-12:3)
   *ast.BinaryExpr
    *ast.BinaryExpr
     *ast.VarExpr
     *ast.NumExpr
    *ast.NumExpr
   *ast.ForStmt [4:5-6:6)
    *ast.ExprStmt [4:10-4:15)
     *ast.AssignExpr
      *ast.VarExpr
      *ast.NumExpr
    *ast.BinaryExpr
     *ast.VarExpr
     *ast.VarExpr
    *ast.ExprStmt [4:24-4:27)
     *ast.IncrExpr
      *ast.VarExpr
    *ast.ExprStmt [5:7-5:19)
     *ast.AugAssignExpr
      *ast.VarExpr
      *ast.BinaryExpr
       *ast.VarExpr
       *ast.NumExpr
   *ast.IfStmt [7:10-11:4)
    *ast.BinaryExpr
     *ast.BinaryExpr
      *ast.VarExpr
      *ast.NumExpr
     *ast.NumExpr
    *ast.PrintStmt [8:5-8:25)
     *ast.StrExpr
    *ast.ExprStmt [10:5-10:13)
     *ast.AssignExpr
      *ast.VarExpr
      *ast.UnaryExpr
       *ast.NumExpr
  *ast.WhileStmt [12:3-12:41)
   *ast.NumExpr
   *ast.DoWhileStmt [12:14-12:40)
    *ast.PrintStmt [12:19-12:29)
     *ast.NumExpr
    *ast.NumExpr
  *ast.ReturnStmt [13:3-13:13)
   *ast.VarExpr
`)
	prog, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		t.Fatalf("error parsing program: %v", err)
	}
	code := &code{}
	ast.Walk(code, &prog.ResolvedProgram.Program)
	result := strings.TrimSpace(code.buf.String())
	if expectedAst != result {
		t.Fatalf("Wrong AST and/or positions:\n%s", result)
	}
}

type code struct {
	indent int
	buf    strings.Builder
}

func (c *code) Visit(node ast.Node) ast.Visitor {
	indent := strings.Repeat(" ", c.indent)
	if node != nil {
		c.buf.WriteString(indent)
		c.buf.WriteString(fmt.Sprintf("%T", node))
		if stmt, ok := node.(ast.Stmt); ok {
			c.buf.WriteString(fmt.Sprintf(" [%s-%s)", stmt.StartPos(), stmt.EndPos()))
		}
		c.buf.WriteString("\n")
		c.indent++
	} else {
		c.indent--
	}
	return c
}

func Example_valid() {
	prog, err := parser.ParseProgram([]byte("$0 { print $1 }"), nil)
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
	prog, err := parser.ParseProgram([]byte("{ for if }"), nil)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(prog)
	}
	// Output:
	// parse error at 1:7: expected ( instead of if
}
