package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
)

// TestProbe6 runs Phase 49's probe (testdata/probe49) under govax, as
// PROBE6.COM runs it on VMS: its image on a volume that is the default
// directory. The report is logged, to set beside VMS's once it has run
// (testdata/probe49/vax); the test checks that every step reported.
func TestProbe6(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 2000, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "probe49", "probe6.mar"))
	if err != nil {
		t.Fatal(err)
	}

	exe := buildImage(t, c, "probe6", string(src))

	if err := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil).Dispatch("SET DEFAULT DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	if err := c.Run(exe, console.RunOptions{}); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out.String())
	}

	report := strings.Join(programLines(out.String()), "\n")
	t.Logf("\n%s", report)

	for _, want := range []string{
		"1 $UPDATE", "2 $GET", "3 FAC", "4a ", "4b ", "4c ", "4d ", "4e ", "5a ", "5b ", "5c ", "6 a process",
		"7a ", "7b ", "7c ", "7d ", "7e ", "7f ", "8 a $QIOW", "8 $OPEN", "9 $SEARCH", "end",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("the report lacks %q", want)
		}
	}
}
