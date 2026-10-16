package paging

import (
	"errors"
	"fmt"
)

// A process's working set is the set of its pages that are valid in its
// page tables: the ones it can touch without a page fault. VMS keeps it
// as the working set list, an array of entries in the process header,
// each naming one virtual page (*VAX/VMS Internals and Data Structures*,
// 14.1.2, figure 14-4). When the list is full and the process faults on
// another page, the pager picks an entry to reuse, removing that entry's
// page from the working set, by a "modified first-in/first-out" rule: the
// page that has been in the list longest goes first, with exceptions
// (the book's 15.4). WorkingSet is that list, with VMS's rules, kept in Go.
//
// The list has two parts, as VMS's has (govax has no third, permanently
// locked part: the kernel stack, the P1 pointer page, and the page tables
// are system pages, never in a working set; the phase doc's Decision 13):
//
//	slots[0 : dyn]          pages locked by $LKWSET or $LCKPAG
//	slots[dyn : len(slots)] the dynamic part, where replacement happens
//
// The dynamic part is a circular buffer. next (VMS's WSNEXT) is the slot
// filled most recently, so the slot after it, wrapping from the list's
// end back to dyn, holds the oldest page. A slot may be empty: its page
// was removed (deleted, purged, or the list was shrunk), or the list has
// never filled. next may also be dyn-1, a position just before the
// dynamic part, which means "start the next scan at dyn": the state of a
// new list, and of one whose newest entry was just locked.
//
// The list's length is normally its size, the working set limit (VMS's
// WSSIZE, the value $ADJWSL changes and JPI$_WSSIZE reports). It is
// longer only when nothing could be replaced (Add, below).
//
// The package knows nothing of PTEs or frames: an entry is a virtual page
// address and whether it's a global page. The caller turns the entries
// Add and Remove give back into transition PTEs and released frames.
type WorkingSet struct {
	slots []slot

	// dyn is the index of the first slot of the dynamic part (VMS's
	// WSDYN): every slot before it holds a locked page.
	dyn int

	// next is the slot filled most recently (VMS's WSNEXT), or dyn-1.
	next int

	// size is the working set limit (WSSIZE).
	size int

	bounds Bounds

	// where finds a page's slot by its address: VMS finds it through the
	// frame's working set list index (the PFN database's WSLX array),
	// which needs the frame; a map by address needs nothing.
	where map[uint32]int

	// process and global count the pages in the set by kind (VMS's
	// PCB$W_PPGCNT and PCB$W_GPGCNT); locked counts the locked slots.
	process, global int

	// peak is the most pages the set has held (JPI$_WSPEAK).
	peak int
}

// slot is one working set list entry (VMS's WSLE, the book's figure
// 14-5): the page's address and control bits. A slot with used false is
// available.
type slot struct {
	used bool
	Page
}

// Page is a working set list entry as the caller sees it.
type Page struct {
	// VA is the page's virtual address, with its byte offset (the low
	// nine bits) zero.
	VA uint32

	// Global is true for a page of a global section (counted in GPGCNT),
	// false for one of the process's own (PPGCNT).
	Global bool

	// Locks says how the page is locked; zero for a page in the dynamic
	// part.
	Locks Locks
}

// Locks are a page's lock bits.
type Locks uint8

const (
	// LockedInWorkingSet is $LKWSET's lock (VMS's WSL$V_WSLOCK): the page
	// is never replaced.
	LockedInWorkingSet Locks = 1 << iota

	// LockedInMemory is $LCKPAG's (WSL$V_PFNLOCK): the page is never
	// replaced, and stays in memory even when the process is outswapped.
	// It implies a lock in the working set (the book's 16.4.2.2), so a
	// page with either bit is in the locked part.
	LockedInMemory
)

// Bounds are the limits a working set's size moves within. They're the
// process's, kept here so the list can apply its own rules.
type Bounds struct {
	// Quota is WSQUOTA: below it, the set always takes a new page into an
	// available entry; at or above it, only while the free list has more
	// than GROWLIM pages (the book's 15.4.3).
	Quota int

	// Extent is WSEXTENT, the largest size Resize allows.
	Extent int

	// Minimum is MINWSCNT (VMS's WSFLUID): the fewest unlocked entries
	// the list keeps, so Resize never goes below it plus the locked
	// pages, and a lock in the working set that would leave fewer fails.
	Minimum int
}

