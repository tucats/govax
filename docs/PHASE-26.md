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
`$ADJWSL`, `$ALLOC`, and `$ASCEFC`.

**Status: in progress.**

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
- **Process state.** Anything that belongs to "the calling process" goes in
  `rtl.Process` (`env.Process`), with a comment naming the VMS field (PCB,
  PHD, JIB, UAF) it stands in for. System-wide state that must outlive one
  Environment (for example common event flag clusters) is owned by the
  `Console` and injected into the Environment after it's built, like
  `Session`.
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
| `$ASCEFC` | 5 | | | |

## Service designs

### The emulated process (`rtl.Process`)

Before this phase, `Environment` had unexported `pid`/`uic` fields, set from
constants and read only by `$ASSIGN` to stamp a device's owner. They're
replaced by `Environment.Process *Process` (`internal/rtl/process.go`):

| Field | Stands in for | Default |
| --- | --- | --- |
| `PID` | `PCB$L_EPID` | `0x00000301` (arbitrary, nonzero) |
| `Username` | `JIB$T_USERNAME` | `SYSTEM` |
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

Planned (see subtask 5).

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
5. **`$ASCEFC`.**

Candidates for later subtasks, since they complete the facilities this batch
starts: `$DALLOC` (the inverse of `$ALLOC`), `$DACEFC`/`$DLCEFC` (disassociate
and delete common event flag clusters), `$WAITFR`/`$WFLOR`/`$WFLAND` (event
flag waits), and a fuller `$GETJPI` reading `rtl.Process`.

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
