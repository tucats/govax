package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_locc.c's LOCC.

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x3A}), emulLocc)
}

// emulLocc is LOCC: the string at addr is scanned for a byte equal to
// match. Z <- 0 if found, Z <- 1 if the string is exhausted without a match
// (including a zero-length string, which never enters the loop and so
// falls straight to the not-found case); N/V/C are always 0. R0/R1 report
// the remaining length (including the matched byte) and its address when
// found, or 0 and the address one past the string's end when not.
func emulLocc(e *Engine, d *Decoded) error {
	matchv, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	match := byte(matchv)

	lenv, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length := uint16(lenv)
	addr := d.Operands[2].Addr

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetV(false)
	psl.SetC(false)

	for length != 0 {
		ch, err := e.mem.LoadByte(e.cpu, addr)
		if err != nil {
			return err
		}

		if ch == match {
			e.cpu.SetGPR(vax.R0, uint32(length))
			e.cpu.SetGPR(vax.R1, addr)
			psl.SetZ(false)
			e.cpu.SetPSL(psl)

			return nil
		}

		addr++
		length--
	}

	e.cpu.SetGPR(vax.R0, 0)
	e.cpu.SetGPR(vax.R1, addr)
	psl.SetZ(true)
	e.cpu.SetPSL(psl)
	
	return nil
}
