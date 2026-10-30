# Phase 76 — FORTRAN's whole front end: every statement, every declaration

**Status:** planned (2026-10-29). Needs Phase 75 (every pass for a
subset). Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN
plan.md` (Decisions 1, 6, 7, 10, 16, and 19).

## Goal

Parse the whole of VAX FORTRAN V5.0, and give every declaration its
meaning and its storage, so that later phases only add code generation
and run-time support:

- **every statement** in the 1988 manual parses to the AST, including
  those whose code later phases generate (I/O statements' control lists
  and I/O lists, FORMAT, STRUCTURE blocks, NAMELIST, ENCODE/DECODE,
  OPTIONS, CDEC$ directives). A statement that parses but has no code
  yet is `%FORT-E-NOTIMPL` at the point semantic analysis reaches it;
- **declarations:** type statements for every type and length (`*n`,
  `CHARACTER*(*)`, `CHARACTER*(expr)`), with initial values in slashes
  (DEC's extension); IMPLICIT and IMPLICIT NONE; PARAMETER (both the
  standard form and appendix A.4's); DIMENSION with constant,
  adjustable, and assumed-size bounds and lower bounds; COMMON (named
  and blank); EQUIVALENCE; DATA with implied DO lists and repeat counts;
  SAVE; EXTERNAL; INTRINSIC; BLOCK DATA; the statement order rules
  (Figure 1-1);
- **constant expressions** folded with `vaxfloat` in every type;
- **the storage:** every program unit's `$LOCAL` layout, every COMMON
  block's psect and layout (with EQUIVALENCE extending it), and DATA's
  initial values in the `.MAR` file, in `$LOCAL` and in COMMON psects
  (BLOCK DATA among them), with the storage map comment.

## What earlier phases leave in place

- **Phase 75:** the lexer, parser, semantic analysis, IR, and code
  generator for the subset, and the end-to-end harness.
- **LINK's overlaid psects:** every contribution to an OVR psect starts
  at its base, so COMMON blocks declared in several modules share
  storage, and the psect is as long as the longest. Initialized
  contributions from BLOCK DATA in another module overlay the others.

## Design notes

- **Storage association** is the classic algorithm: EQUIVALENCE sets
  are merged (union-find) into groups, each member with an offset from
  the group's base; a group containing a COMMON member is placed in the
  block, which may lengthen it at the end and may not start before it
  (an error). Conflicts (one variable at two offsets) are errors. Its
  doc comment works an example through.
- **Alignment:** VAX FORTRAN aligns nothing within COMMON (the VAX
  allows unaligned data); variables in `$LOCAL` are laid out at their
  natural alignment, which is free on the VAX and faster.
- **DATA** writes initial values by offset into a psect's image: the
  generator emits `.BLKB` gaps and `.LONG`, `.WORD`, `.BYTE`, `.ASCII`,
  and floating values in hex (Decision 16), in offset order. Two values
  for one byte is an error.
- **Unresolved:** whether two modules' DATA initializing the same COMMON
  bytes is diagnosed. VMS LINK takes the last; ours does what our LINK
  does, noted in the manual.

## Open questions

- **Initial values in type statements** (`INTEGER I/5/`) are DEC's; they
  are DATA under another name. Confirm the manual's rules for arrays.
- **Hollerith constants in DATA** (appendix C.7) belong here, or with
  the other Hollerith uses in Phase 82.

## Subtasks (provisional)

1. **Survey:** chapters 1, 2, and 4 and appendices A and D's statement
   table; a table of every statement form, with its phase.
2. **The parser's remaining statements**, with golden AST dumps for
   each form in the manual's examples.
3. **Type declarations and IMPLICIT**, every type and length.
4. **PARAMETER and constant folding** in every type.
5. **Arrays:** bounds, lower bounds, adjustable and assumed-size forms,
   element offsets.
6. **COMMON and EQUIVALENCE:** the layout algorithm and its errors.
7. **DATA, SAVE, BLOCK DATA:** initial values in the `.MAR` file.
8. **End to end:** programs sharing COMMON across units and files,
   EQUIVALENCE overlays, DATA-initialized tables.
9. **Close-out:** FORTRAN-INTERNALS.md's chapter on semantic analysis
   and storage.

## Progress log

- **2026-10-29:** Planned with Phase 74.
