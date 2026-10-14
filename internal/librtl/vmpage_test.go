package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// Cells the page tests pass by reference: the count and the address.
const (
	pageCountCell = 0x1000
	pageAddrCell  = 0x1004
)

// pageFixture is an RTL environment whose P0 ends at 0x4000, so the
// pool's first page is there.
func pageFixture(t *testing.T) *corevms.Environment {
	t.Helper()

	env := fixture(t)
	env.RegionSize[0] = 0x4000

	return env
}

// getPages calls LIB$GET_VM_PAGE for n pages and returns the status and
// the address it stored.
func getPages(t *testing.T, env *corevms.Environment, n uint32) (uint32, uint32) {
	t.Helper()

	putLongword(t, env, pageCountCell, n)
	putLongword(t, env, pageAddrCell, 0xDEADBEEF)

	r0 := call(t, env, "LIB$GET_VM_PAGE", pageCountCell, pageAddrCell)

	return r0, longword(t, env, pageAddrCell)
}

// mustGetPages is getPages that fails the test unless it succeeds.
func mustGetPages(t *testing.T, env *corevms.Environment, n uint32) uint32 {
	t.Helper()

	r0, addr := getPages(t, env, n)
	if r0 != ssNormal {
		t.Fatalf("LIB$GET_VM_PAGE(%d) = %#x, want SS$_NORMAL", n, r0)
	}

	return addr
}

// freePages calls LIB$FREE_VM_PAGE for n pages at addr.
func freePages(t *testing.T, env *corevms.Environment, n, addr uint32) uint32 {
	t.Helper()

	putLongword(t, env, pageCountCell, n)
	putLongword(t, env, pageAddrCell, addr)

	return call(t, env, "LIB$FREE_VM_PAGE", pageCountCell, pageAddrCell)
}

// Pages come from P0's end, upward; freed pages are reused lowest first,
// and new pages are taken only when no free group fits.
func TestLibGetVMPage(t *testing.T) {
	env := pageFixture(t)

	a := mustGetPages(t, env, 1)
	b := mustGetPages(t, env, 1)
	c := mustGetPages(t, env, 3)

	if a != 0x4000 || b != 0x4200 || c != 0x4400 {
		t.Fatalf("pages at %X, %X, %X; want 4000, 4200, 4400", a, b, c)
	}

	if env.RegionSize[0] != 0x4A00 {
		t.Errorf("P0 ends at %X, want 4A00", env.RegionSize[0])
	}

	if r0 := freePages(t, env, 1, a); r0 != ssNormal {
		t.Fatalf("LIB$FREE_VM_PAGE = %#x", r0)
	}

	// Two pages don't fit in the free one: they're new.
	if d := mustGetPages(t, env, 2); d != 0x4A00 {
		t.Errorf("2 pages at %X, want 4A00", d)
	}

	// One page does.
	if e := mustGetPages(t, env, 1); e != a {
		t.Errorf("1 page at %X, want the freed %X", e, a)
	}
}

// A group may be freed in parts, and groups from several calls together
// when they lie next to each other.
func TestLibFreeVMPagePartsAndGroups(t *testing.T) {
	env := pageFixture(t)

	a := mustGetPages(t, env, 4)
	b := mustGetPages(t, env, 2)

	// The middle two pages of the first group.
	if r0 := freePages(t, env, 2, a+0x200); r0 != ssNormal {
		t.Fatalf("free of a group's middle = %#x", r0)
	}

	// The first group's last page and the whole second group, in one
	// call.
	if r0 := freePages(t, env, 3, a+0x600); r0 != ssNormal {
		t.Fatalf("free across groups = %#x", r0)
	}

	if b != a+0x800 {
		t.Fatalf("second group at %X, want %X", b, a+0x800)
	}

	// The free pages are one run of five now: five pages fit there.
	if c := mustGetPages(t, env, 5); c != a+0x200 {
		t.Errorf("5 pages at %X, want %X", c, a+0x200)
	}

	if r0 := freePages(t, env, 1, a); r0 != ssNormal {
		t.Errorf("free of the first page = %#x", r0)
	}
}

