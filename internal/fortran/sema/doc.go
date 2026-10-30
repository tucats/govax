// Package sema is the FORTRAN compiler's fourth pass, semantic analysis:
// it reads the parser's syntax tree (package ast) and works out what the
// program means, reporting what is wrong with it (docs/PHASE-75 -
// FORTRAN first light.md, Part B, and Decision 7 of docs/PHASE-74 -
// FORTRAN plan.md).
//
// # What the tree doesn't say
//
// The parser recorded what each statement says, not what it means. In
//
//	      SUBROUTINE SUMUP(T, N, S)
//	      INTEGER N, T(10)
//	      S = 0
//	      DO 100 K = 1, N
//	  100 S = S + T(K)
//	      END
//
// the tree doesn't know that S is REAL (no declaration says so: its
// first letter does, section 2.2.2.2), that T(K) is an element of an
// array rather than a call of a function T (only the INTEGER statement's
// declarator says so), that S + T(K) adds a REAL and an INTEGER and so
// must convert T(K) first, that 0 must become 0.0 to be stored in S, or
// that the statement labeled 100 is the last of the DO loop's body. This
// pass answers all of those, and checks the rules the grammar can't
// express: a label defined once, a loop's terminal statement after it,
// no jump into an IF block, declarations before executable statements.
//
// It works a unit at a time (each FUNCTION, SUBROUTINE, and main program
// has its own names), in two walks over the unit's statements.
//
// # The symbol table
//
// The first walk reads the declarations: the type statements and
// DIMENSION. Every name the unit uses gets a Symbol, with its type
// (declared, or implicit from its first letter), its dimensions if it is
// an array, and its position among the dummy arguments if it is one.
// What a name *is* (its Class) comes from how it is used, since FORTRAN
// has no declaration for most of that: REAL F declares only a type, and
// F becomes a variable when the unit uses F alone, or an external
// function when it uses F(X) without dimensioning F. A name used both
// ways is an error (BADUSE).
//
// Doing the declarations first means every statement sees every
// declaration, which is what FORTRAN requires anyway: Figure 1-1 puts
// the declarations before the executable statements, and a declaration
// after one is an error (LATEDECL).
//
// # Expressions get their types
//
// The second walk reads the executable statements, and with them their
// expressions. Each becomes a typed tree (Expr), each node knowing its
// type, by section 2.3.1.2's rule: an operation on two operands of
// different types is done in the type of higher rank (LOGICAL, then
// INTEGER, then REAL), the other operand converted first. The
// conversion is made an explicit node, so the code generator never has
// to work out where one is needed:
//
//	Y = (I/J)*X     is   Y = (*:R (REAL (/:I I J)) X)
//
// The division is an INTEGER one (I/J truncates), and only its result
// is converted, as the manual's own example of this statement says. An
// assignment converts its value to its target's type the same way
// (Table 3-1): I = X truncates, with a Convert to INTEGER.
//
// An operation on constants is done here (constant folding), with
// package vaxfloat for REAL values, which rounds exactly as the VAX's
// floating instructions do; so a folded expression has the value the
// program would have computed. 1.0/3 becomes the constant 0.33333334,
// whose F_floating longword is ^XAAAB3FAA.
//
// # Blocks and loops
//
// The parser left each unit a flat list of statements. Here the IF
// blocks and DO loops become structured statements, an If holding each
// arm's statements and a Do holding its body, so that later passes
// don't match blocks again. While it walks the list, the pass keeps a
// stack of the constructs open at the current statement: IF THEN pushes
// one, END IF pops it; a DO pushes a loop, and the statement with the
// loop's label pops it, with any other loops that end at the same
// statement (nested loops may share their terminal statement, section
// 5.3.1.2). The stack is also how a construct that doesn't nest is
// found: a DO loop whose terminal statement is inside an IF block that
// started inside the loop, or an ELSE with a loop still open in the
// block before it (DONEST).
//
// # Labels
//
// Every statement records the constructs open at it. A jump (GO TO) to a
// label is checked against the target's list once every label is known:
// it may not enter an IF block from outside, nor go from one block of a
// construct to another (section 5.7.3.1); it may jump to END IF only
// from the block just before it; and only the innermost of the loops
// sharing a terminal statement may jump to it (section 5.3.1.3). A jump
// into a DO loop from outside gets no message: VAX FORTRAN allows one
// from the loop's *extended range* (section 5.3.1.4), code the loop
// jumped out to, which jumps back in.
//
// # Storage
//
// Last, each variable and array gets its place in the unit's static
// storage, the $LOCAL program section: in order of first appearance, on
// longword boundaries. A dummy argument has no storage of its own; it is
// the caller's, reached through the argument list.
//
// # Errors
//
// A mistake is reported once. An expression with an error has the
// Invalid type, and an operation on an Invalid operand is Invalid too,
// without another message. A name whose declaration had an error (a
// bound that isn't a constant, a type this phase doesn't compile) is
// marked, and its uses get no messages of their own. A statement the
// parser rejected still keeps its place in the blocks (an IF THEN with
// a syntax error still opens a block, so its END IF isn't "not in an IF
// block"), and its label, so a jump to it isn't "never defined".
//
// Some declarations can't be read yet at all: COMMON, PARAMETER,
// IMPLICIT, and the rest are NOTIMPL until Phase 76. Without them, most
// of what this pass would say about a unit's names would be wrong (an
// array declared in COMMON would be "not an array" at every use), so a
// unit with one gets only the messages about its structure: labels,
// blocks, loops, and statement order. A unit whose header the parser
// rejected is left out entirely.
//
// # The dump
//
// Dump writes the result for the golden tests in testdata: each unit's
// symbols with their storage, then its statements, indented by the
// blocks they are in, their expressions as prefix lists with each
// operator's type:
//
//	== SUBROUTINE SUMUP
//	  SUMUP                program unit
//	  T         INTEGER    array(1:10)      argument 1
//	  N         INTEGER    variable         argument 2
//	  S         REAL       variable         argument 3
//	  K         INTEGER    variable         $LOCAL+0
//	  $LOCAL is 4 bytes
//	3           S = 0.0
//	4           DO 100 K = 1, N, 1
//	5     100     S = (+:R S (REAL T[K]))
//	6           END
package sema
