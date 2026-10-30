package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Arrays (sections 2.2.3, 4.5, 6.1.1). An array declarator gives a name
// one to seven dimensions, each with a lower and an upper bound, [dl:]du.
// The bounds come in three kinds:
//
//   - constant: DIMENSION A(10), B(0:9, -1:1), C(2*N) where N is a
//     PARAMETER constant. The array's size is known, and if it isn't a
//     dummy argument it gets storage of that size.
//   - adjustable: SUBROUTINE S(A, M, N) with DIMENSION A(M, N). The bound
//     is an expression of constants, dummy arguments, and variables in
//     common blocks, computed when the subprogram is entered. Only a dummy
//     array may have one, since only a dummy array has its storage from
//     somewhere else.
//   - assumed-size: DIMENSION A(M, *). The last upper bound is *: the
//     array is as long as the caller's. The other bounds are found as they
//     would be for any array, since each element's place depends on every
//     dimension but the last.
//
// Elements are stored in column-major order (section 2.2.3.3): the first
// subscript varies fastest. In A(l1:u1, l2:u2, l3:u3), with n1 = u1-l1+1
// and n2 = u2-l2+1, the element A(s1, s2, s3) is number
//
//	(s1 - l1) + (s2 - l2)*n1 + (s3 - l3)*n1*n2
//
// counting from 0. So in DIMENSION BOS(3,3,3), BOS(2,1,3) is number
// 1 + 0 + 2*9 = 19, the 20th, as the manual's Figure 2-1 draws it.
// ElementNumber is this sum for constant subscripts, which EQUIVALENCE
// and DATA need; lowering (ir's mem) makes the same sum into code for
// the others. The last dimension's extent is never in the sum, which is
// why an assumed-size array's can be unknown.

// maxDimensions is the most dimensions an array can have (section 2.2.3).
const maxDimensions = 7

// maxArrayBytes is the largest array: a VAX address reaches no further,
// and a class A descriptor's size wouldn't hold it as a positive number.
const maxArrayBytes = 1<<31 - 1

// adjustableArray is an array declared with bounds that aren't constant,
// to be checked when every declaration has been read (checkArrays). Its
// names' declarations may follow it: DIMENSION A(N) may come before
// COMMON N, and N's type may be declared after it, if that doesn't change
// the type.
type adjustableArray struct {
	sym *Symbol

	// pos is the first bound that isn't constant.
	pos diag.Position

	// names are the names in those bounds, as they are written.
	names []*ast.Name
}

// dimension gives sym the dimensions its array declarator d declares
// (sections 2.2.3.1, 4.5). Whatever goes wrong with its bounds, sym is an
// array, so its uses are elements; it is broken, so they get no messages.
func (a *analyzer) dimension(sym *Symbol, d *ast.Declarator) {
	switch {
	case sym.Class == UnitName || sym.Result || sym.Class == Constant:
		a.badUse(sym, d.Pos, "be an array")

		sym.broken = true

		return

	case sym.Dims != nil:
		a.report("DUPDIM", d.Pos, d.Name, sym.DimPos.Line)

		return
	}

	dims := make([]Dim, len(d.Dims))

	sym.Class, sym.DimPos = Array, d.Pos
	sym.Dims = dims

	if len(d.Dims) > maxDimensions {
		a.report("TOOMANYDIMS", d.Pos, len(d.Dims))

		sym.broken = true

		return
	}

	adj := &adjustableArray{sym: sym}

	for i, b := range d.Dims {
		dim, ok := a.dimensionBounds(adj, b, i == len(d.Dims)-1, d.Pos)
		if !ok {
			sym.broken = true

			return
		}

		dims[i] = dim
	}

	if adj.names != nil {
		a.adjustable = append(a.adjustable, adj)
	}
}

