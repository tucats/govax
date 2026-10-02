package asm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This file checks the listing's pages (listpage.go) against real MACRO's
// listings line for line, with the fidelity bar's masks
// (docs/PHASE-29.md): the dates and times, the assembler's name, and the
// source file's specification in each heading.

// headDateColumn is where the heading's date begins, on both lines, and
// headPageColumn where the first line's "Page" begins.
const (
	headDateColumn = 73
	headPageColumn = 123
)

// maskListing returns lines with each heading's dates, times, assembler
// name, and file specification blanked out.
func maskListing(lines []string) []string {
	out := make([]string, len(lines))

	for i, line := range lines {
		out[i] = line

		switch {
		case strings.HasPrefix(line, "\f") && len(line) > headPageColumn:
			out[i] = line[:headDateColumn] + "<masked>" + line[headPageColumn:]

		case i > 0 && strings.HasPrefix(lines[i-1], "\f") && len(line) > headDateColumn:
			out[i] = line[:headDateColumn] + "<masked>"
			if k := strings.LastIndex(line, "("); k > headDateColumn {
				out[i] += line[k:]
			}
		}
	}

	return out
}

// realSourcePages returns the lines of a real listing's source pages: the
// pages before the symbol table's (or, for a module with no symbols, the
// psect synopsis's).
func realSourcePages(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")

	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i-1], "\f") && (strings.HasPrefix(lines[i], "Symbol table") || strings.HasPrefix(lines[i], "Psect synopsis")) {
			return lines[:i-1]
		}
	}

	t.Fatalf("%s: no symbol table page", path)

	return nil
}

// compareListingLines reports each line of got that differs from want, and a
// difference in their counts.
func compareListingLines(t *testing.T, got, want []string) {
	t.Helper()

	got, want = maskListing(got), maskListing(want)

	errors := 0

	for i := 0; i < len(got) && i < len(want); i++ {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got[i], want[i])

			if errors++; errors == 10 {
				t.Fatal("too many differences")
			}
		}
	}

	if len(got) != len(want) {
		t.Errorf("%d lines, want %d", len(got), len(want))
	}
}

// TestFixtureListings compares the source pages govax lists for the
// Phase 27 fixtures (testdata/mar) and the Phase 30 LINK fixtures
// (testdata/link) with real MACRO's.
func TestFixtureListings(t *testing.T) {
	marDir := filepath.Join("..", "..", "testdata", "mar")
	linkDir := filepath.Join("..", "..", "testdata", "link")

	type fixture struct{ name, source, listing string }

	var cases []fixture

	ladder, err := filepath.Glob(filepath.Join(marDir, "*.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range ladder {
		name := strings.TrimSuffix(filepath.Base(path), ".mar")
		cases = append(cases, fixture{name, path, filepath.Join(marDir, "vax", name+".lis")})
	}

	// testdata/link/vax's listings: its own four modules, and five of
	// the ladder's, assembled again for the LINK run.
	for _, name := range []string{"addr", "defs", "share1", "share2"} {
		cases = append(cases, fixture{"link/" + name, filepath.Join(linkDir, name+".mar"), filepath.Join(linkDir, "vax", name+".lis")})
	}

	for _, name := range []string{"exprs", "extern", "general", "globals", "modes"} {
		cases = append(cases, fixture{"link/" + name, filepath.Join(marDir, name+".mar"), filepath.Join(linkDir, "vax", name+".lis")})
	}

	// The Phase 29 probe's sources listed with the default options.
	for _, name := range []string{"binary", "symtab", "xref", "trace", "failmain", "failsub", "failsig"} {
		cases = append(cases, fixture{"list/" + name, filepath.Join(marDir, "list", name+".mar"), filepath.Join(marDir, "list", "vax", name+".lis")})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(tc.source)
			if err != nil {
				t.Fatal(err)
			}

			a := macroAssembler()
			a.SetListing(true)

			if _, err := a.Assemble(string(src)); err != nil {
				t.Fatalf("assemble: %v", err)
			}

			got := a.Listing(ListingOptions{
				Assembled: time.Now(),
				Assembler: "govax MACRO V0.0-0",
				Source:    strings.ToUpper(filepath.Base(tc.source)) + ";1",
				Revised:   time.Now(),
			})

			compareListingLines(t, got, realSourcePages(t, tc.listing))
		})
	}
}

// TestListingHeading checks the heading's fields and columns exactly,
// without masks.
func TestListingHeading(t *testing.T) {
	a := recordListing(t, "\t.TITLE\tHELLO\tFixture 9: hello, world\n\t.IDENT\t/V1.0/\n", false)

	got := a.Listing(ListingOptions{
		Assembled: time.Date(2026, time.September, 29, 13, 13, 51, 0, time.UTC),
		Assembler: "VAX MACRO V5.4-3",
		Source:    "DUA2:[000000]HELLO.MAR;1",
		Revised:   time.Date(2026, time.September, 2, 13, 10, 48, 0, time.UTC),
	})

	want := []string{
		"\fHELLO                           Fixture 9: hello, world                  29-SEP-2026 13:13:51  VAX MACRO V5.4-3            Page   1",
		"V1.0                                                                      2-SEP-2026 13:10:48  DUA2:[000000]HELLO.MAR;1          (1)",
		"",
		"                                     0000     1 \t.TITLE\tHELLO\tFixture 9: hello, world",
		"                                     0000     2 \t.IDENT\t/V1.0/",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("listing:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestListingPagination checks that each page holds 57 lines under its
// heading.
func TestListingPagination(t *testing.T) {
	var src strings.Builder

	src.WriteString("\t.PSECT\tD\n")

	for i := 0; i < 56; i++ {
		src.WriteString("\t.BYTE\t1\n")
	}

	// Line 58: 13 bytes, one more than a line holds.
	src.WriteString("\t.BYTE\t1,2,3,4,5,6,7,8,9,10,11,12,13\n")

	a := recordListing(t, src.String(), false)
	got := a.Listing(ListingOptions{})

	if len(got) != 2*listHeadLines+59 {
		t.Fatalf("%d lines, want %d", len(got), 2*listHeadLines+59)
	}

	if !strings.HasPrefix(got[listPageLines], "\f") || !strings.HasSuffix(got[listPageLines], "Page   2") {
		t.Errorf("line %d = %q, want page 2's heading", listPageLines+1, got[listPageLines])
	}

	if want := "                                 0D  0044       "; got[len(got)-1] != want {
		t.Errorf("last line %q, want %q", got[len(got)-1], want)
	}
}
