package console

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The subprocess CLI (docs/PHASE-48.md, Decision 8 in docs/PHASE-43.md;
// since docs/PHASE-50's subtask 19, the console's whole DCL).
//
// A process LIB$SPAWN creates, or $CREPRC creates to run LOGINOUT.EXE,
// runs a command interpreter rather than one image. On VMS that is DCL;
// in govax it is the console's own DCL: the same grammar, the same
// commands, command procedures with their levels, symbols, statuses, IF
// and GOTO, OPEN and READ, and so on, each process with its own DCL
// state (its symbols, $STATUS, command levels, default directory,
// logical names, open files). Only the commands that work the machine
// itself (DEBUG, ASM, INITIALIZE/VAX, VMINIT, SPAWN, ...) are process
// 1's (processOneCommands). A line may start with "$"; with LOGOUT, EXIT
// at its command level 0, or the end of its input, the process logs out.
//
// # How it runs
//
// The CLI is Go, but it runs in the process, as a procedure in S0 (the
// "CLI stub"), which the process's startup calls as it would call an
// image's IMAGE$INIT driver:
//
//	stub:   .WORD   0                   ; entry mask
//	loop:   MOVL    #cliShimCode, R0
//	        XFC     #XFC$SHIM           ; EXE$CLI_COMMAND: the CLI, in Go
//	        PUSHL   R0                  ; the CLI's final status
//	        CALLS   #1, @#SYS$EXIT      ; logs out: deletes the process
//	        RET
//	args:   .LONG   0                   ; an empty argument list
//
// The interpreter itself runs on a goroutine of its own (runInterpreter),
// which takes turns with the engine: the shim hands it the CPU (resume),
// and waits until it hands it back (a cliYield), so only one of the two
// ever runs. While the interpreter runs, the console's DCL state is the
// process's (swapDCLState), and the engine is stopped in the shim. The
// interpreter hands the CPU back when it must wait (for a command line
// that hasn't come yet, from the terminal or a mailbox: the shim returns
// corevms.ErrWait, the process waits in the scheduler, and the XFC runs
// again later), when a command runs an image, and when it logs out.
//
// An image a command runs (RUN, MCR, a foreign command; subprocessRun)
// is activated in the process's P0, and the shim asks the engine to call
// its IMAGE$INIT driver as an image's outermost call (corevms.
// CallRequest's Image): the frame's saved PC and FP are the console's
// sentinel, so the image's $EXIT, or its return, ends the image with
// ErrConsoleCallReturned rather than returning into the CLI. StepMachine
// then hands the process back to the CLI (endCLIImage): the image is run
// down, as DCL runs one down, and the registers the CLI had at the XFC
// are put back, with PC at the MOVL, so the shim runs again and resumes
// the interpreter with the image's status, in the middle of the command
// (or command procedure) that ran it. When the interpreter has logged
// out, the shim returns its final status, and the stub logs out with it.
//
// What the interpreter writes goes to the process's SYS$OUTPUT (the
// terminal, a mailbox, NL:, a file), a line at a time (cliOutput); a
// line a full mailbox can't take yet is kept, and written before the
// interpreter runs again.
//
// The CLI's DCL symbols are its own, a copy of its parent's when
// LIB$SPAWN copies them (InheritSymbols); its $STATUS is the status of
// the last command, which becomes the process's final status.
//
// LOGINOUT's CLI (a $CREPRC of LOGINOUT.EXE; corevms.CLIStartup's Login)
// logs a job in, and does two things a spawned one doesn't, as VMS's
// DCL did in testdata/mp/probe4's run: reading its commands from a file
// or a mailbox, it echoes each line as it reads it (DCL's verify, on in
// a job that isn't interactive); and when it logs out, it writes
// LOGOUT's report (corevms's LogoutReport).

// cliShimCode is the XFC$SHIM code of EXE$CLI_COMMAND, the CLI's shim.
// It's govax's own, so it takes a code no LIBRTL routine or kernel.asm
// shim uses.
const cliShimCode = 64

// The CLI stub's layout (see above): the MOVL that starts the loop, and
// the empty argument list each image's driver is called with.
const (
	cliStubLoop    = 2
	cliStubArgList = 24
)

// ssUnsupported is SS$_UNSUPPORTED, the shim's status in a process that
// has no CLI; cliStatusImageFNF is CLI$_IMAGEFNF, RUN's for an image
// that isn't there.
var (
	ssUnsupported     = vmsdef.Symbols["SS$_UNSUPPORTED"]
	cliStatusImageFNF = vmsdef.LibrarySymbols["CLI$_IMAGEFNF"]
)

// processOneCommands are the console commands, by the grammar's name for
// the syntax that runs them (dcl.Result.Active), that work the machine
// itself or process 1's own session: a subprocess's DCL refuses them
// (VAX_NOTSUBPROC). SPAWN is among them for now: a subprocess's own
// subprocess would need its CLI to wait for it.
var processOneCommands = map[string]bool{
	"ASM": true, "ASSEMBLE": true, "BOOT": true,
	"CLEAR": true, "CLEAR_MEMORY": true, "CLEAR_STRINGS": true,
	"DEBUG": true, "INITIALIZE_VAX": true, "LOAD": true, "ROM": true,
	"SAVE": true, "SET_QUANTUM": true, "SPAWN": true, "TEST": true,
	"VMINIT": true, "ZERO": true,
}

// dclState is one process's DCL state: what the console's commands read
// and change as "the process", which swapDCLState exchanges with the
// console's while a subprocess's interpreter runs. Process 1's lives in
// the Console's own fields.
type dclState struct {
	rtl         *corevms.Environment
	logicals    *lnm.Database
	session     *rms.Session
	out         io.Writer
	cli         *subprocessCLI
	quit        bool
	verify      bool
	verifyImage bool
	prefix      string
	echo        *pendingEcho
	imageActive bool
	runHost     bool
	symbols     dclSymbolTable
	levels      []*commandLevel
	imageRan    bool

	controlYLevelActive *controlYPrompt
	controlYTaken       bool
	stopping            bool
	stopExit            procedureExit
	controlYUnwind      *commandLevel
	afterStop           func() error

	terminalFlow  flowState
	status        uint32
	statusErr     error
	commandStatus commandStatus
	statusSet     bool
	statusHandled bool
	endStatus     uint32
	endStatusErr  error
	dclFiles      map[string]*dclOpenFile
	dclOrphans    []*dclOpenFile
	searchStreams map[int32]*searchStream
	imageStatus   uint32
	imageEnded    bool
}

// swapDCLState exchanges the console's DCL state with s: the call that
// puts a subprocess's state in place, and, again, the one that puts
// process 1's back.
func (c *Console) swapDCLState(s *dclState) {
	c.RTL, s.rtl = s.rtl, c.RTL
	c.Logicals, s.logicals = s.logicals, c.Logicals
	c.ContainerSession, s.session = s.session, c.ContainerSession
	c.Out, s.out = s.out, c.Out
	c.subCLI, s.cli = s.cli, c.subCLI
	c.quit, s.quit = s.quit, c.quit
	c.Verify, s.verify = s.verify, c.Verify
	c.verifyImage, s.verifyImage = s.verifyImage, c.verifyImage
	c.verifyPrefix, s.prefix = s.prefix, c.verifyPrefix
	c.echo, s.echo = s.echo, c.echo
	c.imageActive, s.imageActive = s.imageActive, c.imageActive
	c.runHost, s.runHost = s.runHost, c.runHost
	c.dclSymbols, s.symbols = s.symbols, c.dclSymbols
	c.levels, s.levels = s.levels, c.levels
	c.imageRan, s.imageRan = s.imageRan, c.imageRan
	c.controlYLevelActive, s.controlYLevelActive = s.controlYLevelActive, c.controlYLevelActive
	c.controlYTaken, s.controlYTaken = s.controlYTaken, c.controlYTaken
	c.stopping, s.stopping = s.stopping, c.stopping
	c.stopExit, s.stopExit = s.stopExit, c.stopExit
	c.controlYUnwind, s.controlYUnwind = s.controlYUnwind, c.controlYUnwind
	c.afterStop, s.afterStop = s.afterStop, c.afterStop
	c.terminalFlow, s.terminalFlow = s.terminalFlow, c.terminalFlow
	c.status, s.status = s.status, c.status
	c.statusErr, s.statusErr = s.statusErr, c.statusErr
	c.commandStatus, s.commandStatus = s.commandStatus, c.commandStatus
	c.statusSet, s.statusSet = s.statusSet, c.statusSet
	c.statusHandled, s.statusHandled = s.statusHandled, c.statusHandled
	c.endStatus, s.endStatus = s.endStatus, c.endStatus
	c.endStatusErr, s.endStatusErr = s.endStatusErr, c.endStatusErr
	c.dclFiles, s.dclFiles = s.dclFiles, c.dclFiles
	c.dclOrphans, s.dclOrphans = s.dclOrphans, c.dclOrphans
	c.searchStreams, s.searchStreams = s.searchStreams, c.searchStreams
	c.imageStatus, s.imageStatus = s.imageStatus, c.imageStatus
	c.imageEnded, s.imageEnded = s.imageEnded, c.imageEnded
}

// inSubprocess reports whether the DCL running now is a subprocess's.
func (c *Console) inSubprocess() bool { return c.subCLI != nil }

// processOneOnly is VAX_NOTSUBPROC for what (a command, or a way of
// running the machine) in a subprocess's DCL, and nil in process 1's.
func (c *Console) processOneOnly(what string) error {
	if c.inSubprocess() {
		return vmserrors.New(vmserrors.VAX_NOTSUBPROC, what)
	}

	return nil
}

// The ways the interpreter hands the CPU back to the shim (cliYield's
// kind).
const (
	yieldWait   = iota // its input isn't there yet (err): run again later
	yieldImage         // a command runs an image: call driver
	yieldLogout        // it has logged out, with status
)

// cliYield is what the interpreter asks of the shim when it hands the
// CPU back.
type cliYield struct {
	kind   int
	err    error
	driver uint32
	status uint32
}

// errCLIAborted is the panic that unwinds an interpreter whose process
// was deleted while it waited for its turn.
var errCLIAborted = errors.New("command interpreter's process deleted")

// subprocessCLI is one process's CLI.
type subprocessCLI struct {
	c   *Console
	env *corevms.Environment

	// state is its DCL state while process 1's is the console's
	// (swapDCLState).
	state dclState

	// command is the one command LIB$SPAWN gave it to run before it
	// logs out, if hasCommand; commandDone is set once it has been read.
	command                 string
	hasCommand, commandDone bool

	// prompt is written before each command read from the terminal.
	prompt string

	// login is set for LOGINOUT's CLI (see this file's opening comment);
	// reported is set once it has written LOGOUT's report; logoutCommand
	// once a LOGOUT command has logged it out.
	login, reported, logoutCommand bool

	// input is its SYS$INPUT, opened at the first read.
	input corevms.CommandInput

	// stub is the address of its CLI stub.
	stub uint32

	// status is its final status, once it has logged out.
	status uint32

	// output are lines it has still to write to SYS$OUTPUT; partial is
	// the start of a line that has no newline yet.
	output  []string
	partial string

	// started is set once its interpreter's goroutine has started, and
	// finished once it has logged out. resume hands the goroutine the
	// CPU (true: its process has been deleted, and it is to unwind), and
	// yield hands it back; yielded is the last request, not yet acted on.
	started, finished bool
	resume            chan bool
	yield             chan cliYield
	yielded           *cliYield

	// imageActive is set while an image a command started runs; fp and
	// ap are the CLI's registers at the shim's XFC, put back when it
	// ends; imageExit is the image's status, for the command that ran
	// it.
	imageActive bool
	fp, ap      uint32
	imageExit   uint32
}

// cliOf returns env's CLI, made empty the first time.
func (c *Console) cliOf(env *corevms.Environment) *subprocessCLI {
	if c.clis == nil {
		c.clis = map[*corevms.Environment]*subprocessCLI{}
	}

	cli := c.clis[env]
	if cli == nil {
		cli = &subprocessCLI{c: c, env: env}
		cli.state = dclState{rtl: env, logicals: env.Logicals, session: env.Session, out: cliOutput{cli}, cli: cli}
		c.clis[env] = cli
	}

	return cli
}

// cliHost makes the console the System's corevms.CommandInterpreter.
type cliHost struct{ c *Console }

// InheritSymbols gives child a copy of parent's DCL symbols: the
// console's own for process 1, otherwise parent's CLI's.
func (h cliHost) InheritSymbols(parent, child *corevms.Environment) {
	var from *dclSymbolTable

	if parent == h.c.processOne {
		from = &h.c.dclSymbols
	} else if cli := h.c.clis[parent]; cli != nil {
		from = &cli.state.symbols
	} else {
		from = &dclSymbolTable{}
	}

	h.c.cliOf(child).state.symbols = from.clone()
}

// Start starts env's CLI (corevms.CommandInterpreter): it writes the
// CLI stub in a pool page of the process's own, and returns its address
// for the process's startup to call.
func (h cliHost) Start(env *corevms.Environment, start *corevms.CLIStartup) (uint32, error) {
	c := h.c

	if err := c.ensureShims(); err != nil {
		return 0, err
	}

	cli := c.cliOf(env)
	cli.command, cli.hasCommand = start.Command, start.Command != ""
	cli.prompt = start.Prompt
	cli.login = start.Login

	if cli.prompt == "" {
		cli.prompt = "$ "
	}

	// A spawned subprocess's DCL starts with its parent's verification
	// (VMS 7.3 showed a spawned procedure's lines after the parent's SET
	// VERIFY, testdata/dcl50's round 5).
	if !cli.login {
		cli.inheritVerify(env.Process.Owner)
	}

	p := c.imagesOf(env)

	exit, ok := p.p1Stub("SYS$EXIT")
	if !ok {
		return 0, fmt.Errorf("console: no SYS$EXIT stub for a command interpreter")
	}

	page, err := env.AllocateS0(1, env.Process.PID, "CLI")
	if err != nil {
		return 0, err
	}

	code := []byte{
		0x00, 0x00, // entry mask
		0xD0, 0x8F, // MOVL I^#cliShimCode, R0
		byte(cliShimCode), byte(cliShimCode >> 8), byte(cliShimCode >> 16), byte(cliShimCode >> 24),
		0x50,
		0xFC, 0x7D, // XFC #XFC$SHIM
		0xDD, 0x50, // PUSHL R0
	}
	code = append(code, encodeCalls(1, exit)...)
	code = append(code, 0x04) // RET

	for len(code) < cliStubArgList+4 {
		code = append(code, 0) // ... and the empty argument list
	}

	if err := p.storeBytes(page, code); err != nil {
		return 0, err
	}

	cli.stub = page

	return page, nil
}

// inheritVerify gives the CLI the verification of its parent, the
// process whose PID is owner: process 1's (the console's own) or another
// CLI's.
func (cli *subprocessCLI) inheritVerify(owner uint32) {
	c := cli.c

	switch parent := c.clis[c.findEnvironment(owner)]; {
	case c.processOne != nil && owner == c.processOne.Process.PID:
		cli.state.verify, cli.state.verifyImage, cli.state.prefix = c.Verify, c.verifyImage, c.verifyPrefix
	case parent != nil:
		cli.state.verify, cli.state.verifyImage, cli.state.prefix = parent.state.verify, parent.state.verifyImage, parent.state.prefix
	}
}

// findEnvironment returns the process whose PID is pid, nil if there is
// none.
func (c *Console) findEnvironment(pid uint32) *corevms.Environment {
	if c.processOne == nil {
		return nil
	}

	env, _ := c.processOne.FindProcess(pid)

	return env
}

// cliCommand is EXE$CLI_COMMAND, the CLI stub's shim (see this file's
// opening comment): it lets the interpreter run until it hands the CPU
// back, then does what it asks: wait, call an image, or end, returning
// the CLI's final status for the stub to log out with.
func (c *Console) cliCommand(env *corevms.Environment, _ []uint32) (uint32, error) {
	cli := c.clis[env]
	if cli == nil || cli.stub == 0 {
		return ssUnsupported, nil // not a CLI's process
	}

	for {
		// What the interpreter wrote goes out first, so its messages
		// come before the image it starts; a full mailbox makes the
		// shim wait here and come back.
		if err := cli.flush(); err != nil {
			return 0, err
		}

		y := cli.yielded
		if y == nil {
			y = cli.run()
			cli.yielded = y

			continue
		}

		switch y.kind {
		case yieldWait:
			cli.yielded = nil

			return 0, y.err

		case yieldImage:
			cli.yielded = nil
			cli.imageActive = true
			cli.fp, cli.ap = c.CPU.GPR(vax.FP), c.CPU.GPR(vax.AP)

			return 0, &corevms.CallRequest{Routine: y.driver, ArgList: cli.stub + cliStubArgList, Image: true}
		}

		// Logged out: LOGINOUT's report, or a spawned process's one line
		// for LOGOUT (VMS 7.3, testdata/mp/probe4 and probe5), then the
		// final status.
		switch {
		case cli.login && !cli.reported && cli.input != nil:
			cli.reported = true
			cli.output = append(cli.output, cli.env.LogoutReport(cli.input.Interactive())...)

			continue

		case !cli.login && cli.logoutCommand && !cli.reported:
			cli.reported = true
			cli.output = append(cli.output, cli.env.SubprocessLogoutLine())

			continue
		}

		return y.status, nil
	}
}

// run hands the interpreter the CPU, starting it the first time, with
// the process's DCL state the console's, and returns what it asks for
// when it hands the CPU back.
func (cli *subprocessCLI) run() *cliYield {
	c := cli.c

	if cli.finished {
		return &cliYield{kind: yieldLogout, status: cli.status}
	}

	c.swapDCLState(&cli.state)
	defer c.swapDCLState(&cli.state)

	if !cli.started {
		cli.started = true
		cli.resume = make(chan bool)
		cli.yield = make(chan cliYield)

		go cli.runInterpreter()
	}

	cli.resume <- false
	y := <-cli.yield

	return &y
}

// handBack hands the CPU back to the shim with y, and waits for its next
// turn; it unwinds the interpreter (errCLIAborted) if the process has
// been deleted meanwhile. Called on the interpreter's goroutine.
func (cli *subprocessCLI) handBack(y cliYield) {
	cli.yield <- y

	if abort := <-cli.resume; abort {
		panic(errCLIAborted)
	}
}

// abort ends the interpreter of a process being deleted while it waits
// for its turn: it unwinds, with the process's DCL state the console's,
// so that what it leaves behind (command levels, /OUTPUT files) is its
// own.
func (cli *subprocessCLI) abort() {
	if !cli.started || cli.finished {
		return
	}

	c := cli.c

	c.swapDCLState(&cli.state)
	defer c.swapDCLState(&cli.state)

	cli.resume <- true
	<-cli.yield
}

// runInterpreter is the interpreter's goroutine: DCL's command level 0
// for the process, reading command lines (nextCommand) and dispatching
// each as the console's own are dispatched, until the process logs out.
func (cli *subprocessCLI) runInterpreter() {
	c := cli.c

	defer func() {
		if r := recover(); r != nil && r != errCLIAborted { //nolint:errorlint // the sentinel itself
			panic(r)
		}

		cli.finished = true
		cli.yield <- cliYield{kind: yieldLogout, status: cli.status}
	}()

	<-cli.resume

	c.setStatus(ssNormal, nil)

	for c.Running() {
		line, ok := cli.nextCommand()
		if !ok {
			break
		}

		// A job that isn't interactive has DCL's verify on: each line
		// is shown as it is read, and a procedure's lines too.
		if cli.login && !cli.input.Interactive() {
			c.Verify = true

			fmt.Fprintln(c.Out, line)
		}

		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "$"))

		var err error

		if c.Dispatcher == nil {
			err = vmserrors.New(vmserrors.CLI_NEEDRTL, "DCL")
		} else {
			err = c.Dispatcher.DispatchConsole(line)
		}

		if err != nil && !vmserrors.MessageInhibited(err) {
			fmt.Fprintln(c.Out, "%"+err.Error())
		}
	}

	c.closeDCLFiles()

	if cli.status == 0 {
		cli.status = c.status
	}
}

