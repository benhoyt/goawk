// Code generated by "stringer -type=Opcode,AugOp,BuiltinOp"; DO NOT EDIT.

package compiler

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Nop-0]
	_ = x[Num-1]
	_ = x[Str-2]
	_ = x[Dupe-3]
	_ = x[Drop-4]
	_ = x[Swap-5]
	_ = x[At-6]
	_ = x[Field-7]
	_ = x[FieldInt-8]
	_ = x[Global-9]
	_ = x[Local-10]
	_ = x[Special-11]
	_ = x[ArrayGlobal-12]
	_ = x[ArrayLocal-13]
	_ = x[InGlobal-14]
	_ = x[InLocal-15]
	_ = x[AssignField-16]
	_ = x[AssignGlobal-17]
	_ = x[AssignLocal-18]
	_ = x[AssignSpecial-19]
	_ = x[AssignArrayGlobal-20]
	_ = x[AssignArrayLocal-21]
	_ = x[Delete-22]
	_ = x[DeleteAll-23]
	_ = x[IncrField-24]
	_ = x[IncrGlobal-25]
	_ = x[IncrLocal-26]
	_ = x[IncrSpecial-27]
	_ = x[IncrArrayGlobal-28]
	_ = x[IncrArrayLocal-29]
	_ = x[AugAssignField-30]
	_ = x[AugAssignGlobal-31]
	_ = x[AugAssignLocal-32]
	_ = x[AugAssignSpecial-33]
	_ = x[AugAssignArrayGlobal-34]
	_ = x[AugAssignArrayLocal-35]
	_ = x[Regex-36]
	_ = x[IndexMulti-37]
	_ = x[ConcatMulti-38]
	_ = x[Add-39]
	_ = x[Subtract-40]
	_ = x[Multiply-41]
	_ = x[Divide-42]
	_ = x[Power-43]
	_ = x[Modulo-44]
	_ = x[Equals-45]
	_ = x[NotEquals-46]
	_ = x[Less-47]
	_ = x[Greater-48]
	_ = x[LessOrEqual-49]
	_ = x[GreaterOrEqual-50]
	_ = x[Concat-51]
	_ = x[Match-52]
	_ = x[NotMatch-53]
	_ = x[Not-54]
	_ = x[UnaryMinus-55]
	_ = x[UnaryPlus-56]
	_ = x[Boolean-57]
	_ = x[Jump-58]
	_ = x[JumpFalse-59]
	_ = x[JumpTrue-60]
	_ = x[JumpEquals-61]
	_ = x[JumpNotEquals-62]
	_ = x[JumpLess-63]
	_ = x[JumpGreater-64]
	_ = x[JumpLessOrEqual-65]
	_ = x[JumpGreaterOrEqual-66]
	_ = x[Next-67]
	_ = x[Exit-68]
	_ = x[ForIn-69]
	_ = x[BreakForIn-70]
	_ = x[CallBuiltin-71]
	_ = x[CallSplit-72]
	_ = x[CallSplitSep-73]
	_ = x[CallSprintf-74]
	_ = x[CallUser-75]
	_ = x[CallNative-76]
	_ = x[Return-77]
	_ = x[ReturnNull-78]
	_ = x[Nulls-79]
	_ = x[Print-80]
	_ = x[Printf-81]
	_ = x[Getline-82]
	_ = x[GetlineField-83]
	_ = x[GetlineGlobal-84]
	_ = x[GetlineLocal-85]
	_ = x[GetlineSpecial-86]
	_ = x[GetlineArray-87]
	_ = x[EndOpcode-88]
}

const _Opcode_name = "NopNumStrDupeDropSwapAtFieldFieldIntGlobalLocalSpecialArrayGlobalArrayLocalInGlobalInLocalAssignFieldAssignGlobalAssignLocalAssignSpecialAssignArrayGlobalAssignArrayLocalDeleteDeleteAllIncrFieldIncrGlobalIncrLocalIncrSpecialIncrArrayGlobalIncrArrayLocalAugAssignFieldAugAssignGlobalAugAssignLocalAugAssignSpecialAugAssignArrayGlobalAugAssignArrayLocalRegexIndexMultiConcatMultiAddSubtractMultiplyDividePowerModuloEqualsNotEqualsLessGreaterLessOrEqualGreaterOrEqualConcatMatchNotMatchNotUnaryMinusUnaryPlusBooleanJumpJumpFalseJumpTrueJumpEqualsJumpNotEqualsJumpLessJumpGreaterJumpLessOrEqualJumpGreaterOrEqualNextExitForInBreakForInCallBuiltinCallSplitCallSplitSepCallSprintfCallUserCallNativeReturnReturnNullNullsPrintPrintfGetlineGetlineFieldGetlineGlobalGetlineLocalGetlineSpecialGetlineArrayEndOpcode"

