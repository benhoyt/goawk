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
	PRINT
	PRINTF
	RETURN
	WHILE

	// Syntax sugar
	VAR        // indicate an array variable declared as JSON array or object
	F_JARRAY   // [a, b, c]  -> _jarray_(a, b, c) -> _jobject_(1,a, 2,b, 3,c)
	F_JOBJECT  // {a:1, b:2} -> _jobject_(a,1, b,2)

	// Built-in functions
	F_ATAN2
	F_CLOSE
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
	F_SYSTEM
	F_TOLOWER
	F_TOUPPER

	// Literals and names (variables and arrays)
	NAME
	NUMBER
	STRING
	TRUE
	FALSE
	REGEX

	LAST       = REGEX
	FIRST_FUNC = F_ATAN2
	LAST_FUNC  = F_TOUPPER
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
	"print":    PRINT,
	"printf":   PRINTF,
	"return":   RETURN,
	"while":    WHILE,
	"true":     TRUE,
	"false":    FALSE,
	"var":      VAR,

	"atan2":   F_ATAN2,
	"close":   F_CLOSE,
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
	"system":  F_SYSTEM,
	"tolower": F_TOLOWER,
	"toupper": F_TOUPPER,
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
	PRINT:    "print",
	PRINTF:   "printf",
	RETURN:   "return",
	WHILE:    "while",
	TRUE:     "true",
	FALSE:    "false",
	VAR:      "var",

	F_JARRAY:  "_jarray_",
	F_JOBJECT: "_jobject_",

	F_ATAN2:   "atan2",
	F_CLOSE:   "close",
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
	F_SYSTEM:  "system",
	F_TOLOWER: "tolower",
	F_TOUPPER: "toupper",

	NAME:   "name",
	NUMBER: "number",
	STRING: "string",
	REGEX:  "regex",
}

// String returns the string name of this token.
func (t Token) String() string {
	return tokenNames[t]
}
