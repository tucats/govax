package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// What lowering needs to make an array statement's elements (ir/lowerarray.go;
// docs/planning/PHASE-102 - Fortran 90 array expressions.md, finding 5). The
// scalarizer runs an array assignment as loops over normalized indexes, and
// asks each array-valued part of it for "your element at these indexes" as a
// scalar expression, which the rest of lowering then lowers as it lowers any
// other: an element of a section is an Elem, an elemental operation the same
// operation on its operands' elements. Those expressions are built here, since
// an expression's fields are this package's.

// NewSymbol is a scalar variable of type t that no unit declared: one of
// lowering's own, a loop's index or a value computed once before the loops,
// which lowering gives a place.
func NewSymbol(name string, t Type) *Symbol {
	return &Symbol{Name: name, Class: Variable, Type: t, Offset: -1}
}

// NewVar is a reference to the scalar variable s.
func NewVar(s *Symbol, pos diag.Position) *Var {
	return &Var{exprBase{T: s.Type, Pos: pos}, s}
}

// IntConstAt is the INTEGER constant v.
func IntConstAt(v int, pos diag.Position) *Const {
	return &Const{exprBase: exprBase{T: IntegerType, Pos: pos}, Int: int32(v)}
}

// IntSum is x + y, INTEGER, a constant if both are, x alone if y is 0.
func IntSum(x, y Expr) Expr {
	cx, okx := constantInt(x)
	cy, oky := constantInt(y)

	switch {
	case okx && oky:
		return IntConstAt(cx+cy, x.Position())
	case oky && cy == 0:
		return x
	case okx && cx == 0:
		return y
	}

	return &Binary{exprBase{T: IntegerType, Pos: x.Position()}, ast.Add, x, y}
}

// IntProduct is x * y, INTEGER, a constant if both are, x alone if y is 1.
func IntProduct(x, y Expr) Expr {
	cx, okx := constantInt(x)
	cy, oky := constantInt(y)

	switch {
	case okx && oky:
		return IntConstAt(cx*cy, x.Position())
	case oky && cy == 1:
		return x
	case okx && cx == 1:
		return y
	}

	return &Binary{exprBase{T: IntegerType, Pos: x.Position()}, ast.Mul, x, y}
}

// IntDifference is x - y, INTEGER, a constant if both are.
func IntDifference(x, y Expr) Expr {
	cx, okx := constantInt(x)
	cy, oky := constantInt(y)

	switch {
	case okx && oky:
		return IntConstAt(cx-cy, x.Position())
	case oky && cy == 0:
		return x
	}

	return &Binary{exprBase{T: IntegerType, Pos: x.Position()}, ast.Sub, x, y}
}

// NewElem is the element s(subs), whose subscript i is known to be within
// s's bounds when inBounds[i] is set (Elem.InBounds).
func NewElem(s *Symbol, subs []Expr, inBounds []bool, pos diag.Position) *Elem {
	return &Elem{exprBase: exprBase{T: s.Type, Pos: pos}, Sym: s, Subs: subs, InBounds: inBounds}
}

// Checked reports whether /CHECK=BOUNDS checks the element's subscript i.
func (e *Elem) Checked(i int) bool { return i >= len(e.InBounds) || !e.InBounds[i] }

// Scalarize is the array-valued expression e made a scalar one: each array it
// reads (an *ArrayRef, a *Section, an *ArrayConst, or a *Constructor, a leaf)
// replaced by element(leaf), each
// scalar part of it by scalar(part), and each elemental operation between
// them copied without a shape. scalar is how a value computed once before the
// loops stands for itself in them.
func Scalarize(e Expr, element func(leaf Expr) Expr, scalar func(Expr) Expr) Expr {
	if rankOf(e) == 0 {
		return scalar(e)
	}

	walk := func(x Expr) Expr { return Scalarize(x, element, scalar) }

	switch e := e.(type) {
	case *ArrayRef, *Section, *ArrayConst, *Constructor:
		return element(e)

	case *Unary:
		c := *e
		c.S, c.X = nil, walk(e.X)

		return &c

	case *Binary:
		c := *e
		c.S, c.X, c.Y = nil, walk(e.X), walk(e.Y)

		return &c

	case *Compare:
		c := *e
		c.S, c.X, c.Y = nil, walk(e.X), walk(e.Y)

		return &c

	case *Convert:
		c := *e
		c.S, c.X = nil, walk(e.X)

		return &c

	case *Concat:
		c := *e
		c.S, c.Parts = nil, make([]Expr, len(e.Parts))

		for i, p := range e.Parts {
			c.Parts[i] = walk(p)
		}

		return &c

	case *Substring:
		c := *e
		c.S, c.X = nil, walk(e.X)
		c.Lower = scalar(e.Lower)

		if e.Upper != nil {
			c.Upper = scalar(e.Upper)
		}

		return &c
	}

	panic("sema: an array-valued expression Scalarize doesn't know")
}

// Leaves are the arrays an array-valued expression reads, its *ArrayRefs,
// *Sections, *ArrayConsts, and *Constructors, left to right.
func Leaves(e Expr) []Expr {
	var list []Expr

	Scalarize(e, func(leaf Expr) Expr {
		list = append(list, leaf)

		return leaf
	}, func(x Expr) Expr { return x })

	return list
}

// NewArray is an array of type t that no unit declared, with the given
// extents, each dimension's lower bound 1: one of lowering's own, an array
// temporary, which lowering gives a place.
func NewArray(name string, t Type, extents []int) *Symbol {
	s := &Symbol{Name: name, Class: Array, Type: t, Offset: -1}

	for _, n := range extents {
		s.Dims = append(s.Dims, Dim{Lower: 1, Upper: n})
	}

	return s
}

// NewArrayRef is the whole array s as an array-valued expression, with its
// shape.
func NewArrayRef(s *Symbol, pos diag.Position) *ArrayRef {
	r := &ArrayRef{exprBase: exprBase{T: s.Type, Pos: pos}, Sym: s}
	r.S = arrayShape(r)

	return r
}
