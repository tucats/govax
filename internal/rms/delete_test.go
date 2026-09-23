package rms

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// newDeleteTestSession builds a fresh, writable, mounted test volume (device
// DUA0, reusing newTestVolumeFile from mount_test.go, the same helper
// newDirectoryTestSession already relies on) and populates its master file
// directory with FOO.TXT;1, FOO.TXT;2, BAR.TXT;1, BAR.TXT;2, and BAZ.TXT;1
// -- enough distinct names and versions to exercise ";n", ";*", and a
// wildcarded name each deleting more than one file in a single call, the
// same fixture shape ods2's own delete_test.go uses.
func newDeleteTestSession(t *testing.T) (*Session, *volume.Volume) {
	t.Helper()

	mounts := NewMountTable()
	path := newTestVolumeFile(t, "DELVOL")

	if err := mounts.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	vol, ok := mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after Mount = not found")
	}

	for _, name := range []string{"FOO.TXT", "FOO.TXT", "BAR.TXT", "BAR.TXT", "BAZ.TXT"} {
		createTestFile(t, vol, name, "content of "+name)
	}

	s := NewSession(mounts)
	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	return s, vol
}

// mfdEntryNames returns vol's master file directory entries as
// "NAME.TYPE;version" strings, for easy before/after comparison in the
// tests below -- the same shape ods2's own delete_test.go's dirEntryNames
// helper produces.
func mfdEntryNames(t *testing.T, vol *volume.Volume) []string {
	t.Helper()

	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	entries, err := mfd.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = fmt.Sprintf("%s;%d", e.Name, e.Version)
	}

	return names
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}

	return false
}

// TestSession_deleteSpecificVersion confirms the basic happy path: deleting
// one exact "NAME.TYPE;version" removes only that entry, leaving every
// other name/version untouched, and reports it back as a DeletedFile.
func TestSession_deleteSpecificVersion(t *testing.T) {
	s, vol := newDeleteTestSession(t)

	deleted, err := s.Delete("FOO.TXT;1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(deleted) != 1 || deleted[0].Name != "FOO" || deleted[0].Type != "TXT" || deleted[0].Version != 1 {
		t.Errorf("Delete returned %+v, want one {FOO TXT 1}", deleted)
	}

	names := mfdEntryNames(t, vol)
	for _, want := range []string{"FOO.TXT;2", "BAR.TXT;1", "BAR.TXT;2", "BAZ.TXT;1"} {
		if !containsName(names, want) {
			t.Errorf("directory entries = %v, want %s still present", names, want)
		}
	}

	if containsName(names, "FOO.TXT;1") {
		t.Errorf("directory entries = %v, want FOO.TXT;1 removed", names)
	}
}

// TestSession_deleteAllVersionsOfAName confirms ";*" removes every version
// of a name at once, leaving other names untouched.
func TestSession_deleteAllVersionsOfAName(t *testing.T) {
	s, vol := newDeleteTestSession(t)

	deleted, err := s.Delete("FOO.TXT;*")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(deleted) != 2 {
		t.Errorf("Delete(FOO.TXT;*) returned %d files, want 2", len(deleted))
	}

	names := mfdEntryNames(t, vol)
	if containsName(names, "FOO.TXT;1") || containsName(names, "FOO.TXT;2") {
		t.Errorf("directory entries = %v, want every FOO.TXT version removed", names)
	}

	for _, want := range []string{"BAR.TXT;1", "BAR.TXT;2", "BAZ.TXT;1"} {
		if !containsName(names, want) {
			t.Errorf("directory entries = %v, want %s untouched", names, want)
		}
	}
}

// TestSession_deleteWildcardNameAcrossMultipleFiles confirms a wildcarded
// name pattern ("*.TXT;2") can hit several distinct names' matching
// versions in a single Delete call.
func TestSession_deleteWildcardNameAcrossMultipleFiles(t *testing.T) {
	s, vol := newDeleteTestSession(t)

	deleted, err := s.Delete("*.TXT;2")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(deleted) != 2 {
		t.Errorf("Delete(*.TXT;2) returned %d files, want 2", len(deleted))
	}

	names := mfdEntryNames(t, vol)
	if containsName(names, "FOO.TXT;2") || containsName(names, "BAR.TXT;2") {
		t.Errorf("directory entries = %v, want every ;2 version removed", names)
	}

	for _, want := range []string{"FOO.TXT;1", "BAR.TXT;1", "BAZ.TXT;1"} {
		if !containsName(names, want) {
			t.Errorf("directory entries = %v, want %s untouched", names, want)
		}
	}
}

// TestSession_deleteWithoutVersionIsRejected confirms a bare "FOO.TXT" (no
// version at all) is rejected as a *VersionRequiredError before anything on
// the volume is touched -- DELETE's own deliberate refusal to guess "the
// newest version" the way DIRECTORY would.
func TestSession_deleteWithoutVersionIsRejected(t *testing.T) {
	s, vol := newDeleteTestSession(t)

	before := mfdEntryNames(t, vol)

	_, err := s.Delete("FOO.TXT")
	if err == nil {
		t.Fatal("Delete with no version = nil error, want an error")
	}

	var versionRequired *VersionRequiredError
	if !errors.As(err, &versionRequired) {
		t.Fatalf("Delete error = %v, want a *VersionRequiredError", err)
	}

	after := mfdEntryNames(t, vol)
	if len(before) != len(after) {
		t.Errorf("directory entries changed despite the rejected command: before %v, after %v", before, after)
	}
}

