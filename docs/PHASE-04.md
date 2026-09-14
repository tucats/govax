# Phase 04: Core instruction families

## Goal

Port the highest-traffic instruction groups: data movement, integer arithmetic/compare,
and branch/loop control flow — enough to run simple straight-line and looping VAX
programs (e.g. `testdata/asm/hello.asm`, `atoi.asm`).

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/emul_mov.c`, `emul_mova.c`, `emul_clr.c`,
  `emul_push.c`, `emul_increment.c` — data movement.
- `reference/eVAX/eVAX/Source/CPU/emul_integer_math.c`, `emul_integer_cvt.c`,
  `emul_cmp.c`, `emul_ash.c` — integer arithmetic, conversion, compare, shift.
- `reference/eVAX/eVAX/Source/CPU/emul_branch.c`, `emul_loop.c` — conditional/
  unconditional branches and loop instructions (SOBGTR, AOBLEQ, etc.).

## Deliverables

- Instruction handlers registered into the Phase 03 dispatch mechanism for this
  group, with condition-code side effects matching the C source exactly (this family
  is where the historical `LONGWORD` 32-vs-64-bit condition-code bugs lived — see
  `reference/eVAX/AUDIT.md` §"CPU" findings — verify Go's fixed-width arithmetic
  reproduces the *fixed* C behavior, not the pre-fix behavior).
- Table-driven unit tests per instruction (operand combinations, condition-code
  outcomes, overflow/carry edge cases).
- First integration smoke test: assemble-and-run is not available yet (Phase 11), but a
  hand-encoded or C-assembler-produced byte sequence for a trivial program can validate
  the decode+execute loop end-to-end.

## Design notes

### Resolved: Register mode + `OP_AD`/`OP_VA` access now faults (docs/DEVIATIONS.md carry-forward)

Phase 03 left open whether decode should reject Register mode (`mode == 5`) for an
operand whose access is `OP_AD`/`OP_VA` (an address-only operand — a register has no
VAX address) — the C source never checks this generically in `decode_operand()`, only
ad hoc in a few handlers (`emul_mova.c`, half of `emul_push.c`) that individually test
`opcode->is_register[0]` and fault `EXC_RESADDR`. Per user direction, this is fixed in
Go, and fixed generally: `decodeOperand`'s register-mode fast path (`internal/cpu/
operand.go`) now faults `ExcReservedAddr` for any `AccessAddress`/`AccessVarField`
operand resolving to Register mode, rather than repeating the check in every consuming
handler. This covers every current and future `OP_AD`/`OP_VA` consumer at once (Phase
04's `MOVAx`/`PUSHAx`/`JMP`/`JSB`, and Phase 06's bitfield `OP_VA` operands later) — a
single fix at the point where the ambiguity is actually introduced, rather than
scattered per-instruction guards. `docs/DEVIATIONS.md`'s open question is resolved.

### Confirmed ISA fidelity findings (cross-checked against the VAX ISA manual)

While reading each `emul_*.c` file against `reference/vax_instr_set.pdf`'s per-
instruction Condition Codes sections, several confirmed (not just suspected)
mismatches turned up, beyond the register-mode question above. Per the bug-fixing
policy, the clear-cut, narrowly-scoped ones are fixed in Go as each sub-phase lands
(logged to `docs/DEVIATIONS.md` as resolved findings); the ones that are either
structural (baked into the generated instruction table) or pervasive enough to risk a
bad fix mid-phase are logged as open findings and replicated as-is. Summary (detail in
`docs/DEVIATIONS.md`):

- **Fixed**: `emul_movb_negated`'s stray second `vax.pslw.v = 0` clobbers MNEGB's
  overflow flag right after setting it — a copy-paste bug (MOVW/MOVL's negated
  handlers don't have this line) unrelated to any ISA judgment call.
- **Fixed**: BIT's condition codes leave C unchanged per the manual (`C <- C`); the C
  source's shared `emul_cmp` handler zeroes it for BIT (and TST, where the manual
  agrees C should be zeroed — no bug there).
- **Fixed**: the `SETCONDITIONBITS(x, 0L)` idiom (sets N/Z by comparing to a literal
  zero) incidentally always computes C as false and never touches V at all, because the
  macro's C/V formulas are only meaningful for a real two-operand compare. Every
  instruction that uses this idiom to get N/Z as a side effect — MOVQ, ROTL, ASHL,
  ASHQ, AOBLEQ, AOBLSS, SOBGTR, SOBGEQ — ends up with C force-cleared and V stuck at
  whatever the previous instruction left it, when the manual specifies `C <- C`
  (unchanged) for all of them and a real overflow formula for V on ASHL/ASHQ/the loop
  instructions (MOVQ/ROTL are correctly `V <- 0`). Fixed per-instruction as each is
  ported, computing V correctly and leaving C alone.
- **Fixed** (Go-safety, not an ISA question): `emul_integer_math`'s DIVx2/3 divides
  unconditionally before checking the divisor for zero, relying on the C source
  crashing/UB the same way real integer division-by-zero does on the host. Go panics on
  integer division by zero, so this needs a guard regardless of any ISA judgment call;
  the guard leaves the destination unmodified and sets V, matching the overflow check
  the C source clearly already intended to run for this case.
- **Deferred, logged**: ADWC/SBWC operate on **word** operands in both
  `instruction_table.h` (mechanically generated into `instructions_table.go`, see Phase
  03's sub-phase 1) and `emul_integer_math.c`'s handler, but the manual's format line
  (`add.rl, sum.ml`) specifies **longword** operands. Fixing this means changing
  generated table data, which is out of scope for a Phase 04 change — replicated as-is
  (word-sized), revisit in Phase 12 or alongside `instruction_table.h` generation.
- **Deferred, logged**: byte/word carry-flag computation (`INCx`/`DECx` and the
  byte/word paths of `ADDx`/`SUBx`/`ADWC`/`SBWC`) reads the source as a *signed*
  `char`/`short` and then tests `data & 0x100`/`0x10000` for carry-out — which only
  matches the manual's "carry from the most significant bit" definition when the source
  operand's sign bit is clear. This is a pervasive pattern (not an isolated bug) across
  most of this phase's integer arithmetic, so it's replicated as-is rather than
  redesigned mid-phase; revisit in Phase 12.

## Sub-phases

Each is one buildable, testable commit, following Phase 01/02/03's pattern. Floating
operand types (MOVF/MOVD/MNEGF/MNEGD, CMPF/TSTF, ACBF) are explicitly out of scope —
deferred to Phase 05, whose float handlers do the real `fpu_load`/`fpu_store` port; the
opcode table slots they occupy stay on `unimplementedHandler` until then.

1. **Control primitives** — HALT (returns `ErrHalted`), NOP. Minimal, but needed before
   any integration smoke test can end a program.
2. **MOV family (integer)** — `emul_mov.c`'s MOVZBL/MOVZBW/MOVZWL, MOVB/MCOMB/MNEGB,
   MOVW/MCOMW/MNEGW, MOVL/MCOML/MNEGL, MOVQ. Includes the MOVB-negated V-clobber fix and
   MOVQ's C/V fix (see Design notes).
3. **MOVA/PUSHA + PUSHL family** — `emul_mova.c` and `emul_push.c` together (same
   `is_register`-check pattern, same file family in the C source). Includes the
   register-mode `OP_AD` fault fix in `decodeOperand` (see Design notes) — the first
   `OP_AD`-consuming instructions land here.
4. **CLR family** — `emul_clr.c`: CLRB/CLRW/CLRL/CLRQ.
5. **INC/DEC family** — `emul_increment.c`: INCB/DECB/INCW/DECW/INCL/DECL.
6. **Integer arithmetic/logical + XOR** — `emul_integer_math.c`'s two handlers:
   ADDx2/3, SUBx2/3, MULx2/3, DIVx2/3, BISx2/3, BICx2/3, ADWC/SBWC, and XORx2/3.
   Includes the divide-by-zero guard and the ADWC/SBWC word-sizing deviation log.
7. **Integer conversion** — `emul_integer_cvt.c`: CVTBW/CVTBL/CVTWB/CVTWL/CVTLB/CVTLW.
8. **Compare/bit-test/test** — `emul_cmp.c`: CMPB/W/L, BITB/W/L, TSTB/W/L (integer
   paths only; CMPF/TSTF deferred to Phase 05), plus the optimized `emul_cmpl`. Includes
   the BIT C-bit fix.
9. **Shift/rotate** — `emul_ash.c`: ROTL, ASHL, ASHQ. Includes the V/C fixes.
10. **Simple/generic branches** — `emul_branch.c`'s `emul_branch_always` (BRB/BRW/JMP/
    other unconditional branches), the 12 `BRANCH_HANDLER`-macro conditional branches
    (BNEQ/BEQL/BGTR/BLEQ/BGEQ/BLSS/BGTRU/BLEQU/BVC/BVS/BGEQU/BCS), and `emul_branch`'s
    RSB/BSBB/BSBW/JSB/BLBS/BLBC group.
11. **ACB and CASE** — `emul_branch.c`'s `emul_acb` (ACBB/ACBW/ACBL; ACBF deferred to
    Phase 05) and `emul_case` (CASEB/CASEW/CASEL) — more involved control flow, kept
    separate from sub-phase 10's simpler branches.
12. **Loop instructions** — `emul_loop.c`: AOBLEQ, AOBLSS, SOBGTR, SOBGEQ. Includes the
    V/C fixes from the `SETCONDITIONBITS(x, 0L)` finding.
13. **Integration smoke test** — a hand-encoded byte sequence (no assembler yet; Phase
    11) exercising decode+execute end-to-end across several of this phase's families
    (e.g., a small counted loop using SOBGTR/AOBLEQ, some MOV/arithmetic, ending in
    HALT), run through `Engine.Run`.
14. **Close-out** — gap review against this doc's Goal/Deliverables, final
    `docs/DEVIATIONS.md` pass, progress log, full-repo `go build`/`go vet`/`go test`
    clean, coverage check.

## Open questions / notes

- ~~Register mode used where `OP_AD`/`OP_VA` access is required~~ — resolved per user
  direction: fixed in Go at decode time. See Design notes above.

## Progress Log

### 2026-09-14 — Sub-phase 1: control primitives (HALT, NOP)

- Added `internal/cpu/control.go`: `emulHalt` (the port of `emul_misc.c`'s
  `emul_halt` — privileged-instruction fault outside kernel mode, `ErrHalted`
  otherwise; the C source's `vax.debug & DBG_USERHALT` console escape hatch has no
  `vax.debug` equivalent yet, Phase 08, so this always enforces the architected
  kernel-mode check) and `emulNop` (no-op), registered into the shared
  `instructionTable` via a package `init()`.
- Added `internal/cpu/control_test.go`: HALT in kernel mode stops the engine, NOP
  advances PC and does nothing. HALT's fault-outside-kernel-mode case calls `emulHalt`
  directly rather than through `Engine.Step`/`HandleFault`, to avoid depending on
  `setModeStack`'s already-logged `MAPEN=1` deviation (a real User→Kernel mode switch
  needs page tables this test has no reason to set up); what matters here is only that
  HALT itself reports `ExcPrivileged` outside kernel mode.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 2: MOV family (integer)

- Added `internal/cpu/condcodes.go`: shared size-aware condition-code helpers
  (`signExtend`, `maskToSize`, `signBit`, `isZero`, `minSigned`, `setNZ`) reused across
  every remaining Phase 04 sub-phase, so this and later handlers don't each reinvent
  sign-extension/truncation at sizes 1/2/4/8.
- Added `internal/cpu/mov.go`: `emulMove` (MOVB/MOVW/MOVL/MOVQ and
  MOVZBL/MOVZBW/MOVZWL — one handler, since a zero-extended value's sign bit at the
  destination size is always 0, reproducing MOVZ's "N <- 0" without a special case),
  `emulMcom` (MCOMB/MCOMW/MCOML), and `emulMneg` (MNEGB/MNEGW/MNEGL), registered for
  their respective opcodes via a package `init()`. Floating MOVF/MOVD/MNEGF/MNEGD are
  Phase 05's; their table slots stay unimplemented.
- Cross-checked every handler's condition codes against `reference/vax_instr_set.pdf`'s
  MOV/MOVZ/MCOM/MNEG entries and found three confirmed fidelity issues, all fixed here
  (see `docs/DEVIATIONS.md` for full detail): `emul_movb_negated`'s stray extra
  `vax.pslw.v = 0` clobbers MNEGB's overflow flag (copy-paste bug, MOVW/MOVL's negated
  handlers don't have it); MNEGx's carry flag uses "source LSS 0" where the manual
  specifies "source NEQ 0" (wrong for a positive source); and `emul_movq`'s
  `SETCONDITIONBITS(data, 0L)` idiom force-clears C and skips V entirely where the
  manual wants C unaffected and V explicitly 0 (the first instance of a pattern that
  recurs in ROTL/ASHL/ASHQ/the loop instructions — logged once as a general finding,
  fixed per-instruction as each lands).
- Implemented this phase's Design-notes item: `decodeOperand`'s register-mode fast path
  (`internal/cpu/operand.go`) now faults `ExcReservedAddr` for `AccessAddress`/
  `AccessVarField` operands resolving to Register mode, per user direction. Added
  `TestDecodeOperandAccessAddressRejectsRegisterMode` (both access kinds) and
  `TestDecodeOperandAccessAddressAllowsMemoryModes` to `internal/cpu/operand_test.go`.
  `docs/DEVIATIONS.md`'s open question from Phase 03 is now a resolved finding.
- Added `internal/cpu/mov_test.go`: table-driven tests across MOVB/W/L and
  MOVZBL/BW/WL (register-to-register, N/Z outcomes, byte/word destination writes
  preserving the register's upper bytes, C left unaffected), MOVQ (register-pair
  source/destination, zero case), MCOMB (complement + condition codes), and MNEGB
  (positive/negative/zero/overflow cases) plus a word/long sanity check for the shared
  `emulMneg`.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 3: MOVA/PUSHA + PUSHL family

- Added `internal/cpu/mova.go`: `emulMova` (MOVAB/MOVAW/MOVAL/MOVAQ — one handler,
  since the address computation doesn't depend on the addressed datum's size, matching
  the C source's single shared `emul_mova`), `emulPusha` (PUSHAB/PUSHAW/PUSHAL/PUSHAQ),
  and `emulPushl`, plus a small shared `push` helper (`SP -= 4; store longword`) that
  Phase 07's CALLS/CALLG/BSB/JSB will also want. Neither MOVAx nor PUSHAx touch
  condition codes (confirmed against `emul_mova.c`/`emul_push.c`, which never write
  `vax.pslw` on either path, and the manual, which doesn't list MOVA as affecting
  N/Z/V/C).
- This is the first sub-phase to actually exercise the register-mode `OP_AD` fault
  fixed in sub-phase 2 — both MOVAx and PUSHAx now get it for free instead of
  reimplementing `emul_mova.c`/`emul_push.c`'s own `is_register[0]` checks.
- Added `internal/cpu/mova_test.go`: MOVAL computing an address through Register
  deferred mode with condition codes confirmed unaffected, MOVAL with a register-mode
  source faulting end-to-end through `Engine.Step`/`HandleFault`, PUSHAL pushing an
  address, and PUSHL pushing a register's value (not its address).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 4: CLR family

- Added `internal/cpu/clr.go`: `emulClr` (CLRB/CLRW/CLRL/CLRQ), one handler for all
  four sizes since clearing has nothing size-specific about it beyond how many bytes
  `Operand.Store` writes. N/Z/V match the manual and the C source exactly (`N<-0,
  Z<-1, V<-0, C<-C`) — no deviation here.
- Added `internal/cpu/clr_test.go`: CLRB/W/L condition codes and C-unaffected, CLRB
  preserving the register's upper bytes, CLRQ clearing a register pair.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 5: INC/DEC family

- Cross-checking `emul_increment.c` against `vax_instr_set.pdf`'s INC/DEC entries (V is
  explicitly "integer overflow occurs if the largest positive/negative integer is
  incremented/decremented") surfaced deeper, confirmed problems than expected — see the
  substantially revised `docs/DEVIATIONS.md` entry (originally logged as "deferred" in
  sub-phase 2's commit, now rewritten in place with the full analysis and superseded
  correctly rather than silently edited): the byte-size V range check uses the wrong
  constants entirely (127/-128 confused with 255/-256), and the longword `data==udata`
  overflow-detection trick is a tautology now that `LONGWORD` is genuinely 32-bit — it
  can never detect anything, confirmed concretely with `INT32_MAX + 1`.
- Given INCx/DECx are specified as exactly equivalent to `ADDx S^#1`/`SUBx S^#1`, and
  Go has native 64-bit arithmetic to compute the manual's formulas *correctly* (the
  same "widen, then check for truncation" technique the C source's longword path was
  reaching for but structurally couldn't achieve), this was upgraded from "defer, risky
  to fix broadly" to "fix now, using shared spec-derived helpers" — used by this
  sub-phase and, next, by ADD/SUB/ADWC/SBWC, so the two families can't disagree on
  condition codes for what the manual defines as the identical operation.
