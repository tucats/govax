package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/ftnrtl"
	"github.com/tucats/govax/internal/sysvol"
)

// The one-shot subcommands with a system volume (docs/PHASE-87 - system
// volume.md, subtask 9): govax macro, fortran, link, run, library, and
// analyze each start govax, mount the system volume, and run STARTUP.COM,
// whose SYS$LIBRARY and SYS$SHARE lead them to the volume's macro library,
// FORSYSDEF.TLB, and FTNRTL.EXE.

// ftnVersionCaller writes FTN$VERSION's text, then ends with $EXIT_S, a
// macro from the system macro library.
const ftnVersionCaller = `	.TITLE	FTNVER
	.PSECT	DATA,WRT,NOEXE,LONG
TEXT:	.BLKB	11
DESC:	.WORD	11
	.BYTE	14,1
	.ADDRESS TEXT
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	PUSHAQ	DESC
	CALLS	#1,G^FTN$VERSION
	PUSHAQ	DESC
	CALLS	#1,G^LIB$PUT_OUTPUT
	$EXIT_S
	.END	START
`

// libraryUser includes a module of the system volume's FORSYSDEF.TLB and
// one put there by the library subcommand.
const libraryUser = `      PROGRAM LIBUSE
      INCLUDE '($SSDEF)'
      INCLUDE '(GOVAX_TEST)'
      PRINT *, SS$_NORMAL, GOVAX_ANSWER
      END
`

// hostFile writes text to name in dir and returns its path.
func hostFile(t *testing.T, dir, name, text string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

// quietOnce runs command once, failing t if it says anything.
func quietOnce(t *testing.T, command string) {
	t.Helper()

	if out := runOnce(t, nil, command); out != "" {
		t.Fatalf("%s:\n%s", command, out)
	}
}

// TestRun_oneShotsOnSystemVolume runs each subcommand's command as a
// one-shot command on a system volume made by the first of them (timed):
// a MACRO program linked against the volume's FTNRTL, as the full map shows,
// and run; FTNRTL rebuilt from an edited source by BUILDFTN.COM, which the
// program then runs with; a module added to the volume's FORSYSDEF.TLB by
// the library subcommand and included by a FORTRAN program; and ANALYZE
// of an image.
func TestRun_oneShotsOnSystemVolume(t *testing.T) {
	system := filepath.Join(t.TempDir(), "govax-system.dsk")
	withSystemVolume(t, systemFlags{file: system})

	dir := t.TempDir()
	src := hostFile(t, dir, "ftnver.mar", ftnVersionCaller)
	prog := filepath.Join(dir, "ftnver")

	// The first start makes the volume, the default RD54, quietly, as a
	// one-shot command does.
	start := time.Now()

	quietOnce(t, macroCommand(src, macroFlags{}))
	t.Logf("first start, making an RD54 system volume, with a MACRO one-shot: %v", time.Since(start))

	if _, err := os.Stat(system); err != nil {
		t.Fatalf("no system volume: %v", err)
	}

	quietOnce(t, linkCommand([]string{prog}, linkFlags{mapWanted: true, full: true}))

	mapText, err := os.ReadFile(prog + ".map")
	if err != nil {
		t.Fatal(err)
	}

	if want := "SYS$SYSROOT:[SYSLIB]FTNRTL.EXE;1"; !strings.Contains(string(mapText), want) {
		t.Errorf("the map doesn't name %s:\n%s", want, mapText)
	}

	if got, want := runOnce(t, nil, runCommand(prog+".exe", runFlags{})), "FTNRTL V1.0\n"; got != want {
		t.Errorf("run: %q, want %q", got, want)
	}

	// FTNRTL rebuilt from an edited FTN_CONTROL.MAR, as a new version in
	// SYS$SHARE, which the same image runs with.
	control, _ := sysvol.Lookup(sysvol.Sources + "FTN_CONTROL.MAR")

	text, err := control.Contents()
	if err != nil {
		t.Fatal(err)
	}

	edited := strings.Replace(string(text), "/FTNRTL V1.0/", "/FTNRTL V9.9/", 1)
	if edited == string(text) {
		t.Fatalf("%s has no version text to edit", ftnrtl.Name)
	}

	putOnSystemVolume(t, control.Spec(), edited)

	if out := runOnce(t, nil, "@SYS$SOURCES:BUILDFTN"); !strings.HasSuffix(out, "FTNRTL.EXE built and installed in SYS$SHARE\n") {
		t.Fatalf("BUILDFTN:\n%s", out)
	}

	if got, want := runOnce(t, nil, runCommand(prog+".exe", runFlags{})), "FTNRTL V9.9\n"; got != want {
		t.Errorf("run with the rebuilt FTNRTL: %q, want %q", got, want)
	}

	// The library subcommand on the volume's FORSYSDEF.TLB: a text module,
	// named for its file, that FORTRAN's INCLUDE then finds there.
	module := hostFile(t, dir, "govax_test.txt", "      INTEGER GOVAX_ANSWER\n      PARAMETER (GOVAX_ANSWER = 42)\n")
	quietOnce(t, libraryCommand("DUA0:[SYSTEM.SYSLIB]FORSYSDEF.TLB", []string{module}, libraryFlags{text: true}))

	fortran := filepath.Join(dir, "libuse")
	hostFile(t, dir, "libuse.for", libraryUser)

	quietOnce(t, fortranCommand([]string{fortran + ".for"}, fortranFlags{}))
	quietOnce(t, macroCommand(fortran+".mar", macroFlags{}))
	quietOnce(t, linkCommand([]string{fortran}, linkFlags{}))

	if got, want := runOnce(t, nil, runCommand(fortran+".exe", runFlags{})), "          1          42\n"; got != want {
		t.Errorf("run LIBUSE: %q, want %q", got, want)
	}

	// ANALYZE/IMAGE of the program names FTNRTL among its shareable images.
	command, err := analyzeCommand([]string{fortran + ".exe"}, analyzeFlags{image: true})
	if err != nil {
		t.Fatal(err)
	}

	if report := runOnce(t, nil, command); !strings.Contains(report, `1)  "`+ftnrtl.Name+`"`) {
		t.Errorf("ANALYZE/IMAGE's report doesn't list %s:\n%s", ftnrtl.Name, report)
	}
}
