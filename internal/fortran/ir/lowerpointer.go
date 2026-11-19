package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Pointers (docs/planning/PHASE-107 - Fortran 90 pointers.md, findings 2 to
// 5). A scalar pointer's cell is a longword, its target's address or 0; a
// CHARACTER pointer's is a class S descriptor, the target's length and address,
// as a CHARACTER dummy argument's is. So
//
//	INTEGER, POINTER :: P
//	P => N                 MOVAL  N, P
//	P = 5                  MOVL   P, R0
//	                       MOVL   #5, (R0)
//	NULLIFY (P)            CLRL   P
//	IF (ASSOCIATED (P))    TSTL   P
//
// A reference to a pointer is its target's (sema/pointer.go): symbolMem gives
// the place the cell's address points at (pointerTargetMem), and everything that
// reads, writes, or passes a variable goes through it unchanged. An actual
// argument is its target's address, the cell's longword itself.
//
// An array pointer's cell is a class NCA descriptor (subtask 3), laid out as an
// allocatable array's (lowerallocate.go) and standing among the adjustable
// arrays' cells, so bounds, extents, SIZE, and the scalarizer's loops read it.
// Its target may be a section, so its elements are reached as an assumed-shape
// array's are, through every stride (stridedElement; sema.Symbol.Strided), and
// it is never taken to be contiguous. Pointer assignment fills the descriptor
// in: for P => A(2:9:3, J)
//
//	MOVAL  A+4,P+4              POINTER, the first element, A(2, J)
//	MOVL   #12,P+20             S1: 3 elements of 4 bytes
//	MOVL   #1,P+28              L1: a section's bounds start at 1
//	MOVL   #3,P+32              U1: its extent
//	...                         A0 and ARSIZE from those
//
// and for P => A the same with A's own bounds; P => Q copies Q's descriptor.
// NULLIFY clears POINTER, as DEALLOCATE does an allocatable array's.
//
// A pointer dummy argument (subtask 6a) has no storage of its own: its
// argument is the address of the caller's pointer, so its storage is there
// (pointerStorage), and an array pointer dummy's descriptor is a located cell at
// that address (dummyPointerCell). What the routine does to it, it does to the
// caller's pointer:
//
//	SUBROUTINE AIM(P, T)          INTEGER, POINTER :: P
//	P => T                        MOVAL  @AIM.T(AP),@AIM.P(AP)

// charPointerHeader is a CHARACTER pointer's descriptor's first longword for a
// target of length n: the length, DSC$K_DTYPE_T, and DSC$K_CLASS_S.
func charPointerHeader(n int) Expr {
	return LongConst(int32(n | int(DTypeT)<<16 | int(ClassS)<<24))
}

// pointerStorage is where the named pointer s's own storage starts: its cell
// (an array pointer's descriptor, adjustableOf), or for a pointer dummy
// argument the caller's own pointer, at the address its argument holds
// (finding 8: so the dummy's =>, NULLIFY, ALLOCATE, and DEALLOCATE are the
// caller's, with nothing copied in or back).
func (l *lowerer) pointerStorage(s *sema.Symbol) *Mem {
	if s.ArrayPointer() {
		return &Mem{Cell: l.adjustableOf(s), T: Long, Volatile: s.Volatile}
	}

	if p := l.paramOf(s); p != nil {
		return &Mem{Ptr: &ArgPointer{p}, T: Long, Volatile: s.Volatile}
	}

	return &Mem{Cell: l.cellOf(s), T: Long, Volatile: s.Volatile}
}

// pointerAddress is the longword of the pointer s that holds its target's
// address: its storage's first, or a CHARACTER or array pointer's
// descriptor's POINTER.
func (l *lowerer) pointerAddress(s *sema.Symbol) *Mem {
	m := l.pointerStorage(s)

	if s.ArrayPointer() || s.Type.Kind == sema.Character {
		m.Offset = PointerOffset
	}

	return m
}

// dummyPointerCell is the descriptor of the array pointer dummy argument s, a
// located cell (locate.go) at the address its argument holds: the caller's
// pointer's own descriptor, which every routine reading or filling a
// descriptor reaches as a cell. nil if s isn't such a dummy.
func (l *lowerer) dummyPointerCell(s *sema.Symbol) *Cell {
	if !s.ArrayPointer() || s.Component != nil {
		return nil
	}

	p := l.paramOf(s)
	if p == nil {
		return nil
	}

	d := l.ncaDescriptor(s)

	return &Cell{Name: s.Name, Kind: DescriptorCell, T: Long, Size: d.Size(), Desc: d, Arg: p, Sym: s}
}

// pointerLength is the length of the values the CHARACTER pointer x points
// at: its declared one, or a passed-length dummy's, which its actual's
// descriptor holds.
func (l *lowerer) pointerLength(x sema.Expr) Expr {
	if n := x.Type().Size; n > 0 {
		return LongConst(int32(n))
	}

	return l.charLength(sema.PointerSymbol(x))
}

// pointerOf is the address longword of the pointer x, a reference to a pointer
// itself (sema.IsPointer): a named pointer's (pointerAddress), or a pointer
// component's in its record, HEAD%NEXT (subtask 5: the record reached as any
// field reference's is, lowerfield.go), a CHARACTER one's descriptor's POINTER.
func (l *lowerer) pointerOf(x sema.Expr) *Mem {
	if p := sema.PointerSymbol(x); p != nil {
		return l.pointerAddress(p)
	}

	f := x.(*sema.FieldRef)
	m := l.fieldPlace(f, false)

	if charPointer(f.Type()) {
		m.Offset += PointerOffset
	}

	return m
}

// charPointer reports whether a pointer to values of type t is a CHARACTER
// pointer, whose storage is a class S descriptor.
func charPointer(t sema.Type) bool {
	return t.Kind == sema.Character
}

// pointerTargetMem is where the pointer s's target is: at the address its cell
// holds. A disassociated pointer's is address 0, which is no place a program may
// read or write (a reference to one is the program's error; /CHECK=POINTERS will
// be Phase 110's).
func (l *lowerer) pointerTargetMem(s *sema.Symbol) *Mem {
	t := l.typeOf(s.Type)
	if s.Type.Kind == sema.Character {
		t = Byte
	}

	return &Mem{Ptr: &Load{l.pointerAddress(s)}, T: t, Volatile: s.Volatile}
}

// targetAddress is the address of the place target, a variable that sema says
// may be a pointer's target, or for a pointer its target's address (0 if it is
// disassociated): what a pointer assigned to it holds.
func (l *lowerer) targetAddress(target sema.Expr) Expr {
	target = l.targetPointer(target)

	if sema.IsPointer(target) && !isArrayPointer(target) {
		return &Load{l.pointerOf(target)}
	}

	if q := sema.PointerSymbol(target); q != nil {
		return &Load{l.pointerAddress(q)}
	}

	if target.Type().Kind == sema.Character {
		return &Addr{l.chars(target).Mem}
	}

	return &Addr{l.mem(target)}
}

// pointerAssign is Pointer => Target: the target's address stored in the
// pointer, and for a CHARACTER pointer its descriptor's length and type first.
// The length is the pointer's declared one, which sema has checked against the
// target's where both are constants.
func (l *lowerer) pointerAssign(s *sema.PointerAssign) {
	if p := sema.PointerSymbol(s.Pointer); p != nil && p.ArrayPointer() {
		l.arrayPointerAssign(p, s.Target)

		return
	}

	// The target's address first: P%NEXT => P%NEXT%NEXT reads the pointer it
	// changes.
	address := l.targetAddress(s.Target)

	l.setHeader(s.Pointer)
	l.emit(&Assign{origin: l.origin(), Dst: l.pointerOf(s.Pointer), Src: address})
}

// isArrayPointer reports whether x is a reference to an array pointer itself.
func isArrayPointer(x sema.Expr) bool {
	p := sema.PointerSymbol(x)

	return p != nil && p.ArrayPointer()
}

// setPointerHeader writes the CHARACTER pointer p's descriptor's first
// longword, its length and type; nothing for any other pointer.
//
// A passed-length dummy's header is its actual's, which its length is read
// from: nothing is written, the caller having written it (pointerActual).
func (l *lowerer) setPointerHeader(p *sema.Symbol) {
	if p.Type.Kind != sema.Character || p.ArrayPointer() || p.Type.Size == 0 {
		return
	}

	l.emit(&Assign{origin: l.origin(), Dst: l.pointerStorage(p), Src: charPointerHeader(p.Type.Size)})
}

// setHeader is setPointerHeader for the pointer x, a named pointer or a pointer
// component (whose descriptor is in its record).
func (l *lowerer) setHeader(x sema.Expr) {
	if p := sema.PointerSymbol(x); p != nil {
		l.setPointerHeader(p)

		return
	}

	if t := x.Type(); charPointer(t) {
		l.emit(&Assign{origin: l.origin(), Dst: l.fieldPlace(x.(*sema.FieldRef), false), Src: charPointerHeader(t.Size)})
	}
}

// nullify is NULLIFY: each pointer's address 0.
func (l *lowerer) nullify(s *sema.Nullify) {
	for _, x := range s.Pointers {
		l.setHeader(x)
		l.emit(&Assign{origin: l.origin(), Dst: l.pointerOf(x), Src: LongConst(0)})
	}
}

// associated is ASSOCIATED(P [, T]): whether P's address isn't 0; with a
// variable T, whether it is T's address; with a pointer T, whether the two
// addresses are the same and not 0 (13.13.13: false if either is
// disassociated). A CHARACTER pointer's length is its declared one, so the
// addresses decide; a target of no characters, a zero-sized one, is never
// associated (the author's answer 4), which for a substring whose length is
// known only when the program runs is a test of it too.
func (l *lowerer) associated(e *sema.Associated) Expr {
	address := &Load{l.pointerOf(e.Pointer)}

	if e.Target == nil {
		return &Cmp{Op: Ne, X: address, Y: LongConst(0)}
	}

	target := l.targetPointer(e.Target)

	if p := sema.PointerSymbol(e.Pointer); p != nil && p.ArrayPointer() {
		return l.arrayAssociated(p, target)
	}

	same := &Cmp{Op: Eq, X: address, Y: l.targetAddress(target)}

	switch {
	case sema.IsPointer(target):
		return arith(And, Long, same, &Cmp{Op: Ne, X: &Load{l.pointerOf(e.Pointer)}, Y: LongConst(0)})

	case target.Type().Kind == sema.Character && target.Type().Size == 0:
		length := l.chars(target).Len

		return arith(And, Long, same, &Cmp{Op: Gt, X: length, Y: LongConst(0)})
	}

	return same
}

// clearPointers clears, at an entry point, each pointer whose cell is in the
// frame, so that it starts disassociated as a static one does (finding 2: a
// frame isn't cleared, and the standard's undefined status is made
// disassociated, as Fortran 95 has it).
//
// So are the pointer components of each record in the frame, and of each
// element of an array of records there (the author's answer 2; subtask 5).
func (l *lowerer) clearPointers() {
	for _, c := range l.r.Cells {
		switch {
		case !c.Frame || c.Sym == nil || c.Kind != VariableCell && c.Kind != ArrayCell:

		case c.Sym.Pointer:
			if c.Kind != VariableCell {
				continue
			}

			l.setPointerHeader(c.Sym)
			l.emit(&Assign{origin: l.origin(), Dst: l.pointerAddress(c.Sym), Src: LongConst(0)})

		case clearsPointers(c.Sym.Type):
			l.clearRecords(&Mem{Cell: c, T: Byte}, LongConst(int32(c.Sym.Elements())), c.Sym.Struct)
		}
	}
}

// clearUnrolled is the most stores clearRecords writes one by one; past it, a
// loop clears the records.
const clearUnrolled = 16

// clearRecords stores 0 in each pointer of n records of the type st, one after
// another from first, a Byte place with no index: each pointer component's
// address longword, at any depth (sema.Structure.PointerOffsets). A few are
// stores in line; more are a loop over the records:
//
//	MOVAB  first,$T1
//	MOVL   n,$T2
//	BLEQ   2$
//	1$: CLRL 4($T1)             each pointer of the record
//	ADDL2  #size,$T1
//	SOBGTR $T2,1$
//	2$:
func (l *lowerer) clearRecords(first *Mem, n Expr, st *sema.Structure) {
	offsets := st.PointerOffsets()

	if k, ok := n.(*Const); ok && int(k.Int())*len(offsets) <= clearUnrolled {
		for i := range int(k.Int()) {
			for _, o := range offsets {
				m := *first
				m.Offset, m.T = first.Offset+i*st.Size+o, Long
				l.emit(&Assign{origin: l.origin(), Dst: &m, Src: LongConst(0)})
			}
		}

		return
	}

	at, count := l.temp(Long), l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: at, Src: &Addr{first}})
	l.emit(&Assign{origin: l.origin(), Dst: count, Src: n})

	body, done := l.newBlock(), l.newBlock()
	body.Note = "clears " + st.Name + "'s pointers"
	l.branch(&Cmp{Op: Gt, X: &Load{count}, Y: LongConst(0)}, body, done)

	l.begin(body)

	for _, o := range offsets {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Ptr: &Load{at}, Offset: o, T: Long}, Src: LongConst(0)})
	}

	l.emit(&Assign{origin: l.origin(), Dst: at, Src: arith(Add, Long, &Load{at}, LongConst(int32(st.Size)))})
	l.emit(&Assign{origin: l.origin(), Dst: count, Src: arith(Sub, Long, &Load{count}, LongConst(1))})
	l.branch(&Cmp{Op: Gt, X: &Load{count}, Y: LongConst(0)}, body, done)

	l.begin(done)
	l.release()
}

