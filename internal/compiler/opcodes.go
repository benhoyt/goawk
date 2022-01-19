package compiler

//go:generate go run golang.org/x/tools/cmd/stringer@v0.1.8 -type=Opcode
type Opcode uint32

// TODO: simpler if Opcode is signed int32?
// TODO: do we need to optimize the order of the opcodes, if a Go switch is a binary tree?
// TODO: does reducing the number of opcodes actually speed things up, for example all the opcodes required for assignment?

const (
	Nop Opcode = iota

	// Stack operations
	Num // numIndex
	Str // strIndex
	Dupe
	Drop

	// Fetch a field, variable, or array item
	Field
	FieldNum    // index
	Global      // index
	Local       // index
	Special     // index
	ArrayGlobal // arrayIndex
	ArrayLocal  // arrayIndex
	InGlobal    // arrayIndex
	InLocal     // arrayIndex

	// Assign a field, variable, or array item
	AssignField
	AssignGlobal      // index
	AssignLocal       // index
	AssignSpecial     // index
	AssignArrayGlobal // arrayIndex
	AssignArrayLocal  // arrayIndex

	// Delete statement
	// TODO: add arrayScope param and reduce to just Delete and DeleteAll opcodes
	DeleteGlobal    // arrayIndex
	DeleteLocal     // arrayIndex
	DeleteAllGlobal // arrayIndex
	DeleteAllLocal  // arrayIndex

	// Post-increment and post-decrement
	IncrField       // amount
	IncrGlobal      // amount index
	IncrLocal       // amount index
	IncrSpecial     // amount index
	IncrArrayGlobal // amount arrayIndex
	IncrArrayLocal  // amount arrayIndex

	// Augmented assignment (also used for pre-increment and pre-decrement)
	AugAssignField       // operation
	AugAssignGlobal      // operation, index
	AugAssignLocal       // operation, index
	AugAssignSpecial     // operation, index
	AugAssignArrayGlobal // operation, arrayIndex
	AugAssignArrayLocal  // operation, arrayIndex

	// Stand-alone regex expression /foo/
	Regex // regexIndex

	// Multi-index concatenation
	MultiIndex // num

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

	// "for (k in a)" combinations
	ForInGlobal  // varIndex arrayScope arrayIndex offset
	ForInLocal   // varIndex arrayScope arrayIndex offset
	ForInSpecial // varIndex arrayScope arrayIndex offset
	BreakForIn

	// Builtin functions
	CallAtan2
	CallClose
	CallCos
	CallExp
	CallFflush
	CallFflushAll
	CallGsub
	CallIndex
	CallInt
	CallLength
	CallLengthArg
	CallLog
	CallMatch
	CallRand
	CallSin
	// TODO: add arrayScope and reduce opcodes to just CallSplit and CallSplitSep (or even push sep and combine into one)
	CallSplitGlobal    // arrayIndex
	CallSplitLocal     // arrayIndex
	CallSplitSepGlobal // arrayIndex
	CallSplitSepLocal  // arrayIndex
	CallSprintf        // numArgs
	CallSqrt
	CallSrand
	CallSrandSeed
	CallSub
	CallSubstr
	CallSubstrLength
	CallSystem
	CallTolower
	CallToupper

	// User and native functions
	CallUser   // funcIndex, numArrayArgs[, arrayScope1, arrayIndex1, ...]
	CallNative // funcIndex, numArgs
	Return
	ReturnNull
	Nulls // numNulls

	// Print, printf, and getline
	Print          // numArgs, redirect
	Printf         // numArgs, redirect
	Getline        // redirect
	GetlineField   // redirect
	GetlineGlobal  // redirect, index
	GetlineLocal   // redirect, index
	GetlineSpecial // redirect, index
	// TODO: add arrayScope and reduce to one opcode
	GetlineArrayGlobal // redirect, arrayIndex
	GetlineArrayLocal  // redirect, arrayIndex
)
