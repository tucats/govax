# Phase 45 — Multiprocessing, part 3: creating and deleting processes

**Status:** in progress (2026-10-07); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Needs Phases 43 and 44.

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
  but check): settled from the manual in subtask 5. *Answered (subtask
  5):* the manual doesn't say. govax copies the creator's default
  directory and its process-table SYS$DISK at creation, as VMS's process
  quota block carries them (unconfirmed), and nothing else: no other
  process logical names. Privileges are subtask 4's.
- Whether `$CREPRC` with no `image` is meaningful (VMS creates a process
  that does nothing useful); probably SS$_IVLOGNAM or an immediate exit.
  *Answered (subtask 5), unconfirmed:* `$CREPRC` succeeds, and the new
  process stops at startup with RMS$_FNF, as for any image not found.
- PID reuse: the sequence number makes a reused slot's PID different;
  confirm the shape against a probe if Decision 7 allows.

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
- 2026-10-07: Subtask 1 (definitions). Of the names this phase needs,
  `internal/vmsdef` already had the `JPI$_` items (`JPI$_STATE`,
  `JPI$_OWNER`, `JPI$_PRCCNT`, `JPI$_MASTER_PID`, ...) and every `SS$_`
  code `$CREPRC` returns (`SS$_DUPLNAM`, `SS$_NOSLOT`, `SS$_IVQUOTAL`,
  `SS$_EXQUOTA`, `SS$_NOMOREPROC`). Missing were `$PRCDEF`, `$PQLDEF`,
  `$ACCDEF`, and `MSG$_DELPROC` (only in `LibrarySymbols`). The local
  definition files are behind the clean-room hook, so the values come from
  the manual and a probe instead: the VMS 5.0 System Services Reference
  Manual's `$CREPRC` entry gives the termination message's offsets
  (`ACC$`, 84 bytes), STARLET.OLB gives `MSG$_DELPROC` (3), and the
  manual names the `PRC$M_` flags and `PQL$_` codes without values.
  The first commit entered expected values, marked unconfirmed, and added
  a probe, `testdata/mp/defs`: `$PRCDEF`, `$PQLDEF`, `$ACCDEF`, and
  `$MSGDEF`, each called with `GLOBAL`, so the objects' GSDs list every
  name and value. The author ran it on VMS 7.3 the same day; `decode.go`
  turned the objects into `phase45-defined.txt`, and `gen -values` merged
  all 222 names (README.md there), replacing the expected values. Every
  expected `PRC$` and `PQL$` value was right. VMS 7.3's message has
  `ACC$L_JOBID` at offset 12, which the 5.0 manual calls unused; the
  `MSG$_` values agree with STARLET.OLB's. `TestSymbols_CREPRC_values`
  checks the masks against the bits and the message layout. Still to do,
  for subtask 13's MACRO test: have `mkdefs` add `$PRCDEF`/`$PQLDEF`/
  `$ACCDEF` to govax's STARLET.MLB from `phase45-defined.txt`.
