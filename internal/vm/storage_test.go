package vm

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// identityCPU returns a CPU with virtual memory disabled, so Translate is
// the identity function and these tests can address physical memory
// directly by virtual address.
func identityCPU() *vax.CPU {
	return vax.New() // MAPEN == 0
}

func TestByteRoundTrip(t *testing.T) {
	cpu := identityCPU()
	mem := NewMemory(1 << 12)

	if err := mem.StoreByte(cpu, 0x100, 0xAB); err != nil {
		t.Fatalf("StoreByte: %v", err)
	}
	got, err := mem.LoadByte(cpu, 0x100)
	if err != nil {
		t.Fatalf("LoadByte: %v", err)
	}
	if got != 0xAB {
		t.Errorf("LoadByte() = %#02x, want 0xAB", got)
	}
}

func TestWordRoundTrip(t *testing.T) {
	cpu := identityCPU()
	mem := NewMemory(1 << 12)

	if err := mem.StoreWord(cpu, 0x100, 0xBEEF); err != nil {
		t.Fatalf("StoreWord: %v", err)
	}
	got, err := mem.LoadWord(cpu, 0x100)
	if err != nil {
		t.Fatalf("LoadWord: %v", err)
	}
	if got != 0xBEEF {
		t.Errorf("LoadWord() = %#04x, want 0xBEEF", got)
	}

	// VAX is little-endian: low byte at the low address.
	lo, _ := mem.LoadByte(cpu, 0x100)
	hi, _ := mem.LoadByte(cpu, 0x101)
	if lo != 0xEF || hi != 0xBE {
		t.Errorf("byte order = %02x %02x, want EF BE (little-endian)", lo, hi)
	}
}

func TestLongwordRoundTrip(t *testing.T) {
	cpu := identityCPU()
	mem := NewMemory(1 << 12)

	if err := mem.StoreLongword(cpu, 0x100, 0xDEADBEEF); err != nil {
		t.Fatalf("StoreLongword: %v", err)
	}
	got, err := mem.LoadLongword(cpu, 0x100)
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}
	if got != 0xDEADBEEF {
		t.Errorf("LoadLongword() = %#08x, want 0xDEADBEEF", got)
	}
}

func TestQuadwordRoundTrip(t *testing.T) {
	cpu := identityCPU()
	mem := NewMemory(1 << 12)

	const want = 0x0123456789ABCDEF
	if err := mem.StoreQuadword(cpu, 0x100, want); err != nil {
		t.Fatalf("StoreQuadword: %v", err)
	}
	got, err := mem.LoadQuadword(cpu, 0x100)
	if err != nil {
		t.Fatalf("LoadQuadword: %v", err)
	}
	if got != want {
		t.Errorf("LoadQuadword() = %#016x, want %#016x", got, want)
	}

	// Two little-endian longwords, low longword at the low address.
	lo, _ := mem.LoadLongword(cpu, 0x100)
	hi, _ := mem.LoadLongword(cpu, 0x104)
	if lo != 0x89ABCDEF || hi != 0x01234567 {
		t.Errorf("longword halves = %#08x %#08x, want 0x89abcdef 0x01234567", lo, hi)
	}
}

func TestLoadStoreThroughTranslation(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(1*pageSize) + 0x20
	if err := mem.StoreLongword(cpu, vaddr, 0x11223344); err != nil {
		t.Fatalf("StoreLongword: %v", err)
	}

	// Verify it actually landed at the translated physical address, not
	// just readable back through the same virtual address by coincidence.
	physAddr := uint32(ptBase+1*pageSize) + 0x20
	got, err := mem.readPhysLongword(physAddr)
	if err != nil {
		t.Fatalf("readPhysLongword: %v", err)
	}
	if got != 0x11223344 {
		t.Errorf("physical memory at %#08x = %#08x, want 0x11223344", physAddr, got)
	}

	viaVirt, err := mem.LoadLongword(cpu, vaddr)
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}
	if viaVirt != 0x11223344 {
		t.Errorf("LoadLongword() = %#08x, want 0x11223344", viaVirt)
	}
}

