// Package interp is the GoAWK interpreter.
//
// For basic usage, use the Exec function. For more complicated use
// cases and configuration options, first use the parser package to
// parse the AWK source, and then use ExecProgram to execute it with
// a specific configuration.
//
// If you need to re-run the same parsed program repeatedly on different
// inputs or with different variables, use New to instantiate an Interpreter
// and then call the Interpreter.Execute method as many times as you need.
package interp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/benhoyt/goawk/internal/ast"
	"github.com/benhoyt/goawk/internal/compiler"
	"github.com/benhoyt/goawk/internal/resolver"
	"github.com/benhoyt/goawk/parser"
)

var (
	errExit     = errors.New("exit")
	errBreak    = errors.New("break")
	errNext     = errors.New("next")
	errNextfile = errors.New("nextfile")

	errCSVSeparator = errors.New("invalid CSV field separator or comment delimiter")

	crlfNewline = runtime.GOOS == "windows"
	varRegex    = regexp.MustCompile(`^([_a-zA-Z][_a-zA-Z0-9]*)=(.*)`)

	defaultShellCommand = getDefaultShellCommand()
)

// Error (actually *Error) is returned by Exec and Eval functions on
// interpreter error, for example FS being set to an invalid regex.
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

type interp struct {
	// Input/output
	output        io.Writer
	errorOutput   io.Writer
	scanner       *bufio.Scanner
	scanners      map[string]*bufio.Scanner
	stdin         io.Reader
	filenameIndex int
	hadFiles      bool
	input         io.Reader
	inputBuffer   []byte
	inputStreams  map[string]inputStream
	outputStreams map[string]outputStream
	noExec        bool
	noFileWrites  bool
	noFileReads   bool
	shellCommand  []string
	csvOutput     *bufio.Writer
	noArgVars     bool
	splitBuffer   []byte

	// Scalars, arrays, and function state
	globals       []value
	stack         []value
	sp            int
	frame         []value
	arrays        []map[string]value
	localArrays   [][]int
	callDepth     int
	nativeFuncs   []nativeFunc
	scalarIndexes map[string]int
	arrayIndexes  map[string]int

	// File, line, and field handling
	filename        value
	line            string
	lineIsTrueStr   bool
	lineNum         int
	fileLineNum     int
	fields          []string
	fieldsIsTrueStr []bool
	numFields       int
	haveFields      bool
	fieldNames      []string
	fieldIndexes    map[string]int
	reparseCSV      bool

	// Built-in variables
	argc             int
	convertFormat    string
	outputFormat     string
	fieldSep         string
	fieldSepRegex    *regexp.Regexp
	recordSep        string
	recordSepRegex   *regexp.Regexp
	recordTerminator string
	outputFieldSep   string
	outputRecordSep  string
	subscriptSep     string
	matchLength      int
	matchStart       int
	inputMode        IOMode
	csvInputConfig   CSVInputConfig
	outputMode       IOMode
	csvOutputConfig  CSVOutputConfig

	// Parsed program, compiled functions and constants
	program   *parser.Program
	functions []compiler.Function
	nums      []float64
	strs      []string
	regexes   []*regexp.Regexp

	// Context support (for Interpreter.ExecuteContext)
	checkCtx bool
	ctx      context.Context
	ctxDone  <-chan struct{}
	ctxOps   int

	// Misc pieces of state
	random           *rand.Rand
	randSeed         float64
	exitStatus       int
	regexCache       map[string]*regexp.Regexp
	formatCache      map[string]cachedFormat
	csvJoinFieldsBuf bytes.Buffer
}

// Various const configuration. Could make these part of Config if
// we wanted to, but no need for now.
const (
	maxCachedRegexes = 100
	maxCachedFormats = 100
	maxRecordLength  = 10 * 1024 * 1024 // 10MB seems like plenty
	maxFieldIndex    = 1000000
	maxCallDepth     = 1000
	initialStackSize = 100
	outputBufSize    = 64 * 1024
	inputBufSize     = 64 * 1024
)

