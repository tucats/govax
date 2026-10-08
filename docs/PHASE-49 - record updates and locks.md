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

## Survey (subtask 1)

From the RMS Reference Manual (OpenVMS 7.3, `OVMS_731_RMS.pdf`: the
services, RAB$L_ROP, RAB$B_TMO, FAB$B_FAC) and the Guide to OpenVMS File
Applications (7.3: section 8.2, Table 8-1, and 8.6's record stream
context, Table 8-3).

**Which operations a sequential file has** (guide, Table 8-1): $GET,
$PUT (at the end of file only, but for RAB$V_TPT and a random
$PUT with UIF on fixed-length records), $FIND, and $UPDATE (disk
only; "the record length for sequential files cannot change"). $DELETE
"removes an existing record from a relative or indexed file. You cannot
use this service when processing sequential files"; its condition
values include RMS$_IOP, the status govax returns.

**Access** (FAB$B_FAC): $UPDATE needs FAB$V_UPD; $TRUNCATE and
RAB$V_TPT need FAB$V_TRN ("This option applies only to sequential
files"); a $PUT with TPT without TRN is RMS$_FAC (the manual, $PUT).
$DELETE needs FAB$V_DEL.

**The stream's context** (guide 8.6, Table 8-3). Each RAB has a
*current record* and a *next record*:

| Service | Current | Next |
| --- | --- | --- |
| $CONNECT | none | first record (RAB$V_EOF: end of file) |
| $GET, sequential, last service not $FIND | old next | new current + 1 |
| $GET, sequential, after a $FIND | unchanged | current + 1 |
| $GET, random (RFA) | new | new current + 1 |
| $PUT, sequential file | none | end of file |
| $FIND, sequential | old next | new current + 1 |
| $FIND, random | new | unchanged |
| $UPDATE | none | unchanged |
| $TRUNCATE | none | end of file |
| $REWIND | unchanged | first record |
| $FREE, $RELEASE | none | unchanged |

The current record is undefined after $CONNECT, after any failed
operation, and after any successful service but $GET and $FIND; then
$UPDATE, $DELETE, $RELEASE, and $TRUNCATE are rejected (RMS$_CUR, in
their condition values). A failed operation leaves the next record
alone. $FIND writes RAB$W_RFA; RAB$L_RBF and RAB$W_RSZ are undefined
after it. $UPDATE writes RAB$W_RFA too.

**$UPDATE**: the record must be locked by this stream (by its $FIND or
$GET), in move mode (RAB$L_RBF, RAB$W_RSZ). A sequential file's record
can't change length (RMS$_RSZ, govax's choice of status). For a stream
format file "the Update service functions in the same manner as the Put
service, with one exception: ... you do not have to set ... RAB$V_TPT to
update data in the middle of a file": govax takes that as the same
in-place rule, the length unchanged (unconfirmed: what VMS does with a
stream record of another length).

**$TRUNCATE**: "resetting the logical end-of-file position to the
beginning of the current record"; in sequential access only immediately
after a successful $GET or $FIND. Space isn't freed or erased. A
truncated file's records past the end are gone for every stream.

