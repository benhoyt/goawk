package main

import (
	"fmt"
	"os"

	. "github.com/benhoyt/goawk/internal/ast"
	. "github.com/benhoyt/goawk/lexer"
	. "github.com/benhoyt/goawk/parser"
)

type typer struct {
	globals map[string]valueType
	exprs   map[Expr]valueType
}

func newTyper() *typer {
	t := &typer{
		globals: make(map[string]valueType),
		exprs:   make(map[Expr]valueType),
	}
	t.globals["OFS"] = typeStr
	t.globals["ORS"] = typeStr
	return t
}

func (t *typer) dump() {
	fmt.Fprintf(os.Stderr, "// DUMP:\n")
	for name, t := range t.globals {
		fmt.Fprintf(os.Stderr, "// var %s: %s\n", name, t)
	}
	for expr, t := range t.exprs {
		fmt.Fprintf(os.Stderr, "// expr %s: %s\n", expr, t)
	}
}

func (t *typer) program(prog *Program) {
	for _, stmts := range prog.Begin {
		t.stmts(stmts)
	}
	t.actions(prog.Actions)
	for _, stmts := range prog.End {
		t.stmts(stmts)
	}
	for range prog.Functions {
		panic(errorf("functions not yet supported"))
	}
}

func (t *typer) stmts(stmts Stmts) {
	for _, stmt := range stmts {
		t.stmt(stmt)
	}
}

func (t *typer) actions(actions []Action) {
	for _, action := range actions {
		for _, e := range action.Pattern {
			t.expr(e)
		}
		t.stmts(action.Stmts)
	}
}

func (t *typer) stmt(stmt Stmt) {
	switch s := stmt.(type) {
	case *PrintStmt:
		for _, arg := range s.Args {
			t.expr(arg)
		}
		if s.Dest != nil {
			t.expr(s.Dest)
		}

	case *PrintfStmt:
		for _, arg := range s.Args {
			t.expr(arg)
		}
		if s.Dest != nil {
			t.expr(s.Dest)
		}

	case *ExprStmt:
		t.expr(s.Expr)

	case *IfStmt:
		t.expr(s.Cond)
		t.stmts(s.Body)
		t.stmts(s.Else)

	case *ForStmt:
		if s.Pre != nil {
			t.stmt(s.Pre)
		}
		if s.Cond != nil {
			t.expr(s.Cond)
		}
		if s.Post != nil {
			t.stmt(s.Post)
		}
		t.stmts(s.Body)

	case *ForInStmt:
		t.setType(s.Var.Name, typeStr)
		t.stmts(s.Body)

	case *WhileStmt:
		t.expr(s.Cond)
		t.stmts(s.Body)

	case *DoWhileStmt:
		t.stmts(s.Body)
		t.expr(s.Cond)

	case *BreakStmt, *ContinueStmt, *NextStmt:
		return

	case *ExitStmt:
		if s.Status != nil {
			t.expr(s.Status)
		}

	case *DeleteStmt:
		for _, index := range s.Index {
			t.expr(index)
		}

	case *ReturnStmt:
		if s.Value != nil {
			t.expr(s.Value)
		}

	case *BlockStmt:
		t.stmts(s.Body)

	default:
		panic(errorf("unexpected statement type %T", stmt))
	}
}

func (t *typer) setType(name string, typ valueType) {
	if t.globals[name] == typ {
		return
	}
	if t.globals[name] != typeUnknown {
		panic(errorf("variable %q already set to %s, can't set to %s",
			name, t.globals[name], typ))
	}
	if typ != typeUnknown {
		t.globals[name] = typ
	}
}

