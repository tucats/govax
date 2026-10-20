package dclcmd

import (
	"testing"

	"github.com/tucats/govax/internal/backup"
)

// TestRecordedCommand: the command a save set's summary records, as VMS
// recorded B69COM.BCK's (its /COMMENT's quotes gone).
func TestRecordedCommand(t *testing.T) {
	cases := []struct{ line, want string }{
		{`BACKUP [B69...]*.* [SETS]B69COM.BCK/SAVE_SET/COMMENT="Phase 69's probe, with a comment"`,
			"BACKUP [B69...]*.* [SETS]B69COM.BCK/SAVE_SET/COMMENT=Phase 69's probe, with a comment"},
		{"$ backup/log  [b69]*.*\t[sets]x.bck/save", "BACKUP/LOG [B69]*.* [SETS]X.BCK/SAVE"},
		{`BACKUP A.TXT "./x.bck"/SAVE`, "BACKUP A.TXT ./x.bck/SAVE"},
		{`BACKUP A.TXT X.BCK/SAVE/COMMENT="say ""hi"""`, `BACKUP A.TXT X.BCK/SAVE/COMMENT=say "hi"`},
	}

	for _, tc := range cases {
		if got := recordedCommand(tc.line); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.line, got, tc.want)
		}
	}
}

// TestStickyDefaults: an input list's elements take the device and
// directory before them.
func TestStickyDefaults(t *testing.T) {
	cases := []struct{ spec, previous, want string }{
		{"PI.*", "[B69]TEXT.TXT", "[B69]PI.*"},
		{"PI.*", "DUA1:[B69.SUB1]A.TXT", "DUA1:[B69.SUB1]PI.*"},
		{"PI.*", "DUA1:A.TXT", "DUA1:PI.*"},
		{"[X]PI.*", "[B69]TEXT.TXT", "[X]PI.*"},
		{"DUA2:PI.*", "[B69]TEXT.TXT", "DUA2:PI.*"},
		{"PI.*", "", "PI.*"},
		{"PI.*", "TEXT.TXT", "PI.*"},
	}

	for _, tc := range cases {
		if got := stickyDefaults(tc.spec, tc.previous); got != tc.want {
			t.Errorf("%q after %q: %q, want %q", tc.spec, tc.previous, got, tc.want)
		}
	}
}

// TestSaveFilter: /BY_OWNER (with wildcards), /SINCE and /BEFORE on each
// date, /BACKUP's rule for a file never backed up, and /EXCLUDE, which
// never excludes a directory file.
func TestSaveFilter(t *testing.T) {
	const system = 1<<16 | 4

	owned := backup.Attributes{Name: "[B69]OWNED.TXT;1", Owner: 0o200<<16 | 0o201, Revised: 2000, Created: 1000}
	plain := backup.Attributes{Name: "[B69]TEXT.TXT;1", Owner: system, Revised: 3000, Created: 1000, Backup: 2500}
	dir := backup.Attributes{Name: "[B69]SUB1.DIR;1", Owner: system, Characteristics: backup.CharDirectory}

	byOwner := func(text string) saveFilter {
		p, err := parseUICPattern(text, system)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}

		return saveFilter{exclude: newBackupSelection(nil, nil), owner: &p}
	}

	cases := []struct {
		name   string
		filter saveFilter
		file   backup.Attributes
		want   bool
	}{
		{"own UIC, other's file", byOwner(""), owned, false},
		{"own UIC, own file", byOwner(""), plain, true},
		{"[200,201]", byOwner("[200,201]"), owned, true},
		{"[200,*]", byOwner("[200,*]"), owned, true},
		{"[*,4]", byOwner("[*,4]"), owned, false},
		{"since, modified", saveFilter{exclude: newBackupSelection(nil, nil), since: 2500}, plain, true},
		{"since, modified, older", saveFilter{exclude: newBackupSelection(nil, nil), since: 2500}, owned, false},
		{"before, created", saveFilter{exclude: newBackupSelection(nil, nil), before: 1500, date: BackupCreated}, plain, true},
		{"since, backup, never", saveFilter{exclude: newBackupSelection(nil, nil), since: 1, date: BackupBackedUp}, owned, false},
		{"before, backup, never", saveFilter{exclude: newBackupSelection(nil, nil), before: 1, date: BackupBackedUp}, owned, true},
		{"before, backup, later", saveFilter{exclude: newBackupSelection(nil, nil), before: 2000, date: BackupBackedUp}, plain, false},
		{"excluded", saveFilter{exclude: newBackupSelection(nil, []string{"*.TXT"})}, plain, false},
		{"directory not excluded", saveFilter{exclude: newBackupSelection(nil, []string{"*.DIR"})}, dir, true},
	}

	for _, tc := range cases {
		if got := tc.filter.takes(tc.file); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}

	if _, err := parseUICPattern("[1,9]", system); err == nil {
		t.Error("[1,9]: no error for a member that isn't octal")
	}
}

// TestSaveSetName: the name a save set records, from its file's.
func TestSaveSetName(t *testing.T) {
	for full, want := range map[string]string{
		"DUA1:[SETS]B69.BCK;1": "B69.BCK",
		"DUA1:B69.BCK;12":      "B69.BCK",
		"/tmp/out/pi.bck":      "PI.BCK",
	} {
		if got := saveSetName(full, full[0] == '/'); got != want {
			t.Errorf("%s: %q, want %q", full, got, want)
		}
	}
}
