// Package resolver assigns integer indexes to functions and variables, as
// well as determining and checking their types (scalar or array).
package resolver

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/lexer"
)

// ResolvedProgram is a parsed AWK program plus variable scope and type data
// prepared by the resolver that is needed for subsequent interpretation.
type ResolvedProgram struct {
	ast.Program
	resolver *resolver
}

// LookupVar looks up a (possibly-local) variable by function name and
// variable name, returning its scope, info, and whether it exists.
func (r *ResolvedProgram) LookupVar(funcName, name string) (Scope, VarInfo, bool) {
	scope, info, _, exists := r.resolver.lookupVar(funcName, name)
	return scope, info, exists
}

// IterVars iterates over the variables from the given function ("" to iterate
// globals), calling f for each variable.
func (r *ResolvedProgram) IterVars(funcName string, f func(name string, info VarInfo)) {
	for name, info := range r.resolver.varInfo[funcName] {
		f(name, info)
	}
}

// LookupFunc looks up a function by name, returning its info and whether it
// exists.
func (r *ResolvedProgram) LookupFunc(name string) (FuncInfo, bool) {
	info, ok := r.resolver.funcInfo[name]
	return info, ok
}

// IterFuncs iterates over all the functions, including native (Go-defined)
// ones, calling f for each function.
func (r *ResolvedProgram) IterFuncs(f func(name string, info FuncInfo)) {
	for name, info := range r.resolver.funcInfo {
		f(name, info)
	}
}

// VarInfo holds resolved information about a variable.
type VarInfo struct {
	Type  Type
	Index int
}

// FuncInfo holds resolved information about a function.
type FuncInfo struct {
	Native bool // true if function is a native (Go-defined) function
	Index  int
	Params []string // list of parameter names
}

// Scope represents the scope of a variable.
type Scope int

const (
	Local   Scope = iota + 1 // locals (function parameters)
	Special                  // special variables (such as NF)
	Global                   // globals
)

func (s Scope) String() string {
	switch s {
	case Local:
		return "local"
	case Special:
		return "special"
	case Global:
		return "global"
	default:
		return "unknown scope"
	}
}

// Type represents the type of a variable: scalar or array.
type Type int

const (
	unknown Type = iota
	Scalar
	Array
)

func (t Type) String() string {
	switch t {
	case Scalar:
		return "scalar"
	case Array:
		return "array"
	default:
		return "unknown type"
	}
}

// Config holds resolver configuration.
type Config struct {
	// Enable printing of type information
	DebugTypes bool

	// io.Writer to print type information on (for example, os.Stderr)
	DebugWriter io.Writer

	// Map of named Go functions to allow calling from AWK. See docs
	// on interp.Config.Funcs for details.
	Funcs map[string]interface{}
}

