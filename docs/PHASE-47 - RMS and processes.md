# Phase 47 — Multiprocessing, part 5: files shared between processes

**Status:** in progress (subtasks 1-4 done 2026-10-08); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Needs Phase 45 (independent of Phase
46).

The program this phase belongs to is described in
[PHASE-43 - processes](PHASE-43%20-%20processes.md), Part A. Read that first.

## Goal

Two processes using one ODS-2 volume, or one file, must not corrupt it,
and must see VMS's sharing rules:

- **File access arbitration**: an open whose access (`FAB$B_FAC`) or
  sharing (`FAB$B_SHR`) conflicts with another accessor's fails with
  RMS$_FLK, by the RMS Reference Manual's rules.
- **One file, one state**: every accessor of a file sees the same file
  header, extent map, and end of file, so one process's extension or
  append is never lost when another closes the file.
- **Shared sequential files**: with write sharing requested, several
  processes can append to one sequential file, and a reader sees records
  appended after it opened the file.
- **Record locks** where RMS takes them for write-shared files.
- **A lock manager** (`internal/lck`) underneath, as VMS's RMS and XQP
  use VMS's, and **`$ENQ`/`$DEQ`** on it for programs (Decision 10).

## What earlier phases leave in place

- `internal/rms`: per-process file tables (`ifi.go`), each open file an
  ods2 `volume.File` plus an ods2 `rms.Reader` or `rms.Writer`; sequential
  organization; `FAB$B_SHR` ignored. Mounted volumes are shared
  (`rms.MountTable`), and each volume's storage and index bitmaps are
  cached once per volume (`ods2/volume/volume.go`), so allocation is
  already coherent across files.
- The ACP path (`$QIO IO$_ACCESS`, `acp.go`): per-volume access counts by
  file ID, and deletion deferred until the last access ends
  (`mountedVolume.accessed`, `doomed`).
- Every RMS or ACP service runs to completion in Go before the next
  instruction, so no two services ever interleave: the risk is **state
  kept across calls**, not concurrent execution.
- Phase 44's waits and Phase 45's rundown.

## What can go wrong today (to confirm in subtask 1)

- **Per-handle header copies** (bug 8). `volume.File` holds its own
  `Header` and `Extents` (`ods2/volume/file.go`) and writes the header back
  on `Close`. Two writable handles to one file: the second close
  overwrites the first's header, losing extents (blocks allocated and then
  orphaned: a bitmap/header mismatch) or the end of file.
- **Writer state**: an ods2 `rms.Writer` buffers the current block and
  knows the end of file; two writers append at the same place, the second
  overwriting the first's records.
- **Reader state**: an ods2 `rms.Reader` reads up to the end of file it
  saw at open, and may buffer a block; it won't see later appends.
