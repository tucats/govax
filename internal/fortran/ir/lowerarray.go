package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/sema"
)

// The scalarizer (docs/planning/PHASE-102 - Fortran 90 array expressions.md,
// Decision 6 and finding 5): an array assignment becomes a nest of counted
// loops, one per dimension of its target, the leftmost dimension innermost so
// that memory is walked in array element order. Each loop's index runs from 1
// to its dimension's extent by 1, a normalized index; every array the
// statement reads or writes is asked for its element at the indexes k1..kn,
// and the assignment of one element to another is lowered as any scalar
// assignment is. So
//
//	A(2:N) = B(1:N-1) + 2.0 * C
//
// with A(10), B(10), and C(9) is
//
//	DO $I1 = 1, 9
//	  A($I1 + 1) = B($I1) + 2.0 * C($I1)
//	END DO
//
// its extent the constant 9, C's: an extent known when compiling is the one to
// use, and the target's, N - 1, isn't. A section's subscript for index k is l + (k - 1) * s, written
// k * s + (l - s), its parts computed once before the loops; a whole array's
// is k + (l - 1). Everything the statement computes that isn't an element (a
// subscript, a bound, a scalar operand that isn't a variable or a constant) is
// computed once, before the loops, into a temporary: Fortran 90's 7.5.1.5 asks
// that the right side be evaluated before any element is defined, and it is
// cheaper.
//
// What the loops compute stands in sema's terms, as expressions over symbols of
// lowering's own (sema.NewSymbol), each bound to its place here (lowerer.bound):
// an index to an IndexCell, a value computed once to a temporary. So an
// element of an adjustable array, a CHARACTER element, a COMPLEX value, or a
// comparison into a LOGICAL array is lowered by the code that lowers them in
// any statement, and the loops are counted loops the optimizer knows
// (DoLimits, invariant motion, induction, registers), as a DO's are.
//
// Under /CHECK=BOUNDS a section's first and last subscripts are checked once,
// before the loops, not at every element (finding 15), and two extents not
// known to be equal when compiling are compared: if they differ the shapes
// don't conform, FOR$_NOTCONFOR from FTNRTL.

// arrayNest is one array statement as the scalarizer lowers it.
type arrayNest struct {
	l    *lowerer
	name string

	// leaves are what the scalarizer knows of each array the statement reads
	// or writes, by the sema expression that is it, and prepared those
	// expressions in the order they were prepared: the target's first, then
	// the value's, left to right. (A map's order is Go's choice, and the code
	// must come out the same every time.)
	leaves   map[sema.Expr]*arrayLeaf
	prepared []sema.Expr

	// order is how the loops run (nil: every one up), and which scalars must
	// be computed once before them.
	order *sema.ArrayOrder

	// index is each loop's index, the leftmost dimension's first.
	index []*sema.Symbol

	// extents is each dimension's number of elements: a constant, or a
	// temporary computed before the loops.
	extents []sema.Expr

	// kept is lowering's symbols bound for the statement, unbound at its end.
	kept []*sema.Symbol
}

// arrayLeaf is one array the statement reads or writes, a whole array or a
// section, made ready for its elements: each dimension's subscript as a
// function of the index, and its extent.
type arrayLeaf struct {
	sym *sema.Symbol

	// dims is each of the array's dimensions, in order.
	dims []leafDim
}

// leafDim is one dimension of an array leaf. A dimension the leaf ranges over
// (a whole array's, or a section's triplet) has an index, which loop is its,
// and its subscript is index * stride + base; a section's subscript in a
// dimension it doesn't range over is fixed. lower, upper, and extent are what
// the checks need; extent is nil until it is wanted.
type leafDim struct {
	ranges        bool
	loop          int
	vector        sema.Expr
	stride, base  sema.Expr
	fixed         sema.Expr
	lower, upper  Expr
	extent        Expr
	extentPending bool
}

// arrayAssign lowers an array assignment, Target = Value (sema checked that
// they conform, and said how the loops must run, s.Order: which run down, or
// whether the value goes into a temporary first). With a temporary the
// statement is two nests, the value into the temporary, then the temporary
// into the target, each the same as an assignment of its own.
func (l *lowerer) arrayAssign(s *sema.Assign) {
	l.maskedAssign(s, nil)
}

