// Command govax is the interactive entry point for the VAX emulator —
// the Go equivalent of reference/eVAX/eVAX/Source/Console/driver.c's
// main(), matching its startup sequence (load the DCL grammar, allocate a
// minimal machine, run vax.init as the real startup script, then prompt)
// — see docs/PHASE-08.md's progress log for one deliberate departure from
// driver.c: prompted input (via github.com/chzyer/readline for
// history/editing, named explicitly in docs/PHASE-08.md's scope note,
// rather than a bare fgets(stdin)).
//
// File location (evax.dcl/vax.help/vax.init/kernel.asm/ssdef.asm, and any
// other file a console command names) is docs/PHASE-15.md's own departure
// from driver.c's hard CWD-relative "evax.dcl" lookup: a repeatable -path
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
	"github.com/tucats/gopackages/i18n"
	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/cpu"
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

	app := app.New("govax: VAX/VMS emulator")
	app.SetVersion(parseVersion(BuildVersion))
	app.SetCopyright("(C) Copyright Tom Cole 2026")
	app.Action = consoleCmd

	err := app.Run(grammar, os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "govax:", err)
		os.Exit(1)
	}
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

	// Set up the fall-back path resolver for including files that might need to be found in the
	// default bootdata embedded file system.
	resolver := respath.New(paths, bootdata.FS)

	grammarSrc, err := resolver.ReadFile("evax.dcl")
	if err != nil {
		return vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	grammar, err := dcl.ParseGrammar(string(grammarSrc))
	if err != nil {
		return vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	if helpSrc, err := resolver.ReadFile("vax.help"); err != nil {
		fmt.Fprintln(out, "Warning: no help file available:", err)
	} else {
		help = console.ParseHelp(string(helpSrc))
	}

	c := console.New(out)

	c.Paths = resolver
	c.Verbose = (console.CommandLineString == "")

	// rlStdin is what readline.Config.Stdin gets below -- the same reader
	// c.In uses, so there is only ever one real reader of the terminal
	// (see attentionStdin's own doc comment). A test-injected in is used
	// as-is, exactly as before; real interactive use gets an
	// attentionStdin wrapping the real os.Stdin, so Ctrl-C interrupts a
	// running VAX program (see cpu.Engine.Attention) instead of arriving
	// as ordinary input or, since readline's raw mode disables the
	// terminal's own SIGINT generation, being silently lost.
	var rlStdin io.ReadCloser

	if in != nil {
		c.In = in
		rlStdin = in
	} else {
		attn := newAttentionStdin(os.Stdin, func() *cpu.Engine { return c.Engine })
		c.In = attn
		rlStdin = attn

		// Covers the window attentionStdin's own byte filtering can't --
		// see installSigintAttention's own doc comment (attention.go) for
		// why a real SIGINT, not just a 0x03 byte, needs handling here too.
		stopSigint := installSigintAttention(func() *cpu.Engine { return c.Engine })
		defer stopSigint()
	}

	if err := c.Init(minimumVAXMemory); err != nil {
		return vmserrors.Wrap(vmserrors.VAX_ALLOCVAX, err)
	}

	d := console.NewDispatcher(c, grammar, help)
	c.Dispatcher = d

	if len(args) == 0 {
		fmt.Fprintf(out, "govax %s\n", BuildVersion)
	}

	if err := c.Include("vax.init", d.Dispatch); err != nil {
		fmt.Fprintln(out, "vax.init:", err)
	}

	// After that, if we're still running, do a console loop.
	if c.Running() {
		// Applied only from here on, not during vax.init's own boot sequence
		// above -- see this function's own doc comment.
		c.Engine.SetLimits(instructionLimit, timeLimit)

		historyFile := ""
		if in == nil { // real interactive use, not a test with an injected reader
			historyFile = historyFilePath()
		}

		rl, err := readline.NewEx(&readline.Config{
			Prompt:      "VAX> ",
			HistoryFile: historyFile,
			Stdin:       rlStdin,
		})
		if err != nil {
			return vmserrors.Wrap(vmserrors.VAX_READLINE, err)
		}

		defer rl.Close()

		for c.Running() {
			// driver.c's own prompt switches from "VAX> " to "ASM> " while a
			// bare ASM command has put the console into interactive assembler
			// mode (docs/PHASE-19.md) -- the ASM_ADDRPROMPT variant that also
			// shows the current deposit address isn't implemented (off by
			// default in the reference tool; see PHASE-19.md's own scope note).
			if c.InAssemblerMode() {
				rl.SetPrompt("ASM> ")
			} else {
				rl.SetPrompt("VAX> ")
			}

			line, err := rl.Readline()
			if err != nil { // io.EOF (Ctrl-D) or readline.ErrInterrupt (Ctrl-C)
				if errors.Is(err, readline.ErrInterrupt) {
					continue
				}

				break
			}

			if err := d.Dispatch(line); err != nil {
				fmt.Fprintln(out, "%"+err.Error())
			}
		}
	}

	// See if we have trailing stats to print out here.
	printStats(c, out, stats)

	return nil
}

// printStatus dumps out stats if they are enabled to the console when the emulation finishes.
func printStats(c *console.Console, out io.Writer, flag bool) {
	if flag {
		count := c.Engine.InstructionCount()
		translate, readCount, writeCount, mbReadCount, mbWriteCount := c.Engine.Memory().Stats()
		elapsed := time.Since(startTime)

		fmt.Fprintf(out, "\nEmulation Statistics:\n")
		fmt.Fprintf(out, "  CPU:\n")
		fmt.Fprintf(out, "    Elapased Time:       %16s\n", formatDuration(elapsed))
		fmt.Fprintf(out, "    Instructions:        %16s\n", formatLargeNumber(int64(count)))

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

	return filepath.Join(home, ".govax_history")
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
