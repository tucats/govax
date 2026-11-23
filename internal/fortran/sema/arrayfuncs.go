package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Fortran 90's array functions (13.10.13 to 13.10.19; docs/planning/PHASE-102 -
// Fortran 90 array expressions.md, finding 12), computed in line: the
// reductions SUM, PRODUCT, MAXVAL, MINVAL (ARRAY, DIM=, MASK=), ALL, ANY, and
// COUNT (MASK, DIM=); DOT_PRODUCT; MAXLOC and MINLOC (ARRAY, DIM=, MASK=; DIM=
// is Fortran 95's), which count each subscript from 1, the first extreme
// winning, and give zeros when there is none (the author's answer 5); the inquiries
// SIZE, LBOUND, UBOUND (DIM=), and SHAPE; MATMUL; and TRANSPOSE. Each is an
// ArrayFunc.
//
// A constant one (of an array whose bounds are constants, or of constants) was
// folded already (constarray.go). For the rest, lowering (ir/lowerarrayfunc.go)
// works in one of three ways by the function and its rank:
//
//   - a scalar (a reduction with no DIM=, or of an array of rank one; SIZE;
//     LBOUND and UBOUND with DIM=; DOT_PRODUCT) is computed by loops of its own
//     where it is used, before an array statement's loops;
//   - an element of a reduction with DIM=, or of MATMUL, is computed at each
//     element of the statement's loops by an inner loop (ArrayFuncAt), and an
//     element of TRANSPOSE is its argument's element with the indexes
//     swapped: no array temporary;
//   - MAXLOC, MINLOC, SHAPE, and LBOUND and UBOUND without DIM= are a small
//     INTEGER array, one element per dimension of their argument, computed
//     into a temporary before the statement's loops.
//
// MAXLOC and MINLOC with DIM= (Fortran 95's 13.14.65 and 13.14.70) are
// reductions like MAXVAL's: each element the place along DIM of the first
// extreme of its line, counted from 1, or 0 where MASK holds for none of it; of
// an array of rank one, a scalar (Dim 1, no shape).
//
// DIM= that isn't constant on an ARRAY of rank above 1 makes a result whose
// shape depends on its value, computed into a temporary when the program runs
// (transform.go). LBOUND, UBOUND, and SIZE take one that isn't, the result
// being a scalar.

// Element reports whether lowering computes the function's elements one at a
// time where they are used (a reduction with DIM=, MATMUL, TRANSPOSE), rather
// than into a temporary before an array statement's loops.
func (f *ArrayFunc) Element() bool {
	return len(f.S) > 0 && !transformational(f.Name) && (f.Name == "MATMUL" || f.Name == "TRANSPOSE" || f.Dim > 0)
}

