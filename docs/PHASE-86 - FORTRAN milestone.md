# Phase 86 — the FORTRAN milestone and close-out

**Status:** planned (2026-10-29). Needs every earlier phase of the
program. Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN
plan.md`.

## Goal

Show the compiler working on real programs, and finish the program's
documentation:

- **the milestone:** a substantial FORTRAN program, compiled, assembled,
  linked, and run on govax, on volume files and host files, in both
  paging modes and both optimizer settings, its output checked by a
  test. The hope is
  DUNGEON's FORTRAN source, pairing with Phase 72's DUNGEON.EXE: the
  author is looking for a freely available copy whose license allows our
  use (Phase 74's review, answer 6). Without one, a set of classic
  programs written for the corpus;
- **an optional VMS round:** VMS's MACRO assembles a few of our `.MAR`
  files and FTNRTL's sources, VMS's LINK links them, and the programs
  run on VMS 7.1 with our FTNRTL, printing what govax printed. That
  checks our generated code on real VAX semantics. There is no VAX
  FORTRAN compiler on the simh VAX (Phase 74's review, answer 5), so
  our run-time output isn't compared with VAX FORTRAN's;
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

- **The milestone program:** DUNGEON's source, if the author finds a
  freely usable copy. Its dialect (it was written for VAX FORTRAN and
  ported widely) may need features the corpus hasn't; whatever it needs
  is fixed in subtasks of its own.
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
