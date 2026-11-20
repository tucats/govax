package ir

import (
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/sema"
)

// The transformational functions (sema/transform.go; docs/planning/PHASE-104 -
// Fortran 90 dynamic arrays.md, finding 13, subtask 8): SPREAD, CSHIFT,
// EOSHIFT, PACK, UNPACK, RESHAPE of values not constant, and a reduction with a
// DIM= not constant on an array of rank above 1. Each is computed whole into a
// temporary of its shape before the statement's loops, once per statement
// (transformed), its extents first (transformExtents) and then its elements,
// by loops of its own that walk its argument in array element order and put
// each element where it goes:
//
//	B = CSHIFT(A, 2)                    ! A(5)
//
//	DO $I1 = 1, 5                       ! over A
//	  $T = MOD($I1 - 1 - 2, 5)          ! where A($I1) goes, from 0
//	  IF ($T < 0) $T = $T + 5
//	  $W1($T + 1) = A($I1)
//	END DO
//	B = $W1                             ! the statement's own loops
//
// The finding had SPREAD, CSHIFT, and EOSHIFT computed where they are used, as
// TRANSPOSE is; but an element there is the argument's at a subscript computed
// from the loops' (a MOD for CSHIFT, outside the array for EOSHIFT's boundary),
// which the scalarizer's leaves don't take, and the argument may be any array
// expression. One way for all of them, safe whatever the statement's target
// overlaps, costs a copy. Phase 110 weighed the copies on kernels and left them (rare
// in the corpus); making the simple ones elements again waits.

// transformed is the value of f, a function whose value is Transformed,
// computed now into a temporary of its shape, unless the statement has already.
func (l *lowerer) transformed(f *sema.ArrayFunc) *sema.ArrayRef {
	if temp := l.runtime.funcs[f]; temp != nil {
		return temp
	}

	if f.Name == "TRANSFER" {
		// Its bytes, not its elements, are moved (lowertransfer.go).
		return l.keepTransformed(f, l.transferArray(f))
	}

	extents, dim := l.transformExtents(f)

	// A CHARACTER value of a length known only now is as long as its argument's
	// elements (sema's transform keeps such an argument to names).
	var length Expr
	if f.T.Kind == sema.Character && f.T.Size == 0 {
		length = l.valueLength(f.Array)
	}

	temp := l.extentsTempOf(f.T, extents, length)

	switch {
	case f.DimExpr != nil:
		l.reduceByDim(f, temp, dim)
	case f.Name == "SPREAD":
		l.spreadInto(f, temp, extents)
	case f.Name == "CSHIFT" || f.Name == "EOSHIFT":
		l.shiftInto(f, temp, extents)
	case f.Name == "PACK":
		l.packInto(f, temp)
	case f.Name == "UNPACK":
		l.unpackInto(f, temp, extents)
	case f.Name == "RESHAPE":
		l.reshapeInto(f, temp, extents)
	}

	return l.keepTransformed(f, temp)
}

// keepTransformed records temp as the statement's value of f, and returns it.
func (l *lowerer) keepTransformed(f *sema.ArrayFunc, temp *sema.ArrayRef) *sema.ArrayRef {
	if l.runtime.funcs == nil {
		l.runtime.funcs = map[*sema.ArrayFunc]*sema.ArrayRef{}
	}

	l.runtime.funcs[f] = temp

	return temp
}

// transformExtents is each extent of f's value, a Long (a constant, or a
// temporary's value), never below 0; and for a reduction with a DIM= not
// constant, the temporary DIM's value is in.
func (l *lowerer) transformExtents(f *sema.ArrayFunc) ([]Expr, *Mem) {
	switch {
	case f.DimExpr != nil:
		return l.dimExtents(f)

	case f.Name == "SPREAD":
		list := l.extentsOf(f.Array.Shape())
		copies := l.keepLong(l.expr(f.Other))
		l.notNegative(copies)

		return append(append(list[:f.Dim-1:f.Dim-1], &Load{copies}), list[f.Dim-1:]...), nil

	case f.Name == "PACK":
		if f.Other != nil {
			return l.extentsOf(f.Other.Shape()), nil
		}

		if len(f.Mask.Shape()) == 0 {
			// A scalar MASK: every element, or none.
			m := l.keepLong(LongConst(0))
			l.choose(l.cond(f.Mask), m, product(l.extentsOf(f.Array.Shape())))

			return []Expr{&Load{m}}, nil
		}

		return []Expr{&Load{l.reduce(sema.NewCount(f.Mask), nil)}}, nil

	case f.Name == "UNPACK":
		return l.extentsOf(f.Mask.Shape()), nil

	case f.Name == "RESHAPE":
		return l.reshapeExtents(f), nil
	}

	// CSHIFT and EOSHIFT.
	return l.extentsOf(f.Array.Shape()), nil
}

