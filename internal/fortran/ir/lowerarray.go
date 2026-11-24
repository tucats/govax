package ir

import (
	"slices"
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

	// field is the field whose dimensions a component reference's ranked
	// part has (PATH(1)%XS(2:5)), nil for an array's or a record array's own.
	field *sema.Field

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
	target := s.Target
	if s.Order != nil && s.Order.CopyVectors {
		target = l.copyVectors(target)
	}

	if s.Order != nil && s.Order.Fill {
		l.fillConstructor(target, s.Value.(*sema.Constructor))

		return
	}

	if s.Order != nil && s.Order.Temporary {
		t, length := l.tempType(target, s.Value)
		temp := l.shapeTemp(t, s.Order.TempShape, l.knownShape(target.Shape(), s.Value.Shape()), length)

		l.arrayNest(temp, s.Value, nil, mask)
		l.arrayNest(target, temp, nil, mask)

		return
	}

	l.arrayNest(target, s.Value, s.Order, mask)
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

	return sema.NewArrayRef(sym, l.here())
}

// arrayTempKey is what array temporaries are pooled by: their element type and
// size in bytes.
type arrayTempKey struct {
	t    Type
	size int
}

// whereMask is a WHERE's mask as an assignment's loops test it: the LOGICAL
// array, or its temporary, and whether the elements are assigned where it is
// false (ELSEWHERE's). In a Fortran 95 construct (lowerwhere.go's
// generalWhere) a statement's control mask is a chain of them, outer the
// mask tested first: an element is assigned only where every one holds.
type whereMask struct {
	expr   sema.Expr
	negate bool
	outer  *whereMask
}

// terms are the masks of the chain m, outer first.
func (m *whereMask) terms() []*whereMask {
	if m == nil {
		return nil
	}

	return append(m.outer.terms(), m)
}

// leaves are the array leaves of every mask of the chain m, for a nest to
// prepare.
func (m *whereMask) leaves() []sema.Expr {
	var list []sema.Expr

	for _, t := range m.terms() {
		list = append(list, sema.Leaves(t.expr)...)
	}

	return list
}

// scalarMask is a mask of a chain as one element of a nest tests it.
type scalarMask struct {
	expr   sema.Expr
	negate bool
}

// scalarMasks are the chain m's masks as the nest n's elements, outer first,
// made before n's loops so that what they hoist goes before them.
func (n *arrayNest) scalarMasks(m *whereMask) []scalarMask {
	var list []scalarMask

	for _, t := range m.terms() {
		list = append(list, scalarMask{sema.Scalarize(t.expr, n.element, n.hoist), t.negate})
	}

	return list
}

// maskedByAll lowers body where every one of masks holds, each tested only
// where those before it hold.
func (l *lowerer) maskedByAll(masks []scalarMask, body func()) {
	if len(masks) == 0 {
		body()

		return
	}

	l.maskedBy(masks[0].expr, masks[0].negate, func() { l.maskedByAll(masks[1:], body) })
}

// arrayNest lowers target = value as one nest of loops, run as order says
// (nil: every loop up), assigning each element only where mask holds (nil:
// everywhere). The mask's arrays are leaves of the nest beside the value's.
func (l *lowerer) arrayNest(target, value sema.Expr, order *sema.ArrayOrder, mask *whereMask) {
	n := l.newNest(len(target.Shape()), order)
	rank := len(n.index)

	leaves := append(sema.Leaves(target), sema.Leaves(value)...)
	leaves = append(leaves, mask.leaves()...)

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

	m := n.scalarMasks(mask)

	n.loops(rank-1, func() { l.maskedByAll(m, func() { l.assignScalar(t, v) }) })

	n.done()
}

