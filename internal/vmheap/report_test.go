package vmheap

import (
	"fmt"
	"testing"
)

// AreaReports and ListReports: an area's bounds, its free spans, and its
// unallocated end; the lookaside lists' usable sizes and their blocks,
// last freed first; tags left out of a list's size and in its blocks.
func TestZoneReports(t *testing.T) {
	z, _ := newZone(t, Attributes{Algorithm: QuickFit, Lists: 2, BlockSize: 8, Alignment: 8, BoundaryTags: true, ExtendPages: 2})

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
	base := (a - TagSize - AreaHeader) &^ (PageSize - 1)

	if r.Base != base || r.End != base+2*PageSize || r.Top != a-TagSize+96 || r.Unallocated() != r.End-r.Top {
		t.Errorf("area %+v, unallocated %d", r, r.Unallocated())
	}

	if want := []Span{{b - TagSize, b - TagSize + 48}}; fmt.Sprint(r.Free) != fmt.Sprint(want) {
		t.Errorf("free spans %X, want %X", r.Free, want)
	}

	lists := z.ListReports()
	want := []ListReport{
		{Size: 8, Blocks: []Span{{c - TagSize, c + 12}, {a - TagSize, a + 12}}},
		{Size: 16},
	}

	if fmt.Sprint(lists) != fmt.Sprint(want) {
		t.Errorf("lists %+v, want %+v", lists, want)
	}

	if z.Quantum() != 8 || z.Alignment() != 8 {
		t.Errorf("quantum %d, alignment %d", z.Quantum(), z.Alignment())
	}
}

// OnPadding: the bytes skipped to align a block taken from the
// unallocated end are reported, and they're free space; a block from a
// free span reports nothing.
func TestZonePadding(t *testing.T) {
	z, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 16, BoundaryTags: true, ExtendPages: 1})

	var pads []Span

	z.OnPadding(func(s Span) { pads = append(pads, s) })

	// 24 bytes each with their tags, so the second's address must skip
	// 8 bytes to be 16-aligned after its tag.
	a := mustAllocate(t, z, 16)
	b := mustAllocate(t, z, 16)

	if len(pads) != 1 || pads[0] != (Span{a - TagSize + 24, b - TagSize}) || pads[0].Size() != 8 {
		t.Fatalf("padding %X (blocks %X, %X)", pads, a, b)
	}

	if free := z.AreaReports()[0].Free; fmt.Sprint(free) != fmt.Sprint(pads) {
		t.Errorf("free spans %X, want the padding %X", free, pads)
	}

	mustFree(t, z, a)
	pads = nil

	if x := mustAllocate(t, z, 16); x != a || pads != nil {
		t.Errorf("a block at %X from the free span (want %X) reported padding %X", x, a, pads)
	}

	mustCheck(t, z)
}
