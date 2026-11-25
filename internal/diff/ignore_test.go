package diff

import (
	"strings"
	"testing"
)

// TestIgnoreEdits checks each edit and what a record shows, on records like
// the probe's I01 and I20.
func TestIgnoreEdits(t *testing.T) {
	in := Input{Name: "A", Records: [][]byte{
		[]byte("\f"), []byte("Title"), []byte("Date"), []byte("x  =\t1   ! note"),
		[]byte("   "), []byte("a\fb"), []byte("\fPage 2"), []byte("Date 2"), []byte("end"),
	}}

	tests := []struct {
		name  string
		ig    Ignore
		keys  string
		shown string
		nums  string
	}{
		{"none", Ignore{}, "\f|Title|Date|x  =\t1   ! note|   |a\fb|\fPage 2|Date 2|end", "", "1 2 3 4 5 6 7 8 9"},
		{"spacing", Ignore{Spacing: true}, "\f|Title|Date|x = 1 ! note| |a\fb|\fPage 2|Date 2|end", "", "1 2 3 4 5 6 7 8 9"},
		{"comments trailing blank", Ignore{Comments: []Delimiter{{Char: '!'}}, TrailingSpaces: true, BlankLines: true},
			"\f|Title|Date|x  =\t1|a\fb|\fPage 2|Date 2|end", "", "1 2 3 4 6 7 8 9"},
		{"header 1", Ignore{Header: 1}, "\f|Date|x  =\t1   ! note|   |a\fb|Date 2|end", "", "1 3 4 5 6 8 9"},
		{"header 2 form feeds", Ignore{Header: 2, FormFeeds: true}, "|x  =\t1   ! note|   |ab|end", "", "1 4 5 6 9"},
		{"case exact", Ignore{Case: true, Spacing: true, Shown: Exact}, "\f|TITLE|DATE|X = 1 ! NOTE| |A\fB|\fPAGE 2|DATE 2|END",
			"\f|Title|Date|x  =\t1   ! note|   |a\fb|\fPage 2|Date 2|end", "1 2 3 4 5 6 7 8 9"},
		{"pretty", Ignore{TrailingSpaces: true, Shown: PrettyForm}, "\f|Title|Date|x  =\t1   ! note||a\fb|\fPage 2|Date 2|end",
			"<FF>|Title|Date|x  =    1   ! note||a<FF>b|<FF>Page 2|Date 2|end", "1 2 3 4 5 6 7 8 9"},
	}

	for _, tt := range tests {
		keys, shown := tt.ig.Prepare(in)

		var k, s, n []string
		for i := range keys {
			k = append(k, string(keys[i]))
			s = append(s, string(shown.Records[i]))
			n = append(n, string(rune('0'+shown.Numbers[i])))
		}

		if got := strings.Join(k, "|"); got != tt.keys {
			t.Errorf("%s: keys %q, want %q", tt.name, got, tt.keys)
		}

		want := tt.shown
		if want == "" {
			want = tt.keys
			if tt.ig.Case {
				want = strings.Join(s, "|")
			}
		}

		if got := strings.Join(s, "|"); got != want {
			t.Errorf("%s: shown %q, want %q", tt.name, got, want)
		}

		if got := strings.Join(n, " "); got != tt.nums {
			t.Errorf("%s: numbers %q, want %q", tt.name, got, tt.nums)
		}
	}
}

// TestDefaultDelimiters checks the table by type, govax's .F90 among them:
// "!" anywhere, and no comment lines by a letter in column 1.
func TestDefaultDelimiters(t *testing.T) {
	for _, tt := range []struct{ fileType, record, want string }{
		{"FOR", "C a comment line", ""},
		{"FOR", "      X = 1 ! note", "      X = 1 "},
		{"f90", "CALL SUB(X) ! note", "CALL SUB(X) "},
		{"F90", "COUNT = 1", "COUNT = 1"},
		{"COB", "MOVE A TO B. * x", "MOVE A TO B. * x"},
		{"COB", "* a comment", ""},
		{"MAR", "MOVL R0,R1 ; copy", "MOVL R0,R1 "},
		{"TXT", "a ! b", "a ! b"},
	} {
		ig := Ignore{Comments: DefaultDelimiters(tt.fileType)}
		if ig.Comments == nil {
			ig.Comments = []Delimiter{}
		}

		keys, _ := ig.Prepare(Input{Records: [][]byte{[]byte(tt.record)}})
		if got := string(keys[0]); got != tt.want {
			t.Errorf("%s %q: %q, want %q", tt.fileType, tt.record, got, tt.want)
		}
	}
}
