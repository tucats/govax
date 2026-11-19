package sema

import "github.com/tucats/govax/internal/fortran/ast"

// Pointer results (the Fortran 90 standard's 12.5.2.2; docs/planning/PHASE-107
// - Fortran 90 pointers.md, finding 9). A function's value may be a pointer,
//
//	FUNCTION LARGEST(A)
//	REAL, TARGET :: A(:)
//	REAL, POINTER :: LARGEST
//	LARGEST => A(MAXLOC(A, 1))
//	END
//
// so that inside it the value is a pointer like any other, given an association
// by =>, ALLOCATE, or NULLIFY, and the caller is given that association rather
// than a value. A caller can't make sense of a reference unless it knows that,
// so one needs an explicit interface (12.3.1.1; EXPLICITPOINTER).
//
// Where a reference stands, it stands for the pointer's target, as a pointer's
// name does: LARGEST(X) = 0 isn't Fortran (a reference is no variable), but
// Y = 2 * LARGEST(X) reads the target, and so does an actual argument or an
// output list's item. Only pointer assignment sees the association itself,
//
//	P => LARGEST(X)
//
// which P takes as P => Q takes Q's (7.5.2: the target may be a reference to a
// function whose value is a pointer). The reference is a FuncRef with Pointer
// set, and for an array pointer Made too, since the function then describes
// the value's elements in a descriptor of the caller's, as it does an array
// value it makes; lowering's is ir/lowerpointer.go's.

// pointerValued reports whether u, a function's explicit interface, gives it a
// pointer value.
func pointerValued(u *Unit) bool {
	return u != nil && u.Result != nil && u.Result.Pointer && !u.Result.broken
}

// pointerFunctionTarget analyzes x, a reference to a function, as the target
// of a pointer to values of type pt and rank rank (named name, for
// messages): a reference to a function whose value is a pointer, of the
// pointer's type, kind, rank, and constant length. ok reports whether x is
// such a reference at all, the target nil after a message if it doesn't match
// or has an error of its own; not ok, it is any other function's reference,
// for the caller's POINTERFUNC.
func (a *analyzer) pointerFunctionTarget(name string, pt Type, rank int, x ast.Expr) (t Expr, ok bool) {
	apply, isApply := x.(*ast.Apply)
	if !isApply {
		return nil, false
	}

	s := a.scope.find(apply.Name)
	if s == nil || !pointerValued(a.interfaceOf(s)) {
		return nil, false
	}

	// apply, not arrayValue: a record's pointer is no aggregate here.
	v := a.apply(apply)

	f, isRef := v.(*FuncRef)

	switch {
	case v.Type().Kind == Invalid:
		// An error, reported.
		return nil, true
	case !isRef || !f.Pointer:
		return nil, false
	}

	if !a.pointerMatches(name, pt, rank, f, x.Position()) {
		return nil, true
	}

	return f, true
}

// pointerResultDifference is how the value of an interface body here differs
// from the definition's there in being a pointer, or "".
func pointerResultDifference(here, there *Symbol) string {
	if here.Pointer == there.Pointer {
		return ""
	}

	return "its value is " + attributeText(here.Pointer, "a pointer") + " here, " +
		attributeText(there.Pointer, "a pointer") + " there"
}

// entryPointerDifference is how the entry point's value s differs from the
// function's value f in being a pointer, or "": an entry point of a function
// whose value is a pointer is one too, of the same type, and the other way
// round (12.5.2.5: the function's characteristics are each name's, and the
// caller's association comes back in one form).
func entryPointerDifference(s, f *Symbol) string {
	switch {
	case s.Pointer != f.Pointer:
		return "it is " + attributeText(s.Pointer, "a pointer") + ", the function's " + attributeText(f.Pointer, "a pointer")
	case s.Pointer && !sameTypes(s.Type, f.Type):
		return "it points at " + pointedText(s.Type, len(s.Dims)) + ", the function's at " + pointedText(f.Type, len(f.Dims))
	}

	return ""
}
