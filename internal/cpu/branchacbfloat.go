package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_branch.c's emul_acb ACBF case (function
// 0x4F), generalized to a shared F/D handler -- ACBD (0x6F) has no case in
// the C source's switch at all, and no init_emulators.c dispatch entry
// either (see docs/PHASE-05.md's design notes), so it's implemented fresh
// by direct analogy to ACBF.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x4F, emulAcbFloat(4)) // ACBF
	reg(0x6F, emulAcbFloat(8)) // ACBD
}

// emulAcbFloat builds ACBF/ACBD's Handler for the given operand size: the
// addend is added to the index, then the (updated) index is compared with
// the limit to decide whether to branch. N/Z come from the updated index, V
// is always false, and C is left unaffected (never written) -- matching
// emul_acb.c's ACBF case, which never assigns vax.pslw.c either, the same
// "C unaffected" shape as the integer ACB family (branchacb.go). V's
// constant false is likewise confirmed correct rather than a live bug
// despite emul_acb.c's own "vax.pslw.v = 0; /* Need overflow detection here
// */" comment on this case: fpuStore already faults synchronously on real
// overflow (the same reasoning as floatmath.go's setFloatPSL) before a
// V-write here would ever matter, so there's nothing left for "overflow
// detection" to do.
//
// The branch condition applies the same fix as the integer ACB family
// (branchacb.go/docs/DEVIATIONS.md): for a non-negative addend, the manual
// specifies branching when index <= limit, but emul_branch.c's ACBF case
// uses a strict `<` (the identical bug, replicated across all four integer
// ACB variants and here in the one floating variant the C source actually
// implements) -- fixed here directly rather than reproduced a fifth time.
func emulAcbFloat(size int) Handler {
	return func(e *Engine, d *Decoded) error {
		limit, err := loadFloat(e.cpu, e.mem, d.Operands[0])
		if err != nil {
			return err
		}

		addend, err := loadFloat(e.cpu, e.mem, d.Operands[1])
		if err != nil {
			return err
		}

		index, err := loadFloat(e.cpu, e.mem, d.Operands[2])
		if err != nil {
			return err
		}

		index += addend
		setFloatMovePSL(e.cpu, index) // N/Z/V, C left unaffected -- see below
		
		if err := storeFloat(e.cpu, e.mem, d.Operands[2], index); err != nil {
			return err
		}

		var branch bool
		if addend >= 0.0 {
			branch = index <= limit
		} else {
			branch = index >= limit
		}

		if branch {
			e.cpu.SetGPR(vax.PC, d.Operands[3].Addr)
		}
		
		return nil
	}
}