- **Directory changes** (create, delete, rename, supersede) are each
  atomic in Go, but a file can be deleted or superseded while another
  process has it open (the ACP path defers deletion; RMS's may not).

## Design

### `internal/lck`: a minimal lock manager

A leaf package modeled on the VMS lock manager as the System Services
manual describes `$ENQ`:

- **Resources** named by strings, in a tree (a lock may have a parent;
  RMS uses a file lock as the parent of its record locks), scoped by UIC
  group or system-wide.
- **Six modes**: NL, CR, CW, PR, PW, EX, with the manual's compatibility
  table; a request is granted if compatible with every granted lock and
  no earlier request waits (FIFO), else it waits, or fails at once with
  `LCK$M_NOQUEUE`.
- **Conversions** (up and down) with the manual's queueing rules.
- **Owners** are processes; a process's locks are released at its
  rundown (and image-mode locks at image rundown).
- **Waiting** is Phase 44's: the waiting request's predicate is "granted".
- **Value blocks** (16 bytes per resource) only if RMS or a test needs
  them.
- **Deadlock detection** is out of scope (logged); a test shows that a
  deadlock waits forever and Ctrl-C works.

### `$ENQ`, `$ENQW`, `$DEQ`

The services over `internal/lck`: the lock status block, `LCK$M_CONVERT`,
`NOQUEUE`, `SYNCSTS`, `SYSTEM`, the completion AST and event flag, the
blocking AST (delivered to holders of a lock that blocks a new request,
as the manual describes), `$DEQ` of one lock or all (`LCK$M_DEQALL`).
Status codes as the manual's. `$GETLKI` only if needed.

### File access arbitration

On `$OPEN`, `$CREATE`, and the ACP's IO$_ACCESS, RMS computes the
accessor's access and sharing — `FAB$B_FAC`, and `FAB$B_SHR` with the
manual's defaults (SHRGET when FAC is GET; NIL, i.e. no sharing, when it
includes PUT, UPD, DEL, or TRN) — and takes a lock on the file's
resource (volume label or device plus file ID) whose mode encodes them,
or keeps an explicit accessor list per file and applies the rules
directly (subtask 3 chooses; the explicit list may read more clearly).
The rules, from the RMS manual's FAB$B_FAC and FAB$B_SHR descriptions:
a new accessor is refused (RMS$_FLK) if it wants an access some current
accessor doesn't share, or doesn't share an access some current accessor
has. `FAB$V_UPI` turns RMS's locking off for that accessor (block I/O
users interlock themselves). `$CLOSE` and rundown remove the accessor.

Deleting, renaming, or superseding a file that's open is handled as the
manual says (deletion deferred to the last close, as the ACP path does
already; rename allowed; supersede creates a new version).

### A shared file control block

