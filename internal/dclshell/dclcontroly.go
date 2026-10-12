package dclshell

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/dcl"
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

// errLeaveControlY is what a command typed at the Ctrl/Y level returns
// instead of running when it would run an image or a command procedure:
// the interrupted procedure ends first, and the command runs at level 0
// (controlYLevel).
var errLeaveControlY = errors.New("command ends the interrupted procedure")

// controlYPrompt is the Ctrl/Y level while DCL reads it: resume is set by
// CONTINUE.
type controlYPrompt struct {
	resume bool
}

// controlKeys are SET [NO]CONTROL's settings: whether Ctrl/Y and Ctrl/T
// are enabled. Ctrl/Y is read from the terminal's goroutines
// (ControlYEnabled), so it is atomic.
type controlKeys struct {
	noY atomic.Bool
	t   bool
}

// ControlYEnabled reports whether Ctrl/Y does anything: false after SET
// NOCONTROL=Y, when the key is echoed and ignored (the DCL Dictionary,
// SET CONTROL). Safe to call from any goroutine; cmd/govax asks it for
// each Ctrl/Y typed.
func (c *Interpreter) ControlYEnabled() bool { return !c.control.noY.Load() }

// isAttentionKey reports whether key is Ctrl/C or Ctrl/Y.
func isAttentionKey(key byte) bool {
	return key == cpu.AttentionCtrlY || key == cpu.AttentionCtrlC
}

// dclTakesAttention reports whether an attention key, which no AST took,
// is DCL's to act on now: a DCL command procedure is running (not a
// debugger command procedure, or a debugger session), Ctrl/Y is enabled,
// and DCL isn't already reading the Ctrl/Y level or ending every level.
func (c *Interpreter) dclTakesAttention(key byte) bool {
	level := c.currentLevel()

	return isAttentionKey(key) && level != nil && !level.source.plain &&
		!c.machine.InDebugger() && c.ControlYEnabled() && c.controlYLevelActive == nil && !c.stopping
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

	if !c.ControlYEnabled() {
		c.forgetAttention()

		return true
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
	prompt := &controlYPrompt{}
	c.controlYLevelActive = prompt

	defer func() { c.controlYLevelActive = nil }()

	for c.Running() {
		line, ok := c.readTerminal(c.Prompt())
		if !ok {
			if c.machine.Engine != nil && isAttentionKey(c.machine.Engine.PendingAttention()) {
				c.forgetAttention()

				continue
			}

			line = "EXIT"
		}

		err := level.dispatch(line)

		switch {
		case errors.Is(err, errLeaveControlY):
			c.afterStop = func() error { return level.dispatch(line) }
			c.stopProcedures(procedureExit{status: c.status, cause: c.statusErr})
			c.RunDownAbandonedImage()

			return false

		case err != nil && !vmserrors.MessageInhibited(err):
			c.procedureMessage(level, err)
		}

		switch {
		case prompt.resume:
			return true
		case level.exiting && !c.stopping:
			c.stopProcedures(level.exit)
			c.RunDownAbandonedImage()

			return false
		case c.stopping:
			c.RunDownAbandonedImage()

			return false
		}
	}

	return false
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

	if name != "" || pid != "" {
		return c.StopProcess(name, pid)
	}

	c.keepStatus()

	if len(c.levels) == 0 {
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

	for _, k := range keys {
		switch k {
		case "T":
			c.control.t = !negated
		case "Y":
			c.control.noY.Store(negated)

			if level := c.currentLevel(); level != nil && negated {
				level.on.controlY = ""
			}
		}
	}

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

	if c.ControlYEnabled() == enabled {
		keys = append(keys, "Y")
	}

	return strings.Join(keys, ",")
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

	env := c.RTL

	switch {
	case pid != "":
		v, err := strconv.ParseUint(strings.TrimSpace(pid), 16, 32)
		if err != nil {
			return vmserrors.New(vmserrors.SS_IVIDENT)
		}

		var found bool
		if env, found = c.RTL.FindProcess(uint32(v)); !found {
			return vmserrors.New(vmserrors.SS_NONEXPR)
		}

	case name != "":
		var found bool
		if env, found = c.RTL.FindProcessName(c.RTL.Process.UICGroup(), name); !found {
			return vmserrors.New(vmserrors.SS_NONEXPR)
		}
	}

	if env == c.RTL {
		return vmserrors.New(vmserrors.SS_NOPRIV)
	}

	c.RTL.DeleteNow(env)

	return c.machine.ReturnToProcessOne()
}