// arrayPointerAssign is P => target for the array pointer p (associate).
func (l *lowerer) arrayPointerAssign(p *sema.Symbol, target sema.Expr) {
	l.associate(l.adjustableOf(p), p, target)
}

// associate fills cell, the descriptor of the array pointer p, in to describe
// target: target's descriptor copied, if it is a pointer; else target
// described into p's cell, a whole array with its own bounds, a section (or a
// component with a rank) with bounds from 1 (5.1.2.4.3). A target that reads p
// itself, P => P(N:1:-1), is described into a cell of the statement's first,
// and copied: describing it in place would change the bounds and strides it is
// read through. So is a target reached through a pointer of an array pointer
// component, cell a located one: the pointer may lead back to the record
// (H%V => LP%H%V(4:1:-1), LP pointing at H).
func (l *lowerer) associate(cell *Cell, p *sema.Symbol, target sema.Expr) {
	target = l.targetPointer(target)
	l.commonHeader(cell)

	if q := sema.PointerSymbol(target); q != nil {
		l.copyDescriptor(cell, l.adjustableOf(q))

		return
	}

	base := sema.TargetBase(target)
	inPlace := base != p

	if cell.Located() {
		inPlace = base != nil && !base.Pointer
	}

	if inPlace {
		l.describeTarget(cell, p, target)

		return
	}

	temp := l.descriptorCell(cell.Desc.Clone())
	l.describeTarget(temp, p, target)
	l.copyDescriptor(cell, temp)
}

