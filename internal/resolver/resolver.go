package resolver

import (
	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/compiler"
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

	funcIdx int
}

type ResolverConfig struct {
	NativeFuncs map[string]interface{}
}

func Resolve(prog *ast.Program, config *ResolverConfig) (resolvedProg *compiler.Program, err error) {
	r := &resolver{}
	resolvedProg = &compiler.Program{
		Program: *prog,
	}
	r.initResolve(config)

	ast.Walk(r, prog)

	r.resolveUserCalls(prog)
	r.resolveVars(resolvedProg)
	//r.checkMultiExprs()

	return resolvedProg, nil
}

func (r *resolver) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {

	case ast.Function:
		function := n
		name := function.Name
		if _, ok := r.functions[name]; ok {
			panic(r.errorf("function %q already defined", name))
		}
		r.addFunction(name)
		r.locals = make(map[string]bool, 7)
		for _, param := range function.Params {
			if r.locals[param] {
				panic(r.errorf("duplicate parameter name %q", param))
			}
			r.locals[param] = true

		}
		r.startFunction(name)

		ast.WalkStmtList(r, function.Body)

		r.stopFunction()
		r.locals = nil

	case *ast.UserCallExpr:
		name := n.Name
		if r.locals[name] {
			panic(r.errorf("can't call local variable %q as function", name))
		}
		for i, arg := range n.Args {
			r.processUserCallArg(name, arg, i)
		}
		r.recordUserCall(n, pos)
	default:
		return r
	}

	return nil
}
