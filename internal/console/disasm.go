package console

import (
	"fmt"
	"strings"

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

// Disassemble implements DISASSEMBLE/NOSYMBOLIC, the console's own
// layout: DisassembleWith with no options.
func (c *Console) Disassemble(start, end uint32) error {
	return c.DisassembleWith(start, end, DisassembleOptions{})
}

// DisassembleOptions are DISASSEMBLE's qualifiers (docs/PHASE-41.md,
// subtask 11).
type DisassembleOptions struct {
	// Symbolic lays each instruction out as the VMS debugger's
	// EXAMINE/INSTRUCTION does, its location and operands named from the
	// loaded images' debug symbol tables and the console's symbols
	// (Decision 1). Without it, the layout is the console's own,
	// "ADDRESS: MNEMONIC operands", in the text the assembler reads back.
	Symbolic bool

	// Constants names a short literal or immediate that is exactly one
	// constant's value in the module holding the instruction (Decision
	// 5); Shareable shows a G^ reference to a shareable image by the
	// routine it calls (subtask 10). Neither is the debugger's display,
	// so both are off unless asked for.
	Constants bool
	Shareable bool

	// StartLine says the range's start was typed as a line (%LINE 85),
	// so its first instruction is named by the line, as the debugger
	// names it, even where a label is there too.
	StartLine bool
}

// DisassembleWith implements DISASSEMBLE: decodes and prints instructions
// from start through end (at least one, starting at start, even if end <
// start), matching console_disasm.c's own address-range loop, laid out
// as opts say.
func (c *Console) DisassembleWith(start, end uint32, opts DisassembleOptions) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if end < start {
		end = start
	}

	r := memByteReader{c: c}
	names := consoleSymbolizer{c: c, radix: c.symbolRadix()}
	format := c.formatOptions(opts, names)

	for pc := start; pc <= end; {
		if !opts.Symbolic {
			dec, err := c.decodeInstruction(r, pc)
			if err != nil {
				return vmserrors.Wrap(vmserrors.CLI_DISASM, err, pc)
			}

			c.Printf("%08X: %s\n", pc, dec.Format(format))
			pc += dec.Length

			continue
		}

		dec, err := c.decodeAt(r, pc)
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_DISASM, err, pc)
		}

		loc, ok := "", false
		if opts.StartLine && pc == start {
			loc, ok = names.lineName(pc)
		}

		if !ok {
			loc, ok = names.Symbolize(pc)
		}

		if !ok {
			loc = fmt.Sprintf("%08X", pc)
		}

		c.Printf("%s\n", debuggerLine(loc, dec.Format(format)))
		pc += dec.Length

		if strings.HasPrefix(dec.Mnemonic, "CASE") {
			pc = c.printCaseTable(r, dec, pc, names)
		}
	}

	return nil
}

// formatOptions are the disassembler's options for opts: the debugger's
// style and names for DISASSEMBLE/SYMBOLIC, and the constant and fixup
// cell namers when asked for.
func (c *Console) formatOptions(opts DisassembleOptions, names consoleSymbolizer) disasm.Options {
	var format disasm.Options

	if opts.Symbolic {
		format.Style = disasm.StyleDebugger
		format.Symbolizer = names
	}

	if opts.Constants {
		format.Constants = imageConstants{c: c}
	}

	if opts.Shareable {
		format.Cells = c
	}

	return format
}

// symbolRadix is the radix a symbolic name's offsets are written in: the
// console's radix when it's decimal, as the debugger's SET RADIX DECIMAL
// makes them (GLIMIT+589), and hexadecimal otherwise.
func (c *Console) symbolRadix() int {
	if c.Radix == 10 {
		return 10
	}

	return 16
}

// debuggerLine lays out one EXAMINE/INSTRUCTION line as the debugger
// does: the location and a colon, then spaces to the next multiple of 8
// columns (a full 8 when the colon ends on one), then the instruction.
func debuggerLine(loc, text string) string {
	col := len(loc) + 1

	return loc + ":" + strings.Repeat(" ", 8-col%8) + text
}

// maxCaseEntries bounds the case table printed after a CASE instruction,
// whose limit could be as large as a longword: a table that long can't be
// real code (govax's choice).
const maxCaseEntries = 1024

// printCaseTable prints the displacement table that follows a CASEB,
// CASEW, or CASEL at table, as the debugger does: one line per entry, 16
// spaces and the destination (DBGDIS\START\%LINE 100), and returns the
// address after it. The table has limit+1 word entries; a limit that
// isn't a literal or immediate can't be known, so nothing is printed and
// the words that follow are decoded as instructions.
func (c *Console) printCaseTable(r disasm.ByteReader, dec disasm.Decoded, table uint32, names consoleSymbolizer) uint32 {
	if len(dec.Operands) < 3 {
		return table
	}

	limit := dec.Operands[2]
	if limit.Mode != disasm.ModeLiteral && limit.Mode != disasm.ModeImmediate {
		return table
	}

	count := uint64(limit.Value) + 1
	if count > maxCaseEntries {
		count = maxCaseEntries
	}

	pc := table

	for range count {
		disp := int16(uint16(r.ByteAt(pc)) | uint16(r.ByteAt(pc+1))<<8)
		dest := table + uint32(int32(disp))

		text, ok := names.Symbolize(dest)
		if !ok {
			text = fmt.Sprintf("%08X", dest)
		}

		c.Printf("%16s%s\n", "", text)
		pc += 2
	}

	return pc
}

// imageConstants is DISASSEMBLE/CONSTANTS's disasm.ConstantNamer: a
// constant of the module, in the loaded images' debug symbol tables, that
// holds the instruction.
type imageConstants struct {
	c *Console
}

// Constant implements disasm.ConstantNamer.
func (n imageConstants) Constant(pc, value uint32) (string, bool) {
	if p := n.c.debugImageAt(pc); p != nil {
		return p.Constant(pc, value)
	}

	return "", false
}

// decodeInstruction wraps disasm.Disassemble with entry-mask detection: if pc
// is a routine's entry (see entryAt), the word there is a register-save
// mask, not an instruction, and is decoded as one -- matching
// decode_opcode.c's combined execute/disassemble entry point, which scans
// the symbol table by PC for exactly this reason. Without this, a mask
// word like hello.asm's ".entry main, ^m<>" either misdecodes as a bogus
// opcode or, worse, as some unrelated real instruction.
//
// A CALLS or CALLG to an absolute address that is an entry point gets the
// routine's name, for the console's own layout; DISASSEMBLE/SYMBOLIC uses
// decodeAt, and names it as the debugger does.
func (c *Console) decodeInstruction(r disasm.ByteReader, pc uint32) (disasm.Decoded, error) {
	instr, err := c.decodeAt(r, pc)
	if instr.IsMask {
		return instr, err
	}

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

// decodeAt decodes the instruction at pc, or the register-save mask there
// when pc is a routine's entry (entryAt), with no operand named.
func (c *Console) decodeAt(r disasm.ByteReader, pc uint32) (disasm.Decoded, error) {
	if name, ok := c.entryAt(pc); ok {
		return disasm.EntryMask(r, pc, name), nil
	}

	return disasm.Disassemble(r, pc)
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
