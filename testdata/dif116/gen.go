// gen writes Phase 116's probe of the DIFFERENCES command: cases.txt (every
// input file and every case, the one record of what the probe asks, which
// govax's oracle tests read), the input files under in/ (gitignored, made
// again from cases.txt's data), the command procedures dif116.com and
// dif116t.com, and the govax console scripts exchange.cmd and copyout.cmd.
// Run it from the repository root:
//
//	go run testdata/dif116/gen.go
//
// See README.md, and docs/planning/PHASE-116 - DIFFERENCES.md's Part D.
package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/dif116"

// A file is one input file: its name in [DIF116.IN] (or a directory below
// it, written as a VMS directory prefix) and its records. Format says how
// exchange.cmd makes it: "" is variable-length records copied as text, "FIX
// n" fixed records of n bytes copied as binary (so a record may hold any
// byte), "VFC" and "STMLF" text made into those formats.
type file struct {
	Name    string
	Format  string
	Records []string
}

// A probeCase is one DIFFERENCES command. Command is the whole DCL line
// after the dollar sign, without the output qualifier. If Out is true the
// listing goes to [DIF116.OUT]ID.DIF (the procedure adds
// /OUTPUT=DIF$OUT:ID.DIF after the verb), else to the log. Pre and Post are
// DCL lines run around it.
type probeCase struct {
	ID      string
	Section string
	Title   string
	Command string
	Out     bool
	Pre     []string
	Post    []string
}

var (
	files []file
	cases []probeCase
	named = map[string]bool{}
)

// addFile adds an input file. A name may be added more than once: each is a
// new version, in order.
func addFile(name string, records ...string) {
	files = append(files, file{Name: name, Records: records})
	named[name] = true
}

// addFormatted adds a file of a record format other than variable.
func addFormatted(name, format string, records ...string) {
	files = append(files, file{Name: name, Format: format, Records: records})
	named[name] = true
}

// addCase adds a case whose listing goes to its own .DIF file.
func addCase(section, id, title, command string) {
	cases = append(cases, probeCase{ID: id, Section: section, Title: title, Command: command, Out: true})
}

// addLogCase adds a case whose listing (or messages) go to the log.
func addLogCase(section, id, title, command string) {
	cases = append(cases, probeCase{ID: id, Section: section, Title: title, Command: command})
}

// lines makes n records "prefix 1" to "prefix n".
func lines(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s %d", prefix, i+1)
	}

	return out
}

// edit returns records with what is at [from, to) replaced by with.
func edit(records []string, from, to int, with ...string) []string {
	out := append([]string{}, records[:from]...)
	out = append(out, with...)

	return append(out, records[to:]...)
}

func main() {
	layout()
	matching()
	forms()
	ignoring()
	limits()
	names()
	outputs()
	formats()
	generated()

	writeCases()
	writeInputs()
	writeProcedure()
	writeTerminal()
	writeExchange()
	writeCopyout()
}

// layout is A.3's first and fourth questions: the merged listing's bytes for
// each shape of difference, and identical files.
func layout() {
	const s = "LAYOUT"

	base := lines("line", 4)
	addFile("A01.TXT", base...)
	addFile("A01.TST", edit(base, 2, 2, "extra cat", "extra dog")...)
	addCase(s, "A01", "two records inserted (the author's example)", "DIFFERENCES A01.TXT A01.TST")
	addLogCase(s, "A01L", "the same, to the log", "DIFFERENCES A01.TXT A01.TST")

	ten := lines("record", 10)
	addFile("A02.A", ten...)
	addFile("A02.B", edit(ten, 4, 5, "record five changed")...)
	addCase(s, "A02", "one record changed in the middle", "DIFFERENCES A02.A A02.B")

	addFile("A03.A", ten...)
	addFile("A03.B", edit(ten, 4, 6)...)
	addCase(s, "A03", "two records deleted in the middle", "DIFFERENCES A03.A A03.B")

	addFile("A04.A", ten...)
	addFile("A04.B", edit(ten, 0, 1, "first changed")...)
	addCase(s, "A04", "the first record changed", "DIFFERENCES A04.A A04.B")

	addFile("A05.A", ten...)
	addFile("A05.B", edit(ten, 9, 10, "last changed")...)
	addCase(s, "A05", "the last record changed", "DIFFERENCES A05.A A05.B")

	addFile("A06.A", ten...)
	addFile("A06.B", edit(ten, 0, 0, "new first", "new second")...)
	addCase(s, "A06", "records inserted at the start", "DIFFERENCES A06.A A06.B")

	addFile("A07.A", ten...)
	addFile("A07.B", edit(ten, 10, 10, "appended 1", "appended 2")...)
	addCase(s, "A07", "records appended", "DIFFERENCES A07.A A07.B")

	addFile("A08.A", ten...)
	addFile("A08.B", ten[:7]...)
	addCase(s, "A08", "the last records deleted", "DIFFERENCES A08.A A08.B")

	addFile("A09.A", ten...)
	addFile("A09.B", ten[2:]...)
	addCase(s, "A09", "the first records deleted", "DIFFERENCES A09.A A09.B")

	addFile("A10.A")
	addFile("A10.B", ten[:3]...)
	addCase(s, "A10", "the master empty", "DIFFERENCES A10.A A10.B")

	addFile("A11.A", ten[:3]...)
	addFile("A11.B")
	addCase(s, "A11", "the revision empty", "DIFFERENCES A11.A A11.B")

	addFile("A12.A")
	addFile("A12.B")
	addCase(s, "A12", "both empty", "DIFFERENCES A12.A A12.B")
	addLogCase(s, "A12L", "both empty, to the log", "DIFFERENCES A12.A A12.B")

	addFile("A13.A", ten...)
	addFile("A13.B", ten...)
	addCase(s, "A13", "identical files", "DIFFERENCES A13.A A13.B")
	addLogCase(s, "A13L", "identical files, to the log", "DIFFERENCES A13.A A13.B")

	addFile("A14.A", lines("apple", 5)...)
	addFile("A14.B", lines("pear", 7)...)
	addCase(s, "A14", "nothing in common", "DIFFERENCES A14.A A14.B")

	twenty := lines("row", 20)
	addFile("A15.A", twenty...)
	b15 := edit(twenty, 15, 16, "row 16 changed")
	b15 = edit(b15, 3, 4, "row 4 changed")
	addFile("A15.B", b15...)
	addCase(s, "A15", "two sections", "DIFFERENCES A15.A A15.B")

	// Changes separated by one and by two matching records: fewer than
	// /MATCH's 3, so perhaps one section.
	addFile("A16.A", twenty...)
	addFile("A16.B", edit(edit(twenty, 8, 9, "row 9 changed"), 6, 7, "row 7 changed")...)
	addCase(s, "A16", "changes one record apart", "DIFFERENCES A16.A A16.B")

	addFile("A17.A", twenty...)
	addFile("A17.B", edit(edit(twenty, 9, 10, "row 10 changed"), 6, 7, "row 7 changed")...)
	addCase(s, "A17", "changes two records apart", "DIFFERENCES A17.A A17.B")

	addFile("A18.A", twenty...)
	addFile("A18.B", edit(edit(twenty, 10, 11, "row 11 changed"), 6, 7, "row 7 changed")...)
	addCase(s, "A18", "changes three records apart", "DIFFERENCES A18.A A18.B")

	// Records wider than the listing.
	long1 := strings.Repeat("0123456789", 20)
	addFile("A19.A", "short", long1, "tail 1", "tail 2", "tail 3")
	addFile("A19.B", "short", long1[:150]+"X"+long1[151:], "tail 1", "tail 2", "tail 3")
	addCase(s, "A19", "a 200-byte record changed", "DIFFERENCES A19.A A19.B")
	addCase(s, "A19W80", "the same at /WIDTH=80", "DIFFERENCES/WIDTH=80 A19.A A19.B")

	// Tabs, trailing blanks, and blank records, as they are.
	addFile("A20.A", "a\tb\tc", "trail  ", "", "   ", "x", "y", "z")
	addFile("A20.B", "a\tb\td", "trail", "", "    ", "x", "y", "z")
	addCase(s, "A20", "tabs, trailing blanks, blank records", "DIFFERENCES A20.A A20.B")

	// Line numbers past four and five digits.
	big := lines("record", 100010)
	addFile("A21.A", big...)
	addFile("A21.B", edit(edit(big, 100004, 100005, "changed near the end"), 9998, 9999, "changed at 9999")...)
	addCase(s, "A21", "line numbers of five and six digits", "DIFFERENCES A21.A A21.B")

	// A moved block.
	addFile("A22.A", twenty...)
	moved := append(append(append([]string{}, twenty[:3]...), twenty[10:13]...), twenty[3:10]...)
	moved = append(moved, twenty[13:]...)
	addFile("A22.B", moved...)
	addCase(s, "A22", "a block moved up", "DIFFERENCES A22.A A22.B")

	// Repeated records.
	addFile("A23.A", "x", "x", "x", "y", "x", "x", "x", "y", "x", "x")
	addFile("A23.B", "x", "x", "y", "x", "x", "x", "x", "y", "x", "x", "x")
	addCase(s, "A23", "repeated records", "DIFFERENCES A23.A A23.B")

	// A change and an insertion side by side.
	addFile("A24.A", twenty...)
	addFile("A24.B", edit(twenty, 5, 6, "row 6 changed", "inserted after 6")...)
	addCase(s, "A24", "a change with an insertion after it", "DIFFERENCES A24.A A24.B")
}

