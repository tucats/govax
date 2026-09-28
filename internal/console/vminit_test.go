package console

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

func TestVMInit_enablesTranslation(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil { // 128 pages of physical RAM
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if c.CPU.PR(vax.MAPEN) != 1 {
		t.Fatal("expected MAPEN enabled after VMInit")
	}

	if !c.VMInitValid {
		t.Error("expected VMInitValid true")
	}

	// A P0 virtual address (page 1, since page 0 is guarded) should
	// round-trip through the page table VMInit built.
	if err := c.Mem.StoreLongword(c.CPU, 0x200, 0xDEADBEEF); err != nil {
		t.Fatalf("StoreLongword through P0: %v", err)
	}

	v, err := c.Mem.LoadLongword(c.CPU, 0x200)
	if err != nil {
		t.Fatalf("LoadLongword through P0: %v", err)
	}

	if v != 0xDEADBEEF {
		t.Errorf("got %#x, want 0xdeadbeef", v)
	}
}

// TestVMInit_flushesTranslationBuffer matches console_vminit.c's own
// "Dump the translation buffer" step: invalidate_tb() plus a tries/hits/
// pflushes counter reset, run right alongside enabling MAPEN --
// docs/PHASE-21.md.
func TestVMInit_flushesTranslationBuffer(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if got := len(c.Mem.TBSnapshot()); got != 0 {
		t.Errorf("TBSnapshot() len = %d right after VMInit, want 0", got)
	}

	tries, hits, _, pflushes := c.Mem.TBStats()
	if tries != 0 || hits != 0 || pflushes != 0 {
		t.Errorf("TBStats() right after VMInit = tries=%d hits=%d pflushes=%d, want all 0", tries, hits, pflushes)
	}
}

func TestVMInit_guardsFirstP0Page(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if _, err := c.Mem.LoadLongword(c.CPU, 0x0); err == nil {
		t.Error("expected an access violation reading the guarded P0 page 0")
	}
}

func TestVMInit_stackPointersAreDistinctAndInS0(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	ksp := c.CPU.PR(vax.KSP)
	esp := c.CPU.PR(vax.ESP)
	ssp := c.CPU.PR(vax.SSP)
	isp := c.CPU.PR(vax.ISP)

	if ksp == esp || esp == ssp || ssp == isp || ksp == 0 {
		t.Errorf("expected distinct nonzero stacks: KSP=%#x ESP=%#x SSP=%#x ISP=%#x", ksp, esp, ssp, isp)
	}

	if c.CPU.GPR(vax.SP) != ksp {
		t.Errorf("SP = %#x, want KSP %#x", c.CPU.GPR(vax.SP), ksp)
	}
}

func TestVMInit_reservesConsoleScratch(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	scratch, ok := c.Symbols.Get("CONSOLE$SCRATCH")
	if !ok {
		t.Fatal("expected CONSOLE$SCRATCH symbol to be defined")
	}

	if scratch < 0x80000000 {
		t.Errorf("expected CONSOLE$SCRATCH in S0 space, got %#x", scratch)
	}

	if err := c.Mem.StoreLongword(c.CPU, scratch, 0xCAFEF00D); err != nil {
		t.Fatalf("StoreLongword through CONSOLE$SCRATCH: %v", err)
	}

	v, err := c.Mem.LoadLongword(c.CPU, scratch)
	if err != nil {
		t.Fatalf("LoadLongword through CONSOLE$SCRATCH: %v", err)
	}

	if v != 0xCAFEF00D {
		t.Errorf("got %#x, want 0xcafef00d", v)
	}
}

// TestVMInit_stringPoolWritableFromUserMode matches console_vminit_dcl's
// own `setpte_multiple("... PROT=PTE$K_ALL")` override of the string pool's
// pages: unlike the rest of S0 (ProtURKW -- every mode may read, only
// kernel may write), the pool itself must also be writable from user mode,
// since expr.go's parseQuotedString (and a running program's own RTL calls,
// e.g. CALL LIB$PUT_OUTPUT("Hello")) build string descriptors there without
// necessarily switching to kernel mode first.
func TestVMInit_stringPoolWritableFromUserMode(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	base, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_BASE")
	if !ok {
		t.Fatal("expected CONSOLE$STRINGPOOL_BASE symbol to be defined")
	}

	psl := c.CPU.PSL()
	psl.SetCurMod(vax.User)
	c.CPU.SetPSL(psl)

	if err := c.Mem.StoreLongword(c.CPU, base+4, 0xCAFEF00D); err != nil {
		t.Fatalf("StoreLongword through CONSOLE$STRINGPOOL_BASE from user mode: %v", err)
	}

	v, err := c.Mem.LoadLongword(c.CPU, base+4)
	if err != nil {
		t.Fatalf("LoadLongword through CONSOLE$STRINGPOOL_BASE: %v", err)
	}

	if v != 0xCAFEF00D {
		t.Errorf("got %#x, want 0xcafef00d", v)
	}
}

func TestVMInit_requiresInit(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.VMInit(1, 1, 0, 1, 1, 1, 1, 8); err == nil {
		t.Error("expected error before Init")
	}
}

func TestVMInit_rejectsOversizedRequest(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}
	
	if err := c.VMInit(1000, 1000, 0, 1, 1, 1, 1, 8); err == nil {
		t.Error("expected error for a VM request exceeding physical memory")
	}
}

