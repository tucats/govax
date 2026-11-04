package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
)

// for88OpenCases says which of round 2's cases are FTNRTL's OPEN, CLOSE, and INQUIRE
// (docs/PHASE-88 - keyed index files.md, subtask 12): the first three sections, but
// for #92, which makes STD.IDX with WRITEs, subtask 13's. The cases after it use the
// file VMS made.
func for88OpenCases(title string) bool {
	number, _, _ := strings.Cut(strings.TrimPrefix(title, "#"), " ")

	n, err := strconv.Atoi(number)
	if err != nil {
		return false
	}

	return n <= 148 && n != 92
}

// for88CutProgram is for88.mar with the cases for88OpenCases doesn't want taken out
// of its main routine, which runs them in order: #92's call, and every case from the
// section on keyed READs on. Their code stays, calling nothing that runs.
func for88CutProgram(t *testing.T, source string) string {
	t.Helper()

	heading := regexp.MustCompile(`(?m)^(T\d+):\t\.ASCID\t~##### Keyed READs: each match on each key~$`).FindStringSubmatch(source)
	if heading == nil {
		t.Fatal("no heading for the keyed READs in for88.mar")
	}

	from := strings.Index(source, "\tPUSHAB\t"+heading[1]+"\n")
	to := strings.Index(source, "\tPUSHAB\tENDM\n")

	if from < 0 || to < from {
		t.Fatal("no keyed READs in for88.mar's main routine")
	}

	source = source[:from] + source[to:]

	return strings.Replace(source, "\tCALLS\t#0,L^C92\n", "", 1)
}

// TestFor88OnFTNRTL runs round 2's cases of OPEN, CLOSE, and INQUIRE on FTNRTL, by
// for88f.mar's FOR$ names, and holds each one's output, every FAB, RAB, and XABKEY
// shown to its USEROPEN routine among them, and each file it made, to VMS's run of
// them on FORRTL.
func TestFor88OnFTNRTL(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for88.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	source, err := os.ReadFile(filepath.Join(for88Dir, "for88.mar"))
	if err != nil {
		t.Fatal(err)
	}

	program := filepath.Join(t.TempDir(), "for88.mar")
	if err := os.WriteFile(program, []byte(for88CutProgram(t, string(source))), 0o600); err != nil {
		t.Fatal(err)
	}

	adapter, err := filepath.Abs(filepath.Join(for88Dir, "for88f.mar"))
	if err != nil {
		t.Fatal(err)
	}

	save, err := filepath.Abs(filepath.Join(for88Dir, "vax", "for88.bck"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + program + `"/HOST FOR88.MAR`)
	dispatch(`COPY "` + adapter + `"/HOST FOR88F.MAR`)

	if out := dispatch(`BACKUP "` + save + `"/SAVE_SET/SELECT=[000000]STD.IDX [000000]`); strings.Contains(out, "-E-") || strings.Contains(out, "-F-") {
		t.Fatalf("BACKUP: %s", out)
	}

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
	compared := 0

	for title, w := range want {
		if !for88OpenCases(title) {
			continue
		}

		compared++

		if g, ok := got[title]; !ok {
			t.Errorf("%s: not run", title)
		} else if g != w {
			t.Errorf("%s:\nVMS:\n%s\nFTNRTL:\n%s", title, w, g)
		}
	}

	if compared != 148 {
		t.Errorf("%d cases compared, where #0 to #148 less #92 are 148", compared)
	}

	saved := filepath.Join(t.TempDir(), "ftn88.bck")

	dispatch(`BACKUP B*.IDX;* FTN88.BCK/SAVE_SET`)
	dispatch(`COPY FTN88.BCK "` + saved + `"/HOST/BINARY`)

	ours := map[string]*backup.File{}

	for _, f := range readSaveSet(t, saved).Files {
		ours[f.Name] = f
	}

	files := 0

	for _, f := range readSaveSet(t, filepath.Join(for88Dir, "vax", "for88.bck")).Files {
		if !strings.HasPrefix(f.Name, "[000000]B") {
			continue
		}

		files++

		if g, ok := ours[f.Name]; !ok {
			t.Errorf("%s: VMS's run made it, FTNRTL's not", f.Name)
		} else {
			compareConvertFile(t, f, g)
		}
	}

	if files == 0 {
		t.Error("no B files in VMS's save set")
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