// Config defines the interpreter configuration for ExecProgram.
type Config struct {
	// Standard input reader (defaults to os.Stdin)
	Stdin io.Reader

	// Writer for normal output (defaults to a buffered version of os.Stdout).
	// If you need to write to stdout but want control over the buffer size or
	// allocation, wrap os.Stdout yourself and set Output to that.
	Output io.Writer

	// Writer for non-fatal error messages (defaults to os.Stderr)
	Error io.Writer

	// The name of the executable (accessible via ARGV[0])
	Argv0 string

	// Input arguments (usually filenames): empty slice means read
	// only from Stdin, and a filename of "-" means read from Stdin
	// instead of a real file.
	//
	// Arguments of the form "var=value" are treated as variable
	// assignments.
	Args []string

	// Set to true to disable "var=value" assignments in Args.
	NoArgVars bool

	// List of name-value pairs for variables to set before executing
	// the program (useful for setting FS and other built-in
	// variables, for example []string{"FS", ",", "OFS", ","}).
	Vars []string

	// Map of named Go functions to allow calling from AWK. You need
	// to pass this same map to the parser.ParseProgram config.
	//
	// Functions can have any number of parameters, and variadic
	// functions are supported. Functions can have no return values,
	// one return value, or two return values (result, error). In the
	// two-value case, if the function returns a non-nil error,
	// program execution will stop and ExecProgram will return that
	// error.
	//
	// Apart from the error return value, the types supported are
	// bool, integer and floating point types (excluding complex),
	// and string types (string or []byte).
	//
	// It's not an error to call a Go function from AWK with fewer
	// arguments than it has parameters in Go. In this case, the zero
	// value will be used for any additional parameters. However, it
	// is a parse error to call a non-variadic function from AWK with
	// more arguments than it has parameters in Go.
	//
	// Functions defined with the "function" keyword in AWK code
	// take precedence over functions in Funcs.
	Funcs map[string]interface{}

	// Set one or more of these to true to prevent unsafe behaviours,
	// useful when executing untrusted scripts:
	//
	// * NoExec prevents system calls via system() or pipe operator
	// * NoFileWrites prevents writing to files via '>' or '>>'
	// * NoFileReads prevents reading from files via getline or the
	//   filenames in Args
	NoExec       bool
	NoFileWrites bool
	NoFileReads  bool

	// Exec args used to run system shell. Typically, this will
	// be {"/bin/sh", "-c"}
	ShellCommand []string

	// List of name-value pairs to be assigned to the ENVIRON special
	// array, for example []string{"USER", "bob", "HOME", "/home/bob"}.
	// If nil (the default), values from os.Environ() are used.
	//
	// If the script doesn't need environment variables, set Environ to a
	// non-nil empty slice, []string{}.
	Environ []string

	// Mode for parsing input fields and record: default is to use normal FS
	// and RS behaviour. If set to CSVMode or TSVMode, FS and RS are ignored,
	// and input records are parsed as comma-separated values or tab-separated
	// values, respectively. Parsing is done as per RFC 4180 and the
	// "encoding/csv" package, but FieldsPerRecord is not supported,
	// LazyQuotes is always on, and TrimLeadingSpace is always off.
	//
	// You can also enable CSV or TSV input mode by setting INPUTMODE to "csv"
	// or "tsv" in Vars or in the BEGIN block (those override this setting).
	//
	// For further documentation about GoAWK's CSV support, see the full docs
	// in "../docs/csv.md".
	InputMode IOMode

	// Additional options if InputMode is CSVMode or TSVMode. The zero value
	// is valid, specifying a separator of ',' in CSVMode and '\t' in TSVMode.
	//
	// You can also specify these options by setting INPUTMODE in the BEGIN
	// block, for example, to use '|' as the field separator, '#' as the
	// comment character, and enable header row parsing:
	//
	//     BEGIN { INPUTMODE="csv separator=| comment=# header" }
	CSVInput CSVInputConfig

	// Mode for print output: default is to use normal OFS and ORS
	// behaviour. If set to CSVMode or TSVMode, the "print" statement with one
	// or more arguments outputs fields using CSV or TSV formatting,
	// respectively. Output is written as per RFC 4180 and the "encoding/csv"
	// package.
	//
	// You can also enable CSV or TSV output mode by setting OUTPUTMODE to
	// "csv" or "tsv" in Vars or in the BEGIN block (those override this
	// setting).
	OutputMode IOMode

	// Additional options if OutputMode is CSVMode or TSVMode. The zero value
	// is valid, specifying a separator of ',' in CSVMode and '\t' in TSVMode.
	//
	// You can also specify these options by setting OUTPUTMODE in the BEGIN
	// block, for example, to use '|' as the output field separator:
	//
	//     BEGIN { OUTPUTMODE="csv separator=|" }
	CSVOutput CSVOutputConfig
}

