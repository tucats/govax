# Phase 45 — Multiprocessing, part 3: creating and deleting processes

**Status:** planned (2026-10-06); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Not started. Needs Phases 43 and 44.

The program this phase belongs to is described in
[PHASE-43.md](PHASE-43.md), Part A. Read that first.

## Goal

Let a VAX program create and delete processes the VMS way:

- **`$CREPRC`** creates a subprocess (or a detached process) that runs a
  named image, with its own SYS$INPUT, SYS$OUTPUT, and SYS$ERROR, name,
  base priority, quotas, privileges, and status flags, and optionally a
  **termination mailbox**.
- A process ends when its image exits (a process created with an image
  and no CLI), by `$DELPRC`, or when its owner is deleted; **rundown**
  releases everything it held, and the termination mailbox gets VMS's
  accounting message.
- The process-control services that already exist for "the caller" work
  on **other processes**: `$WAKE`, `$SCHDWK`, `$CANWAK`, `$FORCEX`,
  `$DELPRC`, `$SETPRI`, `$SETPRN`, `$SUSPND`, `$RESUME`, `$GETJPI` (and
  its wildcard scan).
- **Jobs**: a process and its subprocesses share a job — its quotas
  (`PRCLM`, the subprocess limit) and its **job logical-name table**.

At the end, a MACRO test program creates a child that runs, exits with a
status, and is reported through the termination mailbox; the parent can
look at the child with `$GETJPI` while it lives.

## What earlier phases leave in place

- Phase 43: the process table, PIDs, per-process address spaces, stacks,
  PCBs, and image state; image activation that takes the target process.
- Phase 44: the scheduler, waits, idle, process-aware run loops; a
  process that ends just stops (state deleted).
- Phase 26 (and later): `processTarget` (`getjpi.go`); `$GETJPI` items for
  the caller; `ImageRundown` (`process.go`), which already deassigns
  user-mode channels, deallocates devices, disassociates event-flag
  clusters, cancels timers, flushes ASTs, and forgets exit handlers;
  `$CREMBX` (mailboxes are system state); `$FORCEX` by a queued `$EXIT`
  AST; `$DELPRC` of the caller as an image exit.
- The logical-name database (`internal/lnm`) with process and system
  directories, and no job table (`LNM$TEMPORARY_MAILBOX` →
  `LNM$PROCESS`, bug 4).

## Design

### `$CREPRC`

```text
SYS$CREPRC [pidadr] ,[image] ,[input] ,[output] ,[error] ,[prvadr]
           ,[quota] ,[prcnam] ,[baspri] ,[uic] ,[mbxunt] ,[stsflg]
           ,[itemlst] ,[node]
```

From the *System Services Reference Manual*:

- `image` is a file spec, found by RMS rules (default type `.EXE`); govax
  activates it with the console's activator (through a hook the console
  installs on the System, since `corevms` can't import `console`). The
  special case of a CLI (`SYS$SYSTEM:LOGINOUT.EXE`, which on VMS starts
  DCL) is Phase 48's subprocess CLI; until then it is an error status at
  process startup.
- `input`, `output`, `error` become the new process's SYS$INPUT,
  SYS$OUTPUT, SYS$ERROR logical names (process table, executive mode).
  Their translation decides where its I/O goes: the terminal (shared with
  its creator), NL:, a mailbox, or a file.
- `prcnam` must be unique (SS$_DUPLNAM); `baspri` is limited by the
  creator's privileges as `$SETPRI` limits it; `quota` is a PQL list,
  recorded (and enforced where govax has the resource: `PRCLM`, `ASTLM`,
  `TQELM` maybe); `prvadr` is limited to the creator's authorized
  privileges unless it holds SETPRV.
- `uic` given makes a **detached** process (its own job; no owner);
  otherwise a **subprocess** in the creator's job.
- `stsflg` bits (`PRC$M_*`): recorded; the ones with an effect govax can
  have get it — notably `PRC$M_SUSPEND` (start suspended) and
  `PRC$M_HIBER`-style starts if the manual lists them. Others are logged.
