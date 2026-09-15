# Phase 13: VMS image activation (RUN)

## Goal

Port `console_run.c`'s `RUN` command: load a real VMS `.exe` file into P0 space,
resolve its sharable-image dependencies, apply load-time fixups, and transfer control
to it — the last piece needed to run `testdata/exe/*.exe` as real user-mode programs
rather than hand-assembled test fixtures.

This phase was split out of Phase 10 (RTL simulators) once starting that phase's own
investigation found `RUN` to be a large, separable concern layered *on top of* the RTL
calling convention, not a small wiring step alongside it — see `docs/PHASE-10.md`'s own
notes on the split and `docs/PLAN.md`'s phase-table note. Phase 10's RTL layer (SYS$/
LIB$ services, RMS, CLI) is fully exercised by direct unit tests (construct a scenario in
`vm.Memory`, invoke the relevant registry entry, assert the result) and doesn't need a
working image loader to be complete on its own.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/Console/console_run.c` — the whole phase: `console_run`
  (the `RUN` verb), `image_load`/`image_fixup` (the actual loader/linker),
  `init_ihd_maps` (IHD/IHI/ISD/IAF field-offset tables), `find_image`/
  `image_file_found` (locating the `.exe` file), `get_region_size`/`set_region_size`
  (P0/P1/S0 region-size bookkeeping, also consumed by Phase 10's `LIB$GET_VM`/`malloc`
  allocator).
- `reference/eVAX/eVAX/Headers/imgdef.h` — `IHD`/`IHI`/`ISD`/`IAF`/`SHR`/`ICB` struct
  layouts and flag bits.
- `reference/eVAX/kernel.asm` — the microkernel bootstrap source assembled by `vax.init`
  at startup; its `.shim` pseudo-op table (lines ~1146-1177, ~1546-1555) is the source of
  truth for every `SHIM$<library>_<vector-offset>` symbol a sharable-image G^ fixup can
  resolve to. Confirmed by direct inspection: this is the *only* place these symbols are
  defined in the whole C source (`shim.c`'s own `rtl_entry_list` only assigns numeric
  dispatch codes 1-32 to a handful of them; most `.shim` entries in `kernel.asm` carry a
  literal `0` and resolve to a dead/unimplemented stub even in the real C emulator).

## Key finding: this does not actually require Phase 11's assembler

`console_run`'s own C implementation leans on `assemble_direct` (build `PUSHL`/`CALLS`
sequences as text, assembled on the fly into `CONSOLE$SCRATCH`) for two things: calling
each dependency's `LIB$INITIALIZE` entry point before the main image, and the final
`CALLS #0,@#IMAGE$MAIN` / `RET` that starts the program. Neither actually needs a real
assembler:

- A "call this entry point with these arguments" primitive can be built directly against
  `internal/cpu`'s existing CALLS machinery (`internal/cpu/call.go`) or by writing a
  short, fixed instruction sequence's raw opcode bytes straight into scratch memory (this
  project already encodes/decodes every VAX opcode byte-for-byte in `internal/cpu`, so
  there's no missing capability here — just a few bytes to place directly instead of
  through a text-parsing assembler). This is also where Phase 07's deferred "RET's
  console-CALL-command sentinel" (the magic `FFFFDEAF` frame) finally gets a consumer:
  a `Console.Call` that plants that sentinel as the return address and runs `Engine.Step`
  until it's hit is the natural way to know the called routine has returned.
- The `SHIM$<library>_<offset>` symbols that sharable-image G^ fixups resolve to don't
  need textual assembly either: `kernel.asm`'s `.shim` table (see above) is a small,
  fully-enumerable `(library, vector offset) -> (name, numeric shim code or "dead")`
  list. A Go port can synthesize the equivalent tiny stub (`MOVL #code,R0` / `XFC #0x7D`
  / `RET`, or a dead `XFC #0x7D` with an intentionally-unregistered code for the
  literal-`0` entries) directly as bytes at `internal/rtl` construction time, and register
  each stub's address under the matching `SHIM$...` name in whatever symbol table `RUN`'s
  fixup resolver consults — no parser needed.

Confirmed against the actual fixtures (not just reasoned about): every fixture in
`testdata/exe/` except `dbl.exe`/`getvm.exe` (not in Phase 10's named milestone list) has
exactly one `ISD_M_FIXUPVEC` section and at least one `ISD_M_GBL` (sharable-image)
dependency — `put.exe`/`getvm.exe` depend on `LIBRTL_001`; `putc.exe`/`sieve.exe`/
`simple.exe` additionally depend on `DECC$SHR_001`, `MTHRTL_001`, and `CMA$TIS_SHR_001`;
`cli.exe` has no `ISD_M_GBL` sections at all (statically self-contained, only a
`FIXUPVEC` ISD for internal relocation). None of the small (2048-byte) fixtures'
sharable-image dependencies are present as loadable files in `testdata/` — matching
`image_load`'s own tolerance for a missing secondary image (`VAX_FNF` is swallowed, not
fatal) — so this phase's fixup step only ever needs to resolve to a `SHIM$` stub for
these fixtures, never to a real loaded LIBRTL/DECC$SHR image. A stub that isn't actually
registered with real behavior (e.g. `LIB$PUT_OUTPUT`, `DECC$MAIN` — both literal-`0` in
`kernel.asm`) only becomes a problem if the loaded program actually calls it; whether
`put.exe`'s compiled code exercises SYS$ services directly (already fully addressable, no
fixup needed — Phase 10 territory) versus an unimplemented LIBRTL entry point is only
knowable by actually loading and running it, which is this phase's job to find out.

