package librtl

import (
	"regexp"
	"slices"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Routines govax lacks (docs/PHASE-53, Decision 6). A program's image
// names each LIBRTL routine it calls by its transfer-vector offset, and
// every one must resolve before the image can run. So every entry of
// LIBRTL.EXE's transfer vector that Routines doesn't provide gets a stub
// too, and a program runs up to its first call of one. That call signals
// SS$_UNSUPPORTED, followed by GOVAX-E-NOTIMPL with the routine's name:
//
//	%SYSTEM-E-UNSUPPORTED, unsupported operation or function
//	-GOVAX-E-NOTIMPL, routine LIB$INDEX is not implemented in govax
//
// The program's handlers see it as any other condition. It's an error,
// not a severe one, so with no handler to take it the catch-all shows the
// message and the program goes on, the call returning SS$_UNSUPPORTED.

// missingCodeBase is the XFC$SHIM code of the first missing routine's
// stub; the rest follow in transfer-vector order. Codes from here up are
// out of the way of the routines govax implements.
const missingCodeBase = 1000

// missingJSBCodeBase is the code of the first missing JSB entry's stub
// (MissingJSB), past every missing CALL entry's.
const missingJSBCodeBase = 2000

var ssUnsupported = vmsdef.Symbols["SS$_UNSUPPORTED"]

// jsbEntry matches the names of LIBRTL's JSB entry points: a routine
// reached with JSB, its arguments in registers, which a CALL stub (whose
// entry mask would run as a HALT) can't stand in for; they get JSB stubs
// (corevms.JSBShimStub). Their names end in the highest register they
// use (_R2, STR$DUPL_CHARR8), or in 6 for the string routines'
// (LIB$SCOPY_DXDX6).
var jsbEntry = regexp.MustCompile(`(R[0-9]+|DX6|DD6)$`)

// dataEntry reports whether name is one of LIBRTL's data tables rather
// than a routine (LIB$AB_UPCASE and its kind), which a stub mustn't stand
// in for: Tables has them.
func dataEntry(name string) bool {
	return strings.HasPrefix(name, "LIB$AB_")
}

// facilityRank orders the names that share an entry (aliases, as
// FOR$CNV_IN_I is OTS$CVT_TI_L): the stub takes the name of the facility
// ranked first, the library's own before the languages'.
var facilityRank = []string{"LIB$", "STR$", "OTS$", "CVT$", "FOR$", "COB$"}

func rank(name string) int {
	for i, f := range facilityRank {
		if strings.HasPrefix(name, f) {
			return i
		}
	}

	return len(facilityRank)
}

// Missing returns a Routine for each CALL entry of LIBRTL.EXE's transfer
// vector that Routines doesn't provide, in transfer-vector order, coded
// from missingCodeBase. Its Fn signals that the routine is missing
// (notImplemented).
func Missing() []Routine {
	names := missingEntries(func(name string) bool { return !jsbEntry.MatchString(name) })
	missing := make([]Routine, len(names))

	for i, name := range names {
		missing[i] = Routine{Name: name, Code: missingCodeBase + uint32(i), Fn: notImplemented(name)}
	}

	return missing
}

// MissingJSB returns a JSBRoutine for each JSB entry of LIBRTL.EXE's
// transfer vector that JSBRoutines doesn't provide (STR$COPY_DX_R8,
// OTS$MOVE3_R5, ...), coded from missingJSBCodeBase. A program that names
// one runs, as with Missing's, until it reaches it; the call then signals
// as a missing CALL entry's does, raised in its caller's frame (a JSB
// routine has none of its own), and if a handler continues it returns
// with R0 SS$_UNSUPPORTED.
func MissingJSB() []JSBRoutine {
	names := missingEntries(jsbEntry.MatchString)
	missing := make([]JSBRoutine, len(names))

	for i, name := range names {
		missing[i] = JSBRoutine{Name: name, Code: missingJSBCodeBase + uint32(i), Fn: notImplementedJSB(name)}
	}

	return missing
}

// missingEntries returns the names of LIBRTL.EXE's transfer-vector
// entries that keep selects and that govax doesn't provide (Routines,
// JSBRoutines, Tables), in offset order: one name per entry, aliases
// (FOR$CNV_IN_I is OTS$CVT_TI_L) giving way to the facility ranked first.
func missingEntries(keep func(name string) bool) []string {
	have := map[uint32]bool{}
	for _, l := range provided() {
		have[l.Offset] = true
	}

	byOffset := map[uint32]string{}

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image != Library || have[sym.Value] || dataEntry(name) || !keep(name) {
			continue
		}

		if other, ok := byOffset[sym.Value]; !ok || rank(name) < rank(other) || rank(name) == rank(other) && name < other {
			byOffset[sym.Value] = name
		}
	}

	offsets := make([]uint32, 0, len(byOffset))
	for off := range byOffset {
		offsets = append(offsets, off)
	}

	slices.Sort(offsets)

	names := make([]string, len(offsets))
	for i, off := range offsets {
		names[i] = byOffset[off]
	}

	return names
}

// notImplemented returns the shim of a routine govax lacks: it signals
// SS$_UNSUPPORTED and GOVAX-E-NOTIMPL with the routine's name, which it
// puts on the stack, in the stub's frame, for the message. If a handler
// continues, the call returns SS$_UNSUPPORTED.
func notImplemented(name string) corevms.ShimFunc {
	return func(env *corevms.Environment, _ []uint32) (uint32, error) {
		desc, err := stackString(env, name)
		if err != nil {
			return memoryFault(env, writing(err))
		}

		return env.SignalFailure([]uint32{ssUnsupported, vmsdef.GovaxNotImplemented, 1, desc}, ssUnsupported)
	}
}

// notImplementedJSB is notImplemented for a JSB entry. The name goes on
// the stack below the JSB's return address; a handler that continues
// resumes with SP as the JSB left it, so the stub's RSB returns.
func notImplementedJSB(name string) corevms.JSBShimFunc {
	return func(env *corevms.Environment, regs *corevms.JSBRegisters) error {
		r0, err := notImplemented(name)(env, nil)
		regs[0] = r0

		return err
	}
}
