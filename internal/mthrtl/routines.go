package mthrtl

import (
	"fmt"
	"regexp"
	"slices"
	"sync"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// Library is the name of the shareable image these routines stand for, as
// a program's image names it and SHIM$<library>_<offset> spells it.
const Library = "MTHRTL"

// codeBase is the XFC$SHIM code of the transfer vector's first entry.
// LIBRTL's codes (its missing JSB entries start at 2000) and the traceback
// handler's (3000) are below it, and the stubs the image activator makes
// for other libraries' routines are far above it (machine's
// unresolvedCodeBase).
const codeBase = 4000

// entrySize is the size of one transfer vector entry, in bytes. A CALL
// entry is a 2-byte entry mask and a 6-byte JMP, a JSB entry a JMP and two
// bytes of padding.
const entrySize = 8

// Routine is one routine reached by CALLS or CALLG. Its code isn't written
// here: it follows from the routine's place in the transfer vector
// (codeOf), so the tables can't disagree with the vector, and a routine
// listed under either of two names that share an entry (MTH$IINT is
// MTH$IIFIX) is the same routine.
type Routine struct {
	// Name is the routine's universal symbol, which must be one of
	// MTHRTL.EXE's.
	Name string

	// Fn is the routine.
	Fn corevms.ShimFunc
}

// JSBRoutine is one routine reached by JSB, its arguments in registers.
type JSBRoutine struct {
	Name string
	Fn   corevms.JSBShimFunc
}

// Routines are the routines reached by CALL that govax provides. The
// transfer vector's other CALL entries signal NOTIMPL.
var Routines = []Routine{
	{"MTH$SQRT", callEntry(vaxfloat.F, squareRoot)},
	{"MTH$DSQRT", callEntry(vaxfloat.D, squareRoot)},
	{"MTH$GSQRT", callEntry(vaxfloat.G, squareRoot)},
	{"MTH$HSQRT", callEntry(vaxfloat.H, squareRoot)},

	{"MTH$EXP", callEntry(vaxfloat.F, exp)},
	{"MTH$DEXP", callEntry(vaxfloat.D, exp)},
	{"MTH$GEXP", callEntry(vaxfloat.G, exp)},
	{"MTH$HEXP", callEntry(vaxfloat.H, exp)},

	{"MTH$ALOG", callEntry(vaxfloat.F, naturalLog)},
	{"MTH$DLOG", callEntry(vaxfloat.D, naturalLog)},
	{"MTH$GLOG", callEntry(vaxfloat.G, naturalLog)},
	{"MTH$HLOG", callEntry(vaxfloat.H, naturalLog)},

	{"MTH$ALOG2", callEntry(vaxfloat.F, base2Log)},
	{"MTH$DLOG2", callEntry(vaxfloat.D, base2Log)},
	{"MTH$GLOG2", callEntry(vaxfloat.G, base2Log)},
	{"MTH$HLOG2", callEntry(vaxfloat.H, base2Log)},

	{"MTH$ALOG10", callEntry(vaxfloat.F, base10Log)},
	{"MTH$DLOG10", callEntry(vaxfloat.D, base10Log)},
	{"MTH$GLOG10", callEntry(vaxfloat.G, base10Log)},
	{"MTH$HLOG10", callEntry(vaxfloat.H, base10Log)},

	{"MTH$SIN", callEntry(vaxfloat.F, sine)},
	{"MTH$DSIN", callEntry(vaxfloat.D, sine)},
	{"MTH$GSIN", callEntry(vaxfloat.G, sine)},
	{"MTH$HSIN", callEntry(vaxfloat.H, sine)},

	{"MTH$COS", callEntry(vaxfloat.F, cosine)},
	{"MTH$DCOS", callEntry(vaxfloat.D, cosine)},
	{"MTH$GCOS", callEntry(vaxfloat.G, cosine)},
	{"MTH$HCOS", callEntry(vaxfloat.H, cosine)},

	{"MTH$TAN", callEntry(vaxfloat.F, tangent)},
	{"MTH$DTAN", callEntry(vaxfloat.D, tangent)},
	{"MTH$GTAN", callEntry(vaxfloat.G, tangent)},
	{"MTH$HTAN", callEntry(vaxfloat.H, tangent)},

	{"MTH$SINCOS", callPairEntry(vaxfloat.F, sineCosine)},
	{"MTH$DSINCOS", callPairEntry(vaxfloat.D, sineCosine)},
	{"MTH$GSINCOS", callPairEntry(vaxfloat.G, sineCosine)},
	{"MTH$HSINCOS", callPairEntry(vaxfloat.H, sineCosine)},

	{"MTH$SIND", callEntry(vaxfloat.F, sineDegrees)},
	{"MTH$DSIND", callEntry(vaxfloat.D, sineDegrees)},
	{"MTH$GSIND", callEntry(vaxfloat.G, sineDegrees)},
	{"MTH$HSIND", callEntry(vaxfloat.H, sineDegrees)},

	{"MTH$COSD", callEntry(vaxfloat.F, cosineDegrees)},
	{"MTH$DCOSD", callEntry(vaxfloat.D, cosineDegrees)},
	{"MTH$GCOSD", callEntry(vaxfloat.G, cosineDegrees)},
	{"MTH$HCOSD", callEntry(vaxfloat.H, cosineDegrees)},

	{"MTH$TAND", callEntry(vaxfloat.F, tangentDegrees)},
	{"MTH$DTAND", callEntry(vaxfloat.D, tangentDegrees)},
	{"MTH$GTAND", callEntry(vaxfloat.G, tangentDegrees)},
	{"MTH$HTAND", callEntry(vaxfloat.H, tangentDegrees)},

	{"MTH$SINCOSD", callPairEntry(vaxfloat.F, sineCosineDegrees)},
	{"MTH$DSINCOSD", callPairEntry(vaxfloat.D, sineCosineDegrees)},
	{"MTH$GSINCOSD", callPairEntry(vaxfloat.G, sineCosineDegrees)},
	{"MTH$HSINCOSD", callPairEntry(vaxfloat.H, sineCosineDegrees)},

	{"MTH$ASIN", callEntry(vaxfloat.F, arcSine)},
	{"MTH$DASIN", callEntry(vaxfloat.D, arcSine)},
	{"MTH$GASIN", callEntry(vaxfloat.G, arcSine)},
	{"MTH$HASIN", callEntry(vaxfloat.H, arcSine)},

	{"MTH$ACOS", callEntry(vaxfloat.F, arcCosine)},
	{"MTH$DACOS", callEntry(vaxfloat.D, arcCosine)},
	{"MTH$GACOS", callEntry(vaxfloat.G, arcCosine)},
	{"MTH$HACOS", callEntry(vaxfloat.H, arcCosine)},

	{"MTH$ATAN", callEntry(vaxfloat.F, arcTangent)},
	{"MTH$DATAN", callEntry(vaxfloat.D, arcTangent)},
	{"MTH$GATAN", callEntry(vaxfloat.G, arcTangent)},
	{"MTH$HATAN", callEntry(vaxfloat.H, arcTangent)},

	{"MTH$ASIND", callEntry(vaxfloat.F, arcSineDegrees)},
	{"MTH$DASIND", callEntry(vaxfloat.D, arcSineDegrees)},
	{"MTH$GASIND", callEntry(vaxfloat.G, arcSineDegrees)},
	{"MTH$HASIND", callEntry(vaxfloat.H, arcSineDegrees)},

	{"MTH$ACOSD", callEntry(vaxfloat.F, arcCosineDegrees)},
	{"MTH$DACOSD", callEntry(vaxfloat.D, arcCosineDegrees)},
	{"MTH$GACOSD", callEntry(vaxfloat.G, arcCosineDegrees)},
	{"MTH$HACOSD", callEntry(vaxfloat.H, arcCosineDegrees)},

	{"MTH$ATAND", callEntry(vaxfloat.F, arcTangentDegrees)},
	{"MTH$DATAND", callEntry(vaxfloat.D, arcTangentDegrees)},
	{"MTH$GATAND", callEntry(vaxfloat.G, arcTangentDegrees)},
	{"MTH$HATAND", callEntry(vaxfloat.H, arcTangentDegrees)},
}

// JSBRoutines are the routines reached by JSB that govax provides. The
// vector's other JSB entries signal NOTIMPL, among them MTH$SQRT_R2,
// MTH$HTAN_R7, and MTH$HTAND_R7, which no manual gives and whose registers wait
// for the VMS probe (Decision 7). A name that has a neighbor with a smaller
// register number, such as MTH$DEXP_R7 beside MTH$DEXP_R6 or MTH$ASIN_R5 beside
// MTH$ASIN_R4, is a second entry: in VMS's image the two jump to the same
// address.
var JSBRoutines = []JSBRoutine{
	{"MTH$SQRT_R3", jsbEntry(vaxfloat.F, squareRoot)},
	{"MTH$DSQRT_R5", jsbEntry(vaxfloat.D, squareRoot)},
	{"MTH$GSQRT_R5", jsbEntry(vaxfloat.G, squareRoot)},
	{"MTH$HSQRT_R8", jsbEntry(vaxfloat.H, squareRoot)},

	{"MTH$EXP_R4", jsbEntry(vaxfloat.F, exp)},
	{"MTH$DEXP_R6", jsbEntry(vaxfloat.D, exp)},
	{"MTH$DEXP_R7", jsbEntry(vaxfloat.D, exp)},
	{"MTH$GEXP_R6", jsbEntry(vaxfloat.G, exp)},
	{"MTH$GEXP_R7", jsbEntry(vaxfloat.G, exp)},
	{"MTH$HEXP_R6", jsbEntry(vaxfloat.H, exp)},

	{"MTH$ALOG_R5", jsbEntry(vaxfloat.F, naturalLog)},
	{"MTH$DLOG_R8", jsbEntry(vaxfloat.D, naturalLog)},
	{"MTH$GLOG_R8", jsbEntry(vaxfloat.G, naturalLog)},
	{"MTH$HLOG_R8", jsbEntry(vaxfloat.H, naturalLog)},

	{"MTH$ALOG10_R5", jsbEntry(vaxfloat.F, base10Log)},
	{"MTH$DLOG10_R8", jsbEntry(vaxfloat.D, base10Log)},
	{"MTH$GLOG10_R8", jsbEntry(vaxfloat.G, base10Log)},
	{"MTH$HLOG10_R8", jsbEntry(vaxfloat.H, base10Log)},

	{"MTH$SIN_R4", jsbEntry(vaxfloat.F, sine)},
	{"MTH$DSIN_R7", jsbEntry(vaxfloat.D, sine)},
	{"MTH$GSIN_R7", jsbEntry(vaxfloat.G, sine)},
	{"MTH$HSIN_R5", jsbEntry(vaxfloat.H, sine)},

	{"MTH$COS_R4", jsbEntry(vaxfloat.F, cosine)},
	{"MTH$DCOS_R7", jsbEntry(vaxfloat.D, cosine)},
	{"MTH$GCOS_R7", jsbEntry(vaxfloat.G, cosine)},
	{"MTH$HCOS_R5", jsbEntry(vaxfloat.H, cosine)},

	{"MTH$TAN_R4", jsbEntry(vaxfloat.F, tangent)},
	{"MTH$TAN_R5", jsbEntry(vaxfloat.F, tangent)},
	{"MTH$DTAN_R7", jsbEntry(vaxfloat.D, tangent)},
	{"MTH$GTAN_R7", jsbEntry(vaxfloat.G, tangent)},
	{"MTH$HTAN_R5", jsbEntry(vaxfloat.H, tangent)},

	{"MTH$SINCOS_R5", jsbPairEntry(vaxfloat.F, sineCosine)},
	{"MTH$DSINCOS_R7", jsbPairEntry(vaxfloat.D, sineCosine)},
	{"MTH$GSINCOS_R7", jsbPairEntry(vaxfloat.G, sineCosine)},
	{"MTH$HSINCOS_R7", jsbPairEntry(vaxfloat.H, sineCosine)},

	{"MTH$SIND_R4", jsbEntry(vaxfloat.F, sineDegrees)},
	{"MTH$DSIND_R7", jsbEntry(vaxfloat.D, sineDegrees)},
	{"MTH$GSIND_R7", jsbEntry(vaxfloat.G, sineDegrees)},
	{"MTH$HSIND_R5", jsbEntry(vaxfloat.H, sineDegrees)},

	{"MTH$COSD_R4", jsbEntry(vaxfloat.F, cosineDegrees)},
	{"MTH$DCOSD_R7", jsbEntry(vaxfloat.D, cosineDegrees)},
	{"MTH$GCOSD_R7", jsbEntry(vaxfloat.G, cosineDegrees)},
	{"MTH$HCOSD_R5", jsbEntry(vaxfloat.H, cosineDegrees)},

	{"MTH$TAND_R4", jsbEntry(vaxfloat.F, tangentDegrees)},
	{"MTH$TAND_R5", jsbEntry(vaxfloat.F, tangentDegrees)},
	{"MTH$DTAND_R7", jsbEntry(vaxfloat.D, tangentDegrees)},
	{"MTH$GTAND_R7", jsbEntry(vaxfloat.G, tangentDegrees)},
	{"MTH$HTAND_R5", jsbEntry(vaxfloat.H, tangentDegrees)},

	{"MTH$SINCOSD_R5", jsbPairEntry(vaxfloat.F, sineCosineDegrees)},
	{"MTH$DSINCOSD_R7", jsbPairEntry(vaxfloat.D, sineCosineDegrees)},
	{"MTH$GSINCOSD_R7", jsbPairEntry(vaxfloat.G, sineCosineDegrees)},
	{"MTH$HSINCOSD_R7", jsbPairEntry(vaxfloat.H, sineCosineDegrees)},

	{"MTH$ASIN_R4", jsbEntry(vaxfloat.F, arcSine)},
	{"MTH$ASIN_R5", jsbEntry(vaxfloat.F, arcSine)},
	{"MTH$DASIN_R7", jsbEntry(vaxfloat.D, arcSine)},
	{"MTH$DASIN_R9", jsbEntry(vaxfloat.D, arcSine)},
	{"MTH$GASIN_R7", jsbEntry(vaxfloat.G, arcSine)},
	{"MTH$GASIN_R9", jsbEntry(vaxfloat.G, arcSine)},
	{"MTH$HASIN_R8", jsbEntry(vaxfloat.H, arcSine)},

	{"MTH$ACOS_R4", jsbEntry(vaxfloat.F, arcCosine)},
	{"MTH$ACOS_R5", jsbEntry(vaxfloat.F, arcCosine)},
	{"MTH$DACOS_R7", jsbEntry(vaxfloat.D, arcCosine)},
	{"MTH$DACOS_R9", jsbEntry(vaxfloat.D, arcCosine)},
	{"MTH$GACOS_R7", jsbEntry(vaxfloat.G, arcCosine)},
	{"MTH$GACOS_R9", jsbEntry(vaxfloat.G, arcCosine)},
	{"MTH$HACOS_R8", jsbEntry(vaxfloat.H, arcCosine)},

	{"MTH$ATAN_R4", jsbEntry(vaxfloat.F, arcTangent)},
	{"MTH$DATAN_R7", jsbEntry(vaxfloat.D, arcTangent)},
	{"MTH$GATAN_R7", jsbEntry(vaxfloat.G, arcTangent)},
	{"MTH$HATAN_R8", jsbEntry(vaxfloat.H, arcTangent)},

	{"MTH$ASIND_R4", jsbEntry(vaxfloat.F, arcSineDegrees)},
	{"MTH$DASIND_R7", jsbEntry(vaxfloat.D, arcSineDegrees)},
	{"MTH$GASIND_R7", jsbEntry(vaxfloat.G, arcSineDegrees)},
	{"MTH$HASIND_R8", jsbEntry(vaxfloat.H, arcSineDegrees)},

	{"MTH$ACOSD_R4", jsbEntry(vaxfloat.F, arcCosineDegrees)},
	{"MTH$DACOSD_R7", jsbEntry(vaxfloat.D, arcCosineDegrees)},
	{"MTH$GACOSD_R7", jsbEntry(vaxfloat.G, arcCosineDegrees)},
	{"MTH$HACOSD_R8", jsbEntry(vaxfloat.H, arcCosineDegrees)},

	{"MTH$ATAND_R4", jsbEntry(vaxfloat.F, arcTangentDegrees)},
	{"MTH$DATAND_R7", jsbEntry(vaxfloat.D, arcTangentDegrees)},
	{"MTH$GATAND_R7", jsbEntry(vaxfloat.G, arcTangentDegrees)},
	{"MTH$HATAND_R8", jsbEntry(vaxfloat.H, arcTangentDegrees)},
}

// jsbName matches the names of the JSB entries: a name like MTH$SIN_R4 is
// reached by JSB. OTS$POWHH_R3 is not: it's a CALL entry whose result comes
// back in R0:R3 (the survey read each entry's first bytes, and this rule
// holds for all 404 names).
var jsbName = regexp.MustCompile(`^MTH\$.*_R[0-9]+$`)

// entry is one entry of the transfer vector.
type entry struct {
	name   string // the first name in alphabetical order, if it has two
	offset uint32
	jsb    bool
}

// entries are the transfer vector's entries, in offset order. It's a
// function made once, on first use (sync.OnceValue), since a new System
// asks for it each time and the answer can't change.
var entries = sync.OnceValue(func() []entry {
	byOffset := map[uint32]string{}

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image != Library {
			continue
		}

		if other, ok := byOffset[sym.Value]; !ok || name < other {
			byOffset[sym.Value] = name
		}
	}

	list := make([]entry, 0, len(byOffset))
	for offset, name := range byOffset {
		list = append(list, entry{name: name, offset: offset, jsb: jsbName.MatchString(name)})
	}

	slices.SortFunc(list, func(a, b entry) int { return int(a.offset) - int(b.offset) })

	return list
})

