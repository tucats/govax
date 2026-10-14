package vmheap

import (
	"errors"
	"math/rand"
	"testing"
)

// fakeRegion stands in for the process's P0 region: Expand hands out
// pages upward from next, up to limit.
type fakeRegion struct {
	next, limit uint32
	calls       []uint32 // each Expand's page count
}

func (r *fakeRegion) End() uint32 { return r.next }

func (r *fakeRegion) Expand(pages uint32) (uint32, bool) {
	r.calls = append(r.calls, pages)

	if uint64(r.next)+uint64(pages)*PageSize > uint64(r.limit) {
		return 0, false
	}

	addr := r.next
	r.next += pages * PageSize

	return addr, true
}

func newRegion() *fakeRegion {
	return &fakeRegion{next: 0x10000, limit: 0x40000000}
}

// defaultAttributes are Table 5-3's default zone, with small areas so
// the tests make several.
var smallAreas = Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 4, ExtendArea: true}

func newZone(t *testing.T, attr Attributes) (*Zone, *fakeRegion) {
	t.Helper()

	r := newRegion()

	z, err := NewZone(NewPagePool(r), attr)
	if err != nil {
		t.Fatal(err)
	}

	return z, r
}

func mustCheck(t *testing.T, c interface{ Check() error }) {
	t.Helper()

	if err := c.Check(); err != nil {
		t.Fatal(err)
	}
}

func mustAllocate(t *testing.T, z *Zone, size uint32) uint32 {
	t.Helper()

	addr, err := z.Allocate(size)
	if err != nil {
		t.Fatalf("Allocate(%d): %v", size, err)
	}

	mustCheck(t, z)

	return addr
}

func mustFree(t *testing.T, z *Zone, addr uint32) {
	t.Helper()

	if _, err := z.Free(addr); err != nil {
		t.Fatalf("Free(%08X): %v", addr, err)
	}

	mustCheck(t, z)
}

func TestPagesFirstFitAndReuse(t *testing.T) {
	r := newRegion()
	p := NewPagePool(r)

	a, _ := p.Allocate(1)
	b, _ := p.Allocate(2)
	c, _ := p.Allocate(1)

	if a != 0x10000 || b != a+PageSize || c != b+2*PageSize {
		t.Fatalf("pages at %X %X %X, want consecutive from 10000", a, b, c)
	}

	if err := p.Free(2, b); err != nil {
		t.Fatal(err)
	}

	mustCheck(t, p)

	// The freed pair is reused, lowest first, before the region grows.
	d, _ := p.Allocate(1)
	if d != b {
		t.Errorf("reused page at %X, want %X", d, b)
	}

	if len(r.calls) != 3 {
		t.Errorf("region expanded %d times, want 3", len(r.calls))
	}

	mustCheck(t, p)
}

func TestPagesPartialAndCombinedFrees(t *testing.T) {
	p := NewPagePool(newRegion())

	a, _ := p.Allocate(4)
	b, _ := p.Allocate(2) // contiguous with a

	// Free part of a group, then the rest of it with the next group in
	// one call: the manual allows both.
	if err := p.Free(1, a+PageSize); err != nil {
		t.Fatal(err)
	}

	if err := p.Free(2, a+2*PageSize); err != nil {
		t.Fatal(err)
	}

	if err := p.Free(2, b); err != nil {
		t.Fatal(err)
	}

	mustCheck(t, p)

	if p.FreePages() != 5 || len(p.free) != 1 {
		t.Fatalf("%d pages free in %d spans, want 5 in 1", p.FreePages(), len(p.free))
	}

	if err := p.Free(1, a); err != nil {
		t.Fatal(err)
	}

	if p.FreePages() != 6 || len(p.free) != 1 {
		t.Errorf("%d pages free in %d spans, want 6 in 1", p.FreePages(), len(p.free))
	}

	mustCheck(t, p)
}

