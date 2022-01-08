// Code generated by "stringer -type=Op"; DO NOT EDIT.

package bytecode

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Nop-0]
	_ = x[Num-1]
	_ = x[Str-2]
	_ = x[Regex-3]
	_ = x[Dupe-4]
	_ = x[Drop-5]
	_ = x[Field-6]
	_ = x[Global-7]
	_ = x[Local-8]
	_ = x[Special-9]
	_ = x[ArrayGlobal-10]
	_ = x[ArrayLocal-11]
	_ = x[InGlobal-12]
	_ = x[InLocal-13]
	_ = x[AssignField-14]
	_ = x[AssignGlobal-15]
	_ = x[AssignLocal-16]
	_ = x[AssignSpecial-17]
	_ = x[AssignArrayGlobal-18]
	_ = x[AssignArrayLocal-19]
	_ = x[DeleteGlobal-20]
	_ = x[DeleteLocal-21]
	_ = x[DeleteAllGlobal-22]
	_ = x[DeleteAllLocal-23]
	_ = x[PostIncrField-24]
	_ = x[PostIncrGlobal-25]
	_ = x[PostIncrLocal-26]
	_ = x[PostIncrSpecial-27]
	_ = x[PostIncrArrayGlobal-28]
	_ = x[PostIncrArrayLocal-29]
	_ = x[PostDecrField-30]
	_ = x[PostDecrGlobal-31]
	_ = x[PostDecrLocal-32]
	_ = x[PostDecrSpecial-33]
	_ = x[PostDecrArrayGlobal-34]
	_ = x[PostDecrArrayLocal-35]
	_ = x[AugAssignField-36]
	_ = x[AugAssignGlobal-37]
	_ = x[AugAssignLocal-38]
	_ = x[AugAssignSpecial-39]
	_ = x[AugAssignArrayGlobal-40]
	_ = x[AugAssignArrayLocal-41]
	_ = x[Add-42]
	_ = x[Subtract-43]
	_ = x[Multiply-44]
	_ = x[Divide-45]
	_ = x[Power-46]
	_ = x[Modulo-47]
	_ = x[Equals-48]
	_ = x[NotEquals-49]
	_ = x[Less-50]
	_ = x[Greater-51]
	_ = x[LessOrEqual-52]
	_ = x[GreaterOrEqual-53]
	_ = x[Concat-54]
	_ = x[Match-55]
	_ = x[NotMatch-56]
	_ = x[Not-57]
	_ = x[UnaryMinus-58]
	_ = x[UnaryPlus-59]
	_ = x[Jump-60]
	_ = x[JumpFalse-61]
	_ = x[JumpTrue-62]
	_ = x[JumpNumEquals-63]
	_ = x[JumpNumNotEquals-64]
	_ = x[JumpNumLess-65]
	_ = x[JumpNumGreater-66]
	_ = x[JumpNumLessOrEqual-67]
	_ = x[JumpNumGreaterOrEqual-68]
	_ = x[JumpStrEquals-69]
	_ = x[JumpStrNotEquals-70]
	_ = x[Return-71]
	_ = x[Next-72]
	_ = x[Exit-73]
	_ = x[ForGlobalInGlobal-74]
	_ = x[ForGlobalInLocal-75]
	_ = x[ForLocalInGlobal-76]
	_ = x[ForLocalInLocal-77]
	_ = x[ForSpecialInGlobal-78]
	_ = x[ForSpecialInLocal-79]
	_ = x[CallAtan2-80]
	_ = x[CallClose-81]
	_ = x[CallCos-82]
	_ = x[CallExp-83]
	_ = x[CallFflush-84]
	_ = x[CallFflushAll-85]
	_ = x[CallGsub-86]
	_ = x[CallGsubField-87]
	_ = x[CallGsubGlobal-88]
	_ = x[CallGsubLocal-89]
	_ = x[CallGsubSpecial-90]
	_ = x[CallGsubArrayGlobal-91]
	_ = x[CallGsubArrayLocal-92]
	_ = x[CallIndex-93]
	_ = x[CallInt-94]
	_ = x[CallLength-95]
	_ = x[CallLengthArg-96]
	_ = x[CallLog-97]
	_ = x[CallMatch-98]
	_ = x[CallRand-99]
	_ = x[CallSin-100]
	_ = x[CallSplitGlobal-101]
	_ = x[CallSplitLocal-102]
	_ = x[CallSplitSepGlobal-103]
	_ = x[CallSplitSepLocal-104]
	_ = x[CallSprintf-105]
	_ = x[CallSqrt-106]
	_ = x[CallSrand-107]
	_ = x[CallSrandSeed-108]
	_ = x[CallSub-109]
	_ = x[CallSubField-110]
	_ = x[CallSubGlobal-111]
	_ = x[CallSubLocal-112]
	_ = x[CallSubSpecial-113]
	_ = x[CallSubArrayGlobal-114]
	_ = x[CallSubArrayLocal-115]
	_ = x[CallSubstr-116]
	_ = x[CallSubstrLength-117]
	_ = x[CallSystem-118]
	_ = x[CallTolower-119]
	_ = x[CallToupper-120]
	_ = x[CallUser-121]
	_ = x[CallNative-122]
	_ = x[Print-123]
	_ = x[Printf-124]
	_ = x[Getline-125]
	_ = x[GetlineFile-126]
	_ = x[GetlineCommand-127]
}

