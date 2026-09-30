package lbr

import (
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

func TestWildMatch(t *testing.T) {
	for _, tc := range []struct {
		pattern, s string
		want       bool
	}{
		{"ABC", "ABC", true},
		{"*", "", true},
		{"*", "ANY", true},
		{"A*", "ABC", true},
		{"*C", "ABC", true},
		{"A*C", "AC", true},
		{"A%C", "ABC", true},
		{"A%C", "AC", false},
		{"*B*B*", "ABCB", true},
		{"*B*B*", "ABC", false},
		{"%%", "A", false},
		{"A*B", "AXBXB", true},
		{"A*B", "AXBX", false},
	} {
		if got := wildMatch(tc.pattern, tc.s); got != tc.want {
			t.Errorf("wildMatch(%q, %q) = %v", tc.pattern, tc.s, got)
		}
	}
}

func TestMatchCase(t *testing.T) {
	mac, _ := Create(TypeMacro)
	obj, _ := Create(TypeObject)

	for _, b := range []*Builder{mac, obj} {
		for _, n := range []string{"ALPHA", "Beta", "GAMMA"} {
			_ = b.Insert(&Entry{Name: n})
		}
	}

	// A macro library upper-cases the pattern; an object library matches
	// case as it is.
	if got := strings.Join(mac.Match("*a"), ","); got != "ALPHA,GAMMA" {
		t.Errorf("macro library *a: %s", got)
	}

	if got := strings.Join(obj.Match("*a"), ","); got != "Beta" {
		t.Errorf("object library *a: %s", got)
	}

	l, _ := Open(mac.Bytes())
	if keys := l.Match("alpha"); len(keys) != 1 || keys[0].Name != "ALPHA" {
		t.Errorf("Library.Match: %v", keys)
	}
}

// TestList checks the listing's lines against LIBRARIAN's formats, for an
// object library with a module whose name is too long for the full
// listing's columns.
func TestList(t *testing.T) {
	at := vmsdef.Time(time.Date(2026, 9, 3, 14, 5, 6, 0, time.UTC))

	b, _ := Create(TypeObject)
	b.Created, b.Updated = at, at

	_ = b.Insert(&Entry{Name: "SHORT", Symbols: []string{"S1", "S2", "S3"}, Inserted: at,
		UserData: []byte{mhdSelectiveSearch, 3, 'V', '1', '0'}})
	_ = b.Insert(&Entry{Name: "A_VERY_LONG_MODULE_NAME", Symbols: []string{"ONLY"}, Inserted: at,
		UserData: []byte{0, 2, 'X', '1'}})

	l, err := Open(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	got, err := l.List(ListOptions{Name: "WORK:[X]T.OLB;1", Now: at, Full: true, Names: true, Width: 70})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"Directory of OBJECT library WORK:[X]T.OLB;1 on  3-SEP-2026 14:05:06",
		"Creation date:   3-SEP-2026 14:05:06      Creator:  govax Librarian",
		"Revision date:   3-SEP-2026 14:05:06      Library format:   3.0",
		"Number of modules:      2                 Max. key length:  31",
		"Other entries:          4                 Preallocated index blocks:     49",
		"Recoverable deleted blocks:      0        Total index blocks used:        2",
		"Max. Number history records:      20      Library history records:        0",
		"",
		"Module A_VERY_LONG_MODULE_NAME Ident X1 Inserted  3-SEP-2026 14:05:06 1 symbol",
		"ONLY",
		"",
		"Module SHORT            Ident V10              Inserted  3-SEP-2026 14:05:06 3 symbols",
		"     Selectively searched",
		"S1                               S2",
		"S3",
		"",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("listing:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// The brief listing is the names.
	got, _ = l.List(ListOptions{Name: "T.OLB", Now: at})
	if tail := strings.Join(got[8:], ","); tail != "A_VERY_LONG_MODULE_NAME,SHORT" {
		t.Errorf("brief listing ends %q", tail)
	}

	if number(123456, 5) != "*****" || number(12, 5) != "   12" {
		t.Error("number's field overflow")
	}
}
