# Phase 112 — Fortran 95

**Status:** planned 2026-11-22; subtask 1, the survey (Part A), done 2026-11-22, awaiting the
author's review. Phase 99's question 2 left Fortran 95's additions to an
optional Phase 112, the author chose go at Phase 111's close-out, and the phase was widened
to the Fortran errata the same day. At the errata survey's review the author split it:
Fortran 95 is this phase, and the errata, with the survey (its Part A), is Phase 113
(`PHASE-113 - Fortran errata.md`, "The author's answers"). A skeletal doc: subtask 1 is the
survey, reviewed before any implementation, as Phases 100 to 110 began. Needs Phases 99 to
111.

## Goal

Fortran 95's additions to Fortran 90 compiled beside it, in the same compiler, under `/F90`:

- **FORALL**, the statement and the construct, with masks, nested FORALL, and WHERE inside;
- **PURE and ELEMENTAL** procedures, external, internal, and module ones, PURE's constraints
  checked, an ELEMENTAL reference with array arguments scalarized;
- **`NULL()`**, `=> NULL()` initialization of pointers and pointer components, and **default
  initialization** of derived types' components (static, frame, automatic, allocated, and
  INTENT(OUT) objects; a module's text carrying them);
- **WHERE inside WHERE** and **ELSEWHERE with a mask** (F95ONLY today);
- the **intrinsic changes**: MAXLOC's and MINLOC's `DIM=`, CEILING's and FLOOR's `KIND=`,
  NULL's `MOLD=`, **CPU_TIME**, and whatever else the survey finds;
- **zero widths** on output, `I0`, `B0`, `O0`, `Z0`, `F0.d`, in FTNRTL under the F90 flag;
- **zero-sized explicit-shape arrays** (`REAL A(0)`), Fortran 90's own rule that govax
  refuses today, under `/F90` only;
