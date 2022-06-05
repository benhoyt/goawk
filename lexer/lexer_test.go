// Test GoAWK Lexer

package lexer_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	. "github.com/benhoyt/goawk/lexer"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		// Comments, whitespace, line continuations
		{"+# foo \n- #foo", `1:1 + "", 1:8 <newline> "", 2:1 - ""`},
		{"+\\\n-", `1:1 + "", 2:1 - ""`},
		{"+\\\r\n-", `1:1 + "", 2:1 - ""`},
		{"+\\-", `1:1 + "", 1:3 <illegal> "expected \\n after \\ line continuation", 1:3 - ""`},

		// Names and keywords
		{"x", `1:1 name "x"`},
		{"x y0", `1:1 name "x", 1:3 name "y0"`},
		{"x 0y", `1:1 name "x", 1:3 number "0", 1:4 name "y"`},
		{"sub SUB", `1:1 sub "", 1:5 name "SUB"`},

		// String tokens
		{`"foo"`, `1:1 string "foo"`},
		{`"a\t\r\n\z\'\"\a\b\f\vb"`, `1:1 string "a\t\r\nz'\"\a\b\f\vb"`},
		{`"x`, `1:3 <illegal> "didn't find end quote in string"`},
		{`"foo\"`, `1:7 <illegal> "didn't find end quote in string"`},
		{"\"x\n\"", `1:3 <illegal> "can't have newline in string", 1:3 <newline> "", 2:2 <illegal> "didn't find end quote in string"`},
		{`'foo'`, `1:1 string "foo"`},
		{`'a\t\r\n\z\'\"b'`, `1:1 string "a\t\r\nz'\"b"`},
		{`'x`, `1:3 <illegal> "didn't find end quote in string"`},
		{"'x\n'", `1:3 <illegal> "can't have newline in string", 1:3 <newline> "", 2:2 <illegal> "didn't find end quote in string"`},
		{`"\x0.\x00.\x0A\x10\xff\xFF\x41"`, `1:1 string "\x00.\x00.\n\x10\xff\xffA"`},
		{`"\xg"`, `1:4 <illegal> "1 or 2 hex digits expected", 1:4 name "g", 1:6 <illegal> "didn't find end quote in string"`},
		{`"\0\78\7\77\777\0 \141 "`, `1:1 string "\x00\a8\a?\xff\x00 a "`},

		// Number tokens
		{"0", `1:1 number "0"`},
		{"9", `1:1 number "9"`},
		{" 0 ", `1:2 number "0"`},
		{"\n  1", `1:1 <newline> "", 2:3 number "1"`},
		{"1234", `1:1 number "1234"`},
		{".5", `1:1 number ".5"`},
		{".5e1", `1:1 number ".5e1"`},
		{"5e+1", `1:1 number "5e+1"`},
		{"5e-1", `1:1 number "5e-1"`},
		{"0.", `1:1 number "0."`},
		{"42e", `1:1 number "42", 1:3 name "e"`},
		{"4.2e", `1:1 number "4.2", 1:4 name "e"`},
		{"1.e3", `1:1 number "1.e3"`},
		{"1.e3", `1:1 number "1.e3"`},
		{"1e3foo", `1:1 number "1e3", 1:4 name "foo"`},
		{"1e3+", `1:1 number "1e3", 1:4 + ""`},
		{"1e3.4", `1:1 number "1e3", 1:4 number ".4"`},
		{"1e-", `1:1 number "1", 1:2 name "e", 1:3 - ""`},
		{"1e+", `1:1 number "1", 1:2 name "e", 1:3 + ""`},
		{"42`", `1:1 number "42", 1:3 <illegal> "unexpected char"`},
		{"0..", `1:1 number "0.", 1:4 <illegal> "expected digits"`},
		{".", `1:2 <illegal> "expected digits"`},

		// Misc errors
		{"&=", `1:2 <illegal> "unexpected char after '&'", 1:2 = ""`},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			l := NewLexer([]byte(test.input))
			strs := []string{}
			for {
				pos, tok, val := l.Scan()
				if tok == EOF {
					break
				}
				if tok == NUMBER {
					// Ensure ParseFloat() works, as that's what our
					// parser uses to convert
					trimmed := strings.TrimRight(val, "eE")
					_, err := strconv.ParseFloat(trimmed, 64)
					if err != nil {
						t.Fatalf("couldn't parse float: %q", val)
					}
				}
				strs = append(strs, fmt.Sprintf("%d:%d %s %q", pos.Line, pos.Column, tok, val))
			}
			output := strings.Join(strs, ", ")
			if output != test.output {
				t.Errorf("expected %q, got %q", test.output, output)
			}
		})
	}
}

