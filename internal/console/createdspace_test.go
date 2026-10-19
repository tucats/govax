package console

import (
	"errors"
	"io"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// Phase 67's subtask 2 (docs/PHASE-67 - address space under paging.md):
// with paging on, P0LR and P1LR describe the pages that exist, and the
// services that create and delete pages move them.

// stackTopPage is UserStackTop's P1 page number: a new space's P1 length
// register, below which the user stack and P1 expansions grow (subtask
// 3).
const stackTopPage = (corevms.UserStackTop - 0x40000000) / 512

// jpiLong is process env's $GETJPI item name, a longword.
func jpiLong(t *testing.T, env *corevms.Environment, name string) uint32 {
	t.Helper()

	data, ok := env.JPIItem(name)
	if !ok {
		t.Fatalf("no %s", name)
	}

	return corevms.ItemLongword(data)
}

// lengths checks that process 1's P0 and P1 length registers, the CPU's
// and its address space's copy alike, are p0 and p1.
func lengths(t *testing.T, c *Console, label string, p0, p1 uint32) {
	t.Helper()

	sp := c.ProcessOne().Space

	if got := c.CPU.PR(registers.P0LR); got != p0 || sp.P0LR != p0 {
		t.Errorf("%s: P0LR %#x (the space's %#x), want %#x", label, got, sp.P0LR, p0)
	}

	if got := c.CPU.PR(registers.P1LR); got != p1 || sp.P1LR != p1 {
		t.Errorf("%s: P1LR %#x (the space's %#x), want %#x", label, got, sp.P1LR, p1)
	}
}

// faultMask returns the reason mask of the fault an access to addr
// makes, failing if it doesn't fault.
func faultMask(t *testing.T, c *Console, addr uint32, access vm.AccessType) byte {
	t.Helper()

	_, err := c.Mem.Translate(c.CPU, addr, access)

	var tf *vm.TranslationFault
	if !errors.As(err, &tf) {
		t.Fatalf("access to %#x: %v, want a fault", addr, err)
	}

	return tf.Mask
}

// TestCreatedSpace: process 1 starts with no P0 pages and P1's permanent
// part; $CRETVA, $EXPREG, $DELTVA, and $CNTREG lengthen and shorten the
// regions, as $GETJPI's FREP0VA, FREP1VA, FREPTECNT, and VIRTPEAK show;
// a page past P0's end is a length violation, one $CRETVA skipped over a
// protection violation; and image rundown leaves P0 empty.
func TestCreatedSpace(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	env := c.ProcessOne()

	if c.Mem.Paging() == nil {
		t.Fatal("VMINIT left paging off")
	}

	lengths(t, c, "at the start", 0, stackTopPage)

	if got := jpiLong(t, env, "JPI$_FREP1VA"); got != 0x40000000+(stackTopPage-1)*512 {
		t.Errorf("FREP1VA %#x at the start", got)
	}

	// The scratch range array's page, then two pages well above it.
	putRange(t, c, vaInadr, vaPages, vaPages+0x3FF)
	lengths(t, c, "the scratch page", vaInadr/512+1, stackTopPage)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$CRETVA = %#x", got)
	}

	end := uint32(vaPages/512 + 2)
	lengths(t, c, "after $CRETVA", end, stackTopPage)

	if got := jpiLong(t, env, "JPI$_FREP0VA"); got != end*512 {
		t.Errorf("FREP0VA %#x, want %#x", got, end*512)
	}

	sp := env.Space
	if got, want := jpiLong(t, env, "JPI$_FREPTECNT"), sp.P0Pages-end+stackTopPage-(0x200000-sp.P1Pages); got != want {
		t.Errorf("FREPTECNT %d, want %d", got, want)
	}

	// The page past the end doesn't exist; one skipped over has no
	// access.
	if m := faultMask(t, c, end*512+4, vm.AccessWrite); m != vm.MaskLength|vm.MaskWrite {
		t.Errorf("a write past P0's end: mask %d, want 5", m)
	}

	if m := faultMask(t, c, vaPages-512, vm.AccessRead); m != 0 {
		t.Errorf("a read of a page $CRETVA skipped: mask %d, want 0", m)
	}

	if err := storeLong(c, vaPages+0x200, 7); err != nil {
		t.Fatal(err)
	}

	// $DELTVA of the last page contracts P0; of both, past the pages
	// $CRETVA skipped, back to the scratch page.
	putRange(t, c, vaInadr, vaPages+0x200, vaPages+0x3FF)

	if got := callService(t, c, "SYS$DELTVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$DELTVA = %#x", got)
	}

	lengths(t, c, "after $DELTVA of the last page", end-1, stackTopPage)

	putRange(t, c, vaInadr, vaPages, vaPages+0x1FF)

	if got := callService(t, c, "SYS$DELTVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$DELTVA = %#x", got)
	}

	lengths(t, c, "after $DELTVA of the other", vaInadr/512+1, stackTopPage)

	// $EXPREG adds pages at each region's end.
	if got := callService(t, c, "SYS$EXPREG", 3, vaRetadr, 0, 0); got != ssNormal {
		t.Fatalf("$EXPREG P0 = %#x", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != vaInadr+0x200 || b != vaInadr+0x7FF {
		t.Errorf("$EXPREG P0's retadr %#x %#x", a, b)
	}

	lengths(t, c, "after $EXPREG P0", vaInadr/512+4, stackTopPage)

	if got := callService(t, c, "SYS$EXPREG", 2, vaRetadr, 0, 1); got != ssNormal {
		t.Fatalf("$EXPREG P1 = %#x", got)
	}

	lengths(t, c, "after $EXPREG P1", vaInadr/512+4, stackTopPage-2)

	low := uint32(0x40000000 + (stackTopPage-2)*512)
	if a, b := getRange(t, c, vaRetadr); a != low+0x3FF || b != low {
		t.Errorf("$EXPREG P1's retadr %#x %#x", a, b)
	}

	if got := jpiLong(t, env, "JPI$_FREP1VA"); got != low-512 {
		t.Errorf("FREP1VA %#x, want %#x", got, low-512)
	}

	peak := jpiLong(t, env, "JPI$_VIRTPEAK")
	if want := uint32(vaPages/512+2) + 0x200000 - stackTopPage; peak != want {
		t.Errorf("VIRTPEAK %d, want %d", peak, want)
	}

	// $CNTREG of P1 gives the two pages back.
	if got := callService(t, c, "SYS$CNTREG", 2, vaRetadr, 0, 1); got != ssNormal {
		t.Fatalf("$CNTREG P1 = %#x", got)
	}

	lengths(t, c, "after $CNTREG P1", vaInadr/512+4, stackTopPage)

	if m := faultMask(t, c, low, vm.AccessRead); m != vm.MaskLength {
		t.Errorf("a read below P1's low end: mask %d, want 1", m)
	}

	// Image rundown deletes P0.
	env.DeleteP0()
	lengths(t, c, "after DeleteP0", 0, stackTopPage)

	if got := jpiLong(t, env, "JPI$_VIRTPEAK"); got != peak {
		t.Errorf("VIRTPEAK %d after DeleteP0, want the peak, %d", got, peak)
	}
}

// TestCreatedSpaceOtherProcess: a process that isn't current starts with
// no P0 pages, and creating its pages changes its address space and its
// hardware PCB, not the CPU's registers.
func TestCreatedSpaceOtherProcess(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	sys := c.RTL.System

	env, err := corevms.NewEnvironment(sys, c.Logicals, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	pid := env.Process.PID

	if env.Space, err = sys.BuildAddressSpace(pid, 64, 8192); err != nil {
		t.Fatal(err)
	}

	if env.Stacks, err = sys.BuildStacks(pid, 4, 2, 2); err != nil {
		t.Fatal(err)
	}

	if env.Space.P0LR != 0 || env.Space.P1LR != stackTopPage {
		t.Errorf("a new space's lengths: P0LR %#x, P1LR %#x", env.Space.P0LR, env.Space.P1LR)
	}

	start := corevms.InitialPCB(env.Space, env.Stacks, 0x400, 0)
	if err := cpu.WritePCB(c.Mem, env.Stacks.PCBB, &start); err != nil {
		t.Fatal(err)
	}

	p0lr := c.CPU.PR(registers.P0LR)

	if err := env.EnsurePages(0x400, 0x300); err != nil {
		t.Fatal(err)
	}

	if err := c.Mem.StoreLongwordIn(c.CPU, env.Space.AddressSpace, 0x6FC, 9); err != nil {
		t.Fatalf("a store to a created page: %v", err)
	}

	pcb, err := cpu.ReadPCB(c.Mem, env.Stacks.PCBB)
	if err != nil {
		t.Fatal(err)
	}

	if env.Space.P0LR != 4 || pcb.P0LR != 4 || pcb.P1LR != stackTopPage || pcb.ASTLVL != 4 {
		t.Errorf("after creating pages 2 and 3: the space's P0LR %d, the PCB's %+v", env.Space.P0LR, pcb)
	}

	if got := c.CPU.PR(registers.P0LR); got != p0lr {
		t.Errorf("the CPU's P0LR %#x, was %#x", got, p0lr)
	}

	if _, err := c.Mem.LoadLongwordIn(c.CPU, env.Space.AddressSpace, 0x800); err == nil {
		t.Error("a read past the other process's P0 succeeded")
	}
}

// TestCreatedSpacePagingOff: with paging off the regions are their whole
// tables, and the services leave the length registers alone.
func TestCreatedSpacePagingOff(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "false"})

	c := newServiceConsole(t)
	env := c.ProcessOne()
	p0, p1 := c.CPU.PR(registers.P0LR), c.CPU.PR(registers.P1LR)

	if p0 != env.Space.P0Pages || p1 != 0x200000-env.Space.P1Pages {
		t.Fatalf("P0LR %#x, P1LR %#x: not the whole tables", p0, p1)
	}

	putRange(t, c, vaInadr, vaPages, vaPages+0x3FF)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$CRETVA = %#x", got)
	}

	if got := callService(t, c, "SYS$EXPREG", 2, vaRetadr, 0, 1); got != ssNormal {
		t.Fatalf("$EXPREG P1 = %#x", got)
	}

	lengths(t, c, "after $CRETVA and $EXPREG", p0, p1)

	if got := jpiLong(t, env, "JPI$_FREP0VA"); got != vaPages+0x400 {
		t.Errorf("FREP0VA %#x, want P0's mark, %#x", got, vaPages+0x400)
	}
}
