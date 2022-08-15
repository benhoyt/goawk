package cover

import (
	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/parser"
)

func Annotate(prog *parser.Program) {
	prog.Begin = annotateStmtsList(prog.Begin)
	return nil
}

func annotateStmtsList(stmtsList []ast.Stmts) (annotatedStmtsList []ast.Stmts) {
	var res []ast.Stmts
	for _, stmts := range stmtsList {
		res = append(res, annotateStmts(stmts))
	}
	return res
}
func annotateStmts(stmts ast.Stmts) (annotatedStmts ast.Stmts) {
	// TODO
}
