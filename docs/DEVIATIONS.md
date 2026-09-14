# ISA / hardware-definition deviations

This is a running log of places where the C reference source (`reference/eVAX/`)
appears not to match the VAX architecture/instruction-set spec, or not to do what its
own comments say it does — found in the natural course of porting, not part of a
dedicated audit.

This is distinct from `reference/eVAX/AUDIT.md`, which documents the C project's own
closed 32-vs-64-bit portability audit. Findings here are about ISA/behavioral fidelity,
not C portability.

## Policy

Per project guidance (see `CLAUDE.md`): the C source's fidelity to the VAX ISA is good
but not perfect. When a suspected ISA/behavior mismatch is found while porting:

- If it's a clear, obvious logic error unrelated to ISA semantics (e.g. an off-by-one,
  a copy-paste mistake, dead/unreachable code) — just fix it in the Go code, no need to
  log it here.
- If it's a genuine ISA/hardware-definition fidelity question (the emulated behavior
  doesn't match the spec, or doesn't match what the C code's own comments claim) — log
  it below with enough detail to revisit later, and **replicate the C source's current
  behavior in the Go port** rather than fixing it now, unless the fix is clear-cut and
  fits naturally in the current change's scope.
- When it's not obvious which of the above applies, ask before deciding unilaterally.

Entries get resolved (fixed or deliberately kept, with rationale) during Phase 12
(`PHASE-12.md`) or whenever the relevant subsystem gets a dedicated debugging pass.

## Open findings

_None yet._

## Resolved findings

### [Phase 03] Autoincrement Deferred (`@(Rn)+`) eagerly loads the operand's value instead of resolving its address

- **Where**: `reference/eVAX/eVAX/Source/CPU/decode_operand.c`, `decode_operand()`'s
  `case 0x09` (~lines 458-479), ported to `internal/cpu/operand.go`'s
  `decodeGeneral`.
- **What**: For every other deferred addressing mode in this function — byte/word/long
  Displacement Deferred (`case 0x0B/0x0D/0x0F`, ~lines 493-551) and the PC-relative
  Byte/Word/Long Relative Deferred forms in `decode_opcode.c`'s PC-mode branch — the
  pattern is consistent: compute a pointer address, dereference it _once_ to get the
  operand's real address, and store that address in `opcode->VAXaddr[n]` for
  `get_operand`/`put_operand` to read or write through at execute time (respecting
  `OP_WR`/`OP_MD` write-back). `case 0x09` (Autoincrement Deferred, `@(Rn)+`) breaks
  this pattern: it dereferences the pointer, then immediately dereferences the _result_
  a second time (`load_register(treg, vax.reg[reg], 4)` then
  `load_register(treg, vax.reg[treg], 4)`) and stores the loaded value's address via
  `opcode->address[n]` (the "already-resolved, no execute-time indirection needed"
  path) instead of `VAXaddr[n]`. This is inconsistent with the addressing mode's own
  definition (`Rn` holds the address of a longword containing the operand's address —
  one level of indirection, not two) and, more importantly, would silently break any
  instruction that _writes_ through this mode: `put_operand` would write into the
  scratch register `get_operand` eagerly loaded into, not back to the real VAX memory
  location the pointer addressed, e.g. `CLRL @(R0)+` or `MOVL R1,@(R2)+` as a
  destination would discard the write.
- **Status**: fixed in Go. `decodeGeneral`'s `case 0x09` dereferences the pointer once
  and resolves an `OperandMemory` operand (`Addr` = the dereferenced value), exactly
  mirroring the other deferred modes' pattern — internally consistent with sibling code
  in the same function, and the natural behavior of this port's value-based `Operand`
  type in the first place (see `docs/PHASE-03.md`'s design notes: nothing in this
  design _has_ a "scratch register to eagerly load into" the way the C source's
  pointer-aliasing mechanism does, so reproducing the bug would have taken deliberately
  added complexity, not less). Verified by
  `internal/cpu/operand_test.go`'s `TestDecodeOperandAutoincrementDeferred`.

### [Phase 03] Indexed-mode-nested-in-Indexed-mode rejection doesn't route through the normal fault mechanism

- **Where**: `reference/eVAX/eVAX/Source/CPU/decode_operand.c`, `case 0x04` (~lines
  403-431): `if (idx) return VAX_ILLADDRFAULT;`.
