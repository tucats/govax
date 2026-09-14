package main

import (
	"bytes"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// emptyStdin lets tests pass a deterministic, immediately-EOF input source
// to run's readline loop instead of relying on the test process's real
// stdin.
func emptyStdin() io.ReadCloser { return io.NopCloser(strings.NewReader("")) }

func testDataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "dcl")
}

// run loads the real grammar/help files and vax.init startup script; the
// script exercises several subsystems this port doesn't implement yet
// (assembler, RTL, devices — see docs/PHASE-08.md), so this only checks
// that startup completes without a fatal error and that grammar/help load
// correctly, not that vax.init runs cleanly end-to-end.
func TestRun_startupDoesNotFatallyFail(t *testing.T) {
	var buf bytes.Buffer
	if err := run(testDataDir(t), &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "govax") {
		t.Errorf("output = %q, want the startup banner", buf.String())
	}
}

func TestRun_missingGrammarFileErrors(t *testing.T) {
	var buf bytes.Buffer
	if err := run(t.TempDir(), &buf, emptyStdin(), nil); err == nil {
		t.Error("expected an error when evax.dcl is missing")
	}
}
