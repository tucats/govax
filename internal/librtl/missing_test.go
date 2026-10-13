package librtl

import (
	"fmt"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestMissing: every LIBRTL.EXE and LIBRTL2.EXE entry is covered once: by a CALL stub
// (Routines', or a missing routine's), a JSB stub (JSBRoutines', or a
// missing JSB entry's), or a data table. Aliases share one, the kinds
// match the names, and the codes are distinct from every other shim's.
func TestMissing(t *testing.T) {
	missing := Missing()

	byName := map[string]corevms.ShimLink{}
	offsets := map[string]string{} // by place: image and offset
	codes := map[uint32]string{}

	place := func(image string, offset uint32) string { return fmt.Sprintf("%s:%X", image, offset) }

	for _, l := range corevms.CRTLLinks {
		codes[l.Code] = l.Name
	}

	for _, l := range Links() {
		if other, dup := offsets[place(l.Library, l.Offset)]; dup {
			t.Errorf("%s and %s share %s's offset %#x", l.Name, other, l.Library, l.Offset)
		}

		if other, dup := codes[l.Code]; dup && l.Code != 0 {
			t.Errorf("%s and %s share code %d", l.Name, other, l.Code)
		}

		offsets[place(l.Library, l.Offset)], codes[l.Code], byName[l.Name] = l.Name, l.Name, l
	}

	for name, sym := range vmsdef.ImageSymbols {
		if !slices.Contains(libraries, sym.Image) {
			continue
		}

		// LIBRTL2's data tables aren't provided (Tables has only LIBRTL's).
		if sym.Image == Library2 && dataEntry(name) {
			continue
		}

		if offsets[place(sym.Image, sym.Value)] == "" {
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
		{"LIB$SIM_TRAP", "LIB$SIM_TRAP"}, // LIBRTL2's
	}

	for _, c := range checks {
		sym := vmsdef.ImageSymbols[c.name]
		if got := offsets[place(sym.Image, sym.Value)]; got != c.want {
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
