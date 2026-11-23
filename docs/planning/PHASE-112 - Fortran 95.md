# Phase 112 — Fortran 95

**Status:** planned 2026-11-22; subtask 1, the survey (Part A), done 2026-11-22 and
accepted by the author the same day ("The author's answers"); subtask 2, recovery and syntax,
done 2026-11-23; subtask 3, zero-sized arrays, done 2026-11-23; subtask 4, `NULL()`, done
2026-11-23; subtask 5, default initialization, done 2026-11-23; subtask 6, PURE, done 2026-11-23;
subtask 7, ELEMENTAL, done 2026-11-23; subtask 8 is next. Phase 99's question 2 left Fortran 95's additions to an
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

*Answered: Part A's A.5 recommended, and the author accepted ("The author's answers").*

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

*Superseded by Part A's A.4, accepted by the author 2026-11-22.*

1. **The survey** (done 2026-11-22; Part A, revised list A.4): the HP manual's Fortran 95 additions read against govax and gfortran, a
   table of each and its subtask, a probe set of programs for gfortran; the open questions.
2. Unrecognized statements recovered from; PURE, ELEMENTAL, and FORALL lexed and parsed.
3. Zero-sized explicit-shape arrays under `/F90` (done 2026-11-23).
4. `NULL()` and pointer initialization.
5. Default initialization of components, the module text among it (done 2026-11-23).
6. PURE: the constraints (done 2026-11-23).
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
2. **Recovery and syntax** (done 2026-11-23): an unrecognized statement kept without breaking its unit
   (A.3's 1); PURE and ELEMENTAL prefixes with RECURSIVE and a type in any order (lexer,
   `ast.FunctionStmt`/`SubroutineStmt`, interface bodies); FORALL's statement and construct
   parsed (`ast.ForallStmt`, END FORALL, names); END INTERFACE's generic name; WHERE's
   construct names parsed. Each new form NOTIMPL by name in sema until its subtask.