// matching is A.3's second question: /MATCH and /WINDOW.
func matching() {
	const s = "MATCHING"

	for _, n := range []string{"1", "2", "3", "5"} {
		addCase(s, "M16_"+n, "changes one record apart, /MATCH="+n, "DIFFERENCES/MATCH="+n+" A16.A A16.B")
		addCase(s, "M18_"+n, "changes three records apart, /MATCH="+n, "DIFFERENCES/MATCH="+n+" A18.A A18.B")
		addCase(s, "M23_"+n, "repeated records, /MATCH="+n, "DIFFERENCES/MATCH="+n+" A23.A A23.B")
	}

	addCase(s, "M01", "/MATCH larger than either file", "DIFFERENCES/MATCH=50 A02.A A02.B")

	// A big change, where /WINDOW decides whether DIFFERENCES finds the
	// files' common tail.
	thirty := lines("item", 30)
	addFile("M02.A", thirty...)
	addFile("M02.B", edit(thirty, 5, 5, lines("new", 8)...)...)

	for _, w := range []string{"1", "2", "3", "5", "8", "9", "20"} {
		addCase(s, "M02_W"+w, "eight records inserted, /WINDOW="+w, "DIFFERENCES/WINDOW="+w+" M02.A M02.B")
	}

	addFile("M03.A", thirty...)
	addFile("M03.B", edit(thirty, 5, 13, lines("other", 8)...)...)

	for _, w := range []string{"1", "4", "8", "9"} {
		addCase(s, "M03_W"+w, "eight records changed, /WINDOW="+w, "DIFFERENCES/WINDOW="+w+" M03.A M03.B")
	}

	addCase(s, "M04", "a block moved, /WINDOW=2", "DIFFERENCES/WINDOW=2 A22.A A22.B")
	addCase(s, "M05", "/MATCH=1/MERGED=1", "DIFFERENCES/MATCH=1/MERGED=1 A15.A A15.B")
}

// forms is A.3's fifth question: every listing form and several together.
func forms() {
	const s = "FORMS"

	// The base pair: three sections of a change, an insertion, a deletion.
	thirty := lines("data line", 30)
	b := edit(thirty, 25, 27)
	b = edit(b, 15, 15, "inserted one", "inserted two")
	b = edit(b, 4, 5, "data line 5, revised")
	addFile("F01.A", thirty...)
	addFile("F01.B", b...)

	f := func(id, title, quals string) {
		addCase(s, id, title, "DIFFERENCES"+quals+" F01.A F01.B")
	}

	f("F01", "the base pair, merged by default", "")
	f("F02", "/MERGED=0", "/MERGED=0")
	f("F03", "/MERGED=2", "/MERGED=2")
	f("F04", "/MERGED=3", "/MERGED=3")
	f("F05", "/MERGED=5/MATCH=5", "/MERGED=5/MATCH=5")
	f("F06", "/SEPARATED", "/SEPARATED")
	f("F07", "/SEPARATED=MASTER", "/SEPARATED=MASTER")
	f("F08", "/SEPARATED=REVISION", "/SEPARATED=REVISION")
	f("F09", "/SEPARATED=(MASTER,REVISION)", "/SEPARATED=(MASTER,REVISION)")
	f("F10", "/PARALLEL", "/PARALLEL")
	f("F11", "/PARALLEL=1", "/PARALLEL=1")
	f("F12", "/PARALLEL=3", "/PARALLEL=3")
	f("F13", "/PARALLEL/WIDTH=80", "/PARALLEL/WIDTH=80")
	f("F14", "/PARALLEL/WIDTH=40", "/PARALLEL/WIDTH=40")
	f("F15", "/NONUMBER", "/NONUMBER")
	f("F16", "/PARALLEL/NONUMBER", "/PARALLEL/NONUMBER")
	f("F17", "/WIDTH=80", "/WIDTH=80")
	f("F18", "/WIDTH=200", "/WIDTH=200")
	f("F19", "/MODE=HEXADECIMAL", "/MODE=HEXADECIMAL")
	f("F20", "/MODE=OCTAL", "/MODE=OCTAL")
	f("F21", "/MODE=(HEX,ASCII)", "/MODE=(HEX,ASCII)")
	f("F22", "/MODE=(OCTAL,HEX,ASCII)", "/MODE=(OCTAL,HEX,ASCII)")
	f("F23", "/MODE=HEX/WIDTH=80", "/MODE=HEX/WIDTH=80")
	f("F24", "/CHANGE_BAR on the verb", "/CHANGE_BAR")
	f("F25", "/CHANGE_BAR=# on the verb", "/CHANGE_BAR=#")
	f("F26", "/CHANGE_BAR=(#,NONUMBER)", "/CHANGE_BAR=(#,NONUMBER)")
	f("F27", "/CHANGE_BAR=(\"!\",NUMBER)/NONUMBER", "/CHANGE_BAR=(\"!\",NUMBER)/NONUMBER")
	f("F28", "/CHANGE_BAR/NONUMBER", "/CHANGE_BAR/NONUMBER")
	f("F29", "/CHANGE_BAR/MODE=HEX", "/CHANGE_BAR/MODE=HEX")
	f("F30", "/CHANGE_BAR/MERGED", "/CHANGE_BAR/MERGED")
	f("F31", "/MERGED/PARALLEL", "/MERGED/PARALLEL")
	f("F32", "/PARALLEL/MERGED", "/PARALLEL/MERGED")
	f("F33", "/SEPARATED/MERGED/PARALLEL/MODE=(HEX,ASCII)", "/SEPARATED/MERGED/PARALLEL/MODE=(HEX,ASCII)")
	f("F34", "/PARALLEL/MODE=HEX", "/PARALLEL/MODE=HEX")
	f("F35", "/SLP", "/SLP")
	f("F36", "/SEPARATED/MODE=OCTAL", "/SEPARATED/MODE=OCTAL")
	f("F37", "/PARALLEL/CHANGE_BAR", "/PARALLEL/CHANGE_BAR")

	addCase(s, "F40", "/CHANGE_BAR on the first file", "DIFFERENCES F01.A/CHANGE_BAR F01.B")
	addCase(s, "F41", "/CHANGE_BAR=# on the second file", "DIFFERENCES F01.A F01.B/CHANGE_BAR=#")
	addCase(s, "F42", "change bars on both files", "DIFFERENCES F01.A/CHANGE_BAR=% F01.B/CHANGE_BAR=#")
	addCase(s, "F43", "/CHANGE_BAR on the verb and the first file", "DIFFERENCES/CHANGE_BAR=# F01.A/CHANGE_BAR=% F01.B")
	addCase(s, "F44", "change bars on identical files", "DIFFERENCES/CHANGE_BAR A13.A A13.B")
	addCase(s, "F45", "/PARALLEL of identical files", "DIFFERENCES/PARALLEL A13.A A13.B")
	addCase(s, "F46", "/PARALLEL with 200-byte records", "DIFFERENCES/PARALLEL A19.A A19.B")
	addCase(s, "F47", "/PARALLEL with tabs and blanks", "DIFFERENCES/PARALLEL A20.A A20.B")
	addCase(s, "F48", "/MODE=HEX with 200-byte records", "DIFFERENCES/MODE=HEX A19.A A19.B")
	addCase(s, "F49", "/MODE=OCTAL with tabs and blanks", "DIFFERENCES/MODE=OCTAL A20.A A20.B")

	// /SLP on each shape.
	for _, p := range [][2]string{
		{"A01", "TXT TST"}, {"A02", "A B"}, {"A03", "A B"}, {"A04", "A B"}, {"A05", "A B"},
		{"A06", "A B"}, {"A07", "A B"}, {"A08", "A B"}, {"A09", "A B"}, {"A10", "A B"},
		{"A11", "A B"}, {"A12", "A B"}, {"A13", "A B"}, {"A14", "A B"}, {"A15", "A B"},
		{"A19", "A B"}, {"A20", "A B"}, {"A22", "A B"}, {"A23", "A B"},
	} {
		types := strings.Fields(p[1])
		addCase(s, "S"+p[0], "/SLP of "+p[0], fmt.Sprintf("DIFFERENCES/SLP %s.%s %s.%s", p[0], types[0], p[0], types[1]))
	}
}

