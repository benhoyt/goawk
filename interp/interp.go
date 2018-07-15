// GoAWK interpreter.
package interp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/benhoyt/goawk/lexer"
	. "github.com/benhoyt/goawk/parser"
)

var (
	errExit     = errors.New("exit")
	errBreak    = errors.New("break")
	errContinue = errors.New("continue")
	errNext     = errors.New("next")
)

// *Error is returned by Exec and Eval functions on interpreter error,
// for example a negative field index.
type Error struct {
	message string
}

func (e *Error) Error() string {
	return e.message
}

func newError(format string, args ...interface{}) error {
	return &Error{fmt.Sprintf(format, args...)}
}

// Interp holds the state of the interpreter
type Interp struct {
	program *Program
	output  io.Writer
	vars    map[string]value
	arrays  map[string]map[string]value
	random  *rand.Rand

	line        string
	fields      []string
	numFields   int
	lineNum     int
	filename    string
	fileLineNum int

	convertFormat   string
	outputFormat    string
	fieldSep        string
	fieldSepRegex   *regexp.Regexp
	outputFieldSep  string
	outputRecordSep string
	subscriptSep    string
	matchLength     int
	matchStart      int
}

func New(output io.Writer) *Interp {
	p := &Interp{}
	p.output = output
	p.vars = make(map[string]value)
	p.arrays = make(map[string]map[string]value)
	p.random = rand.New(rand.NewSource(0))
	p.convertFormat = "%.6g"
	p.outputFormat = "%.6g"
	p.fieldSep = " "
	p.outputFieldSep = " "
	p.outputRecordSep = "\n"
	p.subscriptSep = "\x1c"
	return p
}

func (p *Interp) ExecStream(prog *Program, input io.Reader) error {
	err := p.execBegin(prog.Begin)
	if err != nil && err != errExit {
		return err
	}
	if prog.Actions == nil && prog.End == nil {
		return nil
	}
	if err != errExit {
		err = p.execActions(prog.Actions, "", input)
		if err != nil && err != errExit {
			return err
		}
	}
	err = p.execEnd(prog.End)
	if err != nil && err != errExit {
		return err
	}
	return nil
}

func (p *Interp) ExecFiles(prog *Program, inputPaths []string) error {
	err := p.execBegin(prog.Begin)
	if err != nil && err != errExit {
		return err
	}
	if prog.Actions == nil && prog.End == nil {
		return nil
	}
	if err != errExit {
		for _, path := range inputPaths {
			f, errOpen := os.Open(path)
			if errOpen != nil {
				return errOpen
			}
			err = p.execActions(prog.Actions, path, f)
			f.Close()
			if err != nil {
				break
			}
		}
		if err != nil && err != errExit {
			return err
		}
	}
	err = p.execEnd(prog.End)
	if err != nil && err != errExit {
		return err
	}
	return nil
}