func TestLibVMPageBadArguments(t *testing.T) {
	env := pageFixture(t)

	a := mustGetPages(t, env, 2)

	if r0 := freePages(t, env, 1, a+0x200); r0 != ssNormal {
		t.Fatalf("LIB$FREE_VM_PAGE = %#x", r0)
	}

	for _, n := range []uint32{0, 0xFFFFFFFF, 0x80000000} {
		if r0, addr := getPages(t, env, n); r0 != libBadBlockSize || addr != 0xDEADBEEF {
			t.Errorf("LIB$GET_VM_PAGE(%d) = %#x with %X stored, want LIB$_BADBLOSIZ and nothing stored", int32(n), r0, addr)
		}

		if r0 := freePages(t, env, n, a); r0 != libBadBlockSize {
			t.Errorf("LIB$FREE_VM_PAGE(%d) = %#x, want LIB$_BADBLOSIZ", int32(n), r0)
		}
	}

	for _, c := range []struct {
		name string
		n    uint32
		addr uint32
	}{
		{"freed already", 1, a + 0x200},
		{"one page freed already", 2, a},
		{"not on a page boundary", 1, a + 4},
		{"never allocated", 1, 0x20000},
		{"below the pool", 1, 0x3E00},
	} {
		if r0 := freePages(t, env, c.n, c.addr); r0 != libBadBlockAddress {
			t.Errorf("%s: LIB$FREE_VM_PAGE(%d, %X) = %#x, want LIB$_BADBLOADR", c.name, c.n, c.addr, r0)
		}
	}

	// Nothing was freed by the failures: the first page is still
	// allocated, so one page goes in the free second one.
	if b := mustGetPages(t, env, 1); b != a+0x200 {
		t.Errorf("1 page at %X, want %X", b, a+0x200)
	}

	if r0 := freePages(t, env, 1, a); r0 != ssNormal {
		t.Errorf("free of the first page = %#x", r0)
	}
}

// More pages than P0 can hold: LIB$_INSVIRMEM, nothing allocated, and
// base-address left alone.
func TestLibGetVMPageNoRoom(t *testing.T) {
	env := pageFixture(t)

	for _, n := range []uint32{0x200000, 0x7FFFFFFF} {
		if r0, addr := getPages(t, env, n); r0 != libInsVirMem || addr != 0xDEADBEEF {
			t.Errorf("LIB$GET_VM_PAGE(%#x) = %#x with %X stored, want LIB$_INSVIRMEM and nothing stored", n, r0, addr)
		}
	}

	if env.RegionSize[0] != 0x4000 {
		t.Errorf("P0 ends at %X, want 4000 still", env.RegionSize[0])
	}

	if a := mustGetPages(t, env, 1); a != 0x4000 {
		t.Errorf("1 page at %X, want 4000", a)
	}
}

// The heap's zones take their areas from the same pool, so pages and
// blocks don't overlap, and a zone's pages can't be freed as pages.
func TestLibVMPagesShareThePool(t *testing.T) {
	env := pageFixture(t)

	a := mustGetPages(t, env, 1)

	block, err := env.AllocateVM(16)
	if err != nil || block == 0 {
		t.Fatalf("AllocateVM: %#x, %v", block, err)
	}

	b := mustGetPages(t, env, 1)

	// The RTL's zones' control blocks take a page, then the default
	// zone's first area is 128 pages, between the two.
	if area := a + 0x400; block < area || block >= area+128*0x200 || b != area+128*0x200 {
		t.Fatalf("page %X, block %X, page %X; want the block's area between the pages", a, block, b)
	}

	if r0 := freePages(t, env, 1, block&^0x1FF); r0 != libBadBlockAddress {
		t.Errorf("LIB$FREE_VM_PAGE of the block's page = %#x, want LIB$_BADBLOADR", r0)
	}

	if r0 := freePages(t, env, 2, a); r0 != libBadBlockAddress {
		t.Errorf("LIB$FREE_VM_PAGE of a page and the control blocks' = %#x, want LIB$_BADBLOADR", r0)
	}

	if !env.FreeVM(block) {
		t.Error("the block didn't survive")
	}
}