// ignoring is A.3's sixth question: /IGNORE and /COMMENT_DELIMITER.
func ignoring() {
	const s = "IGNORING"

	addFile("I01.A", "Alpha Beta", "x", "  two  spaces", "tab\there", "trail   ", "", "end 1", "end 2", "end 3")
	addFile("I01.B", "alpha BETA", "x", " two spaces", "tab here", "trail", "", "", "end 1", "end 2", "end 3")

	ig := func(id, quals string) {
		addCase(s, id, quals, "DIFFERENCES"+quals+" I01.A I01.B")
	}

	ig("I01", "")
	ig("I02", "/IGNORE=CASE")
	ig("I03", "/IGNORE=SPACING")
	ig("I04", "/IGNORE=TRAILING_SPACES")
	ig("I05", "/IGNORE=BLANK_LINES")
	ig("I06", "/IGNORE=(CASE,SPACING,TRAILING_SPACES,BLANK_LINES)")
	ig("I07", "/IGNORE=(SPACING,EXACT)")
	ig("I08", "/IGNORE=(SPACING,EDITED)")
	ig("I09", "/IGNORE=(SPACING,PRETTY)")
	ig("I10", "/IGNORE=(CASE,EXACT)")
	ig("I11", "/IGNORE=PRETTY")
	ig("I12", "/IGNORE=(TRAILING_SPACES,BLANK_LINES)/PARALLEL")
	ig("I13", "/IGNORE=(CASE,SPACING)/SLP")
	ig("I14", "/IGNORE=SPACING/MODE=HEX")

	// Form feeds and headers.
	addFile("I20.A", "\f", "Title page 1", "Date 1", "body 1", "body 2", "a\fb", "body 3", "\fPage 2", "Date 2", "body 4", "body 5", "body 6")
	addFile("I20.B", "\f", "Title page 1 revised", "Date 9", "body 1", "body 2", "ab", "body 3", "\fPage 2", "Date 8", "body 4", "body 5", "body 6")

	ff := func(id, quals string) {
		addCase(s, id, quals, "DIFFERENCES"+quals+" I20.A I20.B")
	}

	ff("I20", "")
	ff("I21", "/IGNORE=FORM_FEEDS")
	ff("I22", "/IGNORE=HEADER")
	ff("I23", "/IGNORE=HEADER=1")
	ff("I24", "/IGNORE=HEADER=3")
	ff("I25", "/IGNORE=(HEADER,FORM_FEEDS)")
	ff("I26", "/IGNORE=(FORM_FEEDS,PRETTY)")
	ff("I27", "/PARALLEL")

	// A header whose first record is a form feed and something more.
	addFile("I28.A", "\fHeading", "Second", "Third", "body 1", "body 2", "body 3")
	addFile("I28.B", "\fHeading X", "Second X", "Third X", "body 1", "body 2", "body 3")
	addCase(s, "I28", "/IGNORE=HEADER on a form feed with text", "DIFFERENCES/IGNORE=HEADER I28.A I28.B")

	// Nonprinting characters, for PRETTY and /PARALLEL's formatting.
	addFile("I30.A", "bell\x07here", "esc\x1b[1m", "vt\x0bhere", "del\x7fhere", "eight\xa9bit", "same 1", "same 2", "same 3")
	addFile("I30.B", "bell here", "esc[1m", "vthere", "delhere", "eight bit", "same 1", "same 2", "same 3")
	addCase(s, "I30", "nonprinting characters, merged", "DIFFERENCES I30.A I30.B")
	addCase(s, "I31", "nonprinting characters, /IGNORE=PRETTY", "DIFFERENCES/IGNORE=PRETTY I30.A I30.B")
	addCase(s, "I32", "nonprinting characters, /PARALLEL", "DIFFERENCES/PARALLEL I30.A I30.B")
	addCase(s, "I33", "nonprinting characters, /MODE=HEX", "DIFFERENCES/MODE=HEX I30.A I30.B")

	// Comments: the same text under every file type the manual names, so
	// which characters each type takes, and where, shows.
	master := []string{
		"x = 1 ! bang", "y = 2 ; semi", "* star line", "/ slash line", "C cee line",
		"D dee line", "c lower cee", "d lower dee", "z = 3 * star mid", "w = 4 / slash mid",
		"v = 5 V vee", "u = 6 : colon", "t = 7 , comma", "s = 8 [left] r", "plain 1", "plain 2", "plain 3",
	}
	revision := []string{
		"x = 1 ! BANG CHANGED", "y = 2 ; SEMI CHANGED", "* STAR CHANGED", "/ SLASH CHANGED", "C CEE CHANGED",
		"D DEE CHANGED", "c LOWER CHANGED", "d LOWER CHANGED", "z = 3 * STAR CHANGED", "w = 4 / SLASH CHANGED",
		"v = 5 V VEE CHANGED", "u = 6 : COLON CHANGED", "t = 7 , COMMA CHANGED", "s = 8 [LEFT] R", "plain 1", "plain 2", "plain 3",
	}

	for _, t := range []string{"B2S", "B32", "BAS", "BLI", "CBL", "CMD", "COB", "COM", "COR", "FOR", "HLP", "MAC", "MAR", "R32", "REQ", "TXT", "DAT"} {
		addFile("C1."+t, master...)
		addFile("C2."+t, revision...)
		addCase(s, "IC_"+t, "/IGNORE=COMMENTS on ."+t, fmt.Sprintf("DIFFERENCES/IGNORE=COMMENTS C1.%s C2.%s", t, t))
		addCase(s, "ID_"+t, "/COMMENT_DELIMITER on ."+t, fmt.Sprintf("DIFFERENCES/COMMENT_DELIMITER C1.%s C2.%s", t, t))
	}

	cd := func(id, quals string) {
		addCase(s, id, quals, "DIFFERENCES"+quals+" C1.TXT C2.TXT")
	}

	cd("IE01", "/COMMENT_DELIMITER=V")
	cd("IE02", "/COMMENT_DELIMITER=\"V\"")
	cd("IE03", "/COMMENT_DELIMITER=(COLON,COMMA)")
	cd("IE04", "/COMMENT_DELIMITER=(EXCLAMATION,SEMI_COLON,SLASH)")
	cd("IE05", "/COMMENT_DELIMITER=(LEFT,RIGHT)")
	cd("IE06", "/COMMENT_DELIMITER=(\"!\",\"*\")")
	cd("IE07", "/COMMENT_DELIMITER=\"c\"")
	cd("IE08", "/COMMENT_DELIMITER=(c,C)")
	cd("IE09", "/COMMENT_DELIMITER=SP")
	cd("IE10", "/COMMENT_DELIMITER=\"V\"/IGNORE=EXACT")
	cd("IE11", "/COMMENT_DELIMITER=\"V\"/IGNORE=PRETTY")
	cd("IE12", "/COMMENT_DELIMITER=\"V\"/PARALLEL/WIDTH=80")
	cd("IE13", "/IGNORE=(COMMENTS,SPACING,TRAILING_SPACES,BLANK_LINES)/COMMENT_DELIMITER=\"!\"")

	// The manual's rule: a record of blanks and a comment is ignored as a
	// blank line once its comment and blanks are gone.
	addFile("I40.A", "one", "two", "three", "four", "five")
	addFile("I40.B", "one", "   ! just a comment", "two", "", "three", "four", "five")
	addCase(s, "I40", "comment-only and blank records, all ignored",
		"DIFFERENCES/IGNORE=(COMMENTS,TRAILING_SPACES,BLANK_LINES)/COMMENT_DELIMITER=\"!\" I40.A I40.B")
	addCase(s, "I41", "the same without TRAILING_SPACES",
		"DIFFERENCES/IGNORE=(COMMENTS,BLANK_LINES)/COMMENT_DELIMITER=\"!\" I40.A I40.B")

	// TAB and SPACE as comment characters, and FORM_FEED.
	addFile("I42.A", "key\told", "word other", "pre\fpost", "k1", "k2", "k3")
	addFile("I42.B", "key\tnew", "word else", "pre\fdiff", "k1", "k2", "k3")
	addCase(s, "I42", "/COMMENT_DELIMITER=(TAB,SPACE,FORM_FEED)", "DIFFERENCES/COMMENT_DELIMITER=(TAB,SPACE,FORM_FEED) I42.A I42.B")
}

