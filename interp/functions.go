// Evaluate builtin and user-defined function calls

package interp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	. "github.com/benhoyt/goawk/internal/ast"
	. "github.com/benhoyt/goawk/lexer"
)

// Call builtin function specified by "op" with given args
func (p *interp) callBuiltin(op Token, argExprs []Expr) (value, error) {
	// split() has an array arg (not evaluated) and [g]sub() have an
	// lvalue arg, so handle them as special cases
	switch op {
	case F_SPLIT:
		strValue, err := p.eval(argExprs[0])
		if err != nil {
			return value{}, err
		}
		str := p.toString(strValue)
		var fieldSep string
		if len(argExprs) == 3 {
			sepValue, err := p.eval(argExprs[2])
			if err != nil {
				return value{}, err
			}
			fieldSep = p.toString(sepValue)
		} else {
			fieldSep = p.fieldSep
		}
		arrayExpr := argExprs[1].(*ArrayExpr)
		n, err := p.split(str, arrayExpr.Scope, arrayExpr.Index, fieldSep)
		if err != nil {
			return value{}, err
		}
		return num(float64(n)), nil

	case F_SUB, F_GSUB:
		regexValue, err := p.eval(argExprs[0])
		if err != nil {
			return value{}, err
		}
		regex := p.toString(regexValue)
		replValue, err := p.eval(argExprs[1])
		if err != nil {
			return value{}, err
		}
		repl := p.toString(replValue)
		var in string
		if len(argExprs) == 3 {
			inValue, err := p.eval(argExprs[2])
			if err != nil {
				return value{}, err
			}
			in = p.toString(inValue)
		} else {
			in = p.line
		}
		out, n, err := p.sub(regex, repl, in, op == F_GSUB)
		if err != nil {
			return value{}, err
		}
		if len(argExprs) == 3 {
			p.assign(argExprs[2], str(out))
		} else {
			p.setLine(out)
		}
		return num(float64(n)), nil
	}

	// Now evaluate the argExprs (calls with up to 7 args don't
	// require heap allocation)
	args := make([]value, 0, 7)
	for _, a := range argExprs {
		arg, err := p.eval(a)
		if err != nil {
			return value{}, err
		}
		args = append(args, arg)
	}

	// Then switch on the function for the ordinary functions
	switch op {
	case F_LENGTH:
		switch len(args) {
		case 0:
			return num(float64(len(p.line))), nil
		default:
			return num(float64(len(p.toString(args[0])))), nil
		}

	case F_MATCH:
		re, err := p.compileRegex(p.toString(args[1]))
		if err != nil {
			return value{}, err
		}
		loc := re.FindStringIndex(p.toString(args[0]))
		if loc == nil {
			p.matchStart = 0
			p.matchLength = -1
			return num(0), nil
		}
		p.matchStart = loc[0] + 1
		p.matchLength = loc[1] - loc[0]
		return num(float64(p.matchStart)), nil

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
		return str(s[pos-1 : pos-1+length]), nil

	case F_SPRINTF:
		s, err := p.sprintf(p.toString(args[0]), args[1:])
		if err != nil {
			return value{}, err
		}
		return str(s), nil

	case F_INDEX:
		s := p.toString(args[0])
		substr := p.toString(args[1])
		return num(float64(strings.Index(s, substr) + 1)), nil

	case F_TOLOWER:
		return str(strings.ToLower(p.toString(args[0]))), nil
	case F_TOUPPER:
		return str(strings.ToUpper(p.toString(args[0]))), nil

	case F_ATAN2:
		return num(math.Atan2(args[0].num(), args[1].num())), nil
	case F_COS:
		return num(math.Cos(args[0].num())), nil
	case F_EXP:
		return num(math.Exp(args[0].num())), nil
	case F_INT:
		return num(float64(int(args[0].num()))), nil
	case F_LOG:
		return num(math.Log(args[0].num())), nil
	case F_SQRT:
		return num(math.Sqrt(args[0].num())), nil
	case F_RAND:
		return num(p.random.Float64()), nil
	case F_SIN:
		return num(math.Sin(args[0].num())), nil

	case F_SRAND:
		prevSeed := p.randSeed
		switch len(args) {
		case 0:
			p.random.Seed(time.Now().UnixNano())
		case 1:
			p.randSeed = args[0].num()
			p.random.Seed(int64(math.Float64bits(p.randSeed)))
		}
		return num(prevSeed), nil

	case F_SYSTEM:
		cmdline := p.toString(args[0])
		cmd := exec.Command("sh", "-c", cmdline)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return num(-1), nil
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return num(-1), nil
		}
		err = cmd.Start()
		if err != nil {
			fmt.Fprintln(p.errorOutput, err)
			return num(-1), nil
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
					return num(float64(status.ExitStatus())), nil
				} else {
					fmt.Fprintf(p.errorOutput, "couldn't get exit status for %q: %v\n", cmdline, err)
					return num(-1), nil
				}
			} else {
				fmt.Fprintf(p.errorOutput, "unexpected error running command %q: %v\n", cmdline, err)
				return num(-1), nil
			}
		}
		return num(0), nil

	case F_CLOSE:
		name := p.toString(args[0])
		w, ok := p.streams[name]
		if !ok {
			return num(-1), nil
		}
		err := w.Close()
		if err != nil {
			return num(-1), nil
		}
		return num(0), nil

	default:
		// Shouldn't happen
		panic(fmt.Sprintf("unexpected function: %s", op))
	}
}

