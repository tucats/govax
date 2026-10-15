package paging

import "fmt"

// Database is the PFN database: one record per frame of physical memory,
// and the free and modified page lists that thread through it. The zero
// value isn't usable; New makes one.
type Database struct {
	// entries holds each frame's record, indexed by PFN (VMS keeps one
	// array per field; one array of records is the same thing in Go).
	entries []Entry

	// flink and blink are the lists' links: for a frame on a list, the
	// next and previous frames on it, or noFrame at an end.
	flink, blink []PFN

	free, modified pageList

	// active counts the active frames by Type, so Counts needn't walk
	// the database.
	active [SystemPage + 1]int

	breaker TieBreaker
}

// New makes a database of frames page frames, numbered 0 to frames-1,
// every one on the free list in PFN order and tied to nothing, so the
// first allocations take the lowest frames, as govax's allocator always
// has. The caller then reserves the frames it lays out itself (Reserve).
func New(frames int) *Database {
	d := &Database{
		entries:  make([]Entry, frames),
		flink:    make([]PFN, frames),
		blink:    make([]PFN, frames),
		free:     newList(),
		modified: newList(),
	}

	for pfn := range frames {
		d.pushTail(&d.free, PFN(pfn))
	}

	return d
}

// SetTieBreaker installs the routine that sets a page's PTE back to its
// backing store form when its frame is reused. With none, a reused
// frame's tie is simply dropped (which only tests should rely on).
func (d *Database) SetTieBreaker(t TieBreaker) {
	d.breaker = t
}

// Frames is how many frames the database describes.
func (d *Database) Frames() int {
	return len(d.entries)
}

// Entry returns a copy of the frame's record.
func (d *Database) Entry(pfn PFN) (Entry, error) {
	if err := d.valid(pfn); err != nil {
		return Entry{}, err
	}

	return d.entries[pfn], nil
}

// Counts reports how many frames are on each list and active.
func (d *Database) Counts() Counts {
	return Counts{
		Free:     d.free.count,
		Modified: d.modified.count,
		Process:  d.active[ProcessPage],
		Global:   d.active[GlobalPage],
		System:   d.active[SystemPage],
	}
}

// FreeCount is how many frames are on the free list: what the reclaimer
// compares with FREELIM and FREEGOAL.
func (d *Database) FreeCount() int {
	return d.free.count
}

// ModifiedCount is how many frames are on the modified list: what the
// modified page writer compares with MPW_HILIMIT and MPW_LOLIMIT.
func (d *Database) ModifiedCount() int {
	return d.modified.count
}

// Reserve takes one particular frame off the free list for a system page,
// one never paged and tied to nothing: how VMINIT claims the frames it
// lays out itself (S0, which it maps one-to-one onto the lowest frames).
// The frame must be free. Delete gives it back.
func (d *Database) Reserve(pfn PFN) error {
	if err := d.valid(pfn); err != nil {
		return err
	}

	e := &d.entries[pfn]
	if e.State != Free {
		return fmt.Errorf("%w: reserve of %s frame %d", ErrWrongState, e.State, pfn)
	}

	d.breakTie(pfn)
	d.unlink(&d.free, pfn)
	d.activate(pfn, Entry{Type: SystemPage, RefCount: 1})

	return nil
}

// Allocate takes the frame at the head of the free list for a new use:
// the virtual page owner names, of the given type, whose contents come
// from back. If the frame is still tied to an old virtual page, the tie
// is broken first (TieBreaker). The frame is active, with a reference
// count of 1 (its owner's), and its saved modify bit clear; its contents
// are whatever the frame held (the caller zeroes or reads it).
//
// Only the free list gives frames: one on the modified list must be
// written first. With the free list empty, Allocate returns ErrNoFrame.
func (d *Database) Allocate(typ Type, owner Owner, back Backing) (PFN, error) {
	pfn := d.free.head
	if pfn == noFrame {
		return 0, ErrNoFrame
	}

	d.breakTie(pfn)
	d.unlink(&d.free, pfn)
	d.activate(pfn, Entry{Type: typ, Tied: true, Owner: owner, Backing: back, RefCount: 1})

	return pfn, nil
}

