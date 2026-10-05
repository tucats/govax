package asm

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// This file checks the listing lines the assembler records (listing.go)
// against real MACRO's listings: for each line of a program's own source,
// where it was assembled and what it stored, as the listing's location
// column and binary field show them. How the lines are laid out on a
// page is a later step's; these tests compare the facts only.

// realListLine is one source line of a real MACRO listing.
type realListLine struct {
	number int
	// loc is the location column, when the line has one, and psect the
	// value a .PSECT line shows there instead (hasPsect).
	loc, psect uint32
	hasLoc     bool
	hasPsect   bool
	// binary is the binary field, with any continuation lines' fields
	// before it (they hold the higher addresses, and the field reads
	// right to left), and its blanks taken out.
	binary string
	text   string
}

// readRealListing returns the source lines of a real listing's source
// pages, the ones before the symbol table.
//
// A source line is the binary field (columns 0 to 35, its last column
// for a ' mark), a blank, the location (columns 37 to 40), the line
// number (41 to 46), a blank, and the line as written. A .PSECT line
// shows the psect's 8-digit location in columns 33 to 40. A line with
// no line number and a location continues the binary field of the line
// before.
func readRealListing(t *testing.T, path string) []realListLine {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var out []realListLine

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Symbol table ") {
			break
		}

		if len(line) < 41 {
			continue
		}

		number, numbered := 0, false
		if len(line) >= 47 {
			if n, err := strconv.Atoi(strings.TrimSpace(line[41:47])); err == nil {
				number, numbered = n, true
			}
		}

		field := line[:36]

		switch {
		case numbered && line[36] != ' ' && isHex(line[33:41]):
			out = append(out, realListLine{number: number, psect: hexValue(line[33:41]), hasPsect: true, text: rest(line)})

		case numbered && line[36] == ' ' && isHex(line[37:41]):
			out = append(out, realListLine{number: number, loc: hexValue(line[37:41]), hasLoc: true, binary: noBlanks(field), text: rest(line)})

		case !numbered && line[36] == ' ' && isHex(line[37:41]) && strings.TrimSpace(line[41:]) == "" && len(out) > 0:
			last := &out[len(out)-1]
			last.binary = noBlanks(field) + last.binary
		}
	}

	return out
}

func rest(line string) string {
	if len(line) <= 48 {
		return ""
	}

	return line[48:]
}

func isHex(s string) bool {
	if s == "" {
		return false
	}

	for _, ch := range s {
		if !strings.ContainsRune("0123456789ABCDEF", ch) {
			return false
		}
	}

	return true
}

func hexValue(s string) uint32 {
	v, _ := strconv.ParseUint(s, 16, 32)

	return uint32(v)
}

func noBlanks(s string) string { return strings.Join(strings.Fields(s), "") }

// binaryField renders l's fields as a listing's binary field with its
// blanks taken out: highest address first, each field's value in hex
// (most significant digit first), a ' after a field the linker stores,
// and G for the high digit of a general mode operand's mode byte.
func binaryField(a *Assembler, l *listLine) string {
	fields := a.listFields(l)

	var b strings.Builder

	for i := len(fields) - 1; i >= 0; i-- {
		f := fields[i]

		var hex strings.Builder
		for k := len(f.data) - 1; k >= 0; k-- {
			fmt.Fprintf(&hex, "%02X", f.data[k])
		}

		s := hex.String()

		switch f.mark {
		case markReloc:
			s += "'"
		case markGeneral:
			s = "G" + s[1:]
		}

		b.WriteString(s)
	}

	return b.String()
}

// programLines returns a's recorded lines of the program's own source,
// by line number.
func programLines(a *Assembler) map[int]*listLine {
	out := map[int]*listLine{}

	for _, l := range a.listLines {
		if l.depth == 0 && l.kind == sourceFile {
			out[l.line] = l
		}
	}

	return out
}

