# Phase 49 — Record updates, lock services, and file sections

**Status:** done (2026-10-09). Needs Phases 43–48. Every subtask is
done; a VMS 7.3 probe in three rounds (`testdata/probe49`) settled what
the manuals left open, and what is still govax's choice is listed in
DEVIATIONS.md ("[Phase 49] Rules chosen without a manual or probe").

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

- ~~Which of these the author wants first~~ (the suggested order, RMS
  first; all done).
- Whether relative or indexed files belong in a later phase of their own
  (out of scope here: govax has sequential files only). Still open.

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
- 2026-10-08: Subtask 7, the lock quotas (`corevms/lockquota.go`).
  ENQLM is govax's pooled job quota (`Job.Pooled.ENQLM`; the book's
  JIB$W_ENQLM): every lock of every process in the job counts, RMS's
  file and record locks included, and a new lock past it is
  SS$_EXENQLM (`$ENQ`) or RMS$_EXENQLM (a record lock, through the new
  `rms.Context.CanLock`); a conversion takes none. JPI$_ENQCNT (new) is
  what's left. ASTLM: a lock request's completion AST counts as
  outstanding from the request until it's delivered (`remainingASTs`,
  so JPI$_ASTCNT shows it), and an `$ENQ` asking for a completion or
  blocking AST with none left is SS$_EXQUOTA (unconfirmed;
  SS$_EXASTLM is the other candidate). An armed blocking AST isn't
  counted (unconfirmed). Only the lock services enforce ASTLM; the
  other AST services still don't. Tests: `TestEnq_quotas`,
  `TestRecordLock_quota`.
