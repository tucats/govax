package vm

import "github.com/tucats/govax/internal/vax"

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

// tbGrant is how much access a cached translation lets through without a
// page-table walk: nothing, reads only, or reads and writes. The values
// are ordered so that a single comparison answers "may this access use the
// cached translation?": an access is let through when
// tbGrant(access) < grant, since AccessRead is 0 and AccessWrite is 1.
//
//   - grantNone (0) lets nothing through: AccessRead (0) < 0 is false.
//   - grantRead (1) lets a read through (0 < 1) but not a write (1 < 1 is
//     false).
//   - grantWrite (2) lets both through.
//
// grantNone is deliberately the zero value, so an empty (zeroed) TB slot
// or a flushed STC grants nothing and can never produce a hit.
type tbGrant uint8

const (
	grantNone tbGrant = iota
	grantRead
	grantWrite
)

// protGrants is the protection check precomputed as a table (Study 1, R3
// in docs/PERFORMANCE.md): for each of the 16 protection codes and each of
// the 4 access modes (kernel, executive, supervisor, user), the widest
// access the code permits that mode. It holds exactly what
// Protection.allows computes (see pte.go), so a cached translation can be
// re-checked against whatever mode the CPU is in now with one array index
// instead of allows' arithmetic. A write permission always implies a read
// permission on the VAX (every protection code that lets a mode write
// lets it read too), which is why one ordered value per mode is enough.
//
// `var protGrants = func() ... { ... }()` runs the function once, when the
// package is initialized (before main starts), and stores its result; it
// is Go's idiom for a table computed at startup rather than typed out.
var protGrants = func() (t [16][4]tbGrant) {
	for code := range t {
		for mode := range t[code] {
			switch pr, m := Protection(code), vax.AccessMode(mode); {
			case pr.allows(m, AccessWrite):
				t[code][mode] = grantWrite
			case pr.allows(m, AccessRead):
				t[code][mode] = grantRead
			}
		}
	}

	return t
}()

// tbEntry is the Go equivalent of vm.c's struct TB: everything cached about
// one virtual page's mapping, taken from its page table entry (see pte.go).
// valid replaces the C source's page == -1 sentinel for "this slot is
// empty".
//
// Unlike vm.c's entry, which remembered the one access type (read or
// write) it was last checked for and missed whenever the other one came
// along, an entry here serves reads and writes alike (Study 1, R3 in
// docs/PERFORMANCE.md). A program that reads and then writes the same
// page, as nearly every loop over an array does, used to pay a full
// page-table walk at each switch between the two. To serve both, the
// entry caches the page's protection code and modify bit, and grants
// holds, for each of the four access modes, how much access a hit may
// let through:
//
//   - the protection code's own answer for that mode (protGrants), but
//   - only grantRead where it would be grantWrite while the modify bit is
//     still clear. The first write to a page has to set the bit in the
//     page table entry in memory (see translate's walk), so that write
//     must miss and walk; the walk refills the entry with modified set,
//     and later writes hit.
//
// Because the grant is looked up for the CPU's current mode on every hit,
// a change of mode needs no sweep over the entries: the next hit simply
// reads a different element of grants. (vm.c instead marked every entry
// "protection not verified" on each mode change; see InvalidateProtection.)
type tbEntry struct {
	valid    bool
	modified bool
	grants   [4]tbGrant
	page     uint32
	paddr    uint32
	code     Protection
}