VMS keeps one file control block (FCB) per open file, shared by all its
accessors. govax gets the same: one in-memory FCB per (volume, file ID)
holding the header, extent map, end of file (EBK/FFB), and highest
block, with a reference count, kept by the volume. Every `volume.File`
handle for that file uses it: an extension by one is seen by all, the end
of file is one value, and the header is written back once, consistently
(on the last close, and when an accessor's change must be durable). This
is an ods2 change (Decision 9): new API in `ods2/volume` (and its `rms`
package's Reader and Writer read the end of file from the FCB), tagged
and pinned as `CLAUDE.md` describes (`GOWORK=off` check).

### Shared sequential files

With write sharing (`FAB$V_SHRPUT`), a `$PUT` appends at the FCB's end of
file and updates it at once (under the file's lock), so several writers'
records are each whole and none is lost; the RMS manual's note that for
unshared files the end of file in a XABFHC is "the values at the time of
the last Close or Flush" implies shared ones are current, which govax
follows. A reader's `$GET` past its last known end of file re-reads the
FCB's. `$FLUSH` writes the header. Where VMS limits which sequential
record formats may be write-shared, govax follows the manual if it says
(unconfirmed otherwise).

### Record locks

For write-shared files RMS locks a record when a `$GET` or `$FIND`
retrieves it, releasing at the next operation, `$FREE`, or `$RELEASE`,
with the `RAB$L_ROP` options `RLK`, `ULK`, `NLK`, `RRL`, `WAT`, `TMO`,
and RMS$_RLK for a record another stream holds. For sequential files
(govax's only organization) the rules in the RMS manual decide how much
of this applies; subtask 6 reads them and implements that much on
`internal/lck`, logging anything unconfirmed.

### Directories and the volume

Each directory operation already completes within one service. Phase 47
checks that no directory or index-file state is cached per process or
per handle (directory caches in `internal/rms` sessions, `$SEARCH`
contexts), and that a directory changed by one process is re-read by
another's `$SEARCH`. Console operations that do several volume steps
(COPY, RENAME of several files) run with scheduling frozen, or are fine
because the console only runs at the prompt (Decision 4); checked.

## Subtasks

1. **Survey.** Read ods2's `volume.File`, `rms.Reader`/`Writer`, and
   `internal/rms`'s handles and caches; list every piece of per-handle or
   per-process state that can go stale or be written back stale; write
   failing tests that show each corruption (two writers, extend then
   close, delete while open). Record the findings here.
2. **`internal/lck`** with table-driven tests (compatibility, FIFO,
   conversion, NOQUEUE, release at rundown) and scheduler integration
   (waiting and granting across processes).
3. **File access arbitration** and RMS$_FLK, for RMS and the ACP. Tests
   for each FAC/SHR combination class, UPI, close and rundown releasing.
   Optional probe: the same combinations on VMS 7.3, recording each
   status.
4. **The shared FCB in ods2** (new API, tests in ods2, tag, pin, `GOWORK=off`
   build), then `internal/rms` using it. The subtask 1 corruption tests
   now pass.
5. **Shared sequential files**: appends by several writers, readers
   seeing new records, `$FLUSH`, XABFHC values. Tests with two processes
   and small quanta.
6. **Record locks** as far as the RMS manual applies them to sequential
   files.
7. **`$ENQ`/`$ENQW`/`$DEQ`** (and blocking ASTs). Tests, including a
   MACRO program protecting a global section with a lock (if Phase 46 is
   done; otherwise a shared file).
8. **Directories and volume metadata** checks from the design; fixes as
   found.
9. **Stress tests**: two and three processes, small quanta, appending to
   one file, creating and deleting files in one directory, extending
   files concurrently; after each, dismount, and run ods2's volume
   analysis (no lost or multiply allocated blocks, consistent headers and
   directories) and check every record.
10. **Close-out.** Status, progress log, PLAN.md, CLAUDE.md (`internal/lck`,
    the FCB), HELP, DEVIATIONS for unconfirmed rules.

## Survey findings (subtask 1)

Every piece of state below is kept across service calls, by a handle or a
process, about something another process can change. `sharing_test.go`
(`internal/rms`) has a test for each defect, two processes being two
`Context`s over one `MountTable`; each is skipped (`pending`) until the
subtask named fixes it. All six failed as described when written.

| # | State | Kept by | What goes wrong | Test | Fix |
| --- | --- | --- | --- | --- | --- |
| 1 | No arbitration at all: `FAB$B_SHR` is never read | — | a writer that shares nothing doesn't stop a second open | `accessConflict` (opens, RMS$_NORMAL) | 3 |
| 2 | The file header and extent map (`volume.File.Header`, `Extents`) | each `volume.File` | `CloseWithFinalByte` writes the handle's copy back: a second writer's close drops the first's extents (cluster allocated, used by no file) and end of file (records lost, or a corrupt record read) | `twoWritersAppend`, `extendThenClose` | 4 |
| 3 | The end of file, as the highest block written (`maxWrittenVBN`) | each `volume.File` | the last close decides the end of file for everyone | `extendThenClose` | 4 |
| 4 | The write position and partial last block (`rms.Writer.vbn`, `buf`) | each `rms.Writer` | always starts at VBN 1: an open for `$PUT` overwrites the file from its start, even in one process (RAB$V_EOF isn't implemented); two writers write over each other, and a partial block reaches the disk only at `$CLOSE` | `appendToExisting`, `twoWritersAppend` | 4, 5 |
| 5 | The end of file a reader stops at, and its buffered block (`rms.Reader`'s `blockStream`) | each `rms.Reader` | fixed when the reader is made: records appended later are never seen (RMS$_EOF) | `readerSeesAppend` | 5 |
| 6 | Whether a file is open | only the ACP path (`mountedVolume.accessed`) | an RMS open isn't counted: deleting the file (IO$_DELETE, the console's DELETE, a version limit's purge on `$CREATE`) frees its header and blocks at once, and the next file created reuses them under the open reader | `deleteWhileOpen` | 3 |

Also found, for later subtasks:

- **`$SEARCH` contexts** (`searchState.Dirs`) hold `*volume.Directory`
  values, each with its own copy of the directory's header and extents,
  from the first `$SEARCH` of a wildcard sequence to the last. A
  directory that grows meanwhile (another process creating files) is
  read through the stale map. Subtask 8.
- **Version limits**: `$CREATE`'s `enforceVersionLimit` (ods2) deletes the
  oldest versions without asking whether they're open (row 6).
- **Host files** (`Session.Locate`'s host side) have no arbitration either;
  subtask 3 decides.
- **Rundown** already closes a process's files (`FileTable.Rundown`, Phase
  45), so it only has to release the accessor (subtask 3) and locks
  (subtasks 2 and 6).
- Allocation is coherent: each device's storage and index bitmaps are one
  cache (`Device.Bitmap`/`IndexBitmap`), and directory and header changes
  are each written through within one service.
- No RMS service but `$PUT`, `$GET`, and `$CLOSE` reads or writes records
  of a disk file: `$FLUSH`, `$FIND`, `$FREE`, `$RELEASE`, `$REWIND`,
  `$UPDATE`, `$TRUNCATE`, and `$ERASE` aren't implemented. Subtasks 5 and 6
  add what sharing needs (`$FLUSH`; `$FIND`, `$FREE`, `$RELEASE` if the
  record-lock rules call for them).

The RMS manual (OpenVMS 7.3, `OVMS_731_RMS.pdf`) settles two of the open
questions:

- Several writers may share a sequential file: in "Inserting Records into
  Sequential Files" (the `$PUT` chapter), RMS moves a sharing writer's
  position to the new end of file another writer made, so no record is
  overwritten. No restriction to a record format is given.
- FAB$B_FAC's description gives the arbitration rule with a worked
  example (processes A, B, C): GET implies read access, PUT, DEL, UPD,
  and TRN write access; a new accessor is refused if it isn't compatible
  with every current accessor, both ways. FAB$B_SHR's gives the defaults
  (SHRGET for GET, NIL for any write access) and says NIL takes
  precedence over the other bits.

## Open questions

- ~~Explicit accessor lists or lock modes for file arbitration~~
  (neither: the file system's access counts, in ods2's shared File;
  subtasks 3-4 below).
- ~~Whether VMS 7.3 RMS allows several writers on a sequential file with
  variable-length records~~ (yes, by the manual; see the survey), and how
  a reader sees an append (a probe, if the manual doesn't say).
- ~~Host files~~: decided in subtasks 3-4: no arbitration. A host file is
  the host's; ods2's access counts are per ODS-2 volume, and a host path
  has no file ID to key them by. Logged as a known gap.

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
- 2026-10-08: Subtask 1 (survey) done: findings above, six skipped
  tests in `internal/rms/sharing_test.go`, each failing as described
  before it was skipped.
- 2026-10-08: Subtask 2: `internal/lck`, from the System Services manual
  ($ENQ, $DEQ) and the Internals book's chapter 13. A `Manager` of
  resources (name, UIC group or 0 for system, access mode, parent
  resource) with granted, conversion, and waiting queues; the six modes
  and the manual's compatibility table; NOQUEUE; conversions (a lock
  never blocks its own: section 13.2.2); sublocks (the parent must be
  the caller's and granted; SUBLOCKS on dequeuing a parent); CANCEL;
  DEQALL by access mode or of a lock's sublocks; value blocks (copied to
  a lock granted or converted up, stored by PW or EX on the way down or
  out; INVVALBLK); blocking notices, once per grant. Operations return
  `Event`s (granted, blocking, aborted, canceled), for any owner;
  `lck.Deliver` hands each to its lock's `Data` if that's a `Notifier`,
  which is how $ENQ's completions and RMS's waits will be told.
  `System.Locks` holds the one database; image rundown dequeues the
  process's user-mode locks, process deletion all of them
  (`corevms/locks.go`). Waiting is left to the callers: a waiter sleeps
  on its event flag (Phase 44's waits) and its `Notify` sets it, which
  subtasks 6 and 7 build and test across processes. Rules chosen where
  the sources don't say, unconfirmed:
  - A new lock waits if anything is queued, compatible or not (FIFO,
    as this doc's design said), and a conversion that isn't down waits
    if the conversion queue isn't empty. The book's description
    (13.2.1, 13.2.2) compares only with the granted modes; the FIFO
    rule keeps a stream of compatible requests from starving a queued
    one.
  - A conversion to a mode no more restrictive than the old (every mode
    compatible with the old is compatible with the new) is granted at
    once, even past queued conversions.
  - At process deletion, a PW or EX lock still held invalidates its
    value block, whether the process ended by `$EXIT` or `$DELPRC`;
    image rundown's dequeue doesn't invalidate.
  - A lock on the conversion queue gets blocking notices for what its
    granted mode blocks, as a granted one does.
- 2026-10-08: Subtasks 3 and 4, done together, since where arbitration
  lives decided both. On VMS the file system (the XQP), not RMS, keeps
  each open file's accessor counts in its FCB, and RMS turns FAC and SHR
  into the access it asks for; govax does the same, the FCB being ods2's
  (ods2 Phase 5, `docs/PHASE-05.md` there):
  - **ods2** (`volume/access.go`): `Volume.Access(fid, AccessMode{Write,
    NoRead, NoWrite})`, `AccessFile` for a file just created, and
    `Access.Deaccess`. While a file is accessed, `Access` and `OpenFID`
    return one shared `*File` (the FCB): header, extents, end of file
    (`SetEndOfFile`, `WriteAttributes`). Arbitration is the XQP's: every
    accessor reads, so an access fails (`ErrAccessConflict`) if someone
    denies reading, if it writes and someone denies writing, if it denies
    reading and anyone has the file, or if it denies writing and someone
    writes. `DeleteFile`/`DeleteHeader` of an accessed file remove the
    entry and mark it for delete; the last `Deaccess` frees it (a version
    limit's purge is covered, going through `DeleteFile`). ods2's `rms`:
    `NewAppender`, `Writer.SetShared` (each `Put` at the end of file as it
    is then, written through), `Writer.Flush`, and Readers that ask the
    file for its end of file each time they reach it.
  - **govax RMS** (`internal/rms/sharing.go`): `accessMode` maps FAC and
    SHR (the manual's defaults: SHRGET for a reader, NIL for a writer;
    NIL first) onto ods2's access; `$OPEN`, `$CREATE` (and CIF), and the
    NAM-block opens use it, and a conflict is RMS$_FLK. `$CLOSE` and
    rundown flush the Writer, apply the XABs, and deaccess. `$CONNECT`
    honors RAB$V_EOF (an appender); a `$PUT` stream connected at the start
    of a file with records gets RMS$_NEF, as the manual's `$PUT` lists
    (until now govax overwrote the file from its first block: survey row
    4). A stream on a file others may read or write is a shared Writer.
  - **The ACP** (`acp.go`, `acpdelete.go`, `acpcreate.go`): IO$_ACCESS
    goes through the same counts (`ACPAccessWith`, FIB$M_WRITE, NOREAD,
    NOWRITE; SS$_ACCONFLICT), so RMS and `$QIO` opens are arbitrated
    together; `mountedVolume.accessed` is gone, and `doomed` is kept only
    for temporary files created with an entry.
  - All six survey tests pass (subtask 5's two included), with new ones:
    a table of FAC/SHR classes, the manual's A/B/C example, rundown
    releasing, and RMS against the ACP.
  - Unconfirmed, for a probe: that an RMS opener asking only for PUT
    still counts as reading (the XQP model: it's refused by an opener
    without SHRGET); that FAB$V_UPI alone shares reading as well as
    writing.
  - **ods2 isn't tagged yet.** govax builds with the local `go.work`;
    `GOWORK=off go build ./...` fails until ods2 is tagged (v0.1.16),
    pushed, and pinned (`GOWORK=off go get github.com/tucats/ods2@v0.1.16`).
