// Package parser is an AWK parser and abstract syntax tree.
//
// Use the ParseProgram function to parse an AWK program, and then
// give the result to one of the interp.Exec* functions to execute it.
//
package parser

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/benhoyt/goawk/lexer"
)

// ParseError is the type of error returned by the parse functions
// (actually *ParseError).
type ParseError struct {
	Position Position
	Message  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at %d:%d: %s", e.Position.Line, e.Position.Column, e.Message)
}

// ParseProgram parses an entire AWK program, returning the *Program
// abstract syntax tree or a *ParseError on error.
func ParseProgram(src []byte) (prog *Program, err error) {
	defer func() {
		if r := recover(); r != nil {
			// Convert to ParseError or re-panic
			err = r.(*ParseError)
		}
	}()
	lexer := NewLexer(src)
	// TODO: put this in a newParser() function?
	p := parser{lexer: lexer}
	p.globals = make(map[string]int)
	p.next()
	return p.program(), nil
}

type parser struct {
	lexer       *Lexer
	pos         Position
	tok         Token
	val         string
	progState   progState
	loopCount   int
	inFunction  bool
	locals      map[string]int
	arrayParams map[string]bool
	globals     map[string]int
}

type progState int

const (
	beginState progState = iota
	endState
	actionState
)

// Parse an entire AWK program.
func (p *parser) program() *Program {
	prog := &Program{}
	prog.Functions = make(map[string]Function)
	p.optionalNewlines()
	for p.tok != EOF {
		switch p.tok {
		case BEGIN:
			p.next()
			p.progState = beginState
			prog.Begin = append(prog.Begin, p.stmtsBrace())
		case END:
			p.next()
			p.progState = endState
			prog.End = append(prog.End, p.stmtsBrace())
		case FUNCTION:
			function := p.function(prog.Functions)
			prog.Functions[function.Name] = function
		default:
			p.progState = actionState
			// Allow empty pattern, normal pattern, or range pattern
			pattern := []Expr{}
			if !p.matches(LBRACE, EOF) {
				pattern = append(pattern, p.expr())
			}
			if !p.matches(LBRACE, EOF, NEWLINE) {
				p.commaNewlines()
				pattern = append(pattern, p.expr())
			}
			// Or an empty action (equivalent to { print $0 })
			action := Action{pattern, nil}
			if p.tok == LBRACE {
				action.Stmts = p.stmtsBrace()
			}
			prog.Actions = append(prog.Actions, action)
		}
		p.optionalNewlines()
	}
	prog.Globals = p.globals
	return prog
}

func (p *parser) stmts() Stmts {
	switch p.tok {
	case SEMICOLON:
		p.next()
		return nil
	case LBRACE:
		return p.stmtsBrace()
	default:
		return []Stmt{p.stmt()}
	}
}

func (p *parser) stmtsBrace() Stmts {
	p.expect(LBRACE)
	p.optionalNewlines()
	ss := []Stmt{}
	for p.tok != RBRACE && p.tok != EOF {
		ss = append(ss, p.stmt())
	}
	p.expect(RBRACE)
	return ss
}

func (p *parser) simpleStmt() Stmt {
	switch p.tok {
	case PRINT, PRINTF:
		op := p.tok
		p.next()
		args := p.exprList(p.printExpr)
		if len(args) == 1 {
			if m, ok := args[0].(*MultiExpr); ok {
				args = m.Exprs
			}
		}
		redirect := ILLEGAL
		var dest Expr
		if p.matches(GREATER, APPEND, PIPE) {
			redirect = p.tok
			p.next()
			dest = p.expr()
		}
		if op == PRINT {
			return &PrintStmt{args, redirect, dest}
		} else {
			return &PrintfStmt{args, redirect, dest}
		}
	case DELETE:
		p.next()
		array := p.val
		p.arrayParam(array)
		p.expect(NAME)
		p.expect(LBRACKET)
		index := p.exprList(p.expr)
		p.expect(RBRACKET)
		return &DeleteStmt{array, index}
	case IF, FOR, WHILE, DO, BREAK, CONTINUE, NEXT, EXIT:
		panic(p.error("expected print/printf, delete, or expression"))
	default:
		return &ExprStmt{p.expr()}
	}
}

