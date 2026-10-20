package rms

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ods2/ondisk"
)

// piSaveSet is the first save set VMS wrote (testdata/backup69): two
// blocks of 32256 bytes.
const piSaveSet = "../../testdata/backup69/vax/pi.bck"

// newSaveSetVolume is a session on a volume holding PI.BCK, copied onto
// it as COPY/BINARY copies (undefined records), with SET DEFAULT DUA0:.
func newSaveSetVolume(t *testing.T) (*Session, []byte) {
	t.Helper()

	want, err := os.ReadFile(piSaveSet)
	if err != nil {
		t.Fatal(err)
	}

	s, _ := newCopyTestSession(t)
	if _, err := s.Copy(piSaveSet, true, "PI.BCK", false, CopyOptions{Binary: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	return s, want
}

// checkReadAt reads f in pieces that start and end inside blocks, and
// across them, and whole, comparing each with want.
func checkReadAt(t *testing.T, f *SaveSetFile, want []byte) {
	t.Helper()

	if f.Size != int64(len(want)) {
		t.Fatalf("Size %d, want %d", f.Size, len(want))
	}

	for _, r := range []struct{ off, n int }{
		{0, len(want)}, {0, 256}, {500, 30}, {511, 2}, {32256, 32256}, {1000, 5000}, {64000, 512},
	} {
		got := make([]byte, r.n)

		n, err := f.ReadAt(got, int64(r.off))
		if err != nil || n != r.n {
			t.Fatalf("ReadAt(%d, %d) = %d, %v", r.off, r.n, n, err)
		}

		if !bytes.Equal(got, want[r.off:r.off+r.n]) {
			t.Errorf("ReadAt(%d, %d) differs", r.off, r.n)
		}
	}

	// Past the end: what's there, then io.EOF.
	got := make([]byte, 100)

	n, err := f.ReadAt(got, int64(len(want)-40))
	if n != 40 || !errors.Is(err, io.EOF) || !bytes.Equal(got[:40], want[len(want)-40:]) {
		t.Errorf("ReadAt across the end = %d, %v", n, err)
	}

	if n, err := f.ReadAt(got, int64(len(want))); n != 0 || !errors.Is(err, io.EOF) {
		t.Errorf("ReadAt at the end = %d, %v", n, err)
	}
}

// TestOpenSaveSetVolume: a save set on a volume reads as its bytes, up
// to its end of file, by its full name; its records match VMS's only
// when they're fixed-length records of the block size.
func TestOpenSaveSetVolume(t *testing.T) {
	s, want := newSaveSetVolume(t)

	f, err := s.OpenSaveSet(FileLocation{Name: "PI.BCK"})
	if err != nil {
		t.Fatal(err)
	}

	if f.Name != "DUA0:[000000]PI.BCK;1" || f.Host {
		t.Errorf("Name %q, Host %v", f.Name, f.Host)
	}

	checkReadAt(t, f, want)

	if f.RecordsMatch(32256) {
		t.Error("undefined records match a save set's")
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// Given VMS's attributes, fixed-length records of the block size, it
	// matches, and reads the same.
	fid, _, err := s.Mounts.ACPLookup("DUA0", mfd, "PI.BCK")
	if err != nil {
		t.Fatal(err)
	}

	err = s.Mounts.ACPWriteAttributes("DUA0", fid, func(a *ACPAttributes) {
		ra := recAttrOf(t, *a)
		ra.Format = ondisk.RecordFormatFixed
		ra.RecordSize = 32256
		ra.MaxRecordSize = 32256
		copy(a.RecordAttributes[:], ondisk.EncodeRecAttr(ra))
	})
	if err != nil {
		t.Fatal(err)
	}

	f, err = s.OpenSaveSet(FileLocation{Name: "DUA0:[000000]PI.BCK"})
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	if !f.RecordsMatch(32256) || f.RecordsMatch(8192) {
		t.Errorf("fixed 32256 records: RecordsMatch(32256) %v, (8192) %v", f.RecordsMatch(32256), f.RecordsMatch(8192))
	}

	checkReadAt(t, f, want)
}

// TestOpenSaveSetSharing: while a save set is open others may read it
// but not write it, and Close lets it go.
func TestOpenSaveSetSharing(t *testing.T) {
	s, _ := newSaveSetVolume(t)

	f, err := s.OpenSaveSet(FileLocation{Name: "PI.BCK"})
	if err != nil {
		t.Fatal(err)
	}

	fid, _, err := s.Mounts.ACPLookup("DUA0", mfd, "PI.BCK")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Mounts.ACPAccess("DUA0", fid, true); !errors.Is(err, ErrACPAccessConflict) {
		t.Errorf("a writer while the save set is open: %v", err)
	}

	r, err := s.Mounts.ACPAccess("DUA0", fid, false)
	if err != nil {
		t.Errorf("a reader while the save set is open: %v", err)
	} else {
		_ = r.Deaccess()
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	w, err := s.Mounts.ACPAccess("DUA0", fid, true)
	if err != nil {
		t.Fatalf("a writer after Close: %v", err)
	}

	_ = w.Deaccess()
}

// TestOpenSaveSetErrors: a missing save set is RMS$_FNF with its
// expanded name, as BACKUP's OPENIN shows it, a wildcard RMS$_WLD, and a
// missing host file RMS$_FNF with its path.
func TestOpenSaveSetErrors(t *testing.T) {
	s, _ := newSaveSetVolume(t)

	missingHost := filepath.Join(t.TempDir(), "none.bck")

	for _, c := range []struct {
		loc    FileLocation
		name   string
		status uint32
	}{
		{FileLocation{Name: "NONE.BCK"}, "DUA0:[000000]NONE.BCK;", rmsFileNotFound},
		{FileLocation{Name: "[NOSUCH]NONE.BCK"}, "DUA0:[NOSUCH]NONE.BCK;", rmsDirNotFound},
		{FileLocation{Name: "*.BCK"}, "DUA0:[000000]*.BCK;", rmsWildcardError},
		{FileLocation{Name: missingHost, Host: true}, missingHost, rmsFileNotFound},
	} {
		_, err := s.OpenSaveSet(c.loc)

		var fileErr *DCLFileError
		if !errors.As(err, &fileErr) {
			t.Errorf("%s: %v, want a *DCLFileError", c.loc.Name, err)

			continue
		}

		if fileErr.Name != c.name || fileErr.Status != c.status {
			t.Errorf("%s: %q %%X%08X, want %q %%X%08X", c.loc.Name, fileErr.Name, fileErr.Status, c.name, c.status)
		}
	}

	if _, err := s.OpenSaveSet(FileLocation{Name: t.TempDir(), Host: true}); err == nil {
		t.Error("a host directory opened as a save set")
	}
}

// TestOpenSaveSetHost: a host file reads as its bytes, by its path, and
// has no records to disagree with.
func TestOpenSaveSetHost(t *testing.T) {
	want, err := os.ReadFile(piSaveSet)
	if err != nil {
		t.Fatal(err)
	}

	s := NewSession(NewMountTable())

	f, err := s.OpenSaveSet(FileLocation{Name: piSaveSet, Host: true})
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	if f.Name != piSaveSet || !f.Host || !f.RecordsMatch(32256) {
		t.Errorf("Name %q, Host %v, RecordsMatch %v", f.Name, f.Host, f.RecordsMatch(32256))
	}

	checkReadAt(t, f, want)
}
