package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/vmheap"
)

// heapFixture is an environment whose P0 high-water mark is at 0x1000,
// with P0 pages pages of P0 (0 for the whole region).
func heapFixture(t *testing.T, pages uint32) *Environment {
	t.Helper()

	env, _ := fixture()
	env.RegionSize[0] = 0x1000

	if pages != 0 {
		env.Space = &ProcessSpace{P0Pages: pages}
	}

	return env
}

func malloc(t *testing.T, env *Environment, size uint32) uint32 {
	t.Helper()

	addr, err := shimDeccMalloc(env, []uint32{size})
	if err != nil {
		t.Fatal(err)
	}

	return addr
}

func checkHeap(t *testing.T, env *Environment) {
	t.Helper()

	for _, z := range env.zones {
		if err := z.zone.Check(); err != nil {
			t.Fatal(err)
		}
	}

	if env.heapPages != nil {
		if err := env.heapPages.Check(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMallocFirstBlock(t *testing.T) {
	env := heapFixture(t, 0)

	addr := malloc(t, env, 64)

	// The first page at the P0 high-water mark holds the RTL's zones'
	// control blocks; the default zone's first area starts after it,
	// its first block after the area's control block.
	if want := uint32(0x1000 + pageSize + vmheap.AreaHeader); addr != want {
		t.Errorf("malloc(64) = %#x, want %#x", addr, want)
	}

	// The area is the default zone's 128 pages, taken by $EXPREG's rule.
	if want := uint32(0x1000 + 129*pageSize); env.RegionSize[0] != want {
		t.Errorf("RegionSize[0] = %#x, want %#x", env.RegionSize[0], want)
	}

	if n, ok := env.BlockSize(addr); !ok || n != 64 {
		t.Errorf("BlockSize = %d, %v; want 64, true", n, ok)
	}

	checkHeap(t, env)
}

func TestMallocRoundingAndReuse(t *testing.T) {
	env := heapFixture(t, 0)

	a := malloc(t, env, 1)
	b := malloc(t, env, 16)
	c := malloc(t, env, 17)

	// Blocks are multiples of 8 bytes, quadword aligned (the default
	// zone's attributes), with nothing added.
	if b-a != 8 || c-b != 16 {
		t.Errorf("blocks at %#x, %#x, %#x; want 8 and 16 bytes apart", a, b, c)
	}

	if r0, _ := shimDeccFree(env, []uint32{b}); r0 != 0 {
		t.Fatalf("free(%#x) = %#x, want 0", b, r0)
	}

	// The freed 16 bytes are reused, without reaching into c.
	if d := malloc(t, env, 16); d != b {
		t.Errorf("malloc(16) after free = %#x, want %#x", d, b)
	}

	checkHeap(t, env)
}

func TestMallocZeroSizesAreDistinct(t *testing.T) {
	env := heapFixture(t, 0)

	a, b := malloc(t, env, 0), malloc(t, env, 0)
	if a == 0 || b == 0 || a == b {
		t.Errorf("malloc(0) twice = %#x, %#x; want two distinct blocks", a, b)
	}
}

func TestFree(t *testing.T) {
	env := heapFixture(t, 0)

	if r0, _ := shimDeccFree(env, []uint32{0}); r0 != 0 {
		t.Errorf("free(NULL) = %#x, want 0", r0)
	}

	if r0, _ := shimDeccFree(env, []uint32{0x2000}); r0 != 0xFFFFFFFF {
		t.Errorf("free of an address with no heap yet = %#x, want -1", r0)
	}

	a := malloc(t, env, 32)

	for _, addr := range []uint32{a + 8, 0x2000} {
		if r0, _ := shimDeccFree(env, []uint32{addr}); r0 != 0xFFFFFFFF {
			t.Errorf("free(%#x), not a block's start = %#x, want -1", addr, r0)
		}
	}

	if !env.FreeVM(a) {
		t.Fatal("FreeVM of a block failed")
	}

	if env.FreeVM(a) {
		t.Error("FreeVM of a freed block succeeded")
	}

	if _, ok := env.BlockSize(a); ok {
		t.Error("BlockSize of a freed block found it")
	}

	checkHeap(t, env)
}

func TestHeapStaysInP0(t *testing.T) {
	// 160 pages of P0, the mark at page 8: the control blocks' page and
	// one 128-page area fit, a second area doesn't.
	env := heapFixture(t, 160)

	if a := malloc(t, env, 100); a == 0 {
		t.Fatal("malloc(100) failed")
	}

	// More than the area's free 127 pages needs a second area, of 131
	// pages, where P0 has 23 left.
	if a := malloc(t, env, 130*pageSize); a != 0 {
		t.Errorf("malloc(130 pages) = %#x, want 0: P0 has no room", a)
	}

	if a, _ := env.AllocateVM(100); a == 0 {
		t.Error("AllocateVM(100) failed after a request too large for P0")
	}

	if env.RegionSize[0] > 160*pageSize {
		t.Errorf("RegionSize[0] = %#x, past P0's end %#x", env.RegionSize[0], 160*pageSize)
	}

	checkHeap(t, env)
}

func TestExpregStaysInP0(t *testing.T) {
	env := heapFixture(t, 16)

	if r0, _ := serviceSysExpreg(env, []uint32{9, 0, 0, regionP0}); r0 != ssVasFull {
		t.Errorf("$EXPREG of 9 pages past P0's 16 = %#x, want SS$_VASFULL", r0)
	}

	if env.RegionSize[0] != 0x1000 {
		t.Errorf("RegionSize[0] = %#x after a failed $EXPREG, want 0x1000", env.RegionSize[0])
	}

	if r0, _ := serviceSysExpreg(env, []uint32{8, 0, 0, regionP0}); r0 != ssNormal {
		t.Errorf("$EXPREG of P0's last 8 pages = %#x, want SS$_NORMAL", r0)
	}
}

func TestHeapAndExpregShareP0(t *testing.T) {
	env := heapFixture(t, 0)

	a := malloc(t, env, 64)
	areaEnd := env.RegionSize[0]

	// The program's own $EXPREG gets the pages after the heap's area...
	if r0, _ := serviceSysExpreg(env, []uint32{1, 0, 0, regionP0}); r0 != ssNormal {
		t.Fatalf("$EXPREG = %#x", r0)
	}

	if got := env.cpu.GPR(1); got != areaEnd {
		t.Errorf("$EXPREG's page at %#x, want %#x (after the heap's area)", got, areaEnd)
	}

	// ...and the heap's next area goes after that page.
	b := malloc(t, env, 200*pageSize)
	if b < areaEnd+pageSize {
		t.Errorf("second area's block at %#x, inside the program's page at %#x", b, areaEnd)
	}

	if env.defaultHeap.zone.Areas() != 2 || a == b {
		t.Errorf("%d areas, want 2", env.defaultHeap.zone.Areas())
	}

	checkHeap(t, env)
}

func TestReleaseHeap(t *testing.T) {
	env := heapFixture(t, 0)

	a := malloc(t, env, 64)
	env.releaseHeap()

	if _, ok := env.BlockSize(a); ok {
		t.Error("a block survived the image's rundown")
	}

	if env.FreeVM(a) {
		t.Error("FreeVM of a block from before the rundown succeeded")
	}
}
