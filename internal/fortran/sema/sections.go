package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

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
	if f, ok := e.(*ast.FieldRef); ok {
		return a.componentSubstring(f)
	}

	ap, ok := e.(*ast.Apply)
	if !ok {
		return e
	}

	if s := a.scope.find(ap.Name); s != nil && (s.Class == Array || s.ArrayValue != nil) {
		return e
	}

	return substringReading(e)
}

// componentSubstring is substringForm for a field reference, whose
// parenthesized list the parser reads as a name's (R.A(1:3), P%NAME(2:4)): one
// triplet without a stride after a field that isn't an array is the field's
// substring, as Fortran 77 read it; anything else stays the reference, for
// field to read as an element or a section. A field that can't be found is
// left to field's messages.
func (a *analyzer) componentSubstring(e *ast.FieldRef) ast.Expr {
	if len(e.Args) != 1 {
		return e
	}

	t, ok := e.Args[0].(*ast.Triplet)
	if !ok || t.Stride != nil {
		return e
	}

	if f := a.componentOf(e); f == nil || f.Dims != nil || f.Rank > 0 {
		return e
	}

	whole := *e
	whole.Args = nil

	return &ast.Substring{ExprBase: e.ExprBase, X: &whole, Lower: t.Lower, Upper: t.Upper}
}

// componentOf is the field the reference e names, found quietly from the
// declarations, or nil if it can't be: no record at its start, or a name no
// structure on the way has.
func (a *analyzer) componentOf(e *ast.FieldRef) *Field {
	var st *Structure

	switch x := e.X.(type) {
	case *ast.FieldRef:
		f := a.componentOf(x)
		if f == nil {
			return nil
		}

		st = f.Struct

	case *ast.Name:
		st = a.structureNamed(x.Name)

	case *ast.Apply:
		st = a.structureNamed(x.Name)
	}

	if st == nil {
		return nil
	}

	return st.Field(e.Field)
}

