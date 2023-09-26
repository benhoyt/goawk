// Package lexer is an AWK lexer (tokenizer).
//
// The lexer turns a string of AWK source code into a stream of
// tokens for parsing.
//
// To tokenize some source, create a new lexer with NewLexer(src) and
// then call Scan() until the token type is EOF or ILLEGAL.
package lexer

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// Lexer tokenizes a byte string of AWK source code. Use NewLexer to
// actually create a lexer, and Scan() or ScanRegex() to get tokens.
type Lexer struct {
	src      []byte
	offset   int
	ch       byte
	pos      Position
	nextPos  Position
	hadSpace bool
	lastTok  Token
}

// Position stores the source line and column where a token starts.
type Position struct {
	// Line number of the token (starts at 1).
	Line int
	// Column on the line (starts at 1). Note that this is the byte
	// offset into the line, not rune offset.
	Column int
}

// String returns the position in "line:col" format.
func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// NewLexer creates a new lexer that will tokenize the given source
// code. See the module-level example for a working example.
func NewLexer(src []byte) *Lexer {
	l := &Lexer{src: src}
	l.nextPos.Line = 1
	l.nextPos.Column = 1
	l.next()
	return l
}

// HadSpace returns true if the previously-scanned token had
// whitespace before it. Used by the parser because when calling a
// user-defined function the grammar doesn't allow a space between
// the function name and the left parenthesis.
func (l *Lexer) HadSpace() bool {
	return l.hadSpace
}

// Scan scans the next token and returns its position (line/column),
// token value (one of the uppercase token constants), and the
// string value of the token. For most tokens, the token value is
// empty. For NAME, NUMBER, STRING, and REGEX tokens, it's the
// token's value. For an ILLEGAL token, it's the error message.
func (l *Lexer) Scan() (Position, Token, string) {
	pos, tok, val := l.scan()
	l.lastTok = tok
	return pos, tok, val
}

