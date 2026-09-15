package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// emptyStdin lets tests pass a deterministic, immediately-EOF input source
// to run's readline loop instead of relying on the test process's real
// stdin.
func emptyStdin() io.ReadCloser { return io.NopCloser(strings.NewReader("")) }

// TestRun_startupBootsFromEmbeddedFilesAlone is docs/PHASE-15.md's own named
// deliverable: govax with zero -path flags must still boot correctly using
// only internal/bootdata's embedded copies of evax.dcl/vax.help/vax.init
// (and, via vax.init's own "asm kernel.asm", kernel.asm/ssdef.asm) — no
// testdata/ checkout required.
func TestRun_startupBootsFromEmbeddedFilesAlone(t *testing.T) {
	var buf bytes.Buffer
	if err := run(nil, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "govax") {
		t.Errorf("output = %q, want the startup banner", buf.String())
	}
}

// TestRun_pathOverridesEmbeddedForThatFileOnly exercises the per-file search
// order docs/PHASE-15.md describes: a -path directory holding only a
// customized vax.init is used for vax.init, while evax.dcl/vax.help still
// fall through to the embedded copies (never named on the command line) —
// a full replacement set isn't required to override one file.
func TestRun_pathOverridesEmbeddedForThatFileOnly(t *testing.T) {
	dir := t.TempDir()
	custom := `print "custom-vax-init-ran"` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "vax.init"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := run([]string{dir}, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "custom-vax-init-ran") {
		t.Errorf("output = %q, want the overriding vax.init's own PRINT output", buf.String())
	}
}

// TestRun_asGivenPathWinsOverPathFlag confirms a name that already resolves
// as given (relative to the working directory) is never overridden by a
// -path directory — the "as-given" leg of the search always runs first.
func TestRun_asGivenPathWinsOverPathFlag(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	asGiven := filepath.Join(wd, "vax.init")
	if err := os.WriteFile(asGiven, []byte(`print "as-given-vax-init-ran"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(asGiven) })

	pathDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pathDir, "vax.init"), []byte(`print "path-flag-vax-init-ran"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := run([]string{pathDir}, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "as-given-vax-init-ran") {
		t.Errorf("output = %q, want the CWD-relative vax.init's own PRINT output", buf.String())
	}
}
