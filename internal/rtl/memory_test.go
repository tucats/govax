package rtl

import "testing"

func TestShimDeccMallocFreshAllocation(t *testing.T) {
	env, _ := fixture()
	env.RegionSize[0] = 0x1000

	addr, err := shimDeccMalloc(env, []uint32{64})
	if err != nil {
		t.Fatal(err)
	}

	if addr != 0x1000 {
		t.Errorf("addr = %#x, want 0x1000 (the P0 high-water mark)", addr)
	}

	if len(env.memAllocated) != 1 {
		t.Fatalf("memAllocated has %d entries, want 1", len(env.memAllocated))
	}

	if env.memAllocated[0].reqSize != 64 {
		t.Errorf("reqSize = %d, want 64", env.memAllocated[0].reqSize)
	}

	// Region grows past the requested size, rounded up to a 512-byte page.
	if env.RegionSize[0] != 0x1000+512 {
		t.Errorf("RegionSize[0] = %#x, want %#x", env.RegionSize[0], 0x1000+512)
	}
}

func TestShimDeccMallocReusesFreeBlock(t *testing.T) {
	env, _ := fixture()
	env.memFreed = []*memBlock{{addr: 0x2000, size: 128}}

	addr, err := shimDeccMalloc(env, []uint32{32})
	if err != nil {
		t.Fatal(err)
	}

	if addr != 0x2000 {
		t.Errorf("addr = %#x, want 0x2000 (reused from the free list)", addr)
	}

	if len(env.memFreed) != 1 {
		t.Fatalf("memFreed has %d entries, want 1 (the leftover carved block)", len(env.memFreed))
	}

	if env.memFreed[0].flags != libvmCarved {
		t.Errorf("leftover flags = %#x, want libvmCarved", env.memFreed[0].flags)
	}
}

func TestShimDeccFreeRoundTrip(t *testing.T) {
	env, _ := fixture()
	env.RegionSize[0] = 0x1000

	addr, err := shimDeccMalloc(env, []uint32{64})
	if err != nil {
		t.Fatal(err)
	}

	r0, err := shimDeccFree(env, []uint32{addr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != 0 {
		t.Errorf("r0 = %d, want 0", r0)
	}

	if len(env.memAllocated) != 0 {
		t.Errorf("memAllocated has %d entries, want 0", len(env.memAllocated))
	}
}

func TestShimDeccFreeUnknownAddress(t *testing.T) {
	env, _ := fixture()

	r0, err := shimDeccFree(env, []uint32{0xDEAD})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != 0xFFFFFFFF {
		t.Errorf("r0 = %#x, want -1", r0)
	}
}

func TestShimDeccFreeCoalescesAdjacentBlocks(t *testing.T) {
	env, _ := fixture()
	env.RegionSize[0] = 0x1000

	a, err := shimDeccMalloc(env, []uint32{16})
	if err != nil {
		t.Fatal(err)
	}

	b, err := shimDeccMalloc(env, []uint32{16})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := shimDeccFree(env, []uint32{a}); err != nil {
		t.Fatal(err)
	}

	if _, err := shimDeccFree(env, []uint32{b}); err != nil {
		t.Fatal(err)
	}

	total := uint32(0)
	for _, p := range env.memFreed {
		total += p.size
	}
	// Both freed blocks plus the original page-rounding leftover should
	// all have coalesced back into contiguous free space.
	if len(env.memAllocated) != 0 {
		t.Errorf("memAllocated has %d entries, want 0", len(env.memAllocated))
	}

	if total != 512 {
		t.Errorf("total free space = %d, want 512 (one page, fully coalesced)", total)
	}
}

func TestShimLibGetVMFreeVM(t *testing.T) {
	env, _ := fixture()
	env.RegionSize[0] = 0x4000

	sizeAddr, retAddr := uint32(0x1000), uint32(0x1004)
	putLongword(t, env, sizeAddr, 100)

	r0, err := shimLibGetVM(env, []uint32{sizeAddr, retAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}

	addr, err := env.mem.LoadLongword(env.cpu, retAddr)
	if err != nil {
		t.Fatal(err)
	}

	if addr != 0x4000 {
		t.Errorf("allocated addr = %#x, want 0x4000", addr)
	}

	if env.memAllocated[0].zone != 0 {
		t.Errorf("zone = %d, want 0 (lib_get_vm's own argc==2 bug never actually applies a zone)", env.memAllocated[0].zone)
	}

	r0, err = shimLibFreeVM(env, []uint32{sizeAddr, retAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}

	if len(env.memAllocated) != 0 {
		t.Errorf("memAllocated has %d entries, want 0", len(env.memAllocated))
	}
}

func TestShimLibDeleteVMZone(t *testing.T) {
	env, _ := fixture()
	env.RegionSize[0] = 0x1000
	env.memAllocated = []*memBlock{
		{addr: 0x1000, size: 16, zone: 5},
		{addr: 0x2000, size: 16, zone: 5},
		{addr: 0x3000, size: 16, zone: 9},
	}

	zoneAddr := uint32(0x5000)
	putLongword(t, env, zoneAddr, 5)

	r0, err := shimLibDeleteVMZone(env, []uint32{zoneAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}
	
	if len(env.memAllocated) != 1 || env.memAllocated[0].zone != 9 {
		t.Errorf("remaining allocations = %+v, want only the zone-9 block", env.memAllocated)
	}
}
