package vm

// This is the Go port of vm.c's translation buffer cache (struct TB
// tb[128]) and its "sequential translation cache" (STC, the one-slot
// cached_virtual_page/cached_physical_page fast path) -- see
// docs/PHASE-21.md for the full design rationale, including why this
// reverses Phase 02/16's earlier decision not to port it.

// tbSize matches vm.c's struct TB tb[128]: 4 regions * 32 (5 low bits of
// page number) slots.
const tbSize = 128

// tbProtInvalid is the Go equivalent of vm.c's TB_INVALID (2): a sentinel
// stored in tbEntry.protMode distinct from AccessRead(0)/AccessWrite(1),
// meaning "this slot's mapping may still be valid, but its protection must
// be re-checked" -- set by InvalidateProtection without needing a full
// mapping flush.
const tbProtInvalid = AccessType(2)

// tbEntry is the Go equivalent of vm.c's struct TB. valid replaces the C
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
func tbIndex(region, page uint32) int {
	return int((region << 5) + (page & 0x1F))
}

// tb holds the Memory's translation-buffer cache state: the 128-entry
// array plus its counters, and the independent one-slot sequential
// translation cache (STC) plus its own counters.
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
func (m *Memory) TBSnapshot() []TBEntry {
	var out []TBEntry

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
