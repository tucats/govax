// Command govax is the interactive entry point for the VAX emulator. Its
// job is to mount the system volume, load the DCL grammar, allocate a
// minimal machine, run STARTUP.COM as the real startup script, then prompt
// for console input.
//
// The system volume (docs/planning/PHASE-87 - system volume.md) holds STARTUP.COM,
// the grammars and help files, the kernel's sources, and the libraries.
// Without one, the files are found as docs/planning/PHASE-15.md describes: the
// name exactly as given, then the directories of the repeatable --path
// flag, in order; an embedded copy of the required startup files
// (internal/bootdata) is always the last, implicit search location, so
// "govax" still boots correctly with no testdata/ checkout nearby.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/tucats/gopackages/app-cli/app"
	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/gopackages/i18n"
	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// minimumVAXMemory is the minimal machine allocated before STARTUP.COM
// runs and does the real INIT/VMINIT.
const minimumVAXMemory = 2048 * 512

// Version string. This is injected by the build tool by default, but defaults
// to t this string if built with "go build" rather than the build tool.
var BuildVersion = "0.0-0"

// Build timestamp, injected by build tool else empty string.
var BuildTime string

// Wall-clock time when we started up. Not the same as actual instruction
// execution time if the user uses the console, etc.
var startTime time.Time = time.Now()

func main() {
	// Register the application specific localizations.
	i18n.Register(nil)

	// Store the version string where it can be accessed by the $GETSYI service.
	if BuildVersion != "0.0-0" {
		corevms.VersionString = "V" + BuildVersion
	}

	// Disable subcommands and options we don't use.
	app.MakePrivate("logon")
	app.MakePrivate("format")
	app.MakePrivate("log")
	app.MakePrivate("log-file")
	app.MakePrivate("insecure")
	app.MakePrivate("quiet")

	govaxApp := app.New("govax: VAX/VMS emulator")
	govaxApp.SetVersion(parseVersion(BuildVersion))
	govaxApp.SetCopyright("(C) Copyright Tom Cole 2026")
	govaxApp.SetProfileDirectory(".govax")

	govaxApp.Action = consoleCmd

	err := govaxApp.Run(grammar, os.Args)

	// A --cpu-profile is complete only once the profiler stops.
	stopCPUProfile()

	if err != nil {
		os.Exit(reportFailure(os.Stderr, err))
	}

	auditConfig()
}

// reportFailure shows govax's failure err on w, unless its message has
// been shown already, and returns the exit status for it.
func reportFailure(w io.Writer, err error) int {
	// A run stopped by --instruction-limit or --time-limit has shown its
	// own message already; it only needs its exit status.
	if isLimitStop(err) {
		return limitExitStatus
	}

	// So has a failure the console marked as shown (an image's exit
	// status, a command that reports as it goes), as at the prompt.
	if !vmserrors.MessageInhibited(err) {
		fmt.Fprintln(w, "govax:", err)
	}

	return 1
}

// limitExitStatus is govax's exit status when --instruction-limit or
// --time-limit stops the program a one-shot command runs. It is the status
// the Unix timeout(1) command uses for a command it had to stop, so a
// script can tell "ran out of budget" from an ordinary failure (status 1).
const limitExitStatus = 124

// isLimitStop reports whether err is a run stopped by --instruction-limit
// or --time-limit (the console's RunCommandLine returns one, wrapped).
func isLimitStop(err error) bool {
	return errors.Is(err, cpu.ErrInstructionLimitExceeded) || errors.Is(err, cpu.ErrTimeLimitExceeded)
}

