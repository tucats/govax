package vm

import (
	"errors"
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
	tbTriesBefore, _, _, _ := mem.TBStats() //nolint:dogsled

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (repeat): %v", err)
	}

	stcTriesAfter, stcHitsAfter := mem.STCStats()
	if stcTriesAfter != stcTriesBefore+1 || stcHitsAfter != stcHitsBefore+1 {
		t.Errorf("STCStats() = tries=%d hits=%d, want tries=%d hits=%d (one new try, one new hit)",
			stcTriesAfter, stcHitsAfter, stcTriesBefore+1, stcHitsBefore+1)
	}

	tbTriesAfter, _, _, _ := mem.TBStats() //nolint:dogsled
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

	_, hitsBefore, _, _ := mem.TBStats() //nolint:dogsled

	got, err := mem.Translate(cpu, addrA, AccessRead)
	if err != nil {
		t.Fatalf("Translate A again: %v", err)
	}

	want := uint32(ptBase+2*pageSize) + 0x10
	if got != want {
		t.Errorf("Translate(A) = %#08x, want %#08x", got, want)
	}

	_, hitsAfter, _, _ := mem.TBStats() //nolint:dogsled
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

	_, _, flushesBefore, _ := mem.TBStats() //nolint:dogsled

	mem.InvalidateTB()

	if got := len(mem.TBSnapshot()); got != 0 {
		t.Errorf("TBSnapshot() len = %d after InvalidateTB, want 0", got)
	}

	_, _, flushesAfter, _ := mem.TBStats() //nolint:dogsled
	if flushesAfter != flushesBefore+1 {
		t.Errorf("TBStats() flushes = %d, want %d", flushesAfter, flushesBefore+1)
	}

	_, hitsBefore, _, _ := mem.TBStats() //nolint:dogsled

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate after InvalidateTB: %v", err)
	}

	_, hitsAfter, _, _ := mem.TBStats() //nolint:dogsled
	if hitsAfter != hitsBefore {
		t.Errorf("TBStats() hits changed across a post-flush translation, want a miss (full walk), not a hit")
	}
}

