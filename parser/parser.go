// Package parser is an AWK parser and abstract syntax tree.
//
// Use the ParseProgram function to parse an AWK program, and then give the
// result to interp.Exec, interp.ExecProgram, or interp.New to execute it.
package parser

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/compiler"
	"github.com/benhoyt/goawk/internal/resolver"
	"github.com/benhoyt/goawk/lexer"
)

// ParseError (actually *ParseError) is the type of error returned by
// ParseProgram.
type ParseError struct {
	// Source line/column position where the error occurred.
	Position lexer.Position
	// Error message.
	Message string
}

// Error returns a formatted version of the error, including the line
// and column numbers.
func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at %d:%d: %s", e.Position.Line, e.Position.Column, e.Message)
}

// ParserConfig lets you specify configuration for the parsing
// process (for example printing type information for debugging).
type ParserConfig struct {
	// Enable printing of type information
	DebugTypes bool

	// io.Writer to print type information on (for example, os.Stderr)
	DebugWriter io.Writer

	// Map of named Go functions to allow calling from AWK. See docs
	// on interp.Config.Funcs for details.
	Funcs map[string]any
}

func (c *ParserConfig) toResolverConfig() *resolver.Config {
	if c == nil {
		return nil
	}
	return &resolver.Config{
		DebugTypes:  c.DebugTypes,
		DebugWriter: c.DebugWriter,
		Funcs:       c.Funcs,
	}
}

// ParseProgram parses an entire AWK program, returning the *Program
// abstract syntax tree or a *ParseError on error. "config" describes
// the parser configuration (and is allowed to be nil).
func ParseProgram(src []byte, config *ParserConfig) (prog *Program, err error) {
	defer func() {
		// The parser and resolver use panic with an *ast.PositionError to signal parsing
		// errors internally, and they're caught here. This significantly simplifies
		// the recursive descent calls as we don't have to check errors everywhere.
		if r := recover(); r != nil {
			// Convert to PositionError or re-panic
			posError := *r.(*ast.PositionError)
			err = &ParseError{
				Position: posError.Position,
				Message:  posError.Message,
			}
		}
	}()
	lex := lexer.NewLexer(src)
	p := parser{lexer: lex}
	p.multiExprs = make(map[*ast.MultiExpr]lexer.Position, 3)

	p.next() // initialize p.tok

	// Parse into abstract syntax tree
	astProg := p.program()

	// Resolve variable scopes and types
	prog = &Program{}
	prog.ResolvedProgram = *resolver.Resolve(astProg, config.toResolverConfig())

	// Compile to virtual machine code
	prog.Compiled, err = compiler.Compile(&prog.ResolvedProgram)
	return prog, err
}

// Program is the parsed and compiled representation of an entire AWK program.
type Program struct {
	// These fields aren't intended to be used or modified directly,
	// but are exported for the interpreter (Program itself needs to
	// be exported in package "parser", otherwise these could live in
	// "internal/ast".)
	resolver.ResolvedProgram
	Compiled *compiler.Program
}

// String returns an indented, pretty-printed version of the parsed
// program.
func (p *Program) String() string {
	return p.ResolvedProgram.Program.String()
}

// Disassemble writes a human-readable form of the program's virtual machine
// instructions to writer.
func (p *Program) Disassemble(writer io.Writer) error {
	return p.Compiled.Disassemble(writer)
}

// Parser state
type parser struct {
	// Lexer instance and current token values
	lexer   *lexer.Lexer
	pos     lexer.Position // position of last token (tok)
	tok     lexer.Token    // last lexed token
	prevTok lexer.Token    // previously lexed token
	val     string         // string value of last token (or "")

	// Parsing state
	inAction           bool     // true if parsing an action (false in BEGIN or END)
	funcName           string   // function name if parsing a func, else ""
	loopDepth          int      // current loop depth (0 if not in any loops)
	pendingGetlineLeft ast.Expr // saved expression to the left of |

	// Variable tracking and resolving
	multiExprs map[*ast.MultiExpr]lexer.Position // tracks comma-separated expressions
}

