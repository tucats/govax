package securertl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestTables checks the transfer vector's entries: each once, a system
// service's name (the P1 vector's services aren't SECURESHR's), with codes
// in this package's range, each registered.
func TestTables(t *testing.T) {
	names, codes := map[string]bool{}, map[uint32]string{}

	for _, l := range Links() {
		if names[l.Name] {
			t.Errorf("%s is listed twice", l.Name)
		}

		names[l.Name] = true

		if l.Library != Library || l.Code < codeBase || l.Code >= 10000 || l.Code != Code(l.Name) {
			t.Errorf("%s: library %s, code %d", l.Name, l.Library, l.Code)
		}

		if other, dup := codes[l.Code]; dup {
			t.Errorf("%s and %s share code %d", l.Name, other, l.Code)
		}

		codes[l.Code] = l.Name

		for _, e := range vmsdef.P1VectorTable {
			if e.Name == l.Name {
				t.Errorf("%s is in the P1 vector too", l.Name)
			}
		}
	}

	table := corevms.NewShimTable()
	Register(table)

	for code, name := range codes {
		if !table.Has(code) {
			t.Errorf("%s: code %d not registered", name, code)
		}
	}
}

// TestFirstACE: what $FORMAT_ACL takes from its descriptor (round 3b's
// cases 150 to 157).
func TestFirstACE(t *testing.T) {
	cases := []struct {
		raw  []byte
		size int
		ok   bool
	}{
		{[]byte{0, 1, 0, 0, 1, 0, 0, 0}, 8, true},                        // size 0: the descriptor's
		{[]byte{4, 1, 0, 0, 1, 0, 0, 0}, 8, true},                        // smaller than a header
		{[]byte{2, 1, 0, 0}, 4, true},                                    // short both ways
		{[]byte{12, 1, 0, 0, 1, 0, 0, 0}, 12, true},                      // the descriptor short
		{append(make12(), make12()...), 12, true},                        // two whole ACEs
		{append(make12(), make([]byte, 244)...), 0, false},               // zeros after
		{append(make12(), 3, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0), 0, false}, // an ACE of size 3 after
	}

	for i, c := range cases {
		ace, ok := firstACE(c.raw)
		if ok != c.ok || ok && len(ace) != c.size {
			t.Errorf("case %d: %d bytes, %v; want %d, %v", i, len(ace), ok, c.size, c.ok)
		}
	}
}

// make12 is a 12-byte identifier ACE.
func make12() []byte { return []byte{12, 1, 0, 0, 1, 0, 0, 0, 1, 0, 0x80, 0} }
