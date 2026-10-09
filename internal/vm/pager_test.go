package vm

import (
	"errors"
	"testing"
)

// TestPTEForms checks the invalid PTE forms: a section PTE and a global
// PTE keep their index, protection, and owner, and are told apart from
// each other, from a demand-zero PTE, and from a valid PTE whose modify
// bit (the same bit as TYP0) is set.
func TestPTEForms(t *testing.T) {
	s := SectionPTE(0x123456&MaxPTEIndex, ProtUR, 3)
	if i, ok := s.SectionIndex(); !ok || i != 0x123456&MaxPTEIndex {
		t.Errorf("SectionIndex = %#x, %v", i, ok)
	}

	if _, ok := s.GlobalIndex(); ok {
		t.Error("a section PTE reads as a global one")
	}

	if s.Protection() != ProtUR || s.Owner() != 3 || s.Valid() {
		t.Errorf("section PTE %08X: protection, owner, or valid wrong", uint32(s))
	}

	g := GlobalPTE(42, ProtUW, 3)
	if i, ok := g.GlobalIndex(); !ok || i != 42 {
		t.Errorf("GlobalIndex = %d, %v", i, ok)
	}

	if _, ok := g.SectionIndex(); ok {
		t.Error("a global PTE reads as a section one")
	}

	var dz PTE

	dz.SetProtection(ProtUW)

	if _, ok := dz.SectionIndex(); ok {
		t.Error("a demand-zero PTE reads as a section PTE")
	}

	if _, ok := dz.GlobalIndex(); ok {
		t.Error("a demand-zero PTE reads as a global PTE")
	}

	v := ValidPTE(7, ProtUW, 3)
	v.SetModified(true)

	if _, ok := v.SectionIndex(); ok {
		t.Error("a valid, modified PTE reads as a section PTE")
	}
}

// fakePager records the faults it's given and fills each page with its
// fill byte, or refuses when refuse is set.
type fakePager struct {
	mem    *Memory
	fill   byte
	refuse bool
	faults []PageFault
}

func (p *fakePager) PageIn(f PageFault) (PTE, bool) {
	p.faults = append(p.faults, f)

	if p.refuse {
		return 0, false
	}

	pfn, ok := p.mem.AllocatePage()
	if !ok {
		return 0, false
	}

	buf := make([]byte, pageSize)
	for i := range buf {
		buf[i] = p.fill
	}

	if err := p.mem.StorePhysical(pfn*pageSize, buf); err != nil {
		return 0, false
	}

	return ValidPTE(pfn, f.PTE.Protection(), f.PTE.Owner()), true
}

// TestPagerPageIn checks that an installed pager gets the fault on an
// invalid page, with the PTE, its address, and the access, and that the
// PTE it returns is written back (with the modify bit for a write) and
// used for the access.
func TestPagerPageIn(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)
	mem.SetVMValid(true)

	p := &fakePager{mem: mem, fill: 0xA5}
	mem.SetPager(p)

	pteAddr := uint32(p0PTPhys + 2*4)
	if err := mem.writePhysLongword(pteAddr, uint32(SectionPTE(5, ProtUW, 3))); err != nil {
		t.Fatalf("seed PTE: %v", err)
	}

	vaddr := uint32(2*pageSize + 8)

	b, err := mem.LoadByte(cpu, vaddr)
	if err != nil {
		t.Fatalf("LoadByte: %v", err)
	}

	if b != 0xA5 {
		t.Errorf("byte read = %#x, want the pager's fill 0xA5", b)
	}

	if len(p.faults) != 1 {
		t.Fatalf("pager saw %d faults, want 1", len(p.faults))
	}

	f := p.faults[0]
	if f.Addr != vaddr || f.PTEAddr != pteAddr || f.Write {
		t.Errorf("fault = %+v", f)
	}

	if i, ok := f.PTE.SectionIndex(); !ok || i != 5 {
		t.Errorf("fault's PTE has section index %d, %v", i, ok)
	}

	if f.Space != CurrentAddressSpace(cpu) {
		t.Errorf("fault's space = %+v, want the current one", f.Space)
	}

	raw, _ := mem.readPhysLongword(pteAddr)
	if pte := PTE(raw); !pte.Valid() || pte.Modified() {
		t.Errorf("PTE after the read = %08X, want valid and unmodified", raw)
	}

	// A write to the page now valid doesn't fault again, and sets the
	// modify bit.
	if err := mem.StoreByte(cpu, vaddr, 1); err != nil {
		t.Fatalf("StoreByte: %v", err)
	}

	if len(p.faults) != 1 {
		t.Errorf("pager saw %d faults after the write, want still 1", len(p.faults))
	}

	raw, _ = mem.readPhysLongword(pteAddr)
	if !PTE(raw).Modified() {
		t.Errorf("PTE after the write = %08X, want modified", raw)
	}
}

// TestPagerRefuses checks that a fault the pager can't resolve is a
// translation-not-valid fault, the PTE left as it was.
func TestPagerRefuses(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)
	mem.SetVMValid(true)
	mem.SetPager(&fakePager{mem: mem, refuse: true})

	pteAddr := uint32(p0PTPhys + 1*4)
	pte := GlobalPTE(3, ProtUW, 3)

	if err := mem.writePhysLongword(pteAddr, uint32(pte)); err != nil {
		t.Fatalf("seed PTE: %v", err)
	}

	_, err := mem.Translate(cpu, pageSize, AccessRead)

	var tf *TranslationFault
	if !errors.As(err, &tf) || tf.Kind != TranslationNotValid {
		t.Fatalf("Translate error = %v, want translation not valid", err)
	}

	raw, _ := mem.readPhysLongword(pteAddr)
	if PTE(raw) != pte {
		t.Errorf("PTE = %08X, want it unchanged (%08X)", raw, uint32(pte))
	}
}

// TestPagerThroughSpace checks that a fault reached through TranslateIn
// (another process's page) gives the pager that address space.
func TestPagerThroughSpace(t *testing.T) {
	cpu, mem, _, b := newSpaceFixture(t)
	mem.SetVMValid(true)

	p := &fakePager{mem: mem, fill: 0x3C}
	mem.SetPager(p)

	pteAddr := uint32(spaceBPT + 1*4)
	if err := mem.writePhysLongword(pteAddr, uint32(SectionPTE(1, ProtUW, 3))); err != nil {
		t.Fatalf("seed PTE: %v", err)
	}

	buf := make([]byte, 1)
	if err := mem.LoadIn(cpu, b, pageSize, buf); err != nil {
		t.Fatalf("LoadIn: %v", err)
	}

	if buf[0] != 0x3C || len(p.faults) != 1 || p.faults[0].Space != b {
		t.Errorf("read %#x, faults %+v; want 0x3C and one fault in B's space", buf[0], p.faults)
	}
}

// TestDemandZero checks the helper a pager uses for an ordinary page.
func TestDemandZero(t *testing.T) {
	mem := NewMemory(1 << 16)

	var pte PTE

	pte.SetProtection(ProtUW)
	pte.SetOwner(3)

	if _, ok := mem.DemandZero(pte); ok {
		t.Error("DemandZero before VMINIT succeeded")
	}

	mem.SetVMValid(true)

	got, ok := mem.DemandZero(pte)
	if !ok || !got.Valid() || got.PFN() == 0 || got.Protection() != ProtUW || got.Owner() != 3 {
		t.Errorf("DemandZero = %08X, %v", uint32(got), ok)
	}
}