// maskedAssign is arrayAssign where mask holds (a WHERE's assignment), or
// everywhere for a nil mask. With a temporary both nests are masked: the value
// is computed only where it is assigned.
func (l *lowerer) maskedAssign(s *sema.Assign, mask *whereMask) {
	if s.Order != nil && s.Order.Fill {
		l.fillConstructor(s.Target, s.Value.(*sema.Constructor))

		return
	}

	if s.Order != nil && s.Order.Temporary {
		temp := l.arrayTemp(s.Value.Type(), s.Order.TempShape)

		l.arrayNest(temp, s.Value, nil, mask)
		l.arrayNest(s.Target, temp, nil, mask)

		return
	}

	l.arrayNest(s.Target, s.Value, s.Order, mask)
}

// arrayTemp is an array temporary of type t and the extents shape, for the
// statement being lowered, as a whole array: a cell another statement used
// and gave back, or a new one ($Wn: a work array; $An is a dummy's address,
// lowerentry.go).
func (l *lowerer) arrayTemp(t sema.Type, shape []int) *sema.ArrayRef {
	sym := sema.NewArray("$W", t, shape)
	size := sym.Size()
	key := arrayTempKey{l.typeOf(t), size}

	var c *Cell

	if free := l.freeArray[key]; len(free) > 0 {
		c = free[len(free)-1]
		l.freeArray[key] = free[:len(free)-1]
	} else {
		l.arrays++
		c = l.newCell("$W"+strconv.Itoa(l.arrays), ArrayTempCell, key.t, size)
	}

	l.busy = append(l.busy, c)
	l.bound[sym] = &Mem{Cell: c, T: key.t}
	l.boundArrays = append(l.boundArrays, sym)

	return sema.NewArrayRef(sym, l.from.Position())
}

// arrayTempKey is what array temporaries are pooled by: their element type and
// size in bytes.
type arrayTempKey struct {
	t    Type
	size int
}

// whereMask is a WHERE's mask as an assignment's loops test it: the LOGICAL
// array, or its temporary, and whether the elements are assigned where it is
// false (ELSEWHERE's).
type whereMask struct {
	expr   sema.Expr
	negate bool
}

// arrayNest lowers target = value as one nest of loops, run as order says
// (nil: every loop up), assigning each element only where mask holds (nil:
// everywhere). The mask's arrays are leaves of the nest beside the value's.
func (l *lowerer) arrayNest(target, value sema.Expr, order *sema.ArrayOrder, mask *whereMask) {
	n := l.newNest(len(target.Shape()), order)
	rank := len(n.index)

	leaves := append(sema.Leaves(target), sema.Leaves(value)...)
	if mask != nil {
		leaves = append(leaves, sema.Leaves(mask.expr)...)
	}

	for _, leaf := range leaves {
		n.prepare(leaf)
	}

	n.extents = make([]sema.Expr, rank)
	for d := range rank {
		n.extents[d] = n.extent(d, target, value)
	}

	if l.unit.Checks.Bounds {
		n.checkShapes(leaves)
		n.checkSubscripts()
	}

	t := sema.Scalarize(target, n.element, n.hoist)
	v := sema.Scalarize(value, n.element, n.hoist)

	if mask == nil {
		n.loops(rank-1, func() { l.assignScalar(t, v) })
	} else {
		m := sema.Scalarize(mask.expr, n.element, n.hoist)

		n.loops(rank-1, func() { l.maskedBy(m, mask.negate, func() { l.assignScalar(t, v) }) })
	}

	n.done()
}

// newNest is a nest of rank loops, each with its index, run as order says.
func (l *lowerer) newNest(rank int, order *sema.ArrayOrder) *arrayNest {
	// A bound's inquiry (an automatic array's SIZE(A)) is lowered at an entry
	// point, before any statement.
	name := "array (at entry)"
	if l.from != nil {
		name = "array (line " + strconv.Itoa(l.from.Position().Line) + ")"
	}

	n := &arrayNest{l: l, name: name, leaves: map[sema.Expr]*arrayLeaf{}, order: order}

	for range rank {
		sym := sema.NewSymbol("$I", sema.IntegerType)
		l.bound[sym] = &Mem{Cell: l.indexCell(), T: Long}
		n.index = append(n.index, sym)
		n.kept = append(n.kept, sym)
	}

	return n
}

