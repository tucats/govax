# Phase 07: Procedure calls, privileged & misc instructions

## Goal

Close out the CPU instruction set: procedure-call stack frame management, privileged
instructions, and everything else that doesn't fit the earlier families.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/emul_call.c` — CALLS/CALLG stack frame construction
  and RET.
- `reference/eVAX/eVAX/Source/CPU/emul_procreg.c` — processor register instructions
  (MTPR/MFPR).
- `reference/eVAX/eVAX/Source/CPU/emul_interlock.c` — interlocked (atomic)
  instructions.
- `reference/eVAX/eVAX/Source/CPU/emul_extended.c`, `emul_xfc.c` — extended/
  miscellaneous and the XFC (custom) opcode hook.

## Deliverables

- Instruction handlers with unit tests, in particular CALLS/CALLG frame layout
  round-tripped against a hand-built or fixture-derived expected stack image (this is
  one of the trickier families to get byte-for-byte right — worth extra fixture-based
  coverage).
- With this phase done, the CPU instruction set (Phases 03-07) is complete.

## Open questions / notes

- **LDPCTX/SVPCTX** (`emul_procreg.c`, opcodes 0x06/0x07): not part of this phase's
  scope despite living in the same C file as MTPR/MFPR. Per user direction
  (2026-09-14), deferred and left as `unimplementedHandler` — they're full
  process-context-switch instructions (PCB memory layout, P0BR/P0LR/P1BR/P1LR,
  interrupt-stack flag) with no other consumer of a PCB layout yet to validate
  against; better placed once a real process/PCB concept exists (RTL/console
  phases) than implemented in isolation.
- **XFC** (opcode 0xFC): not a real VAX ISA instruction — eVAX's own escape hatch
  into console I/O, DCL parsing, and RTL system-service dispatch, none of which
  exist in this port yet. Per user direction (2026-09-14), deferred entirely and
  left as `unimplementedHandler`, including its two VM-probe sub-codes (XFC$VMR/
  XFC$VMW) that would otherwise be portable now.
- **REI's AST/software-interrupt tail**: `emul_call.c`'s `emul_rei` ends with two
  blocks that call `interrupt()` (deliver a pending AST, deliver a pending SISR
  software interrupt) — the device-interrupt-queue admission routine Phase 03
  already deferred to Phase 09 (see `docs/PHASE-03.md`). REI's own PC/PSL/stack
  restore has no dependency on that machinery and is fully ported; the AST/SISR
  tail is not.
- **RET's console-CALL-command sentinel**: `emul_ret`'s magic-`FFFFDEAF`-frame
  halt handling is a Phase 08 console feature (returning from a console `CALL`
  command) with no console yet to drive it — not ported.
- **MTPR/MFPR device/interrupt side effects**: `emul_procreg.c`'s `set_priv_reg`
  has register-specific side effects this port doesn't model yet — IPL/SIRR's
  pending-interrupt delivery (both call `interrupt()`, deferred to Phase 09 same
  as REI's tail above), and ICCS/RXCS/TXCS/TXDB's console/clock device modeling
  (also Phase 09 I/O). All fall through to a plain register store in
  `internal/cpu/procreg.go`'s `setPrivReg` rather than replicating device
  behavior that doesn't exist yet. TBIA/TBIS are no-ops (no TB cache exists to
  invalidate — see Phase 02's design notes).

## Progress Log

### 2026-09-14 — Sub-phase 1: CALLS/CALLG/RET/REI

- Added `internal/cpu/call.go`, porting `emul_call.c`'s `emul_call` (shared
  CALLS/CALLG handler), `emul_ret`, and `emul_rei` (minus the AST/SISR tail and
  console sentinel noted above).
- CALLG's arglist operand is table-declared `AccessAddress`; a register-mode
  arglist already faults a reserved-addressing-mode exception at decode time via
  the Phase 03/04 `operand.go` fix, making `emul_call.c`'s own `is_register[0]`
  special case (which would use the register's *value* as the address) dead code
  in this port — not a behavior loss, since the manual's `arglist.ab` (access
  type A) notation forbids register mode the same way MOVAx/PUSHAx already do.
