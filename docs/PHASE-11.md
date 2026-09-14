# Phase 11: Assembler / disassembler

## Goal

Port the MACRO-32-ish assembler/disassembler shared by the console's inline ASM/DISASM
commands and used generally for operand encoding — this unlocks assembling
`testdata/asm/*.asm` fixtures directly in Go rather than depending on pre-built
`.exe`/ROM files for new test cases.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/Assembler/asm.c`, `asm_expr.c`, `asm_label.c`,
  `asm_opcode.c`, `asm_operand.c`, `asm_pseudo.c`, `asm_register.c`, `asm_symbols.c`,
  `asm_value.c` — parsing/assembly.
- `reference/eVAX/eVAX/Source/Assembler/disasm_operand.c` — disassembly/formatting for
  display.
- `reference/eVAX/eVAX/Source/Initialization/init_symbols.c` — symbol table seeding at
  startup.

## Deliverables

- `internal/asm` package with `Assemble(source string) ([]byte, error)`-shaped API (or
  similar) reusing the opcode tables from `internal/cpu` (Phase 03) rather than
  duplicating them, plus a disassembler for console DISASM support.
- Tests: assemble every fixture in `testdata/asm/` and confirm the output matches
  expected encodings (cross-check smaller ones like `hello.asm`, `movc3.asm`,
  `insv.asm` by hand against the ISA manual; use the larger ones — `kernel.asm`,
  `forth.asm`, `bench.asm` — as broad regression coverage). Round-trip test:
  disassemble output should re-assemble to the same bytes where unambiguous.

## Open questions / notes

- `testdata/asm/ssdef.asm` (26KB) looks like a VMS system-service-definitions include
  file rather than a standalone program — confirm its role (likely a `.INCLUDE`
  target for other fixtures like `kernel.asm`) before writing a test harness around it.

## Progress Log

_Not started._
