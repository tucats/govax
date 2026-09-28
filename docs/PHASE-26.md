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
and AST delivery. The fifth adds terminal , , exit
handlers, , more  items, mode-switching AST delivery,
and .

**Status: fifth batch in progress** (subtasks 17-23). Add later services
as new subtasks.

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
  `devices.go` (`$ASSIGN`, `$GETDVIW`, `$ALLOC`), `logicals.go`, `cli.go`,
  `rms.go`, `process.go` (process record and process-control services),
  `eventflags.go` (event flags and common event flag clusters), `timers.go`
  (the timer queue), `vmstime.go` (`$GETTIM` and time conversion),
  `hibernate.go` (`$HIBER`, `$WAKE`, scheduled wakeups), `ast.go` (AST
  delivery), `qio.go` (`$QIO` and the driver registry), `ttdriver.go`
  (the terminal driver's functions). Each file has
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
  device allocations, common event flag associations) runs from
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
| `$GETJPI`, `$GETJPIW` | 8 | `getjpi.go` | `NORMAL`, `ACCVIO`, `BADPARAM`, `ILLEFC`, `INSFARG`, `IVLOGNAM`, `NOMOREPROC`, `NONEXPR`, `UNASEFC` | 21 item codes from `rtl.Process`, in a registry keyed by the generated `$JPIDEF` codes. |
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
these hold, as on VMS:

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

- **No mode switch.** VMS delivers an inner-mode AST to a process running
  in an outer mode by switching to that mode. govax delivers an AST only
  while the CPU is in the AST's own mode. A kernel program queuing a
  user-mode AST sees it only after it drops to user mode, as on VMS; but
  a user-mode program with a kernel AST pending doesn't get it until it
  enters kernel mode. So an inner-mode AST doesn't interrupt an
  outer-mode wait.
- The `ASTLVL` register and `REI`'s AST check are not used.
- No `ASTLM` quota (`SS$_EXQUOTA`) and no `SS$_INSFMEM`.
- `JPI$_ASTACT`, `ASTEN`, and `ASTCNT` aren't reported by `$GETJPI` yet.

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

17. **`$QIO`/`$QIOW` on terminal channels, and `$CANCEL`.** In the new
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
19. **`$EXIT`, `$DCLEXH`, `$CANEXH`: exit handlers.** Per-mode handler
    lists (replacing the single recorded `exitHandler`). `$EXIT` calls
    each handler through a new "service asks the engine to make a call"
    path, then ends the image by unwinding to the console's call frame.
    RUN's image driver calls `$EXIT` with `main`'s status, as VMS's
    image activator does.
20. **`$NUMTIM`.** In `vmstime.go`: a time's numeric breakdown.
21. **`$GETJPI` items for AST and scheduling state**: `JPI$_ASTACT`,
    `ASTEN`, `ASTCNT`, `ASTLM`, and `STATE`, with an `ASTLM` quota on
    `rtl.Process`.
22. **Mode-switching AST delivery.** `NextAST` delivers a more
    privileged mode's AST by switching the CPU into that mode, as VMS
    does, and `$CLRAST` switches back, closing subtask 15's main
    deviation.
23. **`$GETSYI`/`$GETSYIW`.** The `$GETJPI` pattern for system-wide
    information, with `$SYIDEF` generated from real VMS source.


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