// limits is A.3's third question: counting and /MAXIMUM_DIFFERENCES.
func limits() {
	const s = "LIMITS"

	forty := lines("entry", 40)
	b := edit(forty, 34, 35, "entry 35 x")
	b = edit(b, 24, 27, "entry 25 x", "entry 26 x", "entry 27 x")
	b = edit(b, 14, 16, "entry 15 x", "entry 16 x")
	b = edit(b, 4, 5, "entry 5 x")
	b = edit(b, 9, 9, "extra a", "extra b")
	addFile("L01.A", forty...)
	addFile("L01.B", b...)

	addCase(s, "L01", "five sections, no limit", "DIFFERENCES L01.A L01.B")

	for _, n := range []string{"1", "2", "3", "4", "6", "8", "9", "20"} {
		addCase(s, "L01_"+n, "/MAXIMUM_DIFFERENCES="+n, "DIFFERENCES/MAXIMUM_DIFFERENCES="+n+" L01.A L01.B")
	}

	addCase(s, "L02", "/MAXIMUM_DIFFERENCES=3 with three forms", "DIFFERENCES/MAXIMUM_DIFFERENCES=3/MERGED/PARALLEL/SEPARATED L01.A L01.B")
	addCase(s, "L03", "/MAXIMUM_DIFFERENCES=3/SLP", "DIFFERENCES/MAXIMUM_DIFFERENCES=3/SLP L01.A L01.B")
	addLogCase(s, "L04", "/MAXIMUM_DIFFERENCES=2, to the log", "DIFFERENCES/MAXIMUM_DIFFERENCES=2 L01.A L01.B")

	// Sections whose sides differ in size, for which side is counted.
	addFile("L05.A", edit(forty, 10, 11, "only one")...)
	addFile("L05.B", edit(forty, 10, 11, "one", "two", "three", "four")...)
	addCase(s, "L05", "one record against four", "DIFFERENCES L05.A L05.B")
	addCase(s, "L06", "four records against one", "DIFFERENCES L05.B L05.A")
}

