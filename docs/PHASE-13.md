# Phase 13: VMS image activation (RUN)

## Goal

Port `console_run.c`'s `RUN` command: load a real VMS `.exe` file into P0
space, resolve its sharable-image dependencies, apply load-time fixups, and
transfer control to it — the last piece needed to run `testdata/exe/*.exe`
as real user-mode programs rather than hand-assembled test fixtures.

This phase was split out of Phase 10 (RTL simulators) once starting that
phase's own investigation found `RUN` to be a large, separable concern layered
*on top of* the RTL calling convention, not a small wiring step alongside it —
see `docs/PHASE-10.md`'s own notes on the split and `docs/PLAN.md`'s phase-
table note. Phase 10's RTL layer (SYS$/LIB$ services, RMS, CLI) is fully
exercised by direct unit tests (construct a scenario in `vm.Memory`, invoke
the relevant registry entry, assert the result) and doesn't need a working
image loader to be complete on its own.

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

- Port `image_load`: read the IHD/IHI/ISD/IAF headers via a Go equivalent of
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

### 2026-09-15 — Sub-phase 2: image_load

- `internal/console/image.go`: `ICB`/`ISD`/`IAF`/`SHR` Go types and
  `Console.imageLoad`, ported from `console_run.c`'s `image_load` and
  `imgdef.h`. Per the precedent already set by `internal/rtl/rms.go`'s own
  FAB/RAB field access, this reads each struct's fields directly at their
  documented byte offsets (taken from `init_ihd_maps`'s own table) rather
  than porting `structure_mapping.c`'s generic name-keyed `map()`/`STROFF`
  machinery, which exists in the C source only to serve interactive
  EXAMINE/DEPOSIT struct support this port doesn't have.
- Reads go through `vm.Memory`'s ordinary virtual-address Load/Store calls
  (kernel mode, VM already on via VMINIT) rather than the C source's
  translate-to-physical-then-poke-the-array dance (`vm(&paddr,...)` then
  `vax.memory[paddr]`) -- functionally identical since kernel mode already
  has full access to every P0/S0 page VMINIT mapped, and significantly
  simpler than reproducing raw physical-address plumbing that exists in C
  only because there was no higher-level virtual accessor to call instead.
- Found and fixed one clear, obvious bug while porting the ISD walk (not a
  DEVIATIONS.md matter -- this is loader/tooling code, not emulated VAX
  instruction-set behavior, the same category Phase 10's own RTL/console
  findings fell into): `image_load`'s "no user stack loads" skip compares
  the unsigned, byte-extracted `ISD_B_TYPE` value against `-3`, which it
  can never equal (the real constant, `ISD_K_USRSTACK`, is `253`) -- dead
  code in the C source. Fixed directly by comparing against `253`.
- Confirmed by hand against a raw hexdump of `testdata/exe/simple.exe`
  (its `IHD.offset_ident` locates an `IHI` block at file offset `0x60`
  whose name field reads "SIMPLE" directly in the raw bytes, and its
  transfer array's first two longwords are `0x7FFEDF68`/`0x00000200`,
  matching `console_run`'s own transfer[0]-then-transfer[1] main-entry
  fallback) -- `TestImageLoad_simpleExe` checks these ground-truth values,
  plus the loaded ICB's FIXUPVEC/SHR-list shape. Chasing this down also
  surfaced a real ambiguity worth recording here rather than in
  DEVIATIONS.md (it's a file-format-reading detail, not an ISA question):
  several of `simple.exe`'s ISDs carry a *fuller*, version-suffixed name in
  their own `ISD.NAME` field (e.g. `"DECC$SHR_001"`, a GBL-section-type
  naming convention this port doesn't otherwise consume) that is easy to
  confuse with the *shorter*, unversioned names the IAF's own sharable-
  image name list carries (`"DECC$SHR"`) -- confirmed correct because the
  short form is what matches `kernel.asm`'s own `.shim` pseudo-op library
  field spellings, which `SHIM$<name>_<offset>` fixup resolution (next
  sub-phase) depends on matching exactly.
- `TestImageLoad_everyRealFixtureLoads` runs `imageLoad` against every real
  fixture in `testdata/exe/` (`put1.exe` excluded, per
  `docs/PHASE-12.md`'s own scope note -- a zero-byte file) as a smoke test;
  `TestImageLoad_alreadyLoadedIsNoOp` and
  `TestImageLoad_missingFileReportsError` cover `image_load`'s other two
  documented outcomes.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test
  ./...` all clean.

### 2026-09-15 — Sub-phase 3: SHIM$ stub synthesis, image_fixup

- Found and fixed two real bugs in `internal/console/vminit.go`'s `VMInit`
  while writing this sub-phase's own tests (the first XFC$SHIM dispatch to
  actually run *after* a VMINIT, rather than only after a bare INIT --
  every prior XFC/RTL test happened not to exercise this exact
  combination): `VMInit` replaces `c.Engine` and `c.Mem` (a fresh, wiped
  address space) but was never re-running `c.Engine.SetSystemServices(c)`
  (so `e.services` stayed nil after the reassignment) nor recreating
  `c.RTL` (so `rtl.Environment` kept a stale reference to the *pre-VMINIT*
  `vm.Memory`, with no page tables -- reads through it access-violated even
  though the same address read fine through the live `c.Mem`). Both are
  plain missing-reassignment bugs, not ISA/hardware-fidelity questions
  (RTL/console wiring, not emulated VAX behavior -- same category as this
  phase's other loader-side fixes), caught immediately by
  `TestEnsureShims_stubDispatchesThroughXFCShim` failing with "exception
  vector is zero" until both were fixed.
- `internal/console/shim.go`: `shimTable`, a direct transcription of
  `kernel.asm`'s two `.shim` pseudo-op tables (lines ~1146-1177,
  ~1546-1555) as Go data, and `Console.ensureShims`, which synthesizes each
  entry's stub (`MOVL #code,R0` / `XFC #0x7D` / `RET`, matching
  `asm_pseudo.c`'s own `.SHIM` code-generation case byte-for-byte, code 0
  entries included -- see the file's doc comment on why a "dead" stub needs
  no special-casing: `internal/rtl.ShimTable.Lookup(0)` already misses
  naturally) into a dedicated S0 page `VMInit` now reserves (`shimBase`,
  separate from `CONSOLE$SCRATCH`, which stays reserved for RUN's own
  transient `IMAGE$INIT` driver -- conflating the two was an early design
  mistake caught before writing any fixup code, once it was clear real
  shims must outlive any single RUN while the driver procedure doesn't),
  and registers each stub's `SHIM$<library>_<offset>` symbol. Idempotent
  (`shimsReady`), so repeated RUNs keep resolving to the same addresses.
- `internal/console/image.go`'s `Console.imageFixup`, ported from
  `image_fixup`: walks the G^ fixup list (each fixup-vector slot initially
  holds an offset into the target sharable image and is overwritten *in
  place* with the resolved absolute address -- this is what a `G^`
  reference in the compiled code actually indirects through, not a
  separate patch site) and the `.ADDRESS` list (each slot names a
  *different* longword elsewhere in the image whose current offset-into-
  the-dependency value gets rebased by the dependency's load base).
  Resolution order matches the C source exactly: an already-loaded ICB by
  name first, a `SHIM$<name>_<offset>` symbol otherwise -- surfacing a
  clear error if neither exists, exactly as `image_fixup` does (a real gap
  in `shimTable` should be visible, not silently skipped).
- `TestImageFixup_everyRealFixtureFixesUp` runs `imageLoad`+`imageFixup`
  against every real `testdata/exe/` fixture: all seven resolve every G^
  fixup against `shimTable` cleanly, with no unresolved-symbol gaps --
  `shimTable`'s 42 entries (transcribed from the same `kernel.asm` these
  fixtures were historically run against) turned out to be sufficient.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test
  ./...` all clean.
