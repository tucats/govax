package vmheap

import (
	"errors"
	"math/rand"
	"testing"
)

// Quick fit: blocks of a list's size go to the list when freed and come
// back from it, last freed first; other sizes are first fit.
func TestQuickFit(t *testing.T) {
	z, _ := newZone(t, Attributes{Algorithm: QuickFit, Lists: 4, Smallest: 16, BlockSize: 8, Alignment: 8, ExtendPages: 4})

	// The lists hold 16, 24, 32, and 40 bytes.
	a := mustAllocate(t, z, 16)
	b := mustAllocate(t, z, 20) // 24
	c := mustAllocate(t, z, 48) // no list
	d := mustAllocate(t, z, 16)

	mustFree(t, z, a)
	mustFree(t, z, d)
	mustFree(t, z, b)

	if z.listed() != 3 || z.Blocks() != 1 {
		t.Fatalf("%d listed, %d blocks; want 3 and 1", z.listed(), z.Blocks())
	}

	// A listed block is still part of its area: a's space isn't reused
	// by a first fit request that would fit there.
	if e := mustAllocate(t, z, 8); e == a {
		t.Errorf("an 8-byte block took listed block %X's space", a)
	}

	if x := mustAllocate(t, z, 16); x != d {
		t.Errorf("16 bytes at %X, want the last freed, %X", x, d)
	}

	if x := mustAllocate(t, z, 16); x != a {
		t.Errorf("16 bytes at %X, want %X", x, a)
	}

	// 48 bytes has no list: freed, its space is the area's again, and a
	// first fit request takes it.
	mustFree(t, z, c)

	if x := mustAllocate(t, z, 48); x != c {
		t.Errorf("48 bytes at %X, want the freed %X", x, c)
	}

	// Freeing a listed block again changes nothing.
	mustFree(t, z, a)

	if _, err := z.Free(a); !errors.Is(err, ErrFreedAgain) {
		t.Errorf("second Free(%X) = %v, want ErrFreedAgain", a, err)
	}

	mustCheck(t, z)
}

// Frequent sizes: each list takes the size of the first block freed to
// it while empty; a size no list has, with none empty, takes over the
// next list in turn, whose blocks go back to their area (as VMS 7.1
// does: testdata/lib59).
func TestFrequentSizes(t *testing.T) {
	z, _ := newZone(t, Attributes{Algorithm: FrequentSizes, Lists: 2, BlockSize: 8, Alignment: 8, ExtendPages: 4})

	a := mustAllocate(t, z, 16)
	b := mustAllocate(t, z, 32)
	c := mustAllocate(t, z, 48)

	mustFree(t, z, a)
	mustFree(t, z, b)
	mustFree(t, z, c) // no empty list: the first list takes 48

	if z.listed() != 2 || z.lists[0].n != 48 || z.lists[1].n != 32 {
		t.Fatalf("lists %+v, want 48 and 32 with a block each", z.lists)
	}

	if x := mustAllocate(t, z, 48); x != c {
		t.Errorf("48 bytes at %X, want the listed %X", x, c)
	}

	if x := mustAllocate(t, z, 16); x != a {
		t.Errorf("16 bytes at %X, want first fit's %X", x, a)
	}

	// Emptied, a list keeps its size.
	if x := mustAllocate(t, z, 32); x != b {
		t.Errorf("32 bytes at %X, want the listed %X", x, b)
	}

	mustFree(t, z, c)

	if z.lists[0].n != 48 || len(z.lists[0].addrs) != 1 || z.lists[1].n != 32 {
		t.Errorf("lists %+v, want 48 bytes with one block, then 32", z.lists)
	}
}

