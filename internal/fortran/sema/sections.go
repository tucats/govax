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

// SectionSub is one subscript of an array section: a subscript, Index, in a
// dimension the section doesn't range over, or a triplet (Index nil), whose
// Lower, Upper, and Stride are INTEGER. A bound left out is the dimension's:
// a constant for a constant bound, or nil for an adjustable array's, which
// lowering reads from the array's bounds as the program runs. Stride is 1 when
// it is left out.
type SectionSub struct {
	Index                Expr
	Lower, Upper, Stride Expr
}

// Triplet reports whether the subscript is a triplet.
func (s SectionSub) Triplet() bool { return s.Index == nil }

// section analyzes an array section, s(subscripts) with a triplet in at least
// one dimension (Fortran 90's 6.2.2.3): one subscript or triplet per
// dimension, each bound and stride made INTEGER, as an element's subscripts
// are. Its shape has an extent per triplet, MAX((u - l + s) / s, 0) (Table
// 6.1's note), known when l, u, and s are constants. Only the subscripts a
// triplet uses need be within the dimension (B(3:11:7) of B(10) is fine), so
// a constant triplet's first and last are checked as an element's constant
// subscripts are (SUBSCRIPTRANGE). A vector subscript, an array-valued
// subscript, is subtask 6's.
func (a *analyzer) section(s *Symbol, e *ast.Apply) Expr {
	if len(e.Args) != len(s.Dims) {
		a.report("SUBSCRIPTS", e.Pos, e.Name, count(len(s.Dims), "subscript"), len(e.Args))

		return a.bad(e.Pos)
	}

	if a.nullSubscript(e.Pos, e.Args) {
		return a.bad(e.Pos)
	}

	sec := &Section{exprBase: exprBase{T: s.Type, Pos: e.Pos}, Sym: s, Subs: make([]SectionSub, len(e.Args))}

	for i, arg := range e.Args {
		t, ok := arg.(*ast.Triplet)
		if !ok {
			v := a.convert(a.arithmetic(a.subscriptValue(arg)), IntegerType)
			if v.Type().Kind == Invalid {
				return a.bad(e.Pos)
			}

			if c, ok := constantInt(v); ok {
				a.checkSubscript(s, e.Name, s.Dims, i, c, v.Position())
			}

			sec.Subs[i] = SectionSub{Index: v}

			continue
		}

		sub, ok := a.triplet(s, e, i, t)
		if !ok {
			return a.bad(e.Pos)
		}

		sec.Subs[i] = sub
		sec.S = append(sec.S, a.tripletExtent(s, e.Name, sec, i))
	}

	return sec
}

// triplet analyzes the triplet t, the section's subscript i of s.
func (a *analyzer) triplet(s *Symbol, e *ast.Apply, i int, t *ast.Triplet) (SectionSub, bool) {
	d := s.Dims[i]
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
	case d.LowerExpr == nil:
		sub.Lower = intConst(d.Lower)
	}

	switch {
	case t.Upper != nil:
		if sub.Upper = bound(t.Upper); sub.Upper == nil {
			return sub, false
		}
	case d.Assumed:
		a.report("SECTIONEND", e.Pos, e.Name)

		return sub, false
	case d.UpperExpr == nil:
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

// tripletExtent is the extent of the section's subscript i, a triplet: known
// when its bounds and stride are constants, when the subscripts it uses are
// checked against the dimension too.
func (a *analyzer) tripletExtent(s *Symbol, name string, sec *Section, i int) Extent {
	sub := sec.Subs[i]

	l, okl := constantInt(sub.Lower)
	u, oku := constantInt(sub.Upper)
	st, oks := constantInt(sub.Stride)

	if !okl || !oku || !oks {
		return Extent{Of: sec, Dim: i}
	}

	n := max((u-l+st)/st, 0)

	if n > 0 {
		a.checkSubscript(s, name, s.Dims, i, l, sub.Lower.Position())
		a.checkSubscript(s, name, s.Dims, i, l+(n-1)*st, sub.Upper.Position())
	}

	return Extent{Size: n, Known: true}
}

// subscriptValue analyzes a subscript: a scalar, or an INTEGER array of rank
// one, a vector subscript (Fortran 90's 6.2.2.3.2), which is subtask 6's.
func (a *analyzer) subscriptValue(e ast.Expr) Expr {
	v := a.arrayValue(e)
	if rankOf(v) == 0 {
		return v
	}

	a.notImplForm(e.Position(), "a vector subscript")

	return a.bad(e.Position())
}
