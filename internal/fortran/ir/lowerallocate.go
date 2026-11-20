package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Allocatable arrays (docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md,
// findings 1, 2, and 4). An allocatable array's storage is its descriptor, class
// NCA (the OpenVMS Calling Standard's 5.7), in the variable's own cell:
//
//	REAL, ALLOCATABLE :: B(:, :)
//
//	B:   .WORD   4                  ; length: an element's size
//	     .BYTE   DSC$K_DTYPE_F,DSC$K_CLASS_NCA
//	     .LONG   0                  ; POINTER: 0 while B isn't allocated
//	     .BYTE   0,0,0,2            ; scale, digits, flags, dimensions
//	     .LONG   0                  ; ARSIZE, the array's size in bytes
//	     .LONG   0                  ; A0
//	     .LONG   0,0                ; S1, S2: strides, in bytes
//	     .LONG   0,0,0,0            ; L1, U1, L2, U2: bounds
//
// ALLOCATE (B(0:M, 3)) is a call of FTNRTL's FTN$ALLOCATE with the descriptor's
// address, a longword of flags, and the bounds by value, which takes the
// memory from LIB$GET_VM and fills the descriptor in: POINTER the first
// element's address, strides of 4 and 4*(M+1), and A0 the address the element
// B(0, 0) would have, POINTER less S1*L1 + S2*L2. DEALLOCATE (B) is a call of
// FTN$DEALLOCATE, which gives the memory back and clears POINTER, and
// ALLOCATED(B) is a test of POINTER.
//
// Everything else reads the descriptor as an adjustable array's is read
// (lowerbounds.go), its cell standing among the adjustable arrays' (adjustableOf),
// but for two differences. Every bound is in the descriptor, none known when
// compiling. And an element is reached through the strides, in bytes, not the
// multipliers: B(I, J) is at A0 + S2*J, indexed by I, since an allocatable
// array's storage is contiguous, its S1 the element's size:
//
//	MOVL   B+24,R0              S2
//	MULL2  J,R0                 times J
//	ADDL2  B+16,R0              plus A0
//	MOVL   I,R1
//	MOVF   (R0)[R1],R2          the element
//
// A rank-one array's element is A0 indexed by its subscript, @B+16[R1] when the
// optimizer leaves A0 in memory.

// FTNRTL's routines for ALLOCATE and DEALLOCATE (mar/ftn_alloc.mar).
const (
	ftnAllocate      = "FTN$ALLOCATE"
	ftnDeallocate    = "FTN$DEALLOCATE"
	ftnAllocScalar   = "FTN$ALLOC_SCALAR"
	ftnDeallocScalar = "FTN$DEALLOC_SCALAR"
)

// allocatableCell makes the cell c of the allocatable array s its descriptor.
func (l *lowerer) allocatableCell(s *sema.Symbol, c *Cell) {
	c.Kind, c.T, c.Desc = DescriptorCell, Long, l.ncaDescriptor(s)

	if l.adjustable == nil {
		l.adjustable = map[*sema.Symbol]*Cell{}
	}

	l.adjustable[s] = c
}

// ncaDescriptor is the class NCA descriptor of the allocatable array or array
// pointer s, its bounds set when the program runs.
func (l *lowerer) ncaDescriptor(s *sema.Symbol) *Descriptor {
	d := &Descriptor{DType: l.dtypeOf(s.Type), Class: ClassNCA, Length: s.Type.Size}

	for range s.Dims {
		d.Dims = append(d.Dims, Bound{Set: true})
	}

	return d
}

// clearAllocatables writes, at an entry point, what the descriptor of each
// allocatable array in the frame starts as: a frame cell has no initial values,
// and an earlier activation, or anything else, may have left any bytes there. A
// static one's are its initial values, but one its unit frees at every return
// (sema.Symbol.FreedAtReturn) has its POINTER cleared too: a routine left by an
// unwind (a condition handler's $UNWIND) runs none of its returns' code, and the
// next call's ALLOCATE would then find the array still allocated. The block is
// lost, but the program goes on (finding 5). Each pointer in the frame is
// cleared then too (lowerpointer.go's clearPointers).
func (l *lowerer) clearAllocatables() {
	for _, c := range l.r.Cells {
		switch {
		case !isAllocatable(c):
		case c.Frame:
			l.fillDescriptor(c)
		case c.Sym.FreedAtReturn:
			l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: PointerOffset, T: Long}, Src: LongConst(0)})
		}
	}

	l.clearPointers()
	l.clearPointerResult()
}

