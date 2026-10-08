package console

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// newServiceConsole returns a booted console with the P1 system service
// vector deposited, so tests can CALL services directly (callService).
func newServiceConsole(t *testing.T) *Console {
	t.Helper()

	c := newBootableConsole(t)

	src := filepath.Join(t.TempDir(), "p1.asm")
	if err := os.WriteFile(src, []byte("\t.microkernel\n\t.p1vector\n\t.end\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := c.Assemble(src); err != nil {
		t.Fatalf("Assemble(.p1vector): %v", err)
	}

	return c
}

// callService CALLs the system service name with args and returns R0.
func callService(t *testing.T, c *Console, name string, args ...uint32) uint32 {
	t.Helper()

	addr, ok := c.Symbols.Get(name)
	if !ok {
		t.Fatalf("no %s symbol", name)
	}

	if err := c.Call(addr, false, args...); err != nil {
		t.Fatalf("CALL %s: %v", name, err)
	}

	return c.CPU.GPR(vax.R0)
}

// Scratch addresses for the address space tests: an address range array,
// a return range array, and P0 pages well above anything else in use.
const (
	vaInadr  = 0x8000
	vaRetadr = 0x8010
	vaPages  = 0x40000
)

func putRange(t *testing.T, c *Console, addr, a, b uint32) {
	t.Helper()

	if err := c.storeLong(addr, a); err != nil {
		t.Fatal(err)
	}

	if err := c.storeLong(addr+4, b); err != nil {
		t.Fatal(err)
	}
}

func getRange(t *testing.T, c *Console, addr uint32) (uint32, uint32) {
	t.Helper()

	a, err1 := c.Mem.LoadLongword(c.CPU, addr)
	b, err2 := c.Mem.LoadLongword(c.CPU, addr+4)

	if err1 != nil || err2 != nil {
		t.Fatalf("reading the range at %#x: %v %v", addr, err1, err2)
	}

	return a, b
}

func pteAt(t *testing.T, c *Console, addr uint32) vm.PTE {
	t.Helper()

	_, _, pte, err := c.Mem.LookupPTE(c.CPU, addr)
	if err != nil {
		t.Fatalf("LookupPTE(%#x): %v", addr, err)
	}

	return pte
}

// Status values the address space services return.
var (
	ssAccVio    = vmsdef.Symbols["SS$_ACCVIO"]
	ssNoPriv    = vmsdef.Symbols["SS$_NOPRIV"]
	ssPagOwnVio = vmsdef.Symbols["SS$_PAGOWNVIO"]
	ssVasFull   = vmsdef.Symbols["SS$_VASFULL"]
	ssIllPagCnt = vmsdef.Symbols["SS$_ILLPAGCNT"]
	ssBadParam  = vmsdef.Symbols["SS$_BADPARAM"]
)

func TestCretvaDeltva(t *testing.T) {
	c := newServiceConsole(t)

	// Three pages, addresses given high to low, in executive mode's name.
	putRange(t, c, vaInadr, vaPages+0x400, vaPages+0x10)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 1); got != ssNormal {
		t.Fatalf("$CRETVA = %#x", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != vaPages+0x5FF || b != vaPages {
		t.Errorf("retadr %#x-%#x, want %#x-%#x (the order they were created)", a, b, vaPages+0x5FF, vaPages)
	}

	if pte := pteAt(t, c, vaPages+0x200); pte.Owner() != uint8(vax.Executive) || pte.Protection() != vm.ProtEW || pte.Valid() {
		t.Errorf("created PTE %#x: want executive-owned, EW, demand-zero", uint32(pte))
	}

	if c.RTL.RegionSize[0] < vaPages+0x600 {
		t.Errorf("P0 high-water mark %#x, want past the created pages", c.RTL.RegionSize[0])
	}

	// Recreating a page with data in it gives a fresh zero page.
	if err := c.storeLong(vaPages+0x208, 0x1234); err != nil {
		t.Fatal(err)
	}

	putRange(t, c, vaInadr, vaPages+0x200, vaPages+0x200)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, 0, 1); got != ssNormal {
		t.Fatalf("$CRETVA again = %#x", got)
	}

	if v, _ := c.Mem.LoadLongword(c.CPU, vaPages+0x208); v != 0 {
		t.Errorf("recreated page holds %#x, want 0", v)
	}

	// Delete all three (deleted from the array's second address to its
	// first) plus one that never existed.
	putRange(t, c, vaInadr, vaPages, vaPages+0x600)

	if got := callService(t, c, "SYS$DELTVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$DELTVA = %#x", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != vaPages+0x7FF || b != vaPages {
		t.Errorf("retadr %#x-%#x, want %#x-%#x", a, b, vaPages+0x7FF, vaPages)
	}

	if pte := pteAt(t, c, vaPages+0x200); pte != 0 {
		t.Errorf("deleted PTE %#x, want 0", uint32(pte))
	}

	if _, err := c.Mem.LoadLongword(c.CPU, vaPages+0x208); err == nil {
		t.Error("a deleted page can still be read")
	}
}

func TestCretvaDeltva_errors(t *testing.T) {
	c := newServiceConsole(t)

	// A system-region page.
	putRange(t, c, vaInadr, 0x80000000, 0x80000000)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr); got != ssNoPriv {
		t.Errorf("$CRETVA in S0 = %#x, want SS$_NOPRIV", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != 0xFFFFFFFF || b != 0xFFFFFFFF {
		t.Errorf("retadr after creating nothing = %#x-%#x, want -1 twice", a, b)
	}

	if got := callService(t, c, "SYS$DELTVA", vaInadr); got != ssNoPriv {
		t.Errorf("$DELTVA in S0 = %#x, want SS$_NOPRIV", got)
	}

	// Beyond P0's page table.
	beyond := (c.CPU.PR(vax.P0LR) + 4) * 512
	putRange(t, c, vaInadr, beyond, beyond)

	if got := callService(t, c, "SYS$CRETVA", vaInadr); got != ssVasFull {
		t.Errorf("$CRETVA beyond P0 = %#x, want SS$_VASFULL", got)
	}

	if got := callService(t, c, "SYS$DELTVA", vaInadr); got != ssNormal {
		t.Errorf("$DELTVA beyond P0 = %#x, want success (nothing there)", got)
	}

	// An unreadable array.
	if got := callService(t, c, "SYS$CRETVA", 0x3FFFFFF0); got != ssAccVio {
		t.Errorf("$CRETVA with a bad inadr = %#x, want SS$_ACCVIO", got)
	}
}

// TestDeltva_owner checks that user mode can't delete a kernel page, and
// can delete its own.
func TestDeltva_owner(t *testing.T) {
	c := newServiceConsole(t)

	putRange(t, c, vaInadr, vaPages, vaPages+0x200)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, 0, 0); got != ssNormal { // kernel's
		t.Fatalf("$CRETVA = %#x", got)
	}

	c.Engine.SetModeStack(vax.User, false)

	if got := callService(t, c, "SYS$DELTVA", vaInadr, vaRetadr); got != ssPagOwnVio {
		t.Errorf("user $DELTVA of kernel pages = %#x, want SS$_PAGOWNVIO", got)
	}

	if got := callService(t, c, "SYS$CRETVA", vaInadr, 0, 3); got != ssPagOwnVio {
		t.Errorf("user $CRETVA over kernel pages = %#x, want SS$_PAGOWNVIO", got)
	}

	// An ordinary P0 page, user-owned since VMINIT.
	putRange(t, c, vaInadr, vaPages+0x1000, vaPages+0x1000)

	if got := callService(t, c, "SYS$DELTVA", vaInadr); got != ssNormal {
		t.Errorf("user $DELTVA of a user page = %#x", got)
	}
}

func TestCntreg(t *testing.T) {
	c := newServiceConsole(t)
	c.RTL.RegionSize[0] = vaPages + 0x600

	if got := callService(t, c, "SYS$CNTREG", 2, vaRetadr); got != ssNormal {
		t.Fatalf("$CNTREG = %#x", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != vaPages+0x5FF || b != vaPages+0x200 {
		t.Errorf("retadr %#x-%#x, want %#x-%#x", a, b, vaPages+0x5FF, vaPages+0x200)
	}

	if c.RTL.RegionSize[0] != vaPages+0x200 {
		t.Errorf("P0 high-water mark %#x, want %#x", c.RTL.RegionSize[0], vaPages+0x200)
	}

	if pteAt(t, c, vaPages+0x400) != 0 || pteAt(t, c, vaPages) == 0 {
		t.Error("the wrong pages were deleted")
	}

	// P1: its lowest page goes.
	low := uint32(0x40000000) + c.CPU.PR(vax.P1LR)*512

	if got := callService(t, c, "SYS$CNTREG", 1, vaRetadr, 0, 1); got != ssNormal {
		t.Fatalf("$CNTREG P1 = %#x", got)
	}

	if pteAt(t, c, low) != 0 || pteAt(t, c, low+512) == 0 {
		t.Error("P1's lowest page should be deleted, and only it")
	}

	if got := callService(t, c, "SYS$CNTREG", 0); got != ssIllPagCnt {
		t.Errorf("$CNTREG 0 = %#x, want SS$_ILLPAGCNT", got)
	}

	if got := callService(t, c, "SYS$CNTREG", 1, 0, 0, 2); got != ssBadParam {
		t.Errorf("$CNTREG region 2 = %#x, want SS$_BADPARAM", got)
	}
}

// TestVASpace_assembledProgram runs testdata/asm/vaspace.asm
// (docs/PHASE-26.md subtask 35): pages created, used, deleted (an access
// violation caught by the program's handler), and created again empty.
func TestVASpace_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "vaspace.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a service failed?)", got)
	}

	retadr := func(sym string) (uint32, uint32) {
		a, _ := c.Symbols.Get(sym)

		return getRange(t, c, a)
	}

	if a, b := retadr("RETADR1"); a != 0x60000 || b != 0x603FF {
		t.Errorf("$CRETVA's retadr %#x-%#x", a, b)
	}

	if a, b := retadr("RETADR2"); a != 0x603FF || b != 0x60000 {
		t.Errorf("$DELTVA's retadr %#x-%#x", a, b)
	}

	checks := []struct {
		sym  string
		want uint32
	}{
		{"READ1", 0x55},
		{"READ2", 0xFF}, // never stored: the read faulted
		{"BADVA", 0x60208},
		{"READ3", 0},
	}

	for _, ck := range checks {
		if got := word(ck.sym); got != ck.want {
			t.Errorf("%s = %#x, want %#x", ck.sym, got, ck.want)
		}
	}
}
