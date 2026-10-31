package vmsdef

import "testing"

// TestFORMessages: the FOR facility's texts are found by any severity of the
// condition, as VMS 7.1 gave them (testdata/for80).
func TestFORMessages(t *testing.T) {
	cases := []struct {
		name, ident, text string
		fao               int
	}{
		{"FOR$_OUTCONERR", "OUTCONERR", "output conversion error!/  unit !SL  file !AS!/  user PC !XL", 3},
		{"FOR$_INVARGFOR", "INVARGFOR", "invalid argument to FORTRAN Run-Time Library!/!+!+  user PC !XL", 3},
		{"FOR$_ADJARRDIM", "ADJARRDIM", "adjustable array dimension error", 0},
		{"FOR$_INVTEX", "INVTEX", `invalid text is "!AS"`, 1},
	}

	for _, c := range cases {
		code, ok := LibrarySymbols[c.name]
		if !ok {
			t.Fatalf("no %s", c.name)
		}

		for sev := uint32(0); sev < 8; sev++ {
			m, ok := LookupMessage(code&^7 | sev)
			if !ok || m.Facility != "FOR" || m.Ident != c.ident || m.Text != c.text || m.FAOCount != c.fao {
				t.Errorf("%s severity %d: %+v, %v", c.name, sev, m, ok)
			}
		}
	}
}
