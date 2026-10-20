package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestBackupCompareOracle replays Part 5 of VMS's log of the probe, the
// compares, on govax, and matches what VMS wrote line for line: B69TXT
// against the [B69] tree (every file COMPARED, the NOBACKUP file's header
// only), TEXT against a restored copy, the copy with a line appended
// (VERIFYERR for block 1, then COMPARED), and the copy deleted (OPENIN
// with RMS$_FNF, then NOFILES), and $STATUS after each. The [B69] tree is
// restored from B69.BCK first, as the probe made it.
func TestBackupCompareOracle(t *testing.T) {
	d, _, output := mountBackupSets(t)

	if err := d.Dispatch("BACKUP [SETS]B69.BCK/SAVE_SET [*...]/BY_OWNER=ORIGINAL"); err != nil {
		t.Fatalf("restoring [B69]: %v", err)
	}

	output()

	data, err := os.ReadFile(backupSets + "backup69.log")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(data), "\n")

	start := 0
	for start < len(lines) && !strings.Contains(lines[start], "Part 5: compares") {
		start++
	}

	ran := 0

	for i := start; i < len(lines) && !strings.Contains(lines[i], "The restore to [*...]"); i++ {
		if !strings.HasPrefix(lines[i], "$ ") || strings.HasPrefix(lines[i], "$ !") {
			continue
		}

		cmd := strings.TrimPrefix(lines[i], "$ ")

		var want []string

		for i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "$") {
			i++
			want = append(want, lines[i])
		}

		err := d.Dispatch(cmd)
		got := output()

		if err != nil && !vmserrors.MessageInhibited(err) {
			got += "%" + err.Error() + "\n"
		}

		if diff := firstDifference(strings.TrimRight(got, "\n"), strings.Join(want, "\n")); diff != "" {
			t.Errorf("%s: %s", cmd, diff)
		}

		ran++
	}

	if ran < 10 {
		t.Errorf("only %d commands from the log", ran)
	}
}

// TestBackupCompareSelections: /SELECT limits a compare to the files it
// names, and a file whose blocks are past the end of the one compared
// with differs in each of them.
func TestBackupCompareSelections(t *testing.T) {
	d, _, output := mountBackupSets(t)

	if err := d.Dispatch("BACKUP [SETS]PI.BCK/SAVE_SET [CMP]"); err != nil {
		t.Fatal(err)
	}

	// PI.MAR in place of PI.EXE: every block of the image differs, those
	// past the source's end too.
	for _, line := range []string{"DELETE [CMP]PI.EXE;*", "COPY [CMP]PI.MAR [CMP]PI.EXE;1"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	output()

	err := d.Dispatch("BACKUP/COMPARE/LOG [SETS]PI.BCK/SAVE_SET/SELECT=(*.MAR,*.OBJ) [CMP]")
	if err != nil {
		t.Fatal(err)
	}

	want := "%BACKUP-S-COMPARED, compared DUA1:[CMP]PI.MAR;1\n%BACKUP-S-COMPARED, compared DUA1:[CMP]PI.OBJ;1\n"
	if got := output(); got != want {
		t.Errorf("selected:\n%s\nwant\n%s", got, want)
	}

	err = d.Dispatch("BACKUP/COMPARE [SETS]PI.BCK/SAVE_SET/SELECT=PI.EXE [CMP]")
	if err == nil || !strings.Contains(err.Error(), "VERIFYERR") {
		t.Errorf("status %v, want VERIFYERR", err)
	}

	got := output()
	if !strings.HasPrefix(got, "%BACKUP-E-VERIFYERR, verification error for block 1 of DUA1:[CMP]PI.EXE;1\n") {
		t.Errorf("PI.EXE:\n%s", got)
	}

	if strings.Contains(got, "COMPARED") {
		t.Error("COMPARED without /LOG")
	}
}

// TestBackupCompareHost: a restore to the host compares clean against its
// save set, and a host file changed afterwards differs in the piece the
// change is in.
func TestBackupCompareHost(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	out := filepath.Join(t.TempDir(), "out")
	set := `"` + backupSets + `pi.bck"/SAVE_SET`

	if err := d.Dispatch("BACKUP " + set + ` "` + out + `"`); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	if err := d.Dispatch("BACKUP/COMPARE/LOG " + set + ` "` + out + `"`); err != nil {
		t.Fatal(err)
	}

	want := ""
	for _, name := range []string{"PI.EXE;1", "PI.MAR;1", "PI.OBJ;1"} {
		want += "%BACKUP-S-COMPARED, compared " + filepath.Join(out, name) + "\n"
	}

	if got := buf.String(); got != want {
		t.Errorf("clean:\n%s\nwant\n%s", got, want)
	}

	exe := filepath.Join(out, "PI.EXE;1")

	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}

	data[600] ^= 0xFF
	if err := os.WriteFile(exe, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(out, "PI.OBJ;1")); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	if err := d.Dispatch("BACKUP/COMPARE " + set + ` "` + out + `"`); err == nil {
		t.Error("no error status")
	}

	want = "%BACKUP-E-VERIFYERR, verification error for block 2 of " + exe + "\n" +
		"%BACKUP-E-OPENIN, error opening " + filepath.Join(out, "PI.OBJ;1") + " as input\n" +
		"-RMS-E-FNF, file not found\n"
	if got := buf.String(); got != want {
		t.Errorf("changed:\n%s\nwant\n%s", got, want)
	}
}