// Fixed size: every block is the zone's size; larger requests are
// ErrBadSize; freed blocks are reused last in, first out.
func TestFixedSize(t *testing.T) {
	z, _ := newZone(t, Attributes{Algorithm: FixedSize, FixedSize: 20, BlockSize: 8, Alignment: 8, ExtendPages: 1})

	if _, err := z.Allocate(21); !errors.Is(err, ErrBadSize) {
		t.Errorf("Allocate(21) = %v, want ErrBadSize", err)
	}

	a := mustAllocate(t, z, 1)
	b := mustAllocate(t, z, 20)

	if b-a != 24 {
		t.Errorf("blocks %d apart, want 24", b-a)
	}

	if n, _ := z.BlockSize(a); n != 24 {
		t.Errorf("block size %d, want 24", n)
	}

	mustFree(t, z, a)
	mustFree(t, z, b)

	if _, err := z.Free(b); !errors.Is(err, ErrFreedAgain) {
		t.Errorf("second Free = %v, want ErrFreedAgain", err)
	}

	if x := mustAllocate(t, z, 5); x != b {
		t.Errorf("block at %X, want the last freed %X", x, b)
	}
}

// Boundary tags: a longword before and after each block, the block's
// size a multiple of the block size with them, its address aligned.
func TestBoundaryTags(t *testing.T) {
	z, _ := newZone(t, Attributes{BoundaryTags: true, BlockSize: 8, Alignment: 4, ExtendPages: 1})

	a := mustAllocate(t, z, 1) // 1 + 8 bytes of tags: 16
	b := mustAllocate(t, z, 8) // 16
	c := mustAllocate(t, z, 9) // 24

	if a%PageSize != TagSize || b-a != 16 || c-b != 16 {
		t.Errorf("blocks at offset %d, then +%d, +%d; want %d, +16, +16", a%PageSize, b-a, c-a, TagSize)
	}

	if n, _ := z.BlockSize(a); n != 8 {
		t.Errorf("BlockSize(a) = %d, want 8", n)
	}

	before, after, value, ok := z.Tags(c)
	if !ok || before != c-TagSize || after != c+16 || value != 24 {
		t.Errorf("Tags(c) = %X, %X, %d, %v; want %X, %X, 24", before, after, value, ok, c-TagSize, c+16)
	}

	// Alignment 8: each block starts 4 bytes before an 8-byte boundary,
	// so its usable bytes are aligned.
	z8, _ := newZone(t, Attributes{BoundaryTags: true, BlockSize: 8, Alignment: 8, ExtendPages: 1})
	x := mustAllocate(t, z8, 4)
	y := mustAllocate(t, z8, 4)

	if x%8 != 0 || y-x != 16 {
		t.Errorf("8-aligned tagged blocks at %X, %X", x, y)
	}

	untagged, _ := newZone(t, smallAreas)
	if _, _, _, ok := untagged.Tags(mustAllocate(t, untagged, 8)); ok {
		t.Error("Tags of an untagged zone's block succeeded")
	}
}

// An initial area is made with the zone, and the page limit bounds the
// zone's growth: the last area is what's left under the limit.
func TestInitialSizeAndPageLimit(t *testing.T) {
	z, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 4, InitialPages: 2, PageLimit: 3})

	if z.Pages() != 2 || z.Areas() != 1 {
		t.Fatalf("%d pages in %d areas at creation, want 2 in 1", z.Pages(), z.Areas())
	}

	mustAllocate(t, z, 2*PageSize-AreaHeader) // fills the initial area
	mustAllocate(t, z, 100)                   // a 1-page area: the limit's rest

	if z.Pages() != 3 {
		t.Errorf("%d pages, want 3", z.Pages())
	}

	if _, err := z.Allocate(PageSize); !errors.Is(err, ErrPageLimit) {
		t.Errorf("past the limit: %v, want ErrPageLimit", err)
	}

	mustCheck(t, z)

	// A request larger than the limit leaves room for is refused at once.
	z2, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 1, InitialPages: 1, PageLimit: 2})
	if _, err := z2.Allocate(2 * PageSize); !errors.Is(err, ErrPageLimit) {
		t.Errorf("a request beyond the limit: %v, want ErrPageLimit", err)
	}
}

