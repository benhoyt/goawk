// The New...Execute API (allows you to efficiently execute the same program repeatedly).

package interp

import (
	"context"
	"math"

	"github.com/benhoyt/goawk/internal/resolver"
	"github.com/benhoyt/goawk/parser"
)

const checkContextOps = 1000 // for efficiency, only check context every N instructions

// Interpreter is an interpreter for a specific program, allowing you to
// efficiently execute the same program over and over with different inputs.
// Use New to create an Interpreter.
//
// Most programs won't need reusable execution, and should use the simpler
// Exec or ExecProgram functions instead.
type Interpreter struct {
	interp *interp
}

// New creates a reusable interpreter for the given program.
//
// Most programs won't need reusable execution, and should use the simpler
// Exec or ExecProgram functions instead.
func New(program *parser.Program) (*Interpreter, error) {
	p := newInterp(program)
	return &Interpreter{interp: p}, nil
}

// Execute runs this program with the given execution configuration (input,
// output, and variables) and returns the exit status code of the program. A
// nil config is valid and will use the defaults (zero values).
//
// Internal memory allocations are reused, so calling Execute on the same
// Interpreter instance is significantly more efficient than calling
// ExecProgram multiple times.
//
// I/O state is reset between each run, but variables and the random number
// generator seed are not; use ResetVars and ResetRand to reset those.
//
// It's best to set config.Environ to a non-nil slice, otherwise Execute will
// call the relatively inefficient os.Environ each time. Set config.Environ to
// []string{} if the script doesn't need environment variables, or call
// os.Environ once and set config.Environ to that value each execution.
//
// Note that config.Funcs must be the same value provided to
// parser.ParseProgram, and must not change between calls to Execute.
func (p *Interpreter) Execute(config *Config) (int, error) {
	p.interp.resetCore()
	p.interp.checkCtx = false

	err := p.interp.setExecuteConfig(config)
	if err != nil {
		return 0, err
	}

	return p.interp.executeAll()
}

// Array returns a map representing the items in the named AWK array. AWK
// numbers are included as type float64, strings (including "numeric strings")
// are included as type string. If the named array does not exist, return nil.
func (p *Interpreter) Array(name string) map[string]interface{} {
	index, exists := p.interp.arrayIndexes[name]
	if !exists {
		return nil
	}
	array := p.interp.array(resolver.Global, index)
	result := make(map[string]interface{}, len(array))
	for k, v := range array {
		switch v.typ {
		case typeNum:
			result[k] = v.n
		case typeStr, typeNumStr:
			result[k] = v.s
		default:
			result[k] = ""
		}
	}
	return result
}

func (p *interp) resetCore() {
	p.scanner = nil
	for k := range p.scanners {
		delete(p.scanners, k)
	}
	p.input = nil
	for k := range p.inputStreams {
		delete(p.inputStreams, k)
	}
	for k := range p.outputStreams {
		delete(p.outputStreams, k)
	}

	p.sp = 0
	p.localArrays = p.localArrays[:0]
	p.callDepth = 0

	p.filename = null()
	p.line = ""
	p.lineIsTrueStr = false
	p.lineNum = 0
	p.fileLineNum = 0
	p.fields = nil
	p.fieldsIsTrueStr = nil
	p.numFields = 0
	p.haveFields = false

	p.exitStatus = 0
}

func (p *interp) resetVars() {
	// Reset global scalars
	for i := range p.globals {
		p.globals[i] = null()
	}

	// Reset global arrays
	for _, array := range p.arrays {
		for k := range array {
			delete(array, k)
		}
	}

	// Reset special variables
	p.convertFormat = "%.6g"
	p.outputFormat = "%.6g"
	p.fieldSep = " "
	p.fieldSepRegex = nil
	p.recordSep = "\n"
	p.recordSepRegex = nil
	p.recordTerminator = ""
	p.outputFieldSep = " "
	p.outputRecordSep = "\n"
	p.subscriptSep = "\x1c"
	p.matchLength = 0
	p.matchStart = 0
}

// ResetVars resets this interpreter's variables, setting scalar variables to
// null, clearing arrays, and resetting special variables such as FS and RS to
// their defaults.
func (p *Interpreter) ResetVars() {
	p.interp.resetVars()
}

// ResetRand resets this interpreter's random number generator seed, so that
// rand() produces the same sequence it would have after calling New. This is
// a relatively CPU-intensive operation.
func (p *Interpreter) ResetRand() {
	p.interp.randSeed = 1.0
	p.interp.random.Seed(int64(math.Float64bits(p.interp.randSeed)))
}

// ExecuteContext is like Execute, but takes a context to allow the caller to
// set an execution timeout or cancel the execution. For efficiency, the
// context is only tested every 1000 virtual machine instructions.
//
// Context handling is not preemptive: currently long-running operations like
// system() won't be interrupted.
func (p *Interpreter) ExecuteContext(ctx context.Context, config *Config) (int, error) {
	p.interp.resetCore()
	p.interp.checkCtx = ctx != context.Background() && ctx != context.TODO()
	p.interp.ctx = ctx
	p.interp.ctxDone = ctx.Done()
	p.interp.ctxOps = 0

	err := p.interp.setExecuteConfig(config)
	if err != nil {
		return 0, err
	}

	return p.interp.executeAll()
}

func (p *interp) checkContext() error {
	p.ctxOps++
	if p.ctxOps < checkContextOps {
		return nil
	}
	p.ctxOps = 0
	return p.checkContextNow()
}

func (p *interp) checkContextNow() error {
	select {
	case <-p.ctxDone:
		return p.ctx.Err()
	default:
		return nil
	}
}
