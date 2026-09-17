package vm

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// TestTranslateSTCHit exercises the one-slot sequential translation cache:
// a second, identical (same page, same access mode) translation must hit
// the STC and report it in STCStats, without needing to consult the
// 128-entry TB at all -- the same page's TB slot is not even re-verified
// (tb_try does not increment on an STC hit, matching vm.c's own #if STC
// early return before the TB is ever touched).
func TestTranslateSTCHit(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(2*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	// The first Translate's own recursive PTE-address lookup (see
	// newTranslateFixture's doc comment) counts its own STC try, so
	// compare deltas across the repeat rather than absolute totals.
	stcTriesBefore, stcHitsBefore := mem.STCStats()
	tbTriesBefore, _, _, _ := mem.TBStats()

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (repeat): %v", err)
	}

	stcTriesAfter, stcHitsAfter := mem.STCStats()
	if stcTriesAfter != stcTriesBefore+1 || stcHitsAfter != stcHitsBefore+1 {
		t.Errorf("STCStats() = tries=%d hits=%d, want tries=%d hits=%d (one new try, one new hit)",
			stcTriesAfter, stcHitsAfter, stcTriesBefore+1, stcHitsBefore+1)
	}

	tbTriesAfter, _, _, _ := mem.TBStats()
	if tbTriesAfter != tbTriesBefore {
		t.Errorf("TBStats() tries changed from %d to %d on an STC hit, want unchanged", tbTriesBefore, tbTriesAfter)
	}
}

// TestTranslateTBHitAfterSTCMiss translates two different pages (so the
// STC's one slot, pointing at the second page, misses on a repeat of the
// first) and confirms the first page's own 128-entry TB slot is still
// cached and reported as a hit.
func TestTranslateTBHitAfterSTCMiss(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	addrA := uint32(2*pageSize) + 0x10
	addrB := uint32(3*pageSize) + 0x20

	if _, err := mem.Translate(cpu, addrA, AccessRead); err != nil {
		t.Fatalf("Translate A: %v", err)
	}

	if _, err := mem.Translate(cpu, addrB, AccessRead); err != nil {
		t.Fatalf("Translate B: %v", err)
	}

	_, hitsBefore, _, _ := mem.TBStats()

	got, err := mem.Translate(cpu, addrA, AccessRead)
	if err != nil {
		t.Fatalf("Translate A again: %v", err)
	}

	want := uint32(ptBase+2*pageSize) + 0x10
	if got != want {
		t.Errorf("Translate(A) = %#08x, want %#08x", got, want)
	}

	_, hitsAfter, _, _ := mem.TBStats()
	if hitsAfter != hitsBefore+1 {
		t.Errorf("TBStats() hits = %d, want %d (one new TB hit)", hitsAfter, hitsBefore+1)
	}
}

// TestInvalidateTBClearsEverything matches vm.c's invalidate_tb(): every
// slot's mapping is dropped (not just its protection), and the STC is
// flushed too -- a subsequent access to the same page must miss both.
func TestInvalidateTBClearsEverything(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(2*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(mem.TBSnapshot()) == 0 {
		t.Fatalf("TBSnapshot() empty before InvalidateTB, want at least one populated slot")
	}

	_, _, flushesBefore, _ := mem.TBStats()

	mem.InvalidateTB()

	if got := len(mem.TBSnapshot()); got != 0 {
		t.Errorf("TBSnapshot() len = %d after InvalidateTB, want 0", got)
	}

	_, _, flushesAfter, _ := mem.TBStats()
	if flushesAfter != flushesBefore+1 {
		t.Errorf("TBStats() flushes = %d, want %d", flushesAfter, flushesBefore+1)
	}

	_, hitsBefore, _, _ := mem.TBStats()

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate after InvalidateTB: %v", err)
	}

	_, hitsAfter, _, _ := mem.TBStats()
	if hitsAfter != hitsBefore {
		t.Errorf("TBStats() hits changed across a post-flush translation, want a miss (full walk), not a hit")
	}
}

