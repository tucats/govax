package cpu

import (
	"fmt"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
)

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x02, emulRei)  // REI
	reg(0x04, emulRet)  // RET
	reg(0xFA, emulCall) // CALLG
	reg(0xFB, emulCall) // CALLS
}

// emulCall is CALLS/CALLG's single shared handler.
//
// CALLG's arglist operand (operand 0, table-declared AccessAddress) accepts
// Register mode, using the register's own *value* as the arglist address.
// A Phase 04 change briefly made decode fault this generically for every
// OP_AD/OP_VA consumer, reasoning the manual's "arglist.ab" (access type A) notation
// forbids register mode and the C source's own workaround was therefore
// dead code; Phase 12's first real end-to-end run of kernel.asm (this
// project's own hand-written microkernel) found that its CHMK dispatcher's
// `callg ap, (r0)` genuinely depends on exactly this: a tail-call that
// reuses the caller's own AP register value as the new arglist address
// without redundantly rebuilding it in memory. Reverted per user direction
// (2026-09-15); see operand.go's doc comment and docs/DEVIATIONS.md.
func emulCall(e *Engine, d *Decoded) error {
	var newAP uint32

	calls := d.Opcode.Function == 0xFB
	if calls {
		count, err := d.Operands[0].Load(e.cpu, e.mem)
		if err != nil {
			return err
		}

		sp := e.cpu.GPR(registers.SP) - 4
		e.cpu.SetGPR(registers.SP, sp)

		if err := e.mem.StoreLongword(e.cpu, sp, uint32(count)); err != nil {
			return err
		}

		newAP = sp
	} else if d.Operands[0].Kind == OperandRegister {
		newAP = e.cpu.GPR(d.Operands[0].Reg)
	} else {
		newAP = d.Operands[0].Addr
	}

	newPC := d.Operands[1].Addr

	// Save the SP as it stood after any CALLS count push, then coerce it to be longword aligned.
	savedSP := e.cpu.GPR(registers.SP)
	e.cpu.SetGPR(registers.SP, savedSP&0xFFFFFFFC)

	return e.buildCallFrame(newAP, newPC, savedSP, e.cpu.GPR(registers.PC), e.cpu.GPR(registers.FP), calls)
}

// buildCallFrame does the shared frame-construction work behind CALLS/CALLG
// and Engine.CallEntry: reads the entry mask at newPC, checks its reserved
// bits (Note 1), saves R0-R11 per the mask, pushes the return context
// (returnPC/returnFP -- the live PC/FP for a real CALLS/CALLG, or
// SentinelReturn for a console-initiated call with no real caller frame to
// return to), applies the manual's PSL effects, and leaves FP/AP/PC pointing
// at the new frame/entry point. savedSP is the pre-alignment SP the caller
// captured (for the frame's spa bits); newAP/newPC are the resolved
// argument-pointer and entry-point values already computed by the caller.
func (e *Engine) buildCallFrame(newAP, newPC, savedSP, returnPC, returnFP uint32, calls bool) error {
	mask, err := e.mem.LoadWord(e.cpu, newPC)
	if err != nil {
		return err
	}

	newPC += 2

	// Note 1: a reserved operand fault occurs if bits 13:12 of the entry
	// mask are not zero. SP has already moved (the CALLS count push and the
	// caller's alignment), matching Note 2's "on a reserved operand fault,
	// condition codes are UNPREDICTABLE" allowance for partial side effects;
	// nothing else about the frame (registers, PC/FP/AP, the mask longword)
	// has been touched yet.
	if mask&0x3000 != 0 {
		return &Fault{Code: ExcReservedOp}
	}

	for n := 11; n >= 0; n-- {
		if mask&(1<<uint(n)) == 0 {
			continue
		}

		sp := e.cpu.GPR(registers.SP) - 4
		e.cpu.SetGPR(registers.SP, sp)

		if err := e.mem.StoreLongword(e.cpu, sp, e.cpu.GPR(registers.Reg(n))); err != nil {
			return err
		}
	}

	oldAP := e.cpu.GPR(registers.AP)

	if err := push(e, returnPC); err != nil {
		return err
	}

	if err := push(e, returnFP); err != nil {
		return err
	}

	if err := push(e, oldAP); err != nil {
		return err
	}

	// The manual's order: the condition codes are cleared, the PSW is
	// pushed (with T cleared: "the processor status word (PSW) in bits
	// 15:0 with T cleared are pushed on the stack"; the live T is
	// unaffected), and only then are the trap enables set from the entry
	// mask (IV from bit 14, DV from bit 15, FU cleared). So the frame keeps
	// the caller's trap enables, which RET gives back to the caller: VMS
	// 7.1's frames show it (testdata/tb70's part I, the caller's IV clear
	// in the frame of a routine whose mask sets it).
	psl := e.cpu.PSL()
	psl.SetNZVC(false, false, false, false)

	snapshotPSW := uint32(psl) & 0x0000FFFF &^ 0x0010

	psl.SetIV(mask&(1<<14) != 0)
	psl.SetDV(mask&(1<<15) != 0)
	psl.SetFU(false)
	e.cpu.SetPSL(psl)

	var calltypeBit uint32
	if calls {
		calltypeBit = 1
	}

	maskWord := (savedSP&0x3)<<30 | calltypeBit<<29 | (uint32(mask)&0x0FFF)<<16 | snapshotPSW
	if err := push(e, maskWord); err != nil {
		return err
	}

	if err := push(e, 0); err != nil {
		return err
	}

	e.cpu.SetGPR(registers.FP, e.cpu.GPR(registers.SP))
	e.cpu.SetGPR(registers.AP, newAP)
	e.cpu.SetGPR(registers.PC, newPC)

	return nil
}

