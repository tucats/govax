package debugger

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// The debugger's input (docs/PHASE-73 - trace trap.md, subtask 5). The VMS
// debugger reads its commands from DBG$INPUT, the terminal unless that
// logical name names a file: a command procedure that runs a program
// under the debugger can give it a file of commands (testdata/dbgcmd's
// and testdata/trace73's procedures do). The debugger reads the file to
// its end, which is EXIT, and the RUN command is over when the session
// is.
//
// A command procedure run by @ (the manual's 8.1) is read the same way:
// each line is a command, a failing one is reported and the procedure
// goes on, and with SET OUTPUT VERIFY each line is echoed, after a blank,
// as it runs: before its output, and not at all if it fails first.

// debugExitStatus is RUN's status when the session ends (EXIT, QUIT, the
// end of DBG$INPUT) with the image still running, as VMS 7.1's was
// (testdata/trace73's TRUNH: %X10028009, a DEBUG facility success with
// STS$M_INHIB_MSG; unconfirmed for QUIT).
const debugExitStatus = 0x10028009

// stsInhibitMsg is STS$M_INHIB_MSG: a status whose message has been shown.
const stsInhibitMsg = 0x10000000

// readInput runs the commands in the file DBG$INPUT names, name: until
// one ends the session, or the file's end, which ends it as EXIT does. A
// file that can't be read is reported, and the session reads the
// terminal instead.
func (d *Debugger) readInput(name string) {
	_, lines, err := d.Console.ReadDebuggerFile(name)
	if err != nil {
		d.showError(err)

		return
	}

	for _, line := range lines {
		if !d.active {
			return
		}

		d.showError(d.Dispatch(line))
	}

	if d.active {
		d.showError(d.exitSession())
	}
}

// runProcedure is @file: the commands in file, read as readInput reads
// DBG$INPUT's but at a level of their own, so they aren't logged as
// input (with VERIFY they are echoed, and the echo is logged as output).
// Once a command has ended the session (EXIT), the file's remaining lines
// go to dispatch, the console's, as lines read at the console would.
func (d *Debugger) runProcedure(file string, dispatch func(string) error) error {
	_, lines, err := d.Console.ReadDebuggerFile(file)
	if err != nil {
		return err
	}

	name := strings.ToUpper(strings.TrimSpace(file))
	verify := d.output.verify

	if verify {
		d.Machine.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_VERIFYICF, "entering", name))
	}

	d.procDepth++

	for _, line := range lines {
		if !d.active {
			d.showError(dispatch(line))

			continue
		}

		// The echo waits for the command's first output, or its end: a
		// command that fails first isn't echoed, as on VMS (testdata/
		// dbgcmd's errors.dlg). A run shows it as it starts (start).
		if d.output.verify {
			d.pendingEcho = " " + strings.TrimRight(line, " \t")
		}

		err := d.Dispatch(line)
		if err != nil {
			d.pendingEcho = ""
		}

		d.flushEcho()
		d.showError(err)
	}

	d.procDepth--

	if verify && d.active {
		d.Machine.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_VERIFYICF, "exiting", name))
	}

	return nil
}

// flushEcho shows the echo of the procedure line being run, if it hasn't
// been shown yet.
func (d *Debugger) flushEcho() {
	if e := d.pendingEcho; e != "" {
		d.pendingEcho = ""
		d.Machine.Printf("%s\n", e)
	}
}

// showError shows a command's failure as the console shows one typed at
// DBG>, unless its message has been shown already.
func (d *Debugger) showError(err error) {
	if err != nil && !vmserrors.MessageInhibited(err) {
		d.Machine.Printf("%%%s\n", err)
	}
}

// exitSession is EXIT and QUIT: the session ends, and with it an image
// still running under it, which is run down (on VMS EXIT also runs its
// exit handlers; govax doesn't, unconfirmed). RUN's status is the
// image's exit status if it exited, marked STS$M_INHIB_MSG as VMS 7.1's
// was (%X10000001), or debugExitStatus. The log, if one was being
// written, is closed.
func (d *Debugger) exitSession() error {
	c := d.Machine
	image := d.imageDebug || d.imageExited

	status, exited := c.DebuggedImageStatus()

	d.End()
	d.closeLog()
	d.output.logging = false

	if !image {
		return c.ReturnToProcessOne()
	}

	// An image still running is run down; one that exited left its
	// memory, and the sections that hold its file, for the session to
	// examine (corevms's KeepP0), which can go now.
	running := c.ImageActive()
	if running {
		c.EndAbandonedImage()
	} else {
		c.ResetICBList()
	}

	d.imageDebug, d.imageExited = false, false

	if err := c.ReturnToProcessOne(); err != nil {
		return err
	}

	switch {
	case running:
		status = debugExitStatus
	case exited:
		status |= stsInhibitMsg
	default:
		return nil
	}

	return c.Hooks.CommandStatus(status)
}
