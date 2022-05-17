// Tests for GoAWK interpreter.
package interp_test

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

var (
	awkExe string
)

func TestMain(m *testing.M) {
	flag.StringVar(&awkExe, "awk", "gawk", "awk executable name")
	flag.Parse()
	os.Exit(m.Run())
}

type interpTest struct {
	src    string // if this includes "!awk" or "!gawk" those interpreters won't be run
	in     string
	out    string
	err    string // error from GoAWK must equal this
	awkErr string // error from awk/gawk must contain this
}

// Note: a lot of these are really parser tests too.
var interpTests = []interpTest{
	// BEGIN and END work correctly
	{`BEGIN { print "b" }`, "", "b\n", "", ""},
	{`BEGIN { print "b" }`, "foo", "b\n", "", ""},
	{`END { print "e" }`, "", "e\n", "", ""},
	{`END { print "e" }`, "foo", "e\n", "", ""},
	{`BEGIN { print "b"} END { print "e" }`, "", "b\ne\n", "", ""},
	{`BEGIN { print "b"} END { print "e" }`, "foo", "b\ne\n", "", ""},
	{`BEGIN { print "b"} $0 { print NR } END { print "e" }`, "foo", "b\n1\ne\n", "", ""},
	{`BEGIN { printf "x" }; BEGIN { printf "y" }`, "", "xy", "", ""},

	// Patterns
	{`$0`, "foo\n\nbar", "foo\nbar\n", "", ""},
	{`{ print $0 }`, "foo\n\nbar", "foo\n\nbar\n", "", ""},
	{`$1=="foo"`, "foo\n\nbar", "foo\n", "", ""},
	{`$1==42`, "foo\n42\nbar", "42\n", "", ""},
	{`$1=="42"`, "foo\n42\nbar", "42\n", "", ""},
	{`/foo/`, "foo\nx\nfood\nxfooz\nbar", "foo\nfood\nxfooz\n", "", ""},
	{`/foo/ { print NR }  /foo/`, "foo\nx\nfood\n", "1\nfoo\n3\nfood\n", "", ""},
	{`NR==2, NR==4`, "1\n2\n3\n4\n5\n6\n", "2\n3\n4\n", "", ""},
	{`
NR==2, NR==4 { print $0 }
NR==3, NR==5 { print NR }
`, "a\nb\nc\nd\ne\nf\ng", "b\nc\n3\nd\n4\n5\n", "", ""},

	// print and printf statements
	{`BEGIN { print "x", "y" }`, "", "x y\n", "", ""},
	{`BEGIN { print OFS; OFS = ","; print "x", "y" }`, "", " \nx,y\n", "", ""},
	{`BEGIN { print ORS; ORS = "."; print "x", "y" }`, "", "\n\nx y.", "", ""},
	{`BEGIN { print ORS; ORS = ""; print "x", "y" }`, "", "\n\nx y", "", ""},
	{`{ print; print }`, "foo", "foo\nfoo\n", "", ""},
	{`BEGIN { print; print }`, "", "\n\n", "", ""},
	{`BEGIN { printf "%% %d %x %c %f %s", 42, 42, 42, 42, 42 }`, "", "% 42 2a * 42.000000 42", "", ""},
	{`BEGIN { printf "%3d", 42 }`, "", " 42", "", ""},
	{`BEGIN { printf "%3s", "x" }`, "", "  x", "", ""},
	{`BEGIN { printf "%.1g", 42 }  # !windows-gawk`, "", "4e+01", "", ""}, // for some reason gawk gives "4e+001" on Windows
	{`BEGIN { printf "%d", 12, 34 }`, "", "12", "", ""},
	{`BEGIN { printf "%d" }`, "", "", "format error: got 0 args, expected 1", "not enough arg"},
	// Our %c handling is mostly like awk's, except for multiples
	// 256, where awk is weird, and we're like mawk
	{`BEGIN { printf "%c", 0 }`, "", "\x00", "", ""},
	{`BEGIN { printf "%c", 127 }`, "", "\x7f", "", ""},
	{`BEGIN { printf "%c", 128 }  # !gawk`, "", "\x80", "", ""},
	{`BEGIN { printf "%c", 255 }  # !gawk`, "", "\xff", "", ""},
	{`BEGIN { printf "%c", 256 }  # !awk !gawk`, "", "\x00", "", ""},
	{`BEGIN { printf "%c", "xyz" }`, "", "x", "", ""},
	{`BEGIN { printf "%c", "" }  # !awk`, "", "\x00", "", ""},
	{`BEGIN { printf }  # !awk - doesn't error on this`, "", "", "parse error at 1:16: expected printf args, got none", "printf: no arguments"},
	{`BEGIN { printf("%%%dd", 4) }`, "", "%4d", "", ""},

	// if and loop statements
	{`BEGIN { if (1) print "t"; }`, "", "t\n", "", ""},
	{`BEGIN { if (0) print "t"; }`, "", "", "", ""},
	{`BEGIN { if (1) print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if (0) print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if (1==1) print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if (1==2) print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if (1!=1) print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if (1!=2) print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if (1>2) print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if (2>1) print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if (1>2) print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if (2>1) print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if (1>=2) print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if (2>=1) print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if (1<2) print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if (2<1) print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if (1<=2) print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if (2<=1) print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if ("a"=="a") print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if ("a"=="b") print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if ("a"!="a") print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if ("a"!="b") print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if ("a">"b") print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if ("b">"a") print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if ("a">"b") print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if ("b">"a") print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if ("a">="b") print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if ("b">="a") print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if ("a"<"b") print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if ("b"<"a") print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { if ("a"<="b") print "t"; else print "f" }`, "", "t\n", "", ""},
	{`BEGIN { if ("b"<="a") print "t"; else print "f" }`, "", "f\n", "", ""},
	{`BEGIN { for (;;) { print "x"; break } }`, "", "x\n", "", ""},
	{`BEGIN { for (;;) { printf "%d ", i; i++; if (i>2) break; } }`, "", "0 1 2 ", "", ""},
	{`BEGIN { for (i=5; ; ) { printf "%d ", i; i++; if (i>8) break; } }`, "", "5 6 7 8 ", "", ""},
	{`BEGIN { for (i=5; ; i++) { printf "%d ", i; if (i>8) break; } }`, "", "5 6 7 8 9 ", "", ""},
	{`BEGIN { for (i=5; i<8; i++) { printf "%d ", i } }`, "", "5 6 7 ", "", ""},
	{`BEGIN { for (i=3; i>0; i--) { printf "%d ", i } }`, "", "3 2 1 ", "", ""},
	{`BEGIN { for (i=3; i>=0; i--) { printf "%d ", i } }`, "", "3 2 1 0 ", "", ""},
	{`BEGIN { for (i=0; i<10; i++) { if (i < 5) continue; printf "%d ", i } }`, "", "5 6 7 8 9 ", "", ""},
	{`BEGIN { for (i=0; i<100; i++) s+=i; print s }`, "", "4950\n", "", ""},
	{`BEGIN { a[1]=1; a[2]=1; for (k in a) { s++; break } print s }`, "", "1\n", "", ""},
	{`BEGIN { a[1]=1; a[2]=1; a[3]=1; for (k in a) { if (k==2) continue; s++ } print s }`, "", "2\n", "", ""},
	{`function alen(a, k, n) { n=0; for (k in a) n++; return n }  BEGIN { a[1]=1; a[2]=1; print alen(a) }`, "", "2\n", "", ""},
	{`BEGIN { a["x"]=1; for (SUBSEP in a) print SUBSEP, a[SUBSEP] }`, "", "x 1\n", "", ""},
	{`BEGIN { while (i<3) { i++; s++; break } print s }`, "", "1\n", "", ""},
	{`BEGIN { while (i<3) { i++; if (i==2) continue; s++ } print s }`, "", "2\n", "", ""},
	{`BEGIN { do { i++; s++; break } while (i<3); print s }`, "", "1\n", "", ""},
	{`BEGIN { do { i++; if (i==2) continue; s++ } while (i<3); print s }`, "", "2\n", "", ""},
	{`BEGIN { a["x"] = 3; a["y"] = 4; for (k in a) x += a[k]; print x }`, "", "7\n", "", ""},
	{`BEGIN { while (i < 5) { print i; i++ } }`, "", "\n1\n2\n3\n4\n", "", ""},
	{`BEGIN { do { print i; i++ } while (i < 5) }`, "", "\n1\n2\n3\n4\n", "", ""},
	{`BEGIN { for (i=0; i<10; i++); printf "x" }`, "", "x", "", ""},
	{`BEGIN { s="x"; while (s=="x") { print s; s="y" } }`, "", "x\n", "", ""},
	{`BEGIN { s="x"; while (s!="") { print s; s="" } }`, "", "x\n", "", ""},
	{`BEGIN { s="x"; while (s) { print s; s="" } }`, "", "x\n", "", ""},
	// regression tests for break and continue with nested loops
	{`
BEGIN {
	for (i = 0; i < 1; i++) {
		for (j = 0; j < 1; j++) {
			print i, j
		}
		break
	}
}
`, "", "0 0\n", "", ""},
	{`
BEGIN {
	for (i = 0; i < 1; i++) {
		for (j = 0; j < 1; j++) {
			print i, j
		}
		continue
	}
}
`, "", "0 0\n", "", ""},

	// next statement
	{`{ if (NR==2) next; print }`, "a\nb\nc", "a\nc\n", "", ""},
	{`{ if (NR==2) f(); print }  function f() { next }`, "a\nb\nc", "a\nc\n", "", ""},
	{`BEGIN { next }`, "", "", "parse error at 1:9: next can't be inside BEGIN or END", "BEGIN"},
	{`END { next }`, "", "", "parse error at 1:7: next can't be inside BEGIN or END", "END"},

	// Arrays, "in", and delete
	{`BEGIN { a["x"] = 3; print "x" in a, "y" in a }`, "", "1 0\n", "", ""},
	{`BEGIN { a["x"] = 3; a["y"] = 4; delete a["x"]; for (k in a) print k, a[k] }`, "", "y 4\n", "", ""},
	{`BEGIN { a["x"] = 3; a["y"] = 4; for (k in a) delete a[k]; for (k in a) print k, a[k] }`, "", "", "", ""},
	{`BEGIN { a["x"]; "y" in a; for (k in a) print k, a[k] }`, "", "x \n", "", ""},
	{`BEGIN { a[] }`, "", "", "parse error at 1:11: expected expression instead of ]", "syntax error"},
	{`BEGIN { delete a[] }`, "", "", "parse error at 1:18: expected expression instead of ]", "syntax error"},
	{`BEGIN { a["x"] = 3; a["y"] = 4; delete a; for (k in a) print k, a[k] }`, "", "", "", ""},
	{`function f(a) { print "x" in a, "y" in a }  BEGIN { b["x"] = 3; f(b) }`, "", "1 0\n", "", ""},

	// Unary expressions: ! + -
	{`BEGIN { print !42, !1, !0, !!42, !!1, !!0 }`, "", "0 0 1 1 1 0\n", "", ""},
	{`BEGIN { print !42, !1, !0, !!42, !!1, !!0 }`, "", "0 0 1 1 1 0\n", "", ""},
	{`BEGIN { print +4, +"3", +0, +-3, -3, - -4, -"3" }`, "", "4 3 0 -3 -3 4 -3\n", "", ""},
	{`BEGIN { $0="0"; print !$0 }`, "", "0\n", "", ""},
	{`BEGIN { $0="1"; print !$0 }`, "", "0\n", "", ""},
	{`{ print !$0 }`, "0\n", "1\n", "", ""},
	{`{ print !$0 }`, "1\n", "0\n", "", ""},
	{`!seen[$0]++`, "1\n2\n3\n2\n3\n3\n", "1\n2\n3\n", "", ""},
	{`!seen[$0]--`, "1\n2\n3\n2\n3\n3\n", "1\n2\n3\n", "", ""},

	// Comparison expressions: == != < <= > >=
	{`BEGIN { print (1==1, 1==0, "1"==1, "1"==1.0) }`, "", "1 0 1 1\n", "", ""},
	{`{ print ($0=="1", $0==1) }`, "1\n1.0\n+1", "1 1\n0 1\n0 1\n", "", ""},
	{`{ print ($1=="1", $1==1) }`, "1\n1.0\n+1", "1 1\n0 1\n0 1\n", "", ""},
	{`BEGIN { print (1!=1, 1!=0, "1"!=1, "1"!=1.0) }`, "", "0 1 0 0\n", "", ""},
	{`{ print ($0!="1", $0!=1) }`, "1\n1.0\n+1", "0 0\n1 0\n1 0\n", "", ""},
	{`{ print ($1!="1", $1!=1) }`, "1\n1.0\n+1", "0 0\n1 0\n1 0\n", "", ""},
	{`BEGIN { print (0<1, 1<1, 2<1, "12"<"2") }`, "", "1 0 0 1\n", "", ""},
	{`{ print ($1<2) }`, "1\n1.0\n+1", "1\n1\n1\n", "", ""},
	{`BEGIN { print (0<=1, 1<=1, 2<=1, "12"<="2") }`, "", "1 1 0 1\n", "", ""},
	{`{ print ($1<=2) }`, "1\n1.0\n+1", "1\n1\n1\n", "", ""},
	{`BEGIN { print (0>1, 1>1, 2>1, "12">"2") }`, "", "0 0 1 0\n", "", ""},
	{`{ print ($1>2) }`, "1\n1.0\n+1", "0\n0\n0\n", "", ""},
	{`BEGIN { print (0>=1, 1>=1, 2>=1, "12">="2") }`, "", "0 1 1 0\n", "", ""},
	{`{ print ($1>=2) }`, "1\n1.0\n+1", "0\n0\n0\n", "", ""},
	{`{ print($0<2) }`, "10", "0\n", "", ""},
	{`{ print($1<2) }`, "10", "0\n", "", ""},
	{`{ print($1<2) }`, "10x", "1\n", "", ""},
	{`BEGIN { $0="10"; print($0<2) }`, "", "1\n", "", ""},
	{`BEGIN { $1="10"; print($1<2) }`, "", "1\n", "", ""},
	{`BEGIN { $1="10x"; print($1<2) }`, "", "1\n", "", ""},

	// Short-circuit && and || operators
	{`
function t() { print "t"; return 2 }
function f() { print "f"; return 0 }
BEGIN {
	print f() && f()
	print f() && t()
	print t() && f()
	print t() && t()
}
`, "", "f\n0\nf\n0\nt\nf\n0\nt\nt\n1\n", "", ""},
	{`
function t() { print "t"; return 2 }
function f() { print "f"; return 0 }
BEGIN {
	print f() || f()
	print f() || t()
	print t() || f()
	print t() || t()
}
`, "", "f\nf\n0\nf\nt\n1\nt\n1\nt\n1\n", "", ""},
	{`BEGIN { print 0&&0, 0&&2, 2&&0, 2&&2 }`, "", "0 0 0 1\n", "", ""},
	{`BEGIN { print 0||0, 0||2, 2||0, 2||2 }`, "", "0 1 1 1\n", "", ""},

	// Other binary expressions: + - * ^ ** / % CONCAT ~ !~
	{`BEGIN { print 1+2, 1+2+3, 1+-2, -1+2, "1"+"2", 3+.14 }`, "", "3 6 -1 1 3 3.14\n", "", ""},
	{`BEGIN { print 1-2, 1-2-3, 1-+2, -1-2, "1"-"2", 3-.14 }`, "", "-1 -4 -1 -3 -1 2.86\n", "", ""},
	{`BEGIN { print 2*3, 2*3*4, 2*-3, -2*3, "2"*"3", 3*.14 }`, "", "6 24 -6 -6 6 0.42\n", "", ""},
	{`BEGIN { print 2/3, 2/3/4, 2/-3, -2/3, "2"/"3", 3/.14 }`, "", "0.666667 0.166667 -0.666667 -0.666667 0.666667 21.4286\n", "", ""},
	{`BEGIN { print 2%3, 2%3%4, 2%-3, -2%3, "2"%"3", 3%.14 }`, "", "2 2 2 -2 2 0.06\n", "", ""},
	{`BEGIN { print 2^3, 2^3^3, 2^-3, -2^3, "2"^"3", 3^.14 }`, "", "8 134217728 0.125 -8 8 1.16626\n", "", ""},
	{`BEGIN { print 2**3, 2**3**3, 2**-3, -2**3, "2"**"3", 3**.14 }`, "", "8 134217728 0.125 -8 8 1.16626\n", "", ""},
	{`BEGIN { print 1 2, "x" "yz", 1+2 3+4 }`, "", "12 xyz 37\n", "", ""},
	{`BEGIN { print "food"~/oo/, "food"~/[oO]+d/, "food"~"f", "food"~"F", "food"~0 }`, "", "1 1 1 0 0\n", "", ""},
	{`BEGIN { print "food"!~/oo/, "food"!~/[oO]+d/, "food"!~"f", "food"!~"F", "food"!~0 }`, "", "0 0 0 1 1\n", "", ""},
	{`BEGIN { print 1+2*3/4^5%6 7, (1+2)*3/4^5%6 "7" }`, "", "1.005867 0.008789067\n", "", ""},
	{`BEGIN { print 1/0 }`, "", "", "division by zero", "division by zero"},
	{`BEGIN { print 1%0 }`, "", "", "division by zero in mod", "division by zero"},
	{`BEGIN { x /= 0 }`, "", "", "division by zero", "division by zero"},
	{`BEGIN { x %= 0 }`, "", "", "division by zero in mod", "division by zero"},

	// Number, string, and regex expressions
	{`BEGIN { print 1, 1., .1, 1e0, -1, 1e }`, "", "1 1 0.1 1 -1 1\n", "", ""},
	{`BEGIN { print '\"' '\'' 'xy' "z" "'" '\"' }`, "", "\"'xyz'\"\n", "", "syntax error"}, // Check support for single-quoted strings
	{`BEGIN { print "0\n1\t2\r3\a4\b5\f6\v7\x408\xf" }`, "", "0\n1\t2\r3\a4\b5\f6\v7@8\x0f\n", "", ""},
	{`{ print /foo/ }`, "food\nfoo\nxfooz\nbar\n", "1\n1\n1\n0\n", "", ""},
	{`/[a-/`, "foo", "", "parse error at 1:1: error parsing regexp: missing closing ]: `[a-`", "terminated"},
	{`/=foo/`, "=foo", "=foo\n", "", ""},
	{`BEGIN { print "-12"+0, "+12"+0, " \t\r\n7foo"+0, ".5"+0, "5."+0, "+."+0 }`, "", "-12 12 7 0.5 5 0\n", "", ""},
	{`BEGIN { print "1e3"+0, "1.2e-1"+0, "1e+1"+0, "1e"+0, "1e+"+0 }`, "", "1000 0.12 10 1 1\n", "", ""},
	{`BEGIN { print -(11102200000000000000000000000000000000 1040000) }  # !gawk - gawk supports big numbers`,
		"", "-inf\n", "", ""},
	{`BEGIN { print atan2(0, 8020020000000e20G-0)}`, "", "0\n", "", ""},
	{`BEGIN { print 1e1000, -1e1000 }  # !gawk`, "", "inf -inf\n", "", ""},
	{`BEGIN { printf "\x0.\x00.\x0A\x10\xff\xFF\x41" }  # !awk`, "", "\x00.\x00.\n\x10\xff\xffA", "", ""},
	{`BEGIN { printf "\x1.\x01.\x0A\x10\xff\xFF\x41" }`, "", "\x01.\x01.\n\x10\xff\xffA", "", ""},
	{`BEGIN { printf "\0\78\7\77\777\0 \141 " }  # !awk`, "", "\x00\a8\a?\xff\x00 a ", "", ""},
	{`BEGIN { printf "\1\78\7\77\777\1 \141 " }`, "", "\x01\a8\a?\xff\x01 a ", "", ""},

	// Unusual number/exponent handling
	{`BEGIN { e="x"; E="X"; print 1e, 1E }`, "", "1x 1X\n", "", ""},
	{`BEGIN { e="x"; E="X"; print 1e1e, 1E1E }`, "", "10x 10X\n", "", ""},
	{`BEGIN { a=2; print 1e+a, 1E+a, 1e+1, 1E+1 }`, "", "12 12 10 10\n", "", ""},
	{`BEGIN { a=2; print 1e-a, 1E-a, 1e-1, 1E-1 }`, "", "1-2 1-2 0.1 0.1\n", "", ""},
	{`BEGIN { print 1e+ }`, "", "", "parse error at 1:19: expected expression instead of }", "syntax error"},
	{`BEGIN { print 1e- }`, "", "", "parse error at 1:19: expected expression instead of }", "syntax error"},

	// Conditional ?: expression
	{`{ print /x/?"t":"f" }`, "x\ny\nxx\nz\n", "t\nf\nt\nf\n", "", ""},
	{`BEGIN { print 1?2?3:4:5, 1?0?3:4:5, 0?2?3:4:5 }`, "", "3 4 5\n", "", ""},
	{`BEGIN { $0="0"; print ($0?1:0) }`, "", "1\n", "", ""},
	{`{ print $0?1:0 }`, "0\n", "0\n", "", ""},
	{`{ print $0?1:0 }`, "1\n", "1\n", "", ""},
	{`BEGIN { $0="1"; print ($0?1:0) }`, "", "1\n", "", ""},
	{`BEGIN { print 0?1:0, 1?1:0, ""?1:0, "0"?1:0, "1"?1:0, x?1:0 }`, "", "0 1 0 1 1 0\n", "", ""},

	// Built-in variables
	{`BEGIN { print ARGC; ARGC=42; print ARGC }  # !gawk`, "", "1\n42\n", "", ""}, // ARGC is properly tested in goawk_test.go
	{`
BEGIN {
	print CONVFMT, 1.2345678 ""
	CONVFMT = "%.3g"
	print CONVFMT, 1.234567 ""
}`, "", "%.6g 1.23457\n%.3g 1.23\n", "", ""},
	{`BEGIN { FILENAME = "foo"; print FILENAME }`, "", "foo\n", "", ""},
	{`BEGIN { FILENAME = "123.0"; print (FILENAME==123) }`, "", "0\n", "", ""},
	// Other FILENAME behaviour is tested in goawk_test.go
	{`BEGIN { FNR = 123; print FNR }`, "", "123\n", "", ""},
	{`{ print FNR, $0 }`, "a\nb\nc", "1 a\n2 b\n3 c\n", "", ""},
	// Other FNR behaviour is tested in goawk_test.go
	{`BEGIN { print "|" FS "|"; FS="," } { print $1, $2 }`, "a b\na,b\nx,,y", "| |\na b \na b\nx \n", "", ""},
	{`BEGIN { print "|" FS "|"; FS="\\." } { print $1, $2 }`, "a b\na.b\nx..y", "| |\na b \na b\nx \n", "", ""},
	// ASCII unit and record separator
	{`BEGIN { FS="\x1f"; RS="\x1e"; OFS="," } { print $1, $2, $3 }`,
		"id\x1fname\x1fage\x1e1\x1fBob \"Billy\" Smith\x1f42\x1e2\x1fJane\nBrown\x1f37",
		"id,name,age\n1,Bob \"Billy\" Smith,42\n2,Jane\nBrown,37\n", "", ""},
	// Unicode unit and record separator (skip on Windows under gawk due to Unicode command line issues)
	{`BEGIN { FS="␟"; RS="␞"; OFS="," } { print $1, $2, $3 }  # !windows-gawk`,
		"id␟name␟age␞1␟Bob \"Billy\" Smith␟42␞2␟Jane\nBrown␟37",
		"id,name,age\n1,Bob \"Billy\" Smith,42\n2,Jane\nBrown,37\n", "", ""},
	{`BEGIN { FS="\\" } { print $1, $2 }`, "a\\b", "a b\n", "", ""},
	{`{ print NF }`, "\na\nc d\ne f g", "0\n1\n2\n3\n", "", ""},
	{`BEGIN { NR = 123; print NR }`, "", "123\n", "", ""},
	{`{ print NR, $0 }`, "a\nb\nc", "1 a\n2 b\n3 c\n", "", ""},
	{`
BEGIN {
	print OFMT, 1.2345678
	OFMT = "%.3g"
	print OFMT, 1.234567
}`, "", "%.6g 1.23457\n%.3g 1.23\n", "", ""},
	// OFS and ORS are tested above
	{`BEGIN { print RSTART, RLENGTH; RSTART=5; RLENGTH=42; print RSTART, RLENGTH; } `, "",
		"0 0\n5 42\n", "", ""},
	{`BEGIN { print RS }`, "", "\n\n", "", ""},
	{`BEGIN { print RS; RS="|"; print RS }  { print }`, "a b|c d|", "\n\n|\na b\nc d\n", "", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		"a\n\nb\nc",
		"1 (1):\na\n2 (2):\nb\nc\n", "", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		"1\n2\n\na\nb",
		"1 (2):\n1\n2\n2 (2):\na\nb\n", "", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		"a b\nc d\n\ne f\n\n\n  \n\n\ng h\n\n\n",
		"1 (2):\na b\nc d\n2 (1):\ne f\n3 (1):\n  \n4 (1):\ng h\n", "", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		"\n\na b\n\nc d\n",
		"1 (1):\na b\n2 (1):\nc d\n", "", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }  # !awk !gawk - they don't handle CR LF with RS==""`,
		"\r\n\r\na b\r\n\r\nc d\r\n",
		"1 (1):\na b\n2 (1):\nc d\n", "", ""},
	{`BEGIN { RS=""; FS="X" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) printf "%s|", $i }`,
		"aXb\ncXd\n\neXf\n\n\n  \n\n\ngXh\n\n\n",
		"1 (4):\na|b|c|d|2 (2):\ne|f|3 (1):\n  |4 (2):\ng|h|", "", ""},
	{`BEGIN { RS = "" }  { print "got", $0 }`,
		"\n\n\n\n", "", "", ""},
	{`BEGIN { RS="\n" }  { print }`, "a\n\nb\nc", "a\n\nb\nc\n", "", ""},
	{`BEGIN { RS="ö" }  { print }  # !windows-gawk`, "1ötwoöthree", "1\ntwo\nthree\n", "", ""},
	{`BEGIN { RS="\\.+" }  { print }`, "1.two..three...4.", "1\ntwo\nthree\n4\n", "", ""},
	{`BEGIN { RS = "\n|( *[[:upper:]]+ *)" } { print "Record =", $0,"and RT = [" RT "]" }`, // from https://www.gnu.org/software/gawk/manual/html_node/gawk-split-records.html
		"record 1 AAAA record 2 BBBB record 3\n",
		`Record = record 1 and RT = [ AAAA ]
Record = record 2 and RT = [ BBBB ]
Record = record 3 and RT = [
]
`, "", ""},
	{`BEGIN { RS = "\n|( *[[:upper:]]+ *)" } { print "Record =", $0,"and RT = [" RT "]" }`,
		"record 1 AAAA record 2 BBBB record 3",
		`Record = record 1 and RT = [ AAAA ]
Record = record 2 and RT = [ BBBB ]
Record = record 3 and RT = []
`, "", ""},
	{`BEGIN { RS=".." } { print $0 RT }`, "foo bar bazz", "fo\no \nba\nr \nba\nzz\n", "", ""},
	{`BEGIN { RT="foo"; print RT }`, "", "foo\n", "", ""},
	{`
BEGIN {
	print SUBSEP
	a[1, 2] = "onetwo"
	print a[1, 2]
	for (k in a) {
		print k, a[k]
	}
	delete a[1, 2]
	SUBSEP = "|"
	print SUBSEP
	a[1, 2] = "onetwo"
	print a[1, 2]
	for (k in a) {
		print k, a[k]
	}
}`, "", "\x1c\nonetwo\n1\x1c2 onetwo\n|\nonetwo\n1|2 onetwo\n", "", ""},

	// Field expressions and assignment (and interaction with NF)
	{`{ print NF; NF=1; $2="two"; print $0, NF }`, "\n", "0\n two 2\n", "", ""},
	{`{ print NF; NF=2; $2="two"; print $0, NF}`, "\n", "0\n two 2\n", "", ""},
	{`{ print NF; NF=3; $2="two"; print $0, NF}`, "a b c\n", "3\na two c 3\n", "", ""},
	{`{ print; print $1, $3, $NF }`, "a b c d e", "a b c d e\na c e\n", "", ""},
	{`{ print $1,$3; $2="x"; print; print $2 }`, "a b c", "a c\na x c\nx\n", "", ""},
	{`{ print; $0="x y z"; print; print $1, $3 }`, "a b c", "a b c\nx y z\nx z\n", "", ""},
	{`{ print $1^2 }`, "10", "100\n", "", ""},
	{`{ print $-1 }`, "a\nb c\nd e f\n", "a\nc\nf\n", "", "field -1"},
	{`{ print $-2 }`, "a\nb c\nd e f\n", "\nb\ne\n", "", "field -2"},
	{`{ print $-3 }`, "a\nb c\nd e f\n", "\n\nd\n", "", "field -3"},
	{`{ $-1="x"; print }`, "a\nb c\nd e f\n", "x\nb x\nd e x\n", "", "field -1"},
	{`{ $-2="y"; print }`, "a\nb c\nd e f\n", "a\ny c\nd y f\n", "", "field -2"},
	{`{ $-3="z"; print }`, "a\nb c\nd e f\n", "a\nb c\nz e f\n", "", "field -3"},
	{`{ NF=-1; }  # !awk - awk allows setting negative NF`,
		"x", "", "NF set to negative value: -1", "negative value"},
	{`{ NF=1234567; }`, "x", "", "NF set too large: 1234567", ""},
	{`BEGIN { $1234567=1 }`, "", "", "field index too large: 1234567", ""},
	{`0 in FS  # !awk - doesn't flag this as an error`, "x", "",
		`parse error at 1:6: can't use scalar "FS" as array`, "array"},
	// TODO: I think this is happening because we parse this as ($($0))++ rather than ($($0++))
	// {`{ $$0++; print $0 }`, "2 3 4", "3\n", "", ""},
	// {`BEGIN { $0="3 4 5 6 7 8 9"; a=3; print $$a++++; print }`, "", "7\n3 4 6 6 8 8 9\n", "", ""},

	// Lots of NF tests with different combinations of NF, $, and number
	// of input fields. Some of these cause segmentation faults on awk
	// (but work fine on gawk and mawk).
	{`{ NF=1; $1="x"; print $0; print NF }`, "a", "x\n1\n", "", ""},
	{`{ NF=1; $1="x"; print $0; print NF }`, "a b", "x\n1\n", "", ""},
	{`{ NF=1; $1="x"; print $0; print NF }`, "a b c", "x\n1\n", "", ""},
	{`{ NF=1; $2="x"; print $0; print NF }`, "a", "a x\n2\n", "", ""},
	{`{ NF=1; $2="x"; print $0; print NF }`, "a b", "a x\n2\n", "", ""},
	{`{ NF=1; $2="x"; print $0; print NF }`, "a b c", "a x\n2\n", "", ""},
	{`{ NF=1; $3="x"; print $0; print NF }`, "a", "a  x\n3\n", "", ""},
	{`{ NF=1; $3="x"; print $0; print NF }  # !awk - awk differs from gawk (but gawk seems right)`,
		"a b", "a  x\n3\n", "", ""},
	{`{ NF=1; $3="x"; print $0; print NF }  # !awk - awk differs from gawk (but gawk seems right)`,
		"a b c", "a  x\n3\n", "", ""},
	{`{ NF=2; $1="x"; print $0; print NF }`, "a", "x \n2\n", "", ""},
	{`{ NF=2; $1="x"; print $0; print NF }`, "a b", "x b\n2\n", "", ""},
	{`{ NF=2; $1="x"; print $0; print NF }`, "a b c", "x b\n2\n", "", ""},
	{`{ NF=2; $2="x"; print $0; print NF }`, "a", "a x\n2\n", "", ""},
	{`{ NF=2; $2="x"; print $0; print NF }`, "a b", "a x\n2\n", "", ""},
	{`{ NF=2; $2="x"; print $0; print NF }`, "a b c", "a x\n2\n", "", ""},
	{`{ NF=2; $3="x"; print $0; print NF }`, "a", "a  x\n3\n", "", ""},
	{`{ NF=2; $3="x"; print $0; print NF }`, "a b", "a b x\n3\n", "", ""},
	{`{ NF=2; $3="x"; print $0; print NF }`, "a b c", "a b x\n3\n", "", ""},
	{`{ NF=3; $1="x"; print $0; print NF }  # !awk - segmentation fault`,
		"a", "x  \n3\n", "", ""},
	{`{ NF=3; $1="x"; print $0; print NF }  # !awk - segmentation fault`,
		"a b", "x b \n3\n", "", ""},
	{`{ NF=3; $1="x"; print $0; print NF }`, "a b c", "x b c\n3\n", "", ""},
	{`{ NF=3; $2="x"; print $0; print NF }  # !awk - segmentation fault`,
		"a", "a x \n3\n", "", ""},
	{`{ NF=3; $2="x"; print $0; print NF }  # !awk - segmentation fault`,
		"a b", "a x \n3\n", "", ""},
	{`{ NF=3; $2="x"; print $0; print NF }`, "a b c", "a x c\n3\n", "", ""},
	{`{ NF=3; $3="x"; print $0; print NF }`, "a", "a  x\n3\n", "", ""},
	{`{ NF=3; $3="x"; print $0; print NF }`, "a b", "a b x\n3\n", "", ""},
	{`{ NF=3; $3="x"; print $0; print NF }`, "a b c", "a b x\n3\n", "", ""},

	// Assignment expressions and vars
	{`BEGIN { print x; x = 4; print x; }`, "", "\n4\n", "", ""},
	{`BEGIN { a["foo"]=1; b[2]="x"; k="foo"; print a[k], b["2"] }`, "", "1 x\n", "", ""},
	{`BEGIN { s+=5; print s; s-=2; print s; s-=s; print s }`, "", "5\n3\n0\n", "", ""},
	{`BEGIN { x=2; x*=x; print x; x*=3; print x }`, "", "4\n12\n", "", ""},
	{`BEGIN { x=6; x/=3; print x; x/=x; print x; x/=.6; print x }`, "", "2\n1\n1.66667\n", "", ""},
	{`BEGIN { x=12; x%=5; print x }`, "", "2\n", "", ""},
	{`BEGIN { x=2; x^=5; print x; x^=0.5; print x }`, "", "32\n5.65685\n", "", ""},
	{`BEGIN { x=2; x**=5; print x; x**=0.5; print x }`, "", "32\n5.65685\n", "", ""},
	{`{ $2+=10; print; $3/=2; print }`, "1 2 3", "1 12 3\n1 12 1.5\n", "", ""},
	{`BEGIN { a[2] += 1; a["2"] *= 3; print a[2] }`, "", "3\n", "", ""},
	{`function inc(x, n) { x += n; return x }  BEGIN { print inc(3, 2) }`, "", "5\n", "", ""},
	{`function inca(a, k, n) { a[k] += n }  BEGIN { b["x"]=7; inca(b, "x", 2); print b["x"] }`, "", "9\n", "", ""},
	{`BEGIN { NF += 3; print NF }`, "", "3\n", "", ""},
	{`BEGIN { x=1; x += x+=3; print x }`, "", "8\n", "", ""},

	// Incr/decr expressions
	{`BEGIN { print x++; print x }`, "", "0\n1\n", "", ""},
	{`BEGIN { print x; print x++; print ++x; print x }`, "", "\n0\n2\n2\n", "", ""},
	{`BEGIN { print x; print x--; print --x; print x }`, "", "\n0\n-2\n-2\n", "", ""},
	{`BEGIN { s++; s++; print s }`, "", "2\n", "", ""},
	{`BEGIN { y=" "; --x[y = y y]; print length(y) }`, "", "2\n", "", ""},
	{`BEGIN { x[y++]++; print y }`, "", "1\n", "", ""},
	{`BEGIN { x[y++] += 3; print y }`, "", "1\n", "", ""},
	{`BEGIN { $(y++)++; print y }`, "", "1\n", "", ""},
	{`BEGIN { print "s" ++n; print "s" --n }`, "", "s1\ns0\n", "", ""},
	{`function inc(x) { x++; return x }  BEGIN { print inc(3) }`, "", "4\n", "", ""},
	{`function inca(a, k) { a[k]++ }  BEGIN { b["x"]=7; inca(b, "x"); print b["x"] }`, "", "8\n", "", ""},
	{`BEGIN { NF++; print NF }`, "", "1\n", "", ""},

	// Builtin functions
	{`BEGIN { print sin(0), sin(0.5), sin(1), sin(-1) }`, "", "0 0.479426 0.841471 -0.841471\n", "", ""},
	{`BEGIN { print cos(0), cos(0.5), cos(1), cos(-1) }`, "", "1 0.877583 0.540302 0.540302\n", "", ""},
	{`BEGIN { print exp(0), exp(0.5), exp(1), exp(-1) }`, "", "1 1.64872 2.71828 0.367879\n", "", ""},
	{`BEGIN { print log(0), log(0.5), log(1) }`, "", "-inf -0.693147 0\n", "", ""},
	{`BEGIN { print log(-1) }  # !gawk - gawk prints warning for this as well`,
		"", "nan\n", "", ""},
	{`BEGIN { print sqrt(0), sqrt(2), sqrt(4) }`, "", "0 1.41421 2\n", "", ""},
	{`BEGIN { print int(3.5), int("1.9"), int(4), int(-3.6), int("x"), int("") }`, "", "3 1 4 -3 0 0\n", "", ""},
	{`BEGIN { print match("food", "foo"), RSTART, RLENGTH }`, "", "1 1 3\n", "", ""},
	{`BEGIN { print match("x food y", "fo"), RSTART, RLENGTH }`, "", "3 3 2\n", "", ""},
	{`BEGIN { print match("x food y", "fox"), RSTART, RLENGTH }`, "", "0 0 -1\n", "", ""},
	{`BEGIN { print match("x food y", /[fod]+/), RSTART, RLENGTH }`, "", "3 3 4\n", "", ""},
	{`{ print length, length(), length("buzz"), length("") }`, "foo bar", "7 7 4 0\n", "", ""},
	{`BEGIN { print index("foo", "f"), index("foo0", 0), index("foo", "o"), index("foo", "x") }`, "", "1 4 2 0\n", "", ""},
	{`BEGIN { print atan2(1, 0.5), atan2(-1, 0) }`, "", "1.10715 -1.5708\n", "", ""},
	{`BEGIN { print sprintf("%3d", 42) }`, "", " 42\n", "", ""},
	{`BEGIN { print sprintf("%d", 12, 34) }`, "", "12\n", "", ""},
	{`BEGIN { print sprintf("%d") }`, "", "", "format error: got 0 args, expected 1", "not enough arg"},
	{`BEGIN { print sprintf("%d", 12, 34) }`, "", "12\n", "", ""},
	{`BEGIN { print sprintf("% 5d", 42) }`, "", "   42\n", "", ""},
	{`BEGIN { print sprintf("%*s %.*s", 5, "abc", 5, "abcdefghi") }`, "", "  abc abcde\n", "", ""},
	{`BEGIN { print substr("food", 1) }`, "", "food\n", "", ""},
	{`BEGIN { print substr("food", 1, 2) }`, "", "fo\n", "", ""},
	{`BEGIN { print substr("food", 1, 4) }`, "", "food\n", "", ""},
	{`BEGIN { print substr("food", 1, 8) }`, "", "food\n", "", ""},
	{`BEGIN { print substr("food", 2) }`, "", "ood\n", "", ""},
	{`BEGIN { print substr("food", 2, 2) }`, "", "oo\n", "", ""},
	{`BEGIN { print substr("food", 2, 3) }`, "", "ood\n", "", ""},
	{`BEGIN { print substr("food", 2, 8) }`, "", "ood\n", "", ""},
	{`BEGIN { print substr("food", 0, 8) }`, "", "food\n", "", ""},
	{`BEGIN { print substr("food", -1, 8) }`, "", "food\n", "", ""},
	{`BEGIN { print substr("food", 5) }`, "", "\n", "", ""},
	{`BEGIN { print substr("food", -1) }`, "", "food\n", "", ""},
	{`BEGIN { print substr("food", 5, 8) }`, "", "\n", "", ""},
	{`BEGIN { print substr("food", 2, -3), substr("fööd", 2, -3) }`, "", " \n", "", ""},
	{`BEGIN { n = split("", a); for (i=1; i<=n; i++) print a[i] }`, "", "", "", ""},
	{`BEGIN { n = split("", a, "."); for (i=1; i<=n; i++) print a[i] }`, "", "", "", ""},
	{`BEGIN { n = split("ab c d ", a); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n", "", ""},
	{`BEGIN { n = split("ab,c,d,", a, ","); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n\n", "", ""},
	{`BEGIN { n = split("ab,c.d,", a, /[,.]/); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n\n", "", ""},
	{`BEGIN { n = split("1 2", a); print (n, a[1], a[2], a[1]==1, a[2]==2) }`, "", "2 1 2 1 1\n", "", ""},
	{`BEGIN { x = "1.2.3"; print sub(/\./, ",", x); print x }`, "", "1\n1,2.3\n", "", ""},
	{`BEGIN { x = "1.2.3"; print sub(/\./, ",\\", x); print x }`, "", "1\n1,\\2.3\n", "", ""},
	{`{ print sub(/\./, ","); print $0 }`, "1.2.3", "1\n1,2.3\n", "", ""},
	{`BEGIN { x = "1.2.3"; print gsub(/\./, ",", x); print x }`, "", "2\n1,2,3\n", "", ""},
	{`{ print gsub(/\./, ","); print $0 }`, "1.2.3", "2\n1,2,3\n", "", ""},
	{`{ print gsub(/[0-9]/, "(&)"); print $0 }`, "0123x. 42y", "6\n(0)(1)(2)(3)x. (4)(2)y\n", "", ""},
	{`{ print gsub(/[0-9]+/, "(&)"); print $0 }`, "0123x. 42y", "2\n(0123)x. (42)y\n", "", ""},
	{`{ print gsub(/[0-9]/, "\\&"); print $0 }`, "0123x. 42y", "6\n&&&&x. &&y\n", "", ""},
	{`{ print gsub(/[0-9]/, "\\z"); print $0 }`, "0123x. 42y", "6\n\\z\\z\\z\\zx. \\z\\zy\n", "", ""},
	{`{ print gsub("0", "x\\\\y"); print $0 }  # !awk !gawk -- our behaviour is per POSIX spec (gawk -P and mawk)`,
		"0", "1\nx\\y\n", "", ""},
	{`sub("", "\\e", FS)  # !awk !gawk`, "foo bar\nbaz buz\n", "",
		"invalid regex \"\\\\e \": error parsing regexp: invalid escape sequence: `\\e`", ""},
	{`BEGIN { print tolower("Foo BaR") }`, "", "foo bar\n", "", ""},
	{`BEGIN { print toupper("Foo BaR") }`, "", "FOO BAR\n", "", ""},
	{`
BEGIN {
    srand()
	srand(1)
	a = rand(); b = rand(); c = rand()
	srand(1)
	x = rand(); y = rand(); z = rand()
	print (a==b, b==c, x==y, y==z)
	print (a==x, b==y, c==z)
}
`, "", "0 0 0 0\n1 1 1\n", "", ""},
	{`
BEGIN {
	for (i = 0; i < 1000; i++) {
		if (rand() < 0.5) n++
	}
	print (n>400)
}
`, "", "1\n", "", ""},
	{`BEGIN { print system("echo foo"); print system("echo bar") }  # !fuzz`,
		"", "foo\n0\nbar\n0\n", "", ""},
	{`BEGIN { print system(">&2 echo error") }  # !fuzz`,
		"", "error\n0\n", "", ""},
	{`BEGIN { print system("exit 42") }  # !fuzz`, "", "42\n", "", ""},
	{`BEGIN { system("cat") }`, "foo\nbar", "foo\nbar", "", ""},

	// Test bytes/unicode handling (GoAWK currently has char==byte, unlike Gawk).
	{`BEGIN { print match("food", "foo"), RSTART, RLENGTH }  !gawk`, "", "1 1 3\n", "", ""},
	{`BEGIN { print match("x food y", "fo"), RSTART, RLENGTH }  !gawk`, "", "3 3 2\n", "", ""},
	{`BEGIN { print match("x food y", "fox"), RSTART, RLENGTH }  !gawk`, "", "0 0 -1\n", "", ""},
	{`BEGIN { print match("x food y", /[fod]+/), RSTART, RLENGTH }  !gawk`, "", "3 3 4\n", "", ""},
	{`BEGIN { print match("絵 fööd y", /[föd]+/), RSTART, RLENGTH }  !gawk`, "", "5 5 6\n", "", ""},
	{`{ print length, length(), length("buzz"), length("") }  # !gawk`, "foo bar", "7 7 4 0\n", "", ""},
	{`BEGIN { print length("a"), length("絵") }  # !gawk`, "", "1 3\n", "", ""},
	{`BEGIN { print index("foo", "f"), index("foo0", 0), index("foo", "o"), index("foo", "x") }  # !gawk`, "", "1 4 2 0\n", "", ""},
	{`BEGIN { print index("föö", "f"), index("föö0", 0), index("föö", "ö"), index("föö", "x") }  # !gawk`, "", "1 6 2 0\n", "", ""},
	{`BEGIN { print substr("food", 1), substr("fööd", 1) }  # !gawk`, "", "food fööd\n", "", ""},
	{`BEGIN { print substr("food", 1, 2), substr("fööd", 1, 2) }  # !gawk`, "", "fo f\xc3\n", "", ""},
	{`BEGIN { print substr("food", 1, 4), substr("fööd", 1, 4) }  # !gawk`, "", "food fö\xc3\n", "", ""},
	{`BEGIN { print substr("food", 1, 8), substr("fööd", 1, 8) }  # !gawk`, "", "food fööd\n", "", ""},
	{`BEGIN { print substr("food", 2), substr("fööd", 2) }  # !gawk`, "", "ood ööd\n", "", ""},
	{`BEGIN { print substr("food", 2, 2), substr("fööd", 2, 2) }  # !gawk`, "", "oo ö\n", "", ""},
	{`BEGIN { print substr("food", 2, 3), substr("fööd", 2, 3) }  # !gawk`, "", "ood ö\xc3\n", "", ""},
	{`BEGIN { print substr("food", 2, 8), substr("fööd", 2, 8) }   # !gawk`, "", "ood ööd\n", "", ""},
	{`BEGIN { print substr("food", 0, 8), substr("fööd", 0, 8) }  # !gawk`, "", "food fööd\n", "", ""},
	{`BEGIN { print substr("food", -1, 8), substr("fööd", -1, 8) }  # !gawk`, "", "food fööd\n", "", ""},
	{`BEGIN { print substr("food", 5, 8), substr("fööd", 5, 8) }   # !gawk`, "", " \xb6d\n", "", ""},
	{`BEGIN { print substr("food", 2, -3), substr("fööd", 2, -3) }  # !gawk`, "", " \n", "", ""},

	// Conditional expressions parse and work correctly
	{`BEGIN { print 0?"t":"f" }`, "", "f\n", "", ""},
	{`BEGIN { print 1?"t":"f" }`, "", "t\n", "", ""},
	{`BEGIN { print (1+2)?"t":"f" }`, "", "t\n", "", ""},
	{`BEGIN { print (1+2?"t":"f") }`, "", "t\n", "", ""},
	{`BEGIN { print(1 ? x="t" : "f"); print x; }`, "", "t\nt\n", "", ""},

	// Locals vs globals, array params, and recursion
	{`
function f(loc) {
	glob += 1
	loc += 1
	loc = loc * 2
	print glob, loc
}
BEGIN {
	glob = 1
	loc = 42
	f(3)
	print loc
	f(4)
	print loc
}
`, "", "2 8\n42\n3 10\n42\n", "", ""},
	{`
function set(a, x, v) { a[x] = v }
function get(a, x) { return a[x] }
function get2(x, a) { return a[x] }
function get3(x, a, b) { b[0]; return a[x] }
BEGIN {
	a["x"] = 1
	set(b, "y", 2)
	for (k in a) print k, a[k]
	print "---"
	for (k in b) print k, b[k]
	print "---"
	print get(a, "x"), get(b, "y")
	print get2("x", a), get2("y", b)
	print get3("x", a), get2("y", b)
}
`, "", "x 1\n---\ny 2\n---\n1 2\n1 2\n1 2\n", "", ""},
	{`
function fib(n) {
	return n < 3 ? 1 : fib(n-2) + fib(n-1)
}
BEGIN {
	for (i = 1; i <= 7; i++) {
		printf "%d ", fib(i)
	}
}
`, "", "1 1 2 3 5 8 13 ", "", ""},
	{`
function f(a, x) { return a[x] }
function g(b, y) { f(b, y) }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1) }
`, "", "2\n\n", "", ""},
	{`
function g(b, y) { return f(b, y) }
function f(a, x) { return a[x] }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1) }
`, "", "2\n2\n", "", ""},
	{`
function h(b, y) { g(b, y) }
function g(b, y) { f(b, y) }
function f(a, x) { return a[x] }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1) }
`, "", "2\n\n", "", ""},
	{`
function h(b, y) { return g(b, y) }
function g(b, y) { return f(b, y) }
function f(a, x) { return a[x] }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1); print h(c, 1) }
`, "", "2\n2\n2\n", "", ""},
	{`
function get(a, x) { return a[x] }
BEGIN { a[1]=2; print get(a, x); print get(1, 2); }
# !awk - awk doesn't detect this
`, "", "", `parse error at 3:40: can't pass scalar 1 as array param`, "attempt to use scalar"},
	{`
function early() {
	print "x"
	return
	print "y"
}
BEGIN { early() }
`, "", "x\n", "", ""},
	{`BEGIN { return }`, "", "", "parse error at 1:9: return must be inside a function", "return"},
	{`function f() { printf "x" }; BEGIN { f() } `, "", "x", "", ""},
	{`BEGIN { arr[0]; f(arr) } function f(a) { printf "x" }`, "", "x", "", ""},
	{`function f(x) { 0 in _; f(_) }  BEGIN { f() }  # !awk !gawk`, "", "", `calling "f" exceeded maximum call depth of 1000`, ""},
	{`BEGIN { for (i=0; i<1001; i++) f(); print x }  function f() { x++ }`, "", "1001\n", "", ""},
	{`
function bar(y) { return y[1] }
function foo() { return bar(x) }
BEGIN { x[1] = 42; print foo() }
`, "", "42\n", "", ""},
	{`
function f1(x) { }
function f2(x, y) { return x[y] }
BEGIN { a[1]=2; f1(a); print f2(a, 1) }
`, "", "2\n", "", ""},
	{`BEGIN { arr[0]; f(arr) } function f(a) { print "x" }`, "", "x\n", "", ""},
	{`function add(a, b) { return a+b }  BEGIN { print add(1, 2), add(1), add() }`, "", "3 1 0\n", "", ""},

	// Type checking / resolver tests
	{`BEGIN { a[x]; a=42 }`, "", "", `parse error at 1:15: can't use array "a" as scalar`, "array"},
	{`BEGIN { s=42; s[x] }`, "", "", `parse error at 1:15: can't use scalar "s" as array`, "array"},
	{`function get(a, k) { return a[k] }  BEGIN { a = 42; print get(a, 1); }  # !awk - doesn't error in awk`,
		"", "", `parse error at 1:59: can't pass scalar "a" as array param`, "attempt to use scalar parameter `a' as an array"},
	{`function get(a, k) { return a+k } BEGIN { a[42]; print get(a, 1); }`,
		"", "", `parse error at 1:56: can't pass array "a" as scalar param`, "array"},
	{`{ f(z) }  function f(x) { print NR }`, "abc", "1\n", "", ""},
	{`function f() { f() }  BEGIN { f() }  # !awk !gawk`, "", "", `calling "f" exceeded maximum call depth of 1000`, ""},
	{`function f(x) { 0 in x }  BEGIN { f(FS) }  # !awk`, "", "", `parse error at 1:35: can't pass scalar "FS" as array param`, "attempt to use scalar parameter `x' as an array"},
	{`
function foo(x) { print "foo", x }
function bar(foo) { print "bar", foo }
BEGIN { foo(5); bar(10) }
`, "", "foo 5\nbar 10\n", "", ""},
	{`
function foo(foo) { print "foo", foo }
function bar(foo) { print "bar", foo }
BEGIN { foo(5); bar(10) }
`, "", "", `parse error at 2:14: can't use function name as parameter name`, "function name"},
	{`function foo() { print foo }  BEGIN { foo() }`,
		"", "", `parse error at 1:46: global var "foo" can't also be a function`, "function"},
	{`function f(x) { print x, x(); }  BEGIN { f() }`, "", "", `parse error at 1:27: can't call local variable "x" as function`, "function"},

	// Redirected I/O
	{`BEGIN { getline x; print x }`, "foo", "foo\n", "", ""},
	{`function f(x) { getline x; print x }  BEGIN { f(); print x }`, "foo", "foo\n\n", "", ""},
	{`BEGIN { getline SUBSEP; print SUBSEP }`, "foo", "foo\n", "", ""},
	{`BEGIN { getline a[1]; print a[1] }`, "foo", "foo\n", "", ""},
	{`BEGIN { getline $1; print $1 }`, "foo", "foo\n", "", ""},
	{`BEGIN { "echo foo" | getline a[1]; print a[1] }`, "", "foo\n", "", ""},
	{`BEGIN { "echo foo" | getline $1; print $1 }`, "", "foo\n", "", ""},
	{`BEGIN { print "foo" |"sort"; print "bar" |"sort" }  # !fuzz`, "", "bar\nfoo\n", "", ""},
	{`BEGIN { print "foo" |">&2 echo error" }  # !gawk !fuzz`, "", "error\n", "", ""},
	{`BEGIN { "cat" | getline; print }  # !fuzz`, "bar", "bar\n", "", ""},
	{`BEGIN { print getline x < "/no/such/file" }  # !fuzz`, "", "-1\n", "", ""},
	{`BEGIN { print getline "z"; print $0 }`, "foo", "1z\nfoo\n", "", ""},
	{`BEGIN { print getline x+1; print x }`, "foo", "2\nfoo\n", "", ""},
	{`BEGIN { print getline (x+1); print $0 }`, "foo", "11\nfoo\n", "", ""},
	{`BEGIN { print getline foo(); print $0 } function foo() { print "z" }`, "foo", "z\n1\nfoo\n", "", ""},
	// TODO: these forms don't yet work under GoAWK
	//{`BEGIN { print("echo foo" | getline x+1); print x }`, "", "2\nfoo\n", "", ""},
	//{`BEGIN { print("echo foo" | getline $0+1); print }`, "", "2\nfoo\n", "", ""},
	//{`BEGIN { print("echo foo" | getline ($0+1)); print }`, "", "11\nfoo\n", "", ""},
	//{`BEGIN { print("echo foo" | getline foo()); print } function foo() { print "z" }`, "", "z\n1\nfoo\n", "", ""},
	{`BEGIN {
	print "foo" >"out"
	print close("out")
	print "bar" >"out"
	print close("out")
	getline <"out"
	print $0
	print close("out")
	print close("out")
}`, "", "0\n0\nbar\n0\n-1\n", "", ""},
	{`BEGIN {
	print "foo" >"out"
	print "bar" >"out"
	print close("out")
	getline <"out"
	print $0
	print close("out")
	getline <"out"
	print $0
	print close("out")
	print close("out")
}`, "", "0\nfoo\n0\nfoo\n0\n-1\n", "", ""},
	{`BEGIN { print close("nothing") }`, "", "-1\n", "", ""},
	{`BEGIN {
	print "foo">"out"
	close("out")
	print "bar">>"out"
	close("out")
	getline <"out"
	print $0
	getline <"out"
	print $0
}`, "", "foo\nbar\n", "", ""},

	// Ensure data returned by getline (in various forms) is treated as numeric string
	{`BEGIN { getline; print($0==0) }`, "0.0", "1\n", "", ""},
	{`BEGIN { getline x; print(x==0) }`, "0.0", "1\n", "", ""},
	{`BEGIN { "echo 0.0" | getline; print($0==0) }`, "", "1\n", "", ""},
	{`BEGIN { "echo 0.0" | getline x; print(x==0) }`, "", "1\n", "", ""},

	// Redirected I/O errors (we give explicit errors, awk and gawk don't)
	{`BEGIN { print >"out"; getline <"out" }  # !awk !gawk`, "", "", "can't read from writer stream", ""},
	{`BEGIN { print |"out"; getline <"out" }  # !awk !gawk`, "", "", "can't read from writer stream", ""},
	{`BEGIN { print >"out"; close("out"); getline <"out"; print >"out" }  # !awk !gawk`, "", "", "can't write to reader stream", ""},
	{`BEGIN { print >"out"; close("out"); getline <"out"; print |"out" }  # !awk !gawk`, "", "", "can't write to reader stream", ""},

	// Redirecting to or from a filename of "-" means write to stdout or read from stdin
	{`BEGIN { print getline x < "-"; print x }`, "a\nb\n", "1\na\n", "", ""},
	{`{ print $0; print getline x <"-"; print x }`, "one\ntwo\n", "one\n0\n\ntwo\n0\n\n", "", ""},
	{`BEGIN { print "x" >"-"; print "y" >"-" }`, "", "x\ny\n", "", ""},

	// fflush() function - tests parsing and some edge cases, but not
	// actual flushing behavior (that's partially tested in TestFlushes).
	{`BEGIN { print fflush(); print fflush("") }`, "", "0\n0\n", "", ""},
	{`BEGIN { print "x"; print fflush(); print "y"; print fflush("") }`, "", "x\n0\ny\n0\n", "", ""},
	{`BEGIN { print "x" >"out"; print fflush("out"); print "y"; print fflush("") }  # !fuzz`, "", "0\ny\n0\n", "", ""},
	{`BEGIN { print fflush("x") }  # !gawk`, "", "error flushing \"x\": not an output file or pipe\n-1\n", "", ""},
	{`BEGIN { "cat" | getline; print fflush("cat") }  # !gawk !fuzz`, "", "error flushing \"cat\": not an output file or pipe\n-1\n", "", ""},

	// Greater than operator requires parentheses in print statement,
	// otherwise it's a redirection directive
	{`BEGIN { print "x" > "out" }  # !fuzz`, "", "", "", ""},
	{`BEGIN { printf "x" > "out" }  # !fuzz`, "", "", "", ""},
	{`BEGIN { print("x" > "out") }`, "", "1\n", "", ""},
	{`BEGIN { printf("x" > "out") }`, "", "1", "", ""},

	// Grammar should allow blocks wherever statements are allowed
	{`BEGIN { if (1) printf "x"; else printf "y" }`, "", "x", "", ""},
	{`BEGIN { printf "x"; { printf "y"; printf "z" } }`, "", "xyz", "", ""},

	// Backslash line continuation
	{"BEGIN { print 1,\\\n 2 }", "", "1 2\n", "", ""},
	{"BEGIN { print 1,\\\r\n 2 }", "", "1 2\n", "", ""},

	// Ensure syntax errors result in errors
	{`{ $1 = substr($1, 1, 3) print $1 }`, "", "", "parse error at 1:25: expected ; or newline between statements", "syntax error"},
	{`BEGIN { f() }`, "", "", `parse error at 1:9: undefined function "f"`, "defined"},
	{`function f() {} function f() {} BEGIN { }`, "", "", `parse error at 1:26: function "f" already defined`, "define"},
	{`BEGIN { print (1,2),(3,4) }`, "", "", "parse error at 1:15: unexpected comma-separated expression", "syntax"},
	{`BEGIN { print (1,2,(3,4),(5,6)) }`, "", "", "parse error at 1:20: unexpected comma-separated expression", "syntax"},
	{"BEGIN { print 1,\\2 }", "", "1 2\n", `parse error at 1:18: expected \n after \ line continuation`, "backslash not last character on line"},
	{`BEGIN { print . }`, "", "", "parse error at 1:16: expected digits", "syntax"},
	{`BEGIN { print "foo }`, "", "", "parse error at 1:21: didn't find end quote in string", "unterminated string"},
	{"BEGIN { print \"foo\n\"}", "", "", "parse error at 1:19: can't have newline in string", "unterminated string"},
	{`/foo`, "", "", "parse error at 1:5: didn't find end slash in regex", "unterminated regexp"},
	{"/foo\n", "", "", "parse error at 1:5: can't have newline in regex", "unterminated regexp"},
	{`BEGIN { print "\x" }  # !gawk`, "", "", "parse error at 1:18: 1 or 2 hex digits expected", ""},
	{`BEGIN { print 1&*2 }`, "", "", "parse error at 1:17: unexpected char after '&'", "syntax"},
	{"BEGIN { ` }", "", "", "parse error at 1:9: unexpected char", "syntax"},
}

func TestInterp(t *testing.T) {
	// Ensure very long lines work (> 64KB)
	longLine := strings.Repeat("x", 70000)
	tests := append(interpTests,
		interpTest{`{ print length() }`, longLine, fmt.Sprintf("%d\n", len(longLine)), "", ""},
	)

	for _, test := range tests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}

		// Run it through external awk program first
		if awkExe != "" {
			t.Run("awk_"+testName, func(t *testing.T) {
				if strings.Contains(test.src, "!"+awkExe) {
					t.Skipf("skipping under %s", awkExe)
				}
				if strings.Contains(test.src, "!"+runtime.GOOS+"-"+awkExe) {
					t.Skipf("skipping on %s under %s", runtime.GOOS, awkExe)
				}
				cmd := exec.Command(awkExe, test.src, "-")
				if test.in != "" {
					cmd.Stdin = strings.NewReader(test.in)
				}
				out, err := cmd.CombinedOutput()
				if err != nil {
					if test.awkErr != "" {
						if strings.Contains(string(out), test.awkErr) {
							return
						}
						t.Fatalf("expected error %q, got:\n%s", test.awkErr, out)
					} else {
						t.Fatalf("error running %s: %v:\n%s", awkExe, err, out)
					}
				}
				if test.awkErr != "" {
					t.Fatalf(`expected error %q, got ""`, test.awkErr)
				}
				normalized := normalizeNewlines(string(out))
				if normalized != test.out {
					t.Fatalf("expected %q, got %q", test.out, normalized)
				}
			})
		}

		// Then test it in GoAWK
		t.Run(testName, func(t *testing.T) {
			testGoAWK(t, test.src, test.in, test.out, test.err, nil, nil)
		})
	}
	_ = os.Remove("out")
}