// IOMode specifies the input parsing or print output mode.
type IOMode int

const (
	// DefaultMode uses normal AWK field and record separators: FS and RS for
	// input, OFS and ORS for print output.
	DefaultMode IOMode = 0

	// CSVMode uses comma-separated value mode for input or output.
	CSVMode IOMode = 1

	// TSVMode uses tab-separated value mode for input or output.
	TSVMode IOMode = 2
)

// CSVInputConfig holds additional configuration for when InputMode is CSVMode
// or TSVMode.
type CSVInputConfig struct {
	// Input field separator character. If this is zero, it defaults to ','
	// when InputMode is CSVMode and '\t' when InputMode is TSVMode.
	Separator rune

	// If nonzero, specifies that lines beginning with this character (and no
	// leading whitespace) should be ignored as comments.
	Comment rune

	// If true, parse the first row in each input file as a header row (that
	// is, a list of field names), and enable the @"field" syntax to get a
	// field by name as well as the FIELDS special array.
	Header bool
}

// CSVOutputConfig holds additional configuration for when OutputMode is
// CSVMode or TSVMode.
type CSVOutputConfig struct {
	// Output field separator character. If this is zero, it defaults to ','
	// when OutputMode is CSVMode and '\t' when OutputMode is TSVMode.
	Separator rune
}

// ExecProgram executes the parsed program using the given interpreter
// config, returning the exit status code of the program. Error is nil
// on successful execution of the program, even if the program returns
// a non-zero status code.
//
// As of GoAWK version v1.16.0, a nil config is valid and will use the
// defaults (zero values). However, it may be simpler to use Exec in that
// case.
func ExecProgram(program *parser.Program, config *Config) (int, error) {
	p := newInterp(program)
	err := p.setExecuteConfig(config)
	if err != nil {
		return 0, err
	}
	return p.executeAll()
}

func newInterp(program *parser.Program) *interp {
	p := &interp{
		program:   program,
		functions: program.Compiled.Functions,
		nums:      program.Compiled.Nums,
		strs:      program.Compiled.Strs,
		regexes:   program.Compiled.Regexes,
	}

	// Allocate memory for variables and virtual machine stack
	p.scalarIndexes = make(map[string]int)
	p.arrayIndexes = make(map[string]int)
	program.IterVars("", func(name string, info resolver.VarInfo) {
		if info.Type == resolver.Array {
			p.arrayIndexes[name] = info.Index
		} else {
			p.scalarIndexes[name] = info.Index
		}
	})
	p.globals = make([]value, len(p.scalarIndexes))
	p.stack = make([]value, initialStackSize)
	p.arrays = make([]map[string]value, len(p.arrayIndexes), len(p.arrayIndexes)+initialStackSize)
	for i := 0; i < len(p.arrayIndexes); i++ {
		p.arrays[i] = make(map[string]value)
	}

	// Initialize defaults
	p.regexCache = make(map[string]*regexp.Regexp, 10)
	p.formatCache = make(map[string]cachedFormat, 10)
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

	p.inputStreams = make(map[string]inputStream)
	p.outputStreams = make(map[string]outputStream)
	p.scanners = make(map[string]*bufio.Scanner)

	return p
}

