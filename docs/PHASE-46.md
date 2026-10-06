# Phase 46 — Multiprocessing, part 4: interprocess communication

**Status:** planned (2026-10-06); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Not started. Needs Phase 45.

The program this phase belongs to is described in
[PHASE-43.md](PHASE-43.md), Part A. Read that first.

## Goal

Make the ways VMS processes talk to each other work *between* processes:

- **Mailboxes**: one process writes, another reads, with waits, IO$M_NOW,
  attention ASTs, resource waits, IOSBs, event flags, and ASTs delivered
  to the right process.
- **Common event flags**: a `$SETEF` in one process ends another's
  `$WAITFR` on the same cluster.
- **Shared memory**: global sections (`$CRMPSC`, `$MGBLSC`, `$DGBLSC`),
  the same physical pages mapped into several processes' P0.
- **RMS on mailbox and null devices**, so a process's SYS$OUTPUT can be a
  mailbox its parent reads (what `LIB$SPAWN /OUTPUT` and `$CREPRC`'s
  `output` need).
- **The shared terminal**: a process waiting for terminal input no longer
  stops every other process.

## What earlier phases leave in place

- Phase 26's mailboxes: `mailbox.go`, `mbxdriver.go`, `qio.go` (pending
  requests, `$QIOW` waits, `$CANCEL`), resource wait mode (`$SETRWM`),
  attention ASTs. All written for one process: the requester is always
  the current process, so the driver writes buffers and IOSBs through the
  CPU's own mapping.
- Common event flag clusters are system state already
  (`EventFlagClusters`, `eventflags.go`), with `$ASCEFC`/`$DACEFC`/
  `$DLCEFC`.
- `$CRETVA`/`$DELTVA`/`$EXPREG` and PTE ownership (Phase 26 subtask 35);
  `vm.Memory`'s frame allocator.
- Phase 43's `vm.AddressSpace` access; Phase 44's wait states and boosts;
  Phase 45's rundown hooks.
- RMS (`internal/rms`): disk files (ODS-2) and the terminal; no mailbox
  or null-device records.
- Terminal input: `consoleIn` reads (`input.go`, `rms/terminal.go`,
  `LIB$GET_INPUT`), blocking the engine's goroutine (bug 6).

## Design

### I/O requests belong to a process

Each pending `$QIO` request records its owner process. When a request
completes — whichever process is current — the completion writes the
data and IOSB through the **owner's address space**, sets the **owner's**
event flag, queues the AST to the **owner**, and gives the owner the I/O
completion boost (Phase 44). That's the one rule that turns Phase 26's
mailboxes into interprocess mailboxes. The same rule covers every driver
with requests that can wait.

The writer's and reader's PIDs already go into the IOSB (`mbxdriver.go`);
now they differ.

### Mailbox protection and lifetime

`$CREMBX`'s protection mask (recorded, not enforced, today) is checked
against the accessing process's UIC on `$ASSIGN` (SS$_NOPRIV), by the
System Services manual's rules. A temporary mailbox lives while any
process has a channel to it; its logical name goes in the job table
(Phase 45 fixed `LNM$TEMPORARY_MAILBOX`), so a subprocess can `$ASSIGN`
by the name its parent gave.

### Common event flags across processes

A wait for a flag in a common cluster puts the process in CEF state; any
process's `$SETEF` on the cluster makes the waiters computable (Phase
44's predicates). Temporary clusters are deleted when the last associated
process disassociates or is deleted.

### Global sections