// NoExtend keeps the zone to its first area: past it is ErrPageLimit,
// as VMS 7.1 answers (testdata/lib59).
func TestNoExtend(t *testing.T) {
	z, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 8, InitialPages: 1, NoExtend: true})

	mustAllocate(t, z, 400)

	if _, err := z.Allocate(200); !errors.Is(err, ErrPageLimit) {
		t.Errorf("past the initial area: %v, want ErrPageLimit", err)
	}

	if z.Areas() != 1 {
		t.Errorf("%d areas, want 1", z.Areas())
	}
}

// New areas go to the head of the area list, so they're searched first;
// with TailLarge, one larger than ExtendPages goes to its end.
func TestAreaOrder(t *testing.T) {
	for _, tail := range []bool{false, true} {
		z, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 1, TailLarge: tail})

		mustAllocate(t, z, 400)                   // area 1
		big := mustAllocate(t, z, 3*PageSize+400) // area 2, 4 pages, full
		mustAllocate(t, z, 400)                   // area 3: neither has room

		bigFirst := z.areaOf(big) == z.areas[0]
		bigLast := z.areaOf(big) == z.areas[len(z.areas)-1]

		if tail && !bigLast || !tail && bigLast {
			t.Errorf("TailLarge %v: the large area is %s", tail, map[bool]string{true: "last", false: "not last"}[bigLast])
		}

		if bigFirst {
			t.Errorf("TailLarge %v: the large area is first, ahead of the newest", tail)
		}
	}
}

// Reset frees every block, listed or not, and keeps the pages: the next
// block is where the first was.
func TestReset(t *testing.T) {
	z, _ := newZone(t, Attributes{Algorithm: QuickFit, Lists: 2, BlockSize: 8, Alignment: 8, ExtendPages: 1, ExtendArea: true})

	first := mustAllocate(t, z, 8)
	second := mustAllocate(t, z, 100)
	mustAllocate(t, z, 2*PageSize)
	mustFree(t, z, first) // listed

	pages := z.Pages()
	freed := z.Reset()
	mustCheck(t, z)

	if len(freed) != 2 || freed[0].Addr != second || freed[0].End-freed[0].Start != 104 {
		t.Errorf("Reset freed %+v, want 2 blocks starting with %X (104 bytes)", freed, second)
	}

	if z.Pages() != pages || z.Blocks() != 0 || z.listed() != 0 || z.BytesInUse() != 0 {
		t.Errorf("after Reset: %d pages (want %d), %d blocks, %d listed, %d bytes", z.Pages(), pages, z.Blocks(), z.listed(), z.BytesInUse())
	}

	if x := mustAllocate(t, z, 8); x != first {
		t.Errorf("first block after Reset at %X, want %X", x, first)
	}
}

// Delete gives every page back to the pool, which can lend them again.
func TestDelete(t *testing.T) {
	p := NewPagePool(newRegion())

	z, err := NewZone(p, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 2, InitialPages: 3})
	if err != nil {
		t.Fatal(err)
	}

	a := mustAllocate(t, z, 3*PageSize)

	if freed, _ := z.Delete(); len(freed) != 1 || freed[0].Addr != a {
		t.Errorf("Delete freed %+v, want the block at %X", freed, a)
	}

	if p.HeldPages() != 0 || p.FreePages() != p.OwnedPages() {
		t.Errorf("%d held, %d of %d free after Delete; want none held, all free", p.HeldPages(), p.FreePages(), p.OwnedPages())
	}

	mustCheck(t, p)

	if err := p.Reclaim(0x10000, 1); !errors.Is(err, ErrBadAddress) {
		t.Errorf("Reclaim of free pages = %v, want ErrBadAddress", err)
	}
}