// Version of bytes.Buffer that's safe for concurrent writes. This
// makes certain tests that write to Output and Error at once (due
// to os/exec) work correctly.
type concurrentBuffer struct {
	buffer bytes.Buffer
	mutex  sync.Mutex
}

func (b *concurrentBuffer) Write(data []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.Write(data)
}

func (b *concurrentBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.String()
}

func testGoAWK(
	t *testing.T, src, in, out, errStr string,
	funcs map[string]interface{}, configure func(config *interp.Config),
) {
	parserConfig := &parser.ParserConfig{
		Funcs: funcs,
	}
	prog, err := parser.ParseProgram([]byte(src), parserConfig)
	if err != nil {
		if errStr != "" {
			if err.Error() == errStr {
				return
			}
			t.Fatalf("expected error %q, got %q", errStr, err.Error())
		}
		t.Fatal(err)
	}

	// Test that disassembler at least doesn't panic or return an error.
	err = prog.Disassemble(ioutil.Discard)
	if err != nil {
		t.Fatalf("disassembler returned an error: %v", err)
	}

	outBuf := &concurrentBuffer{}
	config := &interp.Config{
		Stdin:  strings.NewReader(in),
		Output: outBuf,
		Error:  outBuf,
		Vars:   []string{"_var", "42"},
		Funcs:  funcs,
	}
	if configure != nil {
		configure(config)
	}
	status, err := interp.ExecProgram(prog, config)
	if err != nil {
		if errStr != "" {
			if err.Error() == errStr {
				return
			}
			t.Fatalf("expected error %q, got %q", errStr, err.Error())
		}
		t.Fatal(err)
	}
	if errStr != "" {
		t.Fatalf(`expected error %q, got ""`, errStr)
	}
	normalized := normalizeNewlines(outBuf.String())
	if normalized != out {
		t.Fatalf("expected %q, got %q", out, normalized)
	}
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
}

