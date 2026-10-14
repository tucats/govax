package vmheap

import (
	"fmt"
	"sort"
)

// AreaHeader is the size of an area's control block: 64 bytes, as the
// 5.0 LIB$ manual gives it (section 5.3.1, Table 5-1). VMS 7.1 keeps an
// area's control block apart from the area (testdata/lib59: a fresh
// area's first block is at its first byte), and so does govax: it's
// counted in a zone's overhead (LIB$SHOW_VM_ZONE) and nowhere else.
const AreaHeader = 64

// TagSize is the size of each of a tagged block's two boundary tags: a
// longword at the block's start and one at its end (5.0 LIB$ manual,
// section 5.3.1, "Boundary Tags": "a minimum of eight bytes" a block).
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

	// FrequentSizes keeps Lists lookaside lists, each for the size of a
	// block freed to it while it was empty; other sizes are first fit.
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
	// lookaside list (rounded up to a multiple of BlockSize); 0 is
	// BlockSize.
	Smallest uint32

	// BlockSize is the allocation quantum in bytes, a power of 2: every
	// request is rounded up to a multiple of it.
	BlockSize uint32

	// Alignment is the address boundary every block starts on, a power
	// of 2. Without boundary tags, the larger of BlockSize and Alignment
	// governs both (section 5.3.1, "Alignment").
	Alignment uint32

	// BoundaryTags (LIB$M_VM_BOUNDARY_TAGS) adds a longword at the start
	// and at the end of each block (tagged blocks are laid out as
	// slot describes).
	BoundaryTags bool

	// ExtendPages is the size of a new area in pages: a zone with no
	// room for a request takes the larger of this and what the request
	// needs from the page pool.
	ExtendPages uint32

	// InitialPages is the size of the area the zone is made with; 0
	// makes none until the first request.
	InitialPages uint32

	// DeferInitial makes the initial area at the zone's first request
	// instead, as VMS 7.1 makes its default zone's and string zone's
	// (testdata/lib59: "0 pages in 0 areas" until then).
	DeferInitial bool

	// PageLimit is the most pages the zone's areas may hold; 0 is no
	// limit. A request that would need more is ErrPageLimit.
	PageLimit uint32

	// ExtendArea (LIB$M_VM_EXTEND_AREA) appends new pages to an area
	// they adjoin, rather than making them an area of their own.
	ExtendArea bool

	// NoExtend (LIB$M_VM_NO_EXTEND) keeps the zone to its first area: a
	// request it can't hold is ErrPageLimit, as VMS 7.1 answers
	// (testdata/lib59).
	NoExtend bool

	// TailLarge (LIB$M_VM_TAIL_LARGE) puts an area larger than
	// ExtendPages at the end of the area list, where it's searched last;
	// other new areas go at its head.
	TailLarge bool
}

// area is one contiguous group of pages a zone took from the pool:
//
//	base                       top               end
//	| blocks and free spans     | not yet allocated |
//
// Below top, the bytes are either blocks (the zone's blocks map, whether
// the program has them or they're on a lookaside list) or free spans
// (free, sorted by address, no two touching). From top to end is the
// area's unallocated end, which is used after the free spans, as the
// manual's algorithm does (section 5.5). Free space below top may reach
// it (release).
type area struct {
	base, top, end uint32
	free           []span
}

// block is one block of a zone: start and end are the bytes it occupies
// (its slot, tags included; its address, the key of the zone's blocks
// map, is its first usable byte), and class the request size it serves,
// rounded to the zone's quantum, which picks its lookaside list. listed
// is a block the program freed that waits on a lookaside list, still
// part of the area.
type block struct {
	start, end, class uint32
	listed            bool
}

func (b *block) size() uint32 {
	return b.end - b.start
}

// lookaside is one lookaside list: free blocks of one class, kept apart
// from the area's free spans, never split or merged, taken last in,
// first out. n is the class (0 for a FrequentSizes list that has no size
// yet). loop, when not 0, is a block the program freed twice in a row:
// as on VMS, whose list then links the block to itself, every request
// the list serves gets that block from then on (FreeAgain).
type lookaside struct {
	n     uint32
	addrs []uint32
	loop  uint32
}