// structureNamed is the structure of the record, object, or derived-type
// constant the name is in scope, or nil.
func (a *analyzer) structureNamed(name string) *Structure {
	s := a.scope.find(name)

	switch {
	case s == nil:
		return nil
	case s.Struct != nil:
		return s.Struct
	case s.Type.Kind == Record:
		return s.Type.Struct
	}

	return nil
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

// SectionSub is one subscript of an array section: a subscript, Index, in a
// dimension the section doesn't range over; a vector subscript, Vector, an
// INTEGER array of rank one whose elements are the subscripts the section
// takes in turn (Fortran 90's 6.2.2.3.2); or a triplet (Index and Vector nil),
// whose Lower, Upper, and Stride are INTEGER. A bound left out is the
// dimension's: a constant for a constant bound, or nil for an adjustable
// array's, which lowering reads from the array's bounds as the program runs.
// Stride is 1 when it is left out.
type SectionSub struct {
	Index                Expr
	Vector               Expr
	Lower, Upper, Stride Expr
}

// Triplet reports whether the subscript is a triplet.
func (s SectionSub) Triplet() bool { return s.Index == nil && s.Vector == nil }

// Ranges reports whether the section ranges over the subscript's dimension: a
// triplet or a vector subscript, each a dimension of the section's shape.
func (s SectionSub) Ranges() bool { return s.Index == nil }

// section analyzes s(subscripts), an element or an array section (Fortran
// 90's 6.2.2): one subscript, triplet, or vector subscript per dimension, each
// bound, stride, and subscript made INTEGER, as an element's subscripts are.
// With no triplet and no vector subscript it is an element, an *Elem. A
// section's shape has an extent per triplet, MAX((u - l + s) / s, 0) (Table
// 6.1's note), known when l, u, and s are constants, and per vector
// subscript, the vector's. Only the subscripts a triplet uses need be within
// the dimension (B(3:11:7) of B(10) is fine), so a constant triplet's first
// and last are checked as an element's constant subscripts are
// (SUBSCRIPTRANGE).
func (a *analyzer) section(s *Symbol, e *ast.Apply) Expr {
	if len(e.Args) != len(s.Dims) {
		a.report("SUBSCRIPTS", e.Pos, e.Name, count(len(s.Dims), "subscript"), len(e.Args))

		return a.bad(e.Pos)
	}

	if a.nullSubscript(e.Pos, e.Args) {
		return a.bad(e.Pos)
	}

	sec := &Section{exprBase: exprBase{T: s.Type, Pos: e.Pos}, Sym: s}

	subs, shape, ok := a.sectionList(s, e.Name, s.Dims, e.Pos, e.Args, sec)
	if !ok {
		return a.bad(e.Pos)
	}

	sec.Subs, sec.S = subs, shape

	if sec.S != nil {
		return sec
	}

	return &Elem{exprBase: exprBase{T: s.Type, Pos: e.Pos}, Sym: s, Subs: indexes(subs)}
}

// indexes are the subscripts of a list with no triplet and no vector
// subscript, an element's.
func indexes(subs []SectionSub) []Expr {
	list := make([]Expr, len(subs))
	for i, sub := range subs {
		list[i] = sub.Index
	}

	return list
}

// sectionList analyzes the subscripts args, one for each of dims, at pos: an
// array's, s's, or a field's (s nil), name naming it in messages. It is shared
// by an array's sections and a component's (record.go). Each subscript,
// triplet, or vector subscript is analyzed as section says, and shape is the
// section's, an extent for each triplet and vector subscript, nil if there are
// none (an element); of is the expression an extent not known when compiling
// is computed from (Extent.Of), whose subscript i is the triplet.
func (a *analyzer) sectionList(s *Symbol, name string, dims []Dim, pos diag.Position, args []ast.Expr,
	of Expr) (subs []SectionSub, shape Shape, ok bool) {
	subs = make([]SectionSub, len(args))

	// A subscript with an error doesn't stop the others' analysis, so each
	// gets its messages.
	failed := false

	for i, arg := range args {
		t, isTriplet := arg.(*ast.Triplet)
		if !isTriplet {
			v := a.convert(a.arithmetic(a.arrayValue(arg)), IntegerType)
			if v.Type().Kind == Invalid {
				failed = true

				continue
			}

			switch rankOf(v) {
			case 0:
				if c, ok := constantInt(v); ok {
					a.checkSubscript(s, name, dims, i, c, v.Position())
				}

				subs[i] = SectionSub{Index: v}

			case 1:
				subs[i] = SectionSub{Vector: v}
				shape = append(shape, v.Shape()[0])

				a.extensions.add(v.Position(), "a vector subscript", f90)

			default:
				a.report("VECTORRANK", v.Position(), rankOf(v))

				failed = true
			}

			continue
		}

		sub, ok := a.triplet(dims[i], name, pos, t)
		if !ok {
			return nil, nil, false
		}

		// Only here is a triplet known to make a section, not a substring.
		a.extensions.add(pos, "an array section", f90)

		subs[i] = sub
		shape = append(shape, a.tripletExtent(s, name, dims, sub, of, i))
	}

	if failed {
		return nil, nil, false
	}

	return subs, shape, true
}

// triplet analyzes the triplet t, a subscript of the dimension d of what name
// names, at pos.
func (a *analyzer) triplet(d Dim, name string, pos diag.Position, t *ast.Triplet) (SectionSub, bool) {
	sub := SectionSub{}

	bound := func(x ast.Expr) Expr {
		v := a.convert(a.arithmetic(a.value(x)), IntegerType)
		if v.Type().Kind == Invalid {
			return nil
		}

		return v
	}

	intConst := func(v int) Expr {
		return &Const{exprBase: exprBase{T: IntegerType, Pos: t.Pos}, Int: int32(v)}
	}

	switch {
	case t.Lower != nil:
		if sub.Lower = bound(t.Lower); sub.Lower == nil {
			return sub, false
		}
	case d.LowerKnown():
		sub.Lower = intConst(d.Lower)
	}

	switch {
	case t.Upper != nil:
		if sub.Upper = bound(t.Upper); sub.Upper == nil {
			return sub, false
		}
	case d.Assumed:
		a.report("SECTIONEND", pos, name)

		return sub, false
	case d.UpperKnown():
		sub.Upper = intConst(d.Upper)
	}

	sub.Stride = intConst(1)

	if t.Stride != nil {
		if sub.Stride = bound(t.Stride); sub.Stride == nil {
			return sub, false
		}

		if v, ok := constantInt(sub.Stride); ok && v == 0 {
			a.report("ZEROSTRIDE", t.Stride.Position())

			return sub, false
		}
	}

	return sub, true
}

// tripletExtent is the extent of sub, a triplet, the subscript i of of (a
// section of s, or a component of a field with dims): known when its bounds
// and stride are constants, when the subscripts it uses are checked against
// the dimension too.
func (a *analyzer) tripletExtent(s *Symbol, name string, dims []Dim, sub SectionSub, of Expr, i int) Extent {
	l, okl := constantInt(sub.Lower)
	u, oku := constantInt(sub.Upper)
	st, oks := constantInt(sub.Stride)

	if !okl || !oku || !oks {
		return Extent{Of: of, Dim: i}
	}

	n := max((u-l+st)/st, 0)

	if n > 0 {
		a.checkSubscript(s, name, dims, i, l, sub.Lower.Position())
		a.checkSubscript(s, name, dims, i, l+(n-1)*st, sub.Upper.Position())
	}

	return Extent{Size: n, Known: true}
}