// done unbinds the nest's own symbols.
func (n *arrayNest) done() {
	for _, sym := range n.kept {
		delete(n.l.bound, sym)
	}
}

// eachElement lowers body once for each element of the array value, in array
// element order, given the element: a nest of loops of the value's own shape.
func (l *lowerer) eachElement(value sema.Expr, body func(element sema.Expr)) {
	n := l.newNest(len(value.Shape()), nil)

	for _, leaf := range sema.Leaves(value) {
		n.prepare(leaf)
	}

	n.extents = make([]sema.Expr, len(n.index))
	for d := range n.extents {
		n.extents[d] = n.extent(d, value, value)
	}

	if l.unit.Checks.Bounds {
		n.checkSubscripts()
	}

	v := sema.Scalarize(value, n.element, n.hoist)

	n.loops(len(n.index)-1, func() { body(v) })
	n.done()
}

// indexCell is an index cell for the statement being lowered: one another
// statement used and gave back, or a new one ($In).
func (l *lowerer) indexCell() *Cell {
	var c *Cell

	if k := len(l.freeIndex); k > 0 {
		c = l.freeIndex[k-1]
		l.freeIndex = l.freeIndex[:k-1]
	} else {
		l.indexes++
		c = l.newCell("$I"+strconv.Itoa(l.indexes), IndexCell, Long, Long.Size())
	}

	l.busy = append(l.busy, c)

	return c
}

// loops lowers the loops of dimensions d down to 0 around body, dimension d's
// outermost.
func (n *arrayNest) loops(d int, body func()) {
	if d < 0 {
		body()

		return
	}

	pos := n.l.from.Position()
	one := sema.IntConstAt(1, pos)
	name := n.name + ", dimension " + strconv.Itoa(d+1)
	var start, limit, step sema.Expr = one, n.extents[d], one

	if n.order != nil && n.order.Down[d] {
		start, limit, step = n.extents[d], one, sema.IntConstAt(-1, pos)
	}

	n.l.countedLoop(name, n.index[d], start, limit, step, false, func(*Block) {
		n.loops(d-1, body)
	})
}

// keepIR is the INTEGER value e, as the loops use it: a constant as it is,
// anything else computed now into a temporary.
func (n *arrayNest) keepIR(e Expr) sema.Expr {
	pos := n.l.from.Position()

	if c, ok := e.(*Const); ok {
		return sema.IntConstAt(int(c.Int()), pos)
	}

	// A value already in one of the statement's temporaries stays there.
	if ld, ok := e.(*Load); ok && ld.Mem.Cell != nil && ld.Mem.Cell.Kind == TempCell && ld.Mem.Index == nil {
		return n.bind(ld.Mem, sema.IntegerType)
	}

	m := n.l.temp(Long)
	n.l.emit(&Assign{origin: n.l.origin(), Dst: m, Src: e})

	return n.bind(m, sema.IntegerType)
}

// keep is the INTEGER sema value e, computed now unless it is a constant.
func (n *arrayNest) keep(e sema.Expr) sema.Expr {
	if _, ok := e.(*sema.Const); ok {
		return e
	}

	return n.keepIR(n.l.expr(e))
}

// bind is a symbol of type t whose place is m, for the statement.
func (n *arrayNest) bind(m *Mem, t sema.Type) sema.Expr {
	sym := sema.NewSymbol("$V", t)
	n.l.bound[sym] = m
	n.kept = append(n.kept, sym)

	return sema.NewVar(sym, n.l.from.Position())
}

// hoist is the scalar e as the loops use it: as it is if the loops compute it
// at each element (sema.Inline: a constant, a variable, ...) and it reads
// nothing the loops write (ArrayOrder.Hoisted), or else computed now into a
// temporary of its type.
func (n *arrayNest) hoist(e sema.Expr) sema.Expr {
	if sema.Inline(e) && !n.order.Hoisted(e) {
		return e
	}

	m := n.l.temp(n.l.typeOf(e.Type()))
	n.l.store(m, e)

	return n.bind(m, e.Type())
}

// prepare makes leaf ready for its elements: each dimension's stride and base,
// or its fixed subscript, computed now where they aren't constants.
func (n *arrayNest) prepare(leaf sema.Expr) { n.prepareIn(leaf, nil) }

