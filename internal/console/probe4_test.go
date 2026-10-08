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

// TestProbe4 runs Phase 48's probe 4 (testdata/mp/probe4) under govax, as
// PROBE4.COM runs it on VMS: its images and P4CMDS.COM on a volume that
// is the default directory, and the DCL symbol P4SYM defined. The report
// is logged, line for line beside VMS's (testdata/mp/probe4/vax), and the
// test checks only that every step reported.
func TestProbe4(t *testing.T) {
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

	for _, name := range []string{"p4child", "p4nocli", "probe4"} {
		src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", "probe4", name+".mar"))
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

	cmds, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", "probe4", "p4cmds.com"))
	if err != nil {
		t.Fatal(err)
	}

	var records [][]byte
	for _, line := range strings.Split(strings.TrimRight(string(cmds), "\n"), "\n") {
		records = append(records, []byte(line))
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]P4CMDS.COM"}, rms.TextRecords, records); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range []string{"SET DEFAULT DUA0:[000000]", `P4SYM == "a global symbol"`} {
		if err := d.Dispatch(cmd); err != nil {
			t.Fatal(err)
		}
	}

	out.Reset()

	if err := c.Run("DUA0:[000000]PROBE4.EXE", console.RunOptions{}); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out.String())
	}

	report := strings.Join(programLines(out.String()), "\n")
	t.Logf("\n%s", report)

	// The spawned processes' output files, as PROBE4.COM types them.
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "9", "10"} {
		name := "DUA0:[000000]P4_" + n + ".LOG"

		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: name}, rms.TextRecords)
		if err != nil {
			t.Logf("%s: %v", name, err)

			continue
		}

		lines := make([]string, len(records))
		for i, r := range records {
			lines[i] = string(r)
		}

		t.Logf("%s:\n%s", name, strings.Join(lines, "\n"))
	}

	for _, want := range []string{"1 RUN", "5 SHOW", "8 flags", "9 NOWAIT: completion", "10 $CREPRC", "11 LIB$SPAWN", "end"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report lacks %q", want)
		}
	}
}
