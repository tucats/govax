package console

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// mountBackupSets mounts a fresh volume as DUA1 holding the probe's save
// sets in [SETS], copied byte for byte (testdata/backup69/vax/*.bck), as
// the probe's exchange volume held them, with DUA1:[000000] the default.
func mountBackupSets(t *testing.T) (*Dispatcher, *Console, func() string) {
	t.Helper()

	d, c, buf := newCommandDispatcher(t)

	path := filepath.Join(t.TempDir(), "sets.dsk")
	if err := c.cmd().InitializeContainer(path, 20000, "BACKUP69", 0, "RD54"); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	if err := c.cmd().Mount("DUA1", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	sets, err := filepath.Glob(backupSets + "*.bck")
	if err != nil || len(sets) == 0 {
		t.Fatalf("no save sets: %v", err)
	}

	for _, line := range []string{"CREATE/DIRECTORY DUA1:[SETS]", "SET DEFAULT DUA1:[000000]"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	for _, set := range sets {
		name := strings.ToUpper(filepath.Base(set))
		if err := d.Dispatch(`COPY/BINARY "` + set + `" [SETS]` + name); err != nil {
			t.Fatalf("COPY %s: %v", name, err)
		}
	}

	output := func() string {
		text := buf.String()
		buf.Reset()

		return text
	}

	output()

	return d, c, output
}

// backupLogListings are the BACKUP/LIST commands in VMS's log of the
// probe (testdata/backup69/vax/backup69.log) and what each wrote: the
// lines after the command up to the procedure's next line.
func backupLogListings(t *testing.T) (commands []string, outputs [][]string) {
	t.Helper()

	data, err := os.ReadFile(backupSets + "backup69.log")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(data), "\n")
	command := regexp.MustCompile(`^\$ +(BACKUP/LIST\S* .*)$`)

	for i := 0; i < len(lines); i++ {
		m := command.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}

		var out []string

		for i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "$") {
			i++
			out = append(out, lines[i])
		}

		commands = append(commands, m[1])
		outputs = append(outputs, out)
	}

	return commands, outputs
}

// TestBackupListOracle: every BACKUP/LIST in VMS's log of the probe,
// run on govax on the same save sets, writes what VMS wrote, line for
// line: the summaries, /BRIEF and /FULL, /SELECT and /EXCLUDE (which a
// listing ignores), the truncated and damaged save sets (XORERRS for
// the recovered header), and the errors after the heading. A listing's
// $STATUS is VMS's %X10000001.
func TestBackupListOracle(t *testing.T) {
	d, c, output := mountBackupSets(t)

	commands, outputs := backupLogListings(t)
	if len(commands) < 30 {
		t.Fatalf("%d BACKUP/LIST commands in the log", len(commands))
	}

	ran := 0

	for i, cmd := range commands {
		// /LIST=file writes no output (TestBackupListFile), and the
		// probe's [B69] tree isn't on this volume.
		if strings.HasPrefix(cmd, "BACKUP/LIST=") || strings.Contains(cmd, "[B69]") {
			continue
		}

		ran++

		err := d.Dispatch(cmd)
		got := output()

		if err != nil {
			got += "%" + err.Error() + "\n"
		} else if status := c.Status(); status != 0x10000001 {
			t.Errorf("%s: $STATUS %%X%08X, want %%X10000001", cmd, status)
		}

		want := strings.Join(outputs[i], "\n")
		if diff := firstDifference(strings.TrimRight(got, "\n"), strings.TrimRight(want, "\n")); diff != "" {
			t.Errorf("%s: %s", cmd, diff)
		}
	}

	t.Logf("%d listings compared", ran)

	if ran < 25 {
		t.Errorf("only %d listings compared", ran)
	}
}

// TestBackupListFile: /LIST=file writes the listing into the file, its
// lines VMS's (testdata/backup69/vax/text.lis and b69full.lis, copied
// out by COPY to the host, a record to a line), and nothing on
// SYS$OUTPUT.
func TestBackupListFile(t *testing.T) {
	d, _, output := mountBackupSets(t)

	for _, tc := range []struct{ command, file, want string }{
		{"BACKUP/LIST=[SETS]TEXT.LIS [SETS]TEXT.BCK/SAVE_SET", "[SETS]TEXT.LIS;1", "text.lis"},
		{"BACKUP/LIST=[SETS]B69FULL.LIS/FULL [SETS]B69.BCK/SAVE_SET", "[SETS]B69FULL.LIS", "b69full.lis"},
		{"BACKUP/LIST=[SETS]DEFTYPE [SETS]TEXT.BCK/SAVE_SET", "[SETS]DEFTYPE.LIS", "text.lis"},
	} {
		if err := d.Dispatch(tc.command); err != nil {
			t.Fatalf("%s: %v", tc.command, err)
		}

		if out := output(); out != "" {
			t.Errorf("%s wrote %q", tc.command, out)
		}

		host := filepath.Join(t.TempDir(), tc.want)
		if err := d.Dispatch(`COPY ` + tc.file + ` "` + host + `"`); err != nil {
			t.Fatalf("COPY %s: %v", tc.file, err)
		}

		output()

		got, err := os.ReadFile(host)
		if err != nil {
			t.Fatal(err)
		}

		want, err := os.ReadFile(backupSets + tc.want)
		if err != nil {
			t.Fatal(err)
		}

		if diff := firstDifference(string(got), string(want)); diff != "" {
			t.Errorf("%s: %s", tc.file, diff)
		}
	}
}

// TestBackupListHost: a host save set lists as a volume one does
// (testdata/backup69/README.md has VMS's listing of pi.bck).
func TestBackupListHost(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := d.Dispatch(`BACKUP/LIST "` + backupSets + `pi.bck"/SAVE_SET`); err != nil {
		t.Fatal(err)
	}

	want := strings.Join([]string{
		"Listing of save set(s)",
		"",
		"Save set:          PI.BCK",
		"Written by:        SYSTEM      ",
		"UIC:               [000001,000004]",
		"Date:              20-OCT-2026 10:45:22.88",
		"Command:           BACKUP [.PI]*.* PI.BCK/SAVE",
		"Operating system:  OpenVMS VAX version V7.1",
		"BACKUP version:    V6.2",
		"CPU ID register:   040704D2",
		"Written on:        _SIMVAX$DUA1:",
		"Block size:        32256",
		"Group size:        10",
		"Buffer count:      8",
		"",
		"[XFER.PI]PI.EXE;1                                           6  20-OCT-2026 10:44",
		"[XFER.PI]PI.MAR;1                                          24  20-OCT-2026 10:41",
		"[XFER.PI]PI.OBJ;1                                           5  20-OCT-2026 10:44",
		"",
		"Total of 3 files, 35 blocks",
		"End of save set\r",
		"",
		"",
	}, "\n")

	if diff := firstDifference(buf.String(), want); diff != "" {
		t.Error(diff)
	}
}

// firstDifference says where got and want first differ, line by line,
// or "" when they're the same.
func firstDifference(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")

	for i := 0; i < len(g) || i < len(w); i++ {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}

		if i < len(w) {
			wl = w[i]
		}

		if gl != wl || i >= len(g) || i >= len(w) {
			return fmt.Sprintf("line %d:\n got  %s\n want %s", i+1, quoteLine(gl, i < len(g)), quoteLine(wl, i < len(w)))
		}
	}

	return ""
}

func quoteLine(s string, present bool) string {
	if !present {
		return "(none)"
	}

	return strings.ReplaceAll(strings.ReplaceAll(s, "\r", `\r`), "\t", `\t`)
}
