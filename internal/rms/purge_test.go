package rms

import (
	"errors"
	"testing"

	"github.com/tucats/ods2/volume"
)

// newPurgeTestSession builds a fresh, writable, mounted test volume (device
// DUA0, reusing newTestVolumeFile from mount_test.go) and populates its
// master file directory with FOO.TXT;1, FOO.TXT;2, FOO.TXT;3, BAR.TXT;1,
// BAR.TXT;2, and BAZ.TXT;1 -- enough distinct names and version counts to
// exercise the default /LIMIT, an explicit /LIMIT, and a glob matching
// several distinct names in one command, the same fixture shape ods2's own
// purge_test.go uses.
func newPurgeTestSession(t *testing.T) (*Session, *volume.Volume) {
	t.Helper()

	mounts := NewMountTable()
	path := newTestVolumeFile(t, "PURGEVOL")

	if err := mounts.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	vol, ok := mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after Mount = not found")
	}

	for _, name := range []string{"FOO.TXT", "FOO.TXT", "FOO.TXT", "BAR.TXT", "BAR.TXT", "BAZ.TXT"} {
		createTestFile(t, vol, name, "content of "+name)
	}

	s := NewSession(mounts)
	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	return s, vol
}

// TestSession_purgeDefaultLimitKeepsOnlyHighestVersion confirms the default
// keep=1 leaves only each name's newest version.
func TestSession_purgeDefaultLimitKeepsOnlyHighestVersion(t *testing.T) {
	s, vol := newPurgeTestSession(t)

	purged, err := s.Purge("*.TXT", 1)
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}

	if len(purged) != 3 {
		t.Errorf("Purge(*.TXT) touched %d distinct names, want 3 (FOO/BAR/BAZ)", len(purged))
	}

	names := mfdEntryNames(t, vol)
	for _, want := range []string{"FOO.TXT;3", "BAR.TXT;2", "BAZ.TXT;1"} {
		if !containsName(names, want) {
			t.Errorf("directory entries = %v, want %s to survive (the highest version)", names, want)
		}
	}

	for _, gone := range []string{"FOO.TXT;1", "FOO.TXT;2", "BAR.TXT;1"} {
		if containsName(names, gone) {
			t.Errorf("directory entries = %v, want %s removed (default /LIMIT=1)", names, gone)
		}
	}
}

// TestSession_purgeExplicitLimitKeepsThatManyVersions confirms an explicit
// keep count other than the default is honored.
func TestSession_purgeExplicitLimitKeepsThatManyVersions(t *testing.T) {
	s, vol := newPurgeTestSession(t)

	if _, err := s.Purge("FOO.TXT", 2); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	names := mfdEntryNames(t, vol)
	for _, want := range []string{"FOO.TXT;2", "FOO.TXT;3", "BAR.TXT;1", "BAR.TXT;2", "BAZ.TXT;1"} {
		if !containsName(names, want) {
			t.Errorf("directory entries = %v, want %s untouched/surviving", names, want)
		}
	}

	if containsName(names, "FOO.TXT;1") {
		t.Errorf("directory entries = %v, want FOO.TXT;1 removed (/LIMIT=2)", names)
	}
}

// TestSession_purgeGlobCoversMultipleDistinctNames confirms an empty
// specText defaults to "*.*", touching every distinct name on the volume
// (including its own reserved single-version files, which the default
// /LIMIT=1 must leave alone rather than erroring on).
func TestSession_purgeGlobCoversMultipleDistinctNames(t *testing.T) {
	s, vol := newPurgeTestSession(t)

	if _, err := s.Purge("", 1); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	names := mfdEntryNames(t, vol)
	for _, want := range []string{"FOO.TXT;3", "BAR.TXT;2", "BAZ.TXT;1"} {
		if !containsName(names, want) {
			t.Errorf("directory entries = %v, want %s to survive", names, want)
		}
	}

	for _, gone := range []string{"FOO.TXT;1", "FOO.TXT;2", "BAR.TXT;1"} {
		if containsName(names, gone) {
			t.Errorf("directory entries = %v, want %s removed", names, gone)
		}
	}
}

