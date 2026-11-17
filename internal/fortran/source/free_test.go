package source

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
)

// update rewrites the golden files of TestFreeFormGolden.
var update = flag.Bool("update", false, "rewrite the golden files")

// readFreeForm reads lines as the free-form file MAIN.F90 under form, with
// includes from fs, and returns the program and its messages.
func readFreeForm(t *testing.T, form Form, fs files, lines ...string) (*Program, []diag.Message) {
	t.Helper()

	var messages diag.List

	p := Read(form, &File{Spec: "MAIN.F90", Lines: lines, Free: true}, fs, &messages)

	return p, messages.Messages()
}

// checkFree compares the statements read from free-form lines with want, and
// that no message was reported.
func checkFree(t *testing.T, form Form, lines []string, want ...string) *Program {
	t.Helper()

	p, messages := readFreeForm(t, form, nil, lines...)
	if len(messages) != 0 {
		t.Errorf("messages %v", messages)
	}

	if got := texts(p); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("statements:\n%q\nwant:\n%q", got, want)
	}

	for _, s := range p.Statements {
		if !s.Free {
			t.Errorf("%q: not marked free form", s.Text)
		}
	}

	return p
}

// expectMessages checks that messages are want, each "IDENT@line:column".
func expectMessages(t *testing.T, messages []diag.Message, want ...string) {
	t.Helper()

	var got []string
	for _, m := range messages {
		got = append(got, fmt.Sprintf("%s@%d:%d", m.Ident, m.Pos.Line, m.Pos.Column))
	}

	if !slices.Equal(got, want) {
		t.Errorf("messages %v, want %v", got, want)
	}
}

// TestFreeStatements checks free form's lines: statements anywhere on a line,
// comments, labels, and every line's kind.
func TestFreeStatements(t *testing.T) {
	p := checkFree(t, Form{F90: true}, []string{
		"! a comment line",
		"program squares   ! a comment after a statement",
		"",
		"    \t  ",
		"  integer :: i",
		"10 continue",
		"   99999 x = 'it''s! not a comment'",
		`  y = "say ""hi""! still not"`,
		"end program squares",
	},
		"0|program squares   ",
		"0|integer :: i",
		"10|continue",
		"99999|x = 'it''s! not a comment'",
		`0|y = "say ""hi""! still not"`,
		"0|end program squares",
	)

	var kinds []LineKind
	for _, line := range p.Lines {
		kinds = append(kinds, line.Kind)
	}

	want := []LineKind{Comment, Initial, Comment, Comment, Initial, Initial, Initial, Initial, Initial}
	if !slices.Equal(kinds, want) {
		t.Errorf("kinds %v, want %v", kinds, want)
	}
}

// TestFreeQuotes checks that " starts a character constant only under /F90:
// under /NOF90 it is VAX FORTRAN's octal constant, and a ! after it a comment.
func TestFreeQuotes(t *testing.T) {
	checkFree(t, Form{F90: true}, []string{`x = "a!b"`}, `0|x = "a!b"`)
	checkFree(t, Form{}, []string{`x = "17 ! octal`}, `0|x = "17 `)
}

// TestFreeSemicolons checks ; between statements (3.3.1.2).
func TestFreeSemicolons(t *testing.T) {
	checkFree(t, Form{F90: true}, []string{
		"a = 1; b = 2",
		"c = 3 ;; ; d = 4;",
		"e = ';'; f = 5 ; ! a comment",
		"10 g = 6; 20 h = 7",
	},
		"0|a = 1",
		"0|b = 2",
		"0|c = 3 ",
		"0|d = 4",
		"0|e = ';'",
		"0|f = 5 ",
		"10|g = 6",
		"20|h = 7",
	)

	p, messages := readFreeForm(t, Form{F90: true}, nil, "; a = 1", "  ;", "b = 2")
	expectMessages(t, messages, "EMPTYSEMI@1:1", "EMPTYSEMI@2:3")

	if got := texts(p); !slices.Equal(got, []string{"0|a = 1", "0|b = 2"}) {
		t.Errorf("statements %q", got)
	}
}