// keepLong is a Long temporary holding x, computed now.
func (l *lowerer) keepLong(x Expr) *Mem {
	m := l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: m, Src: x})

	return m
}

// product is the product of the Longs list, 1 for none.
func product(list []Expr) Expr {
	var p Expr = LongConst(1)
	for _, x := range list {
		p = arith(Mul, Long, p, x)
	}

	return p
}

// extentsTemp is an array temporary of type t and the extents given: a cell if
// every one is a constant (arrayTemp), else a run-time temporary.
func (l *lowerer) extentsTemp(t sema.Type, extents []Expr) *sema.ArrayRef {
	return l.extentsTempOf(t, extents, nil)
}

// extentsTempOf is extentsTemp with length, a CHARACTER element's length when
// t's isn't known when compiling (t.Size 0), which makes it a run-time
// temporary whatever its extents.
func (l *lowerer) extentsTempOf(t sema.Type, extents []Expr, length Expr) *sema.ArrayRef {
	sizes := make([]int, len(extents))

	for i, x := range extents {
		c, ok := x.(*Const)
		if !ok || length != nil {
			return l.runtimeTemp(t, extents, length)
		}

		sizes[i] = int(c.Int())
	}

	return l.arrayTemp(t, sizes)
}

// symExtent is dimension i's extent of the array s, a temporary or any whole
// array: its descriptor's, or a constant.
func (l *lowerer) symExtent(s *sema.Symbol, i int) Expr {
	if c := l.adjustableOf(s); c != nil {
		return l.extent(s, c, i)
	}

	return LongConst(int32(max(s.Dims[i].Extent(), 0)))
}

// walk is a nest of len(extents) loops over the extents given, x's leaves (if x
// is an array) prepared to range over the loops mapping says (nil: dimension k
// over loop k), and x's element at the loops' indexes; done ends the nest.
type walk struct {
	n *arrayNest
}

// newWalk makes a nest over the extents.
func (l *lowerer) newWalk(extents []Expr) *walk {
	n := l.newNest(len(extents), nil)
	n.extents = make([]sema.Expr, len(extents))

	for d, x := range extents {
		n.extents[d] = n.keepIR(x)
	}

	return &walk{n}
}

// of is x's element at the walk's indexes, its dimension k ranging over the
// walk's loop mapping[k] (nil: loop k); a scalar is computed once, now.
func (w *walk) of(x sema.Expr, mapping []int) sema.Expr {
	if x == nil {
		return nil
	}

	if len(x.Shape()) > 0 {
		for _, leaf := range sema.Leaves(x) {
			w.n.prepareIn(leaf, mapping)
		}
	}

	return sema.Scalarize(x, w.n.element, w.n.hoist)
}

// index is the walk's loop d's index, from 1.
func (w *walk) index(d int) sema.Expr {
	return sema.NewVar(w.n.index[d], w.n.l.here())
}

// indexes are every loop's index, loop 0's first.
func (w *walk) indexes() []sema.Expr {
	list := make([]sema.Expr, len(w.n.index))
	for d := range list {
		list[d] = w.index(d)
	}

	return list
}

// value is a sema variable standing for m, a Long, for subscripts.
func (w *walk) value(m *Mem) sema.Expr {
	return w.n.bind(m, sema.IntegerType)
}

// run lowers body inside the walk's loops, then ends the walk.
func (w *walk) run(body func()) {
	w.n.loops(len(w.n.index)-1, body)
	w.n.done()
}