// Conditions are what a choice of entry depends on outside the list. The
// zero value describes a system with no free pages to spare and no TB,
// in which nothing is protected.
type Conditions struct {
	// FreePages is the free list's length, and GrowLimit is GROWLIM: past
	// WSQUOTA, an available entry takes a new page only while FreePages >
	// GrowLimit.
	FreePages int
	GrowLimit int

	// TBSkip is TBSKIPWSL: how many entries still valid in the TB a scan
	// passes over before it reuses the next entry whatever its state (the
	// book's 15.4.4). Zero turns the check off.
	TBSkip int

	// ValidInTB, if set, says whether a page is still valid in the
	// translation buffer (a frequently used page, worth keeping).
	ValidInTB func(va uint32) bool

	// Protected, if set, says whether a page must not be chosen: one
	// translated during the current instruction (the phase doc's
	// Decision 6), which the instruction, or the Go routine running
	// inside it, may still be using.
	Protected func(va uint32) bool
}

// The errors a working set reports. The list is left as it was.
var (
	// ErrInWorkingSet is an Add of a page the set already holds.
	ErrInWorkingSet = errors.New("paging: page already in the working set")

	// ErrNotInWorkingSet is a lock or unlock of a page the set doesn't
	// hold.
	ErrNotInWorkingSet = errors.New("paging: page not in the working set")

	// ErrLockedSetFull is a lock in the working set that would leave
	// fewer than Bounds.Minimum unlocked entries (SS$_LKWSETFUL).
	ErrLockedSetFull = errors.New("paging: locked working set full")

	// ErrBadLock is a lock or unlock that names no lock, or both.
	ErrBadLock = errors.New("paging: not a single lock kind")
)

// pageMask clears a virtual address's byte offset.
const pageMask = ^uint32(0x1FF)

// NewWorkingSet makes an empty working set list of the given size (VMS
// sets a new process's to its default, DFWSCNT; the book's chapter 20),
// within its bounds as Resize keeps it.
func NewWorkingSet(size int, b Bounds) *WorkingSet {
	w := &WorkingSet{bounds: b, where: make(map[uint32]int)}
	w.size = w.clamp(size)
	w.slots = make([]slot, w.size)
	w.next = w.dyn - 1

	return w
}

// Size is the working set limit (WSSIZE).
func (w *WorkingSet) Size() int {
	return w.size
}

// Bounds returns the set's bounds.
func (w *WorkingSet) Bounds() Bounds {
	return w.bounds
}

// SetBounds changes the set's bounds (SET WORKING_SET's /QUOTA and
// /EXTENT). The size is left as it is: the new bounds apply from the
// next Resize, as image exit's reset.
func (w *WorkingSet) SetBounds(b Bounds) {
	w.bounds = b
}

// WorkingSetCounts are a working set's counts.
type WorkingSetCounts struct {
	// Process and Global are the pages in the set by kind (PPGCNT,
	// GPGCNT); their sum is the pages in use.
	Process, Global int

	// Locked is how many of them are locked.
	Locked int

	// Size is the limit (WSSIZE), Entries the list's length (more than
	// Size only while nothing could be replaced), and Peak the most pages
	// the set has held (WSPEAK).
	Size, Entries, Peak int
}

// Counts reports the set's counts.
func (w *WorkingSet) Counts() WorkingSetCounts {
	return WorkingSetCounts{
		Process: w.process,
		Global:  w.global,
		Locked:  w.dyn,
		Size:    w.size,
		Entries: len(w.slots),
		Peak:    w.peak,
	}
}

// InUse is how many pages the set holds (PPGCNT + GPGCNT).
func (w *WorkingSet) InUse() int {
	return w.process + w.global
}

// Lookup returns the entry for the page at va, if the set holds it.
func (w *WorkingSet) Lookup(va uint32) (Page, bool) {
	i, ok := w.where[va&pageMask]
	if !ok {
		return Page{}, false
	}

	return w.slots[i].Page, true
}

