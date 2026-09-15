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
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/chzyer/readline"
	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/respath"
)

// minimumVAXMemory matches driver.c's own MINIMUM_VAX_MEMORY (2048 pages,
// 1MB): the minimal machine allocated before vax.init runs and does the
// real INIT/VMINIT.
const minimumVAXMemory = 2048 * 512

// pathFlag implements flag.Value for a repeatable "-path <dir>" flag —
// each occurrence appends to the list, in the order given, matching a
// PATH-variable's own left-to-right search order.
type pathFlag []string

func (p *pathFlag) String() string {
	if p == nil {
		return ""
	}
	return fmt.Sprint([]string(*p))
}

func (p *pathFlag) Set(dir string) error {
	*p = append(*p, dir)
	return nil
}

func main() {
	var paths pathFlag
	flag.Var(&paths, "path", "directory to search for unqualified file names (e.g. vax.init, kernel.asm); may be given more than once, searched in order given, after an embedded fallback copy")
	timeLimit := flag.Duration("time-limit", 0, "maximum wall-clock time (e.g. 5s, 15ms) a single GO/CALL/STEP command may run the emulated CPU before it's stopped; 0 (default) is unlimited")
	instructionLimit := flag.Int("instruction-limit", 0, "maximum number of instructions a single GO/CALL/STEP command may execute before it's stopped; 0 (default) is unlimited")
	flag.Parse()

	if err := run(paths, *instructionLimit, *timeLimit, os.Stdout, nil, flag.Args()); err != nil {
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
	resolver := respath.New(paths, bootdata.FS)

	grammarSrc, err := resolver.ReadFile("evax.dcl")
	if err != nil {
		return fmt.Errorf("loading command grammar: %w", err)
	}
	grammar, err := dcl.ParseGrammar(string(grammarSrc))
	if err != nil {
		return fmt.Errorf("loading command grammar: %w", err)
	}

	var help *console.Help
	if helpSrc, err := resolver.ReadFile("vax.help"); err != nil {
		fmt.Fprintln(out, "Warning: no help file available:", err)
	} else {
		help = console.ParseHelp(string(helpSrc))
	}

	c := console.New(out)
	c.Paths = resolver
	if in != nil {
		c.In = in
	} else {
		c.In = os.Stdin
	}
	if err := c.Init(minimumVAXMemory); err != nil {
		return fmt.Errorf("allocating initial VAX: %w", err)
	}

	d := console.NewDispatcher(c, grammar, help)
	c.Dispatcher = d

	if len(args) == 0 {
		fmt.Fprintf(out, "govax — a Go port of eVAX (docs/PLAN.md)\n\n")
	}

	if err := c.Include("vax.init", d.Dispatch); err != nil {
		fmt.Fprintln(out, "vax.init:", err)
	}

	if !c.Running() {
		return nil
	}

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
		Stdin:       in,
	})
	if err != nil {
		return fmt.Errorf("initializing readline: %w", err)
	}
	defer rl.Close()

	for c.Running() {
		line, err := rl.Readline()
		if err != nil { // io.EOF (Ctrl-D) or readline.ErrInterrupt (Ctrl-C)
			if errors.Is(err, readline.ErrInterrupt) {
				continue
			}
			break
		}
		if err := d.Dispatch(line); err != nil {
			fmt.Fprintln(out, "%", err)
		}
	}

	return nil
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
