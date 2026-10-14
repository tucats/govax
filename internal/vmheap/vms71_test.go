package vmheap

import (
	"errors"
	"testing"
)

// The rules VMS 7.1 showed (docs/PHASE-59, subtask 10; testdata/lib59),
// each on its own.

// FreeRange trusts the size it's given: part of a block, a block and
// its neighbor, or bytes inside a block; misaligned bytes, bytes past an
// area's top, and bytes free already are refused, the last as
// ErrDoubleFree.
func TestFreeRange(t *testing.T) {
	z, _ := newZone(t, smallAreas)

	a := mustAllocate(t, z, 64)
	b := mustAllocate(t, z, 32)
	c := mustAllocate(t, z, 32)
	mustAllocate(t, z, 8)

	// 32 of a's 64 bytes: the rest stays a block of the program's.
	if n, err := z.FreeRange(a, 32); err != nil || n != 32 {
		t.Fatalf("FreeRange(a, 32) = %d, %v", n, err)
	}

	mustCheck(t, z)

	if n, ok := z.BlockSize(a + 32); !ok || n != 32 {
		t.Errorf("a's last 32 bytes: %d, %v; want a block of 32", n, ok)
	}

	// 64 bytes from b: b and c both.
	if _, err := z.FreeRange(b, 64); err != nil {
		t.Fatalf("FreeRange(b, 64) = %v", err)
	}

	mustCheck(t, z)

	if _, ok := z.BlockSize(c); ok {
		t.Error("c survived a free of 64 bytes from b")
	}

	if _, err := z.FreeRange(b, 8); !errors.Is(err, ErrDoubleFree) || !errors.Is(err, ErrBadAddress) {
		t.Errorf("a double free: %v, want ErrDoubleFree (an ErrBadAddress)", err)
	}

	for _, addr := range []uint32{a + 4, a + 0x10000} {
		if _, err := z.FreeRange(addr, 8); !errors.Is(err, ErrBadAddress) || errors.Is(err, ErrDoubleFree) {
			t.Errorf("FreeRange(%X) = %v, want ErrBadAddress", addr, err)
		}
	}

	if _, err := z.FreeRange(a+32, 0); !errors.Is(err, ErrBadSize) {
		t.Errorf("FreeRange of 0 bytes = %v, want ErrBadSize", err)
	}

	mustCheck(t, z)
}

// A freed block next to an area's unallocated end stays free space; the
// end doesn't grow back.
func TestFreeSpaceStaysFree(t *testing.T) {
	z, _ := newZone(t, smallAreas)

	a := mustAllocate(t, z, 16)
	mustFree(t, z, a)

	r := z.AreaReports()[0]
	if len(r.Free) != 1 || r.Free[0] != (Span{a, a + 16}) || r.Top != a+16 {
		t.Errorf("area %+v; want the block a free span below top", r)
	}

	if x := mustAllocate(t, z, 16); x != a {
		t.Errorf("16 bytes at %X, want the free span's %X", x, a)
	}
}

// A lookaside block freed twice, FreeAgain'd, links the list to itself:
// the list gives that block to every request, and Check reports it.
func TestFreeAgainLoops(t *testing.T) {
	z, _ := newZone(t, Attributes{Algorithm: FixedSize, FixedSize: 16, BlockSize: 8, Alignment: 8, ExtendPages: 1})

	a := mustAllocate(t, z, 16)
	mustFree(t, z, a)

	if _, err := z.Free(a); !errors.Is(err, ErrFreedAgain) {
		t.Fatalf("second Free = %v, want ErrFreedAgain", err)
	}

	z.FreeAgain(a)

	for range 3 {
		if x, err := z.Allocate(16); err != nil || x != a {
			t.Errorf("Allocate = %X, %v; want the looped %X", x, err, a)
		}
	}

	if err := z.Check(); err == nil {
		t.Error("Check of a looped list found nothing")
	}

	if l := z.ListReports()[0]; l.Loop != a {
		t.Errorf("list %+v, want Loop %X", l, a)
	}
}

