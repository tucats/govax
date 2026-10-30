package source

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
)

// files is an Opener over files in memory, by name.
type files map[string][]string

func (fs files) OpenInclude(name string, _ *File) (*File, error) {
	lines, ok := fs[name]
	if !ok {
		return nil, errors.New("no such file")
	}

	return &File{Spec: name, Lines: lines}, nil
}

// read reads lines as the file MAIN.FOR under form, with includes from
// fs, and returns the program and the messages' identifiers.
func read(t *testing.T, form Form, fs files, lines ...string) (*Program, []string) {
	t.Helper()

	var messages diag.List

	p := Read(form, &File{Spec: "MAIN.FOR", Lines: lines}, fs, &messages)

	var idents []string
	for _, m := range messages.Messages() {
		idents = append(idents, m.Ident)
	}

	return p, idents
}

// texts is each statement as "label|text" (label 0 for none, "D|" before
// a directive's text), for comparing.
func texts(p *Program) []string {
	out := make([]string, len(p.Statements))

	for i, s := range p.Statements {
		if s.Directive {
			out[i] = "D|" + s.Text
		} else {
			out[i] = fmt.Sprintf("%d|%s", s.Label, s.Text)
		}
	}

	return out
}

// check compares the statements read from lines with want, and that no
// message was reported.
func check(t *testing.T, form Form, lines []string, want ...string) *Program {
	t.Helper()

	p, idents := read(t, form, nil, lines...)
	if len(idents) != 0 {
		t.Errorf("messages %v", idents)
	}

	if got := texts(p); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("statements:\n%q\nwant:\n%q", got, want)
	}

	return p
}

// TestFixedFormat checks the four fields of a fixed-format line.
func TestFixedFormat(t *testing.T) {
	check(t, Form{}, []string{
		"      PROGRAM SQUARES",
		"   10 CONTINUE",
		"10    X = 1",
		"0 1 0 Y = 2", // blanks and leading zeros are ignored in a label
		"99999 STOP",
		"      END",
	},
		"0|PROGRAM SQUARES",
		"10|CONTINUE",
		"10|X = 1",
		"10|Y = 2",
		"99999|STOP",
		"0|END",
	)
}

// TestComments checks the comment lines and that they're kept among the
// program's lines.
func TestComments(t *testing.T) {
	p := check(t, Form{}, []string{
		"C     a comment",
		"c     lowercase",
		"*     an asterisk",
		"!     an exclamation point",
		"",
		"   \t  ",
		"\f",
		"      X = 1",
		"      ! only a comment, in the statement field",
		"\t! only a comment, after a tab",
		"  ! only a comment, in the label field",
		"CPAR$ DO_PARALLEL",
		"D     X = 2",
	},
		"0|X = 1",
	)

	if len(p.Lines) != 13 {
		t.Fatalf("%d lines", len(p.Lines))
	}

	for i, line := range p.Lines {
		want := Comment
		if i == 7 {
			want = Initial
		}

		if line.Kind != want || line.Number != i+1 || line.Depth != 0 {
			t.Errorf("line %d: %+v", i+1, line)
		}
	}
}

// TestContinuation checks continuation lines, with comment lines among
// them, in fixed and tab format.
func TestContinuation(t *testing.T) {
	check(t, Form{}, []string{
		"   10 TOTAL = TOTAL + X(I)*",
		"C     a comment between",
		"     1        WEIGHT(I)",
		"     $ + 1",
		"      Y = 2",
		"     0 + 3", // a zero in column 6: an initial line
		"\tZ = 4 +",
		"\t1 5",
		"\t0Q = 1", // a zero after the tab: an initial line
		"20\tR = 6",
		"     ! + 7", // ! in column 6 is a continuation indicator
	},
		"10|TOTAL = TOTAL + X(I)*        WEIGHT(I) + 1",
		"0|Y = 2",
		"0| + 3",
		"0|Z = 4 + 5",
		"0|Q = 1",
		"20|R = 6 + 7",
	)
}

