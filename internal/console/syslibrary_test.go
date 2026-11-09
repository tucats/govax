package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sysvol"
)

// The libraries on the system volume (docs/PHASE-87 - system volume.md,
// subtask 8). STARTUP.COM defines SYS$LIBRARY and SYS$SHARE as the
// volume's [SYSTEM.SYSLIB], so MACRO reads its macro library there,
// FORTRAN its FORSYSDEF.TLB, and LINK and RUN its FTNRTL.EXE, ahead of the
// copies built into govax; and the build procedures in SYS$SOURCES make
// new versions of those files from the sources beside them, which are then
// the ones used.

// libraryConsole boots a console on a new system volume, with paging on or
// off, and sets the default to a directory of its own on the volume,
// DUA0:[TEST]. It returns the console and its dispatching routine
// (bootOnSystemVolume's).
func libraryConsole(t *testing.T, paging bool) (*Console, func(string) string) {
	t.Helper()

	c, _, run := bootOnSystemVolume(t, filepath.Join(t.TempDir(), "govax-system.dsk"), paging, "")

	for _, line := range []string{"CREATE/DIRECTORY SYS$SYSDEVICE:[TEST]", "SET DEFAULT SYS$SYSDEVICE:[TEST]"} {
		if out := run(line); out != "" {
			t.Fatalf("%s:\n%s", line, out)
		}
	}

	return c, run
}

// quietly runs each line, failing t if any says anything.
func quietly(t *testing.T, run func(string) string, lines ...string) {
	t.Helper()

	for _, line := range lines {
		if out := run(line); out != "" {
			t.Fatalf("%s:\n%s", line, out)
		}
	}
}

// libraryRun runs image and returns what it wrote, and $STATUS after it.
func libraryRun(run func(string) string, image string) string {
	return run("RUN "+image) + run("SHOW SYMBOL $STATUS")
}

// sourceLines are text's lines, for writeText.
func sourceLines(text string) []string {
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// readVolume reads spec's bytes, failing t if it can't.
func readVolume(t *testing.T, c *Console, spec string) []byte {
	t.Helper()

	data, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: spec})
	if err != nil {
		t.Fatalf("%s: %v", spec, err)
	}

	return data
}

// TestSystemFTNRTL: a MACRO program calling FTNRTL links and runs against
// SYS$SHARE:FTNRTL.EXE; BUILDFTN.COM builds FTNRTL from SYS$SOURCES as a
// new version there, the built-in image but for its times and linker's
// name, and cleans up after itself; the program, as linked before and
// linked again, then runs against the new version. In both paging modes.
func TestSystemFTNRTL(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		c, run := libraryConsole(t, paging)

		const (
			first  = "_DUA0:[SYSTEM.SYSLIB]FTNRTL.EXE;1"
			second = "_DUA0:[SYSTEM.SYSLIB]FTNRTL.EXE;2"
		)

		writeText(t, c, "[TEST]FTNCALL.MAR", sourceLines(ftnCaller)...)
		quietly(t, run, "MACRO FTNCALL", "LINK FTNCALL")

		if got := libraryRun(run, "FTNCALL"); got != ftnCallerRun {
			t.Errorf("FTNCALL:\n%s\nwant:\n%s", got, ftnCallerRun)
		}

		if spec := ftnrtlSpec(c); spec != first {
			t.Errorf("FTNRTL came from %q, want %q", spec, first)
		}

		out := run("@SYS$SOURCES:BUILDFTN")
		if !strings.HasSuffix(out, "FTNRTL.EXE built and installed in SYS$SHARE\n") {
			t.Fatalf("BUILDFTN:\n%s", out)
		}

		checkRebuiltFTNRTL(t, readVolume(t, c, second))

		// The objects and the image built in SYS$SOURCES are gone, and the
		// default directory is as it was.
		for _, spec := range []string{"SYS$SOURCES:*.OBJ;*", "SYS$SOURCES:*.EXE;*"} {
			if names, err := c.ContainerSession.Search(spec); err == nil && len(names) > 0 {
				t.Errorf("BUILDFTN left %v", names)
			}
		}

		if got := run("SHOW DEFAULT"); got != "  SYS$SYSDEVICE:[TEST]\n" {
			t.Errorf("after BUILDFTN, the default is %q", got)
		}

		// The image linked against version 1 runs with version 2, as the
		// image activator finds the highest version; so does one linked
		// against it.
		if got := libraryRun(run, "FTNCALL"); got != ftnCallerRun {
			t.Errorf("FTNCALL after the build:\n%s", got)
		}

		if spec := ftnrtlSpec(c); spec != second {
			t.Errorf("after the build, FTNRTL came from %q, want %q", spec, second)
		}

		quietly(t, run, "LINK FTNCALL")

		if got := libraryRun(run, "FTNCALL"); got != ftnCallerRun {
			t.Errorf("FTNCALL linked again:\n%s", got)
		}

		if spec := ftnrtlSpec(c); spec != second {
			t.Errorf("linked again, FTNRTL came from %q, want %q", spec, second)
		}
	})
}

// libraryGreeter calls two macros from the system macro library: a
// GOVAX_GREETING that only an edited library has, and $EXIT_S.
const libraryGreeter = `	.TITLE	GREET
	.PSECT	DATA,WRT,NOEXE,LONG
	GOVAX_GREETING	TEXT
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	PUSHAQ	TEXT
	CALLS	#1,G^LIB$PUT_OUTPUT
	$EXIT_S
	.END	START
`