// commonHeader writes the descriptor's longwords that never change (its
// elements' length and type, its class, its rank) into the array pointer's
// cell c if c is in a common block: no unit's object gives a common block
// initial values, as $LOCAL and a module's section have them, and a routine
// given the pointer's target as an assumed-shape array reads its class and
// rank (computeShaped, checkDescriptor). So too for an array pointer
// component's, which no record's initial value or clearing gives either.
//
// An array pointer dummy's descriptor (dummyPointerCell) is the caller's,
// which the caller has seen has its header (pointerActual): a passed-length
// dummy's length is the actual's, which writing its own would lose.
func (l *lowerer) commonHeader(c *Cell) {
	if c.Arg != nil || !c.Located() && (c.Common == nil || c.Common.Module != "") {
		return
	}

	for _, f := range c.Desc.Fixed() {
		if f.Offset != PointerOffset {
			l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: f.Offset, T: Long}, Src: LongConst(int32(f.Value))})
		}
	}
}

// copyDescriptor copies the class NCA descriptor in src into dst, both
// describing arrays of the same type and rank, so their first longwords are
// the same already.
func (l *lowerer) copyDescriptor(dst, src *Cell) {
	for offset := PointerOffset; offset < dst.Desc.Size(); offset += Long.Size() {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: dst, Offset: offset, T: Long},
			Src: &Load{&Mem{Cell: src, Offset: offset, T: Long}}})
	}
}

