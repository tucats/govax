package console

import (
	"fmt"

	"github.com/tucats/govax/internal/dbgsym"
)

// Where an instruction is, for the instruction trace, STEP, and SHOW
// CALLS (docs/PHASE-41.md, subtask 12). Inside an image with a debug
// symbol table, and with symbolic display on (the vax.disassemble.symbolic
// setting, as for DISASSEMBLE), a PC is shown as the VMS debugger shows
// it: DBGDIS\START\%LINE 43, FAILSUB\SUB2, TRACE+10. Anywhere else (the
// kernel, the shims, code the ASM command assembled) the console's own
// hexadecimal display is unchanged.

// debugImageAt returns the debug symbol table of the loaded image whose
// address range holds addr, or nil when addr is in no image or its image
// has none.
func (c *Console) debugImageAt(addr uint32) *dbgsym.Program {
	for _, icb := range c.ICBList {
		if icb.Debug != nil && addr >= imageLow(icb) && addr <= icb.End {
			return icb.Debug
		}
	}

	return nil
}

// symbolicLocation names pc as the debugger names a location: the most
// specific name its image's debug symbol table gives (a routine, a label,
// a line, or a routine plus an offset), else its address in 8 hex digits.
// ok is false when the console's display applies instead: symbolic display
// is off, or pc isn't in an image with a debug symbol table.
func (c *Console) symbolicLocation(pc uint32) (loc string, names consoleSymbolizer, ok bool) {
	if c.debugImageAt(pc) == nil || !symbolicDefault() {
		return "", consoleSymbolizer{}, false
	}

	names = consoleSymbolizer{c: c, radix: c.symbolRadix()}

	loc, found := names.Symbolize(pc)
	if !found {
		loc = fmt.Sprintf("%08X", pc)
	}

	return loc, names, true
}

// locationText is pc as STEP's "Stepped to" and a breakpoint's "Break at"
// show it: symbolicLocation's name where it applies (the debugger's
// "stepped to DBGDIS\START\%LINE 43"), else the address in 8 hex digits.
func (c *Console) locationText(pc uint32) string {
	if loc, _, ok := c.symbolicLocation(pc); ok {
		return loc
	}

	return fmt.Sprintf("%08X", pc)
}

// symbolicTraceLine is the instruction trace's text for the instruction
// at pc, after its "[mode SP] " prefix, where symbolicLocation applies:
// the location and the instruction as DISASSEMBLE/SYMBOLIC lays them out
// (FAILSUB\SUB2\%LINE 12:  MOVL     @#00000000,R0). ok is false where it
// doesn't.
func (c *Console) symbolicTraceLine(pc uint32) (text string, ok bool, err error) {
	loc, names, ok := c.symbolicLocation(pc)
	if !ok {
		return "", false, nil
	}

	dec, err := c.decodeAt(memByteReader{c: c}, pc)
	if err != nil {
		return "", true, err
	}

	format := c.formatOptions(DisassembleOptions{Symbolic: true}, names)

	return debuggerLine(loc, dec.Format(format)), true, nil
}
