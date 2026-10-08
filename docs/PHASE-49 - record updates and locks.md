# Phase 49 — Record updates, lock services, and file sections

**Status:** planned (2026-10-08). Needs Phases 43–48.

Phases 43–48 (the multiprocessing program; PHASE-43.md, Part A) left
some work for later, and their carry-forward sections pointed at it. On
2026-10-08, as the program closed, the author chose to gather that work
here rather than leave it scattered: this phase takes it over, and the
earlier docs point here.

## Goal

- **RMS record operations past sequential `$GET` and `$PUT`**: `$FIND`,
  `$UPDATE`, `$TRUNCATE`, and `$DELETE` where the RMS Reference Manual
  allows them for govax's organization (sequential); `RAB$V_TPT`
  (truncate on put); one stream both reading and writing (FAC=GET|PUT);
  `RAB$V_TMO` (a time limit on a record-lock wait).
- **The rest of the lock services**: `$GETLKI` and `$GETLKIW`; the
  ENQLM quota (and ASTLM for blocking and completion ASTs); deadlock
  detection, as the Internals book's chapter 13 describes it
  (SS$_DEADLOCK to the request chosen as victim).
- **File-backed sections**: `$CRMPSC` mapping a file's blocks, private
  and global, and `SEC$M_EXPREG` in P1 (both SS$_UNSUPPORTED now;
  Phase 46).
- **The terminal**: a `$QIO` read that must wait returns at once with
  the read pending, completing later (event flag, IOSB, AST), as on VMS,
  rather than waiting inside the `$QIO` (Phase 46).
- **Smaller items**: host files get the same open arbitration as volume
  files; a wildcard `$SEARCH` that began before a subdirectory was made
  sees it, as VMS's does; per-process buffered and direct I/O counts
  (JPI$_BUFIO, JPI$_DIRIO), shown by SHOW SYSTEM's I/O column, the
  termination message, and LOGOUT's report, which show 0 now (Phase 48).

## What earlier phases leave in place

- `internal/rms` (Phase 47): `Volume.Access` arbitration (FAC/SHR, per
  file, RMS$_FLK); one shared ods2 `*File` per open file (the FCB);
  shared sequential files appending at the current end of file; record
  locks by RFA on the lock database (`recordlock.go`), with RLK, ULK,
  NLK, RRL, WAT, `$FREE`, `$RELEASE`, `$FLUSH`, `$ERASE`.
- `internal/lck` (Phase 47): resources, the six modes, granted,
  conversion, and waiting queues, NOQUEUE, conversions, sublocks,
  CANCEL, DEQALL, value blocks, blocking notices. No deadlock detection;
  no quotas.
- `corevms/enq.go`: `$ENQ`, `$ENQW`, `$DEQ`; the `$GETLKI` macros exist
  (Phase 46's round 6) but the service doesn't.
- `corevms/gblsec.go` (Phase 46): page-file sections, private and
  global, `$CRMPSC`/`$MGBLSC`/`$DGBLSC`, UIC protection.
- `corevms/terminal.go` (Phase 46): the shared terminal, its read queue,
  and `awaitTerminal`.

## Subtasks

1. **Survey**: the RMS Reference Manual's and the File Applications
   guide's rules for each record operation on sequential files (which
   are allowed, on which record formats and devices, with what RAB
   fields and statuses), and the System Services manual's for
   `$GETLKI`'s items and the quotas. Note what only VMS can settle, for
   a probe.
2. **`$FIND`** (sequential and by RFA) and its record lock.
3. **`$UPDATE`** (same-length records in place) and **`$TRUNCATE`**,
   **`RAB$V_TPT`**; **`$DELETE`** as far as the manual allows it for a
   sequential file (likely RMS$_IOP).
4. **A stream reading and writing** (FAC=GET|PUT): the current record
   and the next record pointer as the manual describes them.
5. **`RAB$V_TMO`**: a record-lock wait with a time limit (RMS$_TMO).
6. **`$GETLKI(W)`**: the items govax's lock database can answer, by
   lock ID and by wildcard.
7. **Quotas**: ENQLM (SS$_EXENQLM), and ASTLM for the lock services'
   ASTs.
8. **Deadlock detection**: a search of the wait-for graph when a request
   has waited (the book's description: the timeout, the search, the
   victim), SS$_DEADLOCK.
9. **File-backed sections**: `$CRMPSC` with a channel, private and
   global, written back on `$DELTVA`/`$UPDSEC`; `SEC$M_EXPREG` in P1.
10. **Pending terminal reads**: a `$QIO` read returns SS$_NORMAL with the
    read queued; the terminal's queue completes it.
11. **Smaller items**: host-file arbitration, `$SEARCH` and new
    subdirectories, per-process I/O counts.
12. **Probe** (optional): a VMS 7.3 run for what the survey leaves open.
13. **Close-out**.

## Open questions

- Which of these the author wants first, or at all; the order above is
  a suggestion (RMS first, as programs are likelier to need it).
- Whether relative or indexed files belong in a later phase of their own
  (out of scope here: govax has sequential files only).

## Progress log

- 2026-10-08: Planned, from the carry-forward sections of Phases 45–48
  (PHASE-48.md's close-out).
