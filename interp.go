package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type Value interface{}

func BoolValue(b bool) Value {
	if b {
		return 1.0
	}
	return 0.0
}

func ToBool(v Value) bool {
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

func ToFloat(v Value) float64 {
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

func ToString(v Value) string {
	switch v := v.(type) {
	case float64:
		// TODO: take output format into account
		return fmt.Sprintf("%v", v)
	case string:
		return v
	case nil:
		return ""
	default:
		panic(fmt.Sprintf("unexpected type converting to string: %T", v))
	}
}

type Interp struct {
	program *Program
	vars    map[string]Value
	arrays  map[string]map[string]Value

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

func NewInterp(program *Program) *Interp {
	p := &Interp{}
	p.program = program
	p.vars = make(map[string]Value)
	p.arrays = make(map[string]map[string]Value)
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
			if ToBool(pattern) {
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
		// TODO: convert to string properly, respecting output format
		// TODO: handle nil (undefined)
		args := make([]interface{}, len(s.Args))
		for i, a := range s.Args {
			args[i] = p.Evaluate(a)
		}
		fmt.Println(args...)
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
		return binaryFuncs[e.Op](left, right)
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
		return p.GetArray(e.Name, ToString(index))
	case *AssignExpr:
		rvalue := p.Evaluate(e.Right)
		switch left := e.Left.(type) {
		case *VarExpr:
			p.SetVar(left.Name, rvalue)
			return rvalue
		case *ArrayExpr:
			index := p.Evaluate(left.Index)
			p.SetArray(left.Name, ToString(index), rvalue)
			return rvalue
		case *FieldExpr:
			index := p.Evaluate(left.Index)
			if f, ok := index.(float64); ok {
				p.SetField(int(f), ToString(rvalue))
				return rvalue
			}
			panic(fmt.Sprintf("field index not a number: %q", index))
		default:
			panic(fmt.Sprintf("unexpected lvalue type: %T", e.Left))
		}
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
		p.convertFormat = ToString(value)
	case "FILENAME":
		p.filename = ToString(value)
	case "FNR":
		p.fileLineNum = ToFloat(value)
	case "FS":
		p.fieldSep = ToString(value)
	case "NF":
		p.numFields = ToFloat(value)
	case "NR":
		p.lineNum = ToFloat(value)
	case "OFMT":
		p.outputFormat = ToString(value)
	case "OFS":
		p.outputFieldSep = ToString(value)
	case "ORS":
		p.outputRecordSep = ToString(value)
	case "RLENGTH":
		p.matchLength = ToFloat(value)
	case "RS":
		panic("assigning RS not supported")
	case "RSTART":
		p.matchStart = ToFloat(value)
	case "SUBSEP":
		p.subscriptSep = ToString(value)
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

type binaryFunc func(l, r Value) Value

var binaryFuncs = map[string]binaryFunc{
	"==": equal,
	"!=": func(l, r Value) Value {
		return not(equal(l, r))
	},
	"+": func(l, r Value) Value {
		return ToFloat(l) + ToFloat(r)
	},
	"-": func(l, r Value) Value {
		return ToFloat(l) + ToFloat(r)
	},
	"*": func(l, r Value) Value {
		return ToFloat(l) * ToFloat(r)
	},
	"^": func(l, r Value) Value {
		return math.Pow(ToFloat(l), ToFloat(r))
	},
	"/": func(l, r Value) Value {
		rf := ToFloat(r)
		if rf == 0.0 {
			panic("division by zero")
		}
		return ToFloat(l) / rf
	},
	"%": func(l, r Value) Value {
		rf := ToFloat(r)
		if rf == 0.0 {
			panic("division by zero in mod")
		}
		// TODO: integer/float handling?
		return int(ToFloat(l)) % int(rf)
	},
	"": func(l, r Value) Value {
		return ToString(l) + ToString(r)
	},
}

func equal(l, r Value) Value {
	switch l := l.(type) {
	case float64:
		switch r := r.(type) {
		case float64:
			return BoolValue(l == r)
		case string:
			return BoolValue(l == ToFloat(r))
		}
	case string:
		switch r := r.(type) {
		case string:
			return BoolValue(l == r)
		case float64:
			return BoolValue(ToFloat(l) == r)
		}
	}
	// TODO: uninitialized value (nil)
	return 0.0
}

func not(v Value) Value {
	return BoolValue(!ToBool(v))
}
