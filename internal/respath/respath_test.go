package respath

import (
	"errors"
	"os"
	"path/filepath"
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
