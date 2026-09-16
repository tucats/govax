package console

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func romFixturePath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "rom", "xdefault.rom")
}

func TestLoadROM_realFixture(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.LoadROM(romFixturePath(t)); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}

	if c.ROMBase != 0x20040000 {
		t.Errorf("ROMBase = %#08x, want 0x20040000", c.ROMBase)
	}

	if c.ROMEnd != 0x200BFFFF {
		t.Errorf("ROMEnd = %#08x, want 0x200bffff", c.ROMEnd)
	}

	if len(c.ROM) != 0x80000 {
		t.Errorf("len(ROM) = %#x, want 0x80000", len(c.ROM))
	}
	// A handful of bytes hand-decoded from the fixture's own hex dump, at
	// ROM-relative offset 0 (the first page's data, right after the first
	// page-header ISD).
	want := []byte{0x11, 0x4e, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01}
	if !bytes.Equal(c.ROM[:len(want)], want) {
		t.Errorf("ROM[:8] = % x, want % x", c.ROM[:len(want)], want)
	}
}

func TestSaveROM_roundTripsRealFixtureContent(t *testing.T) {
	// xdefault.rom's own page layout turns out to include two 512-byte
	// entries at addresses that aren't 512-aligned (3328 and 68864 —
	// confirmed by hand-parsing the file's raw (addr,count) entries), so
	// they overlap their neighbors by 256 bytes; whatever produced this
	// fixture wasn't strictly following save_rom's own "base = page*512"
	// convention. SaveROM (matching the *current* save_rom) always emits
	// clean, 512-aligned pages, so a re-saved file can legitimately differ
	// byte-for-byte from this fixture in page framing while still decoding
	// to identical final memory content — which is what this test checks,
	// via a load/save/reload cycle rather than a raw file diff.
	c := New(&bytes.Buffer{})
	orig := romFixturePath(t)

	if err := c.LoadROM(orig); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}

	out := filepath.Join(t.TempDir(), "roundtrip.rom")
	if err := c.SaveROM(out); err != nil {
		t.Fatalf("SaveROM: %v", err)
	}

	c2 := New(&bytes.Buffer{})
	if err := c2.LoadROM(out); err != nil {
		t.Fatalf("LoadROM(roundtrip): %v", err)
	}

	if c2.ROMBase != c.ROMBase || c2.ROMEnd != c.ROMEnd {
		t.Errorf("base/end = %#x/%#x, want %#x/%#x", c2.ROMBase, c2.ROMEnd, c.ROMBase, c.ROMEnd)
	}

	if !bytes.Equal(c2.ROM, c.ROM) {
		t.Error("re-saved and reloaded ROM content differs from the original load")
	}
}

func TestSaveLoadROM_synthetic(t *testing.T) {
	c := New(&bytes.Buffer{})
	c.ROMBase = 0x20040000
	c.ROMEnd = 0x20040000 + 3*512 - 1
	c.ROM = make([]byte, 3*512)
	// Page 0 all zero (should be skipped on save), page 1 has data, page 2
	// all zero again.
	for i := range 16 {
		c.ROM[512+i] = byte(i + 1)
	}

	path := filepath.Join(t.TempDir(), "synthetic.rom")
	if err := c.SaveROM(path); err != nil {
		t.Fatalf("SaveROM: %v", err)
	}

	c2 := New(&bytes.Buffer{})
	if err := c2.LoadROM(path); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}

	if c2.ROMBase != c.ROMBase || c2.ROMEnd != c.ROMEnd {
		t.Errorf("base/end = %#x/%#x, want %#x/%#x", c2.ROMBase, c2.ROMEnd, c.ROMBase, c.ROMEnd)
	}

	if !bytes.Equal(c2.ROM, c.ROM) {
		t.Errorf("ROM content mismatch after round trip")
	}
}

func TestLoadROM_rejectsWrongMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notrom.bin")
	if err := os.WriteFile(path, []byte("not a rom image at all!"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	c := New(&bytes.Buffer{})
	if err := c.LoadROM(path); err == nil {
		t.Error("expected an error loading a non-ROM file")
	}
}

func TestSaveLoadNVRAM_roundTrip(t *testing.T) {
	c := New(&bytes.Buffer{})
	c.NVRAMBase = 0x20140400
	c.NVRAM = []byte{1, 2, 3, 4, 5, 6, 7, 8}

	path := filepath.Join(t.TempDir(), "synthetic.nvram")
	if err := c.SaveNVRAM(path); err != nil {
		t.Fatalf("SaveNVRAM: %v", err)
	}

	c2 := New(&bytes.Buffer{})
	if err := c2.LoadNVRAM(path); err != nil {
		t.Fatalf("LoadNVRAM: %v", err)
	}

	if c2.NVRAMBase != c.NVRAMBase {
		t.Errorf("NVRAMBase = %#x, want %#x", c2.NVRAMBase, c.NVRAMBase)
	}
	
	if !bytes.Equal(c2.NVRAM, c.NVRAM) {
		t.Errorf("NVRAM content mismatch: got % x, want % x", c2.NVRAM, c.NVRAM)
	}
}

func TestSaveROM_errorsWithNoImage(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.SaveROM(filepath.Join(t.TempDir(), "x.rom")); err == nil {
		t.Error("expected an error saving with no ROM loaded")
	}
}