func (p *parser) stmt() Stmt {
	for p.matches(SEMICOLON, NEWLINE) {
		p.next()
	}
	var s Stmt
	switch p.tok {
	case IF:
		p.next()
		p.expect(LPAREN)
		cond := p.expr()
		p.expect(RPAREN)
		p.optionalNewlines()
		body := p.stmts()
		p.optionalNewlines()
		var elseBody Stmts
		if p.tok == ELSE {
			p.next()
			p.optionalNewlines()
			elseBody = p.stmts()
		}
		s = &IfStmt{cond, body, elseBody}
	case FOR:
		// Parse for statement, either "for in" or C-like for loop.
		//
		//     FOR LPAREN NAME IN NAME RPAREN NEWLINE* stmts |
		//     FOR LPAREN [simpleStmt] SEMICOLON NEWLINE*
		//                [expr] SEMICOLON NEWLINE*
		//                [simpleStmt] RPAREN NEWLINE* stmts
		//
		p.next()
		p.expect(LPAREN)
		var pre Stmt
		if p.tok != SEMICOLON {
			pre = p.simpleStmt()
		}
		if pre != nil && p.tok == RPAREN {
			// Match: for (var in array) body
			p.next()
			p.optionalNewlines()
			exprStmt, ok := pre.(*ExprStmt)
			if !ok {
				panic(p.error("expected 'for (var in array) ...'"))
			}
			inExpr, ok := (exprStmt.Expr).(*InExpr)
			if !ok {
				panic(p.error("expected 'for (var in array) ...'"))
			}
			if len(inExpr.Index) != 1 {
				panic(p.error("expected 'for (var in array) ...'"))
			}
			varExpr, ok := (inExpr.Index[0]).(*VarExpr)
			if !ok {
				panic(p.error("expected 'for (var in array) ...'"))
			}
			p.arrayParam(inExpr.Array)
			body := p.loopStmts()
			index := p.getVarIndex(varExpr.Name)
			s = &ForInStmt{index, varExpr.Name, inExpr.Array, body}
		} else {
			// Match: for ([pre]; [cond]; [post]) body
			p.expect(SEMICOLON)
			p.optionalNewlines()
			var cond Expr
			if p.tok != SEMICOLON {
				cond = p.expr()
			}
			p.expect(SEMICOLON)
			p.optionalNewlines()
			var post Stmt
			if p.tok != RPAREN {
				post = p.simpleStmt()
			}
			p.expect(RPAREN)
			p.optionalNewlines()
			body := p.loopStmts()
			s = &ForStmt{pre, cond, post, body}
		}
	case WHILE:
		p.next()
		p.expect(LPAREN)
		cond := p.expr()
		p.expect(RPAREN)
		p.optionalNewlines()
		body := p.loopStmts()
		s = &WhileStmt{cond, body}
	case DO:
		p.next()
		p.optionalNewlines()
		body := p.loopStmts()
		p.expect(WHILE)
		p.expect(LPAREN)
		cond := p.expr()
		p.expect(RPAREN)
		s = &DoWhileStmt{body, cond}
	case BREAK:
		if p.loopCount == 0 {
			panic(p.error("break must be inside a loop body"))
		}
		p.next()
		s = &BreakStmt{}
	case CONTINUE:
		if p.loopCount == 0 {
			panic(p.error("continue must be inside a loop body"))
		}
		p.next()
		s = &ContinueStmt{}
	case NEXT:
		if p.progState != actionState {
			panic(p.error("next can't be in BEGIN or END"))
		}
		p.next()
		s = &NextStmt{}
	case EXIT:
		p.next()
		var status Expr
		if !p.matches(NEWLINE, SEMICOLON, RBRACE) {
			status = p.expr()
		}
		s = &ExitStmt{status}
	case RETURN:
		if !p.inFunction {
			panic(p.error("return must be inside a function"))
		}
		p.next()
		var value Expr
		if !p.matches(NEWLINE, SEMICOLON, RBRACE) {
			value = p.expr()
		}
		s = &ReturnStmt{value}
	default:
		s = p.simpleStmt()
	}
	for p.matches(NEWLINE, SEMICOLON) {
		p.next()
	}
	return s
}

