package rms

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file's tests build their own small mounted volume with a couple of
// real files on it (createTestFile, below) rather than reusing any of
// ods2's own internal test fixtures (which live in an internal/ package
// this module can't import anyway) — the same "call the real public API
// directly" approach internal/rms/create.go's own createOnVolume already
// takes for SYS$CREATE, just invoked straight from a test instead of
// through a whole simulated VAX FAB/RAB.

// newDirectoryTestSession builds a fresh, writable, mounted test volume
// (newTestVolumeFile, from mount_test.go) on device DUA0, and returns both
// a *Session with it already mounted and the *volume.Volume itself, so a
// test can populate it with files via createTestFile before exercising
// Session.Directory.
func newDirectoryTestSession(t *testing.T) (*Session, *volume.Volume) {
	t.Helper()

	mounts := NewMountTable()
	path := newTestVolumeFile(t, "DIRVOL")

	if err := mounts.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	vol, ok := mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after Mount = not found")
	}

	return NewSession(mounts), vol
}

// createTestFile creates name (e.g. "FOO.TXT") in vol's master file
// directory with content as its data — the same Directory.Insert/
// Volume.CreateFile calls internal/rms/create.go's own createOnVolume
// makes on behalf of a real SYS$CREATE, called directly here since these
// tests only need a file to exist for DIRECTORY to list, not a whole
// running VAX program to create it through.
func createTestFile(t *testing.T, vol *volume.Volume, name, content string) {
	t.Helper()

	dir, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	bm, err := dir.Device.Bitmap()
	if err != nil {
		t.Fatalf("Bitmap: %v", err)
	}

	ib, err := dir.Device.IndexBitmap()
	if err != nil {
		t.Fatalf("IndexBitmap: %v", err)
	}

	f, err := vol.CreateFile(dir, name, ondisk.RecAttr{
		Format:        ondisk.RecordFormatStreamLF,
		MaxRecordSize: 512,
	}, bm, ib)
	if err != nil {
		t.Fatalf("CreateFile(%s): %v", name, err)
	}

	block := make([]byte, ondisk.BlockSize)
	copy(block, content)

	if err := f.WriteBlock(1, block); err != nil {
		t.Fatalf("WriteBlock(%s): %v", name, err)
	}

	if err := f.CloseWithFinalByte(uint16(len(content))); err != nil {
		t.Fatalf("Close(%s): %v", name, err)
	}
}

// TestSession_directoryListsFiles confirms the basic happy path: every file
// created on the volume shows up in a bare DIRECTORY listing, grouped under
// one "Directory DUA0:[000000]" header (the master file directory, which
// VMS writes [000000]; govax wrote "[]" until docs/PHASE-34.md).
func TestSession_directoryListsFiles(t *testing.T) {
	s, vol := newDirectoryTestSession(t)
	createTestFile(t, vol, "FOO.TXT", "hello")
	createTestFile(t, vol, "BAR.DAT", "world")

	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, _, err := s.Directory("", DirectoryOptions{})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if !strings.Contains(out, "Directory DUA0:[000000]") {
		t.Errorf("Directory output = %q, want it to contain a DUA0:[000000] header", out)
	}

	if !strings.Contains(out, "FOO.TXT;1") {
		t.Errorf("Directory output = %q, want it to contain FOO.TXT;1", out)
	}

	if !strings.Contains(out, "BAR.DAT;1") {
		t.Errorf("Directory output = %q, want it to contain BAR.DAT;1", out)
	}
}

// TestSession_directoryWildcardFilter confirms an explicit, non-default
// name/type pattern actually narrows the listing, rather than Directory
// silently ignoring specText's own name/type fields and always listing
// everything.
func TestSession_directoryWildcardFilter(t *testing.T) {
	s, vol := newDirectoryTestSession(t)
	createTestFile(t, vol, "FOO.TXT", "hello")
	createTestFile(t, vol, "BAR.DAT", "world")

	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, _, err := s.Directory("*.TXT", DirectoryOptions{})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if !strings.Contains(out, "FOO.TXT;1") {
		t.Errorf("Directory(*.TXT) output = %q, want it to contain FOO.TXT;1", out)
	}

	if strings.Contains(out, "BAR.DAT") {
		t.Errorf("Directory(*.TXT) output = %q, want it to NOT contain BAR.DAT", out)
	}

	if want := "\nDirectory DUA0:[000000]\n\nFOO.TXT;1           \n\nTotal of 1 file.\n"; out != want {
		t.Errorf("Directory(*.TXT) output = %q, want %q", out, want)
	}
}

// TestSession_directoryEmptyIsNotAnError confirms a DIRECTORY that matches
// nothing at all (a name/type pattern nothing on the volume — including its
// own reserved files — can match) lists nothing and reports found false,
// for the caller's NOFILES warning, rather than an error.
func TestSession_directoryEmptyIsNotAnError(t *testing.T) {
	s, _ := newDirectoryTestSession(t)

	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, found, err := s.Directory("*.NOSUCHTYPE", DirectoryOptions{})
	if err != nil {
		t.Fatalf("Directory matching nothing: %v", err)
	}

	if out != "" || found {
		t.Errorf("Directory output = %q, found %v; want nothing", out, found)
	}
}

