package console

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 67's subtask 3 (docs/PHASE-67 - address space under paging.md):
// with paging on, the user stack grows by itself when a user-mode access
// falls below P1's low end, $ADJSTK expands it, and image rundown deletes
// it but for the part still in use.

// inMode switches process 1's CPU to access mode mode, as the VAX does
// (the stack pointer saved and loaded), and returns a function that
// switches it back.
func inMode(c *Console, mode registers.AccessMode) func() {
	old := c.CPU.PSL().CurMod()

	set := func(from, to registers.AccessMode) {
		c.CPU.SetPR(registers.PrivReg(from), c.CPU.GPR(registers.SP))
		c.CPU.SetGPR(registers.SP, c.CPU.PR(registers.PrivReg(to)))

		psl := c.CPU.PSL()
		psl.SetCurMod(to)
		c.CPU.SetPSL(psl)
		c.Mem.InvalidateProtection()
	}

	set(old, mode)

	return func() { set(mode, old) }
}

// stackPagesCharged counts the P1 pages from addr up to UserStackTop that
// are charged against process 1's paging file quota.
func stackPagesCharged(env *corevms.Environment, addr uint32) int {
	n := 0

	for va := addr &^ 511; va < corevms.UserStackTop; va += 512 {
		if env.PageFileChargedAt(va) {
			n++
		}
	}

	return n
}

// TestStackExpansion: a user-mode store just below P1's low end creates
// the page, charged, and a read further down every page down to it; a
// PROBE and an executive-mode access never expand, and are length
// violations; and the pages a new space has above UserStackTop, up to the
// P1 vector, don't exist.
func TestStackExpansion(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	env := c.ProcessOne()
	top := uint32(corevms.UserStackTop)

	lengths(t, c, "at the start", 0, stackTopPage)

	restore := inMode(c, registers.User)

	if err := c.Mem.StoreLongword(c.CPU, top-4, 1); err != nil {
		t.Fatalf("a user-mode push below P1's low end: %v", err)
	}

	lengths(t, c, "after a push", 0, stackTopPage-1)

	if n := stackPagesCharged(env, top-512); n != 1 {
		t.Errorf("%d stack pages charged after a push, want 1", n)
	}

	if c.Mem.ProbeAccess(c.CPU, top-10*512, vm.AccessWrite) {
		t.Error("PROBEW below P1's low end allowed")
	}

	lengths(t, c, "after a PROBE", 0, stackTopPage-1)

	if _, err := c.Mem.LoadLongword(c.CPU, top-6*512+8); err != nil {
		t.Fatalf("a user-mode read five pages below P1's low end: %v", err)
	}

	lengths(t, c, "after the read", 0, stackTopPage-6)

	if n := stackPagesCharged(env, top-6*512); n != 6 {
		t.Errorf("%d stack pages charged after the read, want 6", n)
	}

	restore()

	restore = inMode(c, registers.Executive)

	if m := faultMask(t, c, top-7*512, vm.AccessWrite); m != vm.MaskLength|vm.MaskWrite {
		t.Errorf("an executive-mode write below P1's low end: mask %d, want 5", m)
	}

	restore()

	lengths(t, c, "after the executive-mode write", 0, stackTopPage-6)

	// The pages between the stack's top and the P1 vector exist in P1's
	// length but have no access; the vector's do.
	restore = inMode(c, registers.User)
	defer restore()

	if m := faultMask(t, c, top+512, vm.AccessRead); m != 0 {
		t.Errorf("a read above UserStackTop: mask %d, want 0 (no access)", m)
	}

	if _, err := c.Mem.LoadLongword(c.CPU, vmsdef.P1VectorTable[0].Addr); err != nil {
		t.Errorf("a read of the P1 vector: %v", err)
	}
}

// TestStackExpansionQuota: a user-mode access below P1's low end that the
// paging file quota can't cover is the length violation it would have
// been, with the pages the quota did cover kept.
func TestStackExpansionQuota(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	env := c.ProcessOne()
	top := uint32(corevms.UserStackTop)

	job := env.Process.Job
	job.Pooled.PGFLQUOTA = job.PageFileCharged + 3

	restore := inMode(c, registers.User)
	defer restore()

	if m := faultMask(t, c, top-5*512, vm.AccessWrite); m != vm.MaskLength|vm.MaskWrite {
		t.Errorf("a write five pages below the stack with a quota of 3: mask %d, want 5", m)
	}

	lengths(t, c, "after the write", 0, stackTopPage-3)

	if got := env.PageFileRemaining(); got != 0 {
		t.Errorf("%d pages of quota left, want 0", got)
	}
}