func (t *typer) expr(expr Expr) (typ valueType) {
	defer func() {
		if typ != typeUnknown {
			t.exprs[expr] = typ
		}
	}()

	switch e := expr.(type) {
	case *FieldExpr:
		t.expr(e.Index)
		return typeStr // TODO: should be typeNumStr

	case *UnaryExpr:
		t.expr(e.Value)
		return typeNum

	case *BinaryExpr:
		t.expr(e.Left)
		t.expr(e.Right)
		if e.Op == CONCAT {
			return typeStr
		}
		return typeNum

	case *ArrayExpr:
		return typeUnknown

	case *InExpr:
		for _, index := range e.Index {
			t.expr(index)
		}
		t.expr(e.Array)
		return typeNum

	case *CondExpr:
		t.expr(e.Cond)
		trueType := t.expr(e.True)
		falseType := t.expr(e.False)
		if trueType != falseType {
			panic(errorf("both branches of ?: must yield same type (first is %s, second is %s)",
				trueType, falseType))
		}
		return trueType

	case *NumExpr:
		return typeNum

	case *StrExpr:
		return typeStr

	case *RegExpr:
		return typeNum

	case *VarExpr:
		switch e.Scope {
		case ScopeSpecial:
			return t.specialType(e.Name, e.Index)
		case ScopeGlobal:
			return t.globals[e.Name]
		default:
			panic(errorf("unexpected scope %v", e.Scope))
		}

	case *IndexExpr:
		t.expr(e.Array)
		for _, index := range e.Index {
			t.expr(index)
		}
		switch t.globals[e.Array.Name] {
		case typeArrayStr:
			return typeStr
		case typeArrayNum:
			return typeNum
		}
		return typeUnknown

	case *AssignExpr:
		rightType := t.expr(e.Right)
		switch left := e.Left.(type) {
		case *VarExpr:
			// x = right
			t.setType(left.Name, rightType)
		case *IndexExpr:
			// m[k] = right
			switch rightType {
			case typeStr:
				t.setType(left.Array.Name, typeArrayStr)
			case typeNum:
				t.setType(left.Array.Name, typeArrayNum)
			}
		case *FieldExpr:
			// $1 = right
		}
		t.expr(e.Left)
		return rightType

	case *AugAssignExpr:
		t.expr(e.Right)
		switch left := e.Left.(type) {
		case *VarExpr:
			// x += right
			t.setType(left.Name, typeNum)
		case *IndexExpr:
			// m[k] += right
			t.setType(left.Array.Name, typeArrayNum)
		case *FieldExpr:
			// $1 += right
			// TODO: this should probably return typeStr
		}
		t.expr(e.Left)
		return typeNum

	case *IncrExpr:
		switch left := e.Expr.(type) {
		case *VarExpr:
			// x++
			t.setType(left.Name, typeNum)
		case *IndexExpr:
			// m[k]++
			t.setType(left.Array.Name, typeArrayNum)
		case *FieldExpr:
			// $1++
			// TODO: this should probably return typeStr
		}
		t.expr(e.Expr)
		return typeNum

	case *CallExpr:
		for _, arg := range e.Args {
			t.expr(arg)
		}
		switch e.Func {
		case F_ATAN2, F_CLOSE, F_COS, F_EXP, F_FFLUSH, F_GSUB, F_INDEX, F_INT,
			F_LENGTH, F_LOG, F_MATCH, F_RAND, F_SIN, F_SPLIT, F_SQRT, F_SRAND,
			F_SUB, F_SYSTEM:
			return typeNum
		case F_SPRINTF, F_SUBSTR, F_TOLOWER, F_TOUPPER:
			return typeStr
		default:
			panic(errorf("unexpected function %s", e.Func))
		}

	case *UserCallExpr:
		panic(errorf("functions not yet supported"))

	case *GetlineExpr:
		return typeNum

	default:
		panic(errorf("unexpected expression type %T", expr))
	}
	return typeUnknown
}

func (t *typer) specialType(name string, index int) valueType {
	switch index {
	case V_NF, V_NR, V_RLENGTH, V_RSTART, V_FNR, V_ARGC:
		return typeNum
	case V_CONVFMT, V_FILENAME, V_FS, V_OFMT, V_OFS, V_ORS, V_RS, V_SUBSEP:
		return typeStr
	default:
		panic(errorf("unexpected special variable %s", name))
	}
}
