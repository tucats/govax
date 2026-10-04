package asm

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// readListing returns a real listing's lines.
func readListing(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// splitListing splits a listing's lines into its source pages and its
// closing pages, which start at the page headed "Symbol table" (or, for
// a module with no symbols, "Psect synopsis").
func splitListing(t *testing.T, lines []string) (source, closing []string) {
	t.Helper()

	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i-1], "\f") && (strings.HasPrefix(lines[i], labelSymbols) || strings.HasPrefix(lines[i], labelSynopsis)) {
			return lines[:i-1], lines[i-1:]
		}
	}

	t.Fatal("no symbol table page")

	return nil, nil
}

// listedLibraries returns the macro library names a real listing's
// statistics show.
func listedLibraries(lines []string) []string {
	var names []string

	for i, line := range lines {
		if !strings.HasPrefix(line, "------------------ ") {
			continue
		}

		for _, row := range lines[i+1:] {
			if row == "" || strings.HasPrefix(row, "TOTALS ") {
				break
			}

			names = append(names, strings.Fields(row)[0])
		}
	}

	return names
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

// TestFixtureListings compares the listings govax writes for the Phase
// 27 fixtures (testdata/mar), the Phase 30 LINK fixtures
// (testdata/link), and the Phase 29 probe's sources listed with the
// default options with real MACRO's, line for line. The source pages are
// compared with the heading masks; the closing pages with the allowed
// differences (listingDifferences).
func TestFixtureListings(t *testing.T) {
	marDir := filepath.Join("..", "..", "testdata", "mar")
	linkDir := filepath.Join("..", "..", "testdata", "link")

	type fixture struct {
		name, source, listing string
		// show and noshow are the listing's /SHOW= and /NOSHOW=.
		show, noshow []string
		// xref and xrefKinds are its /CROSS_REFERENCE[=(...)], and
		// noObject its /NOOBJECT.
		xref      bool
		xrefKinds []string
		noObject  bool
	}

	var cases []fixture

	ladder, err := filepath.Glob(filepath.Join(marDir, "*.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range ladder {
		name := strings.TrimSuffix(filepath.Base(path), ".mar")
		cases = append(cases, fixture{name: name, source: path, listing: filepath.Join(marDir, "vax", name+".lis")})
	}

	// testdata/link/vax's listings: its own four modules, and five of
	// the ladder's, assembled again for the LINK run.
	for _, name := range []string{"addr", "defs", "share1", "share2"} {
		cases = append(cases, fixture{name: "link/" + name, source: filepath.Join(linkDir, name+".mar"), listing: filepath.Join(linkDir, "vax", name+".lis")})
	}

	for _, name := range []string{"exprs", "extern", "general", "globals", "modes"} {
		cases = append(cases, fixture{name: "link/" + name, source: filepath.Join(marDir, name+".mar"), listing: filepath.Join(linkDir, "vax", name+".lis")})
	}

	// The Phase 29 probe's sources listed with the default options.
	for _, name := range []string{"binary", "symtab", "xref", "trace", "failmain", "failsub", "failsig", "notitle", "lctl", "errors"} {
		cases = append(cases, fixture{name: "list/" + name, source: filepath.Join(marDir, "list", name+".mar"), listing: filepath.Join(marDir, "list", "vax", name+".lis")})
	}

	// The VMS round's sources (testdata/mar/round).
	for _, name := range []string{"notitle2", "notitle3", "cells", "cellsb"} {
		cases = append(cases, fixture{name: "round/" + name, source: filepath.Join(marDir, "round", name+".mar"), listing: filepath.Join(marDir, "round", "vax", name+".lis")})
	}

	// The listing controls' source, listed again with /SHOW= and
	// /NOSHOW= (list.com).
	lctl := filepath.Join(marDir, "list", "lctl.mar")
	cases = append(cases,
		fixture{name: "list/lctlshow", source: lctl, listing: filepath.Join(marDir, "list", "vax", "lctlshow.lis"), show: []string{"EXPANSIONS", "BINARY"}},
		fixture{name: "list/lctlnosh", source: lctl, listing: filepath.Join(marDir, "list", "vax", "lctlnosh.lis"), noshow: []string{"CONDITIONALS", "CALLS", "DEFINITIONS"}},
	)

	// The cross-reference listings (list.com): XREF with
	// /CROSS_REFERENCE=ALL, and SYMTAB with /CROSS_REFERENCE and
	// /NOOBJECT. (xref.lis, above, is /CROSS_REFERENCE's.)
	cases = append(cases,
		fixture{name: "list/xrefall", source: filepath.Join(marDir, "list", "xref.mar"), listing: filepath.Join(marDir, "list", "vax", "xrefall.lis"), xref: true, xrefKinds: []string{"ALL"}},
		fixture{name: "list/symxref", source: filepath.Join(marDir, "list", "symtab.mar"), listing: filepath.Join(marDir, "list", "vax", "symxref.lis"), xref: true, noObject: true},
	)


	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			real := readListing(t, tc.listing)

			// The system library real MACRO searched, by the name its
			// listing gives it; none of these sources uses a macro.
			var libs []MacroLibrary
			for _, name := range listedLibraries(real) {
				libs = append(libs, NamedMacroLibrary(newMapLibrary(nil), name))
			}

			check := listingCheck{
				show: tc.show, noshow: tc.noshow,
				xref: tc.xref || tc.name == "list/xref", xrefKinds: tc.xrefKinds,
				noObject: tc.noObject,
			}

			check.fails = tc.name == "list/errors"

			checkListing(t, tc.source, tc.listing, libs, check)
		})
	}
}