// arrayFunction analyzes a reference to one of the array functions, its
// arguments in its dummies' order (intrinsicSlots), nil for one left out.
func (a *analyzer) arrayFunction(e *ast.Apply, args []ast.Expr) Expr {
	arg := func(i int) ast.Expr {
		if i < len(args) {
			return args[i]
		}

		return nil
	}

	f := &ArrayFunc{exprBase: exprBase{Pos: e.Pos}, Name: e.Name}

	// array is argument i, which must be an array. The inquiries and
	// TRANSPOSE take an array of any type, a derived type's too (operand).
	array := func(i int, what string) Expr {
		var x Expr

		switch e.Name {
		case "SIZE", "LBOUND", "UBOUND", "SHAPE", "TRANSPOSE":
			x = a.operand(arg(i))
		default:
			x = a.arrayValue(arg(i))
		}

		if x.Type().Kind == Invalid {
			return nil
		}

		if rankOf(x) == 0 {
			a.report("ARRAYARG", x.Position(), e.Name, what)

			return nil
		}

		return x
	}

	switch e.Name {
	case "SIZE", "LBOUND", "UBOUND", "SHAPE":
		if f.Array = array(0, "ARRAY"); f.Array == nil {
			return a.bad(e.Pos)
		}

		a.noteInquired(f.Array)

		f.T = IntegerType

		if e.Name == "SHAPE" || arg(1) == nil {
			if e.Name != "SIZE" {
				f.S = knownShape(rankOf(f.Array))
			}

			return f
		}

		if !a.arrayDim(f, arg(1), true) {
			return a.bad(e.Pos)
		}

		return f

	case "SUM", "PRODUCT", "MAXVAL", "MINVAL":
		if f.Array = array(0, "ARRAY"); f.Array == nil {
			return a.bad(e.Pos)
		}

		t := f.Array.Type()

		switch {
		case e.Name == "SUM" || e.Name == "PRODUCT":
			if t.Kind != Integer && t.Kind != Real && t.Kind != Complex {
				a.report("BADARG", f.Array.Position(), e.Name, "numeric", t.String())

				return a.bad(e.Pos)
			}
		case t.Kind != Integer && t.Kind != Real:
			a.report("BADARG", f.Array.Position(), e.Name, "INTEGER or REAL", t.String())

			return a.bad(e.Pos)
		}

		if t == complex32 {
			a.notImplForm(e.Pos, complex32Intrinsic)

			return a.bad(e.Pos)
		}

		f.T = t

		if !a.arrayMask(f, arg(2)) || arg(1) != nil && !a.arrayDim(f, arg(1), false) {
			return a.bad(e.Pos)
		}

	case "ALL", "ANY", "COUNT":
		if f.Array = array(0, "MASK"); f.Array == nil {
			return a.bad(e.Pos)
		}

		if f.Array.Type().Kind != Logical {
			a.report("BADARG", f.Array.Position(), e.Name, "LOGICAL", f.Array.Type().String())

			return a.bad(e.Pos)
		}

		f.T = f.Array.Type()
		if e.Name == "COUNT" {
			f.T = a.defaultInt
		}

		if arg(1) != nil && !a.arrayDim(f, arg(1), false) {
			return a.bad(e.Pos)
		}

	case "MAXLOC", "MINLOC":
		if f.Array = array(0, "ARRAY"); f.Array == nil {
			return a.bad(e.Pos)
		}

		if t := f.Array.Type(); t.Kind != Integer && t.Kind != Real {
			a.report("BADARG", f.Array.Position(), e.Name, "INTEGER or REAL", t.String())

			return a.bad(e.Pos)
		}

		dim, mask := arg(1), arg(2)
		f.T = IntegerType

		if dim != nil && mask == nil && !keywordAt(e, 1) {
			// MAXLOC(A, M): Fortran 95 has two forms, (ARRAY, DIM, MASK) and
			// Fortran 90's (ARRAY, MASK), told apart by the second argument's
			// type when it has no keyword.
			v := a.arrayValue(dim)

			switch {
			case v.Type().Kind == Invalid:
				return a.bad(e.Pos)
			case v.Type().Kind == Logical:
				if !a.arrayMaskOf(f, v) {
					return a.bad(e.Pos)
				}

				f.S = knownShape(rankOf(f.Array))

				return f
			}

			if !a.arrayDimOf(f, a.scalarValue(v), dim, false) {
				return a.bad(e.Pos)
			}
		} else if dim != nil && !a.arrayDim(f, dim, false) {
			return a.bad(e.Pos)
		}

		if !a.arrayMask(f, mask) {
			return a.bad(e.Pos)
		}

		if dim == nil {
			f.S = knownShape(rankOf(f.Array))

			return f
		}

		if f.Dim == 0 && f.DimExpr == nil {
			// DIM= of an array of rank one: the place, a scalar.
			f.Dim = 1

			return f
		}

	case "DOT_PRODUCT":
		return a.dotProduct(e, array(0, "VECTOR_A"), array(1, "VECTOR_B"))

	case "MATMUL":
		return a.matmul(f, array(0, "MATRIX_A"), array(1, "MATRIX_B"))

	case "TRANSPOSE":
		if f.Array = array(0, "MATRIX"); f.Array == nil {
			return a.bad(e.Pos)
		}

		if rankOf(f.Array) != 2 {
			a.report("ARRAYRANK", f.Array.Position(), e.Name, "MATRIX", "2", rankOf(f.Array))

			return a.bad(e.Pos)
		}

		s := f.Array.Shape()
		f.T, f.S = f.Array.Type(), Shape{s[1], s[0]}

		return f
	}

	// A reduction: a scalar, or with DIM= the array's shape without that
	// dimension; with a DIM= not constant, of the rank below the array's, each
	// extent the function's own, known when the program runs.
	switch {
	case f.Dim > 0:
		for i, x := range f.Array.Shape() {
			if i != f.Dim-1 {
				f.S = append(f.S, x)
			}
		}

	case f.DimExpr != nil:
		for i := range rankOf(f.Array) - 1 {
			f.S = append(f.S, Extent{Of: f, Dim: i})
		}
	}

	return f
}

// arrayDim analyzes DIM=: a constant within ARRAY's rank (DIMRANGE), or,
// when scalar allows it (SIZE, LBOUND, UBOUND), or ARRAY's rank is one, an
// INTEGER value. A DIM= of an array of rank one is no DIM= at all to a
// reduction: its value is a scalar either way.
func (a *analyzer) arrayDim(f *ArrayFunc, x ast.Expr, scalar bool) bool {
	return a.arrayDimOf(f, a.value(x), x, scalar)
}

