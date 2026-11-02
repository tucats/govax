package fortran

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/fortran/source"
)

// testOptions are fixed options, so the header doesn't change between
// runs.
var testOptions = Options{
	Version:     "1.0-1",
	Compiled:    time.Date(2026, time.October, 29, 14, 5, 11, 200_000_000, time.UTC),
	CommandLine: "FORTRAN SQUARES",
}

// TestCompileSquares checks the MACRO file for a small program: the
// header, then the code, each source line before the instructions it
// became, and .END naming the main program.
func TestCompileSquares(t *testing.T) {
	file := File{
		Spec: "DUA0:[WORK]SQUARES.FOR;1",
		Name: "SQUARES",
		Lines: []string{
			"      PROGRAM SQUARES",
			"C     Squares of the first five integers",
			"      DO 10 I = 1, 5",
			"\t PRINT *, I, I*I",
			"10    CONTINUE",
			"      END",
		},
	}

	r := Compile(testOptions, file)
	if r.Errors != 0 || r.Warnings != 0 || len(r.Messages) != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	header := []string{
		"\t.TITLE\tSQUARES",
		"\t.IDENT\t/01/",
		";",
		"; Compiled by govax FORTRAN V1.0-1 on 29-OCT-2026 14:05:11.20",
		"; from DUA0:[WORK]SQUARES.FOR;1",
		"; by the command FORTRAN SQUARES",
		";",
		"; Each line of the FORTRAN source appears as a comment, with its line number, before",
		"; the instructions it became. An INCLUDE file's lines appear only with INCLUDE's",
		"; /LIST, their include depth before the number; a comment names the file either way.",
		"; Each routine's storage map says where its variables are.",
		";",
	}

	if got := strings.Join(r.MACRO[:len(header)], "\n"); got != strings.Join(header, "\n") {
		t.Errorf("header:\n%s\nwant:\n%s", got, strings.Join(header, "\n"))
	}

	code := []string{
		";    1        PROGRAM SQUARES",
		"\t.ENTRY\tSQUARES,^M<>",
		";    2  C     Squares of the first five integers",
		";    3        DO 10 I = 1, 5",
		"\tMOVL\t#5,SQUARES.$C1\t\t; DO 10's count",
		"\tMOVL\t#1,SQUARES.I",
		"SQUARES.$1:\t\t\t\t; DO 10's body",
		";    4  \t PRINT *, I, I*I",
		"\tPUSHL\t#-1",
		"\tPUSHAB\tSQUARES.$IO1",
		"\tCALLS\t#2,G^FTN$WRITE_SL",
		"\tPUSHAQ\tSQUARES.$D1",
		"\tCALLS\t#1,G^FTN$IO_ITEM",
		"\tMULL3\tSQUARES.I,SQUARES.I,SQUARES.$T1",
		"\tPUSHAQ\tSQUARES.$D2",
		"\tCALLS\t#1,G^FTN$IO_ITEM",
		"\tCALLS\t#0,G^FTN$IO_END",
		";    5  10    CONTINUE",
		"SQUARES.L10:",
		"\tINCL\tSQUARES.I",
		"\tDECL\tSQUARES.$C1\t\t; one pass fewer",
		"\tBLEQ\tSQUARES.$2",
		"\tBRW\tSQUARES.$1",
		"SQUARES.$2:",
		";    6        END",
		"\tMOVL\t#1,R0\t\t\t; SS$_NORMAL",
		"\tRET",
		"",
		"\t.END\tSQUARES",
	}

	if got := strings.Join(r.MACRO[len(r.MACRO)-len(code):], "\n"); got != strings.Join(code, "\n") {
		t.Errorf("code:\n%s\nwant:\n%s", got, strings.Join(code, "\n"))
	}
}

