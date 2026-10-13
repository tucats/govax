package dclshell

import (
	"fmt"
	"io"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/machine"
)

// Interpreter is the console's DCL, every process's: each process that
// runs DCL has a Shell of its own (process 1's is home, the others'
// their CLIs'), and the one whose commands run now is embedded, so its
// fields read as the interpreter's (c.Out, c.levels). Console embeds the
// Interpreter.
type Interpreter struct {
	// Shell is the DCL whose commands run now (shell.go): process 1's
	// (home), or a subprocess's while its interpreter runs.
	*Shell

	// machine is the machine DCL runs on: its processes, CPU, and
	// images.
	machine *machine.Machine

	// Grammar is the console's command grammar (console.dcl), which DCL
	// parses each command with (dispatchParsed): DCL's own verbs, bound
	// by bindDCL, and the console's. Nil until the console's first
	// Dispatcher is made.
	Grammar *dcl.Grammar

	// route sends a command line where the console sends one: to the
	// debugger or the interactive assembler when one is reading, else to
	// DCL (DispatchConsole). A procedure's lines and a CALL's go through
	// it, so a procedure can drive the debugger. The console's
	// Dispatcher sets it; until then it is DispatchConsole.
	route func(line string) error

	// line is the command line being dispatched, for a handler that
	// records it (MACRO's SRC header, SET VERIFY).
	line string

	// In is the terminal's byte-level input, from which DCL reads a
	// reply (readReply): INQUIRE, READ SYS$COMMAND, the Ctrl/Y level. It
	// also backs XFC$CONSOLE_READ and (shared with RTL) DECC$GETS/
	// EXE$INPUT/EXE$READ's fid-0 case. It is optional (nil is a legal "no
	// interactive input source" console, matching every Out-only
	// caller/test) -- reads against a nil In report EOF rather than
	// panic.
	In io.Reader

	// control is SET [NO]CONTROL's (dclcontroly.go).
	control controlKeys

	// home is process 1's DCL: the Shell embedded here, but for the while
	// a subprocess's DCL runs, when that is the subprocess's (subcli.go).
	home *Shell

	// clis are the command interpreters of the processes that run one
	// (subcli.go: LIB$SPAWN's, and $CREPRC's of LOGINOUT), by process.
	clis map[*corevms.Environment]*subprocessCLI

	// spawnPage is the S0 pool page SPAWN writes its call to LIB$SPAWN
	// in (spawncmd.go), allocated on first use on each machine.
	spawnPage uint32

	// lending is set while process 1 lends the CPU to the other
	// processes between its DCL commands, and consoleCommands counts the
	// XFC$CONSOLE_CMD calls in progress, during which the machine is
	// mid-instruction and can't run others (dcllend.go).
	lending         bool
	consoleCommands int
}

// Printf writes to the output of the DCL running now.
func (c *Interpreter) Printf(format string, args ...any) {
	fmt.Fprintf(c.Out, format, args...)
}

// RunDownAbandonedImage is Machine.endAbandonedImage in process 1's
// DCL. In a subprocess's it does nothing: the abandoned image is
// process 1's, and its own images are run down by its CLI (subcli.go).
func (c *Interpreter) RunDownAbandonedImage() {
	if c.inSubprocess() {
		return
	}

	c.machine.EndAbandonedImage()
}

// ReadReply reads one line from the console's input, a byte at a time so
// that nothing past the line is taken from a reader shared with the
// command line. ok is false at the end of the input with nothing read,
// or at a Ctrl/Z typed first (the terminal's end of file); a Ctrl/Z
// after text ends the line.
func (c *Interpreter) ReadReply() (line string, ok bool) {
	if c.In == nil {
		return "", false
	}

	var (
		buf  [1]byte
		text []byte
	)

	for {
		n, err := c.In.Read(buf[:])
		if n == 1 {
			switch buf[0] {
			case '\n', '\r':
				return string(text), true
			case ctrlZ:
				return string(text), len(text) > 0
			}

			text = append(text, buf[0])
		}

		if err != nil {
			return string(text), len(text) > 0
		}
	}
}

// ctrlZ is CTRL/Z, the character a terminal user types to end input.
const ctrlZ = 0x1A

// Run is RUN in the DCL running now: process 1's runs the image on the
// machine (Machine.Run); a subprocess's DCL runs it in its process
// (subcli.go).
func (c *Interpreter) Run(fn string, opts machine.RunOptions) error {
	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	// Whatever an earlier image left to chain and nobody ran (one run
	// outside DCL's command loop) is forgotten (dclchain.go).
	c.chained = nil

	if cli := c.subCLI; cli != nil {
		return c.subprocessRun(cli, fn, opts.CommandLine)
	}

	return c.machine.Run(fn, opts)
}

// Running reports whether the console should keep reading commands,
// matching vax.console.running (cleared by QUIT/EXIT.
func (c *Interpreter) Running() bool { return !c.quit }

// Quit implements QUIT/EXIT: stops the command loop.
func (c *Interpreter) Quit() error {
	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	c.quit = true

	return nil
}

// Line is the command line being dispatched, for a handler that records
// it (MACRO's SRC header).
func (c *Interpreter) Line() string { return c.line }

// SetRoute makes route where a procedure's lines and a CALL's are sent:
// the console's routing, to the debugger or the interactive assembler
// when one is reading, else to DCL (DispatchConsole).
func (c *Interpreter) SetRoute(route func(line string) error) { c.route = route }

// Finish ends the session after the command running now: Running
// reports false from then on.
func (c *Interpreter) Finish() { c.quit = true }