// compareListing checks a's recorded lines against the real listing at
// path, reporting every difference.
func compareListing(t *testing.T, a *Assembler, path string) {
	t.Helper()

	lines := programLines(a)
	real := readRealListing(t, path)

	if len(real) == 0 {
		t.Fatalf("%s: no source lines read", path)
	}

	for _, r := range real {
		l := lines[r.number]
		if l == nil {
			t.Errorf("line %d (%q): not recorded", r.number, r.text)

			continue
		}

		if l.text != r.text && !strings.HasPrefix(r.text, l.text) {
			t.Errorf("line %d: text %q, want %q", r.number, l.text, r.text)
		}

		switch {
		case r.hasPsect:
			if l.endLoc != r.psect {
				t.Errorf("line %d (%q): psect location %08X, want %08X", r.number, r.text, l.endLoc, r.psect)
			}

			continue

		case uint16(l.loc) != uint16(r.loc):
			t.Errorf("line %d (%q): location %04X, want %04X", r.number, r.text, l.loc, r.loc)
		}

		// A line of a definition being collected is the listing's
		// choice: the .ENDR of a repeat block lists its first
		// repetition's bytes (see repeatFirstLine).
		if l.collected {
			continue
		}

		var got string

		switch {
		case (l.op == "=" || l.op == ".MDELETE" || l.op == ".IF") && l.hasValue:
			got = fmt.Sprintf("%08X", l.value)
		case strings.HasPrefix(l.op, ".BLK"):
			got = fmt.Sprintf("%08X", l.endLoc)
		default:
			got = binaryField(a, l)
		}

		if got != r.binary {
			t.Errorf("line %d (%q): binary %q, want %q", r.number, r.text, got, r.binary)
		}
	}
}

// TestListingLines records the listing lines of every real listing's
// source whose listing uses MACRO's default listing options, and checks
// each line of the program's own source against the real listing.
func TestListingLines(t *testing.T) {
	marDir := filepath.Join("..", "..", "testdata", "mar")
	listDir := filepath.Join(marDir, "list")

	type fixture struct {
		name, source, listing string
		libraries             func(t *testing.T) []MacroLibrary
	}

	var cases []fixture

	starlet := func(t *testing.T) []MacroLibrary { return []MacroLibrary{govaxStarlet(t)} }

	ladder := ladderSources(t, marDir)

	for _, path := range ladder {
		name := strings.TrimSuffix(filepath.Base(path), ".mar")

		var libs func(t *testing.T) []MacroLibrary
		if usesStarlet[name] {
			libs = starlet
		}

		cases = append(cases, fixture{name, path, filepath.Join(marDir, "vax", name+".lis"), libs})
	}

	for _, name := range []string{"usermac", "libsub1", "libsub2", "libmain"} {
		cases = append(cases, fixture{name, filepath.Join(macrosDir, name+".mar"), filepath.Join(macrosDir, "vax", name+".lis"), nil})
	}

	// fabalign.mar's comments were rewritten after the VAX run, so its
	// lines no longer line up with the listing's.
	for _, name := range []string{"qiow", "rmscopy"} {
		cases = append(cases, fixture{name, filepath.Join(macrosDir, name+".mar"), filepath.Join(macrosDir, "vax", name+".lis"), starlet})
	}

	cases = append(cases, fixture{"uselib", filepath.Join(macrosDir, "uselib.mar"), filepath.Join(macrosDir, "vax", "uselib.lis"), func(t *testing.T) []MacroLibrary {
		return []MacroLibrary{openMacroFile(t, filepath.Join(macrosDir, "vax", "libmac.mlb"))}
	}})

	// The Phase 29 probe's sources listed with the default options.
	for _, name := range []string{"binary", "symtab", "notitle", "xref", "trace", "failmain", "failsub", "failsig"} {
		cases = append(cases, fixture{"list/" + name, filepath.Join(listDir, name+".mar"), filepath.Join(listDir, "vax", name+".lis"), nil})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(tc.source)
			if err != nil {
				t.Fatal(err)
			}

			a := macroAssembler()
			a.SetListing(true)

			if tc.libraries != nil {
				a.SetMacroLibraries(tc.libraries(t)...)
			}

			if _, err := a.Assemble(string(src)); err != nil {
				t.Fatalf("assemble: %v", err)
			}

			compareListing(t, a, tc.listing)
		})
	}
}

