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

// TestFortranFormattedInput reads the terminal through formats (Phase 80,
// subtask 4): one record at a statement's start and another at each slash
// and at reversion; T, TL, and TR in the record; A of a field shorter and
// longer than its item; Q, the characters left; an H constant read into
// its FORMAT, which a later WRITE writes; a run-time format; and the end
// of the input, END=, then IOSTAT's -1. A record of 200 characters is too
// long (FOR$_INPRECTOO, 22: VMS reads at most 199 from SYS$INPUT).
func TestFortranFormattedInput(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		lines := []string{
			"      PROGRAM FIN",
			"      CHARACTER*3 A, B",
			"      CHARACTER*1 C1, C2",
			"      CHARACTER*6 LONG",
			"      CHARACTER*8 RTF",
			"      INTEGER N, IOS",
			"   10 FORMAT (T7,A3,T1,A3)",
			"   20 FORMAT (5HXXXXX)",
			"   30 FORMAT (A3,Q)",
			"      READ (5,10) A, B",
			"      PRINT *, '|', A, '|', B, '|'",
			"      READ (5,'(TR2,A1,TL2,A1)') C1, C2",
			"      PRINT *, C1, C2",
			"      READ (5,30) A, N",
			"      PRINT *, A, N",
			"      READ (5,20)",
			"      WRITE (6,20)",
			"      READ (5,'(A2,/,A6)') A, LONG",
			"      PRINT *, '|', A, '|', LONG, '|'",
			"      RTF = '(A1)'",
			"      READ (5,RTF) A, B",
			"      PRINT *, '|', A, '|', B, '|'",
			"      READ (5,'(A3)', IOSTAT=IOS) A",
			"      PRINT *, 'LONG RECORD', IOS",
			"      READ (5,'(A3)', END=40) A",
			"      PRINT *, 'NOT HERE'",
			"   40 READ (5,'(A3)', IOSTAT=IOS) A",
			"      PRINT *, 'END', IOS",
			"      END",
		}
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]FIN.FOR"},
			rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		deleteFilesAfter(t, d, "FIN")

		for _, command := range []string{"FORTRAN FIN", "MACRO FIN", "LINK FIN"} {
			shareFindRun(t, d, command)

			if out := buf.String(); out != "" {
				t.Fatalf("%s said:\n%s", command, out)
			}
		}

		c.In = strings.NewReader("ABC   XYZ\nABCDE\nHELLO\nHI THERE\nWXYZ\nABCDEFGH\nPQ\nRS\n" +
			strings.Repeat("L", 200) + "\n")

		want := "|XYZ|ABC|\nCB\nHEL           2\nI TH\n|WX |ABCDEF|\n|P  |R  |\nLONG RECORD          22\nEND          -1\n" +
			"  $STATUS == \"%X00000001\"\n"
		if got := shareRun(t, d, buf, "FIN"); got != want {
			t.Errorf("RUN FIN printed:\n%s\nwant:\n%s", got, want)
		}
	})
}
