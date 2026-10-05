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
)

// Activation describes what is starting a debugger session. Later
// subtasks of docs/PHASE-42.md add the other ways in (GO, CALL, and RUN
// of a debug image) and what each needs, such as the address to start at.
type Activation struct {
	Kind ActivationKind
}

// Debugger is what the console asks of the debugger. internal/debugger
// implements it and cmd/govax installs it in Console.Debugger.
type Debugger interface {
	// Start begins a session. It returns once the debugger is ready to
	// read commands (or, for a run that finished by itself, once it is
	// over).
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
