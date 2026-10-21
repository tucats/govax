package dclshell

import (
	"io"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/rms"
)

// NewInterpreter returns the DCL of a console running on m, whose
// terminal output is out: process 1's Shell, on process 1's logical names
// and a default directory of its own, and the machine's Hooks for what
// it asks of DCL (its output and default directory, $STATUS, Ctrl/Y, the
// subprocess CLIs, process 1's creation, the terminal input,
// XFC$CONSOLE_CMD, and QUIT).
func NewInterpreter(m *machine.Machine, out io.Writer) *Interpreter {
	session := rms.NewSession(m.Mounts)
	session.Logicals = m.ProcessOneLogicals()

	c := &Interpreter{
		Shell: &Shell{
			Out:              out,
			Logicals:         m.ProcessOneLogicals(),
			ContainerSession: session,
		},
		machine: m,
	}

	c.home = c.Shell
	c.route = c.DispatchConsole
	m.Hooks = machine.Hooks{
		Output:         func() io.Writer { return c.Out },
		Session:        func() *rms.Session { return c.ContainerSession },
		ProcessOneOnly: c.ProcessOneOnly,
		CommandStatus: func(status uint32) error {
			// An image LIB$PAUSE paused at level 0 and CONTINUE resumed
			// leaves $STATUS when it succeeds (dclpause.go).
			continued := c.pauseContinued
			c.pauseContinued = false

			if continued && status&1 != 0 {
				c.keepStatus()

				return nil
			}

			c.SetCommandStatus(status, nil)

			if status&1 != 0 {
				return nil
			}

			return c.statusFailure(status)
		},
		ControlYTaken:     func() bool { return c.controlYTaken },
		CLIImageEnded:     c.cliImageEnded,
		NewProcessOne:     c.newProcessOne,
		ProcessOneStarted: c.adoptProcedureFiles,
		ProcessDeleted:    c.processDeleted,
		Input:             func() io.Reader { return c.In },
		ConsoleCommand:    c.consoleCommand,
		RequestQuit:       func() { c.quit = true },
		HandleAttention:   c.HandleAttention,
		DCLSymbol:         c.numericSymbol,
	}

	c.setStatus(ssNormal, nil)

	// An input host file named with no directory, and not in the current
	// directory, is looked for along the machine's Paths too, ending with
	// govax's embedded files, so that LIBRARY/LIST of a bare macro
	// library name finds govax's own (internal/rms/hostfile.go). Paths is
	// read at each lookup, since main.go sets it after New returns.
	c.ContainerSession.HostFallback = func(name string) ([]byte, error) {
		return m.Paths.ReadFile(name)
	}

	return c
}

// consoleCommand is Hooks.ConsoleCommand, XFC$CONSOLE_CMD: it runs line
// as a console command line (Dispatcher.DispatchConsole), returning 0 on
// success and 1 on any error or with no grammar yet (no Dispatcher). Console's own
// VAX_xxx statuses have no exact Go equivalent, since Dispatch reports
// failure as a Go error; a caller needing more than "did it work" calls
// Dispatch itself.
func (c *Interpreter) consoleCommand(line string) uint32 {
	if c.Grammar == nil {
		return 1
	}

	c.consoleCommands++
	defer func() { c.consoleCommands-- }()

	if err := c.DispatchConsole(line); err != nil {
		return 1
	}

	return 0
}

// consoleInput and consoleOutput are the RTL's input and output streams:
// whatever c.In and c.Out are when a program reads or writes, not when
// the environment was made, so that changing them (as a test scripting a
// program's input or capturing its output does) reaches LIB$PUT_OUTPUT,
// the terminal driver, and RMS's terminal $GET and $PUT too, as it
// already reaches the console device.
type consoleInput struct{ c *Interpreter }

func (r consoleInput) Read(p []byte) (int, error) {
	if r.c.In == nil {
		return 0, io.EOF
	}

	return r.c.In.Read(p)
}

// Ready makes consoleInput a corevms.TerminalSource: whether a read
// would get a byte (or the end of the input) at once. An input that
// can't say (a test's script, a file) never makes a reader wait.
func (r consoleInput) Ready() bool {
	if src, ok := r.c.In.(corevms.TerminalSource); ok {
		return src.Ready()
	}

	return true
}

// consoleOutput is the terminal, as every process's terminal output
// reaches it: process 1's Out at command level 0, whichever process's DCL
// runs now, and whatever /OUTPUT file a procedure's own output goes to.
type consoleOutput struct{ c *Interpreter }

func (w consoleOutput) Write(p []byte) (int, error) {
	out := w.c.home.Out
	if levels := w.c.home.levels; len(levels) > 0 {
		out = levels[0].sysCommand
	}

	if out == nil {
		return len(p), nil
	}

	return out.Write(p)
}

// newProcessOne is Hooks.NewProcessOne: process 1 on sys, the console's,
// sharing its logical names and session (so a program's RMS calls see
// SET DEFAULT's default directory), on the console's terminal. It is
// process 1's DCL's process (home), and its command interpreter: a
// process LIB$SPAWN creates, or $CREPRC creates to run LOGINOUT, runs
// the subprocess CLI (subcli.go, docs/PHASE-48.md), whose shim is the
// console's.
func (c *Interpreter) newProcessOne(sys *corevms.System) *corevms.Environment {
	c.clis = nil
	c.spawnPage = 0

	sys.Interpreter = cliHost{c}
	sys.Shims().Register(cliShimCode, "EXE$CLI_COMMAND", c.cliCommand)

	// Process 1 is the first process on a new System, whose process table
	// is empty, so adding it can't fail.
	env, err := corevms.NewEnvironment(sys, c.home.Logicals, consoleInput{c}, consoleOutput{c})
	if err != nil {
		panic(err)
	}

	env.Session = c.home.ContainerSession
	c.home.RTL = env

	// The console is process 1's command interpreter: LIB$SPAWN and the
	// LIB$ CLI routines work in it (libcli.go).
	env.CLI = shellCLI{c: c, shell: c.home}

	return env
}

// processDeleted is Hooks.ProcessDeleted: a deleted process's CLI goes
// with it, and its interpreter is unwound if it was waiting for its turn
// (subcli.go).
func (c *Interpreter) processDeleted(env *corevms.Environment) {
	if cli := c.clis[env]; cli != nil {
		cli.abort()
	}

	delete(c.clis, env)
	c.noteControlKeys()
}
