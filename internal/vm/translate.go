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
	Mask byte
}

func (f *TranslationFault) Error() string {
	switch f.Kind {
	case TranslationNotValid:
		return fmt.Sprintf("SYSTEM-F-TNV, translation not valid, address %#08x", f.Addr)
	case ProtectionViolation:
		return fmt.Sprintf("SYSTEM-F-ACCVIO, access violation, reason mask=%02X, address %#08x", f.Mask, f.Addr)
	default:
		return fmt.Sprintf("SYSTEM-F-ACCVIO, access violation, address %#08x", f.Addr)
	}
}

func accessViolation(addr uint32) error {
	return &TranslationFault{Kind: AccessViolation, Addr: addr}
}

func protectionViolation(addr uint32, mask byte) error {
	return &TranslationFault{Kind: ProtectionViolation, Addr: addr, Mask: mask}
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

	if cpu.DebugEnabled(vax.DebugVM) || cpu.DebugEnabled(vax.DebugTB) {
		pa := pte.PFN()<<9 + byteOffset
		w := cpu.DebugWriter()

		if cpu.DebugEnabled(vax.DebugVM) {
			fmt.Fprintf(w, "DEBUG(VM): VA=%08X  R=%02d PTEA=%08X PTE=%08X P=%02X M=%02X PA=%08X\n",
				addr, region, pteAddr, uint32(pte), pte.Protection(), access, pa)
		}

		if cpu.DebugEnabled(vax.DebugTB) {
			// This port's Translate does an uncached page-table walk with
			// no separate TB-hit/miss state (see this function's own doc
			// comment and docs/PHASE-17.md sub-phase 3) -- TB traces the
			// same translation event VM does rather than a distinct
			// cache-hit/miss event that doesn't exist here.
			fmt.Fprintf(w, "DEBUG(TB): VA=%08X  R=%02d PTEA=%08X PTE=%08X P=%02X M=%02X PA=%08X\n",
				addr, region, pteAddr, uint32(pte), pte.Protection(), access, pa)
		}
	}

	modeMask := access

	if !pte.Protection().allows(cpu.PSL().CurMod(), access) {
		return 0, protectionViolation(addr, byte(modeMask))
	}

	if !pte.Valid() {
		if !m.validatePage(pteAddr, &pte) {
			return 0, translationNotValid(addr)
		}
	}

	if access == AccessWrite && !pte.Modified() {
		pte.SetModified(true)

		if err := m.writePhysLongword(pteAddr, uint32(pte)); err != nil {
			return 0, err
		}
	}

	m.translationCount++

	return pte.PFN()<<9 + byteOffset, nil
}

// validatePage is the Go equivalent of vm.c's validate_page: called when
// Translate finds an in-range, protection-permitted but invalid PTE, it
// claims a free physical page (AllocatePage) and writes the now-valid PTE
// back to pteAddr, matching the DYNVM demand-paging path every region
// (P0, P1, and — should a caller ever invalidate an S0 PTE — S0 too) now
// takes unconditionally, DYNVM having become this port's only supported
// mode per docs/PHASE-*.md rather than a build-time #ifdef.
//
// Reports false (leaving pte untouched, so the caller faults TNV exactly as
// it did before demand-paging existed) if no VMINIT has run yet or physical
// memory is exhausted, matching validate_page's own !VMVALID and
// page_map-search-exhausted early-outs — the *pte, addr signature it built
// on top of (an already-loaded copy of the PTE and the store_memory call
// that publishes it) needs no MAPEN save/restore dance here, since
// writePhysLongword already bypasses translation entirely.
func (m *Memory) validatePage(pteAddr uint32, pte *PTE) bool {
	if !m.vmValid {
		return false
	}

	pfn, ok := m.AllocatePage()
	if !ok {
		return false
	}

	pte.SetValid(true)
	pte.SetPFN(pfn)

	if err := m.writePhysLongword(pteAddr, uint32(*pte)); err != nil {
		return false
	}

	return true
}

