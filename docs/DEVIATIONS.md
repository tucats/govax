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

### [Phase 05] `emul_float_math.c`'s D-floating arithmetic path calls `fpu_load` with its source longwords swapped

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_float_math.c`'s `emul_float_math()`,
  the `dsize == 3` (D_FLOAT) branch of the basic-math `switch`:
  `rc = fpu_load( d[1], d[0], &float1 );` (and the identical pattern for `float2`).
- **What**: `fpu_load`'s signature is `fpu_load(LONGWORD src1, LONGWORD src2, double
  *result)`; the correct convention — confirmed empirically, not by tracing `fpu.c`'s
  byte-shuffle loops (see `docs/PHASE-05.md`'s design notes) — is `src1` = the
  operand's low-address (first) longword, `src2` = its high-address (second) longword.
  `emul_mov.c`'s `emul_movd` (MOVD/MNEGD) calls `fpu_load(data[0], data[1], &dbl)`,
  the correct order; `emul_float_math.c`'s D-floating math path passes them reversed.
  Verified with a standalone C harness built against the real `fpu.c`
  (`clang -DLINUX86 -I eVAX/Headers probe.c fpu.c`): storing `3.14159265358979` as
  D_floating and reading it back via `fpu_load(data[1], data[0])` (this bug's order)
  returns `-5.4687269079054549e-19`; via `fpu_load(data[0], data[1])` (the correct
  order) it returns the original value exactly. This corrupts every D-floating source
  read the C reference actually dispatches: `ADDD2/3`, `SUBD3`, `MULD2/3`, `DIVD2/3`,
  and `CVTBD`/`CVTWD`/`CVTLD`.
- **Status**: fixed in Go. `internal/cpu/fpu.go`'s `fpuLoad` is a single, from-scratch
  implementation used uniformly by every D-floating consumer (arithmetic, conversion,
  MOV/MNEG, compare/test, ACB) — there is only one calling convention in the Go port,
  and it's the confirmed-correct one, so this bug has no way to resurface per-caller
  the way it did across `emul_float_math.c`/`emul_mov.c`'s two separate C
  implementations. Verified by `internal/cpu/fpu_test.go`'s
  `TestFpuStoreDoubleFloatingKnownValues`/`TestFpuLoadRoundTrip` (D_floating cases)
  using the harness's own known-good raw bit patterns.

### [Phase 05] `fpu_store`'s underflow flush-to-zero is dead code

- **Where**: `reference/eVAX/eVAX/Source/CPU/fpu.c`'s `fpu_store()`, the underflow
  branch (`if( expon < -127 )`): `local = 0.0; expon = 0;` when `PSL<FU>` is clear.
- **What**: `local` is a separate `double` from `source` (the function's actual
  parameter); every bit-extraction line below this assignment reads through `pd`,
  which was set up earlier as `pd = (ULONGWORD *) &source` — aliasing the original,
  un-zeroed parameter, never `local`. The intended flush to `0.0` therefore has no
  effect on the function's output. Confirmed with the same standalone C harness used
  for the finding above: storing `1e-100` with `PSL<FU>` clear returns `rc == 0` (no
  fault, as expected) but a nonzero raw longword (`0xF977405F`, not `0x00000000`) —
  the original tiny value's mantissa bits with only the exponent field forced to a
  fixed value, not a flush to zero.
- **Status**: fixed in Go. `internal/cpu/fpu.go`'s `fpuStore` returns a genuine `0` on
  underflow with `PSL<FU>` clear, matching the explicit `value == 0` short-circuit one
  branch above it in the same function. Verified by `internal/cpu/fpu_test.go`'s
  `TestFpuStoreUnderflowFlushesToZeroWhenFUClear`.

### [Phase 05] Seven D-floating opcodes have no working operand data or dispatch in the C reference

- **Where**: `reference/eVAX/eVAX/Headers/instruction_table.h` (`SUBD2` at 0x62,
  `CVTDB`/`CVTDW`/`CVTDL`/`CVTRDL` at 0x68-0x6B, `CMPD`/`TSTD` at 0x71/0x73) and
  `reference/eVAX/eVAX/Source/Initialization/init_emulators.c`.
- **What**: `SUBD2`'s table row has `OperandCount: 0` despite correctly populated
  scale/access columns (a pure data-transcription slip — its siblings `MULD2`/`DIVD2`
  are correct) and _does_ have a dispatch entry (`instruction[0x62].routine =
  emul_float_math`), so decode would read it as a zero-operand instruction despite the
  handler expecting two. `CVTDB`/`CVTDW`/`CVTDL`/`CVTRDL`/`CMPD`/`TSTD` are worse: an
  all-zero table row (count, scale, _and_ access all zero) _and_ no
  `init_emulators.c` dispatch entry at all — genuinely never wired up, not just
  mis-tabled. `emul_cmp.c`'s shared handler (which `CMPD`/`TSTD` would need) has no
  `case 3` (D_FLOAT) in its `switch(dsize)` either, so even a hypothetical dispatch fix
  would have no working logic behind it. Confirmed by reading the C source directly
  (not inferred); severity (non-functional, not just subtly wrong, unlike Phase 04's
  BISB3 finding) surfaced to the user rather than decided unilaterally, since it cuts
  into this phase's own named D-floating deliverables.
- **Status**: fixed, per user direction. `internal/cpu/gen/main.go`'s
  `knownTableFixes` patches these seven entries at `go generate` time (so the fix
  survives regeneration rather than being hand-edited into the generated,
  DO-NOT-EDIT `instructions_table.go` and later silently reverted), mirroring each
  entry's already-correct F-floating/sibling D-floating row. Since five of the seven
  have no working C implementation to port at all, their instruction handlers
  (`internal/cpu/`) are implemented from the ISA manual and by direct analogy to the
  F-floating/D-floating siblings that _do_ work, not ported from C. Verified by
  `internal/cpu/instruction_test.go`'s `TestInstructionTableDFloatingFixedEntries`
  (table-level) and by each opcode's own handler tests as they land.

### [Phase 05] Floating ADD/SUB/MUL/DIV never clear V/C, and F_floating overflow silently swallows the fault

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_float_math.c`'s `emul_float_math()`,
  the basic-math `switch(dsize)` block (func 0-3).
