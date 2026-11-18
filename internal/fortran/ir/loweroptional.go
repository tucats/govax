package ir

import "github.com/tucats/govax/internal/fortran/sema"

// OPTIONAL dummy arguments and PRESENT (the Fortran 90 standard's 5.1.2.6 and
// 13.13.80; docs/planning/PHASE-103 - Fortran 90 procedures and
// interfaces.md, finding 9 and the author's answer 4).
//
// An argument left out of a call is VMS's null argument: its longword in the
// argument list is 0, where an argument given holds the address of the actual
// argument. A call govax compiles always passes the whole list, a 0 for each
// argument left out, so that longword is always there. But VMS's own convention
// also lets a caller pass a shorter list, leaving the last arguments out
// altogether (the LIB$ routines read a missing trailing argument as absent), and
// a MACRO program or an old FORTRAN one may call a Fortran 90 routine so. Then
// the longword at 4*n(AP) isn't the routine's at all, and only the argument
// count, the byte at (AP), says so.
//
// So each OPTIONAL dummy is reached through a cell, the mechanism of ENTRY
// statements (lowerentry.go): the routine's entry copies the dummy's longword
// there if the count says the caller passed it, and 0 if not (the code
// generator's writeEntryArgs):
//
//	CLRL    SUB.$A2                 ; TOL is argument 2, if it is there
//	CMPB    (AP),#2
//	BLSSU   SUB.$1
//	MOVL    8(AP),SUB.$A2
//	SUB.$1:
//
// Everything that reaches the dummy goes through the cell, so the routine sees
// one answer however it was called: PRESENT(TOL) is a test of the cell, and
// passing TOL on to another OPTIONAL dummy passes the cell's 0, so that dummy is
// absent too. An internal procedure asking about its host's OPTIONAL dummy tests
// the host's cell (lowerinternal.go).

// prepareOptionals gives each OPTIONAL dummy argument of a routine without
// ENTRY statements its cell; prepareEntries gives every dummy one in a routine
// with them.
func (l *lowerer) prepareOptionals() {
	for i, p := range l.r.Params {
		if p.Sym != nil && p.Sym.Optional && p.Through == nil {
			l.throughCell(i, p)
		}
	}
}

// present is PRESENT(A): whether the cell of the OPTIONAL dummy A, the
// routine's own or its host's, holds an address.
func (l *lowerer) present(e *sema.Present) Expr {
	p := l.paramOf(e.Sym)
	if p == nil || p.Through == nil {
		panic("ir: PRESENT of " + e.Sym.Name + ", which isn't an OPTIONAL dummy argument with a cell")
	}

	return &Cmp{Op: Ne, X: &ArgPointer{p}, Y: LongConst(0)}
}
