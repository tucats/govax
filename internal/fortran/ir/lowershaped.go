package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// Assumed-shape dummy arguments (docs/planning/PHASE-104 - Fortran 90 dynamic
// arrays.md, findings 9 and 10). In
//
//	SUBROUTINE SCALE(A, F)
//	REAL A(0:, :), F
//
// A's argument is the address of a class NCA descriptor (the OpenVMS Calling
// Standard's 5.7) the caller built: the address of the first element, and for
// each dimension a stride in bytes and the bounds the caller gave, whatever they
// are. The caller may have passed a whole array, or every other row of one, or
// an allocatable array's own descriptor; the routine can't know which. So each
// entry point whose list has A copies that descriptor into a cell of its own
// ($B1), with A's own lower bounds, 0 and 1 here (5.1.2.4.2), and upper bounds
// that keep the actual's extents:
//
//	MOVL    4(AP),R0                ; the caller's descriptor
//	MOVL    4(R0),SCALE.$B1+4       ; POINTER, the first element
//	MOVL    20(R0),SCALE.$B1+20     ; S1, a stride in bytes
//	CLRL    SCALE.$B1+28            ; L1, A's own lower bound
//	SUBL3   28(R0),32(R0),R1        ; U1 - L1 of the actual's
//	MOVL    R1,SCALE.$B1+32         ; ... plus A's L1: A's U1
//	...
//
// then A0 and ARSIZE from those. Everything in the routine then reads that one
// cell, in a place it knows, as it reads an allocatable array's descriptor
// (lowerallocate.go): it stands among the adjustable arrays' cells
// (adjustableOf), so bounds, extents, SIZE, and the scalarizer's loops find it.
// One difference: an allocatable array's elements are one after another, so its
// first stride is the element's size and its first subscript an index; an
// assumed-shape array's first stride may be anything (every other element of a
// row), so an element is A0 plus every subscript times its stride:
//
//	A(I, J)  is at  A0 + S1*I + S2*J
//
// An OPTIONAL one the caller left out has no descriptor to copy: its entry code
// is passed over (PRESENT tests the argument, never the cell).
//
// A caller builds the descriptor in a descriptor cell of its own for each call
// (shapedArg): for a whole array its own bounds and the strides its extents
// give, for a section the bounds 1 to each triplet's extent and the strides of
// the triplets' steps through the array, and its POINTER the first element's
// address. An allocatable array's own descriptor and an assumed-shape dummy's
// cell are already one, and are passed as they are: the routine copies what it
// is given, and never writes it.

// shapedCell makes the cell of the assumed-shape dummy s, the nth of the
// routine's array descriptor cells.
func (l *lowerer) shapedCell(s *sema.Symbol, n int) {
	d := &Descriptor{DType: l.dtypeOf(s.Type), Class: ClassNCA, Length: s.Type.Size}

	for range s.Dims {
		d.Dims = append(d.Dims, Bound{Set: true})
	}

	c := l.newCell("$B"+strconv.Itoa(n), DescriptorCell, Long, d.Size())
	c.Desc, c.Dummy = d, s.Name

	if l.adjustable == nil {
		l.adjustable = map[*sema.Symbol]*Cell{}
	}

	l.adjustable[s] = c
	l.adjustableOrder = append(l.adjustableOrder, s)
}

