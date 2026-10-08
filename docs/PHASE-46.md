# Phase 46 — Multiprocessing, part 4: interprocess communication

**Status:** in progress (subtasks 1-5 done, 2026-10-08); decisions taken
2026-10-06 (see PHASE-43.md, Part A). Needs Phase 45.

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
  (`RegionSize`). *Settled in subtask 5:* the mapping starts at the
  first page above the mark and moves it, as `$CRETVA` does.

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
