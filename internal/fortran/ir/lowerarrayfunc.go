package ir

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/sema"
)

// Fortran 90's array functions in line (sema/arrayfuncs.go;
// docs/planning/PHASE-102 - Fortran 90 array expressions.md, finding 12).
//
// A reduction is a nest of loops over its argument, an accumulator in a
// temporary, given the function's value for an empty array first (SUM 0,
// PRODUCT 1, MAXVAL the most negative number of the type, MINVAL the most
// positive, COUNT 0, ANY false, ALL true), and each element, where MASK holds,
// added in. So X = SUM(A, MASK=A > 0) is
//
//	$R = 0.0
//	DO $I1 = 1, 10
//	  IF (A($I1) > 0) $R = $R + A($I1)
//	END DO
//	X = $R
//
// With DIM= its element at the indexes of an array statement's loops is the
// same nest over that one dimension, the others fixed at the statement's
// indexes: the statement's index variables are the inner nest's for those
// dimensions (reduce's mapping), so B = SUM(A, DIM=1) of A(3, 4) is
//
//	DO $I1 = 1, 4
//	  $R = 0.0
//	  DO $I2 = 1, 3
//	    $R = $R + A($I2, $I1)
//	  END DO
//	  B($I1) = $R
//	END DO
//
// An element of MATMUL is the same, the sum of products along the dimension
// the two arguments share. TRANSPOSE needs no loop: its element is its
// argument's with the indexes swapped (prepareFunc's mapping). MAXLOC, MINLOC,
// SHAPE, LBOUND, and UBOUND are computed into a small INTEGER temporary before
// the statement's loops.

// prepareFunc makes info, the array function f's (one whose elements are
// computed where they are used), ready: a dimension per dimension of f's,
// ranging over the loops loopOf says, and f's arguments' arrays prepared as
// leaves too, ranging over the loops their dimensions are f's, so that an
// extent of f's is theirs. A dimension f reduces, or MATMUL's shared one,
// ranges over no loop of the nest's (-1).
func (n *arrayNest) prepareFunc(info *arrayLeaf, f *sema.ArrayFunc, loopOf func(int) int) {
	for i := range f.S {
		info.dims = append(info.dims, leafDim{ranges: true, loop: loopOf(i)})
	}

	prepare := func(x sema.Expr, loops []int) {
		if x == nil || len(x.Shape()) == 0 {
			return
		}

		for _, leaf := range sema.Leaves(x) {
			n.prepareIn(leaf, loops)
		}
	}

	switch ra := len(f.Array.Shape()); {
	case f.Name == "TRANSPOSE":
		prepare(f.Array, []int{loopOf(1), loopOf(0)})

	case f.Name == "MATMUL":
		rb := len(f.Other.Shape())

		switch {
		case ra == 2 && rb == 2:
			prepare(f.Array, []int{loopOf(0), -1})
			prepare(f.Other, []int{-1, loopOf(1)})
		case ra == 1:
			prepare(f.Array, []int{-1})
			prepare(f.Other, []int{-1, loopOf(0)})
		default:
			prepare(f.Array, []int{loopOf(0), -1})
			prepare(f.Other, []int{-1})
		}

	default:
		// A reduction with DIM=: the argument's dimensions but DIM are f's.
		loops := make([]int, ra)

		for d := range ra {
			switch {
			case d < f.Dim-1:
				loops[d] = loopOf(d)
			case d == f.Dim-1:
				loops[d] = -1
			default:
				loops[d] = loopOf(d - 1)
			}
		}

		prepare(f.Array, loops)
		prepare(f.Mask, loops)
	}
}

// funcElement is the array function f's element at the loops' indexes: its
// argument's for TRANSPOSE, else one computed by a loop of its own
// (ArrayFuncAt), at the index variables of f's dimensions' loops.
func (n *arrayNest) funcElement(f *sema.ArrayFunc) sema.Expr {
	if f.Name == "TRANSPOSE" {
		return sema.Scalarize(f.Array, n.element, n.hoist)
	}

	info := n.leaves[f]
	index := make([]*sema.Symbol, len(info.dims))

	for i, d := range info.dims {
		index[i] = n.index[d.loop]
	}

	return sema.NewArrayFuncAt(f, index, f.Pos)
}