func TestNewZoneRejectsBadAlgorithmAttributes(t *testing.T) {
	base := Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 1}

	for _, change := range []func(*Attributes){
		func(a *Attributes) { a.Algorithm = QuickFit },
		func(a *Attributes) { a.Algorithm = FrequentSizes },
		func(a *Attributes) { a.Algorithm = FixedSize },
		func(a *Attributes) { a.Algorithm = 9 },
		func(a *Attributes) { a.InitialPages, a.PageLimit = 3, 2 },
	} {
		attr := base
		change(&attr)

		if _, err := NewZone(NewPagePool(newRegion()), attr); err == nil {
			t.Errorf("NewZone(%+v) succeeded", attr)
		}
	}

	// An initial area the region can't hold.
	r := &fakeRegion{next: 0x10000, limit: 0x10400}
	if _, err := NewZone(NewPagePool(r), Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 1, InitialPages: 3}); !errors.Is(err, ErrNoMemory) {
		t.Errorf("NewZone with an initial area too large = %v, want ErrNoMemory", err)
	}
}

// TestAlgorithmsRandomWorkload runs TestZoneRandomWorkload's mix under
// each algorithm, and with tags, freeing some blocks twice, with the
// checker after every step; then resets the zone and deletes it.
func TestAlgorithmsRandomWorkload(t *testing.T) {
	for _, attr := range []Attributes{
		{Algorithm: QuickFit, Lists: 16, Smallest: 8, BlockSize: 8, Alignment: 8, ExtendPages: 2},
		{Algorithm: QuickFit, Lists: 8, Smallest: 40, BlockSize: 16, Alignment: 4, ExtendPages: 1, ExtendArea: true, BoundaryTags: true},
		{Algorithm: FrequentSizes, Lists: 4, BlockSize: 8, Alignment: 8, ExtendPages: 2},
		{Algorithm: FixedSize, FixedSize: 300, BlockSize: 8, Alignment: 8, ExtendPages: 4},
		{BoundaryTags: true, BlockSize: 8, Alignment: 4, ExtendPages: 2, ExtendArea: true},
		{BoundaryTags: true, BlockSize: 32, Alignment: 64, ExtendPages: 1, TailLarge: true},
	} {
		p := NewPagePool(newRegion())

		z, err := NewZone(p, attr)
		if err != nil {
			t.Fatal(err)
		}

		rnd := rand.New(rand.NewSource(int64(attr.Algorithm)*7 + int64(attr.BlockSize)))
		live := map[uint32]bool{}

		var freed []uint32

		for step := range 3000 {
			switch {
			case len(freed) > 0 && rnd.Intn(20) == 0:
				addr := freed[rnd.Intn(len(freed))]
				if live[addr] {
					break
				}

				if _, err := z.Free(addr); !errors.Is(err, ErrFreedAgain) && !errors.Is(err, ErrBadAddress) {
					t.Fatalf("%+v step %d: Free of freed %X = %v", attr, step, addr, err)
				}

				mustCheck(t, z)

			case len(live) > 0 && rnd.Intn(5) < 2:
				for addr := range live {
					mustFree(t, z, addr)
					delete(live, addr)
					freed = append(freed, addr)

					break
				}

			default:
				size := uint32(1 + rnd.Intn(300))
				if rnd.Intn(50) == 0 && attr.Algorithm != FixedSize {
					size = uint32(1 + rnd.Intn(5*PageSize))
				}

				addr := mustAllocate(t, z, size)

				if n, ok := z.BlockSize(addr); !ok || n < size {
					t.Fatalf("%+v step %d: block %X holds %d bytes, want at least %d", attr, step, addr, n, size)
				}

				live[addr] = true
			}
		}

		if freed := z.Reset(); len(freed) != len(live) {
			t.Errorf("%+v: Reset freed %d blocks, %d live", attr, len(freed), len(live))
		}

		mustCheck(t, z)
		_, _ = z.Delete()
		mustCheck(t, p)

		if p.HeldPages() != 0 {
			t.Errorf("%+v: %d pages held after Delete", attr, p.HeldPages())
		}
	}
}
