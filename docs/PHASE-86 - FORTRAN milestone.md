# Phase 86 — the FORTRAN milestone and close-out

**Status:** in progress (subtasks 1 to 3 done 2026-11-03). Needs every earlier phase of the
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

**Kept local, never committed** (the author's decision, 2026-10-29):
every file says "COPYRIGHT 1980, 1990, INFOCOM ... ALL RIGHTS RESERVED,
COMMERCIAL USAGE STRICTLY PROHIBITED". DUNGEON is used for development
from its folder (or a gitignored local copy), and the milestone test
finds it there and skips without it, as the tests on `testdata/disks`
volumes do. Only govax's own additions (the test, its ITIME, its
scripted game) are committed, and they must not quote the game's source
or text.

## What earlier phases leave in place

- The compiler, FTNRTL, MTHRTL, and the corpus with its tests.

## Open questions

- **Whether the VMS round is wanted**, and on which volume.

## Subtasks (provisional)

1. **DUNGEON compiled** (done 2026-11-03): every source through FORTRAN and MACRO with no
   message; whatever it finds missing is fixed in subtasks of its own.
2. **DUNGEON linked and run** (done 2026-11-03), with its ITIME and its data files.
3. **The milestone test** (done 2026-11-03) in every mode, with a scripted game.
4. **The VMS round** (optional): exchange volume, procedure, results.
5. **The manual chapter.**
6. **FORTRAN-INTERNALS.md finished.**
7. **Close-out:** CLAUDE.md, PLAN.md, the follow-up list.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-10-29:** The author supplied DUNGEON V3.2A's source; surveyed
  above.
- **2026-10-29:** The author decided DUNGEON stays local: used for
  development, never committed.
