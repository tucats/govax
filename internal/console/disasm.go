package console

import (
	"fmt"

	"github.com/tucats/govax/internal/asm"
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
		dec, err := asm.Disassemble(r, pc)
		if err != nil {
			return fmt.Errorf("console: disassemble at %08X: %w", pc, err)
		}

		c.Printf("%08X: %s\n", pc, dec.String())
		pc += dec.Length
	}

	return nil
}