From the System Services manual: `$CRMPSC` with `SEC$M_GBL` creates a
named global section; govax supports page-file sections
(`SEC$M_PAGFIL`, demand-zero), read/write (`SEC$M_WRT`), group or system
names (`SEC$M_SYSGBL`), mapped at the caller's `inadr` range or, with
`SEC$M_EXPREG`, at the end of P0; `retadr` gets the range. `$MGBLSC`
maps an existing section by name (and version ident) into another
process; `$DGBLSC` deletes it (when the last mapping goes). Pages are
allocated when the section is created (or on first touch, by either
process), and each mapping's PTEs point at the same frames, with
reference counts so `$DELTVA`, image rundown, and process deletion unmap
without freeing frames still mapped elsewhere. File-backed sections
(`$CRMPSC` of a file's blocks) are out of scope; they get a clear
unsupported status.

The interlocked instructions (`BBSSI`, `BBCCI`, `ADAWI`, `INSQHI`,
`REMQHI`, ...) are already atomic: an instruction is never split by a
switch. A test uses them as a spinlock in a global section.

### RMS on mailboxes and NL:

`$CREATE`/`$OPEN` of a name that translates to a mailbox or `NL:` gives a
record stream on the device: `$PUT` writes one message (waiting for room
unless resource wait is off), `$GET` reads one (end of file on an
end-of-file message, on `NL:` always), `$CLOSE` deassigns. That's what
lets a child's SYS$OUTPUT be a mailbox: `LIB$PUT_OUTPUT` (RMS on
SYS$OUTPUT) then writes messages its parent reads. Device-specific FAB
and RAB results (`FAB$L_DEV` bits, record attributes) from the RMS
manual.

### The shared terminal

Terminal input moves to a reader goroutine feeding a line buffer
(keeping the console's line editing and the control keys of
`cmd/govax/attention.go` and `terminal_unix.go`); a VAX read that finds
no line waits (Phase 44's LEF or MWAIT state, predicate "a line is
ready"), so other processes run. The console's own prompt reads are
unchanged. Only one process reads at a time (VMS gives the terminal's
reads in request order; govax does the same). Output from several
processes interleaves by whole writes, as on VMS. Ctrl-C and Ctrl-Y go to
the process that enabled the AST, else stop the machine as now.

This is the riskiest piece for the interactive experience; it is last in
the phase and behind the scheduler flag (with the flag off, reads block
as today).

## Subtasks

1. **Request ownership.** Pending I/O records its owner; completion writes
   through the owner's address space; EF, AST, and boost to the owner.
   Audit every completion path (`qio.go`, `mbxdriver.go`, `ttdriver.go`,
   `diskdriver.go`, the ACP) from Phase 43's inventory.
2. **Mailboxes between processes.** Tests with two processes for every
   case `mbxdriver.go` documents: waiting read then write, write then
   read, IO$M_NOW both ways, full mailbox with resource wait on and off,
   end-of-file messages, `$CANCEL` of a waiting read, attention ASTs
   delivered to the enabling process, a write waiting for its message to
   be read.
3. **Protection and lifetime.** UIC checks on `$ASSIGN`; temporary
   mailbox deleted when the last channel in any process goes; logical
   names in the job table; `$GETDVI` of a mailbox from either side
   (message count in `DVI$_DEVDEPEND`, reference count, owner UIC).
4. **Common event flags across processes**, with CEF waits and temporary
   cluster lifetime. Tests.
5. **Global sections**: create, map, delete, rundown unmapping, reference
   counts, name scopes, errors (SS$_GPTFULL-style exhaustion,
   SS$_NOSUCHSEC, SS$_DUPLNAM...). Tests: two processes sharing a counter
   protected by a `BBSSI` spinlock under small quanta.
6. **RMS on mailboxes and NL:**. Tests: a child whose SYS$OUTPUT is a
   mailbox; its `LIB$PUT_OUTPUT` lines read by the parent.
7. **The shared terminal** (bug 6). Tests with a scripted input stream:
   a process waiting for input while another runs; Ctrl-C delivery;
   the console prompt afterwards.
8. **A MACRO test**: `testdata/mp/mbxpingpong.mar` (parent and child via
   `$CREPRC`, exchanging messages both ways through two mailboxes, plus
   a CEF handshake): an early version of Phase 48's milestone, without
   the files.
9. **Close-out.** Status, progress log, PLAN.md, CLAUDE.md, HELP.

## Optional probes (Decision 7)

- Mailbox IOSBs between two processes (the PIDs in each).
- `$GETDVI` of a mailbox with messages queued.
- A global section's `retadr` and `$MGBLSC` behavior.

## Open questions

- Whether a read's IOSB should carry the writer's PID when the write was
  IO$M_NOW (it does in govax now; check the manual's wording).
- `SEC$M_EXPREG` placement interplay with the image's P0 high-water mark
  (`RegionSize`).

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