// TestCompileModuleName checks that the module is named for the first
// program unit (Decision 3), a main program without PROGRAM being
// file$MAIN (section 4.12), and for the file when it has no units.
func TestCompileModuleName(t *testing.T) {
	cases := []struct {
		lines  []string
		module string
		end    string
	}{
		{[]string{"      X = 1", "      END"}, "PROG$MAIN", "\t.END\tPROG$MAIN"},
		{[]string{"      SUBROUTINE S", "      END", "      PROGRAM P", "      END"}, "S", "\t.END\tP"},
		{[]string{"      FUNCTION F(X)", "      F = X", "      END"}, "F", "\t.END"},
		{[]string{"C     nothing"}, "PROG", "\t.END"},
	}

	for _, tc := range cases {
		r := Compile(testOptions, File{Spec: "PROG.FOR", Name: "prog", Lines: tc.lines})
		if len(r.Messages) != 0 || r.MACRO == nil {
			t.Errorf("%q: %v", tc.lines, r.Messages)

			continue
		}

		if r.MACRO[0] != "\t.TITLE\t"+tc.module || r.MACRO[len(r.MACRO)-1] != tc.end {
			t.Errorf("%q: %q ... %q, want module %s and %q", tc.lines, r.MACRO[0], r.MACRO[len(r.MACRO)-1], tc.module, tc.end)
		}
	}
}

// includes is an Opener over files in memory.
type includes map[string][]string

func (fs includes) OpenInclude(name string, _ *source.File) (*source.File, error) {
	lines, ok := fs[name]
	if !ok {
		return nil, errors.New("no such file")
	}

	return &source.File{Spec: "[INC]" + name, Lines: lines}, nil
}

func (includes) OpenModule(_, module string, _ *source.File) (*source.File, error) {
	return nil, errors.New("no module " + module)
}

