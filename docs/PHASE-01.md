# Phase 01: CPU hardware definition

## Goal

Establish the Go equivalent of the C emulator's core machine state — general and
privileged registers, the Processor Status Longword (PSL), and condition codes — as an
instantiated struct with a constructor, matching the "instantiated struct, passed
explicitly" state model locked in during planning (see `docs/PLAN.md`). No instruction
decode/execute yet; this phase is pure data-structure definition plus register
read/write primitives.

## Scope / C source mapping

- `reference/eVAX/eVAX/Headers/vax.h` — `struct VAX`, the single global machine-state
  object in C. Identify which fields belong to "core hardware state" (registers, PSL,
  condition codes) vs. later phases' concerns (VM regions belong to Phase 02;
  fault/interrupt queues to Phase 03; console- and assembler-specific sub-state to
  Phases 08/11) and only port the former here.
- `reference/eVAX/eVAX/Headers/arch.h` — per-platform macros and the `LONGWORD`/
  `ULONGWORD`/`QUADWORD` typedefs. In Go these become plain fixed-width types
  (`int32`/`uint32`/`int64`) — no macro/typedef indirection needed, and no risk of the
  32-vs-64-bit bug `AUDIT.md` documents, since Go's sized integer types make the width
  explicit at every use site.
- `reference/eVAX/eVAX/Source/CPU/registers.c` — register access helpers.

## Deliverables

- An instantiated `vax.Machine` (or similarly named) struct in `internal/vax/` with:
  general registers, PSL fields/condition codes, a constructor (`New()`), and
  register read/write methods.
- Unit tests covering register read/write and PSL/condition-code bit manipulation in
  isolation (no memory or instruction execution involved yet).

## Open questions / notes

- Decide the exact split between `internal/vax` (this phase) and `internal/vm` (Phase
  02) once `struct VAX`'s fields are fully inventoried — some fields may not cleanly
  separate.
- Confirm naming: `Machine` vs `CPU` vs `VAX` as the struct name, and whether privileged
  registers get their own sub-struct.

## Progress Log

_Not started._
