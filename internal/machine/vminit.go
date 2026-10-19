package machine

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// vmRegion is one entry of Console.Regions, matching vax.h's own struct
// VMREGION (name/pte_count/size/v_start/v_end/p_start/p_end) — see that
// field's own doc comment. PStart/PEnd describe where VMInit's own PFN
// counter had reached when the region's page table was written.
// for P0/P1.
type vmRegion struct {
	name           string
	size, pteCount uint32
	vStart, vEnd   uint32
	pStart, pEnd   uint32
}

// maxP1 and spP1: the top of the P1 virtual address region, and the
// fixed initial user stack pointer within it.
const (
	maxP1 = 0x80000000
	spP1  = corevms.UserStackTop
	// p1TotalSlots is the total number of P1-region page slots (the region
	// is 1GB, 0x40000000 bytes), matching P1LR's "pages NOT on the list"
	// convention (see internal/vm/translate.go's P1 handling).
	p1TotalSlots = 0x40000000 >> 9
)

// VMInit implements the VMINIT command: builds P0/P1/S0 page tables and
// turns on virtual memory, with VMS paging on or off as
// vax.sysgen.paging.enable says (VMInitPaging).
func (m *Machine) VMInit(p0Pages, p1Pages, s0Pages, kspPages, espPages, sspPages, ispPages, stringPoolPages uint32) error {
	return m.VMInitPaging(pagingDefault(), p0Pages, p1Pages, s0Pages, kspPages, espPages, sspPages, ispPages, stringPoolPages)
}