func (p *Interp) execBegin(begin []Stmts) error {
	for _, statements := range begin {
		err := p.executes(statements)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Interp) execActions(actions []Action, filename string, input io.Reader) error {
	p.setFile(filename)
	scanner := bufio.NewScanner(input)
	inRange := make([]bool, len(actions))
lineLoop:
	for scanner.Scan() {
		p.nextLine(scanner.Text())
		for i, action := range actions {
			matched := false
			switch len(action.Pattern) {
			case 0:
				// No pattern is equivalent to pattern evaluating to true
				matched = true
			case 1:
				// Single boolean pattern
				v, err := p.evalSafe(action.Pattern[0])
				if err != nil {
					return err
				}
				matched = v.boolean()
			case 2:
				// Range pattern (matches between start and stop lines)
				if !inRange[i] {
					v, err := p.evalSafe(action.Pattern[0])
					if err != nil {
						return err
					}
					inRange[i] = v.boolean()
				}
				matched = inRange[i]
				if inRange[i] {
					v, err := p.evalSafe(action.Pattern[1])
					if err != nil {
						return err
					}
					inRange[i] = !v.boolean()
				}
			}
			if !matched {
				continue
			}
			// No action is equivalent to { print $0 }
			if action.Stmts == nil {
				io.WriteString(p.output, p.line+p.outputRecordSep)
				continue
			}
			err := p.executes(action.Stmts)
			if err == errNext {
				continue lineLoop
			}
			if err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading lines from input: %s", err)
	}
	return nil
}

func (p *Interp) execEnd(end []Stmts) error {
	for _, statements := range end {
		err := p.executes(statements)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Interp) executes(stmts Stmts) error {
	for _, s := range stmts {
		err := p.execute(s)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Interp) execute(stmt Stmt) (execErr error) {
	defer func() {
		if r := recover(); r != nil {
			// Convert to interpreter Error or re-panic
			execErr = r.(*Error)
		}
	}()

	switch s := stmt.(type) {
	case *PrintStmt:
		var line string
		if len(s.Args) > 0 {
			strs := make([]string, len(s.Args))
			for i, a := range s.Args {
				value := p.eval(a)
				strs[i] = value.str(p.outputFormat)
			}
			line = strings.Join(strs, p.outputFieldSep)
		} else {
			line = p.line
		}
		io.WriteString(p.output, line+p.outputRecordSep)
	case *PrintfStmt:
		if len(s.Args) == 0 {
			break
		}
		format := p.toString(p.eval(s.Args[0]))
		args := make([]value, len(s.Args)-1)
		for i, a := range s.Args[1:] {
			args[i] = p.eval(a)
		}
		io.WriteString(p.output, p.sprintf(format, args))
	case *IfStmt:
		if p.eval(s.Cond).boolean() {
			return p.executes(s.Body)
		} else {
			return p.executes(s.Else)
		}
	case *ForStmt:
		if s.Pre != nil {
			err := p.execute(s.Pre)
			if err != nil {
				return err
			}
		}
		for s.Cond == nil || p.eval(s.Cond).boolean() {
			err := p.executes(s.Body)
			if err == errBreak {
				break
			}
			if err != nil && err != errContinue {
				return err
			}
			if s.Post != nil {
				err := p.execute(s.Post)
				if err != nil {
					return err
				}
			}
		}
	case *ForInStmt:
		for index := range p.arrays[s.Array] {
			p.setVar(s.Var, str(index))
			err := p.executes(s.Body)
			if err == errBreak {
				break
			}
			if err == errContinue {
				continue
			}
			if err != nil {
				return err
			}
		}
	case *WhileStmt:
		for p.eval(s.Cond).boolean() {
			err := p.executes(s.Body)
			if err == errBreak {
				break
			}
			if err == errContinue {
				continue
			}
			if err != nil {
				return err
			}
		}
	case *DoWhileStmt:
		for {
			err := p.executes(s.Body)
			if err == errBreak {
				break
			}
			if err == errContinue {
				continue
			}
			if err != nil {
				return err
			}
			if !p.eval(s.Cond).boolean() {
				break
			}
		}
	case *BreakStmt:
		return errBreak
	case *ContinueStmt:
		return errContinue
	case *NextStmt:
		return errNext
	case *ExitStmt:
		// TODO: update to handle s.Status (exit status code)
		return errExit
	case *DeleteStmt:
		index := p.eval(s.Index[0]) // TODO: handle multi index
		delete(p.arrays[s.Array], p.toString(index))
	case *ExprStmt:
		p.eval(s.Expr)
	default:
		panic(fmt.Sprintf("unexpected stmt type: %T", stmt))
	}
	return nil
}

func (p *Interp) evalSafe(expr Expr) (v value, err error) {
	defer func() {
		if r := recover(); r != nil {
			// Convert to interpreter Error or re-panic
			err = r.(*Error)
		}
	}()
	return p.eval(expr), nil
}

func (p *Interp) Eval(expr Expr) (s string, n float64, err error) {
	v, err := p.evalSafe(expr)
	if err != nil {
		return "", 0, err
	}
	return p.toString(v), v.num(), nil
}

func EvalLine(src, line string) (s string, n float64, err error) {
	expr, err := ParseExpr([]byte(src))
	if err != nil {
		return "", 0, err
	}
	interp := New(nil) // expressions can't write to output
	interp.setLine(line)
	return interp.Eval(expr)
}

func Eval(src string) (s string, n float64, err error) {
	return EvalLine(src, "")
}

func (p *Interp) eval(expr Expr) value {
	switch e := expr.(type) {
	case *UnaryExpr:
		value := p.eval(e.Value)
		return unaryFuncs[e.Op](p, value)
	case *BinaryExpr:
		left := p.eval(e.Left)
		switch e.Op {
		case AND:
			if !left.boolean() {
				return num(0)
			}
			right := p.eval(e.Right)
			return boolean(right.boolean())
		case OR:
			if left.boolean() {
				return num(1)
			}
			right := p.eval(e.Right)
			return boolean(right.boolean())
		case MATCH, NOT_MATCH:
			if regExpr, ok := e.Right.(*RegExpr); ok {
				re := p.mustCompile(regExpr.Regex)
				matched := re.MatchString(p.toString(left))
				if e.Op == MATCH {
					return boolean(matched)
				} else {
					return boolean(!matched)
				}
			}
			fallthrough
		default:
			right := p.eval(e.Right)
			return binaryFuncs[e.Op](p, left, right)
		}
	case *InExpr:
		index := p.eval(e.Index)
		_, ok := p.arrays[e.Array][p.toString(index)]
		return boolean(ok)
	case *CondExpr:
		cond := p.eval(e.Cond)
		if cond.boolean() {
			return p.eval(e.True)
		} else {
			return p.eval(e.False)
		}
	case *NumExpr:
		return num(e.Value)
	case *StrExpr:
		return str(e.Value)
	case *RegExpr:
		// Stand-alone /regex/ is equivalent to: $0 ~ /regex/
		re := p.mustCompile(e.Regex)
		return boolean(re.MatchString(p.line))
	case *FieldExpr:
		index := p.eval(e.Index)
		// TODO: should error if index is a non-number string
		return p.getField(int(index.num()))
	case *VarExpr:
		return p.getVar(e.Name)
	case *IndexExpr:
		index := p.eval(e.Index[0]) // TODO: handle multi index
		return p.getArray(e.Name, p.toString(index))
	case *AssignExpr:
		right := p.eval(e.Right)
		if e.Op != ASSIGN {
			left := p.eval(e.Left)
			var op Token
			switch e.Op {
			case ADD_ASSIGN:
				op = ADD
			case SUB_ASSIGN:
				op = SUB
			case DIV_ASSIGN:
				op = DIV
			case MOD_ASSIGN:
				op = MOD
			case MUL_ASSIGN:
				op = MUL
			case POW_ASSIGN:
				op = POW
			default:
				panic(fmt.Sprintf("unexpected assignment operator: %s", e.Op))
			}
			right = binaryFuncs[op](p, left, right)
		}
		p.assign(e.Left, right)
		return right
	case *IncrExpr:
		leftValue := p.eval(e.Left)
		left := leftValue.num()
		var right float64
		switch e.Op {
		case INCR:
			right = left + 1
		case DECR:
			right = left - 1
		}
		rightValue := num(right)
		p.assign(e.Left, rightValue)
		if e.Pre {
			return rightValue
		} else {
			return num(left)
		}
	case *CallExpr:
		switch e.Func {
		case F_SPLIT:
			str := p.toString(p.eval(e.Args[0]))
			var fieldSep string
			if len(e.Args) == 3 {
				fieldSep = p.toString(p.eval(e.Args[2]))
			} else {
				fieldSep = p.fieldSep
			}
			array := e.Args[1].(*VarExpr).Name
			return num(float64(p.split(str, array, fieldSep)))
		case F_SUB, F_GSUB:
			regex := p.toString(p.eval(e.Args[0]))
			repl := p.toString(p.eval(e.Args[1]))
			var in string
			if len(e.Args) == 3 {
				in = p.toString(p.eval(e.Args[2]))
			} else {
				in = p.line
			}
			out, n := p.sub(regex, repl, in, e.Func == F_GSUB)
			if len(e.Args) == 3 {
				p.assign(e.Args[2], str(out))
			} else {
				p.setLine(out)
			}
			return num(float64(n))
		default:
			args := make([]value, len(e.Args))
			for i, a := range e.Args {
				args[i] = p.eval(a)
			}
			return p.call(e.Func, args)
		}
	default:
		panic(fmt.Sprintf("unexpected expr type: %T", expr))
	}
}

func (p *Interp) setFile(filename string) {
	p.filename = filename
	p.fileLineNum = 0
}

func (p *Interp) setLine(line string) {
	p.line = line
	if p.fieldSep == " " {
		p.fields = strings.Fields(line)
	} else {
		p.fields = p.fieldSepRegex.Split(line, -1)
	}
	p.numFields = len(p.fields)
}

func (p *Interp) nextLine(line string) {
	p.setLine(line)
	p.lineNum++
	p.fileLineNum++
}

func (p *Interp) getVar(name string) value {
	switch name {
	case "CONVFMT":
		return str(p.convertFormat)
	case "FILENAME":
		return numStr(p.filename)
	case "FNR":
		return num(float64(p.fileLineNum))
	case "FS":
		return str(p.fieldSep)
	case "NF":
		return num(float64(p.numFields))
	case "NR":
		return num(float64(p.lineNum))
	case "OFMT":
		return str(p.outputFormat)
	case "OFS":
		return str(p.outputFieldSep)
	case "ORS":
		return str(p.outputRecordSep)
	case "RLENGTH":
		return num(float64(p.matchLength))
	case "RS":
		return str("\n")
	case "RSTART":
		return num(float64(p.matchStart))
	case "SUBSEP":
		return str(p.subscriptSep)
	default:
		return p.vars[name]
	}
}

func (p *Interp) SetVar(name string, value string) error {
	return p.setVarError(name, str(value))
}

func (p *Interp) setVarError(name string, v value) error {
	// TODO: should the types of the built-in variables roundtrip?
	// i.e., if you set NF to a string should it read back as a string?
	// $ awk 'BEGIN { NF = "3.0"; print (NF == "3.0"); }'
	// 1
	switch name {
	case "CONVFMT":
		p.convertFormat = p.toString(v)
	case "FILENAME":
		p.filename = p.toString(v)
	case "FNR":
		p.fileLineNum = int(v.num())
	case "FS":
		p.fieldSep = p.toString(v)
		if p.fieldSep != " " {
			re, err := regexp.Compile(p.fieldSep)
			if err != nil {
				return newError("invalid regex %q: %s", p.fieldSep, err)
			}
			p.fieldSepRegex = re
		}
	case "NF":
		p.numFields = int(v.num())
	case "NR":
		p.lineNum = int(v.num())
	case "OFMT":
		p.outputFormat = p.toString(v)
	case "OFS":
		p.outputFieldSep = p.toString(v)
	case "ORS":
		p.outputRecordSep = p.toString(v)
	case "RLENGTH":
		p.matchLength = int(v.num())
	case "RS":
		panic(newError("assigning RS not supported"))
	case "RSTART":
		p.matchStart = int(v.num())
	case "SUBSEP":
		p.subscriptSep = p.toString(v)
	default:
		p.vars[name] = v
	}
	return nil
}

func (p *Interp) setVar(name string, v value) {
	err := p.setVarError(name, v)
	if err != nil {
		panic(err)
	}
}

func (p *Interp) getArray(name, index string) value {
	return p.arrays[name][index]
}

func (p *Interp) setArray(name, index string, v value) {
	array, ok := p.arrays[name]
	if !ok {
		array = make(map[string]value)
		p.arrays[name] = array
	}
	array[index] = v
}

func (p *Interp) getField(index int) value {
	if index < 0 {
		panic(newError("field index negative: %d", index))
	}
	if index == 0 {
		return numStr(p.line)
	}
	if index > len(p.fields) {
		return numStr("")
	}
	return numStr(p.fields[index-1])
}

func (p *Interp) SetField(index int, value string) {
	if index < 0 {
		panic(newError("field index negative: %d", index))
	}
	if index == 0 {
		p.setLine(value)
		return
	}
	if index > len(p.fields) {
		for i := len(p.fields); i < index; i++ {
			p.fields = append(p.fields, "")
		}
	}
	p.fields[index-1] = value
	if index > p.numFields {
		p.fields = p.fields[:index]
	}
	p.numFields = len(p.fields)
	p.line = strings.Join(p.fields, p.outputFieldSep)
}

func (p *Interp) toString(v value) string {
	return v.str(p.convertFormat)
}

func (p *Interp) mustCompile(regex string) *regexp.Regexp {
	// TODO: cache
	re, err := regexp.Compile(regex)
	if err != nil {
		panic(newError("invalid regex %q: %s", regex, err))
	}
	return re
}

type binaryFunc func(p *Interp, l, r value) value

var binaryFuncs = map[Token]binaryFunc{
	EQUALS: (*Interp).equal,
	NOT_EQUALS: func(p *Interp, l, r value) value {
		return p.not(p.equal(l, r))
	},
	LESS: (*Interp).lessThan,
	LTE: func(p *Interp, l, r value) value {
		return p.not(p.lessThan(r, l))
	},
	GREATER: func(p *Interp, l, r value) value {
		return p.lessThan(r, l)
	},
	GTE: func(p *Interp, l, r value) value {
		return p.not(p.lessThan(l, r))
	},
	ADD: func(p *Interp, l, r value) value {
		return num(l.num() + r.num())
	},
	SUB: func(p *Interp, l, r value) value {
		return num(l.num() - r.num())
	},
	MUL: func(p *Interp, l, r value) value {
		return num(l.num() * r.num())
	},
	POW: func(p *Interp, l, r value) value {
		return num(math.Pow(l.num(), r.num()))
	},
	DIV: func(p *Interp, l, r value) value {
		rf := r.num()
		if rf == 0.0 {
			panic(newError("division by zero"))
		}
		return num(l.num() / rf)
	},
	MOD: func(p *Interp, l, r value) value {
		rf := r.num()
		if rf == 0.0 {
			panic(newError("division by zero in mod"))
		}
		return num(math.Mod(l.num(), rf))
	},
	CONCAT: func(p *Interp, l, r value) value {
		return str(p.toString(l) + p.toString(r))
	},
	MATCH: (*Interp).regexMatch,
	NOT_MATCH: func(p *Interp, l, r value) value {
		return p.not(p.regexMatch(l, r))
	},
}

func (p *Interp) equal(l, r value) value {
	if l.isTrueStr() || r.isTrueStr() {
		return boolean(p.toString(l) == p.toString(r))
	} else {
		return boolean(l.n == r.n)
	}
}

func (p *Interp) lessThan(l, r value) value {
	if l.isTrueStr() || r.isTrueStr() {
		return boolean(p.toString(l) < p.toString(r))
	} else {
		return boolean(l.n < r.n)
	}
}

func (p *Interp) regexMatch(l, r value) value {
	re := p.mustCompile(p.toString(r))
	matched := re.MatchString(p.toString(l))
	return boolean(matched)
}

type unaryFunc func(p *Interp, v value) value

var unaryFuncs = map[Token]unaryFunc{
	NOT: (*Interp).not,
	ADD: func(p *Interp, v value) value {
		return num(v.num())
	},
	SUB: func(p *Interp, v value) value {
		return num(-v.num())
	},
}

func (p *Interp) not(v value) value {
	return boolean(!v.boolean())
}

func (p *Interp) call(op Token, args []value) value {
	switch op {
	case F_ATAN2:
		return num(math.Atan2(args[0].num(), args[1].num()))
	case F_COS:
		return num(math.Cos(args[0].num()))
	case F_EXP:
		return num(math.Exp(args[0].num()))
	case F_INDEX:
		s := p.toString(args[0])
		substr := p.toString(args[1])
		return num(float64(strings.Index(s, substr) + 1))
	case F_INT:
		return num(float64(int(args[0].num())))
	case F_LENGTH:
		switch len(args) {
		case 0:
			return num(float64(len(p.line)))
		default:
			return num(float64(len(p.toString(args[0]))))
		}
	case F_LOG:
		return num(math.Log(args[0].num()))
	case F_MATCH:
		re := p.mustCompile(p.toString(args[1]))
		loc := re.FindStringIndex(p.toString(args[0]))
		if loc == nil {
			p.matchStart = 0
			p.matchLength = -1
			return num(0)
		}
		p.matchStart = loc[0] + 1
		p.matchLength = loc[1] - loc[0]
		return num(float64(p.matchStart))
	case F_SPRINTF:
		return str(p.sprintf(p.toString(args[0]), args[1:]))
	case F_SQRT:
		return num(math.Sqrt(args[0].num()))
	case F_RAND:
		return num(p.random.Float64())
	case F_SIN:
		return num(math.Sin(args[0].num()))
	case F_SRAND:
		switch len(args) {
		case 0:
			p.random.Seed(time.Now().UnixNano())
		case 1:
			// TODO: truncating the fraction part here, is that okay?
			p.random.Seed(int64(args[0].num()))
		}
		// TODO: previous seed value should be returned
		return num(0)
	case F_SUBSTR:
		s := p.toString(args[0])
		pos := int(args[1].num())
		if pos > len(s) {
			pos = len(s)
		}
		if pos < 1 {
			pos = 1
		}
		maxLength := len(s) - pos + 1
		length := maxLength
		if len(args) == 3 {
			length = int(args[2].num())
			if length < 0 {
				length = 0
			}
			if length > maxLength {
				length = maxLength
			}
		}
		return str(s[pos-1 : pos-1+length])
	case F_TOLOWER:
		return str(strings.ToLower(p.toString(args[0])))
	case F_TOUPPER:
		return str(strings.ToUpper(p.toString(args[0])))
	default:
		panic(fmt.Sprintf("unexpected function: %s", op))
	}
}

func (p *Interp) split(s, arrayName, fs string) int {
	var parts []string
	if fs == " " {
		parts = strings.Fields(s)
	} else {
		re := p.mustCompile(fs)
		parts = re.Split(s, -1)
	}
	array := make(map[string]value)
	for i, part := range parts {
		array[strconv.Itoa(i+1)] = numStr(part)
	}
	p.arrays[arrayName] = array
	return len(array)
}

func (p *Interp) sub(regex, repl, in string, global bool) (out string, num int) {
	// TODO: ampersand handling
	re := p.mustCompile(regex)
	count := 0
	out = re.ReplaceAllStringFunc(in, func(s string) string {
		if !global && count > 0 {
			return s
		}
		count++
		return repl
	})
	return out, count
}

func parseFmtTypes(s string) (format string, types []byte, err error) {
	out := []byte(s)
	for i := 0; i < len(s); i++ {
		if s[i] == '%' {
			i++
			if i >= len(s) {
				return "", nil, errors.New("expected type specifier after %")
			}
			if s[i] == '%' {
				i++
				continue
			}
			for i < len(s) && bytes.IndexByte([]byte(".-+*#0123456789"), s[i]) >= 0 {
				if s[i] == '*' {
					types = append(types, 'd')
				}
				i++
			}
			if i >= len(s) {
				return "", nil, errors.New("expected type specifier after %")
			}
			var t byte
			switch s[i] {
			case 'd', 'i', 'o', 'x', 'X':
				t = 'd'
			case 'u':
				t = 'u'
				out[i] = 'd'
			case 'c':
				t = 'c'
				out[i] = 's'
			case 'f', 'e', 'E', 'g', 'G':
				t = 'f'
			case 's':
				t = 's'
			default:
				return "", nil, fmt.Errorf("invalid format type %q", s[i])
			}
			types = append(types, t)
		}
	}
	return string(out), types, nil
}

func (p *Interp) sprintf(format string, args []value) string {
	format, types, err := parseFmtTypes(format)
	if err != nil {
		panic(newError("format error: %s", err))
	}
	if len(types) != len(args) {
		panic(newError("format error: got %d args, expected %d", len(args), len(types)))
	}
	converted := make([]interface{}, len(args))
	for i, a := range args {
		var v interface{}
		switch types[i] {
		case 'd':
			v = int(a.num())
		case 'u':
			v = uint32(a.num())
		case 'c':
			var c []byte
			if a.isTrueStr() {
				s := p.toString(a)
				if len(s) > 0 {
					c = []byte{s[0]}
				}
			} else {
				c = []byte{byte(a.num())}
			}
			v = c
		case 'f':
			v = a.num()
		case 's':
			v = p.toString(a)
		}
		converted[i] = v
	}
	return fmt.Sprintf(format, converted...)
}

func (p *Interp) assign(left Expr, right value) {
	switch left := left.(type) {
	case *VarExpr:
		p.setVar(left.Name, right)
	case *IndexExpr:
		index := p.eval(left.Index[0]) // TODO: handle multi index
		p.setArray(left.Name, p.toString(index), right)
	case *FieldExpr:
		index := p.eval(left.Index)
		// TODO: should error if index is a non-number string
		p.SetField(int(index.num()), p.toString(right))
	default:
		panic(fmt.Sprintf("unexpected lvalue type: %T", left))
	}
}