// precompute fills temp, an INTEGER array of one element per dimension of f's
// argument, with the value of f: MAXLOC, MINLOC, SHAPE, LBOUND, or UBOUND.
func (l *lowerer) precompute(f *sema.ArrayFunc, temp *sema.ArrayRef) {
	if f.Name == "MAXLOC" || f.Name == "MINLOC" {
		l.locate(f, temp)

		return
	}

	b := l.boundsOf(f.Array)

	for d := range b.rank {
		var v Expr

		switch f.Name {
		case "SHAPE":
			v = b.extent(d)
		case "LBOUND":
			v = b.lower(d)
		default:
			v = b.upper(d)
		}

		l.storeElement(temp, d, v)
	}

	b.n.done()
}

// storeElement stores v in element d+1 of temp, an array temporary of rank one.
func (l *lowerer) storeElement(temp *sema.ArrayRef, d int, v Expr) {
	at := sema.NewElem(temp.Sym, []sema.Expr{sema.IntConstAt(d+1, temp.Pos)}, []bool{true}, temp.Pos)
	l.emit(&Assign{origin: l.origin(), Dst: l.mem(at), Src: v})
}

// arrayFuncValue is where the scalar array function e (an *ArrayFunc of rank
// zero, or an *ArrayFuncAt) has its value, computed now.
func (l *lowerer) arrayFuncValue(e sema.Expr) *Mem {
	switch e := e.(type) {
	case *sema.ArrayFunc:
		switch e.Name {
		case "SIZE", "LBOUND", "UBOUND":
			return l.boundValue(e)
		}

		return l.reduce(e, nil)

	case *sema.ArrayFuncAt:
		if e.F.Name == "MATMUL" {
			return l.matmulAt(e)
		}

		return l.reduce(e.F, e.Index)
	}

	panic("ir: an array function lowering doesn't know")
}

// dimExtent is an extent of an array the nest's leaves are part of: a constant,
// or computed now from the leaf it is of.
func (n *arrayNest) dimExtent(x sema.Extent) sema.Expr {
	if x.Known {
		return sema.IntConstAt(x.Size, n.l.here())
	}

	return n.keepIR(n.leafExtent(x.Of, n.loopOf(x.Of, x.Dim)))
}

// accumulator is a temporary of the array function f's type, and a variable
// of lowering's own standing for it, for the statement.
func (l *lowerer) accumulator(t sema.Type) (*Mem, sema.Expr) {
	m := l.temp(l.typeOf(t))
	sym := sema.NewSymbol("$R", t)
	l.bound[sym] = m

	return m, sema.NewVar(sym, l.here())
}

