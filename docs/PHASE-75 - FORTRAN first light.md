# Phase 75 — FORTRAN first light: every pass, end to end

**Status:** in progress. Subtask 1, the survey (Part A) and the design
(Part B), was written 2026-10-30 and accepted by the author the same
day, with its four corrections to the plan. Needs Phase 74 (the
FORTRAN command, source forms, FTNRTL's pipeline). Part of the FORTRAN
program laid out in `PHASE-74 - FORTRAN plan.md` (Decisions 4 to 8, 10
to 13, and 20 are first built here).

## Goal

Compile, assemble, link, and run a first FORTRAN program, going through
every pass of the compiler with a small subset of the language, so that
each pass's shape and each interface between passes is settled before
the language is widened. The subset:

- a main program (PROGRAM, END), SUBROUTINE, FUNCTION (INTEGER and
  REAL), CALL, RETURN, arguments by reference;
- INTEGER (INTEGER*4), REAL (REAL*4), and LOGICAL (LOGICAL*4) scalars
  and one-dimensional arrays with constant bounds; INTEGER, REAL,
  LOGICAL, and DIMENSION statements; implicit typing;
- assignment; `+ - * /` and unary minus; comparisons; `.AND.`, `.OR.`,
  `.NOT.`; mixed INTEGER and REAL with the conversion rules;
- logical IF, block IF/ELSE IF/ELSE/END IF, DO with a label and
  CONTINUE, GO TO, STOP, END;
- `PRINT *` and `WRITE (*,*)` of INTEGER, REAL, LOGICAL, and character
  constants (list-directed output).

The point of this phase is the architecture, not the coverage: the
programs it runs are small, and the passes are written so later phases
only add cases.

## What earlier phases leave in place

- **Phase 74:** `internal/fortran`'s driver and `diag`, the FORTRAN
  command and `govax fortran`, `internal/fortran/source` (statements
  with positions), `internal/ftnrtl` with its build, and the locator and
  LINK finding FTNRTL.
- **MACRO, LINK, RUN** as they are. LIBRTL's OTS$ conversions for
  FTNRTL's numeric output.

## Design notes

- **The lexer** (Decision 5): blank removal outside character constants,
  statement classification (assignment, keyword statement, statement
  function, the DO test), then tokens. Its doc explains why FORTRAN
  needs this order, with `DO 10 I = 1.5` and `IF(X) = 3`.
- **The AST** (`internal/fortran/ast`): one Go type per statement and
  expression kind, each with its source position. A program unit is a
  list of statements; the parser doesn't build blocks for IF and DO
  (semantic analysis matches them), which keeps the parser simple and
  puts the label rules in one place.
- **The parser** (Decision 6): one function per statement kind, a
  precedence climber for expressions. Errors recover at the next
  statement.
- **Semantic analysis** (Decision 7, the subset's part): the symbol
  table, implicit types, each expression's type, conversions inserted as
  explicit nodes, IF and DO block matching, label checks, the storage
  layout of `$LOCAL`.
- **The IR** (Decision 8): `ir.Function` of `ir.Block`s, each a list of
  statements (`Assign`, `Call`, `Jump`, `Branch`, `Return`) over typed
  expression trees. A dump format for tests. Lowering of DO to a trip
  count; of PRINT to Decision 12's calls.
- **The code generator:** instructions for the subset's operators in
  longword and F_floating forms, operand addressing (static storage,
  `@n(AP)` for dummy arguments, indexed mode for array elements),
  scratch registers R0–R5, the entry mask, long branches, the `.MAR`
  file's layout (header, storage map, psects, source lines as comments,
  descriptors and constants in `$PDATA`, as in Phase 74's sketch).
- **FTNRTL's first I/O:** `FTN$PRINT_LIST`, `FTN$WRITE_LIST` (unit `*`),
  `FTN$IO_ITEM`, and `FTN$IO_END`: a record buffer, list-directed
  output per Table 7-1 for these types (OTS$CVT_L_TI for integers,
  `OTS$$CVT_F_T_R8` for reals), and SYS$OUTPUT through RMS with
  carriage control. A unit table that later phases fill out.
- **The end-to-end harness:** `testdata/fortran/` (`name.for`,
  `name.out`); `TestFortranPrograms` compiles each with FORTRAN, then
  runs MACRO, LINK, and RUN on govax, and compares the output. It also
  checks that MACRO printed nothing.

Part B below makes each of these concrete.

## Open questions

Each is answered by the survey; the answers are Part A's "The open
questions, answered".

- **List-directed output's exact layout** (Table 7-1's field widths,
  the leading blank, line length 80).
- **The IR's form:** trees in blocks (Decision 8) versus three-address
  code.
- **Where a temporary lives:** static `$LOCAL` cells per routine,
  reused between statements.

# Part A — the survey (2026-10-30)

Section numbers are the 1988 *VAX FORTRAN Language Reference Manual*'s
(the OCR text beside its PDF) unless another book is named. What each
rule means for a pass is in brackets.

## Program units and statements

- **Program units** (1.1.1, 6.2.2, 6.2.3, 5.5): a unit is a main
  program, a FUNCTION, or a SUBROUTINE (BLOCK DATA is Phase 76's), ended
  by END. END in a main program ends the program; in a subprogram it is
  a RETURN. FUNCTION and SUBROUTINE statements must be a unit's first
  statement; a FUNCTION statement is `[type] FUNCTION name [*m]
  [([p[,p]...])]`, a SUBROUTINE statement `SUBROUTINE name
  [([p[,p]...])]` (a `*` dummy is an alternate return, Phase 79).
- **The main program's name** (4.12): PROGRAM is optional and must come
  first. Without it the main program is named **`filename$MAIN`**, the
  source file's name cut to 26 characters. Its name may not name
  anything else in the program. [Phase 74's Decision 11 said `MAIN`;
  see "Corrections to the plan", accepted.]
- **No recursion** (6.2): "A subprogram can refer to other
  subprograms, but it cannot refer to itself, either directly or
  indirectly." [Decision 10's static storage is safe: a routine's
  `$LOCAL` cells are never live in two calls at once.]
- **Statement order** (1.1.2, Figure 1-1, 4.4): the header first;
  specification statements (type declarations, DIMENSION) before
  executable ones; END last. A label may be on any statement, but only
  labels of executable statements (and FORMAT statements) may be
  referred to. [sema checks the order and the label rules.]
- **Names** (1.1.3): 1 to 31 letters, digits, `$`, and `_`, starting
  with a letter. A name names one thing in a unit; a subprogram's,
  common block's, and the main program's names are unique in the whole
  program. [Within one file sema can check the unit names.]
- **END** (5.5): "If an initial line contains only an END in the
  statement field, it is treated as an END statement even if it is
  followed by continuation lines." [The lexer looks at the initial
  line's own text, not the joined statement, for END.]

## Data types, constants, and storage

- **Sizes** (2.1.1, Table 2-1): INTEGER and LOGICAL are 4 bytes (2 with
  `/NOI4`, Phase 78), REAL 4. A numeric storage unit is 4 bytes.
- **Representation** (appendix C): INTEGER*4 is a longword in two's
  complement; REAL*4 is F_floating; a LOGICAL's value is its **bit 0**
  (C.4: set is .TRUE., clear .FALSE.; the other bits are undefined).
  [The value stored for .TRUE. is ours: see "Choices".]
- **Integer constants** (2.2.1.1): decimal digits with an optional
  sign, -2147483648 to 2147483647. A constant from -32768 to 32767 "is
  treated as an INTEGER*2 data type". [Nothing in this subset can tell:
  in an INTEGER*4 expression it is converted up, and passed as an
  argument it is a longword, whose first word is the INTEGER*2 value
  (C.2). Phase 78, which adds INTEGER*2, revisits it. 2147483648 is
  allowed only under a unary minus.]
