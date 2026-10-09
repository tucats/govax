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
// directory. The report is logged, to set beside VMS's
// (testdata/probe49/vax); the test checks that every step reported.
func TestProbe6(t *testing.T) {
	runProbe6(t, "probe6", 0,
		"1 $UPDATE", "2 $GET", "3 FAC", "4a ", "4b ", "4c ", "4d ", "4e ", "5a ", "5b ", "5c ", "6 a process",
		"7a ", "7b ", "7c ", "7d ", "7e ", "7f ", "8 a $QIOW", "8 $OPEN", "9 $SEARCH", "end")
}

// TestProbe6b runs the probe's second round (probe6b.mar) the same way.
func TestProbe6b(t *testing.T) {
	runProbe6(t, "probe6b", 0,
		"1 P6S1.DAT", "1 P6S2.DAT", "4 A's", "4 B's $GET, WAT and TMO=0", "4 B's $GET, WAT and TMO=2",
		"7a ", "7b ", "7c ", "7d ", "7f ", "end")
}

// TestProbe6c runs the probe's third round (probe6c.mar) the same way,
// on a volume of 3-block clusters, as VMS's was: ALQ=10 allocates 12
// blocks, and a user file open's $CREATE ends the file at block 10's end
// (VMS 7.3's 7c line, whole).
func TestProbe6c(t *testing.T) {
	runProbe6(t, "probe6c", 3, "7a ", "7b ", "7d ", "7e $UPDSEC, nothing", "7e $UPDSEC, a page", "7f ", "end",
		"7c after writing page 6: $DELTVA 00000001, $DASSGN 00000001; $OPEN 00010001, EBK 11, FFB 0, HBK 12",
		"7d $CRMPSC of it: 00000619, 3 pages")
}

// runProbe6 builds testdata/probe49's program name, runs it with a new
// volume (of cluster-block clusters; 0 for the default) as the default
// directory, logs its report, and checks the report has each of want.
func runProbe6(t *testing.T, name string, cluster uint16, want ...string) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 2000, "WORK", cluster, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "probe49", name+".mar"))
	if err != nil {
		t.Fatal(err)
	}

	exe := buildImage(t, c, name, string(src))

	if err := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil).Dispatch("SET DEFAULT DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	if err := c.Run(exe, console.RunOptions{}); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out.String())
	}

	report := strings.Join(programLines(out.String()), "\n")
	t.Logf("\n%s", report)

	for _, w := range want {
		if !strings.Contains(report, w) {
			t.Errorf("the report lacks %q", w)
		}
	}
}
