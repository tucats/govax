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
// one "Directory DUA0:[]:" header (the master file directory — an empty
// joined directory path, exactly matching ods2's own cmdDirectory, which
// this phase's own "behavioral reference, not necessarily byte-identical"
// framing means this port deliberately reproduces rather than "improves").
//
// The volume's own reserved files (INDEXF.SYS, BITMAP.SYS, ...) are already
// present in the master file directory from volume.Initialize, so this
// doesn't assert an exact file count — TestSession_directoryWildcardFilter
// covers that, scoped to a name pattern the reserved files can't match.
func TestSession_directoryListsFiles(t *testing.T) {
	s, vol := newDirectoryTestSession(t)
	createTestFile(t, vol, "FOO.TXT", "hello")
	createTestFile(t, vol, "BAR.DAT", "world")

	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, err := s.Directory("", DirectoryOptions{})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if !strings.Contains(out, "Directory DUA0:[]") {
		t.Errorf("Directory output = %q, want it to contain a DUA0:[] header", out)
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

	out, err := s.Directory("*.TXT", DirectoryOptions{})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if !strings.Contains(out, "FOO.TXT;1") {
		t.Errorf("Directory(*.TXT) output = %q, want it to contain FOO.TXT;1", out)
	}

	if strings.Contains(out, "BAR.DAT") {
		t.Errorf("Directory(*.TXT) output = %q, want it to NOT contain BAR.DAT", out)
	}

	if !strings.Contains(out, "Total of 1 file(s).") {
		t.Errorf("Directory(*.TXT) output = %q, want a 1-file total", out)
	}
}

// TestSession_directoryEmptyIsNotAnError confirms a DIRECTORY that matches
// nothing at all (a name/type pattern nothing on the volume — including its
// own reserved files — can match) is a normal, zero-length listing rather
// than an error, matching real VMS, which prints "Total of 0 file(s)."
// rather than failing.
func TestSession_directoryEmptyIsNotAnError(t *testing.T) {
	s, _ := newDirectoryTestSession(t)

	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, err := s.Directory("*.NOSUCHTYPE", DirectoryOptions{})
	if err != nil {
		t.Fatalf("Directory matching nothing: %v", err)
	}

	if !strings.Contains(out, "Total of 0 file(s).") {
		t.Errorf("Directory output = %q, want a 0-file total", out)
	}
}

// TestSession_directoryFullQualifier confirms /FULL turns on all three of
// FILE/SIZE/DATE at once: the file-id, block count, and a revision date all
// appear in the same line, plus the record format ("STREAMLF", matching
// createTestFile's own RecordFormatStreamLF) that only /FULL itself adds.
func TestSession_directoryFullQualifier(t *testing.T) {
	s, vol := newDirectoryTestSession(t)
	createTestFile(t, vol, "FOO.TXT", "hello")

	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	out, err := s.Directory("FOO.TXT", DirectoryOptions{Full: true})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if !strings.Contains(out, "STREAMLF") {
		t.Errorf("Directory/FULL output = %q, want it to contain the STREAMLF record format", out)
	}

	if !strings.Contains(out, "Total of 1 file(s), 1 block(s).") {
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

	out, err := s.Directory("FOO.TXT", DirectoryOptions{Size: true})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	if !strings.Contains(out, "Total of 1 file(s), 1 block(s).") {
		t.Errorf("Directory/SIZE output = %q, want a 1-file, 1-block total", out)
	}

	if strings.Contains(out, "STREAMLF") {
		t.Errorf("Directory/SIZE output = %q, want it to NOT contain the record format (that's /FULL-only)", out)
	}
}

// TestSession_directoryNotMounted confirms specText naming a device with
// nothing mounted on it surfaces as an *rms.NotMountedError (via errors.As,
// the console layer's own SS_DEVNOTMOUNT translation depends on this),
// carrying the specific device name that was resolved.
func TestSession_directoryNotMounted(t *testing.T) {
	s := NewSession(NewMountTable())

	_, err := s.Directory("DUB0:*.*;*", DirectoryOptions{})
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

	_, err := s.Directory("DUA0:[UNTERMINATED", DirectoryOptions{})
	if err == nil {
		t.Fatal("Directory with an unterminated directory bracket = nil error, want an error")
	}

	var notMounted *NotMountedError
	if errors.As(err, &notMounted) {
		t.Errorf("Directory error = %v, want a plain error, not a *NotMountedError", err)
	}
}
