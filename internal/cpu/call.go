package cpu

import (
	"errors"

	"github.com/tucats/govax/internal/vax"
)

// This is the Go port of emul_call.c: CALLS/CALLG procedure-call stack-frame
// construction, RET, and REI.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x02, emulRei)  // REI
	reg(0x04, emulRet)  // RET
	reg(0xFA, emulCall) // CALLG
	reg(0xFB, emulCall) // CALLS
}

// emulCall is CALLS/CALLG's single shared handler (mirroring emul_call.c's
// own emul_call, which dispatches on opcode->function internally rather than
// having two separate routines).
//
// CALLG's arglist operand (operand 0) is table-declared AccessAddress, so a
// register-mode arglist already faults a reserved-addressing-mode exception
// at decode time (internal/cpu/operand.go's global OP_AD-register-mode fix
// from Phase 03/04). emul_call.c's own `is_register[0]` branch -- which
// treats a register-mode arglist as legal and uses the register's *value* as
// the arglist address -- is therefore dead in this port: the manual's own
// "arglist.ab" (access type A) notation forbids register mode the same way
// MOVAx/PUSHAx's address operands do, so this isn't a behavior loss, just
// the same Phase 03/04 fix applying here too.
//
// Unlike emul_call.c, this port implements the manual's full PSW-effect
// description rather than replicating the C source's gap: the entry mask's
// reserved bits 13:12 are checked (reserved-operand fault if either is set,
// per Note 1), condition codes are explicitly cleared, IV/DV are set from
// entry-mask bits 14/15, and FU is cleared (T is left alone, "unaffected").
// These are load-bearing for any real MACRO-32 procedure-call convention, so
// -- per the user's direction (2026-09-14) -- fixed here rather than only
// logged in docs/DEVIATIONS.md.
func emulCall(e *Engine, d *Decoded) error {
	calls := d.Opcode.Function == 0xFB

	var newAP uint32
	if calls {
		count, err := d.Operands[0].Load(e.cpu, e.mem)
		if err != nil {
			return err
		}
		sp := e.cpu.GPR(vax.SP) - 4
		e.cpu.SetGPR(vax.SP, sp)
		if err := e.mem.StoreLongword(e.cpu, sp, uint32(count)); err != nil {
			return err
		}
		newAP = sp
	} else {
		newAP = d.Operands[0].Addr
	}

	newPC := d.Operands[1].Addr

	// Save the SP as it stood after any CALLS count push (matching
	// emul_call.c's ordering: savedSP is captured *after* that push, not
	// before), then coerce it to be longword aligned.
	savedSP := e.cpu.GPR(vax.SP)
	e.cpu.SetGPR(vax.SP, savedSP&0xFFFFFFFC)

	return e.buildCallFrame(newAP, newPC, savedSP, e.cpu.GPR(vax.PC), e.cpu.GPR(vax.FP), calls)
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
		sp := e.cpu.GPR(vax.SP) - 4
		e.cpu.SetGPR(vax.SP, sp)
		if err := e.mem.StoreLongword(e.cpu, sp, e.cpu.GPR(vax.Reg(n))); err != nil {
			return err
		}
	}

	oldAP := e.cpu.GPR(vax.AP)
	if err := push(e, returnPC); err != nil {
		return err
	}
	if err := push(e, returnFP); err != nil {
		return err
	}
	if err := push(e, oldAP); err != nil {
		return err
	}

	// Condition codes are cleared; IV/DV trap enables come from the entry
	// mask; FU is cleared; T is unaffected. Apply this to the live PSL
	// before snapshotting it into the frame, so the frame's saved psw field
	// reflects the same post-CALL state a later RET or exception handler
	// would see live.
	psl := e.cpu.PSL()
	psl.SetNZVC(false, false, false, false)
	psl.SetIV(mask&(1<<14) != 0)
	psl.SetDV(mask&(1<<15) != 0)
	psl.SetFU(false)
	e.cpu.SetPSL(psl)

	// The frame's own psw snapshot additionally has T cleared, per "the
	// processor status word (PSW) in bits 15:0 with T cleared are pushed on
	// the stack" -- distinct from the live PSW's T, which is unaffected.
	snapshotPSW := uint32(psl) & 0x0000FFFF &^ 0x0010

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

	e.cpu.SetGPR(vax.FP, e.cpu.GPR(vax.SP))
	e.cpu.SetGPR(vax.AP, newAP)
	e.cpu.SetGPR(vax.PC, newPC)
	return nil
}

