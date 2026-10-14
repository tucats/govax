package vmheap

import (
	"fmt"
	"sort"
)

// AreaHeader is the room each area keeps at its start for its control
// block: 64 bytes, as the 5.0 LIB$ manual gives an area control block's
// size (section 5.3.1, Table 5-1). It's reserved so that block addresses
// and a zone's overhead come out as VMS's would; nothing is written
// there.
const AreaHeader = 64

// Attributes are a zone's fixed properties (5.0 LIB$ manual, section
// 5.3.1). Only those the allocator itself needs are here so far.
type Attributes struct {
	// BlockSize is the allocation quantum in bytes, a power of 2: every
	// request is rounded up to a multiple of it.
	BlockSize uint32

	// Alignment is the address boundary every block starts on, a power
	// of 2. Without boundary tags, the larger of BlockSize and Alignment
	// governs both (section 5.3.1, "Alignment").
	Alignment uint32

	// ExtendPages is the size of a new area in pages: a zone with no
	// room for a request takes the larger of this and what the request
	// needs from the page pool.
	ExtendPages uint32

	// ExtendArea (LIB$M_VM_EXTEND_AREA) appends new pages to an area
	// they adjoin, rather than making them an area of their own.
	ExtendArea bool
}

// area is one contiguous group of pages a zone took from the pool:
//
//	base           first                 top               end
//	| control block | blocks and free spans | not yet allocated |
//
// Below top, the bytes are either allocated blocks (the zone's blocks
// map) or free spans (free, sorted by address, no two touching, none
// touching top: a span freed next to top moves top down instead). From
// top to end is the area's unallocated end, which is used after the free
// spans, as the manual's algorithm does (section 5.5).
type area struct {
	base, first, top, end uint32
	free                  []span
}

// Zone is a heap zone's block allocator, using the first fit algorithm
// (LIB$K_VM_FIRST_FIT, 5.0 LIB$ manual, section 5.5.1): a request takes
// the lowest-addressed free space in the first area that has room, and a
// freed block is merged with any free space on either side of it.
//
// Which blocks are allocated is a map from each block's address to its
// size, so freeing checks the address exactly: an address inside a
// block, a block freed already, or an address the zone never gave out is
// ErrBadAddress.
type Zone struct {
	attr    Attributes
	quantum uint32
	pages   *PagePool
	areas   []*area
	blocks  map[uint32]uint32
	inUse   uint32
}

// NewZone returns a zone with no areas yet, which takes its pages from
// pages. BlockSize and Alignment must be powers of 2 and ExtendPages
// positive.
func NewZone(pages *PagePool, attr Attributes) (*Zone, error) {
	if !isPowerOf2(attr.BlockSize) || !isPowerOf2(attr.Alignment) || attr.ExtendPages == 0 {
		return nil, fmt.Errorf("vmheap: bad zone attributes %+v", attr)
	}

	return &Zone{
		attr:    attr,
		quantum: max(attr.BlockSize, attr.Alignment),
		pages:   pages,
		blocks:  map[uint32]uint32{},
	}, nil
}

// maxBlock bounds a request, as maxPages does for pages.
const maxBlock = 0x40000000

// Allocate returns the address of a new block of at least size bytes,
// rounded up to the zone's quantum and aligned to it. It returns
// ErrBadSize for size 0, and ErrNoMemory when the pool can't supply the
// pages (nothing is allocated then).
func (z *Zone) Allocate(size uint32) (uint32, error) {
	if size == 0 {
		return 0, ErrBadSize
	}

	if size > maxBlock {
		return 0, ErrNoMemory
	}

	n := alignUp(size, z.quantum)

	for _, a := range z.areas {
		if addr, ok := z.carve(a, n); ok {
			return addr, nil
		}
	}

	a, err := z.extend(n)
	if err != nil {
		return 0, err
	}

	addr, _ := z.carve(a, n)

	return addr, nil
}