// VMInitPaging is VMINIT with its paging mode given: /PAGING or
// /NOPAGING (docs/PHASE-62 - VMS paging.md, Decision 1). The mode lasts
// until the next VMINIT, INIT, or ZERO, since VMINIT is when physical
// memory is wiped and the page tables built.
func (m *Machine) VMInitPaging(paging bool, p0Pages, p1Pages, s0Pages, kspPages, espPages, sspPages, ispPages, stringPoolPages uint32) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if err := m.requireKernelMode(); err != nil {
		return err
	}

	physPages := m.Mem.Size() >> 9

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
		page += pageTablePages(s)
	}

	if page+32 > size[2] {
		return vmserrors.New(vmserrors.CLI_S0TOOSMALL)
	}

	// Wipe physical memory: we're about to overwrite it all with fresh
	// page tables, matching console_vminit_dcl's own initial zero pass.
	m.EndOtherProcesses()

	m.Mem = vm.NewMemory(m.Mem.Size())
	m.Mem.SetPaging(paging) // the PFN database, or the bitmap without paging
	m.Engine = cpu.NewEngine(m.CPU, m.Mem)
	m.paging = paging
	m.newRTL()

	m.Engine.SetSystemServices(m)
	m.CPU.SetPR(registers.MAPEN, 0)

	paddr := uint32(0)
	page = 0

	// S0 system region: identity-style page table starting at physical 0,
	// always eagerly mapped regardless of DYNVM (see the C source's own S0
	// PTE-creation loop, which has no #ifdef DYNVM branch at all — only
	// P0/P1 below start out demand-paged).
	m.CPU.SetPR(registers.SBR, 0)
	m.CPU.SetPR(registers.SLR, size[2])

	for i := uint32(0); i < size[2]; i++ {
		var pte vm.PTE

		pte.SetValid(true)
		pte.SetProtection(vm.ProtURKW)
		pte.SetPFN(page)
		m.Mem.ReservePage(page)

		page++

		if err := m.Mem.StoreLongword(m.CPU, paddr, uint32(pte)); err != nil {
			return err
		}

		paddr += 4
	}

	m.Regions[2] = vmRegion{
		name: "S0", size: size[2], pteCount: pageTablePages(size[2]),
		vStart: 0x80000000, vEnd: 0x80000000 + size[2]<<9,
		pStart: 0, pEnd: size[2] << 9,
	}

	// P0 region: grows up from virtual address 0. Each PTE is
	// corevms.ProcessPTE's, as every process's are: invalid (no physical
	// page assigned; demand-paged on first touch by
	// internal/vm.Memory.Translate/AllocatePage), owned by user mode (a
	// PTE's owner field, bits 23-24, names the access mode a page belongs
	// to, which $CRETVA/$DELTVA check before replacing or deleting it;
	// docs/PHASE-26.md subtask 35), and, but for page 0, a no-access
	// guard, open to every mode.
	paddr = roundUpPage(paddr)
	p0br := paddr
	p0PStart := page

	// With paging on, P0 starts with no pages and P1 with its permanent
	// part, the rest of each table zero, into which the regions grow
	// (corevms's regions.go); with it off, every page of both exists.
	p0lr, p1Length := corevms.InitialLengths(size[0], size[1], paging)

	m.CPU.SetPR(registers.P0LR, p0lr)

	for i := uint32(0); i < size[0]; i++ {
		pte := corevms.InitialPTE(false, i, p1Length, paging)

		page++

		if err := m.Mem.StoreLongword(m.CPU, paddr, uint32(pte)); err != nil {
			return err
		}

		paddr += 4
	}

	m.CPU.SetPR(registers.P0BR, p0br+0x80000000)

	m.Regions[0] = vmRegion{
		name: "P0", size: size[0], pteCount: pageTablePages(size[0]),
		vStart: 0, vEnd: size[0] << 9,
		pStart: p0PStart << 9, pEnd: (p0PStart + size[0]) << 9,
	}

	// P1 region: grows down towards virtual address maxP1. Demand-paged on
	// first touch, same as P0 above. With paging on the two tables are
	// one table area (corevms.ProcessSpace), P0's growing up from its
	// bottom and P1's down from its top, so P1's table ends at the end of
	// its last page rather than starting at its first.
	paddr = roundUpPage(paddr)
	p1TableEnd := roundUpPage(paddr + size[1]*4)

	if paging {
		paddr = p1TableEnd - size[1]*4
	}

	p1lr := uint32(p1TotalSlots) - size[1]
	p1Base := maxP1 - size[1]*512
	p1PStart := page

	m.CPU.SetPR(registers.P1LR, p1Length)
	m.CPU.SetPR(registers.P1BR, (paddr+0x80000000)-p1lr*4)

	for i := uint32(0); i < size[1]; i++ {
		pte := corevms.InitialPTE(true, p1lr+i, p1Length, paging)

		page++

		if err := m.Mem.StoreLongword(m.CPU, paddr, uint32(pte)); err != nil {
			return err
		}

		paddr += 4
	}

	m.Regions[1] = vmRegion{
		name: "P1", size: size[1], pteCount: pageTablePages(size[1]),
		vStart: p1Base, vEnd: p1Base + size[1]<<9,
		pStart: p1PStart << 9, pEnd: (p1PStart + size[1]) << 9,
	}

	// Privileged-mode stacks, allocated in S0 space after the page tables
	// (docs/MODE-STACKS.md). Each stack pointer starts at its region's
	// last longword; stacks grow down.
	//
	//	kernel stack       kspPages, URKW (as S0's other pages)
	//	guard page         NA
	//	executive stack    espPages (default 8), EW
	//	guard page         NA
	//	supervisor stack   sspPages (default 8), SW
	//	interrupt stack    ispPages, addressed physically
	//
	// The kernel stack keeps S0's URKW: the frame the console builds to
	// CALL a program lives there, and a user-mode program's final RET
	// reads it. Only the executive and supervisor stacks are new: before,
	// they shared the kernel stack's protection (only kernel mode could
	// write them), and a caller passing 0 pages put them at the kernel
	// stack's own address.
	paddr = roundUpPage(paddr)

	if espPages == 0 {
		espPages = defaultModeStackPages
	}

	if sspPages == 0 {
		sspPages = defaultModeStackPages
	}

	m.CPU.SetGPR(registers.SP, spP1) // USP, set below via SetPR too

	ksp := 0x80000000 + paddr + kspPages*512 - 4
	paddr += kspPages * 512

	esp, err := m.modeStack(&paddr, espPages, vm.ProtEW)
	if err != nil {
		return err
	}

	ssp, err := m.modeStack(&paddr, sspPages, vm.ProtSW)
	if err != nil {
		return err
	}

	isp := paddr + ispPages*512 - 4 // physical: no translation at interrupt time
	paddr += ispPages * 512

	m.CPU.SetPR(registers.KSP, ksp)
	m.CPU.SetPR(registers.ESP, esp)
	m.CPU.SetPR(registers.SSP, ssp)
	m.CPU.SetPR(registers.ISP, isp)
	m.CPU.SetPR(registers.USP, spP1)

	m.CPU.SetGPR(registers.SP, ksp)
	m.CPU.SetGPR(registers.FP, 0)
	m.CPU.SetGPR(registers.AP, 0)

	psl := m.CPU.PSL()
	psl.SetCurMod(registers.Kernel)
	psl.SetPrvMod(registers.User)
	m.CPU.SetPSL(psl)

	// The console scratch page: a single S0 page reserved for synthesizing
	// small code sequences directly (no assembler needed -- see Phase 13's
	// RUN command, which builds its IMAGE$INIT driver procedure here),
	// matching console_vminit_dcl's own CONSOLE$SCRATCH symbol -- ported
	// now that Phase 13 gives it a real consumer (see this file's earlier
	// doc comment on why it wasn't ported at Phase 08).
	scratch := 0x80000000 + paddr
	m.Symbols.Set("CONSOLE$SCRATCH", scratch, SymbolSystem)

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
	m.shimBase = 0x80000000 + paddr
	m.shimsReady = false
	paddr += shimPageBytes

	m.CPU.SetPR(registers.SCBB, paddr)

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

	m.Symbols.Set("CONSOLE$STRINGPOOL", stringPoolBase, SymbolSystem)
	m.Symbols.Set("CONSOLE$STRINGPOOL_BASE", stringPoolBase, SymbolSystem)
	m.Symbols.Set("CONSOLE$STRINGPOOL_SIZE", stringPoolSize, SymbolSystem)

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

		raw, err := m.Mem.LoadLongword(m.CPU, pteAddr)
		if err != nil {
			return err
		}

		pte := vm.PTE(raw)
		pte.SetProtection(vm.ProtUW)

		if err := m.Mem.StoreLongword(m.CPU, pteAddr, uint32(pte)); err != nil {
			return err
		}
	}

	if err := m.Mem.StoreLongword(m.CPU, paddr, 0); err != nil {
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
	m.s0Free = 0x80000000 + paddr

	// The rest of S0 is the pool new processes' page tables, stacks, and
	// PCBs come from (corevms/s0pool.go). The microkernel, which ASM puts
	// at s0Free, claims its pages from the pool's bottom as it's
	// deposited (depositAsmImage).
	m.processOne.SetS0Pool(corevms.NewS0Pool(m.s0Free, 0x80000000+size[2]<<9))

	// These P0 and P1 tables are process 1's address space; a new
	// process gets tables of its own from the pool (BuildAddressSpace).
	m.processOne.Space = corevms.AdoptAddressSpace(m.CPU, size[0], size[1])
	m.processOne.Stacks = corevms.AdoptStacks(m.CPU)

	// VIRTUALPAGECNT, every new process's table area, is process 1's
	// unless the configuration says otherwise.
	if m.processOne.Sysgen.VirtualPageCount == 0 {
		m.processOne.Sysgen.VirtualPageCount = size[0] + size[1]
	}

	m.DepositAddr = 0x200
	m.CPU.SetPR(registers.MAPEN, 1)
	m.VMInitValid = true
	m.Mem.SetVMValid(true) // let Translate demand-page invalid P0/P1 PTEs from here on

	// With paging on, the pages of process 1's table area that hold no
	// entry of a page that exists give their frames back, to be backed
	// again as its regions grow (corevms's tablearea.go).
	m.processOne.TrimTableArea(m.processOne.Space)

	// With paging on, the S0 pool's pages give their frames back too:
	// each gets one when the pool hands it out, or when the microkernel's
	// ASM claims it (corevms's s0pool.go).
	m.processOne.UnbackS0Pool()

	// Invalidate_tb() plus a reset of its tries/hits/pflushes
	// counters (tb_flush itself and the STC's own counters are left
	// alone, same asymmetry as CLEAR TB — see docs/PHASE-21.md).
	m.Mem.InvalidateTB()
	m.Mem.ResetTBCounters()
	m.asmSession = nil // a fresh address space invalidates any prior ASM session's state
	m.assemblerMode = false
	m.kernelPlaced = false

	return nil
}

