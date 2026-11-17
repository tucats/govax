package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Shapes (Fortran 90's 2.4.5 and 7.1.4; docs/planning/PHASE-102 - Fortran 90
// array expressions.md, findings 3 and 4). Every expression has a shape: nil
// for a scalar, otherwise an Extent per dimension, the leftmost first. A whole
// array's extents are its dimensions'; a section's, its triplets' counts; an
// elemental operation's, its array operands'. A scalar conforms with anything,
// standing for an array of the other operand's shape; two arrays conform when
// they have the same rank and the same extent in each dimension.
//
// An extent is either known when compiling (a constant), or computed when the
// statement that uses it starts: an adjustable array's dimension, a triplet
// whose bounds aren't constant. Then Of and Dim say whose: lowering computes it
// from that array or section. Two extents not both known are checked against
// each other only by /CHECK=BOUNDS, when the program runs (finding 15).

// Shape is an expression's shape: its extents, the leftmost dimension first,
// or nil for a scalar.
type Shape []Extent

// Extent is one dimension's number of elements: Size, if Known when
// compiling; otherwise dimension Dim of Of, an *ArrayRef (the array's
// dimension Dim) or a *Section (its subscript Dim, a triplet), which lowering
// computes when the statement starts.
type Extent struct {
	Size  int
	Known bool
	Of    Expr
	Dim   int
}

// Rank is the shape's number of dimensions, 0 for a scalar.
func (s Shape) Rank() int { return len(s) }

// String is the shape as a message shows it, (10, 3), each extent not known
// when compiling a colon.
func (s Shape) String() string {
	if len(s) == 0 {
		return "a scalar"
	}

	parts := make([]string, len(s))
	for i, e := range s {
		parts[i] = ":"
		if e.Known {
			parts[i] = strconv.Itoa(e.Size)
		}
	}

	return "(" + strings.Join(parts, ", ") + ")"
}

// Size is the number of elements a shape holds, if every extent is known.
func (s Shape) Size() (int, bool) {
	n := 1

	for _, e := range s {
		if !e.Known {
			return 0, false
		}

		n *= e.Size
	}

	return n, true
}

// rankOf is e's rank, 0 for a scalar.
func rankOf(e Expr) int { return len(e.Shape()) }

// conform is the shape of an elemental operation on operands of the given
// shapes: the arrays', a scalar standing for any. Arrays of different ranks,
// or with extents known to differ, don't conform: NOTCONFORM, at pos, and ok
// is false. Where one array's extent is known and another's isn't, the known
// one is the result's.
func (a *analyzer) conform(pos diag.Position, shapes ...Shape) (Shape, bool) {
	var result Shape

	for _, s := range shapes {
		switch {
		case s == nil:
			continue

		case result == nil:
			result = s

			continue

		case len(s) != len(result):
			a.report("NOTCONFORM", pos, result.String(), s.String())

			return nil, false
		}

		merged := make(Shape, len(s))

		for i := range s {
			x, y := result[i], s[i]

			switch {
			case x.Known && y.Known && x.Size != y.Size:
				a.report("NOTCONFORM", pos, result.String(), s.String())

				return nil, false
			case x.Known:
				merged[i] = x
			default:
				merged[i] = y
			}
		}

		result = merged
	}

	return result, true
}

// conforms is conform of the expressions' shapes.
func (a *analyzer) conforms(pos diag.Position, list ...Expr) (Shape, bool) {
	shapes := make([]Shape, len(list))
	for i, e := range list {
		shapes[i] = e.Shape()
	}

	return a.conform(pos, shapes...)
}

// arrayShape is a whole array's shape: each dimension's extent, known for a
// constant one, for an adjustable one dimension i of r. An assumed-size
// array's last dimension has no extent: callers refuse it first (NOSHAPE).
func arrayShape(r *ArrayRef) Shape {
	s := make(Shape, len(r.Sym.Dims))

	for i, d := range r.Sym.Dims {
		if d.known() {
			s[i] = Extent{Size: max(d.Extent(), 0), Known: true}
		} else {
			s[i] = Extent{Of: r, Dim: i}
		}
	}

	return s
}

// wholeArray is a whole array as an array-valued expression: an ArrayRef with
// its shape, or after an error a bad expression. An assumed-size array has no
// shape to give (NOSHAPE), nor an array of records one this phase compiles.
func (a *analyzer) wholeArray(s *Symbol, n *ast.Name) Expr {
	switch {
	case s.AssumedSize():
		a.report("NOSHAPE", n.Pos, s.Name)

		return a.bad(n.Pos)

	case s.Struct != nil:
		a.notImplForm(n.Pos, "an array of records as a whole")

		return a.bad(n.Pos)
	}

	r := &ArrayRef{exprBase: exprBase{T: s.Type, Pos: n.Pos}, Sym: s}
	r.S = arrayShape(r)

	return r
}

// scalarValue is v where a scalar must be: NOTSCALAR if it is an array.
func (a *analyzer) scalarValue(v Expr) Expr {
	if rankOf(v) == 0 {
		return v
	}

	a.report("NOTSCALAR", v.Position())

	return a.bad(v.Position())
}
