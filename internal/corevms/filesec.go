package corevms

import (
	"slices"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// File-backed sections (docs/PHASE-49 - record updates and locks.md,
// subtask 9): $CRMPSC of a file's blocks, private or global; the pager
// that brings their pages in; writing modified pages back; and $UPDSEC.
//
// # What a file section is
//
// A program can map a disk file into its address space, so that the
// file's blocks are its pages: reading memory reads the file, and what it
// writes there goes back to the file. A VAX page and a disk block are both
// 512 bytes, so page n of the section is virtual block vbn+n of the file.
// The program opens the file with RMS's user file open (FAB$V_UFO, which
// leaves it accessed on a channel rather than an RMS stream: rms/ufo.go)
// and passes the channel to $CRMPSC. A *private* section is the process's
// alone; a *global* one has a name (gblsec.go) and other processes map the
// same pages. With SEC$M_WRT the pages are writable and modified ones are
// written back to the file; with SEC$M_CRF (copy on reference) each page
// is read into a private copy and never written back, whether writable or
// not.
//
// # Pages come in when touched
//
// Mapping a file section doesn't read the file. Each page's PTE is left
// invalid, in one of the forms *VAX/VMS Internals and Data Structures*
// (section 14.1.1) describes (internal/vm, pager.go), saying where the
// page is:
//
//   - a page of a private section holds a *process section table index*:
//     its entry (processSection, the book's PSTE, section 14.1.3) names
//     the file, the first block, and the first page mapped;
//   - a page of a global section holds a *global page table index*: its
//     entry (globalPage, the book's global page table entry, section
//     14.3) names the section and which of its pages it is.
//
// The first touch faults, and the pager (System.PageIn, installed on
// vm.Memory) reads the block into a new physical page and makes the PTE
// valid. A global section keeps its pages once read: every process that
// maps it then shares them.
//
// # Writing pages back
//
// The hardware sets a valid PTE's modify bit on the first write. A private
// page that's modified is written back to its block when the page is
// unmapped ($DELTVA, $CRETVA over it, image rundown, process deletion) or
// by $UPDSEC. With paging on, it may also leave the working set while
// modified, its frame waiting on the modified page list until the
// modified page writer (mpw.go) or $UPDSEC writes it to its block. A
// global page's modify bits are spread over the PTEs of the
// processes mapping it; each one's is gathered into the section
// (GlobalSection.Dirty) as that PTE goes, and the section writes its
// modified pages back when it's deleted, or for $UPDSEC.
//
// # Paging
//
// With paging off, nothing takes a page away: once in, a page stays
// until it's unmapped. With it on (workingset.go), a private section's
// page can leave the working set: its PTE keeps its frame in the
// transition form, and when the frame is reused the PTE gets its process
// section table index back (BreakTie), for PageIn to read the page in
// again. A copy-on-reference page is different (Phase 65, subtask 2): the
// read makes it the process's own page, backed by the page file and
// "effectively modified", so it goes to the modified list whenever it
// leaves, the modified page writer (mpw.go) writes it to the page file,
// and a reused frame leaves its PTE in the page file form. Copy-on-
// reference pages, private or global, are charged against the paging
// file quota when they're mapped (pgflquota.go). A global section's page
// leaves a working set too (Phase 65, subtask 3; globalpages.go): the
// process's PTE gets its global page table index back, and the page's
// frame stays in memory while any process holds it valid, then waits on
// a list, tied to the global page, until it's faulted back or reused.
//
// # The file stays accessed
//
// A section keeps its file accessed for as long as the section lasts,
// even after the channel is deassigned or deaccessed (the book's window
// control block, which the section holds): sectionFiles counts the
// sections using each file, and the channel's deaccess leaves it to the
// last section.

// Status values the file section services return.
var (
	ssNoWrt       = vmsdef.Symbols["SS$_NOWRT"]
	ssNotFileDev  = vmsdef.Symbols["SS$_NOTFILEDEV"]
	ssNotModified = vmsdef.Symbols["SS$_NOTMODIFIED"]
	ssIvChnlSec   = vmsdef.Symbols["SS$_IVCHNLSEC"]
)

// sectionFile is a file sections are made of: how many sections use it,
// and whether its channel has let go of it, leaving the last section to
// deaccess it.
type sectionFile struct {
	sections int
	orphaned bool
}

// holdFile counts one more section using f.
func (sys *System) holdFile(f PageSource) {
	if sys.sectionFiles == nil {
		sys.sectionFiles = map[PageSource]*sectionFile{}
	}

	sf := sys.sectionFiles[f]
	if sf == nil {
		sf = &sectionFile{}
		sys.sectionFiles[f] = sf
	}

	sf.sections++
}

// dropFile counts one fewer section using f; the last one deaccesses it if
// its channel has let it go.
func (sys *System) dropFile(f PageSource) {
	sf := sys.sectionFiles[f]
	if sf == nil {
		return
	}

	if sf.sections--; sf.sections > 0 {
		return
	}

	delete(sys.sectionFiles, f)

	if sf.orphaned {
		_ = f.Deaccess()
	}
}

// orphanFile is a channel letting go of f ($DASSGN, IO$_DEACCESS): if
// sections still use it, it's left accessed for them, and orphanFile
// reports true (the channel mustn't deaccess it).
func (sys *System) orphanFile(f PageSource) bool {
	sf := sys.sectionFiles[f]
	if sf == nil {
		return false
	}

	sf.orphaned = true

	return true
}

// processSection is an entry of a process's section table: a private
// section of a file.
type processSection struct {
	// file is the section's file (pagesource.go) and vbn the block of its
	// first page.
	file PageSource
	vbn  uint32

	// first is the virtual address of the section's first page, and
	// present says which of its pages are still mapped to it; mapped
	// counts them (and, while $CRMPSC maps them, one more, so the
	// section isn't dropped by a page it maps over).
	first   uint32
	present []bool
	mapped  int

	// writeBack is set for a section whose modified pages go back to
	// the file: SEC$M_WRT without SEC$M_CRF. copyOnRef is set for one
	// made with SEC$M_CRF: each page read becomes the process's own copy,
	// backed by the page file from then on.
	writeBack bool
	copyOnRef bool

	// pfc is the section's read cluster size, $CRMPSC's pfc argument: 0
	// for PFCDEFAULT (cluster.go).
	pfc uint32
}

// addProcessSection puts ps in the first free slot of the process's
// section table, returning its index (the PTEs' process section table
// index), or false if the table is full.
func (env *Environment) addProcessSection(ps *processSection) (uint32, bool) {
	for i, e := range env.procSections {
		if e == nil {
			env.procSections[i] = ps

			return uint32(i), true
		}
	}

	if len(env.procSections) > vm.MaxPTEIndex {
		return 0, false
	}

	env.procSections = append(env.procSections, ps)

	return uint32(len(env.procSections) - 1), true
}

// ProcessSections is how many private sections the process has: the
// entries in use in its section table.
func (env *Environment) ProcessSections() int {
	n := 0

	for _, ps := range env.procSections {
		if ps != nil {
			n++
		}
	}

	return n
}

// unmapProcessPage notes that page i of process section index idx is no
// longer mapped; the section goes with its last page.
func (env *Environment) unmapProcessPage(idx uint32, i int) {
	if idx >= uint32(len(env.procSections)) {
		return
	}

	ps := env.procSections[idx]
	if ps == nil {
		return
	}

	if i >= 0 && i < len(ps.present) && ps.present[i] {
		ps.present[i] = false
		ps.mapped--
	}

	env.maybeDropProcessSection(idx)
}

// maybeDropProcessSection removes process section idx from the table if
// none of its pages is mapped any more, letting its file go.
func (env *Environment) maybeDropProcessSection(idx uint32) {
	ps := env.procSections[idx]
	if ps == nil || ps.mapped > 0 {
		return
	}

	env.procSections[idx] = nil
	env.dropFile(ps.file)
}

// processSectionAt returns the private section whose page addr is mapped
// to, with its index and the page's number in it.
func (env *Environment) processSectionAt(addr uint32) (*processSection, uint32, int) {
	for idx, ps := range env.procSections {
		if ps == nil || addr < ps.first {
			continue
		}

		if i := int((addr - ps.first) / pageSize); i < len(ps.present) && ps.present[i] {
			return ps, uint32(idx), i
		}
	}

	return nil, 0, 0
}

// releasePTE accounts for old, the PTE of the page at addr, leaving it:
// the page is being deleted, replaced, or unmapped. A page of a global
// section, valid or not, drops the section's reference (and, if the PTE's
// modify bit is set, marks the page modified in the section); a page of a
// private section is written back to its file if it's modified and the
// section writes back, and the section loses it; any other valid page's
// physical page goes back to the free list. With paging on, a page in
// transition is released as a valid one is, its frame taken off the
// list it's on, and modified by its saved modify bit.
func (env *Environment) releasePTE(addr uint32, old vm.PTE) {
	delete(env.sectionPages, addr)
	env.leaveWorkingSet(addr)
	env.refundPage(addr)

	if gi, ok := old.GlobalIndex(); ok {
		env.unrefGlobalPage(gi)

		return
	}

	if si, ok := old.SectionIndex(); ok {
		if si < uint32(len(env.procSections)) && env.procSections[si] != nil {
			env.unmapProcessPage(si, int((addr&^pageMask-env.procSections[si].first)/pageSize))
		}

		return
	}

	// A valid page's frame, or, with paging on, the frame a page in
	// transition still holds (workingset.go's heldFrame).
	var pid uint32
	if env.Process != nil {
		pid = env.Process.PID
	}

	// With paging on, the page's page file block, if it has one, whether
	// in its PTE or its frame's backing store.
	env.releasePageFileBlock(pid, addr&^pageMask, old)

	ps, idx, i := env.processSectionAt(addr &^ pageMask)

	pfn, modified, ok := env.heldFrame(pid, addr, old)
	if !ok {
		// A copy-on-reference section's page whose copy is in the page
		// file (its frame reused since the writer wrote it) holds no
		// frame, but the section still loses it.
		if ps != nil {
			env.unmapProcessPage(idx, i)
		}

		return
	}

	if ps != nil {
		if ps.writeBack && modified {
			_ = env.writePage(ps.file, ps.vbn+uint32(i), pfn)
		}

		env.mem.FreePage(pfn)
		env.unmapProcessPage(idx, i)

		return
	}

	env.releaseFrame(pfn, modified)
}

// writePage writes physical page pfn to block vbn of f: an error for a
// file that can't be written (pagesource.go's pageWriter).
func (sys *System) writePage(f PageSource, vbn, pfn uint32) error {
	w, ok := f.(pageWriter)
	if !ok {
		return errReadOnlySource
	}

	buf := make([]byte, pageSize)

	if err := sys.mem.LoadPhysical(pfn*pageSize, buf); err != nil {
		return err
	}

	return w.WritePage(vbn, buf)
}

// readPage reads block vbn of f into a new physical page for the page c
// describes (frames.go), returning its number, or false if memory is
// exhausted or the block can't be read.
func (sys *System) readPage(f PageSource, vbn uint32, c vm.FrameClaim) (uint32, bool) {
	c.Fill = true

	pfn, ok := sys.mem.AllocateFrame(c)
	if !ok {
		return 0, false
	}

	buf := make([]byte, pageSize)

	if f.ReadPage(vbn, buf) != nil || sys.mem.StorePhysical(pfn*pageSize, buf) != nil {
		sys.mem.FreePage(pfn)

		return 0, false
	}

	return pfn, true
}

// PageIn is the system's pager (vm.Pager; vm/pager.go): it resolves a
// fault on an invalid page by the PTE's form. A global page is the
// section's physical page, read from the file the first time any process
// touches it (or, for a copy-on-reference section, a private copy read
// for this process, which then no longer maps the section); a private
// section's page is read from its file into a new page; with paging on,
// a page the modified page writer put in the page file is read back from
// its block (mpw.go); any other page is demand-zero.
//
// Each fault it resolves on a P0 or P1 page counts in the faulting
// process's page faults, by its kind (accounting.go's countFault).
//
// With paging on (workingset.go), the page enters the process's working
// set first, which may take another page out of it, as VMS's pager makes
// room before it takes a frame (so a clean page leaving can give its
// frame to the page coming in); and a page in transition is a soft
// fault, its frame taken back off its list; and a hard fault reads the
// page's neighbors with it (read clustering, cluster.go).
func (sys *System) PageIn(f vm.PageFault) (vm.PTE, bool) {
	env := sys.pagingProcess(f)
	if env != nil {
		env.enterWorkingSet(f.Addr, sys.faultIsGlobal(f.PTE))
	}

	pte, kind, ok := sys.resolveFault(f)
	if ok {
		sys.countFault(f, kind)

		// A hard fault reads its page, and with paging on its neighbors
		// in the same backing store (cluster.go).
		if kind == faultRead && f.Addr < s0Base {
			sys.PageCounts.PagesRead++

			if env != nil {
				sys.readCluster(env, f)
			}
		}
	} else if env != nil {
		env.leaveWorkingSet(f.Addr)
	}

	sys.noteFault(f.Addr, ok)

	return pte, ok
}

// resolveFault is PageIn's work: the page's new PTE by its old one's
// form, and the fault's kind.
func (sys *System) resolveFault(f vm.PageFault) (vm.PTE, faultKind, bool) {
	prot, owner := f.PTE.Protection(), f.PTE.Owner()

	// With paging off no PTE takes the transition form, and an invalid
	// PTE that isn't a section's is demand-zero whatever its frame field
	// holds.
	if sys.mem.Paging() != nil {
		if pfn, ok := f.PTE.Transition(); ok {
			return sys.softFault(f, pfn)
		}
	}

	if gi, ok := f.PTE.GlobalIndex(); ok {
		pfn, kind, ok := sys.pageInGlobal(gi, f)

		return vm.ValidPTE(pfn, prot, owner), kind, ok
	}

	if si, ok := f.PTE.SectionIndex(); ok {
		env := sys.processForSpace(f.Space)
		if env == nil || si >= uint32(len(env.procSections)) || env.procSections[si] == nil {
			return 0, faultRead, false
		}

		ps := env.procSections[si]
		if f.Addr < ps.first {
			return 0, faultRead, false
		}

		i := (f.Addr - ps.first) / pageSize
		if i >= uint32(len(ps.present)) {
			return 0, faultRead, false
		}

		// A copy-on-reference page's tie to its file ends with this read
		// (the book's 15.2.1.3): its backing store is the page file, a
		// block to be allocated when the writer first writes it, and its
		// saved modify bit is set, so that it goes to the modified list,
		// to be written there, whenever it leaves the working set.
		back := sectionBacking(si, ps.vbn+i)
		if ps.copyOnRef {
			back = copyOnRefBacking
		}

		claim := processClaim(env, f.Addr, back)

		pfn, ok := sys.readPage(ps.file, ps.vbn+i, claim)
		if ok && ps.copyOnRef {
			sys.effectivelyModified(pfn, claim)
			sys.chargeFirstTouch(claim)
		}

		return vm.ValidPTE(pfn, prot, owner), faultRead, ok
	}

	// A page whose contents the modified page writer put in the page file
	// (mpw.go), and whose frame has since gone to another page: a hard
	// fault reads the block back. Block 0, a block never allocated, is a
	// page never written, so demand-zero.
	if vbn, ok := f.PTE.PageFileBlock(); ok && vbn != 0 && sys.mem.Paging() != nil {
		pfn, ok := sys.pageInFromPageFile(vbn, f)

		return vm.ValidPTE(pfn, prot, owner), faultRead, ok
	}

	claim := sys.faultClaim(f, demandZeroBacking)

	pte, ok := sys.mem.DemandZeroFor(f.PTE, claim)
	if ok {
		sys.effectivelyModified(pte.PFN(), claim)
		sys.chargeFirstTouch(claim)
	}

	return pte, faultDemandZero, ok
}

// pageInFromPageFile reads block vbn of the page file into a new frame
// for the page fault f touched, returning the frame's number, or false if
// there's no page file open, memory is exhausted, or the block can't be
// read. The frame's backing store is the block, and its contents match
// it: the page is clean, so if it leaves the working set unmodified its
// frame goes to the free list, and it's never written again unless it's
// modified (the book's 15.2.4).
func (sys *System) pageInFromPageFile(vbn uint32, f vm.PageFault) (uint32, bool) {
	return sys.readPageFileBlock(vbn, sys.faultClaim(f, paging.Backing{}))
}

// readPageFileBlock reads block vbn of the page file into a new frame for
// the page claim c describes, its backing store the block, returning the
// frame's number, or false if there's no page file open, memory is
// exhausted, or the block can't be read.
func (sys *System) readPageFileBlock(vbn uint32, c vm.FrameClaim) (uint32, bool) {
	pf := sys.PageFile()
	if pf == nil {
		return 0, false
	}

	c.Backing = pageFileBacking(vbn)
	c.Fill = true

	pfn, ok := sys.mem.AllocateFrame(c)
	if !ok {
		return 0, false
	}

	buf := make([]byte, pageSize)

	if pf.Read(vbn, buf) != nil || sys.mem.StorePhysical(pfn*pageSize, buf) != nil {
		sys.mem.FreePage(pfn)

		return 0, false
	}

	sys.PageCounts.PageFileReads++

	return pfn, true
}

// pageInGlobal returns the physical page for global page table entry gi:
// the section's own, read in if this is its first touch (with paging on,
// a page-file section's is a frame of zeros or its page file block), or,
// for a copy-on-reference section, a private copy for the process fault f
// touched (and the section loses the reference the faulting PTE held);
// and the fault's kind: a read, or a global valid fault if the section's
// page was in memory already. With paging on, a section's own page may
// also be taken back off a list (pageInSharedGlobal).
func (sys *System) pageInGlobal(gi uint32, f vm.PageFault) (uint32, faultKind, bool) {
	s, i, ok := sys.Sections.globalPage(gi)
	if !ok || !s.hasPageTable() {
		return 0, faultRead, false
	}

	if s.CopyOnRef {
		claim := sys.faultClaim(f, copyOnRefBacking)

		pfn, ok := sys.readPage(s.File, s.VBN+uint32(i), claim)
		if ok {
			sys.effectivelyModified(pfn, claim)
			sys.chargeFirstTouch(claim)

			s.Refs--
			sys.maybeDeleteSection(s)
		}

		return pfn, faultRead, ok
	}

	// With paging on, the page may be valid elsewhere, in transition, or
	// out (globalpages.go).
	if sys.mem.Paging() != nil {
		return sys.pageInSharedGlobal(s, i)
	}

	if s.Frames[i] != 0 {
		return s.Frames[i], faultGlobalValid, true
	}

	pfn, ok := sys.readPage(s.File, s.VBN+uint32(i), globalClaim(s, i))
	if !ok {
		return 0, faultRead, false
	}

	s.Frames[i] = pfn
	sys.Sections.byFrame[pfn] = sectionPage{s, i}

	return pfn, faultRead, true
}

// processForSpace returns the process whose address space is as.
func (sys *System) processForSpace(as vm.AddressSpace) *Environment {
	for _, env := range sys.Processes() {
		if env.Space != nil && env.Space.AddressSpace == as {
			return env
		}
	}

	if cur := sys.Current(); cur != nil && vm.CurrentAddressSpace(sys.cpu) == as {
		return cur
	}

	return nil
}

// unrefGlobalPage drops the reference a PTE holding global page table
// index gi had on its section.
func (sys *System) unrefGlobalPage(gi uint32) {
	if s, _, ok := sys.Sections.globalPage(gi); ok {
		s.Refs--
		sys.maybeDeleteSection(s)
	}
}

// writeBackSection writes a global file section's modified pages to its
// file, if it's one whose pages go back (writable, not copy on
// reference), and marks them unmodified.
//
// With paging on, nothing maps a section being deleted, so each page that
// holds a frame is in transition, and a page on the modified list is one
// to write (globalPageModified).
func (sys *System) writeBackSection(s *GlobalSection) {
	if !s.writesBack() {
		return
	}

	for i, pfn := range s.Frames {
		if pfn != 0 && sys.globalPageModified(s, i) {
			_ = sys.writePage(s.File, s.VBN+uint32(i), pfn)
			s.Dirty[i] = false
		}
	}
}

// mapProcessSection maps a new private section of file f, from block vbn,
// pages long, into r from its lowest page: each page owned by a.mode and
// given an invalid PTE holding the section's index, read/write for
// a.mode and the more privileged modes with SEC$M_WRT, read-only for
// them without. A page already there is replaced as by $CRETVA (its
// owner checked). It returns the status and stores retadr as
// mapSection does. SS$_VASFULL if the process's section table is full.
func (env *Environment) mapProcessSection(f PageSource, vbn, pages uint32, a sectionArgs, r pageRange) uint32 {
	write := a.flags&secWRT != 0

	ps := &processSection{
		file: f, vbn: vbn, first: r.first,
		present: make([]bool, min(pages, (r.last-r.first)/pageSize+1)),
		mapped:  1, writeBack: write && a.flags&secCRF == 0,
		copyOnRef: a.flags&secCRF != 0, pfc: a.pfc,
	}

	idx, ok := env.addProcessSection(ps)
	if !ok {
		_ = env.storeRetadr(a.retadr, nil)

		return ssVasFull
	}

	env.holdFile(f)

	prot := readOnlyProtections[a.mode]
	if write {
		prot = modeProtections[a.mode]
	}

	status := uint32(ssNormal)
	done := make([]uint32, 0, len(ps.present))

	for i := range ps.present {
		addr := r.first + uint32(i)*pageSize

		if st := env.mapChargedPage(addr, vm.SectionPTE(idx, prot, uint8(a.mode)), a.mode, ps.copyOnRef); st != ssNormal {
			status = st

			break
		}

		ps.present[i] = true
		ps.mapped++

		done = append(done, addr)
		env.raiseP0Mark(addr)
	}

	// The hold $CRMPSC had while mapping goes; with no page mapped, so
	// does the section.
	ps.mapped--
	env.maybeDropProcessSection(idx)

	if !env.storeRetadr(a.retadr, done) {
		return ssAccVio
	}

	return status
}

// mapPage makes pte the PTE of the page at addr, for mode: a page already
// there is replaced as by $CRETVA (SS$_PAGOWNVIO if a more privileged
// mode owns it; SS$_NOPRIV for a system page, SS$_VASFULL beyond the page
// table).
func (env *Environment) mapPage(addr uint32, pte vm.PTE, mode registers.AccessMode) uint32 {
	if addr >= s0Base {
		return ssNoPriv
	}

	_, _, old, err := env.mem.LookupPTE(env.cpu, addr)
	if err != nil {
		return ssVasFull
	}

	if pageExists(old) && registers.AccessMode(old.Owner()) < mode {
		return ssPagOwnVio
	}

	if !env.replacePTE(addr, old, pte) {
		return ssVasFull
	}

	return ssNormal
}

// mapChargedPage is mapPage for a page that, if charge is set, is
// charged against the paging file quota as it's mapped: a
// copy-on-reference section's page, whose private copy may one day go to
// the page file (the book's 16.2.1.1; the 5.0 manual's SS$_EXQUOTA "while
// creating copy-on-reference ... pages"). With no quota left for it the
// page isn't mapped: SS$_EXQUOTA, the pages before it mapped, as
// $CRETVA's (unconfirmed for $CRMPSC: the probe's). A page already
// charged keeps its charge, as createPage's does.
func (env *Environment) mapChargedPage(addr uint32, pte vm.PTE, mode registers.AccessMode, charge bool) uint32 {
	if charge && !env.canChargePage(addr) {
		return ssExQuota
	}

	if st := env.mapPage(addr, pte, mode); st != ssNormal {
		return st
	}

	if charge {
		env.chargePage(addr, true)
	}

	return ssNormal
}

// raiseP0Mark moves P0's high-water mark past a P0 page just mapped, as
// $CRETVA does.
func (env *Environment) raiseP0Mark(addr uint32) {
	if addr < p1Base && addr+pageSize > env.RegionSize[0] {
		env.RegionSize[0] = addr + pageSize
	}
}

// fileSectionChannel checks the channel a file section is made from
// ($CRMPSC's chan) and returns its file: SS$_NOPRIV for a channel not
// assigned or assigned from a mode more privileged than the caller's (the
// manual's rule), SS$_NOTFILEDEV for a device that isn't a disk, and
// SS$_IVCHNLSEC for a disk channel with no file accessed (VMS 7.3's;
// testdata/probe49, step 7d). With SEC$M_WRT and not SEC$M_CRF,
// a file accessed read-only is SS$_NOWRT.
func (env *Environment) fileSectionChannel(chanNumber, flags uint32) (*rms.ACPFile, uint32) {
	c, ok := env.findChannel(chanNumber)
	if !ok || c.Mode < uint32(env.cpu.PSL().CurMod()) {
		return nil, ssNoPriv
	}

	if c.Device == nil || c.acp == nil && c.Class != iodev.DeviceClassDisk {
		return nil, ssNotFileDev
	}

	if c.acp == nil {
		return nil, ssIvChnlSec
	}

	if flags&secWRT != 0 && flags&secCRF == 0 && !c.acp.Writable() {
		return nil, ssNoWrt
	}

	return c.acp, ssNormal
}

// fileSectionSize is how many pages a section of f from block vbn (0
// meaning 1) has: the file's blocks up to its end of file from vbn on,
// or pagcnt if that's fewer (and not 0). A vbn past the end of file is
// SS$_ENDOFFILE. (The manual: "The specified page count is compared with
// the number of pages in the section file; if they are different, the
// lower value is used". VMS 7.3 counts to the end of file: a file of ten
// blocks with three written maps three pages; testdata/probe49, round 3,
// step 7d. A file a user file open created ends at its allocation.)
func fileSectionSize(f *rms.ACPFile, vbn, pagcnt uint32) (uint32, uint32, uint32) {
	if vbn == 0 {
		vbn = 1
	}

	used := f.EndOfFileBlock()
	if vbn > used {
		return 0, 0, ssEndOfFile
	}

	pages := used - vbn + 1
	if pagcnt != 0 && pagcnt < pages {
		pages = pagcnt
	}

	return vbn, pages, ssNormal
}

// serviceSysUpdsec is SYS$UPDSEC and SYS$UPDSECW, update section file on
// disk:
//
//	SYS$UPDSEC inadr ,[retadr] ,[acmode] ,[updflg] ,[efn] ,[iosb]
//	           ,[astadr] ,[astprm]
//
// It writes the modified pages of file sections in inadr's range (from
// its first address to its second, either order) back to their files.
// A private section's page goes if its PTE's modify bit is set (and the
// section writes back); a global section's (one that writes back) if, with
// updflg 0, the page is in memory at all ("whether they have been
// modified or not"), or, with updflg 1, if the caller's PTE has it
// modified. Each page written is marked unmodified. A page owned by a
// mode more privileged than acmode (maximized with the caller's) is
// passed over (unconfirmed: VMS may refuse the call), as is any page
// that isn't a writable file section's.
//
// It completes at once, as $GETLKI does: the event flag (efn, default 0)
// is cleared, then set; the IOSB's first word gets the status and its
// second longword the address of the first page not written (see below),
// and the AST is queued. retadr receives the
// first and last pages of the first run of contiguous pages written (what
// VMS's first $QIO would have covered). It returns SS$_NORMAL if any page
// was written, SS$_NOTMODIFIED if none was (the event flag is set and the
// IOSB written then too, but no AST is queued: unconfirmed); SS$_ACCVIO,
// SS$_ILLEFC, SS$_EXQUOTA (an AST with none of ASTLM left), or SS$_NOPRIV
// (a system page), with -1 in both longwords of retadr.
func serviceSysUpdsec(env *Environment, argv []uint32) (uint32, error) {
	retadr, updflg := optArg(argv, 1), optArg(argv, 3)
	efn, iosb, astadr, astprm := optArg(argv, 4), optArg(argv, 5), optArg(argv, 6), optArg(argv, 7)
	mode := max(registers.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())

	fail := func(st uint32) (uint32, error) {
		_ = env.storeRetadr(retadr, nil)

		return st, nil
	}

	r, ok := env.readRange(optArg(argv, 0), true)
	if !ok {
		return fail(ssAccVio)
	}

	flags, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return fail(st)
	}

	if astadr != 0 && env.remainingASTs() == 0 {
		return fail(ssExQuota)
	}

	pages := r.pages()
	if slices.ContainsFunc(pages, func(a uint32) bool { return a >= s0Base }) {
		return fail(ssNoPriv)
	}

	*flags &^= 1 << bit

	if iosb != 0 {
		if env.mem.StoreLongword(env.cpu, iosb, 0) != nil || env.mem.StoreLongword(env.cpu, iosb+4, 0) != nil {
			return fail(ssAccVio)
		}
	}

	written, failedAt, ioStatus := env.updateSections(pages, mode, updflg&1 != 0)

	// retadr: the first run of contiguous pages written.
	run := written
	for i := 1; i < len(written); i++ {
		if d := int64(written[i]) - int64(written[i-1]); d != pageSize && d != -pageSize {
			run = written[:i]

			break
		}
	}

	if !env.storeRetadr(retadr, run) {
		return ssAccVio, nil
	}

	status := uint32(ssNormal)
	if len(written) == 0 && ioStatus == ssNormal {
		status, ioStatus = ssNotModified, ssNotModified
	}

	// The IOSB's second longword is "the virtual address of the first
	// page that was not written" (the manual): a failed page's, or after
	// a successful update the first page of the range (in the order it
	// was scanned) not written, as VMS 7.3 reported it (testdata/probe49,
	// round 3, step 7e). When every page was written, the page past the
	// range (unconfirmed); when none needed writing, 0 (VMS's).
	if failedAt == 0 && len(written) > 0 {
		failedAt = firstUnwritten(pages, written)
	}

	if iosb != 0 {
		_ = env.mem.StoreLongword(env.cpu, iosb, ioStatus&0xFFFF)
		_ = env.mem.StoreLongword(env.cpu, iosb+4, failedAt)
	}

	env.postFlag(efn, sched.ClassIOCompletion)

	if astadr != 0 && status == ssNormal {
		env.queueAST(astadr, astprm, uint32(env.cpu.PSL().CurMod()))
	}

	return status, nil
}

