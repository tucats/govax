package ir

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/sema"
)

// /CHECK=POINTERS (docs/planning/PHASE-110 - Fortran 90 compiler
// usability.md, finding 7): a pointer's target, or an allocatable array's
// element, is reached only once its address has been found not to be 0.
//
// A scalar pointer's cell holds its target's address, and an array pointer's or
// an allocatable array's descriptor its first element's in POINTER; NULLIFY,
// DEALLOCATE, and an entry point's clearing leave 0 there (Phase 107: a pointer
// never assigned is a nullified one). So the test is of that longword, and its
// failure a call of FTNRTL's FTN$NOTASSOC, for a pointer (govax's FOR number
// 311), or FTN$UNALLOCATED, for an allocatable array (312), each a severe
// condition that doesn't come back.
//
// The test is an ir.Guard inside the expression that reaches the place, as
// /CHECK=BOUNDS's InRange is inside a subscript (lowerchecks.go), and for the
// same reason: a program like
//
//	IF (ASSOCIATED (P) .AND. P%KEY .GT. 0) ...
//	DO WHILE (ASSOCIATED (Q) .AND. Q%KEY .LT. K)
//
// counts on the branch's .AND. keeping it from P%KEY when P is disassociated,
// which the code generator's short circuit does. A test ahead of the statement
// would fail there. So
//
//	INTEGER, POINTER :: P
//	P = 5                 TSTL    PROG.P
//	                      BNEQ    PROG.$5
//	                      CALLS   #0,G^FTN$NOTASSOC
//	                      PROG.$5:
//	                      MOVL    #5,@PROG.P
//
// and an allocatable array's element tests POINTER before the element's address,
// computed from A0, is used. What isn't tested: a pointer whose target has gone
// (deallocated through another pointer, or a frame's variable after its routine
// returned), which nothing in the pointer says; an assumed-shape dummy, whose
// caller's actual was tested; and a pointer itself, as => and NULLIFY, ALLOCATE
// and DEALLOCATE, ASSOCIATED and ALLOCATED, and a pointer dummy's actual use it.

// FTNRTL's routines for /CHECK=POINTERS (mar/ftn_control.mar).
const (
	ftnNotAssoc    = "FTN$NOTASSOC"
	ftnUnallocated = "FTN$UNALLOCATED"
)

// guardedAddress is the address x tested by routine under /CHECK=POINTERS: an
// ir.Guard of x, testing test first (nil, or the same as x: x itself); x itself
// if the unit doesn't ask for the check.
func (l *lowerer) guardedAddress(x, test Expr, routine string) Expr {
	if !l.unit.Checks.Pointers || routine == "" {
		return x
	}

	if test != nil && ExprText(test) == ExprText(x) {
		test = nil
	}

	return &Guard{X: x, Test: test, Routine: routine}
}

// arrayGuard is the routine that /CHECK=POINTERS calls if the array s's
// descriptor's POINTER is 0: FTN$NOTASSOC for an array pointer, FTN$UNALLOCATED
// for an allocatable array, and "" for any other array (an assumed-shape dummy,
// an automatic object), or one of the compiler's own (an array-valued function's
// value, always there).
func arrayGuard(s *sema.Symbol) string {
	switch {
	case strings.HasPrefix(s.Name, "$"):
		return ""
	case s.ArrayPointer():
		return ftnNotAssoc
	case s.Allocatable:
		return ftnUnallocated
	}

	return ""
}

// guardedElement is the address x, an element's of the array s whose
// descriptor is c (or one computed from its A0), tested by its POINTER
// (arrayGuard).
func (l *lowerer) guardedElement(x Expr, s *sema.Symbol, c *Cell) Expr {
	routine := arrayGuard(s)
	if routine == "" || c == nil || !l.unit.Checks.Pointers {
		return x
	}

	return l.guardedAddress(x, &Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}, routine)
}

// checkDescribed writes, ahead of what the statement does with the whole array
// s, a test of its descriptor's POINTER (arrayGuard): an allocatable array or
// array pointer passed by its own descriptor, which nothing else reads first. A
// call's actual arguments are computed ahead of the statement anyway.
func (l *lowerer) checkDescribed(s *sema.Symbol, c *Cell) {
	routine := arrayGuard(s)
	if routine == "" || c == nil || !l.unit.Checks.Pointers {
		return
	}

	pointer := &Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}
	scratch := l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: scratch, Src: &Guard{X: pointer, Routine: routine}})
}

// guardedInquiry is the value v of SIZE, LBOUND, or UBOUND of f's array,
// computed ahead of the statement from its descriptor, tested where it is used
// when the array is a whole allocatable array or array pointer: its bounds mean
// nothing while it isn't there, and the test must come where the value would be
// read, after an ALLOCATED or ASSOCIATED that keeps the program from it.
func (l *lowerer) guardedInquiry(f *sema.ArrayFunc, v Expr) Expr {
	r, ok := f.Array.(*sema.ArrayRef)
	if !ok || !l.unit.Checks.Pointers {
		return v
	}

	routine := arrayGuard(r.Sym)
	c := l.adjustableOf(r.Sym)

	if routine == "" || c == nil {
		return v
	}

	return &Guard{X: v, Test: &Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}, Routine: routine}
}