// nextCommand is the CLI's next command line, waiting for it (handing
// the CPU back) while it hasn't come; ok is false at the end of the
// input, when the CLI logs out (a SYS$INPUT that can't be opened has no
// lines, and its status becomes the CLI's).
func (cli *subprocessCLI) nextCommand() (string, bool) {
	for {
		line, ok, err := cli.nextLine()
		if err == nil {
			return line, ok
		}

		cli.handBack(cliYield{kind: yieldWait, err: err})
	}
}

// nextLine is the CLI's next command: LIB$SPAWN's one command, after
// which there are no more; otherwise the next line of SYS$INPUT, opened
// the first time.
func (cli *subprocessCLI) nextLine() (string, bool, error) {
	if cli.hasCommand {
		if cli.commandDone {
			return "", false, nil
		}

		cli.commandDone = true

		return cli.command, true, nil
	}

	if cli.input == nil {
		in, st := cli.env.OpenCommandInput()
		if in == nil {
			cli.status = st

			return "", false, nil
		}

		cli.input = in
	}

	prompt := ""
	if cli.input.Interactive() {
		prompt = cli.prompt
	}

	return cli.input.ReadLine(prompt)
}

// cliOutput is a subprocess's DCL output (the console's Out while its
// interpreter runs): its SYS$OUTPUT, a line at a time.
type cliOutput struct{ cli *subprocessCLI }

