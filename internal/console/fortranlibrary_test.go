package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// fortranLibraryRun runs lines, failing on an error, and returns what the last
// of them wrote.
func fortranLibraryRun(t *testing.T, d *Dispatcher, buf *bytes.Buffer, lines ...string) string {
	t.Helper()

	buf.Reset()

	for _, line := range lines {
		buf.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v\n%s", line, err, buf)
		}
	}

	return buf.String()
}

// fortranLibraryConsole makes a console on a volume with the files named
// (from a map of name to text) on it.
func fortranLibraryConsole(t *testing.T, files map[string]string) (*Dispatcher, *bytes.Buffer) {
	t.Helper()

	d, c, buf := shareFindConsole(t, false)

	for name, text := range files {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name},
			rms.TextRecords, splitSource(text)); err != nil {
			t.Fatal(err)
		}
	}

	return d, buf
}

// fortranLibraryFiles are a program in two files, joined by +, whose INCLUDE
// statements name modules of two libraries, and a module in each of them that
// the other has too, to show which is searched first.
var fortranLibraryFiles = map[string]string{
	"MAIN.FOR": "      PROGRAM MAIN\n" +
		"      INCLUDE '(ONE)'\n" +
		"      INCLUDE '(WHICH)'\n" +
		"      INCLUDE 'OTHER(WHICH)'\n" +
		"      PRINT *, ONE, WHICH, OTHER\n",
	"REST.FOR":  "      PRINT *, 'REST'\n      END\n",
	"ONE.TXT":   "      INTEGER ONE\n      PARAMETER (ONE = 1)\n",
	"WHICH.TXT": "      INTEGER WHICH\n      PARAMETER (WHICH = 2)\n",
	"OTHER.TXT": "      INTEGER OTHER\n      PARAMETER (OTHER = 3)\n",
}

// fortranLibrarySetup makes MINE.TLB, of modules ONE and WHICH, and OTHER.TLB,
// whose WHICH is the text of OTHER.TXT.
var fortranLibrarySetup = []string{
	"LIBRARY/CREATE/TEXT MINE ONE,WHICH",
	"LIBRARY/CREATE/TEXT OTHER OTHER.TXT/MODULE=WHICH",
}