func TestPagesBadFrees(t *testing.T) {
	p := NewPagePool(newRegion())

	a, _ := p.Allocate(2)
	_ = p.Free(1, a)

	cases := []struct {
		name string
		n    uint32
		addr uint32
		want error
	}{
		{"freed already", 1, a, ErrBadAddress},
		{"partly freed already", 2, a, ErrBadAddress},
		{"not page aligned", 1, a + PageSize + 4, ErrBadAddress},
		{"not the pool's", 1, 0x200000, ErrBadAddress},
		{"past the pool's pages", 2, a + PageSize, ErrBadAddress},
		{"count 0", 0, a + PageSize, ErrBadSize},
		{"wraps", 0x100, 0xFFFFFE00, ErrBadAddress},
	}

	for _, c := range cases {
		if err := p.Free(c.n, c.addr); !errors.Is(err, c.want) {
			t.Errorf("%s: Free(%d, %X) = %v, want %v", c.name, c.n, c.addr, err, c.want)
		}
	}

	// None of them changed anything: the second page is still allocated.
	if p.FreePages() != 1 {
		t.Errorf("%d pages free, want 1", p.FreePages())
	}

	mustCheck(t, p)
}

// A zone's pages are held: freeing them as pages is refused, alone or
// with a program's pages beside them, and the program's own pages around
// them free as usual.
func TestPagesHeldByZone(t *testing.T) {
	p := NewPagePool(newRegion())

	before, _ := p.Allocate(1)

	z, err := NewZone(p, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 2})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := z.Allocate(16); err != nil {
		t.Fatal(err)
	}

	after, _ := p.Allocate(1)
	areaBase := before + PageSize

	if after != areaBase+2*PageSize || p.HeldPages() != 2 {
		t.Fatalf("program pages at %X and %X, %d held; want the area between them", before, after, p.HeldPages())
	}

	for _, c := range []struct {
		name string
		n    uint32
		addr uint32
	}{
		{"the area", 2, areaBase},
		{"the area's second page", 1, areaBase + PageSize},
		{"a program page and the area", 3, before},
		{"the area and a program page", 3, areaBase},
	} {
		if err := p.Free(c.n, c.addr); !errors.Is(err, ErrBadAddress) {
			t.Errorf("%s: Free(%d, %X) = %v, want ErrBadAddress", c.name, c.n, c.addr, err)
		}
	}

	if err := p.Free(1, before); err != nil {
		t.Errorf("Free(1, %X): %v", before, err)
	}

	if err := p.Free(1, after); err != nil {
		t.Errorf("Free(1, %X): %v", after, err)
	}

	if p.HeldPages() != 2 || p.FreePages() != 2 {
		t.Errorf("%d held, %d free; want 2 and 2", p.HeldPages(), p.FreePages())
	}

	mustCheck(t, p)
	mustCheck(t, z)
}

func TestPagesGrowJoinsTopRun(t *testing.T) {
	r := newRegion()
	p := NewPagePool(r)

	a, _ := p.Allocate(3)
	_ = p.Free(2, a+PageSize) // the top two pages are free

	// Five pages: the top two free ones and three new.
	b, err := p.Allocate(5)
	if err != nil {
		t.Fatal(err)
	}

	if b != a+PageSize {
		t.Errorf("5 pages at %X, want %X (joining the free top)", b, a+PageSize)
	}

	if got := r.calls[len(r.calls)-1]; got != 3 {
		t.Errorf("region asked for %d pages, want 3", got)
	}

	mustCheck(t, p)
}

func TestPagesRegionMovedByOthers(t *testing.T) {
	r := newRegion()
	p := NewPagePool(r)

	a, _ := p.Allocate(2)
	_ = p.Free(1, a+PageSize)

	// Something else ($EXPREG from the program) takes the next page, so
	// the pool's free top no longer ends at the region's end.
	r.next += PageSize

	b, err := p.Allocate(3)
	if err != nil {
		t.Fatal(err)
	}

	if b != a+3*PageSize {
		t.Errorf("3 pages at %X, want %X (after the page others took)", b, a+3*PageSize)
	}

	mustCheck(t, p)
}

// TestPagesCalls: the pool counts the requests it satisfies, a zone's
// (Lend, Reclaim) among them, and not the ones it refuses; AllocatedPages
// counts a zone's pages with the rest.
func TestPagesCalls(t *testing.T) {
	r := &fakeRegion{next: 0x10000, limit: 0x10000 + 8*PageSize}
	p := NewPagePool(r)

	want := func(gets, frees, pages uint32) {
		t.Helper()

		if g, f := p.Calls(); g != gets || f != frees || p.AllocatedPages() != pages {
			t.Errorf("Calls() = %d, %d, %d pages allocated; want %d, %d, %d",
				g, f, p.AllocatedPages(), gets, frees, pages)
		}
	}

	a, _ := p.Allocate(3)
	want(1, 0, 3)

	_, _ = p.Allocate(0)
	_, _ = p.Allocate(9)
	_ = p.Free(1, a+5*PageSize)
	want(1, 0, 3)

	_ = p.Free(1, a+PageSize)
	want(1, 1, 2)

	z, err := NewZone(p, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 2})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := z.Allocate(16); err != nil {
		t.Fatal(err)
	}

	want(2, 1, 4)

	if _, err := z.Delete(); err != nil {
		t.Fatal(err)
	}

	want(2, 2, 2)
	mustCheck(t, p)
}

