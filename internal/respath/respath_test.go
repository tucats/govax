package respath

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestResolver_asGivenWinsOverEverything(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	name := filepath.Join(wd, "as-given.txt")
	if err := os.WriteFile(name, []byte("as-given"), 0o644); err != nil {
		t.Fatal(err)
	}
	
	t.Cleanup(func() { os.Remove(name) })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "as-given.txt"), []byte("from-dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	
	fallback := fstest.MapFS{"as-given.txt": &fstest.MapFile{Data: []byte("from-fallback")}}

	r := New([]string{dir}, fallback)
	
	got, err := r.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	
	if string(got) != "as-given" {
		t.Errorf("got %q, want the literal as-given file's own content", got)
	}
}

func TestResolver_searchesDirsInOrder(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	
	if err := os.WriteFile(filepath.Join(second, "f.txt"), []byte("from-second"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := New([]string{first, second}, nil)
	
	got, err := r.ReadFile("f.txt")
	if err != nil {
		t.Fatal(err)
	}
	
	if string(got) != "from-second" {
		t.Errorf("got %q, want second dir's content (first dir has no f.txt)", got)
	}

	// Now put it in the first dir too and confirm first wins.
	if err := os.WriteFile(filepath.Join(first, "f.txt"), []byte("from-first"), 0o644); err != nil {
		t.Fatal(err)
	}
	
	got, err = r.ReadFile("f.txt")
	if err != nil {
		t.Fatal(err)
	}
	
	if string(got) != "from-first" {
		t.Errorf("got %q, want first dir's content once both have the file", got)
	}
}

func TestResolver_fallsBackToEmbeddedFS(t *testing.T) {
	dir := t.TempDir() // present but doesn't have the file
	fallback := fstest.MapFS{"f.txt": &fstest.MapFile{Data: []byte("from-fallback")}}

	r := New([]string{dir}, fallback)
	
	got, err := r.ReadFile("f.txt")
	if err != nil {
		t.Fatal(err)
	}
	
	if string(got) != "from-fallback" {
		t.Errorf("got %q, want the fallback FS's content", got)
	}
}

func TestResolver_notFoundAnywhereListsEveryLocationTried(t *testing.T) {
	dir := t.TempDir()
	fallback := fstest.MapFS{}

	r := New([]string{dir}, fallback)
	
	_, err := r.ReadFile("missing.txt")
	if err == nil {
		t.Fatal("expected an error")
	}
	
	var nf *NotFoundError
	
	if !errors.As(err, &nf) {
		t.Fatalf("error = %v, want a *NotFoundError", err)
	}
	
	wantTried := []string{"missing.txt", filepath.Join(dir, "missing.txt"), "embedded:missing.txt"}
	if len(nf.Tried) != len(wantTried) {
		t.Fatalf("Tried = %v, want %v", nf.Tried, wantTried)
	}
	
	for i, w := range wantTried {
		if nf.Tried[i] != w {
			t.Errorf("Tried[%d] = %q, want %q", i, nf.Tried[i], w)
		}
	}
}

func TestResolver_nilResolverActsAsPlainRead(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "f.txt")
	
	if err := os.WriteFile(name, []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	var r *Resolver
	
	got, err := r.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	
	if string(got) != "plain" {
		t.Errorf("got %q, want plain os.ReadFile behavior", got)
	}

	if _, err := r.ReadFile(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("expected an error for a missing file with a nil Resolver")
	}
}

func TestResolver_withDirSearchesAheadOfExistingDirs(t *testing.T) {
	extra := t.TempDir()
	
	if err := os.WriteFile(filepath.Join(extra, "f.txt"), []byte("from-extra"), 0o644); err != nil {
		t.Fatal(err)
	}
	
	configured := t.TempDir()
	if err := os.WriteFile(filepath.Join(configured, "f.txt"), []byte("from-configured"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := New([]string{configured}, nil).WithDir(extra)
	
	got, err := r.ReadFile("f.txt")
	if err != nil {
		t.Fatal(err)
	}
	
	if string(got) != "from-extra" {
		t.Errorf("got %q, want WithDir's own extra directory to win", got)
	}
}

// TestResolver_volumeBetweenDirsAndFallback checks the system volume's
// place in the search (docs/PHASE-87 - system volume.md, Decision 4):
// after the search directories, before the fallback, and each file's
// origin reported.
func TestResolver_volumeBetweenDirsAndFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "host.txt"), []byte("from-dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := New([]string{dir}, fstest.MapFS{
		"host.txt":     &fstest.MapFile{Data: []byte("from-fallback")},
		"volume.txt":   &fstest.MapFile{Data: []byte("from-fallback")},
		"embedded.txt": &fstest.MapFile{Data: []byte("from-fallback")},
	})
	r.Volume = fstest.MapFS{
		"host.txt":   &fstest.MapFile{Data: []byte("from-volume")},
		"volume.txt": &fstest.MapFile{Data: []byte("from-volume")},
	}

	for name, want := range map[string]struct {
		text   string
		origin Origin
	}{
		"host.txt":     {"from-dir", Host},
		"volume.txt":   {"from-volume", Volume},
		"embedded.txt": {"from-fallback", Embedded},
	} {
		got, origin, err := r.ReadFileFrom(name)
		if err != nil || string(got) != want.text || origin != want.origin {
			t.Errorf("ReadFileFrom(%s) = %q, %v, %v; want %q, %v", name, got, origin, err, want.text, want.origin)
		}
	}

	if got, err := r.ReadEmbedded("volume.txt"); err != nil || string(got) != "from-fallback" {
		t.Errorf("ReadEmbedded = %q, %v", got, err)
	}

	// WithDir keeps the volume.
	if got, _, _ := r.WithDir(t.TempDir()).ReadFileFrom("volume.txt"); string(got) != "from-volume" {
		t.Errorf("WithDir lost the volume: %q", got)
	}
}

// failingFS is an fs.FS whose every file fails to open, for a damaged
// volume.
type failingFS struct{}

func (failingFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: errors.New("bad block")}
}

// TestResolver_volumeError checks that a volume's failure, other than a
// missing file, is a VolumeError with the Volume origin, which a caller can
// take as a reason to read the fallback's copy instead.
func TestResolver_volumeError(t *testing.T) {
	r := New(nil, fstest.MapFS{"f.txt": &fstest.MapFile{Data: []byte("built-in")}})
	r.Volume = failingFS{}

	_, origin, err := r.ReadFileFrom("f.txt")

	var ve *VolumeError
	if !errors.As(err, &ve) || origin != Volume || !strings.Contains(err.Error(), "bad block") {
		t.Errorf("ReadFileFrom = %v, %v; want a VolumeError", origin, err)
	}
}