// TestInvalidateProcessTBKeepsSystemSlots checks the flush LDPCTX makes on
// a context switch: P0 and P1 translations go, S0 translations stay, the
// STC is emptied (so an S0 access then hits the TB, not the STC), and
// the flush is counted as a process flush, not a whole-buffer one.
func TestInvalidateProcessTBKeepsSystemSlots(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	p0Addr := uint32(2*pageSize) + 0x10    // P0 page 2
	s0Addr := uint32(sysBase+pageSize) + 4 // S0 page 1, P0's page table

	// Translating the P0 address walks P0's page table, which lives at an
	// S0 address, so both regions get TB entries.
	for _, addr := range []uint32{p0Addr, s0Addr} {
		if _, err := mem.Translate(cpu, addr, AccessRead); err != nil {
			t.Fatalf("Translate(%#x): %v", addr, err)
		}
	}

	regions := func() (process, system int) {
		for _, e := range mem.TBSnapshot() {
			if e.VA>>30 < 2 {
				process++
			} else {
				system++
			}
		}

		return process, system
	}

	if p, s := regions(); p == 0 || s == 0 {
		t.Fatalf("before: %d process and %d system entries, want some of each", p, s)
	}

	_, _, flushesBefore, pflushesBefore := mem.TBStats()

	mem.InvalidateProcessTB()

	if p, s := regions(); p != 0 || s == 0 {
		t.Errorf("after: %d process and %d system entries, want none and some", p, s)
	}

	if _, _, flushes, pflushes := mem.TBStats(); flushes != flushesBefore || pflushes != pflushesBefore+1 {
		t.Errorf("flushes, pflushes = %d, %d; want %d, %d", flushes, pflushes, flushesBefore, pflushesBefore+1)
	}

	stcTries, stcHits := mem.STCStats()
	_, tbHits, _, _ := mem.TBStats() //nolint:dogsled

	if _, err := mem.Translate(cpu, s0Addr, AccessRead); err != nil {
		t.Fatal(err)
	}

	if tries, hits := mem.STCStats(); tries != stcTries+1 || hits != stcHits {
		t.Errorf("STC tries, hits = %d, %d; want %d, %d (a miss: the STC was emptied)", tries, hits, stcTries+1, stcHits)
	}

	if _, hits, _, _ := mem.TBStats(); hits != tbHits+1 { //nolint:dogsled
		t.Errorf("TB hits = %d, want %d (the S0 entry survived)", hits, tbHits+1)
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
	_, hitsBefore, _, _ := mem.TBStats() //nolint:dogsled

	if _, err := mem.Translate(cpu, addrB, AccessRead); err != nil {
		t.Fatalf("Translate B again: %v", err)
	}

	_, hitsAfter, _, _ := mem.TBStats() //nolint:dogsled
	if hitsAfter != hitsBefore+1 {
		t.Errorf("TBStats() hits = %d, want %d (B's slot untouched by InvalidatePage(A))", hitsAfter, hitsBefore+1)
	}
}

// TestInvalidateProtectionKeepsMappingFlushesSTC covers what a mode
// change does to the caches since Study 1's R3 (docs/PERFORMANCE.md):
// InvalidateProtection empties the one-slot STC, which doesn't check the
// mode on a hit, but leaves the TB alone, since a TB entry checks the
// current mode on every hit. So the next access to the same page misses
// the STC and hits the TB. (vm.c's invalidate_tb_prot instead made the
// TB entry miss too, forcing a page-table walk.)
func TestInvalidateProtectionKeepsMappingFlushesSTC(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(2*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	mem.InvalidateProtection()

	if len(mem.TBSnapshot()) == 0 {
		t.Errorf("TBSnapshot() empty after InvalidateProtection, want the mapping to survive")
	}

	_, stcHitsBefore := mem.STCStats()
	_, hitsBefore, _, _ := mem.TBStats() //nolint:dogsled
	walksBefore := mem.translationCount

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate after InvalidateProtection: %v", err)
	}

	if _, stcHits := mem.STCStats(); stcHits != stcHitsBefore {
		t.Errorf("STCStats() hits = %d right after InvalidateProtection, want %d (the STC emptied)", stcHits, stcHitsBefore)
	}

	if _, hits, _, _ := mem.TBStats(); hits != hitsBefore+1 { //nolint:dogsled
		t.Errorf("TBStats() hits = %d, want %d (the TB entry kept, and hit)", hits, hitsBefore+1)
	}

	if mem.translationCount != walksBefore {
		t.Errorf("page-table walks = %d, want %d (no walk)", mem.translationCount, walksBefore)
	}
}

// TestTBReadThenWriteSetsModifyBitOnce covers Study 1's R3: a TB entry
// serves reads and writes alike, but a page's first write must still walk
// the page table, to set the modify (M) bit in its page table entry in
// memory. The page is read first (the walk caches it with M clear), then
// written (a walk, which sets M and refills the entry), then read and
// written again, both of which must hit the cache with no further walk.
func TestTBReadThenWriteSetsModifyBitOnce(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	pteAddr := uint32(p0PTPhys + 2*4)
	vaddr := uint32(2*pageSize) + 0x10
	other := uint32(3*pageSize) + 0x20 // a second page, to empty the STC

	modified := func() bool {
		raw, err := mem.readPhysLongword(pteAddr)
		if err != nil {
			t.Fatalf("read PTE: %v", err)
		}

		return PTE(raw).Modified()
	}

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (read): %v", err)
	}

	if modified() {
		t.Fatal("M bit set by a read")
	}

	// The read left this page in the STC, so the write meets the STC
	// first: the STC must not let it through, or M would never be set.
	walks := mem.translationCount

	if _, err := mem.Translate(cpu, vaddr, AccessWrite); err != nil {
		t.Fatalf("Translate (first write): %v", err)
	}

	if !modified() {
		t.Error("M bit not set by the first write after a read")
	}

	if mem.translationCount != walks+1 {
		t.Errorf("first write did %d page-table walks, want 1 (to set M)", mem.translationCount-walks)
	}

	// From here on the page is cached as written: a read and a write,
	// each after another page has taken the STC, both hit the TB.
	for _, access := range []AccessType{AccessRead, AccessWrite} {
		if _, err := mem.Translate(cpu, other, AccessRead); err != nil {
			t.Fatalf("Translate other: %v", err)
		}

		walks = mem.translationCount
		_, hits, _, _ := mem.TBStats() //nolint:dogsled

		got, err := mem.Translate(cpu, vaddr, access)
		if err != nil {
			t.Fatalf("Translate (access %d): %v", access, err)
		}

		if want := uint32(ptBase+2*pageSize) + 0x10; got != want {
			t.Errorf("Translate (access %d) = %#08x, want %#08x", access, got, want)
		}

		if mem.translationCount != walks {
			t.Errorf("access %d walked the page table, want a TB hit", access)
		}

		if _, h, _, _ := mem.TBStats(); h != hits+1 { //nolint:dogsled
			t.Errorf("access %d: TB hits = %d, want %d", access, h, hits+1)
		}
	}
}