// names is A.3's seventh question: the files, the defaults, and the errors.
func names() {
	const s = "NAMES"

	addFile("V01.TXT", "version one", "same 1", "same 2", "same 3")
	addFile("V01.TXT", "version two", "same 1", "same 2", "same 3")
	addFile("V03.TXT", "three, version one", "same")
	addFile("V03.TXT", "three, version two", "same")
	addFile("V03.TXT", "three, version three", "same")

	addCase(s, "N01", "one file: the next lower version", "DIFFERENCES V01.TXT")
	addCase(s, "N02", "one file, a version that has a lower one", "DIFFERENCES V03.TXT;2")
	addLogCase(s, "N03", "one file with no lower version", "DIFFERENCES V01.TXT;1")
	addCase(s, "N04", "the second's version from the first", "DIFFERENCES V01.TXT;1 V03.TXT")
	addCase(s, "N05", "the same file twice", "DIFFERENCES V01.TXT V01.TXT")
	addCase(s, "N06", "the second only a type", "DIFFERENCES A01.TXT .TST")
	addCase(s, "N07", "the second only a directory", "DIFFERENCES A01.TXT [-.IN]")
	addCase(s, "N08", "the second's directory from the first", "DIFFERENCES [DIF116.IN]A02.A A02.B")
	addCase(s, "N09", "a relative version", "DIFFERENCES V03.TXT;-1 V03.TXT;-2")
	addCase(s, "N10", "the abbreviation DIF", "DIF A02.A A02.B")
	addCase(s, "N11", "the abbreviation DIFF", "DIFF A02.A A02.B")
	addCase(s, "N12", "no file type", "DIFFERENCES V01 V03")

	e := func(id, title, command string) {
		addLogCase(s, id, title, command)
	}

	e("N20", "the first missing", "DIFFERENCES NOSUCH.TXT A01.TST")
	e("N21", "the second missing", "DIFFERENCES A01.TXT NOSUCH.TST")
	e("N22", "a missing directory", "DIFFERENCES [.NODIR]X.TXT A01.TST")
	e("N23", "a device not mounted", "DIFFERENCES DUA99:[X]Y.Z A01.TST")
	e("N24", "a wildcard in the first", "DIFFERENCES A*.TXT A01.TST")
	e("N25", "a wildcard in the second", "DIFFERENCES A01.TXT %01.TST")
	e("N27", "three parameters", "DIFFERENCES A02.A A02.B A03.A")
	e("N28", "the abbreviation DI", "DI A02.A A02.B")
	e("N29", "a directory file", "DIFFERENCES [-]IN.DIR [-]OUT.DIR")

	q := func(id, quals string) {
		addLogCase(s, id, quals, "DIFFERENCES"+quals+" A02.A A02.B")
	}

	q("N30", "/MATCH=0")
	q("N31", "/MATCH=-1")
	q("N32", "/MATCH=ABC")
	q("N33", "/WINDOW=0")
	q("N34", "/MERGED=4")
	q("N35", "/MERGED=-1")
	q("N36", "/PARALLEL=4")
	q("N37", "/WIDTH=0")
	q("N38", "/WIDTH=20")
	q("N39", "/WIDTH=1000")
	q("N40", "/MAXIMUM_DIFFERENCES=0")
	q("N41", "/SLP/MERGED")
	q("N42", "/SLP/PARALLEL")
	q("N43", "/SLP/SEPARATED")
	q("N44", "/SLP/CHANGE_BAR")
	q("N45", "/SLP/MODE=HEX")
	q("N46", "/MODE=BOGUS")
	q("N47", "/IGNORE=BOGUS")
	q("N48", "/SEPARATED=BOGUS")
	q("N49", "/CHANGE_BAR=(AB)")
	q("N50", "/COMMENT_DELIMITER=AB")
	q("N51", "/COMMENT_DELIMITER=(A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,S,T,U,V,W,X,Y,Z,\"1\",\"2\",\"3\",\"4\",\"5\",\"6\",\"7\")")
	q("N52", "/BOGUS")
	q("N53", "/MERGED=4/MATCH=4")
	q("N54", "/IGNORE=HEADER=0")
	q("N55", "/NOMERGED")
	q("N56", "/NOOUTPUT")
	q("N57", "/WINDOW=1/MATCH=2")

	// The closing echo's layout: long names and many qualifiers.
	long := "[DIF116.IN.A_LONG_DIRECTORY_NAME_OF_THIRTY_NINE_CH]"
	addFile(long+"A_FILE_NAME_OF_THIRTY_NINE_CHARACTERS_X.A_TYPE_OF_THIRTY_NINE_CHARACTERS_XXXXXX", "one", "two", "three", "four")
	addFile(long+"A_FILE_NAME_OF_THIRTY_NINE_CHARACTERS_X.B_TYPE_OF_THIRTY_NINE_CHARACTERS_XXXXXX", "one", "TWO", "three", "four")
	longPair := long + "A_FILE_NAME_OF_THIRTY_NINE_CHARACTERS_X.A_TYPE_OF_THIRTY_NINE_CHARACTERS_XXXXXX " +
		long + "A_FILE_NAME_OF_THIRTY_NINE_CHARACTERS_X.B_TYPE_OF_THIRTY_NINE_CHARACTERS_XXXXXX"
	addCase(s, "N60", "long names", "DIFFERENCES "+longPair)
	addCase(s, "N61", "long names at /WIDTH=80", "DIFFERENCES/WIDTH=80 "+longPair)
	addCase(s, "N62", "long names in parallel", "DIFFERENCES/PARALLEL "+longPair)
	addCase(s, "N63", "every qualifier in the echo",
		"DIFFERENCES/IGNORE=(BLANK_LINES,CASE,COMMENTS,FORM_FEEDS,HEADER=2,SPACING,TRAILING_SPACES,EXACT)"+
			"/COMMENT_DELIMITER=(\"!\",\";\",COLON,TAB)/MATCH=4/MERGED=2/PARALLEL=1/SEPARATED=MASTER/WINDOW=10"+
			"/MAXIMUM_DIFFERENCES=50/MODE=(ASCII,HEX)/NONUMBER/WIDTH=100/CHANGE_BAR=(\"$\",NUMBER) F01.A F01.B")
	addCase(s, "N64", "every qualifier at /WIDTH=80",
		"DIFFERENCES/IGNORE=(BLANK_LINES,CASE,COMMENTS,FORM_FEEDS,HEADER=2,SPACING,TRAILING_SPACES,EXACT)"+
			"/COMMENT_DELIMITER=(\"!\",\";\",COLON,TAB)/MATCH=4/MERGED=2/PARALLEL=1/SEPARATED=MASTER/WINDOW=10"+
			"/MAXIMUM_DIFFERENCES=50/MODE=(ASCII,HEX)/NONUMBER/WIDTH=80 F01.A/CHANGE_BAR=% F01.B/CHANGE_BAR=#")
	addCase(s, "N65", "the defaults written out", "DIFFERENCES/MATCH=3/MERGED=1/NUMBER/WIDTH=132/MODE=ASCII/IGNORE=EDITED F01.A F01.B")
}