// Parse an entire AWK program.
func (p *parser) program() *ast.Program {
	prog := &ast.Program{}

	// Terminator "(SEMICOLON|NEWLINE) NEWLINE*" is required after each item
	// with two exceptions where it is optional:
	//
	// 1. after the last item, or
	// 2. when the previous item ended with a closing brace.
	//
	// NOTE: The second exception does not seem to be correct according to
	// the POSIX grammar definition, but it is the common behaviour for the
	// major AWK implementations.
	needsTerminator := false

	for p.tok != lexer.EOF {
		if needsTerminator {
			if !p.matches(lexer.NEWLINE, lexer.SEMICOLON) {
				panic(p.errorf("expected ; or newline between items"))
			}
			p.next()
			needsTerminator = false
		}
		p.optionalNewlines()
		switch p.tok {
		case lexer.EOF:
			// End of file
		case lexer.BEGIN:
			p.next()
			prog.Begin = append(prog.Begin, p.stmtsBrace())
		case lexer.END:
			p.next()
			prog.End = append(prog.End, p.stmtsBrace())
		case lexer.FUNCTION:
			function := p.function()
			prog.Functions = append(prog.Functions, function)
		default:
			p.inAction = true
			// Allow empty pattern, normal pattern, or range pattern
			pattern := []ast.Expr{}
			if !p.matches(lexer.LBRACE, lexer.EOF) {
				pattern = append(pattern, p.expr())
			}
			if !p.matches(lexer.LBRACE, lexer.EOF, lexer.NEWLINE, lexer.SEMICOLON) {
				p.commaNewlines()
				pattern = append(pattern, p.expr())
			}
			// Or an empty action (equivalent to { print $0 })
			action := &ast.Action{Pattern: pattern}
			if p.tok == lexer.LBRACE {
				action.Stmts = p.stmtsBrace()
			} else {
				needsTerminator = true
			}
			prog.Actions = append(prog.Actions, action)
			p.inAction = false
		}
	}

	p.checkMultiExprs()

	return prog
}

// Parse a list of statements.
func (p *parser) stmts() ast.Stmts {
	switch p.tok {
	case lexer.SEMICOLON:
		// This is so things like this parse correctly:
		// BEGIN { for (i=0; i<10; i++); print "x" }
		p.next()
		return nil
	case lexer.LBRACE:
		return p.stmtsBrace()
	default:
		return []ast.Stmt{p.stmt()}
	}
}

// Parse a list of statements surrounded in {...} braces.
func (p *parser) stmtsBrace() ast.Stmts {
	p.expect(lexer.LBRACE)
	p.optionalNewlines()
	ss := []ast.Stmt{}
	for p.tok != lexer.RBRACE && p.tok != lexer.EOF {
		if p.matches(lexer.SEMICOLON, lexer.NEWLINE) {
			p.next()
			continue
		}
		ss = append(ss, p.stmt())
	}
	p.expect(lexer.RBRACE)
	if p.tok == lexer.SEMICOLON {
		p.next()
	}
	return ss
}

// Parse a "simple" statement (eg: allowed in a for loop init clause).
func (p *parser) simpleStmt() ast.Stmt {
	startPos := p.pos
	switch p.tok {
	case lexer.PRINT, lexer.PRINTF:
		op := p.tok
		p.next()
		args := p.exprList(p.printExpr)
		if len(args) == 1 {
			// This allows parens around all the print args
			if m, ok := args[0].(*ast.MultiExpr); ok {
				args = m.Exprs
				p.useMultiExpr(m)
			}
		}
		redirect := lexer.ILLEGAL
		var dest ast.Expr
		if p.matches(lexer.GREATER, lexer.APPEND, lexer.PIPE) {
			redirect = p.tok
			p.next()
			dest = p.expr()
		}
		if op == lexer.PRINT {
			return &ast.PrintStmt{Args: args, Redirect: redirect, Dest: dest, Start: startPos, End: p.pos}
		} else {
			if len(args) == 0 {
				panic(p.errorf("expected printf args, got none"))
			}
			return &ast.PrintfStmt{Args: args, Redirect: redirect, Dest: dest, Start: startPos, End: p.pos}
		}
	case lexer.DELETE:
		p.next()
		name, namePos := p.expectName()
		var index []ast.Expr
		if p.tok == lexer.LBRACKET {
			p.next()
			index = p.exprList(p.expr)
			if len(index) == 0 {
				panic(p.errorf("expected expression instead of ]"))
			}
			p.expect(lexer.RBRACKET)
		}
		return &ast.DeleteStmt{Array: name, ArrayPos: namePos, Index: index, Start: startPos, End: p.pos}
	case lexer.IF, lexer.FOR, lexer.WHILE, lexer.DO, lexer.BREAK, lexer.CONTINUE, lexer.NEXT, lexer.NEXTFILE, lexer.EXIT, lexer.RETURN:
		panic(p.errorf("expected print/printf, delete, or expression"))
	default:
		return &ast.ExprStmt{Expr: p.expr(), Start: startPos, End: p.pos}
	}
}

