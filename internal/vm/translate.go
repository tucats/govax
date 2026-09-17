package vm

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
)

// AccessType distinguishes a read access from a write access during address
// translation and protection checking (vm.c's VM_READ/VM_WRITE).
//
// `const (... = iota ...)` is Go's idiom for a set of related integer
// constants: iota starts at 0 inside a const block and increases by one on
// each subsequent line, so AccessRead is 0 and AccessWrite is 1, without
// having to spell out either number. This same pattern appears again for
// FaultKind just below, and it's how Go typically expresses what other
// languages call an "enum".
type AccessType int

const (
	AccessRead AccessType = iota
	AccessWrite
)

// pageSize is the VAX virtual memory page size in bytes. Every virtual and
// physical page is exactly this many bytes, and every page-table entry
// (see pte.go) describes exactly one such page.
const pageSize = 512

// FaultKind identifies why a virtual address failed to translate. VAX
// documentation and the C source both call a failed memory access a
// "fault" — not necessarily an error in the emulator, but a signal the
// running program (or, more often, its operating system) is expected to
// handle, such as by paging in a page that was swapped out, or by
// terminating a program that touched memory it had no business touching.
type FaultKind int

const (
	// AccessViolation: the page number is outside the region's base/length
	// registers -- vm.c's own EXC_ACCVIO subcode 0x0001. In plain terms:
	// the address doesn't correspond to any page this region's page table
	// even covers — e.g. an address further into the stack than the
	// process has ever grown, or past the end of a program's allocated
	// heap.
	AccessViolation FaultKind = iota
	// ProtectionViolation: the page is in range, but its protection code
	// denies this access at the current privilege mode -- vm.c's own
	// EXC_ACCVIO subcode 0x0002. In plain terms: the page exists and is
	// mapped, but the rules recorded in its PTE (see pte.go's Protection
	// type) say the CPU's current privilege level isn't allowed to do
	// this particular read or write to it — e.g. a user-mode program
	// trying to write to a page the kernel marked read-only. Split out
	// from AccessViolation in Phase 12 (previously collapsed into one
	// kind, always reporting the length/base subcode regardless of which
	// check actually failed) -- see docs/DEVIATIONS.md.
	ProtectionViolation
	// TranslationNotValid: the page is in range and the access is
	// permitted, but the page table entry's valid bit is clear. In plain
	// terms: this virtual page is a legitimate part of the address space
	// and the access would otherwise be allowed, but there's currently no
	// physical page backing it — on a real VAX/VMS system this is usually
	// resolved by paging the data back in from disk; this emulator's own
	// (much simpler) equivalent is validatePage's demand-paging below.
	TranslationNotValid
)

// TranslationFault reports a failed virtual-to-physical address translation.
// Turning this into the CPU-level exception/fault machinery (EXC_ACCVIO,
// EXC_TNV, and the rest of set_fault's bookkeeping) belongs to Phase 03,
// which owns fault/interrupt handling; this only reports what went wrong at
// the given virtual address.
//
// Like memory.go's PhysicalAddressError, this is a custom error type: its
// Error() string method (below) is what makes *TranslationFault satisfy
// Go's built-in `error` interface, so it can be returned from Translate
// wherever an `error` is expected. Keeping Kind/Addr/Mask as their own
// fields (rather than only producing a formatted string) means a caller
// further up the stack — eventually Phase 03's fault-handling machinery —
// can inspect *which* fault happened and act differently for a
// TranslationNotValid than for a ProtectionViolation, instead of having to
// pattern-match on error text.
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
// This is the method to read first if you're trying to understand how VAX
// address translation actually works — everything else in this file is
// either a variant of it (ProbeTranslate, LookupPTE, StorePTE) or a helper
// it calls (validatePage). A few architectural terms it relies on:
//
//   - MAPEN ("map enable") is a processor register: when it's zero,
//     virtual memory is switched off entirely and every address is used
//     as-is, which is how a VAX starts up before its operating system has
//     set up any page tables.
//   - P0BR/P0LR, P1BR/P1LR, and SBR/SLR are three pairs of "base and
//     length" registers, one pair per region (P0, P1, S0 — see the
//     package doc comment). The base register holds the physical address
//     where that region's page table starts; the length register limits
//     how many pages that table actually describes (a table doesn't have
//     to cover the full theoretical size of its region — see the region
//     switch inside translate below for exactly how the limit is
//     checked).
//   - cur_mod is part of the Processor Status Longword (PSL) and records
//     which of the four privilege modes (Kernel/Executive/Supervisor/
//     User) the CPU is currently running in — see pte.go's Protection
//     type for how that interacts with a page's protection code.
//
// This is a direct port of vm.c's vm() routine called with a "real",
// signaling access -- see ProbeTranslate for the VM_NOSIGNAL counterpart,
// and docs/PHASE-21.md for the translation-buffer/sequential-translation-
// cache design this and ProbeTranslate share via translate.
func (m *Memory) Translate(cpu *vax.CPU, addr uint32, access AccessType) (uint32, error) {
	return m.translate(cpu, addr, access, true)
}

