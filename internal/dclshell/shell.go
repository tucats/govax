package dclshell

import (
	"io"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
)

// Shell is one process's DCL: the process its commands act for, and the
// state DCL keeps for it (symbols, command levels, $STATUS, open files,
// ...). Process 1's is the console's own (Console.home); each process
// that runs a CLI has one of its own (subcli.go). Console embeds the one
// whose commands run now, so its fields read as the console's: c.RTL,
// c.Out, c.levels.
type Shell struct {
	// RTL is the process the commands act for: Phase 10's SYS$/LIB$
	// calling-convention environment, which also backs the console's
	// cpu.SystemServices (services.go). Process 1's is created fresh
	// alongside the Engine on every Init/Zero/VMINIT (init_vax.go), since
	// it's addressed through the same CPU/Memory pair.
	RTL *corevms.Environment

	// Logicals is the process's logical-name database (internal/lnm,
	// Phase 25). Process 1's is built once, in New, with the standard VMS
	// tables and the process-permanent terminal names, the way a VMS
	// process gets them at login rather than per-INIT, and survives
	// ZERO.
	Logicals *lnm.Database

	// ContainerSession is docs/PHASE-23.md's operator-console state for
	// working with an already-mounted ODS-2 container's files: the
	// current default device/directory (SET DEFAULT/SHOW DEFAULT) a
	// partial file spec typed at the console resolves against, plus (via
	// its own Mounts field, set to the console's *rms.MountTable -- see
	// NewSession's own doc comment) the shared lookup helper
	// DIRECTORY/DELETE/PURGE/COPY/TYPE all need. Named "ContainerSession"
	// rather than "Files" to avoid reading as a collision with
	// internal/rms.Context.Files -- that's a running VAX *program*'s own
	// open-file table (Phase 22), not this default-directory state.
	// Process 1's is built once, in New, and survives an address-space
	// wipe.
	ContainerSession *rms.Session

	// Out is where the commands' output goes: for process 1, the
	// console's terminal stream, which also backs XFC$CONSOLE_WRITE and
	// every other console output path; for another process, its
	// SYS$OUTPUT (subcli.go's cliOutput).
	Out io.Writer

	// subCLI is the process's CLI, nil in process 1's DCL.
	subCLI *subprocessCLI

	// quit is set by QUIT (misc.go), and by LOGOUT in a subprocess; read
	// via Console.Running.
	quit bool

	// Verify is DCL's procedure verification, SET VERIFY's: whether a
	// command procedure's lines are shown as they're read; verifyImage
	// is image verification, whether its data lines are (dclverify.go).
	// verifyPrefix is SET PREFIX's control string, and echo the line a
	// procedure is about to run, until it is shown.
	Verify       bool
	verifyImage  bool
	verifyPrefix string
	echo         *pendingEcho

	// dclSymbols are the DCL symbols assignments at the prompt define
	// (dclsym.go): foreign commands and command abbreviations, global
	// and local.
	dclSymbols dclSymbolTable

	// levels is the input stack (procedure.go): the command levels above
	// the terminal, one for each command procedure being run, innermost
	// last. Each has its own local symbol table in dclSymbols.
	levels []*commandLevel

	// imageRan says the command a procedure just ran was one VMS runs as
	// an image, not one DCL performs itself (dclinput.go): the data lines
	// after it that nothing read are skipped (SKPDAT), where after one of
	// DCL's own commands a data line is read as a command.
	imageRan bool

	// chained is what the image a command ran left for DCL to run next
	// (LIB$DO_COMMAND, LIB$RUN_PROGRAM; dclchain.go), nil for nothing.
	chained *chainedCommand

	// pauseContinued is set when CONTINUE resumes an image LIB$PAUSE
	// paused at command level 0, whose end then leaves $STATUS
	// (dclpause.go).
	pauseContinued bool

	// controlYLevelActive is set while DCL reads the Ctrl/Y level
	// (dclcontroly.go); controlYTaken says a Ctrl/Y stopped the image a
	// procedure's line ran, for the procedure to act on. stopping is set
	// by STOP until every command level has ended, and afterStop is the
	// command typed at the Ctrl/Y level that runs once they have.
	controlYLevelActive *controlYPrompt
	controlYTaken       bool
	stopping            bool
	stopExit            procedureExit
	controlYUnwind      *commandLevel
	afterStop           func() error

	// control is the process's SET [NO]CONTROL settings (dclcontroly.go),
	// which LIB$DISABLE_CTRL and LIB$ENABLE_CTRL change too (libcli.go).
	control controlKeys

	// terminalFlow is the terminal's IF blocks (dclif.go): command level
	// 0's, whose lines cmd/govax's prompt reads.
	terminalFlow flowState

	// status is $STATUS, the last command's condition value, and
	// commandStatus what the command being dispatched says about it
	// (dclstatus.go). statusSet is set each time a command sets $STATUS,
	// for the procedure loop's ON check, and statusHandled when the
	// command acted on it already (branchStatus). statusErr is the error
	// $STATUS came from, if any.
	status        uint32
	statusErr     error
	commandStatus commandStatus
	statusSet     bool
	statusHandled bool

	// endStatus is the status a procedure that ends now (EXIT with no
	// status, or its end of file) ends with, and endStatusErr its error:
	// $STATUS's, except after an /ERROR or /END_OF_FILE branch, which
	// leaves $STATUS the failure's and endStatus CLI$_NORMAL
	// (dclstatus.go).
	endStatus    uint32
	endStatusErr error

	// dclFiles are the files DCL's OPEN has open (dclfile.go), by
	// logical name, and dclOrphans those whose logical names are gone.
	dclFiles   map[string]*dclOpenFile
	dclOrphans []*dclOpenFile

	// searchStreams are F$SEARCH's streams (dcllexfile.go), by stream
	// id: where each search has got to.
	searchStreams map[int32]*searchStream

	// deviceStreams are F$DEVICE's streams (dcllexfile.go), by stream
	// id, 0 for the one used without an id.
	deviceStreams map[int32]*deviceStream

	// processContexts are F$CONTEXT's contexts (dcllexcontext.go), by
	// number, and lastContext the last number given one.
	processContexts map[int32]*processContext
	lastContext     int32
}
