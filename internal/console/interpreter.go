package console

import (
	"fmt"
	"io"

	"github.com/tucats/govax/internal/corevms"
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
	machine *Machine

	// dispatchConsole runs one command line in the DCL running now
	// (Dispatcher.DispatchConsole): a subprocess's CLI reads its
	// commands and has them run by it. Nil until the console's first
	// Dispatcher is made.
	dispatchConsole func(line string) error

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

// runDownAbandonedImage is Machine.endAbandonedImage in process 1's
// DCL. In a subprocess's it does nothing: the abandoned image is
// process 1's, and its own images are run down by its CLI (subcli.go).
func (c *Interpreter) runDownAbandonedImage() {
	if c.inSubprocess() {
		return
	}

	c.machine.endAbandonedImage()
}