// Pages lists the set's entries: the locked ones first, then the dynamic
// part from the oldest page to the newest.
func (w *WorkingSet) Pages() []Page {
	out := make([]Page, 0, w.InUse())

	for _, i := range w.order() {
		if w.slots[i].used {
			out = append(out, w.slots[i].Page)
		}
	}

	return out
}

// order is every slot's index in Pages' order: the locked part, then the
// dynamic part starting after next.
func (w *WorkingSet) order() []int {
	out := make([]int, 0, len(w.slots))
	for i := range w.dyn {
		out = append(out, i)
	}

	n := len(w.slots) - w.dyn
	for k := range n {
		out = append(out, w.dyn+(w.next+1-w.dyn+k)%n)
	}

	return out
}

// Add puts the page at va in the working set: the pager does this for
// each page it makes valid. If the set has no room for it, Add removes
// the page the replacement rules choose, and returns it; the caller makes
// its PTE a transition PTE and releases its frame. Add can remove more
// than one page: a set left over its limit (below) is first brought back
// down to it, the oldest pages going first.
//
// The rules, in the book's order (15.4.1, 15.4.3, 15.4.4): the scan
// starts at the entry after next, the oldest. An available entry takes
// the page if the set isn't full and is below its quota, or the free list
// has more than GROWLIM pages; otherwise it's passed over. A protected
// page is passed over. A page still valid in the TB
// is passed over, up to TBSKIPWSL of them; then the next page whatever
// its state. The first entry not passed over is reused.
//
// When every entry is passed over (every page in the dynamic part is
// protected, or the dynamic part is empty), the list grows
// past its limit: the page takes a new entry after next, and nothing is
// removed. A Go routine inside a single instruction can touch more pages
// than the limit; the list comes back down at the next Add after the
// instruction (the phase doc's finding 10).
func (w *WorkingSet) Add(va uint32, global bool, c Conditions) ([]Page, error) {
	va &= pageMask
	if _, ok := w.where[va]; ok {
		return nil, fmt.Errorf("%w: %08X", ErrInWorkingSet, va)
	}

	removed := w.trim(w.size, c, true)

	i, ok := w.choose(c, true)
	if !ok {
		i = w.insertAfterNext()
	} else if s := w.slots[i]; s.used {
		removed = append(removed, s.Page)
		w.empty(i)
	}

	w.fill(i, Page{VA: va, Global: global})
	w.next = i

	return removed, nil
}

// Over reports whether the list is longer than its limit: it had to grow
// past it because nothing could be replaced (Add).
func (w *WorkingSet) Over() bool {
	return len(w.slots) > w.size
}

// Trim brings a list that grew past its limit back down to it, as the
// next Add would first, and returns the pages removed, oldest first: the
// caller makes their PTEs transition PTEs and releases their frames. The
// pages it passes over by c's rules stay, and the list with them, if
// nothing else can go.
func (w *WorkingSet) Trim(c Conditions) []Page {
	return w.trim(w.size, c, true)
}

// mayGrow says whether an available entry may take a new page (the
// book's 15.4.3, checks 1, 3, and 4).
func (w *WorkingSet) mayGrow(c Conditions) bool {
	used := w.InUse()
	if used >= w.size {
		return false
	}

	return used < w.bounds.Quota || c.FreePages > c.GrowLimit
}

// choose scans the dynamic part for the entry to fill: an available one,
// if growing is allowed and the growth checks pass, or the page to
// replace. It returns false when every entry is passed over.
func (w *WorkingSet) choose(c Conditions, growing bool) (int, bool) {
	n := len(w.slots) - w.dyn
	if n == 0 {
		return 0, false
	}

	grow := growing && w.mayGrow(c)

	// The book's first step: the entry at next itself, if it's available
	// (its page was removed since it was filled).
	if grow && w.next >= w.dyn && !w.slots[w.next].used {
		return w.next, true
	}

	i := w.next
	skipped := 0

	// Two passes at most: in the first, pages still valid in the TB may
	// be passed over; a second finds the first page not protected, should
	// the first pass have skipped them all.
	for k := range 2 * n {
		i++
		if i >= len(w.slots) {
			i = w.dyn
		}

		s := &w.slots[i]
		if !s.used {
			if grow {
				return i, true
			}

			continue
		}

		if c.Protected != nil && c.Protected(s.VA) {
			continue
		}

		if k < n && skipped < c.TBSkip && c.ValidInTB != nil && c.ValidInTB(s.VA) {
			skipped++

			continue
		}

		return i, true
	}

	return 0, false
}

