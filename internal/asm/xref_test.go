package asm

import (
	"fmt"
	"strings"
	"testing"
)

// This file checks the cross reference (xref.go) on cases the probe's
// three real cross-reference listings don't show, which are govax's own
// choices (docs/PHASE-29.md, subtask 9). TestFixtureListings compares
// those three listings whole.

// xrefSection assembles src with a listing and /CROSS_REFERENCE=(kinds),
// and returns the lines of the cross-reference section titled title, from
// its column headings to the end of its entries.
func xrefSection(t *testing.T, src, title string, kinds ...string) []string {
	t.Helper()

	a := macroAssembler()
	a.SetListing(true)

	if err := a.SetCrossReference(true, kinds); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Assemble(src); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	lines := a.Listing(ListingOptions{})

	for i, line := range lines {
		if !strings.HasSuffix(line, "! "+title+" !") {
			continue
		}

		var out []string

		for _, l := range lines[i+3:] {
			if strings.HasPrefix(l, "\f") {
				break
			}

			if l == "" {
				if len(out) > 0 {
					break
				}

				continue
			}

			out = append(out, l)
		}

		return out
	}

	t.Fatalf("no %q section in the listing", title)

	return nil
}

// TestXrefWrapping checks a symbol with more references than fit on a
// line: five to a line (as many 16-column references as fit in the 132
// columns after the definition), each further line indented to the first
// reference's column. It also checks the order of references with line
// numbers of different lengths, which real MACRO lists in their text's
// order (line 38 before line 8 in xref.lis), so 10 and 100 come before 9.
func TestXrefWrapping(t *testing.T) {
	var src strings.Builder

	src.WriteString("\t.PSECT\tDATA\nVAL = 1\n")

	for line := 3; line <= 100; line++ {
		switch line {
		case 9, 10, 20, 30, 40, 50, 100:
			src.WriteString("\t.LONG\tVAL\n")
		default:
			src.WriteString("\n")
		}
	}

	src.WriteString("\t.END\n")

	got := xrefSection(t, src.String(), "Symbol Cross Reference")
	want := []string{
		"SYMBOL          VALUE        DEFINITION      REFERENCES... ",
		"------          -----        ----------      ------------- ",
		"VAL            =00000001     2      (1)      10     (1)      100    (1)      20     (1)      30     (1)      40     (1)    ",
		strings.Repeat(" ", 43) + "  50     (1)      9      (1)    ",
	}

	requireListed(t, got, want...)
}

// TestXrefLongNames checks a symbol name longer than 15 characters: the
// name column is 31 wide, as the symbol table's is.
func TestXrefLongNames(t *testing.T) {
	got := xrefSection(t, "A_SYMBOL_OF_20_CHARS = 2\nB = A_SYMBOL_OF_20_CHARS\n\t.END\n", "Symbol Cross Reference")
	want := []string{
		fmt.Sprintf("%-32s%-13s%-16s%s", "SYMBOL", "VALUE", "DEFINITION", "REFERENCES... "),
		fmt.Sprintf("%-32s%-13s%-16s%s", "------", "-----", "----------", "------------- "),
		fmt.Sprintf("%-31s=00000002     1      (1)      2      (1)    ", "A_SYMBOL_OF_20_CHARS"),
		fmt.Sprintf("%-31s=00000002     2      (1)    ", "B"),
	}

	requireListed(t, got, want...)
}

// TestXrefOnceALine checks that a line naming a symbol or register twice
// is one reference.
func TestXrefOnceALine(t *testing.T) {
	src := "\t.PSECT\tCODE\nV = 1\n\t.ENTRY\tGO, ^M<>\n\tADDL3\t#V, #V, R1\n\tMOVL\tR1, R1\n\tRET\n\t.END\n"

	requireListed(t, xrefSection(t, src, "Symbol Cross Reference")[2:],
		"GO              00000000-R   3      (1)    ",
		"V              =00000001     2      (1)    #-4      (1)    ",
	)

	requireListed(t, xrefSection(t, src, "Register Cross Reference", "REGISTERS")[2:],
		"R1                 2         #-4      (1)    #-5      (1)    ",
	)
}

