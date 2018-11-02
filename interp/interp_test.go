// Tests for GoAWK interpreter.
package interp_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

var (
	awkExe string
)

func TestMain(m *testing.M) {
	flag.StringVar(&awkExe, "awk", "", "awk executable name")
	flag.Parse()
	os.Exit(m.Run())
}

// Note: a lot of these are really parser tests too.
func TestInterp(t *testing.T) {
	longLine := strings.Repeat("x", 70000)

	tests := []struct {
		src    string // if this includes "!awk" or "!gawk" those interpreters won't be run
		in     string
		out    string
		err    string // error from GoAWK must equal this
		awkErr string // error from gawk must contain this
	}{
		// BEGIN and END work correctly
		{`BEGIN { print "b" }`, "", "b\n", "", ""},
		{`BEGIN { print "b" }`, "foo", "b\n", "", ""},
		{`END { print "e" }`, "", "e\n", "", ""},
		{`END { print "e" }`, "foo", "e\n", "", ""},
		{`BEGIN { print "b"} END { print "e" }`, "", "b\ne\n", "", ""},
		{`BEGIN { print "b"} END { print "e" }`, "foo", "b\ne\n", "", ""},
		{`BEGIN { print "b"} $0 { print NR } END { print "e" }`, "foo", "b\n1\ne\n", "", ""},

		// Patterns
		{`$0`, "foo\n\nbar", "foo\nbar\n", "", ""},
		{`{ print $0 }`, "foo\n\nbar", "foo\n\nbar\n", "", ""},
		{`$1=="foo"`, "foo\n\nbar", "foo\n", "", ""},
		{`$1==42`, "foo\n42\nbar", "42\n", "", ""},
		{`$1=="42"`, "foo\n42\nbar", "42\n", "", ""},
		{`/foo/`, "foo\nx\nfood\nxfooz\nbar", "foo\nfood\nxfooz\n", "", ""},
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
		{`BEGIN { printf "%.1g", 42 }`, "", "4e+01", "", ""},
		{`BEGIN { printf "%d", 12, 34 }`, "", "12", "", ""},
		{`BEGIN { printf "%d" }`, "", "", "format error: got 0 args, expected 1", "not enough arg"},

		// if and loop statements
		{`BEGIN { if (1) print "t"; }`, "", "t\n", "", ""},
		{`BEGIN { if (0) print "t"; }`, "", "", "", ""},
		{`BEGIN { if (1) print "t"; else print "f" }`, "", "t\n", "", ""},
		{`BEGIN { if (0) print "t"; else print "f" }`, "", "f\n", "", ""},
		{`BEGIN { for (;;) { print "x"; break } }`, "", "x\n", "", ""},
		{`BEGIN { for (;;) { printf "%d ", i; i++; if (i>2) break; } }`, "", "0 1 2 ", "", ""},
		{`BEGIN { for (i=5; ; ) { printf "%d ", i; i++; if (i>8) break; } }`, "", "5 6 7 8 ", "", ""},
		{`BEGIN { for (i=5; ; i++) { printf "%d ", i; if (i>8) break; } }`, "", "5 6 7 8 9 ", "", ""},
		{`BEGIN { for (i=5; i<8; i++) { printf "%d ", i } }`, "", "5 6 7 ", "", ""},
		{`BEGIN { for (i=0; i<10; i++) { if (i < 5) continue; printf "%d ", i } }`, "", "5 6 7 8 9 ", "", ""},
		{`BEGIN { a["x"] = 3; a["y"] = 4; for (k in a) x += a[k]; print x }`, "", "7\n", "", ""},
		{`BEGIN { while (i < 5) { print i; i++ } }`, "", "\n1\n2\n3\n4\n", "", ""},
		{`BEGIN { do { print i; i++ } while (i < 5) }`, "", "\n1\n2\n3\n4\n", "", ""},
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
		{`BEGIN { next }`, "", "", "parse error at 1:9: next can't be in BEGIN or END", "BEGIN"},
		{`END { next }`, "", "", "parse error at 1:7: next can't be in BEGIN or END", "END"},

		// Arrays, "in", and delete
		{`BEGIN { a["x"] = 3; print "x" in a, "y" in a }`, "", "1 0\n", "", ""},
		{`BEGIN { a["x"] = 3; a["y"] = 4; delete a["x"]; for (k in a) print k, a[k] }`, "", "y 4\n", "", ""},
		{`BEGIN { a["x"] = 3; a["y"] = 4; for (k in a) delete a[k]; for (k in a) print k, a[k] }`, "", "", "", ""},

		// Unary expressions: ! + -
		{`BEGIN { print !42, !1, !0, !!42, !!1, !!0 }`, "", "0 0 1 1 1 0\n", "", ""},
		{`BEGIN { print !42, !1, !0, !!42, !!1, !!0 }`, "", "0 0 1 1 1 0\n", "", ""},
		{`BEGIN { print +4, +"3", +0, +-3, -3, - -4, -"3" }`, "", "4 3 0 -3 -3 4 -3\n", "", ""},

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

		// Other binary expressions: + - * ^ / % CONCAT ~ !~
		{`BEGIN { print 1+2, 1+2+3, 1+-2, -1+2, "1"+"2", 3+.14 }`, "", "3 6 -1 1 3 3.14\n", "", ""},
		{`BEGIN { print 1-2, 1-2-3, 1-+2, -1-2, "1"-"2", 3-.14 }`, "", "-1 -4 -1 -3 -1 2.86\n", "", ""},
		{`BEGIN { print 2*3, 2*3*4, 2*-3, -2*3, "2"*"3", 3*.14 }`, "", "6 24 -6 -6 6 0.42\n", "", ""},
		{`BEGIN { print 2/3, 2/3/4, 2/-3, -2/3, "2"/"3", 3/.14 }`, "", "0.666667 0.166667 -0.666667 -0.666667 0.666667 21.4286\n", "", ""},
		{`BEGIN { print 2%3, 2%3%4, 2%-3, -2%3, "2"%"3", 3%.14 }`, "", "2 2 2 -2 2 0.06\n", "", ""},
		{`BEGIN { print 2^3, 2^3^3, 2^-3, -2^3, "2"^"3", 3^.14 }`, "", "8 134217728 0.125 -8 8 1.16626\n", "", ""},
		{`BEGIN { print 1 2, "x" "yz", 1+2 3+4 }`, "", "12 xyz 37\n", "", ""},
		{`BEGIN { print "food"~/oo/, "food"~/[oO]+d/, "food"~"f", "food"~"F", "food"~0 }`, "", "1 1 1 0 0\n", "", ""},
		{`BEGIN { print "food"!~/oo/, "food"!~/[oO]+d/, "food"!~"f", "food"!~"F", "food"!~0 }`, "", "0 0 0 1 1\n", "", ""},
		{`BEGIN { print 1+2*3/4^5%6 7, (1+2)*3/4^5%6 "7" }`, "", "1.005867 0.008789067\n", "", ""},

		// Number, string, and regex expressions
		{`BEGIN { print 1, 1., .1, 1e0, -1, 1e }`, "", "1 1 0.1 1 -1 1\n", "", ""},
		{`BEGIN { print '\"' '\'' 'xy' "z" "'" '\"' }`, "", "\"'xyz'\"\n", "", "syntax error"}, // Check support for single-quoted strings
		{`{ print /foo/ }`, "food\nfoo\nxfooz\nbar\n", "1\n1\n1\n0\n", "", ""},
		{`/[a-/`, "foo", "", "parse error at 1:1: error parsing regexp: missing closing ]: `[a-`", "terminated"},
		{`BEGIN { print "-12"+0, "+12"+0, " \t\r\n7foo"+0, ".5"+0, "5."+0, "+."+0 }`, "", "-12 12 7 0.5 5 0\n", "", ""},
		{`BEGIN { print "1e3"+0, "1.2e-1"+0, "1e+1"+0, "1e"+0, "1e+"+0 }`, "", "1000 0.12 10 1 1\n", "", ""},
		{`BEGIN { print -(11102200000000000000000000000000000000 1040000) }  # !gawk - gawk supports big numbers`,
			"", "-inf\n", "", ""},
		{`BEGIN { print atan2(0, 8020020000000e20G-0)}`, "", "0\n", "", ""},
		{`BEGIN { print 1e1000, -1e1000 }`, "", "inf -inf\n", "", ""},

		// Conditional ?: expression
		{`{ print /x/?"t":"f" }`, "x\ny\nxx\nz\n", "t\nf\nt\nf\n", "", ""},
		{`BEGIN { print 1?2?3:4:5, 1?0?3:4:5, 0?2?3:4:5 }`, "", "3 4 5\n", "", ""},

		// Built-in variables
		// ARGC is tested in goawk_test.go
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
		{`BEGIN { RS="\n" }  { print }`, "a\n\nb\nc", "a\n\nb\nc\n", "", ""},
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
		{`{ print $-1 }`, "x", "", "field index negative: -1", "field -1"},
		{`{ NF=-1; }  # !awk - awk allows setting negative NF`,
			"x", "", "NF set to negative value: -1", "negative value"},
		{`{ NF=1234567; }`, "x", "", "NF set too large: 1234567", ""},
		{`BEGIN { $1234567=1 }`, "", "", "field index too large: 1234567", ""},

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
		{`{ $2+=10; print; $3/=2; print }`, "1 2 3", "1 12 3\n1 12 1.5\n", "", ""},
		{`BEGIN { a[2] += 1; a["2"] *= 3; print a[2] }`, "", "3\n", "", ""},

		// Incr/decr expressions
		{`BEGIN { print x; print x++; print ++x; print x }`, "", "\n0\n2\n2\n", "", ""},
		{`BEGIN { print x; print x--; print --x; print x }`, "", "\n0\n-2\n-2\n", "", ""},
		{`BEGIN { s++; s++; print s }`, "", "2\n", "", ""},

		// Builtin functions
		{`BEGIN { print sin(0), sin(0.5), sin(1), sin(-1) }`, "", "0 0.479426 0.841471 -0.841471\n", "", ""},
		{`BEGIN { print cos(0), cos(0.5), cos(1), cos(-1) }`, "", "1 0.877583 0.540302 0.540302\n", "", ""},
		{`BEGIN { print exp(0), exp(0.5), exp(1), exp(-1) }`, "", "1 1.64872 2.71828 0.367879\n", "", ""},
		{`BEGIN { print log(0), log(0.5), log(1) }`, "", "-inf -0.693147 0\n", "", ""},
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
		{`BEGIN { print substr("food", 5, 8) }`, "", "\n", "", ""},
		{`BEGIN { n = split("ab c d ", a); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n", "", ""},
		{`BEGIN { n = split("ab,c,d,", a, ","); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n\n", "", ""},
		{`BEGIN { n = split("ab,c.d,", a, /[,.]/); for (i=1; i<=n; i++) print a[i] }`, "", "ab\nc\nd\n\n", "", ""},
		{`BEGIN { n = split("1 2", a); print (n, a[1], a[2], a[1]==1, a[2]==2) }`, "", "2 1 2 1 1\n", "", ""},
		{`BEGIN { x = "1.2.3"; print sub(/\./, ",", x); print x }`, "", "1\n1,2.3\n", "", ""},
		{`{ print sub(/\./, ","); print $0 }`, "1.2.3", "1\n1,2.3\n", "", ""},
		{`BEGIN { x = "1.2.3"; print gsub(/\./, ",", x); print x }`, "", "2\n1,2,3\n", "", ""},
		{`{ print gsub(/\./, ","); print $0 }`, "1.2.3", "2\n1,2,3\n", "", ""},
		{`{ print gsub(/[0-9]/, "(&)"); print $0 }`, "0123x. 42y", "6\n(0)(1)(2)(3)x. (4)(2)y\n", "", ""},
		{`{ print gsub(/[0-9]+/, "(&)"); print $0 }`, "0123x. 42y", "2\n(0123)x. (42)y\n", "", ""},
		{`{ print gsub(/[0-9]/, "\\&"); print $0 }`, "0123x. 42y", "6\n&&&&x. &&y\n", "", ""},
		{`BEGIN { print tolower("Foo BaR") }`, "", "foo bar\n", "", ""},
		{`BEGIN { print toupper("Foo BaR") }`, "", "FOO BAR\n", "", ""},
		{`
BEGIN {
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
`, "", "2 4\n42\n3 5\n42\n", "", ""},
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
`, "", "x 1\n---\ny 2\n---\n1 2\n", "", ""},
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
function h(b, y) { g(b, y) }
function g(b, y) { f(b, y) }
function f(a, x) { return a[x] }
BEGIN { c[1]=2; print f(c, 1); print g(c, 1) }
`, "", "2\n\n", "", ""},
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

		// Type checking / resolver tests
		{`BEGIN { a[x]; a=42 }`, "", "", `parse error at 1:15: can't use array "a" as scalar`, "array"},
		{`BEGIN { s=42; s[x] }`, "", "", `parse error at 1:15: can't use scalar "s" as array`, "array"},
		{`function get(a, k) { return a[k] }  BEGIN { a = 42; print get(a, 1); }  # !awk - doesn't error in awk`,
			"", "", `parse error at 1:59: can't pass scalar "a" as array param`, "attempt to use scalar parameter `a' as an array"},
		{`function get(a, k) { return a+k } BEGIN { a[42]; print get(a, 1); }`,
			"", "", `parse error at 1:56: can't pass array "a" as scalar param`, "array"},
		{`{ f(z) }  function f(x) { print NR }`, "abc", "1\n", "", ""},

		// Redirected I/O
		// TODO: these tests currently panic() due to bug with s.(io.Reader) in interp.go
		//{`BEGIN { print >"out"; getline <"out" }`, "", "", "can't read from writer stream", ""},
		//{`BEGIN { print |"out"; getline <"out" }`, "", "", "", ""},

		// Greater than operator requires parentheses in print statement,
		// otherwise it's a redirection directive
		{`BEGIN { print "x" > "out" }`, "", "", "", ""},
		{`BEGIN { printf "x" > "out" }`, "", "", "", ""},
		{`BEGIN { print("x" > "out") }`, "", "1\n", "", ""},
		{`BEGIN { printf("x" > "out") }`, "", "1", "", ""},

		// Grammar should allow blocks wherever statements are allowed
		{`BEGIN { if (1) printf "x"; else printf "y" }`, "", "x", "", ""},
		{`BEGIN { printf "x"; { printf "y"; printf "z" } }`, "", "xyz", "", ""},

		// Ensure certain odd syntax matches awk behaviour
		// {`BEGIN { printf "x" }; BEGIN { printf "y" }`, "", "xy", "", ""},
		// {`BEGIN { printf "x" };; BEGIN { printf "y" }`, "", "xy", "", ""},

		// Ensure syntax errors result in errors
		// {`{ $1 = substr($1, 1, 3) print $1 }`, "", "", "ERROR", "syntax error"},
		{`BEGIN { f() }`, "", "", `parse error at 1:9: undefined function "f"`, "defined"},
		{`function f() {} function f() {} BEGIN { }`, "", "", `parse error at 1:26: function "f" already defined`, "define"},

		// Ensure very long lines work (> 64KB)
		{`{ print length() }`, longLine, fmt.Sprintf("%d\n", len(longLine)), "", ""},
	}
	for _, test := range tests {
		testName := test.src
		if len(testName) > 70 {
			testName = testName[:70]
		}

		if awkExe != "" && !strings.Contains(test.src, "!"+awkExe) {
			// Run it through external awk program first
			t.Run("awk_"+testName, func(t *testing.T) {
				srcFile, err := ioutil.TempFile("", "goawktest_")
				if err != nil {
					t.Fatalf("error creating temp file: %v", err)
				}
				defer os.Remove(srcFile.Name())
				_, err = srcFile.Write([]byte(test.src))
				if err != nil {
					t.Fatalf("error writing temp file: %v", err)
				}
				cmd := exec.Command(awkExe, "-f", srcFile.Name(), "-")
				if test.in != "" {
					stdin, err := cmd.StdinPipe()
					if err != nil {
						t.Fatalf("error fetching stdin pipe: %v", err)
					}
					go func() {
						defer stdin.Close()
						stdin.Write([]byte(test.in))
					}()
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
			prog, err := parser.ParseProgram([]byte(test.src), nil)
			if err != nil {
				if test.err != "" {
					if err.Error() == test.err {
						return
					}
					t.Fatalf("expected error %q, got %q", test.err, err.Error())
				}
				t.Fatal(err)
			}
			outBuf := &bytes.Buffer{}
			errBuf := &bytes.Buffer{}
			config := &interp.Config{
				Stdin:  strings.NewReader(test.in),
				Output: outBuf,
				Error:  errBuf,
			}
			_, err = interp.ExecProgram(prog, config)
			if err != nil {
				if test.err != "" {
					if err.Error() == test.err {
						return
					}
					t.Fatalf("expected error %q, got %q", test.err, err.Error())
				}
				t.Fatal(err)
			}
			if test.err != "" {
				t.Fatalf(`expected error %q, got ""`, test.err)
			}
			out := outBuf.String() + errBuf.String()
			normalized := normalizeNewlines(out)
			if normalized != test.out {
				t.Fatalf("expected %q, got %q", test.out, normalized)
			}
		})
	}
	_ = os.Remove("out")
}

func benchmarkProgram(b *testing.B, input, expected, srcFormat string, args ...interface{}) {
	b.StopTimer()
	src := fmt.Sprintf(srcFormat, args...)
	prog, err := parser.ParseProgram([]byte(src), nil)
	if err != nil {
		b.Fatalf("error parsing %s: %v", b.Name(), err)
	}
	outBuf := &bytes.Buffer{}
	config := &interp.Config{
		Stdin:  strings.NewReader(input),
		Output: outBuf,
		Error:  ioutil.Discard,
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
	benchmarkProgram(b, "", "a 1", `
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
	benchmarkProgram(b, "", "b 2", `
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
	benchmarkProgram(b, "", "0 10", `
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
	benchmarkProgram(b, "", "", `
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
	benchmarkProgram(b, "", "21", `
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
	benchmarkProgram(b, "", "134", `
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
	benchmarkProgram(b, "", "134", `
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
	benchmarkProgram(b, "", " brown fox", `
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
	benchmarkProgram(b, "", "27", `
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
	benchmarkProgram(b, "", "22", `
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
	benchmarkProgram(b, "", "1 164", `
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
	benchmarkProgram(b, "", "1 164", `
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
	benchmarkProgram(b, "", "3 224", `
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
	benchmarkProgram(b, "", "3 224", `
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
	benchmarkProgram(b, "", "A 123   foo 3.14", `
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
	benchmarkProgram(b, "", "55", `
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
	benchmarkProgram(b, "", "75", `
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

func BenchmarkForLoop(b *testing.B) {
	benchmarkProgram(b, "", "", `
BEGIN {
  for (i = 0; i < %d; i++) {
  	for (j = 0; j < 100; j++);
  }
}
`, b.N)
}

func BenchmarkForInLoop(b *testing.B) {
	benchmarkProgram(b, "", "", `
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
	benchmarkProgram(b, "", "0", `
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
	benchmarkProgram(b, "", "0", `
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
	benchmarkProgram(b, input, expected, "$0")
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
	benchmarkProgram(b, input, expected, "{ print $1, $3 }")
}

func BenchmarkSetField(b *testing.B) {
	benchmarkProgram(b, "1 2 3", "one 2 three", `
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
	benchmarkProgram(b, "", "1", `
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
	benchmarkProgram(b, "", "5.0293", `
BEGIN {
  for (i = 0; i < %d; i++) {
    res = (1+2*3/4^5) + (1+2*3/4^5) + (1+2*3/4^5) + (1+2*3/4^5) + (1+2*3/4^5)
  }
  print res
}
`, b.N)
}

func BenchmarkConcatSmall(b *testing.B) {
	b.StopTimer()
	benchmarkProgram(b, "", "100", `
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
	benchmarkProgram(b, "", "1000000", `
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
	benchmarkProgram(b, "", "1", `
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
	benchmarkProgram(b, "", "243", `
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
	benchmarkProgram(b, "", "0 1 2 3 4", `
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
	benchmarkProgram(b, "", "5 -9 729 32 3.0536 2", `
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

func normalizeNewlines(s string) string {
	return strings.Replace(s, "\r\n", "\n", -1)
}
