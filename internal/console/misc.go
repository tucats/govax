package console

import (
	"strings"
	"time"

	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/vmserrors"
)

// CommandLineString is any "left over" text from govax's own command line,
// put here by cmd/govax when it parses its options: the one-shot command,
// which RunCommandLine runs once vax.init is done. Empty, there is none.
var CommandLineString string

// RunCommandLine is the text after the image's file name in govax's own
// "run" command line (cmd/govax): the one-shot command's RUN gives it to
// the image as its command text (RunOptions.CommandLine), as a foreign
// command's parameters are given. The host's shell has already parsed it,
// so it's passed as it is, without DCL's uppercasing.
var RunCommandLine string

// Running reports whether the console should keep reading commands,
// matching vax.console.running (cleared by QUIT/EXIT.
func (c *Interpreter) Running() bool { return !c.quit }

// CommandLineErr returns the failure of the one-shot command given on
// govax's command line, or nil.
func (c *Console) CommandLineErr() error { return c.commandLineErr }

// Quit implements QUIT/EXIT: stops the command loop.
func (c *Interpreter) Quit() error {
	if err := c.machine.requireInit(); err != nil {
		return err
	}

	c.quit = true

	return nil
}

// Time runs one command (via dispatch) and prints how long it took.
func (c *Console) Time(cmd string, dispatch func(string) error) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if strings.TrimSpace(cmd) == "" {
		c.Printf("Time is %s\n", time.Now().Format(time.RFC1123))

		return nil
	}

	start := time.Now()
	err := dispatch(cmd)
	elapsed := time.Since(start)

	c.Printf("Elapsed time: %f seconds\n", elapsed.Seconds())

	return err
}

// RunCommandLine runs the one-shot command: the text left on govax's own
// command line once its options are parsed (CommandLineString). cmd/govax
// calls it once vax.init has run, as a login command procedure runs before
// the command interpreter reads its input (docs/PHASE-50 - DCL command
// procedures.md): the command is level 0's input, in place of the
// terminal, and the session ends when it's done. It reports whether there
// was a command to run; its failure is CommandLineErr's.
func (c *Console) RunCommandLine(dispatch func(string) error) bool {
	if CommandLineString == "" {
		return false
	}

	text := CommandLineString
	CommandLineString = ""
	c.runCommandLine, RunCommandLine = RunCommandLine, ""

	// The command is the user's program, not boot: hold it to the
	// --instruction-limit and --time-limit options.
	c.ApplyRunLimits()

	// The command ends the session either way: a failed one-shot command
	// shouldn't leave the user at a prompt. run (cmd/govax) reports its
	// failure and exits nonzero.
	c.limitStop = nil

	defer func() { c.quit = true }()

	if status := dispatch(text); status != nil {
		c.commandLineErr = status
	} else if c.limitStop != nil {
		// The program was stopped by --instruction-limit or --time-limit.
		// ReportStop has shown the message and let the command finish
		// normally, as it should at the prompt; but a one-shot command
		// that didn't run to its end has failed. The message is marked as
		// shown already, so govax doesn't print it a second time.
		c.commandLineErr = vmserrors.InhibitMessage(c.limitStop)
	}

	return true
}

// Printf writes to the output of the DCL running now.
func (c *Console) Printf(format string, args ...any) {
	c.Interpreter.Printf(format, args...)
}

// Help implements the HELP console command: the topic words name in h,
// written to the console (help.File.Show).
func (c *Console) Help(h *help.File, words []string) error {
	h.Show(c.Out, words)

	return nil
}

// EndSession ends the console's use of the machine as govax exits: an
// image of process 1's that never exited is run down, the other processes
// are deleted (their files closed), and every file process 1 and DCL's
// OPEN have open is closed, so that what they wrote reaches the volumes
// before they're dismounted.
func (c *Console) EndSession() {
	c.runDownAbandonedImage()
	c.endOtherProcesses()
	c.closeDCLFiles()

	if c.RTL != nil {
		c.RTL.CloseFiles()
	}
}
