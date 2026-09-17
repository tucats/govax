package vm

// This is the Go port of vm.c's translation buffer cache (struct TB
// tb[128]) and its "sequential translation cache" (STC, the one-slot
// cached_virtual_page/cached_physical_page fast path) -- see
// docs/PHASE-21.md for the full design rationale, including why this
// reverses Phase 02/16's earlier decision not to port it.
//
// # Why cache a translation at all
//
// As translate.go's package doc comment explains, converting a virtual
// address to a physical one means walking a page table in memory — at
// least one extra memory read (sometimes more, since a P0/P1 page table's
// own address must itself be translated through the system region's page
// table first). Doing that on *every single* memory access a program
// makes would be needlessly slow, especially since real programs tend to
// access the same handful of pages over and over in a short span of time
// (a property usually called "locality of reference"). A translation
// buffer (real VAX hardware has one too — this isn't just an emulator
// trick) is a small cache remembering the results of recent translations,
// so a repeat access to a page whose mapping is already cached can skip
// the page-table walk entirely.
//
// This is purely a speed optimization: consulting the cache and walking
// the page table always produce the same physical address for the same
// virtual address (as long as the cache is correctly invalidated whenever
// a page table entry changes — see InvalidateTB/InvalidatePage/
// InvalidateProtection below). Nothing in translate.go's actual address-
// translation *logic* depends on this file; if you're trying to understand
// what a virtual address means, read translate.go's Translate first, and
// come back here once you're curious about performance or about matching
// the reference emulator's own cache-hit statistics (SHOW TB).
//
// # Two caches, not one
//
// This file actually implements two caches layered on top of each other,
// matching the reference C source exactly:
//
//   - The "sequential translation cache" (STC): a single slot remembering
//     only the *most recent* translation. Checked first, and very cheap
//     to check, because most real code accesses memory sequentially
//     (fetching successive instruction bytes, walking an array, and so
//     on) and so tends to repeatedly hit the same page it just used.
//   - The 128-entry translation buffer (TB) proper: a bigger cache that
//     can remember many pages at once, indexed by tbIndex (see below), so
//     a page can still be found quickly even after other pages have been
//     accessed in between.
//
// Every real translation checks the STC first, then the TB, and only
// falls back to actually walking the page table if both miss — see
// translate.go's translate for the full sequence.

// tbSize matches vm.c's struct TB tb[128]: 4 regions * 32 (5 low bits of
// page number) slots.
const tbSize = 128

// tbProtInvalid is the Go equivalent of vm.c's TB_INVALID (2): a sentinel
// stored in tbEntry.protMode distinct from AccessRead(0)/AccessWrite(1),
// meaning "this slot's mapping may still be valid, but its protection must
// be re-checked" -- set by InvalidateProtection without needing a full
// mapping flush.
const tbProtInvalid = AccessType(2)

// tbEntry is the Go equivalent of vm.c's struct TB: everything cached about
// one virtual page's mapping, corresponding one-to-one with a page table
// entry's own PFN and protection code (see pte.go), plus a couple of
// cache-management fields with no PTE equivalent. valid replaces the C
// source's page == -1 sentinel for "this slot is empty"; protMode replaces
// prot_valid, doing the same double duty (either the last-verified
// AccessType, or tbProtInvalid).
type tbEntry struct {
	valid    bool
	page     uint32
	paddr    uint32
	code     Protection
	protMode AccessType
}

// tbIndex computes vm.c's tb_idx: (region << 5) + (page & 0x1F).
//
// The 128-entry cache can't have a slot for every possible virtual page —
// there are far more than 128 pages in the address space — so instead
// every page number is mapped down ("hashed") onto one of 128 slots, and
// that slot is shared by every page that happens to map to the same index.
// This particular mapping keeps the low 5 bits of the page number
// (page & 0x1F, so 32 possible values — 0x1F is 0b11111, a mask for the
// bottom 5 bits) and combines them with the 2-bit region number shifted up
// by 5 bits (region << 5), so each of the 4 regions (P0/P1/S0/S1, see the
// package doc comment) gets its own disjoint block of 32 slots within the
// 128 total (4 regions * 32 = 128 = tbSize). Two different pages that
// share the same low 5 bits and region will collide in the same slot —
// whichever was cached most recently simply overwrites the other's entry,
// which is fine for a performance cache (a "miss" just means falling back
// to the slower page-table walk, never a wrong answer).
func tbIndex(region, page uint32) int {
	return int((region << 5) + (page & 0x1F))
}

