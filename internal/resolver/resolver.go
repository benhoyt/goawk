package resolver

import (
	"github.com/benhoyt/goawk/internal/ast"
	"io"
)

type resolver struct {
	// Parsing state
	// TODO this reflects the var in parser - is this needed?
	funcName string // function name if parsing a func, else ""

	// Variable tracking and resolving
	locals    map[string]bool                // current function's locals (for determining scope)
	varTypes  map[string]map[string]typeInfo // map of func name to var name to type
	varRefs   []varRef                       // all variable references (usually scalars)
	arrayRefs []arrayRef                     // all array references
	//multiExprs map[*ast.MultiExpr]lexer.Position // tracks comma-separated expressions

	// Function tracking
	functions   map[string]int // map of function name to index
	userCalls   []userCall     // record calls so we can resolve them later
	nativeFuncs map[string]interface{}

	funcIdx int

	// Configuration and debugging
	debugTypes  bool      // show variable types for debugging
	debugWriter io.Writer // where the debug output goes
}

type Config struct {
	// Enable printing of type information
	DebugTypes bool

	// io.Writer to print type information on (for example, os.Stderr)
	DebugWriter io.Writer

	// Map of named Go functions to allow calling from AWK. See docs
	// on interp.Config.Funcs for details.
	Funcs map[string]interface{}
}

func Resolve(prog *ast.Program, config *Config) *ast.ResolvedProgram {
	r := &resolver{}
	resolvedProg := &ast.ResolvedProgram{
		Program: *prog,
	}
	r.initResolve(config)

	ast.Walk(r, prog)

	r.resolveUserCalls(prog)
	r.resolveVars(resolvedProg)
	//r.checkMultiExprs()

	return resolvedProg
}

func (r *resolver) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {

	case ast.Function:
		function := n
		name := function.Name
		if _, ok := r.functions[name]; ok {
			panic(function.Pos.Errorf("function %q already defined", name))
		}
		r.addFunction(name)
		r.locals = make(map[string]bool, 7)
		for _, param := range function.Params {
			//if r.locals[param] {
			//	panic(function.ParamsPos[i].Errorf( "duplicate parameter name %q", param))
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
		ast.WalkExprList(r, n.Args)
		name := n.Name
		if r.locals[name] {
			panic(n.EndPos.Errorf("can't call local variable %q as function", name))
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
