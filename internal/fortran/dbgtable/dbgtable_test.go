package dbgtable

import (
	"reflect"
	"testing"
)

// TestRoundTrip reads back each kind of line it writes.
func TestRoundTrip(t *testing.T) {
	vars := []Var{
		{Unit: "U", Name: "I", Type: Integer, Size: 4, Place: Place{Kind: Static, Symbol: "U.I"}},
		{Unit: "U", Name: "M", Type: RealD, Size: 8, Place: Place{Kind: Static, Symbol: "U.M"}, Bounds: []Bound{{Lower: -1, Upper: 2, Known: true}, {Lower: 1, Upper: 3, Known: true}}},
		{Unit: "U", Name: "A", Type: RealF, Size: 4, Place: Place{Kind: Argument, N: 2}, Bounds: []Bound{{}}, Descriptor: "U.$B1"},
		{Unit: "U", Name: "B", Type: Integer, Size: 2, Place: Place{Kind: Argument, N: 3}, Bounds: []Bound{{Lower: 1, Known: true, Assumed: true}}},
		{Unit: "U", Name: "S", Type: Char, Size: 0, Place: Place{Kind: Through, Symbol: "U.$A1"}},
		{Unit: "U", Name: "F", Type: Logical, Size: 4, Place: Place{Kind: Frame, N: -16}},
		{Unit: "U", Name: "R", Type: Integer, Size: 4, Place: Place{Kind: InRegister, N: 6}},
	}

	lines := []string{"; a comment", EntryLine("U", "U"), EntryLine("U", "OTHER")}
	for _, v := range vars {
		lines = append(lines, v.Line())
	}

	table := Parse(lines)
	if table == nil || table.Entries["OTHER"] != table.Units["U"] {
		t.Fatalf("entries: %+v", table)
	}

	for _, v := range vars {
		got := table.Units["U"].Vars[v.Name]
		if got == nil || !reflect.DeepEqual(*got, v) {
			t.Errorf("%s: got %+v, want %+v, from %q", v.Name, got, v, v.Line())
		}
	}

	if Parse([]string{"; nothing", ".TITLE X"}) != nil {
		t.Error("a table from a file with none")
	}
}

// TestFortranLine holds the source lines' form against what isn't one.
func TestFortranLine(t *testing.T) {
	for record, want := range map[string]int{
		";   30        TOTAL = 0": 30,
		";    1  C  A COMMENT":    1,
		";1  12        INCLUDED":  12,
		"; Storage map for X":     0,
		";   I     INTEGER":       0,
		"\tMOVL\tA,B":             0,
		";":                       0,
	} {
		n, ok := FortranLine(record)
		if n != want || ok != (want != 0) {
			t.Errorf("%q: %d, %v, want %d", record, n, ok, want)
		}
	}
}
