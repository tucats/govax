# Deviations from ISA/Hardware/Software

This is a running log of places where govax 
appears not to match the VAX architecture/instruction-set spec, or the emulation
of VAX/VMS services, library functions, etc. In short, this tracks known cases
where fidelity to a real VAX/VMS system is intentionally different, or is different
and a resolution has not yet been implemented.

## Policy

Per project guidance (see `CLAUDE.md`): the C source's fidelity to the VAX ISA is good
but not perfect. When a suspected ISA/behavior mismatch is found while porting:

- If it's a clear, obvious logic error unrelated to ISA semantics (e.g. an off-by-one,
  a copy-paste mistake, dead/unreachable code) — just fix it in the Go code, no need to
  log it here.
- If it's a genuine ISA/hardware-definition fidelity question (the emulated behavior
  doesn't match the spec, or doesn't match what the C code's own comments claim) — log
  it below with enough detail to revisit later, and **replicate the C source's current
  behavior in the Go port** rather than fixing it now, unless the fix is clear-cut and
  fits naturally in the current change's scope.
- When it's not obvious which of the above applies, ask before deciding unilaterally.

Entries get resolved (fixed or deliberately kept, with rationale) during Phase 12
(`PHASE-12.md`) or whenever the relevant subsystem gets a dedicated debugging pass.


## Phase 22 (RMS / `ods2`) findings

Phase 22 (`PHASE-22.md`) has no `reference/eVAX` counterpart at all — its own
correctness reference is the real VMS RMS manual (`rms_manual.pdf`) and the
sibling `github.com/tucats/ods2` module's actual on-disk/record-format
behavior, not the C source. The entries below are that phase's own findings
in this same spirit (a real-RMS-vs-actual-implementation gap worth recording
rather than silently guessing at), kept in this file per that phase's own
"Bug-fixing policy"-style direction rather than starting a second log.

### [Phase 22] `FAB$C_UDF` (undefined record format, `FAB$B_RFM` = 0) is rejected outright instead of resolved to a default

- **Where**: `internal/rms/create.go`'s `createOnVolume` (`SYS$CREATE`'s
  real-volume path): `format := ondisk.RecordFormat(rfm); if format <
  ondisk.RecordFormatFixed || format > ondisk.RecordFormatStreamCR { return
  0, rmsInvalidRFM, nil }` — `ondisk.RecordFormatUndefined` (`FAB$C_UDF`,
  the real VMS value `0`) falls outside that range and is reported as
  `RMS$_RFM`, a hard failure.
- **What**: real RMS lets a calling program leave `FAB$B_RFM` unset (`0`,
  `FAB$C_UDF`) and picks a sensible default record format on its behalf
  (`rms_manual.pdf`'s own `$CREATE`/FAB description) rather than rejecting
  the call outright. This phase's own acceptance-test program and every
  unit-test fixture built so far always sets `FAB$B_RFM` explicitly (per
  `docs/PHASE-22.md` subtask 5's own progress-log note), so there's no
  concrete calling program yet to motivate *which* default `ods2` should
  pick (Fixed? Stream? something derived from `FAB$B_RAT`?) — a real,
  deliberate gap in what's implemented, not a bug in what is.
- **Status**: open, deferred. Left rejecting `FAB$C_UDF` with `RMS$_RFM`
  until a real calling program that relies on the default-format behavior
  shows up to motivate the right default, rather than guessing one now.

## Phase 25 (logical names) findings

Phase 25 (`PHASE-25.md`) replaced eVAX's logical-name support rather than
porting it. That was the user's explicit direction: the existing behavior was
treated as suspect, and VAX/VMS 7.3 is the target. The references are the VSI
OpenVMS User's Manual (chapter 11), the VMS 5.0 System Services Reference
Manual, and the CLRM §2.2. The first group of entries below records each eVAX
behavior the phase deliberately changed, so the difference from
`reference/eVAX` can be traced. The second group records the gaps that remain
against real VMS.

### [Phase 25] No job or cluster tables, no quotas or table protection

- **Where**: `internal/lnm.NewDatabase`; `$CRELNT`'s `quota`/`promsk`.
- **What**: VMS's `LNM$FILE_DEV` is process, job, group, system (and cluster).
  govax has one process and no cluster, so it has no `LNM$JOB` table, and
  MOUNT's `DISK$label` always goes in the system table (VMS uses the job
  table without `/SYSTEM`). Table quotas and protection masks are accepted and
  ignored.
- **Status**: open, deliberate (open question 1). The directory-based
  design makes adding `LNM$JOB` a few lines.

### [Phase 25] Smaller DCL/RMS simplifications

- **Where**: `internal/console/logical.go`, `internal/rms/logicals.go`,
  `internal/rms/copy.go`, `internal/rms/directory.go`, `internal/console/run.go`.
- **What**:
  - `/TRANSLATION_ATTRIBUTES` applies to every equivalence string. In VMS it
    is positional.
  - COPY takes a search list's first element for its source, not the first
    element where the file exists.
  - User-mode names are run down only when RUN's image returns from its main
    routine. govax has no `SYS$EXIT`, so an image ending in HALT or a fatal
    exception isn't run down.
  - (Phase 66) Nor are they run down after govax's own commands that are
    images on VMS (MACRO, LINK, ...): a `DEFINE/USER_MODE SYS$OUTPUT`
    before MACRO is still defined at the next command, where VMS's went
    at MACRO's exit (seen as `%DCL-I-SUPERSEDE` on the next DEFINE when
    `testdata/pg66/pg66.com` ran on govax). And their `$STATUS` on
    success is DCL's own CLI$_NORMAL (%X00030001), where VMS's is the
    image's exit status: DELETE left %X10000001 in VMS 7.1's run of
    `pg66.com`.
  - `$CRELOG`/`$DELLOG`/`$TRNLOG` follow the V4-era interface descriptions.
    The 7.3 source archive doesn't contain them to check against.
- **Status**: open, deferred. None affects an existing test or fixture.
  Revisit case by case if a workload needs the exact VMS behavior.

## Phase 26 (system services) findings

Phase 26 (`PHASE-26.md`) adds VMS system services eVAX never implemented, so
there is no C behavior to preserve; the reference is the VMS 5.0 System
Services Reference Manual. These entries record where govax's version is
knowingly simpler than real VMS, plus the one existing service whose behavior
changed as a result.

### [Phase 26] `$ADJSTK` doesn't probe the new stack segment

- **Where**: `internal/corevms/process.go`'s `serviceSysAdjstk`.
- **What**: the manual returns `SS$_ACCVIO` when "a portion of the new stack
  segment cannot be written by the caller". govax checks only that `newadr`
  can be read and written, not the memory the new stack pointer points at.
- **Status**: open, deliberate simplification.

### [Phase 26] Working-set limits are recorded, not enforced

- **Where**: `internal/corevms/process.go` (`Process.WSLimit`/`WSDefault`/
  `WSQuota`/`WSExtent`/`MinWSCount`, `serviceSysAdjwsl`).
- **What**: `$ADJWSL` adjusts and clamps the limit as documented, but govax
  has no paging or working set, so the limit has no effect on execution. The
  quota values are nominal, not from a real UAF or SYSGEN.
- **Status**: open, by design (the user asked for `$ADJWSL` for
  completeness).

### [Phase 26] `$ALLOC` simplifications

- **Where**: `internal/corevms/devices.go`'s `serviceSysAlloc`.
- **What**: condition values govax never returns, because it has nothing to
  check them against:
  - `SS$_DEVOFFLINE`: devices have no online/offline state.
  - `SS$_NOPRIV`: no privileges or device protection; no spooled devices
    (`ALLSPOOL`).
  - `SS$_TEMPLATEDEV`, `SS$_NONLOCAL`, and the `$ENQ` statuses: no template
    devices, cluster, or lock manager.
  - `SS$_DEVALLOC` for "other processes had channels assigned" to a
    shareable device: there is only one process.

  Also, a generic allocation (`flags` bit 0) matches `devnam` only against
  device-type names (`RA81`), not against a generic device name such as
  `DU:`; and `$ASSIGN` doesn't implicitly allocate a nonshareable device, as
  VMS does.
- **Status**: open, deliberate simplifications.

### [Phase 26] `$ASCEFC` simplifications

- **Where**: `internal/corevms/eventflags.go`'s `serviceSysAscefc`.
- **What**: no `TQELM` quota (`SS$_EXQUOTA`), no multiport shared memory
  (`SS$_EXPORTQUOTA`, `SS$_INTERLOCK`, `SS$_NOSHMBLOCK`,
  `SS$_SHMNOTCNCT`). Cluster names are compared case-sensitively as
  given. (`PRMCEB` was always held until subtask 38 gave the process
  privileges.)
- **Status**: open, deliberate simplifications.

### [Phase 26] `$GETJPI` simplifications

- **Where**: `internal/corevms/getjpi.go`.
- **What**:
  - Only 28 item codes are supported (21 until subtask 21 added the AST
    items, the priorities, and `STATE`; `IMAGNAME` since Phase 66). The
    others, for state govax doesn't model (most quotas, privileges, CPU
    and I/O accounting), are `SS$_BADPARAM`.
  - `JPI$_STATE` is always `SCH$C_CUR`, and the priorities are fixed
    values: there's one process and no scheduler.
  - The AST quota (`JPI$_ASTLM`/`ASTCNT`) is reported but not enforced.
  - (`astadr` is delivered as an AST since subtask 16.)
  - The wildcard context stored at `pidadr` is govax's own value.
  - `SS$_NOPRIV`/`SS$_SUSPENDED` can't happen, since there is one process.
  - `JPI$_WSSIZE` reports the working-set limit, since there is no real
    working set.
- **Status**: open, deliberate simplifications.

### [Phase 26] Event-flag waits re-execute the service instead of blocking

- **Where**: `internal/corevms/eventflags.go` (`$WAITFR`/`$WFLAND`/`$WFLOR`),
  `internal/cpu/xfc.go` (`ErrServiceWait`).
- **What**: an unsatisfied wait re-executes the service's `XFC` on every
  instruction step, instead of descheduling the process. Interrupts are
  still delivered between retries, so a timer interrupt handler can end
  the wait, and so are ASTs (since subtask 15), so an AST interrupts a
  wait and the wait resumes afterwards, as on VMS. Not modeled:
  - the `LEF`/`CEF` process state and `JPI$_EFWM`;
  - releasing the host CPU while waiting.
- **Status**: open, by design.

### [Phase 26] `$SETIMR`/`$CANTIM` simplifications

- **Where**: `internal/corevms/timers.go`, `internal/cpu/systime.go`.
- **What**:
  - The CPU-time flag is treated as elapsed time.
  - There is no `TQELM` quota.
  - (Resolved in subtasks 15-16: `astadr` is now delivered as an AST, and
    timers expire before every instruction rather than only at the next
    event-flag service.)
  - In quantum mode, emulated time is one millisecond per quantum tick
    (20 instructions by default), so a "second" is a fixed amount of
    execution, not wall-clock time. Set `vax.hardware.clock` for real
    time.
- **Status**: open, by design.

### [Phase 26] `$GETTIM`'s clock isn't rounded to 10ms

- **Where**: `internal/corevms/vmstime.go`, `internal/cpu/systime.go`.
- **What**: VMS updates its system time every 10ms, so `$GETTIM` returns
  multiples of 100,000. govax's system time moves in 1ms steps (one per
  interval-clock tick) and is returned as is.
- **Status**: open, by design.

### [Phase 26] `$ASCTIM`/`$BINTIM` details the manual leaves open

