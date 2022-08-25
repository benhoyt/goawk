package resolver

import "github.com/benhoyt/goawk/internal/ast"

type resolver struct {
}

type ResolveResult struct {
}

type Program struct {
	Begin     []ast.Stmts
	Actions   []ast.Action
	End       []ast.Stmts
	Functions []ast.Function
}

func Resolve(prog *Program) (resolveResult *ResolveResult, err error) {

}
