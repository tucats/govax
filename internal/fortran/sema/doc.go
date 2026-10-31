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
// Before anything else, the unit's IMPLICIT statements say what type
// each first letter gives a name (implicit.go): by default INTEGER for I
// to N and REAL for the rest, or none at all with IMPLICIT NONE. Then
// the first walk reads the declarations: the type statements, of every
// type and length (Table 2-1: INTEGER*2, REAL*8, COMPLEX*16,
// CHARACTER*n, ...), and DIMENSION. Every name the unit uses gets a
// Symbol, with its type (declared, or implicit from its first letter),
// its dimensions if it is an array, and its position among the dummy
// arguments if it is one.
// What a name *is* (its Class) comes from how it is used, since FORTRAN
// has no declaration for most of that: REAL F declares only a type, and
// F becomes a variable when the unit uses F alone, or an external
// function when it uses F(X) without dimensioning F. A name used both
// ways is an error (BADUSE).
//
// Doing the declarations first means every statement sees every
// declaration, which is what FORTRAN requires anyway: Figure 1-1 puts
// the declarations before the executable statements, and a declaration
// after one is an error (LATEDECL). The same walk checks the rest of
// Figure 1-1's order (order.go: IMPLICIT before the other declarations,
// statement functions after them, ORDER), and finds the statement
// function definitions, F(X) = ... before the first executable
// statement with F not an array, which only the declarations tell from
// assignments to array elements.
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
// PARAMETER's values, array bounds, and CHARACTER lengths are constant
// expressions in every type (constant.go, constfuncs.go, parameter.go):
// INTEGER, REAL*4, REAL*8, REAL*16, COMPLEX, LOGICAL, and CHARACTER, with
// ** of an INTEGER exponent and the intrinsic functions section 4.11
// lists. Writing a constant's name is writing its value. An executable
// statement's expression comes to the same evaluator only for the forms
// it compiles only as constants (2**10, MAX(1, 4), 'A'//'B').
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
// Once the declarations are read, the names in COMMON statements get
// their places in their blocks, one after another with nothing between
// them (common.go), and the EQUIVALENCE statements tie names into groups
// that are placed together, a group with a name in a block going in that
// block, perhaps lengthening it (equivalence.go; its comment works an
// example through).
//
// Last, each other variable and array gets its place in the unit's static
// storage, the $LOCAL program section: in order of first appearance,
// each on its type's natural boundary (a longword on a multiple of 4, a
// LOGICAL*1 anywhere). A dummy argument has no storage of its own; it is
// the caller's, reached through the argument list.
//
// A name may have any type, but expressions are compiled only on the types
// Type.Compiled says; a name of any other type is NOTIMPL where an
// expression first uses it (Phase 78's), and only declaring it costs nothing
// but its storage.
//
// # Errors
//
// A mistake is reported once. An expression with an error has the
// Invalid type, and an operation on an Invalid operand is Invalid too,
// without another message. A name whose declaration had an error (a
// bound that isn't a constant, a length its type doesn't have) is
// marked, and its uses get no messages of their own. A statement the
// parser rejected still keeps its place in the blocks (an IF THEN with
// a syntax error still opens a block, so its END IF isn't "not in an IF
// block"), and its label, so a jump to it isn't "never defined".
//
// # Not implemented yet
//
// The parser builds a node for every statement and form of VAX FORTRAN,
// and this pass is where one it can't compile yet is reported, as it
// reaches it (notimpl.go): a statement it doesn't handle (OPEN, REWRITE,
// VOLATILE) is "the OPEN statement is not implemented yet" at the
// statement, keeping its place in the blocks and its label; a form
// inside a statement it does handle (a Hollerith constant in an expression,
// a variable format expression) is reported where it is analyzed. Each
// later phase moves statements and forms from one side to the other.
//
// Some declarations can't be read yet at all: VOLATILE, RECORD, and
// the rest are NOTIMPL until later phases.
// Without them, most of what this pass would say about a unit's names
// would be wrong (a name declared in a RECORD statement would be "not a
// record" at every use), so a unit with one gets only the messages about its
// structure (labels, blocks, loops, and statement order) and the
// NOTIMPLs of what it wrote (notImplForm). A unit whose header the
// parser rejected is left out entirely.
//
// # Initial values
//
// A DATA statement's values, and a type statement's in slashes, make no
// code: each is an Initial, an item's place and its bytes as the VAX holds
// them, which the code generator writes in the MACRO file (data.go). A
// BLOCK DATA unit is a unit with only these and its declarations.
//
// An ENTRY statement (entry.go) makes an Entry, with a dummy list of its own.
// Its names are entered before the unit's declarations, and it stands in the
// body as an EntryPoint, where its code begins. A function's entry points'
// names are Result names, sharing one place.
//
// An alternate return's * (altreturn.go) is a place in a dummy list, no dummy,
// and a CALL's *s is a jump, no argument: neither is in an argument list.
//
// # The dump
//
// Dump writes the result for the golden tests in testdata: each unit's
// symbols with their storage, then its statements, indented by the
// blocks they are in, their expressions as prefix lists with each
// operator's type:
//
//	== SUBROUTINE SUMUP
//	  SUMUP                   program unit
//	  T         INTEGER       array(1:10)      argument 1
//	  N         INTEGER       variable         argument 2
//	  S         REAL          variable         argument 3
//	  K         INTEGER       variable         $LOCAL+0
//	  $LOCAL is 4 bytes
//	3           S = 0.0
//	4           DO 100 K = 1, N, 1
//	5     100     S = (+:R S (REAL T[K]))
//	6           END
package sema
