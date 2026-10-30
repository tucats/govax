# Phase 86 — the FORTRAN milestone and close-out

**Status:** planned (2026-10-29). Needs every earlier phase of the
program. Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN
plan.md`.

## Goal

Show the compiler working on a real program, and finish the program's
documentation:

- **the milestone:** DUNGEON V3.2A (below), compiled, assembled, linked,
  and run on govax, on volume files and host files, in both paging modes
  and both optimizer settings, its output checked by a test. It pairs
  with Phase 72, which ran a VMS-built DUNGEON.EXE;
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

## DUNGEON V3.2A

The author placed DUNGEON V3.2A's source (R. M. Supnik's FORTRAN
translation, 1-Oct-94, cleared as freely available) in
`~/Documents/Technical Doc/VMS/dungeon-3.2A/` on 2026-10-29:

- nine source files (`dungeon.f`, `game.f`, `gdt.f`, `parser.f`,
  `subr.f`, `rooms.f`, `verbs.f`, `objects.f`, `timefnc.f`) and the
  INCLUDE file `dparam.for`, about 11,600 lines;
- the data files `dindx` (text) and `dtext` (3391 fixed 80-byte records,
  binary, with little-endian longwords, as on the VAX);
- `dungeon.doc`, `dundiffs.doc`, and `hints.doc`;
- `unixsubr.c`, Unix's ITIME, which isn't needed here.

It was written for VAX FORTRAN on OpenVMS. `dungeon.doc` gives the VMS
build (`FORT/NOCHECK/CONT=40` on each source, then LINK and RUN, with
DINDX.DAT and DTEXT.DAT in the default directory or named by logical
names). It also lists the VAX FORTRAN extensions the program needs:

- `!` comments;
- READONLY in OPEN;
- `.AND.`, `.OR.`, and `.XOR.` on integers as bitwise operations;
- no integer overflow checking;
- `$` in FORMAT;
- octal constants (`'xxx'O`) in DATA and octal edit descriptors in
  FORMAT.

A survey of the source (2026-10-29) adds the rest of the language it
uses: tab-format lines throughout, INCLUDE, `IMPLICIT INTEGER (A-Z)`,
some 490 PARAMETER statements, COMMON and EQUIVALENCE, DATA with
character constants in integer variables, statement functions, computed
GO TO, LOGICAL and CHARACTER variables, FUNCTIONs, ISHFT, FLOAT, RAN,
IDATE, and ITIME.

Its I/O:

- formatted sequential READ of `dindx` (I8, L4, A);
- unformatted direct-access READ of `dtext` (RECL=20 longwords: Phase
  72's keyed reads);
- unformatted sequential WRITE and READ for SAVE and RESTORE
  (`DSAVE.DAT`);
- OPEN with NAME=, STATUS=, READONLY, FORM=, ACCESS=, RECL=, and ERR=,
  and CLOSE;
- terminal I/O through formatted WRITE and READ.

What it needs, by the phase that should provide it (each phase's survey
checks its part):

- `/CONTINUATIONS=n` (Phase 74, subtask 3), and no `/CHECK` by default
  (Phase 85);
- logical operators on integers as bitwise operations, ISHFT, FLOAT,
  and character constants as integer values (Phases 78 and 82);
- octal constants and octal editing, and `$` editing (Phases 80 and 82);
- OPEN's NAME= and READONLY, direct-access unformatted READ, and
  unformatted sequential files (Phase 81);
- RAN and IDATE (Phase 83's system subroutines);
- **ITIME**, which VAX FORTRAN doesn't have. The kit's `vmssubr.for`,
  which supplied it on VMS, isn't in this copy. The milestone supplies a
  small ITIME of its own (a FORTRAN subroutine over VAX FORTRAN's TIME,
  or `$NUMTIM`), kept beside the test, not in FTNRTL;
- DTEXT.DAT on a volume as a file of fixed 80-byte records, made by
  COPY's record format qualifiers or by the test.

## What earlier phases leave in place

- The compiler, FTNRTL, MTHRTL, and the corpus with its tests.

## Open questions

- **Where DUNGEON's source lives.** Every file says "COPYRIGHT 1980,
  1990, INFOCOM ... ALL RIGHTS RESERVED, COMMERCIAL USAGE STRICTLY
  PROHIBITED", and `dungeon.doc` says copyright is retained by Infocom.
  The author has cleared it as freely available. Whether to commit it
  under `testdata/`, or keep it local with a test that skips without it
  (as `testdata/disks` is kept), is the author's call.
- **A scripted game:** the test needs a fixed sequence of moves whose
  output doesn't depend on the random seed (IDATE and ITIME seed RAN),
  or a way to fix the seed (a logical name the test's ITIME reads).
- **Whether the VMS round is wanted**, and on which volume.

## Subtasks (provisional)

1. **DUNGEON compiled:** every source through FORTRAN and MACRO with no
   message; whatever it finds missing is fixed in subtasks of its own.
2. **DUNGEON linked and run**, with its ITIME and its data files.
3. **The milestone test** in every mode, with a scripted game.
4. **The VMS round** (optional): exchange volume, procedure, results.
5. **The manual chapter.**
6. **FORTRAN-INTERNALS.md finished.**
7. **Close-out:** CLAUDE.md, PLAN.md, the follow-up list.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-10-29:** The author supplied DUNGEON V3.2A's source; surveyed
  above.
