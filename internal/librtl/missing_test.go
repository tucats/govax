package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestMissing: every LIBRTL.EXE entry has a stub, once: Routines', or a
// missing routine's. JSB entries and data tables have none, aliases
// share one, and the codes are distinct from every other shim's.
func TestMissing(t *testing.T) {
	missing := Missing()

	byName := map[string]Routine{}
	offsets := map[uint32]string{}
	codes := map[uint32]string{}

	for _, l := range corevms.CRTLLinks {
		codes[l.Code] = l.Name
	}

	for _, r := range append(append([]Routine(nil), Routines...), missing...) {
		if other, dup := offsets[r.Offset()]; dup {
			t.Errorf("%s and %s share offset %#x", r.Name, other, r.Offset())
		}

		if other, dup := codes[r.Code]; dup && r.Code != 0 {
			t.Errorf("%s and %s share code %d", r.Name, other, r.Code)
		}

		offsets[r.Offset()], codes[r.Code], byName[r.Name] = r.Name, r.Name, r
	}

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image != Library {
			continue
		}

		_, stub := byName[name]
		skipped := jsbEntry.MatchString(name) || dataEntry(name)

		switch {
		case skipped && stub:
			t.Errorf("%s has a stub, but is a JSB entry or data", name)
		case !skipped && offsets[sym.Value] == "":
			t.Errorf("%s (offset %#x) has no stub", name, sym.Value)
		}
	}

	checks := []struct{ name, want string }{
		{"LIB$INDEX", "LIB$INDEX"},
		{"LIB$MOVC3", "LIB$MOVC3"},       // ends in a digit, but a CALL entry
		{"FOR$CNV_IN_I", "OTS$CVT_TI_L"}, // an alias: OTS$ ranks first
		{"LIB$SCOPY_DXDX6", ""},          // JSB
		{"STR$DUPL_CHARR8", ""},          // JSB
		{"LIB$AB_UPCASE", ""},            // data
		{"LIB$PUT_OUTPUT", "LIB$PUT_OUTPUT"},
	}

	for _, c := range checks {
		got := offsets[vmsdef.ImageSymbols[c.name].Value]
		if c.want == "" {
			if _, ok := byName[c.name]; ok {
				t.Errorf("%s has a stub; want none", c.name)
			}

			continue
		}

		if got != c.want {
			t.Errorf("%s's stub is %q, want %q", c.name, got, c.want)
		}
	}

	for _, r := range missing {
		if r.Code < missingCodeBase || r.Fn == nil {
			t.Errorf("%s: code %d, Fn nil %v", r.Name, r.Code, r.Fn == nil)
		}
	}
}

// TestMissingRegistered: Register installs the missing routines' shims.
func TestMissingRegistered(t *testing.T) {
	env := fixture(t)

	for _, r := range Missing() {
		if _, ok := env.Shims().Lookup(r.Code); !ok {
			t.Errorf("%s (code %d) isn't registered", r.Name, r.Code)
		}
	}
}