- 2026-10-07: Subtask 2 (jobs), from *VAX/VMS Internals and Data
  Structures*, section 20.1.1 (steps 3, 8, 17, 19), figure 20-2, and
  table 20-3. `corevms/job.go`: a `Job` (VMS's JIB) with the master PID
  (`JIB$L_MPID`), the subprocess limit and count (`JIB$W_PRCLIM`/
  `PRCCNT`), and the other pooled quotas' limits (BYTLM, FILLM,
  PGFLQUOTA, TQELM, ENQLM, JTQUOTA), recorded but not enforced. Each
  `Process` gains `Owner` (`PCB$L_OWNER`, 0 when detached),
  `SubprocessCount` (`PCB$W_PRCCNT`, the subprocesses it created itself),
  and `Job`. `NewEnvironment` makes a detached process, the master of a
  new job: process 1, and the extra processes Phase 43/44's tests make.
  `NewSubprocess(owner, in, out)` makes a process in its owner's job,
  with the owner's user name, account, and UIC, counted against PRCLM
  first (SS$_EXQUOTA when the job is full, the 5.0 manual's status; VMS
  also has SS$_EXPRCLM, which a later VMS may return instead:
  unconfirmed), and given back if the process table is full (SS$_NOSLOT).
  `RemoveProcess` takes a subprocess out of its owner's and job's counts
  (`leaveJob`). `$GETJPI` reads the job: `JPI$_MASTER_PID` (was the
  caller's PID), `JPI$_OWNER` (was 0), and new `JPI$_PRCCNT`,
  `JPI$_JOBPRCCNT`, `JPI$_PRCLM`, `JPI$_BYTLM`, `JPI$_FILLM`,
  `JPI$_PGFLQUOTA`, `JPI$_TQLM`, `JPI$_ENQLM`. The quota values are
  nominal (PRCLM 10, ...), standing in for the SYSTEM account's UAF
  entry. Tests: `job_test.go` (process 1's job, figure 20-2's tree, PRCLM,
  a full table).
- 2026-10-07: Subtask 3 (per-process logical-name views and the job
  table), from the User's Manual's tables 11-1, 11-2, and 11-4.
  `lnm.Database` is now one process's view: its process directory and
  private tables are its own, and the system directory with every
  shareable table (system, group, and job tables) is held once, in a
  `sharedTables` every view points to. `NewDatabase(uic)` makes the first
  view and the shared tables; `NewJobTable` adds a job's table to the
  system directory (permanent, kernel mode); `NewProcessView(uic,
  jobTable)` makes another process's view, creating its group's table if
  it's the group's first process. Each process directory has
  `LNM$PROCESS`, `LNM$JOB`, and `LNM$GROUP`; `LNM$FILE_DEV` is PROCESS,
  JOB, GROUP, SYSTEM; and `LNM$TEMPORARY_MAILBOX` is `LNM$JOB`, fixing
  PHASE-43.md's bug 4. Default `LNM$xxxx` table names are counted
  system-wide. Job table names stand for VMS's JIB address
  (`LNM$JOB_xxxxxxxx`); govax's are `LNM$JOB_80000100`, `..._80000200`,
  ..., a choice. `NewSubprocess` gives the subprocess a new view in its
  owner's job; the console's database is still process 1's view (it
  outlives INIT, as before). Visible changes: SHOW LOGICAL lists the job
  table between the process and group tables, and SHOW
  LOGICAL/STRUCTURE shows it under the system directory, as VMS's do.
  The console's DEFINE, ASSIGN, DEASSIGN, and SHOW LOGICAL gain `/JOB`
  (grammar, handlers, HELP). MOUNT's `DISK$` names stay in the system
  table: VMS puts a private mount's in the job table, but govax's mounts
  are system-wide. Tests: `lnm`'s `TestProcessView_sharing` (a name in a
  subprocess's process table isn't seen by its parent, one in the job
  table is) and `TestProcessView_jobsAndGroups`; `corevms`'s
  `TestJob_logicalNames` (the same through `$CRELNM`/`$TRNLNM`, and a
  subprocess's `$CREMBX` finding its owner's mailbox by name); the
  console's `TestLogical_jobTable`.