- **What**: two related omissions. First, the function sets `vax.pslw.n`/`vax.pslw.z`
  from the result but never explicitly clears `vax.pslw.v`/`vax.pslw.c` on the normal
  path — the manual's ADDF/SUBF/MULF/DIVF (and D-floating counterparts) entry specifies
  `V <- 0, C <- 0`, the same "forgot to clear" pattern already found and fixed
  repeatedly elsewhere in this project via the `SETCONDITIONBITS(x, 0L)` idiom (Phase
  04), just without even that idiom's incidental C-clearing here. Second, and more
  serious: on F_floating overflow specifically, `fpu_store`'s fault return sets
  `vax.pslw.v = 1` but — unlike the D_floating branch two lines below it in the same
  `if`/`else`, which correctly does `if( rc ) return rc;` — falls through to
  `put_operand`, writing whatever `fpu_store` left in its (never actually populated on
  the fault path) destination longword and returning success instead of propagating
  the synchronous arithmetic fault. A real VAX floating overflow is always a
  synchronous exception; this is an inconsistency between two nearly-identical
  branches of the same conditional, not a considered design choice.
- **Status**: fixed in Go. `internal/cpu/floatmath.go`'s `setFloatPSL` always sets
  `V <- 0, C <- 0` on the surviving path (genuine overflow already faults before this
  runs); `storeFloatResult`/`storeFloat`/`fpuStore` propagate a real `*Fault` uniformly
  for both F_floating and D_floating, with no special-casing that could reintroduce
  the asymmetry. Verified by `internal/cpu/floatmath_test.go`'s
  `TestEmulFAddZeroSetsZ` (C explicitly primed dirty beforehand) and
  `TestEmulFAddOverflowFaults`.

