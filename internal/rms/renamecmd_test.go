package rms

import (
	"strings"
	"testing"
)

// session is a Session on fx's volume, defaulting to DUA0:[A].
func (fx renameFixture) session(t *testing.T) *Session {
	t.Helper()

	s := NewSession(fx.ctx.Mounts)
	s.Logicals = fx.ctx.Logicals

	if err := s.SetDefault("DUA0:[A]"); err != nil {
		t.Fatal(err)
	}

	return s
}

// renameCmd runs RENAME and returns its results, failing the test on any
// that isn't a success.
func renameCmd(t *testing.T, s *Session, inputs []string, output string, opts RenameOptions) []RenamedFile {
	t.Helper()

	results := s.Rename(inputs, output, opts)
	for _, r := range results {
		if !r.OK() {
			t.Errorf("RENAME %v %s: %s -> %s failed: %#x/%#x at stage %d", inputs, output, r.Old, r.New, r.Status, r.STV, r.Stage)
		}
	}

	return results
}

// wantRenamed checks results' old and new names, in order.
func wantRenamed(t *testing.T, results []RenamedFile, pairs ...string) {
	t.Helper()

	got := make([]string, 0)

	for _, r := range results {
		got = append(got, r.Old, r.New)
	}

	if strings.Join(got, " ") != strings.Join(pairs, " ") {
		t.Errorf("renamed:\n  %s\nwant:\n  %s", strings.Join(got, " "), strings.Join(pairs, " "))
	}
}

var defaultRename = RenameOptions{NewVersion: true}

// TestSessionRename_outputDefaultsFromInput: what the output leaves out
// is the input file's.
func TestSessionRename_outputDefaultsFromInput(t *testing.T) {
	fx := newRenameFixture(t)
	fx.create(t, fx.a, "X.TXT", 0)
	s := fx.session(t)

	wantRenamed(t, renameCmd(t, s, []string{"X.TXT"}, ".DAT", defaultRename),
		"DUA0:[A]X.TXT;1", "DUA0:[A]X.DAT;1")

	wantRenamed(t, renameCmd(t, s, []string{"X.DAT"}, "[B]", defaultRename),
		"DUA0:[A]X.DAT;1", "DUA0:[B]X.DAT;1")

	wantRenamed(t, renameCmd(t, s, []string{"[B]X.DAT"}, "Y", defaultRename),
		"DUA0:[B]X.DAT;1", "DUA0:[B]Y.DAT;1")
}

// TestSessionRename_versions covers LIB$RENAME_FILE's version rules.
func TestSessionRename_versions(t *testing.T) {
	fx := newRenameFixture(t)
	fx.create(t, fx.a, "V.TXT", 3)
	fx.create(t, fx.a, "V.TXT", 5)
	fx.create(t, fx.b, "W.TXT", 8)
	s := fx.session(t)

	// No input version: the highest, given the next version of its new
	// name.
	wantRenamed(t, renameCmd(t, s, []string{"V.TXT"}, "[B]W.TXT", defaultRename),
		"DUA0:[A]V.TXT;5", "DUA0:[B]W.TXT;9")

	// An explicit input version: kept.
	wantRenamed(t, renameCmd(t, s, []string{"V.TXT;3"}, "[B]W.TXT", defaultRename),
		"DUA0:[A]V.TXT;3", "DUA0:[B]W.TXT;3")

	// /NONEW_VERSION: kept, even with no input version.
	wantRenamed(t, renameCmd(t, s, []string{"[B]W.TXT"}, "[A]Z.TXT", RenameOptions{}),
		"DUA0:[B]W.TXT;9", "DUA0:[A]Z.TXT;9")

	// ";*": every version, each keeping its own.
	wantRenamed(t, renameCmd(t, s, []string{"[B]W.TXT;*"}, "[A]*.OLD", defaultRename),
		"DUA0:[B]W.TXT;8", "DUA0:[A]W.OLD;8", "DUA0:[B]W.TXT;3", "DUA0:[A]W.OLD;3")

	// An explicit output version, and "*" for the input's.
	wantRenamed(t, renameCmd(t, s, []string{"W.OLD;8"}, "W.OLD;20", defaultRename),
		"DUA0:[A]W.OLD;8", "DUA0:[A]W.OLD;20")

	wantRenamed(t, renameCmd(t, s, []string{"W.OLD"}, "W.NEW;*", defaultRename),
		"DUA0:[A]W.OLD;20", "DUA0:[A]W.NEW;20")
}

