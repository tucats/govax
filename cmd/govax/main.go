// Command govax is the interactive entry point for the VAX emulator —
// the Go equivalent of reference/eVAX/eVAX/Source/Console/driver.c's
// main(), matching its startup sequence (load the DCL grammar, allocate a
// minimal machine, run testdata/dcl/vax.init as the real startup script,
// then prompt) — see docs/PHASE-08.md's progress log for the two spots
// this deliberately departs from driver.c: file-location strategy (a
// -data flag pointing at the directory holding evax.dcl/vax.help/vax.init,
// rather than driver.c's hard CWD-relative "evax.dcl" lookup — go:embed
// isn't usable here since testdata/dcl isn't a subdirectory of this
// package, and the project's own reference/CLAUDE.md already documents
// the C binary's own working-directory convention this mirrors) and
// prompted input (via github.com/chzyer/readline for history/editing,
// named explicitly in docs/PHASE-08.md's scope note, rather than a bare
// fgets(stdin)).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chzyer/readline"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
)

// minimumVAXMemory matches driver.c's own MINIMUM_VAX_MEMORY (2048 pages,
// 1MB): the minimal machine allocated before vax.init runs and does the
// real INIT/VMINIT.
const minimumVAXMemory = 2048 * 512

func main() {
	dataDir := flag.String("data", "testdata/dcl", "directory containing evax.dcl, vax.help, and vax.init")
	flag.Parse()

	if err := run(*dataDir, os.Stdout, nil, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "govax:", err)
		os.Exit(1)
	}
}

// run drives startup and the command loop. in, when non-nil, is used as
// readline's input source instead of the real os.Stdin — tests pass a
// controlled reader so startup can be exercised deterministically without
// depending on the test process's own stdin.
func run(dataDir string, out io.Writer, in io.ReadCloser, args []string) error {
	grammar, err := dcl.LoadGrammarFile(filepath.Join(dataDir, "evax.dcl"))
	if err != nil {
		return fmt.Errorf("loading command grammar: %w", err)
	}

	help, err := console.LoadHelpFile(filepath.Join(dataDir, "vax.help"))
	if err != nil {
		fmt.Fprintln(out, "Warning: no help file available:", err)
		help = nil
	}

	c := console.New(out)
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

	initPath := filepath.Join(dataDir, "vax.init")
	if _, err := os.Stat(initPath); err == nil {
		if err := c.Include(initPath, d.Dispatch); err != nil {
			fmt.Fprintln(out, "vax.init:", err)
		}
	}

	if !c.Running() {
		return nil
	}

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