// newTBEntry builds the cache entry for one page from what a page-table
// walk found: the page number, the physical address of the page's first
// byte, its protection code, and whether its modify bit is set.
func newTBEntry(page, paddr uint32, code Protection, modified bool) tbEntry {
	e := tbEntry{valid: true, modified: modified, page: page, paddr: paddr, code: code}

	// e.grants[mode] for each mode is copied from the table, then held
	// down to reads if the page hasn't been written yet. `e.grants =
	// protGrants[code&0xF]` copies the whole 4-element array at once (Go
	// arrays are values, so assignment copies them); &0xF keeps the
	// index inside the table even for a malformed code.
	e.grants = protGrants[code&0xF]

	if !modified {
		for m, g := range e.grants {
			if g == grantWrite {
				e.grants[m] = grantRead
			}
		}
	}

	return e
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

	// stcVPage/stcPPage/stcGrant are the STC's own one-slot cache of the
	// most recent translation, matching vm.c's cached_virtual_page/
	// cached_physical_page/cached_mbit. stcVPage and stcPPage are the
	// virtual and physical addresses of the page's first byte.
	//
	// stcGrant is how much access the slot lets through (see tbGrant): the
	// cached entry's grant for the access mode the CPU was in when the
	// slot was filled. grantNone means the slot is empty, replacing vm.c's
	// -1 sentinel on cached_virtual_page. Where vm.c's cached_mbit held
	// the one access type the slot was filled for, stcGrant lets a read
	// hit a slot a write filled, and a write hit a slot a read filled once
	// the page is known writable and modified.
	//
	// Unlike a TB entry, the slot doesn't re-check the mode on a hit, to
	// keep the hit as cheap as possible (it is consulted on every memory
	// access). So it must be emptied whenever the CPU's mode changes,
	// which is what InvalidateProtection does.
	stcVPage          uint32
	stcPPage          uint32
	stcGrant          tbGrant
	stcTries, stcHits int64
}

// stcFlush is the Go equivalent of vm.c's STC_FLUSH macro: it empties the
// one-slot cache, so the next translation can't hit it.
func (t *tb) stcFlush() { t.stcGrant = grantNone }

// stcFill makes the STC remember entry's translation for the virtual page
// starting at vpage, letting through what entry grants the access mode
// mode.
func (t *tb) stcFill(vpage uint32, entry *tbEntry, mode vax.AccessMode) {
	t.stcVPage = vpage
	t.stcPPage = entry.paddr
	t.stcGrant = entry.grants[mode&0x3]
}

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

// InvalidateProtection is called whenever the CPU's current access mode
// (CurMod) actually changes, at a real mode transition (see
// docs/PHASE-21.md's design notes on why this port hooks the real
// mode-change sites directly rather than replicating read_psl_bits/
// write_psl_bits's call-site parity). The same page can be permitted to
// one mode and denied to another (see pte.go's Protection type), so
// anything cached about what the *old* mode was allowed must be forgotten.
//
// It began as the Go port of vm.c's invalidate_tb_prot(), which marked
// every one of the 128 TB entries "protection not verified", forcing the
// next access to each page to walk the page table again. Since Study 1's
// R3 (docs/PERFORMANCE.md) a TB entry checks the current mode on every
// hit (see tbEntry), so the entries need nothing, and only the STC, which
// doesn't check the mode, is emptied. That turns a 128-entry sweep at
// every REI, CHMx, and AST delivery into one store, and pages stay cached
// across mode changes.
func (m *Memory) InvalidateProtection() {
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
// TBSnapshot, matching dump_tb's own `if (tb[n].page == -1L) continue;`
// skip.
//
// Modified is the page's modify bit as the entry cached it: until it is
// set, a write to the page doesn't hit the entry (see tbEntry).
type TBEntry struct {
	Index    int
	VA       uint32
	PA       uint32
	Prot     Protection
	Modified bool
}

// Permits reports the widest access a translation-buffer hit on this
// entry lets through for an access mode: AccessWrite (reads and writes),
// AccessRead (reads only), or ok false (neither, so every access from
// that mode walks the page table, and faults if the protection code
// really denies it). It replaces vm.c's per-entry "mode last verified",
// which SHOW TB used to show, now that an entry serves every mode (see
// tbEntry).
func (e TBEntry) Permits(mode vax.AccessMode) (access AccessType, ok bool) {
	switch newTBEntry(0, 0, e.Prot, e.Modified).grants[mode&0x3] {
	case grantWrite:
		return AccessWrite, true
	case grantRead:
		return AccessRead, true
	default:
		return AccessRead, false
	}
}

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
			Modified: e.modified,
		})
	}

	return out
}