func TestRegex(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{`/foo/`, `1:1 regex "foo"`},
		{`/=foo/`, `1:1 regex "=foo"`},
		{`/a\/b/`, `1:1 regex "a/b"`},
		{`/a\/\zb/`, `1:1 regex "a/\\zb"`},
		{`/a`, `1:3 <illegal> "didn't find end slash in regex"`},
		{"/a\n", `1:3 <illegal> "can't have newline in regex"`},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			l := NewLexer([]byte(test.input))
			l.Scan() // Scan first token (probably DIV)
			pos, tok, val := l.ScanRegex()
			output := fmt.Sprintf("%d:%d %s %q", pos.Line, pos.Column, tok, val)
			if output != test.output {
				t.Errorf("expected %q, got %q", test.output, output)
			}
		})
	}
}

func TestScanRegexInvalid(t *testing.T) {
	defer func() {
		r := recover()
		if message, ok := r.(string); ok {
			expected := "ScanRegex should only be called after DIV or DIV_ASSIGN token"
			if message != expected {
				t.Fatalf("expected %q, got %q", expected, message)
			}
		} else {
			t.Fatalf("expected panic of string type")
		}
	}()
	l := NewLexer([]byte("foo/"))
	l.Scan() // Scan first token (NAME foo)
	l.ScanRegex()
}

func TestHadSpace(t *testing.T) {
	tests := []struct {
		input  string
		tokens []Token
		spaces []bool
	}{
		{`foo(x)`, []Token{NAME, LPAREN, NAME, RPAREN}, []bool{false, false, false, false}},
		{`foo (x) `, []Token{NAME, LPAREN, NAME, RPAREN}, []bool{false, true, false, false}},
		{` foo ( x ) `, []Token{NAME, LPAREN, NAME, RPAREN}, []bool{true, true, true, true}},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			l := NewLexer([]byte(test.input))
			for i := 0; ; i++ {
				_, tok, _ := l.Scan()
				if tok == EOF {
					break
				}
				if tok != test.tokens[i] {
					t.Errorf("expected %s for token %d, got %s", test.tokens[i], i, tok)
				}
				if l.HadSpace() != test.spaces[i] {
					t.Errorf("expected %v for space %d, got %v", test.spaces[i], i, l.HadSpace())
				}
			}
		})
	}
}

func TestPeekByte(t *testing.T) {
	l := NewLexer([]byte("foo()"))
	b := l.PeekByte()
	if b != 'f' {
		t.Errorf("expected 'f', got %q", b)
	}
	_, tok, _ := l.Scan()
	if tok != NAME {
		t.Errorf("expected name, got %s", tok)
	}
	b = l.PeekByte()
	if b != '(' {
		t.Errorf("expected '(', got %q", b)
	}
	_, tok, _ = l.Scan()
	if tok != LPAREN {
		t.Errorf("expected (, got %s", tok)
	}
	_, tok, _ = l.Scan()
	if tok != RPAREN {
		t.Errorf("expected ), got %s", tok)
	}
	b = l.PeekByte()
	if b != 0 {
		t.Errorf("expected 0, got %q", b)
	}
}

func TestKeywordToken(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
	}{
		{"print", PRINT},
		{"split", F_SPLIT},
		{"BEGIN", BEGIN},
		{"foo", ILLEGAL},
		{"GoAWK", ILLEGAL},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tok := KeywordToken(test.name)
			if tok != test.tok {
				t.Errorf("expected %v, got %v", test.tok, tok)
			}
		})
	}
}

