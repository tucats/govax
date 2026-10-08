# Phase 45 — Multiprocessing, part 3: creating and deleting processes

**Status:** done (2026-10-07); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Needs Phases 43 and 44.

The program this phase belongs to is described in
[PHASE-43 - processes](PHASE-43%20-%20processes.md), Part A. Read that first.

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
  have get it — notably `PRC$M_HIBER` (start hibernating; VMS 7.3 has no
  `PRC$M_SUSPEND`, found in subtask 9). Others are logged.
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
14. **Close-out.** Status, progress log, PLAN.md, CLAUDE.md, HELP. *Done
    (2026-10-07).*

    **For future SHOW DEVICE work** (author's note, 2026-10-07): the VMS
    7.1 system's `SHOW DEVICE/FULL` of a terminal and of a mailbox, for
    when govax's generic layout for those is replaced the way NLA0:'s was:

        Terminal TTA0:, device type unknown, is online, record-oriented device, carriage
            control.

            Error count                    0    Operations completed                  0
            Owner process                 ""    Owner UIC                      [SYSTEM]
            Owner process ID        00000000    Dev Prot              S:RWPL,O:RWPL,G,W
            Reference count                0    Default buffer size                  80

        Device MBA11:, device type local memory mailbox, is online, record-oriented
            device, shareable, mailbox device.

            Error count                    0    Operations completed                  6
            Owner process                 ""    Owner UIC                      [SYSTEM]
            Owner process ID        00000000    Dev Prot              S:RWPL,O:RWPL,G,W
            Reference count                1    Default buffer size               65535

    Differences from NLA0:'s: the first word is the class (`Terminal`;
    `Device` for a mailbox), the owner UIC prints as its identifier
    (`[SYSTEM]`) where one exists and `[g,m]` where none does, the
    protection is `S:RWPL,O:RWPL,G,W` (no group or world access shown as
    just the letter), a terminal's DEVCHAR adds "carriage control" after
    "record-oriented device", and the first line wraps at 80 columns
    (`carriage` fits on the first line; "control." does not). A mailbox's
    type is "local memory mailbox" and its buffer size 65535 there.

    **Done** (2026-10-07; was: to do before leaving Phase 45, author's
    note after probe 2): the console's `SHOW DEVICE/FULL NLA0:` should print the
    layout the VMS 7.1 system gives it, not govax's generic one. VMS shows:

        Device NLA0:, device type null device, is online, record-oriented device,
            shareable, mailbox device.

            Error count                    0    Operations completed                 31
            Owner process                 ""    Owner UIC                         [1,1]
            Owner process ID        00000000    Dev Prot    S:RWPL,O:RWPL,G:RWPL,W:RWPL
            Reference count               10    Default buffer size                 512

    That is `DEVCHAR` 0C150001 (REC, SHR, AVL, MBX, ...), the type name
    "null device", `Owner UIC [1,1]`, the protection `S:RWPL,O:RWPL,G:RWPL,
    W:RWPL`, and a default buffer size of 512. The operation and reference
    counts are the system's own. The same layout is wanted for `SHOW
    DEVICE` and `F$GETDVI`-style items where govax prints them
    (`internal/console/device.go`).

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

## Carry forward

Things Phase 45 did not get to, so that they are not lost. Each is either
waiting on a run on the VAX, or optional work for a later phase.

1. **Round 5 probe (not yet run).** `testdata/mp/macros/r5_misc.mar` (48
   calls; `exchange.cmd`, `macros.com`, and `copyout.cmd` are for it, the
   exchange volume `testdata/disks/mp-macros.dsk` is built, and its log
   will be `vax/macros5.log`). It asks for: `$IDTOASC`'s third argument
   (a descriptor; real MACRO takes no keyword `RESNAM` for it, and the 9
   other names tried in round 4 were not it: the probe tries 34 more, and
   its size by position), `$TRNLOG`'s LOGNAM and RSLLEN sizes, and
   `$CRELNT`'s TABNAM size. After the run: audit `vax/macros5.log` (it may
   quote a line of a macro's expansion), read it with `dumpcode.go
   -calls`, correct the `$IDTOASC`, `$TRNLOG`, and `$CRELNT` macros in
   `starlet.mar` (and the comment that lists them as unconfirmed), add
   `r5_` to `TestServiceMacroObjects`' list of probe prefixes (the log
   case for `r5_` is already there), run `go generate ./internal/bootdata`,
   and update this doc. Until then the macros use `RESNAM`, a quadword for
   it, a quadword for TRNLOG's LOGNAM and a word for RSLLEN, and a quadword
   for CRELNT's TABNAM.
2. **Unconfirmed behavior** (each marked where it is described above): a
   child's priority at creation (+2 above its base, as probe 1 showed
   once), a child's working-set size (+4 pages), SYSTEM's AST limit (50),
   the default directory and other state a subprocess inherits, PID reuse
   (the shape of a reused slot's PID), `$SETPRI`'s boost rules for
   real-time priorities across processes. Probes 1 and 2 on VMS 7.1 settled
   what they could; a probe on VMS 7.3 (or the same system) could settle
   these.
3. **The argument-count check** (`argcount.go`) has minimums for the
   services probed and about 35 required-argument minimums from the
   manuals; the rest of the services, and any new one, have none. A probe
   could find them as probe 1 found `SS$_INSFARG` for `$GETDVI`.
4. **Macros not probed in other forms.** The list and `_G` forms of the
   other services (round 4) follow the rule found for the first 23 and
   are checked only where round 3's clean calls cover them. `$FAO`'s list
   form is the one with a variable length (3 plus the P arguments given);
   it matches round 3's calls. `$HIBER` has no list or `_G` macro in real
   MACRO and none here for the list form.
5. **Use the macros.** *Done (2026-10-07), after the close-out:* `child.mar`,
   `crechild.mar`, `probe1/probe1.mar`, `probe1/info.mar`, and
   `probe2/probe2.mar` call the services through the macros (`$CREPRC_S`,
   `$CREMBX_S`, `$GETDVIW_S`, `$GETJPIW_S`, `$QIOW_S`, `$FAO_S`, `$TRNLNM_S`,
   `$SETPRI_S`, and the rest), and their tests print the same output as
   the hand-written pushes did (160 lines compared, but for the clock
   fields of the termination message and the length of a temporary path).
   They have not been assembled by real MACRO; the macros have, in effect,
   for the same calls (`TestServiceMacroObjects`).
6. **`SHOW DEVICE/FULL` for terminals and mailboxes** in VMS 7.1's layouts
   (the note under subtask 14 has both): govax prints its generic layout
   for them. Wanted before Phase 46, which makes mailboxes real.
7. **Services with no macros.** The system services govax does not
   implement have none either (`$ENQ`/`$DEQ`: Phase 47; `$CRMPSC`,
   `$MGBLSC`, global sections: Phase 46). Each new service gets its macro
   by the same method: a `testdata/mp/macros` probe (`gen.go`), a run on
   the VAX, `starlet.mar`, and `TestServiceMacroObjects`.
8. **Phases 46 to 48** need what this phase built: the mailbox
   (`mailbox.go`), event flags, and the null device are the starting
   points; the termination mailbox's message is built in `termmsg.go`.

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
  turned the objects into `defined.txt`, and `gen -values` merged
  all 222 names (README.md there), replacing the expected values. Every
  expected `PRC$` and `PQL$` value was right. VMS 7.3's message has
  `ACC$L_JOBID` at offset 12, which the 5.0 manual calls unused; the
  `MSG$_` values agree with STARLET.OLB's. `TestSymbols_CREPRC_values`
  checks the masks against the bits and the message layout. Still to do,
  for subtask 13's MACRO test: have `mkdefs` add `$PRCDEF`/`$PQLDEF`/
  `$ACCDEF` to govax's STARLET.MLB from `defined.txt`.
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
- 2026-10-07: Subtask 9 (the process-control services across processes).
  `$SCHDWK`, `$CANWAK`, `$FORCEX`, and `$SETPRI` now take any process
  (`processTarget`), and with `$WAKE` all of them need GROUP or WORLD for
  a process with another UIC (`mayAffect`: SS$_NOPRIV; `$WAKE` had no
  check). `$SCHDWK` and `$CANWAK` use the target's own timer queue
  (`Environment.timers`), expired for every process at each scheduling
  call (`pollEvents`). `$FORCEX` queues the user-mode `$EXIT` AST on the
  target (once), and a waiting target that can take it becomes
  computable (`wakeWaiters`). `$SETPRI` changes the target's base
  priority in its PCB fields and in the scheduler, limited by the
  *caller's* ALTPRI and authorized priority; `prvpri` is the target's old
  base. `$SETPRN` still names only the caller (as the manual has it).
  `callerTarget` now serves only `$GETJPI` (subtask 10).
  **Suspension** (`suspend.go`): `$SUSPND [pidadr] [prcnam] [flags]` and
  `$RESUME [pidadr] [prcnam]`, a process able to suspend itself. A
  suspended process is in state SUSP and is never chosen; a computable
  one becomes computable again, unboosted, when resumed; a waiting one
  (HIB, LEF, ...) keeps its wait, is skipped by `wakeWaiters` and
  `retryWaiters` while suspended, and goes back to its wait state when
  resumed, so a wakeup that came meanwhile is acted on only then.
  `$SUSPND` of a suspended process is SS$_SUSPENDED, `$RESUME` of one
  that isn't is SS$_NORMAL (both unconfirmed), and `flags` is accepted
  and ignored. VMS delivers kernel-mode ASTs to a suspended process;
  govax delivers none, but `$DELPRC` (`markForDeletion`) resumes its
  target first, as the book's step 2 says, so its deletion runs.
  `PRC$M_SUSPEND` doesn't exist in VMS 7.3's `$PRCDEF` (the design's
  guess), so `$CREPRC` has no start-suspended flag. Tests:
  `corevms/crossprocess_test.go` (the services on a hibernating child,
  the privilege rules for all seven, suspending computable, waiting, and
  self, a wakeup during suspension, `$DELPRC` of a suspended process) and
  `console/crossprocess_test.go` (a MACRO program suspends and wakes a
  higher-priority child, which stays suspended with its wakeup pending,
  then `$RESUME`s and `$FORCEX`es it: it exits with status 2C).
- 2026-10-07: Subtask 10 (`$GETJPI` across processes and wildcard
  scans). `$GETJPI` reads the items of whatever `processTarget` finds
  (by PID, name, or the scan): every item function already read its
  environment's process, so it is called with the target's. Looking at
  another process needs GROUP or WORLD unless it has the caller's UIC
  (`mayAffect`; SS$_NOPRIV for a named process). `JPI$_STATE` is the
  scheduler's state code for the target (CUR, COM, HIB, LEF, SUSP,
  MWAIT, ...), or CUR/COM without the scheduler. `JPI$_JOBTYPE` and
  `JPI$_MODE` are LOCAL/INTERACTIVE in process 1's job and DETACHED/
  OTHER in a job `$CREPRC` made, subprocesses included (unconfirmed for
  subprocesses). Wildcard scan: `-1` starts it and the longword at
  `pidadr` then holds govax's context, `0xFFFF0000` | the next process
  table index; each call returns the next process (in index order) the
  caller may look at and skips the others, and the call after the last is
  SS$_NOMOREPROC. VMS keeps its own position there, which programs
  don't read. A process deleted between calls is simply not seen.
  `callerTarget` is gone (the tests that used it have their own copy).
  No new items were needed beyond subtasks 2 to 7's. The optional VMS 7.3
  probe isn't done. Tests: `corevms/crossprocess_test.go`
  (`TestGetjpi_otherProcess`: PID, owner, master, state, name by PID and
  by name, a hibernating child, NONEXPR, NOPRIV; `TestGetjpi_wildcard`:
  table order, skipped processes, a deletion).
- 2026-10-07: Subtask 11 (NL:). The null device is `NLA0:`, defined by
  `vax.init` (`define/device nla0/devclass=misc`) like TTA0:, with a
  driver in `corevms/nulldriver.go`. `NL:` (the generic name) is
  translated to `NLA0` by `deviceName`, so `$ASSIGN`, `$GETDVI`, and
  `$ALLOC` take either. Its class is `DC$_MISC` (200: new
  `iodev.DeviceClassMisc`, displayed "miscellaneous", and a `misc`
  keyword in `dev_class` in `console.dcl`); 200 is from memory and
  unconfirmed. Functions: writes (virtual, logical, physical) succeed
  with the full count and the data is discarded (an unreadable buffer is
  SS$_ACCVIO); reads are SS$_ENDOFFILE with a count of 0 and the buffer
  untouched; `IO$_WRITEOF`, `IO$_SETMODE`, and `IO$_SETCHAR` succeed;
  sense-mode returns the class and type; anything else is
  SS$_ILLIOFUNC. Several processes may have it assigned at once (it is
  never allocated). RMS on NL: is Phase 46's. A process created with
  `NL:` as SYS$INPUT, SYS$OUTPUT, or SYS$ERROR already worked, since
  those are just equivalence names; now the name leads to a device.
  Tests: `corevms/nulldriver_test.go` (write, read, access violation,
  sense and set mode, an unknown function, two processes, both names)
  and `console`'s `TestNullDevice_defined`.
- 2026-10-07: Subtask 12 (the console). `STOP [process-name]` and
  `STOP/IDENTIFICATION=pid` (VMS's two forms; the PID is hexadecimal, as
  for SHOW PROCESS, and wins if both are given; `/ID` works as an
  abbreviation) delete the process and its subprocesses at once
  (`Console.StopProcess`, `stop.go`; grammar verb `stop` in
  `console.dcl`). A process that isn't there is NONEXPR, a bad PID
  IVIDENT, and process 1, named or not, NOPRIV (govax has no logging
  out; EXIT ends the session). The deletion is immediate rather than the
  scheduler's: `System.DeleteNow` (new, `delete.go`) deletes the
  subprocesses first (leaves up), resumes a suspended process, and gives
  one whose image hadn't ended the final status SS$_ABORT; files are
  closed and the termination message sent. If the CPU was in the
  process, `ReturnToProcessOne` moves it to process 1, and the memory is
  freed at that switch. `System.DeleteOtherProcesses` does this to every
  process but process 1, and the console calls it, while the old memory
  is still the machine's (a rundown reads the process's tables), at
  the start of INIT, VMINIT, and ZERO, and in `EndSession` (govax's
  exit): so files other processes wrote are on the volumes before
  they're dismounted, which the earlier replacing of the System in
  `newRTL` never did. HELP has a STOP topic and lists it. SHOW SYSTEM
  doesn't show owners: VMS's layout has no column for them. Tests:
  `console/stop_test.go` (both forms and the abbreviation, the refusals,
  an owner stopped with its subprocess, INIT deleting what's left) and
  the grammar-split and verb-count tests.
- 2026-10-07: Subtask 13 (the MACRO test). `testdata/mp/crechild.mar`
  and `child.mar`: the parent reads the child's image from its command
  line (`LIB$GET_FOREIGN`; `RunOptions.CommandLine`), `$CREMBX`es a
  temporary mailbox, finds its unit with `$GETDVIW`, learns its own PID
  and base priority with `$GETJPIW`, and `$CREPRC`s `CHILD_1` one
  priority step above itself, with `TT:` as SYS$OUTPUT and the mailbox as
  its termination mailbox. The child runs at once, prints, and
  hibernates; the parent `$GETJPIW`s it (state 7, SCH$C_HIB; its name;
  its owner and master PIDs are the parent's), `$WAKE`s it, and `$QIOW`s
  the termination message, printing the child's final status, 7. The
  output is five lines (listed in `crechild.mar`). `TestCreChild`
  (`console/crechild_test.go`) builds both with govax's MACRO and LINK,
  runs the parent as process 1's image, and checks them. No govax macros
  are used (govax has none yet for the process services), so the same
  sources assemble on VMS, which makes them a run oracle (README.md
  there); that run has not been made. Also done for this subtask,
  subtask 1's loose end: `mkdefs` reads
  `testdata/mp/defs/defined.txt` as well, so govax's macro
  library now has `$ACCDEF`, `$MSGDEF`, `$PQLDEF`, and `$PRCDEF`.
- 2026-10-07: Probe 1 (a VMS 7.3 run for the subtasks so far),
  `testdata/mp/probe1/` (README.md there). `probe1.mar` creates a
  termination mailbox and a named information mailbox, a first child
  (`child.mar`, output `NL:`) and, once that has hibernated, prints
  `$GETJPI` of it and of the parent (thirty longword items, each with its
  status), wakes it, and dumps the 84-byte termination message as 21
  longwords. A second child (`info.mar`), created with no input, output,
  or error, reports through the mailbox what `SYS$INPUT`, `SYS$OUTPUT`,
  `SYS$ERROR`, and `SYS$COMMAND` translate to. It settles: the unknown
  termination message fields (job ID, the word after the type, counts),
  `JPI$_STATE`/`JOBTYPE`/`MODE` for a subprocess, the nominal quotas, and
  the default I/O of a process created without any (govax gives it
  none). `TestProbe1` runs the programs under govax; govax's report is in
  its `-v` output for the side-by-side. The VMS run is for the author
  (`exchange.cmd`, `probe1.com`, `copyout.cmd`).
- 2026-10-07: Probe 1's VMS runs (simh, **OpenVMS V7.1**; logs in
  `testdata/mp/probe1/vax/`). Runs 1 and 2 ended with INSFARG: VMS
  checks a service's argument count where govax doesn't (`$GETDVIW`
  needs 8; `$CREMBX` needs all 7 through `lognam`), so the probe and
  `crechild.mar` pass them; `step n` markers show where a run stops.
  Run 3 got through the child's `$GETJPI`, the parent's, and the
  termination message; its second-child section printed nothing and the
  program ended silently (run 4 will print the failing status and the
  image lengths). What run 3 settled:
  - **Termination message**: confirmed as govax builds it for the type
    word and its neighbor (3, 0), the final status (7), PID, the job ID
    (0, as the 5.0 manual's "unused" and govax say), the termination and
    login times, the blank-filled account and user names (`SYSTEM`), and
    the owner. VMS also fills the counts govax leaves at 0: the page
    faults (0x5F at +30), +38 (0x80), the buffered and direct I/O counts
    (5 and 3); the volume count is 0 (as govax's) and the CPU time 0
    for a child that ran under 10 ms.
  - **`$GETJPI` of a hibernating subprocess**: STATE 7 (HIB), PRIB 6,
    OWNER and MASTER_PID the parent's, PRCCNT 0, JOBPRCCNT 1, JOBTYPE
    and MODE 3 (LOCAL, INTERACTIVE: govax's choice for a job of the
    console's was right, and a subprocess inherits its master's), TMBU
    the termination mailbox's unit, CREPRC_FLAGS 0, BIOLM and DIOLM
    0x12, ASTLM 0x18 (as govax). Differences, taken: the child's
    AUTHPRI is its base priority (6) rather than the creator's (4), now
    `max(creator's, base)`; the SYSTEM account's pooled quotas, which
    govax's nominal values now follow (BYTLM 27392, FILLM 300, PGFLQUOTA
    40960, TQELM 30, ENQLM 200, from 32768, 100, 50000, 20, 300), and
    process 1's working set (WSDEFAULT 512, WSQUOTA 1024, WSEXTENT
    16400, from 150, 256, 1024; `process_services.asm` and its golden
    file follow). Not taken: the child's current priority was 8, two
    above its base, where govax says 6 (a boost at creation, or after
    its run; unconfirmed); a child's working set values are 4 pages
    above its creator's; the SYSTEM account's ASTLM is 50 (govax 24).
    The parent's CREPRC_FLAGS is 0x400 (PRC$M_INTER) and its ASTCNT 0x30.
  - **No input, output, or error** (runs 4 to 6; the second child failed
    with INSFARG, SS$_INSFARG = `0x114`, on its own `$ASSIGN` with 2 of 4
    arguments, until run 6): a process `$CREPRC` created with none of
    them finds `SYS$INPUT`, `SYS$OUTPUT`, `SYS$ERROR`, and `SYS$COMMAND`
    with no translation in `LNM$FILE_DEV` (the probe can't tell an
    undefined name from an empty one). govax defines none of them for such
    a process, which agrees. Its termination status was 1.
  - **Argument counts**: VMS 7.1 rejects a service called with fewer
    arguments than it takes (SS$_INSFARG), where govax never does:
    `$GETDVIW` needs 8, `$CREMBX` 7, `$ASSIGN` 4. A check in govax's
    dispatcher, with the minimums from the manual, would catch such
    programs. Done afterwards, as below.
  - **The argument-count check** (`corevms/argcount.go`):
    `SystemService` refuses a call with fewer arguments than
    `serviceMinArgs` lists, SS$_INSFARG, before the service runs. Eleven
    services have their full counts, the ones VMS 7.1 refused or took in
    probe 1 ($GETDVI(W) 8, $CREMBX 7, $ASSIGN 4, and, taken as exactly
    enough, $GETJPI(W) 7, $CREPRC 12, $QIO(W) 12, $TRNLNM 5, $WAKE 2); the
    thirty-odd others have only the required arguments of the manual's
    syntax, so govax is never stricter than the manual. The RMS services
    and any service not listed have no minimum. Calls made from Go (the
    unit tests) are not checked. govax's own MACRO test programs
    (`delprc`, `termmsg`, `crossprocess` tests) had short argument lists;
    they now pass every argument. Tests: `corevms/argcount_test.go` and
    `TestEnvironmentSystemServiceInsfarg`.
  Probe 1 is finished (`vax/probe1-run6.log` is the complete run).
