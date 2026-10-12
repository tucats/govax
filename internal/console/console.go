package console

import (
	"io"

	"github.com/tucats/govax/internal/machine"
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

	return &Console{Machine: m, Interpreter: NewInterpreter(m, out)}
}