// recordListing assembles src with listing lines recorded, failing the
// test on an error unless wantErr.
func recordListing(t *testing.T, src string, wantErr bool) *Assembler {
	t.Helper()

	a := macroAssembler()
	a.SetListing(true)

	_, err := a.Assemble(src)
	if (err != nil) != wantErr {
		t.Fatalf("assemble: %v", err)
	}

	return a
}

// lineSummary is a recorded line's frame, state, and binary field, for
// comparing with what a test expects.
func lineSummary(a *Assembler, l *listLine) string {
	var flags []string

	if l.collected {
		flags = append(flags, "collected")
	}

	if l.skipped {
		flags = append(flags, "skipped")
	}

	if l.continued {
		flags = append(flags, "continued")
	}

	s := fmt.Sprintf("%d %d:%d %04X %s", l.kind, l.depth, l.line, l.loc, binaryField(a, l))
	if len(flags) > 0 {
		s += " " + strings.Join(flags, ",")
	}

	return s
}

func requireLines(t *testing.T, a *Assembler, want ...string) {
	t.Helper()

	got := make([]string, len(a.listLines))
	for i, l := range a.listLines {
		got[i] = lineSummary(a, l)
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestListingOffByDefault(t *testing.T) {
	a := macroAssembler()
	if _, err := a.Assemble("\t.PSECT\tD\n\t.BYTE\t1\n"); err != nil {
		t.Fatal(err)
	}

	if len(a.listLines) != 0 {
		t.Fatalf("recorded %d lines without SetListing", len(a.listLines))
	}
}

// TestListingFrames checks the frame of each line: the program's lines
// (kind 0, depth 0), a macro's expansion (kind 2) and a repeat block's
// repetitions (kind 3) one level down, and the lines of the definitions
// they were collected from. .MACRO and .REPEAT are statements; the lines
// after them, through .ENDM or .ENDR, are collected.
func TestListingFrames(t *testing.T) {
	a := recordListing(t, strings.Join([]string{
		"\t.PSECT\tD",
		"\t.MACRO\tTWO\tA",
		"\t.BYTE\tA",
		"\t.BYTE\tA+1",
		"\t.ENDM\tTWO",
		"\tTWO\t5",
		"\t.REPEAT\t2",
		"\t.WORD\t^X1234",
		"\t.ENDR",
		"; a comment",
		"",
	}, "\n"), false)

	requireLines(t, a,
		"0 0:1 0000 ",
		"0 0:2 0000 ",
		"0 0:3 0000  collected",
		"0 0:4 0000  collected",
		"0 0:5 0000  collected",
		"0 0:6 0000 ",
		"2 1:1 0000 05",
		"2 1:2 0001 06",
		"2 1:3 0002 ",
		"0 0:7 0002 ",
		"0 0:8 0002  collected",
		"0 0:9 0002  collected",
		"3 1:1 0002 1234",
		"3 1:2 0004 ",
		"3 1:1 0004 1234",
		"3 1:2 0006 ",
		"0 0:10 0006 ",
	)

	// Each expansion and repetition ends with its .ENDM or .ENDR line,
	// the directive taken out.
	if got := a.listLines[8].text; got != "\t" {
		t.Errorf("expansion end = %q, want %q", got, "\t")
	}

	// The expansion's lines are the definition's, with the argument
	// substituted.
	if got := a.listLines[6].text; got != "\t.BYTE\t5" {
		t.Errorf("expansion line 1 = %q, want %q", got, "\t.BYTE\t5")
	}

	// The call and the repeat block's end assembled their own statement,
	// or none.
	if l := a.listLines[5]; l.op != "TWO" || l.stmt == 0 {
		t.Errorf("call line: op %q, stmt %d", l.op, l.stmt)
	}
}

// TestListingConditionalsAndContinuation checks the lines a conditional
// leaves out and a statement continued over two lines.
func TestListingConditionalsAndContinuation(t *testing.T) {
	a := recordListing(t, strings.Join([]string{
		"\t.PSECT\tD",
		"\t.IF\tEQ 1",
		"\t.BYTE\t1",
		"\t.ENDC",
		"\t.LONG\t1, -",
		"\t\t2",
	}, "\n"), false)

	requireLines(t, a,
		"0 0:1 0000 ",
		"0 0:2 0000 ",
		"0 0:3 0000  skipped",
		"0 0:4 0000  skipped",
		"0 0:5 0000  continued",
		"0 0:6 0000 0000000200000001",
	)
}

// TestListingValues checks what a direct assignment, ". =", and a .PSECT
// leave for the listing to show.
func TestListingValues(t *testing.T) {
	a := recordListing(t, strings.Join([]string{
		"VAL = 42",
		"\t.PSECT\tD",
		"\t.BYTE\t1",
		"\t. = . + 7",
		"REL = . + 2",
		"\t.PSECT\tE",
		"\t.PSECT\tD",
	}, "\n"), false)

	l := a.listLines

	if !l[0].hasValue || l[0].value != 42 || l[0].valueSect != nil || l[0].op != "=" {
		t.Errorf("VAL = 42: value %v %d %v, op %q", l[0].hasValue, l[0].value, l[0].valueSect, l[0].op)
	}

	if !l[3].hasValue || l[3].value != 8 || l[3].valueSect == nil || l[3].endLoc != 8 {
		t.Errorf(". = . + 7: value %d, end %d", l[3].value, l[3].endLoc)
	}

	if !l[4].hasValue || l[4].value != 10 || l[4].valueSect == nil || l[4].valueSect.name != "D" {
		t.Errorf("REL = . + 2: value %d in %v", l[4].value, l[4].valueSect)
	}

	// Back in D, the .PSECT leaves the location where D left it.
	if l[6].op != ".PSECT" || l[6].endSect.name != "D" || l[6].endLoc != 8 {
		t.Errorf(".PSECT D: op %q, ends in %s at %d", l[6].op, l[6].endSect.name, l[6].endLoc)
	}
}

// TestListingMessages checks that each line keeps its own errors,
// warnings, and .PRINT messages, an error inside an expansion going to
// the expansion's line.
func TestListingMessages(t *testing.T) {
	a := recordListing(t, strings.Join([]string{
		"\t.PSECT\tD",
		"\t.BYTE\t300",
		"\t.WARN\t; careful",
		"\t.PRINT\t; hello",
		"\t.MACRO\tBAD",
		"\t.BYTE\t999",
		"\t.ENDM\tBAD",
		"\tBAD",
	}, "\n"), true)

	l := a.listLines

	if n := l[1].notes; len(n) != 1 || n[0].warning || !strings.Contains(n[0].err.Error(), "300") {
		t.Errorf(".BYTE 300: notes %v", n)
	}

	if n := l[2].notes; len(n) != 1 || !n[0].warning || len(l[3].messages) != 1 || l[3].messages[0] != " hello" {
		t.Errorf(".WARN: %v; .PRINT: %q", n, l[3].messages)
	}

	call, expansion := l[7], l[8]
	if len(call.notes) != 0 || expansion.kind != sourceMacro || len(expansion.notes) != 1 {
		t.Errorf("call notes %v; expansion (kind %d) notes %v", call.notes, expansion.kind, expansion.notes)
	}
}

// TestListingLibraryDefinitions checks that a macro library's definition
// isn't recorded, though its expansion is.
func TestListingLibraryDefinitions(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(macrosDir, "uselib.mar"))
	if err != nil {
		t.Fatal(err)
	}

	a := macroAssembler()
	a.SetListing(true)
	a.SetMacroLibraries(openMacroFile(t, filepath.Join(macrosDir, "vax", "libmac.mlb")))

	if _, err := a.Assemble(string(src)); err != nil {
		t.Fatal(err)
	}

	program, expansions := 0, 0

	for _, l := range a.listLines {
		switch {
		case l.kind == sourceLibrary:
			t.Fatalf("recorded a library definition's line %q", l.text)
		case l.depth == 0:
			program++
		case l.kind == sourceMacro:
			expansions++
		}
	}

	if want := len(strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")); program != want {
		t.Errorf("recorded %d program lines, want %d", program, want)
	}

	if expansions == 0 {
		t.Error("recorded no expansion lines")
	}
}