// prepareIn is prepare for a leaf whose dimension k the nest's loop loops[k]
// ranges over (nil: loop k). A vector subscript's arrays are leaves too,
// ranging over the loop of the dimension the vector is in: in A(2, IV) with
// IV(5), IV's one dimension is the nest's only loop.
func (n *arrayNest) prepareIn(leaf sema.Expr, loops []int) {
	if n.leaves[leaf] != nil {
		return
	}

	loopOf := func(k int) int {
		if loops == nil {
			return k
		}

		return loops[k]
	}

	info := &arrayLeaf{}
	n.leaves[leaf] = info
	n.prepared = append(n.prepared, leaf)

	// whole makes info a whole array's: each dimension ranging from its
	// lower bound by 1.
	whole := func(sym *sema.Symbol, pos diag.Position) {
		info.sym = sym

		for i := range sym.Dims {
			lower := n.lowerBound(sym, i)
			info.dims = append(info.dims, leafDim{
				ranges: true, loop: loopOf(i),
				stride: sema.IntConstAt(1, pos),
				base:   n.keepIR(arith(Sub, Long, lower, LongConst(1))),
				lower:  lower,
			})
		}
	}

	switch leaf := leaf.(type) {
	case *sema.ArrayRef:
		whole(leaf.Sym, leaf.Pos)

	case *sema.ArrayConst:
		// An array constant is read from its datum, as a whole array of
		// lowering's own whose lower bounds are 1.
		sym := sema.NewArray("$K", leaf.T, constExtents(leaf))
		n.l.bound[sym] = n.l.arrayConstMem(leaf)
		n.kept = append(n.kept, sym)
		whole(sym, leaf.Pos)

	case *sema.Constructor:
		// A constructor is built now, before the loops, into an array
		// temporary, which they read (lowerctor.go).
		temp := n.l.arrayTemp(leaf.T, constExtents(leaf))
		n.l.fillConstructor(temp, leaf)
		whole(temp.Sym, leaf.Pos)

	case *sema.ArrayFunc:
		// An array function: its elements computed where they are used, or
		// the whole of it now, into a temporary (lowerarrayfunc.go).
		if leaf.Element() {
			n.prepareFunc(info, leaf, loopOf)

			break
		}

		temp := n.l.arrayTemp(leaf.T, constExtents(leaf))
		n.l.precompute(leaf, temp)
		whole(temp.Sym, leaf.Pos)

	case *sema.Section:
		info.sym = leaf.Sym
		loop := 0

		for i, sub := range leaf.Subs {
			if sub.Vector != nil {
				info.dims = append(info.dims, leafDim{ranges: true, loop: loopOf(loop), vector: sub.Vector})

				for _, inner := range sema.Leaves(sub.Vector) {
					n.prepareIn(inner, []int{loopOf(loop)})
				}

				loop++

				continue
			}

			if !sub.Triplet() {
				info.dims = append(info.dims, leafDim{fixed: n.keep(sub.Index)})

				continue
			}

			lower := n.keepIR(n.bound(sub.Lower, leaf.Sym, i, n.lowerBound))
			upper := n.keepIR(n.bound(sub.Upper, leaf.Sym, i, n.upperBound))
			stride := n.keep(sub.Stride)

			info.dims = append(info.dims, leafDim{
				ranges: true, loop: loopOf(loop),
				stride: stride,
				base:   n.keepIR(arith(Sub, Long, n.l.expr(lower), n.l.expr(stride))),
				lower:  n.l.expr(lower),
				upper:  n.l.expr(upper),
				// A triplet's extent is computed only if something needs it.
				extentPending: true,
			})
			loop++
		}
	}
}

// bound is a triplet's bound: e, or if it was left out (nil) the dimension's
// own, which of computes.
func (n *arrayNest) bound(e sema.Expr, s *sema.Symbol, i int, of func(*sema.Symbol, int) Expr) Expr {
	if e != nil {
		return n.l.expr(e)
	}

	return of(s, i)
}

// lowerBound is dimension i of s's lower bound: a constant, or an adjustable
// array's from its descriptor.
func (n *arrayNest) lowerBound(s *sema.Symbol, i int) Expr {
	if c := n.l.adjustableOf(s); c != nil {
		return n.l.lowerBound(s, c, i)
	}

	return LongConst(int32(s.Dims[i].Lower))
}

