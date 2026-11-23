package sema

import "github.com/tucats/govax/internal/fortran/ast"

// The FORALL statement (Fortran 95's 7.5.4; the HP Fortran manual's 4.2.5;
// docs/planning/PHASE-112 - Fortran 95.md, subtask 8):
//
//	FORALL (I = 1:N, J = 1:N, A(I,J) /= 0.0) B(I,J) = 1.0 / A(I,J)
//
// assigns B(I,J) for every pair of I and J in the index set whose mask is true.
// It is an array assignment whose elements are named by indexes of its own
// rather than by sections, and it keeps an array assignment's rule (7.5.4.4):
// every index's bounds and stride are computed first, then the mask for the
// whole index set, then every value and every target's subscripts, and only
// then is anything stored. So
//
//	FORALL (I = 2:8) V(I) = V(I-1)
//
// shifts V by one place, where a DO loop running up would copy V(1) into
// every element.
//
// The indexes are the statement's own names (7.5.4.1: "statement entities"),
// as a statement function's dummy arguments are its own: a FORALL is a scope
// inside the unit's (scope.go) where each index is a scalar INTEGER symbol of
// its own, Forall.Indexes, in none of the unit's tables, taking only its type's
// being INTEGER from the entity the unit calls by its name, which it doesn't
// change. Lowering gives each a place (ir/lowerforall.go).
//
// Running the assignment for each tuple of indexes in turn, as a nest of DO
// loops, is right when no assignment can change what another reads: when
// nothing the value, the mask, or the target's subscripts read may share the
// target's storage, or what does is the very element the target names (the
// same text: B(I,J) = B(I,J) * 2), which no other tuple writes, since a FORALL
// may assign an element only once (7.5.4.5). Otherwise (Forall.Temporary) the
// mask and the values are computed first for every tuple into temporaries of
// the index set's shape, and a second nest stores them. Only procedures
// declared PURE may be referenced in a FORALL (7.5.4.3), so computing the
// value has no effect but its result.

// forallStatement analyzes the FORALL statement s, or nil after an error.
func (a *analyzer) forallStatement(s *ast.ForallStmt) Stmt {
	f := &Forall{stmtBase: stmtBase{s}}
	ok := true

	// The statement's scope, inside the unit's: its indexes are its own
	// names, and every other name in it is the unit's.
	unitScope := a.scope
	a.scope = newInnerScope(unitScope)

	defer func() { a.scope = unitScope }()

	for _, x := range s.Header.Indexes {
		index, good := a.forallIndex(x, f)
		ok = ok && good

		f.Indexes = append(f.Indexes, index)
		a.scope.define(index.Sym)
	}

	// The bounds and strides, which may not use an index of this FORALL
	// (7.5.4.1's constraint): read in the statement's scope, so such a use
	// finds the index and is reported, rather than reading the unit's
	// variable of its name.
	for i, x := range s.Header.Indexes {
		index := f.Indexes[i]
		index.Lower = a.forallBound(x.Lower, f)
		index.Upper = a.forallBound(x.Upper, f)

		if x.Stride != nil {
			index.Stride = a.forallBound(x.Stride, f)

			if n, isConst := constantInt(index.Stride); isConst && n == 0 {
				a.report("FORALLSTRIDE", x.Stride.Position())

				ok = false
			}
		}

		ok = ok && index.Lower != nil && index.Upper != nil && (x.Stride == nil || index.Stride != nil)
	}

	if s.Header.Mask != nil {
		f.Mask = a.forallMask(s.Header.Mask)
		ok = ok && f.Mask != nil
	}

	first := len(a.unit.calls)

	switch then := s.Then.(type) {
	case *ast.AssignStmt:
		st := a.assignment(then)
		if st == nil {
			return nil
		}

		assign, isAssign := st.(*Assign)
		if !isAssign {
			// A defined assignment, a subroutine's call (operators.go).
			a.notImplForm(then.Pos, "a defined assignment in a FORALL statement")

			return nil
		}

		f.Body = assign

	case *ast.PointerAssignStmt:
		a.notImplForm(then.Pos, "a pointer assignment in a FORALL statement")

		return nil

	default:
		return nil
	}

	ok = a.forallReferences(first) && ok
	ok = a.forallTargetIndex(f) && ok

	if !ok || !a.forallOrder(f) {
		return nil
	}

	return f
}