// Does the real work of scanning. Scan() wraps this to more easily
// set lastTok.
func (l *Lexer) scan() (Position, Token, string) {
	// Skip whitespace (except newline, which is a token)
	l.hadSpace = false
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' || l.ch == '\\' {
		l.hadSpace = true
		if l.ch == '\\' {
			l.next()
			if l.ch == '\r' {
				l.next()
			}
			if l.ch != '\n' {
				return l.pos, ILLEGAL, "expected \\n after \\ line continuation"
			}
		}
		l.next()
	}
	if l.ch == '#' {
		// Skip comment till end of line
		l.next()
		for l.ch != '\n' && l.ch != 0 {
			l.next()
		}
	}
	if l.ch == 0 {
		// l.next() reached end of input
		return l.pos, EOF, ""
	}

	pos := l.pos
	tok := ILLEGAL
	val := ""

	ch := l.ch
	l.next()

	// Names: keywords and functions
	if isNameStart(ch) {
		start := l.offset - 2
		for isNameStart(l.ch) || isDigit(l.ch) {
			l.next()
		}
		name := string(l.src[start : l.offset-1])
		tok := KeywordToken(name)
		if tok == ILLEGAL {
			tok = NAME
			val = name
		}
		return pos, tok, val
	}

	// These are ordered by my guess at frequency of use. Should run
	// through a corpus of real AWK programs to determine actual
	// frequency.
	switch ch {
	case '$':
		tok = DOLLAR
	case '@':
		tok = AT
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
		// Avoid make/append and use l.offset directly for performance
		start := l.offset - 2
		gotDigit := false
		if ch != '.' {
			gotDigit = true
			for isDigit(l.ch) {
				l.next()
			}
			if l.ch == '.' {
				l.next()
			}
		}
		for isDigit(l.ch) {
			gotDigit = true
			l.next()
		}
		if !gotDigit {
			return l.pos, ILLEGAL, "expected digits"
		}
		if l.ch == 'e' || l.ch == 'E' {
			l.next()
			gotSign := false
			if l.ch == '+' || l.ch == '-' {
				gotSign = true
				l.next()
			}
			gotDigit = false
			for isDigit(l.ch) {
				l.next()
				gotDigit = true
			}
			// Per awk/gawk, "1e" is allowed and parsed as "1 e" (with "e"
			// considered a variable). "1e+" is parsed as "1e + ...".
			if !gotDigit {
				if gotSign {
					l.unread() // unread the '+' or '-'
				}
				l.unread() // unread the 'e' or 'E'
			}
		}
		tok = NUMBER
		val = string(l.src[start : l.offset-1])
	case '{':
		tok = LBRACE
	case '}':
		tok = RBRACE
	case '=':
		tok = l.choice('=', ASSIGN, EQUALS)
	case '<':
		tok = l.choice('=', LESS, LTE)
	case '>':
		switch l.ch {
		case '=':
			l.next()
			tok = GTE
		case '>':
			l.next()
			tok = APPEND
		default:
			tok = GREATER
		}
	case '"', '\'':
		// Note: POSIX awk spec doesn't allow single-quoted strings,
		// but this helps with quoting, especially on Windows
		// where the shell quote character is " (double quote).
		s, err := parseString(ch, func() byte { return l.ch }, l.next)
		if err != nil {
			return l.pos, ILLEGAL, err.Error()
		}
		if l.ch != ch {
			return l.pos, ILLEGAL, "didn't find end quote in string"
		}
		l.next()
		tok = STRING
		val = s
	case '(':
		tok = LPAREN
	case ')':
		tok = RPAREN
	case ',':
		tok = COMMA
	case ';':
		tok = SEMICOLON
	case '+':
		switch l.ch {
		case '+':
			l.next()
			tok = INCR
		case '=':
			l.next()
			tok = ADD_ASSIGN
		default:
			tok = ADD
		}
	case '-':
		switch l.ch {
		case '-':
			l.next()
			tok = DECR
		case '=':
			l.next()
			tok = SUB_ASSIGN
		default:
			tok = SUB
		}
	case '*':
		switch l.ch {
		case '*':
			l.next()
			tok = l.choice('=', POW, POW_ASSIGN)
		case '=':
			l.next()
			tok = MUL_ASSIGN
		default:
			tok = MUL
		}
	case '/':
		tok = l.choice('=', DIV, DIV_ASSIGN)
	case '%':
		tok = l.choice('=', MOD, MOD_ASSIGN)
	case '[':
		tok = LBRACKET
	case ']':
		tok = RBRACKET
	case '\n':
		tok = NEWLINE
	case '^':
		tok = l.choice('=', POW, POW_ASSIGN)
	case '!':
		switch l.ch {
		case '=':
			l.next()
			tok = NOT_EQUALS
		case '~':
			l.next()
			tok = NOT_MATCH
		default:
			tok = NOT
		}
	case '~':
		tok = MATCH
	case '?':
		tok = QUESTION
	case ':':
		tok = COLON
	case '&':
		tok = l.choice('&', ILLEGAL, AND)
		if tok == ILLEGAL {
			return l.pos, ILLEGAL, "unexpected char after '&'"
		}
	case '|':
		tok = l.choice('|', PIPE, OR)
	default:
		tok = ILLEGAL
		val = "unexpected char"
	}
	return pos, tok, val
}

// ScanRegex parses an AWK regular expression in /slash/ syntax. The
// AWK grammar has somewhat special handling of regex tokens, so the
// parser can only call this after a DIV or DIV_ASSIGN token has just
// been scanned.
func (l *Lexer) ScanRegex() (Position, Token, string) {
	pos, tok, val := l.scanRegex()
	l.lastTok = tok
	return pos, tok, val
}

// Does the real work of scanning a regex. ScanRegex() wraps this to
// more easily set lastTok.
func (l *Lexer) scanRegex() (Position, Token, string) {
	pos := l.pos
	chars := make([]byte, 0, 32) // most won't require heap allocation
	switch l.lastTok {
	case DIV:
		// Regex after '/' (the usual case)
		pos.Column -= 1
	case DIV_ASSIGN:
		// Regex after '/=' (happens when regex starts with '=')
		pos.Column -= 2
		chars = append(chars, '=')
	default:
		panic("ScanRegex should only be called after DIV or DIV_ASSIGN token")
	}
	for l.ch != '/' {
		c := l.ch
		if c == 0 {
			return l.pos, ILLEGAL, "didn't find end slash in regex"
		}
		if c == '\r' || c == '\n' {
			return l.pos, ILLEGAL, "can't have newline in regex"
		}
		if c == '\\' {
			l.next()
			if l.ch != '/' {
				chars = append(chars, '\\')
			}
			c = l.ch
		}
		chars = append(chars, c)
		l.next()
	}
	l.next()
	return pos, REGEX, string(chars)
}

