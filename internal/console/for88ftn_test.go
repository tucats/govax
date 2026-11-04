package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
)

// for88FTNRTLDifferences are the cases whose output on FTNRTL differs from VMS's, by
// the case's line, with why: as FORRTL's over govax (for88Differences), but for NAME
// by file, which FTNRTL's own INQUIRE has.
var for88FTNRTLDifferences = map[string]string{
	"#295 many records, past a bucket: ten buckets' worth": "W7's splits",
}

// TestFor88OnFTNRTL runs all of round 2's cases on FTNRTL, by for88f.mar's FOR$ names,
// and holds each one's output, every FAB, RAB, and XABKEY shown to its USEROPEN routine
// among them, and each file it made, to VMS's run of them on FORRTL.
func TestFor88OnFTNRTL(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for88.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	program, err := filepath.Abs(filepath.Join(for88Dir, "for88.mar"))
	if err != nil {
		t.Fatal(err)
	}

	adapter, err := filepath.Abs(filepath.Join(for88Dir, "for88f.mar"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + program + `"/HOST FOR88.MAR`)
	dispatch(`COPY "` + adapter + `"/HOST FOR88F.MAR`)

	for _, command := range []string{"MACRO/NOLIST FOR88", "MACRO/NOLIST FOR88F", "LINK/NOMAP FOR88,FOR88F"} {
		if out := dispatch(command); out != "" {
			t.Fatalf("%s: %s", command, out)
		}
	}

	out := dispatch("RUN FOR88")

	if !strings.Contains(out, "##### the end of the cases") {
		t.Errorf("the probe stopped before its end; its last lines:\n%s", lastLines(out, 30))
	}

	text, err := os.ReadFile(filepath.Join(for88Dir, "vax", "for88-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	got, want := for88Cases(out), for88Cases(string(text))

	if len(want) != 310 {
		t.Errorf("%d cases in VMS's run, where gen.go wrote 310", len(want))
	}

	for title, w := range want {
		_, expected := for88FTNRTLDifferences[title]

		switch g, ok := got[title]; {
		case !ok:
			t.Errorf("%s: not run", title)
		case g != w && !expected:
			t.Errorf("%s:\nVMS:\n%s\nFTNRTL:\n%s", title, w, g)
		case g == w && expected:
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from for88FTNRTLDifferences", title)
		}
	}

	saved := filepath.Join(t.TempDir(), "ftn88.bck")

	dispatch(`BACKUP *.IDX;* FTN88.BCK/SAVE_SET`)
	dispatch(`COPY FTN88.BCK "` + saved + `"/HOST/BINARY`)

	ours := map[string]*backup.File{}

	for _, f := range readSaveSet(t, saved).Files {
		ours[f.Name] = f
	}

	for _, f := range readSaveSet(t, filepath.Join(for88Dir, "vax", "for88.bck")).Files {
		g, ok := ours[f.Name]

		switch {
		case !ok:
			t.Errorf("%s: VMS's run made it, FTNRTL's not", f.Name)
		case f.Name == "[000000]W7.IDX;1":
			compareRecordsOnly(t, f, g)
		default:
			compareConvertFile(t, f, g)
		}
	}
}

// TestFortranIndexedOpen runs the compiler's OPEN, INQUIRE, and CLOSE on an indexed
// file: KEY= as the compiler passes it, with a key's bounds known only at run time,
// makes the keys it says, and an OPEN of the file holds KEY= to them, each by its
// IOSTAT (testdata/for88's #125, #133, #139, and #97).
func TestFortranIndexedOpen(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "ftn88.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FTN88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	source := filepath.Join(t.TempDir(), "keyopen.for")
	text := `      PROGRAM KEYOPN
      CHARACTER*12 ORG, ACC, FRM, TYP, KYD
      INTEGER IOS, N, RL
      N = 9
      OPEN (UNIT=2, FILE='K.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='FORMATTED', RECL=20,
     2 KEY=(1:4, N:N+3:INTEGER:DESCENDING), IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'NEW', IOS
      INQUIRE (2, ORGANIZATION=ORG, ACCESS=ACC, FORM=FRM, RECL=RL,
     1 RECORDTYPE=TYP, KEYED=KYD)
      WRITE (6, '(1X,A,A,A,A,A,I3)') ORG, ACC, FRM, TYP, KYD, RL
      CLOSE (2)
      OPEN (UNIT=2, FILE='K.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='FORMATTED',
     2 KEY=(1:4, N:N+3:INTEGER:DESCENDING), IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'SAME', IOS
      CLOSE (2)
      OPEN (UNIT=2, FILE='K.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='FORMATTED', KEY=(2:5), IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'PLACE', IOS
      OPEN (UNIT=2, FILE='K.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='FORMATTED',
     2 KEY=(1:4:CHARACTER:DESCENDING), IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'DOWN', IOS
      OPEN (UNIT=2, FILE='K.IDX', STATUS='OLD',
     1 ORGANIZATION='SEQUENTIAL', IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'SEQ', IOS
      OPEN (UNIT=2, FILE='K.IDX', STATUS='OLD', IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'FOUND', IOS
      CLOSE (2)
      OPEN (UNIT=3, FILE='L.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 RECL=20, IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'NOKEY', IOS
      END
`

	if err := os.WriteFile(source, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + source + `"/HOST KEYOPEN.FOR`)

	for _, command := range []string{"FORTRAN KEYOPEN", "MACRO KEYOPEN", "LINK KEYOPEN"} {
		if out := dispatch(command); out != "" {
			t.Fatalf("%s: %s", command, out)
		}
	}

	// A dispatched command's output has a bar for each new line, its first blank gone.
	want := "NEW  0|INDEXED     KEYED       FORMATTED   FIXED       YES          20|" +
		"SAME  0|PLACE 49|DOWN 49|SEQ 51|FOUND  0|NOKEY 49"

	if out := strings.TrimSpace(dispatch("RUN KEYOPEN")); out != want {
		t.Errorf("the program's output:\n%s\nwant:\n%s", out, want)
	}

	report := dispatch("ANALYZE/RMS_FILE/OUTPUT=SYS$OUTPUT K.IDX")

	// Its keys, as KEY= said: the second's place a variable's value.
	for _, s := range []string{"KEY DESCRIPTOR #0", "KEY DESCRIPTOR #1", "Segment Positions:       8", "Data Type: descending"} {
		if !strings.Contains(report, s) {
			t.Errorf("ANALYZE/RMS_FILE has no %q:\n%s", s, strings.ReplaceAll(report, "|", "\n"))
		}
	}
}

// TestFortranIndexedRecords runs the record statements on an indexed file from FORTRAN
// source: WRITE out of order, every specifier on an ascending and a descending key, an
// INTEGER*2 value on a longword key, READ in sequence after a keyed one, REWRITE, DELETE,
// UNLOCK, and each statement's IOSTAT where VMS's run has one (testdata/for88).
func TestFortranIndexedRecords(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "ftn88.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FTN88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	source := filepath.Join(t.TempDir(), "keyrec.for")
	text := `      PROGRAM KEYREC
      CHARACTER*4 NAME, DOWN
      INTEGER*2 SHORT
      INTEGER ID, IOS
      OPEN (UNIT=2, FILE='P.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', RECL=3,
     2 KEY=(1:4, 5:8:INTEGER, 9:12:CHARACTER:DESCENDING), IOSTAT=IOS)
      WRITE (6, 90) 'OPEN', IOS
      WRITE (2) 'MMMM', 30, 'bbbb'
      WRITE (2) 'CCCC', 10, 'dddd'
      WRITE (2) 'TTTT', 20, 'aaaa'
      WRITE (2, IOSTAT=IOS) 'CCCC', 40, 'cccc'
      WRITE (6, 90) 'DUP', IOS
      READ (2, KEY='MMMM') NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      READ (2, KEYGT='MMMM') NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      READ (2, KEYGE='D') NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      READ (2, KEY='ZZZZ', IOSTAT=IOS) NAME
      WRITE (6, 90) 'NONE', IOS
      READ (2, KEYLT='MMMM', IOSTAT=IOS) NAME
      WRITE (6, 90) 'LT', IOS
      SHORT = 20
      READ (2, KEY=SHORT, KEYID=1) NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      READ (2) NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      READ (2, IOSTAT=IOS) NAME
      WRITE (6, 90) 'END', IOS
      READ (2, KEYLT='cccc', KEYID=2) NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      READ (2, KEYNXT='cccc') NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      READ (2, KEYGE='cccc', IOSTAT=IOS) NAME
      WRITE (6, 90) 'GE', IOS
      READ (2, KEY=9, KEYID=7, IOSTAT=IOS) NAME
      WRITE (6, 90) 'KEYID', IOS
      READ (2, KEY='MMMM', KEYID=0) NAME, ID, DOWN
      REWRITE (2) NAME, 31, 'eeee'
      REWRITE (2, IOSTAT=IOS) NAME, 32, 'ffff'
      WRITE (6, 90) 'AGAIN', IOS
      READ (2, KEY='MMMM') NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      REWRITE (2, IOSTAT=IOS) 'NNNN', ID, DOWN
      WRITE (6, 90) 'CHANGED', IOS
      READ (2, KEY='CCCC') NAME
      DELETE (2)
      DELETE (2, IOSTAT=IOS)
      WRITE (6, 90) 'GONE', IOS
      READ (2, KEY='CCCC', IOSTAT=IOS) NAME
      WRITE (6, 90) 'NOTHERE', IOS
      UNLOCK (2)
      REWIND (2, IOSTAT=IOS)
      WRITE (6, 90) 'REWIND', IOS
      CLOSE (2)
      OPEN (UNIT=2, FILE='P.IDX', STATUS='OLD', ORGANIZATION='INDEXED',
     1 ACCESS='SEQUENTIAL', FORM='UNFORMATTED', IOSTAT=IOS)
      READ (2) NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      READ (2, KEY='TTTT', IOSTAT=IOS) NAME
      WRITE (6, 90) 'SEQKEY', IOS
      READ (2) NAME, ID, DOWN
      WRITE (6, 91) NAME, ID, DOWN
      CLOSE (2)
      OPEN (UNIT=3, FILE='F.IDX', STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='FORMATTED', RECL=8, KEY=(1:2),
     2 IOSTAT=IOS)
      WRITE (3, '(A)') 'BBb2'
      WRITE (3, '(A)') 'AAa1'
      WRITE (3, '(A)') 'CCc3'
      READ (3, '(A4/A4)', KEY='AA') NAME, DOWN
      WRITE (6, '(1X,A,1X,A)') NAME, DOWN
      CLOSE (3)
90    FORMAT (1X, A, I3)
91    FORMAT (1X, A, I3, 1X, A)
      END
`

	if err := os.WriteFile(source, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + source + `"/HOST KEYREC.FOR`)

	for _, command := range []string{"FORTRAN KEYREC", "MACRO KEYREC", "LINK KEYREC"} {
		if out := dispatch(command); out != "" {
			t.Fatalf("%s: %s", command, out)
		}
	}

	// The FOR numbers: 50 INCKEYCHG, 36 ATTACCNON, 49 INVKEYSPE, 53 NO_CURREC, 20 REWERR,
	// 26 OPEDEFREQ, 94 INVMATKEY, and -1 for an end of file. KEYLT and KEYNXT of 'cccc' on the
	// descending key both find 'bbbb', the next in its order.
	want := []string{
		"OPEN  0", "DUP 50", "MMMM 30 bbbb", "TTTT 20 aaaa", "MMMM 30 bbbb", "NONE 36", "LT 94",
		"TTTT 20 aaaa", "MMMM 30 bbbb", "END -1", "MMMM 30 bbbb", "MMMM 30 bbbb", "GE 94", "KEYID 49",
		"AGAIN 53", "MMMM 31 eeee", "CHANGED 50", "GONE 53", "NOTHERE 36", "REWIND 20",
		"MMMM 31 eeee", "SEQKEY 26", "TTTT 20 aaaa", "AAa1 BBb2",
	}

	if out := strings.TrimSpace(dispatch("RUN KEYREC")); out != strings.Join(want, "|") {
		t.Errorf("the program's output:\n%s\nwant:\n%s", strings.ReplaceAll(out, "|", "\n"), strings.Join(want, "\n"))
	}
}
