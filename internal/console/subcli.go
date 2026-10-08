package console

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// The subprocess CLI (docs/PHASE-48.md, Decision 8 in docs/PHASE-43.md).
//
// A process LIB$SPAWN creates, or $CREPRC creates to run LOGINOUT.EXE,
// runs a command interpreter rather than one image. On VMS that is DCL;
// in govax it's this small interpreter, which understands the commands
// that run images:
//
//	RUN file                    run an image
//	MCR image [text]            run an image, with text for LIB$GET_FOREIGN
//	name [text]                 a foreign command: a DCL symbol whose
//	                            value is "$image"; or an alias, whose
//	                            value replaces the word
//	name :== value (and :=, ==, =)   define a DCL symbol
//	DELETE/SYMBOL name          remove one
//	EXIT [status], LOGOUT       log out
//
// Anything else is an unrecognized verb (DCL's %DCL-W-IVVERB). A line may
// start with "$", as a command procedure's lines do; "!" starts a comment.
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
// Each time the shim runs, it reads the next command (from the
// command-string LIB$SPAWN gave, or from SYS$INPUT; corevms's
// CommandInput) and carries it out. A command that runs an image has it
// activated in the process's P0, then the shim asks the engine to call
// the image's IMAGE$INIT driver as an image's outermost call
// (corevms.CallRequest's Image): the frame's saved PC and FP are the
// console's sentinel, so the image's $EXIT, or its return, ends the
// image with ErrConsoleCallReturned rather than returning into the CLI.
// StepMachine then hands the process back to the CLI (endCLIImage): the
// image is run down, as DCL runs one down, and the registers the CLI
// had at the XFC are put back, with PC at the MOVL, so the shim runs
// again and reads the next command. When there are no more (the end of
// the input, or LOGOUT), the shim returns the CLI's status, and the stub
// logs out with it.
//
// Waiting fits the engine's usual model: a command line that hasn't come
// yet (the terminal's, a mailbox's) makes the shim return corevms.ErrWait,
// the process waits in the scheduler, and the XFC runs again later. So
// do lines the CLI writes to a SYS$OUTPUT that's a full mailbox; they're
// kept until written.
//
// The CLI's DCL symbols are its own, a copy of its parent's when
// LIB$SPAWN copies them (InheritSymbols); its $STATUS is the status of
// the last command, which becomes the process's final status.

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

// cliStatusIVVERB is CLI$_IVVERB, DCL's status for an unrecognized
// command verb; cliStatusInsfprm is CLI$_INSFPRM, for a command missing
// a parameter it requires.
var (
	ssNormal         = vmsdef.Symbols["SS$_NORMAL"]
	ssUnsupported    = vmsdef.Symbols["SS$_UNSUPPORTED"]
	cliStatusIVVERB  = vmsdef.LibrarySymbols["CLI$_IVVERB"]
	cliStatusInsfprm = vmsdef.LibrarySymbols["CLI$_INSFPRM"]
)

// subprocessCLI is one process's CLI.
type subprocessCLI struct {
	env *corevms.Environment

	// symbols are its DCL symbols.
	symbols dclSymbols

	// command is the one command LIB$SPAWN gave it to run before it
	// logs out, if hasCommand; commandDone is set once it has been read.
	command                 string
	hasCommand, commandDone bool

	// prompt is written before each command read from the terminal.
	prompt string

	// input is its SYS$INPUT, opened at the first read.
	input corevms.CommandInput

	// stub is the address of its CLI stub.
	stub uint32

	// status is its $STATUS: the last command's status.
	status uint32

	// output are lines it has still to write to SYS$OUTPUT.
	output []string

	// loggedOut is set when it has no more commands to run.
	loggedOut bool

	// imageActive is set while an image a command started runs; fp and
	// ap are the CLI's registers at the shim's XFC, put back when it
	// ends.
	imageActive bool
	fp, ap      uint32

	// depth counts alias substitutions in the command being run.
	depth int
}

// cliOf returns env's CLI, made empty the first time.
func (c *Console) cliOf(env *corevms.Environment) *subprocessCLI {
	if c.clis == nil {
		c.clis = map[*corevms.Environment]*subprocessCLI{}
	}

	cli := c.clis[env]
	if cli == nil {
		cli = &subprocessCLI{env: env, status: ssNormal}
		c.clis[env] = cli
	}

	return cli
}

// cliHost makes the console the System's corevms.CommandInterpreter.
type cliHost struct{ c *Console }