// TestInvalidatePageClearsOnlyOneSlot matches vm.c's invalidate_page(addr)
// (TBIS): only the one slot addr's own region/page maps to is cleared,
// leaving every other cached mapping alone.
func TestInvalidatePageClearsOnlyOneSlot(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	addrA := uint32(2*pageSize) + 0x10
	addrB := uint32(3*pageSize) + 0x20

	if _, err := mem.Translate(cpu, addrA, AccessRead); err != nil {
		t.Fatalf("Translate A: %v", err)
	}

	if _, err := mem.Translate(cpu, addrB, AccessRead); err != nil {
		t.Fatalf("Translate B: %v", err)
	}

	before := len(mem.TBSnapshot())

	mem.InvalidatePage(addrA)

	after := len(mem.TBSnapshot())
	if after != before-1 {
		t.Errorf("TBSnapshot() len = %d after InvalidatePage(A), want %d (only A's slot dropped)", after, before-1)
	}

	// B's own slot must still be a hit.
	_, hitsBefore, _, _ := mem.TBStats()

	if _, err := mem.Translate(cpu, addrB, AccessRead); err != nil {
		t.Fatalf("Translate B again: %v", err)
	}

	_, hitsAfter, _, _ := mem.TBStats()
	if hitsAfter != hitsBefore+1 {
		t.Errorf("TBStats() hits = %d, want %d (B's slot untouched by InvalidatePage(A))", hitsAfter, hitsBefore+1)
	}
}

// TestInvalidateProtectionKeepsMappingButForcesRecheck matches vm.c's
// invalidate_tb_prot(): the cached page/paddr survive, but the next
// access must redo the protection check (i.e. it's reported as a TB miss,
// not a hit), and after that it's cached again as a hit.
func TestInvalidateProtectionKeepsMappingButForcesRecheck(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(2*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	mem.InvalidateProtection()

	if len(mem.TBSnapshot()) == 0 {
		t.Errorf("TBSnapshot() empty after InvalidateProtection, want the mapping to survive")
	}

	_, hitsBefore, _, _ := mem.TBStats()

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate after InvalidateProtection: %v", err)
	}

	_, hitsAfter, _, _ := mem.TBStats()
	if hitsAfter != hitsBefore {
		t.Errorf("TBStats() hits changed right after InvalidateProtection, want a miss (re-checked protection)")
	}

	// Now it's cached again -- the next identical access after this one
	// (via a different page first, to dodge the STC) is a real TB hit.
	other := uint32(3*pageSize) + 0x20
	if _, err := mem.Translate(cpu, other, AccessRead); err != nil {
		t.Fatalf("Translate other: %v", err)
	}

	_, hitsBefore2, _, _ := mem.TBStats()

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate again: %v", err)
	}

	_, hitsAfter2, _, _ := mem.TBStats()
	if hitsAfter2 != hitsBefore2+1 {
		t.Errorf("TBStats() hits = %d, want %d (re-cached after the forced recheck)", hitsAfter2, hitsBefore2+1)
	}
}

// TestProbeTranslateNeverHitsSTC matches vm.c's VM_NOSIGNAL flag
// interaction with the STC (see ProbeTranslate's own doc comment): even a
// repeated ProbeTranslate of the identical address/mode never reports an
// STC hit, though it can still hit the 128-entry TB.
func TestProbeTranslateNeverHitsSTC(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(2*pageSize) + 0x10

	if _, err := mem.ProbeTranslate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("ProbeTranslate: %v", err)
	}

	if _, err := mem.ProbeTranslate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("ProbeTranslate (repeat): %v", err)
	}

	if _, hits := mem.STCStats(); hits != 0 {
		t.Errorf("STCStats() hits = %d, want 0 -- ProbeTranslate must never hit the STC", hits)
	}

	_, tbHits, _, _ := mem.TBStats()
	if tbHits == 0 {
		t.Errorf("TBStats() hits = 0, want at least one TB hit from the repeated ProbeTranslate")
	}
}

