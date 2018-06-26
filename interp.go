package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Type int

const (
	TypeNil Type = iota
	TypeStr
	TypeNum
)

type Value struct {
	typ      Type
	isNumStr bool
	str      string
	num      float64
}

func Num(n float64) Value {
	return Value{typ: TypeNum, num: n}
}

func Str(s string) Value {
	return Value{typ: TypeStr, str: s}
}

func NumStr(s string) Value {
	n, err := strconv.ParseFloat(s, 64)
	return Value{typ: TypeStr, isNumStr: err == nil, str: s, num: n}
}

func Bool(b bool) Value {
	if b {
		return Num(1)
	}
	return Num(0)
}

func (v Value) isTrueStr() bool {
	return v.typ == TypeStr && !v.isNumStr
}

func (v Value) Bool() bool {
	if v.isTrueStr() {
		return v.str != ""
	} else {
		return v.num != 0
	}
}

func (v Value) String(floatFormat string) string {
	switch v.typ {
	case TypeNum:
		if v.num == float64(int(v.num)) {
			return strconv.Itoa(int(v.num))
		} else {
			return fmt.Sprintf(floatFormat, v.num)
		}
	case TypeStr:
		return v.str
	default:
		return ""
	}
}

func (v Value) AWKString() string {
	switch v.typ {
	case TypeNum:
		return v.String("%.6g")
	case TypeStr:
		return strconv.Quote(v.str)
	default:
		return "<undefined>"
	}
}

func (v Value) Float() float64 {
	switch v.typ {
	case TypeNum:
		return v.num
	case TypeStr:
		// TODO: handle cases like "3x"
		f, _ := strconv.ParseFloat(v.str, 64)
		return f
	default:
		return 0
	}
}

type Interp struct {
	program *Program
	output  io.Writer
	vars    map[string]Value
	arrays  map[string]map[string]Value
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
	outputFieldSep  string
	outputRecordSep string
	subscriptSep    string
	matchLength     int
	matchStart      int
}

func NewInterp(program *Program, output io.Writer) *Interp {
	p := &Interp{}
	p.program = program
	p.output = output
	p.vars = make(map[string]Value)
	p.arrays = make(map[string]map[string]Value)
	p.random = rand.New(rand.NewSource(0))
	p.convertFormat = "%.6g"
	p.outputFormat = "%.6g"
	p.fieldSep = " "
	p.outputFieldSep = " "
	p.outputRecordSep = "\n"
	p.subscriptSep = "\x1c"
	return p
}

func (p *Interp) ExecuteBegin() error {
	for _, statements := range p.program.Begin {
		p.Executes(statements)
		// TODO: error handling
		// if err != nil {
		//     return err
		// }
	}
	return nil
}