// TestSession_purgeIgnoresTypedVersion confirms a version typed on the
// spec (which DELETE would honor, or reject as missing) has no effect on
// PURGE at all -- every surviving version is still considered, matching
// ods2's own "spec.Version = \"*\"" override.
func TestSession_purgeIgnoresTypedVersion(t *testing.T) {
	s, vol := newPurgeTestSession(t)

	if _, err := s.Purge("FOO.TXT;1", 1); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	names := mfdEntryNames(t, vol)
	if !containsName(names, "FOO.TXT;3") {
		t.Errorf("directory entries = %v, want FOO.TXT;3 to survive", names)
	}

	if containsName(names, "FOO.TXT;1") || containsName(names, "FOO.TXT;2") {
		t.Errorf("directory entries = %v, want FOO.TXT;1 and ;2 removed despite the spec naming only ;1", names)
	}
}

// TestSession_purgeZeroLimitRejected confirms keep=0 is rejected as an
// *InvalidLimitError before anything on the volume is touched.
func TestSession_purgeZeroLimitRejected(t *testing.T) {
	s, vol := newPurgeTestSession(t)

	before := mfdEntryNames(t, vol)

	_, err := s.Purge("FOO.TXT", 0)
	if err == nil {
		t.Fatal("Purge with /LIMIT=0 = nil error, want an error")
	}

	var badLimit *InvalidLimitError
	if !errors.As(err, &badLimit) {
		t.Fatalf("Purge error = %v, want a *InvalidLimitError", err)
	}

	if badLimit.Limit != 0 {
		t.Errorf("InvalidLimitError.Limit = %d, want 0", badLimit.Limit)
	}

	after := mfdEntryNames(t, vol)
	if len(before) != len(after) {
		t.Errorf("directory entries changed despite the rejected /LIMIT=0 command: before %v, after %v", before, after)
	}
}

// TestSession_purgeNotMounted confirms specText naming a device with
// nothing mounted on it surfaces as an *NotMountedError, the same failure
// mode Session.Delete/Session.Directory already report this way.
func TestSession_purgeNotMounted(t *testing.T) {
	s := NewSession(NewMountTable())

	_, err := s.Purge("DUB0:*.*", 1)
	if err == nil {
		t.Fatal("Purge against an unmounted device = nil error, want an error")
	}

	var notMounted *NotMountedError
	if !errors.As(err, &notMounted) {
		t.Fatalf("Purge error = %v, want a *NotMountedError", err)
	}

	if notMounted.Device != "DUB0" {
		t.Errorf("NotMountedError.Device = %q, want DUB0", notMounted.Device)
	}
}

// TestSession_purgeBadFileSpec confirms a malformed file specification is
// reported as a plain error, and specifically neither a *NotMountedError
// nor a *InvalidLimitError.
func TestSession_purgeBadFileSpec(t *testing.T) {
	s, _ := newPurgeTestSession(t)

	_, err := s.Purge("DUA0:[UNTERMINATED", 1)
	if err == nil {
		t.Fatal("Purge with an unterminated directory bracket = nil error, want an error")
	}

	var notMounted *NotMountedError
	if errors.As(err, &notMounted) {
		t.Errorf("Purge error = %v, want a plain error, not a *NotMountedError", err)
	}

	var badLimit *InvalidLimitError
	if errors.As(err, &badLimit) {
		t.Errorf("Purge error = %v, want a plain error, not a *InvalidLimitError", err)
	}
}

// TestSession_purgeNoMatchesIsNotAnError confirms purging a name pattern
// that matches nothing on the volume is a normal no-op, not an error --
// matching ods2's own cmdPurge, which has no special "nothing matched"
// handling at all.
func TestSession_purgeNoMatchesIsNotAnError(t *testing.T) {
	s, _ := newPurgeTestSession(t)

	purged, err := s.Purge("*.NOSUCHTYPE", 1)
	if err != nil {
		t.Fatalf("Purge matching nothing: %v", err)
	}

	if len(purged) != 0 {
		t.Errorf("Purge(*.NOSUCHTYPE) touched %v, want no names touched", purged)
	}
}
