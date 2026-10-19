package paging

import (
	"errors"
	"math/rand"
	"testing"
)

// page is the address of page n.
func page(n int) uint32 {
	return uint32(n) * 0x200
}

// pageNumbers turns entries into their page numbers, for comparing.
func pageNumbers(pages []Page) []int {
	out := make([]int, len(pages))
	for i, p := range pages {
		out[i] = int(p.VA / 0x200)
	}

	return out
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// checkSet fails the test if the working set breaks a rule.
func checkSet(t *testing.T, w *WorkingSet) {
	t.Helper()

	if err := w.Check(); err != nil {
		t.Fatal(err)
	}
}

// add adds page n (a process page) and checks that the pages removed are
// want's.
func add(t *testing.T, w *WorkingSet, n int, c Conditions, want ...int) {
	t.Helper()

	removed, err := w.Add(page(n), false, c)
	if err != nil {
		t.Fatal(err)
	}

	checkSet(t, w)

	if got := pageNumbers(removed); !sameInts(got, want) {
		t.Fatalf("adding page %d removed %v, want %v", n, got, want)
	}
}

// inSet is the set's pages, oldest first.
func inSet(w *WorkingSet) []int {
	return pageNumbers(w.Pages())
}

// small is a set of 4 entries, quota 4, room to grow to 8, and a minimum
// of 2 unlocked entries.
func small() *WorkingSet {
	return NewWorkingSet(4, Bounds{Quota: 4, Extent: 8, Minimum: 2})
}

// Below the limit every page takes an available entry; at it, the oldest
// page goes first (the book's 15.4: first in, first out).
func TestWorkingSetFIFO(t *testing.T) {
	w := small()
	checkSet(t, w)

	for n := range 4 {
		add(t, w, n, Conditions{})
	}

	if c := w.Counts(); c.Process != 4 || c.Global != 0 || c.Size != 4 || c.Entries != 4 || c.Peak != 4 {
		t.Fatalf("counts %+v", c)
	}

	add(t, w, 4, Conditions{}, 0)
	add(t, w, 5, Conditions{}, 1)

	if got := inSet(w); !sameInts(got, []int{2, 3, 4, 5}) {
		t.Fatalf("set %v", got)
	}

	// The list wraps: page 6 takes the entry page 2 had.
	add(t, w, 6, Conditions{}, 2)
	add(t, w, 7, Conditions{}, 3)
	add(t, w, 8, Conditions{}, 4)

	if got := inSet(w); !sameInts(got, []int{5, 6, 7, 8}) {
		t.Fatalf("set after wrapping %v", got)
	}

	if _, err := w.Add(page(7)+0x44, false, Conditions{}); !errors.Is(err, ErrInWorkingSet) {
		t.Fatalf("second add of a page: %v", err)
	}

	if p, ok := w.Lookup(page(6) + 3); !ok || p != (Page{VA: page(6)}) {
		t.Fatalf("lookup of page 6: %+v %v", p, ok)
	}

	if _, ok := w.Lookup(page(2)); ok {
		t.Fatal("lookup of a removed page found it")
	}
}

// Global pages count apart from process pages, and the peak is the most
// pages held.
func TestWorkingSetCounts(t *testing.T) {
	w := small()

	if _, err := w.Add(page(1), true, Conditions{}); err != nil {
		t.Fatal(err)
	}

	add(t, w, 2, Conditions{})
	add(t, w, 3, Conditions{})

	if c := w.Counts(); c.Process != 2 || c.Global != 1 || w.InUse() != 3 || c.Peak != 3 {
		t.Fatalf("counts %+v", c)
	}

	w.Remove(page(1))
	w.Remove(page(2))
	checkSet(t, w)

	if c := w.Counts(); c.Process != 1 || c.Global != 0 || c.Peak != 3 {
		t.Fatalf("counts after removals %+v", c)
	}
}

// Past the quota, an available entry takes a new page only while the
// free list has more than GROWLIM pages (the book's 15.4.3).
func TestWorkingSetGrowthPastQuota(t *testing.T) {
	w := NewWorkingSet(6, Bounds{Quota: 3, Extent: 8, Minimum: 2})
	plenty := Conditions{FreePages: 64, GrowLimit: 63}
	scarce := Conditions{FreePages: 63, GrowLimit: 63}

	for n := range 3 {
		add(t, w, n, scarce)
	}

	// At the quota with the free list at GROWLIM: the oldest page goes.
	add(t, w, 3, scarce, 0)

	// With more than GROWLIM free, the set grows to its limit, but only
	// once the scan reaches the available entries: page 3 took page 0's
	// entry, so pages 1 and 2 come first (the book's 15.4.1: an entry in
	// use that the scan reaches is reused).
	add(t, w, 4, plenty, 1)
	add(t, w, 5, plenty, 2)
	add(t, w, 6, plenty)
	add(t, w, 7, plenty)
	add(t, w, 8, plenty)

	if got := inSet(w); !sameInts(got, []int{3, 4, 5, 6, 7, 8}) {
		t.Fatalf("set %v", got)
	}

	// Full is full, however many pages are free.
	add(t, w, 9, plenty, 3)
}

// AddAvailable fills available entries, taking no page out: never at the
// limit, past the quota only with more than GROWLIM pages free, and into
// an available entry even when Add's scan would reach a page first.
func TestWorkingSetAddAvailable(t *testing.T) {
	w := NewWorkingSet(6, Bounds{Quota: 3, Extent: 8, Minimum: 2})
	plenty := Conditions{FreePages: 64, GrowLimit: 63}
	scarce := Conditions{FreePages: 63, GrowLimit: 63}

	addAvailable := func(n int, c Conditions, want bool) {
		t.Helper()

		got, err := w.AddAvailable(page(n), false, c)
		if err != nil {
			t.Fatal(err)
		}

		checkSet(t, w)

		if got != want {
			t.Fatalf("AddAvailable of page %d: %v, want %v", n, got, want)
		}
	}

	addAvailable(0, scarce, true)
	addAvailable(1, scarce, true)
	addAvailable(2, scarce, true)

	// At the quota with the free list at GROWLIM: no room.
	addAvailable(3, scarce, false)

	// Pages in the way of Add's scan (page 3 takes page 0's entry, so
	// pages 1 and 2 come next): AddAvailable goes past them to the
	// available entry, as Add's scan does past a protected page, and the
	// page's place in the circular list is that entry's.
	add(t, w, 3, scarce, 0)
	addAvailable(4, plenty, true)

	if got := inSet(w); !sameInts(got, []int{3, 1, 2, 4}) {
		t.Fatalf("set %v", got)
	}

	addAvailable(5, plenty, true)
	addAvailable(6, plenty, true)

	// Full is full.
	addAvailable(7, plenty, false)

	if _, err := w.AddAvailable(page(6), false, plenty); !errors.Is(err, ErrInWorkingSet) {
		t.Fatalf("AddAvailable of a page in the set: %v", err)
	}

	if got := inSet(w); !sameInts(got, []int{3, 1, 2, 4, 5, 6}) {
		t.Fatalf("set %v", got)
	}
}

// The entries the scan passes over: up to TBSKIPWSL pages still valid in
// the TB, then the next whatever its state (the book's 15.4.4); and
// always a protected page.
func TestWorkingSetSkips(t *testing.T) {
	inTB := map[uint32]bool{page(0): true, page(1): true, page(2): true}
	c := Conditions{TBSkip: 2, ValidInTB: func(va uint32) bool { return inTB[va] }}

	w := small()
	for n := range 4 {
		add(t, w, n, c)
	}

	// Pages 0 and 1 are skipped; 2 is valid too, but the skips are used
	// up.
	add(t, w, 4, c, 2)

	// TBSKIPWSL 0 turns the check off.
	c.TBSkip = 0
	add(t, w, 5, c, 3)

	// Every page valid in the TB, skips to spare: the second pass reuses
	// the oldest.
	w = small()
	all := Conditions{TBSkip: 10, ValidInTB: func(uint32) bool { return true }}

	for n := range 4 {
		add(t, w, n, all)
	}

	add(t, w, 4, all, 0)

	// A protected page is passed over in either pass.
	protected := map[uint32]bool{page(1): true, page(2): true}
	all.Protected = func(va uint32) bool { return protected[va] }
	add(t, w, 5, all, 3)
}

// When every page is protected, the list grows past its limit for the
// new page, and comes back down at the next add, the oldest pages first
// (the phase doc's finding 10).
func TestWorkingSetOverflow(t *testing.T) {
	w := small()

	for n := range 4 {
		add(t, w, n, Conditions{})
	}

	everything := Conditions{Protected: func(uint32) bool { return true }}
	add(t, w, 4, everything)
	add(t, w, 5, everything)

	if c := w.Counts(); c.Entries != 6 || c.Size != 4 || c.Process != 6 || c.Peak != 6 {
		t.Fatalf("counts after overflowing %+v", c)
	}

	if got := inSet(w); !sameInts(got, []int{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("set %v", got)
	}

	// Page 5 is still in use by the instruction; the rest can go.
	nowFive := Conditions{Protected: func(va uint32) bool { return va == page(5) }}
	add(t, w, 6, nowFive, 0, 1, 2)

	if c := w.Counts(); c.Entries != 4 || c.Process != 4 {
		t.Fatalf("counts after coming back down %+v", c)
	}

	if got := inSet(w); !sameInts(got, []int{3, 4, 5, 6}) {
		t.Fatalf("set %v", got)
	}

	add(t, w, 7, Conditions{}, 3)

	// A page removed from a list over its limit takes its entry with it.
	add(t, w, 8, everything)
	w.Remove(page(5))
	checkSet(t, w)

	if c := w.Counts(); c.Entries != 4 || c.Process != 4 {
		t.Fatalf("counts after a removal over the limit %+v", c)
	}

	if got := inSet(w); !sameInts(got, []int{4, 6, 7, 8}) {
		t.Fatalf("set %v", got)
	}
}

// A list at its limit with available entries past its quota, every page
// in it protected and the free list at GROWLIM: no page can go and no
// entry may be grown into, so the page takes an available entry rather
// than a new one past the limit (a list over its limit has every entry in
// use). Found running the whole suite with paging forced on and 30-page
// working sets (docs/PHASE-66 - mapped image activation.md, subtask 7).
func TestWorkingSetNowhereElse(t *testing.T) {
	w := NewWorkingSet(6, Bounds{Quota: 3, Extent: 8, Minimum: 2})
	scarce := Conditions{FreePages: 63, GrowLimit: 63}
	everything := Conditions{FreePages: 63, GrowLimit: 63, Protected: func(uint32) bool { return true }}

	for n := range 3 {
		add(t, w, n, scarce)
	}

	add(t, w, 3, everything)
	add(t, w, 4, everything)

	if c := w.Counts(); c.Entries != 6 || c.Process != 5 || w.Over() {
		t.Fatalf("counts after adding with every page protected %+v", c)
	}

	// Only once every entry is in use does the list grow past its limit.
	add(t, w, 5, everything)
	add(t, w, 6, everything)

	if c := w.Counts(); c.Entries != 7 || c.Process != 7 || !w.Over() {
		t.Fatalf("counts after overflowing %+v", c)
	}
}

// Trim brings a list over its limit back down without adding a page,
// the oldest pages going first, passing over protected ones; a list at
// its limit is left alone.
func TestWorkingSetTrim(t *testing.T) {
	w := small()

	for n := range 4 {
		add(t, w, n, Conditions{})
	}

	if w.Over() || len(w.Trim(Conditions{})) != 0 {
		t.Fatal("a list at its limit is over it, or was trimmed")
	}

	everything := Conditions{Protected: func(uint32) bool { return true }}
	add(t, w, 4, everything)
	add(t, w, 5, everything)

	if !w.Over() {
		t.Fatal("a list of 6 entries isn't over its limit of 4")
	}

	if removed := w.Trim(everything); len(removed) != 0 || !w.Over() {
		t.Fatalf("trimmed %v with every page protected", pageNumbers(removed))
	}

	notOne := Conditions{Protected: func(va uint32) bool { return va == page(1) }}
	removed := w.Trim(notOne)
	checkSet(t, w)

	if got := pageNumbers(removed); !sameInts(got, []int{0, 2}) || w.Over() {
		t.Fatalf("trim removed %v (over %v), want [0 2]", got, w.Over())
	}

	if got := inSet(w); !sameInts(got, []int{1, 3, 4, 5}) {
		t.Fatalf("set %v", got)
	}
}

// A removed page leaves its entry available, and the scan uses it when
// it gets there (or at once, if it was the newest).
func TestWorkingSetRemove(t *testing.T) {
	w := small()
	for n := range 4 {
		add(t, w, n, Conditions{})
	}

	if p, ok := w.Remove(page(3) + 8); !ok || p.VA != page(3) {
		t.Fatalf("remove of page 3: %+v %v", p, ok)
	}

	checkSet(t, w)

	if _, ok := w.Remove(page(3)); ok {
		t.Fatal("second remove of page 3 found it")
	}

	// Page 3 was the newest: its entry, at next, is used first.
	add(t, w, 4, Conditions{})

	w.Remove(page(1))
	checkSet(t, w)

	// The scan reaches page 0 (the oldest) first, then 1's available
	// entry.
	add(t, w, 5, Conditions{}, 0)
	add(t, w, 6, Conditions{})

	if got := inSet(w); !sameInts(got, []int{2, 4, 5, 6}) {
		t.Fatalf("set %v", got)
	}
}

// Locked pages leave the dynamic part and are never replaced; a lock's
// previous bits tell WASSET from WASCLR.
func TestWorkingSetLock(t *testing.T) {
	w := NewWorkingSet(5, Bounds{Quota: 5, Extent: 8, Minimum: 2})
	for n := range 5 {
		add(t, w, n, Conditions{})
	}

	before, err := w.Lock(page(1), LockedInWorkingSet)
	if err != nil || before != 0 {
		t.Fatalf("first lock: %d %v", before, err)
	}

	checkSet(t, w)

	if before, err = w.Lock(page(1), LockedInWorkingSet); err != nil || before != LockedInWorkingSet {
		t.Fatalf("second lock: %d %v", before, err)
	}

	if before, err = w.Lock(page(1), LockedInMemory); err != nil || before != LockedInWorkingSet {
		t.Fatalf("memory lock of a locked page: %d %v", before, err)
	}

	checkSet(t, w)

	if p, _ := w.Lookup(page(1)); p.Locks != LockedInWorkingSet|LockedInMemory {
		t.Fatalf("locks %d", p.Locks)
	}

	// Page 1 moved to the locked part, exchanged with page 0, which was at
	// the dynamic part's start; the oldest unlocked page goes first.
	add(t, w, 5, Conditions{}, 0)
	add(t, w, 6, Conditions{}, 2)

	if got := inSet(w); !sameInts(got, []int{1, 3, 4, 5, 6}) {
		t.Fatalf("set %v", got)
	}

	if c := w.Counts(); c.Locked != 1 {
		t.Fatalf("counts %+v", c)
	}

	// Lock 3 of the 5 entries; a fourth would leave fewer than 2
	// unlocked.
	for _, n := range []int{3, 4} {
		if _, err := w.Lock(page(n), LockedInWorkingSet); err != nil {
			t.Fatal(err)
		}

		checkSet(t, w)
	}

	if _, err := w.Lock(page(5), LockedInWorkingSet); !errors.Is(err, ErrLockedSetFull) {
		t.Fatalf("lock past the minimum: %v", err)
	}

	// A lock in memory has no such limit.
	if _, err := w.Lock(page(5), LockedInMemory); err != nil {
		t.Fatal(err)
	}

	checkSet(t, w)

	// Only page 6 can be replaced now.
	add(t, w, 7, Conditions{}, 6)
	add(t, w, 8, Conditions{}, 7)

	if _, err := w.Lock(page(2), LockedInWorkingSet); !errors.Is(err, ErrNotInWorkingSet) {
		t.Fatalf("lock of a page not in the set: %v", err)
	}

	if _, err := w.Lock(page(8), LockedInWorkingSet|LockedInMemory); !errors.Is(err, ErrBadLock) {
		t.Fatalf("lock of both kinds: %v", err)
	}
}

// Unlocking returns a page to the dynamic part once it has no lock left,
// at the dynamic part's start, exchanged with the last locked entry (the
// book's 16.4.2.3).
func TestWorkingSetUnlock(t *testing.T) {
	w := NewWorkingSet(5, Bounds{Quota: 5, Extent: 8, Minimum: 2})
	for n := range 5 {
		add(t, w, n, Conditions{})
	}

	for _, n := range []int{0, 1} {
		if _, err := w.Lock(page(n), LockedInWorkingSet); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := w.Lock(page(1), LockedInMemory); err != nil {
		t.Fatal(err)
	}

	// Page 1 is still locked in memory: it stays in the locked part.
	before, err := w.Unlock(page(1), LockedInWorkingSet)
	if err != nil || before != LockedInWorkingSet|LockedInMemory {
		t.Fatalf("unlock: %d %v", before, err)
	}

	checkSet(t, w)

	if c := w.Counts(); c.Locked != 2 {
		t.Fatalf("counts %+v", c)
	}

	// An unlock of a page that isn't locked: no locks before.
	if before, err = w.Unlock(page(3), LockedInWorkingSet); err != nil || before != 0 {
		t.Fatalf("unlock of an unlocked page: %d %v", before, err)
	}

	if before, err = w.Unlock(page(0), LockedInWorkingSet); err != nil || before != LockedInWorkingSet {
		t.Fatalf("unlock of page 0: %d %v", before, err)
	}

	checkSet(t, w)

	if c := w.Counts(); c.Locked != 1 {
		t.Fatalf("counts %+v", c)
	}

	// Page 0 is back at the dynamic part's start, where the scan goes
	// next (it wraps there from page 4, the newest), so it goes first.
	add(t, w, 5, Conditions{}, 0)
	add(t, w, 6, Conditions{}, 2)
	add(t, w, 7, Conditions{}, 3)
	add(t, w, 8, Conditions{}, 4)

	if _, err := w.Unlock(page(2), LockedInMemory); !errors.Is(err, ErrNotInWorkingSet) {
		t.Fatalf("unlock of a page not in the set: %v", err)
	}
}

// Removing a locked page takes it out of the locked part.
func TestWorkingSetRemoveLocked(t *testing.T) {
	w := small()
	for n := range 4 {
		add(t, w, n, Conditions{})
	}

	if _, err := w.Lock(page(2), LockedInWorkingSet); err != nil {
		t.Fatal(err)
	}

	p, ok := w.Remove(page(2))
	if !ok || p.Locks != LockedInWorkingSet {
		t.Fatalf("remove of a locked page: %+v %v", p, ok)
	}

	checkSet(t, w)

	if c := w.Counts(); c.Locked != 0 || c.Process != 3 {
		t.Fatalf("counts %+v", c)
	}

	// Page 2's lock exchanged it with page 0, so page 0 sat in page 2's
	// entry, after page 1: page 2's emptied entry takes page 4, and page
	// 1 is then the oldest.
	add(t, w, 4, Conditions{})
	add(t, w, 5, Conditions{}, 1)
}

// RemoveRange is $PURGWS: every page in the range, locked ones too unless
// kept, oldest first.
func TestWorkingSetRemoveRange(t *testing.T) {
	w := NewWorkingSet(8, Bounds{Quota: 8, Extent: 8, Minimum: 2})
	for _, n := range []int{5, 1, 7, 3, 9, 2} {
		add(t, w, n, Conditions{})
	}

	if _, err := w.Lock(page(3), LockedInWorkingSet); err != nil {
		t.Fatal(err)
	}

	removed := w.RemoveRange(page(2), page(7)+0x1FF, true, Conditions{})
	checkSet(t, w)

	// Locking page 3 exchanged it with page 5, the first entry, so page 5
	// now comes after page 7.
	if got := pageNumbers(removed); !sameInts(got, []int{7, 5, 2}) {
		t.Fatalf("purge kept locked pages, removed %v", got)
	}

	// A protected page stays, whatever the range.
	protect9 := Conditions{Protected: func(va uint32) bool { return va == page(9) }}

	removed = w.RemoveRange(0, 0x7FFFFFFF, false, protect9)
	checkSet(t, w)

	if got := pageNumbers(removed); !sameInts(got, []int{3, 1}) {
		t.Fatalf("purge with page 9 protected removed %v", got)
	}

	removed = w.RemoveRange(0, 0x7FFFFFFF, false, Conditions{})
	checkSet(t, w)

	if got := pageNumbers(removed); !sameInts(got, []int{9}) {
		t.Fatalf("purge of everything removed %v", got)
	}

	if c := w.Counts(); c.Process != 0 || c.Locked != 0 || c.Entries != 8 {
		t.Fatalf("counts %+v", c)
	}

	for n := range 8 {
		add(t, w, n+20, Conditions{})
	}
}

// Resize keeps the limit within its bounds, and a smaller limit removes
// the oldest pages over it.
func TestWorkingSetResize(t *testing.T) {
	w := NewWorkingSet(6, Bounds{Quota: 6, Extent: 10, Minimum: 3})
	for n := range 6 {
		add(t, w, n, Conditions{})
	}

	add(t, w, 6, Conditions{}, 0)
	add(t, w, 7, Conditions{}, 1)

	// The list has wrapped; shrinking to 4 removes the oldest two and lays
	// the rest out from the start.
	removed := w.Resize(4, Conditions{})
	checkSet(t, w)

	if got := pageNumbers(removed); !sameInts(got, []int{2, 3}) {
		t.Fatalf("shrink removed %v", got)
	}

	if got := inSet(w); !sameInts(got, []int{4, 5, 6, 7}) || w.Size() != 4 || w.Counts().Entries != 4 {
		t.Fatalf("set %v, size %d, counts %+v", got, w.Size(), w.Counts())
	}

	add(t, w, 8, Conditions{}, 4)

	// Clamped: no more than the extent, no less than the minimum plus
	// the locked pages.
	if removed = w.Resize(100, Conditions{}); removed != nil || w.Size() != 10 {
		t.Fatalf("grow past the extent: size %d, removed %v", w.Size(), removed)
	}

	checkSet(t, w)

	// The new entries come after the newest page; the set grows into
	// them at once.
	// (Past the quota of 6, only while more than GROWLIM pages are free.)
	plenty := Conditions{FreePages: 64, GrowLimit: 63}
	for n := 9; n < 15; n++ {
		add(t, w, n, plenty)
	}

	add(t, w, 15, plenty, 5)

	if _, err := w.Lock(page(9), LockedInWorkingSet); err != nil {
		t.Fatal(err)
	}

	removed = w.Resize(0, Conditions{})
	checkSet(t, w)

	if w.Size() != 4 {
		t.Fatalf("shrink to nothing: size %d", w.Size())
	}

	// Locking page 9 moved page 15, the newest, out of the first entry
	// to page 9's, between pages 8 and 10, and the scan starts after the
	// locked part, so page 15 goes with the old pages.
	if got := pageNumbers(removed); !sameInts(got, []int{6, 7, 8, 15, 10, 11}) {
		t.Fatalf("shrink removed %v", got)
	}

	if got := inSet(w); !sameInts(got, []int{9, 12, 13, 14}) {
		t.Fatalf("set %v", got)
	}

	// A shrink that meets protected pages stops over the limit; the next
	// add brings it down.
	w = NewWorkingSet(6, Bounds{Quota: 6, Extent: 10, Minimum: 3})
	for n := range 6 {
		add(t, w, n, Conditions{})
	}

	removed = w.Resize(3, Conditions{Protected: func(va uint32) bool { return va <= page(3) }})
	checkSet(t, w)

	if got := pageNumbers(removed); !sameInts(got, []int{4, 5}) || w.Counts().Entries != 4 {
		t.Fatalf("protected shrink removed %v, counts %+v", got, w.Counts())
	}

	add(t, w, 6, Conditions{}, 0, 1)
}

// SetBounds changes the bounds without changing the size.
func TestWorkingSetBounds(t *testing.T) {
	w := small()
	b := Bounds{Quota: 2, Extent: 5, Minimum: 1}
	w.SetBounds(b)

	if w.Bounds() != b || w.Size() != 4 {
		t.Fatalf("bounds %+v, size %d", w.Bounds(), w.Size())
	}

	add(t, w, 0, Conditions{})
	add(t, w, 1, Conditions{})
	add(t, w, 2, Conditions{}, 0)

	w.Resize(9, Conditions{})

	if w.Size() != 5 {
		t.Fatalf("size %d after growing past the new extent", w.Size())
	}

	// The floor wins over an extent below it.
	w = NewWorkingSet(4, Bounds{Quota: 4, Extent: 2, Minimum: 3})
	if w.Size() != 3 {
		t.Fatalf("size %d with the extent below the minimum", w.Size())
	}
}

// Random operations leave the set whole, and it never holds more pages
// than its limit except while pages are protected.
func TestWorkingSetRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(63))
	w := NewWorkingSet(12, Bounds{Quota: 10, Extent: 24, Minimum: 4})

	protected := map[uint32]bool{}
	c := Conditions{
		FreePages: 100,
		GrowLimit: 63,
		TBSkip:    3,
		ValidInTB: func(va uint32) bool { return (va/0x200)%3 == 0 },
		Protected: func(va uint32) bool { return protected[va] },
	}

	for step := range 20000 {
		va := page(rng.Intn(48))
		c.FreePages = rng.Intn(128)

		var err error

		switch op := rng.Intn(20); {
		case op < 10:
			if _, ok := w.Lookup(va); !ok {
				_, err = w.Add(va, rng.Intn(4) == 0, c)
			}

		case op < 12:
			w.Remove(va)

		case op < 14:
			kind := LockedInWorkingSet
			if rng.Intn(3) == 0 {
				kind = LockedInMemory
			}

			_, err = w.Lock(va, kind)

		case op < 16:
			_, err = w.Unlock(va, Locks(1+rng.Intn(2)))

		case op < 17:
			w.Resize(w.Size()+rng.Intn(9)-4, c)

		case op < 18:
			lo := page(rng.Intn(48))
			w.RemoveRange(lo, lo+page(rng.Intn(8)), rng.Intn(2) == 0, Conditions{})

		case op < 19:
			// A new instruction: a few pages in use.
			clear(protected)

			for range rng.Intn(3) {
				protected[page(rng.Intn(48))] = true
			}

		default:
			clear(protected)
		}

		if err != nil && !errors.Is(err, ErrNotInWorkingSet) && !errors.Is(err, ErrLockedSetFull) {
			t.Fatalf("step %d: %v", step, err)
		}

		if err := w.Check(); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}

		if w.Counts().Entries > max(w.Size(), w.InUse()) {
			t.Fatalf("step %d: %d entries for a limit of %d", step, w.Counts().Entries, w.Size())
		}
	}
}

