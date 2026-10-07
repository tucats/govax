package vm

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// Process B's page table, for the address-space tests: S0 page 2 maps it,
// at physical spaceBPT, and its P0 pages map onto spaceBData's pages.
// Process A is newTranslateFixture's own (P0BR = S0 page 1, data at
// ptBase), and is the one the CPU's registers describe.
const (
	spaceBPT   = 0x5000 // physical: process B's P0 page table
	spaceBData = 0x6000 // physical: process B's P0 data pages
)

// newSpaceFixture extends newTranslateFixture with a second process, B,
// whose four P0 pages map to frames of their own. It returns A's and B's
// address spaces.
func newSpaceFixture(t *testing.T) (*vax.CPU, *Memory, AddressSpace, AddressSpace) {
	t.Helper()

	cpu, mem := newTranslateFixture(t, 4)

	var s0pte PTE

	s0pte.SetValid(true)
	s0pte.SetProtection(ProtKW)
	s0pte.SetPFN(spaceBPT >> 9)

	if err := mem.writePhysLongword(sysPTBase+2*4, uint32(s0pte)); err != nil {
		t.Fatalf("seed S0 PTE: %v", err)
	}

	for i := range uint32(4) {
		var pte PTE

		pte.SetValid(true)
		pte.SetProtection(ProtUW)
		pte.SetPFN(spaceBData>>9 + i)

		if err := mem.writePhysLongword(spaceBPT+i*4, uint32(pte)); err != nil {
			t.Fatalf("seed B's PTE %d: %v", i, err)
		}
	}

	a := CurrentAddressSpace(cpu)
	b := AddressSpace{P0BR: sysBase + 2*pageSize, P0LR: 4}

	return cpu, mem, a, b
}

// TestAddressSpacesSeparateP0: one P0 address names different frames in
// two address spaces, and a store through one leaves the other alone.
func TestAddressSpacesSeparateP0(t *testing.T) {
	cpu, mem, a, b := newSpaceFixture(t)

	const addr = 2*pageSize + 0x10

	pa, err := mem.TranslateIn(cpu, a, addr, AccessRead)
	if err != nil {
		t.Fatalf("TranslateIn A: %v", err)
	}

	pb, err := mem.TranslateIn(cpu, b, addr, AccessRead)
	if err != nil {
		t.Fatalf("TranslateIn B: %v", err)
	}

	if pa != ptBase+addr || pb != spaceBData+addr {
		t.Fatalf("A -> %#x, B -> %#x; want %#x and %#x", pa, pb, ptBase+addr, spaceBData+addr)
	}

	if err := mem.StoreLongwordIn(cpu, a, addr, 0xAAAAAAAA); err != nil {
		t.Fatalf("StoreLongwordIn A: %v", err)
	}

	if err := mem.StoreLongwordIn(cpu, b, addr, 0xBBBBBBBB); err != nil {
		t.Fatalf("StoreLongwordIn B: %v", err)
	}

	if v, _ := mem.LoadLongwordIn(cpu, a, addr); v != 0xAAAAAAAA {
		t.Errorf("A's longword = %#x", v)
	}

	if v, _ := mem.LoadLongwordIn(cpu, b, addr); v != 0xBBBBBBBB {
		t.Errorf("B's longword = %#x", v)
	}

	// The CPU's registers describe A: an ordinary load sees A's value.
	if v, _ := mem.LoadLongword(cpu, addr); v != 0xAAAAAAAA {
		t.Errorf("LoadLongword through the registers = %#x, want A's", v)
	}
}

// TestAddressSpacesShareS0: an S0 address translates the same in every
// address space.
func TestAddressSpacesShareS0(t *testing.T) {
	cpu, mem, a, b := newSpaceFixture(t)

	const addr = sysBase + pageSize + 8

	pa, errA := mem.TranslateIn(cpu, a, addr, AccessRead)
	pb, errB := mem.TranslateIn(cpu, b, addr, AccessRead)

	if errA != nil || errB != nil || pa != pb || pa != p0PTPhys+8 {
		t.Errorf("S0 -> %#x (%v) and %#x (%v), want %#x for both", pa, errA, pb, errB, p0PTPhys+8)
	}
}