func (o cliOutput) Write(p []byte) (int, error) {
	o.cli.printf("%s", p)

	// Written at once where it can be, so it comes before a prompt the
	// next read writes; what a full mailbox can't take waits for the
	// shim.
	_ = o.cli.flush()

	return len(p), nil
}

// printf queues text for SYS$OUTPUT a line at a time, keeping the start
// of a line that has no newline yet.
func (cli *subprocessCLI) printf(format string, args ...any) {
	text := cli.partial + fmt.Sprintf(format, args...)

	for {
		line, rest, found := strings.Cut(text, "\n")
		if !found {
			cli.partial = text

			return
		}

		cli.output = append(cli.output, line)
		text = rest
	}
}

// flush writes the lines queued for SYS$OUTPUT (corevms's PutOutput:
// the terminal, a mailbox, or NL:), returning ErrWait, with the rest
// kept, while a mailbox is full.
func (cli *subprocessCLI) flush() error {
	for len(cli.output) > 0 {
		if _, err := cli.env.PutOutput(cli.output[0]); err != nil {
			return err
		}

		cli.output = cli.output[1:]
	}

	return nil
}

// subprocessRun is RUN (and MCR, and a foreign command) in a
// subprocess's DCL: the image is activated in the process, and the
// interpreter hands the CPU back for the engine to run it, getting it
// back when the image has ended (endCLIImage). The image's status is the
// command's; a failure's message is shown unless the image showed it
// (STS$M_INHIB_MSG).
func (c *Console) subprocessRun(cli *subprocessCLI, image, commandLine string) error {
	driver, status := c.cliActivate(cli, image, commandLine)
	if driver == 0 {
		c.setCommandStatus(status, nil)

		return vmserrors.InhibitMessage(c.statusFailure(status))
	}

	cli.handBack(cliYield{kind: yieldImage, driver: driver})

	status = cli.imageExit
	c.setCommandStatus(status, nil)

	if status&1 != 0 {
		return nil
	}

	return c.statusFailure(status)
}