// trim brings the list's length down to limit, dropping available
// entries first, then the oldest pages the rules allow (with the TB
// check only if tb is set), and returns the pages removed. It stops
// early if every page left is passed over.
func (w *WorkingSet) trim(limit int, c Conditions, tb bool) []Page {
	var removed []Page

	if !tb {
		c.TBSkip = 0
	}

	for len(w.slots) > limit {
		if i, ok := w.lastAvailable(); ok {
			w.drop(i)

			continue
		}

		i, ok := w.choose(c, false)
		if !ok {
			break
		}

		removed = append(removed, w.slots[i].Page)
		w.empty(i)
		w.drop(i)
	}

	return removed
}

// lastAvailable is the available entry nearest the list's end, if any.
func (w *WorkingSet) lastAvailable() (int, bool) {
	for i := len(w.slots) - 1; i >= w.dyn; i-- {
		if !w.slots[i].used {
			return i, true
		}
	}

	return 0, false
}

// insertAfterNext makes a new available entry just after next, the
// position of the newest page, and returns its index: how the list grows
// past its limit.
func (w *WorkingSet) insertAfterNext() int {
	i := w.next + 1

	w.slots = append(w.slots, slot{})
	copy(w.slots[i+1:], w.slots[i:])
	w.slots[i] = slot{}

	for j := i + 1; j < len(w.slots); j++ {
		if w.slots[j].used {
			w.where[w.slots[j].VA] = j
		}
	}

	return i
}

// drop takes the available entry at i out of the list, shortening it.
func (w *WorkingSet) drop(i int) {
	copy(w.slots[i:], w.slots[i+1:])
	w.slots = w.slots[:len(w.slots)-1]

	for j := i; j < len(w.slots); j++ {
		if w.slots[j].used {
			w.where[w.slots[j].VA] = j
		}
	}

	if w.next >= i {
		w.next--
	}

	if w.next < w.dyn-1 {
		w.next = w.dyn - 1
	}
}

// fill puts page p in the available entry i.
func (w *WorkingSet) fill(i int, p Page) {
	w.slots[i] = slot{used: true, Page: p}
	w.where[p.VA] = i

	if p.Global {
		w.global++
	} else {
		w.process++
	}

	w.peak = max(w.peak, w.InUse())
}

// empty makes entry i available, removing its page from the set.
func (w *WorkingSet) empty(i int) {
	p := w.slots[i].Page
	delete(w.where, p.VA)

	if p.Global {
		w.global--
	} else {
		w.process--
	}

	w.slots[i] = slot{}
}

// Remove takes the page at va out of the working set, locked or not, and
// returns its entry: as when the page is deleted or purged. Its entry
// becomes available, or, in a list over its limit, leaves the list. It
// returns false if the set doesn't hold the page.
func (w *WorkingSet) Remove(va uint32) (Page, bool) {
	i, ok := w.where[va&pageMask]
	if !ok {
		return Page{}, false
	}

	if i < w.dyn {
		i = w.unpin(i)
	}

	p := w.slots[i].Page
	w.empty(i)

	if len(w.slots) > w.size {
		w.drop(i)
	}

	return p, true
}

// RemoveRange takes every page from lo's to hi's out of the working set
// and returns their entries, oldest first: $PURGWS, which walks the list
// rather than the range (the phase doc's finding 4), since a range of all
// of P0 and P1 is millions of pages. With keepLocked, locked pages stay,
// and so do the pages c.Protected names (the TB isn't asked: a purge
// isn't a choice).
func (w *WorkingSet) RemoveRange(lo, hi uint32, keepLocked bool, c Conditions) []Page {
	lo &= pageMask
	hi &= pageMask

	var vas []uint32

	for _, p := range w.Pages() {
		if p.VA < lo || p.VA > hi || p.Locks != 0 && keepLocked {
			continue
		}

		if c.Protected == nil || !c.Protected(p.VA) {
			vas = append(vas, p.VA)
		}
	}

	removed := make([]Page, 0, len(vas))

	for _, va := range vas {
		p, _ := w.Remove(va)
		removed = append(removed, p)
	}

	return removed
}

