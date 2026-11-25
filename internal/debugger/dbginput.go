package debugger

import (
	"errors"
	"io"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// The debugger's input (docs/planning/PHASE-73 - trace trap.md, subtask 5). The VMS
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

	src := d.pushInput(lines)
	defer d.popInput()

	for line, ok := src.read(); ok; line, ok = src.read() {
		if !d.active {
			return
		}

		d.showError(d.Dispatch(line))
	}

	if d.active {
		// The end of DBG$INPUT is an EXIT (unconfirmed).
		d.showError(d.exitSession(false))
	}
}

// runProcedure is @file: the commands in file, read as readInput reads
// DBG$INPUT's but at a level of their own, so they aren't logged as
// input (with VERIFY they are echoed, and the echo is logged as output).
// Once a command has ended the session, the file's remaining lines go to
// dispatch, the console's, as lines read at the console would. EXIT or
// QUIT in the file, debugging an image, ends the file, not the session
// (the VMS debugger
// manual's EXIT and QUIT; VMS 7.1's DBG113Q: the session went on to
// DBG$INPUT's next command).
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

	src := d.pushInput(lines)

	for line, ok := src.read(); ok; line, ok = src.read() {
		if !d.active {
			d.showError(dispatch(line))

			continue
		}

		// The echo waits for the command's first output, or its end: a
		// command refused as it's read isn't echoed, as on VMS (testdata/
		// dbgcmd's errors.dlg, notEchoed). A run shows it as it starts
		// (start).
		if d.output.verify {
			d.pendingEcho = " " + strings.TrimRight(line, " \t")
		}

		err := d.Dispatch(line)
		if err != nil && refusedAsRead(line, err) {
			d.pendingEcho = ""
		}

		d.flushEcho()
		d.showError(err)

		// EXIT or QUIT in the procedure ended it (exitSession).
		if d.leaving {
			d.leaving = false

			break
		}
	}

	d.popInput()
	d.procDepth--

	if verify && d.active {
		d.Machine.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_VERIFYICF, "exiting", name))
	}

	return nil
}

// inputSource is a file of commands being read: DBG$INPUT's or @'s. A
// command that reads lines of its own (HELP's responses) takes them from
// the same place, so the commands after them are read after them.
type inputSource struct {
	lines []string
	next  int
}

// read returns the next line, and false at the end.
func (s *inputSource) read() (string, bool) {
	if s.next >= len(s.lines) {
		return "", false
	}

	s.next++

	return s.lines[s.next-1], true
}

// pushInput makes lines the innermost input, and popInput ends it.
func (d *Debugger) pushInput(lines []string) *inputSource {
	src := &inputSource{lines: lines}
	d.inputs = append(d.inputs, src)

	return src
}

func (d *Debugger) popInput() {
	d.inputs = d.inputs[:len(d.inputs)-1]
}

// readResponse reads a response to a prompt of HELP's: VMS 7.1's debugger
// read them from its own input (testdata/help90's part F, whose DBG$INPUT
// file had to answer each prompt). From a file the next line is taken and
// neither the prompt nor the line is shown or logged, as VMS's log
// (dbgh.dlg) shows neither. Otherwise the console reads it: a command
// procedure's next data line, or a line typed after the prompt, which goes
// to the terminal and not to the log. io.EOF is the input's end or Ctrl/Z,
// which ends HELP.
func (d *Debugger) readResponse(prompt string) (string, error) {
	if n := len(d.inputs); n > 0 {
		line, ok := d.inputs[n-1].read()
		if !ok {
			return "", io.EOF
		}

		return line, nil
	}

	line, ok := d.Console.ReadDebuggerReply(prompt)
	if !ok {
		return "", io.EOF
	}

	return line, nil
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
// still running under it, which is run down. EXIT first runs the image's
// exit handlers, as $EXIT does (VMS 7.1's DBG113M: "DBG113: the exit
// handler ran" after an EXIT at a break), and QUIT doesn't (the VMS
// debugger manual). RUN's status is the image's exit status if it exited
// by itself, marked STS$M_INHIB_MSG as VMS 7.1's was (%X10000001), or
// debugExitStatus. The log, if one was being written, is closed. In an @
// procedure, debugging an image, either ends the procedure alone
// (runProcedure).
func (d *Debugger) exitSession(quit bool) error {
	// Debugging an image, as on VMS. The console's own session (DEBUG
	// with no image) ends, and hands the file's rest to the console.
	if d.procDepth > 0 && (d.imageDebug || d.imageExited) {
		d.leaving = true

		return nil
	}

	c := d.Machine
	image := d.imageDebug || d.imageExited
	stopped := d.imageDebug && c.ImageActive()

	if stopped && !quit {
		d.runExitHandlers()
	}

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
	case running || stopped:
		status = debugExitStatus
	case exited:
		status |= stsInhibitMsg
	default:
		return nil
	}

	return c.Hooks.CommandStatus(status)
}

// notEchoed are the messages of a command VMS's debugger refuses as it
// reads it, before it runs, so its log has no echo of the command: syntax,
// a name it doesn't know, a missing part, or a location with no successor.
// A command that fails as it runs (NOACCESSR, SYMNOTACT, NOTINSCOPE, ...)
// is echoed. Found from every probe session's log (testdata/dbgcmd,
// testdata/dbg89, testdata/dbg113).
var notEchoed = []uint32{
	vmserrors.DBG_SYNTAX, vmserrors.DBG_NOSYMBOL, vmserrors.DBG_NEEDMORE, vmserrors.DBG_NONXTLIN,
	vmserrors.DBG_NOSUCC, vmserrors.DBG_MISOPEMIS, vmserrors.DBG_NODEFSCPE, vmserrors.DBG_NOSCOPELIST,
	vmserrors.DBG_NOTNUMSCOPE, vmserrors.DBG_MISINVNUM, vmserrors.DBG_NOTRECORD, vmserrors.DBG_TOOMANSUB,
	vmserrors.DBG_TOOFEWSUB, vmserrors.CLI_EXTRAPARAMETER, vmserrors.CLI_NEEDEXPR,
}

// refusedAsRead reports whether err is a refusal of line as it's read
// (notEchoed). A module no image has is one, but for SET MODULE's, which
// VMS 7.1 echoed (testdata/dbg113's DBG113M: not SET SOURCE/MODULE's or
// TYPE's).
func refusedAsRead(line string, err error) bool {
	var e vmserrors.VMSError
	if !errors.As(err, &e) {
		return false
	}

	if e.Equals(vmserrors.DBG_NOSUCHMODULE) {
		f := strings.Fields(strings.ToUpper(line))

		return len(f) < 2 || f[0] != "SET" || !strings.HasPrefix("MODULE", f[1]) || len(f[1]) < 3
	}

	for _, code := range notEchoed {
		if e.Equals(code) {
			return true
		}
	}

	return false
}

// runExitHandlers ends the image stopped under the debugger as $EXIT
// does, so its exit handlers run, with the eventpoints gone and nothing
// said of the exit (the session is ending). The status the handlers are
// given is SS$_NORMAL (unconfirmed: VMS's handler didn't show it).
func (d *Debugger) runExitHandlers() {
	c := d.Machine

	addr, err := c.EvalWholeIn(16, "SYS$EXIT")
	if err != nil {
		return
	}

	d.Breakpoints, d.Tracepoints, d.Watchpoints = nil, nil, nil
	d.pendingReturns = nil
	d.quietExit = true

	defer func() { d.quietExit = false }()

	_, _ = d.callRun(addr, false, []uint32{1})
}
