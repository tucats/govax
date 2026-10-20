package console

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// backupRestoreLines are the lines of VMS's log of the probe from Part 4
// on that TestBackupRestoreOracle runs, each with what VMS wrote after
// it: the restores, the commands they depend on (CREATE/DIRECTORY, SET
// DEFAULT), the DIRECTORY reports of what they made, and SHOW SYMBOL
// $STATUS after each. Listings, compares, and the lines that change files
// some other way are left out, and so is a SHOW SYMBOL after one of them.
func backupRestoreLines(t *testing.T) (commands []string, outputs [][]string) {
	t.Helper()

	data, err := os.ReadFile(backupSets + "backup69.log")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(data), "\n")

	start := 0
	for start < len(lines) && !strings.Contains(lines[start], "Part 4: restores") {
		start++
	}

	command := regexp.MustCompile(`^\$ +(\S.*)$`)
	run := regexp.MustCompile(`^(BACKUP(/LOG)? |CREATE/DIRECTORY|SET DEFAULT|DIRECTORY/(SIZE|OWNER))`)

	// The save sets' own listing shows when they were copied here.
	skip := regexp.MustCompile(`^DIRECTORY.* \[SETS\]$`)
	ranLast := false

	for i := start; i < len(lines); i++ {
		m := command.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}

		line := m[1]

		switch {
		case line == "SHOW SYMBOL $STATUS":
			if !ranLast {
				continue
			}
		case run.MatchString(line) && !skip.MatchString(line):
			ranLast = true
		default:
			if strings.HasPrefix(line, "BACKUP") {
				ranLast = false
			}

			continue
		}

		var out []string

		for i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "$") {
			i++
			out = append(out, lines[i])
		}

		commands = append(commands, line)
		outputs = append(outputs, out)
	}

	return commands, outputs
}

// directoryTimes are the times DIRECTORY/DATE shows for the directory
// files a restore made, which are the run's own.
var directoryTimes = regexp.MustCompile(`^(\S+\.DIR;\d+ .*)\d\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d\.\d\d$`)

// maskRestoreOutput masks what differs from run to run in a restore's
// output: a made directory's DIRECTORY/DATE time.
func maskRestoreOutput(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = directoryTimes.ReplaceAllString(l, "${1}(time)")
	}

	return strings.Join(lines, "\n")
}

// TestBackupRestoreOracle replays the restores of VMS's log of the probe
// (Parts 4 to 6) on govax, on a volume holding the same save sets, and
// matches what VMS wrote line for line: /LOG's CREDIR and CREATED lines
// for every output directory form, /SELECT and /EXCLUDE, a new name, one
// version, the /BY_OWNER options (with DIRECTORY/OWNER of the results),
// an existing file without and with /NEW_VERSION, /OVERLAY, and
// /REPLACE, the other save sets, a /SELECT that matches nothing, and the
// damaged save sets (VBNMISSING, XORERRS); the DIRECTORY reports of the
// trees made; and each $STATUS. The [B69] tree the restores to [*...]
// find is restored from B69.BCK first, as the probe made it.
func TestBackupRestoreOracle(t *testing.T) {
	d, _, output := mountBackupSets(t)

	if err := d.Dispatch("BACKUP [SETS]B69.BCK/SAVE_SET [*...]/BY_OWNER=ORIGINAL"); err != nil {
		t.Fatalf("restoring [B69]: %v", err)
	}

	output()

	commands, outputs := backupRestoreLines(t)
	if len(commands) < 40 {
		t.Fatalf("%d commands from the log", len(commands))
	}

	for i, cmd := range commands {
		err := d.Dispatch(cmd)
		got := output()

		if err != nil && !vmserrors.MessageInhibited(err) {
			got += "%" + err.Error() + "\n"
		}

		want := strings.Join(outputs[i], "\n")
		if diff := firstDifference(maskRestoreOutput(strings.TrimRight(got, "\n")), maskRestoreOutput(strings.TrimRight(want, "\n"))); diff != "" {
			t.Errorf("%s: %s", cmd, diff)
		}
	}

	t.Logf("%d commands compared", len(commands))

	// [R7] after the version qualifiers: TEXT.TXT;2 from /NEW_VERSION,
	// and ;1 overlaid and then replaced, every header the saved file's
	// (VMS's r7.lst), the file IDs masked.
	if err := d.Dispatch("DIRECTORY/FULL [R7]"); err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(backupSets + "r7.lst")
	if err != nil {
		t.Fatal(err)
	}

	fileIDs := regexp.MustCompile(`File ID:  \(\d+,\d+,0\)\s*`)
	mask := func(s string) string { return strings.TrimSpace(fileIDs.ReplaceAllString(s, "File ID: (masked)")) }

	if diff := firstDifference(mask(output()), mask(string(want))); diff != "" {
		t.Errorf("DIRECTORY/FULL [R7]: %s", diff)
	}
}

// TestBackupRestoreHost: a host save set restored to a host directory,
// flat and as a tree ("dir/..." for [dir...]), each file named as COPY
// names one, its data VMS's (PI.EXE byte for byte); /LOG's CREDIR names
// the deepest directory made. Without /LOG nothing is shown.
func TestBackupRestoreHost(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	out := filepath.Join(t.TempDir(), "out")
	set := `"` + backupSets + `pi.bck"/SAVE_SET`

	if err := d.Dispatch("BACKUP/LOG " + set + ` "` + out + `/..."`); err != nil {
		t.Fatal(err)
	}

	tree := filepath.Join(out, "XFER", "PI")
	want := strings.Join([]string{
		"%BACKUP-S-CREDIR, created directory " + tree,
		"%BACKUP-S-CREATED, created " + filepath.Join(tree, "PI.EXE;1"),
		"%BACKUP-S-CREATED, created " + filepath.Join(tree, "PI.MAR;1"),
		"%BACKUP-S-CREATED, created " + filepath.Join(tree, "PI.OBJ;1"),
		"",
	}, "\n")

	if got := buf.String(); got != want {
		t.Errorf("output:\n%s\nwant\n%s", got, want)
	}

	got, err := os.ReadFile(filepath.Join(tree, "PI.EXE;1"))
	if err != nil {
		t.Fatal(err)
	}

	exe, err := os.ReadFile(backupSets + "pi.exe")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, exe) {
		t.Error("PI.EXE isn't VMS's")
	}

	buf.Reset()

	flat := filepath.Join(t.TempDir(), "flat")
	if err := d.Dispatch("BACKUP " + set + ` "` + flat + `"`); err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Errorf("without /LOG: %q", buf.String())
	}

	if _, err := os.Stat(filepath.Join(flat, "PI.MAR;1")); err != nil {
		t.Error(err)
	}
}

// TestBackupRestoreBadOwner: /BY_OWNER with a UIC that isn't one is
// SS$_IVIDENT, and nothing is restored.
func TestBackupRestoreBadOwner(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	err := d.Dispatch(`BACKUP "` + backupSets + `pi.bck"/SAVE_SET DUA0:[R]/BY_OWNER=[NOT,A,UIC]`)
	if err == nil || !strings.Contains(err.Error(), "IVIDENT") {
		t.Errorf("%v, want IVIDENT", err)
	}

	if _, err := c.cmd().ContainerSession.FileAttributes("DUA0:[R]PI.MAR;1"); err == nil {
		t.Error("PI.MAR was restored")
	}
}