// run drives startup and the command loop. in, when non-nil, is used as
// readline's input source instead of the real os.Stdin — tests pass a
// controlled reader so startup can be exercised deterministically without
// depending on the test process's own stdin. instructionLimit/timeLimit are
// -instruction-limit/-time-limit (docs/planning/PHASE-15.md's sub-phase 2), applied
// only once STARTUP.COM's own startup script has finished running — a limit
// meant to catch a runaway *user* program shouldn't also cut short the
// emulator's own boot sequence.
func run(paths []string, instructionLimit int, timeLimit time.Duration, out io.Writer, in io.ReadCloser, args []string) error {
	// Squirrel away the command line arguments.
	argText := strings.Builder{}

	for _, arg := range args {
		if strings.TrimSpace(arg) != "" {
			if argText.Len() > 0 {
				argText.WriteRune(' ')
			}

			argText.WriteString(arg)
		}
	}

	console.CommandLineString = argText.String()
	dclcmd.BuildVersion = BuildVersion

	// Set up the fall-back path resolver for including files that might need to be found in the
	// default bootdata embedded file system.
	resolver := respath.New(paths, bootdata.FS)

	// Read from the real terminal, the console writes through a
	// promptOutput (promptout.go), so that what the other processes
	// write while a command line is typed doesn't garble the line.
	var output *promptOutput

	consoleOut := out

	if in == nil {
		output = newPromptOutput(out)
		consoleOut = output
	}

	// The authorization file, made the first time with the SYSTEM and
	// DEFAULT accounts (docs/planning/PHASE-95 - authentication.md). One
	// govax can't read stops it here, rather than starting as SYSTEM with
	// the file's accounts silently lost.
	uafPath, err := ensureAuthorizationFile()
	if err != nil {
		return err
	}

	// The account --user names must be one that may log in.
	if err := checkLogin(uafPath, out, in == nil); err != nil {
		return err
	}

	// The queue database (docs/planning/PHASE-98 - batch queues.md,
	// Decision 3): checked here for the same reason.
	queuePath, err := ensureQueueFile()
	if err != nil {
		return err
	}

	c := console.New(consoleOut)
	c.AuthorizationFile = uafPath
	c.QueueFile = queuePath

	c.Paths = resolver
	c.Verbose = (console.CommandLineString == "")

	// rlStdin is what readline.Config.Stdin gets below -- the same reader
	// c.In uses, so there is only ever one real reader of the terminal
	// (see attentionStdin's own doc comment). A test-injected in is used
	// as-is; real use gets an attentionStdin wrapping the real os.Stdin,
	// which gives the control keys VMS's meanings (attention.go): CTRL/C
	// interrupts, CTRL/Y ends govax, CTRL/Z is end of file.
	var (
		rlStdin io.ReadCloser
		attn    *attentionStdin
	)

	if in != nil {
		// A byte at a time, so that readline's buffered reader never
		// takes more than the command line it's reading: the lines after
		// it may be answers to a procedure's INQUIRE, which reads c.In.
		shared := &byteReader{r: in}
		c.In = shared
		rlStdin = shared
	} else {
		restoreTerminal, hostMode, vmsMode := vmsTerminalMode(int(os.Stdin.Fd()))
		defer restoreTerminal()

		// A second CTRL/Y while govax is still ending after the first
		// (a command that doesn't finish) exits at once.
		forceExit := func() {
			restoreTerminal()
			os.Exit(1)
		}

		// At a terminal the pump reads through terminalInput, which can stop
		// so that a host program started by a command gets the keys.
		var (
			input io.Reader = os.Stdin
			keys  *terminalInput
		)

		if vmsMode {
			keys = newTerminalInput(int(os.Stdin.Fd()))
			input = keys
			c.TerminalEcho = terminalEcho(int(os.Stdin.Fd()))
		}

		attn = newAttentionStdin(input, out, vmsMode, func() *cpu.Engine { return c.Engine }, forceExit)
		attn.controlY = c.ControlYEnabled
		c.LendTerminal = lendTerminal(attn, keys, hostMode)
		c.In = attn
		rlStdin = attn.promptReader()

		stopSignals := installKeySignals(attn)
		defer stopSignals()
	}

	if err := c.Init(minimumVAXMemory); err != nil {
		return vmserrors.Wrap(vmserrors.VAX_ALLOCVAX, err)
	}

	// Volumes still mounted when the session ends are dismounted, so
	// their pending writes reach their containers, after the files
	// process 1 left open are closed. Then the RAM disks' memory is let
	// go (docs/planning/PHASE-96 - RAM disk.md): the process's end would
	// free it anyway, but tests that start many sessions shouldn't hold it.
	defer func() {
		c.EndSession()

		if err := c.Mounts.DismountAll(); err != nil {
			fmt.Fprintln(out, "%"+err.Error())
		}

		if err := c.Mounts.ReleaseMemory(); err != nil {
			fmt.Fprintln(out, "%"+err.Error())
		}
	}()

	if len(args) == 0 {
		fmt.Fprintf(out, "govax %s\n", BuildVersion)
	}

	// The system volume first (docs/planning/PHASE-87 - system volume.md), so that
	// it gets its device, then the --mount options, whose devices may be a
	// controller alone ("DUA") for its next free unit. Mounting it needs
	// no grammar: it is RMS's and ods2's work, in Go.
	system := c.Commands().MountSystemVolume(systemVolume())

	// The grammars and help files, the system volume's copies before
	// govax's own (grammarfiles.go).
	files := grammarFiles{resolver: resolver, out: out}

	if system.Mounted {
		resolver.Volume = c.Commands().SystemFS()
		files.device = system.Device
	}

	d, err := installConsoleGrammar(c, files)
	if err != nil {
		return err
	}

	// The debugger works on this console's machine. It is idle until the
	// DEBUG command (or GO, CALL, or RUN of a debug image) starts a
	// session.
	if err := installDebuggerGrammar(c, files); err != nil {
		return err
	}

	for _, m := range mountRequests {
		if err := c.Commands().Mount(c.Commands().ResolveUnit(m.device), m.path, m.write); err != nil {
			return err
		}
	}

	// The configured default volume, if any, is mounted and made the
	// default device and directory (internal/dclcmd/defvolume.go).
	c.Commands().MountDefaultVolume(dclcmd.DefaultVolume{
		File:      settings.Get("vax.default.volume.file"),
		Label:     settings.Get("vax.default.volume.label"),
		Device:    settings.Get("vax.default.volume.device"),
		Type:      settings.Get("vax.default.volume.type"),
		Directory: settings.Get("vax.default.volume.directory"),
	})

	// A directory the configuration names for the default volume is
	// SYSTEM's home (docs/planning/PHASE-95 - authentication.md, Decision
	// 3): where SET DEFAULT has just put the console.
	systemHome := ""
	if strings.TrimSpace(settings.Get("vax.default.volume.directory")) != "" {
		if device, directory, ok := c.VolumeDefault(); ok {
			systemHome = device + directory
		}
	}

	// The limits are recorded now but applied only to the user's commands:
	// the one-shot command and the interactive prompt below. See this
	// function's doc comment.
	c.SetRunLimits(instructionLimit, timeLimit)

	// We've done all the setup we can before tracking actual user run time.
	// Track the initialization time, and reset the clock for emulation time.
	c.InitTime = time.Since(startTime)
	startTime = time.Now()

	// STARTUP.COM is govax's login command procedure, run at command level
	// 1 (docs/planning/PHASE-50 - DCL command procedures.md): the system volume's,
	// or govax's own without one (docs/planning/PHASE-87 - system volume.md). A
	// command in it that fails has shown its message, which ends it.
	if err := c.RunStartup(d.Dispatch); err != nil && !vmserrors.MessageInhibited(err) {
		fmt.Fprintln(out, "%"+err.Error())
	}

	// With paging on, the page file is installed now, after STARTUP.COM and
	// before any of the user's commands, as VMS installs its page file at
	// boot: created, or made its configured size, so its space is the
	// system's from the start (unless vax.sysgen.paging.defer leaves it
	// for the pager's first need).
	c.StartPageFile()

	// If a startup procedure of the user's own is configured, run it: a
	// volume file or a host file, by the form of its name. It is part of
	// the system's startup, as STARTUP.COM is, so it runs as SYSTEM,
	// before process 1 logs in as --user's account: a MOUNT or another
	// privileged command in it works whoever logs in. One that isn't
	// there is passed over without a message; one that is runs as a
	// command procedure, and its failure is reported.
	if startup := settings.Get("vax.startup.procedure"); startup != "" {
		// The name's form says whether it is a volume file's or a host
		// file's.
		if rms.IsODSFileName(startup) {
			if names, err := c.ContainerSession.Search(startup); len(names) > 0 && err == nil {
				fileName := names[0]
				// It is an ODS file name, so run it as a command procedure.
				if err := c.RunProcedure(fileName, d.Dispatch); err != nil && !vmserrors.MessageInhibited(err) {
					fmt.Fprintln(out, "%"+err.Error())
				}
			}
		} else {
			// See if the host file exists. If it does not, there is no error.
			// But if it does, run it as a host command procedure.
			if _, err := os.Stat(startup); err == nil {
				if err := c.RunHostProcedure(startup, d.Dispatch); err != nil && !vmserrors.MessageInhibited(err) {
					fmt.Fprintln(out, "%"+err.Error())
				}
			}
		}
	}

	// The queue manager starts here if the queue database says it was
	// running, as VMS starts it at boot (docs/planning/PHASE-98 - batch
	// queues.md, Decision 11): queues come back stopped, and a job the
	// last session left executing is requeued (/RESTART) or ended. A
	// one-shot command doesn't start it.
	if console.CommandLineString == "" {
		c.ResumeQueueManager()
	}

	// Then process 1 logs in as its account, SYSTEM unless --user names
	// another (docs/planning/PHASE-95 - authentication.md, Decision 7). At
	// a terminal the banner says when the account last logged in.
	login, err := c.LogIn(loginAccount(), systemHome)
	if err != nil {
		return err
	}

	if c.Verbose {
		fmt.Fprint(out, login.Previous().LastLoginText())
	}

	// In a session, DCL runs the login procedures, SYS$SYLOGIN and then the
	// account's LGICMD or SYS$LOGIN:LOGIN.COM (docs/planning/PHASE-95 -
	// authentication.md, Decision 8). A one-shot command runs without them,
	// so its output stays predictable.
	if console.CommandLineString == "" && c.Running() {
		if err := c.RunLoginProcedures(login.Record.LGICMD, d.Dispatch); err != nil {
			fmt.Fprintln(out, "%"+err.Error())
		}
	}

	// Then the command interpreter reads level 0's input: the one-shot
	// command left on govax's command line, if there is one, after which
	// the session ends, or else the terminal.
	if c.Running() {
		c.RunCommandLine(d.Dispatch)
	}

	// After that, if we're still running, do a console loop.
	if c.Running() {
		// Applied only from here on, not during STARTUP.COM's own boot sequence
		// above -- see this function's own doc comment.
		c.ApplyRunLimits()

		historyFile := ""
		if in == nil { // real interactive use, not a test with an injected reader
			historyFile = historyFilePath()
		}

		// The history files are the prompt modes' own (history.go), so
		// readline keeps its history in memory only.
		histories := newModeHistories(historyFile)

		rlConfig := &readline.Config{
			Prompt: c.Prompt(),
			Stdin:  rlStdin,
		}

		keys := newPromptKeys(rlConfig)

		if attn != nil {
			rlConfig.FuncMakeRaw, rlConfig.FuncExitRaw = attn.rawMode()
		}

		rl, err := readline.NewEx(rlConfig)
		if err != nil {
			return vmserrors.Wrap(vmserrors.VAX_READLINE, err)
		}

		c.Verbose = true

		defer rl.Close()

		// readline's Close waits for its goroutine, which may be waiting
		// to read; closing attn first (deferred calls run last first) ends
		// that read.
		if attn != nil {
			defer attn.Close()
		}

		// aborted is CTRL/Y's check (attentionStdin.abortRequested).
		aborted := func() bool { return attn != nil && attn.abortRequested() }

		// endConfirmed is govax's end at a terminal with batch jobs
		// executing: true unless the user, asked, says No (confirmEnd).
		endConfirmed := func() bool {
			return attn == nil || !attn.vmsMode || confirmEnd(c, rl, out)
		}

	session:
		for c.Running() && !aborted() {
			// Switches between console, debugger, and mini-assembler prompts.
			// The console prompt can be set via a SET PROMPT command.
			prompt := c.Prompt()
			mode := ""

			if c.InAssemblerMode() {
				prompt, mode = "ASM> ", "asm"
			} else if c.InDebugger() {
				prompt, mode = debugger.Prompt, "debug"
			} else if d.Continuing() {
				prompt = "_" + prompt
			}

			rl.SetPrompt(prompt)

			// The history recalled is the mode's own (history.go), so
			// leaving the debugger doesn't bring its commands back at the
			// console's prompt.
			histories.use(rl, mode)

			// CTRL/Z typed after the last line's text makes this read end
			// of file, with nothing typed.
			if keys.eofPending() {
				fmt.Fprint(out, prompt+echoExit+"\n")
				d.CancelContinuation()
				endOfFile(c, d, out)

				continue
			}

			line, err := readCommandLine(rl, c, attn, output)

			if aborted() {
				if !endConfirmed() {
					attn.forgetAbort()

					continue
				}

				break
			}

			if err != nil {
				switch {
				case keys.endOfFile(err):
					d.CancelContinuation()
					endOfFile(c, d, out)

					continue

				case errors.Is(err, readline.ErrInterrupt): // CTRL/C
					d.CancelContinuation()

					continue
				}

				break // the end of the input (io.EOF)
			}

			histories.add(line)

			// A command that has shown its own messages (RENAME)
			// returns its failure with the message inhibited.
			if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
				fmt.Fprintln(out, "%"+err.Error())
			}

			// EXIT, LOGOUT, or CTRL/Y with batch jobs executing asks
			// first, at a terminal; No goes back to the prompt.
			if (!c.Running() || aborted()) && !endConfirmed() {
				c.Resume()
				attn.forgetAbort()

				continue session
			}
		}
	}

	// Ending with batch jobs still executing, and no terminal to ask at
	// (a one-shot command, input from a pipe), says so and ends them.
	if c.ExecutingJobs() > 0 {
		confirmEnd(c, nil, out)
	}

	// See if we have trailing stats to print out here.
	printStats(c, out, stats)

	return c.CommandLineErr()
}