func (p *interp) setExecuteConfig(config *Config) error {
	if config == nil {
		config = &Config{}
	}
	if len(config.Vars)%2 != 0 {
		return newError("length of config.Vars must be a multiple of 2, not %d", len(config.Vars))
	}
	if len(config.Environ)%2 != 0 {
		return newError("length of config.Environ must be a multiple of 2, not %d", len(config.Environ))
	}

	// Set up I/O mode config (Vars will override)
	p.inputMode = config.InputMode
	p.csvInputConfig = config.CSVInput
	switch p.inputMode {
	case CSVMode:
		if p.csvInputConfig.Separator == 0 {
			p.csvInputConfig.Separator = ','
		}
	case TSVMode:
		if p.csvInputConfig.Separator == 0 {
			p.csvInputConfig.Separator = '\t'
		}
	case DefaultMode:
		if p.csvInputConfig != (CSVInputConfig{}) {
			return newError("input mode configuration not valid in default input mode")
		}
	}
	p.outputMode = config.OutputMode
	p.csvOutputConfig = config.CSVOutput
	switch p.outputMode {
	case CSVMode:
		if p.csvOutputConfig.Separator == 0 {
			p.csvOutputConfig.Separator = ','
		}
	case TSVMode:
		if p.csvOutputConfig.Separator == 0 {
			p.csvOutputConfig.Separator = '\t'
		}
	case DefaultMode:
		if p.csvOutputConfig != (CSVOutputConfig{}) {
			return newError("output mode configuration not valid in default output mode")
		}
	}

	// Set up ARGV and other variables from config
	argvIndex := p.arrayIndexes["ARGV"]
	p.setArrayValue(resolver.Global, argvIndex, "0", str(config.Argv0))
	p.argc = len(config.Args) + 1
	for i, arg := range config.Args {
		p.setArrayValue(resolver.Global, argvIndex, strconv.Itoa(i+1), numStr(arg))
	}
	p.noArgVars = config.NoArgVars
	p.filenameIndex = 1
	p.hadFiles = false
	for i := 0; i < len(config.Vars); i += 2 {
		err := p.setVarByName(config.Vars[i], config.Vars[i+1])
		if err != nil {
			return err
		}
	}

	// After Vars has been handled, validate CSV configuration.
	err := validateCSVInputConfig(p.inputMode, p.csvInputConfig)
	if err != nil {
		return err
	}
	err = validateCSVOutputConfig(p.outputMode, p.csvOutputConfig)
	if err != nil {
		return err
	}

	// Set up ENVIRON from config or environment variables
	environIndex := p.arrayIndexes["ENVIRON"]
	if config.Environ != nil {
		for i := 0; i < len(config.Environ); i += 2 {
			p.setArrayValue(resolver.Global, environIndex, config.Environ[i], numStr(config.Environ[i+1]))
		}
	} else {
		for _, kv := range os.Environ() {
			eq := strings.IndexByte(kv, '=')
			if eq >= 0 {
				p.setArrayValue(resolver.Global, environIndex, kv[:eq], numStr(kv[eq+1:]))
			}
		}
	}

	// Set up system shell command
	if len(config.ShellCommand) != 0 {
		p.shellCommand = config.ShellCommand
	} else {
		p.shellCommand = defaultShellCommand
	}

	// Set up I/O structures
	p.noExec = config.NoExec
	p.noFileWrites = config.NoFileWrites
	p.noFileReads = config.NoFileReads
	p.stdin = config.Stdin
	if p.stdin == nil {
		p.stdin = os.Stdin
	}
	p.output = config.Output
	if p.output == nil {
		p.output = bufio.NewWriterSize(os.Stdout, outputBufSize)
	}
	p.errorOutput = config.Error
	if p.errorOutput == nil {
		p.errorOutput = os.Stderr
	}

	// Initialize native Go functions
	if p.nativeFuncs == nil {
		err := p.initNativeFuncs(config.Funcs)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateCSVInputConfig(mode IOMode, config CSVInputConfig) error {
	if mode != CSVMode && mode != TSVMode {
		return nil
	}
	if config.Separator == config.Comment || !validCSVSeparator(config.Separator) ||
		config.Comment != 0 && !validCSVSeparator(config.Comment) {
		return errCSVSeparator
	}
	return nil
}

func validateCSVOutputConfig(mode IOMode, config CSVOutputConfig) error {
	if mode != CSVMode && mode != TSVMode {
		return nil
	}
	if !validCSVSeparator(config.Separator) {
		return errCSVSeparator
	}
	return nil
}

func validCSVSeparator(r rune) bool {
	return r != 0 && r != '"' && r != '\r' && r != '\n' && utf8.ValidRune(r) && r != utf8.RuneError
}

func (p *interp) executeAll() (int, error) {
	defer p.closeAll()

	// Execute the program: BEGIN, then pattern/actions, then END
	err := p.execute(p.program.Compiled.Begin)
	if err != nil && err != errExit {
		if p.checkCtx {
			ctxErr := p.checkContextNow()
			if ctxErr != nil {
				return 0, ctxErr
			}
		}
		return 0, err
	}
	if len(p.program.Compiled.Actions) == 0 && len(p.program.Compiled.End) == 0 {
		return p.exitStatus, nil // only BEGIN specified, don't process input
	}
	if err != errExit {
		err = p.execActions(p.program.Compiled.Actions)
		if err != nil && err != errExit {
			if p.checkCtx {
				ctxErr := p.checkContextNow()
				if ctxErr != nil {
					return 0, ctxErr
				}
			}
			return 0, err
		}
	}
	err = p.execute(p.program.Compiled.End)
	if err != nil && err != errExit {
		if p.checkCtx {
			ctxErr := p.checkContextNow()
			if ctxErr != nil {
				return 0, ctxErr
			}
		}
		return 0, err
	}
	return p.exitStatus, nil
}

// Exec provides a simple way to parse and execute an AWK program
// with the given field separator. Exec reads input from the given
// reader (nil means use os.Stdin) and writes output to stdout (nil
// means use a buffered version of os.Stdout).
func Exec(source, fieldSep string, input io.Reader, output io.Writer) error {
	prog, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		return err
	}
	config := &Config{
		Stdin:  input,
		Output: output,
		Error:  io.Discard,
		Vars:   []string{"FS", fieldSep},
	}
	_, err = ExecProgram(prog, config)
	return err
}

// Execute pattern-action blocks (may be multiple)
func (p *interp) execActions(actions []compiler.Action) error {
	var inRange []bool
lineLoop:
	for {
		// Read and setup next line of input
		line, err := p.nextLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		p.setLine(line, false)
		p.reparseCSV = false

		// Execute all the pattern-action blocks for each line
		for i, action := range actions {
			// First determine whether the pattern matches
			matched := false
			switch len(action.Pattern) {
			case 0:
				// No pattern is equivalent to pattern evaluating to true
				matched = true
			case 1:
				// Single boolean pattern
				err := p.execute(action.Pattern[0])
				if err != nil {
					return err
				}
				matched = p.pop().boolean()
			case 2:
				// Range pattern (matches between start and stop lines)
				if inRange == nil {
					inRange = make([]bool, len(actions))
				}
				if !inRange[i] {
					err := p.execute(action.Pattern[0])
					if err != nil {
						return err
					}
					inRange[i] = p.pop().boolean()
				}
				matched = inRange[i]
				if inRange[i] {
					err := p.execute(action.Pattern[1])
					if err != nil {
						return err
					}
					inRange[i] = !p.pop().boolean()
				}
			}
			if !matched {
				continue
			}

			// No action is equivalent to { print $0 }
			if len(action.Body) == 0 {
				err := p.printLine(p.output, p.line)
				if err != nil {
					return err
				}
				continue
			}

			// Execute the body statements
			err := p.execute(action.Body)
			switch {
			case err == errNext:
				// "next" statement skips straight to next line
				continue lineLoop
			case err == errNextfile:
				// Tell nextLine to move on to next file
				p.scanner = nil
				continue lineLoop
			case err != nil:
				return err
			}
		}
	}
	return nil
}

// Get a special variable by index
func (p *interp) getSpecial(index int) value {
	switch index {
	case ast.V_NF:
		p.ensureFields()
		return num(float64(p.numFields))
	case ast.V_NR:
		return num(float64(p.lineNum))
	case ast.V_RLENGTH:
		return num(float64(p.matchLength))
	case ast.V_RSTART:
		return num(float64(p.matchStart))
	case ast.V_FNR:
		return num(float64(p.fileLineNum))
	case ast.V_ARGC:
		return num(float64(p.argc))
	case ast.V_CONVFMT:
		return str(p.convertFormat)
	case ast.V_FILENAME:
		return p.filename
	case ast.V_FS:
		return str(p.fieldSep)
	case ast.V_OFMT:
		return str(p.outputFormat)
	case ast.V_OFS:
		return str(p.outputFieldSep)
	case ast.V_ORS:
		return str(p.outputRecordSep)
	case ast.V_RS:
		return str(p.recordSep)
	case ast.V_RT:
		return str(p.recordTerminator)
	case ast.V_SUBSEP:
		return str(p.subscriptSep)
	case ast.V_INPUTMODE:
		return str(inputModeString(p.inputMode, p.csvInputConfig))
	case ast.V_OUTPUTMODE:
		return str(outputModeString(p.outputMode, p.csvOutputConfig))
	default:
		panic(fmt.Sprintf("unexpected special variable index: %d", index))
	}
}

// Set a variable by name (specials and globals only)
func (p *interp) setVarByName(name, value string) error {
	index := ast.SpecialVarIndex(name)
	if index > 0 {
		return p.setSpecial(index, numStr(value))
	}
	index, ok := p.scalarIndexes[name]
	if ok {
		p.globals[index] = numStr(value)
		return nil
	}
	// Ignore variables that aren't defined in program
	return nil
}

// Set special variable by index to given value
func (p *interp) setSpecial(index int, v value) error {
	switch index {
	case ast.V_NF:
		numFields := int(v.num())
		if numFields < 0 {
			return newError("NF set to negative value: %d", numFields)
		}
		if numFields > maxFieldIndex {
			return newError("NF set too large: %d", numFields)
		}
		p.ensureFields()
		p.numFields = numFields
		if p.numFields < len(p.fields) {
			p.fields = p.fields[:p.numFields]
			p.fieldsIsTrueStr = p.fieldsIsTrueStr[:p.numFields]
		}
		for i := len(p.fields); i < p.numFields; i++ {
			p.fields = append(p.fields, "")
			p.fieldsIsTrueStr = append(p.fieldsIsTrueStr, false)
		}
		p.line = p.joinFields(p.fields)
		p.lineIsTrueStr = true
	case ast.V_NR:
		p.lineNum = int(v.num())
	case ast.V_RLENGTH:
		p.matchLength = int(v.num())
	case ast.V_RSTART:
		p.matchStart = int(v.num())
	case ast.V_FNR:
		p.fileLineNum = int(v.num())
	case ast.V_ARGC:
		argc := int(v.num())
		if argc > maxFieldIndex {
			return newError("ARGC set too large: %d", argc)
		}
		p.argc = argc
	case ast.V_CONVFMT:
		p.convertFormat = p.toString(v)
	case ast.V_FILENAME:
		p.filename = v
	case ast.V_FS:
		p.fieldSep = p.toString(v)
		if utf8.RuneCountInString(p.fieldSep) > 1 { // compare to interp.ensureFields
			re, err := regexp.Compile(compiler.AddRegexFlags(p.fieldSep))
			if err != nil {
				return newError("invalid regex %q: %s", p.fieldSep, err)
			}
			p.fieldSepRegex = re
		}
	case ast.V_OFMT:
		p.outputFormat = p.toString(v)
	case ast.V_OFS:
		p.outputFieldSep = p.toString(v)
	case ast.V_ORS:
		p.outputRecordSep = p.toString(v)
	case ast.V_RS:
		p.recordSep = p.toString(v)
		switch { // compare to interp.newScanner
		case len(p.recordSep) <= 1:
			// Simple cases use specialized splitters, not regex
		case utf8.RuneCountInString(p.recordSep) == 1:
			// Multi-byte unicode char falls back to regex splitter
			sep := regexp.QuoteMeta(p.recordSep) // not strictly necessary as no multi-byte chars are regex meta chars
			p.recordSepRegex = regexp.MustCompile(sep)
		default:
			re, err := regexp.Compile(compiler.AddRegexFlags(p.recordSep))
			if err != nil {
				return newError("invalid regex %q: %s", p.recordSep, err)
			}
			p.recordSepRegex = re
		}
	case ast.V_RT:
		p.recordTerminator = p.toString(v)
	case ast.V_SUBSEP:
		p.subscriptSep = p.toString(v)
	case ast.V_INPUTMODE:
		var err error
		p.inputMode, p.csvInputConfig, err = parseInputMode(p.toString(v))
		if err != nil {
			return err
		}
		err = validateCSVInputConfig(p.inputMode, p.csvInputConfig)
		if err != nil {
			return err
		}
	case ast.V_OUTPUTMODE:
		var err error
		p.outputMode, p.csvOutputConfig, err = parseOutputMode(p.toString(v))
		if err != nil {
			return err
		}
		err = validateCSVOutputConfig(p.outputMode, p.csvOutputConfig)
		if err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unexpected special variable index: %d", index))
	}
	return nil
}

// Determine the index of given array into the p.arrays slice. Global
// arrays are just at p.arrays[index], local arrays have to be looked
// up indirectly.
func (p *interp) arrayIndex(scope resolver.Scope, index int) int {
	if scope == resolver.Global {
		return index
	} else {
		return p.localArrays[len(p.localArrays)-1][index]
	}
}

// Return array with given scope and index.
func (p *interp) array(scope resolver.Scope, index int) map[string]value {
	return p.arrays[p.arrayIndex(scope, index)]
}

// Return local array with given index.
func (p *interp) localArray(index int) map[string]value {
	return p.arrays[p.localArrays[len(p.localArrays)-1][index]]
}

// Set a value in given array by key (index)
func (p *interp) setArrayValue(scope resolver.Scope, arrayIndex int, index string, v value) {
	array := p.array(scope, arrayIndex)
	array[index] = v
}

// Get the value of given numbered field, equivalent to "$index"
func (p *interp) getField(index int) value {
	if index == 0 {
		if p.lineIsTrueStr {
			return str(p.line)
		} else {
			return numStr(p.line)
		}
	}
	p.ensureFields()
	if index < 1 {
		index = len(p.fields) + 1 + index
		if index < 1 {
			return str("")
		}
	}
	if index > len(p.fields) {
		return str("")
	}
	if p.fieldsIsTrueStr[index-1] {
		return str(p.fields[index-1])
	} else {
		return numStr(p.fields[index-1])
	}
}

// Get the value of a field by name (for CSV/TSV mode), as in @"name".
func (p *interp) getFieldByName(name string) (value, error) {
	if p.fieldIndexes == nil {
		// Lazily create map of field names to indexes.
		if p.fieldNames == nil {
			return null(), newError(`@ only supported if header parsing enabled; use -H or add "header" to INPUTMODE`)
		}
		p.fieldIndexes = make(map[string]int, len(p.fieldNames))
		for i, n := range p.fieldNames {
			p.fieldIndexes[n] = i + 1
		}
	}
	index := p.fieldIndexes[name]
	if index == 0 {
		return str(""), nil
	}
	return p.getField(index), nil
}

// Sets a single field, equivalent to "$index = value"
func (p *interp) setField(index int, value string) error {
	if index == 0 {
		p.setLine(value, true)
		return nil
	}
	if index > maxFieldIndex {
		return newError("field index too large: %d", index)
	}
	// If there aren't enough fields, add empty string fields in between
	p.ensureFields()
	if index < 1 {
		index = len(p.fields) + 1 + index
		if index < 1 {
			return nil
		}
	}
	for i := len(p.fields); i < index; i++ {
		p.fields = append(p.fields, "")
		p.fieldsIsTrueStr = append(p.fieldsIsTrueStr, true)
	}
	p.fields[index-1] = value
	p.fieldsIsTrueStr[index-1] = true
	p.numFields = len(p.fields)
	p.line = p.joinFields(p.fields)
	p.lineIsTrueStr = true
	return nil
}

func (p *interp) joinFields(fields []string) string {
	switch p.outputMode {
	case CSVMode, TSVMode:
		p.csvJoinFieldsBuf.Reset()
		_ = p.writeCSV(&p.csvJoinFieldsBuf, fields)
		line := p.csvJoinFieldsBuf.Bytes()
		line = line[:len(line)-lenNewline(line)]
		return string(line)
	default:
		return strings.Join(fields, p.outputFieldSep)
	}
}

// Convert value to string using current CONVFMT
func (p *interp) toString(v value) string {
	return v.str(p.convertFormat)
}

// Compile regex string (or fetch from regex cache)
func (p *interp) compileRegex(regex string) (*regexp.Regexp, error) {
	if re, ok := p.regexCache[regex]; ok {
		return re, nil
	}
	re, err := regexp.Compile(compiler.AddRegexFlags(regex))
	if err != nil {
		return nil, newError("invalid regex %q: %s", regex, err)
	}
	// Dumb, non-LRU cache: just cache the first N regexes
	if len(p.regexCache) < maxCachedRegexes {
		p.regexCache[regex] = re
	}
	return re, nil
}

func getDefaultShellCommand() []string {
	executable := "/bin/sh"
	if runtime.GOOS == "windows" {
		executable = "sh"
	}
	return []string{executable, "-c"}
}

func inputModeString(mode IOMode, csvConfig CSVInputConfig) string {
	var s string
	var defaultSep rune
	switch mode {
	case CSVMode:
		s = "csv"
		defaultSep = ','
	case TSVMode:
		s = "tsv"
		defaultSep = '\t'
	case DefaultMode:
		return ""
	}
	if csvConfig.Separator != defaultSep {
		s += " separator=" + string([]rune{csvConfig.Separator})
	}
	if csvConfig.Comment != 0 {
		s += " comment=" + string([]rune{csvConfig.Comment})
	}
	if csvConfig.Header {
		s += " header"
	}
	return s
}

func parseInputMode(s string) (mode IOMode, csvConfig CSVInputConfig, err error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return DefaultMode, CSVInputConfig{}, nil
	}
	switch fields[0] {
	case "csv":
		mode = CSVMode
		csvConfig.Separator = ','
	case "tsv":
		mode = TSVMode
		csvConfig.Separator = '\t'
	default:
		return DefaultMode, CSVInputConfig{}, newError("invalid input mode %q", fields[0])
	}
	for _, field := range fields[1:] {
		key := field
		val := ""
		equals := strings.IndexByte(field, '=')
		if equals >= 0 {
			key = field[:equals]
			val = field[equals+1:]
		}
		switch key {
		case "separator":
			r, n := utf8.DecodeRuneInString(val)
			if n == 0 || n < len(val) {
				return DefaultMode, CSVInputConfig{}, newError("invalid CSV/TSV separator %q", val)
			}
			csvConfig.Separator = r
		case "comment":
			r, n := utf8.DecodeRuneInString(val)
			if n == 0 || n < len(val) {
				return DefaultMode, CSVInputConfig{}, newError("invalid CSV/TSV comment character %q", val)
			}
			csvConfig.Comment = r
		case "header":
			if val != "" && val != "true" && val != "false" {
				return DefaultMode, CSVInputConfig{}, newError("invalid header value %q", val)
			}
			csvConfig.Header = val == "" || val == "true"
		default:
			return DefaultMode, CSVInputConfig{}, newError("invalid input mode key %q", key)
		}
	}
	return mode, csvConfig, nil
}