- **2026-11-03, subtask 1:** DUNGEON compiled, by its own documentation's commands.
  Phase 84 had already built it, with `/CONTINUATIONS=40` and a command per file, and
  compared two builds' games. `TestDungeonCompiles` holds the kit's build as it's typed
  (`FORT/NOCHECK/CONT=40 DUNGEON.F,GAME.F,...`, the abbreviations, `.F` names, two lists):
  - **The result.** Silent through FORTRAN and MACRO on a volume and on host files, nine
    objects that `obj.Check` accepts, the MACRO files the same on both sides but for the
    comments naming the source and INCLUDE files. The same under 11 settings (`/NOOPTIMIZE`, `/CHECK=ALL` both ways,
    `/DEBUG/NOOPTIMIZE`, `/LIST/CROSS_REFERENCE/SHOW=ALL`, `/NOF77`, `/G_FLOATING`,
    `/ALIGN=ALL`, `/D_LINES`, `/EXTEND_SOURCE`). Without `/CONT=40`, three statements are
    TOOMANYCONT, so the qualifier is needed. Tried once, not kept: `/NOI4` is rightly 425
    INTRANGEs (the source's 32768s) and a SYSARG, and `/DEBUG` with the optimizer says only
    DEBUGOPTIMIZE; `/STANDARD=ALL` and `/WARNINGS=ALL` gave no panic.
  - **MACRO takes a list** (found here; the author's direction, with VMS's HELP MACRO
    text). `MACRO A,B` assembles each to an object and listing of its own, and `MACRO A+B`
    concatenates them into one input, for a single object and listing named for the first
    file, which the author checked on VMS. A later name takes its defaults from the name
    before it. An error goes on to the next assembly, the command ending with the last
    failure's status, as FORTRAN's list does (`dclcmd.MacroList`). `govax macro` takes
    several sources, `a+b` joined. `/joinable` no longer needs a positional qualifier
    (`dcl.Parse`). Files joined are numbered as one input in messages and listings, and
    `/DEBUG`'s source file is the first: **unconfirmed** against VMS, as is what a `.END` in
    a file before the last does (govax's assembler stops at it, so what follows is unread).
    VMS's positional `/LIBRARY` (`MACRO LIB/LIBRARY+SRC`) isn't done; govax keeps its
    `/LIBRARY=(...)`.
  - **A false warning fixed.** `/WARNINGS=UNUSED` said `REAL RAN` was never used in a unit
    calling RAN: a system function's reference was noted under the routine, not the declared
    name (`sema`'s `noteSystem`, as `noteIntrinsic`). Of DUNGEON's 14 findings the other 13
    were read against their units and are true; the test keeps the count.
  - **Left for later:** a typed name that's a system or intrinsic function (`REAL SQRT`)
    still gets a `$LOCAL` cell and a variable's line in the map, harmless but wasted; a full
    volume is `%DCL-E-BADFILESPEC` with the reason at its end, where VMS says the device is
    full; and `-` continuation typed at the console isn't honored (`LINK A,-`, as the kit's
    LINK is written), which subtask 2 is to fix.
- **2026-11-03, subtask 2:** DUNGEON linked and run, by the kit's commands and with
  its data files. `TestDungeonRuns` builds it from the kit's files and plays:
  - **On a volume:** everything brought in by COPY, built, and played, SAVE in one run
    and RESTORE in the next, whose score and moves match. Nothing in the program or
    its link needed changing.
  - **On host files:** the sources, objects, and image on the host, and the data on a
    volume by the logical names `dungeon.doc` gives (DINDX, DTEXT). A program's RMS reaches
    only volume files (Phase 81's answer 3), so with no default directory on a volume SAVE
    reports that it can't reach the file, which the game handles. That's what "host files"
    means for this milestone.
  - **ITIME** is `testdata/dungeon86/vmssubr.for`, the kit's missing VMSSUBR.FOR: the
    system subroutine TIME's text read as three integers, by an internal READ.
  - **COPY's record qualifiers** (the author's direction; `rms/copyformat.go`). A host file
    says nothing of records, so `/RECORD_FORMAT` (FIXED, VARIABLE, VFC, STREAM, STREAM_LF,
    STREAM_CR, UNDEFINED), `/RECORD_SIZE`, `/CARRIAGE_CONTROL`, and `/CONTROL_FIELD_SIZE`, named
    as in FDL's RECORD section, say what to make; `/BINARY` says whether the host file is read
    as bytes or lines. DTEXT.DAT is `COPY "dtext"/HOST DTEXT.DAT/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=80`.
    govax's own, there being no host on VMS. Our choices, unconfirmed: lines padded with blanks,
    PRINT's control bytes (a line feed before, a carriage return after).
  - **`-` continuation at the terminal** (found in subtask 1): `Dispatcher.Dispatch` holds a
    line ending in a hyphen at command level 0 and joins the next, `dclshell.JoinContinued`
    with a procedure's rule, the prompt an underscore and the usual one. CTRL/C or CTRL/Z
    drops what's held. A subprocess's DCL does the same (`subprocessCLI.nextLine`). The
    Ctrl/Y level's prompt doesn't.
- **2026-11-03, subtask 3:** the milestone test, `TestDungeonMilestone`: DUNGEON built by its
  commands and playing `testdata/dungeon86/game.in`, on a volume and from the host, with paging
  on and off, with and without the optimizer, 8 runs printing one SHA-256; on a volume a game is
  also saved and restored. The author accepted a digest, since the game's text can't be kept.
  - **Why the digest can be trusted: gfortran.** The author installed gfortran during this
    subtask, so a digest needn't rest on reading a transcript.
    `testdata/dungeon86/gfortran/compare.py` builds the same source with gfortran (a scratch copy,
    each patch only saying in its dialect what VAX FORTRAN says in its own) and with govax, plays a
    script on both, and compares every line, normalizing only what gfortran does with column 1. RAN
    is MTH$RANDOM's manual's generator there, so combat and the thief follow the same numbers.
    `randgame.py` writes random games, whose debugging-tool stretches turn death off, move the
    player to any room, hand over any object, and show its tables, octal ones among them.
    **Result: no line differed**, in 22 games, the longest 3,666 lines typed, about 48,000 lines
    printed in all, and 5 of them again under `/NOOPTIMIZE`. Both compilers' subscript checks
    (`/CHECK=BOUNDS`, `-fcheck=bounds`) ran four of the games with nothing to report. So the compiler, FTNRTL, MTHRTL,
    and RMS agree with another compiler on a real program, which is more than the plan hoped for:
    Phase 74's review had nothing to compare our run-time output with.
  - **The seed was the date too.** `game.f` makes the seed from IDATE and ITIME, so Phase 84's
    fixed ITIME alone gave one game per day, which a digest can't take.
    `testdata/dungeon86/fixtime.for` has both, compiled as `GAME.F+FIXTIME.FOR` so the file's own
    IDATE wins over the system subroutine, and `vmssubr.for` stays the real ITIME that
    `TestDungeonRuns` plays.
  - **A bug the input redirection found.** `DEFINE/USER SYS$INPUT file` then `RUN` read nothing, and
    made a new, empty version of the input: FTN$$UNIT_OPEN did a `$CREATE` for every unit on
    SYS$INPUT:. Phase 72 had seen FORRTL `$CREATE` it, on a program whose first statement wrote it.
    Asked with `SET DEBUG RMS`, FORRTL's first READ is a `$OPEN`, whether SYS$INPUT is a file, the
    terminal, or a procedure (a `$PARSE` before it, which we don't need). FTNRTL now does the same.
    A unit on SYS$OUTPUT: stays a `$CREATE`, so a READ of one gets ERRDURREA as
    `TestFortranReadOutputFile` wants; FORRTL wasn't asked about that. `TestFortranReadInputFile`.
  - **What looked like bugs and weren't.** A game that reaches the end of its input loops: DUNGEON's
    `END=` tries again, and on gfortran a second read past the end is fatal, so the two only differ
    there, and a game in the echo room can't even QUIT. The volume damage after that (an index file
    inconsistency) was the harness's, killing govax before it dismounted. Without a file, `>` ends
    its line where a terminal read does; with one the next record starts a line, as it should.
- **2026-11-03, subtask 4, prepared:** the author asked for the VMS round: does a program we compile
  run on a real VMS, with FTNRTL built there from its sources, and a set of test programs?
  `testdata/ftn86` (README.md) is the round. `gen.go` writes `exchange.cmd`, a volume with FTNRTL's
  sources, and every corpus program, kernel, and module program compiled by govax both ways, each in a
  directory of its own, since a program may leave files; `ftn86.com` has VMS's MACRO and LINK build
  FTNRTL, then each program against that FTNRTL, 206 runs; `copyout.cmd` brings back the log, FTNRTL's
  objects, image, and map, to hold our MACRO and LINK to VMS's on it. `exchanged.cmd`, `ftn86d.com`,
  and `copyoutd.cmd` are the same for DUNGEON, whose log stays in `testdata/disks`.
  - **Checked before the VAX:** FTNRTL's outside references. `OTS$$CVT_x_T_R8` is in VMS 7.1's LIBRTL
    symbol table, and each `FOR$_` value is defined in FTNRTL's own sources. Only the LINK differs:
    `/NOSYSLIB` is for govax's build, with its own symbol tables, and VMS's needs IMAGELIB.
  - **The rehearsal, `TestFTN86Procedure`:** the same procedure on govax, FTNRTL from its sources and
    every program linked against that. All 206 runs print their `name.out`, code addresses masked, but
    for `ftn86FileDifferences`, each saying why: a `name.out` is a terminal's output.
  - **Another bug found: LIB$GET_INPUT with SYS$INPUT a file.** `services.for` read the console's next
    commands, since a name DCL defined for the image led to no process-permanent file and so to the
    terminal; the code said so, as unconfirmed. Like an image's SYS$OUTPUT file, it now opens the
    file at the first read and holds it until rundown (`corevms/outfile.go`, `inputFileRecord`).
    `TestLibGetInputFile`. The round will confirm it: `services.in` has its line.
  - **Not bugs:** `SET NOON` doesn't reach a `CALL`'s level, so a program's fatal condition ended the
    subroutine, as it would on VMS; `ONE` has its own.