// cliActivate activates image in the CLI's process, with commandLine as
// what LIB$GET_FOREIGN returns, and returns its IMAGE$INIT driver's
// address; or 0 and the failure's status, its messages shown as VMS's
// DCL shows them.
func (c *Console) cliActivate(cli *subprocessCLI, image, commandLine string) (uint32, uint32) {
	env := cli.env

	driver, err := c.activateCreatedImage(env, image, false)
	if err != nil {
		say := func(format string, args ...any) { fmt.Fprintf(c.Out, format+"\n", args...) }

		say("%%DCL-W-ACTIMAGE, error activating image %s", strings.ToUpper(image))
		c.traceCLI(cli, "can't run %s: %v", image, err)

		// The status has STS$M_INHIB_MSG set, its message having been
		// shown, as VMS's DCL returned it (testdata/mp/probe4).
		status := corevms.StartupStatus(err)

		switch found, ok := c.foundImageFile(env, image); {
		case status == rmsFNF:
			status = cliStatusImageFNF
			say("-CLI-E-IMAGEFNF, image file not found %s", c.imageFileSpec(env, image))

		case ok && !fixupFailed(err):
			// The file is there but isn't an image: VMS 7.3 named it and
			// blamed its header (testdata/mp/probe5/vax, step 11).
			status = imgactBadHdr
			say("-CLI-E-IMGNAME, image file %s", found)
			say("-IMGACT-F-BADHDR, an error was discovered in the image header")

		default:
			say("-%s", strings.TrimPrefix(env.StatusText(status), "%"))
		}

		return 0, status | stsInhibitMsg
	}

	env.CommandLine = commandLine
	c.traceCLI(cli, "runs %s", image)

	return driver, 0
}