// readCommandLine reads a command line with rl. At a terminal, it is
// read on a goroutine of its own while the console lets the other
// processes run (docs/planning/PHASE-51, subtask 4; Console.LendWhileReading),
// their output going through the line editor (promptOutput). Otherwise
// (input from a pipe or a test's reader), the read is all that happens,
// as before: the other processes ran after the last command until they
// were idle (Console.LendWhileIdle), and run again after the next.
func readCommandLine(rl *readline.Instance, c *console.Console, attn *attentionStdin, output *promptOutput) (string, error) {
	if attn == nil || !attn.vmsMode || output == nil {
		return rl.Readline()
	}

	var (
		line string
		err  error
	)

	done := make(chan struct{})

	attn.lineRead.Store(true)
	output.begin(rl.Stdout())

	go func() {
		defer close(done)

		line, err = rl.Readline()
	}()

	c.LendWhileReading(done)

	output.end()
	attn.lineRead.Store(false)

	return line, err
}

// confirmEnd is govax's end with batch jobs executing (docs/planning/
// PHASE-98 - batch queues.md, Decision 11): it says how many, and, with
// rl (a terminal), asks whether to end them; on No it returns false, and
// govax goes on. Otherwise the jobs are ended as a VMS system's stop ends
// them (requeued to restart if /RESTART, else ended with JBC$_QUERESET),
// it says so, and it returns true. With none executing it returns true.
func confirmEnd(c *console.Console, rl *readline.Instance, out io.Writer) bool {
	n := c.ExecutingJobs()
	if n == 0 {
		return true
	}

	what, them := "1 batch job", "it"
	if n > 1 {
		what, them = fmt.Sprintf("%d batch jobs", n), "them"
	}

	fmt.Fprintln(out, "%"+vmserrors.New(vmserrors.VAX_JOBSEXECUTING, what, them).Error())

	if rl != nil {
		rl.SetPrompt("End " + them + " and exit? (Y or N) [N]: ")

		answer, err := rl.Readline()
		if err == nil && !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(answer)), "Y") {
			return false
		}
	}

	requeued, ended, err := c.EndBatchJobs()
	if err != nil {
		fmt.Fprintln(out, "%"+err.Error())

		return true
	}

	fmt.Fprintln(out, "%"+vmserrors.New(vmserrors.VAX_JOBSENDED, requeued, ended).Error())

	return true
}

