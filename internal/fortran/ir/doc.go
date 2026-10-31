// Package ir is the FORTRAN compiler's intermediate representation, and
// lowering, the fifth pass, which turns semantic analysis's tree into it
// (docs/PHASE-75 - FORTRAN first light.md, Part B, and Decision 8 of
// docs/PHASE-74 - FORTRAN plan.md).
//
// # Why another representation
//
// After semantic analysis the program is a tree that still reads like
// FORTRAN: a DO statement holds its body, an IF its arms, PRINT its list
// of items, and an array element is an array and its subscripts. The
// code generator wants something closer to the machine, where every
// step the program takes is written down: where each value is in
// memory, which instructions run one after another, and where control
// goes next. Lowering is where FORTRAN's implicit rules become explicit
// code, so that the code generator only has to choose instructions:
//
//   - a DO loop becomes the computation of its iteration count, a test,
//     and the steps at its end (section 5.3.1.1);
//   - an IF block becomes branches around its blocks;
//   - an array element becomes an address: the array's, a constant
//     offset, and an index the VAX's indexed mode multiplies by the
//     element's size;
//   - PRINT becomes calls of FTNRTL's routines, a descriptor for each
//     item;
//   - an argument that isn't a variable becomes a temporary holding its
//     value, whose address is passed;
//   - END becomes a return, with a function's value or a main program's
//     status.
//
// # Basic blocks and trees
//
// A Routine's code is a list of basic blocks (Block). A block is a run
// of statements that always execute together, one after another, ended
// by a terminator that says where control goes: a Jump, a Branch on a
// condition, a Switch on an index (the computed GO TO), a Sign of a value
// (the arithmetic IF), an IndirectJump through a variable an ASSIGN set
// (the assigned GO TO, which lists the blocks it may reach), a Return, or
// NoReturn (after STOP). Nothing jumps into the
// middle of a block. That is the usual shape of a compiler's middle: it
// is what makes control flow something a pass can see (which blocks can
// follow which), and it is what an optimizer (Phase 84) works on.
//
// The statements in a block are only two: Assign (store a value) and
// Call (call a routine, by the VAX calling standard). Their values are
// expression trees: constants, loads from memory, addresses, and
// operations. The VAX is why they stay trees rather than becoming
// three-address code (one operation per statement, every intermediate
// value named): its instructions take any operand straight from memory,
// so I = J*K + L is two instructions with one register between them,
//
//	MULL3   J,K,R0
//	ADDL3   R0,L,I
//
// and a tree shows the code generator which operands are leaves it can
// name in place. A function reference is the exception: lowering takes
// every call out of its expression and puts it before the statement,
// its value in a temporary, so no value is ever held in a register
// across a CALLS.
//
// A COMPLEX expression is split into two trees, one for its real part
// and one for its imaginary part, over F, D, or G operations
// (lowercomplex.go), so the code generator has no complex arithmetic. A
// complex value as a whole is a Quad or an Octa, which is only moved
// (MOVQ, MOVO), passed, or returned; a part is a Mem of its own, the
// imaginary part 4 or 8 bytes past the real one. The products and
// quotients VAX FORTRAN computes in the OTS$ routines are Calls whose
// arguments are the parts by value.
//
// A CHARACTER value is a Text: a Byte place (an element's index multiplied
// by its length, since the VAX's indexed mode multiplies only by the
// operand's size, a byte's 1) and a length, which a substring with bounds
// that aren't constant computes. Assigning one is a statement, CharMove, and
// comparing two an expression, CharCmp, that may stand only where no
// register is held across it: as a Branch's condition (under And, Or, and
// Not), or as the whole value of an Assign; anywhere else lowering computes
// it into a temporary first, as it does a call.
//
// A CHARACTER argument is passed by descriptor, so a CHARACTER dummy's text
// is a Mem whose Ptr is the address its descriptor holds, and a passed-length
// dummy's length the descriptor's first word, made a longword by Zext. A
// concatenation's parts are CharMoves into a temporary (lowerchars.go): a
// TextCell as long as the value can be, or a dynamic string LIB$SGET1_DD
// allocates and LIB$SFREE1_DD frees at the statement's end, before any
// branch that ends its block.
//
// # Storage
//
// FORTRAN's variables are static (Decision 10): each routine's live in
// its part of the $LOCAL program section, where semantic analysis laid
// them out. Lowering adds the compiler's own cells after them (Cell):
// a DO loop's count ($C1) and, when it isn't a constant, its step ($S1);
// temporaries ($T1), which one statement uses and the next may use
// again; and descriptors whose addresses are only known when the routine
// runs ($D1). Read-only data goes in $PDATA (Datum): constants passed by
// reference ($K1), character constants' texts, descriptors whose
// addresses the linker knows, and I/O control blocks ($IO1). The same
// datum serves every use of the same contents.
//
// A dummy argument has no storage of its own: its argument list longword
// holds the address of the caller's variable. A Mem (a place in memory)
// can be based on one (Param), as on a cell or a datum.
//
// # An example
//
// The subroutine
//
//	      SUBROUTINE SUMUP(T, N, S)
//	      INTEGER N, T(10)
//	      S = 0
//	      DO 100 K = 1, N
//	  100 S = S + T(K)
//	      END
//
// lowers to this (as Dump writes it; @N is the variable dummy argument N
// points at, and @T-4[K] the element of T's array at its address less 4
// plus 4 times K, which is T(K) since T's first element is T(1)):
//
//	routine SUMUP subroutine
//	  param T       L  argument 1
//	  param N       L  argument 2
//	  param S       F  argument 3
//	  cell  K       L  $LOCAL+0    variable K
//	  cell  $C1     L  $LOCAL+4    DO 100's count
//	  $LOCAL is 8 bytes
//	B1:
//	  ; line 3
//	  assign @S = 0.0
//	  ; line 4
//	  assign $C1 = @N
//	  assign K = 1
//	  branch le.L($C1, 0) B3 B2
//	B2: L100  ; DO 100's body
//	  ; line 5
//	  assign @S = add.F(@S, cvt.LF(@T-4[K]))
//	  assign K = add.L(K, 1)
//	  assign $C1 = sub.L($C1, 1)
//	  branch gt.L($C1, 0) B2 B3
//	B3:
//	  ; line 6
//	  return
//
// The loop's count, (N - 1 + 1)/1, has become N: lowering folds
// constants as it builds expressions (arith), collecting a Long sum's
// constants into one, so DO loops' counts and subscripts come out short.
// The count is computed before K is set, as section 5.3.1.1 orders, and
// a count of zero or less skips the body.
//
// Every statement remembers the FORTRAN statement it came from, so the
// code generator can write each source line, as a comment, just before
// the instructions it became; the dump shows that as "; line n".
//
// Check verifies a routine's structure (every block terminated, every
// jump to a block of the routine, every place the routine's, operand
// types agreeing), which the tests run on every routine they lower.
package ir
