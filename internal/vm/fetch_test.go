package vm

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// The tests in this file cover the instruction-fetch window (fetch.go).
// They use newTranslateFixture's mapping (translate_test.go): virtual P0
// page N maps onto physical page ptBase/pageSize+N, with every page
// readable and writable from every mode.

// fillPattern writes a recognizable byte pattern into the physical pages
// behind the fixture's first npages P0 pages: each byte is its own
// physical address's low 8 bits plus 3*page, so neighboring pages differ
// and a read from the wrong page shows up.
func fillPattern(t *testing.T, mem *Memory, npages int) {
	t.Helper()

	for p := 0; p < npages; p++ {
		base := uint32(ptBase + p*pageSize)

		b, err := mem.phys(base, pageSize)
		if err != nil {
			t.Fatalf("phys page %d: %v", p, err)
		}

		for i := range b {
			b[i] = byte(int(base) + i + 3*p)
		}
	}
}

// remapPage points the fixture's P0 page n at physical page pfn, directly
// in its page table, without invalidating anything.
func remapPage(t *testing.T, mem *Memory, n int, pfn uint32) {
	t.Helper()

	addr := uint32(p0PTPhys + n*4)

	raw, err := mem.readPhysLongword(addr)
	if err != nil {
		t.Fatalf("read PTE %d: %v", n, err)
	}

	pte := PTE(raw)
	pte.SetPFN(pfn)

	if err := mem.writePhysLongword(addr, uint32(pte)); err != nil {
		t.Fatalf("write PTE %d: %v", n, err)
	}
}

// TestFetchMatchesLoad: every Fetch method returns exactly what the
// matching Load method returns, at the start, middle, and end of a page,
// including a word or longword that runs into the next page (which the
// window can't serve, so the slow path must).
func TestFetchMatchesLoad(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)
	fillPattern(t, mem, 4)

	// Move page 2 away from page 1 in physical memory, so a value that
	// spans the two pages is assembled from two unrelated places, as it
	// generally is on a real system.
	remapPage(t, mem, 2, 0x6000>>9)

	if b, err := mem.phys(0x6000, pageSize); err == nil {
		for i := range b {
			b[i] = byte(0xA0 + i)
		}
	}

	addrs := []uint32{
		pageSize, pageSize + 1, pageSize + 0x100,
		2*pageSize - 4, 2*pageSize - 3, 2*pageSize - 2, 2*pageSize - 1,
		2 * pageSize, 2*pageSize + 7,
	}

	for _, addr := range addrs {
		wantB, err := mem.LoadByte(cpu, addr)
		if err != nil {
			t.Fatalf("LoadByte(%#x): %v", addr, err)
		}

		if got, err := mem.FetchByte(cpu, addr); err != nil || got != wantB {
			t.Errorf("FetchByte(%#x) = %#x, %v; want %#x", addr, got, err, wantB)
		}

		wantW, err := mem.LoadWord(cpu, addr)
		if err != nil {
			t.Fatalf("LoadWord(%#x): %v", addr, err)
		}

		if got, err := mem.FetchWord(cpu, addr); err != nil || got != wantW {
			t.Errorf("FetchWord(%#x) = %#x, %v; want %#x", addr, got, err, wantW)
		}

		wantL, err := mem.LoadLongword(cpu, addr)
		if err != nil {
			t.Fatalf("LoadLongword(%#x): %v", addr, err)
		}

		if got, err := mem.FetchLongword(cpu, addr); err != nil || got != wantL {
			t.Errorf("FetchLongword(%#x) = %#x, %v; want %#x", addr, got, err, wantL)
		}
	}
}

// TestFetchWindowHitsWithoutTranslating: the first fetch in a page fills
// the window through the slow path; further fetches in that page are
// window hits that don't translate at all (the STC sees no new tries).
func TestFetchWindowHitsWithoutTranslating(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)
	fillPattern(t, mem, 4)

	addr := uint32(pageSize) + 0x10

	if _, err := mem.FetchByte(cpu, addr); err != nil {
		t.Fatalf("FetchByte: %v", err)
	}

	hits, fills := mem.FetchStats()
	if fills != 1 || hits != 0 {
		t.Fatalf("FetchStats() after the first fetch = hits=%d fills=%d, want 0, 1", hits, fills)
	}

	stcTries, _ := mem.STCStats()

	if _, err := mem.FetchByte(cpu, addr+1); err != nil {
		t.Fatalf("FetchByte: %v", err)
	}

	if _, err := mem.FetchWord(cpu, addr+2); err != nil {
		t.Fatalf("FetchWord: %v", err)
	}

	if _, err := mem.FetchLongword(cpu, addr+4); err != nil {
		t.Fatalf("FetchLongword: %v", err)
	}

	// fillPattern's byte for P0 page 1, offset 0x18 (truncated to 8 bits).
	want := byte((ptBase + pageSize + 0x18 + 3) & 0xFF)
	if b, ok := mem.TryFetchByte(addr + 8); !ok || b != want {
		t.Errorf("TryFetchByte = %#x, %v; want %#x, true", b, ok, want)
	}

	hits, fills = mem.FetchStats()
	if hits != 4 || fills != 1 {
		t.Errorf("FetchStats() = hits=%d fills=%d, want 4, 1", hits, fills)
	}

	if after, _ := mem.STCStats(); after != stcTries {
		t.Errorf("STC tries went from %d to %d on window hits, want unchanged", stcTries, after)
	}
}

