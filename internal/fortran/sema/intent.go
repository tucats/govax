package sema

import (
	"github.com/tucats/govax/internal/fortran/diag"
)

// INTENT (the Fortran 90 standard's 5.1.2.3; docs/planning/PHASE-103 - Fortran
// 90 procedures and interfaces.md, finding 10). A dummy argument may say what
// the routine does with it:
//
//	      SUBROUTINE SCALE(A, N, F, AMAX)
//	      INTEGER, INTENT(IN) :: N
//	      REAL, INTENT(INOUT) :: A(N)
//	      REAL, INTENT(IN) :: F
//	      REAL, INTENT(OUT) :: AMAX
//
// IN: the routine uses its value and never changes it. OUT: the routine gives it
// a value before using it; it is undefined when the routine starts. INOUT: both.
// No INTENT is FORTRAN-77's rule, whatever the actual argument allows.
//
// INTENT changes no code. What the compiler checks is what it can see:
//
//   - an INTENT(IN) dummy given a value (an assignment's target, a DO or
//     implied-DO variable, an input item, ASSIGN's variable, a returned specifier
//     like IOSTAT=), or passed through an explicit interface to an INTENT(OUT) or
//     INTENT(INOUT) dummy, is INTENTIN, an error: the standard's "must not";
//   - an INTENT(OUT) dummy used on a line before every line that may give it a
//     value is INTENTOUTREF, a warning. "May give it a value" counts an actual
//     argument passed as itself, since the routine it goes to may set it. It is
//     the simplest sound form of "where the compiler can tell", in the order of
//     the statements, not their flow: a backward GO TO can make a later
//     definition come first when the program runs, which is why it is only a
//     warning, and why a use on the same line as a definition (X = X + 1, or
//     READ *, N, A(N)) says nothing.
//
// Through an implicit interface nothing can be said: the routine's dummies
// aren't known where it is called.

// definedSymbol is the variable or array the place e is part of, for a check of
// what gives it a value; nil for anything else, a pointer component's target
// among them (X%P = 4 changes no part of X: docs/planning/PHASE-107 - Fortran
// 90 pointers.md, subtask 5b), as is an array pointer component's (X%V(1) = 4,
// whose symbol is no dummy).
func definedSymbol(e Expr) *Symbol {
	if f, ok := e.(*FieldRef); ok && f.ThroughPointer() {
		return nil
	}

	switch e := e.(type) {
	case *Var:
		return e.Sym
	case *Elem:
		return e.Sym
	case *ArrayRef:
		return e.Sym
	case *Section:
		return e.Sym
	case *FieldRef:
		return e.Sym
	case *Substring:
		return definedSymbol(e.X)
	}

	return nil
}

// checkIntentIn reports INTENTIN if s is an INTENT(IN) dummy argument, about to
// be given a value at pos; and in a PURE procedure, NOTPURE if s is a global
// (pure.go).
func (a *analyzer) checkIntentIn(s *Symbol, pos diag.Position) {
	if s != nil && !a.readFrom && s.Dummy != 0 && s.Intent == "IN" && !s.broken {
		a.report("INTENTIN", pos, s.Name)
	}

	a.pureDefines(s, pos, "be given a value")
}

// noteNotRead notes that the mention at pos neither reads nor defines its name.
func (a *analyzer) noteNotRead(pos diag.Position) {
	if a.unit.notRead == nil {
		a.unit.notRead = map[Reference]bool{}
	}

	a.unit.notRead[Reference{pos.Line, pos.Column, pos.Serial, RefUsed}] = true
}

// noteInquired notes that the mention of e, a name or a part of one, is an
// inquiry function's argument, asking only about its type, kind, length, or
// shape: no use of its value, which may still be undefined (the standard's
// 7.1.6.1 allows an inquiry of such an object in a specification expression,
// and 14.7.5 says what makes a variable defined), and no definition either.
// So SIZE(V) of an INTENT(OUT) V says nothing, and a use of V after it still
// may (Phase 111, subtask 5a).
func (a *analyzer) noteInquired(e Expr) {
	var pos diag.Position

	switch e := e.(type) {
	case *Var, *Elem, *ArrayRef, *Section, *Substring:
		pos = e.Position()
	case *FieldRef:
		pos = e.NamePos
	default:
		return
	}

	a.noteInquiredAt(pos)
}

