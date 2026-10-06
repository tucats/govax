// Command govax is the interactive entry point for the VAX emulator —
// the Go equivalent of reference/eVAX/eVAX/Source/Console/driver.c's
// main(), matching its startup sequence (load the DCL grammar, allocate a
// minimal machine, run vax.init as the real startup script, then prompt)
// — see docs/PHASE-08.md's progress log for one deliberate departure from
// driver.c: prompted input (via github.com/chzyer/readline for
// history/editing, named explicitly in docs/PHASE-08.md's scope note,
// rather than a bare fgets(stdin)).
//
// File location (console.dcl/vax.help/vax.init/kernel.asm/ssdef.asm, and any
// other file a console command names) is docs/PHASE-15.md's own departure
// from driver.c's hard CWD-relative "console.dcl" lookup: a repeatable -path
// flag names directories searched, in order, after the name exactly as
// given; an embedded copy of the required startup files
// (internal/bootdata) is always the last, implicit search location, so
// "govax" with no -path flags at all still boots correctly with no
// testdata/ checkout nearby.
//
// -instruction-limit/-time-limit (docs/PHASE-15.md's sub-phase 2, no C
// reference equivalent) bound how long a single GO/CALL/STEP command may
// run the emulated CPU, so a runaway program under development doesn't
// hang the session; both default to unlimited.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/chzyer/readline"
	"github.com/tucats/gopackages/app-cli/app"
	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/gopackages/i18n"
	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vmserrors"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// minimumVAXMemory matches driver.c's own MINIMUM_VAX_MEMORY (2048 pages,
// 1MB): the minimal machine allocated before vax.init runs and does the
// real INIT/VMINIT.
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
		// A run stopped by --instruction-limit or --time-limit has shown
		// its own message already; it only needs its exit status.
		if isLimitStop(err) {
			os.Exit(limitExitStatus)
		}

		fmt.Fprintln(os.Stderr, "govax:", err)
		os.Exit(1)
	}

	auditConfig()
}

// limitExitStatus is govax's exit status when --instruction-limit or
// --time-limit stops the program a one-shot command runs. It is the status
// the Unix timeout(1) command uses for a command it had to stop, so a
// script can tell "ran out of budget" from an ordinary failure (status 1).
const limitExitStatus = 124

// isLimitStop reports whether err is a run stopped by --instruction-limit
// or --time-limit (the console's IncludeCommandLine returns one, wrapped).
func isLimitStop(err error) bool {
	return errors.Is(err, cpu.ErrInstructionLimitExceeded) || errors.Is(err, cpu.ErrTimeLimitExceeded)
}