- Added to `internal/cpu/condcodes.go`: `setArithPSL` (the shared "N/Z from result, V/C
  as given" pattern) and `addResult`/`addCarryResult`/`subResult`/`subCarryResult`/
  `mulResult`/`divResult` — size-parameterized, wide-arithmetic (`int64`) condition-code
  helpers implementing the manual's ADD/SUB/MUL/DIV formulas exactly, replacing the C
  source's narrow/wrong-constant/tautological ones. `addCarryResult`/`subCarryResult`
  (for ADWC/SBWC) and `mulResult`/`divResult` (for MUL/DIV) aren't exercised until the
  next sub-phase but land together since they're one coherent set of formulas.
- Added `internal/cpu/increment.go`: `emulInc`/`emulDec`, built on `addResult`/
  `subResult`.
- Added `internal/cpu/increment_test.go`: table-driven byte-size tests for both INCB
  and DECB covering the two confirmed-wrong-in-C cases directly (INCB of 0x7F must
  overflow; INCB of 0xFF must carry; DECB of 0x00 must borrow), a longword test
  reproducing the `INT32_MAX`/`INT32_MIN` boundary the C source's truncation trick
  can't detect, and a word test confirming upper-byte preservation.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 6: integer arithmetic/logical + XOR

- Added `internal/cpu/integermath.go`: `emulAdd`/`emulSub`/`emulMul`/`emulDiv`/
  `emulBis`/`emulBic`/`emulXor`/`emulAdwc`/`emulSbwc`, one handler per operation
  (registered across each opcode's byte/word/long/2-op/3-op variants) built on the
  `addResult`/`subResult`/`mulResult`/`divResult`/`addCarryResult`/`subCarryResult`
  helpers added in sub-phase 5, plus a `setLogicalPSL` (N/Z from result, V<-0, C
  unaffected) for BIS/BIC/XOR. Unlike `emul_integer_math.c`'s single opcode-decomposing
  routine, each opcode gets its own small registered handler — decode already carries
  operand count/size per opcode, so there's nothing left to decompose from the raw
  function byte at runtime.
- Cross-checking the full family against `vax_instr_set.pdf` found two more confirmed
  issues beyond what sub-phase 5 already fixed generically (both written up in
  `docs/DEVIATIONS.md`, folded into that sub-phase's now-substantially-rewritten entry):
  BIS force-clears C where the manual specifies unaffected, and BIC computes a
  meaningless arithmetic-carry value instead of leaving C alone — both fixed via the
  shared `setLogicalPSL`.
- Found and logged one more deferred, table-structural finding while writing this:
  `instruction_table.h`'s `BISB3` entry declares its destination operand as
  longword-scaled (`{1, 1, 4, ...}`) where every sibling `Bxx3` instruction correctly
  uses byte (`{1, 1, 1, ...}`) — a clear transcription error, but (per the same
  reasoning as the ADWC/SBWC sizing finding) out of scope to fix since it's baked into
  the mechanically generated table. `internal/cpu/integermath.go`'s handlers use each
  operand's own declared size generically rather than assuming uniform sizing, so this
  surfaces naturally without needing special-case code to reproduce it.
- Pinned down operand ordering carefully for SUB/DIV/BIC, where (unlike commutative
  ADD/MUL/BIS/XOR) getting operand 0 vs. operand 1 backwards silently produces a
  plausible-looking but wrong answer — confirmed against the manual's format lines for
  both the 2-operand and 3-operand forms of each.
- Added `internal/cpu/integermath_test.go`: ADD's signed-overflow-without-carry and
  carry-without-overflow cases, SUB's operand order (2- and 3-operand forms) plus
  borrow/overflow, MUL's overflow (product doesn't fit a byte) with C always clear,
  DIV's normal/negative-truncation case plus the divide-by-zero and `MinInt/-1` guards
  (both would panic in Go without them), BIS/BIC/XOR's condition codes including the
  C-unaffected fix, the BISB3 destination-scale deviation reproduced end-to-end via a
  register destination, and ADWC/SBWC carry/borrow propagation across a simulated
  multi-word add/subtract.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 7: integer conversion

- Added `internal/cpu/cvt.go`: `emulCvt` (CVTBL/CVTBW/CVTWL/CVTWB/CVTLB/CVTLW), one
  handler for every pair since the conversion logic is fully generic over source/
  destination size. Built on a new `convertResult` helper in `internal/cpu/
  condcodes.go` (sign-extend at source size, truncate to destination size, V from the
  manual's own "truncated bits don't match the destination's sign bit" definition).
- Found and fixed another confirmed fidelity issue while cross-checking against the
  manual: `emul_integer_cvt.c` computes N/Z from the *source* value (sign-extended, but
  not yet truncated) rather than the destination (post-truncation) value the manual
  specifies — these only disagree on overflow, but can disagree outright when
  truncation flips the sign (128 long → byte truncates to -128). Its byte-destination V
  check has the same wrong-constants bug already fixed in sub-phases 5-6. Full
  writeup in `docs/DEVIATIONS.md`.
- Added `internal/cpu/cvt_test.go`: sign-extending conversion with no overflow, the
  N/Z-from-destination fix demonstrated concretely (128 → 0x80, N must be true), the
  byte-range V-check fix, and a plain positive-value conversion.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 8: compare/bit-test/test

- Added `internal/cpu/cmp.go`: `emulCmp` (CMPB/W/L), `emulBit` (BITB/W/L), `emulTst`
  (TSTB/W/L). `emulCmp` reuses `subResult`'s V/C computation (CMP's condition codes are
  exactly "src1 - src2, discarded"), forcing V to 0 as the manual specifies (CMP's own
  V is always 0, unlike SUB's). `emulBit` reuses `setLogicalPSL` from the integer-math
  sub-phase. `CMPL`'s C-source table entry attaches a separate, size-specialized
  `emul_cmpl` purely as a performance optimization (`init_emulators.c`) — not ported as
  a separate handler, same category of speed hack Phase 03 already declined to carry
  over, and `emulCmp` is already generic over size.
- Fixed the BIT C-bit deviation flagged ahead of time in sub-phase 2's design notes
  (`docs/DEVIATIONS.md`'s "BIT's carry flag is force-cleared" entry, now marked
  resolved): `emul_cmp.c`'s shared handler zeroes C for BIT where the manual specifies
  unaffected; TST was already correct.
- Added `internal/cpu/cmp_test.go`: CMP's signed/unsigned-disagreement case (N and C
  can point opposite ways for the same comparison, V always 0 regardless), confirming
  operands are never modified, CMPL routing through the shared generic handler, BIT's
  C-unaffected fix, and TST's zero/nonzero condition codes.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 9: shift/rotate

- Added `internal/cpu/ash.go`: `emulRotl` (ROTL, built on `math/bits.RotateLeft32` --
  its negative-count-rotates-right convention matches ROTL's own, so the C source's
  manual bit-by-bit loop doesn't need porting), `emulAshl`/`emulAshq` (ASHL/ASHQ, using
  Go's native shift semantics directly rather than the C source's explicit
  `count > 32`/`count <= -31` boundary checks — a shift count exceeding the type's
  width is well-defined in Go the same way the manual's own boundary notes describe,
  so no extra handling was needed).
- Fixed this phase's first `SETCONDITIONBITS(x, 0L)`-idiom instance flagged back in
  sub-phase 5's `docs/DEVIATIONS.md` entry: added `shiftOverflow32`/`shiftOverflow64`
  to `internal/cpu/condcodes.go` (the manual's real ASHL/ASHQ overflow formula — shift
  left then back right and check the original value survives), and left C untouched
  entirely (never written) rather than force-cleared, for all three instructions.
- Added `internal/cpu/ash_test.go`: ROTL's left/right rotation (negative count) with
  C confirmed unaffected, ASHL's left-shift overflow (`INT32_MAX` shifted left),
  right-shift never overflowing (arithmetic sign-extension confirmed), zero count
  leaving the value unchanged, and ASHQ's quadword counterparts including its own
  overflow case (`INT64_MAX` shifted left, built from a register pair).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 10: simple/generic branches

- Added `internal/cpu/branch.go`: `emulBranchAlways` (BRB/BRW/JMP), the twelve
  `condBranch`-built conditional branches (BNEQ/BEQL/BGTR/BLEQ/BGEQ/BLSS/BGTRU/BLEQU/
  BVC/BVS/BGEQU/BCS), and `emulRsb`/`emulBsb`/`emulJsb`/`emulBlbs`/`emulBlbc`. Verified
  the exact opcode-to-C-handler mapping against `init_emulators.c` rather than assuming
  it from the instruction table alone (JMP and JSB share a table shape with BRW/BSBB
  respectively but use different handlers in the C source; JSB turned out to be
  identical in behavior to BSB once decode has already resolved the target into
  `Operand.Addr`, so it just calls `emulBsb`).
- Double-checked BLBS/BLBC's C-source comparison (`data == (opcode->function == 0xE8) ?
  1 : 0`), which reads like it could be an operator-precedence bug at first glance —
  worked through the actual precedence (`==` binds tighter than `?:`) and confirmed
  it's correct despite the confusing style, so nothing to fix or log here.
- Noted (comment only, not a `docs/DEVIATIONS.md` entry — this isn't an ISA question)
  that `emul_branch.c`'s RSB case ignores `load_register`'s return value entirely, so a
  faulting pop would silently continue in the C source; Go's explicit error return
  makes that class of mistake impossible to reproduce by accident.
- Added `internal/cpu/branch_test.go`: BRB/JMP target computation, all twelve
  conditional branches' taken/not-taken behavior (table-driven), RSB's pop-and-jump,
  BSBB's push-then-jump (return address verified on the stack), JSB reusing the same
  logic through an address operand, and BLBS/BLBC's low-bit test.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.

### 2026-09-14 — Sub-phase 11: ACB and CASE

- Added `internal/cpu/branchacb.go`: `emulAcb` (ACBB/ACBW/ACBL, one size-parameterized
  builder) and `emulCase` (CASEB/CASEW/CASEL). ACB reuses `addResult` (the manual's
  own overflow note — "the index operand is replaced by the low-order bits of the true
  result" — is exactly what `addResult`'s wraparound already does) and leaves C
  untouched entirely, matching both the manual and the C source (which never assigns
  `vax.pslw.c` in `emul_acb` at all — the one instance in this phase where the C source
  gets an "unaffected" condition code right by simply never touching it).
- Found and fixed a confirmed branch-condition bug while cross-checking ACB against the
  manual: for a non-negative addend, `emul_acb.c` branches only when the updated index
  is strictly less than the limit, but the manual specifies less-than-*or-equal* —
  silently dropping a counting-up loop's final iteration when the index lands exactly
  on the limit. Fixed in Go; full writeup in `docs/DEVIATIONS.md`.
- Worked through CASE carefully since the C source's condition-code and branch/skip
  arithmetic looked, at first read, like it might hide more bugs of the kind found
  elsewhere this phase — cross-checked every formula (N/Z/V/C, the skip-distance
  calculation, the table-index calculation, note 1's "PC already points at displ[0]")
  against the manual line by line and found it's actually all correct. The one open
  question — whether the byte/word `selector`/`base`/`limit` operands should be
  sign-extended to 32 bits before the internal arithmetic (what the C source does) or
  computed at their own declared width (the convention every other instruction in this
  phase follows) — the manual's own text doesn't settle, so it's logged as a genuine
  open question in `docs/DEVIATIONS.md` (not a confirmed finding) and replicated as-is.
- Added `internal/cpu/branchacb_test.go`: ACB's positive- and negative-addend boundary
  cases (index landing exactly on the limit), the loop-exit case, C left unaffected,
  and CASE's in-range/at-the-limit/out-of-range (table-skip) cases with their
  respective condition codes and PC targets.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...` all clean.
