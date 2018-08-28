// Package interp is the GoAWK interpreter (a simple tree-walker).
//
// For basic usage, use the top-level Exec function. For more
// complicated use cases and configuration options, use New to create
// an interpreter and then call Interp.Exec to execute a whole
// program or Interp.EvalNum or Interp.EvalStr to evaluate a
// stand-alone expression.
//
package interp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	. "github.com/benhoyt/goawk/lexer"
	. "github.com/benhoyt/goawk/parser"
)

var (
	errExit     = errors.New("exit")
	errBreak    = errors.New("break")
	errContinue = errors.New("continue")
	errNext     = errors.New("next")

	crlfNewline = runtime.GOOS == "windows"
	varRegex    = regexp.MustCompile(`^([_a-zA-Z][_a-zA-Z0-9]*)=(.*)`)
)

// Error (actually *Error) is returned by Exec and Eval functions on
// interpreter error, for example a negative field index.
type Error struct {
	message string
}

func (e *Error) Error() string {
	return e.message
}

func newError(format string, args ...interface{}) error {
	return &Error{fmt.Sprintf(format, args...)}
}

type returnValue struct {
	Value value
}

func (r returnValue) Error() string {
	return "<return " + r.Value.str("%.6g") + ">"
}

// Interp holds the state of the GoAWK interpreter. Call New to
// actually create an Interp.
type Interp struct {
	program     *Program
	output      io.Writer
	flushOutput bool
	errorOutput io.Writer
	flushError  bool
	vars        map[string]value
	arrays      map[string]map[string]value
	argc        int
	random      *rand.Rand
	randSeed    float64
	exitStatus  int
	streams     map[string]io.Closer
	commands    map[string]*exec.Cmd
	regexCache  map[string]*regexp.Regexp

	scanner       *bufio.Scanner
	scanners      map[string]*bufio.Scanner
	stdin         io.Reader
	filenameIndex int
	hadFiles      bool
	input         io.Reader

	locals        []map[string]value
	localArrays   []map[string]string
	nilLocalArray int

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
	recordSep       string
	outputFieldSep  string
	outputRecordSep string
	subscriptSep    string
	matchLength     int
	matchStart      int
}

const maxCachedRegexes = 100

// New creates and sets up a new interpeter and sets the output and
// error output writers to the given values (if nil, they're set to
// buffered versions of os.Stdout and os.Stderr, respectively).
func New(output, errorOutput io.Writer) *Interp {
	p := &Interp{}

	if output == nil {
		output = bufio.NewWriterSize(os.Stdout, 64*1024)
		p.flushOutput = true
	}
	p.output = output
	if errorOutput == nil {
		errorOutput = bufio.NewWriterSize(os.Stderr, 64*1024)
		p.flushError = true
	}
	p.errorOutput = errorOutput

	p.vars = make(map[string]value)
	p.arrays = make(map[string]map[string]value)
	p.regexCache = make(map[string]*regexp.Regexp, 10)
	p.randSeed = 1.0
	seed := math.Float64bits(p.randSeed)
	p.random = rand.New(rand.NewSource(int64(seed)))
	p.convertFormat = "%.6g"
	p.outputFormat = "%.6g"
	p.fieldSep = " "
	p.recordSep = "\n"
	p.outputFieldSep = " "
	p.outputRecordSep = "\n"
	p.subscriptSep = "\x1c"
	return p
}

// ExitStatus returns the exit status code of the program (call after
// calling Exec).
func (p *Interp) ExitStatus() int {
	return p.exitStatus
}