// tb holds the Memory's translation-buffer cache state: the 128-entry
// array plus its counters, and the independent one-slot sequential
// translation cache (STC) plus its own counters.
//
// `entries [tbSize]tbEntry` is a Go array of exactly tbSize (128) tbEntry
// values, embedded directly in tb — unlike memory.go's ram/pageMap slices,
// its size never changes, so a fixed-size array (no separate heap
// allocation, contiguous storage right inside tb, and inside Memory in
// turn) is the natural fit. tries/hits/flushes/pflushes are running
// counters purely for statistics (SHOW TB); they never influence the
// result of a translation, only what gets reported about it.
type tb struct {
	entries [tbSize]tbEntry

	tries, hits, flushes, pflushes int64

	// stcValid/stcVPage/stcPPage/stcMode are the STC's own one-slot cache
	// of the most recent translation, matching vm.c's cached_virtual_page/
	// cached_physical_page/cached_mbit (stcValid replaces the -1 sentinel
	// on cached_virtual_page).
	stcValid          bool
	stcVPage          uint32
	stcPPage          uint32
	stcMode           AccessType
	stcTries, stcHits int64
}

// stcFlush is the Go equivalent of vm.c's STC_FLUSH macro.
func (t *tb) stcFlush() { t.stcValid = false }

// Why "invalidate" matters: a cache is only safe to use if it's kept in
// sync with the thing it's caching. If a page table entry changes — its
// physical page reassigned, its protection tightened, or the whole mapping
// torn down — any cached copy of the *old* mapping must be thrown away
// (invalidated), or a later access could wrongly reuse stale information:
// reading/writing the wrong physical page, or being allowed an access that
// should now be denied. The three Invalidate* methods below are the only
// ways cached entries get cleared outside of being naturally overwritten
// by a newer translation, and every place elsewhere in this package (or in
// internal/cpu, internal/console) that changes a PTE is expected to call
// one of them — see translate.go's StorePTE and docs/PHASE-21.md's design
// notes for the full list of real call sites.

// InvalidateTB is the Go port of vm.c's invalidate_tb(): a full flush of
// every TB slot (mapping and protection both), used whenever the OS
// remaps P0BR/P1BR/etc. wholesale -- TBIA, and (once implemented) a
// context switch via LDPCTX/SVPCTX, see docs/PHASE-21.md's open item.
func (m *Memory) InvalidateTB() {
	m.tb.flushes++

	for i := range m.tb.entries {
		m.tb.entries[i] = tbEntry{}
	}

	m.tb.stcFlush()
}

// InvalidatePage is the Go port of vm.c's invalidate_page(addr): flushes
// only the one TB slot addr's region/page maps to -- TBIS, and any direct
// PTE write (SET PTE/SET PAGE) that must not leave a stale mapping cached
// for that specific page.
func (m *Memory) InvalidatePage(addr uint32) {
	region := (addr >> 30) & 0x3
	page := (addr & 0x3FFFFFFF) >> 9

	m.tb.entries[tbIndex(region, page)] = tbEntry{}
	m.tb.stcFlush()
}

