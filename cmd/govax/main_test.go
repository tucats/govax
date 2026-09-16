package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if err := run(nil, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(buf.String(), "govax") {
		t.Errorf("output = %q, want the startup banner", buf.String())
	}
}

// TestRun_instructionLimitStopsARunawayProgram exercises govax's own
// -instruction-limit flag end to end (docs/PHASE-15.md's sub-phase 2):
// after the embedded vax.init finishes booting (leaving PC at 0x200, per
// its own "SET PC=200"), an interactively deposited infinite loop (NOP;
// BRB back to itself) must not hang the process when a low instruction
// limit is configured -- GO should return control to the prompt instead.
func TestRun_instructionLimitStopsARunawayProgram(t *testing.T) {
	script := "D 200 01\nD 201 11\nD 202 0FD\nGO\n"
	in := io.NopCloser(strings.NewReader(script))

	var buf bytes.Buffer
	if err := run(nil, 5, 0, &buf, in, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(buf.String(), "INSTRLIMIT") {
		t.Errorf("output = %q, want an instruction-limit message", buf.String())
	}
}

// TestRun_timeLimitStopsARunawayProgram is -time-limit's own counterpart to
// TestRun_instructionLimitStopsARunawayProgram above.
func TestRun_timeLimitStopsARunawayProgram(t *testing.T) {
	script := "D 200 01\nD 201 11\nD 202 0FD\nGO\n"
	in := io.NopCloser(strings.NewReader(script))

	var buf bytes.Buffer

	if err := run(nil, 0, 10*time.Millisecond, &buf, in, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	
	if !strings.Contains(buf.String(), "TIMELIMIT") {
		t.Errorf("output = %q, want a time-limit message", buf.String())
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

	if err := run([]string{dir}, 0, 0, &buf, emptyStdin(), nil); err != nil {
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

	if err := run([]string{pathDir}, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(buf.String(), "as-given-vax-init-ran") {
		t.Errorf("output = %q, want the CWD-relative vax.init's own PRINT output", buf.String())
	}
}

// TestRun_interactiveAsmRepl exercises docs/PHASE-19.md's own interactive
// "ASM" mode end to end through the real readline loop: a bare ASM enters
// assembler mode, several lines are typed one at a time (matching what a
// real terminal session would feed the readline loop), and "END <entry>"
// both exits the mode and auto-CALLs the routine just typed -- confirmed by
// EXAMINE-ing the register it set afterward, back in ordinary command mode.
func TestRun_interactiveAsmRepl(t *testing.T) {
	script := strings.Join([]string{
		"ASM",
		".ENTRY MYTEST,^M<>",
		"MOVL #42,R0",
		"RET",
		"END MYTEST",
		"EXAM R0",
	}, "\n") + "\n"

	in := io.NopCloser(strings.NewReader(script))

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, in, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	// #42 is hex (this port's -- and the reference tool's -- default
	// numeric radix; see docs/PHASE-11.md's own progress log on this).
	if !strings.Contains(buf.String(), "00000042") {
		t.Errorf("output = %q, want R0 = 00000042 from the auto-CALLed routine", buf.String())
	}
}
