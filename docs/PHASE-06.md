# Phase 06: String, bitfield & queue instructions

## Goal

Port the VAX's character-string, bitfield, and CRC instruction families — a
self-contained, well-testable group.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/emul_bitfield.c` — bitfield instructions
  (EXTV/INSV/etc.).
- `reference/eVAX/eVAX/Source/CPU/emul_cmpc.c`, `emul_locc.c`, `emul_matchc.c`,
  `emul_movc.c`, `emul_skpc.c` — character-string instructions.
- `reference/eVAX/eVAX/Source/CPU/emul_crc.c` — CRC instruction.

## Deliverables

- Instruction handlers with unit tests covering variable-length string operands,
  edge cases (zero-length strings, fill-byte behavior), and bitfield boundary
  conditions (field spanning byte/word boundaries).
- `testdata/asm/movc3.asm`, `insv.asm` in `testdata/asm/` are direct fixtures for this
  family and should be used once the assembler (Phase 11) or a hand-encoded byte
  sequence can exercise them.

## Open questions / notes

- None yet.

## Progress Log

_Not started._