// TestStatementField checks where the statement field ends: column 72,
// with the sequence field ignored, or 132 with /EXTEND_SOURCE; a tab in
// the label field stands for the columns before column 7.
func TestStatementField(t *testing.T) {
	field := strings.Repeat("A", 66) // columns 7 to 72

	check(t, Form{}, []string{
		"      " + field + "SEQ00010",
		"\t" + field + "SEQ00020",
		"\t1" + field[:65] + "B", // a continuation: the digit is column 6
		"      X = 'SHORT", // not padded to column 72
		"     1LINE'",
	},
		"0|"+field,
		"0|"+field+field[:65]+"B",
		"0|X = 'SHORTLINE'",
	)

	long := strings.Repeat("B", 126) // columns 7 to 132

	check(t, Form{ExtendSource: true}, []string{
		"      " + long,
		"\t" + long,
		"      " + long + "      ", // blanks past 132 are allowed
	},
		"0|"+long,
		"0|"+long,
		"0|"+long,
	)

	p, idents := read(t, Form{ExtendSource: true}, nil, "      X = 1", "      "+long+"Y", "      Z = 2")
	if strings.Join(idents, ",") != "LINETOOLONG" || len(p.Statements) != 1 {
		t.Errorf("past column 132: %v, %q", idents, texts(p))
	}
}

// TestDebugLines checks D lines with and without /D_LINES.
func TestDebugLines(t *testing.T) {
	lines := []string{
		"D     PRINT *, X,",
		"D    1  Y",
		"D  10 CONTINUE",
		"d\tZ = 1",
		"      W = 2",
	}

	check(t, Form{}, lines, "0|W = 2")
	check(t, Form{DLines: true}, lines,
		"0|PRINT *, X,  Y",
		"10|CONTINUE",
		"0|Z = 1",
		"0|W = 2",
	)
}

// TestDirectives checks CDEC$ directive lines.
func TestDirectives(t *testing.T) {
	p := check(t, Form{}, []string{
		"CDEC$ IDENT '1.0'",
		"cdec$\tTITLE 'X'  ! a comment",
		"CDEC$X not a directive",
		"      X = 1",
	},
		"D|IDENT '1.0'",
		"D|TITLE 'X'  ",
		"0|X = 1",
	)

	if p.Lines[0].Kind != Directive || p.Lines[2].Kind != Comment {
		t.Errorf("kinds %v %v", p.Lines[0].Kind, p.Lines[2].Kind)
	}
}

// TestEndOfLineComments checks where a ! starts a comment, and where it
// is part of a constant.
func TestEndOfLineComments(t *testing.T) {
	check(t, Form{}, []string{
		"      X = 1 ! set X",
		"      C = 'WHAT!' ! a constant",
		"      C = 'IT''S!' // '!'",
		"      C = 'CONTINUED",
		"     1 ON!' ! the constant goes on",
		"  100 FORMAT (1X, 6HHELLO!, 2 H'!, 'A') ! Hollerith",
		"      DATA A /2*4HAB!C/ ! a repeated Hollerith",
		"      REAL*8 H ! not a Hollerith count",
		"      CHARACTER*4 HNAME!",
		"      DO 10 H = 1, 2 ! a label",
		"      READ (1'10) X ! a record number",
		"      IF (C .EQ. 1H!) X = 2 ! after .EQ.",
		"      X = 12",
		"     1H!ABCDEFGHIJ ! a count and its characters on two lines",
	},
		"0|X = 1 ",
		"0|C = 'WHAT!' ",
		"0|C = 'IT''S!' // '!'",
		"0|C = 'CONTINUED ON!' ",
		"100|FORMAT (1X, 6HHELLO!, 2 H'!, 'A') ",
		"0|DATA A /2*4HAB!C/ ",
		"0|REAL*8 H ",
		"0|CHARACTER*4 HNAME",
		"0|DO 10 H = 1, 2 ",
		"0|READ (1'10) X ",
		"0|IF (C .EQ. 1H!) X = 2 ",
		"0|X = 12H!ABCDEFGHIJ ",
	)
}

// TestLayoutErrors checks the messages about a line's layout.
func TestLayoutErrors(t *testing.T) {
	cases := []struct {
		lines []string
		want  string
	}{
		{[]string{"     1X = 1"}, "NOINITIAL"},
		{[]string{"CDEC$ IDENT 'A'", "     1X = 1"}, "NOINITIAL"},
		{[]string{"      X = 1", "   5 1 + 2"}, "CONTLABEL"},
		{[]string{"  X = 1"}, "BADLABEL"},
		{[]string{"123456X = 1"}, "NOINITIAL"}, // column 6 holds the 6
		{[]string{"1 2 A X = 1"}, "BADLABEL"},
		{[]string{"00000 X = 1"}, "ZEROLABEL"},
	}

	for _, c := range cases {
		if _, idents := read(t, Form{}, nil, c.lines...); strings.Join(idents, ",") != c.want {
			t.Errorf("%q: %v, want %s", c.lines, idents, c.want)
		}
	}
}

