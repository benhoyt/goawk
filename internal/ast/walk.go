package ast

import "fmt"

// Visitor has a Visit method which is invoked for each node encountered by Walk.
// If the result visitor w is not nil, Walk visits each of the children
// of node with the visitor w, followed by a call of w.Visit(nil).
type Visitor interface {
	Visit(node Node) (w Visitor)
}

// WalkExprList walks a visitor over a list of expression AST nodes
func WalkExprList(v Visitor, exprs []Expr) {
	for _, expr := range exprs {
		Walk(v, expr)
	}
}

// WalkStmtList walks a visitor over a list of statement AST nodes
func WalkStmtList(v Visitor, stmts []Stmt) {
	for _, stmt := range stmts {
		Walk(v, stmt)
	}
}

// Walk traverses an AST in depth-first order: It starts by calling
// v.Visit(node); if node is nil, it does nothing. If the visitor w returned by
// v.Visit(node) is not nil, Walk is invoked recursively with visitor
// w for each of the non-nil children of node, followed by a call of
// w.Visit(nil).
func Walk(v Visitor, node Node) {
	if node == nil {
		return
	}
	if v = v.Visit(node); v == nil {
		return
	}

	// walk children
	// (the order of the cases matches the order
	// of the corresponding node types in ast.go)
	switch n := node.(type) {

	// expressions
	case *FieldExpr:
		Walk(v, n.Index)

	case *NamedFieldExpr:
		Walk(v, n.Field)

	case *UnaryExpr:
		Walk(v, n.Value)

	case *BinaryExpr:
		Walk(v, n.Left)
		Walk(v, n.Right)

	case *InExpr:
		WalkExprList(v, n.Index)

	case *CondExpr:
		Walk(v, n.Cond)
		Walk(v, n.True)
		Walk(v, n.False)

	case *NumExpr: // leaf
	case *StrExpr: // leaf
	case *RegExpr: // leaf
	case *VarExpr: // leaf
	case *IndexExpr:
		WalkExprList(v, n.Index)

	case *AssignExpr:
		Walk(v, n.Left)
		Walk(v, n.Right)

	case *AugAssignExpr:
		Walk(v, n.Left)
		Walk(v, n.Right)

	case *IncrExpr:
		Walk(v, n.Expr)

	case *CallExpr:
		WalkExprList(v, n.Args)

	case *UserCallExpr:
		WalkExprList(v, n.Args)

	case *MultiExpr:
		WalkExprList(v, n.Exprs)

	case *GetlineExpr:
		Walk(v, n.Command)
		Walk(v, n.Target)
		Walk(v, n.File)

	case *GroupingExpr:
		Walk(v, n.Expr)

	// statements
	case *PrintStmt:
		WalkExprList(v, n.Args)
		Walk(v, n.Dest)

	case *PrintfStmt:
		WalkExprList(v, n.Args)
		Walk(v, n.Dest)

	case *ExprStmt:
		Walk(v, n.Expr)

	case *IfStmt:
		Walk(v, n.Cond)
		WalkStmtList(v, n.Body)
		WalkStmtList(v, n.Else)

	case *ForStmt:
		Walk(v, n.Pre)
		Walk(v, n.Cond)
		Walk(v, n.Post)
		WalkStmtList(v, n.Body)

	case *ForInStmt:
		WalkStmtList(v, n.Body)

	case *WhileStmt:
		Walk(v, n.Cond)
		WalkStmtList(v, n.Body)

	case *DoWhileStmt:
		WalkStmtList(v, n.Body)
		Walk(v, n.Cond)

	case *BreakStmt: // leaf
	case *ContinueStmt: // leaf
	case *NextStmt: // leaf
	case *NextfileStmt: // leaf
	case *ExitStmt:
		Walk(v, n.Status)

	case *DeleteStmt:
		WalkExprList(v, n.Index)

	case *ReturnStmt:
		Walk(v, n.Value)

	case *BlockStmt:
		WalkStmtList(v, n.Body)

	case *Program:
		for _, stmts := range n.Begin {
			WalkStmtList(v, stmts)
		}
		for _, action := range n.Actions {
			Walk(v, action)
		}
		for _, function := range n.Functions {
			Walk(v, function)
		}
		for _, stmts := range n.End {
			WalkStmtList(v, stmts)
		}

	case *Action:
		WalkExprList(v, n.Pattern)
		WalkStmtList(v, n.Stmts)

	case *Function:
		WalkStmtList(v, n.Body)

	default:
		panic(fmt.Sprintf("ast.Walk: unexpected node type %T", n))
	}

	v.Visit(nil)
}
