package codegen

import (
	"strings"
	"testing"
)

// TestStatementColumns checks a statement's layout: label, opcode at
// column 8, operands at 16, comment at 40.
func TestStatementColumns(t *testing.T) {
	var w Writer

	w.Statement("", ".TITLE", "SQUARES", "")
	w.Statement("SQUARES.L10", "INCL", "SQUARES.I", "I = I + 1")
	w.Statement("L1", "BRB", "L1", "")
	w.Statement("", "RET", "", "")
	w.Statement("", "MOVL", "#1,R0", "status")

	want := []string{
		"\t.TITLE\tSQUARES",
		"SQUARES.L10:",
		"\tINCL\tSQUARES.I\t\t; I = I + 1",
		"L1:\tBRB\tL1",
		"\tRET",
		"\tMOVL\t#1,R0\t\t\t; status",
	}

	got := w.Lines()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines:\n%q\nwant\n%q", got, want)
	}

	// Each comment starts at column 40.
	for _, line := range got {
		if i := strings.Index(line, ";"); i >= 0 && columnWidth(line[:i]) != commentColumn {
			t.Errorf("%q: comment at column %d", line, columnWidth(line[:i]))
		}
	}
}

// TestStatementLongLabel checks that a label that would reach the opcode
// column goes on a line of its own: six characters and a colon still
// fit, seven don't.
func TestStatementLongLabel(t *testing.T) {
	var w Writer

	w.Statement("SIXCHR", "NOP", "", "")
	w.Statement("SEVENCH", "NOP", "", "")

	got := w.Lines()
	want := []string{"SIXCHR:\tNOP", "SEVENCH:", "\tNOP"}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines = %q, want %q", got, want)
	}
}

// TestStatementLongComment checks that a comment that would take the
// statement past MACRO's limit goes on its own line first.
func TestStatementLongComment(t *testing.T) {
	var w Writer

	comment := strings.Repeat("x", 120)
	w.Statement("", "MOVL", "#1,R0", comment)

	got := w.Lines()
	if len(got) != 2 || got[0] != "; "+comment || got[1] != "\tMOVL\t#1,R0" {
		t.Errorf("lines = %q", got)
	}
}

// TestComment checks comment lines: empty, trailing blanks trimmed,
// control characters replaced, and a long one split at blanks.
func TestComment(t *testing.T) {
	var w Writer

	w.Comment("")
	w.Comment("text  \t")
	w.Comment("form\ffeed")
	w.Comment(strings.Repeat("word ", 40))

	got := w.Lines()
	if got[0] != ";" || got[1] != "; text" || got[2] != "; form?feed" {
		t.Errorf("first lines = %q", got[:3])
	}

	if len(got) != 5 {
		t.Fatalf("a 200-character comment took %d lines, want 2", len(got)-3)
	}

	for _, line := range got[3:] {
		if len(line) > MaxLine || !strings.HasPrefix(line, "; word") || strings.HasSuffix(line, " ") {
			t.Errorf("split line %q", line)
		}
	}
}

// TestSourceLine checks a FORTRAN line copied as a comment: its prefix is
// one tab stop wide, so a tab-format line keeps its layout.
func TestSourceLine(t *testing.T) {
	var w Writer

	w.SourceLine(1, "      PROGRAM SQUARES")
	w.SourceLine(12, "10\tCONTINUE")
	w.SourceLine(3, "")
	w.SourceLine(123456, "C BIG")

	want := []string{
		";    1        PROGRAM SQUARES",
		";   12  10\tCONTINUE",
		";    3",
		";123456 C BIG",
	}

	got := w.Lines()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines:\n%q\nwant\n%q", got, want)
	}

	// The tab in line 12 goes to the same column (16) as it would in the
	// source (8), shifted by exactly the 8-character prefix.
	if columnWidth(";   12  10\t") != 16 {
		t.Error("the prefix doesn't keep tab stops")
	}
}

// TestSourceLineAt checks an included line's prefix: the depth, then the
// number, still one tab stop wide.
func TestSourceLineAt(t *testing.T) {
	var w Writer

	w.SourceLineAt(1, 12, "\tPARAMETER (M=100)")
	w.SourceLineAt(10, 3, "      X = 1")
	w.SourceLineAt(2, 12345, "C LONG")

	want := []string{
		";1  12  \tPARAMETER (M=100)",
		";10  3        X = 1",
		";2:12345 C LONG",
	}

	if got := w.Lines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines:\n%q\nwant\n%q", got, want)
	}
}

// TestSourceLineLong checks that a source line too long for MACRO is
// continued, every piece within the limit, nothing lost.
func TestSourceLineLong(t *testing.T) {
	var w Writer

	text := strings.Repeat("ABCDEFGHIJ", 30)
	w.SourceLine(7, text)

	got := w.Lines()
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(got), got)
	}

	var joined strings.Builder

	for i, line := range got {
		if len(line) > MaxLine {
			t.Errorf("line %d is %d characters", i, len(line))
		}

		want := ";    -  "
		if i == 0 {
			want = ";    7  "
		}

		if !strings.HasPrefix(line, want) {
			t.Errorf("line %d = %q, want prefix %q", i, line, want)
		}

		joined.WriteString(line[len(want):])
	}

	if joined.String() != text {
		t.Error("the pieces don't make up the line")
	}
}
