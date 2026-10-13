package dclshell

import "strings"

// Chaining (docs/PHASE-57, "Chaining"): an image calls LIB$DO_COMMAND or
// LIB$RUN_PROGRAM, which leave a command line or an image's name with
// the process's DCL (shellCLI's Chain, which sets Shell.chained) and end
// the image. DCL runs it once the command that ran the image (RUN, MCR,
// a foreign command) is done: as a line of its own at the same command
// level, as if it were the next one read there, so its status becomes
// $STATUS and, in a procedure, the level's ON action applies to it.
//
// Choices the manuals leave open, each unconfirmed and a probe case:
//   - The chained line isn't shown by SET VERIFY: verification shows
//     lines a procedure reads, and this one wasn't read.
//   - A command line that's empty runs nothing, leaving $STATUS the
//     image's.
//   - What an image chained is dropped if the command that ran it
//     failed.
//   - An image run under the debugger leaves the session open when it
//     exits, as VMS's debugger does; what it chained runs when the
//     debugger's EXIT ends the session (the console calls RunChained).
//     VMS has no such case to compare.

// chainedCommand is what an image left for DCL to run: a command line,
// or, with program set, the name of an image to run as RUN does.
type chainedCommand struct {
	text    string
	program bool
}

// runChained runs what the image the command just dispatched left, if
// anything, given err, the command's own result: a command that failed
// chains nothing. A chained image may chain again, so this goes on until
// nothing is left. It returns the last chained command's error.
func (c *Interpreter) runChained(err error) error {
	for err == nil && c.chained != nil {
		next := *c.chained
		c.chained = nil

		if next.program {
			err = c.runProgram(next.text)
		} else {
			err = c.DispatchConsole(next.text)
		}
	}

	c.chained = nil

	return err
}

// RunChained runs what an image run under the debugger left for DCL to
// run (runChained); the console calls it after the debugger command that
// ends the session.
func (c *Interpreter) RunChained() error {
	if c.chained == nil {
		return nil
	}

	return c.runChained(nil)
}

// runProgram runs LIB$RUN_PROGRAM's text as a foreign command's image:
// the name, then, past a blank, the image's command text, by DCL's rules
// for a foreign command's (dclText). An empty name runs nothing, leaving
// $STATUS the image's that chained it (VMS V7.1, testdata/lib57).
func (c *Interpreter) runProgram(text string) error {
	name, rest, _ := strings.Cut(strings.TrimSpace(text), " ")
	if name == "" {
		return nil
	}

	return c.statusOf(func() error { return c.runForeign(name, rest) })
}
