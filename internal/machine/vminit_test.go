package machine

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

func TestVMInit_enablesTranslation(t *testing.T) {
	c := NewMachine(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil { // 128 pages of physical RAM
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if c.CPU.PR(registers.MAPEN) != 1 {
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
	c := NewMachine(&bytes.Buffer{})
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
	c := NewMachine(&bytes.Buffer{})
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
	c := NewMachine(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	ksp := c.CPU.PR(registers.KSP)
	esp := c.CPU.PR(registers.ESP)
	ssp := c.CPU.PR(registers.SSP)
	isp := c.CPU.PR(registers.ISP)

	if ksp == esp || esp == ssp || ssp == isp || ksp == 0 {
		t.Errorf("expected distinct nonzero stacks: KSP=%#x ESP=%#x SSP=%#x ISP=%#x", ksp, esp, ssp, isp)
	}

	if c.CPU.GPR(registers.SP) != ksp {
		t.Errorf("SP = %#x, want KSP %#x", c.CPU.GPR(registers.SP), ksp)
	}
}

func TestVMInit_reservesConsoleScratch(t *testing.T) {
	c := NewMachine(&bytes.Buffer{})
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
	c := NewMachine(&bytes.Buffer{})
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
	psl.SetCurMod(registers.User)
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
	c := NewMachine(&bytes.Buffer{})
	if err := c.VMInit(1, 1, 0, 1, 1, 1, 1, 8); err == nil {
		t.Error("expected error before Init")
	}
}

func TestVMInit_rejectsOversizedRequest(t *testing.T) {
	c := NewMachine(&bytes.Buffer{})
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
	c := NewMachine(&bytes.Buffer{})
	if err := c.Init(8192 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// 0 pages asks for the default.
	if err := c.VMInit(2048, 8192, 2048, 20, 0, 0, 0, 0); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	ksp, esp, ssp := c.CPU.PR(registers.KSP), c.CPU.PR(registers.ESP), c.CPU.PR(registers.SSP)

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
	write := func(mode registers.AccessMode, addr uint32) error {
		psl := c.CPU.PSL()
		psl.SetCurMod(mode)
		c.CPU.SetPSL(psl)
		c.Mem.InvalidateProtection()

		return c.Mem.StoreLongword(c.CPU, addr, 1)
	}

	for _, tc := range []struct {
		mode registers.AccessMode
		addr uint32
		ok   bool
	}{
		{registers.Executive, esp, true},
		{registers.Kernel, esp, true},
		{registers.Supervisor, esp, false},
		{registers.Supervisor, ssp, true},
		{registers.Executive, ssp, true},
		{registers.User, ssp, false},
		{registers.Executive, eGuard, false},
		{registers.Kernel, sGuard, false},
	} {
		if err := write(tc.mode, tc.addr); (err == nil) != tc.ok {
			t.Errorf("mode %d writing %#x: err %v, want ok=%v", tc.mode, tc.addr, err, tc.ok)
		}
	}
}

// TestVMInit_modeStackSizes: explicit sizes are used as given.
func TestVMInit_modeStackSizes(t *testing.T) {
	c := NewMachine(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 3, 5, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	// Past KSP's page: a guard page, then 3 executive pages.
	ksp, esp, ssp := c.CPU.PR(registers.KSP), c.CPU.PR(registers.ESP), c.CPU.PR(registers.SSP)

	if esp-ksp != (1+3)*512 || ssp-esp != (1+5)*512 {
		t.Errorf("KSP %#x, ESP %#x, SSP %#x: want ESP 4 pages above KSP, SSP 6 above ESP", ksp, esp, ssp)
	}
}

// TestVMInit_pageTablesPacked checks VMINIT lays its page tables out end
// to end: a table that fills its last page exactly is followed directly
// by the next, and one that doesn't is padded only to the page's end. (It
// used to skip a whole page after every table, even a full one.)
func TestVMInit_pageTablesPacked(t *testing.T) {
	c := NewMachine(&bytes.Buffer{})
	if err := c.Init(16384 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// S0's table (8192 entries) fills 64 pages exactly, and P0's (16384)
	// 128. P1's 100 entries fill 400 bytes of one page.
	if err := c.VMInit(16384, 100, 8192, 20, 0, 0, 4, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if got, want := c.CPU.PR(registers.P0BR), uint32(0x80000000+64*512); got != want {
		t.Errorf("P0BR = %#x, want %#x (right after S0's 64-page table)", got, want)
	}

	// P1BR points 2**21 entries before the end of P1's table; P1's table
	// starts right after P0's 128 pages.
	p1Table := c.CPU.PR(registers.P1BR) + c.CPU.PR(registers.P1LR)*4
	if want := uint32(0x80000000 + (64+128)*512); p1Table != want {
		t.Errorf("P1's table at %#x, want %#x (right after P0's 128-page table)", p1Table, want)
	}

	// The kernel stack follows P1's table, padded to its one page.
	ksp := c.CPU.PR(registers.KSP)
	if want := uint32(0x80000000 + (64+128+1+20)*512 - 4); ksp != want {
		t.Errorf("KSP = %#x, want %#x (a 20-page stack after P1's one-page table)", ksp, want)
	}
}

func TestPageRounding(t *testing.T) {
	for _, tc := range []struct{ in, want uint32 }{{0, 0}, {1, 512}, {511, 512}, {512, 512}, {513, 1024}} {
		if got := roundUpPage(tc.in); got != tc.want {
			t.Errorf("roundUpPage(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}

	for _, tc := range []struct{ in, want uint32 }{{0, 0}, {1, 1}, {128, 1}, {129, 2}, {16384, 128}} {
		if got := pageTablePages(tc.in); got != tc.want {
			t.Errorf("pageTablePages(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestVMInit_tableArea: with paging on, VMINIT's P0 and P1 tables are one
// table area (Phase 67's subtask 4): P1's table ends at the area's end
// rather than starting at its page's beginning, and only the area's pages
// holding entries of pages that exist keep frames (here P1's 200 pages,
// in its last two; P0 has none). The rest of S0's layout doesn't move, and
// VIRTUALPAGECNT defaults to /P0 plus /P1.
func TestVMInit_tableArea(t *testing.T) {
	c := NewMachine(&bytes.Buffer{})
	if err := c.Init(16384 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// P0's 300 entries fill three pages, P1's 200 two: a 5-page area
	// after S0's table. S0 takes the rest of physical memory, so paging
	// off has no leftover to fold into it and both modes' S0 is the same
	// size (with paging on, S0 isn't tied to physical memory).
	const s0 = 16384 - 300 - 200

	if err := c.VMInitPaging(false, 300, 200, s0, 20, 0, 0, 4, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	area := c.CPU.PR(registers.P0BR)
	ksp := c.CPU.PR(registers.KSP)

	if got := c.CPU.PR(registers.P1BR) + c.CPU.PR(registers.P1LR)*4; got != area+3*512 {
		t.Errorf("paging off: P1's table at %#x, want %#x, after P0's three pages", got, area+3*512)
	}

	if err := c.VMInitPaging(true, 300, 200, s0, 20, 0, 0, 4, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	if got := c.CPU.PR(registers.P0BR); got != area {
		t.Errorf("P0BR %#x, want the area's start %#x", got, area)
	}

	if got := c.CPU.PR(registers.P1BR) + 0x200000*4; got != area+5*512 {
		t.Errorf("P1's table ends at %#x, want the area's end %#x", got, area+5*512)
	}

	if got := c.CPU.PR(registers.KSP); got != ksp {
		t.Errorf("KSP %#x with paging on, %#x with it off: the layout moved", got, ksp)
	}

	for i := range uint32(5) {
		_, _, pte, err := c.Mem.LookupPTE(c.CPU, area+i*512)
		if err != nil {
			t.Fatal(err)
		}

		if want := i >= 3; pte.Valid() != want || !want && pte != 0 {
			t.Errorf("table page %d: PTE %08X, want a frame %v", i, uint32(pte), want)
		}
	}

	if got := c.ProcessOne().Sysgen.VirtualPageCount; got != 500 {
		t.Errorf("VIRTUALPAGECNT %d, want /P0 + /P1, 500", got)
	}

	// The P1 vector's region still reads through the backed pages: P1's
	// top page's PTE is ProcessPTE's.
	if pte, err := c.Mem.LoadLongword(c.CPU, area+5*512-4); err != nil || vm.PTE(pte).Protection() != vm.ProtUW {
		t.Errorf("P1's top entry %08X (%v), want a demand-zero UW page", pte, err)
	}

	if err := c.Mem.Paging().Check(); err != nil {
		t.Error(err)
	}
}

// newSizedMachine is a machine with pages pages of physical memory, after
// INIT and before VMINIT.
func newSizedMachine(t *testing.T, pages uint32) *Machine {
	t.Helper()

	c := NewMachine(&bytes.Buffer{})
	if err := c.Init(pages * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return c
}

// TestVMInit_pagingDefaults: with paging on, a region given 0 takes its
// paging-mode default (docs/PHASE-67 - address space under paging.md,
// subtask 8), whatever physical memory is, and VIRTUALPAGECNT is their
// table area; with paging off, 0 still shares physical memory.
func TestVMInit_pagingDefaults(t *testing.T) {
	c := newSizedMachine(t, 16384)

	if err := c.VMInitPaging(true, 0, 0, 0, 20, 0, 0, 4, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	if p0, p1 := c.VMInitSizes(); p0 != 19456 || p1 != 5120 {
		t.Errorf("paging on: /P0 %d, /P1 %d, want 19456 and 5120", p0, p1)
	}

	if got := c.CPU.PR(registers.SLR); got != 16384 {
		t.Errorf("paging on: SLR %d, want 16384", got)
	}

	if got := c.ProcessOne().Sysgen.VirtualPageCount; got != 24576 {
		t.Errorf("VIRTUALPAGECNT %d, want 24576", got)
	}

	// A region given its size keeps it.
	if err := c.VMInitPaging(true, 1000, 0, 0, 20, 0, 0, 4, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	if p0, p1 := c.VMInitSizes(); p0 != 1000 || p1 != 5120 {
		t.Errorf("paging on, /P0=1000: /P0 %d, /P1 %d, want 1000 and 5120", p0, p1)
	}

	// Paging off: the three regions share the 16384 pages.
	if err := c.VMInitPaging(false, 0, 0, 0, 20, 0, 0, 4, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	p0, p1 := c.VMInitSizes()
	if s0 := c.CPU.PR(registers.SLR); p0 != 5461 || p1 != 5461 || p0+p1+s0 != 16384 {
		t.Errorf("paging off: /P0 %d, /P1 %d, S0 %d, want shares of 16384", p0, p1, s0)
	}
}

// TestVMInit_s0BeyondMemory: with paging on S0 may be larger than
// physical memory, since only VMINIT's layout is mapped (each page to
// the frame of its own number) and every other S0 page starts with the
// all-zero PTE; with paging off, where all of S0 is mapped, it's refused.
func TestVMInit_s0BeyondMemory(t *testing.T) {
	c := newSizedMachine(t, 4096)

	if err := c.VMInitPaging(true, 1000, 5120, 32768, 20, 0, 0, 4, 8); err != nil {
		t.Fatalf("VMInitPaging with S0 eight times memory: %v", err)
	}

	if got := c.CPU.PR(registers.SLR); got != 32768 {
		t.Errorf("SLR %d, want 32768", got)
	}

	layout := (c.S0Free() - 0x80000000) / 512

	for _, page := range []uint32{0, layout - 1} {
		_, _, pte, err := c.Mem.LookupPTE(c.CPU, 0x80000000+page*512)
		if err != nil || !pte.Valid() || pte.PFN() != page {
			t.Errorf("layout page %d: PTE %08X (%v), want frame %d", page, uint32(pte), err, page)
		}
	}

	for _, page := range []uint32{layout, 4096, 32767} {
		_, _, pte, err := c.Mem.LookupPTE(c.CPU, 0x80000000+page*512)
		if err != nil || pte != 0 {
			t.Errorf("pool page %d: PTE %08X (%v), want 0", page, uint32(pte), err)
		}
	}

	if _, limit := c.ProcessOne().S0Pool().Range(); limit != 0x80000000+32768*512 {
		t.Errorf("the pool ends at %08X, want S0's end", limit)
	}

	// Only the layout's frames are taken (fewer, once process 1's unused
	// table pages are given back): the rest are free.
	if counts := c.Mem.Paging().Counts(); counts.Free < 4096-int(layout) {
		t.Errorf("%d free frames and a %d-page layout, want the layout's frames the only ones taken", counts.Free, layout)
	}

	if err := c.Mem.Paging().Check(); err != nil {
		t.Error(err)
	}

	// A pool page gets a frame when it's allocated, any free frame.
	sys := c.ProcessOne().System

	addr, err := sys.AllocateS0(2, 0x99, "test")
	if err != nil {
		t.Fatalf("AllocateS0: %v", err)
	}

	for va := addr; va < addr+2*512; va += 512 {
		if _, _, pte, err := c.Mem.LookupPTE(c.CPU, va); err != nil || !pte.Valid() {
			t.Errorf("allocated pool page %08X: PTE %08X (%v), want a frame", va, uint32(pte), err)
		}
	}

	if err := c.VMInitPaging(false, 1000, 1000, 32768, 20, 0, 0, 4, 8); !errors.Is(err, vmserrors.New(vmserrors.CLI_VMTOOLARGE)) {
		t.Errorf("paging off, S0 eight times memory: %v, want VMTOOLARGE", err)
	}
}

// TestVMInit_pagingLimits: with paging on, VMINIT refuses an S0 larger
// than the architecture's 1 GB, a layout (the system page table among
// it) that doesn't fit in physical memory, and an S0 too small for its
// layout.
func TestVMInit_pagingLimits(t *testing.T) {
	c := newSizedMachine(t, 256)

	tests := []struct {
		name       string
		p0, p1, s0 uint32
		want       uint32
	}{
		{"S0 past 1 GB", 20, 20, 0x200001, vmserrors.CLI_VMTOOLARGE},
		{"a system page table bigger than memory", 20, 20, 0x200000, vmserrors.CLI_VMTOOLARGE},
		{"S0 smaller than its layout", 20, 20, 40, vmserrors.CLI_S0TOOSMALL},
	}

	for _, tt := range tests {
		err := c.VMInitPaging(true, tt.p0, tt.p1, tt.s0, 2, 2, 2, 2, 8)
		if !errors.Is(err, vmserrors.New(tt.want)) {
			t.Errorf("%s: %v, want %s", tt.name, err, vmserrors.New(tt.want))
		}
	}

	if err := c.VMInitPaging(true, 20, 20, 96, 2, 2, 2, 2, 8); err != nil {
		t.Errorf("a layout that fits: %v", err)
	}
}

// TestVMInit_layoutPages: layoutPages, which VMINIT checks its sizes
// with before laying S0 out, counts the pages it lays out, in both
// modes: S0Free is the first address past them.
func TestVMInit_layoutPages(t *testing.T) {
	c := newSizedMachine(t, 16384)

	tests := []struct {
		paging                    bool
		p0, p1, s0                uint32
		ksp, esp, ssp, isp, strng uint32
	}{
		{true, 0, 0, 0, 20, 0, 0, 4, 8},
		{true, 300, 200, 1000, 4, 3, 5, 2, 1},
		{false, 16384, 8192, 8192, 20, 8, 8, 4, 8},
		{false, 300, 200, 0, 20, 0, 0, 4, 8},
	}

	for _, tt := range tests {
		if err := c.VMInitPaging(tt.paging, tt.p0, tt.p1, tt.s0, tt.ksp, tt.esp, tt.ssp, tt.isp, tt.strng); err != nil {
			t.Fatalf("%+v: %v", tt, err)
		}

		p0, p1 := c.VMInitSizes()
		size := [3]uint32{p0, p1, c.CPU.PR(registers.SLR)}
		esp, ssp := tt.esp, tt.ssp // VMINIT's defaults for 0

		if esp == 0 {
			esp = defaultModeStackPages
		}

		if ssp == 0 {
			ssp = defaultModeStackPages
		}

		if got, want := layoutPages(size, tt.ksp, esp, ssp, tt.isp, tt.strng)*512, c.S0Free()-0x80000000; got != want {
			t.Errorf("%+v: layoutPages %#x bytes, VMINIT laid out %#x", tt, got, want)
		}
	}
}
