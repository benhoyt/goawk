package cover

import (
	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/parser"
)

func Annotate(prog *parser.Program) {
	prog.Begin = annotateStmtsList(prog.Begin)
	prog.End = annotateStmtsList(prog.End)
	for _, action := range prog.Actions {
		action.Stmts = annotateStmts(action.Stmts)
	}
	for _, function := range prog.Functions {
		function.Body = annotateStmts(function.Body)
	}
}

func annotateStmtsList(stmtsList []ast.Stmts) (res []ast.Stmts) {
	for _, stmts := range stmtsList {
		res = append(res, annotateStmts(stmts))
	}
	return
}
func annotateStmts(stmts ast.Stmts) (res ast.Stmts) {
	trackProg, err := parser.ParseProgram([]byte(`BEGIN { __COVER[0]++ }`), nil)
	if err != nil {
		panic(err)
	}
	res = append(res, trackProg.Begin[0][0])
	res = append(res, stmts...)
	return
}