// Call user-defined function with given index and arguments, return
// return value (or null value if it doesn't return anything)
func (p *interp) callUser(index int, args []Expr) (value, error) {
	f := p.program.Functions[index]

	// Evaluate the arguments and push them onto the locals stack
	oldFrame := p.frame
	newFrameStart := len(p.stack)
	var arrays []int
	for i, arg := range args {
		if f.Arrays[i] {
			a := arg.(*VarExpr)
			arrays = append(arrays, a.Index)
		} else {
			argValue, err := p.eval(arg)
			if err != nil {
				return value{}, err
			}
			p.stack = append(p.stack, argValue)
		}
	}
	// Push zero value for any additional parameters (it's valid to
	// call a function with fewer arguments than it has parameters)
	oldArraysLen := len(p.arrays)
	for i := len(args); i < len(f.Params); i++ {
		if f.Arrays[i] {
			arrays = append(arrays, len(p.arrays))
			p.arrays = append(p.arrays, nil)
		} else {
			p.stack = append(p.stack, value{})
		}
	}
	p.frame = p.stack[newFrameStart:]
	p.localArrays = append(p.localArrays, arrays)

	// Execute the function!
	err := p.executes(f.Body)

	// Pop the locals off the stack
	p.stack = p.stack[:newFrameStart]
	p.frame = oldFrame
	p.localArrays = p.localArrays[:len(p.localArrays)-1]
	p.arrays = p.arrays[:oldArraysLen]

	if r, ok := err.(returnValue); ok {
		return r.Value, nil
	}
	if err != nil {
		return value{}, err
	}
	return value{}, nil
}

// Guts of the split() function
func (p *interp) split(s string, scope VarScope, index int, fs string) (int, error) {
	var parts []string
	if fs == " " {
		parts = strings.Fields(s)
	} else if s != "" {
		re, err := p.compileRegex(fs)
		if err != nil {
			return 0, err
		}
		parts = re.Split(s, -1)
	}
	array := make(map[string]value)
	for i, part := range parts {
		array[strconv.Itoa(i+1)] = numStr(part)
	}
	p.arrays[p.getArrayIndex(scope, index)] = array
	return len(array), nil
}

// Guts of the sub() and gsub() functions
func (p *interp) sub(regex, repl, in string, global bool) (out string, num int, err error) {
	re, err := p.compileRegex(regex)
	if err != nil {
		return "", 0, err
	}
	count := 0
	out = re.ReplaceAllStringFunc(in, func(s string) string {
		// Only do the first replacement for sub(), or all for gsub()
		if !global && count > 0 {
			return s
		}
		count++
		// Handle & (ampersand) properly in replacement string
		r := make([]byte, 0, 64) // Up to 64 byte replacement won't require heap allocation
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
	return out, count, nil
}

type cachedFormat struct {
	format string
	types  []byte
}

// Parse given sprintf format string into Go format string, along with
// type conversion specifiers. Output is memoized in a simple cache
// for performance.
func (p *interp) parseFmtTypes(s string) (format string, types []byte, err error) {
	if item, ok := p.formatCache[s]; ok {
		return item.format, item.types, nil
	}

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
			case 's':
				t = 's'
			case 'd', 'i', 'o', 'x', 'X':
				t = 'd'
			case 'f', 'e', 'E', 'g', 'G':
				t = 'f'
			case 'u':
				t = 'u'
				out[i] = 'd'
			case 'c':
				t = 'c'
				out[i] = 's'
			default:
				return "", nil, fmt.Errorf("invalid format type %q", s[i])
			}
			types = append(types, t)
		}
	}

	// Dumb, non-LRU cache: just cache the first N formats
	format = string(out)
	if len(p.formatCache) < maxCachedFormats {
		p.formatCache[s] = cachedFormat{format, types}
	}
	return format, types, nil
}

// Guts of sprintf() function (also used by "printf" statement)
func (p *interp) sprintf(format string, args []value) (string, error) {
	format, types, err := p.parseFmtTypes(format)
	if err != nil {
		return "", newError("format error: %s", err)
	}
	if len(types) > len(args) {
		return "", newError("format error: got %d args, expected %d", len(args), len(types))
	}
	converted := make([]interface{}, len(types))
	for i, t := range types {
		a := args[i]
		var v interface{}
		switch t {
		case 's':
			v = p.toString(a)
		case 'd':
			v = int(a.num())
		case 'f':
			v = a.num()
		case 'u':
			v = uint32(a.num())
		case 'c':
			var c []byte
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
		}
		converted[i] = v
	}
	return fmt.Sprintf(format, converted...), nil
}