**RAB$V_TPT** (truncate on put): a sequential $PUT "can occur at any
point in the file, truncating the file at that point. The end-of-file
mark is set to the position immediately following the last byte
written." Without it, a $PUT anywhere but the end of file is RMS$_NEF
(govax's rule since Phase 47).

**RAB$V_TMO** with RAB$V_WAT: RAB$B_TMO is the longest wait in seconds
(0 to 255) for a locked record; when it runs out the operation fails
with RMS$_TMO. (TMO also has terminal and mailbox meanings, not this
subtask's.)

**$FIND** takes RAB$B_RAC SEQ or RFA (KEY is for relative and indexed
files: RMS$_RAC here), RAB$V_NLK, RLK, REA, RRL, ULK, WAT, TMO, and
returns RMS$_OK_* as $GET does. It locks the record as $GET would.

**What only VMS can settle** (unconfirmed choices, for a probe):
$UPDATE's status for a changed length on a sequential file (govax
RMS$_RSZ); $UPDATE of a stream file's record to another length; the
status for an RFA that doesn't start a record (govax RMS$_RFA); whether
$PUT with TPT locks the record it writes; whether $TRUNCATE drops the
stream's record locks.

## Open questions

- Which of these the author wants first, or at all; the order above is
  a suggestion (RMS first, as programs are likelier to need it).
- Whether relative or indexed files belong in a later phase of their own
  (out of scope here: govax has sequential files only).

## Progress log

- 2026-10-08: Planned, from the carry-forward sections of Phases 45–48
  (PHASE-48.md's close-out).
- 2026-10-08: Started, in the suggested order (RMS first). Subtask 1,
  the survey, is above ("Survey").
- 2026-10-08: Subtasks 2 to 4, in one change since they share the
  stream's context. `internal/rms/stream.go` keeps each stream's current
  and next records (the guide's Table 8-3) in `FileHandle.stream`, and
  `locate` is $GET's and $FIND's common step: the record by RAB$B_RAC
  (sequential or RFA; `validRFA` checks an RFA is in the file and, for
  Fixed and Variable records, on a record boundary), its lock
  (`lockFound`), and the context's move. The record is read again after
  a lock wait, so a stream that waited sees the other's $UPDATE (the old
  `streamLocks.pending` copy is gone). `recordops.go` has `$FIND`,
  `$UPDATE` (in place, same length; `writeAt`), `$TRUNCATE`
  (`truncateAt`: the end of file moves; blocks stay), `$DELETE`
  (RMS$_IOP: sequential files only), and `$REWIND`; `$PUT` checks the
  end-of-file rule and RAB$V_TPT (`putPosition`), replacing
  `FileHandle.NotAtEOF`. `$CONNECT` arms a Reader and a Writer as
  FAB$B_FAC asks (both for GET and PUT); a stream that also reads or
  changes records in place gets a shared (write-through) Writer and
  reads from the disk each time (`streamContext.fresh`). Wrong-direction
  $GET and $PUT are now RMS$_FAC, as the manual lists, not RMS$_PRV.
  Decisions, unconfirmed: UPD, DEL, and TRN access let a stream $GET and
  $FIND (`facReads`); $UPDATE in a record-locking stream needs the
  record locked (RMS$_RNL); a stream-format record can't change length
  either (RMS$_RSZ); $FIND on the terminal or a mailbox reads a record
  and drops it; $REWIND there does nothing. ods2 (sibling module)
  gained `Reader.Offset`/`SeekTo`, and a shared Writer now rereads the
  file's last block at each Put (an in-place update there isn't
  overwritten): ods2 commit e4697bd, not yet tagged, so govax's go.mod
  pin needs a new ods2 release before `GOWORK=off` builds. Tests:
  `recordops_test.go` (eight), and `TestRun_recordUpdates`
  (`testdata/rms49/update.mar`, a MACRO program run end to end).
- 2026-10-08: Subtask 5, RAB$V_TMO. When a record lock wait (RAB$V_WAT)
  begins with TMO set, the stream's `streamLocks.deadline` is
  RAB$B_TMO seconds on (`lockDeadline`, by the new `Context.Clock`);
  called again after it with the lock still not granted, the $GET or
  $FIND gives up the wait and fails with RMS$_TMO (the record stays the
  next, and the stream's locks go unless ULK). `Context.AwaitLock` now
  takes the deadline: corevms keeps it as `Environment.waitDeadline`,
  which ends the wait, and which `nextTimer` counts so an idle machine
  moves time to it; it isn't a timer request ($CANTIM, JPI$_TQCNT don't
  see it). A TMO of 0 times out at the first look after the wait begins
  (unconfirmed: VMS may refuse at once, as without WAT). TMO's terminal
  and mailbox meanings are untouched. Tests: `TestRecordLock_timeout`,
  `TestLocks_waitDeadline`.
- 2026-10-08: Subtask 6, `$GETLKI` and `$GETLKIW` (`corevms/getlki.go`),
  from the System Services manual (VMS 5.0's, the one on hand). Items:
  PID, STATE, PARENT, LCKREFCNT, RSBREFCNT, LOCKID, LKID, MSTLKID,
  REMLKID, CSID, MSTCSID, SYSTEM (cluster IDs 0, a lock its own master),
  NAMSPACE, RESNAM, VALBLK, GRANTCOUNT/LCKCOUNT, CVTCOUNT, WAITCOUNT,
  and the lists BLOCKING, BLOCKEDBY, LOCKS (24-byte entries,
  LKI$C_LENGTH); the range and byte-range items are SS$_BADPARAM. The
  return length is the manual's longword (bytes, entry size in 16-30,
  bit 31 when the buffer was short; govax writes whole entries only).
  Access: the caller's mode must be the lock's or inner (SS$_IVMODE), a
  system-wide resource needs exec/kernel mode or SYSLCK (SS$_NOSYSLCK),
  another group's lock WORLD (SS$_NOWORLD); a wildcard scan (lock ID 0
  or -1) skips what the caller may not see and ends with
  SS$_NOMORELOCK. It completes at once, as $GETJPI does (event flag,
  IOSB, AST). `internal/lck/info.go` gained `All`, `Sublocks`,
  `Subresources`, `QueueLengths`, and the blocking relation `Blocks`
  (with `Blockers`, `BlockedBy`): a lock holding an incompatible mode,
  or queued ahead asking for one (the queues are granted in order) —
  the book's rule for the deadlock search, which subtask 8 will follow.
  Minimum argument count 3 (the manual's required arguments).
  Unconfirmed: the wildcard context written back to LKIDADR (the high
  bit and the last lock ID); a waiting lock's granted mode (reported as
  NL); whole list entries on a short buffer.
