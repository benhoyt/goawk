// AWKGo: walk parse tree and determine expression and variable types

package main

import (
	"github.com/benhoyt/goawk/internal/ast"
	. "github.com/benhoyt/goawk/lexer"
)

// typer walks the parse tree and builds a mappings of variables and
// expressions to their types.
type typer struct {
	globals      map[string]valueType
	scalarRefs   map[string]bool
	arrayRefs    map[string]bool
	exprs        map[ast.Expr]valueType
	funcName     string // function name if inside a func, else ""
	nextUsed     bool
	oFSRSChanged bool
}

func newTyper() *typer {
	t := &typer{
		globals:    make(map[string]valueType),
		scalarRefs: make(map[string]bool),
		arrayRefs:  make(map[string]bool),
		exprs:      make(map[ast.Expr]valueType),
	}
	t.globals["FS"] = typeStr
	t.globals["OFS"] = typeStr
	t.globals["ORS"] = typeStr
	t.globals["OFMT"] = typeStr
	t.globals["CONVFMT"] = typeStr
	t.globals["RSTART"] = typeNum
	t.globals["RLENGTH"] = typeNum
	t.globals["SUBSEP"] = typeStr
	return t
}

func (t *typer) program(prog *ast.Program) {
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

	for name := range t.scalarRefs {
		if t.globals[name] == typeUnknown {
			panic(errorf("type of %q not known; need assignment?", name))
		}
	}
	for name := range t.arrayRefs {
		if t.globals[name] == typeUnknown {
			panic(errorf("type of array %q not known; need array assignment?", name))
		}
	}
}

func (t *typer) stmts(stmts ast.Stmts) {
	for _, stmt := range stmts {
		t.stmt(stmt)
	}
}

func (t *typer) actions(actions []ast.Action) {
	for _, action := range actions {
		for _, e := range action.Pattern {
			t.expr(e)
		}
		t.stmts(action.Stmts)
	}
}

func (t *typer) stmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.PrintStmt:
		for _, arg := range s.Args {
			t.expr(arg)
		}
		if s.Dest != nil {
			t.expr(s.Dest)
		}

	case *ast.PrintfStmt:
		for _, arg := range s.Args {
			t.expr(arg)
		}
		if s.Dest != nil {
			t.expr(s.Dest)
		}

	case *ast.ExprStmt:
		t.expr(s.Expr)

	case *ast.IfStmt:
		t.expr(s.Cond)
		t.stmts(s.Body)
		t.stmts(s.Else)

	case *ast.ForStmt:
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

	case *ast.ForInStmt:
		t.setType(s.Var.Name, typeStr)
		t.stmts(s.Body)

	case *ast.WhileStmt:
		t.expr(s.Cond)
		t.stmts(s.Body)

	case *ast.DoWhileStmt:
		t.stmts(s.Body)
		t.expr(s.Cond)

	case *ast.BreakStmt, *ast.ContinueStmt:
		return

	case *ast.NextStmt:
		if t.funcName != "" {
			panic(errorf(`"next" inside a function not yet supported`))
		}
		t.nextUsed = true
		return

	case *ast.ExitStmt:
		if s.Status != nil {
			t.expr(s.Status)
		}

	case *ast.DeleteStmt:
		for _, index := range s.Index {
			t.expr(index)
		}

	case *ast.ReturnStmt:
		if s.Value != nil {
			t.expr(s.Value)
		}

	case *ast.BlockStmt:
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