// Shrink takes the oldest pages out down to a count, passing over locked
// and protected pages but not those valid in the TB, and leaves the
// limit as it was; the emptied entries take new pages past the quota
// only while the free list has more than GROWLIM pages.
func TestWorkingSetShrink(t *testing.T) {
	w := NewWorkingSet(8, Bounds{Quota: 4, Extent: 8, Minimum: 2})
	plenty := Conditions{FreePages: 64, GrowLimit: 63}
	scarce := Conditions{FreePages: 10, GrowLimit: 63}

	for n := range 8 {
		add(t, w, n, plenty)
	}

	if _, err := w.Lock(page(2), LockedInWorkingSet); err != nil {
		t.Fatal(err)
	}

	checkSet(t, w)

	c := scarce
	c.TBSkip = 8
	c.ValidInTB = func(uint32) bool { return true }
	c.Protected = func(va uint32) bool { return va == page(0) }

	removed := w.Shrink(4, c)
	checkSet(t, w)

	if got := pageNumbers(removed); !sameInts(got, []int{1, 3, 4, 5}) {
		t.Fatalf("Shrink removed %v, want [1 3 4 5]", got)
	}

	if got := w.Counts(); got.Size != 8 || w.InUse() != 4 {
		t.Fatalf("after Shrink: counts %+v, %d in use", got, w.InUse())
	}

	// Below the free list's GROWLIM, the set stays at its quota: a new
	// page replaces the oldest unprotected one rather than taking an
	// available entry.
	add(t, w, 8, scarce, 0)

	// With pages to spare, it grows into the emptied entries.
	add(t, w, 9, plenty)

	if w.InUse() != 5 {
		t.Fatalf("after growing: %d in use, want 5", w.InUse())
	}

	// Nothing to take: a count above the pages held removes nothing.
	if removed := w.Shrink(6, scarce); len(removed) != 0 {
		t.Fatalf("Shrink to 6 removed %v", pageNumbers(removed))
	}
}
