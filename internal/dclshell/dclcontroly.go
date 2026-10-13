package dclshell

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// STOP and Ctrl/Y in command procedures (docs/PHASE-50 - DCL command
// procedures.md, subtask 18), by the OpenVMS User's Manual's rules (13.7,
// 13.8.1, 13.11 to 13.13) and the DCL Dictionary's (STOP, CONTINUE, EXIT,
// SET CONTROL).
//
// STOP in a command procedure ends every procedure: "all command levels
// are unstacked and control returns to command level 0". Each level's
// loop (runLevel) sees Console.stopping after its line and ends, and so
// does each level outside it, without taking its ON action.
//
// Ctrl/Y, typed while a procedure runs, is DCL's (unless the image
// running has a Ctrl/Y AST, or SET NOCONTROL=Y is in effect). What it
// does depends on the innermost level:
//
//   - With an ON CONTROL_Y command, the image running, if any, is forced
//     to exit, and the command runs at that level, as ON's other commands
//     do; it stays in effect (13.12).
//   - Without one, a procedure nested inside another exits to the level
//     above, which goes on (13.8.1's table, Figure 13-3).
//   - Without one, at command level 1, the procedure is interrupted, and
//     DCL reads commands from the terminal at the Ctrl/Y level
//     (controlYLevel). CONTINUE resumes the procedure, and the image if
//     one was interrupted; EXIT and STOP end the procedure; a command
//     that runs an image (or a procedure) ends it too, and then runs at
//     level 0 (13.11.1).
//
// A command DCL performs itself completes before the Ctrl/Y is acted on;
// an image is interrupted where it is (HandleAttention, on the engine's
// Step loop). Ctrl/C, with no Ctrl/C AST to take it, acts as Ctrl/Y in a
// procedure (CONTINUE resumes what "was interrupted by pressing Ctrl/Y or
// Ctrl/C"). At the terminal, outside a procedure, the keys keep govax's
// meanings (cmd/govax/attention.go): Ctrl/C interrupts, Ctrl/Y ends
// govax.
//
// The keys go to the process the terminal is attached to (Phase 51,
// subtask 3): process 1, the subprocess a SPAWN without /NOWAIT gave it
// to, or the process an ATTACH gave it to (corevms's Attached and
// attach.go; the DCL Dictionary, SPAWN and ATTACH). A subprocess's DCL follows the same rules in
// its procedures. An image it runs is interrupted by an AST of DCL's
// (subcli.go's "Ctrl/Y"), with the image paused while DCL acts:
// CONTINUE at the Ctrl/Y level lets it go on. Outside a procedure, a
// subprocess's Ctrl/Y or Ctrl/C ends the image it runs (SS$_CONTROLY),
// and its DCL goes on with its next command; it doesn't end govax
// (govax's choice: VMS's DCL would prompt, with the image paused).

// errLeaveControlY is what a command typed at the Ctrl/Y level returns
// instead of running when it would run an image or a command procedure:
// the interrupted procedure ends first, and the command runs at level 0
// (controlYLevel).
var errLeaveControlY = errors.New("command ends the interrupted procedure")

// controlYPrompt is the Ctrl/Y level while DCL reads it: resume is set
// by CONTINUE, and end by EXIT or STOP at command level 0, where only
// LIB$PAUSE reads it (dclpause.go). pause is set for LIB$PAUSE's.
type controlYPrompt struct {
	resume, end, pause bool
}

// controlKeys are a process's SET [NO]CONTROL settings (Shell.control):
// noY is set while Ctrl/Y is disabled, t while Ctrl/T is enabled. A
// subprocess's DCL starts with both keys as process 1's does at login,
// not with its parent's settings (unconfirmed, a probe case).
type controlKeys struct {
	noY bool
	t   bool
}

