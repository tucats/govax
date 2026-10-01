package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRMSCopy_govaxStarlet assembles Phase 28's rmscopy.mar with govax's
// own STARLET.MLB (no VMS library: the host library directory is empty),
// links it, and runs it with its source on a volume: it copies
// RMSCOPY.MAR to RMSCOPY.OUT a record at a time through $FAB, $RAB, and
// the RMS service macros (docs/PHASE-32.md, subtask 7), as it does on
// VMS (testdata/mar/macros/vax/macros.log).
func TestRMSCopy_govaxStarlet(t *testing.T) {
	c := newBootableConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	c.HostLibrary = t.TempDir()

	mountFreshRMSVolume(t, c)

	src := filepath.Join("..", "..", "testdata", "mar", "macros", "rmscopy.mar")
	dir := t.TempDir()

	if err := c.Macro(MacroOptions{Source: src, Object: filepath.Join(dir, "rmscopy.obj")}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "rmscopy")}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if err := d.Dispatch(`COPY "` + src + `"/HOST DUA0:[000000]RMSCOPY.MAR`); err != nil {
		t.Fatalf("COPY to the volume: %v", err)
	}

	if err := c.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	if r0 := runImage(t, c, filepath.Join(dir, "rmscopy.exe")); r0&1 != 1 {
		t.Fatalf("R0 = %#x, want a success status", r0)
	}

	out := filepath.Join(dir, "rmscopy.out")
	if err := d.Dispatch(`COPY DUA0:[000000]RMSCOPY.OUT "` + out + `"/HOST`); err != nil {
		t.Fatalf("COPY from the volume: %v", err)
	}

	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	if norm := func(b []byte) string { return strings.TrimRight(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") }; norm(got) != norm(want) {
		t.Errorf("RMSCOPY.OUT:\n%s\nwant RMSCOPY.MAR:\n%s", got, want)
	}
}