// Zone is a heap zone's block allocator (5.0 LIB$ manual, section 5.5):
// areas from a page pool, divided into blocks by the zone's algorithm.
// Space not on a lookaside list is first fit: a request takes the
// lowest-addressed free space in the first area that has room, and a
// freed block is merged with any free space on either side of it.
type Zone struct {
	attr   Attributes
	round  uint32 // a block's size is a multiple of this
	align  uint32 // a block's address is a multiple of this
	tagged bool
	pages  Pages
	areas  []*area
	blocks map[uint32]*block
	lists  []lookaside
	inUse  uint32

	// freed is LIB$SHOW_VM_ZONE's "bytes freed and not yet reallocated",
	// a running count as VMS 7.1 keeps it (FreedBytes).
	freed uint32

	// next is the FrequentSizes list a size no list has takes over when
	// every list has one (round robin).
	next int

	// onRelease and onList, when set, are told of the free space each
	// free makes (OnRelease, OnList).
	onRelease func(freed, merged Span)
	onList    func(Span)
}

// NewZone returns a zone that takes its pages from pages (a *PagePool,
// or a Borrowed for a zone with page routines of the program's), with its
// initial area if InitialPages asks for one. It returns an error for
// attributes that break their rules (BlockSize and Alignment powers of
// 2; ExtendPages positive unless NoExtend; Lists positive for the
// lookaside algorithms; FixedSize positive for FixedSize; InitialPages
// within PageLimit), and ErrNoMemory or ErrPageLimit when the initial
// area can't be had.
func NewZone(pages Pages, attr Attributes) (*Zone, error) {
	if err := attr.validate(); err != nil {
		return nil, err
	}

	z := &Zone{
		attr:   attr,
		round:  max(attr.BlockSize, attr.Alignment),
		align:  max(attr.BlockSize, attr.Alignment),
		tagged: attr.BoundaryTags,
		pages:  pages,
		blocks: map[uint32]*block{},
	}

	// With tags, the block size and alignment are separate: a block's
	// size (tags included) is a multiple of the block size, and its
	// first usable byte, after its first tag, is aligned.
	if attr.BoundaryTags {
		z.round, z.align = attr.BlockSize, attr.Alignment
	}

	z.makeLists()

	if attr.InitialPages > 0 && !attr.DeferInitial {
		if _, err := z.addArea(attr.InitialPages); err != nil {
			return nil, err
		}
	}

	return z, nil
}

