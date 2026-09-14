package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_mova.c and emul_push.c.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x9E, emulMova) // MOVAB
	reg(0x3E, emulMova) // MOVAW
	reg(0xDE, emulMova) // MOVAL
	reg(0x7E, emulMova) // MOVAQ

	reg(0x9F, emulPusha) // PUSHAB
	reg(0x3F, emulPusha) // PUSHAW
	reg(0xDF, emulPusha) // PUSHAL
	reg(0x7F, emulPusha) // PUSHAQ

	reg(0xDD, emulPushl) // PUSHL
}

// emulMova is MOVA{B,W,L,Q}: the destination longword operand is replaced by
// the source operand's VAX address, never dereferenced -- port of
// emul_mova.c's single shared handler (the C source uses one routine for all
// four sizes; there's no per-size behavior to distinguish in Go either,
// since the address computation doesn't depend on the addressed datum's
// size).
//
// Register mode for the source -- an address-only (OP_AD) operand, and a
// register has no VAX address -- already faults a reserved-addressing-mode
// exception at decode time (internal/cpu/operand.go), replacing
// emul_mova.c's own `is_register[0]` check; see docs/DEVIATIONS.md.
//
// MOVAx doesn't touch any condition codes: emul_mova.c never writes
// vax.pslw, matching the manual (MOVA isn't listed as affecting N/Z/V/C).
func emulMova(e *Engine, d *Decoded) error {
	return d.Operands[1].Store(e.cpu, e.mem, uint64(d.Operands[0].Addr))
}

// emulPusha is PUSHA{B,W,L,Q}: pushes the source operand's VAX address, never
// dereferenced, onto the stack -- port of emul_push.c's non-PUSHL path
// (`opcode->function != 0xDD`). Same register-mode fault and
// no-condition-codes behavior as MOVAx.
func emulPusha(e *Engine, d *Decoded) error {
	return push(e, d.Operands[0].Addr)
}

// emulPushl is PUSHL: pushes the source operand's *value* (not its address)
// onto the stack -- port of emul_push.c's PUSHL path. Unlike PUSHAx, the
// operand's access is a plain OP_RD, so a register source is legal (PUSHL R0
// pushes R0's value).
func emulPushl(e *Engine, d *Decoded) error {
	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	return push(e, uint32(v))
}

// push implements the "SP -= 4; store a longword at SP" pattern shared by
// PUSHAx/PUSHL (and, later, CALLS/CALLG/BSB/JSB -- see Phase 07).
func push(e *Engine, v uint32) error {
	sp := e.cpu.GPR(vax.SP) - 4
	e.cpu.SetGPR(vax.SP, sp)
	return e.mem.StoreLongword(e.cpu, sp, v)
}
