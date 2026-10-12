package cpu

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// LOCC (Locate Character) and SKPC (Skip Character).
//
//	LOCC char.rb, len.rw, addr.ab
//	SKPC char.rb, len.rw, addr.ab
//
// LOCC scans the len-byte string at addr for a byte equal to char; SKPC
// scans it for a byte that isn't char. R0 gets the bytes left, counting
// the one found, and R1 its address; when none is found (or the string
// is empty) R0 is 0 and R1 the address just past the string. Z is set
// exactly when none was found; N, V, and C are cleared.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x3A, emulLocc) // LOCC
	reg(0x3B, emulSkpc) // SKPC
}

// LocateCharacter is LOCC's (equal true) and SKPC's (equal false) scan of
// the length-byte string at addr: it stops at the first byte that is
// (LOCC) or isn't (SKPC) ch, and returns the bytes left, counting that
// one, or 0 when it reached the end. LIBRTL's LIB$LOCC and LIB$SKPC share
// it.
func LocateCharacter(cpu *registers.CPU, mem *vm.Memory, ch byte, length uint16, addr uint32, equal bool) (uint16, error) {
	for i := range length {
		b, err := mem.LoadByte(cpu, addr+uint32(i))
		if err != nil {
			return 0, err
		}

		if (b == ch) == equal {
			return length - i, nil
		}
	}

	return 0, nil
}

func emulLocc(e *Engine, d *Decoded) error { return locateCharacter(e, d, true) }
func emulSkpc(e *Engine, d *Decoded) error { return locateCharacter(e, d, false) }

func locateCharacter(e *Engine, d *Decoded, equal bool) error {
	ch, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	lenv, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length, addr := uint16(lenv), d.Operands[2].Addr

	left, err := LocateCharacter(e.cpu, e.mem, byte(ch), length, addr, equal)
	if err != nil {
		return err
	}

	e.cpu.SetGPR(registers.R0, uint32(left))
	e.cpu.SetGPR(registers.R1, addr+uint32(length-left))

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(left == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}
