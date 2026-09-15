package cpu

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// stackNames matches interrupt.c:216's stackname[] -- the mode-stack names
// handle_fault's own DBG_EXCEPTIONS trace reports.
var stackNames = [4]string{"KSP", "ESP", "SSP", "USP"}

// ErrNoExceptionHandler is returned by Engine.HandleFault when the SCB
// vector for the faulting exception is 0xFFFFFFFF — this emulator's own
// convention (not a VAX architectural value) for "no handler installed,
// fall back to a built-in one," matching interrupt.c's handle_fault calling
// format_exception() in that case. No console exists yet to provide that
// fallback (Phase 08), so this is surfaced as an error instead; no state is
// changed when this is returned.
var ErrNoExceptionHandler = vmserrors.New(vmserrors.VAX_NOHANDLER)

// ErrUnhandledVector is returned by Engine.HandleFault when the SCB vector
// is zero. Unlike ErrNoExceptionHandler, the C source (and this port) still
// builds and pushes the full exception stack frame and sets PC to the
// (zero) vector before reporting this — matching interrupt.c's handle_fault
// returning VAX_UNHANDLED only after doing so.
var ErrUnhandledVector = vmserrors.New(vmserrors.VAX_UNHANDLED)

// HandleFault runs the VAX exception-delivery sequence for f: fetch the
// handler vector from the System Control Block (at SCBB + f.Code, read with
// virtual memory disabled — the SCB is addressed physically, matching
// handle_fault's temporary vax.MAPEN = 0), switch to the appropriate stack
// and access mode, push the old PSL/PC/signal-args, and set the CPU's PC to
// the vector.
//
// This is the Go port of interrupt.c's handle_fault and set_mode_stack,
// minus the parts that are console or device-interrupt concerns interleaved
// with them in the C source (breakpoint-on-fault checking in execute_vax,
// the DBG_EXCEPTIONS trace printf, format_exception's console fallback) —
// see docs/PHASE-03.md's design notes.
func (e *Engine) HandleFault(f *Fault) error {
	scbb := e.cpu.PR(vax.SCBB)

	savedMAPEN := e.cpu.PR(vax.MAPEN)
	e.cpu.SetPR(vax.MAPEN, 0)
	rawVector, err := e.mem.LoadLongword(e.cpu, scbb+uint32(f.Code))
	e.cpu.SetPR(vax.MAPEN, savedMAPEN)
	if err != nil {
		return err
	}

	if rawVector == 0xFFFFFFFF {
		return ErrNoExceptionHandler
	}

	stack := rawVector & 0x3
	vector := rawVector &^ 0x7

	newMode := vax.Kernel
	if f.Code >= ExcChangeModeK && f.Code <= ExcChangeModeU {
		newMode = vax.AccessMode((f.Code & 0x0F) >> 2)
	}

	// Capture the PSL to push onto the new stack before anything below
	// changes it (the access-mode switch, or the IPL update just after).
	savedPSL := e.cpu.PSL()

	// The IPL privileged register holds the new IPL to take, set by the
	// (not yet ported — Phase 09) device-interrupt admission logic; for a
	// pure exception/fault raised by decode or an instruction handler it's
	// already in sync with the current PSL's IPL, making this a no-op.
	psl := e.cpu.PSL()
	psl.SetIPL(e.cpu.PR(vax.IPL))
	e.cpu.SetPSL(psl)

	switch stack {
	case 0:
		e.setModeStack(newMode, false)
	case 1:
		e.setModeStack(vax.Kernel, true)
	}

	sp := e.cpu.GPR(vax.SP)

	if e.cpu.DebugEnabled(vax.DebugExceptions) {
		stackDesc := "ISP, VM=OFF"
		if stack == 0 {
			stackDesc = stackNames[newMode]
		}
		fmt.Fprintf(e.cpu.DebugWriter(), "DEBUG(EXCEPTION): TAKE, CODE=%04X  VECTOR=%08X  STACK=%08X [%s]\n",
			f.Code, vector, sp, stackDesc)
	}

	sp -= 4
	if err := e.mem.StoreLongword(e.cpu, sp, uint32(savedPSL)); err != nil {
		return err
	}
	sp -= 4
	if err := e.mem.StoreLongword(e.cpu, sp, e.instructionPC); err != nil {
		return err
	}
	// Only the first two signal arguments are ever pushed, matching
	// set_fault/handle_fault exactly — every exception this port raises
	// carries at most two (e.g. EXC_ACCVIO's faulting address and
	// read/write code), consistent with the VAX architecture reference's
	// exception parameter lists, so this isn't a truncation in practice.
	for i := 0; i < len(f.Args) && i < 2; i++ {
		sp -= 4
		if err := e.mem.StoreLongword(e.cpu, sp, f.Args[i]); err != nil {
			return err
		}
	}
	e.cpu.SetGPR(vax.SP, sp)
	e.cpu.SetGPR(vax.PC, vector)

	if vector == 0 {
		return ErrUnhandledVector
	}
	return nil
}

// SetModeStack is setModeStack exported for non-fault-handling console
// callers (see docs/PHASE-13.md's RUN command, which -- like the C source's
// own console_run.c -- must run image loading and fixups in kernel mode
// regardless of the mode the console happened to be in, then restore it
// afterward).
func (e *Engine) SetModeStack(newMode vax.AccessMode, interruptStack bool) {
	e.setModeStack(newMode, interruptStack)
}

// setModeStack switches to a new access mode's stack (interruptStack false,
// newMode the target mode) or to the interrupt stack (interruptStack true;
// newMode is unused in that case, matching set_mode_stack's mode-parameter
// value of 4 meaning "interrupt stack" rather than a real access mode).
func (e *Engine) setModeStack(newMode vax.AccessMode, interruptStack bool) {
	psl := e.cpu.PSL()
	curMod := psl.CurMod()

	if !interruptStack && curMod == newMode {
		return
	}

	psl.SetPrvMod(curMod)

	var newSP uint32
	if interruptStack {
		newSP = e.cpu.PR(vax.ISP)
	} else {
		newSP = e.cpu.PR(vax.PrivReg(newMode)) // KSP/ESP/SSP/USP == 0/1/2/3, same as AccessMode
	}

	if psl.IS() {
		e.cpu.SetPR(vax.ISP, e.cpu.GPR(vax.SP))
	} else {
		e.cpu.SetPR(vax.PrivReg(curMod), e.cpu.GPR(vax.SP))
	}

	if interruptStack {
		e.cpu.SetGPR(vax.SP, newSP)
		psl.SetIS(true)
		psl.SetCurMod(vax.Kernel)
		e.cpu.SetPR(vax.MAPEN, 0)
	} else {
		psl.SetCurMod(newMode)
		e.cpu.SetGPR(vax.SP, newSP)
		psl.SetIS(false)
		// set_mode_stack's own comment flags this write as uncertain
		// ("Not sure about this!!"); replicated as-is rather than
		// second-guessed. See docs/DEVIATIONS.md.
		e.cpu.SetPR(vax.MAPEN, 1)
	}

	e.cpu.SetPSL(psl)
}
