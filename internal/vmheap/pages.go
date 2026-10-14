package vmheap

import "sort"

// Region is where a PagePool's pages come from: in govax, the process's
// P0 region, through $EXPREG's code.
type Region interface {
	// End is the address of the page the next Expand would return.
	End() uint32

	// Expand adds pages new pages at the region's end and returns the
	// first one's address, or false when the region can't grow that far.
	Expand(pages uint32) (uint32, bool)
}

// maxPages bounds a single request: more pages than a VAX region holds
// (2^30 bytes) can't be satisfied, and refusing them early keeps the
// arithmetic below from overflowing.
const maxPages = 0x40000000 / PageSize

// PagePool is the processwide pool of pages LIB$GET_VM_PAGE and
// LIB$FREE_VM_PAGE manage (5.0 LIB$ manual, section 5.2). Pages are
// handed out page-aligned; a group may be freed in parts, and contiguous
// groups from several calls may be freed together, as the manual allows.
//
// The pool keeps two lists of spans, each sorted by address: the pages
// it owns (every page the region ever gave it, adjacent spans merged)
// and, within those, the pages that are free (adjacent spans merged, so
// each free span is as large as it can be). A page is allocated when the
// pool owns it and it isn't free.
//
// A third list, held, is the allocated pages a zone took as its areas
// (Lend). Free refuses them: a zone's pages go back with the zone, so a
// program's LIB$FREE_VM_PAGE of them can't hand them out a second time
// while the zone's blocks are in them.
//
// The pool also counts its successful requests (Calls), for LIB$STAT_VM
// and LIB$SHOW_VM: allocations (Allocate, and Lend, a zone's) and frees
// (Free, and Reclaim, a zone's).
type PagePool struct {
	region Region
	owned  []span
	free   []span
	held   []span
	gets   uint32
	frees  uint32
}

// NewPagePool returns an empty pool that takes its pages from region.
func NewPagePool(region Region) *PagePool {
	return &PagePool{region: region}
}

// Allocate returns the address of n contiguous pages: the lowest free
// ones that fit, or new ones from the region when none do. It returns
// ErrBadSize for n of 0, and ErrNoMemory when the region can't grow
// enough (nothing is allocated then).
func (p *PagePool) Allocate(n uint32) (uint32, error) {
	if n == 0 {
		return 0, ErrBadSize
	}

	if n > maxPages {
		return 0, ErrNoMemory
	}

	if addr, ok := p.take(n); ok {
		p.gets++

		return addr, nil
	}

	if err := p.grow(n); err != nil {
		return 0, err
	}

	addr, _ := p.take(n)
	p.gets++

	return addr, nil
}

// take allocates n pages from the lowest free span that holds them.
func (p *PagePool) take(n uint32) (uint32, bool) {
	bytes := n * PageSize

	for i, s := range p.free {
		if s.size() < bytes {
			continue
		}

		if s.size() == bytes {
			p.free = append(p.free[:i], p.free[i+1:]...)
		} else {
			p.free[i].start += bytes
		}

		return s.start, true
	}

	return 0, false
}

// grow gets pages from the region so that n contiguous free pages exist.
// When the highest free span ends where the region will grow from, only
// the pages it lacks are asked for: the new pages join it.
func (p *PagePool) grow(n uint32) error {
	need := n

	if k := len(p.free); k > 0 && p.free[k-1].end == p.region.End() {
		need -= p.free[k-1].size() / PageSize
	}

	for {
		addr, ok := p.region.Expand(need)
		if !ok {
			return ErrNoMemory
		}

		added := span{addr, addr + need*PageSize}
		p.owned = insertSpan(p.owned, added)
		p.free = insertSpan(p.free, added)

		// The region normally gives exactly where End said, so the new
		// pages complete the run. If they landed elsewhere (another
		// caller moved the region's end in between), ask for the whole
		// request again; the pages just added stay in the pool.
		if p.hasRun(n) {
			return nil
		}

		need = n
	}
}

// hasRun reports whether some free span holds n pages.
func (p *PagePool) hasRun(n uint32) bool {
	for _, s := range p.free {
		if s.size() >= n*PageSize {
			return true
		}
	}

	return false
}

// Lend allocates n pages as Allocate does, for a zone's area: they're
// held, so Free refuses them.
func (p *PagePool) Lend(n uint32) (uint32, error) {
	addr, err := p.Allocate(n)
	if err != nil {
		return 0, err
	}

	p.held = insertSpan(p.held, span{addr, addr + n*PageSize})

	return addr, nil
}

