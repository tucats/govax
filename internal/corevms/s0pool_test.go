package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// poolBase and poolPages describe the pool the S0Pool tests use: ten
// pages starting at an S0 address.
const (
	poolBase  = 0x80010000
	poolPages = 10
)

func newTestPool() *S0Pool {
	return NewS0Pool(poolBase, poolBase+poolPages*pageSize)
}

func mustAllocate(t *testing.T, p *S0Pool, pages, pid uint32, purpose string) uint32 {
	t.Helper()

	addr, err := p.Allocate(pages, pid, purpose)
	if err != nil {
		t.Fatalf("Allocate(%d): %v", pages, err)
	}

	return addr
}

// TestS0PoolFirstFit checks allocations are contiguous runs taken from the
// lowest free run long enough, so a freed run in the middle is reused by
// an allocation that fits it and skipped by one that doesn't.
func TestS0PoolFirstFit(t *testing.T) {
	p := newTestPool()

	a := mustAllocate(t, p, 3, 0x302, "P0 page table")
	b := mustAllocate(t, p, 2, 0x302, "kernel stack")
	c := mustAllocate(t, p, 2, 0x303, "P0 page table")

	if a != poolBase || b != poolBase+3*pageSize || c != poolBase+5*pageSize {
		t.Fatalf("addresses %#x %#x %#x, want consecutive runs from %#x", a, b, c, poolBase)
	}

	if !p.Free(b) {
		t.Fatal("Free(b) found nothing")
	}

	// Three pages don't fit b's two-page hole: they go after c.
	if d := mustAllocate(t, p, 3, 0x303, "P1 page table"); d != poolBase+7*pageSize {
		t.Errorf("3-page run at %#x, want %#x", d, poolBase+7*pageSize)
	}

	// One page does.
	if e := mustAllocate(t, p, 1, 0x303, "PCB"); e != b {
		t.Errorf("1-page run at %#x, want the hole at %#x", e, b)
	}

	if got := p.FreePages(); got != 1 {
		t.Errorf("FreePages = %d, want 1", got)
	}

	allocs := p.Allocations()
	for i := 1; i < len(allocs); i++ {
		if allocs[i].Addr < allocs[i-1].End() {
			t.Errorf("allocations out of order or overlapping: %+v", allocs)
		}
	}
}

// TestS0PoolExhausted checks the error when no free run is long enough
// even though enough pages are free in all.
func TestS0PoolExhausted(t *testing.T) {
	p := newTestPool()

	mustAllocate(t, p, 4, 0x302, "a")
	b := mustAllocate(t, p, 2, 0x302, "b")
	mustAllocate(t, p, 1, 0x302, "c")
	p.Free(b)

	// Free: b's 2 pages and the last 3; 5 in all, 3 at most together.
	_, err := p.Allocate(4, 0x303, "too big")

	var ex *S0ExhaustedError
	if !errors.As(err, &ex) {
		t.Fatalf("err = %v, want *S0ExhaustedError", err)
	}

	if ex.Pages != 4 || ex.Largest != 3 {
		t.Errorf("S0ExhaustedError = %+v, want Pages 4, Largest 3", *ex)
	}

	if _, err := p.Allocate(0, 0x303, "nothing"); err == nil {
		t.Error("Allocate(0) succeeded")
	}
}

// TestS0PoolFreeProcess frees only the named process's runs.
func TestS0PoolFreeProcess(t *testing.T) {
	p := newTestPool()

	mustAllocate(t, p, 3, 0x302, "P0 page table")
	mustAllocate(t, p, 2, 0x303, "P0 page table")
	mustAllocate(t, p, 1, 0x302, "PCB")

	if got := p.FreeProcess(0x302); got != 4 {
		t.Errorf("FreeProcess freed %d pages, want 4", got)
	}

	allocs := p.Allocations()
	if len(allocs) != 1 || allocs[0].PID != 0x303 {
		t.Errorf("allocations left = %+v, want process 303's only", allocs)
	}
}

// TestS0PoolClaim checks the pool's start moves up past S0 the console's
// ASM has filled, rounded up to a page, but never over an allocation.
func TestS0PoolClaim(t *testing.T) {
	p := NewS0Pool(poolBase+5, poolBase+poolPages*pageSize)

	if base, _ := p.Range(); base != poolBase+pageSize {
		t.Fatalf("base = %#x, want it rounded up to %#x", base, poolBase+pageSize)
	}

	if err := p.Claim(poolBase + 2*pageSize + 1); err != nil {
		t.Fatal(err)
	}

	if base, _ := p.Range(); base != poolBase+3*pageSize {
		t.Errorf("base after Claim = %#x, want %#x", base, poolBase+3*pageSize)
	}

	if err := p.Claim(poolBase); err != nil {
		t.Errorf("Claim below the base: %v", err)
	}

	a := mustAllocate(t, p, 1, 0x302, "PCB")
	if a != poolBase+3*pageSize {
		t.Errorf("allocation at %#x, want the new base", a)
	}

	if err := p.Claim(a + 1); err == nil {
		t.Error("Claim over an allocation succeeded")
	}
}

// TestAllocateS0ClearsPages runs on a System with S0 mapped as VMINIT maps
// it, and checks AllocateS0 clears what it hands out and that removing a
// process frees what's left of its pages.
func TestAllocateS0ClearsPages(t *testing.T) {
	env := newMappedEnvironment(t)
	sys := env.System

	if _, err := sys.AllocateS0(1, 0x301, "x"); err == nil {
		t.Fatal("AllocateS0 with no pool succeeded")
	}

	_, limit := s0TestRange()
	sys.SetS0Pool(NewS0Pool(limit-4*pageSize, limit))

	// Dirty the pool's pages first.
	for va := limit - 4*pageSize; va < limit; va += 4 {
		if err := sys.mem.StoreLongword(sys.cpu, va, 0xFFFFFFFF); err != nil {
			t.Fatal(err)
		}
	}

	addr, err := sys.AllocateS0(2, env.Process.PID, "kernel stack")
	if err != nil {
		t.Fatal(err)
	}

	for va := addr; va < addr+2*pageSize; va += 4 {
		v, err := sys.mem.LoadLongword(sys.cpu, va)
		if err != nil {
			t.Fatal(err)
		}

		if v != 0 {
			t.Fatalf("%#x = %#x after AllocateS0, want 0", va, v)
		}
	}

	sys.RemoveProcess(env)

	if got := sys.S0Pool().FreePages(); got != 4 {
		t.Errorf("FreePages after RemoveProcess = %d, want 4", got)
	}
}

// s0TestPages is how many S0 pages newMappedEnvironment maps.
const s0TestPages = 64

// s0TestRange returns the S0 addresses newMappedEnvironment maps.
func s0TestRange() (base, limit uint32) {
	return 0x80000000, 0x80000000 + s0TestPages*pageSize
}

// newMappedEnvironment returns fixture's Environment with memory mapping
// on and S0 mapped as VMINIT maps it: the system page table at physical
// address 0, S0 page n on physical page n.
func newMappedEnvironment(t *testing.T) *Environment {
	t.Helper()

	env, _ := fixture()

	for i := uint32(0); i < s0TestPages; i++ {
		var pte vm.PTE

		pte.SetValid(true)
		pte.SetProtection(vm.ProtURKW)
		pte.SetPFN(i)

		putLongword(t, env, i*4, uint32(pte)) // mapping is still off: physical
	}

	env.cpu.SetPR(vax.SBR, 0)
	env.cpu.SetPR(vax.SLR, s0TestPages)
	env.cpu.SetPR(vax.MAPEN, 1)

	return env
}
