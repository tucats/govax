# Phase 03: Instruction decode engine

## Goal

Build the variable-length VAX instruction fetch/decode engine, operand resolution, and
the fetch-decode-execute loop — the scaffolding that Phases 04-07's instruction
families plug into. At the end of this phase the engine should be able to decode (not
yet execute) any instruction opcode and its operand specifiers.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/decode_opcode.c` — opcode fetch/dispatch.
- `reference/eVAX/eVAX/Source/CPU/decode_operand.c` — operand specifier parsing
  (addressing modes).
- `reference/eVAX/eVAX/Source/CPU/vax.c` — the main fetch-decode-execute loop.
- `reference/eVAX/eVAX/Source/CPU/interrupt.c`, `memory_io.c` — interrupt/fault queues
  and memory-mapped I/O hooks the loop interacts with.
- `reference/eVAX/eVAX/Headers/instruction_table.h` — opcode table structure.
- `reference/eVAX/eVAX/Source/Initialization/init_emulators.c` — builds the opcode
  table at startup; port as the Go equivalent's table construction (likely a static Go
  table/map rather than a runtime-built structure, since Go doesn't need C's
  init-time construction pattern).

## Deliverables

- An `internal/cpu` package with a decode step that can walk any VAX opcode + operand
  specifiers against `internal/vm` memory and `internal/vax.Machine` registers, and an
  execute dispatch mechanism ready for Phases 04-07 to register instruction handlers
  into.
- Unit tests: decode correctness for a representative opcode from each addressing mode
  and each opcode length/prefix pattern, cross-checked against
  `reference/eVAX/AUDIT.md` and the VAX ISA manual for any previously-fixed corner
  cases.

## Open questions / notes

- Decide the instruction-dispatch mechanism in Go: a big switch, a table of function
  values, or per-family sub-dispatch — likely informed by how Phases 04-07 end up
  grouping instructions.

## Progress Log

_Not started._
