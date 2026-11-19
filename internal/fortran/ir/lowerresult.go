package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Functions of a derived type (docs/planning/PHASE-105 - Fortran 90 derived
// types.md, finding 11). The Calling Standard returns any value up to a
// quadword in R0, or R0 and R1, and a derived type's value is no exception:
// TYPE(PAIR) FUNCTION F, of two INTEGER*2 components, returns its 4 bytes in
// R0, as an INTEGER*4 function would. The function's value is a cell of its
// own, rounded up to a longword or a quadword (sema's RecordRoom), whose
// components the function assigns as any record's; its return loads the cell
// whole, MOVL or MOVQ, so R0's or R1's bytes past the type's are whatever the
// cell held there. The caller stores R0 (and R1) into a temporary of the same
// room and uses the temporary's first bytes as the record, or, when the type's
// size is that of a byte, word, longword, or quadword and the value is
// assigned, stores the registers straight into the target. Past 8 bytes the
// value goes through a hidden first argument, the address of a temporary of
// the type, as a COMPLEX*16's does.
//
// So records still move only through memory: the registers are crossed only
// at the call and the return, as a COMPLEX*8's are.

// resultType is the machine type in which a function of type t returns its
// value: typeOf's, but for a derived type's value that fits in R0 and R1, a
// longword or a quadword.
func (l *lowerer) resultType(t sema.Type) Type {
	if t.Kind == sema.Record && !t.HiddenResult() {
		return wholeType(sema.RecordRoom(t))
	}

	return l.typeOf(t)
}

// wholeType is the machine type that moves n bytes, 1, 2, 4, or 8, in one
// instruction.
func wholeType(n int) Type {
	switch n {
	case 1:
		return Byte
	case 2:
		return Word
	case 4:
		return Long
	}

	return Quad
}

// valueType is the machine type in which the function's name (or an entry
// point's) s returns its value: resultType's, but for a pointer its
// association (lowerpointer.go's pointer results), a scalar's address, a
// longword, or a CHARACTER one's descriptor, a quadword.
func (l *lowerer) valueType(s *sema.Symbol) Type {
	switch {
	case !s.Pointer:
		return l.resultType(s.Type)
	case charPointer(s.Type):
		return Quad
	}

	return Long
}

// resultValue is the value the function returns for its name (or an entry
// point's) s, which goes in R0 and R1: the variable, loaded whole as
// valueType says, or a pointer's own storage.
func (l *lowerer) resultValue(s *sema.Symbol) Expr {
	m := l.symbolMem(s)
	if s.Pointer {
		m = l.pointerStorage(s)
	}

	m.T = l.valueType(s)

	return &Load{m}
}

// recordResult calls the function of a derived type f refers to, and returns
// the place of its value: a temporary, in which the function puts it through
// its hidden argument, or which the caller fills from R0 and R1.
func (l *lowerer) recordResult(f *sema.FuncRef) *Mem {
	if f.T.HiddenResult() {
		t := l.textTemp(f.T.Size)
		l.callFunction(f, t)

		return t
	}

	t := l.textTemp(sema.RecordRoom(f.T))
	r := *t
	r.T = l.resultType(f.T)
	l.callFunction(f, &r)

	return t
}

// assignResult lowers P = F(...), the value of a function of a derived type
// assigned, and reports whether it could store R0 (and R1) straight into P:
// only when the type is as long as a byte, word, longword, or quadword, so that
// one instruction stores exactly its bytes.
func (l *lowerer) assignResult(target sema.Expr, f *sema.FuncRef) bool {
	switch f.T.Size {
	case 1, 2, 4, 8:
	default:
		return false
	}

	// recordPlace takes away an index, which the VAX would multiply by the
	// size of the store's type, not the record's.
	dst := *l.recordPlace(l.mem(target))
	dst.T = wholeType(f.T.Size)
	l.callFunction(f, &dst)

	return true
}