// LookupPTE walks the page table for a virtual address the same way
// Translate does (region/page selection, P0/P1/S0 base-length-register
// check, recursive P0/P1 PTE-address translation), but performs no access
// itself: no protection or valid-bit check, no modify-bit update. For
// read-only diagnostic use (SHOW PAGE, console_show.c's tracevm) that
// wants to report a PTE's raw contents even for a page that wouldn't
// currently be a legal access.
//
// Returns the region index (0=P0, 1=P1, 2=S0) and the PTE's own address
// exactly as tracevm's own "PTE Address" line reports it (a virtual
// address for P0/P1, already-physical for S0, matching Translate's own
// pteVirtAddr before recursion), plus the PTE itself. Reports a
// *TranslationFault (AccessViolation) if MAPEN is off (there is no page
// table to walk) or the page number is outside the region's base/length
// registers, matching tracevm's own length-violation check.
func (m *Memory) LookupPTE(cpu *vax.CPU, addr uint32) (region int, pteAddr uint32, pte PTE, err error) {
	if cpu.PR(vax.MAPEN) == 0 {
		return 0, 0, 0, accessViolation(addr)
	}

	region = int((addr >> 30) & 0x3)
	page := (addr & 0x3FFFFFFF) >> 9

	var (
		pteVirtAddr  uint32
		pteRecursive bool
	)

	switch region {
	case 0:
		if page > cpu.PR(vax.P0LR) {
			return region, 0, 0, accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.P0BR) + page*4
		pteRecursive = true

	case 1:
		if page <= cpu.PR(vax.P1LR) {
			return region, 0, 0, accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.P1BR) + page*4
		pteRecursive = true

	case 2:
		if page > cpu.PR(vax.SLR) {
			return region, 0, 0, accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.SBR) + page*4
		pteRecursive = false

	default:
		return region, 0, 0, accessViolation(addr)
	}

	physPTEAddr := pteVirtAddr
	if pteRecursive {
		physPTEAddr, err = m.Translate(cpu, pteVirtAddr, AccessRead)
		if err != nil {
			return region, pteVirtAddr, 0, err
		}
	}

	raw, err := m.readPhysLongword(physPTEAddr)
	if err != nil {
		return region, pteVirtAddr, 0, err
	}

	return region, pteVirtAddr, PTE(raw), nil
}

// StorePTE writes pte back to the page table entry for a virtual address,
// the write-side counterpart to LookupPTE — matching console_set.c's setpte
// (SET PTE/SET PAGE): the same region/base/length-register walk LookupPTE
// does, but resolving all the way to the entry's physical address (via
// Translate for a recursively-mapped P0/P1 PTE) so the new value can
// actually be stored. This port has no translation-buffer cache to flush
// afterward (see LookupPTE's own doc comment and ShowTB's "not applicable to
// this port"), so there is no invalidate_page equivalent to call.
func (m *Memory) StorePTE(cpu *vax.CPU, addr uint32, pte PTE) error {
	if cpu.PR(vax.MAPEN) == 0 {
		return accessViolation(addr)
	}

	region := (addr >> 30) & 0x3
	page := (addr & 0x3FFFFFFF) >> 9

	var (
		pteVirtAddr  uint32
		pteRecursive bool
	)

	switch region {
	case 0:
		if page > cpu.PR(vax.P0LR) {
			return accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.P0BR) + page*4
		pteRecursive = true

	case 1:
		if page <= cpu.PR(vax.P1LR) {
			return accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.P1BR) + page*4
		pteRecursive = true

	case 2:
		if page > cpu.PR(vax.SLR) {
			return accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.SBR) + page*4
		pteRecursive = false

	default:
		return accessViolation(addr)
	}

	physPTEAddr := pteVirtAddr

	if pteRecursive {
		var err error

		physPTEAddr, err = m.Translate(cpu, pteVirtAddr, AccessRead)
		if err != nil {
			return err
		}
	}

	return m.writePhysLongword(physPTEAddr, uint32(pte))
}