// Lock locks the page at va, which the set must hold, with one kind of
// lock, and returns the locks it had before (the services' WASSET and
// WASCLR come from them). A page not yet locked moves out of the dynamic
// part, so it's never replaced: it's exchanged with the entry at dyn,
// which then moves past it (the book's 16.4.2.1).
//
// A lock in the working set fails with ErrLockedSetFull if it would leave
// fewer than Bounds.Minimum unlocked entries of the limit: govax's form
// of the book's rule that enough of the dynamic working set must remain
// (the phase doc's finding 6, unconfirmed). A lock in memory has no such
// check, as $LCKPAG has no such status.
func (w *WorkingSet) Lock(va uint32, kind Locks) (Locks, error) {
	if err := singleLock(kind); err != nil {
		return 0, err
	}

	i, ok := w.where[va&pageMask]
	if !ok {
		return 0, fmt.Errorf("%w: %08X", ErrNotInWorkingSet, va&pageMask)
	}

	before := w.slots[i].Locks
	if before == 0 {
		if kind == LockedInWorkingSet && w.size-(w.dyn+1) < w.bounds.Minimum {
			return 0, fmt.Errorf("%w: %d of %d entries locked", ErrLockedSetFull, w.dyn, w.size)
		}

		i = w.pin(i)
	}

	w.slots[i].Locks |= kind

	return before, nil
}

// Unlock removes one kind of lock from the page at va, which the set must
// hold, and returns the locks it had before. A page left with no lock
// goes back to the dynamic part, exchanged with the last locked entry
// (the book's 16.4.2.3).
func (w *WorkingSet) Unlock(va uint32, kind Locks) (Locks, error) {
	if err := singleLock(kind); err != nil {
		return 0, err
	}

	i, ok := w.where[va&pageMask]
	if !ok {
		return 0, fmt.Errorf("%w: %08X", ErrNotInWorkingSet, va&pageMask)
	}

	before := w.slots[i].Locks
	w.slots[i].Locks &^= kind

	if before != 0 && w.slots[i].Locks == 0 {
		w.unpin(i)
	}

	return before, nil
}

func singleLock(kind Locks) error {
	if kind != LockedInWorkingSet && kind != LockedInMemory {
		return fmt.Errorf("%w: %d", ErrBadLock, kind)
	}

	return nil
}

// pin moves the dynamic entry at i into the locked part and returns its
// new index, dyn's old value.
func (w *WorkingSet) pin(i int) int {
	at := w.dyn
	w.swap(i, at)
	w.dyn++

	// If next was the entry now locked (or the position before the
	// dynamic part), the next scan starts at the new dyn, the oldest
	// page. If it was i, the entry there now came from the old dyn, and
	// the scan still starts after it.
	if w.next < w.dyn {
		w.next = w.dyn - 1
	}

	return at
}

// unpin moves the locked entry at i back to the dynamic part, at its
// start, and returns its new index. If next was the position before the
// dynamic part, it now names this entry, which so counts as the newest.
func (w *WorkingSet) unpin(i int) int {
	at := w.dyn - 1
	w.swap(i, at)
	w.dyn--

	return at
}

// swap exchanges entries i and j.
func (w *WorkingSet) swap(i, j int) {
	if i == j {
		return
	}

	w.slots[i], w.slots[j] = w.slots[j], w.slots[i]

	for _, k := range []int{i, j} {
		if w.slots[k].used {
			w.where[w.slots[k].VA] = k
		}
	}
}

