package asm

import (
	"strings"
	"testing"
)

// This file checks the listing controls (listctl.go) on cases the
// probe's lctl.mar doesn't cover. TestFixtureListings compares lctl.mar's
// three real listings whole.

// listedText assembles src with show and noshow as /SHOW= and /NOSHOW=,
// and returns the text of each line its source pages list (the part after
// the line number), without the page headings.
func listedText(t *testing.T, src string, show, noshow []string) []string {
	t.Helper()

	a := macroAssembler()
	a.SetListing(true)

	if err := a.SetListingShow(show, noshow); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Assemble(src); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	pages, _ := splitListing(t, a.Listing(ListingOptions{}))

	var out []string

	for i := 0; i < len(pages); i++ {
		if strings.HasPrefix(pages[i], "\f") {
			i += listHeadLines - 1

			continue
		}

		out = append(out, rest(pages[i]))
	}

	return out
}

// requireListed reports a difference between got and want.
func requireListed(t *testing.T, got []string, want ...string) {
	t.Helper()

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("listed:\n%q\nwant:\n%q", got, want)
	}
}

// TestListingLevels checks .SHOW and .NOSHOW without arguments, which
// are .LIST and .NLIST: below level 0 nothing is listed; above it every
// line is, a macro's expansion too (unconfirmed by a real listing), and
// the directive's own line is listed only when it leaves the level above
// 0.
func TestListingLevels(t *testing.T) {
	got := listedText(t, `	.PSECT	D
	.MACRO	ONE
	.BYTE	1
	.ENDM	ONE
	.NOSHOW
	.BYTE	2
	.SHOW
	.SHOW
	ONE
	.NLIST
	ONE
`, nil, nil)

	requireListed(t, got,
		"\t.PSECT\tD",
		"\t.MACRO\tONE",
		"\t.BYTE\t1",
		"\t.ENDM\tONE",
		"\t.SHOW",
		"\tONE",
		"\t.BYTE\t1",
		"\t",
		"\tONE",
	)
}

// TestListingOptionErrors checks that an option name MACRO doesn't have
// is an error, in .SHOW and in SetListingShow.
func TestListingOptionErrors(t *testing.T) {
	a := macroAssembler()
	if _, err := a.Assemble("\t.SHOW\tEXPANSIONS, SYMBOLS\n"); err == nil {
		t.Error(".SHOW SYMBOLS: no error")
	}

	if err := a.SetListingShow(nil, []string{"SYMBOLS"}); err == nil {
		t.Error("SetListingShow SYMBOLS: no error")
	}

	if err := a.SetListingShow([]string{"me", "MEB"}, []string{"Cnd"}); err != nil {
		t.Errorf("SetListingShow: %v", err)
	}
}

// TestListingExpansionEnds checks the line an expansion ends with: an
// .ENDM in the first column ends it with an empty line, and a labeled
// .ENDM with its label, and no other line.
func TestListingExpansionEnds(t *testing.T) {
	got := listedText(t, `	.PSECT	D
	.MACRO	A
	.BYTE	1
.ENDM	A
	.MACRO	B
	.BYTE	2
L1:	.ENDM	B
	A
	B
`, []string{"EXPANSIONS"}, []string{"DEFINITIONS"})

	requireListed(t, got,
		"\t.PSECT\tD",
		"\tA",
		"\t.BYTE\t1",
		"",
		"\tB",
		"\t.BYTE\t2",
		"L1:",
	)
}

// TestListingPageAtStart checks that a .PAGE before anything is listed
// starts no empty page, and that .SBTTL's text heads its own page when
// it's the page's first line, and the pages after.
func TestListingPageAtStart(t *testing.T) {
	a := recordListing(t, "\t.PAGE\n\t.SBTTL\tPart one\n\t.PAGE\n\t.PSECT\tD\n", false)

	pages, _ := splitListing(t, a.Listing(ListingOptions{}))

	var headings []string

	for i, line := range pages {
		if strings.HasPrefix(line, "\f") {
			label := pages[i+1]
			if len(label) > headDateColumn {
				label = label[:headDateColumn]
			}

			headings = append(headings, strings.TrimRight(label, " "))
		}
	}

	// The table of contents, the page .SBTTL starts (which it heads),
	// and the page after the second .PAGE.
	want := []string{"Table of contents", "                                Part one", "                                Part one"}
	if strings.Join(headings, "|") != strings.Join(want, "|") {
		t.Errorf("headings %q, want %q", headings, want)
	}
}