// Parse any top-level statement.
func (p *parser) stmt() ast.Stmt {
	var s ast.Stmt
	startPos := p.pos
	switch p.tok {
	case lexer.IF:
		p.next()
		p.expect(lexer.LPAREN)
		cond := p.expr()
		p.expect(lexer.RPAREN)
		p.optionalNewlines()
		bodyStart := p.pos
		body := p.stmts()
		p.optionalNewlines()
		var elseBody ast.Stmts
		if p.tok == lexer.ELSE {
			p.next()
			p.optionalNewlines()
			elseBody = p.stmts()
		}
		s = &ast.IfStmt{Cond: cond, BodyStart: bodyStart, Body: body, Else: elseBody, Start: startPos, End: p.pos}
	case lexer.FOR:
		// Parse for statement, either "for in" or C-like for loop.
		//
		//     FOR LPAREN NAME IN NAME RPAREN NEWLINE* stmts |
		//     FOR LPAREN [simpleStmt] SEMICOLON NEWLINE*
		//                [expr] SEMICOLON NEWLINE*
		//                [simpleStmt] RPAREN NEWLINE* stmts
		//
		p.next()
		p.expect(lexer.LPAREN)
		var pre ast.Stmt
		if p.tok != lexer.SEMICOLON {
			pre = p.simpleStmt()
		}
		if pre != nil && p.tok == lexer.RPAREN {
			// Match: for (var in array) body
			p.next()
			p.optionalNewlines()
			exprStmt, ok := pre.(*ast.ExprStmt)
			if !ok {
				panic(p.errorf("expected 'for (var in array) ...'"))
			}
			inExpr, ok := exprStmt.Expr.(*ast.InExpr)
			if !ok {
				panic(p.errorf("expected 'for (var in array) ...'"))
			}
			if len(inExpr.Index) != 1 {
				panic(p.errorf("expected 'for (var in array) ...'"))
			}
			varExpr, ok := inExpr.Index[0].(*ast.VarExpr)
			if !ok {
				panic(p.errorf("expected 'for (var in array) ...'"))
			}
			bodyStart := p.pos
			body := p.loopStmts()
			s = &ast.ForInStmt{
				Var:       varExpr.Name,
				VarPos:    varExpr.Pos,
				Array:     inExpr.Array,
				ArrayPos:  inExpr.ArrayPos,
				BodyStart: bodyStart,
				Body:      body,
				Start:     startPos,
				End:       p.pos,
			}
		} else {
			// Match: for ([pre]; [cond]; [post]) body
			p.expect(lexer.SEMICOLON)
			p.optionalNewlines()
			var cond ast.Expr
			if p.tok != lexer.SEMICOLON {
				cond = p.expr()
			}
			p.expect(lexer.SEMICOLON)
			p.optionalNewlines()
			var post ast.Stmt
			if p.tok != lexer.RPAREN {
				post = p.simpleStmt()
			}
			p.expect(lexer.RPAREN)
			p.optionalNewlines()
			bodyStart := p.pos
			body := p.loopStmts()
			s = &ast.ForStmt{Pre: pre, Cond: cond, Post: post, BodyStart: bodyStart, Body: body, Start: startPos, End: p.pos}
		}
	case lexer.WHILE:
		p.next()
		p.expect(lexer.LPAREN)
		cond := p.expr()
		p.expect(lexer.RPAREN)
		p.optionalNewlines()
		bodyStart := p.pos
		body := p.loopStmts()
		s = &ast.WhileStmt{Cond: cond, BodyStart: bodyStart, Body: body, Start: startPos, End: p.pos}
	case lexer.DO:
		p.next()
		p.optionalNewlines()
		body := p.loopStmts()
		p.optionalNewlines()
		p.expect(lexer.WHILE)
		p.expect(lexer.LPAREN)
		cond := p.expr()
		p.expect(lexer.RPAREN)
		s = &ast.DoWhileStmt{Body: body, Cond: cond, Start: startPos, End: p.pos}
	case lexer.BREAK:
		if p.loopDepth == 0 {
			panic(p.errorf("break must be inside a loop body"))
		}
		p.next()
		s = &ast.BreakStmt{Start: startPos, End: p.pos}
	case lexer.CONTINUE:
		if p.loopDepth == 0 {
			panic(p.errorf("continue must be inside a loop body"))
		}
		p.next()
		s = &ast.ContinueStmt{Start: startPos, End: p.pos}
	case lexer.NEXT:
		if !p.inAction && p.funcName == "" {
			panic(p.errorf("next can't be inside BEGIN or END"))
		}
		p.next()
		s = &ast.NextStmt{Start: startPos, End: p.pos}
	case lexer.NEXTFILE:
		if !p.inAction && p.funcName == "" {
			panic(p.errorf("nextfile can't be inside BEGIN or END"))
		}
		p.next()
		s = &ast.NextfileStmt{Start: startPos, End: p.pos}
	case lexer.EXIT:
		p.next()
		var status ast.Expr
		if !p.matches(lexer.NEWLINE, lexer.SEMICOLON, lexer.RBRACE) {
			status = p.expr()
		}
		s = &ast.ExitStmt{Status: status, Start: startPos, End: p.pos}
	case lexer.RETURN:
		if p.funcName == "" {
			panic(p.errorf("return must be inside a function"))
		}
		p.next()
		var value ast.Expr
		if !p.matches(lexer.NEWLINE, lexer.SEMICOLON, lexer.RBRACE) {
			value = p.expr()
		}
		s = &ast.ReturnStmt{Value: value, Start: startPos, End: p.pos}
	case lexer.LBRACE:
		body := p.stmtsBrace()
		s = &ast.BlockStmt{Body: body, Start: startPos, End: p.pos}
	default:
		s = p.simpleStmt()
	}

	// Ensure statements are separated by ; or newline
	if !p.matches(lexer.NEWLINE, lexer.SEMICOLON, lexer.RBRACE) && p.prevTok != lexer.NEWLINE && p.prevTok != lexer.SEMICOLON && p.prevTok != lexer.RBRACE {
		panic(p.errorf("expected ; or newline between statements"))
	}
	for p.matches(lexer.NEWLINE, lexer.SEMICOLON) {
		p.next()
	}
	return s
}

