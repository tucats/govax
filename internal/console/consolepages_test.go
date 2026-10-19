package console

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// Phase 67's subtask 7 (docs/PHASE-67 - address space under paging.md):
// with paging on, the console's ASM and DEPOSIT (and the debugger's,
// WriteBytes) create the P0 pages they write in process 1, so loading a
// program by hand works as it does with paging off.

// TestConsolePagesPagingOn: DEPOSIT past P0's end creates the page it
// writes (charged, P0 growing to it, the pages it passes over still
// nonexistent), WriteBytes does the same, ASM creates its program's
// pages, and a write below P1's low end is still refused.
func TestConsolePagesPagingOn(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	env := c.ProcessOne()

	lengths(t, c, "at the start", 0, stackTopPage)

	charged := env.PageFileCharged()

	// DEPOSIT of a longword in page 3: P0 grows to 4 pages, one created.
	if err := c.Deposit("", 0x604, machine.SizeLongword, 0x12345678); err != nil {
		t.Fatalf("Deposit past P0's end: %v", err)
	}

	lengths(t, c, "after DEPOSIT at 0x604", 4, stackTopPage)

	if got := env.PageFileCharged(); got != charged+1 {
		t.Errorf("%d pages charged after DEPOSIT, want %d", got, charged+1)
	}

	if v, err := c.Mem.LoadLongword(c.CPU, 0x604); err != nil || v != 0x12345678 {
		t.Errorf("the deposited longword: %#x, %v", v, err)
	}

	// The pages it passed over don't exist: a protection violation.
	if mask := faultMask(t, c, 0x200, vm.AccessRead); mask != 0 {
		t.Errorf("a skipped page's reason mask %d, want 0", mask)
	}

	// A longword across a page boundary creates both pages.
	if err := c.Deposit("", 0x9FE, machine.SizeLongword, 0xCAFEF00D); err != nil {
		t.Fatalf("Deposit across pages 4 and 5: %v", err)
	}

	lengths(t, c, "after DEPOSIT at 0x9FE", 6, stackTopPage)

	// The debugger's DEPOSIT writes through WriteBytes.
	if err := c.WriteBytes(0x1000, []byte{1, 2, 3}); err != nil {
		t.Fatalf("WriteBytes past P0's end: %v", err)
	}

	lengths(t, c, "after WriteBytes at 0x1000", 9, stackTopPage)

	// ASM's program, at a new session's origin (where DEPOSIT left off;
	// here page 1, 0x200).
	c.ResetAsmSession()
	c.DepositAddr = 0x200

	src := filepath.Join(t.TempDir(), "prog.asm")
	if err := os.WriteFile(src, []byte("\tMOVL\t#1,R0\n\tHALT\n\t.END\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := c.Assemble(src); err != nil {
		t.Fatalf("ASM into a page that doesn't exist: %v", err)
	}

	if b, err := c.Mem.LoadByte(c.CPU, 0x200); err != nil || b != 0xD0 {
		t.Errorf("ASM's first byte at 0x200: %#x, %v (want MOVL's D0)", b, err)
	}

	lengths(t, c, "after ASM", 9, stackTopPage)

	// P1 isn't extended: a kernel-mode write below its low end is refused.
	below := uint32(0x40000000 + stackTopPage*512 - 4)
	if err := c.Deposit("", below, machine.SizeLongword, 1); err == nil {
		t.Error("Deposit below P1's low end succeeded")
	}

	lengths(t, c, "after the P1 deposit", 9, stackTopPage)
}

// TestConsolePagesPagingOff: with paging off every page of the tables
// exists, and DEPOSIT leaves the length registers alone.
func TestConsolePagesPagingOff(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "false"})

	c := newServiceConsole(t)
	p0, p1 := c.CPU.PR(registers.P0LR), c.CPU.PR(registers.P1LR)

	if err := c.Deposit("", 0x604, machine.SizeLongword, 7); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	if got := c.CPU.PR(registers.P0LR); got != p0 {
		t.Errorf("P0LR %#x, was %#x", got, p0)
	}

	if got := c.CPU.PR(registers.P1LR); got != p1 {
		t.Errorf("P1LR %#x, was %#x", got, p1)
	}
}