- 2026-10-07: Probe 2's VMS run (OpenVMS V7.1, `testdata/mp/probe2/vax/
  probe2.log`). What it settled, and what govax now does:
  - **`$SUSPND`/`$RESUME`**: `$RESUME` of a process that isn't suspended
    and `$SUSPND` of one already suspended are both SS$_NORMAL (govax
    had SS$_SUSPENDED for the second). A hibernating child that is
    suspended still shows `JPI$_STATE` 7 (HIB), not SUSP: govax's
    suspended waiting process keeps its wait state too (it still isn't
    woken until resumed), and only a computable or running one shows
    SUSP (unconfirmed for that case).
  - **`$SETPRI` of a hibernating child**: base priority 9, `JPI$_PRI`
    left at 8: VMS doesn't make the current priority the new base for a
    waiting process. `sched.SetBasePriority` now does the same
    (`SetCurrentPriority`, new, sets the current priority directly). Not
    matched: VMS's child had PRI 8 above its base 6 from the start
    (a boost at creation); govax has 6.
  - **Termination messages**: `$FORCEX` code `10000` is the final status
    (as govax); an image that doesn't exist is RMS$_FNF (`00018292`, as
    govax); **`$DELPRC` of a hibernating child gives final status 0**,
    not SS$_ABORT: govax now reports 0 (the `ssAbort` assignments in
    `markForDeletion` and `DeleteNow` are gone).
  - **`$CREPRC` errors**: a duplicate name (`00000094`), a 16 character
    name (`00000154`), a reserved `stsflg` bit (`0000017C`): govax's.
    A PID that doesn't exist: `000008E8` (NONEXPR), as govax.
  - **The wildcard scan**: the PID longword holds `FFFF0000` | the
    process *index* of the process just returned (`FFFF0001`,
    `FFFF0005`, ...), the scan goes in index order, and the last call is
    SS$_NOMOREPROC (`000009A8`) with the longword unchanged. govax's
    context was the index to look at *next*; it is now the index
    returned, as VMS's.
  - **NLA0:** `DVI$_DEVCLASS` is 160 (DC$_MAILBOX, not govax's 200: my
    memory was wrong), `DEVTYPE` 3, `DEVCHAR` `0C150001`; a write has
    IOSB `00000001` (transfer count **0**); a read is SS$_ENDOFFILE
    (`870`) leaving the buffer; **sense mode is SS$_ILLIOFUNC** (`F4`).
    govax now defines NLA0: that way (`vax.init`: class mailbox, type
    `null`, new in `console.dcl`'s `dev_type`, DEVCHAR 202702849), picks
    the null driver by device type (`driverFor`, since the class is the
    mailbox driver's), counts nothing written, refuses sense mode, and
    `removeStaleMailboxes` leaves it. `SHOW DEVICE/FULL NLA0:` is still
    govax's generic layout: see the to-do under subtask 14.
  - **`SYSGEN SHOW/PQL`** (current values): PQL_D: ASTLM 24, BIOLM 18,
    BYTLM 8192, CPULM 0, DIOLM 18, FILLM 16, PGFLQUOTA 16400, PRCLM 8,
    TQELM 8, WSDEFAULT 294, WSQUOTA 588, WSEXTENT 16400, ENQLM 128,
    JTQUOTA 1024; PQL_M: ASTLM 4, BIOLM 4, BYTLM 1024, CPULM 0, DIOLM 4,
    FILLM 2, PGFLQUOTA 16400, PRCLM 0, TQELM 0, WSDEFAULT 512, WSQUOTA
    1024, WSEXTENT 16400, ENQLM 30, JTQUOTA 0. govax's `quotaRules` now
    use them (they were the stock SYSGEN defaults: WS 100/200/400,
    PGFLQUOTA 8192/512, ENQLM 30/4). Probe 1's child working set was
    each minimum plus 4 pages, which govax doesn't add. Also:
    MAXPROCESSCNT 230, DEFPRI 4, QUANTUM 20.
  - Tests changed to match: the delete and wake tests (final status 0),
    the suspend tests (HIB, SS$_NORMAL), the quota and null-device tests,
    and a `SetCurrentPriority` call in the scheduler tests'
    `setBasePriority` helper.
- 2026-10-07: `SHOW DEVICE/FULL NLA0:` (the to-do under subtask 14)
  prints the VMS 7.1 layout: `Console.showNullDeviceFull`
  (`console/device.go`) writes the two-line sentence (phrases from the
  device type and DEVCHAR's REC, SHR, and MBX bits, wrapped at column 78),
  then the four rows in VMS's two columns (first value right-justified to
  32 columns, second to 39). Owner process is the allocating process's
  name or `""`, the owner UIC `[g,m]` in octal, the protection VMS's
  `S:RWPL,O:RWPL,G:RWPL,W:RWPL` (govax keeps none per device), and the
  counts the device's own (VMS's 31 operations and 10 references are its
  system's). `vax.init` gives NLA0: `ownuic` [1,1] and a buffer size of
  512. The name works with or without a colon, in any case, for every
  SHOW DEVICE. Terminals and mailboxes keep the generic layout; the same
  could be done for them from a VMS run. Test:
  `TestShowDeviceFull_nla0`.
- 2026-10-07: The process-service macros. `internal/bootdata/files/
  starlet.mar` gains `$CREPRC_S`, `$DELPRC_S`, `$WAKE_S`, `$HIBER_S`,
  `$SCHDWK_S`, `$CANWAK_S`, `$FORCEX_S`, `$SUSPND_S`, `$RESUME_S`,
  `$SETPRI_S`, `$SETPRN_S`, `$GETJPI_S`, `$GETJPIW_S`, `$GETDVI_S`,
  `$GETDVIW_S`, `$CREMBX_S`, `$DELMBX_S`, `$SETIMR_S`, `$CANTIM_S`,
  `$WAITFR_S`, `$SETEF_S`, `$CLREF_S`, and `$READEF_S`, from the manual's
  argument lists and two rounds of `testdata/mp/macros` probes on real
  MACRO (round 1 in `vax/round1`; round 2, with the true required
  arguments and a marker after each call, in `vax/`). What the probes
  showed: the `_S` forms push their arguments last first as the QIO
  macros do (an omitted address or value is `PUSHL #0`, a word is
  `MOVZWL`, an address is `$PUSHADR` with the size of what it points to:
  longword for PIDADR, QUOTA, ITMLST, ASTADR, STATE, PRVPRI, quadword for
  descriptors, DAYTIM, REPTIM, IOSB, PRVADR, a word for CREMBX's CHAN);
  `$CREPRC_S` is a 14 argument call (two zeros past STSFLG, with no
  keyword; ITEMLST= and NODE= are syntax errors) with BASPRI defaulting
  to 2; `$GETDVI_S` is 8 (keyword NULLARG) and `$CREMBX_S` 8 (keyword
  FLAGS), `$GETJPI_S` 7; and each macro joins particular adjacent
  arguments into one `CLRQ` when both are omitted or `#0`: ASTPRM and
  ASTADR (`$GETJPI`), NULLARG and ASTPRM (`$GETDVI`), LOGNAM and ACMODE and
  PROMSK and BUFQUO (`$CREMBX`), REQIDT and FLAGS and DAYTIM and EFN
  (`$SETIMR`), UIC and BASPRI (`$CREPRC`), the two of `$CANTIM`, and no
  others. A call without a required argument (SCHDWK's DAYTIM, SETPRI's
  PRI, GETJPI's and GETDVI's ITMLST, CREMBX's CHAN, SETIMR's DAYTIM,
  DELMBX's CHAN, EFN of WAITFR, SETEF, CLREF, READEF with STATE) is an
  error here; real MACRO's leaves a broken instruction stream, which
  govax doesn't copy. `TestServiceMacroObjects` (`internal/asm`, with
  helpers that rebuild the CODE section's bytes and relocations from an
  object and split them at the probes' markers) requires govax's macros to
  make the same code as real MACRO for every `_S` call real MACRO took
  without an error: 1,300-odd calls in the 23 probes. It found one bug in
  the older helpers: `$PUSHADRVAL` and `$PUSHVALADR` called `$PUSHADR
  ADDR,CONTEXT=CONTEXT`, whose formal name is replaced on both sides of
  the "=", so any sized addressing mode (`-(R6)`, `(R6)+`, `L[R7]`, a
  literal) failed there ("Undefined symbol Q"): they now pass CONTEXT by
  position. Not written: the argument-list form (`$NAME`: a `.LONG`
  count and one `.LONG` per argument, in line, which takes its values
  without `#`) and `$NAME_G`; the probes' long forms were given `#`
  values and the `_G` probe gave a keyword, so they showed only that the
  forms exist (real MACRO's `$CREPRC_G ARGLST=X` stops with an operand
  syntax error after emitting a CALLG). A third round could settle them.
  NULLARG and FLAGS were accepted by real MACRO, as the manual's
  8-argument `$GETDVI` and later `$CREMBX` suggest.

- 2026-10-07: Macro probes, round 3 (`testdata/mp/macros`, results in `vax/`
  with `macros3.log`): the 23 services in the argument-list form and the
  `_G` form, 46 other system services, and `$GETDVI`'s NULLARG and
  `$CREMBX`'s FLAGS beside the arguments they pair with. Findings, now in
  `starlet.mar` and checked by `TestServiceMacroObjects` (which now
  includes `lst_*`, and the plain `$NAME` calls of `svc_*`; calls real
  MACRO took with an error are left out, as are `_G` calls spelled
  `ARGLST=X`, whose keyword real MACRO reports and drops):
  - `$NAME` (no suffix) is `.LONG n` and one `.ADDRESS` per argument, in
    line: an omitted argument is `.ADDRESS 0`, values are written without
    `#`, `$CREPRC`'s BASPRI defaults to 2, and an addressing mode
    `.ADDRESS` can't take (`#5`, `(R6)`, `-(R6)`, `ADR[R7]`, `@#ADR`) is an
    error in real MACRO. `$HIBER` has no such form.
  - `$NAME_G LST` is `CALLG LST,G^SYS$NAME`; the operand may be any mode.
    The macro's one formal is named LST, so `ARGLST=ADR1` is not a
    keyword (real MACRO reports it).
  - `$CREPRC` has two more arguments, the keywords ITMLST (13th) and NODE
    (14th); `$GETDVI`'s last is NULLARG, an address that pairs with
    ASTPRM (the pair is one `CLRQ -(SP)` when both are omitted); `$CREMBX`'s
    last is FLAGS, a value. The contexts of ITMLST and NODE (longword,
    quadword) are unconfirmed.
  - `$FAO_S` pushes only the P arguments given, and its count is 3 plus
    how many were given (macros for it are not written yet).
  Not yet written: macros for the other services. Round 3 left their
  keywords and required arguments partly wrong (CRELOG and DELLOG's
  TBLFLG, IDTOASC and TRNLOG's names, ALLOC's fifth argument, ADJSTK's,
  GETMSG's, and CRELNT's required arguments), so a round 4 should settle
  them first.

- 2026-10-07: Macro probes, round 4 (`testdata/mp/macros/r4_*`, results in
  `vax/` with `macros4.log`), and the macros for every other system service
  govax implements: `$ADJSTK` through `$UNWIND`, `$FAO`, `$FAOL`, `$GETMSG`,
  the logical-name services, the memory services, `$GETSYI(W)`,
  `$IDTOASC`/`$ASCTOID`, and `$BRKTHRU(W)` (52 services), each as `NAME_S`,
  `NAME` (an argument list in line) and `NAME_G`, at the end of
  `starlet.mar`. The probes gave each service's keywords by position and by
  name, its required arguments (a call without one is an error MACRO
  reports at the call), what an omitted argument pushes (`#0`, or `0` for an
  address; GETMSG's FLAGS `#15`, BRKTHRU's CARCON `#32`), the size of what
  each address argument points to (`-(R6)` shows it), and which adjacent
  arguments are pushed as one `CLRQ` when both are omitted or zero:
  ALLOC (FLAGS, ACMODE), ASCEFC (NAME, EFN), BRKTHRU (ASTPRM, ASTADR),
  (TIMOUT, REQID), (FLAGS, CARCON), (IOSB, SNDTYP), (MSGBUF, EFN), CNTREG
  and EXPREG (RETADR, PAGCNT), CRELOG and DELLOG (LOGNAM, TBLFLG), DCLAST
  (ACMODE, ASTPRM), GETSYI(W) (ASTPRM, ASTADR), SETEXV (ACMODE, ADDRES),
  SETPRT (PROT, ACMODE), SYNCH (IOSB, EFN), WFLAND and WFLOR (MASK, EFN).
  What was surprising: a keyword a `_S` macro doesn't have is not an error:
  the text becomes the first positional argument still free, and the object
  refers to a global symbol of that name (the manual's names were wrong for
  `$TRNLOG`, whose are `RSLLEN` and `RSLBUF`, for `$CRELOG` and `$DELLOG`
  (`TBLFLG`), and for `$IDTOASC`'s third argument, whose real name is not
  known); `$ADJSTK`'s ADJUST is a signed word (`CVTWL`); several
  arguments the manual shows in brackets are not checked (`$CRELNM` and
  `$TRNLNM`'s ITMLST, all of `$IDTOASC`'s and `$BRKTHRU`'s) while others
  the manual brackets are required (`$CRELNT`'s PARTAB, `$SETPRT`'s PROT);
  `$FAO` pushes only the P arguments given (up to 17), with a count of 3
  plus how many; `$CREPRC`'s ITMLST is a longword and NODE a quadword.
  `TestServiceMacroObjects` now takes the `lst_`, `ext_`, and `r4_` probes
  too, and excludes a call real MACRO reported an error for (by the code
  offset in the log, `macros3.log`/`macros4.log`) or whose object refers
  to a symbol that isn't a system service: 667 further calls of round 4,
  and the earlier rounds' calls of these services, make the same code in
  govax's macros. Unconfirmed: the keyword and size of `$IDTOASC`'s third
  argument, the sizes of `$TRNLOG`'s LOGNAM and RSLLEN and `$CRELNT`'s
  TABNAM (the usual ones for a name and a length are used). A small
  round 5 (`r5_misc`) asks for them.

- 2026-10-07: Close-out (subtask 14). Status, PLAN.md, CLAUDE.md, and the
  console help (STOP, and the macros MACRO's own library now has) updated.
  What the phase leaves, for the record:
  - Done: `$CREPRC` and process startup, rundown and deletion, the
    termination message, jobs and job logical names, the process-control
    services across processes, `$GETJPI`/`$GETDVI` of others and wildcards,
    NL:, STOP, the argument-count check (`SS$_INSFARG`), two probes on VMS
    7.1 (`testdata/mp/probe1`, `probe2`), the MACRO test, and macros for 75
    system services.
  - What is left is listed under "Carry forward", above the progress log.
