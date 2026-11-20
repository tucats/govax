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
		// A RECURSIVE unit's: a dummy through a frame cell, and a descriptor in the frame.
		{Unit: "U", Name: "P", Type: Char, Size: 0, Place: Place{Kind: ThroughFrame, N: -56}},
		{Unit: "U", Name: "X", Type: Integer, Size: 4, Place: Place{Kind: ThroughFrame, N: -52}, Bounds: []Bound{{}}, DescriptorInFrame: true, DescriptorOffset: -40},
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

// TestStructureLines reads back a record's layout: a derived type nesting another, a
// DEC structure, and a variable of each, whatever order the lines come in.
func TestStructureLines(t *testing.T) {
	point := &Structure{Name: "POINT", Size: 8, Derived: true, Components: []*Component{
		{Name: "X", Offset: 0, Type: RealF, Size: 4},
		{Name: "Y", Offset: 4, Type: RealF, Size: 4},
	}}
	shape := &Structure{Name: "SHAPE", Size: 36, Derived: true, Components: []*Component{
		{Name: "TAG", Offset: 0, Type: Char, Size: 4},
		{Name: "P", Offset: 4, Type: Record, Struct: "POINT", Size: 8, Bounds: []Bound{{Lower: 1, Upper: 4, Known: true}}},
	}}
	date := &Structure{Name: "DATE", Size: 4, Components: []*Component{{Name: "DAY", Type: Integer, Size: 2}, {Name: "$1", Offset: 2, Type: Record, Struct: "$1", Size: 2}}}
	v := Var{Unit: "U", Name: "S", Type: Record, Struct: "SHAPE", Size: 36, Place: Place{Kind: Static, Symbol: "U.S"}}

	// A component's line before its structure's.
	lines := []string{ComponentLine("U", "POINT", point.Components[0]), v.Line(), StructureLine("U", point),
		ComponentLine("U", "POINT", point.Components[1])}

	for _, s := range []*Structure{shape, date} {
		lines = append(lines, StructureLine("U", s))
		for _, c := range s.Components {
			lines = append(lines, ComponentLine("U", s.Name, c))
		}
	}

	u := Parse(lines).Units["U"]

	for _, want := range []*Structure{point, shape, date} {
		if got := u.Structures[want.Name]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, want %+v", want.Name, got, want)
		}
	}

	if got := u.Vars["S"]; got == nil || !reflect.DeepEqual(*got, v) {
		t.Errorf("S: got %+v, from %q", got, v.Line())
	}

	if point.Separator() != "%" || date.Separator() != "." || shape.Component("P") != shape.Components[1] || shape.Component("Q") != nil {
		t.Error("Separator or Component")
	}

	// A plain REC, as files before the structure lines have it, is still read.
	if got := Parse([]string{Prefix + "V U R REC 8 S:U.R - -"}).Units["U"].Vars["R"]; got == nil || got.Type != Record || got.Struct != "" {
		t.Errorf("a plain REC: %+v", got)
	}
}
