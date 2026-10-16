package vm

import (
	"testing"

	"github.com/tucats/govax/internal/paging"
)

// checkDB fails the test if the PFN database's invariants don't hold.
func checkDB(t *testing.T, m *Memory) {
	t.Helper()

	if err := m.Paging().Check(); err != nil {
		t.Fatal(err)
	}
}

// fillFrame writes b over every byte of frame pfn.
func fillFrame(t *testing.T, m *Memory, pfn uint32, b byte) {
	t.Helper()

	buf := make([]byte, pageSize)
	for i := range buf {
		buf[i] = b
	}

	if err := m.StorePhysical(pfn<<9, buf); err != nil {
		t.Fatal(err)
	}
}

// frameHolds reports whether every byte of frame pfn is b.
func frameHolds(t *testing.T, m *Memory, pfn uint32, b byte) bool {
	t.Helper()

	buf := make([]byte, pageSize)
	if err := m.LoadPhysical(pfn<<9, buf); err != nil {
		t.Fatal(err)
	}

	for _, got := range buf {
		if got != b {
			return false
		}
	}

	return true
}

// With paging off, a Memory has no PFN database, a claim's description
// changes nothing, and no frame counts as translated.
func TestFramesPagingOff(t *testing.T) {
	m := NewMemory(4 * pageSize)

	if m.Paging() != nil {
		t.Fatal("a new Memory has a PFN database")
	}

	claim := FrameClaim{Tied: true, Type: paging.ProcessPage, Owner: paging.Owner{Handle: 1, VA: 0x200}}

	pfn, ok := m.AllocateFrame(claim)
	if !ok || pfn != 1 {
		t.Fatalf("AllocateFrame = %d, %v; want frame 1", pfn, ok)
	}

	m.BeginInstruction()

	if m.TranslatedThisInstruction(pfn) {
		t.Error("a frame counts as translated with paging off")
	}

	m.SetPaging(true)
	m.SetPaging(false)

	if m.Paging() != nil || m.MappedPages() != 0 {
		t.Errorf("after paging on and off: database %v, %d frames claimed", m.Paging() != nil, m.MappedPages())
	}
}

// With paging on, frames come from the PFN database: frame 0 never, the
// free list's head first, cleared when handed out unless the caller fills
// them, and a freed frame keeps its contents at the free list's head.
func TestFramesPagingOn(t *testing.T) {
	m := NewMemory(6 * pageSize)
	m.SetPaging(true)

	db := m.Paging()
	if db == nil {
		t.Fatal("SetPaging(true) left no PFN database")
	}

	if e, _ := db.Entry(0); e.State != paging.Active || e.Type != paging.SystemPage {
		t.Fatalf("frame 0's record %+v, want a system page", e)
	}

	if m.MappedPages() != 0 {
		t.Fatalf("MappedPages = %d on a fresh database, want 0 (frame 0 isn't counted)", m.MappedPages())
	}

	// VMINIT's reservations, the second of each harmless.
	m.ReservePage(0)
	m.ReservePage(1)
	m.ReservePage(1)
	checkDB(t, m)

	// A frame holding old contents is cleared when it's handed out.
	fillFrame(t, m, 2, 0x5A)

	pfn, ok := m.AllocatePage()
	if !ok || pfn != 2 {
		t.Fatalf("AllocatePage = %d, %v; want frame 2", pfn, ok)
	}

	if !frameHolds(t, m, pfn, 0) {
		t.Error("an allocated frame wasn't cleared")
	}

	// A frame the caller fills isn't.
	fillFrame(t, m, 3, 0x77)

	claim := FrameClaim{
		Tied:    true,
		Type:    paging.ProcessPage,
		Owner:   paging.Owner{Handle: 0x21, VA: 0x1200},
		Backing: paging.Backing{Kind: paging.SectionFile, Index: 4, Block: 9},
		Fill:    true,
	}

	pfn, ok = m.AllocateFrame(claim)
	if !ok || pfn != 3 {
		t.Fatalf("AllocateFrame = %d, %v; want frame 3", pfn, ok)
	}

	if !frameHolds(t, m, pfn, 0x77) {
		t.Error("a frame claimed with Fill was cleared")
	}

	e, _ := db.Entry(paging.PFN(pfn))
	if e.State != paging.Active || !e.Tied || e.Type != paging.ProcessPage || e.Owner != claim.Owner || e.Backing != claim.Backing {
		t.Errorf("tied frame's record %+v", e)
	}

	if m.MappedPages() != 3 {
		t.Errorf("MappedPages = %d, want 3 (frames 1 to 3)", m.MappedPages())
	}

	checkDB(t, m)

	// A freed frame keeps its contents and is the next one handed out.
	fillFrame(t, m, 2, 0x33)
	m.FreePage(2)

	if !frameHolds(t, m, 2, 0x33) {
		t.Error("a freed frame was cleared with paging on")
	}

	if e, _ := db.Entry(2); e.State != paging.Free {
		t.Errorf("freed frame's record %+v", e)
	}

	checkDB(t, m)

	if pfn, _ = m.AllocatePage(); pfn != 2 || !frameHolds(t, m, 2, 0) {
		t.Errorf("AllocatePage after the free = %d; want frame 2 again, cleared", pfn)
	}

	m.FreePage(0)    // ignored
	m.FreePage(9999) // ignored

	// Frames 4 and 5 are left, then none.
	for _, want := range []uint32{4, 5} {
		if pfn, ok = m.AllocatePage(); !ok || pfn != want {
			t.Fatalf("AllocatePage = %d, %v; want frame %d", pfn, ok, want)
		}
	}

	if _, ok = m.AllocatePage(); ok {
		t.Error("AllocatePage with every frame claimed succeeded")
	}

	checkDB(t, m)
}