// TestTBDRDisablesConsultationNotPopulation matches vm.c's own `if
// (!vax.TBDR)` gate: with TBDR nonzero the TB is never consulted (no
// try/hit increments and no early return), but a successful full walk
// still populates the slot -- so re-enabling TBDR afterward finds it
// already cached.
func TestTBDRDisablesConsultationNotPopulation(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	cpu.SetPR(vax.TBDR, 1) // disable TB consultation

	vaddr := uint32(2*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (TBDR set): %v", err)
	}

	tries, hits, _, _ := mem.TBStats()
	if tries != 0 || hits != 0 {
		t.Errorf("TBStats() = tries=%d hits=%d with TBDR set, want 0/0 (never consulted)", tries, hits)
	}

	if len(mem.TBSnapshot()) == 0 {
		t.Fatalf("TBSnapshot() empty, want the slot populated despite TBDR")
	}

	// A different page first, to dodge the STC and force a real TB
	// consultation.
	other := uint32(3*pageSize) + 0x20

	cpu.SetPR(vax.TBDR, 0) // re-enable

	if _, err := mem.Translate(cpu, other, AccessRead); err != nil {
		t.Fatalf("Translate other: %v", err)
	}

	_, hitsBefore, _, _ := mem.TBStats()

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (TBDR clear): %v", err)
	}

	_, hitsAfter, _, _ := mem.TBStats()
	if hitsAfter != hitsBefore+1 {
		t.Errorf("TBStats() hits = %d, want %d (the TBDR-disabled walk's own population survives)", hitsAfter, hitsBefore+1)
	}
}

// TestStorePTEInvalidatesItsOwnSlot matches console_set.c's setpte, whose
// invalidate_page(addr) call means a direct PTE write (SET PTE/SET PAGE)
// must not leave a stale cached mapping for that page.
func TestStorePTEInvalidatesItsOwnSlot(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(2 * pageSize)

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(mem.TBSnapshot()) == 0 {
		t.Fatalf("TBSnapshot() empty before StorePTE, want the slot populated")
	}

	_, _, pte, err := mem.LookupPTE(cpu, vaddr)
	if err != nil {
		t.Fatalf("LookupPTE: %v", err)
	}

	pte.SetPFN(pte.PFN() + 1) // repoint the page elsewhere

	// A different page first, to dodge the STC and force a real TB
	// consultation for vaddr's own slot below.
	other := uint32(3*pageSize) + 0x20
	if _, err := mem.Translate(cpu, other, AccessRead); err != nil {
		t.Fatalf("Translate other: %v", err)
	}

	if err := mem.StorePTE(cpu, vaddr, pte); err != nil {
		t.Fatalf("StorePTE: %v", err)
	}

	// If StorePTE's own InvalidatePage(vaddr) hadn't dropped vaddr's own
	// slot, this would return the stale, pre-StorePTE physical address
	// instead of re-walking to the new PFN.
	got, err := mem.Translate(cpu, vaddr, AccessRead)
	if err != nil {
		t.Fatalf("Translate after StorePTE: %v", err)
	}

	want := uint32(ptBase+3*pageSize) + 0 // PFN bumped by one page
	if got != want {
		t.Errorf("Translate() = %#08x after StorePTE, want %#08x (the new PFN, not a stale cached one)", got, want)
	}
}

// TestResetTBCountersLeavesFlushesAndSTCAlone matches console_clear.c's
// CLEAR TB and console_vminit.c's VMINIT: both reset only tries/hits/
// pflushes, deliberately leaving the flush counter and the STC's own
// counters untouched.
func TestResetTBCountersLeavesFlushesAndSTCAlone(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(2*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (repeat, STC hit): %v", err)
	}

	mem.InvalidateTB() // bumps flushes

	_, _, flushesBefore, _ := mem.TBStats()
	stcTriesBefore, stcHitsBefore := mem.STCStats()

	mem.ResetTBCounters()

	tries, hits, flushes, pflushes := mem.TBStats()
	if tries != 0 || hits != 0 || pflushes != 0 {
		t.Errorf("TBStats() after ResetTBCounters = tries=%d hits=%d pflushes=%d, want all 0", tries, hits, pflushes)
	}

	if flushes != flushesBefore {
		t.Errorf("TBStats() flushes = %d, want unchanged %d", flushes, flushesBefore)
	}

	stcTriesAfter, stcHitsAfter := mem.STCStats()
	if stcTriesAfter != stcTriesBefore || stcHitsAfter != stcHitsBefore {
		t.Errorf("STCStats() = %d/%d, want unchanged %d/%d", stcTriesAfter, stcHitsAfter, stcTriesBefore, stcHitsBefore)
	}
}
