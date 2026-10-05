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
// Decoding and rendering are separate steps. Disassemble returns a
// Decoded whose Operands keep each specifier's parts (operand.go): its
// mode, registers, displacement, literal value, and, where it's known
// without running the program, the address it refers to (Target). The
// rendering is done afterward:
//
//   - Decoded.String (format.go) gives the text internal/asm's Assemble
//     reads back in, so assembling, disassembling, and reassembling gives
//     the same bytes.
//   - Decoded.Format (symbolic.go) renders it in a Style: the assembler's
//     text, or the VMS debugger's EXAMINE/INSTRUCTION text
//     (StyleDebugger). Options can name what the numbers mean: a
//     Symbolizer turns each Target into a name (DBGDIS\COUNT, START+2),
//     a ConstantNamer names literals, and a CellNamer names a shareable
//     image's routine called through a G^ fixup cell. The disassembler
//     itself knows no symbols; internal/dbgsym and the console supply
//     them.
//
// It began in internal/asm (docs/PHASE-11.md) and moved here in Phase 41
// (docs/PHASE-41.md), so other packages (the console, and later a
// debugger) can use it without the assembler.
package disasm