// Demand-zero pages come out cleared with paging on, though freed frames
// keep their contents.
func TestDemandZeroPaging(t *testing.T) {
	m := NewMemory(4 * pageSize)
	m.SetPaging(true)
	m.SetVMValid(true)

	pfn, _ := m.AllocatePage()
	fillFrame(t, m, pfn, 0xEE)
	m.FreePage(pfn)

	var pte PTE

	pte.SetProtection(ProtUW)

	got, ok := m.DemandZero(pte)
	if !ok || got.PFN() != pfn || !frameHolds(t, m, pfn, 0) {
		t.Errorf("DemandZero = %08X, %v; want frame %d, cleared", uint32(got), ok, pfn)
	}
}

// The translated-frames set: a frame counts from its translation (a page
// table walk, a TB hit, TranslateIn) to the next instruction boundary, and
// the STC's and the fetch window's frames count whenever they are there.
func TestTranslatedThisInstruction(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)
	mem.SetPaging(true)

	frame := func(page uint32) uint32 { return ptBase>>9 + page }

	translate := func(page uint32) {
		t.Helper()

		if _, err := mem.Translate(cpu, page*pageSize, AccessRead); err != nil {
			t.Fatalf("Translate page %d: %v", page, err)
		}
	}

	expect := func(page uint32, want bool) {
		t.Helper()

		if got := mem.TranslatedThisInstruction(frame(page)); got != want {
			t.Errorf("page %d's frame translated this instruction = %v, want %v", page, got, want)
		}
	}

	expect(1, false)

	translate(1) // a page table walk
	expect(1, true)
	expect(2, false)

	mem.BeginInstruction()
	expect(1, true) // still the STC's page

	translate(2)
	expect(1, false)
	expect(2, true)

	mem.BeginInstruction()
	translate(1) // a TB hit
	expect(1, true)
	expect(2, false)

	// The fetch window's page counts after the boundary that follows.
	if _, err := mem.FetchByte(cpu, 3*pageSize); err != nil {
		t.Fatalf("FetchByte: %v", err)
	}

	mem.BeginInstruction()
	translate(1)
	expect(3, true)

	// Another process's page, through TranslateIn.
	mem.BeginInstruction()
	expect(0, false)

	if _, err := mem.TranslateIn(cpu, CurrentAddressSpace(cpu), 0x10, AccessRead); err != nil {
		t.Fatalf("TranslateIn: %v", err)
	}

	expect(0, true)

	// Frames beyond memory never count.
	if mem.TranslatedThisInstruction(1 << 20) {
		t.Error("a frame beyond memory counts as translated")
	}

	// When the generation wraps, the set starts over.
	mem.touchGen = ^uint32(0)
	mem.touched[frame(2)] = mem.touchGen
	mem.BeginInstruction()

	if mem.touchGen != 1 || mem.touched[frame(2)] != 0 {
		t.Errorf("after the wrap: generation %d, page 2's frame %d", mem.touchGen, mem.touched[frame(2)])
	}
}

// TestAtNextInstruction: the routine runs once, at the next boundary,
// after the new generation began (a frame translated before it no longer
// counts); a second request replaces the first; with paging off nothing
// runs.
func TestAtNextInstruction(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	ran := 0

	mem.AtNextInstruction(func() { ran++ })
	mem.BeginInstruction()

	if ran != 0 {
		t.Fatal("a routine ran with paging off")
	}

	mem.SetPaging(true)

	if _, err := mem.Translate(cpu, 1*pageSize, AccessRead); err != nil {
		t.Fatal(err)
	}

	mem.InvalidateTB() // so that the STC doesn't keep page 1's frame counting

	var counted bool

	mem.AtNextInstruction(func() { ran += 10 })
	mem.AtNextInstruction(func() {
		ran++
		counted = mem.TranslatedThisInstruction(ptBase>>9 + 1)
	})

	if ran != 0 {
		t.Fatal("the routine ran before the boundary")
	}

	mem.BeginInstruction()
	mem.BeginInstruction()

	if ran != 1 || counted {
		t.Errorf("ran %d (want 1, once and not the replaced routine); page 1's frame counted %v", ran, counted)
	}
}