// newNest is a nest of rank loops, each with its index, run as order says.
func (l *lowerer) newNest(rank int, order *sema.ArrayOrder) *arrayNest {
	// A bound's inquiry (an automatic array's SIZE(A)) is lowered at an entry
	// point, before any statement.
	name := "array (at entry)"
	if l.from != nil {
		name = "array (line " + strconv.Itoa(l.here().Line) + ")"
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
		delete(n.l.lengths, sym)
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

// elementCount is the number of elements of the array value, its extents'
// product, computed when the statement reaches it (a section's from its
// triplets, an allocatable array's from its descriptor), with no loop.
func (l *lowerer) elementCount(value sema.Expr) Expr {
	n := l.newNest(len(value.Shape()), nil)
	defer n.done()

	for _, leaf := range sema.Leaves(value) {
		n.prepare(leaf)
	}

	var count Expr = LongConst(1)
	for d := range value.Shape() {
		count = arith(Mul, Long, count, l.expr(n.extent(d, value, value)))
	}

	return count
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

	pos := n.l.here()
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
	pos := n.l.here()

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

	return sema.NewVar(sym, n.l.here())
}

// hoist is the scalar e as the loops use it: as it is if the loops compute it
// at each element (sema.Inline: a constant, a variable, ...) and it reads
// nothing the loops write (ArrayOrder.Hoisted), or else computed now into a
// temporary of its type.
func (n *arrayNest) hoist(e sema.Expr) sema.Expr {
	if sema.Inline(e) && !n.order.Hoisted(e) {
		return e
	}

	if e.Type().Kind == sema.Character {
		return n.hoistText(e)
	}

	if e.Type().Kind == sema.Record {
		// A record, a constructor's value, or a function's of a derived type,
		// copied once into a temporary of its own (the value of A = P, with
		// P one of A's elements, is P before any element changes).
		m := n.l.textTemp(e.Type().Size)

		if v, ok := e.(*sema.StructValue); ok {
			n.l.buildRecord(m, v)
		} else {
			n.l.moveBytes(m, n.l.mem(e), e.Type().Size)
		}

		return n.bind(m, e.Type())
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
		// temporary, which they read (lowerctor.go): a cell if its extent and
		// its elements' length are known when compiling, else a run-time
		// temporary of its extent, or of the count of its items, made by
		// running through them first (countItems).
		var temp *sema.ArrayRef

		switch {
		case !leaf.S[0].Known:
			temp = n.l.runtimeTemp(leaf.T, []Expr{n.l.countItems(leaf.Items)}, n.l.ctorLength(leaf))
		case leaf.T.Kind == sema.Character && leaf.T.Size == 0:
			temp = n.l.runtimeTemp(leaf.T, []Expr{LongConst(int32(leaf.S[0].Size))}, n.l.ctorLength(leaf))
		default:
			temp = n.l.arrayTemp(leaf.T, constExtents(leaf))
		}

		n.l.fillConstructor(temp, leaf)
		whole(temp.Sym, leaf.Pos)

	case *sema.ArrayFunc:
		// An array function: its elements computed where they are used, or
		// the whole of it now, into a temporary (lowerarrayfunc.go).
		if leaf.Element() {
			n.prepareFunc(info, leaf, loopOf)

			break
		}

		// A transformational function's whole value, computed now
		// (lowertransform.go).
		if leaf.Transformed() {
			whole(n.l.transformed(leaf).Sym, leaf.Pos)

			break
		}

		temp := n.l.arrayTemp(leaf.T, constExtents(leaf))
		n.l.precompute(leaf, temp)
		whole(temp.Sym, leaf.Pos)

	case *sema.FuncRef:
		// An array-valued function's value, computed now, before the loops,
		// into a temporary of its shape, which the call's hidden first
		// argument points at (callArrayFunction): the loops read it. A value
		// whose extents or CHARACTER length are known only when the program runs
		// is the function's to make, described in a descriptor of the
		// statement's (valueResult).
		if leaf.Made {
			whole(n.l.valueResult(leaf).Sym, leaf.Pos)

			break
		}

		temp := n.l.arrayTemp(leaf.T, constExtents(leaf))
		n.l.callArrayFunction(leaf, temp)
		whole(temp.Sym, leaf.Pos)

	case *sema.Section:
		n.prepareSections(info, leaf.Sym, nil, leaf.Subs, loopOf)

	case *sema.FieldRef:
		// A component reference's ranked part is a section of the record
		// array, or of an array field, whose elements At makes.
		n.prepareSections(info, leaf.Sym, leaf.RankedField(), leaf.Sections, loopOf)
	}
}

// prepareSections makes info ready as a section's leaf, of the array sym or, if
// field isn't nil, of that field's dimensions: its subscripts subs, the loop of
// its ranked subscript k loopOf(k).
func (n *arrayNest) prepareSections(info *arrayLeaf, sym *sema.Symbol, field *sema.Field, subs []sema.SectionSub,
	loopOf func(int) int) {
	info.sym, info.field = sym, field
	loop := 0

	for i, sub := range subs {
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

		lower := n.keepIR(n.bound(sub.Lower, sym, i, n.lowerBound))
		upper := n.keepIR(n.bound(sub.Upper, sym, i, n.upperBound))
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
		case *sema.ArrayConst:
			count = LongConst(int32(leaf.Shape()[i].Size))

		case *sema.FuncRef:
			// Its value's extent, or the descriptor's the function filled in.
			count = LongConst(int32(leaf.S[i].Size))
			if !leaf.S[i].Known {
				count = n.l.extent(info.sym, n.l.adjustableOf(info.sym), i)
			}

		case *sema.Constructor:
			// Its temporary's extent (prepare).
			count = LongConst(int32(leaf.Shape()[i].Size))
			if !leaf.S[i].Known {
				count = n.l.extent(info.sym, n.l.adjustableOf(info.sym), i)
			}

		case *sema.ArrayFunc:
			if x := leaf.S[i]; x.Known {
				count = LongConst(int32(x.Size))
			} else if x.Of == sema.Expr(leaf) {
				// An extent of its own: its temporary's.
				count = n.l.symExtent(info.sym, i)
			} else {
				count = n.leafExtent(x.Of, n.loopOf(x.Of, x.Dim))
			}

		case *sema.Section, *sema.FieldRef:
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
	pos := n.l.here()

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

	if f, ok := leaf.(*sema.FieldRef); ok {
		return f.At(n.subscripts(leaf))
	}

	info := n.leaves[leaf]

	return sema.NewElem(info.sym, n.subscripts(leaf), n.inBounds(leaf), leaf.Position())
}

// inBounds says which of the leaf's subscripts are known to be within its
// dimensions at the loops' indexes: all but a vector subscript's, whose values
// are checked where they are used.
func (n *arrayNest) inBounds(leaf sema.Expr) []bool {
	info := n.leaves[leaf]
	list := make([]bool, len(info.dims))

	for i, d := range info.dims {
		list[i] = d.vector == nil
	}

	return list
}

// subscripts are the leaf's subscripts at the loops' indexes.
func (n *arrayNest) subscripts(leaf sema.Expr) []sema.Expr {

	info := n.leaves[leaf]
	subs := make([]sema.Expr, len(info.dims))

	for i, d := range info.dims {
		if d.vector != nil {
			subs[i] = sema.Scalarize(d.vector, n.element, n.hoist)

			continue
		}

		if !d.ranges {
			subs[i] = d.fixed

			continue
		}

		k := sema.NewVar(n.index[d.loop], leaf.Position())
		subs[i] = sema.IntSum(sema.IntProduct(k, d.stride), d.base)
	}

	return subs
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

		switch leaf.(type) {
		case *sema.Section, *sema.FieldRef:
		default:
			continue
		}

		for i, d := range info.dims {
			// The dimensions are the field's when a component's part is the
			// ranked one (T%V(1:N)), and its bounds constants; the symbol
			// is then the record's, which may have no dimensions at all.
			var low, high Expr
			if info.field != nil {
				dim := info.field.Dims[i]
				low, high = LongConst(int32(dim.Lower)), LongConst(int32(dim.Upper))
			} else {
				low, high = n.lowerBound(info.sym, i), n.checkedUpper(info.sym, i)
			}

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

// elementalCall lowers an elemental subroutine's call on arrays (Fortran 90's
// MVBITS and Fortran 95's ELEMENTAL subroutines; sema's Call.Shape): a nest of
// loops over their shape, the leftmost dimension innermost, calling the
// subroutine once for each element with each array argument's element there and
// each scalar one as it is, a scalar that isn't a variable or a constant
// computed once before the loops. The calls go in array element order, which is
// what the standard asks (Fortran 95's 12.4.3; for MVBITS when TO is FROM
// itself or overlaps it, Fortran 90's 13.11). The first array argument is the
// one whose extents the loops take where none is known when compiling.
func (l *lowerer) elementalCall(s *sema.Call) {
	l.maskedCall(s, nil)
}

// maskedCall is elementalCall where mask holds (an elemental defined
// assignment in a WHERE), or everywhere for a nil mask. A defined assignment's
// calls run as its Order says: with a temporary the value, argument 2, is
// computed into one first, where the mask holds, and the calls take its
// elements; otherwise its loops run in the order's directions, a scalar value
// that reads the target computed once before them (sema's ArrayOrder.Hoisted).
func (l *lowerer) maskedCall(s *sema.Call, mask *whereMask) {
	if s.Order != nil && s.Order.Recorded {
		// The value's elements' length isn't known (lowerrecord.go): each
		// call's value recorded with its target, then the calls made.
		walked := *s
		walked.Order = nil

		l.recordedStores(func() { l.maskedCall(&walked, mask) }, s)

		return
	}

	call := *s
	call.Args = slices.Clone(s.Args)
	order := s.Order

	if order != nil && order.Temporary {
		value := call.Args[1].Expr
		t, length := l.tempType(value, value)
		temp := l.shapeTemp(t, order.TempShape, l.knownShape(s.Shape, value.Shape()), length)

		l.arrayNest(temp, value, nil, mask)
		call.Args[1].Expr = temp
		order = nil
	}

	var target sema.Expr

	var leaves []sema.Expr

	for _, a := range call.Args {
		if a.Expr == nil {
			continue
		}

		if len(a.Expr.Shape()) > 0 && target == nil {
			target = a.Expr
		}

		leaves = append(leaves, sema.Leaves(a.Expr)...)
	}

	leaves = append(leaves, mask.leaves()...)

	n := l.newNest(len(s.Shape), order)
	rank := len(n.index)

	for _, leaf := range leaves {
		n.prepare(leaf)
	}

	n.extents = make([]sema.Expr, rank)

	for d := range rank {
		n.extents[d] = n.extent(d, target, target)

		for _, a := range call.Args {
			if a.Expr == nil {
				continue
			}

			if sh := a.Expr.Shape(); d < len(sh) && sh[d].Known {
				n.extents[d] = sema.IntConstAt(sh[d].Size, l.here())

				break
			}
		}
	}

	if l.unit.Checks.Bounds {
		n.checkShapes(leaves)
		n.checkSubscripts()
	}

	each := call
	each.Shape, each.Order = nil, nil
	each.Args = make([]sema.Arg, len(call.Args))

	for i, a := range call.Args {
		each.Args[i] = a
		if a.Expr != nil {
			each.Args[i].Expr = sema.Scalarize(a.Expr, n.element, n.hoist)
		}
	}

	m := n.scalarMasks(mask)

	n.loops(rank-1, func() { l.maskedByAll(m, func() { l.call(&each) }) })

	n.done()
}
