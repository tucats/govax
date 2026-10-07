package vm

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/vax"
)

// AddressSpace names one process's private half of the virtual address
// space: where its P0 and P1 page tables are and how long they are (Phase
// 43).
//
// Every VAX process sees the same S0 (system) region, mapped by the one
// system page table SBR points at, but each has its own P0 (program) and P1
// (control/stack) regions, mapped by page tables of its own. The CPU knows
// only the current process's, through its P0BR/P0LR and P1BR/P1LR
// registers, which LDPCTX loads from the process's PCB on every process
// switch. Go code sometimes has to reach into a process that isn't the
// current one -- to write a mailbox message into a waiting reader's buffer,
// or to load an image into a process before it ever runs -- and an
// AddressSpace is how it says which process's P0 and P1 it means.
//
// The fields hold what the registers would hold:
//
//   - P0BR and P1BR are the S0 virtual addresses of the page tables'
//     entry 0. (P1BR is, as the architecture defines it, the address P1's
//     entry 0 *would* have: P1's table only describes its top pages, so
//     the entries for P1's low pages, below P1LR, don't exist.)
//   - P0LR is how many P0 pages the table describes; P1LR is the number
//     of the lowest P1 page that is *not* described (P1 grows down).
//
// The lengths are plain page counts: the PCB's longwords that also carry
// ASTLVL and PME beside the lengths must be unpacked first (internal/cpu's
// PCB does that).
type AddressSpace struct {
	P0BR, P0LR uint32
	P1BR, P1LR uint32
}

// CurrentAddressSpace returns the address space the CPU's registers
// describe now: the current process's.
func CurrentAddressSpace(cpu *vax.CPU) AddressSpace {
	return AddressSpace{
		P0BR: cpu.PR(vax.P0BR),
		P0LR: cpu.PR(vax.P0LR),
		P1BR: cpu.PR(vax.P1BR),
		P1LR: cpu.PR(vax.P1LR),
	}
}

// TranslateIn converts a virtual address to a physical one as Translate
// does, except that a P0 or P1 address is looked up in as's page tables
// rather than the current process's, and protection is always checked as
// kernel mode (the access is the system's, on the process's behalf). An S0
// address translates through the system page table, as it does for every
// process.
//
// It neither reads nor fills the translation buffer or the STC: those
// cache the current process's translations, and an entry for another
// process's page would be wrong the moment that process's own P0 address
// was looked up through the registers. So each call walks the page
// tables. As Translate does, it allocates a demand-zero page for a valid
// PTE that has none yet (validatePage) and sets the modify bit on a write.
// With MAPEN off there is no translation at all, and addr is returned as
// it is, as Translate returns it.
func (m *Memory) TranslateIn(cpu *vax.CPU, as AddressSpace, addr uint32, access AccessType) (uint32, error) {
	if cpu.PR(vax.MAPEN) == 0 {
		return addr, nil
	}

	pteAddr, err := m.spacePTEAddress(cpu, as, addr)
	if err != nil {
		return 0, err
	}

	return m.resolvePTE(pteAddr, addr, access)
}

// spacePTEAddress finds the physical address of the PTE that maps addr in
// as (or in the system page table, for an S0 address), checking the
// region's length register as translate does.
func (m *Memory) spacePTEAddress(cpu *vax.CPU, as AddressSpace, addr uint32) (uint32, error) {
	region := (addr >> 30) & 0x3
	page := (addr & 0x3FFFFFFF) >> 9

	var pteVirtAddr uint32

	switch region {
	case 0: // P0 grows up: pages 0 through P0LR-1 exist.
		if page >= as.P0LR {
			return 0, accessViolation(addr)
		}

		pteVirtAddr = as.P0BR + page*4

	case 1: // P1 grows down: only pages above P1LR exist.
		if page < as.P1LR {
			return 0, accessViolation(addr)
		}

		pteVirtAddr = as.P1BR + page*4

	case 2: // S0: SBR is a physical address, so the PTE's is too.
		if page >= cpu.PR(vax.SLR) {
			return 0, accessViolation(addr)
		}

		return cpu.PR(vax.SBR) + page*4, nil

	default: // S1 doesn't exist.
		return 0, accessViolation(addr)
	}

	// A process page table lives in S0, so the PTE's own address is an S0
	// virtual address; it is translated through the system page table,
	// which is the same for every process (the recursion translate does
	// through Translate, here without the TB).
	if (pteVirtAddr>>30)&0x3 != 2 {
		return 0, accessViolation(addr)
	}

	return m.TranslateIn(cpu, as, pteVirtAddr, AccessRead)
}