// Exec executes the given program using the given input reader (nil
// means os.Stdin) and input arguments (usually filenames: empty
// slice means read only from stdin, and a filename of "-" means read
// stdin instead of a real file).
func (p *Interp) Exec(program *Program, stdin io.Reader, args []string) error {
	p.program = program
	if stdin == nil {
		stdin = os.Stdin
	}
	p.stdin = stdin
	p.argc = len(args) + 1
	for i, arg := range args {
		p.setArray("ARGV", strconv.Itoa(i+1), str(arg))
	}
	p.filenameIndex = 1
	p.hadFiles = false
	p.streams = make(map[string]io.Closer)
	p.commands = make(map[string]*exec.Cmd)
	p.scanners = make(map[string]*bufio.Scanner)
	defer p.closeAll()

	err := p.execBegin(p.program.Begin)
	if err != nil && err != errExit {
		return err
	}
	if p.program.Actions == nil && p.program.End == nil {
		return nil
	}
	if err != errExit {
		err = p.execActions(p.program.Actions)
		if err != nil && err != errExit {
			return err
		}
	}
	err = p.execEnd(p.program.End)
	if err != nil && err != errExit {
		return err
	}
	return nil
}

// Exec provides a simple way to parse and execute an AWK program
// with the given field separator. Exec reads input from the given
// reader (nil means use os.Stdin) and writes output to stdout (nil
// means use os.Stdout).
func Exec(src, fieldSep string, input io.Reader, output io.Writer) error {
	prog, err := ParseProgram([]byte(src))
	if err != nil {
		return err
	}
	p := New(output, ioutil.Discard)
	return p.Exec(prog, input, nil)
}

