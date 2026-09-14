# Phase 10: RTL simulators

## Goal

Emulate the VMS runtime-library/system-service calling convention on top of the CPU:
argument-list marshaling, RMS file I/O, the CLI, and the LIB$ math/string/time/print/
memory utility routines.

**Scope note (2026-09-14, expanded then re-bounded before implementation started):**
starting this phase surfaced a cluster of work other phases explicitly deferred *into*
Phase 10 by name, beyond what the original scope note above says:

- **XFC (opcode 0xFC)** — Phase 07 left this entirely `unimplementedHandler`
  (`docs/PHASE-07.md`'s open questions): it's eVAX's own escape hatch, and selector
  `0x7A` (`XFC$P1VECTOR`) is literally how a `CHMK`/SYS$-style call reaches this phase's
  dispatch code (`reference/eVAX/eVAX/Source/CPU/emul_xfc.c`'s `call_service`), selector
  `0x7D` (`XFC$SHIM`) is how a LIB$/CRTL call reaches `shim()`. Per user direction
  (2026-09-14), this phase implements all of XFC, including the console-callback
  selectors (`0x01`-`0x03` console read/write/dispatch-command, `0x79` DCL parse
  callback) that don't strictly need RTL — closing out the opcode completely rather than
  leaving a partial handler.
- **SYS$ASSIGN/GETDVIW/TRNLNM and channel assignment** — Phase 09 explicitly deferred
  these to Phase 10 (`docs/PHASE-09.md`'s open questions), needing the RTL calling
  convention this phase now provides and a process/PID/UIC concept that doesn't exist
  yet. Per user direction (2026-09-14), this phase adds a **minimal process stub** (a
  channel table, a nominal PID/UIC) rather than real process management, just enough to
  make these three services work against `internal/io`.
- **`console_run.c`'s `RUN` command and the `CALL` verb** — Phase 08 deferred both to
  "Phase 10/11" (`docs/PHASE-08.md`'s close-out notes). Investigating this while scoping
  Phase 10 found `RUN` (real `.exe` image activation: ICB/ISD/IHD/IHI struct mapping,
  sharable-image fixups, `LIB$INITIALIZE` calling) to be a large, separable concern
  layered *on top of* the RTL calling convention rather than a small wiring step — **split
  out into a new `docs/PHASE-13.md`** (see `docs/PLAN.md`'s phase table) rather than
  ballooning this phase further. Phase 10's own RTL surface (SYS$/LIB$ services, RMS,
  CLI) is fully exercised by direct unit tests that construct a scenario in `vm.Memory`
  and invoke the relevant registry entry, so it doesn't need a working image loader to be
  complete and closed out on its own.
- **Design constraint, per explicit user direction while scoping this phase**: every
  dispatch-by-code-or-name subsystem this phase adds (SYS$ services, LIB$/shim routines,
  XFC selectors) is a registry/function-table keyed by code or name — the same shape as
  `internal/cpu`'s instruction `Table`/`Handler`/`SetHandler`/`HandlerFor` — not a switch
  statement, so future phases can register more entries without restructuring dispatch.
  See memory note `feedback_table_driven_dispatch` for the rationale.

With `RUN` moved to Phase 13, this phase's own milestone becomes: every SYS$/LIB$
service, RMS operation, and CLI request the C source actually implements is ported and
unit-tested, ready for Phase 13 to drive through a real loaded image.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/emul_xfc.c` — the XFC opcode (all selectors).
- `reference/eVAX/eVAX/Source/RTL/p1_vector.c`, `shim.c` — dispatch `SYS$`/`LIB$`
  system-service calls by reading an argument-list vector out of emulated VAX memory.
  This is the foundational piece the rest of this phase builds on. Confirmed by reading
  `p1_vector.c`'s `declare_services()`: only 11 `SYS$` services are ever actually wired
  to a handler (`SYS$CLREF/SETEF/READEF/EXPREG/TRNLNM/DCLEXH/ASSIGN/GETDVIW/GETJPIW/
  SETAST/CLI`, plus RMS's `CREATE/CONNECT/PUT` below) — the rest of `p1_vector[]`'s ~250
  entries are real VMS service *addresses* (so a program that calls them doesn't
  immediately fault "no such vector") with a null handler, reported as "Unimplemented
  native service" if actually invoked. Same shape as `shim.c`'s `rtl_entry_list`: only
  codes 1-32 have a registered handler.
- `reference/eVAX/eVAX/Source/RTL/service.c` — `sys_setast`/`sys_dclexh`/`sys_expreg`/
  `sys_clref`/`sys_setef`/`sys_readef`/`sys_getjpiw`.
- `reference/eVAX/eVAX/Source/RTL/devices.c` — `sys_getdviw`/`sys_assign` (this phase's
  half of that file; device/channel *data structures* are Phase 09's, already in
  `internal/io`).
- `reference/eVAX/eVAX/Source/RTL/logical_names.c` — `sys_trnlnm` (ditto: the logical
  name table itself is Phase 09's `internal/io.LogicalNameTable`).
- `reference/eVAX/eVAX/Source/RTL/cli.c` — `sys_cli`. Confirmed minimal: only the "get
  symbol" request is even partially handled (returns `CLI$_UNDSYM`, unconditionally,
  since symbol tables aren't implemented); every other request is an explicit
  `default:`/unknown-request halt in the C source itself, replicated as-is rather than
  invented.
- `reference/eVAX/eVAX/Source/RTL/rms.c` + `structure_mapping.c` — RMS file I/O,
  mapping FAB/RAB struct fields (`reference/eVAX/eVAX/Headers/fab.h`/`rab.h`) onto VAX
  memory via a declarative offset table (`structure_mapping.c`'s `map()`, `STROFF`
  macro in `memmap.h`). `AUDIT.md` fixed FAB/RAB native-pointer fields to store VAX
  addresses (not host pointers) — the Go port should use a VAX-address type
  consistently here from the start rather than repeat that class of bug. Confirmed by
  reading `rms.c` in full: only `SYS$CREATE`/`SYS$CONNECT`/`SYS$PUT` are implemented (no
  read/close/etc. exist in the C source at all) — sequential organization only.
- `reference/eVAX/eVAX/Source/RTL/librtl_file.c`, `librtl_input.c`, `librtl_math.c`,
  `librtl_memory.c`, `librtl_print.c`, `librtl_strings.c`, `librtl_time.c`,
  `librtl_utils.c` — individual LIB$/CRTL shim routines, dispatched by numeric code via
  `shim()`/`XFC$SHIM`, not by symbolic address like the `SYS$` table above.

## Deliverables

- `internal/rtl` package implementing the calling convention (argument-list marshaling
  from a VAX arg pointer) and the library routines above as registry entries, depending
  on `internal/io` (Phase 09) for device/logical-name resolution.
- `internal/cpu`'s XFC opcode, fully implemented against a small `SystemServices`
  hook interface (so `internal/cpu` doesn't need to import `internal/console`/
  `internal/rtl`) that `internal/console.Console` implements, delegating SYS$/shim
  selectors to an embedded `*rtl.Environment`.
- Tests: unit tests per LIB$ routine and per SYS$ service where feasible (constructing
  the argument scenario directly in `vm.Memory`, no image loader needed), plus RMS
  round-trip tests (create/connect/put through the emulated FAB/RAB layer, including the
  `TTA0:`-to-console-output special case `rms_create`/`rms_put` hard-code).

## Open questions / notes

- The string-symbol native-pointer-truncation bug class `AUDIT.md` fixed (N1: adding a
  proper `svalue` field instead of casting through `LONGWORD`) is a general pattern to
  watch for anywhere this phase needs to carry a host-side string/pointer alongside a
  VAX-side numeric value — Go's type system makes the two impossible to conflate by
  construction, but keep the underlying scenario in mind when structuring RTL argument
  types.
- `librtl_memory.c`'s `decc_malloc`/`lib_get_vm`/`lib_free_vm` grow the P0 region via
  `get_region_size`/`set_region_size`, which in the C source read/write a VAX memory
  cell addressed indirectly through an `EXE$P0_RGN` symbol — that indirection exists
  only because `console_run.c` (now Phase 13) and this phase's allocator are separate
  translation units needing to share one value the C way. The Go port skips the
  indirection and shares an `Environment.RegionSize` field directly; Phase 13 will read/
  write the same field when it lands, not reintroduce the symbol-table hop.
- `docs/PHASE-13.md` (split out of this phase) has the full research on why `RUN` is a
  separate phase and confirms it doesn't actually need Phase 11's assembler.

## Progress Log

### 2026-09-14 — Sub-phase 1: XFC opcode

- Added `internal/cpu/services.go` (`SystemServices` hook interface — console
  I/O, DCL parse callback, SYS$ dispatch, LIB$/CRTL shim dispatch) and
  `internal/cpu/xfc.go` (full `emul_xfc.c` port), closing out Phase 07's
  deferred XFC opcode. `Engine.SetSystemServices` installs the hook; every
  selector that needs it (0x01-0x03, 0x79, 0x7A, 0x7D) faults
  `ExcPrivileged` when none is installed, matching an XFC executed before
  the microkernel environment it depends on exists. The VM-probe selectors
  (0x7E/0x7F) and quit/halt (0x78/0x7B/0x7C) need no hook — implemented
  directly against `Engine`'s own CPU/memory.
- `XFC$P1VECTOR`'s `pc` argument is `vax.PC - 2` (the XFC opcode's own
  address) rather than `emul_xfc.c`'s literal `vax.PC - 4` — this port's
  `Engine.Step` advances PC past the whole instruction (opcode + one-byte
  immediate operand = 2 bytes total) before dispatching, not just past a
  4-byte fixed-format opcode word the way the C source's inline dispatch
  loop does; verified by `TestEmulXfcP1Vector` asserting the exact `pc`
  value a fake hook receives.
- One pre-existing `internal/vm.Translate` edge case found (not fixed, out
  of this phase's scope): translating virtual address 0 with `MAPEN=1` and
  an all-zero `P0BR`/`P0LR` recurses infinitely (`page > P0LR` is `0 > 0` =
  false, so page 0 is treated as in-range, and its PTE address resolves to
  0 again). Not exercised by any existing test; found by an early draft of
  this phase's own VM-probe test using address 0, which was changed to a
  realistic nonzero address instead of chasing a Phase 02/03-owned fix here.
  Worth a look in Phase 12 if a real page-zero access ever needs to work.
- `internal/cpu/xfc_test.go` covers every selector against a `fakeServices`
  test double: console write/read/command-dispatch (including the
  empty-length no-op), quit/halt/halt-silent, all four DCL subfunctions
  (including the "no buffer available" `DCLGetString` case) and an unknown
  subfunction fault, SYS$ dispatch (handled, unhandled-faults, and an
  error propagated from the hook), shim dispatch (handled and
  unhandled-faults), both VM-probe selectors' failure path plus MAPEN
  restoration, an unknown top-level selector, and the no-services-installed
  fault path. The kernel-mode check for `XFC$QUIT_EMULATION` is tested by
  calling `emulXfc` directly rather than through `Engine.Step`, following
  `procreg_test.go`'s established pattern for avoiding the `setModeStack`
  MAPEN side effect (docs/DEVIATIONS.md) a real User→Kernel mode switch
  would otherwise require a page table for.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean.
