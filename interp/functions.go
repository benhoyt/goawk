// Call native Go functions; helpers for some builtin function calls.

package interp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/benhoyt/goawk/internal/resolver"
	. "github.com/benhoyt/goawk/lexer"
)

// Call native-defined function with given name and arguments, return
// its return value (or null value if it doesn't return anything).
func (p *interp) callNative(index int, args []value) (value, error) {
	f := p.nativeFuncs[index]
	minIn := len(f.in) // Minimum number of args we should pass
	var variadicType reflect.Type
	if f.isVariadic {
		variadicType = f.in[len(f.in)-1].Elem()
		minIn--
	}

	// Build list of args to pass to function
	values := make([]reflect.Value, 0, 7) // up to 7 args won't require heap allocation
	for i, a := range args {
		var argType reflect.Type
		if !f.isVariadic || i < len(f.in)-1 {
			argType = f.in[i]
		} else {
			// Final arg(s) when calling a variadic are all of this type
			argType = variadicType
		}
		values = append(values, p.toNative(a, argType))
	}
	// Use zero value for any unspecified args
	for i := len(args); i < minIn; i++ {
		values = append(values, reflect.Zero(f.in[i]))
	}

	// Call Go function, determine return value
	outs := f.value.Call(values)
	switch len(outs) {
	case 0:
		// No return value, return null value to AWK
		return null(), nil
	case 1:
		// Single return value
		return fromNative(outs[0]), nil
	case 2:
		// Two-valued return of (scalar, error)
		if !outs[1].IsNil() {
			return null(), outs[1].Interface().(error)
		}
		return fromNative(outs[0]), nil
	default:
		// Should never happen (checked at parse time)
		panic(fmt.Sprintf("unexpected number of return values: %d", len(outs)))
	}
}

// Convert from an AWK value to a native Go value
func (p *interp) toNative(v value, typ reflect.Type) reflect.Value {
	switch typ.Kind() {
	case reflect.Bool:
		return reflect.ValueOf(v.boolean())
	case reflect.Int:
		return reflect.ValueOf(int(v.num()))
	case reflect.Int8:
		return reflect.ValueOf(int8(v.num()))
	case reflect.Int16:
		return reflect.ValueOf(int16(v.num()))
	case reflect.Int32:
		return reflect.ValueOf(int32(v.num()))
	case reflect.Int64:
		return reflect.ValueOf(int64(v.num()))
	case reflect.Uint:
		return reflect.ValueOf(uint(v.num()))
	case reflect.Uint8:
		return reflect.ValueOf(uint8(v.num()))
	case reflect.Uint16:
		return reflect.ValueOf(uint16(v.num()))
	case reflect.Uint32:
		return reflect.ValueOf(uint32(v.num()))
	case reflect.Uint64:
		return reflect.ValueOf(uint64(v.num()))
	case reflect.Float32:
		return reflect.ValueOf(float32(v.num()))
	case reflect.Float64:
		return reflect.ValueOf(v.num())
	case reflect.String:
		return reflect.ValueOf(p.toString(v))
	case reflect.Slice:
		if typ.Elem().Kind() != reflect.Uint8 {
			// Shouldn't happen: prevented by checkNativeFunc
			panic(fmt.Sprintf("unexpected argument slice: %s", typ.Elem().Kind()))
		}
		return reflect.ValueOf([]byte(p.toString(v)))
	default:
		// Shouldn't happen: prevented by checkNativeFunc
		panic(fmt.Sprintf("unexpected argument type: %s", typ.Kind()))
	}
}

