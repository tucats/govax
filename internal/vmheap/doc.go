// Package vmheap holds the mechanisms beneath the Run-Time Library's
// memory routines (Phase 59; docs/PHASE-59 - LIB VM zones.md): a page
// pool and a zone's block allocator, as plain Go with no CPU or memory of
// its own, so they can be tested on their own.
//
// The 5.0 LIB$ manual (chapter 5) describes three levels of memory
// allocation, each built on the one below:
//
//   - The memory management system services. $EXPREG adds pages at the
//     end of P0, the process's program region (the low half of the
//     process's address space, where images and their data live).
//   - The page routines, LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE, which keep
//     a processwide pool of 512-byte pages, reuse freed ones, and call
//     $EXPREG when none fit. That is PagePool here.
//   - The heap routines, LIB$GET_VM and LIB$FREE_VM, which hand out
//     blocks of any size from zones. A zone takes pages from the pool in
//     contiguous groups called areas and divides them into blocks. That
//     is Zone here.
//
// What this package does not know is where pages come from: a PagePool
// asks a Region (in govax, the process's P0 region, through $EXPREG's
// code in internal/corevms). Every address is a plain uint32, and
// nothing here reads or writes the memory it manages. The bookkeeping
// (which pages and blocks are free) is kept in Go, not in the free
// memory itself as VMS keeps it, so a program that writes past the end
// of its block damages only its own data, never the heap (the phase
// doc's Decision 2).
//
// Both levels keep their free space as address-ordered lists of spans
// that are merged with *both* neighbors when something is freed, so free
// space never stays split into pieces that could be one, and both take
// the lowest-addressed space that fits ("first fit"), which keeps a
// program's data together and its heap small. Check verifies those
// rules and the accounting; the tests run it after every operation.
package vmheap

import "errors"

// PageSize is the VAX page: 512 bytes.
const PageSize = 512

// The errors the pool and zones report. The RTL routines turn them into
// their condition values (LIB$_BADBLOSIZ, LIB$_BADBLOADR,
// LIB$_INSVIRMEM).
var (
	// ErrBadSize is a size of zero (a block or page count must be
	// positive).
	ErrBadSize = errors.New("vmheap: bad block size")

	// ErrBadAddress is an address that isn't the start of an allocated
	// block, or pages that aren't allocated (never, or freed already).
	ErrBadAddress = errors.New("vmheap: bad block address")

	// ErrNoMemory means the region can't supply the pages a request
	// needs. Nothing is allocated in part.
	ErrNoMemory = errors.New("vmheap: insufficient virtual memory")
)

// span is the address range [start, end): start is its first byte, end
// the byte after its last.
type span struct {
	start, end uint32
}

func (s span) size() uint32 {
	return s.end - s.start
}

// alignUp rounds n up to a multiple of align, a power of 2.
func alignUp(n, align uint32) uint32 {
	return (n + align - 1) &^ (align - 1)
}

// isPowerOf2 reports whether n is 1, 2, 4, 8, ...
func isPowerOf2(n uint32) bool {
	return n != 0 && n&(n-1) == 0
}