// logoutCommand is LOGOUT: in a subprocess, it logs the process out,
// which says so (a spawned process's one line); in process 1 it is QUIT.
// It leaves $STATUS as it was, the process's final status.
func (c *Console) logoutCommand() error {
	c.keepStatus()

	if cli := c.subCLI; cli != nil {
		cli.logoutCommand = true
		c.quit = true

		return nil
	}

	return c.Quit()
}

// endCLIImage ends the image a CLI's command ran (see this file's
// opening comment): its status, in R0, is kept for the command, the
// image is run down, and the CLI's registers are put back, at the start
// of its loop, so the shim resumes the interpreter.
func (c *Console) endCLIImage(cli *subprocessCLI) {
	env := cli.env
	status := c.CPU.GPR(vax.R0)

	cli.imageActive = false
	cli.imageExit = status
	c.traceCLI(cli, "image ends, status %08X", status)

	// Image rundown, as for process 1's images (imageRundown): the
	// user-mode logical names in the process table, then everything
	// corevms keeps per image.
	_, _ = env.Logicals.Delete(lnm.ProcessTableName, "", lnm.User)
	env.ImageRundown()
	env.CommandLine = ""

	c.CPU.SetGPR(vax.FP, cli.fp)
	c.CPU.SetGPR(vax.AP, cli.ap)
	c.CPU.SetGPR(vax.PC, cli.stub+cliStubLoop)
}

