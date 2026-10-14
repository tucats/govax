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

// TagSize is the size of each of a tagged block's two boundary tags: a
// longword before the block's first byte and one after its last (5.0
// LIB$ manual, section 5.3.1, "Boundary Tags": "a minimum of eight
// bytes" a block).
const TagSize = 4

// Algorithm is a zone's allocation algorithm (5.0 LIB$ manual, section
// 5.5). The RTL's LIB$K_VM_ codes are one more than these.
type Algorithm int

const (
	// FirstFit takes the lowest-addressed free space that holds a
	// request, and merges a freed block with the free space beside it.
	FirstFit Algorithm = iota

	// QuickFit keeps Lists lookaside lists of free blocks, one for each
	// size from Smallest up in steps of BlockSize; other sizes are first
	// fit.
	QuickFit

	// FrequentSizes keeps Lists lookaside lists, each for the size of
	// the first block freed to it while it was empty; other sizes are
	// first fit.
	FrequentSizes

	// FixedSize hands out blocks of one size, FixedSize bytes, and keeps
	// freed blocks on a single list.
	FixedSize
)

// Attributes are a zone's fixed properties (5.0 LIB$ manual, section
// 5.3.1, and LIB$CREATE_VM_ZONE). The zero value of each is its default
// but for BlockSize, Alignment, and ExtendPages, which must be given.
type Attributes struct {
	// Algorithm is how blocks are found and kept (FirstFit when unset).
	Algorithm Algorithm

	// Lists is the number of lookaside lists, for QuickFit (1 to 128)
	// and FrequentSizes (1 to 16).
	Lists uint32

	// FixedSize is a FixedSize zone's block size in bytes: every
	// request is that many bytes or fewer, and gets that many.
	FixedSize uint32

	// Smallest is the size, in bytes, of a QuickFit zone's first
	// lookaside list; 0 is BlockSize.
	Smallest uint32

	// BlockSize is the allocation quantum in bytes, a power of 2: every
	// request is rounded up to a multiple of it.
	BlockSize uint32

	// Alignment is the address boundary every block starts on, a power
	// of 2. Without boundary tags, the larger of BlockSize and Alignment
	// governs both (section 5.3.1, "Alignment").
	Alignment uint32

	// BoundaryTags (LIB$M_VM_BOUNDARY_TAGS) adds a longword before and
	// after each block, which holds the block's size. A block's address
	// is its first usable byte, after the first tag.
	BoundaryTags bool

	// ExtendPages is the size of a new area in pages: a zone with no
	// room for a request takes the larger of this and what the request
	// needs from the page pool.
	ExtendPages uint32

	// InitialPages is the size of the area the zone is made with; 0
	// makes none until the first request.
	InitialPages uint32

	// PageLimit is the most pages the zone's areas may hold; 0 is no
	// limit. A request that would need more is ErrPageLimit.
	PageLimit uint32

	// ExtendArea (LIB$M_VM_EXTEND_AREA) appends new pages to an area
	// they adjoin, rather than making them an area of their own.
	ExtendArea bool

	// NoExtend (LIB$M_VM_NO_EXTEND) keeps the zone to its initial area:
	// a request it can't hold is ErrNoMemory.
	NoExtend bool

	// TailLarge (LIB$M_VM_TAIL_LARGE) puts an area larger than
	// ExtendPages at the end of the area list, where it's searched last;
	// other new areas go at its head.
	TailLarge bool
}

// area is one contiguous group of pages a zone took from the pool:
//
//	base           first                 top               end
//	| control block | blocks and free spans | not yet allocated |
//
// Below top, the bytes are either blocks (the zone's blocks map, whether
// the program has them or they're on a lookaside list) or free spans
// (free, sorted by address, no two touching, none touching top: a span
// freed next to top moves top down instead). From top to end is the
// area's unallocated end, which is used after the free spans, as the
// manual's algorithm does (section 5.5).
type area struct {
	base, first, top, end uint32
	free                  []span
}

// block is one block of a zone: start and n are the bytes it occupies,
// boundary tags included (its address, the key of the zone's blocks map,
// is start plus the tag offset). listed is a block the program freed
// that waits on a lookaside list, still part of the area.
type block struct {
	start, n uint32
	listed   bool
}