- **`/STANDARD=F90`** judging the widened language;
- a compiler that recovers from a statement it doesn't know without losing the unit's
  structure (today PURE's UNRECSTMT cascades into AFTERCONTAINS, ENDKIND, TWOMAINS).

Nothing VAX FORTRAN or Fortran 90 compiles changes: `TestNoF90Unchanged` and every corpus
program's output hold.

## Decisions from the review (2026-11-22)

From the author's answers in Phase 113's doc ("The author's answers"):

1. **Sources**: the HP Fortran manual (`~/Documents/Technical Doc/VMS/VMS-FORTRAN-LANGUAGE.txt`,
   beside its PDF), which describes Fortran 95 with DEC's extensions; gfortran `-std=f95` the
   witness and the backup where the manual is unclear or DEC's. No Fortran 95 standard text is
   at hand; where the manual's form is DEC's, gfortran's `-std=f95` is the standard's.
2. **`/F90` takes in Fortran 95.** There is no separate F95 mode or `/STANDARD=F95` keyword:
   Fortran 95's additions are part of what `/F90` compiles, and `/STANDARD=F90` judges by
   Fortran 90 plus them. F95ONLY retires.
3. **Zero-sized explicit-shape arrays under `/F90` only**: VAX FORTRAN's BOUNDORDER stays
   under `/NOF90`.
4. **No VMS probe of the language**: there is no VAX Fortran 95 (as there was none for Fortran
   90). Phase 113's probe round may ask what VMS's FORRTL does with a zero width, if anything
   here wants the answer.

## The seed list

From Phase 113's survey, A.4 (each checked 2026-11-22 by compiling one program: none is in),
with the items the split brought along. Subtask 1 makes it whole from the HP manual.

1. FORALL, statement and construct: UNRECSTMT. On Phase 102's scalarizer and overlap rules
   (a FORALL is an array assignment over a named index set; every right side computed before
   any left side is stored).
2. PURE and ELEMENTAL: UNRECSTMT. ELEMENTAL references as intrinsic elementals are
   (`ir`'s `elementalCall`); PURE procedures allowed in FORALL and specification expressions.
3. `NULL()`, `=> NULL()`, default initialization: NOTIMPL (`sema/derived.go`'s "a component's
   default initial value"), and a module's text unable to carry a structure with initial
   values (`sema/modwrite.go`; Phase 113's A.3.2). Storage cleared or set at its start as
   Phase 107's `clearRecords` clears pointer components.
4. CPU_TIME: a subroutine over `$GETJPI`'s CPUTIM, in FTNRTL.
5. WHERE in WHERE, masked ELSEWHERE: F95ONLY, `sema/where.go`'s three sites.
6. MAXLOC/MINLOC `DIM=` and CEILING/FLOOR `KIND=` (KEYWORDNAME today), NULL's MOLD=, and
   Fortran 95's other intrinsic changes the manual lists.
7. Zero widths: OUTCONERR today (`ftn_edit.mar`, `ftn_float.mar`).
8. `/STANDARD=F90` widened (decision 2); the deleted features (a REAL DO variable, a branch to
   END IF from outside, PAUSE, ASSIGN and assigned GO TO, H editing) are already flagged
   OBSOLESCENT under it.
9. Zero-sized explicit-shape arrays (Phase 113's A.3.5): BOUNDORDER today, `/F90` too.
10. Recovery from an unrecognized statement (Phase 113's A.1.11).

Already Fortran 95's in govax: allocatable locals deallocated at return (Phase 99's Decision
8), every pointer disassociated at the start (Phase 107's answer 2).

## Open questions (for subtask 1)

*Answered by the survey's recommendations in Part A's A.5, for the author's review.*

1. **Deleted features under `/STANDARD=F90`**: Fortran 95 deletes what Fortran 90 called
   obsolescent. Keep the OBSOLESCENT message for them, or a new one saying "deleted in Fortran
   95"? They stay compiled either way (VAX FORTRAN has them).
2. **`/NOF90`**: FORALL, PURE, and ELEMENTAL aren't VAX FORTRAN's. Compile them in a fixed-form
   `/NOF90` unit too, as `::` declarations are (Phase 100), or only under `/F90`?
3. **CPU_TIME's resolution and start**: the process's CPU time since it started (VMS's 10 ms
   ticks), or since the image started? The standard leaves it to the processor.
4. **Default initialization and SAVE**: a local of a type with default initialization is
   implied SAVE in Fortran 95 only when it is given an initial value explicitly; the survey
   is to confirm the manual's reading against gfortran.

## Subtasks (provisional)

*Superseded, once the author accepts it, by Part A's A.4.*

1. **The survey** (done 2026-11-22; Part A, revised list A.4): the HP manual's Fortran 95 additions read against govax and gfortran, a
   table of each and its subtask, a probe set of programs for gfortran; the open questions.
2. Unrecognized statements recovered from; PURE, ELEMENTAL, and FORALL lexed and parsed.
3. Zero-sized explicit-shape arrays under `/F90`.
4. `NULL()` and pointer initialization.
5. Default initialization of components, the module text among it.
6. PURE: the constraints.
7. ELEMENTAL procedures and their references.
8. FORALL, the statement.
9. FORALL, the construct (nesting, WHERE inside, PURE references).
10. WHERE in WHERE and masked ELSEWHERE; F95ONLY retired.
11. The intrinsic changes and CPU_TIME.
12. Zero widths in FTNRTL.
13. `/STANDARD=F90` widened; listings, cross references, and the debugger on the new forms.
14. Programs end to end (`testdata/fortran90`, held to gfortran `-std=f95` by `compare.py`).
15. The close-out: MANUAL.md's 14.13, FORTRAN-INTERNALS.md's section 37, HELP FORTRAN.

## Part A — the survey (subtask 1, 2026-11-22)

Method: the HP manual's Fortran 95 material read whole (its 1.3, "Fortran 95 Features",
and every page it points to; Appendix A's deleted and obsolescent lists; each intrinsic
entry marked "a Fortran 95 feature"); a probe program per group, `testdata/f95` (its
README), built with `gfortran -std=f95` (16.2) for the witness's answers, kept as
`name.gfortran`; then each compiled by govax (`govax fortran`, `/F90`), and the two that
compile, or compile once a Fortran 95 word is taken out, run through MACRO, LINK, and RUN
on a scratch volume. Nothing was changed in govax.

### A.1 What Fortran 95 adds (the manual's list, checked against gfortran)

The manual's 1.3 is the whole list; nothing else in its text is marked Fortran 95 but the
entries listed here. Each is the standard's, gfortran `-std=f95` accepting it, except
where the row says.

| # | Addition | Manual | gfortran `-std=f95` |
|---|---|---|---|
| 1 | FORALL statement and construct (triplets, a mask that may use the indexes, a named construct, nested FORALL and WHERE inside, pointer assignment as the body; every right side computed before any left side is stored; a construct is its statements one after another, each whole) | 4.2.5 | as the manual |
| 2 | PURE prefix: functions and subroutines, external, internal, module, and in interface bodies; dummies' INTENT required (IN for a function's); no SAVE or initialized locals; globals and INTENT(IN) dummies never defined or made targets; no external I/O, PAUSE, STOP; only PURE procedures called; every intrinsic function and MVBITS pure | 8.5.1.2 | as the manual |
| 3 | ELEMENTAL prefix: scalar dummies, no POINTER, `*`, or dummy procedures, a scalar non-pointer result, an explicit interface at every reference; array actuals make an array reference, element by element; PURE's rules too | 8.5.1.3 | as the manual |
| 4 | An elemental defined assignment in an array assignment and in WHERE (4.2.4's "only if the routine implementing the defined assignment is elemental") | 4.2.4 | 2, 4, 26, 28 (`elemasg`) |
| 5 | A PURE function in a specification expression (the manual's "Pure functions are allowed in specification statements"; the standard's specification functions) | 1.3 | as the manual (`misc95`) |
| 6 | `NULL([MOLD])`: as a pointer assignment's target, an initial value (`=> NULL()`), a component's default, in a structure constructor, as an actual argument, in DATA (`DATA P /NULL()/`) | 9.4.111, 5.5 | as the manual, but a structure constructor with `NULL()` in a PARAMETER is Fortran 2003's |
| 7 | Default initialization of components (`= expr`, `=> NULL()` for a pointer component, an array component from a constructor or one scalar); applies even if the type's components are PRIVATE; applies to INTENT(OUT) dummies; doesn't imply SAVE; an explicit initial value overrides it; a component of a type with defaults gets the outer default | 3.3.1, 3.3.2 | a local of such a type initialized at each call (`nullinit`, 1995 both times) |
| 8 | Pointer initialization, `INTEGER, POINTER :: P => NULL()` (implies SAVE, as any initial value) | 3.3.1 | as the manual |
| 9 | WHERE inside WHERE; ELSEWHERE with a mask (`(.NOT. m1) .AND. m2`, cumulative); named WHERE constructs, the name allowed on ELSEWHERE and required on END WHERE | 4.2.4 | as the manual (`where95`) |
| 10 | END INTERFACE with its generic name | 8.9.2 | as the manual |
| 11 | MAXLOC and MINLOC with `DIM=` | 9.4.96, 9.4.101 | as the manual |
| 12 | CEILING and FLOOR with `KIND=` | 9.4.23, 9.4.52 | as the manual |
| 13 | CPU_TIME (TIME), any real kind, INTENT(OUT), negative if no time can be had | 9.4.33 | as the manual |
| 14 | Zero field widths on output: `I0`, `B0`, `O0`, `Z0`, `F0.d` (and `Iw.m` forms such as `I0.4`): the least width with no asterisks | 1.3, 11.3 | `[42]`, `[1010]`, `[FF]`, `[3.14]`, `[-.500]`, `[0005]` (`zerowid`) |
| 15 | Comments, `!` to the end of the line, in namelist input | 1.3, 10.3 | as the manual |
| 16 | Allocatable arrays deallocated at a procedure's end | 6.2.1 | already govax's |
| 17 | Deleted: ASSIGN and assigned GO TO, an assigned FORMAT, a jump to END IF from outside, H editing, PAUSE, a REAL or DOUBLE PRECISION DO variable (all still compiled) | A.1 | warnings under `-std=f95` |
| 18 | Newly obsolescent: an alternate return, the arithmetic IF (both carried over), assumed-length character functions, `CHARACTER*n`, the computed GO TO, DATA among executable statements, fixed source form, a shared DO termination or one on a statement other than END DO or CONTINUE (carried over), statement functions | A.2 | warnings with `-Wall -std=f95`, not all |
| 19 | SIGN of a negative zero, and printing −0.0 | 1.3 | **nothing to do**: the VAX formats have no negative zero (its bit pattern is the reserved operand) |

Not Fortran 95, though the manual lists them beside it: MAXLOC's and MINLOC's `KIND=`
(DEC's, Fortran 2003's; gfortran refuses `SIZE(..., KIND=)` under `-std=f95` the same
way) and the HPF_LIBRARY routines. **Not planned.**

### A.2 What govax does with each today

| # | Addition | govax today (probe) | Where |
|---|---|---|---|
| 1 | FORALL | UNRECSTMT, then SYNTAX, TWOMAINS, ENDNONAME as the unit's structure goes (`forall`, 17 messages) | `lex/kinds.go`, `lex/f90stmt.go` |
| 2, 3 | PURE, ELEMENTAL | UNRECSTMT on the FUNCTION or SUBROUTINE statement, then AFTERCONTAINS, ENDKIND, TWOMAINS, INTERFACESTMT, ATTRDUMMY for each such unit (`pure`, 40 messages; `misc95`, 21) | `lex/lex.go`'s prefix test (RECURSIVE only) |
| 4 | Elemental defined assignment | UNRECSTMT (the ELEMENTAL); without the word, RECORDASSIGN and NOTSCALAR, rightly | `sema/operators.go` |
| 5 | Specification functions | BOUNDREF ("arrays and functions aren't allowed there") | `sema/arrays.go`'s `boundFunction`, `sema/charlength.go` |
| 6 | NULL() | `P => NULL(P)` POINTERFUNC; `=> NULL()` in a declaration EXTRATEXT; `DATA P /NULL()/` POINTERATTR; `P => NULL()` with no MOLD not tried separately (NULL isn't in the table) | `sema/intrinsics.go`, `sema/pointer.go`, `parse`'s declarations |
| 7 | Default initialization | NOTIMPL, "a component's default initial value", at each `=` in a TYPE; `=> NULL()` there EXTRATEXT; a module's text can't carry it (`modwrite.go`'s "a structure with initial values in a module") | `sema/derived.go:241`, `sema/modwrite.go:870` |
| 8 | `P => NULL()` initialization | EXTRATEXT | the parser's entity list |
| 9 | Nested WHERE, masked ELSEWHERE, named WHERE | F95ONLY (three sites), NOTCONSTRUCT on the name, EXTRATEXT on the names after ELSEWHERE and END WHERE, then WHEREBODY | `sema/where.go`, `sema/constructs.go` |
| 10 | END INTERFACE name | EXTRATEXT | `parse/units.go` |
| 11 | MAXLOC/MINLOC DIM= | KEYWORDNAME (the table's row has ARRAY, MASK) | `sema/intrinsics.go:435`, `ir/lowerarrayfunc.go`'s `locate` |
| 12 | CEILING/FLOOR KIND= | KEYWORDNAME | `sema/intrinsics.go:462`, `constfuncs.go`'s `wholeFunction` |
| 13 | CPU_TIME | not tried alone (the program stopped earlier); not in the subroutines' table | `sema/intrinsicsub.go`, `ftn_system.mar` |
| 14 | Zero widths | compiled (FORMAT's check takes `I0`); every one OUTCONERR at run time, 8 of 8 | `ftn_edit.mar`, `ftn_float.mar` |
| 15 | Namelist comments | SYNERRNAM at the `!` (`&grp` in lower case is fine) | `ftn_namelist.mar` |
| 16 | Deallocation at return | done (Phase 99's Decision 8, Phase 104) | — |
| 17, 18 | Deleted and obsolescent | the deleted six and the carried-over obsolescent ones flagged OBSOLESCENT under `/STANDARD=F90`, "obsolescent in Fortran 90"; the five newly obsolescent ones not flagged | `sema/standard.go`'s `obsolete`, `obsolescentStatements` |
| — | Zero-sized explicit-shape arrays (the seed's 9) | BOUNDORDER for `A(0)`, `B(5:1)`, `C(3,0)` | `sema/arrays.go:214`, `sema/structure.go:682` |

### A.3 Findings, and what they change

1. **The recovery comes first.** Every probe with PURE, ELEMENTAL, or FORALL loses its
   unit's structure at the first one, so nothing after it can be judged. Subtask 2 makes the
   prefixes and FORALL statements the lexer knows, with the parser's units intact; and an
   unknown statement starting with a word the lexer knows as a prefix, in general, kept as
   a `BadStmt` that doesn't end or begin a unit (Phase 113's A.1.11). PURE and ELEMENTAL go
   with RECURSIVE in any order and with a type in front or behind (the manual's `type
   [keyword]`, `keyword [type]`), in `lex.Statement` beside `Recursive`.
2. **Five items the seed list hadn't**: named WHERE constructs (9's third part), END
   INTERFACE's generic name (10), comments in namelist input (15), specification functions
   (5); and an elemental defined assignment (4), which falls to ELEMENTAL's subtask.
3. **NULL is a function sema doesn't know at all.** It belongs with pointer assignment, not
   the intrinsic table's rows: its type is its context's (the manual's table, 9.4.111), so
   sema should read `NULL()` and `NULL(MOLD)` where a pointer's value goes (`=>`'s right
   side, a declaration's `=>`, a component's default, a constructor's pointer component, an
   actual argument to a pointer dummy, DATA) as a disassociated value of that pointer's
   kind, and refuse it elsewhere. Lowering is Phase 107's NULLIFY (a scalar's longword 0, a
   descriptor's A0 0).
4. **Default initialization is two things.** The bytes: a type's default value is a
   record constant (Phase 105's `Const` with `Raw`), each component's default folded into
   its place, a pointer component's zero, a component of a type with defaults that type's
   bytes, an explicit default overriding them. Where they go: static storage that is
   initialized once (a main program's, a module's, a SAVE'd local, COMMON isn't allowed)
   gets them as `$LOCAL`'s initial bytes, as DATA does (`sema/data.go`'s `Initial`); an
   unsaved local is set at **every entry** (gfortran, A.1's 7), as Phase 107's
   `clearRecords` clears frame records, but by a copy from a `$PDATA` image rather than a
   clear (MOVC3; MOVC5 with a zero fill where the bytes are mostly zero); ALLOCATE of such a
   type copies it into each element (FTNRTL's clear bit is then the wrong tool: the IR
   copies after the call); an INTENT(OUT) dummy gets it at entry; a function result of such
   a type at entry. So a non-recursive unit's static locals change from "set once" to "set
   at entry" only for types with defaults; nothing else moves. The module text (A.2's 7)
   needs `modwrite.go` to write a component's default as its constant text and `modread.go`
   to read it back.
5. **A specification function is a call at entry.** Automatic arrays' bounds and lengths
   are computed at entry already (Phase 104's `ir/lowerautomatic.go`, `lowerlength.go`); a
   PURE function's reference there is a `FuncRef` like any, with the dummies' cells
   already filled. sema's `boundFunction` widens to a PURE (or ELEMENTAL) function with an
   explicit interface; it stays refused in a constant expression.
6. **FORALL is Phase 102's machinery with named indexes.** A FORALL statement is an array
   assignment whose index variables are the statement's own (`$In` symbols with the
   triplets' bounds, the scalarizer's leaves given the FORALL's names, `lowerer.bound`),
   the mask a WHERE-like test per index tuple, and the overlap rule Phase 102's
   `ArrayOrder` (apart, ordered, or a temporary). The difference from an array assignment:
   the left side's subscripts are expressions of the indexes, not sections, so overlap
   must compare the two sides' subscript expressions for each index; where it can't tell, a
   temporary of the index set's size (the mask computed once into a logical temporary,
   then the right sides, then the stores) is always right. A construct is its statements
   in turn, each a FORALL statement with the same header; a nested FORALL adds indexes; a
   WHERE inside is a mask on the elements; a pointer assignment as the body is Phase 107's
   `arrayPointerAssign`'s scalar form per element. The index names are construct entities:
   a FORALL index hides a variable of its name only inside (sema's scope chain, Phase 100,
   a scope per FORALL, as a statement function's is). The index set's size is known at run
   time only, so the temporaries are Phase 104's `runtimeTemp`.
7. **ELEMENTAL references reuse `elementalCall`** (Phase 108's MVBITS): the scalarizer
   calls the routine once per element, each argument an element, a scalar argument as is.
   An elemental function's result is a scalarizer leaf. Overlap: an elemental
   subroutine's INTENT(OUT)/(INOUT) actuals against the other actuals (the standard's
   rule that the result is "as if applied in any order" needs no temporary unless an
   INTENT(OUT) actual overlaps an INTENT(IN) one; ArrayOrder's test).
8. **PURE's constraints are a walk over a unit's statements**, like `/STANDARD`'s
   (`sema/standard.go`): an assignment's target, a DO variable, an input item, ALLOCATE's
   objects, the actuals to INTENT(OUT) dummies, `=>` targets, each checked against "is a
   global or an INTENT(IN) dummy" (a COMMON or module variable, a host's, a dummy without
   OUT); external I/O, PAUSE, STOP; references to procedures without a PURE interface
   (every intrinsic is pure). A new message per kind, NOTPURE ("%s can't %s in a PURE
   procedure"). A PURE interface must match a PURE procedure (INTERFACEDIFF's widening),
   and a procedure passed to a PURE dummy must be PURE.
9. **WHERE's nesting** is `sema/where.go`'s and `ir/lowerwhere.go`'s: each level's mask the
   AND of its control mask and its own, evaluated once at the construct's start; a masked
   ELSEWHERE's mask `(.NOT. m1) .AND. m2` with each later one's control mask growing
   (`.NOT. m1 .AND. .NOT. m2 .AND. m3`). The construct name joins `sema/constructs.go`'s
   stack (NOTCONSTRUCT today) with WRONGNAME and MISSINGNAME as for IF.
10. **The intrinsics**: MAXLOC and MINLOC with DIM= return an array of rank n−1, the
    reductions' run-time DIM= machinery (Phase 104, subtask 8) with `locate`'s comparison;
    CEILING and FLOOR with KIND= are a conversion's width (the `KIND=` rule of Phase 100's
    conversions, `intrinsickeys.go`); CPU_TIME is a row of the subroutines' table and an
    FTNRTL routine per kind (`FTN$CPU_TIME_F` to `_H`) over `$GETJPI`'s JPI$_CPUTIM (10 ms
    ticks), divided by 100.
11. **Zero widths and namelist comments are FTNRTL's**, under the F90 flag (control
    block bit 4, Phase 109), so VAX FORTRAN's OUTCONERR for `I0` stays without it. A zero
    width is the field's digits with no padding (`I0` the number's digits and sign, `Iw.m`
    with w 0 at least m digits, `F0.d` the integer part's digits, a sign, the point, d
    digits). gfortran's `F0.3` of −0.5 is `-.500` (the optional leading zero left out, the
    least width); recommended: the same, so `compare.py` can witness it. Namelist input
    takes `!` as the start of a comment to the record's end, outside a character value, in
    both the `&` and `$` forms (`ftn_namelist.mar`'s `FTN$$LI_SKIP`).
12. **`/STANDARD=F90`**: the new statements and forms need `statementRows` and the walk's
    rows marked `f90` (decision 2: Fortran 95 is part of what F90 judges), and the
    obsolescent list becomes Fortran 95's (A.5's question 1).
13. **Zero-sized explicit-shape arrays** (the seed's 9) touch more than the bound check: a
    zero extent must survive the element-number arithmetic (`ElementNumber`), a zero
    `$LOCAL` size, a descriptor's zero extent and multipliers, the scalarizer's loops
    (already trip-count based), SIZE/SHAPE/LBOUND/UBOUND (LBOUND of a zero extent is 1 in
    Fortran 90, not the declared lower bound: gfortran's `lbound(b)` for `B(5:1)` is 1),
    and the I/O list. A zero-sized array in COMMON or EQUIVALENCE takes no space.
14. **Nothing here asks for a VMS probe** (decision 4): no VAX Fortran 95 exists, and
    FORRTL's answer to `I0` would only say VAX FORTRAN's OUTCONERR, which govax keeps
    without the flag.

### A.4 The subtasks, revised

1. The survey (this Part A).
2. **Recovery and syntax**: an unrecognized statement kept without breaking its unit
   (A.3's 1); PURE and ELEMENTAL prefixes with RECURSIVE and a type in any order (lexer,
   `ast.FunctionStmt`/`SubroutineStmt`, interface bodies); FORALL's statement and construct
   parsed (`ast.ForallStmt`, END FORALL, names); END INTERFACE's generic name; WHERE's
   construct names parsed. Each new form NOTIMPL by name in sema until its subtask.
3. **Zero-sized explicit-shape arrays** under `/F90` (A.3's 13).
4. **NULL()** (A.3's 3): every context of A.1's 6, pointer initialization (A.1's 8),
   DATA.
5. **Default initialization** (A.3's 4), the module text with it.
6. **PURE**: the constraints (A.3's 8), PURE interfaces, specification functions (A.3's 5).
7. **ELEMENTAL** procedures and references (A.3's 7), elemental defined assignment and
   operators in array assignments and WHERE.
8. **FORALL, the statement** (A.3's 6).
9. **FORALL, the construct**: nesting, WHERE inside, pointer assignment, PURE references.
10. **WHERE** in WHERE, masked ELSEWHERE, named WHERE (A.3's 9); F95ONLY retired.
11. **The intrinsics** (A.3's 10): MAXLOC/MINLOC DIM=, CEILING/FLOOR KIND=, CPU_TIME.
12. **FTNRTL**: zero widths, namelist comments (A.3's 11).
13. **`/STANDARD=F90`** widened, Fortran 95's deleted and obsolescent lists (A.3's 12,
    question 1); listings, cross references (FORALL indexes, PURE in the attributes
    column), and the debugger on the new forms (a FORALL index has no storage of its own
    unless the temporaries give it one; a default-initialized object reads as any).
14. **Programs end to end**: `testdata/f95`'s probes finished into `testdata/fortran90`,
    with outputs worked by hand, held to gfortran by `compare.py`'s `PROGRAMS90`; a program
    of some size using them together.
15. The close-out: MANUAL.md's 14.13, FORTRAN-INTERNALS.md's section 37, HELP FORTRAN,
    `TestNoF90Unchanged` and the corpus held.

The order puts the syntax first (nothing else can be judged without it), then the
storage items that FORALL's and ELEMENTAL's tests will use (NULL, default values), then
the constraints PURE needs before ELEMENTAL and FORALL lean on it.

### A.5 Questions for the author

1. **Deleted features under `/STANDARD=F90`** (the open question 1). Recommended: a new
   message, `%FORT-I-DELETED, deleted in Fortran 95: %s`, for the six of A.1's 17, under the
   OBSOLESCENT keyword (one switch for both); OBSOLESCENT's text becomes "obsolescent in
   Fortran 95", and its list adds the newly obsolescent ones (A.1's 18): the computed GO
   TO, statement functions, DATA among executable statements, an assumed-length CHARACTER
   function, `CHARACTER*n` (the length form, not `CHARACTER*(*)` dummies). Fixed source form
   flagged once per file, at its first statement, rather than per line, or left out?
   Recommended: once per file.
2. **`/NOF90`** (the open question 2). Recommended: FORALL, PURE, ELEMENTAL, NULL(), named
   and nested WHERE, END INTERFACE's name, default initialization, and the intrinsics'
   new keywords compiled under `/NOF90` too, as Fortran 90's statements are (Phase 99:
   `/F90` settles only the texts the two read differently; none of these is read
   differently by VAX FORTRAN, FORALL's `(...)` followed by an assignment being no
   variable). Under `/F90` only: zero-sized arrays (decision 3), zero widths and namelist
   comments (the F90 flag, as Phase 109's edits).
3. **CPU_TIME** (the open question 3). Recommended: the process's CPU time (JPI$_CPUTIM, as
   VMS keeps it, so it counts from the process's start, not the image's), in seconds, in
   10 ms steps, in the argument's kind. Only differences mean anything (the standard's
   words), so either start serves; the process's is one `$GETJPI` with no state.
4. **Default initialization and SAVE** (the open question 4): answered by the manual (3.3.2,
   "does not imply the SAVE attribute") and gfortran (A.1's 7): an unsaved local of such a
   type is set at every entry, A.3's 4. **Nothing to decide** unless the author prefers
   otherwise.
5. **`F0.d`'s leading zero.** Recommended: gfortran's form, `-.500` and `.500` (the least
   width; the standard leaves the zero optional), so the witness agrees.
6. **MAXLOC's and MINLOC's KIND=** (DEC's, in the manual): recommended **not planned** (it
   is Fortran 2003's; KEYWORDNAME stays).

## Progress log

- **2026-11-22:** Planned at the review of the errata survey (`PHASE-113 - Fortran
  errata.md`): the author split the phase, Fortran 95 here and the errata in Phase 113, and
  left the split's lines to the survey's judgment. This doc takes A.14's first item: Phase
  113's A.4, with A.1.11, A.3.2's module text, and A.3.5. Subtask 1, the survey, is next.