// TestFreeContinuation checks & outside a character context (3.3.1.3.1): with
// and without the second &, tokens split, comment lines between, and a
// statement continued after a ; and onto a line that starts another.
func TestFreeContinuation(t *testing.T) {
	p := checkFree(t, Form{F90: true}, []string{
		"x = a + &   ! the first line",
		"! a comment line between",
		"",
		"    b",
		"inte&",
		"  &ger :: count",
		"y = 1; z = &",
		"  & 2; w = 3",
		"call s(1, &",
		"       2)",
	},
		"0|x = a +     b",
		"0|integer :: count",
		"0|y = 1",
		"0|z =  2",
		"0|w = 3",
		"0|call s(1,        2)",
	)

	if s := p.Statements[0]; s.First != 0 || s.Last != 3 {
		t.Errorf("x's lines %d to %d", s.First, s.Last)
	}

	if s := p.Statements[3]; s.First != 6 || s.Last != 7 {
		t.Errorf("z's lines %d to %d", s.First, s.Last)
	}

	if s := p.Statements[4]; s.First != 7 || s.Last != 7 {
		t.Errorf("w's lines %d to %d", s.First, s.Last)
	}

	if p.Lines[3].Kind != Continuation || p.Lines[7].Kind != Continuation {
		t.Errorf("continuation lines' kinds %v %v", p.Lines[3].Kind, p.Lines[7].Kind)
	}
}

// TestFreeCharacterContinuation checks & in a character context (3.3.1.3.2):
// the constant goes on after the second &, its blanks before the first & kept,
// a ! in it text; without the second &, MISSINGAMP.
func TestFreeCharacterContinuation(t *testing.T) {
	checkFree(t, Form{F90: true}, []string{
		"s = 'one, two  &",
		"     &three! four &",
		"   &five'   ! a comment",
		`t = "it's &`,
		`  &fine"`,
		"10 format (5h!a;b&, i3)",
	},
		"0|s = 'one, two  three! four five'   ",
		`0|t = "it's fine"`,
		"10|format (5h!a;b&, i3)",
	)

	p, messages := readFreeForm(t, Form{F90: true}, nil, "s = 'ab&", "cd'")
	expectMessages(t, messages, "MISSINGAMP@2:1")

	if got := texts(p); !slices.Equal(got, []string{"0|s = 'abcd'"}) {
		t.Errorf("statements %q", got)
	}
}

// TestFreeAmpersandErrors checks the misplaced &s: alone on a line, starting a
// line that continues nothing, and continuing past the file's end.
func TestFreeAmpersandErrors(t *testing.T) {
	p, messages := readFreeForm(t, Form{F90: true}, nil,
		"  &",
		"x = 1 + &",
		"  &   ! alone before a comment",
		"  2",
		"&y = 3",
		"z = &",
	)
	expectMessages(t, messages, "LONEAMP@1:3", "LONEAMP@3:3", "NOINITIAL@5:1", "CONTNOLINE@6:5")

	if got := texts(p); !slices.Equal(got, []string{"0|x = 1 +   2", "0|y = 3", "0|z = "}) {
		t.Errorf("statements %q", got)
	}

	// In a character context, a ! after the leading & is the constant's text.
	checkFree(t, Form{F90: true}, []string{"s = 'a&", "&! b'"}, "0|s = 'a! b'")
}

// TestFreeLabels checks free form's labels (3.3.1.4): digits first, a blank
// after them, 1 to 5 digits, not all zeros.
func TestFreeLabels(t *testing.T) {
	p, messages := readFreeForm(t, Form{F90: true}, nil,
		"  00010 continue",
		"123456 continue",
		"0 continue",
		"20continue",
		"30",
	)
	expectMessages(t, messages, "BADLABEL@2:1", "ZEROLABEL@3:1", "LABELBLANK@4:3")

	want := []string{"10|continue", "0|continue", "0|continue", "20|continue", "30|"}
	if got := texts(p); !slices.Equal(got, want) {
		t.Errorf("statements %q, want %q", got, want)
	}

	if pos := p.Statements[0].LabelPosition(); pos.Line != 1 || pos.Column != 3 {
		t.Errorf("label at %+v", pos)
	}
}

// TestFreeContinuationLimit checks the 39 continuation lines of 3.3.1.4, and
// /CONTINUATIONS=n in free form.
func TestFreeContinuationLimit(t *testing.T) {
	lines := func(n int) []string {
		out := []string{"x = 1 &"}
		for range n - 1 {
			out = append(out, "  + 1 &")
		}

		return append(out, "  + 1")
	}

	_, messages := readFreeForm(t, Form{}, nil, lines(39)...)
	expectMessages(t, messages)

	_, messages = readFreeForm(t, Form{}, nil, lines(40)...)
	expectMessages(t, messages, "TOOMANYCONT@41:0")

	_, messages = readFreeForm(t, Form{Continuations: 2}, nil, lines(3)...)
	expectMessages(t, messages, "TOOMANYCONT@4:0")

	_, messages = readFreeForm(t, Form{Continuations: 60}, nil, lines(50)...)
	expectMessages(t, messages)
}