const _Op_name = "NopNumStrRegexDupeDropFieldGlobalLocalSpecialArrayGlobalArrayLocalInGlobalInLocalAssignFieldAssignGlobalAssignLocalAssignSpecialAssignArrayGlobalAssignArrayLocalDeleteGlobalDeleteLocalDeleteAllGlobalDeleteAllLocalPostIncrFieldPostIncrGlobalPostIncrLocalPostIncrSpecialPostIncrArrayGlobalPostIncrArrayLocalPostDecrFieldPostDecrGlobalPostDecrLocalPostDecrSpecialPostDecrArrayGlobalPostDecrArrayLocalAugAssignFieldAugAssignGlobalAugAssignLocalAugAssignSpecialAugAssignArrayGlobalAugAssignArrayLocalAddSubtractMultiplyDividePowerModuloEqualsNotEqualsLessGreaterLessOrEqualGreaterOrEqualConcatMatchNotMatchNotUnaryMinusUnaryPlusJumpJumpFalseJumpTrueJumpNumEqualsJumpNumNotEqualsJumpNumLessJumpNumGreaterJumpNumLessOrEqualJumpNumGreaterOrEqualJumpStrEqualsJumpStrNotEqualsReturnNextExitForGlobalInGlobalForGlobalInLocalForLocalInGlobalForLocalInLocalForSpecialInGlobalForSpecialInLocalCallAtan2CallCloseCallCosCallExpCallFflushCallFflushAllCallGsubCallGsubFieldCallGsubGlobalCallGsubLocalCallGsubSpecialCallGsubArrayGlobalCallGsubArrayLocalCallIndexCallIntCallLengthCallLengthArgCallLogCallMatchCallRandCallSinCallSplitGlobalCallSplitLocalCallSplitSepGlobalCallSplitSepLocalCallSprintfCallSqrtCallSrandCallSrandSeedCallSubCallSubFieldCallSubGlobalCallSubLocalCallSubSpecialCallSubArrayGlobalCallSubArrayLocalCallSubstrCallSubstrLengthCallSystemCallTolowerCallToupperCallUserCallNativePrintPrintfGetlineGetlineFileGetlineCommand"

var _Op_index = [...]uint16{0, 3, 6, 9, 14, 18, 22, 27, 33, 38, 45, 56, 66, 74, 81, 92, 104, 115, 128, 145, 161, 173, 184, 199, 213, 226, 240, 253, 268, 287, 305, 318, 332, 345, 360, 379, 397, 411, 426, 440, 456, 476, 495, 498, 506, 514, 520, 525, 531, 537, 546, 550, 557, 568, 582, 588, 593, 601, 604, 614, 623, 627, 636, 644, 657, 673, 684, 698, 716, 737, 750, 766, 772, 776, 780, 797, 813, 829, 844, 862, 879, 888, 897, 904, 911, 921, 934, 942, 955, 969, 982, 997, 1016, 1034, 1043, 1050, 1060, 1073, 1080, 1089, 1097, 1104, 1119, 1133, 1151, 1168, 1179, 1187, 1196, 1209, 1216, 1228, 1241, 1253, 1267, 1285, 1302, 1312, 1328, 1338, 1349, 1360, 1368, 1378, 1383, 1389, 1396, 1407, 1421}

func (i Op) String() string {
	if i >= Op(len(_Op_index)-1) {
		return "Op(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Op_name[_Op_index[i]:_Op_index[i+1]]
}
