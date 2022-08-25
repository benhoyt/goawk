package resolver

import (
	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/lexer"
)

type resolver struct {
	// Parsing state
	// TODO this reflects the var in parser - is this needed?
	funcName string // function name if parsing a func, else ""

	// Variable tracking and resolving
	locals     map[string]bool                   // current function's locals (for determining scope)
	varTypes   map[string]map[string]typeInfo    // map of func name to var name to type
	varRefs    []varRef                          // all variable references (usually scalars)
	arrayRefs  []arrayRef                        // all array references
	multiExprs map[*ast.MultiExpr]lexer.Position // tracks comma-separated expressions

	// Function tracking
	functions   map[string]int // map of function name to index
	userCalls   []userCall     // record calls so we can resolve them later
	nativeFuncs map[string]interface{}
}

type ResolveResult struct {
}

type Program struct {
	Begin     []ast.Stmts
	Actions   []ast.Action
	End       []ast.Stmts
	Functions []ast.Function
}

type ResolverConfig struct {
	NativeFuncs map[string]interface{}
}

func Resolve(prog *Program, config *ResolverConfig) (resolveResult *ResolveResult, err error) {
	r := &resolver{}
	r.initResolve(config)

	// TODO resolution step to iterate over AST
	// 1. process functions
	for i, function := range prog.Functions {
		name := function.Name
		r.addFunction(name, i)
		if _, ok := r.functions[name]; ok {
			panic(r.errorf("function %q already defined", name))
		}
		r.locals = make(map[string]bool, 7)
		for _, param := range function.Params {
			if r.locals[param] {
				panic(r.errorf("duplicate parameter name %q", param))
			}
			r.locals[param] = true

		}
		r.startFunction(name)
		// TODO process body
		r.stopFunction()
		r.locals = nil
	}

	r.resolveUserCalls(prog)
	r.resolveVars(prog)
	r.checkMultiExprs()

}