// validate checks a's rules (NewZone's).
func (a Attributes) validate() error {
	ok := isPowerOf2(a.BlockSize) && isPowerOf2(a.Alignment) &&
		a.BlockSize <= PageSize && a.Alignment <= PageSize &&
		(a.ExtendPages > 0 || a.NoExtend) &&
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

// makeLists makes the zone's lookaside lists: a QuickFit zone's classes
// run from Smallest up by the quantum; a FrequentSizes zone's are set as
// blocks are freed; a FixedSize zone has one, of its size.
func (z *Zone) makeLists() {
	z.next = 0

	switch z.attr.Algorithm {
	case QuickFit:
		n := z.Smallest()

		z.lists = make([]lookaside, z.attr.Lists)
		for i := range z.lists {
			z.lists[i].n = n + uint32(i)*z.round
		}

	case FrequentSizes:
		z.lists = make([]lookaside, z.attr.Lists)

	case FixedSize:
		z.lists = []lookaside{{n: z.class(z.attr.FixedSize)}}
	}
}

// Smallest is a QuickFit zone's first list's class: Smallest rounded up
// to the quantum (BlockSize when 0), as VMS 7.1 shows it.
func (z *Zone) Smallest() uint32 {
	return z.class(max(z.attr.Smallest, z.attr.BlockSize))
}

// class is the class a request of size bytes falls in: the size rounded
// up to the zone's quantum (a FixedSize zone's is its size's).
func (z *Zone) class(size uint32) uint32 {
	if z.attr.Algorithm == FixedSize {
		size = z.attr.FixedSize
	}

	return alignUp(size, z.round)
}

// Nominal is the bytes LIB$STAT_VM counts for a block of size bytes: its
// class, and with boundary tags their eight bytes, as VMS 7.1 counts
// them (testdata/lib59).
func (z *Zone) Nominal(size uint32) uint32 {
	n := z.class(size)
	if z.tagged {
		n += 2 * TagSize
	}

	return n
}

// slot is where a block of size bytes would lie in the space from s:
// its address d and the end of the bytes it occupies, e. An untagged
// block starts at s aligned and is its class long. A tagged block (as
// VMS 7.1 lays them out, testdata/lib59) starts at s with its first tag,
// has its address at the first aligned byte after that tag, holds the
// request rounded to a longword, and ends with its second tag, the whole
// rounded up to the block size.
func (z *Zone) slot(s, size uint32) (d uint32, e uint64) {
	if !z.tagged {
		d = alignUp(s, z.align)

		return d, uint64(d) + uint64(z.class(size))
	}

	d = alignUp(s+TagSize, z.align)
	e = uint64(d) + uint64(alignUp(size, TagSize)) + TagSize

	return d, (e + uint64(z.round) - 1) &^ uint64(z.round-1)
}

// Attributes returns the zone's attributes.
func (z *Zone) Attributes() Attributes {
	return z.attr
}

// maxBlock bounds a request, as maxPages does for pages.
const maxBlock = 0x40000000

// Allocate returns the address of a new block of at least size bytes,
// rounded up to the zone's quantum and aligned to it: from a lookaside
// list of its class when one has a block, else first fit. It returns
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

	if z.attr.DeferInitial && z.attr.InitialPages > 0 && len(z.areas) == 0 {
		if _, err := z.addArea(z.attr.InitialPages); err != nil {
			return 0, err
		}
	}

	c := z.class(size)

	if l := z.listFor(c, false); l != nil {
		if addr, ok := z.pop(l, size); ok {
			return addr, nil
		}
	}

	for _, a := range z.areas {
		if addr, ok := z.carve(a, size); ok {
			return addr, nil
		}
	}

	a, err := z.extend(size)
	if err != nil {
		return 0, err
	}

	addr, _ := z.carve(a, size)

	return addr, nil
}

// pop takes the latest block from l for a request of size bytes, if it
// has one that holds it (a tagged block's slot depends on where it lay;
// one that's too small for this request stays on the list).
func (z *Zone) pop(l *lookaside, size uint32) (uint32, bool) {
	addr := l.loop

	if addr == 0 {
		if len(l.addrs) == 0 {
			return 0, false
		}

		addr = l.addrs[len(l.addrs)-1]
		b := z.blocks[addr]

		if z.tagged && uint64(addr)+uint64(alignUp(size, TagSize))+TagSize > uint64(b.end) {
			return 0, false
		}

		l.addrs = l.addrs[:len(l.addrs)-1]
	}

	z.reused(l.n + z.tagBytes())

	b := z.blocks[addr]
	b.listed = false
	z.inUse += b.size()

	return addr, true
}

// listFor returns the lookaside list for blocks of class n, or nil when
// none takes them. When freeing, a FrequentSizes zone gives the size to
// an empty list if no list has it, and when none is empty, to the next
// list in turn, whose blocks go back to their areas' free space (as VMS
// 7.1 does, testdata/lib59).
func (z *Zone) listFor(n uint32, freeing bool) *lookaside {
	for i := range z.lists {
		if z.lists[i].n == n {
			return &z.lists[i]
		}
	}

	if !freeing || z.attr.Algorithm != FrequentSizes {
		return nil
	}

	for i := range z.lists {
		if z.lists[i].n == 0 {
			z.lists[i].n = n

			return &z.lists[i]
		}
	}

	l := &z.lists[z.next]
	z.next = (z.next + 1) % len(z.lists)

	if l.loop == 0 {
		for _, addr := range l.addrs {
			z.unlist(addr)
		}

		l.addrs, l.n = nil, n
	}

	if l.n != n {
		return nil // a looped list can't be emptied
	}

	return l
}

