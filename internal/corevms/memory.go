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
// pool, returning the first one's address, or false when P0 has no room
// or, with paging on, the job's paging file quota hasn't enough pages
// left for them.
func (env *Environment) expandP0(pages uint32) (uint32, bool) {
	return env.growP0(pages, false)
}

// growP0 is expandP0's work. The new pages are charged against the
// paging file quota, all or none (pgflquota.go): the heap can't use part
// of an expansion. With force set they're charged even past the quota,
// as the system zones' control page is (vmzone.go): on VMS the RTL's own.
func (env *Environment) growP0(pages uint32, force bool) (uint32, bool) {
	start := p0Start(env.RegionSize[0])
	end := uint64(start) + uint64(pages)*pageSize

	if end > env.p0Limit() {
		return 0, false
	}

	if force {
		for i := range pages {
			env.chargePage(start+i*pageSize, true)
		}
	} else if !env.chargePages(start, pages) {
		return 0, false
	}

	env.RegionSize[0] = uint32(end)

	return start, true
}

// poolChunk is the fewest pages the page pool takes from P0 when it
// grows, VMS 7.1's 128 (testdata/lib59).
const poolChunk = 128

// pagePool returns the image's page pool, the processwide pool
// LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE manage and the heap's zones take
// their areas from, making it at the image's first use.
func (env *Environment) pagePool() *vmheap.PagePool {
	if env.heapPages == nil {
		env.heapPages = vmheap.NewPagePool(p0Region{env})
		env.heapPages.Chunk = poolChunk
	}

	return env.heapPages
}

// releaseHeap forgets the image's heap, as VMS's image rundown deletes
// the P0 pages the heap was in: the next image is loaded from the bottom
// of P0 again, over where the blocks were.
func (env *Environment) releaseHeap() {
	env.heapPages, env.defaultHeap, env.stringHeap = nil, nil, nil
	env.zones, env.zoneControl, env.vmCounts = nil, nil, VMCounts{}
	env.cHeap = nil
}

// allocate allocates size bytes from the RTL's zone z (the default or
// the string zone), returning the block's address, or 0 when P0 has no
// room. A size of 0 gets the smallest block, so that each call returns
// a distinct address.
func (env *Environment) allocate(z *heapZone, err error, size uint32) uint32 {
	if err != nil {
		return 0
	}

	addr, err := env.allocateIn(z, max(size, 1))
	if err != nil {
		return 0
	}

	return addr
}

// release frees the block that starts at addr in the RTL's zone z,
// returning the bytes the statistics count for it, and false when none
// of z's blocks starts there (or z hasn't been made: it has no blocks).
func (env *Environment) release(z *heapZone, addr uint32) (uint32, bool) {
	if z == nil {
		return 0, false
	}

	class, ok := z.zone.Class(addr)
	if !ok {
		return 0, false
	}

	if _, err := z.zone.Free(addr); err != nil {
		return 0, false
	}

	if z.zone.Tagged() {
		class += 2 * vmheap.TagSize
	}

	return class, true
}

// blockSize returns the size of the block that starts at addr in the
// RTL's zone z, and whether one does.
func blockSize(z *heapZone, addr uint32) (uint32, bool) {
	if z == nil {
		return 0, false
	}

	return z.zone.BlockSize(addr)
}

// The C library's heap, the blocks of LIBRTL's LIB$VM_MALLOC family: VMS
// 7.1 keeps them apart from the zones LIB$FIND_VM_ZONE finds, each with
// 16 bytes before the address it returns, and LIB$STAT_VM doesn't count
// them, nor its pages (testdata/lib59: its first allocation changes no
// figure). govax's is a hidden quick fit zone (its blocks of one size
// reused, as VMS's are) in 16-page areas from P0 directly, not the page
// pool; the 16 bytes hold the size asked for. Its layout beyond that is
// govax's (unconfirmed).
var cHeapZone = vmheap.Attributes{Algorithm: vmheap.QuickFit, Lists: 128, Smallest: 16,
	BlockSize: 8, Alignment: 8, ExtendPages: 16}

// cHeader is the bytes before each C block's address.
const cHeader = 16

// CAllocate allocates size bytes from the C library's heap, returning
// the address the program uses, or 0 when there's no room.
func (env *Environment) CAllocate(size uint32) uint32 {
	if env.cHeap == nil {
		var err error
		if env.cHeap, err = vmheap.NewZone(vmheap.NewPagePool(p0Region{env}), cHeapZone); err != nil {
			panic(err) // cHeapZone's attributes are valid
		}
	}

	if size > 0xFFFFFFFF-cHeader {
		return 0
	}

	block, err := env.cHeap.Allocate(size + cHeader)
	if err != nil {
		return 0
	}

	_ = env.mem.StoreLongword(env.cpu, block, size)

	return block + cHeader
}

// CFree frees the C block at addr, reporting false when there's none.
func (env *Environment) CFree(addr uint32) bool {
	if env.cHeap == nil || addr < cHeader {
		return false
	}

	_, err := env.cHeap.Free(addr - cHeader)

	return err == nil
}

// CSize returns how many bytes the C block at addr can hold, and
// whether there's one there.
func (env *Environment) CSize(addr uint32) (uint32, bool) {
	if env.cHeap == nil || addr < cHeader {
		return 0, false
	}

	n, ok := env.cHeap.BlockSize(addr - cHeader)
	if !ok {
		return 0, false
	}

	return n - cHeader, true
}

// shimDeccMalloc is DECC$MALLOC: malloc(size) returns a new block's
// address, or 0 (NULL) when there's no room.
func shimDeccMalloc(env *Environment, argv []uint32) (uint32, error) {
	z, err := env.defaultZone()

	return env.allocate(z, err, optArg(argv, 0)), nil
}

// shimDeccFree is DECC$FREE: free(ptr) frees the block malloc returned.
// free(NULL) does nothing; a pointer that isn't a block's returns -1.
func shimDeccFree(env *Environment, argv []uint32) (uint32, error) {
	ptr := optArg(argv, 0)
	if _, ok := env.release(env.defaultHeap, ptr); ptr == 0 || ok {
		return 0, nil
	}

	return 0xFFFFFFFF, nil
}

func registerMemoryShims(t *ShimTable) {
	t.Register(15, "DECC$MALLOC", shimDeccMalloc)
	t.Register(16, "DECC$FREE", shimDeccFree)
}