- **Real constants** (2.2.1.2): `s.nn`, `snn.nn`, `snn.`, each
  optionally with `Esnn`, or an integer with `Esnn`; REAL*4, about 7
  significant digits, magnitude 0.29E-38 to 1.7E38. `D` and `Q`
  exponents make REAL*8 and REAL*16 constants (Phase 78). [vaxfloat
  converts the text, rounding once; a constant out of range is an error.]
- **Logical constants** (2.2.1.5): `.TRUE.`, `.FALSE.`.
- **Character constants** (2.2.1.6): apostrophes, `''` for one
  apostrophe, 1 to 2000 characters; "in a numeric context ... it is
  considered a Hollerith constant" (Phase 82). [In this subset a
  character constant is only a PRINT or WRITE item, or STOP's display.]
- **Implicit types** (2.2.2.2): I to N INTEGER, the rest REAL*4. A
  name's type may be declared once (2.2.2.1, 4.4), and not after the
  name has been used with its implicit type (4.4).
- **Type declarations** (4.4.1): `type [*n] v [*n] [,v [*n]]...`, v a
  name or an array declarator, `*n` after an array's name. Data
  initialized in a declaration (`/clist/`) is Phase 76's DATA.
- **DIMENSION** (4.5) and **array declarators** (2.2.3.1): `a(d)`,
  `d` is `[dl:]du`, lower bound 1 when omitted, du ≥ dl; a name in one
  array declarator only. Bounds that aren't constants make adjustable
  arrays (Phase 79), not allowed in a main program.