// unlist returns a listed block to its area's free space.
func (z *Zone) unlist(addr uint32) {
	b := z.blocks[addr]
	delete(z.blocks, addr)

	if a := z.areaOf(b.start); a != nil {
		z.release(a, span{b.start, b.end})
	}
}

// carve allocates a block of size bytes from area a: from its lowest
// free span that holds it, else from its unallocated end.
func (z *Zone) carve(a *area, size uint32) (uint32, bool) {
	for i, s := range a.free {
		d, e := z.slot(s.start, size)
		if e > uint64(s.end) {
			continue
		}

		start, end := s.start, uint32(e)
		var rest []span

		if !z.tagged && d > s.start {
			rest = append(rest, span{s.start, d})
			start = d
		}

		if end < s.end {
			rest = append(rest, span{end, s.end})
		}

		a.free = append(a.free[:i], append(rest, a.free[i+1:]...)...)

		if end < s.end && z.onRelease != nil {
			z.onRelease(Span{}, Span{end, s.end})
		}

		z.reused(z.Nominal(size))

		return z.record(start, d, end, size), true
	}

	d, e := z.slot(a.top, size)
	if e > uint64(a.end) {
		return 0, false
	}

	// An untagged zone's top is always aligned (its blocks are multiples
	// of its alignment, from a page), so its block starts there too.
	start := a.top
	a.top = uint32(e)

	return z.record(start, d, uint32(e), size), true
}

// tagBytes is the bytes a block's tags take: 8 with boundary tags.
func (z *Zone) tagBytes() uint32 {
	if z.tagged {
		return 2 * TagSize
	}

	return 0
}

// reused takes n bytes freed space gave a request off the freed count.
func (z *Zone) reused(n uint32) {
	z.freed -= min(n, z.freed)
}

// FreedBytes is the bytes freed and not yet allocated again, as VMS 7.1
// counts them for LIB$SHOW_VM_ZONE (testdata/lib59): every free whose
// size is good adds its bytes, though it fails (bytes outside the zone,
// or free already), and every request served from freed space takes
// its own off (a list linked to itself too); a reset clears it. A
// block's bytes are Nominal's, its tags included.
func (z *Zone) FreedBytes() uint32 {
	return z.freed
}

// record notes the block from start to end, address d, as allocated for
// a request of size bytes, and returns d.
func (z *Zone) record(start, d, end, size uint32) uint32 {
	z.blocks[d] = &block{start: start, end: end, class: z.class(size)}
	z.inUse += end - start

	return d
}

// extend gives the zone room for a block of size bytes: the larger of
// ExtendPages and the pages the block needs, from the pool, within the
// page limit. A NoExtend zone has only its first area.
func (z *Zone) extend(size uint32) (*area, error) {
	if z.attr.NoExtend && len(z.areas) > 0 {
		return nil, ErrPageLimit
	}

	// An area starts on a page, so a block from its start needs this
	// much of it.
	_, e := z.slot(0, size)

	need64 := (e + PageSize - 1) / PageSize
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

	a := &area{base: addr, top: addr, end: end}

	if z.attr.TailLarge && pages > z.attr.ExtendPages {
		z.areas = append(z.areas, a)
	} else {
		z.areas = append([]*area{a}, z.areas...)
	}

	return a, nil
}

// Free frees the block at addr and returns the bytes it occupied (its
// slot, tags included). The block goes to a lookaside list of its class
// if the zone has one, else back to its area's free space. It returns
// ErrBadAddress when no block is allocated at addr, and ErrFreedAgain,
// changing nothing, when the block is on a lookaside list already.
func (z *Zone) Free(addr uint32) (Span, error) {
	b, ok := z.blocks[addr]
	if !ok {
		return Span{}, ErrBadAddress
	}

	z.freed += b.class + z.tagBytes()

	if b.listed {
		return Span{}, ErrFreedAgain
	}

	z.inUse -= b.size()
	slot := Span{b.start, b.end}

	if l := z.listFor(b.class, true); l != nil {
		z.list(l, addr)

		return slot, nil
	}

	a := z.areaOf(b.start)
	if a == nil {
		return Span{}, ErrBadAddress // can't happen: every block is in an area
	}

	delete(z.blocks, addr)
	z.release(a, span{b.start, b.end})

	return slot, nil
}