// describeTarget fills the class NCA descriptor cell c in with a description
// of target, which is not a pointer, as the array pointer p's target: its
// elements p's type (sema has checked that they are, but for a CHARACTER
// length known only when the program runs, which p's declared length stands
// for).
func (l *lowerer) describeTarget(c *Cell, p *sema.Symbol, target sema.Expr) {
	el := ncaElement{t: p.Type}

	fill := func(_ ncaElement, pointer Expr, dims []ncaDim) *Mem {
		l.fillNCA(c, p, el, pointer, dims)

		return nil
	}

	switch t := target.(type) {
	case *sema.ArrayRef:
		pointer, dims := l.wholeDims(t.Sym)
		fill(el, pointer, dims)

	case *sema.Section:
		l.describeSection(t, nil, fill)

	case *sema.FieldRef:
		l.describeSection(t, nil, func(_ ncaElement, pointer Expr, dims []ncaDim) *Mem {
			// T%COORD keeps COORD's bounds; PATH%X, its rank from the
			// array of records, is a section, from 1 (as Fortran 95 says
			// plainly: 6.2.2).
			if t.Whole() && t.Ranked == len(t.Path) {
				dims = ownBounds(t, dims)
			}

			return fill(el, pointer, dims)
		})

	case *sema.Substring:
		switch x := t.X.(type) {
		case *sema.Section:
			l.describeSection(x, t, fill)
		case *sema.FieldRef:
			l.describeSection(x, t, fill)
		case *sema.ArrayRef:
			// Every element's substring: a section, its bounds from 1.
			pointer, dims := l.wholeDims(x.Sym)
			l.substringOf(x.Sym, t, &pointer)

			for i := range dims {
				dims[i].lower, dims[i].upper = LongConst(1), dims[i].extent
			}

			fill(el, pointer, dims)

		default:
			panic("ir: a pointer's target that is a substring of neither an array nor a section")
		}

	default:
		panic("ir: an array pointer's target that is neither an array nor a section")
	}
}

