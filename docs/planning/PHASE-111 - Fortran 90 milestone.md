# Phase 111 — Fortran 90: the milestone and close-out

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs every phase from 99 to 110.

## Goal

Fortran 90 shown working on substantial programs, and the program documented:

- **a milestone program**, by Phase 99's question 7: one the author supplies, or
  a set of our own that uses every phase together (a module-based linked list and
  tree with pointers and derived types, a matrix library with assumed-shape
  arrays and generic interfaces, a text processor with the character intrinsics
  and nonadvancing I/O, a simulation with allocatable arrays and WHERE);
- compiled and run in **every mode** (`/OPTIMIZE` both ways, `/RECURSIVE` both
  ways, both paging modes, `/CHECK=ALL`, `/DEBUG`), printing the same output;
- **gfortran's witness**: the same sources built by gfortran, every line of output
  compared (`compare.py`, extended), differences explained against the standard;
- **the old program unchanged**: DUNGEON and the FORTRAN-77 corpus give the same
  MACRO files under `/NOF90` as before Phase 99 (`TestNoF90Unchanged`), and play
  the same games;
- **the optional VMS round**: VMS 7.1's MACRO and LINK build FTNRTL and the
  Fortran 90 corpus from govax's `.MAR` files and run them (`testdata/ftn86`'s
  pattern, a new `testdata/f90r`);
- **documentation finished**: MANUAL.md's 14.13 as a finished section (with
  "What Fortran 90 isn't here" and the differences in 14.7), FORTRAN-INTERNALS.md's
  Fortran 90 sections finished and a revised map, HELP FORTRAN, PLAN.md's summary,
  CLAUDE.md.

## Subtasks (provisional)

1. **The milestone program compiled**, whatever it finds fixed in subtasks of its
   own.
2. **Run in every mode**, the milestone test.
3. **gfortran's witness.**
4. **The VMS round** (optional).
5. **The manual's section.**
6. **FORTRAN-INTERNALS.md finished.**
7. **Close-out**: PLAN.md, CLAUDE.md, the follow-up list ("Where things stand"),
   and Phase 112's go or no-go (Phase 99's question 2).

## Progress log

- **2026-11-17:** Planned with Phase 99.