// forallIndex is the index x of the FORALL f: a symbol of its own, of type
// INTEGER. The name must be one the unit could give a scalar INTEGER variable
// (7.5.4.1: "a named scalar variable of type integer"): FORALLINDEX if the
// unit's entity of that name is anything else, NOTYPE if it has no type.
// FORALLTWICE for a name given twice. False after an error.
func (a *analyzer) forallIndex(x *ast.ForallIndex, f *Forall) (*ForallIndex, bool) {
	sym := NewSymbol(x.Name, IntegerType)
	sym.Pos = x.Pos
	index := &ForallIndex{Sym: sym}

	for _, other := range f.Indexes {
		if other.Sym.Name == x.Name {
			a.report("FORALLTWICE", x.Pos, x.Name)

			return index, false
		}
	}

	var (
		t       Type
		untyped bool
		what    string
	)

	if s := a.scope.find(x.Name); s != nil {
		s.typesEntity = true

		if s.broken {
			return index, false
		}

		t, untyped = s.Type, s.untyped && !s.Typed

		switch {
		case s.Class == Constant:
			what = "a named constant"
		case len(s.Dims) > 0:
			what = "an array"
		case s.Class != Variable && s.Class != Unclassified:
			what = "not a variable"
		}
	} else {
		t, untyped = a.scope.rules().typeOf(x.Name)

		if a.warnings.Declarations && !untyped {
			a.report("UNDECLARED", x.Pos, x.Name, t.String())
		}
	}

	switch {
	case untyped:
		a.report("NOTYPE", x.Pos, x.Name)

		return index, false

	case what == "" && t.Kind != Integer:
		what = article(t.String())
	}

	if what != "" {
		a.report("FORALLINDEX", x.Pos, x.Name, what)

		return index, false
	}

	return index, true
}

// forallBound is a FORALL index's bound or stride, a scalar INTEGER
// (FORALLINT) converted to the default INTEGER, that uses no index of the
// FORALL f (FORALLBOUND); nil after an error.
func (a *analyzer) forallBound(e ast.Expr, f *Forall) Expr {
	v := a.numericConst(a.value(e))

	switch t := v.Type(); {
	case t.Kind == Invalid:
		return nil

	case t.Kind != Integer:
		a.report("FORALLINT", e.Position(), article(t.String()))

		return nil

	case t != IntegerType:
		v = a.convert(v, IntegerType)
	}

	for _, s := range readSymbols(v) {
		for _, index := range f.Indexes {
			if s == index.Sym {
				a.report("FORALLBOUND", e.Position(), s.Name)

				return nil
			}
		}
	}

	return v
}

// forallMask is a FORALL's mask, a scalar LOGICAL (an INTEGER's bit 0, as an
// IF's condition is, VAX FORTRAN's rule), or nil after an error.
func (a *analyzer) forallMask(e ast.Expr) Expr {
	m := a.numericConst(a.value(e))

	switch t := m.Type(); t.Kind {
	case Invalid:
		return nil
	case Logical:
		return m
	case Integer:
		return a.convert(m, Type{Kind: Logical, Size: t.Size})
	default:
		a.report("FORALLMASK", e.Position(), article(t.String()))
	}

	return nil
}

// forallReferences reports each procedure a FORALL references that isn't
// declared PURE (7.5.4.3; FORALLPURE): those the statement's analysis added to
// the unit's calls after the first. False if there is one.
func (a *analyzer) forallReferences(first int) bool {
	ok := true

	for _, r := range a.unit.calls[first:] {
		if r.sym.broken {
			continue
		}

		if u := a.interfaceOf(r.sym); u == nil || !u.Pure {
			a.report("FORALLPURE", r.pos, r.sym.Name)

			ok = false
		}
	}

	return ok
}

// forallTargetIndex reports an assignment in the FORALL f to one of its
// indexes, which only the FORALL may give values (FORALLSET). False if it is.
func (a *analyzer) forallTargetIndex(f *Forall) bool {
	target := definedSymbol(f.Body.Target)

	for _, index := range f.Indexes {
		if target == index.Sym {
			a.report("FORALLSET", f.Body.Target.Position(), target.Name)

			return false
		}
	}

	return true
}

