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
// before the statement has run; what comes after doesn't. An error, not
// severe, is signaled after the statement's record, and the program goes
// on (OUTCONERR, run 3's case A), ending with status 1.
func TestFortranIOSignals(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	for _, tc := range []struct {
		name, statement, want, status string
		goesOn                        bool
	}{
		{"INVLOGUNI", "WRITE (1000, *) 1", "%FOR-F-INVLOGUNI, invalid logical unit number\n  unit 1000  file \n", "10188104", false},
		{"RECIO_OPE", "WRITE (6, *) 'OUTER', INNER(6)", "%FOR-F-RECIO_OPE, recursive I/O operation\n  unit 6  file \n", "10188144", false},
		{"NOTIMP", "WRITE (LINE, *) 1", "%FOR-F-NOTIMP, not implemented\n", "10188014", false},
		{"VFEVALERR", "WRITE (6, 20) 1", "%FOR-F-VFEVALERR, variable format expression value error\n  unit 6  file \n", "10188224", false},
		{"OUTCONERR", "WRITE (6, 10) 3244", "**\n%FOR-E-OUTCONERR, output conversion error\n  unit 6  file \n", "00000001", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{
				"      PROGRAM SIGNAL",
				"      CHARACTER*8 LINE",
				"   10 FORMAT (1X, I2)",
				"   20 FORMAT (1X, I<-1>)",
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

			traceback, after := "%TRACE-F-TRACEBACK", false
			if tc.goesOn {
				traceback, after = "%TRACE-E-TRACEBACK", true
			}

			if !strings.HasPrefix(out, "BEFORE\n"+tc.want+"  user PC ") || strings.Contains(out, "AFTER\n") != after ||
				!strings.Contains(out, traceback) || !strings.HasSuffix(out, `$STATUS == "%X`+tc.status+`"`+"\n") {
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

// TestFortranVFEInput reads with variable format expressions (Phase 80,
// subtask 8): each is computed when the scan reaches it, so an item read
// earlier in the list can say how later ones are read (section 8.3.2). And a
// run-time format's H, read into, isn't copied back (section 8.7).
func TestFortranVFEInput(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		lines := []string{
			"      PROGRAM VIN",
			"      INTEGER V(3)",
			"      CHARACTER*12 S",
			"   10 FORMAT (I2, <K>I3)",
			"   20 FORMAT (I1, F<K>.1)",
			"      READ (5,10) K, (V(I), I = 1, K)",
			"      WRITE (6,'(1X,4I3)') K, V",
			"      READ (5,20) K, X",
			"      WRITE (6,'(1X,I2,F6.2)') K, X",
			"      S = '(6H ABCDE)'",
			"      READ (5,S)",
			"      WRITE (6,'(1X,A)') S",
			"      WRITE (6,S)",
			"      END",
		}
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]VIN.FOR"},
			rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		deleteFilesAfter(t, d, "VIN")

		for _, command := range []string{"FORTRAN VIN", "MACRO VIN", "LINK VIN"} {
			shareFindRun(t, d, command)

			if out := buf.String(); out != "" {
				t.Fatalf("%s said:\n%s", command, out)
			}
		}

		c.In = strings.NewReader(" 3  1  2  3\n3123\nVWXYZQ\n")

		want := "  3  1  2  3\n 3 12.30\n(6H ABCDE)  \nABCDE\n  $STATUS == \"%X00000001\"\n"
		if got := shareRun(t, d, buf, "VIN"); got != want {
			t.Errorf("RUN VIN printed:\n%s\nwant:\n%s", got, want)
		}
	})
}

// TestFortranIntegerInput reads integers and logicals at the terminal
// (Phase 80, subtask 5): a unit with no OPEN takes blanks as zeros, and BN
// passes them over; a comma ends a field early; O and Z read octal and
// hexadecimal; L reads T, .F., and TRUE; a field the item can't hold is
// INPCONERR (64), and the item is 0. TestFor80InputOracle holds the
// probe's cases to VMS's; this runs FORMAT statements in both paging
// modes.
func TestFortranIntegerInput(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		lines := []string{
			"      PROGRAM IIN",
			"      INTEGER I, J, K, IOS",
			"      INTEGER*2 W",
			"      LOGICAL L1, L2, L3",
			"   10 FORMAT (I3, BN, I3)",
			"   20 FORMAT (3I6)",
			"   30 FORMAT (O6, Z6, I6)",
			"   40 FORMAT (L2, L3, L5)",
			"      READ (5,10) I, J",
			"      PRINT *, I, J",
			"      READ (5,20) I, J, K",
			"      PRINT *, I, J, K",
			"      READ (5,30) I, J, K",
			"      PRINT *, I, J, K",
			"      READ (5,40) L1, L2, L3",
			"      PRINT *, L1, L2, L3",
			"      W = 7",
			"      READ (5,'(I6)', IOSTAT=IOS) W",
			"      PRINT *, W, IOS",
			"      END",
		}
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]IIN.FOR"},
			rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		deleteFilesAfter(t, d, "IIN")

		for _, command := range []string{"FORTRAN IIN", "MACRO IIN", "LINK IIN"} {
			shareFindRun(t, d, command)

			if out := buf.String(); out != "" {
				t.Fatalf("%s said:\n%s", command, out)
			}
		}

		c.In = strings.NewReader("1 21 2\n1,-2,,\n   17,FF,12\n T.F.TRUE\n 40000\n")

		want := "        102          12\n          1          -2           0\n" +
			"         15         255          12\nT F T\n     0          64\n" +
			"  $STATUS == \"%X00000001\"\n"
		if got := shareRun(t, d, buf, "IIN"); got != want {
			t.Errorf("RUN IIN printed:\n%s\nwant:\n%s", got, want)
		}
	})
}

