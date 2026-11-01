package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/fortran/sysdef"
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

// builtinSSDEF is a program using a definition module with no library in sight.
const builtinSSDEF = "      INCLUDE '($ssdef)'\n      PRINT *, SS$_ABORT\n      END\n"

// TestFortranBuiltinModules finds a module built into govax: in any case, with
// no lines in the MACRO file but the comments naming it.
func TestFortranBuiltinModules(t *testing.T) {
	d, buf := fortranLibraryConsole(t, map[string]string{"B.FOR": builtinSSDEF})

	out := fortranLibraryRun(t, d, buf, "FORTRAN B", "MACRO B", "LINK B", "RUN B")
	if !strings.Contains(out, " 44") {
		t.Errorf("the program wrote %q", out)
	}

	out = strings.ReplaceAll(fortranLibraryRun(t, d, buf, "TYPE B.MAR"), "\r", "")

	if want := "; INCLUDE file FORSYSDEF.TLB (built into govax)($SSDEF):\n; end of FORSYSDEF.TLB (built into govax)($SSDEF)\n"; !strings.Contains(out, want) {
		t.Errorf("no %q in:\n%s", want, out)
	}

	if strings.Contains(out, "SS$_CANCEL") {
		t.Errorf("a name the program doesn't use is in the MACRO file:\n%s", out)
	}
}

// TestFortranBuiltinModulesLast checks that a library of any kind comes before
// the built-in modules, and that 'lib(module)' doesn't reach them.
func TestFortranBuiltinModulesLast(t *testing.T) {
	d, buf := fortranLibraryConsole(t, map[string]string{
		"B.FOR":     builtinSSDEF,
		"C.FOR":     "      INCLUDE 'MINE($IODEF)'\n      END\n",
		"SSDEF.TXT": "      INTEGER SS$_ABORT\n      PARAMETER (SS$_ABORT = 7)\n",
	})

	for _, tc := range []struct {
		name  string
		lines []string
		want  string
	}{
		{"built in", []string{"FORTRAN B"}, " 44"},
		{"/LIBRARY", []string{"FORTRAN B+MINE/LIBRARY"}, " 7"},
		{"FORT$LIBRARY", []string{"DEFINE FORT$LIBRARY DUA1:[000000]MINE", "FORTRAN B"}, " 7"},
	} {
		if tc.name == "built in" {
			fortranLibraryRun(t, d, buf, "LIBRARY/CREATE/TEXT MINE SSDEF.TXT/MODULE=$SSDEF")
		}

		out := fortranLibraryRun(t, d, buf, append(tc.lines, "MACRO B", "LINK B", "RUN B")...)
		if !strings.Contains(out, tc.want+"\r") && !strings.Contains(out, tc.want+"\n") {
			t.Errorf("%s: the program wrote %q, want %q", tc.name, out, tc.want)
		}
	}

	buf.Reset()

	err := d.Dispatch("FORTRAN C")
	if err == nil || !strings.Contains(buf.String(), "the library has no module $IODEF") {
		t.Errorf("'lib(module)': %v, %q", err, buf)
	}
}

// TestFortranBuiltinLibrary reads the built-in modules as the text library
// mkforsysdef writes, with LIBRARY and as FORTRAN's own /LIBRARY.
func TestFortranBuiltinLibrary(t *testing.T) {
	d, buf := fortranLibraryConsole(t, map[string]string{"B.FOR": builtinSSDEF})

	data, err := sysdef.Library(time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	host := filepath.Join(t.TempDir(), "forsysdef.tlb")
	if err := os.WriteFile(host, data, 0o644); err != nil {
		t.Fatal(err)
	}

	quoted := `"` + host + `"`

	out := fortranLibraryRun(t, d, buf, "LIBRARY/LIST "+quoted)

	for _, want := range []string{"$SSDEF", "$SYSSRVNAM", "$CHFDEF"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in the listing:\n%s", want, out)
		}
	}

	out = fortranLibraryRun(t, d, buf, "FORTRAN B+"+quoted+"/LIBRARY", "MACRO B", "LINK B", "RUN B")
	if !strings.Contains(out, " 44") {
		t.Errorf("the program wrote %q", out)
	}

	out = fortranLibraryRun(t, d, buf, "TYPE B.MAR")
	if !strings.Contains(out, host+"($SSDEF)") {
		t.Errorf("the module isn't the library's:\n%s", out)
	}

	extracted := filepath.Join(filepath.Dir(host), "ssdef.txt")
	fortranLibraryRun(t, d, buf, `LIBRARY/TEXT/EXTRACT=$SSDEF/OUTPUT="`+extracted+`" `+quoted)

	text, err := os.ReadFile(extracted)
	if err != nil || !strings.Contains(string(text), "PARAMETER (SS$_ABORT = '0000002C'X)") {
		t.Errorf("the module's text: %v, %.200q", err, text)
	}
}

// TestFortranServiceNames checks $SYSSRVNAM's cost: the .EXTRN of a unit that
// includes it names only the service it calls.
func TestFortranServiceNames(t *testing.T) {
	d, buf := fortranLibraryConsole(t, map[string]string{
		"S.FOR": "      INCLUDE '($SYSSRVNAM)'\n      INTEGER*4 T(2), I\n      I = SYS$GETTIM(T)\n      END\n",
	})

	out := fortranLibraryRun(t, d, buf, "FORTRAN S", "TYPE S.MAR")

	if !strings.Contains(out, ".EXTRN\tSYS$GETTIM") || strings.Contains(out, "SYS$ASCTIM") {
		t.Errorf("the MACRO file:\n%s", out)
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
