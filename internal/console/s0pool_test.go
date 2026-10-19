package console

import (
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vm"
)

// TestS0PoolAfterVaxInit boots as cmd/govax does, running vax.init (VMINIT
// /P0=16384 /P1=8192 /S0=8192 /KSP=20, then ASM of the microkernel), and
// checks the S0 pool that's left for new processes: it starts past the
// microkernel, ends at S0's end, and holds the number of processes
// corevms/s0pool.go's arithmetic promises.
func TestS0PoolAfterVaxInit(t *testing.T) {
	c, _ := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	pool := c.RTL.S0Pool()
	if pool == nil {
		t.Fatal("no S0 pool after VMINIT")
	}

	base, limit := pool.Range()
	t.Logf("S0 pool %08X-%08X: pages %d-%d, %d free; VMINIT's layout ends at %08X",
		base, limit, (base-0x80000000)/512, (limit-0x80000000)/512-1, pool.FreePages(), c.S0Free())

	if base <= c.S0Free() {
		t.Errorf("pool base %08X, want it past the microkernel at %08X", base, c.S0Free())
	}

	if want := uint32(0x80000000 + 8192*512); limit != want {
		t.Errorf("pool limit %08X, want S0's end %08X", limit, want)
	}

	// The pool starts at the first page past the microkernel's S0.
	if c.AsmSession() == nil {
		t.Fatal("no ASM session after vax.init")
	}

	if end := c.AsmSession().S0End(); base != (end+511)&^511 {
		t.Errorf("pool base %08X, want the page after the microkernel's end, %08X", base, end)
	}

	const perProcess = 128 + 64 + 38 + 1 // page tables, stacks, PCB
	if n := pool.FreePages() / perProcess; n < 34 {
		t.Errorf("room for %d processes, want at least 34", n)
	}
}

// s0Mapped reports whether the S0 page at va has a frame, failing on a
// page without one whose PTE isn't the all-zero one.
func s0Mapped(t *testing.T, c *Console, va uint32) bool {
	t.Helper()

	_, _, pte, err := c.Mem.LookupPTE(c.CPU, va)
	if err != nil {
		t.Fatalf("S0 page %08X: %v", va, err)
	}

	if !pte.Valid() && pte != 0 {
		t.Errorf("S0 page %08X without a frame has PTE %08X, want 0", va, uint32(pte))
	}

	return pte.Valid()
}

// TestS0PoolOnDemand: with paging on (Phase 67, subtask 5), vax.init
// leaves frames behind VMINIT's layout, the microkernel (claimed before
// it was written), and the pool's allocations, and none behind the
// pool's other pages, which have no access; AllocateS0 gives a run
// cleared frames, and FreeS0 and FreeProcessS0 take them back.
func TestS0PoolOnDemand(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c, _ := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	sys := c.RTL.System
	db := c.Mem.Paging()

	if db == nil {
		t.Fatal("vax.init left paging off")
	}

	pool := sys.S0Pool()
	base, limit := pool.Range()

	// The microkernel's pages, from VMINIT's layout's end to the pool's.
	for va := c.S0Free() &^ 511; va < base; va += 512 {
		if !s0Mapped(t, c, va) {
			t.Errorf("the microkernel's page %08X has no frame", va)
		}
	}

	// The pool: its allocations' pages have frames, the rest none.
	inUse := map[uint32]bool{}

	for _, a := range pool.Allocations() {
		for va := a.Addr; va < a.End(); va += 512 {
			inUse[va] = true
		}
	}

	for va := base; va < limit; va += 512 {
		if got := s0Mapped(t, c, va); got != inUse[va] {
			t.Errorf("pool page %08X has a frame %v, want %v", va, got, inUse[va])
		}
	}

	if mapped := sys.MappedS0Pages(); mapped > 512 {
		t.Errorf("%d S0 pages mapped, want VMINIT's layout, the microkernel, and the pool's few allocations", mapped)
	}

	// An unallocated pool page can't even be read, where a page in use
	// can be (S0's URKW).
	free := limit - 512
	if inUse[free] {
		t.Fatalf("the pool's last page %08X is in use", free)
	}

	if _, err := c.Mem.Translate(c.CPU, free, vm.AccessRead); err == nil {
		t.Errorf("a read of unallocated pool page %08X succeeded", free)
	}

	if _, err := c.Mem.Translate(c.CPU, base-512, vm.AccessRead); err != nil {
		t.Errorf("a read of the microkernel's last page: %v", err)
	}

	// AllocateS0 backs its run with cleared frames, FreeS0 takes them back.
	system := db.Counts().System

	addr, err := sys.AllocateS0(3, 0x99, "test")
	if err != nil {
		t.Fatal(err)
	}

	if got := db.Counts().System; got != system+3 {
		t.Errorf("system frames %d after allocating 3 pages, want %d", got, system+3)
	}

	for va := addr; va < addr+3*512; va += 512 {
		if v, err := c.Mem.LoadLongword(c.CPU, va+256); err != nil || v != 0 {
			t.Errorf("allocated page %08X: %08X (%v), want a cleared page", va, v, err)
		}
	}

	if !sys.FreeS0(addr) {
		t.Fatal("FreeS0 found no run")
	}

	if got := db.Counts().System; got != system {
		t.Errorf("system frames %d after FreeS0, want %d", got, system)
	}

	for va := addr; va < addr+3*512; va += 512 {
		if s0Mapped(t, c, va) {
			t.Errorf("freed page %08X still has a frame", va)
		}
	}

	// A process's runs all go back with FreeProcessS0.
	if _, err := sys.AllocateS0(2, 0x99, "one"); err != nil {
		t.Fatal(err)
	}

	if _, err := sys.AllocateS0(1, 0x99, "two"); err != nil {
		t.Fatal(err)
	}

	if n := sys.FreeProcessS0(0x99); n != 3 {
		t.Errorf("FreeProcessS0 freed %d pages, want 3", n)
	}

	if got := db.Counts().System; got != system {
		t.Errorf("system frames %d after FreeProcessS0, want %d", got, system)
	}

	if err := db.Check(); err != nil {
		t.Error(err)
	}
}

// TestS0PoolPagingOff: with paging off the pool's pages keep VMINIT's
// frames, allocated or freed.
func TestS0PoolPagingOff(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "false"})

	c := newServiceConsole(t)
	sys := c.RTL.System
	base, limit := sys.S0Pool().Range()

	for va := base; va < limit; va += 512 {
		if !s0Mapped(t, c, va) {
			t.Fatalf("pool page %08X has no frame with paging off", va)
		}
	}

	addr, err := sys.AllocateS0(2, 0x99, "test")
	if err != nil {
		t.Fatal(err)
	}

	sys.FreeS0(addr)

	if !s0Mapped(t, c, addr) || !s0Mapped(t, c, addr+512) {
		t.Error("a freed pool page lost its frame with paging off")
	}
}