func (p *parser) loopStmts() Stmts {
	p.loopCount++
	ss := p.stmts()
	p.loopCount--
	return ss
}

func (p *parser) function(functions map[string]Function) Function {
	if p.inFunction {
		// Should never actually get here (FUNCTION token is only
		// handled at the top level), but just in case.
		panic(p.error("can't nest functions"))
	}
	p.inFunction = true
	p.next()
	name := p.val
	if _, ok := functions[name]; ok {
		panic(p.error("function %q already defined", name))
	}
	p.expect(NAME)
	p.expect(LPAREN)
	first := true
	params := make([]string, 0, 10) // TODO: arbitrary, re-use allocation?
	for p.tok != RPAREN {
		if !first {
			p.commaNewlines()
		}
		first = false
		param := p.val
		p.expect(NAME)
		params = append(params, param)
	}
	p.locals = make(map[string]int, len(params))
	for i, param := range params {
		p.locals[param] = -i - 1
	}
	p.arrayParams = make(map[string]bool, len(params))
	p.expect(RPAREN)
	p.optionalNewlines()

	body := p.stmtsBrace()

	arrays := make([]bool, len(params))
	for i, name := range params {
		_, isArray := p.arrayParams[name]
		arrays[i] = isArray
	}

	p.arrayParams = nil
	p.locals = nil
	p.inFunction = false
	return Function{name, params, arrays, body}
}

func (p *parser) arrayParam(name string) {
	if p.inFunction {
		p.arrayParams[name] = true
	}
}

func (p *parser) exprList(parse func() Expr) []Expr {
	exprs := []Expr{}
	first := true
	for !p.matches(NEWLINE, SEMICOLON, RBRACE, RBRACKET, RPAREN, GREATER, PIPE, APPEND) {
		if !first {
			p.commaNewlines()
		}
		first = false
		exprs = append(exprs, parse())
	}
	return exprs
}

// Parse a single expression.
func (p *parser) expr() Expr      { return p.getLine() }
func (p *parser) printExpr() Expr { return p._assign(p.printCond) }

// Parse an "expr | getline [var]" expression:
//
//     assign [PIPE GETLINE [NAME]]
//
func (p *parser) getLine() Expr {
	expr := p._assign(p.cond)
	if p.tok == PIPE {
		p.next()
		p.expect(GETLINE)
		name := ""
		index := 0
		if p.tok == NAME {
			name = p.val
			index = p.getVarIndex(name)
			p.next()
		}
		return &GetlineExpr{expr, index, name, nil}
	}
	return expr
}

// Parse an = assignment expression:
//
//     lvalue [assign_op assign]
//
// An lvalue is a variable name, an array[expr] index expression, or
// an $expr field expression.
//
func (p *parser) _assign(higher func() Expr) Expr {
	expr := higher()
	if IsLValue(expr) && p.matches(ASSIGN, ADD_ASSIGN, DIV_ASSIGN,
		MOD_ASSIGN, MUL_ASSIGN, POW_ASSIGN, SUB_ASSIGN) {
		op := p.tok
		p.next()
		right := p._assign(higher)
		return &AssignExpr{expr, op, right}
	}
	return expr
}

// Parse a ?: conditional expression:
//
//     or [QUESTION NEWLINE* cond COLON NEWLINE* cond]
//
func (p *parser) cond() Expr      { return p._cond(p.or) }
func (p *parser) printCond() Expr { return p._cond(p.printOr) }

func (p *parser) _cond(higher func() Expr) Expr {
	expr := higher()
	if p.tok == QUESTION {
		p.next()
		p.optionalNewlines()
		t := p.expr()
		p.expect(COLON)
		p.optionalNewlines()
		f := p.expr()
		return &CondExpr{expr, t, f}
	}
	return expr
}

