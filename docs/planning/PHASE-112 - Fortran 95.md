# Phase 112 — Fortran 95

**Status:** planned 2026-11-22. Phase 99's question 2 left Fortran 95's additions to an
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

## Part A — the seed list

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

1. **The survey**: the HP manual's Fortran 95 additions read against govax and gfortran, a
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

## Progress log

- **2026-11-22:** Planned at the review of the errata survey (`PHASE-113 - Fortran
  errata.md`): the author split the phase, Fortran 95 here and the errata in Phase 113, and
  left the split's lines to the survey's judgment. This doc takes A.14's first item: Phase
  113's A.4, with A.1.11, A.3.2's module text, and A.3.5. Subtask 1, the survey, is next.