- **What**: The VAX architecture reserves Index mode as the base addressing mode of
  another Index mode operand specifier (a reserved addressing mode fault,
  `EXC_RESADDR`). The C source clearly intends to reject this — it's the only
  addressing-mode-structural check in the function — but signals it with
  `VAX_ILLADDRFAULT`, a raw return-code sentinel that never touches `vax.fault`/
  `set_fault` the way every other decode-time fault in this file does (e.g. the
  short-literal-write-access check just above it, which does call `set_fault`). Its
  caller (`execute_vax` in `vax.c`) unconditionally calls `handle_fault()` on any
  nonzero decode result, which reads `vax.fault.code` — left stale from whatever fault
  (if any) last set it, not this one. The clearly-intended behavior (reject with
  `EXC_RESADDR`) doesn't actually reach the exception vector it should.
- **Status**: fixed in Go. `decodeGeneral`'s `case 0x04` returns
  `&Fault{Code: ExcReservedAddr}` directly through this port's normal fault-return
  path (the same one every other decode-time fault uses), rather than a disconnected
  sentinel — this is fixing broken plumbing behind an already-clear intent, not
  second-guessing an ISA judgment call. Verified by
  `internal/cpu/operand_test.go`'s `TestDecodeOperandDoubleIndexedFaults`.

### [Phase 03] `set_mode_stack` sets `MAPEN = 1` on every non-interrupt-stack mode switch

- **Where**: `reference/eVAX/eVAX/Source/CPU/interrupt.c`, `set_mode_stack()`
  (~line 481): `vax.MAPEN = 1;        /* Not sure about this!! */`.
- **What**: the C source's own comment flags this write as uncertain — every switch to
  a KSP/ESP/SSP/USP-based mode (as opposed to the interrupt stack, which explicitly
  sets `MAPEN = 0`) unconditionally forces virtual memory _on_, regardless of whatever
  `MAPEN` held before the exception. This isn't obviously wrong (kernel-mode exception
  handlers on a running VMS-like OS would have VM enabled anyway), but it's also not
  obviously right for early boot / console-level fault handling before VM is set up,
  and the original author didn't resolve it either.
- **Status**: deferred — replicated as-is in `Engine.setModeStack`
  (`internal/cpu/handlefault.go`), per this project's policy of not second-guessing an
  ISA judgment call the original author explicitly marked as unresolved. Revisit in
  Phase 12 or whenever VM-disabled fault handling is actually exercised end-to-end.

### [Phase 02, noticed in Phase 03] `vm.TranslationFault` collapses `EXC_ACCVIO`'s two distinct signal subcodes

- **Where**: `reference/eVAX/eVAX/Source/CPU/vm.c`'s `vm()` (~lines 423-503): a region
  length/base violation signals `set_fault(EXC_ACCVIO, 2, addr, 0x0001)`, a protection
  violation signals `set_fault(EXC_ACCVIO, 2, addr, 0x0002)` — the second signal
  argument distinguishes *why* the access violation happened. `internal/vm/translate.go`
  (Phase 02) has a single `AccessViolation` `FaultKind` for both cases (see its
  `accessViolation` helper), losing that distinction.
