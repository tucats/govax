package console

import (
	"errors"
	"io"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// bootedConsole returns a console that has run vax.init: VMINIT, the
// microkernel, and the P1 vector.
func bootedConsole(t *testing.T) *Console {
	t.Helper()

	c, _ := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.Include("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	return c
}

// TestProcessOneAdoptsVMInitTables: after VMINIT, process 1's address
// space is VMINIT's tables, which the pool doesn't own.
func TestProcessOneAdoptsVMInitTables(t *testing.T) {
	c := bootedConsole(t)
	s := c.RTL.Space

	if s == nil {
		t.Fatal("process 1 has no address space after VMINIT")
	}

	if s.AddressSpace != vm.CurrentAddressSpace(c.CPU) || s.Owned() {
		t.Errorf("process 1's space %+v (owned %v), want the CPU's registers, not owned", s.AddressSpace, s.Owned())
	}

	if s.P0Pages != 16384 || s.P1Pages != 8192 {
		t.Errorf("sizes %d/%d, want vax.init's 16384/8192", s.P0Pages, s.P1Pages)
	}
}

// TestBuildAddressSpace builds a second process's address space after
// vax.init and checks its P0 is its own, its P1 vector is process 1's
// physical pages (read-only), its P0 page 0 is a guard, and teardown
// gives every page back.
func TestBuildAddressSpace(t *testing.T) {
	c := bootedConsole(t)
	sys := c.RTL.System
	one := c.RTL.Space.AddressSpace

	// Process 1's P0 page at addr, read first so that its demand-zero
	// page isn't counted as process 2's below.
	const addr = 0x200

	before, err := c.Mem.LoadLongwordIn(c.CPU, one, addr)
	if err != nil {
		t.Fatal(err)
	}

	poolFree := sys.S0Pool().FreePages()
	mapped := c.Mem.MappedPages()

	s, err := sys.BuildAddressSpace(0x302, 16384, 8192)
	if err != nil {
		t.Fatalf("BuildAddressSpace: %v", err)
	}

	if used := poolFree - sys.S0Pool().FreePages(); used != 128+64 {
		t.Errorf("tables took %d pool pages, want 192", used)
	}

	if s.P0LR != 16384 || s.P1LR != 0x200000-8192 || s.P1BR+s.P1LR*4 != s.P1Table {
		t.Errorf("registers %+v, tables at %08X/%08X", s.AddressSpace, s.P0Table, s.P1Table)
	}

	// The P1 vector: the same physical page as process 1's, and read-only.
	vec := vmsdef.P1VectorTable[0].Addr

	p1, err1 := c.Mem.TranslateIn(c.CPU, one, vec, vm.AccessRead)
	p2, err2 := c.Mem.TranslateIn(c.CPU, s.AddressSpace, vec, vm.AccessRead)

	if err1 != nil || err2 != nil || p1 != p2 {
		t.Errorf("P1 vector at %08X: process 1 %#x (%v), process 2 %#x (%v); want the same page", vec, p1, err1, p2, err2)
	}

	var tf *vm.TranslationFault
	if err := c.Mem.StoreLongwordIn(c.CPU, s.AddressSpace, vec, 0); !errors.As(err, &tf) || tf.Kind != vm.ProtectionViolation {
		t.Errorf("writing process 2's P1 vector: %v, want a protection violation", err)
	}

	// P0: separate pages. Page 0 is a no-access guard.
	if err := c.Mem.StoreLongwordIn(c.CPU, s.AddressSpace, addr, ^before); err != nil {
		t.Fatalf("store into process 2's P0: %v", err)
	}

	if after, _ := c.Mem.LoadLongwordIn(c.CPU, one, addr); after != before {
		t.Errorf("process 1's %08X changed from %#x to %#x", addr, before, after)
	}

	if _, err := c.Mem.TranslateIn(c.CPU, s.AddressSpace, 0, vm.AccessRead); !errors.As(err, &tf) || tf.Kind != vm.ProtectionViolation {
		t.Errorf("process 2's P0 page 0: %v, want a protection violation", err)
	}

	// P1's lowest and highest pages are both there (demand zero).
	for _, va := range []uint32{0x80000000 - 8192*512, spP1 - 4} {
		if err := c.Mem.StoreLongwordIn(c.CPU, s.AddressSpace, va, 1); err != nil {
			t.Errorf("process 2's P1 %08X: %v", va, err)
		}
	}

	// Teardown frees the frames process 2 was given (three), but not the
	// shared vector page, and the tables' pool pages.
	if got := c.Mem.MappedPages() - mapped; got != 3 {
		t.Errorf("process 2 was given %d pages, want 3", got)
	}

	if err := sys.TeardownAddressSpace(s); err != nil {
		t.Fatalf("TeardownAddressSpace: %v", err)
	}

	if got := c.Mem.MappedPages(); got != mapped {
		t.Errorf("%d pages mapped after teardown, want %d", got, mapped)
	}

	if got := sys.S0Pool().FreePages(); got != poolFree {
		t.Errorf("%d pool pages free after teardown, want %d", got, poolFree)
	}

	if _, err := c.Mem.TranslateIn(c.CPU, one, vec, vm.AccessRead); err != nil {
		t.Errorf("process 1's P1 vector after teardown: %v", err)
	}

	// Process 1's own tables are never torn down, and the current
	// space can't be.
	if err := sys.TeardownAddressSpace(c.RTL.Space); err != nil || c.RTL.Space.AddressSpace != one {
		t.Errorf("tearing down process 1's: %v", err)
	}
}

// TestBuildAddressSpaceErrors: sizes out of range and an exhausted pool
// are errors that leave nothing allocated.
func TestBuildAddressSpaceErrors(t *testing.T) {
	c := bootedConsole(t)
	sys := c.RTL.System
	free := sys.S0Pool().FreePages()

	for _, sizes := range [][2]uint32{{0, 8}, {8, 0}, {0x200001, 8}} {
		if _, err := sys.BuildAddressSpace(0x302, sizes[0], sizes[1]); err == nil {
			t.Errorf("BuildAddressSpace(%d, %d) succeeded", sizes[0], sizes[1])
		}
	}

	// A P1 table bigger than the pool: the P0 table, allocated first, is
	// given back.
	if _, err := sys.BuildAddressSpace(0x302, 128, 0x200000); err == nil {
		t.Error("BuildAddressSpace with a 16384-page P1 table succeeded")
	}

	if got := sys.S0Pool().FreePages(); got != free {
		t.Errorf("%d pool pages free after the failures, want %d", got, free)
	}
}

// TestRemoveProcessTearsDownSpace: removing a process from the table
// gives back its address space's pages.
func TestRemoveProcessTearsDownSpace(t *testing.T) {
	c := bootedConsole(t)
	sys := c.RTL.System

	poolFree := sys.S0Pool().FreePages()
	mapped := c.Mem.MappedPages()

	env, err := corevms.NewEnvironment(sys, c.Logicals, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if env.Space, err = sys.BuildAddressSpace(env.Process.PID, 64, 64); err != nil {
		t.Fatal(err)
	}

	if err := c.Mem.StoreLongwordIn(c.CPU, env.Space.AddressSpace, 0x400, 7); err != nil {
		t.Fatal(err)
	}

	sys.RemoveProcess(env)

	if c.Mem.MappedPages() != mapped || sys.S0Pool().FreePages() != poolFree {
		t.Errorf("after RemoveProcess: %d pages mapped (want %d), %d pool pages free (want %d)",
			c.Mem.MappedPages(), mapped, sys.S0Pool().FreePages(), poolFree)
	}
}
