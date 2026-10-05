package debugger

import (
	"errors"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the debugger's side of RUN (docs/PHASE-42.md, subtask 5):
// starting an image under the debugger, and what the debugger does when
// that image ends or fails.
//
// A little VMS background for readers new to it. RUN loads an *image* (an
// executable file, .EXE) into memory and starts it. If the image was linked
// with /DEBUG, or the user typed RUN/DEBUG, VMS starts the *debugger*
// first, and the debugger stops the program at its first instruction so
// the user can set breakpoints before any of it runs. govax's RUN builds
// a small driver routine that calls the image's main routine (see
// console/run.go), so "starting the image" here means running that driver
// until the program reaches the main routine's first instruction.
//
// Two events end an image's life, and the debugger watches for both:
//
//   - The image *exits*, which on VMS is the $EXIT system service, called
//     by the program or, when its main routine returns, by the driver.
//     The debugger reports the exit status and stays open, so the user can
//     still examine the program's data; GO and STEP then have nothing to
//     run.
//   - A *condition* (VMS's word for an error or exception, such as an
//     access violation) isn't handled by any of the program's condition
//     handlers. VMS's catch-all handler would print the condition's
//     message and end the image. The debugger breaks first, so the user
//     can see where it happened.

// startImage runs an image the console has loaded, under the debugger: it
// shows the debugger's start-up messages, runs the driver (and with it any
// shareable image's initialization routines) until the program is about to
// execute its main routine's first instruction, and stops there.
func (d *Debugger) startImage(a console.Activation) (runOutcome, error) {
	c := d.Console

	// VMS shows its version banner, a blank line, and then the language
	// and module of the main routine. govax shows its own banner, since
	// the debugger isn't VMS's.
	c.Printf("\n         govax VAX DEBUG\n\n")
	c.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_INITIAL, a.Language, a.Module))

	d.imageDebug = true
	d.imageExited = false

	// The program stops at its first instruction without a message: this
	// breakpoint is the debugger's own, not one the user set.
	start := &Breakpoint{Kind: BreakAddress, Addr: *a.StopAt, Temporary: true, Quiet: true}
	d.Breakpoints = append(d.Breakpoints, start)

	defer d.removeBreakpointPtr(start)

	outcome, err := d.callRun(*a.Addr, false, nil)
	if outcome == runEnded {
		// The image ended before it reached its main routine (or stopped
		// being a program the debugger can follow): nothing to debug.
		d.imageDebug = false
	}

	return outcome, err
}

// onUnhandled is the console's report that a condition no handler
// continued is about to be given to VMS's catch-all handler (the
// console.Console.OnUnhandled hook). While an image runs under the
// debugger, it takes the condition: the run loop sees it pending after
// the instruction that raised it and stops. Outside that (a plain GO of
// kernel code, an image run with /NODEBUG), the catch-all acts as it
// always has.
func (d *Debugger) onUnhandled(u console.UnhandledException) bool {
	if !d.imageDebug || d.unhandled != nil {
		return false
	}

	d.unhandled = &u

	return true
}

// unhandledBreak reports whether a condition was left unhandled by the
// instruction just executed, and if so says so, as the VMS debugger does:
//
//	break on unhandled exception at SUB\ROUTINE\%LINE 12
//
// "at" names the instruction that raised a hardware exception; "preceding"
// names the instruction after the call that signaled one with LIB$SIGNAL,
// since that is as far as the program had got. The program stays paused
// where it is; GO lets VMS's catch-all handler finish with it.
func (d *Debugger) unhandledBreak() bool {
	u := d.unhandled
	if u == nil {
		return false
	}

	d.unhandled = nil

	where := "at"
	if u.Preceding {
		where = "preceding"
	}

	d.Console.Printf("break on unhandled exception %s %s\n", where, d.Console.LocationText(u.PC))

	return true
}

// imageExit handles a run that ended because the image exited: it shows
// the exit status (the program's R0 at $EXIT's return, which the driver
// leaves there), and keeps the session open with nothing left to run.
// ok is false when err isn't an image exit (or this run is a condition
// handler's call, nested inside the image's own run, which returns the
// same way but doesn't end the image).
func (d *Debugger) imageExit(err error) (ok bool) {
	c := d.Console

	if !d.imageDebug || d.running != 1 || !c.ImageActive() || !errors.Is(err, cpu.ErrConsoleCallReturned) {
		return false
	}

	status := c.CPU.GPR(vax.R0)

	// The status's own message line keeps its percent sign: VMS shows
	// "is '%SYSTEM-S-NORMAL, normal successful completion'".
	c.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_EXITSTATUS, c.StatusText(status)))

	d.imageDebug = false
	d.imageExited = true

	return true
}

// requireProgram is the check GO and STEP make before running from the
// current PC: after the image has exited there is no program to run. The
// VMS debugger says so with %DEBUG-E-BADSTARTPC, showing PC 0 as it has
// no program counter left.
func (d *Debugger) requireProgram() error {
	if d.imageExited {
		return vmserrors.New(vmserrors.DBG_BADSTARTPC, uint32(0))
	}

	return nil
}
