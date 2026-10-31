package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// TestFortranIOSignals checks the I/O conditions a statement with none of
// ERR=, END=, and IOSTAT= can't take: FTNRTL signals each as VMS's FORTRAN
// run-time library does (testdata/for80's run 3, cases I and J), with the
// unit, an empty file name, and the user PC, and the severe ones end the
// image with the condition as its status, message inhibited. What runs
// before the statement has run; what comes after doesn't.
func TestFortranIOSignals(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	for _, tc := range []struct {
		name, statement, want, status string
	}{
		{"INVLOGUNI", "WRITE (1000, *) 1", "%FOR-F-INVLOGUNI, invalid logical unit number\n  unit 1000  file \n", "10188104"},
		{"RECIO_OPE", "WRITE (6, *) 'OUTER', INNER(6)", "%FOR-F-RECIO_OPE, recursive I/O operation\n  unit 6  file \n", "10188144"},
		{"NOTIMP", "PRINT 10, 1", "%FOR-F-NOTIMP, not implemented\n", "10188014"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{
				"      PROGRAM SIGNAL",
				"   10 FORMAT (1X, I5)",
				"      PRINT *, 'BEFORE'",
				"      " + tc.statement,
				"      PRINT *, 'AFTER'",
				"      END",
				"      INTEGER FUNCTION INNER(IU)",
				"      WRITE (IU, *) 'INNER'",
				"      INNER = IU",
				"      END",
			}

			if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]SIGNAL.FOR"},
				rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
				t.Fatal(err)
			}

			deleteFilesAfter(t, d, "SIGNAL")

			buf.Reset()

			for _, command := range []string{"FORTRAN SIGNAL", "MACRO SIGNAL", "LINK SIGNAL"} {
				shareFindRun(t, d, command)

				if out := buf.String(); out != "" {
					t.Fatalf("%s said:\n%s", command, out)
				}
			}

			// A message's lines after its first end as $FAO's !/ ends them,
			// with a carriage return and a line feed.
			out := strings.ReplaceAll(shareRun(t, d, buf, "SIGNAL"), "\r\n", "\n")

			if !strings.HasPrefix(out, "BEFORE\n"+tc.want+"  user PC ") || strings.Contains(out, "AFTER") ||
				!strings.Contains(out, "%TRACE-F-TRACEBACK") || !strings.HasSuffix(out, `$STATUS == "%X`+tc.status+`"`+"\n") {
				t.Errorf("RUN SIGNAL printed:\n%q", out)
			}
		})
	}
}
