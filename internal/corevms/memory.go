package corevms

import "github.com/tucats/govax/internal/vmheap"

// The process heap: the blocks LIB$GET_VM, DECC$MALLOC, and the RTL's
// own routines (dynamic strings, FIND_FILE's and the timers' blocks, ...)
// allocate. Its mechanisms are internal/vmheap's: a page pool over the
// P0 region, and a zone that divides the pool's pages into blocks
// (docs/PHASE-59 - LIB VM zones.md, subtask 2). Both belong to the
// running image: image rundown forgets them (releaseHeap), as VMS
// deletes the P0 pages they were in.

// defaultZone is the default zone's attributes, the 5.0 LIB$ manual's
// Table 5-3: blocks a multiple of 8 bytes, quadword aligned, from areas
// of 128 pages; and LIB$M_VM_EXTEND_AREA, which the manual's
// LIB$SHOW_VM_ZONE example shows the default zone has.
var defaultZone = vmheap.Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 128, ExtendArea: true}

// p0Region is the process's P0 region as the heap's page pool sees it:
// it grows by $EXPREG's rule (expandP0).
type p0Region struct {
	env *Environment
}

func (r p0Region) End() uint32 {
	return p0Start(r.env.RegionSize[0])
}

func (r p0Region) Expand(pages uint32) (uint32, bool) {
	return r.env.expandP0(pages)
}

// p0Start is where pages added at P0's high-water mark begin: the
// mark rounded up to a page, and never page 0, which is no-access so
// that a null pointer faults.
func p0Start(mark uint32) uint32 {
	return max((mark+pageMask)&^pageMask, pageSize)
}

// p0Limit is the address P0 ends at: the end of its page table, which
// VMINIT (or $CREPRC) made P0Pages long, every page in it a demand-zero
// page until it's touched. An environment without an address space of
// its own (in tests) has the whole region.
func (env *Environment) p0Limit() uint64 {
	if env.Space != nil {
		return uint64(env.Space.P0Pages) * pageSize
	}

	return p1Base
}

// expandP0 adds pages pages at P0's high-water mark for the heap's page
// pool, returning the first one's address, or false when P0 has no room.
func (env *Environment) expandP0(pages uint32) (uint32, bool) {
	start := p0Start(env.RegionSize[0])
	end := uint64(start) + uint64(pages)*pageSize

	if end > env.p0Limit() {
		return 0, false
	}

	env.RegionSize[0] = uint32(end)

	return start, true
}

// pagePool returns the image's page pool, the processwide pool
// LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE manage and the heap's zones take
// their areas from, making it at the image's first use.
func (env *Environment) pagePool() *vmheap.PagePool {
	if env.heapPages == nil {
		env.heapPages = vmheap.NewPagePool(p0Region{env})
	}

	return env.heapPages
}

// heap returns the image's heap zone, making it at the image's first
// allocation.
func (env *Environment) heap() *vmheap.Zone {
	if env.heapZone == nil {
		zone, err := vmheap.NewZone(env.pagePool(), defaultZone)
		if err != nil {
			panic(err) // defaultZone's attributes are valid
		}

		env.heapZone = zone
	}

	return env.heapZone
}

// releaseHeap forgets the image's heap, as VMS's image rundown deletes
// the P0 pages the heap was in: the next image is loaded from the bottom
// of P0 again, over where the blocks were.
func (env *Environment) releaseHeap() {
	env.heapZone, env.heapPages = nil, nil
}

// allocate allocates size bytes from the heap, returning the block's
// address, or 0 when P0 has no room. A size of 0 gets the smallest
// block, so that each call returns a distinct address.
func (env *Environment) allocate(size uint32) uint32 {
	addr, err := env.heap().Allocate(max(size, 1))
	if err != nil {
		return 0
	}

	return addr
}

// release frees the heap block that starts at addr, reporting false when
// no block starts there.
func (env *Environment) release(addr uint32) bool {
	if env.heapZone == nil {
		return false
	}

	_, err := env.heapZone.Free(addr)

	return err == nil
}

// shimDeccMalloc is DECC$MALLOC: malloc(size) returns a new block's
// address, or 0 (NULL) when there's no room.
func shimDeccMalloc(env *Environment, argv []uint32) (uint32, error) {
	return env.allocate(optArg(argv, 0)), nil
}

// shimDeccFree is DECC$FREE: free(ptr) frees the block malloc returned.
// free(NULL) does nothing; a pointer that isn't a block's returns -1.
func shimDeccFree(env *Environment, argv []uint32) (uint32, error) {
	ptr := optArg(argv, 0)
	if ptr == 0 || env.release(ptr) {
		return 0, nil
	}

	return 0xFFFFFFFF, nil
}

func registerMemoryShims(t *ShimTable) {
	t.Register(15, "DECC$MALLOC", shimDeccMalloc)
	t.Register(16, "DECC$FREE", shimDeccFree)
}
