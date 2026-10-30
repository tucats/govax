# Phase 81 — FORTRAN files

**Status:** planned (2026-10-29). Needs Phase 80 (formatting and the
error model). Part of the FORTRAN program laid out in `PHASE-74 -
FORTRAN plan.md` (Decisions 12, 13, and 21).

## Goal

FORTRAN's file statements and every access method this program
supports, on volume files and host files:

- **OPEN** with every keyword in chapter 9 (Table 9-1): FILE, NAME,
  DEFAULTFILE, STATUS, DISPOSE, ACCESS, FORM, ORGANIZATION, RECORDTYPE,
  RECL, RECORDSIZE, CARRIAGECONTROL, BLANK, BLOCKSIZE, BUFFERCOUNT,
  EXTENDSIZE, INITIALSIZE, MAXREC, ASSOCIATEVARIABLE, READONLY, SHARED,
  NOSPANBLOCKS, TYPE, USEROPEN (a program's routine called to do the
  `$OPEN` or `$CREATE`), ERR, IOSTAT;
- **CLOSE** (STATUS and DISPOSE: KEEP, DELETE, PRINT, SUBMIT), and the
  units closed at image exit (an exit handler);
- **INQUIRE** by unit and by file, every specifier in 9.3;
- **REWIND, BACKSPACE, ENDFILE**;
- **unformatted sequential** I/O (VAX FORTRAN's segmented records for
  RECORDTYPE='SEGMENTED');
- **direct access:** READ and WRITE with REC= on fixed-length sequential
  files, ASSOCIATEVARIABLE, MAXREC; DEFINE FILE and FIND (appendix A);
- **relative files:** ORGANIZATION='RELATIVE', direct and sequential
  access, DELETE of a record;
- **default names:** unit n is file FORnnn.DAT unless a logical name
  FORnnn is defined, and the terminal statements' names (FOR$READ,
  FOR$ACCEPT, FOR$PRINT, FOR$TYPE): the User Manual's conventions,
  marked unconfirmed;
- **the RMS (and `ods2`) additions** these need.

Indexed files (ORGANIZATION='INDEXED', keyed READ, REWRITE, DELETE,
UNLOCK on them) are not in this phase. Phase 74's question 4 decides
whether they come later in this program or in another.

## What earlier phases leave in place

- **Phase 80:** FTNRTL's formatting and error model.
- **RMS:** sequential files on volumes and the host, FAB/RAB/NAM/XAB
  handling, FORTRAN carriage control, file sharing and record locks
  (Phase 47), `$UPDATE` and `$TRUNCATE` (Phase 49), keyed reads of
  fixed-length sequential files (Phase 72).
- **`ods2`**, which this phase may change (Decision 21: change the
  sibling checkout, tag, then `GOWORK=off go get`).

## Design notes

- **FTNRTL's unit table** gets, for each open unit, a FAB, RAB, NAM, and
  record buffer from LIB$GET_VM, and the unit's attributes; INQUIRE
  answers from it and from the NAM.
- **RMS additions expected:** keyed `$PUT` and `$UPDATE` on fixed-length
  sequential files (direct-access WRITE), which may need the file
  extended through the record written; the relative organization
  (prologue, cells with their deleted and present flags, `$GET`, `$PUT`,
  `$UPDATE`, `$DELETE`, `$FIND` by record number and sequentially),
  which needs ods2 to create files with FAB$C_REL's attributes. The
  survey settles which of these need ods2 changes.
- **Segmented records:** VAX FORTRAN's unformatted sequential files hold
  a logical record as variable-length RMS records, each beginning with a
  control word (first, last, middle, or only segment); the manual's
  RECORDTYPE section describes them.

## Open questions

- **Relative files' prologue and cell layout:** from the *File
  Applications* guide and the RMS manual; checking against a VMS-made
  relative file (an exchange volume) would confirm the layout.
- **DISPOSE='PRINT' and 'SUBMIT':** govax has no print or batch queues;
  the proposal is to keep the file and report that it wasn't queued.

## Subtasks (provisional)

1. **Survey:** chapter 9, appendix A.2–A.3, the RMS manual's relative
   files, and what RMS and ods2 lack.
2. **OPEN and CLOSE** on sequential files, the unit table, default
   names, the exit handler.
3. **INQUIRE.**
4. **REWIND, BACKSPACE, ENDFILE.**
5. **Unformatted sequential I/O** and segmented records.
6. **RMS keyed writes** on fixed-length sequential files; direct
   access, DEFINE FILE, FIND.
7. **RMS relative files** (and ods2 as needed).
8. **FORTRAN on relative files**, DELETE.
9. **USEROPEN, SHARED, READONLY**, and the remaining keywords.
10. **End to end**, then **close-out** (MANUAL.md's FORTRAN files section).

## Progress log

- **2026-10-29:** Planned with Phase 74. The author confirmed that ods2
  changes are fair game.
