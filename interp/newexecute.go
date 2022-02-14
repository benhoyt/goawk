// The New...Execute API (allows you to efficiently execute the same program repeatedly).

package interp

import (
	"math"

	"github.com/benhoyt/goawk/parser"
)

// Interpreter is an interpreter for a specific program, allowing you to
// efficiently execute the same program over and over with different inputs.
// Use New to create an Interpreter.
//
// Most programs won't need reusable execution, and should use the simpler
// Exec or ExecProgram functions instead.
type Interpreter struct {
	interp  *interp
	noReset bool
}

// New creates a reusable interpreter for the given program.
//
// Most programs won't need reusable execution, and should use the simpler
// Exec or ExecProgram functions instead.
func New(program *parser.Program) (*Interpreter, error) {
	p := newInterp(program)
	return &Interpreter{interp: p, noReset: true}, nil
}

// Execute runs this program with the given execution configuration (input,
// output, and variables) and returns the exit status code of the program. A
// nil config is valid and will use the defaults (zero values).
//
// Interpreter state is reset between each run, except for resetting the
// random number generator seed, because that is an expensive operation (call
// the ResetRand method if you need to reset that). Internal memory
// allocations are reused, so calling Execute on the same interpreter is
// significantly more efficient than calling ExecProgram multiple times.
//
// Note that config.Funcs must be the same value provided to
// parser.ParseProgram, and must not change between calls to Execute.
func (p *Interpreter) Execute(config *Config) (int, error) {
	if !p.noReset {
		p.interp.reset()
	}
	p.noReset = false

	err := p.interp.setExecuteConfig(config)
	if err != nil {
		return 0, err
	}

	return p.interp.executeAll()
}

func (p *interp) reset() {
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
	for k := range p.commands {
		delete(p.commands, k)
	}

	for i := range p.globals {
		p.globals[i] = null()
	}
	p.sp = 0
	for _, array := range p.arrays {
		for k := range array {
			delete(array, k)
		}
	}
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

	p.exitStatus = 0
}

// ResetRand resets this interpreter's random number generator seed, so that
// rand() produces the same sequence it would have after calling New.
func (p *Interpreter) ResetRand() {
	p.interp.randSeed = 1.0
	p.interp.random.Seed(int64(math.Float64bits(p.interp.randSeed)))
}
