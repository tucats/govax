package rms

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// newCopyTestSession builds a fresh, writable, mounted test volume (device
// DUA0, reusing newTestVolumeFile from mount_test.go, with the operator's
// current default already pointed at it via SetDefault) pre-populated with
// FOO.TXT (a two-line Stream_LF file, createTestFile from directory_test.go)
// and a two-version DUP.TXT;1/;2 -- the same shape newTypeTestSession
// (type_test.go) uses for its own "no version selects the highest" and
// "wildcarded versions of one name" cases, reused here for COPY's own
// single-match-only-without-a-directory-destination restriction.
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

// createFormattedTestFile is createTestFile (directory_test.go) generalized
// to an arbitrary record format -- needed by this file's own /STREAM and
// /IGNORE tests, which specifically need a Stream_CR or Stream_CRLF file
// (createTestFile always creates Stream_LF) to exercise format-specific
// behavior.
func createFormattedTestFile(t *testing.T, vol *volume.Volume, name, content string, format ondisk.RecordFormat) {
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
		Format:        format,
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

// TestSessionCopy_volumeToVolume confirms the plain "COPY FOO.TXT BAR.TXT"
// case (neither /HOST set, no qualifiers): FOO.TXT's content is reframed
// onto a brand-new BAR.TXT;1 on the same mounted volume, readable back
// through the already-tested Session.Type exactly as written.
func TestSessionCopy_volumeToVolume(t *testing.T) {
	s, _ := newCopyTestSession(t)

	results, err := s.Copy("FOO.TXT", false, "BAR.TXT", false, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Copy returned %d results, want 1", len(results))
	}

	r := results[0]
	if r.Kind != CopyCopied {
		t.Errorf("Kind = %v, want CopyCopied", r.Kind)
	}

	if r.Source != "FOO.TXT;1" {
		t.Errorf("Source = %q, want %q", r.Source, "FOO.TXT;1")
	}

	if r.Dest != "DUA0:[000000]BAR.TXT;1" {
		t.Errorf("Dest = %q, want %q", r.Dest, "DUA0:[000000]BAR.TXT;1")
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
func TestSessionCopy_volumeToVolumeInheritsSourceName(t *testing.T) {
	s, _ := newCopyTestSession(t)

	results, err := s.Copy("FOO.TXT", false, "DUA0:", false, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if results[0].Dest != "DUA0:[000000]FOO.TXT;2" {
		t.Errorf("Dest = %q, want %q", results[0].Dest, "DUA0:[000000]FOO.TXT;2")
	}

	text, err := s.Type("FOO.TXT;2")
	if err != nil {
		t.Fatalf("Type(FOO.TXT;2): %v", err)
	}

	if text != "line one\nline two\n" {
		t.Errorf("copied content = %q, want %q", text, "line one\nline two\n")
	}
}

// TestSessionCopy_wildcardVolumeToVolumeDirectory is subtask 10's own
// headline feature: a wildcarded SOURCE ("*.TXT", matching FOO.TXT and
// DUP.TXT -- each name's own highest surviving version only, since an
// unspecified version selector defaults to "highest", the same default
// Session.Type/Session.Directory already rely on -- filespec.Glob never
// expands to "every version" absent an explicit ";*") copied onto a
// destination naming no file of its own (just "DUA0:", a directory) copies
// every match in under its own name, rather than failing the way a single,
// specific destination name would.
func TestSessionCopy_wildcardVolumeToVolumeDirectory(t *testing.T) {
	s, _ := newCopyTestSession(t)

	results, err := s.Copy("*.TXT", false, "DUA0:", false, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	// FOO.TXT;1, DUP.TXT;2 -- two matches for "*.TXT" (DUP.TXT;1 is not
	// the highest version of DUP.TXT, so the default version selector
	// excludes it).
	if len(results) != 2 {
		t.Fatalf("Copy returned %d results, want 2: %+v", len(results), results)
	}

	for _, r := range results {
		if r.Kind != CopyCopied {
			t.Errorf("result %+v: Kind = %v, want CopyCopied", r, r.Kind)
		}
	}

	// Every copy lands as a new version of its own name (FOO.TXT;2,
	// DUP.TXT;3) since "DUA0:" is the same directory the sources already
	// live in.
	if _, err := s.Type("FOO.TXT;2"); err != nil {
		t.Errorf("Type(FOO.TXT;2) after wildcard copy: %v", err)
	}

	if _, err := s.Type("DUP.TXT;3"); err != nil {
		t.Errorf("Type(DUP.TXT;3) after wildcard copy: %v", err)
	}
}

// TestSessionCopy_wildcardRejectedForNonDirectoryVolumeDestination
// confirms a wildcarded source is still rejected when the volume
// destination names a specific file of its own, not just a directory --
// subtask 10 only lifts the restriction for a directory destination.
func TestSessionCopy_wildcardRejectedForNonDirectoryVolumeDestination(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy("*.TXT", false, "OUT.TXT", false, CopyOptions{})
	if err == nil {
		t.Fatal("wildcarded Copy onto a named destination file = nil error, want *MultipleMatchesError")
	}

	var multiple *MultipleMatchesError
	if !errors.As(err, &multiple) {
		t.Fatalf("Copy error = %v, want *MultipleMatchesError", err)
	}

	if multiple.Count != 2 {
		t.Errorf("MultipleMatchesError.Count = %d, want 2", multiple.Count)
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

	results, err := s.Copy(hostPath, true, "BAZ.TXT", false, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if results[0].Source != hostPath {
		t.Errorf("Source = %q, want %q", results[0].Source, hostPath)
	}

	if results[0].Dest != "DUA0:[000000]BAZ.TXT;1" {
		t.Errorf("Dest = %q, want %q", results[0].Dest, "DUA0:[000000]BAZ.TXT;1")
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

	results, err := s.Copy(hostPath, true, "DUA0:", false, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if results[0].Dest != "DUA0:[000000]MYFILE.DAT;1" {
		t.Errorf("Dest = %q, want %q", results[0].Dest, "DUA0:[000000]MYFILE.DAT;1")
	}
}

// TestSessionCopy_toHost confirms COPY's /HOST-on-DESTINATION direction
// ("COPY FOO.TXT bar.txt/HOST"): the volume source's content is rendered
// as text onto a literal host output path.
func TestSessionCopy_toHost(t *testing.T) {
	s, _ := newCopyTestSession(t)

	outPath := filepath.Join(t.TempDir(), "out.txt")

	results, err := s.Copy("FOO.TXT", false, outPath, true, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if results[0].Source != "FOO.TXT;1" {
		t.Errorf("Source = %q, want %q", results[0].Source, "FOO.TXT;1")
	}

	if results[0].Dest != outPath {
		t.Errorf("Dest = %q, want %q", results[0].Dest, outPath)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != "line one\nline two\n" {
		t.Errorf("copied content = %q, want %q", got, "line one\nline two\n")
	}
}

// TestSessionCopy_wildcardToHostDirectory mirrors
// TestSessionCopy_wildcardVolumeToVolumeDirectory for a host destination:
// a wildcarded source copied onto an existing host directory lands every
// match there under its own "NAME.TYPE;version".
func TestSessionCopy_wildcardToHostDirectory(t *testing.T) {
	s, _ := newCopyTestSession(t)

	destDir := t.TempDir()

	results, err := s.Copy("*.TXT", false, destDir, true, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Copy returned %d results, want 2: %+v", len(results), results)
	}

	for _, want := range []string{"FOO.TXT;1", "DUP.TXT;2"} {
		if _, err := os.Stat(filepath.Join(destDir, want)); err != nil {
			t.Errorf("expected %s to exist in %s: %v", want, destDir, err)
		}
	}
}

// TestSessionCopy_wildcardRejectedForNonDirectoryHostDestination mirrors
// TestSessionCopy_wildcardRejectedForNonDirectoryVolumeDestination for a
// host destination that isn't an existing directory.
func TestSessionCopy_wildcardRejectedForNonDirectoryHostDestination(t *testing.T) {
	s, _ := newCopyTestSession(t)

	outPath := filepath.Join(t.TempDir(), "out.txt")

	_, err := s.Copy("*.TXT", false, outPath, true, CopyOptions{})
	if err == nil {
		t.Fatal("wildcarded Copy onto a literal host path = nil error, want *MultipleMatchesError")
	}

	var multiple *MultipleMatchesError
	if !errors.As(err, &multiple) {
		t.Errorf("Copy error = %v, want *MultipleMatchesError", err)
	}
}

// TestSessionCopy_toHostExistingDirectory confirms that a single-match
// source onto an existing host directory (rather than a literal file
// path) writes the file there under its own "NAME.TYPE;version" --
// matching ods2's own resolveDestination directory case.
func TestSessionCopy_toHostExistingDirectory(t *testing.T) {
	s, _ := newCopyTestSession(t)

	destDir := t.TempDir()

	results, err := s.Copy("FOO.TXT", false, destDir, true, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	wantPath := filepath.Join(destDir, "FOO.TXT;1")
	if results[0].Dest != wantPath {
		t.Errorf("Dest = %q, want %q", results[0].Dest, wantPath)
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

	_, err := s.Copy(src, true, dst, true, CopyOptions{})
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

	_, err := s.Copy("NOSUCH.TXT", false, "BAR.TXT", false, CopyOptions{})
	if err == nil {
		t.Fatal("Copy of a nonexistent file = nil error, want *NotFoundError")
	}

	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("Copy error = %v, want *NotFoundError", err)
	}
}

// TestSessionCopy_hostSourceMissing confirms a /HOST source path that
// doesn't exist on the host filesystem at all fails with a plain error
// (surfaced by os.Stat), not silently treated as "not found on the
// volume" (*NotFoundError is a volume-side concept and would be
// misleading here).
func TestSessionCopy_hostSourceMissing(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy(filepath.Join(t.TempDir(), "nope.txt"), true, "BAR.TXT", false, CopyOptions{})
	if err == nil {
		t.Fatal("Copy of a missing host file = nil error")
	}
}

// TestSessionCopy_hostSourceIsDirectory confirms a /HOST source naming an
// existing host directory (rather than a file) is rejected outright.
func TestSessionCopy_hostSourceIsDirectory(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy(t.TempDir(), true, "BAR.TXT", false, CopyOptions{})
	if err == nil {
		t.Fatal("Copy of a host directory = nil error, want an error")
	}
}

// TestSessionCopy_sourceNotMounted confirms a source spec naming a device
// with nothing mounted on it surfaces as *NotMountedError, matching every
// other command in this phase's own convention for the same condition.
func TestSessionCopy_sourceNotMounted(t *testing.T) {
	s, _ := newCopyTestSession(t)

	_, err := s.Copy("DUB0:FOO.TXT", false, "BAR.TXT", false, CopyOptions{})
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

	_, err := s.Copy("FOO.TXT", false, "DUB0:BAR.TXT", false, CopyOptions{})
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

// TestSessionCopy_test confirms /TEST previews a copy (a single
// CopyTested result, with no version yet assigned to the volume-side
// destination display) without actually creating anything.
func TestSessionCopy_test(t *testing.T) {
	s, _ := newCopyTestSession(t)

	results, err := s.Copy("FOO.TXT", false, "BAR.TXT", false, CopyOptions{Test: true})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if len(results) != 1 || results[0].Kind != CopyTested {
		t.Fatalf("Copy results = %+v, want a single CopyTested result", results)
	}

	if results[0].Dest != "DUA0:[000000]BAR.TXT" {
		t.Errorf("Dest = %q, want %q (no version yet under /TEST)", results[0].Dest, "DUA0:[000000]BAR.TXT")
	}

	if _, err := s.Type("BAR.TXT"); err == nil {
		t.Error("BAR.TXT exists after a /TEST copy -- /TEST must not actually write anything")
	}
}

// TestSessionCopy_testToHost mirrors TestSessionCopy_test for a host
// destination: the previewed path includes the version (known directly
// from the matched file, unlike the volume-destination case), and no
// host file is actually created.
func TestSessionCopy_testToHost(t *testing.T) {
	s, _ := newCopyTestSession(t)

	outPath := filepath.Join(t.TempDir(), "out.txt")

	results, err := s.Copy("FOO.TXT", false, outPath, true, CopyOptions{Test: true})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if len(results) != 1 || results[0].Kind != CopyTested {
		t.Fatalf("Copy results = %+v, want a single CopyTested result", results)
	}

	if results[0].Dest != outPath {
		t.Errorf("Dest = %q, want %q", results[0].Dest, outPath)
	}

	if _, err := os.Stat(outPath); err == nil {
		t.Error("output file exists after a /TEST copy -- /TEST must not actually write anything")
	}
}

// readRawVolumeFile reads name's exact on-disk bytes back off vol's master
// file directory, bypassing odsrms.Reader's own per-record-format parsing
// entirely (Session.Type, which goes through that parsing, can't be used to
// verify a /BINARY-copied file's content: /BINARY creates an
// Undefined-format destination -- see createBinaryFile/
// createBinaryFileFromHost -- and odsrms.Reader treats Undefined exactly
// like Fixed, reading MaxRecordSize-byte records with no framing of its
// own; a raw copy whose length isn't an exact multiple of that record size,
// like this file's own short test content, would fail with ErrCorruptRecord
// on the final short record even though the bytes themselves are exactly
// right -- the same "no un-framed reading of an Undefined file" limitation
// ods2's own rms.Reader has, not a govax-specific defect). Mirrors
// copyRawToVolume/copyRawToHost's own block-by-block reading, using the
// exported odsrms.FileByteLength to know where the real data ends.
func readRawVolumeFile(t *testing.T, vol *volume.Volume, fullName string) []byte {
	t.Helper()

	dir, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	entry, err := dir.Lookup(fullName, 0)
	if err != nil {
		t.Fatalf("Lookup(%s): %v", fullName, err)
	}

	f, err := vol.OpenFID(entry.Fid)
	if err != nil {
		t.Fatalf("OpenFID(%s): %v", fullName, err)
	}

	remaining := odsrms.FileByteLength(f.Header.RecordAttributes)

	var data []byte

	block := make([]byte, ondisk.BlockSize)
	for vbn := uint32(1); remaining > 0; vbn++ {
		if err := f.ReadBlock(vbn, block); err != nil {
			t.Fatalf("ReadBlock(%s): %v", fullName, err)
		}

		n := int64(len(block))
		if n > remaining {
			n = remaining
		}

		data = append(data, block[:n]...)
		remaining -= n
	}

	return data
}

// TestSessionCopy_binaryVolumeToVolume confirms /BINARY copies exact raw
// bytes rather than the default text reframing: FOO.TXT is recreated
// here (via createTestFile with content lacking a trailing newline) so
// the default text path's own "always append a line ending" behavior
// would visibly add one -- /BINARY must not.
func TestSessionCopy_binaryVolumeToVolume(t *testing.T) {
	s, vol := newCopyTestSession(t)

	createTestFile(t, vol, "RAW.TXT", "abc")

	if _, err := s.Copy("RAW.TXT", false, "RAWCOPY.TXT", false, CopyOptions{Binary: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	text := readRawVolumeFile(t, vol, "RAWCOPY.TXT")

	if string(text) != "abc" {
		t.Errorf("binary-copied content = %q, want exactly %q (no appended line ending)", text, "abc")
	}
}

// TestSessionCopy_binaryFromHost mirrors TestSessionCopy_binaryVolumeToVolume
// for a /HOST source.
func TestSessionCopy_binaryFromHost(t *testing.T) {
	s, vol := newCopyTestSession(t)

	hostPath := filepath.Join(t.TempDir(), "raw.dat")
	if err := os.WriteFile(hostPath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := s.Copy(hostPath, true, "RAWFROMHOST.DAT", false, CopyOptions{Binary: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	text := readRawVolumeFile(t, vol, "RAWFROMHOST.DAT")

	if string(text) != "abc" {
		t.Errorf("binary-copied content = %q, want exactly %q", text, "abc")
	}
}

// TestSessionCopy_binaryToHost confirms /BINARY on a container-to-host
// copy writes the source's exact raw bytes, again using a
// trailing-newline-free fixture to distinguish it from the default text
// path.
func TestSessionCopy_binaryToHost(t *testing.T) {
	s, vol := newCopyTestSession(t)

	createTestFile(t, vol, "RAW.TXT", "abc")

	outPath := filepath.Join(t.TempDir(), "raw.out")

	if _, err := s.Copy("RAW.TXT", false, outPath, true, CopyOptions{Binary: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != "abc" {
		t.Errorf("binary-copied host content = %q, want exactly %q (no appended line ending)", got, "abc")
	}
}

// TestSessionCopy_streamPreservesRawBytesForStreamSource confirms /STREAM
// copies a Stream-format source's exact on-disk bytes rather than
// re-splitting it into records and reinserting the default '\n' line
// ending -- exercised here with a Stream_CR file (raw '\r'-delimited
// content), whose default text-mode copy would normalize those '\r's to
// '\n's.
func TestSessionCopy_streamPreservesRawBytesForStreamSource(t *testing.T) {
	s, vol := newCopyTestSession(t)

	createFormattedTestFile(t, vol, "CR.TXT", "line1\rline2\r", ondisk.RecordFormatStreamCR)

	outPath := filepath.Join(t.TempDir(), "stream.out")

	if _, err := s.Copy("CR.TXT", false, outPath, true, CopyOptions{Stream: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != "line1\rline2\r" {
		t.Errorf("/STREAM-copied content = %q, want the original raw bytes %q", got, "line1\rline2\r")
	}
}

// TestSessionCopy_withoutStreamNormalizesLineEndings is
// TestSessionCopy_streamPreservesRawBytesForStreamSource's own control:
// without /STREAM, the same Stream_CR source is reframed with the
// default '\n' line ending instead of preserving the original '\r's --
// confirming the two tests are actually exercising different code paths,
// not that /STREAM simply has no effect either way.
func TestSessionCopy_withoutStreamNormalizesLineEndings(t *testing.T) {
	s, vol := newCopyTestSession(t)

	createFormattedTestFile(t, vol, "CR.TXT", "line1\rline2\r", ondisk.RecordFormatStreamCR)

	outPath := filepath.Join(t.TempDir(), "normalized.out")

	if _, err := s.Copy("CR.TXT", false, outPath, true, CopyOptions{}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != "line1\nline2\n" {
		t.Errorf("default (non-/STREAM) copied content = %q, want normalized %q", got, "line1\nline2\n")
	}
}

// TestSessionCopy_crlf confirms /CRLF selects "\r\n" as the line ending
// for a container-to-host text copy, in place of the default "\n".
func TestSessionCopy_crlf(t *testing.T) {
	s, _ := newCopyTestSession(t)

	outPath := filepath.Join(t.TempDir(), "crlf.out")

	if _, err := s.Copy("FOO.TXT", false, outPath, true, CopyOptions{CRLF: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != "line one\r\nline two\r\n" {
		t.Errorf("/CRLF-copied content = %q, want %q", got, "line one\r\nline two\r\n")
	}
}

// TestSessionCopy_ignoreRecoversFromCorruptRecord confirms /IGNORE
// recovers from a corrupt record (a Stream_CRLF file ending in a lone
// '\r' with no following '\n' -- odsrms.ErrCorruptRecord's own documented
// trigger for that format) by restarting the destination as an exact raw
// byte copy, matching ods2's own cmdCopy/copyOneFile.
func TestSessionCopy_ignoreRecoversFromCorruptRecord(t *testing.T) {
	s, vol := newCopyTestSession(t)

	const corrupt = "line one\r\nbroken\r"
	createFormattedTestFile(t, vol, "CORRUPT.TXT", corrupt, ondisk.RecordFormatStreamCRLF)

	outPath := filepath.Join(t.TempDir(), "ignored.out")

	if _, err := s.Copy("CORRUPT.TXT", false, outPath, true, CopyOptions{Ignore: true}); err != nil {
		t.Fatalf("Copy with /IGNORE: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != corrupt {
		t.Errorf("/IGNORE-recovered content = %q, want the original raw bytes %q", got, corrupt)
	}
}

// TestSessionCopy_withoutIgnoreFailsOnCorruptRecord is this file's own
// control for TestSessionCopy_ignoreRecoversFromCorruptRecord: without
// /IGNORE, the same corrupt source fails the whole copy instead of
// recovering.
func TestSessionCopy_withoutIgnoreFailsOnCorruptRecord(t *testing.T) {
	s, vol := newCopyTestSession(t)

	createFormattedTestFile(t, vol, "CORRUPT.TXT", "line one\r\nbroken\r", ondisk.RecordFormatStreamCRLF)

	outPath := filepath.Join(t.TempDir(), "failed.out")

	if _, err := s.Copy("CORRUPT.TXT", false, outPath, true, CopyOptions{}); err == nil {
		t.Fatal("Copy of a corrupt-record source without /IGNORE = nil error, want an error")
	}
}

// TestSessionCopy_time confirms /TIME sets the copied host file's
// modification time to the source's own VMS revision date, and leaves
// CopyResult.Warning empty on success.
func TestSessionCopy_time(t *testing.T) {
	s, _ := newCopyTestSession(t)

	outPath := filepath.Join(t.TempDir(), "timed.out")

	results, err := s.Copy("FOO.TXT", false, outPath, true, CopyOptions{Time: true})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if results[0].Warning != "" {
		t.Errorf("Warning = %q, want empty on a successful /TIME copy", results[0].Warning)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	// volume.Initialize/CreateFile stamp a revision date at file-creation
	// time (effectively "now" for this test's own newCopyTestSession
	// call); a generous one-minute tolerance avoids flakiness from VMS
	// time's own coarser-than-Go encoding/rounding without weakening the
	// check into a no-op.
	if diff := info.ModTime().Sub(time.Now()); diff > time.Minute || diff < -time.Minute {
		t.Errorf("copied file's mtime = %v, want close to now (source's own revision date)", info.ModTime())
	}
}

// TestSessionCopy_dirsPreservesSubdirectoriesAndMaterializesDirEntries
// confirms /DIRS, for a wildcarded, recursive source copied onto an
// existing host directory: (1) a matched ordinary file found in a
// subdirectory has that subdirectory path mirrored under the
// destination, and (2) a matched directory entry itself is materialized
// as an empty host directory, rather than either being silently
// flattened/skipped the way they are without /DIRS.
//
// The fixture is deliberately scoped to its own TESTDIR subdirectory
// (rather than recursing from the volume's own master file directory)
// so the wildcard match set is exactly the two entries this test cares
// about -- volume.Initialize's own reserved files (INDEXF.SYS and
// friends) live only in the MFD itself, never inside a user-created
// subdirectory, so they can't appear in TESTDIR's own recursive listing.
func TestSessionCopy_dirsPreservesSubdirectoriesAndMaterializesDirEntries(t *testing.T) {
	s, vol := newCopyTestSession(t)

	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	bm, err := mfd.Device.Bitmap()
	if err != nil {
		t.Fatalf("Bitmap: %v", err)
	}

	ib, err := mfd.Device.IndexBitmap()
	if err != nil {
		t.Fatalf("IndexBitmap: %v", err)
	}

	testDir, err := vol.CreateDirectory(mfd, "TESTDIR.DIR", 0, bm, ib)
	if err != nil {
		t.Fatalf("CreateDirectory(TESTDIR.DIR): %v", err)
	}

	nested, err := vol.CreateFile(testDir, "NESTED.TXT", ondisk.RecAttr{
		Format:        ondisk.RecordFormatStreamLF,
		MaxRecordSize: 512,
	}, bm, ib)
	if err != nil {
		t.Fatalf("CreateFile(NESTED.TXT): %v", err)
	}

	const nestedContent = "nested content\n"

	block := make([]byte, ondisk.BlockSize)
	copy(block, nestedContent)

	if err := nested.WriteBlock(1, block); err != nil {
		t.Fatalf("WriteBlock(NESTED.TXT): %v", err)
	}

	if err := nested.CloseWithFinalByte(uint16(len(nestedContent))); err != nil {
		t.Fatalf("Close(NESTED.TXT): %v", err)
	}

	if _, err := vol.CreateDirectory(testDir, "SUBDIR.DIR", 0, bm, ib); err != nil {
		t.Fatalf("CreateDirectory(SUBDIR.DIR): %v", err)
	}

	destDir := t.TempDir()

	results, err := s.Copy("[TESTDIR...]*.*;*", false, destDir, true, CopyOptions{Dirs: true})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Copy returned %d results, want 2 (NESTED.TXT + SUBDIR.DIR): %+v", len(results), results)
	}

	nestedPath := filepath.Join(destDir, "TESTDIR", "NESTED.TXT;1")

	got, err := os.ReadFile(nestedPath)
	if err != nil {
		t.Fatalf("expected %s to exist with the subdirectory path preserved: %v", nestedPath, err)
	}

	if string(got) != nestedContent {
		t.Errorf("nested file content = %q, want %q", got, nestedContent)
	}

	subdirPath := filepath.Join(destDir, "TESTDIR", "SUBDIR")

	info, err := os.Stat(subdirPath)
	if err != nil {
		t.Fatalf("expected %s to exist as a materialized directory: %v", subdirPath, err)
	}

	if !info.IsDir() {
		t.Errorf("%s exists but is not a directory", subdirPath)
	}
}

// TestSessionCopy_withoutDirsFlattensAndSkipsDirEntries is this file's own
// control for the /DIRS test above: without /DIRS, the same recursive
// wildcard match set still copies NESTED.TXT (a matched file with a
// non-empty Dirs still copies even without /DIRS -- it's just flattened
// directly into the destination directory rather than mirrored under a
// matching subdirectory there), but skips SUBDIR.DIR entirely, since
// there's no "create an empty directory" operation without /DIRS to
// materialize it -- matching ods2's own default-skip behavior for a
// directory entry.
func TestSessionCopy_withoutDirsFlattensAndSkipsDirEntries(t *testing.T) {
	s, vol := newCopyTestSession(t)

	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	bm, err := mfd.Device.Bitmap()
	if err != nil {
		t.Fatalf("Bitmap: %v", err)
	}

	ib, err := mfd.Device.IndexBitmap()
	if err != nil {
		t.Fatalf("IndexBitmap: %v", err)
	}

	testDir, err := vol.CreateDirectory(mfd, "TESTDIR.DIR", 0, bm, ib)
	if err != nil {
		t.Fatalf("CreateDirectory(TESTDIR.DIR): %v", err)
	}

	if _, err := vol.CreateFile(testDir, "NESTED.TXT", ondisk.RecAttr{
		Format:        ondisk.RecordFormatStreamLF,
		MaxRecordSize: 512,
	}, bm, ib); err != nil {
		t.Fatalf("CreateFile(NESTED.TXT): %v", err)
	}

	if _, err := vol.CreateDirectory(testDir, "SUBDIR.DIR", 0, bm, ib); err != nil {
		t.Fatalf("CreateDirectory(SUBDIR.DIR): %v", err)
	}

	destDir := t.TempDir()

	results, err := s.Copy("[TESTDIR...]*.*;*", false, destDir, true, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	// SUBDIR.DIR is skipped (no /DIRS); NESTED.TXT is still copied, just
	// flattened directly into destDir rather than under TESTDIR/.
	if len(results) != 1 {
		t.Fatalf("Copy returned %d results, want 1 (NESTED.TXT only): %+v", len(results), results)
	}

	flatPath := filepath.Join(destDir, "NESTED.TXT;1")
	if _, err := os.Stat(flatPath); err != nil {
		t.Errorf("expected %s to exist (flattened, no /DIRS): %v", flatPath, err)
	}

	if _, err := os.Stat(filepath.Join(destDir, "TESTDIR")); err == nil {
		t.Error("TESTDIR subdirectory should not have been created without /DIRS")
	}
}

// TestSessionCopy_dirsTest confirms /TEST previews a materialized
// directory entry with CopyDirTested rather than CopyDirCreated, and
// doesn't actually create anything.
func TestSessionCopy_dirsTest(t *testing.T) {
	s, vol := newCopyTestSession(t)

	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	bm, err := mfd.Device.Bitmap()
	if err != nil {
		t.Fatalf("Bitmap: %v", err)
	}

	ib, err := mfd.Device.IndexBitmap()
	if err != nil {
		t.Fatalf("IndexBitmap: %v", err)
	}

	if _, err := vol.CreateDirectory(mfd, "EMPTYSUB.DIR", 0, bm, ib); err != nil {
		t.Fatalf("CreateDirectory(EMPTYSUB.DIR): %v", err)
	}

	destDir := t.TempDir()

	results, err := s.Copy("EMPTYSUB.DIR", false, destDir, true, CopyOptions{Dirs: true, Test: true})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if len(results) != 1 || results[0].Kind != CopyDirTested {
		t.Fatalf("Copy results = %+v, want a single CopyDirTested result", results)
	}

	if _, err := os.Stat(filepath.Join(destDir, "EMPTYSUB")); err == nil {
		t.Error("directory should not have been created under /TEST")
	}
}
