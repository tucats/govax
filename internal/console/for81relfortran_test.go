package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// relativeCases are testdata/for81's cases on relative files and the
// statements of their records, DELETE, REWRITE, UNLOCK, and the keyed READ, as
// FORTRAN programs written in directCase's steps. Round 1's #572 opened its
// file formatted by mistake, so its unformatted reads are FOR$_MIXFILACC, and
// it shows what a unit with no current record does; round 2's #33 is the same
// done right.
var relativeCases = []directCase{
	{"for81-vax.txt", 571, "REL1.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=2, ACCESS='DIRECT', STATUS='UNKNOWN'", "Q",
		"W 1 8", "W 2 8", "W 5 8", "R 1 8", "R 3 8", "R 6 8", "W 2 8", "X 2", "R 2 8", "X 2",
		"W 2 8", "R 2 8", "K 8", "Z", "Z", "L", "C"}},
	{"for81-vax.txt", 572, "REL1.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', STATUS='OLD'", "Q", "s 8", "s 8", "s 8", "Z", "N", "s 8", "s 8",
		"S 8", "b", "E", "C"}},
	{"for81-vax.txt", 573, "REL2.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=11, ACCESS='DIRECT', FORM='FORMATTED', STATUS='UNKNOWN'",
		"w 1 AAA", "w 3 CCCCCCCCCCC", "w 4 DDDDDDDDDDDD", "r 1 11", "r 3 11", "k REWRITTEN", "r 3 11", "C"}},
	{"for81-vax.txt", 574, "REL3.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=40, RECORDTYPE='VARIABLE', ACCESS='DIRECT', FORM='FORMATTED', STATUS='UNKNOWN'",
		"w 1 A", "w 2 BBBBBB", "w 4 DDDD", "r 2 40", "k LONGER_REWRITE", "C"}},
	{"for81-vax.txt", 575, "REL4.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=2, ACCESS='DIRECT', MAXREC=3, STATUS='UNKNOWN'",
		"W 3 8", "W 4 8", "R 4 8", "C", "O ORGANIZATION='RELATIVE', ACCESS='DIRECT', STATUS='OLD'"}},
	{"for81-vax.txt", 576, "REL1.DAT", false, false, []string{
		"P", "O STATUS='OLD'", "C", "O STATUS='OLD', ORGANIZATION='SEQUENTIAL'", "C",
		"O FILE='WALK.DAT', STATUS='OLD', ORGANIZATION='RELATIVE'", "C",
		"O STATUS='OLD', ORGANIZATION='RELATIVE', RECL=3", "C",
		"O STATUS='OLD', ORGANIZATION='RELATIVE', RECORDTYPE='VARIABLE'", "C",
		"O STATUS='OLD', ORGANIZATION='RELATIVE', ACCESS='KEYED'", "C"}},
	{"for81-vax.txt", 577, "WALK2.DAT", false, false, []string{
		"M WALK2", "O STATUS='OLD'", "f 8", "Z", "X 1", "k TEXTX", "L", "C", "U 18", "Z", "L", "K 0"}},
	{"for81-vax.txt", 578, "RWF.DAT", false, false, []string{
		"O RECORDTYPE='FIXED', RECL=4, STATUS='UNKNOWN'", "T ABCD", "T EFGH", "N", "f 4", "k WXYZ",
		"f 4", "k Q", "C"}},
	{"for81-vax.txt", 579, "RWV.DAT", false, false, []string{
		"O STATUS='UNKNOWN'", "T ABCD", "N", "f 4", "k WXYZ", "N", "f 4", "k WX", "C"}},
	{"for81-vax.txt", 580, "WALK2.DAT", false, false, []string{"O STATUS='OLD'", "Y"}},
	{"for81f-vax.txt", 28, "AR.DAT", false, false, []string{
		"M AR", "O ACCESS='APPEND', STATUS='OLD', ORGANIZATION='RELATIVE'", "Q", "T _APPENDED", "b",
		"f 12", "N", "f 12", "T _AFTER_REWIND", "C"}},
	{"for81f-vax.txt", 33, "RS1.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=2, ACCESS='DIRECT', STATUS='UNKNOWN'", "W 1 8", "W 2 8", "W 5 8", "C",
		"O ORGANIZATION='RELATIVE', STATUS='OLD', FORM='UNFORMATTED'", "Q", "s 8", "s 8", "Z", "s 8", "Q",
		"s 8", "N", "s 8", "s 8", "S 8", "K 8", "b", "E", "N", "s 8", "s 8", "s 8", "s 8", "s 8", "C"}},
	{"for81f-vax.txt", 34, "RS2.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=2, FORM='UNFORMATTED', STATUS='UNKNOWN'",
		"S 8", "S 8", "N", "S 8", "s 8", "N", "s 8", "S 8", "S 8", "C"}},
	{"for81f-vax.txt", 35, "RS3.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=8, STATUS='UNKNOWN'", "T ONE", "T TWO", "T THREE", "N", "f 8",
		"Z", "Z", "f 8", "k REPLACED", "f 8", "f 8", "C"}},
	{"for81f-vax.txt", 36, "RS4.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=2, ACCESS='DIRECT', MAXREC=9, STATUS='UNKNOWN'", "W 2 8",
		"X 2", "X 2", "X 7", "X 0", "X 10", "K 8", "F 2", "Z", "F 7", "C"}},
	{"for81f-vax.txt", 37, "WALK2.DAT", false, false, []string{"O STATUS='OLD'", "Y"}},
	{"for81f-vax.txt", 38, "", false, false, []string{"U 18", "Y"}},
	{"for81f-vax.txt", 39, "WALK2.DAT", false, false, []string{"O STATUS='OLD'", "y"}},
	{"for81f-vax.txt", 40, "", false, false, []string{"U 18", "y"}},
	{"for81f-vax.txt", 68, "LK1.DAT", false, false, []string{
		"O ORGANIZATION='RELATIVE', RECL=2, ACCESS='DIRECT', STATUS='UNKNOWN', SHARED", "W 1 8", "W 2 8",
		"U 11", "O ORGANIZATION='RELATIVE', ACCESS='DIRECT', STATUS='OLD', SHARED",
		"U 10", "R 1 8", "U 11", "R 2 8", "U 10", "R 2 8", "U 11", "L", "U 10", "R 2 8", "L",
		"U 11", "R 1 8", "R 1 8"}},
}

