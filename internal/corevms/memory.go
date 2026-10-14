package corevms

import "github.com/tucats/govax/internal/vmheap"

// The process heap: the blocks LIB$GET_VM, DECC$MALLOC, and the RTL's
// own routines (dynamic strings, FIND_FILE's and the timers' blocks, ...)
// allocate. Its mechanisms are internal/vmheap's: a page pool over the
// P0 region, and zones that divide the pool's pages into blocks
// (docs/PHASE-59 - LIB VM zones.md, subtask 2). The RTL has two zones of
// its own (vmzone.go): the default zone, which LIB$GET_VM's zone 0,
// DECC$MALLOC, and the RTL's own blocks use, and the string zone,
// dynamic strings' (subtask 5). All belong to the running image: image
// rundown forgets them (releaseHeap), as VMS deletes the P0 pages they
// were in.

// defaultZone is the default zone's attributes, the 5.0 LIB$ manual's
// Table 5-3: blocks a multiple of 8 bytes, quadword aligned, from areas
// of 128 pages; and LIB$M_VM_EXTEND_AREA, which the manual's
// LIB$SHOW_VM_ZONE example shows the default zone has. Its name is the
// one the manual's LIB$FIND_VM_ZONE example shows.
var defaultZone = ZoneSettings{
	Attributes: vmheap.Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 128, ExtendArea: true},
	Name:       "DEFAULT_ZONE",
}

// stringZone is the string zone's attributes. Neither manual gives
// them, nor its name; these are the default zone's, under a name of
// govax's (unconfirmed: a probe case).
var stringZone = ZoneSettings{
	Attributes: defaultZone.Attributes,
	Name:       "STRING_ZONE",
}

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

// releaseHeap forgets the image's heap, as VMS's image rundown deletes
// the P0 pages the heap was in: the next image is loaded from the bottom
// of P0 again, over where the blocks were.
func (env *Environment) releaseHeap() {
	env.heapPages, env.defaultHeap, env.stringHeap = nil, nil, nil
	env.zones, env.zoneControl, env.zoneSeq = nil, nil, 0
}

// allocate allocates size bytes from the RTL's zone z (the default or
// the string zone, neither of which tags or fills), returning the
// block's address, or 0 when P0 has no room. A size of 0 gets the
// smallest block, so that each call returns a distinct address.
func allocate(z *heapZone, err error, size uint32) uint32 {
	if err != nil {
		return 0
	}

	addr, err := z.zone.Allocate(max(size, 1))
	if err != nil {
		return 0
	}

	return addr
}

// release frees the block that starts at addr in the RTL's zone z,
// reporting false when none of z's blocks starts there (or z hasn't
// been made: it has no blocks).
func release(z *heapZone, addr uint32) bool {
	if z == nil {
		return false
	}

	_, err := z.zone.Free(addr)

	return err == nil
}

// blockSize returns the size of the block that starts at addr in the
// RTL's zone z, and whether one does.
func blockSize(z *heapZone, addr uint32) (uint32, bool) {
	if z == nil {
		return 0, false
	}

	return z.zone.BlockSize(addr)
}

// shimDeccMalloc is DECC$MALLOC: malloc(size) returns a new block's
// address, or 0 (NULL) when there's no room.
func shimDeccMalloc(env *Environment, argv []uint32) (uint32, error) {
	z, err := env.defaultZone()

	return allocate(z, err, optArg(argv, 0)), nil
}

// shimDeccFree is DECC$FREE: free(ptr) frees the block malloc returned.
// free(NULL) does nothing; a pointer that isn't a block's returns -1.
func shimDeccFree(env *Environment, argv []uint32) (uint32, error) {
	ptr := optArg(argv, 0)
	if ptr == 0 || release(env.defaultHeap, ptr) {
		return 0, nil
	}

	return 0xFFFFFFFF, nil
}

func registerMemoryShims(t *ShimTable) {
	t.Register(15, "DECC$MALLOC", shimDeccMalloc)
	t.Register(16, "DECC$FREE", shimDeccFree)
}