// Convert from a native Go value to an AWK value
func fromNative(v reflect.Value) value {
	switch v.Kind() {
	case reflect.Bool:
		return boolean(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return num(float64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return num(float64(v.Uint()))
	case reflect.Float32, reflect.Float64:
		return num(v.Float())
	case reflect.String:
		return str(v.String())
	case reflect.Slice:
		if b, ok := v.Interface().([]byte); ok {
			return str(string(b))
		}
		// Shouldn't happen: prevented by checkNativeFunc
		panic(fmt.Sprintf("unexpected return slice: %s", v.Type().Elem().Kind()))
	default:
		// Shouldn't happen: prevented by checkNativeFunc
		panic(fmt.Sprintf("unexpected return type: %s", v.Kind()))
	}
}

// Used for caching native function type information on init
type nativeFunc struct {
	isVariadic bool
	in         []reflect.Type
	value      reflect.Value
}

// Check and initialize native functions
func (p *interp) initNativeFuncs(funcs map[string]interface{}) error {
	for name, f := range funcs {
		err := checkNativeFunc(name, f)
		if err != nil {
			return err
		}
	}

	// Sort functions by name, then use those indexes to build slice
	// (this has to match how the parser sets the indexes).
	names := make([]string, 0, len(funcs))
	for name := range funcs {
		names = append(names, name)
	}
	sort.Strings(names)
	p.nativeFuncs = make([]nativeFunc, len(names))
	for i, name := range names {
		f := funcs[name]
		typ := reflect.TypeOf(f)
		in := make([]reflect.Type, typ.NumIn())
		for j := 0; j < len(in); j++ {
			in[j] = typ.In(j)
		}
		p.nativeFuncs[i] = nativeFunc{
			isVariadic: typ.IsVariadic(),
			in:         in,
			value:      reflect.ValueOf(f),
		}
	}
	return nil
}

// Got this trick from the Go stdlib text/template source
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Check that native function with given name is okay to call from
// AWK, return an *interp.Error if not. This checks that f is actually
// a function, and that its parameter and return types are good.
func checkNativeFunc(name string, f interface{}) error {
	if KeywordToken(name) != ILLEGAL {
		return newError("can't use keyword %q as native function name", name)
	}

	typ := reflect.TypeOf(f)
	if typ.Kind() != reflect.Func {
		return newError("native function %q is not a function", name)
	}
	for i := 0; i < typ.NumIn(); i++ {
		param := typ.In(i)
		if typ.IsVariadic() && i == typ.NumIn()-1 {
			param = param.Elem()
		}
		if !validNativeType(param) {
			return newError("native function %q param %d is not int or string", name, i)
		}
	}

	switch typ.NumOut() {
	case 0:
		// No return value is fine
	case 1:
		// Single scalar return value is fine
		if !validNativeType(typ.Out(0)) {
			return newError("native function %q return value is not int or string", name)
		}
	case 2:
		// Returning (scalar, error) is handled too
		if !validNativeType(typ.Out(0)) {
			return newError("native function %q first return value is not int or string", name)
		}
		if typ.Out(1) != errorType {
			return newError("native function %q second return value is not an error", name)
		}
	default:
		return newError("native function %q returns more than two values", name)
	}
	return nil
}

// Return true if typ is a valid parameter or return type.
func validNativeType(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Bool:
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Float32, reflect.Float64:
		return true
	case reflect.String:
		return true
	case reflect.Slice:
		// Only allow []byte (convert to string in AWK)
		return typ.Elem().Kind() == reflect.Uint8
	default:
		return false
	}
}

// Guts of the split() function
func (p *interp) split(s string, scope resolver.Scope, index int, fs string, mode IOMode) (int, error) {
	var parts []string
	switch {
	case mode == CSVMode || mode == TSVMode:
		// Set up for parsing a CSV/TSV record
		splitter := csvSplitter{
			separator: p.csvInputConfig.Separator,
			sepLen:    utf8.RuneLen(p.csvInputConfig.Separator),
			comment:   p.csvInputConfig.Comment,
			fields:    &parts,
		}
		scanner := bufio.NewScanner(strings.NewReader(s))
		scanner.Split(splitter.scan)
		if p.splitBuffer == nil {
			p.splitBuffer = make([]byte, inputBufSize)
		}
		scanner.Buffer(p.splitBuffer, maxRecordLength)

		// Parse one record. Errors shouldn't happen, but if there is one,
		// len(parts) will be 0.
		scanner.Scan()
	case fs == " ":
		parts = strings.Fields(s)
	case s == "":
		// Leave parts 0 length on empty string
	case utf8.RuneCountInString(fs) <= 1:
		parts = strings.Split(s, fs)
	default:
		re, err := p.compileRegex(fs)
		if err != nil {
			return 0, err
		}
		parts = re.Split(s, -1)
	}
	array := make(map[string]value, len(parts))
	for i, part := range parts {
		array[strconv.Itoa(i+1)] = numStr(part)
	}
	p.arrays[p.arrayIndex(scope, index)] = array
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
						r = append(r, '&')
					case '\\':
						r = append(r, '\\')
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
				continue
			}
			for i < len(s) && bytes.IndexByte([]byte(" .-+*#0123456789"), s[i]) >= 0 {
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
	converted := make([]interface{}, 0, 7) // up to 7 args won't require heap allocation
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
			v = uint(a.num())
		case 'c':
			var c []byte
			n, isStr := a.isTrueStr()
			if isStr {
				s := p.toString(a)
				if len(s) > 0 {
					c = []byte{s[0]}
				} else {
					c = []byte{0}
				}
			} else {
				// Follow the behaviour of awk and mawk, where %c
				// operates on bytes (0-255), not Unicode codepoints
				c = []byte{byte(n)}
			}
			v = c
		}
		converted = append(converted, v)
	}
	return fmt.Sprintf(format, converted...), nil
}