// InheritSymbols gives child a copy of parent's DCL symbols: the
// console's own for process 1, otherwise parent's CLI's.
func (h cliHost) InheritSymbols(parent, child *corevms.Environment) {
	var from dclSymbols

	if parent == h.c.RTL {
		from = h.c.dclSymbols
	} else if cli := h.c.clis[parent]; cli != nil {
		from = cli.symbols
	}

	h.c.cliOf(child).symbols = from.clone()
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

	if cli.prompt == "" {
		cli.prompt = "$ "
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

// cliCommand is EXE$CLI_COMMAND, the CLI stub's shim (see this file's
// opening comment): it carries out commands until one runs an image,
// which it asks the engine to call, or there are none left, when it
// returns the CLI's final status for the stub to log out with.
func (c *Console) cliCommand(env *corevms.Environment, _ []uint32) (uint32, error) {
	cli := c.clis[env]
	if cli == nil || cli.stub == 0 {
		return ssUnsupported, nil // not a CLI's process
	}

	for {
		if err := cli.flush(); err != nil {
			return 0, err // a full mailbox: called again
		}

		if cli.loggedOut {
			return cli.status, nil
		}

		line, ok, err := cli.nextLine()
		if err != nil {
			return 0, err // waiting for the line: called again
		}

		if !ok {
			cli.loggedOut = true

			continue
		}

		cli.depth = 0

		driver := c.cliExecute(cli, line)
		if driver == 0 {
			continue
		}

		cli.imageActive = true
		cli.fp, cli.ap = c.CPU.GPR(vax.FP), c.CPU.GPR(vax.AP)

		return 0, &corevms.CallRequest{Routine: driver, ArgList: cli.stub + cliStubArgList, Image: true}
	}
}

// nextLine is the CLI's next command: LIB$SPAWN's one command, after
// which there are no more; otherwise the next line of SYS$INPUT, opened
// the first time (a SYS$INPUT that can't be opened has no lines, and its
// status becomes the CLI's).
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

// say queues a line for SYS$OUTPUT.
func (cli *subprocessCLI) say(format string, args ...any) {
	cli.output = append(cli.output, fmt.Sprintf(format, args...))
}

// flush writes the lines say queued to SYS$OUTPUT (corevms's PutOutput:
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

// cliExecute carries out one command line (see this file's opening
// comment), returning the address of an image's IMAGE$INIT driver to
// call if the command runs one, or 0.
func (c *Console) cliExecute(cli *subprocessCLI, line string) uint32 {
	line = strings.TrimSpace(line)
	line = strings.TrimSpace(strings.TrimPrefix(line, "$"))

	if line == "" || strings.HasPrefix(line, "!") {
		return 0
	}

	c.traceCLI(cli, "command %q", line)

	if name, op, value, ok := splitAssignment(line); ok {
		cli.complete(cli.symbols.assign(name, op, value))

		return 0
	}

	verb, rest := readCommandVerb(line)

	if isDeleteSymbol(verb, rest) {
		cli.complete(cli.symbols.delete(rest))

		return 0
	}

	if sym, ok := cli.symbols.lookup(verb); ok {
		if image, foreign := strings.CutPrefix(sym.value, "$"); foreign {
			return c.cliRunImage(cli, strings.Trim(strings.TrimSpace(image), `"`), dclText(rest, true))
		}

		if cli.depth >= maxSymbolDepth {
			cli.fail(cliStatusIVVERB, "%DCL-W-IVVERB, unrecognized command verb - check validity and spelling", verb)

			return 0
		}

		cli.depth++

		return c.cliExecute(cli, sym.value+rest)
	}

	word := strings.ToUpper(verb)

	switch {
	case isVerb(word, "RUN", 1):
		return c.cliRun(cli, rest)

	case isVerb(word, "MCR", 2):
		image, text, _ := strings.Cut(strings.TrimSpace(rest), " ")
		if image == "" {
			cli.fail(cliStatusInsfprm, "%DCL-W-INSFPRM, missing command parameters - supply all required parameters", "")

			return 0
		}

		return c.cliRunImage(cli, image, dclText(text, true))

	case isVerb(word, "LOGOUT", 2):
		cli.loggedOut = true

	case isVerb(word, "EXIT", 3):
		if v := strings.TrimSpace(dclText(rest, false)); v != "" {
			if status, err := parseStatus(v); err == nil {
				cli.status = status
			}
		}

		cli.loggedOut = true

	default:
		cli.fail(cliStatusIVVERB, "%DCL-W-IVVERB, unrecognized command verb - check validity and spelling", word)
	}

	return 0
}

// cliRun is RUN: rest is its qualifiers and its one parameter, the image.
// The qualifiers are accepted and ignored: an image in a subprocess runs
// without the debugger.
func (c *Console) cliRun(cli *subprocessCLI, rest string) uint32 {
	image := ""

	for _, field := range commandFields(rest) {
		if !strings.HasPrefix(field, "/") && image == "" {
			image = field
		}
	}

	if image == "" {
		cli.fail(cliStatusInsfprm, "%DCL-W-INSFPRM, missing command parameters - supply all required parameters", "")

		return 0
	}

	return c.cliRunImage(cli, strings.Trim(image, `"`), "")
}

// cliRunImage activates image in the CLI's process, with commandLine as
// what LIB$GET_FOREIGN returns, and returns its IMAGE$INIT driver's
// address, or 0 (with the failure reported) if it can't.
func (c *Console) cliRunImage(cli *subprocessCLI, image, commandLine string) uint32 {
	env := cli.env

	driver, err := c.activateCreatedImage(env, image, false)
	if err != nil {
		cli.status = corevms.StartupStatus(err)
		cli.say("%%DCL-W-ACTIMAGE, error activating image %s", strings.ToUpper(image))
		cli.say("-%s", strings.TrimPrefix(env.StatusText(cli.status), "%"))
		c.traceCLI(cli, "can't run %s: %v", image, err)

		return 0
	}

	env.CommandLine = commandLine
	c.traceCLI(cli, "runs %s", image)

	return driver
}

// endCLIImage ends the image a CLI's command ran (see this file's
// opening comment): its status, in R0, becomes $STATUS, and DCL's
// message for it is shown if it's a failure whose message hasn't been
// (STS$M_INHIB_MSG); the image is run down; and the CLI's registers are
// put back, at the start of its loop, to read the next command.
func (c *Console) endCLIImage(cli *subprocessCLI) {
	env := cli.env
	status := c.CPU.GPR(vax.R0)

	cli.imageActive = false
	cli.status = status
	c.traceCLI(cli, "image ends, status %08X", status)

	if status&1 == 0 && status&stsInhibitMsg == 0 {
		cli.say("%s", env.StatusText(status))
	}

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

// stsInhibitMsg is STS$M_INHIB_MSG: a status whose message has already
// been shown.
const stsInhibitMsg = 0x10000000

// complete sets $STATUS from a command's error, showing its message.
func (cli *subprocessCLI) complete(err error) {
	if err == nil {
		cli.status = ssNormal

		return
	}

	cli.status = cliStatusIVVERB
	cli.say("%%%s", strings.TrimPrefix(err.Error(), "%"))
}

// fail sets $STATUS to status and shows DCL's message for it, with the
// offending word, if any, on a line of its own between backslashes, as
// DCL shows it.
func (cli *subprocessCLI) fail(status uint32, message, word string) {
	cli.status = status
	cli.say("%s", message)

	if word != "" {
		cli.say(" \\%s\\", word)
	}
}

// commandFields splits a command's text after its verb into its
// parameters and qualifiers, as DCL reads them: blanks separate
// parameters, and a "/" starts a qualifier; inside quotes, neither
// counts, so a quoted host path stays whole.
func commandFields(text string) []string {
	var (
		fields []string
		b      strings.Builder
		quoted bool
	)

	end := func() {
		if b.Len() > 0 {
			fields = append(fields, b.String())
			b.Reset()
		}
	}

	for i := 0; i < len(text); i++ {
		ch := text[i]

		switch {
		case ch == '"':
			quoted = !quoted
		case quoted:
		case ch == ' ' || ch == '\t':
			end()

			continue
		case ch == '/':
			end()
		}

		b.WriteByte(ch)
	}

	end()

	return fields
}

// isVerb reports whether word is verb or an abbreviation of it at least
// minLength characters long.
func isVerb(word, verb string, minLength int) bool {
	return len(word) >= minLength && strings.HasPrefix(verb, word)
}

// parseStatus reads EXIT's status: a decimal number, or a hexadecimal
// one written %Xnnnn, as DCL writes them.
func parseStatus(s string) (uint32, error) {
	var v uint64

	if hex, ok := strings.CutPrefix(strings.ToUpper(s), "%X"); ok {
		if _, err := fmt.Sscanf(hex, "%x", &v); err != nil {
			return 0, err
		}

		return uint32(v), nil
	}

	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return 0, err
	}

	return uint32(v), nil
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
