// GoAWK parser.
package parser

import (
	"fmt"
	"strconv"

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
	lexer *Lexer
	pos   Position
	tok   Token
	val   string
}

func (p *parser) program() *Program {
	prog := &Program{}
	for p.tok != EOF {
		switch p.tok {
		case BEGIN:
			p.next()
			prog.Begin = append(prog.Begin, p.stmtsBrace())
		case END:
			p.next()
			prog.End = append(prog.End, p.stmtsBrace())
		default:
			// Can have an empty pattern (always true)
			var pattern Expr
			if p.tok != LBRACE {
				pattern = p.expr()
			}
			// Or an empty action (equivalent to { print $0 })
			action := Action{pattern, nil}
			if p.tok == LBRACE {
				action.Stmts = p.stmtsBrace()
			}
			prog.Actions = append(prog.Actions, action)
		}
	}
	return prog
}

func (p *parser) stmts() Stmts {
	if p.tok == LBRACE {
		return p.stmtsBrace()
	} else {
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
		tok := p.tok
		p.next()
		args := p.exprList()
		if tok == PRINT {
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
	var s Stmt
	switch p.tok {
	case IF:
		p.next()
		p.expect(LPAREN)
		cond := p.expr()
		p.expect(RPAREN)
		p.optionalNewlines()
		body := p.stmts()
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
			body := p.stmts()
			s = &ForInStmt{varExpr.Name, inExpr.Array, body}
		} else {
			// Match: for ([pre]; [cond]; [post]) body
			p.expect(SEMICOLON)
			var cond Expr
			if p.tok != SEMICOLON {
				cond = p.expr()
			}
			p.expect(SEMICOLON)
			var post Stmt
			if p.tok != SEMICOLON {
				post = p.simpleStmt()
			}
			p.expect(RPAREN)
			body := p.stmts()
			s = &ForStmt{pre, cond, post, body}
		}
	case WHILE:
		p.next()
		p.expect(LPAREN)
		cond := p.expr()
		p.expect(RPAREN)
		body := p.stmts()
		s = &WhileStmt{cond, body}
	case DO:
		p.next()
		p.optionalNewlines()
		body := p.stmts()
		p.expect(WHILE)
		p.expect(LPAREN)
		cond := p.expr()
		p.expect(RPAREN)
		s = &DoWhileStmt{body, cond}
	case BREAK:
		// TODO: must be in a loop
		p.next()
		s = &BreakStmt{}
	case CONTINUE:
		// TODO: must be in a loop
		p.next()
		s = &ContinueStmt{}
	case NEXT:
		// TODO: must not be in BEGIN or END
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
	if p.matches(NEWLINE, SEMICOLON) {
		p.next()
	}
	return s
}

func (p *parser) exprList() []Expr {
	exprs := []Expr{}
	first := true
	for !p.matches(NEWLINE, SEMICOLON, RBRACE, RBRACKET, RPAREN) {
		if !first {
			p.expect(COMMA)
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
		n, _ := strconv.ParseFloat(p.val, 64)
		p.next()
		return &NumExpr{n}
	case STRING:
		s := p.val
		p.next()
		return &StrExpr{s}
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
		}
		return &VarExpr{name}
	case LPAREN:
		p.next()
		expr := p.expr()
		p.expect(RPAREN)
		return expr
	default:
		panic(p.error("expected expression"))
	}
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