// emulRet is RET: pops and unwinds the call frame CALLS/CALLG built.
func emulRet(e *Engine, d *Decoded) error {
	sp := e.cpu.GPR(registers.FP) + 4

	maskWord, err := e.mem.LoadLongword(e.cpu, sp)
	if err != nil {
		return err
	}

	sp += 4

	// Note 1: a reserved operand fault occurs if tmp1<15:8> is nonzero.
	// Checked before any other state changes, matching Note 2's "on a
	// reserved operand fault, condition codes are UNPREDICTABLE" allowance.
	if maskWord&0x0000FF00 != 0 {
		return &Fault{Code: ExcReservedOp}
	}

	ap, err := e.mem.LoadLongword(e.cpu, sp)
	if err != nil {
		return err
	}

	sp += 4

	fp, err := e.mem.LoadLongword(e.cpu, sp)
	if err != nil {
		return err
	}

	sp += 4

	pc, err := e.mem.LoadLongword(e.cpu, sp)
	if err != nil {
		return err
	}

	sp += 4
	mask := (maskWord >> 16) & 0x0FFF

	for n := 0; n <= 11; n++ {
		if mask&(1<<uint(n)) == 0 {
			continue
		}

		v, err := e.mem.LoadLongword(e.cpu, sp)
		if err != nil {
			return err
		}

		sp += 4

		e.cpu.SetGPR(registers.Reg(n), v)
	}

	spa := maskWord >> 30
	sp += spa

	if maskWord&(1<<29) != 0 { // CALLS
		// The argument count longword is popped, and "four times the
		// unsigned value of the low byte of this longword is added to SP"
		// (the VAX Architecture Reference Manual's RET): an argument list
		// holds at most 255 arguments, and the count's other three bytes
		// are reserved, so they don't move SP.
		count, err := e.mem.LoadLongword(e.cpu, sp)
		if err != nil {
			return err
		}

		sp += 4 + (count&0xFF)*4
	}

	psl := uint32(e.cpu.PSL())
	psl = (psl &^ 0x0000FFFF) | (maskWord & 0x0000FFFF)
	e.cpu.SetPSL(registers.PSL(psl))

	e.cpu.SetGPR(registers.AP, ap)
	e.cpu.SetGPR(registers.FP, fp)
	e.cpu.SetGPR(registers.PC, pc)
	e.cpu.SetGPR(registers.SP, sp)

	if fp == SentinelReturn && pc == SentinelReturn {
		return ErrConsoleCallReturned
	}

	return nil
}

// SentinelReturn is the magic PC/FP value the console CALL
// command calls FFFFDEAF ("if-def"): a return-frame value no real CALLS
// instruction can ever produce, used by Engine.CallEntry
// so a subsequent RET can be recognized as returning from a console-
// initiated call rather than to a real caller.
const SentinelReturn = 0xFFFFDEAF

// ErrConsoleCallReturned is returned by Step (via emulRet) when RET pops a
// frame built by Engine.CallEntry, signalling clean completion of that call.
var ErrConsoleCallReturned = vmserrors.New(vmserrors.VAX_CALLRET)

// CallEntry builds a CALLS-shaped procedure-call frame directly (bypassing
// instruction fetch/decode, the way console CALL hand-
// builds its own frame rather than executing a real CALLS instruction)
// to invoke entry with zero arguments -- the "caller" here is the console,
// which has no VAX PC/FP context of its own to save, so the frame's return
// PC/FP are SentinelReturn rather than live register values. A subsequent
// RET is caught by emulRet and reported via ErrConsoleCallReturned instead
// of resuming at that meaningless address.
//
// args is the console CALL command's optional "(args...)" list, pushed
// right-to-left exactly like a real CALLS instruction's own argument list,
// followed by the argument count, before the frame itself is built. Phase
// 13's own call site (RUN's IMAGE$INIT driver) always passes none.
func (e *Engine) CallEntry(entry uint32, args ...uint32) error {
	for i := len(args) - 1; i >= 0; i-- {
		sp := e.cpu.GPR(registers.SP) - 4
		e.cpu.SetGPR(registers.SP, sp)

		if err := e.mem.StoreLongword(e.cpu, sp, args[i]); err != nil {
			return err
		}
	}

	sp := e.cpu.GPR(registers.SP) - 4

	e.cpu.SetGPR(registers.SP, sp)

	if err := e.mem.StoreLongword(e.cpu, sp, uint32(len(args))); err != nil {
		return err
	}

	newAP := sp

	savedSP := e.cpu.GPR(registers.SP)
	e.cpu.SetGPR(registers.SP, savedSP&0xFFFFFFFC)

	return e.buildCallFrame(newAP, entry, savedSP, SentinelReturn, SentinelReturn, true)
}