func (p *Interp) ExecuteFile(filename string, input io.Reader) error {
	// TODO: error handling
	p.SetFile(filename)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		p.NextLine(scanner.Text())
		for _, action := range p.program.Actions {
			pattern := p.Evaluate(action.Pattern)
			if pattern.Bool() {
				p.Executes(action.Stmts)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading lines from input: %s", err)
	}
	return nil
}

func (p *Interp) ExecuteEnd() error {
	for _, statements := range p.program.End {
		p.Executes(statements)
		// TODO: error handling
		// if err != nil {
		//     return err
		// }
	}
	return nil
}

func (p *Interp) Executes(stmts Stmts) {
	for _, s := range stmts {
		p.Execute(s)
	}
}

func (p *Interp) Execute(stmt Stmt) {
	switch s := stmt.(type) {
	case *PrintStmt:
		strs := make([]string, len(s.Args))
		for i, a := range s.Args {
			value := p.Evaluate(a)
			strs[i] = value.String(p.outputFormat)
		}
		line := strings.Join(strs, p.outputFieldSep)
		io.WriteString(p.output, line+p.outputRecordSep)
	case *IfStmt:
		if p.Evaluate(s.Cond).Bool() {
			p.Executes(s.Body)
		} else {
			p.Executes(s.Else)
		}
	case *ForStmt:
		p.Execute(s.Pre)
		for p.Evaluate(s.Cond).Bool() {
			p.Executes(s.Body)
			p.Execute(s.Post)
		}
	case *WhileStmt:
		for p.Evaluate(s.Cond).Bool() {
			p.Executes(s.Body)
		}
	case *DoWhileStmt:
		for {
			p.Executes(s.Body)
			if !p.Evaluate(s.Cond).Bool() {
				break
			}
		}
	case *ExprStmt:
		p.Evaluate(s.Expr)
	default:
		panic(fmt.Sprintf("unexpected stmt type: %T", stmt))
	}
}

func (p *Interp) Evaluate(expr Expr) Value {
	switch e := expr.(type) {
	case *UnaryExpr:
		value := p.Evaluate(e.Value)
		return unaryFuncs[e.Op](p, value)
	case *BinaryExpr:
		left := p.Evaluate(e.Left)
		switch e.Op {
		case "&&":
			if !left.Bool() {
				return Num(0)
			}
			right := p.Evaluate(e.Right)
			return Bool(right.Bool())
		case "||":
			if left.Bool() {
				return Num(1)
			}
			right := p.Evaluate(e.Right)
			return Bool(right.Bool())
		default:
			right := p.Evaluate(e.Right)
			return binaryFuncs[e.Op](p, left, right)
		}
	case *CondExpr:
		cond := p.Evaluate(e.Cond)
		if cond.Bool() {
			return p.Evaluate(e.True)
		} else {
			return p.Evaluate(e.False)
		}
	case *ConstExpr:
		return e.Value
	case *FieldExpr:
		index := p.Evaluate(e.Index)
		// TODO: should error if index is a non-number string
		return p.GetField(int(index.Float()))
	case *VarExpr:
		return p.GetVar(e.Name)
	case *ArrayExpr:
		index := p.Evaluate(e.Index)
		return p.GetArray(e.Name, p.ToString(index))
	case *AssignExpr:
		right := p.Evaluate(e.Right)
		if e.Op != "" {
			left := p.Evaluate(e.Left)
			right = binaryFuncs[e.Op](p, left, right)
		}
		p.assign(e.Left, right)
		return right
	case *IncrExpr:
		leftValue := p.Evaluate(e.Left)
		left := leftValue.Float()
		var right float64
		switch e.Op {
		case "++":
			right = left + 1
		case "--":
			right = left - 1
		}
		rightValue := Num(right)
		p.assign(e.Left, rightValue)
		if e.Pre {
			return rightValue
		} else {
			return Num(left)
		}
	case *CallExpr:
		args := make([]Value, len(e.Args))
		for i, a := range e.Args {
			args[i] = p.Evaluate(a)
		}
		return p.call(e.Name, args)
	default:
		panic(fmt.Sprintf("unexpected expr type: %T", expr))
	}
}

func (p *Interp) SetFile(filename string) {
	p.filename = filename
	p.fileLineNum = 0
}

func (p *Interp) SetLine(line string) {
	p.line = line
	if p.fieldSep == " " {
		p.fields = strings.Fields(line)
	} else {
		// TODO: use regex field separator
	}
	p.numFields = len(p.fields)
}

func (p *Interp) NextLine(line string) {
	p.SetLine(line)
	p.lineNum++
	p.fileLineNum++
}

func (p *Interp) GetVar(name string) Value {
	switch name {
	case "CONVFMT":
		return Str(p.convertFormat)
	case "FILENAME":
		return NumStr(p.filename)
	case "FNR":
		return Num(float64(p.fileLineNum))
	case "FS":
		return Str(p.fieldSep)
	case "NF":
		return Num(float64(p.numFields))
	case "NR":
		return Num(float64(p.lineNum))
	case "OFMT":
		return Str(p.outputFormat)
	case "OFS":
		return Str(p.outputFieldSep)
	case "ORS":
		return Str(p.outputRecordSep)
	case "RLENGTH":
		return Num(float64(p.matchLength))
	case "RS":
		return Str("\n")
	case "RSTART":
		return Num(float64(p.matchStart))
	case "SUBSEP":
		return Str(p.subscriptSep)
	default:
		return p.vars[name]
	}
}

func (p *Interp) SetVar(name string, value Value) {
	switch name {
	case "CONVFMT":
		p.convertFormat = p.ToString(value)
	case "FILENAME":
		p.filename = p.ToString(value)
	case "FNR":
		p.fileLineNum = int(value.Float())
	case "FS":
		p.fieldSep = p.ToString(value)
	case "NF":
		p.numFields = int(value.Float())
	case "NR":
		p.lineNum = int(value.Float())
	case "OFMT":
		p.outputFormat = p.ToString(value)
	case "OFS":
		p.outputFieldSep = p.ToString(value)
	case "ORS":
		p.outputRecordSep = p.ToString(value)
	case "RLENGTH":
		p.matchLength = int(value.Float())
	case "RS":
		panic("assigning RS not supported")
	case "RSTART":
		p.matchStart = int(value.Float())
	case "SUBSEP":
		p.subscriptSep = p.ToString(value)
	default:
		p.vars[name] = value
	}
}

func (p *Interp) GetArray(name, index string) Value {
	return p.arrays[name][index]
}

func (p *Interp) SetArray(name, index string, value Value) {
	array, ok := p.arrays[name]
	if !ok {
		array = make(map[string]Value)
		p.arrays[name] = array
	}
	array[index] = value
}

func (p *Interp) GetField(index int) Value {
	if index < 0 {
		panic(fmt.Sprintf("field index negative: %d", index))
	}
	if index == 0 {
		return NumStr(p.line)
	}
	if index > len(p.fields) {
		return NumStr("")
	}
	return NumStr(p.fields[index-1])
}

func (p *Interp) SetField(index int, value string) {
	if index < 0 {
		panic(fmt.Sprintf("field index negative: %d", index))
	}
	if index == 0 {
		p.SetLine(value)
		return
	}
	if index > len(p.fields) {
		// TODO: append "" fields as needed
	}
	p.fields[index-1] = value
}

func (p *Interp) ToString(v Value) string {
	return v.String(p.convertFormat)
}

type binaryFunc func(p *Interp, l, r Value) Value

var binaryFuncs = map[string]binaryFunc{
	"==": (*Interp).equal,
	"!=": func(p *Interp, l, r Value) Value {
		return p.not(p.equal(l, r))
	},
	"<": (*Interp).lessThan,
	">=": func(p *Interp, l, r Value) Value {
		return p.not(p.lessThan(l, r))
	},
	">": func(p *Interp, l, r Value) Value {
		return p.lessThan(r, l)
	},
	"<=": func(p *Interp, l, r Value) Value {
		return p.not(p.lessThan(r, l))
	},
	"+": func(p *Interp, l, r Value) Value {
		return Num(l.Float() + r.Float())
	},
	"-": func(p *Interp, l, r Value) Value {
		return Num(l.Float() - r.Float())
	},
	"*": func(p *Interp, l, r Value) Value {
		return Num(l.Float() * r.Float())
	},
	"^": func(p *Interp, l, r Value) Value {
		return Num(math.Pow(l.Float(), r.Float()))
	},
	"/": func(p *Interp, l, r Value) Value {
		rf := r.Float()
		if rf == 0.0 {
			panic("division by zero")
		}
		return Num(l.Float() / rf)
	},
	"%": func(p *Interp, l, r Value) Value {
		rf := r.Float()
		if rf == 0.0 {
			panic("division by zero in mod")
		}
		return Num(math.Mod(l.Float(), rf))
	},
	"": func(p *Interp, l, r Value) Value {
		return Str(p.ToString(l) + p.ToString(r))
	},
	"~": (*Interp).regexMatch,
	"!~": func(p *Interp, l, r Value) Value {
		return p.not(p.regexMatch(l, r))
	},
}

func (p *Interp) equal(l, r Value) Value {
	if l.isTrueStr() || r.isTrueStr() {
		return Bool(p.ToString(l) == p.ToString(r))
	} else {
		return Bool(l.num == r.num)
	}
}

func (p *Interp) lessThan(l, r Value) Value {
	if l.isTrueStr() || r.isTrueStr() {
		return Bool(p.ToString(l) < p.ToString(r))
	} else {
		return Bool(l.num < r.num)
	}
}

func (p *Interp) regexMatch(l, r Value) Value {
	matched, err := regexp.MatchString(p.ToString(r), p.ToString(l))
	if err != nil {
		panic(fmt.Sprintf("invalid regex %q", p.ToString(r)))
	}
	return Bool(matched)
}

type unaryFunc func(p *Interp, v Value) Value

var unaryFuncs = map[string]unaryFunc{
	"!": (*Interp).not,
	"+": func(p *Interp, v Value) Value {
		return Num(v.Float())
	},
	"-": func(p *Interp, v Value) Value {
		return Num(-v.Float())
	},
}

func (p *Interp) not(v Value) Value {
	return Bool(!v.Bool())
}

func (p *Interp) checkNumArgs(name string, actual, expected int) {
	if actual != expected {
		panic(fmt.Sprintf("%s() expects %d args, got %d", name, expected, actual))
	}
}

func (p *Interp) call(name string, args []Value) Value {
	switch name {
	case "atan2":
		p.checkNumArgs("atan2", len(args), 2)
		return Num(math.Atan2(args[0].Float(), args[1].Float()))
	case "cos":
		p.checkNumArgs("cos", len(args), 1)
		return Num(math.Cos(args[0].Float()))
	case "exp":
		p.checkNumArgs("exp", len(args), 1)
		return Num(math.Exp(args[0].Float()))
	case "index":
		p.checkNumArgs("index", len(args), 2)
		s := p.ToString(args[0])
		substr := p.ToString(args[1])
		return Num(float64(strings.Index(s, substr) + 1))
	case "int":
		p.checkNumArgs("int", len(args), 1)
		return Num(float64(int(args[0].Float())))
	case "length":
		switch len(args) {
		case 0:
			return Num(float64(len(p.line)))
		case 1:
			return Num(float64(len(p.ToString(args[0]))))
		default:
			panic(fmt.Sprintf("length() expects 0 or 1 arg, got %d", len(args)))
		}
	case "log":
		p.checkNumArgs("log", len(args), 1)
		return Num(math.Log(args[0].Float()))
	case "match":
		p.checkNumArgs("match", len(args), 2)
		// TODO: regexen are probably fixed, cache
		re, err := regexp.Compile(p.ToString(args[1]))
		if err != nil {
			panic(fmt.Sprintf("invalid regex %q", p.ToString(args[1])))
		}
		loc := re.FindStringIndex(p.ToString(args[0]))
		if loc == nil {
			p.matchStart = 0
			p.matchLength = -1
			return Num(0)
		}
		p.matchStart = loc[0] + 1
		p.matchLength = loc[1] - loc[0]
		return Num(float64(p.matchStart))
	case "sprintf":
		if len(args) < 1 {
			panic(fmt.Sprintf("sprintf() expects 1 or more args, got %d", len(args)))
		}
		vals := make([]interface{}, len(args)-1)
		for i, a := range args[1:] {
			vals[i] = interface{}(a)
		}
		return Str(fmt.Sprintf(p.ToString(args[0]), vals...))
	case "sqrt":
		p.checkNumArgs("sqrt", len(args), 1)
		return Num(math.Sqrt(args[0].Float()))
	case "rand":
		p.checkNumArgs("rand", len(args), 0)
		return Num(p.random.Float64())
	case "sin":
		p.checkNumArgs("sin", len(args), 1)
		return Num(math.Sin(args[0].Float()))
	case "srand":
		switch len(args) {
		case 0:
			p.random.Seed(time.Now().UnixNano())
		case 1:
			// TODO: truncating the fraction part here, is that okay?
			p.random.Seed(int64(args[0].Float()))
		default:
			panic(fmt.Sprintf("srand() expects 0 or 1 arg, got %d", len(args)))
		}
		// TODO: previous seed value should be returned
		return Num(0)
	case "sub", "gsub":
		// TODO: ampersand handling
		var input string
		switch len(args) {
		case 2:
			input = p.line
		case 3:
			input = p.ToString(args[2])
		default:
			panic(fmt.Sprintf("sub/gsub() expects 2 or 3 args, got %d", len(args)))
		}
		re, err := regexp.Compile(p.ToString(args[0]))
		if err != nil {
			panic(fmt.Sprintf("invalid regex %q", p.ToString(args[0])))
		}
		repl := p.ToString(args[1])
		count := 0
		output := re.ReplaceAllStringFunc(input, func(s string) string {
			if name == "sub" && count > 0 {
				// TODO: kind of a hacky way to do it
				return s
			}
			count++
			return repl
		})
		// TODO: fix for when input is specific (sub/gsub are actually assignments!)
		p.SetLine(output)
		return Num(float64(count))
	case "substr":
		// TODO: untested
		if len(args) != 2 && len(args) != 3 {
			panic(fmt.Sprintf("substr() expects 2 or 3 args, got %d", len(args)))
		}
		str := p.ToString(args[0])
		pos := int(args[1].Float())
		if pos < 1 {
			pos = 1
		}
		if pos > len(str) {
			pos = len(str)
		}
		maxLength := len(str) - pos + 1
		length := maxLength
		if len(args) == 3 {
			length = int(args[2].Float())
			if length < 0 {
				length = 0
			}
			if length > maxLength {
				length = maxLength
			}
		}
		return Str(str[pos-1 : pos-1+length])
	case "tolower":
		p.checkNumArgs("tolower", len(args), 1)
		return Str(strings.ToLower(p.ToString(args[0])))
	case "toupper":
		p.checkNumArgs("toupper", len(args), 1)
		return Str(strings.ToUpper(p.ToString(args[0])))
	default:
		panic(fmt.Sprintf("unexpected function name: %q", name))
	}
}

func (p *Interp) assign(left Expr, right Value) {
	switch left := left.(type) {
	case *VarExpr:
		p.SetVar(left.Name, right)
	case *ArrayExpr:
		index := p.Evaluate(left.Index)
		p.SetArray(left.Name, p.ToString(index), right)
	case *FieldExpr:
		index := p.Evaluate(left.Index)
		// TODO: should error if index is a non-number string
		p.SetField(int(index.Float()), p.ToString(right))
	default:
		panic(fmt.Sprintf("unexpected lvalue type: %T", left))
	}
}
