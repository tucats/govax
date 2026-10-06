package console

import (
	"github.com/tucats/govax/internal/vmserrors"
)

// The console and the debugger are two front ends to one machine
// (docs/PHASE-42.md). The console is the VMS command line (SET DEFAULT,
// MACRO, LINK, RUN, ...); the debugger, in internal/debugger, is the
// machine debugger modeled on the VMS debugger (EXAMINE, STEP, SET BREAK,
// ...), with its own grammar and its own "DBG> " prompt.
//
// The debugger imports this package (it needs the engine, the memory, the
// symbol tables, and the expression evaluator), so this package can't
// import it back. Instead the console holds the debugger through the
// small interface below, and cmd/govax installs the real one.

// ActivationKind says what is starting a debugger session.
type ActivationKind int

const (
	// ActivateAttach starts a session on the machine as it stands, with
	// nothing running: the console's DEBUG command. It is the counterpart
	// of typing Ctrl/Y and then DEBUG at a VMS process.
	ActivateAttach ActivationKind = iota

	// ActivateGo runs the program from Activation.Addr (or the current PC):
	// the console's GO and EXECUTE.
	ActivateGo

	// ActivateCall runs the routine at Activation.Addr with Args: the
	// console's CALL, and the console's own internal calls (RUN's
	// IMAGE$INIT driver, a VMS condition handler).
	ActivateCall

	// ActivateStep executes one STEP from Activation.Addr (or the current
	// PC) in Activation.StepMode.
	ActivateStep

	// ActivateImage runs an image RUN has loaded, under the debugger:
	// Activation.Addr is the driver that calls the image (and its shareable
	// images' initialization routines), and the debugger stops the program
	// at Activation.StopAt, the first instruction of the image's main
	// routine, before any of it runs. It is RUN/DEBUG, and RUN of an image
	// linked /DEBUG.
	ActivateImage
)

// Activation describes what is starting a debugger session. Later
// subtasks of docs/PHASE-42.md add the other ways in (RUN of a debug
// image) and what each needs.
type Activation struct {
	Kind ActivationKind

	// Addr is where a GO or STEP starts (nil: at the current PC), or the
	// routine a CALL invokes.
	Addr *uint32

	// Args are a CALL's arguments, in order.
	Args []uint32

	// Step, on a CALL, runs only the routine's first instruction and stops
	// (CALL/STEP).
	Step bool

	// StepMode is a STEP's mode word (INTO, OVER, or RETURN); empty means
	// the debugger's own default (SET STEP).
	StepMode string

	// The rest of a STEP's request (the debugger's STEP command; the
	// console's own STEP leaves them zero). Each empty or nil value means
	// "the debugger's default" (SET STEP).
	//
	// StepUnit is "LINE" or "INSTRUCTION": how far one step goes. StepClass
	// is "BRANCH" or "CALL": step to the next instruction of that class
	// instead. StepSilent and StepSource turn the report and the source
	// line on or off for this STEP. Count is how many steps to take (zero
	// is one).
	StepUnit, StepClass string
	StepSilent          *bool
	StepSource          *bool
	Count               int

	// StopAt is where an ActivateImage run first stops: the main routine's
	// first instruction after its entry mask (a VAX routine begins with a
	// 16-bit mask saying which registers it saves, which isn't code).
	StopAt *uint32

	// Module and Language name the module the image's main routine is in,
	// and its source language, for the debugger's start-up message
	// (%DEBUG-I-INITIAL).
	Module, Language string
}

// UnhandledException describes a condition that no condition handler
// continued, at the moment VMS's catch-all handler is about to deal with it
// (ending the image, for a severe one). The debugger is told so that it can
// stop the program there, as the VMS debugger does.
type UnhandledException struct {
	// Condition is the condition value, such as SS$_ACCVIO.
	Condition uint32

	// PC is where the condition happened. Preceding says that PC is the
	// instruction *after* the one that signaled it (a call of LIB$SIGNAL),
	// not the one that raised it (a hardware fault).
	PC        uint32
	Preceding bool
}

// Debugger is what the console asks of the debugger. internal/debugger
// implements it and cmd/govax installs it in Console.Debugger.
type Debugger interface {
	// Start begins a session, or runs the program the activation names
	// under the debugger. It returns when the run has ended, or when the
	// debugger has stopped it and is waiting for commands. A run that ended
	// by itself (a HALT, a CALLed routine's return) closes the session it
	// opened; a stop leaves the session open, and the prompt "DBG> ".
	Start(a Activation) error

	// Active reports whether a session is in progress, which is whether
	// the prompt is "DBG> " and command lines go to Dispatch.
	Active() bool

	// Dispatch runs one line of debugger command.
	Dispatch(line string) error
}

// StartDebugger is the console's DEBUG command: it starts a debugger
// session on the machine as it stands. From here until the debugger's
// EXIT, command lines go to the debugger's grammar, not the console's
// (Dispatcher.Dispatch routes them).
func (c *Console) StartDebugger() error {
	if c.Debugger == nil {
		return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
	}

	if err := c.requireInit(); err != nil {
		return err
	}

	// Already in a session (DEBUG from a command file the debugger is
	// reading): nothing to start.
	if c.Debugger.Active() {
		return nil
	}

	return c.Debugger.Start(Activation{Kind: ActivateAttach})
}

// InDebugger reports whether a debugger session is in progress, for the
// front end's choice of prompt and grammar.
func (c *Console) InDebugger() bool {
	return c.Debugger != nil && c.Debugger.Active()
}

