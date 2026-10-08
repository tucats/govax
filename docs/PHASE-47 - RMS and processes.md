# Phase 47 — Multiprocessing, part 5: files shared between processes

**Status:** planned (2026-10-06); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Not started. Needs Phase 45 (independent of Phase
46).

The program this phase belongs to is described in
[PHASE-43.md](PHASE-43%20-%20processes.md), Part A. Read that first.

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

## Open questions

- Explicit accessor lists or lock modes for file arbitration (subtask 3).
- Whether VMS 7.3 RMS allows several writers on a sequential file with
  variable-length records, and how a reader sees an append (the RMS
  manual and, if allowed, a probe).
- Host files (`Session.Locate`'s host side): two processes writing one
  host file get no arbitration today. Probably: the same accessor list
  keyed by host path; decided in subtask 3.

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
