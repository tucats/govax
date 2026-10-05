package asm

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
// with the allowed differences withoutMacroInternals makes.
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

				check.allow = withoutMacroInternals(src)
			}

			checkListing(t, source, listing, libs, check)
		})
	}
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
