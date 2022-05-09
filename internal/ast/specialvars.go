// Special variable constants

package ast

import (
	"fmt"
)

const (
	V_ILLEGAL = iota
	V_ARGC
	V_CONVFMT
	V_FILENAME
	V_FNR
	V_FS
	V_INPUTMODE
	V_NF
	V_NR
	V_OFMT
	V_OFS
	V_ORS
	V_OUTPUTMODE
	V_RLENGTH
	V_RS
	V_RSTART
	V_RT
	V_SUBSEP

	V_LAST = V_SUBSEP
)

var specialVars = map[string]int{
	"ARGC":       V_ARGC,
	"CONVFMT":    V_CONVFMT,
	"FILENAME":   V_FILENAME,
	"FNR":        V_FNR,
	"FS":         V_FS,
	"INPUTMODE":  V_INPUTMODE,
	"NF":         V_NF,
	"NR":         V_NR,
	"OFMT":       V_OFMT,
	"OFS":        V_OFS,
	"ORS":        V_ORS,
	"OUTPUTMODE": V_OUTPUTMODE,
	"RLENGTH":    V_RLENGTH,
	"RS":         V_RS,
	"RSTART":     V_RSTART,
	"RT":         V_RT,
	"SUBSEP":     V_SUBSEP,
}

// SpecialVarIndex returns the "index" of the special variable, or 0
// if it's not a special variable.
func SpecialVarIndex(name string) int {
	return specialVars[name]
}

// SpecialVarName returns the name of the special variable by index.
func SpecialVarName(index int) string {
	switch index {
	case V_ILLEGAL:
		return "ILLEGAL"
	case V_ARGC:
		return "ARGC"
	case V_CONVFMT:
		return "CONVFMT"
	case V_FILENAME:
		return "FILENAME"
	case V_FNR:
		return "FNR"
	case V_FS:
		return "FS"
	case V_INPUTMODE:
		return "INPUTMODE"
	case V_NF:
		return "NF"
	case V_NR:
		return "NR"
	case V_OFMT:
		return "OFMT"
	case V_OFS:
		return "OFS"
	case V_ORS:
		return "ORS"
	case V_OUTPUTMODE:
		return "OUTPUTMODE"
	case V_RLENGTH:
		return "RLENGTH"
	case V_RS:
		return "RS"
	case V_RSTART:
		return "RSTART"
	case V_RT:
		return "RT"
	case V_SUBSEP:
		return "SUBSEP"
	default:
		return fmt.Sprintf("<unknown special var %d>", index)
	}
}