// Same as stmts(), but tracks that we're in a loop (as break and
// continue can only occur inside a loop).
func (p *parser) loopStmts() ast.Stmts {
	p.loopDepth++
	ss := p.stmts()
	p.loopDepth--
	return ss
}

// Parse a function definition and body. As it goes, this resolves
// the local variable indexes and tracks which parameters are array
// parameters.
func (p *parser) function() *ast.Function {
	if p.funcName != "" {
		// Should never actually get here (FUNCTION token is only
		// handled at the top level), but just in case.
		panic(p.errorf("can't nest functions"))
	}
	p.next()
	name, funcNamePos := p.expectName()
	p.expect(lexer.LPAREN)
	first := true
	params := make([]string, 0, 7) // pre-allocate some to reduce allocations
	locals := make(map[string]bool, 7)
	for p.tok != lexer.RPAREN {
		if !first {
			p.commaNewlines()
		}
		first = false
		param := p.val
		if param == name {
			panic(p.errorf("can't use function name as parameter name"))
		}
		if locals[param] {
			panic(p.errorf("duplicate parameter name %q", param))
		}
		p.expect(lexer.NAME)
		params = append(params, param)
		locals[param] = true
	}
	p.expect(lexer.RPAREN)
	p.optionalNewlines()

	// Parse the body
	p.funcName = name

	body := p.stmtsBrace()

	p.funcName = ""

	return &ast.Function{Name: name, Params: params, Body: body, Pos: funcNamePos}
}

// Parse expressions separated by commas: args to print[f] or user
// function call, or multi-dimensional index.
func (p *parser) exprList(parse func() ast.Expr) []ast.Expr {
	exprs := []ast.Expr{}
	first := true
	for !p.matches(lexer.NEWLINE, lexer.SEMICOLON, lexer.RBRACE, lexer.RBRACKET, lexer.RPAREN, lexer.GREATER, lexer.PIPE, lexer.APPEND) {
		if !first {
			p.commaNewlines()
		}
		first = false
		exprs = append(exprs, parse())
	}
	return exprs
}

// Here's where things get slightly interesting: only certain
// expression types are allowed in print/printf statements,
// presumably so `print a, b > "file"` is a file redirect instead of
// a greater-than comparison. So we kind of have two ways to recurse
// down here: expr(), which parses all expressions, and printExpr(),
// which skips PIPE GETLINE and GREATER expressions.

// Parse a single expression.
func (p *parser) expr() ast.Expr      { return p._assign(p.getline) }
func (p *parser) printExpr() ast.Expr { return p._assign(p.printCond) }

// Parse an "expr | getline [lvalue]" expression:
//
//	assign [PIPE GETLINE [lvalue]]
func (p *parser) getline() ast.Expr {
	// NOTE: getline is special, see https://github.com/benhoyt/goawk/pull/216
	p.pendingGetlineLeft = nil
	left := p.cond()
	if p.tok == lexer.PIPE {
		p.pendingGetlineLeft = left
		return p.cond()
	}
	return left
}

// Parse an = assignment expression:
//
//	lvalue [assign_op assign]
//
// An lvalue is a variable name, an array[expr] index expression, or
// an $expr field expression.
func (p *parser) _assign(higher func() ast.Expr) ast.Expr {
	leftPos := p.pos
	expr := higher()
	if p.matches(lexer.ASSIGN, lexer.ADD_ASSIGN, lexer.DIV_ASSIGN, lexer.MOD_ASSIGN, lexer.MUL_ASSIGN, lexer.POW_ASSIGN, lexer.SUB_ASSIGN) {
		_, isNamedField := expr.(*ast.NamedFieldExpr)
		if isNamedField {
			panic(p.errorf("assigning @ expression not supported"))
		}
		op := p.tok
		p.next()
		right := p._assign(higher)
		if !ast.IsLValue(expr) {
			// Partial backtracking to allow expressions like "1 && x=1",
			// which isn't really valid, as assignments are lower-precedence
			// than binary operators, but onetrueawk, Gawk, and mawk all
			// support this for logical, match and comparison operators. See
			// issue #166.
			binary, isBinary := expr.(*ast.BinaryExpr)
			if isBinary && ast.IsLValue(binary.Right) {
				switch binary.Op {
				case lexer.AND, lexer.OR, lexer.MATCH, lexer.NOT_MATCH, lexer.EQUALS, lexer.NOT_EQUALS, lexer.LESS, lexer.LTE, lexer.GTE, lexer.GREATER:
					assign := makeAssign(binary.Right, op, right)
					return &ast.BinaryExpr{Left: binary.Left, Op: binary.Op, Right: assign}
				}
			}
			panic(ast.PosErrorf(leftPos, "expected lvalue before %s", op))
		}
		return makeAssign(expr, op, right)
	}
	return expr
}

