package librtl

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Library is the name of the shareable image these routines stand for, as
// a program's image names it and SHIM$<library>_<offset> spells it.
const Library = "LIBRTL"

// Routine is one LIBRTL routine govax provides.
type Routine struct {
	// Name is the routine's universal symbol, e.g. "LIB$GET_VM". Its
	// entry in LIBRTL.EXE's transfer vector -- the offset a program's G^
	// fixup asks for -- is VMS 7.3's, from vmsdef.ImageSymbols (which LINK
	// uses too), so every name must be one LIBRTL.EXE has (Offset).
	Name string

	// Code is the XFC$SHIM dispatch code the routine's stub passes in R0.
	// Codes are shared with corevms's own shims (CRTLLinks), so a new
	// routine takes the next code no other shim uses; tests check they're
	// distinct.
	//
	// Code 0 means the routine is written in VAX code in the microkernel
	// (kernel.asm), under its own name, rather than in Go: the stub is
	// the routine itself (docs/PHASE-53, Decision 1). Fn is then nil.
	Code uint32

	// Fn is the routine.
	Fn corevms.ShimFunc
}

// Offset returns the routine's entry in LIBRTL.EXE's transfer vector.
// It panics for a name LIBRTL.EXE doesn't have, a mistake in Routines
// that TestRoutinesMatchLIBRTL reports by name.
func (r Routine) Offset() uint32 {
	sym, ok := vmsdef.ImageSymbols[r.Name]
	if !ok || sym.Image != Library {
		panic(fmt.Sprintf("librtl: %s is not in %s's transfer vector", r.Name, Library))
	}

	return sym.Value
}

// Routines are the LIBRTL routines govax provides. The codes of the ones
// that came from rtl are the ones they had there, so the stubs, and
// anything that names a code, are unchanged.
var Routines = []Routine{
	{"LIB$ADAWI", 1, libAdawi},
	{"STR$UPCASE", 2, strUpcase},
	{"LIB$GET_VM", 29, libGetVM},
	{"LIB$FREE_VM", 30, libFreeVM},
	{"LIB$DELETE_VM_ZONE", 31, libDeleteVMZone},
	{"LIB$SIGNAL", 33, libSignal},
	{"LIB$STOP", 34, libStop},
	{"LIB$ESTABLISH", 35, libEstablish},
	{"LIB$REVERT", 36, libRevert},
	{"LIB$SIG_TO_RET", 37, libSigToRet},
	{"LIB$MATCH_COND", 38, libMatchCond},
	{"LIB$CREATE_DIR", 39, libCreateDir},
	{"LIB$PUT_OUTPUT", 40, libPutOutput},
	{"LIB$GET_FOREIGN", 41, libGetForeign},
	{"LIB$GET_INPUT", 42, libGetInput},
	{"LIB$SPAWN", 43, libSpawn},
	{"LIB$CALLG", 0, nil}, // kernel.asm
}

// Register installs every routine in Routines written in Go into t.
func Register(t *corevms.ShimTable) {
	for _, r := range Routines {
		if r.Fn != nil {
			t.Register(r.Code, r.Name, r.Fn)
		}
	}
}

// Links are the routines' places in LIBRTL.EXE, for their stubs
// (corevms.ShimLink).
func Links() []corevms.ShimLink {
	links := make([]corevms.ShimLink, 0, len(Routines))

	for _, r := range Routines {
		links = append(links, corevms.ShimLink{Name: r.Name, Library: Library, Offset: r.Offset(), Code: r.Code})
	}

	return links
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