func TestNative(t *testing.T) {
	tests := []struct {
		src   string
		in    string
		out   string
		err   string
		funcs map[string]interface{}
	}{
		{`BEGIN { print foo() }`, "", "", `parse error at 1:15: undefined function "foo"`,
			nil},
		{`BEGIN { print foo() }`, "", "\n", "",
			map[string]interface{}{
				"foo": func() {},
			}},
		{`BEGIN { print foo() }`, "", "FOO\n", "",
			map[string]interface{}{
				"foo": func() string { return "FOO" },
			}},
		{`BEGIN { print foo() }`, "", "BYTES\n", "",
			map[string]interface{}{
				"foo": func() []byte { return []byte("BYTES") },
			}},
		{`BEGIN { print repeat("xy", 5) }`, "", "xyxyxyxyxy\n", "",
			map[string]interface{}{
				"repeat": strings.Repeat,
			}},
		{`BEGIN { print repeat("xy", 5) }`, "", "xyxyxyxyxy\n", "",
			map[string]interface{}{
				"repeat": strings.Repeat,
			}},
		{`
BEGIN {
	print r0()
	print r1(), r1(5)
	print r2(), r2(5)
}`, "", "\n0 25\n0 25\n", "",
			map[string]interface{}{
				"r0": func() {},
				"r1": func(n int) int { return n * n },
				"r2": func(n int) (int, error) {
					return n * n, nil
				},
			}},
		{`
BEGIN {
	print r2()
}`, "", "", "NATIVE ERROR",
			map[string]interface{}{
				"r2": func(n int) (int, error) {
					return n * n, fmt.Errorf("NATIVE ERROR")
				},
			}},
		{`
BEGIN {
	print
	print bool(), bool(0), bool(1), bool(""), bool("0"), bool("x")
	print i(), i(42), i(-5), i(3.75), i(-3.75)
	print i8(), i8(42), i8(-5.6), i8(127), i8(-128)
	print i16(), i16(42), i16(-5.6), i16(32767), i16(-32768)
	print i32(), i32(42), i32(-5.6), i32(2147483647), i32(-2147483648)
	print i64(), i64(42), i64(-5.6), i64(2147483647000), i64(-2147483647000)
	print u(), u(42), u(0), u(1)
	print u8(), u8(42), u8(-5.6), u8(127), u8(128), u8(255)
	print u16(), u16(42), u16(-1), u16(65535)
	print u32(), u32(42), u32(-1), u32(4294967295)
	print u64(), u64(42), u64(1), u64(4294967296), u64(2147483647000)
	print s() "." s("") "." s("Foo bar") "." s(1234)
	print b() "." b("") "." b("Foo bar") "." b(1234)
}`, "", `
0 0 1 0 1 1
0 42 -5 3 -3
0 42 -5 127 -128
0 42 -5 32767 -32768
0 42 -5 2147483647 -2147483648
0 42 -5 2147483647000 -2147483647000
0 42 0 1
0 42 251 127 128 255
0 42 65535 65535
0 42 4294967295 4294967295
0 42 1 4294967296 2147483647000
..Foo bar.1234
..Foo bar.1234
`, "",
			map[string]interface{}{
				"bool": func(b bool) bool { return b },
				"i":    func(n int) int { return n },
				"i8":   func(n int8) int8 { return n },
				"i16":  func(n int16) int16 { return n },
				"i32":  func(n int32) int32 { return n },
				"i64":  func(n int64) int64 { return n },
				"u":    func(n uint) uint { return n },
				"u8":   func(n uint8) uint8 { return n },
				"u16":  func(n uint16) uint16 { return n },
				"u32":  func(n uint32) uint32 { return n },
				"u64":  func(n uint64) uint64 { return n },
				"b":    func(b []byte) []byte { return b },
				"s":    func(s string) string { return s },
			}},
		{`
BEGIN {
	print
	print sum(), sum(1), sum(2, 3), sum(4, 5, 6, 7, 8)
	print fmt_ints()
	print fmt_ints("%5d")
	print fmt_ints("%5d", 123)
	print fmt_ints("%d %d", 123, 456)
	print fmt_ints("%d %d %d", 123, 456, 789)
}`, "", `
0 1 5 30

%!d(MISSING)
  123
123 456
123 456 789
`, "",
			map[string]interface{}{
				"sum": func(args ...int) int {
					sum := 0
					for _, a := range args {
						sum += a
					}
					return sum
				},
				"fmt_ints": func(s string, args ...int) string {
					fmtArgs := make([]interface{}, len(args))
					for i, a := range args {
						fmtArgs[i] = a
					}
					return fmt.Sprintf(s, fmtArgs...)
				},
			}},
		{`BEGIN { 0 }`, "", "", `native function "f" is not a function`,
			map[string]interface{}{
				"f": 0,
			}},
		{`BEGIN { 1 }`, "", "", `native function "g" param 0 is not int or string`,
			map[string]interface{}{
				"g": func(s complex64) {},
			}},
		{`BEGIN { 2 }`, "", "", `native function "g" param 2 is not int or string`,
			map[string]interface{}{
				"g": func(x, y int, s []int, t string) {},
			}},
		{`BEGIN { 3 }`, "", "", `native function "h" param 0 is not int or string`,
			map[string]interface{}{
				"h": func(a ...map[string]int) {},
			}},
		{`BEGIN { 4 }`, "", "", `native function "h" param 1 is not int or string`,
			map[string]interface{}{
				"h": func(x int, a ...complex64) {},
			}},
		{`BEGIN { 5 }`, "", "", `native function "r" return value is not int or string`,
			map[string]interface{}{
				"r": func() map[string]int { return nil },
			}},
		{`BEGIN { 6 }`, "", "", `native function "r" first return value is not int or string`,
			map[string]interface{}{
				"r": func() (map[string]int, error) { return nil, nil },
			}},
		{`BEGIN { 7 }`, "", "", `native function "r" second return value is not an error`,
			map[string]interface{}{
				"r": func() (int, int) { return 0, 0 },
			}},
		{`BEGIN { 8 }`, "", "", `native function "r" returns more than two values`,
			map[string]interface{}{
				"r": func() (int, error, int) { return 0, nil, 0 },
			}},
		{`BEGIN { print f(), f(1, 2) }`, "", "", `parse error at 1:20: "f" called with more arguments than declared`,
			map[string]interface{}{
				"f": func(n int) {},
			}},
		{`BEGIN { print split("x y", a) }`, "", "", `can't use keyword "split" as native function name`,
			map[string]interface{}{
				"split": func() {},
			}},
		{`
function foo(n) { return n * 2 }
BEGIN { print foo(42) }
`, "", "84\n", "", map[string]interface{}{
			"foo": func(n int) int { return n / 2 },
		}},
		{`BEGIN { x=3; print foo(x) }`, "", "9\n", ``,
			map[string]interface{}{
				"foo": func(n int) int { return n * n },
			}},
		{`
function bar(n) { return foo(n) }
BEGIN { x=4; y=5; print foo(x), bar(y) }
`, "", "16 25\n", ``,
			map[string]interface{}{
				"foo": func(n int) int { return n * n },
			}},
		{`BEGIN { a["x"]=1; print foo(a) }`, "", "",
			`parse error at 1:25: can't pass array "a" to native function`,
			map[string]interface{}{
				"foo": func(n int) int { return n * n },
			}},
		{`BEGIN { x["x"]=1; print f(x) }  function f(a) { return foo(a) }`, "", "",
			`parse error at 1:56: can't pass array "a" to native function`,
			map[string]interface{}{
				"foo": func(n int) int { return n * n },
			}},
		{`function f(a) { return foo(a) }  BEGIN { x["x"]=1; print f(x) }`, "", "",
			`parse error at 1:24: can't pass array "a" to native function`,
			map[string]interface{}{
				"foo": func(n int) int { return n * n },
			}},
		{`BEGIN { x["x"]=1; print f(x["x"]) }  function f(a) { return foo(a) }`, "", "1\n", "",
			map[string]interface{}{
				"foo": func(n int) int { return n * n },
			}},
		{`BEGIN { print add(1, add(2, 3)) }`, "", "6\n", "",
			map[string]interface{}{
				"add": func(a, b float64) float64 { return a + b },
			}},
		{`BEGIN { print add(1, add(2, 3)) }`, "", "6\n", "",
			map[string]interface{}{
				"add": func(a, b float32) float32 { return a + b },
			}},
		{`BEGIN { print foo(x) }`, "", "0\n", "",
			map[string]interface{}{
				"foo": func(i int) int { return i },
			}},
		{`BEGIN { print foo(_var) }`, "", "42\n", "",
			map[string]interface{}{
				"foo": func(i int) int { return i },
			}},
		{`function foo(y) { return y/2 }  BEGIN { print foo(_var) }`, "", "21\n", "",
			map[string]interface{}{
				"foo": func(i int) int { return i },
			}},
	}
	for _, test := range tests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}
		t.Run(testName, func(t *testing.T) {
			testGoAWK(t, test.src, test.in, test.out, test.err, test.funcs, nil)
		})
	}
}