// entryClears reports whether the routine has an allocatable array or a
// pointer that each entry point clears (clearAllocatables).
func (l *lowerer) entryClears() bool {
	if l.pointerResult() != nil {
		return true
	}

	for _, c := range l.r.Cells {
		if isAllocatable(c) && (c.Frame || c.Sym.FreedAtReturn) {
			return true
		}

		if c.Frame && c.Sym != nil && (c.Kind == VariableCell && c.Sym.Pointer ||
			(c.Kind == VariableCell || c.Kind == ArrayCell) && clearsPointers(c.Sym.Type)) {
			return true
		}
	}

	return false
}

// freesAtReturn reports whether a return of the routine deallocates anything.
func (l *lowerer) freesAtReturn() bool {
	for _, c := range l.r.Cells {
		if isAllocatable(c) && c.Sym.FreedAtReturn {
			return true
		}
	}

	return false
}

// freeAllocatables deallocates, before a return, each allocatable array the
// unit frees at every return that is allocated now (the standard's 6.3.3.1):
//
//	TSTL   W+4                  POINTER: allocated?
//	BEQL   1$
//	PUSHL  #0                   no STAT=
//	PUSHAB W
//	CALLS  #2,G^FTN$DEALLOCATE
//	1$:
//
// The statements to be done at the statement's end come first.
func (l *lowerer) freeAllocatables() {
	l.flush()

	for _, c := range l.r.Cells {
		if !isAllocatable(c) || !c.Sym.FreedAtReturn {
			continue
		}

		free, next := l.newBlock(), l.newBlock()
		pointer := &Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}
		l.branch(&Cmp{Op: Ne, X: pointer, Y: LongConst(0)}, free, next)

		l.begin(free)
		free.Note = "deallocates " + c.Sym.Name + " at the return"
		l.emit(&Call{origin: l.origin(), Routine: ftnDeallocate,
			Args: []Arg{{Ref: &Mem{Cell: c, T: Long}}, {Value: LongConst(0)}}})
		l.begin(next)
	}
}

// isAllocatable reports whether c is an allocatable array's descriptor.
func isAllocatable(c *Cell) bool {
	return c.Kind == DescriptorCell && c.Desc != nil && c.Desc.Class == ClassNCA && c.Sym != nil
}

// allocate lowers ALLOCATE or DEALLOCATE: a call of FTNRTL for each object, in
// the statement's order. Without STAT= the flags are 0, and FTNRTL signals a
// failure itself. With it they are 1, and FTNRTL returns 0 or the failure's FOR
// number, which goes to STAT='s variable; the first failure ends the statement:
//
//	PUSHL  N                    ALLOCATE (A(N), B(N), STAT=IS)
//	PUSHL  #1
//	PUSHL  #1                   STAT= given
//	PUSHAB A
//	CALLS  #4,G^FTN$ALLOCATE
//	MOVL   R0,$T1
//	BNEQ   1$                   failed: B is left as it was
//	...                         the same for B
//	1$: MOVL $T1,IS
func (l *lowerer) allocate(s *sema.Allocate) {
	var status *Mem

	flags := LongConst(0)
	done := l.newBlock()

	if s.Stat != nil {
		status, flags = l.temp(Long), LongConst(allocStat)
	}

	for i, obj := range s.Objects {
		switch {
		case obj.Component != nil:
			l.allocateScalar(obj.Component, s.Deallocate, flags, status)
		case obj.Sym.Pointer && !obj.Sym.ArrayPointer():
			l.allocateScalar(sema.NewVar(obj.Sym, obj.Sym.Pos), s.Deallocate, flags, status)
		default:
			l.allocateArray(obj, s.Deallocate, flags, status)
		}

		if status != nil && i < len(s.Objects)-1 {
			next := l.newBlock()
			l.branch(&Cmp{Op: Ne, X: &Load{status}, Y: LongConst(0)}, done, next)
			l.begin(next)
		}
	}

	if status == nil {
		l.made--

		return
	}

	l.begin(done)

	dst := l.mem(s.Stat)

	var value Expr = &Load{status}
	if dst.T != Long {
		value = arith(Cvt, dst.T, value, nil)
	}

	l.emit(&Assign{origin: l.origin(), Dst: dst, Src: value})
}

