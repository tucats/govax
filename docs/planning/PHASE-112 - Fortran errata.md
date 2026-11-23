# Phase 112 — Fortran errata

**Status:** planned 2026-11-22 at Phase 111's close-out. Phase 99's question 2
left Fortran 95's additions to an optional Phase 112, and the author chose go
(`PHASE-111 - Fortran 90 milestone.md`, "The author's answer on Phase 112"),
then widened the phase the same day: it is the **errata** of both Fortran
programs and the tools beside them. A skeletal doc: subtask 1, the survey,
makes Part A's list whole and proposes what this phase does, reviewed before
any implementation. Needs Phases 74 to 111.

## Goal

A complete, sorted **to-do list** of every gap left behind, and as much of it
done as the author chooses:

- **VAX FORTRAN** (Phases 74 to 89): the language, the compiler, FTNRTL, and
  the run-time libraries a FORTRAN program calls (MTHRTL, LIBRTL's STR$ and
  LIB$, the services);
- **Fortran 90** (Phases 99 to 111): every NOTIMPL, every "measured, left for
  later", every unconfirmed choice;
- **Fortran 95's additions**, the author's go: FORALL, PURE and ELEMENTAL,
  `NULL()` and default initialization, CPU_TIME, nested WHERE, and the rest
  (Part A's group D);
- **the tools beside them**: MACRO, LINK, LIBRARY, ANALYZE, the debugger,
  and what the VMS rounds found in VMS's tools or ods2's volumes.

The list may run past what one phase does. Each item ends in one of these
places: **done** here, with a test; **its own phase** (planned and named);
**a probe** (a question only a VMS run answers, for the next simh round);
**measured and left**; **not planned** (by decision, with the reason); or
**already resolved**, which the survey records where the item was written.

**Resolved items are corrected where they were written** (the author's
direction, 2026-11-22): an old phase doc's "Where things stand", a
DEVIATIONS.md entry, or MANUAL.md's lists that still names something a later
phase did gets a note saying it was resolved and in which phase
("*(Resolved in Phase 88.)*"). A DEVIATIONS.md entry whose every bullet is
resolved moves to `DEVIATIONS-RESOLVED.md`. The close-out began this for
Phases 81 to 86 and DEVIATIONS.md's Phase 27 and 28 entries; the survey
finishes it for every phase doc from 27 on.

## What earlier phases leave in place

- Each phase's "Where things stand" (Phases 78 to 111; Phase 86's is the
  VAX FORTRAN program's, Phase 111's the Fortran 90 program's).
- DEVIATIONS.md, by phase; MANUAL.md's 14.7 ("What isn't there", "Choices")
  and 14.13 ("What Fortran 90 isn't here").
- The compiler's NOTIMPL, F95ONLY, and UNRECSTMT reports, findable in the
  code (`sema/notimpl.go`, `report("NOTIMPL"`), and FTNRTL's NOTIMP.
- The oracles' lists of known differences (`ftn86VMSDifferences`,
  `f90rObjectDifferences`, `tir111Differences`, `expectedDifferences`,
  `for88Differences`, ...), each item with its reason.

# Part A — the list, as the close-out found it

A seed for the survey, gathered from the sources above on 2026-11-22. Each item
names where it was written; the survey checks each against the code (some may
be done already), adds what the sources above hold that this misses, and gives
each its place.

## A. VAX FORTRAN: the language and the compiler

1. `CONVERT=` other than NATIVE (FOR's NOTIMP; Phases 81, 86).
2. A running program's files on the host (Phase 81's Answer 3; MANUAL.md
   14.7).
3. Object modules from the compiler itself (`/OBJECT` runs MACRO; Phase 74's
   Future work).
4. `/STANDARD=SEMANTIC`, what a program means rather than how it's written; a
   FUNCTION with no parentheses and `&s`; a bare `X` (Phase 85).
5. A typed name that's a system or intrinsic function (`REAL SQRT`) still
   getting a `$LOCAL` cell and a map line (Phase 86).
6. Frame alignment a longword at most, whatever `/ALIGN` asks; `/CHECK` for a
   frame variable read before it's set (Phase 89).
7. An AUTOMATIC record of a natural structure only on a longword; an aligned
   block's layout differing between separately compiled files with no message
   (Phase 82).
8. Not planned, recorded: DICTIONARY, CDEC$ PACK, CPAR$ directives, threads
   and `/REENTRANCY`, `NON_RECURSIVE` (Phases 82, 86, 89).

## B. FTNRTL and the run-time libraries

1. STR$'s ADD, DIVIDE, MUL, RECIP, ROUND, and STR$ELEMENT_R8 NOTIMPL
   (Phase 83's answer 2).
2. LIB$DECODE_FAULT, SIM_TRAP, FIND_IMAGE_SYMBOL, GET_OPCODE, and
   LIB$INITIALIZE, waiting for Phase 61, which is still planned (Phase 83's
   answer 4); Phase 60's trees likewise.
3. 191 of the service vector's entries signal NOTIMPL (Phase 83's Decision
   6); `$GETCHN` among them (`ftn86VMSDifferences`).