// Hold adds a reason to keep an active frame active (VMS increments the
// reference count to lock a page down). Release gives it back.
func (d *Database) Hold(pfn PFN) error {
	e, err := d.activeEntry(pfn, "hold")
	if err != nil {
		return err
	}

	e.RefCount++

	return nil
}

// Release takes away one reason to keep an active frame active, as when
// its page leaves a working set. modified is the PTE's modify bit, which
// is ORed into the saved modify bit. When the last reason goes, the frame
// keeps its contents and its tie and goes to the tail of the modified
// list if its saved modify bit is set, else the free list (the book's
// 15.2.1.1, step 4). Release returns the frame's state afterward.
//
// The frame must be tied: an untied frame (a reserved one) has no page to
// fault it back, so Delete is how it's given up.
func (d *Database) Release(pfn PFN, modified bool) (State, error) {
	e, err := d.activeEntry(pfn, "release")
	if err != nil {
		return 0, err
	}

	if !e.Tied {
		return 0, fmt.Errorf("%w: release of untied frame %d", ErrWrongState, pfn)
	}

	if e.RefCount == 1 && e.ShareCount > 0 {
		return 0, fmt.Errorf("%w: release of shared frame %d's last reference", ErrWrongState, pfn)
	}

	e.SavedModify = e.SavedModify || modified
	d.dropReference(pfn)

	return d.entries[pfn].State, nil
}

// Share records one more process mapping a global page's frame. The
// first share adds a reason to keep the frame (VMS's SHRCNT going from 0
// to 1 increments REFCNT; the book's 14.2.7).
func (d *Database) Share(pfn PFN) error {
	e, err := d.activeEntry(pfn, "share")
	if err != nil {
		return err
	}

	if e.Type != GlobalPage {
		return fmt.Errorf("%w: share of %s frame %d", ErrWrongState, e.Type, pfn)
	}

	e.ShareCount++
	if e.ShareCount == 1 {
		e.RefCount++
	}

	return nil
}

// Unshare records one process fewer mapping a global page's frame;
// modified is that process's PTE's modify bit, ORed into the saved
// modify bit. When the last process lets go, the share's reason to keep
// the frame goes too (as Release). Unshare returns the frame's state
// afterward.
func (d *Database) Unshare(pfn PFN, modified bool) (State, error) {
	e, err := d.activeEntry(pfn, "unshare")
	if err != nil {
		return 0, err
	}

	if e.ShareCount == 0 {
		return 0, fmt.Errorf("%w: frame %d", ErrNotShared, pfn)
	}

	e.SavedModify = e.SavedModify || modified
	e.ShareCount--

	if e.ShareCount == 0 {
		d.dropReference(pfn)
	}

	return d.entries[pfn].State, nil
}

// Reclaim takes a frame off the free or modified list and makes it
// active again for its old virtual page: a soft fault (the book's
// 15.2.1.2). Its saved modify bit is kept, so a frame reclaimed from the
// modified list still goes back there when it's released. The frame must
// be on a list and tied.
func (d *Database) Reclaim(pfn PFN) error {
	if err := d.valid(pfn); err != nil {
		return err
	}

	e := &d.entries[pfn]

	list := d.listFor(e.State)
	if list == nil || !e.Tied {
		return fmt.Errorf("%w: reclaim of %s frame %d (tied %v)", ErrWrongState, e.State, pfn, e.Tied)
	}

	d.unlink(list, pfn)

	reclaimed := *e
	reclaimed.RefCount = 1
	d.activate(pfn, reclaimed)

	return nil
}

// Delete ends a frame's tie because its virtual page is deleted, and
// puts the frame at the head of the free list, to be reused first (the
// book's 14.2.3, the delete bit). The frame may be on a list, or active
// with only its owner's reference (no holds, no shares). Its contents are
// left as they are; a new owner zeroes or reads it.
func (d *Database) Delete(pfn PFN) error {
	if err := d.valid(pfn); err != nil {
		return err
	}

	e := &d.entries[pfn]

	switch e.State {
	case Active:
		if e.RefCount != 1 || e.ShareCount != 0 {
			return fmt.Errorf("%w: delete of frame %d with %d references, %d shares",
				ErrWrongState, pfn, e.RefCount, e.ShareCount)
		}

		d.active[e.Type]--

	default:
		d.unlink(d.listFor(e.State), pfn)
	}

	d.entries[pfn] = Entry{State: Free}
	d.pushHead(&d.free, pfn)

	return nil
}