func TestLoadStoreSpansPageBoundary(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	// P0 pages 0 and 1 map to contiguous physical pages (ptBase, ptBase+
	// pageSize) in the fixture, but a real VAX doesn't guarantee that in
	// general -- make page 1 map somewhere physically disjoint so a value
	// genuinely straddling the boundary can only round-trip correctly if
	// each byte is independently translated.
	var pte PTE
	pte.SetValid(true)
	pte.SetProtection(ProtUW)
	const farPhysPage = 0x10000
	pte.SetPFN(farPhysPage >> 9)
	if err := mem.writePhysLongword(p0PTPhys+1*4, uint32(pte)); err != nil {
		t.Fatalf("seed PTE: %v", err)
	}

	vaddr := uint32(pageSize - 2) // last 2 bytes of page 0, spilling into page 1
	if err := mem.StoreLongword(cpu, vaddr, 0xCAFEBABE); err != nil {
		t.Fatalf("StoreLongword: %v", err)
	}

	got, err := mem.LoadLongword(cpu, vaddr)
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}
	if got != 0xCAFEBABE {
		t.Errorf("LoadLongword() across page boundary = %#08x, want 0xCAFEBABE", got)
	}

	// 0xCAFEBABE little-endian is bytes BE BA FE CA. The first two (BE, BA)
	// land at the end of page 0's physical page; the last two (FE, CA) at
	// the start of page 1's (disjoint) physical page.
	page0Bytes, err := mem.phys(uint32(ptBase+pageSize-2), 2)
	if err != nil {
		t.Fatalf("phys: %v", err)
	}
	page1Bytes, err := mem.phys(farPhysPage, 2)
	if err != nil {
		t.Fatalf("phys: %v", err)
	}
	if page0Bytes[0] != 0xBE || page0Bytes[1] != 0xBA || page1Bytes[0] != 0xFE || page1Bytes[1] != 0xCA {
		t.Errorf("split bytes = %02x %02x / %02x %02x, want be ba / fe ca",
			page0Bytes[0], page0Bytes[1], page1Bytes[0], page1Bytes[1])
	}
}

func TestLoadRegisterArchitectedPreservesUpperBytes(t *testing.T) {
	cpu := identityCPU()
	mem := NewMemory(1 << 12)

	cpu.SetGPR(vax.R2, 0xFFFFFFFF)
	if err := mem.StoreWord(cpu, 0x100, 0xBEEF); err != nil {
		t.Fatalf("StoreWord: %v", err)
	}

	if err := mem.LoadRegister(cpu, vax.R2, 0x100, 2); err != nil {
		t.Fatalf("LoadRegister: %v", err)
	}
	if got, want := cpu.GPR(vax.R2), uint32(0xFFFF0000)|0xBEEF; got != want {
		t.Errorf("R2 = %#08x, want %#08x (upper bytes preserved)", got, want)
	}
}

func TestLoadRegisterScratchZeroesFirst(t *testing.T) {
	cpu := identityCPU()
	mem := NewMemory(1 << 12)

	const scratch = vax.Reg(20) // > R15, a reusable temporary
	cpu.SetGPR(scratch, 0xFFFFFFFF)
	if err := mem.StoreByte(cpu, 0x100, 0x42); err != nil {
		t.Fatalf("StoreByte: %v", err)
	}

	if err := mem.LoadRegister(cpu, scratch, 0x100, 1); err != nil {
		t.Fatalf("LoadRegister: %v", err)
	}
	if got := cpu.GPR(scratch); got != 0x42 {
		t.Errorf("scratch register = %#08x, want 0x42 (zero-extended)", got)
	}
}

func TestLoadRegisterFullLongword(t *testing.T) {
	cpu := identityCPU()
	mem := NewMemory(1 << 12)

	if err := mem.StoreLongword(cpu, 0x100, 0x12345678); err != nil {
		t.Fatalf("StoreLongword: %v", err)
	}
	if err := mem.LoadRegister(cpu, vax.R3, 0x100, 4); err != nil {
		t.Fatalf("LoadRegister: %v", err)
	}
	if got := cpu.GPR(vax.R3); got != 0x12345678 {
		t.Errorf("R3 = %#08x, want 0x12345678", got)
	}
}

func TestPhysicalAddressOutOfRange(t *testing.T) {
	cpu := identityCPU()
	mem := NewMemory(16)

	_, err := mem.LoadByte(cpu, 100)
	var pe *PhysicalAddressError
	if !errors.As(err, &pe) {
		t.Fatalf("error = %v (%T), want *PhysicalAddressError", err, err)
	}
	if pe.Addr != 100 {
		t.Errorf("Addr = %d, want 100", pe.Addr)
	}
}