3. **Zero-sized explicit-shape arrays** under `/F90` (A.3's 13) (done 2026-11-23).
4. **NULL()** (A.3's 3): every context of A.1's 6, pointer initialization (A.1's 8),
   DATA (done 2026-11-23; a component's `=> NULL()` default is subtask 5's).
5. **Default initialization** (A.3's 4), the module text with it (done 2026-11-23).
6. **PURE**: the constraints (A.3's 8), PURE interfaces, specification functions (A.3's 5)
   (done 2026-11-23).
7. **ELEMENTAL** procedures and references (A.3's 7), elemental defined assignment and
   operators in array assignments and WHERE (done 2026-11-23).
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

## The author's answers (2026-11-22)

The author reviewed Part A, accepted it and A.4's subtasks as they stand, and concurred
with every recommendation of A.5:

1. **Deleted features**: a new message, `%FORT-I-DELETED, deleted in Fortran 95: %s`, for
   A.1's 17 (the six deleted features), under the OBSOLESCENT keyword; OBSOLESCENT's text
   becomes "obsolescent in Fortran 95", its list Fortran 95's (A.1's 18: the computed GO TO,
   statement functions, DATA among executable statements, an assumed-length CHARACTER
   function, `CHARACTER*n` added); fixed source form flagged once per file, at its first
   statement. Subtask 13.
2. **`/NOF90`**: FORALL, PURE, ELEMENTAL, NULL(), named and nested WHERE, END INTERFACE's
   name, default initialization, and the intrinsics' new keywords compiled under `/NOF90`
   too; zero-sized arrays, zero widths, and namelist comments under `/F90` (the F90 flag)
   only.
3. **CPU_TIME**: the process's CPU time (JPI$_CPUTIM), in seconds, in 10 ms steps, in the
   argument's kind.
4. **Default initialization and SAVE**: no implied SAVE; an unsaved local of such a type
   set at every entry (A.3's 4).
5. **`F0.d`**: gfortran's form, no leading zero (`-.500`, `.500`).
6. **MAXLOC's and MINLOC's KIND=**: not planned; KEYWORDNAME stays.

## Progress log

- **2026-11-22:** Planned at the review of the errata survey (`PHASE-113 - Fortran
  errata.md`): the author split the phase, Fortran 95 here and the errata in Phase 113, and
  left the split's lines to the survey's judgment. This doc takes A.14's first item: Phase
  113's A.4, with A.1.11, A.3.2's module text, and A.3.5. Subtask 1, the survey, is next.
- **2026-11-22:** Subtask 1, the survey: Part A. The HP manual's Fortran 95 additions read
  whole (A.1, 19 rows); nine probe programs, `testdata/f95` (README), built by gfortran
  `-std=f95` and kept with its outputs, then compiled by govax (A.2): only `zerowid.f90`
  compiles, and it stops at OUTCONERR. Found beyond the seed list: named WHERE, END
  INTERFACE's generic name, comments in namelist input, specification functions, and
  elemental defined assignment. Default initialization sets an unsaved local at every entry
  (the manual and gfortran agree). A.3 has the findings, A.4 the subtasks revised (the
  same fifteen, with A.3's items placed), A.5 six questions with recommendations.
- **2026-11-22:** The author accepted Part A, A.4's subtasks, and every recommendation of
  A.5 ("The author's answers"). Subtask 2, recovery and syntax, is next.
- **2026-11-23:** Subtask 2, recovery and syntax. **The lexer** (`lex/f95stmt.go`):
  PURE and ELEMENTAL are prefixes beside RECURSIVE (`Statement.Pure`, `Elemental`), any
  of the three in any order, before the type or between it and FUNCTION
  (`headerPrefixes`, `prefixWords`, and `typedFunction`'s loop); a prefix given twice is
  DUPPREFIX with the header kept (RECURSIVE twice was UNRECSTMT; `TestRecursiveRefused`
  lost that case). The recovery, `lostHeader`: a unit's first statement that would be
  unrecognized but has words before a prefix, FUNCTION, or SUBROUTINE (free form: words
  ending at a blank) is UNRECPREFIX and read as the header from the known word on, so
  IMPURE ELEMENTAL SUBROUTINE S keeps its unit and its ELEMENTAL. FORALL is told apart as
  WHERE is (`forallStatement`: `Forall` with its statement as `Then`, `ForallBlock`
  alone; `EndForall`, END FORALL's blank in `keywordSplits`); a construct name may stand
  on a WHERE or FORALL construct (NOTCONSTRUCT's text widened). Prefix words are looked
  for only at a unit's start, where squeezed fixed form could also read `INTEGER
  PUREFUNCTIONX` as a header, as it already read `RECURSIVEFUNCTIONX`. **The parser**
  (`parse/f95.go`): `ast.ForallStmt`, `ForallBlockStmt`, `EndForallStmt`, and
  `ForallHeader` (indexes, each a name = lower : upper [: stride], then a mask, last; an
  item is an index if it is a name followed by =); a FORALL statement executes an
  assignment or a pointer assignment, else BADFORALLSTMT; ELSEWHERE and END WHERE take
  their construct's name (`constructAfter`); END INTERFACE takes a generic specification
  (`genericSpec`, shared with INTERFACE), checked against the block's by the unit stack
  (`checkEndInterface`, ENDINTERFACE); `=> expr` after a declared entity is
  `Declarator.PointerInit`. **Semantic analysis**: named WHERE constructs are done, not
  NOTIMPL, since `push` already names a construct: `elseWhere` and `endWhere` call
  `checkName` (WRONGNAME, MISSINGNAME; subtask 10 is left nesting and masked
  ELSEWHERE). PURE and ELEMENTAL headers are NOTIMPL (`pendingPrefixes`), as is `=>`
  initialization (`f90Entity`; a component's `=> NULL()` is the component default's
  NOTIMPL already); FORALL's statement and construct are NOTIMPL once, through
  `notimpl.go`'s pending constructs, the construct's statements passed over quietly
  (`pendingBodies`, `quietBody`) so its indexes cause nothing under IMPLICIT NONE, a nested
  FORALL counted but not reported. Tests: `lex`'s `TestF95Prefixes`,
  `TestF95PrefixErrors`, `TestForall`; `parse`'s `TestF95Forms` and `TestF95Probes`
  (every probe of `testdata/f95` parses without a message); `sema`'s `TestF95Pending` and
  `TestWhere`'s named cases. The probes compiled by govax now give only NOTIMPLs and their
  own subtasks' messages (`pure` and `forall` 7 NOTIMPLs each where they had 40 and 17
  messages; `nullinit`'s NOTYPEDEFs follow from its module's NOTIMPL). Left for later: a
  module's text doesn't carry PURE or ELEMENTAL yet (subtask 6, when such a module first
  compiles); ELEMENTAL with RECURSIVE, which Fortran 95 forbids, is subtask 7's check.
- **2026-11-23:** Subtask 3, zero-sized explicit-shape arrays. Under `/F90` a dimension
  whose constant upper bound is below its lower is kept as **(1:0)** (`sema/arrays.go`'s
  `emptyDimension`, called for variables by `dimensionBounds` and for components by
  `structure.go`'s bounds; without `/F90` still BOUNDORDER): the bounds LBOUND and UBOUND
  give a whole array's empty dimension (the standard's 13.13.52 and 13.13.111: 1 and 0), so
  `Extent`, `Elements`, and `Size` are 0 with no other change, and every later pass sees an
  ordinary array of no elements. Chosen over keeping the declared bounds with a clamped
  extent, which would have needed LBOUND's and UBOUND's folding, descriptors, and the
  debugger table told apart; the cost is that the storage map, the `/DEBUG` table, the
  debugger's out-of-bounds message, and a module's text show (1:0), not `(5:1)`.
  **`Elements() == 0` meant "not known"** in three places, now told apart by
  `Symbol.Empty` (a known shape with an empty dimension): `tooLarge` (a divide by zero),
  and DATASIZE in `data.go` and `datatypes.go` (so an empty array may take an initial value,
  a PARAMETER's or `= 2.0`'s, as gfortran agrees). **The module text** wrote no empty
  array constant ("an empty constructor would need its type"): `arrayConstText` now writes a
  zero of the type, which the declared `(1:0)` shape takes for its no elements, as
  `parameter.go` already reads a scalar value. **The debugger** showed an element of an
  empty array (`wholeArray` stepped once before testing the bounds): no element now, for a
  declared empty array and for an allocatable or automatic one of no extent, whose bounds
  come from its descriptor. What already worked, found by the probes: automatic arrays of
  a run-time extent of zero or less (size 0, LBOUND 1, UBOUND 0, as gfortran), empty
  sections, the reductions of nothing, an empty array in an output list, an implied DO,
  a list-directed READ, NAMELIST output, and unformatted I/O, PACK and constructors that
  make nothing, MATMUL of an empty inner dimension, assumed-shape and explicit-shape
  dummies given one, COMMON and records (no storage), EQUIVALENCE of the whole name (and
  EQUIVRANGE for an element), and `/CHECK=BOUNDS` (SUBRNG for any subscript). Tests:
  `sema`'s `TestEmptyArrays`, `fortran`'s `TestModuleEmptyArrays` (every type's empty
  PARAMETER through a module file), `debugger`'s `TestFortranEmptyArrays`, and
  `testdata/fortran90/zerosize.f90` (in every mode, with paging too; `compare.py` the same
  on gfortran but NAMELIST's layout). MANUAL.md's 14.13 "Arrays as values" has a bullet
  for them, and its table of what isn't there loses the row; Phase 113's A.1's 5 is marked
  resolved. `testdata/f90r`'s procedure regenerated by its `gen.go` (the round's rehearsal
  runs every corpus program), and ZEROSIZE added to `f90rAwaitingVMS`, for Phase 113's VMS
  round to run. Subtask 4, `NULL()`, is next.
- **2026-11-23:** Subtask 4, `NULL()`. **Semantic analysis** (`sema/null.go`): NULL has no
  row of the intrinsic table, its type being its context's; each place a pointer's value goes
  asks for it by name (`nullReference`: an `ast.Apply` of NULL where the unit hasn't made the
  name a variable, a dummy, or an external, under `/NOF90` too, the author's answer 2) and
  analyzes it against the pointer it is for (`nullValue`: MOLD, positional or by keyword,
  a pointer whose type, kind, and rank must be the pointer's, POINTERTYPE; INTRINARGS,
  NULLINTRINARG, KEYWORDNAME). `P => NULL()` is a `Nullify` (`pointerAssignment`); a
  structure constructor's pointer component and a pointer dummy's actual are a `sema.Null`
  of the pointer's type and rank (`structvalue.go`, `pointerArg`). A declaration's `=>
  NULL()` (`decl.go`'s `f90Entity`, `data.go`'s `pointerInitial`) and DATA's `NULL()`
  (`dataConstant` keeps the reference, `dataPointer` takes it for a whole pointer, an array
  pointer one item, repeat counts and all) give no bytes: a static pointer's storage starts at
  0 (Phase 107's finding 2). They set `Symbol.nullInit`, which `recursiveStorage` counts as
  an initial value, so such a pointer is static in a RECURSIVE unit, and `checkAutomaticInitials`
  makes AUTOINIT of an AUTOMATIC one; the declaration's also implies SAVE (`impliedSaves`). A
  pointer in a list is let through `dataName`'s checks by `dataNameFor` (a pointer to a type with
  pointer components too). Anywhere else NULL is NULLPLACE (`expr.go`, and `initialBytes` for
  DATA of what isn't a pointer), where the name is the intrinsic's (`/F90`, or an INTRINSIC
  statement; without either it stays a program's function, as VAX FORTRAN had it). New
  messages: NULLPLACE, POINTERINIT (`=>` followed by anything but `NULL()`), NULLMOLD;
  POINTERATTR's reason for an initial value is now "be given an initial value but NULL()".
  **Lowering** (`ir/lowerpointer.go`): a `Null` is address 0 for a scalar component
  (`targetAddress`), POINTER 0 for an array pointer component (`associate`), and for a pointer
  dummy a pointer of the statement's own, cleared (`nullPointer`: a longword, a CHARACTER
  pointer's class S descriptor, or a class NCA descriptor cell), which `pointerActual` passes
  as a named one. **A fix, generics with pointer dummies**: `resolveGeneric` gave a pointer
  dummy's actual through `sequenced`, so the specific got the target's address, not the
  pointer's (an access violation at the dummy's first use; Phase 107 never tried a generic
  of pointer dummies). A chosen specific's pointer dummies now take `pointerArg`. NULL() among
  a generic's arguments (`genericNull`) fits only a pointer dummy, of MOLD's kind if it has a
  MOLD; without one it fits any, and if it lets two specifics fit that is NULLMOLD (the
  standard's 13.14.79: MOLD is then required). Not done here: a component's `=> NULL()`
  default, which is default initialization's (subtask 5; still that NOTIMPL), and
  `ASSOCIATED(P, NULL())` (NULLPLACE; its answer would always be false). Tests: `sema`'s `TestNull` (every
  context, MOLD, the errors, generics, `/NOF90`, the storage of a recursive unit's pointers)
  and `TestPointers`' new texts; `testdata/fortran90/nulls.f90` (every context in every mode,
  with paging too, a recursive routine's SAVE'd `=> NULL()` local, a generic chosen by MOLD;
  `compare.py` the same on gfortran); `testdata/f90r` regenerated, NULLS added to
  `f90rAwaitingVMS`. MANUAL.md's "Pointers" has a `NULL()` paragraph, and its closing list
  loses `NULL()` (and FORALL, PURE, and ELEMENTAL are NOTIMPL there now, as subtask 2 made
  them); Phase 113's A.1's 3 is marked resolved in part. Subtask 5, default initialization,
  is next.
- **2026-11-23:** Subtask 5, default initialization. **The components** (`sema/derived.go`'s
  `componentDefault`): `= constant` after a component is read by the structure constructor's
  own code (`componentConst`: converted to the component's type, CONSTRVALUE; an array
  component one value for every element or an array of its shape, NOTCONFORM), kept as
  `Field.Default` (a `Const` or `ArrayConst`) and its bytes as the structure's `Initials`, a
  record component's by `partInitials`; `=> NULL()` for a pointer component is
  `Field.NullDefault` (no bytes). Errors: NOTPOINTER (`=>` for no pointer), POINTERATTR (`=` for
  a pointer), POINTERINIT, and the new COMPSLASH (VAX FORTRAN's `/value/`, a STRUCTURE field's
  form). `Structure.DefaultInit` says whether a derived type has default initialization at any
  depth (a STRUCTURE's initial values aren't: they stay static, as VAX FORTRAN has them), and
  `DefaultBytes` is a record as its defaults leave it, from `image`, which now leaves out a
  component's type's defaults when the component gives its own. **Where they go**
  (`sema/defaults.go`): a main program's and a module's objects, and a subprogram's SAVE'd ones
  or ones given a value of their own, get the bytes as initial values, as before for a
  STRUCTURE (`recordInitials`; a used module variable's are the module's, and an allocatable
  array, a pointer, or an automatic array gets none there); a subprogram's other locals, its
  INTENT(OUT) dummies, an automatic array, and its value (the function's own name's place) are
  `Unit.Defaults`, each an `Assign` of the record constant `DefaultBytes` to the whole object,
  an OPTIONAL dummy's with its `Present` test, marked `Symbol.entryDefault` so they get no
  initial values; ALLOCATE's objects get `Allocation.Default` (the array, the pointer's target,
  or the pointer component's). In a recursive unit such a local stays in the frame
  (`recursiveStorage`'s static test leaves default-initialized types out): no implied SAVE, the
  author's answer 4. A name only used through a component is still unclassified when
  `defaults` runs, so it counts. New DEFAULTINIT: such an object in a common block (Fortran
  95's 5.5.2; gfortran too), named in DATA (ours: gfortran `-std=f95` takes it but leaves the
  value out), or an assumed-size INTENT(OUT) dummy (gfortran too). EQUIVALENCE of a SEQUENCE
  one is allowed, as gfortran allows it. **A fix found on the way**: SAVE of a derived type's
  object was BADUSE, the VAX rule for records (section 4.13) applied to derived types too;
  Fortran 90 allows it (5.2.4), and it now decides where the defaults go. **Lowering**
  (`ir/lowerdefaults.go`): `entryDefaults` follows `clearAllocatables` at the main entry and
  in each ENTRY's block (`entryClears` now true when there are defaults), setting a dummy or an
  automatic array only at an entry point that defines it (`defines`); each assignment is an
  aggregate move, a MOVC3 from a `$PDATA` datum of the bytes, or the scalarizer's loop over an
  array's elements (assumed-shape ones through their strides); a frame record with defaults
  isn't also cleared (`clearPointers`'s `defaulted`). `allocationDefault` follows each
  ALLOCATE call, with STAT= only when it returned 0. **The module text** (`modwrite.go`'s
  `componentDefault`): each component's default after its name, by `constText` and
  `arrayConstText`, `=> NULL()` for a pointer, read back by the compiler's own passes.
  Tests: `sema`'s `TestDefaults` (the forms, the errors, the bytes, which objects are set at
  entry and which are static, a recursive unit's frame, `/NOF90`), `fortran`'s
  `TestModuleDefaults` (the text, and a unit compiled apart the same as one compiled with the
  module), and `testdata/fortran90/defaults.f90` (every place an object is made, in every
  mode, with paging too; `compare.py` the same on gfortran). The survey's probe
  `testdata/f95/nullinit.f90` now compiles and prints gfortran's values in every mode (its
  formats begin without a blank, so the terminal takes each line's first character for
  carriage control, as FORTRAN does). `testdata/f90r` regenerated, DEFAULTS
  added to `f90rAwaitingVMS`. MANUAL.md's "Derived types" has a "Default initialization"
  paragraph and its closing list loses the item; HELP FORTRAN DERIVED_TYPES the same; Phase
  113's A.1's 3 and A.3.2 are marked resolved. Not done here: a STRUCTURE with initial values
  in a module's text (`modwrite.go` still refuses it; Phase 113's A.3.2). Subtask 6, PURE, is next.
- **2026-11-23:** Subtask 6, PURE. **The header**: `Unit.Pure` and `Unit.Elemental` (an
  ELEMENTAL unit is PURE too), set by `header` for subprograms and interface bodies alike;
  PURE is no longer NOTIMPL (`pendingPrefixes` keeps ELEMENTAL's). **The constraints**
  (`sema/pure.go`, its comment the list), each where the analysis already sees the thing it
  judges: `checkPure`, from `finishUnit`, checks the dummies (PUREINTENT: a function's
  INTENT(IN), a subroutine's any INTENT; pointers and dummy procedures exempt; interface bodies
  get this check alone), the locals (NOTPURE for one saved, STATIC, or given an initial value by
  `=`, `=> NULL()`, DATA, or a record's value, and a SAVE with no list, `saveAllPos`), and
  `unit.calls`, every reference the unit makes (each must reach a PURE procedure through an
  explicit interface, `interfaceOf`, so an external one without an interface body, a dummy
  procedure without one, and a generic's or defined operator's or assignment's specific that
  isn't PURE are NOTPURE; and an actual to an INTENT(OUT), INTENT(INOUT), or pointer dummy may not
  be a global). A **global** (`globalWhy`) is a module's variable (by USE, or a module
  procedure's own module's by host association), a host's, or one in a common block; an
  INTENT(IN) dummy stays INTENTIN's. Definitions go through `checkIntentIn`, which now also
  calls `pureDefines`, so every place INTENTIN is checked (assignment, DO and implied DO, input
  items, specifiers, a pointer component's ALLOCATE) is checked for globals too; pointer
  assignment (the pointer and its target), NULLIFY, and ALLOCATE/DEALLOCATE of a named object
  call it themselves (`pointerOwner`). Statements: `executable` is now a wrapper of
  `executableStmt` that hands each result to `pureStatement` (STOP, PAUSE, an `IO` that isn't
  an internal file, ENCODE, or DECODE, `FileStmt`, `DefineFile`, and `IOLength`). Routines with
  no explicit interface: the system subroutines and functions, and every intrinsic subroutine
  but MVBITS (`pureSystem`); intrinsic functions are pure and never reach `calls`. New messages:
  NOTPURE ("%s can't %s in a PURE procedure", the subject saying why: "G, a variable of the
  module M,"), PUREINTENT, PUREACTUAL. **PURE interfaces**: `interfaceDifference` adds "it is PURE
  here, not there" (a PURE procedure may still have an interface that doesn't say so), and
  `checkInterface`'s `pureProcedureArgument` reports PUREACTUAL for a procedure not declared PURE
  given to a dummy procedure whose interface is PURE (`Procedure.Sym`, new, names it). **The
  module text** writes PURE (or ELEMENTAL) before a procedure's FUNCTION or SUBROUTINE, read back
  by the compiler's own passes. **Specification functions** (A.3's 5): `boundFunction` takes a
  function `specificationFunction` accepts (PURE by an explicit interface already given, a
  module procedure's or an interface body's, scalar, not internal or a dummy), its arguments a
  bound's expressions, for an automatic array's bounds and a CHARACTER length alike
  (`charlength.go` shares `boundOperands`); lowering needed nothing, the reference computed at
  entry with the other bounds. The standard's "not recursive" is left out, by our choice
  (unconfirmed): `/RECURSIVE` makes every unit recursive and a module's text says so, so the rule
  would let a compiling option change what is legal; the corpus program failed exactly so under
  `recursiveModes` before the rule was dropped. Not done: storage association (an EQUIVALENCE
  partner of a global), a pointer component's target reached through a global or an INTENT(IN)
  dummy (`X%P = 1`, which INTENTIN doesn't judge either), a structure constructor's pointer
  component given a global, and the rule that an internal procedure of a PURE one be PURE (its
  references are checked, which is what matters). Tests: `sema`'s `TestPure` (each rule, the
  messages, PURE interfaces, PUREACTUAL, specification functions and what isn't one),
  `TestF95Pending`'s PURE case now clean, `fortran`'s `TestModulePure` (the text, a unit compiled
  apart using PURE and a specification function with the same code, an impure reference refused
  both ways), and `testdata/fortran90/pures.f90` (module, internal, and external PURE procedures,
  a PURE recursive function, a PURE dummy procedure, an internal file, specification functions;
  every mode, paging too; gfortran `-std=f95` prints the same). The survey's `misc95.f90` now
  compiles without a message, and `pure.f90` gives only ELEMENTAL's NOTIMPLs. `testdata/f90r`
  regenerated, PURES added to `f90rAwaitingVMS`. MANUAL.md's "Procedures and interfaces" has a
  "PURE procedures" paragraph and its closing list loses PURE; HELP FORTRAN FORTRAN_90
  PROCEDURES the same; Phase 113's A.4's 2 is marked resolved in part. Subtask 7, ELEMENTAL, is
  next.
- **2026-11-23:** Subtask 7, ELEMENTAL (`sema/elemental.go`, its comment the whole design).
  **Definitions**: `checkElemental`, from `finishUnit` beside `checkPure` (an ELEMENTAL unit is
  PURE and gets PURE's checks too), holds each dummy to a scalar, not a pointer or a dummy
  procedure (ELEMDUMMY), no alternate return (ELEMRETURN), a function's value a scalar, not a
  pointer (ELEMRESULT); ELEMRECURSIVE for the two prefixes together, from the header's own
  prefix only (`elementalPrefixes`), since `/RECURSIVE` makes every unit recursive, and a
  module's text writes no RECURSIVE before ELEMENTAL (`modwrite.go`), else a unit compiled
  `/RECURSIVE` couldn't read its own module back. `pendingPrefixes` is gone. **References**:
  through an explicit interface that says ELEMENTAL (`elementalTarget`), `argsTo`,
  `operandsTo`, and `resolveGeneric` give a scalar dummy its actual as it is
  (`elementalArg`: no sequence association, and a whole array given its shape, NOSHAPE for an
  assumed-size one); `checkInterface` lets such a dummy take an array (no ARGRANK).
  `funcRef` (now returning an `Expr`) makes a function's reference with arrays a `FuncRef` with
  their shape and `Elemental` set (`elementalShape`: the arrays conform, NOTCONFORM; an
  INTENT(OUT) or INTENT(INOUT) dummy's actual an array when any is, ELEMOUT), and
  `Scalarize` reaches into its arguments as into an elemental intrinsic's, so it is no leaf of
  the scalarizer: the reference is made at each element (and `Leaves` gives overlap the
  arrays its arguments read, so `R(2:5) = CLIP(R(1:4), ...)` runs down).
  `FuncRef.HiddenValue` leaves it out. A CALL's `callStmt` gives it `Call.Shape`
  (`elementalCallShape`), which lowering already made a call per element in array element
  order (MVBITS's `elementalCall`), as Fortran 95's 12.4.3 asks. A reference through an
  implicit interface to a procedure of the file that is ELEMENTAL is EXPLICITELEMENTAL.
  **Generics and operators**: `fits` takes `elementwise`; `resolveGeneric` and
  `resolveOperator` try every specific exactly first, then an elemental one element by element
  (14.1.2.4.1), so a generic with a vector specific and an elemental one picks the vector's for
  a rank-1 array and the elemental one for a scalar or a matrix. **Defined assignment**:
  `assignOperands` lets an ASSIGNMENT(=) interface take an array of records too;
  `definedAssignment` returns whether it took the statement (so an elemental one's error isn't
  followed by the intrinsic assignment's); `elementalAssignment` gives the CALL its shape and
  `Call.Order`, new, the array assignment's order between target and value (no fill from a
  constructor), since the value is in parentheses and so whole before the target changes.
  Lowering's `maskedCall` (`elementalCall` is it unmasked) computes the value into a temporary
  first when the order says so, else runs the nest in the order's directions with
  `ArrayOrder.Hoisted`'s scalars; a CHARACTER value of a run-time length through a temporary
  is NOTIMPL. **WHERE**: `whereAssign` returns a `Stmt`, a defined assignment's `Call` when an
  elemental subroutine takes it (WHEREDEFINED for one that isn't elemental), and
  `finishWhere` computes the mask into its temporary first for a construct with such a call
  (a subroutine may change what the mask reads); `ir/lowerwhere.go` lowers a `Call` arm
  statement by `maskedCall` with the mask; fusion already left such constructs alone.
  `assignParts`, unused after this, is gone. Not done: the standard's rule that an elemental
  procedure's dummy appear in its specification expressions only as an inquiry's argument
  (12.7.1; a bound of a local from a dummy is accepted). Tests: `sema`'s
  `TestElementalProcedures` (`elementalproc_test.go`: references, each definition rule,
  NOTCONFORM and ELEMOUT, EXPLICITELEMENTAL, generics, WHEREDEFINED), `TestF95Pending`'s
  ELEMENTAL case now clean, `fortran`'s `TestModuleElemental` (the text, never RECURSIVE under
  `/RECURSIVE`, and a unit compiled apart with the same code), and
  `testdata/fortran90/elementals.f90` (module, internal, and external elemental functions and
  subroutines with arrays, sections, expressions, a reduction, an overlapping target, a
  CHARACTER value, an I/O list; a generic's two specifics; an elemental defined operator; an
  elemental defined assignment over its own target and in a WHERE with ELSEWHERE; every mode,
  paging too; gfortran `-std=f95` prints the same). The survey's `pure.f90` now gives only
  FORALL's NOTIMPL, and `elemasg.f90` compiles and prints gfortran's line. `testdata/f90r`
  regenerated, ELEMENTALS added to `f90rAwaitingVMS`. MANUAL.md's "Procedures and interfaces"
  has an "ELEMENTAL procedures" paragraph and its closing list loses ELEMENTAL; HELP FORTRAN
  the same; Phase 113's A.4's 2 is marked resolved. Subtask 8, the FORALL statement, is
  next.
