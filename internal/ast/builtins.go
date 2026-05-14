// Built-in function definitions for AWK

package ast

// BuiltinFunc represents a built-in AWK function. A zero value (BuiltinNone)
// indicates a user-defined function call rather than a built-in.
type BuiltinFunc int

const (
	BuiltinNone BuiltinFunc = iota

	BuiltinAtan2
	BuiltinClose
	BuiltinCos
	BuiltinExp
	BuiltinFflush
	BuiltinGsub
	BuiltinIndex
	BuiltinInt
	BuiltinLength
	BuiltinLog
	BuiltinMatch
	BuiltinRand
	BuiltinSin
	BuiltinSplit
	BuiltinSprintf
	BuiltinSqrt
	BuiltinSrand
	BuiltinSub
	BuiltinSubstr
	BuiltinSystem
	BuiltinTolower
	BuiltinToupper
)

var builtinNames = map[BuiltinFunc]string{
	BuiltinAtan2:   "atan2",
	BuiltinClose:   "close",
	BuiltinCos:     "cos",
	BuiltinExp:     "exp",
	BuiltinFflush:  "fflush",
	BuiltinGsub:    "gsub",
	BuiltinIndex:   "index",
	BuiltinInt:     "int",
	BuiltinLength:  "length",
	BuiltinLog:     "log",
	BuiltinMatch:   "match",
	BuiltinRand:    "rand",
	BuiltinSin:     "sin",
	BuiltinSplit:   "split",
	BuiltinSprintf: "sprintf",
	BuiltinSqrt:    "sqrt",
	BuiltinSrand:   "srand",
	BuiltinSub:     "sub",
	BuiltinSubstr:  "substr",
	BuiltinSystem:  "system",
	BuiltinTolower: "tolower",
	BuiltinToupper: "toupper",
}

// String returns the name of the built-in function.
func (b BuiltinFunc) String() string {
	if name, ok := builtinNames[b]; ok {
		return name
	}
	return "<unknown builtin>"
}

// BuiltinFuncByName looks up a built-in function by name, returning the
// BuiltinFunc and true if found, or BuiltinNone and false if not.
func BuiltinFuncByName(name string) (BuiltinFunc, bool) {
	b, ok := builtinByName[name]
	return b, ok
}

var builtinByName = map[string]BuiltinFunc{
	"atan2":   BuiltinAtan2,
	"close":   BuiltinClose,
	"cos":     BuiltinCos,
	"exp":     BuiltinExp,
	"fflush":  BuiltinFflush,
	"gsub":    BuiltinGsub,
	"index":   BuiltinIndex,
	"int":     BuiltinInt,
	"length":  BuiltinLength,
	"log":     BuiltinLog,
	"match":   BuiltinMatch,
	"rand":    BuiltinRand,
	"sin":     BuiltinSin,
	"split":   BuiltinSplit,
	"sprintf": BuiltinSprintf,
	"sqrt":    BuiltinSqrt,
	"srand":   BuiltinSrand,
	"sub":     BuiltinSub,
	"substr":  BuiltinSubstr,
	"system":  BuiltinSystem,
	"tolower": BuiltinTolower,
	"toupper": BuiltinToupper,
}