// Parse an || or expresion:
//
//     and [OR NEWLINE* and] [OR NEWLINE* and] ...
//
func (p *parser) or() Expr      { return p.binaryLeft(p.and, true, OR) }
func (p *parser) printOr() Expr { return p.binaryLeft(p.printAnd, true, OR) }

// Parse an && and expresion:
//
//     in [AND NEWLINE* in] [AND NEWLINE* in] ...
//
func (p *parser) and() Expr      { return p.binaryLeft(p.in, true, AND) }
func (p *parser) printAnd() Expr { return p.binaryLeft(p.printIn, true, AND) }

// Parse an "in" expression:
//
//     match [IN NAME] [IN NAME] ...
//
func (p *parser) in() Expr      { return p._in(p.match) }
func (p *parser) printIn() Expr { return p._in(p.printMatch) }

func (p *parser) _in(higher func() Expr) Expr {
	expr := higher()
	for p.tok == IN {
		p.next()
		array := p.val
		p.arrayParam(array)
		p.expect(NAME)
		expr = &InExpr{[]Expr{expr}, array}
	}
	return expr
}

// Parse a ~ match expression:
//
//     compare [MATCH|NOT_MATCH compare]
//
func (p *parser) match() Expr      { return p._match(p.compare) }
func (p *parser) printMatch() Expr { return p._match(p.printCompare) }

func (p *parser) _match(higher func() Expr) Expr {
	expr := higher()
	if p.matches(MATCH, NOT_MATCH) {
		op := p.tok
		p.next()
		right := p.regexStr(higher) // Not match() as these aren't associative
		return &BinaryExpr{expr, op, right}
	}
	return expr
}

// Parse a comparison expression:
//
//     concat [EQUALS|NOT_EQUALS|LESS|LTE|GREATER|GTE concat]
//
func (p *parser) compare() Expr      { return p._compare(EQUALS, NOT_EQUALS, LESS, LTE, GTE, GREATER) }
func (p *parser) printCompare() Expr { return p._compare(EQUALS, NOT_EQUALS, LESS, LTE, GTE) }

func (p *parser) _compare(ops ...Token) Expr {
	expr := p.concat()
	if p.matches(ops...) {
		op := p.tok
		p.next()
		right := p.concat() // Not compare() as these aren't associative
		return &BinaryExpr{expr, op, right}
	}
	return expr
}

func (p *parser) concat() Expr {
	expr := p.add()
	for p.matches(DOLLAR, NOT, NAME, NUMBER, STRING, LPAREN) ||
		(p.tok >= FIRST_FUNC && p.tok <= LAST_FUNC) {
		right := p.add()
		expr = &BinaryExpr{expr, CONCAT, right}
	}
	return expr
}

func (p *parser) add() Expr {
	return p.binaryLeft(p.mul, false, ADD, SUB)
}

func (p *parser) mul() Expr {
	return p.binaryLeft(p.pow, false, MUL, DIV, MOD)
}

func (p *parser) pow() Expr {
	// Note that pow (expr ^ expr) is right-associative
	expr := p.preIncr()
	if p.tok == POW {
		p.next()
		right := p.pow()
		return &BinaryExpr{expr, POW, right}
	}
	return expr
}

func (p *parser) preIncr() Expr {
	if p.tok == INCR || p.tok == DECR {
		op := p.tok
		p.next()
		expr := p.preIncr()
		if !IsLValue(expr) {
			panic(p.error("expected lvalue after ++ or --"))
		}
		return &IncrExpr{expr, op, true}
	}
	return p.postIncr()
}

func (p *parser) postIncr() Expr {
	expr := p.primary()
	if p.tok == INCR || p.tok == DECR {
		if !IsLValue(expr) {
			panic(p.error("expected lvalue before ++ or --"))
		}
		op := p.tok
		p.next()
		return &IncrExpr{expr, op, false}
	}
	return expr
}

