package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Automatic arrays (docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md,
// findings 6 and 7). A subprogram's own array whose bounds aren't constants,
//
//	SUBROUTINE SMOOTH(A, N)
//	REAL A(N), W(0:N+1)
//
// has bounds computed at each entry point, as an adjustable array's are
// (lowerbounds.go), and storage made then, on the stack below the frame, which
// lasts until the routine returns: RET puts SP back where the caller left it, and
// so does an unwind. Its own cell is the class A descriptor that says where the
// storage is, standing among the adjustable arrays' (adjustableOf), so that every
// use of the array reads it as it would a dummy's:
//
//	SUBL2   #1,R0                 ; N+1 - 0 + 1 - 1 ...
//	MOVL    R0,SMOOTH.W+20        ; M1, W's extent
//	...
//	MULL3   #4,SMOOTH.W+20,SMOOTH.W+12    ; ARSIZE
//	SUBL2   SMOOTH.W+12,SP        ; storage of a size known only now
//	BICL2   #3,SP
//	MOVL    SP,SMOOTH.W+4         ; POINTER
//	MOVAL   @SMOOTH.W+4,SMOOTH.W+16       ; A0, W(0)'s address
//
// The standard gives an array whose upper bound is below its lower bound no
// elements (5.1.2.4.1), so such an extent is 0, where an adjustable array's
// isn't changed (FORTRAN-77 called it an error, which /CHECK=BOUNDS reports).

// automaticCell makes the cell c of the automatic array s its descriptor. An
// automatic CHARACTER scalar, whose length isn't constant (lowerlength.go), has a
// class S descriptor.
func (l *lowerer) automaticCell(s *sema.Symbol, c *Cell) {
	d := &Descriptor{DType: l.dtypeOf(s.Type), Class: ClassA, Length: s.Type.Size}
	if s.Dims == nil {
		d.Class = ClassS
	}

	// As for an adjustable array's (prepareBounds): nothing reads the length of
	// an array of records too long for the word.
	if s.Struct != nil && d.Length > maxWordLength {
		d.Length = 0
	}

	for _, dim := range s.Dims {
		d.Dims = append(d.Dims, Bound{dim.Lower, dim.Upper, dim.LowerExpr != nil || dim.UpperExpr != nil})
	}

	c.Kind, c.T, c.Desc = DescriptorCell, Long, d

	if l.adjustable == nil {
		l.adjustable = map[*sema.Symbol]*Cell{}
	}

	l.adjustable[s] = c
}

// computeAutomatic fills in the descriptor c of the automatic array s and makes
// its storage: its bounds and extents, its size, the storage, and A0.
func (l *lowerer) computeAutomatic(s *sema.Symbol, c *Cell) {
	d := c.Desc

	// Before the first statement, what the code comes from is the unit's header:
	// a bound's inquiry (SIZE(A)) is lowered as a statement's would be.
	if l.from == nil {
		l.from = l.unit.AST.Header

		defer func() { l.from = nil }()
	}

	field := func(offset int) *Mem {
		return &Mem{Cell: c, Offset: offset, T: Long}
	}

	store := func(offset int, x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: field(offset), Src: x})
	}

	l.fillDescriptor(c)

	// A CHARACTER length that isn't constant is the descriptor's first word,
	// which every use of the object reads.
	if s.LengthValue != nil {
		n := l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: n, Src: l.expr(s.LengthValue)})
		l.notNegative(n)
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, T: Word}, Src: arith(Cvt, Word, &Load{n}, nil)})
	}

	for i, dim := range s.Dims {
		if !d.Dims[i].Set {
			continue
		}

		lower := Expr(LongConst(int32(dim.Lower)))
		upper := Expr(LongConst(int32(dim.Upper)))

		if dim.LowerValue != nil {
			store(d.LowerOffset(i), l.expr(dim.LowerValue))
			lower = &Load{field(d.LowerOffset(i))}
		}

		if dim.UpperValue != nil {
			store(d.LowerOffset(i)+Long.Size(), l.expr(dim.UpperValue))
			upper = &Load{field(d.LowerOffset(i) + Long.Size())}
		}

		store(d.MultiplierOffset(i), arith(Add, Long, arith(Sub, Long, upper, lower), LongConst(1)))
		l.notNegative(field(d.MultiplierOffset(i)))
	}

	count := l.charLength(s)
	if s.Type.Kind != sema.Character {
		count = LongConst(int32(s.Type.Size))
	}

	if s.Dims == nil {
		l.emit(&StackAllocate{origin: l.origin(), Size: count, Dst: field(PointerOffset)})
		l.release()

		return
	}

	for i := range s.Dims {
		count = arith(Mul, Long, count, l.extent(s, c, i))
	}

	store(ArsizeOffset, count)

	l.emit(&StackAllocate{origin: l.origin(), Size: &Load{field(ArsizeOffset)}, Dst: field(PointerOffset)})

	// A0 is the first element's address, less the element number the lowest
	// subscripts would have counting from all zeros (computeArrayBounds).
	base := &Mem{Ptr: &Load{field(PointerOffset)}, T: l.typeOf(s.Type)}
	first := l.lowerBound(s, c, len(s.Dims)-1)

	for i := len(s.Dims) - 2; i >= 0; i-- {
		first = arith(Add, Long, arith(Mul, Long, first, l.extent(s, c, i)), l.lowerBound(s, c, i))
	}

	store(A0Offset, &Addr{l.elementAt(base, s, negated(first))})

	l.release()
}

// notNegative makes the Long at m 0 if it is less than 0.
func (l *lowerer) notNegative(m *Mem) {
	set, next := l.newBlock(), l.newBlock()
	l.branch(&Cmp{Lt, &Load{m}, LongConst(0)}, set, next)

	l.begin(set)
	l.emit(&Assign{origin: l.origin(), Dst: m, Src: LongConst(0)})
	l.jump(next)

	l.begin(next)
}