// list puts the block at addr on l.
func (z *Zone) list(l *lookaside, addr uint32) {
	b := z.blocks[addr]
	b.listed = true
	l.addrs = append(l.addrs, addr)

	if z.onList != nil {
		z.onList(Span{b.start, b.end})
	}
}

// FreeRange frees the size bytes at addr in an untagged zone, as VMS 7.1
// does (testdata/lib59): it trusts the size it's given. The bytes, size
// rounded to the zone's quantum (a FixedSize zone's size), must be
// aligned, below an area's unallocated end, and not free already; a
// block they cover in part keeps the rest, as a block of its own. They
// go to a lookaside list of their class if the zone has one, else back
// to the area's free space. It returns the bytes freed; ErrBadSize for
// size 0 (or more than a FixedSize zone's size); ErrBadAddress for bytes
// that aren't the zone's blocks; and ErrFreedAgain, changing nothing,
// for a block on a lookaside list already (FreeAgain).
func (z *Zone) FreeRange(addr, size uint32) (uint32, error) {
	if z.tagged {
		return 0, fmt.Errorf("vmheap: FreeRange of a tagged zone: %w", ErrBadAddress)
	}

	if size == 0 || (z.attr.Algorithm == FixedSize && size > z.attr.FixedSize) {
		return 0, ErrBadSize
	}

	if size > maxBlock {
		return 0, ErrBadAddress
	}

	c := z.class(size)
	r := span{addr, addr + c}
	z.freed += c

	a := z.areaOf(addr)
	if addr%z.align != 0 || a == nil || r.end < addr || r.end > a.top {
		return 0, ErrBadAddress
	}

	if overlapsAny(a.free, r) {
		return 0, ErrDoubleFree
	}

	var cut []uint32

	for k, b := range z.blocks {
		if b.start >= r.end || b.end <= r.start {
			continue
		}

		if b.listed {
			if b.start == r.start && b.end == r.end {
				return 0, ErrFreedAgain
			}

			return 0, ErrBadAddress
		}

		cut = append(cut, k)
	}

	// What the bytes cover of each block is freed; the rest of it stays
	// the program's, as blocks of their own.
	for _, k := range cut {
		b := z.blocks[k]
		delete(z.blocks, k)
		z.inUse -= b.size()

		if b.start < r.start {
			z.blocks[b.start] = &block{start: b.start, end: r.start, class: r.start - b.start}
			z.inUse += r.start - b.start
		}

		if r.end < b.end {
			z.blocks[r.end] = &block{start: r.end, end: b.end, class: b.end - r.end}
			z.inUse += b.end - r.end
		}
	}

	if l := z.listFor(c, true); l != nil {
		z.blocks[addr] = &block{start: addr, end: r.end, class: c}
		z.list(l, addr)

		return c, nil
	}

	z.release(a, r)

	return c, nil
}

// FreeAgain frees again the block at addr, which waits on a lookaside
// list (Free or FreeRange returned ErrFreedAgain), as VMS does: "freeing
// a block more than once returns SS$_NORMAL" there (5.0 LIB$ manual,
// chapter 6), and its list links the block to itself, so that the list
// gives that block to every request it serves from then on (testdata/
// lib59: two LIB$GET_VM calls return the same block). Check reports the
// list damaged.
func (z *Zone) FreeAgain(addr uint32) {
	b, ok := z.blocks[addr]
	if !ok || !b.listed {
		return
	}

	for i := range z.lists {
		l := &z.lists[i]
		if k := len(l.addrs); k > 0 && l.addrs[k-1] == addr {
			l.loop = addr
		}
	}
}