// put assigns v to temp's element at subs.
func (l *lowerer) put(temp *sema.ArrayRef, subs []sema.Expr, v sema.Expr) {
	l.assignScalar(sema.NewElem(temp.Sym, subs, make([]bool, len(subs)), temp.Pos), v)
}

// lineMap maps the dimensions of a line argument (SHIFT, BOUNDARY: an array of
// the rank below ARRAY's) to the loops of a walk over ARRAY, whose dimension
// dim (from 0) the line doesn't have.
func lineMap(rank, dim int) []int {
	var m []int

	for d := range rank {
		if d != dim {
			m = append(m, d)
		}
	}

	return m
}

// spreadInto fills temp with SPREAD(SOURCE, DIM, NCOPIES): at each place,
// SOURCE's element at the subscripts but DIM's.
func (l *lowerer) spreadInto(f *sema.ArrayFunc, temp *sema.ArrayRef, extents []Expr) {
	w := l.newWalk(extents)
	v := w.of(f.Array, lineMap(len(extents), f.Dim-1))

	w.run(func() { l.put(temp, w.indexes(), v) })
}

// shiftInto fills temp with CSHIFT or EOSHIFT of ARRAY: EOSHIFT's BOUNDARY
// first everywhere, then each element of ARRAY where SHIFT moves it, j - SHIFT
// along DIM, for CSHIFT around the end (MOD), for EOSHIFT only if that is in
// the array.
func (l *lowerer) shiftInto(f *sema.ArrayFunc, temp *sema.ArrayRef, extents []Expr) {
	rank, dim := len(extents), f.Dim-1
	line := lineMap(rank, dim)
	n := extents[dim]

	if f.Name == "EOSHIFT" {
		w := l.newWalk(extents)
		b := w.of(f.Pad, line)

		if b == nil {
			b = defaultBoundary(f.T, f.Pos)
		}

		w.run(func() { l.put(temp, w.indexes(), b) })
	}

	w := l.newWalk(extents)
	v := w.of(f.Array, nil)
	s := w.of(f.Other, line)
	to := l.temp(Long)
	at := w.value(to)

	w.run(func() {
		j := &Load{l.symbolMem(w.n.index[dim])}
		l.emit(&Assign{origin: l.origin(), Dst: to, Src: arith(Sub, Long, j, l.expr(s))})

		subs := w.indexes()
		subs[dim] = at

		if f.Name == "CSHIFT" {
			// (j - 1 - s) mod n, made 0 to n - 1, then from 1.
			l.emit(&Assign{origin: l.origin(), Dst: to, Src: arith(Sub, Long, &Load{to}, LongConst(1))})
			l.emit(&Assign{origin: l.origin(), Dst: to, Src: arith(Sub, Long, &Load{to},
				arith(Mul, Long, arith(Div, Long, &Load{to}, n), n))})
			l.choose(&Cmp{Lt, &Load{to}, LongConst(0)}, to, arith(Add, Long, &Load{to}, n))
			l.emit(&Assign{origin: l.origin(), Dst: to, Src: arith(Add, Long, &Load{to}, LongConst(1))})
			l.put(temp, subs, v)

			return
		}

		inside, check, next := l.newBlock(), l.newBlock(), l.newBlock()
		l.branch(&Cmp{Ge, &Load{to}, LongConst(1)}, check, next)
		l.begin(check)
		l.branch(&Cmp{Le, &Load{to}, n}, inside, next)
		l.begin(inside)
		l.put(temp, subs, v)
		l.jump(next)
		l.begin(next)
	})
}

// defaultBoundary is EOSHIFT's BOUNDARY when none is given (13.13.32): 0 of a
// number's type, .FALSE., or blanks.
func defaultBoundary(t sema.Type, pos diag.Position) sema.Expr {
	switch t.Kind {
	case sema.Logical:
		return sema.LogicalConstAt(false, pos)
	case sema.Character:
		return sema.BlankConst(pos)
	}

	return sema.ZeroConst(t, pos)
}