func (p *parser) primary() Expr {
	switch p.tok {
	case NUMBER:
		// AWK allows forms like "1.5e", but ParseFloat doesn't
		s := strings.TrimRight(p.val, "eE")
		n, _ := strconv.ParseFloat(s, 64)
		p.next()
		return &NumExpr{n}
	case STRING:
		s := p.val
		p.next()
		return &StrExpr{s}
	case DIV, DIV_ASSIGN:
		regex := p.nextRegex()
		return &RegExpr{regex}
	case DOLLAR:
		p.next()
		return &FieldExpr{p.primary()}
	case NOT, ADD, SUB:
		op := p.tok
		p.next()
		return &UnaryExpr{op, p.primary()}
	case NAME:
		name := p.val
		p.next()
		if p.tok == LBRACKET {
			p.next()
			index := p.exprList(p.expr)
			p.expect(RBRACKET)
			p.arrayParam(name)
			return &IndexExpr{name, index}
		} else if p.tok == LPAREN && !p.lexer.HadSpace() {
			p.next()
			args := p.exprList(p.expr)
			p.expect(RPAREN)
			return &UserCallExpr{name, args}
		}
		index := p.getVarIndex(name)
		return &VarExpr{index, name}
	case LPAREN:
		p.next()
		exprs := p.exprList(p.expr)
		switch len(exprs) {
		case 0:
			panic(p.error("expected expression, not %s", p.tok))
		case 1:
			p.expect(RPAREN)
			return exprs[0]
		default:
			// Multi-dimensional array "in" requires parens around index
			p.expect(RPAREN)
			if p.tok == IN {
				p.next()
				array := p.val
				p.arrayParam(array)
				p.expect(NAME)
				return &InExpr{exprs, array}
			}
			return &MultiExpr{exprs}
		}
	case GETLINE:
		p.next()
		name := ""
		index := 0
		if p.tok == NAME {
			name = p.val
			index = p.getVarIndex(name)
			p.next()
		}
		var file Expr
		if p.tok == LESS {
			p.next()
			file = p.expr()
		}
		return &GetlineExpr{nil, index, name, file}
	case F_SUB, F_GSUB:
		op := p.tok
		p.next()
		p.expect(LPAREN)
		regex := p.regexStr(p.expr)
		p.commaNewlines()
		repl := p.expr()
		args := []Expr{regex, repl}
		if p.tok == COMMA {
			p.commaNewlines()
			in := p.expr()
			if !IsLValue(in) {
				panic(p.error("3rd arg to sub/gsub must be lvalue"))
			}
			args = append(args, in)
		}
		p.expect(RPAREN)
		return &CallExpr{op, args}
	case F_SPLIT:
		p.next()
		p.expect(LPAREN)
		str := p.expr()
		p.commaNewlines()
		array := p.val
		p.arrayParam(array)
		p.expect(NAME)
		// This is kind of an abuse of VarExpr just to represent an
		// array name - TODO: change to use CallSplitExpr instead?
		args := []Expr{str, &VarExpr{0, array}}
		if p.tok == COMMA {
			p.commaNewlines()
			args = append(args, p.regexStr(p.expr))
		}
		p.expect(RPAREN)
		return &CallExpr{F_SPLIT, args}
	case F_MATCH:
		p.next()
		p.expect(LPAREN)
		str := p.expr()
		p.commaNewlines()
		regex := p.regexStr(p.expr)
		p.expect(RPAREN)
		return &CallExpr{F_MATCH, []Expr{str, regex}}
	case F_RAND:
		p.next()
		p.expect(LPAREN)
		p.expect(RPAREN)
		return &CallExpr{F_RAND, nil}
	case F_SRAND:
		p.next()
		p.expect(LPAREN)
		var args []Expr
		if p.tok != RPAREN {
			args = append(args, p.expr())
		}
		p.expect(RPAREN)
		return &CallExpr{F_SRAND, args}
	case F_LENGTH:
		p.next()
		var args []Expr
		// AWK quirk: "length" is allowed to be called without parens
		if p.tok == LPAREN {
			p.next()
			if p.tok != RPAREN {
				args = append(args, p.expr())
			}
			p.expect(RPAREN)
		}
		return &CallExpr{F_LENGTH, args}
	case F_SUBSTR:
		p.next()
		p.expect(LPAREN)
		str := p.expr()
		p.commaNewlines()
		start := p.expr()
		args := []Expr{str, start}
		if p.tok == COMMA {
			p.commaNewlines()
			args = append(args, p.expr())
		}
		p.expect(RPAREN)
		return &CallExpr{F_SUBSTR, args}
	case F_SPRINTF:
		p.next()
		p.expect(LPAREN)
		args := []Expr{p.expr()}
		for p.tok == COMMA {
			p.commaNewlines()
			args = append(args, p.expr())
		}
		p.expect(RPAREN)
		return &CallExpr{F_SPRINTF, args}
	case F_COS, F_SIN, F_EXP, F_LOG, F_SQRT, F_INT, F_TOLOWER, F_TOUPPER, F_SYSTEM, F_CLOSE:
		// 1-argument functions
		op := p.tok
		p.next()
		p.expect(LPAREN)
		arg := p.expr()
		p.expect(RPAREN)
		return &CallExpr{op, []Expr{arg}}
	case F_ATAN2, F_INDEX:
		// 2-argument functions
		op := p.tok
		p.next()
		p.expect(LPAREN)
		arg1 := p.expr()
		p.commaNewlines()
		arg2 := p.expr()
		p.expect(RPAREN)
		return &CallExpr{op, []Expr{arg1, arg2}}
	default:
		panic(p.error("expected expression instead of %s", p.tok))
	}
}

