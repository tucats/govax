# Phase 78 — FORTRAN expressions and data types

**Status:** done 2026-10-31: subtasks 1 (the survey, accepted by the
author), 2, 3, 4, 5, 6, and 7 done 2026-10-30, and 8, 9, 10, and 11
2026-10-31. Needs Phase 76 (declarations and
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
  OTS$POWxx; COMPLEX*8 multiplication inline, COMPLEX*16's by
  OTS$MULCD_R3 or MULCG_R3, division by OTS$DIVC and its D and G forms;
- **CHARACTER:** substrings, concatenation (into temporaries sized at
  compile time where they can be), assignment with blank fill and
  truncation, comparison with blank padding (CMPC5), assumed-length
  dummy arguments;
- **relational and logical operators**, `.EQV.`, `.NEQV.`, `.XOR.`, and
  logical values as VAX FORTRAN keeps them (low bit; -1 for .TRUE.);
- **the intrinsic functions** of Table D-3: inline where an instruction
  or two does it (ABS, MIN, MAX, integer MOD, SIGN, DIM, INT, NINT,
  REAL, DBLE, CMPLX, AIMAG, CONJG, ICHAR, CHAR, LEN, ZEXT, DPROD, the
  bit functions IAND, IOR, IEOR, NOT, ISHFT, ISHFTC, IBITS, BTEST,
  IBSET, IBCLR), INDEX and the lexical comparisons (LGE, LGT, LLE,
  LLT) inline or through LIBRTL, and the rest through MTHRTL; generic
  names resolved by argument type; intrinsics passed as actual
  arguments (their specific names), with an FTNRTL routine for each name
  MTHRTL lacks.

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

# Part A — the survey (2026-10-30)

Section numbers are the 1988 *VAX FORTRAN Language Reference Manual*'s.
Where the OCR text looked wrong, the PDF was read: its page 424 (D-42)
and page 420 (D-38). What a rule means for a pass is in brackets. There
is no VAX FORTRAN compiler to compare with (Phase 74's review, answer
5), so a rule the manual leaves open is our choice, marked unconfirmed,
and goes in MANUAL.md's differences at the close-out.

## Data types and how they're stored (2.1, appendix C)

Every type below is declared, laid out, and given initial values already
(Phase 76). This phase adds their code.

| FORTRAN | IR type | Letter | Bytes | Registers | DTYPE | Table 7-1 |
|---|---|---|---:|---:|---|---|
| BYTE, LOGICAL*1 | Byte | B | 1 | 1 | BU (2) | I5 |
| LOGICAL*2 | Word | W | 2 | 1 | WU (3) | L2 |
| LOGICAL*4 | Long | L | 4 | 1 | LU (4) | L2 |
| INTEGER*2 | Word | W | 2 | 1 | W (7) | I7 |
| INTEGER*4 | Long | L | 4 | 1 | L (8) | I12 |
| REAL*4 | Float | F | 4 | 1 | F (10) | 1PG15.7E2 |
| REAL*8 | Double | D | 8 | 2 | D (11) | 1PG24.16E2 |
| REAL*8, `/G_FLOATING` | GFloat | G | 8 | 2 | G (27) | 1PG24.15E3 |
| REAL*16 | HFloat | H | 16 | 4 | H (28) | 1PG43.33E4 |
| COMPLEX*8 | two Floats | F | 8 | 2 | FC (12) | `'(',1PG14.7E2,',',1PG14.7E2,')'` |
| COMPLEX*16 | two Doubles | D | 16 | 4 | DC (13) | the same, with G23.16E2 |
| COMPLEX*16, `/G_FLOATING` | two GFloats | G | 16 | 4 | GC (29) | the same, with G23.15E3 |
| CHARACTER*n | bytes | B | n | none | T (14) | An |

- **Default lengths** (Table 2-1): INTEGER and LOGICAL are 4 bytes, or 2
  with `/NOI4`. [`sema.IntegerType` and `LogicalType` are package
  variables today, and they become each unit's defaults.]
- **`/[NO]I4` and `/[NO]G_FLOATING`** are FORTRAN command qualifiers and
  OPTIONS qualifiers (1.4.3): an OPTIONS statement "must be the first
  statement in a program unit" and overrides the command line until the
  unit's end. So the format of REAL*8 and the default integer are **a
  unit's**, not a file's. [`Type.format()` has no unit to ask, and has to
  move to the analyzer.]
- **LOGICAL** (C.3, C.4): bit 0 decides, and the other bits are
  undefined. LOGICAL*1 and BYTE hold -128 to 127, and Table 7-1 prints
  them as integers.
- **Integer constants assigned to short data** (2.2.1.1): BYTE and
  LOGICAL*1 take -128 to 255 and LOGICAL*2 and INTEGER*2 take -32768 to
  65535, unsigned values keeping their bits: `X = 255` in a BYTE is -1,
  `X = 65535` in an INTEGER*2 is -1. [A constant outside those ranges is
  INTRANGE, as DATA's is (our choice).]

## Expressions (2.3)

- **Rank** (2.3.1.2): Logical 1, INTEGER*2 2, INTEGER*4 3, REAL*4 4,
  REAL*8 5, REAL*16 6, COMPLEX*8 7, COMPLEX*16 8. An operation's type is
  the higher operand's, but COMPLEX*8 with REAL*8 or REAL*16 is
  COMPLEX*16, "the REAL*8 element is not rounded". [`higher` has this.
  COMPLEX*8 with REAL*16 loses the REAL*16's low bits, as the rule says.]
- **Logical in arithmetic** "is treated as integer". Rank 1 means a
  LOGICAL takes the other operand's type, so LOGICAL*4 + INTEGER*2 is
  INTEGER*2. LOGICAL*4 alone is INTEGER*4 and LOGICAL*2 is INTEGER*2.
  There is no INTEGER*1, so LOGICAL*1 alone is INTEGER*2 (our reading,
  unconfirmed). [`arithmetic` makes every LOGICAL an `IntegerType`
  today.]
- **Integer division truncates.** VAX DIVx does.
- **A real constant in a higher-precision expression** "retains additional
  precision": `1.0D0 + 0.3333333` is as if `0.3333333000000000D0`. Phase
  76's evaluator keeps a literal's exact value for this (`Const.literal`,
  `exactRe`), and its `convert` uses it. [`fold.go`'s older path rounds a
  literal to F first. The two constant paths become one, the evaluator.
  So `D = 0.1` is 0.1D0, where Table 3-1 alone would give a REAL's value
  with a low part of zero; the same as Phase 76's PARAMETER. Unconfirmed.]
- **Precedence:** `**` right to left, then `*` `/`, `+` `-` `//`, the
  relationals, `.NOT.`, `.AND.`, `.OR.`, and last `.XOR.` `.EQV.` `.NEQV.`
  (2.3.4). `A**-B*C` is `A**(-(B*C))`. [The parser has all of this,
  checked.]
- **Parentheses and order:** an equal-precedence order must be
  "algebraically equivalent to" left to right, and parentheses are "often
  important", so floating expressions are never reordered. [`linear`
  collects only integer constants, which stay right modulo 2^32 or 2^16.]
- **Not evaluating a part:** "the compiler can determine the result"
  without a subexpression (2.3.4), and whether a function is called isn't
  promised. [Calls come out of expressions before the statement, so
  `A .AND. F(X)` always calls F. `branchTo` short-circuits the rest.]
- **Relational** (2.3.3): COMPLEX only with .EQ. and .NE., equal if both
  parts are. A lower-ranked operand is converted up. Character operands:
  the shorter padded with blanks, ordered by ASCII. [A new message for
  COMPLEX .LT. and its kind. `logical` reports BADOPERAND only for REAL
  and must add COMPLEX.]
- **Logical operators** (2.3.4): on logicals, a logical value; on integers
  or a mix, an integer, bit by bit. [By rank, with sizes:
  INTEGER*2 .AND. INTEGER*4 sign-extends. `.XOR.` and `.NEQV.` are XORx,
  and `.EQV.` is XORx then MCOMx, each right in bit 0.]
- **Character expressions** (2.3.2): `//` joins left to right, and its
  length is the sum. Parentheses change nothing.

## Assignment and conversion (chapter 3, Table 3-1)

Table 3-1's cells come down to four rules, for the real part where the
source is COMPLEX: integer to real appends `.0`; real to integer
truncates; real to a narrower real rounds; real to a wider real assigns
the high part with a low part of 0. A complex target's imaginary part is
0.0 unless the source is complex, and then it is converted the same way.
A CHARACTER target is truncated or filled with blanks on the right, and
positions outside a substring are left alone (3.3).

The VAX has every conversion these need: `CVTxy` for x and y any of B, W,
L, F, D, G, H but between D and G, and `CVTRxL` to round to a longword
(checked, below). D and G never meet in a unit, since a unit has one
format. [`convert` compares kinds today, so INTEGER*2 to INTEGER*4 gets no
`Convert`. It must compare whole types.] A narrowing integer assignment is
CVTLW or CVTLB, not a move, so that `/CHECK=OVERFLOW` (Phase 85) can trap.

## Substrings and CHARACTER dummies (2.2.4, 6.1.1.3)

`v(e1:e2)` and `a(s)(e1:e2)`, with e1 and e2 numeric (truncated to
integers), defaulting to 1 and the length. The bounds must satisfy
`1 <= e1 <= e2 <= len`, which only `/CHECK=BOUNDS` checks at run time
(Phase 85). [A constant bound out of range is an error now.] A dummy's
length is its actual's, each time control arrives. By Decision 11 it
comes by descriptor: `MOVZWL @n(AP),Rx` is the length, the descriptor's
first word; the address is `MOVL n(AP),Rx` then `MOVL 4(Rx),Rx`.

## The intrinsic functions (6.3, Tables 6-3 and D-3, D.5)

`sema/intrinsics.go`'s set was compared with Table D-3, and has all 219
names and no others. (The OCR's `AFLOAT` is `QFLOAT` in the PDF.)

Rules beyond the table:

- **Generic selection** is by argument type, each reference by itself
  (6.3.1.1). IMPLICIT doesn't change an intrinsic's type (6.3.1). A name
  that is a statement function, dummy, variable, array, or common block
  isn't generic. EXTERNAL makes it the user's.
- **Footnote 10:** INT, IDINT, IQINT, NINT, IDNINT, IQNINT, IFIX, MAX1,
  MIN1, and ZEXT return the default integer, by `/[NO]I4`, whatever the
  argument. The others follow the argument (ABS of an INTEGER*2 is
  IIABS, an INTEGER*2).
- **Bit functions** (D.5.2): the generic maps to the J form "if either I or
  J or both were INTEGER*4", so the higher of the arguments decides, and
  "if ISHFT is called with constant arguments, it returns an INTEGER*4".
- **Footnote 11:** REAL of a real, DBLE of a double, INT of an integer, and
  QEXT of a REAL*16 return the value unconverted. [INT of an INTEGER*2
  keeps its value and has footnote 10's type.]
- **As actual arguments** (4.9, 6.3.1.1): named in INTRINSIC; a generic
  name passed has no argument list to select by, so it "is treated
  according to the rules for handling specific" names.
- **ICHAR** of a longer string is its first byte's value. **SIZEOF** isn't
  allowed an assumed-size array or passed-length character (Table D-3).

The table's families, and how each is compiled. Under `/G_FLOATING` a D
routine is its G routine (`MTH$DSQRT` is `MTH$GSQRT`, `MTH$CDSQRT` is
`MTH$CGSQRT`, `IIDINT` is `IIGINT`); a Q specific is its H routine
(`QSQRT` is `MTH$HSQRT`, `JIQINT` is `MTH$JIHINT`).

| Generic | Specific names | How |
|---|---|---|
| SQRT, EXP, SIN, COS | x, Dx, Qx, Cx, CDx | MTH$ |
| LOG | ALOG, DLOG, QLOG, CLOG, CDLOG | MTH$ |
| LOG10 | ALOG10, DLOG10, QLOG10 | MTH$ |
| SIND, COSD, TAN, TAND, ASIN, ASIND, ACOS, ACOSD, ATAN, ATAND, SINH, COSH, TANH | x, Dx, Qx | MTH$ |
| ATAN2, ATAN2D | x, Dx, Qx (`ATAN2D` is `MTH$ATAND2`) | MTH$ |
| ABS | IIABS, JIABS, ABS, DABS, QABS | a test and MNEGx |
| | CABS, CDABS | MTH$ |
| IABS | IIABS, JIABS | a test and MNEGx |
| INT, IDINT, IQINT, IFIX | IINT, JINT, IIDINT, JIDINT, IIQINT, JIQINT, IIFIX, JIFIX; complex has no name | CVTxW, CVTxL |
| AINT, ANINT | AINT, DINT, QINT, ANINT, DNINT, QNINT | MTH$ (a CVT round trip fails for large values) |
| NINT, IDNINT, IQNINT | ININT, JNINT, IIDNNT, JIDNNT, IIQNNT, JIQNNT | CVTRxL, then CVTLW for a word |
| ZEXT | IZEXT, JZEXT | MOVZxy |
| REAL, FLOAT, DBLE, DFLOAT, QEXT, QFLOAT | FLOATI, FLOATJ, SNGL, SNGLQ, DBLE, DBLEQ, DFLOTI, DFLOTJ, QEXT, QEXTD | CVTxy |
| CMPLX, DCMPLX | none, one or two arguments | parts |
| — | REAL, DREAL, AIMAG, DIMAG | parts |
| CONJG | CONJG, DCONJG | parts, MNEGx |
| — | DPROD | CVTFD twice, MULD (exact) |
| MAX, MIN, MAX0, MIN0, MAX1, MIN1, AMAX0, AMIN0 | IMAX0, JMAX0, AMAX1, DMAX1, QMAX1, IMAX1, JMAX1, AIMAX0, AJMAX0, and MIN's | compares and branches |
| DIM, IDIM | IIDIM, JIDIM, DIM, DDIM, QDIM | a compare and a branch |
| SIGN, ISIGN | IISIGN, JISIGN, SIGN, DSIGN, QSIGN | a test and a branch |
| MOD | IMOD, JMOD | DIVx, MULx, SUBx |
| | AMOD, DMOD, QMOD | MTH$ |
| IAND, IOR, IEOR, NOT | II and J forms, INOT, JNOT | BICx, BISx, XORx, MCOMx |
| ISHFT | IISHFT, JISHFT | a constant count: ASHL or EXTZV; otherwise MTH$ |
| IBITS | IIBITS, JIBITS | EXTZV |
| IBSET, IBCLR | II and J forms | BISL, BICL, with a mask made by ASHL if the bit isn't constant |
| BTEST | BITEST, BJTEST | EXTZV, then MNEGL |
| ISHFTC | IISHFTC, JISHFTC | FTNRTL's FTN$IISHFTC, FTN$JISHFTC: no instruction does it, and MTHRTL has none |
| — | LEN | a constant, or the descriptor's first word |
| — | INDEX | MATCHC |
| — | ICHAR | MOVZBL |
| — | CHAR | CVTLB into a one-byte temporary |
| — | LLT, LLE, LGT, LGE | as the relational operators |
| — | NWORKERS | the constant 1: Decision 1 leaves out the parallel directives, so one process runs a routine |
| — | SIZEOF | a constant |

ASHL is an arithmetic shift, and ISHFT's is logical (D.5.2), so a right
shift by a constant is EXTZV, and one whose direction isn't known at
compile time is `MTH$JISHFT` or `MTH$IISHFT`. MVBITS is a system
subroutine, Phase 83's.

**The table's three odd rows.** IIBITS, IIBCLR, and IISHFTC are printed
with an INTEGER*2 argument and an **INTEGER*4 result**, against IIBSET's
INTEGER*2. It isn't the OCR: page D-42 prints it so. The author's answer 3
follows the table.

**Specific names with no routine to pass.** MTHRTL has no entry for
LEN, INDEX, ICHAR, CHAR, LLT, LLE, LGT, LGE, BITEST, BJTEST, IIBITS, JIBITS,
IIBSET, JIBSET, IIBCLR, JIBCLR, IISHFTC, JISHFTC, IZEXT, JZEXT, SNGLQ, DBLEQ,
QEXT, and QEXTD. NWORKERS and SIZEOF aren't routines. Every other specific
name has one, by the mapping above. The author's answer 4 gives each an FTNRTL
routine.

## Which routines the calls use (Phase 77's survey)

| Routines | Arguments | Result |
|---|---|---|
| MTH$, F or integer result | by reference | R0 |
| MTH$, D or G result; F complex | by reference | R0:R1 |
| MTH$, H, D complex, or G complex result | by reference, after the result's address | through the first argument |
| OTS$ | **by value**: a D two longwords, an H or D complex four | R0, R0:R1, or R0:R3 |

The powers, by the base's and exponent's types after conversion:

| Base | INTEGER*2 | INTEGER*4 | REAL*4 | REAL*8 | REAL*16 | COMPLEX |
|---|---|---|---|---|---|---|
| INTEGER*2 | POWII | POWJJ | as REAL*4 | as REAL*8 | as REAL*16 | as COMPLEX |
| INTEGER*4 | POWJJ | POWJJ | as REAL*4 | as REAL*8 | as REAL*16 | as COMPLEX |
| REAL*4 | POWRJ | POWRJ | POWRR | POWRD | as REAL*16 | as COMPLEX |
| REAL*8 | POWDJ | POWDJ | POWDR | POWDD | as REAL*16 | as COMPLEX*16 |
| REAL*16 | POWHJ_R3 | POWHJ_R3 | POWHH_R3 | POWHH_R3 | POWHH_R3 | as COMPLEX*16 |
| COMPLEX*8 | POWCJ | POWCJ | POWCC | POWCDCD_R3 | POWCDCD_R3 | POWCC, or CDCD |
| COMPLEX*16 | POWCDJ_R3 | POWCDJ_R3 | POWCDCD_R3 | POWCDCD_R3 | POWCDCD_R3 | POWCDCD_R3 |

With G, `G` stands for `D` (POWGJ, POWGG, POWCGJ_R3, POWCGCG_R3); POWRD and
POWDR have no G forms, so a REAL*4 with a REAL*8 converts to G and uses
POWGG. An INTEGER exponent is widened to a longword, as a REAL base's only
integer power takes one. The cells the manual's rank rule fills are ours
where they're said to be "as" something. Small constant exponents go in
line, in **the order `vaxfloat.PowerBySquaring` uses**, which is OTS$'s and the
folding's: X**7 is `X * X**2 * X**4`, each square rounded. A negative
exponent multiplies the squares' reciprocals, as VMS 7.1's does (Phase 77),
so it isn't worth doing in line: only exponents from 2 to 8 are, and the
rest call.

Complex: `OTS$MULCD_R3`, `OTS$MULCG_R3`, `OTS$DIVC`, `OTS$DIVCD_R3`,
`OTS$DIVCG_R3`. **There is no `OTS$MULC`:** F's product has to be in line.

## What was checked on govax's MACRO

A one-off program (not kept) of every instruction form below assembled with
no message on 2026-10-30: B and W arithmetic with `#-1`, `#255`, and
`#65535`; every CVT and CVTR above; ADD, SUB, MUL, DIV, MOV, MNEG, CMP, TST,
and CLR in D, G, and H; MOVQ, MOVO, `MOVQ R0,-(SP)`, `MOVO R0,-(SP)`; MOVA and
PUSHA in B, W, F, Q, D, G, O, and H, indexed; MOVC3, MOVC5, CMPC3, CMPC5,
MATCHC, LOCC; the unsigned branches; ASHL, ROTL, EXTV, EXTZV, INSV, BBS, BBC,
BIT; `MOVAB @4(AP)[R0],R1` and `PUSHAQ @4(AP)[R0]`. So MACRO needs nothing
found yet. The CPU has them all (Phase 35's oracle).

## What the compiler has to change

Most of Phase 75's code assumes a value is a longword, or one of two
longword types. By file:

**`sema`:** `Compiled` and `uncompiledUse` go. `convert` compares kinds
(above). `arithmetic` and `logical` name `IntegerType`. `fold.go`'s
`realConst`, `realOp`, and `foldConvert` assume F_floating; `folded`'s
"not compiled" test goes with them. Character assignment, substrings,
`//`, comparison, and character arguments are NOTIMPL. `format()` needs the
unit. DO variables stay longword or F_floating until Phase 79 (NOTIMPL
otherwise), since `tripCount` has only those.

**`ir`:** `Type` has Long and Float. `Const.Bits` is 64 bits and an H value
needs 128 (`vaxfloat.Bits`). `foldUnary` and `foldBinary` treat "not Long"
as F_floating, and so would misread any new type. `Arg.Value` is a longword;
`Call.Result` assumes R0. `typeOf` and `dtypeOf` know four cases. Temporaries
are by type already.

**`codegen`:**

- `need` counts every value as one register. D, G, and complex*8 values need
  a consecutive pair, and H and complex*16 four, so `need`, `pairNeed`, and
  `alloc` take a width.
- `spill` makes every spill cell a longword, and `localSize` adds 4 each.
- **Indexed addressing scales by the instruction's own type**, so `MOVAL` (in
  `eval` and `store`) and `pushAddress`'s `PUSHAL` are wrong for any element
  that isn't a longword. A complex element's parts need `Index*2` and F's or
  D's scale, and a character element `Index*length` and a byte's.
- `constant` writes `#n` or an F datum, and `store` moves an F constant with
  MOVL; D, G, and H need a datum and `MOVx`.
- `conditions` has signed branches only. **CMPC sets the codes as an
  unsigned comparison**, so character comparison needs BLSSU, BGTRU, and the
  rest.
- `branchTo`'s `BLBC x` **reads a longword**, which at a BYTE or LOGICAL*2 is
  bytes past the variable, and can fault at the end of memory. A memory
  operand shorter than a longword tests with BITx.
- `arithmetic`'s INC and DEC are Long's only, and `.AND.`'s mask (`opOperands`)
  is made with Long constants and operators.
- `entryMask` saves what `alloc` used. **An `OTS$..._R3` routine destroys R2 and
  R3, and the character instructions R0 to R5**, none of which `alloc` saw, so
  they must be marked used.

**`internal/ftnrtl`:** `FTN$IO_ITEM`'s CASEB has four types. Table 7-1 needs
the other nine; `OTS$$CVT_D_T_R8`, `_G_`, and `_H_` give the digits. It gets
the routines for names MTHRTL lacks (below), and its minor id goes up.

**The command:** `console.dcl`, `dclcmd/fortran.go`, `fortran.Options`, and
HELP get `/[NO]I4` and `/[NO]G_FLOATING`; sema reads OPTIONS's. OPTIONS's
other qualifiers stay NOTIMPL (Phase 85).

## Decisions

Each is the survey's, accepted by the author (2026-10-30).

1. **Complex values are split in lowering.** A complex expression becomes
   its real part's tree and its imaginary part's, over F, D, or G
   operations, so the code generator has no complex arithmetic. A target that
   may overlap an operand (`C = C*D`) is computed into a temporary. A whole
   complex value moved or passed is MOVQ or MOVO. The doc's design note
   stands.
2. **Character operations are IR statements, not expressions:** `CharMove`
   (MOVC3 or MOVC5) and `CharCompare` (CMPC5, as a `Branch` condition or a
   value). A character place is an address expression and a length expression.
   They hold no register across them: every operand of a MOVC is evaluated
   before it runs, and lowering takes what must survive into cells. So R0 to R5
   being destroyed asks nothing of the allocator but the entry mask.
3. **Registers:** a value takes a consecutive run of its width. The pool
   grows to R0–R11, which only the entry mask pays for and which keeps a run
   from failing to fragmentation; `budget` stays a limit on what's held, and
   `fit` spills cells of the value's own size.
4. **Calls use CALL entries, never JSB,** for one calling sequence. JSB
   entries' savings are Phase 84's to consider.
5. **By-value arguments and wide results are the IR's.** `Arg.Value` takes
   any type (`MOVQ`/`MOVO x,-(SP)`, the CALLS count in longwords), `Call.Result`
   stores from R0 up by its width, and a hidden result is an ordinary first
   argument.
6. **In-line intrinsics that branch** (MIN, MAX, SIGN, DIM, ABS) lower as
   calls do: their blocks come before the statement, their value in a temporary.
7. **The intrinsics are a table** (as the RTL's routines are): for each
   name, its generic's selections or its specific's argument and result types,
   and its code: an IR operation, an in-line lowering function, or a routine by
   format. `constFuncs` stays for constant expressions, and the same names
   agree by test.
8. **A concatenation always builds in a temporary,** even when the target
   doesn't overlap, because a dummy or EQUIVALENCE can hide an overlap and 3.3
   doesn't forbid one. Static cells while lengths are known. A passed-length
   operand takes LIB$SGET1_DD's string, freed at the statement's end. MOVC's
   overlap handling already makes a plain `S(2:) = S` right.
9. **`PRINT *, 5` stays I12.** The author's answer 2's rule applies only inside a mixed
   operation; a constant alone remains a longword, as an argument is.
10. **External FUNCTION results stay Phase 79's** (its subtask 5). This phase
    has the mechanism for MTH$ and OTS$; the FUNCTION statement of a new type,
    and a reference to one, stay NOTIMPL.

## The author's answers (2026-10-30)

The author accepted Part A's decisions, and answered its questions:

1. **COMPLEX products and folding.** Accepted as proposed. In-line MULs round
   at each step, where folding's `vaxfloat.ComplexMul` is exact and rounded
   once, shared with `OTS$MULCD_R3`, and a folded expression must have the value
   the program would compute. So D and G call `OTS$MULCD_R3` and `OTS$MULCG_R3`,
   the same code as folding by construction. F, with no routine to call, is in
   line in F's own instructions, and folding of an F product changes to those
   same steps (`vaxfloat.ComplexMulSteps`).
2. **Small integer constants and INTEGER*2.** Accepted as proposed. A constant
   from -32768 to 32767 "is treated as an INTEGER*2 data type" (2.2.1.1), and
   2.3.1.2 says the same of PARAMETER constants. In a mixed operation, an INTEGER
   constant that fits takes an INTEGER*2 or logical partner's type, so `I2*1000`
   is MULW, and with `I2` = 100 wraps to -31072. Two constants fold in the default
   integer, so 30000+30000 is 60000, as today. Unconfirmed.
3. **IIBITS, IIBCLR, and IISHFTC's INTEGER*4 results.** Accepted as proposed:
   the table as printed, computing in a word and sign-extending. D.5.2's "for each
   type" and IIBSET's INTEGER*2 suggest misprints, but the table is the only
   statement, and it shows only in PRINT's width and in mixed arithmetic.
4. **Passing a name with no routine: FTNRTL gets a routine for each** (the author's
   choice, in place of the proposed NOTPASSABLE error). See "Routines for names MTHRTL
   lacks" below.
5. **The corrections below** are accepted.

## Routines for names MTHRTL lacks

4.9 limits nothing, so every function name but SIZEOF can be passed. Each name with
no MTHRTL entry gets a small routine in FTNRTL, in MACRO, of the same name under
`FTN$`, called as any FORTRAN function of its type would be through a dummy (Decision
11): numeric arguments by reference, character ones by descriptor, a value in R0, or
through a hidden first argument (CHAR's by descriptor, QEXT's by reference).

| Routines | Do | Result |
|---|---|---|
| `FTN$LEN`, `FTN$INDEX`, `FTN$ICHAR` | a descriptor's length, MATCHC, MOVZBL | R0, INTEGER*4 |
| `FTN$CHAR` | MOVB through the result's descriptor | hidden, CHARACTER |
| `FTN$LLT`, `FTN$LLE`, `FTN$LGT`, `FTN$LGE` | CMPC5, -1 or 0 | R0, LOGICAL*4 |
| `FTN$BITEST`, `FTN$BJTEST` | EXTZV | R0, LOGICAL*2, LOGICAL*4 |
| `FTN$IIBITS`, `FTN$JIBITS` | EXTZV | R0, INTEGER*4 (Table D-3) |
| `FTN$IIBSET`, `FTN$JIBSET` | BISx | R0, INTEGER*2, INTEGER*4 |
| `FTN$IIBCLR`, `FTN$JIBCLR` | BICx | R0, INTEGER*4 (Table D-3) |
| `FTN$IISHFTC`, `FTN$JISHFTC` | a circular shift of the low bits | R0, INTEGER*4 (Table D-3) |
| `FTN$IZEXT`, `FTN$JZEXT` | MOVZ, of the argument's low word | R0, INTEGER*2, INTEGER*4 |
| `FTN$SNGLQ` | CVTHF | R0, REAL*4 |
| `FTN$DBLEQ`, and `FTN$GBLEQ` under `/G_FLOATING` | CVTHD, CVTHG | R0:R1, REAL*8 |
| `FTN$QEXT` | CVTFH | hidden, REAL*16 |
| `FTN$QEXTD`, and `FTN$QEXTG` under `/G_FLOATING` | CVTDH, CVTGH | hidden, REAL*16 |
| `FTN$QFLOAT` | CVTLH; QFLOAT has no specific name, so an INTEGER*4 argument | hidden, REAL*16 |
| `FTN$NWORKERS` | the constant 1 | R0, INTEGER*4 |

That is 28 routines: the 24 names, two more that `/G_FLOATING` needs, and QFLOAT and
NWORKERS, which would otherwise be the only functions unpassable but SIZEOF, so that
the rule has one exception.

A routine reached by reference can't see its argument's length, and Table D-3 gives
CHAR, IZEXT, and JZEXT several argument types. The VAX stores low bytes first, so a
shorter read of any of them is its low part: CHAR reads a byte, and IZEXT and JZEXT read
a word. So a JZEXT passed as an argument and given an INTEGER*4 loses that value's high
word, which a direct reference doesn't (unconfirmed).

**SIZEOF** takes anything and has no value at run time, so it alone is an error as an
argument: NOTPASSABLE, naming it, and listed in MANUAL.md's differences.

**Generic names** passed are treated as specific (6.3.1.1): a name that is also a
specific name is that one; another is its default-typed specific where one exists
(`IABS` as `JIABS`, `MOD` as `JMOD`, `LOG` as `ALOG`, `INT` as `JINT`, `MAX` as `JMAX0`),
by `/[NO]I4` where that chooses, unconfirmed. `CMPLX` and `DCMPLX` are MTHRTL's own
(`MTH$CMPLX`, `MTH$DCMPLX`, or `MTH$GCMPLX`).

The ISHFTC routines are also what ordinary references to ISHFTC call, since no
instruction does it. Each routine is tested directly from MACRO (Decision 20), beside
the in-line form it must agree with. `ftnrtl.opt`'s minor id goes up.

## Corrections to the plan (accepted 2026-10-30)

1. **Character arguments move from Phase 79 to here:** descriptors for variables,
   elements, substrings, constants, and expressions as actual arguments, and
   passed-length dummies. This phase's Goal already claims assumed-length dummies
   and LEN, which can't be tested without calls. Phase 79 keeps Hollerith arguments
   and CHARACTER functions.
2. **List-directed output of every type moves from Phase 80 to here**, with
   each type's subtask: Table 7-1's formats, so that each type's programs can
   print what they compute. Phase 80 keeps list-directed input and the rest. Its
   open question on records and line length stays open.
3. **`/[NO]I4` and `/[NO]G_FLOATING` and those two OPTIONS qualifiers are
   here;** Phase 85 keeps OPTIONS's `/CHECK` and `/EXTEND_SOURCE`.
4. **Passing an intrinsic's name is here, and calling it through a dummy is
   Phase 79's** (dummy procedures). This phase tests a passed name with a MACRO
   helper, as `TestFortranCodeRuns` does.
5. **AINT, ANINT, real MOD, and ISHFT by a variable count aren't in line,** though
   the Goal lists AINT, ANINT, and MOD: none is "an instruction or two".

Phases 79, 80, and 85's docs have these.

## The open questions, answered

- **LOGICAL's representation:** C.4 settles only that bit 0 decides. -1 for
  .TRUE. is Phase 75's choice and stays, in every length (`FF`, `FFFF`,
  `FFFFFFFF`), which makes `.NOT.` a plain MCOM and keeps `.AND.` and `.OR.`
  bit by bit on every operand.
- **Mixed INTEGER*2 and INTEGER*4:** 2.3.1.2 settles it. The result has the
  higher rank; two INTEGER*2s compute in a word (ADDW3), wrapping, with no
  widening of intermediate results, and an assignment converts the final value.
  What 2.2.1.1 does to constants is the author's answer 2.

## Subtasks (revised by the survey)

Each adds its types' list-directed output and its programs to
`testdata/fortran`, in both D and G where a type has both, and each
stops for review.

1. **Survey** (this Part A). Done.
2. **Integer and logical lengths:** Byte and Word in the IR, one constant path in
   sema, conversions, every operator, `.EQV.`, `.NEQV.`, `.XOR.`, indexes scaled by
   type, BIT tests, `/[NO]I4` and OPTIONS's. Done.
3. **Floating:** D, G, and H, 128-bit constants, register runs, wide spills,
   `/[NO]G_FLOATING`, Table 3-1's real cells. Done.
4. **COMPLEX:** splitting in lowering, by-value arguments and wide results, products
   and OTS$DIVC, the folding change. Done.
5. **CHARACTER values:** places, substrings, array elements, assignment, comparison,
   the unsigned branches. Done.
6. **CHARACTER arguments and concatenation:** descriptors at run time, passed-length
   dummies, static temporaries, LIB$SGET1_DD. Done.
7. **Exponentiation:** the table above, in-line small powers in OTS$'s order. Done.
8. **In-line intrinsics:** the table, generic selection, footnotes 10 and 11,
   conversions, bit functions, FTN$IISHFTC and FTN$JISHFTC, character functions,
   argument messages. Done.
9. **MTHRTL intrinsics** by format, intrinsic names as arguments, and FTNRTL's
   routines for the names MTHRTL lacks. Done.
10. **End to end:** programs on each type, mixed arithmetic, character handling, the
    math functions, in both paging modes for a subset. Done.
11. **Close-out:** FORTRAN-INTERNALS.md's chapter on instruction selection, MANUAL.md's
    chapter 14 and differences, HELP. Done.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-10-30, subtask 1:** the survey, Part A. Read chapters 2 and 3,
  appendix C, Tables 6-3, 7-1, and D-3 (the odd rows in the PDF), and the
  compiler's sema, ir, and codegen; assembled a probe of the instructions to
  come on govax's MACRO (no messages); compared sema's intrinsic names
  with the table and with MTHRTL's entries. Nothing in the code changes.
- **2026-10-30:** the author accepted the survey's decisions, its proposals, and
  its corrections to the plan, but for one: each intrinsic name with no MTHRTL
  entry gets an FTNRTL routine, where the survey proposed an error. Part A and
  Phases 79, 80, and 85's docs now say so.
- **2026-10-30, subtask 2:** every length of INTEGER and LOGICAL.
  - **`sema`:** `fold` now uses the evaluator's operations, so there is one set
    of constant rules (`fold.go` kept only `fold` itself, and an executable
    statement's literals keep their exact values). `operationType` and
    `logicalType` are the ranks with logicals (a logical takes the other's type;
    two are an integer of the longer length, INTEGER*2 for two bytes);
    `mixedType` is the author's answer 2; `assigned` (in `exec.go`) is 2.2.1.1's
    unsigned ranges for a constant, past which INTRANGE. `convert` compares whole
    types. `.EQV.`, `.NEQV.`, and `.XOR.` compile. `Analyze` takes `Options`, each
    unit's defaults are `defaultInt` and `defaultLogical`, and `prologue` reads
    OPTIONS's `/I4` and `/NOI4` for the unit following; its other qualifiers are
    NOTIMPL, one message each.
  - **`ir`:** `Byte` and `Word`; `IntConst` and `Const.Int` by type; `Xor`; `.EQV.`
    as `not(xor)`; folding by type. A constant datum is a `.LONG` whatever its
    length: the low bytes come first, so a word's reference reads it, and what
    follows stays aligned. Word DO counts are computed in longwords.
  - **`codegen`:** INC and DEC by type, `addressOp` (MOVAx and PUSHAx by the element's
    type), XOR, and a short operand in memory tested with BIT rather than BLBC,
    which reads a longword. Spill cells stay longwords until subtask 3.
  - **FTNRTL:** Table 7-1's I5 and I7 and L2 of a word; `LOGICAL` tests its first
    byte alone, for the same reason.
  - **The command:** `/[NO]I4`, `govax fortran --no-i4`, HELP, and MANUAL.md.
  - **Tests:** `testdata/fortran/lengths`, in both paging modes;
    `shorts.for` golden in `ir` and `codegen`; `TestOperationTypes`, `TestNoI4`.
  - **Beyond the survey:** Decision 10 leaves a FUNCTION of a new type to Phase 79,
    but a word or byte function needs nothing that isn't here (its value in R0,
    stored with MOVW or MOVB), and with `/NOI4` every implicitly typed integer
    function is one, so `Compiled` admits them. An INTEGER*2 DO variable is the same
    case, since `/NOI4` makes every implicit one a word; the other DO forms stay
    Phase 79's. REAL*8 functions and the rest stay NOTIMPL.
  - **Unconfirmed, ours:** two LOGICAL*1s in arithmetic are an INTEGER*2; INTRANGE
    for a constant past a short variable's unsigned range; `.TRUE.` and comparisons
    remain LOGICAL*4 under `/NOI4`, which only SIZEOF could show.
- **2026-10-30, subtask 3:** REAL\*8 in D and G, and REAL\*16.
  - **`sema`:** a REAL\*8's format is the unit's (`analyzer.format`, in place of
    `Type.format`): D, or G with `/G_FLOATING` or the unit's OPTIONS
    `/G_FLOATING` (`/NOG_FLOATING` undoes it); `Unit.GFloating` tells lowering.
    Constants round, fold, and range-check in it (`realRange` names G's range,
    0.56E-308 to 0.9E308), and initial values' bytes are packed in it.
    `Compiled` admits every REAL; `returned` keeps a REAL\*16 function, defined
    or referenced, NOTIMPL (its hidden argument is Phase 79's).
  - **`ir`:** `Double`, `GFloat`, `HFloat`, with `Floating`, `Format`, and
    `Width` (registers: 1, 2, or 4); `Const.High` holds an H value's upper eight
    bytes, `RealConst` packs by type and `Longwords` unpacks for the data; folding
    by format, conversions between floating types rounding only when narrowing;
    `typeOf`, `dtypeOf`, and `constOf` are the lowerer's, since they ask the
    unit; DSC$K_DTYPE_D, G, and H; the dump writes D and Q exponents.
  - **`codegen`:** Decision 3: R0 to R11, `alloc` taking the lowest run of a
    type's width, `inRegister` holding the run. `need` counts widths:
    `pairNeed` is the harder operand first, then what its value holds (`held`)
    beside the other's need, which for longwords is the old rule exactly, so no
    golden file of an earlier subtask changed. Spill cells are the value's size,
    at least a longword, and reused by size. A D, G, or H constant is a datum of
    its longwords moved with MOVx; MOVA and PUSHA take D, G, and H for an
    element's scale. A function's D or G value comes back in R0 and R1.
  - **FTNRTL:** Table 7-1's 1PG24.16E2, 1PG24.15E3, and 1PG43.33E4: one G
    editing routine, parameterized by width, digits, and exponent digits, with
    `DIGITS_OF` calling `OTS$$CVT_F_`, `D_`, `G_`, or `H_T_R8`. The vector has no
    new entry, so GSMATCH stays 1,4.
  - **The command:** `/[NO]G_FLOATING`, `govax fortran --g-floating`, HELP, and
    MANUAL.md.
  - **Tests:** `testdata/fortran/reals` (D) and `realsg` (the same in G, by
    OPTIONS, with a value past D's range), in both paging modes, their outputs
    checked against exact rational arithmetic rounded to each format's bits, not
    copied from a run; `floats.for` golden in `ir` and `codegen`; `TestGFloating`.
    A program of nested indexed H and D expressions (not kept) spilled and gave
    the values worked by hand.
  - **Beyond the survey:** a REAL\*8 FUNCTION, like subtask 2's word functions,
    needs nothing that isn't here (MOVD into R0 and R1, and MOVD from them at the
    call), so it compiles; REAL\*16 functions stay Phase 79's. A REAL\*8 or
    REAL\*16 DO variable compiles too: `tripCount`'s floating path is by type
    now.
  - **Unconfirmed, ours:** zero in E form at every width (`0.0000000000000000E+00`),
    as subtask 1 chose for REAL\*4; an exponent written with all of the format's
    e digits (`E-0018`), Ew.dEe's form.
- **2026-10-30, subtask 4:** COMPLEX\*8 and COMPLEX\*16.
  - **`sema`:** `Compiled` admits COMPLEX, so COMPLEX names, constants, PARAMETERs,
    and IMPLICIT COMPLEX compile; a COMPLEX operand of `.LT.`, `.LE.`, `.GT.`, or
    `.GE.` is BADOPERAND, as it already was in a constant expression (the survey
    proposed a new message; BADOPERAND says the same). Folding a COMPLEX\*8 product
    is `vaxfloat.ComplexMulSteps` (the author's answer 1): `complexArith` is the
    operators', `complexOTS` the exact form, which the powers keep, since
    OTS$POWCJ multiplies exactly in F_floating too.
  - **`ir`:** Decision 1 in `lowercomplex.go`: `complexParts` gives a complex
    expression's two trees; `parts` makes a value's place into its parts' (an
    element's index doubled, the imaginary part a part's size on); sums,
    differences, and negation part by part; F products in line, each operand part
    used twice made `reusable`; D and G products and every quotient by
    `complexCall` (OTS$MULCD_R3, MULCG_R3, DIVC, DIVCD_R3, DIVCG_R3); conversion
    either way by Table 3-1; `.EQ.` as the `and` of the parts' comparisons, `.NE.`
    as the `or`. The whole value is a new type, `Quad` or `Octa`, only moved,
    passed, or returned (Decision 5: `Arg.Value` of any width, `Call.Result` from R0
    up); a constant is a `Const` of one with its `Part` type. `assignParts` stores
    the real part through a temporary when the imaginary part's tree reads memory
    that might be the real part's place (`C = C*D`), and straight otherwise.
    `arith` drops a floating `+ 0.0`, which a real operand's imaginary part would
    otherwise add. DSC\$K_DTYPE_FC, DC, and GC.
  - **`codegen`:** a value argument pushed with MOVQ or MOVO for 8 or 16 bytes, the
    CALLS count in longwords; a result's registers marked used, so a routine that
    calls an `_R3` routine saves R2 and R3 in its entry mask; MOVAQ, MOVAO, PUSHAQ,
    PUSHAO for whole complex values; complex constants from their data.
  - **FTNRTL:** Table 7-1's complex formats, each part's G field (FORMAT_REAL, now
    a subroutine REAL and the complex types share) without its blanks, between
    " (", ",", and ")", put as one field. No new vector entry: GSMATCH stays 1,4.
  - **Tests:** `testdata/fortran/complex` (COMPLEX\*16 in D) and `complexg` (in G,
    by OPTIONS, with a product past D's range), in both paging modes, their outputs
    worked with exact rational arithmetic rounded to each format, the F products by
    steps, not copied from a run; `complexes.for` golden in `ir` and `codegen`;
    `TestComplexMulSteps`; sema cases for the folding and BADOPERAND.
  - **Beyond the survey:** a COMPLEX\*8 FUNCTION needs nothing that isn't here (its
    value in R0 and R1, MOVQ), so it compiles, as subtask 3's REAL\*8 functions do;
    COMPLEX\*16 functions, whose value comes back through a hidden argument, stay
    Phase 79's.
  - **Unconfirmed, ours:** the blank before a complex value's field (every other
    numeric field begins with one, and it keeps the value apart from the one
    before); a complex value never split across records, though the manual allows
    a split after the comma. Efficiency is Phase 84's: an element's index is
    computed once per part, and a dummy's offset makes an index of its own.
- **2026-10-30, subtask 5:** CHARACTER values.
  - **`sema`:** `Compiled` and `uncompiledUse` are gone (every type compiles);
    `characterDummy` reports a CHARACTER dummy argument NOTIMPL where a statement
    first uses it, subtask 6's, and any character value as an argument is
    NOTIMPL too. `Substring` (`substring`): of a variable or array element, its
    bounds made INTEGER, Lower 1 when left out and Upper nil for the last
    character; its type CHARACTER of its length, or of size 0 when a bound isn't
    constant. A constant bound outside the variable is SUBRANGE (both constant)
    or SUBBOUND (one), and a substring of anything else SUBSTR (a constant's is
    BADUSE). Assignment between CHARACTER places of any lengths is an `Assign`
    with no `Convert`; CHARACTER and anything else is ASSIGNTYPE, but for a
    character constant, still a Hollerith constant (NOTIMPL). Two CHARACTER values
    compare as they are; with a number, COMPARETYPE. A CHARACTER value in
    arithmetic or as a logical operand is CHARNUMBER, and as a condition
    NOTLOGICAL. A CHARACTER FUNCTION stays Phase 79's (`returned`).
  - **`ir`:** Decision 2. A character value's place is a `Text`, a Byte place (a
    CHARACTER cell's type is Byte now; an element's index is multiplied by its
    length in `mem`) and a Long length (`chars`: a substring's offset and index
    from its first bound, its length `(upper+1)-lower`). `CharMove` is the
    assignment; `CharCmp` the comparison, which `cond` keeps in a branch's
    condition under `.AND.`, `.OR.`, and `.NOT.` and `assign` keeps as an Assign's
    whole value, and which `expr` computes into a temporary anywhere else; `Check`
    refuses it elsewhere. A substring output item whose length isn't constant has a
    descriptor cell whose length is stored with CVTLW (`describeText`). `linear`
    folds `X - X` when the two read alike, so `S(I:I)` is one character long.
  - **`codegen`:** `chars.go`. MOVC3 when both lengths are constant and the
    source's is at least the destination's, MOVC5 with a blank fill otherwise;
    CMPC3 for two equal constant lengths, CMPC5 otherwise, branched on with the
    unsigned conditions (`unsigned`, and their opposites); a comparison's value is
    a branch to `MNEGL #1` or `CLRL`, the register taken after the CMPC. Each marks
    R0 to R5 or R0 to R3 changed, for the entry mask, and checks that no register
    holds a value (`holdingNothing`).
  - **FTNRTL:** nothing new: FTN$IO_ITEM already wrote text, a whole array of it
    too (Table 7-1's An).
  - **Tests:** `testdata/fortran/chars`, in both paging modes, its output worked by
    hand; `chars.for` golden in `ir` and `codegen`; sema cases for each message.
  - **Unconfirmed, ours:** the messages' wording; a bound that isn't constant
    unchecked until `/CHECK=BOUNDS` (Phase 85), a negative length becoming the
    unsigned word MOVC5 reads (MANUAL.md's differences); a comparison computed
    before its statement when it isn't a branch's condition, so `L = I .GT. 0
    .AND. S(I:I) .EQ. 'X'` compares even when I is 0 (2.3.4 allows either).
- **2026-10-30, subtask 6:** CHARACTER arguments and concatenation.
  - **`sema`:** a CHARACTER dummy compiles (`characterDummy` is gone), and so does
    any character value as an argument, a constant's among them. A character
    constant given to a numeric dummy of a routine in the file is a Hollerith
    constant, NOTIMPL (`checkReferences`, whose `routineRef` now keeps the
    arguments); one given to a routine outside the file is passed by descriptor.
    A parenthesized substring is a copy, as a variable is. `//` is `Concat`: its
    parts flattened (parentheses change nothing), constants side by side joined,
    its length their sum or 0 when one isn't known; a non-CHARACTER operand is
    BADOPERAND, as in a constant expression. A substring of a passed-length dummy
    (`passedSubstring`) checks constant bounds only against 1 (SUBLOW) and each
    other (SUBORDER).
  - **`ir`:** `lowerchars.go`. A `Mem` may be at an address computed at run time
    (`Ptr`), and `ArgPointer` is the argument list's longword itself, so a
    CHARACTER dummy's text is `@(@(arg X)+4)`, the address its descriptor holds,
    and a passed-length one's length `zext.WL(@(arg X))` (the new `Zext`,
    MOVZWL: a length is an unsigned word). A passed-length array's element's index
    is its element number times that length. Every character argument is a
    descriptor (`charArg`): a constant's text, a place's (`describeText`), a
    parenthesized one's copy; a passed-length dummy, or a whole array that is one,
    passed on with its own; a whole array a class A descriptor (`arrayDescriptor`,
    whose length, size, and A0 a passed-length array stores at run time), an
    assumed-size one its first element's class S. A concatenation's parts are
    CharMoves into a temporary one after another (`build`): a `TextCell` of the
    value's most characters (`maxLength`; a substring with a bound that isn't
    constant can have its variable's), pooled by size, or, when that isn't known, a
    class D descriptor cell given its text by LIB$SGET1_DD and freed by
    LIB$SFREE1_DD. The free is one of `lowerer.after`'s statements, added at the
    statement's end or before the terminator that ends its block (`flush`), and a
    character comparison of a dynamic string in a branch's condition is computed
    first so it can be freed before the branch (`cond`). An output item of any
    CHARACTER value goes through `describeText`. `cvt.LW(zext.WL(x))` is `x`.
  - **`codegen`:** `pointerMem`: a place at a computed address through it in
    memory by a deferred mode when it has no offset or index (`direct`:
    `@4(R0)`, `@PROG.$D1+4`), and otherwise with the address in a register,
    `1(R3)`, `(R2)[R3]`; `memNeed`, `memHeld`, and `fitMem` count and spill the
    address as they do an index. `ArgPointer` is `SUB.X(AP)`; `Zext` is MOVZWL. A
    CharMove whose two lengths are one expression is a MOVC3 with the length
    computed once. DSC$K_CLASS_D.
  - **FTNRTL:** nothing new.
  - **Tests:** `testdata/fortran/strings`, in both paging modes, its output worked
    by hand; `strings.for` golden in `ir` and `codegen`; sema cases for the
    messages and for what is no longer NOTIMPL.
  - **Unconfirmed, ours:** a whole CHARACTER array passed by a class A descriptor and
    an assumed-size one by its first element's class S (the *VAX FORTRAN User
    Manual*, which would say, isn't at hand; a routine reads only the length and
    the address of either); a declared-length dummy passed on with a new
    descriptor of its declared length; a character constant passed to a routine
    outside the file always by descriptor (VAX FORTRAN's compiler and linker make
    it a Hollerith constant for a numeric dummy, 6.1.1.4); a concatenation longer
    than 65535 characters not reported. LEN, which section 6.1.1.3's example uses
    with a passed-length dummy, is subtask 8's; `strings.for` gets along without it.
- **2026-10-30, subtask 7:** exponentiation.
  - **`sema`:** `**` compiles (`power`); it isn't folded unless its operands are
    constants and the exponent an INTEGER, as before. `powerTypes` is the survey's
    table: an INTEGER base and exponent are both of the higher type, by
    `mixedType`, so `I2**2` is a word (the author's answer 2); a REAL or COMPLEX
    base keeps its type, an INTEGER exponent widened to a longword; a REAL\*4 and
    a REAL\*8 keep their own types in D_floating (OTS$POWRD, OTS$POWDR), both G in
    G_floating; anything else both of the higher type. So a `**` Binary's operands
    have the routine's types, not the result's, the one Binary whose don't.
  - **`ir`:** `lowerpower.go`. The routine follows from the operands' IR types
    (`realPowers`, `complexPowers`); a call passes both by value, an INTEGER\*2
    converted to a longword (`Check` now refuses a value argument shorter than a
    longword, which PUSHL would misread). An INTEGER or REAL base with a constant
    exponent from 1 to 8 is multiplied out (`inlinePower`), in
    `vaxfloat.PowerBySquaring`'s order, a value used again `reusable`. The survey
    said 2 to 8; 1 is the base itself, exactly, so it's in line too. A COMPLEX
    power is always called (`complexPower`; OTS$POWCJ and its kind with a longword
    exponent, OTS$POWCC and its kind through `complexCall`): the routines round a
    product's parts once, as constant folding does (`complexOTS`), and a COMPLEX\*8
    product in line rounds each step, so a small power written in line would differ
    from its folded value.
  - **`codegen`:** nothing new: a call's value arguments and wide results were
    subtask 4's, and an `_R3` routine's R2 and R3 are saved by the entry mask.
  - **FTNRTL:** nothing new.
  - **Tests:** `testdata/fortran/powers` in both paging modes, its output worked by
    hand (powers of 3 and 1.5 exactly, the square roots and 1.5\*\*1.5 to each
    format's digits, the complex identities); `powers.for` golden in `ir` and
    `codegen`; `TestPowerTypes`; sema's NOTIMPL case for `**` replaced.
  - **Unconfirmed, ours:** an INTEGER power in line wraps on overflow (as any
    multiplication does without `/CHECK=OVERFLOW`, Phase 85) where OTS$POWJJ and
    OTS$POWII signal SS$_INTOVF; a base used again that isn't a variable,
    element, or constant is computed before its statement, so in `IF (I .NE. 0
    .AND. (10/I)**2 .GT. 1)` the quotient is computed even when I is 0 (2.3.4
    allows it, as subtask 5 noted for comparisons).
- **2026-10-31, subtask 8:** the intrinsic functions written in line.
  - **`sema`:** Decision 7's table is `intrinsics.go`'s `intrinsicTable`, Table D-3
    a row per line (generic name, specific name, argument count, argument and result
    types, how it compiles), read at start-up into `intrinsicForms`; the 219 names
    are its names now (`TestIntrinsicTable`, which also checks that each of
    `constFuncs`'s names is the table's, with the same argument count).
    `intrinsicref.go` analyzes a reference: the count (INTRINARGS), the arguments,
    and the row (`selectForm`): a generic name's rows by the arguments' type
    (`argumentType`: one kind, but for CMPLX's two; the highest type, a constant that
    fits a word beside an INTEGER\*2 being one, as `mixedType` has it, so the bit
    functions follow D.5.2's "if either I or J or both were INTEGER\*4"); of two rows
    differing only in an INTEGER result, the default integer's (footnote 10); an
    INTEGER constant that fits a word is taken by a specific name's INTEGER\*2
    argument (`IIABS(5)`); BADARG otherwise. What a row becomes: a `Convert` for the
    conversions (footnote 11's unconverted values among them; INT of an integer has
    rows to both lengths, so footnote 10 applies to it too), the operators for IAND,
    IOR, IEOR, NOT, DPROD, and the lexical comparisons, a constant for LEN of a known
    length, NWORKERS, SIZEOF, and INDEX of constants, and otherwise a new expression,
    `IntrinsicRef`, of an `IntrinsicOp`, computed in the arguments' type and
    converted to the result's (MAX1). The functions MTHRTL computes stay NOTIMPL
    (subtask 9), and so does ISHFT by a count that isn't constant, MTH$JISHFT's.
    `constNint` returns the default integer, as footnote 10 says.
  - **`ir`:** `lowerintrinsic.go`. ABS, MAX, MIN, DIM, and SIGN branch (Decision 6):
    their value goes in a temporary, through blocks of their own before the
    statement; `divert` ends a block mid-statement without the statement's end's
    frees (`after`), which wait for its end. SIGN with a constant second argument
    is decided when compiling. MOD of INTEGERs is `X-(X/Y)*Y`; NINT is a new
    operator, `Round` (CVTRxL); ZEXT is `Zext`; ISHFT by a constant is `Ash` (ASHL,
    another new operator) to the left and a `Field` (EXTZV, a new expression) to the
    right, an INTEGER\*2's done in a longword (MOVZWL) and its low word taken by
    `Low` (a third operator: MOVW, unchecked); IBITS and BTEST are `Field`s, IBSET
    and IBCLR an `Ash` mask with OR and AND; ISHFTC calls FTNRTL; ICHAR is MOVZBL of
    the first byte, CHAR a byte temporary; INDEX is a new expression, `CharIndex`
    (MATCHC), which may only be an Assign's whole value, as CharCmp may, its value
    computed from a base (the first string's length plus 1, less the second's)
    computed before it, less MATCHC's R2. CMPLX and CONJG are complex parts.
    `Check` knows the new forms.
  - **`codegen`:** CVTRxL, ASHL, MOVW for `Low` (nothing when it is the same
    register), EXTZV (`field`, a constant base in a register, since a literal can't
    be one), MATCHC (`charIndex`). An operand an instruction reads as another type
    than its own (`Low`'s, ASHL's count, EXTZV's base and size) is put in a register
    if an index reaches it (`unindexed`), since the index would be scaled by the
    wrong size.
  - **FTNRTL:** `ftn_intrinsics.mar`, FTN$JISHFTC and FTN$IISHFTC (the field written
    twice side by side in a quadword, then SIZE bits of it from SIZE-SHIFT, put
    back by INSV); vector offsets 48 and 56; GSMATCH 1,5.
  - **Tests:** `testdata/fortran/intrinsics`, in both paging modes, its output
    worked by hand; `intrinsics.for` golden in `ir` and `codegen`;
    `TestIntrinsicRefs`, `TestIntrinsicErrors`, `TestIntrinsicTable`.
  - **Unconfirmed, ours:** a generic's arguments of one kind (but CMPLX's), and a
    specific name's of its own type but for a constant that fits; LOGICAL arguments
    only where Table D-3 lists them; IZEXT's third argument type INTEGER\*2 (the OCR
    prints LOGICAL\*2 twice); the branching functions computed before their
    statement, so their arguments are evaluated even where `.AND.` would have
    skipped them (2.3.4 allows it); SIZEOF of an adjustable array NOTIMPL (its size
    is a run-time value, Phase 79's); ISHFTC's arguments must be within Table D-3's
    limits, and FTNRTL returns I for a SIZE less than 1.
- **2026-10-31, subtask 9:** MTHRTL's intrinsic functions, intrinsic names as
  arguments, and FTNRTL's routines for the names MTHRTL lacks.
  - **`sema`:** a row of `MTH` is an `IntrinsicRef` of the new `CallOp`, whose
    `Routine` is the routine called; ISHFT by a count that isn't constant is
    one too (MTH$JISHFT, MTH$IISHFT), and so is ISHFTC (FTNRTL's), so
    `CircularOp` is gone. `routines.go`: `routineName` is the survey's mapping
    from a specific name to its routine, MTH$ and the name with the format's
    letter (a REAL\*16 row's Q is H, `IIQINT` `MTH$IIHINT`; under `/G_FLOATING`
    a REAL\*8 or COMPLEX\*16 row's D is G, `CDSQRT` `MTH$CGSQRT`, `IIDNNT`
    `MTH$IIGNNT`, and `DBLE` and `SNGL` are `MTH$GDBLE` and `MTH$SNGLG`; ATAN2D's
    is ATAND2's), or FTN$ and the name for `ftnRoutines`, the 26 names the
    survey's table lists (DBLEQ and QEXTD are GBLEQ and QEXTG under
    `/G_FLOATING`). An INTRINSIC name as an actual argument is the new
    `Procedure` (`procedure`): `passedForm` is a specific name's row, or a
    generic name's specific row of the default types (an argument of the
    default integer, else a REAL\*4, a result of the default integer if
    INTEGER at all); SIZEOF is the new NOTPASSABLE. An EXTERNAL name as an
    argument stays NOTIMPL (Phase 79), and an INTRINSIC name anywhere else
    than an argument is BADUSE (it was NOTIMPL as an output item).
    `TestIntrinsicRoutines` checks every row's routine, in both formats,
    against VMS 7.1's MTHRTL symbol table (`vmsdef.ImageSymbols`), and the
    FTN$ ones against `FTNRoutines`, the list FTNRTL's test checks its vector
    against.
  - **`ir`:** `callIntrinsic` (`lowerintrinsic.go`): the arguments by
    reference, as any function's; a REAL\*16 or COMPLEX\*16 value through a
    hidden first argument, the temporary's address (Decision 5), the rest in R0
    and up. A COMPLEX value from MTHRTL is a temporary whose parts
    `complexParts` reads, and which an assignment moves whole. `Arg.Routine` is
    a routine's address passed.
  - **`codegen`:** `PUSHAB G^routine` for a routine passed; the calls needed
    nothing new.
  - **FTNRTL:** `ftn_intrinsics.mar`'s 26 routines for passed names: LEN, INDEX
    (MATCHC), ICHAR, CHAR (through the result's descriptor), the four lexical
    comparisons (one CMPC5 for all, the condition codes kept by BSBB and RSB),
    BITEST and BJTEST (BBC), IIBITS and JIBITS (EXTZV), IIBSET, JIBSET, IIBCLR,
    JIBCLR (BBSS, BBCC in R0), IZEXT and JZEXT (MOVZWL), SNGLQ, DBLEQ, GBLEQ
    (CVTHx into R0), QEXT, QEXTD, QEXTG, QFLOAT (CVTxH through the hidden
    argument), NWORKERS. Vector offsets 64 to 264; GSMATCH 1,6.
  - **Tests:** `testdata/fortran/mathfns` (F, D, H) and `mathfnsg` (G, with
    values past D's range), in both paging modes, their outputs computed apart
    from govax: each function to 80 decimal digits (Python's `decimal`, with
    series of its own for the trigonometric functions and Machin's formula for
    pi), rounded to its format's bits, and written as list-directed output
    writes it; govax's matched in every digit. `TestFortranPassedIntrinsics`
    (console) passes MTHRTL's and each of FTNRTL's routines to a MACRO routine
    that calls the routine it is given, beside each function written in line, in
    both paging modes. `mathcalls.for` golden in `ir` and `codegen`;
    `TestIntrinsicCalls`, `TestPassedIntrinsics`, `TestIntrinsicRoutines`;
    FTNRTL's `TestImage` has every new offset.
  - **Unconfirmed, ours:** a generic name passed as its specific name of the
    default types (6.3.1.1 says generic names passed are treated as specific
    ones, and no more); the FTN$ routines' names and calling sequences, which
    are govax's own; a passed IZEXT or JZEXT reading a word and CHAR a byte,
    whatever they are given.
- **2026-10-31, subtask 10:** end to end. Each type's programs came with its
  subtask (`lengths`, `reals`, `realsg`, `complex`, `complexg`, `chars`, `strings`,
  `powers`, `intrinsics`, `mathfns`, `mathfnsg`); this one adds programs that use them
  together, as a program would.
  - **Tests:** four programs in `testdata/fortran`, each run in both paging modes
    (`fortranPagingPrograms`), their outputs worked by hand line by line (the REAL\*16
    polynomial and pi simulated at 113 bits, a REAL\*4 constant's value at 24):
    `mixed.for` (a value through every type and back by Table 3-1; operations between
    every rank, LOGICAL as integer, COMPLEX\*8 with REAL\*8; a REAL\*4 constant's digits
    kept in a REAL\*8 expression; REAL subscripts and DO limits; mixed types in
    conditions; one polynomial in REAL\*8, REAL\*16, and COMPLEX), and `mixedg.for`, the
    same in G_floating; `text.for` (words split by INDEX and sorted by LLT, reversing,
    palindromes, case and a cipher through ICHAR and CHAR, letters counted, a number's
    digits, a centered title, a sentence of substrings cut to its target);
    `numeric.for` (Newton's method, a quadratic's roots by CSQRT, Simpson's rule, pi in
    F, D, and H, degrees and logarithms, statistics, compound interest, elimination,
    Euclid), whose results depending on rounding along the way are checked against
    tolerances or rounded, so that what it prints is the mathematics'. A scratch probe
    of further mixed forms (complex and REAL\*16 arrays computed from BYTE and
    INTEGER\*2 elements, COMPLEX\*16 with REAL\*16, a COMPLEX power and quotient in one
    statement, concatenation in a condition, the generic functions on mixed results)
    printed what hand computation gave, so no code changed.
  - **Not compiled yet, as planned:** a REAL\*16 FUNCTION (Phase 79, Decision 10) and
    adjustable arrays (Phase 79), so `mixed.for` has a subroutine and an assumed-size
    array instead; a main program's name used as another unit's dummy argument is
    BADUSE, by sema's rule that a unit's name names nothing else in the file
    (`text.for`'s CENTER takes STR), which F77's 18.1.2 may allow (unconfirmed).
- **2026-10-31, subtask 11:** the close-out.
  - **FORTRAN-INTERNALS.md:** a new section 11, "Every type and its
    instructions", follows a third program, AREA (a REAL\*8 function call, a
    power in line, a COMPLEX\*8 product with a REAL\*8 sum, a passed-length
    CHARACTER comparison and assignment), through the IR's types, an
    instruction chosen by its type's letter and the places that pattern
    breaks (indexed mode's scale, BLBC's longword, floating constants as data,
    unsigned character branches), register runs, mixed types, COMPLEX split in
    lowering, CHARACTER as statements, powers, and the intrinsic table; its
    IR and MACRO are the compiler's own output. FTNRTL and testing are now
    sections 12 and 13; the introduction, the pass diagram, section 10's
    registers and constants, FTNRTL's descriptors and formats, and the
    testing section's outputs are brought up to date.
  - **MANUAL.md:** chapter 14's "where things stand", section 6.5's summary of
    the subset, the command line's `govax fortran` synopsis (`--continuations`,
    `--no-i4`, `--g-floating`), and a difference: a unit's name names nothing
    else in its file. Section 14.2 and the other differences were written with
    each subtask.
  - **HELP FORTRAN:** the subset (every type and the intrinsic functions),
    FTNRTL's routines and COMPLEX and CHARACTER output, and the host shell's
    `--no-i4` and `--g-floating`.

## Where things stand

Every expression in every type compiles: BYTE, LOGICAL and INTEGER of each
length, REAL\*4, REAL\*8 in D or G, REAL\*16, COMPLEX\*8 and \*16, and
CHARACTER with substrings, concatenation, and passed-length dummies; every
operator, `**`, and every intrinsic function of Table D-3, in line, by MTHRTL,
or by FTNRTL, and passed as arguments; and each type's list-directed output.
Left for later phases, as planned: FUNCTION statements of REAL\*16,
COMPLEX\*16, and CHARACTER types and calls through dummy procedures (Phase
79, with adjustable arrays and statement functions), Hollerith arguments
(Phase 79), `/CHECK` (Phase 85), and the optimizer's clean-ups of what this
phase writes, such as multiplying by 0.0 and 1.0 or widening a value only to
round it back (Phase 84).