func TestSafeMode(t *testing.T) {
	tests := []struct {
		src  string
		in   string
		out  string
		err  string
		args []string
	}{
		{`BEGIN { print "hi" >"out" }`, "", "", "can't write to file due to NoFileWrites", nil},
		{`BEGIN { print "hi" >>"out" }`, "", "", "can't write to file due to NoFileWrites", nil},
		{`BEGIN { print "hi" |"sort" }`, "", "", "can't write to pipe due to NoExec", nil},
		{`BEGIN { getline <"in" }`, "", "", "can't read from file due to NoFileReads", nil},
		{`$0  # no files`, "1\n2\n", "1\n2\n", "", nil},
		{`$0  # files`, "1\n2\n", "1\n2\n", "can't read from file due to NoFileReads", []string{"f1"}},
		{`BEGIN { "echo foo" |getline }`, "", "", "can't read from pipe due to NoExec", nil},
		{`BEGIN { system("echo foo") }`, "", "", "can't call system() due to NoExec", nil},
	}
	for _, test := range tests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}
		t.Run(testName, func(t *testing.T) {
			testGoAWK(t, test.src, test.in, test.out, test.err, nil, func(config *interp.Config) {
				config.Args = test.args
				config.NoExec = true
				config.NoFileWrites = true
				config.NoFileReads = true
			})
		})
	}
}