// setControl enables (on) or disables the keys SET [NO]CONTROL names: T
// for Ctrl/T and Y for Ctrl/Y. Disabling Ctrl/Y also cancels the current
// command level's ON CONTROL_Y (the User's Manual, 13.13.1).
func (s *Shell) setControl(t, y, on bool) {
	if t {
		s.control.t = on
	}

	if y {
		s.control.noY = !on

		if level := s.currentLevel(); level != nil && !on {
			level.on.controlY = ""
		}
	}
}

// ControlYEnabled reports whether Ctrl/Y does anything: false after SET
// NOCONTROL=Y in the DCL of the process the terminal is attached to,
// when the key is echoed and ignored (the DCL Dictionary, SET CONTROL).
// Safe to call from any goroutine; cmd/govax asks it for each Ctrl/Y
// typed. It reads the copy noteControlKeys keeps.
func (c *Interpreter) ControlYEnabled() bool { return !c.terminalNoY.Load() }

// terminalShell is the DCL of the process the terminal is attached to:
// a subprocess's (attachedCLI), or process 1's.
func (c *Interpreter) terminalShell() *Shell {
	if cli := c.attachedCLI(); cli != nil {
		return cli.shell
	}

	return c.home
}

// terminalControlY reports whether Ctrl/Y is enabled in the DCL of the
// process the terminal is attached to, as ControlYEnabled does, but
// from the processes themselves: for the engine's goroutine.
func (c *Interpreter) terminalControlY() bool { return !c.terminalShell().control.noY }

// noteControlKeys updates ControlYEnabled's copy of the attached
// process's Ctrl/Y setting. It is called wherever that may change: a
// setting changed (SET CONTROL, LIB$DISABLE_CTRL and LIB$ENABLE_CTRL), a
// subprocess's DCL starting (it may be attached by a waiting SPAWN), and
// a process deleted (the terminal returns to the one that spawned it),
// and an ATTACH (shellCLI's Attach).
func (c *Interpreter) noteControlKeys() {
	c.terminalNoY.Store(!c.terminalControlY())
}

// isAttentionKey reports whether key is Ctrl/C or Ctrl/Y.
func isAttentionKey(key byte) bool {
	return key == cpu.AttentionCtrlY || key == cpu.AttentionCtrlC
}

// dclTakesAttention reports whether an attention key, which no AST took,
// is the DCL running now's to act on (dclTakesControlY).
func (c *Interpreter) dclTakesAttention(key byte) bool {
	return isAttentionKey(key) && c.dclTakesControlY()
}

// dclTakesControlY reports whether a Ctrl/Y is the DCL running now's to
// act on: the terminal is attached to its process, a DCL command
// procedure is running (not a debugger command procedure, or a debugger
// session), Ctrl/Y is enabled, and DCL isn't already reading the Ctrl/Y
// level or ending every level.
func (c *Interpreter) dclTakesControlY() bool {
	level := c.currentLevel()

	return level != nil && !level.source.plain && c.attachedCLI() == c.subCLI &&
		!c.machine.InDebugger() && !c.control.noY && c.controlYLevelActive == nil && !c.stopping
}

// attachedCLI is the CLI of the subprocess the terminal is attached to,
// or nil when that is process 1 (corevms's Attached).
func (c *Interpreter) attachedCLI() *subprocessCLI {
	one := c.machine.ProcessOne()
	if one == nil {
		return nil
	}

	if env := one.Attached(); env != one {
		return c.clis[env]
	}

	return nil
}

