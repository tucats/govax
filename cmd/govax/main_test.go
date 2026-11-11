package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmserrors"
)

// emptyStdin lets tests pass a deterministic, immediately-EOF input source
// to run's readline loop instead of relying on the test process's real
// stdin.
func emptyStdin() io.ReadCloser { return io.NopCloser(strings.NewReader("")) }

// TestRun_startupBootsFromEmbeddedFilesAlone is docs/planning/PHASE-15.md's own named
// deliverable: govax with zero -path flags must still boot correctly using
// only internal/bootdata's embedded copies of console.dcl/startup.com
// (and, via startup.com's own "asm kernel.asm", kernel.asm/ssdef.asm) — no
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

// TestRun_helpMountAndDismountFromEmbeddedFile: with no system volume (this
// package's tests have none, TestMain), HELP reads govax's own HELPLIB.HLB,
// built into it (internal/bootdata), and finds MOUNT's and DISMOUNT's
// topics. /NOPROMPT, since HELP otherwise prompts for more and would read
// the next line as a topic.
func TestRun_helpMountAndDismountFromEmbeddedFile(t *testing.T) {
	script := "HELP/NOPROMPT MOUNT\nHELP/NOPROMPT DISMOUNT\n"
	in := io.NopCloser(strings.NewReader(script))

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, in, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Sorry, no documentation") {
		t.Errorf("output = %q, want real MOUNT/DISMOUNT help text", out)
	}

	if !strings.Contains(out, "Mounts a container, a host file holding an ODS-2 volume") {
		t.Errorf("output = %q, want HELP MOUNT's own body text", out)
	}

	if !strings.Contains(out, "Dismounts the volume mounted on a device") {
		t.Errorf("output = %q, want HELP DISMOUNT's own body text", out)
	}
}

// TestRun_instructionLimitStopsARunawayProgram exercises govax's own
// -instruction-limit flag end to end (docs/planning/PHASE-15.md's sub-phase 2):
// after the embedded startup.com finishes booting, an infinite loop (NOP; BRB
// back to itself) deposited from the debugger (DEBUG starts a session on
// the booted machine) must not hang the process when a low instruction
// limit is configured -- GO should return control to the prompt instead.
func TestRun_instructionLimitStopsARunawayProgram(t *testing.T) {
	script := "DEBUG\nDEPOSIT 200 = 01\nDEPOSIT 201 = 11\nDEPOSIT 202 = 0FD\nGO 200\n"
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
	script := "DEBUG\nDEPOSIT 200 = 01\nDEPOSIT 201 = 11\nDEPOSIT 202 = 0FD\nGO 200\n"
	in := io.NopCloser(strings.NewReader(script))

	var buf bytes.Buffer

	if err := run(nil, 0, 10*time.Millisecond, &buf, in, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	
	if !strings.Contains(buf.String(), "TIMELIMIT") {
		t.Errorf("output = %q, want a time-limit message", buf.String())
	}
}

// TestRun_limitsStopAOneShotRun is the one-shot counterpart of the two
// tests above: a command given on govax's command line (here, RUN of a
// program that loops forever) runs from inside startup.com, through
// INCLUDE/COMMAND_LINE, so the limits have to be applied there too, not
// only before the interactive prompt. A one-shot run a limit stops has
// failed: run returns the limit error, and govax exits with
// limitExitStatus. Each limit is tried on its own, and
// the boot itself, which runs the microkernel's initialization, mustn't
// be cut short by them (the run would never start if it were).
func TestRun_limitsStopAOneShotRun(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "spin.mar")

	// A program whose main routine branches to itself forever.
	if err := os.WriteFile(src, []byte("\t.PSECT\tC,NOWRT,EXE\n\t.ENTRY\tGO,^M<>\n10$:\tBRB\t10$\n\t.END\tGO\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{macroCommand(src, macroFlags{}), linkCommand([]string{filepath.Join(dir, "spin")}, linkFlags{})} {
		var buf bytes.Buffer
		if err := run(nil, 0, 0, &buf, emptyStdin(), []string{command}); err != nil {
			t.Fatalf("%s: %v\n%s", command, err, buf.String())
		}
	}

	runSpin := "RUN " + dclQuote(filepath.Join(dir, "spin.exe"))

	for _, tc := range []struct {
		name         string
		instructions int
		duration     time.Duration
		want         string
	}{
		{"instruction limit", 100_000, 0, "INSTRLIMIT"},
		{"time limit", 0, 50 * time.Millisecond, "TIMELIMIT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			// run returns once the limit stops the program; without the
			// limits applied it never would, and the test would time out.
			err := run(nil, tc.instructions, tc.duration, &buf, emptyStdin(), []string{runSpin})

			if strings.Count(buf.String(), tc.want) != 1 {
				t.Errorf("output = %q, want one %s message", buf.String(), tc.want)
			}

			// The one-shot command failed, so govax exits with
			// limitExitStatus; the message isn't shown again.
			if !isLimitStop(err) {
				t.Errorf("run = %v, want a limit stop (exit status %d)", err, limitExitStatus)
			}

			if !vmserrors.MessageInhibited(err) {
				t.Errorf("run = %v, want its message marked as already shown", err)
			}
		})
	}
}

