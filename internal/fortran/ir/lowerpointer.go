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

// charPointerHeader is a CHARACTER pointer's descriptor's first longword for a
// target of length n: the length, DSC$K_DTYPE_T, and DSC$K_CLASS_S.
func charPointerHeader(n int) Expr {
	return LongConst(int32(n | int(DTypeT)<<16 | int(ClassS)<<24))
}

// pointerAddress is the longword of the pointer s that holds its target's
// address: its cell, or a CHARACTER pointer's descriptor's POINTER.
func (l *lowerer) pointerAddress(s *sema.Symbol) *Mem {
	m := &Mem{Cell: l.cellOf(s), T: Long, Volatile: s.Volatile}

	if s.Type.Kind == sema.Character {
		m.Offset = PointerOffset
	}

	return m
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
	if sema.IsPointer(target) {
		return &Load{l.pointerAddress(target.(*sema.Var).Sym)}
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
	p := s.Pointer.(*sema.Var).Sym

	l.setPointerHeader(p)
	l.emit(&Assign{origin: l.origin(), Dst: l.pointerAddress(p), Src: l.targetAddress(s.Target)})
}

// setPointerHeader writes the CHARACTER pointer p's descriptor's first
// longword, its length and type; nothing for any other pointer.
func (l *lowerer) setPointerHeader(p *sema.Symbol) {
	if p.Type.Kind != sema.Character {
		return
	}

	l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: l.cellOf(p), T: Long, Volatile: p.Volatile},
		Src: charPointerHeader(p.Type.Size)})
}

// nullify is NULLIFY: each pointer's address 0.
func (l *lowerer) nullify(s *sema.Nullify) {
	for _, x := range s.Pointers {
		p := x.(*sema.Var).Sym

		l.setPointerHeader(p)
		l.emit(&Assign{origin: l.origin(), Dst: l.pointerAddress(p), Src: LongConst(0)})
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
	p := e.Pointer.(*sema.Var).Sym
	address := &Load{l.pointerAddress(p)}

	if e.Target == nil {
		return &Cmp{Op: Ne, X: address, Y: LongConst(0)}
	}

	same := &Cmp{Op: Eq, X: address, Y: l.targetAddress(e.Target)}

	switch {
	case sema.IsPointer(e.Target):
		return arith(And, Long, same, &Cmp{Op: Ne, X: &Load{l.pointerAddress(p)}, Y: LongConst(0)})

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
func (l *lowerer) clearPointers() {
	for _, c := range l.r.Cells {
		if c.Frame && c.Sym != nil && c.Sym.Pointer && c.Kind == VariableCell {
			l.setPointerHeader(c.Sym)
			l.emit(&Assign{origin: l.origin(), Dst: l.pointerAddress(c.Sym), Src: LongConst(0)})
		}
	}
}