// TestFortranLibrary is INCLUDE '(module)' and 'lib(module)', + and /LIBRARY
// through the command's grammar, to a program that runs.
func TestFortranLibrary(t *testing.T) {
	d, buf := fortranLibraryConsole(t, fortranLibraryFiles)

	out := fortranLibraryRun(t, d, buf, append(fortranLibrarySetup,
		"FORTRAN MAIN+REST+MINE/LIBRARY", "MACRO MAIN", "LINK MAIN", "RUN MAIN")...)

	if !strings.Contains(out, "1           2           3") || !strings.Contains(out, "REST") {
		t.Errorf("the program wrote %q", out)
	}

	// The MACRO file names each file and module, without their lines.
	out = fortranLibraryRun(t, d, buf, "TYPE MAIN.MAR")

	for _, want := range []string{
		"; from DUA1:[000000]MAIN.FOR;1\n; from DUA1:[000000]REST.FOR;1\n",
		"; INCLUDE file DUA1:[000000]MINE.TLB;1(ONE):\n; end of DUA1:[000000]MINE.TLB;1(ONE)\n",
		"; INCLUDE file DUA1:[000000]OTHER.TLB;1(WHICH):\n",
		"; source file DUA1:[000000]REST.FOR;1:\n",
	} {
		if !strings.Contains(strings.ReplaceAll(out, "\r", ""), want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}

	if strings.Contains(out, "PARAMETER (ONE") {
		t.Errorf("a module's lines are in the MACRO file:\n%s", out)
	}
}

// TestFortranLibraryLogical checks the order of the search: /LIBRARY's, then
// FORT$LIBRARY's.
func TestFortranLibraryLogical(t *testing.T) {
	d, buf := fortranLibraryConsole(t, map[string]string{
		"TWO.FOR":   "      INCLUDE '(WHICH)'\n      PRINT *, WHICH\n      END\n",
		"WHICH.TXT": fortranLibraryFiles["WHICH.TXT"],
		"THREE.TXT": "      INTEGER WHICH\n      PARAMETER (WHICH = 33)\n",
	})

	out := fortranLibraryRun(t, d, buf,
		"LIBRARY/CREATE/TEXT MINE WHICH",
		"LIBRARY/CREATE/TEXT THEIRS THREE.TXT/MODULE=WHICH",
		"DEFINE FORT$LIBRARY DUA1:[000000]THEIRS",
		"FORTRAN TWO", "MACRO TWO", "LINK TWO", "RUN TWO")

	if !strings.Contains(out, "33") {
		t.Errorf("FORT$LIBRARY's module: the program wrote %q", out)
	}

	out = fortranLibraryRun(t, d, buf, "FORTRAN TWO+MINE/LIBRARY", "MACRO TWO", "LINK TWO", "RUN TWO")

	if strings.Contains(out, "33") || !strings.Contains(out, "2") {
		t.Errorf("/LIBRARY comes first: the program wrote %q", out)
	}
}

// TestFortranLibrarySysDef finds FORSYSDEF.TLB through SYS$LIBRARY, last, and a
// library of VMS's own making.
func TestFortranLibrarySysDef(t *testing.T) {
	d, buf := fortranLibraryConsole(t, map[string]string{
		"V.FOR": "      INCLUDE '(TXA)'\n      PRINT *, A\n      END\n",
	})

	vax, err := filepath.Abs(for83Dir + "vax/s7.tlb")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(vax); err != nil {
		t.Fatal(err)
	}

	out := fortranLibraryRun(t, d, buf,
		`COPY "`+vax+`"/HOST FORSYSDEF.TLB`,
		"DEFINE SYS$LIBRARY DUA1:[000000]",
		"FORTRAN V", "MACRO V", "LINK V", "RUN V")

	if !strings.Contains(out, " 1") {
		t.Errorf("the program wrote %q", out)
	}
}

// TestFortranLibraryErrors checks a library or module that can't be used.
func TestFortranLibraryErrors(t *testing.T) {
	d, buf := fortranLibraryConsole(t, map[string]string{
		"M.FOR":   "      INCLUDE '(NOSUCH)'\n      END\n",
		"N.FOR":   "      INCLUDE 'THE(NOSUCH)'\n      END\n",
		"O.FOR":   "      END\n",
		"THE.TXT": "C\n",
		"X.MAR":   "\t.MACRO\tX\n\t.ENDM\tX\n",
	})

	fortranLibraryRun(t, d, buf, "LIBRARY/CREATE/TEXT THE THE", "LIBRARY/CREATE/MACRO MAC X")

	cases := []struct{ line, want string }{
		{"FORTRAN M+THE/LIBRARY", "no library has a module NOSUCH"},
		{"FORTRAN N", "the library has no module NOSUCH"},
		{"FORTRAN O+NONE/LIBRARY", "NONE"},
		{"FORTRAN O+MAC.MLB/LIBRARY", "not a text library"},
		{"FORTRAN THE/LIBRARY", "no source file"},
	}

	for _, tc := range cases {
		buf.Reset()

		err := d.Dispatch(tc.line)
		if err == nil {
			t.Errorf("%s: succeeded", tc.line)

			continue
		}

		text := buf.String()
		if !vmserrors.MessageInhibited(err) {
			text += err.Error()
		}

		if !strings.Contains(text, tc.want) {
			t.Errorf("%s: %q, want %q", tc.line, text, tc.want)
		}
	}
}

// TestLibraryTextLog checks /LOG's facility, which the rest of LIBRARY shares.
func TestLibraryTextLog(t *testing.T) {
	d, buf := fortranLibraryConsole(t, map[string]string{"A.TXT": "X\n"})

	out := fortranLibraryRun(t, d, buf, "LIBRARY/CREATE/TEXT/LOG L A")

	if !strings.Contains(out, "%LIBRAR-S-INSERTED, module A inserted in DUA1:[000000]L.TLB;1") {
		t.Errorf("%q", out)
	}
}