// listingCheck is how checkListing compares a listing.
type listingCheck struct {
	// source, when set, is the source to assemble instead of the file's
	// (see sourceFromListing).
	source string
	// noClosing compares the source pages only.
	noClosing bool
	// allow, when set, applies allowed differences of the fixture's own
	// to both sides' closing pages, once headings are gone.
	allow func([]string) []string
	// show and noshow are the listing options MACRO's /SHOW= and
	// /NOSHOW= set.
	show, noshow []string
	// extraRecords is how many more object records than govax's real
	// MACRO's object holds, for a known difference in the object (not
	// the listing).
	extraRecords int
	// xref and xrefKinds are MACRO's /CROSS_REFERENCE[=(...)], and
	// noObject its /NOOBJECT: no object, so the record count is 0.
	xref      bool
	xrefKinds []string
	noObject  bool
	// fails says the source has errors. Real MACRO still writes an
	// object, which govax doesn't (Phase 27's choice), so the listing's
	// record count is 0.
	fails bool
}

// checkListing assembles source with libs, and compares its listing with
// the real one at listing: the source pages with the heading masks, and
// the closing pages with the allowed differences (listingDifferences).
func checkListing(t *testing.T, source, listing string, libs []MacroLibrary, check listingCheck) {
	t.Helper()

	src := check.source
	if src == "" {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}

		src = string(data)
	}

	real := readListing(t, listing)

	a := macroAssembler()
	a.SetListing(true)
	a.SetMacroLibraries(libs...)

	if err := a.SetListingShow(check.show, check.noshow); err != nil {
		t.Fatal(err)
	}

	if err := a.SetCrossReference(check.xref, check.xrefKinds); err != nil {
		t.Fatal(err)
	}

	_, err := a.Assemble(src)

	switch {
	case check.fails && err == nil:
		t.Fatal("assembly succeeded, want errors")
	case !check.fails && err != nil:
		t.Fatalf("assemble: %v", err)
	case !check.fails && !check.noObject:
		if _, err := a.Object(ObjectOptions{}); err != nil {
			t.Fatalf("object: %v", err)
		}
	}

	got := a.Listing(ListingOptions{
		Assembled: time.Now(),
		Assembler: "govax MACRO V0.0-0",
		Source:    strings.ToUpper(filepath.Base(source)) + ";1",
		Revised:   time.Now(),
		Command:   real[len(real)-1],
	})

	gotSource, gotClosing := splitListing(t, got)
	realSource, realClosing := splitListing(t, real)

	compareListingLines(t, gotSource, realSource)

	// The closing pages start under the same heading.
	if label := realClosing[1][:headDateColumn]; !strings.HasPrefix(gotClosing[1], label) {
		t.Errorf("closing heading %q, want %q", gotClosing[1], label)
	}

	if check.noClosing {
		return
	}

	missing := check.extraRecords

	// With no object, every record real MACRO counted is a difference.
	if check.fails {
		for _, line := range realClosing {
			if m := recordCount.FindStringSubmatch(line); m != nil {
				missing, _ = strconv.Atoi(m[1])
			}
		}
	}

	gotText := closingText(gotClosing)
	realText := closingText(realClosingAllowed(realClosing, missing))

	if check.allow != nil {
		gotText, realText = check.allow(gotText), check.allow(realText)
	}

	compareListingLines(t, gotText, realText)
}