// TestTryFetchByteMissesOnEmptyWindow: TryFetchByte never reads through
// an empty window, or outside the window's page.
func TestTryFetchByteMissesOnEmptyWindow(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	if _, ok := mem.TryFetchByte(0); ok {
		t.Error("TryFetchByte(0) hit an empty window")
	}

	if _, err := mem.FetchByte(cpu, pageSize); err != nil {
		t.Fatalf("FetchByte: %v", err)
	}

	for _, addr := range []uint32{pageSize - 1, 2 * pageSize, 0xFFFFFFFF} {
		if _, ok := mem.TryFetchByte(addr); ok {
			t.Errorf("TryFetchByte(%#x) hit, outside the window's page", addr)
		}
	}
}

// TestFetchWindowSurvivesDataAccess: a data access to another page (which
// takes over the one-slot STC) leaves the window alone, so the next
// instruction fetch still hits. This is the point of the window: before
// it, a data operand in another page evicted the code page from the STC.
func TestFetchWindowSurvivesDataAccess(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	code := uint32(pageSize) + 0x10
	data := uint32(3*pageSize) + 0x20

	if _, err := mem.FetchByte(cpu, code); err != nil {
		t.Fatalf("FetchByte: %v", err)
	}

	if _, err := mem.LoadLongword(cpu, data); err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}

	if err := mem.StoreLongword(cpu, data, 1); err != nil {
		t.Fatalf("StoreLongword: %v", err)
	}

	if _, err := mem.FetchByte(cpu, code+1); err != nil {
		t.Fatalf("FetchByte: %v", err)
	}

	if hits, fills := mem.FetchStats(); hits != 1 || fills != 1 {
		t.Errorf("FetchStats() = hits=%d fills=%d, want 1, 1 (no refill)", hits, fills)
	}
}

// TestFetchWindowSeesStores: the window caches where the page is, not its
// contents, so a store into the code page (a program modifying its own
// instructions, or the console depositing into them) is fetched at once.
func TestFetchWindowSeesStores(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	addr := uint32(pageSize) + 0x10

	if _, err := mem.FetchByte(cpu, addr); err != nil {
		t.Fatalf("FetchByte: %v", err)
	}

	if err := mem.StoreByte(cpu, addr+1, 0x5A); err != nil {
		t.Fatalf("StoreByte: %v", err)
	}

	if got, err := mem.FetchByte(cpu, addr+1); err != nil || got != 0x5A {
		t.Errorf("FetchByte after StoreByte = %#x, %v; want 0x5a", got, err)
	}
}

// TestFetchWindowEmptiedByInvalidation: each of the TB invalidations
// (TBIS, TBIA, and the mode-change InvalidateProtection) empties the
// window, so the next fetch translates afresh. After the page is remapped
// and TBIS'd, the fetch reads the new physical page.
func TestFetchWindowEmptiedByInvalidation(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)
	fillPattern(t, mem, 4)

	addr := uint32(pageSize) + 0x10

	invalidations := []struct {
		name string
		do   func()
	}{
		{"InvalidatePage", func() { mem.InvalidatePage(addr) }},
		{"InvalidateTB", mem.InvalidateTB},
		{"InvalidateProtection", mem.InvalidateProtection},
	}

	for _, inv := range invalidations {
		if _, err := mem.FetchByte(cpu, addr); err != nil {
			t.Fatalf("%s: FetchByte: %v", inv.name, err)
		}

		_, before := mem.FetchStats()

		inv.do()

		if _, ok := mem.TryFetchByte(addr); ok {
			t.Errorf("%s: the window still hits", inv.name)
		}

		if _, err := mem.FetchByte(cpu, addr); err != nil {
			t.Fatalf("%s: FetchByte: %v", inv.name, err)
		}

		if _, after := mem.FetchStats(); after != before+1 {
			t.Errorf("%s: fills went from %d to %d, want one refill", inv.name, before, after)
		}
	}

	// Remap page 1 onto page 3's physical page, as an operating system
	// would (new PTE, then TBIS), and fetch through the same address.
	remapPage(t, mem, 1, (ptBase>>9)+3)
	mem.InvalidatePage(addr)

	want := byte((ptBase + 3*pageSize + 0x10 + 3*3) & 0xFF)
	if got, err := mem.FetchByte(cpu, addr); err != nil || got != want {
		t.Errorf("FetchByte after remap = %#x, %v; want %#x (page 3's byte)", got, err, want)
	}
}