// TestAdjstkExpands: $ADJSTK of the user stack to below P1's low end
// creates the pages down to the new stack pointer; past the quota it's
// SS$_EXQUOTA, the stack pointer unchanged, the pages created kept.
func TestAdjstkExpands(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	env := c.ProcessOne()
	top := uint32(corevms.UserStackTop)

	c.CPU.SetPR(registers.USP, top)

	if err := storeLong(c, vaInadr, top-8*512); err != nil {
		t.Fatal(err)
	}

	if got := callService(t, c, "SYS$ADJSTK", 3, 0, vaInadr); got != ssNormal {
		t.Fatalf("$ADJSTK = %#x", got)
	}

	if got := c.CPU.PR(registers.USP); got != top-8*512 {
		t.Errorf("USP %#x, want %#x", got, top-8*512)
	}

	lengths(t, c, "after $ADJSTK", vaInadr/512+1, stackTopPage-8)

	job := env.Process.Job
	job.Pooled.PGFLQUOTA = job.PageFileCharged + 2

	if err := storeLong(c, vaInadr, top-20*512); err != nil {
		t.Fatal(err)
	}

	if got := callService(t, c, "SYS$ADJSTK", 3, 0, vaInadr); got != vmsdef.Symbols["SS$_EXQUOTA"] {
		t.Errorf("$ADJSTK past the quota = %#x, want SS$_EXQUOTA", got)
	}

	if got := c.CPU.PR(registers.USP); got != top-8*512 {
		t.Errorf("USP %#x after the failure, want %#x", got, top-8*512)
	}

	lengths(t, c, "after the failure", vaInadr/512+1, stackTopPage-10)
}

// TestStackRundown: image rundown deletes the user stack, but for the
// pages from the one the user stack pointer is in up, which hold what
// called the image; with the pointer at UserStackTop, all of it, its
// charges refunded.
func TestStackRundown(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	env := c.ProcessOne()
	top := uint32(corevms.UserStackTop)

	if err := env.CreateUserStack(0); err != nil {
		t.Fatal(err)
	}

	lengths(t, c, "the stack created", 0, stackTopPage-20)

	if n := stackPagesCharged(env, top-20*512); n != 20 {
		t.Errorf("%d stack pages charged, want 20", n)
	}

	// The pointer two pages down: those two pages stay.
	c.CPU.SetPR(registers.USP, top-2*512+0x100)
	env.DeleteP0()
	lengths(t, c, "after a rundown with SP two pages down", 0, stackTopPage-2)

	if n := stackPagesCharged(env, top-20*512); n != 2 {
		t.Errorf("%d stack pages charged after the rundown, want 2", n)
	}

	c.CPU.SetPR(registers.USP, top)
	env.DeleteP0()
	lengths(t, c, "after a rundown with SP at the top", 0, stackTopPage)

	if n := stackPagesCharged(env, top-20*512); n != 0 {
		t.Errorf("%d stack pages charged after the second rundown, want 0", n)
	}
}

// TestUserStackPagingOff: with paging off nothing expands or is deleted:
// the stack's reserve is part of P1's table from the start.
func TestUserStackPagingOff(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "false"})

	c := newServiceConsole(t)
	env := c.ProcessOne()
	p1 := c.CPU.PR(registers.P1LR)

	if err := env.CreateUserStack(0); err != nil {
		t.Fatal(err)
	}

	restore := inMode(c, registers.User)

	if err := c.Mem.StoreLongword(c.CPU, corevms.UserStackTop-4, 1); err != nil {
		t.Fatalf("a push onto the stack: %v", err)
	}

	restore()

	env.DeleteP0()

	if got := c.CPU.PR(registers.P1LR); got != p1 {
		t.Errorf("P1LR %#x, was %#x", got, p1)
	}
}