// TestSession_deleteWithTrailingSemicolonIsRejected confirms "FOO.TXT;" (an
// empty version field, which filespec.Parse can't tell apart from "no
// version typed at all") is rejected the same way a bare "FOO.TXT" is.
func TestSession_deleteWithTrailingSemicolonIsRejected(t *testing.T) {
	s, _ := newDeleteTestSession(t)

	_, err := s.Delete("FOO.TXT;")
	if err == nil {
		t.Fatal("Delete with an empty version (FOO.TXT;) = nil error, want an error")
	}

	var versionRequired *VersionRequiredError
	if !errors.As(err, &versionRequired) {
		t.Fatalf("Delete error = %v, want a *VersionRequiredError", err)
	}
}

// TestSession_deleteNonexistentFileErrors confirms a name that matches
// nothing at all on the volume surfaces as a *NotFoundError.
func TestSession_deleteNonexistentFileErrors(t *testing.T) {
	s, _ := newDeleteTestSession(t)

	_, err := s.Delete("NOSUCHFILE.TXT;1")
	if err == nil {
		t.Fatal("Delete of a nonexistent file = nil error, want an error")
	}

	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("Delete error = %v, want a *NotFoundError", err)
	}
}

// TestSession_deleteNonexistentVersionErrors confirms an existing name but a
// version nothing on the volume has also surfaces as a *NotFoundError, not a
// crash or a silent no-op.
func TestSession_deleteNonexistentVersionErrors(t *testing.T) {
	s, _ := newDeleteTestSession(t)

	_, err := s.Delete("FOO.TXT;9")
	if err == nil {
		t.Fatal("Delete of a nonexistent version = nil error, want an error")
	}

	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("Delete error = %v, want a *NotFoundError", err)
	}
}

// TestSession_deleteReclaimsStorageForReuse confirms the freed header slot
// and blocks genuinely reach disk (not just this process's in-memory bitmap
// cache) by opening brand-new Bitmap/IndexBitmap instances afterward and
// successfully creating a new file through them -- mirroring ods2's own
// TestCmdDeleteReclaimsStorageForReuse.
func TestSession_deleteReclaimsStorageForReuse(t *testing.T) {
	s, vol := newDeleteTestSession(t)

	if _, err := s.Delete("FOO.TXT;1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	dev := vol.Devices[0]

	ib, err := volume.OpenIndexBitmap(dev)
	if err != nil {
		t.Fatalf("OpenIndexBitmap: %v", err)
	}

	if _, err := ib.FindFreeSlot(); err != nil {
		t.Errorf("FindFreeSlot after delete: %v (former header slot not reclaimed)", err)
	}

	bm, err := volume.OpenBitmap(dev)
	if err != nil {
		t.Fatalf("OpenBitmap: %v", err)
	}

	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory: %v", err)
	}

	f, err := vol.CreateFile(mfd, "NEW.TXT", ondisk.RecAttr{Format: ondisk.RecordFormatStreamLF}, bm, ib)
	if err != nil {
		t.Fatalf("CreateFile(NEW.TXT) after delete: %v", err)
	}

	if err := f.CloseWithFinalByte(0); err != nil {
		t.Fatalf("Close(NEW.TXT): %v", err)
	}

	names := mfdEntryNames(t, vol)
	if !containsName(names, "NEW.TXT;1") {
		t.Errorf("directory entries = %v, want NEW.TXT;1 to have been created successfully", names)
	}
}

// TestSession_deleteNotMounted confirms specText naming a device with
// nothing mounted on it surfaces as an *NotMountedError, the same failure
// mode Session.Directory already reports this way.
func TestSession_deleteNotMounted(t *testing.T) {
	s := NewSession(NewMountTable())

	_, err := s.Delete("DUB0:FOO.TXT;1")
	if err == nil {
		t.Fatal("Delete against an unmounted device = nil error, want an error")
	}

	var notMounted *NotMountedError
	if !errors.As(err, &notMounted) {
		t.Fatalf("Delete error = %v, want a *NotMountedError", err)
	}

	if notMounted.Device != "DUB0" {
		t.Errorf("NotMountedError.Device = %q, want DUB0", notMounted.Device)
	}
}

// TestSession_deleteBadFileSpec confirms a malformed file specification is
// reported as a plain error, and specifically none of NotMountedError/
// VersionRequiredError/NotFoundError -- the console layer's Console.Delete
// relies on being able to tell these apart.
func TestSession_deleteBadFileSpec(t *testing.T) {
	s, _ := newDeleteTestSession(t)

	_, err := s.Delete("DUA0:[UNTERMINATED")
	if err == nil {
		t.Fatal("Delete with an unterminated directory bracket = nil error, want an error")
	}

	var notMounted *NotMountedError
	if errors.As(err, &notMounted) {
		t.Errorf("Delete error = %v, want a plain error, not a *NotMountedError", err)
	}

	var versionRequired *VersionRequiredError
	if errors.As(err, &versionRequired) {
		t.Errorf("Delete error = %v, want a plain error, not a *VersionRequiredError", err)
	}

	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		t.Errorf("Delete error = %v, want a plain error, not a *NotFoundError", err)
	}
}