// outputs is A.3's eighth question: where /OUTPUT puts the listing, and the
// listing file's attributes.
func outputs() {
	const s = "OUTPUT"

	pre := []string{"SET DEFAULT DIF$TOP:[DIF116.SCR]"}
	post := []string{
		"DIRECTORY DIF$TOP:[DIF116.SCR],DIF$TOP:[DIF116.IN]*.DIF",
		"DELETE DIF$TOP:[DIF116.SCR]*.*;*/NOLOG",
		"DELETE DIF$TOP:[DIF116.IN]*.DIF;*/NOLOG",
		"SET DEFAULT DIF$TOP:[DIF116.IN]",
	}

	o := func(id, title, command string) {
		cases = append(cases, probeCase{ID: id, Section: s, Title: title, Command: command, Pre: pre, Post: post})
	}

	o("O01", "/OUTPUT with no file", "DIFFERENCES/OUTPUT [-.IN]A02.A [-.IN]A02.B")
	o("O02", "/OUTPUT=name, no type", "DIFFERENCES/OUTPUT=LISTING [-.IN]A02.A [-.IN]A02.B")
	o("O03", "/OUTPUT=a directory", "DIFFERENCES/OUTPUT=[-.SCR] [-.IN]A02.A [-.IN]A02.B")
	o("O04", "/SLP with no /OUTPUT", "DIFFERENCES/SLP [-.IN]A02.A [-.IN]A02.B")
	o("O05", "/SLP/OUTPUT with no file", "DIFFERENCES/SLP/OUTPUT [-.IN]A02.A [-.IN]A02.B")
	o("O06", "/OUTPUT with no file, identical files", "DIFFERENCES/OUTPUT [-.IN]A13.A [-.IN]A13.B")
	o("O07", "/OUTPUT with no file, the first missing", "DIFFERENCES/OUTPUT [-.IN]NOSUCH.A [-.IN]A13.B")
	o("O08", "/OUTPUT, the qualifier after the files", "DIFFERENCES [-.IN]A02.A [-.IN]A02.B/OUTPUT")
	o("O09", "no /OUTPUT, SYS$OUTPUT defined to a file", "DIFFERENCES [-.IN]A02.A [-.IN]A02.B")
	cases[len(cases)-1].Pre = append(append([]string{}, pre...), "DEFINE/USER_MODE SYS$OUTPUT DIF$TOP:[DIF116.SCR]REDIRECT.LIS")

	// The attributes of a listing file and its bytes as records.
	addLogCase(s, "O20", "a listing's attributes and records",
		"DIFFERENCES/OUTPUT=DIF$OUT:O20.DIF A02.A A02.B")
	cases[len(cases)-1].Post = []string{
		"DIRECTORY/FULL DIF$OUT:O20.DIF",
		"DUMP/RECORD DIF$OUT:O20.DIF",
		"DUMP/RECORD DIF$OUT:F10.DIF",
		"DUMP/RECORD DIF$OUT:F19.DIF",
		"DUMP/RECORD DIF$OUT:F35.DIF",
		"DUMP/RECORD DIF$OUT:A20.DIF",
		"DUMP/RECORD DIF$OUT:I30.DIF",
		"DUMP/RECORD DIF$OUT:I32.DIF",
	}
}

// formats is A.3's ninth question: record formats other than variable, and
// records holding any byte.
func formats() {
	const s = "FORMATS"

	recs := lines("text", 12)
	changed := edit(recs, 5, 6, "text 6 changed")

	addFormatted("R01.VFC", "VFC", recs...)
	addFormatted("R02.VFC", "VFC", changed...)
	addCase(s, "R01", "VFC files", "DIFFERENCES R01.VFC R02.VFC")

	addFormatted("R01.STM", "STMLF", recs...)
	addFormatted("R02.STM", "STMLF", changed...)
	addCase(s, "R02", "STREAM_LF files", "DIFFERENCES R01.STM R02.STM")
	addCase(s, "R03", "a STREAM_LF file and a variable one", "DIFFERENCES R01.STM A02.A")
	addCase(s, "R04", "VFC against STREAM_LF, the same records", "DIFFERENCES R01.VFC R01.STM")

	pad := func(rs []string, n int) []string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r + strings.Repeat(" ", n-len(r))
		}

		return out
	}

	addFormatted("R01.FIX", "FIX 16", pad(recs, 16)...)
	addFormatted("R02.FIX", "FIX 16", pad(changed, 16)...)
	addCase(s, "R05", "fixed files", "DIFFERENCES R01.FIX R02.FIX")
	addCase(s, "R06", "fixed against variable, the same text", "DIFFERENCES R01.FIX R01.VFC")
	addCase(s, "R07", "fixed files, /IGNORE=TRAILING_SPACES", "DIFFERENCES/IGNORE=TRAILING_SPACES R01.FIX R01.VFC")

	// Every byte: control characters including CR and LF, and 8-bit ones.
	var bin1, bin2 []string
	for i := 0; i < 16; i++ {
		var r strings.Builder
		for j := 0; j < 16; j++ {
			r.WriteByte(byte(i*16 + j))
		}

		bin1 = append(bin1, r.String())
		bin2 = append(bin2, r.String())
	}

	bin2[0] = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0eX"
	bin2[8] = strings.Repeat("\x80", 16)
	bin2[15] = "\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfeX"
	addFormatted("R10.FIX", "FIX 16", bin1...)
	addFormatted("R11.FIX", "FIX 16", bin2...)
	addCase(s, "R10", "every byte, merged", "DIFFERENCES R10.FIX R11.FIX")
	addCase(s, "R11", "every byte, /IGNORE=PRETTY", "DIFFERENCES/IGNORE=PRETTY R10.FIX R11.FIX")
	addCase(s, "R12", "every byte, /PARALLEL", "DIFFERENCES/PARALLEL R10.FIX R11.FIX")
	addCase(s, "R13", "every byte, /MODE=(HEX,OCTAL)", "DIFFERENCES/MODE=(HEX,OCTAL) R10.FIX R11.FIX")
	addCase(s, "R14", "every byte, /SLP", "DIFFERENCES/SLP R10.FIX R11.FIX")
	addCase(s, "R15", "every byte, /IGNORE=(SPACING,TRAILING_SPACES,CASE)", "DIFFERENCES/IGNORE=(SPACING,TRAILING_SPACES,CASE) R10.FIX R11.FIX")
}

// generated is the matching rule's evidence: pairs made from a fixed seed,
// each run with the defaults, and some again with other qualifiers.
func generated() {
	const s = "GENERATED"

	rng := rand.New(rand.NewPCG(116, 2026))

	pick := func(words []string) string { return words[rng.IntN(len(words))] }

	few := []string{"alpha", "bravo", "charlie", "delta"}
	many := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet", "kilo", "lima"}

	// mutate applies k random edits.
	mutate := func(m []string, words []string, k int) []string {
		r := append([]string{}, m...)

		for ; k > 0; k-- {
			switch op := rng.IntN(6); {
			case op == 0 || len(r) == 0: // insert a run
				at := rng.IntN(len(r) + 1)
				var run []string
				for n := 1 + rng.IntN(3); n > 0; n-- {
					run = append(run, pick(words))
				}

				r = edit(r, at, at, run...)
			case op == 1: // delete a run
				at := rng.IntN(len(r))
				r = edit(r, at, min(len(r), at+1+rng.IntN(3)))
			case op == 2 || op == 3: // change one
				at := rng.IntN(len(r))
				r = edit(r, at, at+1, pick(words))
			case op == 4: // move a block
				at := rng.IntN(len(r))
				end := min(len(r), at+1+rng.IntN(3))
				block := append([]string{}, r[at:end]...)
				r = edit(r, at, end)
				to := rng.IntN(len(r) + 1)
				r = edit(r, to, to, block...)
			default: // duplicate one
				at := rng.IntN(len(r))
				r = edit(r, at, at, r[at])
			}
		}

		return r
	}

	for i := 1; i <= 240; i++ {
		words := few
		if i%2 == 0 {
			words = many
		}

		n := rng.IntN(15)
		if i%40 == 0 {
			n = 0
		}

		var m []string
		for j := 0; j < n; j++ {
			m = append(m, pick(words))
		}

		k := 1 + rng.IntN(4)
		if i%20 == 7 {
			k = 0
		}

		id := fmt.Sprintf("G%03d", i)
		addFile(id+".A", m...)
		addFile(id+".B", mutate(m, words, k)...)
		addCase(s, id, "generated", fmt.Sprintf("DIFFERENCES %s.A %s.B", id, id))
	}

	for i := 1; i <= 30; i++ {
		var m []string
		for j := 30 + rng.IntN(31); j > 0; j-- {
			m = append(m, pick(many))
		}

		id := fmt.Sprintf("H%02d", i)
		addFile(id+".A", m...)
		addFile(id+".B", mutate(m, many, 3+rng.IntN(6))...)
		addCase(s, id, "generated, longer", fmt.Sprintf("DIFFERENCES %s.A %s.B", id, id))
		addCase(s, id+"_W2", "generated, longer, /WINDOW=2", fmt.Sprintf("DIFFERENCES/WINDOW=2 %s.A %s.B", id, id))
		addCase(s, id+"_W5", "generated, longer, /WINDOW=5", fmt.Sprintf("DIFFERENCES/WINDOW=5 %s.A %s.B", id, id))
	}

	variant := func(prefix, quals string, from, to int) {
		for i := from; i <= to; i++ {
			id := fmt.Sprintf("G%03d", i)
			addCase(s, prefix+id, "generated, "+quals, fmt.Sprintf("DIFFERENCES%s %s.A %s.B", quals, id, id))
		}
	}

	variant("M1", "/MATCH=1", 1, 40)
	variant("M2", "/MATCH=2", 41, 80)
	variant("M5", "/MATCH=5", 81, 100)
	variant("P", "/PARALLEL", 101, 130)
	variant("S", "/SLP", 131, 170)
	variant("E", "/SEPARATED", 171, 190)
	variant("Z", "/MERGED=0", 191, 210)
	variant("X", "/MAXIMUM_DIFFERENCES=2", 211, 240)
}

