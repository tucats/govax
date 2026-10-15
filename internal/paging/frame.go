package paging

import (
	"errors"
	"fmt"
)

// PFN is a page frame number: the index of a 512-byte frame of physical
// memory, counted from 0.
type PFN uint32

// noFrame ends a list: it's the link of a list's first frame back, and of
// its last frame forward. (VMS uses PFN 0, which is why VMS can't put
// frame 0 on a list; govax can.)
const noFrame = ^PFN(0)

// State is where a frame is: VMS's page location code (the low three
// bits of the PFN STATE array element, the book's figure 14-11), for the
// three locations govax uses.
type State uint8

const (
	// Free: on the free page list. Its contents are safe on backing
	// store, or never need to be, so it can be reused without a write.
	Free State = iota

	// Modified: on the modified page list. Its contents must be written
	// to backing store before it can be reused.
	Modified

	// Active: in use. A process's valid page, a global page some process
	// maps, or a system page (one never on a list: S0, page tables).
	Active
)

func (s State) String() string {
	switch s {
	case Free:
		return "free"
	case Modified:
		return "modified"
	case Active:
		return "active"
	}

	return fmt.Sprintf("State(%d)", uint8(s))
}

// Type is what kind of page a frame holds: VMS's PFN TYPE array (the
// book's figure 14-12), for the kinds govax tells apart.
type Type uint8

const (
	// ProcessPage is a page of one process's P0 or P1 space. Its owner is
	// the process's handle and the page's virtual address.
	ProcessPage Type = iota

	// GlobalPage is a page of a global section, which several processes
	// may map at once. Its owner is the global page table entry, named by
	// a handle and index of the caller's choosing (the share count says
	// how many processes map it).
	GlobalPage

	// SystemPage is a frame that's never paged: S0's resident pages, page
	// tables, the P1 vector's frames (the phase doc's Decision 13).
	SystemPage
)

func (t Type) String() string {
	switch t {
	case ProcessPage:
		return "process"
	case GlobalPage:
		return "global"
	case SystemPage:
		return "system"
	}

	return fmt.Sprintf("Type(%d)", uint8(t))
}

// Owner names the virtual page a frame is tied to: VMS's PFN PTE array,
// which holds the address of the PTE that maps the frame. Handle is
// opaque to this package; for a process page it's the process's PID and
// VA the page's virtual address. Two tied frames of the same Type never
// have the same Owner (Check), since a virtual page lives in one frame at
// a time.
type Owner struct {
	Handle uint32
	VA     uint32
}

// BackingKind says where a frame's contents come from and go back to
// (the phase doc's Decision 8).
type BackingKind uint8

const (
	// NoBacking: a demand-zero page never written anywhere. Its contents
	// start as zeros.
	NoBacking BackingKind = iota

	// PageFile: the page file. Block is the page file block that holds
	// the page, or 0 if a block will be allocated when the page is first
	// written (as VMS's BAK holds a page file index with block 0).
	PageFile

	// SectionFile: a process section, an image file or a file $CRMPSC
	// mapped. Index is the process section table index (PSTX), Block the
	// page's block in the file.
	SectionFile

	// GlobalSection: a global section's page. Index is the global page
	// table index (GPTX).
	GlobalSection
)

func (k BackingKind) String() string {
	switch k {
	case NoBacking:
		return "none"
	case PageFile:
		return "page file"
	case SectionFile:
		return "section"
	case GlobalSection:
		return "global"
	}

	return fmt.Sprintf("BackingKind(%d)", uint8(k))
}

// Backing is a frame's backing store address: VMS's PFN BAK array, the
// PTE form the page's PTE returns to when its frame is reused.
type Backing struct {
	Kind  BackingKind
	Index uint32
	Block uint32
}

// Entry is one frame's record in the database (a copy: changing it
// changes nothing; Database.Entry returns it).
type Entry struct {
	// State is the frame's location: on a list, or active.
	State State

	// Type is the kind of page it holds.
	Type Type

	// Tied is whether the frame is tied to a virtual page, Owner. A frame
	// on a list keeps its tie until it's reused or its page is deleted;
	// a frame never used, or whose page was deleted, has none.
	Tied  bool
	Owner Owner

	// Backing is where the page's contents go back to.
	Backing Backing

	// RefCount is how many reasons there are to keep the frame active
	// (VMS's REFCNT): zero for a frame on a list. Being mapped by a
	// process (in its working set, from Phase 63) is one reason, and a
	// global page's share count, while nonzero, is one more.
	RefCount int

	// ShareCount, for a global page, is how many processes map it (VMS's
	// SHRCNT).
	ShareCount int

	// SavedModify is VMS's saved modify bit: the frame's contents differ
	// from its backing store, so it goes to the modified list, not the
	// free list, when it's released.
	SavedModify bool
}

// The errors the database reports. Each is a caller's mistake or memory
// running out; the database is left as it was.
var (
	// ErrNoFrame means the free list is empty: every frame is active or
	// waiting on the modified list to be written.
	ErrNoFrame = errors.New("paging: no free page frame")

	// ErrBadFrame is a PFN beyond the database's frames.
	ErrBadFrame = errors.New("paging: no such page frame")

	// ErrWrongState is an operation on a frame in the wrong place, such
	// as a release of a frame that isn't active or a reclaim of one that
	// isn't on a list.
	ErrWrongState = errors.New("paging: page frame in the wrong state")

	// ErrNotShared is an Unshare of a frame no process shares.
	ErrNotShared = errors.New("paging: global page frame not shared")
)

// TieBreaker is how the database breaks a frame's tie to its old virtual
// page when the frame is reused (Allocate): the caller sets the page's
// PTE back to its backing store form, which the arguments describe. The
// frame's record is cleared afterward.
type TieBreaker interface {
	BreakTie(pfn PFN, typ Type, owner Owner, back Backing)
}

// Counts is how many frames are in each place.
type Counts struct {
	Free     int
	Modified int

	// Active frames, by type.
	Process int
	Global  int
	System  int
}

// Active is how many frames are in use, of every type.
func (c Counts) Active() int {
	return c.Process + c.Global + c.System
}