// ownBounds is dims, the dimensions describeSection gave the whole array
// component f (T%COORD), with the bounds the component was declared with,
// which it keeps, as a whole array does (the standard's 13.13.52, LBOUND).
func ownBounds(f *sema.FieldRef, dims []ncaDim) []ncaDim {
	for i, d := range f.RankedDims() {
		dims[i].lower = LongConst(int32(d.Lower))
		dims[i].upper = LongConst(int32(d.Upper))
	}

	return dims
}

// arrayAssociated is ASSOCIATED(P, T) for the array pointer p: whether P
// describes the very elements T is, in the same order. T, a pointer's own
// descriptor or a description of the variable made for the question, is
// compared field by field: the same first element, each dimension's extent
// and stride (the bounds don't matter: P => A(:N) is associated with A when N
// is A's upper bound), and some elements at all (the author's answer 4: a
// zero-sized target is never associated). P's POINTER may not be 0, so that
// two disassociated pointers aren't associated with each other.
func (l *lowerer) arrayAssociated(p *sema.Symbol, target sema.Expr) Expr {
	target = l.targetPointer(target)
	pc := l.adjustableOf(p)

	var tc *Cell
	if q := sema.PointerSymbol(target); q != nil {
		tc = l.adjustableOf(q)
	} else {
		tc = l.descriptorCell(pc.Desc.Clone())
		l.describeTarget(tc, p, target)
	}

	field := func(c *Cell, offset int) Expr {
		return &Load{&Mem{Cell: c, Offset: offset, T: Long}}
	}

	same := func(offset int) Expr {
		return &Cmp{Op: Eq, X: field(pc, offset), Y: field(tc, offset)}
	}

	result := arith(And, Long, same(PointerOffset), &Cmp{Op: Ne, X: field(pc, PointerOffset), Y: LongConst(0)})
	d := pc.Desc

	for i := range p.Dims {
		lower := d.LowerOffset(i)
		upper := lower + Long.Size()

		result = arith(And, Long, result, same(d.StrideOffset(i)))
		result = arith(And, Long, result, &Cmp{Op: Eq,
			X: arith(Sub, Long, field(pc, upper), field(pc, lower)),
			Y: arith(Sub, Long, field(tc, upper), field(tc, lower))})
	}

	return arith(And, Long, result, &Cmp{Op: Gt, X: field(tc, ArsizeOffset), Y: LongConst(0)})
}

