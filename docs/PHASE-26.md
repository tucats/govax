# Phase 26: Expanding system services

## Goal

Grow govax's set of emulated VMS system services beyond the handful ported from
eVAX (Phase 10) and the logical-name services (Phase 25), one service per
subtask. Many of these services act on "the calling process", so the phase
also builds up the **emulated process**: a record of process identity and
quota state (`rtl.Process`) that services read and update, in place of the
loose PID/UIC constants Phase 10 left behind.

This document is meant to be **extended as more services are added**. Each new
service gets:

- a row in the [service catalog](#service-catalog),
- a subsection under [Service designs](#service-designs),
- a numbered [subtask](#subtasks), and
- a [progress log](#progress-log) entry.

Later service work should start from the
[conventions](#conventions-for-implementing-a-service) below.

The first batch, requested by the user on 2026-09-27, is `$ADJSTK`,
`$ADJWSL`, `$ALLOC`, and `$ASCEFC`. The second adds `$DALLOC`,
`$DACEFC`/`$DLCEFC`, `$GETJPI`, and the event-flag waits. The third adds
`$DASSGN` and the timers; the fourth, the time conversions, hibernation,
and AST delivery. The fifth adds terminal `$QIO`, `$SYNCH`, exit
handlers, `$NUMTIM`, more `$GETJPI` items, mode-switching AST delivery,
and `$GETSYI`. The sixth adds `$FAO`/`$FAOL`, `$GETMSG`/`$PUTMSG`,
`$CMKRNL`/`$CMEXEC`, CTRL/C and CTRL/Y ASTs, the full `$GETDVI`,
mailboxes, and the small process-control services.

**Status: sixth batch in progress** (subtasks 1-23 done; 24-30 planned).
Add later services as new subtasks.

## References

eVAX implements none of these services (they're in its P1 vector, but
`call_service` reports them as unimplemented), so, as in Phases 22-25, the
correctness references are real VMS materials, not `reference/eVAX`:

- *VMS System Services Reference Manual*, VMS 5.0
  (`~/Documents/Technical Doc/VMS/AA-LA69A-TE_VMS_5.0_System_Services_Reference_Manual_198804.pdf`).
  It is the primary source for each service's arguments, behavior, and
  condition values. `pdftotext -layout` extracts it cleanly.
- Real VMS 7.3 definition sources under
  `~/Documents/Technical Doc/VMS/vmssrc_archive/v73/` (the same archive Phase 25
  took `lnmdef.sdl` and `ssdef.txt` from). Numeric constants come from these
  through `internal/vmsdef/gen`, not typed in by hand. The archive has the
  SDL/listing files, but not the MACRO-32 source of the services themselves
  (`SYSASCEFC` and friends aren't in it), so the manual remains the behavioral
  reference.
- `$SSDEF` status codes are already generated (`vmsdef.SSConstants`,
  Phase 25).

## Conventions for implementing a service

These are the patterns the existing services follow. New services should
follow them too, and this list should grow when a new pattern is settled.

- **Where the code lives.** Services are grouped by VMS facility into files
  under `internal/rtl`: `core.go` (event flags, `$EXPREG`, `$GETJPIW`, ...),
  `devices.go` (`$ASSIGN`, `$ALLOC`, `$DALLOC`, `$DASSGN`), `logicals.go`, `cli.go`,
  `rms.go`, `process.go` (process record and process-control services),
  `eventflags.go` (event flags and common event flag clusters), `timers.go`
  (the timer queue), `vmstime.go` (`$GETTIM`, `$NUMTIM`, and time conversion),
  `hibernate.go` (`$HIBER`, `$WAKE`, scheduled wakeups), `ast.go` (AST
  delivery), `qio.go` (`$QIO` and the driver registry), `ttdriver.go`
  (the terminal driver's functions), `exit.go` (`$EXIT` and exit
  handlers), `getsyi.go` (`$GETSYI`), `itemlist.go` (item-list walking
  and the item values the `$GETxxx` services return), `fao.go` (`$FAO`
  formatting, which later services reuse through `formatFAO`), `message.go`
  (`$GETMSG`, `$PUTMSG`), `cmode.go` (`$CMKRNL`, `$CMEXEC`), `ctrlast.go`
  (CTRL/C and CTRL/Y ASTs), `getdvi.go` (`$GETDVI`). Each file has
  a `register*Services(t *ServiceTable)` function called from
  `registerServices` in `service.go`.
- **Registration.** Every service is a `ServiceFunc` registered by its
  P1-vector name (`t.Register("SYS$ADJSTK", serviceSysAdjstk)`). Dispatch is
  table-driven, never a switch. `TestRegisteredServicesExistInP1Vector`
  checks the name is a real `vmsdef.P1VectorTable` entry.
- **Arguments.** `argv` holds the raw longwords of the call's argument list.
  - An optional trailing argument may be missing from the list entirely.
    Read it with `optArg(argv, i)`, which gives 0 for a missing argument, the
    same as an argument passed as 0. Index `argv` directly only for arguments
    the manual says are required. (`callHandler` recovers the panic an
    under-length list would cause, but that's a safety net, not the design.)
  - "By value" arguments are the longword itself. When the manual says only
    the low-order byte or word is used, mask it (`int16(argv[1])`).
  - "By reference" arguments are an address to load or store through
    `env.mem`. A store or load that fails returns `SS$_ACCVIO`.
  - "By descriptor" strings are read with `strGet`, and returned with
    `storeDescriptor` (which reports truncation, for `SS$_BUFFEROVF`).
- **Access modes.** The caller's mode is `env.cpu.PSL().CurMod()`. When a
  service takes an access-mode argument, it is "maximized": the less
  privileged of the argument and the caller's mode (the larger number) is
  used (`max(acmode&3, curMod)`).
- **Privileges.** govax has no privilege model. The emulated process holds
  every privilege, so privilege checks (`SYSNAM`, `PRMCEB`, `ALLSPOOL`, ...)
  always pass. Checks that aren't about privileges, such as access-mode
  ordering or a cluster's UIC protection, are still enforced. (Same rule as
  Phase 25's logical-name services.)
- **Status codes.** Return values come from `vmsdef.SSConstants` (the real
  `$SSDEF` numbers), cached in a package-level `var` next to the service, or
  from the older `ss*` constants in `status.go` for codes those already
  cover. A handler returns a Go `error` only for an emulator failure, never
  for a VMS condition: VMS conditions are the `uint32` R0 value.
- **Process and system state.** Anything that belongs to "the calling
  process" goes in `rtl.Process` (`env.Process`), with a comment naming the
  VMS field (PCB, PHD, JIB, UAF) it stands in for. System-wide state that
  VMS keeps in system memory (for example common event flag clusters,
  `env.EventFlagClusters`) belongs to the Environment, which INIT/VMINIT/
  ZERO rebuild, just as they wipe memory. Only state that must survive
  those (devices, logical names, mounts) is owned by the `Console` and
  injected into the Environment.
- **Waiting.** A service that must put the process in a wait state
  returns `rtl.ErrWait` while its condition isn't met. The engine then
  re-executes the service's `XFC` on the next step (see the event-flag
  wait design). `$HIBER` and `$SYNCH` use it too, and any later
  waiting service should.
- **Completing requests.** A service with an `efn`/`iosb`/`astadr`
  completion (`$GETJPI`, `$QIO`, ...) completes during the call: clear
  the flag and IOSB as it starts, then write the IOSB, set the flag, and
  `queueAST` in the caller's mode as it finishes. A request rejected
  before it starts returns its error in R0 and completes nothing (`$QIO`
  also sets the flag then, as its manual says). So the `...W` form is the
  same service.
- **I/O functions.** A `$QIO` function is an `ioFunc` in a driver's
  function table, keyed by the `$IODEF` function code, and a driver is
  found by device class in `ioDrivers`. A new device class adds a table;
  a new function adds an entry.
- **Calling guest code.** A service that needs a guest procedure called
  and then to continue (`$EXIT` and its handlers, `$PUTMSG`'s action
  routine, `$CMKRNL`'s routine) returns a
  `*CallRequest`: the engine calls the routine with the service's `XFC`
  as its return address, so the service runs again afterwards. Keep the
  progress in RTL state the service can pick up from. When the service
  runs again, it can tell a returning call from a new one by the frame
  pointer (the stub's, restored by the routine's `RET`) and the stack
  pointer (back where the call frame was built); keep calls in progress
  as a stack, so the routine can call the service again. (AST delivery,
  which interrupts rather than continues, uses `NextAST` instead.)
- **Picking a process.** Services that take the `[pidadr] ,[prcnam]` pair
  call `processTarget` (`getjpi.go`): it accepts only this process, writes
  its PID back to a `pidadr` holding 0, and returns `SS$_NONEXPR`,
  `SS$_IVLOGNAM`, or `SS$_ACCVIO` otherwise. Only `$GETJPI` passes
  `wildcard`.
- **Time.** Anything that needs "now" reads `env.Clock()` (VMS 64-bit
  time), which the console binds to the engine's `SystemTime`. Don't call
  `time.Now()` in a service: that would break determinism in quantum mode
  and disagree with the interval clock.
- **Image rundown.** Per-image cleanup (user-mode logical names, user-mode
  device allocations, common event flag associations, user-mode exit
  handlers, ...) runs from
  `Environment.ImageRundown`, which the console calls when an image started
  by RUN returns.
- **Tests.** Each service gets unit tests in `internal/rtl/*_test.go` that
  call the `ServiceFunc` directly, using the memory `arena` helper in
  `logicals_test.go` to lay out descriptors, longwords, and buffers. Cover
  the success path, each documented condition value the implementation can
  return, and omitted optional arguments.
- **Deviations.** A documented behavior that govax deliberately doesn't
  implement (quotas, shared memory, clusters, other processes) is listed in
  the service's design subsection and recorded in `docs/DEVIATIONS.md` under
  this phase's section.

## Service catalog

Every service this phase adds, in the order implemented. "Condition values"
lists the ones the implementation can actually return.

| Service | Subtask | File | Condition values | Notes |
| --- | --- | --- | --- | --- |
| (process record) | 1 | `process.go` | — | PID, username SYSTEM, UIC [1,4], working-set quotas. |
| `$ADJSTK` | 2 | `process.go` | `NORMAL`, `ACCVIO`, `NOPRIV` | Sets a less privileged mode's saved SP (`KSP`/`ESP`/`SSP`/`USP`). |
| `$ADJWSL` | 3 | `process.go` | `NORMAL`, `ACCVIO` | Adjusts `Process.WSLimit`, clamped to [`MINWSCNT`, `WSEXTENT`]; recorded, not enforced. |
| `$ALLOC` | 4 | `devices.go` | `NORMAL`, `BUFFEROVF`, `DEVALRALLOC`, `ACCVIO`, `DEVALLOC`, `DEVMOUNT`, `IVDEVNAM`, `IVLOGNAM`, `IVSTSFLG`, `NODEVAVL`, `NOSUCHDEV`, `TOOMANYLNAM` | Marks a device `DEV$M_ALL` with the process's PID and access mode; generic allocation by device type. |
| `$ASCEFC` | 5 | `eventflags.go` | `NORMAL`, `ACCVIO`, `ILLEFC`, `IVLOGNAM`, `NOPRIV` | Creates/associates a named common event flag cluster for flags 64-127; `$SETEF`/`$CLREF`/`$READEF` reach it. |
| `$DALLOC` | 6 | `devices.go` | `NORMAL`, `ACCVIO`, `DEVASSIGN`, `DEVNOTALLOC`, `IVLOGNAM`, `NOPRIV`, `NOSUCHDEV`, `TOOMANYLNAM` | Releases one allocation, or (no `devnam`) all at `acmode` or outer. |
| `$DACEFC` | 7 | `eventflags.go` | `NORMAL`, `ILLEFC` | Drops an association; an unassociated number still succeeds. |
| `$DLCEFC` | 7 | `eventflags.go` | `NORMAL`, `ACCVIO`, `IVLOGNAM` | Marks a cluster for deletion; deleted when unassociated. |
| `$GETJPI`, `$GETJPIW` | 8 | `getjpi.go` | `NORMAL`, `ACCVIO`, `BADPARAM`, `ILLEFC`, `INSFARG`, `IVLOGNAM`, `NOMOREPROC`, `NONEXPR`, `UNASEFC` | 21 item codes from `rtl.Process`, in a registry keyed by the generated `$JPIDEF` codes (28 since subtask 21). |
| `$WAITFR`, `$WFLAND`, `$WFLOR` | 9 | `eventflags.go` | `NORMAL`, `ILLEFC`, `UNASEFC` | Wait by re-executing the service's `XFC` until satisfied; timer interrupts run in between. |
| `$DASSGN` | 10 | `devices.go` | `NORMAL`, `IVCHAN`, `NOPRIV` | Releases a channel; image rundown releases user-mode channels. |
| `$SETIMR`, `$CANTIM` | 11 | `timers.go` | `NORMAL`, `ACCVIO`, `ILLEFC`, `UNASEFC` | RTL timer queue on the engine's system time (1 ms per interval-clock tick); no guest interrupt needed. |
| `$GETTIM` | 12 | `vmstime.go` | `NORMAL`, `ACCVIO` | The engine's system time, now local time as on VMS. |
| `$ASCTIM` | 13 | `vmstime.go` | `NORMAL`, `ACCVIO`, `BUFFEROVF`, `IVTIME` | Binary time to `dd-mmm-yyyy hh:mm:ss.cc` / `dddd hh:mm:ss.cc`. |
| `$BINTIM` | 13 | `vmstime.go` | `NORMAL`, `ACCVIO`, `IVTIME` | The reverse, with omitted fields defaulted as the manual describes. |
| `$HIBER` | 14 | `hibernate.go` | `NORMAL` | Waits (re-executing its `XFC`) until a wakeup is pending, then consumes it. |
| `$WAKE` | 14 | `hibernate.go` | `NORMAL`, `ACCVIO`, `IVLOGNAM`, `NONEXPR` | Sets `Process.WakePending`. |
| `$SCHDWK` | 14 | `hibernate.go` | `NORMAL`, `ACCVIO`, `IVLOGNAM`, `IVTIME`, `NONEXPR` | A wakeup entry on the `$SETIMR` timer queue, optionally repeating (10ms minimum). |
| `$CANWAK` | 14 | `hibernate.go` | `NORMAL`, `ACCVIO`, `IVLOGNAM`, `NONEXPR` | Removes queued wakeups only. |
| `$DCLAST` | 15 | `ast.go` | `NORMAL` | Queues an AST for the caller's mode or a less privileged one. |
| `$SETAST` | 15 | `ast.go` | `WASSET`, `WASCLR` | Per-mode AST enable, replacing eVAX's single recorded flag. |
| `$CLRAST` | 15 | `ast.go` | (restored R0) | The AST exit: an AST routine's `RET` returns through it. Not called directly. |
| (`$SETIMR`, `$GETJPI` ASTs) | 16 | `timers.go`, `getjpi.go` | — | `astadr` now queues an AST: a timer's with `reqidt`, `$GETJPI`'s with `astprm`. |
| `$QIO`, `$QIOW` | 17 | `qio.go`, `ttdriver.go` | `NORMAL`, `ACCVIO`, `ILLEFC`, `ILLIOFUNC`, `IVCHAN`, `NOPRIV`, `UNASEFC`; IOSB: `NORMAL`, `ENDOFFILE`, `TIMEOUT` | Terminal reads (plain and prompted), writes with carriage control, sense/set mode; completes during the call. |
| `$CANCEL` | 17 | `qio.go` | `NORMAL`, `IVCHAN`, `NOPRIV` | Checks the channel; nothing is ever outstanding. |
| `$SYNCH` | 18 | `eventflags.go` | `NORMAL`, `ACCVIO`, `ILLEFC`, `UNASEFC` | Waits for the flag and a nonzero IOSB status, clearing false alarms. |
| `$DCLEXH` | 19 | `exit.go` | `NORMAL`, `ACCVIO`, `IVSSRQ`, `NOHANDLER` | Per-mode exit handler lists, linked in memory; replaces eVAX's recording stub. |
| `$CANEXH` | 19 | `exit.go` | `NORMAL`, `ACCVIO`, `IVSSRQ`, `NOHANDLER` | Removes one block, or all of the mode's. |
| `$EXIT` | 19 | `exit.go` | (none: doesn't return) | Calls the mode's handlers via `cpu.ServiceCall`, then unwinds to the console's call frame. RUN's driver calls it with `main`'s status. |
| `$NUMTIM` | 20 | `vmstime.go` | `NORMAL`, `ACCVIO`, `IVTIME` | Seven numeric fields; a delta's year and month are 0. |
| (`$GETJPI` items) | 21 | `getjpi.go` | — | `JPI$_ASTACT`, `ASTEN`, `ASTCNT`, `ASTLM`, `PRI`, `PRIB`, `STATE`; `Process` gains `ASTLimit` and priorities. |
| (AST delivery) | 22 | `ast.go` | — | An inner-mode AST interrupts outer-mode code by switching mode and stack; `$CLRAST` switches back. |
| `$GETSYI`, `$GETSYIW` | 23 | `getsyi.go` | `NORMAL`, `ACCVIO`, `BADPARAM`, `ILLEFC`, `INSFARG`, `IVLOGNAM`, `NOMORENODE`, `NOSUCHNODE`, `UNASEFC` | 10 items: version, node name, SID/CPU, boot time, cluster membership, `MINWSCNT`; this node only. |
| `$FAO`, `$FAOL` | 24 | `fao.go` | `NORMAL`, `BUFFEROVF`, `ACCVIO`, `BADPARAM` | The manual's directives plus VMS 7's `A`/`I`/`H`/`J`/`Q` sizes; a registry of directive functions. |
| `$GETMSG` | 25 | `message.go` | `NORMAL`, `BUFFEROVF`, `MSGNOTFND`, `ACCVIO`, `INSFARG` | Texts of 1,426 messages (CLI, LIB, MTH, OTS, RMS, SYSTEM) generated from the VMS 7.3 message file. |
| `$PUTMSG` | 25 | `message.go` | `NORMAL`, `ACCVIO` | Formats a message vector with `formatFAO`; an action routine is called through a `CallRequest`. |
| `$CMKRNL`, `$CMEXEC` | 26 | `cmode.go` | (the routine's R0) | Switch into the mode, call the routine through a `CallRequest`, switch back when it returns. |
| `$GETDVI`, `$GETDVIW` | 28 | `getdvi.go` | `NORMAL`, `ACCVIO`, `BADPARAM`, `ILLEFC`, `INSFARG`, `IVDEVNAM`, `IVLOGNAM`, `NOPRIV`, `NOSUCHDEV`, `UNASEFC` | 39 named items plus 28 `DEVCHAR` and 50 terminal-characteristic Booleans, from a generated `$DVIDEF`; replaces eVAX's three-item `$GETDVIW`. |
| (CTRL/C, CTRL/Y ASTs) | 27 | `ctrlast.go`, `ttdriver.go` | — | `IO$_SETMODE!IO$M_CTRLCAST`/`CTRLYAST` enable one-shot ASTs the host's Ctrl-C delivers instead of stopping the machine. |

## Service designs

### The emulated process (`rtl.Process`)

Before this phase, `Environment` had unexported `pid`/`uic` fields, set from
constants and read only by `$ASSIGN` to stamp a device's owner. They're
replaced by `Environment.Process *Process` (`internal/rtl/process.go`):

| Field | Stands in for | Default |
| --- | --- | --- |
| `PID` | `PCB$L_EPID` | `0x00000301` (arbitrary, nonzero) |
| `Username` | `JIB$T_USERNAME` | `SYSTEM` |
| `Name`, `Account`, `Terminal`, `CLIName` | process name, UAF account, login terminal, CLI (added in subtask 8) | `SYSTEM`, `SYSTEM`, `TTA0:`, `DCL` |
| `UIC` | `PCB$L_UIC` | `[1,4]` (`0x00010004`) |
| `WSLimit` | the current working-set limit (kept in the process header) | `WSDefault` |
| `WSDefault` / `WSQuota` / `WSExtent` | UAF `WSDEFAULT`/`WSQUOTA`/`WSEXTENT` | 150 / 256 / 1024 pages |
| `MinWSCount` | SYSGEN `MINWSCNT` | 20 pages |

The working-set numbers are nominal: in the range VMS 5 used for the SYSTEM
account, not copied from a real UAF. govax doesn't page, so nothing enforces
them.

A `Process` is built by `NewEnvironment`, so INIT/VMINIT/ZERO start a fresh
process, the emulated equivalent of logging in again. `rtl.NominalUIC` stays
exported: the console names the group logical-name table from it before an
Environment exists.

### `$ADJSTK` — Adjust Outer Mode Stack Pointer

`SYS$ADJSTK [acmode] ,[adjust] ,newadr`

Modifies the stack pointer of an access mode **less privileged** than the
caller's. VMS uses it to fix up an outer mode's stack after pushing arguments
onto it.

- `acmode` (by value, default kernel) is maximized with the caller's mode.
  If the result is the caller's own mode, the call fails with `SS$_NOPRIV`
  (the manual: "equal to or more privileged than the calling access mode").
  So from user mode the service always fails, and from kernel mode the
  default `acmode` does too.
- `adjust` is by value, but only its low-order word is used, as a signed
  value.
- `newadr` (by reference, required) is read, adjusted, written back, and
  loaded as the mode's stack pointer. When the longword it points to is 0,
  the mode's current stack pointer is adjusted instead. That covers all four
  rows of the manual's `adjust`/`newadr` table; in every case the result is
  written back to `newadr`.
- The target mode is never the one executing, so its stack pointer is the
  saved copy in the `KSP`/`ESP`/`SSP`/`USP` privileged register
  (`env.cpu.PR`), which the CPU loads the next time it enters that mode
  (`Engine.setModeStack`). The live `SP` is never touched.
- `newadr` of 0 is `SS$_ACCVIO` explicitly (page 0 is never accessible on
  VMS, but tests run with virtual memory off, where it would be).

Not implemented: the manual's `SS$_ACCVIO` for "a portion of the new stack
segment cannot be written by the caller". govax doesn't probe the new stack,
only `newadr` itself.

govax runs RUN images in the console's own mode, kernel by default, so a
program can use `$ADJSTK` on any of the three outer modes.

### `$ADJWSL` — Adjust Working Set Limit

`SYS$ADJWSL [pagcnt] ,[wsetlm]`

Adjusts the process's working-set limit by `pagcnt` pages (a signed
longword, by value) and returns the new limit through `wsetlm` (by
reference, optional).

- The limit is `Process.WSLimit`, which starts at `WSDefault`.
- A result above `WSExtent` or below `MinWSCount` is clamped there with no
  error, as the manual says.
- `pagcnt` 0 (or omitted) changes nothing, and `wsetlm` receives the current,
  unadjusted limit.
- If `wsetlm` can't be written the call returns `SS$_ACCVIO` and the limit
  is left unchanged: the store happens before the new limit is committed.

**The limit is recorded but not enforced.** govax has no paging or working
set, so the value only matters to code that adjusts or reads it back. That's
the purpose the user gave for this service: completeness, and documenting
what VAX code does. A later `$GETJPI` could report `JPI$_WSEXTENT` and
friends from the same fields.

### `$ALLOC` — Allocate Device

`SYS$ALLOC devnam ,[phylen] ,[phybuf] ,[acmode] ,[flags]`

Allocates a device for the calling process's exclusive use.

**Allocation state** lives on the device record (`iodev.Device`), where VMS
keeps it in the UCB:

- `DevChar`'s `DEV$M_ALL` bit marks the device allocated
  (`Device.Allocated()`).
- `Device.PID` is the owning process. `$ASSIGN` already stamped it (eVAX
  behavior), so it's only an allocation owner while `DEV$M_ALL` is set.
- `Device.AllocMode` (new, standing in for `UCB$B_AMOD`) is the allocation's
  access mode.
- `Device.Allocate(pid, mode)` / `Deallocate()` set and clear them, for
  `$ALLOC` now and `$DALLOC` later.

The `DEV$` bits come from a new generated map, `vmsdef.DEVConstants`, parsed
from the real VMS 7.3 `$DEVDEF` SDL source (`reference/vms/devdef.sdl`). The
SDL parser in `internal/vmsdef/gen` gained `aggregate ... union` support for
it: `$DEVDEF` is a union of two bitfield structures (`DEVCHAR` and
`DEVCHAR2`), each numbered from bit 0.

**The service:**

- `devnam` (by descriptor, required) is a physical device name or a logical
  name. It's translated like `$ASSIGN`'s (`env.deviceName`: logical names,
  a leading `_` to suppress translation). A missing descriptor is
  `SS$_IVDEVNAM`; an empty name or one over 63 characters is
  `SS$_IVLOGNAM`; an unknown device is `SS$_NOSUCHDEV`.
- `acmode` (by value) is maximized with the caller's mode and recorded in
  `AllocMode`. RUN images run in the console's mode (kernel by default), so
  by default an allocation is kernel mode and lasts until the device is
  explicitly deallocated.
- `flags` bit 0 is generic allocation: `devnam` names a device type
  (`RA81`, compared case-blind with `iodev.DeviceTypeName`), and the first
  unallocated, unmounted device of that type, by name, is allocated. A
  device the process already holds doesn't count as available. Types with
  no available device are `SS$_NODEVAVL`; unknown types `SS$_NOSUCHDEV`. Any
  other flag bit is `SS$_IVSTSFLG`.
- A device already allocated to this process succeeds with
  `SS$_DEVALRALLOC`. One allocated to another PID fails with
  `SS$_DEVALLOC`. A mounted device (`DEV$M_MNT`, or a volume in the
  `MountTable`) or a mailbox (`DEV$M_MBX`) fails with `SS$_DEVMOUNT`.
- The physical name, `_` + device + `:` (`_TTA0:`), goes to `phybuf` (by
  descriptor) and its length to `phylen` (by reference), both optional. A
  name cut short by a small buffer returns `SS$_BUFFEROVF`, still a success:
  the device is allocated.

**Related changes:**

- `$ASSIGN` now returns `SS$_DEVALLOC` for a device allocated to another
  process, instead of assigning a channel and taking over its PID.
- **Image rundown.** `Environment.ImageRundown` (new, in `process.go`)
  deallocates this process's user-mode allocations. The console's
  `imageRundown`, which already deleted user-mode logical names when a RUN
  image returns, now calls it.
- **SHOW DEVICE/FULL** reports an allocated device: the disk header reads
  `Disk DKA0:, is online, allocated, ...` as on VMS, and other devices'
  headers read `Device TTA1, allocated`.

With a single process, "allocated to another process" can only come from
state set directly (as the tests do). The check is in place for when there
are more processes. Deliberate gaps (`SS$_DEVOFFLINE`, spooled devices,
template devices, and so on) are listed in `docs/DEVIATIONS.md`.

### `$ASCEFC` — Associate Common Event Flag Cluster

`SYS$ASCEFC efn ,name ,[prot] ,[perm]`

Associates a named common event flag cluster with cluster number 2 or 3,
creating the cluster if it doesn't exist.

**Event flag model.** A process has 128 event flags in four 32-flag
clusters:

| Cluster | Flags | Where govax keeps it |
| --- | --- | --- |
| 0, 1 (local) | 0-63 | `Process.LocalEventFlags[0..1]` |
| 2, 3 (common) | 64-127 | `Process.CommonClusters[0..1]`: a pointer to the associated `EventFlagCluster`, or nil |

`EventFlagCluster` (the VMS common event block) has a `Name`, the creator's
UIC `Group` (names are unique per group), `CreatorUIC`, `Protected`,
`Permanent`, its 32 `Flags`, and a reference count. The clusters themselves
are in `CommonEventFlags`, a system-wide table keyed by (group, name), at
`Environment.EventFlagClusters`. On VMS clusters live in system memory, so
the table belongs to the Environment: INIT/VMINIT/ZERO wipe it along with
memory, and a permanent cluster survives from one RUN to the next.

**The service** (`internal/rtl/eventflags.go`):

- `efn` (by value, low byte only) is any flag in the target cluster: 64-95
  for cluster 2, 96-127 for cluster 3. Anything else is `SS$_ILLEFC`.
- `name` (by descriptor, required) is 1-15 characters, else
  `SS$_IVLOGNAM`. A missing descriptor is `SS$_ACCVIO`. Names are
  compared exactly (case-sensitive), and implicitly qualified by the
  process's UIC group.
- If the cluster doesn't exist it is created with all flags clear. `prot`
  (low bit) makes it usable only by the creator's UIC; `perm` (low bit)
  makes it permanent. Both only matter at creation. Creating a permanent
  cluster needs `PRMCEB`, which the emulated process always has.
- An existing protected cluster refuses a process with a different UIC:
  `SS$_NOPRIV`.
- If the cluster number is already associated with another cluster, that
  association is dropped first. Associating the cluster it already has is
  a no-op (disassociating first could delete a temporary cluster and
  recreate it empty).
- Each association adds a reference. Dropping the last reference to a
  temporary cluster deletes it; permanent clusters stay until deleted
  (`$DLCEFC`, subtask 7).
- **Image rundown** (`Environment.ImageRundown`) disassociates both cluster
  numbers: associations last "for the execution of the current image".

**`$SETEF`, `$CLREF`, `$READEF`** moved from `core.go` to `eventflags.go`
and now follow the manual, since common clusters would be unreachable
otherwise:

- Flags 64-127 go to the associated common cluster. If the cluster number
  isn't associated, the result is `SS$_UNASEFC`. (eVAX kept flags 64-127 in
  process-local storage.)
- `efn` uses its low byte; above 127 is `SS$_ILLEFC`. (eVAX's `% 0xFF`
  let flags 128-254 through and indexed past its four-longword array.)
- `$SETEF` and `$CLREF` return `SS$_WASSET`/`SS$_WASCLR` for the flag's
  previous state. (eVAX always returned `SS$_NORMAL`, which is numerically
  `SS$_WASCLR`, so only the "was set" case changed.)
- `$READEF` writes the cluster's 32 flags to `state` when given (the manual
  requires it; eVAX allowed omitting it, which still works) and reports the
  flag itself as `WASSET`/`WASCLR`.

These changes are recorded in `docs/DEVIATIONS.md`.

Not implemented: `SS$_EXQUOTA` (no `TQELM` quota), and the multiport
shared-memory statuses (`SS$_EXPORTQUOTA`, `SS$_INTERLOCK`,
`SS$_NOSHMBLOCK`, `SS$_SHMNOTCNCT`), since govax has no shared memory.
`SS$_INSFMEM` can't happen. With a single process, the only way to see
another UIC's view is to change `Process.UIC`, which the tests do.

### `$DALLOC` — Deallocate Device

`SYS$DALLOC [devnam] ,[acmode]`

The inverse of `$ALLOC`, using the same `iodev.Device` allocation state.

- `acmode` (by value) is maximized with the caller's mode. An allocation
  can be released only from its own mode or a more privileged one: if
  `Device.AllocMode` is more privileged than the maximized mode, the call
  fails with `SS$_NOPRIV`.
- `devnam` (by descriptor) is translated like `$ALLOC`'s. An empty name, or
  one over 63 characters, is `SS$_IVLOGNAM`.
  - An unknown device is `SS$_NOSUCHDEV`. The manual doesn't list that
    code for `$DALLOC`, but it's what `$ALLOC` returns for the same case.
  - A mailbox succeeds without doing anything, as the manual says.
  - A device that isn't allocated to this process is `SS$_DEVNOTALLOC`.
  - A device the process still has a channel to stays allocated:
    `SS$_DEVASSIGN`. govax has no `$DASSGN` yet, so this holds for any
    device the process has `$ASSIGN`ed.
- With `devnam` omitted, every device the process allocated in the
  maximized mode or a less privileged one is released, skipping any with a
  channel assigned, and the call succeeds.

Image rundown's user-mode deallocation (subtask 4) is unchanged. It is the
same operation as `$DALLOC` with no `devnam` and `acmode` user.

### `$DACEFC` and `$DLCEFC` — Disassociate and Delete Common Event Flag Cluster

`SYS$DACEFC efn` and `SYS$DLCEFC name`

They complete the cluster lifecycle `$ASCEFC` started (`eventflags.go`).

- **`$DACEFC`** drops the process's association with the cluster holding
  `efn`. `efn` uses its low byte; outside 64-127 it is `SS$_ILLEFC`. A
  cluster number with no association succeeds, as the manual says.
- **`$DLCEFC`** marks the named cluster in the process's UIC group for
  deletion (`EventFlagCluster.DeletePending`). It doesn't disassociate
  anyone. The name is read like `$ASCEFC`'s (`clusterName`: 1-15
  characters, else `SS$_IVLOGNAM`; `SS$_ACCVIO` for a missing
  descriptor). A cluster that doesn't exist still succeeds.
- **Deletion rule** (`CommonEventFlags.deleteIfUnused`): a cluster is
  deleted when it has no associations and is either temporary or marked for
  deletion. The check runs whenever an association is dropped (`$DACEFC`,
  reassociation, image rundown) and when `$DLCEFC` marks a cluster, so an
  unused permanent cluster goes at once.
- The manual requires `PRMCEB` or the creator's UIC to delete. The emulated
  process always holds `PRMCEB`, so `SS$_NOPRIV` never happens.
- A marked cluster can still be found and associated with by `$ASCEFC`
  until it is actually deleted. The manual doesn't say otherwise.

### `$GETJPI` / `$GETJPIW` — Get Job/Process Information

`SYS$GETJPI[W] [efn] ,[pidadr] ,[prcnam] ,itmlst ,[iosb] ,[astadr] ,[astprm]`

Replaces the eVAX-derived `$GETJPIW`, which knew two hard-coded items
(`ACCOUNT` = `"USER"`, `CLINAME` = `"DCL"`), with a real `$GETJPI` reading
`rtl.Process` (`internal/rtl/getjpi.go`). Both names are registered to the
same function.

**`$JPIDEF`.** The item codes come from the real VMS 7.3 `jpidef.sdl`
(`reference/vms/`), generated into `vmsdef.JPIConstants` (219 entries). The
SDL parser learned three more forms for it:

- `%x` hexadecimal values;
- `NAME@N`, an earlier constant shifted left `N` bits (`$JPIDEF` numbers
  each item list from `JPI$C_PCBTYPE@8` and so on);
- a bitfield structure nested in a structure aggregate
  (`JPICTLFLGS structure longword unsigned fill;`), continuing the bit
  count.

`prefix JPI tag $C` already composed `JPI$C_...` correctly. The generated
`JPI$_ACCOUNT` (515) and `JPI$_CLINAME` (522) match the numbers eVAX
hard-coded.

**Items** are a registry (`jpiItemsByName`, keyed by `$JPIDEF` name and
turned into a code-keyed map at startup), not a switch:

| Item | Returns |
| --- | --- |
| `ACCOUNT` | `Process.Account`, 8 bytes blank-padded |
| `USERNAME` | `Process.Username`, 12 bytes blank-padded |
| `PRCNAM`, `TERMINAL`, `CLINAME` | `Process.Name`, `.Terminal`, `.CLIName` |
| `PID`, `MASTER_PID` | `Process.PID` (the process is its own job's master) |
| `OWNER` | 0 (not a subprocess) |
| `UIC`, `GRP`, `MEM` | `Process.UIC` and its halves |
| `MODE`, `JOBTYPE` | `JPI$K_INTERACTIVE`, `JPI$K_LOCAL` (a console login) |
| `EFCS`, `EFCU` | local event flag clusters 0 and 1 |
| `DFWSCNT`, `WSQUOTA`, `WSEXTENT` | `WSDefault`, `WSQuota`, `WSExtent` |
| `WSAUTH`, `WSAUTHEXT` | `WSQuota`, `WSExtent` (authorized = current quotas) |
| `WSSIZE` | `WSLimit`, the limit `$ADJWSL` adjusts |

Any other item code is `SS$_BADPARAM`, which stops the list. Data is
truncated to the buffer length (a longword is stored low byte first), and
the length actually written goes to the return-length word. `JPI$_CHAIN`
continues with another item list (the `walkItemListChain` Phase 25 added
for `LNM$_CHAIN`).

**Picking the process**, per the manual's Table SYS-5:

- `pidadr` holding a nonzero PID uses that PID, and `prcnam` is ignored.
  The process's own PID works; any other is `SS$_NONEXPR`.
- Otherwise `prcnam`, when given, must match `Process.Name` exactly (no
  abbreviation or case folding). A mismatch is `SS$_NONEXPR`; an empty name
  or one over 15 characters is `SS$_IVLOGNAM`.
- With neither, the caller is used.
- When `pidadr` holds 0, the PID found is written back.
- **Wildcard:** `-1` at `pidadr` returns this process and leaves govax's
  "scan finished" context (`0xFFFFFFFE`) there. The next call with that
  context returns `SS$_NOMOREPROC`. VMS keeps its own scan position in that
  longword, and programs don't interpret it.

**Completion.** The request completes immediately, so `$GETJPI` and
`$GETJPIW` behave identically:

1. `efn` (default 0) is cleared, and a bad or unassociated flag number is
   returned at once (`SS$_ILLEFC`/`SS$_UNASEFC`).
2. `iosb`, if given, is zeroed.
3. The items are returned.
4. The final status goes into the IOSB's first longword, the event flag is
   set, and the status is returned in R0.

An argument list shorter than 7 is `SS$_INSFARG`, as before. `astadr`
was accepted but ignored until subtask 16; now the completed request
queues an AST for it with `astprm` (see that subtask's design).

Not implemented: items for state govax doesn't model (`STATE`, `PRI`,
`PRIB`, `IMAGNAME`, quotas other than working set, privileges, CPU and I/O
counts, and so on), which are `SS$_BADPARAM`. There's also no
`SS$_NOPRIV` or `SS$_SUSPENDED`, since no other process exists. These are
in `docs/DEVIATIONS.md`.

### `$WAITFR`, `$WFLAND`, `$WFLOR` — Wait for Event Flags

`SYS$WAITFR efn`, `SYS$WFLAND efn ,mask`, `SYS$WFLOR efn ,mask`

`$WAITFR` waits for one flag. `$WFLAND` waits for all the flags `mask`
selects in `efn`'s cluster, and `$WFLOR` for any of them. `efn` uses its
low byte and is checked like `$SETEF`'s: `SS$_ILLEFC`, or `SS$_UNASEFC`
for an unassociated common cluster. When the condition already holds,
each returns `SS$_NORMAL` at once. An empty `$WFLAND` mask is satisfied
immediately; an empty `$WFLOR` mask never is, so that process waits for
good, as on VMS.

**Waiting.** A service runs to completion inside the `XFC` instruction of
its P1-vector stub, so it has no way to block. Instead, when the condition
isn't met:

1. The service returns `rtl.ErrWait`, not a status.
2. The console translates it to `cpu.ErrServiceWait` (as it does
   `rtl.ErrHalt` → `cpu.ErrHalted`).
3. `emulXfcP1Vector` sets PC back to the `XFC` (`e.instructionPC`) and
   leaves R0 alone.
4. The next `Step` executes the `XFC` again, calling the service again.

So the process waits in emulated time, re-checking once per instruction
step, with the `CALLS` frame and argument list untouched. Because each
retry is an ordinary instruction boundary, **interrupts are delivered
between retries**. The user confirmed the interval timer is govax's only
truly asynchronous source. A timer interrupt handler that sets the flag
ends the wait: the handler returns (`REI`) to the `XFC`, and the next
retry succeeds. Console attention (^C) and the `instruction-limit`/
`time-limit` options still stop a wait that can never end. A future
`$SETIMR` would set flags from the same timer path.

`DEBUG(SERVICES)` traces only a wait's first attempt (`..., waits`) and its
completion (`..., returns 00000001`), not every retry
(`Environment.waitingPC`).

Not implemented: the process state (`LEF`/`CEF`) and `JPI$_EFWM` wait
mask; and giving up the host CPU while waiting, since each retry costs an
emulated instruction step. (VMS's "wait interrupted by an AST, then
resumed" came with AST delivery in subtask 15: each retry is a point
where an AST can be delivered.)

### `$DASSGN` — Deassign I/O Channel

`SYS$DASSGN chan`

Releases a channel `$ASSIGN` created (`internal/rtl/devices.go`).

- `$ASSIGN` now records each channel's access mode (`channel.Mode`: its
  `acmode` argument maximized with the caller's mode). Until now it
  ignored `acmode`.
- `chan` uses its low word. 0 is `SS$_IVCHAN`. A channel that isn't
  assigned, or was assigned from a more privileged mode than the caller's,
  is `SS$_NOPRIV`, as the manual lists.
- Releasing a channel removes it and drops the device's `RefCnt`. Once
  nothing references an unallocated device, its owner `PID` (which
  `$ASSIGN` stamped) is cleared too. govax channels carry no I/O requests,
  open files, mailboxes, or network links, so there's nothing else to
  cancel or close.
- **Image rundown** now deassigns user-mode channels first. Then it
  deallocates user-mode allocations using exactly `$DALLOC`'s no-name rule
  (`deallocateAll`), so a device the process still has a more privileged
  channel to stays allocated. Before this subtask, rundown released
  user-mode allocations unconditionally. That only differed when a kernel
  channel was held, which the new tests exercise.

With `$DASSGN` in place, `$DALLOC`'s `SS$_DEVASSIGN` is no longer
permanent: deassign the channel, then deallocate.

### `$SETIMR` and `$CANTIM` — Set and Cancel Timer

`SYS$SETIMR [efn] ,daytim ,[astadr] ,[reqidt] ,[flags]` and
`SYS$CANTIM [reqidt] ,[acmode]`

#### The design question: the microkernel's timer interrupt, or the RTL's own?

The user asked whether `$SETIMR` should use the timer interrupt the
microkernel already runs, or whether the RTL timer should be independent.
What already existed:

- **The interval clock** (`ICR`/`NICR`/`ICCS`) is ticked by the engine
  (`tickIntervalClock`). By default a tick is taken every quantum of
  instructions (`tickQuantum`, 20 instructions), which is deterministic.
  With `vax.hardware.clock` set, a tick is taken every wall-clock
  millisecond.
- **The microkernel's timer interrupt** is guest VAX code: kernel.asm's
  `exe$interval`, installed with `.SCB exc$interval`. It runs only when
  the guest has set ICCS's RUN and IE bits (kernel.asm's `exe$initialize`
  does), and only at an IPL below 22.
- **No emulated notion of "now" existed.** `DECC$TIME` reads the host
  clock.

**Decision: the RTL owns its timer queue, and shares the engine's clock
but not the guest's interrupt path.**

- **Why not the guest interrupt.** On VMS the timer queue belongs to the
  executive. The hardware clock interrupt drives the executive's software
  timer, and user processes never arrange for it. In govax the executive
  is the Go RTL; kernel.asm is guest code with its own ideas about the
  clock. If `$SETIMR` depended on the guest's interrupt, a timer would
  silently never fire:
  - in any program that didn't boot kernel.asm, or that left ICCS
    interrupts off;
  - in the default RUN setup, which runs in kernel mode at whatever IPL
    the console has, where an IPL-22 clock interrupt may be masked.

  It would also need a new guest-to-Go hook in `exe$interval`. The
  services would then depend on how one particular guest kernel is
  written.
- **Why share the clock.** A timer must mean the same thing as the
  interval clock, and it must stay deterministic in quantum mode so tests
  and reruns behave identically. So the engine now has a **system time**
  (`Engine.SystemTime`, `internal/cpu/systime.go`), in VMS 64-bit format,
  driven by exactly what drives the interval clock: **one tick is one
  millisecond** in both modes.
  - In hardware-clock mode it is the wall-clock time.
  - In quantum mode it is the engine's creation time plus one
    millisecond per quantum tick. At the default quantum of 20, that's 20
    instructions per emulated millisecond. `SET QUANTUM 0` makes every
    step a tick.

  The console binds `Environment.Clock` to it. Unit tests substitute a
  hand-advanced clock. `vmsdef.Time`/`vmsdef.UnixEpoch` convert Go time
  to VMS time for both packages.
- **When timers fire.** Expired timers are processed at the one choke
  point every event-flag service passes through (`eventFlagWord` →
  `expireTimers`), not by a new engine-to-RTL tick hook. Event flags are
  only observable through services, so this can't be told apart from
  expiring on the tick itself. A waiting process retries its `$WAITFR`
  every instruction step, so it sees its timer on the first check after
  expiry. Two things would need an eager hook: AST delivery, and a
  console display of flags, which could simply call the same function.
  (AST delivery added that hook in subtask 15: `NextAST` expires timers
  before every instruction.)

The two mechanisms coexist. A guest that runs its own interval-timer
interrupt still does (`wait_timer.asm`), and RTL timers work whether or
not it does (`timer_services.asm`).

#### The services (`internal/rtl/timers.go`)

- **`$SETIMR`** checks `efn` (default 0; `SS$_ILLEFC`/`SS$_UNASEFC` as for
  `$SETEF`), reads the `daytim` quadword (`SS$_ACCVIO` if unreadable or 0),
  clears the flag, and queues a timer:
  - A **negative** `daytim` is a delta from now.
  - A **positive** one is an absolute time; if it has already passed,
    the timer fires at the next check.
  - `reqidt` and the caller's access mode are recorded for `$CANTIM`.
- When a timer expires it sets its flag. If the flag is in a common
  cluster the process has since disassociated, the timer is dropped with
  no effect.
- **`$CANTIM`** cancels the requests with ID `reqidt` (all of them when
  it's 0) that were made from `acmode`, maximized with the caller's mode,
  or from a less privileged mode. It always returns `SS$_NORMAL`.
- **Image rundown** cancels all outstanding timers, as the manual says.
- `astadr` was accepted but ignored until subtask 16, which delivers it.

Not implemented (see `docs/DEVIATIONS.md`):

- The CPU-time flag: a govax process's CPU time is its elapsed time, so
  the flag changes nothing.
- The `TQELM` quota (`SS$_EXQUOTA`).

### `$GETTIM` — Get Time

`SYS$GETTIM timadr`

Stores the current system time, `env.Clock()`, in the quadword at
`timadr` (`internal/rtl/vmstime.go`). A zero or unwritable `timadr` is
`SS$_ACCVIO`. The clock is the one `$SETIMR` uses: the engine's
`SystemTime`, one millisecond per interval-clock tick.

**System time is now local time.** VMS keeps its clock at local wall-clock
time, with no time zone. `vmsdef.Time` used to convert Go times as UTC; it
now adds the time's zone offset, so the engine's `SystemTime` (and the host
clock fallback, `wallClock`) read local time. `$GETTIM` followed by
`$ASCTIM` (subtask 13) then prints what a clock on the wall says. Delta
times and timers are unaffected. `vmsdef.GoTime` is the inverse, for
formatting.

Not implemented: VMS updates the clock every 10ms, so its times are
multiples of 100,000 ticks. govax's clock has 1ms steps and isn't rounded.

### `$ASCTIM` and `$BINTIM` — Convert Between Binary and ASCII Time

`SYS$ASCTIM [timlen] ,timbuf ,[timadr] ,[cvtflg]` and
`SYS$BINTIM timbuf ,timadr`

A VMS time is a signed quadword of 100ns ticks. A positive value is an
absolute time since 17-Nov-1858; a negative one is a delta (an interval).
The two text forms are:

| Kind | Form | Length | Example |
| --- | --- | --- | --- |
| Absolute | `dd-mmm-yyyy hh:mm:ss.cc` | 23 | ` 9-OCT-1988 07:05:03.45` |
| Delta | `dddd hh:mm:ss.cc` | 16 | `   5 03:18:32.07` |

The conversions are pure functions in `internal/rtl/vmstime.go`
(`formatVMSTime`, `parseVMSTime`), so the manual's example table is tested
directly. The services are thin wrappers.

**`$ASCTIM`** formats the quadword at `timadr`, or the current time when
`timadr` is 0.

- The day of the month is padded to two characters, and a delta's day
  count to four, with blanks. Month names are upper case. Hundredths are
  truncated.
- `cvtflg` bit 0 returns only `hh:mm:ss.cc`.
- The text goes to the `timbuf` descriptor's buffer, and its length to the
  word at `timlen`. A shorter buffer gets what fits, with the success
  status `SS$_BUFFEROVF`. That's how the manual's table gets the date
  alone: a 12-byte buffer.
- A delta of 10,000 days or more is `SS$_IVTIME`. So is an absolute time
  past the year 9999, which doesn't fit `yyyy` (the manual doesn't say).

**`$BINTIM`** parses the `timbuf` string into the quadword at `timadr`:

- Leading blanks, and any blanks between the two fields, are allowed.
  There can be none inside a field.
- A first field containing a hyphen makes the time absolute. Any omitted
  date or time field takes the current value (`-- :50` is today at
  `hh:50:ss.cc` of now). Leading fields need their punctuation; trailing
  ones can be dropped.
- Otherwise it's a delta. With two fields the first is the day count;
  with one, it's the time (`05` is five hours, per the manual's example).
  Omitted time fields are 0. The result is negated.
- The fraction is a true fraction (`.1` is ten hundredths). A third digit
  rounds, carrying into the seconds if need be; later digits are ignored.
- Months must be upper case. Field ranges are checked (hours 0-23, a day
  that exists in its month, years 1858-9999, days 0-9999, and not before
  17-Nov-1858). Anything else is `SS$_IVTIME`.

Both return `SS$_ACCVIO` for an argument they can't read or write. The
manual says those raise an access violation instead; govax returns the
status, as its other services do.

One of the manual's examples, `--1989 0:0:0.0` giving `29-DEC-1989`, is
taken as a typo. By its own rule the omitted day is today's, so govax gives
`30-DEC-1989`.

### `$HIBER`, `$WAKE`, `$SCHDWK`, `$CANWAK` — Hibernation and Wakeups

`SYS$HIBER`, `SYS$WAKE [pidadr] ,[prcnam]`,
`SYS$SCHDWK [pidadr] ,[prcnam] ,daytim ,[reptim]`,
`SYS$CANWAK [pidadr] ,[prcnam]`

Hibernation is one flag per process, **wake pending**
(`Process.WakePending`, VMS's `PCB$V_WAKEPEN`), in
`internal/rtl/hibernate.go`:

- **`$WAKE`** sets it.
- **`$SCHDWK`** sets it later, from the timer queue.
- **`$HIBER`** waits until it's set, then clears it and returns
  `SS$_NORMAL`. If it's already set, `$HIBER` returns at once.

It's a flag, not a count: two wakeups before a `$HIBER` end only that one.

**Waiting.** `$HIBER` returns `ErrWait` while nothing is pending, so it
waits the way `$WAITFR` does: the engine re-executes its `XFC` every step,
with interrupts delivered in between. Each retry expires due timers first,
so a scheduled wakeup ends the wait on the first step at or after its
time. Once ASTs exist (subtask 15), an AST routine that calls `$WAKE` ends
a `$HIBER`, as on VMS.

**The target process.** All but `$HIBER` take `[pidadr] ,[prcnam]`, read
by `processTarget` (factored out of `$GETJPI`'s `jpiTarget`): only this
process is accepted, by PID, by exact name, or by default. A `pidadr`
holding 0 gets the PID written back. Any other PID or name is
`SS$_NONEXPR`, including -1, which is only a wildcard to `$GETJPI`. The
GROUP/WORLD privileges VMS checks always pass.

**Scheduled wakeups share the `$SETIMR` timer queue**, as they do on VMS.
`timerRequest` gained `wake` and `repeat`:

- `daytim` is absolute or (negative) delta, like `$SETIMR`'s. An absolute
  time already past wakes at the next check. 0 or unreadable is
  `SS$_ACCVIO`.
- `reptim`, when given and nonzero, must be a delta time (positive is
  `SS$_IVTIME`); shorter than 10ms becomes 10ms. The entry then stays
  queued and fires every `reptim`. Repetitions missed between checks
  collapse into one wakeup (the flag can't count them), and the next
  stays on the original grid.
- An absolute `daytim` whose first repetition is already past is
  `SS$_IVTIME`, as the manual says.
- **`$CANWAK`** removes every queued wakeup. It doesn't clear a wakeup
  that is already pending. **`$CANTIM`** now skips wakeup entries, since
  they aren't timer requests.
- **Image rundown** cancels scheduled wakeups along with timers
  (`cancelTimers`), as the manual requires. A pending wakeup survives.

Not implemented: the `ASTLM` quota (`SS$_EXQUOTA`) and `SS$_INSFMEM`;
`SS$_NOPRIV` (no other processes); the `HIB` process state.

### AST delivery, `$DCLAST`, `$SETAST`

`SYS$DCLAST astadr ,[astprm] ,[acmode]` and `SYS$SETAST enbflg`

#### What an AST is

An **AST** (asynchronous system trap) is a procedure call VMS makes on a
program's behalf, interrupting it, when something the program asked about
happens: a timer expires (`$SETIMR`'s `astadr`), a request completes
(`$GETJPI`'s), or the program asks for one (`$DCLAST`). The program then
continues as if nothing had happened. VMS programs use ASTs where others
would use threads or callbacks: start an operation, do something else (or
`$HIBER`), and let the AST routine handle the completion.

Each AST belongs to an **access mode** and runs in it. Each mode has its
own queue, its own enable switch (`$SETAST`), and at most one AST running:
an AST routine is never interrupted by another AST of its own mode.

The routine is called with five arguments: `astprm`, then the interrupted
program's R0, R1, PC, and PSL.

#### Design: the RTL delivers ASTs, the engine calls them

On a real VAX, AST delivery is split between hardware (the `ASTLVL`
register, checked by `REI`, requesting an IPL 2 software interrupt) and
the executive (whose IPL 2 handler builds the call). As with the timer
queue (subtask 11), govax's executive is the Go RTL, and the guest's
interrupt path isn't used: an AST must not depend on a particular guest
kernel's software-interrupt handler. The work is split along the existing
package lines, with neither package learning the other's data:

| Step | Who | Where |
| --- | --- | --- |
| 1. Before every instruction, ask whether an AST can run | engine | `Engine.Step` → `deliverAST` (`internal/cpu/ast.go`) |
| 2. Decide, pick the AST, push its argument list, mark it active | RTL | `Environment.NextAST` (`internal/rtl/ast.go`) |
| 3. Call the routine exactly as `CALLG` would | engine | `buildCallFrame`, shared with `CALLS`/`CALLG` |
| 4. On the routine's `RET`, restore the interrupted state | RTL | `serviceSysClrast` |

The engine reaches the RTL through a new **optional** interface,
`cpu.ASTSource` (`NextAST() (ASTCall, bool, error)`).
`SetSystemServices` checks for it with a type assertion, so the test
doubles that implement only `SystemServices` are unaffected. The console
implements it by delegating to its `rtl.Environment` and converting types,
the same way it translates `rtl.ErrWait`, so `internal/rtl` still doesn't
import `internal/cpu`.

#### When an AST is delivered

`NextAST` delivers the oldest queued AST for the current mode when all of
these hold, as on VMS (since subtask 22, also an AST of a more privileged
mode, by switching into it: see "Mode-switching AST delivery"):

- IPL is below 2 (`IPL$_ASTDEL`), and the CPU isn't on the interrupt stack.
  Interrupt handlers (IPL 20+) and kernel code that raised IPL are never
  interrupted. It also means an interrupt taken in the same step (IPL now
  high) is never itself interrupted by an AST.
- ASTs are enabled for this mode and every more privileged mode.
- No AST of this mode is already active.

Before checking, `NextAST` expires due timers, whatever the IPL. So
timers now fire on the tick they're due, not only at the next event-flag
service (subtask 11's lazy expiry remains for code without an engine).

#### The frame and the return path

`NextAST` pushes six longwords on the current stack. They are the
routine's argument list and the state to restore:

```text
SP+0   5        argument count
SP+4   astprm
SP+8   R0       ─┐
SP+12  R1        │ the interrupted program's state
SP+16  PC        │
SP+20  PSL      ─┘
```

The engine then builds a `CALLG (SP), routine` frame, whose saved return
PC is **`SYS$CLRAST` + 2**: the `XFC` in that P1-vector entry, past its
entry mask. On VMS, `$CLRAST` is the (undocumented) service AST delivery
returns through; govax uses its vector entry the same way. The routine's
`RET` unwinds its frame. `CALLG` frames don't pop their argument list, so
SP is left at the six longwords. Execution continues at the `XFC`, which
calls `serviceSysClrast`. That service restores R1, PC, and PSL, pops the
frame, clears the mode's active flag, and returns the saved R0. The `XFC`
handler stores it in R0 as it does any service status.

Details:

- **No argument list.** `$CLRAST` is reached by `RET`, not `CALLS`, so
  AP is the interrupted code's. It is registered with the new
  `ServiceTable.RegisterNoArgs`, and `SystemService` doesn't read an
  argument list for it.
- **Misuse is harmless.** If no AST is active in the mode, or SP isn't at
  the active AST's frame (a direct call, or an unbalanced routine),
  `$CLRAST` changes nothing and returns `SS$_NORMAL`.
- **No privilege from the stack.** The restored PSL keeps the current
  mode and interrupt-stack bits, so a routine that overwrote its frame
  can't raise its privilege.
- **The stub must exist.** `NextAST` checks that the `XFC` is at
  `SYS$CLRAST` + 2 (the P1 vector's stubs are only in memory once
  `.P1VECTOR` has been assembled, as they are for any program that calls
  services). If it isn't, or the frame can't be pushed, `NextAST` returns
  an error and the engine stops (VMS would delete the process).
- **A bad routine address** (unreadable entry mask) faults as the routine
  is given control, with the routine's address as the faulting PC, as the
  manual describes.

#### ASTs and waits

`$WAITFR` and `$HIBER` wait by re-executing their `XFC` (subtask 9), so
every retry is an instruction boundary where an AST can be delivered. The
AST's saved PC is the `XFC`, so after the routine returns the wait runs
again. This is VMS's "the wait is interrupted by the AST, then
re-executed". If the routine set the flag or called `$WAKE`, the retry
succeeds.

#### The services

- **`$DCLAST`** queues an AST for `astadr` with `astprm`, in `acmode`
  maximized with the caller's mode. It always returns `SS$_NORMAL`. As the
  manual says, `astadr` isn't validated. An AST for the caller's own mode
  typically runs as soon as the call returns: `NextAST` delivers it
  before the stub's `RET`.
- **`$SETAST`** enables (low byte of `enbflg` nonzero) or disables ASTs
  for the caller's mode, returning `SS$_WASSET` or `SS$_WASCLR` for the
  previous state. ASTs queued while disabled are delivered once enabled.
  eVAX recorded one process-wide flag and always returned `SS$_NORMAL`;
  the old `Environment.astEnabled` is gone.
- **Image rundown** discards queued user-mode ASTs, and resets user mode
  to ASTs enabled with none active (`flushUserASTs`).

State lives in `Process.ast` (`astState`: the queue, standing in for
`PCB$L_ASTQFL`; per-mode `enabled`, `PCB$B_ASTEN`; per-mode `active` and
frame address, `PCB$B_ASTACT`).

Not implemented (see `docs/DEVIATIONS.md`):

- (Mode switching: VMS delivers an inner-mode AST to a process running in
  an outer mode by switching to that mode. Subtask 15 didn't; subtask 22
  does.)
- The `ASTLVL` register and `REI`'s AST check are not used.
- No `ASTLM` quota (`SS$_EXQUOTA`) and no `SS$_INSFMEM`.
- (`JPI$_ASTACT`, `ASTEN`, and `ASTCNT` are reported by `$GETJPI` since
  subtask 21.)

### ASTs from `$SETIMR` and `$GETJPI`

With delivery in place (subtask 15), the two services that accepted an
`astadr` and ignored it now deliver it, through `queueAST`:

- **`$SETIMR`**: `timerRequest.astadr` is recorded. When the timer
  expires, its event flag is set and an AST is queued in the mode the
  timer was set from, with `reqidt` as the parameter, as the manual says.
  A cancelled timer (`$CANTIM`, image rundown) queues nothing. A timer
  whose flag is in a common cluster the process has since disassociated
  sets no flag, but still queues its AST, which doesn't need the flag.
- **`$GETJPI`/`$GETJPIW`**: the request completes at once, so the AST is
  queued as it completes (with the event flag and IOSB), with `astprm`, in
  the caller's mode. It then runs as soon as the service returns. A
  request that fails in its items (`SS$_BADPARAM`) still completes, so its
  AST runs. One rejected before it starts (`SS$_ILLEFC`, `SS$_NONEXPR`,
  `SS$_INSFARG`, ...) completes nothing and queues no AST.

Timers now expire before every instruction (`NextAST` calls
`expireTimers`), so a timer's AST interrupts a program on the tick it's
due, even in a loop that calls no services. The event-flag services still
expire timers too, for code driven without an engine.

**The classic pattern** is `$SETIMR` with an AST, then `$HIBER`. The
timer's AST is delivered between `$HIBER`'s retries, and its routine
calls `$WAKE`. When the routine returns, `$HIBER` runs again and returns.
The acceptance fixture does exactly this.

### `$QIO`/`$QIOW` on terminals, and `$CANCEL`

`SYS$QIO[W] [efn] ,chan ,func [,iosb] [,astadr] [,astprm] [,p1]...[,p6]`
and `SYS$CANCEL chan`

#### How VMS device I/O works

A program gets a channel to a device from `$ASSIGN`, then queues requests
on it with `$QIO`. `func`'s low 6 bits are the **function code**
(`IO$_READVBLK`, `IO$_WRITEVBLK`, ...) and the bits above it **function
modifiers** (`IO$M_NOECHO`, ...); each device class has its own. `p1`-`p6`
are the function's parameters. When the request finishes, its outcome
goes to the **I/O status block** (`iosb`: a status word, a transfer-count
word, and a device-specific longword), the event flag is set, and the AST
(if any) is queued. `$QIO`'s own R0 only says whether the request was
accepted. `$QIOW` is `$QIO` plus the wait.

#### Design: complete during the call, drivers in a registry

Every request completes before `$QIO` returns, as `$GETJPI` already does.
VMS allows this, and a correct program can't tell. So `$QIOW` is the same
service, and `$CANCEL` never finds an outstanding request (it only checks
the channel).

`serviceSysQio` follows VMS's order: clear the event flag
(`ILLEFC`/`UNASEFC`), check the channel (`IVCHAN` for 0, `NOPRIV` if
unassigned or assigned from a more privileged mode), clear the IOSB
(`ACCVIO`), find the function, perform it. From the channel check on, a
failure sets the event flag and returns in R0, writing no IOSB status and
queuing no AST.

The function is found in two tables: `ioDrivers` maps a device class to
its driver's function table, which maps an `$IODEF` function code to an
`ioFunc`. An `ioFunc` returns the IOSB contents, or rejects the request
(`SS$_ACCVIO` for a buffer it can't access, checked with `accessible`
before any input is consumed). Only terminals have a driver; anything
else is `SS$_ILLIOFUNC`.

`$IODEF` is generated from `reference/vms/iodef.sdl` (from the VMS 7.3
`starlet` sources) as `vmsdef.IOConstants`. The SDL parser learned the
module's `#fcode_size = 6;` local symbol, bitfield lengths like
`16-#fcode_size`, and constants inside an aggregate (SDL's default
prefix and tag `K`).

#### The terminal driver (`ttdriver.go`)

Every terminal is the console: reads use the console input stream (the
one `DECC$GETS` reads, sharing its buffer), writes the output stream.

| Function | Parameters | Does |
| --- | --- | --- |
| `IO$_READVBLK` (also `READLBLK`, `READPBLK`, `TTYREADALL`) | `p1` buffer, `p2` size, `p3` time limit, `p4` terminators | Reads until a terminator or a full buffer. |
| `IO$_READPROMPT` (also `TTYREADPALL`) | as above, `p5`/`p6` the prompt | Writes the prompt, then reads. |
| `IO$_WRITEVBLK` (also `WRITELBLK`, `WRITEPBLK`) | `p1` buffer, `p2` size, `p4` carriage control | Writes the bytes. |
| `IO$_SENSEMODE` (also `SENSECHAR`) | `p1` buffer, `p2` size (8 or 12) | Returns class, type, width, characteristics. |
| `IO$_SETMODE` (also `SETCHAR`) | as above | Sets type, width, characteristics. |

- **Reads.** The default terminators are the control characters except
  BS, TAB, LF, VT, and FF. `p4` can give a short-form (32 control
  characters) or long-form (a mask of up to 256 bits) set instead. A host
  newline, or `"\r\n"`, is read as RETURN. The IOSB holds the status, the
  data count (the terminator's offset), the terminator, and its size (0
  when the buffer filled first). The terminator is also stored after the
  data when there's room. `IO$M_CVTLOW` upcases letters; `IO$M_PURGE`
  discards type-ahead; `IO$M_TIMED` with a zero limit reads only what's
  already buffered, ending in `SS$_TIMEOUT`. The end of host input is
  `SS$_ENDOFFILE`.
- **Carriage control.** `p4` of 0 writes the data as is. A nonzero low
  byte is a FORTRAN carriage-control character (`" "` new line before,
  carriage return after; `"0"` two new lines; `"1"` form feed; `"+"` no
  new line; `"$"` no carriage return). Otherwise bytes 2 and 3 are a
  prefix and postfix in RMS print-file form: a new-line count, or a C0 or
  C1 control character.
- **Characteristics** live on the device record, where VMS keeps them:
  `DevType`, `DevBufSize` (page width), `DevDepend` (characteristics and
  page length), `DevDepend2` (extended). `IO$M_TYPEAHDCNT` returns the
  count of buffered characters and the first one.

Not implemented (see `docs/DEVIATIONS.md`): true asynchrony (a read
blocks until the host delivers input); drivers for other device classes;
echo control, line editing, nonzero time limits, and the other modifiers
the host terminal can't honour; `IO$_SETMODE`'s CTRL/C, CTRL/Y, and
out-of-band ASTs; the I/O quotas.

### `$SYNCH` — Synchronize

`SYS$SYNCH [efn] ,[iosb]`

An event flag alone can't say a request finished: flags are shared, and a
timer, `$SETEF`, or another request may set the same one. The IOSB can,
since only the request writes its status word, and always nonzero.
`$SYNCH` combines the two, as the manual describes:

1. Wait until the event flag is set (the `$WAITFR` mechanism: `ErrWait`,
   re-executing the `XFC`).
2. If the IOSB's status word is nonzero, return `SS$_NORMAL`. The flag is
   left set, so another `$SYNCH` for a second request sharing it still
   sees it.
3. Otherwise it was a false alarm: clear the flag and go back to 1.

With `iosb` omitted, only step 1 is done. Only the IOSB's first word (the
condition value) is tested: a `$QIO` IOSB's transfer count shares its
first longword. Every govax request completes during its call, so a
`$SYNCH` after `$QIO`/`$GETJPI` returns at once; the false-alarm path
matters for requests a program completes itself, from an AST.

Condition values: `SS$_NORMAL`; `SS$_ILLEFC`/`SS$_UNASEFC` as for
`$WAITFR`; `SS$_ACCVIO` for an unreadable IOSB (the manual lists only
`SS$_NORMAL`, but VMS would fail with an access violation there).

### `$EXIT`, `$DCLEXH`, `$CANEXH` — Exit Handlers

`SYS$EXIT [code]`, `SYS$DCLEXH desblk`, and `SYS$CANEXH [desblk]`

#### What exit handlers are

A program can have VMS call procedures of its own when its image exits,
however it exits, to tidy up. It describes each in an **exit control
block** and declares it with `$DCLEXH`:

```text
desblk+0    forward link (written by VMS)
desblk+4    exit handler address
desblk+8    argument count (low byte; the other 3 bytes 0)
desblk+12   address of a longword VMS fills with the exit status
desblk+16   more arguments, optionally
```

From `desblk+8` on, the block is an argument list, so the handler is
called as `CALLG desblk+8, handler`. VMS keeps one list per access mode,
newest first; kernel mode has none (`SS$_IVSSRQ`). `$EXIT` calls the
handlers, each once, then ends the image; it never returns.

#### Design: the service asks the engine to make the call

Calling a guest procedure from a service is the same problem AST delivery
solved, with one difference: the service isn't done when the procedure
returns. So a service may now return a **call request**
(`rtl.CallRequest`, translated by the console to `cpu.ServiceCall`). The
XFC handler (`Engine.callForService`) builds a `CALLG` frame for the
routine whose return address is the **XFC itself**. When the handler
executes `RET`, the XFC runs again, calling `$EXIT` again, which moves on
to the next handler. Since `$EXIT` removes each handler from its list
before asking for the call, the lists themselves are all the state it
needs, and a handler that calls `$EXIT` just continues with the rest.

When no handlers are left, `$EXIT` returns `rtl.ErrExit` (the console's
`cpu.ErrImageExit`) with the status as R0. `Engine.exitImage` then ends
the image: it follows the frame chain (each frame's saved FP, at FP+12)
to the console's own call frame (`CallEntry`'s, with saved PC and FP both
`SentinelReturn`) and executes `RET` from it. That discards every frame
in between and reports `ErrConsoleCallReturned`, the normal end of RUN,
so image rundown follows. Without such a frame (a program begun with
`START`), the machine halts with R0 set.

#### The services

- **`$DCLEXH`** writes the current front block's address (0 if none) to
  the new block's forward link (`SS$_ACCVIO` if it can't), and adds the
  block to the caller's mode's list. `SS$_NOHANDLER` for 0.
- **`$CANEXH`** removes a block, relinking the block declared after it
  past it (`SS$_ACCVIO` if that fails). `SS$_NOHANDLER` if it isn't
  declared; with no block, every block for the mode is removed.
- **`$EXIT`** saves the status in `Process.ExitStatus` (SS$_NORMAL for a
  call with no argument list, as the `$EXIT_S` macro passes), then calls
  the caller's mode's handlers newest first, storing the status through
  each one's first argument, then ends the image. A block whose handler
  address can't be read is skipped.
- **RUN's image driver** (`buildImageInitDriver`) now calls `$EXIT` with
  the value `main` returns, as VMS's image activator does, when the
  P1 vector's `SYS$EXIT` stub is in memory. So an image that returns
  normally still has its handlers called.
- **Image rundown** forgets user-mode exit handlers, which live in the
  image's memory.

State: `Process.exitHandlers` (per mode, oldest first: VMS's
`CTL$GL_THEXIT` lists) and `Process.ExitStatus`. The old recorded
`Environment.exitHandler` and `core.go`'s `$DCLEXH` stub are gone.

Not implemented (see `docs/DEVIATIONS.md`): `$EXIT` calls only the
caller's mode's handlers, not the supervisor- and executive-mode ones VMS
calls afterwards; `$FORCEX`; handlers for an image that ends by a fatal
exception (the console reports those and stops).

### `$NUMTIM` — Convert Binary Time to Numeric Time

`SYS$NUMTIM timbuf ,[timadr]`

`$NUMTIM` breaks the time at `timadr` (the current time, `env.Clock()`,
if omitted) into seven words at `timbuf`: year, month, day, hour, minute,
second, hundredths. It is `numericTime`, a pure function next to
`formatVMSTime`, with the same arithmetic: the date from
`vmsdef.GoTime`, the time of day from the ticks into the day, hundredths
truncated.

- A time of 0 is the base date, 17-NOV-1858 00:00:00.00.
- A delta time gives year and month 0 and its whole days as the day.
  10,000 days or more is `SS$_IVTIME`.
- An unreadable time, or a `timbuf` that is 0 or can't be written (all 14
  bytes are checked first, so nothing is half-written), is `SS$_ACCVIO`.

### `$GETJPI` items for AST and scheduling state

With ASTs delivered (subtask 15), the `$GETJPI` items describing them now
mean something. They join the registry in `getjpi.go`:

| Item | Returns |
| --- | --- |
| `JPI$_ASTEN` | Bit vector of modes with ASTs enabled (bit 0 kernel ... bit 3 user), from `Process.ast.enabled`. |
| `JPI$_ASTACT` | Bit vector of modes with an AST running, from `Process.ast.active`. |
| `JPI$_ASTLM` | The AST quota, `Process.ASTLimit` (24, the usual UAF default). |
| `JPI$_ASTCNT` | What's left of it: the quota less every queued AST and every `$SETIMR` timer that will queue one, as VMS charges an AST from request to delivery. Never below 0. |
| `JPI$_PRI`, `JPI$_PRIB` | Current and base priority, `Process.Priority`/`BasePriority` (4, SYSGEN `DEFPRI`). |
| `JPI$_STATE` | Always `SCH$C_CUR` (14): the manual says a process that is executing is always in that state, and the only process that can ask is the one executing. |

`$STATEDEF` has no source file of its own in the VMS 7.3 archive, so
`reference/vms/statedef.txt` was extracted mechanically from the
`literal SCH$C_...` declarations of the `LIB.REQ` listing
(`trace/lis/lib.lis`) into `ssdef.txt`'s BLISS layout, and generated as
`vmsdef.STATEConstants`.

The quota is reported, not enforced: nothing fails with `SS$_EXQUOTA`
when `ASTCNT` reaches 0.

### Mode-switching AST delivery

Subtask 15 delivered an AST only while the CPU was in the AST's own mode,
so a kernel-mode AST (say, from a timer set in kernel mode) couldn't
interrupt user-mode code, not even a user-mode `$HIBER` waiting for it.
VMS does deliver it, by switching the process into the AST's mode. govax
now does the same.

#### Which AST goes next

`NextAST` looks at modes from kernel out to the CPU's current mode and
takes the first that has a queued AST, has ASTs enabled (in it and every
more privileged mode), and has no AST already running. Within that mode
the oldest AST goes. So the most privileged deliverable AST always goes
first, and an AST of a *less* privileged mode than the CPU's still waits
(a user AST never interrupts kernel code). IPL and the interrupt stack
are checked first, as before.

#### Switching in and out

Each mode has its own stack, whose pointer is kept in a processor
register (KSP, ESP, SSP, USP) while the mode isn't running. To deliver an
AST of a more privileged mode, `enterASTMode`:

1. saves SP in the current mode's register and loads the AST mode's
   (`switchMode`), setting `PSL<CUR_MOD>` to the AST's mode and
   `PSL<PRV_MOD>` to the interrupted one;
2. pushes the usual six-longword frame on the AST mode's stack, holding
   the interrupted R0, R1, PC, and (outer-mode) PSL.

If the frame can't be pushed, the switch is undone and delivery fails.
The engine side is unchanged: it builds the `CALLG` frame on whatever
stack the RTL left current.

`$CLRAST` restores from the frame as before, and if the saved PSL's mode
is less privileged than the AST's, switches back (`switchMode` again),
the equivalent of `REI`. A saved mode more privileged than the AST's is
ignored, so a forged frame can't raise privilege. Both directions drop
the memory system's cached access checks (`InvalidateProtection`), as the
engine's own mode changes do.

Since waits retry their `XFC` each step, an inner-mode AST now also
interrupts an outer-mode wait: the classic "user program hibernates until
a kernel timer AST wakes it" works.

### `$GETSYI` / `$GETSYIW` — Get Systemwide Information

`SYS$GETSYI[W] [efn] ,[csidadr] ,[nodename] ,itmlst [,iosb] [,astadr] [,astprm]`

The `$GETJPI` pattern, for the system rather than a process: an item list
says what to return, and the request completes during the call (flag
cleared then set, IOSB, AST with `astprm` in the caller's mode), so both
forms are the same service. A request rejected before it starts
(`SS$_INSFARG` for fewer than four arguments, a bad event flag, an
unwritable IOSB, a node that isn't this one) completes nothing; an
unsupported item still completes, with `SS$_BADPARAM`.

#### The node

VMS can report on any node of a VAXcluster. govax is one node outside any
cluster (`nodeTarget`):

- `csidadr` holding 0, this node's cluster system ID (a node outside a
  cluster has none), or omitted: this node. -1 starts a wildcard scan that
  returns this node, then `SS$_NOMORENODE`, as `$GETJPI`'s process
  wildcard does. Any other CSID is `SS$_NOSUCHNODE`.
- `nodename` must be exactly this node's name, `Environment.NodeName`
  (`GOVAX`), or it's `SS$_NOSUCHNODE`; empty or over 15 characters is
  `SS$_IVLOGNAM`. With both arguments, both must name this node.

#### Items

| Item | Returns |
| --- | --- |
| `SYI$_VERSION` | `"V7.3    "` (8 characters, blank-padded) |
| `SYI$_NODE_SWVERS`, `SYI$_NODE_SWTYPE` | `"V7.3"`, `"VMS "` (4 characters) |
| `SYI$_NODENAME` | `Environment.NodeName` |
| `SYI$_SID`, `SYI$_CPU` | The CPU's SID register, and its processor-type byte (bits 31:24) |
| `SYI$_BOOTTIME` | `Environment.BootTime`: when the INIT/VMINIT/ZERO that built the Environment ran, on the engine's clock |
| `SYI$_CLUSTER_MEMBER`, `SYI$_NODE_CSID` | 0: not a cluster member, no CSID |
| `SYI$_MINWSCNT` | The SYSGEN parameter `$ADJWSL` clamps to (`Process.MinWSCount`) |

The version is that of the VMS sources govax's definitions come from:
the `$SYIDEF`, `$IODEF`, `$JPIDEF`, ... codes are VMS 7.3's, so a program
that tests the version finds the release those definitions describe.
The `*_EMULATED` items are left out: decimal-string instructions aren't
implemented, so any answer would mislead.

`$SYIDEF` is generated from `reference/vms/syidef.txt`, the VMS 7.3 VEST
listing (`vest_dblrtl/lis/syidef.txt`), whose `NAME, I4, value` layout the
BLISS-literal parser now accepts (the `SYI$_...` item codes and
`SYI$C_` values, 309 symbols). The archive's `syidef.sdl` has lost its
line breaks, so its comments can't be parsed.

The item values `$GETJPI` and `$GETSYI` return are built with shared
helpers in `itemlist.go` (`itemString`, `itemPadded`, `itemByte`,
`itemWord`, `itemLong`, `itemQuad`, `storeItem`), which replaced
`getjpi.go`'s JPI-only ones.

### `$FAO` / `$FAOL` — Formatted ASCII Output

`SYS$FAO ctrstr ,[outlen] ,outbuf ,[p1]...[p20]` and
`SYS$FAOL ctrstr ,[outlen] ,outbuf ,[prmlst]`

A VMS program builds text to print with `$FAO`: a control string of
ordinary text and `!` directives, and parameters the directives consume
in order (`"Found !UL file!%S"` with 3 is `"Found 3 files"`). `$FAOL`
takes the parameters from an array instead of the call. The system's
message texts are `$FAO` control strings too, so `$GETMSG`/`$PUTMSG`
(subtask 25) reuse the formatter.

#### Design: a directive registry

`formatFAO` walks the control string. For each directive it reads the
optional repeat count (`!n(...)`) and width (`!mDD`), either of which may
be `#` (take it from the next parameter), then the name, which it looks
up in `faoDirectives`: directive name to the Go function performing it.
The 40 numeric directives (radix `O`/`X`/`Z`/`U`/`S` times size) are
generated into the registry at package load from two small tables, so
each rule is written once.

| Directives | Do |
| --- | --- |
| `!AC`, `!AD`, `!AF`, `!AS` | Counted, length-and-address, filtered (nonprintables as `.`), and descriptor strings. |
| `!Ox`, `!Xx`, `!Zx`, `!Ux`, `!Sx` | Octal, hexadecimal, zero-filled decimal, unsigned, signed. Sizes `B`, `W`, `L`; VMS 7's `A`, `I`, `H`, `J` (longwords on a VAX); `Q` (a quadword, by reference). |
| `!/`, `!_`, `!^`, `!!` | CR-LF, tab, form feed, `!`. |
| `!%S` | `S` (or `s`, after a lower-case letter) unless the last number was 1. |
| `!%D`, `!%T` | `$ASCTIM`'s date-and-time and time-only text, of the quadword a parameter points to (0 is now, from `env.Clock()`). |
| `!%U`, `!%I` | A UIC (`[1,4]`, octal); an identifier (the process's UIC is `[SYSTEM]`). |
| `!n<` ... `!>`, `!n*c` | A fixed-width field; `c` written `n` times. |
| `!-`, `!+` | Reuse the previous parameter; skip one. |

Field widths follow the manual's table: octal and hexadecimal are
zero-filled to their size's width, then blank-padded or left-truncated
to the given width; decimal is right-justified (zeros for `!Zx`) and all
asterisks if it doesn't fit; strings are left-justified and
right-truncated.

#### Status

- `SS$_BUFFEROVF` when the result doesn't fit `outbuf` (it's truncated,
  and `outlen` is the truncated length).
- `SS$_BADPARAM` for an unknown directive (lower-case letters included);
  what came before it is still stored.
- `SS$_ACCVIO` for an unreadable control string, parameter, or string,
  an `outbuf` or `outlen` that can't be written, or `$FAOL` needing a
  parameter with `prmlst` omitted. Nothing is stored.

As the manual says, `$FAO` doesn't check its argument list's length: a
parameter past the list reads as 0.

Not implemented (see `docs/DEVIATIONS.md`): `!%C`, `!%E`, `!%F`; a rights
database for `!%I`.

### `$GETMSG` / `$PUTMSG` — Get Message, Put Message

`SYS$GETMSG msgid ,msglen ,bufadr ,[flags] ,[outadr]` and
`SYS$PUTMSG msgvec ,[actrtn] ,[facnam] ,[actprm]`

Every condition value has a message, written
`%FACILITY-S-IDENT, text`: the facility (bits 16-27 of the value), a
severity letter (bits 0-2: W, S, E, I, F), a short identifier, and a
text that is a `$FAO` control string (`SS$_ACCVIO`'s is "access
violation, reason mask=!XB, virtual address=!XH, PC=!XH, PS=!XL").
Message *flags* pick the parts: 1 text, 2 identifier, 4 severity, 8
facility; 0 means all of them, the process default.

#### The message texts

`reference/vms/sysmsg.txt` is the CLI, LIB, MTH, OTS, RMS, and SYSTEM
facility blocks, verbatim, of the VMS 7.3 system message file's listing
(`msgfil/lis/sysmsg.lis`). Each message line there carries the value
the MESSAGE compiler assigned, so no `.SEVERITY`/`.BASE` arithmetic is
needed. `internal/vmsdef/gen/msg.go` parses it into
`vmsdef.Messages` (1,426 messages, in a second generated file,
`messages_generated.go`), keyed by the value without its severity and
control bits, with each message's `/FAO=` parameter count and `/ID=`
identifier. The listing cut nine of these lines at 132 columns; their
texts are kept as far as they go, and a lost `/FAO=` count is worked out
from the directives left.

A value with no message gets VMS 7.3's stand-in, `%SYSTEM-W-NOMSG,
Message number 00007FF8` (`NONAME` for a facility the file doesn't
have).

#### `$GETMSG`

Stores the message line, unformatted, in `bufadr` and its length at
`msglen`, with `flags`' parts (0 or omitted: all). The severity letter is
the one in `msgid`. `outadr`, if given, gets four bytes: 0, the text's
`$FAO` parameter count, 0 (no message here has a user value), 0.
`SS$_MSGNOTFND` (a success) for the stand-in, `SS$_BUFFEROVF` for a
truncated line, `SS$_INSFARG` for fewer than three arguments.

#### `$PUTMSG`

The message vector is a longword (the count of longwords after it, and
default flags in bits 16-19) and then messages. What follows each
condition value depends on its facility:

- SYSTEM (0): as many `$FAO` parameters as its text takes (usually
  none, so the next longword is the next message).
- RMS (1): the status value (STV). It is the text's parameter if the
  text takes one; otherwise a nonzero STV is a SYSTEM condition value,
  written as a message of its own.
- Any other: a longword with the parameter count (low word) and, if
  nonzero, new flags for this and later messages (high word), then the
  parameters.

Each text is formatted with `formatFAO`; if that fails (a parameter
missing from the vector), the text is written unformatted, as the manual
says. The first line starts with `%`, the rest `-`; `facnam` replaces the
first line's facility. Lines go to the terminal (`consoleOut`), ending in
a new line as an RMS record written there does.

#### The action routine

With `actrtn`, each line goes to the routine before it is written, and
is written only if the routine returns an odd R0. This uses subtask 19's
call path: `$PUTMSG` lowers SP and stores the routine's argument list (a
descriptor of the line, and `actprm` if the call had one), the
descriptor, and the text there, then returns a `CallRequest` whose
argument list is that SP. The routine's `RET` returns to `$PUTMSG`'s
`XFC`, so the service runs again. It finds its call in
`Process.putmsg` (matched by the stub's frame pointer and the lowered
SP), checks R0, restores SP, and goes on to the next line. The calls are
a stack, so an action routine may call `$PUTMSG` itself. Image rundown
forgets any left pending.

Not implemented (see `docs/DEVIATIONS.md`): message sections in an
image, and SET MESSAGE's process message file and default flags; other
facilities' messages; writing to `SYS$ERROR` and `SYS$OUTPUT`
separately.

### `$CMKRNL` / `$CMEXEC` — Change to Kernel / Executive Mode

`SYS$CMKRNL routin ,[arglst]` and `SYS$CMEXEC routin ,[arglst]`

A program whose user holds the CMKRNL (or CMEXEC) privilege can have a
routine of its own run in kernel (or executive) mode, to do something
user mode can't: execute a privileged instruction, change a system data
structure. VMS switches the process into the mode, calls the routine
with `CALLG arglst, routin`, switches back when it returns, and returns
its R0.

#### Design: a mode switch around a call request

Each service runs twice, combining subtask 22's `switchMode` with
subtask 19's `CallRequest`:

1. Called by the program: the target mode is the more privileged of the
   one asked for and the caller's (so `$CMEXEC` from kernel mode stays
   in kernel mode, as the manual says). If it differs from the caller's,
   `switchMode` saves SP in the caller's stack register and loads the
   target's, and `PSL<PRV_MOD>` becomes the caller's mode. A
   `cmodeCall` (the stub's FP, the target mode's SP, the caller's mode
   and previous mode) goes on `Process.cmode`, and the service returns a
   `CallRequest` for `routin` with `arglst` (AP is 0 if it's omitted).
   The engine builds the call frame on the target mode's stack.
2. The routine's `RET` returns to the service's `XFC`. The service
   finds the top `cmodeCall` matching FP, SP, and the current mode,
   switches back, restores `PSL<PRV_MOD>`, and returns the routine's R0,
   which the `XFC` handler leaves in R0.

The calls are a stack, so a routine may call `$CMKRNL` itself; image
rundown forgets calls whose routine never returned. The process holds
every privilege, so `SS$_NOPRIV` never happens.

Each mode needs a stack its code can write. VMINIT sets ESP and SSP to
the kernel stack, whose pages only kernel mode can write when memory
management is on, so a program that uses `$CMEXEC` must give executive
mode a stack first (as the acceptance fixture does); VMS gives every
mode its own.

Not implemented (see `docs/DEVIATIONS.md`): R4 isn't loaded with a PCB
address for `$CMKRNL` (govax has no PCB).

### CTRL/C and CTRL/Y ASTs

`$QIO[W] chan, IO$_SETMODE!IO$M_CTRLCAST (or IO$M_CTRLYAST), p1=astadr,
p2=astprm, p3=acmode`

On VMS, CTRL/Y interrupts the running program (the command interpreter
takes over), and CTRL/C does the same unless a program asks for it. A
program asks by enabling a CTRL/C (or CTRL/Y) AST on a terminal channel:
the terminal driver then calls its AST routine when the key is typed,
and the program keeps running. The AST is one-shot: it must be enabled
again for the next key. `p1` of 0 cancels; `p3` is maximized with the
caller's mode. When a key is typed:

- CTRL/C: every enabled CTRL/C AST is delivered; with none, CTRL/C is
  taken as CTRL/Y.
- CTRL/Y: every enabled CTRL/Y AST is delivered; with none, the program
  is interrupted.

`$CANCEL` and `$DASSGN` cancel a channel's requests (so image rundown,
deassigning user channels, does too).

#### Design: the engine offers the key before stopping

Before, the host's Ctrl-C (`cmd/govax/attention.go`: a 0x03 byte from the
terminal in raw mode, or SIGINT in cooked mode) called `Engine.Attention`,
and the next `Step` returned `ErrAttention`, stopping the machine.

Now the engine remembers *which* key (`AttentionKey`: `AttentionCtrlC` or
`AttentionCtrlY`), and `Step` first offers it to the services' optional
`cpu.AttentionHandler` (found by `SetSystemServices`, as `ASTSource` is).
The console forwards to `rtl.Environment.Attention`, which applies the
rules above: if it queues an AST, the key is consumed and `Step` goes on
(delivering the AST at that same boundary); if not, `ErrAttention`
stops the machine as before.

The requests are the Environment's (`attentionASTs`: key, channel,
routine, parameter, mode), since every terminal is the console. The
terminal driver's `ttSetMode` records them (`ttAttentionAST`).

Host Ctrl-Y isn't intercepted: readline uses it at the `VAX>` prompt,
and macOS treats it as DSUSP. So a program's CTRL/Y AST is reached
through Ctrl-C with no CTRL/C AST enabled, as VMS itself does; the engine
API accepts either key for tests and later use.

Not implemented (see `docs/DEVIATIONS.md`): echoing `^C`; interrupting a
terminal read blocked on host input.

### `$GETDVI` / `$GETDVIW` — Get Device/Volume Information

`SYS$GETDVI[W] [efn] ,[chan] ,[devnam] ,itmlst [,iosb] [,astadr] [,astprm] [,nullarg]`

The `$GETJPI`/`$GETSYI` pattern over a device: the request completes
during the call (flag cleared then set, IOSB, AST with `astprm` in the
caller's mode), so both forms are the same service, and an unsupported
item still completes, with `SS$_BADPARAM`. It replaces eVAX's
`$GETDVIW`, which knew only `DEVCLASS`, `DEVTYPE`, and `DEVBUFSIZ` (and
wrote the first two as bytes) and required exactly eight arguments.

#### The device

`dviTarget` picks it: a nonzero `chan` (low word) is the device the
channel is assigned to (`SS$_NOPRIV` if it isn't assigned, or was
assigned from a more privileged mode — the manual's rule, where eVAX
returned `SS$_IVCHAN`); otherwise `devnam`, translated through logical
names like `$ASSIGN`'s (`SS$_IVLOGNAM` if empty or over 63 characters,
`SS$_NOSUCHDEV`); neither is `SS$_IVDEVNAM`.

#### Items

`$DVIDEF` (159 symbols) and `$TTDEF` (200, both `TT$` and `TT2$`) are
generated from the VMS 7.3 VEST listings `reference/vms/dvidef.txt` and
`ttdef.txt`, like `$SYIDEF`; the BLISS-literal parser now accepts the
mixed-case names a few of them have. The registry, `dviItemsByName`,
holds:

| Items | Return |
| --- | --- |
| `DEVCLASS`, `DEVTYPE`, `DEVBUFSIZ`, `DEVCHAR`, `DEVCHAR2`, `DEVDEPEND`, `DEVDEPEND2`, `DEVSTS`, `STS` | The device record's fields, as longwords. `DEVCHAR` includes `DEV$M_MNT` (and `DEV$M_SWL` for a read-only mount) while a volume is mounted. |
| `UNIT` | The digits the device name ends with. |
| `PID`, `OWNUIC`, `REFCNT`, `ERRCNT`, `OPCNT`, `ACPPID`, `LOCKID`, `RECSIZ`, `SERIALNUM` | Fields of the record. |
| `DEVNAM`, `TT_PHYDEVNAM` | `_TTA0:` (`TT_PHYDEVNAM` is empty for a non-terminal). |
| `FULLDEVNAM`, `ALLDEVNAM` | `_GOVAX$TTA0:`, with the node name. |
| `ROOTDEVNAM`, `MEDIA_NAME`, `MEDIA_TYPE` | Strings from the record. |
| `VOLNAM`, `MAXBLOCK`, `FREEBLOCKS`, `CLUSTER`, `MAXFILES` | From the mounted ODS-2 volume, as SHOW DEVICE/FULL reports them; else the record's fields. |
| `CYLINDERS`, `SECTORS`, `MOUNTCNT` | The record's fields. |
| `ALLOCLASS`, `REMOTE_DEVICE`, `SERVED_DEVICE`, `VOLSETMEM` | 0: one node, no volume sets. |
| `REC`, `CCL`, `TRM`, ... `WCK` (28) | 1 if the device has `DEV$M_x`, else 0. |
| `TT_x` (50) | 1 if the terminal's `DEVDEPEND` has `TT$M_x` (or `DEVDEPEND2` has `TT2$M_x`). `TT_PAGE` is the page length, `DEVDEPEND`'s high byte. |

The Boolean items are added by `init` from the two lists, so every
`DVI$_TT_` code in `$DVIDEF` is supported (a test checks). An item
code's `DVI$M_SECONDARY` and `DVI$M_NOREDIRECT` bits are ignored: a govax
device has no separate secondary device.

Not implemented (see `docs/DEVIATIONS.md`): other nodes' devices
(`SS$_NONLOCAL`); host, shadow-set, and lock-name items; the `ASTLM`
quota.

Not implemented (see `docs/DEVIATIONS.md`): other cluster nodes; SYSGEN
parameters beyond `MINWSCNT`; the `ASTLM` quota (`SS$_EXASTLM`).

## Subtasks

1. **Done.** **Emulated process record.** `rtl.Process` replaces
   `Environment`'s `pid`/`uic` fields; `$ASSIGN` reads the PID/UIC from it.
   Adds `optArg` for omitted trailing arguments. This document.
2. **Done.** **`$ADJSTK`.** `serviceSysAdjstk` in `process.go`, registered
   by the new `registerProcessServices`.
3. **Done.** **`$ADJWSL`.** `serviceSysAdjwsl` in `process.go`.
4. **Done.** **`$ALLOC`.** `serviceSysAlloc` in `devices.go`; `$DEVDEF` from
   real VMS source (`vmsdef.DEVConstants`); allocation state on
   `iodev.Device`; image rundown; `$ASSIGN` and SHOW DEVICE/FULL honor
   allocation.
5. **Done.** **`$ASCEFC`.** `serviceSysAscefc` and the common event flag
   cluster table in the new `eventflags.go`; `$SETEF`/`$CLREF`/`$READEF`
   moved there and route flags 64-127 to associated clusters; image
   rundown disassociates. Acceptance fixture
   `testdata/asm/process_services.asm` covers subtasks 2-5.

Second batch, requested by the user on 2026-09-27 (the candidates the first
batch listed):

6. **Done.** **`$DALLOC`.** `serviceSysDalloc` in `devices.go`.
7. **Done.** **`$DACEFC` and `$DLCEFC`.** In `eventflags.go`.
8. **Done.** **`$GETJPI`/`$GETJPIW`** reading `rtl.Process`, in the new
   `getjpi.go`; `$JPIDEF` generated from real VMS source.
9. **Done.** **`$WAITFR`, `$WFLAND`, `$WFLOR`.** In `eventflags.go`, with
   `cpu.ErrServiceWait` re-executing the `XFC`. Acceptance fixture
   `testdata/asm/wait_timer.asm`.

Third batch, requested by the user on 2026-09-27:

10. **Done.** **`$DASSGN`.** In `devices.go`; image rundown deassigns
    user-mode channels.
11. **Done.** **`$SETIMR` and `$CANTIM`.** In `timers.go`: the RTL's own
    timer queue, running on the engine's new `SystemTime` (the interval
    clock's time base) rather than on the guest's timer interrupt.

Fourth batch, requested by the user on 2026-09-28 (the candidates the
third batch listed):

12. **Done.** **`$GETTIM`.** In the new `vmstime.go`; system time becomes
    local time (`vmsdef.Time`).

13. **Done.** **`$ASCTIM` and `$BINTIM`.** In `vmstime.go`, as the pure
    functions `formatVMSTime`/`parseVMSTime` plus service wrappers.

14. **Done.** **`$HIBER`, `$WAKE`, `$SCHDWK`, `$CANWAK`.** In the new
    `hibernate.go`; wakeups on the timer queue. Acceptance fixture
    `testdata/asm/hibernate.asm`.

15. **Done.** **AST delivery, `$DCLAST`, `$SETAST`.** The RTL's `ast.go`
    decides and restores; the engine's `ast.go` calls the routine through
    `buildCallFrame`; the AST exit is the `SYS$CLRAST` vector entry.
    Acceptance fixture `testdata/asm/ast_delivery.asm`.

16. **Done.** **ASTs from `$SETIMR` and `$GETJPI`.** `astadr` queued via
    `queueAST`. Acceptance fixture `testdata/asm/timer_ast.asm`.

Fifth batch, requested by the user on 2026-09-28 (the candidates the
fourth batch listed):

17. **Done.** **`$QIO`/`$QIOW` on terminal channels, and `$CANCEL`.** In the new
    `qio.go`: a request is validated, then handed to a per-device-class
    driver registry (only terminals have a driver), which completes it
    at once: IOSB, event flag, and AST, as `$GETJPI` does. Terminal
    reads, prompted reads, and writes go to the console's input and
    output streams; `IO$_SENSEMODE`/`SETMODE` read and write the
    device's characteristics. `$IODEF` is generated from real VMS
    source, which needs the SDL parser to learn `#local` symbols.
18. **Done.** **`$SYNCH`.** In `eventflags.go`: wait for the event flag, then
    check the IOSB, clearing the flag and waiting again while it's
    still 0.
19. **Done.** **`$EXIT`, `$DCLEXH`, `$CANEXH`: exit handlers.** Per-mode handler
    lists (replacing the single recorded `exitHandler`). `$EXIT` calls
    each handler through a new "service asks the engine to make a call"
    path, then ends the image by unwinding to the console's call frame.
    RUN's image driver calls `$EXIT` with `main`'s status, as VMS's
    image activator does.
20. **Done.** **`$NUMTIM`.** In `vmstime.go`: a time's numeric breakdown.
21. **Done.** **`$GETJPI` items for AST and scheduling state**: `JPI$_ASTACT`,
    `ASTEN`, `ASTCNT`, `ASTLM`, and `STATE`, with an `ASTLM` quota on
    `rtl.Process`.
22. **Done.** **Mode-switching AST delivery.** `NextAST` delivers a more
    privileged mode's AST by switching the CPU into that mode, as VMS
    does, and `$CLRAST` switches back, closing subtask 15's main
    deviation.
23. **Done.** **`$GETSYI`/`$GETSYIW`.** The `$GETJPI` pattern for system-wide
    information, with `$SYIDEF` generated from real VMS source.


Sixth batch, requested by the user on 2026-09-28 (the candidates the
fifth batch listed):

24. **Done.** **`$FAO`/`$FAOL`.** In the new `fao.go`: a directive
    registry, with the numeric directives generated from radix and size
    tables. Acceptance fixture `testdata/asm/fao.asm`.
25. **Done.** **`$GETMSG`/`$PUTMSG`.** In the new `message.go`, with
    the CLI, LIB, MTH, OTS, RMS, and SYSTEM message texts generated from
    the VMS 7.3 message file's listing. `$PUTMSG` formats a message
    vector with `formatFAO`, calling an action routine through a
    `CallRequest`. Acceptance fixture `testdata/asm/putmsg.asm`.
26. **Done.** **`$CMKRNL`/`$CMEXEC`.** In the new `cmode.go`: switch
    in, call the routine through subtask 19's `CallRequest` with the
    service's `XFC` as the return, and switch back when it returns, with
    its R0. Acceptance fixture `testdata/asm/cmkrnl.asm`.
27. **Done.** **CTRL/C and CTRL/Y ASTs.** `IO$_SETMODE` with
    `IO$M_CTRLCAST`/`IO$M_CTRLYAST` arms a one-shot AST (the new
    `ctrlast.go`) that the engine's attention check, through the new
    `cpu.AttentionHandler`, queues instead of stopping the machine.
    Acceptance fixture `testdata/asm/ctrlc_ast.asm`.
28. **Done.** **`$GETDVI`/`$GETDVIW` in full.** In the new `getdvi.go`:
    the `$GETJPI`/`$GETSYI` pattern over the device record, from a
    generated `$DVIDEF` and `$TTDEF`, replacing eVAX's three-item
    `$GETDVIW`. Acceptance fixture `testdata/asm/getdvi.asm`.
29. **Mailboxes.** `$CREMBX`/`$DELMBX` and a mailbox driver: the first
    device whose `$QIO` requests really wait (a read for a write), so
    `$QIOW` and `$CANCEL` gain real work.
30. **Small process-control services.** `$SETPRN`, `$SETPRI`, and
    `$FORCEX`/`$DELPRC` on this process.

## Open questions

None yet.

## Progress Log

### 2026-09-27 — Planning; subtask 1: emulated process record

- The user asked for `$ADJSTK`, `$ADJWSL`, `$ALLOC`, and `$ASCEFC`, one
  subtask and commit each, in a phase document that later services keep
  extending.
- Confirmed `reference/eVAX` implements none of the four (no matches outside
  `p1_vector.c`), and that the VMS 7.3 archive has no source for them. The
  VMS 5.0 System Services Reference Manual is the behavioral reference.
- **`rtl.Process`** (`internal/rtl/process.go`) holds the PID, username,
  UIC, and working-set state. `Environment.pid`/`uic` are gone; `$ASSIGN`
  stamps `Process.PID`/`Process.UIC`. `optArg` reads an omitted trailing
  argument as 0.
- Tests: `process_test.go` checks the default process (PID, SYSTEM, [1,4],
  quota ordering) and `optArg`. `go test ./...` passes.

### 2026-09-27 — Subtask 2: `$ADJSTK`

- `serviceSysAdjstk` (`internal/rtl/process.go`), per the design above.
  `registerProcessServices` is new and is called from `registerServices`.
- `SS$_NOPRIV` is read from `vmsdef.SSConstants` (`ssNoPriv`).
- Tests (`process_test.go`): each `adjust`/`newadr` combination from the
  manual's table, the low-word-only `adjust`, the caller's-own-mode and
  maximized-mode `SS$_NOPRIV` cases from kernel and user mode, an omitted
  argument list, and a zero `newadr`. `TestPhase26ServicesRegistered`
  dispatches each of this phase's services through `SystemService` at its
  real P1-vector address; later subtasks add their services to its list.

### 2026-09-27 — Subtask 3: `$ADJWSL`

- `serviceSysAdjwsl` (`internal/rtl/process.go`), per the design above,
  working on `Process.WSLimit`.
- Tests: report-only (`pagcnt` 0 and an omitted argument list), grow,
  shrink, clamping at both `WSEXTENT` and `MINWSCNT`, and an unwritable
  `wsetlm` leaving the limit unchanged.

### 2026-09-27 — Subtask 4: `$ALLOC`

- **`$DEVDEF`.** Copied `devdef.sdl` from the VMS 7.3 archive
  (`starlet_b64/lis/`; the `starlet/lis/` copy is identical) to
  `reference/vms/`. `internal/vmsdef/gen`'s SDL parser now accepts
  `aggregate NAME union prefix P$;` whose members are nested
  `MEMBER structure [fill];` ... `end MEMBER;` bitfield blocks, each numbered
  from bit 0. `go generate` produces `vmsdef.DEVConstants` (128 entries:
  `DEV$V_`/`DEV$M_` for 32 `DEVCHAR` and 32 `DEVCHAR2` bits). The
  `go:generate` line gained `-devdef`.
- **`iodev.Device`** gained `AllocMode`, `Allocated()`, `Allocate()`, and
  `Deallocate()`.
- **`serviceSysAlloc`**, `allocatable`, and `genericDevice` in
  `internal/rtl/devices.go`, per the design above. `$ASSIGN` checks
  another process's allocation. `Environment.ImageRundown` deallocates
  user-mode allocations, called from the console's `imageRundown`.
  SHOW DEVICE/FULL shows `allocated`.
- **Docs.** New "Phase 26 (system services) findings" section in
  `docs/DEVIATIONS.md`, with entries for `$ADJSTK`'s unprobed stack,
  the unenforced working-set limit, `$ALLOC`'s simplifications, and the
  `$ASSIGN` change.
- **Tests.**
  - `vmsdef/gen`: a union aggregate (including a quoted `"2P"` name), and
    two new rejected forms.
  - `vmsdef`: pinned `DEV$` values, and the prefix guard extended to
    `DEVConstants`.
  - `rtl/devices_test.go`: allocate by name and via a logical name,
    `SS$_DEVALRALLOC`, devnam-only calls, maximized access mode, image
    rundown (user mode released, others kept), every error status the
    service returns, `SS$_BUFFEROVF` with truncated output, generic
    allocation (skipping another process's device and its own,
    `SS$_NODEVAVL`, unknown type), and `$ASSIGN` refusing another
    process's device.
  - `console/device_test.go`: SHOW DEVICE/FULL's `allocated` for a disk and
    a terminal, and the console's image rundown releasing only the
    user-mode allocation.

### 2026-09-27 — Subtask 5: `$ASCEFC`; first batch complete

- **`internal/rtl/eventflags.go`** (new): `EventFlagCluster`,
  `CommonEventFlags` (with `Lookup` and `All` for a future SHOW command or
  `$GETJPI`), `serviceSysAscefc`, and `registerEventFlagServices`.
  `$SETEF`/`$CLREF`/`$READEF` moved here from `core.go` and share
  `eventFlagWord`, which maps a flag number to its local longword or its
  associated cluster.
- `Environment.EventFlags [4]uint32` is gone, replaced by
  `Process.LocalEventFlags` and `Process.CommonClusters`, plus
  `Environment.EventFlagClusters`. `ImageRundown` also disassociates
  clusters.
- **Decision:** the cluster table belongs to the Environment, not the
  console. It sits in system memory on VMS, and INIT/VMINIT/ZERO wipe
  memory. (The conventions section had guessed the console; corrected.)
- **Behavior change to existing services**, per the manual: `SS$_UNASEFC`,
  `SS$_ILLEFC`, and `$SETEF`/`$CLREF`'s `WASSET`/`WASCLR`. The one existing
  test that expected `SS$_NORMAL` from `$CLREF` of a set flag now expects
  `SS$_WASSET`. No fixture in `testdata/asm` uses event flags.
- **Acceptance fixture.** `testdata/asm/process_services.asm` calls
  `$ADJSTK`, `$ADJWSL`, `$ALLOC`, and `$ASCEFC` (then `$SETEF`/`$READEF` on
  the cluster) through their P1-vector addresses and checks each result
  itself. `internal/console/process_services_test.go` assembles and runs it,
  then checks the USP, working-set limit, TTA0's allocation, and the
  cluster's flags.
- **Docs.** `docs/DEVIATIONS.md` has two more Phase 26 entries (the
  event-flag changes, and `$ASCEFC`'s simplifications). `docs/PLAN.md` has
  a Phase 26 narrative.
- **Tests** (`eventflags_test.go`): `WASSET`/`WASCLR` and the low-byte rule,
  `ILLEFC`/`UNASEFC` for all three flag services, creating and using a
  cluster, `$READEF`'s state longword, associating one cluster with both
  numbers, the no-op reassociation, reassociation deleting a temporary
  cluster, rundown keeping a permanent cluster and its flags for the next
  image, protection by UIC, group-scoped names, and every `$ASCEFC` error
  status.
- `go test ./...` passes.
- **Phase status.** The four requested services are done. Candidates for
  the next batch are listed under Subtasks.

### 2026-09-27 — Second batch planned; subtask 6: `$DALLOC`

- The user asked for the candidates listed after the first batch. They
  became subtasks 6-9.
- **Wait-state design (for subtask 9), settled before starting.** Nothing
  in govax sets an event flag asynchronously: there are no ASTs, `$QIO`, or
  `$SETIMR`. The user confirmed that the timer interrupt is the only truly
  asynchronous operation. So an unsatisfied wait won't fail or return early:
  it re-executes the service's `XFC` (see the subtask 9 design). The process
  then waits in emulated time, with timer interrupts still delivered between
  retries.
- **`serviceSysDalloc`** and `hasChannel` in `internal/rtl/devices.go`, per
  the design above.
- Tests (`devices_test.go`): release by name and via a logical name;
  `SS$_DEVNOTALLOC` for a device that is unallocated or allocated to
  another process; `SS$_DEVASSIGN`; the mailbox no-op; `SS$_NOSUCHDEV`;
  both `SS$_IVLOGNAM` cases; `SS$_NOPRIV` for a more privileged allocation,
  with acmode maximized; and the no-`devnam` form releasing exactly the
  outer-mode allocations of this process.

### 2026-09-27 — Subtask 7: `$DACEFC` and `$DLCEFC`

- `serviceSysDacefc`, `serviceSysDlcefc`, and the shared `clusterName`
  (factored out of `$ASCEFC`) in `internal/rtl/eventflags.go`.
- `EventFlagCluster.DeletePending` is new. `CommonEventFlags.release` now
  goes through `deleteIfUnused`, which also deletes a marked permanent
  cluster.
- Tests (`eventflags_test.go`):
  - `$DACEFC` deleting a temporary cluster, keeping a permanent one, the
    low-byte rule, an unassociated number, and `SS$_ILLEFC`.
  - `$DLCEFC` deleting an unused permanent cluster at once, marking an
    associated one that stays usable until image rundown deletes it, a
    nonexistent name, and the name errors.

### 2026-09-27 — Subtask 8: `$GETJPI`/`$GETJPIW`

- **`$JPIDEF`.** Copied `jpidef.sdl` from the VMS 7.3 archive to
  `reference/vms/`. The SDL parser gained `%x` values, `NAME@N` shift
  expressions (looking up constants defined earlier in the file), and
  bitfield structures nested in a structure aggregate. `go generate`
  produces `vmsdef.JPIConstants` (219 entries), and the `go:generate` line
  gained `-jpidef`.
- **`internal/rtl/getjpi.go`** (new): `serviceSysGetjpi` (registered as
  both `SYS$GETJPI` and `SYS$GETJPIW`), the `jpiItemsByName` registry,
  `storeJPIItem`, and `jpiTarget`. The old `serviceSysGetjpiw` and its
  constants are gone from `core.go`, and its tests from `core_test.go`.
- `rtl.Process` gained `Name`, `Account`, `Terminal`, and `CLIName`.
- **Changed from eVAX:** `JPI$_ACCOUNT` returns the process's account
  (`SYSTEM`) rather than eVAX's stand-in `USER`, with a return length of 8
  (eVAX reported 4). `JPI$_CLINAME` writes only `DCL`, truncated to the
  buffer (eVAX always wrote 4 bytes, `DCL` plus a NUL, whatever the buffer
  length). An argument list of more than 7 longwords is now accepted.
  Recorded in `docs/DEVIATIONS.md`.
- Tests (`getjpi_test.go`):
  - every supported item's value and length, and the IOSB status;
  - truncation of a string and of a longword, and `JPI$_CHAIN`;
  - process selection by PID 0, its own PID, another PID, PID winning over
    `prcnam`, an exact name (with PID written back), abbreviated and
    lower-case names, name length errors, and the wildcard sequence;
  - completion (the event flag set, the default flag 0, `SS$_BADPARAM` in
    R0 and the IOSB, an unassociated `efn`), too few arguments, and the
    kept `DebugProcess` trace;
  - the registry naming only real `$JPIDEF` codes;
  - `vmsdef/gen` parser tests for the three new SDL forms and two new
    rejected forms.

### 2026-09-27 — Subtask 9: `$WAITFR`, `$WFLAND`, `$WFLOR`; second batch complete

- **`cpu.ErrServiceWait`** (`internal/cpu/services.go`).
  `emulXfcP1Vector` handles it by resetting PC to `e.instructionPC` and
  leaving R0 alone. `TestEmulXfcP1VectorWait` shows three waiting steps
  parked on the `XFC`, then completion continuing past it.
- **`rtl.ErrWait`**, `waitFor`, and the three services in
  `internal/rtl/eventflags.go`. The console's `translateHalt` maps
  `ErrWait` to `cpu.ErrServiceWait`. `Environment.SystemService` traces a
  wait once (`waitingPC`).
- **Acceptance fixture** `testdata/asm/wait_timer.asm`: it installs an
  interval-timer handler with `.SCB`, starts the clock, lowers IPL, and
  waits in `$WAITFR` for flag 3, which only the handler's `$SETEF` sets.
  Then it checks `$WFLAND`/`$WFLOR` on flags already set. The tests in
  `internal/console/process_services_test.go`:
  - `TestEventFlagWait_timerInterrupt` runs it to R0 = 1, so the timer
    interrupt ended the wait.
  - `TestEventFlagWait_blocksUntilSet` keeps ICCS clear so no tick can
    happen, shows the program parked on the `$WAITFR` stub after 5,000
    steps, then sets the flag from Go and lets the program finish.
- **rtl tests**: each service's satisfied and waiting cases, the empty
  masks, the low-byte rule, `SS$_ILLEFC`/`SS$_UNASEFC`, a wait on a common
  cluster, and the once-per-wait trace.
- `go test ./...` passes.
- **Phase status.** Both batches are done. Next candidates: `$DASSGN`, and
  `$SETIMR`/`$CANTIM`.

### 2026-09-27 — Third batch planned; subtask 10: `$DASSGN`

- The user asked for the suggested next services, `$DASSGN` and then
  `$SETIMR`/`$CANTIM`. For the timers, they asked for a judgment on
  whether to build on the microkernel's timer interrupt or keep the RTL
  timer independent (subtask 11).
- `serviceSysDassgn`, `releaseChannel`, and `deassignUserChannels` in
  `internal/rtl/devices.go`. `channel.Mode` is recorded by `$ASSIGN`.
  `deallocateAll` is shared by `$DALLOC` (no name) and image rundown.
- Found while testing: rundown's user-mode deallocation ignored channels,
  unlike `$DALLOC`. It now uses the same rule.
- Tests (`devices_test.go`): the low-word rule; the reference count and
  owner PID across two channels; `SS$_NOPRIV` for an unassigned channel
  and for a kernel channel from supervisor mode; `SS$_IVCHAN`; rundown
  releasing only the user-mode channel, and keeping an allocation that a
  kernel channel still holds; and `$DASSGN` then `$DALLOC` succeeding.

### 2026-09-27 — Subtask 11: `$SETIMR` and `$CANTIM`; third batch complete

- **Design:** the RTL timer queue is independent of the guest's timer
  interrupt but shares the engine's clock. The reasoning is in the design
  section above.
- **`internal/cpu/systime.go`**: `Engine.SystemTime`, backed by the new
  `bootTime`/`clockTicks` fields. `tickQuantum` counts `clockTicks`.
- **`internal/vmsdef/time.go`**: `Time` and `UnixEpoch`
  (1-Jan-1970 = `0x007C95674BEB4000`, as pinned by a test).
- **`internal/rtl/timers.go`** (new): the timer queue, `serviceSysSetimr`,
  `serviceSysCantim`, `expireTimers`, `cancelTimers`, and `PendingTimers`.
  `Environment.Clock` defaults to the host clock; the console's `newRTL`
  binds it to `Engine.SystemTime`. `eventFlagWord` now expires timers
  first; the old lookup is `flagWord`. `ImageRundown` cancels timers.
- **Acceptance fixture** `testdata/asm/timer_services.asm`: set a 20ms
  timer and cancel it, set a 50ms one and `$WAITFR` on it, then check that
  the cancelled timer's flag is still clear. It does no interrupt setup at
  all. `TestTimerServices_assembledProgram` also checks that the engine's
  system time advanced at least 50ms during the run.
- **Tests:**
  - `cpu`: quantum ticks advance `SystemTime` by exactly 1ms each.
  - `vmsdef`: epoch conversions.
  - `rtl/timers_test.go`: delta expiry exactly at the deadline, absolute
    times (future and past), the default flag 0, `$WAITFR` on a timer,
    error statuses with nothing queued, a common cluster (including a
    disassociated one), `$CANTIM` by ID, all, and access mode (with
    maximization), and rundown cancelling.
- `go test ./...` passes.

### 2026-09-28 — Fourth batch planned; subtask 12: `$GETTIM`

- The user asked for the listed candidates: `$GETTIM`, `$BINTIM`/`$ASCTIM`,
  `$SCHDWK`/`$HIBER`/`$WAKE`, and AST delivery. Planned as subtasks 12-16:
  `$GETTIM`; `$ASCTIM`/`$BINTIM`; `$HIBER`/`$WAKE`/`$SCHDWK`/`$CANWAK`;
  the AST delivery mechanism with `$DCLAST`/`$SETAST`; and ASTs for
  `$SETIMR`/`$GETJPI`.
- **Decisions (asked of the user):**
  - System time is **local time**, as on VMS, not UTC.
  - ASTs are delivered only at **IPL < 2**, as on VMS. Programs run after a
    normal boot are in user mode at IPL 0 (kernel.asm's `exe$initialize`).
    (The question assumed the Go fixture tests start at IPL 31. Subtask 15
    found the console is at kernel mode, IPL 0 after VMINIT, so they
    don't need to lower it.)
- `serviceSysGettim`, `loadQuad`, and `storeQuad` in the new
  `internal/rtl/vmstime.go`. `$SETIMR` reads `daytim` with `loadQuad`.
- `vmsdef.Time` adds the zone offset; `vmsdef.GoTime` and
  `vmsdef.TicksPerSecond` are new.
- Tests: `$GETTIM` against a hand-set clock, its `SS$_ACCVIO` cases; the
  local/UTC readings of one instant; `GoTime` round trips. The epoch test
  now converts a UTC time.

### 2026-09-28 — Subtask 13: `$ASCTIM` and `$BINTIM`

- `formatVMSTime`, `parseVMSTime` (with `parseAbsoluteTime`,
  `parseTimeOfDay`, `parseTimeNumber`), `serviceSysAsctim`, and
  `serviceSysBintim` in `internal/rtl/vmstime.go`, per the design above.
- **Found while testing:** `vmsdef.Time` and `GoTime` went through
  nanoseconds since 1970, which overflow an int64 outside 1678-2262. They
  now work in seconds plus a remainder, so 31-DEC-9999 converts correctly.
- Tests (`vmstime_test.go`):
  - the manual's `$BINTIM` example table, run through both conversions;
  - more accepted forms (blanks, truncated dates, a leap day, time zero,
    the last representable time, rounding carry, the largest delta);
  - 25 rejected strings, one or more for each rule;
  - `formatVMSTime`'s padding, truncation, time-only form, and 10,000-day
    limit;
  - `$ASCTIM` for now, a given delta, and a 12-byte buffer
    (`SS$_BUFFEROVF`), plus its error statuses;
  - `$BINTIM` storing a time, a `$BINTIM` delta driving `$SETIMR`, and its
    error statuses leaving `timadr` untouched.
- `vmsdef`: a 9999 round-trip case.

### 2026-09-28 — Subtask 14: `$HIBER`, `$WAKE`, `$SCHDWK`, `$CANWAK`

- `internal/rtl/hibernate.go` (new): the four services and
  `registerHibernateServices`. `Process.WakePending` is new.
- `timers.go`: `timerRequest.wake`/`repeat`; `expireTimers` wakes the
  process and reschedules repeating entries; `$CANTIM` skips wakeups.
- `getjpi.go`: `jpiTarget` became the shared `processTarget`, with a
  `wildcard` switch only `$GETJPI` sets. `ErrWait`'s message no longer
  mentions event flags.
- **Acceptance fixture** `testdata/asm/hibernate.asm`: `$WAKE` then an
  immediate `$HIBER`; a `$SCHDWK` at 30ms repeating every 10ms and three
  `$HIBER`s; `$CANWAK`. `TestHibernate_assembledProgram` checks R0, at
  least 50ms of system time, and an empty queue.
- Tests (`hibernate_test.go`): waiting until `$WAKE` and consuming it;
  uncounted wakeups; process selection and its errors (-1 isn't a
  wildcard); delta and absolute (future and past) `$SCHDWK`; repeats,
  including collapsed missed repetitions staying on the grid; the 10ms
  minimum; every `$SCHDWK` error leaving nothing queued; `$CANWAK` versus
  `$CANTIM` and a pending `$WAKE`; rundown cancelling a repeating wakeup.

### 2026-09-28 — Subtask 15: AST delivery, `$DCLAST`, `$SETAST`

- **`internal/cpu/ast.go`** (new): `ASTCall`, the optional `ASTSource`
  interface, and `Engine.deliverAST`. `SetSystemServices` records
  `astSource`, and `Step` calls `deliverAST` after interrupt delivery. A
  frame fault is raised with the routine's address as the PC.
- **`internal/rtl/ast.go`** (new): `astState` (in `Process.ast`),
  `queueAST`, `PendingASTs`, `NextAST`, `pushASTFrame`, and the services
  `$DCLAST`, `$SETAST`, and `$CLRAST` (the AST exit, via the new
  `ServiceTable.RegisterNoArgs`/`ReadsArgs`). `ImageRundown` calls
  `flushUserASTs`.
- `$SETAST` moved from `core.go` and became per mode, returning
  `WASSET`/`WASCLR`. `Environment.astEnabled` is gone.
- `internal/console/services.go`: `Console.NextAST` bridges the two
  packages. Stale "no AST delivery" comments in `console/set.go` and
  `cpu/call.go` were updated.
- **Correction:** the IPL question (subtask 12) assumed the fixture tests
  start at IPL 31. The console is at kernel mode, IPL 0 after VMINIT. The
  fixture sets IPL 0 anyway; changing that to IPL 2 was checked to make it
  fail with the AST still queued.
- **Acceptance fixture** `testdata/asm/ast_delivery.asm`: a `$DCLAST`
  delivered immediately (R0, R1, and R2 intact afterwards, though the
  routine clobbers them); `$SETAST(0)` holding a second AST back;
  `$SETAST(1)` releasing it. `TestASTDelivery_assembledProgram` also checks
  the routine saw five arguments and nothing is left queued.
- **Tests:**
  - `cpu/ast_test.go`: a delivered AST's `CALLG` frame (saved PC is the
    return address, entry-mask registers saved, AP at the argument list);
    `RET` leaving SP at the argument list; `Step` asking at each boundary
    and running the routine's first instruction; the bad-routine fault;
    no `astSource` for services without it.
  - `rtl/ast_test.go`: `$DCLAST` queuing with maximized modes; the frame
    `NextAST` pushes; one active AST per mode; FIFO order; each
    delivery condition (other modes, IPL 2, interrupt stack, disabled
    here or in a more privileged mode); the missing-stub and bad-stack
    errors; `$CLRAST` restoring everything, ignoring non-AST calls and
    unbalanced stacks, refusing a forged mode, and reached with an
    unreadable AP; `$SETAST`'s statuses, low byte, and held-back AST;
    rundown.
- `go test ./...` passes.

### 2026-09-28 — Subtask 16: ASTs from `$SETIMR` and `$GETJPI`; fourth batch complete

- `timers.go`: `timerRequest.astadr`; `$SETIMR` records it and
  `expireTimers` queues the AST (parameter `reqidt`, the timer's mode).
- `getjpi.go`: a completed request queues its AST (`astprm`, the caller's
  mode).
- Docs: the `$GETJPI`, wait, and `$SETIMR` sections and their
  `DEVIATIONS.md` entries no longer say ASTs aren't delivered.
- **Acceptance fixture** `testdata/asm/timer_ast.asm`:
  - `$SETIMR` with an AST whose routine calls `$WAKE`, ending a `$HIBER`;
  - a `$CANTIM`-cancelled timer's AST never running;
  - a timer AST breaking a spin loop that calls no services;
  - `$GETJPIW`'s AST having run by the time the call returns.

  `TestTimerAST_assembledProgram` also checks the system time advanced at
  least 70ms, `JPI$_PID` was returned, and nothing is left queued.
- Tests (`rtl/ast_test.go`): a timer's AST (mode, parameter, and flag),
  also expired by `NextAST` alone; no AST from a cancelled, rundown, or
  AST-less timer; a timer on a disassociated cluster still queuing its
  AST; `$HIBER` interrupted by a timer AST that wakes it, resuming on
  `$HIBER`'s `XFC`; `$GETJPI`'s AST on success and on `SS$_BADPARAM`, and
  none when rejected early or without `astadr`.
- `go test ./...` passes.
- **Phase status.** The fourth batch (subtasks 12-16) is done. Candidates
  for the next batch are listed under Subtasks.

### 2026-09-28 — Fifth batch planned; subtask 17: terminal `$QIO`/`$QIOW`, `$CANCEL`

- Planned the fifth batch (subtasks 17-23) from the fourth batch's
  candidate list.
- **`$IODEF`**: `reference/vms/iodef.sdl`, imported from the VMS 7.3
  `starlet_b64` sources, generated into `vmsdef.IOConstants` (544
  symbols). `gen/sdl.go` now understands `#NAME = V` local symbols,
  bitfield lengths that use them (`16-#fcode_size`), and constants inside
  an aggregate (default prefix, tag `K`). `TestIOConstants_values` pins
  the function codes and terminal modifiers.
- **`internal/rtl/qio.go`** (new): `serviceSysQio` (registered as both
  `SYS$QIO` and `SYS$QIOW`), `serviceSysCancel`, the `ioDrivers`
  registry, and `accessible` (a buffer check through `ProbeTranslate`,
  also bounded by memory size since it passes everything with MAPEN off).
- **`internal/rtl/ttdriver.go`** (new): the terminal driver's function
  table: reads, prompted reads, writes with FORTRAN and print-file
  carriage control, sense/set mode, and the type-ahead count.
- **Acceptance fixture** `testdata/asm/terminal_qio.asm`: a prompted
  `$QIOW` read, an asynchronous `$QIO` write with an AST and
  `$WAITFR`, a `$QIOW` write, an illegal function, `$CANCEL`, and
  `$DASSGN`. `TestTerminalQIO_assembledProgram` feeds it `Tom` and
  checks the output is `"Name? \nHello, \rTom"`. (The assembler's
  default radix is hex: decimal numbers above 9 need `^D`.)
- Tests (`rtl/qio_test.go`): writes, with the IOSB, flag, and AST; each
  carriage-control form; reads ending by terminator, full buffer, and end
  of input (with and without data); `"\r\n"`; `CVTLOW`, `PURGE`, and
  zero-time `TIMED` reads; short- and long-form terminator sets; the
  prompted read; sense/set mode in both sizes and with a modifier; the
  type-ahead count; every rejection (flag set, IOSB untouched or cleared
  as appropriate, no AST); low-word `chan`/`func`; `$CANCEL`.
- `go test ./...` passes.

### 2026-09-28 — Subtask 18: `$SYNCH`

- `serviceSysSynch` in `eventflags.go`: waits for the flag, then tests the
  IOSB's status word, clearing the flag and waiting again on a false
  alarm.
- **Acceptance fixture** `testdata/asm/synch.asm`: `$SYNCH` after a
  `$GETJPI` returns at once; then two timers share event flag 4, the
  first a false alarm at 10ms and the second at 30ms with an AST that
  writes the IOSB, and `$SYNCH` returns only after the second.
  `TestSynch_assembledProgram` checks at least 30ms passed. A mutant
  `$SYNCH` that returns on the false alarm fails it.
- Tests (`eventflags_test.go`): each step, including only the status word
  counting; no IOSB; the default flag; the errors; `$SYNCH` after a
  `$QIO`.
- `go test ./...` passes.

### 2026-09-28 — Subtask 19: `$EXIT`, `$DCLEXH`, `$CANEXH`

- **`internal/rtl/exit.go`** (new): `CallRequest`, `ErrExit`, the three
  services, `ExitHandlers`, and `cancelUserExitHandlers` (called by
  `ImageRundown`). `Process` gains `exitHandlers` and `ExitStatus`.
  eVAX's `$DCLEXH` stub (`core.go`), `Environment.exitHandler`, and its
  test are removed.
- **`internal/cpu`**: `ServiceCall` and `ErrImageExit` (`services.go`);
  `exit.go` (new) with `callForService` and `exitImage`;
  `emulXfcP1Vector` handles both. The console's `translateHalt` maps the
  RTL's signals to them, and `SystemService`'s trace shows "calls" and
  "exits with status".
- **RUN**: `buildImageInitDriver` appends `PUSHL R0`, `CALLS #1,
  SYS$EXIT` after the call to `main` when `p1Stub` finds the stub. The
  milestone `.exe` tests (`TestRun_everyMilestoneFixture`, with
  `kernel.asm`'s P1 vector) now end through `$EXIT`, unchanged.
- **Acceptance fixtures**: `testdata/asm/exit_handlers.asm`, run in user
  mode: three handlers declared, one cancelled, `$EXIT(^X2C)` calling the
  other two newest first with the status, and never returning.
  `testdata/asm/exit_on_return.asm`, run through the RUN driver: `main`
  returns 7 and its handler still runs and sees 7
  (`TestRun_driverCallsExitHandlers`).
- Tests: `cpu/exit_test.go` (the call's frame and return to the XFC with
  R0 untouched; unwinding two frames to the console's; halting with no
  console frame). `rtl/exit_test.go` (`$DCLEXH` links and errors;
  `$CANEXH` relinking, all, and errors; `$EXIT`'s order, status
  argument, argument-less blocks, default status, skipped unreadable
  blocks, only the caller's mode; rundown).
- `go test ./...` passes.

### 2026-09-28 — Subtask 20: `$NUMTIM`

- `numericTime` and `serviceSysNumtim` in `vmstime.go`.
- **Acceptance fixture** `testdata/asm/numtim.asm`: `$BINTIM` then
  `$NUMTIM` for an absolute time (a leap day, the last hundredth) and a
  delta; `TestNumtim_assembledProgram` checks all fourteen fields.
- Tests (`vmstime_test.go`): the base date, absolute and delta times,
  truncated hundredths, the longest delta, the current time, and each
  error.
- `go test ./...` passes.

### 2026-09-28 — Subtask 21: `$GETJPI` AST and scheduling items

- `getjpi.go`: seven new registry entries, with `astModeMask` and
  `remainingASTs`. `process.go`: `ASTLimit`, `Priority`,
  `BasePriority`.
- **`$STATEDEF`**: `reference/vms/statedef.txt`, extracted by script from
  `trace/lis/lib.lis` (the archive has no `$STATEDEF` source), generated
  as `vmsdef.STATEConstants` through the existing BLISS-literal parser.
- **Acceptance fixture** `testdata/asm/jpi_ast_state.asm`: the items read
  normally, with ASTs disabled, from inside an AST routine (ASTACT's
  kernel bit set), and with a timer AST outstanding (ASTCNT 23).
  `TestJPIASTState_assembledProgram` checks all seven values.
- Tests (`getjpi_test.go`): the new items' defaults in the item test; the
  AST items against a set-up queue, timers (only `$SETIMR` ones with an
  AST count), and per-mode flags; ASTCNT floored at 0.
- `go test ./...` passes.

### 2026-09-28 — Subtask 22: mode-switching AST delivery

- `ast.go`: `NextAST` picks the most privileged deliverable mode at or
  inside the CPU's; `enterASTMode` (replacing `pushASTFrame`) switches
  into it and pushes the frame, undoing the switch on failure;
  `switchMode` saves and loads the per-mode stack pointers;
  `serviceSysClrast` switches back to a less privileged saved mode, never
  to a more privileged one. `internal/cpu/ast.go`'s comment notes the
  stack may already be the inner mode's.
- **Acceptance fixture** `testdata/asm/mode_switch_ast.asm`: a timer set
  from kernel mode, an `REI` to user mode, and a user-mode `$HIBER` that
  the kernel-mode AST's `$WAKE` ends. `TestModeSwitchAST_assembledProgram`
  (memory management on, so the stacks' protection is real) checks the
  AST ran in kernel mode with user as the previous mode, and the program
  continued in user mode.
- Tests (`ast_test.go`): the switch and return (mode, previous mode,
  stacks, frame PSL, KSP restored); most privileged first, and no
  outer-mode AST inside an inner-mode one; a failed push undoing the
  switch; a forged frame lowering but never raising the mode.
  `TestNextASTConditions` now checks only that a less privileged AST
  waits.
- `docs/DEVIATIONS.md`: the AST entry's first point is resolved.
- `go test ./...` passes.

### 2026-09-28 — Subtask 23: `$GETSYI`/`$GETSYIW`; fifth batch complete

- **`$SYIDEF`**: `reference/vms/syidef.txt`, the VMS 7.3 VEST listing,
  generated as `vmsdef.SYIConstants` (309 symbols). `gen/bliss.go`'s
  literal pattern now accepts the VEST `I4` type
  (`TestParseBlissLiterals_vestListing`).
- **`internal/rtl/getsyi.go`** (new): `serviceSysGetsyi` (both entry
  points), `nodeTarget`, and the item registry. `Environment` gains
  `NodeName` and `BootTime`; the console resets `BootTime` after binding
  the engine's clock.
- **`itemlist.go`**: the item-value helpers shared by `$GETJPI` and
  `$GETSYI` (`itemValue`, `itemString`, `itemPadded`, `itemByte`,
  `itemWord`, `itemLong`, `itemQuad`, `storeItem`), replacing
  `getjpi.go`'s `jpiValue` family.
- **Acceptance fixture** `testdata/asm/getsyi.asm`: the manual's example
  (version and node name) and a wildcard scan counting nodes.
  `TestGetsyi_assembledProgram` checks both strings and one node.
- Tests (`getsyi_test.go`): every item's bytes (including the quadword
  boot time and the byte cluster flag), a truncated item, node selection
  by CSID, name, both, and wildcard, with each error, completion with
  flag, IOSB, and AST, `SS$_BADPARAM` still completing, rejection
  completing nothing, omitted optional arguments, and the registry.
- `go test ./...` passes.
- **Phase status.** The fifth batch (subtasks 17-23) is done. Candidates
  for the next batch are listed under Subtasks.

### 2026-09-28 — Sixth batch planned; subtask 24: `$FAO`/`$FAOL`

- The user asked for the fifth batch's candidates, `$FAO` through the
  small process-control services. They became subtasks 24-30. (Subtask
  17 is also now marked done in the list; it was done, only unmarked.)
- **`internal/rtl/fao.go`** (new): `formatFAO`, the `faoFormatter` that
  parses directives, the `faoDirectives` registry (the numeric entries
  generated by `init` from `faoRadixes` and `faoSizes`), and the
  services `serviceSysFao`/`serviceSysFaol` sharing `faoOutput`.
- The VMS 7.3 SYSTEM message texts (for subtask 25) use `!XH`, so the
  VMS 7 size letters are included.
- **Found while writing the fixture:** the assembler's `.ASCIC` stores a
  16-bit count, where MACRO-32's is one byte, so `!AC` can't read it.
  Recorded under "Open findings" in `docs/DEVIATIONS.md`; the fixture
  uses `.BYTE` and `.ASCII`.
- **Acceptance fixture** `testdata/asm/fao.asm`: `$FAO` with parameters
  in the call and `$FAOL` with a list, each written by `$QIOW`, and
  `SS$_BADPARAM` for an unknown directive. `TestFAO_assembledProgram`
  checks the terminal output.
- Tests (`fao_test.go`): the manual's examples 1-10; each directive
  family, the field rules (octal and hexadecimal widths, asterisks,
  string truncation and padding, `!n<` truncation), sign extension,
  quadwords, `!%S`'s case, `!%U`/`!%I`, `!n*c` inside a repeat, `#` for
  both counts; `SS$_BADPARAM` keeping the output before it,
  `SS$_BUFFEROVF`, omitted `outlen`, the `SS$_ACCVIO` cases, and `$FAO`'s
  parameters past p20.
- `go test ./...` passes.

### 2026-09-28 — Subtask 25: `$GETMSG`/`$PUTMSG`

- **Message texts.** `reference/vms/sysmsg.txt` is the six facilities'
  blocks, each `.FACILITY` line through its `.END`, copied verbatim by
  script from the VMS 7.3 `msgfil/lis/sysmsg.lis` (the whole listing has
  40-odd facilities and 4,010 messages; these six are the ones a govax
  program meets). `internal/vmsdef/gen/msg.go` (new) parses it; `go
  generate` gained `-sysmsg` and `-msgout` and now also writes
  `internal/vmsdef/messages_generated.go`. `vmsdef.Message` and
  `LookupMessage` are in the new `messages.go`.
- **Changed from the plan:** the plan said other facilities' texts would
  come from `internal/vmserrors`. They don't: its RMS and CLI codes
  aren't VMS's real values (its CLI facility is 2, VMS's is 3), and its
  VAX facility (15) is VMS's RUF, so it would show wrong texts for real
  condition values. The VMS message file covers the facilities instead.
- **`internal/rtl/message.go`** (new): `messageFor`, `messageLine`,
  `serviceSysGetmsg`, `serviceSysPutmsg`, `parseMessageVector`, and
  `putmsgCall` (in the new `Process.putmsg`); `ImageRundown` calls
  `cancelPutmsgCalls`.
- **Acceptance fixture** `testdata/asm/putmsg.asm`: `$GETMSG` for the text
  alone; the manual's `$PUTMSG` example through an action routine that
  lets only the first line through; `SS$_ACCVIO` with its parameters.
  `TestPutmsg_assembledProgram` checks the output and the line length the
  routine saw.
- Tests: `vmsdef/gen` (each listing-line form, the two errors, the
  parameter count); `rtl/message_test.go` (`$GETMSG`'s flag combinations,
  severity from `msgid`, unformatted text, `outadr`, other facilities,
  the stand-in, truncation, and errors; `$PUTMSG`'s vector forms, default
  and new flags, `facnam`, unformattable texts, and errors; the action
  routine's stack layout, R0 deciding, SP restored, one argument without
  `actprm`, and rundown; pinned generated messages).
- `go test ./...` passes.

### 2026-09-28 — Subtask 26: `$CMKRNL`/`$CMEXEC`

- **`internal/rtl/cmode.go`** (new): `serviceSysCmkrnl`,
  `serviceSysCmexec`, their shared `changeMode`, and `cmodeCall` (in the
  new `Process.cmode`); `ImageRundown` calls `cancelChangeModeCalls`.
  The conventions' "calling guest code" entry now describes telling a
  returning call from a new one.
- **Found while writing the fixture:** VMINIT sets ESP and SSP to the
  kernel stack. With memory management on, executive mode can't write
  it, so `$CMEXEC`'s call frame faulted (an access violation at the
  stub). The fixture gives executive mode a stack of its own, as VMS
  would; recorded in `docs/DEVIATIONS.md`.
- **Acceptance fixture** `testdata/asm/cmkrnl.asm`: after an `REI` to user
  mode, `$CMKRNL` runs a routine that records its mode and previous mode,
  executes `MFPR`, and returns a status built from its argument;
  `$CMEXEC` runs one in executive mode. `TestCmkrnl_assembledProgram`
  (memory management on) checks the modes, the `MFPR`, both statuses, and
  user mode afterwards.
- Tests (`cmode_test.go`): the two runs of a `$CMKRNL` from user mode
  (mode, previous mode, stacks, the routine's R0); the target mode for
  each service from each caller mode, including `$CMEXEC` from kernel;
  a nested call; rundown.
- `go test ./...` passes.

### 2026-09-28 — Subtask 27: CTRL/C and CTRL/Y ASTs

- **`internal/cpu/attention.go`** (new): `AttentionCtrlC`/`AttentionCtrlY`,
  the optional `AttentionHandler` interface, and `checkAttention`, which
  `Step` now calls. `Engine.attentionRequested` became `attentionKey`
  (which key); `Attention` is `AttentionKey(AttentionCtrlC)`;
  `SetSystemServices` finds the handler.
- **`internal/rtl/ctrlast.go`** (new): the requests (`attentionAST`,
  `Environment.attentionASTs`), `armAttentionAST`, `disarmChannel`, and
  `Attention`, which applies VMS's rules. `ttdriver.go`'s `ttSetMode`
  sends `IO$M_CTRLCAST`/`CTRLYAST` to the new `ttAttentionAST`;
  `$CANCEL` and `releaseChannel` (`$DASSGN`, rundown) disarm.
- **`internal/console/services.go`**: `Console.HandleAttention` makes the
  console a `cpu.AttentionHandler`.
- **`cmd/govax/attention.go`**: a comment on why host Ctrl-Y isn't
  intercepted.
- **Acceptance fixture** `testdata/asm/ctrlc_ast.asm`: a CTRL/C AST, then a
  CTRL/Y AST that a CTRL/C reaches. `TestCtrlCAST_assembledProgram`
  "types" CTRL/C (`Engine.Attention`) as the program reaches each spin
  loop, and checks each AST ran with its parameter and the program
  finished rather than stopping.
- Tests: `cpu/attention_test.go` (a taken key runs the instruction and
  clears; a declined one stops until `BeginRun`; no handler stops);
  `rtl/ctrlast_test.go` (delivery, mode maximized, one-shot, CTRL/C not
  catching CTRL/Y, CTRL/C falling back to CTRL/Y, replacement per
  channel, several channels, `p1` 0, `$CANCEL`, `$DASSGN`, rundown);
  the console's handler and the two packages' keys agreeing.
- `docs/DEVIATIONS.md`: the terminal `$QIO` entry no longer says CTRL/C
  ASTs aren't delivered; a new CTRL/C entry.
- `go test ./...` passes.

### 2026-09-28 — Subtask 28: `$GETDVI`/`$GETDVIW`

- **`$DVIDEF` and `$TTDEF`**: `reference/vms/dvidef.txt` and `ttdef.txt`,
  the VMS 7.3 VEST listings (`vest_dblrtl/lis/`), generated as
  `vmsdef.DVIConstants` and `TTConstants` (flags `-dvidef`, `-ttdef`).
  `gen/bliss.go` accepts mixed-case names (`DVI$_SHDW_spare_bit_1`).
- **`internal/rtl/getdvi.go`** (new): `serviceSysGetdvi` (both entry
  points), `dviTarget`, the item registry with its generated Booleans,
  `devChar` and `volumeInfo` (live mount state). eVAX's
  `serviceSysGetdviw` and its `dvi*` constants are gone from `devices.go`,
  and its tests from `devices_test.go`.
- **Changed from eVAX:** items are the sizes VMS returns (a longword for
  `DEVCLASS`, truncated to a shorter buffer); the argument list needs at
  least four arguments, not exactly eight; an unassigned channel is
  `SS$_NOPRIV`. Recorded in `docs/DEVIATIONS.md`.
- **Acceptance fixture** `testdata/asm/getdvi.asm`: `$GETDVIW` by channel
  (`DEVNAM`, `DEVCLASS`, `UNIT`, `REFCNT`), then `$GETDVI` by `SYS$OUTPUT`
  for `FULLDEVNAM` with `$SYNCH`. `TestGetdvi_assembledProgram` checks the
  values.
- Tests: `vmsdef` pins `DVI$`/`TT$`/`TT2$` values and the prefix guard;
  `gen` a mixed-case literal; `rtl/getdvi_test.go` (the items of a
  terminal, a disk's unit, the secondary flag, a one-byte buffer, the
  registry covering every `DVI$_TT_` code, completion, `SS$_BADPARAM`
  completing, each rejection completing nothing, `SS$_INSFARG`, an
  unwritable IOSB, the debug trace). The logical-name test now uses
  `$GETDVI`.
- `go test ./...` passes.

