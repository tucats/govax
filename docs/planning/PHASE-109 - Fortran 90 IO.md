# Phase 109 — Fortran 90: I/O

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 105.

## Goal

What Fortran 90 adds to I/O (the standard's chapters 9 and 10), in the compiler
and FTNRTL:

- **nonadvancing I/O**: `ADVANCE='NO'` on formatted sequential READ and WRITE,
  `SIZE=` and `EOR=` on READ, a prompt without a new line at the terminal;
- **OPEN's new specifiers**: ACTION= (READ, WRITE, READWRITE), POSITION=
  (ASIS, REWIND, APPEND), PAD=, DELIM=, STATUS='REPLACE'; and the same as
  INQUIRE's answers, with INQUIRE's IOLENGTH= form;
- **edit descriptors**: B (binary), EN (engineering), ES (scientific), G for
  INTEGER, LOGICAL, and CHARACTER as well as REAL, `Bw.m`, `Ow.m`, `Zw.m`, and
  a zero width where the standard allows none (it doesn't in 90: checked);
- **list-directed and namelist output's delimiters** (DELIM=) and Fortran 90's
  **NAMELIST form** (`&GROUP X=1 /`) beside DEC's `$GROUP … $END`, on input
  and output;
- **I/O items by descriptor**: an array section, an assumed-shape array, an
  allocatable array, a pointer's target, as one item with an NCA descriptor
  rather than an element at a time (Phase 102's expansion kept for what can't);
  and a section as an internal file or ENCODE's buffer, NOTIMPL in Phase 102;
- **derived types** in lists, a component at a time (Phase 105), checked again
  here with the new edits and nonadvancing records.
- **character constants between quotation marks in a run-time format held in
  a variable**: Phase 99's subtask 6 lets a FORMAT statement have them under
  `/F90`, giving FTNRTL the format with apostrophes, and Phase 101's subtask 5
  does the same for a format written as a character constant or a PARAMETER
  (`WRITE (*, '(1X, "X =", I3)') X`; sema's `constantFormat`). A variable's
  text is known only at run time, and FTNRTL's interpreter still refuses a `"`
  (SYNERRFOR, as VMS's FORRTL does), not knowing whether the program was
  compiled `/F90`: it needs FTNRTL to accept the mark (a new entry, or a flag in
  the statement's control block).

## What earlier phases leave in place

- FTNRTL's statement contexts, format interpreter, list-directed and namelist
  I/O, units, RMS's sequential, relative, and indexed files (Phases 80, 81, 88).
- `format`'s checker (`internal/fortran/format`), which learns the new edits.

## Design notes

- **Nonadvancing records**: the unit keeps its record open between statements
  (`UB_` flags in `ftn_units.mar`); the record is written when an advancing
  statement ends it, the unit is closed, or the image ends (the `$` prompt's
  rundown, Phase 80, shows the way).
- **POSITION='APPEND'** is RMS's RAB$V_EOF; ACTION= sets FAC; STATUS='REPLACE'
  deletes and creates.
- **FOR's condition values** for the new errors (EOR, a nonadvancing statement
  on an unsuitable unit) take new message numbers in govax's wording (Decision
  15).
- **Teaching:** FORTRAN-INTERNALS.md's FTNRTL section gains how a record is kept
  open across statements.
- **From Phase 100's survey** (2026-11-17): I/O of COMPLEX(16), which Phase
  100 declares and computes with, is NOTIMPL until this phase (FTNRTL's
  editing of H complex values).

## Open questions

1. VAX FORTRAN's `$` edit descriptor and Fortran 90's ADVANCE='NO' are close
   relatives; should they share FTNRTL's mechanism? Proposed: yes.
2. Namelist output's form under `/F90`: Fortran 90's `&GROUP … /`, DEC's
   `$GROUP … $END` otherwise? Proposed: yes, by `/F90`, since output changes.

## Subtasks (provisional)

1. **Survey** (the standard's 9.2.1.3, 9.3.4, 9.6, 10.2, 10.5, 10.8, 10.9; the
   Handbook's chapters 9 and 10), for review.
2. **The new edit descriptors** in `format` and FTNRTL.
3. **Nonadvancing I/O**, SIZE=, EOR=.
4. **OPEN and INQUIRE's new specifiers**, IOLENGTH=.
5. **NAMELIST's Fortran 90 form and DELIM=.**
6. **Items by NCA descriptor** (`FTN$IO_ITEM` learns class 10).
7. **Programs end to end**, every mode, gfortran's witness (records compared
   byte for byte where the standard fixes them).
8. **Close-out**: FORTRAN-INTERNALS.md, MANUAL.md.

## Progress log

- **2026-11-17:** Planned with Phase 99.
