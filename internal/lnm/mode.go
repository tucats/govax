package lnm

import "github.com/tucats/govax/internal/vmsdef"

// Mode is a VAX access mode, numbered as in PSL<CUR_MOD> and $PSLDEF:
// Kernel is the most privileged (innermost), User the least (outermost).
type Mode uint8

const (
	Kernel Mode = iota
	Executive
	Supervisor
	User
)

// String returns the abbreviation SHOW LOGICAL/FULL prints for m.
func (m Mode) String() string {
	switch m {
	case Kernel:
		return "kernel"
	case Executive:
		return "exec"
	case Supervisor:
		return "super"
	default:
		return "user"
	}
}

// Attribute bits, from the real $LNMDEF ($CRELNM/$CRELNT/$TRNLNM attr
// arguments, and the LNM$_ATTRIBUTES item).
var (
	AttrNoAlias   = vmsdef.Symbols["LNM$M_NO_ALIAS"]
	AttrConfine   = vmsdef.Symbols["LNM$M_CONFINE"]
	AttrCrelog    = vmsdef.Symbols["LNM$M_CRELOG"]
	AttrTable     = vmsdef.Symbols["LNM$M_TABLE"]
	AttrConcealed = vmsdef.Symbols["LNM$M_CONCEALED"]
	AttrTerminal  = vmsdef.Symbols["LNM$M_TERMINAL"]
	AttrExists    = vmsdef.Symbols["LNM$M_EXISTS"]
	AttrShareable = vmsdef.Symbols["LNM$M_SHAREABLE"]
	AttrCreateIf  = vmsdef.Symbols["LNM$M_CREATE_IF"]
	AttrCaseBlind = vmsdef.Symbols["LNM$M_CASE_BLIND"]
)

// Limits, from $LNMDEF and the $CRELNM description.
var (
	// MaxNameLength is the longest logical name or equivalence string
	// (LNM$C_NAMLENGTH).
	MaxNameLength = int(vmsdef.Symbols["LNM$C_NAMLENGTH"])

	// MaxTableNameLength is the longest name a directory table may hold
	// (LNM$C_TABNAMLEN), which bounds every table name.
	MaxTableNameLength = int(vmsdef.Symbols["LNM$C_TABNAMLEN"])

	// MaxDepth is how many levels of iterative translation are allowed
	// before SS$_TOOMANYLNAM (LNM$C_MAXDEPTH).
	MaxDepth = int(vmsdef.Symbols["LNM$C_MAXDEPTH"])
)

// MaxEquivalences is how many equivalence strings one logical name may
// have: $CRELNM assigns each an index from 0 to 127. $LNMDEF has no
// constant for it.
const MaxEquivalences = 128
