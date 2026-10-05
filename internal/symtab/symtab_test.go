package symtab

import "testing"

func table() *Table {
	t := New()
	t.Set(Symbol{Name: "COUNT", Value: 0x200, Flags: Data})
	t.Set(Symbol{Name: "TABLE", Value: 0x204, Flags: Data, Size: 16})
	t.Set(Symbol{Name: "START", Value: 0x400, Flags: Entry | Global})
	t.Set(Symbol{Name: "BEGIN", Value: 0x400, Flags: Label})
	t.Set(Symbol{Name: "LOOP", Value: 0x4DD, Flags: Label})
	t.Set(Symbol{Name: "LIMIT", Value: 10, Flags: Literal})

	return t
}

func TestGetIgnoresCase(t *testing.T) {
	s, ok := table().Get("start")
	if !ok || s.Name != "START" || !s.IsEntry() {
		t.Fatalf("Get(start) = %+v, %v", s, ok)
	}
}

func TestSetReplaces(t *testing.T) {
	tab := table()
	tab.Set(Symbol{Name: "count", Value: 0x300})

	if s, _ := tab.Get("COUNT"); s.Value != 0x300 || s.Name != "count" {
		t.Errorf("COUNT = %+v", s)
	}

	if tab.Len() != 6 {
		t.Errorf("Len = %d, want 6", tab.Len())
	}

	// The address index sees the change.
	if s, ok := tab.At(0x300, nil); !ok || s.Name != "count" {
		t.Errorf("At(0x300) = %+v, %v", s, ok)
	}
}

func TestAt(t *testing.T) {
	tab := table()

	// Two symbols at 0x400: the first by name, unless match picks.
	if s, _ := tab.At(0x400, nil); s.Name != "BEGIN" {
		t.Errorf("At(0x400) = %s, want BEGIN", s.Name)
	}

	isEntry := func(s *Symbol) bool { return s.IsEntry() }
	if s, _ := tab.At(0x400, isEntry); s.Name != "START" {
		t.Errorf("At(0x400, entry) = %s, want START", s.Name)
	}

	if _, ok := tab.At(0x401, nil); ok {
		t.Error("At(0x401) found a symbol")
	}
}

func TestNearest(t *testing.T) {
	tab := table()
	notLiteral := func(s *Symbol) bool { return !s.Has(Literal) }

	for _, tc := range []struct {
		v      uint32
		match  func(*Symbol) bool
		name   string
		offset uint32
		ok     bool
	}{
		{0x20C, nil, "TABLE", 8, true},
		{0x402, nil, "BEGIN", 2, true},
		{0x4E0, nil, "LOOP", 3, true},
		{0x200, nil, "COUNT", 0, true},
		{0x100, nil, "LIMIT", 0xF6, true},
		{0x100, notLiteral, "", 0, false},
		{5, nil, "", 0, false},
		{0x402, func(s *Symbol) bool { return s.IsEntry() }, "START", 2, true},
	} {
		s, off, ok := tab.Nearest(tc.v, tc.match)
		if ok != tc.ok || (ok && (s.Name != tc.name || off != tc.offset)) {
			t.Errorf("Nearest(%#x) = %v+%#x, %v; want %s+%#x, %v", tc.v, s, off, ok, tc.name, tc.offset, tc.ok)
		}
	}
}

func TestDeleteIf(t *testing.T) {
	tab := table()

	n := tab.DeleteIf(func(s *Symbol) bool { return s.Has(Label) })
	if n != 2 || tab.Len() != 4 {
		t.Fatalf("DeleteIf removed %d, left %d", n, tab.Len())
	}

	if s, _ := tab.At(0x400, nil); s.Name != "START" {
		t.Errorf("At(0x400) after DeleteIf = %s, want START", s.Name)
	}

	tab.Delete("start")

	if _, ok := tab.Get("START"); ok {
		t.Error("START survived Delete")
	}
}

func TestAllSorted(t *testing.T) {
	all := table().All()

	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Fatalf("All not sorted: %s before %s", all[i-1].Name, all[i].Name)
		}
	}
}
