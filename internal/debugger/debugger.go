package debugger

import (
	"slices"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/machine"
)

// Prompt is what the front end shows while a debugger session is active,
// as the VMS debugger shows "DBG> ".
const Prompt = "DBG> "

// Debugger is one debugger: the state of a session and the commands that
// act on it. It works on a Console's machine, and implements
// machine.Debugger so that the console can start it and route command
// lines to it.
//
// Subtasks of docs/PHASE-42.md move the rest of the debugging state here
// (the breakpoints, the step mode, the radix) as they move the commands.
type Debugger struct {
	// Machine is the machine this debugger works on. Its output (the
	// console's) is the debugger's too.
	Machine *machine.Machine

	// Console is the console the debugger is installed in.
	Console Console

	// Dispatcher parses and runs the debugger's commands.
	Dispatcher *Dispatcher

	// Breakpoints is the list of address breakpoints, user-set and the
	// one-shot ones a STEP arms for itself (runcontrol.go).
	Breakpoints []*Breakpoint

	// Tracepoints are the tracepoints SET TRACE made (tracepoint.go). They
	// are Breakpoint values, reached by the same rules, but the program
	// goes on after one is reported.
	Tracepoints []*Breakpoint

	// Watchpoints are the locations SET WATCH watches (watch.go).
	Watchpoints []*Watchpoint

	// shown remembers, while the program runs, which pc's source line the
	// last report showed, so that two reports at one pc show it once
	// (showSource). active is true only inside runLoop.
	shown struct {
		pc     uint32
		ok     bool
		active bool
	}

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

	// interrupted is set when Ctrl-C (cpu.ErrAttention) stopped the
	// outermost run in progress (reportStop). Start then opens no session
	// for a run the console started.
	interrupted bool

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
	unhandled *machine.UnhandledException

	// signalled is a condition just signaled that SET BREAK/EXCEPTION
	// wants to stop at, and signalBP the breakpoint that wants it; the run
	// loop takes them at the next instruction boundary (signalBreak).
	signalled *machine.UnhandledException
	signalBP  *Breakpoint

	// pendingDo is the DO clause of the breakpoint that just stopped the
	// program. Start runs it once the debugger is at its prompt.
	pendingDo string

	// symbolic and operands are SET MODE's display modes for
	// EXAMINE/INSTRUCTION: whether names come from the debug symbols, and
	// whether each operand gets an explanation. inputRadix and outputRadix
	// are SET RADIX's: the radix numbers without a prefix are typed in,
	// and the one the offsets in names are shown in. The debugger's own
	// radix is separate from the console's (Decision 8). languageRadix is
	// the radix the image's language starts them at, which CANCEL RADIX
	// goes back to (languageRadixOf).
	symbolic      bool
	modes         displayModes
	operands      machine.OperandsMode
	inputRadix    int
	outputRadix   int
	languageRadix int

	// defaultType is SET TYPE's type for locations the program's symbols
	// don't type, nil for the default, a longword; overrideType is SET
	// TYPE/OVERRIDE's type for every location, nil for none (types.go).
	defaultType  *dataType
	overrideType *dataType

	// curval is %CURVAL, the value last shown or stored, and exception the
	// current exception, the one the program last stopped for, which the
	// %EXC_ symbols describe (builtins.go). A run clears exception.
	curval    currentValue
	exception *machine.UnhandledException

	// numericNames is set while an EXAMINE/NOSYMBOLIC runs: its
	// locations are named by their addresses (data.go).
	numericNames bool

	// examined is where EXAMINE last looked (data.go): what "EXAMINE" with
	// no location, "." and "^" are relative to.
	examined examineState

	// symbolsSet are the modules whose symbols are "set" (SET MODULE,
	// program.go), which SHOW MODULE reports and a SHOW SYMBOL without IN
	// searches. nil until first needed or until an image starts.
	symbolsSet map[*dbgsym.Module]bool

	// active is true while a session is in progress, which is whether
	// the prompt is "DBG> " and command lines come here.
	active bool
}

// Console is what the debugger needs of the console it is installed in
// (internal/console's Console): the machine it debugs, and the console's
// command line, for @file's lines and for naming a console command typed
// at DBG>.
type Console interface {
	// DebugMachine is the machine the debugger works on.
	DebugMachine() *machine.Machine

	// ConsoleDispatch is the console's routing of a command line to
	// whichever grammar is current (the debugger's, or the console's once
	// a line ends the session), or nil when the console has none.
	ConsoleDispatch() func(line string) error

	// IsConsoleVerb reports whether word is one of the console's verbs.
	IsConsoleVerb(word string) bool

	// RunDebuggerProcedure runs a debugger command procedure (@file),
	// each line through dispatch.
	RunDebuggerProcedure(file string, dispatch func(string) error) error
}