// HandleAttention makes Console a cpu.AttentionHandler: an attention key
// typed while a program runs is offered first to the running process,
// whose Ctrl/C or Ctrl/Y AST may take it (docs/PHASE-26.md, subtask 27).
// Otherwise, with SET NOCONTROL=Y in effect, the key is ignored; in a
// command procedure, it is DCL's Ctrl/Y: with no ON CONTROL_Y at any
// level, the Ctrl/Y level is read now, with the image paused, and
// CONTINUE lets it go on; anything else stops the image, and the
// procedure's loop takes the action when the command has returned
// (takeControlY).
func (c *Interpreter) HandleAttention(key byte) bool {
	if env := c.machine.RunningProcess(); env != nil && env.AttentionAny(env, key) {
		return true
	}

	if !c.terminalControlY() {
		c.forgetAttention()

		return true
	}

	if cli := c.attachedCLI(); cli != nil && isAttentionKey(key) {
		return c.subprocessAttention(cli)
	}

	if !c.dclTakesAttention(key) {
		return false
	}

	c.forgetAttention()

	level := c.currentLevel()
	if c.controlYHandler() == nil && c.controlYLevel(level) {
		return true
	}

	// The image stops (ErrAttention), and the procedure acts when RUN
	// returns.
	c.controlYTaken = true

	return false
}

// subprocessAttention is HandleAttention for cli, the subprocess the
// terminal is attached to: its DCL takes the key, however deep its
// command level. An image it runs is interrupted by its Ctrl/Y AST
// (subcli.go), which its DCL acts on (imageInterrupted); otherwise its
// DCL is in a command of its own, which it notes (interrupted) and acts
// on when the command is done. A second key while the AST is still
// queued, not taken (the image runs with ASTs disabled, say), isn't
// taken: the machine stops, as it would for a key no DCL takes.
func (c *Interpreter) subprocessAttention(cli *subprocessCLI) bool {
	switch {
	case cli.controlYAST == astQueued:
		return false

	case cli.imageActive && cli.controlYAST == astNone:
		cli.controlYAST = astQueued
		cli.env.QueueCLIAST(cli.stub + cliStubControlY)

	default:
		cli.interrupted = true
	}

	c.forgetAttention()

	return true
}

// imageInterrupted acts on a Ctrl/Y in cli's DCL that its AST
// interrupted an image for (subcli.go), and returns how the AST ends: in
// a procedure, by the rules HandleAttention follows in process 1, with
// the Ctrl/Y level read while the image is paused, and CONTINUE letting
// it go on (yieldContinue); otherwise the image ends (yieldEndImage),
// and the procedure acts when the command that ran it has returned
// (takeControlY). Outside a procedure, the image ends. Called on the
// interpreter's goroutine, with its Shell the console's.
func (c *Interpreter) imageInterrupted(cli *subprocessCLI) cliYield {
	ended := cliYield{kind: yieldEndImage, status: ssControlY}

	// LIB$PAUSE's call of the AST's routine reads the Ctrl/Y level at
	// any command level (dclpause.go).
	if cli.pausing {
		cli.pausing = false

		if c.readControlYLevel(c.currentLevel(), true) {
			return cliYield{kind: yieldContinue}
		}

		return ended
	}

	if !c.dclTakesControlY() {
		return ended
	}

	if c.controlYHandler() == nil && c.controlYLevel(c.currentLevel()) {
		return cliYield{kind: yieldContinue}
	}

	c.controlYTaken = true

	return ended
}

// forgetAttention forgets the attention key typed, and the one that
// stopped the last run: DCL has dealt with it, so cmd/govax doesn't take
// it for its own (a Ctrl/Y that ends govax).
func (c *Interpreter) forgetAttention() {
	if c.machine.Engine != nil {
		c.machine.Engine.ClearAttention()
	}
}

// takeControlY reports, once, whether a Ctrl/Y (or Ctrl/C) is DCL's to
// act on after the command a procedure just ran: one that interrupted
// an image (HandleAttention), or one typed while DCL ran a command of
// its own, still pending.
func (c *Interpreter) takeControlY() bool {
	if c.controlYTaken {
		c.controlYTaken = false
		c.forgetAttention()

		return true
	}

	// A subprocess's Ctrl/Y, noted while one of DCL's own commands ran
	// (subprocessAttention).
	if cli := c.subCLI; cli != nil && cli.interrupted {
		cli.interrupted = false

		return c.dclTakesControlY()
	}

	if c.machine.Engine == nil || !c.dclTakesAttention(c.machine.Engine.PendingAttention()) {
		return false
	}

	c.forgetAttention()

	return true
}

