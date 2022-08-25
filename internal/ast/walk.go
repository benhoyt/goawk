// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ast

import "fmt"

// A Visitor's Visit method is invoked for each node encountered by Walk.
// If the result visitor w is not nil, Walk visits each of the children
// of node with the visitor w, followed by a call of w.Visit(nil).
type Visitor interface {
	Visit(node Node) (w Visitor)
}

// Helper functions for common node lists. They may be empty.

func walkExprList(v Visitor, list []Expr) {
	for _, x := range list {
		Walk(v, x)
	}
}

func walkStmtList(v Visitor, list []Stmt) {
	for _, x := range list {
		Walk(v, x)
	}
}

// Walk traverses an AST in depth-first order: It starts by calling
// v.Visit(node); node must not be nil. If the visitor w returned by
// v.Visit(node) is not nil, Walk is invoked recursively with visitor
// w for each of the non-nil children of node, followed by a call of
// w.Visit(nil).
//
func Walk(v Visitor, node Node) {
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

	case *ArrayExpr: // leaf
	case *InExpr:
		walkExprList(v, n.Index)
		Walk(v, n.Array)

	case *CondExpr:
		Walk(v, n.Cond)
		Walk(v, n.True)
		Walk(v, n.False)

	case *NumExpr: // leaf
	case *StrExpr: // leaf
	case *RegExpr: // leaf
	case *VarExpr: // leaf
	case *IndexExpr:
		Walk(v, n.Array)
		walkExprList(v, n.Index)

	case *AssignExpr:
		Walk(v, n.Left)
		Walk(v, n.Right)

	case *AugAssignExpr:
		Walk(v, n.Left)
		Walk(v, n.Right)

	case *IncrExpr:
		Walk(v, n.Expr)

	case *CallExpr:
		walkExprList(v, n.Args)

	case *UserCallExpr:
		walkExprList(v, n.Args)

	case *MultiExpr:
		walkExprList(v, n.Exprs)

	case *GetlineExpr:
		Walk(v, n.Command)
		Walk(v, n.Target)
		Walk(v, n.File)

	// statements
	case *PrintStmt:
		walkExprList(v, n.Args)
		Walk(v, n.Dest)

	case *PrintfStmt:
		walkExprList(v, n.Args)
		Walk(v, n.Dest)

	case *ExprStmt:
		Walk(v, n.Expr)

	case *IfStmt:
		Walk(v, n.Cond)
		walkStmtList(v, n.Body)
		walkStmtList(v, n.Else)

	case *ForStmt:
		Walk(v, n.Pre)
		Walk(v, n.Cond)
		Walk(v, n.Post)
		walkStmtList(v, n.Body)

	case *ForInStmt:
		Walk(v, n.Var)
		Walk(v, n.Array)
		walkStmtList(v, n.Body)

	case *WhileStmt:
		Walk(v, n.Cond)
		walkStmtList(v, n.Body)

	case *DoWhileStmt:
		walkStmtList(v, n.Body)
		Walk(v, n.Cond)

	case *BreakStmt: // leaf
	case *ContinueStmt: // leaf
	case *NextStmt: // leaf
	case *ExitStmt:
		Walk(v, n.Status)

	case *DeleteStmt:
		Walk(v, n.Array)
		walkExprList(v, n.Index)

	case *ReturnStmt:
		Walk(v, n.Value)

	case *BlockStmt:
		walkStmtList(v, n.Body)

	default:
		panic(fmt.Sprintf("ast.Walk: unexpected node type %T", n))
	}

	v.Visit(nil)
}