// componentCell is the located cell (locate.go) that is the descriptor of the
// array pointer component s stands for (sema/pointercomp.go), in its record
// where s.Component says: at the place itself if the place has neither a
// computed address nor an index, else at the address computed now into a
// temporary of the statement's. Each use of the component makes its own, so
// that the record is found where the statement is when it is used: in an
// implied DO, (H(I)%V(1), I = 1, 3), a different one each time.
func (l *lowerer) componentCell(s *sema.Symbol) *Cell {
	return l.locatedCell(s.Name, l.ncaDescriptor(s), l.fieldPlace(s.Component, false))
}

// locatedCell is a located cell named name of the descriptor d, which is at
// place.
func (l *lowerer) locatedCell(name string, d *Descriptor, place *Mem) *Cell {
	c := &Cell{Name: name, Kind: DescriptorCell, T: Long, Size: d.Size(), Desc: d}

	if place.Ptr == nil && place.Index == nil && place.Param == nil {
		at := *place
		at.T = Long
		c.At, c.Common, c.Frame = &at, nil, place.Cell != nil && place.Cell.Frame

		return c
	}

	base := l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: base, Src: &Addr{place}})
	c.Base = base.Cell

	return c
}

// pointerActual is the actual argument x, a pointer, of a pointer dummy
// (sema.Arg.Pointer): the address of x's own storage, its longword or its
// descriptor, so the callee changes x itself (the author's answer 5). A
// CHARACTER pointer's descriptor and an array pointer's in COMMON or a record
// have their unchanging longwords written first, which nothing else need
// have given them, and from which a passed-length dummy takes its length and
// FTNRTL's ALLOCATE an array's element size and rank.
func (l *lowerer) pointerActual(x sema.Expr) Arg {
	if p := sema.PointerSymbol(x); p != nil {
		if p.ArrayPointer() {
			c := l.adjustableOf(p)
			l.commonHeader(c)

			return Arg{Ref: &Mem{Cell: c, T: Long, Volatile: p.Volatile}}
		}

		l.setPointerHeader(p)

		return Arg{Ref: l.pointerStorage(p)}
	}

	l.setHeader(x)

	return Arg{Ref: l.fieldPlace(x.(*sema.FieldRef), false)}
}

// Pointer results (subtask 6b; finding 9). A function whose value is a pointer
// returns an association, not a value: a scalar pointer's address in R0, a
// CHARACTER one's class S descriptor in R0 and R1, as a record of 8 bytes comes
// back (lowerresult.go), and an array pointer's class NCA descriptor through a
// hidden first argument, the address of a descriptor of the caller's. In the
// function the value is a pointer like any other: a scalar one a cell of its
// own, which the return loads whole (resultValue), and an array one the
// caller's descriptor, reached as an array pointer dummy's is (dummyPointerCell),
// so that =>, ALLOCATE, and NULLIFY of it fill the caller's descriptor in. Each
// entry point clears it first (clearPointerResult), so that a function that
// gives it no association returns a disassociated pointer.
//
//	FUNCTION FIRST(L)                 TYPE(NODE), POINTER :: FIRST
//	FIRST => L%HEAD                   MOVL   @FIRST.L(AP),FIRST.FIRST
//	END                               MOVL   FIRST.FIRST,R0
//	                                  RET
//
// In the caller each reference is a pointer of the statement's own, which the
// call fills in (pointerValue): a temporary longword or descriptor from R0 (and
// R1), or a descriptor cell passed as the hidden argument. The reference then
// stands for that pointer, and so for its target, as a pointer's name does:
// everything that reads a pointer's target reads it, and P => F(X) copies its
// association into P as P => Q copies Q's.

// pointerResult is the function's value if it is a pointer, else nil.
func (l *lowerer) pointerResult() *sema.Symbol {
	if l.r.Kind != Function || l.unit.Result == nil || !l.unit.Result.Pointer {
		return nil
	}

	return l.unit.Result
}

