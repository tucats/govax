# Phase 83 — FORTRAN and VMS: definitions, services, handlers, ASTs

**Status:** planned (2026-10-29). Needs Phase 81 (files) and Phase 82
(structures). Part of the FORTRAN program laid out in `PHASE-74 -
FORTRAN plan.md` (Decisions 11, 17, and 21).

## Goal

Everything a FORTRAN program needs to use VMS as VMS programmers did:

- **INCLUDE '(module)'** from text libraries: `/LIBRARY` on the command
  (`FORTRAN PROG+MYLIB/LIBRARY` and the qualifier), FORT$LIBRARY,
  `SYS$LIBRARY:FORSYSDEF.TLB` when present, then govax's own modules
  generated from `vmsdef` (Decision 17): `$SSDEF`, `$IODEF`, `$JPIDEF`,
  `$SYIDEF`, `$DVIDEF`, `$RMSDEF`, `$LIBDEF`, `$FORDEF`, `$MTHDEF`,
  `$DSCDEF`, `$STSDEF`, `$LNMDEF`, `$SECDEF`, `$PSLDEF`, `$PRVDEF`,
  `$CHFDEF`, and `$SYSSRVNAM` (each service declared as an INTEGER*4
  function), as many others as programs need;
- **LIBRARY/TEXT** (create and insert text modules), if govax's LIBRARY
  lacks it, so users can build their own;
- **calling system services and RTL routines:** status functions
  (`ISTAT = SYS$GETJPIW(...)`), `IF (.NOT. ISTAT) CALL LIB$SIGNAL(%VAL(ISTAT))`,
  item lists from structures, descriptors for character arguments,
  quadword times, `%VAL` for flags, and null arguments;
- **condition handlers** written in FORTRAN: LIB$ESTABLISH and
  LIB$REVERT, a handler function's signal and mechanism arrays,
  SS$_RESIGNAL, SS$_CONTINUE, LIB$SIGNAL and LIB$STOP from FORTRAN,
  `$UNWIND`, LIB$MATCH_COND; and the condition values of FTNRTL's
  errors reaching them;
- **AST routines and exit handlers** in FORTRAN (`$SETIMR` with an AST,
  `$QIO` completion ASTs, `$DCLEXH`);
- **the system subroutines** of appendix D.4: DATE, IDATE, ERRSNS
  (Phase 80), EXIT, SECNDS, TIME, RAN, and MVBITS (D.5.3);
- **the LIB$ routines this needs** that govax lacks, Phase 61's among
  them (LIB$SIG_TO_RET, LIB$FIXUP_FLT, LIB$FLT_UNDER, LIB$INT_OVER), and
  any STR$ routine the example programs call.

## What earlier phases leave in place

- **Phase 79:** the calling standard, %VAL/%REF/%DESCR/%LOC, null
  arguments, procedure arguments.
- **Phase 82:** structures for item lists.
- **`vmsdef`**: every symbolic constant, and the system services'
  entries for LINK.
- **`internal/lbr`**: text libraries.
- **LIBRTL's condition routines** and Phase 70's condition search.

## Design notes

- **govax's FORSYSDEF modules** are generated in Go at compile time from
  `vmsdef.Symbols` by prefix, formatted as FORTRAN source (PARAMETER
  statements with INTEGER*4 declarations). No licensed text is copied:
  the values are those `vmsdef` already holds. A command (or test)
  writes them out as a `.TLB`, so users can see them.
- **LIB$ESTABLISH** works for FORTRAN as for MACRO: it stores the
  handler's address at 0(FP) of the caller's frame, which a FORTRAN
  routine's CALLS frame has.
- **AST reentrancy:** an AST routine in FORTRAN uses static storage; if
  it calls a routine the interrupted code was in, they share variables.
  That is VAX FORTRAN's behavior too; AUTOMATIC (Phase 82) is the
  remedy, and the manual's chapter says so.

## Open questions

- **Which definition modules** to generate first: the list above, then
  whatever the corpus programs INCLUDE.
- **STRUCTURE definitions** in VMS's FORSYSDEF (some modules declare
  structures as well as constants): generated where `vmsdef` has the
  layout, otherwise left to the program.

## Subtasks (provisional)

1. **Survey:** the system service and RTL manuals' FORTRAN examples,
   which definition modules programs use, Phase 61's status.
2. **Text library INCLUDE** and `/LIBRARY`; LIBRARY/TEXT if needed.
3. **govax's definition modules** from `vmsdef`.
4. **Calling services and RTL routines:** programs calling `$ASCTIM`,
   `$GETJPIW` with an item list, `$TRNLNM`, LIB$GET_INPUT,
   LIB$SPAWN, LIB$DATE_TIME.
5. **Condition handlers** in FORTRAN.
6. **ASTs and exit handlers** in FORTRAN.
7. **The appendix D system subroutines.**
8. **Missing LIB$ and STR$ routines** the programs found.
9. **End to end**, then **close-out** (MANUAL.md's "Calling VMS from
   FORTRAN").

## Progress log

- **2026-10-29:** Planned with Phase 74.