// dimensionBounds is one dimension's bounds.
func (a *analyzer) dimensionBounds(adj *adjustableArray, b ast.Bound, last bool, pos diag.Position) (Dim, bool) {
	dim := Dim{Lower: 1}

	var ok bool

	if b.Lower != nil {
		dim.Lower, dim.LowerExpr, ok = a.bound(adj, b.Lower)
		if !ok {
			return dim, false
		}
	}

	if b.Star {
		if adj.sym.Dummy == 0 || !last {
			a.report("ASSUMEDSIZE", pos)

			return dim, false
		}

		dim.Assumed = true

		return dim, true
	}

	dim.Upper, dim.UpperExpr, ok = a.bound(adj, b.Upper)
	if !ok {
		return dim, false
	}

	if dim.LowerExpr == nil && dim.UpperExpr == nil && dim.Upper < dim.Lower {
		a.report("BOUNDORDER", b.Upper.Position(), dim.Upper, dim.Lower)

		return dim, false
	}

	return dim, true
}

// bound is an array bound's value if it is constant, converted to INTEGER
// (section 2.2.3.1: DIMENSION A(2.5) has two elements), and its expression
// if it isn't, for an adjustable array: then its names go on adj.
func (a *analyzer) bound(adj *adjustableArray, e ast.Expr) (int, ast.Expr, bool) {
	ev := a.declEval()

	c, err := ev.eval(e)

	switch {
	case err == nil:
		v, ok := a.constantBound(ev, c, e.Position())

		return v, nil, ok

	case err.kind == constFailed:
		return 0, nil, false

	case adj.sym.Dummy == 0:
		a.report("ADJNOTDUMMY", e.Position(), adj.sym.Name)

		return 0, nil, false
	}

	before := len(adj.names)

	if !a.boundOperands(adj, e) {
		return 0, nil, false
	}

	if len(adj.names) == before {
		// Not constant, for a reason that isn't a name: 2.0 ** 0.5.
		a.report("BADBOUND", e.Position())

		return 0, nil, false
	}

	if before == 0 {
		adj.pos = e.Position()
	}

	return 0, e, true
}

// constantBound is the INTEGER value of a constant bound of value c.
func (a *analyzer) constantBound(ev *evaluator, c *Const, pos diag.Position) (int, bool) {
	if c.T.Kind != Integer && c.T.Kind != Real {
		a.report("BADBOUND", pos)

		return 0, false
	}

	c, err := ev.convert(c, IntegerType, pos)
	if err != nil {
		if err.kind != constFailed {
			a.report("BADBOUND", pos)
		}

		return 0, false
	}

	return int(c.Int), true
}

// boundOperands checks the form of a bound that isn't constant: it is
// made of the arithmetic operators, constants, and names (section
// 2.2.3.1), and "array references and references to user-defined
// functions" are not allowed. The names go on adj for checkArrays, once
// it is known what each is, and on boundVars for checkBoundType.
func (a *analyzer) boundOperands(adj *adjustableArray, e ast.Expr) bool {
	if c, err := (&evaluator{a: a, quiet: true, typeless: true}).eval(e); err == nil {
		if c.T.Kind != Integer && c.T.Kind != Real {
			a.report("BADBOUND", e.Position())

			return false
		}

		return true
	}

	switch e := e.(type) {
	case *ast.Paren:
		return a.boundOperands(adj, e.X)

	case *ast.Unary:
		if e.Op == ast.Add || e.Op == ast.Sub {
			return a.boundOperands(adj, e.X)
		}

	case *ast.Binary:
		switch e.Op {
		case ast.Add, ast.Sub, ast.Mul, ast.Div, ast.Pow:
			// Both, to find what's wrong with each.
			x := a.boundOperands(adj, e.X)
			y := a.boundOperands(adj, e.Y)

			return x && y
		}

	case *ast.Name:
		adj.names = append(adj.names, e)

		s := a.lookup(e.Name, e.Pos)
		if _, seen := a.boundVars[s]; !seen {
			a.boundVars[s] = e.Pos
		}

		return true

	case *ast.Apply:
		a.report("BOUNDREF", e.Pos, e.Name)

		return false
	}

	a.report("BADBOUND", e.Position())

	return false
}

