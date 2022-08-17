package cover

import (
	"fmt"
	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/parser"
	"os"
	"strings"
)

// TODO mention thread-safety
type annotator struct {
	covermode       string
	currentFileName string
	annotationIdx   int
	boundaries      map[int]ast.Boundary
	fileNames       map[int]string
	program         *parser.Program
}

func NewAnnotator(covermode string, parserConfig *parser.ParserConfig) *annotator {
	return &annotator{covermode, "", 0,
		map[int]ast.Boundary{}, map[int]string{}, parseProg("", parserConfig)}
}

var onlyParseParserConfig = &parser.ParserConfig{
	DebugWriter:    os.Stderr,
	OnlyParseToAST: true,
}

func (annotator *annotator) AddFile(filename string, code []byte) {
	prog, err := parser.ParseProgram(code, onlyParseParserConfig)
	if err != nil {
		panic(err) // at this point the code should be already valid
	}
	annotator.currentFileName = filename
	annotator.annotate(prog)
	annotator.appendProgram(prog)
}

func (annotator *annotator) appendProgram(prog *parser.Program) {
	annotator.program.Begin = append(annotator.program.Begin, prog.Begin...)
	annotator.program.End = append(annotator.program.End, prog.End...)
	annotator.program.Actions = append(annotator.program.Actions, prog.Actions...)
	annotator.program.Functions = append(annotator.program.Functions, prog.Functions...)
}

func (annotator *annotator) GetResultProgram() *parser.Program {
	annotator.addCoverageEnd()
	program := annotator.program
	program.Resolve()
	err := program.Compile()
	if err != nil {
		panic(err)
	}
	return program
}

func (annotator *annotator) annotate(prog *parser.Program) {
	//annotator := &annotator{covermode, 0, map[int]ast.Boundary{}}
	prog.Begin = annotator.annotateStmtsList(prog.Begin)
	prog.Actions = annotator.annotateActions(prog.Actions)
	prog.End = annotator.annotateStmtsList(prog.End)
	prog.Functions = annotator.annotateFunctions(prog.Functions)
	//annotator.addCoverageEnd(prog)
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
	annotator.fileNames[annotator.annotationIdx] = annotator.currentFileName
	annotator.boundaries[annotator.annotationIdx] = ast.Boundary{
		Start: statements[0].(ast.SimpleStmt).GetBoundary().Start,
		End:   statements[len(statements)-1].(ast.SimpleStmt).GetBoundary().End,
	}
	return parseProg(fmt.Sprintf(`BEGIN { __COVER[%d]%s }`, annotator.annotationIdx, op), nil).Begin[0][0]
}

func parseProg(code string, parserConfig *parser.ParserConfig) *parser.Program {
	prog, err := parser.ParseProgram([]byte(code), parserConfig)
	if err != nil {
		panic(err)
	}
	return prog
}

func (annotator *annotator) addCoverageEnd() {
	var code strings.Builder
	code.WriteString("END {\n")
	for i := 1; i <= annotator.annotationIdx; i++ {
		code.WriteString(fmt.Sprintf("__COVER_DATA[%d]=\"%s:%s\"\n",
			i, annotator.fileNames[i], renderCoverBoundary(annotator.boundaries[i])))
	}
	code.WriteString("}")
	annotator.program.End = append(annotator.program.End, parseProg(code.String(), nil).End...)
}

func renderCoverBoundary(boundary ast.Boundary) string {
	return fmt.Sprintf("%d.%d,%d.%d",
		boundary.Start.Line, boundary.Start.Column,
		boundary.End.Line, boundary.End.Column)
}
