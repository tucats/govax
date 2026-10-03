package librtl

import "github.com/tucats/govax/internal/corevms"

// Library is the name of the shareable image these routines stand for, as
// a program's image names it and SHIM$<library>_<offset> spells it.
const Library = "LIBRTL"

// Routine is one LIBRTL routine govax provides.
type Routine struct {
	// Name is the routine's universal symbol, e.g. "LIB$GET_VM".
	Name string

	// Offset is the routine's entry in LIBRTL.EXE's transfer vector: what
	// a program's G^ fixup asks for. It must match VMS 7.3's LIBRTL.EXE
	// (as captured in vmsdef.ImageSymbols, which LINK uses); a test checks.
	Offset uint32

	// Code is the XFC$SHIM dispatch code the routine's stub passes in R0.
	// Codes are shared with rtl's own shims (1-38 at Phase 34), so a new
	// routine takes a code no other shim uses; the console's shim test
	// checks they're distinct.
	Code uint32

	// Fn is the routine.
	Fn corevms.ShimFunc
}

// Routines are the LIBRTL routines govax provides. The codes of the ones
// that came from rtl are the ones they had there, so the stubs, and
// anything that names a code, are unchanged.
var Routines = []Routine{
	{"LIB$ADAWI", 0x0A70, 1, libAdawi},
	{"STR$UPCASE", 0x0778, 2, strUpcase},
	{"LIB$GET_VM", 0x0550, 29, libGetVM},
	{"LIB$FREE_VM", 0x0548, 30, libFreeVM},
	{"LIB$DELETE_VM_ZONE", 0x0A48, 31, libDeleteVMZone},
	{"LIB$SIGNAL", 0x04F0, 33, libSignal},
	{"LIB$STOP", 0x04F8, 34, libStop},
	{"LIB$ESTABLISH", 0x03C0, 35, libEstablish},
	{"LIB$REVERT", 0x0490, 36, libRevert},
	{"LIB$SIG_TO_RET", 0x0500, 37, libSigToRet},
	{"LIB$MATCH_COND", 0x0460, 38, libMatchCond},
	{"LIB$CREATE_DIR", 0x0A28, 39, libCreateDir},
}

// Register installs every routine in Routines into t.
func Register(t *corevms.ShimTable) {
	for _, r := range Routines {
		t.Register(r.Code, r.Name, r.Fn)
	}
}

// arg returns argument i of argv, or 0 when the call passed fewer: an
// omitted optional argument, to a routine whose arguments are addresses,
// is the same as a 0 address.
func arg(argv []uint32, i int) uint32 {
	if i < len(argv) {
		return argv[i]
	}

	return 0
}