// computeShaped copies, at an entry point, the descriptor the caller passed for
// the assumed-shape dummy s into its cell c, with s's own bounds.
func (l *lowerer) computeShaped(s *sema.Symbol, c *Cell) {
	// Before the first statement, what the code comes from is the unit's
	// header: a lower bound may ask an inquiry function (computeAutomatic).
	if l.from == nil {
		l.from = l.unit.AST.Header

		defer func() { l.from = nil }()
	}

	p := l.paramOf(s)
	d := c.Desc

	actual := func(offset int) Expr {
		return &Load{descriptorField(p, offset, Long)}
	}

	field := func(offset int) *Mem {
		return &Mem{Cell: c, Offset: offset, T: Long}
	}

	store := func(offset int, x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: field(offset), Src: x})
	}

	// An OPTIONAL dummy left out has no descriptor.
	var next *Block

	if s.Optional {
		var given *Block

		given, next = l.newBlock(), l.newBlock()
		l.branch(&Cmp{Op: Ne, X: &ArgPointer{p}, Y: LongConst(0)}, given, next)
		l.begin(given)
	}

	l.fillDescriptor(c)

	// A passed length is the actual's elements' length, in its descriptor's
	// first word; one computed at entry is the dummy's (lowerlength.go).
	if s.Type.Kind == sema.Character && s.Type.Size == 0 {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, T: Word}, Src: l.lengthWord(s)})
	}

	store(PointerOffset, actual(PointerOffset))

	for i, dim := range s.Dims {
		store(d.StrideOffset(i), actual(d.StrideOffset(i)))

		lower := Expr(LongConst(int32(dim.Lower)))
		if dim.LowerValue != nil {
			store(d.LowerOffset(i), l.expr(dim.LowerValue))
			lower = &Load{field(d.LowerOffset(i))}
		} else {
			store(d.LowerOffset(i), lower)
		}

		// The actual's extent, never below 0, kept by an upper bound of the
		// dummy's own.
		extent := l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: extent,
			Src: arith(Add, Long, arith(Sub, Long, actual(d.LowerOffset(i)+Long.Size()), actual(d.LowerOffset(i))), LongConst(1))})
		l.notNegative(extent)
		store(d.LowerOffset(i)+Long.Size(), arith(Sub, Long, arith(Add, Long, lower, &Load{extent}), LongConst(1)))
	}

	// A0, where the element of all zero subscripts would be: POINTER less each
	// lower bound's step through its dimension.
	a0 := Expr(&Load{field(PointerOffset)})
	for i := range s.Dims {
		a0 = arith(Sub, Long, a0, arith(Mul, Long, &Load{field(d.StrideOffset(i))}, l.lowerBound(s, c, i)))
	}

	store(A0Offset, a0)

	size := l.elementSize(s)
	for i := range s.Dims {
		size = arith(Mul, Long, size, l.extent(s, c, i))
	}

	store(ArsizeOffset, size)

	if next != nil {
		l.jump(next)
		l.begin(next)
	}

	l.release()
}

// elementSize is the size in bytes of an element of the array s: its type's,
// or a CHARACTER array's length (a passed length, or one computed at entry).
func (l *lowerer) elementSize(s *sema.Symbol) Expr {
	if s.Type.Kind == sema.Character {
		return l.charLength(s)
	}

	return LongConst(int32(s.Type.Size))
}

// baseStride is the stride in bytes of dimension i of the array s, whose
// descriptor cell is c (nil for an array whose bounds are constants): a class
// NCA descriptor's own, or else the element's size times the extents of the
// dimensions before it, the array being one element after another.
func (l *lowerer) baseStride(s *sema.Symbol, c *Cell, i int) Expr {
	if c != nil && c.Desc.Class == ClassNCA {
		return &Load{&Mem{Cell: c, Offset: c.Desc.StrideOffset(i), T: Long}}
	}

	stride := l.elementSize(s)

	for j := range i {
		if c != nil {
			stride = arith(Mul, Long, stride, l.extent(s, c, j))
		} else {
			stride = arith(Mul, Long, stride, LongConst(int32(s.Dims[j].Extent())))
		}
	}

	return stride
}

// ncaDim is one dimension of a class NCA descriptor a caller builds: its
// bounds, its extent, and its stride in bytes.
type ncaDim struct {
	lower, upper, extent, stride Expr
}

