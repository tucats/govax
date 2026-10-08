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
- SYS$GETDVIW/SYS$ASSIGN/SYS$TRNLNM (the RTL system-service calls built on top of this
  phase's data structures) and channel assignment need the RTL calling convention
  (argc/argv marshaling from VAX memory) and a process/PID/UIC concept, neither of
  which exist yet — left for Phase 10.

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

### 2026-09-14 — Sub-phase 2: console wiring, close out Phase 09

- Added `internal/console/device.go`: `Console.DefineDevice`/`ShowDevices`
  (`define_device.c`/`show_device.c`) and `Console.DefineLogical`/
  `ShowLogicals` (`define_logical.c`/`show_logical.c`) against the new
  `Console.Devices`/`Logicals` fields (`machine.go`) — matching those C
  handlers' own lack of a `vax_init` check (devices/logical tables exist
  independent of `INIT`, so these commands work before/without one).
  `Console.New` seeds `Logicals` via `InitLogicals` unconditionally,
  matching `init_symbols.c`'s `init_system_symbols` calling
  `logical_names.c`'s `init_logicals()` once as part of one-time process
  startup (not gated behind `INIT` either).
  `ShowDevices` replicates `show_device.c`'s own asymmetry against
  `ShowLogicals`/`show_logical.c`: a no-match `SHOW DEVICE` prints nothing
  at all, while a no-match `SHOW LOGICAL_NAMES` prints "No matching
  logical names." — confirmed in the C source, not an inconsistency
  introduced by this port.
- Wired `DEFINE_DEVICE`/`SHOW_DEVICE`/`DEFINE_LOGICAL`/`SHOW_LOGICAL` into
  `dispatch.go`'s `bindGrammar`, closing the two `SHOW`/`DEFINE`
  sub-dispatches (`show_types`'s `devices`/`logical_names` keywords,
  `define`'s `device`/`logical` qualifiers) Phase 08's close-out entry
  explicitly deferred here. `DEFINE/LOGICAL`'s default table
  ("LNM_PROCESS" when `/TABLE` isn't given) is applied in the bind
  closure, matching `define_logical.c`'s own default.
- Found and fixed a real bug in Phase 08's `internal/console/dcl` package
  while wiring `DEFINE_DEVICE`'s `DEVCLASS`/`DEVTYPE` qualifiers (both
  keyword-typed, per `evax.dcl`'s `dev_class`/`dev_type` grammar types):
  `dcl.Result.Int`'s own doc comment promises "the matched Keyword's ID
  for a keyword-typed value," but a keyword match and a plain string match
  were both stored with only `isString = true` and no further tag, so
  `Int` (which returned 0 for any `isString` value) silently returned 0
  for every keyword-typed qualifier — never previously caught because no
  Phase 08 command happened to call `Int` on a keyword-typed field
  (`SHOW_TYPE`'s keyword dispatch, the only prior keyword-typed consumer,
  uses `Keyword` instead). Separately, `String`/`Keyword` shared the same
  under-specified check, so `String` would (contrary to its own doc
  comment) return a keyword's display name instead of `""`, and `Keyword`
  would (contrary to *its* doc comment) accept a plain string field. This
  is RTL-console tooling, not emulated VAX ISA behavior — like the rest of
  `internal/console/dcl` (Phase 08's own scope note) — so
  `docs/DEVIATIONS.md`'s ISA-fidelity policy doesn't apply; it's a clear,
  obvious doc/implementation mismatch, fixed directly per `CLAUDE.md`'s
  bug-fixing policy rather than logged and deferred. Fixed by adding an
  explicit `Value.IsKeyword`/`matchedValue.isKeyword` discriminator (set
  only at the one keyword-match construction site in `resolveValue`) and
  updating all three accessors to check it; verified not to regress any
  existing caller (`SHOW_TYPE`'s `Keyword` call, and every existing
  `Int`/`String` test, all target fields whose type was already
  unambiguous under the old code). Regression test:
  `internal/console/dcl/parse_test.go`'s
  `TestResult_keywordValueDiscriminator`, using this phase's own
  `DEFINE/DEVICE .../DEVCLASS=DISK` as the concrete keyword-typed case.
- `internal/console/device_test.go` covers: `DefineDevice`/`ShowDevices`
  (plain and `/FULL`, including the disk-class-only detail block and the
  no-match-prints-nothing case) and `DefineLogical`/`ShowLogicals`
  (including the no-match message and that `Logicals` comes pre-seeded at
  construction) directly against `Console`, plus `DEFINE/DEVICE`/
  `SHOW DEVICES`/`DEFINE/LOGICAL` (both with and without `/TABLE`)/
  `SHOW LOGICAL_NAMES` end-to-end through the real DCL grammar via
  `Dispatcher.Dispatch`. `dispatch_test.go`'s
  `TestDispatch_unboundShowSubformErrors` (which asserted `SHOW DEVICES`
  was unbound, true before this sub-phase) now targets `SHOW NVRAM`, still
  genuinely unbound, to keep checking that an unimplemented `SHOW`
  sub-form still reports a clear dispatch error.
- Full `docs/DEVIATIONS.md`-policy review for this phase: no VAX ISA/
  hardware-fidelity findings — `devices.c`/`logical_names.c` and
  `internal/console/dcl` are all RTL/console tooling, not emulated VAX
  instruction-set behavior. The two real bugs found this phase
  (`get_dev_class_name`'s loop bound, sub-phase 1; the `dcl.Result`
  keyword-value discriminator, this sub-phase) are both documented at the
  point they were found rather than only here, per this project's own
  established pattern (e.g. Phase 08's SHOW PAGE/READ/WRITE fix).
- Full-repo `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and
  `go test ./...` all clean; `go test ./internal/console/... -cover`:
  79.2%; `go test ./internal/console/dcl/... -cover`: 80.9%. Phase
  complete.
