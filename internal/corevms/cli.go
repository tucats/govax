package corevms

import "github.com/tucats/govax/internal/vmserrors"

// CLI is a process's command language interpreter as the LIB$ CLI
// routines reach it (docs/PHASE-57): on VMS they call the CLI through the
// $CLI service, whose request block no manual at hand describes, so govax
// gives them this Go interface instead. dclshell gives one to each
// process whose DCL is govax's (Environment.CLI): process 1, and each
// process LIB$SPAWN creates or $CREPRC creates to run LOGINOUT. A process
// $CREPRC creates to run an image has none, and the routines return
// LIB$_NOCLI there.
//
// Each method is called on the engine's goroutine while the process's
// image runs, so its DCL is waiting for the image: it works on that
// process's own DCL, not the one whose turn it is in the console.
// Statuses are VMS condition values: SS$_NORMAL, or the LIB$ condition
// the routine returns.
type CLI interface {
	// Symbol looks name (uppercase) up as DCL does: the local tables, from
	// the command level that ran the image out, then the global table,
	// taking an abbreviation a symbol allows. It returns the symbol's value
	// as text (an integer's in decimal) and the table it was in
	// (LIB$K_CLI_LOCAL_SYM or LIB$K_CLI_GLOBAL_SYM), or LIB$_NOSUCHSYM.
	Symbol(name string) (value string, table uint32, status uint32)

	// SetSymbol defines name as a string symbol with value, in the local
	// table of the command level that ran the image or in the global
	// table. An "*" in name marks the shortest abbreviation, as in a DCL
	// assignment. LIB$_AMBSYMDEF if an abbreviation of it would also mean
	// another symbol of that table.
	SetSymbol(name, value string, table uint32) uint32

	// DeleteSymbol deletes the symbol name (or an abbreviation of it)
	// means in the table given: LIB$_NOSUCHSYM if there's none.
	DeleteSymbol(name string, table uint32) uint32

	// Chain leaves text for DCL to run once the image now running has
	// exited (LIB$DO_COMMAND and LIB$RUN_PROGRAM): a command line, or,
	// with program set, the file name of an image to run. A second call
	// before the image exits replaces the first.
	Chain(text string, program bool)

	// ControlKeys enables the control keys enable names and disables
	// those disable names (LIB$ENABLE_CTRL and LIB$DISABLE_CTRL: SET
	// CONTROL and SET NOCONTROL), each a mask of CLICtrlT and CLICtrlY
	// bits, and returns the keys enabled before, in the same form. The
	// setting is the DCL's, so it outlasts the image.
	ControlKeys(enable, disable uint32) (old uint32)

	// Pause hands the terminal to DCL with the image paused, as Ctrl/Y
	// does (LIB$PAUSE): DCL reads commands at the Ctrl/Y level until
	// CONTINUE, when it returns SS$_NORMAL for the routine to return.
	// Otherwise (EXIT, STOP, a command that runs an image) the image
	// ends, without its exit handlers: Pause returns ErrExit, or a
	// CallRequest the routine returns as its own, which ends it.
	Pause() (uint32, error)

	// Attach gives the terminal to the process whose PID is pid
	// (LIB$ATTACH, DCL's ATTACH; Environment.AttachTerminal), returning
	// SS$_NORMAL, SS$_NONEXPR, or LIB$_ATTREQREF. The caller then waits
	// for the terminal to come back (Environment.AwaitAttach).
	Attach(pid uint32) uint32
}

// The table-type-indicator values of the LIB$ symbol routines
// ($LIBCLIDEF).
const (
	CLILocalSymbol  = 1 // LIB$K_CLI_LOCAL_SYM
	CLIGlobalSymbol = 2 // LIB$K_CLI_GLOBAL_SYM
)

// The control keys' bits in LIB$ENABLE_CTRL's and LIB$DISABLE_CTRL's
// masks ($LIBCLIDEF): each key's bit is its control character's code.
const (
	CLICtrlT = 1 << 20 // LIB$M_CLI_CTRLT
	CLICtrlY = 1 << 25 // LIB$M_CLI_CTRLY
)

const cliGetSymbol = 0x1305

// cliUndefinedSymbol is CLI$_UNDSYM.
const cliUndefinedSymbol = 0x38140

// ErrHalt is returned by a ServiceFunc/ShimFunc to request that the machine
// halt. Kept as a package-local sentinel rather than internal/cpu's
// ErrHalted so this package doesn't need to import internal/cpu — whatever
// wires an Environment into internal/cpu.SystemServices (internal/console)
// translates this into cpu.ErrHalted.
var ErrHalt = vmserrors.New(vmserrors.LIB_HALT)

// serviceSysCli is SYS$CLI: dispatches one fixed-format callback request.
// Every request other than "get symbol" is unimplemented in the C source
// itself (a single default case lists over a dozen request codes as
// comments, none actually handled), replicated as-is: an unrecognized
// request halts the machine.
func serviceSysCli(env *Environment, argv []uint32) (uint32, error) {
	reqAddr := argv[0]

	request, err := env.mem.LoadByte(env.cpu, reqAddr)
	if err != nil {
		return ssAccVio, nil
	}

	subrequest, err := env.mem.LoadByte(env.cpu, reqAddr+1)
	if err != nil {
		return ssAccVio, nil
	}

	length, err := env.mem.LoadLongword(env.cpu, reqAddr+4)
	if err != nil {
		return ssAccVio, nil
	}

	ptr, err := env.mem.LoadLongword(env.cpu, reqAddr+8)
	if err != nil {
		return ssAccVio, nil
	}

	reqword := uint32(subrequest)<<8 | uint32(request)

	switch reqword {
	case cliGetSymbol:
		if _, err := loadString(env, ptr, int(length)); err != nil {
			return ssAccVio, nil
		}

		return cliUndefinedSymbol, nil

	default:
		return ssInvArg, ErrHalt
	}
}

func registerCLIService(t *ServiceTable) {
	t.Register("SYS$CLI", serviceSysCli)
}