// Free returns n pages starting at addr to the pool. Every page must be
// allocated, and not held by a zone: ErrBadAddress is an addr that isn't
// page-aligned or pages the pool doesn't own, that are free already, or
// that a zone holds, and ErrBadSize is n of 0. Nothing is freed unless
// all of them can be.
func (p *PagePool) Free(n, addr uint32) error {
	if n == 0 {
		return ErrBadSize
	}

	if addr%PageSize != 0 || n > maxPages || uint64(addr)+uint64(n)*PageSize > 1<<32 {
		return ErrBadAddress
	}

	s := span{addr, addr + n*PageSize}

	if !covered(p.owned, s) || overlapsAny(p.free, s) || overlapsAny(p.held, s) {
		return ErrBadAddress
	}

	p.free = insertSpan(p.free, s)
	p.frees++

	return nil
}

// Reclaim returns n pages starting at addr, which a zone held (Lend),
// to the pool's free pages: a zone deleted gives its areas back this
// way. It returns ErrBadAddress, and changes nothing, unless a zone
// holds every one of them.
func (p *PagePool) Reclaim(addr, n uint32) error {
	s := span{addr, addr + n*PageSize}

	if !covered(p.held, s) {
		return ErrBadAddress
	}

	p.held = removeSpan(p.held, s)
	p.free = insertSpan(p.free, s)
	p.frees++

	return nil
}

// Calls is how many requests for pages the pool has satisfied (gets:
// Allocate and Lend) and how many frees it has taken (Free and
// Reclaim). Refused requests aren't counted.
func (p *PagePool) Calls() (gets, frees uint32) {
	return p.gets, p.frees
}

// AllocatedPages is how many of the pool's pages are allocated, held by
// zones or not: those it owns that aren't free.
func (p *PagePool) AllocatedPages() uint32 {
	return p.OwnedPages() - p.FreePages()
}

// OwnedPages is how many pages the region has given the pool.
func (p *PagePool) OwnedPages() uint32 {
	return totalSize(p.owned) / PageSize
}

// FreePages is how many of the pool's pages are free.
func (p *PagePool) FreePages() uint32 {
	return totalSize(p.free) / PageSize
}

// HeldPages is how many of the pool's pages zones hold (Lend).
func (p *PagePool) HeldPages() uint32 {
	return totalSize(p.held) / PageSize
}

// Holds reports whether a zone holds (Lend) every one of the n pages
// starting at addr: Zone.Check asks it of each area.
func (p *PagePool) Holds(addr, n uint32) bool {
	return covered(p.held, span{addr, addr + n*PageSize})
}

// insertSpan adds s to list, which is sorted by address and has no two
// spans touching, merging s with the span before it and the span after
// it when they touch, and returns the list. s must overlap nothing in
// list.
func insertSpan(list []span, s span) []span {
	// i is the first span that starts after s.
	i := sort.Search(len(list), func(k int) bool { return list[k].start > s.start })

	mergePrev := i > 0 && list[i-1].end == s.start
	mergeNext := i < len(list) && list[i].start == s.end

	switch {
	case mergePrev && mergeNext:
		list[i-1].end = list[i].end

		return append(list[:i], list[i+1:]...)

	case mergePrev:
		list[i-1].end = s.end

		return list

	case mergeNext:
		list[i].start = s.start

		return list
	}

	list = append(list, span{})
	copy(list[i+1:], list[i:])
	list[i] = s

	return list
}

// removeSpan takes s out of list (sorted, touching spans merged), which
// must cover it (covered), and returns the list: the span holding s is
// cut down to what lies before s and after it, either of which may be
// nothing.
func removeSpan(list []span, s span) []span {
	i := sort.Search(len(list), func(k int) bool { return list[k].end > s.start })
	whole := list[i]

	var rest []span

	if whole.start < s.start {
		rest = append(rest, span{whole.start, s.start})
	}

	if s.end < whole.end {
		rest = append(rest, span{s.end, whole.end})
	}

	return append(list[:i], append(rest, list[i+1:]...)...)
}

// covered reports whether s lies entirely within one span of list
// (sorted, touching spans merged, so a range that is all in the list is
// in one span).
func covered(list []span, s span) bool {
	i := sort.Search(len(list), func(k int) bool { return list[k].end > s.start })

	return i < len(list) && list[i].start <= s.start && s.end <= list[i].end
}

// overlapsAny reports whether s shares a byte with any span of list
// (sorted).
func overlapsAny(list []span, s span) bool {
	i := sort.Search(len(list), func(k int) bool { return list[k].end > s.start })

	return i < len(list) && list[i].start < s.end
}

// totalSize is the bytes in list's spans.
func totalSize(list []span) uint32 {
	var n uint32

	for _, s := range list {
		n += s.size()
	}

	return n
}
