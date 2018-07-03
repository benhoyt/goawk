// GoAWK lexer (tokenizer).
package lexer

import (
	"fmt"
	"unicode/utf8"
)

type Token int

const (
	ILLEGAL Token = iota
	EOF
	NEWLINE

	// Symbols
	ADD
	ADD_ASSIGN
	AND
	ASSIGN
	COLON
	COMMA
	DECR
	DIV
	DIV_ASSIGN
	DOLLAR
	EQUALS
	GTE
	GREATER
	INCR
	LBRACE
	LBRACKET
	LESS
	LPAREN
	LTE
	MATCH
	MOD
	MOD_ASSIGN
	MUL
	MUL_ASSIGN
	NOT_MATCH
	NOT
	NOT_EQUALS
	OR
	POW
	POW_ASSIGN
	QUESTION
	RBRACE
	RBRACKET
	RPAREN
	SEMICOLON
	SUB
	SUB_ASSIGN

	// Keywords
	BEGIN
	BREAK
	CONTINUE
	DELETE
	DO
	ELSE
	END
	EXIT
	FOR
	IF
	IN
	NEXT
	PRINT
	PRINTF
	RETURN
	WHILE

	// Built-in functions
	F_ATAN2
	F_COS
	F_EXP
	F_GSUB
	F_INDEX
	F_INT
	F_LENGTH
	F_LOG
	F_MATCH
	F_RAND
	F_SIN
	F_SPLIT
	F_SPRINTF
	F_SQRT
	F_SRAND
	F_SUB
	F_SUBSTR
	F_TOLOWER
	F_TOUPPER

	// Literals and names (variables and arrays)
	NAME
	NUMBER
	STRING

	LAST = STRING
)

var keywordTokens = map[string]Token{
	"BEGIN":    BEGIN,
	"break":    BREAK,
	"continue": CONTINUE,
	"delete":   DELETE,
	"do":       DO,
	"else":     ELSE,
	"END":      END,
	"exit":     EXIT,
	"for":      FOR,
	"if":       IF,
	"in":       IN,
	"next":     NEXT,
	"print":    PRINT,
	"printf":   PRINTF,
	"return":   RETURN,
	"while":    WHILE,

	"atan2":   F_ATAN2,
	"cos":     F_COS,
	"exp":     F_EXP,
	"gsub":    F_GSUB,
	"index":   F_INDEX,
	"int":     F_INT,
	"length":  F_LENGTH,
	"log":     F_LOG,
	"match":   F_MATCH,
	"rand":    F_RAND,
	"sin":     F_SIN,
	"split":   F_SPLIT,
	"sprintf": F_SPRINTF,
	"sqrt":    F_SQRT,
	"srand":   F_SRAND,
	"sub":     F_SUB,
	"substr":  F_SUBSTR,
	"tolower": F_TOLOWER,
	"toupper": F_TOUPPER,
}

var tokenNames = map[Token]string{
	ILLEGAL: "<illegal>",
	EOF:     "<eof>",
	NEWLINE: "<newline>",

	ADD:        "+",
	ADD_ASSIGN: "+=",
	AND:        "&&",
	ASSIGN:     "=",
	COLON:      ":",
	COMMA:      ",",
	DECR:       "--",
	DIV:        "/",
	DIV_ASSIGN: "/=",
	DOLLAR:     "$",
	EQUALS:     "==",
	GTE:        ">=",
	GREATER:    ">",
	INCR:       "++",
	LBRACE:     "{",
	LBRACKET:   "[",
	LESS:       "<",
	LPAREN:     "(",
	LTE:        "<=",
	MATCH:      "~",
	MOD:        "%",
	MOD_ASSIGN: "%=",
	MUL:        "*",
	MUL_ASSIGN: "*=",
	NOT_MATCH:  "!~",
	NOT:        "!",
	NOT_EQUALS: "!=",
	OR:         "||",
	POW:        "^",
	POW_ASSIGN: "^=",
	QUESTION:   "?",
	RBRACE:     "}",
	RBRACKET:   "]",
	RPAREN:     ")",
	SEMICOLON:  ";",
	SUB:        "-",
	SUB_ASSIGN: "-=",

	BEGIN:    "BEGIN",
	BREAK:    "break",
	CONTINUE: "continue",
	DELETE:   "delete",
	DO:       "do",
	ELSE:     "else",
	END:      "END",
	EXIT:     "exit",
	FOR:      "for",
	IF:       "if",
	IN:       "in",
	NEXT:     "next",
	PRINT:    "print",
	PRINTF:   "printf",
	RETURN:   "return",
	WHILE:    "while",

	F_ATAN2:   "atan2",
	F_COS:     "cos",
	F_EXP:     "exp",
	F_GSUB:    "gsub",
	F_INDEX:   "index",
	F_INT:     "int",
	F_LENGTH:  "length",
	F_LOG:     "log",
	F_MATCH:   "match",
	F_RAND:    "rand",
	F_SIN:     "sin",
	F_SPLIT:   "split",
	F_SPRINTF: "sprintf",
	F_SQRT:    "sqrt",
	F_SRAND:   "srand",
	F_SUB:     "sub",
	F_SUBSTR:  "substr",
	F_TOLOWER: "tolower",
	F_TOUPPER: "toupper",

	NAME:   "<name>",
	NUMBER: "<number>",
	STRING: "<string>",
}