// TestRun_pathOverridesEmbeddedForThatFileOnly exercises the per-file search
// order docs/planning/PHASE-15.md describes: a -path directory holding only a
// customized startup.com is used for startup.com, while console.dcl
// still falls through to the embedded copies (never named on
// the command line) — a full replacement set isn't required to override
// one file. With no system volume (this package's tests have none unless
// they ask, TestMain) the startup procedure is found this way.
func TestRun_pathOverridesEmbeddedForThatFileOnly(t *testing.T) {
	dir := t.TempDir()
	custom := `$ print "custom-startup-ran"` + "\n"

	if err := os.WriteFile(filepath.Join(dir, "startup.com"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := run([]string{dir}, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(buf.String(), "custom-startup-ran") {
		t.Errorf("output = %q, want the overriding startup.com's own PRINT output", buf.String())
	}
}

// TestRun_asGivenPathWinsOverPathFlag confirms a name that already resolves
// as given (relative to the working directory) is never overridden by a
// -path directory — the "as-given" leg of the search always runs first.
// (The startup procedure with no system volume, as above.)
func TestRun_asGivenPathWinsOverPathFlag(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	asGiven := filepath.Join(wd, "startup.com")
	if err := os.WriteFile(asGiven, []byte(`$ print "as-given-startup-ran"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.Remove(asGiven) })

	pathDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pathDir, "startup.com"), []byte(`$ print "path-flag-startup-ran"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := run([]string{pathDir}, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(buf.String(), "as-given-startup-ran") {
		t.Errorf("output = %q, want the CWD-relative startup.com's own PRINT output", buf.String())
	}
}

// TestRun_interactiveAsmRepl exercises docs/planning/PHASE-19.md's own interactive
// "ASM" mode (a debugger command once the microkernel is in place) end to end through the real readline loop: a bare ASM enters
// assembler mode, several lines are typed one at a time (matching what a
// real terminal session would feed the readline loop), and "END <entry>"
// both exits the mode and auto-CALLs the routine just typed -- confirmed by
// EXAMINE-ing the register it set afterward from the debugger.
func TestRun_interactiveAsmRepl(t *testing.T) {
	script := strings.Join([]string{
		"DEBUG",
		"ASM",
		".ENTRY MYTEST,^M<>",
		"MOVL #42,R0",
		"RET",
		"END MYTEST",
		"EXAMINE R0",
	}, "\n") + "\n"

	in := io.NopCloser(strings.NewReader(script))

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, in, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	// #42 is decimal, the assembler's default radix as in MACRO-32; the
	// debugger's EXAMINE shows it in hex.
	if !strings.Contains(buf.String(), "0000002A") {
		t.Errorf("output = %q, want R0 = 0000002A from the auto-CALLed routine", buf.String())
	}
}

// TestRun_bareAsmStartsAtX200 checks that, with startup.com no longer setting
// PC=200 (docs/planning/PHASE-42.md), a bare ASM after boot still assembles at
// X^200: statements typed with no .ORG land there, which the debugger's
// EXAMINE/INSTRUCTION then shows.
func TestRun_bareAsmStartsAtX200(t *testing.T) {
	script := strings.Join([]string{
		"DEBUG",
		"ASM",
		"MOVL #42,R0",
		"HALT",
		"END",
		"EXAMINE/INSTRUCTION 200",
	}, "\n") + "\n"

	in := io.NopCloser(strings.NewReader(script))

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, in, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(buf.String(), "MOVL") || !strings.Contains(buf.String(), "R0") {
		t.Errorf("output = %q, want the MOVL assembled at 200", buf.String())
	}
}

// TestReportFailure_shownOnce runs a one-shot "govax run" of a program
// that fails with an access violation: the console shows the
// %SYSTEM-F-ACCVIO message as the image ends, and govax must then exit
// nonzero without showing the failure again (it once added a second
// line, "govax: SYSTEM-F-ACCVIO, ..., reason mask=!XB, ...", from the
// message's unformatted text).
func TestReportFailure_shownOnce(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "crash.mar")

	program := "\t.ENTRY\tCRASH, ^M<>\n\tCLRL\tR1\n\tMOVL\t(R1), R0\n\tRET\n\t.END\tCRASH\n"
	if err := os.WriteFile(source, []byte(program), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	for _, command := range []string{
		macroCommand(source, macroFlags{}),
		linkCommand([]string{filepath.Join(dir, "crash.obj")}, linkFlags{}),
	} {
		var out bytes.Buffer
		if err := run(nil, 0, 0, &out, emptyStdin(), []string{command}); err != nil {
			t.Fatalf("%s: %v\n%s", command, err, out.String())
		}
	}

	var out bytes.Buffer

	err := run(nil, 0, 0, &out, emptyStdin(), []string{runCommand(filepath.Join(dir, "crash.exe"), runFlags{})})
	if err == nil {
		t.Fatalf("run of a failing image = nil error, want its failure\n%s", out.String())
	}

	if got := strings.Count(out.String(), "ACCVIO"); got != 1 {
		t.Errorf("console showed ACCVIO %d times, want once:\n%s", got, out.String())
	}

	var report bytes.Buffer
	if status := reportFailure(&report, err); status != 1 {
		t.Errorf("reportFailure status = %d, want 1", status)
	}

	if report.Len() != 0 {
		t.Errorf("reportFailure showed the failure again: %q", report.String())
	}

	// A failure the console hasn't shown is still reported.
	report.Reset()

	if status := reportFailure(&report, vmserrors.New(vmserrors.CLI_NOFILE)); status != 1 || report.Len() == 0 {
		t.Errorf("reportFailure of an unshown failure = %d, %q; want 1 and its message", status, report.String())
	}
}
