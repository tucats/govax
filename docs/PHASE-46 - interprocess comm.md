# Phase 46 — Multiprocessing, part 4: interprocess communication

**Status:** done (2026-10-08); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Needs Phase 45. What's left is in "Carry forward".

The program this phase belongs to is described in
[PHASE-43 - processes](PHASE-43%20-%20processes.md), Part A. Read that first.

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

`$CREMBX`'s protection mask (recorded, not enforced, before subtask 3) is checked
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
   *Partly done in Phase 45's subtask 7*, which needed it for the
   termination message: `ioRequest.owner`, `completeIO` (IOSB through
   the owner's address space, its event flag, AST, and wait), the
   mailbox driver's read data, and attention ASTs. Still to do: the
   I/O completion boost to the owner, and the audit of the other
   drivers. *Done (2026-10-07).*
2. **Mailboxes between processes.** Tests with two processes for every
   case `mbxdriver.go` documents: waiting read then write, write then
   read, IO$M_NOW both ways, full mailbox with resource wait on and off,
   end-of-file messages, `$CANCEL` of a waiting read, attention ASTs
   delivered to the enabling process, a write waiting for its message to
   be read. *Done (2026-10-07).*
3. **Protection and lifetime.** UIC checks on `$ASSIGN`; temporary
   mailbox deleted when the last channel in any process goes; logical
   names in the job table; `$GETDVI` of a mailbox from either side
   (message count in `DVI$_DEVDEPEND`, reference count, owner UIC).
   *Done (2026-10-07).*
4. **Common event flags across processes**, with CEF waits and temporary
   cluster lifetime. Tests. *Done (2026-10-07).*
5. **Global sections**: create, map, delete, rundown unmapping, reference
   counts, name scopes, errors (SS$_GPTFULL-style exhaustion,
   SS$_NOSUCHSEC, SS$_DUPLNAM...). Tests: two processes sharing a counter
   protected by a `BBSSI` spinlock under small quanta. *Done (2026-10-08).*
6. **RMS on mailboxes and NL:**. Tests: a child whose SYS$OUTPUT is a
   mailbox; its `LIB$PUT_OUTPUT` lines read by the parent.
   *Done (2026-10-08).*
7. **The shared terminal** (bug 6). Tests with a scripted input stream:
   a process waiting for input while another runs; Ctrl-C delivery;
   the console prompt afterwards. *Done (2026-10-08).*
8. **A MACRO test**: `testdata/mp/mbxpingpong.mar` (parent and child via
   `$CREPRC`, exchanging messages both ways through two mailboxes, plus
   a CEF handshake): an early version of Phase 48's milestone, without
   the files. *Done (2026-10-08).*
9. **Close-out.** Status, progress log, PLAN.md, CLAUDE.md, HELP.
   *Done (2026-10-08).*

## Optional probes (Decision 7)

- Mailbox IOSBs between two processes (the PIDs in each).
- `$GETDVI` of a mailbox with messages queued.
- A global section's `retadr` and `$MGBLSC` behavior.

## Open questions

- Whether a read's IOSB should carry the writer's PID when the write was
  IO$M_NOW. *Settled by probe 3:* it does, on VMS 7.3 as in govax.
- `SEC$M_EXPREG` placement interplay with the image's P0 high-water mark
  (`RegionSize`). *Settled in subtask 5:* the mapping starts at the
  first page above the mark and moves it, as `$CRETVA` does.

## Carry forward

Things Phase 46 did not get to, so that they are not lost. Each is either
waiting on a run on the VAX, or optional work for a later phase.

1. **The keywords of `$ENQ`'s 12th and 13th arguments and `$GETLKI`'s
   7th** aren't known (round 6: not NULLARG); govax's macros call them
   ARG12, ARG13, and ARG7. A later probe round can try candidates, as
   round 5 did for `$IDTOASC`.
2. **Unconfirmed behavior** (each marked in the progress log; probe 3
   settled the rest): the boost classes of a terminal set or sense mode,
   of a mailbox attention AST, of `$SETEF` (none), and of a CTRL/C or
   CTRL/Y AST to a waiting process; that either access is enough for a
   mailbox `$ASSIGN`, and SETPROT's argument and who may use it;
   `$DGBLSC` not checking the protection mask; no logical-name
   translation of a section name; which rule makes a file section with
   no channel SS$_IVSECFLG; whether `$OPEN` of a mailbox stores 0 in
   FAB$W_MRS or leaves it alone.
3. **Terminal simplifications:** a terminal `$QIO` read that must wait
   makes the `$QIO` itself wait, rather than returning with the read
   pending (a program doing other work before it waits for the read's
   event flag would see the difference): moved to Phase 49. *Done
   (2026-10-08, the program's close-out):* the old `DECC$GETS` and
   `EXE$INPUT` shims wait their turn at the shared terminal, as
   LIB$GET_INPUT does, rather than blocking every process
   (`TestTerminal_inputShims`). The console's own prompt after a run
   stopped while a process waited for input isn't tested.
4. **File-backed sections** (`$CRMPSC` of a file's blocks, private or
   global) are SS$_UNSUPPORTED; so is `SEC$M_EXPREG` in P1.
5. **The flaky test** `TestExecute_stopsOnAttention` (it raced
   `Engine.Attention` on a goroutine against `Execute`'s start, whose
   `BeginRun` clears a CTRL/C typed before it). *Fixed (2026-10-08):*
   the goroutine presses CTRL/C every millisecond until `Execute`
   returns.

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
- 2026-10-07: Subtask 1 (request ownership), finishing what Phase 45's
  subtask 7 started. From *VAX/VMS Internals and Data Structures*,
  sections 10.2.3 (SCH$RSE) and 10.2.4 (Table 10-3).
  - **Events are reported when they happen.** `Environment.reportEvent`
    (`waits.go`) is SCH$RSE's part: if the process is waiting (and not
    suspended), and its wait is now over or it can now take an AST
    where it waits, it becomes computable at once, boosted by the
    *event's* class; the engine is asked to reschedule when its new
    priority is at least the current process's. A process that isn't
    waiting ignores the event. Before this, the owner of a completed
    request stayed in its wait until the scheduler's next call (its
    test of every waiter), which, with another process spinning, could
    be a whole quantum later. That test stays for events not reported
    this way (a `$WAKE`, a `$SETEF`, ...). `endWait` now returns
    whether the process may preempt.
  - **Who reports.** `completeIO` reports for the request's owner, with
    the request's class; the mailbox driver's attention ASTs report for
    the process that enabled them; `expireTimers` reports when any of a
    process's timers expired, with PRI$_TIMER (3), so a `$WAITFR` ended
    by a `$SETIMR` flag now gets 3 rather than an event flag wait's 2.
  - **The class of a request** (`ioRequest.boost`, `ioBoost` in
    `qio.go`): terminal reads (`terminalInputFunctions`, `ttdriver.go`)
    PRI$_TICOM (6), any other terminal function PRI$_TOCOM (4), every
    other device (disk direct I/O, mailbox and null buffered I/O)
    PRI$_IOCOM (2). *Unconfirmed:* the class VMS's terminal driver
    gives set and sense mode (govax: output's), and the class of a
    mailbox attention AST (govax: PRI$_IOCOM; the book says routines
    queuing ASTs may choose any class).
  - **Audit** of the completion paths. Only the mailbox driver completes
    a request for a process that may not be current: a pending read
    completed by another process's write, a write waiting for its
    message to be read, `$CANCEL`/`$DASSGN`, the termination message,
    and attention ASTs. All go through `completeIO` (or, for the ASTs,
    `deliverAttention`), so to the owner. Everything else completes
    during the requester's own call, when the requester is the current
    process: the terminal driver (reads block, until subtask 7), the
    disk driver and its ACP functions (`diskcreate.go`, `diskdelete.go`,
    `diskattr.go`, `disklogical.go`), the null device, and the
    services that complete like a `$QIO` (`$GETJPI`, `$GETDVI`,
    `$GETSYI`, `$BRKTHRU`), which write their IOSB through the CPU's
    own mapping, correctly.
  - Tests: `corevms/ioboost_test.go` (`reportEvent`'s rules: an event
    that doesn't end the wait, a suspended process, the event's class
    winning over the wait's; another process's mailbox write ending a
    `$QIOW` read at once with a boost of 2; a timer's boost of 3; the
    class table) and `console/ioboost_test.go` (`TestIOCompletion_preempts`:
    on a booted machine, process 1 waits for a mailbox read and
    process 2, at the same base priority, writes with IO$M_NOW and
    spins; process 1 runs at the next instruction boundary, before
    process 2 goes on past its `$QIO`. With the report taken out, it
    fails: with a long quantum, nothing gives process 1 the CPU).
- 2026-10-07: Subtask 2 (mailboxes between processes).
  - **Harness** (`console/mbxpair_test.go`): a booted machine, process 1
    and a hand-built process 2, each running its own MACRO program in
    its own P0, sharing a permanent mailbox PIPE (process 1 `$CREMBX`es
    it and `$WAKE`s process 2, which `$ASSIGN`s it). The order of events
    is made certain by priority: process 1 at base 10, process 2 at 2,
    so process 2, even boosted by 6, runs only while process 1 waits,
    and process 1 runs again as soon as its wait ends. Where process 1
    must wait for process 2 to block, it hibernates with a `$SCHDWK`
    10 s on, which the idle loop reaches once process 2 waits. (On the
    booted test machine each instruction is one emulated millisecond,
    so a 10 ms timer fired before process 2 had done anything.)
  - **Cases**, each checking both IOSBs (with the other process's PID)
    and the data: a waiting `$QIOW` read, then a plain write handed to
    it (process 1 seen in LEF, and running before process 2's `$QIOW`
    returns); a plain write queued and pending until read (the writer
    seen waiting for it, in LEF); IO$M_NOW reads and writes, and an
    end-of-file message; a full mailbox with resource wait on (the
    writer seen in MWAIT/RWMBX, its write finishing into process 1's
    waiting read) and off (`$SETRWM`: SS$_MBFULL, the message lost);
    `$CANCEL` of a pending read (SS$_CANCEL, and the next message goes
    to the next read, not the cancelled one's buffer); and the three
    attention ASTs (read attention, write attention, room
    notification), each run in the process that enabled it, ending its
    `$HIBER`, and in no other.
  - **Fixed:** a write handed straight to a waiting read put the
    *writer's* PID in its IOSB rather than the reader's (`send`, in
    `mbxdriver.go`); within one process the two are the same, so no
    earlier test could see it. The fix is caught by two of the cases.
    *Unconfirmed:* an IO$M_NOW write handed to a waiting read gets the
    reader's PID too (a queued one gets 0, as the driver's comment has
    said since Phase 26).
  - **Fixed:** a writer waiting in RWMBX was noticed only at the
    scheduler's next look at its waiters. A read that makes room, or a
    read that starts waiting, now reports a resource available
    (`mailboxResourceAvailable`, PRI$_RESAVL) to each process waiting
    for a mailbox, as subtask 1 does for I/O completions;
    `reportEvent`'s test of the wait leaves a writer to another,
    still full mailbox waiting. `corevms/ioboost_test.go`'s
    `TestReportEvent_mailboxRoom` checks it (the console cases can't:
    there process 1 waits right after making room, and the scheduler
    finds the writer anyway).
- 2026-10-07: Subtask 3 (protection and lifetime).
  - **UIC protection** (`corevms/uicprot.go`, `Process.uicAccess`): the
    16-bit mask of four categories (SYSTEM, OWNER, GROUP, WORLD; a set
    bit denies read, write, logical, or physical access), a process in
    every category it qualifies for (SYSTEM: group up to MAXSYSGROUP,
    octal 10, or SYSPRV, or GRPPRV in the owner's group), an access
    allowed if any of its categories allows it; BYPASS allows all,
    READALL read. From the *Guide to VMS System Security*; ACLs aren't
    modeled.
  - **Mailbox protection:** `$ASSIGN`, and `$CREMBX` reaching an
    existing mailbox by name, need read or write access (SS$_NOPRIV);
    each read needs read access and each write or end-of-file message
    write access, checked when the `$QIO` is made. The system's own
    messages (termination message, OPCOM's replies) aren't checked.
    `IO$_SETMODE!IO$M_SETPROT` sets the mask from p2, for the owner or a
    process with BYPASS or SYSPRV. *Unconfirmed:* that either access is
    enough for `$ASSIGN`; SETPROT's argument and who may use it.
  - **Owner:** a mailbox's owner (DVI$_PID, DVI$_OWNUIC) stays its
    creator. Before, every `$ASSIGN` made the assigning process the
    device's owner, which, with two processes, would have checked the
    protection against the wrong UIC; other devices keep that rule.
    *Unconfirmed:* that VMS's DVI$_PID of a mailbox is its creator's.
  - **Lifetime:** a temporary mailbox already went with its last
    channel, the reference count being the device's, shared by every
    process. **Fixed:** its logical name is deleted from the table it
    went in (`LNM$JOB_xxxxxxxx`, resolved when it's defined), not
    `LNM$TEMPORARY_MAILBOX` as seen by the deleting process, which, in
    another job, is another job table: the name was left behind.
  - **$GETDVI:** a mailbox's DVI$_DEVDEPEND has its message count in the
    low word (the I/O User's Reference Manual's description; to check
    with the optional probe), and DVI$_VPROT its protection mask (0
    for other devices, whose protection govax doesn't model).
  - Tests (`corevms/mbxprot_test.go`): the access rules; `$ASSIGN` and
    `$CREMBX` refused and allowed by category and privilege; read-only
    and write-only access; SETPROT by owner, non-owner, and SYSPRV; a
    temporary mailbox outliving its creator's channel in a subprocess,
    invisible by name to another job, going with its last channel in
    another job (its name with it, from the creator's job table) and
    with a deleted process's rundown; and `$GETDVI` from both sides and
    by name.
- 2026-10-07: Subtask 4 (common event flags across processes). From
  *VAX/VMS Internals and Data Structures*, sections 10.1 (common event
  blocks, CEF wait queues) and 12.1.2-12.1.4 (SCH$POSTEF).
  - **Already in place:** clusters are system state shared by every
    process in a UIC group; a wait on a common flag is a CEF wait whose
    test reads the cluster each time; image rundown (and so process
    deletion, which runs it) disassociates, and a temporary cluster goes
    with its last association. The scheduler's test of every waiter
    already ended another process's CEF wait, and `$SETEF` of a common
    flag asked for a reschedule so that test came soon.
  - **SCH$POSTEF** (`Environment.postFlag`, `eventflags.go`): every place
    that sets an event flag (`$SETEF`, a request completing in
    `completeIO`, a `$QIO` rejected after its flag was cleared, a timer,
    `$GETJPI`, `$GETDVI`, `$GETSYI`, `$BRKTHRU`) now sets it through
    one routine, which, for a common cluster, reports the event
    (`reportEvent`) to every process associated with the cluster
    (`reportClusterEvent`): each whose wait it satisfies becomes
    computable at once, with the setter's boost class, and may preempt.
    Before, only `$SETEF` caused a reschedule: a common flag set by a
    timer or a completing request in one process left the others
    waiting until the scheduler next looked, up to a quantum later.
  - **Boost classes:** a request's own class (I/O completion for
    `$GETJPI` and the like, the request's for a `$QIO`), PRI$_TIMER for
    a timer, and none (PRI$_NULL) for `$SETEF`, which Table 10-3 doesn't
    list (its "other events with no boost"). *Unconfirmed:* `$SETEF`'s
    class. The fallback test of every waiter still gives an event flag
    wait an I/O completion's boost.
  - Tests: `corevms/cefwait_test.go` (`$SETEF` ending another process's
    `$WAITFR` at once with no boost; a `$WFLAND` across processes needing
    both flags; the same cluster under cluster numbers 2 and 3; a
    same-named cluster in another UIC group left alone; a timer's flag
    (boost 3) and a `$GETJPIW`'s (boost 2) ending another's wait; a
    temporary cluster outliving its creator's `$DACEFC` in another
    process, keeping its flags, and going with that process's deletion,
    a permanent one staying) and `console/cefpair_test.go`
    (`TestCommonFlagPair_handshake`: on a booted machine, two processes
    hand flags 65-68 back and forth, each seen in CEF; each wait ends
    before the setter goes on past the instruction that set the flag.
    Its last flag is set by a `$GETJPIW`; with `postFlag`'s report taken
    out, process 2 runs on past it).
- 2026-10-08: Subtask 5 (global sections). From the System Services
  manual's descriptions and *VAX/VMS Internals and Data Structures*,
  sections 14.3 (the global section descriptor) and 16.3 (the section
  services).
  - **`corevms/gblsec.go`:** `$CRMPSC`, `$MGBLSC`, `$DGBLSC`, and the
    system's `GlobalSections` table (`System.Sections`). Page-file
    sections only (`SEC$M_GBL!SEC$M_PAGFIL`, with `SEC$M_WRT`,
    `SEC$M_SYSGBL`, `SEC$M_PERM`, `SEC$M_EXPREG`); a file section
    (private or global) or one mapped by PFN is SS$_UNSUPPORTED. The
    `SEC$` names are in `vmsdef.LibrarySymbols` (STARLET.OLB's
    `$SECDEF`), not `vmsdef.Symbols`.
  - **Frames:** a section's physical pages are allocated, zeroed, when
    it's created, and the section owns them; a mapping writes valid PTEs
    pointing at them (no global page table, no paging). The reference
    count is the number of process PTEs mapping the section, as the
    book says. Every place that gave back a replaced PTE's physical page
    (`replacePTE`, so `$CRETVA`/`$DELTVA`/`$CNTREG`, and
    `TeardownAddressSpace`) now goes through `System.releaseFrame`,
    which, for a section's page, drops the count instead. A temporary
    section goes when the count reaches zero; a permanent one
    (PRMGBL) stays until `$DGBLSC`, which (book, 16.3.3) takes the name
    off the list at once (a new section may take it), clears the
    permanent flag, and leaves the section to go with its last mapping.
  - **Image rundown** (`unmapSections`): VMS deletes P0 at image exit;
    govax keeps P0, but each page the process mapped to a section is
    reset to a new process's demand-zero PTE and the section loses the
    reference. Process deletion runs it too, and teardown catches
    anything left.
  - **Names and scope:** 1-43 characters (SS$_IVLOGNAM), compared as
    given, a group section seen in its creator's UIC group, a system
    one (SYSGBL to create or delete) everywhere. *Unconfirmed:* no
    logical-name translation of the name (the book mentions one for
    shared-memory sections; govax doesn't model shared memory).
  - **Idents:** a section's version (major in bits 24-31, minor 0-23)
    from the ident quadword; `$MGBLSC` matches with SEC$K_MATALL,
    MATEQU (same version), or MATLEQ (same major, section minor at
    least the mapper's); another control is SS$_IVSECIDCTL.
    *Unconfirmed:* a `$CRMPSC` whose ident matches no existing section
    of the name creates another (SS$_CREATED), several sections sharing
    the name.
  - **Mapping:** at inadr's pages (either order), lowest first, as many
    as both the range and the section from relpag have; read-only
    (KR/ER/SR/UR for the access mode) or, with SEC$M_WRT, read/write.
    `$CRMPSC` of an existing section maps it and returns SS$_NORMAL,
    SS$_CREATED when it made it. Mapping needs read access by the UIC
    protection mask (`prot`, against the creator's UIC; `uicprot.go`),
    write access too for a writable mapping, and a writable mapping of a
    section not created writable is SS$_NOPRIV. With SEC$M_EXPREG the
    mapping starts above P0's high-water mark (never page 0) and moves
    it; expanding P1 is SS$_UNSUPPORTED.
  - **Limits:** GBLSECTIONS (128, SS$_GSDFULL) and GBLPAGES (4096,
    SS$_GPTFULL), fields of the table; physical memory running out is
    SS$_INSFMEM.
  - *Unconfirmed statuses:* SS$_ILLPAGCNT for pagcnt 0, SS$_BADPARAM
    for relpag past the section's end, SS$_IVSECFLG for SEC$M_CRF with
    SEC$M_PAGFIL and for PERM, SYSGBL, or PAGFIL without GBL; `$DGBLSC`
    doesn't check the protection mask (only the privileges).
  - Tests: `console/gblsec_test.go` (on process 1, through the P1
    vector: create and map, a second `$CRMPSC` and a read-only
    `$MGBLSC` from page 1 onto the same frames; the flag, name, and
    argument errors; protection; each ident match control; a temporary
    section going with its last `$DELTVA` and its frames freed, a
    permanent one keeping its contents unmapped, `$DGBLSC` while
    mapped, image rundown; the limits and SEC$M_EXPREG) and
    `console/gblpair_test.go` (`TestGlobalSectionPair_spinlock`: two
    processes map COUNTER at different addresses and each add 1 to it
    300 times under a `BBSSI`/`BBCCI` spinlock, with a 5-instruction
    quantum; the CPU changes process while the lock is held, and all
    600 increments land. Without the lock, about 240 are lost.
    Process 2's deletion takes its reference away and leaves process
    1's).
- 2026-10-08: Subtask 6 (RMS on mailboxes and NL:).
  - **RMS side** (`rms/recdevice.go`): a `RecordDevice` (a stream on one
    channel: `Put`, `Get`, `Close`, its DEVCHAR and largest record) and
    a `DeviceOpener`, `rms.Context.Devices`. `$CREATE` and `$OPEN` of a
    name whose device lookup is a mailbox or NL: open a stream (before
    the terminal and volume cases), with FAB$L_DEV and FAB$L_SDC the
    device's DEVCHAR and FAB$W_MRS its largest message; `$CONNECT` has
    nothing to arm; `$PUT` and `$GET` go to the stream; `$CLOSE` and
    RMS rundown close it; `$DISPLAY` of one is RMS$_IFI, as for the
    terminal. Statuses: a record longer than the mailbox's largest
    message RMS$_RSZ; another failure of a put RMS$_WER, of a get
    RMS$_RER, with the SS$ status in STV (the RMS manual's convention);
    the end of the data RMS$_EOF; an open refused by the mailbox's
    protection RMS$_PRV (STV SS$_NOPRIV). *Unconfirmed:* FAB$W_MRS.
  - **Device side** (`corevms/recdevice.go`): the Environment is the
    opener. Opening assigns an executive-mode channel (so a temporary
    mailbox lasts while the file is open) after checking the FAB's
    access against the mailbox's protection. A `$PUT` is a mailbox write
    that doesn't wait for its reader (a waiting `$QIO` read gets it at
    once, or it's queued); a full mailbox makes it wait for room
    (RWMBX), or, with resource wait off, SS$_MBFULL. A `$GET` takes the
    oldest message, completing a write waiting for it and making room,
    or, with none, waits in LEF; every write that queues a message
    (`send`, a `$QIO`'s or a `$PUT`'s) reports the event to the
    processes waiting in a `$GET` (`Mailbox.recordReaders`), so the
    reader runs at once, as subtask 1's completions do. NL: takes any
    record and gives none. *Unconfirmed:* that RMS's `$PUT` to a mailbox
    doesn't wait for the message to be read (the design's choice; a
    probe could settle it).
  - **`LIB$PUT_OUTPUT`** now writes to SYS$OUTPUT, not straight to the
    terminal: `Environment.PutOutput` translates SYS$OUTPUT and, for a
    mailbox or NL:, writes the record as a message through a stream the
    process keeps open in executive mode (VMS's process-permanent
    SYS$OUTPUT; reopened if the translation changes), returning the
    write's status. Anything else is a line on the terminal, as before.
  - **Fixed:** a shim that waited (`ErrWait`, now possible for
    `LIB$PUT_OUTPUT` to a full mailbox) wasn't run again: `XFC$SHIM` set
    R0 (which selects the shim) to the shim's result and went on, so the
    call was lost. It now re-executes the XFC with R0 untouched, as
    `XFC$P1VECTOR` does for a waiting service (`cpu/xfc.go`;
    `TestEmulXfcShimWait`). Subtask 7's terminal reads will need it too.
  - Tests (`console/rmsmbx_test.go`): `TestRMSMailbox_childOutput`
    (process 1 makes the temporary mailbox CHILDOUT, room for two
    lines, and `$CREPRC`s a child at a lower priority with CHILDOUT as
    its output; while process 1 sleeps the child writes two lines and
    waits for room (seen in MWAIT); process 1 `$OPEN`s CHILDOUT and
    `$GET`s the four lines whole and in order, waiting (seen in LEF)
    for the ones not yet written) and `TestRMSMailbox_null` (NL::
    `$CREATE`, `$CONNECT`, `$PUT`, `$CLOSE`, `$OPEN`, `$CONNECT` succeed,
    `$GET` is RMS$_EOF, FAB$L_DEV says record oriented).
- 2026-10-08: Subtask 7 (the shared terminal; bug 6).
  - **No new reader goroutine was needed:** cmd/govax's
    `attentionStdin` already has one pump goroutine owning the host
    terminal and a byte channel that both readline (at a prompt) and the
    programs' reads take from. What was missing was a way to ask whether
    a read would wait: `attentionStdin.Ready` (a byte queued, or the
    input ended), passed on by the console's `consoleInput`, is
    `corevms.TerminalSource`. An input that can't say (a test's script,
    a file) is always ready, so nothing changes for it.
  - **`corevms/terminal.go`:** every process reads the terminal through
    one buffer (`System.terminalReader`; before, each process wrapped the
    shared stream in its own `bufio.Reader`, and what one took ahead was
    lost to the others). A read (`awaitTerminal`) joins the terminal's
    FIFO queue and goes ahead only when it's first and a whole line is
    buffered (`lineReady`: its terminator, CTRL/Z, its size, or the end
    of the input, pulling in whatever the source has without waiting);
    otherwise it waits in LEF, its service run again when that's true.
    A read's prompt is written once, when its turn comes, so a second
    process's prompt doesn't appear while the first is still being
    typed to. A read that has its line, or a process's rundown, gives
    up the turn. Only with the scheduler on and a `TerminalSource`
    input; otherwise reads block as before.
  - **The readers:** the terminal driver's reads (`terminalRead`: a read
    that must wait returns `ioResourceWait`, so the `$QIO` is made again;
    IO$M_PURGE discards type-ahead only when the read is first made;
    IO$M_TIMED with 0 seconds never waits), RMS's terminal `$GET`
    (through `rms.Context.AwaitTerminal`/`TerminalDone`; the RAB's prompt
    goes with the read), and `LIB$GET_INPUT` and `LIB$GET_FOREIGN`
    (`Environment.ReadInputLine` now returns ErrWait, which subtask 6's
    `XFC$SHIM` fix lets a shim return). The old `DECC$GETS` and
    `EXE$INPUT` shims still block. *Unconfirmed simplification:* a
    terminal `$QIO` read that must wait makes the `$QIO` itself wait,
    rather than returning with the read pending; a program that waits
    for the read's event flag right away sees no difference.
  - **Idling:** with every process waiting and no timer due, a process
    waiting for a line makes the idle loop wait in the host for input
    (polling `Ready` each millisecond, up to 50 ms, then back to the
    engine, which checks CTRL/C), instead of spinning the waiters.
  - **CTRL/C and CTRL/Y** go to the process that enabled an AST for the
    key, the running one first, then the others by PID
    (`System.AttentionAny`, used by `Console.HandleAttention`); a waiting
    process is told at once (boost class PRI$_TICOM, *unconfirmed*). With
    no AST enabled anywhere, the machine stops as before.
  - Tests: `corevms/terminal_test.go` (two processes reading in turn,
    each in LEF until its line and its turn; a partial line ends no
    wait; prompts written once, the second's only when the first has
    its line; type-ahead kept for the next read; a deleted reader gives
    up its turn; no waiting without the scheduler) and
    `console/sharedterm_test.go` (`TestSharedTerminal`: on a booted
    machine with scripted input, process 1 waits in a `$QIOW` read
    (LEF) while process 2 counts; CTRL/C runs process 2's CTRL/C AST
    and doesn't stop the machine; the typed line completes process 1's
    read, IOSB and data). Not tested here: the console's own prompt
    after a run stopped while a process waited (readline's reads go
    through `promptReader`, which this doesn't change).
  - `TestExecute_stopsOnAttention` failed once in a full run and passed
    30 times alone: it races `Engine.Attention` on a goroutine against
    `Execute`'s start. A pre-existing flake, not this subtask's.
- 2026-10-08: Subtask 8 (a MACRO test).
  - **`testdata/mp/mbxpingpong.mar`** and its child **`mbxpong.mar`**,
    written, like Phase 45's `crechild.mar`, with system services and RTL
    routines only, to run unchanged on VMS. The parent associates a
    temporary common event flag cluster (PINGPONG, flags 64-95), makes
    two temporary mailboxes with logical names (PP_TO_CHILD,
    PP_TO_PARENT) and a termination mailbox, and `$CREPRC`s the child
    named on its command line. The child associates the cluster,
    `$ASSIGN`s the mailboxes by name (from the job table they share), and
    sets flag 65; the parent waits for it, then three times writes
    "PING n" (a plain write, finishing when the child has read it) and
    reads the child's "PONG n". An end-of-file message ends the child's
    loop, which it reports with flag 66; the parent's flag 67 lets it
    end, and the parent prints the child's final status from the
    termination message.
  - **Deterministic output:** each side prints before it sends and waits
    for the other before it prints again, so the fifteen lines (in the
    program's header) are the same however the two processes are
    scheduled.
  - Test: `console/mbxpingpong_test.go` (`TestMbxPingPong`: both images
    assembled and linked by govax, the parent run as process 1's image,
    at a long quantum and at a 5-instruction one; the output whole, and
    only process 1 left). It passed as first written: no fixes were
    needed. Not yet run on VMS (`testdata/mp/README.md` has the
    commands).
- 2026-10-08: Subtask 9 (close-out): status, the carry-forward list,
  PLAN.md, CLAUDE.md, and `console.help` (KEYS: CTRL/C and CTRL/Y ASTs in
  any process, and a program waiting for input no longer holding up the
  others).
- 2026-10-08: The VAX runs prepared (`testdata/mp/run46`), so that one
  session answers everything Phases 45 and 46 left for VMS:
  - **Macro probe round 6** (`testdata/mp/macros/r6_*.mar`, from gen.go,
    round 4's generator now taking the round's prefix): `$CRMPSC`,
    `$MGBLSC`, `$DGBLSC`, `$ENQ(W)`, `$DEQ`, `$GETLKI(W)`; round 5 runs in
    the same session, each round with its own log
    (`TestServiceMacroObjects` knows round 6's).
  - **Definition probes** for `$SECDEF`, `$LCKDEF`, `$LKIDEF`, `$PSLDEF`,
    and `$DCDEF` (`testdata/mp/defs`; Phase 45's log is now `defs45.log`).
  - **Probe 3** (`testdata/mp/probe3`): part 1, mailbox IOSBs between two
    processes (the child is a mailbox echo that reports each read's IOSB),
    `$GETDVI` from both sides and with messages queued; part 2, global
    section results and statuses (by `CALLS`); part 3, RMS on a mailbox
    and NL:, with an asynchronous `$PUT` to ask whether it waits for the
    read without risking a deadlock. `TestProbe3` runs it under govax.
  - **Fixed:** `SYS$WAIT` wasn't implemented (its P1 vector entry was a
    reserved operand fault), which probe 3's part 3 found under govax.
    govax's RMS completes every operation before returning, with or
    without RAB$V_ASY, so `$WAIT` returns RAB$L_STS (`rms/wait.go`,
    `TestSysWait`).
  - The ping-pong pair gets `pingpong.com`; its VMS log will be
    `testdata/mp/vax/pingpong.log`.
- 2026-10-08: The VAX session ran (`testdata/mp/run46`; the author
  audited the MACRO logs, and the clean-room hook's entry for them is
  gone).
  - **Probe 3** stopped at its first read: VMS refuses a mailbox read
    whose buffer is longer than the mailbox's largest message
    (SS$_MBTOOSML), as it refuses such a write. Its child was left waiting
    for its report to be read and had to be stopped by hand. **Fixed:**
    govax's mailbox driver refuses the read too (`mbxdriver.go`;
    `TestMailboxDriver_*` in `mailbox_test.go`); with it, govax's run of
    the original probe stopped where VMS's did. The probe now reads no
    more than the largest message, and deletes its child if it ends
    early. It is to run again by itself (`testdata/mp/probe3`).
  - **The ping-pong pair** ran on VMS; the parent's ten lines match
    govax's (the child's went to the terminal, not the log).
  - **Definitions:** `$SECDEF`, `$LCKDEF`, `$LKIDEF`, `$PSLDEF`, and
    `$DCDEF` (819 new names; `SEC$` and `DC$` agree with STARLET.OLB's),
    merged into `vmsdef.Symbols`, and their macros are now in govax's
    library (`mkdefs` builds every family in the file, now named
    `testdata/mp/defs/defined.txt`).
  - **Macros, round 5:** `$IDTOASC`'s third keyword is NAMBUF (the manual
    says RESNAM), and ID is required; `$TRNLOG`'s and `$CRELNT`'s sizes
    were as assumed. **Round 6:** `$CRMPSC`, `$MGBLSC`, `$DGBLSC`, `$ENQ`,
    `$ENQW`, `$DEQ`, `$GETLKI`, and `$GETLKIW` written, in all three
    forms; every call real MACRO assembled without error gives the same
    code (`TestServiceMacroObjects`, with rounds 5 and 6 added). VMS 7.3's
    `$ENQ` has 13 arguments and `$GETLKI` 7; the keywords of the ones past
    the manual's lists aren't known.
- 2026-10-08: Probe 3's second run (`testdata/mp/probe3/vax/probe3.log`)
  ran to the end. govax's report (`TestProbe3`) now matches it line for
  line, apart from PIDs and the mapped addresses (VMS's P0 holds the
  shareable RTL images after the program, govax's doesn't; the sizes
  match). **Confirmed** as govax had them: the PIDs in every mailbox
  IOSB but one, `$GETDVI`'s message count and reference count, the
  section statuses for flags without SEC$M_GBL, ident matching (and a
  `$CRMPSC` whose ident matches no section of the name creates another),
  NOSUCHSEC, `$DGBLSC`, FAB$L_DEV, and `$GET` of messages and end of file.
  **Changed** to VMS's behavior:
  - A mailbox's DVI$_PID is 0, from either side (it was the creator's);
    its owner UIC is still the creator's.
  - An IO$M_NOW write's IOSB has PID 0 even when a waiting read takes
    the message (it had the reader's).
  - `$CRMPSC` and `$MGBLSC` write -1 to both longwords of retadr when
    they fail before mapping a page (`failRetadr`).
  - `$CRMPSC` with pagcnt 0 and `$MGBLSC` with relpag past the section's
    end are SS$_ENDOFFILE (were SS$_ILLPAGCNT and SS$_BADPARAM); a file
    section with no channel is SS$_IVSECFLG (was SS$_UNSUPPORTED, which
    a file section on a channel still is).
  - A section created without SEC$M_WRT may be mapped writable (was
    SS$_NOPRIV).
  - `$OPEN` of a mailbox leaves FAB$W_MRS alone (it stored the largest
    message; VMS showed 0).
  - **An RMS `$PUT` to a mailbox finishes when its message has been
    read** (it finished at once): VMS returned RMS$_PENDING for an
    asynchronous `$PUT` no one was reading. A synchronous `$PUT` (and so
    `LIB$PUT_OUTPUT` to a mailbox SYS$OUTPUT) waits in LEF; with
    RAB$V_ASY it returns RMS$_PENDING and `$WAIT` waits
    (`rms/recdevice.go`, `rms/wait.go`, `corevms/recdevice.go`). The
    write sets no event flag (`ioRequest.noFlag`).
    `TestRMSMailbox_childOutput` now sees the child waiting for its first
    line to be read.
  - RAB$L_STV after a `$GET` of end of file is 0 (it was RMS$_EOF).
