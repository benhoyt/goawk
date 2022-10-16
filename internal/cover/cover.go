// Package cover implements AWK code coverage and reporting.
package cover

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/parseutil"
	"github.com/benhoyt/goawk/lexer"
)

const ArrayName = "__COVER"

type Cover struct {
	mode          string
	append        bool
	fileReader    *parseutil.FileReader
	annotationIdx int
	boundaries    map[int]boundary
	stmtsCnt      map[int]int
}

type boundary struct {
	start lexer.Position
	end   lexer.Position
	path  string
}

func New(mode string, append bool, fileReader *parseutil.FileReader) *Cover {
	return &Cover{
		mode,
		append,
		fileReader,
		0,
		map[int]boundary{},
		map[int]int{},
	}
}

// Annotate annotates the program with coverage tracking code.
func (cov *Cover) Annotate(prog *ast.Program) {
	prog.Begin = cov.annotateStmtsList(prog.Begin)
	prog.Actions = cov.annotateActions(prog.Actions)
	prog.End = cov.annotateStmtsList(prog.End)
	prog.Functions = cov.annotateFunctions(prog.Functions)
}

// StoreCoverData writes result coverage report data to coverprofile file
func (cov *Cover) StoreCoverData(coverprofile string, coverData map[string]string) error {
	// 1a. If file doesn't exist - create and write cover mode line
	// 1b. If file exists and coverappend=true  - open it for writing in append mode
	// 1c. If file exists and coverappend=false - truncate it and follow 1a.
	// 2.  Write all coverData lines

	coverDataInts := prepareCoverData(coverData)
	isNewFile := true

	var f *os.File
	if _, err := os.Stat(coverprofile); os.IsNotExist(err) {
		f, err = os.OpenFile(coverprofile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
	} else if err == nil {
		// file exists
		fileOpt := os.O_TRUNC
		if cov.append {
			isNewFile = false
			fileOpt = os.O_APPEND
		}
		f, err = os.OpenFile(coverprofile, os.O_WRONLY|fileOpt, 0644)
		if err != nil {
			return err
		}
	} else {
		return err
	}

	if isNewFile {
		_, err := fmt.Fprintf(f, "mode: %s\n", cov.mode)
		if err != nil {
			return err
		}
	}

	for i := 1; i <= cov.annotationIdx; i++ {
		_, err := f.WriteString(renderCoverDataLine(cov.boundaries[i], cov.stmtsCnt[i], coverDataInts[i]))
		if err != nil {
			return err
		}
	}
	return nil
}

func prepareCoverData(coverData map[string]string) map[int]int {
	res := map[int]int{}
	for k, v := range coverData {
		ki, err := strconv.Atoi(k)
		if err != nil {
			panic("non-int index in coverData: " + k)
		}
		vi, err := strconv.Atoi(v)
		if err != nil {
			panic("non-int value in coverData: " + v)
		}
		res[ki] = vi
	}
	return res
}

func (cov *Cover) annotateActions(actions []*ast.Action) (res []*ast.Action) {
	for _, action := range actions {
		action.Stmts = cov.annotateStmts(action.Stmts)
		res = append(res, action)
	}
	return
}

func (cov *Cover) annotateFunctions(functions []*ast.Function) (res []*ast.Function) {
	for _, function := range functions {
		function.Body = cov.annotateStmts(function.Body)
		res = append(res, function)
	}
	return
}

func (cov *Cover) annotateStmtsList(stmtsList []ast.Stmts) (res []ast.Stmts) {
	for _, stmts := range stmtsList {
		res = append(res, cov.annotateStmts(stmts))
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
func (cov *Cover) annotateStmts(stmts ast.Stmts) (res ast.Stmts) {
	var trackedBlockStmts []ast.Stmt
	for _, stmt := range stmts {
		blockEnds := true
		switch s := stmt.(type) {
		case *ast.IfStmt:
			s.Body = cov.annotateStmts(s.Body)
			s.Else = cov.annotateStmts(s.Else)
		case *ast.ForStmt:
			s.Body = cov.annotateStmts(s.Body) // TODO should we do smth with pre & post?
		case *ast.ForInStmt:
			s.Body = cov.annotateStmts(s.Body)
		case *ast.WhileStmt:
			s.Body = cov.annotateStmts(s.Body)
		case *ast.DoWhileStmt:
			s.Body = cov.annotateStmts(s.Body)
		case *ast.BlockStmt:
			s.Body = cov.annotateStmts(s.Body)
		default:
			blockEnds = false
		}
		trackedBlockStmts = append(trackedBlockStmts, stmt)
		if blockEnds {
			res = append(res, cov.trackStatement(trackedBlockStmts))
			res = append(res, trackedBlockStmts...)
			trackedBlockStmts = []ast.Stmt{}
		}
	}
	if len(trackedBlockStmts) > 0 {
		res = append(res, cov.trackStatement(trackedBlockStmts))
		res = append(res, trackedBlockStmts...)
	}
	return
	// TODO complete handling of if/else/else if
}

func endPos(stmt ast.Stmt) lexer.Position {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		return s.BodyStart
	case *ast.ForStmt:
		return s.BodyStart
	case *ast.ForInStmt:
		return s.BodyStart
	case *ast.WhileStmt:
		return s.BodyStart
	default:
		return s.EndPos()
	}
}

func (cov *Cover) trackStatement(stmts []ast.Stmt) ast.Stmt {
	cov.annotationIdx++
	start1 := stmts[0].StartPos()
	end2 := endPos(stmts[len(stmts)-1])
	path, startLine := cov.fileReader.FileLine(start1.Line)
	_, endLine := cov.fileReader.FileLine(end2.Line)
	cov.boundaries[cov.annotationIdx] = boundary{
		start: lexer.Position{startLine, start1.Column},
		end:   lexer.Position{endLine, end2.Column},
		path:  path,
	}
	cov.stmtsCnt[cov.annotationIdx] = len(stmts)
	left := &ast.IndexExpr{
		Array: ast.ArrayRef(ArrayName, lexer.Position{}),
		Index: []ast.Expr{&ast.StrExpr{Value: strconv.Itoa(cov.annotationIdx)}},
	}
	if cov.mode == "count" {
		// AST for __COVER[index]++
		return &ast.ExprStmt{Expr: &ast.IncrExpr{Expr: left, Op: lexer.INCR}}
	}
	// AST for __COVER[index] = 1
	return &ast.ExprStmt{Expr: &ast.AssignExpr{Left: left, Right: &ast.NumExpr{Value: 1}}}
}

func renderCoverDataLine(boundary boundary, stmtsCnt int, cnt int) string {
	return fmt.Sprintf("%s:%d.%d,%d.%d %d %d\n",
		toAbsolutePath(boundary.path),
		boundary.start.Line, boundary.start.Column,
		boundary.end.Line, boundary.end.Column,
		stmtsCnt, cnt,
	)
}

func toAbsolutePath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}