func TestConfigVarsCorrect(t *testing.T) {
	prog, err := parser.ParseProgram([]byte(`BEGIN { print x }`), nil)
	if err != nil {
		t.Fatalf("error parsing: %v", err)
	}
	config := &interp.Config{
		Stdin:  strings.NewReader(""),
		Output: &bytes.Buffer{},
		Error:  ioutil.Discard,
		Vars:   []string{"FS"},
	}
	_, err = interp.ExecProgram(prog, config)
	expected := "length of config.Vars must be a multiple of 2, not 1"
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error %q, got: %v", expected, err)
	}
}

func TestShellCommand(t *testing.T) {
	testGoAWK(t, `BEGIN { system("echo hello world") }`, "", "hello world\n", "", nil, nil)

	if runtime.GOOS == "windows" {
		testGoAWK(t, `BEGIN { system("echo hello world") }`, "", "hello world\n", "", nil,
			func(config *interp.Config) {
				config.ShellCommand = []string{"cmd.exe", "/c"}
			})
	} else {
		testGoAWK(t, `BEGIN { system("world") }`, "", "hello world\n", "", nil,
			func(config *interp.Config) {
				config.ShellCommand = []string{"/bin/echo", "hello"}
			})
		testGoAWK(t, `BEGIN { "world" | getline; print }`, "", "hello world\n", "", nil,
			func(config *interp.Config) {
				config.ShellCommand = []string{"/bin/echo", "hello"}
			})
		testGoAWK(t, `BEGIN { print "hello world" | "-" }`, "", "hello world\n", "", nil,
			func(config *interp.Config) {
				config.ShellCommand = []string{"/bin/cat"}
			})
		testGoAWK(t, `BEGIN { print system("echo hi") }`, "", "exec: \"foobar3982\": executable file not found in $PATH\n-1\n", "", nil,
			func(config *interp.Config) {
				config.ShellCommand = []string{"foobar3982"}
			})
	}
}

