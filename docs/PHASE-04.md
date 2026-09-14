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

## Open questions / notes

- None yet — straightforward port once Phase 03's dispatch mechanism is settled.

## Progress Log

_Not started._
