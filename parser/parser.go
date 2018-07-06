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

func (p *parser) stmt() Stmt {
	var s Stmt
	switch p.tok {
	case PRINT, PRINTF:
		tok := p.tok
		p.next()
		args := p.exprList()
		if tok == PRINT {
			s = &PrintStmt{args}
		} else {
			s = &PrintfStmt{args}
		}
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
		pre := p.stmt() // TODO: p.optionalSimpleStmt()
		if p.tok == RPAREN {
			// Match: for (var in array) body
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
			// Match: for (pre; cond; post) body
			p.expect(SEMICOLON)
			cond := p.optionalExpr()
			p.expect(SEMICOLON)
			post := p.stmt() // TODO: p.optionalSimpleStmt()
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
		p.next()
		s = &BreakStmt{}
	case CONTINUE:
		p.next()
		s = &ContinueStmt{}
	case NEXT:
		p.next()
		s = &NextStmt{}
	case EXIT:
		p.next()
		status := p.optionalExpr()
		s = &ExitStmt{status}
	case DELETE:
		p.next()
		array := p.val
		p.expect(NAME)
		p.expect(LBRACKET)
		index := p.exprList()
		p.expect(RBRACKET)
		s = &DeleteStmt{array, index}
	default:
		s = &ExprStmt{p.expr()}
	}
	if p.matches(NEWLINE, SEMICOLON) {
		p.next()
	}
	return s
}

func (p *parser) optionalExpr() Expr {
	if p.matches(NEWLINE, SEMICOLON) {
		return nil
	}
	return p.expr()
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
		return &FieldExpr{p.expr()}
	default:
		panic(p.error("expected expression"))
	}
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

func ParseProgram_TEST(src []byte) (*Program, error) {
	// TODO
	program := &Program{
		Begin: []Stmts{
			{
				// &ExprStmt{
				//  &AssignExpr{&VarExpr{"RS"}, "", StrExpr("|")},
				// },
				&ExprStmt{
					&CallExpr{"srand", []Expr{&NumExpr{1.2}}},
				},
			},
		},
		Actions: []Action{
			{
				Pattern: &BinaryExpr{
					Left:  &FieldExpr{&NumExpr{0}},
					Op:    "!=",
					Right: &StrExpr{""},
				},
				Stmts: []Stmt{
					&PrintStmt{
						Args: []Expr{
							&CallSubExpr{&StrExpr{`\.`}, &StrExpr{","}, &FieldExpr{&NumExpr{0}}, true},
							&FieldExpr{&NumExpr{0}},
						},
					},
					// &ForInStmt{
					//  Var:   "x",
					//  Array: "a",
					//  Body: []Stmt{
					//      &PrintStmt{
					//          Args: []Expr{
					//              &VarExpr{"x"},
					//              &IndexExpr{"a", &VarExpr{"x"}},
					//          },
					//      },
					//  },
					// },
				},
			},
		},
	}
	return program, nil
}
