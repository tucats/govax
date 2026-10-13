package dclshell

import "github.com/tucats/govax/internal/corevms"

// LIB$PAUSE (docs/PHASE-57, "PAUSE"; the 5.0 manual's LIB-299): an image
// asks to be paused and to give DCL the terminal, as a Ctrl/Y typed at
// the terminal would. DCL reads the Ctrl/Y level (readControlYLevel)
// wherever the image was run from, command level 0 included, with its
// ordinary prompt:
//
//   - CONTINUE resumes the image: LIB$PAUSE returns SS$_NORMAL.
//   - EXIT and STOP end the image, without its exit handlers, as a
//     Ctrl/Y's EXIT does; in a procedure they end every procedure, as at
//     the Ctrl/Y level. The image's status, $STATUS after it, is
//     SS$_NORMAL (VMS V7.1). After CONTINUE at command level 0, the
//     image's own status doesn't become $STATUS (VMS V7.1:
//     pauseContinued).
//   - A command that would run an image, a procedure, or a SPAWN ends
//     the image first, then runs: at level 0 once the command that ran
//     the image is done (as LIB$DO_COMMAND's text runs, dclchain.go), in
//     a procedure at level 0 once every procedure has ended (afterStop).
//     VMS lets SPAWN run with the image paused (the DCL Dictionary's
//     SPAWN example); govax can't while process 1 is inside a shim.
//   - Any other command runs with the image paused, in the context of
//     the level that ran it.
//
// The image isn't run down while DCL reads: the shim ends it once DCL
// has done (corevms.ErrExit), as $EXIT would, and the run that started
// it runs it down. In process 1 the shim runs on the console's own
// goroutine, so DCL reads there and then (pauseProcessOne). In a
// subprocess, whose DCL runs on a goroutine of its own, the shim makes a
// tail call of the CLI stub's Ctrl/Y routine (subcli.go), as if the
// Ctrl/Y AST had come at once: its shim hands the interpreter the CPU,
// which reads the level (imageInterrupted), and the routine returns
// SS$_NORMAL to LIB$PAUSE's caller or ends the image.
//
// VMS V7.1 showed nothing before the prompt, read the terminal in a
// procedure too, and paused with SET NOCONTROL=Y in effect
// (testdata/lib57's interactive part). Unconfirmed: a CLI reading its
// commands from a file or mailbox (LOGINOUT's, not interactive) is
// batch, where LIB$PAUSE does nothing (the 5.0 manual). Not reproduced:
// VMS's %DCL-W-COMPLX for a lexical function at a pause in a procedure
// (I5's F$ENVIRONMENT("DEPTH")), which one case can't explain.

// pauseProcessOne is LIB$PAUSE in process 1 (see above). The machine is
// mid-instruction, so the other processes don't run meanwhile
// (consoleCommands, as for XFC$CONSOLE_CMD).
func (c *Interpreter) pauseProcessOne() (uint32, error) {
	c.consoleCommands++
	defer func() { c.consoleCommands-- }()

	level := c.currentLevel()

	if c.readControlYLevel(level, true) {
		// At command level 0 the image's end, once it has gone on,
		// leaves $STATUS as it was (VMS V7.1: testdata/lib57's
		// interactive part, I1).
		c.pauseContinued = level == nil

		return ssNormal, nil
	}

	// EXIT or STOP: $STATUS is SS$_NORMAL (VMS V7.1, I2, I3, I6).
	c.RTL.Process.ExitStatus = ssNormal

	return ssControlY, corevms.ErrExit
}

// pause is LIB$PAUSE in cli's process (see above): a tail call of the
// CLI stub's Ctrl/Y routine, its AST marked as taken, so the shim there
// hands the interpreter the CPU to read the Ctrl/Y level. A CLI that
// isn't interactive does nothing (SS$_NORMAL), and so does a pause while
// a Ctrl/Y's AST is queued, which reads the Ctrl/Y level itself once the
// process takes it.
func (cli *subprocessCLI) pause() (uint32, error) {
	switch {
	case cli.login && cli.input != nil && !cli.input.Interactive():
		return ssNormal, nil
	case !cli.imageActive, cli.controlYAST != astNone:
		return ssNormal, nil
	}

	cli.controlYAST = astQueued
	cli.pausing = true

	return 0, &corevms.CallRequest{Routine: cli.stub + cliStubControlY, ArgList: cli.stub + cliStubArgList, Tail: true}
}