// TestSession_directoryFullQualifier confirms /FULL's layout, VMS 7.3's
// (testdata/dcl50's round 4): the file ID, used/allocated size, owner,
// dates, and the record format ("Stream_LF", matching createTestFile's
// own RecordFormatStreamLF), then "Total of 1 file, 1/1 block."
func TestSession_directoryFullQualifier(t *testing.T) {
	s, vol := newDirectoryTestSession(t)
	createTestFile(t, vol, "FOO.TXT", "hello")

	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, _, err := s.Directory("FOO.TXT", DirectoryOptions{Full: true})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	for _, want := range []string{
		"\nFOO.TXT;1                     File ID:  (",
		"\nSize:            1/1          Owner:    [1,4]\n",
		"\nRecord format:      Stream_LF, maximum ",
		"\nFile protection:    System:",
		"\nAccess Cntrl List:  None\n\nTotal of 1 file, 1/1 block.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Directory/FULL output = %q, want it to contain %q", out, want)
		}
	}

	if !strings.Contains(out, "Total of 1 file, 1/1 block.") {
		t.Errorf("Directory/FULL output = %q, want a 1-file, 1-block total (SIZE is implied by FULL)", out)
	}

	// A file-id looks like "(N,1,1)" (ondisk.Fid.String) -- just confirm
	// some parenthesized id appears, rather than pinning down FOO.TXT's
	// exact, allocation-order-dependent file number.
	if !strings.Contains(out, "(") {
		t.Errorf("Directory/FULL output = %q, want it to contain a file id (FILE is implied by FULL)", out)
	}
}

// TestSession_directorySizeQualifier confirms /SIZE alone (without /FULL)
// adds the per-file and total block counts but not the file-id/record
// format /FULL would also include.
func TestSession_directorySizeQualifier(t *testing.T) {
	s, vol := newDirectoryTestSession(t)
	createTestFile(t, vol, "FOO.TXT", "hello")

	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, _, err := s.Directory("FOO.TXT", DirectoryOptions{Size: true})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if want := "\nDirectory DUA0:[000000]\n\nFOO.TXT;1                  1\n\nTotal of 1 file, 1 block.\n"; out != want {
		t.Errorf("Directory/SIZE output = %q, want %q", out, want)
	}

	out, _, err = s.Directory("FOO.TXT", DirectoryOptions{Size: true, SizeMode: SizeAll})
	if err != nil {
		t.Fatal(err)
	}

	if want := "\nDirectory DUA0:[000000]\n\nFOO.TXT;1                  1/1      \n\nTotal of 1 file, 1/1 block.\n"; out != want {
		t.Errorf("Directory/SIZE=ALL output = %q, want %q", out, want)
	}

	if strings.Contains(out, "Stream_LF") {
		t.Errorf("Directory/SIZE output = %q, want it to NOT contain the record format (that's /FULL-only)", out)
	}
}

// TestSession_directoryNotMounted confirms specText naming a device with
// nothing mounted on it surfaces as an *rms.NotMountedError (via errors.As,
// the console layer's own SS_DEVNOTMOUNT translation depends on this),
// carrying the specific device name that was resolved.
func TestSession_directoryNotMounted(t *testing.T) {
	s := NewSession(NewMountTable())

	_, _, err := s.Directory("DUB0:*.*;*", DirectoryOptions{})
	if err == nil {
		t.Fatal("Directory against an unmounted device = nil error, want an error")
	}

	var notMounted *NotMountedError
	if !errors.As(err, &notMounted) {
		t.Fatalf("Directory error = %v, want a *NotMountedError", err)
	}

	if notMounted.Device != dub0TestDevice {
		t.Errorf("NotMountedError.Device = %q, want DUB0", notMounted.Device)
	}
}

// TestSession_directoryBadFileSpec confirms a malformed file specification
// is reported as a plain error, and specifically NOT as a *NotMountedError
// -- the console layer's Console.Directory relies on being able to tell
// these two failure modes apart (CLI_BADFILESPEC vs. SS_DEVNOTMOUNT).
func TestSession_directoryBadFileSpec(t *testing.T) {
	s, _ := newDirectoryTestSession(t)

	_, _, err := s.Directory("DUA0:[UNTERMINATED", DirectoryOptions{})
	if err == nil {
		t.Fatal("Directory with an unterminated directory bracket = nil error, want an error")
	}

	var notMounted *NotMountedError
	if errors.As(err, &notMounted) {
		t.Errorf("Directory error = %v, want a plain error, not a *NotMountedError", err)
	}
}

// DIRECTORY/FULL's file attributes line ends with the characteristics VMS
// 7.1 names (testdata/pg64's page file, testdata/backup69's tree), and
// nothing for the others.
func TestCharacteristicsText(t *testing.T) {
	for _, c := range []struct {
		fch  uint32
		want string
	}{
		{0, ""},
		{ondisk.FchNoBackup, ", Backups disabled"},
		{ondisk.FchNoBackup | fchNoMove, ", Backups disabled, MoveFile disabled"},
		{ondisk.FchContigB, ", Contiguous best try"},
		{ondisk.FchContig | ondisk.FchDirectory, ", Contiguous, Directory file"},
		{ondisk.FchLocked, ""},
	} {
		if got := characteristicsText(c.fch); got != c.want {
			t.Errorf("characteristicsText(%#x) = %q, want %q", c.fch, got, c.want)
		}
	}
}

// A directory's version limit is its files' default, 32767 when it has
// none, as VMS 7.1's DIRECTORY/FULL shows it (testdata/backup69's tree).
func TestVersionLimitText(t *testing.T) {
	for _, c := range []struct {
		attr FileAttributes
		want string
	}{
		{FileAttributes{}, "No version limit"},
		{FileAttributes{VersionLimit: 3}, "Version limit: 3"},
		{FileAttributes{Characteristics: ondisk.FchDirectory}, "Default version limit: 32767"},
		{FileAttributes{Characteristics: ondisk.FchDirectory, VersionLimit: 5}, "Default version limit: 5"},
	} {
		if got := versionLimitText(c.attr); got != c.want {
			t.Errorf("versionLimitText(%+v) = %q, want %q", c.attr, got, c.want)
		}
	}
}
