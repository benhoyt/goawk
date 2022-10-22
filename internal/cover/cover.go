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

type Mode int

const (
	ModeUnspecified Mode = iota
	ModeSet
	ModeCount
)

func (m Mode) String() string {
	switch m {
	case ModeSet:
		return "set"
	case ModeCount:
		return "count"
	default:
		return fmt.Sprintf("<unknown mode %d>", m)
	}
}

type Cover struct {
	mode          Mode
	append        bool
	fileReader    *parseutil.FileReader
	trackedBlocks []trackedBlock
}

type trackedBlock struct {
	start    lexer.Position
	end      lexer.Position
	path     string
	numStmts int
}

func New(mode Mode, append bool, fileReader *parseutil.FileReader) *Cover {
	return &Cover{
		mode:       mode,
		append:     append,
		fileReader: fileReader,
	}
}

// Annotate annotates the program with coverage tracking code.
func (cover *Cover) Annotate(prog *ast.Program) {
	prog.Begin = cover.annotateStmtsList(prog.Begin)
	prog.Actions = cover.annotateActions(prog.Actions)
	prog.End = cover.annotateStmtsList(prog.End)
	prog.Functions = cover.annotateFunctions(prog.Functions)
}

// WriteProfile writes coverage data to a file at the given path.
func (cover *Cover) WriteProfile(path string, data map[string]interface{}) error {
	// 1a. If file doesn't exist - create and write cover mode line
	// 1b. If file exists and coverappend=true  - open it for writing in append mode
	// 1c. If file exists and coverappend=false - truncate it and follow 1a.
	// 2.  Write all cover data lines

	dataInts, err := dataToInts(data)
	if err != nil {
		return err
	}
	isNewFile := true

	var f *os.File
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
	} else if err == nil {
		// file exists
		fileOpt := os.O_TRUNC
		if cover.append {
			isNewFile = false
			fileOpt = os.O_APPEND
		}
		f, err = os.OpenFile(path, os.O_WRONLY|fileOpt, 0644)
		if err != nil {
			return err
		}
	} else {
		return err
	}

	if isNewFile {
		_, err := fmt.Fprintf(f, "mode: %s\n", cover.mode)
		if err != nil {
			return err
		}
	}

	for i, block := range cover.trackedBlocks {
		_, err := fmt.Fprintf(f, "%s:%d.%d,%d.%d %d %d\n",
			toAbsolutePath(block.path),
			block.start.Line, block.start.Column,
			block.end.Line, block.end.Column,
			block.numStmts, dataInts[i+1],
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func dataToInts(data map[string]interface{}) (map[int]int, error) {
	res := make(map[int]int)
	for k, v := range data {
		ki, err := strconv.Atoi(k)
		if err != nil {
			return nil, fmt.Errorf("non-int index in data: %s", k)
		}
		vf, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("non-float64 value in data: %v", v)
		}
		res[ki] = int(vf)
	}
	return res, nil
}

func (cover *Cover) annotateActions(actions []*ast.Action) []*ast.Action {
	if actions == nil {
		return nil
	}
	res := make([]*ast.Action, 0, len(actions))
	for _, action := range actions {
		action.Stmts = cover.annotateStmts(action.Stmts)
		res = append(res, action)
	}
	return res
}

func (cover *Cover) annotateFunctions(functions []*ast.Function) []*ast.Function {
	if functions == nil {
		return nil
	}
	res := make([]*ast.Function, 0, len(functions))
	for _, function := range functions {
		function.Body = cover.annotateStmts(function.Body)
		res = append(res, function)
	}
	return res
}

func (cover *Cover) annotateStmtsList(stmtsList []ast.Stmts) []ast.Stmts {
	if stmtsList == nil {
		return nil
	}
	res := make([]ast.Stmts, 0, len(stmtsList))
	for _, stmts := range stmtsList {
		res = append(res, cover.annotateStmts(stmts))
	}
	return res
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
func (cover *Cover) annotateStmts(stmts ast.Stmts) ast.Stmts {
	var res ast.Stmts
	var trackedBlockStmts []ast.Stmt
	for _, stmt := range stmts {
		blockEnds := true
		switch s := stmt.(type) {
		case *ast.IfStmt:
			s.Body = cover.annotateStmts(s.Body)
			s.Else = cover.annotateStmts(s.Else)
		case *ast.ForStmt:
			s.Body = cover.annotateStmts(s.Body)
		case *ast.ForInStmt:
			s.Body = cover.annotateStmts(s.Body)
		case *ast.WhileStmt:
			s.Body = cover.annotateStmts(s.Body)
		case *ast.DoWhileStmt:
			s.Body = cover.annotateStmts(s.Body)
		case *ast.BlockStmt:
			s.Body = cover.annotateStmts(s.Body)
		default:
			blockEnds = false
		}
		trackedBlockStmts = append(trackedBlockStmts, stmt)
		if blockEnds {
			res = append(res, cover.trackStatement(trackedBlockStmts))
			res = append(res, trackedBlockStmts...)
			trackedBlockStmts = []ast.Stmt{}
		}
	}
	if len(trackedBlockStmts) > 0 {
		res = append(res, cover.trackStatement(trackedBlockStmts))
		res = append(res, trackedBlockStmts...)
	}
	return res
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

func (cover *Cover) trackStatement(stmts []ast.Stmt) ast.Stmt {
	start1 := stmts[0].StartPos()
	end2 := endPos(stmts[len(stmts)-1])
	path, startLine := cover.fileReader.FileLine(start1.Line)
	_, endLine := cover.fileReader.FileLine(end2.Line)
	cover.trackedBlocks = append(cover.trackedBlocks, trackedBlock{
		start:    lexer.Position{startLine, start1.Column},
		end:      lexer.Position{endLine, end2.Column},
		path:     path,
		numStmts: len(stmts),
	})
	left := &ast.IndexExpr{
		Array: ast.ArrayRef(ArrayName, lexer.Position{}),
		Index: []ast.Expr{&ast.StrExpr{Value: strconv.Itoa(len(cover.trackedBlocks))}},
	}
	if cover.mode == ModeCount {
		// AST for __COVER[index]++
		return &ast.ExprStmt{Expr: &ast.IncrExpr{Expr: left, Op: lexer.INCR}}
	}
	// AST for __COVER[index] = 1
	return &ast.ExprStmt{Expr: &ast.AssignExpr{Left: left, Right: &ast.NumExpr{Value: 1}}}
}

func toAbsolutePath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}