func (p *Interp) closeAll() {
	if prevInput, ok := p.input.(io.Closer); ok {
		prevInput.Close()
	}
	for _, w := range p.streams {
		_ = w.Close()
	}
	for _, cmd := range p.commands {
		_ = cmd.Wait()
	}
	if p.flushOutput {
		p.output.(*bufio.Writer).Flush()
	}
	if p.flushError {
		p.errorOutput.(*bufio.Writer).Flush()
	}
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

func (p *Interp) execActions(actions []Action) error {
	inRange := make([]bool, len(actions))
lineLoop:
	for {
		line, err := p.nextLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		p.setLine(line)
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
				writeOutput(p.output, p.line)
				writeOutput(p.output, p.outputRecordSep)
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
			if r == errExit {
				execErr = errExit
				return
			}
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
		output := p.getOutputStream(s.Redirect, s.Dest)
		writeOutput(output, line)
		writeOutput(output, p.outputRecordSep)
	case *PrintfStmt:
		if len(s.Args) == 0 {
			break
		}
		format := p.toString(p.eval(s.Args[0]))
		args := make([]value, len(s.Args)-1)
		for i, a := range s.Args[1:] {
			args[i] = p.eval(a)
		}
		output := p.getOutputStream(s.Redirect, s.Dest)
		writeOutput(output, p.sprintf(format, args))
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
		for index := range p.arrays[p.getArrayName(s.Array)] {
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
		if s.Status != nil {
			p.exitStatus = int(p.eval(s.Status).num())
		}
		return errExit
	case *ReturnStmt:
		var value value
		if s.Value != nil {
			value = p.eval(s.Value)
		}
		return returnValue{value}
	case *DeleteStmt:
		index := p.evalIndex(s.Index)
		delete(p.arrays[p.getArrayName(s.Array)], index)
	case *ExprStmt:
		p.eval(s.Expr)
	default:
		panic(fmt.Sprintf("unexpected stmt type: %T", stmt))
	}
	return nil
}

func (p *Interp) getOutputStream(redirect Token, dest Expr) io.Writer {
	if redirect == ILLEGAL {
		return p.output
	}
	name := p.toString(p.eval(dest))
	if s, ok := p.streams[name]; ok {
		if w, ok := s.(io.Writer); ok {
			return w
		}
		panic(newError("can't write to reader stream"))
	}
	switch redirect {
	case GREATER, APPEND:
		flags := os.O_CREATE | os.O_WRONLY
		if redirect == GREATER {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_APPEND
		}
		w, err := os.OpenFile(name, flags, 0644)
		if err != nil {
			panic(newError("output redirection error: %s", err))
		}
		p.streams[name] = w
		return w
	case PIPE:
		cmd := exec.Command("sh", "-c", name)
		w, err := cmd.StdinPipe()
		if err != nil {
			panic(newError("error connecting to stdin pipe: %v", err))
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			panic(newError("error connecting to stdout pipe: %v", err))
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			panic(newError("error connecting to stderr pipe: %v", err))
		}
		err = cmd.Start()
		if err != nil {
			fmt.Fprintln(p.errorOutput, err)
			return ioutil.Discard
		}
		go func() {
			io.Copy(p.output, stdout)
		}()
		go func() {
			io.Copy(p.errorOutput, stderr)
		}()
		p.commands[name] = cmd
		p.streams[name] = w
		return w
	default:
		panic(fmt.Sprintf("unexpected redirect type %s", redirect))
	}
}

func (p *Interp) getInputScanner(name string, isFile bool) *bufio.Scanner {
	if s, ok := p.streams[name]; ok {
		if _, ok := s.(io.Reader); ok {
			return p.scanners[name]
		}
		panic(newError("can't read from writer stream"))
	}
	if isFile {
		r, err := os.Open(name)
		if err != nil {
			panic(newError("input redirection error: %s", err))
		}
		scanner := p.newScanner(r)
		p.scanners[name] = scanner
		p.streams[name] = r
		return scanner
	} else {
		cmd := exec.Command("sh", "-c", name)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			panic(newError("error connecting to stdin pipe: %v", err))
		}
		r, err := cmd.StdoutPipe()
		if err != nil {
			panic(newError("error connecting to stdout pipe: %v", err))
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			panic(newError("error connecting to stderr pipe: %v", err))
		}
		err = cmd.Start()
		if err != nil {
			fmt.Fprintln(p.errorOutput, err)
			return bufio.NewScanner(strings.NewReader(""))
		}
		go func() {
			io.Copy(stdin, p.stdin)
			stdin.Close()
		}()
		go func() {
			io.Copy(p.errorOutput, stderr)
		}()
		scanner := p.newScanner(r)
		p.commands[name] = cmd
		p.streams[name] = r
		p.scanners[name] = scanner
		return scanner
	}
}

func (p *Interp) newScanner(input io.Reader) *bufio.Scanner {
	switch p.recordSep {
	case "\n":
		// Scanner default is to split on newlines
		return bufio.NewScanner(input)
	case "":
		// Empty string for RS means split on newline and skip blank lines
		scanner := bufio.NewScanner(input)
		scanner.Split(scanLinesSkipBlank)
		return scanner
	default:
		scanner := bufio.NewScanner(input)
		splitter := byteSplitter{p.recordSep[0]}
		scanner.Split(splitter.scan)
		return scanner
	}
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

// Copied from bufio/scan.go in the standard library
func scanLinesSkipBlank(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// Skip additional newlines
		j := i + 1
		for j < len(data) && (data[j] == '\n' || data[j] == '\r') {
			j++
		}
		// We have a full newline-terminated line.
		return j, dropCR(data[0:i]), nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Request more data.
	return 0, nil, nil
}

type byteSplitter struct {
	sep byte
}

func (s byteSplitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, s.sep); i >= 0 {
		// We have a full sep-terminated record
		return i + 1, data[0:i], nil
	}
	// If at EOF, we have a final, non-terminated record; return it
	if atEOF {
		return len(data), data, nil
	}
	// Request more data
	return 0, nil, nil
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

// EvalStr evaluates the given expression and returns the result as a
// string (or a *Error on error).
func (p *Interp) EvalStr(expr Expr) (string, error) {
	v, err := p.evalSafe(expr)
	if err != nil {
		return "", err
	}
	return p.toString(v), nil
}

// EvalNum evaluates the given expression and returns the result as a
// number (or a *Error on error).
func (p *Interp) EvalNum(expr Expr) (float64, error) {
	v, err := p.evalSafe(expr)
	if err != nil {
		return 0, err
	}
	return v.num(), nil
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
		default:
			right := p.eval(e.Right)
			return binaryFuncs[e.Op](p, left, right)
		}
	case *InExpr:
		index := p.evalIndex(e.Index)
		_, ok := p.arrays[p.getArrayName(e.Array)][index]
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
		indexNum, err := index.numChecked()
		if err != nil {
			panic(newError("field index not a number: %q", p.toString(index)))
		}
		return p.getField(int(indexNum))
	case *VarExpr:
		return p.getVar(e.Name)
	case *IndexExpr:
		index := p.evalIndex(e.Index)
		return p.getArray(e.Name, index)
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
	case *UserCallExpr:
		return p.userCall(e.Name, e.Args)
	case *MultiExpr:
		// Note: should figure out a good way to make this a parse-time error
		panic(newError("unexpected comma-separated expression: %s", expr))
	case *GetlineExpr:
		var line string
		switch {
		case e.Command != nil:
			name := p.toString(p.eval(e.Command))
			scanner := p.getInputScanner(name, false)
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return num(-1)
				}
				return num(0)
			}
			line = scanner.Text()
		case e.File != nil:
			name := p.toString(p.eval(e.File))
			scanner := p.getInputScanner(name, true)
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return num(-1)
				}
				return num(0)
			}
			line = scanner.Text()
		default:
			var err error
			line, err = p.nextLine()
			if err == io.EOF {
				return num(0)
			}
			if err != nil {
				return num(-1)
			}
		}
		if e.Var != "" {
			p.setVar(e.Var, str(line))
		} else {
			p.setLine(line)
		}
		return num(1)
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
	} else if line == "" {
		p.fields = nil
	} else {
		p.fields = p.fieldSepRegex.Split(line, -1)
	}
	p.numFields = len(p.fields)
}

