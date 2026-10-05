package console

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/disasm"
)

// dbgImagePath is a path to one of testdata/dbg/vax's images: Phase 41's
// probe, linked by real LINK (DBGDIS) and by govax (GVDBGDIS).
func dbgImagePath(t *testing.T, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "dbg", "vax", name)
}

// TestSharedImageReference: DBGDIS's CALLS #1,G^LIB$PUT_OUTPUT (line 93)
// goes through a fixup cell at SUB2+0F0. By default it's shown as VMS's
// debugger showed it; with the console as the cell namer, as the source
// wrote it. Both real LINK's image and govax's are checked.
func TestSharedImageReference(t *testing.T) {
	for _, image := range []string{"dbgdis.exe", "gvdbgdis.exe"} {
		c, _ := runStepped(t, dbgImagePath(t, image))

		main := c.findMainICB()
		if main == nil || main.Debug == nil {
			t.Fatalf("%s: no main image with debug symbols", image)
		}

		var addr uint32

		for _, m := range main.Debug.Modules {
			if m.Name == "DBGDIS" {
				addr, _ = m.AddressOfLine(93)
			}
		}

		if addr == 0 {
			t.Fatalf("%s: no line 93 in DBGDIS", image)
		}

		dec, err := c.decodeInstruction(memByteReader{c: c}, addr)
		if err != nil {
			t.Fatalf("%s: decoding line 93: %v", image, err)
		}

		names := dbgsym.Names{Program: main.Debug}

		cell := dec.Operands[1].Target
		if _, ok := main.Cells[cell]; !ok {
			t.Errorf("%s: %08X isn't among the image's fixup cells %v", image, cell, main.Cells)
		}

		for _, tc := range []struct {
			opts disasm.Options
			want string
		}{
			{disasm.Options{Style: disasm.StyleDebugger, Symbolizer: names}, "CALLS    S^#01,@L^SUB2+0F0"},
			{disasm.Options{Style: disasm.StyleDebugger, Symbolizer: names, Cells: c}, "CALLS    S^#01,G^LIB$PUT_OUTPUT"},
			{disasm.Options{Cells: c}, "CALLS S^#1,G^LIB$PUT_OUTPUT"},
		} {
			if got := dec.Format(tc.opts); got != tc.want {
				t.Errorf("%s: line 93 is %q, want %q", image, got, tc.want)
			}
		}

		// The cell holds the shim's stub, which the console's table names.
		target, err := c.loadLong(cell)
		if err != nil {
			t.Fatalf("%s: reading the cell: %v", image, err)
		}

		if name, ok := c.Symbols.EntryAt(target); !ok || name != "LIB$PUT_OUTPUT" {
			t.Errorf("%s: the cell holds %08X, named %q (%v), want LIB$PUT_OUTPUT's stub", image, target, name, ok)
		}
	}
}

// TestSharedName: an offset in a shareable image is named by the shim
// that stands in for it, else by VMS's image's own symbols.
func TestSharedName(t *testing.T) {
	cases := []struct {
		image  string
		offset uint32
		want   string
	}{
		{"LIBRTL", 0x0478, "LIB$PUT_OUTPUT"}, // a shim
		{"LIBRTL", 0x0940, "COB$CNVOUT"},     // no shim: vmsdef.ImageSymbols
		{"DECC$SHR", 0x0380, "DECC$PRINTF"},  // a shim for an image vmsdef lacks
		{"LIBRTL", 0x0001, ""},               // not an entry
		{"NOSUCH", 0x0478, ""},
	}

	for _, tc := range cases {
		got, ok := sharedName(tc.image, tc.offset)
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("sharedName(%s, %X) = %q, %v; want %q", tc.image, tc.offset, got, ok, tc.want)
		}
	}
}

// TestSharedSymbolizer: an address in a loaded shareable image (here a
// stand-in ICB, as a real LIBRTL.EXE would be loaded) is named by the
// image's symbol at its offset; the main image's addresses aren't.
func TestSharedSymbolizer(t *testing.T) {
	c := &Console{}
	c.ICBList = []*ICB{
		{Name: "MAIN", Base: 0, End: 0x1FFF, Flags: icbMain},
		{Name: "LIBRTL", Base: 0x10000, End: 0x30FFF},
	}

	s := sharedSymbolizer{c: c}

	if got, ok := s.Symbolize(0x10478); !ok || got != "LIB$PUT_OUTPUT" {
		t.Errorf("Symbolize(10478) = %q, %v; want LIB$PUT_OUTPUT", got, ok)
	}

	if got, ok := s.Symbolize(0x478); ok {
		t.Errorf("Symbolize(478), in the main image, = %q; want nothing", got)
	}

	if got, ok := s.Symbolize(0x40478); ok {
		t.Errorf("Symbolize(40478), outside every image, = %q; want nothing", got)
	}
}
