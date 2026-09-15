package console

import (
	"fmt"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/rtl"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// maxP1 and spP1 match console_vminit.c's MAX_P1/SP_P1 constants: the top
// of the 1GB P1 virtual address region, and the fixed initial user stack
// pointer within it.
const (
	maxP1 = 0x80000000
	spP1  = 0x7FE00000
	// p1TotalSlots is the total number of P1-region page slots (the region
	// is 1GB, 0x40000000 bytes), matching P1LR's "pages NOT on the list"
	// convention (see internal/vm/translate.go's P1 handling).
	p1TotalSlots = 0x40000000 >> 9
)

// VMInit implements the VMINIT command: builds P0/P1/S0 page tables and
// turns on virtual memory, matching console_vminit.c's console_vminit_dcl
// — using its `#ifndef DYNVM` code path (every page pre-mapped valid up
// front) rather than the `#ifdef DYNVM` demand-paging path the C source's
// default build actually uses (`vax.h` defines `DYNVM`) — see
// docs/PHASE-08.md's progress log for why: DYNVM's on-first-touch page
// allocation needs Phase 02's internal/vm.Translate itself to gain new
// state/API for an invalid PTE to trigger it, which is out of this file's
// scope (console_vminit.c only); pre-mapping every requested page is a
// real, C-source-supported behavior mode, not a fabricated one.
//
// Also not ported, as pure conveniences with no other consumer yet:
// CONSOLE$STRINGPOOL* (serves the inline mini-assembler, Phase 11's scope)
// and the PTE$K_NONE guard page installed one page below each privileged
// stack (done via the C source's own `setpte` mini-parser, also assembler-
// adjacent). CONSOLE$SCRATCH itself *is* reserved below, now that Phase 13's
// image loader and SHIM$ stub synthesis are real consumers.
func (c *Console) VMInit(p0Pages, p1Pages, s0Pages, kspPages, espPages, sspPages, ispPages uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}
	if err := c.requireKernelMode(); err != nil {
		return err
	}

	physPages := c.Mem.Size() >> 9

	size := [3]uint32{p0Pages, p1Pages, s0Pages}
	explicitTotal := size[0] + size[1] + size[2]
	if explicitTotal > physPages {
		return fmt.Errorf("console: requested VM size exceeds physical memory")
	}

	zcount := uint32(0)
	for _, s := range size {
		if s == 0 {
			zcount++
		}
	}
	remaining := physPages - explicitTotal
	var share uint32
	if zcount > 0 {
		share = remaining / zcount
	}
	for i := range size {
		if size[i] == 0 {
			size[i] = share
		}
	}
	if leftover := physPages - (size[0] + size[1] + size[2]); leftover > 0 {
		size[2] += leftover
	}

	page := uint32(0)
	for _, s := range size {
		page += (s*4)/512 + 1
	}
	if page+32 > size[2] {
		return fmt.Errorf("console: S0 region too small to hold its own page tables")
	}

	// Wipe physical memory: we're about to overwrite it all with fresh
	// page tables, matching console_vminit_dcl's own initial zero pass.
	c.Mem = vm.NewMemory(c.Mem.Size())
	c.Engine = cpu.NewEngine(c.CPU, c.Mem)
	c.RTL = rtl.NewEnvironment(c.CPU, c.Mem, c.Devices, c.Logicals, c.In, c.Out)
	c.Engine.SetSystemServices(c)
	c.CPU.SetPR(vax.MAPEN, 0)

	paddr := uint32(0)
	page = 0

	// S0 system region: identity-style page table starting at physical 0.
	c.CPU.SetPR(vax.SBR, 0)
	c.CPU.SetPR(vax.SLR, size[2])
	for i := uint32(0); i < size[2]; i++ {
		var pte vm.PTE
		pte.SetValid(true)
		pte.SetProtection(vm.ProtURKW)
		pte.SetPFN(page)
		page++
		if err := c.Mem.StoreLongword(c.CPU, paddr, uint32(pte)); err != nil {
			return err
		}
		paddr += 4
	}

	// P0 region: grows up from virtual address 0.
	paddr = roundUpPage(paddr)
	p0br := paddr
	c.CPU.SetPR(vax.P0LR, size[0])
	for i := uint32(0); i < size[0]; i++ {
		var pte vm.PTE
		pte.SetValid(true)
		pte.SetProtection(vm.ProtUW)
		pte.SetPFN(page)
		page++
		if i == 0 {
			pte.SetProtection(vm.ProtNA) // guard the bottom-most page
		}
		if err := c.Mem.StoreLongword(c.CPU, paddr, uint32(pte)); err != nil {
			return err
		}
		paddr += 4
	}
	c.CPU.SetPR(vax.P0BR, p0br+0x80000000)

	// P1 region: grows down towards virtual address maxP1.
	paddr = roundUpPage(paddr)
	p1lr := uint32(p1TotalSlots) - size[1]
	c.CPU.SetPR(vax.P1LR, p1lr)
	c.CPU.SetPR(vax.P1BR, (paddr+0x80000000)-p1lr*4)
	for i := uint32(0); i < size[1]; i++ {
		var pte vm.PTE
		pte.SetValid(true)
		pte.SetProtection(vm.ProtUW)
		pte.SetPFN(page)
		page++
		if err := c.Mem.StoreLongword(c.CPU, paddr, uint32(pte)); err != nil {
			return err
		}
		paddr += 4
	}

	// Privileged-mode stacks, allocated in S0 space after the page tables.
	paddr = roundUpPage(paddr)
	c.CPU.SetGPR(vax.SP, spP1) // USP, set below via SetPR too

	ksp := 0x80000000 + paddr + kspPages*512 - 4
	paddr += kspPages * 512
	esp := 0x80000000 + paddr + espPages*512 - 4
	paddr += espPages * 512
	ssp := 0x80000000 + paddr + sspPages*512 - 4
	paddr += sspPages * 512
	isp := paddr + ispPages*512 - 4 // physical: no translation at interrupt time
	paddr += ispPages * 512

	c.CPU.SetPR(vax.KSP, ksp)
	c.CPU.SetPR(vax.ESP, esp)
	c.CPU.SetPR(vax.SSP, ssp)
	c.CPU.SetPR(vax.ISP, isp)
	c.CPU.SetPR(vax.USP, spP1)

	c.CPU.SetGPR(vax.SP, ksp)
	c.CPU.SetGPR(vax.FP, 0)
	c.CPU.SetGPR(vax.AP, 0)

	psl := c.CPU.PSL()
	psl.SetCurMod(vax.Kernel)
	psl.SetPrvMod(vax.User)
	c.CPU.SetPSL(psl)

	// The console scratch page: a single S0 page reserved for synthesizing
	// small code sequences directly (no assembler needed -- see Phase 13's
	// RUN command, which builds its IMAGE$INIT driver procedure here),
	// matching console_vminit_dcl's own CONSOLE$SCRATCH symbol -- ported
	// now that Phase 13 gives it a real consumer (see this file's earlier
	// doc comment on why it wasn't ported at Phase 08).
	scratch := 0x80000000 + paddr
	c.Symbols.Set("CONSOLE$SCRATCH", scratch, SymbolSystem)
	paddr += 512

	// A second, dedicated page for Phase 13's synthesized SHIM$ stubs
	// (shim.go's ensureShims): unlike CONSOLE$SCRATCH, which RUN reuses
	// and overwrites on every invocation for its transient IMAGE$INIT
	// driver, these stubs must stay valid for the life of the process once
	// a G^ fixup resolves to one -- matching the real system's kernel.asm
	// boot assembly writing its `.shim` table into the running microkernel
	// image itself, not into the interactive scratch area. Tracked
	// internally (shimBase) rather than as a named VAX symbol: nothing in
	// the on-disk image format ever references this area directly, only
	// the individual SHIM$<name>_<offset> symbols pointing into it.
	c.shimBase = 0x80000000 + paddr
	c.shimsReady = false
	paddr += 512

	c.CPU.SetPR(vax.SCBB, paddr)

	// Reserve a dedicated page for the SCB itself -- matching
	// console_vminit_dcl's own treatment (a full page, mirrored by its
	// SCBB$BASE convenience symbol), and, more importantly, unlike
	// CONSOLE$SCRATCH/the shim page above, this address was previously
	// never actually advanced past: SCBB pointed at "whatever comes next"
	// with nothing reserving it. That was harmless before Phase 12 (nothing
	// yet computed "the first free S0 address" from it), but a live ASM
	// session depositing kernel.asm's own code starting exactly there would
	// silently overwrite the .SCB vector table it had just written a few
	// bytes into the same page.
	paddr += 512

	// The first S0 virtual address past every VMINIT-reserved region (page
	// tables, privileged stacks, CONSOLE$SCRATCH, the SHIM$ stub page, and
	// the SCB) -- where a live ASM session's own S0 content must start
	// (see asmSession's doc comment and asm.go's Assemble): the assembler's
	// own default S0 origin (0x80000000) is really only valid for a
	// standalone assembly with no live page table backing it, since that
	// literal address is where this VM's S0 page table itself lives.
	c.s0Free = 0x80000000 + paddr

	c.DepositAddr = 0x200
	c.CPU.SetPR(vax.MAPEN, 1)
	c.VMInitValid = true
	c.asmSession = nil // a fresh address space invalidates any prior ASM session's state

	return nil
}

func roundUpPage(addr uint32) uint32 {
	return (addr>>9 + 1) << 9
}