// run drives startup and the command loop. in, when non-nil, is used as
// readline's input source instead of the real os.Stdin — tests pass a
// controlled reader so startup can be exercised deterministically without
// depending on the test process's own stdin. instructionLimit/timeLimit are
// -instruction-limit/-time-limit (docs/PHASE-15.md's sub-phase 2), applied
// only once vax.init's own startup script has finished running — a limit
// meant to catch a runaway *user* program shouldn't also cut short the
// emulator's own boot sequence.
func run(paths []string, instructionLimit int, timeLimit time.Duration, out io.Writer, in io.ReadCloser, args []string) error {
	var help *console.Help

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
	console.BuildVersion = BuildVersion

	// Set up the fall-back path resolver for including files that might need to be found in the
	// default bootdata embedded file system.
	resolver := respath.New(paths, bootdata.FS)

	grammarSrc, err := resolver.ReadFile("console.dcl")
	if err != nil {
		return vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	grammar, err := dcl.ParseGrammar(string(grammarSrc))
	if err != nil {
		return vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	if helpSrc, err := resolver.ReadFile("console.help"); err != nil {
		fmt.Fprintln(out, "Warning: no help file available:", err)
	} else {
		help = console.ParseHelp(string(helpSrc))
	}

	// The debugger has its own grammar and help file (docs/PHASE-42.md).
	// They are required, like the console's: the DEBUG command and the
	// DBG> prompt depend on them.
	debugGrammarSrc, err := resolver.ReadFile("debug.dcl")
	if err != nil {
		return vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	debugGrammar, err := dcl.ParseGrammar(string(debugGrammarSrc))
	if err != nil {
		return vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	var debugHelp *console.Help

	if helpSrc, err := resolver.ReadFile("debug.help"); err != nil {
		fmt.Fprintln(out, "Warning: no debugger help file available:", err)
	} else {
		debugHelp = console.ParseHelp(string(helpSrc))
	}

	c := console.New(out)

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
		c.In = in
		rlStdin = in
	} else {
		restoreTerminal, vmsMode := vmsTerminalMode(int(os.Stdin.Fd()))
		defer restoreTerminal()

		// A second CTRL/Y while govax is still ending after the first
		// (a command that doesn't finish) exits at once.
		forceExit := func() {
			restoreTerminal()
			os.Exit(1)
		}

		attn = newAttentionStdin(os.Stdin, out, vmsMode, func() *cpu.Engine { return c.Engine }, forceExit)
		c.In = attn
		rlStdin = attn.promptReader()

		stopSignals := installKeySignals(attn)
		defer stopSignals()
	}

	if err := c.Init(minimumVAXMemory); err != nil {
		return vmserrors.Wrap(vmserrors.VAX_ALLOCVAX, err)
	}

	// Volumes still mounted when the session ends are dismounted, so
	// their pending writes reach their containers.
	defer func() {
		if err := c.Mounts.DismountAll(); err != nil {
			fmt.Fprintln(out, "%"+err.Error())
		}
	}()

	d := console.NewDispatcher(c, grammar, help)
	c.Dispatcher = d

	// The debugger works on this console's machine. It is idle until the
	// DEBUG command (or, in later subtasks, GO, CALL, or RUN of a debug
	// image) starts a session.
	debugger.Install(c, debugGrammar, debugHelp)

	if len(args) == 0 {
		fmt.Fprintf(out, "govax %s\n", BuildVersion)
	}

	for _, m := range mountRequests {
		if err := c.Mount(m.device, m.path, m.write); err != nil {
			return err
		}
	}

	// The configured default volume, if any, is mounted and made the
	// default device and directory (internal/console/defvolume.go).
	c.MountDefaultVolume(console.DefaultVolume{
		File:      settings.Get("vax.default.volume.file"),
		Label:     settings.Get("vax.default.volume.label"),
		Device:    settings.Get("vax.default.volume.device"),
		Type:      settings.Get("vax.default.volume.type"),
		Directory: settings.Get("vax.default.volume.directory"),
	})

	// The limits are recorded now but applied only to the user's commands:
	// the one-shot command (INCLUDE/COMMAND_LINE, at the end of vax.init)
	// and the interactive prompt below. See this function's doc comment.
	c.SetRunLimits(instructionLimit, timeLimit)

	if err := c.Include("vax.init", d.Dispatch); err != nil {
		fmt.Fprintln(out, "vax.init:", err)
	}

	// After that, if we're still running, do a console loop.
	if c.Running() {
		// Applied only from here on, not during vax.init's own boot sequence
		// above -- see this function's own doc comment.
		c.ApplyRunLimits()

		historyFile := ""
		if in == nil { // real interactive use, not a test with an injected reader
			historyFile = historyFilePath()
		}

		rlConfig := &readline.Config{
			Prompt:      c.Prompt(),
			HistoryFile: historyFile,
			Stdin:       rlStdin,
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

		for c.Running() && !aborted() {
			// driver.c's own prompt switches from "VAX> " to "ASM> " while a
			// bare ASM command has put the console into interactive assembler
			// mode (docs/PHASE-19.md) -- the ASM_ADDRPROMPT variant that also
			// shows the current deposit address isn't implemented (off by
			// default in the reference tool; see PHASE-19.md's own scope note).
			// The debugger's session has its own prompt too, as the VMS
			// debugger's does. Assembler mode comes first, as it does in
			// Dispatcher.Dispatch. The console's own prompt is asked for
			// each time, since SET PROMPT can change it.
			prompt := c.Prompt()

			if c.InAssemblerMode() {
				prompt = "ASM> "
			} else if c.InDebugger() {
				prompt = debugger.Prompt
			}

			rl.SetPrompt(prompt)

			// CTRL/Z typed after the last line's text makes this read end
			// of file, with nothing typed.
			if keys.eofPending() {
				fmt.Fprint(out, prompt+echoExit+"\n")
				endOfFile(c, d, out)

				continue
			}

			line, err := rl.Readline()
			if aborted() {
				break
			}

			if err != nil {
				switch {
				case keys.endOfFile(err):
					endOfFile(c, d, out)

					continue

				case errors.Is(err, readline.ErrInterrupt): // CTRL/C
					continue
				}

				break // the end of the input (io.EOF)
			}

			// A command that has shown its own messages (RENAME)
			// returns its failure with the message inhibited.
			if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
				fmt.Fprintln(out, "%"+err.Error())
			}
		}
	}

	// See if we have trailing stats to print out here.
	printStats(c, out, stats)

	return c.CommandLineErr()
}

// endOfFile is CTRL/Z at a prompt. In the debugger it is EXIT, ending the
// session, and in the interactive assembler .END. At the console's prompt
// it does nothing more than echo *Exit*, as at DCL's: govax ends only by
// EXIT, QUIT, or CTRL/Y.
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

// printStatus dumps out stats if they are enabled to the console when the emulation finishes.
func printStats(c *console.Console, out io.Writer, flag bool) {
	if flag {
		instructionCount := c.Engine.InstructionCount()
		interruptCount := c.Engine.InterruptCount()
		translate, readCount, writeCount, mbReadCount, mbWriteCount := c.Engine.Memory().Stats()
		elapsed := time.Since(startTime)

		fmt.Fprintf(out, "\nEmulation Statistics:\n")
		fmt.Fprintf(out, "  CPU:\n")
		fmt.Fprintf(out, "    Elapased Time:       %16s\n", formatDuration(elapsed))
		fmt.Fprintf(out, "    Instructions:        %16s\n", formatLargeNumber(int64(instructionCount)))
		fmt.Fprintf(out, "    Interrupts:          %16s\n", formatLargeNumber(int64(interruptCount)))

		fmt.Fprintf(out, "\n  Memory:\n")
		fmt.Fprintf(out, "    Page Translations:   %16s\n", formatLargeNumber(translate))
		fmt.Fprintf(out, "    Single-Byte Reads:   %16s\n", formatLargeNumber(readCount))
		fmt.Fprintf(out, "    Single-Byte Writes:  %16s\n", formatLargeNumber(writeCount))
		fmt.Fprintf(out, "    Multi-byte Reads:    %16s\n", formatLargeNumber(mbReadCount))
		fmt.Fprintf(out, "    Multi-byte Writes:   %16s\n", formatLargeNumber(mbWriteCount))

		stcTries, stcHits := c.Engine.Memory().STCStats()
		tbTries, tbHits, tbFlushes, _ := c.Engine.Memory().TBStats()

		fmt.Fprintf(out, "\n  Translation Buffer:\n")
		fmt.Fprintf(out, "    Sequential Cache Tries: %13s\n", formatLargeNumber(stcTries))
		fmt.Fprintf(out, "    Sequential Cache Hits:  %13s\n", formatLargeNumber(stcHits))
		fmt.Fprintf(out, "    TB Cache Tries:         %13s\n", formatLargeNumber(tbTries))
		fmt.Fprintf(out, "    TB Cache Hits:          %13s\n", formatLargeNumber(tbHits))
		fmt.Fprintf(out, "    TB Flushes:             %13s\n", formatLargeNumber(tbFlushes))

		// The instruction-fetch window (internal/vm/fetch.go): hits are
		// instruction-stream reads served straight from RAM, without a
		// translation (so they appear in none of the counts above); fills
		// are the times a fetch had to translate a new page into it.
		fetchHits, fetchFills := c.Engine.Memory().FetchStats()

		fmt.Fprintf(out, "    Fetch Window Hits:      %13s\n", formatLargeNumber(fetchHits))
		fmt.Fprintf(out, "    Fetch Window Fills:     %13s\n", formatLargeNumber(fetchFills))
	}
}

func formatDuration(d time.Duration) string {
	text := d.String()

	result := strings.Builder{}
	decimal := false
	digits := 0

	for _, ch := range text {
		if ch == '.' {
			decimal = true
		}

		// If it's not a decimal, just copy it and continue
		if !unicode.IsDigit(ch) {
			result.WriteRune(ch)

			continue
		}

		// It is a digit. See if we've already seen the decimal and
		// enough places to ignore this value.
		if decimal && digits > 2 {
			continue
		}

		if decimal {
			digits++
		}

		result.WriteRune(ch)
	}

	return result.String()
}

// formatLargeNumber formats an int64 using commas for ease of readability.
// This uses the experimental text package.
func formatLargeNumber(v int64) string {
	p := message.NewPrinter(language.English)
	result := p.Sprintf("%d", v)

	return result
}

// historyFilePath returns a per-user location for readline's command
// history, or "" (disabling history persistence) if the home directory
// can't be determined.
func historyFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".govax", "history.txt")
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

// This is a list of all valid govax configuration keys. During startup, if a
// configuration key is found that isn't in this list, a warning is printed.
// It usually means the user misspelled something.
//
// ALWAYS ADD NEW CONFIG ITEMS TO THIS LIST!
var validConfigs = map[string]bool{
	"vax.debug.vm":         true,
	"vax.debug.tb":         true,
	"vax.debug.symbols":    true,
	"vax.debug.exceptions": true,
	"vax.debug.interrupts": true,
	"vax.debug.chm":        true,
	"vax.debug.registers":  true,
	"vax.debug.fulldisasm": true,
	"vax.debug.userhalt":   true,
	"vax.debug.keyboard":   true,
	"vax.debug.images":     true,
	"vax.debug.services":   true,
	"vax.debug.dcl":        true,
	"vax.debug.command":    true,
	"vax.debug.logicals":   true,
	"vax.debug.devices":    true,
	"vax.debug.process":    true,
	"vax.debug.libinit":    true,
	"vax.debug.rms":        true,
	"vax.debug.userstep":   true,
	"vax.hardware.clock":   true,
	"vax.library":          true,
	"vax.link.library":     true,
	"vax.path":             true,
	"vax.quantum":          true,
	"vax.console.prompt":   true,
	"vax.console.lines":    true,

	"vax.default.volume.file":      true,
	"vax.default.volume.label":     true,
	"vax.default.volume.device":    true,
	"vax.default.volume.type":      true,
	"vax.default.volume.directory": true,
	"vax.disassemble.symbolic":     true,
}

func auditConfig() {
	count := 0
	keys := settings.Keys()

	for _, key := range keys {
		if !validConfigs[key] {
			if count == 0 {
				fmt.Printf("\nWarning, the following invalid or unrecognized config value(s) will be ignored:\n")
			}

			fmt.Printf("   %s\n", key)

			count++
		}
	}
}
