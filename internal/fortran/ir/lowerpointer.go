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

// charPointerHeader is a CHARACTER pointer's descriptor's first longword for a
// target of length n: the length, DSC$K_DTYPE_T, and DSC$K_CLASS_S.
func charPointerHeader(n int) Expr {
	return LongConst(int32(n | int(DTypeT)<<16 | int(ClassS)<<24))
}

// pointerAddress is the longword of the pointer s that holds its target's
// address: its cell, or a CHARACTER pointer's descriptor's POINTER.
func (l *lowerer) pointerAddress(s *sema.Symbol) *Mem {
	if s.ArrayPointer() {
		return &Mem{Cell: l.adjustableOf(s), Offset: PointerOffset, T: Long, Volatile: s.Volatile}
	}

	m := &Mem{Cell: l.cellOf(s), T: Long, Volatile: s.Volatile}

	if s.Type.Kind == sema.Character {
		m.Offset = PointerOffset
	}

	return m
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
func (l *lowerer) setPointerHeader(p *sema.Symbol) {
	if p.Type.Kind != sema.Character || p.ArrayPointer() {
		return
	}

	l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: l.cellOf(p), T: Long, Volatile: p.Volatile},
		Src: charPointerHeader(p.Type.Size)})
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

	if p := sema.PointerSymbol(e.Pointer); p != nil && p.ArrayPointer() {
		return l.arrayAssociated(p, e.Target)
	}

	same := &Cmp{Op: Eq, X: address, Y: l.targetAddress(e.Target)}

	switch {
	case sema.IsPointer(e.Target):
		return arith(And, Long, same, &Cmp{Op: Ne, X: &Load{l.pointerOf(e.Pointer)}, Y: LongConst(0)})

	case e.Target.Type().Kind == sema.Character && e.Target.Type().Size == 0:
		length := l.chars(e.Target).Len

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

// arrayPointerAssign is P => target for the array pointer p: target's
// descriptor copied, if it is a pointer; else target described into p's cell,
// a whole array with its own bounds, a section (or a component with a rank)
// with bounds from 1 (5.1.2.4.3). A target that reads p itself, P => P(N:1:-1),
// is described into a cell of the statement's first, and copied: describing it
// in place would change the bounds and strides it is read through.
func (l *lowerer) arrayPointerAssign(p *sema.Symbol, target sema.Expr) {
	cell := l.adjustableOf(p)
	l.commonHeader(cell)

	if q := sema.PointerSymbol(target); q != nil {
		l.copyDescriptor(cell, l.adjustableOf(q))

		return
	}

	if sema.TargetBase(target) != p {
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
// rank (computeShaped, checkDescriptor).
func (l *lowerer) commonHeader(c *Cell) {
	if c.Common == nil || c.Common.Module != "" {
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