func makeAssign(left ast.Expr, op lexer.Token, right ast.Expr) ast.Expr {
	switch op {
	case lexer.ASSIGN:
		return &ast.AssignExpr{Left: left, Right: right}
	case lexer.ADD_ASSIGN:
		op = lexer.ADD
	case lexer.DIV_ASSIGN:
		op = lexer.DIV
	case lexer.MOD_ASSIGN:
		op = lexer.MOD
	case lexer.MUL_ASSIGN:
		op = lexer.MUL
	case lexer.POW_ASSIGN:
		op = lexer.POW
	case lexer.SUB_ASSIGN:
		op = lexer.SUB
	}
	return &ast.AugAssignExpr{Left: left, Op: op, Right: right}
}

// Parse a ?: conditional expression:
//
//	or [QUESTION NEWLINE* cond COLON NEWLINE* cond]
func (p *parser) cond() ast.Expr      { return p._cond(p.or) }
func (p *parser) printCond() ast.Expr { return p._cond(p.printOr) }

func (p *parser) _cond(higher func() ast.Expr) ast.Expr {
	expr := higher()
	if p.tok == lexer.QUESTION {
		p.next()
		p.optionalNewlines()
		t := p.expr()
		p.expect(lexer.COLON)
		p.optionalNewlines()
		f := p.expr()
		return &ast.CondExpr{Cond: expr, True: t, False: f}
	}
	return expr
}

// Parse an || or expression:
//
//	and [OR NEWLINE* and] [OR NEWLINE* and] ...
func (p *parser) or() ast.Expr      { return p.binaryLeft(p.and, true, lexer.OR) }
func (p *parser) printOr() ast.Expr { return p.binaryLeft(p.printAnd, true, lexer.OR) }

// Parse an && and expression:
//
//	in [AND NEWLINE* in] [AND NEWLINE* in] ...
func (p *parser) and() ast.Expr      { return p.binaryLeft(p.in, true, lexer.AND) }
func (p *parser) printAnd() ast.Expr { return p.binaryLeft(p.printIn, true, lexer.AND) }

// Parse an "in" expression:
//
//	match [IN NAME] [IN NAME] ...
func (p *parser) in() ast.Expr      { return p._in(p.match) }
func (p *parser) printIn() ast.Expr { return p._in(p.printMatch) }

func (p *parser) _in(higher func() ast.Expr) ast.Expr {
	expr := higher()
	for p.tok == lexer.IN {
		p.next()
		name, namePos := p.expectName()
		expr = &ast.InExpr{Index: []ast.Expr{expr}, Array: name, ArrayPos: namePos}
	}
	return expr
}

// Parse a ~ match expression:
//
//	compare [MATCH|NOT_MATCH compare]
func (p *parser) match() ast.Expr      { return p._match(p.compare) }
func (p *parser) printMatch() ast.Expr { return p._match(p.printCompare) }

