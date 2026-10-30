package fortran

import (
	"errors"
	"strings"
	"testing"
	"time"

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
		"; the instructions it became; a line from an INCLUDE file has its include depth",
		"; before the number. Each routine's storage map says where its variables are.",
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
		"\tPUSHAB\tSQUARES.$IO1",
		"\tCALLS\t#1,G^FTN$PRINT_LIST",
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

// TestCompileIncludes checks that included files' lines are in the MACRO
// file, marked with their depth and their files' names.
func TestCompileIncludes(t *testing.T) {
	opts := testOptions
	opts.Includes = includes{
		"A": {"      INCLUDE 'B'", "      X = 1"},
		"B": {"      Y = 2", "      INCLUDE 'C'"},
		"C": {"C     innermost"},
	}

	r := Compile(opts, File{
		Spec:  "MAIN.FOR",
		Name:  "MAIN",
		Lines: []string{"      INCLUDE 'A'", "      INCLUDE 'C'", "      END"},
	})
	if len(r.Messages) != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	want := []string{
		";    1        INCLUDE 'A'",
		"; INCLUDE file [INC]A:",
		";1   1        INCLUDE 'B'",
		"; INCLUDE file [INC]B:",
		";2   1        Y = 2",
		"\tMOVL\t#^X00004100,MAIN$MAIN.Y\t; 2.0",
		";2   2        INCLUDE 'C'",
		"; INCLUDE file [INC]C:",
		";3   1  C     innermost",
		"; end of [INC]C",
		"; end of [INC]B",
		";1   2        X = 1",
		"\tMOVL\t#^X00004080,MAIN$MAIN.X\t; 1.0",
		"; end of [INC]A",
		";    2        INCLUDE 'C'",
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
// NOTIMPL for a statement the compiler can't compile yet, are the
// compilation's, each statement reported, and no MACRO file is written.
// Semantic analysis runs after the parser's errors too (UNDEFLBL).
func TestCompileParseErrors(t *testing.T) {
	r := Compile(testOptions, File{
		Spec: "P.FOR",
		Name: "P",
		Lines: []string{
			"      PROGRAM P",
			"      X = (1 + 2",
			"      READ *, Y",
			"      GO TO 10",
		},
	})

	var got []string
	for _, m := range r.Messages {
		got = append(got, m.Ident)
	}

	if r.MACRO != nil || r.Errors != 4 || strings.Join(got, " ") != "SYNTAX NOTIMPL NOEND UNDEFLBL" {
		t.Errorf("result: %d errors, %v", r.Errors, r.Messages)
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