func (t Token) String() string {
	return tokenNames[t]
}

type Position struct {
	Line   int
	Column int
}

type Lexer struct {
	src      []byte
	offset   int
	ch       rune
	errorMsg string
	pos      Position
	nextPos  Position
}

func NewLexer(src []byte) *Lexer {
	l := &Lexer{src: src}
	l.nextPos.Line = 1
	l.nextPos.Column = 1
	l.next()
	return l
}

func (l *Lexer) next() {
	l.pos = l.nextPos
	ch, size := utf8.DecodeRune(l.src[l.offset:])
	if size == 0 {
		l.ch = -1
		return
	}
	if ch == utf8.RuneError {
		l.ch = -1
		l.errorMsg = fmt.Sprintf("invalid UTF-8 byte 0x%02x", l.src[l.offset])
		return
	}
	if ch == '\n' {
		l.nextPos.Line++
		l.nextPos.Column = 1
	} else {
		l.nextPos.Column++
	}
	l.ch = ch
	l.offset += size
}

func (l *Lexer) skipWhite() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.next()
	}
}

func isNameStart(ch rune) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func (l *Lexer) twoSymbol(secondChar rune, oneChar, twoChar Token) Token {
	if l.ch == secondChar {
		l.next()
		return twoChar
	}
	return oneChar
}

func (l *Lexer) Scan() (Position, Token, string) {
	l.skipWhite()
	if l.ch < 0 {
		if l.errorMsg != "" {
			return l.pos, ILLEGAL, l.errorMsg
		}
		return l.pos, EOF, ""
	}

	pos := l.pos
	tok := ILLEGAL
	val := ""

	ch := l.ch
	l.next()

	// Names: keywords and functions
	if isNameStart(ch) {
		runes := []rune{ch}
		for isNameStart(l.ch) || (l.ch >= '0' && l.ch <= '9') {
			runes = append(runes, l.ch)
			l.next()
		}
		name := string(runes)
		tok, isKeyword := keywordTokens[name]
		if !isKeyword {
			tok = NAME
			val = name
		}
		return pos, tok, val
	}

	switch ch {
	case '\n':
		tok = NEWLINE
	case ':':
		tok = COLON
	case ',':
		tok = COMMA
	case '/':
		switch l.ch {
		case '=':
			l.next()
			tok = DIV_ASSIGN
		case ' ', '\t':
			tok = DIV
		default:
			// TODO: this isn't really correct
			// tok, val := l.parseString('/')
			return pos, STRING, "TODO"
		}
	case '{':
		tok = LBRACE
	case '[':
		tok = LBRACKET
	case '(':
		tok = LPAREN
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
	case '%':
		tok = l.twoSymbol('=', MOD, MOD_ASSIGN)
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
	case '}':
		tok = RBRACE
	case ']':
		tok = RBRACKET
	case ')':
		tok = RPAREN
	case '*':
		tok = l.twoSymbol('=', MUL, MUL_ASSIGN)
	case '=':
		tok = l.twoSymbol('=', ASSIGN, EQUALS)
	case '^':
		tok = l.twoSymbol('=', POW, POW_ASSIGN)
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
	case '<':
		tok = l.twoSymbol('=', LESS, LTE)
	case '>':
		tok = l.twoSymbol('=', GREATER, GTE)
	case '~':
		tok = MATCH
	case '?':
		tok = QUESTION
	case ';':
		tok = SEMICOLON
	case '$':
		tok = DOLLAR
	case '&':
		switch l.ch {
		case '&':
			l.next()
			tok = AND
		default:
			tok = ILLEGAL
			val = fmt.Sprintf("unexpected %c after &", ch)
		}
	case '|':
		switch l.ch {
		case '|':
			l.next()
			tok = OR
		default:
			tok = ILLEGAL
			val = fmt.Sprintf("unexpected %c after |", ch)
		}
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// TODO: handle floats
		runes := []rune{ch}
		for l.ch >= '0' && l.ch <= '9' {
			runes = append(runes, l.ch)
			l.next()
		}
		tok = NUMBER
		val = string(runes)
	case '"':
		runes := []rune{}
		for l.ch != '"' {
			c := l.ch
			if c < 0 {
				return pos, ILLEGAL, "didn't find end quote in string"
			}
			if c == '\r' || c == '\n' {
				return pos, ILLEGAL, "can't have newline in string"
			}
			if c == '\\' {
				l.next()
				switch l.ch {
				case '"', '\\':
					c = l.ch
				case 't':
					c = '\t'
				case 'r':
					c = '\r'
				case 'n':
					c = '\n'
				default:
					return pos, ILLEGAL, fmt.Sprintf("invalid string escape \\%c", l.ch)
				}
			}
			runes = append(runes, c)
			l.next()
		}
		l.next()
		tok = STRING
		val = string(runes)
	default:
		tok = ILLEGAL
		val = fmt.Sprintf("unexpected %c", ch)
	}
	return pos, tok, val
}
