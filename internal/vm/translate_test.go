package vm

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// Fixture layout for the P0/S0 translation tests below:
//
//   - sysPTBase (physical): the S0 system page table itself. SBR points
//     here directly — S0 PTEs are read without a recursive translation.
//   - System virtual page 1 (address sysBase+pageSize) is mapped by
//     sysPTBase's PTE index 1 onto p0PTPhys, where P0's own page table
//     lives — exercising the real C source behavior that a process page
//     table is itself found via a (here, one-level) system-space
//     translation.
//   - P0BR is set to that system virtual address, so P0 page N's PTE is
//     found at p0PTPhys + N*4.
//   - Each valid P0 PTE maps its page onto physical page ptBase/pageSize+N.
const (
	sysPTBase = 0x2000     // physical: S0's own page table
	p0PTPhys  = 0x3000     // physical: P0's page table entries
	ptBase    = 0x1000     // physical: P0's mapped data pages
	sysBase   = 0x80000000 // virtual base of S0 space (region bits == 2)
)

func newTranslateFixture(t *testing.T, npages int) (*vax.CPU, *Memory) {
	t.Helper()

	cpu := vax.New()
	mem := NewMemory(1 << 20) // 1MB of RAM

	cpu.SetPR(vax.MAPEN, 1)

	cpu.SetPR(vax.SBR, sysPTBase)
	cpu.SetPR(vax.SLR, uint32(npages-1))

	var s0pte PTE
	s0pte.SetValid(true)
	s0pte.SetProtection(ProtUW)
	s0pte.SetPFN(p0PTPhys >> 9)
	if err := mem.writePhysLongword(sysPTBase+1*4, uint32(s0pte)); err != nil {
		t.Fatalf("seed S0 PTE: %v", err)
	}

	cpu.SetPR(vax.P0BR, sysBase+pageSize) // system virtual page 1
	cpu.SetPR(vax.P0LR, uint32(npages-1))

	for i := 0; i < npages; i++ {
		var pte PTE
		pte.SetValid(true)
		pte.SetProtection(ProtUW) // wide open, so tests can focus on one thing
		pte.SetPFN((ptBase >> 9) + uint32(i))
		if err := mem.writePhysLongword(p0PTPhys+uint32(i)*4, uint32(pte)); err != nil {
			t.Fatalf("seed P0 PTE %d: %v", i, err)
		}
	}

	return cpu, mem
}

func TestTranslateMAPENDisabledIsIdentity(t *testing.T) {
	cpu := vax.New() // MAPEN == 0
	mem := NewMemory(1 << 16)

	got, err := mem.Translate(cpu, 0xDEADBE00, AccessRead)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if got != 0xDEADBE00 {
		t.Errorf("Translate() = %#08x, want identity 0xDEADBE00", got)
	}
}

func TestTranslateP0RoundTrip(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	// P0 page 2, virtual address region 0, page 2, byte 0x10.
	vaddr := uint32(2*pageSize) + 0x10
	got, err := mem.Translate(cpu, vaddr, AccessRead)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	wantPhys := uint32(ptBase+2*pageSize) + 0x10
	if got != wantPhys {
		t.Errorf("Translate(%#08x) = %#08x, want %#08x", vaddr, got, wantPhys)
	}
}

func TestTranslateP0LengthViolation(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	// Page 4 is beyond P0LR == 3.
	vaddr := uint32(4 * pageSize)
	_, err := mem.Translate(cpu, vaddr, AccessRead)
	assertAccessViolation(t, err, vaddr)
}

func TestTranslateP1LengthViolation(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	// P1 space is region 1 (top bits 01); P1LR semantics are inverted: a
	// page is valid only if page > P1LR. With P1LR left at its zero value,
	// page 0 (<=0) must fault.
	vaddr := uint32(1) << 30 // region 1, page 0
	_, err := mem.Translate(cpu, vaddr, AccessRead)
	assertAccessViolation(t, err, vaddr)
}

