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
`$DACEFC`/`$DLCEFC`, `$GETJPI`, and the event-flag waits.

**Status: third batch in progress** (subtasks 10-11); subtasks 1-9 are
complete. Add later services as new
subtasks.

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
  `eventflags.go` (event flags and common event flag clusters). Each file has
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
  wait design). Any later waiting service ($HIBER, $SYNCH, ...) should use
  the same mechanism.
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

An argument list shorter than 7 is `SS$_INSFARG`, as before. `astadr` is
accepted but no AST is delivered: govax has no AST delivery.

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

Not implemented: VMS's "wait interrupted by an AST, then resumed" (govax
has no AST delivery); the process state (`LEF`/`CEF`) and `JPI$_EFWM` wait
mask; and giving up the host CPU while waiting, since each retry costs an
emulated instruction step.

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
11. **`$SETIMR` and `$CANTIM`,** with the timer design question the user
    raised: whether they should use the microkernel's timer interrupt, or
    an RTL timer of their own.

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
