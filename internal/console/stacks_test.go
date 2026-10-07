package console

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// s0Protection returns the protection code of the S0 page holding va.
func s0Protection(t *testing.T, c *Console, va uint32) vm.Protection {
	t.Helper()

	_, _, pte, err := c.Mem.LookupPTE(c.CPU, va)
	if err != nil {
		t.Fatalf("LookupPTE(%08X): %v", va, err)
	}

	return pte.Protection()
}

// TestBuildStacks lays out a new process's stacks after vax.init, with
// vax.init's sizes, and checks MODE-STACKS.md's layout and protections,
// the stack pointers, and the PCB page.
func TestBuildStacks(t *testing.T) {
	c := bootedConsole(t)
	sys := c.RTL.System
	free := sys.S0Pool().FreePages()

	st, err := sys.BuildStacks(0x302, 20, 8, 8)
	if err != nil {
		t.Fatalf("BuildStacks: %v", err)
	}

	if used := free - sys.S0Pool().FreePages(); used != 38+1 {
		t.Errorf("stacks and PCB took %d pool pages, want 39", used)
	}

	b := st.Base
	if st.KSP != b+20*512-4 || st.ESP != b+29*512-4 || st.SSP != b+38*512-4 || st.USP != corevms.UserStackTop {
		t.Errorf("pointers KSP %08X ESP %08X SSP %08X USP %08X, base %08X", st.KSP, st.ESP, st.SSP, st.USP, b)
	}

	want := []struct {
		page uint32
		prot vm.Protection
	}{
		{0, vm.ProtURKW}, {19, vm.ProtURKW}, // kernel stack
		{20, vm.ProtNA},                     // guard
		{21, vm.ProtEW}, {28, vm.ProtEW},    // executive stack
		{29, vm.ProtNA},                     // guard
		{30, vm.ProtSW}, {37, vm.ProtSW},    // supervisor stack
	}

	for _, w := range want {
		if got := s0Protection(t, c, b+w.page*512); got != w.prot {
			t.Errorf("stack page %d: protection %v, want %v", w.page, got, w.prot)
		}
	}

	// Executive mode may push on its stack but not past its bottom.
	if !s0Protection(t, c, st.ESP).Allows(vax.Executive, vm.AccessWrite) ||
		s0Protection(t, c, st.ESP-8*512).Allows(vax.Executive, vm.AccessWrite) {
		t.Error("the executive stack or its guard page has the wrong access")
	}

	// The PCB: a kernel-only page, at a physical address S0's table
	// gives.
	if got := s0Protection(t, c, st.PCB); got != vm.ProtKW {
		t.Errorf("PCB page protection %v, want KW", got)
	}

	if pa, err := c.Mem.TranslateIn(c.CPU, vm.AddressSpace{}, st.PCB, vm.AccessRead); err != nil || pa != st.PCBB {
		t.Errorf("PCB %08X -> %#x (%v), PCBB %#x", st.PCB, pa, err, st.PCBB)
	}

	// Freed and allocated again, the pages are back to S0's default.
	sys.S0Pool().FreeProcess(0x302)

	again, err := sys.AllocateS0(39, 0x303, "x")
	if err != nil || again != b {
		t.Fatalf("AllocateS0 = %08X, %v; want %08X", again, err, b)
	}

	for page := range uint32(39) {
		if got := s0Protection(t, c, b+page*512); got != vm.ProtURKW {
			t.Errorf("reallocated page %d: protection %v, want URKW", page, got)
		}
	}

	if _, err := sys.BuildStacks(0x302, 0, 8, 8); err == nil {
		t.Error("BuildStacks with no kernel stack succeeded")
	}
}

// TestProcessOneStacksAndPCB: process 1 adopts VMINIT's stacks, and gets
// a PCB page the first time one is needed, and the same one after.
func TestProcessOneStacksAndPCB(t *testing.T) {
	c := bootedConsole(t)
	st := c.RTL.Stacks

	if st == nil || st.KSP != c.CPU.PR(vax.KSP) || st.USP != spP1 || st.Base != 0 || st.PCB != 0 {
		t.Fatalf("process 1's stacks %+v", st)
	}

	pcbb, err := c.RTL.EnsurePCB(c.RTL)
	if err != nil || pcbb == 0 || pcbb != st.PCBB {
		t.Fatalf("EnsurePCB = %#x, %v (stacks %+v)", pcbb, err, st)
	}

	if again, _ := c.RTL.EnsurePCB(c.RTL); again != pcbb {
		t.Errorf("second EnsurePCB = %#x, want %#x", again, pcbb)
	}
}

// TestInitialPCB: a new process's first PCB holds its stacks and address
// space, round-trips through memory, and has no AST pending.
func TestInitialPCB(t *testing.T) {
	c := bootedConsole(t)
	sys := c.RTL.System

	space, err := sys.BuildAddressSpace(0x302, 64, 64)
	if err != nil {
		t.Fatal(err)
	}

	st, err := sys.BuildStacks(0x302, 4, 2, 2)
	if err != nil {
		t.Fatal(err)
	}

	var psl vax.PSL

	psl.SetCurMod(vax.User)
	psl.SetPrvMod(vax.User)

	p := corevms.InitialPCB(space, st, 0x200, psl)

	if err := cpu.WritePCB(c.Mem, st.PCBB, &p); err != nil {
		t.Fatal(err)
	}

	back, err := cpu.ReadPCB(c.Mem, st.PCBB)
	if err != nil || back != p {
		t.Errorf("PCB read back %+v (%v), want %+v", back, err, p)
	}

	if p.AddressSpace() != space.AddressSpace || p.SP[vax.Kernel] != st.KSP || p.SP[vax.User] != corevms.UserStackTop || p.ASTLVL != 4 {
		t.Errorf("initial PCB %+v", p)
	}
}