// Load the next character into l.ch (or 0 on end of input) and update
// line and column position.
func (l *Lexer) next() {
	l.pos = l.nextPos
	if l.offset >= len(l.src) {
		// For last character, move offset 1 past the end as it
		// simplifies offset calculations in NAME and NUMBER
		if l.ch != 0 {
			l.ch = 0
			l.offset++
			l.nextPos.Column++
		}
		return
	}
	ch := l.src[l.offset]
	if ch == '\n' {
		l.nextPos.Line++
		l.nextPos.Column = 1
	} else if ch != '\r' {
		l.nextPos.Column++
	}
	l.ch = ch
	l.offset++
}

// Un-read the character just scanned (doesn't handle line boundaries).
func (l *Lexer) unread() {
	l.offset--
	l.pos.Column--
	l.nextPos.Column--
	l.ch = l.src[l.offset-1]
}

func isNameStart(ch byte) bool {
	return ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

// Return the hex digit 0-15 corresponding to the given ASCII byte,
// or -1 if it's not a valid hex digit.
func hexDigit(ch byte) int {
	switch {
	case isDigit(ch):
		return int(ch - '0')
	case ch >= 'a' && ch <= 'f':
		return int(ch - 'a' + 10)
	case ch >= 'A' && ch <= 'F':
		return int(ch - 'A' + 10)
	default:
		return -1
	}
}

func (l *Lexer) choice(ch byte, one, two Token) Token {
	if l.ch == ch {
		l.next()
		return two
	}
	return one
}

// PeekByte returns the next unscanned byte; used when parsing
// "getline lvalue" expressions. Returns 0 at end of input.
func (l *Lexer) PeekByte() byte {
	return l.ch
}

// Unescape unescapes the backslash escapes in s (which shouldn't include the
// surrounding quotes) and returns the unquoted string. It's intended for use
// when unescaping command line var=value assignments, as required by the
// POSIX AWK spec.
func Unescape(s string) (string, error) {
	i := 0
	ch := func() byte {
		if i >= len(s) {
			return 0
		}
		return s[i]
	}
	next := func() {
		i++
	}
	return parseString(0, ch, next)
}

// Parses a string ending with given quote character (not parsed). The ch
// function returns the current character (or 0 at the end); the next function
// moves forward one character.
func parseString(quote byte, ch func() byte, next func()) (string, error) {
	chars := make([]byte, 0, 32) // most strings won't require heap allocation
	for {
		c := ch()
		if c == quote || c == 0 {
			break
		}
		if c == '\r' || c == '\n' {
			return "", errors.New("can't have newline in string")
		}
		if c != '\\' {
			// Normal, non-escaped character
			chars = append(chars, c)
			next()
			continue
		}
		// Escape sequence, skip over \ and process
		next()
		switch ch() {
		case 'n':
			c = '\n'
			next()
		case 't':
			c = '\t'
			next()
		case 'r':
			c = '\r'
			next()
		case 'a':
			c = '\a'
			next()
		case 'b':
			c = '\b'
			next()
		case 'f':
			c = '\f'
			next()
		case 'v':
			c = '\v'
			next()
		case 'x':
			// Hex byte of one or two hex digits
			next()
			digit := hexDigit(ch())
			if digit < 0 {
				return "", errors.New("1 or 2 hex digits expected")
			}
			c = byte(digit)
			next()
			digit = hexDigit(ch())
			if digit >= 0 {
				c = c*16 + byte(digit)
				next()
			}
		case 'u':
			// Hex Unicode character of 1-8 digits
			next()
			r := hexDigit(ch())
			if r < 0 {
				return "", errors.New("1-8 hex digits expected")
			}
			next()
			for i := 0; i < 7; i++ {
				digit := hexDigit(ch())
				if digit < 0 {
					break
				}
				next()
				r = r*16 + digit
			}
			if !utf8.ValidRune(rune(r)) {
				return "", errors.New("invalid Unicode character")
			}
			runeBytes := make([]byte, utf8.UTFMax)
			n := utf8.EncodeRune(runeBytes, rune(r))
			chars = append(chars, runeBytes[:n]...)
			continue
		case '0', '1', '2', '3', '4', '5', '6', '7':
			// Octal byte of 1-3 octal digits
			c = ch() - '0'
			next()
			for i := 0; i < 2 && ch() >= '0' && ch() <= '7'; i++ {
				c = c*8 + ch() - '0'
				next()
			}
		default:
			// Any other escape character is just the char
			// itself, eg: "\z" is just "z".
			c = ch()
			if c == 0 {
				// Expect backslash right at the end of the string, which is
				// interpreted as a literal backslash (only for Unescape).
				c = '\\'
			}
			next()
		}
		chars = append(chars, c)
	}
	return string(chars), nil
}