// TestContinuationLimit checks /CONTINUATIONS.
func TestContinuationLimit(t *testing.T) {
	statement := func(n int) []string {
		lines := []string{"      X = 0"}
		for range n {
			lines = append(lines, "     1 + 1")
		}

		return lines
	}

	cases := []struct {
		form  Form
		lines int
		want  string
	}{
		{Form{}, 19, ""},
		{Form{}, 20, "TOOMANYCONT"},
		{Form{Continuations: 40}, 40, ""},
		{Form{Continuations: 40}, 45, "TOOMANYCONT"},
		{Form{Continuations: NoContinuations}, 0, ""},
		{Form{Continuations: NoContinuations}, 1, "TOOMANYCONT"},
	}

	for _, c := range cases {
		p, idents := read(t, c.form, nil, statement(c.lines)...)
		if strings.Join(idents, ",") != c.want {
			t.Errorf("%+v, %d lines: %v, want %q", c.form, c.lines, idents, c.want)
		}

		if want := "X = 0" + strings.Repeat(" + 1", c.lines); len(p.Statements) != 1 || p.Statements[0].Text != want {
			t.Errorf("%+v, %d lines: %q", c.form, c.lines, texts(p))
		}
	}
}

// TestPositions checks that a statement's text maps back to its lines
// and columns.
func TestPositions(t *testing.T) {
	p := check(t, Form{}, []string{
		"C     comment",
		"   10 A = B +",
		"\t1 C ! comment",
		"     2D",
	},
		"10|A = B + C D",
	)

	s := p.Statements[0]
	if s.First != 1 || s.Last != 3 {
		t.Errorf("lines %d to %d", s.First, s.Last)
	}

	cases := []struct {
		char       byte
		line, col  int
		occurrence int
	}{
		{'A', 2, 7, 0},
		{'+', 2, 13, 0},
		{'C', 3, 4, 0},
		{'D', 4, 7, 0},
	}

	for _, c := range cases {
		offset := strings.IndexByte(s.Text, c.char)
		pos := s.Position(offset)

		if pos.Line != c.line || pos.Column != c.col || pos.File != "MAIN.FOR" {
			t.Errorf("%c at %d: %+v, want line %d column %d", c.char, offset, pos, c.line, c.col)
		}

		if pos.Text[pos.Column-1] != c.char {
			t.Errorf("%c: the column points at %q", c.char, pos.Text[pos.Column-1])
		}
	}

	if pos := s.Position(len(s.Text)); pos.Line != 4 || pos.Column != 8 {
		t.Errorf("end: %+v", pos)
	}

	if pos := s.LabelPosition(); pos.Line != 2 || pos.Column != 4 {
		t.Errorf("label: %+v", pos)
	}
}

// TestInclude checks INCLUDE files: their lines and statements in the
// INCLUDE statement's place, nested files, and the forms of the name.
func TestInclude(t *testing.T) {
	fs := files{
		"COMMON.FOR": {
			"C     the common block",
			"      PARAMETER (M=100)",
			"      INCLUDE 'INNER'",
			"      COMMON X(M),",
			"     1  Y(M)",
		},
		"INNER": {"      INTEGER K"},
	}

	p, idents := read(t, Form{}, fs,
		"      PROGRAM MAIN",
		"      INCLUDE 'COMMON.FOR'",
		"C     after",
		"      IN CLUDE",
		"     1 'INNER/NOLIST'",
		"      INCLUDE 'COMMON.FOR / list' ! listed",
		"      END",
	)

	if len(idents) != 0 {
		t.Errorf("messages %v", idents)
	}

	want := []string{
		"0|PROGRAM MAIN",
		"0|PARAMETER (M=100)",
		"0|INTEGER K",
		"0|COMMON X(M),  Y(M)",
		"0|INTEGER K",
		"0|PARAMETER (M=100)",
		"0|INTEGER K",
		"0|COMMON X(M),  Y(M)",
		"0|END",
	}

	if got := texts(p); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("statements:\n%q\nwant:\n%q", got, want)
	}

	var lines []string
	for _, line := range p.Lines {
		lines = append(lines, fmt.Sprintf("%d %s:%d", line.Depth, line.File.Spec, line.Number))
	}

	wantLines := []string{
		"0 MAIN.FOR:1", "0 MAIN.FOR:2",
		"1 COMMON.FOR:1", "1 COMMON.FOR:2", "1 COMMON.FOR:3",
		"2 INNER:1",
		"1 COMMON.FOR:4", "1 COMMON.FOR:5",
		"0 MAIN.FOR:3", "0 MAIN.FOR:4", "0 MAIN.FOR:5",
		"1 INNER:1",
		"0 MAIN.FOR:6",
		"1 COMMON.FOR:1", "1 COMMON.FOR:2", "1 COMMON.FOR:3",
		"2 INNER:1",
		"1 COMMON.FOR:4", "1 COMMON.FOR:5",
		"0 MAIN.FOR:7",
	}

	if strings.Join(lines, "\n") != strings.Join(wantLines, "\n") {
		t.Errorf("lines:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(wantLines, "\n"))
	}

	if s := p.Statements[3]; s.Depth != 1 || p.Lines[s.First].File.Spec != "COMMON.FOR" || s.Last != s.First+1 {
		t.Errorf("COMMON statement: %+v", s)
	}
}