// forallOrder decides whether the FORALL f's assignment may run tuple by tuple,
// or must compute its values first (Forall.Temporary; see the top of this
// file). False after NOTIMPL, for the forms the temporaries don't take yet: a
// target that is an array at each tuple, a CHARACTER target whose length is
// known only when the program runs, and a target whose subscripts read its own
// storage, which would need its subscripts kept too.
func (a *analyzer) forallOrder(f *Forall) bool {
	target := f.Body.Target
	ts := forallTargetSymbol(target)
	same := DumpExpr(target)

	if forallSubscriptsRead(target, ts) {
		a.notImplForm(target.Position(), "a FORALL target whose subscripts read its own storage")

		return false
	}

	if !readsElsewhere(f.Body.Value, ts, same) && !readsElsewhere(f.Mask, ts, same) {
		return true
	}

	switch t := target.Type(); {
	case rankOf(target) > 0:
		a.notImplForm(target.Position(), "a FORALL whose array target its value overlaps")

		return false

	case t.Kind == Character && t.Size == 0:
		a.notImplForm(target.Position(), "a FORALL whose target of run-time length its value overlaps")

		return false
	}

	f.Temporary = true

	return true
}

// forallTargetSymbol is the symbol whose storage the FORALL target t writes.
func forallTargetSymbol(t Expr) *Symbol {
	if s := baseSymbol(t); s != nil {
		return s
	}

	return definedSymbol(t)
}

// forallSubscriptsRead reports whether the target's subscripts, or a
// substring's bounds, may read storage ts may share.
func forallSubscriptsRead(target Expr, ts *Symbol) bool {
	switch t := target.(type) {
	case *Elem:
		for _, s := range t.Subs {
			if readsAny(s, ts) {
				return true
			}
		}

	case *Section:
		for _, s := range t.Subs {
			for _, x := range []Expr{s.Index, s.Vector, s.Lower, s.Upper, s.Stride} {
				if x != nil && arrayReads(x, ts) {
					return true
				}
			}
		}

	case *Substring:
		return forallSubscriptsRead(t.X, ts) || readsAny(t.Lower, ts) || t.Upper != nil && readsAny(t.Upper, ts)

	case *FieldRef:
		for _, x := range t.Subs {
			if readsAny(x, ts) {
				return true
			}
		}

		for _, step := range t.Path {
			for _, x := range step.Subs {
				if readsAny(x, ts) {
					return true
				}
			}
		}
	}

	return false
}

// readsElsewhere reports whether e may read storage ts may share anywhere but
// at the reference whose text is same, the target's: arrayReads, passing over
// each part of e that is that very reference.
func readsElsewhere(e Expr, ts *Symbol, same string) bool {
	if e == nil {
		return false
	}

	if DumpExpr(e) == same {
		return false
	}

	switch e := e.(type) {
	case *Unary:
		return readsElsewhere(e.X, ts, same)
	case *Convert:
		return readsElsewhere(e.X, ts, same)
	case *Binary:
		return readsElsewhere(e.X, ts, same) || readsElsewhere(e.Y, ts, same)
	case *Compare:
		return readsElsewhere(e.X, ts, same) || readsElsewhere(e.Y, ts, same)
	case *Substring:
		return readsElsewhere(e.X, ts, same) || readsElsewhere(e.Lower, ts, same) || readsElsewhere(e.Upper, ts, same)
	case *Concat:
		for _, p := range e.Parts {
			if readsElsewhere(p, ts, same) {
				return true
			}
		}

		return false
	case *IntrinsicRef:
		for _, x := range e.Args {
			if readsElsewhere(x, ts, same) {
				return true
			}
		}

		return false
	case *FuncRef:
		// What a function can reach besides its arguments (readsAny's rule).
		if e.Pointer || e.Elemental || ts.Common != nil || ts.Dummy != 0 || ts.group != nil {
			return arrayReads(e, ts)
		}

		for _, x := range e.Args {
			if x.Expr != nil && readsElsewhere(x.Expr, ts, same) {
				return true
			}
		}

		return false
	}

	return arrayReads(e, ts)
}