func (p *Interp) nextLine() (string, error) {
	for {
		if p.scanner == nil {
			if prevInput, ok := p.input.(io.Closer); ok && p.input != p.stdin {
				prevInput.Close()
			}
			if p.filenameIndex >= p.argc && !p.hadFiles {
				p.input = p.stdin
				p.setFile("")
				p.hadFiles = true
			} else {
				if p.filenameIndex >= p.argc {
					return "", io.EOF
				}
				index := strconv.Itoa(p.filenameIndex)
				filename := p.toString(p.getArray("ARGV", index))
				p.filenameIndex++
				matches := varRegex.FindStringSubmatch(filename)
				if len(matches) >= 3 {
					p.setVar(matches[1], numStr(matches[2]))
					continue
				} else if filename == "" {
					p.input = nil
					continue
				} else if filename == "-" {
					p.input = p.stdin
					p.setFile("")
				} else {
					input, err := os.Open(filename)
					if err != nil {
						return "", err
					}
					p.input = input
					p.setFile(filename)
					p.hadFiles = true
				}
			}
			p.scanner = p.newScanner(p.input)
		}
		if p.scanner.Scan() {
			break
		}
		if err := p.scanner.Err(); err != nil {
			return "", fmt.Errorf("error reading from input: %s", err)
		}
		p.scanner = nil
	}
	p.lineNum++
	p.fileLineNum++
	return p.scanner.Text(), nil
}

func (p *Interp) getVar(name string) value {
	if len(p.locals) > 0 {
		v, ok := p.locals[len(p.locals)-1][name]
		if ok {
			return v
		}
	}
	switch name {
	case "ARGC":
		return num(float64(p.argc))
	case "CONVFMT":
		return str(p.convertFormat)
	case "FILENAME":
		return str(p.filename)
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
		return str(p.recordSep)
	case "RSTART":
		return num(float64(p.matchStart))
	case "SUBSEP":
		return str(p.subscriptSep)
	default:
		return p.vars[name]
	}
}

// SetVar sets the named variable to value (useful for setting FS and
// other built-in variables before calling Exec).
func (p *Interp) SetVar(name string, value string) error {
	return p.setVarError(name, str(value))
}