- **Where**: `internal/corevms/vmstime.go`.
- **What**:
  - `$ASCTIM` truncates hundredths rather than rounding them.
  - An absolute time past 31-DEC-9999 is `SS$_IVTIME` from `$ASCTIM`
    (it doesn't fit `yyyy`).
  - Bad addresses return `SS$_ACCVIO` instead of raising an access
    violation.
  - The manual's example `--1989 0:0:0.0` → `29-DEC-1989` is taken as a
    typo; govax uses today's day of the month, giving `30-DEC-1989`.
- **Status**: open, by design; revisit if a real VMS disagrees.

### [Phase 26] Hibernation simplifications

- **Where**: `internal/corevms/hibernate.go`, `internal/corevms/timers.go`.
- **What**:
  - `$WAKE`, `$SCHDWK`, and `$CANWAK` can only name the calling process
    (there are no others), so `SS$_NOPRIV` never happens.
  - No `ASTLM` quota (`SS$_EXQUOTA`) or `SS$_INSFMEM`.
  - `$HIBER` waits by re-executing its `XFC`, like the event-flag waits.
    There's no `HIB` process state.
- **Status**: open, by design.

### [Phase 26] AST delivery simplifications

- **Where**: `internal/corevms/ast.go`, `internal/cpu/ast.go`.
- **What**:
  - (Resolved in subtask 22: an inner-mode AST is now delivered to
    outer-mode code by switching into the AST's mode, as on VMS. Until
    then it waited for the CPU to enter that mode.)
  - Delivery is done by the RTL at instruction boundaries, not through
    the `ASTLVL` register, `REI`, and an IPL 2 software interrupt. The
    conditions checked (IPL < 2, not on the interrupt stack, enabled,
    none active) are VMS's.
  - The AST exit is the `SYS$CLRAST` vector entry, reached by the
    routine's `RET` rather than by VMS's own dispatcher code. (Since
    Phase 70's subtask 9 the routine returns first to `EXE$ASTRET`, a
    stub in S0 as VMS's AST return point is, which jumps to that entry,
    so a traceback dump shows a system-space PC for the AST frame.)
  - No `ASTLM` quota (`SS$_EXQUOTA`) or `SS$_INSFMEM`.
- **Status**: open, by design.

### [Phase 26] Terminal `$QIO` simplifications

- **Where**: `internal/corevms/qio.go`, `internal/corevms/ttdriver.go`.
- **What**:
  - ~~Every terminal request completes before `$QIO` returns. A read with
    no input typed yet blocks the whole emulator until the host delivers
    a line.~~ (Mailbox requests can wait, since subtask 29.) Fixed
    2026-10-09 (Phase 49, subtask 10): with the scheduler on, a read
    whose line isn't there stays pending and completes when it's typed;
    with the scheduler off a read still blocks.
  - Only terminals and mailboxes have drivers. A `$QIO` to any other
    device (a disk, say) is `SS$_ILLIOFUNC`, where VMS would perform the
    I/O.
  - Every terminal is the console: reads come from the host's input,
    which is line-buffered and already echoed by the host (or not echoed
    at all, when redirected). So `IO$M_NOECHO`/`TRMNOECHO` change
    nothing, there's no line editing, and a host newline stands for
    RETURN. The end of the host's input is `SS$_ENDOFFILE`.
  - `IO$M_TIMED` with a zero time limit reads only characters already
    buffered; a nonzero limit is ignored (the read waits indefinitely).
  - `IO$M_NOFILTR`, `REFRESH`, `ESCAPE`, `DSABLMBX`, and the write
    modifiers are accepted and have no effect. `IO$_SETMODE` with
    `IO$M_OUTBAND`, `HANGUP`, and the other modifiers succeeds without
    doing anything. (`IO$M_CTRLCAST` and `CTRLYAST` work since subtask
    27; see the CTRL/C entry below.)
  - `IO$_SENSEMODE`'s IOSB reports no line speeds, fill counts, or
    parity.
  - No `BIOLM`/`DIOLM`/`BYTLM`/`ASTLM` quotas (`SS$_EXQUOTA`),
    `SS$_INSFMEM`, `SS$_DEVOFFLINE`, or network functions.
- **Status**: open, by design.

### [Phase 26] Exit handler simplifications

- **Where**: `internal/corevms/exit.go`, `internal/cpu/exit.go`.
- **What**:
  - `$EXIT` calls only the exit handlers of the mode it's called from.
    VMS then runs the supervisor- and executive-mode handlers in their
    own modes during rundown.
  - An image that ends by an unhandled exception doesn't have its exit
    handlers called; the console reports the exception and stops.
  - `$EXIT` ends the image by returning from the console's call frame
    (or halting, without one), not by transferring to a command
    interpreter.
  - eVAX's `$DCLEXH` recorded one handler address and nothing called it;
    that stub is replaced.
- **Status**: open, by design.

### [Phase 26] `$GETSYI` simplifications

- **Where**: `internal/corevms/getsyi.go`.
- **What**:
  - The system is one node outside any cluster: only this node can be
    named, a wildcard scan finds only it, and the cluster items report
    no membership and CSID 0.
  - Only 10 item codes are supported, of the system's identity, boot
    time, and `MINWSCNT`. Other SYSGEN parameters, the `*_EMULATED`
    flags, and hardware model names are `SS$_BADPARAM`.
  - `SYI$_VERSION` reports `V7.3`, the release of the VMS definitions
    govax is built from, though the services follow the VMS 5.0 manual.
  - No `ASTLM` quota (`SS$_EXASTLM`).
- **Status**: open, by design.

### [Phase 26] `$FAO`/`$FAOL` simplifications

- **Where**: `internal/corevms/fao.go`.
- **What**:
  - The directives are the VMS 5.0 manual's, plus the VMS 7 size letters
    `A`, `I`, `H`, and `J` (all a longword on a VAX, which is how the VMS
    7.3 message texts' `!XH` reads) and `Q` (a quadword, by reference).
    VMS 7's `!%C`, `!%E`, and `!%F` (plural choices) are `SS$_BADPARAM`.
  - `!%I` has no rights database: it names only the process's own UIC
    (`[SYSTEM]`), writes other UICs as `!%U` does, and other identifiers
    as `%X` and eight hexadecimal digits.
  - `!%U` writes the group and member in octal without padding
    (`[1,4]`).
  - A `!n<` field whose text is longer than `n` is truncated to `n`.
  - A decimal number too wide for an explicit field fills it with
    asterisks for `!Zx` too (the manual's table gives the rule only for
    signed and unsigned decimal).
- **Status**: open, by design.

### [Phase 26] `$GETMSG`/`$PUTMSG` simplifications

- **Where**: `internal/corevms/message.go`, `internal/vmsdef/gen/msg.go`.
- **What**:
  - Only the CLI, LIB, MTH, OTS, RMS, and SYSTEM facilities of the VMS
    7.3 system message file are known. Images can't carry message
    sections, and there's no process message file or SET MESSAGE: the
    default flags are always all four parts.
  - The texts are VMS 7.3's, which differ in places from the VMS 5.0
    manual's (`SS$_DUPLNAM` is "duplicate name", not "duplicate process
    name").
  - Nine texts were cut at 132 columns in the listing they come from
    (CLI `WRGSUBSHSYN`, `UNTERMSUBSH`, `DUPREDSYN`, `INVCONCHAR`; RMS
    `DELJNS`; SYSTEM `SIG_ARGMISMATCH`, `HPARITH`, `PAGRDERRXM`,
    `ILLEGAL_SHADOW`) and are shown as far as they go.
  - A value without a message gets VMS 7.3's stand-in, `%FAC-S-NOMSG,
    Message number XXXXXXXX`, rather than the VMS 5.0 manual's
    `NONAME` form.
  - `$PUTMSG` writes to the console once; VMS writes to `SYS$ERROR` and
    also `SYS$OUTPUT` when they differ. It doesn't refuse a call from
    kernel mode.
- **Status**: open, by design.

### [Phase 26] `$CMKRNL`/`$CMEXEC` simplifications, and VMINIT's stacks

- **Where**: `internal/corevms/cmode.go`; VMINIT (`internal/console`).
- **What**:
  - ~~The process holds every privilege~~: since subtask 38, a caller in
    supervisor or user mode needs `CMKRNL`/`CMEXEC`.
  - `$CMKRNL` doesn't load R4 with the address of a process control
    block: govax has none.
  - VMINIT's executive and supervisor stacks had the kernel stack's
    protection (only kernel mode could write them), and a caller passing
    0 pages put them at the kernel stack's own address, so a routine run
    by `$CMEXEC` faulted on its first push. Fixed at the user's request:
    each now has its own pages (8 by default), protected `EW`/`SW`, with a
    guard page below; see `docs/MODE-STACKS.md`. They're in S0, not P1
    as on VMS.
- **Status**: the services' simplifications are open, by design; the
  stacks are fixed.

### [Phase 26] `$GETDVI` simplifications

- **Where**: `internal/corevms/getdvi.go`.
- **What**:
  - One node: `FULLDEVNAM` and `ALLDEVNAM` carry its name, the allocation
    class is 0, and no device is remote or served (`SS$_NONLOCAL` can't
    happen).
  - No host, shadow-set, lock-name (`DEVLOCKNAM`), `LOGVOLNAM`,
    `NEXTDEVNAM`, `TRACKS`, `VPROT`, or `ACPTYPE` items (`SS$_BADPARAM`).
  - No secondary devices: `DVI$M_SECONDARY` is ignored.
  - No `ASTLM` quota (`SS$_EXASTLM`).
- **Status**: open, by design.

### [Phase 26] Mailbox simplifications

- **Where**: `internal/corevms/mailbox.go`, `internal/corevms/mbxdriver.go`,
  `internal/lnm/database.go`.
- **What**:
  - ~~One process: a mailbox connects the process with itself (its AST
    routines, its parts). The IOSB's process IDs are always its own.~~
    Fixed in Phase 46, subtask 2 (mailboxes between processes).
  - ~~`LNM$TEMPORARY_MAILBOX` is `LNM$PROCESS`, not `LNM$JOB`: govax has
    no job table.~~ Fixed in Phase 45 (job tables).
  - ~~A write that doesn't fit completes with `SS$_MBFULL`~~ and
    ~~`IO$_SETMODE` has no attention ASTs~~: both fixed in subtask 37
    (resource wait mode, `$SETRWM`, and `IO$M_READATTN`/`WRTATTN`/
    `MB_ROOM_NOTIFY`). ~~`IO$_SETMODE` still makes no protection
    changes; `promsk` is recorded, not enforced.~~ Fixed in Phase 46,
    subtask 3: the mask is checked on `$ASSIGN`, reads, and writes, and
    `IO$M_SETPROT` changes it.
  - A waiting writer doesn't show the RWMBX state in `$GETJPI`'s
    `JPI$_STATE`.
  - No `BYTLM` quota (`SS$_EXBYTLM`), shared-memory mailboxes, or
    termination mailboxes.
  - `$DASSGN` and `$CANCEL` end pending requests with `SS$_CANCEL`;
    whether VMS's mailbox driver says `SS$_ABORT` for requests it holds
    is an open question (`docs/PHASE-26.md`).
  - INIT/VMINIT/ZERO delete every mailbox, permanent ones included.
- **Status**: open, by design.

### [Phase 26] Process-control simplifications

- **Where**: `internal/corevms/process.go`.
- **What**:
  - There is one process, so `$SETPRI`, `$FORCEX`, and `$DELPRC` of any
    other process are `SS$_NONEXPR`, and `$SETPRN` never finds a
    duplicate name (`SS$_DUPLNAM`).
  - `$SETPRI` sets the current priority to the new base priority; there
    is no scheduler to boost it, and the priority changes nothing.
  - `$DELPRC` of the caller ends the image without exit handlers, but
    the process isn't deleted: the console runs the next image in it.
- **Status**: open, by design.

### [Phase 26] CTRL/C and CTRL/Y AST simplifications

- **Where**: `internal/corevms/ctrlast.go`, `internal/cpu/attention.go`,
  `cmd/govax/attention.go`.
- **What**:
  - ~~Only the host's Ctrl-C reaches the program~~ (fixed 2026-10-06: host
    Ctrl-Y is now VMS's CTRL/Y, so it reaches a CTRL/Y AST too).
  - ~~The terminal echoes nothing~~ (fixed 2026-10-06: `*Interrupt*`, as
    simh's VMS echoes for both keys).
  - ~~A key typed while the program is blocked in a terminal read is
    only seen after the read returns~~ (fixed 2026-10-06: the read ends).
    But it ends as end of file even when the program's AST then takes
    the key and the program goes on; on VMS the read would presumably
    wait on (unconfirmed).
  - The requests belong to the process's channels, not to a terminal
    device: every terminal is the console.
  - No `ASTLM` quota.
- **Status**: open, by design.

### [2026-10-06] The control keys (Ctrl/C, Ctrl/Y, Ctrl/Z): choices unconfirmed

- **Where**: `cmd/govax/attention.go`, `cmd/govax/terminal_unix.go`,
  `internal/rms/terminal.go`, `internal/debugger/debugger.go`.
- **What**: the keys follow VMS's meanings (`HELP KEYS`); the author
  confirmed on simh that both CTRL/C and CTRL/Y echo `*Interrupt*` during
  a run, that CTRL/Z echoes `*Exit*` on an empty line, and that CTRL/Z
  after text ends the line with the end of file left for the next read.
  Chosen without a check:
  - CTRL/C at a prompt echoes `*Interrupt*` too, and cancels the line.
  - CTRL/Y ends govax itself (there is no DCL to return to). At the
    console's prompt CTRL/Z only echoes `*Exit*`, as at DCL's (the
    author's call, 2026-10-06); Ctrl-D on an empty line does nothing
    (readline would take it as the end of input and end govax).
  - A Ctrl-C stopping a run the console started returns to the console,
    not DBG> (Phase 42's Decision 2 opened a session): VMS's DCL takes
    the CTRL/C. DEBUG then examines the stopped program. A new RUN
    doesn't run down the interrupted image first.
  - CTRL/Z after text: the record is the text, and the end of file is
    the next `$GET`'s. A `$QIO` read gets CTRL/Z as its terminator in
    both reads (the default terminator set has it).
  - The echoes go where the cursor is, followed by a new line.
  - Without vmsTerminalMode (not macOS or Linux, or input that isn't a
    terminal), only Ctrl-C works, and Ctrl-Y and Ctrl-Z keep the host's
    meanings while a program runs.
- **Status**: open; check the unconfirmed items on simh.


### [Phase 26] Condition dispatch simplifications

- **Where**: `internal/corevms/condition.go`, `internal/cpu/handlefault.go`.
- **What**:
  - Only exceptions kernel.asm's SCB sends to `console$handler` are
    dispatched (access violation, translation not valid, privileged and
    reserved instructions, reserved operand and addressing mode,
    arithmetic). Breakpoint, trace, and compatibility-mode exceptions
    aren't signaled.
  - ~~Every exception is still delivered as a fault: the signal array's
    PC is the faulting instruction's, even for the arithmetic *traps*
    (types 1-7), whose architected PC is the next instruction's.~~
    Resolved: the arithmetic traps save the next instruction's PC
    (`cpu.Engine.arithmeticTrap`), INDEX's subscript range trap since
    Phase 70 (see "[Phase 70] Integer overflow and divide-by-zero never
    trapped").
  - An exception on the interrupt stack, or one whose stack can't be
    written, goes to the console's report instead of VMS's fatal-error
    handling.
  - The catch-all prints no traceback, and writes its message to the
    console terminal (VMS's goes to `SYS$ERROR` and `SYS$OUTPUT`).
  - On VMS the kernel dispatches an exception in the mode it happened
    in only after checking that mode's stack; govax pushes the arrays
    on whatever stack is current.
  - `LIB$SIGNAL`/`LIB$STOP` (subtask 33) put 0 in the mechanism array's
    R0, where VMS puts the caller's R0: the shim stub's
    `MOVL #code, R0` has replaced it before the shim runs. So a handler
    that continues without setting it makes `LIB$SIGNAL` return 0.
  - `$UNWIND` (subtask 34) from a vectored handler with no `depadr`
    does nothing: the default depth is the establisher's plus one, and a
    vectored handler has no establisher frame. `newpc` is taken as the
    resume address itself.
- **Status**: open, by design.

### [Phase 26] Privilege simplifications

- **Where**: `internal/corevms/privilege.go` and the services that check.
- **What**:
  - No UIC-based object protection: VMS lets a process write a logical
    name table (or delete a cluster, a mailbox, ...) the object's
    protection grants it, whatever its privileges, and a system-UIC
    process such as SYSTEM gets system access to most objects. govax
    checks only the privileges, so disabling SYSNAM stops SYSTEM writing
    the system table where VMS wouldn't.
  - `$DELLNM`'s table check looks at the first table `tabnam` resolves
    to, not the table the name is eventually found in.
  - No installed images: `IMAGPRIV` is always empty.
  - Services govax doesn't have (`$CREPRC`'s DETACH, `$MOUNT`'s MOUNT,
    ...) and devices (LOG_IO, PHY_IO, SHARE) check nothing.
- **Status**: open, by design.

### [Phase 26] Operator and broadcast simplifications

- **Where**: `internal/corevms/operator.go`.
- **What**:
  - One operator terminal, the console. `$SNDOPR`'s `OPC$T_MS_ONAME`
    terminal names are ignored; there's no operator log file and no
    REPLY command, so a request is only answered by a program's own
    `OPC$_RQ_REPLY` or by `OPC$_NOPERATOR`.
  - The `$OPCMSG` reply statuses other than `OPC$_NOPERATOR` aren't in
    the reference set: a cancelled request gets no mailbox reply
    (`OPC$_RQSTCAN` on VMS). The request codes other than
    `OPC$_RQ_RQST` are inferred from OPCOM's dispatch order. OPCOM's
    texts for cancel, enable, status, and log file follow the manual's
    examples where it has them.
  - The newer request formats (`OPC$_X_...`) and security requests
    aren't handled (`SS$_BADPARAM`).
  - `$BRKTHRU` writes once to the console for any number of terminals;
    screen formatting (`BRK$M_SCREEN`, ...), `timout`, `reqid`
    filtering (SET BROADCAST), and cluster broadcasts don't apply.
- **Status**: open, by design.

### [Phase 26] Rights database simplifications

- **Where**: `internal/corevms/rights.go`.
- **What**: no RIGHTSLIST.DAT: the database is the process's own UIC
  identifier and the six environmental identifiers, built in memory, so
  there are no other users' identifiers, no group identifiers, no
  attributes, no holder records (`$FIND_HELD`, `$ADD_IDENT`, ... aren't
  implemented), and no wildcard names in `$ASCTOID`. The environmental
  identifiers' values are the ones AUTHORIZE assigns, not read from a
  database. The process's rights list isn't kept (no access control
  lists use it).
- **Status**: open, by design.

### [Phase 26] Disk `$QIO` simplifications

- **Where**: `internal/corevms/diskdriver.go`, `internal/rms/acp.go`.
- **What**:
  - Only the file-level (ACP) functions: `IO$_ACCESS` (lookup, and
    access with `IO$M_ACCESS`), `IO$_DEACCESS`, `IO$_MODIFY` (extension
    only), `IO$_READVBLK`, `IO$_WRITEVBLK`. No creating or deleting files
    through the ACP (`IO$M_CREATE`, `IO$M_DELETE`: `SS$_ILLIOFUNC`), no
    attribute lists (`p5` is ignored), no wildcard lookups, no logical or
    physical block I/O (`IO$_READLBLK`, ...), no directory-entry
    changes.
  - At deaccess, the end of file moves to just past the highest block
    written, if that's beyond it (ods2's `File.Close`); VMS's ACP leaves
    the end of file to the attributes a program (RMS) writes.
  - No file sharing or interlocks between channels (`FIB$M_NOWRITE`,
    ...); every access is granted if the mount allows it.
  - Every request completes during the call; disk errors are
    `SS$_DRVERR`.
