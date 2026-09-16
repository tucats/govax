package console

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/rtl"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// vmRegion is one entry of Console.Regions, matching vax.h's own struct
// VMREGION (name/pte_count/size/v_start/v_end/p_start/p_end) — see that
// field's own doc comment. PStart/PEnd describe where VMInit's own PFN
// counter had reached when the region's page table was written, exactly as
// console_vminit.c's own "page" variable does — for P0/P1, whose pages are
// now demand-paged (see below) rather than pre-mapped to that counter, this
// remains purely descriptive bookkeeping carried over unchanged from the
// C source's own (pre-DYNVM-support) display, not a claim that a page at
// that physical address is actually resident yet.
type vmRegion struct {
	name           string
	size, pteCount uint32
	vStart, vEnd   uint32
	pStart, pEnd   uint32
}

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
// — taking its `#ifdef DYNVM` demand-paging branch unconditionally (the
// build the C source actually ships, `vax.h` always defining DYNVM), rather
// than a prior version of this file's own `#ifndef DYNVM` eager-mapping
// substitute (docs/PHASE-08.md's progress log has the original rationale
// for that stand-in, which no longer applies now that internal/vm.Memory
// has a real free-page allocator — see AllocatePage/ReservePage/
// MappedPages and translate.go's Translate). S0 is still always eagerly
// mapped, matching the C source's S0 PTE loop having no #ifdef DYNVM branch
// of its own; only P0 and P1 pages start invalid and get demand-paged in on
// first touch.
//
// Also not ported, as a pure convenience with no other consumer yet: the
// PTE$K_NONE guard page installed one page below each privileged stack
// (done via the C source's own `setpte` mini-parser, assembler-adjacent).
// CONSOLE$SCRATCH itself *is* reserved below, now that Phase 13's image
// loader and SHIM$ stub synthesis are real consumers, and CONSOLE$STRINGPOOL*
// is reserved below too, now that expr.go's quoted-string literal support
// is a real consumer (see that file's parseQuotedString).
func (c *Console) VMInit(p0Pages, p1Pages, s0Pages, kspPages, espPages, sspPages, ispPages, stringPoolPages uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if err := c.requireKernelMode(); err != nil {
		return err
	}

	physPages := c.Mem.Size() >> 9

	size := [3]uint32{p0Pages, p1Pages, s0Pages}
	explicitTotal := size[0] + size[1] + size[2]

	// Under DYNVM (this port's only supported mode -- see this function's
	// own doc comment), P0/P1 are virtual page counts backed by demand
	// paging, not a pre-mapped allocation, so requesting more of them than
	// exist in physical memory is not by itself an error -- matching
	// console_vminit_dcl's own `#ifndef DYNVM` guard around this same
	// check (i.e. it's skipped whenever DYNVM is the active build, which is
	// unconditionally the case here).
	if false && (explicitTotal > physPages) {
		return vmserrors.New(vmserrors.CLI_VMTOOLARGE)
	}

	zcount := uint32(0)

	for _, s := range size {
		if s == 0 {
			zcount++
		}
	}

	var share uint32

	if zcount > 0 && physPages > explicitTotal {
		share = (physPages - explicitTotal) / zcount
	}

	for i := range size {
		if size[i] == 0 {
			size[i] = share
		}
	}

	// Any physical pages left over after the explicit/shared sizes above
	// get folded into S0. size[0..2] are virtual page counts and are
	// allowed to exceed physPages (sparse address spaces are intentional,
	// see the disabled check above), so this must only run when there
	// really is a positive leftover: doing the subtraction unconditionally
	// in uint32 underflows and wraps size[2] up to billions of pages,
	// which then runs the S0 page-table-build loop below straight off the
	// end of physical memory.
	if total := size[0] + size[1] + size[2]; physPages > total {
		size[2] += physPages - total
	}

	page := uint32(0)
	for _, s := range size {
		page += (s*4)/512 + 1
	}

	if page+32 > size[2] {
		return vmserrors.New(vmserrors.CLI_S0TOOSMALL)
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

	// S0 system region: identity-style page table starting at physical 0,
	// always eagerly mapped regardless of DYNVM (see the C source's own S0
	// PTE-creation loop, which has no #ifdef DYNVM branch at all — only
	// P0/P1 below start out demand-paged).
	c.CPU.SetPR(vax.SBR, 0)
	c.CPU.SetPR(vax.SLR, size[2])

	for i := uint32(0); i < size[2]; i++ {
		var pte vm.PTE

		pte.SetValid(true)
		pte.SetProtection(vm.ProtURKW)
		pte.SetPFN(page)
		c.Mem.ReservePage(page)

		page++

		if err := c.Mem.StoreLongword(c.CPU, paddr, uint32(pte)); err != nil {
			return err
		}

		paddr += 4
	}

	c.Regions[2] = vmRegion{
		name: "S0", size: size[2], pteCount: (size[2]*4)/512 + 1,
		vStart: 0x80000000, vEnd: 0x80000000 + size[2]<<9,
		pStart: 0, pEnd: size[2] << 9,
	}

	// P0 region: grows up from virtual address 0. PTEs start out invalid
	// (no physical page assigned) and are demand-paged on first touch by
	// internal/vm.Memory.Translate/AllocatePage — this port's only supported
	// mode now, matching console_vminit.c's own #ifdef DYNVM branch (see
	// docs/DEVIATIONS.md on why the non-DYNVM eager-mapping branch this
	// file used to take is no longer replicated).
	paddr = roundUpPage(paddr)
	p0br := paddr
	p0PStart := page

	c.CPU.SetPR(vax.P0LR, size[0])

	for i := uint32(0); i < size[0]; i++ {
		var pte vm.PTE

		pte.SetProtection(vm.ProtUW)

		if i == 0 {
			pte.SetProtection(vm.ProtNA) // guard the bottom-most page
		}

		page++

		if err := c.Mem.StoreLongword(c.CPU, paddr, uint32(pte)); err != nil {
			return err
		}

		paddr += 4
	}

	c.CPU.SetPR(vax.P0BR, p0br+0x80000000)

	c.Regions[0] = vmRegion{
		name: "P0", size: size[0], pteCount: (size[0]*4)/512 + 1,
		vStart: 0, vEnd: size[0] << 9,
		pStart: p0PStart << 9, pEnd: (p0PStart + size[0]) << 9,
	}

	// P1 region: grows down towards virtual address maxP1. Demand-paged on
	// first touch, same as P0 above.
	paddr = roundUpPage(paddr)
	p1lr := uint32(p1TotalSlots) - size[1]
	p1Base := maxP1 - size[1]*512
	p1PStart := page

	c.CPU.SetPR(vax.P1LR, p1lr)
	c.CPU.SetPR(vax.P1BR, (paddr+0x80000000)-p1lr*4)

	for i := uint32(0); i < size[1]; i++ {
		var pte vm.PTE

		pte.SetProtection(vm.ProtUW)

		page++

		if err := c.Mem.StoreLongword(c.CPU, paddr, uint32(pte)); err != nil {
			return err
		}

		paddr += 4
	}

	c.Regions[1] = vmRegion{
		name: "P1", size: size[1], pteCount: (size[1]*4)/512 + 1,
		vStart: p1Base, vEnd: p1Base + size[1]<<9,
		pStart: p1PStart << 9, pEnd: (p1PStart + size[1]) << 9,
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

	// The string pool area, used by expr.go's quoted-string literal support
	// to build VAX string descriptors on the fly -- matching
	// console_vminit_dcl's own "after the SCB and such" placement.
	// CONSOLE$STRINGPOOL is the bump-allocated insertion point (initially
	// == CONSOLE$STRINGPOOL_BASE); the head-of-chain longword ShowString
	// walks (via CONSOLE$STRINGPOOL_BASE) starts zeroed, matching the C
	// source's own initial `store_memory(stringpool, &zero, 4)`. Zeroed via
	// the physical address (paddr), not the virtual one (stringPoolBase):
	// MAPEN is still off here (it isn't set until below), so translation is
	// a no-op passthrough and a virtual S0 address would be read as a raw
	// physical one instead -- unlike the C source, which turns MAPEN on
	// earlier and so can use its virtual `stringpool` address directly for
	// this same write.
	stringPoolSize := stringPoolPages * 512
	stringPoolBase := 0x80000000 + paddr

	c.Symbols.Set("CONSOLE$STRINGPOOL", stringPoolBase, SymbolSystem)
	c.Symbols.Set("CONSOLE$STRINGPOOL_BASE", stringPoolBase, SymbolSystem)
	c.Symbols.Set("CONSOLE$STRINGPOOL_SIZE", stringPoolSize, SymbolSystem)

	// Widen the pool's pages from the rest of S0's default protection
	// (ProtURKW: every mode may read, only kernel may write) to PTE$K_ALL
	// (every mode may read AND write), matching console_vminit_dcl's own
	// `setpte_multiple("... PROT=PTE$K_ALL")` call over this exact address
	// range. Without this, user-mode code building a string descriptor
	// here (expr.go's parseQuotedString, or a running program's own RTL
	// calls) takes a protection-violation page fault -- the S0 loop above
	// leaves every page it creates, this range included, at ProtURKW.
	// Read-modify-write via each page's raw physical PTE address (SBR==0
	// for S0, so S0 page N's PTE lives at physical address N*4) rather
	// than StorePTE/LookupPTE, which both require MAPEN already on -- not
	// yet true this early in VMInit.
	for off := uint32(0); off < stringPoolSize; off += 512 {
		pteAddr := ((paddr + off) >> 9) * 4

		raw, err := c.Mem.LoadLongword(c.CPU, pteAddr)
		if err != nil {
			return err
		}

		pte := vm.PTE(raw)
		pte.SetProtection(vm.ProtUW)

		if err := c.Mem.StoreLongword(c.CPU, pteAddr, uint32(pte)); err != nil {
			return err
		}
	}

	if err := c.Mem.StoreLongword(c.CPU, paddr, 0); err != nil {
		return err
	}

	paddr += stringPoolSize

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
	c.Mem.SetVMValid(true) // let Translate demand-page invalid P0/P1 PTEs from here on
	c.asmSession = nil     // a fresh address space invalidates any prior ASM session's state
	c.assemblerMode = false

	return nil
}

func roundUpPage(addr uint32) uint32 {
	return (addr>>9 + 1) << 9
}
