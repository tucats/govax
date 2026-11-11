package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
)

// The system volume's STARTUP.COM, logical names, and grammars, end to end
// (docs/planning/PHASE-87 - system volume.md, subtask 5).

// runOnce runs govax once with a one-shot command (none for an interactive
// start with no input) and returns its output.
func runOnce(t *testing.T, paths []string, command ...string) string {
	t.Helper()

	var buf bytes.Buffer
	if err := run(paths, 0, 0, &buf, emptyStdin(), command); err != nil {
		t.Fatalf("run %q: %v\n%s", command, err, buf.String())
	}

	return buf.String()
}

// putOnSystemVolume copies text to spec on the system volume (DUA0:), as a
// user's COPY of a host file would, through a run of its own.
func putOnSystemVolume(t *testing.T, spec, text string) {
	t.Helper()

	host := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(host, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	if out := runOnce(t, nil, `COPY "`+host+`" DUA0:`+spec); strings.Contains(out, "-E-") || strings.Contains(out, "-F-") {
		t.Fatalf("COPY to %s:\n%s", spec, out)
	}
}

// builtin returns one of govax's built-in files, its lines ended by line
// feeds.
func builtin(t *testing.T, name string) string {
	t.Helper()

	data, err := fs.ReadFile(bootdata.FS, name)
	if err != nil {
		t.Fatal(err)
	}

	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// TestRun_systemVolumeStartup checks the startup procedure with a system
// volume: the volume's STARTUP.COM runs, never a host startup.com, and
// defines the system logical names from govax's two; and an edited
// STARTUP.COM on the volume is the one that runs, booting as govax's own
// does (its data lines reach the debugger after its VMINIT).
func TestRun_systemVolumeStartup(t *testing.T) {
	withSystemVolume(t, systemFlags{file: filepath.Join(t.TempDir(), "govax-system.dsk")})
	runOnce(t, nil, `WRITE SYS$OUTPUT "made"`) // makes the volume

	hostDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostDir, "startup.com"), []byte(`$ write sys$output "host startup ran"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runOnce(t, []string{hostDir},
		`WRITE SYS$OUTPUT F$TRNLNM("SYS$SYSDEVICE"), " ", F$TRNLNM("SYS$SYSROOT"), " ", F$TRNLNM("SYS$LIBRARY")`)

	if strings.Contains(out, "host startup ran") {
		t.Errorf("a host startup.com ran with a system volume:\n%s", out)
	}

	if want := "_DUA0: _DUA0:[SYSTEM.] SYS$SYSROOT:[SYSLIB]\n"; !strings.Contains(out, want) {
		t.Errorf("output lacks %q:\n%s", want, out)
	}

	// An edited STARTUP.COM: govax's, with a line of the user's at its end.
	putOnSystemVolume(t, "[SYSTEM.SYSMGR]STARTUP.COM", builtin(t, "startup.com")+`$ write sys$output "edited startup ran"`+"\n")

	out = runOnce(t, nil, `WRITE SYS$OUTPUT "then the command"`)

	if !strings.Contains(out, "edited startup ran\nthen the command\n") {
		t.Errorf("the edited STARTUP.COM didn't run, or the command after it didn't:\n%s", out)
	}

	if strings.Contains(out, "SYSKEPT") {
		t.Errorf("govax's copy hasn't changed, so nothing should be said:\n%s", out)
	}
}

// TestRun_systemVolumeKernel checks the microkernel and ROM STARTUP.COM
// loads (subtask 6): the volume's with a system volume, an edited
// KERNEL.ASM the one that boots; govax's built-in ones without.
func TestRun_systemVolumeKernel(t *testing.T) {
	out := runOnce(t, nil, "SHOW ROM")
	if want := `ROM FILE="default.rom"`; !strings.Contains(out, want) {
		t.Errorf("with no system volume, output lacks %s:\n%s", want, out)
	}

	withSystemVolume(t, systemFlags{file: filepath.Join(t.TempDir(), "govax-system.dsk")})

	out = runOnce(t, nil, "SHOW ROM")
	if want := `ROM FILE="SYS$SYSROOT:[SYSEXE]DEFAULT.ROM;1"`; !strings.Contains(out, want) {
		t.Errorf("with a system volume, output lacks %s:\n%s", want, out)
	}

	const message = "%SYSTEM-S-INIT, Kernel initialized"

	kernel := builtin(t, "kernel.asm")
	if !strings.Contains(kernel, message) {
		t.Fatal("kernel.asm isn't as this test expects")
	}

	putOnSystemVolume(t, "[SYSTEM.SOURCES]KERNEL.ASM", strings.Replace(kernel, message, "%SYSTEM-S-INIT, Your kernel initialized", 1))

	out = runOnce(t, nil)
	if !strings.Contains(out, "%SYSTEM-S-INIT, Your kernel initialized\n") {
		t.Errorf("the edited kernel didn't boot:\n%s", out)
	}
}

// TestRun_systemVolumeGrammar checks the console's grammar on the system
// volume: one that doesn't parse, and one that lacks a command govax has,
// are each a warning, and govax's own grammar is used, so the command
// still runs; a good edited one is used, its change seen.
func TestRun_systemVolumeGrammar(t *testing.T) {
	withSystemVolume(t, systemFlags{file: filepath.Join(t.TempDir(), "govax-system.dsk")})
	runOnce(t, nil, `WRITE SYS$OUTPUT "made"`)

	const spec = "[SYSTEM.SYSMGR]CONSOLE.DCL"

	grammar := builtin(t, "console.dcl")
	warning := "%VAX-W-SYSGRAMMAR, DUA0:" + spec + " on the system volume not used; govax's own instead"

	// Doesn't parse.
	putOnSystemVolume(t, spec, "grammar console\n    verb\n")

	out := runOnce(t, nil, `WRITE SYS$OUTPUT "still works"`)
	if !strings.Contains(out, warning+": ") || !strings.Contains(out, "still works\n") {
		t.Errorf("a grammar that doesn't parse:\n%s", out)
	}

	// Lacks EXIT.
	exit := "    verb exit\n        parameter   status/id=1645              -\n                    /type=$rest_of_line\n"
	if !strings.Contains(grammar, exit) {
		t.Fatal("console.dcl's EXIT isn't as this test expects")
	}

	putOnSystemVolume(t, spec, strings.Replace(grammar, exit, "", 1))

	out = runOnce(t, nil, `WRITE SYS$OUTPUT "still works"`)
	if !strings.Contains(out, warning+": it has no EXIT\n") || !strings.Contains(out, "still works\n") {
		t.Errorf("a grammar without EXIT:\n%s", out)
	}

	// A good one, with an alias of the user's: used.
	putOnSystemVolume(t, spec, strings.Replace(grammar, "    verb exit\n", "    verb sayhi/alias=write\n\n    verb exit\n", 1))

	out = runOnce(t, nil, `SAYHI SYS$OUTPUT "hello"`)
	if strings.Contains(out, "SYSGRAMMAR") || !strings.Contains(out, "hello\n") {
		t.Errorf("an edited grammar:\n%s", out)
	}
}
