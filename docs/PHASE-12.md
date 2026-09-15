# Phase 12: Integration & regression

## Goal

Prove the full stack works end-to-end against every fixture, establish a durable
regression suite, and do a final polish pass before considering the port complete.

## Scope

- Assemble (via Phase 11) and run every `testdata/asm/*.asm` fixture; run every real
  `testdata/exe/*.exe` binary (except `put1.exe`, a zero-byte file not usable per
  `reference/eVAX/AUDIT.md`); exercise SAVE/LOAD against `testdata/rom/xdefault.rom`.
- Build a fixture-driven regression test suite (likely in `internal/console` or a
  top-level `integration_test.go`) that runs all of the above in CI-friendly form, so
  future changes can't silently regress behavior the C source got right.
- Cross-check final behavior against `reference/eVAX/AUDIT.md`'s own verification notes
  for each fixed finding (N1, N2, V1, V7, V8, R1, etc.) — those describe the exact
  expected behavior for several of these fixtures and are a ready-made acceptance
  checklist.
- Performance pass: profile the fetch-decode-execute loop under `bench.asm` (a fixture
  apparently intended for this purpose) and address any obvious hotspots.
- Documentation polish: update `README.md` and `docs/PLAN.md` to reflect the finished
  state; consider whether `reference/eVAX/` should be trimmed or kept as permanent
  historical reference.

## Deliverables

- A green, fixture-driven regression suite covering CPU, console, I/O, RTL, and
  assembler behavior together.
- Every entry in `docs/DEVIATIONS.md` resolved: either fixed in the Go port, or
  deliberately kept with rationale recorded there.

## Open questions / notes

- This phase's real scope depends heavily on what Phases 01-11 turn out to need —
  treat the above as a checklist to refine once those are underway, not a fixed plan.

## Progress Log

### 2026-09-15 — Sub-phase 1: ASM/CALL wired to live memory

- `internal/console/asm.go`'s `Console.Assemble` implements the batch
  `ASM <filename>` command (`console_include.c`'s `console_asm`, called
  through `console_include` with an implicit `/ASM`): reads the file,
  assembles it with Phase 11's `internal/asm`, deposits the resulting P0
  and S0 bytes into live memory, and merges the program's own symbols into
  `Console.Symbols` so they're immediately usable by name (EXAMINE/DEPOSIT/
  CALL/DISASSEMBLE). Per Phase 11's own progress-log note that wiring this
  was left as follow-up work.
- Discovered mid-implementation that a naive "fresh `asm.Assembler` per ASM
  call" design doesn't match the reference tool: the real console's
  `vax.assembler` is one persistent, session-wide object, so a later ASM
  command can reference an earlier one's labels — concretely,
  `testdata/asm/hello.asm`'s `@#lib$put_output` only resolves because
  `testdata/asm/kernel.asm` (assembled first, exactly as `vax.init`'s own
  boot sequence does) defines `lib$put_output` as a real `.ENTRY` procedure.
  `Console.asmSession` (machine.go) is now a persistent `*asm.Assembler`,
  reused across ASM calls and reset (nil, lazily recreated) only by VMINIT.
  This surfaced two real `internal/asm` bugs that only show up when one
  Assembler instance assembles more than one file in sequence (invisible to
  Phase 11's own one-file-per-test fixture suite):
  - `Assemble` didn't reset the `.END`-set `stop` flag, so a second
    `Assemble` call on the same instance silently no-op'd its first line
    onward. Fixed by clearing `stop` at the top of `Assemble` (not
    `assembleLines`, which `.INCLUDE` also calls and must keep the
    C source's "an `.END` inside an included file ends the includer too"
    behavior).
  - `.END`'s "an entry was named" flag (`entrySeen`/`entryAddr`) was a
    plain, non-consuming getter (`Entry`); reused across files, an earlier
    file's `.END name` would spuriously reappear as the *next* file's own
    entry even when that file's own `.END` was bare. The C source avoids
    this because `console.c`'s post-command hook clears `ASM_ENTRY` the
    instant it fires the one-shot auto `CALL __ENTRY` this flag triggers.
    Added `TakeEntry` (read-and-clear) alongside the existing non-destructive
    `Entry`, and `cmdAssemble` (dispatch.go) uses it to replicate that same
    one-shot auto-call behavior.
  - Also found, the same way: the assembler's literal default S0 origin
    (0x80000000) is only valid for a standalone assembly with no live VM
    backing it (Phase 11's own fixture tests) — depositing a live session's
    S0 content there corrupts VMINIT's own S0 page table, which is mapped
    starting at that exact virtual address (an identity map: S0 virtual
    page 0 is backed by the physical page the table itself occupies).
    Added `SetS0Origin`/a real `s0Origin` field (previously `S0Origin()`
    just returned the constant), and `VMInit` now records the first free S0
    address past every region it reserves (`Console.s0Free`) for
    `Console.Assemble` to relocate a freshly created session's S0 content
    to. None of this is an ISA/hardware-fidelity question (DEVIATIONS.md
    territory) — it's Go-port wiring that simply never had a live-memory
    consumer before this phase.
- `internal/cpu/call.go`'s `Engine.CallEntry` gained a variadic `args
  ...uint32` parameter (pushed right-to-left ahead of the argument count,
  exactly like a real CALLS instruction), and `internal/console/call.go`'s
  `Console.Call` passes them through — Phase 13 had deliberately left this
  unimplemented since RUN's own call site never needed it; this phase's
  `bench.asm`/`hello.asm`-style fixtures (`CALL main(...)`,
  `TIME CALL main(^d10000)`) are exactly console_exec.c's own named use
  case for it.
- `dispatch.go`'s `CALL` fixed command (previously `cmdNotImplemented`) now
  reaches `cmdCall`: `CALL [/STEP] <entry-expr>[(arg1[,arg2...])]`, matching
  `console_call`'s own parsing (an optional single leading qualifier, then
  an address expression, then an optional comma-separated argument list).
  `ASM`/`ASSEMBLE` reach `cmdAssemble`.
- Tests: `internal/asm/symbol_test.go` (`TestSymbols`/`TestTakeEntry`/
  `TestSetS0Origin`); `internal/cpu/call_test.go`
  (`TestEngineCallEntryWithArguments`); `internal/console/asm_test.go`
  (assembling `xor.asm` and calling it by its merged symbol name; the
  persistent-session symbol-sharing/entry-isolation behavior across two
  files; `kernel.asm` then `hello.asm` in one session, bounded); new cases
  in `internal/console/dispatch_test.go` for `ASM`+`CALL` end-to-end,
  `CALL`'s argument-list syntax, and `CALL/STEP`.
- Noted for this phase's later debugging sub-phase, not chased further
  here: `kernel.asm` then `hello.asm` (`TestAssemble_kernelThenHelloRunsBounded`)
  reaches a bounded, clearly reported fault (`vm: no physical storage at
  0xfffffffc`) partway through `hello.asm`'s `LIB$PUT_OUTPUT`/`CHMK`
  dispatch chain, rather than completing cleanly — likely related to the
  same RTL/CHMK dispatch surface `put.exe`/`cli.exe`/`sieve.exe` didn't
  fully clear in Phase 13.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test
  ./...` all clean.