// TestFreeLineLength checks the 132 characters of a free-form line: blanks
// past them are allowed, anything else is fatal, as /EXTEND_SOURCE's are.
func TestFreeLineLength(t *testing.T) {
	long := "x = " + strings.Repeat("1", 128)

	checkFree(t, Form{}, []string{long + "     ", "y = 2"}, "0|"+long+"     ", "0|y = 2")

	p, messages := readFreeForm(t, Form{}, nil, "a = 1", long+"2", "y = 2")
	expectMessages(t, messages, "LINETOOLONG@2:133")

	if got := texts(p); !slices.Equal(got, []string{"0|a = 1"}) {
		t.Errorf("statements %q", got)
	}

	if len(p.Lines) != 2 {
		t.Errorf("%d lines kept, want 2", len(p.Lines))
	}
}

// TestFreeDirectives checks !DEC$ in free form, anywhere a line's first
// characters, and in fixed form in column 1; and a directive inside a
// continued statement.
func TestFreeDirectives(t *testing.T) {
	p := checkFree(t, Form{F90: true}, []string{
		"!DEC$ IDENT '1.0'",
		"   !dec$\tTITLE 'X'  ! a comment",
		"!DEC$X a comment",
		"x = 1",
	},
		"D| IDENT '1.0'",
		"D|\tTITLE 'X'  ",
		"0|x = 1",
	)

	if p.Lines[0].Kind != Directive || p.Lines[2].Kind != Comment {
		t.Errorf("kinds %v %v", p.Lines[0].Kind, p.Lines[2].Kind)
	}

	if pos := p.Statements[1].Position(1); pos.Column != 10 {
		t.Errorf("directive's TITLE at %+v", pos)
	}

	check(t, Form{}, []string{"!DEC$ IDENT '1.0'", "!DEC$X a comment", " !DEC$ a comment", "      X = 1"},
		"D|IDENT '1.0'", "0|X = 1")

	_, messages := readFreeForm(t, Form{}, nil, "x = 1 + &", "!DEC$ IDENT 'A'", "  2")
	expectMessages(t, messages, "DIRINSTMT@2:1")
}

// TestFreePositions checks that each character of a free-form statement's
// text maps back to its line and column, a split token's halves included.
func TestFreePositions(t *testing.T) {
	p := checkFree(t, Form{F90: true}, []string{
		"10 total = to&",
		"   ! a comment",
		"      &tal + x(i) * &",
		"  weight(i); y = 1",
	},
		"10|total = total + x(i) *   weight(i)",
		"0|y = 1",
	)

	s := p.Statements[0]

	cases := []struct {
		offset    int
		line, col int
	}{
		{0, 1, 4}, // the first t
		{strings.Index(s.Text, "= to") + 2, 1, 12},  // the to before the &
		{strings.Index(s.Text, "= to") + 4, 3, 8},   // tal after the &
		{strings.Index(s.Text, "*"), 3, 19},         // the *
		{strings.Index(s.Text, "weight") - 1, 4, 2}, // a blank the line brought
		{strings.Index(s.Text, "weight"), 4, 3},     // weight
		{len(s.Text), 4, 12},                        // just past the end
	}

	for _, c := range cases {
		pos := s.Position(c.offset)
		if pos.Line != c.line || pos.Column != c.col || pos.File != "MAIN.F90" {
			t.Errorf("offset %d: %+v, want line %d column %d", c.offset, pos, c.line, c.col)
		}
	}

	if pos := p.Statements[1].Position(0); pos.Line != 4 || pos.Column != 14 {
		t.Errorf("y at %+v", pos)
	}

	if pos := s.LabelPosition(); pos.Line != 1 || pos.Column != 1 {
		t.Errorf("label at %+v", pos)
	}
}

// TestFreeInclude checks INCLUDE across the forms: each file read in its own
// form, by its type (Decision 3), and INCLUDE "name" under /F90.
func TestFreeInclude(t *testing.T) {
	fs := files{
		"DECLS.FOR": {"      INTEGER I,", "     1        J"},
		"MORE.F90":  {"k = 1; &", "  l = 2"},
	}

	p, messages := readFreeForm(t, Form{F90: true}, fs,
		"program p",
		"include 'DECLS.FOR'",
		`include "MORE.F90"`,
		"end program p",
	)
	expectMessages(t, messages)

	want := []string{"0|program p", "0|INTEGER I,        J", "0|k = 1", "0|  l = 2", "0|end program p"}
	if got := texts(p); !slices.Equal(got, want) {
		t.Errorf("statements %q, want %q", got, want)
	}

	if p.Statements[1].Free || !p.Statements[2].Free {
		t.Errorf("forms: DECLS %v, MORE %v", p.Statements[1].Free, p.Statements[2].Free)
	}

	// A fixed-form file including a free-form one.
	p, idents := read(t, Form{}, fs, "      PROGRAM P", "      INCLUDE 'MORE.F90'", "      END")
	if len(idents) != 0 {
		t.Errorf("messages %v", idents)
	}

	if got := texts(p); !slices.Equal(got, []string{"0|PROGRAM P", "0|k = 1", "0|  l = 2", "0|END"}) {
		t.Errorf("statements %q", got)
	}
}