// Parse /.../ regex or generic espression:
//
//     REGEX | expr
//
func (p *parser) regexStr(parse func() Expr) Expr {
	if p.matches(DIV, DIV_ASSIGN) {
		regex := p.nextRegex()
		return &StrExpr{regex}
	}
	return parse()
}

// Parse left-associative binary operator. Allow newlines after
// operator if allowNewline is true.
//
//     parse [op parse] [op parse] ...
//
func (p *parser) binaryLeft(higher func() Expr, allowNewline bool, ops ...Token) Expr {
	expr := higher()
	for p.matches(ops...) {
		op := p.tok
		p.next()
		if allowNewline {
			p.optionalNewlines()
		}
		right := higher()
		expr = &BinaryExpr{expr, op, right}
	}
	return expr
}

// Parse comma followed by optional newlines:
//
//     COMMA NEWLINE*
//
func (p *parser) commaNewlines() {
	p.expect(COMMA)
	p.optionalNewlines()
}

// Parse zero or more optional newlines:
//
//    [NEWLINE] [NEWLINE] ...
//
func (p *parser) optionalNewlines() {
	for p.tok == NEWLINE {
		p.next()
	}
}

// Parse next token into p.tok (and set p.pos and p.val).
func (p *parser) next() {
	p.pos, p.tok, p.val = p.lexer.Scan()
	if p.tok == ILLEGAL {
		panic(p.error("%s", p.val))
	}
}

func (p *parser) nextRegex() string {
	p.pos, p.tok, p.val = p.lexer.ScanRegex()
	if p.tok == ILLEGAL {
		panic(p.error("%s", p.val))
	}
	regex := p.val
	p.next()
	return regex
}

// Ensure current token is tok, and parse next token into p.tok.
func (p *parser) expect(tok Token) {
	if p.tok != tok {
		panic(p.error("expected %s instead of %s", tok, p.tok))
	}
	p.next()
}

// Return true iff current token matches one of the given operators,
// but don't parse next token.
func (p *parser) matches(operators ...Token) bool {
	for _, operator := range operators {
		if p.tok == operator {
			return true
		}
	}
	return false
}

func (p *parser) error(format string, args ...interface{}) error {
	message := fmt.Sprintf(format, args...)
	return &ParseError{p.pos, message}
}

func (p *parser) getVarIndex(name string) int {
	index := p.locals[name]
	if index != 0 {
		return index
	}
	index = specialVars[name]
	if index != 0 {
		// Special variable like ARGC
		return index
	}
	index = p.globals[name]
	if index != 0 {
		// Global variable that's already been seen
		return index
	}
	// New global variable
	index = len(p.globals) + V_LAST + 1
	p.globals[name] = index
	return index
}