- **Status**: open, by design.

### [Phase 26] Virtual address space simplifications

- **Where**: `internal/corevms/vaspace.go`, `regions.go`, `pgflquota.go`,
  `pageprot.go`, `internal/machine/vminit.go`.
- **What** (revised 2026-10-22: Phases 63 to 67 changed much of this
  with paging on, `vax.sysgen.paging.enable`):
  - With paging off, the page tables are built once by VMINIT, every
    page in them exists from the start, and the length registers never
    move: `$CRETVA` beyond the tables is `SS$_VASFULL`, where VMS would
    extend the region. With paging on (Phase 67, created space) the
    lengths describe the pages that exist and the creating services
    lengthen the regions, within the process's table area
    (`vax.sysgen.virtualpagecnt`); `SS$_VASFULL` is a page past the
    area, or a P1 range it can't hold (refused before any page is made).
  - The paging file quota (`SS$_EXQUOTA`) is checked only with paging
    on (Phase 64, `pgflquota.go`). The working-set limit `SS$_INSFWSL`
    is never returned: with paging on, physical memory with no frame for
    a page-table page the region needs is `SS$_VASFULL` (govax's choice;
    VMS's tables are in the process header, charged to the working set).
  - With paging off, `$CNTREG` deletes the pages but leaves
    `P0LR`/`P1LR` alone; the pages are gone either way. With paging on
    it shortens the region (`contractRegions`). Its P1 end is the lowest
    page `P1LR` admits, the page `P1LR` names (since the length checks
    were fixed, Phase 43; before, the page after it).
  - `$SETPRT` never charges the paging file quota, in either mode, and
    has no rules of its own for section pages (a writable section page
    made read-only or the reverse); sections exist since Phases 46 and
    49, so these cases can now arise.
  - Page locking: with paging on (Phase 63) a locked page is faulted in
    and pinned in the working set list, and `SS$_LKWSETFUL` comes from
    the list; with paging off nothing pages, so the services only
    remember which pages are locked (for `SS$_WASSET`/`WASCLR`) and
    `SS$_LKWSETFUL` is counted from those records against MINWSCNT
    (unconfirmed). In both modes `$LCKPAG` has no system limit and never
    returns `SS$_LCKPAGFUL` (Phase 63's finding 7), and image rundown
    unlocks every page, not just the image's user-mode locks.
- **Status**: open, by design (paging off keeps the simpler model).

## Phase 27 (MACRO-32 object modules) findings

Phase 27 (`PHASE-27.md`) adds a `MACRO` command that writes VAX object
modules. eVAX had no object output, so there is no C behavior to
preserve. The references are the VAX MACRO manual, the VMS 5.0 Linker
manual's object language chapter, and objects real VAX MACRO V5.4-3
wrote from the fixtures in `testdata/mar/`. All twelve fixtures match
real MACRO record for record (traceback records too, since Phase 29),
and VMS 7.3 accepts govax's objects (`ANALYZE/OBJECT`, `LINK`, `RUN`).
These entries record what govax knowingly leaves out, and the encoding
choices no fixture has confirmed yet.

### [Phase 27] Smaller MACRO simplifications and extensions

- **Where**: `internal/asm/macrodir.go`, `internal/dclcmd/macro.go`,
  `internal/rms/copy.go`.
- **What**:
  - `.ENABLE TRUNCATION` and `VECTOR` aren't supported: enabling one is
    the warning `VAX_IGNORED`, and assembly goes on.
  - The `MACRO` command has no `A+B` concatenation of several sources
    into one module, nor VMS's `PROG+LIB/LIBRARY`; macro libraries come
    from a govax-style `/LIBRARY=(file,...)` (Phase 28). (`/LIST`,
    `/SHOW`, `/CROSS_REFERENCE`, `/DEBUG`, `/ENABLE`, and `/DISABLE`
    came with Phase 29.)
  - Error messages are govax's (`%CLI-E-ASSEMBLING, ... line N:
    VAX-E-...`), not MACRO's. After an error, assembly goes on to the
    next statement, so a later message can follow from an earlier
    error, as in real MACRO. At the end, only the first undefined
    symbol is reported.
  - `.INCLUDE` isn't MACRO-32. It's kept as an extension, and resolves
    names across host and ODS-2 files.
  - The object's language processor header is `govax MACRO V<build>`,
    and its SRC header is the command line as typed, as real MACRO
    records its own.
  - A host `.OBJ` is kept in ODS-2's on-disk variable-length record
    layout, and COPY always copies `.OBJ` files (and the other types in
    `internal/rms/recordfile.go`'s `hostRecordTypes`) as records. Host
    files have no record attributes, so that's govax's convention, not
    VMS behavior.
- **Status**: open, by design.

### [Phase 27] Object encoding choices no fixture confirms

- **Where**: `internal/asm/object.go`, `operand.go`, `value.go`.
- **What**: each follows the confirmed cases by analogy:
  - A byte displacement-mode field left to the linker is `STO_SB`. The
    word form, `STO_SW`, is confirmed.
  - A binary operation on constants within an expression left to the
    linker isn't folded (`EXT+<2*3>` stacks 2, 3, `OPR_MUL`). Only unary
    minus is confirmed (`EXT+<-4>`).
  - A constant whose value is negative (`EXT+^XFFFFFFFC`) is stacked
    with the signed short forms `STA_SB`/`STA_SW`.
  - `. =` backward is a negative `CTL_AUGRB`. Forward is confirmed.
  - The location in `.  ABS  .` is set at the start of assembly unless
    code, data, or a label before any `.PSECT` moves assembly into
    `. BLANK .` first, and again on a return there (confirmed in Phase
    28: `$RMSDEF`'s `.RESTORE`). An unnamed `.PSECT` first, or an
    absolute psect first, isn't covered by a fixture.
- **Status**: open. Settle each with a fixture when a program needs it;
  VMS links either form to the same image.

### [Phase 28] Macro facility and librarian: what govax leaves out or does differently

- **Where**: `internal/asm` (macros, `maclib.go`, `message.go`),
  `internal/lbr` (`write.go`, `input.go`), `internal/dclcmd` (`macro.go`,
  `library.go`), `internal/bootdata/files/starlet.mar`.
- **What**:
  - (govax's own STARLET.MLB, which this entry listed as small, grew in
    Phase 32 and after: `starlet.mar`'s system service and RMS macros,
    `$FAB` and `$RAB` among them, and `starletdef.mar`'s `$xxxDEF`
    macros. A system macro it still lacks needs VMS's, through
    `SYS$LIBRARY` or the `vax.library` setting.)
  - **A `;` inside `<...>`** in a macro call's arguments still ends the
    statement (preprocessing sees it as a comment), so `<A;B>` is
    `VAX_NOCLOSE`. Whether real MACRO takes the `;` as part of the
    argument isn't confirmed by a fixture.
  - **`.ERROR` and `.WARN` messages** show the comment as written (a
    leading blank, a closing `;`), by analogy with `.PRINT`, whose text
    real MACRO's log confirms. Real MACRO's own `.ERROR`/`.WARN` output
    isn't in a fixture yet; govax's prefix is its own (`VAX-E-GENERR,
    Generated ERROR:`).
  - **An overwritten constant in an object** (a field stored twice, as
    `$FAB` stores FNA and FNS) is written as each store left it, from the
    byte's history; a byte a fixup finishes after being stored over is
    written from the image, which holds the later store's value. No
    fixture has that case, and the image links the same either way.
  - **The console dialect (`ASM`)** defines and calls macros and repeat
    blocks, but searches no macro libraries (the scope's "only if free").
  - **LIBRARY builds the whole library in memory** and writes it once
    every step has succeeded; a changed volume library is rewritten at
    its own version but gets a new file ID and creation date (LIBRARIAN
    updates in place). A new library is allocated only the blocks it
    uses, where LIBRARIAN allocates 100, contiguous best try. No DCX data
    reduction on writing, and no update history records.
  - **Index trees are built bottom-up** from the sorted keys, so a
    library is laid out as LIBRARIAN lays out one created in a single
    command (byte for byte on the fixtures). A library LIBRARIAN grew by
    many separate inserts may have its index blocks split differently;
    govax reads either.
- **Status**: open, by design, except the `;` and `.ERROR`/`.WARN`
  items, which a fixture should settle when a program needs them.

### [Phase 32] RMS store macros: choices real MACRO's output didn't settle

- **Where**: `internal/bootdata/files/starlet.mar`'s `$xxx_STORE` macros,
  written from the RMS Reference Manual and the Phase 32 oracle's objects
  (docs/RMS-MACROS.md).
- **What**: the first oracle round left three choices resting on little
  evidence:
  - *the order of stores in a call with several keywords*, from one
    example;
  - *`$NAM_STORE DVI=` given a register*, never tried;
  - *`$XABPRO_STORE PRO=` and `UIC=` with one element*, never tried.

  govax had guessed: keyword fields, then addresses, then values; DVI=
  always an error; and a list told from an address by its comma.
- **Status**: fixed in Go, after the second oracle round (2026-10-01,
  testdata/mar/rms/r2_*):
  - *Order.* Each store macro has a fixed order of its own, whatever order
    the call gives (both orders of every keyword assemble the same). The
    orders weren't the guessed grouping: `$FAB_STORE` starts FAC, SHR, FNA,
    FNS, DNA, DNS, for example. govax now uses real MACRO's order for every
    macro, and the nine probes are byte-identical.
  - *DVI=*. Every form is an error in real MACRO: a register and an
    immediate give govax's message, and `(R2)` an operand syntax error.
    Kept as it was.
  - *PRO=/UIC=*. PRO= is a protection list when it holds only R, W, E, D,
    and commas. So `PRO=<R>` is ^XFFFE, and `PRO=RW` is RW protection
    rather than the word at RW; any other text is an address. UIC= is a
    list only with a comma (`UIC=<377>` is the address 377). In the
    `$XABPRO` initialization macro, a UIC= that isn't two elements is
    "INVALID UIC_FIELD". govax now follows all of these.

### [Phase 32] `$RENAME` with no arguments is an error in real MACRO

- **Where**: `internal/bootdata/files/starlet.mar`'s `$RENAME`, against the
  Phase 32 oracle's `services.mar` (testdata/mar/rms/vax/RMS.LOG).
- **What**: called with no arguments, real MACRO's `$RENAME` emits
  `CALLG (AP),G^SYS$RENAME`, as the manual says a service macro does, and
  then reports `%MACRO-E-UNRECSTMT, Unrecognized statement` at the
  location after it. Its object is otherwise the same as with any other
  service, but its severity is ERROR. govax's `$RENAME` emits the CALLG
  and nothing else, as every other service macro does.
- **Status**: kept, deliberately: the manual documents the call as valid,
  and a program using it gets the same code. `TestOracleObjects` treats the
  services probe's object severity as SUCCESS for this.

### [Phase 34] DIRECTORY/OWNER shows a UIC, not an identifier name

- **Where**: `internal/rms/directory.go`, against `CREDIR.LOG`'s
  `DIRECTORY/OWNER/PROTECTION` listing.
- **What**: VMS shows an owner with an identifier name by the name
  (`[SYSTEM]` for `[1,4]`) and lays the columns out its own way; govax
  shows `[1,4]`, in its own one-file-per-line layout (docs/PHASE-23.md's
  open question). The protection column, `(RWE,RWE,RE,E)`, is VMS's.
- **Status**: deferred: govax has no rights database to name UICs from.

### [Phase 34] LIB$CREATE_DIR's relative directory got an entry limit of 1

- **Where**: `internal/librtl/createdir.go`, against the LIB$CREATE_DIR
  probe (`testdata/credir/libcrd.mar`, both of VMS 7.3's runs; it recurred).
- **What**: `LIB$CREATE_DIR("[.SUBREL]")`, with `[000000]` the default
  directory, made `[SUBREL]` with an MFD entry whose version limit is 1.
  Every directory made from an absolute specification got an entry with
  no limit, as did those `CREATE/DIRECTORY` made relative to a default
  (`[.SUB]`, `[-.SIBLING]`). The directory file itself (its default limit
  for files) is the same either way. Neither the LIB$ manual nor the DCL
  dictionary explains it; 1 is the limit on the MFD's own entry,
  `000000.DIR;1`, which may be where it comes from.
- **Status**: deferred: govax writes no limit. `TestLibCreateDirOracle`
  masks the field for `[SUBREL]`.

### [2026-10-06] A FAB asking for no access, and LIB$GET_INPUT: choices unconfirmed

- **Where**: `internal/rms/fab.go` (`facAccess`), `open.go`, `create.go`,
  `connect.go`; `internal/librtl/input.go`.
- **What**: `$OPEN` and `$CREATE` used to refuse a FAB whose `FAB$B_FAC`
  asks for no access (a `$FAB` with no `FAC=`) with RMS$_PRV, so a
  program opening `SYS$INPUT` that way read nothing. Now, as the RMS
  Reference describes FAB$B_FAC, `$OPEN` implies GET and `$CREATE` PUT;
  `$CONNECT` of such a FAB arms the file for the access it was opened
  with (`FileHandle.Access`). Chosen without a check on VMS:
  - The FAB's FAC is left 0 (VMS may write the implied bit back).
  - `$CREATE` of a FAB asking for access but not PUT (GET alone) is
    still refused with RMS$_PRV.
  - LIB$GET_INPUT leaves get-string and resultant-length alone at end of
    file, and reads the terminal even if SYS$INPUT is redefined to a
    file.
- **Status**: open; check on VMS.

### [Phase 35] An octaword in R12 or above

- **Where**: `internal/cpu/operand.go`'s register-mode decode.
- **What**: an octaword (or H_floating) register operand spans Rn through
  Rn+3. The manual calls one starting in R12 or above UNPREDICTABLE, since
  it would reach the PC. Real hardware may do anything.
- **Status**: a deliberate choice (Phase 35 Decision 4): govax raises a
  reserved-addressing-mode fault at decode, for a source or a destination.
  The Phase 35 oracle may show what VMS 7.3's MicroVAX does.
  `TestOctawordRegisterLimit`.

### [Phase 35] DIVP with an invalid divisor digit

- **Where**: `internal/cpu/decimalmath.go`'s `emulDivp`, and the packed
  decimal core's treatment of an invalid digit nibble (`decimal.go`).
- **What**: the manual makes the quotient UNPREDICTABLE when the divisor
  or dividend has an invalid nibble (^XA-^XF). govax uses a nibble's
  value, as VMS 7.1 on a VAX 8600 did for CVTPL, CMPP, and ADDP
  (testdata/insn35), so the probe's DIVP of 100 by the one-digit "E"
  gives 100/14 = 7. VMS gave 12, as if it had divided by 8; its divide
  evidently reads an invalid digit some other way than its add does.
- **Status**: deliberate (2026-10-02, Phase 35 subtask 11): the result is
  UNPREDICTABLE by definition, and modelling one machine's microcode for
  it would mean guessing from one sample. The oracle test masks this
  case.

### [Phase 35] EDITPC's aborts don't set PSL<FPD>

- **Where**: `internal/cpu/editpc.go`.
- **What**: on a reserved operand abort part way through (a reserved
  pattern operator, too few or too many source digits), the manual
  (EDITPC's note 11) sets PSL<FPD> and leaves its part-way state in R0-R5,
  so a handler can fix the pattern and continue; VMS's signal arrays show
  FPD set. govax doesn't model FPD for any instruction (the plan's
  decision), so it leaves FPD and the registers as they were.
- **Status**: deliberate (2026-10-02, Phase 35 subtask 13). The output
  produced before the abort does stay in the destination, as on VMS.
  `TestInsn35Oracle` masks the FPD bit in these three cases.

### [Phase 42] The debugger's known differences from the VMS 7.3 debugger

- **Where**: `internal/debugger`; checked by `TestDebuggerSessionOracle`
  (`session_oracle_test.go`), whose `expectedDifferences` is the
  authoritative list, each entry with its reason.
- **What**: the commands that still print something other than VMS's log
  (docs/PHASE-42.md, subtask 16): (1) a `WHEN` condition's `.LABEL` is the
  contents at the label, where VMS reads the contents at the address the
  label holds (`condition.go`); (2) at a break on an unhandled exception
  the PC is in the condition dispatcher, not at the fault, so `.PC` and
  `SHOW CALLS` there differ, and STEP from such a break runs the paused
  dispatcher; (3) `CALL` takes govax's argument forms and prints no
  "value returned is" (`%VAL`/`%REF`/`%DESCR` are Future features);
  (4) `EXAMINE` refuses two type qualifiers where VMS takes the last;
  (5) a `%LINE` with no code is an undefined name, not `%DEBUG-E-LINEINFO`;
  (6) a data address past the last label is named by that label, where VMS
  names it by the global constant before it (`GLIMIT+25D`);
  (7) `SHOW CALLS` in a condition handler lacks the "above condition
  handler called" lines; (8) `SET MODE NOLINE` and the other display modes
  are recorded and shown but don't change the output; (9) the `ACCVIO`
  message's text is `PS=` and mask `02` where VMS 7.3's run prints `PSL=`
  and `00` (the text is generated from VMS's definition files);
  (10) SHOW's sizes and image lists (VMS's own tables and images), and
  addresses on VMS's stack and heap, are of course not govax's.
- **Status**: deferred (each is logged in the phase doc and in the test);
  (9) is a message-table question for a later phase.

### [Phase 42] Debugger rules chosen without a probe or a manual to settle them

- **Where**: `internal/debugger`, as listed in docs/PHASE-42.md's progress
  log under "Unconfirmed".
- **What**: where neither the *VMS 5.5 Debugger Manual* nor a probe
  session said, govax made a choice and logged it. The ones a later probe
  should check: the order of `SHOW BREAK` beyond one list of addresses and
  the report order of tracepoints that share a PC (each reported moves to
  the end of the list); `/AFTER` and `WHEN` on tracepoints and watchpoints;
  `CANCEL TRACE`/`WATCH` of nothing saying `NOTRACES`/`NOWATCHES`;
  STEP/RETURN's pending frame (a second one replaces it, GO fires it, an
  unwind or image exit drops it) and STEP/OVER interrupted by a break
  ending; a line step ending at a RET in the middle of the caller's line;
  the instruction text added to exception-break locations only under
  `SET STEP INSTRUCTION`; `EXAMINE` with no location continuing from the
  last one, and an entry mask's next location being 2 bytes on;
  `/OPERANDS=FULL` equal to brief, and the layout of operands that name
  no register or memory; `SHOW SYMBOL` with both `/ADDRESS` and `/TYPE`
  and multi-dimension array bounds; `SHOW SCOPE`'s numbering past two
  levels of one routine; `SHOW LANGUAGE`'s `UNKNOWN`; `SET SOURCE`/`SHOW
  SOURCE` wording and `%DEBUG-E-NOSUCHMODULE`; a source record over 144
  columns; `EXIT` and `QUIT` being alike.
- **Status**: deferred, for a later simh round.

### [Phase 42] The console's `SET name=value` still assigns a register

- **Where**: `internal/console/set.go` (`SetSymbolQualified`).
- **What**: the plan moved register assignment to the debugger
  (`DEPOSIT R0 = 5`). The console's `SET R=5` still assigns, since the
  assignment form is shared with symbols and an assignment wins over a
  keyword it abbreviates; taking registers out would add a special case.
- **Status**: deliberate (2026-10-05, Phase 42 subtask 14).

### [Phase 47] File sharing and lock rules chosen without a manual or probe

- **Where**: `internal/rms/sharing.go`, `recordlock.go`, `erase.go`;
  `internal/lck`; `internal/corevms/enq.go`, `locks.go`; ods2's
  `volume/access.go`.
- **What**: the *Guide to OpenVMS File Applications* (VMS 7.3; read
  2026-10-08, after Phase 47 was first closed) settled most of what the
  RMS and System Services manuals left open: file arbitration per record
  operation (Tables 7-3 and 7-4: write access implies read, write
  sharing implies read sharing), exclusive record locks by default for
  every stream, the RLK/REA/NLK compatibilities (Table 7-6), an error
  unlocking the stream's record unless ULK, RMS$_OK_WAT after a wait,
  RMS$_OK_ALK, RRL's RMS$_SUC, and shared sequential files written to the
  disk on each change. govax follows it (see the phase doc's log). What
  is still govax's choice:
  - FAB$V_UPI shares every operation. And with UPI the guide says RMS
    keeps neither buffers nor the end of file coherent between openers
    (readers see appends only by closing and opening again); govax's
    shared FCB keeps them coherent anyway.
  - The lock manager queues a new lock behind anything queued, and a
    conversion that isn't down behind queued conversions (FIFO); a
    down conversion is granted at once. A converting lock gets blocking
    notices for its granted mode.
  - A PW or EX lock still held at process deletion invalidates its value
    block, whether the process exited or was deleted; image rundown's
    dequeue doesn't. SS$_NOTQUEUED is written to the LKSB too. A grant
    boosts as "resource available".
  - Record-lock deadlocks: the guide has the lock manager deny one
    request after DEADLOCK_WAIT (RMS$_DEADLOCK with RAB$V_WAT); govax has
    no deadlock detection, so they wait forever.
  - `$ERASE` of a file another process writes is RMS$_FLK (no
    FAB$V_ERL in VMS 7.3's definitions); an open FAB is RMS$_IFI.
  - DISMOUNT with files open is refused (VMS marks the volume for
    dismount instead).
  - Host files get no arbitration.
- **Status**: unconfirmed; candidates for a VMS 7.3 probe round.

### [Phases 43–48] Multiprocessing rules chosen without a manual or probe

- **Where**: `internal/sched`; `internal/corevms` (`proctable.go`,
  `schedule.go`, `waits.go`, `creprc.go`, `startup.go`, `delete.go`,
  `termmsg.go`, `mbxdriver.go`, `gblsec.go`, `terminal.go`, `spawn.go`,
  `cliprocess.go`, `outfile.go`); `internal/console/subcli.go`,
  `spawncmd.go`; `internal/cpu/context.go`.
- **What**: the multiprocessing program followed DIGITAL's manuals, the
  book *VAX/VMS Internals and Data Structures* (Decision 6), and the VMS
  runs of `testdata/mp` (probes 1 to 3, the ping-pong pair). Where none
  of them settled a rule, govax chose one. This is the list in one place;
  each phase's doc says more where the rule is described (search it for
  "unconfirmed"), and Phase 47's file and lock rules are the entry above.
  - **Phase 43:** the PID's layout (an 8-bit slot index and a 13-bit
    sequence, every slot starting at sequence 2, so process 2 is
    00000302); the IPL (3) and interrupt-style PSL a context switch
    leaves behind.
  - **Phase 44:** that VMS 7.3 numbers MWAIT resources as the book's VMS
    3.3 does; the boost classes of events whose reporter govax doesn't
    track (a wakeup or resource: 3; an event flag or `$QIOW`: 2);
    `$SETPRI`'s "Set Priority" boost not applied; SHOW SYSTEM's and SHOW
    PROCESS's layouts where no sample showed them (a short node name's
    padding, uptime past 9 days, SHOW PROCESS from memory).
  - **Phase 45:** a child's priority at creation, its working-set size,
    SYSTEM's AST limit, the PQL defaults and minimums; what a subprocess
    inherits besides its default directory and SYS$DISK (nothing); the
    privileges and authorized mask for an omitted prvadr; PID reuse; the
    termination statuses of a startup that fails (RMS$_FNF, SS$_ABORT);
    `$SETPRI`'s rules for real-time priorities across processes; the
    services' argument-count minimums beyond those probed.
  - **Phase 46:** the boost classes of a terminal set/sense mode, a
    mailbox attention AST, `$SETEF`, and a CTRL/C or CTRL/Y AST; that
    either access suffices for a mailbox `$ASSIGN`; SETPROT's argument;
    `$DGBLSC` not checking protection; no logical-name translation of a
    section name; which rule makes a file section with no channel
    SS$_IVSECFLG; FAB$W_MRS after `$OPEN` of a mailbox; a terminal read
    that must wait making the `$QIO` itself wait (fixed 2026-10-09,
    Phase 49: the read is left pending, as on VMS).
  - **Phase 48:** `testdata/mp/probe4`'s VMS run (2026-10-08) confirmed
    the default process name, the event flag's clearing, which process
    logical names and symbols a subprocess gets, the CLI's statuses
    (and that it goes on after a failing command), EXIT's, and DCL's
    messages, and govax now matches it in full (`TestProbe4`). Still
    chosen: the access modes the copied names keep (their own); the
    order of the completion's effects (status, event flag, AST); the
    verb abbreviations (`R` for RUN, `MC`, `LO`, `EXI`, `SH`, `SYM`,
    `LOG`); LIB$SPAWN's flags NOTIFY, NOCONTROL, NOKEYPAD, TRUSTED,
    AUTHPRIV, and SUBSYSTEM, and its cli and table-name arguments, doing
    nothing; a SYS$OUTPUT file written through whole at each line;
    STS$M_INHIB_MSG on every activation failure's status (VMS showed it
    for an image not found); SHOW LOGICAL's status when a name has no
    translation; an interactive LOGINOUT job's one-line LOGOUT message;
    LOGOUT's report with zero I/O counts, page faults, and peaks (govax
    keeps none, as in the termination message). The console keeps no
    `$STATUS` symbol for SPAWN to set.
  - **The close-out** (2026-10-08): TTA0:'s DEVCHAR (0C040007); SHOW
    DEVICE/FULL naming only [1,4] by identifier (`[SYSTEM]`), its
    sentence wrapped a word at a time within 80 columns, and a terminal's
    protection always VMS 7.1's `S:RWPL,O:RWPL,G,W`; an operation counted
    for every `$QIO` that completes, whatever its status; the debugger's
    SHOW PROCESS layout (govax's own).
  - **Settled by `testdata/mp/final`** (2026-10-08) and now govax's:
    a child's working set 4 pages above the quota rules'; SYSTEM's AST
    limit 50; PID slots taken in rotation; a failed `$ENQ` leaving the
    LKSB alone; FAB$W_MRS 0 after a mailbox `$OPEN`; three argument
    minimums; `$ENQ`'s PRIORITY and `$GETLKI`'s RESERVED keywords; the
    subprocess CLI's logout line, NOTRAN status, and RUN of a non-image;
    SHOW LOGICAL/FULL's table header; SHOW DEVICE's brief layout and
    NOSUCHDEV. Confirmed as govax had them: inheritance, copied names
    and their modes, value blocks after a holder's deletion, `$ERASE` of
    an open file, NLA0:'s characteristics.
  - **Still open after it:** a child's current priority at creation (VMS
    7.1 saw base+2, 7.3 base+3; the book's boost of 6 fits neither;
    govax: the base); `$ENQ`'s twelfth argument's keyword (ARG12);
    the brief SHOW DEVICE's disk columns (from memory).
  - **Statuses of an all-zero call** (`p5args`, 2026-10-08): every
    argument minimum now matches VMS 7.3's, but with every argument 0
    some services report a different error first: VMS gave SS$_ACCVIO
    for `$CRELOG`, `$CRMPSC`, `$DGBLSC`, `$MGBLSC`, `$GETJPI`, and
    `$TRNLOG` (govax: SS$_BADPARAM, the section services' own statuses,
    or, for `$GETJPI`, success), and SS$_BADPARAM for `$GETSYI` and
    `$SNDOPR` (govax: SS$_ACCVIO). Which argument VMS checked first
    can't be told from an all-zero call, so govax's order stands.
  - **Not modeled:** services that run in the caller's mode (`$ASCTIM`,
    `$BINTIM`, `$FAO`, `$FAOL`, `$NUMTIM`) fault on a bad address on
    VMS, which signals an access violation; govax's return SS$_ACCVIO.
    MCR runs an image from SYS$SYSTEM on VMS; govax has no SYS$SYSTEM,
    and its CLI's MCR runs the name as RUN would.
- **Status**: unconfirmed but for what the VMS runs settled (probes 1
  to 5, the ping-pong pair, the milestone, macro rounds 1 to 7); the
  rest are candidates for later probes.

### [Phase 49] Rules chosen without a manual or probe

- **Where**: `internal/rms` (`stream.go`, `recordops.go`, `recordlock.go`,
  `ufo.go`, `iocount.go`, `hostshare.go`, `search.go`); `internal/lck`
  (`deadlock.go`, `info.go`); `internal/corevms` (`getlki.go`,
  `lockquota.go`, `filesec.go`, `gblsec.go`, `terminal.go`,
  `iocount.go`, `file.go`).
- **What**: Phase 49 followed the RMS Reference Manual, the File
  Applications guide, the System Services manual, and the Internals
  book, and its probe's three VMS 7.3 runs (`testdata/probe49`, every
  answer adopted; govax's reports now differ from VMS's only in the
  environment). Still chosen without either:
  - **RMS:** DEL access letting a stream `$GET` (UPD's and TRN's
    confirmed); `$FIND` on the terminal or a mailbox reading a record
    and dropping it, `$REWIND` there doing nothing; a stream `$UPDATE`
    that would run past the end of the file being RMS$_RSZ; a failed
    `$GET` for RMS$_RLK leaving RAB$W_RFA (RMS$_TMO's clears it, as
    VMS's did); RAB$V_NODLCKWT and NODLCKBLK ignored.
  - **Locks:** an ASTLM shortage on `$ENQ` being SS$_EXQUOTA (not
    SS$_EXASTLM), an armed blocking AST not counting against ASTLM, and
    only the lock services enforcing ASTLM; the deadlock victim being
    the request the search started from (every govax process's deadlock
    priority is 0); no deadlock search with the scheduler off;
    DEADLOCK_WAIT fixed at 10 seconds.
  - **Sections:** `$UPDSEC` passing over pages a more privileged mode
    owns, and its IOSB's second longword when every page was written
    (the page past the range); the `$UPDSEC` macros' argument pairing
    (not checked against real MACRO); the page fault cluster ignored;
    page-file sections taking their frames at creation; P1's expansion
    region below a fixed 1,024-page user stack reserve, which a deeper
    stack would run into.
  - **Terminal:** IO$M_TIMED with a nonzero limit waits without one;
    IO$M_PURGE discards what is typed ahead even when reads queued
    before it would have read it.
  - **I/O counts:** RMS's are a model fitted to two VMS totals
    (`rms/iocount.go`: the file system's calls buffered, one direct I/O
    per 16-block buffer, `$CREATE` 2 and 5); how VMS's grow with file
    size, buffer counts, and other services is unknown.
  - **Host files:** EXE$OPEN's arbitration is govax's own (VMS has no
    such shim); it uses RMS's rule, a read-only open as FAB$M_GET and
    any other as a writer, with RMS's default sharing.
- **Status**: unconfirmed; candidates for a later probe.

### [Phase 50] Command procedure rules chosen without a manual or probe

- **Where**: `internal/console/procedure.go`; `internal/vmserrors/codes_cli.go`
  (CLI_DEFOVF, CLI_STKOVF, CLI_SKPDAT).
- **What**: `@` follows the OpenVMS User's Manual (7.3), chapters 12 to
  14. Chosen without it or a VMS run:
  - (Settled, see the status.) A ninth parameter's message and the
    deepest level allowed: govax first gave MAXPARM and its own MAXDEPTH,
    an error, refusing the 33rd level above the terminal.
  - One SKPDAT warning for each run of skipped data lines. A `$` must be
    the record's first character (an indented `$` makes a data line).
  - `/OUTPUT=` with no file type gets `.LIS` (the manual says so for
    `@`). A bare name is in the default directory, not beside the
    procedure. NL: (or NLA0:) discards the output, and no other device
    is recognized. `/OUTPUT` may be abbreviated to `/O`, blanks are
    allowed around its `=`, and it can't be given without a value.
  - A blank before a qualifier makes it a parameter (13.6.4), even
    between the file name and the first parameter.
  - An unquoted host file name, which DCL uppercases, is also tried in
    lowercase. A host procedure is found through govax's search path.
  - A command line reached while the debugger reads the procedure's data
    ends its session (EXIT), and in the interactive assembler is `.END`,
    each as end of file on its input.
  - A failure that isn't a VMS status counts as an error, so it ends the
    procedure.
- **Status**: unconfirmed; candidates for Phase 50's VMS probe
  (subtask 20). DCL's message file has CLI$_DEFOVF ("too many command
  procedure parameters - limit to eight") and CLI$_STKOVF ("command
  procedures too deeply nested - limit to 32 levels"), and VMS 7.3 gave
  them for a ninth parameter and for the @ that would make the 32nd
  level, the terminal's among them, both as warnings (`testdata/dcl50`);
  govax now does the same. VMS made `/OUTPUT=.LOG` a file named `.LOG`;
  govax does too since Phase 51 (a new volume file may have a type and
  no name).

### [Phase 50] Substitution and expression rules, settled by a probe

- **Where**: `internal/console/dclsubst.go`, `dclexpr.go`, `dcllexical.go`,
  `dispatch.go`; `internal/vmserrors/codes_cli.go`.
- **What**: symbol substitution and expressions follow the OpenVMS
  User's Manual (7.3), 12.4 to 12.13. Where it was silent, govax first
  chose, and `testdata/dcl50`'s probe then asked VMS 7.3 (2026-10-09);
  govax now does what VMS did (the README lists each answer, and
  `TestProbe50Oracle` replays the expression and substitution cases
  against VMS's log). The choices the probe overturned: division by zero
  (2147483647, not an error), an unclosed quote (runs to the end of the
  line, not an error), a missing closing apostrophe (substitutes), `&` in
  `:=` and `@` (not substituted), an alias of an alias (IVVERB, not
  followed), `.EQ` with no closing dot (accepted), `.NOT.` after an
  operator (accepted), `1 + 2)` (assigned, then SYMDEL), too many
  lexical arguments (SYMDEL, not MAXPARM), a self-naming symbol (EXPSYN),
  segment lines (only on some messages), lexical abbreviations (against
  VMS's whole set of names), and the message prefix (`%DCL-`, for every
  message of govax's CLI facility).
  Still chosen without VMS: the quotient of a negative number divided by
  zero (2147483647 too); that `''NAME` inside quotes needs its closing
  apostrophe; that a comment isn't scanned; that a foreign command's text
  gets no `&` substitution; that an `&` value in a command is one quoted
  token, never read again for qualifiers; that a command line not
  starting with a letter, `$`, or `_` is NOCOMD.
  SHOW SYMBOL shows a quote in a value as it is, not doubled, as the
  manual's 12.6.1 example and VMS's log do (Phase 34 doubled it).
- **Status**: fixed to VMS 7.3's behavior where the probe answered; the
  rest unconfirmed, for a later round.

### [Phase 50] Statuses and ON: rules from the manual, the probe, and choice

- **Where**: `internal/console/dclstatus.go`, `procedure.go`, `run.go`;
  `internal/vmsdef/messages.go`, `internal/corevms/message.go`.
- **What**: `$STATUS`, `$SEVERITY`, EXIT's status, ON, and SET [NO]ON
  follow the OpenVMS User's Manual (7.3), 13.8 to 13.15. VMS 7.3's run of
  `testdata/dcl50` settled: `$STATUS` and `$SEVERITY` are global string
  symbols (`"%X00030001"`, `"0"`); a command that succeeds gives
  CLI$_NORMAL; a DCL message's status is its CLI$_ value, without bit 28
  though its message was shown (`TestProbe50Oracle` compares every case's
  `$STATUS`); SHOW SYMBOL leaves `$STATUS` alone; the end of a procedure
  passes `$STATUS` on, unmarked and not shown again (each of the 31
  levels of `probe50r.com` ended with STKOVF's %X00038128); EXIT without
  a status passes it with bit 28 set (%X10038028); `EXIT n` passes n, its
  message shown at the level above unless n is odd or has bit 28 set, and
  `$STATUS` is n as given; the system facility's message 0 is SS$_NORMAL's
  only (EXIT 0, 2, and 4 show `%NONAME-x-NOMSG, Message number
  0000000n`; `vmsdef.LookupMessage` and corevms's NOMSG stand-in now say
  so). `TestProbe50Statuses` replays section D.
  Chosen without VMS:
  - govax's statuses are its own codes; a failure's condition value is
    VMS's for the message of the same name in the matching facility
    (CLI_IVVERB is CLI$_IVVERB), with govax's severity, and govax's own
    code where VMS has no such name. Any other Go error is SS$_ABORT.
    Successful govax commands that are images on VMS (DIRECTORY, TYPE,
    ...) give CLI$_NORMAL, not the image's status.
  - EXIT without a status sets bit 28 whatever the status, a success too
    (the probe showed it only after a warning).
  - When ON's action is taken, `$STATUS` is the failing command's, and
    the action's own command sets it as any command does (CONTINUE
    leaves it). The manual says `$STATUS` "in general" becomes success
    (13.9.1); the common `ON ERROR THEN GOTO` handler that reads
    `$STATUS` suggests otherwise.
  - ON's command, already through apostrophe substitution as part of
    the ON line, is dispatched as a line again when it runs, so a value
    holding apostrophes is substituted a second time.
  - ON CONTROL_Y doesn't change the error action or SET NOON; an error
    ON turns SET NOON off (13.10 says so). ON and SET [NO]ON at the
    terminal are accepted and do nothing.
  - SKPDAT's warning sets `$STATUS` and can take ON's action.
  - RUN of an image that ends with a failure shows the status's message
    (unless bit 28 is set) and fails; one stopped another way (a HALT, a
    limit, CTRL/C) reports nothing, and a run under the debugger leaves
    `$STATUS` alone.
  - (Fixed, Phase 51.) A procedure that isn't found was govax's
    SS$_NOSUCHFILE, a warning; VMS gave DCL-E-OPENIN with RMS-E-FNF
    (`$STATUS` %X10018292), an error, which ends the calling procedure,
    and govax now does too (`procedureNotFound`). TYPE of a missing file
    is now VMS's TYPE-W-SEARCHFAIL with RMS-E-FNF (%X10951238). Chosen
    without VMS: a missing directory is RMS-E-DNF with
    -SYSTEM-W-NOSUCHFILE under it, as OPEN's is; a host file is shown by
    its path.
- **Status**: fixed to VMS 7.3's behavior where the probe answered; the
  rest unconfirmed, for a later round.

### [Phase 50] Labels, IF blocks, GOSUB, and CALL: choices without a probe

- **Where**: `internal/console/dcllabel.go`, `dclif.go`, `dclcall.go`,
  `dispatch.go` (`DispatchConsole`).
- **What**: labels and GOTO, IF/THEN/ELSE/ENDIF, GOSUB/RETURN, and
  CALL/SUBROUTINE/ENDSUBROUTINE follow the OpenVMS User's Manual (7.3),
  13.2 and 14.16 to 14.17: the duplicate-label rules (13.2.2), the 15
  nested IF blocks, the 16 nested GOSUBs, odd values as true, no GOTO or
  CALL into a separate block or subroutine, a CALL level reading its
  subroutine's lines only, and line-by-line running passing over a
  subroutine. The messages are DCL's CLI$_ messages, all warnings.
  Chosen without VMS:
  - A missing GOTO or GOSUB label leaves the procedure at its end of
    file, so it ends with the warning's status (not shown again) unless
    an ON action moves it first (the manual says GOSUB's "is forced to
    exit"). A missing CALL target is a warning and the procedure goes on.
  - A forward search records the labels it passes (at the GOTO's own
    block level), so a later GOTO finds a label the procedure jumped
    over; the manual says only that labels are entered as DCL
    "encounters" them.
  - A label inside a block the procedure has left can't be a target
    again (the forward search may still find a later one); a search that
    reaches the ELSE of an enclosing block treats that branch as a
    separate block.
  - `IF expr THEN` with nothing after THEN is CLI$_INSFPRM (as ON's is),
    not a block; anything after the expression other than THEN is
    CLI$_NOTHEN.
  - The command after a block's IF that isn't THEN is CLI$_NOTHEN, the
    block is dropped, and the command isn't run. THEN, ELSE, or ENDIF out
    of place, and a 16th nested block, are CLI$_INVIFNEST.
  - THEN, ELSE, ENDIF, GOSUB, SUBROUTINE, and a label alone on its line
    leave `$STATUS` alone, as IF and GOTO do (13.15); RETURN with no
    status does too. RETURN with a status sets `$STATUS` without showing
    a message, and the level's ON action looks at it.
  - THEN, ELSE, ENDIF, SUBROUTINE, and ENDSUBROUTINE may be shortened to
    four characters when DCL passes over lines or scans for a label.
  - Block IF works at the terminal (level 0) as in a procedure. There a
    label is CLI$_NOLBLS (and the command after it runs), GOTO is
    CLI$_USGOTO, GOSUB CLI$_INVGOSUB, RETURN CLI$_BADRET, CALL
    CLI$_USCALL, and SUBROUTINE and ENDSUBROUTINE CLI$_INVCALL.
  - A SUBROUTINE with no ENDSUBROUTINE, reached line by line or as a
    CALL's target, is CLI$_MSNGENDS; a search for a label that runs into
    one is MSNGENDS instead of USGOTO. A stray ENDSUBROUTINE is
    CLI$_INVCALL.
  - A CALL's target must be a label on a SUBROUTINE line; a plain label
    is CLI$_USCALL.
- **Status**: unconfirmed; candidates for the probe's next round.

### [Phase 50] Lexical functions: choices without the DCL Dictionary

- **Where**: `internal/console/dcllexical.go` (the table) and
  `dcllexstring.go`, `dcllexprocess.go`, `dcllexfile.go`,
  `dcllexname.go`, `dcllextime.go`.
- **What**: the lexical functions follow the OpenVMS User's Manual (7.3),
  chapter 15, which describes them only by example (the DCL Dictionary,
  their reference, isn't among the project's manuals). The items
  F$GETJPI, F$GETSYI, and F$GETDVI return are those `$GETJPI`, `$GETSYI`,
  and `$GETDVI` support. Chosen without VMS:
  - Keyword arguments (items, fields, edits, formats) must be spelled in
    full; one the function doesn't have is CLI$_IVKEYW with the keyword
    as its segment. A privilege name that isn't one is IVKEYW too.
  - An argument that should name a symbol (F$TYPE's, F$PID's) and isn't
    a name is CLI$_IVSYMB.
  - F$EXTRACT with a negative offset or length, F$ELEMENT with a negative
    number, and F$CVSI/F$CVUI with a field outside the string or wider
    than 32 bits are CLI$_INVRANGE; F$ELEMENT's delimiter of other than
    one character is CLI$_IVVALU. F$LOCATE of an empty string is 0.
  - F$TYPE says INTEGER for a string that is an integer, allowing blanks
    around it, one sign, and a radix prefix.
  - F$EDIT never edits quoted text, for any edit, and makes its edits in
    a fixed order (UNCOMMENT, COLLAPSE, COMPRESS, TRIM, LOWERCASE,
    UPCASE) however they're listed.
  - F$FAO writes an integer given to a string directive in decimal, and
    converts a string given to a numeric one; !%D and !%T take only 0
    (the current time).
  - F$ENVIRONMENT: CONTROL is "Y" and NOCONTROL "T"; ON_SEVERITY is
    "NONE" at the terminal and after SET NOON, "ERROR" by default, and
    "SEVERE" for ON SEVERE_ERROR; PROTECTION is VMS's default, written as
    the manual's 15.6.2 example shows it.
  - F$VERIFY with only its first argument sets image verification too.
    (Its second argument without the first changes nothing: VMS 7.3,
    testdata/dcl50's round 2.)
  - F$PID keeps a count in its context symbol and sets it back to "" at
    the end of the list, so the next call starts again.
  - F$GETJPI writes PIDs (PID, MASTER_PID, OWNER) as eight hexadecimal
    digits, UICs as `$FAO`'s !%I does ("[SYSTEM]"), privileges as their
    names in bit order (DETACH and NOACNT where two names share a bit),
    MODE and JOBTYPE as keywords, and STATE as SHOW SYSTEM does.
    USERNAME keeps its blank padding. F$USER is F$GETJPI's UIC.
  - F$SEARCH searches once per stream for a new specification, and after
    the "" that ends a search the stream starts again, so a name without
    wildcards is found on every other call. A field the specification
    leaves out isn't a wildcard (no default name). Host files are
    matched without regard to case.
  - F$PARSE returns "" for a specification RMS can't parse; with no
    device at all (a default on the host), it returns the fields without
    checking anything.
  - F$CVTIME: an absolute time's missing date fields are today's and its
    missing time fields 0 ("14:00", "9-OCT-2026" at midnight); in
    comparison format DAYOFYEAR and the other OFYEAR fields are
    zero-filled (3, 4, 6, and 8 digits); in absolute format the date has
    no leading blank ("9-OCT-2026"); a delta's DATETIME is "1
    02:03:04.50". A combination splits at the first + or - where both
    halves read.
  - F$MESSAGE takes one argument, as in VMS 7.3 (later releases added a
    second), and returns the message file's text with its FAO directives
    as they are.
- **Status**: unconfirmed; candidates for the probe's next round.

### [Phase 50] Verification: settled by a probe, and the rest chosen

- **Where**: `internal/console/dclverify.go` (SET VERIFY, SET PREFIX,
  the echo), `dclsubst.go` (F$VERIFY in a comment), `dispatch.go`,
  `procedure.go`, `dclif.go`, `dclcall.go`.
- **What**: testdata/dcl50's round 2 (`probe50v.com`, VMS 7.3's log in
  `vax/probe50v.log`, replayed by `TestProbe50VOracle`) settled: a line
  is shown after its apostrophe substitutions, a continued command a
  record at a time, each record substituted; comment and blank `$`
  lines are shown; the THEN of a false block IF isn't, its ELSE and
  ENDIF are, and nothing between; a CALL shows the SUBROUTINE line, then
  the body, then ENDSUBROUTINE; an ON action isn't shown;
  `'F$VERIFY(n)'` in a comment (or a command's trailing comment) is
  performed, abbreviated or not, with or without its closing
  apostrophe, and the line is shown with the call replaced by its value;
  a nested procedure's SET NOVERIFY or `'F$VERIFY(0)'` lasts after it
  returns; SET VERIFY's keywords may be abbreviated (IMA was taken); SET
  NOVERIFY with keywords is CLI$_CONFLICT (`\NOVERIFY\` as typed); SET
  PREFIX with `!UL`/`!AS`/`!XL`, or `"!%D !"`, is CLI$_INVPREFIX and the
  prefix stays; F$VERIFY(,1) changes nothing. Chosen without VMS:
  - Which prefix control strings are valid: only !%T and !%D may take an
    argument (0, the current time), with a field width; !/, !_, !^, !!,
    and !n*c are allowed; anything else, and a "!" at the end, is
    INVPREFIX (whether `!%D` alone is valid is unconfirmed: the refused
    `"!%D !"` ends with a lone "!"). A control string over 64 characters
    is CLI$_IVVALU.
  - Nothing in a comment but F$VERIFY is substituted, and a call with an
    error is left as it is.
  - A continued command whose records can't be matched to its scanned
    line (an apostrophe pair spanning two records) is shown as one line.
  - The comment on a continued command's first record isn't scanned, so
    F$VERIFY in it isn't performed.
- **Section V6**, image verification, came with subtask 16 (below, "SYS$INPUT
  from a procedure"): VMS shows each data line TYPE SYS$INPUT reads
  (then TYPE writes it), and a data line after a DCL command that isn't
  an image (SET VERIFY) was shown and then read as a command,
  `%DCL-W-IVVERB \DATA\`, not passed over with SKPDAT.
- **Status**: VMS's answers followed; the choices above unconfirmed.

### [Phase 50] SYS$INPUT from a procedure: settled by two probes, and the rest chosen

- **Where**: `internal/console/dclinput.go` (`procedureInput`,
  `readData`, DECK and EOD, `dclOwnCommands`, TYPE SYS$INPUT),
  `procedure.go` (`runLevel`, `procedureSource.Read`), `dispatch.go`,
  `internal/corevms/input.go` (`RedirectInput`).
- **What**: an image a procedure runs reads the procedure's data lines
  as SYS$INPUT, to the next `$` line (User's Manual, 13.1.3, 14.1); DECK
  and EOD follow the DCL Dictionary. VMS 7.3 settled (testdata/dcl50's
  V6 and round 3, `probe50i.com`, replayed by `TestProbe50IOracle`):
  - Image verification shows each line as it is read, before the image
    uses it, without the prefix; the DECK record and the record that
    ends a deck aren't shown.
  - After an image, the data up to the next `$` record is skipped with
    one SKPDAT: SHOW LOGICAL, DIRECTORY (alone or after IF's THEN), and
    a TYPE that failed were images. After DCL's own commands a data line
    is read as a command: assignments, SHOW SYMBOL (failing too), SHOW
    DEFAULT, SHOW TRANSLATION, SET DEFAULT, SET PROMPT, SET [NO]ON, SET
    VERIFY, SET SYMBOL, DEFINE, ASSIGN, DEASSIGN, IF (false, or true
    with one of these), CONTINUE, WRITE, WAIT, an unknown verb, GOTO and
    a label, GOSUB's RETURN, CALL, and @; so is a procedure's first
    record.
  - A lone `$` ends the skipping: the data line after it was read as a
    command.
  - A `$` after blanks or a tab is data; an empty record is data.
  - DECK read by DCL as a command is CLI$_IVVALU, a warning (%X00038088),
    with or without /DOLLARS, and starts no deck. EOD read by DCL does
    nothing and leaves `$STATUS` alone.
  - /DOLLARS's string is uppercased unless quoted, and ends the deck at a
    record that starts with it in column 1, case and all (`ENDING`
    ended a deck of END; `end, lowercase` didn't). /DOLLARS with no
    string ends at EOD. An EOD with no deck (`$EOD` too) ends the input
    and is used up; the data after it is skipped (SKPDAT), being after
    an image.
  - TYPE of a logical name that translates to SYS$INPUT, directly or
    through another, types the data.
  - A line `%%` read as a command is IVVERB, `\%%\`, not NOCOMD: `%`
    can start a verb (round 1 gave NOCOMD for an apostrophe).
  - For subtask 17: READ SYS$INPUT reads the next record whatever it is,
    a `$` line or an EOD included (so it reaches no end of file at one),
    and the data line after the record it took is read as a command;
    WRITE leaves `$STATUS` at RMS$_NORMAL, %X00010001.
- **Chosen without VMS**:
  - govax's own commands (PRINT, ECHO, TIME, SET RADIX, ...) count as
    images. A command whose qualifiers the grammar refuses counts as one
    of DCL's (no image would have run).
  - A `$!` comment record ends the skipping as a lone `$` does.
  - DECK at the terminal is IVVALU too.
  - DECK's other forms (a parameter, another qualifier) read by an image
    aren't a DECK, so the record ends its input like any `$` record. A
    deck an image started and didn't read to its end goes on when DCL
    reads again: its records are data.
  - What an image has taken into its input buffer but not read when it
    ends is lost, not skipped with SKPDAT (an image reads a line at a
    time, so this is only part of a line).
  - TYPE follows each logical name's first equivalence string, and one
    with a directory or file name after the device is a file. At the
    terminal TYPE SYS$INPUT reads to Ctrl/Z.
  - Lines the debugger or the interactive assembler read are given to
    them as before; an image they run (the debugger's GO) reads the
    procedure's data too, so the two interleave as on VMS.
  - A subprocess an image starts keeps the procedure's input only until
    the command ends; after that it reads end of file.
- **Status**: VMS's answers followed; the choices above unconfirmed.

### [Phase 50] DCL's file I/O: settled by a probe, and the rest chosen

- **Where**: `internal/console/dclfile.go` (OPEN, CLOSE, READ, WRITE,
  INQUIRE), `internal/rms/dclfile.go` (`Session.OpenDCLFile`,
  `DCLFile`), `internal/console/dclstatus.go` (`branchStatus`,
  `endStatus`), `procedure.go` (`procedureLine`'s ON check),
  `logical.go` (SHOW LOGICAL/FULL's name attributes, DEASSIGN's status),
  `type.go` and `internal/rms/type.go` (TYPE of an open file),
  `page.go` (`readReply`'s Ctrl/Z), `internal/vmserrors/error.go` (the
  segment line's length).
- **What**: the five commands as the DCL Dictionary (7.3-1) describes
  them, checked against VMS 7.3 in round 4 of testdata/dcl50
  (`probe50f.com`, and the interactive `probe50q.com`, replayed by
  `TestProbe50FOracle` and `TestProbe50QOracle`; every section matches,
  but for the layouts of DELETE, DIRECTORY, and DUMP). VMS 7.3 settled:
  - **Statuses.** OPEN and INQUIRE leave %X00000001, an OPEN passed over
    (the name already has a file) and READ CLI$_NORMAL, WRITE and CLOSE
    RMS$_NORMAL, DEASSIGN %X00000001, CLOSE/NOLOG of a name never opened
    CLI$_NORMAL (and no message). OPEN's failure is `%DCL-E-OPENIN, error
    opening NAME as input` (OPENOUT only for a new file: /APPEND and
    /READ/WRITE of a missing file are OPENIN), then RMS's message, and
    under RMS$_DNF `-SYSTEM-W-NOSUCHFILE`; `$STATUS` is the RMS status
    with bit 28 set, even at an /ERROR label. READ's and WRITE's failures
    are RMS's message alone, their status without bit 28, at a label
    too. A READ that fails leaves its symbol alone (no local symbol is
    made).
  - **A branch is success to the procedure.** After an /END_OF_FILE
    branch `$STATUS` was RMS$_EOF, but the procedure's EXIT (no status)
    ended it with %X10030001: DCL's own status after a branch is
    CLI$_NORMAL (govax's `endStatus`, used by EXIT and the end of file).
  - **Files.** A new file is VFC, 2 bytes of fixed control, print-file
    carriage control (DIRECTORY/FULL); records as READ reads them, the
    fixed control left out (DUMP/RECORD). An open that writes may read
    and update: READ of a new file was RMS$_EOF, WRITE/UPDATE RMS$_CUR,
    not RMS$_FAC; /READ/APPEND reads from the end. WRITE before
    /READ/WRITE's end of file is RMS$_NEF, WRITE/UPDATE of another length
    RMS$_RSZ.
  - **Sharing.** A file OPEN/WRITE has open (no /SHARE) keeps every other
    opener out: a reader, a /SHARE reader, and TYPE (`%TYPE-W-OPENIN`,
    RMS$_FLK). Two /SHARE=READ readers share a file and keep /APPEND
    out. So without /SHARE the sharing is RMS's default (GET for a
    reader, none for a writer), not the manual's "others may read".
  - **The logical name** is `[super,confine]`, equated to
    `_SIMVAX$DUA1` (the node's physical device name, no colon)
    `[terminal]`. It is the way to the file: TYPE of it typed the file,
    using up READ's records; once deassigned, READ and CLOSE were UNDFIL.
  - **Names.** WRITE `SYS$OUTPUT:` (with a colon) was UNDFIL; a logical
    name for SYS$OUTPUT works. WRITE SYS$INPUT in a procedure is
    RMS$_FAC. WRITE SYS$ERROR and SYS$COMMAND go to the terminal, even
    under /OUTPUT (not to the log). OPEN/WRITE/APPEND is CLI$_CONFLICT
    `\WRITE\`; a bad /SHARE keyword CLI$_IVKEYW; missing parameters
    INSFPRM; READ's bad symbol IVSYMB; WRITE with no data INSFPRM; a
    trailing comma or two items with no comma EXPSYN.
  - **Long values.** Without /SYMBOL, a WRITE item longer than 255
    characters is CLI$_TKNOVF, the segment line cut to 255 characters
    (no closing backslash); WRITE/SYMBOL and READ handle 600.
  - **Data lines** after OPEN, READ, and CLOSE are read as commands.
  - **The terminal.** READ's prompt is "Data:" and a tab (the manual
    says "DATA:"); Ctrl/Z is RMS$_EOF, shown without a label. INQUIRE's
    prompt is its string or the symbol's name and ": "; the reply is
    substituted, then uppercased and compressed outside quotes; Ctrl/Z
    gives "" and %X00000001; with procedure verification on, the
    substituted reply is shown on the procedure's output (in the /OUTPUT
    log, not the terminal), and nothing at Ctrl/Z. INQUIRE's prompt goes
    to the terminal under /OUTPUT.
- **Still chosen without VMS**:
  - (Settled by round 6, "[Phase 51] Round 6 of the probe".) The fixed
    control written is 01 8D (a line feed before, a carriage return
    after), as VMS's DUMP/BLOCK showed. TYPE shows such a record of a
    print file as a line.
  - (Settled by round 6.) SHOW LOGICAL indents every name it looks up
    three blanks (`   "OUT" ...`), not only a process-permanent file's;
    govax now does. `no_alias`'s place among a name's attributes is
    unconfirmed.
  - A host file's logical name is equated to its path.
  - DEASSIGN's %X00000001 is taken as $DELLNM's SS$_NORMAL (it could be
    DEASSIGN leaving the OPEN's); DEFINE's status is unchanged. (Phase
    71: VMS 7.1's DEFINE of a new name left %X00000001, and govax's now
    does.)
  - READ SYS$INPUT's success status is CLI$_NORMAL, as a file's is.
  - A colon after an OPEN's logical name in READ, WRITE, and CLOSE is
    accepted (the manual's `logical-name[:]`); only the
    process-permanent names were seen refusing it.
  - TYPE shares a file as every operation (SHR GET, PUT, UPD, DEL).
  - CLOSE of SYS$INPUT and the other process-permanent names is UNDFIL.
  - A missing parameter at the terminal is INSFPRM; DCL would prompt.
  - READ's 2048-byte limit (RMS$_RTB, the record cut) and /TIME_OUT,
    WRITE's 1024-byte buffer, record locks, indexed files, OPEN of a
    mailbox or terminal (NL: works), and an image's access to a file
    DCL has open are not done or not checked. (Phase 71 did the last:
    an image's `$OPEN` of OPEN's name reaches DCL's file, VMS 7.1's
    part F.)
- **Fixed in passing**: TYPE (and COPY to the host, which share
  `rms.writeRecords`) treated every VFC file as a print file, expanding
  its fixed control as carriage control; the RMS manual makes it carriage
  control only with RAT=PRN, so a VFC file with RAT=CR is now a line per
  record. govax's TYPE read a file other openers kept readers out of;
  it is now one of the file's RMS openers (RMS$_FLK). The console's
  terminal reads (`readReply`, INQUIRE's and READ's) took a Ctrl/Z as a
  character, not the end of the input.
- **Status**: VMS's answers followed; the choices above unconfirmed.

### [Phase 50] STOP and Ctrl/Y: rules from the manuals, and choices without a probe

- **Where**: `internal/console/dclcontroly.go` (STOP, Ctrl/Y's actions,
  the Ctrl/Y level, SET [NO]CONTROL), `procedure.go` (`procedureLine`,
  `runLevel`, `finishProcedure`), `execute.go` (`ReportStop`),
  `internal/cpu/engine.go` (`ClearAttention`), `cmd/govax/attention.go`.
- **What**: from the User's Manual (13.7.1, 13.8.1's table, 13.11 to
  13.13 and Figures 13-2 and 13-3) and the DCL Dictionary (STOP,
  CONTINUE, EXIT, SET CONTROL):
  - STOP in a procedure (or at the Ctrl/Y level) ends every command
    level, without the outer levels' ON actions.
  - Ctrl/Y with an ON CONTROL_Y: an image is forced to exit, a DCL
    command completes, and the command runs at that level; it stays in
    effect.
  - Ctrl/Y without one, below level 1: the procedure exits to the level
    above, which goes on (even if it has an ON CONTROL_Y, Figure 13-3).
  - Ctrl/Y without one, at level 1: the Ctrl/Y level. Commands run in
    the procedure's context; CONTINUE resumes (the image too); EXIT and
    STOP end the procedure; a command that runs an image ends it and
    then runs at level 0.
  - SET NOCONTROL=Y: Ctrl/Y echoes *Interrupt* and does nothing; Ctrl/C
    does nothing unless an AST takes it; the current level's ON
    CONTROL_Y is cancelled.
- **Chosen without VMS** (round 5 of testdata/dcl50 asks):
  - A procedure STOP ended has SS$_ABORT with bit 28 (%X1000002C); one
    Ctrl/Y ended (below level 1) has SS$_CONTROLY (%X00000611).
  - EXIT at the Ctrl/Y level ends the procedure with its status, as an
    EXIT in it would; Ctrl/Z there (or the end of the input) is EXIT.
  - At the Ctrl/Y level, F$ENVIRONMENT("DEPTH") is the procedure's level
    (1), symbols are the procedure's, and IF blocks are the terminal's.
  - Which commands count as images there is `dclOwnCommands`'s rule (so
    govax's own PRINT counts as one); @ counts as one too.
  - Ctrl/C in a procedure, where no AST takes it, acts as Ctrl/Y (VMS
    does this for an image with no Ctrl/C AST); at the terminal, both
    keys keep govax's meanings (Ctrl/C interrupts, Ctrl/Y ends govax),
    not VMS's (interrupt, with CONTINUE).
  - Ctrl/Y typed at the Ctrl/Y level prompts again.
  - Exit handlers: govax's image rundown runs none, so EXIT and STOP at
    the Ctrl/Y level run down an interrupted image alike.
  - (Done in Phase 51, "[Phase 51] STOP/IMAGE and STOP/[NO]EXIT".)
    STOP/IMAGE and STOP/[NO]EXIT were not done.
- **Status**: the manual's rules followed; the choices above unconfirmed.

### [Phase 50] The subprocess's DCL: choices without a probe

- **Where**: `internal/console/subcli.go`, `logical.go` (SHOW LOGICAL's
  status), `procedure.go` (`Console.Exit` at level 0).
- **What**: a subprocess runs the console's DCL (docs/PHASE-50, "The
  subprocess's DCL"). Chosen without VMS:
  - The commands refused in a subprocess (`processOneCommands`) are
    govax's machine commands. (SPAWN was refused too until Phase 51's
    subtask 2; a subprocess now SPAWNs its own, as VMS allows.)
  - A job that isn't interactive (LOGINOUT with a file or mailbox
    input) has SET VERIFY on from its start, so a procedure it runs
    shows its lines (VMS's batch default; probe 4 showed only the
    job's own lines).
  - Ctrl/Y in a subprocess's procedure wasn't delivered (the Ctrl/Y
    handling was process 1's) until Phase 51's subtask 3 ("[Phase 51]
    Ctrl/Y in a subprocess").
  - A subprocess's procedures run, like its images, only while the
    machine runs (since subtask 21, that includes process 1's lending
    the CPU between its DCL commands, during WAIT, and at the prompt).
  - SHOW LOGICAL's status in process 1 is now SS$_NORMAL (or
    SHOW$_NOTRAN) with bit 28, as VMS gave in a subprocess (probes 4 and
    5); it was CLI$_NORMAL.
- **Status**: unconfirmed. SPAWN in a subprocess is done (Phase 51,
  subtask 2: it calls LIB$SPAWN's Go and waits in the scheduler; its
  messages and status are process 1's SPAWN's, unconfirmed for a
  subprocess); Ctrl/Y in a subprocess's procedures is done too (Phase
  51, subtask 3).

### [Phase 50] Round 5 of the probe: STOP, Ctrl/Y, flow, lexicals, subprocesses

- **Where**: `internal/console/dclcontroly.go`, `dclif.go`,
  `dcllabel.go`, `dclcall.go`, `dclstatus.go`, `dclexpr.go`,
  `dclsubst.go`, `dcllexstring.go`, `dcllextime.go`, `dcllexfile.go`,
  `dcllexprocess.go`, `spawncmd.go`, `subcli.go`, `dclwait.go`;
  `internal/rms/lexical.go`, `internal/corevms/lexical.go`,
  `internal/vmsdef/messages.go`.
- **What**: testdata/dcl50's round 5 (`probe50t.com`, replayed by
  `TestProbe50TOracle`; the interactive `probe50y.com`) answered most of
  what "[Phase 50] Statuses and ON", "Labels, IF blocks, GOSUB, and
  CALL", "Lexical functions", "STOP and Ctrl/Y", and "The subprocess's
  DCL" chose; govax now does what VMS 7.3 did (the README's "What VMS
  answered in round 5" lists each answer). Those entries' choices are
  confirmed or replaced as follows:
  - Confirmed: EXIT's bit 28 after a success; `$STATUS` at an ON ERROR
    THEN GOTO label; ON's command substituted when ON is read; ON
    turning SET NOON off; SKPDAT taking ON WARNING's action; a forward
    search recording the labels it passes; THEN, ELSE, ENDIF, GOSUB,
    and labels leaving `$STATUS`; ENDI and ENDS; full keyword spellings;
    F$ELEMENT past the end; F$LOCATE of ""; F$TYPE's integers; F$CVTIME's
    absolute DATE with no leading blank and its combinations; ON
    CONTROL_Y staying in effect; SET NOCONTROL=Y cancelling it; Ctrl/C as
    Ctrl/Y; CONTINUE resuming an image; F$ENVIRONMENT's DEPTH at the
    Ctrl/Y level.
  - Replaced: STOP's status (now `$STATUS` unchanged, not SS$_ABORT);
    Ctrl/Y below level 1 (now the prompt unless some level has an ON
    CONTROL_Y, and then the levels inside it end with `$STATUS` marked as
    shown, not SS$_CONTROLY); EXIT at the Ctrl/Y level (every level
    ends); `IF 1 THEN` alone (nothing, not INSFPRM); the command after a
    block's IF (passed over, not NOTHEN); INVIFNEST and INVCALL (errors);
    RETURN's status (%X10000001, and a failure's message); a missing
    GOSUB or CALL target (USGOTO's status; CALL's procedure ends); GOTO,
    RETURN, and CALL at the terminal; F$EXTRACT's and F$ELEMENT's range
    rules; F$TYPE of a non-name (ARGREQ); JOBTYPE (an integer); ACNT;
    F$SEARCH without wildcards; F$CVTIME's OFYEAR fields (gone);
    DIRECTORY's and TYPE's success statuses; a subprocess's verification;
    SPAWN's status.
- **Still chosen without VMS**: WAIT's status when it runs to its end
  (left alone, as when Ctrl/Y interrupts it); RETURN at the terminal
  (BADRET's status, marked as shown); ELSE before a block's THEN
  (INVIFNEST, closing the block); a CALL of a plain label (as a missing
  target); a delta's hundredths written with the digits given (one case
  seen); F$FAO given the wrong kind of value (govax converts it; VMS
  wrote garbage); SPAWN not repeating a status of govax's own, which has
  no message in the message file. (A subprocess's WAIT no longer blocks
  the machine: Phase 50's subtask 21 made it wait in the scheduler.)
- **Status**: VMS's answers followed; the choices above unconfirmed.
  Round 6 of the probe answered WAIT's status, RETURN at the terminal,
  ELSE before THEN, a CALL of a plain label, and a delta's hundredths
  ("[Phase 51] Round 6 of the probe").

### [Phase 51] Round 6 of the probe: WAIT, flow, SHOW LOGICAL, deltas

- **Where**: `internal/console/dclif.go` (ELSE before THEN, the 16th
  block), `dclcall.go` (GOSUB's depth, CALL of a plain label, RETURN at
  the terminal), `dispatch.go` (a digit starts a verb), `dclwait.go`
  (IVDTIME's segment), `dcllextime.go` (`deltaAsTyped`), `logical.go`
  (SHOW LOGICAL's layout).
- **What**: testdata/dcl50's round 6 (`probe50a.com` with /OUTPUT,
  replayed by `TestProbe50AOracle`; the lines typed at the terminal,
  `TestProbe50ATerminal`) asked what round 5 left open and a few older
  choices. govax now does what VMS 7.3 did (the README's "What VMS
  answered in round 6"):
  - Confirmed: WAIT leaves `$STATUS` alone; `/OUTPUT`'s default type
    .LIS, `/O=`, and blanks around its `=`; OPEN/WRITE's fixed control
    01 8D; `.X` and `(X)` are NOCOMD; SUBROUTINE, ENDSUBROUTINE, and
    ELSE at the terminal are INVCALL and INVIFNEST.
  - Replaced: an ELSE before a block's THEN (passed over, then the THEN
    is INVIFNEST and closes the block; it was INVIFNEST at the ELSE); a
    false IF's lines before its THEN aren't shown with verification;
    CALL of a plain label (runs from the label at a new level, then
    INVCALL; it was USCALL); GOSUB's depth (15, not 16); the 16th block
    IF (accepted, its THEN refused; the IF was refused); RETURN at the
    terminal (silent, `$STATUS` marked as shown, or set to its status;
    it was BADRET); SHOW LOGICAL's indentation (three blanks for a name
    looked up, `1  ` for an iterated level), its tab before a search
    list's further values, and no blank line after an empty table's
    header; F$CVTIME's delta fields written as typed; a digit starting
    a verb (IVVERB, not NOCOMD); WAIT's IVDTIME segment (`0  ` before
    the time; the author saw `\0  BOB\` for WAIT BOB too, a known quirk
    of WAIT's message).
- **Still chosen without VMS**: what ends a plain label's CALL level
  when no EXIT does (the end of the file); an even RETURN status's
  message at the terminal (shown, as in a procedure); whether lines
  before a false IF's THEN, other than ELSE, are hidden (one case, with
  an ELSE among them); the CALL level's symbol scope for a plain label
  (a new level's, as for a SUBROUTINE). VMS wrote a NUL byte after WAIT
  NOTATIME's segment, which govax doesn't write. VMS dropped the blank
  in one verified comment (`@'s /OUTPUT` shown as `@'s/OUTPUT`), which
  govax doesn't do (unexplained; the oracle allows it).
- **Status**: VMS's answers followed; the choices above unconfirmed.

### [Phase 51] Ctrl/Y in a subprocess: choices without a probe

- **Where**: `internal/dclshell/dclcontroly.go` (`attachedCLI`,
  `subprocessAttention`, `imageInterrupted`), `subcli.go` (the CLI's
  Ctrl/Y AST), `dclwait.go`, `dclfile.go` (`readTerminal`);
  `internal/corevms/spawn.go` (`Attached`), `ast.go` (`QueueCLIAST`),
  `input.go` (`ReadCommandLine`); `internal/cpu/xfc.go` (a shim may end
  the image).
- **What**: the terminal's Ctrl/Y and Ctrl/C go to the process it is
  attached to: process 1, or the subprocess a SPAWN (or a program's
  LIB$SPAWN) without /NOWAIT waits for, down a chain of SPAWNs, as the
  DCL Dictionary's SPAWN and ATTACH describe. That subprocess's DCL
  follows Phase 50's rules in its procedures (ON CONTROL_Y, the
  unwinding to an outer level's handler, the Ctrl/Y level, CONTINUE,
  STOP, WAIT interrupted). Chosen without VMS:
  - An image a subprocess runs is interrupted by a user-mode AST of
    DCL's that calls the CLI's shim (on VMS, DCL's Ctrl/Y AST is
    supervisor mode); the image is paused beneath it, so it can't be
    interrupted while it runs with user-mode ASTs disabled or in an
    inner mode for good. A second key while that AST hasn't been taken
    stops the machine, as a key no DCL takes does (at process 1's level
    0, Ctrl/Y then ends govax).
  - An image the Ctrl/Y ends (ON CONTROL_Y, EXIT, STOP, a command that
    runs an image at the Ctrl/Y level) is run down with no exit handlers
    called, as process 1's abandoned images are.
  - Outside a procedure, a subprocess's Ctrl/Y or Ctrl/C ends the image
    it runs, the command's status SS$_CONTROLY, and its DCL goes on with
    its next command (a SPAWN of one command logs out). VMS's DCL would
    show its prompt with the image paused, for CONTINUE. Neither key
    ends govax from a subprocess.
  - A subprocess made with /NOWAIT shares the terminal but isn't
    attached: the keys don't reach it (VMS's Ctrl/Y "interrupts all
    such subprocesses").
  - SET [NO]CONTROL is one setting for every process's DCL.
  - A subprocess's DCL reads the terminal (INQUIRE, READ SYS$COMMAND,
    the Ctrl/Y level) as it reads its commands, waiting in the
    scheduler while no line has come; it was a host read that held the
    machine. During a procedure line, that read is SYS$COMMAND, not the
    procedure's data lines.
  - No ATTACH: the terminal moves only with SPAWN and the subprocess's
    end.
- **Status**: unconfirmed.

### [Phase 51] F$CONTEXT, F$DEVICE, F$FILE_ATTRIBUTES, F$IDENTIFIER: choices without a probe

- **Where**: `internal/dclshell/dcllexcontext.go` (F$CONTEXT, and
  F$PID with a context), `dcllexfile.go` (F$DEVICE, F$FILE_ATTRIBUTES),
  `dcllexprocess.go` (F$IDENTIFIER); `internal/rms/fileattr.go`
  (`Session.FileAttributes`); `internal/corevms/lexical.go`
  (`DeviceScan`, `IdentifierValue`, `IdentifierName`).
- **What**: the four functions follow the DCL Dictionary (7.3-1).
  F$CSID, F$GETQUI, and F$LICENSE stay CLI$_LEXNOTIMPL by choice: govax
  has no cluster, queues, or licenses for them to report. Chosen without
  VMS:
  - F$CONTEXT's symbol's value is the context's number, an integer; once
    F$PID's "" or CANCEL ends the context it is "", a string. A symbol
    that names no live context is CLI$_NOTCTX, a criterion added after
    F$PID has used the context CLI$_FRZNCTX (the message file's words);
    a selection item, qualifier, keyword value, or privilege it doesn't
    know is CLI$_IVKEYW, an integer value that isn't one CLI$_IVVALU, a
    missing value or qualifier CLI$_ARGREQ. A list's values are split at
    commas outside brackets, so "[1,4]" is one UIC; UIC also takes an
    identifier's name. HW_MODEL, HW_NAME, and STS aren't supported
    (IVKEYW). No privilege is asked for to see other processes.
  - F$DEVICE returns names with the node's, "_GOVAX$DUA0:" (VMS 7.3
    equated OPEN's logical name to "_SIMVAX$DUA1"), sorted by name; the
    class and type are $DCDEF's and $DTDEF's names without their
    prefixes, one that isn't is IVKEYW, and a type without a class
    SS$_BADPARAM. MOUNT's disks have no device type, so a type selects
    none of them. Streams behave as F$SEARCH's.
  - F$FILE_ATTRIBUTES applies no default name (no type is an empty
    type); a wildcard is RMS$_WLD, a missing file RMS$_FNF (RMS$_DNF for
    a missing directory), each shown as its message. EOF is the blocks
    used, as DIRECTORY/SIZE counts them; MRS is the header's maximum
    record size and LRL its record size, which RMS keeps as the longest
    record; a date never set is "17-NOV-1858 00:00:00.00"; PRO is
    "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD="; UIC as !%I writes it;
    DVI as OPEN's logical name is equated, "_GOVAX$DUA0" without a colon;
    BLS 512; VERLIMIT 32767 for none; KNOWN and SHELVED FALSE, MOVE and
    SHELVABLE TRUE, STORED_SEMANTICS "", FILE_LENGTH_HINT "(-1,-1)". A
    host file reads as stream-LF with CR carriage control, its size in
    blocks, its modification time as CDT and RDT, owned by [1,4].
  - F$IDENTIFIER: a name the rights database doesn't have is 0, and so is
    one that can't be an identifier ($ASCTOID's SS$_IVIDENT); a value it
    doesn't have is "". NUMBER_TO_NAME of a UIC is the username without
    brackets, as the Dictionary's example shows.
- **Status**: unconfirmed; candidates for a probe.

### [Phase 51] STOP/IMAGE and STOP/[NO]EXIT: choices without a probe

- **Where**: `internal/dclshell/dclcontroly.go` (`stopCommand`,
  `StopImage`, `stopProcess`), `subcli.go` (`stopAfterImage`),
  `internal/corevms/process.go` (`ForceExit`).
- **What**: from the DCL Dictionary (STOP). STOP/IMAGE calls $FORCEX's
  code. Chosen without VMS:
  - The forced exit's status is SS$_FORCEDEXIT (the Dictionary doesn't
    say what DCL passes $FORCEX); the subprocess's RUN shows its
    message, as for any image failing.
  - STOP/IMAGE of a subprocess at its DCL, running no image, does
    nothing; of process 1 by name, it runs down an image Ctrl/C or
    Ctrl/Y left stopped, as STOP alone does.
  - govax calls exit handlers only in the mode that calls $EXIT, and
    images have them only in user mode, so /EXIT=KERNEL_MODE,
    EXECUTIVE_MODE (the default), SUPERVISOR_MODE, and /NOEXIT all
    delete at once, as STOP did before. /EXIT=USER_MODE forces the
    image's exit with SS$_ABORT, and the process is deleted once it has
    exited (a subprocess's DCL logs out, its procedures ended, final
    status SS$_ABORT); STOP returns before then. /EXIT's mode may be
    abbreviated to a letter.
- **Status**: unconfirmed.


<!--
Entry template:

### [Phase NN] Short title

- **Where**: `reference/eVAX/eVAX/Source/.../file.c` (function/lines), ported to
  `internal/.../file.go`.
- **What**: what the C code does vs. what the ISA manual / the C code's own comments
  say it should do.
- **Status**: deferred (Go replicates C behavior as-is) | fixed in Go (rationale).
-->


### [Phase 71] Process-permanent files: the choices VMS didn't settle

- **Where**: `internal/rms/ppf.go`, `ppfaccess.go`, `ppfstream.go`;
  `internal/corevms/ppfstart.go`, `ppfdcl.go`, `outfile.go`;
  `internal/dclshell/dclppf.go`, `dclfile.go`; `internal/lnm/ppf.go`.
- **What**: VMS 7.1's run of `testdata/ppf71` settled the strings, the
  entries' sharing, indirect access, and DCL's names
  (`docs/PHASE-71 - process-permanent files.md`, Part A), and the
  oracles hold govax to it. Chosen without VMS:
  - Entries and streams are numbered lowest free first (VMS's numbers
    depend on its process I/O segment; masked), and the indirect STV is
    0 (VMS's varied; masked).
  - The terminal's input entry's access is GET; a mailbox's device text
    in its string, `_MBAn:`, and its DVI$_FULLDEVNAM, with the node's
    name; a created process's TT for terminal or file input.
  - An image's own SYS$OUTPUT file (a name its DCL defined for it alone)
    is VAR with CR; `$DISPLAY`'s resultant string is VMS's for the
    terminal's output entry only.
  - The guide's "no random access" for SQO files isn't enforced (an
    indirect `$GET` by RFA works), and a name whose file has been closed
    is RMS$_IFI at `$OPEN` and RMS$_DEV at `$PARSE`.
  - DEFINE superseding a name keeps DCL's status; SHOW TRANSLATION's
    layout is VMS's for every name (both logs show it only for
    process-permanent names); DEASSIGN SYS$OUTPUT at level 0 removes
    DCL's supervisor-mode name.
  - govax's COPY/HOST gives a copied procedure no record attributes,
    where VMS's EXCHANGE gave CR (the oracle rewrites VMS's RAT).
- **Status**: open, unconfirmed; each would need another VAX run.