- Found that `emul_call.c` and `emul_ret` implement only a fraction of the
  manual's documented CALLS/CALLG/RET PSW effects, and per the user's explicit
  direction (2026-09-14) — given how load-bearing these instructions are —
  fixed rather than merely logged in `docs/DEVIATIONS.md`:
  - CALLS/CALLG: `emul_call.c` never checks Note 1 (reserved-operand fault if
    entry mask bits 13:12 are nonzero), never clears condition codes, never sets
    IV/DV from entry mask bits 14/15, and never clears FU. All four now
    implemented against the live PSL, with the frame's saved psw snapshot
    additionally forcing T to 0 (per "PSW... with T cleared", distinct from the
    live PSW's T, which is unaffected).
  - RET: `emul_ret` only ever restores PSL bits 6:15 (FU/DV and otherwise-unused
    bits) from the popped frame, leaving N/Z/V/C/T/IV at whatever the callee's
    own execution left them, and never checks Note 1 (reserved-operand fault if
    the popped mask longword's bits 15:8 are nonzero). Replaced with the
    manual's actual "PSW <- tmp1<15:0>" full 16-bit replacement (which subsumes
    the separately-stated N/Z/V/C-from-tmp1<3:0> formula) plus the Note 1 check.
  - The CALLS-count-pop's use of the full 32-bit popped value rather than just
    its low byte (manual: "four times the unsigned value of the low byte") was
    left as-is — a much lower-stakes quirk (self-consistent with CALLS's own
    unmasked push of the same value) than the PSW-effect gaps above, so not
    included in the fix.
- `internal/cpu/call_test.go` covers: CALLS's frame construction hand-verified
  against a fully worked address trace (including a misaligned initial SP to
  exercise the SPA bits), a zero-argument CALLS/RET round trip, CALLG's arglist-
  as-address semantics and its register-mode decode-time fault, both new
  reserved-operand-fault fixes (entry mask bits 13:12 on CALL, popped mask bits
  15:8 on RET), the condition-code/IV/DV/FU fix on CALL, RET's full-PSW-restore
  fix (via a CALLG/RET round trip that tampers with live PSL in between to prove
  the restore reads the frame, not live state), and REI reversing a hand-built
  mode-switch frame (PC, full PSL including CurMod, and both mode stack
  pointers).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean.

### 2026-09-14 — Sub-phase 2: MTPR/MFPR

- Added `internal/cpu/procreg.go`, porting `emul_procreg.c`'s `emul_mtpr`,
  `emul_mfpr`, and `set_priv_reg` (including the `init_reg_access`
  privileged-register access-kind table). LDPCTX/SVPCTX, also in this C file,
  remain deferred per this doc's open questions.
- Register-specific side effects that depend on subsystems this project hasn't
  built yet (device-interrupt admission, console/clock devices, the
  translation-buffer cache) are not replicated — see this doc's open questions
  above for the full list and rationale. Everything else (bounds/mode/access
  checking, IPL's PSL mirroring, ASTLVL's range check, SIRR's SISR queuing,
  every other register's plain read/write) is ported faithfully.
- Found one asymmetry worth noting but not fixing (out of scope per the user's
  CALL/RET-specific direction above): `emul_mtpr.c` truncates its register-
  number operand to a 16-bit signed `short` before range-checking it, while
  `emul_mfpr.c` checks its own register-number operand at full 32-bit width —
  both replicated as read.
- `internal/cpu/procreg_test.go` covers a default-register MTPR/MFPR round
  trip, the kernel-mode check (called directly rather than through
  `Engine.Step`, to avoid needing a page table for the mode-switch side effect
  `docs/DEVIATIONS.md` already covers elsewhere), out-of-range and no-access
  register faults for both instructions, IPL's masking/PSL-mirroring, ASTLVL's
  bounds check (both the fault and a valid case), SIRR's SISR-queuing, and
  TBIA/TBIS's no-op behavior.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean.

### 2026-09-14 — Sub-phase 3: ADAWI, EMUL, EDIV

- Added `internal/cpu/interlock.go` (ADAWI, the only interlocked instruction this
  emulator implements) and `internal/cpu/extended.go` (EMUL, EDIV), porting
  `emul_interlock.c` and `emul_extended.c`. `emul_extended.c`'s longword-pair swap
  dance around its `union XLONG` (a 64-bit-`LONGWORD`-build workaround, see
  `reference/AUDIT.md`) isn't ported — this port computes both instructions'
  64-bit product/dividend directly with Go's native `int64`.
- Found two deviations from the manual in `emul_interlock.c`'s ADAWI condition
  codes, and one in `emul_extended.c`'s EDIV, all logged in `docs/DEVIATIONS.md`
  and replicated as-is (not covered by this phase's CALL/RET-specific fix
  direction): ADAWI's C is always cleared rather than reflecting a real carry
  (`SETCONDITIONBITS`'s unsigned-vs-zero comparison can never be true); ADAWI's
  N/Z come from the untruncated 32-bit sum rather than the word actually stored,
  which disagree exactly in the overflow case; and EDIV never detects a genuine
  quotient-overflow (only a zero divisor sets V), consistent with no instruction
  in this codebase yet raising the architected arithmetic-trap fault for an
  integer-overflow V at all.
- `internal/cpu/interlock_test.go`/`extended_test.go` cover ADAWI's normal-add
  path, its overflow/V case (also demonstrating the untruncated-sum N/Z
  deviation), its never-sets-carry deviation, its register-mode-destination and
  odd-address reserved-operand faults; EMUL's positive/negative/zero-product
  condition codes; and EDIV's normal division, negative-dividend remainder-sign
  (Note 1), and divide-by-zero fallback (Note 3).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean.
