package vmheap

import (
	"fmt"
	"testing"
)

// AreaReports and ListReports: an area's bounds, its free spans, and its
// unallocated end; the lookaside lists' classes and their blocks, last
// freed first, each block's bytes with its tags.
func TestZoneReports(t *testing.T) {
	z, _ := newZone(t, Attributes{Algorithm: QuickFit, Lists: 2, BlockSize: 8, Alignment: 4, BoundaryTags: true, ExtendPages: 2})

	a := mustAllocate(t, z, 8)  // 16 bytes with tags: list 1
	b := mustAllocate(t, z, 40) // 48: no list
	c := mustAllocate(t, z, 8)
	mustAllocate(t, z, 8)

	mustFree(t, z, a)
	mustFree(t, z, c)
	mustFree(t, z, b)

	areas := z.AreaReports()
	if len(areas) != 1 {
		t.Fatalf("%d areas, want 1", len(areas))
	}

	r := areas[0]
	base := (a - TagSize) &^ (PageSize - 1)

	if r.Base != base || r.End != base+2*PageSize || r.Top != a-TagSize+96 || r.Unallocated() != r.End-r.Top {
		t.Errorf("area %+v, unallocated %d", r, r.Unallocated())
	}

	if want := []Span{{b - TagSize, b - TagSize + 48}}; fmt.Sprint(r.Free) != fmt.Sprint(want) {
		t.Errorf("free spans %X, want %X", r.Free, want)
	}

	lists := z.ListReports()
	want := []ListReport{
		{Size: 8, Blocks: []Span{{c - TagSize, c + 12}, {a - TagSize, a + 12}}, Addrs: []uint32{c, a}},
		{Size: 16},
	}

	if fmt.Sprint(lists) != fmt.Sprint(want) {
		t.Errorf("lists %+v, want %+v", lists, want)
	}

	if z.Quantum() != 8 || z.Alignment() != 4 {
		t.Errorf("quantum %d, alignment %d", z.Quantum(), z.Alignment())
	}
}