func (p *parser) _match(higher func() ast.Expr) ast.Expr {
	expr := higher()
	if p.matches(lexer.MATCH, lexer.NOT_MATCH) {
		op := p.tok
		p.next()
		right := p.regexStr(higher) // Not match() as these aren't associative
		return &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

// Parse a comparison expression:
//
//	concat [EQUALS|NOT_EQUALS|LESS|LTE|GREATER|GTE concat]
func (p *parser) compare() ast.Expr {
	return p._compare(lexer.EQUALS, lexer.NOT_EQUALS, lexer.LESS, lexer.LTE, lexer.GTE, lexer.GREATER)
}
func (p *parser) printCompare() ast.Expr {
	return p._compare(lexer.EQUALS, lexer.NOT_EQUALS, lexer.LESS, lexer.LTE, lexer.GTE)
}

func (p *parser) _compare(ops ...lexer.Token) ast.Expr {
	expr := p.concat()
	if p.matches(ops...) {
		op := p.tok
		p.next()
		right := p.concat() // Not compare() as these aren't associative
		return &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) concat() ast.Expr {
	expr := p.add()
	for p.matches(lexer.DOLLAR, lexer.AT, lexer.NOT, lexer.NAME, lexer.NUMBER, lexer.STRING, lexer.LPAREN, lexer.INCR, lexer.DECR) ||
		p.tok >= lexer.FIRST_FUNC && p.tok <= lexer.LAST_FUNC {
		right := p.add()
		expr = &ast.BinaryExpr{Left: expr, Op: lexer.CONCAT, Right: right}
	}
	return expr
}

func (p *parser) add() ast.Expr {
	return p.binaryLeft(p.mul, false, lexer.ADD, lexer.SUB)
}

func (p *parser) mul() ast.Expr {
	return p.binaryLeft(p.pow, false, lexer.MUL, lexer.DIV, lexer.MOD)
}

func (p *parser) pow() ast.Expr {
	// Note that pow (expr ^ expr) is right-associative
	expr := p.postIncr()
	if p.tok == lexer.POW {
		p.next()
		right := p.pow()
		return &ast.BinaryExpr{Left: expr, Op: lexer.POW, Right: right}
	}
	return expr
}

func (p *parser) postIncr() ast.Expr {
	expr := p.primary()
	if (p.tok == lexer.INCR || p.tok == lexer.DECR) && ast.IsLValue(expr) {
		op := p.tok
		p.next()
		return &ast.IncrExpr{Expr: expr, Op: op}
	}
	return expr
}

func (p *parser) primary() ast.Expr {
	if p.pendingGetlineLeft != nil {
		p.expect(lexer.PIPE)
		p.expect(lexer.GETLINE)
		left := p.pendingGetlineLeft
		p.pendingGetlineLeft = nil
		target := p.optionalLValue()
		return &ast.GetlineExpr{Command: left, Target: target}
	}
	switch p.tok {
	case lexer.NUMBER:
		// AWK allows forms like "1.5e", but ParseFloat doesn't
		s := strings.TrimRight(p.val, "eE")
		n, _ := strconv.ParseFloat(s, 64)
		p.next()
		return &ast.NumExpr{Value: n}
	case lexer.STRING:
		s := p.val
		p.next()
		return &ast.StrExpr{Value: s}
	case lexer.DIV, lexer.DIV_ASSIGN:
		// If we get to DIV or DIV_ASSIGN as a primary expression,
		// it's actually a regex.
		regex := p.nextRegex()
		return &ast.RegExpr{Regex: regex}
	case lexer.DOLLAR:
		p.next()
		var expr ast.Expr = &ast.FieldExpr{Index: p.primary()}
		// Post-increment operators have lower precedence than primary
		// expressions by default, except for field expressions with
		// post-increments (e.g., $$1++ = $($1++), NOT $($1)++).
		if p.tok == lexer.INCR || p.tok == lexer.DECR {
			op := p.tok
			p.next()
			expr = &ast.IncrExpr{Expr: expr, Op: op}
		}
		return expr
	case lexer.AT:
		p.next()
		return &ast.NamedFieldExpr{Field: p.primary()}
	case lexer.NOT, lexer.ADD, lexer.SUB:
		op := p.tok
		p.next()
		return &ast.UnaryExpr{Op: op, Value: p.pow()}
	case lexer.INCR, lexer.DECR:
		op := p.tok
		p.next()
		exprPos := p.pos
		expr := p.optionalLValue()
		if expr == nil {
			panic(ast.PosErrorf(exprPos, "expected lvalue after %s", op))
		}
		return &ast.IncrExpr{Expr: expr, Op: op, Pre: true}
	case lexer.NAME:
		name, namePos := p.expectName()
		if p.tok == lexer.LBRACKET {
			// a[x] or a[x, y] array index expression
			p.next()
			index := p.exprList(p.expr)
			if len(index) == 0 {
				panic(p.errorf("expected expression instead of ]"))
			}
			p.expect(lexer.RBRACKET)
			return &ast.IndexExpr{Array: name, ArrayPos: namePos, Index: index}
		} else if p.tok == lexer.LPAREN && !p.lexer.HadSpace() {
			// Grammar requires no space between function name and
			// left paren for user function calls, hence the funky
			// lexer.HadSpace() method.
			return p.userCall(name, namePos)
		}
		return &ast.VarExpr{Name: name, Pos: namePos}
	case lexer.LPAREN:
		parenPos := p.pos
		p.next()
		exprs := p.exprList(p.expr)
		switch len(exprs) {
		case 0:
			panic(p.errorf("expected expression, not %s", p.tok))
		case 1:
			p.expect(lexer.RPAREN)
			return &ast.GroupingExpr{Expr: exprs[0]}
		default:
			// Multi-dimensional array "in" requires parens around index
			p.expect(lexer.RPAREN)
			if p.tok == lexer.IN {
				p.next()
				name, namePos := p.expectName()
				return &ast.InExpr{Index: exprs, Array: name, ArrayPos: namePos}
			}
			// MultiExpr is used as a pseudo-expression for print[f] parsing.
			return p.multiExpr(exprs, parenPos)
		}
	case lexer.GETLINE:
		p.next()
		target := p.optionalLValue()
		var file ast.Expr
		if p.tok == lexer.LESS {
			p.next()
			file = p.primary()
		}
		return &ast.GetlineExpr{Target: target, File: file}
	// Below is the parsing of all the builtin function calls. We
	// could unify these but several of them have special handling
	// (array/lvalue/regex params, optional arguments, and so on).
	// Doing it this way means we can check more at parse time.
	case lexer.F_SUB, lexer.F_GSUB:
		op := p.tok
		p.next()
		p.expect(lexer.LPAREN)
		regex := p.regexStr(p.expr)
		p.commaNewlines()
		repl := p.expr()
		args := []ast.Expr{regex, repl}
		if p.tok == lexer.COMMA {
			p.commaNewlines()
			inPos := p.pos
			in := p.expr()
			if !ast.IsLValue(in) {
				panic(ast.PosErrorf(inPos, "3rd arg to sub/gsub must be lvalue"))
			}
			args = append(args, in)
		}
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: op, Args: args}
	case lexer.F_SPLIT:
		p.next()
		p.expect(lexer.LPAREN)
		str := p.expr()
		p.commaNewlines()
		name, namePos := p.expectName()
		args := []ast.Expr{str, &ast.VarExpr{Name: name, Pos: namePos}}
		if p.tok == lexer.COMMA {
			p.commaNewlines()
			args = append(args, p.regexStr(p.expr))
		}
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: lexer.F_SPLIT, Args: args}
	case lexer.F_MATCH:
		p.next()
		p.expect(lexer.LPAREN)
		str := p.expr()
		p.commaNewlines()
		regex := p.regexStr(p.expr)
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: lexer.F_MATCH, Args: []ast.Expr{str, regex}}
	case lexer.F_RAND:
		p.next()
		p.expect(lexer.LPAREN)
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: lexer.F_RAND}
	case lexer.F_SRAND:
		p.next()
		p.expect(lexer.LPAREN)
		var args []ast.Expr
		if p.tok != lexer.RPAREN {
			args = append(args, p.expr())
		}
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: lexer.F_SRAND, Args: args}
	case lexer.F_LENGTH:
		p.next()
		var args []ast.Expr
		// AWK quirk: "length" is allowed to be called without parens
		if p.tok == lexer.LPAREN {
			p.next()
			if p.tok != lexer.RPAREN {
				args = append(args, p.expr())
			}
			p.expect(lexer.RPAREN)
		}
		return &ast.CallExpr{Func: lexer.F_LENGTH, Args: args}
	case lexer.F_SUBSTR:
		p.next()
		p.expect(lexer.LPAREN)
		str := p.expr()
		p.commaNewlines()
		start := p.expr()
		args := []ast.Expr{str, start}
		if p.tok == lexer.COMMA {
			p.commaNewlines()
			args = append(args, p.expr())
		}
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: lexer.F_SUBSTR, Args: args}
	case lexer.F_SPRINTF:
		p.next()
		p.expect(lexer.LPAREN)
		args := []ast.Expr{p.expr()}
		for p.tok == lexer.COMMA {
			p.commaNewlines()
			args = append(args, p.expr())
		}
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: lexer.F_SPRINTF, Args: args}
	case lexer.F_FFLUSH:
		p.next()
		p.expect(lexer.LPAREN)
		var args []ast.Expr
		if p.tok != lexer.RPAREN {
			args = append(args, p.expr())
		}
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: lexer.F_FFLUSH, Args: args}
	case lexer.F_COS, lexer.F_SIN, lexer.F_EXP, lexer.F_LOG, lexer.F_SQRT, lexer.F_INT, lexer.F_TOLOWER, lexer.F_TOUPPER, lexer.F_SYSTEM, lexer.F_CLOSE:
		// Simple 1-argument functions
		op := p.tok
		p.next()
		p.expect(lexer.LPAREN)
		arg := p.expr()
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: op, Args: []ast.Expr{arg}}
	case lexer.F_ATAN2, lexer.F_INDEX:
		// Simple 2-argument functions
		op := p.tok
		p.next()
		p.expect(lexer.LPAREN)
		arg1 := p.expr()
		p.commaNewlines()
		arg2 := p.expr()
		p.expect(lexer.RPAREN)
		return &ast.CallExpr{Func: op, Args: []ast.Expr{arg1, arg2}}
	default:
		panic(p.errorf("expected expression instead of %s", p.tok))
	}
}