// reduce computes the reduction f (SUM, PRODUCT, MAXVAL, MINVAL, ALL, ANY,
// COUNT) into a temporary: over every dimension of its argument, or with DIM=
// over that one, its others at the index variables outer (one per dimension
// of f's value, the leftmost first).
func (l *lowerer) reduce(f *sema.ArrayFunc, outer []*sema.Symbol) *Mem {
	arr := f.Array
	rank := len(arr.Shape())

	var free []int

	if f.Dim > 0 {
		free = []int{f.Dim - 1}
	} else {
		for d := range rank {
			free = append(free, d)
		}
	}

	// The nest's loops are the free dimensions'; the statement's index
	// variables follow them in its index list, for the fixed dimensions.
	n := l.newNest(len(free), nil)
	mapping := make([]int, rank)
	j := 0

	for d := range rank {
		if k := slices.Index(free, d); k >= 0 {
			mapping[d] = k

			continue
		}

		n.index = append(n.index, outer[j])
		mapping[d] = len(n.index) - 1
		j++
	}

	for _, x := range []sema.Expr{arr, f.Mask} {
		if x != nil && len(x.Shape()) > 0 {
			for _, leaf := range sema.Leaves(x) {
				n.prepareIn(leaf, mapping)
			}
		}
	}

	n.extents = make([]sema.Expr, len(free))
	for k, d := range free {
		n.extents[k] = n.dimExtent(arr.Shape()[d])
	}

	if outer == nil && l.unit.Checks.Bounds {
		n.checkSubscripts()
	}

	v := sema.Scalarize(arr, n.element, n.hoist)

	var mask sema.Expr
	if f.Mask != nil {
		mask = sema.Scalarize(f.Mask, n.element, n.hoist)
	}

	acc, accVar := l.accumulator(f.T)
	t := acc.T

	switch f.Name {
	case "SUM":
		l.assignScalar(accVar, sema.ZeroConst(f.T, f.Pos))
	case "PRODUCT":
		l.assignScalar(accVar, sema.OneConst(f.T, f.Pos))
	case "MAXVAL", "MINVAL":
		l.emit(&Assign{origin: l.origin(), Dst: acc, Src: extremeOf(t, f.Name == "MINVAL")})
	case "COUNT", "ANY":
		l.emit(&Assign{origin: l.origin(), Dst: acc, Src: IntConst(t, 0)})
	case "ALL":
		l.emit(&Assign{origin: l.origin(), Dst: acc, Src: l.logical(t, true)})
	}

	body := func() {
		switch f.Name {
		case "SUM":
			l.assignScalar(accVar, sema.NewBinary(ast.Add, f.T, accVar, v))
		case "PRODUCT":
			l.assignScalar(accVar, sema.NewBinary(ast.Mul, f.T, accVar, v))
		case "MAXVAL", "MINVAL":
			x := l.temp(t)
			l.emit(&Assign{origin: l.origin(), Dst: x, Src: l.expr(v)})

			op := Gt
			if f.Name == "MINVAL" {
				op = Lt
			}

			l.choose(&Cmp{op, &Load{x}, &Load{acc}}, acc, &Load{x})
		case "COUNT":
			l.choose(l.cond(v), acc, arith(Add, t, &Load{acc}, IntConst(t, 1)))
		case "ANY":
			l.choose(l.cond(v), acc, l.logical(t, true))
		case "ALL":
			l.choose(arith(Not, l.typeOf(v.Type()), l.cond(v), nil), acc, l.logical(t, false))
		}
	}

	n.loops(len(free)-1, func() { l.masked(mask, body) })
	n.done()

	return acc
}

// masked lowers body where mask, a LOGICAL value, holds; everywhere for none.
func (l *lowerer) masked(mask sema.Expr, body func()) {
	if mask == nil {
		body()

		return
	}

	yes, next := l.newBlock(), l.newBlock()
	l.branch(l.cond(mask), yes, next)
	l.begin(yes)
	body()
	l.jump(next)
	l.begin(next)
}

// logical is the LOGICAL constant v of the type t.
func (l *lowerer) logical(t Type, v bool) *Const {
	c := l.constOf(sema.LogicalConstAt(v, l.here()))

	return IntConst(t, c.Int())
}

// extremeOf is the most negative number of the type t, or with positive the
// most positive: MAXVAL's and MINVAL's value for an empty array (Fortran 90's
// 13.13.68 and 13.13.70; the VAX has no infinities, so the largest number's
// negative).
func extremeOf(t Type, positive bool) *Const {
	if t.Floating() {
		h := t.Format().Huge()
		if !positive {
			h = h.Neg()
		}

		return RealConst(t, h)
	}

	bits := uint(t.Size()*8 - 1)
	if positive {
		return IntConst(t, int32(int64(1)<<bits-1))
	}

	return IntConst(t, int32(-(int64(1) << bits)))
}