func TestAllTokens(t *testing.T) {
	input := "# comment line\n" +
		"+ += && = : , -- /\n/= $ @ == >= > >> ++ { [ < ( #\n" +
		"<= ~ % %= * *= !~ ! != | || ^ ^= ** **= ? } ] ) ; - -= " +
		"BEGIN break continue delete do else END exit " +
		"for function getline if in next print printf return while " +
		"atan2 close cos exp fflush gsub index int length log match rand " +
		"sin split sprintf sqrt srand sub substr system tolower toupper " +
		"x \"str\\n\" 1234\n" +
		"` ."

	strs := make([]string, 0, LAST+1)
	seen := make([]bool, LAST+1)
	l := NewLexer([]byte(input))
	for {
		_, tok, _ := l.Scan()
		strs = append(strs, tok.String())
		seen[int(tok)] = true
		if tok == EOF {
			break
		}
	}
	output := strings.Join(strs, " ")

	expected := "<newline> " +
		"+ += && = : , -- / <newline> /= $ @ == >= > >> ++ { [ < ( <newline> " +
		"<= ~ % %= * *= !~ ! != | || ^ ^= ^ ^= ? } ] ) ; - -= " +
		"BEGIN break continue delete do else END exit " +
		"for function getline if in next print printf return while " +
		"atan2 close cos exp fflush gsub index int length log match rand " +
		"sin split sprintf sqrt srand sub substr system tolower toupper " +
		"name string number <newline> " +
		"<illegal> <illegal> EOF"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}

	for i, s := range seen {
		if !s && Token(i) != CONCAT && Token(i) != REGEX {
			t.Errorf("token %s (%d) not seen", Token(i), i)
		}
	}

	l = NewLexer([]byte(`/foo/`))
	_, tok1, _ := l.Scan()
	_, tok2, val := l.ScanRegex()
	if tok1 != DIV || tok2 != REGEX || val != "foo" {
		t.Errorf(`expected / regex "foo", got %s %s %q`, tok1, tok2, val)
	}

	l = NewLexer([]byte(`/=foo/`))
	_, tok1, _ = l.Scan()
	_, tok2, val = l.ScanRegex()
	if tok1 != DIV_ASSIGN || tok2 != REGEX || val != "=foo" {
		t.Errorf(`expected /= regex "=foo", got %s %s %q`, tok1, tok2, val)
	}
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		input  string
		output string
		error  string
	}{
		{``, "", ""},
		{`foo bar`, "foo bar", ""},
		{`foo\tbar`, "foo\tbar", ""},
		{"foo\nbar", "", "can't have newline in string"},
		{`foo"`, "foo\"", ""},
		{`O'Connor`, "O'Connor", ""},
		{`foo\`, "foo\\", ""},
		// Other cases tested in TestLexer string handling.
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := Unescape(test.input)
			if err != nil {
				if err.Error() != test.error {
					t.Fatalf("expected error %q, got %q", test.error, err)
				}
			} else {
				if test.error != "" {
					t.Fatalf("expected error %q, got %q", test.error, "")
				}
				if got != test.output {
					t.Fatalf("expected %q, got %q", test.output, got)
				}
			}
		})
	}
}

func benchmarkLexer(b *testing.B, repeat int, source string) {
	fullSource := []byte(strings.Repeat(source+"\n", repeat))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(fullSource)
		for {
			_, tok, _ := l.Scan()
			if tok == EOF || tok == ILLEGAL {
				break
			}
		}
	}
}

func BenchmarkProgram(b *testing.B) {
	benchmarkLexer(b, 5, `{ print $1, ($3+$4)*$5 }`)
}

func BenchmarkNames(b *testing.B) {
	benchmarkLexer(b, 5, `x y i foobar abcdefghij0123456789 _`)
}

func BenchmarkKeywords(b *testing.B) {
	benchmarkLexer(b, 5, `BEGIN END print sub if length`)
}

func BenchmarkSimpleTokens(b *testing.B) {
	benchmarkLexer(b, 5, "\n : , { [ ( } ] ) ~ ? ; $")
}

func BenchmarkChoiceTokens(b *testing.B) {
	benchmarkLexer(b, 5, `/ /=  % %= + ++ += * ** **= *= = == ^ ^= ! != !~ < <= > >= >> && | ||`)
}

func BenchmarkNumbers(b *testing.B) {
	benchmarkLexer(b, 5, `0 1 .5 1234 1234567890 1234.56789e-50`)
}

func BenchmarkStrings(b *testing.B) {
	benchmarkLexer(b, 5, `"x" "y" "xyz" "foo" "foo bar baz" "foo\tbar\rbaz\n"`)
}

func BenchmarkRegex(b *testing.B) {
	source := `/x/ /./ /foo/ /bar/ /=equals=/ /\/\/\/\//`
	fullSource := []byte(strings.Repeat(source+" ", 5))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(fullSource)
		for {
			_, tok, _ := l.Scan()
			if tok == EOF {
				break
			}
			if tok != DIV && tok != DIV_ASSIGN {
				b.Fatalf("expected / or /=, got %s", tok)
			}
			_, tok, _ = l.ScanRegex()
			if tok != REGEX {
				b.Fatalf("expected regex, got %s", tok)
			}
		}
	}
}

func Example() {
	lexer := NewLexer([]byte(`$0 { print $1 }`))
	for {
		pos, tok, val := lexer.Scan()
		if tok == EOF {
			break
		}
		fmt.Printf("%d:%d %s %q\n", pos.Line, pos.Column, tok, val)
	}
	// Output:
	// 1:1 $ ""
	// 1:2 number "0"
	// 1:4 { ""
	// 1:6 print ""
	// 1:12 $ ""
	// 1:13 number "1"
	// 1:15 } ""
}
