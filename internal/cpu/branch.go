package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_branch.c's simple branch-always/conditional
// branches and its generic RSB/BSB/JSB/BLBS/BLBC handler. ACB and CASE (also
// in emul_branch.c) are a separate, more involved sub-phase; see branchacb.go.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x11, emulBranchAlways) // BRB
	reg(0x17, emulBranchAlways) // JMP
	reg(0x31, emulBranchAlways) // BRW

	reg(0x12, emulBneq)
	reg(0x13, emulBeql)
	reg(0x14, emulBgtr)
	reg(0x15, emulBleq)
	reg(0x18, emulBgeq)
	reg(0x19, emulBlss)
	reg(0x1A, emulBgtru)
	reg(0x1B, emulBlequ)
	reg(0x1C, emulBvc)
	reg(0x1D, emulBvs)
	reg(0x1E, emulBgequ)
	reg(0x1F, emulBcs)

	reg(0x05, emulRsb)
	reg(0x10, emulBsb) // BSBB
	reg(0x30, emulBsb) // BSBW
	reg(0x16, emulJsb)
	reg(0xE8, emulBlbs)
	reg(0xE9, emulBlbc)
}

// emulBranchAlways is BRB/BRW/JMP: PC is unconditionally replaced by the
// already-decoded target operand (a branch displacement for BRB/BRW, a
// general address operand -- never a register, see docs/DEVIATIONS.md's
// OP_AD finding -- for JMP).
func emulBranchAlways(e *Engine, d *Decoded) error {
	e.cpu.SetGPR(vax.PC, d.Operands[0].Addr)

	return nil
}

// condBranch builds a Handler for one of the twelve simple conditional
// branches: PC is replaced by the target operand iff test(psl) is true,
// otherwise nothing happens -- matching emul_branch.c's BRANCH_HANDLER
// macro.
func condBranch(test func(vax.PSL) bool) Handler {
	return func(e *Engine, d *Decoded) error {
		if test(e.cpu.PSL()) {
			e.cpu.SetGPR(vax.PC, d.Operands[0].Addr)
		}

		return nil
	}
}

var (
	emulBneq  = condBranch(func(p vax.PSL) bool { return !p.Z() })
	emulBeql  = condBranch(func(p vax.PSL) bool { return p.Z() })
	emulBgtr  = condBranch(func(p vax.PSL) bool { return !(p.N() || p.Z()) })
	emulBleq  = condBranch(func(p vax.PSL) bool { return p.N() || p.Z() })
	emulBgeq  = condBranch(func(p vax.PSL) bool { return !p.N() })
	emulBlss  = condBranch(func(p vax.PSL) bool { return p.N() })
	emulBgtru = condBranch(func(p vax.PSL) bool { return !(p.C() || p.Z()) })
	emulBlequ = condBranch(func(p vax.PSL) bool { return p.C() || p.Z() })
	emulBvc   = condBranch(func(p vax.PSL) bool { return !p.V() })
	emulBvs   = condBranch(func(p vax.PSL) bool { return p.V() })
	emulBgequ = condBranch(func(p vax.PSL) bool { return !p.C() })
	emulBcs   = condBranch(func(p vax.PSL) bool { return p.C() })
)

// emulRsb is RSB: pop the return address off the stack into PC.
//
// emul_branch.c's RSB case calls load_register(15, vax.SP, 4) (load the
// longword at vax.SP into register 15/PC) without checking its return
// value at all, so a faulting pop would silently continue in the C source
// rather than raising an access violation. Go's explicit error return makes
// that mistake impossible to reproduce by accident, so this isn't
// specifically replicated.
func emulRsb(e *Engine, d *Decoded) error {
	sp := e.cpu.GPR(vax.SP)
	
	ret, err := e.mem.LoadLongword(e.cpu, sp)
	if err != nil {
		return err
	}

	e.cpu.SetGPR(vax.SP, sp+4)
	e.cpu.SetGPR(vax.PC, ret)

	return nil
}

// emulBsb is BSBB/BSBW: push the return address (PC, already advanced past
// this instruction by Engine.Step before the handler runs), then jump to
// the branch-displacement target.
func emulBsb(e *Engine, d *Decoded) error {
	if err := push(e, e.cpu.GPR(vax.PC)); err != nil {
		return err
	}

	e.cpu.SetGPR(vax.PC, d.Operands[0].Addr)

	return nil
}

// emulJsb is JSB: identical to BSB (push the return address, then jump) --
// the only difference from BSBB/BSBW is the addressing used to reach the
// target, which decode has already resolved into the same Operand.Addr
// field either way.
func emulJsb(e *Engine, d *Decoded) error {
	return emulBsb(e, d)
}

// emulBlbs is BLBS: branch to operand 1's target iff operand 0's low bit is
// set.
func emulBlbs(e *Engine, d *Decoded) error {
	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	if v&1 != 0 {
		e.cpu.SetGPR(vax.PC, d.Operands[1].Addr)
	}

	return nil
}

// emulBlbc is BLBC: branch to operand 1's target iff operand 0's low bit is
// clear.
func emulBlbc(e *Engine, d *Decoded) error {
	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	if v&1 == 0 {
		e.cpu.SetGPR(vax.PC, d.Operands[1].Addr)
	}
	
	return nil
}