- `mbxunt`: the unit of a mailbox to receive the termination message.
- The returned PID goes to `pidadr`. Errors the manual lists (SS$_ACCVIO,
  SS$_DUPLNAM, SS$_EXQUOTA for PRCLM, SS$_IVQUOTAL, SS$_NOPRIV, SS$_NOSLOT
  when S0 or the process table is full, ...) are returned before anything
  is built.

Like VMS, `$CREPRC` returns as soon as the process exists; finding and
activating the image happens in the new process, so an image that can't be
found ends the new process with that status (seen in its termination
message), not the creator's call.

### Process startup

The new process's first dispatch runs a Go "process startup" step in its
own context (the scheduler calls it instead of executing an instruction,
the first time): create the SYS$ logical names, set the default directory
(the creator's, unless the manual says otherwise; unconfirmed until
checked), activate the image into the process's P0 through its address
space, write its IMAGE$INIT driver (Phase 43), and build the sentinel
frame on its stack, so the image runs as RUN's does, in user mode. Any
failure deletes the process with the failing status.

### Image exit, rundown, deletion

- A process created to run an image is **deleted when its image exits**
  (main returns, `$EXIT`, an unhandled condition, `$FORCEX`). The sentinel
  return and `ErrImageExit` in a non-console process lead here (Phase 44
  left a stub).
- **Process rundown**: image rundown (the existing `ImageRundown`), then
  everything else the process holds — all its channels (any mode), its
  open RMS files (closed, flushed), its temporary mailboxes' channels (so
  a temporary mailbox with no channels left is deleted), its locks (Phase
  47 adds), its global section mappings (Phase 46 adds), its subprocesses
  (deleted first, recursively), then its pages, page tables, stacks, and
  PCB, and its entries in the process table and job.
- **The termination message**: if the process had a termination mailbox,
  the message described in the System Services manual's `$CREPRC` entry
  (`ACC$K_TERMLEN`, 84 bytes: message type `MSG$_DELPROC`, final status,
  PID, job ID, logout time, account and user names, CPU time, page faults,
  ... login time, owner PID) is written to it, as from the deleted process.
  Fields govax doesn't track (page faults, I/O counts) are zero; which
  fields are confirmed is logged.
- **`$DELPRC` of another process** marks it for deletion; the deletion
  runs when that process is next dispatched (like VMS's kernel-mode AST
  to the target), so rundown happens in its own context. Exit handlers
  don't run, as for `$DELPRC` of self today.
- Deleting a process with the console's run in progress in it (process 1)
  stays as today: process 1 is never deleted, only its image ends.

### Services on other processes

`processTarget` finds any process in the table by PID or name (with the
manual's rules for `prcnam` scope: the caller's UIC group), and these act
on the target, not the caller:

- `$WAKE`, `$SCHDWK`, `$CANWAK`: the target's wake flag and timer queue;
  a wake makes a hibernating target computable (with its boost).
