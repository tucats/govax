package vmheap

import (
	"errors"
	"fmt"
)

// Pages is where a zone's areas come from. A zone asks for n contiguous
// pages when it needs a new area (Lend) and gives each area back when
// it's deleted (Reclaim, perhaps several areas' worth that adjoin, as
// one call). Holds and Check are for Zone.Check.
//
// The image's page pool is one (*PagePool). A zone made by
// LIB$CREATE_VM_ZONE with get-page and free-page routines of the
// program's takes its pages from those instead (the VSI LIB$ manual's
// LIB$CREATE_VM_ZONE): a Borrowed.
type Pages interface {
	// Lend returns the address of n new pages for the zone.
	Lend(n uint32) (uint32, error)

	// Reclaim gives back the n pages starting at addr, which Lend gave.
	Reclaim(addr, n uint32) error

	// Holds reports whether every one of the n pages starting at addr
	// is lent to the zone.
	Holds(addr, n uint32) bool

	// Check verifies the source's own bookkeeping.
	Check() error
}

// ErrBadPages is pages a Borrowed's Get gave that a zone can't use: an
// address of 0 or not on a page boundary, a range past the end of the
// address space, or pages the zone has already.
var ErrBadPages = errors.New("vmheap: page routine returned unusable pages")

// Borrowed is a zone's pages from routines of the caller's: Get gets n
// pages and returns the first one's address, and Put gives n pages back.
// In govax they call the program's get-page and free-page routines
// (internal/corevms), each the way LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE
// are called.
//
// It keeps the pages lent as an address-ordered list of spans, adjoining
// ones merged, so that Holds can answer for an area that grew from
// several calls (ExtendArea), and Reclaim of it is a single Put of all
// its pages, as LIB$FREE_VM_PAGE allows.
type Borrowed struct {
	Get func(n uint32) (uint32, error)
	Put func(addr, n uint32) error

	lent []span
}

// Lend calls Get for n pages. Get's error is Lend's, and so is
// ErrBadPages for pages the zone can't use (which are neither kept nor
// given back: the zone never had them).
func (b *Borrowed) Lend(n uint32) (uint32, error) {
	if n == 0 {
		return 0, ErrBadSize
	}

	if n > maxPages {
		return 0, ErrNoMemory
	}

	addr, err := b.Get(n)
	if err != nil {
		return 0, err
	}

	s := span{addr, addr + n*PageSize}

	if addr == 0 || addr%PageSize != 0 || uint64(addr)+uint64(n)*PageSize > 1<<32 || overlapsAny(b.lent, s) {
		return 0, ErrBadPages
	}

	b.lent = insertSpan(b.lent, s)

	return addr, nil
}

// Reclaim forgets the n pages starting at addr and calls Put for them,
// returning Put's error: the pages are no longer the zone's either way.
// It returns ErrBadAddress, calling nothing, unless every one of them
// was lent.
func (b *Borrowed) Reclaim(addr, n uint32) error {
	s := span{addr, addr + n*PageSize}

	if !covered(b.lent, s) {
		return ErrBadAddress
	}

	b.lent = removeSpan(b.lent, s)

	return b.Put(addr, n)
}

// Holds reports whether every one of the n pages starting at addr is
// lent.
func (b *Borrowed) Holds(addr, n uint32) bool {
	return covered(b.lent, span{addr, addr + n*PageSize})
}

// Check verifies the list of pages lent: sorted, apart, page-aligned.
func (b *Borrowed) Check() error {
	if err := checkList("lent", b.lent); err != nil {
		return err
	}

	for _, s := range b.lent {
		if s.start%PageSize != 0 || s.end%PageSize != 0 {
			return fmt.Errorf("vmheap: lent pages %08X-%08X aren't page-aligned", s.start, s.end)
		}
	}

	return nil
}

// LentPages is how many pages the zone has from the routines.
func (b *Borrowed) LentPages() uint32 {
	return totalSize(b.lent) / PageSize
}
