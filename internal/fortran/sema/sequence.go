package sema

// Sections and array expressions given to dummies that aren't assumed-shape
// (the Fortran 90 standard's 12.4.1.1 and 12.4.1.4; docs/planning/PHASE-104 -
// Fortran 90 dynamic arrays.md, finding 11).
//
//	      SUBROUTINE TOTAL(V, N, T)
//	      REAL V(N)
//	      ...
//	      CALL TOTAL(A(1, :), 10, T)
//
// TOTAL's V is an explicit-shape dummy, as every FORTRAN-77 array dummy is: it
// takes the address of its actual's first element and finds the rest one after
// another from there (sequence association). A whole array or an element is
// passed that way, as before. A section's elements may not be one after another:
// A(1, :) of A(10, 10) is every tenth element. So the caller passes the address
// of elements that are, in array element order:
//
//   - a section known to be contiguous when compiling (Contiguous: A(:, J),
//     A(2:5, J), A(:, :, K)) is passed as its first element's address, with no
//     copy;
//   - any other section, or a whole assumed-shape array (whose elements may be a
//     section of its caller's), is copied into a temporary of the statement's,
//     which is passed, and copied back after the call returns; so is a component
//     with a rank, PATH%X, whose elements are a type's size apart
//     (docs/planning/PHASE-105 - Fortran 90 derived types.md, finding 7);
//   - an assumed-shape array, or a section of one, is first tested when the
//     program runs: if its strides say its elements are one after another after
//     all, it is passed with no copy (the author's answer 4);
//   - an array expression, a section with a vector subscript, or an array in
//     parentheses is computed into a temporary and never copied back: the dummy
//     isn't definable then (NOTDEFINABLE through an explicit interface that says
//     INTENT(OUT) or INTENT(INOUT)).
//
// Nor is anything copied back to an INTENT(IN) dummy's actual. A CHARACTER one is
// passed by a class A descriptor of whichever is passed. A function reference's
// copy back is done when its call returns, before the rest of its statement.
//
// Arg.Sequence marks such an argument, and Arg.CopyBack the ones copied back
// when a copy is made; the IR's lowersequence.go does the rest.

// sequenced is arg, analyzed by argValue, as the actual argument of p, a dummy
// that isn't assumed-shape (nil: the dummy isn't known, an implicit interface):
// an array section or array expression, or a whole assumed-shape array, is
// marked Sequence, for the IR to pass the address of its elements one after
// another. Anything else is arg as it was.
func (a *analyzer) sequenced(arg Arg, p *Symbol) Arg {
	whole := false
	if r, ok := arg.Expr.(*ArrayRef); ok && r.Sym.AssumedShape() && !r.Sym.broken {
		whole = true
	}

	if !arg.expression && !whole {
		return arg
	}

	arg.expression = false
	v := arg.Expr

	switch {
	case arg.Hollerith || arg.Typeless || arg.Mode != ByDefault:
		return a.arrayChecked(Arg{Expr: v, expression: true})
	}

	// A temporary's CHARACTER elements need a length, which a value whose
	// length is known only when the program runs gives only if it is made of
	// names (as a constructor's first value must, Constructor.LengthOf).
	if t := v.Type(); t.Kind == Character && t.Size == 0 && !Described(arg) && !lengthFromNames(v) {
		a.notImplForm(v.Position(), "a CHARACTER array expression of a length known only when the program runs, not made of names, as an actual argument")

		return Arg{Expr: a.bad(v.Position())}
	}

	arg.Sequence = true
	arg.CopyBack = Described(arg) && (p == nil || p.Intent != "IN")

	return arg
}

// Contiguous reports whether the elements of the section e are known, when
// compiling, to be one after another in array element order, so that the
// address of its first element can stand for it (sequence association): its
// array's are (it isn't an assumed-shape dummy), and each dimension is the whole
// dimension by a stride of 1, up to one that is a stride of 1 over part of it, or
// a single subscript, after which every one is a single subscript. A(:, J),
// A(2:5, J), and A(:, :) are; A(2:5, :), A(J, :), and A(1:9:2) aren't (unless a
// dimension has one element, which isn't looked at).
func Contiguous(e *Section) bool {
	if e.Sym.AssumedShape() {
		return false
	}

	partial := false

	for i, sub := range e.Subs {
		switch {
		case partial:
			if sub.Ranges() {
				return false
			}

		case sub.Vector != nil:
			return false

		case sub.Index != nil:
			partial = true

		case !isInt(sub.Stride, 1):
			return false

		case !wholeDimension(e.Sym.Dims[i], sub):
			partial = true
		}
	}

	return true
}

// wholeDimension reports whether the triplet sub is known to cover the whole
// dimension d: each bound left out (nil, for one known only when the program
// runs) or the dimension's own constant.
func wholeDimension(d Dim, sub SectionSub) bool {
	lower := sub.Lower == nil || (d.LowerKnown() && isInt(sub.Lower, d.Lower))
	upper := sub.Upper == nil || (d.UpperKnown() && isInt(sub.Upper, d.Upper))

	return lower && upper
}

// isInt reports whether e is the integer constant v.
func isInt(e Expr, v int) bool {
	c, ok := constantInt(e)

	return ok && c == v
}
