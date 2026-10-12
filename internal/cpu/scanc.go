package cpu

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// SCANC (Scan Characters) and SPANC (Span Characters).
//
//	SCANC len.rw, addr.ab, tbladdr.ab, mask.rb
//	SPANC len.rw, addr.ab, tbladdr.ab, mask.rb
//
// Each byte of the len-byte string at addr indexes the 256-byte table at
// tbladdr, and the table's byte is ANDed with mask. SCANC stops at the
// first nonzero AND, SPANC at the first zero one. R0 gets the bytes left,
// counting the one it stopped at, and R1 that byte's address; when it
// reached the end (or the string is empty) R0 is 0 and R1 the address
// just past the string. R2 is 0 and R3 the table's address. Z is set
// exactly when it reached the end, as the manual's Condition Codes say
// (docs/DEVIATIONS.md has the polarity once ported backwards); N, V, and
// C are cleared. The length is unsigned, 0 to 65,535.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x2A, emulScanc) // SCANC
	reg(0x2B, emulSpanc) // SPANC
}

// ScanTable is SCANC's (scan true) and SPANC's (scan false) walk of the
// length-byte string at addr through the table at table under mask: the
// bytes left, counting the one it stopped at, or 0 when it reached the
// end. LIBRTL's LIB$SCANC and LIB$SPANC share it.
func ScanTable(cpu *registers.CPU, mem *vm.Memory, length uint16, addr, table uint32, mask byte, scan bool) (uint16, error) {
	for i := range length {
		ch, err := mem.LoadByte(cpu, addr+uint32(i))
		if err != nil {
			return 0, err
		}

		entry, err := mem.LoadByte(cpu, table+uint32(ch))
		if err != nil {
			return 0, err
		}

		if (entry&mask != 0) == scan {
			return length - i, nil
		}
	}

	return 0, nil
}

func emulScanc(e *Engine, d *Decoded) error { return scanTable(e, d, true) }
func emulSpanc(e *Engine, d *Decoded) error { return scanTable(e, d, false) }

func scanTable(e *Engine, d *Decoded, scan bool) error {
	lenv, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length, addr, table := uint16(lenv), d.Operands[1].Addr, d.Operands[2].Addr

	mask, err := d.Operands[3].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	left, err := ScanTable(e.cpu, e.mem, length, addr, table, byte(mask), scan)
	if err != nil {
		return err
	}

	e.cpu.SetGPR(registers.R0, uint32(left))
	e.cpu.SetGPR(registers.R1, addr+uint32(length-left))
	e.cpu.SetGPR(registers.R2, 0)
	e.cpu.SetGPR(registers.R3, table)

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(left == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}