// InvalidateProtection is the Go port of vm.c's invalidate_tb_prot(): every
// slot's cached mapping (page/paddr/code) is left alone, but its verified
// access mode is cleared to tbProtInvalid, forcing the next access to that
// page to re-run the protection check -- called whenever CurMod actually
// changes for a real mode transition (see docs/PHASE-21.md's design notes
// on why this port hooks the real mode-change sites directly rather than
// replicating read_psl_bits/write_psl_bits's call-site parity).
//
// This is a cheaper, more targeted invalidation than InvalidateTB: rather
// than discarding what physical page each virtual page maps to (which
// hasn't changed), it only forgets *which access modes were last verified
// permitted*, which can change independently whenever the processor's
// current privilege mode (CurMod) changes — see pte.go's Protection type
// for why the same page can be permitted for one mode and denied for
// another.
func (m *Memory) InvalidateProtection() {
	for i := range m.tb.entries {
		m.tb.entries[i].protMode = tbProtInvalid
	}

	m.tb.stcFlush()
}

// ResetTBCounters clears the try/hit/pflush counters without flushing any
// cached mapping, matching console_clear.c's CLEAR TB and
// console_vminit.c's VMINIT, both of which call invalidate_tb() (a real
// flush) immediately alongside this same reset -- tb_flush itself and the
// STC's own try/hit counters are deliberately left alone by both C call
// sites, replicated as-is (see docs/PHASE-21.md).
func (m *Memory) ResetTBCounters() {
	m.tb.tries = 0
	m.tb.hits = 0
	m.tb.pflushes = 0
}

// TBStats reports the 128-entry translation buffer's own counters,
// matching console_show.c's SHOW TB tb_try/tb_hit/tb_flush/tb_pflush
// report.
func (m *Memory) TBStats() (tries, hits, flushes, pflushes int64) {
	return m.tb.tries, m.tb.hits, m.tb.flushes, m.tb.pflushes
}

// STCStats reports the sequential translation cache's own counters,
// matching console_show.c's SHOW TB cached_page_try/cached_page_hit
// report.
func (m *Memory) STCStats() (tries, hits int64) {
	return m.tb.stcTries, m.tb.stcHits
}

// TBEntry is a read-only snapshot of one populated translation-buffer
// slot, for SHOW TB's own dump (the Go equivalent of dump_tb()'s per-slot
// printf). Only slots with a cached mapping (valid) are ever returned by
// TBSnapshot -- an entry that's merely protection-invalidated (ProtMode ==
// tbProtInvalid) still has a mapping and is included, matching dump_tb's
// own `if (tb[n].page == -1L) continue;` skip (which does not test
// prot_valid).
type TBEntry struct {
	Index    int
	VA       uint32
	PA       uint32
	Prot     Protection
	ProtMode AccessType // AccessRead, AccessWrite, or tbProtInvalid
}

// ProtValid reports whether this entry's cached protection check is still
// trusted (ProtMode != the invalid sentinel), matching dump_tb's own
// `tb[n].prot_valid != TB_INVALID` test.
func (e TBEntry) ProtValid() bool { return e.ProtMode != tbProtInvalid }

// TBSnapshot returns every currently-populated TB slot, in index order,
// matching dump_tb()'s own `for (n = 0; n < 128; n++)` scan. VA is
// reconstructed from the slot index's region bits and the entry's own
// stored (possibly aliased) page number, exactly as dump_tb computes it:
// `va = ((n>>5)&3)<<30 + (page<<9)`.
//
// `var out []TBEntry` declares out as a nil slice — a slice with no
// backing storage yet, length 0 — and `append` (used inside the loop
// below) grows it as needed, allocating storage lazily. If no slots are
// populated, TBSnapshot simply returns that nil slice, which callers can
// treat exactly like an empty one (ranging over it, or checking len() == 0,
// works fine either way); Go doesn't require an explicit empty slice
// literal here the way some languages require an explicit empty list.
func (m *Memory) TBSnapshot() []TBEntry {
	out := make([]TBEntry, 0, len(m.tb.entries))

	for n, e := range m.tb.entries {
		if !e.valid {
			continue
		}

		region := (uint32(n) >> 5) & 0x3
		va := region<<30 + e.page<<9

		out = append(out, TBEntry{
			Index:    n,
			VA:       va,
			PA:       e.paddr,
			Prot:     e.code,
			ProtMode: e.protMode,
		})
	}

	return out
}