// listingDifferences are the lines of real MACRO's closing pages that
// govax leaves out: the statistics about real MACRO's own memory, which
// govax has no figure for (Decision 2): its working set limit, and the
// memory its intermediate code, symbol table, and macros used.
//
// realClosingAllowed applies these and the other allowed differences
// between govax's closing pages and real MACRO's (docs/PHASE-29.md, "The
// fidelity bar"):
//
//   - The performance indicators' page faults column (Decision 2), which
//     is taken out of real MACRO's lines.
//   - With those lines gone, the closing pages break at different
//     places, so the closing pages are compared without their page
//     headings (closingText). TestClosingPageLabels checks those.
//   - The CPU and elapsed times, which are masked.
//   - The object record count, less the records real MACRO's object has
//     that govax's doesn't, for a known difference in the object
//     (listingCheck.extraRecords and fails).
var listingDifferences = []*regexp.Regexp{
	regexp.MustCompile(`^The working set limit was \d+ pages\.$`),
	regexp.MustCompile(`^\d+ bytes \(\d+ pages?\) of virtual memory were used to buffer the intermediate code\.$`),
	regexp.MustCompile(`^There were \d+ pages of symbol table space allocated to hold \d+ non-local and \d+ local symbols\.$`),
	regexp.MustCompile(`^\d+ pages? of virtual memory (were|was) used to define \d+ macros?\.$`),
}

// pageFaultStart and pageFaultEnd bound the performance indicators' page
// faults column in real MACRO's listing.
const pageFaultStart, pageFaultEnd = 25, 40

// recordCount finds the object record count in the source line count's
// line.
var recordCount = regexp.MustCompile(`producing (\d+) object records`)

// realClosingAllowed returns real MACRO's closing pages with the allowed
// differences applied, missing being the number of records its object
// has that govax's doesn't.
func realClosingAllowed(lines []string, missing int) []string {
	var out []string

	table := false

	for _, line := range lines {
		if matchesAny(listingDifferences, line) {
			continue
		}

		switch {
		case strings.HasPrefix(line, "Phase "):
			table = true
		case line == "":
			table = false
		}

		if table && len(line) > pageFaultEnd {
			line = line[:pageFaultStart] + line[pageFaultEnd:]
		}

		if m := recordCount.FindStringSubmatchIndex(line); m != nil {
			n, _ := strconv.Atoi(line[m[2]:m[3]])
			line = line[:m[2]] + strconv.Itoa(n-missing) + line[m[3]:]
		}

		out = append(out, line)
	}

	return out
}

// matchesAny reports whether any of res matches s.
func matchesAny(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}

	return false
}

// phaseTime matches a time in the performance indicators.
var phaseTime = regexp.MustCompile(`\d\d:\d\d:\d\d\.\d\d`)

// closingText returns closing pages' lines without their page headings,
// and with their times masked.
func closingText(lines []string) []string {
	var out []string

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "\f") {
			i += listHeadLines - 1

			continue
		}

		out = append(out, phaseTime.ReplaceAllString(lines[i], "hh:mm:ss.cc"))
	}

	return out
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

	got, _ = splitListing(t, got)

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
	got, _ := splitListing(t, a.Listing(ListingOptions{}))

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