// escape writes a record for cases.txt: backslash, a byte below a space or
// above a tilde as \xNN (but tab, \t), and a trailing blank as \x20, so the
// file is plain text an editor can't change.
func escape(r string) string {
	var b strings.Builder

	end := len(strings.TrimRight(r, " "))

	for i := 0; i < len(r); i++ {
		c := r[i]

		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < ' ' || c > '~' || (c == ' ' && i >= end):
			fmt.Fprintf(&b, `\x%02X`, c)
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}

// writeCases writes cases.txt, the probe's whole content: the files, in
// the order they are made (a name again is its next version), then the
// cases, in the order the procedure runs them.
func writeCases() {
	var b strings.Builder

	b.WriteString(`# Phase 116's probe of DIFFERENCES: every input file and every case.
# Written by testdata/dif116/gen.go; read by govax's oracle tests.
#
# file NAME [FORMAT]   an input file in [DIF116.IN] (a name again is its
#                      next version); its records follow, one per "|" line,
#                      with \\, \t, and \xNN escapes; FORMAT is FIX n, VFC,
#                      or STMLF (variable when absent)
# case ID SECTION OUT  a case: OUT is "file" (its listing in
#                      [DIF116.OUT]ID.DIF) or "log"; then
#   title TEXT         what it shows
#   pre COMMAND        a DCL line run before it
#   command COMMAND    the DIFFERENCES line, without /OUTPUT
#   post COMMAND       a DCL line run after it
#
`)

	for _, f := range files {
		fmt.Fprintf(&b, "file %s", f.Name)
		if f.Format != "" {
			fmt.Fprintf(&b, " %s", f.Format)
		}

		b.WriteString("\n")

		if len(f.Records) > 1000 {
			// A21's big files: written as a rule, not record by record.
			fmt.Fprintf(&b, "%s\n", bigRule(f))
			continue
		}

		for _, r := range f.Records {
			fmt.Fprintf(&b, "|%s\n", escape(r))
		}
	}

	for _, c := range cases {
		out := "log"
		if c.Out {
			out = "file"
		}

		fmt.Fprintf(&b, "case %s %s %s\n", c.ID, c.Section, out)
		fmt.Fprintf(&b, "  title %s\n", c.Title)

		for _, p := range c.Pre {
			fmt.Fprintf(&b, "  pre %s\n", p)
		}

		fmt.Fprintf(&b, "  command %s\n", c.Command)

		for _, p := range c.Post {
			fmt.Fprintf(&b, "  post %s\n", p)
		}
	}

	write("cases.txt", b.String())
}

// bigRule describes a big file as "record 1" to "record n" with some
// records changed: "=lines PREFIX N" then "=at K TEXT" for each change.
func bigRule(f file) string {
	n := len(f.Records)
	rule := fmt.Sprintf("=lines record %d", n)

	for i, r := range f.Records {
		if r != fmt.Sprintf("record %d", i+1) {
			rule += fmt.Sprintf("\n=at %d %s", i+1, escape(r))
		}
	}

	return rule
}

// hostName is the name exchange.cmd copies a file from: in/NNNN, its place
// in the file list.
func hostName(i int) string { return fmt.Sprintf("in/%04d", i+1) }

// writeInputs writes in/: each file as the host form exchange.cmd copies,
// text lines or, for a fixed file, its records' bytes end to end.
func writeInputs() {
	in := filepath.Join(dir, "in")
	if err := os.RemoveAll(in); err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(in, 0o755); err != nil {
		log.Fatal(err)
	}

	for i, f := range files {
		var b strings.Builder

		for _, r := range f.Records {
			b.WriteString(r)

			if !strings.HasPrefix(f.Format, "FIX") {
				if strings.ContainsAny(r, "\n") || strings.HasSuffix(r, "\r") {
					log.Fatalf("%s: a text record can't hold LF or end in CR", f.Name)
				}

				b.WriteString("\n")
			}
		}

		write(hostName(i), b.String())
	}
}

// procedureHead starts dif116.com.
const procedureHead = `$ ! DIF116.COM - Phase 116's probe of the DIFFERENCES command.
$ ! Written by testdata/dif116/gen.go. Run it as any user, with the
$ ! exchange volume mounted, from its [DIF116] directory:
$ !
$ !     $ SET DEFAULT DUA1:[DIF116]
$ !     $ @DIF116/OUTPUT=DIF116.LOG
$ !
$ ! It makes the listings in [DIF116.OUT], one per case, and writes each
$ ! case's status to the log. It changes nothing outside [DIF116...].
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT "######## ENVIRONMENT"
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME"), " ", F$GETSYI("NODENAME")
$ WRITE SYS$OUTPUT F$TIME()
$ TOP = F$PARSE("[000000]",,,"DEVICE")
$ DEFINE DIF$TOP 'TOP'
$ DEFINE DIF$OUT DIF$TOP:[DIF116.OUT]
$ SHOW LOGICAL DIF$TOP
$ IF F$SEARCH("DIF$TOP:[DIF116]OUT.DIR") .EQS. "" THEN CREATE/DIRECTORY DIF$TOP:[DIF116.OUT]
$ IF F$SEARCH("DIF$TOP:[DIF116]SCR.DIR") .EQS. "" THEN CREATE/DIRECTORY DIF$TOP:[DIF116.SCR]
$ DELETE DIF$TOP:[DIF116.OUT]*.*;*/NOLOG
$ DELETE DIF$TOP:[DIF116.SCR]*.*;*/NOLOG
$ SET DEFAULT DIF$TOP:[DIF116.IN]
$ SHOW DEFAULT
$ DIRECTORY/SIZE V0*.TXT
`

// writeProcedure writes dif116.com: each case under a heading, its status,
// and F$MESSAGE's text of it.
func writeProcedure() {
	var b strings.Builder

	b.WriteString(procedureHead)

	section := ""

	for _, c := range cases {
		if c.Section != section {
			section = c.Section
			fmt.Fprintf(&b, "$ !\n$ WRITE SYS$OUTPUT \"######## %s\"\n", section)
		}

		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"#### CASE %s: %s\"\n", c.ID, strings.ReplaceAll(c.Title, "\"", "\"\""))

		for _, p := range c.Pre {
			fmt.Fprintf(&b, "$ %s\n", p)
		}

		command := c.Command
		if c.Out {
			verb, rest, _ := strings.Cut(command, " ")
			command = fmt.Sprintf("%s/OUTPUT=DIF$OUT:%s.DIF %s", verb, c.ID, rest)
		}

		fmt.Fprintf(&b, "$ %s\n", dclLine(command))

		b.WriteString("$ S = $STATUS\n")
		b.WriteString("$ WRITE SYS$OUTPUT \"STATUS \", F$FAO(\"%X!XL\",S), \" \", F$MESSAGE(S)\n")

		for _, p := range c.Post {
			fmt.Fprintf(&b, "$ %s\n", p)
		}
	}

	b.WriteString(`$ !
$ WRITE SYS$OUTPUT "######## THE LISTINGS"
$ DIRECTORY/SIZE DIF$OUT:
$ SET DEFAULT DIF$TOP:[DIF116]
$ WRITE SYS$OUTPUT "DIF116 FINISHED"
$ SET NOVERIFY
$ EXIT
`)

	write("dif116.com", b.String())
}