// arrayDimOf is arrayDim of DIM's value, dv, analyzed already, x its text.
func (a *analyzer) arrayDimOf(f *ArrayFunc, dv Expr, x ast.Expr, scalar bool) bool {
	rank := rankOf(f.Array)

	d := a.convert(a.arithmetic(dv), IntegerType)
	if d.Type().Kind == Invalid {
		return false
	}

	v, ok := constantInt(d)

	switch {
	case ok && (v < 1 || v > rank):
		a.report("DIMRANGE", d.Position(), f.Name, ast.DumpExpr(x), rank)

		return false

	case ok && !scalar && rank == 1:
		// The whole array's reduction.

	case ok:
		f.Dim = v

	case scalar:
		f.DimExpr = d

	case rank > 1:
		// The result's shape depends on DIM's value: the function is computed
		// into a temporary when the program runs (ir/lowertransform.go).
		f.DimExpr = d
	}

	return true
}

// keywordAt reports whether e's argument i (as written) has a keyword.
func keywordAt(e *ast.Apply, i int) bool {
	if i >= len(e.Args) {
		return false
	}

	_, ok := e.Args[i].(*ast.KeywordArg)

	return ok
}

// arrayMask analyzes MASK=, if there is one: a LOGICAL array that conforms
// with ARRAY, or a LOGICAL scalar.
func (a *analyzer) arrayMask(f *ArrayFunc, x ast.Expr) bool {
	if x == nil {
		return true
	}

	return a.arrayMaskOf(f, a.arrayValue(x))
}

// arrayMaskOf is arrayMask of MASK's value, m, analyzed already.
func (a *analyzer) arrayMaskOf(f *ArrayFunc, m Expr) bool {
	switch {
	case m.Type().Kind == Invalid:
		return false

	case m.Type().Kind != Logical:
		a.report("BADARG", m.Position(), f.Name, "a LOGICAL MASK", m.Type().String())

		return false
	}

	if _, ok := a.conforms(m.Position(), f.Array, m); !ok {
		return false
	}

	f.Mask = m

	return true
}

// dotProduct is DOT_PRODUCT(VECTOR_A, VECTOR_B) (13.13.30): SUM(VECTOR_A *
// VECTOR_B) of numbers, the first's complex conjugate's for COMPLEX values,
// and ANY(VECTOR_A .AND. VECTOR_B) of LOGICAL ones, two arrays of rank one
// that conform.
func (a *analyzer) dotProduct(e *ast.Apply, x, y Expr) Expr {
	if x == nil || y == nil {
		return a.bad(e.Pos)
	}

	for _, v := range []Expr{x, y} {
		if rankOf(v) != 1 {
			a.report("ARRAYRANK", v.Position(), e.Name, "arguments", "1", rankOf(v))

			return a.bad(e.Pos)
		}
	}

	shape, ok := a.conforms(e.Pos, x, y)
	if !ok {
		return a.bad(e.Pos)
	}

	f := &ArrayFunc{exprBase: exprBase{Pos: e.Pos}, Name: "SUM"}

	switch tx, ty := x.Type(), y.Type(); {
	case tx.Kind == Logical && ty.Kind == Logical:
		t := tx
		if ty.Size > t.Size {
			t = ty
		}

		f.Name, f.T = "ANY", t
		f.Array = &Binary{exprBase{T: t, Pos: e.Pos, S: shape}, ast.And, a.convert(x, t), a.convert(y, t)}

	case tx.Kind == Logical || ty.Kind == Logical || tx.Kind == Character || ty.Kind == Character:
		a.report("BADARG", e.Pos, e.Name, "both numeric or both LOGICAL", tx.String()+" and "+ty.String())

		return a.bad(e.Pos)

	default:
		t := operationType(tx, ty)
		if t == complex32 {
			a.notImplForm(e.Pos, complex32Intrinsic)

			return a.bad(e.Pos)
		}

		xs := a.convert(x, t)
		if t.Kind == Complex {
			// The first vector's complex conjugate.
			xs = &IntrinsicRef{exprBase{T: t, Pos: e.Pos, S: shape}, "CONJG", ConjgOp, []Expr{xs}, ""}
		}

		f.T = t
		f.Array = &Binary{exprBase{T: t, Pos: e.Pos, S: shape}, ast.Mul, xs, a.convert(y, t)}
	}

	return f
}

