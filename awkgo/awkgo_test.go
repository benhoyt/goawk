package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/parser"
)

type interpTest struct {
	src string
	in  string
	out string
	err string
}

// Note: a lot of these are really parser tests too.
var interpTests = []interpTest{
	// BEGIN and END work correctly
	{`BEGIN { print "b" }`, "", "b\n", ""},
	{`BEGIN { print "b" }`, "foo", "b\n", ""},
	{`END { print "e" }`, "", "e\n", ""},
	{`END { print "e" }`, "foo", "e\n", ""},
	{`BEGIN { print "b"} END { print "e" }`, "", "b\ne\n", ""},
	{`BEGIN { print "b"} END { print "e" }`, "foo", "b\ne\n", ""},
	{`BEGIN { print "b"} $0 { print NR } END { print "e" }`, "foo", "b\n1\ne\n", ""},
	{`BEGIN { printf "x" }; BEGIN { printf "y" }`, "", "xy", ""},

	// Patterns
	{`$0`, "foo\n\nbar", "foo\nbar\n", ""},
	{`{ print $0 }`, "foo\n\nbar", "foo\n\nbar\n", ""},
	{`$1=="foo"`, "foo\n\nbar", "foo\n", ""},
	{`$1==42`, "foo\n42\nbar", "42\n", ""},
	{`$1=="42"`, "foo\n42\nbar", "42\n", ""},
	{`/foo/`, "foo\nx\nfood\nxfooz\nbar", "foo\nfood\nxfooz\n", ""},
	{`NR==2, NR==4`, "1\n2\n3\n4\n5\n6\n", "2\n3\n4\n", ""},
	{`
NR==2, NR==4 { print $0 }
NR==3, NR==5 { print NR }
`, "a\nb\nc\nd\ne\nf\ng", "b\nc\n3\nd\n4\n5\n", ""},

	// print and printf statements
	{`BEGIN { print "x", "y" }`, "", "x y\n", ""},
	{`BEGIN { print OFS; OFS = ","; print "x", "y" }`, "", " \nx,y\n", ""},
	{`BEGIN { print ORS; ORS = "."; print "x", "y" }`, "", "\n\nx y.", ""},
	{`BEGIN { print ORS; ORS = ""; print "x", "y" }`, "", "\n\nx y", ""},
	{`{ print; print }`, "foo", "foo\nfoo\n", ""},
	{`BEGIN { print; print }`, "", "\n\n", ""},
	{`BEGIN { printf "%% %d %x %c %f %s", 42, 42, 42, 42, 42 }`, "", "% 42 2a * 42.000000 42", ""},
	{`BEGIN { printf "%3d", 42 }`, "", " 42", ""},
	{`BEGIN { printf "%3s", "x" }`, "", "  x", ""},
	// {`BEGIN { printf "%.1g", 42 }`, "", "4e+01", ""}, // TODO: comment out for now, for some reason gives "4e+001" on Windows
	{`BEGIN { printf "%d", 12, 34 }`, "", "12", ""},
	{`BEGIN { printf "%d" }`, "", "", `not enough arguments (0) for format string "%d"`},
	// Our %c handling is mostly like awk's, except for multiples
	// 256, where awk is weird and we're like mawk
	{`BEGIN { printf "%c", 0 }`, "", "\x00", ""},
	{`BEGIN { printf "%c", 127 }`, "", "\x7f", ""},
	{`BEGIN { printf "%c", 128 }  # !gawk`, "", "\x80", ""},
	{`BEGIN { printf "%c", 255 }  # !gawk`, "", "\xff", ""},
	{`BEGIN { printf "%c", 256 }  # !awk !gawk`, "", "\x00", ""},
	{`BEGIN { printf "%c", "xyz" }`, "", "x", ""},
	{`BEGIN { printf "%c", "" }  # !awk`, "", "\x00", ""},
	{`BEGIN { printf }  # !awk - doesn't error on this`, "", "", "parse error at 1:16: expected printf args, got none"},
	{`BEGIN { printf("%%%dd", 4) }`, "", "%4d", ""},

	// if and loop statements
	{`BEGIN { if (1) print "t"; }`, "", "t\n", ""},
	{`BEGIN { if (0) print "t"; }`, "", "", ""},
	{`BEGIN { if (1) print "t"; else print "f" }`, "", "t\n", ""},
	{`BEGIN { if (0) print "t"; else print "f" }`, "", "f\n", ""},
	{`BEGIN { for (;;) { print "x"; break } }`, "", "x\n", ""},
	{`BEGIN { for (;;) { printf "%d ", i; i++; if (i>2) break; } }`, "", "0 1 2 ", ""},
	{`BEGIN { for (i=5; ; ) { printf "%d ", i; i++; if (i>8) break; } }`, "", "5 6 7 8 ", ""},
	{`BEGIN { for (i=5; ; i++) { printf "%d ", i; if (i>8) break; } }`, "", "5 6 7 8 9 ", ""},
	{`BEGIN { for (i=5; i<8; i++) { printf "%d ", i } }`, "", "5 6 7 ", ""},
	{`BEGIN { for (i=0; i<10; i++) { if (i < 5) continue; printf "%d ", i } }`, "", "5 6 7 8 9 ", ""},
	{`BEGIN { a[1]=1; a[2]=1; for (k in a) { s++; break } print s }`, "", "1\n", ""},
	{`BEGIN { a[1]=1; a[2]=1; a[3]=1; for (k in a) { if (k==2) continue; s++ } print s }`, "", "2\n", ""},
	{`BEGIN { while (i<3) { i++; s++; break } print s }`, "", "1\n", ""},
	{`BEGIN { while (i<3) { i++; if (i==2) continue; s++ } print s }`, "", "2\n", ""},
	{`BEGIN { do { i++; s++; break } while (i<3); print s }`, "", "1\n", ""},
	{`BEGIN { do { i++; if (i==2) continue; s++ } while (i<3); print s }`, "", "2\n", ""},
	{`BEGIN { a["x"] = 3; a["y"] = 4; for (k in a) x += a[k]; print x }`, "", "7\n", ""},
	{`BEGIN { while (i < 5) { print i; i++ } }`, "", "\n1\n2\n3\n4\n", ""},
	{`BEGIN { do { print i; i++ } while (i < 5) }`, "", "\n1\n2\n3\n4\n", ""},
	{`BEGIN { for (i=0; i<10; i++); printf "x" }`, "", "x", ""},
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
`, "", "0 0\n", ""},
	{`
BEGIN {
	for (i = 0; i < 1; i++) {
		for (j = 0; j < 1; j++) {
			print i, j
		}
		continue
	}
}
`, "", "0 0\n", ""},

	// next statement
	{`{ if (NR==2) next; print }`, "a\nb\nc", "a\nc\n", ""},
	{`BEGIN { next }`, "", "", "parse error at 1:9: next can't be in BEGIN or END"},
	{`END { next }`, "", "", "parse error at 1:7: next can't be in BEGIN or END"},

	// Arrays, "in", and delete
	{`BEGIN { a["x"] = 3; print "x" in a, "y" in a }`, "", "1 0\n", ""},
	{`BEGIN { a["x"] = 3; a["y"] = 4; delete a["x"]; for (k in a) print k, a[k] }`, "", "y 4\n", ""},
	{`BEGIN { a["x"] = 3; a["y"] = 4; for (k in a) delete a[k]; for (k in a) print k, a[k] }`, "", "", ""},
	{`BEGIN { a["x"]; "y" in a; for (k in a) print k, a[k] }`, "", "x \n", ""},
	{`BEGIN { a[] }`, "", "", "parse error at 1:11: expected expression instead of ]"},
	{`BEGIN { delete a[] }`, "", "", "parse error at 1:18: expected expression instead of ]"},
	{`BEGIN { a["x"] = 3; a["y"] = 4; delete a; for (k in a) print k, a[k] }`, "", "", ""},

	// Unary expressions: ! + -
	{`BEGIN { print !42, !1, !0, !!42, !!1, !!0 }`, "", "0 0 1 1 1 0\n", ""},
	{`BEGIN { print !42, !1, !0, !!42, !!1, !!0 }`, "", "0 0 1 1 1 0\n", ""},
	{`BEGIN { print +4, +"3", +0, +-3, -3, - -4, -"3" }`, "", "4 3 0 -3 -3 4 -3\n", ""},
	// TODO: {`BEGIN { $0="0"; print !$0 }`, "", "0\n", ""},
	{`BEGIN { $0="1"; print !$0 }`, "", "0\n", ""},
	{`{ print !$0 }`, "0\n", "1\n", ""},
	{`{ print !$0 }`, "1\n", "0\n", ""},
	{`!seen[$0]++`, "1\n2\n3\n2\n3\n3\n", "1\n2\n3\n", ""},
	{`!seen[$0]--`, "1\n2\n3\n2\n3\n3\n", "1\n2\n3\n", ""},

	// Comparison expressions: == != < <= > >=
	{`BEGIN { print (1==1, 1==0, "1"==1, "1"==1.0) }`, "", "1 0 1 1\n", ""},
	{`{ print ($0=="1", $0==1) }`, "1\n1.0\n+1", "1 1\n0 1\n0 1\n", ""},
	{`{ print ($1=="1", $1==1) }`, "1\n1.0\n+1", "1 1\n0 1\n0 1\n", ""},
	{`BEGIN { print (1!=1, 1!=0, "1"!=1, "1"!=1.0) }`, "", "0 1 0 0\n", ""},
	{`{ print ($0!="1", $0!=1) }`, "1\n1.0\n+1", "0 0\n1 0\n1 0\n", ""},
	{`{ print ($1!="1", $1!=1) }`, "1\n1.0\n+1", "0 0\n1 0\n1 0\n", ""},
	{`BEGIN { print (0<1, 1<1, 2<1, "12"<"2") }`, "", "1 0 0 1\n", ""},
	{`{ print ($1<2) }`, "1\n1.0\n+1", "1\n1\n1\n", ""},
	{`BEGIN { print (0<=1, 1<=1, 2<=1, "12"<="2") }`, "", "1 1 0 1\n", ""},
	{`{ print ($1<=2) }`, "1\n1.0\n+1", "1\n1\n1\n", ""},
	{`BEGIN { print (0>1, 1>1, 2>1, "12">"2") }`, "", "0 0 1 0\n", ""},
	{`{ print ($1>2) }`, "1\n1.0\n+1", "0\n0\n0\n", ""},
	{`BEGIN { print (0>=1, 1>=1, 2>=1, "12">="2") }`, "", "0 1 1 0\n", ""},
	{`{ print ($1>=2) }`, "1\n1.0\n+1", "0\n0\n0\n", ""},

	// Short-circuit && and || operators
	{`
function t() { print "t"; return 1 }
function f() { print "f"; return 0 }
BEGIN {
	print f() && f()
	print f() && t()
	print t() && f()
	print t() && t()
}
`, "", "f\n0\nf\n0\nt\nf\n0\nt\nt\n1\n", ""},
	{`
function t() { print "t"; return 1 }
function f() { print "f"; return 0 }
BEGIN {
	print f() || f()
	print f() || t()
	print t() || f()
	print t() || t()
}
`, "", "f\nf\n0\nf\nt\n1\nt\n1\nt\n1\n", ""},

	// Other binary expressions: + - * ^ / % CONCAT ~ !~
	{`BEGIN { print 1+2, 1+2+3, 1+-2, -1+2, "1"+"2", 3+.14 }`, "", "3 6 -1 1 3 3.14\n", ""},
	{`BEGIN { print 1-2, 1-2-3, 1-+2, -1-2, "1"-"2", 3-.14 }`, "", "-1 -4 -1 -3 -1 2.86\n", ""},
	{`BEGIN { print 2*3, 2*3*4, 2*-3, -2*3, "2"*"3", 3*.14 }`, "", "6 24 -6 -6 6 0.42\n", ""},
	{`BEGIN { print 2/3, 2/3/4, 2/-3, -2/3, "2"/"3", 3/.14 }`, "", "0.666667 0.166667 -0.666667 -0.666667 0.666667 21.4286\n", ""},
	{`BEGIN { print 2%3, 2%3%4, 2%-3, -2%3, "2"%"3", 3%.14 }`, "", "2 2 2 -2 2 0.06\n", ""},
	{`BEGIN { print 2^3, 2^3^3, 2^-3, -2^3, "2"^"3", 3^.14 }`, "", "8 134217728 0.125 -8 8 1.16626\n", ""},
	{`BEGIN { print 1 2, "x" "yz", 1+2 3+4 }`, "", "12 xyz 37\n", ""},
	{`BEGIN { print "food"~/oo/, "food"~/[oO]+d/, "food"~"f", "food"~"F", "food"~0 }`, "", "1 1 1 0 0\n", ""},
	{`BEGIN { print "food"!~/oo/, "food"!~/[oO]+d/, "food"!~"f", "food"!~"F", "food"!~0 }`, "", "0 0 0 1 1\n", ""},
	{`BEGIN { print 1+2*3/4^5%6 7, (1+2)*3/4^5%6 "7" }`, "", "1.005867 0.008789067\n", ""},

	// Number, string, and regex expressions
	{`BEGIN { print 1, 1., .1, 1e0, -1, 1e }`, "", "1 1 0.1 1 -1 1\n", ""},
	{`BEGIN { print '\"' '\'' 'xy' "z" "'" '\"' }`, "", "\"'xyz'\"\n", ""}, // Check support for single-quoted strings
	{`{ print /foo/ }`, "food\nfoo\nxfooz\nbar\n", "1\n1\n1\n0\n", ""},
	{`/[a-/`, "foo", "", "parse error at 1:1: error parsing regexp: missing closing ]: `[a-`"},
	{`BEGIN { print "-12"+0, "+12"+0, " \t\r\n7foo"+0, ".5"+0, "5."+0, "+."+0 }`, "", "-12 12 7 0.5 5 0\n", ""},
	{`BEGIN { print "1e3"+0, "1.2e-1"+0, "1e+1"+0, "1e"+0, "1e+"+0 }`, "", "1000 0.12 10 1 1\n", ""},
	{`BEGIN { print -(11102200000000000000000000000000000000 1040000) }  # !gawk - gawk supports big numbers`,
		"", "-inf\n", ""},
	{`BEGIN { print atan2(0, 8020020000000e20G-0)}`, "", "0\n", ""},
	{`BEGIN { print 1e1000, -1e1000 }  # !gawk`, "", "inf -inf\n", ""},
	{`BEGIN { printf "\x0.\x00.\x0A\x10\xff\xFF\x41" }  # !awk`, "", "\x00.\x00.\n\x10\xff\xffA", ""},
	{`BEGIN { printf "\x1.\x01.\x0A\x10\xff\xFF\x41" }`, "", "\x01.\x01.\n\x10\xff\xffA", ""},
	{`BEGIN { printf "\0\78\7\77\777\0 \141 " }  # !awk`, "", "\x00\a8\a?\xff\x00 a ", ""},
	{`BEGIN { printf "\1\78\7\77\777\1 \141 " }`, "", "\x01\a8\a?\xff\x01 a ", ""},

	// Conditional ?: expression
	{`{ print /x/?"t":"f" }`, "x\ny\nxx\nz\n", "t\nf\nt\nf\n", ""},
	{`BEGIN { print 1?2?3:4:5, 1?0?3:4:5, 0?2?3:4:5 }`, "", "3 4 5\n", ""},
	// TODO: {`BEGIN { $0="0"; print ($0?1:0) }`, "", "1\n", ""},
	{`{ print $0?1:0 }`, "0\n", "0\n", ""},
	{`{ print $0?1:0 }`, "1\n", "1\n", ""},
	{`BEGIN { $0="1"; print ($0?1:0) }`, "", "1\n", ""},
	{`BEGIN { print 0?1:0, 1?1:0, ""?1:0, "0"?1:0, "1"?1:0, x?1:0 }`, "", "0 1 0 1 1 0\n", ""},

	// Built-in variables
	// ARGC is tested in goawk_test.go
	{`
BEGIN {
	print CONVFMT, 1.2345678 ""
	CONVFMT = "%.3g"
	print CONVFMT, 1.234567 ""
}`, "", "%.6g 1.23457\n%.3g 1.23\n", ""},
	{`BEGIN { FILENAME = "foo"; print FILENAME }`, "", "foo\n", ""},
	{`BEGIN { FILENAME = "123.0"; print (FILENAME==123) }`, "", "0\n", ""},
	// Other FILENAME behaviour is tested in goawk_test.go
	{`BEGIN { FNR = 123; print FNR }`, "", "123\n", ""},
	{`{ print FNR, $0 }`, "a\nb\nc", "1 a\n2 b\n3 c\n", ""},
	// Other FNR behaviour is tested in goawk_test.go
	{`BEGIN { print "|" FS "|"; FS="," } { print $1, $2 }`, "a b\na,b\nx,,y", "| |\na b \na b\nx \n", ""},
	{`BEGIN { print "|" FS "|"; FS="\\." } { print $1, $2 }`, "a b\na.b\nx..y", "| |\na b \na b\nx \n", ""},
	{`BEGIN { FS="\\" } { print $1, $2 }`, "a\\b", "a b\n", ""},
	{`{ print NF }`, "\na\nc d\ne f g", "0\n1\n2\n3\n", ""},
	{`BEGIN { NR = 123; print NR }`, "", "123\n", ""},
	{`{ print NR, $0 }`, "a\nb\nc", "1 a\n2 b\n3 c\n", ""},
	{`
BEGIN {
	print OFMT, 1.2345678
	OFMT = "%.3g"
	print OFMT, 1.234567
}`, "", "%.6g 1.23457\n%.3g 1.23\n", ""},
	// OFS and ORS are tested above
	{`BEGIN { print RSTART, RLENGTH; RSTART=5; RLENGTH=42; print RSTART, RLENGTH; } `, "",
		"0 0\n5 42\n", ""},
	{`BEGIN { print RS }`, "", "\n\n", ""},
	{`BEGIN { print RS; RS="|"; print RS }  { print }`, "a b|c d|", "\n\n|\na b\nc d\n", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		"a\n\nb\nc",
		"1 (1):\na\n2 (2):\nb\nc\n", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		"1\n2\n\na\nb",
		"1 (2):\n1\n2\n2 (2):\na\nb\n", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		"a b\nc d\n\ne f\n\n\n  \n\n\ng h\n\n\n",
		"1 (2):\na b\nc d\n2 (1):\ne f\n3 (1):\n  \n4 (1):\ng h\n", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		"\n\na b\n\nc d\n",
		"1 (1):\na b\n2 (1):\nc d\n", ""},
	{`BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }  # !awk !gawk - they don't handle CR LF with RS==""`,
		"\r\n\r\na b\r\n\r\nc d\r\n",
		"1 (1):\na b\n2 (1):\nc d\n", ""},
	{`BEGIN { RS=""; FS="X" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) printf "%s|", $i }`,
		"aXb\ncXd\n\neXf\n\n\n  \n\n\ngXh\n\n\n",
		"1 (4):\na|b|c|d|2 (2):\ne|f|3 (1):\n  |4 (2):\ng|h|", ""},
	{`BEGIN { RS = "" }  { print "got", $0 }`,
		"\n\n\n\n", "", ""},
	{`BEGIN { RS="\n" }  { print }`, "a\n\nb\nc", "a\n\nb\nc\n", ""},
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
}`, "", "\x1c\nonetwo\n1\x1c2 onetwo\n|\nonetwo\n1|2 onetwo\n", ""},

	// Field expressions and assignment (and interaction with NF)
	{`{ print NF; NF=1; $2="two"; print $0, NF }`, "\n", "0\n two 2\n", ""},
	{`{ print NF; NF=2; $2="two"; print $0, NF}`, "\n", "0\n two 2\n", ""},
	{`{ print NF; NF=3; $2="two"; print $0, NF}`, "a b c\n", "3\na two c 3\n", ""},
	{`{ print; print $1, $3, $NF }`, "a b c d e", "a b c d e\na c e\n", ""},
	{`{ print $1,$3; $2="x"; print; print $2 }`, "a b c", "a c\na x c\nx\n", ""},
	{`{ print; $0="x y z"; print; print $1, $3 }`, "a b c", "a b c\nx y z\nx z\n", ""},
	{`{ print $1^2 }`, "10", "100\n", ""},
	{`{ print $-1 }`, "x", "", "field index negative: -1"},
	{`{ NF=-1; }  # !awk - awk allows setting negative NF`,
		"x", "", "NF set to negative value: -1"},
	{`{ NF=1234567; }`, "x", "", "NF set too large: 1234567"},
	{`BEGIN { $1234567=1 }`, "", "", "field index too large: 1234567"},
	{`0 in FS  # !awk - doesn't flag this as an error`, "x", "",
		`parse error at 1:6: can't use scalar "FS" as array`},
	// TODO: I think this is happening because we parse this as ($($0))++ rather than ($($0++))
	// TODO: {`{ $$0++; print $0 }`, "2 3 4", "3\n", ""},
	// TODO: {`BEGIN { $0="3 4 5 6 7 8 9"; a=3; print $$a++++; print }`, "", "7\n3 4 6 6 8 8 9\n", ""},

	// Lots of NF tests with different combinations of NF, $, and number
	// of input fields. Some of these cause segmentation faults on awk
	// (but work fine on gawk and mawk).
	{`{ NF=1; $1="x"; print $0; print NF }`, "a", "x\n1\n", ""},
	{`{ NF=1; $1="x"; print $0; print NF }`, "a b", "x\n1\n", ""},
	{`{ NF=1; $1="x"; print $0; print NF }`, "a b c", "x\n1\n", ""},
	{`{ NF=1; $2="x"; print $0; print NF }`, "a", "a x\n2\n", ""},
	{`{ NF=1; $2="x"; print $0; print NF }`, "a b", "a x\n2\n", ""},
	{`{ NF=1; $2="x"; print $0; print NF }`, "a b c", "a x\n2\n", ""},
	{`{ NF=1; $3="x"; print $0; print NF }`, "a", "a  x\n3\n", ""},
	{`{ NF=1; $3="x"; print $0; print NF }  # !awk - awk differs from gawk (but gawk seems right)`,
		"a b", "a  x\n3\n", ""},
	{`{ NF=1; $3="x"; print $0; print NF }  # !awk - awk differs from gawk (but gawk seems right)`,
		"a b c", "a  x\n3\n", ""},
	{`{ NF=2; $1="x"; print $0; print NF }`, "a", "x \n2\n", ""},
	{`{ NF=2; $1="x"; print $0; print NF }`, "a b", "x b\n2\n", ""},
	{`{ NF=2; $1="x"; print $0; print NF }`, "a b c", "x b\n2\n", ""},
	{`{ NF=2; $2="x"; print $0; print NF }`, "a", "a x\n2\n", ""},
	{`{ NF=2; $2="x"; print $0; print NF }`, "a b", "a x\n2\n", ""},
	{`{ NF=2; $2="x"; print $0; print NF }`, "a b c", "a x\n2\n", ""},
	{`{ NF=2; $3="x"; print $0; print NF }`, "a", "a  x\n3\n", ""},
	{`{ NF=2; $3="x"; print $0; print NF }`, "a b", "a b x\n3\n", ""},
	{`{ NF=2; $3="x"; print $0; print NF }`, "a b c", "a b x\n3\n", ""},
	{`{ NF=3; $1="x"; print $0; print NF }  # !awk - segmentation fault`,
		"a", "x  \n3\n", ""},
	{`{ NF=3; $1="x"; print $0; print NF }  # !awk - segmentation fault`,
		"a b", "x b \n3\n", ""},
	{`{ NF=3; $1="x"; print $0; print NF }`, "a b c", "x b c\n3\n", ""},
	{`{ NF=3; $2="x"; print $0; print NF }  # !awk - segmentation fault`,
		"a", "a x \n3\n", ""},
	{`{ NF=3; $2="x"; print $0; print NF }  # !awk - segmentation fault`,
		"a b", "a x \n3\n", ""},
	{`{ NF=3; $2="x"; print $0; print NF }`, "a b c", "a x c\n3\n", ""},
	{`{ NF=3; $3="x"; print $0; print NF }`, "a", "a  x\n3\n", ""},
	{`{ NF=3; $3="x"; print $0; print NF }`, "a b", "a b x\n3\n", ""},
	{`{ NF=3; $3="x"; print $0; print NF }`, "a b c", "a b x\n3\n", ""},

	// Assignment expressions and vars
	{`BEGIN { print x; x = 4; print x; }`, "", "\n4\n", ""},
	{`BEGIN { a["foo"]=1; b[2]="x"; k="foo"; print a[k], b["2"] }`, "", "1 x\n", ""},
	{`BEGIN { s+=5; print s; s-=2; print s; s-=s; print s }`, "", "5\n3\n0\n", ""},
	{`BEGIN { x=2; x*=x; print x; x*=3; print x }`, "", "4\n12\n", ""},
	{`BEGIN { x=6; x/=3; print x; x/=x; print x; x/=.6; print x }`, "", "2\n1\n1.66667\n", ""},
	{`BEGIN { x=12; x%=5; print x }`, "", "2\n", ""},
	{`BEGIN { x=2; x^=5; print x; x^=0.5; print x }`, "", "32\n5.65685\n", ""},
	{`{ $2+=10; print; $3/=2; print }`, "1 2 3", "1 12 3\n1 12 1.5\n", ""},
	{`BEGIN { a[2] += 1; a["2"] *= 3; print a[2] }`, "", "3\n", ""},

	// Incr/decr expressions
	{`BEGIN { print x++; print x }`, "", "0\n1\n", ""},
	{`BEGIN { print x; print x++; print ++x; print x }`, "", "\n0\n2\n2\n", ""},
	{`BEGIN { print x; print x--; print --x; print x }`, "", "\n0\n-2\n-2\n", ""},
	{`BEGIN { s++; s++; print s }`, "", "2\n", ""},
	{`BEGIN { y=" "; --x[y = y y]; print length(y) }`, "", "2\n", ""},
	{`BEGIN { x[y++]++; print y }`, "", "1\n", ""},
	{`BEGIN { x[y++] += 3; print y }`, "", "1\n", ""},
	{`BEGIN { $(y++)++; print y }`, "", "1\n", ""},

	// Builtin functions
	{`BEGIN { print sin(0), sin(0.5), sin(1), sin(-1) }`, "", "0 0.479426 0.841471 -0.841471\n", ""},
	{`BEGIN { print cos(0), cos(0.5), cos(1), cos(-1) }`, "", "1 0.877583 0.540302 0.540302\n", ""},
	{`BEGIN { print exp(0), exp(0.5), exp(1), exp(-1) }`, "", "1 1.64872 2.71828 0.367879\n", ""},
	{`BEGIN { print log(0), log(0.5), log(1) }`, "", "-inf -0.693147 0\n", ""},
	{`BEGIN { print log(-1) }  # !gawk - gawk prints warning for this as well`,
		"", "nan\n", ""},
	{`BEGIN { print sqrt(0), sqrt(2), sqrt(4) }`, "", "0 1.41421 2\n", ""},
	{`BEGIN { print int(3.5), int("1.9"), int(4), int(-3.6), int("x"), int("") }`, "", "3 1 4 -3 0 0\n", ""},
	{`BEGIN { print match("food", "foo"), RSTART, RLENGTH }`, "", "1 1 3\n", ""},
	{`BEGIN { print match("x food y", "fo"), RSTART, RLENGTH }`, "", "3 3 2\n", ""},
	{`BEGIN { print match("x food y", "fox"), RSTART, RLENGTH }`, "", "0 0 -1\n", ""},
	{`BEGIN { print match("x food y", /[fod]+/), RSTART, RLENGTH }`, "", "3 3 4\n", ""},
	{`{ print length, length(), length("buzz"), length("") }`, "foo bar", "7 7 4 0\n", ""},
	{`BEGIN { print index("foo", "f"), index("foo0", 0), index("foo", "o"), index("foo", "x") }`, "", "1 4 2 0\n", ""},
	{`BEGIN { print atan2(1, 0.5), atan2(-1, 0) }`, "", "1.10715 -1.5708\n", ""},
	{`BEGIN { print sprintf("%3d", 42) }`, "", " 42\n", ""},
	{`BEGIN { print sprintf("%d", 12, 34) }`, "", "12\n", ""},
	{`BEGIN { print sprintf("%d") }`, "", "", `not enough arguments (0) for format string "%d"`},
	{`BEGIN { print sprintf("%d", 12, 34) }`, "", "12\n", ""},
	{`BEGIN { print sprintf("% 5d", 42) }`, "", "   42\n", ""},
	{`BEGIN { print substr("food", 1) }`, "", "food\n", ""},
	{`BEGIN { print substr("food", 1, 2) }`, "", "fo\n", ""},
	{`BEGIN { print substr("food", 1, 4) }`, "", "food\n", ""},
	{`BEGIN { print substr("food", 1, 8) }`, "", "food\n", ""},
	{`BEGIN { print substr("food", 2) }`, "", "ood\n", ""},
	{`BEGIN { print substr("food", 2, 2) }`, "", "oo\n", ""},
	{`BEGIN { print substr("food", 2, 3) }`, "", "ood\n", ""},
	{`BEGIN { print substr("food", 2, 8) }`, "", "ood\n", ""},
	{`BEGIN { print substr("food", 0, 8) }`, "", "food\n", ""},
	{`BEGIN { print substr("food", -1, 8) }`, "", "food\n", ""},
	{`BEGIN { print substr("food", 5, 8) }`, "", "\n", ""},
	{`BEGIN { n = split("ab c d ", a); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n", ""},
	{`BEGIN { n = split("ab,c,d,", a, ","); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n\n", ""},
	{`BEGIN { n = split("ab,c.d,", a, /[,.]/); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n\n", ""},
	{`BEGIN { n = split("1 2", a); print (n, a[1], a[2], a[1]==1, a[2]==2) }`, "", "2 1 2 1 1\n", ""},
	{`BEGIN { x = "1.2.3"; print sub(/\./, ",", x); print x }`, "", "1\n1,2.3\n", ""},
	{`{ print sub(/\./, ","); print $0 }`, "1.2.3", "1\n1,2.3\n", ""},
	{`BEGIN { x = "1.2.3"; print gsub(/\./, ",", x); print x }`, "", "2\n1,2,3\n", ""},
	{`{ print gsub(/\./, ","); print $0 }`, "1.2.3", "2\n1,2,3\n", ""},
	{`{ print gsub(/[0-9]/, "(&)"); print $0 }`, "0123x. 42y", "6\n(0)(1)(2)(3)x. (4)(2)y\n", ""},
	{`{ print gsub(/[0-9]+/, "(&)"); print $0 }`, "0123x. 42y", "2\n(0123)x. (42)y\n", ""},
	{`{ print gsub(/[0-9]/, "\\&"); print $0 }`, "0123x. 42y", "6\n&&&&x. &&y\n", ""},
	{`{ print gsub(/[0-9]/, "\\z"); print $0 }`, "0123x. 42y", "6\n\\z\\z\\z\\zx. \\z\\zy\n", ""},
	{`{ print gsub("0", "x\\\\y"); print $0 }  # !awk !gawk -- our behaviour is per POSIX spec (gawk -P and mawk)`,
		"0", "1\nx\\y\n", ""},
	{`sub("", "\\e", FS)  # !awk !gawk`, "foo bar\nbaz buz\n", "",
		"invalid regex \"\\\\e \": error parsing regexp: invalid escape sequence: `\\e`"},
	{`BEGIN { print tolower("Foo BaR") }`, "", "foo bar\n", ""},
	{`BEGIN { print toupper("Foo BaR") }`, "", "FOO BAR\n", ""},
	{`
BEGIN {
	srand(1)
	a = rand(); b = rand(); c = rand()
	srand(1)
	x = rand(); y = rand(); z = rand()
	print (a==b, b==c, x==y, y==z)
	print (a==x, b==y, c==z)
}
`, "", "0 0 0 0\n1 1 1\n", ""},
	{`
BEGIN {
	for (i = 0; i < 1000; i++) {
		if (rand() < 0.5) n++
	}
	print (n>400)
}
`, "", "1\n", ""},
	{`BEGIN { print system("echo foo"); print system("echo bar") }  # !fuzz`,
		"", "foo\n0\nbar\n0\n", ""},
	{`BEGIN { print system(">&2 echo error") }  # !fuzz`,
		"", "error\n0\n", ""},

	// Conditional expressions parse and work correctly
	{`BEGIN { print 0?"t":"f" }`, "", "f\n", ""},
	{`BEGIN { print 1?"t":"f" }`, "", "t\n", ""},
	{`BEGIN { print (1+2)?"t":"f" }`, "", "t\n", ""},
	{`BEGIN { print (1+2?"t":"f") }`, "", "t\n", ""},
	{`BEGIN { print(1 ? x="t" : "f"); print x; }`, "", "t\nt\n", ""},

	// Locals vs globals, array params, and recursion
	{`
function f(loc) {
	glob += 1
	loc += 1
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
`, "", "2 4\n42\n3 5\n42\n", ""},
	{`
function set(a, x, v) {
	a[x] = v
}
function get(a, x) {
	return a[x]
}
BEGIN {
	a["x"] = 1
	set(b, "y", 2)
	for (k in a) print k, a[k]
	print "---"
	for (k in b) print k, b[k]
	print "---"
	print get(a, "x"), get(b, "y")
}
`, "", "x 1\n---\ny 2\n---\n1 2\n", ""},
	{`
function fib(n) {
	return n < 3 ? 1 : fib(n-2) + fib(n-1)
}
BEGIN {
	for (i = 1; i <= 7; i++) {
		printf "%d ", fib(i)
	}
}
`, "", "1 1 2 3 5 8 13 ", ""},
	{`
function f(a, x) { return a[x] }
function g(b, y) { f(b, y) }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1) }
`, "", "2\n\n", ""},
	{`
function g(b, y) { return f(b, y) }
function f(a, x) { return a[x] }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1) }
`, "", "2\n2\n", ""},
	{`
function h(b, y) { g(b, y) }
function g(b, y) { f(b, y) }
function f(a, x) { return a[x] }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1) }
`, "", "2\n\n", ""},
	{`
function h(b, y) { return g(b, y) }
function g(b, y) { return f(b, y) }
function f(a, x) { return a[x] }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1); print h(c, 1) }
`, "", "2\n2\n2\n", ""},
	{`
function get(a, x) { return a[x] }
BEGIN { a[1]=2; print get(a, x); print get(1, 2); }
# !awk - awk doesn't detect this
`, "", "", `parse error at 3:40: can't pass scalar 1 as array param`},
	{`
function early() {
	print "x"
	return
	print "y"
}
BEGIN { early() }
`, "", "x\n", ""},
	{`BEGIN { return }`, "", "", "parse error at 1:9: return must be inside a function"},
	{`function f() { printf "x" }; BEGIN { f() } `, "", "x", ""},
	{`function f(x) { 0 in _; f(_) }  BEGIN { f() }  # !awk !gawk`, "", "",
		`parse error at 1:25: can't pass array "_" as scalar param`},
	{`BEGIN { for (i=0; i<1001; i++) f(); print x }  function f() { x++ }`, "", "1001\n", ""},
	{`
function bar(y) { return y[1] }
function foo() { return bar(x) }
BEGIN { x[1] = 42; print foo() }
`, "", "42\n", ""},
	// TODO: failing because f1 doesn't use x, so resolver assumes its type is scalar
	// 		{`
	// function f1(x) { }
	// function f2(x, y) { return x[y] }
	// BEGIN { a[1]=2; f1(a); print f2(a, 1) }
	// `, "", "2\n", ""},

	// Type checking / resolver tests
	{`BEGIN { a[x]; a=42 }`, "", "", `parse error at 1:15: can't use array "a" as scalar`},
	{`BEGIN { s=42; s[x] }`, "", "", `parse error at 1:15: can't use scalar "s" as array`},
	{`function get(a, k) { return a[k] }  BEGIN { a = 42; print get(a, 1); }  # !awk - doesn't error in awk`,
		"", "", `parse error at 1:59: can't pass scalar "a" as array param`},
	{`function get(a, k) { return a+k } BEGIN { a[42]; print get(a, 1); }`,
		"", "", `parse error at 1:56: can't pass array "a" as scalar param`},
	{`{ f(z) }  function f(x) { print NR }`, "abc", "1\n", ""},
	{`function f() { f() }  BEGIN { f() }  # !awk !gawk`, "", "", `calling "f" exceeded maximum call depth of 1000`},
	{`function f(x) { 0 in x }  BEGIN { f(FS) }  # !awk`, "", "", `parse error at 1:35: can't pass scalar "FS" as array param`},
	{`
function foo(x) { print "foo", x }
function bar(foo) { print "bar", foo }
BEGIN { foo(5); bar(10) }
`, "", "foo 5\nbar 10\n", ""},
	{`
function foo(foo) { print "foo", foo }
function bar(foo) { print "bar", foo }
BEGIN { foo(5); bar(10) }
`, "", "", `parse error at 2:14: can't use function name as parameter name`},
	{`function foo() { print foo }  BEGIN { foo() }`,
		"", "", `parse error at 1:46: global var "foo" can't also be a function`},
	{`function f(x) { print x, x(); }  BEGIN { f() }`, "", "", `parse error at 1:27: can't call local variable "x" as function`},

	// Redirected I/O (we give explicit errors, awk and gawk don't)
	// TODO: the following two tests sometimes fail under TravisCI with: "write |1: broken pipe"
	// {`BEGIN { print >"out"; getline <"out" }  # !awk !gawk`, "", "", "can't read from writer stream"},
	// {`BEGIN { print |"out"; getline <"out" }  # !awk !gawk`, "", "", "can't read from writer stream"},
	{`BEGIN { print >"out"; close("out"); getline <"out"; print >"out" }  # !awk !gawk`, "", "", "can't write to reader stream"},
	{`BEGIN { print >"out"; close("out"); getline <"out"; print |"out" }  # !awk !gawk`, "", "", "can't write to reader stream"},
	// TODO: currently we support "getline var" but not "getline lvalue"
	// TODO: {`BEGIN { getline a[1]; print a[1] }`, "foo", "foo\n", ""},
	// TODO: {`BEGIN { getline $1; print $1 }`, "foo", "foo\n", ""},
	{`BEGIN { print "foo" |"sort"; print "bar" |"sort" }  # !fuzz`, "", "bar\nfoo\n", ""},
	{`BEGIN { print "foo" |">&2 echo error" }  # !fuzz`, "", "error\n", ""},
	{`BEGIN { "cat" | getline; print }  # !fuzz`, "bar", "bar\n", ""},
	// TODO: fix test flakiness on Windows (sometimes returns "\nerror\n")
	// {`BEGIN { ">&2 echo error" | getline; print }`, "", "error\n\n", ""},
	{`BEGIN { print getline x < "/no/such/file" }  # !fuzz`, "", "-1\n", ""},

	// Redirecting to or from a filename of "-" means write to stdout or read from stdin
	{`BEGIN { print getline x < "-"; print x }`, "a\nb\n", "1\na\n", ""},
	{`{ print $0; print getline x <"-"; print x }`, "one\ntwo\n", "one\n0\n\ntwo\n0\n\n", ""},
	{`BEGIN { print "x" >"-"; print "y" >"-" }`, "", "x\ny\n", ""},

	// fflush() function - tests parsing and some edge cases, but not
	// actual flushing behavior (that's partially tested in TestFlushes).
	{`BEGIN { print fflush(); print fflush("") }`, "", "0\n0\n", ""},
	{`BEGIN { print "x"; print fflush(); print "y"; print fflush("") }`, "", "x\n0\ny\n0\n", ""},
	{`BEGIN { print "x" >"out"; print fflush("out"); print "y"; print fflush("") }  # !fuzz`, "", "0\ny\n0\n", ""},
	{`BEGIN { print fflush("x") }  # !gawk`, "", "error flushing \"x\": not an output file or pipe\n-1\n", ""},
	{`BEGIN { "cat" | getline; print fflush("cat") }  # !gawk !fuzz`, "", "error flushing \"cat\": not an output file or pipe\n-1\n", ""},

	// Greater than operator requires parentheses in print statement,
	// otherwise it's a redirection directive
	{`BEGIN { print "x" > "out" }  # !fuzz`, "", "", ""},
	{`BEGIN { printf "x" > "out" }  # !fuzz`, "", "", ""},
	{`BEGIN { print("x" > "out") }`, "", "1\n", ""},
	{`BEGIN { printf("x" > "out") }`, "", "1", ""},

	// Grammar should allow blocks wherever statements are allowed
	{`BEGIN { if (1) printf "x"; else printf "y" }`, "", "x", ""},
	{`BEGIN { printf "x"; { printf "y"; printf "z" } }`, "", "xyz", ""},

	// Ensure syntax errors result in errors
	// TODO: {`{ $1 = substr($1, 1, 3) print $1 }`, "", "", "ERROR"},
	{`BEGIN { f() }`, "", "", `parse error at 1:9: undefined function "f"`},
	{`function f() {} function f() {} BEGIN { }`, "", "", `parse error at 1:26: function "f" already defined`},
	{`BEGIN { print (1,2),(3,4) }`, "", "", "parse error at 1:15: unexpected comma-separated expression"},
	{`BEGIN { print (1,2,(3,4),(5,6)) }`, "", "", "parse error at 1:20: unexpected comma-separated expression"},
}

func TestAWKGo(t *testing.T) {
	tempDir := t.TempDir()

	for i, test := range interpTests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}
		t.Run(testName, func(t *testing.T) {
			output, err := os.Create(filepath.Join(tempDir, fmt.Sprintf("test_%d.go", i)))
			if err != nil {
				t.Fatalf("error creating temp file: %v", err)
			}
			defer os.Remove(output.Name())

			prog, err := parser.ParseProgram([]byte(test.src), nil)
			if err != nil {
				if test.err != "" {
					if err.Error() == test.err {
						return
					}
					t.Fatalf("expected error %q, got %q", test.err, err.Error())
				}
				t.Fatalf("parse error: %v", err)
			}

			err = compile(prog, output)
			if err != nil {
				if test.err != "" {
					if err.Error() == test.err {
						return
					}
					t.Fatalf("expected error %q, got %q", test.err, err.Error())
				}
				t.Fatalf("compile error: %v", err)
			}

			err = output.Close()
			if err != nil {
				t.Fatalf("error closing temp file: %v", err)
			}

			cmd := exec.Command("go", "run", output.Name())
			cmd.Stdin = strings.NewReader(test.in)
			out, err := cmd.CombinedOutput()
			if err != nil {
				if test.err != "" {
					if err.Error() == test.err {
						return
					}
					t.Fatalf("expected error %q, got %q", test.err, err.Error())
				}
				t.Fatalf("go run error: %v:\n%s", err, out)
			}
			if string(out) != test.out {
				t.Fatalf("expected %q, got %q", test.out, out)
			}
		})
	}
}