### [Phase 05] CVTRFL/CVTRDL (round-to-nearest float→long) always fault instead of rounding

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_float_math.c`'s `emul_float_math()`,
  the `func == 4` (float→integer) handler's `switch( dsize2 )`.
- **What**: the switch only has cases for `0` (Byte), `1` (Word), and `2` (Long);
  `dsize2 == 3` — `op & 0x03 == 3`, which is exactly `CVTRFL`/`CVTRDL` — falls to
  `default: set_fault( EXC_PRIV, 0 ); return VAX_FAULT;`. `CVTRFL`/`CVTRDL` are
  correctly tabled and dispatched (this is a gap in the shared handler's own switch,
  unlike this doc's D-floating table/dispatch finding above), so every use of either
  instruction in the C reference faults as a reserved/privileged-instruction violation
  rather than rounding — there is no working "truncate instead of round" behavior to
  preserve either, it simply never executes.
- **Status**: fixed in Go. `internal/cpu/cvtfloat.go`'s `emulCvtRoundFloatToInt`
  implements real round-to-nearest (ties away from zero, via `math.Round`, applied
  before the overflow bounds check since rounding can itself push an in-range value
  out of range) — implemented fresh from the manual's `CVTRFL`/`CVTRDL` entries, not
  ported from C. Verified by `internal/cpu/cvtfloat_test.go`'s
  `TestEmulCvtRoundVsTruncate` (rounding vs. truncation, including a tie case and a
  negative-direction case) and `TestEmulCvtRoundFloatToIntOverflow` (a value that only
  overflows after rounding).

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
- **Status**: fixed in Go, all eight instances. N/Z computed explicitly, V computed per
  the manual's overflow definition where one applies (0 for MOVQ/ROTL; a real
  shift/increment overflow check for ASHL/ASHQ/AOBLEQ/AOBLSS/SOBGTR/SOBGEQ, the latter
  four reusing `addResult`/`subResult` since they're arithmetically exactly
  INCL/DECL), C left untouched throughout. MOVQ: `internal/cpu/mov.go`. ROTL/ASHL/ASHQ:
  `internal/cpu/ash.go`. AOBLEQ/AOBLSS/SOBGTR/SOBGEQ: `internal/cpu/loop.go`.

### [Phase 04] BIT's carry flag is force-cleared instead of left unchanged

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_cmp.c`'s `emul_cmp()`: `cbit` is
  initialized to 0 and only ever reassigned in the CMP case (`func == 0`); BIT and TST
  both fall through with `cbit == 0`, and `vax.pslw.c = cbit;` runs unconditionally
  after the switch.
- **What**: `vax_instr_set.pdf`'s BIT entry specifies `C <- C` (unchanged); its TST
  entry specifies `C <- 0`, which the C source gets right. Only BIT is wrong.
- **Status**: fixed in Go. `internal/cpu/cmp.go`'s `emulBit` uses `setLogicalPSL` (the
  same BIS/BIC/XOR-shaped "N/Z from result, V <- 0, C unaffected" helper from
  `internal/cpu/integermath.go`) instead of reusing CMP's C computation; `emulTst`
  explicitly sets both V and C to 0, matching the C source's (already-correct) TST
  behavior. Verified by `internal/cpu/cmp_test.go`'s `TestEmulBitLeavesCarryUnaffected`.

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

### [Phase 04] `emul_increment.c`/`emul_integer_math.c`'s N/Z/V/C formulas are broken for byte and longword sizes, and for BIS/BIC's C

This entry originally read "deferred" — logged below is the superseded reasoning,
followed by what actually shipped. Kept for the record rather than silently rewritten.

