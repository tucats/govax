# Phase 05: Floating point

## Goal

Port F/D-floating conversion and arithmetic, mapped onto native Go `float64`.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/fpu.c` — F/D-floating ↔ native `double` conversion.
- `reference/eVAX/eVAX/Source/CPU/emul_float_math.c` — floating arithmetic and
  float↔integer conversion instructions (CVTFB/CVTFW/CVTFL/CVTRFL and D-floating
  counterparts).
- `reference/eVAX/eVAX/Headers/fpu.h`.

## Deliverables

- Floating instruction handlers in `internal/cpu`, with explicit attention to the
  overflow-bounds bug class `reference/eVAX/AUDIT.md` documents as fixed (N2: CVTFL/
  CVTFW had wrong integer-overflow bounds — Byte -128..127, Word -32768..32767, Long
  -2147483648..2147483647, per the VAX ISA manual §8.3 "Data Types" — the Go port
  should encode these as named constants, not re-derive them ad hoc).
- Table-driven tests including round-trip conversions and the exact boundary cases
  `AUDIT.md`'s N2 fix verification used (e.g. word bounds rejecting 40000 but accepting
  1000).

## Open questions / notes

- None yet.

## Progress Log

_Not started._