// matmulAt computes the element of MATMUL at e's indexes: the sum of the
// products of MATRIX_A's row (or the vector) and MATRIX_B's column (or the
// vector), along the dimension they share; of LOGICAL values, the .OR. of the
// .AND.s.
func (l *lowerer) matmulAt(e *sema.ArrayFuncAt) *Mem {
	f := e.F
	ra, rb := len(f.Array.Shape()), len(f.Other.Shape())

	// One loop, over the shared dimension; e's indexes follow it.
	n := l.newNest(1, nil)

	var mapA, mapB []int

	switch {
	case ra == 2 && rb == 2:
		n.index = append(n.index, e.Index[0], e.Index[1])
		mapA, mapB = []int{1, 0}, []int{0, 2}
	case ra == 1:
		n.index = append(n.index, e.Index[0])
		mapA, mapB = []int{0}, []int{0, 1}
	default:
		n.index = append(n.index, e.Index[0])
		mapA, mapB = []int{1, 0}, []int{0}
	}

	for _, leaf := range sema.Leaves(f.Array) {
		n.prepareIn(leaf, mapA)
	}

	for _, leaf := range sema.Leaves(f.Other) {
		n.prepareIn(leaf, mapB)
	}

	n.extents = []sema.Expr{n.dimExtent(f.Array.Shape()[ra-1])}

	x := sema.Scalarize(f.Array, n.element, n.hoist)
	y := sema.Scalarize(f.Other, n.element, n.hoist)

	acc, accVar := l.accumulator(f.T)

	if f.T.Kind == sema.Logical {
		l.emit(&Assign{origin: l.origin(), Dst: acc, Src: IntConst(acc.T, 0)})
	} else {
		l.assignScalar(accVar, sema.ZeroConst(f.T, f.Pos))
	}

	n.loops(0, func() {
		if f.T.Kind == sema.Logical {
			l.assignScalar(accVar, sema.NewBinary(ast.Or, f.T, accVar, sema.NewBinary(ast.And, f.T, x, y)))

			return
		}

		l.assignScalar(accVar, sema.NewBinary(ast.Add, f.T, accVar, sema.NewBinary(ast.Mul, f.T, x, y)))
	})
	n.done()

	return acc
}

// locate computes MAXLOC or MINLOC (Fortran 90's 13.13.65 and 13.13.69) into
// temp: the subscripts, each counted from 1 (the loops' own indexes), of the
// first element in array element order that is greatest (least) where MASK
// holds, or zeros if there is none (the author's answer 5).
func (l *lowerer) locate(f *sema.ArrayFunc, temp *sema.ArrayRef) {
	arr := f.Array
	rank := len(arr.Shape())
	n := l.newNest(rank, nil)

	for _, x := range []sema.Expr{arr, f.Mask} {
		if x != nil && len(x.Shape()) > 0 {
			for _, leaf := range sema.Leaves(x) {
				n.prepare(leaf)
			}
		}
	}

	n.extents = make([]sema.Expr, rank)
	for d := range rank {
		n.extents[d] = n.extent(d, arr, arr)
	}

	if l.unit.Checks.Bounds {
		n.checkSubscripts()
	}

	v := sema.Scalarize(arr, n.element, n.hoist)

	var mask sema.Expr
	if f.Mask != nil {
		mask = sema.Scalarize(f.Mask, n.element, n.hoist)
	}

	t := l.typeOf(arr.Type())
	best, seen := l.temp(t), l.temp(Long)
	where := make([]*Mem, rank)

	l.emit(&Assign{origin: l.origin(), Dst: seen, Src: LongConst(0)})

	for d := range where {
		where[d] = l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: where[d], Src: LongConst(0)})
	}

	op := Gt
	if f.Name == "MINLOC" {
		op = Lt
	}

	n.loops(rank-1, func() {
		l.masked(mask, func() {
			x := l.temp(t)
			l.emit(&Assign{origin: l.origin(), Dst: x, Src: l.expr(v)})

			set, compare, next := l.newBlock(), l.newBlock(), l.newBlock()
			l.branch(&Cmp{Eq, &Load{seen}, LongConst(0)}, set, compare)
			l.begin(compare)
			l.branch(&Cmp{op, &Load{x}, &Load{best}}, set, next)
			l.begin(set)
			l.emit(&Assign{origin: l.origin(), Dst: best, Src: &Load{x}})
			l.emit(&Assign{origin: l.origin(), Dst: seen, Src: LongConst(1)})

			for d := range where {
				l.emit(&Assign{origin: l.origin(), Dst: where[d], Src: &Load{l.symbolMem(n.index[d])}})
			}

			l.jump(next)
			l.begin(next)
		})
	})
	n.done()

	for d := range where {
		l.storeElement(temp, d, &Load{where[d]})
	}
}