var _Opcode_index = [...]uint16{0, 3, 6, 9, 13, 17, 21, 23, 28, 36, 42, 47, 54, 65, 75, 83, 90, 101, 113, 124, 137, 154, 170, 176, 185, 194, 204, 213, 224, 239, 253, 267, 282, 296, 312, 332, 351, 356, 366, 377, 380, 388, 396, 402, 407, 413, 419, 428, 432, 439, 450, 464, 470, 475, 483, 486, 496, 505, 512, 516, 525, 533, 543, 556, 564, 575, 590, 608, 612, 616, 621, 631, 642, 651, 663, 674, 682, 692, 698, 708, 713, 718, 724, 731, 743, 756, 768, 782, 794, 803}

func (i Opcode) String() string {
	if i < 0 || i >= Opcode(len(_Opcode_index)-1) {
		return "Opcode(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Opcode_name[_Opcode_index[i]:_Opcode_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[AugOpAdd-0]
	_ = x[AugOpSub-1]
	_ = x[AugOpMul-2]
	_ = x[AugOpDiv-3]
	_ = x[AugOpPow-4]
	_ = x[AugOpMod-5]
}

const _AugOp_name = "AugOpAddAugOpSubAugOpMulAugOpDivAugOpPowAugOpMod"

var _AugOp_index = [...]uint8{0, 8, 16, 24, 32, 40, 48}

func (i AugOp) String() string {
	if i < 0 || i >= AugOp(len(_AugOp_index)-1) {
		return "AugOp(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _AugOp_name[_AugOp_index[i]:_AugOp_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[BuiltinAtan2-0]
	_ = x[BuiltinClose-1]
	_ = x[BuiltinCos-2]
	_ = x[BuiltinExp-3]
	_ = x[BuiltinFflush-4]
	_ = x[BuiltinFflushAll-5]
	_ = x[BuiltinGsub-6]
	_ = x[BuiltinIndex-7]
	_ = x[BuiltinInt-8]
	_ = x[BuiltinLength-9]
	_ = x[BuiltinLengthArg-10]
	_ = x[BuiltinLog-11]
	_ = x[BuiltinMatch-12]
	_ = x[BuiltinRand-13]
	_ = x[BuiltinSin-14]
	_ = x[BuiltinSqrt-15]
	_ = x[BuiltinSrand-16]
	_ = x[BuiltinSrandSeed-17]
	_ = x[BuiltinSub-18]
	_ = x[BuiltinSubstr-19]
	_ = x[BuiltinSubstrLength-20]
	_ = x[BuiltinSystem-21]
	_ = x[BuiltinTolower-22]
	_ = x[BuiltinToupper-23]
}

const _BuiltinOp_name = "BuiltinAtan2BuiltinCloseBuiltinCosBuiltinExpBuiltinFflushBuiltinFflushAllBuiltinGsubBuiltinIndexBuiltinIntBuiltinLengthBuiltinLengthArgBuiltinLogBuiltinMatchBuiltinRandBuiltinSinBuiltinSqrtBuiltinSrandBuiltinSrandSeedBuiltinSubBuiltinSubstrBuiltinSubstrLengthBuiltinSystemBuiltinTolowerBuiltinToupper"

var _BuiltinOp_index = [...]uint16{0, 12, 24, 34, 44, 57, 73, 84, 96, 106, 119, 135, 145, 157, 168, 178, 189, 201, 217, 227, 240, 259, 272, 286, 300}

func (i BuiltinOp) String() string {
	if i < 0 || i >= BuiltinOp(len(_BuiltinOp_index)-1) {
		return "BuiltinOp(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _BuiltinOp_name[_BuiltinOp_index[i]:_BuiltinOp_index[i+1]]
}