// TestFreeOptions checks that OPTIONS and END are followed in free form, so
// /F90 changes for one unit; and that a continued END is END.
func TestFreeOptions(t *testing.T) {
	p := checkFree(t, Form{F90: true}, []string{
		"options /nof90",
		"subroutine s",
		"end",
		"subroutine t",
		"end &",
		"  subroutine t",
		"subroutine u",
		"end",
	},
		"0|options /nof90", "0|subroutine s", "0|end",
		"0|subroutine t", "0|end   subroutine t",
		"0|subroutine u", "0|end",
	)

	var got []bool
	for _, s := range p.Statements {
		got = append(got, s.F90)
	}

	if want := []bool{true, false, false, true, true, true, true}; !slices.Equal(got, want) {
		t.Errorf("F90 %v, want %v", got, want)
	}

	if s := p.Statements[4]; s.Initial() != s.Text {
		t.Errorf("Initial %q, want the whole statement", s.Initial())
	}
}

// TestFreeLineMarks checks that /STANDARD's line marks pass over free-form
// lines, which FORTRAN-77 has none of (Phase 110 is /STANDARD=F90).
func TestFreeLineMarks(t *testing.T) {
	p, _ := readFreeForm(t, Form{}, nil, "do i = 1, 2", "! comment", "end do")
	if marks := p.LineMarks(); len(marks) != 0 {
		t.Errorf("marks %v", marks)
	}
}

// TestFreeFormGolden reads each free-form file in testdata/free and its
// fixed-form twin, writes the statements and where each piece of them came
// from, and holds the free form's to the golden file. The two forms must give
// the same statements once blanks are taken out, as the lexer takes them out.
func TestFreeFormGolden(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "free", "*.f90"))
	if err != nil || len(names) == 0 {
		t.Fatalf("no testdata: %v", err)
	}

	for _, name := range names {
		base := strings.TrimSuffix(name, ".f90")

		t.Run(filepath.Base(base), func(t *testing.T) {
			free := readGoldenSource(t, name, true)
			fixed := readGoldenSource(t, base+".for", false)

			if a, b := squeezedTexts(free), squeezedTexts(fixed); !slices.Equal(a, b) {
				t.Errorf("the forms differ:\nfree  %q\nfixed %q", a, b)
			}

			got := dumpStatements(free)
			golden := base + ".golden"

			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}

				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}

			if got != string(want) {
				t.Errorf("%s differs (go test -update rewrites it):\n%s", golden, got)
			}
		})
	}
}

// readGoldenSource reads a test file in its form, failing on any message.
func readGoldenSource(t *testing.T, name string, isFree bool) *Program {
	t.Helper()

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}

	var messages diag.List

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	p := Read(Form{F90: true}, &File{Spec: filepath.Base(name), Lines: lines, Free: isFree}, nil, &messages)

	if m := messages.Messages(); len(m) != 0 {
		t.Fatalf("%s: messages %v", name, m)
	}

	return p
}

// squeezedTexts is each statement as label|text with blanks outside
// apostrophes taken out and letters in capitals: what the lexer would make of
// it, near enough for comparing the two forms.
func squeezedTexts(p *Program) []string {
	var out []string

	for _, s := range p.Statements {
		var b strings.Builder

		quoted := false

		for i := 0; i < len(s.Text); i++ {
			c := s.Text[i]

			switch {
			case c == '\'':
				quoted = !quoted
			case quoted:
			case c == ' ' || c == '\t':
				continue
			case c >= 'a' && c <= 'z':
				c -= 'a' - 'A'
			}

			b.WriteByte(c)
		}

		out = append(out, fmt.Sprintf("%d|%s", s.Label, b.String()))
	}

	return out
}

// dumpStatements writes each statement, and for each character of it that
// starts a run from one line, the line and column it came from.
func dumpStatements(p *Program) string {
	var b strings.Builder

	for _, s := range p.Statements {
		fmt.Fprintf(&b, "%d|%s\n", s.Label, s.Text)

		if s.Label != 0 {
			pos := s.LabelPosition()
			fmt.Fprintf(&b, "    label at %d:%d\n", pos.Line, pos.Column)
		}

		for _, piece := range s.pieces {
			end := len(s.Text)

			for _, next := range s.pieces {
				if next.offset > piece.offset {
					end = next.offset

					break
				}
			}

			fmt.Fprintf(&b, "    %d:%d %q\n", piece.line.Number, piece.column, s.Text[piece.offset:end])
		}
	}

	return b.String()
}