// upperBound is dimension i of s's upper bound, as lowerBound is.
func (n *arrayNest) upperBound(s *sema.Symbol, i int) Expr {
	if c := n.l.adjustableOf(s); c != nil && !s.Dims[i].UpperKnown() && !s.Dims[i].Assumed {
		return &Load{&Mem{Cell: c, Offset: c.Desc.LowerOffset(i) + Long.Size(), T: Long}}
	}

	return LongConst(int32(s.Dims[i].Upper))
}

// leafExtent is the extent of the leaf's dimension that loop loop is: a
// constant, or computed now (once) into a temporary, never below 0.
func (n *arrayNest) leafExtent(leaf sema.Expr, loop int) Expr {
	info := n.leaves[leaf]

	for i := range info.dims {
		d := &info.dims[i]
		if !d.ranges || d.loop != loop {
			continue
		}

		if d.extent != nil {
			return d.extent
		}

		var count Expr

		switch leaf := leaf.(type) {
		case *sema.ArrayConst, *sema.Constructor:
			count = LongConst(int32(leaf.Shape()[i].Size))

		case *sema.ArrayFunc:
			if x := leaf.S[i]; x.Known {
				count = LongConst(int32(x.Size))
			} else {
				count = n.leafExtent(x.Of, n.loopOf(x.Of, x.Dim))
			}

		case *sema.Section:
			if d.vector != nil {
				// A vector subscript's extent is the vector's: known, or one
				// of its own arrays'.
				if e := d.vector.Shape()[0]; e.Known {
					d.extent = LongConst(int32(e.Size))
				} else {
					d.extent = n.leafExtent(e.Of, n.loopOf(e.Of, e.Dim))
				}

				return d.extent
			}

			count = tripCount(Long, d.lower, d.upper, n.l.expr(d.stride))

		case *sema.ArrayRef:
			if c := n.l.adjustableOf(leaf.Sym); c != nil {
				count = n.l.extent(leaf.Sym, c, i)
			} else {
				count = LongConst(int32(leaf.Sym.Dims[i].Extent()))
			}

		default:
			count = tripCount(Long, d.lower, d.upper, n.l.expr(d.stride))
		}

		d.extent = n.atLeastZero(count)

		return d.extent
	}

	panic("ir: an array leaf with no such dimension")
}

// atLeastZero is e, a Long, or 0 if it is less: a constant as it is, anything
// else computed now into a temporary (Fortran 90's Table 6.1: a section's
// extent is never negative).
func (n *arrayNest) atLeastZero(e Expr) Expr {
	if c, ok := e.(*Const); ok {
		return LongConst(max(c.Int(), 0))
	}

	m := n.l.temp(Long)
	n.l.emit(&Assign{origin: n.l.origin(), Dst: m, Src: e})

	negative, next := n.l.newBlock(), n.l.newBlock()
	n.l.branch(&Cmp{Lt, &Load{m}, LongConst(0)}, negative, next)
	n.l.begin(negative)
	n.l.emit(&Assign{origin: n.l.origin(), Dst: m, Src: LongConst(0)})
	n.l.jump(next)
	n.l.begin(next)

	return &Load{m}
}

// extent is dimension d's extent for the loops: the shape's if it is known when
// compiling (the target's, or the value's), or the target's computed now.
func (n *arrayNest) extent(d int, target, value sema.Expr) sema.Expr {
	pos := n.l.from.Position()

	for _, s := range []sema.Shape{target.Shape(), value.Shape()} {
		if d < len(s) && s[d].Known {
			return sema.IntConstAt(s[d].Size, pos)
		}
	}

	e := target.Shape()[d]

	return n.keepIR(n.leafExtent(e.Of, n.loopOf(e.Of, e.Dim)))
}

// loopOf is which loop the leaf's dimension dim (a whole array's dimension, a
// section's subscript) is.
func (n *arrayNest) loopOf(leaf sema.Expr, dim int) int {
	return n.leaves[leaf].dims[dim].loop
}

