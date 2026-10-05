package console

import (
	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/vmserrors"
)

// memByteReader adapts a live vax.CPU + vm.Memory pair to disasm.ByteReader,
// so the disassembler (Phase 11's, which Phase 41 moved from internal/asm
// to internal/disasm) can read instruction bytes straight out of the
// running VAX's memory. A translation/access fault
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
// comment: disasm.Disassemble leaves naming addresses to its caller,
// which sets an operand's Symbol (see decodeInstruction).
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

// decodeInstruction wraps disasm.Disassemble with entry-mask detection: if pc
// is a routine's entry (see entryAt), the word there is a register-save
// mask, not an instruction, and is decoded as one -- matching
// decode_opcode.c's combined execute/disassemble entry point, which scans
// the symbol table by PC for exactly this reason. Without this, a mask
// word like hello.asm's ".entry main, ^m<>" either misdecodes as a bogus
// opcode or, worse, as some unrelated real instruction.
func (c *Console) decodeInstruction(r disasm.ByteReader, pc uint32) (disasm.Decoded, error) {
	if name, ok := c.entryAt(pc); ok {
		return disasm.EntryMask(r, pc, name), nil
	}

	instr, err := disasm.Disassemble(r, pc)

	// A CALLS or CALLG to an absolute address that is an entry point
	// shows the routine's name.
	if instr.Mnemonic == "CALLS" || instr.Mnemonic == "CALLG" {
		if len(instr.Operands) > 1 && instr.Operands[1].Mode == disasm.ModeAbsolute {
			if name, ok := c.entryAt(instr.Operands[1].Target); ok {
				instr.Operands[1].Symbol = name
			}
		}
	}

	return instr, err
}

// entryAt reports the name of the CALLS/CALLG routine whose entry (its
// register-save mask) is at pc: a console symbol flagged as an entry
// point (the ASM command's .ENTRY symbols, or SET/ENTRY's), else a
// routine in a loaded image's debug symbol table (Phase 41). The DST's
// routines are how the mask of a real VMS image's routine is found: RUN
// puts none of an image's names in the console's table. Both images
// linked /DEBUG and those linked with only traceback name their routines
// there. A JSB routine (dbgsym.Routine.NoCall) has no mask, so it isn't
// one.
func (c *Console) entryAt(pc uint32) (string, bool) {
	if name, ok := c.Symbols.EntryAt(pc); ok {
		return name, true
	}

	for _, icb := range c.ICBList {
		if icb.Debug == nil {
			continue
		}

		if r, _, ok := icb.Debug.RoutineAt(pc); ok && r.Address == pc && !r.NoCall {
			return r.Name, true
		}
	}

	return "", false
}