func (p *Interp) setVarError(name string, v value) error {
	if len(p.locals) > 0 {
		_, ok := p.locals[len(p.locals)-1][name]
		if ok {
			p.locals[len(p.locals)-1][name] = v
			return nil
		}
	}

	switch name {
	case "ARGC":
		p.argc = int(v.num())
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
		numFields := int(v.num())
		if numFields < 0 {
			return newError("NF set to negative value: %d", numFields)
		}
		p.numFields = numFields
		if p.numFields < len(p.fields) {
			p.fields = p.fields[:p.numFields]
		}
		for i := len(p.fields); i < p.numFields; i++ {
			p.fields = append(p.fields, "")
		}
		p.line = strings.Join(p.fields, p.outputFieldSep)
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
		sep := p.toString(v)
		if len(sep) > 1 {
			return newError("RS must be at most 1 char")
		}
		p.recordSep = sep
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

func (p *Interp) getArrayName(name string) string {
	if len(p.localArrays) > 0 {
		n, ok := p.localArrays[len(p.localArrays)-1][name]
		if ok {
			return n
		}
	}
	return name
}

func (p *Interp) getArray(name, index string) value {
	return p.arrays[p.getArrayName(name)][index]
}

func (p *Interp) setArray(name, index string, v value) {
	name = p.getArrayName(name)
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
		return str("")
	}
	return numStr(p.fields[index-1])
}

// SetField sets a single field, equivalent to "$index = value".
func (p *Interp) SetField(index int, value string) {
	if index < 0 {
		panic(newError("field index negative: %d", index))
	}
	if index == 0 {
		p.setLine(value)
		return
	}
	for i := len(p.fields); i < index; i++ {
		p.fields = append(p.fields, "")
	}
	p.fields[index-1] = value
	p.numFields = len(p.fields)
	p.line = strings.Join(p.fields, p.outputFieldSep)
}

func (p *Interp) SetArgv0(argv0 string) {
	p.setArray("ARGV", "0", str(argv0))
}

// SetArgs sets the command-line arguments for the ARGV array
// accessible to the AWK program. This function is DEPRECATED and
// does nothing now that Exec sets ARGV.
func (p *Interp) SetArgs(args []string) {}

func (p *Interp) toString(v value) string {
	return v.str(p.convertFormat)
}

func (p *Interp) mustCompile(regex string) *regexp.Regexp {
	if re, ok := p.regexCache[regex]; ok {
		return re
	}
	re, err := regexp.Compile(regex)
	if err != nil {
		panic(newError("invalid regex %q: %s", regex, err))
	}
	// Dumb, non-LRU cache: just cache the first N regexes
	if len(p.regexCache) < maxCachedRegexes {
		p.regexCache[regex] = re
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
	case F_CLOSE:
		name := p.toString(args[0])
		w, ok := p.streams[name]
		if !ok {
			return num(-1)
		}
		err := w.Close()
		if err != nil {
			return num(-1)
		}
		return num(0)
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
		prevSeed := p.randSeed
		switch len(args) {
		case 0:
			p.random.Seed(time.Now().UnixNano())
		case 1:
			p.randSeed = args[0].num()
			p.random.Seed(int64(math.Float64bits(p.randSeed)))
		}
		return num(prevSeed)
	case F_SUBSTR:
		s := p.toString(args[0])
		pos := int(args[1].num())
		if pos > len(s) {
			pos = len(s) + 1
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
	case F_SYSTEM:
		cmdline := p.toString(args[0])
		cmd := exec.Command("sh", "-c", cmdline)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return num(-1)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return num(-1)
		}
		err = cmd.Start()
		if err != nil {
			fmt.Fprintln(p.errorOutput, err)
			return num(-1)
		}
		go func() {
			io.Copy(p.output, stdout)
		}()
		go func() {
			io.Copy(p.errorOutput, stderr)
		}()
		err = cmd.Wait()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					return num(float64(status.ExitStatus()))
				} else {
					fmt.Fprintf(p.errorOutput, "couldn't get exit status for %q: %v\n", cmdline, err)
					return num(-1)
				}
			} else {
				fmt.Fprintf(p.errorOutput, "unexpected error running command %q: %v\n", cmdline, err)
				return num(-1)
			}
		}
		return num(0)
	default:
		panic(fmt.Sprintf("unexpected function: %s", op))
	}
}

