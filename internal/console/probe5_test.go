package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
)

// TestProbe5 runs the multiprocessing program's last probe
// (testdata/mp/probe5) under govax, as PROBE5.COM runs it on VMS: its
// images on a volume that is the default directory. The report and
// P5_INFO.LOG are logged, to set beside VMS's once it has run
// (testdata/mp/probe5/vax); the test checks that every step reported.
func TestProbe5(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 2000, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	for _, name := range []string{"p5sleep", "p5lock", "p5info", "probe5"} {
		src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", "probe5", name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		mar, obj := filepath.Join(dir, name+".mar"), filepath.Join(dir, name+".obj")
		if err := os.WriteFile(mar, src, 0o644); err != nil {
			t.Fatal(err)
		}

		for _, cmd := range []string{
			`MACRO "` + mar + `"/OBJECT="` + obj + `"`,
			`LINK "` + obj + `"/EXECUTABLE=DUA0:[000000]` + strings.ToUpper(name) + `.EXE`,
		} {
			if err := d.Dispatch(cmd); err != nil {
				t.Fatalf("%s: %v\n%s", cmd, err, out.String())
			}
		}
	}

	if err := d.Dispatch("SET DEFAULT DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	if err := c.Run("DUA0:[000000]PROBE5.EXE", console.RunOptions{}); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out.String())
	}

	report := strings.Join(programLines(out.String()), "\n")
	t.Logf("\n%s", report)

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]P5_INFO.LOG"}, rms.TextRecords)
	if err != nil {
		t.Errorf("P5_INFO.LOG: %v", err)
	}

	lines := make([]string, len(records))
	for i, r := range records {
		lines[i] = string(r)
	}

	t.Logf("P5_INFO.LOG:\n%s", strings.Join(lines, "\n"))

	for _, want := range []string{"1 child", "1 self", "2 the same", "3 P5INFO", "4 $ENQW", "5 $ENQW", "6 $OPEN", "7 $ERASE", "8 SYS$ULWSET", "end"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report lacks %q", want)
		}
	}
}