// endOfFile is CTRL/Z at a prompt. In the debugger it is EXIT, ending the
// session, and in the interactive assembler .END. At the console's prompt
// it does nothing more than echo *Exit*, as at DCL's: govax ends only by
// EXIT, LOGOUT, or CTRL/Y.
func endOfFile(c *console.Console, d *console.Dispatcher, out io.Writer) {
	var command string

	switch {
	case c.InAssemblerMode():
		command = ".END"
	case c.InDebugger():
		command = "EXIT"
	default:
		return
	}

	if err := d.Dispatch(command); err != nil && !vmserrors.MessageInhibited(err) {
		fmt.Fprintln(out, "%"+err.Error())
	}
}

// modeHistory is the history file of a prompt's mode, beside console,
// the console's: "debug_history.txt" for the debugger's DBG>, and
// "asm_history.txt" for the assembler's ASM>; console itself for the
// console's mode (""). With no console history file (a test, or no home
// directory), there is none.
func modeHistory(console, mode string) string {
	if console == "" || mode == "" {
		return console
	}

	return filepath.Join(filepath.Dir(console), mode+"_history.txt")
}

// parseVersion is a helper function that parses a version string into its major, minor, and build components.
// The version string is expected to be in the format "major.minor-build". If the version string does not match
// this format, an error message is printed to the console, and the program exits with a status code of 1.
//
// Parameters:
//
//	version (string): The version string to be parsed.
//
// Returns:
//
//	major (int): The major component of the version.
//	minor (int): The minor component of the version.
//	build (int): The build component of the version.
func parseVersion(version string) (major int, minor int, build int) {
	count, err := fmt.Sscanf(version, "%d.%d-%d", &major, &minor, &build)
	if count != 3 || err != nil {
		fmt.Printf("Invalid version string: %s\n", version)
		os.Exit(1)
	}

	return
}