// codeOf returns the XFC$SHIM code of name's stub. It panics for a name
// MTHRTL.EXE doesn't have, a mistake in the tables that TestTables reports
// by name.
func codeOf(name string) uint32 {
	sym, ok := vmsdef.ImageSymbols[name]
	if !ok || sym.Image != Library {
		panic(fmt.Sprintf("mthrtl: %s is not in %s's transfer vector", name, Library))
	}

	return codeBase + sym.Value/entrySize
}

// Register installs every entry's routine in t: the tables' routines, and
// for each other entry one that signals NOTIMPL. The console does this for
// each new System.
func Register(t *corevms.ShimTable) {
	for _, e := range entries() {
		code := codeBase + e.offset/entrySize

		if e.jsb {
			t.RegisterJSB(code, e.name, librtl.NotImplementedJSB(e.name))
		} else {
			t.Register(code, e.name, librtl.NotImplemented(e.name))
		}
	}

	for _, r := range Routines {
		t.Register(codeOf(r.Name), r.Name, r.Fn)
	}

	for _, r := range JSBRoutines {
		t.RegisterJSB(codeOf(r.Name), r.Name, r.Fn)
	}
}

// Links are the transfer vector's entries as corevms.ShimLinks, for their
// stubs: one for each of the 402 entries, whatever the table holds, in
// offset order, so a stub's address doesn't change when its routine is
// written. Two names that share an entry have one stub, under the name
// that sorts first.
func Links() []corevms.ShimLink {
	list := entries()
	links := make([]corevms.ShimLink, len(list))

	for i, e := range list {
		links[i] = corevms.ShimLink{
			Name: e.name, Library: Library, Offset: e.offset,
			Code: codeBase + e.offset/entrySize, JSB: e.jsb,
		}
	}

	return links
}

// arg returns argument i of argv, or 0 when the call passed fewer. The
// MTH$ routines' arguments are addresses, and 0 is one no program can
// reach, so a missing argument is an access violation, as on VMS.
func arg(argv []uint32, i int) uint32 {
	if i < len(argv) {
		return argv[i]
	}

	return 0
}
