package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_skpc.c's SKPC.

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x3B}), emulSkpc)
}

// emulSkpc is SKPC: the string at addr is scanned while its bytes equal ch.
// Z <- 1 if the string is exhausted without an inequality (including a
// zero-length string, which never enters the loop and so leaves the
// remaining length at its already-zero starting value), Z <- 0 if an
// unequal byte was found; N/V/C are always 0. R0/R1 report the remaining
// length (including the unequal byte) and its address when one was found,
// or 0 and the address one past the string's end when not.
func emulSkpc(e *Engine, d *Decoded) error {
	chv, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	
	ch := byte(chv)

	lenv, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length := uint16(lenv)
	addr := d.Operands[2].Addr

	for length != 0 {
		ch2, err := e.mem.LoadByte(e.cpu, addr)
		if err != nil {
			return err
		}

		if ch != ch2 {
			break
		}

		length--
		addr++
	}

	e.cpu.SetGPR(vax.R0, uint32(length))
	e.cpu.SetGPR(vax.R1, addr)

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(length == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}