**Superseded plan**: the first pass through `emul_increment.c` found that its byte/word
carry computation reads the source as a *signed* `char`/`short` and tests
`data & 0x100`/`0x10000` for carry-out, which only matches the manual's "carry from the
most significant bit" when the operand's sign bit is clear (e.g. incrementing byte 0xFF:
C source computes `d1 = -1`, `data = 0`, reports C=0, but the true unsigned carry
(255+1=256) should set C). This looked like a pervasive, hard-to-fix-safely pattern
across INC/DEC and ADD/SUB/ADWC/SBWC, so the plan was to replicate it as-is and defer a
proper fix to Phase 12.

**What changed**: while deriving INC/DEC's overflow formula from the manual's own
explicit notes ("integer overflow occurs if the largest positive integer is
incremented"), a wider check of `emul_integer_math.c`'s V/C formulas against
`vax_instr_set.pdf`'s ADD/SUB/MUL/DIV/BIS/BIC entries turned up more, and worse,
confirmed defects than the carry-flag one alone:

- **Byte-size V range check uses the wrong constants.** `emul_increment.c`/
  `emul_integer_math.c`'s byte case tests `data > 255 || data < -256` for overflow. The
  analogous word case correctly tests `data > 32767 || data < -32768` — exactly
  `INT16_MAX`/`INT16_MIN`, the true signed-word bounds. The byte case's bounds (255,
  -256) are not the signed-byte bounds (127, -128) at all; concretely, incrementing byte
  0x7F (127, the largest positive byte) computes `data = 128`, and `128 > 255` is false,
  so no overflow is reported — but incrementing the largest positive integer is the
  textbook overflow case the manual's own INC note calls out by name. Comparing the
  (correct) word case to the (wrong) byte case in the same function makes this a clear,
  obvious constant error, not an ISA judgment call.
- **The longword V check via `data == udata` cannot detect anything, ever, now that
  `LONGWORD` is genuinely 32-bit.** This "compute the same add as both signed and
  unsigned, overflow iff they disagree" trick only works if the comparison happens in
  *wider* precision than the operands — which is exactly what the pre-audit-fix 64-bit
  `LONGWORD` accidentally provided. Post-fix, both `data` (`LONGWORD`) and `udata`
  (declared `LONGWORD` in `emul_increment.c`, `ULONGWORD` in `emul_integer_math.c`) are
  computed at the *same* 32-bit width as the inputs; two's-complement addition produces
  an identical bit pattern whether the intermediate type is signed or unsigned, so
  `(ULONGWORD)data == udata` is a tautology — always true, always reporting "no
  overflow." Verified concretely: `d1 = 0x7FFFFFFF`, `d2 = 1` — `data` and `udata` both
  come out `0x80000000`, so the check reports no overflow for `INT32_MAX + 1`, a
  textbook signed overflow. This is a real, confirmed regression: the
  `reference/eVAX/AUDIT.md` `LONGWORD`-width fix that closed the C project's own audit
  silently broke this particular overflow-detection idiom everywhere it's used at
  longword size, since the idiom was never widening on purpose — it was relying on
  `LONGWORD` accidentally already being wider than a VAX longword.
- **BIS's C is force-cleared, and BIC's uses the (already-broken) arithmetic carry
  formula.** `emul_integer_math()`'s byte/word cases special-case `func == 4` (BIS) to
  `vax.pslw.c = 0` and fall through to the generic (broken) `data & 0x100` computation
  for `func == 5` (BIC); the manual specifies `C <- C` (unchanged) for both — they're
  bitwise ops, carry has no meaning for them at all.
- **DIV's V doesn't cover the `MinInt / -1` overflow case** (only division-by-zero),
  and dividing by zero unconditionally before the `d1 == 0` check is undefined behavior
  in C (and a runtime panic in Go — the reason this file was touched in the first place;
  see below).