// TestVMInit_modeStacks checks the executive and supervisor stacks
// (docs/MODE-STACKS.md): separate runs of pages, 8 by default, each
// protected for its mode, with a no-access guard page below.
func TestVMInit_modeStacks(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(8192 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// 0 pages asks for the default.
	if err := c.VMInit(2048, 8192, 2048, 20, 0, 0, 0, 0); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	ksp, esp, ssp := c.CPU.PR(vax.KSP), c.CPU.PR(vax.ESP), c.CPU.PR(vax.SSP)

	// Each stack's pages, lowest first: from its pointer's page down
	// eight pages, and the guard page below them.
	stackPages := func(sp uint32) (bottom, guard uint32) {
		top := sp &^ 511
		bottom = top - (defaultModeStackPages-1)*512

		return bottom, bottom - 512
	}

	eBottom, eGuard := stackPages(esp)
	sBottom, sGuard := stackPages(ssp)

	if eGuard <= ksp || sGuard <= esp {
		t.Fatalf("stacks overlap: KSP %#x, ESP %#x (guard %#x), SSP %#x (guard %#x)", ksp, esp, eGuard, ssp, sGuard)
	}

	protection := func(addr uint32) vm.Protection {
		t.Helper()

		_, _, pte, err := c.Mem.LookupPTE(c.CPU, addr)
		if err != nil {
			t.Fatalf("LookupPTE(%#x): %v", addr, err)
		}

		return pte.Protection()
	}

	for _, tc := range []struct {
		name       string
		from, to   uint32
		protection vm.Protection
	}{
		{"executive stack", eBottom, esp, vm.ProtEW},
		{"its guard page", eGuard, eGuard, vm.ProtNA},
		{"supervisor stack", sBottom, ssp, vm.ProtSW},
		{"its guard page", sGuard, sGuard, vm.ProtNA},
	} {
		for addr := tc.from &^ 511; addr <= tc.to; addr += 512 {
			if got := protection(addr); got != tc.protection {
				t.Errorf("%s: page %#x protection %d, want %d", tc.name, addr, got, tc.protection)
			}
		}
	}

	// Each mode can push on its own stack but not on a more privileged
	// one's; user mode can't touch either.
	write := func(mode vax.AccessMode, addr uint32) error {
		psl := c.CPU.PSL()
		psl.SetCurMod(mode)
		c.CPU.SetPSL(psl)
		c.Mem.InvalidateProtection()

		return c.Mem.StoreLongword(c.CPU, addr, 1)
	}

	for _, tc := range []struct {
		mode vax.AccessMode
		addr uint32
		ok   bool
	}{
		{vax.Executive, esp, true},
		{vax.Kernel, esp, true},
		{vax.Supervisor, esp, false},
		{vax.Supervisor, ssp, true},
		{vax.Executive, ssp, true},
		{vax.User, ssp, false},
		{vax.Executive, eGuard, false},
		{vax.Kernel, sGuard, false},
	} {
		if err := write(tc.mode, tc.addr); (err == nil) != tc.ok {
			t.Errorf("mode %d writing %#x: err %v, want ok=%v", tc.mode, tc.addr, err, tc.ok)
		}
	}
}

// TestVMInit_modeStackSizes: explicit sizes are used as given.
func TestVMInit_modeStackSizes(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 3, 5, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	// Past KSP's page: a guard page, then 3 executive pages.
	ksp, esp, ssp := c.CPU.PR(vax.KSP), c.CPU.PR(vax.ESP), c.CPU.PR(vax.SSP)

	if esp-ksp != (1+3)*512 || ssp-esp != (1+5)*512 {
		t.Errorf("KSP %#x, ESP %#x, SSP %#x: want ESP 4 pages above KSP, SSP 6 above ESP", ksp, esp, ssp)
	}
}