// lookaside is one lookaside list: free blocks of n bytes (tags
// included), kept apart from the area's free spans, never split or
// merged, taken last in, first out. n is 0 for a FrequentSizes list that
// has no size yet.
type lookaside struct {
	n     uint32
	addrs []uint32
}

// Zone is a heap zone's block allocator (5.0 LIB$ manual, section 5.5):
// areas from a page pool, divided into blocks by the zone's algorithm.
// Space not on a lookaside list is first fit: a request takes the
// lowest-addressed free space in the first area that has room, and a
// freed block is merged with any free space on either side of it.
//
// Which blocks exist is a map from each block's address, so freeing
// checks the address exactly: an address inside a block, a block freed
// already, or an address the zone never gave out is ErrBadAddress; a
// block freed twice to a lookaside list is ErrFreedAgain, the zone left
// as it was.
type Zone struct {
	attr   Attributes
	round  uint32 // a block's size is a multiple of this
	align  uint32 // a block's address is a multiple of this
	offset uint32 // the tag before a block: 0 or TagSize
	pages  *PagePool
	areas  []*area
	blocks map[uint32]*block
	lists  []lookaside
	inUse  uint32
}

// NewZone returns a zone that takes its pages from pages, with its
// initial area if InitialPages asks for one. It returns an error for
// attributes that break their rules (BlockSize and Alignment powers of
// 2; ExtendPages positive unless NoExtend, which needs InitialPages;
// Lists positive for the lookaside algorithms; FixedSize positive for
// FixedSize; InitialPages within PageLimit), and ErrNoMemory or
// ErrPageLimit when the initial area can't be had.
func NewZone(pages *PagePool, attr Attributes) (*Zone, error) {
	if err := attr.validate(); err != nil {
		return nil, err
	}

	z := &Zone{
		attr:   attr,
		round:  max(attr.BlockSize, attr.Alignment),
		align:  max(attr.BlockSize, attr.Alignment),
		pages:  pages,
		blocks: map[uint32]*block{},
	}

	// With tags, the block size and alignment are separate: a block's
	// size (tags included) is a multiple of the block size, and its
	// first usable byte, after its first tag, is aligned.
	if attr.BoundaryTags {
		z.round, z.align, z.offset = attr.BlockSize, attr.Alignment, TagSize
	}

	z.makeLists()

	if attr.InitialPages > 0 {
		if _, err := z.addArea(attr.InitialPages); err != nil {
			return nil, err
		}
	}

	return z, nil
}

// validate checks a's rules (NewZone's).
func (a Attributes) validate() error {
	ok := isPowerOf2(a.BlockSize) && isPowerOf2(a.Alignment) &&
		(a.ExtendPages > 0 || a.NoExtend) &&
		(!a.NoExtend || a.InitialPages > 0) &&
		(a.PageLimit == 0 || a.InitialPages <= a.PageLimit) &&
		a.InitialPages <= maxPages && a.ExtendPages <= maxPages

	switch a.Algorithm {
	case FirstFit:
	case QuickFit, FrequentSizes:
		ok = ok && a.Lists > 0 && a.Lists <= 1024 && a.Smallest <= maxBlock
	case FixedSize:
		ok = ok && a.FixedSize > 0 && a.FixedSize <= maxBlock
	default:
		ok = false
	}

	if !ok {
		return fmt.Errorf("%w %+v", ErrBadAttributes, a)
	}

	return nil
}

// makeLists makes the zone's lookaside lists: a QuickFit zone's sizes
// run from Smallest's block up by BlockSize; a FrequentSizes zone's are
// set as blocks are freed; a FixedSize zone has one, of its size.
func (z *Zone) makeLists() {
	switch z.attr.Algorithm {
	case QuickFit:
		smallest := z.attr.Smallest
		if smallest == 0 {
			smallest = z.attr.BlockSize
		}

		n := z.blockBytes(smallest)

		z.lists = make([]lookaside, z.attr.Lists)
		for i := range z.lists {
			z.lists[i].n = n + uint32(i)*z.round
		}

	case FrequentSizes:
		z.lists = make([]lookaside, z.attr.Lists)

	case FixedSize:
		z.lists = []lookaside{{n: z.blockBytes(z.attr.FixedSize)}}
	}
}

// blockBytes is the bytes a request of size bytes occupies: with its
// tags, rounded up to the zone's quantum.
func (z *Zone) blockBytes(size uint32) uint32 {
	return alignUp(size+2*z.offset, z.round)
}