// noteInquiredAt is noteInquired for the mention at pos.
func (a *analyzer) noteInquiredAt(pos diag.Position) {
	if a.unit.inquired == nil {
		a.unit.inquired = map[Reference]bool{}
	}

	a.unit.inquired[Reference{pos.Line, pos.Column, pos.Serial, RefUsed}] = true
}

// noteReadsToo notes that the place e, which the statement gives a value
// (noteAssigned), it reads first: the mention noteAssigned made.
func (a *analyzer) noteReadsToo(e Expr) {
	pos := e.Position()

	switch e := e.(type) {
	case *FieldRef:
		pos = e.NamePos
	case *Substring:
		a.noteReadsToo(e.X)

		return
	}

	if a.unit.readsToo == nil {
		a.unit.readsToo = map[Reference]bool{}
	}

	a.unit.readsToo[Reference{pos.Line, pos.Column, pos.Serial, RefAssigned}] = true
}

// notePassed notes an actual argument passed as itself, which the routine it
// goes to may read or give a value.
func (a *analyzer) notePassed(arg Arg) {
	if arg.Copy || arg.Expr == nil {
		return
	}

	switch e := arg.Expr.(type) {
	case *Var, *Elem, *ArrayRef, *Section, *Substring:
		a.noteNotRead(e.Position())
	case *FieldRef:
		a.noteNotRead(e.NamePos)
	}
}

// reportIntentOut warns of each INTENT(OUT) dummy argument of the unit used on a
// line before the first line that may give it a value (INTENTOUTREF), once.
// After an error it says nothing, a statement with an error having perhaps lost
// its mentions. In a host, a mention in an internal procedure (whose lines come
// after the host's) may be a definition its calls make anywhere, so a dummy any of
// them mentions isn't looked at.
func (a *analyzer) reportIntentOut() {
	if a.messages.Failed() {
		return
	}

	limit := -1
	if len(a.unit.AST.Contains) > 0 {
		limit = a.unit.AST.Contains[0].Header.Position().Line
	}

	for _, s := range a.unit.Dummies {
		if s.Intent == "OUT" && !s.broken {
			a.intentOut(s, limit)
		}
	}
}

// intentOut is reportIntentOut for one dummy, s; limit is the first line of the
// unit's internal procedures, or -1.
func (a *analyzer) intentOut(s *Symbol, limit int) {
	at := func(r Reference) Reference { return Reference{r.Line, r.Column, r.Serial, RefUsed} }
	// A place with a stronger mention than a use is a declaration or a
	// definition, whose lookup noted a use too.
	stronger := map[Reference]bool{}
	first := -1

	for _, r := range s.refs {
		if limit >= 0 && r.Line >= limit {
			return
		}

		if r.Kind != RefUsed {
			stronger[at(r)] = true
		}

		if (r.Kind == RefAssigned || a.unit.notRead[at(r)]) && (first < 0 || r.Serial < first) {
			first = r.Serial
		}
	}

	var use *Reference

	for i, r := range s.refs {
		if r.Kind != RefUsed || a.unit.notRead[at(r)] || a.unit.inquired[at(r)] || stronger[at(r)] ||
			first >= 0 && r.Serial >= first {
			continue
		}

		if use == nil || r.Serial < use.Serial || r.Serial == use.Serial && r.Column < use.Column {
			use = &s.refs[i]
		}
	}

	if use != nil {
		a.report("INTENTOUTREF", a.mention(*use), s.Name)
	}
}

// pointerHome is the variable whose storage holds the pointer p itself (a
// reference to a pointer, IsPointer), for INTENTIN where a statement changes
// its association: a pointer component's record, if no pointer leads to it
// (X%P, X%V; not X%NEXT%P, which is in X%NEXT's target); nil for a named
// pointer, which can be no dummy argument yet.
func pointerHome(p Expr) *Symbol {
	if s := PointerSymbol(p); s != nil {
		if s.Component == nil {
			return nil
		}

		p = s.Component
	}

	f, ok := p.(*FieldRef)
	if !ok {
		return nil
	}

	for _, step := range f.Path[:len(f.Path)-1] {
		if step.Field.Pointer {
			return nil
		}
	}

	return f.Sym
}