// arrayBounds are an array's extents and bounds, computed when the statement
// runs: a nest prepared for its leaves, whose done the caller calls.
type arrayBounds struct {
	l    *lowerer
	n    *arrayNest
	x    sema.Expr
	rank int
	ext  []Expr
}

// boundsOf prepares x's extents.
func (l *lowerer) boundsOf(x sema.Expr) *arrayBounds {
	rank := len(x.Shape())
	n := l.newNest(rank, nil)

	for _, leaf := range sema.Leaves(x) {
		n.prepare(leaf)
	}

	b := &arrayBounds{l: l, n: n, x: x, rank: rank, ext: make([]Expr, rank)}

	for d := range rank {
		b.ext[d] = l.expr(n.extent(d, x, x))
	}

	return b
}

// extent is dimension d's extent.
func (b *arrayBounds) extent(d int) Expr { return b.ext[d] }

// lower is dimension d's lower bound: a whole array's own, or 1 if its extent
// is zero, and 1 for any other array (Fortran 90's 13.13.52).
func (b *arrayBounds) lower(d int) Expr {
	r, ok := b.x.(*sema.ArrayRef)
	if !ok {
		return LongConst(1)
	}

	return b.ifEmpty(d, b.n.lowerBound(r.Sym, d), 1)
}

// upper is dimension d's upper bound: a whole array's own, or 0 if its extent
// is zero, and the extent for any other array (13.13.111).
func (b *arrayBounds) upper(d int) Expr {
	r, ok := b.x.(*sema.ArrayRef)
	if !ok {
		return b.ext[d]
	}

	return b.ifEmpty(d, b.n.upperBound(r.Sym, d), 0)
}

// ifEmpty is v, or empty if dimension d's extent is zero.
func (b *arrayBounds) ifEmpty(d int, v Expr, empty int32) Expr {
	if c, ok := b.ext[d].(*Const); ok {
		if c.Int() <= 0 {
			return LongConst(empty)
		}

		return v
	}

	m := b.l.temp(Long)
	b.l.emit(&Assign{origin: b.l.origin(), Dst: m, Src: v})
	b.l.choose(&Cmp{Le, b.ext[d], LongConst(0)}, m, LongConst(empty))

	return &Load{m}
}

// boundValue computes SIZE, or LBOUND or UBOUND with DIM=, into a temporary.
// A DIM= not constant chooses among the dimensions when the program runs.
func (l *lowerer) boundValue(f *sema.ArrayFunc) *Mem {
	b := l.boundsOf(f.Array)
	defer b.n.done()

	value := func(d int) Expr {
		switch f.Name {
		case "LBOUND":
			return b.lower(d)
		case "UBOUND":
			return b.upper(d)
		}

		return b.extent(d)
	}

	m := l.temp(Long)

	switch {
	case f.Dim > 0:
		l.emit(&Assign{origin: l.origin(), Dst: m, Src: value(f.Dim - 1)})

	case f.DimExpr != nil:
		dim := l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: dim, Src: l.expr(f.DimExpr)})
		l.emit(&Assign{origin: l.origin(), Dst: m, Src: LongConst(0)})

		for d := range b.rank {
			l.choose(&Cmp{Eq, &Load{dim}, LongConst(int32(d + 1))}, m, value(d))
		}

	default:
		// SIZE of the whole array: the product of its extents.
		var size Expr = LongConst(1)
		for d := range b.rank {
			size = arith(Mul, Long, size, b.extent(d))
		}

		l.emit(&Assign{origin: l.origin(), Dst: m, Src: size})
	}

	return m
}
