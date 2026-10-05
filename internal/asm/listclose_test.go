package asm

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// This file checks the closing pages (listclose.go) in the cases the
// real listings TestFixtureListings compares don't reach.

// closingLines returns the closing pages of a's listing.
func closingLines(t *testing.T, a *Assembler, opts ListingOptions) []string {
	t.Helper()

	_, closing := splitListing(t, a.Listing(opts))

	return closing
}

// headings returns the second heading line's label of each page in
// lines.
func headings(lines []string) []string {
	var out []string

	for i, line := range lines {
		if strings.HasPrefix(line, "\f") {
			out = append(out, strings.TrimRight(lines[i+1][:headDateColumn], " "))
		}
	}

	return out
}

// TestClosingPageLabels checks the second heading line of each closing
// page: the symbol table's, the psect synopsis's when there are no
// symbols, and the statistics' when a page breaks in them.
func TestClosingPageLabels(t *testing.T) {
	// A symbol table of 144 symbols with names longer than 15
	// characters, so two columns to a page: 114 on its first page, 30
	// on the next, which goes on with the psect synopsis (8 lines) and
	// the start of the statistics; the third page starts in the
	// statistics.
	var src strings.Builder
	for i := 0; i < 144; i++ {
		fmt.Fprintf(&src, "SYMBOL_NUMBER_%03d = %d\n", i, i)
	}

	a := recordListing(t, src.String(), false)

	got := headings(closingLines(t, a, ListingOptions{}))
	want := []string{labelSymbols, labelSymbols, labelStatistics}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("headings %q, want %q", got, want)
	}

	// With no symbols, the closing pages start with the psect synopsis
	// (a blank line, then its box), under its own heading.
	a = recordListing(t, "\t.TITLE\tEMPTY\n", false)
	closing := closingLines(t, a, ListingOptions{})

	if got := headings(closing); got[0] != labelSynopsis {
		t.Errorf("first heading %q, want %q", got[0], labelSynopsis)
	}

	if closing[3] != "" || !strings.HasSuffix(closing[4], "+----------------+") {
		t.Errorf("psect synopsis starts %q, %q", closing[3], closing[4])
	}
}

// TestSymbolTableColumns checks the symbol table's lines: the flags and
// psect numbers, a suppressed symbol left out, and the name column
// widened to 31 by a symbol (listed or not) longer than 15 characters.
// The wide lines are as real MACRO's FABALIGN.LIS writes them.
func TestSymbolTableColumns(t *testing.T) {
	src := strings.Join([]string{
		"\t.ENABLE\tSUPPRESSION",
		"A_NAME_LONGER_THAN_15 = 1", // unreferenced: not listed
		"\t.DISABLE\tSUPPRESSION",
		"$$.TMP = 0",
		"\t.PSECT\tDATA, WRT, NOEXE, LONG",
		"GOOD:\t.LONG\t0",
		"$$.TAB = GOOD + 81",
		"\t.LONG\tEXT",
		"",
	}, "\n")

	a := recordListing(t, src, false)
	closing := closingLines(t, a, ListingOptions{})

	want := []string{
		"$$.TAB                         = 00000051 R     01        ",
		"$$.TMP                         = 00000000                 ",
		"EXT                              ********   X   01        ",
		"GOOD                             00000000 R     01        ",
		"",
	}

	if got := closing[3 : 3+len(want)]; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("symbol table:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestLibraryStatistics checks the macro library statistics: each
// library in search order with the macros defined from it, their total,
// and the records read to define them.
func TestLibraryStatistics(t *testing.T) {
	lib1 := newMapLibrary(map[string]string{"TWO": "\t.MACRO\tTWO\n\t.BYTE\t2\n\t.ENDM"})
	lib2 := newMapLibrary(map[string]string{"ONE": "\t.MACRO\tONE\n\t.ENDM"})

	a := macroAssembler()
	a.SetListing(true)
	a.SetMacroLibraries(NamedMacroLibrary(lib1, "DUA1:[000000]LIB1.MLB;1"), NamedMacroLibrary(lib2, "DUA1:[000000]LIB2.MLB;1"))

	if _, err := a.Assemble("\t.PSECT\tDATA\n\tTWO\n\tONE\n"); err != nil {
		t.Fatal(err)
	}

	closing := closingLines(t, a, ListingOptions{Command: "MACRO/LIST X"})

	want := []string{
		"Macro library name                           Macros defined      ",
		"------------------                           --------------      ",
		"DUA1:[000000]LIB1.MLB;1                                 1        ",
		"DUA1:[000000]LIB2.MLB;1                                 1        ",
		"TOTALS (all libraries)                                  2        ",
		"",
		"5 GETS were required to define 2 macros.",
		"",
		"There were no errors, warnings or information messages.",
		"",
		"MACRO/LIST X",
	}

	if got := closing[len(closing)-len(want):]; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("statistics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestListingSummary checks the summary of an assembly with errors and
// a warning: the counts, then the lines, five to a row, as real MACRO's
// ERRORS.LIS lays them out. A message in a macro expansion counts on the
// line that called the macro.
func TestListingSummary(t *testing.T) {
	src := strings.Join([]string{
		"\t.MACRO\tBAD",
		"\tNOSUCH",
		"\t.ENDM",
		"\t.PSECT\tDATA",
		"\tNOSUCH", // 5
		"\tNOSUCH",
		"\t.WARN\t; careful",
		"\tNOSUCH",
		"\tNOSUCH",
		"\tBAD", // 10
		"",
	}, "\n")

	a := recordListing(t, src, true)
	closing := closingLines(t, a, ListingOptions{Command: "MACRO/LIST X"})

	want := []string{
		"There were 5 errors, 1 warnings and 0 information messages, on lines:",
		"    5 (1)         6 (1)         7 (1)         8 (1)         9 (1)     ",
		"   10 (1)     ",
		"",
		"MACRO/LIST X",
	}

	if got := closing[len(closing)-len(want):]; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("summary:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestDurationText checks the performance indicators' times.
func TestDurationText(t *testing.T) {
	cases := map[time.Duration]string{
		0:                       "00:00:00.00",
		9 * time.Millisecond:    "00:00:00.00",
		110 * time.Millisecond:  "00:00:00.11",
		61*time.Second + 5e7:    "00:01:01.05",
		2*time.Hour + time.Hour: "03:00:00.00",
	}

	for d, want := range cases {
		if got := durationText(d); got != want {
			t.Errorf("durationText(%v) = %q, want %q", d, got, want)
		}
	}
}