4. The last bits of OTS$ negative powers and of MTHRTL's complex functions
   against VMS's (Phase 86, six orders of operations tried).
5. FTNRTL's per-statement paths, about 170 instructions a record; `astrace`
   parsing formats 80,000 times (Phase 84, subtask 11).
6. The same frames after an unwind: a routine called again after a handler
   unwound a statement is RECIO_OPE on that unit (Phases 83 and 89;
   DEVIATIONS.md).
7. Zero-sized and unwound heap blocks: an unwind leaves an allocatable local's
   or a function value's block allocated until the next use (Phase 104).
8. A NAMELIST object past 8 levels or 15 dimensions (Phase 109).

## C. Fortran 90

From Phase 111's "Where things stand", which gathers Phases 99 to 110's:

1. NOTIMPL, named (MANUAL.md's "What Fortran 90 isn't here"): DIM= of
   CSHIFT, EOSHIFT, SPREAD and RESHAPE's ORDER= not constant; an
   assumed-shape array to `%LOC`, `%REF`, `%DESCR`, or a system subroutine; a
   CHARACTER argument of run-time length not made of names; a BACK= that is
   an array; TRANSFER to a CHARACTER MOLD of zero or run-time length; a
   component or function value of CHARACTER length zero; COMPLEX(16) to SUM
   and its kin; a statement function of a derived type; a name an internal
   procedure's declarations use that its host uses only in executable
   statements; a CHARACTER pointer of run-time length but a `LEN=*` dummy.
2. Modules (Phase 106): ENTRY in a module procedure; an external procedure a
   specific of two generics; a type known by two local names; a hidden entity
   whose name the module has for something else; a COMMON block a module and
   a module it uses both declare; a structure with initial values, an unnamed
   nested structure, a constant whose bits are no number, in a module's text;
   govax's definition modules as module files to USE.
3. Noticed, not changed: NOTRECORD's wording for `F(A)%N`; `Y(D(1)%N)` in a
   bound BADBOUND (Phase 107); EXPONENT and FRACTION faulting on a reserved
   operand in their MOVF; TRANSFER to a derived type never a constant
   (Phase 108).
4. Not planned, ours (Phase 104): a constructor of a substring's values.

## D. Fortran 95's additions (the author's go)

1. FORALL, the statement and the construct (UNRECSTMT today).
2. PURE and ELEMENTAL procedures, module procedures among them (UNRECSTMT).
3. `NULL()`, `=> NULL()`, and default initialization of components (NOTIMPL).
4. CPU_TIME.
5. A WHERE inside a WHERE, and ELSEWHERE with a mask (F95ONLY).
6. MAXLOC's and MINLOC's `DIM=`, CEILING's and FLOOR's `KIND=`, and the
   other intrinsic changes.
7. Zero widths, `I0`, `B0`, `O0`, `Z0`, `F0.d` (OUTCONERR today).
8. `/STANDARD=F95` beside `F77` and `F90`, the deleted features judged.

Already Fortran 95's in govax: allocatable locals deallocated at return
(Phase 99's Decision 8), every pointer disassociated at the start (Phase
107's answer 2).

## E. The optimizer, measured and left

1. Global CSE and PRE, autoincrement, a rotated test copied to its way in,
   unread `Call.Result`s, read-only data no statement uses (Phase 84).
2. Small waste: a loop's copy after the DO's own store; a copy into a register
   before the loop sets it; a `$H` in memory for lack of a register (Phase 84).
3. A run-time test of two sections' overlap choosing a loop's direction;
   `A = F(X)` into A; SPREAD and CSHIFT as elements; copies of sections to old
   dummies (one for INTENT(OUT) too); a whole-array copy as MOVC3; fusion of
   arrays of run-time extent (Phases 102, 104, 110).
4. `caseForm`'s thresholds, unmeasured (Phase 101); what internal
   procedures cost the optimizer (Phase 103).
5. (Real MACRO assembling `/OPTIMIZE`'s output: resolved in Phase 86, and
   marked so in Phase 84's doc.)

## F. MACRO

1. `PROG+LIB/LIBRARY`, the positional macro library (Phases 27, 86).
2. No object for a module with errors, where VMS's MACRO writes one
   (DEVIATIONS.md, Phase 111); a register name in a data expression,
   unsettled (Phase 111).
3. VMS MACRO's occasional byte of a long immediate run through the stack
   (`tir111Differences`; no rule found).
4. `.ENABLE TRUNCATION` and `VECTOR` ignored; messages in govax's form; only
   the first undefined symbol reported (Phase 27).
5. Object encoding choices no fixture confirms (Phase 27); a `;` inside
   `<...>` in a macro call, and `.ERROR`/`.WARN`'s text (Phase 28).
6. `MACRO A+B`'s numbering of files and `/DEBUG`'s source file, and a `.END`
   before the last file, unconfirmed (Phase 86).

## G. LINK, LIBRARY, and ANALYZE

1. LINK doesn't translate a logical name for an options file
   (`LINK X/OPTIONS`; Phase 86).
2. `testdata/link82`'s C22, C23, C26, C27, and C29 differ from VMS
   (Phase 82; DEVIATIONS.md).
3. The GST padded to a whole block where real LINK ends mid-block (Phase 29's
   Decision 7); the image's symbol tables past the DST not compared in
   `TestFTN86Objects` (Phase 86).
4. LINK's `STACK=` doing nothing with paging off, and a stack past P1's top
   1,024 pages meeting a P1 expansion (Phase 89).
5. LIBRARY's `/COMPRESS` (Phase 83); a new library allocated only the blocks
   it uses (Phase 28); WD4's block counts and W13's binary keys (Phase 90).

## H. The debugger

1. Real FORTRAN debugger records (Phase 85's review, level C); the language
   stays MACRO (Phases 85, 110).
2. `SET BREAK %LINE n`, SHOW CALLS's line column, TYPE, and EXAMINE/SOURCE
   counting the MACRO file's lines (Phases 110, 111).
3. A component after subscripts in an expression (Phase 110).
4. Phase 42's known differences from VMS's debugger (DEVIATIONS.md, items 1 to
   10; `expectedDifferences`) and its unconfirmed rules, for a simh round.
5. `testdata/dbg85`'s differences left and `dbg85c.dlg` not replayed
   (Phase 85); Phase 89's unconfirmed scope details.

## I. The VMS rounds' open items

1. VMS 7.1's XQP and ANALYZE/DISK_STRUCTURE failing on an index file
   extension header ods2 made (Phase 111; DEVIATIONS.md): which field.
   `tools/odscheck`, the seed of an ANALYZE/DISK_STRUCTURE/REPAIR phase.
2. Real LIBRTL on govax faulting at `SHARE$LIBRTL+0CE1` (Phase 86).
3. RECURAST's WAITED on the VAX (Phase 89), SECNDS within a second
   (Phase 86): timing.
4. `testdata/idx88`'s split points, check characters, DATA_FILL, and SIDRs
   (Phase 88's items 1 to 6).

## J. Unconfirmed choices

Each phase's own list (Phases 80, 81, 83, 84, 85, 88, 89, 100 to 111),
those a user sees in MANUAL.md's 14.7 "Choices". The survey sorts them into
those a VMS run could settle (FORRTL, LINK, MACRO, the debugger: a probe
round) and those no VMS can (there is no VAX FORTRAN or Fortran 90), which
stay recorded.

## K. Small things seen on the way

1. A full volume `%DCL-E-BADFILESPEC` where VMS says the device is full; the
   Ctrl/Y level's prompt without `-` continuation (Phase 86).
2. COPY's `%COPY-S-COPIED` without `/LOG`; TYPE after a record with no
   carriage control; STR$MATCH_WILD with `%` (Phase 83).
3. DIRECTORY/TOTAL; ods2's null type (`rms.ExplicitNullType`) (Phase 88).

## Open questions

1. **Scope**: does Phase 112 do Fortran 95 (group D) and a chosen part of the
   rest, leaving the others to named phases, or the list alone first?
2. **Sources for Fortran 95**: no text of its standard is in
   `~/Documents/Technical Doc/VMS/`; the HP Fortran manual
   (`VMS-FORTRAN-LANGUAGE.txt`) describes Fortran 95 with DEC's extensions,
   and gfortran is the witness. Enough?
3. **A simh round** for groups F to J's questions a VMS run can settle?
4. **Phases 60 and 61** (LIB$ trees and conditions, still planned): folded in
   where FORTRAN needs them, or left?

## Subtasks (provisional)

1. **The survey**: every phase doc from 27 on, DEVIATIONS.md, MANUAL.md's
   lists, and the code's NOTIMPLs swept; each item checked, placed, and
   resolved ones marked where they were written; the list for the author's
   review, with the questions above.
2. Onward: as the survey's review decides.

## Progress log

- **2026-11-22:** Planned at Phase 111's close-out: first as Fortran 95's
  additions, the author's go; then, by the author's direction, as the errata
  of both Fortran programs and their tools, with Part A's seed list gathered
  from the phase docs, DEVIATIONS.md, and MANUAL.md. Resolved items found
  while gathering it marked in their own docs: Phases 81, 82, 83, 84, 85, and
  86 (indexed files, Phase 88; reentrant routines, SET SCOPE, and which call a
  dummy means, Phase 89; LIBRARY in place, Phase 90; `$QUIDEF` and
  `$SJCDEF`, Phase 98; a record's fields in the debugger, Phase 110; `/CHECK`
  through the optimizer, Phase 85; real MACRO on `/OPTIMIZE`'s output, Phase 86), and DEVIATIONS.md's Phase 27 (`MACRO A+B`,
  Phase 86) and Phase 28 (LIBRARY, Phase 90) entries. Subtask 1, the survey,
  is next.