// packInto fills temp with PACK(ARRAY, MASK, VECTOR): VECTOR first, if given,
// then ARRAY's elements where MASK holds, in array element order, from the first.
func (l *lowerer) packInto(f *sema.ArrayFunc, temp *sema.ArrayRef) {
	if f.Other != nil {
		l.arrayNest(temp, f.Other, nil, nil)
	}

	k := l.keepLong(LongConst(0))

	w := l.newWalk(l.extentsOf(f.Array.Shape()))
	v := w.of(f.Array, nil)
	m := w.of(f.Mask, nil)
	at := w.value(k)

	w.run(func() {
		l.masked(m, func() {
			l.emit(&Assign{origin: l.origin(), Dst: k, Src: arith(Add, Long, &Load{k}, LongConst(1))})
			l.put(temp, []sema.Expr{at}, v)
		})
	})
}

// unpackInto fills temp with UNPACK(VECTOR, MASK, FIELD): where MASK holds,
// VECTOR's next element, from the first; elsewhere FIELD's. VECTOR is copied
// first, its elements then being taken by number.
func (l *lowerer) unpackInto(f *sema.ArrayFunc, temp *sema.ArrayRef, extents []Expr) {
	vector := l.extentsTemp(f.Array.Type(), l.extentsOf(f.Array.Shape()))
	l.arrayNest(vector, f.Array, nil, nil)

	k := l.keepLong(LongConst(0))

	w := l.newWalk(extents)
	m := w.of(f.Mask, nil)
	field := w.of(f.Other, nil)
	at := w.value(k)

	w.run(func() {
		yes, no, next := l.newBlock(), l.newBlock(), l.newBlock()
		l.branch(l.cond(m), yes, no)

		l.begin(yes)
		l.emit(&Assign{origin: l.origin(), Dst: k, Src: arith(Add, Long, &Load{k}, LongConst(1))})
		l.put(temp, w.indexes(), sema.NewElem(vector.Sym, []sema.Expr{at}, []bool{false}, f.Pos))
		l.jump(next)

		l.begin(no)
		l.put(temp, w.indexes(), field)
		l.jump(next)

		l.begin(next)
	})
}

// flatten is the array value x copied into a temporary of rank one, in array
// element order, and the number of its elements.
func (l *lowerer) flatten(x sema.Expr) (*sema.ArrayRef, Expr) {
	count := l.keepLong(product(l.extentsOf(x.Shape())))
	temp := l.extentsTemp(x.Type(), []Expr{&Load{count}})

	k := l.keepLong(LongConst(0))
	sym := sema.NewSymbol("$V", sema.IntegerType)
	l.bound[sym] = k

	defer delete(l.bound, sym)

	at := sema.NewVar(sym, x.Position())

	l.eachElement(x, func(e sema.Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: k, Src: arith(Add, Long, &Load{k}, LongConst(1))})
		l.put(temp, []sema.Expr{at}, e)
	})

	return temp, &Load{count}
}

// reshapeExtents is RESHAPE's SHAPE's values: constants, or computed now, each
// never below 0.
func (l *lowerer) reshapeExtents(f *sema.ArrayFunc) []Expr {
	list := make([]Expr, len(f.S))

	if known := allKnownShape(f.S); known {
		for i, e := range f.S {
			list[i] = LongConst(int32(e.Size))
		}

		return list
	}

	shape := l.arrayTemp(sema.IntegerType, []int{len(f.S)})
	l.arrayNest(shape, f.Other, nil, nil)

	for i := range list {
		at := sema.NewElem(shape.Sym, []sema.Expr{sema.IntConstAt(i+1, f.Pos)}, []bool{false}, f.Pos)
		m := l.keepLong(l.expr(at))
		l.notNegative(m)
		list[i] = &Load{m}
	}

	return list
}

// allKnownShape reports whether every extent of the shape is known when
// compiling.
func allKnownShape(s sema.Shape) bool {
	for _, e := range s {
		if !e.Known {
			return false
		}
	}

	return true
}

