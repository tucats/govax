package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
)

// Fortran 90's intrinsic subroutines (the standard's 13.11; docs/planning/
// PHASE-108 - Fortran 90 intrinsic procedures.md, finding 2): MVBITS,
// DATE_AND_TIME, SYSTEM_CLOCK, RANDOM_NUMBER, and RANDOM_SEED, and Fortran
// 95's CPU_TIME. Like the
// intrinsic functions they take keyword arguments by their dummies' names
// without an explicit interface, CALL DATE_AND_TIME(VALUES=V), so they have a
// table of their own beside the functions', read by the same keyword code
// (intrinsickeys.go's keywordSlots): each subroutine's dummies, a ? after an
// optional one, and whether it is elemental (E), a call on arrays being a call
// for each element.
//
// MVBITS was VAX FORTRAN's already (D.5.3, system.go before Phase 108), and is
// intrinsic under both /F90 settings; the others under /F90, or where an
// INTRINSIC statement names them (Decision 2 of docs/planning/PHASE-99 -
// Fortran 90.md). As for the system subroutines, a unit's own routine of the
// name wins: the callers come here only for a name the unit hasn't
// classified, and that no unit of the file is.
const intrinsicSubroutineTable = `
MVBITS        E FROM,FROMPOS,LEN,TO,TOPOS
DATE_AND_TIME - DATE?,TIME?,ZONE?,VALUES?
SYSTEM_CLOCK  - COUNT?,COUNT_RATE?,COUNT_MAX?
RANDOM_NUMBER - HARVEST
RANDOM_SEED   - SIZE?,PUT?,GET?
CPU_TIME      - TIME
`

// intrinsicSub is one of the table's subroutines.
type intrinsicSub struct {
	name      string
	elemental bool
	dummies   []dummyArg

	// analyze is what a CALL of it becomes, given its arguments in the
	// dummies' order (nil for one left out).
	analyze func(*analyzer, *ast.CallStmt, []ast.Expr) Stmt
}

// intrinsicSubroutines are the table's subroutines, by name.
var intrinsicSubroutines = readIntrinsicSubroutines()

// readIntrinsicSubroutines reads intrinsicSubroutineTable.
func readIntrinsicSubroutines() map[string]*intrinsicSub {
	subs := map[string]*intrinsicSub{}

	for line := range strings.SplitSeq(intrinsicSubroutineTable, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}

		dummies, _ := readDummies(f[2])
		subs[f[0]] = &intrinsicSub{name: f[0], elemental: f[1] == "E", dummies: dummies}
	}

	subs["MVBITS"].analyze = (*analyzer).mvbits
	subs["DATE_AND_TIME"].analyze = (*analyzer).dateAndTime
	subs["SYSTEM_CLOCK"].analyze = (*analyzer).systemClock
	subs["RANDOM_NUMBER"].analyze = (*analyzer).randomNumber
	subs["RANDOM_SEED"].analyze = (*analyzer).randomSeed
	subs["CPU_TIME"].analyze = (*analyzer).cpuTime

	return subs
}

// intrinsicSubroutine is the intrinsic subroutine a CALL of name with the
// symbol s (nil if the unit has none) is, or nil: MVBITS always, the others
// under /F90 or by an INTRINSIC statement; never one the unit made something
// else, nor one a unit of the file is, nor one called with alternate returns.
func (a *analyzer) intrinsicSubroutine(name string, s *Symbol, altReturns bool) *intrinsicSub {
	sub := intrinsicSubroutines[name]

	switch {
	case sub == nil || a.units[name] != nil || altReturns:
		return nil
	case s != nil && s.Class == Intrinsic:
		return sub
	case s != nil && (s.Class != Unclassified || s.Dummy != 0):
		return nil
	case name == "MVBITS" || a.f90:
		return sub
	}

	return nil
}

// intrinsicCall analyzes CALL sub(list).
func (a *analyzer) intrinsicCall(s *ast.CallStmt, sub *intrinsicSub, list []ast.Expr) Stmt {
	a.extensions.intrinsicProcedure("the intrinsic subroutine", s.Name, s.NamePos)

	if sub.name == "MVBITS" {
		a.noteSystem(s.Name, s.NamePos)
	} else {
		a.noteOther(whatIntrinsic, s.Name, s.NamePos, RefUsed)

		// Of the intrinsic subroutines only MVBITS is PURE (the Fortran 95
		// standard's 13.1).
		a.pureSystem("the intrinsic subroutine", s.Name, s.NamePos, "be called")
	}

	slots, ok := a.keywordSlots(s.NamePos, s.Name, true, list, sub.dummies, false,
		func() string { return takesDummies(sub.dummies) }, false)
	if !ok {
		return nil
	}

	return sub.analyze(a, s, slots)
}

