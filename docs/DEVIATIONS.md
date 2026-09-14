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
  now — `Engine.fault`'s `vm.TranslationFault` → `cpu.Fault` mapping (`internal/cpu/
  dispatch.go`) has no subcode to recover and reports `0x0001` (length/base violation)
  unconditionally for every `AccessViolation`, matching the more common of the two C
  call sites (and the same subcode `decode_opcode.c`/`decode_operand.c`'s own inlined
  physical-address-resolution faults already use).
- **Status**: deferred. A real fix means widening `vm.TranslationFault.Kind` with a
  third case (or a length-vs-protection sub-field) in Phase 02's territory, which is
  out of scope for a Phase 03 change; revisit alongside Phase 02 or in Phase 12.

## Open questions carried forward (not yet findings)

### [Phase 03] Register mode used where an address is required (`OP_AD`/`OP_VA`/`OP_BR` access)

`decode_operand.c` never rejects Register mode (`mode == 5`) for an operand whose
access is `OP_AD` (e.g. `MOVAL`/`PUSHAL`, `INSQUE`/`REMQUE`) — a register has no VAX
address, so this is plausibly a reserved-addressing-mode fault case the C source
simply doesn't check (unlike the short-literal-for-write check it does have). In this
Go port, `Operand{Kind: OperandRegister}` has no `Addr` field to be garbage in the way
the C source's stale, never-reset `VAXaddr[n]` would be for this case, so the
dangerous failure mode doesn't reproduce here either way — but whether decode should
actively fault this case (rather than silently letting it through as the C source
does) is a real open question, not resolved in this phase. Deferred to whichever of
Phase 04-07 implements the first `OP_AD`-consuming instruction family (Phase 04's
`MOVA`/`PUSHA`), with the VAX architecture manual in hand rather than guessed at here.

<!--
Entry template:

### [Phase NN] Short title

- **Where**: `reference/eVAX/eVAX/Source/.../file.c` (function/lines), ported to
  `internal/.../file.go`.
- **What**: what the C code does vs. what the ISA manual / the C code's own comments
  say it should do.
- **Status**: deferred (Go replicates C behavior as-is) | fixed in Go (rationale).
-->