func TestSystemCommandNotFound(t *testing.T) {
	prog, err := parser.ParseProgram([]byte(`BEGIN { print system("foobar3982") }`), nil)
	if err != nil {
		t.Fatalf("error parsing: %v", err)
	}
	outBuf := &concurrentBuffer{}
	config := &interp.Config{
		Output: outBuf,
		Error:  outBuf,
	}
	_, err = interp.ExecProgram(prog, config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got := outBuf.buffer.String()
	if !strings.Contains(got, "foobar3982") || !strings.Contains(got, "not found") {
		t.Fatalf(`expected output to contain "foobar3982" and "not found", got %q`, got)
	}
}

type mockFlusher struct {
	bytes.Buffer
	flushes []string
}

func (f *mockFlusher) Flush() error {
	f.flushes = append(f.flushes, normalizeNewlines(f.String()))
	return nil
}

func TestFlushes(t *testing.T) {
	src := `
BEGIN {
	print fflush()
	print "x"
	print "y"
	print fflush()
	print "z"
	print fflush("")
}`
	f := &mockFlusher{}
	testGoAWK(t, src, "", "", "", nil, func(config *interp.Config) {
		config.Output = f
	})
	// The last one is from GoAWK itself flushing output after finishing
	expected := []string{"", "0\nx\ny\n", "0\nx\ny\n0\nz\n", "0\nx\ny\n0\nz\n0\n"}
	if !reflect.DeepEqual(f.flushes, expected) {
		t.Fatalf("expected flushes %q, got %q", expected, f.flushes)
	}

	// Ensure output is flushed before getline reads from stdin
	src = `
BEGIN {
	printf "Prompt: "
	getline x
	print x
}`
	f = &mockFlusher{}
	testGoAWK(t, src, "42\n", "", "", nil, func(config *interp.Config) {
		config.Output = f
	})
	expected = []string{"Prompt: ", "Prompt: 42\n"}
	if !reflect.DeepEqual(f.flushes, expected) {
		t.Fatalf("expected flushes %q, got %q", expected, f.flushes)
	}

	// Ensure output is flushed before system()
	src = `
BEGIN {
  print "one"
  system("echo .")
  print "two"
}`
	f = &mockFlusher{}
	testGoAWK(t, src, "", "", "", nil, func(config *interp.Config) {
		config.Output = f
	})
	expected = []string{"one\n", "one\n.\ntwo\n"}
	if !reflect.DeepEqual(f.flushes, expected) {
		t.Fatalf("expected flushes %q, got %q", expected, f.flushes)
	}
}

type errorFlusher struct {
	bytes.Buffer
}

func (f *errorFlusher) Flush() error {
	return errors.New("that's not good, hackers")
}

func TestFlushError(t *testing.T) {
	f := &errorFlusher{}
	testGoAWK(t, `BEGIN { fflush() }`, "", "", "", nil, func(config *interp.Config) {
		config.Output = f
		config.Error = f
	})
	expected := "error flushing \"stdout\": that's not good, hackers\n"
	if f.String() != expected {
		t.Fatalf("expected %q, got %q", expected, f.String())
	}
}

func TestEnviron(t *testing.T) {
	os.Setenv("GOAWK_TEN", "10") // to test that ENVIRON[x] is numeric string
	src := `
BEGIN {
	n = 0
	for (k in ENVIRON)
		n++
	print(n, ENVIRON["USER"], ENVIRON["GOAWK_TEN"] < 2)
}`
	expected := fmt.Sprintf("%d %s 0\n", len(os.Environ()), os.Getenv("USER"))
	testGoAWK(t, src, "", expected, "", nil, nil)

	expected = "2 bob 0\n"
	testGoAWK(t, src, "", expected, "", nil, func(config *interp.Config) {
		config.Environ = []string{"USER", "bob", "GOAWK_TEN", "10"}
	})

	expected = "0  1\n"
	testGoAWK(t, src, "", expected, "", nil, func(config *interp.Config) {
		config.Environ = []string{}
	})

	testGoAWK(t, src, "", "", "length of config.Environ must be a multiple of 2, not 3", nil, func(config *interp.Config) {
		config.Environ = []string{"b", "a", "d"}
	})
}

func TestExit(t *testing.T) {
	tests := []struct {
		src    string
		out    string
		status int
	}{
		{`BEGIN { print "x"; exit; print "y" }  { print "a" }  END { print "z" }`, "x\nz\n", 0},
		{`BEGIN { print "x"; exit 1+2; print "y" }  { print "a" }  END { print "z" }`, "x\nz\n", 3},
		{`{ print "x"; exit; print "y" }  END { print "z" }`, "x\nz\n", 0},
		{`{ print "x"; exit 1+2; print "y" }  END { print "z" }`, "x\nz\n", 3},
		{`END { print "x"; exit; print "y" }`, "x\n", 0},
		{`END { print "x"; exit 1+2; print "y" }`, "x\n", 3},
	}
	for _, test := range tests {
		t.Run(test.src, func(t *testing.T) {
			prog, err := parser.ParseProgram([]byte(test.src), nil)
			if err != nil {
				t.Fatalf("error parsing: %v", err)
			}
			outBuf := &bytes.Buffer{}
			config := &interp.Config{
				Stdin:  strings.NewReader("line\n"),
				Output: outBuf,
			}
			status, err := interp.ExecProgram(prog, config)
			if err != nil {
				t.Fatalf("error interpreting: %v", err)
			}
			normalized := normalizeNewlines(outBuf.String())
			if normalized != test.out {
				t.Fatalf("expected %q, got %q", test.out, normalized)
			}
			if status != test.status {
				t.Fatalf("expected status %d, got %d", test.status, status)
			}
		})
	}
}

type csvTest struct {
	src       string
	in        string
	out       string
	err       string
	configure func(config *interp.Config)
}

var csvTests = []csvTest{
	// INPUTMODE combinations
	{`BEGIN { INPUTMODE="" } { print $1, $3 }`, "name,email\nBob C. Smith,bob@smith.com\nJane X. Brown,jane@brown.com", "name,email \nBob Smith,bob@smith.com\nJane Brown,jane@brown.com\n", "", nil},
	{`BEGIN { INPUTMODE="csv header" } { print $1, $3 }`, "name,email,age\nBob\tSmith,bob@smith.com,42\n\nJane,jane@brown.com,37\n# not a comment", "Bob\tSmith 42\nJane 37\n# not a comment \n", "", nil},
	{`BEGIN { INPUTMODE="csv separator=|" } { print $1, $3 }`, "Bob,Smith|bob@smith.com|42\nJane|jane@brown.com|37", "Bob,Smith 42\nJane 37\n", "", nil},
	{`BEGIN { INPUTMODE="csv comment=#" } { print $1, $3 }`, "# this is a comment\nBob\tSmith,bob@smith.com,42\nJane,jane@brown.com,37", "Bob\tSmith 42\nJane 37\n", "", nil},
	{`BEGIN { INPUTMODE="csv" } { print $1, $3 }`, "name,email,age\nBob,bob@smith.com,42\nJane,jane@brown.com,37", "name age\nBob 42\nJane 37\n", "", nil},
	{`BEGIN { INPUTMODE="csv header" } { print @"age", @"name" }`, "name,email,age\nBob,bob@smith.com,42\nJane,jane@brown.com,37", "42 Bob\n37 Jane\n", "", nil},
	{`BEGIN { INPUTMODE="csv header" } { x="name"; print @"age", @x }`, "name,age\nBob,42", "42 Bob\n", "", nil},
	{`BEGIN { INPUTMODE="csv" } { print @"age", @"name" }`, "name,email,age\nBob,bob@smith.com,42\nJane,jane@brown.com,37", "", `@ only supported if header parsing enabled; use -H or add "header" to INPUTMODE`, nil},
	{`BEGIN { INPUTMODE="tsv header" } { print $1, $3 }`, "name\temail\tage\nBob,Smith\tbob@smith.com\t42\nJane\tjane@brown.com\t37", "Bob,Smith 42\nJane 37\n", "", nil},

	// OUTPUTMODE combinations
	{`BEGIN { OUTPUTMODE="csv" } { print $2, $1 }`, "a\"b c\nd e", "c,\"a\"\"b\"\ne,d\n", "", nil},
	{`BEGIN { OUTPUTMODE="tsv" } { print $2, $1 }`, "a\"b c\nd e", "c\t\"a\"\"b\"\ne\td\n", "", nil},
	{`BEGIN { OUTPUTMODE="csv separator=|" } { print $2, $1 }`, "a\"b c\nd e", "c|\"a\"\"b\"\ne|d\n", "", nil},

	// Both input and output in CSV (or TSV) mode
	{`BEGIN { INPUTMODE="csv header"; OUTPUTMODE="csv"; print "age", "name" } { print $2, $1 }`, "name,age\nBob,42\n\"J B\",37\n\"A\"\"B\",7", "age,name\n42,Bob\n37,J B\n7,\"A\"\"B\"\n", "", nil},
	{`BEGIN { INPUTMODE="csv"; OUTPUTMODE="tsv"; } { $1=$1; print }`, "name,age\nBob,42\n\"J B\",37\n\"A\"\"B\",7", "name\tage\nBob\t42\nJ B\t37\n\"A\"\"B\"\t7\n", "", nil},

	// Configure via interp.Config struct
	{`{ print $2, $1 }`, "name,age\nBob,42", "age name\n42 Bob\n", "", func(config *interp.Config) {
		config.InputMode = interp.CSVMode
	}},
	{`{ print $2, $1 }`, "name\tage\nBob\t42", "age name\n42 Bob\n", "", func(config *interp.Config) {
		config.InputMode = interp.TSVMode
	}},
	{`{ print $2, $1 }`, "# comment\nBob;42", "42 Bob\n", "", func(config *interp.Config) {
		config.InputMode = interp.CSVMode
		config.CSVInput.Separator = ';'
		config.CSVInput.Comment = '#'
	}},
	{`{ print $1, $2 }`, "", "", "input mode configuration not valid in default input mode", func(config *interp.Config) {
		config.CSVInput.Separator = ';'
	}},
	{`{ print $2, $1 }`, "Bob,42\nJane,37", "42\tBob\n37\tJane\n", "", func(config *interp.Config) {
		config.InputMode = interp.CSVMode
		config.OutputMode = interp.TSVMode
	}},
	{`BEGIN { INPUTMODE="tsv header"; OUTPUTMODE="csv" } { print @"age", @"name" }`, "name\tage\nBob\t42", "42,Bob\n", "", func(config *interp.Config) {
		config.InputMode = interp.CSVMode // will be overridden by BEGIN
		config.OutputMode = interp.TSVMode
	}},
	{`{ print @"age", @"name" }`, "name\tage\nBob\t42", "42,Bob\n", "", func(config *interp.Config) {
		config.InputMode = interp.CSVMode // will be overridden by Vars
		config.OutputMode = interp.TSVMode
		config.Vars = []string{"INPUTMODE", "tsv header", "OUTPUTMODE", "csv"}
	}},
	{`{ print $2, $1 }`, "Bob 42", "42,Bob\n", "", func(config *interp.Config) {
		config.OutputMode = interp.CSVMode
	}},
	{`{ print $2, $1 }`, "Bob 42", "42\tBob\n", "", func(config *interp.Config) {
		config.OutputMode = interp.TSVMode
	}},
	{`{ print $2, $1 }`, "Bob 42", "42;Bob\n", "", func(config *interp.Config) {
		config.OutputMode = interp.CSVMode
		config.CSVOutput.Separator = ';'
	}},
	{`{ print $1, $2 }`, "", "", "output mode configuration not valid in default output mode", func(config *interp.Config) {
		config.CSVOutput.Separator = ';'
	}},

	// $0 still works as expected in CSV mode
	{`BEGIN { INPUTMODE="csv header" } { print }`, "name,age\nBob,42\nJane,37", "Bob,42\nJane,37\n", "", nil},
	{`BEGIN { INPUTMODE="csv header" } { print $0 }`, "name,age\nBob,42\nJane,37", "Bob,42\nJane,37\n", "", nil},
	{`BEGIN { INPUTMODE="csv header" } { print $0; $0=NR; print $0 }`, "name,age\nBob,42\nJane,37", "Bob,42\n1\nJane,37\n2\n", "", nil},
	{`BEGIN { INPUTMODE="csv header comment=#" } { print $0 } END { for (i=1; i in FIELDS; i++) print i, FIELDS[i] }`,
		"# comment\n\nname,age\n# comment\n\nBob,42\n# comment\nJane,37\n\nFoo,5",
		"Bob,42\nJane,37\nFoo,5\n1 name\n2 age\n", "", nil},

	// CSV filters
	{`BEGIN { INPUTMODE="csv header" } /foo/ { print $2 }`, "id,type\n1,food\n2,bar\n3,foo\n", "food\nfoo\n", "", nil},
	{`BEGIN { INPUTMODE="csv header" } $1==2 { print $2 }`, "id,type\n1,food\n2,bar\n3,foo\n", "bar\n", "", nil},
	{`BEGIN { INPUTMODE="csv" } { s += $-1 } END { print s }`, "a,1\nb,2\nc,3\n", "6\n", "", nil},

	// Updating fields
	{`BEGIN { INPUTMODE="csv" } { $1 = $1 $1; print $1, $2 }`, "a,1\nb,2", "aa 1\nbb 2\n", "", nil},
	{`BEGIN { INPUTMODE="csv" } { $1 = $1 $1; print }`, "a,1\nb,2", "aa 1\nbb 2\n", "", nil},
	{`BEGIN { INPUTMODE="csv" } { $0 = "X,3"; print $1, $2 }`, "a,1\nb,2", "X 3\nX 3\n", "", nil},
	{`BEGIN { INPUTMODE="csv" } { $0 = "X,3"; print }`, "a,1\nb,2", "X,3\nX,3\n", "", nil},
	{`BEGIN { INPUTMODE=OUTPUTMODE="csv" } { $1 = $1 $1; print $1, $2 }`, "a,1\nb,2", "aa,1\nbb,2\n", "", nil},
	{`BEGIN { INPUTMODE=OUTPUTMODE="csv" } { $1 = $1 $1; print }`, "a,1\nb,2", "aa,1\nbb,2\n", "", nil},
	{`BEGIN { INPUTMODE=OUTPUTMODE="csv" } { $0 = "X,3"; print $1, $2 }`, "a,1\nb,2", "X,3\nX,3\n", "", nil},
	{`BEGIN { INPUTMODE=OUTPUTMODE="csv" } { $0 = "X,3"; print }`, "a,1\nb,2", "X,3\nX,3\n", "", nil},
	{`BEGIN { OUTPUTMODE="csv"; $0 = "a b c"; printf "%s|%s %s %s\n", $0, $1, $2, $3; NF=2; printf "%s|%s %s\n", $0, $1, $2 }`, "", "a b c|a b c\na,b|a b\n", "", nil},
	{`BEGIN { OUTPUTMODE="csv"; $0 = "a b c"; printf "%s|%s %s %s\n", $0, $1, $2, $3; NF=4; printf "%s|%s %s %s %s\n", $0, $1, $2, $3, $4 }`, "", "a b c|a b c\na,b,c,|a b c \n", "", nil},

	// FIELDS array
	{`BEGIN { INPUTMODE="csv header" } NR==1 { for (i=1; i in FIELDS; i++) print i, FIELDS[i] }`, "name,email,age\na,b,c", "1 name\n2 email\n3 age\n", "", nil},
	{`BEGIN { INPUTMODE="csv" } NR==1 { for (i=1; i in FIELDS; i++) print FIELDS[i] }`, "name,email,age\na,b,c", "", "", nil},

	// Parsing and formatting of INPUTMODE and OUTPUTMODE special variables
	{`BEGIN { INPUTMODE="csv separator=,"; print INPUTMODE }`, "", "csv\n", "", nil},
	{`BEGIN { INPUTMODE="csv header=true comment=# separator=|"; print INPUTMODE }`, "", "csv separator=| comment=# header\n", "", nil},
	{`BEGIN { OUTPUTMODE="csv separator=,"; printf "%s", OUTPUTMODE }`, "", "csv", "", nil},
	{`BEGIN { OUTPUTMODE="csv separator=|"; printf "%s", OUTPUTMODE }`, "", "csv separator=|", "", nil},

	// Error handling when parsing INPUTMODE and OUTPUTMODE
	{`BEGIN { INPUTMODE="xyz" }`, "", "", `invalid input mode "xyz"`, nil},
	{`BEGIN { INPUTMODE="csv separator=foo" }`, "", "", `invalid CSV/TSV separator "foo"`, nil},
	{`BEGIN { INPUTMODE="csv comment=bar" }`, "", "", `invalid CSV/TSV comment character "bar"`, nil},
	{`BEGIN { INPUTMODE="csv header=x" }`, "", "", `invalid header value "x"`, nil},
	{`BEGIN { INPUTMODE="csv foo=bar" }`, "", "", `invalid input mode key "foo"`, nil},
	{`BEGIN { OUTPUTMODE="xyz" }`, "", "", `invalid output mode "xyz"`, nil},
	{`BEGIN { OUTPUTMODE="csv separator=foo" }`, "", "", `invalid CSV/TSV separator "foo"`, nil},
	{`BEGIN { OUTPUTMODE="csv foo=bar" }`, "", "", `invalid output mode key "foo"`, nil},

	// Other errors
	{`BEGIN { @"x" = "y" }`, "", "", "parse error at 1:14: assigning @ expression not supported", nil},
	{`BEGIN { x="a"; @x = "y" }`, "", "", "parse error at 1:19: assigning @ expression not supported", nil},
	{`BEGIN { @"x" += "y" }`, "", "", "parse error at 1:14: assigning @ expression not supported", nil},
	{`BEGIN { x="a"; @x += "y" }`, "", "", "parse error at 1:19: assigning @ expression not supported", nil},
}

func TestCSV(t *testing.T) {
	for _, test := range csvTests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}
		t.Run(testName, func(t *testing.T) {
			testGoAWK(t, test.src, test.in, test.out, test.err, nil, test.configure)
		})
	}
}