- `$FORCEX`: queues the `$EXIT` AST to the target.
- `$SETPRI`: the target's base priority; reschedules.
- `$SUSPND`/`$RESUME`: the SUSP state (VMS's suspend lets kernel ASTs
  through; a suspended process doesn't run its user code until resumed).
- `$SETPRN`: unique names across the table.
- `$GETJPI`/`$GETJPIW`: any process's items, and the wildcard scan
  (`pidadr` = −1: a context value walked through the table, ending with
  SS$_NOMOREPROC). New items as needed: `JPI$_STATE`, `JPI$_OWNER`,
  `JPI$_PRCCNT`, `JPI$_MASTER_PID`, `JPI$_CPUTIM`, `JPI$_PID`,
  `JPI$_PRCNAM`, `JPI$_MODE` (INTERACTIVE/OTHER/...), `JPI$_JOBTYPE`.
  Privilege checks (GROUP, WORLD) pass for the SYSTEM-privileged
  processes govax creates; the checks are coded anyway.

### Jobs and logical names

A job record (VMS's JIB) holds the job's pooled quotas, the subprocess
count, the master PID, and the job logical-name table. `internal/lnm`'s
`Database` becomes a per-process view: its own process directory and
table, the job's table, and the shared system directory with the system
and group tables. `LNM$JOB` and `LNM$FILE_DEV` = PROCESS, JOB, GROUP,
SYSTEM; `LNM$TEMPORARY_MAILBOX` → `LNM$JOB` (bug 4). The console's
`SHOW LOGICAL`/`DEFINE` keep working on process 1's view, and gain
`/JOB` if the grammar lacks it.

### The null device

`NL:` (`NLA0:`), VMS's null device: writes are discarded, reads return
end of file. A device in the device table with a trivial driver, usable
by `$ASSIGN`/`$QIO` and (in Phase 46) RMS. It's the natural SYS$INPUT
for a process created without one.

### The console

- `STOP/IDENTIFICATION=pid` (and `STOP name`) deletes a process (not
  process 1).
- INIT, VMINIT, ZERO, and govax's exit delete every process but process 1
  (flushing their files).
- SHOW SYSTEM (Phase 44) shows owner relationships if VMS's layout does.

## Subtasks

1. **Definitions.** Add the missing symbols to `internal/vmsdef`
   (`PRC$M_*`, `PQL$_*`, `ACC$*` offsets and `ACC$K_TERMLEN`,
   `MSG$_DELPROC`, new `JPI$_*` codes) through `internal/vmsdef/gen` from
   the local definition files, or from the manuals' tables.
2. **Jobs.** The job record, PRCLM accounting, owner/master PIDs;
   process 1's job.
3. **Per-process logical-name views** and the job table (bug 4). Tests:
   a logical defined in a subprocess's process table isn't seen by its
   parent; one in the job table is.
4. **`$CREPRC`'s checks and creation** (no startup yet): argument
   validation and every error status; the process exists, computable,
   with its startup pending. Tests in Go.
5. **Process startup**: SYS$ names, default directory, image activation
   into the new process, the driver and sentinel frame. Tests: a child
   image prints a line on the terminal (shared SYS$OUTPUT) and exits.
6. **Image exit → process deletion; rundown; teardown.** Including the
   pages, tables, stacks, PCB, and S0 pool returned (a test creates and
   deletes 100 processes and checks the pool and frame counts return to
   where they started).
7. **The termination message.** Tests read it from the mailbox and check
   each field; optional probe: the same parent/child on VMS 7.3, dumping
   the message.
8. **`$DELPRC` of others, deletion of subprocesses with their owner.**
9. **`$WAKE`/`$SCHDWK`/`$CANWAK`/`$FORCEX`/`$SETPRI`/`$SETPRN`/
   `$SUSPND`/`$RESUME` across processes.** Tests per service.
10. **`$GETJPI` across processes and wildcard scans**, with the new
    items. Optional probe: `$GETJPI` of a subprocess on VMS 7.3.
11. **NL:**, with `$QIO` tests.
12. **Console**: STOP, process cleanup on INIT/VMINIT/ZERO and exit,
    help.
13. **The MACRO test**: `testdata/mp/crechild.mar` (or similar): create a
    child with a termination mailbox, `$GETJPI` it, wait for the
    termination message, print the child's status; assembled and linked
    by govax in the test.
14. **Close-out.** Status, progress log, PLAN.md, CLAUDE.md, HELP.

## Open questions

- The default directory and other inherited state of a `$CREPRC`
  subprocess (privileges? process logical names? none, per the manual,
  but check): settled from the manual in subtask 5.
- Whether `$CREPRC` with no `image` is meaningful (VMS creates a process
  that does nothing useful); probably SS$_IVLOGNAM or an immediate exit.
- PID reuse: the sequence number makes a reused slot's PID different;
  confirm the shape against a probe if Decision 7 allows.

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
