package asm

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
)

// This file checks the listings of the Phase 28 macro fixtures
// (testdata/mar/macros) against real MACRO's, whole: macro calls, library
// macros, repeat blocks, and conditionals, listed as MACRO's default
// options list them.

// systemLibraryName is the system macro library's specification, as real
// MACRO's listings name it.
const systemLibraryName = "SYS$COMMON:[SYSLIB]STARLET.MLB;1"

// TestMacroFixtureListings compares the listing govax writes for each
// Phase 28 fixture, with the macro libraries real MACRO searched, with
// real MACRO's, as TestFixtureListings does. govax's own system library
// stands in for VMS's, so a fixture that calls system macros is compared
// with the allowed differences systemMacroDifferences makes.
func TestMacroFixtureListings(t *testing.T) {
	vaxDir := filepath.Join(macrosDir, "vax")

	type library func(t *testing.T) MacroLibrary

	// file is a library real MACRO searched, by the specification its
	// listing shows.
	file := func(path, name string) library {
		return func(t *testing.T) MacroLibrary {
			return NamedMacroLibrary(openMacroFile(t, filepath.Join(vaxDir, path)), name)
		}
	}

	system := func(t *testing.T) MacroLibrary {
		return NamedMacroLibrary(govaxStarlet(t), systemLibraryName)
	}

	cases := []struct {
		name, source string
		libraries    []library
		// systemMacros says the fixture calls system macros.
		systemMacros bool
		// fromListing says the fixture's source was edited after the
		// VAX run (fabalign.mar's comments were rewritten, which moved
		// its line numbers), so the source assembled is the one the
		// listing shows (sourceFromListing).
		fromListing bool
	}{
		{"usermac", "usermac", []library{system}, false, false},
		{"qiow", "qiow", []library{system}, true, false},
		{"rmscopy", "rmscopy", []library{system}, true, false},
		{"fabalign", "fabalign", []library{system}, true, true},
		{"uselib", "uselib", []library{file("libmac.mlb", "DUA1:[000000]LIBMAC.MLB;1"), system}, false, false},
		{"gv_uselibm", "uselib", []library{file("gv_libmac.mlb", "DUA1:[000000]GV_LIBMAC.MLB;1"), system}, false, false},
		{"libsub1", "libsub1", []library{system}, false, false},
		{"libsub2", "libsub2", []library{system}, false, false},
		{"libmain", "libmain", []library{system}, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var libs []MacroLibrary
			for _, lib := range tc.libraries {
				libs = append(libs, lib(t))
			}

			source := filepath.Join(macrosDir, tc.source+".mar")
			listing := filepath.Join(vaxDir, tc.name+".lis")

			var check listingCheck

			if tc.fromListing {
				check.source = sourceFromListing(t, listing)
			}

			if tc.systemMacros {
				src := check.source
				if src == "" {
					data, err := os.ReadFile(source)
					if err != nil {
						t.Fatal(err)
					}

					src = string(data)
				}

				check.allow = systemMacroDifferences(t, src)
			}

			checkListing(t, source, listing, libs, check)
		})
	}
}

// TestListingDefaultsSection compares the first section of the probe's
// lctl.mar, which is listed with MACRO's default options, with real
// MACRO's listing of it, line for line without the page headings: macro
// calls, a nested call, macros with conditionals and with no bytes,
// repeat blocks, and the program's own conditionals (true, false, .IIF,
// and the subconditionals). The rest of lctl.mar, and the table of
// contents and subtitles in its headings, are the listing controls'
// (subtask 7).
func TestListingDefaultsSection(t *testing.T) {
	listDir := filepath.Join("..", "..", "testdata", "mar", "list")

	src, err := os.ReadFile(filepath.Join(listDir, "lctl.mar"))
	if err != nil {
		t.Fatal(err)
	}

	a := recordListing(t, string(src), false)

	// lctl.mar's line 71 starts the next section.
	const end = 71

	got := numberedSection(a.Listing(ListingOptions{}), end)
	want := numberedSection(readListing(t, filepath.Join(listDir, "vax", "lctl.lis")), end)

	if len(want) < end-1 {
		t.Fatalf("%d lines read from the real listing, want at least %d", len(want), end-1)
	}

	compareListingLines(t, got, want)
}