// Resolve assigns integer indexes to functions and variables, as well as
// determining and checking their types (scalar or array).
func Resolve(prog *ast.Program, config *Config) *ResolvedProgram {
	if config == nil {
		config = &Config{}
	}

	// Assign indexes to native (Go-defined) functions, in order of name.
	// Do this before our first pass, so that AWK-defined functions override
	// Go-defined ones and take precedence.
	funcInfo := make(map[string]FuncInfo)
	var nativeNames []string
	for name := range config.Funcs {
		nativeNames = append(nativeNames, name)
	}
	sort.Strings(nativeNames)
	for i, name := range nativeNames {
		funcInfo[name] = FuncInfo{Native: true, Index: i}
	}

	// First pass determines call graph so we can process functions in
	// topological order: e.g., if f() calls g(), process g first, then f.
	callGraph := callGraphVisitor{
		calls:    make(map[string]map[string]struct{}),
		funcs:    make(map[string]*ast.Function),
		funcInfo: funcInfo,
	}
	ast.Walk(&callGraph, prog)
	orderedFuncs := topoSort(callGraph.calls)

	// Ensure functions that weren't called are added to the orderedFuncs list
	// (order of those doesn't matter, so add them at the end).
	called := make(map[string]struct{}, len(orderedFuncs))
	for _, name := range orderedFuncs {
		called[name] = struct{}{}
	}
	for name := range callGraph.funcs {
		if _, ok := called[name]; !ok {
			orderedFuncs = append(orderedFuncs, name)
		}
	}

	// Define the local variable names (we don't know their types yet).
	varInfo := make(map[string]map[string]VarInfo)
	for funcName, info := range funcInfo {
		if info.Native {
			continue
		}
		varInfo[funcName] = make(map[string]VarInfo)
		for _, param := range info.Params {
			varInfo[funcName][param] = VarInfo{}
		}
	}

	// Create our type resolver.
	r := resolver{varInfo: varInfo, funcInfo: funcInfo, funcs: callGraph.funcs}
	r.varInfo[""] = make(map[string]VarInfo) // func of "" stores global vars

	// Interpreter relies on ARGV and other built-in arrays being present.
	r.recordVar("", "ARGV", Array, lexer.Position{1, 1})
	r.recordVar("", "ENVIRON", Array, lexer.Position{1, 1})
	r.recordVar("", "FIELDS", Array, lexer.Position{1, 1})

	// Main resolver pass: determine types of variables and find function
	// information. Can't call ast.Walk on prog directly, as it will not
	// iterate through functions in topological (call graph) order.
	main := mainVisitor{r: &r, nativeFuncs: config.Funcs}
	updates := r.updates
	main.walkOrdered(prog, orderedFuncs)

	// Do additional passes while we're still making type updates. Topological
	// sorting takes care of ordinary call graphs, but additional passes are
	// needed for at least these two cases:
	//
	// 1. Functions which don't use their parameters, such as f1's A parameter
	//    in this example:
	//
	//  function f1(A) {}  function f2(x, A) { x[0]; f1(a); f2(a) }
	//
	// 2. For complex mutually-recursive functions, such as this example:
	//
	//  function f1(a) { if (0) f5(z1); f2(a) }
	//  function f2(b) { if (0) f4(z2); f3(b) }
	//  function f3(c) { if (0) f3(z3); f4(c) }
	//  function f4(d) { if (0) f2(z4); f5(d) }
	//  function f5(i) { if (0) f1(z5); i[1]=42 }
	//  BEGIN { x[1]=3; f5(x); print x[1] }
	//
	// Limit it to a sensible maximum number of iterations that almost
	// certainly won't happen in the real world.
	for i := 0; r.updates != updates; i++ {
		updates = r.updates
		main.walkOrdered(prog, orderedFuncs)
		if i >= 100 {
			panic(ast.PosErrorf(lexer.Position{1, 1},
				"too many iterations trying to resolve variable types"))
		}
	}

	// For any variables that are still unknown, set their type to scalar.
	// This can happen for unused variables, such as in the following:
	//  { f(z) }  function f(x) { print NR }
	for _, infos := range r.varInfo {
		for varName, info := range infos {
			if info.Type == unknown {
				infos[varName] = VarInfo{Type: Scalar, Index: info.Index}
			}
		}
	}

	// Assign indexes to globals and locals (separate for scalars and arrays).
	for funcName, infos := range r.varInfo {
		var names []string
		if funcName == "" {
			// For global vars, order indexes by name.
			for name := range infos {
				names = append(names, name)
			}
			sort.Strings(names)
		} else {
			// For local vars, order indexes by parameter order.
			names = r.funcInfo[funcName].Params
		}
		scalar := 0
		array := 0
		for _, name := range names {
			info := infos[name]
			if info.Type == Array {
				infos[name] = VarInfo{Type: info.Type, Index: array}
				array++
			} else {
				infos[name] = VarInfo{Type: info.Type, Index: scalar}
				scalar++
			}
		}
	}

	if config.DebugTypes {
		printVarTypes(config.DebugWriter, r.varInfo, r.funcInfo)
	}

	return &ResolvedProgram{
		Program:  *prog,
		resolver: &r,
	}
}

// Print variable type information (for debugging) on given writer.
func printVarTypes(w io.Writer, varInfo map[string]map[string]VarInfo, funcInfo map[string]FuncInfo) {
	var funcNames []string
	for funcName := range varInfo {
		funcNames = append(funcNames, funcName)
	}
	sort.Strings(funcNames)
	for _, funcName := range funcNames {
		if funcName != "" {
			info := funcInfo[funcName]
			fmt.Fprintf(w, "function %s(%s)  # index %d\n",
				funcName, strings.Join(info.Params, ", "), info.Index)
		} else {
			fmt.Fprintln(w, "globals")
		}
		var varNames []string
		for name := range varInfo[funcName] {
			varNames = append(varNames, name)
		}
		sort.Strings(varNames)
		for _, name := range varNames {
			info := varInfo[funcName][name]
			fmt.Fprintf(w, "  %s: %s %d\n", name, info.Type, info.Index)
		}
	}
}