// SetBacking changes a tied frame's backing store address: as when a
// copy-on-reference page's first fault makes the page file its backing
// store, or the modified page writer allocates its page file block.
func (d *Database) SetBacking(pfn PFN, back Backing) error {
	if err := d.valid(pfn); err != nil {
		return err
	}

	e := &d.entries[pfn]
	if !e.Tied {
		return fmt.Errorf("%w: backing for untied frame %d", ErrWrongState, pfn)
	}

	e.Backing = back

	return nil
}

// SetModified sets an active frame's saved modify bit, so it goes to the
// modified list when it's released whatever its PTE says: how a
// copy-on-reference or demand-zero page is "effectively modified" from
// its first fault (the book's 14.2.3).
func (d *Database) SetModified(pfn PFN) error {
	e, err := d.activeEntry(pfn, "set modified")
	if err != nil {
		return err
	}

	e.SavedModify = true

	return nil
}

// FirstModified is the frame at the head of the modified list, the one
// that has waited longest to be written, and false if the list is empty.
func (d *Database) FirstModified() (PFN, bool) {
	return d.modified.head, d.modified.head != noFrame
}

// Cleaned records that a frame on the modified list has been written to
// its backing store: its saved modify bit is cleared, and it moves to the
// tail of the free list, still tied, so a fault can take it back softly
// (the book's 15.2.1.1, step 8).
func (d *Database) Cleaned(pfn PFN) error {
	if err := d.valid(pfn); err != nil {
		return err
	}

	e := &d.entries[pfn]
	if e.State != Modified {
		return fmt.Errorf("%w: clean of %s frame %d", ErrWrongState, e.State, pfn)
	}

	d.unlink(&d.modified, pfn)
	e.SavedModify = false
	e.State = Free
	d.pushTail(&d.free, pfn)

	return nil
}

// valid checks that pfn names one of the database's frames.
func (d *Database) valid(pfn PFN) error {
	if int(pfn) >= len(d.entries) {
		return fmt.Errorf("%w: %d (of %d)", ErrBadFrame, pfn, len(d.entries))
	}

	return nil
}

// activeEntry is pfn's record if the frame is active; op names the
// operation for the error otherwise.
func (d *Database) activeEntry(pfn PFN, op string) (*Entry, error) {
	if err := d.valid(pfn); err != nil {
		return nil, err
	}

	e := &d.entries[pfn]
	if e.State != Active {
		return nil, fmt.Errorf("%w: %s of %s frame %d", ErrWrongState, op, e.State, pfn)
	}

	return e, nil
}

// activate makes pfn, off every list, active with the record e.
func (d *Database) activate(pfn PFN, e Entry) {
	e.State = Active
	d.entries[pfn] = e
	d.active[e.Type]++
}

// dropReference takes away one of an active frame's references; at the
// last, the frame goes to the tail of the list its saved modify bit
// chooses.
func (d *Database) dropReference(pfn PFN) {
	e := &d.entries[pfn]

	e.RefCount--
	if e.RefCount > 0 {
		return
	}

	d.active[e.Type]--

	if e.SavedModify {
		e.State = Modified
		d.pushTail(&d.modified, pfn)
	} else {
		e.State = Free
		d.pushTail(&d.free, pfn)
	}
}

// breakTie ends a free frame's tie to its old virtual page, telling the
// TieBreaker so the page's PTE can go back to its backing store form,
// and clears its record.
func (d *Database) breakTie(pfn PFN) {
	e := d.entries[pfn]
	if e.Tied && d.breaker != nil {
		d.breaker.BreakTie(pfn, e.Type, e.Owner, e.Backing)
	}

	d.entries[pfn] = Entry{State: e.State}
}