// checkBoundType reports a type statement giving t to a name already used
// in an array's bounds, if that changes its type (section 2.2.3.1;
// section 6.1.1.1's invalid example has DIMENSION A(X) with INTEGER X).
func (a *analyzer) checkBoundType(s *Symbol, t Type, ok bool, pos diag.Position) {
	if first, used := a.boundVars[s]; used && ok && t != s.Type {
		a.report("BOUNDTYPE", pos, s.Name, first.Line)
	}
}

// checkArrays checks what depends on every declaration, once they have
// been read: each array's size, whose element type may be declared after
// its dimensions, and the names in each adjustable array's bounds. An
// adjustable array with nothing wrong is NOTIMPL, at its first bound that
// isn't constant: its code is Phase 79's. It stays an array whose uses are
// checked.
func (a *analyzer) checkArrays() {
	for _, s := range a.unit.Symbols {
		if s.Class == Array && !s.broken && tooLarge(s) {
			a.report("ARRAYSIZE", s.DimPos, s.Name)

			s.broken = true
		}
	}

	for _, adj := range a.adjustable {
		ok := true

		for _, n := range adj.names {
			ok = a.checkBoundName(n) && ok
		}

		if ok {
			a.notImplForm(adj.pos, "an adjustable array")
		} else {
			adj.sym.broken = true
		}
	}
}

// checkBoundName checks a name in an adjustable array's bound: a dummy
// argument or a variable in a common block (section 6.1.1.1), of a type
// that converts to INTEGER. Until COMMON is read (subtask 6), a unit with
// a COMMON statement is incomplete, and these messages aren't reported in
// it.
func (a *analyzer) checkBoundName(n *ast.Name) bool {
	s := a.unit.byName[n.Name]

	switch {
	case s.broken:
		return false

	case s.Class == Array || s.Class == Function || s.Class == StmtFunction:
		a.report("BOUNDREF", n.Pos, n.Name)

		return false

	case s.Dummy == 0:
		// A constant among them, defined after the bound that uses it.
		a.report("BOUNDVAR", n.Pos, n.Name)

		return false

	case s.Type.Kind == Invalid:
		// No type (IMPLICIT NONE): NOTYPE says so.
		return true

	case s.Type.Kind != Integer && s.Type.Kind != Real:
		a.report("BADBOUND", n.Pos)

		return false
	}

	return true
}

// tooLarge reports whether s's known dimensions alone take more than
// maxArrayBytes. It stops as soon as they do, so that the product of seven
// large extents never overflows.
func tooLarge(s *Symbol) bool {
	n := s.Type.Size
	if n == 0 {
		// CHARACTER*(*): at least a byte each.
		n = 1
	}

	for _, d := range s.Dims {
		if !d.known() {
			continue
		}

		if d.Extent() > maxArrayBytes/n {
			return true
		}

		n *= d.Extent()
	}

	return false
}

// ElementNumber is the place in storage order, from 0, of the element of
// s whose subscripts are the constants subs (the sum above), or false if
// the element isn't in the array: a subscript outside its bounds, or the
// wrong number of them. An assumed-size array's last upper bound is
// unknown, so any subscript at or above its lower bound will do. An
// adjustable array's elements have no constant place. EQUIVALENCE and DATA
// use this for their constant subscripts.
func (s *Symbol) ElementNumber(subs []int) (int, bool) {
	if len(subs) != len(s.Dims) {
		return 0, false
	}

	number, multiplier := 0, 1

	for i, d := range s.Dims {
		switch {
		case d.LowerExpr != nil || d.UpperExpr != nil:
			return 0, false
		case subs[i] < d.Lower:
			return 0, false
		case !d.Assumed && subs[i] > d.Upper:
			return 0, false
		}

		number += (subs[i] - d.Lower) * multiplier
		multiplier *= d.Extent()
	}

	return number, true
}
