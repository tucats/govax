# Phase 78 — FORTRAN expressions and data types

**Status:** planned (2026-10-29). Needs Phase 76 (declarations and
storage) and Phase 77 (MTHRTL). Part of the FORTRAN program laid out in
`PHASE-74 - FORTRAN plan.md` (Decisions 8, 11, 14, and 16).

## Goal

Generate code for every expression in every type, so that any
computation a FORTRAN program can write compiles and runs correctly:

- **every type and length:** BYTE, LOGICAL*1/2/4, INTEGER*2/4, REAL*4
  (F), REAL*8 (D, or G with `/G_FLOATING`), REAL*16 (H), COMPLEX*8,
  COMPLEX*16 (D or G), CHARACTER*n; `/[NO]I4` for the default integer;
- **conversions** by Table 3-1 in assignment and the manual's rules for
  mixed-type expressions (2.3.1.2);
- **arithmetic:** every operator in every type; integer division
  truncating; `**` inline for small integer constant exponents, else
  OTS$POWxx; complex multiplication inline, division by OTS$DIVC;
- **CHARACTER:** substrings, concatenation (into temporaries sized at
  compile time where they can be), assignment with blank fill and
  truncation, comparison with blank padding (CMPC5), assumed-length
  dummy arguments;
- **relational and logical operators**, `.EQV.`, `.NEQV.`, `.XOR.`, and
  logical values as VAX FORTRAN keeps them (low bit; -1 for .TRUE.);
- **the intrinsic functions** of Table D-3: inline where an instruction
  or two does it (ABS, MIN, MAX, MOD, SIGN, DIM, INT, NINT, AINT, ANINT,
  REAL, DBLE, CMPLX, AIMAG, CONJG, ICHAR, CHAR, LEN, ZEXT, DPROD, the
  bit functions IAND, IOR, IEOR, NOT, ISHFT, ISHFTC, IBITS, BTEST,
  IBSET, IBCLR), INDEX and the lexical comparisons (LGE, LGT, LLE,
  LLT) inline or through LIBRTL, and the rest through MTHRTL; generic
  names resolved by argument type; intrinsics passed as actual
  arguments (their specific names).

## What earlier phases leave in place

- **Phase 76:** every expression parsed and typed, constants folded
  (`sema/constant.go`). Its folding computes a COMPLEX product's or
  quotient's parts exactly and rounds once (`complexArith`, unconfirmed);
  the code generated here should round the same, or folding should change
  with it, so a folded expression has the value the program would compute.
- **Phase 77:** MTHRTL's routines and the OTS$ powers.
- **Phase 75:** the generator's instruction selection for longword and
  F_floating; registers; the subset's operators.

## Design notes

- **Instruction selection by type** is a table: each IR operator and
  operand type gives the VAX opcode (ADDB/ADDW/ADDL, ADDF/ADDD/ADDG/ADDH,
  CVTxy). The generator picks two- or three-operand forms by whether the
  destination is an operand.
- **Complex values** are pairs in consecutive storage; the IR treats a
  complex operation as two real operations where it can (addition) and
  a sequence or call where it can't.
- **Character temporaries** for concatenation are static cells sized by
  the expression's maximum length; a concatenation whose length isn't
  known at compile time (an assumed-length operand) takes its space from
  a per-routine dynamic string (LIB$SGET1_DD) and frees it at the
  statement's end.
- **Integer overflow** is not trapped unless `/CHECK=OVERFLOW` (Phase 85)
  sets the IV bit in the entry mask.
- **Order of evaluation** follows the manual where it speaks
  (parenthesized expressions are evaluated as written); otherwise the
  generator's choice.

## Open questions

- **LOGICAL's representation:** VAX FORTRAN tests the low bit and stores
  -1 for .TRUE.; the manual's appendix C.4 settles it.
- **Mixed INTEGER*2 and INTEGER*4** arithmetic: the result type and
  whether intermediate results widen.

## Subtasks (provisional)

1. **Survey:** chapter 2's expressions, Table 3-1, appendix C's
   representations, Table D-3's intrinsics with their specific names
   and argument types.
2. **Every integer and logical type and length.**
3. **Every floating type**, D/G selection, conversions.
4. **COMPLEX.**
5. **CHARACTER:** substrings, concatenation, assignment, comparison.
6. **Exponentiation.**
7. **Inline intrinsics** and the bit functions.
8. **MTHRTL intrinsics**, generic resolution, intrinsics as arguments.
9. **End to end:** programs exercising each type, mixed arithmetic,
   character handling, the math functions.
10. **Close-out:** FORTRAN-INTERNALS.md's chapter on instruction
    selection.

## Progress log

- **2026-10-29:** Planned with Phase 74.
