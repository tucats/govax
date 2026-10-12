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

var ssUnsupported = vmsdef.Symbols["SS$_UNSUPPORTED"]

// jsbEntry matches the names of LIBRTL's JSB entry points: a routine
// reached with JSB, its arguments in registers, which a stub (a CALL
// entry, whose entry mask would run as a HALT) can't stand in for. Their
// names end in the highest register they use (_R2, STR$DUPL_CHARR8), or
// in 6 for the string routines' (LIB$SCOPY_DXDX6). Phase 54 brings JSB
// entries.
var jsbEntry = regexp.MustCompile(`(R[0-9]+|DX6|DD6)$`)

// dataEntry reports whether name is one of LIBRTL's data tables rather
// than a routine (LIB$AB_UPCASE and its kind), which a stub mustn't stand
// in for either.
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
	have := map[uint32]bool{}
	for _, r := range Routines {
		have[r.Offset()] = true
	}

	byOffset := map[uint32]string{}

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image != Library || have[sym.Value] || jsbEntry.MatchString(name) || dataEntry(name) {
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

	missing := make([]Routine, len(offsets))

	for i, off := range offsets {
		name := byOffset[off]
		missing[i] = Routine{Name: name, Code: missingCodeBase + uint32(i), Fn: notImplemented(name)}
	}

	return missing
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