// allocateArray is the call of FTN$ALLOCATE or FTN$DEALLOCATE for obj, an
// allocatable array or an array pointer, its descriptor's address first. An
// array pointer's flags have allocPointer too, and one in COMMON has its
// descriptor's unchanging longwords written first, which FTN$ALLOCATE reads
// (lowerpointer.go's commonHeader).
func (l *lowerer) allocateArray(obj *sema.Allocation, deallocate bool, flags *Const, status *Mem) {
	c := l.adjustableOf(obj.Sym)

	// Elements with pointers in them start disassociated (the author's answer 2).
	if clearsPointers(obj.Sym.Type) {
		flags = LongConst(flags.Int() | allocClear)
	}

	if obj.Sym.Pointer {
		flags = LongConst(flags.Int() | allocPointer)

		if !deallocate {
			l.commonHeader(c)
		}
	}

	args := []Arg{{Ref: &Mem{Cell: c, T: Long}}, {Value: flags}}

	routine := ftnDeallocate
	if !deallocate {
		routine = ftnAllocate

		for i := range obj.Upper {
			args = append(args, Arg{Value: l.expr(obj.Lower[i])}, Arg{Value: l.expr(obj.Upper[i])})
		}
	}

	l.emit(&Call{origin: l.origin(), Routine: routine, Args: args, Result: status})
}

// allocateScalar is the call of FTN$ALLOC_SCALAR or FTN$DEALLOC_SCALAR for
// the scalar pointer p: the address of the longword that holds its target's
// address (a CHARACTER pointer's descriptor's POINTER, its length and type
// written first), the flags, and the target's size in bytes, by value:
//
//	PUSHL  #4                   ALLOCATE (P), P an INTEGER pointer
//	PUSHL  #2                   a pointer's
//	PUSHAL P
//	CALLS  #3,G^FTN$ALLOC_SCALAR
func (l *lowerer) allocateScalar(p sema.Expr, deallocate bool, flags *Const, status *Mem) {
	routine := ftnDeallocScalar
	if !deallocate {
		routine = ftnAllocScalar

		l.setHeader(p)
	}

	bits := flags.Int() | allocPointer
	if !deallocate && clearsPointers(p.Type()) {
		bits |= allocClear
	}

	size := Expr(LongConst(int32(p.Type().Size)))
	if charPointer(p.Type()) {
		size = l.pointerLength(p)
	}

	args := []Arg{{Ref: l.pointerOf(p)}, {Value: LongConst(bits)}, {Value: size}}

	l.emit(&Call{origin: l.origin(), Routine: routine, Args: args, Result: status})
}

// FTN$ALLOCATE's, FTN$DEALLOCATE's, and the scalar routines' flags:
// allocStat for a statement with STAT=, to return a failure's FOR number
// rather than signal it; allocPointer for a pointer, whose block FTNRTL marks
// so that DEALLOCATE can tell it made it, and which ALLOCATE gives a new
// target whatever it had (docs/planning/PHASE-107 - Fortran 90 pointers.md,
// finding 6); allocClear for elements of a type with pointer components,
// whose block FTNRTL clears so that each pointer in it starts disassociated
// (the author's answer 2).
const (
	allocStat    = 1
	allocPointer = 2
	allocClear   = 4
)

