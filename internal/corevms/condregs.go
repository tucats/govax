package corevms

import (
	"errors"

	"github.com/tucats/govax/internal/registers"
)

// ConditionRegisters are the registers as they were when an exception
// happened, for a routine called from its condition handler: LIB$FIXUP_FLT,
// which has to read and change an operand of the instruction that faulted.
//
// While a handler runs the registers are in three places. R0 and R1 are
// in the mechanism array, from which a continue restores them. R2 to R11
// are wherever the handler, and what it called, left them: a procedure's
// entry mask saves a register in its call frame before using it, so a
// register is in the frame nearest the dispatcher that saved it, or if none
// did, still in the register. FP and SP are the dispatch's, and AP was saved
// by the handler's call.
type ConditionRegisters struct {
	env  *Environment
	d    *conditionDispatch
	ap   uint32
	save [12]uint32 // where a frame saved R0 to R11, or 0
}

// ConditionRegisters finds the registers of the hardware exception whose
// signal array is at sig, for the handler now running. ok is false if sig
// isn't such an exception's: a condition LIB$SIGNAL signaled has no
// faulting instruction to look at.
func (env *Environment) ConditionRegisters(sig uint32) (regs *ConditionRegisters, ok bool) {
	var d *conditionDispatch

	for _, c := range env.Process.conditions {
		if c.sig == sig && c.calling && c.kind == kindException {
			d = c
		}
	}

	if d == nil {
		return nil, false
	}

	regs = &ConditionRegisters{env: env, d: d}

	// From the running stub's frame out to the handler's, the frame the
	// dispatcher called; an outer frame's save is the older value.
	frame := env.cpu.GPR(registers.FP)

	for range maxHandlerFrames {
		maskPSW, err1 := env.mem.LoadLongword(env.cpu, frame+frameMaskPSW)
		savedPC, err2 := env.mem.LoadLongword(env.cpu, frame+frameSavedPC)
		ap, err3 := env.mem.LoadLongword(env.cpu, frame+frameSavedAP)
		caller, err4 := env.mem.LoadLongword(env.cpu, frame+frameSavedFP)

		if errors.Join(err1, err2, err3, err4) != nil {
			return nil, false
		}

		at := frame + frameSavedRegs

		for n := range regs.save {
			if maskPSW>>(16+n)&1 != 0 {
				regs.save[n] = at
				at += 4
			}
		}

		if savedPC == srchandlerAddr {
			regs.ap = ap

			return regs, true
		}

		frame = caller
	}

	return nil, false
}

// maxHandlerFrames bounds the walk from a running stub out to its handler's
// frame, in case the frames have been damaged.
const maxHandlerFrames = 10000

// Get returns register n, 0 to 14 (PC is the signal array's).
func (r *ConditionRegisters) Get(n int) (uint32, error) {
	switch {
	case n <= 1:
		return r.env.mem.LoadLongword(r.env.cpu, r.d.mech+12+4*uint32(n))
	case n <= 11:
		if at := r.save[n]; at != 0 {
			return r.env.mem.LoadLongword(r.env.cpu, at)
		}

		return r.env.cpu.GPR(registers.Reg(n)), nil
	case n == int(registers.AP):
		return r.ap, nil
	case n == int(registers.FP):
		return r.d.resumeFP, nil
	}

	return r.d.resumeSP, nil
}

// Set changes register n, 0 to 11, so that the program has the new value
// when the handler continues it.
func (r *ConditionRegisters) Set(n int, v uint32) error {
	switch {
	case n <= 1:
		return r.env.mem.StoreLongword(r.env.cpu, r.d.mech+12+4*uint32(n), v)
	case r.save[n] != 0:
		return r.env.mem.StoreLongword(r.env.cpu, r.save[n], v)
	}

	r.env.cpu.SetGPR(registers.Reg(n), v)

	return nil
}
