package dclcmd

import (
	"strings"
	"testing"
)

// TestBackupSelection: /SELECT and /EXCLUDE as VMS V7.1 applied them in
// the probe's restores, and the part of a file's directory a restore to
// [dir...] carries: all of it without /SELECT, the ellipsis's part with
// it, and a /SELECT with no directory taken as [*...].
func TestBackupSelection(t *testing.T) {
	for _, tc := range []struct {
		selects, excludes []string
		name              string
		selected          bool
		carried           string
	}{
		// No /SELECT: every file, its whole directory carried ([R1.B69]).
		{nil, nil, "[B69]ACL.TXT;1", true, "B69"},
		{nil, nil, "[B69.SUB1.SUB2]B.TXT;1", true, "B69.SUB1.SUB2"},

		// /SELECT=[B69.SUB1...]: [R3] and [R3.SUB2].
		{[]string{"[B69.SUB1...]"}, nil, "[B69.SUB1]A.TXT;1", true, ""},
		{[]string{"[B69.SUB1...]"}, nil, "[B69.SUB1.SUB2]BIG.DAT;1", true, "SUB2"},
		{[]string{"[B69.SUB1...]"}, nil, "[B69]SUB1.DIR;1", false, ""},

		// /SELECT=*.TXT is [*...]*.TXT: [R5] and [R5.SUB1].
		{[]string{"*.TXT"}, nil, "[B69]ACL.TXT;1", true, ""},
		{[]string{"*.TXT"}, nil, "[B69.SUB1]A.TXT;1", true, "SUB1"},
		{[]string{"*.TXT"}, nil, "[B69.SUB1]MEDIUM.DAT;1", false, ""},

		// A name with no version selects every version; one with a
		// version, that one; ";" and ";-1" every version.
		{[]string{"[B69]MULTI.TXT"}, nil, "[B69]MULTI.TXT;3", true, ""},
		{[]string{"[B69]MULTI.TXT;2"}, nil, "[B69]MULTI.TXT;3", false, ""},
		{[]string{"[B69]MULTI.TXT;2"}, nil, "[B69]MULTI.TXT;2", true, ""},
		{[]string{"[B69]MULTI.TXT;"}, nil, "[B69]MULTI.TXT;1", true, ""},
		{[]string{"[B69]MULTI.TXT;-1"}, nil, "[B69]MULTI.TXT;3", true, ""},

		// Two selections, either; a device is ignored; case doesn't
		// matter.
		{[]string{"[B69]TEXT.TXT", "[B69]OWNED.TXT"}, nil, "[B69]OWNED.TXT;1", true, ""},
		{[]string{"dua1:[b69]text.txt"}, nil, "[B69]TEXT.TXT;1", true, ""},
		{[]string{"[NOSUCH]*.*"}, nil, "[B69]TEXT.TXT;1", false, ""},

		// /EXCLUDE=*.DAT, in any directory, and over /SELECT.
		{nil, []string{"*.DAT"}, "[B69.SUB1]MEDIUM.DAT;1", false, ""},
		{nil, []string{"*.DAT"}, "[B69.SUB1]A.TXT;1", true, "B69.SUB1"},
		{[]string{"*.*"}, []string{"[B69]TEXT.TXT"}, "[B69]TEXT.TXT;1", false, ""},

		// An ellipsis inside a directory carries what it matched.
		{[]string{"[B69...SUB2]"}, nil, "[B69.SUB1.SUB2]B.TXT;1", true, "SUB1"},
		{[]string{"[B69.%UB1]"}, nil, "[B69.SUB1]A.TXT;1", true, ""},
	} {
		carried, ok := newBackupSelection(tc.selects, tc.excludes).selected(tc.name)
		if ok != tc.selected || strings.Join(carried, ".") != tc.carried {
			t.Errorf("/SELECT=%v /EXCLUDE=%v %s: %v [%s], want %v [%s]",
				tc.selects, tc.excludes, tc.name, ok, strings.Join(carried, "."), tc.selected, tc.carried)
		}
	}
}

// TestBackupTargetLocation: where each output directory form puts a
// saved file, and the name, type, and version an output specifier gives.
func TestBackupTargetLocation(t *testing.T) {
	saved := parseSavedName("[B69.SUB1]A.TXT;3")
	carried := []string{"SUB1"}

	for _, tc := range []struct {
		target backupTarget
		want   string
	}{
		{backupTarget{device: "DUA1", dirs: []string{"R4"}}, "DUA1:[R4]A.TXT;3"},
		{backupTarget{device: "DUA1", dirs: []string{"R1"}, form: restoreTree}, "DUA1:[R1.SUB1]A.TXT;3"},
		{backupTarget{device: "DUA1", form: restoreSaved}, "DUA1:[B69.SUB1]A.TXT;3"},
		{backupTarget{device: "DUA1", form: restoreTree}, "DUA1:[SUB1]A.TXT;3"},
		{backupTarget{device: "DUA1"}, "DUA1:[000000]A.TXT;3"},
		{backupTarget{device: "DUA1", dirs: []string{"R9"}, name: "NEW", typ: "TXT"}, "DUA1:[R9]NEW.TXT;3"},
		{backupTarget{device: "DUA1", dirs: []string{"R9"}, name: "*", typ: "OLD", version: "7"}, "DUA1:[R9]A.OLD;7"},
		{backupTarget{host: true, path: "out"}, "out/A.TXT;3"},
		{backupTarget{host: true, path: "out", form: restoreTree}, "out/SUB1/A.TXT;3"},
	} {
		if got := tc.target.location(saved, carried).Name; got != tc.want {
			t.Errorf("%+v: %s, want %s", tc.target, got, tc.want)
		}
	}

	// A saved directory file goes to the host as a directory of its name.
	dir := parseSavedName("[B69]SUB1.DIR;1")
	if got := (backupTarget{host: true, path: "out", form: restoreTree}).location(dir, []string{"B69"}).Name; got != "out/B69/SUB1" {
		t.Errorf("host directory: %s", got)
	}
}
