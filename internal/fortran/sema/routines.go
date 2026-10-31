package sema

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
)

// The routines behind the intrinsic functions: the one a reference calls,
// when the function isn't written in line, and the one an intrinsic's name
// passes as an actual argument.
//
// Most are MTHRTL's, the VMS mathematics library, named MTH$ and the
// function's specific name (MTH$SQRT, MTH$JIABS), with the floating format
// in place of the name's own letter where they differ (Phase 78's survey,
// "The intrinsic functions"): under /G_FLOATING a REAL*8 function's D is G
// (DSQRT is MTH$GSQRT, CDSQRT MTH$CGSQRT, IIDINT MTH$IIGINT; DBLE and SNGL
// are MTH$GDBLE and MTH$SNGLG), and a REAL*16 function's Q is always H
// (QSQRT is MTH$HSQRT, JIQINT MTH$JIHINT). ATAN2D's routines are ATAND2's
// (MTH$ATAND2, MTH$DATAND2).
//
// MTHRTL has no routine for some names, which are written in line when they
// are referred to. A name of these passed as an argument is FTNRTL's routine
// of its name under FTN$ (the author's answer 4 in Phase 78's survey): FTNRTL
// is govax's FORTRAN run-time library, and has one for each of them.

// ftnRoutines are the names whose routines are FTNRTL's, not MTHRTL's.
// QFLOAT, a generic name with no specific one, and NWORKERS, a constant, are
// among them, so that every function's name but SIZEOF can be passed.
var ftnRoutines = setOf(`LEN INDEX ICHAR CHAR LLT LLE LGT LGE BITEST BJTEST
	IIBITS JIBITS IIBSET JIBSET IIBCLR JIBCLR IISHFTC JISHFTC IZEXT JZEXT
	SNGLQ DBLEQ QEXT QEXTD QFLOAT NWORKERS`)

// routineOf is the routine the row f calls in the unit's format.
func (a *analyzer) routineOf(f form) string {
	return routineName(f, a.gFloating)
}

// routineName is the routine of the row f: in G_floating's form if g is set.
func routineName(f form, g bool) string {
	name := f.specific
	if name == "" {
		name = f.generic // CMPLX, DCMPLX, QFLOAT
	}

	if ftnRoutines[name] {
		if g {
			switch name {
			case "DBLEQ":
				name = "GBLEQ"
			case "QEXTD":
				name = "QEXTG"
			}
		}

		return "FTN$" + name
	}

	name = strings.Replace(name, "ATAN2D", "ATAND2", 1)

	// The letter after a word or longword INTEGER's "II" or "JI" is the
	// argument's format, as in IIDINT and JIQNNT.
	integerOf := strings.HasPrefix(name, "II") || strings.HasPrefix(name, "JI")

	switch {
	case f.uses(Type{Real, 16}):
		switch {
		case name[0] == 'Q':
			name = "H" + name[1:]
		case integerOf && name[2] == 'Q':
			name = name[:2] + "H" + name[3:]
		}

	case g && (f.uses(Type{Real, 8}) || f.uses(Type{Complex, 16})):
		switch {
		case name == "DBLE":
			name = "GDBLE"
		case name == "SNGL":
			name = "SNGLG"
		case strings.HasPrefix(name, "CD"):
			name = "CG" + name[2:]
		case name[0] == 'D':
			name = "G" + name[1:]
		case integerOf && name[2] == 'D':
			name = name[:2] + "G" + name[3:]
		}
	}

	return "MTH$" + name
}

// uses reports whether the row takes or returns a value of type t.
func (f form) uses(t Type) bool {
	return f.arg == t || f.result == t
}

// procedure is the intrinsic function n, named in an INTRINSIC statement,
// given as an actual argument (section 4.9): its routine, which the routine
// called may call in its turn. SIZEOF has no routine (NOTPASSABLE).
func (a *analyzer) procedure(n *ast.Name) Expr {
	if n.Name == "SIZEOF" {
		a.report("NOTPASSABLE", n.Pos, n.Name)

		return a.bad(n.Pos)
	}

	f := a.passedForm(n.Name)

	return &Procedure{exprBase: exprBase{f.result, n.Pos}, Name: n.Name, Routine: a.routineOf(f)}
}

// passedForm is the row of Table D-3 the name passed as an argument stands
// for. A generic name passed "is treated according to the rules for handling
// specific" names (section 6.3.1.1), so a name that is also a specific name is
// that one (ABS is ABS, of a REAL), and another stands for its specific name
// of the default types: of an argument of the unit's default integer, or else
// of a REAL*4, with a result of the default integer if it is an INTEGER at all
// (IABS is JIABS, MAX JMAX0, LOG ALOG, INT JINT, under /NOI4 IIABS, IMAX0,
// ALOG, IINT; unconfirmed). CMPLX, DCMPLX, and QFLOAT have no specific name,
// and stand for themselves.
func (a *analyzer) passedForm(name string) form {
	var named []form

	for _, f := range intrinsicForms {
		if f.specific == name {
			return f
		}

		if f.generic == name && f.specific != "" {
			named = append(named, f)
		}
	}

	if len(named) == 0 {
		return formsNamed(name)[0]
	}

	candidates := named[:0:0]

	for _, f := range named {
		if f.result.Kind != Integer || f.result == a.defaultInt {
			candidates = append(candidates, f)
		}
	}

	if len(candidates) == 0 {
		candidates = named
	}

	for _, t := range []Type{a.defaultInt, {Real, 4}} {
		for _, f := range candidates {
			if f.arg == t {
				return f
			}
		}
	}

	return candidates[0]
}

// FTNRoutines are FTNRTL's routines for intrinsic functions, in order: the
// one for each name in ftnRoutines, and FTN$GBLEQ and FTN$QEXTG, DBLEQ's and
// QEXTD's under /G_FLOATING. FTNRTL's tests find each in its vector.
var FTNRoutines = []string{
	"FTN$BITEST", "FTN$BJTEST", "FTN$CHAR", "FTN$DBLEQ", "FTN$GBLEQ", "FTN$ICHAR",
	"FTN$IIBCLR", "FTN$IIBITS", "FTN$IIBSET", "FTN$IISHFTC", "FTN$INDEX", "FTN$IZEXT",
	"FTN$JIBCLR", "FTN$JIBITS", "FTN$JIBSET", "FTN$JISHFTC", "FTN$JZEXT", "FTN$LEN",
	"FTN$LGE", "FTN$LGT", "FTN$LLE", "FTN$LLT", "FTN$NWORKERS", "FTN$QEXT",
	"FTN$QEXTD", "FTN$QEXTG", "FTN$QFLOAT", "FTN$SNGLQ",
}