func (p *Interp) userCall(name string, args []Expr) value {
	f, ok := p.program.Functions[name]
	if !ok {
		panic(newError("undefined function %q", name))
	}
	if len(args) > len(f.Params) {
		panic(newError("%q called with more arguments than declared", name))
	}

	locals := make(map[string]value)
	arrays := make(map[string]string)
	for i, arg := range args {
		if f.Arrays[i] {
			a, ok := arg.(*VarExpr)
			if !ok {
				panic(newError("%s() argument %q must be an array", name, f.Params[i]))
			}
			arrays[f.Params[i]] = a.Name
		} else {
			locals[f.Params[i]] = p.eval(arg)
		}
	}
	for i := len(args); i < len(f.Params); i++ {
		if f.Arrays[i] {
			arrays[f.Params[i]] = "__nla" + strconv.Itoa(p.nilLocalArray)
			p.nilLocalArray++
		} else {
			locals[f.Params[i]] = value{}
		}
	}
	p.locals = append(p.locals, locals)
	p.localArrays = append(p.localArrays, arrays)

	err := p.executes(f.Body)

	p.locals = p.locals[:len(p.locals)-1]
	for i := len(args); i < len(f.Params); i++ {
		if f.Arrays[i] {
			p.nilLocalArray--
			delete(p.arrays, "__nla"+strconv.Itoa(p.nilLocalArray))
		}
	}
	p.localArrays = p.localArrays[:len(p.localArrays)-1]

	if r, ok := err.(returnValue); ok {
		return r.Value
	}
	if err != nil {
		panic(err)
	}
	return value{}
}

func (p *Interp) split(s, arrayName, fs string) int {
	var parts []string
	if fs == " " {
		parts = strings.Fields(s)
	} else if s != "" {
		re := p.mustCompile(fs)
		parts = re.Split(s, -1)
	}
	array := make(map[string]value)
	for i, part := range parts {
		array[strconv.Itoa(i+1)] = numStr(part)
	}
	p.arrays[p.getArrayName(arrayName)] = array
	return len(array)
}

func (p *Interp) sub(regex, repl, in string, global bool) (out string, num int) {
	re := p.mustCompile(regex)
	count := 0
	out = re.ReplaceAllStringFunc(in, func(s string) string {
		if !global && count > 0 {
			return s
		}
		count++
		// Handle & (ampersand) properly in replacement string
		r := make([]byte, 0, len(repl))
		for i := 0; i < len(repl); i++ {
			switch repl[i] {
			case '&':
				r = append(r, s...)
			case '\\':
				i++
				if i < len(repl) {
					switch repl[i] {
					case '&':
						r = append(r, repl[i])
					default:
						r = append(r, '\\', repl[i])
					}
				} else {
					r = append(r, '\\')
				}
			default:
				r = append(r, repl[i])
			}
		}
		return string(r)
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
	if len(types) > len(args) {
		panic(newError("format error: got %d args, expected %d", len(args), len(types)))
	}
	converted := make([]interface{}, len(types))
	for i, t := range types {
		a := args[i]
		var v interface{}
		switch t {
		case 'd':
			v = int(a.num())
		case 'u':
			v = uint32(a.num())
		case 'c':
			c := make([]byte, 0, 4)
			if a.isTrueStr() {
				s := p.toString(a)
				if len(s) > 0 {
					c = []byte{s[0]}
				} else {
					c = []byte{0}
				}
			} else {
				r := []rune{rune(a.num())}
				c = []byte(string(r))
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
		index := p.evalIndex(left.Index)
		p.setArray(left.Name, index, right)
	case *FieldExpr:
		index := p.eval(left.Index)
		indexNum, err := index.numChecked()
		if err != nil {
			panic(newError("field index not a number: %q", p.toString(index)))
		}
		p.SetField(int(indexNum), p.toString(right))
	default:
		panic(fmt.Sprintf("unexpected lvalue type: %T", left))
	}
}

func (p *Interp) evalIndex(indexExprs []Expr) string {
	indices := make([]string, len(indexExprs))
	for i, expr := range indexExprs {
		indices[i] = p.toString(p.eval(expr))
	}
	return strings.Join(indices, p.subscriptSep)
}

func writeOutput(w io.Writer, s string) {
	if crlfNewline {
		// First normalize to \n, then convert all newlines to \r\n (on Windows)
		s = strings.Replace(s, "\r\n", "\n", -1)
		s = strings.Replace(s, "\n", "\r\n", -1)
	}
	io.WriteString(w, s)
}
