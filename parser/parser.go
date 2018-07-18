// GoAWK parser.
package parser

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/benhoyt/goawk/lexer"
)

type ParseError struct {
	Position Position
	Message  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at %d:%d: %s", e.Position.Line, e.Position.Column, e.Message)
}

type parser struct {
	lexer     *Lexer
	pos       Position
	tok       Token
	val       string
	progState progState
	inLoop    bool
}

type progState int

const (
	beginState progState = iota
	endState
	actionState
)

func (p *parser) program() *Program {
	prog := &Program{}
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
		gotParen := false
		if p.tok == LPAREN {
			p.next()
			gotParen = true
		}
		args := p.exprList()
		if gotParen {
			p.expect(RPAREN)
		}
		if op == PRINT {
			return &PrintStmt{args}
		} else {
			return &PrintfStmt{args}
		}
	case DELETE:
		p.next()
		array := p.val
		p.expect(NAME)
		p.expect(LBRACKET)
		index := p.exprList()
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
			varExpr, ok := (inExpr.Index).(*VarExpr)
			if !ok {
				panic(p.error("expected 'for (var in array) ...'"))
			}
			body := p.loopStmts()
			s = &ForInStmt{varExpr.Name, inExpr.Array, body}
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
		if !p.inLoop {
			panic(p.error("break must be inside a loop body"))
		}
		p.next()
		s = &BreakStmt{}
	case CONTINUE:
		if !p.inLoop {
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
	default:
		s = p.simpleStmt()
	}
	for p.matches(NEWLINE, SEMICOLON) {
		p.next()
	}
	return s
}

func (p *parser) loopStmts() Stmts {
	p.inLoop = true
	ss := p.stmts()
	p.inLoop = false
	return ss
}

func (p *parser) exprList() []Expr {
	exprs := []Expr{}
	first := true
	for !p.matches(NEWLINE, SEMICOLON, RBRACE, RBRACKET, RPAREN) {
		if !first {
			p.commaNewlines()
		}
		first = false
		exprs = append(exprs, p.expr())
	}
	return exprs
}

func (p *parser) expr() Expr {
	return p.assign()
}

func (p *parser) assign() Expr {
	expr := p.cond()
	if IsLValue(expr) && p.matches(ASSIGN, ADD_ASSIGN, DIV_ASSIGN,
		MOD_ASSIGN, MUL_ASSIGN, POW_ASSIGN, SUB_ASSIGN) {
		op := p.tok
		p.next()
		right := p.assign()
		return &AssignExpr{expr, op, right}
	}
	return expr
}

func (p *parser) cond() Expr {
	expr := p.or()
	if p.tok == QUESTION {
		p.next()
		t := p.cond()
		p.expect(COLON)
		f := p.cond()
		return &CondExpr{expr, t, f}
	}
	return expr
}

func (p *parser) or() Expr {
	return p.binaryLeft(p.and, OR)
}

func (p *parser) and() Expr {
	return p.binaryLeft(p.in, AND)
}

func (p *parser) in() Expr {
	expr := p.match()
	for p.tok == IN {
		p.next()
		array := p.val
		p.expect(NAME)
		expr = &InExpr{expr, array}
	}
	return expr
}

func (p *parser) match() Expr {
	expr := p.compare()
	if p.matches(MATCH, NOT_MATCH) {
		op := p.tok
		p.next()
		right := p.compare() // Not match() as these aren't associative
		return &BinaryExpr{expr, op, right}
	}
	return expr
}

func (p *parser) compare() Expr {
	expr := p.concat()
	if p.matches(EQUALS, NOT_EQUALS, LESS, LTE, GREATER, GTE) {
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
	return p.binaryLeft(p.mul, ADD, SUB)
}

func (p *parser) mul() Expr {
	return p.binaryLeft(p.unary, MUL, DIV, MOD)
}

func (p *parser) unary() Expr {
	switch p.tok {
	case NOT, ADD, SUB:
		op := p.tok
		p.next()
		return &UnaryExpr{op, p.unary()}
	default:
		return p.pow()
	}
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
	case REGEX:
		regex := p.val
		p.next()
		return &RegExpr{regex}
	case DOLLAR:
		p.next()
		return &FieldExpr{p.primary()}
	case NAME:
		name := p.val
		p.next()
		if p.tok == LBRACKET {
			p.next()
			index := p.exprList()
			p.expect(RBRACKET)
			return &IndexExpr{name, index}
		} else if p.tok == LPAREN && !p.lexer.HadSpace() {
			panic(p.error("user-defined functions not yet supported"))
		}
		return &VarExpr{name}
	case LPAREN:
		p.next()
		expr := p.expr()
		p.expect(RPAREN)
		return expr
	case F_SUB, F_GSUB:
		op := p.tok
		p.next()
		p.expect(LPAREN)
		regex := p.regex()
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
		p.expect(NAME)
		args := []Expr{str, &VarExpr{array}}
		if p.tok == COMMA {
			p.commaNewlines()
			args = append(args, p.regex())
		}
		p.expect(RPAREN)
		return &CallExpr{F_SPLIT, args}
	case F_MATCH:
		p.next()
		p.expect(LPAREN)
		str := p.expr()
		p.commaNewlines()
		regex := p.regex()
		p.expect(RPAREN)
		return &CallExpr{F_MATCH, []Expr{str, regex}}
	case F_RAND:
		// TODO: "rand" is allowed to be called without parens (any other funcs like this?)
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
	case F_COS, F_SIN, F_EXP, F_LOG, F_SQRT, F_INT, F_TOLOWER, F_TOUPPER:
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

func (p *parser) regex() Expr {
	if p.tok == REGEX {
		regex := p.val
		p.next()
		return &StrExpr{regex}
	}
	return p.expr()
}

func (p *parser) binaryLeft(parse func() Expr, ops ...Token) Expr {
	expr := parse()
	for p.matches(ops...) {
		op := p.tok
		p.next()
		right := parse()
		expr = &BinaryExpr{expr, op, right}
	}
	return expr
}

func (p *parser) commaNewlines() {
	p.expect(COMMA)
	p.optionalNewlines()
}

func (p *parser) optionalNewlines() {
	for p.tok == NEWLINE {
		p.next()
	}
}

func (p *parser) next() {
	p.pos, p.tok, p.val = p.lexer.Scan()
	if p.tok == ILLEGAL {
		panic(p.error("%s", p.val))
	}
}

func (p *parser) expect(tok Token) {
	if p.tok != tok {
		panic(p.error("expected %s instead of %s", tok, p.tok))
	}
	p.next()
}

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

func ParseProgram(src []byte) (prog *Program, err error) {
	defer func() {
		if r := recover(); r != nil {
			// Convert to ParseError or re-panic
			err = r.(*ParseError)
		}
	}()
	lexer := NewLexer(src)
	p := parser{lexer: lexer}
	p.next()
	return p.program(), nil
}

func ParseExpr(src []byte) (expr Expr, err error) {
	defer func() {
		if r := recover(); r != nil {
			// Convert to ParseError or re-panic
			err = r.(*ParseError)
		}
	}()
	lexer := NewLexer(src)
	p := parser{lexer: lexer}
	p.next()
	return p.expr(), nil
}
