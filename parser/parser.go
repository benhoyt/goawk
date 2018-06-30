// GoAWK parser

package parser

func Parse(src string) (*Program, error) {
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