// Resize changes the working set limit to n, as $ADJWSL and image exit's
// reset do, and returns the pages removed. n is first kept within the
// bounds: no more than Extent, and no less than Minimum entries more than
// the locked pages (a limit beyond them is quietly clamped, as $ADJWSL
// does; the book's 16.4.1).
//
// A smaller limit removes pages while there are more than the new limit,
// oldest first (passing over protected pages, but not those valid in the
// TB: the removal isn't a choice). Then, whatever the change, the dynamic
// part is laid out afresh: its pages oldest to newest from dyn, then the
// available entries up to the limit, so the next scan reaches the
// available entries first and a larger limit is used at once. VMS leaves
// holes, adds entries at the list's end, and removes the pages beyond a
// new end (16.4.1); which pages go, and when, and where a larger list's
// room comes, are the probe's questions (unconfirmed).
func (w *WorkingSet) Resize(n int, c Conditions) []Page {
	w.size = w.clamp(n)
	c.TBSkip = 0

	var removed []Page

	for w.InUse() > w.size {
		i, ok := w.choose(c, false)
		if !ok {
			break
		}

		removed = append(removed, w.slots[i].Page)
		w.empty(i)
	}

	w.compact()

	return removed
}

// clamp keeps a limit within the bounds; the floor wins over the extent.
func (w *WorkingSet) clamp(n int) int {
	n = min(n, w.bounds.Extent)

	return max(n, w.dyn+w.bounds.Minimum)
}

// compact lays the dynamic part out oldest to newest from dyn, then
// available entries up to the limit (none, if the pages left are more).
func (w *WorkingSet) compact() {
	order := w.order()[w.dyn:]
	pages := make([]slot, 0, len(order))

	for _, i := range order {
		if w.slots[i].used {
			pages = append(pages, w.slots[i])
		}
	}

	length := max(w.size, w.dyn+len(pages))
	slots := make([]slot, length)
	copy(slots, w.slots[:w.dyn])
	copy(slots[w.dyn:], pages)
	w.slots = slots

	for i := w.dyn; i < len(w.slots); i++ {
		if w.slots[i].used {
			w.where[w.slots[i].VA] = i
		}
	}

	w.next = w.dyn + len(pages) - 1
}

// Check verifies the list's rules:
//
//   - dyn is within the list, and next is a dynamic entry or dyn-1;
//   - the list is at least its limit long, and longer only with every
//     entry in use;
//   - every locked entry holds a page with a lock, and no dynamic entry
//     holds a locked page;
//   - each page's address is page-aligned, in one entry, and found there
//     by its address;
//   - the counts match the entries, and the peak isn't below them.
//
// It returns the first rule broken, or nil.
func (w *WorkingSet) Check() error {
	n := len(w.slots)

	if w.dyn < 0 || w.dyn > n {
		return fmt.Errorf("paging: working set's locked part ends at %d of %d entries", w.dyn, n)
	}

	if w.next < w.dyn-1 || w.next >= max(n, w.dyn) {
		return fmt.Errorf("paging: working set's next entry %d, dynamic part %d to %d", w.next, w.dyn, n)
	}

	if n < w.size {
		return fmt.Errorf("paging: working set list has %d entries, its limit is %d", n, w.size)
	}

	var process, global int

	for i, s := range w.slots {
		if !s.used {
			if s.Page != (Page{}) {
				return fmt.Errorf("paging: available working set entry %d holds %+v", i, s.Page)
			}

			if i < w.dyn {
				return fmt.Errorf("paging: locked working set entry %d is available", i)
			}

			continue
		}

		if (s.Locks != 0) != (i < w.dyn) {
			return fmt.Errorf("paging: working set entry %d (locked part ends at %d) has locks %d", i, w.dyn, s.Locks)
		}

		if s.VA&^pageMask != 0 {
			return fmt.Errorf("paging: working set entry %d's address %08X isn't a page's", i, s.VA)
		}

		if j, ok := w.where[s.VA]; !ok || j != i {
			return fmt.Errorf("paging: working set page %08X is in entry %d but found at %d (%v)", s.VA, i, j, ok)
		}

		if s.Global {
			global++
		} else {
			process++
		}
	}

	if n > w.size && process+global != n {
		return fmt.Errorf("paging: working set list of %d entries over its limit of %d holds %d pages",
			n, w.size, process+global)
	}

	if len(w.where) != process+global {
		return fmt.Errorf("paging: working set finds %d pages but holds %d", len(w.where), process+global)
	}

	if process != w.process || global != w.global {
		return fmt.Errorf("paging: working set counts %d process and %d global pages, but holds %d and %d",
			w.process, w.global, process, global)
	}

	if w.peak < process+global {
		return fmt.Errorf("paging: working set's peak %d is below its %d pages", w.peak, process+global)
	}

	return nil
}