// TestCompileIncludes checks included files' lines in the MACRO file: with
// /LIST all the way down, each marked with its depth, and every file named.
func TestCompileIncludes(t *testing.T) {
	opts := testOptions
	opts.Includes = includes{
		"A": {"      INCLUDE 'B/LIST'", "      X = 1"},
		"B": {"      Y = 2", "      INCLUDE 'C/LIST'"},
		"C": {"C     innermost"},
	}

	r := Compile(opts, File{
		Spec:  "MAIN.FOR",
		Name:  "MAIN",
		Lines: []string{"      INCLUDE 'A/LIST'", "      INCLUDE 'C/LIST'", "      END"},
	})
	if len(r.Messages) != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	want := []string{
		";    1        INCLUDE 'A/LIST'",
		"; INCLUDE file [INC]A:",
		";1   1        INCLUDE 'B/LIST'",
		"; INCLUDE file [INC]B:",
		";2   1        Y = 2",
		"\tMOVL\t#^X00004100,MAIN$MAIN.Y\t; 2.0",
		";2   2        INCLUDE 'C/LIST'",
		"; INCLUDE file [INC]C:",
		";3   1  C     innermost",
		"; end of [INC]C",
		"; end of [INC]B",
		";1   2        X = 1",
		"\tMOVL\t#^X00004080,MAIN$MAIN.X\t; 1.0",
		"; end of [INC]A",
		";    2        INCLUDE 'C/LIST'",
		"; INCLUDE file [INC]C:",
		";1   1  C     innermost",
		"; end of [INC]C",
		";    3        END",
		"\tMOVL\t#1,R0\t\t\t; SS$_NORMAL",
		"\tRET",
		"",
		"\t.END\tMAIN$MAIN",
	}

	got := r.MACRO[len(r.MACRO)-len(want):]
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("MACRO file ends:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestCompileIncludesNoList checks INCLUDE's default: the code is there and the
// files are named, but not their lines, and /LIST in a file not listed brings
// nothing back.
func TestCompileIncludesNoList(t *testing.T) {
	opts := testOptions
	opts.Includes = includes{
		"A": {"      INCLUDE 'B/LIST'", "      X = 1"},
		"B": {"      Y = 2"},
	}

	r := Compile(opts, File{Spec: "MAIN.FOR", Name: "MAIN", Lines: []string{"      INCLUDE 'A'", "      END"}})
	if len(r.Messages) != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	want := []string{
		";    1        INCLUDE 'A'",
		"; INCLUDE file [INC]A:",
		"; INCLUDE file [INC]B:",
		"\tMOVL\t#^X00004100,MAIN$MAIN.Y\t; 2.0",
		"; end of [INC]B",
		"\tMOVL\t#^X00004080,MAIN$MAIN.X\t; 1.0",
		"; end of [INC]A",
		";    2        END",
	}

	text := strings.Join(r.MACRO, "\n")
	if !strings.Contains(text, strings.Join(want, "\n")) {
		t.Errorf("MACRO file:\n%s\nwant, among it:\n%s", text, strings.Join(want, "\n"))
	}
}

// TestCompileAll checks files joined by + : one module, named for the first
// file, whose unit runs from one file into the next, each file named.
func TestCompileAll(t *testing.T) {
	r := CompileAll(testOptions, []File{
		{Spec: "A.FOR", Name: "A", Lines: []string{"      X = 1"}},
		{Spec: "B.FOR", Name: "B", Lines: []string{"      Y = 2", "      END"}},
	})
	if len(r.Messages) != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	text := strings.Join(r.MACRO, "\n")

	for _, want := range []string{"\t.TITLE\tA$MAIN", "; from A.FOR\n; from B.FOR\n", "; end of A.FOR\n; source file B.FOR:\n;    1        Y = 2", "\t.END\tA$MAIN"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
}

// TestCompileSourceErrors checks that the source pass's errors are the
// compilation's: no MACRO file, the messages counted.
func TestCompileSourceErrors(t *testing.T) {
	r := Compile(testOptions, File{
		Spec:  "BAD.FOR",
		Name:  "BAD",
		Lines: []string{"     1X = 1", "      INCLUDE 'NONE'", "      END"},
	})

	if r.MACRO != nil || r.Errors != 2 || len(r.Messages) != 2 ||
		r.Messages[0].Ident != "NOINITIAL" || r.Messages[1].Ident != "INCLOPEN" {
		t.Errorf("result: %d errors, %v", r.Errors, r.Messages)
	}

	// The continuation limit and /D_LINES reach the source pass.
	opts := testOptions
	opts.Continuations = source.NoContinuations
	opts.DLines = true

	r = Compile(opts, File{Spec: "D.FOR", Name: "D", Lines: []string{"D     X = 1", "D    1 + 2"}})
	if r.Errors != 1 || r.Messages[0].Ident != "TOOMANYCONT" {
		t.Errorf("/CONTINUATIONS=0 /D_LINES: %v", r.Messages)
	}
}

// TestCompileParseErrors checks that the parser runs: its messages, and
// semantic analysis's NOTIMPL for a statement the compiler can't compile
// yet, are the compilation's, each statement reported, and no MACRO file
// is written. Semantic analysis runs after the parser's errors too.
func TestCompileParseErrors(t *testing.T) {
	r := Compile(testOptions, File{
		Spec: "P.FOR",
		Name: "P",
		Lines: []string{
			"      PROGRAM P",
			"      X = (1 + 2",
			"      READ (1, KEYLE=1) Y",
			"      GO TO 10",
		},
	})

	var got []string
	for _, m := range r.Messages {
		got = append(got, m.Ident)
	}

	if r.MACRO != nil || r.Errors != 4 || strings.Join(got, " ") != "SYNTAX NOEND NOTIMPL UNDEFLBL" {
		t.Errorf("result: %d errors, %v", r.Errors, r.Messages)
	}
}

// TestIdent checks that CDEC$ IDENT reaches the object module: the file's first
// directive's string, whichever unit it is in, and whatever characters it holds,
// through the .IDENT MACRO reads back.
func TestIdent(t *testing.T) {
	allPunctuation := "!\"#$%&''()*+,-./:;<=>?@[\\]^_`{|}~"

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"none", []string{"      END"}, "01"},
		{"before the unit", []string{"CDEC$ IDENT 'V1.2'", "      END"}, "V1.2"},
		{"in the unit", []string{"      X = 1", "CDEC$ IDENT 'V1.2'", "      END"}, "V1.2"},
		{"in a structure", []string{"      STRUCTURE /S/", "      INTEGER I", "CDEC$ IDENT 'V1.2'", "      END STRUCTURE", "      END"}, "V1.2"},
		{"after the unit", []string{"      END", "CDEC$ IDENT 'V1.2'"}, "V1.2"},
		{"in no unit", []string{"CDEC$ IDENT 'V1.2'"}, "V1.2"},
		{"the first of a unit", []string{"CDEC$ IDENT 'A'", "      X = 1", "CDEC$ IDENT 'B'", "      END"}, "A"},
		{"the first of the file", []string{"      END", "CDEC$ IDENT 'A'", "      SUBROUTINE Y", "CDEC$ IDENT 'B'", "      END"}, "A"},
		{"case and blanks kept", []string{"CDEC$   IDENT 'a  B' ! a comment", "      END"}, "a  B"},
		{"an empty first string", []string{"CDEC$ IDENT ''", "CDEC$ IDENT 'B'", "      END"}, "01"},
		{"31 characters", []string{"CDEC$ IDENT '" + strings.Repeat("X", 31) + "'", "      END"}, strings.Repeat("X", 31)},
		{"every punctuation mark", []string{"CDEC$ IDENT '" + allPunctuation[:31] + "'", "      END"}, strings.ReplaceAll(allPunctuation[:31], "''", "'")},
		{"the rest of them", []string{"CDEC$ IDENT '" + allPunctuation[31:] + "'", "      END"}, allPunctuation[31:]},
	}

	// And each character alone, which may be the delimiter MACRO would be given.
	for c := byte(' '); c <= '~'; c++ {
		text := string(c)
		if c == '\'' {
			text = "''"
		}

		cases = append(cases, struct {
			name  string
			lines []string
			want  string
		}{"the character " + string(c), []string{"CDEC$ IDENT '" + text + "'", "      END"}, string(c)})
	}

	for _, c := range cases {
		r := Compile(testOptions, File{Spec: "I.FOR", Name: "I", Lines: c.lines})
		if r.MACRO == nil {
			t.Errorf("%s: %v", c.name, r.Messages)

			continue
		}

		a := asm.New(false)
		a.SetDialect(asm.DialectMACRO)

		if _, err := a.Assemble(strings.Join(r.MACRO, "\n") + "\n"); err != nil {
			t.Errorf("%s: %s doesn't assemble: %v", c.name, r.MACRO[1], err)

			continue
		}

		if got := a.Ident(); got != c.want {
			t.Errorf("%s: %s made the ident %q, want %q", c.name, r.MACRO[1], got, c.want)
		}
	}
}

// TestCompileEmpty checks that an empty source still makes a file MACRO
// can assemble, its module named for the file.
func TestCompileEmpty(t *testing.T) {
	r := Compile(testOptions, File{Spec: "empty.for", Name: "empty"})

	if r.MACRO == nil || r.MACRO[0] != "\t.TITLE\tEMPTY" || r.MACRO[len(r.MACRO)-1] != "\t.END" {
		t.Errorf("MACRO file = %q", r.MACRO)
	}
}

// TestCompileLinesFit checks that every line of the MACRO file is within
// MACRO's 132 characters, whatever the source holds.
func TestCompileLinesFit(t *testing.T) {
	file := File{
		Spec:  strings.Repeat("D", 200) + ".FOR",
		Name:  "LONG",
		Lines: []string{strings.Repeat("X", 300), "\x0c", "ok"},
	}

	r := Compile(Options{CommandLine: strings.Repeat("FORTRAN ", 30)}, file)

	for _, line := range r.MACRO {
		if len(line) > 132 {
			t.Errorf("line of %d characters: %q", len(line), line)
		}
	}
}

// TestModuleName checks the module name made from a source file's name.
func TestModuleName(t *testing.T) {
	cases := map[string]string{
		"squares":                       "SQUARES",
		"Hello-World":                   "HELLO_WORLD",
		"MY$PROG_2":                     "MY$PROG_2",
		"2fast":                         "F_2FAST",
		"":                              "FORTRAN",
		strings.Repeat("ABCDEFGHIJ", 4): "ABCDEFGHIJABCDEFGHIJABCDEFGHIJA",
		"café":                          "CAF_",
	}

	for name, want := range cases {
		if got := ModuleName(name); got != want {
			t.Errorf("ModuleName(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestVMSTime checks the header's date and time form.
func TestVMSTime(t *testing.T) {
	if got := vmsTime(time.Date(2026, time.March, 9, 8, 7, 6, 50_000_000, time.UTC)); got != "9-MAR-2026 08:07:06.05" {
		t.Errorf("vmsTime = %q", got)
	}
}

// moduleIncludes is an Opener whose modules are in memory.
type moduleIncludes map[string][]string

func (moduleIncludes) OpenInclude(name string, _ *source.File) (*source.File, error) {
	return nil, errors.New("no file " + name)
}

func (m moduleIncludes) OpenModule(_, module string, _ *source.File) (*source.File, error) {
	return &source.File{Spec: module, Lines: m[module]}, nil
}

// TestLibraryNamesCostNothing checks Decision 5 of Phase 83: of a text library
// module's EXTERNAL names only those used are in the .EXTRN, where a program's
// own are all there; and the storage map leaves out a variable a module declares
// and the unit never uses.
func TestLibraryNamesCostNothing(t *testing.T) {
	opts := testOptions
	opts.Includes = moduleIncludes{
		"NESTED": {"      EXTERNAL NESTUSE,NESTOFF"},
		"NAMES": {
			"      EXTERNAL CALLED,FUNCTN,PASSED,ADDRESS,UNUSED",
			"      INTEGER*4 FUNCTN,USEDVAR,IDLEVAR",
			"      INCLUDE '(NESTED)'",
		},
	}

	r := Compile(opts, File{Spec: "T.FOR", Name: "T", Lines: []string{
		"      PROGRAM T",
		"      INCLUDE '(NAMES)'",
		"      EXTERNAL OWNDATA",
		"      INTEGER*4 OWNIDLE",
		"      CALL CALLED(PASSED)",
		"      CALL NESTUSE",
		"      USEDVAR = FUNCTN(%LOC(ADDRESS))",
		"      END",
	}})
	if r.Errors != 0 || r.Warnings != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	text := strings.Join(r.MACRO, "\n")

	var extrn []string

	for _, line := range r.MACRO {
		if rest, ok := strings.CutPrefix(line, "\t.EXTRN\t"); ok {
			extrn = strings.Split(strings.Fields(rest)[0], ",")
		}
	}

	for _, name := range []string{"CALLED", "FUNCTN", "PASSED", "ADDRESS", "NESTUSE", "OWNDATA"} {
		if !slices.Contains(extrn, name) {
			t.Errorf(".EXTRN has no %s: %v", name, extrn)
		}
	}

	for _, name := range []string{"UNUSED", "NESTOFF"} {
		if slices.Contains(extrn, name) {
			t.Errorf(".EXTRN has %s: %v", name, extrn)
		}
	}

	if !strings.Contains(text, ";   USEDVAR") || !strings.Contains(text, ";   OWNIDLE") {
		t.Errorf("the map lacks a variable:\n%s", text)
	}

	if strings.Contains(text, ";   IDLEVAR") {
		t.Errorf("the map has a module's unused variable:\n%s", text)
	}
}

// listingOf compiles the lines with a listing, and returns it as one string too.
func listingOf(t *testing.T, opts Options, lines ...string) (Result, string) {
	t.Helper()

	opts.List = true

	r := Compile(opts, File{Spec: "DUA0:[WORK]P.FOR;1", Name: "P", Lines: lines})
	if len(r.Listing) < 3 {
		t.Fatalf("listing %q", r.Listing)
	}

	return r, strings.Join(r.Listing, "\n")
}

// TestListingMapIsTheMACROFiles: the storage map's words are the same in both, and
// each unit's is there, after the whole source.
func TestListingMapIsTheMACROFiles(t *testing.T) {
	opts := testOptions
	opts.Show.Map = true

	r, text := listingOf(t, opts,
		"      PROGRAM P",
		"      INTEGER I",
		"      I = 1",
		"      CALL S(I)",
		"      END",
		"      SUBROUTINE S(K)",
		"      K = 2",
		"      END")

	macro := strings.Join(r.MACRO, "\n")

	for _, want := range []string{"Storage map for P, the main program", "Storage map for S, a subroutine"} {
		if !strings.Contains(text, "\n"+want+"\n") || !strings.Contains(macro, "; "+want+"\n") {
			t.Errorf("%s isn't in both:\n%s", want, text)
		}
	}

	for _, line := range r.Listing {
		if strings.HasPrefix(line, "  ") && strings.Contains(line, "$LOCAL+") && !strings.Contains(macro, "; "+line+"\n") {
			t.Errorf("the MACRO file doesn't have %q", line)
		}
	}

	if strings.Count(text, "\f") != 2 || !strings.HasPrefix(text, "\fP ") || !strings.Contains(text, "\fS ") {
		t.Errorf("each unit should start a page with its name:\n%s", text)
	}
}

// TestListingWithErrors: a listing has the messages under their lines and no map,
// and the compilation still has no MACRO file.
func TestListingWithErrors(t *testing.T) {
	opts := testOptions
	opts.Show.Map = true

	r, text := listingOf(t, opts, "      PROGRAM P", "      GO TO 30", "      END")

	if r.Errors == 0 || r.MACRO != nil {
		t.Fatalf("errors %d, MACRO %v", r.Errors, r.MACRO)
	}

	if !strings.Contains(text, "    2        GO TO 30\n") || !strings.Contains(text, "%FORT-E-") || strings.Contains(text, "Storage map") {
		t.Errorf("listing:\n%s", text)
	}
}

// memoryIncludes reads INCLUDE files from a map, a new File at each reading.
type memoryIncludes map[string][]string

func (m memoryIncludes) OpenInclude(name string, _ *source.File) (*source.File, error) {
	lines, ok := m[name]
	if !ok {
		return nil, errors.New("no such file")
	}

	return &source.File{Spec: name, Lines: lines}, nil
}

func (m memoryIncludes) OpenModule(_, module string, _ *source.File) (*source.File, error) {
	return nil, errors.New("no module " + module)
}

// TestListingIncludeTwice: a message is under the reading of an included line it's
// about, here the second unit's, which shows though nobody asked for its lines.
func TestListingIncludeTwice(t *testing.T) {
	opts := testOptions
	opts.Includes = memoryIncludes{"L": {"      GO TO 30"}}

	_, text := listingOf(t, opts,
		"      PROGRAM P",
		"30    CONTINUE",
		"      INCLUDE 'L'",
		"      END",
		"      SUBROUTINE S",
		"      INCLUDE 'L'",
		"      END")

	if got := strings.Count(text, "  1 1        GO TO 30"); got != 1 {
		t.Fatalf("the included line shows %d times:\n%s", got, text)
	}

	if strings.Index(text, "\fS ") > strings.Index(text, "GO TO 30") {
		t.Errorf("the message belongs under the second reading:\n%s", text)
	}
}

// TestListingAfterFatal: a compilation the source pass stopped has a listing, of the
// lines read, with the file's name for its title.
func TestListingAfterFatal(t *testing.T) {
	opts := testOptions

	r, text := listingOf(t, opts, "      PROGRAM P", "      INCLUDE 'GONE'", "      END")

	if r.Errors == 0 || !strings.HasPrefix(text, "\fP ") || !strings.Contains(text, "%FORT-F-INCLOPEN") {
		t.Errorf("listing:\n%s", text)
	}
}

// TestListingTitle: directives in a structure and between units reach the heading,
// on the next page.
func TestListingTitle(t *testing.T) {
	opts := testOptions

	_, text := listingOf(t, opts,
		"      PROGRAM P",
		"      STRUCTURE /S/",
		"CDEC$ TITLE 'IN A STRUCTURE'",
		"      INTEGER F",
		"      END STRUCTURE",
		"      END",
		"CDEC$ SUBTITLE 'BEFORE A UNIT'",
		"      SUBROUTINE Q",
		"CDEC$ TITLE",
		"      END")

	if !strings.HasPrefix(text, "\fP ") {
		t.Errorf("the first page's title should be the unit's:\n%s", text)
	}

	second := text[strings.LastIndex(text, "\f"):]
	if !strings.HasPrefix(second, "\fIN A STRUCTURE ") {
		t.Errorf("the second page's title:\n%s", second)
	}

	// The SUBTITLE is on the second page, so it heads a third, if there is one.
	if strings.Contains(second[:strings.Index(second, "\n\n")], "BEFORE A UNIT") {
		t.Errorf("the SUBTITLE took effect on its own page:\n%s", second)
	}
}
