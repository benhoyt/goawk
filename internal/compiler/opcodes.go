package compiler

//go:generate go run golang.org/x/tools/cmd/stringer@v0.1.8 -type=Opcode,AugOp,BuiltinOp

// Opcode represents a single virtual machine instruction (or argument). The
// comments beside each opcode show any arguments that instruction consumes.
//
// Normally this is called "bytecode", but I've avoided that term here as each
// opcode is a 32-bit word, not an 8-bit byte.
//
// I tested various bit widths, and I believe 32 bit was the fastest, but also
// means we don't have to worry about jump offsets overflowing. That's tested
// in the compiler, but who's going to have an AWK program bigger than 2GB?
type Opcode int32

const (
	Nop Opcode = iota

	// Stack operations
	Num // numIndex
	Str // strIndex
	Dupe
	Drop
	Swap

	// Fetch a field, variable, or array item
	Field
	FieldInt // index
	FieldByName
	FieldByNameStr // strIndex
	Global         // index
	Local          // index
	Special        // index
	ArrayGlobal    // arrayIndex
	ArrayLocal     // arrayIndex
	InGlobal       // arrayIndex
	InLocal        // arrayIndex

	// Assign a field, variable, or array item
	AssignField
	AssignGlobal      // index
	AssignLocal       // index
	AssignSpecial     // index
	AssignArrayGlobal // arrayIndex
	AssignArrayLocal  // arrayIndex

	// Delete statement
	Delete    // arrayScope arrayIndex
	DeleteAll // arrayScope arrayIndex

	// Post-increment and post-decrement
	IncrField       // amount
	IncrGlobal      // amount index
	IncrLocal       // amount index
	IncrSpecial     // amount index
	IncrArrayGlobal // amount arrayIndex
	IncrArrayLocal  // amount arrayIndex

	// Augmented assignment (also used for pre-increment and pre-decrement)
	AugAssignField       // augOp
	AugAssignGlobal      // augOp index
	AugAssignLocal       // augOp index
	AugAssignSpecial     // augOp index
	AugAssignArrayGlobal // augOp arrayIndex
	AugAssignArrayLocal  // augOp arrayIndex

	// Stand-alone regex expression /foo/
	Regex // regexIndex

	// Multi-index concatenation
	IndexMulti // num

	// Multi-value concatenation
	ConcatMulti // num

	// Binary operators
	Add
	Subtract
	Multiply
	Divide
	Power
	Modulo
	Equals
	NotEquals
	Less
	Greater
	LessOrEqual
	GreaterOrEqual
	Concat
	Match
	NotMatch

	// Unary operators
	Not
	UnaryMinus
	UnaryPlus
	Boolean

	// Control flow
	Jump               // offset
	JumpFalse          // offset
	JumpTrue           // offset
	JumpEquals         // offset
	JumpNotEquals      // offset
	JumpLess           // offset
	JumpGreater        // offset
	JumpLessOrEqual    // offset
	JumpGreaterOrEqual // offset
	Next
	Exit
	ForIn // varScope varIndex arrayScope arrayIndex offset
	BreakForIn

	// Builtin functions
	CallBuiltin  // builtinOp
	CallSplit    // arrayScope arrayIndex
	CallSplitSep // arrayScope arrayIndex
	CallSprintf  // numArgs

	// User and native functions
	CallUser   // funcIndex numArrayArgs [arrayScope1 arrayIndex1 ...]
	CallNative // funcIndex numArgs
	Return
	ReturnNull
	Nulls // numNulls

	// Print, printf, and getline
	Print          // numArgs redirect
	Printf         // numArgs redirect
	Getline        // redirect
	GetlineField   // redirect
	GetlineGlobal  // redirect index
	GetlineLocal   // redirect index
	GetlineSpecial // redirect index
	GetlineArray   // redirect arrayScope arrayIndex

	EndOpcode
)

// AugOp represents an augmented assignment operation.
type AugOp Opcode

const (
	AugOpAdd AugOp = iota
	AugOpSub
	AugOpMul
	AugOpDiv
	AugOpPow
	AugOpMod
)

// BuiltinOp represents a builtin function call.
type BuiltinOp Opcode

const (
	BuiltinAtan2 BuiltinOp = iota
	BuiltinClose
	BuiltinCos
	BuiltinExp
	BuiltinFflush
	BuiltinFflushAll
	BuiltinGsub
	BuiltinIndex
	BuiltinInt
	BuiltinLength
	BuiltinLengthArg
	BuiltinLog
	BuiltinMatch
	BuiltinRand
	BuiltinSin
	BuiltinSqrt
	BuiltinSrand
	BuiltinSrandSeed
	BuiltinSub
	BuiltinSubstr
	BuiltinSubstrLength
	BuiltinSystem
	BuiltinTolower
	BuiltinToupper
)