func TestPagesNoMemory(t *testing.T) {
	r := &fakeRegion{next: 0x10000, limit: 0x10000 + 4*PageSize}
	p := NewPagePool(r)

	if _, err := p.Allocate(5); !errors.Is(err, ErrNoMemory) {
		t.Errorf("Allocate(5) = %v, want ErrNoMemory", err)
	}

	if p.OwnedPages() != 0 {
		t.Errorf("pool owns %d pages after a failed request, want 0", p.OwnedPages())
	}

	if _, err := p.Allocate(0); !errors.Is(err, ErrBadSize) {
		t.Errorf("Allocate(0) = %v, want ErrBadSize", err)
	}

	if _, err := p.Allocate(0xFFFFFFFF); !errors.Is(err, ErrNoMemory) {
		t.Errorf("Allocate(FFFFFFFF) = %v, want ErrNoMemory", err)
	}

	if _, err := p.Allocate(4); err != nil {
		t.Errorf("Allocate(4) = %v, want the region's last pages", err)
	}
}

func TestZoneRoundingAndAlignment(t *testing.T) {
	z, _ := newZone(t, smallAreas)

	a := mustAllocate(t, z, 1)
	b := mustAllocate(t, z, 9)
	c := mustAllocate(t, z, 16)

	if a%PageSize != AreaHeader {
		t.Errorf("first block at offset %d in its page, want %d (after the area's control block)", a%PageSize, AreaHeader)
	}

	if b-a != 8 || c-b != 16 {
		t.Errorf("blocks at +0, +%d, +%d; want +0, +8, +24", b-a, c-a)
	}

	if n, ok := z.BlockSize(b); !ok || n != 16 {
		t.Errorf("BlockSize(b) = %d, %v; want 16, true", n, ok)
	}

	// A block size of 8 with alignment 64: the larger governs both.
	z64, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 64, ExtendPages: 1})
	x := mustAllocate(t, z64, 1)
	y := mustAllocate(t, z64, 65)
	w := mustAllocate(t, z64, 1)

	if x%64 != 0 || y-x != 64 || w-y != 128 {
		t.Errorf("64-aligned blocks at %X, %X, %X", x, y, w)
	}
}

func TestZoneBadRequests(t *testing.T) {
	z, _ := newZone(t, smallAreas)

	if _, err := z.Allocate(0); !errors.Is(err, ErrBadSize) {
		t.Errorf("Allocate(0) = %v, want ErrBadSize", err)
	}

	for _, size := range []uint32{0xFFFFFFF0, 0xFFFFFFFF, maxBlock + 1} {
		if _, err := z.Allocate(size); !errors.Is(err, ErrNoMemory) {
			t.Errorf("Allocate(%X) = %v, want ErrNoMemory", size, err)
		}
	}

	a := mustAllocate(t, z, 32)

	for _, addr := range []uint32{a + 8, a - 8, 0, 0x7FFFFFF8} {
		if _, err := z.Free(addr); !errors.Is(err, ErrBadAddress) {
			t.Errorf("Free(%X) = %v, want ErrBadAddress", addr, err)
		}
	}

	mustFree(t, z, a)

	if _, err := z.Free(a); !errors.Is(err, ErrBadAddress) {
		t.Errorf("second Free(%X) = %v, want ErrBadAddress", a, err)
	}
}

func TestZoneMergesBothNeighbors(t *testing.T) {
	z, _ := newZone(t, smallAreas)

	a := mustAllocate(t, z, 48)
	b := mustAllocate(t, z, 48)
	c := mustAllocate(t, z, 48)
	d := mustAllocate(t, z, 48) // keeps c's free space off the unallocated end

	mustFree(t, z, a)
	mustFree(t, z, c)
	mustFree(t, z, b) // between two free blocks

	ar := z.areas[0]
	if len(ar.free) != 1 || ar.free[0] != (span{a, d}) {
		t.Fatalf("free spans %v, want one, %X-%X", ar.free, a, d)
	}

	// Freeing the last block returns everything to the unallocated end.
	mustFree(t, z, d)

	if len(ar.free) != 0 || ar.top != ar.first {
		t.Errorf("after freeing all: %d free spans, top %X; want none, %X", len(ar.free), ar.top, ar.first)
	}
}

