package sema

// layout gives each of the unit's variables and arrays its place in the
// unit's static storage, $LOCAL (Decision 10 of docs/PHASE-74 - FORTRAN
// plan.md): in order of first appearance, each on its type's natural
// boundary (Type.Align: a longword's on a multiple of 4, a LOGICAL*1 or
// a CHARACTER variable anywhere). The size is rounded up to a longword,
// so that the compiler's own cells, which follow, are aligned.
// Storage is static because FORTRAN-77's routines aren't recursive
// (section 6.2), so a routine's variables are never live in two calls at
// once, and a variable keeps its value from one call to the next as VAX
// FORTRAN's do.
//
// A dummy argument has no storage: it is the caller's, reached through
// the argument list. Nor has a routine's name. A name only declared,
// never used, is a variable, given storage as VAX FORTRAN gives it.
func (a *analyzer) layout() {
	offset := 0

	for _, s := range a.unit.Symbols {
		if s.Class == Unclassified {
			s.Class = Variable
		}

		if s.Dummy != 0 || (s.Class != Variable && s.Class != Array) {
			continue
		}

		align := s.Type.Align()
		s.Offset = (offset + align - 1) / align * align
		offset = s.Offset + s.Size()
	}

	a.unit.LocalSize = (offset + 3) &^ 3
}