// release returns s to a's free space, merged with its neighbors. Free
// space that reaches the area's unallocated end stays free space: VMS
// 7.1 doesn't give it back to the end (testdata/lib59: a block freed
// last in an area is found on its free list, and the area's bytes not
// yet allocated don't grow). The zone's OnRelease function is told what
// was freed and what it became.
func (z *Zone) release(a *area, s span) {
	a.free = insertSpan(a.free, s)

	i := sort.Search(len(a.free), func(k int) bool { return a.free[k].end >= s.end })
	merged := a.free[i]

	if z.onRelease != nil {
		z.onRelease(Span{s.start, s.end}, Span{merged.start, merged.end})
	}
}

// Block is a block the program had, as Reset and Delete report them so
// the RTL can fill them and count them: its address, the bytes it
// occupies (Start to End, tags included), and its class.
type Block struct {
	Addr, Start, End, Class uint32
}

// Reset frees every block, the lookaside lists' too, leaving each area
// empty (LIB$RESET_VM_ZONE): the pages stay the zone's. It returns the
// blocks the program had, by address.
func (z *Zone) Reset() []Block {
	freed := z.allocated()

	for _, a := range z.areas {
		a.top, a.free = a.base, nil
	}

	clear(z.blocks)
	z.inUse, z.freed = 0, 0
	z.makeLists()

	return freed
}

// Delete frees every block and gives the zone's pages back to where
// they came from (LIB$DELETE_VM_ZONE), returning the blocks the program
// had, by address. The areas go back in the order they're searched,
// stopping at the first that can't be given back (a program's free-page
// routine's failure, which is the error; a PagePool's areas always go
// back), as VMS 7.1 does (testdata/lib59). The zone can't be used
// afterwards.
func (z *Zone) Delete() ([]Block, error) {
	freed := z.Reset()

	for _, a := range z.areas {
		if err := z.pages.Reclaim(a.base, (a.end-a.base)/PageSize); err != nil {
			z.areas = nil

			return freed, err
		}
	}

	z.areas = nil

	return freed, nil
}

// allocated returns the blocks the program has, sorted by address.
func (z *Zone) allocated() []Block {
	var list []Block

	for addr, b := range z.blocks {
		if !b.listed {
			list = append(list, Block{addr, b.start, b.end, b.class})
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

// BlockSize returns how many bytes the allocated block at addr can hold
// (from its address to its end, its second tag left out), and whether
// the program has one there.
func (z *Zone) BlockSize(addr uint32) (uint32, bool) {
	b, ok := z.blocks[addr]
	if !ok || b.listed {
		return 0, false
	}

	n := b.end - addr
	if z.tagged {
		n -= TagSize
	}

	return n, true
}

// Class returns the class of the allocated block at addr (its request
// rounded to the quantum), and whether the program has one there.
func (z *Zone) Class(addr uint32) (uint32, bool) {
	b, ok := z.blocks[addr]
	if !ok || b.listed {
		return 0, false
	}

	return b.class, true
}

// Tags returns the addresses of the block at addr's two boundary tags
// (at its first byte and its last longword) and the bytes it occupies,
// and false when the zone has no tags or no such block, allocated or
// listed. The RTL writes and checks them.
func (z *Zone) Tags(addr uint32) (before, after, size uint32, ok bool) {
	b, found := z.blocks[addr]
	if !found || !z.tagged {
		return 0, 0, 0, false
	}

	return b.start, b.end - TagSize, b.size(), true
}

// Freed reports whether addr lies in the zone's free space or in a block
// on a lookaside list: a tagged block freed already.
func (z *Zone) Freed(addr uint32) bool {
	if b, ok := z.blocks[addr]; ok {
		return b.listed
	}

	a := z.areaOf(addr)

	return a != nil && (addr >= a.top || overlapsAny(a.free, span{addr, addr + 1}))
}

// listed is how many blocks wait on lookaside lists.
func (z *Zone) listed() int {
	n := 0

	for _, l := range z.lists {
		n += len(l.addrs)
	}

	return n
}

// Blocks is how many blocks the program has.
func (z *Zone) Blocks() int {
	n := 0

	for _, b := range z.blocks {
		if !b.listed {
			n++
		}
	}

	return n
}

// BytesInUse is the bytes in the program's blocks (the bytes they
// occupy, tags included).
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
		list = append(list, span{b.start, b.end})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].start < list[j].start })

	return list
}