- **What**: Phase 02 predates fault-signaling entirely (`docs/PHASE-02.md` explicitly
  left `set_fault`'s translation to Phase 03), so this wasn't a fidelity question until
  now — `wrapMemError`'s `vm.TranslationFault` → `cpu.Fault` mapping (`internal/cpu/
  dispatch.go`, called from `Engine.raise` in `internal/cpu/engine.go`) has no subcode
  to recover and reports `0x0001` (length/base violation) unconditionally for every
  `AccessViolation`, matching the more common of the two C call sites (and the same
  subcode `decode_opcode.c`/`decode_operand.c`'s own inlined physical-address-
  resolution faults already use).
- **Status**: deferred. A real fix means widening `vm.TranslationFault.Kind` with a
  third case (or a length-vs-protection sub-field) in Phase 02's territory, which is
  out of scope for a Phase 03 change; revisit alongside Phase 02 or in Phase 12.

### [Phase 04] Register mode used where `OP_AD`/`OP_VA` access is required

- **Where**: `reference/eVAX/eVAX/Source/CPU/decode_operand.c`'s `decode_operand()`
  never rejects Register mode (`mode == 5`) for an operand whose access is `OP_AD`
  (e.g. `MOVAL`/`PUSHAL`, `JMP`/`JSB`) or `OP_VA` — a register has no VAX address. A few
  individual C handlers (`emul_mova.c`, half of `emul_push.c`) work around this ad hoc
  by testing `opcode->is_register[0]` themselves and faulting `EXC_RESADDR`, but decode
  itself never enforces it, so any future `OP_AD`/`OP_VA` consumer that forgets the
  check would silently accept an illegal encoding.
- **What**: per user direction (explicitly requested when starting Phase 04), this
  should fault, and generally rather than per-handler.
- **Status**: fixed in Go, at decode time. `decodeOperand`'s register-mode fast path
  (`internal/cpu/operand.go`) now returns `&Fault{Code: ExcReservedAddr}` for any
  `AccessAddress`/`AccessVarField` operand that resolves to Register mode — one fix
  covering every current and future `OP_AD`/`OP_VA` consumer (Phase 04's
  `MOVAx`/`PUSHAx`/`JMP`/`JSB` now, Phase 06's bitfield `OP_VA` operands later) rather
  than a check repeated in each handler. Verified by `internal/cpu/operand_test.go`'s
  `TestDecodeOperandAccessAddressRejectsRegisterMode`.

### [Phase 04] `emul_movb_negated`'s stray extra `vax.pslw.v = 0` discards MNEGB's overflow flag

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_mov.c`'s `emul_movb_negated()`
  (~line 172): `vax.pslw.v = 0;` runs unconditionally right after the `n`/`z` are set,
  a few lines after the MNEGB overflow branch already set `vax.pslw.v = 1`.
  `emul_movw_negated`/`emul_movl_negated` don't have this extra line.
- **What**: a clear copy-paste bug, not an ISA judgment call — MNEGB can never actually
  report overflow in the C source, while MNEGW/MNEGL (same shape, same file) do.
- **Status**: fixed in Go. `emulMneg` (`internal/cpu/mov.go`), shared across
  MNEGB/MNEGW/MNEGL, sets V once and doesn't clobber it. Verified by
  `internal/cpu/mov_test.go`'s `TestEmulMneg/overflow_(largest_negative)`.

### [Phase 04] MNEGx's carry flag uses "source LSS 0" instead of the manual's "source NEQ 0"

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_mov.c`'s `emul_movb_negated`/
  `emul_movw_negated`/`emul_movl_negated`, the MNEG branch: `vax.pslw.c = (data < 0) ?
  1 : 0;` where `data` is the *original* source value.
- **What**: `vax_instr_set.pdf`'s MNEG entry defines `C <- dst NEQ 0`. Since negation
  maps zero to zero and nothing else to zero (including the overflow case, which
  leaves `dst` as the unchanged nonzero source), `dst NEQ 0` is equivalent to `source
  NEQ 0` — true for *any* nonzero source, not just a negative one. The C source's
  formula agrees only when the source is already negative or zero; for a positive
  source (e.g. negating 5) the manual says C should be 1 (0 − 5 borrows) but the C
  source computes 0.
- **Status**: fixed in Go. `emulMneg` (`internal/cpu/mov.go`) sets `C` from `source !=
  0`. Verified by `internal/cpu/mov_test.go`'s `TestEmulMneg` table (`"positive"` and
  `"negative"` cases both expect `C=true`).

### [Phase 04] The `SETCONDITIONBITS(x, 0L)` idiom incidentally forces C false and skips V

- **Where**: several `emul_*.c` handlers call `SETCONDITIONBITS(data, 0L)` purely to
  get N/Z ("is the result negative/zero") as a side effect of the macro's two-operand
  compare shape (`vax.h`'s `SETCONDITIONBITS`). Confirmed instances in this phase's
  scope: `emul_mov.c`'s `emul_movq`, `emul_ash.c`'s `emul_rotl`/`emul_ash` (both ASHL
  and ASHQ), and `emul_loop.c`'s `emul_aobleq`/`emul_aoblss`/`emul_sobgtr`/
  `emul_sobgeq`.
- **What**: the macro's C formula (`(ULONGWORD)(v1) < (ULONGWORD)(v2)`) is always false
  when `v2` is a literal 0 (nothing is less than unsigned zero), and the macro never
  touches V at all. `vax_instr_set.pdf` specifies `C <- C` (unchanged) for every one of
  these instructions, and a real V formula for ROTL (`V <- 0`, matching what the macro
  accidentally produces) and ASHL/ASHQ/the four loop instructions (`V <- {integer
  overflow}`, never computed by the C source at all — V is left as whatever the
  previous instruction set it to).
- **Status**: fixed in Go, per instruction, as each is ported: N/Z computed explicitly,
  V computed per the manual's overflow definition where one applies (0 for MOVQ/ROTL;
  a real shift/increment overflow check for ASHL/ASHQ/AOBLEQ/AOBLSS/SOBGTR/SOBGEQ), C
  left untouched. MOVQ fixed in `internal/cpu/mov.go` (this sub-phase); ROTL/ASHL/ASHQ
  and the loop instructions follow the same fix when their sub-phases land.

### [Phase 04] BIT's carry flag is force-cleared instead of left unchanged

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_cmp.c`'s `emul_cmp()`: `cbit` is
  initialized to 0 and only ever reassigned in the CMP case (`func == 0`); BIT and TST
  both fall through with `cbit == 0`, and `vax.pslw.c = cbit;` runs unconditionally
  after the switch.
- **What**: `vax_instr_set.pdf`'s BIT entry specifies `C <- C` (unchanged); its TST
  entry specifies `C <- 0`, which the C source gets right. Only BIT is wrong.
- **Status**: to be fixed when this phase's compare/bit-test/test sub-phase lands (see
  `docs/PHASE-04.md`'s sub-phase list) — noted here now since it was found while
  reading `emul_cmp.c` ahead of that sub-phase, not deferred to avoid forgetting it.

### [Phase 04, deferred] ADWC/SBWC operate on word operands; the manual specifies longword

- **Where**: `reference/eVAX/eVAX/Headers/instruction_table.h`'s ADWC/SBWC entries
  (operand scale `{2, 2, ...}`, i.e. word) and `emul_integer_math.c`'s `emul_integer_
  math()`, which special-cases `op == 0xD8`/`0xD9` to `dsize = 5` (word) rather than
  falling through to the longword (`dsize == 6`) case its opcode range would otherwise
  select.
- **What**: `vax_instr_set.pdf`'s ADWC format line reads `add.rl, sum.ml` — longword
  operands (`.rl`/`.ml`), not word.
- **Status**: deferred, replicated as-is (word-sized). The operand size is baked into
  the *mechanically generated* `instructions_table.go` (see Phase 03's sub-phase 1 —
  generated from `instruction_table.h`, not hand-maintained), so a real fix means
  changing generated table data or the generator itself, not a Phase 04 handler change
  — out of scope for this phase. Revisit in Phase 12 or alongside `instruction_table.h`
  generation.

### [Phase 04, deferred] Byte/word carry-flag formulas use signed narrow-type arithmetic

- **Where**: `emul_increment.c` (INCx/DECx) and `emul_integer_math.c`'s byte/word paths
  (ADDx/SUBx/ADWC/SBWC) read the source operand(s) as a *signed* `char`/`short`, then
  test `data & 0x100`/`0x10000` (a bit above the operand's width) for carry-out.
- **What**: this only matches `vax_instr_set.pdf`'s "carry from the most significant
  bit" definition when the operand's sign bit is clear. E.g. incrementing byte 0xFF
  (255 unsigned, -1 signed): the C source computes `d1 = -1`, `data = d1 + 1 = 0`,
  `data & 0x100 == 0` → C reported as 0, but the true unsigned carry (255 + 1 = 256)
  should set C. The bug only appears when the source's high bit is set — the common
  case (small positive values) is unaffected, which is presumably why it's gone
  unnoticed.
- **Status**: deferred, replicated as-is. This is a pervasive pattern across most of
  this phase's integer arithmetic, not an isolated bug — fixing it well means
  redesigning the carry computation consistently across every affected handler, which
  risks introducing new bugs mid-phase rather than "fits naturally in scope." Revisit
  in Phase 12 with real test vectors (including hardware-verified ones if available)
  rather than guessing at the fix under phase-completion pressure.

<!--
Entry template:

### [Phase NN] Short title

- **Where**: `reference/eVAX/eVAX/Source/.../file.c` (function/lines), ported to
  `internal/.../file.go`.
- **What**: what the C code does vs. what the ISA manual / the C code's own comments
  say it should do.
- **Status**: deferred (Go replicates C behavior as-is) | fixed in Go (rationale).
-->