// greetingMacro is the macro an edited STARLET.MAR adds.
const greetingMacro = `	.MACRO	GOVAX_GREETING NAME
NAME:	.ASCID	/Hello from SYS$LIBRARY/
	.ENDM	GOVAX_GREETING
`

// TestSystemMacroLibrary: BUILDSTARLET.COM builds the macro library from
// SYS$SOURCES as a new version in SYS$LIBRARY, identical to govax's
// built-in library but for its times; with a macro added to STARLET.MAR,
// the library it builds is the one MACRO uses.
func TestSystemMacroLibrary(t *testing.T) {
	c, run := libraryConsole(t, false)

	const installed = "STARLET.MLB built and installed in SYS$LIBRARY\n"

	if out := run("@SYS$SOURCES:BUILDSTARLET"); !strings.HasSuffix(out, installed) {
		t.Fatalf("BUILDSTARLET:\n%s", out)
	}

	sameButTimes(t, readVolume(t, c, "SYS$LIBRARY:STARLET.MLB;2"))

	if names, err := c.ContainerSession.Search("SYS$SOURCES:*.MLB;*"); err == nil && len(names) > 0 {
		t.Errorf("BUILDSTARLET left %v", names)
	}

	// The program's macro isn't in the library yet.
	writeText(t, c, "[TEST]GREET.MAR", sourceLines(libraryGreeter)...)

	if out := run("MACRO GREET"); !strings.Contains(out, `"GOVAX_GREETING"`) {
		t.Errorf("MACRO GREET without GOVAX_GREETING:\n%s", out)
	}

	source, _ := sysvol.Lookup(sysvol.Sources + "STARLET.MAR")
	text, err := source.Contents()
	if err != nil {
		t.Fatal(err)
	}

	writeVolumeFile(t, c, source.Spec(), append(text, greetingMacro...), sysvol.Text)

	if out := run("@SYS$SOURCES:BUILDSTARLET"); !strings.HasSuffix(out, installed) {
		t.Fatalf("BUILDSTARLET of the edited source:\n%s", out)
	}

	quietly(t, run, "MACRO GREET", "LINK GREET")

	if got, want := libraryRun(run, "GREET"), "Hello from SYS$LIBRARY\n  $STATUS == \"%X00000001\"\n"; got != want {
		t.Errorf("GREET:\n%s\nwant:\n%s", got, want)
	}
}

// sameButTimes fails t unless built, a macro library LIBRARY made from
// govax's STARLET.MAR and STARLETDEF.MAR, is byte for byte the library
// govax builds from its own copies of them at the time LIBRARY gave it:
// the library's creation and revision time, and each module's insertion,
// all one time, since LIBRARY takes the time once per command. (A time
// can't be masked byte by byte: a module's header may straddle a block,
// the next block's link word in the middle of its time.)
func sameButTimes(t *testing.T, built []byte) {
	t.Helper()

	lib, err := lbr.Open(built)
	if err != nil {
		t.Fatal(err)
	}

	src, err := bootdata.StarletSources(bootdata.FS)
	if err != nil {
		t.Fatal(err)
	}

	want, err := bootdata.BuildStarletAt(src, lib.Created)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(built, want) {
		t.Errorf("BUILDSTARLET's library (%d bytes, %d modules) isn't govax's built at its time (%d bytes)",
			len(built), lib.Modules, len(want))
	}
}

// fortranLibraryUser includes a module of govax's FORSYSDEF.TLB and one
// only an edited library has.
const fortranLibraryUser = `      PROGRAM LIBUSE
      INCLUDE '($SSDEF)'
      INCLUDE '(GOVAX_TEST)'
      PRINT *, SS$_NORMAL, GOVAX_ANSWER
      END
`

// TestSystemFortranLibrary: FORTRAN searches SYS$LIBRARY:FORSYSDEF.TLB, so
// a module added to it there is found, and the program it compiles links
// and runs against SYS$SHARE:FTNRTL.EXE.
func TestSystemFortranLibrary(t *testing.T) {
	c, run := libraryConsole(t, false)

	writeText(t, c, "[TEST]LIBUSE.FOR", sourceLines(fortranLibraryUser)...)

	if out := run("FORTRAN LIBUSE"); !strings.Contains(out, "GOVAX_TEST") {
		t.Errorf("FORTRAN without GOVAX_TEST:\n%s", out)
	}

	writeText(t, c, "[TEST]ANSWER.TXT",
		"      INTEGER GOVAX_ANSWER",
		"      PARAMETER (GOVAX_ANSWER = 42)")
	quietly(t, run, "LIBRARY/TEXT SYS$LIBRARY:FORSYSDEF.TLB ANSWER.TXT/MODULE=GOVAX_TEST")
	quietly(t, run, "FORTRAN LIBUSE", "MACRO LIBUSE", "LINK LIBUSE")

	if got, want := libraryRun(run, "LIBUSE"), "          1          42\n  $STATUS == \"%X00000001\"\n"; got != want {
		t.Errorf("LIBUSE:\n%q\nwant:\n%q", got, want)
	}

	if spec, want := ftnrtlSpec(c), "_DUA0:[SYSTEM.SYSLIB]FTNRTL.EXE;1"; spec != want {
		t.Errorf("FTNRTL came from %q, want %q", spec, want)
	}
}
