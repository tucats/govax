package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_bitfield.c's bit-branch instructions: BBS/BBC
// (emul_bb) and BBSS/BBCS/BBSC/BBCC/BBSSI/BBCCI (emul_bbstate). Field
// extraction and storage reuse getRegisterField/setRegisterField and
// getMemoryField/setMemoryField (bitfield.go) at size 1 rather than porting
// emul_bbstate's own separate byte-pointer-based bit access -- both define
// the identical "test/set/clear one bit at a bit position from a register or
// memory base" semantics, and the value-based helpers already used by
// EXTV/INSV/etc. have no pointer-endianness concern to inherit in the first
// place (see docs/PHASE-03.md's design notes on why operands are values, not
// pointers). This project does not emulate multiple processors, so the
// interlocked variants (BBSSI/BBCCI) behave identically to their
// non-interlocked counterparts, matching emul_bbstate's own comment to that
// effect.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0xE0, emulBb)      // BBS
	reg(0xE1, emulBb)      // BBC
	reg(0xE2, emulBbState) // BBSS
	reg(0xE3, emulBbState) // BBCS
	reg(0xE4, emulBbState) // BBSC
	reg(0xE5, emulBbState) // BBCC
	reg(0xE6, emulBbState) // BBSSI
	reg(0xE7, emulBbState) // BBCCI
}

// bitBranchPosition reads a bit-branch instruction's position operand,
// sign-extended -- the same position operand EXTV/CMPV/INSV/FFx use.
func bitBranchPosition(e *Engine, posOp Operand) (int32, error) {
	p, err := posOp.Load(e.cpu, e.mem)
	if err != nil {
		return 0, err
	}
	return int32(signExtend(p, posOp.Size)), nil
}

// emulBb is BBS/BBC: branches to the third operand's target iff the single
// bit at the first operand's position within the second (base) operand is
// set (BBS) or clear (BBC). Neither operand is modified, and no condition
// codes are affected -- port of emul_bitfield.c's emul_bb.
func emulBb(e *Engine, d *Decoded) error {
	position, err := bitBranchPosition(e, d.Operands[0])
	if err != nil {
		return err
	}
	bit, err := loadField(e, d.Operands[1], position, 1)
	if err != nil {
		return err
	}
	test := uint32(1)
	if d.Opcode.Function == 0xE1 { // BBC
		test = 0
	}
	if bit == test {
		e.cpu.SetGPR(vax.PC, d.Operands[2].Addr)
	}
	return nil
}

// emulBbState is BBSS/BBCS/BBSC/BBCC/BBSSI/BBCCI: the single bit at the
// first operand's position within the second (base) operand is tested
// against a per-opcode value, then unconditionally set or cleared to a
// (possibly different) per-opcode value; branches to the third operand's
// target iff the *original* bit matched the test. No condition codes are
// affected -- port of emul_bitfield.c's emul_bbstate, whose own comment
// derives the same test/set-value decode from the opcode's low two bits
// used here (BBSSI/BBCCI first normalized to their non-interlocked
// equivalents, since this project doesn't emulate multiple processors).
func emulBbState(e *Engine, d *Decoded) error {
	position, err := bitBranchPosition(e, d.Operands[0])
	if err != nil {
		return err
	}
	base := d.Operands[1]
	original, err := loadField(e, base, position, 1)
	if err != nil {
		return err
	}

	f := d.Opcode.Function
	if f == 0xE6 { // BBSSI -> BBSS
		f = 0xE2
	}
	if f == 0xE7 { // BBCCI -> BBCC
		f = 0xE5
	}
	test := uint32(1)
	if f&0x01 != 0 {
		test = 0
	}
	newBit := uint32(0)
	if f&0x02 != 0 {
		newBit = 1
	}

	if err := storeField(e, base, position, 1, newBit); err != nil {
		return err
	}
	if original == test {
		e.cpu.SetGPR(vax.PC, d.Operands[2].Addr)
	}
	return nil
}
