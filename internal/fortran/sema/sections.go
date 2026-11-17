package sema

import "github.com/tucats/govax/internal/fortran/ast"

// Sections and substrings. Since Fortran 90 a parenthesized list after a
// name may hold subscript triplets, [l]:[u][:s], which make an array section
// of an array (A(1:N:2), A(:, J)). A substring of a CHARACTER variable,
// C(1:3), is written the same way, and the parser, which knows nothing of
// declarations, reads both as an ast.Apply with an ast.Triplet among its
// arguments (docs/planning/PHASE-102 - Fortran 90 array expressions.md,
// finding 2). Here, where the declarations are known, the two are told apart.

// substringForm is e as the analyzer's substring code expects it: an Apply
// whose list is one triplet without a stride, after a name that isn't an
// array, becomes the ast.Substring the parser made before Fortran 90 (C(1:3)
// is a substring of the variable C). Anything else is e itself: an array
// section stays an Apply, for element to report; a list with a triplet after
// a name that isn't an array but isn't one substring range either gets its
// message from the reference's own analysis.
//
// Every place that tells an element from a substring by the syntax tree's
// node calls it first, so old programs' substrings take the path they always
// took (TestNoF90Unchanged).
func (a *analyzer) substringForm(e ast.Expr) ast.Expr {
	ap, ok := e.(*ast.Apply)
	if !ok {
		return e
	}

	if s := a.scope.find(ap.Name); s != nil && s.Class == Array {
		return e
	}

	return substringReading(e)
}

// substringReading is e as Fortran 77 read it: one triplet without a stride
// after a name is a substring, whatever the name is. An EQUIVALENCE list reads
// it so, since an array section is never an equivalence object (the
// standard's R546): A(1:2) of an array is a substring of a whole array there,
// NEEDSUBSCRIPT as before.
func substringReading(e ast.Expr) ast.Expr {
	ap, ok := e.(*ast.Apply)
	if !ok || len(ap.Args) != 1 {
		return e
	}

	t, ok := ap.Args[0].(*ast.Triplet)
	if !ok || t.Stride != nil {
		return e
	}

	return &ast.Substring{
		ExprBase: ap.ExprBase,
		X:        &ast.Name{ExprBase: ap.ExprBase, Name: ap.Name},
		Lower:    t.Lower,
		Upper:    t.Upper,
	}
}

// hasTriplet reports whether a subscript list holds a subscript triplet: an
// array section's list, not an element's.
func hasTriplet(list []ast.Expr) bool {
	for _, e := range list {
		if _, ok := e.(*ast.Triplet); ok {
			return true
		}
	}

	return false
}
