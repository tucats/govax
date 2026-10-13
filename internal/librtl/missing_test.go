package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestMissing: every LIBRTL.EXE entry is covered once: by a CALL stub
// (Routines', or a missing routine's), a JSB stub (JSBRoutines', or a
// missing JSB entry's), or a data table. Aliases share one, the kinds
// match the names, and the codes are distinct from every other shim's.
func TestMissing(t *testing.T) {
	missing := Missing()

	byName := map[string]corevms.ShimLink{}
	offsets := map[uint32]string{}
	codes := map[uint32]string{}

	for _, l := range corevms.CRTLLinks {
		codes[l.Code] = l.Name
	}

	for _, l := range Links() {
		if other, dup := offsets[l.Offset]; dup {
			t.Errorf("%s and %s share offset %#x", l.Name, other, l.Offset)
		}

		if other, dup := codes[l.Code]; dup && l.Code != 0 {
			t.Errorf("%s and %s share code %d", l.Name, other, l.Code)
		}

		offsets[l.Offset], codes[l.Code], byName[l.Name] = l.Name, l.Name, l
	}

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image != Library {
			continue
		}

		if offsets[sym.Value] == "" {
			t.Errorf("%s (offset %#x) has no stub or table", name, sym.Value)

			continue
		}

		l, ok := byName[name]
		if !ok {
			continue // an alias
		}

		switch {
		case dataEntry(name) != (l.Data != nil):
			t.Errorf("%s: data %v, want %v", name, l.Data != nil, dataEntry(name))
		case jsbEntry.MatchString(name) != l.JSB:
			t.Errorf("%s: JSB %v, want %v", name, l.JSB, jsbEntry.MatchString(name))
		}
	}

	checks := []struct{ name, want string }{
		{"LIB$INDEX", "LIB$INDEX"},
		{"LIB$MOVC3", "LIB$MOVC3"},       // ends in a digit, but a CALL entry
		{"FOR$CNV_IN_I", "OTS$CVT_TI_L"}, // an alias: OTS$ ranks first
		{"LIB$SCOPY_DXDX6", "LIB$SCOPY_DXDX6"},
		{"COB$CVTDP_R9", "OTS$CVTDP_R9"}, // a JSB alias
		{"LIB$AB_UPCASE", "LIB$AB_UPCASE"},
		{"LIB$PUT_OUTPUT", "LIB$PUT_OUTPUT"},
	}

	for _, c := range checks {
		if got := offsets[vmsdef.ImageSymbols[c.name].Value]; got != c.want {
			t.Errorf("%s's stub is %q, want %q", c.name, got, c.want)
		}
	}

	for _, r := range missing {
		if r.Code < missingCodeBase || r.Code >= missingJSBCodeBase || r.Fn == nil {
			t.Errorf("%s: code %d, Fn nil %v", r.Name, r.Code, r.Fn == nil)
		}
	}

	for _, r := range MissingJSB() {
		if r.Code < missingJSBCodeBase || r.Fn == nil {
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

	for _, r := range MissingJSB() {
		if _, ok := env.Shims().LookupJSB(r.Code); !ok {
			t.Errorf("%s (code %d) isn't registered as a JSB routine", r.Name, r.Code)
		}
	}
}