// TestSyncFetchWindow: SyncFetchWindow empties the window when MAPEN or
// the CPU's access mode has changed since it was filled, and leaves it
// alone otherwise.
func TestSyncFetchWindow(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	addr := uint32(pageSize) + 0x10

	fill := func() {
		t.Helper()

		if _, err := mem.FetchByte(cpu, addr); err != nil {
			t.Fatalf("FetchByte: %v", err)
		}
	}

	fill()
	mem.SyncFetchWindow(cpu)

	if _, ok := mem.TryFetchByte(addr); !ok {
		t.Error("SyncFetchWindow emptied the window with nothing changed")
	}

	// A change of mode. (The CPU engine also calls InvalidateProtection
	// at every real mode change; SyncFetchWindow doesn't depend on it.)
	psl := cpu.PSL()
	psl.SetCurMod(vax.User)
	cpu.SetPSL(psl)
	mem.SyncFetchWindow(cpu)

	if _, ok := mem.TryFetchByte(addr); ok {
		t.Error("the window still hits after a change of mode")
	}

	// A change of MAPEN: mapping off, so addr is now a physical address,
	// in a different page of RAM from the one the window held.
	fill()
	cpu.SetPR(vax.MAPEN, 0)
	mem.SyncFetchWindow(cpu)

	if _, ok := mem.TryFetchByte(addr); ok {
		t.Error("the window still hits after MAPEN changed")
	}

	if err := mem.StoreByte(cpu, addr, 0x77); err != nil {
		t.Fatalf("StoreByte (physical): %v", err)
	}

	if got, err := mem.FetchByte(cpu, addr); err != nil || got != 0x77 {
		t.Errorf("FetchByte with MAPEN off = %#x, %v; want 0x77 (the physical byte)", got, err)
	}
}

// TestFetchFaultsLikeLoad: a fetch from an address that can't be read
// faults exactly as LoadByte does, even while the window holds another
// page, and leaves the window empty.
func TestFetchFaultsLikeLoad(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	if _, err := mem.FetchByte(cpu, pageSize); err != nil {
		t.Fatalf("FetchByte: %v", err)
	}

	bad := uint32(6*pageSize) + 0x10 // past P0LR

	_, wantErr := mem.LoadByte(cpu, bad)

	var want *TranslationFault
	if !errors.As(wantErr, &want) {
		t.Fatalf("LoadByte(%#x) error = %v, want a *TranslationFault", bad, wantErr)
	}

	checks := []struct {
		name string
		do   func() error
	}{
		{"FetchByte", func() error { _, err := mem.FetchByte(cpu, bad); return err }},
		{"FetchWord", func() error { _, err := mem.FetchWord(cpu, bad); return err }},
		{"FetchLongword", func() error { _, err := mem.FetchLongword(cpu, bad); return err }},
	}

	for _, c := range checks {
		var got *TranslationFault
		if err := c.do(); !errors.As(err, &got) || *got != *want {
			t.Errorf("%s(%#x) error = %v, want %v", c.name, bad, err, wantErr)
		}

		if _, ok := mem.TryFetchByte(pageSize); ok {
			t.Errorf("%s: the window still hits after a fault", c.name)
		}
	}
}

// TestFetchWindowSkipsROM: a page outside main RAM (here, console ROM,
// with mapping off) is fetched correctly through the slow path but never
// put in the window, whose fast path reads only RAM.
func TestFetchWindowSkipsROM(t *testing.T) {
	cpu := vax.New()
	mem := NewMemory(1 << 20)

	mem.ROM = make([]byte, 1024)
	mem.ROMBase = 1 << 21
	mem.ROMEnd = mem.ROMBase + uint32(len(mem.ROM)) - 1
	mem.ROM[5] = 0xC3

	if got, err := mem.FetchByte(cpu, mem.ROMBase+5); err != nil || got != 0xC3 {
		t.Fatalf("FetchByte(ROM) = %#x, %v; want 0xc3", got, err)
	}

	if _, fills := mem.FetchStats(); fills != 0 {
		t.Errorf("fills = %d after a ROM fetch, want 0", fills)
	}

	// A RAM page, with mapping off, does go in the window.
	if _, err := mem.FetchByte(cpu, 0x1234); err != nil {
		t.Fatalf("FetchByte(RAM): %v", err)
	}

	if _, ok := mem.TryFetchByte(0x1235); !ok {
		t.Error("a RAM page wasn't put in the window")
	}
}
