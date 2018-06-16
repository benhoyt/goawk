package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Value interface{}

func BoolValue(b bool) Value {
	if b {
		return 1.0
	}
	return 0.0
}

type Interp struct {
	program *Program
	output  io.Writer
	vars    map[string]Value
	arrays  map[string]map[string]Value
	random  *rand.Rand

	line        string
	fields      []string
	numFields   float64
	lineNum     float64
	filename    string
	fileLineNum float64

	convertFormat   string
	outputFormat    string
	fieldSep        string
	outputFieldSep  string
	outputRecordSep string
	subscriptSep    string
	matchLength     float64
	matchStart      float64
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
			if p.ToBool(pattern) {
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
			strs[i] = p.ToOutputString(p.Evaluate(a))
		}
		line := strings.Join(strs, p.outputFieldSep)
		io.WriteString(p.output, line+p.outputRecordSep)
	case *ExprStmt:
		p.Evaluate(s.Expr)
	default:
		panic(fmt.Sprintf("unexpected stmt type: %T", stmt))
	}
}

func (p *Interp) Evaluate(expr Expr) Value {
	switch e := expr.(type) {
	case *BinaryExpr:
		left := p.Evaluate(e.Left)
		right := p.Evaluate(e.Right)
		return binaryFuncs[e.Op](p, left, right)
	case *ConstExpr:
		return e.Value
	case *FieldExpr:
		index := p.Evaluate(e.Index)
		if f, ok := index.(float64); ok {
			return p.GetField(int(f))
		}
		panic(fmt.Sprintf("field index not a number: %q", index))
	case *VarExpr:
		return p.GetVar(e.Name)
	case *ArrayExpr:
		index := p.Evaluate(e.Index)
		return p.GetArray(e.Name, p.ToString(index))
	case *AssignExpr:
		rvalue := p.Evaluate(e.Right)
		switch left := e.Left.(type) {
		case *VarExpr:
			p.SetVar(left.Name, rvalue)
			return rvalue
		case *ArrayExpr:
			index := p.Evaluate(left.Index)
			p.SetArray(left.Name, p.ToString(index), rvalue)
			return rvalue
		case *FieldExpr:
			index := p.Evaluate(left.Index)
			if f, ok := index.(float64); ok {
				p.SetField(int(f), p.ToString(rvalue))
				return rvalue
			}
			panic(fmt.Sprintf("field index not a number: %q", index))
		default:
			panic(fmt.Sprintf("unexpected lvalue type: %T", e.Left))
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
	p.fields = strings.Fields(line)
	p.numFields = float64(len(p.fields))
}

func (p *Interp) NextLine(line string) {
	p.SetLine(line)
	p.lineNum++
	p.fileLineNum++
}

func (p *Interp) GetVar(name string) Value {
	switch name {
	case "CONVFMT":
		return p.convertFormat
	case "FILENAME":
		return p.filename
	case "FNR":
		return p.fileLineNum
	case "FS":
		return p.fieldSep
	case "NF":
		return p.numFields
	case "NR":
		return p.lineNum
	case "OFMT":
		return p.outputFormat
	case "OFS":
		return p.outputFieldSep
	case "ORS":
		return p.outputRecordSep
	case "RLENGTH":
		return p.matchLength
	case "RS":
		return "\n"
	case "RSTART":
		return p.matchStart
	case "SUBSEP":
		return p.subscriptSep
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
		p.fileLineNum = p.ToFloat(value)
	case "FS":
		p.fieldSep = p.ToString(value)
	case "NF":
		p.numFields = p.ToFloat(value)
	case "NR":
		p.lineNum = p.ToFloat(value)
	case "OFMT":
		p.outputFormat = p.ToString(value)
	case "OFS":
		p.outputFieldSep = p.ToString(value)
	case "ORS":
		p.outputRecordSep = p.ToString(value)
	case "RLENGTH":
		p.matchLength = p.ToFloat(value)
	case "RS":
		panic("assigning RS not supported")
	case "RSTART":
		p.matchStart = p.ToFloat(value)
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
		return p.line
	}
	if index > len(p.fields) {
		return ""
	}
	return p.fields[index-1]
}

func (p *Interp) SetField(index int, value string) {
	if index < 0 {
		panic(fmt.Sprintf("field index negative: %d", index))
	}
	if index == 0 {
		// TODO: update p.line and re-set fields
		p.SetLine(value)
		return
	}
	if index > len(p.fields) {
		// TODO: append "" fields as needed
	}
	p.fields[index-1] = value
}

func (p *Interp) ToBool(v Value) bool {
	switch v := v.(type) {
	case float64:
		return v != 0
	case string:
		return v != ""
	case nil:
		return false
	default:
		panic(fmt.Sprintf("unexpected type converting to bool: %T", v))
	}
}

func (p *Interp) ToFloat(v Value) float64 {
	switch v := v.(type) {
	case float64:
		return v
	case string:
		// TODO: handle cases like "3x"
		f, _ := strconv.ParseFloat(v, 64)
		return f
	case nil:
		return 0.0
	default:
		panic(fmt.Sprintf("unexpected type converting to float: %T", v))
	}
}

func (p *Interp) ToString(v Value) string {
	switch v := v.(type) {
	case float64:
		if v == float64(int(v)) {
			return fmt.Sprintf("%d", v)
		} else {
			return fmt.Sprintf(p.convertFormat, v)
		}
	case string:
		return v
	case nil:
		return ""
	default:
		panic(fmt.Sprintf("unexpected type converting to string: %T", v))
	}
}

func (p *Interp) ToOutputString(v Value) string {
	switch v := v.(type) {
	case float64:
		i := int(v)
		if v == float64(i) {
			return fmt.Sprintf("%d", i)
		} else {
			return fmt.Sprintf(p.outputFormat, v)
		}
	case string:
		return v
	case nil:
		return ""
	default:
		panic(fmt.Sprintf("unexpected type converting to string: %T", v))
	}
}

type binaryFunc func(p *Interp, l, r Value) Value

var binaryFuncs = map[string]binaryFunc{
	"==": (*Interp).equal,
	"!=": func(p *Interp, l, r Value) Value {
		return p.not(p.equal(l, r))
	},
	"+": func(p *Interp, l, r Value) Value {
		return p.ToFloat(l) + p.ToFloat(r)
	},
	"-": func(p *Interp, l, r Value) Value {
		return p.ToFloat(l) + p.ToFloat(r)
	},
	"*": func(p *Interp, l, r Value) Value {
		return p.ToFloat(l) * p.ToFloat(r)
	},
	"^": func(p *Interp, l, r Value) Value {
		return math.Pow(p.ToFloat(l), p.ToFloat(r))
	},
	"/": func(p *Interp, l, r Value) Value {
		rf := p.ToFloat(r)
		if rf == 0.0 {
			panic("division by zero")
		}
		return p.ToFloat(l) / rf
	},
	"%": func(p *Interp, l, r Value) Value {
		rf := p.ToFloat(r)
		if rf == 0.0 {
			panic("division by zero in mod")
		}
		// TODO: integer/float handling?
		return int(p.ToFloat(l)) % int(rf)
	},
	"": func(p *Interp, l, r Value) Value {
		return p.ToString(l) + p.ToString(r)
	},
}

func (p *Interp) equal(l, r Value) Value {
	switch l := l.(type) {
	case float64:
		switch r := r.(type) {
		case float64:
			return BoolValue(l == r)
		case string:
			return BoolValue(l == p.ToFloat(r))
		}
	case string:
		switch r := r.(type) {
		case string:
			return BoolValue(l == r)
		case float64:
			return BoolValue(p.ToFloat(l) == r)
		}
	}
	// TODO: uninitialized value (nil)
	return 0.0
}

func (p *Interp) not(v Value) Value {
	return BoolValue(!p.ToBool(v))
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
		return math.Atan2(p.ToFloat(args[0]), p.ToFloat(args[1]))
	case "cos":
		p.checkNumArgs("cos", len(args), 1)
		return math.Cos(p.ToFloat(args[0]))
	case "exp":
		p.checkNumArgs("exp", len(args), 1)
		return math.Exp(p.ToFloat(args[0]))
	case "index":
		p.checkNumArgs("index", len(args), 2)
		s := p.ToString(args[0])
		substr := p.ToString(args[1])
		return float64(strings.Index(s, substr) + 1)
	case "int":
		p.checkNumArgs("int", len(args), 1)
		return float64(int(p.ToFloat(args[0])))
	case "log":
		p.checkNumArgs("log", len(args), 1)
		return math.Log(p.ToFloat(args[0]))
	case "sqrt":
		p.checkNumArgs("sqrt", len(args), 1)
		return math.Sqrt(p.ToFloat(args[0]))
	case "rand":
		p.checkNumArgs("rand", len(args), 0)
		return p.random.Float64()
	case "sin":
		p.checkNumArgs("sin", len(args), 1)
		return math.Sin(p.ToFloat(args[0]))
	case "srand":
		switch len(args) {
		case 0:
			p.random.Seed(time.Now().UnixNano())
		case 1:
			// TODO: truncating the fraction part here, is that okay?
			p.random.Seed(int64(p.ToFloat(args[0])))
		default:
			panic(fmt.Sprintf("srand() expects 0 or 1 arg, got %d", len(args)))
		}
		return nil
	case "tolower":
		p.checkNumArgs("tolower", len(args), 1)
		return strings.ToLower(p.ToString(args[0]))
	case "toupper":
		p.checkNumArgs("toupper", len(args), 1)
		return strings.ToUpper(p.ToString(args[0]))
	default:
		panic(fmt.Sprintf("unexpected function name: %q", name))
	}
}