// resolver tracks variable scopes and types as well as function information.
type resolver struct {
	varInfo  map[string]map[string]VarInfo
	funcInfo map[string]FuncInfo
	funcs    map[string]*ast.Function
	updates  int
}

// Look up variable from function funcName and return its scope and type
// information, the function it was defined in, and whether it exists.
func (r *resolver) lookupVar(funcName, varName string) (scope Scope, info VarInfo, varFunc string, exists bool) {
	// If inside a function, try looking for a local variable first.
	if funcName != "" {
		if info, exists = r.varInfo[funcName][varName]; exists {
			return Local, info, funcName, true
		}
	}
	// Next try looking for a special variable (such as NR).
	index := ast.SpecialVarIndex(varName)
	if index > 0 {
		// Special variables are all scalar (ARGV and similar are done as
		// regular arrays).
		return Special, VarInfo{Type: Scalar, Index: index}, "", true
	}
	// Then try looking for a global variable.
	if info, exists = r.varInfo[""][varName]; exists {
		return Global, info, "", true
	}
	return 0, VarInfo{}, "", false // not defined at all
}

// Record that the given variable (in function funcName) is of the given type.
func (r *resolver) recordVar(funcName, varName string, typ Type, pos lexer.Position) {
	_, info, varFunc, exists := r.lookupVar(funcName, varName)
	if !exists {
		// Doesn't exist as a local or a global, add it as a new global.
		r.varInfo[""][varName] = VarInfo{Type: typ}
		r.updates++
		if _, isFunc := r.funcs[varName]; isFunc {
			panic(ast.PosErrorf(pos, "global var %q can't also be a function", varName))
		}
		return
	}
	if info.Type != typ && info.Type != unknown && typ != unknown {
		panic(ast.PosErrorf(pos, "can't use %s %q as %s", info.Type, varName, typ))
	}
	if info.Type == unknown && typ != unknown {
		r.varInfo[varFunc][varName] = VarInfo{Type: typ, Index: info.Index}
		r.updates++
	}
}

// callGraphVisitor records what functions are called by the current function
// to build our call graph.
type callGraphVisitor struct {
	calls    map[string]map[string]struct{} // map of current function to called function
	funcs    map[string]*ast.Function
	funcInfo map[string]FuncInfo
	curFunc  string
}

func (v *callGraphVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.Function:
		if _, ok := v.funcs[n.Name]; ok {
			panic(ast.PosErrorf(n.Pos, "function %q already defined", n.Name))
		}
		v.funcInfo[n.Name] = FuncInfo{Index: len(v.funcs), Params: n.Params}
		v.funcs[n.Name] = n
		v.curFunc = n.Name
		ast.WalkStmtList(v, n.Body)
		v.curFunc = ""

	case *ast.UserCallExpr:
		if _, ok := v.calls[v.curFunc]; !ok {
			v.calls[v.curFunc] = make(map[string]struct{})
		}
		v.calls[v.curFunc][n.Name] = struct{}{}
		ast.WalkExprList(v, n.Args)

	default:
		return v
	}
	return nil
}

// mainVisitor records types of variables and performs various checks.
type mainVisitor struct {
	r           *resolver
	nativeFuncs map[string]interface{}
	curFunc     string
}

// Walk prog's AST, with functions walked as ordered by orderedFuncs.
func (v *mainVisitor) walkOrdered(prog *ast.Program, orderedFuncs []string) {
	for _, funcName := range orderedFuncs {
		if funcName == "" {
			continue // BEGIN, END, and actions are processed below
		}
		function, exists := v.r.funcs[funcName]
		if !exists {
			// Happens in the case where someone tries to call a local
			// variable as a function: function f(x) { x() }. That is checked
			// and flagged as an error in the visitor.
			continue
		}
		v.curFunc = funcName
		ast.WalkStmtList(v, function.Body)
		v.curFunc = ""
	}
	for _, stmts := range prog.Begin {
		ast.WalkStmtList(v, stmts)
	}
	for _, action := range prog.Actions {
		ast.Walk(v, action)
	}
	for _, stmts := range prog.End {
		ast.WalkStmtList(v, stmts)
	}
}

