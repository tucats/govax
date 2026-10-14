package vmheap

import (
	"fmt"
	"sort"
)

// Check verifies the pool's lists: each sorted by address, no two spans
// touching or overlapping (so every free span is as large as it can be),
// every free page one the pool owns, and every held page an allocated
// one. It returns the first rule broken, or nil.
func (p *PagePool) Check() error {
	if err := checkList("owned", p.owned); err != nil {
		return err
	}

	if err := checkList("free", p.free); err != nil {
		return err
	}

	if err := checkList("held", p.held); err != nil {
		return err
	}

	for _, s := range p.free {
		if !covered(p.owned, s) {
			return fmt.Errorf("vmheap: free pages %08X-%08X aren't the pool's", s.start, s.end)
		}
	}

	for _, s := range p.held {
		if !covered(p.owned, s) || overlapsAny(p.free, s) {
			return fmt.Errorf("vmheap: held pages %08X-%08X aren't allocated", s.start, s.end)
		}
	}

	for _, s := range p.owned {
		if s.start%PageSize != 0 || s.end%PageSize != 0 {
			return fmt.Errorf("vmheap: pages %08X-%08X aren't page-aligned", s.start, s.end)
		}
	}

	return nil
}

// checkList verifies one sorted list of spans: nonempty spans, in
// address order, none touching or overlapping the next.
func checkList(name string, list []span) error {
	for i, s := range list {
		if s.start >= s.end {
			return fmt.Errorf("vmheap: %s span %08X-%08X is empty", name, s.start, s.end)
		}

		if i > 0 && list[i-1].end >= s.start {
			return fmt.Errorf("vmheap: %s spans %08X-%08X and %08X-%08X touch or overlap",
				name, list[i-1].start, list[i-1].end, s.start, s.end)
		}
	}

	return nil
}

// Check verifies the zone, and where its pages come from (Pages.Check):
//
//   - each area's pages are held for the zone, and no two areas overlap;
//   - in each area, base <= first <= top <= end, and its free spans are
//     sorted, apart, between first and top, none touching top;
//   - every block's address (after its tag) is aligned, its size a
//     multiple of the quantum, and it lies between an area's first and
//     top, overlapping no other block and no free span;
//   - in each area, the blocks and free spans add up to first..top
//     exactly, so no byte is lost or counted twice;
//   - each lookaside list holds listed blocks of its size, and every
//     listed block is on one list once;
//   - the bytes in use are the program's blocks' sizes added up.
//
// It returns the first rule broken, or nil.
func (z *Zone) Check() error {
	if err := z.pages.Check(); err != nil {
		return err
	}

	areas := append([]*area(nil), z.areas...)
	sort.Slice(areas, func(i, j int) bool { return areas[i].base < areas[j].base })

	for i, a := range areas {
		if err := z.checkArea(a); err != nil {
			return err
		}

		if i > 0 && areas[i-1].end > a.base {
			return fmt.Errorf("vmheap: areas at %08X and %08X overlap", areas[i-1].base, a.base)
		}
	}

	for addr, b := range z.blocks {
		if addr != b.start+z.offset || addr%z.align != 0 || b.n%z.round != 0 || b.n < 2*z.offset+1 {
			return fmt.Errorf("vmheap: block %08X (%08X, %d bytes) isn't aligned or sized by the zone's rules", addr, b.start, b.n)
		}
	}

	if err := z.checkLists(); err != nil {
		return err
	}

	blocks := z.sortedBlocks()
	used := make(map[*area]uint32)

	for i, b := range blocks {
		if i > 0 && blocks[i-1].end > b.start {
			return fmt.Errorf("vmheap: blocks %08X and %08X overlap", blocks[i-1].start, b.start)
		}

		a := z.areaOf(b.start)
		if a == nil || b.start < a.first || b.end > a.top {
			return fmt.Errorf("vmheap: block %08X-%08X is outside its area's blocks", b.start, b.end)
		}

		if overlapsAny(a.free, b) {
			return fmt.Errorf("vmheap: block %08X-%08X overlaps free space", b.start, b.end)
		}

		used[a] += b.size()
	}

	for _, a := range areas {
		if used[a]+totalSize(a.free) != a.top-a.first {
			return fmt.Errorf("vmheap: area %08X's blocks (%d bytes) and free spans (%d) don't make %d bytes",
				a.base, used[a], totalSize(a.free), a.top-a.first)
		}
	}

	var inUse uint32

	for _, b := range z.blocks {
		if !b.listed {
			inUse += b.n
		}
	}

	if inUse != z.inUse {
		return fmt.Errorf("vmheap: %d bytes in use counted, %d in the blocks", z.inUse, inUse)
	}

	return nil
}

// checkLists verifies the lookaside lists against the blocks.
func (z *Zone) checkLists() error {
	seen := map[uint32]bool{}

	for _, l := range z.lists {
		for _, addr := range l.addrs {
			b, ok := z.blocks[addr]
			if !ok || !b.listed || b.n != l.n || seen[addr] {
				return fmt.Errorf("vmheap: lookaside list of %d bytes holds %08X wrongly", l.n, addr)
			}

			seen[addr] = true
		}
	}

	for addr, b := range z.blocks {
		if b.listed && !seen[addr] {
			return fmt.Errorf("vmheap: listed block %08X is on no lookaside list", addr)
		}
	}

	return nil
}

// checkArea verifies one area's own bounds and free list.
func (z *Zone) checkArea(a *area) error {
	if !z.pages.Holds(a.base, (a.end-a.base)/PageSize) {
		return fmt.Errorf("vmheap: area %08X-%08X isn't held in the pool", a.base, a.end)
	}

	if a.base > a.first || a.first > a.top || a.top > a.end {
		return fmt.Errorf("vmheap: area %08X's bounds are out of order (first %08X, top %08X, end %08X)",
			a.base, a.first, a.top, a.end)
	}

	if err := checkList("area free", a.free); err != nil {
		return err
	}

	for _, s := range a.free {
		if s.start < a.first || s.end > a.top {
			return fmt.Errorf("vmheap: free span %08X-%08X is outside area %08X's blocks", s.start, s.end, a.base)
		}

		if s.end == a.top {
			return fmt.Errorf("vmheap: free span %08X-%08X touches area %08X's unallocated end", s.start, s.end, a.base)
		}
	}

	return nil
}