// rmsFNF is RMS$_FNF, the status corevms gives an image that isn't
// there.
var rmsFNF = vmsdef.Symbols["RMS$_FNF"]

// fixupFailed reports whether an activation failed fixing up an image
// that loaded (vmserrors.CLI_FIXUP), rather than loading it.
func fixupFailed(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if ve, ok := e.(vmserrors.VMSError); ok && ve.Status == vmserrors.CLI_FIXUP {
			return true
		}
	}

	return false
}

// imgactBadHdr is IMGACT$_BADHDR, the image activator's status for a
// file whose image header is wrong (VMS 7.3's, %X004D8C84; govax's
// tables have no IMGACT$ facility).
const imgactBadHdr = 0x004D8C84

// foundImageFile is the full name, version and all, of the volume file
// RUN image found (with the default type .EXE), and true; or false when
// there is none or it's a host file.
func (c *Console) foundImageFile(env *corevms.Environment, image string) (string, bool) {
	s := env.Session
	if s == nil {
		return "", false
	}

	loc, err := s.Locate(image, false)
	if err != nil || loc.Host {
		return "", false
	}

	_, found, err := s.ReadRawFile(withDefaultType(loc, "EXE"))
	if err != nil {
		return "", false
	}

	return found.Name, true
}

// imageFileSpec is image as RMS expands it, with the default type .EXE,
// for DCL's IMAGEFNF message ("DUA0:[000000]NOSUCH.EXE;"); as typed,
// with .EXE;, when it doesn't name a volume file.
func (c *Console) imageFileSpec(env *corevms.Environment, image string) string {
	name := strings.ToUpper(image)

	if s := env.Session; s != nil {
		if loc, err := s.Locate(image, false); err == nil && !loc.Host {
			if spec, err := s.ExpandName(withDefaultType(loc, "EXE").Name); err == nil {
				return spec
			}
		}
	}

	if !strings.Contains(name[strings.LastIndexAny(name, ":]>")+1:], ".") {
		name += ".EXE"
	}

	return name + ";"
}

// traceCLI writes a DEBUG(PROCESS) line about cli's process, when that
// debug flag is on.
func (c *Console) traceCLI(cli *subprocessCLI, format string, args ...any) {
	if c.CPU.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(c.CPU.DebugWriter(), "DEBUG(PROCESS): %08X CLI %s\n", cli.env.Process.PID, fmt.Sprintf(format, args...))
	}
}

// cliImageEnded is StepMachine's step when env, a process other than
// process 1, has returned from its outermost call: if that was an image
// its CLI ran, the CLI goes on (true); otherwise the process has ended.
func (c *Console) cliImageEnded(env *corevms.Environment) bool {
	cli := c.clis[env]
	if cli == nil || !cli.imageActive {
		return false
	}

	c.endCLIImage(cli)

	return true
}