// reshapeInto fills temp with RESHAPE(SOURCE, SHAPE, PAD, ORDER): SOURCE's
// elements in array element order, then PAD's over and over, placed with
// ORDER's first dimension varying fastest. SOURCE and PAD are copied first, their
// elements then taken by number. With too few and no PAD (or an empty one) the
// rest is left as it was: the standard calls that program wrong.
func (l *lowerer) reshapeInto(f *sema.ArrayFunc, temp *sema.ArrayRef, extents []Expr) {
	source, ns := l.flatten(f.Array)

	var pad *sema.ArrayRef

	var np Expr

	if f.Pad != nil {
		pad, np = l.flatten(f.Pad)
	}

	// The walk's loop L is the result's dimension Order[L], loop 0 innermost.
	ordered := make([]Expr, len(extents))
	for L, d := range f.Order {
		ordered[L] = extents[d]
	}

	k := l.keepLong(LongConst(0))
	j := l.temp(Long)

	w := l.newWalk(ordered)
	at, padAt := w.value(k), w.value(j)

	w.run(func() {
		subs := make([]sema.Expr, len(extents))
		for L, d := range f.Order {
			subs[d] = w.index(L)
		}

		l.emit(&Assign{origin: l.origin(), Dst: k, Src: arith(Add, Long, &Load{k}, LongConst(1))})

		fromSource, fromPad, next := l.newBlock(), l.newBlock(), l.newBlock()
		l.branch(&Cmp{Le, &Load{k}, ns}, fromSource, fromPad)

		l.begin(fromSource)
		l.put(temp, subs, sema.NewElem(source.Sym, []sema.Expr{at}, []bool{false}, f.Pos))
		l.jump(next)

		l.begin(fromPad)

		if pad != nil {
			// PAD's element (k - ns - 1) mod np, from 1, if PAD has any.
			some := l.newBlock()
			l.branch(&Cmp{Gt, np, LongConst(0)}, some, next)
			l.begin(some)
			l.emit(&Assign{origin: l.origin(), Dst: j, Src: arith(Sub, Long, arith(Sub, Long, &Load{k}, ns), LongConst(1))})
			l.emit(&Assign{origin: l.origin(), Dst: j, Src: arith(Sub, Long, &Load{j},
				arith(Mul, Long, arith(Div, Long, &Load{j}, np), np))})
			l.emit(&Assign{origin: l.origin(), Dst: j, Src: arith(Add, Long, &Load{j}, LongConst(1))})
			l.put(temp, subs, sema.NewElem(pad.Sym, []sema.Expr{padAt}, []bool{false}, f.Pos))
		}

		l.jump(next)
		l.begin(next)
	})
}

// dimExtents is the extents of a reduction with a DIM= not constant: ARRAY's but
// the one DIM's value names, chosen when the program runs, all 0 for a DIM out
// of range (under /CHECK=BOUNDS, FOR's NOTCONFOR); and DIM's temporary.
func (l *lowerer) dimExtents(f *sema.ArrayFunc) ([]Expr, *Mem) {
	dim := l.keepLong(l.expr(f.DimExpr))
	all := l.extentsOf(f.Array.Shape())
	rank := len(all)

	if l.unit.Checks.Bounds {
		l.notConforming(&Cmp{Lt, &Load{dim}, LongConst(1)})
		l.notConforming(&Cmp{Gt, &Load{dim}, LongConst(int32(rank))})
	}

	list := make([]Expr, rank-1)

	for j := range list {
		m := l.keepLong(LongConst(0))

		for d := 1; d <= rank; d++ {
			src := all[j]
			if j >= d-1 {
				src = all[j+1]
			}

			l.choose(&Cmp{Eq, &Load{dim}, LongConst(int32(d))}, m, src)
		}

		list[j] = &Load{m}
	}

	return list, dim
}

// reduceByDim fills temp with the reduction f for DIM's value: for each value
// from 1 to the rank, the reduction with that constant DIM, its elements
// computed where they are used (lowerarrayfunc.go).
func (l *lowerer) reduceByDim(f *sema.ArrayFunc, temp *sema.ArrayRef, dim *Mem) {
	next := l.newBlock()

	for d := 1; d <= len(f.Array.Shape()); d++ {
		this, other := l.newBlock(), l.newBlock()
		l.branch(&Cmp{Eq, &Load{dim}, LongConst(int32(d))}, this, other)

		l.begin(this)
		l.arrayNest(temp, sema.WithDim(f, d), nil, nil)
		l.jump(next)

		l.begin(other)
	}

	l.jump(next)
	l.begin(next)
}
