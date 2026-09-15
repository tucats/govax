package vm

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
)

// AccessType distinguishes a read access from a write access during address
// translation and protection checking (vm.c's VM_READ/VM_WRITE).
type AccessType int

const (
	AccessRead AccessType = iota
	AccessWrite
)

// pageSize is the VAX virtual memory page size in bytes.
const pageSize = 512

// FaultKind identifies why a virtual address failed to translate.
type FaultKind int

const (
	// AccessViolation: the page number is outside the region's base/length
	// registers -- vm.c's own EXC_ACCVIO subcode 0x0001.
	AccessViolation FaultKind = iota
	// ProtectionViolation: the page is in range, but its protection code
	// denies this access at the current privilege mode -- vm.c's own
	// EXC_ACCVIO subcode 0x0002. Split out from AccessViolation in Phase
	// 12 (previously collapsed into one kind, always reporting the
	// length/base subcode regardless of which check actually failed) --
	// see docs/DEVIATIONS.md.
	ProtectionViolation
	// TranslationNotValid: the page is in range and the access is
	// permitted, but the page table entry's valid bit is clear.
	TranslationNotValid
)

// TranslationFault reports a failed virtual-to-physical address translation.
// Turning this into the CPU-level exception/fault machinery (EXC_ACCVIO,
// EXC_TNV, and the rest of set_fault's bookkeeping) belongs to Phase 03,
// which owns fault/interrupt handling; this only reports what went wrong at
// the given virtual address.
type TranslationFault struct {
	Kind FaultKind
	Addr uint32
}

func (f *TranslationFault) Error() string {
	switch f.Kind {
	case TranslationNotValid:
		return fmt.Sprintf("vm: translation not valid at %#08x", f.Addr)
	case ProtectionViolation:
		return fmt.Sprintf("vm: protection violation at %#08x", f.Addr)
	default:
		return fmt.Sprintf("vm: access violation at %#08x", f.Addr)
	}
}

func accessViolation(addr uint32) error {
	return &TranslationFault{Kind: AccessViolation, Addr: addr}
}

func protectionViolation(addr uint32) error {
	return &TranslationFault{Kind: ProtectionViolation, Addr: addr}
}

func translationNotValid(addr uint32) error {
	return &TranslationFault{Kind: TranslationNotValid, Addr: addr}
}

// Translate converts a virtual address to a physical address, consulting
// cpu's MAPEN, region base/length registers (P0BR/P0LR, P1BR/P1LR, SBR/SLR)
// and current access mode (PSL cur_mod). If virtual memory is disabled
// (MAPEN == 0) addr is returned unchanged, matching vm.c's vm().
//
// This is a direct port of vm.c's vm() routine, minus two things that are
// deliberately out of scope here:
//
//   - The translation-buffer and single-page "sequential translation cache"
//     (struct TB / the STC cached_* globals) are pure 1999-era performance
//     hacks with no effect on the result, same rationale as Phase 01 not
//     porting struct PSL_W — see docs/PHASE-01.md.
//   - DYNVM dynamic page-in-on-demand (validate_page(), gated on
//     vax.console.vminit_valid/page_map) is console/microkernel state that
//     belongs to Phase 08's VMINIT command; until that exists, an invalid
//     page table entry always faults TNV here, which is also exactly what
//     validate_page() itself does whenever VMVALID is false (the normal case
//     until a real OS or VMINIT has run).
func (m *Memory) Translate(cpu *vax.CPU, addr uint32, access AccessType) (uint32, error) {
	if cpu.PR(vax.MAPEN) == 0 {
		return addr, nil
	}

	region := (addr >> 30) & 0x3
	page := (addr & 0x3FFFFFFF) >> 9
	byteOffset := addr & (pageSize - 1)

	var (
		pteVirtAddr  uint32
		pteRecursive bool
	)

	switch region {
	case 0: // P0: process program region
		if page > cpu.PR(vax.P0LR) {
			return 0, accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.P0BR) + page*4
		pteRecursive = true

	case 1: // P1: process control region. P1BR/P1LR list invalid pages, so
		// the length comparison is inverted relative to P0.
		if page <= cpu.PR(vax.P1LR) {
			return 0, accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.P1BR) + page*4
		pteRecursive = true

	case 2: // S0: system region. SBR is already a physical address.
		if page > cpu.PR(vax.SLR) {
			return 0, accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.SBR) + page*4
		pteRecursive = false

	default: // S1: unsupported by this implementation, same as the C source.
		return 0, accessViolation(addr)
	}

	pteAddr := pteVirtAddr
	if pteRecursive {
		var err error

		pteAddr, err = m.Translate(cpu, pteVirtAddr, AccessRead)
		if err != nil {
			return 0, err
		}
	}

	raw, err := m.readPhysLongword(pteAddr)
	if err != nil {
		return 0, err
	}
	
	pte := PTE(raw)

	if !pte.Protection().allows(cpu.PSL().CurMod(), access) {
		return 0, protectionViolation(addr)
	}

	if !pte.Valid() {
		return 0, translationNotValid(addr)
	}

	if access == AccessWrite && !pte.Modified() {
		pte.SetModified(true)
		if err := m.writePhysLongword(pteAddr, uint32(pte)); err != nil {
			return 0, err
		}
	}

	return pte.PFN()<<9 + byteOffset, nil
}
