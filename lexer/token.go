// Lexer tokens

package lexer

// Token is the type of a single token.
type Token int

const (
	ILLEGAL Token = iota
	EOF
	NEWLINE
	CONCAT // Not really a token, but used as an operator

	// Symbols

	ADD
	ADD_ASSIGN
	AND
	APPEND
	ASSIGN
	AT
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
	PIPE
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
	FUNCTION
	GETLINE
	IF
	IN
	NEXT
	NEXTFILE
	PRINT
	PRINTF
	RETURN
	WHILE

	// Literals and names (variables and arrays)

	NAME
	NUMBER
	STRING
	REGEX

	LAST = REGEX
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
	"function": FUNCTION,
	"getline":  GETLINE,
	"if":       IF,
	"in":       IN,
	"next":     NEXT,
	"nextfile": NEXTFILE,
	"print":    PRINT,
	"printf":   PRINTF,
	"return":   RETURN,
	"while":    WHILE,
}

// KeywordToken returns the token associated with the given keyword
// string, or ILLEGAL if given name is not a keyword.
func KeywordToken(name string) Token {
	return keywordTokens[name]
}

var tokenNames = map[Token]string{
	ILLEGAL: "<illegal>",
	EOF:     "EOF",
	NEWLINE: "<newline>",
	CONCAT:  "<concat>",

	ADD:        "+",
	ADD_ASSIGN: "+=",
	AND:        "&&",
	APPEND:     ">>",
	ASSIGN:     "=",
	AT:         "@",
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
	PIPE:       "|",
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
	FUNCTION: "function",
	GETLINE:  "getline",
	IF:       "if",
	IN:       "in",
	NEXT:     "next",
	NEXTFILE: "nextfile",
	PRINT:    "print",
	PRINTF:   "printf",
	RETURN:   "return",
	WHILE:    "while",

	NAME:   "name",
	NUMBER: "number",
	STRING: "string",
	REGEX:  "regex",
}

// String returns the string name of this token.
func (t Token) String() string {
	return tokenNames[t]
}
