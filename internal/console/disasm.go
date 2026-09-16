package console

import (
	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/vmserrors"
)

// memByteReader adapts a live vax.CPU + vm.Memory pair to asm.ByteReader,
// so internal/asm's disassembler (Phase 11) can read instruction bytes
// straight out of the running VAX's memory. A translation/access fault
// reads back as 0 rather than aborting the whole disassembly — matching
// EXAMINE's per-unit error handling being about that one memory access,
// not the disassembler's own multi-byte instruction-length bookkeeping,
// which would otherwise be left inconsistent partway through decoding one
// instruction.
type memByteReader struct {
	c *Console
}

func (r memByteReader) ByteAt(addr uint32) byte {
	b, err := r.c.Mem.LoadByte(r.c.CPU, addr)
	if err != nil {
		return 0
	}

	return b
}

// Disassemble implements DISASSEMBLE/DISA: decodes and prints instructions
// from start through end (at least one, starting at start, even if end <
// start), matching console_disasm.c's own address-range loop. Unlike the
// reference tool's disasm_operand.c, this doesn't substitute a matching
// label's name for a raw hex address, or append a branch-destination
// comment — internal/asm's Disassemble deliberately leaves those out (see
// its own doc comment); a caller wanting them can post-process
// Decoded.Operands against d.Console.Symbols itself.
func (c *Console) Disassemble(start, end uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if end < start {
		end = start
	}

	r := memByteReader{c: c}
	for pc := start; pc <= end; {
		dec, err := c.decodeInstruction(r, pc)
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_DISASM, err, pc)
		}

		c.Printf("%08X: %s\n", pc, dec.String())
		pc += dec.Length
	}

	return nil
}

// decodeInstruction wraps asm.Disassemble with entry-mask detection: if pc
// is a symbol's .ENTRY address (Console.Symbols' IsEntry, merged from
// internal/asm's own SymEntry flag — see asm.go's Assemble), the word there
// is a register-save mask, not an instruction, and is decoded as one —
// matching decode_opcode.c's combined execute/disassemble entry point,
// which scans the symbol table by PC for exactly this reason. Without this,
// a mask word like hello.asm's ".entry main, ^m<>" either misdecodes as a
// bogus opcode or, worse, as some unrelated real instruction.
func (c *Console) decodeInstruction(r asm.ByteReader, pc uint32) (asm.Decoded, error) {
	if name, ok := c.Symbols.EntryAt(pc); ok {
		mask := uint16(r.ByteAt(pc)) | uint16(r.ByteAt(pc+1))<<8

		return asm.Decoded{
			Mnemonic: ".ENTRY",
			Operands: []string{name, asm.FormatMask(mask)},
			Length:   2,
		}, nil
	}

	return asm.Disassemble(r, pc)
}