// ProbeTranslate is Translate for a caller matching vm.c's VM_NOSIGNAL
// mode flag: PROBEx's own addressability check (emul_misc.c's emul_probe),
// and LookupPTE's recursive PTE-address step (tracevm's own VM_NOSIGNAL
// call for the same purpose). The only observable difference from
// Translate is that the sequential translation cache's own one-slot hit
// check is never taken -- vm.c's STC compares the *raw* mode parameter
// (which carries the VM_NOSIGNAL bit for these callers) against a
// cached_mbit that's only ever written with that bit already stripped, so
// a VM_NOSIGNAL translation can never hit the STC, only the 128-entry TB
// (whose own hit-check runs after the bit is stripped in the C source).
// Population of either cache on a successful walk is unaffected: vm.c
// does that unconditionally regardless of the flag. See docs/PHASE-21.md.
func (m *Memory) ProbeTranslate(cpu *vax.CPU, addr uint32, access AccessType) (uint32, error) {
	return m.translate(cpu, addr, access, false)
}

// translate is the shared implementation behind Translate/ProbeTranslate.
// signal is true for a real (Translate) access and false for a probing
// (ProbeTranslate) one -- see that function's doc comment for its one
// effect (gating the STC hit-check).
//
// DYNVM dynamic page-in-on-demand (validate_page(), gated on
// vax.console.vminit_valid/page_map) is console/microkernel state that
// belongs to Phase 08's VMINIT command; until that exists, an invalid page
// table entry always faults TNV here, which is also exactly what
// validate_page() itself does whenever VMVALID is false (the normal case
// until a real OS or VMINIT has run).
func (m *Memory) translate(cpu *vax.CPU, addr uint32, access AccessType, signal bool) (uint32, error) {
	// cpu.PR(vax.MAPEN) reads the MAPEN processor register's current
	// value; a machine that hasn't enabled virtual memory yet (MAPEN==0)
	// uses every address as a physical address directly, with no
	// translation step at all.
	if cpu.PR(vax.MAPEN) == 0 {
		return addr, nil
	}

	// Every virtual address splits into two parts: which *page* it falls
	// in, and the *offset* of this particular byte within that page.
	// Since pageSize (512) is a power of two, both can be computed with
	// bit masking instead of slower division/remainder arithmetic:
	// pageSize-1 (511, i.e. 0x1FF) has exactly the low 9 bits set, so
	// `addr &^ (pageSize-1)` clears those low 9 bits, leaving just the
	// page-aligned base address of addr's page (vpage), while
	// `addr & (pageSize-1)` keeps only those low 9 bits, giving the
	// offset within the page (byteOffset). The final physical address
	// for *any* successful translation below is always
	// "the physical page's base address" + byteOffset — the offset within
	// the page never changes during translation, only which page it's an
	// offset into.
	vpage := addr &^ uint32(pageSize-1)
	byteOffset := addr & (pageSize - 1)

	// The sequential translation cache: a one-slot fast path for the
	// (very common) case of the same page/mode as the immediately
	// preceding translation, consulted before the 128-entry TB and
	// unaffected by TBDR -- matching vm.c's own #if STC block, always
	// compiled in. See ProbeTranslate's doc comment for why this check is
	// gated on signal.
	m.tb.stcTries++

	if signal && m.tb.stcValid && vpage == m.tb.stcVPage && access == m.tb.stcMode {
		m.tb.stcHits++

		return m.tb.stcPPage + byteOffset, nil
	}

	// region is the address's top two bits (>>30 shifts them down to
	// occupy just bits 0-1; &0x3 is a defensive mask, since >>30 of a
	// 32-bit value can only ever leave 2 bits anyway) — see the package
	// doc comment for what the four possible values (0=P0, 1=P1, 2=S0,
	// 3=S1) mean. page is everything else: mask off the region bits
	// (& 0x3FFFFFFF, the low 30 bits) and then divide by pageSize the
	// same way vpage/byteOffset did above (>>9 instead of &^/& because
	// this time we want the page *number*, not a page-aligned address —
	// i.e. "which 512-byte page is this," not "the address of that
	// page's first byte").
	region := (addr >> 30) & 0x3
	page := (addr & 0x3FFFFFFF) >> 9

	idx := tbIndex(region, page)
	entry := &m.tb.entries[idx]

	// The 128-entry translation buffer, consulted only when TBDR == 0
	// ("TB caching enabled") -- matching vm.c's `if (!vax.TBDR)`. Note
	// this gates consultation only: a successful full walk below still
	// populates entry regardless of TBDR, exactly as vm.c does.
	if cpu.PR(vax.TBDR) == 0 {
		m.tb.tries++

		if entry.valid && entry.page == page && entry.protMode == access {
			m.tb.hits++

			paddr := entry.paddr + byteOffset

			m.tb.stcValid = true
			m.tb.stcVPage = vpage
			m.tb.stcPPage = entry.paddr
			m.tb.stcMode = access

			if cpu.DebugEnabled(vax.DebugTB) {
				ratio := 0.0
				if m.tb.tries > 0 {
					ratio = float64(m.tb.hits) / float64(m.tb.tries) * 100.0
				}

				fmt.Fprintf(cpu.DebugWriter(), "DEBUG(TB):  TB CACHE HIT IDX=%02X; VA=%08X  PA=%08X RATIO=%d%%\n",
					idx, addr, paddr, int(ratio))
			}

			return paddr, nil
		}
	}

	// pteVirtAddr will hold the address of this page's page table entry,
	// and pteRecursive records whether that address is itself a *virtual*
	// address needing its own translation (true for P0/P1) or already a
	// physical one that can be used as-is (false for S0) — see the
	// per-region cases below for why they differ.
	var (
		pteVirtAddr  uint32
		pteRecursive bool
	)

	// This switch is where the region number selected above actually
	// determines how to find the right page table. Every case does the
	// same three things: (1) check whether `page` falls within the
	// bounds this region's page table actually covers, faulting
	// AccessViolation if not; (2) compute the address of this page's own
	// page table entry, by treating the base register as the address of
	// entry 0 and adding page*4 (each PTE is 4 bytes/one longword, so
	// entry N starts at base + N*4 — exactly the same "index times
	// element size" arithmetic as indexing a Go slice, just done by hand
	// on a raw address instead of through slice syntax); and (3) record
	// whether that PTE address still needs its own translation.
	//
	// On failure, every case first clears this TB slot (`*entry =
	// tbEntry{}`, i.e. "assign entry the zero value of tbEntry" — see
	// tb.go) and flushes the STC, exactly mirroring vm.c's own behavior
	// of invalidating the slot around every fault path, not just the
	// success path — a subtlety carried over from the reference
	// implementation rather than something this port added on its own.
	switch region {
	case 0: // P0: process program region.
		// P0LR is the number of pages actually mapped, growing upward
		// from page 0 — a P0 address is in range only if its page number
		// doesn't exceed that count.
		if page > cpu.PR(vax.P0LR) {
			*entry = tbEntry{}

			m.tb.stcFlush()

			return 0, accessViolation(addr)
		}

		// P0BR (P0 Base Register) holds the *virtual* address of P0's
		// page table's first entry (it lives in the system region, S0,
		// which is why pteRecursive is true below: finding *this* page's
		// mapping first requires translating the address of the page
		// table itself).
		pteVirtAddr = cpu.PR(vax.P0BR) + page*4
		pteRecursive = true

	case 1: // P1: process control region. P1BR/P1LR list invalid pages, so
		// the length comparison is inverted relative to P0: P1 grows
		// *downward* from the top of the region (this is where the stack
		// lives, and stacks conventionally grow toward lower addresses),
		// so P1LR instead marks the *lowest* page number still considered
		// part of the region — anything at or below P1LR is out of
		// bounds, the mirror image of P0's check.
		if page <= cpu.PR(vax.P1LR) {
			*entry = tbEntry{}

			m.tb.stcFlush()

			return 0, accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.P1BR) + page*4
		pteRecursive = true

	case 2: // S0: system region. SBR is already a physical address.
		// Unlike P0BR/P1BR, SBR (System Base Register) holds a *physical*
		// address directly — the system region's own page table doesn't
		// need translating to be found, since the kernel that owns it is
		// mapped in a fixed, always-resident location. That's exactly why
		// pteRecursive is false here but true for P0/P1 above.
		if page > cpu.PR(vax.SLR) {
			*entry = tbEntry{}

			m.tb.stcFlush()

			return 0, accessViolation(addr)
		}

		pteVirtAddr = cpu.PR(vax.SBR) + page*4
		pteRecursive = false

	default: // S1: unsupported by this implementation, same as the C source.
		*entry = tbEntry{}

		m.tb.stcFlush()

		return 0, accessViolation(addr)
	}

	// pteAddr will end up as the *physical* address of the PTE itself —
	// for S0 that's just pteVirtAddr already (see above), but for a
	// recursively-mapped P0/P1 PTE, its own virtual address must first be
	// translated through Translate, exactly the same function this whole
	// block is inside of. This is the recursion the package doc comment
	// mentions: translating a P0/P1 address requires translating another
	// (system-region) address first, to find the page table that
	// describes the first one.
	pteAddr := pteVirtAddr

	if pteRecursive {
		var err error

		pteAddr, err = m.Translate(cpu, pteVirtAddr, AccessRead)
		if err != nil {
			return 0, err
		}
	}

	// With the PTE's physical address known, read it directly (no further
	// translation — a PTE's own storage is addressed physically) and
	// interpret the raw 32-bit value as a PTE (see pte.go for what its
	// bits mean).
	raw, err := m.readPhysLongword(pteAddr)
	if err != nil {
		return 0, err
	}

	pte := PTE(raw)

	if cpu.DebugEnabled(vax.DebugVM) {
		pa := pte.PFN()<<9 + byteOffset

		fmt.Fprintf(cpu.DebugWriter(), "DEBUG(VM): VA=%08X  R=%02d PTEA=%08X PTE=%08X P=%02X M=%02X PA=%08X\n",
			addr, region, pteAddr, uint32(pte), pte.Protection(), access, pa)
	}

	// The page exists and is described by a real PTE, but is this
	// particular access (read or write, at the CPU's current privilege
	// mode) actually allowed against it? See pte.go's Protection.allows
	// for the full logic; cpu.PSL().CurMod() is how the current privilege
	// mode is read out of the Processor Status Longword.
	if !pte.Protection().allows(cpu.PSL().CurMod(), access) {
		*entry = tbEntry{}

		m.tb.stcFlush()

		return 0, protectionViolation(addr, byte(access))
	}

	// The access is permitted in principle, but the PTE's Valid bit might
	// still be clear, meaning no physical page is actually behind this
	// virtual page right now. validatePage (below) is this emulator's
	// simplified stand-in for real demand-paging: rather than reading the
	// missing page back in from a disk (which this emulator doesn't
	// model), it just grabs any free physical page and marks the PTE
	// valid, as if the page had always been full of zeros. If even that
	// fails (no VMINIT has run yet, or physical memory is exhausted), the
	// access becomes a genuine TranslationNotValid fault.
	if !pte.Valid() {
		ok := m.validatePage(pteAddr, &pte)

		// Matching vm.c's own `tbp->page = -1L; STC_FLUSH;` immediately
		// around its validate_page() call: the current slot is
		// invalidated regardless of whether demand-paging succeeded.
		*entry = tbEntry{}
		
		m.tb.stcFlush()

		if !ok {
			return 0, translationNotValid(addr)
		}
	}

	// The Modify bit (see pte.go) records whether a page has ever been
	// written to, so the operating system can tell which pages are
	// "dirty" and would need writing back to disk if evicted. Hardware
	// sets it automatically, the first time a write happens — which is
	// exactly what this does: only on a write, and only if the bit isn't
	// already set (no need to rewrite the PTE to set a bit that's already
	// set).
	if access == AccessWrite && !pte.Modified() {
		pte.SetModified(true)

		if err := m.writePhysLongword(pteAddr, uint32(pte)); err != nil {
			return 0, err
		}
	}

	m.translationCount++

	// Finally, the actual answer: the physical page's base address
	// (pte.PFN()<<9 — see pte.go's PFN method) plus the byteOffset
	// computed at the very top of this function. Having gone to the
	// trouble of a full page-table walk, the result is also written into
	// both caches (the TB slot selected earlier, and the one-slot STC) so
	// that the *next* access to this same page — very likely to happen
	// soon, per the locality-of-reference reasoning in tb.go's doc
	// comment — can skip straight to the fast paths near the top of this
	// function instead of walking the page table all over again.
	paddr := pte.PFN()<<9 + byteOffset

	*entry = tbEntry{valid: true, page: page, paddr: pte.PFN() << 9, code: pte.Protection(), protMode: access}

	m.tb.stcValid = true
	m.tb.stcVPage = vpage
	m.tb.stcPPage = pte.PFN() << 9
	m.tb.stcMode = access

	return paddr, nil
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
//
// pte is a `*PTE` (a pointer) rather than a plain PTE so that
// pte.SetValid(true)/pte.SetPFN(pfn) below modify the caller's own PTE
// variable — the copy translate already loaded from memory — rather than
// a throwaway local copy; translate then uses that same updated value both
// to compute the physical address it returns and to populate the
// translation cache, without having to read the PTE back from memory a
// second time. See pte.go's SetPFN doc comment for the general rule on
// when a pointer receiver/parameter is needed.
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
//
// The region/page/base-and-length-register logic here is the same
// per-region switch translate uses internally (see its own, more heavily
// commented copy above) — repeated here rather than factored into a
// shared helper because LookupPTE and StorePTE each need to do slightly
// different things with the intermediate pteVirtAddr/pteRecursive result
// (LookupPTE only reads the PTE; StorePTE writes it; translate also
// checks protection/validity and updates the TB). If you're trying to
// understand *what* a page table walk does, read translate's copy first;
// this one is the same logic adapted for a read-only diagnostic caller.
//
// The four return values are declared as *named* return parameters
// (region int, pteAddr uint32, pte PTE, err error) rather than left
// anonymous. Doing so lets a `return` statement provide new values for
// each (as every return below does) while also documenting, right in the
// function signature, what each position in the result means — handy
// here since a plain `(int, uint32, PTE, error)` signature alone wouldn't
// tell a caller which uint32 is which.
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
		// Matching tracevm's own recursive PTE-address translation, which
		// passes VM_READ | VM_NOSIGNAL -- see ProbeTranslate's doc
		// comment.
		physPTEAddr, err = m.ProbeTranslate(cpu, pteVirtAddr, AccessRead)
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
// actually be stored. Invalidates addr's own translation-buffer slot on a
// successful write, matching setpte's own invalidate_page(addr) call — the
// PFN database entry just changed, so any cached mapping for it is stale.
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

	if err := m.writePhysLongword(physPTEAddr, uint32(pte)); err != nil {
		return err
	}

	m.InvalidatePage(addr)

	return nil
}