// resolvePTE finishes a translation once the PTE's physical address is
// known: the protection check (as kernel mode), demand-zero allocation for
// an invalid page, and the modify bit for a write; translate's last steps,
// without its caches.
func (m *Memory) resolvePTE(pteAddr, addr uint32, access AccessType) (uint32, error) {
	raw, err := m.readPhysLongword(pteAddr)
	if err != nil {
		return 0, err
	}

	pte := PTE(raw)

	if !pte.Protection().allows(vax.Kernel, access) {
		return 0, protectionViolation(addr, byte(access))
	}

	if !pte.Valid() && !m.validatePage(pteAddr, &pte) {
		return 0, translationNotValid(addr)
	}

	if access == AccessWrite && !pte.Modified() {
		pte.SetModified(true)

		if err := m.writePhysLongword(pteAddr, uint32(pte)); err != nil {
			return 0, err
		}
	}

	m.translationCount++

	return pte.PFN()<<9 + addr&(pageSize-1), nil
}

// LoadIn reads len(dest) bytes starting at addr in as (see TranslateIn)
// into dest. It translates once per page the bytes touch, so it may cross
// page boundaries freely.
func (m *Memory) LoadIn(cpu *vax.CPU, as AddressSpace, addr uint32, dest []byte) error {
	for len(dest) > 0 {
		paddr, err := m.TranslateIn(cpu, as, addr, AccessRead)
		if err != nil {
			return err
		}

		n := spanInPage(addr, len(dest))

		if err := m.LoadPhysical(paddr, dest[:n]); err != nil {
			return err
		}

		addr += uint32(n)
		dest = dest[n:]
	}

	return nil
}

// StoreIn writes src starting at addr in as (see TranslateIn), one page's
// worth at a time. Nothing is written past the first page that fails to
// translate; the pages before it have been written.
func (m *Memory) StoreIn(cpu *vax.CPU, as AddressSpace, addr uint32, src []byte) error {
	for len(src) > 0 {
		paddr, err := m.TranslateIn(cpu, as, addr, AccessWrite)
		if err != nil {
			return err
		}

		n := spanInPage(addr, len(src))

		if err := m.StorePhysical(paddr, src[:n]); err != nil {
			return err
		}

		addr += uint32(n)
		src = src[n:]
	}

	return nil
}

// LoadLongwordIn reads a longword at addr in as.
func (m *Memory) LoadLongwordIn(cpu *vax.CPU, as AddressSpace, addr uint32) (uint32, error) {
	var buf [4]byte

	if err := m.LoadIn(cpu, as, addr, buf[:]); err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint32(buf[:]), nil
}

// StoreLongwordIn writes a longword at addr in as.
func (m *Memory) StoreLongwordIn(cpu *vax.CPU, as AddressSpace, addr uint32, v uint32) error {
	var buf [4]byte

	binary.LittleEndian.PutUint32(buf[:], v)

	return m.StoreIn(cpu, as, addr, buf[:])
}

// spanInPage is how many of n bytes starting at addr lie in addr's page.
func spanInPage(addr uint32, n int) int {
	left := int(pageSize - addr&(pageSize-1))
	if n < left {
		return n
	}

	return left
}
