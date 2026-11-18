package sema

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Keyword arguments, OPTIONAL dummies, and PRESENT (the Fortran 90 standard's
// 12.4.1, 5.1.2.6, and 13.13.80; docs/planning/PHASE-103 - Fortran 90
// procedures and interfaces.md, findings 8 and 9). Through an explicit
// interface (an internal procedure's or an interface body's) a reference may
// name its arguments by their dummies' names, and leave out an OPTIONAL dummy:
//
//	      INTERFACE
//	        SUBROUTINE SOLVE(A, N, TOL, ITER)
//	          REAL A(N)
//	          INTEGER N
//	          REAL, OPTIONAL :: TOL
//	          INTEGER, OPTIONAL :: ITER
//	        END SUBROUTINE
//	      END INTERFACE
//	      CALL SOLVE(X, 10, ITER=50)
//
// The arguments without a keyword come first and go to the dummies in order (X to
// A, 10 to N), and each with one goes to the dummy it names (50 to ITER); no dummy
// may be given twice, and only an OPTIONAL one may be left out (TOL). That is the
// rule intrinsicSlots follows for an intrinsic function, here against the
// interface's dummies: the names the interface the caller sees gives them, which
// for an interface body may differ from the definition's.
//
// actuals puts a reference's arguments in its dummies' order before anything else
// looks at them, so every later step (the analysis of each argument, the check
// against the interface, the IR's argument list) sees a plain list of positional
// arguments, a nil for each dummy left out. A nil is the null argument VAX FORTRAN
// always had (CALL S(A,,B)): its longword in the argument list is 0. So an
// argument left out is passed as VMS passes one (Decision 13 of docs/planning/
// PHASE-99 - Fortran 90.md), and the list is always as long as the dummies, the
// author's answer 4.
//
// PRESENT(TOL) asks whether a call gave TOL an argument. TOL must be an OPTIONAL
// dummy argument of the unit, or of its host (an internal procedure may ask about
// its host's). How the routine tells, even when its caller passed a shorter list
// than its dummies, is the IR's (ir/loweroptional.go).

// actuals is the argument list of a reference to the procedure s (list, the
// alternate return specifiers taken out, which were in the places places)
// in the order of its explicit interface's dummies, with a nil for each one left
// out. A reference through no explicit interface keeps its list; a keyword in it
// is KEYWORDNOINTERFACE. ok is false, after a message, if the list can't be put
// in order; a list whose only fault is how many arguments it has is returned as it
// is, for checkInterface's ARGCOUNT.
func (a *analyzer) actuals(s *Symbol, list []ast.Expr, places []int, pos diag.Position) ([]ast.Expr, bool) {
	keyword := slices.IndexFunc(list, func(e ast.Expr) bool {
		_, ok := e.(*ast.KeywordArg)

		return ok
	})

	target := a.interfaceOf(s)
	if target == nil || target.incomplete || s.broken {
		if keyword >= 0 {
			if target == nil {
				a.report("KEYWORDNOINTERFACE", list[keyword].Position(), s.Name)
			}

			return nil, false
		}

		return list, true
	}

	if keyword < 0 && firstOptional(target.Params) == nil {
		return list, true
	}

	// Each argument's place in the whole list, alternate returns counted.
	full := make([]int, len(list))

	for i, at := 0, 0; i < len(list); i, at = i+1, at+1 {
		for slices.Contains(places, at) {
			at++
		}

		full[i] = at
	}

	if keyword >= 0 && len(places) > 0 && places[len(places)-1] > full[keyword] {
		a.report("KEYWORDORDER", pos, s.Name)

		return nil, false
	}

	slots := make([]ast.Expr, len(target.Params))
	given := make([]bool, len(target.Params))

	for i, e := range list {
		if k, ok := e.(*ast.KeywordArg); ok {
			j := slices.IndexFunc(target.Params, func(p *Symbol) bool { return p.Name == k.Name })

			switch {
			case j < 0:
				a.report("KEYWORDNAME", k.Pos, s.Name, k.Name, dummyList(target.Params))

				return nil, false
			case given[j]:
				a.report("KEYWORDTWICE", k.Pos, k.Name, s.Name)

				return nil, false
			}

			slots[j], given[j] = k.X, true

			continue
		}

		if keyword >= 0 && i > keyword {
			a.report("KEYWORDORDER", e.Position(), s.Name)

			return nil, false
		}

		// The dummy at that place, the interface's alternate returns counted.
		j := full[i]
		for _, alt := range target.AltPlaces {
			if alt < full[i] {
				j--
			}
		}

		if j >= len(target.Params) || slices.Contains(target.AltPlaces, full[i]) {
			if keyword < 0 {
				return list, true
			}

			a.report("ARGCOUNT", pos, s.Name, count(len(target.Params)+len(target.AltPlaces), "dummy argument"),
				target.AST.Position().Line, len(list)+len(places))

			return nil, false
		}

		slots[j], given[j] = e, true
	}

	for j, p := range target.Params {
		if given[j] || p.Optional {
			continue
		}

		if keyword < 0 {
			return list, true
		}

		a.report("MISSINGARG", pos, s.Name, p.Name)

		return nil, false
	}

	return slots, true
}

// interfaceOf is the explicit interface a reference to s in this unit goes
// through: an internal procedure's or an interface body's, or for a recursive
// procedure's reference to itself its own (12.3.1: a procedure's interface is
// explicit within it). nil if there is none.
func (a *analyzer) interfaceOf(s *Symbol) *Unit {
	if u := explicitInterface(s); u != nil {
		return u
	}

	if u := a.unit; !a.inBody && s.Name == u.Name && (s.Class == UnitName || s.Result) && u.Kind != ast.MainProgram {
		return u
	}

	return nil
}

// dummyList is a routine's dummies' names, as KEYWORDNAME lists them.
func dummyList(params []*Symbol) string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Name
	}

	return nameList(names)
}

// present analyzes PRESENT(A) (13.13.80): A, or A= with it, the name of an
// OPTIONAL dummy argument of this unit or its host (NOTOPTIONAL otherwise).
func (a *analyzer) present(e *ast.Apply) Expr {
	switch {
	case len(e.Args) != 1:
		a.report("INTRINARGS", e.Pos, "PRESENT", "1 argument", len(e.Args))

		return a.bad(e.Pos)

	case e.Args[0] == nil:
		a.report("NULLINTRINARG", e.Pos, "PRESENT")

		return a.bad(e.Pos)
	}

	arg := e.Args[0]
	if k, ok := arg.(*ast.KeywordArg); ok {
		if k.Name != "A" {
			a.report("KEYWORDNAME", k.Pos, "the intrinsic function PRESENT", k.Name, "A")

			return a.bad(e.Pos)
		}

		arg = k.X
	}

	n, ok := arg.(*ast.Name)
	s := (*Symbol)(nil)

	if ok {
		s = a.scope.find(n.Name)
	}

	if s == nil || s.Dummy == 0 || !s.Optional {
		if s == nil || !s.broken {
			a.report("NOTOPTIONAL", arg.Position())
		}

		return a.bad(e.Pos)
	}

	// A mention, but not a use of the dummy's value (reportIntentOut).
	a.note(&s.refs, n.Pos, RefUsed)
	a.noteNotRead(n.Pos)

	return &Present{exprBase{T: LogicalType, Pos: e.Pos}, s}
}
