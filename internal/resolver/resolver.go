package resolver

import (
	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
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

// Program represents the resolved program.
type Program struct {
	ast.Program
	Scalars map[string]int
	Arrays  map[string]int
}

func Resolve(prog *ast.Program, config *ResolverConfig) (resolvedProg *Program, err error) {
	// TODO errors handling via panic recover
	r := &resolver{}
	resolvedProg = &Program{
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
			panic(parser.PosErrorf(function.Pos, "function %q already defined", name))
		}
		r.addFunction(name)
		r.locals = make(map[string]bool, 7)
		for _, param := range function.Params {
			//if r.locals[param] {
			//	panic(parser.PosErrorf(function.ParamsPos[i], "duplicate parameter name %q", param))
			//}
			r.locals[param] = true

		}
		r.startFunction(name)

		ast.WalkStmtList(r, function.Body)

		r.stopFunction()
		r.locals = nil

	case *ast.VarExpr:
		r.recordVarRef(n)

	case *ast.ArrayExpr:
		r.recordArrayRef(n)

	case *ast.UserCallExpr:
		name := n.Name
		if r.locals[name] {
			panic(parser.PosErrorf(n.Pos, "can't call local variable %q as function", name))
		}
		for i, arg := range n.Args {
			r.processUserCallArg(name, arg, i)
		}
		r.recordUserCall(n, n.Pos)
	default:
		return r
	}

	return nil
}
