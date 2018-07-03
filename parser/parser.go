// GoAWK parser.
package parser

import (
//	. "github.com/benhoyt/goawk/lexer"
)

/*
type parser struct {
	lexer *Lexer
	pos Position
	tok Token
	val string
}

func (p *parser) next() {
	p.pos, p.tok, p.val = p.lexer.Next()
	if p.tok == ILLEGAL {
		p.error("%s", p.val)
	}
}

func (p *parser) error(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	panic(Error{p.pos, message})
}
*/

func ParseProgram(src []byte) (*Program, error) {
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

func ParseExpr(src []byte) (Expr, error) {
	// TODO
	return &FieldExpr{&NumExpr{0}}, nil
}