// VMInitSizes returns the P0 and P1 sizes, in pages, the last VMINIT
// gave process 1's page tables (/P0 and /P1): with paging off its
// regions' sizes, with it on its table area's split.
func (m *Machine) VMInitSizes() (p0, p1 uint32) {
	return m.Regions[0].size, m.Regions[1].size
}

// defaultModeStackPages is the size of the executive and supervisor
// stacks when VMINIT isn't given one (docs/MODE-STACKS.md).
const defaultModeStackPages = 8

// modeStack lays out one mode's stack at the physical S0 address *paddr:
// a no-access guard page, then `pages` pages with protection prot. It
// advances *paddr past them and returns the stack's initial pointer (the
// virtual address of its last longword). A push past the stack's bottom
// lands on the guard page and faults.
func (m *Machine) modeStack(paddr *uint32, pages uint32, prot vm.Protection) (uint32, error) {
	if err := m.setS0Protection(*paddr, 1, vm.ProtNA); err != nil {
		return 0, err
	}

	*paddr += 512

	if err := m.setS0Protection(*paddr, pages, prot); err != nil {
		return 0, err
	}

	*paddr += pages * 512

	return 0x80000000 + *paddr - 4, nil
}

// setS0Protection sets the protection of the pages S0 pages starting at
// physical (and S0-offset) address paddr. It edits the PTEs through their
// physical addresses (SBR is 0, so S0 page N's PTE is at N*4), since
// VMINIT runs this before turning MAPEN on.
func (m *Machine) setS0Protection(paddr, pages uint32, prot vm.Protection) error {
	for i := uint32(0); i < pages; i++ {
		pteAddr := (paddr>>9 + i) * 4

		raw, err := m.Mem.LoadLongword(m.CPU, pteAddr)
		if err != nil {
			return err
		}

		pte := vm.PTE(raw)
		pte.SetProtection(prot)

		if err := m.Mem.StoreLongword(m.CPU, pteAddr, uint32(pte)); err != nil {
			return err
		}
	}

	return nil
}

// roundUpPage rounds addr up to the next page boundary; an address
// already on one is unchanged.
func roundUpPage(addr uint32) uint32 {
	return (addr + 511) &^ 511
}

// pageTablePages is how many pages a page table of n entries (4 bytes
// each, 128 to a page) fills.
func pageTablePages(n uint32) uint32 {
	return (n*4 + 511) / 512
}
