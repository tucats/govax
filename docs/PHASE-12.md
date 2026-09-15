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

### 2026-09-15 — Sub-phase 2: chasing kernel.asm+hello.asm to (nearly) completion

Sub-phase 1's own noted follow-up (`kernel.asm` then `hello.asm` faulting with a
garbage-looking `vm: no physical storage at 0xfffffffc`) turned out to be five
separate, real bugs stacked on top of each other — this microkernel had never
actually been assembled into a live address space and run before, so nothing in
Phases 01-11 could have caught any of them. Chased one at a time, each fix exposing
the next:

1. **`Console.Assemble`'s S0 origin collided with VMINIT's own S0 page table.**
   `internal/asm`'s default S0 origin (0x80000000) is only meaningful for a
   standalone assembly with no live VM backing it; a real VMINIT'd console's S0
   virtual page 0 is identity-mapped to the *physical* page the S0 page table itself
   occupies, so depositing `kernel.asm`'s own code starting there corrupted the
   table it was mapped through. Fixed with `asm.Assembler.SetS0Origin`/`Origin`
   (new) and `Console.s0Free` (`vminit.go`, the first free S0 address past every
   VMINIT-reserved region), used when `Console.Assemble` lazily creates its
   persistent session.
2. **VMINIT never actually reserved a page for the SCB.** Unlike
   `CONSOLE$SCRATCH`/the shim page (each explicitly `paddr += 512`), `SCBB` was set
   to `paddr` with nothing advancing past it — harmless before this phase (nothing
   yet computed "the next free S0 address" from it), but once fix 1 introduced
   exactly that, kernel.asm's own code (deposited starting there) silently
   overwrote its own `.SCB` vector table a few bytes in. Fixed by reserving a
   dedicated page for the SCB, matching `CONSOLE$SCRATCH`/the shim page's own
   treatment (and `console_vminit_dcl`'s real, dedicated-page SCB layout).
3. **`.SCB`'s poke never reached live memory at all.** `pseudoSCB` writes a
   longword directly at `0x80000000+SCBB+code` — deliberately *below* where
   `Console.Assemble`'s normal `[S0Origin, S0End)` copy range starts (kernel.asm's
   own code goes *after* the SCB page, not inside it) — so it stayed in the
   assembler's own private image buffer forever, never reaching `c.Mem`. This is
   the exact class of bug the user flagged mid-session ("the assembler's workspace
   memory and the real memory" possibly diverging) — checked for other instances
   (`.VECTOR`/`.PSL`/`.MODE`/`.PTE`/`.CONSOLE` are recognized-but-unimplemented
   no-ops per `pseudo.go`'s own doc comment, and none are used by any current
   fixture including kernel.asm; `.BASE`/`.ALIGN`/`.REGION` all move the ordinary,
   tracked deposit counter rather than poking an untracked address) — `.SCB` was
   the only real instance. Fixed by also depositing the SCB page's current content
   on every `Assemble` call (idempotent).
4. **CHMK/CHME/CHMS/CHMU had no execute-time handler at all.** The generated
   instruction table has always had correct rows for all four opcodes (0xBC-0xBF,
   matching `instruction_table.h` exactly — a red herring early on, since a
   sloppy grep match briefly suggested a table transcription bug that further
   inspection ruled out), but no phase ever registered a `Handler` for them, so
   every real CHMK — i.e. every SYS$/LIB$ call this whole project's RTL layer is
   built on — silently fell through to `unimplementedHandler` (a reserved-
   instruction fault) the instant real code exercised one; nothing before this
   phase ever ran a real CHMK. `internal/cpu/changemode.go`'s new `emulChmx` ports
   `emul_chmx` (raise the corresponding synchronous exception with the operand's
   sign-extended code as its one signal argument); the manual's own "refuse on the
   interrupt stack" and 12-byte new-stack probe aren't ported (no current fixture
   exercises either).
