package bytecode

//go:generate go run golang.org/x/tools/cmd/stringer@v0.1.8 -type=Op
type Op uint32 // TODO: rename to Opcode?

// TODO: do we need to optimize the order of the opcodes, if a Go switch is a binary tree?
// TODO: does reducing the number of opcodes actually speed things up, for example all the opcodes required for assignment?

const (
	Nop Op = iota

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
	Jump
	JumpFalse
	JumpTrue
	JumpNumEquals
	JumpNumNotEquals
	JumpNumLess
	JumpNumGreater
	JumpNumLessOrEqual
	JumpNumGreaterOrEqual
	JumpStrEquals
	JumpStrNotEquals
	Return
	Next
	Exit

	// "for (k in a)" combinations
	ForGlobalInGlobal  // varIndex arrayIndex offset
	ForGlobalInLocal   // varIndex arrayIndex offset
	ForLocalInGlobal   // varIndex arrayIndex offset
	ForLocalInLocal    // varIndex arrayIndex offset
	ForSpecialInGlobal // varIndex arrayIndex offset
	ForSpecialInLocal  // varIndex arrayIndex offset
	BreakForIn

	// Builtin functions
	CallAtan2
	CallClose
	CallCos
	CallExp
	CallFflush
	CallFflushAll
	CallGsub
	CallGsubField
	CallGsubGlobal      // index
	CallGsubLocal       // index
	CallGsubSpecial     // index
	CallGsubArrayGlobal // arrayIndex
	CallGsubArrayLocal  // arrayIndex
	CallIndex
	CallInt
	CallLength
	CallLengthArg
	CallLog
	CallMatch
	CallRand
	CallSin
	CallSplitGlobal    // arrayIndex
	CallSplitLocal     // arrayIndex
	CallSplitSepGlobal // arrayIndex
	CallSplitSepLocal  // arrayIndex
	CallSprintf        // numArgs
	CallSqrt
	CallSrand
	CallSrandSeed
	CallSub
	CallSubField
	CallSubGlobal      // index
	CallSubLocal       // index
	CallSubSpecial     // index
	CallSubArrayGlobal // arrayIndex
	CallSubArrayLocal  // arrayIndex
	CallSubstr
	CallSubstrLength
	CallSystem
	CallTolower
	CallToupper

	// User and native functions
	CallUser   // index, numArgs
	CallNative // index, numArgs

	// Print and getline operations
	Print  // numArgs, redirect
	Printf // numArgs, redirect
	Getline
	GetlineFile
	GetlineCommand
)