// element is leaf's element at the loops' indexes. A vector subscript is the
// vector's element there, itself an array expression's (A(IV + 1)); each of
// its values is checked by /CHECK=BOUNDS where it is used, the others once,
// before the loops.
func (n *arrayNest) element(leaf sema.Expr) sema.Expr {
	if f, ok := leaf.(*sema.ArrayFunc); ok && f.Element() {
		return n.funcElement(f)
	}

	info := n.leaves[leaf]
	subs := make([]sema.Expr, len(info.dims))
	inBounds := make([]bool, len(info.dims))

	for i, d := range info.dims {
		if d.vector != nil {
			subs[i] = sema.Scalarize(d.vector, n.element, n.hoist)

			continue
		}

		inBounds[i] = true

		if !d.ranges {
			subs[i] = d.fixed

			continue
		}

		k := sema.NewVar(n.index[d.loop], leaf.Position())
		subs[i] = sema.IntSum(sema.IntProduct(k, d.stride), d.base)
	}

	return sema.NewElem(info.sym, subs, inBounds, leaf.Position())
}

// checkShapes compares, under /CHECK=BOUNDS, every leaf's extent in each
// dimension with the loops' where the two aren't known to be equal when
// compiling: if any differs, the shapes don't conform (FTN$NOTCONFOR, which
// doesn't return).
func (n *arrayNest) checkShapes(leaves []sema.Expr) {
	for _, leaf := range leaves {
		for loop := range n.extents {
			if loop >= len(leaf.Shape()) {
				continue
			}

			want := n.l.expr(n.extents[loop])

			if c, ok := want.(*Const); ok && leaf.Shape()[loop].Known && int32(leaf.Shape()[loop].Size) == c.Int() {
				continue
			}

			got := n.leafExtent(leaf, loop)
			if ExprText(got) == ExprText(want) {
				continue
			}

			bad, next := n.l.newBlock(), n.l.newBlock()
			n.l.branch(&Cmp{Ne, got, want}, bad, next)

			n.l.begin(bad)
			n.l.emit(&Call{origin: n.l.origin(), Routine: ftnNotConfor})
			n.l.terminate(&NoReturn{n.l.origin()})

			n.l.begin(next)
		}
	}
}

// checkSubscripts checks, under /CHECK=BOUNDS, the subscripts each section
// uses, once: in a dimension it ranges over the first and the last, if it has
// any elements (Fortran 90's 6.2.2.3.1: only the subscripts used need be in
// bounds), and its fixed subscripts. A check of a constant in range is left
// out (inRange).
func (n *arrayNest) checkSubscripts() {
	for _, leaf := range n.prepared {
		info := n.leaves[leaf]
		if _, ok := leaf.(*sema.Section); !ok {
			continue
		}

		for i, d := range info.dims {
			low, high := n.lowerBound(info.sym, i), n.checkedUpper(info.sym, i)

			switch {
			case d.vector != nil:
				continue
			case !d.ranges:
				n.check(inRange(n.l.expr(d.fixed), low, high))

				continue
			}

			extent := n.leafExtent(leaf, d.loop)
			stride := n.l.expr(d.stride)
			last := arith(Add, Long, d.lower, arith(Mul, Long, arith(Sub, Long, extent, LongConst(1)), stride))

			first, final := inRange(d.lower, low, high), inRange(last, low, high)

			_, firstChecked := first.(*InRange)
			_, finalChecked := final.(*InRange)

			if !firstChecked && !finalChecked {
				continue
			}

			c, known := extent.(*Const)
			if known && c.Int() <= 0 {
				continue
			}

			next := n.l.newBlock()

			if !known {
				some := n.l.newBlock()
				n.l.branch(&Cmp{Gt, extent, LongConst(0)}, some, next)
				n.l.begin(some)
			}

			n.check(first)
			n.check(final)
			n.l.jump(next)
			n.l.begin(next)
		}
	}
}

// checkedUpper is dimension i of s's upper bound as a check needs it: none (the
// most a Long can be) for an open-ended dimension.
func (n *arrayNest) checkedUpper(s *sema.Symbol, i int) Expr {
	if c := n.l.adjustableOf(s); c != nil {
		return n.l.upperBound(s, c, i)
	}

	return upperOf(sema.OpenEnded(s, s.Dims, i), int64(s.Dims[i].Upper))
}

// check computes e, a subscript's check, for its trap alone, if it is one.
func (n *arrayNest) check(e Expr) {
	if _, ok := e.(*InRange); !ok {
		return
	}

	n.l.emit(&Assign{origin: n.l.origin(), Dst: n.l.temp(Long), Src: e})
}
