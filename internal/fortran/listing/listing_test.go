package listing

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/source"
)

// lines makes source lines of one file, numbered from 1, each its own reading.
func lines(file *source.File, depth int, serial int, texts ...string) []*source.Line {
	out := make([]*source.Line, len(texts))
	for i, text := range texts {
		out[i] = &source.Line{File: file, Number: i + 1, Text: text, Depth: depth, Listed: depth == 0, Serial: serial + i}
	}

	return out
}

func testInput(list []*source.Line) Input {
	return Input{
		Lines:    list,
		Show:     DefaultShow(),
		Title:    "FILE",
		File:     "DUA0:[WORK]P.FOR;1",
		Version:  "1.0-1",
		Compiled: time.Date(2026, time.November, 2, 14, 5, 11, 0, time.UTC),
	}
}

// heading is a page's first heading line.
func heading(title string, page int) string {
	return fmt.Sprintf("\f%-41s2-NOV-2026 14:05:11  %-30sPage %d", title, "govax FORTRAN V1.0-1", page)
}

func compare(t *testing.T, got, want []string) {
	t.Helper()

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestLayout is the layout the author accepted: numbers, include depth, a message
// under its line with a pointer, the map after the source. An included line shows
// only if listed.
func TestLayout(t *testing.T) {
	file := &source.File{Spec: "DUA0:[WORK]P.FOR;1"}
	inc := &source.File{Spec: "DUA0:[WORK]L.FOR;1"}

	list := lines(file, 0, 1, "      PROGRAM P", "      INCLUDE 'L.FOR/LIST'")
	listed := lines(inc, 1, 3, "      PARAMETER (MAX = 10)")
	listed[0].Listed = true
	hidden := lines(inc, 1, 4, "      PARAMETER (MIN = 1)")
	rest := lines(file, 0, 5, "\tGO TO 30", "      END")
	rest[0].Number, rest[1].Number = 3, 4

	in := testInput(append(append(append(list, listed...), hidden...), rest...))
	in.Units = []Unit{{0, "P"}}
	in.Maps = []Map{{"P", []string{"Storage map for P, the main program", "  I  INTEGER  $LOCAL+0  P.I"}}}
	in.Messages = []diag.Message{{Severity: diag.Error, Ident: "UNDEFLBL", Text: "statement label 30 is never defined",
		Pos: diag.Position{File: file.Spec, Line: 3, Column: 7, Serial: 5}}}

	compare(t, Make(in), []string{
		heading("P", 1),
		fmt.Sprintf("%-41s%s", "", "DUA0:[WORK]P.FOR;1"),
		"",
		"    1        PROGRAM P",
		"    2        INCLUDE 'L.FOR/LIST'",
		"  1 1        PARAMETER (MAX = 10)",
		"    3          GO TO 30",
		"                    ^",
		"%FORT-E-UNDEFLBL, statement label 30 is never defined",
		"    4        END",
		"",
		"",
		"Storage map for P, the main program",
		"  I  INTEGER  $LOCAL+0  P.I",
	})
}

// TestShow: /SHOW=INCLUDE lists every included line, NOMAP drops the maps, and a
// line with a message shows whatever anyone asked, in the reading it's about.
func TestShow(t *testing.T) {
	file := &source.File{Spec: "P"}
	inc := &source.File{Spec: "L"}
	first := lines(inc, 1, 1, "C FIRST")
	second := lines(inc, 1, 2, "C FIRST")
	second[0].Number = 1

	build := func(show Show, messages ...diag.Message) []string {
		in := testInput(append(append(lines(file, 0, 3, "      END"), first...), second...))
		in.Show = show
		in.Maps = []Map{{"P", []string{"THE MAP"}}}
		in.Messages = messages

		return Make(in)[3:]
	}

	compare(t, build(Show{Map: true}), []string{"    1        END", "", "", "THE MAP"})
	compare(t, build(Show{Include: true}), []string{"    1        END", "  1 1  C FIRST", "  1 1  C FIRST"})
	compare(t, build(Show{}, diag.Message{Severity: diag.Warning, Ident: "W", Text: "T", Pos: diag.Position{Line: 1, Serial: 2}}),
		[]string{"    1        END", "  1 1  C FIRST", "%FORT-W-W, T"})
}

// TestMessageWithoutLine: a message about no line comes after the source, whole.
func TestMessageWithoutLine(t *testing.T) {
	in := testInput(lines(&source.File{Spec: "P"}, 0, 1, "      END"))
	in.Messages = []diag.Message{{Severity: diag.Fatal, Ident: "X", Text: "gone", Pos: diag.Position{File: "P"}}}

	compare(t, Make(in)[3:], []string{"    1        END", "", "%FORT-F-X, gone", "  in P"})
}

// TestEmptySource still has a page.
func TestEmptySource(t *testing.T) {
	got := Make(testInput(nil))
	if len(got) != 3 || !strings.HasPrefix(got[0], heading("FILE", 1)) {
		t.Errorf("got %q", got)
	}
}

// TestNumberWidth: numbers line up by the longest, and a tab goes to its stop
// whatever is before the line.
func TestNumberWidth(t *testing.T) {
	texts := make([]string, 100)
	for i := range texts {
		texts[i] = "\tX"
	}

	got := Make(testInput(lines(&source.File{Spec: "P"}, 0, 1, texts...)))
	if got[3] != "      1          X" {
		t.Errorf("line 1 is %q", got[3])
	}

	if last := got[len(got)-1]; last != "    100          X" {
		t.Errorf("line 100 is %q", last)
	}
}

// pageHeads is each page's title and subtitle, and how many lines of text it has.
func pageHeads(out []string) []string {
	var heads []string

	for i := 0; i < len(out); i++ {
		if strings.HasPrefix(out[i], "\f") {
			heads = append(heads, strings.TrimSpace(out[i][1:42])+"/"+strings.TrimSpace(out[i+1][:41]))
		}
	}

	return heads
}

// TestTitles is section 10.3.3's rules: a directive heads the next page, the last on
// a page winning. With none the title is the unit's name, and each unit starts a page.
func TestTitles(t *testing.T) {
	texts := make([]string, 200)
	for i := range texts {
		texts[i] = "C"
	}

	in := testInput(lines(&source.File{Spec: "P"}, 0, 1, texts...))
	in.Units = []Unit{{0, "FIRST"}, {70, "SECOND"}, {75, "THIRD"}}
	in.Headings = []Heading{
		{Line: 72, Text: "EARLY"},
		{Line: 73, Text: "LATE"},
		{Line: 74, Subtitle: true, Text: "SUB"},
	}

	got := pageHeads(Make(in))
	want := []string{"FIRST/", "FIRST/", "SECOND/", "LATE/SUB", "LATE/SUB"}

	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestPages: no page is longer than 66 lines, and a line stays with its messages.
func TestPages(t *testing.T) {
	texts := make([]string, 63)
	for i := range texts {
		texts[i] = "C"
	}

	in := testInput(lines(&source.File{Spec: "P"}, 0, 1, texts...))
	in.Messages = []diag.Message{{Ident: "M", Text: "T", Pos: diag.Position{Line: 63, Serial: 63}}}

	out := Make(in)

	if !strings.HasPrefix(out[62+3], "\f") || !strings.HasSuffix(out[62+3+3], "C") || out[62+3+4] != "%FORT-I-M, T" {
		t.Errorf("the last line and its message: %q", out[60:])
	}
}

// TestShowKeywords checks /SHOW's keywords, applied in order.
func TestShowKeywords(t *testing.T) {
	cases := []struct {
		words []string
		want  Show
		fails bool
	}{
		{nil, Show{true, true}, false},
		{[]string{"ALL"}, Show{true, true}, false},
		{[]string{"NONE"}, Show{}, false},
		{[]string{"INCLUDE"}, Show{true, true}, false},
		{[]string{"NOMAP"}, Show{}, false},
		{[]string{"ALL", "NOINCLUDE"}, Show{false, true}, false},
		{[]string{"inc"}, Show{true, true}, false},
		{[]string{"NOI"}, Show{false, true}, false},
		{[]string{"NO"}, Show{}, true},
		{[]string{"SINGLE"}, Show{}, true},
	}

	for _, c := range cases {
		got, err := DefaultShow().With(c.words)
		if (err != nil) != c.fails || (err == nil && got != c.want) {
			t.Errorf("%v: got %+v, %v", c.words, got, err)
		}
	}
}
