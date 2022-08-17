package cover

import (
	"fmt"
	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/parser"
)

type annotator struct {
	covermode     string
	annotationIdx int
}

func Annotate(prog *parser.Program, covermode string) {
	annotator := &annotator{covermode, 0}
	prog.Begin = annotator.annotateStmtsList(prog.Begin)
	prog.Actions = annotator.annotateActions(prog.Actions)
	prog.End = annotator.annotateStmtsList(prog.End)
	prog.Functions = annotator.annotateFunctions(prog.Functions)
}

func (annotator *annotator) annotateActions(actions []ast.Action) (res []ast.Action) {
	for _, action := range actions {
		action.Stmts = annotator.annotateStmts(action.Stmts)
		res = append(res, action)
	}
	return
}

func (annotator *annotator) annotateFunctions(functions []ast.Function) (res []ast.Function) {
	for _, function := range functions {
		function.Body = annotator.annotateStmts(function.Body)
		res = append(res, function)
	}
	return
}

func (annotator *annotator) annotateStmtsList(stmtsList []ast.Stmts) (res []ast.Stmts) {
	for _, stmts := range stmtsList {
		res = append(res, annotator.annotateStmts(stmts))
	}
	return
}

// annotateStmts takes a list of statements and adds counters to the beginning of
// each basic block at the top level of that list. For instance, given
//
//	S1
//	if cond {
//		S2
//	}
//	S3
//
// counters will be added before S1,S2,S3.
func (annotator *annotator) annotateStmts(stmts ast.Stmts) (res ast.Stmts) {
	var simpleStatements []ast.Stmt
	for _, stmt := range stmts {
		wasBlock := true
		switch s := stmt.(type) {
		case *ast.IfStmt:
			s.Body = annotator.annotateStmts(s.Body)
			s.Else = annotator.annotateStmts(s.Else)
		case *ast.ForStmt:
			s.Body = annotator.annotateStmts(s.Body) // TODO should we do smth with pre & post?
		case *ast.ForInStmt:
			s.Body = annotator.annotateStmts(s.Body)
		case *ast.WhileStmt:
			s.Body = annotator.annotateStmts(s.Body)
		case *ast.DoWhileStmt:
			s.Body = annotator.annotateStmts(s.Body)
		case *ast.BlockStmt:
			s.Body = annotator.annotateStmts(s.Body)
		default:
			wasBlock = false
		}
		if wasBlock {
			res = append(res, annotator.trackStatement())
			res = append(res, simpleStatements...)
			res = append(res, stmt)
			simpleStatements = []ast.Stmt{}
		} else {
			simpleStatements = append(simpleStatements, stmt)
		}
	}
	if len(simpleStatements) > 0 {
		res = append(res, annotator.trackStatement())
		res = append(res, simpleStatements...)
	}
	return
	// TODO complete handling of if/else/else if
}
func (annotator *annotator) trackStatement() ast.Stmt {
	op := "=1"
	if annotator.covermode == "count" {
		op = "++"
	}
	annotator.annotationIdx++
	trackProg, err := parser.ParseProgram([]byte(fmt.Sprintf(`BEGIN { __COVER[%d]%s }`, annotator.annotationIdx, op)), nil)
	if err != nil {
		panic(err)
	}
	return trackProg.Begin[0][0]
}
