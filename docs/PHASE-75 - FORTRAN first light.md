# Phase 75 — FORTRAN first light: every pass, end to end

**Status:** planned (2026-10-29). Needs Phase 74 (the FORTRAN command,
source forms, FTNRTL's pipeline). Part of the FORTRAN program laid out
in `PHASE-74 - FORTRAN plan.md` (Decisions 4 to 8, 10 to 13, and 20
are first built here).

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

## Open questions

- **List-directed output's exact layout** (Table 7-1's field widths,
  the leading blank, line length 80): the manual gives the formats;
  whether VAX FORTRAN starts each record with a blank for carriage
  control is the User Manual's, unconfirmed.
- **The IR's form:** trees in blocks (Decision 8) versus three-address
  code. Trees suit the VAX's rich operand modes; this phase confirms it
  on real code before Phase 76 builds on it.
- **Where a temporary lives:** static `$LOCAL` cells per routine
  (simple, as in the sketch), reused between statements.

## Subtasks (provisional)

1. **Survey:** the subset's rules in the manual (chapters 2, 3, 5, 6;
   Table 3-1's conversions; Table 7-1), the calling standard's VAX
   chapter, and a written design of the AST and IR in this doc.
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