func TestZoneFirstFitByAddress(t *testing.T) {
	z, _ := newZone(t, smallAreas)

	var blocks []uint32
	for range 6 {
		blocks = append(blocks, mustAllocate(t, z, 64))
	}

	// Free the fifth, then the second: a request that fits either takes
	// the lower, whatever the order they were freed in.
	mustFree(t, z, blocks[4])
	mustFree(t, z, blocks[1])

	if got := mustAllocate(t, z, 24); got != blocks[1] {
		t.Errorf("allocated at %X, want %X (the lowest free space)", got, blocks[1])
	}

	// What's left of that hole (40 bytes) is used next for what fits it.
	if got := mustAllocate(t, z, 40); got != blocks[1]+24 {
		t.Errorf("allocated at %X, want %X (the rest of the hole)", got, blocks[1]+24)
	}
}

func TestZoneExactFitIsntOverlapped(t *testing.T) {
	// The heap this package replaced marked a free block that fitted a
	// request with the request's rounded size, which could be larger, and
	// so overlapped the next block. Here a hole of exactly the rounded
	// size is used without touching its neighbor.
	z, _ := newZone(t, smallAreas)

	a := mustAllocate(t, z, 16)
	b := mustAllocate(t, z, 16)
	mustFree(t, z, a)

	if got := mustAllocate(t, z, 16); got != a {
		t.Errorf("allocated at %X, want %X", got, a)
	}

	if n, _ := z.BlockSize(a); a+n > b {
		t.Errorf("block %X+%d runs into %X", a, n, b)
	}
}

func TestZoneAreas(t *testing.T) {
	z, r := newZone(t, smallAreas)

	// A request larger than ExtendPages gets an area of its own size.
	big := mustAllocate(t, z, 10*PageSize)
	if z.Pages() < 11 {
		t.Errorf("zone has %d pages after a 10-page block, want at least 11", z.Pages())
	}

	// The next area's pages adjoin the first, so ExtendArea appends them.
	mustAllocate(t, z, 3*PageSize)

	if z.Areas() != 1 {
		t.Errorf("%d areas, want 1 (extended)", z.Areas())
	}

	// Pages that don't adjoin (the program took the next page itself)
	// make a new area.
	r.next += PageSize
	mustAllocate(t, z, 4*PageSize)

	if z.Areas() != 2 {
		t.Errorf("%d areas, want 2", z.Areas())
	}

	// Without ExtendArea, every extension is an area.
	z2, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 1})
	mustAllocate(t, z2, 400)
	mustAllocate(t, z2, 400)

	if z2.Areas() != 2 {
		t.Errorf("%d areas without ExtendArea, want 2", z2.Areas())
	}

	mustFree(t, z, big)
}

func TestZoneNoMemory(t *testing.T) {
	r := &fakeRegion{next: 0x10000, limit: 0x10000 + 8*PageSize}

	z, err := NewZone(NewPagePool(r), smallAreas)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := z.Allocate(9 * PageSize); !errors.Is(err, ErrNoMemory) {
		t.Errorf("Allocate(9 pages) = %v, want ErrNoMemory", err)
	}

	if z.Areas() != 0 || z.Blocks() != 0 {
		t.Errorf("%d areas, %d blocks after a failed request, want none", z.Areas(), z.Blocks())
	}

	mustCheck(t, z)
}

func TestNewZoneRejectsBadAttributes(t *testing.T) {
	for _, attr := range []Attributes{
		{BlockSize: 12, Alignment: 8, ExtendPages: 1},
		{BlockSize: 8, Alignment: 0, ExtendPages: 1},
		{BlockSize: 8, Alignment: 8},
	} {
		if _, err := NewZone(NewPagePool(newRegion()), attr); err == nil {
			t.Errorf("NewZone(%+v) succeeded", attr)
		}
	}
}