// FreedBytes counts every free whose size is good (failures too), less
// what requests take back from freed space; a reset clears it.
func TestFreedBytes(t *testing.T) {
	z, _ := newZone(t, smallAreas)

	a := mustAllocate(t, z, 16)
	b := mustAllocate(t, z, 24)

	if _, err := z.FreeRange(a, 16); err != nil {
		t.Fatal(err)
	}

	if _, err := z.FreeRange(a, 16); !errors.Is(err, ErrDoubleFree) {
		t.Fatal(err)
	}

	if _, err := z.FreeRange(0x7000000, 8); !errors.Is(err, ErrBadAddress) {
		t.Fatal(err)
	}

	if n := z.FreedBytes(); n != 40 {
		t.Errorf("FreedBytes = %d, want 40 (16, 16 again, and 8 outside)", n)
	}

	mustAllocate(t, z, 8) // from a's free span

	if n := z.FreedBytes(); n != 32 {
		t.Errorf("FreedBytes = %d, want 32", n)
	}

	_ = b
	z.Reset()

	if n := z.FreedBytes(); n != 0 {
		t.Errorf("FreedBytes after Reset = %d", n)
	}
}

// Tagged blocks with alignment 8, laid out as VMS 7.1 lays them out:
// the first block's address 8 into its area, a block of 1 byte 16 bytes
// long, one of 8 bytes 24 (its second tag pushes the next block's
// address to the next quadword).
func TestTaggedLayout(t *testing.T) {
	z, _ := newZone(t, Attributes{BlockSize: 8, Alignment: 8, BoundaryTags: true, ExtendPages: 1})

	a := mustAllocate(t, z, 1)
	b := mustAllocate(t, z, 8)
	c := mustAllocate(t, z, 9)

	if a%PageSize != 8 || b-a != 16 || c-a != 40 {
		t.Errorf("blocks at %d, +%d, +%d; want 8, +16, +40", a%PageSize, b-a, c-a)
	}

	if before, after, size, ok := z.Tags(b); !ok || before != a+8 || after != c-12 || size != 24 {
		t.Errorf("Tags(b) = %X, %X, %d, %v", before, after, size, ok)
	}

	if n := z.Nominal(9); n != 24 {
		t.Errorf("Nominal(9) = %d, want 24 (16 and the tags)", n)
	}
}

// Frequent sizes: with every list holding a size, a new size takes over
// the next list in turn, whose blocks go back to the area.
func TestFrequentSizesTakeOver(t *testing.T) {
	z, _ := newZone(t, Attributes{Algorithm: FrequentSizes, Lists: 2, BlockSize: 8, Alignment: 8, ExtendPages: 1})

	var blocks []uint32
	for _, size := range []uint32{16, 32, 48, 64} {
		blocks = append(blocks, mustAllocate(t, z, size))
	}

	for _, b := range blocks {
		mustFree(t, z, b)
	}

	// 16 and 32 take the lists; 48 takes over the first, 64 the second.
	lists := z.ListReports()
	if lists[0].Size != 48 || lists[1].Size != 64 || len(lists[0].Blocks) != 1 || len(lists[1].Blocks) != 1 {
		t.Errorf("lists %+v, want 48 and 64 with a block each", lists)
	}
}

// The pool's Chunk: it grows by at least Chunk pages, and by just what
// it needs when the region can't give a whole chunk.
func TestPagePoolChunk(t *testing.T) {
	r := &fakeRegion{next: 0x10000, limit: 0x10000 + 200*PageSize}
	p := NewPagePool(r)
	p.Chunk = 128

	a, err := p.Allocate(1)
	if err != nil || a != 0x10000 || r.next != 0x10000+128*PageSize {
		t.Fatalf("Allocate(1) = %X, %v; region at %X, want 128 pages", a, err, r.next)
	}

	// 127 free pages at the top; 130 need 3 more, and the region has
	// room for 72, not a chunk: the pool takes 3.
	if _, err := p.Allocate(130); err != nil {
		t.Fatalf("Allocate(130) = %v", err)
	}

	if r.next != 0x10000+131*PageSize {
		t.Errorf("region at %X after a request past a chunk the region can't give whole, want %X",
			r.next, 0x10000+131*PageSize)
	}

	mustCheck(t, p)
}