// TestTBWriteThenReadHitsSTC: a page whose first access was a write is
// cached as writable, so a read of it right after is an STC hit (it used
// to miss, since vm.c's STC remembered only the access type it was filled
// for).
func TestTBWriteThenReadHitsSTC(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	vaddr := uint32(2*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessWrite); err != nil {
		t.Fatalf("Translate (write): %v", err)
	}

	_, stcHits := mem.STCStats()

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (read): %v", err)
	}

	if _, h := mem.STCStats(); h != stcHits+1 {
		t.Errorf("STCStats() hits = %d, want %d (a read after a write hits the STC)", h, stcHits+1)
	}
}

// TestTBHitChecksCurrentMode covers Study 1's R3: a TB entry is checked
// against the CPU's current mode on every hit, so a kernel-only page
// cached by a kernel-mode access faults when user mode touches it, with
// exactly the fault an uncached access gets (the hit falls through to the
// page-table walk, which reports it). The mode is changed here without
// InvalidateProtection, and the STC is emptied by touching another page,
// so it is the TB entry's own check that is tested.
func TestTBHitChecksCurrentMode(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	setProtection(t, mem, 1, ProtKW)

	vaddr := uint32(1*pageSize) + 0x10
	other := uint32(3*pageSize) + 0x20

	setMode := func(m vax.AccessMode) {
		psl := cpu.PSL()
		psl.SetCurMod(m)
		cpu.SetPSL(psl)
	}

	setMode(vax.Kernel)

	if _, err := mem.Translate(cpu, vaddr, AccessWrite); err != nil {
		t.Fatalf("Translate (kernel write): %v", err)
	}

	if _, err := mem.Translate(cpu, other, AccessRead); err != nil {
		t.Fatalf("Translate other: %v", err)
	}

	setMode(vax.User)

	for _, access := range []AccessType{AccessRead, AccessWrite} {
		_, err := mem.Translate(cpu, vaddr, access)

		var tf *TranslationFault
		if !errors.As(err, &tf) {
			t.Fatalf("user access %d: error = %v, want a *TranslationFault", access, err)
		}

		want := TranslationFault{Kind: ProtectionViolation, Addr: vaddr, Mask: byte(access)}
		if *tf != want {
			t.Errorf("user access %d: fault = %+v, want %+v", access, *tf, want)
		}

		// The fault cleared the entry, as vm.c clears a slot on every
		// fault; cache it again from kernel mode for the next round.
		setMode(vax.Kernel)

		if _, err := mem.Translate(cpu, vaddr, AccessWrite); err != nil {
			t.Fatalf("Translate (kernel write again): %v", err)
		}

		if _, err := mem.Translate(cpu, other, AccessRead); err != nil {
			t.Fatalf("Translate other: %v", err)
		}

		setMode(vax.User)
	}

	// Back in kernel mode, the cached entry serves the kernel again.
	setMode(vax.Kernel)

	_, hits, _, _ := mem.TBStats() //nolint:dogsled

	if _, err := mem.Translate(cpu, vaddr, AccessWrite); err != nil {
		t.Fatalf("Translate (kernel write, cached): %v", err)
	}

	if _, h, _, _ := mem.TBStats(); h != hits+1 { //nolint:dogsled
		t.Errorf("TB hits = %d, want %d (kernel write served from the cache)", h, hits+1)
	}
}