// shapedArg is the class NCA descriptor of e, the actual argument of an
// assumed-shape dummy: a whole array, or a section (sema's shaped.go).
func (l *lowerer) shapedArg(e sema.Expr) *Mem {
	switch e := e.(type) {
	case *sema.ArrayRef:
		s := e.Sym
		c := l.adjustableOf(s)

		// An allocatable array's own descriptor, or an assumed-shape dummy's
		// cell: one already.
		if c != nil && c.Desc.Class == ClassNCA {
			return &Mem{Cell: c, T: Long}
		}

		dims := make([]ncaDim, len(s.Dims))

		for i := range s.Dims {
			extent := Expr(LongConst(int32(s.Dims[i].Extent())))
			if c != nil {
				extent = l.extent(s, c, i)
			}

			dims[i] = ncaDim{l.lowerBound(s, c, i), l.upperBound(s, c, i), extent, l.baseStride(s, c, i)}
		}

		return l.buildNCA(s, &Addr{l.symbolMem(s)}, dims)

	case *sema.Section:
		return l.sectionNCA(e)
	}

	panic("ir: an assumed-shape dummy's actual argument that is neither an array nor a section")
}

// sectionNCA is the class NCA descriptor of the section e: its first element's
// address, and for each triplet the bounds 1 and its extent, and its step
// through the array, the dimension's stride times the triplet's. The section's
// subscripts are computed once, as the scalarizer computes them (arrayNest), and
// under /CHECK=BOUNDS the first and last subscript of each triplet are checked.
func (l *lowerer) sectionNCA(e *sema.Section) *Mem {
	s := e.Sym
	c := l.adjustableOf(s)

	n := l.newNest(len(e.S), nil)
	defer n.done()

	n.prepare(e)

	if l.unit.Checks.Bounds {
		n.checkSubscripts()
	}

	info := n.leaves[e]
	subs := make([]sema.Expr, len(info.dims))
	inBounds := make([]bool, len(info.dims))

	var dims []ncaDim

	for i, d := range info.dims {
		inBounds[i] = true

		if !d.ranges {
			subs[i] = d.fixed

			continue
		}

		// The triplet's first subscript, its base plus one step.
		subs[i] = sema.IntSum(d.stride, d.base)

		extent := n.leafExtent(e, d.loop)
		stride := arith(Mul, Long, l.baseStride(s, c, i), l.expr(d.stride))
		dims = append(dims, ncaDim{LongConst(1), extent, extent, stride})
	}

	first := l.mem(sema.NewElem(s, subs, inBounds, e.Pos))

	return l.buildNCA(s, &Addr{first}, dims)
}

// buildNCA is a class NCA descriptor, in a descriptor cell of the statement's,
// of elements of the array s's type, the first at pointer, with the dimensions
// dims; A0 and ARSIZE computed from them.
func (l *lowerer) buildNCA(s *sema.Symbol, pointer Expr, dims []ncaDim) *Mem {
	d := &Descriptor{DType: l.dtypeOf(s.Type), Class: ClassNCA, Length: s.Type.Size}

	for range dims {
		d.Dims = append(d.Dims, Bound{Set: true})
	}

	c := l.descriptorCell(d)

	field := func(offset int) *Mem {
		return &Mem{Cell: c, Offset: offset, T: Long}
	}

	// value stores x at offset, and is x again: itself if it is a constant,
	// else what was stored, read back, so that it is computed once.
	value := func(offset int, x Expr) Expr {
		l.emit(&Assign{origin: l.origin(), Dst: field(offset), Src: x})

		if _, ok := x.(*Const); ok {
			return x
		}

		return &Load{field(offset)}
	}

	if s.Type.Kind == sema.Character && s.Type.Size == 0 {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, T: Word}, Src: l.lengthWord(s)})
	}

	a0 := value(PointerOffset, pointer)
	size := l.elementSize(s)

	for i, dm := range dims {
		stride := value(d.StrideOffset(i), dm.stride)
		lower := value(d.LowerOffset(i), dm.lower)
		value(d.LowerOffset(i)+Long.Size(), dm.upper)

		a0 = arith(Sub, Long, a0, arith(Mul, Long, stride, lower))
		size = arith(Mul, Long, size, dm.extent)
	}

	value(A0Offset, a0)
	value(ArsizeOffset, size)

	return &Mem{Cell: c, T: Long}
}