// matmul is MATMUL(MATRIX_A, MATRIX_B) (13.13.62), in any of its three forms:
// a matrix times a matrix, (n, m) by (m, p), (n, p); a vector times a matrix,
// (m) by (m, p), (p); and a matrix times a vector, (n, m) by (m), (n). Numbers
// multiply and add, LOGICAL values .AND. and .OR.; the arguments' extents
// along m conform.
func (a *analyzer) matmul(f *ArrayFunc, x, y Expr) Expr {
	if x == nil || y == nil {
		return a.bad(f.Pos)
	}

	rx, ry := rankOf(x), rankOf(y)

	switch {
	case rx > 2 || ry > 2:
		a.report("ARRAYRANK", f.Pos, f.Name, "arguments", "1 or 2", max(rx, ry))

		return a.bad(f.Pos)

	case rx == 1 && ry == 1:
		a.report("ARRAYRANK", y.Position(), f.Name, "MATRIX_B", "2 when MATRIX_A's is 1", ry)

		return a.bad(f.Pos)
	}

	sx, sy := x.Shape(), y.Shape()

	if _, ok := a.conform(f.Pos, Shape{sx[rx-1]}, Shape{sy[0]}); !ok {
		return a.bad(f.Pos)
	}

	switch tx, ty := x.Type(), y.Type(); {
	case tx.Kind == Logical && ty.Kind == Logical:
		f.T = tx
		if ty.Size > tx.Size {
			f.T = ty
		}

	case tx.Kind == Logical || ty.Kind == Logical || tx.Kind == Character || ty.Kind == Character:
		a.report("BADARG", f.Pos, f.Name, "both numeric or both LOGICAL", tx.String()+" and "+ty.String())

		return a.bad(f.Pos)

	default:
		if f.T = operationType(tx, ty); f.T == complex32 {
			a.notImplForm(f.Pos, complex32Intrinsic)

			return a.bad(f.Pos)
		}
	}

	f.Array, f.Other = a.convert(x, f.T), a.convert(y, f.T)

	switch {
	case rx == 2 && ry == 2:
		f.S = Shape{sx[0], sy[1]}
	case rx == 1:
		f.S = Shape{sy[1]}
	default:
		f.S = Shape{sx[0]}
	}

	return f
}

// WithDim is the reduction f with the constant DIM= d (1 to its array's rank)
// in place of its DIM= that isn't constant: lowering's arm for that value
// (ir/lowertransform.go).
func WithDim(f *ArrayFunc, d int) *ArrayFunc {
	g := *f
	g.Dim, g.DimExpr, g.S = d, nil, nil

	for i, x := range f.Array.Shape() {
		if i != d-1 {
			g.S = append(g.S, x)
		}
	}

	return &g
}

// NewCount is COUNT(MASK) of the LOGICAL array mask, an INTEGER scalar.
func NewCount(mask Expr) *ArrayFunc {
	return &ArrayFunc{exprBase: exprBase{T: IntegerType, Pos: mask.Position()}, Name: "COUNT", Array: mask}
}

// BlankConst is the CHARACTER constant of one blank, EOSHIFT's BOUNDARY when
// none is given: assigned to a longer element, it is padded with blanks.
func BlankConst(pos diag.Position) *Const {
	return &Const{exprBase: exprBase{T: Type{Kind: Character, Size: 1}, Pos: pos}, Text: " "}
}

// Transformed reports whether lowering computes f's whole value into a
// temporary of its shape, known when compiling or only when the program runs:
// one of transform.go's functions, or a reduction with a DIM= not constant.
func (f *ArrayFunc) Transformed() bool {
	return transformational(f.Name) || (f.DimExpr != nil && len(f.S) > 0)
}

// NewArrayFuncAt is f's element at the loops' indexes index, one per
// dimension of f.
func NewArrayFuncAt(f *ArrayFunc, index []*Symbol, pos diag.Position) *ArrayFuncAt {
	return &ArrayFuncAt{exprBase: exprBase{T: f.T, Pos: pos}, F: f, Index: index}
}

// NewBinary is x op y, of type t, both of which have that type.
func NewBinary(op ast.Op, t Type, x, y Expr) Expr {
	return &Binary{exprBase{T: t, Pos: x.Position()}, op, x, y}
}

// LogicalConstAt is the LOGICAL constant v.
func LogicalConstAt(v bool, pos diag.Position) *Const { return logicalConst(v, pos) }

// ZeroConst is the constant 0 of the numeric type t.
func ZeroConst(t Type, pos diag.Position) *Const {
	return &Const{exprBase: exprBase{T: t, Pos: pos}}
}

// OneConst is the constant 1 of the numeric type t (1.0, (1.0, 0.0)).
func OneConst(t Type, pos diag.Position) *Const {
	c := &Const{exprBase: exprBase{T: t, Pos: pos}, Int: 1}
	if t.Kind == Real || t.Kind == Complex {
		c.Int, c.Real = 0, vaxfloat.FromInt(1)
	}

	return c
}
