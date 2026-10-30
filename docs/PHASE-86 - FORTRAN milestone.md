# Phase 86 — the FORTRAN milestone and close-out

**Status:** planned (2026-10-29). Needs every earlier phase of the
program. Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN
plan.md`.

## Goal

Show the compiler working on real programs, and finish the program's
documentation:

- **the milestone:** a substantial FORTRAN program (Phase 74's question
  6 chooses it: DUNGEON's FORTRAN source if a usable copy exists, or a
  set of classic programs), compiled, assembled, linked, and run on
  govax, on volume files and host files, in both paging modes and both
  optimizer settings, its output checked by a test;
- **an optional VMS round:** VMS's MACRO assembles a few of our `.MAR`
  files and FTNRTL's sources, VMS's LINK links them, and the programs
  run on VMS 7.1 with our FTNRTL, printing what govax printed. That
  checks our generated code on real VAX semantics. If the simh VAX has
  a VAX FORTRAN compiler (Phase 74's question 5), the same programs
  compiled by it show where our run-time output differs;
- **the documentation:** MANUAL.md's chapter "Programming in FORTRAN"
  (the command, the language as supported, FTNRTL, calling VMS,
  debugging through the `.MAR` file, "Differences from VAX FORTRAN"),
  `docs/FORTRAN-INTERNALS.md` finished as a guided tour, HELP FORTRAN,
  CLAUDE.md, and PLAN.md's summary of the program;
- **what's left:** a list of the language features not supported and
  the follow-up work found along the way.

## What earlier phases leave in place

- The compiler, FTNRTL, MTHRTL, and the corpus with its tests.

## Open questions

- **The milestone program** (Phase 74's question 6).
- **Whether the VMS round is wanted**, and on which volume.

## Subtasks (provisional)

1. **The milestone program:** obtain or write it; whatever it finds
   missing is fixed in subtasks of its own.
2. **The milestone test** in every mode.
3. **The VMS round** (optional): exchange volume, procedure, results.
4. **The manual chapter.**
5. **FORTRAN-INTERNALS.md finished.**
6. **Close-out:** CLAUDE.md, PLAN.md, the follow-up list.

## Progress log

- **2026-10-29:** Planned with Phase 74.