## Deliverables

- Port `image_load`: read the IHD/IHI/ISD headers via a Go equivalent of
  `init_ihd_maps`'s declarative field table, load each ISD's pages (including DZRO
  zeroing), track the P0 high-water mark via Phase 10's `Environment.RegionSize`,
  recursively resolve the SHR (sharable-image) list, tolerating a missing secondary image
  file exactly as the C source does.
- Port `image_fixup`: G^ and `.ADDRESS` fixup lists, resolving against either an already-
  loaded ICB's base address or a synthesized `SHIM$` stub (see above).
- A `Console.Call` primitive (entry address/symbol, argument list, optional single-step)
  built on `internal/cpu`'s CALLS machinery plus the Phase 07 `FFFFDEAF` sentinel, used
  both for per-dependency `LIB$INITIALIZE` calls and the final main-image transfer.
- `Console.Run` wiring the `RUN` verb through the above, matching `console_run`'s own
  `/NOINIT`, `/INIT`, `/BREAK`/`/DEBUG`/`/STEP`, `/NOEXECUTE` qualifiers.
- Milestone check: `testdata/exe/put.exe`, `putc.exe`, `cli.exe`, `sieve.exe`,
  `simple.exe` load and run end-to-end (to whatever extent their actual code paths are
  covered by Phase 10's RTL surface — a program calling an unregistered `SHIM$`/`SYS$`
  entry is expected to report that clearly, not silently misbehave).

## Progress Log

### 2026-09-15 — Sub-phase 1: CONSOLE$SCRATCH, Engine.CallEntry, Console.Call

- `internal/console/vminit.go`'s `VMInit` now reserves the S0
  `CONSOLE$SCRATCH` page (one page after the privileged-mode stacks,
  matching `console_vminit_dcl`'s own placement) and defines the symbol --
  previously deferred at Phase 08 for lack of a consumer; this phase's
  image loader and SHIM$ stub synthesis (sub-phases to follow) are that
  consumer. `TestVMInit_reservesConsoleScratch` checks it round-trips a
  store/load in kernel mode.
- `internal/cpu/call.go`: factored `emulCall`'s frame-construction body out
  into `Engine.buildCallFrame` (parametrized by the frame's return PC/FP,
  rather than always reading the live registers), and added
  `Engine.CallEntry` on top of it -- builds a CALLS-shaped frame directly
  (no instruction fetch/decode) with `SentinelReturn` (`0xFFFFDEAF`) as the
  return PC/FP, the same magic value `console_exec.c`'s `console_call`
  uses. `emulRet` now recognizes a popped frame with both PC and FP equal
  to `SentinelReturn` and reports `ErrConsoleCallReturned` instead of
  resuming at that address -- the Go equivalent of `emul_call.c`'s
  `vax.console.CALL_active`/FFFFDEAF check, minus the C source's
  `CALL_active` guard flag (redundant here: `SentinelReturn` is deliberately
  a value no real CALLS instruction can ever produce, so there's no
  legitimate-program false-positive to guard against). Only the zero-
  argument case is implemented -- see `CallEntry`'s doc comment for why
  that's sufficient for this phase's actual call site (RUN's single call to
  its synthesized `IMAGE$INIT` driver procedure; that procedure's own inner
  CALLS to `LIB$INITIALIZE`/the main image use ordinary, already-correct
  PUSHL/CALLS instructions, not this primitive).
  `TestEngineCallEntryRunsUntilSentinelReturn` builds a two-level call chain
  and steps it to completion.
- `internal/console/call.go`'s `Console.Call` wraps `CallEntry` in a
  step-until-done loop (mirroring `execute.go`'s `Execute`'s HALT handling),
  with an optional per-instruction trace for the future `/STEP` qualifier.
  `TestCall_returnsCleanlyThroughSentinelFrame` exercises it end-to-end.
- Renamed `internal/console/run.go`/`run_test.go` to `execute.go`/
  `execute_test.go` (per the user's request) since their actual export is
  `Console.Execute`/`Console.Step` (the GO/EXEC/STEP commands), freeing the
  `run.go` name for this phase's own `RUN <filename>` command once it
  lands.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test
  ./...` all clean.
