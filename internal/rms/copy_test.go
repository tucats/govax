package rms

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ods2/volume"
)

// newCopyTestSession builds a fresh, writable, mounted test volume (device
// DUA0, reusing newTestVolumeFile from mount_test.go, with the operator's
// current default already pointed at it via SetDefault) pre-populated with
// FOO.TXT (a two-line Stream_LF file, createTestFile from directory_test.go)
// and a two-version DUP.TXT;1/;2 -- the same shape newTypeTestSession
// (type_test.go) uses for its own "no version selects the highest" and
// "wildcarded versions of one name" cases, reused here for COPY's own
// single-match-only restriction.
func newCopyTestSession(t *testing.T) (*Session, *volume.Volume) {
	t.Helper()

	mounts := NewMountTable()
	path := newTestVolumeFile(t, "COPYVOL")

	if err := mounts.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	vol, ok := mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after Mount = not found")
	}

	createTestFile(t, vol, "FOO.TXT", "line one\nline two\n")
	createTestFile(t, vol, "DUP.TXT", "version one")
	createTestFile(t, vol, "DUP.TXT", "version two")

	s := NewSession(mounts)
	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	return s, vol
}

// TestSessionCopy_volumeToVolume confirms the plain "COPY FOO.TXT BAR.TXT"
// case (neither /HOST set): FOO.TXT's content is reframed onto a
// brand-new BAR.TXT;1 on the same mounted volume, readable back through
// the already-tested Session.Type exactly as written.
func TestSessionCopy_volumeToVolume(t *testing.T) {
	s, _ := newCopyTestSession(t)

	result, err := s.Copy("FOO.TXT", false, "BAR.TXT", false)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if result.Source != "FOO.TXT;1" {
		t.Errorf("Source = %q, want %q", result.Source, "FOO.TXT;1")
	}

	if result.Dest != "DUA0:[000000]BAR.TXT;1" {
		t.Errorf("Dest = %q, want %q", result.Dest, "DUA0:[000000]BAR.TXT;1")
	}

	text, err := s.Type("BAR.TXT")
	if err != nil {
		t.Fatalf("Type(BAR.TXT): %v", err)
	}

	if text != "line one\nline two\n" {
		t.Errorf("copied content = %q, want %q", text, "line one\nline two\n")
	}
}

// TestSessionCopy_volumeToVolumeInheritsSourceName confirms that a
// destination spec naming no file of its own (just a device/directory --
// "DUA0:", which resolves to no Name/Type since SetDefault("DUA0:") never
// set one either) falls back to the source file's own name/type
// (destNameType), rather than failing or creating some empty-named file.
// Copying FOO.TXT onto its own directory this way lands as a new version
// of FOO.TXT itself (;2), which doubles as a check that CreateFile's own
// version auto-assignment is honored correctly.
func TestSessionCopy_volumeToVolumeInheritsSourceName(t *testing.T) {
	s, _ := newCopyTestSession(t)

	result, err := s.Copy("FOO.TXT", false, "DUA0:", false)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if result.Dest != "DUA0:[000000]FOO.TXT;2" {
		t.Errorf("Dest = %q, want %q", result.Dest, "DUA0:[000000]FOO.TXT;2")
	}

	text, err := s.Type("FOO.TXT;2")
	if err != nil {
		t.Fatalf("Type(FOO.TXT;2): %v", err)
	}

	if text != "line one\nline two\n" {
		t.Errorf("copied content = %q, want %q", text, "line one\nline two\n")
	}
}