// TestZoneRandomWorkload allocates and frees at random, with the
// checker after every step, under a few attribute sets; then frees
// everything and expects each area to be one unallocated end again.
func TestZoneRandomWorkload(t *testing.T) {
	for _, attr := range []Attributes{
		smallAreas,
		{BlockSize: 8, Alignment: 8, ExtendPages: 128, ExtendArea: true},
		{BlockSize: 16, Alignment: 64, ExtendPages: 2},
		{BlockSize: 512, Alignment: 4, ExtendPages: 1, ExtendArea: true},
	} {
		z, _ := newZone(t, attr)
		rnd := rand.New(rand.NewSource(int64(attr.BlockSize*1000 + attr.Alignment)))
		live := map[uint32]uint32{}

		for step := range 4000 {
			if len(live) > 0 && rnd.Intn(5) < 2 {
				for addr := range live {
					mustFree(t, z, addr)
					delete(live, addr)

					break
				}

				continue
			}

			size := uint32(1 + rnd.Intn(300))
			if rnd.Intn(50) == 0 {
				size = uint32(1 + rnd.Intn(5*PageSize))
			}

			addr := mustAllocate(t, z, size)

			if n, ok := z.BlockSize(addr); !ok || n < size {
				t.Fatalf("%+v step %d: block %X holds %d bytes, want at least %d", attr, step, addr, n, size)
			}

			live[addr] = size
		}

		for addr := range live {
			mustFree(t, z, addr)
		}

		for _, a := range z.areas {
			if len(a.free) != 0 || a.top != a.first {
				t.Errorf("%+v: area %X not empty after freeing all: %d spans, top %X, first %X",
					attr, a.base, len(a.free), a.top, a.first)
			}
		}

		if z.BytesInUse() != 0 {
			t.Errorf("%+v: %d bytes in use after freeing all", attr, z.BytesInUse())
		}
	}
}

// TestZoneFragmentationBound is the workload that showed the old heap's
// faults (docs/PHASE-59, the survey's review): rounds of 50 blocks of 8
// to 207 bytes, 90% of the live ones then freed in random order. With
// every free merged and first fit by address, the zone stays within a
// small multiple of what's live at its peak.
func TestZoneFragmentationBound(t *testing.T) {
	z, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 1, ExtendArea: true})
	rnd := rand.New(rand.NewSource(1))

	var (
		live []uint32
		peak uint32
	)

	for range 200 {
		for range 50 {
			live = append(live, mustAllocate(t, z, uint32(8+rnd.Intn(200))))
		}

		peak = max(peak, z.BytesInUse())

		rnd.Shuffle(len(live), func(i, j int) { live[i], live[j] = live[j], live[i] })

		for _, a := range live[:len(live)*9/10] {
			mustFree(t, z, a)
		}

		live = live[len(live)*9/10:]
	}

	if limit := 2*peak + 2*PageSize; z.Pages()*PageSize > limit {
		t.Errorf("zone grew to %d bytes for a peak of %d in use (limit %d)", z.Pages()*PageSize, peak, limit)
	}

	if z.Areas() != 1 {
		t.Errorf("%d areas, want 1 (each extension adjoins)", z.Areas())
	}
}

// TestPagesRandomWorkload allocates page groups and frees them, whole,
// in parts, and across neighbors, with the checker after each step.
func TestPagesRandomWorkload(t *testing.T) {
	p := NewPagePool(newRegion())
	rnd := rand.New(rand.NewSource(7))

	allocated := map[uint32]bool{} // each allocated page's address

	for range 3000 {
		if len(allocated) > 0 && rnd.Intn(2) == 0 {
			// Free a run of allocated pages starting at one of them.
			var start uint32
			for a := range allocated {
				start = a

				break
			}

			n := uint32(0)
			for allocated[start+n*PageSize] && n < uint32(1+rnd.Intn(6)) {
				delete(allocated, start+n*PageSize)
				n++
			}

			if err := p.Free(n, start); err != nil {
				t.Fatalf("Free(%d, %X): %v", n, start, err)
			}
		} else {
			n := uint32(1 + rnd.Intn(8))

			addr, err := p.Allocate(n)
			if err != nil {
				t.Fatal(err)
			}

			for i := range n {
				if allocated[addr+i*PageSize] {
					t.Fatalf("page %X handed out twice", addr+i*PageSize)
				}

				allocated[addr+i*PageSize] = true
			}
		}

		mustCheck(t, p)

		if want := p.OwnedPages() - uint32(len(allocated)); p.FreePages() != want {
			t.Fatalf("%d pages free, want %d", p.FreePages(), want)
		}
	}
}