- 2026-10-09: Subtask 8, deadlock detection (`internal/lck/deadlock.go`),
  from the Internals book, section 13.3. Each queued request gets a due
  time DEADLOCK_WAIT (SYSGEN's default, 10 seconds) after it queues
  (`Manager.Now`, `DeadlockWait`); `CheckDeadlocks` searches from each
  request past due, following `Blocks` and then the blocking owners'
  queued requests, a process searched only once (the book's bitmap); a
  search back to the starting process is a deadlock. Conversion
  deadlocks fall out of the same search (`Blocks` counts a queued lock
  ahead). The victim's request is refused (`EventDeadlock`): a new lock
  removed (`Lock.Deadlocked`), a conversion back to its granted mode;
  `$ENQ` completes with SS$_DEADLOCK, an RMS record lock wait with
  RMS$_DEADLOCK. A search that finds nothing sets a new due time.
  corevms runs it before each scheduling choice (`pollEvents`,
  `System.checkDeadlocks`), and `nextTimer` includes the next due
  search, so an idle machine (all waiting) jumps to it. Unconfirmed:
  the victim is the request the search started from (every govax
  process's deadlock priority is 0, and the book doesn't say which of
  several zero-priority participants VMS refuses); no search without
  the scheduler; RAB$V_NODLCKWT/NODLCKBLK are ignored; DEADLOCK_WAIT is
  fixed, not a setting. Tests: `lck`'s `TestDeadlock_*`,
  `TestEnq_deadlock`, `TestRecordLock_deadlock`.
- 2026-10-09: Subtask 9, file-backed sections, from the System Services
  manual ($CRMPSC, $UPDSEC, $EXPREG; VMS 5.0's) and the Internals book's
  chapter 14 (the PTE forms, the process section table, the global page
  table). Two design decisions, the author's: pages come in lazily, on
  the first touch, through a pager hook in internal/vm, so that a
  working set manager and a page file can come later behind the same
  seam; and the channel comes from RMS's user file open, which govax's
  RMS gained here, for files on mounted volumes.
  - **internal/vm** (`pager.go`): the invalid PTE forms (TYP0, bit 26: a
    process section table index in bits 21-0; TYP1, bit 22: a global
    page table index), and `Pager`, installed with `SetPager`: every
    fault on an invalid page whose access is allowed goes to it (with
    the address space, the address, the PTE and its address), and the
    valid PTE it returns is written back; without a pager every invalid
    page is demand-zero, as before. `DemandZero` is the helper a pager
    uses for an ordinary page.
  - **RMS** (`rms/ufo.go`): FAB$V_UFO on `$OPEN`, `$CREATE`, and the NAM
    open. The open runs as usual (sharing, NAM, XABs), then its file
    access becomes an `ACPFile` (keeping the open's place among the
    file's openers) on a new channel (`Context.AssignFileChannel`,
    corevms's `assignFileChannel`, at the caller's mode), whose number
    goes to FAB$L_STV; FAB$W_IFI stays 0. `ACPFile.ReadPage`/`WritePage`
    are page I/O: any allocated block, the end of file untouched.
  - **corevms** (`filesec.go`, `gblsec.go`): `$CRMPSC` of a file on a
    channel, private (no SEC$M_GBL; an entry in the process section
    table, `Environment.procSections`; status SS$_NORMAL) or global (an
    entry per page in the system's global page table,
    `GlobalSections.pageTable`). Each mapped page gets the invalid form;
    `System.PageIn` reads the block into a new page on the first touch
    (a global section's page once, then shared; a copy-on-reference
    page into a private copy for each process). A private section's
    modified pages (the PTE's modify bit) are written back when they're
    unmapped ($DELTVA, $CRETVA over them, image rundown, process
    deletion: all through `releasePTE`); a global section gathers its
    mappers' modify bits as their PTEs go (`Dirty`) and writes its pages
    back when it's deleted. `$UPDSEC`/`$UPDSECW` write pages back on
    request (updflg 0: every page of a global section in memory; 1: the
    caller's modified ones), completing at once as `$GETLKI` does. A
    section keeps its file accessed after its channel is deassigned or
    deaccessed (`sectionFiles`); the last section deaccesses it.
    Page-file sections are unchanged (frames at creation, valid PTEs).
    SEC$M_PFNMAP stays SS$_UNSUPPORTED.
  - **P1 expansion**: P1's tables are built at full size, so P1 can't
    grow down past its table as VMS's does; govax keeps the top
    `userStackPages` (1,024) pages below the user stack's top for the
    stack and expands P1 downwards below them (`expandP1`), to P1LR's
    page (then SS$_VASFULL). SEC$M_EXPREG in P1 maps there, low to high
    as the manual says; `$EXPREG` of region 1, which counted up from
    address 0 (a leftover of the port), now takes its pages there too,
    creates them as $CRETVA does, and returns the range high page
    first, as the manual describes.
  - A global section created with no inadr (and no SEC$M_EXPREG) is
    created but not mapped (the manual's case for a permanent one).
  - Statuses: SS$_NOPRIV for a channel not assigned or assigned from a
    more privileged mode, SS$_NOTFILEDEV for one not to a disk,
    SS$_NOWRT for SEC$M_WRT (without SEC$M_CRF) on a read-only access,
    SS$_ENDOFFILE for vbn past the file (the manual's).
  - Macros: `$UPDSEC`, `$UPDSECW` (_S, list, _G) in govax's STARLET.
  - Unconfirmed (for a probe): the section's size counts the file's
    allocated blocks, not its end of file (so a file just created with
    ALQ maps); a disk channel with no file accessed is SS$_FILNOTACC;
    `$MGBLSC` with SEC$M_WRT of a file section created read-only is
    SS$_NOWRT; a private section's status is SS$_NORMAL; section page
    writes leave the end of file alone; `$UPDSEC` passes over pages a
    more privileged mode owns, returns 0 in the IOSB's second longword
    when every page was written, and with SS$_NOTMODIFIED sets the event
    flag and writes the IOSB but queues no AST; the `$UPDSEC` macros'
    argument pairing and required INADR; a UFO open of a non-disk device
    is RMS$_SUPPORT (VMS gives a channel); the user stack's reserve.
  - Not done: the user stack isn't kept out of P1's expansion region
    (a stack deeper than 1,024 pages would run into it); the page fault
    cluster (pfc) is ignored; SHOW PAGE doesn't decode the invalid
    forms; page-file sections still take their frames at creation.
  - Tests: `vm`'s `TestPTEForms`, `TestPagerPageIn`, `TestPagerRefuses`,
    `TestPagerThroughSpace`, `TestDemandZero`; `rms`'s `TestUFO_create`,
    `TestUFO_open`, `TestUFO_refused`; `TestRun_fileSections`
    (`testdata/sec49/mapfile.mar`, 44 steps: private sections in P0 and
    P1, $UPDSEC, $DELTVA's write-back, SS$_NOWRT, a global section
    mapped twice, a section outliving its channel, $EXPREG of P1,
    $UPDSEC's updflg, a global copy-on-reference section);
    `TestFileSectionPair` (`secparent.mar`, `secchild.mar`: a global
    file section shared by two processes, under two quanta);
    `TestFileSectionRundown` (`rundown.mar`: image rundown writes a
    private section back, and not a copy-on-reference one).
- 2026-10-09: Subtask 10, pending terminal reads (`corevms/terminal.go`,
  `ttdriver.go`). A terminal `$QIO` read that can't finish at once (a
  read ahead of it in the terminal's queue, or its line not typed yet)
  now returns SS$_NORMAL with the request pending (`queueTerminalRead`),
  in the terminal's queue beside the synchronous readers (RMS's $GET,
  LIB$GET_INPUT, the input shims, which still wait in their service).
  Before each scheduling choice `pollEvents` calls `serviceTerminal`,
  which reads the line of each pending read at the head of the queue
  whose line is there (`readQIOLine`, storing through the owner's
  address space, since another process may be current) and completes
  it: IOSB, event flag, AST, and the owner's wait ended, boosted as
  terminal input. A synchronous read finishing services the next read at
  once (`dropTerminalRead`). `$CANCEL`/`$DASSGN`/rundown complete a
  pending read with SS$_CANCEL and take it out of the queue
  (`pruneTerminalQueue`). The queue's entries are now per read, not per
  process, so a process may have a pending `$QIO` read and a synchronous
  one. `$QIOW` of the terminal now waits as for any pending request
  (no more retrying the `$QIO`). Unchanged: with the scheduler off, or a
  console input that isn't a `TerminalSource`, a read blocks as before;
  IO$M_TIMED with a nonzero limit still waits without a limit (now
  implementable through the pending request, not done); IO$M_PURGE
  discards what's typed ahead when the read is made, even if reads ahead
  of it would have read it (unconfirmed). Tests: `TestTerminal_pendingQIO`,
  `TestTerminal_typedAhead`, `TestTerminal_pendingOrder`,
  `TestTerminal_cancelPending`; the console's `TestPendingTerminalRead`
  (a MACRO program working while its read is pending), and
  `TestSharedTerminal` ($QIOW) unchanged.
- 2026-10-09: Subtask 11, part 1: per-process I/O counts
  (`corevms/iocount.go`), from the Internals book, section 18.3.3 (I/O
  postprocessing increments PHD$L_DIOCNT or PHD$L_BIOCNT as a request
  completes). `Process.BufferedIO`/`DirectIO`; JPI$_BUFIO and JPI$_DIRIO;
  SHOW SYSTEM's I/O column (their sum); the termination message's
  ACC$L_BIOCNT/ACC$L_DIOCNT; LOGOUT's report. Counted: every `$QIO`
  completing for a process (`completeIO`; a cancelled one too, a
  rejected one not), direct for a disk, buffered otherwise; one
  buffered I/O per record RMS reads or writes on the terminal, a
  mailbox, or NL: (`rms.Context.CountBufferedIO`, the record device's
  Put and Get), and per LIB$PUT_OUTPUT/LIB$GET_INPUT line; and, as
  direct I/O, the ods2 block operations each RMS service did on the
  mounted volumes (`countVolumeIO`, by `MountTable.TotalOperations`
  before and after; the RMS services are now registered from one table,
  `rmsServices`, through it), and LIB$PUT_OUTPUT's to a file.
  Unconfirmed approximation: ods2's caching and transfer sizes aren't
  RMS's, so the direct counts won't match a VMS run's; section paging
  isn't counted (paging I/O, as on VMS). Tests: `TestIOCount_terminal`,
  `TestIOCount_disk`, `TestIOCount_volume`; the subprocess CLI's LOGOUT
  report now shows its buffered count.
- 2026-10-09: Subtask 11, part 2: host-file arbitration
  (`rms/hostshare.go`). A running program reaches host files only
  through the console's EXE$OPEN shim (RMS opens volume files only), so
  that's what is arbitrated: `rms.HostOpeners` (`System.HostOpeners`)
  keeps each host file's openers, known by the host's file identity
  (`os.SameFile`), and applies sharing.go's rule (the guide's per
  operation one, the same `opener.compatible`). EXE$OPEN's flags map to
  an access: read-only is FAB$M_GET (sharing reading, RMS's default),
  anything else a writer's (sharing nothing); the claim is made before
  the host open, so a refused open (-1) doesn't truncate the file; a
  file the open creates is attached once it's open. EXE$CLOSE and image
  rundown (`CloseFiles`) release it. Not arbitrated: the console's own
  commands' host files (they run while no program does) and other host
  programs. Test: `TestShimExeOpenSharing`.
- 2026-10-09: Subtask 11, part 3: `$SEARCH` and new subdirectories
  (`rms/search.go`). A wildcard directory search's list of directories
  was walked once, when the search (or its search list element) began.
  Now, as each directory is finished (`nextDir`), the tree is walked
  again (`searchDirs`) and the search goes on with the first directory
  after the current one that it hasn't searched (`searchState.Visited`,
  by path): a subdirectory made meanwhile is searched if it comes later
  in the walk, and one made behind the search's place isn't, as a walk
  that reads each directory when it reaches it would behave. If the
  current directory has been deleted, the search goes on through the
  walk it had. Unconfirmed (for a probe): that VMS's search sees a
  directory made ahead of it and not one made behind it. Test:
  `TestSharing_searchNewSubdirectory`. Subtask 11 is done.
- 2026-10-09: A fix found while writing the probe (subtask 12): `$OPEN`
  still refused a FAB whose only access was DEL or TRN (RMS$_PRV), a
  check from before this phase implemented them; it's gone. And
  `$OPEN` now counts every write access (PUT, UPD, DEL, TRN) as asking
  to write (`openFID`), so a TRN-only stream's truncation is written
  back at `$CLOSE`, and such an open on a volume mounted read-only is
  RMS$_PRV. Tests: `TestTruncate_trnOnly`, `TestSysOpen_facDeleteOnly`
  (replacing `TestSysOpen_facUnimplementedOnly`).
- 2026-10-09: Subtask 12, the probe, prepared (`testdata/probe49`,
  README.md): `probe6.mar`, one process, nine steps asking what the
  subtasks left unconfirmed: $UPDATE to another length (variable and
  stream), an RFA inside a record, UPD-only and TRN-only streams' $GET,
  record locks between two streams ($PUT with TPT, $TRUNCATE, NLK and
  $UPDATE, WAT with TMO 0 and 2), `$GETLKI`'s wildcard context, a
  waiting lock's granted mode, a short LKI$_LOCKS buffer, a process
  deadlocked on its own lock, file sections (the size of an unwritten
  ALQ file, the end of file after section writes, `$UPDSEC` with
  nothing modified, a disk channel with no file, `$MGBLSC` WRT of a
  read-only file section, UFO on NLA0:), the I/O counts of four
  operations, and a `$SEARCH` seeing directories made during it.
  `TestProbe6` runs it under govax and logs the report; the README
  lists govax's answers beside each question. `probe6.com` keeps
  MACRO's output in a log of its own, on the clean-room hook's
  unaudited list until the author has checked it. Waiting for the VAX
  run. Not asked: terminal reads (IO$M_PURGE with reads queued, a
  timed read), the deadlock victim among several processes, ASTLM.
- 2026-10-09: The VAX run of the probe (`testdata/probe49/vax/probe6.log`;
  the MACRO log empty, audited by the author). What it settled, and
  govax now does:
  - Step 2: VMS doesn't check that an RFA starts a record; one byte into
    a variable-length record it read a length word from there and
    returned RMS$_EOF. `validRFA` checks only that the RFA is in the
    file, and a record reached by RFA that ods2 finds corrupt (its
    length past the end) is RMS$_EOF (`locate`).
  - Step 5a: the wildcard context `$GETLKI` writes back is ^XFE in the
    high byte and the lock's index below (FE0000E6 for lock 1A0000E6);
    govax's is FE and its lock ID (`lkiWildcardContext`). VMS's process
    held other locks too (DCL's, RMS's), so the scan didn't reach
    SS$_NOMORELOCK in six calls.
  - Steps 5b, 5c, 6: as govax (NL for a waiting lock's granted mode;
    80180018; SS$_DEADLOCK after 9 seconds for a process waiting for
    its own lock).
  - Step 7d: a disk channel with no file is SS$_IVCHNLSEC, not
    SS$_FILNOTACC.
  - Step 7f: a UFO open of NLA0: succeeds with a channel to the device
    in FAB$L_STV (`userFileOpen`, `AssignFileChannel` with no file).
  - Step 8: a `$QIOW` write to NLA0: is direct I/O; a LIB$PUT_OUTPUT
    line to a file counts nothing; `$CREATE`, ten `$PUT`s, `$CLOSE` were
    3 buffered and 7 direct; `$OPEN`, `$GET`s, `$CLOSE` 2 and 1. The
    count of ods2's block operations (29 and 16) is gone; RMS counts by
    a model fitted to these two totals (`rms/iocount.go`: `$CREATE` 2
    and 5, `$OPEN` 1 buffered, `$CLOSE` 1 and 1 direct if written, one
    direct I/O per 16-block buffer read or written, `$ERASE`/`$RENAME`
    1 buffered; `Context.CountIO`). Unconfirmed beyond the two totals.
  - Steps 1 (the variable-length file), 3, 4a to 4d, 9: as govax.
  - Left open: step 1's stream file (VMS's `$UPDATE` to another length
    succeeded: what does the file hold then?); step 4e's 2-second case
    (VMS answered RMS$_RFA at once: the failed TMO=0 `$GET` must have
    changed RAB$W_RFA, which the probe didn't set again); and step 7,
    where every case after 7a looks like the knock-on of a failed
    `$CREATE` in 7a (its status wasn't printed; with no channel,
    `$CRMPSC` of a private section is SS$_IVSECFLG). A second round
    asks these.
  Tests: `TestFind_byRFA`, `TestGetlki_access`, `TestUFO_device`,
  `TestIOCount_null`, `rms`'s `TestIOCount_model`.
- 2026-10-09: A fix found by the probe's second round: `$GET` didn't
  set RAB$L_RBF. The RMS manual (section 7.16) says the Get service sets
  it to the record's address, in move mode the user buffer
  (`storeRecordStatus`); a program printing RBF after `$GET` printed
  whatever it last pointed RBF at. The RMS tests' `$GET` helper now
  reads the record through RBF.
- 2026-10-09: The probe's second round prepared (`testdata/probe49`,
  `probe6b.mar`; README.md, "Round 2"): a stream file's `$UPDATE` to
  another length and the file read back; the record-lock timeout with
  the RFA set again, and what the RAB holds after; and the file-section
  cases with every status printed, a written file's section size added.
  `TestProbe6b` runs it under govax. Waiting for the VAX run.
- 2026-10-09: The probe's second round on VMS (`vax/probe6b.log`; its
  MACRO log audited). Settled, and govax now does:
  - A stream file's `$UPDATE` to another length writes the record and
    its terminator over the file's bytes, moving nothing (past the end
    of the file, RMS$_RSZ, unconfirmed; `streamTerminator`). The RMS
    manual's STREAM rule (records end at CR LF, LF, VT, or FF; the last
    three kept in the record; leading nulls skipped; a lone CR is data)
    is now ods2's reader's (sibling module, commit f9df5a9: needs a
    tagged release and govax's go.mod pin before `GOWORK=off` builds
    pass). The COPY /IGNORE tests' corrupt file is now a Variable
    record whose length runs past the end.
  - A `$GET` that times out (RMS$_TMO) clears RAB$W_RFA (RMS$_RLK's
    unconfirmed: left alone).
  - A user file open of a write-shared file (SHRPUT, SHRUPD, SHRDEL)
    without FAB$V_UPI is RMS$_SHR (the RMS manual, FAB$V_UFO and
    FAB$V_UPI; `ufoSharing`), and a UFO `$CREATE` puts the end of file
    at the end of block ALQ (the manual; `ufoEndOfFile`).
  - A private file section is SS$_CREATED.
  - `testdata/sec49`'s programs follow: UPI on the shared UFO opens,
    SS$_CREATED for private sections.
  A third round (`probe6c.mar`) asks step 7 again with UPI: the end of
  file after a section write, a written file's section size,
  `$UPDSEC`, `$MGBLSC` writable of a read-only file's section.
  Tests: `TestUpdate_stream`, `TestRecordLock_timeout`, `TestUFO_create`,
  `TestProbe6b`, `TestProbe6c`; ods2's `TestReaderStreamCRLFTerminators`.
- 2026-10-09: The probe's third round on VMS (`vax/probe6c.log`; MACRO
  log audited). Settled, and govax now does: a file section ends at the
  file's end of file, not its allocation (`fileSectionSize`; a UFO
  `$CREATE`'s file ends at block ALQ, so it maps every block asked for);
  the end of file a UFO `$CREATE` sets comes from FAB$L_ALQ as the
  program set it, not the cluster-rounded allocation `$CREATE` writes
  back (`ufoEndOfFile`); `$UPDSEC`'s IOSB second longword is the first
  page of the range not written (the manual's words; when every page
  was written, the page past the range, unconfirmed); `$MGBLSC`
  writable of a read-only file section is SS$_NOPRIV. Confirmed as
  govax had them: SS$_NOTMODIFIED's IOSB (0 in the second longword) and
  no AST; the AST and SS$_NORMAL after a write. `TestProbe6c` runs on a
  volume of 3-block clusters, as VMS's was, and checks the 7c and 7d
  lines whole. Subtask 12 is done: every difference left between the
  probe's three reports and VMS's is the environment's.
- 2026-10-09: Subtask 13, the close-out. DEVIATIONS.md has the rules
  still chosen without a manual or probe ("[Phase 49] Rules chosen
  without a manual or probe"), and Phase 46's entry marks the terminal
  `$QIO` read fixed; PLAN.md's index and CLAUDE.md say the phase is
  done. Along the way the phase fixed bugs outside its own scope:
  `$OPEN` refusing DEL- or TRN-only access, `$GET` not setting
  RAB$L_RBF, and ods2's STREAM reader (CR LF only; ods2 v0.1.20 ends
  records at LF, VT, and FF too). Left for later: relative and indexed
  files; IO$M_TIMED's time limit on a terminal read (now possible
  through the pending request); a page file and working set manager
  behind `vm.Pager`; keeping the user stack out of P1's expansion
  region; the page fault cluster; SHOW PAGE decoding the invalid PTE
  forms.
