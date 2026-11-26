package asm

import (
	"strings"
	"testing"
)

// TestListingUpdated checks how a listing shows a source SUMSLP updated
// (update.go), on testdata/sum117's SUMA.MAR updated as its M01, M02, and
// M13 cases updated it: SUMSLP's numbers, its field over the binary field's
// first 16 columns and the 20 left beside it, a warning before its line, and
// the summary's line labels.
func TestListingUpdated(t *testing.T) {
	a := macroAssembler()
	a.SetListing(true)
	a.SetUpdate(0, &Update{
		Inputs: 9,
		Lines: []UpdatedLine{
			{Line: 0, Insert: ".1", Before: []UpdateNote{{Lines: []string{
				"%SUM-W-SLPSYNERR, SLP command syntax error", "-A    DUA1:[SUM117.IN]Y08.UPD;1",
			}}}},
			{Line: 3},
			{Line: 3, Insert: ".1", Field: "; M01"},
			{Line: 5, Field: "-1"},
			{Line: 6},
			{Line: 7},
			{Line: 7, Insert: ".1"},
			{Line: 8},
		},
	})

	src := "\tNOP\n" +
		"\t.PSECT\tDATA,NOEXE,WRT\n" +
		"VALUE:\t.LONG\t7\n" +
		"\t.PSECT\tCODE,EXE,NOWRT\n" +
		"\t.ENTRY\tSTART,^M<>\n" +
		"\tMOVL\tVALUE,R0\n" +
		"\tMOVL\t#1\n" +
		"\tRET\n"

	if _, err := a.Assemble(src); err == nil {
		t.Fatal("MOVL #1 assembled")
	}

	got := sourcePageLines(a)
	want := []string{
		"                                     0000       ",
		"                                     0000       ",
		"%SUM-W-SLPSYNERR, SLP command syntax error",
		"-A    DUA1:[SUM117.IN]Y08.UPD;1",
		"                                     0000       ",
		"                                 01  0000    .1 \tNOP",
		"                                 00000000     3 \t.PSECT\tDATA,NOEXE,WRT",
		"; M01                      00000007  0000    .1 VALUE:\t.LONG\t7",
		"-1                               00000000     5 \t.PSECT\tCODE,EXE,NOWRT",
		"                               0000  0000     6 \t.ENTRY\tSTART,^M<>",
		"                   00000000'EF   D0  0002     7 \tMOVL\tVALUE,R0",
		"                            50       0008       ",
		"                            01   D0  0009    .1 \tMOVL\t#1",
		"%MACRO-E-NOTENUFOPR, Not enough operands supplied                !",
		"                                     000B       ",
		"                                 04  000B     8 \tRET",
	}

	compareLines(t, "source pages", got, want)

	listing := strings.Join(a.Listing(ListingOptions{}), "\n")
	if !strings.Contains(listing, "There were 1 error, 1 warning and 0 information messages, on lines:\n    9 (1)         7.1 (1)   ") {
		t.Errorf("summary doesn't list the warning on 9 and the error on 7.1:\n%s", listing)
	}
}
