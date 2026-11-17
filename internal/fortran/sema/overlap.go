package sema

import "github.com/tucats/govax/internal/fortran/diag"

// Overlap (docs/planning/PHASE-102 - Fortran 90 array expressions.md,
// finding 6). An array assignment must act as if its whole right side were
// computed before any element of its target is defined (Fortran 90's
// 7.5.1.5); the scalarizer computes it an element at a time, as it assigns, so
// that is right only where no element the loops read was written by an
// earlier pass. This is the test. So far it knows two answers: the sides are
// apart (different storage), or they are the same element (the target's own
// section read with the same subscripts, A = A + 1, A(1:N) = 2 * A(1:N)).
// Anything else may overlap, and is NOTIMPL until the ordered answer and
// temporaries come (subtask 5).

// checkOverlap reports an array assignment whose value may read an element of
// the target after the loops write it: false after NOTIMPL. Scalars computed
// once before the loops (Inline false) are read before any element is written,
// and can't.
func (a *analyzer) checkOverlap(pos diag.Position, target, value Expr) bool {
	ts := baseSymbol(target)

	ok := true

	Scalarize(value, func(leaf Expr) Expr {
		if s := baseSymbol(leaf); ok && mayShare(ts, s) && !sameElements(target, leaf) {
			ok = false
		}

		return leaf
	}, func(x Expr) Expr {
		if ok && Inline(x) {
			for _, s := range readSymbols(x) {
				if mayShare(ts, s) {
					ok = false
				}
			}
		}

		return x
	})

	if !ok {
		a.notImplForm(pos, "an array assignment whose value may read its target's changed elements")
	}

	return ok
}

// baseSymbol is the array an array-valued leaf or target is of.
func baseSymbol(e Expr) *Symbol {
	switch e := e.(type) {
	case *ArrayRef:
		return e.Sym
	case *Section:
		return e.Sym
	case *Substring:
		return baseSymbol(e.X)
	}

	return nil
}

// mayShare reports whether x and y may be the same storage, wholly or in
// part: the same name; two names in one common block whose places meet; or
// two names an EQUIVALENCE ties together. Two dummy arguments are taken to be
// apart, by the rule the optimizer keeps too (the standard's 12.5.2.9: a
// program may not change what one name refers to through another).
func mayShare(x, y *Symbol) bool {
	switch {
	case x == nil || y == nil:
		return false
	case x == y:
		return true
	case x.Dummy != 0 || y.Dummy != 0:
		return false
	case x.Common != nil || y.Common != nil:
		return x.Common == y.Common && x.Offset < y.Offset+y.Size() && y.Offset < x.Offset+x.Size()
	}

	return x.group != nil && x.group == y.group
}

// sameElements reports whether leaf reads, at each index of the loops, the
// element target writes there: the same array, whole on both sides, or a
// section with the same subscripts written the same way on both. Their
// values are computed once, before the loops, so the same text is the same
// value.
func sameElements(target, leaf Expr) bool {
	if ts, ok := target.(*Substring); ok {
		ls, ok := leaf.(*Substring)

		return ok && DumpExpr(ts.Lower) == DumpExpr(ls.Lower) && DumpExpr(ts.Upper) == DumpExpr(ls.Upper) &&
			sameElements(ts.X, ls.X)
	}

	switch t := target.(type) {
	case *ArrayRef:
		l, ok := leaf.(*ArrayRef)

		return ok && l.Sym == t.Sym

	case *Section:
		l, ok := leaf.(*Section)

		return ok && l.Sym == t.Sym && DumpExpr(l) == DumpExpr(t)
	}

	return false
}

// Inline reports whether the scalarizer computes the scalar e in its loops,
// at each element, rather than once before them: a constant, a variable, an
// element whose subscripts are constants, and any CHARACTER value, whose text
// it reads where it is (the CHARACTER values it can't, a function's or a
// concatenation's of a length known only when the program runs, are NOTIMPL:
// checkArrayParts).
func Inline(e Expr) bool {
	switch e := e.(type) {
	case *Const, *Var:
		return true

	case *Elem:
		for _, s := range e.Subs {
			if _, ok := s.(*Const); !ok {
				return false
			}
		}

		return true
	}

	return e.Type().Kind == Character
}

// readSymbols are the variables and arrays a scalar inline expression reads.
func readSymbols(e Expr) []*Symbol {
	var list []*Symbol

	var walk func(Expr)

	walk = func(e Expr) {
		switch e := e.(type) {
		case *Var:
			list = append(list, e.Sym)
		case *Elem:
			list = append(list, e.Sym)

			for _, s := range e.Subs {
				walk(s)
			}
		case *FieldRef:
			list = append(list, e.Sym)
		case *Substring:
			walk(e.X)
			walk(e.Lower)
			walk(e.Upper)
		case *Concat:
			for _, p := range e.Parts {
				walk(p)
			}
		case *Convert:
			walk(e.X)
		}
	}

	walk(e)

	return list
}

// checkArrayParts reports the parts of an array assignment's value the
// scalarizer can't compute yet: a CHARACTER value made by a function, which
// would be called at each element, and a concatenation in an array expression
// whose length is known only when the program runs, whose dynamic string would
// be made at each element. False after NOTIMPL.
func (a *analyzer) checkArrayParts(value Expr) bool {
	ok := true

	var visit func(Expr)

	visit = func(e Expr) {
		if !ok || e == nil {
			return
		}

		switch e := e.(type) {
		case *FuncRef, *StmtFuncRef:
			if e.Type().Kind == Character {
				a.notImplForm(e.Position(), "a CHARACTER function's value in an array expression")
				ok = false
			}

		case *Concat:
			if e.T.Size == 0 {
				a.notImplForm(e.Position(), "a concatenation of a length not constant in an array expression")
				ok = false

				return
			}

			for _, p := range e.Parts {
				visit(p)
			}

		case *Substring:
			visit(e.X)

		case *Unary:
			visit(e.X)

		case *Binary:
			visit(e.X)
			visit(e.Y)

		case *Compare:
			visit(e.X)
			visit(e.Y)

		case *Convert:
			visit(e.X)
		}
	}

	visit(value)

	return ok
}