// controlYHandler returns the innermost command level with an ON
// CONTROL_Y, nil for none.
func (c *Interpreter) controlYHandler() *commandLevel {
	for i := len(c.levels) - 1; i >= 0; i-- {
		if c.levels[i].on.controlY != "" {
			return c.levels[i]
		}
	}

	return nil
}

// controlYAction is what a Ctrl/Y does to level, the innermost command
// level, once its command has returned, as VMS 7.3 did it
// (testdata/dcl50's round 5): with an ON CONTROL_Y of its own, its
// command runs (next); with one at a level outside it, the levels inside
// that one end (stop, with $STATUS marked as shown), and that level goes
// on at the line after its @ or CALL, its own action not taken (the
// User's Manual's Figure 13-3); with none at any level, DCL reads the
// Ctrl/Y level, whatever the depth. An image the Ctrl/Y stopped is run
// down first, as one forced to exit.
func (c *Interpreter) controlYAction(level *commandLevel) (exit procedureExit, stop bool, next func() error) {
	switch handler := c.controlYHandler(); {
	case handler == level:
		c.RunDownAbandonedImage()

		command := level.on.controlY

		return procedureExit{}, false, func() error { return level.dispatch(command) }

	case handler != nil:
		c.RunDownAbandonedImage()
		c.controlYUnwind = handler

		return c.unwindExit(), true, nil
	}

	c.controlYLevel(level)

	return procedureExit{}, false, nil
}

// unwindExit is how a level that a Ctrl/Y unwinds ends: with $STATUS,
// marked as shown (VMS 7.3 gave %X10010001 after a WRITE's
// %X00010001).
func (c *Interpreter) unwindExit() procedureExit {
	return procedureExit{status: c.status | stsInhibitMsg, cause: c.statusErr}
}

// controlYLevel reads commands from the terminal at the Ctrl/Y level,
// for level, the procedure Ctrl/Y interrupted, and reports whether the
// procedure goes on (CONTINUE). The commands are read in the
// procedure's context (its symbols; 13.11.1's note), with the
// terminal's IF blocks. EXIT and STOP end every procedure ("both the
// EXIT and STOP command return you to the DCL level", 13.7.1), as does
// a command that would run an image or a procedure, which then runs at
// level 0 (Console.afterStop). Ctrl/Y typed at the prompt gives a new
// prompt; Ctrl/Z, or the end of the input, is EXIT.
func (c *Interpreter) controlYLevel(level *commandLevel) bool {
	return c.readControlYLevel(level, false)
}

// readControlYLevel is controlYLevel, and LIB$PAUSE's Ctrl/Y level too
// (pause set; dclpause.go), which may be read at command level 0 (level
// nil). There, EXIT and STOP end the image, as they end the procedure
// above (controlYPrompt.end), and a command that would run an image or a
// procedure runs once the image has ended, after the command that ran
// it, as a command LIB$DO_COMMAND leaves does (dclchain.go). A pause's
// image isn't run down here: LIB$PAUSE's shim ends it, as the Ctrl/Y
// AST's does in a subprocess.
func (c *Interpreter) readControlYLevel(level *commandLevel, pause bool) bool {
	prompt := &controlYPrompt{pause: pause}
	c.controlYLevelActive = prompt

	defer func() { c.controlYLevelActive = nil }()

	dispatch := c.DispatchConsole
	if level != nil {
		dispatch = level.dispatch
	}

	runDown := func() {
		if !pause {
			c.RunDownAbandonedImage()
		}
	}

	for c.Running() {
		line, ok := c.readTerminal(c.commandPrompt())
		if !ok {
			if c.machine.Engine != nil && isAttentionKey(c.machine.Engine.PendingAttention()) {
				c.forgetAttention()

				continue
			}

			line = "EXIT"
		}

		// A subprocess's Ctrl/Y typed at this level does nothing more
		// than end a WAIT typed here (subprocessAttention): one typed
		// before the command, or during it, is forgotten.
		cli := c.subCLI
		if cli != nil {
			cli.interrupted = false
		}

		err := dispatch(line)

		if cli != nil {
			cli.interrupted = false
		}

		switch {
		case errors.Is(err, errLeaveControlY) && level == nil:
			c.chained = &chainedCommand{text: line}

			return false

		case errors.Is(err, errLeaveControlY):
			c.afterStop = func() error { return level.dispatch(line) }
			c.stopProcedures(procedureExit{status: c.status, cause: c.statusErr})
			runDown()

			return false

		case err != nil && !vmserrors.MessageInhibited(err):
			if level != nil {
				c.procedureMessage(level, err)
			} else {
				fmt.Fprintln(c.Out, "%"+err.Error())
			}
		}

		switch {
		case prompt.resume:
			return true
		case prompt.end:
			return false
		case level != nil && level.exiting && !c.stopping:
			c.stopProcedures(level.exit)
			runDown()

			return false
		case c.stopping:
			runDown()

			return false
		}
	}

	return false
}

