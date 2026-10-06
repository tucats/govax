package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// TestDecodeAcrossPageIntoInvalidPage decodes, with memory mapping on, an
// instruction that starts at the end of one page and runs into the next.
// The decoder reads the instruction stream through the memory's
// instruction-fetch window (internal/vm/fetch.go), which holds one page,
// so the part in the second page must take the slow path: a fault while
// that page is inaccessible (an all-zero PTE, whose protection code
// grants no access, so an access violation at page 1's first byte), and
// the right bytes once it is valid and the TB entry for it is invalidated
// (TBIS), as an operating system would do.
func TestDecodeAcrossPageIntoInvalidPage(t *testing.T) {
	cpu, mem := fixture()

	const (
		sbr     = 0x3000     // physical address of the system page table
		page0PA = 0x2000     // system virtual page 0 -> physical 0x2000
		page1PA = 0x4000     // page 1, once it's made valid
		startVA = 0x800001FC // four bytes before the end of page 0
	)

	pte := func(pa uint32) uint32 {
		var p vm.PTE

		p.SetValid(true)
		p.SetProtection(vm.ProtKW)
		p.SetPFN(pa >> 9)

		return uint32(p)
	}

	putLongword(t, cpu, mem, sbr, pte(page0PA)) // page 0: valid
	putLongword(t, cpu, mem, sbr+4, 0)          // page 1: invalid

	// MOVL I^#0x44332211, R0: opcode D0, specifier 8F (immediate), the
	// four bytes of the longword, then specifier 50 (R0). Its first four
	// bytes are the last four of page 0; the rest are in page 1.
	putBytes(t, cpu, mem, page0PA+0x1FC, 0xD0, 0x8F, 0x11, 0x22)
	putBytes(t, cpu, mem, page1PA, 0x33, 0x44, 0x50)

	cpu.SetPR(vax.SBR, sbr)
	cpu.SetPR(vax.SLR, 2)
	cpu.SetPR(vax.MAPEN, 1)
	cpu.SetGPR(vax.PC, startVA)

	_, err := decodeInstructionValue(cpu, mem, instructionTable)

	var tf *vm.TranslationFault
	if !errors.As(err, &tf) || tf.Addr != startVA+4 {
		t.Fatalf("decode into an inaccessible page: error = %v, want a translation fault at %#x", err, startVA+4)
	}

	// Make page 1 valid and accessible, and invalidate its translation as
	// the operating system would (MTPR TBIS).
	cpu.SetPR(vax.MAPEN, 0)
	putLongword(t, cpu, mem, sbr+4, pte(page1PA))
	cpu.SetPR(vax.MAPEN, 1)
	mem.InvalidatePage(startVA + 4)

	d, err := decodeInstructionValue(cpu, mem, instructionTable)
	if err != nil {
		t.Fatalf("decode after the page was made valid: %v", err)
	}

	if d.Operands[0].Value != 0x44332211 {
		t.Errorf("immediate = %#x, want 0x44332211", d.Operands[0].Value)
	}

	if d.Operands[1].Kind != OperandRegister || d.Operands[1].Reg != vax.R0 {
		t.Errorf("destination = %+v, want register R0", d.Operands[1])
	}

	if d.NextPC != startVA+7 {
		t.Errorf("NextPC = %#x, want %#x", d.NextPC, startVA+7)
	}
}
