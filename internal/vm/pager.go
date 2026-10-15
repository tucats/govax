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
//   - TYP0 set, TYP1 clear: a *process section table index* in bits
//     21-0. The page belongs to a section of a file the process mapped
//     privately ($CRMPSC, or an image file), and the process section
//     table entry says which file and which block.
//   - TYP1 set, TYP0 clear: a *global page table index* in bits 21-0. The
//     page belongs to a global section; the global page table entry says
//     which section and which of its pages.
//   - both set: a *page file block* in bits 21-0. The page was written to
//     the page file, at that block; block 0 means a block will be given
//     to it when it's first written, and hasn't been yet.
//   - both clear, and a page frame number in bits 20-0: a page *in
//     transition*. The page has left the process's working set, but its
//     frame, that one, still holds it, on the free or modified page list;
//     a fault takes it back with no I/O (a "soft" fault).
//   - both clear, and the frame number 0: a demand-zero page, whose first
//     touch gives it a page of zeros.
//
// Only VMS paging (VMINIT/PAGING) writes the page file and transition
// forms; with it off, an invalid PTE is a section, a global page, or
// demand-zero.
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

// MaxPageFileBlock is the largest page file block an invalid PTE can
// hold.
const MaxPageFileBlock = pteMaskIndex

// SectionIndex returns an invalid PTE's process section table index, and
// whether it is one (valid bit clear, TYP0 set, TYP1 clear).
func (p PTE) SectionIndex() (uint32, bool) {
	if p.Valid() || p&pteBitTyp0 == 0 || p&pteBitTyp1 != 0 {
		return 0, false
	}

	return uint32(p) & pteMaskIndex, true
}

// PageFileBlock returns an invalid PTE's page file block, and whether it
// is one (valid bit clear, TYP0 and TYP1 set). Block 0 is a page whose
// block is yet to be allocated.
func (p PTE) PageFileBlock() (uint32, bool) {
	if p.Valid() || p&pteBitTyp0 == 0 || p&pteBitTyp1 == 0 {
		return 0, false
	}

	return uint32(p) & pteMaskIndex, true
}

// Transition returns the frame of an invalid PTE in transition, and
// whether it is one (valid bit clear, TYP0 and TYP1 clear, a frame number
// other than 0).
func (p PTE) Transition() (uint32, bool) {
	if p.Valid() || p&(pteBitTyp0|pteBitTyp1) != 0 || p.PFN() == 0 {
		return 0, false
	}

	return p.PFN(), true
}

// IsDemandZero reports whether an invalid PTE is the demand-zero form
// (valid bit clear, TYP0 and TYP1 clear, frame number 0). With paging
// off, any invalid page that isn't a section or global page is treated as
// demand-zero whatever its other bits say; this is the form itself.
func (p PTE) IsDemandZero() bool {
	return !p.Valid() && p&(pteBitTyp0|pteBitTyp1) == 0 && p.PFN() == 0
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

// PageFilePTE returns an invalid PTE holding page file block block (0:
// a block yet to be allocated), with protection prot and owner mode
// owner.
func PageFilePTE(block uint32, prot Protection, owner uint8) PTE {
	p := PTE(block&pteMaskIndex) | pteBitTyp0 | pteBitTyp1
	p.SetProtection(prot)
	p.SetOwner(owner)

	return p
}

// TransitionPTE returns an invalid PTE for a page in transition in frame
// pfn (which must not be 0, the demand-zero form), with protection prot
// and owner mode owner: the form a page's PTE takes when the page leaves
// its working set but keeps its frame.
func TransitionPTE(pfn uint32, prot Protection, owner uint8) PTE {
	var p PTE

	p.SetPFN(pfn)
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
	return m.DemandZeroFor(pte, FrameClaim{})
}

// DemandZeroFor is DemandZero for the page c describes: with paging on,
// the frame is recorded as that page's (its owner and backing store). A
// claim to fill the frame is ignored: a demand-zero page reads as zeros.
func (m *Memory) DemandZeroFor(pte PTE, c FrameClaim) (PTE, bool) {
	if !m.vmValid {
		return pte, false
	}

	c.Fill = false

	pfn, ok := m.AllocateFrame(c)
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
