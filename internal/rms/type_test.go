package rms

import (
	"errors"
	"testing"

	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
	"github.com/tucats/ods2/volume"
)

// newTypeTestSession builds a fresh, writable, mounted test volume (device
// DUA0, reusing newTestVolumeFile from mount_test.go) with a Stream_LF
// file, FOO.TXT, and a two-version file, DUP.TXT;1/;2, to exercise TYPE's
// "no version selects the highest" and "a name with several surviving
// versions is fine, a wildcard matching several distinct names is not"
// rules.
func newTypeTestSession(t *testing.T) (*Session, *volume.Volume) {
	t.Helper()

	mounts := NewMountTable()
	path := newTestVolumeFile(t, "TYPEVOL")

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

// createVFCTestFile creates name as a VFC-formatted file whose records are
// each of lines, using odsrms.Writer directly (records.go's writeRecords is
// the code under test here, so the fixture has to be built independently of
// it). Each record's VFC control bytes are {0, 1}: no leading control, and
// a trailing "one newline then one carriage return" -- vfcTrailing's
// simplest non-zero encoding (see rms/vfc.go in the sibling ods2 module) --
// so the expected rendered text is fully predictable: each line followed
// by "\n\r", with nothing prepended.
func createVFCTestFile(t *testing.T, vol *volume.Volume, name string, lines []string) {
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
		Format:        ondisk.RecordFormatVFC,
		VfcSize:       2,
		MaxRecordSize: 132,
	}, bm, ib)
	if err != nil {
		t.Fatalf("CreateFile(%s): %v", name, err)
	}

	w, err := odsrms.NewWriter(f)
	if err != nil {
		t.Fatalf("NewWriter(%s): %v", name, err)
	}

	for _, line := range lines {
		record := append([]byte{0, 1}, []byte(line)...)
		if err := w.Put(record); err != nil {
			t.Fatalf("Put(%s, %q): %v", name, line, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close(%s): %v", name, err)
	}
}

// TestSession_typeStreamFile confirms the basic happy path: a Stream_LF
// file's content comes back verbatim, matching ods2's own
// TestCmdTypeStreamFile.
func TestSession_typeStreamFile(t *testing.T) {
	s, _ := newTypeTestSession(t)

	text, err := s.Type("FOO.TXT")
	if err != nil {
		t.Fatalf("Type: %v", err)
	}

	const want = "line one\nline two\n"
	if text != want {
		t.Errorf("Type(FOO.TXT) = %q, want %q", text, want)
	}
}

// TestSession_typeVFCFile confirms VFC records are rendered with their
// carriage control expanded, not left as raw framed bytes -- the
// "record-format-aware" half of this subtask's own requirement, which a
// Stream_LF-only test fixture can't exercise.
func TestSession_typeVFCFile(t *testing.T) {
	s, vol := newTypeTestSession(t)
	createVFCTestFile(t, vol, "REPORT.LIS", []string{"LINE1", "LINE2"})

	text, err := s.Type("REPORT.LIS")
	if err != nil {
		t.Fatalf("Type: %v", err)
	}

	const want = "LINE1\n\rLINE2\n\r"
	if text != want {
		t.Errorf("Type(REPORT.LIS) = %q, want %q", text, want)
	}
}

// TestSession_typeNotFound confirms a name matching nothing at all on the
// volume surfaces as a *NotFoundError, the same failure mode Session.Delete
// already reports this way.
func TestSession_typeNotFound(t *testing.T) {
	s, _ := newTypeTestSession(t)

	_, err := s.Type("NOSUCHFILE.TXT")
	if err == nil {
		t.Fatal("Type of a nonexistent file = nil error, want an error")
	}

	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("Type error = %v, want a *NotFoundError", err)
	}
}

// TestSession_typeAmbiguousWildcard confirms a name/type pattern matching
// more than one distinct file surfaces as an *AmbiguousError -- TYPE never
// accepts wildcards, matching ods2's own cmdType.
func TestSession_typeAmbiguousWildcard(t *testing.T) {
	s, _ := newTypeTestSession(t)

	_, err := s.Type("*.TXT")
	if err == nil {
		t.Fatal("Type with a wildcard matching several files = nil error, want an error")
	}

	var ambiguous *AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("Type error = %v, want a *AmbiguousError", err)
	}

	if ambiguous.Count < 2 {
		t.Errorf("AmbiguousError.Count = %d, want at least 2", ambiguous.Count)
	}
}

// TestSession_typeNoVersionSelectsHighest confirms a name with several
// surviving versions but no version typed is normal, unambiguous VMS
// behavior (the single highest version), not an *AmbiguousError -- matching
// ods2's own TestCmdTypeNoVersionSelectsHighest.
func TestSession_typeNoVersionSelectsHighest(t *testing.T) {
	s, _ := newTypeTestSession(t)

	text, err := s.Type("DUP.TXT")
	if err != nil {
		t.Fatalf("Type(DUP.TXT) with no version given: %v", err)
	}

	// createTestFile's content has no embedded newline, so its one Stream
	// record has no trailing delimiter of its own -- writeRecords still
	// appends the default '\n' line ending after it, the same way ods2's
	// own writeRecords always terminates the last record of a delimiter-
	// less stream file.
	const want = "version two\n"
	if text != want {
		t.Errorf("Type(DUP.TXT) = %q, want the highest version's content %q", text, want)
	}
}

// TestSession_typeExplicitVersion confirms an explicit version selects
// exactly that version rather than always defaulting to the highest.
func TestSession_typeExplicitVersion(t *testing.T) {
	s, _ := newTypeTestSession(t)

	text, err := s.Type("DUP.TXT;1")
	if err != nil {
		t.Fatalf("Type(DUP.TXT;1): %v", err)
	}

	const want = "version one\n"
	if text != want {
		t.Errorf("Type(DUP.TXT;1) = %q, want %q", text, want)
	}
}

// TestSession_typeNotMounted confirms specText naming a device with
// nothing mounted on it surfaces as an *NotMountedError, the same failure
// mode every other command in this phase already reports this way.
func TestSession_typeNotMounted(t *testing.T) {
	s := NewSession(NewMountTable())

	_, err := s.Type("DUB0:FOO.TXT")
	if err == nil {
		t.Fatal("Type against an unmounted device = nil error, want an error")
	}

	var notMounted *NotMountedError
	if !errors.As(err, &notMounted) {
		t.Fatalf("Type error = %v, want a *NotMountedError", err)
	}

	if notMounted.Device != "DUB0" {
		t.Errorf("NotMountedError.Device = %q, want DUB0", notMounted.Device)
	}
}

// TestSession_typeBadFileSpec confirms a malformed file specification is
// reported as a plain error, and specifically none of NotMountedError/
// NotFoundError/AmbiguousError -- the console layer's Console.Type relies
// on being able to tell these apart.
func TestSession_typeBadFileSpec(t *testing.T) {
	s, _ := newTypeTestSession(t)

	_, err := s.Type("DUA0:[UNTERMINATED")
	if err == nil {
		t.Fatal("Type with an unterminated directory bracket = nil error, want an error")
	}

	var notMounted *NotMountedError
	if errors.As(err, &notMounted) {
		t.Errorf("Type error = %v, want a plain error, not a *NotMountedError", err)
	}

	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		t.Errorf("Type error = %v, want a plain error, not a *NotFoundError", err)
	}

	var ambiguous *AmbiguousError
	if errors.As(err, &ambiguous) {
		t.Errorf("Type error = %v, want a plain error, not a *AmbiguousError", err)
	}
}
