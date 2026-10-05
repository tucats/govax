package debugger

import (
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
)

// Prompt is what the front end shows while a debugger session is active,
// as the VMS debugger shows "DBG> ".
const Prompt = "DBG> "

// Debugger is one debugger: the state of a session and the commands that
// act on it. It works on a Console's machine, and implements
// console.Debugger so that the console can start it and route command
// lines to it.
//
// Subtasks of docs/PHASE-42.md move the rest of the debugging state here
// (the breakpoints, the step mode, the radix) as they move the commands.
type Debugger struct {
	// Console is the console whose machine this debugger works on. Its
	// output stream is the debugger's too.
	Console *console.Console

	// Dispatcher parses and runs the debugger's commands.
	Dispatcher *Dispatcher

	// active is true while a session is in progress, which is whether
	// the prompt is "DBG> " and command lines come here.
	active bool
}

// New returns a debugger for c, with its commands bound to grammar g
// (parsed from debug.dcl) and its HELP command reading help (parsed from
// debug.help; nil is allowed, and HELP then says there is none).
func New(c *console.Console, g *dcl.Grammar, help *console.Help) *Debugger {
	d := &Debugger{Console: c}
	d.Dispatcher = newDispatcher(d, g, help)

	return d
}

// Install creates a debugger for c, as New does, and makes it the one
// the console starts. cmd/govax calls it once at start-up. The debugger
// is returned for the caller that wants it (a test).
func Install(c *console.Console, g *dcl.Grammar, help *console.Help) *Debugger {
	d := New(c, g, help)
	c.Debugger = d

	return d
}

// Active reports whether a session is in progress.
func (d *Debugger) Active() bool { return d.active }

// Start begins a session, as the console's DEBUG command asks. With
// nothing running there is no state to set up yet: the session is just
// the DBG> prompt on the machine as it stands. (The other ways in are
// added with the commands that need them, in later subtasks.)
func (d *Debugger) Start(a console.Activation) error {
	d.active = true

	return nil
}

// Dispatch parses and runs one line of debugger command, as the console's
// Dispatch does for the console's.
func (d *Debugger) Dispatch(line string) error {
	return d.Dispatcher.Dispatch(line)
}

// End ends the session: the prompt and the command lines go back to the
// console. EXIT and QUIT call it.
func (d *Debugger) End() { d.active = false }

// firstWord returns the first word of a command line, the part up to a
// blank or a qualifier's slash.
func firstWord(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.IndexAny(line, " \t/"); i >= 0 {
		return line[:i]
	}

	return line
}
