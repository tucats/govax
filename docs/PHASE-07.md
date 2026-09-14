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

- None yet.

## Progress Log

_Not started._