- 2026-10-07: Subtask 4 (`$CREPRC`'s checks and creation), from the VMS
  5.0 System Services Reference Manual's `$CREPRC` entry.
  `corevms/creprc.go`: `serviceSysCreprc` reads the arguments and
  `Environment.CreateProcess(CreateRequest)` does the rest, so Go code
  (and tests) can create a process without building an argument list.
  Statuses, each before anything is built: SS$_UNSUPPORTED with
  `vax.process.scheduler` off; SS$_ACCVIO for an unreadable string,
  descriptor, privilege mask, or quota list, or an unwritable pidadr
  (probed by reading the longword and writing it back); SS$_IVLOGNAM
  for a process name of 0 or more than 15 characters, or an image,
  input, output, or error string over 255 (the manual's argument text
  says 63 for image; its status list says 255 for all four, which govax
  takes); SS$_IVQUOTAL for a quota code `$PQLDEF` lacks; SS$_IVSTSFLG for
  a flag above `PRC$V_TCB` (bit 17, VMS 7.3's highest; the 5.0 manual
  stops at bit 10); SS$_NOPRIV (DETACH or CMKRNL for a detached process
  with another UIC, `PRC$M_BATCH`, or `PRC$M_NETWRK`; NETMBX for a
  network process; PSWAPM and NOACNT for their flags); SS$_DUPLNAM in
  the new process's UIC group; SS$_EXQUOTA for PRCLM and for CPU time
  the creator can't spare; SS$_NOSLOT for a full process table, or no
  room in the S0 pool (or no pool: no VMINIT), as this document's design
  says (the manual's SS$_INSFMEM is for dynamic memory, which Go
  provides). A failure after the process took its slot removes it
  (`RemoveProcess`), giving back the job's count and every pool page.
  A `uic` or `PRC$M_DETACH` makes a detached process: a new job with its
  own logical-name table, no owner, the creator's user name and account.
  `itemlst` and `node` (later VMS) are accepted and ignored.
  The new process gets page tables of its creator's size (VMS sizes
  every process's by one SYSGEN parameter; govax's stand-in is process
  1's, from `vax.init`: 128 + 64 pool pages), VMINIT's default stacks (4
  kernel, 8 executive, 8 supervisor pages, with guard pages: 22 pages),
  and a PCB page, whose initial state is user mode on its stacks with PC
  0 until startup sets it. It's computable in the scheduler at its base
  priority (baspri's low five bits, no higher than the creator's without
  ALTPRI; an omitted baspri is 0). Privileges: those asked for, or the
  creator's current ones if prvadr is omitted (unconfirmed), ANDed with
  the creator's without SETPRV; authorized for those and the creator's
  authorized ones (unconfirmed). `PRC$M_SSRWAIT` turns resource wait
  mode off; the other flags and mbxunt are recorded (`Process.CreateFlags`,
  `TerminationMailbox`) and reported by `$GETJPI` (`JPI$_CREPRC_FLAGS`,
  `JPI$_TMBU`); `PRC$M_HIBER` is subtask 5's. `Environment.Startup` holds
  the image and the SYS$ equivalence strings; until subtask 5, a switch to
  a process whose startup is pending stops the run with an error saying
  so. Quotas (`quotas.go`, table-driven by PQL$_ code): the manual's three
  steps (default; the list's last value; raised to the minimum, then
  lowered to the creator's unless a detached process is created with
  DETACH), nondeductible quotas in the process, pooled ones in the job
  (set only for a detached process's new job), and CPULM's own rules
  (half the creator's when not given; taken out of a limited creator's,
  `Process.cpuDeducted`, for subtask 6 to give back). The manual's
  status list also calls a subprocess quota above its creator's
  SS$_EXQUOTA, against step 3's lowering; govax lowers. JTQUOTA, which
  the manual calls deductible, is treated as the job's. The SYSGEN
  defaults and minimums (PQL_D*, PQL_M*) are nominal, unconfirmed; a VMS
  7.3 `SYSGEN SHOW/PQL` would settle them. New `Process` fields
  `BufferedIOLimit`, `DirectIOLimit`, and `CPULimit` with `$GETJPI`'s
  `JPI$_BIOLM`, `JPI$_DIOLM`, and `JPI$_CPULIM`. The manual also settles
  subtask 2's open point: SS$_EXPRCLM is the UAF's MAXDETACH limit on
  detached processes, not PRCLM. Tests: `corevms/creprc_test.go` (21
  refusals, each leaving the table, scheduler, counts, CPU limit, and
  pidadr unchanged; the quota steps; CPULM's rules) and
  `console/creprc_test.go` (a MACRO program's `$CREPRC` of a subprocess
  after `vax.init`, a detached process and per-group names, a creator
  without SETPRV and ALTPRI, PRCLM and a full pool leaving nothing
  behind, the pending-startup error).
- 2026-10-07: Subtask 5 (process startup). `corevms/startup.go`: when
  the scheduler first switches to a created process (`switchTo`, once the
  CPU holds its context), `startProcess` runs instead of its first
  instruction: it defines SYS$INPUT, SYS$OUTPUT, and SYS$ERROR in the
  process's own process table, executive mode, with no attributes (each
  only if `$CREPRC` gave it), has the console activate the image
  (`System.ActivateImage`, installed by `newRTL` as
  `Console.activateCreatedImage`: `imagesOf(env).activateImage` into the
  process's P0, then its IMAGE$INIT driver, running LIB$INITIALIZE as
  RUN does by default), and calls the driver with `Engine.CallEntry`, in
  user mode on the process's user stack, on a sentinel frame, so the
  image's return or `$EXIT` stops the process as Phase 44 stops any
  process but process 1 (deletion is subtask 6). `PRC$M_HIBER` puts a
  `CALLS #0, SYS$HIBER` at the head of the driver, so the process
  hibernates (state HIB) until woken, before LIB$INITIALIZE and the
  image. The default directory and SYS$DISK are copied from the creator
  when `$CREPRC` creates the process (`inheritDefaults`,
  `rms.Session.ForProcess`), not at startup: VMS hands them over in the
  process quota block it builds then (unconfirmed). The image is found
  as RUN finds one, by the console's default directory (the same as the
  child's while process 1 is the creator), never as `/HOST`. A startup
  that fails stops the process with a status: RMS$_FNF for a missing
  image (and for an empty image name, answering an open question),
  SS$_UNSUPPORTED for `LOGINOUT` (Phase 48's command interpreter), a
  system or RMS status found in the error, or SS$_ABORT for govax's own
  activation errors (all unconfirmed against VMS's termination
  statuses). `Schedule` then chooses again before anything runs.
  Tests (`console/creprc_test.go`): a child image, assembled and linked
  by govax's MACRO and LINK, preempts process 1, writes its line on the
  shared terminal, and stops with its status (3), after which process 1
  runs on; its SYS$OUTPUT, missing SYS$INPUT, and copied default
  directory; `PRC$M_HIBER` (hibernates, writes nothing until process 1's
  `$WAKE`, then runs); and the failures (missing image, empty name,
  LOGINOUT), each leaving process 1 running.
- 2026-10-07: Subtask 6 (image exit → process deletion; rundown;
  teardown), from *VAX/VMS Internals and Data Structures*, chapter 22
  (section 22.2.1's steps). `corevms/delete.go`: `DeleteProcess`
  replaces Phase 44's `StopProcess`, for a created process whose image
  ends (the console's `StepMachine`) or can't start (`startProcess`). In
  the book's order: image rundown (`ImageRundown`), then process rundown
  (`processRundown`): RMS's rundown closes every open file
  (`rms.FileTable.Rundown`, new; no XABs apply, as no FAB is at hand), so
  a record written and never closed is on the volume; host files opened
  through the C library's descriptors are closed; every channel in every
  mode is deassigned, so a temporary mailbox whose last channel it was
  goes, with its name; every device it allocated is deallocated; every
  AST, exit handler, and I/O request in any mode is forgotten. Then the
  unused part of the CPU time a subprocess took from its owner goes back
  to the owner (step 10, the only quota returned; a detached process
  returns nothing, even when subtask 4 took its limit from a creator
  without DETACH). A detached process's job ends with it: its job
  logical-name table, which `$CREPRC` made (`Job.LogicalTable`), is
  deleted (`lnm.Database.DeleteJobTable`, new; step 19); process 1's job
  table, and those of test processes sharing its names, are never
  deleted. The process leaves the scheduler, the table, and its job's
  and owner's counts, and `Environment.Stopped` is now `Deleted`. Its
  memory (page tables, P0 and P1 pages, stacks, PCB, image driver: every
  pool page charged to its PID) is freed at once if the CPU isn't in it;
  if it is (a process deleting itself), `switchTo` frees it once the CPU
  has loaded the next process, as VMS frees the deleted process's last
  pages after its final SVPCTX (steps 14 to 16). Saving its context into
  the PCB about to be freed is harmless and leaves the CPU on the
  interrupt stack, where loading the next one starts. `RemoveProcess`
  is now only for a process that never ran (`$CREPRC`'s clean-up). The
  System's new `ProcessDeleted` hook lets the console drop the process's
  image state (`otherImages`). SHOW SYSTEM no longer needs to skip
  stopped processes. Not done here: the termination message (subtask
  7), subprocesses deleted with their owner and `$DELPRC` of others
  (subtask 8); the book's user rundown routines, global sections (Phase
  46), and private volumes (govax's mounts are system-wide) don't apply.
  Noticed: process 1's image rundown didn't close the image's RMS files
  as VMS's does at image exit (fixed next). Tests:
  `corevms/delete_test.go` (a subprocess's kernel-mode channel and
  temporary mailbox, its kernel AST and executive exit handler, the
  counts, the CPU time returned or not; a detached job's table deleted
  and process 1's kept), `lnm`'s `TestDeleteJobTable`, and
  `console/deleteprc_test.go`: a child's image return deletes it, its
  page tables kept while the CPU is in it and freed at the switch to
  process 1, with none of its pool pages left; 100 children created,
  run, and deleted one after another leave the S0 pool's pages in use
  and the physical pages mapped where they were; and a child that
  `$CREATE`s, `$CONNECT`s, and `$PUT`s without `$CLOSE` leaves its record
  in the file (the test fails without RMS's rundown).
- 2026-10-07: Subtask 6 follow-up, at the author's request: an image's
  files are closed when the image ends, in process 1 too, as RMS's
  rundown closes them on VMS. `Environment.CloseFiles` (`delete.go`)
  closes the RMS files (`rms.FileTable.Rundown`) and the C library's host
  files, and `ImageRundown` now calls it first, so process deletion gets
  it through image rundown. VMS keeps a process-permanent file (opened in
  executive mode, by DCL) open across images; govax doesn't record the
  mode a file was opened in, and closes them all. An image RUN started
  that was stopped (HALT, CTRL/C, CTRL/Y, a limit) and never resumed is
  run down when the next RUN starts (`runDownAbandonedImage`), as VMS
  runs it down when the next image starts. INIT, VMINIT, and ZERO close
  the old process 1's files before building the new System (`newRTL`),
  and govax's exit calls the new `Console.EndSession` (run down an
  abandoned image, close process 1's files) before dismounting the
  volumes. (A second CTRL/Y's forced exit skips both, as it skips the
  dismount.) Tests: `console/imagefiles_test.go`: a record written and
  never closed is on the volume after the image exits, and, for an image
  the instruction limit stopped, after the next `RUN/NOEXECUTE`, after
  ZERO, and after `EndSession`; each test fails without its fix.
- 2026-10-07: Subtask 7 (the termination message), from the VMS 5.0
  System Services Reference Manual's `$CREPRC` entry (mbxunt) and *VAX/VMS
  Internals and Data Structures*, section 22.2.1, step 11, and table
  22-1. `corevms/termmsg.go`: `DeleteProcess` sends it after the
  process's rundown and the CPU time returned to its owner, before it
  leaves the scheduler (the book's order; the manual says "before process
  rundown is initiated", which govax reads as the final deletion). The
  84-byte message is the manual's: `MSG$_DELPROC`, the final status (the
  image's `$EXIT` status, or a failed startup's, so a creator learns of
  RMS$_FNF this way), the PID, the deletion time, the account and user
  names blank filled, the CPU time in 10 ms units, the login time, and
  the owner's PID. The job ID (offset 12, which VMS 7.3 names
  `ACC$L_JOBID`; the manual and the book call it unused), the word after
  the message type, page faults, the two peaks, the I/O counts, and the
  volume count are 0 (govax has no paging, counts no I/O, and mounts for
  the whole system); all unconfirmed against a VMS run. It goes to the
  mailbox as an IO$M_NOW write would, as from the deleted process (the
  reader's IOSB has its PID); a mailbox that doesn't exist, is too small
  (maxmsg under 84), or is full loses the message, as the manual says,
  and so does the process's own temporary mailbox, gone with its last
  channel in the rundown. Not modeled: the manual's "after the process
  name has been set to null" (the process has left the table before any
  other process runs). `Process.LoginTime` is new, set when the process
  table takes a process in, with `$GETJPI`'s `JPI$_LOGINTIM`.
  Needed for this, and pulled forward from Phase 46's subtask 1: an I/O
  request records the process that made it (`ioRequest.owner`), and
  `completeIO` completes it for that process, whichever is current: the
  IOSB through the owner's address space (`Environment.storeOwn`), its
  event flag, AST, and wait. The mailbox driver stores a read's data
  through the reader's address space, and a mailbox's attention ASTs go
  to the process that enabled them. Without that, a creator's read that
  is waiting when its child is deleted would have been completed in the
  child's memory. Tests: `corevms/termmsg_test.go` (every field, the
  read's IOSB, a waiting read completed with the creator's flag and AST
  rather than the child's, the four ways a message is lost) and
  `console/termmsg_test.go` (a MACRO parent `$CREMBX`es, `$GETDVIW`s the
  unit, `$CREPRC`s a child, and `$QIOW`s the message: with the read
  waiting in process 1 while the child is deleted, and with the message
  waiting in the mailbox; and a child whose image doesn't exist,
  reported as RMS$_FNF). The optional VMS 7.3 probe isn't done.
- 2026-10-07: Subtask 8 (`$DELPRC` of others, subprocesses deleted with
  their owner), from *VAX/VMS Internals and Data Structures*, sections
  22.1.1, 22.2.1 (step 4), 22.2.2, and 22.2.3, and the VMS 5.0 System
  Services Reference Manual's `$DELPRC` privilege rules. `$DELPRC` now
  reaches any process (`processTarget`), and deleting one other than the
  caller needs GROUP (same group) or WORLD unless it has the caller's UIC
  (`mayAffect`, new, for subtask 9's services too; SS$_NOPRIV). It only
  marks the target (`markForDeletion`, `Environment.deletePending`):
  marking one already marked succeeds and does nothing more; a waiting
  target becomes computable with a boost of 3 (the book's "potential
  boost of 3"); and the service returns at once. The deletion runs in the
  target's own context, as VMS's special kernel-mode AST does: `switchTo`
  deletes a marked process as soon as the CPU holds it, before anything of
  its own runs, and `Schedule` chooses again. Without the scheduler
  installed nothing would dispatch the target, so it's deleted at once.
  A process deleted that way ends with SS$_ABORT, unless its image had
  already called `$EXIT` with a status of its own (VMS's final status for
  an unfinished image is unconfirmed). `DeleteProcess` of a process
  that owns subprocesses (found, as the book says, by scanning the table
  for its PID as their owner) marks each of them and waits (MWAIT,
  RWAST; the waiter takes no ASTs), marked itself; each subprocess's
  deletion leaves its owner's count, and when none is left the owner's
  wait ends and its deletion runs when it next gets the CPU. A tree is
  deleted from its leaves up, an owner image's exit included (the
  console's `StepMachine` deletes it; it waits instead). govax runs the
  owner's whole rundown after its subprocesses have gone, where VMS runs
  its RMS rundown first (steps 2 and 3); no program can tell. Process 1
  is never deleted: another process's `$DELPRC` of it forgets its exit
  handlers and queues a `$FORCEX`-style user-mode AST to `$EXIT` with
  SS$_NORMAL, so the console's run ends, and its subprocesses live on, as
  after any of its images (govax's choice; on VMS the console's process
  would log out). Suspension isn't modeled yet: subtask 9's `$SUSPND` must
  resume a target `$DELPRC` marks, as the book's step 2 says. Tests:
  `corevms/delprc_test.go` (marking a hibernating process, the kept
  `$EXIT` status, the privilege rules, process 1, deleting at once without
  the scheduler, a tree with an owner waiting, ASTs or not, until its
  last subprocess has gone) and `console/delprc_test.go`: process 1's
  MACRO program `$CREPRC`s a hibernating child and `$DELPRC`s it; one
  whose child has a hibernating grandchild, deleted first; and a child
  whose image returns while its own subprocess hibernates, deleted after
  it. Each deleted process's pool pages and page tables are freed.