// clearPointerResult disassociates, at an entry point, the function's value if
// it is a pointer: an array pointer's descriptor, the caller's, given its
// unchanging longwords (its elements' length and type, its class, its rank),
// which a caller's descriptor of the statement's needn't have yet, and POINTER
// 0; a scalar one's address 0, and a CHARACTER one's header.
func (l *lowerer) clearPointerResult() {
	s := l.pointerResult()
	if s == nil {
		return
	}

	if s.ArrayPointer() {
		c := l.adjustableOf(s)

		for _, f := range c.Desc.Fixed() {
			if f.Offset != PointerOffset {
				l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: f.Offset, T: Long}, Src: LongConst(int32(f.Value))})
			}
		}

		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: PointerOffset, T: Long}, Src: LongConst(0)})

		return
	}

	l.setPointerHeader(s)
	l.emit(&Assign{origin: l.origin(), Dst: l.pointerAddress(s), Src: LongConst(0)})
}

// pointerValue is the pointer the reference f, to a function whose value is a
// pointer (sema.FuncRef.Pointer), gives: a pointer of the statement's own,
// which the function is called now to fill in, as a reference to the pointer
// (a Var, or for an array pointer an ArrayRef), which stands for its target.
// A scalar pointer is a temporary of the statement's, from R0, or for CHARACTER
// from R0 and R1.
func (l *lowerer) pointerValue(f *sema.FuncRef) sema.Expr {
	if len(f.S) > 0 {
		return l.pointerArray(f)
	}

	var cell *Mem

	if charPointer(f.T) {
		cell = l.textTemp(sema.StringDescriptorSize)
	} else {
		cell = l.temp(Long)
	}

	r := *cell
	r.T = Long
	if charPointer(f.T) {
		r.T = Quad
	}

	args, backs := l.args(f.Args)
	l.callWith(f, args, &r, backs)

	sym := sema.NewSymbol("$P", f.T)
	sym.Pointer = true
	l.cells[sym] = cell.Cell

	return sema.NewVar(sym, f.Pos)
}

// pointerArray is the array pointer the reference f, to a function whose value
// is an array pointer, gives: a class NCA descriptor of the statement's, whose
// address is the hidden first argument, which the function fills in. A
// statement calls each reference once, however many times its lowering asks for
// the value (its extents, then its elements).
func (l *lowerer) pointerArray(f *sema.FuncRef) *sema.ArrayRef {
	if ref := l.pointerValues[f]; ref != nil {
		return ref
	}

	sym := sema.NewSymbol("$P", f.T)
	sym.Class, sym.Pointer = sema.Array, true

	for range f.S {
		sym.Dims = append(sym.Dims, sema.Dim{Deferred: true, Lower: 1})
	}

	c := l.descriptorCell(l.ncaDescriptor(sym))

	if l.adjustable == nil {
		l.adjustable = map[*sema.Symbol]*Cell{}
	}

	l.adjustable[sym] = c

	args, backs := l.args(f.Args)
	l.callWith(f, append([]Arg{{Ref: &Mem{Cell: c, T: Long}}}, args...), nil, backs)

	ref := sema.NewArrayRef(sym, f.Pos)

	if l.pointerValues == nil {
		l.pointerValues = map[*sema.FuncRef]*sema.ArrayRef{}
	}

	l.pointerValues[f] = ref

	return ref
}

// targetPointer is the target of =>, ASSOCIATED, or a constructor's pointer
// component as the pointer it is, if it is a reference to a function whose
// value is a pointer (pointerValue), whose association is then the pointer's;
// else the target as it is.
func (l *lowerer) targetPointer(target sema.Expr) sema.Expr {
	if p, ok := l.pointerFunction(target); ok {
		return p
	}

	return target
}

// pointerFunction reports whether e is a reference to a function whose value is
// a pointer, and if so gives the pointer it stands for (pointerValue).
func (l *lowerer) pointerFunction(e sema.Expr) (sema.Expr, bool) {
	if f, ok := e.(*sema.FuncRef); ok && f.Pointer {
		return l.pointerValue(f), true
	}

	return nil, false
}