func TestCSVMultiRead(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		reads []string
		out   string
	}{{
		name:  "UnquotedHeader",
		src:   `BEGIN { INPUTMODE="csv header"; OFS="|" } { print $0, $1, $2 }`,
		reads: []string{"name,age\n", "Bob", ",42\n", "", "Jill,", "37", ""},
		out:   "Bob,42|Bob|42\nJill,37|Jill|37\n",
	}, {
		name:  "QuotedHeader",
		src:   `BEGIN { INPUTMODE="csv header"; OFS="|" } { print $0, $1, $2 }`,
		reads: []string{"name,age\n", "\"Bo", "b\"", ",42\n", "\"Ji\n", "ll\",", "37"},
		out:   "\"Bob\",42|Bob|42\n\"Ji\nll\",37|Ji\nll|37\n",
	}, {
		name:  "UnquotedNewline",
		src:   `BEGIN { INPUTMODE="csv header"; OFS="|" } { print $0, $1, $2 }`,
		reads: []string{"name,age\n", "Bob", ",42\n", "Jill,", "37", "\n"},
		out:   "Bob,42|Bob|42\nJill,37|Jill|37\n",
	}, {
		name:  "QuotedNewline",
		src:   `BEGIN { INPUTMODE="csv header"; OFS="|" } { print $0, $1, $2 }`,
		reads: []string{"name,age\n", "\"Bo", "b\"", ",42\n", "\"Ji\n", "ll\",", "37\n"},
		out:   "\"Bob\",42|Bob|42\n\"Ji\nll\",37|Ji\nll|37\n",
	}, {
		name:  "UnquotedNoHeader",
		src:   `BEGIN { INPUTMODE="csv"; OFS="|" } { print $0, $1, $2 }`,
		reads: []string{"Bob", ",42\n", "", "Jill,", "37", ""},
		out:   "Bob,42|Bob|42\nJill,37|Jill|37\n",
	}, {
		name:  "QuotedNoHeader",
		src:   `BEGIN { INPUTMODE="csv"; OFS="|" } { print $0, $1, $2 }`,
		reads: []string{"\"Bo", "b\"", ",42\n", "\"Ji\n", "ll\",", "37\n"},
		out:   "\"Bob\",42|Bob|42\n\"Ji\nll\",37|Ji\nll|37\n",
	}, {
		name:  "QuotedCRLF",
		src:   `BEGIN { INPUTMODE="csv" } { printf "%s|%s|%s", $0, $1, $2 }`,
		reads: []string{"\"Ji\r\n", "ll\",", "37"},
		out:   "\"Ji\nll\",37|Ji\nll|37",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prog, err := parser.ParseProgram([]byte(test.src), nil)
			if err != nil {
				t.Fatalf("error parsing program: %v", err)
			}
			outBuf := &concurrentBuffer{}
			config := &interp.Config{
				Stdin:  &sliceReader{reads: test.reads},
				Output: outBuf,
				Error:  outBuf,
			}
			status, err := interp.ExecProgram(prog, config)
			if err != nil {
				t.Fatalf("error executing program: %v", err)
			}
			out := outBuf.String()
			if runtime.GOOS == "windows" {
				out = normalizeNewlines(out)
			}
			if out != test.out {
				t.Fatalf("expected %q, got %q", test.out, out)
			}
			if status != 0 {
				t.Fatalf("expected status 0, got %d", status)
			}
		})
	}
}

type sliceReader struct {
	reads []string
}

func (r *sliceReader) Read(buf []byte) (int, error) {
	if len(r.reads) == 0 {
		return 0, io.EOF
	}
	n := copy(buf, r.reads[0])
	if n < len(r.reads[0]) {
		r.reads[0] = r.reads[0][:len(buf)]
	} else {
		r.reads = r.reads[1:]
	}
	return n, nil
}

func benchmarkProgram(b *testing.B, funcs map[string]interface{},
	input, expected, srcFormat string, args ...interface{},
) {
	b.StopTimer()
	src := fmt.Sprintf(srcFormat, args...)
	parserConfig := &parser.ParserConfig{
		Funcs: funcs,
	}
	prog, err := parser.ParseProgram([]byte(src), parserConfig)
	if err != nil {
		b.Fatalf("error parsing %s: %v", b.Name(), err)
	}
	outBuf := &bytes.Buffer{}
	config := &interp.Config{
		Stdin:  strings.NewReader(input),
		Output: outBuf,
		Error:  ioutil.Discard,
		Funcs:  funcs,
	}
	b.StartTimer()
	_, err = interp.ExecProgram(prog, config)
	b.StopTimer()
	if err != nil {
		b.Fatalf("error interpreting %s: %v", b.Name(), err)
	}
	if expected != "" {
		expected += "\n"
	}
	outStr := strings.Replace(outBuf.String(), "\r\n", "\n", -1)
	if outStr != expected {
		b.Fatalf("expected %q, got %q", expected, outStr)
	}
}

func BenchmarkGlobalVars(b *testing.B) {
	benchmarkProgram(b, nil, "", "a 1", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	x = 1; y = "a"; t = x; x = y; y = t
  	x = 1; y = "a"; t = x; x = y; y = t
  	x = 1; y = "a"; t = x; x = y; y = t
  	x = 1; y = "a"; t = x; x = y; y = t
  	x = 1; y = "a"; t = x; x = y; y = t
  }
  print x, y
}
`, b.N)
}

func BenchmarkLocalVars(b *testing.B) {
	benchmarkProgram(b, nil, "", "b 2", `
function f(i, x, y, t) {
  for (i = 0; i < %d; i++) {
  	x = 2; y = "b"; t = x; x = y; y = t
  	x = 2; y = "b"; t = x; x = y; y = t
  	x = 2; y = "b"; t = x; x = y; y = t
  	x = 2; y = "b"; t = x; x = y; y = t
  	x = 2; y = "b"; t = x; x = y; y = t
  }
  print x, y
}

BEGIN {
  f()
}
`, b.N)
}

func BenchmarkIncrDecr(b *testing.B) {
	benchmarkProgram(b, nil, "", "0 10", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	x++; x++; x++; x++; x++; x++; x++; x++; x++; x++
  	y = x
  	x--; x--; x--; x--; x--; x--; x--; x--; x--; x--
  }
  print x, y
}
`, b.N)
}

func BenchmarkSimpleBuiltins(b *testing.B) {
	benchmarkProgram(b, nil, "", "", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	sin(0); cos(0); exp(0); log(1); sqrt(2); int("x");
  	sin(0); cos(0); exp(0); log(1); sqrt(2); int("x");
  	sin(0); cos(0); exp(0); log(1); sqrt(2); int("x");
  	sin(0); cos(0); exp(0); log(1); sqrt(2); int("x");
  	sin(0); cos(0); exp(0); log(1); sqrt(2); int("x");
  }
}
`, b.N)
}

func BenchmarkBuiltinMatch(b *testing.B) {
	benchmarkProgram(b, nil, "", "21", `
BEGIN {
  s = "The quick brown fox jumps over the lazy dog"
  for (i = 0; i < %d; i++) {
  	match(s, /j[a-z]+p/); match(s, /j[a-z]+p/)
  	match(s, /j[a-z]+p/); match(s, /j[a-z]+p/)
  	match(s, /j[a-z]+p/); match(s, /j[a-z]+p/)
  	match(s, /j[a-z]+p/); match(s, /j[a-z]+p/)
  	match(s, /j[a-z]+p/); x = match(s, /j[a-z]+p/)
  }
  print x
}
`, b.N)
}

func BenchmarkBuiltinLength(b *testing.B) {
	benchmarkProgram(b, nil, "", "134", `
BEGIN {
  s = "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog."
  for (i = 0; i < %d; i++) {
  	length(s); length(s); length(s); length(s); length(s);
  	length(s); length(s); length(s); length(s); length(s);
  	length(s); length(s); length(s); length(s); length(s);
  	length(s); length(s); length(s); length(s); length(s);
  	length(s); length(s); length(s); length(s); x = length(s);
  }
  print x
}
`, b.N)
}

func BenchmarkBuiltinIndex(b *testing.B) {
	benchmarkProgram(b, nil, "", "134", `
BEGIN {
  s = "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog!?!"
  for (i = 0; i < %d; i++) {
  	index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!")
  	index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!")
  	index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!")
  	index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!")
  	index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); index(s, "!?!"); x = index(s, "!?!")
  }
  print x
}
`, b.N)
}

func BenchmarkBuiltinSubstr(b *testing.B) {
	benchmarkProgram(b, nil, "", " brown fox", `
BEGIN {
  s = "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog!?!"
  for (i = 0; i < %d; i++) {
  	substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10)
  	substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10)
  	substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10)
  	substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10)
  	substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); substr(s, 100, 10); x = substr(s, 100, 10)
  }
  print x
}
`, b.N)
}

func BenchmarkBuiltinSplitSpace(b *testing.B) {
	benchmarkProgram(b, nil, "", "27", `
BEGIN {
  s = "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog!?!"
  for (i = 0; i < %d; i++) {
  	split(s, a, " "); split(s, a, " "); split(s, a, " ")
  	split(s, a, " "); split(s, a, " "); split(s, a, " ")
  	split(s, a, " "); split(s, a, " "); split(s, a, " ")
  	split(s, a, " "); split(s, a, " "); split(s, a, " ")
  	split(s, a, " "); split(s, a, " "); split(s, a, " ")
  }
  for (k in a) n++
  print n
}
`, b.N)
}

func BenchmarkBuiltinSplitRegex(b *testing.B) {
	benchmarkProgram(b, nil, "", "22", `
BEGIN {
  s = "a fox ab fax abc fix a fox ab fax abc fix a fox ab fax abc fix a fox ab fax abc fix a fox ab fax abc fix a fox ab fax abc fix a fox ab fax abc fix"
  for (i = 0; i < %d; i++) {
  	split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x")
  	split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x")
  	split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x")
  	split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x")
  	split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x"); split(s, a, "f[a-z]x")
  }
  for (k in a) n++
  print n
}
`, b.N)
}

func BenchmarkBuiltinSub(b *testing.B) {
	benchmarkProgram(b, nil, "", "1 164", `
BEGIN {
  for (i = 0; i < %d; i++) {
    s = "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog."
  	sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s)
  	sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s)
  	sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s)
  	sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s)
  	sub(/f[a-z]x/, "foxes", s); sub(/f[a-z]x/, "foxes", s); x = sub(/f[a-z]x/, "foxes", s)
  }
  print x, length(s)
}
`, b.N)
}

func BenchmarkBuiltinSubAmpersand(b *testing.B) {
	benchmarkProgram(b, nil, "", "1 164", `
BEGIN {
  for (i = 0; i < %d; i++) {
    s = "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog."
  	sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s)
  	sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s)
  	sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s)
  	sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s)
  	sub(/f[a-z]x/, "&es", s); sub(/f[a-z]x/, "&es", s); x = sub(/f[a-z]x/, "&es", s)
  }
  print x, length(s)
}
`, b.N)
}

func BenchmarkBuiltinGsub(b *testing.B) {
	benchmarkProgram(b, nil, "", "3 224", `
BEGIN {
  for (i = 0; i < %d; i++) {
    s = "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog."
  	gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s)
  	gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s)
  	gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s)
  	gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s)
  	gsub(/f[a-z]x/, "foxes", s); gsub(/f[a-z]x/, "foxes", s); x = gsub(/f[a-z]x/, "foxes", s)
  }
  print x, length(s)
}
`, b.N)
}

func BenchmarkBuiltinGsubAmpersand(b *testing.B) {
	benchmarkProgram(b, nil, "", "3 224", `
BEGIN {
  for (i = 0; i < %d; i++) {
    s = "The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog."
  	gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s)
  	gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s)
  	gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s)
  	gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s)
  	gsub(/f[a-z]x/, "&es", s); gsub(/f[a-z]x/, "&es", s); x = gsub(/f[a-z]x/, "&es", s)
  }
  print x, length(s)
}
`, b.N)
}

func BenchmarkBuiltinSprintf(b *testing.B) {
	benchmarkProgram(b, nil, "", "A 123   foo 3.14", `
