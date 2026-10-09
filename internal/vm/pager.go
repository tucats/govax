package vm

// The pager hook (docs/PHASE-49 - record updates and locks.md, subtask 9).
//
// # What happens on a page fault
//
// A page whose PTE has its valid bit clear has no physical page behind it
// right now. When a program touches it (and the PTE's protection allows
// the access, which the hardware checks first), the VAX takes a
// "translation not valid" fault, and the operating system's *pager*
// decides where the page's contents are, gives it a physical page, fills
// that page, makes the PTE valid, and lets the instruction run again.
//
// Where the contents are is written in the invalid PTE itself. *VAX/VMS
// Internals and Data Structures* (section 14.1.1, figure 14-3) lays out
// the forms an invalid PTE takes; bits 31 (valid), 30-27 (protection), and
// 24-23 (owner) mean the same in all of them, and bits 26 (TYP0) and 22
// (TYP1) say which form the rest is:
//
//   - TYP0 set: a *process section table index* in bits 21-0. The page
//     belongs to a section of a file the process mapped privately
//     ($CRMPSC), and the process section table entry says which file and
//     which block.
//   - TYP1 set, TYP0 clear: a *global page table index* in bits 21-0. The
//     page belongs to a global section; the global page table entry says
//     which section and which of its pages.
//   - both clear, and the other bits 0: a demand-zero page, whose first
//     touch gives it a page of zeros. (VMS also keeps a page file block
//     number or a page "in transition" in this form; govax has neither
//     yet.)
//
// (In a *valid* PTE, bit 26 is the modify bit and bit 22 a software bit;
// the forms apply only while the valid bit is clear.)
//
// # How govax does it
//
// A Memory with no Pager does what it always has: every invalid PTE whose
// access is allowed is demand-zero (validatePage), whatever its other
// bits. Once a Pager is installed (SetPager), every such fault goes to it
// instead: it returns the PTE the page should now have, valid and pointing
// at the physical page it filled, or reports that it can't, and the
// access is then a translation-not-valid fault. internal/corevms installs
// the pager that reads section pages from their files and hands out
// demand-zero pages (DemandZero) for the rest.
//
// This is the seam a working set manager and a page file would use: they
// would decide which pages stay valid, write modified pages out when they
// are taken away, and leave an invalid PTE that says where each page went,
// for PageIn to find.

// PTE type bits of an invalid page table entry (see above).
const (
	pteBitTyp0 = 1 << 26
	pteBitTyp1 = 1 << 22

	// pteMaskIndex is the index field of a section or global PTE: bits
	// 21-0.
	pteMaskIndex = 0x3FFFFF
)

// MaxPTEIndex is the largest process section table index or global page
// table index an invalid PTE can hold.
const MaxPTEIndex = pteMaskIndex

// SectionIndex returns an invalid PTE's process section table index, and
// whether it is one (valid bit clear, TYP0 set).
func (p PTE) SectionIndex() (uint32, bool) {
	if p.Valid() || p&pteBitTyp0 == 0 {
		return 0, false
	}

	return uint32(p) & pteMaskIndex, true
}

// GlobalIndex returns an invalid PTE's global page table index, and
// whether it is one (valid bit clear, TYP1 set, TYP0 clear).
func (p PTE) GlobalIndex() (uint32, bool) {
	if p.Valid() || p&pteBitTyp0 != 0 || p&pteBitTyp1 == 0 {
		return 0, false
	}

	return uint32(p) & pteMaskIndex, true
}

// SectionPTE returns an invalid PTE holding process section table index
// index, with protection prot and owner mode owner.
func SectionPTE(index uint32, prot Protection, owner uint8) PTE {
	p := PTE(index&pteMaskIndex) | pteBitTyp0
	p.SetProtection(prot)
	p.SetOwner(owner)

	return p
}

// GlobalPTE returns an invalid PTE holding global page table index index,
// with protection prot and owner mode owner.
func GlobalPTE(index uint32, prot Protection, owner uint8) PTE {
	p := PTE(index&pteMaskIndex) | pteBitTyp1
	p.SetProtection(prot)
	p.SetOwner(owner)

	return p
}

// ValidPTE returns a valid PTE for physical page pfn, with protection
// prot and owner mode owner, not yet modified: what a pager makes a page's
// PTE once it's in memory.
func ValidPTE(pfn uint32, prot Protection, owner uint8) PTE {
	var p PTE

	p.SetValid(true)
	p.SetPFN(pfn)
	p.SetProtection(prot)
	p.SetOwner(owner)

	return p
}

// PageFault describes a fault on an invalid page for a Pager.
type PageFault struct {
	// Space is the address space the page belongs to: the current
	// process's, or, for an access through TranslateIn (LoadIn, StoreIn),
	// the one named there. It doesn't matter for an S0 page.
	Space AddressSpace

	// Addr is the virtual address touched, and PTEAddr the physical
	// address of the page's PTE, which PTE holds.
	Addr, PTEAddr uint32
	PTE           PTE

	// Write is set when the access is a write.
	Write bool
}

// Pager resolves page faults on invalid pages (see this file's opening
// comment).
type Pager interface {
	// PageIn returns the PTE the faulting page should now have, valid
	// and pointing at a physical page holding the page's contents, or
	// false if it can't be brought in (the access then faults). Memory
	// writes the PTE; the modify bit is Memory's to set, on a write.
	PageIn(f PageFault) (PTE, bool)
}

// SetPager installs the pager that resolves faults on invalid pages; nil
// goes back to treating every invalid page as demand-zero.
func (m *Memory) SetPager(p Pager) { m.pager = p }

// DemandZero gives a demand-zero page its physical page: it claims a free
// page (AllocatePage hands out pages already zeroed) and returns pte made
// valid and pointing at it, its other forms' bits cleared. It reports
// false if no VMINIT has run yet or physical memory is exhausted.
func (m *Memory) DemandZero(pte PTE) (PTE, bool) {
	if !m.vmValid {
		return pte, false
	}

	pfn, ok := m.AllocatePage()
	if !ok {
		return pte, false
	}

	return ValidPTE(pfn, pte.Protection(), pte.Owner()), true
}

// pageIn resolves a fault on the invalid page whose PTE is *pte, at
// physical address f.PTEAddr: through the pager if one is installed, or
// as a demand-zero page. On success *pte is the new, valid PTE, already
// written back.
func (m *Memory) pageIn(f PageFault, pte *PTE) bool {
	if m.pager == nil {
		return m.validatePage(f.PTEAddr, pte)
	}

	f.PTE = *pte

	next, ok := m.pager.PageIn(f)
	if !ok || !next.Valid() {
		return false
	}

	if err := m.writePhysLongword(f.PTEAddr, uint32(next)); err != nil {
		return false
	}

	*pte = next

	return true
}