// TestIncludeErrors checks INCLUDE's failures.
func TestIncludeErrors(t *testing.T) {
	fs := files{
		"SELF":   {"      INCLUDE 'SELF'"},
		"CONT":   {"     1 X = 1"},
		"LATER":  {"      Y = 2"},
		"BADONE": {"      INCLUDE 'NONE'", "      Z = 3"},
	}

	cases := []struct {
		lines      []string
		want       string
		statements int
	}{
		{[]string{"      INCLUDE 'NONE'", "      X = 1"}, "INCLOPEN", 0},
		{[]string{"      INCLUDE 'BADONE'", "      X = 1"}, "INCLOPEN", 0},
		{[]string{"      INCLUDE 'SELF'"}, "INCLDEPTH", 0},
		{[]string{"      INCLUDE 'CONT'", "      X = 2"}, "NOINITIAL", 1},
		{[]string{"      INCLUDE 'LATER' X", "      X = 2"}, "BADINCLUDE", 1},
		{[]string{"      INCLUDE 'LATER", "      X = 2"}, "BADINCLUDE", 1},
		{[]string{"      INCLUDE ''", "      X = 2"}, "BADINCLUDE", 1},
		{[]string{"      INCLUDE '/LIST'", "      X = 2"}, "BADINCLUDE", 1},
		{[]string{"      INCLUDE '($SSDEF)'", "      X = 2"}, "NOTIMPL", 1},
		{[]string{"      INCLUDE 'SYS$LIBRARY:FORSYSDEF(SSDEF)'", "      X = 2"}, "NOTIMPL", 1},
		{[]string{"      INCLUDEX = 1"}, "", 1},
		{[]string{"      INCLUDE = 1"}, "", 1},
	}

	for _, c := range cases {
		p, idents := read(t, Form{}, fs, c.lines...)
		if strings.Join(idents, ",") != c.want || len(p.Statements) != c.statements {
			t.Errorf("%q: %v and %q, want %s and %d statements", c.lines, idents, texts(p), c.want, c.statements)
		}
	}

	// With no opener, an INCLUDE can't be read.
	var messages diag.List

	Read(Form{}, &File{Spec: "X", Lines: []string{"      INCLUDE 'A'"}}, nil, &messages)

	if m := messages.Messages(); len(m) != 1 || m[0].Ident != "INCLOPEN" {
		t.Errorf("no opener: %v", m)
	}
}

// TestIncludeMessage checks an INCLUDE failure's message: its position is
// the file name's.
func TestIncludeMessage(t *testing.T) {
	var messages diag.List

	Read(Form{}, &File{Spec: "MAIN.FOR", Lines: []string{"\tINCLUDE 'MISSING'"}}, files{}, &messages)

	want := "%FORT-F-INCLOPEN, can't read the INCLUDE file 'MISSING': no such file\n" +
		"  at line 1 of MAIN.FOR\n" +
		"  \tINCLUDE 'MISSING'\n" +
		"  \t--------^"

	if m := messages.Messages(); len(m) != 1 || m[0].String() != want {
		t.Errorf("message:\n%v\nwant:\n%s", m, want)
	}
}
