package console

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// This file is the Go port of interrupt.c's chf() (Condition Handling
// Facility frame search) and format_exception() -- see docs/PHASE-20.md.
// Both only ever run for a fault whose SCB vector is kernel.asm's own
// "console$handler" sentinel (0xFFFFFFFF): a real vector switches stack/
// mode and resumes at a real handler address inside Engine.HandleFault
// itself, with none of this involved. Unlike that case, chf/format_exception
// never touch the mode stack at all -- they operate directly on whatever
// stack/mode was live when the fault happened, matching the C source's own
// vax.SP-relative pushes with no set_mode_stack call anywhere in either
// routine.
//
// This lives in internal/console, not internal/cpu, because invoking a
// condition handler found on the frame chain means making a nested call and
// running it to completion -- exactly Console.Call's own job (built for
// Phase 13's LIB$INITIALIZE/main-image transfer, reused here for the same
// "call an entry point, run until it returns via RET" primitive), which
// internal/cpu deliberately doesn't provide on its own (see docs/PHASE-13.md's
// "Key finding").

// maxCHFDepth matches interrupt.c's MAX_CHF_DEPTH: the frame-chase bail-out
// so a corrupt or cyclic FP chain can't hang the search forever.
const maxCHFDepth = 4096

// handleConsoleFault is the Go port of format_exception(): try the
// Condition Handling Facility frame search first (matching format_exception's
// own `if (chf() == VAX_OK) return VAX_OK`), and if no installed handler
// wanted the exception, print a %VAX-E-CONHANDLER-style diagnostic and halt
// the machine cleanly -- never returning an error, since both outcomes are
// this port's own definition of "handled" (a real condition handler ran, or
// the console's own reporting did).
func (c *Console) handleConsoleFault(f *cpu.ConsoleHandlerFault) error {
	handled, err := c.chf(f)
	if err != nil {
		return err
	}

	if handled {
		return nil
	}

	return c.formatException(f)
}

// chf walks the call-frame chain from the live FP looking for a VMS
// condition handler (frame offset 0, matching the standard CALL frame
// layout emul_call.c's own buildCallFrame produces), invoking the first one
// found (invokeHandler) and, if it declines, continuing the search from
// that frame's own caller -- exactly interrupt.c's own chf loop. Returns
// true once some handler continues (see invokeHandler's own doc comment for
// exactly what that means and a real C-source bug fixed along the way).
func (c *Console) chf(f *cpu.ConsoleHandlerFault) (bool, error) {
	fp := c.CPU.GPR(vax.FP)
	ap := c.CPU.GPR(vax.AP)

	if fp == 0 || ap == 0 {
		return false, nil
	}

	for count := 0; count < maxCHFDepth; count++ {
		handler, err := c.Mem.LoadLongword(c.CPU, fp)
		if err != nil {
			return false, err
		}

		if handler != 0 {
			handled, err := c.invokeHandler(f, handler, fp, count+1)
			if err != nil {
				return false, err
			}

			if handled {
				return true, nil
			}
		}

		// Skip the mask longword, saved AP, and saved-register area exactly
		// as interrupt.c's own chf does, purely to advance addr past them
		// and detect a corrupt frame the same way it would (a failing
		// load here is a real error worth surfacing) -- the C source
		// itself never actually consults the values once read: the *next*
		// iteration's frame address always comes from the saved-FP slot
		// below, not from this walk, so this is dead-but-faithfully-ported
		// bookkeeping, not a control-flow dependency.
		maskWord, err := c.Mem.LoadLongword(c.CPU, fp+4)
		if err != nil {
			return false, err
		}

		if _, err := c.Mem.LoadLongword(c.CPU, fp+8); err != nil { // saved AP
			return false, err
		}

		newFP, err := c.Mem.LoadLongword(c.CPU, fp+12)
		if err != nil {
			return false, err
		}

		addr := fp + 16
		mask := (maskWord >> 16) & 0x0FFF

		for n := uint(0); n <= 11; n++ {
			if mask&(1<<n) == 0 {
				continue
			}

			if _, err := c.Mem.LoadLongword(c.CPU, addr); err != nil {
				return false, err
			}

			addr += 4
		}

		fp = newFP

		if fp == 0 || fp == cpu.SentinelReturn {
			break
		}
	}

	return false, nil
}

