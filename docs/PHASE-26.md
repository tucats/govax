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
| `$ADJSTK` | 2 | | | |
| `$ADJWSL` | 3 | | | |
| `$ALLOC` | 4 | | | |
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

Planned (see subtask 2).

### `$ADJWSL` — Adjust Working Set Limit

`SYS$ADJWSL [pagcnt] ,[wsetlm]`

Planned (see subtask 3).

### `$ALLOC` — Allocate Device

`SYS$ALLOC devnam ,[phylen] ,[phybuf] ,[acmode] ,[flags]`

Planned (see subtask 4).

### `$ASCEFC` — Associate Common Event Flag Cluster

`SYS$ASCEFC efn ,name ,[prot] ,[perm]`

Planned (see subtask 5).

## Subtasks

1. **Done.** **Emulated process record.** `rtl.Process` replaces
   `Environment`'s `pid`/`uic` fields; `$ASSIGN` reads the PID/UIC from it.
   Adds `optArg` for omitted trailing arguments. This document.
2. **`$ADJSTK`.**
3. **`$ADJWSL`.**
4. **`$ALLOC`.**
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
