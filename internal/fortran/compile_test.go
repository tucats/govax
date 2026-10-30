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

// TestCompileSkeleton checks the whole MACRO file this stage writes for
// a small program: the header, the source as comments, and .END.
func TestCompileSkeleton(t *testing.T) {
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

	want := []string{
		"\t.TITLE\tSQUARES",
		"\t.IDENT\t/01/",
		";",
		"; Compiled by govax FORTRAN V1.0-1 on 29-OCT-2026 14:05:11.20",
		"; from DUA0:[WORK]SQUARES.FOR;1",
		"; by the command FORTRAN SQUARES",
		";",
		"; Each line of the FORTRAN source appears below as a comment, with its line number;",
		"; a line from an INCLUDE file has its include depth before the number.",
		"; This version of the compiler generates no code yet; MACRO assembles this file",
		"; into an empty module.",
		";",
		";    1        PROGRAM SQUARES",
		";    2  C     Squares of the first five integers",
		";    3        DO 10 I = 1, 5",
		";    4  \t PRINT *, I, I*I",
		";    5  10    CONTINUE",
		";    6        END",
		"",
		"\t.END",
	}

	if got := strings.Join(r.MACRO, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("MACRO file:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
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
		";2   2        INCLUDE 'C'",
		"; INCLUDE file [INC]C:",
		";3   1  C     innermost",
		"; end of [INC]C",
		"; end of [INC]B",
		";1   2        X = 1",
		"; end of [INC]A",
		";    2        INCLUDE 'C'",
		"; INCLUDE file [INC]C:",
		";1   1  C     innermost",
		"; end of [INC]C",
		";    3        END",
		"",
		"\t.END",
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

	if r.MACRO != nil || r.Errors != 3 || strings.Join(got, " ") != "SYNTAX NOTIMPL NOEND" {
		t.Errorf("result: %d errors, %v", r.Errors, r.Messages)
	}
}

// TestCompileEmpty checks that an empty source still makes a file MACRO
// can assemble.
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