// invokeHandler builds the VMS signal-argument and mechanism-argument
// vectors on the current stack (matching chf's own push sequence exactly:
// PSL, PC, each signal argument high-to-low, the condition code, then the
// signal-array length; then R1, R0, the frame-chase depth, the live FP, and
// the mechanism-array length) and calls handler(sigargs, mechargs) via
// Console.Call, restoring SP afterward regardless of outcome -- matching
// chf's own saved_sp restore, which happens whether or not the handler
// continued.
//
// On return, R0/R1 are always restored to their pre-call values first
// (matching chf's own unconditional `vax.R0 = saved_r0; vax.R1 = saved_r1`
// right after reading the handler's return code) -- a handler that declines
// must not leave register corruption behind for the *next* frame's handler
// to inherit. Only if the handler's R0 return code has bit 0 set (VMS's
// SS$_CONTINUE convention -- tested with a real bitwise AND; see this
// function's own note below on interrupt.c's `rc && 0x00000001`, almost
// certainly a `&&`/`&` typo) does it additionally reload R0, R1, and PC from
// the mechanism/signal arrays' own R0/R1/PC slots -- exactly VMS's own
// resignal/continue protocol, where a handler communicates where to resume
// (ordinarily the original fault PC, but a handler may have overwritten
// that slot to redirect execution, e.g. to skip the faulting instruction)
// by writing directly into those stack slots rather than through its own
// return value. Fixing the C source's `&&` here isn't a replicate-as-is
// case: with `&&`, any nonzero R0 (SS$_RESIGNAL and friends included, both
// nonzero) would short-circuit true, so chf would treat every handler as
// "continue" and never search further up the frame chain -- silently
// breaking real resignaling. That's a plain C typo (VMS's SS$_CONTINUE
// bit-0 convention is unambiguous), not an ISA fidelity question, so it's
// fixed directly per this project's bug-fixing policy for clear-cut cases.
func (c *Console) invokeHandler(f *cpu.ConsoleHandlerFault, handler, frameFP uint32, depth int) (bool, error) {
	savedSP := c.CPU.GPR(vax.SP)
	savedHalted := c.Engine.Halted()
	savedR0 := c.CPU.GPR(vax.R0)
	savedR1 := c.CPU.GPR(vax.R1)

	push := func(v uint32) (uint32, error) {
		sp := c.CPU.GPR(vax.SP) - 4
		c.CPU.SetGPR(vax.SP, sp)

		return sp, c.Mem.StoreLongword(c.CPU, sp, v)
	}

	restore := func() {
		c.CPU.SetGPR(vax.SP, savedSP)

		if !savedHalted {
			// Console.Call's own step loop may have halted the machine
			// running the handler (an explicit HALT inside it, however
			// unusual) -- matching chf's own `vax.halted = saved_halt`
			// unconditional restore.
			c.Engine.ClearHalted()
		}
	}

	if _, err := push(uint32(f.PSL)); err != nil {
		return false, err
	}

	argPC, err := push(f.PC)
	if err != nil {
		restore()

		return false, err
	}

	for i := len(f.Args) - 1; i >= 0; i-- {
		if _, err := push(f.Args[i]); err != nil {
			restore()

			return false, err
		}
	}

	if _, err := push(uint32(f.Code)); err != nil {
		restore()

		return false, err
	}

	sigargs := c.CPU.GPR(vax.SP) - 4

	if _, err := push(uint32(3 + len(f.Args))); err != nil {
		restore()

		return false, err
	}

	mechR1, err := push(f.R1)
	if err != nil {
		restore()

		return false, err
	}

	mechR0, err := push(f.R0)
	if err != nil {
		restore()

		return false, err
	}

	if _, err := push(uint32(depth)); err != nil {
		restore()

		return false, err
	}

	if _, err := push(frameFP); err != nil {
		restore()

		return false, err
	}

	mechargs := c.CPU.GPR(vax.SP) - 4
	
	if _, err := push(4); err != nil {
		restore()

		return false, err
	}

	if c.CPU.DebugEnabled(vax.DebugExceptions) {
		c.Printf("DEBUG: condition handler executing CALL %08X ( %08X, %08X )\n", handler, sigargs, mechargs)
	}

	callErr := c.Call(handler, false, sigargs, mechargs)

	restore()

	if callErr != nil {
		return false, callErr
	}

	rc := c.CPU.GPR(vax.R0)

	if c.CPU.DebugEnabled(vax.DebugExceptions) {
		c.Printf("DEBUG: condition handler returns %08X\n", rc)
	}

	c.CPU.SetGPR(vax.R0, savedR0)
	c.CPU.SetGPR(vax.R1, savedR1)

	if rc&1 == 0 {
		return false, nil
	}

	r0, err := c.Mem.LoadLongword(c.CPU, mechR0)
	if err != nil {
		return false, err
	}

	r1, err := c.Mem.LoadLongword(c.CPU, mechR1)
	if err != nil {
		return false, err
	}

	pc, err := c.Mem.LoadLongword(c.CPU, argPC)
	if err != nil {
		return false, err
	}

	c.CPU.SetGPR(vax.R0, r0)
	c.CPU.SetGPR(vax.R1, r1)
	c.CPU.SetGPR(vax.PC, pc)

	return true, nil
}

// formatException is the Go port of format_exception's own fallback
// reporting (once chf found no handler that wanted the exception): print a
// %VAX-E-CONHANDLER-style diagnostic naming the exception and its signal
// arguments, then halt. Uses exceptionName's short SCB-style names (RESOP,
// ACCVIO, ...) rather than porting errors.c's separate, longer-named
// `exceptions[]` table (Reserved Operand, Access Control Violation, ...) --
// matching this port's own existing precedent (reportStopReason's "Break on
// fault" message, show.go's SHOW FAULT) of using the one exception-name
// table it already has everywhere a C source call site would use either of
// its two nearly-identical tables.
func (c *Console) formatException(f *cpu.ConsoleHandlerFault) error {
	c.Printf("%%VAX-E-CONHANDLER, %s, PC=%08X  PSL=%08X\n", exceptionName(f.Code), f.PC, uint32(f.PSL))

	if len(f.Args) > 0 {
		c.Printf("-VAX-E-CONSIGARG, There are %d signal arguments,\n       ", len(f.Args))

		for i, a := range f.Args {
			if i > 0 {
				c.Printf(", ")
			}

			c.Printf("%08X", a)
		}

		c.Printf("\n")
	}

	c.Engine.Halt()

	return nil
}