// New returns a debugger for the console c's machine, with its commands bound to grammar g
// (parsed from debug.dcl) and its HELP command reading help (parsed from
// debug.help; nil is allowed, and HELP then says there is none).
func New(front Console, g *dcl.Grammar, help *help.File) *Debugger {
	c := front.DebugMachine()
	d := &Debugger{
		Machine: c, Console: front, StepMode: StepOver,
		symbolic: machine.SymbolicDefault(), inputRadix: 16, outputRadix: 16,
		languageRadix: 16, modes: defaultDisplayModes(),
	}
	d.Dispatcher = newDispatcher(d, g, help)
	c.OnUnhandled = d.onUnhandled
	c.OnSignal = d.onSignal
	c.DebugBuiltins = d.builtinSymbol

	return d
}

// Install creates a debugger for front, as New does, and makes it the one
// the console starts. cmd/govax calls it once at start-up. The debugger
// is returned for the caller that wants it (a test).
func Install(front Console, g *dcl.Grammar, help *help.File) *Debugger {
	d := New(front, g, help)
	d.Machine.Debugger = d

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
//     breakpoint, a completed STEP) opens a session if there was none,
//     and the DBG> prompt appears. A run that *ended* (a HALT, the
//     CALLed routine's return) leaves no session behind if it opened none,
//     so GO at the console that halts returns to VAX>. Ctrl-C returns to
//     the command line that started the run: a run the console started
//     (RUN, GO) opens no session, as VMS's DCL takes the CTRL/C and its
//     prompt comes back. A session that was open when the run started
//     stays open either way, until EXIT.
func (d *Debugger) Start(a machine.Activation) error {
	if a.Kind == machine.ActivateAttach {
		d.active = true

		return nil
	}

	nested := d.running > 0

	// A run stopped in another process left the CPU there; the debugger
	// runs process 1 (docs/PHASE-44.md, subtask 9). A nested run is in
	// whatever process its outer run is.
	if !nested {
		if err := d.Machine.ReturnToProcessOne(); err != nil {
			return err
		}
	}

	d.running++

	if !nested {
		d.interrupted = false

		// The current exception is the last one the program stopped for,
		// until it runs again.
		d.exception = nil
	}

	defer func() { d.running-- }()

	var (
		outcome runOutcome
		err     error
	)

	switch a.Kind {
	case machine.ActivateGo:
		outcome, err = d.goRun(a.Addr)

	case machine.ActivateCall:
		outcome, err = d.callRun(*a.Addr, a.Step, a.Args)

	case machine.ActivateImage:
		outcome, err = d.startImage(a)

	case machine.ActivateStep:
		var req stepRequest

		if req, err = d.buildStepRequest(a); err != nil {
			return err
		}

		outcome, err = d.stepRun(a.Addr, req)
	}

	// A run started inside another one (a condition handler) is a
	// subroutine of that run; only the outermost one decides whether the
	// session opens.
	if !nested && outcome == runStopped && err == nil && !(d.interrupted && !d.active) {
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

// NewImage forgets what the debugger kept from the image before, when RUN
// starts another (machine.Debugger). A VMS image gets a debugger of its
// own, so an earlier session's eventpoints (breakpoints of every kind,
// SET BREAK/FAULT's in the engine, tracepoints, watchpoints), SET STEP's
// defaults, a STEP/RETURN or condition it was waiting on, and the
// modules and locations it was looking at all go. What only changes how
// things are shown (SET MODE, SET RADIX, SET TYPE) stays.
func (d *Debugger) NewImage() {
	if d.running > 0 {
		// RUN from inside a run (a program's XFC$CONSOLE_CMD): a STEP in
		// progress keeps the breakpoints it armed for itself, as CANCEL
		// BREAK/ALL does.
		d.Breakpoints = slices.DeleteFunc(d.Breakpoints, func(bp *Breakpoint) bool { return !bp.Step && !bp.Quiet })
	} else {
		d.Breakpoints = nil
	}

	d.Tracepoints = nil
	d.Watchpoints = nil
	d.InstructionBreakpoints = nil
	d.Machine.Engine.ClearFaultBreakpoints()

	d.StepMode = StepOver
	d.stepDefaults = stepSettings{}
	d.pendingReturn = nil

	d.imageDebug = false
	d.imageExited = false
	d.unhandled = nil
	d.signalled, d.signalBP = nil, nil
	d.pendingDo = ""
	d.exception = nil

	d.symbolsSet = nil
	d.examined = examineState{}
	d.curval = currentValue{}
}

// firstWord returns the first word of a command line, the part up to a
// blank or a qualifier's slash.
func firstWord(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.IndexAny(line, " \t/"); i >= 0 {
		return line[:i]
	}

	return line
}
