// Test GoAWK Lexer

package lexer_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	. "github.com/benhoyt/goawk/lexer"
)

// TODO: add other lexer tests

func TestNumber(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"0", "1:1 number 0"},
		{"9", "1:1 number 9"},
		{" 0 ", "1:2 number 0"},
		{"\n  1", "1:1 <newline> , 2:3 number 1"},
		{"1234", "1:1 number 1234"},
		{".5", "1:1 number .5"},
		{".5e1", "1:1 number .5e1"},
		{"5e+1", "1:1 number 5e+1"},
		{"5e-1", "1:1 number 5e-1"},
		{"0.", "1:1 number 0."},
		{"42e", "1:1 number 42e"},
		{"4.2e", "1:1 number 4.2e"},
		{"1.e3", "1:1 number 1.e3"},
		{"1.e3", "1:1 number 1.e3"},
		{"1e3foo", "1:1 number 1e3, 1:4 name foo"},
		{"1e3+", "1:1 number 1e3, 1:4 + "},
		{"1e3.4", "1:1 number 1e3, 1:4 number .4"},
		{"42@", "1:1 number 42, 1:3 <illegal> unexpected '@'"},
		{"0..", "1:1 number 0., 1:4 <illegal> expected digits"},
		{".", "1:2 <illegal> expected digits"},
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
				strs = append(strs, fmt.Sprintf("%d:%d %s %s", pos.Line, pos.Column, tok, val))
			}
			output := strings.Join(strs, ", ")
			if output != test.output {
				t.Errorf("expected %q, got %q", test.output, output)
			}
		})
	}
}

func TestStringMethod(t *testing.T) {
	input := "# comment line\n" +
		"+ += && = : , -- /\n/= $ == >= > >> ++ { [ < ( #\n" +
		"<= ~ % %= * *= !~ ! != | || ^ ^= ** **= ? } ] ) ; - -= " +
		"BEGIN break continue delete do else END exit " +
		"for function getline if in next print printf return while " +
		"atan2 close cos exp gsub index int length log match rand " +
		"sin split sprintf sqrt srand sub substr system tolower toupper " +
		"x \"str\\n\" 1234\n" +
		"@ ."

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
		"+ += && = : , -- / <newline> /= $ == >= > >> ++ { [ < ( <newline> " +
		"<= ~ % %= * *= !~ ! != | || ^ ^= ^ ^= ? } ] ) ; - -= " +
		"BEGIN break continue delete do else END exit " +
		"for function <illegal> if in next print printf return while " +
		"atan2 close cos exp gsub index int length log match rand " +
		"sin split sprintf sqrt srand sub substr system tolower toupper " +
		"name string number <newline> " +
		"<illegal> <illegal> EOF"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}

	for i, s := range seen {
		if !s && Token(i) != CONCAT && Token(i) != GETLINE && Token(i) != REGEX {
			t.Errorf("token %s (%d) not seen", Token(i), i)
		}
	}

	l = NewLexer([]byte(`/foo/`))
	_, tok1, _ := l.Scan()
	_, tok2, val := l.ScanRegex()
	if tok1 != Token(DIV) || tok2 != Token(REGEX) || val != "foo" {
		t.Errorf(`expected / regex "foo", got %s %s %q`, tok1, tok2, val)
	}

	l = NewLexer([]byte(`/=foo/`))
	_, tok1, _ = l.Scan()
	_, tok2, val = l.ScanRegex()
	if tok1 != Token(DIV_ASSIGN) || tok2 != Token(REGEX) || val != "=foo" {
		t.Errorf(`expected /= regex "=foo", got %s %s %q`, tok1, tok2, val)
	}
}