// TestAddressSpaceLeavesTBAlone: TranslateIn neither consults nor fills
// the TB or the STC, which hold the current process's translations.
func TestAddressSpaceLeavesTBAlone(t *testing.T) {
	cpu, mem, _, b := newSpaceFixture(t)

	// Cache A's page 1 in the TB.
	if _, err := mem.Translate(cpu, pageSize, AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	before := mem.TBSnapshot()
	tries, hits, _, _ := mem.TBStats()

	if _, err := mem.TranslateIn(cpu, b, pageSize, AccessWrite); err != nil {
		t.Fatalf("TranslateIn: %v", err)
	}

	after := mem.TBSnapshot()
	tries2, hits2, _, _ := mem.TBStats()

	if len(before) != len(after) || tries != tries2 || hits != hits2 {
		t.Errorf("TB changed: %d -> %d entries, tries %d -> %d, hits %d -> %d",
			len(before), len(after), tries, tries2, hits, hits2)
	}

	// And A's page 1 still translates to A's frame.
	if p, err := mem.Translate(cpu, pageSize, AccessRead); err != nil || p != ptBase+pageSize {
		t.Errorf("A's page 1 -> %#x (%v), want %#x", p, err, ptBase+pageSize)
	}
}

// TestAddressSpaceCrossesPages: LoadIn and StoreIn split an access at a
// page boundary, each page through its own PTE.
func TestAddressSpaceCrossesPages(t *testing.T) {
	cpu, mem, _, b := newSpaceFixture(t)

	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	addr := uint32(2*pageSize - 3)

	if err := mem.StoreIn(cpu, b, addr, data); err != nil {
		t.Fatalf("StoreIn: %v", err)
	}

	// Physically, B's pages 1 and 2 are adjacent, so the bytes are too.
	got := make([]byte, len(data))
	if err := mem.LoadPhysical(spaceBData+addr, got); err != nil || !bytes.Equal(got, data) {
		t.Errorf("physical bytes %v (%v), want %v", got, err, data)
	}

	back := make([]byte, len(data))
	if err := mem.LoadIn(cpu, b, addr, back); err != nil || !bytes.Equal(back, data) {
		t.Errorf("LoadIn %v (%v), want %v", back, err, data)
	}
}

// TestAddressSpaceDemandZeroAndModify: an invalid page is given a frame
// on first touch, and a write sets the modify bit.
func TestAddressSpaceDemandZeroAndModify(t *testing.T) {
	cpu, mem, _, b := newSpaceFixture(t)
	mem.SetVMValid(true)

	var pte PTE

	pte.SetProtection(ProtUW) // valid bit clear: no frame yet

	if err := mem.writePhysLongword(spaceBPT+3*4, uint32(pte)); err != nil {
		t.Fatal(err)
	}

	if err := mem.StoreLongwordIn(cpu, b, 3*pageSize, 42); err != nil {
		t.Fatalf("StoreLongwordIn: %v", err)
	}

	raw, _ := mem.readPhysLongword(spaceBPT + 3*4)
	got := PTE(raw)

	if !got.Valid() || !got.Modified() || got.PFN() == 0 {
		t.Errorf("PTE after store = %#x: want valid, modified, with a frame", raw)
	}

	if v, _ := mem.LoadLongwordIn(cpu, b, 3*pageSize); v != 42 {
		t.Errorf("read back %d", v)
	}
}

// TestAddressSpaceChecksKernelProtectionAndLength: protection is checked
// as kernel mode, whatever mode the CPU is in, and the address space's
// own lengths bound it.
func TestAddressSpaceChecksKernelProtectionAndLength(t *testing.T) {
	cpu, mem, _, b := newSpaceFixture(t)

	// User mode on the CPU doesn't matter: B's page table (S0 page 2,
	// ProtKW) is reachable, as it is for kernel mode.
	psl := cpu.PSL()
	psl.SetCurMod(vax.User)
	cpu.SetPSL(psl)

	if _, err := mem.TranslateIn(cpu, b, 0, AccessWrite); err != nil {
		t.Errorf("kernel-mode access through user CPU: %v", err)
	}

	// A no-access page refuses even kernel mode.
	var pte PTE

	pte.SetValid(true)
	pte.SetProtection(ProtNA)
	pte.SetPFN(spaceBData >> 9)

	if err := mem.writePhysLongword(spaceBPT, uint32(pte)); err != nil {
		t.Fatal(err)
	}

	var tf *TranslationFault

	_, err := mem.TranslateIn(cpu, b, 0x10, AccessRead)
	if !errors.As(err, &tf) || tf.Kind != ProtectionViolation {
		t.Errorf("no-access page: %v, want a protection violation", err)
	}

	// Page 4 is past B's P0LR (4).
	_, err = mem.TranslateIn(cpu, b, 4*pageSize, AccessRead)
	assertAccessViolation(t, err, 4*pageSize)

	// P1 has 2**21 pages; a P1LR of 2**21 says none of them exist, so
	// even P1's top page is a length violation. (The CPU's own P1LR, 0,
	// would admit it: the address space's lengths are the ones used.)
	b.P1LR = 1 << 21

	const top = 0x7FFFFE00

	_, err = mem.TranslateIn(cpu, b, top, AccessRead)
	assertAccessViolation(t, err, top)
}