// emulRei is REI: return from exception or interrupt, restoring the mode the
// machine was in before the exception/interrupt was taken.
func emulRei(e *Engine, d *Decoded) error {
	sp := e.cpu.GPR(registers.SP)

	newPC, err := e.mem.LoadLongword(e.cpu, sp)
	if err != nil {
		return err
	}

	sp += 4

	newPSL, err := e.mem.LoadLongword(e.cpu, sp)
	if err != nil {
		return err
	}

	sp += 4

	// The popped PSL must be one REI may restore from here; if it isn't,
	// REI is a reserved operand fault, and nothing has changed yet (SP
	// was only read from, so the fault leaves it where it was).
	oldPSL := e.cpu.PSL()
	if !reiAllows(oldPSL, registers.PSL(newPSL)) {
		return &Fault{Code: ExcReservedOp}
	}

	if oldPSL.IS() {
		e.cpu.SetPR(registers.ISP, sp)
	} else {
		e.cpu.SetPR(registers.PrivReg(oldPSL.CurMod()), sp)
	}

	// The instruction after an REI is traced if T was set when the REI
	// began (Step copied it into TP then) or TP is set in the restored
	// PSL (trace.go).
	restored := registers.PSL(newPSL)
	if oldPSL.TP() {
		restored.SetTP(true)
	}

	e.cpu.SetPSL(restored)
	e.cpu.SetPR(registers.IPL, e.cpu.PSL().IPL())

	// The new stack: the interrupt stack if the restored PSL is on it (an
	// REI from one interrupt level to a lower one), else the restored
	// mode's.
	if restored.IS() {
		e.cpu.SetGPR(registers.SP, e.cpu.PR(registers.ISP))
	} else {
		e.cpu.SetGPR(registers.SP, e.cpu.PR(registers.PrivReg(restored.CurMod())))
	}

	e.cpu.SetGPR(registers.PC, newPC)

	if newMode := e.cpu.PSL().CurMod(); oldPSL.CurMod() != newMode {
		e.mem.InvalidateProtection()

		if e.cpu.DebugEnabled(registers.DebugCHM) {
			fmt.Fprintf(e.cpu.DebugWriter(), "DEBUG(CHM): CHANGE MODE FROM %s TO %s AT %08X\n",
				accessModeNames[oldPSL.CurMod()], accessModeNames[newMode], e.instructionPC)
		}
	}

	return nil
}

var accessModeNames = [4]string{"KERNEL", "EXEC", "SUPER", "USER"}

// pslMBZ is the PSL's must-be-zero bits: 15:8 (the PSW's reserved byte),
// 21 (between IPL and the previous mode), and 29:28 (between FPD and TP).
const pslMBZ = 0x0000FF00 | 1<<21 | 3<<28

// reiAllows reports whether REI may restore next while the CPU's PSL is
// cur. REI is the only way back out of an exception or interrupt, and any
// code may execute it, user mode included, so the architecture has it
// check the PSL it pops: a PSL that would give the code more privilege
// than it has, or that is inconsistent in itself, is a reserved operand
// fault (VAX Architecture Reference Manual, 1987, REI's operation):
//
//   - IS set with IPL 0: the interrupt stack is only ever used above IPL 0.
//   - IPL above 0 outside kernel mode: only kernel code may raise IPL.
//     (With the rule above, this also refuses IS set outside kernel mode,
//     which the 1982 manual lists as a check of its own.)
//   - a previous mode more privileged than the current one: a mode is
//     only ever entered from one at least as privileged as it records.
//   - a must-be-zero bit set.
//   - a current mode more privileged than the CPU's now: REI can't move
//     inward (that takes CHMx, which goes only where the operating
//     system's vectors say).
//   - IS set when the CPU isn't on the interrupt stack now: getting onto
//     it takes an interrupt or an exception that chooses it.
//   - an IPL above the CPU's now: REI only lowers IPL or leaves it.
//   - CM set: compatibility mode (the PDP-11's instruction set) isn't
//     implemented, and a processor without it faults any PSL asking for it.
//
// Access modes are numbered from kernel (0) to user (3), so "more
// privileged" is "numerically lower".
func reiAllows(cur, next registers.PSL) bool {
	switch {
	case next.IS() && next.IPL() == 0,
		next.IPL() > 0 && next.CurMod() != registers.Kernel,
		next.PrvMod() < next.CurMod(),
		uint32(next)&pslMBZ != 0,
		next.CurMod() < cur.CurMod(),
		next.IS() && !cur.IS(),
		next.IPL() > cur.IPL(),
		next.CM():
		return false
	}

	return true
}