// TestSessionRename_wildcardsAndLists: a wildcard input renames every
// match; a list's device and directory carry from item to item.
func TestSessionRename_wildcardsAndLists(t *testing.T) {
	fx := newRenameFixture(t)
	fx.create(t, fx.a, "P.TXT", 0)
	fx.create(t, fx.a, "Q.TXT", 0)
	fx.create(t, fx.b, "R.DAT", 0)
	fx.create(t, fx.b, "S.DAT", 0)
	s := fx.session(t)

	wantRenamed(t, renameCmd(t, s, []string{"*.TXT"}, "*.OLD", defaultRename),
		"DUA0:[A]P.TXT;1", "DUA0:[A]P.OLD;1", "DUA0:[A]Q.TXT;1", "DUA0:[A]Q.OLD;1")

	// S.DAT is in [B], R.DAT's directory, not the default [A].
	wantRenamed(t, renameCmd(t, s, []string{"[B]R.DAT", "S.DAT"}, "*.KEEP", defaultRename),
		"DUA0:[B]R.DAT;1", "DUA0:[B]R.KEEP;1", "DUA0:[B]S.DAT;1", "DUA0:[B]S.KEEP;1")
}

// TestSessionRename_movesDirectory: a directory file moves like any
// other, taking its contents.
func TestSessionRename_movesDirectory(t *testing.T) {
	fx := newRenameFixture(t)
	inside := fx.create(t, fx.b, "IN.TXT", 0)
	s := fx.session(t)

	wantRenamed(t, renameCmd(t, s, []string{"[000000]B.DIR;1"}, "[A]", RenameOptions{}),
		"DUA0:[000000]B.DIR;1", "DUA0:[A]B.DIR;1")

	if got, ok := fx.lookup(t, []string{"A", "B"}, "IN.TXT", 0); !ok || got != inside {
		t.Errorf("[A.B]IN.TXT = %v, %v; want %v", got, ok, inside)
	}
}

// TestSessionRename_failures: each failure is reported with its stage and
// status, and the other files are still renamed.
func TestSessionRename_failures(t *testing.T) {
	fx := newRenameFixture(t)
	fx.create(t, fx.a, "X.TXT", 0)
	fx.create(t, fx.a, "Y.TXT", 0)
	fx.create(t, fx.a, "TAKEN.TXT", 1)

	if err := fx.ctx.Mounts.Mount("DUA1", newTestVolumeFile(t, "OTHER"), true); err != nil {
		t.Fatal(err)
	}

	s := fx.session(t)

	for _, tc := range []struct {
		what          string
		inputs        []string
		output        string
		stage         RenameStage
		sts, stv      uint32
		oldName, name string
	}{
		{"a missing file", []string{"NOPE.TXT"}, "Z.TXT", RenameSearching, rmsFileNotFound, rmsFileNotFound, "DUA0:[A]NOPE.TXT;", ""},
		{"a missing directory", []string{"[NOPE]X.TXT"}, "Z.TXT", RenameSearching, rmsDirNotFound, rmsDirNotFound, "DUA0:[NOPE]X.TXT;", ""},
		{"an unmounted device", []string{"DUA7:[A]X.TXT"}, "Z.TXT", RenameSearching, rmsDeviceNotReady, rmsDeviceNotReady, "DUA7:[A]X.TXT;", ""},
		{"a partial output wildcard", []string{"X.TXT"}, "Z*.TXT", RenameParsing, rmsWildcardError, rmsWildcardError, "DUA0:[A]X.TXT;1", "DUA0:[A]Z*.TXT;"},
		{"another device", []string{"X.TXT"}, "DUA1:[000000]", RenameRenaming, rmsDeviceError, rmsDeviceError, "DUA0:[A]X.TXT;1", "DUA1:[000000]X.TXT;"},
		{"an existing version", []string{"X.TXT"}, "TAKEN.TXT;1", RenameRenaming, rmsEnterFailed, ssDuplicateFileName, "DUA0:[A]X.TXT;1", "DUA0:[A]TAKEN.TXT;1"},
	} {
		results := s.Rename(tc.inputs, tc.output, defaultRename)
		if len(results) != 1 {
			t.Errorf("%s: %d results, want 1", tc.what, len(results))

			continue
		}

		r := results[0]
		if r.OK() || r.Stage != tc.stage || r.Status != tc.sts || r.STV != tc.stv || r.Old != tc.oldName || r.New != tc.name {
			t.Errorf("%s: got %+v\nwant stage %d, %#x/%#x, %s -> %s", tc.what, r, tc.stage, tc.sts, tc.stv, tc.oldName, tc.name)
		}
	}

	// A failure doesn't stop the rest of the list.
	results := s.Rename([]string{"NOPE.TXT", "Y.TXT"}, "Y.NEW", defaultRename)
	if len(results) != 2 || results[0].OK() || !results[1].OK() {
		t.Errorf("a list with one bad item: %+v", results)
	}

	if _, ok := fx.lookup(t, []string{"A"}, "X.TXT", 1); !ok {
		t.Error("[A]X.TXT;1 is gone after only failed renames")
	}
}
