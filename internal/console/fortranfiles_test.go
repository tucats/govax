package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// fortranBuild compiles, assembles, and links a program of these lines on the
// volume, each of the tools saying nothing.
func fortranBuild(t *testing.T, d *Dispatcher, c *Console, buf *bytes.Buffer, name string, lines []string) {
	t.Helper()

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name + ".FOR"},
		rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
		t.Fatal(err)
	}

	deleteFilesAfter(t, d, name)

	for _, command := range []string{"FORTRAN ", "MACRO ", "LINK "} {
		if err := d.Dispatch(command + name); err != nil {
			t.Fatalf("%s%s: %v\n%s", command, name, err, buf.String())
		}
	}
}

// TestFortranFiles is what testdata/fortran/files.for can't see of itself:
// image exit closes each unit by its disposition (docs/PHASE-81 - FORTRAN
// files.md, Decision 12, as testdata/for81's FOR81X showed), keeping an
// ordinary file and a unit with no OPEN, with their records, and removing
// DISPOSE='DELETE' and STATUS='SCRATCH' files.
func TestFortranFiles(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	fortranBuild(t, d, c, buf, "EXITS", []string{
		"      PROGRAM EXITS",
		"      OPEN (9, FILE='EXITKEEP.DAT', STATUS='NEW')",
		"      WRITE (9, '(A)') 'KEPT'",
		"      OPEN (10, FILE='EXITDEL.DAT', STATUS='NEW', DISPOSE='DELETE')",
		"      WRITE (10, '(A)') 'NOT KEPT'",
		"      OPEN (11, FILE='EXITSCR.DAT', STATUS='SCRATCH')",
		"      WRITE (12, '(A)') 'IMPLICIT'",
		"      END",
	})
	deleteFilesAfter(t, d, "EXITKEEP")
	deleteFilesAfter(t, d, "FOR012")

	buf.Reset()

	if out := shareRun(t, d, buf, "EXITS"); !strings.HasSuffix(out, `$STATUS == "%X00000001"`+"\n") {
		t.Fatalf("RUN EXITS printed:\n%s", out)
	}

	buf.Reset()
	shareFindRun(t, d, "TYPE EXITKEEP.DAT")
	shareFindRun(t, d, "TYPE FOR012.DAT")

	if out := buf.String(); !strings.Contains(out, "KEPT") || !strings.Contains(out, "IMPLICIT") {
		t.Errorf("the files kept hold:\n%s", out)
	}

	for _, name := range []string{"EXITDEL.DAT", "EXITSCR.DAT"} {
		if _, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords); err == nil {
			t.Errorf("%s is still there", name)
		}
	}
}

// TestFortranOpenSignals checks a file statement with no ERR= or IOSTAT=: its
// failure is signaled, a severe condition ending the image, with the unit, the
// name RMS got as far as, the user PC, and RMS's own status (testdata/for81's
// #371 and #373; the name's text is unconfirmed, the probe showing only the
// signal array's size).
func TestFortranOpenSignals(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	for _, tc := range []struct {
		name, statement, want, status string
	}{
		{"FILNOTFOU", "OPEN (10, FILE='NOSUCH.DAT', STATUS='OLD')",
			"%FOR-F-FILNOTFOU, file not found\n  unit 10  file DUA1:[000000]NOSUCH.DAT;\n", "101880EC"},
		{"INVLOGUNI", "OPEN (120, FILE='X.DAT')", "%FOR-F-INVLOGUNI, invalid logical unit number\n  unit 120  file \n", "10188104"},
		{"CLOSE", "CLOSE (-5)", "%FOR-F-INVLOGUNI, invalid logical unit number\n  unit -5  file \n", "10188104"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fortranBuild(t, d, c, buf, "SIGNAL", []string{
				"      PROGRAM SIGNAL",
				"      PRINT *, 'BEFORE'",
				"      " + tc.statement,
				"      PRINT *, 'AFTER'",
				"      END",
			})

			buf.Reset()

			out := strings.ReplaceAll(shareRun(t, d, buf, "SIGNAL"), "\r\n", "\n")
			if !strings.HasPrefix(out, "BEFORE\n"+tc.want+"  user PC ") || strings.Contains(out, "AFTER") ||
				!strings.HasSuffix(out, `$STATUS == "%X`+tc.status+`"`+"\n") {
				t.Errorf("RUN SIGNAL printed:\n%q", out)
			}
		})
	}

	if t.Failed() {
		return
	}

	// RMS's status follows the message for a failure RMS reported.
	fortranBuild(t, d, c, buf, "SIGNAL", []string{
		"      PROGRAM SIGNAL",
		"      OPEN (10, FILE='NOSUCH.DAT', STATUS='OLD')",
		"      END",
	})
	buf.Reset()

	if out := shareRun(t, d, buf, "SIGNAL"); !strings.Contains(out, "-RMS-E-FNF, file not found") {
		t.Errorf("RUN SIGNAL printed:\n%s", out)
	}
}