// dclLine breaks a command longer than 250 characters into continuation
// lines at slashes, as DCL reads them.
func dclLine(command string) string {
	if len(command) <= 250 {
		return command
	}

	var out strings.Builder

	for len(command) > 200 {
		cut := strings.LastIndex(command[1:200], "/") + 1
		if cut == 0 {
			break
		}

		out.WriteString(command[:cut] + "-\n")
		command = command[cut:]
	}

	out.WriteString(command)

	return out.String()
}

// writeTerminal writes dif116t.com, the cases at a terminal, run by hand.
func writeTerminal() {
	write("dif116t.com", `$ ! DIF116T.COM - Phase 116's probe: DIFFERENCES at a terminal.
$ ! Written by testdata/dif116/gen.go. Run it at a terminal, from the
$ ! exchange volume's [DIF116] directory, and keep everything the
$ ! terminal shows (README.md, "The terminal"):
$ !
$ !     $ SET DEFAULT DUA1:[DIF116]
$ !     $ @DIF116T
$ !
$ SET NOON
$ SET VERIFY
$ WIDTH = F$GETDVI("TT","DEVBUFSIZ")
$ SET DEFAULT [.IN]
$ SET TERMINAL/WIDTH=80
$ DIFFERENCES F01.A F01.B
$ SHOW SYMBOL $STATUS
$ DIFFERENCES/WIDTH=132 F01.A F01.B
$ DIFFERENCES/PARALLEL F01.A F01.B
$ DIFFERENCES/MODE=HEX F01.A F01.B
$ DIFFERENCES A19.A A19.B
$ DIFFERENCES/PARALLEL A19.A A19.B
$ DIFFERENCES I30.A I30.B
$ DIFFERENCES A13.A A13.B
$ SHOW SYMBOL $STATUS
$ ! No parameters: answer the prompts with A02.A and A02.B.
$ DIFFERENCES
$ SHOW SYMBOL $STATUS
$ DIFFERENCES/OUTPUT=SYS$OUTPUT/WIDTH=132 A19.A A19.B
$ DIFFERENCES/OUTPUT=TT:/WIDTH=132 A19.A A19.B
$ SET TERMINAL/WIDTH=132
$ DIFFERENCES F01.A F01.B
$ DIFFERENCES/WIDTH=80 F01.A F01.B
$ DIFFERENCES/PARALLEL F01.A F01.B
$ DIFFERENCES/MODE=HEX F01.A F01.B
$ DIFFERENCES A19.A A19.B
$ DIFFERENCES/PARALLEL A19.A A19.B
$ ! Does VMS 7.1 have DIFFERENCES' paging qualifiers? Each, if accepted,
$ ! may page: press Return or Ctrl/Z to go on.
$ DIFFERENCES/PAGE A02.A A02.B
$ SHOW SYMBOL $STATUS
$ DIFFERENCES/PAGE=SCROLL A02.A A02.B
$ DIFFERENCES/SEARCH="record" A02.A A02.B
$ DIFFERENCES/WRAP A02.A A02.B
$ DIFFERENCES/HIGHLIGHT A02.A A02.B
$ DIFFERENCES/EXACT A02.A A02.B
$ SET TERMINAL/WIDTH='WIDTH'
$ SET DEFAULT [-]
$ SET NOVERIFY
`)
}

// writeExchange writes exchange.cmd, which builds the exchange volume.
func writeExchange() {
	var b strings.Builder

	b.WriteString(`! EXCHANGE.CMD - builds Phase 116's probe exchange volume with govax.
! Written by testdata/dif116/gen.go. Run from the repository root, after
! gen.go has written in/:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/dif116/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/dif116-exchange.dsk" /DEVICE=RD53 DIF116
MOUNT/WRITE DUA5 "testdata/disks/dif116-exchange.dsk"
CREATE/DIRECTORY DUA5:[DIF116]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[DIF116.IN]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[DIF116.OUT]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[DIF116.SCR]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[DIF116.IN.A_LONG_DIRECTORY_NAME_OF_THIRTY_NINE_CH]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
COPY "testdata/dif116/dif116.com"/HOST DUA5:[DIF116]DIF116.COM
COPY "testdata/dif116/dif116t.com"/HOST DUA5:[DIF116]DIF116T.COM
`)

	for i, f := range files {
		target := "DUA5:[DIF116.IN]" + f.Name
		if strings.HasPrefix(f.Name, "[") {
			target = "DUA5:" + f.Name
		}

		quals := "/HOST"

		switch {
		case f.Format == "VFC":
			quals += "/RECORD_FORMAT=VFC"
		case f.Format == "STMLF":
			quals += "/RECORD_FORMAT=STREAM_LF"
		case strings.HasPrefix(f.Format, "FIX "):
			quals += "/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=" + strings.TrimPrefix(f.Format, "FIX ")
		}

		fmt.Fprintf(&b, "COPY \"%s/%s\"%s %s/QUIET\n", dir, hostName(i), quals, target)
	}

	b.WriteString("DIRECTORY/SIZE DUA5:[DIF116...]\nDISMOUNT DUA5\n")

	write("exchange.cmd", b.String())
}

// writeCopyout writes copyout.cmd, which brings the run's results back.
func writeCopyout() {
	write("copyout.cmd", `! COPYOUT.CMD - copies Phase 116's probe results off dif116-exchange.dsk
! into testdata/dif116/vax (README.md). Written by testdata/dif116/gen.go.
! Run from the repository root after the VAX runs:
!
!     mkdir -p testdata/dif116/vax/out
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/dif116/copyout.cmd
!
MOUNT DUA5 "testdata/disks/dif116-exchange.dsk"
DIRECTORY/SIZE DUA5:[DIF116],[DIF116.OUT]
COPY DUA5:[DIF116]DIF116.LOG "testdata/dif116/vax/dif116.log"/HOST/QUIET
COPY DUA5:[DIF116.OUT]*.DIF "testdata/dif116/vax/out/"/HOST/QUIET
DISMOUNT DUA5
`)
}

// write writes one file under dir.
func write(name, text string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}