// Usable is how many bytes the block a request of size bytes gets can
// hold: its size, tags left out. A FixedSize zone's is always its size.
// The RTL compares the size LIB$FREE_VM is given with a block's this
// way.
func (z *Zone) Usable(size uint32) uint32 {
	if z.attr.Algorithm == FixedSize {
		size = z.attr.FixedSize
	}

	return z.blockBytes(size) - 2*z.offset
}

// Attributes returns the zone's attributes.
func (z *Zone) Attributes() Attributes {
	return z.attr
}

// maxBlock bounds a request, as maxPages does for pages.
const maxBlock = 0x40000000

// Allocate returns the address of a new block of at least size bytes,
// rounded up to the zone's quantum and aligned to it: from a lookaside
// list of its size when one has a block, else first fit. It returns
// ErrBadSize for size 0 (or more than a FixedSize zone's size),
// ErrNoMemory when the pool can't supply the pages, and ErrPageLimit
// when they would take the zone past its page limit (nothing is
// allocated then).
func (z *Zone) Allocate(size uint32) (uint32, error) {
	if size == 0 || (z.attr.Algorithm == FixedSize && size > z.attr.FixedSize) {
		return 0, ErrBadSize
	}

	if size > maxBlock {
		return 0, ErrNoMemory
	}

	if z.attr.Algorithm == FixedSize {
		size = z.attr.FixedSize
	}

	n := z.blockBytes(size)

	if l := z.listFor(n, false); l != nil && len(l.addrs) > 0 {
		addr := l.addrs[len(l.addrs)-1]
		l.addrs = l.addrs[:len(l.addrs)-1]
		z.blocks[addr].listed = false
		z.inUse += n

		return addr, nil
	}

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

// listFor returns the lookaside list for blocks of n bytes, or nil when
// none takes them. When freeing, a FrequentSizes zone gives an empty
// list the size if no list has it.
func (z *Zone) listFor(n uint32, freeing bool) *lookaside {
	for i := range z.lists {
		if z.lists[i].n == n {
			return &z.lists[i]
		}
	}

	if freeing && z.attr.Algorithm == FrequentSizes {
		for i := range z.lists {
			if len(z.lists[i].addrs) == 0 {
				z.lists[i].n = n

				return &z.lists[i]
			}
		}
	}

	return nil
}

// place is where an n-byte block would start in the space from s, so
// that its address (after its tag) is aligned.
func (z *Zone) place(s uint32) uint32 {
	return alignUp(s+z.offset, z.align) - z.offset
}

// carve allocates n bytes from area a: from its lowest free span that
// holds them, else from its unallocated end.
func (z *Zone) carve(a *area, n uint32) (uint32, bool) {
	for i, s := range a.free {
		start := z.place(s.start)
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

	start := z.place(a.top)
	if uint64(start)+uint64(n) > uint64(a.end) {
		return 0, false
	}

	if start > a.top {
		a.free = append(a.free, span{a.top, start})
	}

	a.top = start + n

	return z.record(start, n), true
}

// record notes the block at start of n bytes as allocated and returns
// its address.
func (z *Zone) record(start, n uint32) uint32 {
	addr := start + z.offset
	z.blocks[addr] = &block{start: start, n: n}
	z.inUse += n

	return addr
}

// extend gives the zone room for an n-byte block: the larger of
// ExtendPages and the pages n needs (with its control block and
// alignment), from the pool, within the page limit.
func (z *Zone) extend(n uint32) (*area, error) {
	if z.attr.NoExtend {
		return nil, ErrNoMemory
	}

	need64 := (uint64(AreaHeader) + uint64(n) + uint64(z.align) + PageSize - 1) / PageSize
	if need64 > maxPages {
		return nil, ErrNoMemory
	}

	need := uint32(need64)
	pages := max(z.attr.ExtendPages, need)

	// At the page limit, an area as large as what's left will do, if the
	// request fits in it.
	if limit := z.attr.PageLimit; limit > 0 && z.Pages()+pages > limit {
		if z.Pages()+need > limit {
			return nil, ErrPageLimit
		}

		pages = limit - z.Pages()
	}

	return z.addArea(pages)
}

// addArea takes pages pages from the pool for the zone. With ExtendArea,
// pages that start where an area ends join that area; otherwise they're
// a new area, at the head of the area list (or, with TailLarge and more
// than ExtendPages of them, at its end).
func (z *Zone) addArea(pages uint32) (*area, error) {
	addr, err := z.pages.Lend(pages)
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

	first := z.place(addr + AreaHeader)
	a := &area{base: addr, first: first, top: first, end: end}

	if z.attr.TailLarge && pages > z.attr.ExtendPages {
		z.areas = append(z.areas, a)
	} else {
		z.areas = append([]*area{a}, z.areas...)
	}

	return a, nil
}

// Free frees the block at addr and returns its usable size (as
// BlockSize gave it). The block goes to a lookaside list of its size if
// the zone has one, else back to its area's free space. It returns
// ErrBadAddress when no block is allocated at addr, and ErrFreedAgain,
// changing nothing, when the block is on a lookaside list already.
func (z *Zone) Free(addr uint32) (uint32, error) {
	b, ok := z.blocks[addr]
	if !ok {
		return 0, ErrBadAddress
	}

	if b.listed {
		return 0, ErrFreedAgain
	}

	z.inUse -= b.n

	if l := z.listFor(b.n, true); l != nil {
		b.listed = true
		l.addrs = append(l.addrs, addr)

		return b.n - 2*z.offset, nil
	}

	a := z.areaOf(b.start)
	if a == nil {
		return 0, ErrBadAddress // can't happen: every block is in an area
	}

	delete(z.blocks, addr)
	a.release(span{b.start, b.start + b.n})

	return b.n - 2*z.offset, nil
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

// Block is a block the program had, as Reset and Delete report them so
// the RTL can fill them: its address and usable size.
type Block struct {
	Addr, Size uint32
}

// Reset frees every block, the lookaside lists' too, leaving each area
// empty (LIB$RESET_VM_ZONE): the pages stay the zone's. It returns the
// blocks the program had, by address.
func (z *Zone) Reset() []Block {
	freed := z.allocated()

	for _, a := range z.areas {
		a.top, a.free = a.first, nil
	}

	clear(z.blocks)
	z.inUse = 0
	z.makeLists()

	return freed
}

// Delete frees every block and gives the zone's pages back to the pool
// (LIB$DELETE_VM_ZONE), returning the blocks the program had, by
// address. The zone can't be used afterwards.
func (z *Zone) Delete() []Block {
	freed := z.Reset()

	for _, a := range z.areas {
		if err := z.pages.Reclaim(span{a.base, a.end}); err != nil {
			panic(err) // can't happen: an area's pages are held (Check)
		}
	}

	z.areas = nil

	return freed
}

// allocated returns the blocks the program has, sorted by address.
func (z *Zone) allocated() []Block {
	var list []Block

	for addr, b := range z.blocks {
		if !b.listed {
			list = append(list, Block{addr, b.n - 2*z.offset})
		}
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Addr < list[j].Addr })

	return list
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

// BlockSize returns the usable size of the allocated block at addr (as
// allocated: rounded up from what was asked for, tags left out), and
// whether the program has one there.
func (z *Zone) BlockSize(addr uint32) (uint32, bool) {
	b, ok := z.blocks[addr]
	if !ok || b.listed {
		return 0, false
	}

	return b.n - 2*z.offset, true
}

// Tags returns the addresses of the block at addr's two boundary tags
// and the value each holds (the block's size, tags included), and false
// when the zone has no tags or no such block. The RTL writes them when
// the block is allocated and checks them when it's freed.
func (z *Zone) Tags(addr uint32) (before, after, value uint32, ok bool) {
	b, found := z.blocks[addr]
	if !found || z.offset == 0 {
		return 0, 0, 0, false
	}

	return b.start, b.start + b.n - TagSize, b.n, true
}

// Blocks is how many blocks the program has.
func (z *Zone) Blocks() int {
	return len(z.blocks) - z.listed()
}

// listed is how many blocks wait on lookaside lists.
func (z *Zone) listed() int {
	n := 0

	for _, l := range z.lists {
		n += len(l.addrs)
	}

	return n
}

// BytesInUse is the bytes in the program's blocks (as rounded, tags
// included).
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

// sortedBlocks returns every block, listed or not, as spans (tags
// included) sorted by address.
func (z *Zone) sortedBlocks() []span {
	list := make([]span, 0, len(z.blocks))

	for _, b := range z.blocks {
		list = append(list, span{b.start, b.start + b.n})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].start < list[j].start })

	return list
}