func outputModeString(mode IOMode, csvConfig CSVOutputConfig) string {
	var s string
	var defaultSep rune
	switch mode {
	case CSVMode:
		s = "csv"
		defaultSep = ','
	case TSVMode:
		s = "tsv"
		defaultSep = '\t'
	case DefaultMode:
		return ""
	}
	if csvConfig.Separator != defaultSep {
		s += " separator=" + string([]rune{csvConfig.Separator})
	}
	return s
}

func parseOutputMode(s string) (mode IOMode, csvConfig CSVOutputConfig, err error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return DefaultMode, CSVOutputConfig{}, nil
	}
	switch fields[0] {
	case "csv":
		mode = CSVMode
		csvConfig.Separator = ','
	case "tsv":
		mode = TSVMode
		csvConfig.Separator = '\t'
	default:
		return DefaultMode, CSVOutputConfig{}, newError("invalid output mode %q", fields[0])
	}
	for _, field := range fields[1:] {
		key := field
		val := ""
		equals := strings.IndexByte(field, '=')
		if equals >= 0 {
			key = field[:equals]
			val = field[equals+1:]
		}
		switch key {
		case "separator":
			r, n := utf8.DecodeRuneInString(val)
			if n == 0 || n < len(val) {
				return DefaultMode, CSVOutputConfig{}, newError("invalid CSV/TSV separator %q", val)
			}
			csvConfig.Separator = r
		default:
			return DefaultMode, CSVOutputConfig{}, newError("invalid output mode key %q", key)
		}
	}
	return mode, csvConfig, nil
}
