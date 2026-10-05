package debugger

import (
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/cpu"
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

	// Breakpoints is the list of address breakpoints, user-set and the
	// one-shot ones a STEP arms for itself (runcontrol.go).
	Breakpoints []*Breakpoint

	// InstructionBreakpoints holds every opcode currently flagged to break
	// on execution — SET BREAK/INSTRUCTION, the Go equivalent of vax.c's
	// own instruction[n].debugdata & OP_DBG_BREAK flag. Kept separate from
	// Breakpoints because the C source itself never folds this into its
	// breakpoint_list either (instbreak.go).
	InstructionBreakpoints map[*cpu.Instruction]bool

	// StepMode is what STEP does about routine calls by default (SET STEP
	// INTO, OVER, or RETURN). New sets it to the VMS default, StepOver.
	StepMode StepMode

	// stepDefaults are SET STEP's other defaults: the unit (line or
	// instruction), silent, and source (step.go).
	stepDefaults stepSettings

	// pendingReturn is a STEP/RETURN still waiting for the RET it was
	// given for (step.go), or nil.
	pendingReturn *pendingReturn

	// running counts the runs in progress. It is more than one when a run
	// starts another from inside itself: the console's condition handling
	// calls a VMS condition handler while the faulting program's run is
	// still going. Only the outermost run opens or closes the session.
	running int

	// imageDebug is true while an image RUN loaded is running under the
	// debugger (startImage) and hasn't exited. It is what makes the
	// debugger break on an unhandled condition and report the image's
	// exit; a plain GO of other code does neither.
	imageDebug bool

	// imageExited is true once that image has exited: GO and STEP then
	// have no program to run (requireProgram) until the next RUN.
	imageExited bool

	// unhandled is a condition no handler continued, set by onUnhandled
	// while the machine runs and taken by the run loop at the next
	// instruction boundary (unhandledBreak).
	unhandled *console.UnhandledException

	// signalled is a condition just signaled that SET BREAK/EXCEPTION
	// wants to stop at, and signalBP the breakpoint that wants it; the run
	// loop takes them at the next instruction boundary (signalBreak).
	signalled *console.UnhandledException
	signalBP  *Breakpoint

	// pendingDo is the DO clause of the breakpoint that just stopped the
	// program. Start runs it once the debugger is at its prompt.
	pendingDo string

	// symbolic and operands are SET MODE's display modes for
	// EXAMINE/INSTRUCTION: whether names come from the debug symbols, and
	// whether each operand gets an explanation. inputRadix and outputRadix
	// are SET RADIX's: the radix numbers without a prefix are typed in,
	// and the one the offsets in names are shown in. The debugger's own
	// radix is separate from the console's (Decision 8).
	symbolic    bool
	modes       displayModes
	operands    console.OperandsMode
	inputRadix  int
	outputRadix int

	// examined is where EXAMINE last looked (data.go): what "EXAMINE" with
	// no location, "." and "^" are relative to.
	examined examineState

	// active is true while a session is in progress, which is whether
	// the prompt is "DBG> " and command lines come here.
	active bool
}

// New returns a debugger for c, with its commands bound to grammar g
// (parsed from debug.dcl) and its HELP command reading help (parsed from
// debug.help; nil is allowed, and HELP then says there is none).
func New(c *console.Console, g *dcl.Grammar, help *console.Help) *Debugger {
	d := &Debugger{
		Console: c, StepMode: StepOver,
		symbolic: console.SymbolicDefault(), inputRadix: 16, outputRadix: 16,
		modes: defaultDisplayModes(),
	}
	d.Dispatcher = newDispatcher(d, g, help)
	c.OnUnhandled = d.onUnhandled
	c.OnSignal = d.onSignal

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

// Start begins a session, or runs what the activation names under the
// debugger (docs/PHASE-42.md, Decision 2).
//
//   - ActivateAttach just opens the session: the DBG> prompt on the
//     machine as it stands.
//   - GO, CALL, and STEP run the program. A run the debugger *stopped* (a
//     breakpoint, a completed STEP, Ctrl-C) opens a session if there was
//     none, and the DBG> prompt appears. A run that *ended* (a HALT, the
//     CALLed routine's return) leaves no session behind if it opened none,
//     so GO at the console that halts returns to VAX>. A session that was
//     open when the run started stays open either way, until EXIT.
func (d *Debugger) Start(a console.Activation) error {
	if a.Kind == console.ActivateAttach {
		d.active = true

		return nil
	}

	nested := d.running > 0
	d.running++

	defer func() { d.running-- }()

	var (
		outcome runOutcome
		err     error
	)

	switch a.Kind {
	case console.ActivateGo:
		outcome, err = d.goRun(a.Addr)

	case console.ActivateCall:
		outcome, err = d.callRun(*a.Addr, a.Step, a.Args)

	case console.ActivateImage:
		outcome, err = d.startImage(a)

	case console.ActivateStep:
		var req stepRequest

		if req, err = d.buildStepRequest(a); err != nil {
			return err
		}

		outcome, err = d.stepRun(a.Addr, req)
	}

	// A run started inside another one (a condition handler) is a
	// subroutine of that run; only the outermost one decides whether the
	// session opens.
	if !nested && outcome == runStopped && err == nil {
		d.active = true

		d.runDo()
	}

	return err
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
