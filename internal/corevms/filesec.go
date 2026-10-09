package corevms

import (
	"slices"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
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
// by $UPDSEC. A global page's modify bits are spread over the PTEs of the
// processes mapping it; each one's is gathered into the section
// (GlobalSection.Dirty) as that PTE goes, and the section writes its
// modified pages back when it's deleted, or for $UPDSEC.
//
// # How govax leaves room for a pager
//
// Nothing takes a page away yet: once in, a page stays until it's
// unmapped (there's no working set limit and no page file). The pieces a
// working set manager would need are the ones here: the invalid PTE forms
// say where each page lives; PageIn reads a page in by them; writePage
// writes one back; and releasePTE is the one place a page leaving a PTE
// is accounted for. Taking a page away would be writing it back if
// modified (writePage), giving its frame up, and putting the invalid form
// back in the PTE, for PageIn to find again.
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
)

// sectionFile is a file sections are made of: how many sections use it,
// and whether its channel has let go of it, leaving the last section to
// deaccess it.
type sectionFile struct {
	sections int
	orphaned bool
}

// holdFile counts one more section using f.
func (sys *System) holdFile(f *rms.ACPFile) {
	if sys.sectionFiles == nil {
		sys.sectionFiles = map[*rms.ACPFile]*sectionFile{}
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
func (sys *System) dropFile(f *rms.ACPFile) {
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
func (sys *System) orphanFile(f *rms.ACPFile) bool {
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
	// file is the section's file and vbn the block of its first page.
	file *rms.ACPFile
	vbn  uint32

	// first is the virtual address of the section's first page, and
	// present says which of its pages are still mapped to it; mapped
	// counts them (and, while $CRMPSC maps them, one more, so the
	// section isn't dropped by a page it maps over).
	first   uint32
	present []bool
	mapped  int

	// writeBack is set for a section whose modified pages go back to
	// the file: SEC$M_WRT without SEC$M_CRF.
	writeBack bool
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
// physical page goes back to the free list.
func (env *Environment) releasePTE(addr uint32, old vm.PTE) {
	delete(env.sectionPages, addr)

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

	if !old.Valid() {
		return
	}

	if ps, idx, i := env.processSectionAt(addr &^ pageMask); ps != nil {
		if ps.writeBack && old.Modified() {
			_ = env.writePage(ps.file, ps.vbn+uint32(i), old.PFN())
		}

		env.mem.FreePage(old.PFN())
		env.unmapProcessPage(idx, i)

		return
	}

	env.releaseFrame(old.PFN(), old.Modified())
}

// writePage writes physical page pfn to block vbn of f.
func (sys *System) writePage(f *rms.ACPFile, vbn, pfn uint32) error {
	buf := make([]byte, pageSize)

	if err := sys.mem.LoadPhysical(pfn*pageSize, buf); err != nil {
		return err
	}

	return f.WritePage(vbn, buf)
}

// readPage reads block vbn of f into a new physical page, returning its
// number, or false if memory is exhausted or the block can't be read.
func (sys *System) readPage(f *rms.ACPFile, vbn uint32) (uint32, bool) {
	pfn, ok := sys.mem.AllocatePage()
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
// section's page is read from its file into a new page; any other page is
// demand-zero.
func (sys *System) PageIn(f vm.PageFault) (vm.PTE, bool) {
	prot, owner := f.PTE.Protection(), f.PTE.Owner()

	if gi, ok := f.PTE.GlobalIndex(); ok {
		pfn, ok := sys.pageInGlobal(gi)

		return vm.ValidPTE(pfn, prot, owner), ok
	}

	if si, ok := f.PTE.SectionIndex(); ok {
		env := sys.processForSpace(f.Space)
		if env == nil || si >= uint32(len(env.procSections)) || env.procSections[si] == nil {
			return 0, false
		}

		ps := env.procSections[si]
		if f.Addr < ps.first {
			return 0, false
		}

		i := (f.Addr - ps.first) / pageSize
		if i >= uint32(len(ps.present)) {
			return 0, false
		}

		pfn, ok := sys.readPage(ps.file, ps.vbn+i)

		return vm.ValidPTE(pfn, prot, owner), ok
	}

	return sys.mem.DemandZero(f.PTE)
}

// pageInGlobal returns the physical page for global page table entry gi:
// the section's own, read in if this is its first touch, or, for a
// copy-on-reference section, a private copy (and the section loses the
// reference the faulting PTE held).
func (sys *System) pageInGlobal(gi uint32) (uint32, bool) {
	s, i, ok := sys.Sections.globalPage(gi)
	if !ok || s.File == nil {
		return 0, false
	}

	if s.CopyOnRef {
		pfn, ok := sys.readPage(s.File, s.VBN+uint32(i))
		if ok {
			s.Refs--
			sys.maybeDeleteSection(s)
		}

		return pfn, ok
	}

	if s.Frames[i] == 0 {
		pfn, ok := sys.readPage(s.File, s.VBN+uint32(i))
		if !ok {
			return 0, false
		}

		s.Frames[i] = pfn
		sys.Sections.byFrame[pfn] = sectionPage{s, i}
	}

	return s.Frames[i], true
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
func (sys *System) writeBackSection(s *GlobalSection) {
	if !s.writesBack() {
		return
	}

	for i, pfn := range s.Frames {
		if pfn != 0 && s.Dirty[i] {
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
func (env *Environment) mapProcessSection(f *rms.ACPFile, vbn, pages uint32, a sectionArgs, r pageRange) uint32 {
	write := a.flags&secWRT != 0

	ps := &processSection{
		file: f, vbn: vbn, first: r.first,
		present: make([]bool, min(pages, (r.last-r.first)/pageSize+1)),
		mapped:  1, writeBack: write && a.flags&secCRF == 0,
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

		if st := env.mapPage(addr, vm.SectionPTE(idx, prot, uint8(a.mode)), a.mode); st != ssNormal {
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
func (env *Environment) mapPage(addr uint32, pte vm.PTE, mode vax.AccessMode) uint32 {
	if addr >= s0Base {
		return ssNoPriv
	}

	_, _, old, err := env.mem.LookupPTE(env.cpu, addr)
	if err != nil {
		return ssVasFull
	}

	if pageExists(old) && vax.AccessMode(old.Owner()) < mode {
		return ssPagOwnVio
	}

	if !env.replacePTE(addr, old, pte) {
		return ssVasFull
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
// SS$_FILNOTACC for a disk channel with no file accessed (unconfirmed:
// the manual lists no status for it). With SEC$M_WRT and not SEC$M_CRF,
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
		return nil, ssFilNotAcc
	}

	if flags&secWRT != 0 && flags&secCRF == 0 && !c.acp.Writable() {
		return nil, ssNoWrt
	}

	return c.acp, ssNormal
}

// fileSectionSize is how many pages a section of f from block vbn (0
// meaning 1) has: the blocks the file has allocated from vbn on, or
// pagcnt if that's fewer (and not 0). A vbn past the allocation is
// SS$_ENDOFFILE. (The manual: "The specified page count is compared with
// the number of pages in the section file; if they are different, the
// lower value is used". govax counts the file's allocated blocks, not its
// end of file, so that a file just created with an allocation can be
// mapped: unconfirmed.)
func fileSectionSize(f *rms.ACPFile, vbn, pagcnt uint32) (uint32, uint32, uint32) {
	if vbn == 0 {
		vbn = 1
	}

	alloc := f.AllocatedBlocks()
	if vbn > alloc {
		return 0, 0, ssEndOfFile
	}

	pages := alloc - vbn + 1
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
// second longword the address of the first page not written (0 when every
// page was: unconfirmed), and the AST is queued. retadr receives the
// first and last pages of the first run of contiguous pages written (what
// VMS's first $QIO would have covered). It returns SS$_NORMAL if any page
// was written, SS$_NOTMODIFIED if none was (the event flag is set and the
// IOSB written then too, but no AST is queued: unconfirmed); SS$_ACCVIO,
// SS$_ILLEFC, SS$_EXQUOTA (an AST with none of ASTLM left), or SS$_NOPRIV
// (a system page), with -1 in both longwords of retadr.
func serviceSysUpdsec(env *Environment, argv []uint32) (uint32, error) {
	retadr, updflg := optArg(argv, 1), optArg(argv, 3)
	efn, iosb, astadr, astprm := optArg(argv, 4), optArg(argv, 5), optArg(argv, 6), optArg(argv, 7)
	mode := max(vax.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod())

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

// updateSections is $UPDSEC's work over pages: it writes each page that
// should go back (see serviceSysUpdsec) and clears its modify bit. It
// returns the pages written, and, if a write failed, the page's address
// and the status (SS$_NORMAL and 0 otherwise).
func (env *Environment) updateSections(pages []uint32, mode vax.AccessMode, callerOnly bool) ([]uint32, uint32, uint32) {
	var written []uint32

	for _, addr := range pages {
		_, _, pte, err := env.mem.LookupPTE(env.cpu, addr)
		if err != nil || !pageExists(pte) || vax.AccessMode(pte.Owner()) < mode {
			continue
		}

		var err2 error

		switch {
		case pte.Valid():
			if ps, _, i := env.processSectionAt(addr); ps != nil {
				if !ps.writeBack || !pte.Modified() {
					continue
				}

				err2 = env.writePage(ps.file, ps.vbn+uint32(i), pte.PFN())

				break
			}

			sp, ok := env.Sections.byFrame[pte.PFN()]
			if !ok || !sp.s.writesBack() || callerOnly && !pte.Modified() {
				continue
			}

			err2 = env.writePage(sp.s.File, sp.s.VBN+uint32(sp.page), pte.PFN())
			sp.s.Dirty[sp.page] = false

		default:
			// A global page this process hasn't touched: with updflg 0
			// it's written if another process brought it in.
			gi, ok := pte.GlobalIndex()
			if !ok || callerOnly {
				continue
			}

			s, i, ok := env.Sections.globalPage(gi)
			if !ok || !s.writesBack() || s.Frames[i] == 0 {
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
