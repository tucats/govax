// Package disasm decodes VAX instructions back into text: the
// disassembler behind the console's DISASSEMBLE command, its instruction
// trace, and STEP.
//
// A VAX instruction is an opcode (one byte, or two for the extended
// opcodes FD to FF) followed by its operand specifiers. Each specifier
// starts with a byte whose high four bits are an addressing mode and
// whose low four bits are a register (R0 to R11, AP, FP, SP, PC), and
// may be followed by a displacement, an address, or an immediate value.
// Disassemble reads them with internal/cpu's instruction table, the same
// table the CPU executes from and internal/asm assembles from, so the
// three always agree on what an opcode's operands are.
//
// The text Disassemble produces is what internal/asm's Assemble reads
// back in, so assembling, disassembling, and reassembling gives the same
// bytes.
//
// It began in internal/asm (docs/PHASE-11.md) and moved here in Phase 41
// (docs/PHASE-41.md), so other packages (the console, and later a
// debugger) can use it without the assembler.
package disasm
