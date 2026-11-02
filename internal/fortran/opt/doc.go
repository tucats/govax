// Package opt is the FORTRAN compiler's optimizer: what /OPTIMIZE adds between
// lowering and the code generator (docs/PHASE-84 - FORTRAN optimizer.md).
//
// # What an optimizer may do
//
// An optimizer rewrites a program so that it runs in fewer instructions, or
// touches memory less, without changing what the program does. "What the
// program does" is the whole difficulty. Take
//
//	      DO 10 I = 1, N
//	         A(I) = B(J)*C
//	   10 CONTINUE
//
// B(J)*C looks the same on every trip, so computing it once before the loop
// looks like a free gain. But if N is 0 the loop never runs, and if C were
// a reserved floating operand the rewritten program would fault where the
// original didn't. And it isn't the same on every trip at all if J is in a
// COMMON block that A's elements overlap by EQUIVALENCE. Each pass here has to
// know such rules, and the phase doc's Decisions list them.
//
// # Passes
//
// The optimizer is a list of passes, Passes, run in order on each routine. A
// pass is a function from a routine in the intermediate representation
// (package ir) to the same routine improved, each in a file of its own whose
// comment teaches its technique on a small FORTRAN example. A pass changes the
// routine in place; it needs nothing from the pass before it but a correct
// routine, which is why ir.Check runs after each one, so that a pass that
// breaks a routine is named at once, rather than being found later as a
// program that prints the wrong thing.
//
// The list starts empty, and each subtask of the phase adds to it, so for now
// /OPTIMIZE does nothing but say so in the MACRO file's header.
//
// # How it is tested
//
// A pass's own tests look at what it does to small routines. What says it is
// right is that every program of the compiler's corpus runs both ways, with
// /OPTIMIZE and /NOOPTIMIZE, and must print the same
// (internal/console/fortranprograms_test.go, TestFortranPrograms); some of those
// programs (optalias.for, optfaults.for, optflow.for) are there to catch an
// optimizer that breaks the rules above. Without /OPTIMIZE this package isn't
// run at all, and the compiler's output is what it always was.
package opt