func (v *mainVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.VarExpr:
		v.r.recordVar(v.curFunc, n.Name, Scalar, n.Pos)

	case *ast.ForInStmt:
		v.r.recordVar(v.curFunc, n.Var, Scalar, n.VarPos)
		v.r.recordVar(v.curFunc, n.Array, Array, n.ArrayPos)
		ast.WalkStmtList(v, n.Body)

	case *ast.IndexExpr:
		ast.WalkExprList(v, n.Index)
		v.r.recordVar(v.curFunc, n.Array, Array, n.ArrayPos)

	case *ast.InExpr:
		ast.WalkExprList(v, n.Index)
		v.r.recordVar(v.curFunc, n.Array, Array, n.ArrayPos)

	case *ast.DeleteStmt:
		v.r.recordVar(v.curFunc, n.Array, Array, n.ArrayPos)
		ast.WalkExprList(v, n.Index)

	case *ast.CallExpr:
		switch n.Func {
		case lexer.F_SPLIT:
			ast.Walk(v, n.Args[0])
			varExpr := n.Args[1].(*ast.VarExpr) // split()'s 2nd arg is always an array
			v.r.recordVar(v.curFunc, varExpr.Name, Array, varExpr.Pos)
			ast.WalkExprList(v, n.Args[2:])

		case lexer.F_LENGTH:
			if len(n.Args) > 0 {
				if varExpr, ok := n.Args[0].(*ast.VarExpr); ok {
					// In a call to length(x), x may be a scalar or an array,
					// so set it to unknown for now.
					v.r.recordVar(v.curFunc, varExpr.Name, unknown, varExpr.Pos)
					return nil
				}
			}
			ast.WalkExprList(v, n.Args)

		default:
			ast.WalkExprList(v, n.Args)
		}

	case *ast.UserCallExpr:
		_, _, varFunc, exists := v.r.lookupVar(v.curFunc, n.Name)
		if varFunc != "" && exists {
			panic(ast.PosErrorf(n.Pos, "can't call local variable %q as function", n.Name))
		}

		funcInfo, exists := v.r.funcInfo[n.Name]
		if !exists {
			panic(ast.PosErrorf(n.Pos, "undefined function %q", n.Name))
		}

		numParams := len(funcInfo.Params)
		if funcInfo.Native {
			typ := reflect.TypeOf(v.nativeFuncs[n.Name])
			numParams = typ.NumIn()
			if typ.IsVariadic() {
				numParams = 1000000000 // bigger than any reasonable len(n.Args) value!
			}
		}
		if len(n.Args) > numParams {
			panic(ast.PosErrorf(n.Pos, "%q called with more arguments than declared", n.Name))
		}

		for i, arg := range n.Args {
			varExpr, ok := arg.(*ast.VarExpr)
			if !ok {
				// Argument is not a variable, process normally.
				if !funcInfo.Native {
					paramInfo := v.r.varInfo[n.Name][funcInfo.Params[i]] // type info of corresponding parameter
					if paramInfo.Type == Array {
						panic(ast.PosErrorf(n.Pos, "can't pass scalar %s as array param", arg))
					}
				}
				ast.Walk(v, arg)
				continue
			}

			if funcInfo.Native {
				// Arguments to native function can only be scalar.
				v.r.recordVar(v.curFunc, varExpr.Name, Scalar, varExpr.Pos)
				continue
			}

			// Variable passed to AWK-defined function may be scalar or array,
			// determine from how it was used elsewhere.
			paramName := funcInfo.Params[i]             // name of corresponding parameter
			paramInfo := v.r.varInfo[n.Name][paramName] // type info of parameter
			_, varInfo, _, _ := v.r.lookupVar(v.curFunc, varExpr.Name)
			switch {
			case varInfo.Type == unknown && paramInfo.Type != unknown:
				// Variable's type is not known but param type is, set variable type.
				v.r.recordVar(v.curFunc, varExpr.Name, paramInfo.Type, varExpr.Pos)
			case varInfo.Type != unknown && paramInfo.Type == unknown:
				// Variable's type is known but param type is not, set param type.
				funcPos := v.r.funcs[n.Name].Pos // best position we have at this point
				v.r.recordVar(n.Name, paramName, varInfo.Type, funcPos)
			case varInfo.Type != paramInfo.Type && varInfo.Type != unknown && paramInfo.Type != unknown:
				// Both types are known but don't match -- type error!
				panic(ast.PosErrorf(varExpr.Pos, "can't pass %s %q as %s param",
					varInfo.Type, varExpr.Name, paramInfo.Type))
			default:
				// Ensure variable references are recorded, even if the type
				// is not yet known.
				v.r.recordVar(v.curFunc, varExpr.Name, unknown, varExpr.Pos)
			}
		}

	default:
		return v
	}
	return nil
}