// madeFile is the statements that make a file a case reads, which the probe
// made in an earlier case: WALK2.DAT as round 1's #544 left it, and AR.DAT as
// round 2's #19 made it.
func madeFile(name string) []string {
	switch name {
	case "WALK2":
		return []string{"      OPEN (99, FILE='WALK2.DAT', STATUS='NEW')",
			"      WRITE (99, '(A)') ' ONE'", "      WRITE (99, '(A)') ' NEW'", "      CLOSE (99)"}
	case "AR":
		return []string{"      OPEN (99, FILE='AR.DAT', STATUS='NEW', ORGANIZATION='RELATIVE', RECL=8)",
			"      WRITE (99, '(A)') ' ONE'", "      CLOSE (99)"}
	}

	return nil
}

// TestFor81RelativeFortranOracle holds FORTRAN's statements on relative files,
// and DELETE, REWRITE, UNLOCK, and the keyed READ on any file, to VMS's FORRTL
// (testdata/for81, rounds 1 and 2): each statement's IOSTAT, the bytes each
// read returned, and INQUIRE's OPENED, NEXTREC, and RECL.
func TestFor81RelativeFortranOracle(t *testing.T) {
	runs := map[string]string{}

	for _, run := range []string{"for81-vax.txt", "for81f-vax.txt"} {
		text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", run))
		if err != nil {
			t.Fatal(err)
		}

		runs[run] = string(text)
	}

	d, c, buf := shareFindConsole(t, false)

	t.Cleanup(func() {
		for _, name := range []string{"REL1", "REL2", "REL3", "REL4", "WALK", "WALK2", "RWF", "RWV", "AR",
			"RS1", "RS2", "RS3", "RS4", "LK1", "FOR018"} {
			_ = d.Dispatch("DELETE " + name + ".DAT;*")
		}
	})

	// The cases run in order, as the probe's did: #572 and #576 read #571's file.
	for _, dc := range relativeCases {
		events, _ := directExpected(t, runs[dc.run], dc)
		want := directWant(t, dc, events)

		name := fmt.Sprintf("REL%d", dc.number)
		fortranBuild(t, d, c, buf, name, fixedForm(directProgram(dc)))

		out := strings.ReplaceAll(shareRun(t, d, buf, name), "\r\n", "\n")
		shareFindRun(t, d, "DELETE "+name+".EXE;*,"+name+".OBJ;*,"+name+".MAR;*")

		if got := strings.TrimRight(out[:strings.LastIndex(out, "$STATUS")], " "); got != want {
			t.Errorf("%s #%d printed:\n%s\nVMS's answers:\n%s", dc.run, dc.number, got, want)
		}
	}
}
