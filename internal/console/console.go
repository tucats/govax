package console

import (
	"io"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/rms"
)

// Console is the govax interactive monitor's machine-facing state: the
// instantiated CPU/Memory/Engine (nil until an INIT command creates them,
// matching the C source's vax_init flag), the symbol table, the console's
// own radix/deposit-address/verbosity settings, and breakpoints — the Go
// equivalent of vax.console's fields in reference/eVAX/eVAX/Headers/vax.h,
// minus everything owned by a later phase (device state, RTL/microkernel
// validity, the assembler's own settings).
type Console struct {
	// Machine is the emulated system (vaxmachine.go).
	*machine.Machine

	// Interpreter is the DCL (interpreter.go).
	*Interpreter

	// Dispatcher backs XFC$CONSOLE_CMD (a running VAX program asking the
	// console to execute a command line on its behalf). Unlike Engine/RTL,
	// this isn't created by Init/Zero — a Dispatcher needs the DCL grammar
	// and help text (main.go's own startup sequence), which Console
	// itself knows nothing about — so it's nil until whoever constructs
	// both (see main.go) assigns it explicitly. XFC$CONSOLE_CMD
	// reports failure if this is still nil.
	Dispatcher *Dispatcher

	// commandLineErr is the failure of the one-shot command given on
	// govax's command line (RunCommandLine), which ends the
	// session; read via CommandLineErr.
	commandLineErr error

	// ScreenSize, when set, gives the screen's height in lines and width
	// in columns (0 if unknown) for paged output (TYPE/PAGE, page.go),
	// instead of asking the terminal. Tests set it; nil asks Out.
	ScreenSize func() (height, width int)

	// HostLibrary is the host directory LINK and MACRO look in for
	// IMAGELIB.OLB, STARLET.OLB, shareable images, and STARLET.MLB when
	// SYS$LIBRARY and SYS$SHARE don't lead to them (internal/console/
	// syslib.go). Empty means the vax.library setting's (or the older
	// vax.link.library's).
	HostLibrary string

	// runCommandLine is RunCommandLine, taken when govax's one-shot
	// command runs, for that command's RUN to give its image.
	runCommandLine string
}

// New returns a Console with no machine allocated yet (vax_init == 0 in the
// C source's terms) — an INIT command (see init.go) must run before most
// other commands will accept.
func New(out io.Writer) *Console {
	m := machine.NewMachine(out)

	session := rms.NewSession(m.Mounts)
	session.Logicals = m.ProcessOneLogicals()

	c := &Console{
		Machine: m,
		Interpreter: &Interpreter{
			Shell: &Shell{
				Out:              out,
				Logicals:         m.ProcessOneLogicals(),
				ContainerSession: session,
			},
		},
	}

	c.home = c.Shell
	c.machine = c.Machine
	c.route = c.DispatchConsole
	c.Hooks = machine.Hooks{
		Output:         func() io.Writer { return c.Out },
		Session:        func() *rms.Session { return c.ContainerSession },
		ProcessOneOnly: c.processOneOnly,
		CommandStatus: func(status uint32) error {
			c.setCommandStatus(status, nil)

			if status&1 != 0 {
				return nil
			}

			return c.statusFailure(status)
		},
		ControlYTaken:   func() bool { return c.controlYTaken },
		CLIImageEnded:   c.cliImageEnded,
		NewProcessOne:   c.newProcessOne,
		ProcessDeleted:  c.processDeleted,
		Input:           func() io.Reader { return c.In },
		ConsoleCommand:  c.consoleCommand,
		RequestQuit:     func() { c.quit = true },
		HandleAttention: c.Interpreter.HandleAttention,
	}

	c.setStatus(ssNormal, nil)

	// An input host file named with no directory, and not in the current
	// directory, is looked for along c.Paths too, ending with govax's
	// embedded files, so that LIBRARY/LIST of a bare macro library name
	// finds govax's own (internal/rms/hostfile.go). c.Paths is read at
	// each lookup, since main.go sets it after NewConsole returns.
	c.ContainerSession.HostFallback = func(name string) ([]byte, error) {
		return c.Paths.ReadFile(name)
	}

	return c
}

// consoleCommand is Hooks.ConsoleCommand, XFC$CONSOLE_CMD: it runs line
// as a console command line (Dispatcher.DispatchConsole), returning 0 on
// success and 1 on any error or with no Dispatcher yet. Console's own
// VAX_xxx statuses have no exact Go equivalent, since Dispatch reports
// failure as a Go error; a caller needing more than "did it work" calls
// Dispatch itself.
func (c *Console) consoleCommand(line string) uint32 {
	if c.Dispatcher == nil {
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
type consoleInput struct{ c *Console }

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
// reaches it: process 1's Out, whichever process's DCL runs now.
type consoleOutput struct{ c *Console }

func (w consoleOutput) Write(p []byte) (int, error) {
	out := w.c.home.Out

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
func (c *Console) newProcessOne(sys *corevms.System) *corevms.Environment {
	c.clis = nil
	c.spawnPage = 0

	sys.Interpreter = cliHost{c.Interpreter}
	sys.Shims().Register(cliShimCode, "EXE$CLI_COMMAND", c.cliCommand)

	// Process 1 is the first process on a new System, whose process table
	// is empty, so adding it can't fail.
	env, err := corevms.NewEnvironment(sys, c.home.Logicals, consoleInput{c}, consoleOutput{c})
	if err != nil {
		panic(err)
	}

	env.Session = c.home.ContainerSession
	c.home.RTL = env

	// The console is process 1's command interpreter: LIB$SPAWN works in
	// it (corevms.Environment.HasCLI).
	env.HasCLI = true

	return env
}

// processDeleted is Hooks.ProcessDeleted: a deleted process's CLI goes
// with it, and its interpreter is unwound if it was waiting for its turn
// (subcli.go).
func (c *Console) processDeleted(env *corevms.Environment) {
	if cli := c.clis[env]; cli != nil {
		cli.abort()
	}

	delete(c.clis, env)
}