// Parse an optional lvalue
func (p *parser) optionalLValue() ast.Expr {
	switch p.tok {
	case lexer.NAME:
		if p.lexer.PeekByte() == '(' {
			// User function call, e.g., foo() not lvalue.
			return nil
		}
		name, namePos := p.expectName()
		if p.tok == lexer.LBRACKET {
			// a[x] or a[x, y] array index expression
			p.next()
			index := p.exprList(p.expr)
			if len(index) == 0 {
				panic(p.errorf("expected expression instead of ]"))
			}
			p.expect(lexer.RBRACKET)
			return &ast.IndexExpr{Array: name, ArrayPos: namePos, Index: index}
		}
		return &ast.VarExpr{Name: name, Pos: namePos}
	case lexer.DOLLAR:
		p.next()
		return &ast.FieldExpr{Index: p.primary()}
	default:
		return nil
	}
}

// Parse /.../ regex or generic expression:
//
//	REGEX | expr
func (p *parser) regexStr(parse func() ast.Expr) ast.Expr {
	if p.matches(lexer.DIV, lexer.DIV_ASSIGN) {
		regex := p.nextRegex()
		return &ast.StrExpr{Value: regex, Regex: true}
	}
	return parse()
}

// Parse left-associative binary operator. Allow newlines after
// operator if allowNewline is true.
//
//	parse [op parse] [op parse] ...
func (p *parser) binaryLeft(higher func() ast.Expr, allowNewline bool, ops ...lexer.Token) ast.Expr {
	expr := higher()
	for p.matches(ops...) {
		op := p.tok
		p.next()
		if allowNewline {
			p.optionalNewlines()
		}
		right := higher()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

// Parse comma followed by optional newlines:
//
//	COMMA NEWLINE*
func (p *parser) commaNewlines() {
	p.expect(lexer.COMMA)
	p.optionalNewlines()
}

// Parse zero or more optional newlines:
//
//	[NEWLINE] [NEWLINE] ...
func (p *parser) optionalNewlines() {
	for p.tok == lexer.NEWLINE {
		p.next()
	}
}

// Parse next token into p.tok (and set p.pos and p.val).
func (p *parser) next() {
	p.prevTok = p.tok
	p.pos, p.tok, p.val = p.lexer.Scan()
	if p.tok == lexer.ILLEGAL {
		panic(p.errorf("%s", p.val))
	}
}

// Parse next regex and return it (must only be called after DIV or
// DIV_ASSIGN token).
func (p *parser) nextRegex() string {
	p.pos, p.tok, p.val = p.lexer.ScanRegex()
	if p.tok == lexer.ILLEGAL {
		panic(p.errorf("%s", p.val))
	}
	regex := p.val
	_, err := regexp.Compile(compiler.AddRegexFlags(regex))
	if err != nil {
		panic(p.errorf("%v", err))
	}
	p.next()
	return regex
}

// Ensure current token is tok, and parse next token into p.tok.
func (p *parser) expect(tok lexer.Token) {
	if p.tok != tok {
		panic(p.errorf("expected %s instead of %s", tok, p.tok))
	}
	p.next()
}

// Ensure current token is a name, parse it, and return name and position.
func (p *parser) expectName() (string, lexer.Position) {
	name, pos := p.val, p.pos
	p.expect(lexer.NAME)
	return name, pos
}

// Return true iff current token matches one of the given operators,
// but don't parse next token.
func (p *parser) matches(operators ...lexer.Token) bool {
	for _, operator := range operators {
		if p.tok == operator {
			return true
		}
	}
	return false
}

// Format given string and args with Sprintf and return *ParseError
// with that message and the current position.
func (p *parser) errorf(format string, args ...any) error {
	return ast.PosErrorf(p.pos, format, args...)
}

// Parse call to a user-defined function (and record call site for
// resolving later).
func (p *parser) userCall(name string, pos lexer.Position) *ast.UserCallExpr {
	p.expect(lexer.LPAREN)
	args := []ast.Expr{}
	i := 0
	for !p.matches(lexer.NEWLINE, lexer.RPAREN) {
		if i > 0 {
			p.commaNewlines()
		}
		arg := p.expr()
		args = append(args, arg)
		i++
	}
	p.expect(lexer.RPAREN)
	return &ast.UserCallExpr{Name: name, Args: args, Pos: pos}
}

// Record a "multi expression" (comma-separated pseudo-expression
// used to allow commas around print/printf arguments).
func (p *parser) multiExpr(exprs []ast.Expr, pos lexer.Position) ast.Expr {
	expr := &ast.MultiExpr{Exprs: exprs}
	p.multiExprs[expr] = pos
	return expr
}

// Mark the multi expression as used (by a print/printf statement).
func (p *parser) useMultiExpr(expr *ast.MultiExpr) {
	delete(p.multiExprs, expr)
}

// Check that there are no unused multi expressions (syntax error).
func (p *parser) checkMultiExprs() {
	if len(p.multiExprs) == 0 {
		return
	}
	// Show error on first comma-separated expression
	min := lexer.Position{Line: 1000000000, Column: 1000000000}
	for _, pos := range p.multiExprs {
		if pos.Line < min.Line || pos.Line == min.Line && pos.Column < min.Column {
			min = pos
		}
	}
	panic(ast.PosErrorf(min, "unexpected comma-separated expression"))
}