// commandPrompt is the prompt of the DCL running now: a subprocess's own
// (SPAWN/PROMPT), or the console's.
func (c *Interpreter) commandPrompt() string {
	if cli := c.subCLI; cli != nil {
		return cli.prompt
	}

	return c.Prompt()
}

// leaveControlY returns errLeaveControlY when DCL is reading the Ctrl/Y
// level and a command that runs an image (own false) would run: the
// interrupted procedure ends before it does.
func (c *Interpreter) leaveControlY(own bool) error {
	if c.controlYLevelActive != nil && !own {
		return errLeaveControlY
	}

	return nil
}

// continueCommand is CONTINUE: at the Ctrl/Y level, it resumes the
// procedure Ctrl/Y interrupted; elsewhere it does nothing (the target of
// an ON or IF, 13.9). It leaves $STATUS as it was.
func (c *Interpreter) continueCommand() error {
	if c.controlYLevelActive != nil {
		c.controlYLevelActive.resume = true
	}

	c.keepStatus()

	return nil
}

// stopProcedures makes every command level end (STOP): each level's loop
// ends after its line, and the levels outside it after theirs, each with
// exit (for STOP, $STATUS as it is: VMS 7.3 left it unchanged,
// testdata/dcl50's round 5). runLevel turns stopping off when the last
// level has gone.
func (c *Interpreter) stopProcedures(exit procedureExit) {
	if len(c.levels) > 0 {
		c.stopping = true
		c.stopExit = exit
	}
}

// stopCommand is STOP [process-name] [/IDENTIFICATION=pid]. With a
// process, it deletes that process (StopProcess). Without one, in a
// command procedure, or at the Ctrl/Y level, it ends every command
// procedure, returning to level 0 (the DCL Dictionary, STOP); an image
// Ctrl/Y interrupted is run down, without its exit handlers (govax runs
// none). At the terminal it runs down an image Ctrl/C or Ctrl/Y left
// stopped.
func (c *Interpreter) stopCommand(id int64, r *dcl.Result) error {
	name, pid := r.String("PROCESS_NAME"), r.String("IDENTIFICATION")

	exit := stopExitExecutive
	if r.Negated("EXIT") {
		exit = stopNoExit
	} else if r.Present("EXIT") {
		mode, err := stopExitMode(r.String("EXIT"))
		if err != nil {
			return err
		}

		exit = mode
	}

	if name != "" || pid != "" {
		if r.Present("IMAGE") {
			return c.StopImage(name, pid)
		}

		return c.stopProcess(name, pid, exit)
	}

	c.keepStatus()

	if len(c.levels) == 0 {
		// At LIB$PAUSE's Ctrl/Y level, the paused image ends
		// (dclpause.go).
		if prompt := c.controlYLevelActive; prompt != nil {
			prompt.end = true

			return nil
		}

		c.RunDownAbandonedImage()

		return nil
	}

	c.stopProcedures(procedureExit{status: c.status, cause: c.statusErr})

	return nil
}