// emulRet is RET: pops and unwinds the call frame CALLS/CALLG built, port of
// emul_call.c's emul_ret.
//
// Unlike emul_call.c, this port implements the manual's full description
// rather than replicating the C source's gap: "the PSW is replaced by bits
// 15:0 of the temporary" (the popped mask longword) -- a full 16-bit
// replacement, which subsumes the separately-stated "N <- tmp1<3>" etc.
// condition-code formula, since bits 3:0 of the PSW *are* N/Z/V/C -- and
// Note 1's reserved-operand fault if tmp1<15:8> is nonzero. emul_call.c's
// C source instead only ever restores bits 6:15 (FU, DV, and the otherwise-
// unused bits 8:15) and leaves bits 0:5 (N/Z/V/C, T, IV) at whatever the
// callee left them, with no reserved-operand check at all. Fixed here per
// the user's direction (2026-09-14) -- these are load-bearing for any real
// MACRO-32 procedure-return convention, not an edge case worth only
// documenting.
//
// If this frame was built by Engine.CallEntry (its saved PC and FP are both
// SentinelReturn -- a combination no real CALLS instruction can produce),
// this reports that completion via ErrConsoleCallReturned instead of
// resuming at the sentinel "address", matching emul_call.c's own
// vax.console.CALL_active/FFFFDEAF check in its emul_ret (see
// docs/PHASE-13.md). Unlike the C source, this skips the CALL_active guard
// flag: SentinelReturn is defined precisely because no legitimate program
// state can produce it (see the C source's own comment to that effect), so
// the flag only guards against an already-impossible coincidence.
func emulRet(e *Engine, d *Decoded) error {
	sp := e.cpu.GPR(vax.FP) + 4

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
		e.cpu.SetGPR(vax.Reg(n), v)
	}

	spa := maskWord >> 30
	sp += spa

	if maskWord&(1<<29) != 0 { // CALLS
		// The manual specifies only the popped count longword's low byte is
		// used ("four times the unsigned value of the low byte... is added
		// to SP"), but emul_call.c's emul_ret uses the full 32-bit value
		// unmasked. Replicated as-is -- see docs/DEVIATIONS.md.
		count, err := e.mem.LoadLongword(e.cpu, sp)
		if err != nil {
			return err
		}
		sp += 4 + count*4
	}

	psl := uint32(e.cpu.PSL())
	psl = (psl &^ 0x0000FFFF) | (maskWord & 0x0000FFFF)
	e.cpu.SetPSL(vax.PSL(psl))

	e.cpu.SetGPR(vax.AP, ap)
	e.cpu.SetGPR(vax.FP, fp)
	e.cpu.SetGPR(vax.PC, pc)
	e.cpu.SetGPR(vax.SP, sp)

	if fp == SentinelReturn && pc == SentinelReturn {
		return ErrConsoleCallReturned
	}
	return nil
}

// SentinelReturn is the magic PC/FP value console_exec.c's console CALL
// command calls FFFFDEAF ("if-def"): a return-frame value no real CALLS
// instruction can ever produce (see emul_call.c), used by Engine.CallEntry
// so a subsequent RET can be recognized as returning from a console-
// initiated call rather than to a real caller.
const SentinelReturn = 0xFFFFDEAF

// ErrConsoleCallReturned is returned by Step (via emulRet) when RET pops a
// frame built by Engine.CallEntry, signalling clean completion of that call.
var ErrConsoleCallReturned = errors.New("cpu: console call returned")

// CallEntry builds a CALLS-shaped procedure-call frame directly (bypassing
// instruction fetch/decode, the way console_exec.c's console_call hand-
// builds its own frame rather than executing a real CALLS instruction)
// to invoke entry with zero arguments -- the "caller" here is the console,
// which has no VAX PC/FP context of its own to save, so the frame's return
// PC/FP are SentinelReturn rather than live register values. A subsequent
// RET is caught by emulRet and reported via ErrConsoleCallReturned instead
// of resuming at that meaningless address.
//
// Only the zero-argument case is implemented: console_exec.c's own optional
// "(args...)" list is a console-CALL-command-only feature with no consumer
// in this port yet -- Phase 13's RUN command always invokes its IMAGE$INIT
// driver procedure with no arguments; the driver's own inner CALLS to
// LIB$INITIALIZE/the main image push their own arguments via ordinary
// PUSHL/CALLS instructions, already fully implemented by emulCall above.
func (e *Engine) CallEntry(entry uint32) error {
	sp := e.cpu.GPR(vax.SP) - 4
	e.cpu.SetGPR(vax.SP, sp)
	if err := e.mem.StoreLongword(e.cpu, sp, 0); err != nil { // argument count: 0
		return err
	}
	newAP := sp

	savedSP := e.cpu.GPR(vax.SP)
	e.cpu.SetGPR(vax.SP, savedSP&0xFFFFFFFC)

	return e.buildCallFrame(newAP, entry, savedSP, SentinelReturn, SentinelReturn, true)
}

// emulRei is REI: return from exception or interrupt, restoring the mode the
// machine was in before the exception/interrupt was taken. Port of
// emul_call.c's emul_rei, minus its AST-delivery and pending-software-
// interrupt (SISR) tail: both call interrupt(), the device-interrupt-queue
// admission routine that Phase 03 already deferred to Phase 09 (see
// docs/PHASE-03.md's design notes) -- REI's own PC/PSL/stack restore has no
// dependency on that machinery and is fully ported here.
//
// emul_rei.c restores SP from vax.preg[vax.pslw.cur_mod] unconditionally
// after installing the new PSL, never consulting the new PSL's IS bit the
// way setModeStack (internal/cpu/handlefault.go) does for the opposite
// (fault-delivery) direction -- so a REI whose popped PSL has IS set doesn't
// restore SP from ISP. Replicated as-is -- see docs/DEVIATIONS.md.
func emulRei(e *Engine, d *Decoded) error {
	sp := e.cpu.GPR(vax.SP)

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

	oldPSL := e.cpu.PSL()
	if oldPSL.IS() {
		e.cpu.SetPR(vax.ISP, sp)
	} else {
		e.cpu.SetPR(vax.PrivReg(oldPSL.CurMod()), sp)
	}

	e.cpu.SetPSL(vax.PSL(newPSL))
	e.cpu.SetPR(vax.IPL, e.cpu.PSL().IPL())

	e.cpu.SetGPR(vax.SP, e.cpu.PR(vax.PrivReg(e.cpu.PSL().CurMod())))
	e.cpu.SetGPR(vax.PC, newPC)
	return nil
}
