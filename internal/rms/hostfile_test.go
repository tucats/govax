package rms

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// fallbackSession is a COPY test session whose HostFallback serves the
// one file fallback.txt.
func fallbackSession(t *testing.T) *Session {
	t.Helper()

	s, _ := newCopyTestSession(t)
	files := fstest.MapFS{"fallback.txt": &fstest.MapFile{Data: []byte("from the fallback\n")}}

	s.HostFallback = func(name string) ([]byte, error) {
		return fs.ReadFile(files, name)
	}

	return s
}

// TestReadRawFile_hostFallback reads a bare host name that isn't in the
// current directory through HostFallback, in either case.
func TestReadRawFile_hostFallback(t *testing.T) {
	s := fallbackSession(t)

	for _, name := range []string{"fallback.txt", "FALLBACK.TXT"} {
		data, found, err := s.ReadRawFile(FileLocation{Host: true, Name: name})
		if err != nil {
			t.Fatalf("ReadRawFile(%s): %v", name, err)
		}

		if string(data) != "from the fallback\n" || found.Name != name {
			t.Errorf("ReadRawFile(%s) = %q at %q", name, data, found.Name)
		}
	}

	records, _, err := s.ReadRecordFile(FileLocation{Host: true, Name: "fallback.txt"}, TextRecords)
	if err != nil || len(records) != 1 || string(records[0]) != "from the fallback" {
		t.Errorf("ReadRecordFile = %q, %v", records, err)
	}
}

// TestReadRawFile_hostFallbackOnlyForBareNames never looks for a name
// with a directory anywhere else, and a file that is there wins.
func TestReadRawFile_hostFallbackOnlyForBareNames(t *testing.T) {
	s := fallbackSession(t)
	dir := t.TempDir()

	_, _, err := s.ReadRawFile(FileLocation{Host: true, Name: filepath.Join(dir, "fallback.txt")})
	if !os.IsNotExist(err) {
		t.Errorf("a missing file with a directory: err = %v, want the host's not-exist error", err)
	}

	if _, _, err := s.ReadRawFile(FileLocation{Host: true, Name: "absent.txt"}); !os.IsNotExist(err) {
		t.Errorf("a name the fallback lacks: err = %v, want the host's not-exist error", err)
	}

	path := filepath.Join(dir, "fallback.txt")
	if err := os.WriteFile(path, []byte("from the host\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if data, _, err := s.ReadRawFile(FileLocation{Host: true, Name: path}); err != nil || string(data) != "from the host\n" {
		t.Errorf("ReadRawFile(%s) = %q, %v", path, data, err)
	}
}

// TestSessionCopy_fromHostFallback copies a /HOST source that only the
// fallback has onto the volume.
func TestSessionCopy_fromHostFallback(t *testing.T) {
	s := fallbackSession(t)

	results, err := s.Copy("fallback.txt", true, "COPIED.TXT", false, CopyOptions{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if results[0].Dest != "DUA0:[000000]COPIED.TXT;1" {
		t.Errorf("Dest = %q", results[0].Dest)
	}

	if text, err := s.Type("COPIED.TXT"); err != nil || text != "from the fallback\n" {
		t.Errorf("copied content = %q, %v", text, err)
	}
}
