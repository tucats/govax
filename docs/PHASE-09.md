# Phase 09: I/O & device support

## Goal

Port device abstraction and logical name tables — the layer that gives the emulator
symbolic device/file names and a pluggable device model, sitting under both the
console and the RTL.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/RTL/devices.c` — device abstraction (note: this file
  lives under the C tree's `Source/RTL/`, but `docs/PLAN.md` calls out I/O as its own
  concern distinct from the RTL library simulators — kept as a separate phase here per
  that plan, even though the C source doesn't draw the line at a separate directory).
- `reference/eVAX/eVAX/Source/RTL/logical_names.c` — VMS-style logical name tables.

## Deliverables

- `internal/io` package with a device abstraction and logical-name-table
  implementation, consumed by Phase 08 (console SHOW DEVICE, etc.) and Phase 10 (RTL
  file I/O resolving logical names).
- Unit tests: logical name define/translate/delete, device registration and lookup.

## Open questions / notes

- Confirm the device model's shape once RMS (Phase 10) requirements are clearer — file
  I/O and device abstraction are tightly coupled in VMS, so this phase may need minor
  revisiting once Phase 10 starts.

## Progress Log

### 2026-09-14 — Sub-phase 1: device abstraction and logical name tables

- Added `internal/io`: `Device`/`DeviceTable` (`device.go`, porting
  `devices.c`'s `struct DEVICE`/`find_device`/`define_device`/
  `get_dev_class_name`) and `LogicalName`/`LogicalNameTable`
  (`logical.go`, porting `logical_names.c`'s `struct LNM`/`get_logical`/
  `set_logical`/`init_logicals`). Both C files are RTL/emulator tooling,
  not emulated VAX ISA behavior (like Phase 08's DCL engine), so
  `docs/DEVIATIONS.md`'s ISA-fidelity policy doesn't apply — findings
  below are just fixed or documented here directly.
- `get_dev_class_name`'s `for (n = 0; n < 100; n++)` loop over a 3-element
  static array (reading 97 elements past its end) is a plain out-of-bounds
  loop-bound bug; `DeviceClassName` uses a small map instead, sidestepping
  it rather than replicating it.
- `logical_names.c`'s `struct LNM` plays two roles (a table header, whose
  `tables` field points at its list of name entries; a plain name entry,
  whose own `tables` field is always nil) — split into an unexported
  `logicalTable` (header) and the exported `LogicalName` (entry) rather
  than one overloaded Go type, matching Phase 08's own precedent for
  restructuring the C source's own tooling structs when doing so doesn't
  lose any behavior. Confirmed it doesn't: `get_logical`'s recursive
  `LNM$TABLE`-alias fallback (used when a table name has no direct header)
  can never actually match anything in this codebase, in C or here — its
  own `attr` filter requires the found name entry's `attr` to equal
  `LNM_M_TERMINAL` exactly, but every table this project ever creates
  (via `Set`'s own unconditional `LNM_M_TABLE` header stamp, or
  `InitLogicals`' literal `LNM_M_TABLE` argument) is registered with attr
  `LNM_M_TABLE`, never `LNM_M_TERMINAL` — traced, not assumed. `Get`'s own
  doc comment records this rather than reproducing a no-op code path as
  inert complexity.
- `LogicalNameTable.Delete` is a Go-native addition with no direct C
  equivalent (`logical_names.c` has no delete function; `SYS$DELLNM`
  appears in `p1_vector.c`'s system-service jump table but its routine
  pointer is `0`, never implemented) — added because this phase's own
  deliverable names "define/translate/delete" as the expected test
  coverage, the same kind of low-risk CRUD-completeness addition as
  Phase 08's DEPOSIT command.
- Deliberately not ported yet (Phase 10's territory, needing the RTL
  system-service calling convention this package doesn't touch):
  `SYS$GETDVIW`, `SYS$ASSIGN`, `SYS$TRNLNM` (the argc/argv-from-VAX-memory
  marshaling and item-list walking in `devices.c`/`logical_names.c`), and
  `CHAN`/channel assignment (`SYS$ASSIGN`'s device-to-channel binding,
  which also needs a process/PID/UIC concept this project doesn't have
  yet).
- `internal/io/{device,logical}_test.go` cover: `DeviceClassName`'s table
  and fallback cases; device define/find (including trailing-`:`
  stripping and a duplicate-name definition shadowing the older one, not
  replacing it); logical name set/get (including the `LNM$ROOT` default,
  redefinition leaving `attr` unchanged, and the `attr` exact-match
  filter); `Delete`; `InitLogicals` against the real seeded values; and
  `AllMatching`'s table/name filtering.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test
  ./...` all clean. `go test ./internal/io -cover`: 98.6%.