- **Subscripts** (2.2.3.2, 2.2.3.3): one per dimension; any arithmetic
  expression, a non-integer one truncated to an integer; elements are
  stored in column-major order. [The address of `A(s)` is `A + (s -
  dl) * size`. No bounds checking: `/CHECK=BOUNDS` is Phase 85's.]
- **Whole arrays** (2.2.3.5): an array's name without a subscript is
  allowed only in declarations, I/O lists, and as an actual or dummy
  argument (and COMMON, DATA, EQUIVALENCE, SAVE, NAMELIST later).
  [A whole array in a PRINT list is one item call with a class A
  descriptor (Decision 12).]

## Expressions

- **Arithmetic** (2.3.1): `**`, then `*` and `/`, then `+` and `-`;
  equal precedence left to right, except `**`, right to left.
  "VAX FORTRAN allows two consecutive operators if the second operator
  is a plus or minus", and such a sign takes the rest of the term:
  `A**-B*C` is `A**(-(B*C))`. [The precedence climber parses a unary
  sign after an operator at the additive level. `**` is NOTIMPL until
  the OTS$ powers (Phases 77, 78).]
- **An expression's type** (2.3.1.2): ranks LOGICAL 1, INTEGER*2 2,
  INTEGER*4 3, REAL*4 4 (REAL*8 5, ...). An operation's result has its
  higher-ranked operand's type; the lower one is converted first.
  Integer division truncates (`1/4` is 0), and conversion happens per
  operation, so `Y = (I/J)*X` divides as integers. A LOGICAL in an
  arithmetic context is an integer. [sema puts an explicit conversion
  node on the lower operand of each mixed operation.]
- **Relational** (2.3.3): `.LT. .LE. .EQ. .NE. .GT. .GE.`, below the
  arithmetic operators, all of equal precedence; mixed operands are
  converted to the higher rank before comparing.
- **Logical** (2.3.4): precedence below the relations is `.NOT.`, then
  `.AND.`, then `.OR.`, then `.EQV.`, `.NEQV.`, `.XOR.` (the last three
  later phases'). On LOGICAL operands the result is LOGICAL; on INTEGER
  operands, or a mix, the operation is "carried out bit-by-bit" and the
  result is INTEGER. "Some subexpressions may not be evaluated if the
  compiler can determine the result by testing other subexpressions":
  short-circuiting is allowed. Consecutive periods only before `.NOT.`
  (`A .AND. .NOT. B`).

## Assignment

- **Arithmetic and logical assignment** (3.1, 3.2, Table 3-1): the
  value is converted to the target's type. For this subset's types:
  INTEGER or LOGICAL from INTEGER or LOGICAL, assigned as is; INTEGER or
  LOGICAL from REAL, **truncated**; REAL from INTEGER or LOGICAL, "append
  fraction (.0)". A character value can't be assigned to a numeric
  target. [The VAX's CVTFL truncates and CVTLF rounds a longword too
  big for 24 bits, which is the "append .0" of an exact conversion
  where one exists.]

## Control statements

- **CALL** (5.1): `CALL sub [([a][,[a]]...)]`; the arguments agree in
  number, order, and type with the dummies. Null arguments (`CALL
  S(A,,B)`) pass 0 (Decision 11). A constant, an expression, or a
  parenthesized variable is an actual argument whose value the callee
  sees but whose change the caller never does.
- **CONTINUE** (5.2): does nothing; the usual DO terminal.
- **DO** (5.3.1, 5.3.1.1): `DO s [,] v = e1, e2 [, e3]`, v an INTEGER
  or REAL variable, e3 1 when omitted, not zero. The parameters are
  converted to v's type; v is set to e1; the iteration count is
  `[(e2 - e1 + e3)/e3]` ([e] truncation toward zero), and the body is
  skipped when it is zero or negative (`/NOF77`'s one trip is a later
  option). After each pass, v += e3, the count drops by one, and the
  loop goes on while it is positive, so v ends one step past its last
  value (the manual's examples: `DO 100 K=1,50,2` runs 25 times, K is
  51 after). Changing e1, e2, e3 inside the loop doesn't change the
  count; changing v is not allowed. The terminal statement may not be
  a GO TO, arithmetic IF, block IF statement, END, RETURN, or DO.
  Nested loops may share a terminal statement, which only the innermost
  loop may jump to (5.3.1.3). A jump into a loop from outside it is
  forbidden, except from the loop's own *extended range* (5.3.1.4: code
  a jump out of the loop reached, which jumps back in). [The count and
  the step are each kept in a cell of their own, computed once.]
- **GO TO** (5.6.1): to an executable statement's label in the same
  unit.
- **Logical IF** (5.7.2): `IF (e) st`, st any executable statement but
  a block IF statement, DO, END DO, or another logical IF.
- **Block IF** (5.7.3): `IF (e) THEN`, `ELSE IF (e) THEN`, `ELSE`,
  `END IF`; an ELSE block is followed by END IF; blocks nest only
  wholly. "IF (e) THEN I = J" is the logical IF `IF (e) THENI = J`
  (the manual's own note). Labels on ELSE IF and ELSE can't be referred
  to; a jump to END IF's label comes only from the block just before
  it. Control may leave a block but never enter one from outside, nor
  go from one block to another (5.7.3.1), and a DO loop and its terminal
  statement lie in one block.
- **RETURN** (5.9): only in a subprogram. `RETURN i` is an alternate
  return (Phase 79).
- **STOP** (5.10): `STOP [disp]`, disp a character constant or one to
  five digits, which STOP displays; "FORTRAN STOP" without it.
  [FTN$STOP, Phase 74, takes the display's descriptor.]

## Subprograms and arguments

- **Association** (6.1, 6.1.1): actuals agree with dummies in order,
  number, and type. The last value a subprogram assigns a dummy is the
  actual's. A dummy array is associated with an array or an element of
  one of the same type.
- **Character constants as actuals** (6.1.1.4): a numeric dummy may
  take a character constant (it becomes a Hollerith constant). [Phase
  82's; in this subset a character constant passed to a routine is
  NOTIMPL.]
- **Mechanisms** (6.1.2.1, Table 6-1): by default numeric and logical
  values and arrays go by reference, character values by descriptor.
  %VAL, %REF, %DESCR (Phase 79).
- **Functions** (6.2.2): the value is what the function last assigned
  to its own name; the name's type is the function's, declared in the
  FUNCTION statement, by a type statement in the unit, or implicitly,
  and the caller must agree.

## List-directed output

- **The statements** (7.6, 7.1.1.2): `PRINT * [, iolist]` writes to the
  implicit output device; `WRITE (*,*) iolist` (or `WRITE (UNIT=*,
  FMT=*)`) to the default unit. Which file each of those is comes from
  logical names the VAX FORTRAN User Manual documents (FOR$PRINT and
  the like, Phase 81); in this phase both are SYS$OUTPUT.
- **The formats** (7.3.1.2, Table 7-1): LOGICAL*4 is `L2`, INTEGER*4
  `I12`, REAL `1PG15.7E2`, CHARACTER `An` (n its length, "without
  delimiting apostrophes", an inner apostrophe written once). "Each
  output record begins with a space for carriage control. Each output
  statement writes one or more complete records. Each individual output
  value is contained within a single record, with the exception of
  character constants longer than one record length."
- **G editing** (8.3.6.4, Table 8-2): `Gw.d` writes a value m with
  0.1 ≤ m < 10**d as F editing in w-4 columns with d significant digits
  (0.1 ≤ m < 1 is `F(w-4).d`, 1 ≤ m < 10 `F(w-4).(d-1)`, and so on to
  10**(d-1) ≤ m < 10**d, `F(w-4).0`) followed by 4 blanks, and other
  values (m < 0.1, m ≥ 10**d) as `Ew.d`. The OCR lost Table 8-2's middle
  rows; they are FORTRAN-77's.
- **The scale factor** (8.3.7): `1P` has no effect when G chooses F
  editing; with E editing it puts one digit before the point and d
  after it (the manual's `1PE12.3` of -270.139 is `-2.701E+02`). So
  `1PG15.7E2` shows 7 significant digits in F form and 8 in E form:
  3.4 is `   3.400000    `, 1.0E-5 is ` 1.0000000E-05`.
- **L editing** (8.3.8): T or F at the field's right end: `L2` is
  ` T`.
- **Carriage control** (8.4, Table 8-5): the record's first character
  is its carriage control (`' '` a new line, `'0'` a blank line first,
  `'1'` a new page, `'+'` overprint, `'$'` a prompt, NUL none), under the
  default `CARRIAGECONTROL='FORTRAN'`.

## The calling standard (*OpenVMS Calling Standard*, chapter 2)

- **Registers** (2.1.1): R0 and R1 return values and are scratch; R2
  to R11 are preserved, by the entry mask only ("so that any stack
  unwinding done by the condition-handling mechanism restores all
  registers correctly"); AP, FP, SP, PC are the CALL's. FP may not be
  changed.
- **The frame** (2.2, Figure 2-1): CALLS/CALLG build it; local storage
  is what the routine subtracts from SP, freed by RET. [Not used here:
  storage is static (Decision 10).]
- **The argument list** (2.3, 2.4): CALLS pushes the count after the
  arguments, pushed last to first; the count is the low byte of the
  first longword, and a routine reads it as a byte (`MOVZBL`, `TSTB`,
  `CMPB`); a null argument is a longword 0; the list is read-only to
  the callee. Order of evaluation is the language's: by-reference
  arguments may be evaluated left to right and their addresses pushed
  right to left.
- **Function values** (2.5): in R0 if 32 bits or fewer, R0:R1 if 64,
  otherwise through a hidden first argument (Phase 78, CHARACTER and
  REAL*16 and COMPLEX*16 functions).
- **Data types** (chapter 4's atomic types): a longword logical is
  `LU` (DSC$K_DTYPE_LU, 4), a longword integer `L` (8), F_floating `F`
  (10), text `T` (14). [FTNRTL tells an item's type by its descriptor's
  DTYPE, so a LOGICAL*4 item is `LU` and an INTEGER*4 `L`.]

## What govax gives the passes

- **MACRO** takes everything the design below writes, checked on
  2026-10-30 by assembling a probe with govax's MACRO (no messages):
  local symbols with periods (`PROBE$MAIN.I`) and `$`; an argument
  offset as a symbol (`SUB.X = 4`, then `@SUB.X(AP)`); `@8(AP)[R0]`;
  a static array element as `PROBE$MAIN.A-4[R0]`; `.LONG ^X00004080`
  for 1.0; `DECL`, `BLEQ` over a `BRW`; `PUSHAL` of an indexed operand;
  `.END PROBE$MAIN`. MACRO doesn't lengthen branches (Decision 8), and
  the byte-displacement branches include `SOBGTR`, `AOBLEQ`, `BLBS`,
  and `BLBC`, not only the `Bcc`s.
- **MACRO and floating literals:** in a floating instruction MACRO reads
  `#n` as a floating value (`MOVF #^X4080,R0` would load 16512.0), so
  a REAL constant is never an immediate operand: it is a `.LONG` in
  `$PDATA` (Decision 16), except 0.0 (`CLRF`) and a whole move of one
  (`MOVL #^X...`, the bits copied as an integer).
- **OTS$** (`internal/librtl`): `OTS$CVT_L_TI` (LIBRTL offset ^X588)
  and `OTS$$CVT_F_T_R8` (^X568, a JSB routine working in its caller's
  frame, `librtl/otscvtfor.go`: the exact decimal digits of a value,
  rounded for E or F as asked, unrounded for G) are both in LIBRTL's
  vector, so FTNRTL links against them.
- **RMS's FORTRAN carriage control** (`rms/ftncc.go`, Phase 72): a
  stream opened indirectly through a process-permanent file with
  `FAB$M_FTN` in its record attributes has each record's first byte
  turned into the terminal's line model. FTNRTL's SYS$OUTPUT is such a
  stream. What happens when SYS$OUTPUT is a file (a procedure's
  `/OUTPUT=`, a subprocess's log) is subtask 7's question.
- **The DSC$ constants** are in `vmsdef` (`DSC$K_DTYPE_L` 8, `_LU` 4,
  `_F` 10, `_T` 14, `DSC$K_CLASS_S` 1, `_A` 4), but govax's STARLET
  has no `$DSCDEF` (Phase 74's close-out), so the `.MAR` file defines
  the few it uses, as FTNRTL's module does.
- **The console test helpers** (`internal/console/ftnrtl_test.go`:
  `eachLoader`, `shareFindConsole`, `shareFindRun`, `shareRun`) already
  write a file to a volume, run MACRO, LINK, and RUN, and capture a
  run's output and `$STATUS`; the harness uses them.

## The open questions, answered

- **List-directed output's layout.** Settled by the manual: each record
  starts with a blank (7.3.1.2), and the fields are Table 7-1's, G
  editing by Table 8-2 with 8.3.7's scale factor. Not settled, and so
  ours: the record length (80, the blank included; the User Manual's),
  that items follow one another with no blank between them beyond their
  fields' own (character items included), and that zero, below 0.1 by
  Table 8-2's rule, prints in E form (` 0.0000000E+00`). All three are
  marked unconfirmed (there is no VAX FORTRAN to check them, Phase 74's
  review, answer 5) and go into MANUAL.md's "Differences from VAX
  FORTRAN".
- **The IR's form: trees in blocks**, as Decision 8 says. The VAX's
  instructions take memory operands with any addressing mode for each
  of three operands, so `I = J*K + L` is two instructions with one
  register (`MULL3 J,K,R0`, `ADDL3 R0,L,I`); a tree shows the code
  generator which operands are leaves and can be named in place, where
  three-address code would make every interior value a temporary and
  leave the generator to fold them back. Control flow, the part trees
  don't express, is the blocks'. The design below confirms it on the
  subset's statements; Phase 84's optimizer works on blocks either way.
- **Where a temporary lives.** Two kinds, decided by lowering:
  *cells*, named `$LOCAL` longwords, for values that live across
  statements or whose address is taken (a DO loop's count and step;
  an expression or constant passed by reference; an I/O item's value),
  and *registers* for the values inside one expression, which the code
  generator assigns. A DO loop's cells are its own. A statement's
  argument and item cells come from a per-routine pool, given back at
  the statement's end, so the routine's `$LOCAL` holds as many as its
  busiest statement needs.

## Corrections to the plan (accepted 2026-10-30)

1. **The main program's default name is `filename$MAIN`** (4.12), not
   `MAIN` as Decision 11 and the sketch say. The `.MAR` file's module is
   named for the first program unit (Decision 3), so a file whose first
   unit is an unnamed main program makes module `SQUARES$MAIN`.
2. **The sketch's loop closing** (`SOBGTR SQUARES.$C1,SQUARES.$1`) is a
   byte branch, out of range for any loop body over 127 bytes, and its
   `BLEQ` to the loop's end the same. First light closes a loop with
   `DECL` and a branch around a `BRW` (Part B, "The code generator").
   `SOBGTR`/`AOBLEQ` are Phase 84's peephole, where sizes are known.
3. **`MOVL #SS$_NORMAL,R0`** needs `$SSDEF` or a global LINK resolves;
   the main program returns `MOVL #1,R0` with the comment `SS$_NORMAL`.
4. **The begin call takes its control block**: `PUSHAB SQUARES.$IO1`,
   `CALLS #1,G^FTN$PRINT_LIST`, as Decision 12 says, not the sketch's
   `CALLS #0`.

None changes a decision's substance; the first is a fact the plan
missed. The author accepted all four on 2026-10-30, and Phase 74's
Decision 11 and its sketch's introduction now say so.

# Part B — the design

The pipeline after Phase 74's source pass, with the type each pass
hands on:

```
source.Program --lex--> []lex.Statement --parse--> *ast.File
  --sema--> *sema.Program --ir.Lower--> []*ir.Routine --codegen--> MACRO lines
```

`fortran.Compile` runs them in order, every pass reporting to the one
`diag.List`. A pass runs only if the passes before it reported no
error, except that the parser goes on after a statement's error (it
recovers at the next statement) and sema goes on after one unit's.

## The lexer (`internal/fortran/lex`, subtask 2)

Input: one `source.Statement`. Output: a `lex.Statement`: its kind, its
tokens, its label, and the source statement (for positions). Three
steps, which the package's `doc.go` teaches in this order:

1. **Squeeze.** Remove blanks and tabs outside character constants,
   keeping for each remaining character its offset in the statement's
   text (`source.Statement.Position` maps that to file, line, and
   column). An unterminated character constant is an error (`NOQUOTE`).
   Hollerith constants (`nH...`, whose text keeps its blanks) are found
   by the source pass's rule and lexed; the parser reports them NOTIMPL
   until Phase 82.
2. **Classify.** In this order:
   - text starting `IF(` or `ELSEIF(`: find the matching `)`. If `=`
     follows it, the statement is an assignment (to an array element
     named IF). Otherwise it is a block IF (the rest is exactly `THEN`),
     an arithmetic IF (the rest is `label,label,label`; Phase 79), or a
     logical IF, whose rest is classified as a statement of its own
     (`IF(X)GOTO10`, and the manual's `IF(E)THENI=J`);
   - an `=` at parenthesis level 0: a DO statement if the text starts
     with `DO` and a `,` at level 0 follows the `=` (`DO10I=1,5`), else
     an assignment (`DO10I=1.5` assigns to `DO10I`, `CALLX=1` to
     `CALLX`, `IF(X)=3` was caught above). A statement function
     definition looks like an assignment to an array element; sema tells
     them apart;
   - otherwise a keyword statement: the longest keyword in a table of
     **every** VAX FORTRAN statement keyword that starts the text
     (`ENDIF` before `END`, `ELSEIF` before `ELSE`, `GOTO`,
     `DOUBLEPRECISION`, ...), so a statement outside the subset is named
     in its NOTIMPL message ("the READ statement is not implemented
     yet") rather than being a syntax error. A type keyword followed by
     `FUNCTION` is a FUNCTION statement only as a unit's first statement:
     `INTEGERFUNCTIONF(10)` elsewhere declares an array `FUNCTIONF`. END
     is recognized by the initial line's text (5.5). Nothing matching is
     `UNRECSTMT`.
3. **Tokenize** the text after the keyword: names (letters, digits, `$`,
   `_`), integer and real constants, character constants, the dotted
   operators and logical constants (`.EQ.`, `.AND.`, `.TRUE.`, ...),
   `** * / + - // ( ) , = : '`, and `*` in `PRINT *`. The hard case is
   a period after digits: `1.EQ.2` is the integer 1 and `.EQ.`, `1.E5`
   is a real, so after `nn.` a known dotted operator wins over an
   exponent. A name is greedy only because the keyword is already
   split off: `DO10I=1,5` gives the keyword DO, then `10`, `I`, `=`,
   ...

Messages: `NOQUOTE`, `HOLLEND` (a Hollerith count past the statement's
end), `UNRECSTMT`, `BADCHAR` (a character FORTRAN doesn't use, outside
a constant). A period that starts no dotted operator is the operator
`.`, left to the parser (Phase 82's record fields use it).

## The AST (`internal/fortran/ast`, subtask 3)

Plain syntax: no types, no symbols. Every node has its `diag.Position`,
and every statement its label (0 for none).

```go
type File struct{ Units []*Unit }

type Unit struct {
	Header Stmt   // *ProgramStmt, *FunctionStmt, *SubroutineStmt, or nil (an unnamed main program)
	Body   []Stmt // the statements after the header, in order, *EndStmt last
}
```

Statements: `ProgramStmt{Name}`, `FunctionStmt{Type *TypeSpec; Name;
Params}`, `SubroutineStmt{Name; Params}`, `TypeStmt{Type TypeSpec;
Entities []*Declarator}`, `DimensionStmt{Entities}` (a `Declarator` is
a name, a `*n`, and its dimensions, each `Bound{Lower, Upper Expr}`),
`AssignStmt{Target, Value Expr}`, `CallStmt{Name; Args []Expr}` (a nil
argument is a null one), `GotoStmt{Target LabelRef}`, `IfStmt{Cond;
Then Stmt}`, `IfThenStmt{Cond}`, `ElseIfStmt{Cond}`, `ElseStmt`,
`EndIfStmt`, `DoStmt{Target LabelRef; Var *Name; Start, Limit, Step
Expr}`, `ContinueStmt`, `StopStmt{Display Expr}`, `ReturnStmt{Alt
Expr}`, `EndStmt`, `PrintStmt{Format FormatSpec; Items []Expr}`,
`WriteStmt{Unit, Format; Items}` (a `FormatSpec` is `*`, a label, or an
expression; only `*` gets past sema in this phase), and
`NotImplStmt{What string; Executable bool}`, which keeps an unsupported
statement's label so that a jump to it isn't also UNDEFLBL.

Expressions: `IntConst{Text}`, `RealConst{Text}`, `LogicalConst{Value}`,
`CharConst{Value}`, `Name{Name}`, `Apply{Name; Args}` (an array element
or a function reference: `A(I)` can't be told apart without
declarations, so sema decides), `Unary{Op; X}`, `Binary{Op; X, Y}`,
`Paren{X}` (kept: a parenthesized variable passed as an argument is a
copy, and parentheses fix the order of evaluation).

**The dump** (`ast.Dump`, for golden tests) is one line per statement,
the line number and label first, expressions as prefix lists:

```
1      PROGRAM SQUARES
2      INTEGER I
3      DO 10 I = 1, 5
4      PRINT * I (* I I)
5  10  CONTINUE
6      END
```

## The parser (`internal/fortran/parse`, subtask 3)

A function per statement kind, chosen by the lexer's kind
(`parseDo`, `parsePrint`, ...), each consuming the statement's tokens
and building its node; tokens left over are an error at the first one
(`EXTRATEXT`). Expressions by precedence climbing over Part A's table,
with the two special rules: `**` binds right to left, and a sign after
another operator takes the rest of the term. Units are formed here: a
header statement or the first statement after an END starts one, END
ends it, and statements after a file's last END without one are
`NOEND`. An error is reported and the parser goes to the next
statement, so one compile reports every syntax error.

Messages: `SYNTAX` (expected X, found Y), `EXTRATEXT`, `NOEND`,
`BADLABELREF`.

## Semantic analysis (`internal/fortran/sema`, subtask 4)

Per unit:

1. **Declarations.** The symbol table: each name's class (variable,
   array, dummy argument, the function's own result, external function
   or subroutine), type (`sema.Type`: kind and size; implicit from the
   first letter unless declared), dimensions, and dummy position. A
   declaration after the name was used, a second declaration, a second
   array declarator, or a declaration after the first executable
   statement is an error.
2. **Expressions** become typed trees (`sema.Expr`): `Const` (its value:
   an int32, an F_floating's bits from vaxfloat, a bool, a string),
   `Var`, `Elem` (the array and one subscript per dimension, each
   converted to INTEGER), `FuncRef` (the function and its arguments),
   `Unary`, `Binary`, `Compare`, `Convert`. Each has its type, and each
   mixed operation gets a `Convert` on its lower operand (2.3.1.2). An
   `Apply` is an `Elem` if its name is an array, else a function
   reference (an external function of the name's type). A name used as
   a function that is one of FORTRAN's intrinsic functions (ABS, MOD,
   SQRT, ...) is NOTIMPL until Phase 78, not an external. Constant
   subexpressions are folded with vaxfloat, so they round as the CPU
   would (Decision 16).
3. **Statements** become a structured tree (`sema.Stmt`): an `If` with
   its arms and else, a `Do` with its body, so lowering never matches
   blocks again. Matching follows Part A: IF/ELSE IF/ELSE/END IF nest
   wholly; a DO's body ends at the statement with its label, and loops
   sharing that statement nest (the innermost holds it). An assignment
   whose target is `F(X)` with F not an array is a statement function
   definition: NOTIMPL until Phase 79.
4. **Labels.** Each defined once; each referred-to label on an
   executable statement (`NOTEXEC` otherwise); no jump into an IF block
   from outside, to an ELSE or ELSE IF label, or to a shared DO terminal
   from outside the innermost loop. A jump into a DO loop from outside
   has no message, since it may be from the loop's extended range.
5. **Storage.** Each local variable and array gets its `$LOCAL` offset
   (in order of first appearance, each longword aligned); a dummy has
   none (it is at `n(AP)`); a function has a cell for its result.

Messages (identifiers ours, Decision 19): `UNDEFLBL`, `DUPLABEL`,
`NOTEXEC`, `INTOBLOCK`, `DOTERM` (a terminal statement 5.3.1 forbids),
`DONEST`, `DOVAR` (not INTEGER or REAL), `DOSTEP0` (a zero constant
step), `NOENDIF`, `NOIF` (ELSE, ELSE IF, or END IF with no IF),
`NODO` (a DO's label never comes), `DUPDECL`, `LATEDECL`, `DUPDIM`,
`SUBSCRIPTS` (wrong number), `NOTARRAY`, `MIXTYPES` (character with
numeric), `BADTARGET`, `MAINRETURN` (RETURN in a main program),
`INTRANGE`, `REALRANGE`, and NOTIMPL for the rest.

## The IR and lowering (`internal/fortran/ir`, subtask 5)

```go
type Routine struct {
	Name   string     // the entry's name: SQUARES, ISQ
	Kind   Kind       // Main, Subroutine, Function
	Result Type       // a function's type
	Cells  []*Cell    // $LOCAL: variables, arrays, the result, counts, temporaries
	Data   []*Datum   // $PDATA: constants, descriptors, I/O control blocks
	Blocks []*Block   // the code, the first block the entry
}

type Block struct {
	Name  string      // B1, B2, ...
	Label int         // the FORTRAN label at its start, or 0
	Stmts []Stmt      // Assign{Dst Mem; Src Expr}, Call{Routine; Args; Result *Cell}
	End   Terminator  // Jump{To}, Branch{Cond; True, False}, Return{Value}, NoReturn
	Lines [2]int      // the source lines it came from, for the comments
}
```

An expression is a tree of `Const{Type; Bits}`, `Load{Mem}`,
`Op{Op; Type; X, Y}` (add, sub, mul, div, neg, and, or, not, cvt), and
`Cmp{Op; Type; X, Y}` (a LOGICAL). A `Mem` is a place in memory: a
base (a cell or dummy argument n), a constant byte offset, and an
optional INTEGER index tree scaled by the element's size, which is the
VAX's indexed mode. An argument (`Arg`) is by reference (a `Mem`'s
address), by value, or by descriptor (a `Datum`). **Calls are
statements, never inside expressions**: lowering takes each function
reference out of its expression into a `Call` before the statement,
its value in a cell (or straight into the target when the statement is
`J = F(...)` and the types agree), so the code generator never has a
value in a register across a CALLS.

Lowering makes explicit what FORTRAN leaves implicit:

- **Array elements**: `A(s)` is `Mem{A, -dl*size, s}` (`A-4[R0]` for
  dl = 1); a dummy array's base is the address in its argument slot.
- **DO** becomes: v = e1; count = (e2 - e1 + e3) / e3 (computed in v's
  type, a REAL count converted by truncation); if count ≤ 0, skip; the
  body; v += e3; count -= 1; loop while count > 0. Constant parameters
  fold the count (`DO 10 I = 1, 5` is 5, the test gone).
- **IF** blocks become branches; a logical IF is a branch around one
  statement, or a branch to the target for `IF (e) GO TO n`.
- **Arguments**: a variable, element, or dummy by its address; a
  constant by the address of a `Datum` in `$PDATA` (read-only, so a
  callee that assigns it gets an access violation, as on VMS); an
  expression or a parenthesized variable by a temporary cell's address.
- **PRINT and WRITE** become Decision 12's calls: `FTN$PRINT_LIST` or
  `FTN$WRITE_LIST` with the statement's control block, an
  `FTN$IO_ITEM` with a descriptor per item (a static item's descriptor
  a `Datum`, a dummy's a cell filled at run time, an expression's value
  in a cell), and `FTN$IO_END`.
- **STOP** is `Call FTN$STOP` with the display's descriptor, then
  `NoReturn`; END and RETURN are `Return` (a function's result cell as
  the value, a main program's 1).

**The dump** (`ir.Dump`) for SQUARES:

```
routine SQUARES main
  cell I    L  $LOCAL+0  variable I
  cell $C1  L  $LOCAL+4  DO 10's count
  cell $T1  L  $LOCAL+8  temporary
  datum $IO1  PRINT * control block
  datum $D1   descriptor L S I
  datum $D2   descriptor L S $T1
B1:                                     ; line 3
  assign I = 1
  assign $C1 = 5
  jump B2
B2:                                     ; line 4: DO 10's body
  call FTN$PRINT_LIST(ref $IO1)
  call FTN$IO_ITEM(ref $D1)
  assign $T1 = mul.L(I, I)
  call FTN$IO_ITEM(ref $D2)
  call FTN$IO_END()
  jump B3
B3: L10                                 ; line 5
  assign I = add.L(I, 1)
  assign $C1 = sub.L($C1, 1)
  branch gt.L($C1, 0) B2 B4
B4:                                     ; line 6
  return 1
```

## The code generator (`internal/fortran/codegen`, subtask 6)

- **Instructions** for the subset: `MOVL`, `CLRL`, `INCL`, `DECL`,
  `ADDL2/3`, `SUBL2/3`, `MULL2/3`, `DIVL2/3`, `MNEGL`, `MCOML`
  (.NOT.), `BICL3` with `MCOML` (.AND.), `BISL3` (.OR.), `CMPL`,
  `TSTL`; `MOVF`, `CLRF`, `ADDF2/3`, `SUBF2/3`, `MULF2/3`, `DIVF2/3`,
  `MNEGF`, `CMPF`, `TSTF`; `CVTLF` and `CVTFL` (which truncates, as
  Table 3-1 wants); `PUSHAL`, `PUSHL`, `CALLS`, `RET`, `BRW`, the
  `Bcc`s, `BLBS`/`BLBC`.
- **Selection** by tree: a leaf is an operand in place (`I`, `#5`,
  `@ISQ.N(AP)`, `SQUARES.A-4[R0]`); `Assign{x, Op(add, x, #1)}` is
  `INCL x`; an operation whose operands are leaves writes its result
  straight to the target (`MULL3 I,I,SQUARES.$T1`). An interior value
  goes in a register, chosen by a Sethi–Ullman count of each subtree's
  needs from R0 to R5 (an index register too); a tree needing more
  spills to a temporary cell. R2 to R5, when used, go in the entry
  mask (the calling standard's preserved registers).
- **Conditions** in a branch are evaluated for control: a comparison is
  `CMPx` and a branch; `.AND.`, `.OR.`, `.NOT.` of conditions branch
  early (short-circuit, which 2.3.4 allows); a LOGICAL value is tested
  with `BLBS`/`BLBC` (bit 0, C.4). As a value (`L = A .GT. B`) a
  comparison stores -1 or 0.
- **Branches** are always long enough: a jump is `BRW`, and a
  conditional branch to a block that isn't next is the opposite
  condition's branch around a `BRW` (`BLEQ SQUARES.$5` / `BRW
  SQUARES.$1` / `SQUARES.$5:`), whose own displacement is 3. A routine
  over 32K bytes would need `JMP`; MACRO reports it if one ever is.
- **Names** (Decision 10): a routine's entry its FORTRAN name; its
  variables `unit.name`; its dummy offsets symbols (`ISQ.N = 4`, used
  as `@ISQ.N(AP)`); FORTRAN label n `unit.Ln`; the compiler's labels
  `unit.$n`, its cells `unit.$Cn` (counts), `unit.$Tn` (temporaries),
  `unit.$Kn` (constants), `unit.$Dn` (descriptors), `unit.$IOn` (I/O
  control blocks); a name past 31 characters gets a numbered form, in
  the storage map.
- **Layout**: the header (Phase 74's); then per unit, a storage map
  comment, its `$LOCAL` and `$PDATA` contributions, and its `$CODE`;
  in the code, before each statement's instructions, every source line
  not yet written up to the statement's last line, so comments and
  declarations appear where they are in the source; `.END` with the
  main program's entry, if the file has one. The constants the file
  uses (`DSC$K_DTYPE_L = 8`, ...) are defined at the top.

SQUARES (the plan's example) comes out so:

```
DSC$K_DTYPE_L = 8                       ; longword integer
DSC$K_CLASS_S = 1                       ; scalar
        .PSECT  $LOCAL,PIC,CON,REL,LCL,NOSHR,NOEXE,RD,WRT,LONG
SQUARES.I:      .BLKL   1               ; I
SQUARES.$C1:    .BLKL   1               ; DO 10's count
SQUARES.$T1:    .BLKL   1               ; temporary
        .PSECT  $PDATA,PIC,CON,REL,LCL,SHR,NOEXE,RD,NOWRT,LONG
SQUARES.$IO1:   ...                     ; PRINT *'s control block
SQUARES.$D1:    .WORD   4               ; descriptor of I
                .BYTE   DSC$K_DTYPE_L,DSC$K_CLASS_S
                .ADDRESS SQUARES.I
SQUARES.$D2:    ...                     ; descriptor of $T1
        .PSECT  $CODE,PIC,CON,REL,LCL,SHR,EXE,RD,NOWRT,LONG
;      1        PROGRAM SQUARES
        .ENTRY  SQUARES,^M<>
;      2        INTEGER I
;      3        DO 10 I = 1, 5
        MOVL    #1,SQUARES.I            ; I = 1
        MOVL    #5,SQUARES.$C1          ; count: (5-1+1)/1
SQUARES.$1:                             ; DO 10's body
;      4           PRINT *, I, I*I
        PUSHAB  SQUARES.$IO1
        CALLS   #1,G^FTN$PRINT_LIST
        PUSHAQ  SQUARES.$D1
        CALLS   #1,G^FTN$IO_ITEM
        MULL3   SQUARES.I,SQUARES.I,SQUARES.$T1
        PUSHAQ  SQUARES.$D2
        CALLS   #1,G^FTN$IO_ITEM
        CALLS   #0,G^FTN$IO_END
;      5  10    CONTINUE
SQUARES.L10:
        INCL    SQUARES.I               ; I = I + 1
        DECL    SQUARES.$C1             ; one pass fewer
        BLEQ    SQUARES.$2              ; none left
        BRW     SQUARES.$1              ; another pass
SQUARES.$2:
;      6        END
        MOVL    #1,R0                   ; SS$_NORMAL
        RET
        .END    SQUARES
```

and a function, `INTEGER FUNCTION ISQ(N)` / `ISQ = N*N` / `END`:

```
ISQ.N = 4                               ; N is argument 1
        .ENTRY  ISQ,^M<>
        MULL3   @ISQ.N(AP),@ISQ.N(AP),ISQ.ISQ
        MOVL    ISQ.ISQ,R0              ; the function's value
        RET
```

## FTNRTL's first I/O (`internal/ftnrtl`, subtask 7)

- **Entries**, appended to the vector (16, 24, 32, 40), GSMATCH's minor
  number raised by one per entry: `FTN$PRINT_LIST(icb)`,
  `FTN$WRITE_LIST(icb)`, `FTN$IO_ITEM(desc)`, `FTN$IO_END()`, the last
  returning a status in R0 (SS$_NORMAL here; FOR$_ values with Phase
  80's error model).
- **The control block** (`icb`, a `unit.$IOn` in `$PDATA`): a word for
  its size and a word of flags (which of UNIT, FMT, REC, ERR, END,
  IOSTAT the statement has), the unit (-1 for `*`), and room the later
  phases define. Its layout is settled in subtask 7 and documented in
  the module; a unit given by an expression (Phase 81) will be a second
  argument, since `$PDATA` can't hold it.
- **The modules**: `ftn_units.mar` (the unit table, a NOSHR psect;
  this phase's one entry is the default output unit: a FAB and RAB for
  SYS$OUTPUT, created at first use with `RAT=FTN`, `RFM=VAR`, `FAC=PUT`,
  `$CREATE` and `$CONNECT`) and `ftn_listout.mar` (the statement's
  state, the record buffer, and the list-directed formats: `I12` by
  OTS$CVT_L_TI, `L2` from bit 0, `An` copied, `1PG15.7E2` from
  `OTS$$CVT_F_T_R8`'s digits, the G and E choice and the rounding done
  in FTNRTL). An item that won't fit the record's 80 columns starts a
  new one (a character item longer than a record is split); the begin
  call starts the record with its blank, the end call writes it.
- **Tests**: MACRO programs calling the entries directly, as Phase 74's
  `TestFTNRTL` does, with the field widths, a record that wraps, and
  REAL values on each side of G's ranges.

## The end-to-end harness (subtask 8)

`testdata/fortran/name.for` with `name.out`, the expected terminal
output (and `$STATUS`). `TestFortranPrograms` (`internal/console`)
writes each source to a volume, runs FORTRAN, MACRO, LINK, and RUN
through the console as `TestFTNRTL` does, checks that FORTRAN reported
nothing and MACRO and LINK printed nothing, and compares RUN's output.
Every program runs with paging off, and one with paging on too
(Decision 20's "both paging modes for a subset"). The first programs:
squares (DO, PRINT), factorials (INTEGER and REAL arithmetic, mixed
conversion), primes (nested DO, logical IF, GO TO, MOD by hand), and a
subroutine and a function (arguments by reference, a changed actual, an
expression argument, an array argument), plus block IF and STOP.

## Choices of ours, unconfirmed

To go into the progress log as each is built, and into MANUAL.md's
"Differences from VAX FORTRAN":

- `.TRUE.` is stored as -1 and `.FALSE.` as 0 (only bit 0 is defined,
  C.4); .NOT. is `MCOML`, so a LOGICAL stays -1 or 0, and .AND. and
  .OR. are the bitwise operations 2.3.4 gives integers.
- A constant actual argument is in read-only `$PDATA`; a callee that
  assigns it gets an access violation.
- Integer overflow is not detected (the entry mask leaves IV clear, as
  a default `/CHECK=NOOVERFLOW` would; Phase 85's `/CHECK`); integer
  divide by zero traps, as the VAX always does (Phase 70).
- List-directed output's record length (80), items without separating
  blanks, zero in E form (above).
- `PRINT *` and `WRITE (*,*)` both write SYS$OUTPUT.
- A REAL move is `MOVF` (a reserved operand faults, as it would in an
  arithmetic instruction), not `MOVL`.
- An integer constant is a longword wherever it is stored or passed.
- LOGICAL functions come with INTEGER and REAL ones: the value is in R0
  either way.

## Subtasks (provisional)

1. **Survey** (done 2026-10-30, accepted): the subset's rules in the
   manual (chapters 2, 3, 5, 6; Table 3-1's conversions; Table 7-1),
   the calling standard's VAX chapter, and a written design of the AST
   and IR in this doc.
2. **The lexer and statement classifier**, with tests of FORTRAN's
   hard cases.
3. **The AST and the parser** for the subset, with golden AST dumps.
4. **Semantic analysis** for the subset: symbols, types, conversions,
   block matching, labels, storage.
5. **The IR and lowering**, with golden IR dumps.
6. **The code generator** for the subset, with golden MACRO fragments
   and an assemble-cleanly check.
7. **FTNRTL's list-directed output** to SYS$OUTPUT, tested from MACRO.
8. **End to end:** the harness and the first programs (squares,
   factorials, primes, a subroutine and a function).
9. **Close-out:** `docs/FORTRAN-INTERNALS.md`'s first chapters (the
   pipeline, lexing, parsing), MANUAL.md, HELP.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-10-30:** Subtask 1 done, for review: the survey (Part A) and
  the design (Part B).
  - Read the manual's chapters 1 to 6 for the subset, 7.3.1.2 and
    Table 7-1, 8.3.6.4 (G editing, Table 8-2), 8.3.7 (the scale
    factor), 8.3.8, 8.4, and appendix C; and the *OpenVMS Calling
    Standard*'s chapter 2 (extracted from its PDF with `pdftotext`) and
    its atomic data types.
  - The open questions answered: the leading blank is the manual's
    (7.3.1.2); the record length, the separation of items, and zero's
    form are ours, unconfirmed; the IR is trees in blocks; temporaries
    are `$LOCAL` cells for what outlives an expression and registers
    within one.
  - Four corrections to Phase 74's plan, for the author's review: the
    main program's default name is `filename$MAIN` (4.12); the sketch's
    `SOBGTR` and `BLEQ` are byte branches, replaced by long forms;
    `SS$_NORMAL` is written `#1`; the begin call takes its control
    block.
  - A MACRO probe (not committed) assembled every operand form the
    design writes without a message.
  - Found along the way: MACRO reads `#n` in a floating instruction as a
    floating value, so REAL constants are `$PDATA` longwords;
    `OTS$$CVT_F_T_R8` and `OTS$CVT_L_TI` are in LIBRTL's vector, so
    FTNRTL can call them.
- **2026-10-30:** The author accepted the survey, the design, and the
  four corrections. Phase 74's Decision 11 now names `filename$MAIN`,
  and its sketch points here for the settled form. Subtask 2 is next.
- **2026-10-30:** Subtask 2 done: the lexer (`internal/fortran/lex`).
  - `Lex` turns the source pass's statements into `lex.Statement`s:
    `Kind`, `Label`, `Tokens` (each with its kind, text, and offset in the
    statement's text), `Then` (a logical IF's statement, lexed as one of
    its own), `FunctionType` (a typed FUNCTION's type keyword), `Offset`,
    and `Body` (where a keyword statement's text starts after its
    keyword). `Dump` is the one-line form tests compare.
  - Three steps, each in its own file, as Part B designed: `squeeze.go`
    (blanks out outside constants, capitals, each character's offset and
    whether it is in a constant, with the source pass's rules for
    character constants, Hollerith counts, and READ's `1'10`);
    `lex.go`'s `classify` (IF and ELSE IF by what follows the condition,
    the `=` and DO-comma test, then the longest keyword of `kinds.go`'s
    table of every VAX FORTRAN statement, typed FUNCTION only at a unit's
    start, DO WHILE); `tokenize.go` (names, numbers with the dotted
    operator test, character, Hollerith, and typeless constants, `**` and
    `//`, the other operators, `<` and `>` for Phase 80's variable format
    expressions).
  - Two cases the design didn't foresee: a DO statement's label followed
    by a name starting E, D, or Q and a digit (`DO 10 E1 = 1, 5`
    squeezes to `DO10E1=1,5`, whose `10E1` is a real constant), and a
    type length before such a name (`REAL*8 D1`). The tokenizer takes
    digits only there (`modeLabel`, `modeLengths`); in a declaration a
    `*` after a number is still a repeat count (`X/3*1.5E2/`).
  - FORMAT statements aren't tokenized (F8.2 is one edit descriptor):
    their specification is read from `Body` by Phase 80's parser.
  - The END rule (5.5) needed the initial line's text:
    `source.Statement.Initial`.
  - Choices of ours: `INTEGER FUNCTIONX` at a unit's start is a FUNCTION
    statement, though a main program could begin with that declaration;
    an empty statement (a label alone) is kind `Empty`, left to the
    parser.
  - Messages: NOQUOTE, HOLLEND, UNRECSTMT, BADCHAR. `Compile` now runs
    the lexer after the source pass, so the FORTRAN command reports them.
  - Left for later: VAX's old `PARAMETER X = 1` (no parentheses) is an
    assignment to PARAMETERX by the `=` test (Phase 76); `ASSIGN 10 TO
    I` tokenizes `TOI` as one name, for Phase 79's parser to split.
  - DUNGEON V3.2A (local only): all 51,717 statements of its nine
    sources, with `/CONTINUATIONS=40`, lex without a message, every one
    classified.
  - Tests: `TestClassify` (71 statements: the DO and IF tests, keywords
    as names, typed FUNCTIONs, lengths, numbers and dotted operators,
    constants, the other keyword statements), `TestTokenKinds`,
    `TestUnitStarts`, `TestEndWithContinuation`, `TestLabels`,
    `TestPositions` (across a continuation line), `TestMessages`,
    `TestDirective`, `TestKindTable`, `TestFormatBody`.