// setControlCommand is SET [NO]CONTROL[=(T,Y)]: Y, the default, enables
// or disables Ctrl/Y (and Ctrl/C where no AST takes it), and T, Ctrl/T,
// which govax doesn't show. SET NOCONTROL=Y also cancels the current
// level's ON CONTROL_Y (13.13.1). A keyword other than T or Y is
// CLI$_IVKEYW.
func (c *Interpreter) setControlCommand(r *dcl.Result) error {
	negated := r.Negated("WHAT")

	keys := []string{"Y"}
	if list := splitKeywordList(r.List("KEYWORDS")); len(list) > 0 {
		keys = list
	}

	for _, k := range keys {
		if k != "T" && k != "Y" {
			return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, k)
		}
	}

	c.setControl(slices.Contains(keys, "T"), slices.Contains(keys, "Y"), !negated)
	c.noteControlKeys()

	return nil
}

// splitKeywordList splits a keyword list parameter's items, "(A,B)" or
// "A", into its keywords, uppercase.
func splitKeywordList(items []string) []string {
	var keywords []string

	for _, item := range items {
		for _, k := range strings.Split(strings.Trim(item, "()"), ",") {
			if k = strings.ToUpper(strings.TrimSpace(k)); k != "" {
				keywords = append(keywords, k)
			}
		}
	}

	return keywords
}

// controlString is F$ENVIRONMENT's CONTROL (enabled true) or NOCONTROL:
// the control keys SET CONTROL enabled or disabled, "T,Y" or one of them.
func (c *Interpreter) controlString(enabled bool) string {
	var keys []string

	if c.control.t == enabled {
		keys = append(keys, "T")
	}

	if !c.control.noY == enabled {
		keys = append(keys, "Y")
	}

	return strings.Join(keys, ",")
}

// stopExit is STOP's /EXIT or /NOEXIT: which exit handlers run before
// the process is deleted.
type stopExit int

const (
	stopNoExit         stopExit = iota // /NOEXIT
	stopExitKernel                     // /EXIT=KERNEL_MODE
	stopExitExecutive                  // /EXIT=EXECUTIVE_MODE, the default
	stopExitSupervisor                 // /EXIT=SUPERVISOR_MODE
	stopExitUser                       // /EXIT=USER_MODE
)

// stopExitModes are /EXIT's access modes, by keyword.
var stopExitModes = map[string]stopExit{
	"KERNEL_MODE":     stopExitKernel,
	"EXECUTIVE_MODE":  stopExitExecutive,
	"SUPERVISOR_MODE": stopExitSupervisor,
	"USER_MODE":       stopExitUser,
}

// stopExitMode is /EXIT's value as a mode: a keyword or any abbreviation
// of one (each starts with its own letter); another is CLI_IVKEYW.
func stopExitMode(value string) (stopExit, error) {
	key := strings.ToUpper(strings.TrimSpace(value))

	for name, mode := range stopExitModes {
		if key != "" && strings.HasPrefix(name, key) {
			return mode, nil
		}
	}

	return 0, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
}

// stopTarget is the process STOP names, by name (in process 1's UIC
// group) or by /IDENTIFICATION (hexadecimal; the PID wins if both are
// given): SS$_IVIDENT for a PID that can't be read, SS$_NONEXPR for a
// process that isn't there.
func (c *Interpreter) stopTarget(name, pid string) (*corevms.Environment, error) {
	if pid != "" {
		v, err := strconv.ParseUint(strings.TrimSpace(pid), 16, 32)
		if err != nil {
			return nil, vmserrors.New(vmserrors.SS_IVIDENT)
		}

		env, found := c.RTL.FindProcess(uint32(v))
		if !found {
			return nil, vmserrors.New(vmserrors.SS_NONEXPR)
		}

		return env, nil
	}

	env, found := c.RTL.FindProcessName(c.RTL.Process.UICGroup(), name)
	if !found {
		return nil, vmserrors.New(vmserrors.SS_NONEXPR)
	}

	return env, nil
}