// TestFortranListInput reads list-directed input from the terminal, by each
// statement that can: a repeat count across a whole array, a null value, an
// implied-DO list, complex values (a number alone is the real part), character
// values cut off and padded, a logical, a character constant longer than the
// context's record that runs across four records, the end of the file, which
// END= and IOSTAT= take, and a value that can't be one, signaled.
func TestFortranListInput(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		lines := []string{
			"      PROGRAM LIN",
			"      INTEGER I(4), J, IOS",
			"      REAL R(3)",
			"      COMPLEX C(2)",
			"      CHARACTER*5 T(2)",
			"      CHARACTER*600 LONG",
			"      LOGICAL L",
			"      R(2) = 9.0",
			"      READ *, I",
			"      PRINT *, I",
			"      READ (5,*) (R(J), J = 1, 3)",
			"      PRINT *, R",
			"      ACCEPT *, C",
			"      PRINT *, C",
			"      READ (*,*) T, L",
			"      PRINT *, T, L",
			"      READ *, LONG",
			"      PRINT *, LONG(1:1), LONG(190:191), LONG(600:600)",
			"      READ (5,*) J",
			"      PRINT *, J",
			"      READ (5,*,END=10) J",
			"      PRINT *, 'NOT HERE'",
			"   10 READ (5,*,IOSTAT=IOS) J",
			"      PRINT *, 'IOS', IOS",
			"      END",
		}
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]LIN.FOR"},
			rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		deleteFilesAfter(t, d, "LIN")

		for _, command := range []string{"FORTRAN LIN", "MACRO LIN", "LINK LIN"} {
			shareFindRun(t, d, command)

			if out := buf.String(); out != "" {
				t.Fatalf("%s said:\n%s", command, out)
			}
		}

		c.In = strings.NewReader("1 2*7\n 9 THE REST IS LOST\n1.5,,2.5\n(1,2)\n 2.5\n'AB' 'CDEFGH' .T\n'" +
			strings.Repeat("A", 190) + "\n" + strings.Repeat("B", 190) + "\n" + strings.Repeat("C", 190) + "\n" +
			strings.Repeat("D", 190) + "'\n1.5E1\n")

		want := "          1           7           7           9\n" +
			"  1.500000       9.000000       2.500000    \n" +
			"(1.000000,2.000000) (2.500000,0.0000000E+00)\n" +
			"AB   CDEFG T\n" +
			"AABD\n" +
			"         15\n" +
			"IOS          -1\n" +
			"  $STATUS == \"%X00000001\"\n"
		if got := shareRun(t, d, buf, "LIN"); got != want {
			t.Errorf("RUN LIN printed:\n%s\nwant:\n%s", got, want)
		}

		lines = []string{
			"      PROGRAM LBAD",
			"      READ *, J",
			"      PRINT *, 'NOT HERE'",
			"      END",
		}
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]LBAD.FOR"},
			rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		deleteFilesAfter(t, d, "LBAD")

		for _, command := range []string{"FORTRAN LBAD", "MACRO LBAD", "LINK LBAD"} {
			shareFindRun(t, d, command)
		}

		buf.Reset()

		c.In = strings.NewReader("ABC\n")

		got := shareRun(t, d, buf, "LBAD")
		if !strings.Contains(got, "%FOR-F-LISIO_SYN, ") || strings.Contains(got, "NOT HERE") {
			t.Errorf("RUN LBAD printed:\n%s", got)
		}
	})
}

