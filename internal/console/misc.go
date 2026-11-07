package console

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/vmserrors"
)

// CommandLineString is any "left over" text from govax's own command line,
// put here by cmd/govax when it parses its options: the one-shot command,
// which RunCommandLine runs once STARTUP.COM is done. Empty, there is none.
var CommandLineString string

// RunCommandLine is the text after the image's file name in govax's own
// "run" command line (cmd/govax): the one-shot command's RUN gives it to
// the image as its command text (RunOptions.CommandLine), as a foreign
// command's parameters are given. The host's shell has already parsed it,
// so it's passed as it is, without DCL's uppercasing.
var RunCommandLine string

// CommandLineErr returns the failure of the one-shot command given on
// govax's command line, or nil.
func (c *Console) CommandLineErr() error { return c.commandLineErr }

// Time runs one command (via dispatch) and prints how long it took.
func (c *Console) Time(cmd string, dispatch func(string) error) error {
	if err := c.RequireInit(); err != nil {
		return err
	}

	if strings.TrimSpace(cmd) == "" {
		c.Printf("Time is %s\n", time.Now().Format(time.RFC1123))

		return nil
	}

	startCount := c.Engine.InstructionCount()
	start := time.Now()
	err := dispatch(cmd)
	elapsed := time.Since(start)
	executed := c.Engine.InstructionCount() - startCount

	// Only a command that ran VAX instructions has a count and a rate.
	if executed > 0 && elapsed > 0 {
		rate := float64(executed) / elapsed.Seconds()

		c.Printf("Elapsed time: %f seconds, %s instructions, %s instructions/second\n",
			elapsed.Seconds(), scaleCount(float64(executed)), scaleCount(rate))
	} else {
		c.Printf("Elapsed time: %f seconds\n", elapsed.Seconds())
	}

	return err
}

// scaleCount formats a count with a k, m, or b suffix, such as 2.3m.
func scaleCount(n float64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.1fb", n/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fm", n/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fk", n/1e3)
	}

	return fmt.Sprintf("%.0f", n)
}

// RunCommandLine runs the one-shot command: the text left on govax's own
// command line once its options are parsed (CommandLineString). cmd/govax
// calls it once STARTUP.COM has run, as a login command procedure runs before
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
	_ = c.TakeLimitStop()

	defer c.Finish()

	status := dispatch(text)
	limit := c.TakeLimitStop()

	if status != nil {
		c.commandLineErr = status
	} else if limit != nil {
		// The program was stopped by --instruction-limit or --time-limit.
		// ReportStop has shown the message and let the command finish
		// normally, as it should at the prompt; but a one-shot command
		// that didn't run to its end has failed. The message is marked as
		// shown already, so govax doesn't print it a second time.
		c.commandLineErr = vmserrors.InhibitMessage(limit)
	}

	return true
}

// Printf writes to the output of the DCL running now.
func (c *Console) Printf(format string, args ...any) {
	c.Interpreter.Printf(format, args...)
}

// Help writes the topic words name in h, govax's built-in help, to the
// console (help.File.Show).
func (c *Console) Help(h *help.File, words []string) error {
	h.Show(c.Out, words)

	return nil
}

// HelpCommand implements DCL's HELP (dclcmd.Context.Help, docs/PHASE-90 -
// VMS help.md, subtask 6): its qualifiers, read from r, and its responses
// read from SYS$INPUT. When there's no /LIBRARY and SYS$HELP:HELPLIB.HLB
// can't be opened, it shows h, govax's built-in help, as before (until the
// libraries are part of govax's build, the phase's subtask 7).
func (c *Console) HelpCommand(h *help.File, r *dcl.Result) error {
	// given: typed on the command line, not a /DEFAULT the grammar filled in.
	given := func(q string) bool { return r.Present(q) && !r.Defaulted(q) && !r.Negated(q) }

	topic := r.String("TOPIC")
	opts := dclcmd.HelpOptions{
		Words:         help.Split(topic),
		Prompt:        !r.Negated("PROMPT"),
		Instructions:  !r.Negated("INSTRUCTIONS"),
		LibList:       !r.Negated("LIBLIST"),
		NoLibrary:     r.Negated("LIBRARY"),
		NoUserLibrary: r.Negated("USERLIBRARY"),
		OutputGiven:   given("OUTPUT"),
		NoOutput:      r.Negated("OUTPUT"),
		Paged:         given("PAGE"),
		NoPage:        r.Negated("PAGE"),
	}

	if !opts.NoLibrary {
		opts.Library = r.String("LIBRARY")
	}

	if opts.OutputGiven {
		opts.Output = r.String("OUTPUT")
	}

	if given("USERLIBRARY") {
		opts.UserLibraries = append([]string{}, r.List("USERLIBRARY")...)
	}

	if opts.Paged {
		opts.Page = r.List("PAGE")
	}

	for _, q := range []string{"SEARCH", "EXACT", "HIGHLIGHT", "WRAP", "MESSAGE"} {
		if given(q) {
			opts.Unimplemented = "/" + q

			break
		}
	}

	defer c.EndHelpInput()

	err := c.cmd().Help(opts, c.ReadHelpResponse, c.OutputToFile())
	if errors.Is(err, dclcmd.ErrNoMainLibrary) {
		// The old help's words: a "/" starts a new word, as it does in
		// DCL, so HELP SHOW SYMBOL/ALL finds the same "/ALL" topic as
		// HELP SHOW SYMBOL /ALL.
		return c.Help(h, strings.Fields(strings.ReplaceAll(topic, "/", " /")))
	}

	return err
}

// EndSession ends the console's use of the machine as govax exits: an
// image of process 1's that never exited is run down, the other processes
// are deleted (their files closed), and every file process 1 and DCL's
// OPEN have open is closed, so that what they wrote reaches the volumes
// before they're dismounted; so is the page file.
func (c *Console) EndSession() {
	c.RunDownAbandonedImage()
	c.EndOtherProcesses()
	c.CloseDCLFiles()

	if c.RTL != nil {
		c.RTL.CloseFiles()

		if err := c.RTL.ClosePageFile(); err != nil {
			fmt.Fprintf(c.Out, "%%GOVAX-W-PAGEFILE, closing the page file: %v\n", err)
		}
	}
}