None of this needed guessing at ISA intent — every formula above (`ADD`'s "same-sign
operands, different-sign result", `SUB`'s "borrow" and "different-sign operands,
result differs from the minuend's sign", `MUL`'s "product doesn't fit the destination",
`DIV`'s "divisor is zero, or `MinInt / -1`", `BIS`/`BIC`'s "C unaffected") is stated
explicitly in `vax_instr_set.pdf`'s Condition Codes section for that instruction, and Go
has native 64-bit arithmetic to implement each one *correctly* by computing in wider
precision and checking for truncation — the exact technique the C source's own longword
path was reaching for and structurally couldn't achieve. Given that, and given that
INCx/DECx are specified as exactly equivalent to `ADDx S^#1`/`SUBx S^#1` (so shipping
INC with a correct formula while ADD keeps the broken one would make two opcodes
computing the identical operation disagree on their own condition codes), this was
reclassified from "defer, pervasive and risky" to "fix now, per-instruction, using
spec-derived formulas" — the same bar already used for MNEG's overflow/carry fixes in
sub-phase 2.

- **Status**: fixed in Go. `internal/cpu/condcodes.go` gained generic, size-parameterized
  `addResult`/`subResult`/`mulResult`/`divResult` helpers (wide-arithmetic overflow/carry
  per the formulas above); `internal/cpu/increment.go` (sub-phase 5) and
  `internal/cpu/integermath.go` (sub-phase 6) use them for INC/DEC and
  ADD/SUB/MUL/DIV/BIS/BIC/ADWC/SBWC respectively, replacing every one of the narrow/
  tautological/wrong-constant formulas described above. Divide-by-zero and
  `MinInt / -1` are guarded before the actual Go division (which would otherwise panic)
  rather than left as C's undefined behavior; see each sub-phase's progress log entry
  for the specific tests.

### [Phase 04, deferred] BISB3's destination operand is declared longword-sized

- **Where**: `reference/eVAX/eVAX/Headers/instruction_table.h`'s `BISB3` entry
  (~line 1524-1533): operand scales `{1, 1, 4, 0, 0, 0}` — the third (destination)
  operand is 4 bytes. Every sibling instruction of the identical `Bxx3` shape (ADDB3,
  SUBB3, MULB3, DIVB3, BICB3, XORB3) correctly declares all three operands as
  byte-sized (`{1, 1, 1, ...}`).
- **What**: this looks like a plain transcription error in the reference table (a
  stray `4` where every neighboring entry has `1`), not a deliberate design choice —
  there's no ISA reading under which BISB3 alone would have a wider destination than
  BISB2 or its own siblings. Concretely, `BISB3 mask,src,Rn` with a register
  destination overwrites all 4 bytes of `Rn` (the byte OR result zero-extended)
  instead of only the low byte the way every other `Bxx3`/`Bxx2` form does.
- **Status**: deferred, replicated as-is, for the same reason as the ADWC/SBWC sizing
  finding above: the operand size is baked into the *mechanically generated*
  `instructions_table.go`, so a real fix means changing generated table data or the
  generator, out of scope for a Phase 04 handler change. `internal/cpu/integermath.go`'s
  handlers use each operand's own declared size generically (no special-casing), so
  this deviation surfaces naturally rather than needing separate code to reproduce it.
  Verified (not just asserted) by `internal/cpu/integermath_test.go`'s
  `TestEmulBisb3DestinationScaleDeviation`. Revisit in Phase 12 or alongside
  `instruction_table.h` generation, together with the ADWC/SBWC finding.