func TestTranslateS1AlwaysFaults(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(3) << 30 // region 3 (S1), unsupported
	_, err := mem.Translate(cpu, vaddr, AccessRead)
	assertAccessViolation(t, err, vaddr)
}

func TestTranslateS0LengthViolation(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := (uint32(2) << 30) | (4 * pageSize) // S0 page 4, beyond SLR == 3
	_, err := mem.Translate(cpu, vaddr, AccessRead)
	assertAccessViolation(t, err, vaddr)
}

func TestTranslateProtectionViolation(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	// Narrow P0 page 1's protection to kernel-only, then access it from
	// user mode.
	var pte PTE
	pte.SetValid(true)
	pte.SetProtection(ProtKW)
	pte.SetPFN(ptBase >> 9)
	if err := mem.writePhysLongword(p0PTPhys+1*4, uint32(pte)); err != nil {
		t.Fatalf("seed PTE: %v", err)
	}

	psl := cpu.PSL()
	psl.SetCurMod(vax.User)
	cpu.SetPSL(psl)

	vaddr := uint32(1 * pageSize)
	_, err := mem.Translate(cpu, vaddr, AccessRead)
	assertAccessViolation(t, err, vaddr)
}

func TestTranslateInvalidPageIsTNV(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	var pte PTE
	pte.SetValid(false)
	pte.SetProtection(ProtUW)
	if err := mem.writePhysLongword(p0PTPhys+1*4, uint32(pte)); err != nil {
		t.Fatalf("seed PTE: %v", err)
	}

	vaddr := uint32(1 * pageSize)
	_, err := mem.Translate(cpu, vaddr, AccessRead)

	var tf *TranslationFault
	if !errors.As(err, &tf) {
		t.Fatalf("Translate error = %v (%T), want *TranslationFault", err, err)
	}
	if tf.Kind != TranslationNotValid {
		t.Errorf("Kind = %v, want TranslationNotValid", tf.Kind)
	}
	if tf.Addr != vaddr {
		t.Errorf("Addr = %#08x, want %#08x", tf.Addr, vaddr)
	}
}

func TestTranslateSetsModifyBitOnFirstWrite(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	pteAddr := uint32(p0PTPhys + 1*4)
	raw, err := mem.readPhysLongword(pteAddr)
	if err != nil {
		t.Fatalf("read PTE: %v", err)
	}
	if PTE(raw).Modified() {
		t.Fatal("fixture PTE already has M set, test needs it clear")
	}

	vaddr := uint32(1 * pageSize)
	if _, err := mem.Translate(cpu, vaddr, AccessWrite); err != nil {
		t.Fatalf("Translate (write): %v", err)
	}

	raw, err = mem.readPhysLongword(pteAddr)
	if err != nil {
		t.Fatalf("read PTE after write: %v", err)
	}
	if !PTE(raw).Modified() {
		t.Error("M bit not set after first write translation")
	}

	// A read should never set the M bit.
	pte := PTE(raw)
	pte.SetModified(false)
	if err := mem.writePhysLongword(pteAddr, uint32(pte)); err != nil {
		t.Fatalf("reset M bit: %v", err)
	}
	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (read): %v", err)
	}
	raw, _ = mem.readPhysLongword(pteAddr)
	if PTE(raw).Modified() {
		t.Error("M bit set by a read translation")
	}
}

func assertAccessViolation(t *testing.T, err error, wantAddr uint32) {
	t.Helper()
	var tf *TranslationFault
	if !errors.As(err, &tf) {
		t.Fatalf("error = %v (%T), want *TranslationFault", err, err)
	}
	if tf.Kind != AccessViolation {
		t.Errorf("Kind = %v, want AccessViolation", tf.Kind)
	}
	if tf.Addr != wantAddr {
		t.Errorf("Addr = %#08x, want %#08x", tf.Addr, wantAddr)
	}
}