// ssForcedExit is the status STOP/IMAGE's forced exit gives the image
// (unconfirmed against VMS: the Dictionary says only that it calls
// $FORCEX).
var ssForcedExit = vmsdef.Symbols["SS$_FORCEDEXIT"]

// StopImage is STOP/IMAGE with a process: the image the process runs is
// made to exit ($FORCEX, corevms.Environment.ForceExit), its user-mode
// exit handlers called, with the status SS$_FORCEDEXIT, and the process
// isn't deleted (the DCL Dictionary). A subprocess's DCL then goes on
// with its next command; a process that runs only an image (one $CREPRC
// made) ends with it, as when its image ends any other way. A subprocess
// at its DCL, running no image, has nothing to stop. Process 1's own
// image, one Ctrl/C or Ctrl/Y left stopped, is run down, as STOP alone
// runs it down.
func (c *Interpreter) StopImage(name, pid string) error {
	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	env, err := c.stopTarget(name, pid)
	if err != nil {
		return err
	}

	switch cli := c.clis[env]; {
	case env == c.RTL:
		c.RunDownAbandonedImage()
	case cli == nil || cli.imageActive:
		c.RTL.ForceExit(env, ssForcedExit)
	}

	return nil
}

// stopProcess is STOP of a process, with /EXIT's mode or /NOEXIT. govax
// runs exit handlers only in the mode that calls $EXIT, and images
// declare them only in user mode, so only /EXIT=USER_MODE has any to
// run: the process's image is made to exit first, as STOP/IMAGE makes
// it, its exit handlers called, with the status SS$_ABORT, and the
// process deleted when it has ended (a subprocess's DCL then logs out,
// its procedures ended, stopAfterImage). A process running no image is
// deleted at once. Every other mode, and /NOEXIT, deletes the process at
// once (StopProcess).
func (c *Interpreter) stopProcess(name, pid string, exit stopExit) error {
	if exit != stopExitUser {
		return c.StopProcess(name, pid)
	}

	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	env, err := c.stopTarget(name, pid)
	if err != nil {
		return err
	}

	switch cli := c.clis[env]; {
	case env == c.RTL:
		return vmserrors.New(vmserrors.SS_NOPRIV)
	case cli == nil:
		c.RTL.ForceExit(env, ssAbort)
	case cli.imageActive:
		cli.stopAfterImage = true
		c.RTL.ForceExit(env, ssAbort)
	default:
		return c.StopProcess(name, pid)
	}

	return nil
}

// StopProcess is STOP: it deletes a process and its subprocesses at once,
// as the operator with every privilege would. The process is named, as
// for SHOW PROCESS, by name (in process 1's UIC group) or by /IDENTIFICATION
// (hexadecimal; the PID wins if both are given). A process that isn't
// there is %SYSTEM-W-NONEXPR. Process 1 is the console's own, so STOP
// of it is refused (%SYSTEM-F-NOPRIV): govax has no logging out, and
// EXIT ends the session. (STOP with neither a name nor an
// identification ends command procedures instead: stopCommand.)
//
// The deleted process's files are closed, and its termination message
// goes to its termination mailbox if it had one, with the final status
// SS$_ABORT. If the CPU was in the process (a run stopped there), it is
// returned to process 1.
func (c *Interpreter) StopProcess(name, pid string) error {
	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	env, err := c.stopTarget(name, pid)
	if err != nil {
		return err
	}

	if env == c.RTL {
		return vmserrors.New(vmserrors.SS_NOPRIV)
	}

	c.RTL.DeleteNow(env)

	return c.machine.ReturnToProcessOne()
}