// TestXrefNoCross checks .NOCROSS and .CROSS: with no symbols they turn
// everything off and on, directives and instructions included; .CROSS
// with no symbols leaves the symbols a .NOCROSS named off (the manual's
// note 1); and a .CROSS naming a symbol while everything is off takes
// effect once everything is on again (note 2). A .CROSS that turns
// everything on isn't listed itself, as xref.lis's line 23 isn't: it
// was off when the line began.
func TestXrefNoCross(t *testing.T) {
	src := strings.Join([]string{
		"\t.PSECT\tDATA",  // 1
		"A = 1",           // 2
		"B = 2",           // 3
		"\t.NOCROSS A, B", // 4
		"\t.NOCROSS",      // 5
		"\t.LONG\tA, B",   // 6
		"\t.CROSS B",      // 7
		"\t.CROSS",        // 8
		"\t.LONG\tA, B",   // 9
		"\t.END",          // 10
	}, "\n") + "\n"

	requireListed(t, xrefSection(t, src, "Symbol Cross Reference")[2:],
		"A              =00000001     2      (1)    ",
		"B              =00000002     3      (1)      9      (1)    ",
	)

	requireListed(t, xrefSection(t, src, "Directives Cross Reference", "DIRECTIVES")[2:],
		".END           10     (1)    ",
		".LONG          9      (1)    ",
		".NOCROSS       4      (1)    5      (1)    ",
		".PSECT         1      (1)    ",
	)
}

// TestXrefOpcodes checks the opcode cross reference: an instruction is
// listed by the name written (an alias too), and a two-byte opcode's
// value is its bytes as a word, the FD prefix low.
func TestXrefOpcodes(t *testing.T) {
	src := "\t.PSECT\tCODE\n\t.ENTRY\tGO, ^M<>\n\tCVTDH\tR0, R2\n\tBNEQU\t10$\n10$:\tRET\n\t.END\n"

	requireListed(t, xrefSection(t, src, "Opcode Cross Reference", "OPCODES")[2:],
		"BNEQU          0012      4      (1)    ",
		"CVTDH          32FD      3      (1)    ",
		"RET            0004      5      (1)    ",
	)
}

// TestXrefLibraryMacro checks a library macro in the macro cross
// reference: listed at its calls, with no definition line.
func TestXrefLibraryMacro(t *testing.T) {
	a := macroAssembler()
	a.SetListing(true)
	a.SetMacroLibraries(newMapLibrary(map[string]string{"LIBM": ".MACRO LIBM\n.BYTE 1\n.ENDM LIBM\n"}))

	if err := a.SetCrossReference(true, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Assemble("\t.PSECT\tDATA\n\tLIBM\n\tLIBM\n\t.END\n"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	text := strings.Join(a.Listing(ListingOptions{}), "\n")
	want := fmt.Sprintf("%-18s%-11d%17s   %-7d(1)       %-7d(1)    ", "LIBM", 1, "", 2, 3)

	if !strings.Contains(text, "\n"+want+"\n") {
		t.Errorf("no line %q in:\n%s", want, text)
	}
}

// TestXrefKeywords checks /CROSS_REFERENCE's keywords: NONE, an unknown
// keyword, and no cross reference unless one is asked for.
func TestXrefKeywords(t *testing.T) {
	a := macroAssembler()

	if err := a.SetCrossReference(true, []string{"SYMBOLS", "BOGUS"}); err == nil {
		t.Error("BOGUS accepted")
	}

	listing := func(on bool, kinds ...string) string {
		t.Helper()

		a := macroAssembler()
		a.SetListing(true)

		if err := a.SetCrossReference(on, kinds); err != nil {
			t.Fatal(err)
		}

		if _, err := a.Assemble("A = 1\nB = A\n\t.END\n"); err != nil {
			t.Fatal(err)
		}

		return strings.Join(a.Listing(ListingOptions{}), "\n")
	}

	for _, tc := range []struct {
		name string
		text string
		xref bool
	}{
		{"default", listing(true), true},
		{"NONE", listing(true, "NONE"), false},
		{"ALL then NONE", listing(true, "ALL", "NONE"), false},
		{"off", listing(false, "ALL"), false},
	} {
		if got := strings.Contains(tc.text, labelCrossReference); got != tc.xref {
			t.Errorf("%s: cross reference written %v, want %v", tc.name, got, tc.xref)
		}
	}

	// Without a listing, nothing is recorded (and nothing fails).
	b := macroAssembler()
	if err := b.SetCrossReference(true, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := b.Assemble("A = 1\n\t.LONG A\n\t.END\n"); err != nil {
		t.Fatal(err)
	}
}