// TestSessionCopy_fromHost confirms COPY's /HOST-on-SOURCE direction
// ("COPY foo.txt/HOST BAR.TXT"): a plain host file's text content is
// reframed onto a brand-new file on the mounted volume, under the
// destination's own explicitly given name.
func TestSessionCopy_fromHost(t *testing.T) {
	s, _ := newCopyTestSession(t)

	hostPath := filepath.Join(t.TempDir(), "host.txt")
	if err := os.WriteFile(hostPath, []byte("line a\nline b\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := s.Copy(hostPath, true, "BAZ.TXT", false)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if result.Source != hostPath {
		t.Errorf("Source = %q, want %q", result.Source, hostPath)
	}

	if result.Dest != "DUA0:[000000]BAZ.TXT;1" {
		t.Errorf("Dest = %q, want %q", result.Dest, "DUA0:[000000]BAZ.TXT;1")
	}

	text, err := s.Type("BAZ.TXT")
	if err != nil {
		t.Fatalf("Type(BAZ.TXT): %v", err)
	}

	if text != "line a\nline b\n" {
		t.Errorf("copied content = %q, want %q", text, "line a\nline b\n")
	}
}

// TestSessionCopy_fromHostInheritsHostBaseName confirms that a
// destination naming no file of its own derives the created file's
// name/type from the host source's own base name (hostBaseNameType),
// upper-cased, splitting on the LAST '.'.
func TestSessionCopy_fromHostInheritsHostBaseName(t *testing.T) {
	s, _ := newCopyTestSession(t)

	hostPath := filepath.Join(t.TempDir(), "myfile.dat")
	if err := os.WriteFile(hostPath, []byte("payload\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := s.Copy(hostPath, true, "DUA0:", false)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if result.Dest != "DUA0:[000000]MYFILE.DAT;1" {
		t.Errorf("Dest = %q, want %q", result.Dest, "DUA0:[000000]MYFILE.DAT;1")
	}
}

// TestSessionCopy_toHost confirms COPY's /HOST-on-DESTINATION direction
// ("COPY FOO.TXT bar.txt/HOST"): the volume source's content is rendered
// as text onto a literal host output path.
func TestSessionCopy_toHost(t *testing.T) {
	s, _ := newCopyTestSession(t)

	outPath := filepath.Join(t.TempDir(), "out.txt")

	result, err := s.Copy("FOO.TXT", false, outPath, true)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if result.Source != "FOO.TXT;1" {
		t.Errorf("Source = %q, want %q", result.Source, "FOO.TXT;1")
	}

	if result.Dest != outPath {
		t.Errorf("Dest = %q, want %q", result.Dest, outPath)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != "line one\nline two\n" {
		t.Errorf("copied content = %q, want %q", got, "line one\nline two\n")
	}
}

// TestSessionCopy_toHostExistingDirectory confirms that a destination
// naming an existing host directory (rather than a literal file path)
// writes the file there under its own "NAME.TYPE;version" -- matching
// ods2's own resolveDestination directory case.
func TestSessionCopy_toHostExistingDirectory(t *testing.T) {
	s, _ := newCopyTestSession(t)

	destDir := t.TempDir()

	result, err := s.Copy("FOO.TXT", false, destDir, true)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	wantPath := filepath.Join(destDir, "FOO.TXT;1")
	if result.Dest != wantPath {
		t.Errorf("Dest = %q, want %q", result.Dest, wantPath)
	}

	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("expected %s to exist: %v", wantPath, err)
	}
}

// TestSessionCopy_hostToHostRejected confirms that /HOST on both SOURCE
// and DESTINATION at once -- no container endpoint at all -- fails with a
// *HostToHostError rather than falling back to some host-to-host copy
// behavior COPY was never meant to provide.
func TestSessionCopy_hostToHostRejected(t *testing.T) {
	s, _ := newCopyTestSession(t)

	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	dst := filepath.Join(dir, "b.txt")

	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := s.Copy(src, true, dst, true)
	if err == nil {
		t.Fatal("Copy with /HOST on both sides = nil error, want *HostToHostError")
	}

	var hostToHost *HostToHostError
	if !errors.As(err, &hostToHost) {
		t.Errorf("Copy error = %v, want *HostToHostError", err)
	}
}

// TestSessionCopy_sourceNotFound confirms a source spec matching nothing
// on its volume fails with a *NotFoundError -- the same failure
// Session.Delete/Session.Type already report this way.
func TestSessionCopy_sourceNotFound(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy("NOSUCH.TXT", false, "BAR.TXT", false)
	if err == nil {
		t.Fatal("Copy of a nonexistent file = nil error, want *NotFoundError")
	}

	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("Copy error = %v, want *NotFoundError", err)
	}
}

// TestSessionCopy_sourceAmbiguous confirms a wildcarded source spec
// matching more than one file fails with a *MultipleMatchesError -- COPY's
// own "no multi-file copying yet" restriction (see copy.go's top-of-file
// doc comment), distinct from (but worded like) Session.Type's own
// AmbiguousError.
func TestSessionCopy_sourceAmbiguous(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy("DUP.TXT;*", false, "OUT.TXT", false)
	if err == nil {
		t.Fatal("Copy of a wildcarded multi-version spec = nil error, want *MultipleMatchesError")
	}

	var multiple *MultipleMatchesError
	if !errors.As(err, &multiple) {
		t.Fatalf("Copy error = %v, want *MultipleMatchesError", err)
	}

	if multiple.Count != 2 {
		t.Errorf("MultipleMatchesError.Count = %d, want 2", multiple.Count)
	}
}

// TestSessionCopy_hostSourceMissing confirms a /HOST source path that
// doesn't exist on the host filesystem at all fails with a plain error
// (surfaced by os.Stat), not silently treated as "not found on the
// volume" (*NotFoundError is a volume-side concept and would be
// misleading here).
func TestSessionCopy_hostSourceMissing(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy(filepath.Join(t.TempDir(), "nope.txt"), true, "BAR.TXT", false)
	if err == nil {
		t.Fatal("Copy of a missing host file = nil error")
	}
}

// TestSessionCopy_hostSourceIsDirectory confirms a /HOST source naming an
// existing host directory (rather than a file) is rejected outright.
func TestSessionCopy_hostSourceIsDirectory(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy(t.TempDir(), true, "BAR.TXT", false)
	if err == nil {
		t.Fatal("Copy of a host directory = nil error, want an error")
	}
}

// TestSessionCopy_sourceNotMounted confirms a source spec naming a device
// with nothing mounted on it surfaces as *NotMountedError, matching every
// other command in this phase's own convention for the same condition.
func TestSessionCopy_sourceNotMounted(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy("DUB0:FOO.TXT", false, "BAR.TXT", false)
	if err == nil {
		t.Fatal("Copy from an unmounted device = nil error, want *NotMountedError")
	}

	var notMounted *NotMountedError
	if !errors.As(err, &notMounted) {
		t.Fatalf("Copy error = %v, want *NotMountedError", err)
	}

	if notMounted.Device != "DUB0" {
		t.Errorf("NotMountedError.Device = %q, want %q", notMounted.Device, "DUB0")
	}
}

// TestSessionCopy_destNotMounted mirrors TestSessionCopy_sourceNotMounted
// for the destination side of a volume-to-volume copy.
func TestSessionCopy_destNotMounted(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy("FOO.TXT", false, "DUB0:BAR.TXT", false)
	if err == nil {
		t.Fatal("Copy to an unmounted device = nil error, want *NotMountedError")
	}

	var notMounted *NotMountedError
	if !errors.As(err, &notMounted) {
		t.Fatalf("Copy error = %v, want *NotMountedError", err)
	}

	if notMounted.Device != "DUB0" {
		t.Errorf("NotMountedError.Device = %q, want %q", notMounted.Device, "DUB0")
	}
}
