package bytecode

type Opcode uint32

const (
	Nop Opcode = iota

	// Stack operations
	Num   // numIndex
	Str   // strIndex
	Regex // regexIndex
	Dupe
	Drop

	// Fetch a field, variable, or array item
	Field
	Global      // index
	Local       // index
	Special     // index
	ArrayGlobal // index
	ArrayLocal  // index
	InGlobal    // index
	InLocal     // index

	// Assign a field, variable, or array item
	AssignField
	AssignGlobal      // index
	AssignLocal       // index
	AssignSpecial     // index
	AssignArrayGlobal // index
	AssignArrayLocal  // index
	DeleteGlobal      // arrayIndex
	DeleteLocal       // arrayIndex
	DeleteAllGlobal   // arrayIndex
	DeleteAllLocal    // arrayIndex

	// Post-increment and post-decrement
	PostIncrField
	PostIncrGlobal
	PostIncrLocal
	PostIncrSpecial
	PostIncrArrayGlobal
	PostIncrArrayLocal
	PostDecrField
	PostDecrGlobal
	PostDecrLocal
	PostDecrSpecial
	PostDecrArrayGlobal
	PostDecrArrayLocal

	// Augmented assignment (also used for pre-increment and pre-decrement)
	AugAssignField       // operation
	AugAssignGlobal      // operation, index
	AugAssignLocal       // operation, index
	AugAssignSpecial     // operation, index
	AugAssignArrayGlobal // operation, index
	AugAssignArrayLocal  // operation, index

	// Binary operators
	Add
	Sub
	Mul
	Div
	Pow
	Mod
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
	ForIn
	CallBuiltin // func, numArgs
	CallUser    // index, numArgs
	CallNative  // index, numArgs
	Return
	Next
	Exit

	// Other operations
	Print          // numArgs
	PrintRedirect  // numArgs, redirect
	Printf         // numArgs
	PrintfRedirect // numArgs, redirect
	Getline
	GetlineFile
	GetlineCommand
)