// takesDummies is how many arguments a procedure with the dummies takes, as
// INTRINSUBARGS says it: "5 arguments", "0 to 4 arguments".
func takesDummies(dummies []dummyArg) string {
	lo := 0

	for _, d := range dummies {
		if !d.optional {
			lo++
		}
	}

	if lo == len(dummies) {
		return count(lo, "argument")
	}

	return strconv.Itoa(lo) + " to " + count(len(dummies), "argument")
}

// mvbits analyzes CALL MVBITS(FROM, FROMPOS, LEN, TO, TOPOS) (13.13.74): LEN
// bits of FROM from FROMPOS up copied into TO from TOPOS up, TO's other bits
// unchanged. TO is a variable, an array element, an array, or a section, of
// INTEGER of any kind; FROM any INTEGER expression of TO's kind (a constant of
// another converted, as one is assigned); FROMPOS, LEN, and TOPOS any INTEGER
// expressions, converted to TO's kind, since FTNRTL's routine for the kind
// reads every argument at its length (FTN$BMVBITS, FTN$IMVBITS, FTN$JMVBITS,
// ftn_system.mar). It is elemental: with arrays among its arguments, which must
// conform and of which TO must be one, it is a call for each element
// (Call.Shape; ir's elementalCall). VAX FORTRAN's MVBITS (D.5.3) took only
// variables and elements for FROM and TO, all of one length: those are all
// still the same calls, and the rest were errors (the author's answer 8).
func (a *analyzer) mvbits(s *ast.CallStmt, slots []ast.Expr) Stmt {
	const where = "an integer expression where it says a position or a length"

	switch a.substringForm(slots[3]).(type) {
	case *ast.Name, *ast.Apply, *ast.Substring, *ast.FieldRef:
	default:
		a.report("SYSARG", slots[3].Position(), s.Name, "an INTEGER variable, array element, or array where it says TO")

		return nil
	}

	to := a.target(slots[3])
	if to == nil {
		return nil
	}

	a.noteAssigned(to)
	a.noteReadsToo(to)

	t := to.Type()
	if t.Kind != Integer {
		a.report("SYSARG", to.Position(), s.Name, "an INTEGER variable, array element, or array where it says TO")

		return nil
	}

	args := make([]Expr, 5)
	args[3] = to

	for _, i := range []int{0, 1, 2, 4} {
		v := a.arrayValue(slots[i])

		switch vt := v.Type(); {
		case vt.Kind == Invalid:
			return nil

		case vt.Kind != Integer && i == 0:
			a.report("SYSARG", v.Position(), s.Name, "an INTEGER expression where it says FROM")

			return nil

		case vt.Kind != Integer:
			a.report("SYSARG", v.Position(), s.Name, where)

			return nil

		case i == 0 && vt != t:
			// FROM is of TO's kind; a constant of another is converted as an
			// assignment converts it (INTRANGE past the kind's range).
			if _, ok := v.(*Const); !ok {
				a.report("SYSLEN", v.Position(), s.Name)

				return nil
			}

			if v = a.assigned(v, t); v.Type().Kind == Invalid {
				return nil
			}

		default:
			v = a.convert(v, t)
		}

		args[i] = v
	}

	shape, ok := a.conforms(s.NamePos, args...)
	if !ok {
		return nil
	}

	if shape != nil && rankOf(to) == 0 {
		a.report("SYSARG", to.Position(), s.Name, "an array where it says TO, since another argument is one")

		return nil
	}

	routine := map[int]string{1: "FTN$BMVBITS", 2: "FTN$IMVBITS", 4: "FTN$JMVBITS"}[t.Size]

	list := make([]Arg, len(args))
	for i, x := range args {
		list[i] = Arg{Expr: x}
	}

	c := systemCall(s, routine, list).(*Call)
	c.Shape = shape

	return c
}