5. **`internal/cpu/operand.go`'s Phase 04 Register-mode reject for `AccessAddress`/
   `AccessVarField` operands broke CALLG's own legitimate use of Register mode**
   (kernel.asm's CHMK dispatcher does `callg ap, (r0)`, a real tail-call idiom) —
   see `docs/DEVIATIONS.md`'s "Register mode used where OP_AD/OP_VA access is
   required" entry for the full story and revert. Found only because fix 4 let
   execution reach this instruction for the first time.
6. **`internal/asm`'s own indexed-addressing-mode encoder corrupted the operand
   after an indexed one.** `assembleOperandRec`'s index-prefix lookahead ("BASE[Rx]")
   writes the index byte, then recurses to parse BASE alone — but the `(Rn)`
   register-deferred branch (and, structurally, several sibling branches) returned
   as soon as the base itself was consumed, without skipping the trailing "[Rx]"
   text still sitting in the cursor. Invisible for a single-operand instruction
   (`TestAddressingModes`' own "indexed" case only ever exercised this on `CLRL`,
   which has nothing after to corrupt); kernel.asm's real, working
   `EXE$DISPATCH` (`movl (r3)[r2], r0`) is a genuine two-operand instance — the
   leftover `[r2]` got reinterpreted as the start of the destination operand,
   producing a garbled multi-byte encoding that decoded as a nonsense instruction
   at runtime (an early, confusing symptom: a wild PC jump into never-assembled
   memory, chased at length before the real cause was found). Fixed by consuming
   the leftover `[Rx]` once, at the single call site that recurses with
   `parsingIndex=true`, rather than teaching each base-mode branch individually.
   `internal/asm/operand_test.go`'s new `TestIndexedModeFollowedByAnotherOperand`
   regresses this directly.

With all six fixed, `kernel.asm` then `hello.asm` (in one ASM session, matching
`vax.init`'s own boot sequence) now runs correctly through `main` →
`LIB$PUT_OUTPUT` → `LIB$PUT_ONE` → a real CHMK 0 (`EXE$PUT_CONSOLE`) dispatch,
successfully writing the first byte of "Hello world" — as far as this port's
current I/O modeling goes. It doesn't complete: `EXE$$PUT_CONSOLE`'s own design
clears a memory flag (`exe$tx_ready`), does `MTPR` to TXDB, then spin-waits on
that same flag, expecting the `EXC$CONWRITE` interrupt kernel.asm's own ISR
(`exe$tx`) handles to set it back to 1. `internal/cpu/procreg.go`'s `setPrivReg`
TXCS/TXDB cases are a plain register store with no device or interrupt-delivery
modeling (its own doc comment already flagged this as deferred to Phase 09, which
evidently never actually implemented it) — so `exe$tx_ready` is only ever cleared,
never reset, and the byte after the first spins forever. This is exactly the
mechanism the user asked about mid-session, suspecting "the byte-at-a-time VAX
hardware console... involving process-privilege registers and a count down timer"
— confirmed correct in spirit (it is a console-hardware simulation gap causing
what looks like an infinite loop), though the actual missing piece is interrupt
*delivery* (`vax.interrupt_pending`/`vax.iqueue` admission, checked between
instructions in `vax.c`'s main loop — a real, moderately-sized feature with no Go
port yet at all) rather than a timer specifically; the reference's own TXCS/TXDB
handling has no delay of any kind ("the RDY bit is always set; we never have to
wait for a console transmit to complete"). Tracked as a genuine, well-understood
Phase 09 gap (device/interrupt modeling) rather than chased further as part of
this phase — `internal/console/asm_test.go`'s `TestAssemble_kernelThenHelloRunsBounded`
documents this explicitly and asserts the current (bounded, non-panicking) outcome
rather than a false "it works" claim.

- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean.

### 2026-09-15 — Sub-phase 3: fixture-driven regression suite (asm side)

`internal/console/regression_test.go` is this phase's own named deliverable: every
`testdata/asm/*.asm` fixture not already covered elsewhere (`xor.asm` in
`asm_test.go`; `hello.asm`, `kernel.asm` itself, and `forth.asm`'s own assembly-only
check in `internal/asm/fixtures_test.go`) is now assembled and run through the real
ASM/CALL pipeline sub-phases 1-2 built, alongside the existing exe-fixture milestone
check (`run_test.go`, Phase 13) and ROM/NVRAM save/load round trip (`rom_test.go`,
Phase 08) that already run under the same `go test ./...`.

- `movq.asm`/`movc3.asm`: hand-verified exact results (R0, the quadword round-tripped
  through `data2`, the moved string bytes) — the two fixtures whose expected outcome
  is simple enough to state exactly. `movq.asm`'s own `.ENTRY main, ^m<r4,r5,r6,r7>`
  mask restores R4-R7 on RET, so those two registers read back as their pre-call
  value of 0 after `Call` returns; checking them post-return would only re-test
  CALLS/RET's own save/restore (already covered directly in `internal/cpu`), so
  only R0 and the `data2` memory side effect are asserted.
- `ff.asm`/`insv.asm`/`dbl.asm` (FFS/INSV/EXTZV bit-field and D-floating math) and
  `float1.asm` (F-floating divide, run PC-first via `Console.Execute` rather than
  `CALL` — it's a plain label ending in `HALT`, not a real `.ENTRY` procedure ending
  in `RET`): held to a bounded-completion bar rather than hand-derived exact bit
  patterns, since each underlying instruction already has dedicated, hand-verified
  unit coverage (`internal/cpu`'s own `bitfield_test.go`/`cvt_test.go`/`fpu_test.go`)
  — this suite's own value is catching a regression in the assemble-deposit-call
  pipeline driving them, not re-deriving expected values by hand a second time here.
- `foo.asm`/`atoi.asm`/`fmt.asm`/`input.asm`/`logname.asm`/`test.asm`: assembled
  against a real `kernel.asm` in the same session first (matching `vax.init`'s own
  boot sequence), then bounded-run. Confirmed empirically (not assumed) which
  currently complete/fault cleanly (`atoi.asm`, `fmt.asm`, `logname.asm`) versus
  which hang at the step cap (`foo.asm`, `input.asm`, `test.asm` — all three call
  either `LIB$PUT_OUTPUT` or `LIB$GET_INPUT`, both of which poll a ready/available
  flag only an `EXC$CONWRITE`/`EXC$CONREAD` interrupt's own kernel.asm ISR can reset
  — the same TXCS/TXDB interrupt-delivery gap sub-phase 2 found and documented via
  `hello.asm`). The test records this split explicitly (a `wantHitCap` table) rather
  than either failing the suite for a known, already-documented gap or silently
  accepting any outcome, which would mask a real future regression.
- Not attempted here: `bench.asm` (no `.END` entry and no single canonical call the
  way the others have — its own header comment says to invoke it interactively,
  `TIME CALL main(^d10000)`, and it's this phase's own named fixture for the later
  performance-profiling sub-phase instead) and `forth.asm` (a large, interactive
  FORTH interpreter with no single well-defined entry point to drive here, and its
  own separate `.MICROKERNEL` configuration — already covered for assembly
  correctness by `internal/asm/fixtures_test.go`'s `TestAssembleForth`).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean.

### 2026-09-15 — Sub-phase 4: cross-check against reference/eVAX/AUDIT.md

Went through this phase's own named acceptance checklist (N1, N2, V1, V7, V8, R1) —
the C project's own closed 32-vs-64-bit `LONGWORD` audit, plus its live-testing
follow-ups — to confirm each is genuinely addressed in the Go port, not just
inherited by assumption:

- **V1** (ROM/NVRAM binary formats broken against `xdefault.rom`): already fixed and
  directly cross-referenced — `internal/console/rom.go`'s own doc comments cite V1's
  4-byte field pinning by name; `TestLoadROM_realFixture`/`TestSaveROM_round
  TripsRealFixtureContent` exercise the real fixture (Phase 08).
- **N2** (`CVTFL`/`CVTRFL`/`CVTDL` reusing the byte-case's `[-128,127]` overflow
  bounds instead of a real longword-range check): already fixed with the correct,
  size-specific bounds — `internal/cpu/fpu.go`'s `longMin`/`longMax` constants cite
  N2 by name directly in their own doc comment (Phase 05).
- **N1** (native host pointers smuggled through `LONGWORD`/symbol `value` fields —
  the assembler's own symbol table, `console_dispatch`'s string-argument passing):
  doesn't apply — this bug class only exists because C's `LONGWORD` was standing in
  for both a 4-byte VAX value and, in these specific spots, a native host pointer.
  This port's `asm.symbol.value`/`console.Symbol.Value` are plain `uint32`s with no
  such dual use anywhere; a host-side string (e.g. a future `CONSOLE$ARG_CMD`) would
  live in an ordinary Go `string` field, impossible to conflate with a VAX-facing
  numeric value by construction. Already reasoned through once, independently, in
  `docs/PHASE-10.md`'s own open-questions section.
- **R1** (`FAB`/`RAB` pointer-typed fields declared as native 8-byte pointers,
  populated with only 4 VAX-side bytes, leaving garbage high bits on every RMS file
  operation) — and, by the same root cause, **R2-R5** (`SYS$CLI`'s buffer index,
  every system-service `argv` slot, `str_get`/`str_put`'s descriptor-data pointer,
  `SYS$GETDVIW`/`SYS$GETJPIW`'s item-list addresses): none apply, for the same
  structural reason as N1. `internal/rtl`'s service handlers uniformly take
  `argv []uint32` (checked across every `service*.go`/`cli.go`/`devices.go`
  file — no exceptions found), and `rms.go`'s FAB/RAB field access reads a real VAX
  address via `vm.Memory.LoadLongword` and resolves it through `loadString`/further
  memory accesses on demand (`serviceSysCreate`'s `fabFNA` read, e.g.) — RMS's own
  "internal file identifier" is a `uint16` handle into `env.ifiFiles`, not a native
  pointer at all. There is no 8-byte-native/4-byte-VAX size mismatch anywhere in this
  design for a "garbage high bits" bug to occur in.
- **V7** (`union VALUE`'s bitfield/longword union-size pattern, the same family as
  the already-fixed `MASKREG`/`PTE` unions): doesn't apply — `internal/asm`'s
  `.BYTE`/`.WORD`/`.LONG` pseudo-ops (`pseudo.go`'s `storeScaled`) call explicitly
  byte/word/longword-sized `image.storeByte`/`storeWord`/`storeLongword`, the same
  pattern V9 (the audit's own "good existing practice" counter-example) recommends;
  Go has no `union` construct for this size-mismatch class of bug to hide in at all.
- **V8** (VMS image-header structs vs. loading the repo's own `.exe` fixtures) —
  the audit's own explicitly **unresolved** finding, left as "read `console_run.c`'s
  image-loading path directly... before triaging this further": confirmed, by
  direct inspection of `console_run.c` (every `IHD`/`IHI`/`ISD`/`IAF` field is read
  via an individual `load_memory` call at a computed offset — never a raw struct
  cast over the file buffer), that its better-case branch applies, so V8 was never a
  live bug to begin with. This is now recorded directly in
  `internal/console/image.go`'s own doc comment, alongside the "field-by-field, not
  a generic struct-mapping port" design note Phase 13 already had reasoning parallel
  to it.

No action items came out of this pass beyond the one doc-comment addition (V8) —
every other finding was already fixed-and-cross-referenced (V1, N2) or structurally
inapplicable to a Go port with no `LONGWORD`/native-pointer/union conflation to
reproduce in the first place (N1, R1-R5, V7).

### 2026-09-15 — Sub-phase 5: resolving `docs/DEVIATIONS.md`'s remaining open items

Per this doc's own policy ("Entries get resolved... during Phase 12"), went through
every entry still marked deferred/not-fixed/not-resolved and made a final call on
each — fixed six with clear, manual-backed answers and no real judgment call
involved; confirmed three as genuinely, deliberately kept (re-checked against new
evidence rather than left stale):

- **CMPC5's fill-padding loops running after an inequality was found**: settled
  directly against `vax_instr_set.pdf`'s own CMPC entry ("comparison proceeds until
  inequality is detected **or** all bytes... examined[;] condition codes affected by
  the **last** byte comparison") — an inequality ends the operation, so the two
  fill loops must not run afterward. Fixed in `internal/cpu/cmpc.go`'s `emulCmpc5`.
- **ADAWI's condition codes** (C always false; N/Z from the untruncated 32-bit sum):
  fixed using the same wide-arithmetic `addResult` helper ADD/ADWC already use.
- **EDIV's missing quotient-overflow detection**: fixed, checking the true quotient
  against `longMin`/`longMax` and applying the manual's Note 3 fallback, the same as
  the already-handled zero-divisor case.
- **ADWC/SBWC's word-sized operands** (the manual specifies longword) and **BISB3's
  longword-scaled destination** (every sibling `Bxx3` form is byte-scaled): both
  fixed via `internal/cpu/gen/main.go`'s `knownTableFixes` mechanism (the same one
  already used for the D-floating/REMQUE/REMQHI/REMQTI table fixes) and a `go
  generate` re-run — no handler code changes needed, since both handler families
  already derive operand width generically from the table.
- **`vm.TranslationFault` collapsing `EXC_ACCVIO`'s length-vs-protection subcode**:
  fixed by splitting `vm.AccessViolation` into `AccessViolation`/`ProtectionViolation`
  `FaultKind`s and threading the right subcode (0x0001/0x0002) through
  `wrapMemError`.
- **`set_mode_stack`'s unconditional `MAPEN=1`** and **CASE's internal arithmetic
  width**: both re-examined (VM-disabled fault handling still isn't exercised
  anywhere in this port; `vax_instr_set.pdf`'s own CASE Note 2 is confirmed, by
  direct reading, to be genuinely silent on signed-vs-widened arithmetic, not just
  unchecked) and left deliberately kept, since nothing new resolves either.
- **PC-relative Immediate mode not rejected for an `AccessAddress` operand**:
  deliberately left open rather than fixed, and for a concrete reason found this
  same phase — its own cited precedent (the Phase 04 Register-mode reject) was
  itself reverted this phase once `kernel.asm` running for real showed it broke
  CALLG's legitimate use of Register mode. Adding a *new* blanket decode-time reject
  for an addressing mode no current fixture exercises either way risks the exact
  same mistake, so this stays open until a real program actually needs one behavior
  or the other.
- **Character-string length operands' signed/unsigned reading**: unchanged — no new
  fixture in this phase's regression suite exercises a length anywhere near the
  32KB+ needed to expose the difference, so the existing "low priority" disposition
  still holds.

Every fix has a dedicated regression test (`TestEmulCmpc5/inequality_found_...`,
`TestEmulAdawiOverflowSetsV`/`TestEmulAdawiSetsRealCarry`,
`TestEmulEdivQuotientOverflow`, `TestEmulAdwcOperatesOnLongwords`,
`TestEmulBisb3DestinationScaleFixed`, `TestTranslateProtectionViolation`/
`TestWrapMemErrorProtectionViolation`) and an updated `docs/DEVIATIONS.md` entry
(moved to "Resolved findings" where it wasn't already there, with a `[..., resolved
Phase 12]` tag).

- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean.

### 2026-09-15 — Sub-phase 6: performance pass

`internal/console/bench_test.go`'s new `BenchmarkSieve` is this phase's own named
performance-pass target (`bench.asm`): it calls the fixture's `SIEVE` routine
directly (bypassing `MAIN`'s own `LIB$PUT_OUTPUT`-style printing, which would run
into the console-I/O interrupt gap `docs/PHASE-14.md` now tracks) with a fixed
argument (10000), so it profiles a pure, self-contained, CPU-bound VAX computation
with a checkable result (R0 = 9973, the largest prime below 10000).

Profiling (`-cpuprofile`/`-memprofile`) found one clear, worth-fixing hotspot:
`Engine.Step` allocated a fresh `Decoded` (a `[6]Operand`-sized struct) on the heap
*every single instruction* — `decodeInstruction` returns one by value, but taking
its address and passing it through `handler` (an indirectly-called function value)
defeats Go's escape analysis, forcing heap promotion regardless of the value's
actual lifetime. Confirmed by an allocation profile: 180,891 allocations/op (~1 per
VAX instruction executed), 57.8 MB/op, both attributed directly to `Step`'s own
frame. Fixed by giving `Engine` a reusable `decoded Decoded` field, decoding into a
local and copying it there (a plain, non-escaping struct copy — the field's memory
already exists as part of the already-heap-resident `*Engine`) instead of taking the
address of a fresh local each time.

Before/after, `go test ./internal/console -bench BenchmarkSieve -benchmem`
(`benchtime=200x`, Apple M5 Max):

| | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| before | 17,539,626 | — (not measured) | — |
| after | 11,004,645 | 24 | 3 |

Wall-clock time dropped ~37% (17.5ms → 11ms per `sieve(10000)` call) and allocations
dropped from 180,891/op to 3/op (the 3 remaining are `testing.B`'s own loop
bookkeeping, not this emulator's). A second post-fix profile confirms the
improvement is real, not measurement noise: hot samples are now dominated by
`vm.Memory.Translate`/`LoadByte`/`decodeOperand`/`decodeInstruction` — exactly the
work you'd expect an instruction-level emulator's fetch-decode path to spend time
on, with `runtime.duffcopy` (plain struct-copy work, not allocation) the next-largest
single contributor and no other single frame standing out disproportionately. Not
chased further: this is normal, proportionate cost for the work being done, not a
second "obvious hotspot" the way the allocation was.

`Engine` is single-threaded by design (one machine, stepped sequentially — nothing
in this codebase runs `Step` concurrently on the same `Engine`), so the new shared
`decoded` field needed no synchronization; confirmed with `go test -race ./...`
(clean) alongside the full suite.

- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` and
  `go test -race ./...` both clean.
