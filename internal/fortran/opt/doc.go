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
// Each subtask of the phase adds to the list. OneCharacter (onechar.go) makes a string
// of one character the byte it is; Forward (forward.go) uses what a store
// said in place of reading it back, and settles branches that come out constant;
// Unreachable (unreachable.go) takes out what can't run after that; DeadCode
// (deadcode.go) takes out the stores nothing reads; DeferredIndexes (deferred.go) reaches
// an adjustable array's element through its A0 where that is; LoopInvariants
// (invariant.go) computes before a loop what every trip computes the same;
// CommonSubexpressions (cse.go) computes once what a block computes several times, and
// finds a place once for an operation on it; Registers (registers.go), last, keeps in R6 to
// R11 what the loops use most.
//
// # Analyses
//
// What a pass needs to know of a routine comes from four files, none of which
// changes anything. graph.go is the control flow graph, with every way into the
// routine, and its dominators. loops.go is the natural loops, found from the graph
// and not from DO statements. places.go says which bytes of storage a Mem can be,
// and whether two places conflict. effects.go says what a statement reads and
// writes, and what a call may change that the statement doesn't name. A Graph and a
// Storage are each a picture of a routine: a pass that changes it makes new ones.
//
// A second list, CodePasses, works on a routine's instructions rather than its
// IR, once the code generator has chosen them and before they are written
// (codegen.Code, an instruction list): that is the only place to ask how far a
// branch goes. Peephole and Branches are these. Generate calls them through
// Improve, and Code.Check runs after each, as ir.Check does for the others.
//
// # How it is tested
//
// A pass's own tests look at what it does to small routines. What says it is
// right is that every program of the compiler's corpus runs both ways, with
// /OPTIMIZE and /NOOPTIMIZE, and must print the same
// (internal/console/fortranprograms_test.go, TestFortranPrograms); some of those
// programs (optalias.for, optasts.for, optfaults.for, optflow.for, optbranch.for, optforward.for, optcse.for, optinvar.for, optchar.for, optreg.for) are there to
// catch an optimizer that breaks the rules above. Without /OPTIMIZE this package isn't
// run at all, and the compiler's output is what it always was.
package opt