### [Phase 04] CVTxy computes N/Z from the source value instead of the truncated destination

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_integer_cvt.c`'s `emul_integer_cvt()`:
  `SETCONDITIONBITS(data, 0L)` runs right after `data` is read and sign-extended at the
  *source* size, before the second `switch` that truncates it to the destination size
  and writes it out. Its byte-destination V check also uses the same wrong constants
  (`255`/`-256` instead of `127`/`-128`) already found in `emul_increment.c`/
  `emul_integer_math.c`.
- **What**: `vax_instr_set.pdf`'s CVT entry specifies `N <- dst LSS 0`, `Z <- dst EQL 0`
  — the *destination* (post-truncation) value. These only disagree when the conversion
  overflows, but then they can disagree outright: converting long `0x00000080` (128,
  positive) to byte truncates to `0x80` (-128, negative) — the manual's N is true, the
  C source's is false.
- **Status**: fixed in Go, using the same wide-arithmetic approach as the other
  integer-arithmetic findings in this phase. `internal/cpu/condcodes.go`'s
  `convertResult` sign-extends the source and truncates to the destination size,
  returning the masked result and a truncation-based V (the manual's own definition:
  "any truncated bits not equal to the sign bit of the destination"); `internal/cpu/
  cvt.go`'s `emulCvt` computes N/Z from that result, not the pre-truncation source.
  Verified by `internal/cpu/cvt_test.go`'s `TestEmulCvtOverflowNZFromDestination` (the
  sign-flip case) and `TestEmulCvtByteOverflowRangeCheck` (the byte constant fix).

### [Phase 04] ACB's branch condition uses strict `<` where the manual specifies `<=`

- **Where**: `reference/eVAX/eVAX/Source/CPU/emul_branch.c`'s `emul_acb()`, the
  non-negative-addend branch condition (present identically in all four size
  variants): `if( addend_b >= 0 && index_b < limit_b ) branch = 1;`.
- **What**: `vax_instr_set.pdf`'s ACB entry: "If the addend operand is positive (or
  zero) and the comparison is less than or equal to zero [index <= limit] ... the
  branch displacement is added to the PC." The C source's strict `<` misses the
  boundary case where a counting-up loop's index lands exactly on the limit on its
  final iteration — e.g. `ACBL #10, #1, index, loop` with `index` reaching exactly
  10 fails to branch, silently dropping the last iteration. The negative-addend
  condition (`index >= limit`) was already correct — inclusive, matching the manual's
  "greater than or equal to zero."
- **Status**: fixed in Go. `internal/cpu/branchacb.go`'s `emulAcb` uses `<=` for the
  non-negative-addend case. Verified by `internal/cpu/branchacb_test.go`'s
  `TestEmulAcbPositiveAddendBoundary` (an index landing exactly on the limit) and
  `TestEmulAcbNotTaken` (confirming the loop still correctly exits once past it).

## Open questions carried forward (not yet findings)

### [Phase 04] CASE's internal arithmetic width for byte/word selector, base, and limit

`emul_case.c` reads the byte/word `selector`/`base`/`limit` operands through a signed
`char`/`short` pointer, so they're sign-extended to full 32 bits before the
subtraction (`idx = selector - base`), the unsigned comparison against `limit`, and
the table-index/skip-distance arithmetic. `vax_instr_set.pdf`'s own note ("the
selector and base operands can both be considered as either signed or unsigned
integers") doesn't settle whether the *internal* arithmetic is meant to happen at the
operand's own declared width (the convention every other instruction in this phase
uses — CMPB, ADDB, etc. all compute purely within their declared byte/word/long size)
or genuinely widened to 32 bits the way the C source does. The two choices only
produce different results when an operand's own high bit is set (e.g. a `CASEB` with a
`selector` byte of 0x80 or above), which is a fairly unusual case values would take in
practice. Not resolved either way — replicated as the C source's sign-extend-then-
32-bit-arithmetic behavior in `internal/cpu/branchacb.go`'s `emulCase` rather than
guessed at. Revisit with the hardware/architecture reference in hand, or ask, rather
than deciding unilaterally.

<!--
Entry template:

### [Phase NN] Short title

- **Where**: `reference/eVAX/eVAX/Source/.../file.c` (function/lines), ported to
  `internal/.../file.go`.
- **What**: what the C code does vs. what the ISA manual / the C code's own comments
  say it should do.
- **Status**: deferred (Go replicates C behavior as-is) | fixed in Go (rationale).
-->