BEGIN {
  x = "foo"
  y = 3.14159
  for (i = 0; i < %d; i++) {
  	sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y)
  	sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y)
  	sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y)
  	sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y)
  	sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y); s = sprintf("%%c %%d %%5s %%.3g", 65, 123, x, y)
  }
  print s
}
`, b.N)
}

func BenchmarkRecursiveFunc(b *testing.B) {
	benchmarkProgram(b, nil, "", "55", `
function fib(n) {
  if (n <= 2) {
    return 1
  }
  return fib(n-1) + fib(n-2)
}

BEGIN {
  for (i = 0; i < %d; i++) {
    res = fib(10)
  }
  print res
}
`, b.N)
}

func BenchmarkFuncCall(b *testing.B) {
	benchmarkProgram(b, nil, "", "75", `
function add(a, b) {
  return a + b
}

BEGIN {
  for (i = 0; i < %d; i++) {
    sum = add(0, add(1, add(2, add(3, add(4, 5)))))
    sum = add(sum, add(1, add(2, add(3, add(4, 5)))))
    sum = add(sum, add(1, add(2, add(3, add(4, 5)))))
    sum = add(sum, add(1, add(2, add(3, add(4, 5)))))
    sum = add(sum, add(1, add(2, add(3, add(4, 5)))))
  }
  print sum
}
`, b.N)
}

func BenchmarkNativeFunc(b *testing.B) {
	funcs := map[string]interface{}{
		"add": func(a, b float64) float64 { return a + b },
	}
	benchmarkProgram(b, funcs, "", "75", `
BEGIN {
  for (i = 0; i < %d; i++) {
    sum = add(0, add(1, add(2, add(3, add(4, 5)))))
    sum = add(sum, add(1, add(2, add(3, add(4, 5)))))
    sum = add(sum, add(1, add(2, add(3, add(4, 5)))))
    sum = add(sum, add(1, add(2, add(3, add(4, 5)))))
    sum = add(sum, add(1, add(2, add(3, add(4, 5)))))
  }
  print sum
}
`, b.N)
}

func BenchmarkForLoop(b *testing.B) {
	benchmarkProgram(b, nil, "", "", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	for (j = 0; j < 100; j++);
  }
}
`, b.N)
}

func BenchmarkForInLoop(b *testing.B) {
	benchmarkProgram(b, nil, "", "", `
BEGIN {
  for (j = 0; j < 100; j++) {
  	a[j] = j
  }
  for (i = 0; i < %d; i++) {
    for (k in a);
  }
}
`, b.N)
}

func BenchmarkIfStatement(b *testing.B) {
	benchmarkProgram(b, nil, "", "0", `
BEGIN {
  c = 1
  d = 0
  for (i = 0; i < %d; i++) {
  	if (c) { x = 1 } else { x = 0 }
  	if (c) { x = 1 } else { x = 0 }
  	if (c) { x = 1 } else { x = 0 }
  	if (d) { x = 1 } else { x = 0 }
  	if (d) { x = 1 } else { x = 0 }
  	if (d) { x = 1 } else { x = 0 }
  }
  print x
}
`, b.N)
}

func BenchmarkCondExpr(b *testing.B) {
	benchmarkProgram(b, nil, "", "0", `
BEGIN {
  c = 1
  d = 0
  for (i = 0; i < %d; i++) {
  	x = c ? 1 : 0
  	x = c ? 1 : 0
  	x = c ? 1 : 0
  	x = d ? 1 : 0
  	x = d ? 1 : 0
  	x = d ? 1 : 0
  }
  print x
}
`, b.N)
}

func BenchmarkSimplePattern(b *testing.B) {
	b.StopTimer()
	inputLines := []string{}
	expectedLines := []string{}
	for i := 0; i < b.N; i++ {
		if i != 0 && i%2 == 0 {
			line := fmt.Sprintf("%d", i)
			inputLines = append(inputLines, line)
			expectedLines = append(expectedLines, line)
		} else {
			inputLines = append(inputLines, "")
		}
	}
	input := strings.Join(inputLines, "\n")
	expected := strings.Join(expectedLines, "\n")
	benchmarkProgram(b, nil, input, expected, "$0")
}

func BenchmarkGetField(b *testing.B) {
	b.StopTimer()
	inputLines := []string{}
	expectedLines := []string{}
	for i := 1; i < b.N+1; i++ {
		inputLines = append(inputLines, fmt.Sprintf("%d %d %d", i, i*2, i*3))
		expectedLines = append(expectedLines, fmt.Sprintf("%d %d", i, i*3))
	}
	input := strings.Join(inputLines, "\n")
	expected := strings.Join(expectedLines, "\n")
	benchmarkProgram(b, nil, input, expected, "{ print $1, $3 }")
}

func BenchmarkSetField(b *testing.B) {
	benchmarkProgram(b, nil, "1 2 3", "one 2 three", `
{
  for (i = 0; i < %d; i++) {
    $1 = "one"; $3 = "three"
    $1 = "one"; $3 = "three"
    $1 = "one"; $3 = "three"
    $1 = "one"; $3 = "three"
    $1 = "one"; $3 = "three"
  }
}
END {
	print $0
}
`, b.N)
}

func BenchmarkRegexMatch(b *testing.B) {
	benchmarkProgram(b, nil, "", "1", `
BEGIN {
  s = "The quick brown fox jumps over the lazy dog"
  for (i = 0; i < %d; i++) {
  	x = s ~ /j[a-z]+p/
  	x = s ~ /j[a-z]+p/
  	x = s ~ /j[a-z]+p/
  	x = s ~ /j[a-z]+p/
  	x = s ~ /j[a-z]+p/
  }
  print x
}
`, b.N)
}

func BenchmarkBinaryOperators(b *testing.B) {
	benchmarkProgram(b, nil, "", "5.0293", `
BEGIN {
  for (i = 0; i < %d; i++) {
    res = (1+2*3/4^5) + (1+2*3/4^5) + (1+2*3/4^5) + (1+2*3/4^5) + (1+2*3/4^5)
  }
  print res
}
`, b.N)
}

func BenchmarkConcatTwo(b *testing.B) {
	b.StopTimer()
	benchmarkProgram(b, nil, "", "20", `
BEGIN {
  x = "0123456789"
  for (i = 0; i < %d; i++) {
    y = x x
  }
  print length(y)
}
`, b.N)
}

func BenchmarkConcatSmall(b *testing.B) {
	b.StopTimer()
	benchmarkProgram(b, nil, "", "100", `
BEGIN {
  x = "0123456789"
  for (i = 0; i < %d; i++) {
  	y = x x x x x x x x x x
  }
  print length(y)
}
`, b.N)
}

func BenchmarkConcatLarge(b *testing.B) {
	b.StopTimer()
	benchmarkProgram(b, nil, "", "1000000", `
BEGIN {
  x = "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"
  for (i = 0; i < %d; i++) {
  	y = x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x \
  	    x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x
    z = y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y \
        y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y y
  }
  print length(z)
}
`, b.N)
}

func BenchmarkComparisons(b *testing.B) {
	b.StopTimer()
	benchmarkProgram(b, nil, "", "1", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	x = ((((((1 < 2) <= 3) > 4) >= 5) == 6) != 7)
  	x = ((((((1 < 2) <= 3) > 4) >= 5) == 6) != 7)
  	x = ((((((1 < 2) <= 3) > 4) >= 5) == 6) != 7)
  }
  print x
}
`, b.N)
}

func BenchmarkArrayOperations(b *testing.B) {
	b.StopTimer()
	benchmarkProgram(b, nil, "", "243", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	a[0] = 1
  	a[0] = a[0] + a[0] + a[0]
  	a[0] = a[0] + a[0] + a[0]
  	a[0] = a[0] + a[0] + a[0]
  	a[0] = a[0] + a[0] + a[0]
  	a[0] = a[0] + a[0] + a[0]
  }
  print a[0]
}
`, b.N)
}

func BenchmarkAssign(b *testing.B) {
	b.StopTimer()
	benchmarkProgram(b, nil, "", "0 1 2 3 4", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	v=0; w=1; x=2; y=3; z=4
  	v=0; w=1; x=2; y=3; z=4
  	v=0; w=1; x=2; y=3; z=4
  	v=0; w=1; x=2; y=3; z=4
  	v=0; w=1; x=2; y=3; z=4
  }
  print v, w, x, y, z
}
`, b.N)
}

func BenchmarkAugAssign(b *testing.B) {
	b.StopTimer()
	benchmarkProgram(b, nil, "", "5 -9 729 32 3.0536 2", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	a = 0; b = 1; c = 3; d = 1024; e = 2; f = 14
  	a += 1; b -= 2; c *= 3; d /= 2; e ^= 1.1; f %%= 6
  	a += 1; b -= 2; c *= 3; d /= 2; e ^= 1.1; f %%= 6
  	a += 1; b -= 2; c *= 3; d /= 2; e ^= 1.1; f %%= 6
  	a += 1; b -= 2; c *= 3; d /= 2; e ^= 1.1; f %%= 6
  	a += 1; b -= 2; c *= 3; d /= 2; e ^= 1.1; f %%= 6
  }
  print a, b, c, d, e, f
}
`, b.N)
}

func BenchmarkPrint(b *testing.B) {
	b.StopTimer()
	src := fmt.Sprintf(`
BEGIN {
  for (i = 0; i < %d; i++) {
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
    print i, "foo", i, "bar"
  }
}
`, b.N)

	prog, err := parser.ParseProgram([]byte(src), nil)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}
	b.StartTimer()
	_, err = interp.ExecProgram(prog, &interp.Config{
		Output:  ioutil.Discard,
		Environ: []string{},
	})
	b.StopTimer()
	if err != nil {
		b.Fatalf("execute error: %v", err)
	}
}

func BenchmarkPrintf(b *testing.B) {
	b.StopTimer()
	src := fmt.Sprintf(`
BEGIN {
  for (i = 0; i < %d; i++) {
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
    printf "%%d %%s %%d %%s\n", i, "foo", i, "bar"
  }
}
`, b.N)

	prog, err := parser.ParseProgram([]byte(src), nil)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}
	b.StartTimer()
	_, err = interp.ExecProgram(prog, &interp.Config{
		Output:  ioutil.Discard,
		Environ: []string{},
	})
	b.StopTimer()
	if err != nil {
		b.Fatalf("execute error: %v", err)
	}
}

func BenchmarkRepeatExecProgram(b *testing.B) {
	prog, err := parser.ParseProgram([]byte(`BEGIN {}`), nil)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}
	config := interp.Config{
		Output:  ioutil.Discard,
		Environ: []string{},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := interp.ExecProgram(prog, &config)
		if err != nil {
			b.Fatalf("execute error: %v", err)
		}
	}
}

func BenchmarkRepeatNew(b *testing.B) {
	prog, err := parser.ParseProgram([]byte(`BEGIN {}`), nil)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}
	p, err := interp.New(prog)
	if err != nil {
		b.Fatalf("interp.New error: %v", err)
	}
	config := interp.Config{
		Output:  ioutil.Discard,
		Environ: []string{},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Execute(&config)
		if err != nil {
			b.Fatalf("execute error: %v", err)
		}
	}
}

func BenchmarkRepeatIOExecProgram(b *testing.B) {
	prog, err := parser.ParseProgram([]byte(`{ for (i=1; i<=NF; i++) print $i }`), nil)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}
	inputStr := "foo bar\nbazz\n"
	input := strings.NewReader(inputStr)
	var output bytes.Buffer
	config := interp.Config{
		Stdin:   input,
		Output:  &output,
		Environ: []string{},
	}
	expected := "foo\nbar\nbazz\n"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input.Reset(inputStr)
		output.Reset()
		_, err := interp.ExecProgram(prog, &config)
		if err != nil {
			b.Fatalf("execute error: %v", err)
		}
		if output.String() != expected {
			b.Fatalf("expected %q, got %q", expected, output.String())
		}
	}
}

func BenchmarkRepeatIONew(b *testing.B) {
	prog, err := parser.ParseProgram([]byte(`{ for (i=1; i<=NF; i++) print $i }`), nil)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}
	p, err := interp.New(prog)
	if err != nil {
		b.Fatalf("interp.New error: %v", err)
	}
	inputStr := "foo bar\nbazz\n"
	input := strings.NewReader(inputStr)
	var output bytes.Buffer
	config := interp.Config{
		Stdin:   input,
		Output:  &output,
		Environ: []string{},
	}
	expected := "foo\nbar\nbazz\n"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input.Reset(inputStr)
		output.Reset()
		_, err := p.Execute(&config)
		if err != nil {
			b.Fatalf("execute error: %v", err)
		}
		if output.String() != expected {
			b.Fatalf("expected %q, got %q", expected, output.String())
		}
	}
}

func BenchmarkCSVInputGoAWK(b *testing.B) {
	b.StopTimer()
	s := 0
	var inputLines []string
	for i := 0; i < b.N; i++ {
		s += i
		inputLines = append(inputLines, fmt.Sprintf(`%d,foo,Bob Smith,"foo,bar,baz",email@example.com`, i))
	}
	input := strings.Join(inputLines, "\n")
	expected := fmt.Sprintf("%d", s)
	src := `BEGIN { INPUTMODE="csv" } { s += $1 } END { print s }`
	benchmarkProgram(b, nil, input, expected, src)
}

func BenchmarkCSVInputReader(b *testing.B) {
	b.StopTimer()
	s := 0
	var inputLines []string
	for i := 0; i < b.N; i++ {
		s += i
		inputLines = append(inputLines, fmt.Sprintf(`%d,foo,Bob Smith,"foo,bar,baz",email@example.com`, i))
	}
	input := strings.Join(inputLines, "\n")
	reader := csv.NewReader(strings.NewReader(input))
	total := 0
	b.StartTimer()
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			b.Fatalf("read error: %v", err)
		}
		v, _ := strconv.Atoi(record[0])
		total += v
	}
	if s != total {
		b.Fatalf("expected %d, got %d", s, total)
	}
}

func BenchmarkCSVOutputGoAWK(b *testing.B) {
	b.StopTimer()
	var expectedLines []string
	for i := 0; i < b.N; i++ {
		expectedLines = append(expectedLines, fmt.Sprintf(`%d,foo,Bob Smith,"foo,bar,baz",email@example.com`, i))
	}
	expected := strings.Join(expectedLines, "\n")
	benchmarkProgram(b, nil, "", expected, `
BEGIN {
	OUTPUTMODE = "csv";
	for (i=0; i<%d; i++)
		print i, "foo", "Bob Smith", "foo,bar,baz", "email@example.com"
}
`, b.N)
}

func BenchmarkCSVOutputWriter(b *testing.B) {
	b.StopTimer()
	var expectedLines []string
	for i := 0; i < b.N; i++ {
		expectedLines = append(expectedLines, fmt.Sprintf(`%d,foo,Bob Smith,"foo,bar,baz",email@example.com`, i))
	}
	expected := strings.Join(expectedLines, "\n") + "\n"
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err := writer.Write([]string{strconv.Itoa(i), "foo", "Bob Smith", "foo,bar,baz", "email@example.com"})
		if err != nil {
			b.Fatalf("write error: %v", err)
		}
	}
	writer.Flush()
	b.StopTimer()
	output := buf.String()
	if output != expected {
		b.Fatalf("expected %q, got %q\n", expected, output)
	}
}

func normalizeNewlines(s string) string {
	return strings.Replace(s, "\r\n", "\n", -1)
}