// carve allocates n bytes from area a: from its lowest free span that
// holds them, else from its unallocated end.
func (z *Zone) carve(a *area, n uint32) (uint32, bool) {
	for i, s := range a.free {
		start := alignUp(s.start, z.quantum)
		if uint64(start)+uint64(n) > uint64(s.end) {
			continue
		}

		// What's left of the span on either side stays free.
		var rest []span

		if start > s.start {
			rest = append(rest, span{s.start, start})
		}

		if start+n < s.end {
			rest = append(rest, span{start + n, s.end})
		}

		a.free = append(a.free[:i], append(rest, a.free[i+1:]...)...)

		return z.record(start, n), true
	}

	start := alignUp(a.top, z.quantum)
	if uint64(start)+uint64(n) > uint64(a.end) {
		return 0, false
	}

	if start > a.top {
		a.free = append(a.free, span{a.top, start})
	}

	a.top = start + n

	return z.record(start, n), true
}

// record notes the block at addr of n bytes as allocated and returns
// addr.
func (z *Zone) record(addr, n uint32) uint32 {
	z.blocks[addr] = n
	z.inUse += n

	return addr
}

// extend gives the zone room for an n-byte block: the larger of
// ExtendPages and the pages n needs (with its control block and
// alignment), from the pool. With ExtendArea, pages that start where an
// area ends join that area; otherwise they're a new area, searched
// after the others.
func (z *Zone) extend(n uint32) (*area, error) {
	need := (uint64(AreaHeader) + uint64(n) + uint64(z.quantum) + PageSize - 1) / PageSize
	if need > maxPages {
		return nil, ErrNoMemory
	}

	pages := max(z.attr.ExtendPages, uint32(need))

	addr, err := z.pages.Allocate(pages)
	if err != nil {
		return nil, err
	}

	end := addr + pages*PageSize

	if z.attr.ExtendArea {
		for _, a := range z.areas {
			if a.end == addr {
				a.end = end

				return a, nil
			}
		}
	}

	first := alignUp(addr+AreaHeader, z.quantum)
	a := &area{base: addr, first: first, top: first, end: end}
	z.areas = append(z.areas, a)

	return a, nil
}

// Free frees the block that starts at addr and returns its size. It
// returns ErrBadAddress when no allocated block starts there.
func (z *Zone) Free(addr uint32) (uint32, error) {
	n, ok := z.blocks[addr]
	if !ok {
		return 0, ErrBadAddress
	}

	a := z.areaOf(addr)
	if a == nil {
		return 0, ErrBadAddress // can't happen: every block is in an area
	}

	delete(z.blocks, addr)
	z.inUse -= n
	a.release(span{addr, addr + n})

	return n, nil
}

// release returns s to a's free space, merged with its neighbors; free
// space that reaches top becomes part of the unallocated end.
func (a *area) release(s span) {
	a.free = insertSpan(a.free, s)

	if k := len(a.free); k > 0 && a.free[k-1].end == a.top {
		a.top = a.free[k-1].start
		a.free = a.free[:k-1]
	}
}

// areaOf returns the area holding addr, or nil.
func (z *Zone) areaOf(addr uint32) *area {
	for _, a := range z.areas {
		if a.base <= addr && addr < a.end {
			return a
		}
	}

	return nil
}

// BlockSize returns the size of the allocated block that starts at addr
// (as allocated: rounded up from what was asked for), and whether one
// does.
func (z *Zone) BlockSize(addr uint32) (uint32, bool) {
	n, ok := z.blocks[addr]

	return n, ok
}

// Blocks is how many blocks are allocated.
func (z *Zone) Blocks() int {
	return len(z.blocks)
}

// BytesInUse is the bytes in allocated blocks (as rounded).
func (z *Zone) BytesInUse() uint32 {
	return z.inUse
}

// Pages is how many pages the zone's areas hold.
func (z *Zone) Pages() uint32 {
	var n uint32

	for _, a := range z.areas {
		n += (a.end - a.base) / PageSize
	}

	return n
}

// Areas is how many areas the zone has.
func (z *Zone) Areas() int {
	return len(z.areas)
}

// sortedBlocks returns the allocated blocks as spans sorted by address.
func (z *Zone) sortedBlocks() []span {
	list := make([]span, 0, len(z.blocks))

	for addr, n := range z.blocks {
		list = append(list, span{addr, addr + n})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].start < list[j].start })

	return list
}
