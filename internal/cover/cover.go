package cover

import (
	"fmt"
	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/parseutil"
	. "github.com/benhoyt/goawk/lexer"
	. "github.com/benhoyt/goawk/parser"
	"os"
	"strconv"
)

const ArrCover = "__COVER"

type annotator struct {
	covermode     string
	coverappend   bool
	fileReader    *parseutil.FileReader
	annotationIdx int
	boundaries    map[int]boundary
	stmtsCnt      map[int]int
}

type boundary struct {
	start    Position
	end      Position
	fileName string
}

func NewAnnotator(covermode string, coverappend bool, fileReader *parseutil.FileReader) *annotator {
	return &annotator{
		covermode,
		coverappend,
		fileReader,
		0,
		map[int]boundary{},
		map[int]int{},
	}
}

func (annotator *annotator) Annotate(prog *ast.Program) {
	prog.Begin = annotator.annotateStmtsList(prog.Begin)
	prog.Actions = annotator.annotateActions(prog.Actions)
	prog.End = annotator.annotateStmtsList(prog.End)
	prog.Functions = annotator.annotateFunctions(prog.Functions)
}

func (annotator *annotator) AppendCoverData(coverprofile string, coverData map[string]string) error {
	// 1a. If file doesn't exist - create and write covermode line
	// 1b. If file exists - open it for writing in append mode
	// 2.  Write all coverData lines

	coverDataInts := convertCoverData(coverData)

	var f *os.File
	if _, err := os.Stat(coverprofile); os.IsNotExist(err) {
		f, err = os.OpenFile(coverprofile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		_, err := f.WriteString("mode: " + annotator.covermode + "\n")
		if err != nil {
			return err
		}
	} else if err == nil {
		// file exists
		fileOpt := os.O_TRUNC
		if annotator.coverappend {
			fileOpt = os.O_APPEND
		}
		f, err = os.OpenFile(coverprofile, os.O_WRONLY|fileOpt, 0644)
		if err != nil {
			return err
		}
	} else {
		panic(err)
	}
	for i := 1; i <= annotator.annotationIdx; i++ {
		_, err := f.WriteString(renderCoverDataLine(annotator.boundaries[i], annotator.stmtsCnt[i], coverDataInts[i]))
		if err != nil {
			return err
		}
	}
	return nil
}

func convertCoverData(coverData map[string]string) map[int]int {
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

func (annotator *annotator) annotateActions(actions []*ast.Action) (res []*ast.Action) {
	for _, action := range actions {
		action.Stmts = annotator.annotateStmts(action.Stmts)
		res = append(res, action)
	}
	return
}

func (annotator *annotator) annotateFunctions(functions []*ast.Function) (res []*ast.Function) {
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
			if len(simpleStatements) > 0 {
				res = append(res, annotator.trackStatement(simpleStatements))
				res = append(res, simpleStatements...)
			}
			res = append(res, stmt)
			simpleStatements = []ast.Stmt{}
		} else {
			simpleStatements = append(simpleStatements, stmt)
		}
	}
	if len(simpleStatements) > 0 {
		res = append(res, annotator.trackStatement(simpleStatements))
		res = append(res, simpleStatements...)
	}
	return
	// TODO complete handling of if/else/else if
}
func (annotator *annotator) trackStatement(statements []ast.Stmt) ast.Stmt {
	op := "=1"
	if annotator.covermode == "count" {
		op = "++"
	}
	annotator.annotationIdx++
	start1, _ := statements[0].(ast.BoundaryProvider).GetBoundary()
	_, end2 := statements[len(statements)-1].(ast.BoundaryProvider).GetBoundary()
	path, startLine := annotator.fileReader.FileLine(start1.Line)
	_, endLine := annotator.fileReader.FileLine(end2.Line)
	annotator.boundaries[annotator.annotationIdx] = boundary{
		start:    Position{startLine, start1.Column},
		end:      Position{endLine, end2.Column},
		fileName: path,
	}
	annotator.stmtsCnt[annotator.annotationIdx] = len(statements)
	return parseProg(fmt.Sprintf(`BEGIN { %s[%d]%s }`, ArrCover, annotator.annotationIdx, op)).Begin[0][0]
}

func parseProg(code string) *Program {
	prog, err := ParseProgram([]byte(code), nil)
	if err != nil {
		panic(err)
	}
	return prog
}

func renderCoverDataLine(boundary boundary, stmtsCnt int, cnt int) string {
	return fmt.Sprintf("%s:%d.%d,%d.%d %d %d\n",
		boundary.fileName,
		boundary.start.Line, boundary.start.Column,
		boundary.end.Line, boundary.end.Column,
		stmtsCnt, cnt,
	)
}
