package diff

import (
	"strings"
	"testing"
)

func records(lines ...string) [][]byte {
	out := make([][]byte, len(lines))
	for i, l := range lines {
		out[i] = []byte(l)
	}

	return out
}

// The author's session on the VAX (the phase doc's Goal): two records
// inserted are one section, no master records, the revision's two, and
// /MERGED's one matching record after each side.
func TestMergedInsertion(t *testing.T) {
	master := Input{Name: "SYS$SYSROOT:[SYSMGR]FOO.TXT;10", Records: records("line 1", "line 2", "line 3", "line 4")}
	revision := Input{Name: "SYS$SYSROOT:[SYSMGR]FOO.TST;1",
		Records: records("line 1", "line 2", "extra cat", "extra dog", "line 3", "line 4")}

	res := Compare(master.Records, revision.Records, Options{Match: 3})

	l := NewListing(132)
	Merged(l, master, revision, res, Style{After: 1})
	Summary(l, res)
	Echo(l, []string{"/IGNORE=()", "/MERGED=1"}, []string{master.Name, revision.Name})

	want := `************
File SYS$SYSROOT:[SYSMGR]FOO.TXT;10
    3   line 3
******
File SYS$SYSROOT:[SYSMGR]FOO.TST;1
    3   extra cat
    4   extra dog
    5   line 3
************

Number of difference sections found: 1
Number of difference records found: 2

DIFFERENCES /IGNORE=()/MERGED=1-
    SYS$SYSROOT:[SYSMGR]FOO.TXT;10-
    SYS$SYSROOT:[SYSMGR]FOO.TST;1
`
	if got := l.Text(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// Records that agree inside a section, fewer than /MATCH, are part of it,
// and count (testdata/dif116's A16).
func TestSectionTakesInShortMatches(t *testing.T) {
	a := records("1", "2", "x", "4", "y", "6", "7", "8", "9")
	b := records("1", "2", "X", "4", "Y", "6", "7", "8", "9")

	res := Compare(a, b, Options{Match: 3})
	if len(res.Sections) != 1 || res.Records != 3 {
		t.Fatalf("got %+v, want one section of 3", res)
	}

	if s := res.Sections[0]; s.Master != (Span{2, 5}) || s.Revision != (Span{2, 5}) {
		t.Errorf("section %+v", s)
	}
}

func TestIdenticalFiles(t *testing.T) {
	a := records("one", "two")

	res := Compare(a, a, Options{Match: 3})
	if len(res.Sections) != 0 || res.Records != 0 {
		t.Errorf("got %+v", res)
	}

	l := NewListing(132)
	Summary(l, res)

	if got := l.Text(); got != "Number of difference sections found: 0\nNumber of difference records found: 0\n\n" {
		t.Errorf("got %q", got)
	}
}

// The echo's and the counts' breaking at narrow widths, as VMS 7.1 broke
// them (testdata/dif116's N37 at /WIDTH=0, which is 12, and N38 at 20).
func TestNarrowWidths(t *testing.T) {
	names := []string{"DUA1:[DIF116.IN]A02.A;1", "DUA1:[DIF116.IN]A02.B;1"}

	for _, tc := range []struct {
		width  int
		pieces []string
		want   string
	}{
		{0, []string{"/IGNORE=()", "/WIDTH=12", "/MERGED=1"}, `Number of d-
ifference s-
ections fou-
nd: 1
DIFFERENCES-
 /IGNORE=()-
    /WIDTH=-
12/MERGED=1-
    DUA1:[D-
IF116.IN]A0-
2.A;1-
    DUA1:[D-
IF116.IN]A0-
2.B;1
`},
		{20, []string{"/IGNORE=()", "/WIDTH=20", "/MERGED=1"}, `Number of differenc-
e sections found: 1
DIFFERENCES -
    /IGNORE=()-
    /WIDTH=20-
    /MERGED=1-
    DUA1:[DIF116.IN-
]A02.A;1-
    DUA1:[DIF116.IN-
]A02.B;1
`},
	} {
		l := NewListing(tc.width)
		l.Wrap("Number of difference sections found: 1")
		Echo(l, tc.pieces, names)

		if got := l.Text(); got != tc.want {
			t.Errorf("width %d: got\n%s\nwant\n%s", tc.width, got, tc.want)
		}
	}
}

// A list's pieces break between elements (N64, at /WIDTH=80).
func TestEchoBreaksLists(t *testing.T) {
	l := NewListing(80)
	Echo(l, []string{
		"/IGNORE=(SPACING,", "TRAILING_SPACES,", "HEADER=2,", "FORM_FEEDS,", "CASE,", "EXACT,", "COMMENTS,",
		"BLANK_LINES)", "/COMMENT_DELIMITERS=(EXCLAMATION,", `";",`, "COLON,", "TAB)", "/WIDTH=80", "/MATCH=4",
		"/MAXIMUM_DIFFERENCES=50", "/MERGED=2", "/MODE=(HEXADECIMAL,", "ASCII)",
		"/OUTPUT=DUA1:[DIF116.OUT]N64.DIF;1", "/PARALLEL=1", "/SEPARATED=MASTER", "/WINDOW=10", "/NONUMBER",
	}, []string{`DUA1:[DIF116.IN]F01.A;1/CHANGE_BAR=("%")`, `DUA1:[DIF116.IN]F01.B;1/CHANGE_BAR=("#")`})

	want := strings.Join([]string{
		"DIFFERENCES /IGNORE=(SPACING,TRAILING_SPACES,HEADER=2,FORM_FEEDS,CASE,EXACT,-",
		`    COMMENTS,BLANK_LINES)/COMMENT_DELIMITERS=(EXCLAMATION,";",COLON,TAB)-`,
		"    /WIDTH=80/MATCH=4/MAXIMUM_DIFFERENCES=50/MERGED=2/MODE=(HEXADECIMAL,ASCII)-",
		"    /OUTPUT=DUA1:[DIF116.OUT]N64.DIF;1/PARALLEL=1/SEPARATED=MASTER/WINDOW=10-",
		"    /NONUMBER-",
		`    DUA1:[DIF116.IN]F01.A;1/CHANGE_BAR=("%")-`,
		`    DUA1:[DIF116.IN]F01.B;1/CHANGE_BAR=("#")`,
	}, "\n") + "\n"

	if got := l.Text(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// The printable form /PARALLEL uses (testdata/dif116's R11 and R12): tabs to
// every eighth column, LF, VT, FF, and CR named, other controls, DEL, the C1
// controls, and %XFF a period.
func TestPretty(t *testing.T) {
	in := []byte("\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\v\f\r\x0eX\x7f\x80\x9f\xa0\xfe\xff")
	want := ".........       <LF><VT><FF><CR>.X...\xa0\xfe."

	if got := Pretty(in); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A change bar marks a section's records on its file's side, or where that
// side has none, the record after the place (F40, F41).
func TestMarks(t *testing.T) {
	a := records("1", "2", "3", "4", "5", "6", "7")
	b := records("1", "x", "2", "3", "4", "5", "7")
	res := Compare(a, b, Options{Match: 3})

	show := func(m []bool) string {
		var s strings.Builder

		for _, x := range m {
			if x {
				s.WriteByte('!')
			} else {
				s.WriteByte('.')
			}
		}

		return s.String()
	}

	if got := show(Marks(res, len(a), masterSpan)); got != ".!...!." {
		t.Errorf("master marks %s", got)
	}

	if got := show(Marks(res, len(b), revisionSpan)); got != ".!....!" {
		t.Errorf("revision marks %s", got)
	}
}

// TestDumpLayout holds the dump's line to the widths the probe showed (8
// hexadecimal longwords at 132, 4 at 80, 6 octal ones at 132) and its
// partial longwords' digits.
func TestDumpLayout(t *testing.T) {
	for _, c := range []struct {
		radix         Radix
		width, longwd int
	}{
		{Hexadecimal, 132, 8}, {Hexadecimal, 80, 4}, {Octal, 132, 6}, {Hexadecimal, 12, 2},
	} {
		if d := newDumpLayout(c.radix, c.width); d.longwords != c.longwd {
			t.Errorf("radix %d at %d: %d longwords, want %d", c.radix, c.width, d.longwords, c.longwd)
		}
	}

	// A20.A's first record in octal (F49): a 3-digit longword of one byte.
	d := newDumpLayout(Octal, 132)
	want := strings.Repeat(" ", 57) + "143 01130404541 a.b.c" + strings.Repeat(".", 19) + " 000000"

	if got := d.line([]byte("a\tb\tc"), 0); got != want {
		t.Errorf("octal line:\n%q\nwant\n%q", got, want)
	}
}