func (t *typer) expr(expr ast.Expr) (typ valueType) {
	defer func() {
		if typ != typeUnknown {
			t.exprs[expr] = typ
		}
	}()

	switch e := expr.(type) {
	case *ast.FieldExpr:
		t.expr(e.Index)
		return typeStr

	case *ast.UnaryExpr:
		t.expr(e.Value)
		return typeNum

	case *ast.BinaryExpr:
		t.expr(e.Left)
		t.expr(e.Right)
		if e.Op == CONCAT {
			return typeStr
		}
		return typeNum

	case *ast.ArrayExpr:
		return typeUnknown

	case *ast.InExpr:
		for _, index := range e.Index {
			t.expr(index)
		}
		t.expr(e.Array)
		return typeNum

	case *ast.CondExpr:
		t.expr(e.Cond)
		trueType := t.expr(e.True)
		falseType := t.expr(e.False)
		if trueType != falseType {
			panic(errorf("both branches of ?: must yield same type (first is %s, second is %s)",
				trueType, falseType))
		}
		return trueType

	case *ast.NumExpr:
		return typeNum

	case *ast.StrExpr:
		return typeStr

	case *ast.RegExpr:
		return typeNum

	case *ast.VarExpr:
		switch e.Scope {
		case ast.ScopeSpecial:
			return t.specialType(e.Name, e.Index)
		case ast.ScopeGlobal:
			t.scalarRefs[e.Name] = true
			return t.globals[e.Name]
		default:
			panic(errorf("unexpected scope %v", e.Scope))
		}

	case *ast.IndexExpr:
		t.arrayRefs[e.Array.Name] = true
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

	case *ast.AssignExpr:
		rightType := t.expr(e.Right)
		switch left := e.Left.(type) {
		case *ast.VarExpr:
			// x = right
			t.setType(left.Name, rightType)
			if left.Name == "OFS" || left.Name == "ORS" {
				t.oFSRSChanged = true
			}
		case *ast.IndexExpr:
			// m[k] = right
			switch rightType {
			case typeStr:
				t.setType(left.Array.Name, typeArrayStr)
			case typeNum:
				t.setType(left.Array.Name, typeArrayNum)
			}
		case *ast.FieldExpr:
			// $1 = right
		}
		t.expr(e.Left)
		return rightType

	case *ast.AugAssignExpr:
		t.expr(e.Right)
		switch left := e.Left.(type) {
		case *ast.VarExpr:
			// x += right
			t.setType(left.Name, typeNum)
			if left.Name == "OFS" || left.Name == "ORS" {
				t.oFSRSChanged = true
			}
		case *ast.IndexExpr:
			// m[k] += right
			t.setType(left.Array.Name, typeArrayNum)
		case *ast.FieldExpr:
			// $1 += right
		}
		t.expr(e.Left)
		return typeNum

	case *ast.IncrExpr:
		switch left := e.Expr.(type) {
		case *ast.VarExpr:
			// x++
			t.setType(left.Name, typeNum)
			if left.Name == "OFS" || left.Name == "ORS" {
				t.oFSRSChanged = true
			}
		case *ast.IndexExpr:
			// m[k]++
			t.setType(left.Array.Name, typeArrayNum)
		case *ast.FieldExpr:
			// $1++
		}
		t.expr(e.Expr)
		return typeNum

	case *ast.CallExpr:
		switch e.Func {
		case F_SPLIT:
			// split's second arg is an array arg
			t.expr(e.Args[0])
			arrayExpr := e.Args[1].(*ast.ArrayExpr)
			if t.globals[arrayExpr.Name] != typeUnknown && t.globals[arrayExpr.Name] != typeArrayStr {
				panic(errorf("%q already set to %s, can't use as %s in split()",
					arrayExpr.Name, t.globals[arrayExpr.Name], typeArrayStr))
			}
			t.globals[arrayExpr.Name] = typeArrayStr
			if len(e.Args) == 3 {
				t.expr(e.Args[2])
			}
			return typeNum
		case F_SUB, F_GSUB:
			t.expr(e.Args[0])
			t.expr(e.Args[1])
			if len(e.Args) == 3 {
				// sub and gsub's third arg is actually an lvalue
				switch left := e.Args[2].(type) {
				case *ast.VarExpr:
					t.setType(left.Name, typeStr)
				case *ast.IndexExpr:
					t.setType(left.Array.Name, typeArrayStr)
				}
			}
			return typeNum
		}
		for _, arg := range e.Args {
			t.expr(arg)
		}
		switch e.Func {
		case F_ATAN2, F_CLOSE, F_COS, F_EXP, F_FFLUSH, F_INDEX, F_INT, F_LENGTH,
			F_LOG, F_MATCH, F_RAND, F_SIN, F_SQRT, F_SRAND, F_SYSTEM:
			return typeNum
		case F_SPRINTF, F_SUBSTR, F_TOLOWER, F_TOUPPER:
			return typeStr
		default:
			panic(errorf("unexpected function %s", e.Func))
		}

	case *ast.UserCallExpr:
		panic(errorf("functions not yet supported"))

	case *ast.GetlineExpr:
		return typeNum

	default:
		panic(errorf("unexpected expression type %T", expr))
	}
}

func (t *typer) specialType(name string, index int) valueType {
	switch index {
	case ast.V_NF, ast.V_NR, ast.V_RLENGTH, ast.V_RSTART, ast.V_FNR, ast.V_ARGC:
		return typeNum
	case ast.V_CONVFMT, ast.V_FILENAME, ast.V_FS, ast.V_OFMT, ast.V_OFS, ast.V_ORS, ast.V_RS, ast.V_SUBSEP:
		return typeStr
	default:
		panic(errorf("unexpected special variable %s", name))
	}
}