// TestTBWriteToReadOnlyPageFaults: a page every mode may read but none may
// write (PTE$K_UR), cached by a read, must not let a write through; the
// write faults just as it would uncached.
func TestTBWriteToReadOnlyPageFaults(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	setProtection(t, mem, 1, ProtUR)

	vaddr := uint32(1*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (read): %v", err)
	}

	_, err := mem.Translate(cpu, vaddr, AccessWrite)

	var tf *TranslationFault
	if !errors.As(err, &tf) || *tf != (TranslationFault{Kind: ProtectionViolation, Addr: vaddr, Mask: byte(AccessWrite)}) {
		t.Errorf("write error = %v, want a protection violation for a write", err)
	}
}

// TestProtGrantsMatchesAllows checks the precomputed protection table
// against Protection.allows, for every protection code, mode, and access.
func TestProtGrantsMatchesAllows(t *testing.T) {
	for code := range 16 {
		for mode := range 4 {
			g := protGrants[code][mode]

			for _, access := range []AccessType{AccessRead, AccessWrite} {
				want := Protection(code).allows(vax.AccessMode(mode), access)
				if got := tbGrant(access) < g; got != want {
					t.Errorf("code %d mode %d access %d: table says %v, allows says %v", code, mode, access, got, want)
				}
			}
		}
	}
}

// TestTBEntryPermits checks what SHOW TB reports for an entry: a writable
// page is reads-only until its modify bit is set, and a mode the code
// denies gets nothing.
func TestTBEntryPermits(t *testing.T) {
	for _, tc := range []struct {
		prot     Protection
		modified bool
		mode     vax.AccessMode
		want     AccessType
		ok       bool
	}{
		{ProtKW, true, vax.Kernel, AccessWrite, true},
		{ProtKW, false, vax.Kernel, AccessRead, true},
		{ProtKW, true, vax.User, AccessRead, false},
		{ProtUR, true, vax.User, AccessRead, true},
		{ProtURKW, true, vax.User, AccessRead, true},
		{ProtURKW, true, vax.Kernel, AccessWrite, true},
	} {
		got, ok := TBEntry{Prot: tc.prot, Modified: tc.modified}.Permits(tc.mode)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Permits(%v, M=%v, mode %d) = %d, %v, want %d, %v",
				tc.prot, tc.modified, tc.mode, got, ok, tc.want, tc.ok)
		}
	}
}

// setProtection rewrites the protection code of newTranslateFixture's P0
// page n directly in its page table, before anything has cached it.
func setProtection(t *testing.T, mem *Memory, n int, prot Protection) {
	t.Helper()

	addr := uint32(p0PTPhys + n*4)

	raw, err := mem.readPhysLongword(addr)
	if err != nil {
		t.Fatalf("read PTE %d: %v", n, err)
	}

	pte := PTE(raw)
	pte.SetProtection(prot)

	if err := mem.writePhysLongword(addr, uint32(pte)); err != nil {
		t.Fatalf("write PTE %d: %v", n, err)
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

	_, tbHits, _, _ := mem.TBStats() //nolint:dogsled
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

	_, hitsBefore, _, _ := mem.TBStats() //nolint:dogsled

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (TBDR clear): %v", err)
	}

	_, hitsAfter, _, _ := mem.TBStats() //nolint:dogsled
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

	_, _, flushesBefore, _ := mem.TBStats() //nolint:dogsled
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

// TestProbeTranslateChecksProbedMode: PROBEx (internal/cpu's emulProbe)
// lowers the CPU's mode to the probed one around a ProbeTranslate. With
// vm.c's TB, an entry cached by a kernel-mode read hit for any later read,
// whatever the mode, so probing a kernel-only page for user mode after the
// kernel had read it said "accessible". A TB entry now checks the current
// mode on every hit (Study 1, R3 in docs/PERFORMANCE.md), so the probe
// gets user mode's answer.
func TestProbeTranslateChecksProbedMode(t *testing.T) {
	cpu, mem := newTranslateFixture(t, 4)

	setProtection(t, mem, 1, ProtKW)

	vaddr := uint32(1*pageSize) + 0x10

	if _, err := mem.Translate(cpu, vaddr, AccessRead); err != nil {
		t.Fatalf("Translate (kernel read): %v", err)
	}

	psl := cpu.PSL()
	psl.SetCurMod(vax.User)
	cpu.SetPSL(psl)

	if _, err := mem.ProbeTranslate(cpu, vaddr, AccessRead); err == nil {
		t.Error("ProbeTranslate (user read of a kernel-only page) succeeded, want a protection violation")
	}
}