// TestListingRepeatsAndMdelete checks, on lines no real listing has:
// .MDELETE counts only the macros it deleted; an .ENDR shows the bytes of
// its block's first repetition's first line, with the block's location,
// even when they need a continuation line, and none when that line
// stores nothing; and an empty repeat block shows none.
func TestListingRepeatsAndMdelete(t *testing.T) {
	a := recordListing(t, `	.PSECT	D
	.MACRO	A
	.ENDM	A
	.MACRO	B
	.ENDM	B
	.MDELETE A, B, C
	.MDELETE A
	.REPEAT	2
	.BYTE	1,2,3,4,5,6,7,8,9,10,11,12,13
	.ENDR
	.IRP	X, <1, 2>
	; nothing
	.BYTE	X
	.ENDR
	.REPEAT	0
	.BYTE	1
	.ENDR
`, false)

	got, _ := splitListing(t, a.Listing(ListingOptions{}))

	want := []string{
		"                           00000002  0000     6 \t.MDELETE A, B, C",
		"                           00000000  0000     7 \t.MDELETE A",
		"                                     0000     8 \t.REPEAT\t2",
		"                                     0000     9 \t.BYTE\t1,2,3,4,5,6,7,8,9,10,11,12,13",
		"0C 0B 0A 09 08 07 06 05 04 03 02 01  0000    10 \t.ENDR",
		"                                 0D  000C       ",
		"                                     001A    11 \t.IRP\tX, <1, 2>",
		"                                     001A    12 \t; nothing",
		"                                     001A    13 \t.BYTE\tX",
		"                                     001A    14 \t.ENDR",
		"                                     001C    15 \t.REPEAT\t0",
		"                                     001C    16 \t.BYTE\t1",
		"                                     001C    17 \t.ENDR",
	}

	got = got[listHeadLines+5:]
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("listing:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// numberedSection returns a listing's lines without its page headings,
// from source line 1 up to source line end: each numbered line, and the
// unnumbered lines that follow one (continuation lines, a .PRINT's).
func numberedSection(lines []string, end int) []string {
	var out []string

	started := false

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "\f") {
			i += listHeadLines - 1

			continue
		}

		line := lines[i]

		n := 0
		if len(line) >= 47 {
			n, _ = strconv.Atoi(strings.TrimSpace(line[41:47]))
		}

		switch {
		case n == 1:
			started = true
		case n >= end:
			return out
		}

		if started {
			out = append(out, line)
		}
	}

	return out
}

// getsLine matches the macro library statistics' count of GETs and
// macros, and macroCount a library's count of macros.
var (
	getsLine   = regexp.MustCompile(`^\d+ GETS were required to define \d+ macros`)
	macroCount = regexp.MustCompile(`\d+( +)$`)
)

// systemMacroDifferences returns the allowed differences between the
// closing pages of a listing of src, a program that calls system macros,
// from govax's system macro library and from VMS's. govax's macros are
// written clean room (docs/PHASE-32.md), so they work differently inside:
//
//   - They use symbols of their own: a symbol whose name begins "$$" is
//     left out of both symbol tables.
//   - They refer to different $xxxDEF symbols (govax's $FAB builds its
//     options from FAB$M_ masks where VMS's uses the FAB$V_ bit numbers),
//     and a listing shows the ones referred to: a symbol a $xxxDEF macro
//     defines is left out of both tables unless src names it itself.
//   - Their definitions are split into helper macros differently, and
//     are a different number of lines: the library statistics' counts of
//     the system library's macros (and the totals) and of GETs are
//     masked.
func systemMacroDifferences(t *testing.T, src string) func([]string) []string {
	t.Helper()

	defs := definitionSymbols(t)
	named := map[string]bool{}

	for _, name := range strings.FieldsFunc(strings.ToUpper(src), func(ch rune) bool { return ch > 0x7F || !isSymbolChar(byte(ch)) }) {
		named[name] = true
	}

	return func(lines []string) []string {
		var out []string

		// The symbol table comes first, and ends at the first blank line
		// (the closing pages' headings are already gone).
		table := true

		for _, line := range lines {
			if line == "" {
				table = false
			}

			if table {
				name := strings.Fields(line)[0]
				if strings.HasPrefix(name, "$$") || defs[name] && !named[name] {
					continue
				}
			}

			if strings.HasPrefix(line, systemLibraryName+" ") || strings.HasPrefix(line, "TOTALS ") {
				line = macroCount.ReplaceAllString(line, "n$1")
			}

			out = append(out, getsLine.ReplaceAllString(line, "n GETS were required to define n macros"))
		}

		return out
	}
}

// definitionAssignment matches a symbol definition in govax's $xxxDEF
// macros.
var definitionAssignment = regexp.MustCompile(`^([A-Z0-9$_]+) ==? \^X`)

// definitionSymbols returns the names govax's $xxxDEF macros define.
func definitionSymbols(t *testing.T) map[string]bool {
	t.Helper()

	data, err := fs.ReadFile(bootdata.FS, bootdata.StarletDefSource)
	if err != nil {
		t.Fatal(err)
	}

	out := map[string]bool{}

	for _, line := range strings.Split(string(data), "\n") {
		if m := definitionAssignment.FindStringSubmatch(line); m != nil {
			out[m[1]] = true
		}
	}

	return out
}

// sourceFromListing returns the source a real listing lists: each
// numbered line's text, in order. Under MACRO's default listing options
// every line of the source is listed, as written, tabs kept.
func sourceFromListing(t *testing.T, path string) string {
	t.Helper()

	pages, _ := splitListing(t, readListing(t, path))

	var b strings.Builder

	next := 1

	for i, line := range pages {
		// A page's heading isn't the source's.
		if strings.HasPrefix(line, "\f") || i > 0 && strings.HasPrefix(pages[i-1], "\f") || len(line) < 47 {
			continue
		}

		n, err := strconv.Atoi(strings.TrimSpace(line[41:47]))
		if err != nil {
			continue
		}

		if n != next {
			t.Fatalf("%s: line %d where %d was expected", path, n, next)
		}

		next++

		b.WriteString(rest(line))
		b.WriteString("\n")
	}

	return b.String()
}
