package sema

import "github.com/tucats/govax/internal/fortran/ast"

// Implicit typing (section 4.8). A name no declaration gives a type has
// the type its first letter has: by default INTEGER for I to N and REAL
// for the rest (section 2.2.2.2), as if every unit began
//
//	IMPLICIT INTEGER (I-N), REAL (A-H, O-Z)
//
// An IMPLICIT statement changes the type of the letters it names, for
// every name in the unit with no type of its own, whether it comes
// before the statement or after (the dummy arguments, named on the
// header before it, among them): so the unit's IMPLICIT statements are
// read before anything else in it (implicits). IMPLICIT NONE takes every
// letter's type away, and then a name with no declaration is an error
// (NOTYPE), reported once the whole unit has been read (untyped).

// implicitRules are a unit's letters' types.
type implicitRules struct {
	// types are the letters' types, A first.
	types [26]Type

	// lines are the lines of the IMPLICIT statements that gave the
	// letters their types, or 0 for a letter that still has its default.
	lines [26]int

	// none is set by IMPLICIT NONE; noneLine is its line.
	none     bool
	noneLine int
}

// defaultImplicit is the rules with no IMPLICIT statement.
func defaultImplicit(defaultInt Type) *implicitRules {
	r := &implicitRules{}

	for c := 'A'; c <= 'Z'; c++ {
		if c >= 'I' && c <= 'N' {
			r.types[c-'A'] = defaultInt
		} else {
			r.types[c-'A'] = RealType
		}
	}

	return r
}

// typeOf is the type a name has from its first letter. With IMPLICIT
// NONE it is Invalid and untyped is set: the name must be declared.
func (r *implicitRules) typeOf(name string) (t Type, untyped bool) {
	if name == "" || name[0] < 'A' || name[0] > 'Z' {
		return RealType, false
	}

	if r.none {
		return invalid, true
	}

	return r.types[name[0]-'A'], false
}

// implicits reads the unit's IMPLICIT statements, before anything else in
// it. A letter given a type twice is DUPIMPLICIT; IMPLICIT NONE with
// another IMPLICIT statement is IMPLNONE. A rule whose type has an error
// leaves its letters their default types.
func (a *analyzer) implicits(body []ast.Stmt) {
	r := a.implicit

	var other ast.Stmt // an IMPLICIT statement other than IMPLICIT NONE

	for _, s := range body {
		st, ok := s.(*ast.ImplicitStmt)
		if !ok {
			continue
		}

		if st.None {
			switch {
			case r.none:
				a.rejectImplicit(st, "IMPLICIT NONE", r.noneLine)
			case other != nil:
				a.rejectImplicit(st, "IMPLICIT", other.Position().Line)
			default:
				r.none, r.noneLine = true, st.Pos.Line
			}

			continue
		}

		if r.none {
			a.rejectImplicit(st, "IMPLICIT NONE", r.noneLine)

			continue
		}

		other = st

		for _, rule := range st.Rules {
			t, ok := a.typeOf(rule.Type, rule.Type.Len, rule.Type.Pos)

			for _, l := range rule.Letters {
				a.implicitLetters(t, ok, l, st.Pos.Line)
			}
		}
	}
}

// rejectImplicit reports an IMPLICIT statement in a unit that has
// IMPLICIT NONE and another IMPLICIT statement (IMPLNONE): the other is
// what, on line. The statement is left out, of the statement order too.
func (a *analyzer) rejectImplicit(s *ast.ImplicitStmt, what string, line int) {
	a.report("IMPLNONE", s.Pos, what, line)

	a.rejected[s] = true
}

// implicitLetters gives the letters l names the type t (if ok: its type
// had no error), for the IMPLICIT statement on line.
func (a *analyzer) implicitLetters(t Type, ok bool, l ast.LetterRange, line int) {
	r := a.implicit

	for c := l.First; c <= l.Last; c++ {
		if earlier := r.lines[c-'A']; earlier != 0 {
			a.report("DUPIMPLICIT", l.Pos, string(rune(c)), earlier)

			return
		}
	}

	for c := l.First; c <= l.Last; c++ {
		r.lines[c-'A'] = line

		if ok {
			r.types[c-'A'] = t
		}
	}
}

// untyped reports each name that needs a type and has none, under
// IMPLICIT NONE (NOTYPE): a variable, an array, a function, a statement
// function, a dummy argument. A subroutine and a program unit's name need
// none. The name is broken, so a later pass ignores it.
func (a *analyzer) untyped() {
	for _, s := range a.unit.Symbols {
		if !s.untyped || s.Typed || s.broken || s.Class == Subroutine || s.Class == UnitName {
			continue
		}

		a.report("NOTYPE", s.Pos, s.Name)

		s.broken = true
	}
}

// passedLengths reports each name given a passed length, CHARACTER*(*),
// that may not have one (PASSEDLEN): only a dummy argument, a function,
// and a PARAMETER constant (sections 4.4.2, 6.2.2.2) take their lengths
// from elsewhere.
func (a *analyzer) passedLengths() {
	for _, s := range a.unit.Symbols {
		if s.Type.Kind != Character || s.Type.Size != 0 || s.broken {
			continue
		}

		switch {
		case s.Dummy != 0, s.Result, s.Class == Function, s.Class == Constant:
			continue
		}

		pos := s.TypePos
		if !s.Typed {
			pos = s.Pos
		}

		a.report("PASSEDLEN", pos, s.Name)

		s.broken = true
	}
}