// firstUnwritten is the first of pages, a range in the order $UPDSEC
// scanned it, that isn't one of written; or, if all are, the page after
// the range's last in that order.
func firstUnwritten(pages, written []uint32) uint32 {
	for _, p := range pages {
		if !slices.Contains(written, p) {
			return p
		}
	}

	last := pages[len(pages)-1]
	if len(pages) > 1 && pages[0] > last {
		return last - pageSize
	}

	return last + pageSize
}

// updateSections is $UPDSEC's work over pages: it writes each page that
// should go back (see serviceSysUpdsec) and clears its modify bit. With
// paging on, a private section's page is modified by its frame's saved
// modify bit too, and a page out of the working set whose frame waits on
// the modified list is written from there (modifiedTransitionPage). It
// returns the pages written, and, if a write failed, the page's address
// and the status (SS$_NORMAL and 0 otherwise).
func (env *Environment) updateSections(pages []uint32, mode registers.AccessMode, callerOnly bool) ([]uint32, uint32, uint32) {
	var written []uint32

	for _, addr := range pages {
		_, _, pte, err := env.mem.LookupPTE(env.cpu, addr)
		if err != nil || !pageExists(pte) || registers.AccessMode(pte.Owner()) < mode {
			continue
		}

		var err2 error

		switch {
		case pte.Valid():
			if ps, _, i := env.processSectionAt(addr); ps != nil {
				// A page soft-faulted back after it was written has a
				// clean PTE and the modify bit saved in its frame's
				// record.
				saved := env.savedModify(addr, pte.PFN())
				if !ps.writeBack || !pte.Modified() && !saved {
					continue
				}

				err2 = env.writePage(ps.file, ps.vbn+uint32(i), pte.PFN())
				if err2 == nil && saved {
					_ = env.mem.Paging().ClearModified(paging.PFN(pte.PFN()))
				}

				break
			}

			sp, ok := env.Sections.byFrame[pte.PFN()]
			if !ok || !sp.s.writesBack() || callerOnly && !pte.Modified() {
				continue
			}

			err2 = env.writePage(sp.s.File, sp.s.VBN+uint32(sp.page), pte.PFN())
			sp.s.Dirty[sp.page] = false

			// With paging on, the frame's saved modify bit holds what
			// the PTEs that have let go of it wrote.
			if _, _, ok := env.globalFrame(sp.s, sp.page); ok && err2 == nil {
				_ = env.mem.Paging().ClearModified(paging.PFN(pte.PFN()))
			}

		case env.modifiedTransitionPage(addr, pte) != nil:
			// A page out of the working set, its frame waiting on the
			// modified list: written as the modified page writer
			// would, and moved to the free list (mpw.go).
			ps, _, i := env.processSectionAt(addr)
			pfn, _ := pte.Transition()

			err2 = env.writePage(ps.file, ps.vbn+uint32(i), pfn)
			if err2 == nil {
				_ = env.mem.Paging().Cleaned(paging.PFN(pfn))
			}

		default:
			// A global page the caller's PTE doesn't hold valid.
			gi, ok := pte.GlobalIndex()
			if !ok {
				continue
			}

			s, i, ok := env.Sections.globalPage(gi)
			if !ok || !s.writesBack() || s.Frames[i] == 0 {
				continue
			}

			// With paging on, the page may have left the caller's
			// working set, so its global page table entry says what
			// there is to write (globalpages.go), by updflg.
			if env.mem.Paging() != nil {
				wrote, err := env.updateGlobalPage(s, i, callerOnly)
				if !wrote && err == nil {
					continue
				}

				err2 = err

				break
			}

			// With paging off, a page the caller hasn't touched: with
			// updflg 0 it's written if another process brought it in.
			if callerOnly {
				continue
			}

			err2 = env.writePage(s.File, s.VBN+uint32(i), s.Frames[i])
			s.Dirty[i] = false
		}

		if err2 != nil {
			return written, addr, acpStatus(err2)
		}

		if pte.Valid() && pte.Modified() {
			pte.SetModified(false)
			_ = env.mem.StorePTE(env.cpu, addr, pte)
		}

		written = append(written, addr)
	}

	return written, 0, ssNormal
}

func registerFileSectionServices(t *ServiceTable) {
	t.Register("SYS$UPDSEC", serviceSysUpdsec)
	t.Register("SYS$UPDSECW", serviceSysUpdsec)
}