// clearsPointers reports whether new storage for values of type t must be
// cleared: a derived type with a pointer component, at any depth.
func clearsPointers(t sema.Type) bool {
	return t.Kind == sema.Record && t.Struct != nil && t.Struct.HasPointers()
}

// allocated is ALLOCATED(A): whether A's descriptor has an address.
func (l *lowerer) allocated(e *sema.Allocated) Expr {
	c := l.adjustableOf(e.Sym)

	return &Cmp{Op: Ne, X: &Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}, Y: LongConst(0)}
}

// allocatableBase is where the allocatable array s's first element is, the
// address in its descriptor's POINTER: what a whole array passed by reference is.
func (l *lowerer) allocatableBase(s *sema.Symbol) *Mem {
	c := l.adjustableOf(s)

	pointer := l.guardedElement(&Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}, s, c)

	return &Mem{Ptr: pointer, T: l.typeOf(s.Type), Volatile: s.Volatile, Pointee: arrayPointee(s)}
}

// stridedElement is where the element e of the array whose class NCA
// descriptor is c is: A0 plus each subscript after the first times its
// dimension's stride, indexed by the first, the element's size being the
// first's stride (finding 2). An assumed-shape array's or an array pointer's
// first stride may be any (lowershaped.go, lowerpointer.go), so its first
// subscript is multiplied by it too.
func (l *lowerer) stridedElement(e *sema.Elem, c *Cell) *Mem {
	s := e.Sym
	subs := make([]Expr, len(e.Subs))

	for i, sub := range e.Subs {
		subs[i] = l.expr(sub)

		if l.unit.Checks.Bounds && e.Checked(i) {
			subs[i] = inRange(subs[i], l.lowerBound(s, c, i), l.upperBound(s, c, i))
		}
	}

	var ptr Expr = &Load{&Mem{Cell: c, Offset: A0Offset, T: Long}}

	first := 1
	if s.Strided() {
		first = 0
	}

	for i := first; i < len(subs); i++ {
		stride := &Load{&Mem{Cell: c, Offset: c.Desc.StrideOffset(i), T: Long}}
		ptr = arith(Add, Long, ptr, arith(Mul, Long, subs[i], stride))
	}

	ptr = l.guardedElement(ptr, s, c)

	base := &Mem{Ptr: ptr, T: l.typeOf(s.Type), Volatile: s.Volatile, Pointee: arrayPointee(s)}

	if first == 0 {
		return base
	}

	return l.elementAt(base, s, subs[0])
}

// ncaAsClassA is a class A descriptor of the whole array whose class NCA
// descriptor is c, for what reads only class A (FTNRTL's I/O lists and run-time
// formats, a CHARACTER array argument): an allocatable array is contiguous, so
// its multipliers are its extents, and its A0 is the same address.
func (l *lowerer) ncaAsClassA(s *sema.Symbol, c *Cell) *Mem {
	d := &Descriptor{DType: c.Desc.DType, Class: ClassA, Length: c.Desc.Length}
	for range s.Dims {
		d.Dims = append(d.Dims, Bound{Set: true})
	}

	a := l.descriptorCell(d)

	copyField := func(offset int) {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: a, Offset: offset, T: Long},
			Src: &Load{&Mem{Cell: c, Offset: offset, T: Long}}})
	}

	// A CHARACTER length known only when the program runs (a function's value,
	// lowervalue.go) is the NCA descriptor's first word.
	if d.Length == 0 && s.Type.Kind == sema.Character {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: a, T: Word}, Src: &Load{&Mem{Cell: c, T: Word}}})
	}

	l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: a, Offset: PointerOffset, T: Long},
		Src: l.guardedElement(&Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}, s, c)})
	copyField(ArsizeOffset)
	copyField(A0Offset)

	for i := range s.Dims {
		copyField(d.LowerOffset(i))
		copyField(d.LowerOffset(i) + Long.Size())
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: a, Offset: d.MultiplierOffset(i), T: Long},
			Src: l.extent(s, c, i)})
	}

	return &Mem{Cell: a, T: Long}
}
