package librtl

import (
	"fmt"
	"slices"
	"sync"

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
func (r Routine) Offset() uint32 { return offsetOf(r.Name) }

// offsetOf returns name's entry in LIBRTL.EXE's transfer vector, from
// vmsdef.ImageSymbols, panicking for a name LIBRTL.EXE doesn't have.
func offsetOf(name string) uint32 {
	sym, ok := vmsdef.ImageSymbols[name]
	if !ok || sym.Image != Library {
		panic(fmt.Sprintf("librtl: %s is not in %s's transfer vector", name, Library))
	}

	return sym.Value
}

// JSBRoutine is one LIBRTL routine reached by JSB, its arguments in
// registers (docs/PHASE-54, "The JSB entry points"). Its stub is
// corevms.JSBShimStub, and Code is in the same space as Routine's.
type JSBRoutine struct {
	Name string
	Code uint32
	Fn   corevms.JSBShimFunc
}

// JSBRoutines are the routines reached by JSB that govax provides.
var JSBRoutines = []JSBRoutine{
	{"LIB$SGET1_DD_R6", 82, libSget1DDR6},
	{"LIB$SFREE1_DD6", 83, libSfree1DD6},
	{"LIB$SFREEN_DD6", 84, libSfreenDD6},
	{"LIB$SCOPY_DXDX6", 88, libScopyDxdx6},
	{"LIB$SCOPY_R_DX6", 89, libScopyRDx6},
	{"LIB$ANALYZE_SDESC_R2", 90, libAnalyzeSdescR2},
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
	{"LIB$EXTV", 44, libExtv},
	{"LIB$EXTZV", 45, libExtzv},
	{"LIB$INSV", 46, libInsv},
	{"LIB$FFC", 47, libFfc},
	{"LIB$FFS", 48, libFfs},
	{"LIB$ADDX", 49, libAddx},
	{"LIB$SUBX", 50, libSubx},
	{"LIB$EMUL", 51, libEmul},
	{"LIB$EDIV", 52, libEdiv},
	{"LIB$EMODF", 53, libEmodf},
	{"LIB$EMODD", 54, libEmodd},
	{"LIB$EMODG", 55, libEmodg},
	{"LIB$EMODH", 56, libEmodh},
	{"LIB$POLYF", 57, libPolyf},
	{"LIB$POLYD", 58, libPolyd},
	{"LIB$POLYG", 59, libPolyg},
	{"LIB$POLYH", 60, libPolyh},
	{"LIB$INSQHI", 61, libInsqhi},
	{"LIB$INSQTI", 62, libInsqti},
	{"LIB$REMQHI", 63, libRemqhi},
	// Code 64 is DCL's EXE$CLI_COMMAND (internal/dclshell/subcli.go).
	{"LIB$REMQTI", 65, libRemqti},
	{"LIB$BBSSI", 66, libBbssi},
	{"LIB$BBCCI", 67, libBbcci},
	{"LIB$LOCC", 68, libLocc},
	{"LIB$SKPC", 69, libSkpc},
	{"LIB$MATCHC", 70, libMatchc},
	{"LIB$SCANC", 71, libScanc},
	{"LIB$SPANC", 72, libSpanc},
	{"LIB$MOVC3", 73, libMovc3},
	{"LIB$MOVC5", 74, libMovc5},
	{"LIB$MOVTC", 75, libMovtc},
	{"LIB$MOVTUC", 76, libMovtuc},
	{"LIB$CRC_TABLE", 77, libCrcTable},
	{"LIB$CRC", 78, libCrc},
	{"LIB$SGET1_DD", 79, libSget1DD},
	{"LIB$SFREE1_DD", 80, libSfree1DD},
	{"LIB$SFREEN_DD", 81, libSfreenDD},
	{"LIB$SCOPY_DXDX", 85, libScopyDxdx},
	{"LIB$SCOPY_R_DX", 86, libScopyRDx},
	{"LIB$ANALYZE_SDESC", 87, libAnalyzeSdesc},
	{"LIB$LEN", 91, libLen},
	{"LIB$CHAR", 92, libChar},
	{"LIB$ICHAR", 93, libIchar},
	{"LIB$INDEX", 94, libIndex},
	{"LIB$TRA_ASC_EBC", 95, libTraAscEbc},
	{"LIB$TRA_EBC_ASC", 96, libTraEbcAsc},
	{"LIB$CVT_DTB", 97, libCvtDtb},
	{"LIB$CVT_HTB", 98, libCvtHtb},
	{"LIB$CVT_OTB", 99, libCvtOtb},
	{"LIB$LOOKUP_KEY", 100, libLookupKey},
	{"LIB$DIGIT_SEP", 101, libDigitSep},
	{"LIB$RADIX_POINT", 102, libRadixPoint},
	{"LIB$CURRENCY", 103, libCurrency},
	{"LIB$LP_LINES", 104, libLpLines},
}

// missing is Missing's list, and missingJSB MissingJSB's, made once.
var (
	missing    = sync.OnceValue(Missing)
	missingJSB = sync.OnceValue(MissingJSB)
)

// all is every routine reached by CALL with a stub or a home in
// kernel.asm: Routines, then the ones govax lacks (Missing).
func all() []Routine {
	return slices.Concat(Routines, missing())
}

// allJSB is every routine reached by JSB: JSBRoutines, then the ones
// govax lacks (MissingJSB).
func allJSB() []JSBRoutine {
	return slices.Concat(JSBRoutines, missingJSB())
}

// Register installs every routine written in Go into t: Routines' and
// JSBRoutines', and the shims of the routines govax lacks.
func Register(t *corevms.ShimTable) {
	for _, r := range all() {
		if r.Fn != nil {
			t.Register(r.Code, r.Name, r.Fn)
		}
	}

	for _, r := range allJSB() {
		t.RegisterJSB(r.Code, r.Name, r.Fn)
	}
}

// provided are the places in LIBRTL.EXE of everything govax provides
// itself: Routines, JSBRoutines, and Tables.
func provided() []corevms.ShimLink {
	links := make([]corevms.ShimLink, 0, len(Routines)+len(JSBRoutines)+len(Tables))

	for _, r := range Routines {
		links = append(links, corevms.ShimLink{Name: r.Name, Library: Library, Offset: r.Offset(), Code: r.Code})
	}

	for _, r := range JSBRoutines {
		links = append(links, corevms.ShimLink{Name: r.Name, Library: Library, Offset: offsetOf(r.Name), Code: r.Code, JSB: true})
	}

	for _, t := range Tables {
		links = append(links, corevms.ShimLink{Name: t.Name, Library: Library, Offset: offsetOf(t.Name), Data: t.Data[:]})
	}

	return links
}

// Links are the places in LIBRTL.EXE of every routine Register installs
// and of the ones in kernel.asm, for their stubs, and of the data tables
// (corevms.ShimLink). Routines come first, in their table's order, so the
// first stubs (the ones in VMINIT's page) keep their addresses.
func Links() []corevms.ShimLink {
	links := provided()

	for _, r := range missing() {
		links = append(links, corevms.ShimLink{Name: r.Name, Library: Library, Offset: r.Offset(), Code: r.Code})
	}

	for _, r := range missingJSB() {
		links = append(links, corevms.ShimLink{Name: r.Name, Library: Library, Offset: offsetOf(r.Name), Code: r.Code, JSB: true})
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
