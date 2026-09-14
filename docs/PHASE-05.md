# Phase 05: Floating point

## Goal

Port F/D-floating conversion and arithmetic, mapped onto native Go `float64`.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/fpu.c` — F/D-floating ↔ native `double` conversion.
- `reference/eVAX/eVAX/Source/CPU/emul_float_math.c` — floating arithmetic and
  float↔integer conversion instructions (CVTFB/CVTFW/CVTFL/CVTRFL and D-floating
  counterparts).
- `reference/eVAX/eVAX/Source/CPU/emul_mov.c`'s `emul_movf`/`emul_movd` — MOVF/MNEGF/
  MOVD/MNEGD (not in a separate `emul_movf.c`/`emul_movd.c` despite the function
  names — both live in `emul_mov.c` alongside the integer MOV family already ported in
  Phase 04).
- `reference/eVAX/eVAX/Source/CPU/emul_cmp.c`'s `emul_cmp()` — CMPF/TSTF (its existing
  F_FLOAT case) and CMPD/TSTD (its *missing* D_FLOAT case — see Design notes).
- `reference/eVAX/eVAX/Source/CPU/emul_branch.c`'s `emul_acb()` — ACBF (existing) and
  ACBD (missing entirely — see Design notes).
- `reference/eVAX/eVAX/Headers/fpu.h`.

## Deliverables

- Floating instruction handlers in `internal/cpu`, with explicit attention to the
  overflow-bounds bug class `reference/eVAX/AUDIT.md` documents as fixed (N2: CVTFL/
  CVTFW had wrong integer-overflow bounds — Byte -128..127, Word -32768..32767, Long
  -2147483648..2147483647, per the VAX ISA manual §8.3 "Data Types" — the Go port
  should encode these as named constants, not re-derive them ad hoc). N2 is already
  fixed in the current `reference/eVAX` source (confirmed by reading it and by
  `AUDIT.md`'s own 2026-09-14 resolution entry); the Go port replicates the *fixed*
  bounds, not a live bug.
- Table-driven tests including round-trip conversions and the exact boundary cases
  `AUDIT.md`'s N2 fix verification used (e.g. word bounds rejecting 40000 but accepting
  1000).
- Full F_floating **and** D_floating support for the instruction families this phase's
  scope names (arithmetic, both conversion directions, MOV/MNEG, CMP/TST, ACB) — not
  just F_floating with D_floating left partially stubbed, despite the C reference's own
  D_floating support being substantially incomplete (see Design notes: several
  D-floating opcodes have no working C implementation to port at all).

## Design notes

### The C reference's D-floating support is substantially broken and, for several opcodes, entirely unimplemented

Investigated up front because it changes what "port the C source" even means for a good
third of this phase's D-floating scope. Three independent problems, found by reading
`instruction_table.h`, `init_emulators.c`, `emul_float_math.c`, and `emul_cmp.c`
together, then confirmed empirically (not just by inspection) with a standalone C test
harness built against the real `fpu.c` object code (`clang -DLINUX86 -I eVAX/Headers
probe.c eVAX/Source/CPU/fpu.c`), the same verification technique `AUDIT.md`'s own C3
finding used:

1. **Seven D-floating opcodes have no operand table data, no dispatch entry, or
   both**: `SUBD2` (0x62) has a dispatch entry (`instruction[0x62].routine =
   emul_float_math`) but its `instruction_table.h` row has `OperandCount: 0` with the
   scale/access columns populated (a pure data-transcription slip — every sibling row,
   `MULD2`/`DIVD2`, is correct). `CVTDB`/`CVTDW`/`CVTDL`/`CVTRDL` (0x68-0x6B) and
   `CMPD`/`TSTD` (0x71/0x73) have **both** an all-zero table row **and** no
   `init_emulators.c` dispatch entry at all — they are not merely mis-tabled, they were
   never wired up in the C reference. `emul_cmp.c`'s own `switch(dsize)` (the shared
   handler CMPD/TSTD would need) has no `case 3` (D_FLOAT) either, only F_FLOAT/Byte/
   Word/Long — so even a hypothetical dispatch fix wouldn't have working logic to call.
   Per user direction (asked explicitly, given the severity — this is well past Phase
   04's BISB3/ADWC-SBWC precedent of "one field wrong"), all seven are fixed: the Go
   table gets correct entries mirroring their F-floating siblings
   (`internal/cpu/instructions_table.go`, hand-patched post-generation with a comment
   explaining the deviation from the mechanical `go generate` output — see
   `docs/DEVIATIONS.md`), and their handlers are implemented from the ISA manual and by
   direct analogy to the already-correct F-floating/sibling D-floating code, since
   there is no C behavior to port for these seven.
2. **`emul_float_math.c`'s D_FLOAT arithmetic/conversion path loads its source
   operands with `fpu_load`'s two arguments swapped.** The dsize==3 branch calls
   `fpu_load(d[1], d[0], &float1)` (passing the *second*, higher-address longword as
   `src1` and the first as `src2`); `emul_movd` (MOVD/MNEGD, a different handler in
   `emul_mov.c`) calls `fpu_load(data[0], data[1], &dbl)` — the opposite order. Only
   one of these can be right. Confirmed which, empirically: stored a known D_floating
   value with `fpu_store`, then read it back both ways with the actual harness —
   `fpu_load(data[0], data[1])` (MOVD's order) reproduces the original value exactly;
   `fpu_load(data[1], data[0])` (`emul_float_math`'s order) returns
   `-5.4687269079054549e-19` for a stored `3.14159265358979`. This silently corrupts
   every D-floating source read in `ADDD2/3`, `SUBD3`, `MULD2/3`, `DIVD2/3`, and
   `CVTBD`/`CVTWD`/`CVTLD` — every D-floating opcode the C reference *does* dispatch.
   There is no correct C behavior to replicate here either; the Go port's `fpuLoad`
   uses the one convention confirmed correct by the harness (low-address longword as
   `src1`) uniformly for every D-floating consumer, MOVD included.
3. **`fpu_store`'s underflow flush-to-zero is dead code.** On underflow with
   `PSL<FU>` clear, the C source sets a local `double local = 0.0;` intending to store
   a flushed zero, but every bit-extraction line below it reads through `pd`, a pointer
   aliasing the original `source` parameter, never `local` — so the zero assignment has
   no effect on the output. Confirmed with the harness: storing `1e-100` with
   `PSL<FU>=0` produces `rc=0` (no fault, as expected) but a nonzero raw longword
   (`0xF977405F`, not `0x00000000`), built from the original tiny value's mantissa bits
   with only the exponent field forced to a fixed (non-zero-meaning) value — not the
   flush-to-zero the code is clearly trying to do. Fixed in Go: a genuine flush to raw
   zero, matching the explicit `value == 0.0` short-circuit case already one line above
   it in the same function.

Findings 2 and 3 are "obvious logic error, not an ISA judgment call" (an argument-order
transcription slip between two structurally-identical call sites; a stale-variable bug
where the fix was clearly attempted but doesn't compile-through to the output) — fixed
directly per the bug-fixing policy, no need to defer. Finding 1's *severity*
(non-functional, not just subtly wrong) was surfaced to the user rather than decided
unilaterally, given it cuts into this phase's own named deliverables; **decided: fix**
(see above). Full detail and verification transcripts: `docs/DEVIATIONS.md`.

### CVTRFL/CVTRDL (round-to-nearest float→long) are unimplemented in the C reference, not just present-but-wrong

`emul_float_math.c`'s float→integer `switch( dsize2 )` only has cases for `0`
(Byte), `1` (Word), and `2` (Long) — `dsize2 == 3` (`CVTRFL`/`CVTRDL`, the "round"
variant, `op & 0x03 == 3`) falls to `default: set_fault(EXC_PRIV, 0); return
VAX_FAULT;`, an always-fault privileged-instruction path, not a rounding-vs-truncation
bug. Since `CVTRFL`/`CVTRDL` are correctly tabled and dispatched (this is a gap in the
shared handler's own switch, not a table/dispatch problem like the finding above),
implementing real round-to-nearest for these two — the manual's distinction from
`CVTFL`/`CVTDL`'s truncate-toward-zero is unambiguous — is a clear-cut fix, done
directly rather than deferred.

### Short-literal float operands arrive pre-converted; the register/memory path does not

Phase 03's `decodeOperand` (`internal/cpu/operand.go`) already special-cases
`ShortLiteralFloat`: for addressing modes 0-3 on a float-typed instruction, it stores
`math.Float64bits(shortDouble[optype])` directly as the immediate operand's value — an
**already-decoded IEEE double**, not VAX F/D-floating bits. Every float handler in this
phase must branch on `Operand.Kind` accordingly: `OperandImmediate` reads through
`math.Float64frombits` directly, while `OperandRegister`/`OperandMemory` read through
this phase's `fpuLoad` (the loaded bits are real VAX F_floating, 4 bytes, or D_floating,
8 bytes, needing conversion). A shared `loadFloat`/`storeFloat` pair in `fpu.go` centralizes
this so individual instruction handlers don't each re-derive it.

### `fpuLoad`/`fpuStore` are implemented as clean bit arithmetic, not a port of `fpu.c`'s byte-shuffle loops

`fpu.c`'s own implementation converts through two obfuscated byte-shuffle loops (a
historical relic of porting across big- and little-endian hosts) whose net effect, for
a fixed byte order, is a simple 16-bit half-word swap. This was derived and cross-
checked against the harness's own verified output (round-tripping the same five values
`AUDIT.md`'s C3 verification used, plus zero, negative, sub-one, and boundary cases)
rather than trusted from tracing the C loops by eye — the loops are exactly the kind of
code where a byte-by-byte trace is easy to get subtly wrong, and the project already
has a harness-based verification precedent for this file (`AUDIT.md`'s own C3 write-up
did the same). This is a from-scratch idiomatic reimplementation of a pure data-
conversion algorithm (not an ISA-defined register/instruction behavior), so it is not a
"port the C source" situation the way instruction semantics are — see condcodes.go's
existing precedent (Phase 04) for the same kind of judgment call on low-level bit
manipulation.

Verified layout (VAX F/D-floating, derived empirically, matching the VAX architecture
manual's documented "word-swapped" floating format): treat the operand's low-address
longword as `wordSwap(sign<<31 | biasedExp<<23 | frac23)` where `wordSwap` exchanges a
32-bit value's upper and lower 16 bits, `biasedExp` is VAX excess-128, and `frac23` is
the IEEE mantissa's top 23 bits (20 from the double's high longword, 3 from its low
longword). D_floating's second (high-address) longword is
`wordSwap((lo32 << 3) & 0xFFFFFFFF)`, holding the IEEE mantissa's remaining 29 bits
(the low 3 bits are always zero — a real `double` only has 52 mantissa bits to offer
D_floating's 55-bit field). F_floating store additionally rounds (adds 1 at the first
dropped bit, propagating carry into the exponent, matching `fpu.c`'s explicit rounding
block) since it only keeps 23 of the 52 IEEE mantissa bits.

## Sub-phases

Each is one buildable, testable commit, following Phase 04's pattern.

1. **FPU load/store core** (`internal/cpu/fpu.go`) — `fpuLoad`/`fpuStore` (VAX F/D-
   floating ↔ `float64`), `wordSwap`, named fault-subcode constants
   (`faultFltOvf`/`faultFltUnd`/`trapIntOvf`, matching `fpu.h`), overflow/underflow/
   reserved-operand fault paths (the corrected, non-dead-code underflow flush), and
   `loadFloat`/`storeFloat` operand-level helpers handling the short-literal-vs-register/
   memory distinction. Table-driven round-trip and boundary tests using the values
   verified against the C reference via the standalone harness (see Design notes) —
   this phase's tests are cross-checked against real `fpu.c` output, not just internal
   consistency.
2. **D-floating table/dispatch fix** — patch `internal/cpu/instructions_table.go`'s
   seven broken entries (`SUBD2`, `CVTDB`, `CVTDW`, `CVTDL`, `CVTRDL`, `CMPD`, `TSTD`)
   to mirror their F-floating/sibling D-floating rows, with a comment marking the
   deviation from the mechanically generated output. `docs/DEVIATIONS.md` entry.
   Decode-level test confirming each opcode now parses the right operand count/size.
3. **Floating arithmetic** — `ADDF2/3`, `SUBF2/3`, `MULF2/3`, `DIVF2/3` and their
   D-floating counterparts (`ADDD2/3`, `SUBD2`(newly tabled)/`3`, `MULD2/3`,
   `DIVD2/3`), one generic size-parameterized handler built on `loadFloat`/`storeFloat`
   — naturally fixes the D-floating `fpu_load` argument-order bug for every consumer at
   once, since there's only one (correct) `fpuLoad` in Go. Table-driven tests per
   operation including overflow (fault) and a short-literal source operand case.
4. **Float↔integer conversion** — `CVTFB/W/L/RFL`, `CVTBF/W/L`, `CVTDB/W/L/RDL`,
   `CVTBD/W/L/LD`. Named integer-overflow-bounds constants (byte/word/long, matching
   the already-fixed N2 bounds). Fixes, generically: the shared C handler's "always
   read a 4-byte F_FLOAT source regardless of dsize" bug (would have broken any
   D-floating conversion had one ever been dispatched) and CVTRFL/CVTRDL's real
   round-to-nearest (vs. CVTFL/CVTDL's truncate-toward-zero).
5. **MOVF/MOVD, MNEGF/MNEGD** — `emul_mov.c`'s float paths. N/Z from the result, V
   always 0 (matches the C source and the manual — negating a valid F/D-floating value
   can't overflow the format the way integer MNEG can).
6. **CMPF/TSTF, CMPD/TSTD** — extends compare/test to floating operands; CMPD/TSTD
   implemented fresh (see Design notes finding 1).
7. **ACBF, ACBD** — extends Phase 04's `emulAcb` machinery to a floating index/addend/
   limit variant; ACBD implemented fresh (see Design notes finding 1). V is always 0 on
   the surviving path (fpuStore already faults on genuine overflow before an ACBF/ACBD
   V-write would ever matter — confirmed not a live bug despite the C source's own
   "Need overflow detection here" comment on `emul_acb.c`'s ACBF case).
8. **Integration smoke test** — a hand-encoded byte sequence exercising several of this
   phase's families together (float load via short literal or MOV, arithmetic,
   conversion, compare, ACB-driven loop) through `Engine.Run`.
9. **Close-out** — gap review against this doc's Goal/Deliverables, final
   `docs/DEVIATIONS.md` pass, progress log, full-repo `go build`/`go vet`/`go test`
   clean, coverage check.

## Open questions / notes

- None yet.

## Progress Log

### 2026-09-14 — Sub-phase 1: FPU load/store core

- Added `internal/cpu/fpu.go`: `fpuStore`/`fpuLoad` (VAX F_floating/D_floating ↔
  `float64`), `wordSwap`, the named fault-subcode constants
  (`trapIntOvf`/`faultFltOvf`/`faultFltUnd`), and `loadFloat`/`storeFloat` (the
  operand-level helpers handling the short-literal-vs-register/memory distinction
  documented in this phase's design notes).
- Before writing any of this, spent significant time up front investigating the C
  reference's D-floating support and `fpu.c`'s conversion algorithm, using a
  standalone C harness built directly against `reference/eVAX/eVAX/Source/CPU/fpu.c`
  (`clang -DLINUX86 -I eVAX/Headers probe.c fpu.c`) — see this doc's Design notes for
  the full write-up. This surfaced four confirmed bugs beyond the already-fixed N2:
  `emul_float_math.c`'s D-floating arithmetic path calls `fpu_load` with its two
  source longwords swapped (corrupting every dispatched D-floating source read), and
  `fpu_store`'s underflow flush-to-zero is dead code (reads through a pointer
  aliasing the original, un-zeroed parameter) — both fixed directly here, logged in
  `docs/DEVIATIONS.md`. The other two (the D-floating table/dispatch gaps, and
  `CVTRFL`/`CVTRDL` always faulting instead of rounding) are logged when fixed in
  sub-phases 2 and 4 respectively, where they're actually implemented. None of the
  four are replicated — `fpuLoad`/`fpuStore` implement the corrected behavior
  directly, verified against the harness's own output rather than by tracing
  `fpu.c`'s byte-shuffle loops by eye.
- Added `internal/cpu/fpu_test.go`: round-trip and known-value tests using raw F/D
  bit patterns produced by the harness (including `reference/eVAX/AUDIT.md`'s own C3
  verification values), overflow/underflow fault tests (including the corrected
  flush-to-zero behavior), reserved-operand and negative-zero load tests, and
  `loadFloat`/`storeFloat`'s short-literal-vs-register distinction.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean. `go test ./internal/cpu -run TestFpu -coverprofile` on this file: 85.6%
  (the uncovered lines are `fpuStore`'s F_floating rounding-carry-into-overflow edge
  and `loadFloat`/`storeFloat`'s `Operand.Load`/`Store` error-propagation paths,
  already covered generically by Phase 03's `operandaccess_test.go`).

### 2026-09-14 — Sub-phase 2: D-floating table/dispatch fix

- Added `internal/cpu/gen/main.go`'s `knownTableFixes` map and `applyKnownFixes`,
  applied between parsing `instruction_table.h` and generating
  `instructions_table.go`, patching the seven broken D-floating entries identified in
  this doc's design notes (`SUBD2`, `CVTDB`, `CVTDW`, `CVTDL`, `CVTRDL`, `CMPD`,
  `TSTD`) to mirror their already-correct F-floating/sibling D-floating rows. Applied
  at generation time rather than hand-edited into the generated (DO-NOT-EDIT)
  `instructions_table.go`, so the fix survives a future `go generate` instead of
  being silently reverted by it; `applyKnownFixes` fails loudly if an expected name
  goes missing, so a future `instruction_table.h` change that fixes these upstream
  won't go unnoticed either.
- Ran `go generate ./internal/cpu` and confirmed via `git diff` that exactly these
  seven entries changed, nothing else.
- Logged the full finding (severity, confirmation method, fix) in
  `docs/DEVIATIONS.md`.
- Added `internal/cpu/instruction_test.go`'s `TestInstructionTableDFloatingFixedEntries`,
  pinning down each of the seven entries' operand count/scale/access/type directly at
  the table level, independent of any handler (none exist yet for the five
  previously-undispatched opcodes — that's sub-phases 3/4/6).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all
  clean.
