package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// CHARACTER lengths that aren't constant (docs/planning/PHASE-104 - Fortran
// 90 dynamic arrays.md, finding 7; sema/charlength.go). In
//
//	SUBROUTINE PAD(S, N)
//	CHARACTER(LEN=N) :: S, T
//
// the lengths of S and T are N's value at entry, and stay that way whatever N
// is set to later, as an adjustable array's bounds do (lowerbounds.go). Each is
// computed at each entry point whose list defines it, and kept:
//
//   - S, a dummy argument, keeps its text where its actual argument's is (its
//     descriptor's POINTER, as for any CHARACTER dummy), and its length in a cell
//     of its own, $L1, a longword: not the actual's, which may be longer;
//   - T, the subprogram's own, is an automatic object, whose own cell is a class
//     S descriptor of its text: the length in its first word and the address of
//     storage made on the stack (StackAllocate, lowerautomatic.go), so that it is
//     passed as an argument as it stands. An automatic array of such a length
//     keeps the length in its class A descriptor's first word, an element's
//     length, which is where FTNRTL and a callee look for it too.
//
// A length less than 0 is 0 (the Fortran 90 standard's 5.1.1.5), and a length
// more than 65,535, which a descriptor's word can't hold, isn't checked.

// prepareLength makes the cell of s's CHARACTER length if s is a dummy argument
// whose length isn't constant, and reports whether it did.
func (l *lowerer) prepareLength(s *sema.Symbol) bool {
	if s.LengthValue == nil || s.Dummy == 0 || s.ZeroLength() {
		return false
	}

	if l.lengths == nil {
		l.lengths = map[*sema.Symbol]*Cell{}
	}

	c := l.newCell("$L"+strconv.Itoa(len(l.lengths)+1), LengthCell, Long, Long.Size())
	c.Dummy = s.Name
	l.lengths[s] = c

	return true
}

// lengthOf is the length cell of the dummy argument s, the routine's own or its
// host's, or nil if s has none.
func (l *lowerer) lengthOf(s *sema.Symbol) *Cell {
	c := l.lengths[s]
	if c != nil || l.host == nil {
		return c
	}

	return l.reach(l.host.lengthOf(s))
}

// computeLength stores the dummy argument s's length in its cell c, at an entry
// point.
func (l *lowerer) computeLength(s *sema.Symbol, c *Cell) {
	if l.from == nil {
		l.from = l.unit.AST.Header

		defer func() { l.from = nil }()
	}

	m := &Mem{Cell: c, T: Long}
	l.emit(&Assign{origin: l.origin(), Dst: m, Src: l.expr(s.LengthValue)})
	l.notNegative(m)
	l.release()
}

// runtimeLength is the length, a Long, of s, whose length isn't constant: its
// cell's, or an automatic object's descriptor's first word.
func (l *lowerer) runtimeLength(s *sema.Symbol) Expr {
	if c := l.lengthOf(s); c != nil {
		return &Load{&Mem{Cell: c, T: Long}}
	}

	return &Op{Zext, Long, &Load{&Mem{Cell: l.adjustableOf(s), T: Word}}, nil}
}

// lengthWord is the length of s as a Word, for a descriptor's first word: a
// passed length's from its own descriptor, an automatic object's from its, a
// dummy argument's cell's converted, or a constant.
func (l *lowerer) lengthWord(s *sema.Symbol) Expr {
	switch {
	case s.PassedLength():
		return &Load{descriptorField(l.paramOf(s), 0, Word)}
	case s.ZeroLength():
		return IntConst(Word, 0)
	case s.LengthValue != nil && l.lengthOf(s) == nil:
		return &Load{&Mem{Cell: l.adjustableOf(s), T: Word}}
	}

	return arith(Cvt, Word, l.charLength(s), nil)
}