// TestFortranFloatInput reads F, E, and D fields from the terminal: the
// digits after the point a field with no point has, BN and BZ, the scale
// factor, which an exponent overrides, fields a comma ends, an exponent in
// a REAL*8, a complex value's two fields, text that isn't a number
// (INPCONERR, 64, the item zeros), and F of an INTEGER, which stores the
// F_floating value's bytes and fails with FORVARMIS (61).
func TestFortranFloatInput(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		lines := []string{
			"      PROGRAM FIN",
			"      REAL X, Y, Z",
			"      REAL*8 V",
			"      COMPLEX W",
			"      INTEGER I, IOS",
			"   10 FORMAT (2F8.2)",
			"   20 FORMAT (BN, F8.2, BZ, F8.2)",
			"   30 FORMAT (3PE10.5, E10.5)",
			"   40 FORMAT (3F5.2)",
			"   50 FORMAT (D15.3)",
			"   60 FORMAT (2F5.1)",
			"      READ (5,10) X, Y",
			"      WRITE (6,'(1X,2F10.3)') X, Y",
			"      READ (5,20) X, Y",
			"      WRITE (6,'(1X,2F10.3)') X, Y",
			"      READ (5,30) X, Y",
			"      WRITE (6,'(1X,2E13.5)') X, Y",
			"      READ (5,40) X, Y, Z",
			"      WRITE (6,'(1X,3F6.2)') X, Y, Z",
			"      READ (5,50) V",
			"      WRITE (6,'(1X,F13.3)') V",
			"      READ (5,60) W",
			"      WRITE (6,'(1X,2F6.1)') W",
			"      X = 7.0",
			"      READ (5,'(F5.2)',IOSTAT=IOS) X",
			"      WRITE (6,'(1X,F5.2,I4)') X, IOS",
			"      READ (5,'(F4.1)',IOSTAT=IOS) I",
			"      WRITE (6,'(1X,I6,I4)') I, IOS",
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

		c.In = strings.NewReader("    1234   12.34\n  1 2  3  1 2  3\n   37.614  37.614E2\n1,5,,\n" +
			"367.4981763D+04\n  1.5  2.5\nABC\n12.5\n")

		want := "    12.340    12.340\n     1.230  1020.030\n  0.37614E-01  0.37614E+04\n" +
			"  0.01  0.05  0.00\n  3674981.763\n   1.5   2.5\n 0.00  64\n 16968  61\n" +
			"  $STATUS == \"%X00000001\"\n"
		if got := shareRun(t, d, buf, "FIN"); got != want {
			t.Errorf("RUN FIN printed:\n%s\nwant:\n%s", got, want)
		}
	})
}
