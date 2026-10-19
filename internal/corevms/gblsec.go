package corevms

import (
	"maps"
	"slices"

	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Global sections (docs/PHASE-46.md, subtask 5): $CRMPSC, $MGBLSC, and
// $DGBLSC, the way VMS processes share memory.
//
// # What a global section is
//
// A global section is a named run of pages that several processes map
// into their own address spaces: each process's page table entries for
// its range point at the same physical pages, so what one process stores
// there the others read at once, with no copying. A *page-file* section
// (SEC$M_PAGFIL) has no file behind it: its pages start out as zeros and
// its contents last only as long as the section does. It's what programs
// use for shared memory; the processes coordinate their use of it with
// event flags, locks, or interlocked instructions (BBSSI and BBCCI as a
// spinlock, INSQHI and REMQHI for a shared queue), which govax runs
// whole, never split by a process switch.
//
// A section's name is either a *group* name, seen only by processes in
// its creator's UIC group (the default), or a *system* name
// (SEC$M_SYSGBL), seen by every process. Several sections may share a
// name if their version idents differ (see identMatch). A section is
// protected by a UIC protection mask (uicprot.go) against its creator's
// UIC: mapping needs read access, and write access to map it writable.
//
// # Lifetime
//
// From *VAX/VMS Internals and Data Structures*, sections 14.3 and 16.3:
// a section's reference count is how many process page table entries
// map it. A *temporary* section is deleted when that count falls to
// zero: when the last process mapping it deletes the pages ($DELTVA,
// $CRETVA over them), its image exits, or it's deleted. A *permanent*
// one (SEC$M_PERM, which needs the PRMGBL privilege) stays until
// $DGBLSC, which marks the section for deletion: its name goes at once
// (no new mapping finds it, and a new section may take the name), and
// the section itself goes when the last mapping does.
//
// # How govax does it
//
// A section's physical pages belong to the section, not to the processes
// mapping it. A file section's (filesec.go) come in when first touched:
// each has an entry in the global page table (globalPage), and a mapping
// process's PTEs hold its index until the pager reads the page in. With
// paging off, a page-file section's are allocated, zeroed, when it's
// created, and a mapping process's PTEs are valid from the start,
// pointing at them; with paging on, its pages are global demand-zero
// pages (globalpages.go), with global page table entries as a file
// section's, each given a frame of zeros by the first fault anywhere.
// Either way the code that gives back a page when its PTE is replaced
// (releasePTE, TeardownAddressSpace) asks the System first
// (releaseFrame): a page of a section only drops the section's
// reference count. With paging on, a section's page can leave memory
// while the section lasts (globalpages.go).
//
// Sections mapped by PFN (SEC$M_PFNMAP) aren't supported:
// SS$_UNSUPPORTED.

// Status values the section services return.
var (
	ssNoSuchSec  = vmsdef.Symbols["SS$_NOSUCHSEC"]
	ssGptFull    = vmsdef.Symbols["SS$_GPTFULL"]
	ssGsdFull    = vmsdef.Symbols["SS$_GSDFULL"]
	ssIvSecFlg   = vmsdef.Symbols["SS$_IVSECFLG"]
	ssIvSecIdCtl = vmsdef.Symbols["SS$_IVSECIDCTL"]
	ssUnsupport  = vmsdef.Symbols["SS$_UNSUPPORTED"]
)

// The $CRMPSC and $MGBLSC flags ($SECDEF, among STARLET.OLB's
// definitions rather than vmsdef.Symbols).
var (
	secGBL    = vmsdef.LibrarySymbols["SEC$M_GBL"]
	secCRF    = vmsdef.LibrarySymbols["SEC$M_CRF"]
	secDZRO   = vmsdef.LibrarySymbols["SEC$M_DZRO"]
	secWRT    = vmsdef.LibrarySymbols["SEC$M_WRT"]
	secPERM   = vmsdef.LibrarySymbols["SEC$M_PERM"]
	secSYSGBL = vmsdef.LibrarySymbols["SEC$M_SYSGBL"]
	secPFNMAP = vmsdef.LibrarySymbols["SEC$M_PFNMAP"]
	secEXPREG = vmsdef.LibrarySymbols["SEC$M_EXPREG"]
	secPAGFIL = vmsdef.LibrarySymbols["SEC$M_PAGFIL"]
)

// The version ident's match controls ($SECDEF).
var (
	secMatAll = vmsdef.LibrarySymbols["SEC$K_MATALL"]
	secMatEqu = vmsdef.LibrarySymbols["SEC$K_MATEQU"]
	secMatLeq = vmsdef.LibrarySymbols["SEC$K_MATLEQ"]
)

// The privileges the section services check.
var (
	privSYSGBL = privilegeBit("SYSGBL")
	privPRMGBL = privilegeBit("PRMGBL")
)

// maxSectionNameLength is the longest global section name.
const maxSectionNameLength = 43

// The SYSGEN limits on global sections, at govax's defaults (the phase
// doc's question 2: govax's earlier constants, so that no program meets a
// limit it didn't before, and the VAX's GBLPAGFIL): how many sections may
// exist (GBLSECTIONS; SS$_GSDFULL beyond it), how many global page table
// entries they may have in all (GBLPAGES, counted by globalPageEntries;
// SS$_GPTFULL beyond it), and how many pages the page-file sections may
// have in all (GBLPAGFIL; SS$_EXGBLPAGFIL beyond it). The vax.sysgen.*
// keys change them (Sysgen).
const (
	DefaultGlobalSectionLimit  = 128
	DefaultGlobalPageLimit     = 4096
	DefaultGlobalPageFileLimit = 4096
)

// ssExGblPagFil is $CRMPSC's status for a page-file section that would
// take the page-file sections past GBLPAGFIL.
var ssExGblPagFil = vmsdef.Symbols["SS$_EXGBLPAGFIL"]

// globalPageEntries is how many of GBLPAGES a section of pages pages
// uses: a page table entry per page and two more, rounded up to an even
// number (the System Generation utility manual's rule for GBLPAGES).
func globalPageEntries(pages int) int {
	return (pages + 2 + 1) &^ 1
}

// GlobalSection is one global section.
type GlobalSection struct {
	// Name is the section's name, and System whether it's a system
	// section (seen by every process) rather than a group one, seen by
	// the processes in UIC group Group.
	Name   string
	System bool
	Group  uint32

	// Owner is the creator's UIC, and Protection the UIC protection mask
	// mapping is checked against.
	Owner      uint32
	Protection uint16

	// Ident is the version ident: the major ident in bits 24-31, the
	// minor in 0-23.
	Ident uint32

	// Writable is set for a section created with SEC$M_WRT. VMS maps a
	// page-file section read/write on request whether or not it was
	// created writable (testdata/mp/probe3), so nothing checks it.
	// Permanent is set for one created with SEC$M_PERM that $DGBLSC
	// hasn't deleted.
	Writable  bool
	Permanent bool

	// DeletePending is set once $DGBLSC has deleted the section's name:
	// it goes when its last mapping does.
	DeletePending bool

	// Frames are the section's physical pages, in order. A file
	// section's page not read in yet is 0, and, with paging on, one whose
	// frame has gone to another page (globalpages.go); so is a paged
	// page-file section's page not touched yet, or out.
	Frames []uint32

	// Refs is how many process page table entries map the section.
	Refs int

	// File is the file a file section's pages are blocks of, from block
	// VBN; nil for a page-file section. CopyOnRef is set for one created
	// with SEC$M_CRF: each process gets its own copy of a page when it
	// touches it. Dirty marks the pages modified since they were last
	// written back. gpt is the global page table index of the section's
	// first page.
	File      *rms.ACPFile
	VBN       uint32
	CopyOnRef bool
	Dirty     []bool
	gpt       uint32

	// PFC is a file section's read cluster size, its creator's $CRMPSC
	// pfc argument: 0 for PFCDEFAULT (cluster.go). A page-file section
	// has none: the manual says pfc isn't used for one.
	PFC uint32

	// serial numbers the section among every section the system has
	// made, from 1: its frames' owner in the PFN database (frames.go).
	serial uint32

	// paged is set for a page-file section made with paging on: its
	// pages are global demand-zero pages, faulted in through global page
	// table entries (globalpages.go). blocks holds, for each of its pages
	// that's out, the page file block its contents are in (0 for a page
	// never written, still demand zero).
	paged  bool
	blocks []uint32
}

// PageFileBlocks returns, for each page of a page-file section made with
// paging on, the page file block holding its contents while it's out (0
// for a page in memory, or never written); nil for any other section.
func (s *GlobalSection) PageFileBlocks() []uint32 {
	return slices.Clone(s.blocks)
}

// hasPageTable reports whether s's pages have global page table entries,
// which a mapping process's PTEs hold until the page is faulted in: a
// file section's, and a paged page-file section's.
func (s *GlobalSection) hasPageTable() bool {
	return s.File != nil || s.paged
}

// writesBack reports whether s's modified pages go back to its file: a
// writable file section that isn't copy on reference.
func (s *GlobalSection) writesBack() bool {
	return s.File != nil && s.Writable && !s.CopyOnRef
}

// sectionPage is one page of a global section: the section and the
// page's number in it. It's what a physical page of a section, and a
// global page table entry, stand for.
type sectionPage struct {
	s    *GlobalSection
	page int
}

// GlobalSections is the system's table of global sections.
type GlobalSections struct {
	// sections are the sections whose names can be found, in creation
	// order; deleted lists those $DGBLSC has deleted that still have
	// mappings; pending those gone (nothing maps them, their names can't
	// be found) whose entries, frames, and page file blocks wait to be
	// released (ReleasePendingSections).
	sections, deleted, pending []*GlobalSection

	// byFrame finds the section page a physical page is.
	byFrame map[uint32]sectionPage

	// pageTable is the global page table: an entry for each page of each
	// section that has them (hasPageTable), which a mapping process's
	// invalid PTE holds the index of (vm.GlobalPTE). An entry whose
	// section is nil is free.
	pageTable []sectionPage

	// SectionLimit, PageLimit, and PageFileLimit are GBLSECTIONS,
	// GBLPAGES, and GBLPAGFIL.
	SectionLimit, PageLimit, PageFileLimit int

	// pages is how many of GBLPAGES the sections use in all
	// (globalPageEntries), and pageFilePages how many pages the page-file
	// sections have.
	pages, pageFilePages int

	// serials is the last section serial number given (newSerial).
	serials uint32
}

// newSerial gives s the next section serial number.
func (g *GlobalSections) newSerial(s *GlobalSection) {
	g.serials++
	s.serial = g.serials
}

// NewGlobalSections returns an empty table with the default limits.
func NewGlobalSections() *GlobalSections {
	return &GlobalSections{
		byFrame:      map[uint32]sectionPage{},
		SectionLimit:  DefaultGlobalSectionLimit,
		PageLimit:     DefaultGlobalPageLimit,
		PageFileLimit: DefaultGlobalPageFileLimit,
	}
}

// FreePages is how many of GBLPAGES are left (SYI$_FREE_GBLPAGES), and
// FreeSections how many sections more may be made (SYI$_FREE_GBLSECTS).
func (g *GlobalSections) FreePages() int {
	return max(g.PageLimit-g.pages, 0)
}

// FreeSections is how many more sections may be made under GBLSECTIONS.
func (g *GlobalSections) FreeSections() int {
	return max(g.SectionLimit-g.count(), 0)
}

// checkLimits returns the status that stops a new section of pages pages
// (a page-file section's, if pageFile is set): SS$_GSDFULL past
// GBLSECTIONS, SS$_GPTFULL past GBLPAGES, SS$_EXGBLPAGFIL past GBLPAGFIL;
// SS$_NORMAL if none does. The limits are checked in that order
// (unconfirmed: the probe's).
func (g *GlobalSections) checkLimits(pages int, pageFile bool) uint32 {
	switch {
	case g.count() >= g.SectionLimit:
		return ssGsdFull
	case g.pages+globalPageEntries(pages) > g.PageLimit:
		return ssGptFull
	case pageFile && g.pageFilePages+pages > g.PageFileLimit:
		return ssExGblPagFil
	}

	return ssNormal
}

// account adds s's use of GBLPAGES and GBLPAGFIL, of pages pages, to the
// totals, or, with sign -1, takes it away, and adds s to the sections
// (or leaves them alone).
func (g *GlobalSections) account(s *GlobalSection, pages, sign int) {
	g.pages += sign * globalPageEntries(pages)

	if s.File == nil {
		g.pageFilePages += sign * pages
	}
}

// bySerial returns the section, its name deleted or not, whose serial
// number is serial.
func (g *GlobalSections) bySerial(serial uint32) *GlobalSection {
	for _, list := range [][]*GlobalSection{g.sections, g.deleted, g.pending} {
		for _, s := range list {
			if s.serial == serial {
				return s
			}
		}
	}

	return nil
}

// Sections returns the sections whose names can be found, in creation
// order.
func (g *GlobalSections) Sections() []*GlobalSection {
	return slices.Clone(g.sections)
}

// PendingSections returns the sections gone by name whose entries,
// frames, and page file blocks wait to be released (maybeDeleteSection).
func (g *GlobalSections) PendingSections() []*GlobalSection {
	return slices.Clone(g.pending)
}

// count is how many sections use GBLSECTIONS: those that exist, deleted
// ones still mapped and those waiting to be released included.
func (g *GlobalSections) count() int {
	return len(g.sections) + len(g.deleted) + len(g.pending)
}

// sectionIdent is a version ident argument: a match control and the
// ident it applies to.
type sectionIdent struct {
	control, version uint32
}

// identMatch reports whether a section with version ident have matches
// the ident a mapper asks for, by the match control: SEC$K_MATALL
// matches any version, SEC$K_MATEQU only the same major and minor
// idents, SEC$K_MATLEQ the same major ident and a section minor ident at
// least the mapper's (the section is the same version or a compatible
// later one).
func identMatch(have uint32, want sectionIdent) bool {
	switch want.control {
	case secMatEqu:
		return have == want.version
	case secMatLeq:
		return have>>24 == want.version>>24 && have&0xFFFFFF >= want.version&0xFFFFFF
	default:
		return true
	}
}

// find returns the first section, in creation order, named name in the
// scope given (system, or group group) whose ident matches.
func (g *GlobalSections) find(name string, system bool, group uint32, ident sectionIdent) *GlobalSection {
	for _, s := range g.sections {
		if s.Name == name && s.System == system && (system || s.Group == group) && identMatch(s.Ident, ident) {
			return s
		}
	}

	return nil
}

// globalPage returns the section and page number global page table
// entry gi stands for.
func (g *GlobalSections) globalPage(gi uint32) (*GlobalSection, int, bool) {
	if gi >= uint32(len(g.pageTable)) || g.pageTable[gi].s == nil {
		return nil, 0, false
	}

	e := g.pageTable[gi]

	return e.s, e.page, true
}

// allocPageTable gives s's pages global page table entries, a run of
// them, the first free run long enough or new ones at the end; it reports
// false if the table would pass the largest index a PTE can hold.
func (g *GlobalSections) allocPageTable(s *GlobalSection, pages int) bool {
	first, run := -1, 0

	for i, e := range g.pageTable {
		if e.s != nil {
			run = 0

			continue
		}

		if run++; run == pages {
			first = i - pages + 1

			break
		}
	}

	if first < 0 {
		first = len(g.pageTable)
		if first+pages > vm.MaxPTEIndex+1 {
			return false
		}

		g.pageTable = append(g.pageTable, make([]sectionPage, pages)...)
	}

	for i := range pages {
		g.pageTable[first+i] = sectionPage{s, i}
	}

	s.gpt = uint32(first)

	return true
}

// releaseFrame gives back the physical page pfn, which a page table
// entry no longer points at: to the free list, unless it's a global
// section's page, when the section loses a reference instead (and is
// deleted if that was its last, unless it's permanent), and, if the PTE
// had modified the page, the page is marked modified in the section.
// With paging on, a file section's page loses the PTE's share instead,
// its modify bit gathered into the frame's saved modify bit, and the
// frame goes to a list with its last share (globalpages.go).
func (sys *System) releaseFrame(pfn uint32, modified bool) {
	sp, ok := sys.Sections.byFrame[pfn]
	if !ok {
		sys.mem.FreePage(pfn)

		return
	}

	shared, toModified := sys.unshareGlobalFrame(pfn, modified)
	if !shared && modified && sp.s.Dirty != nil {
		sp.s.Dirty[sp.page] = true
	}

	sp.s.Refs--
	sys.maybeDeleteSection(sp.s)

	// A frame left on the modified list by a section that lives on may
	// wake the writer.
	if toModified {
		sys.wakeModifiedPageWriter()
	}
}

// maybeDeleteSection deletes s if nothing maps it and nothing keeps it:
// a temporary section, or one $DGBLSC has deleted. A page-file or
// copy-on-reference section goes at once by name, but its use of
// GBLSECTIONS, GBLPAGES, and GBLPAGFIL, its frames, and its page file
// blocks wait on the pending list for the next global section's creation
// or an image's rundown (ReleasePendingSections): VMS 7.1 held them so
// (testdata/pg65: G2's page-file sections were released as the next was
// made, R4's section kept its blocks, and C5's copy-on-reference section,
// gone when its only mapper had touched every page, still counted at
// C7), where a file section went at once (G3). Why VMS waits isn't
// known, and when else it releases them is govax's choice (unconfirmed).
func (sys *System) maybeDeleteSection(s *GlobalSection) {
	g := sys.Sections
	if s.Refs > 0 || s.Permanent || slices.Contains(g.pending, s) {
		return
	}

	if s.File == nil || s.CopyOnRef {
		g.sections = slices.DeleteFunc(g.sections, func(x *GlobalSection) bool { return x == s })
		g.deleted = slices.DeleteFunc(g.deleted, func(x *GlobalSection) bool { return x == s })
		g.pending = append(g.pending, s)

		return
	}

	sys.releaseSection(s)
}

// ReleasePendingSections releases every section on the pending list
// (maybeDeleteSection): at a global section's creation, and at image
// rundown (DeleteP0).
func (sys *System) ReleasePendingSections() {
	g := sys.Sections

	for len(g.pending) > 0 {
		s := g.pending[0]
		sys.releaseSection(s)
	}
}

// releaseSection releases s, which nothing maps or keeps. A file
// section's modified pages are written back to its file first, and the
// file is let go. Its physical pages go back to the free list (cleared).
// With paging on, a section's pages that hold frames are all in
// transition then (nothing maps the section, so no PTE is valid on them):
// those on the modified list are the ones written, and every frame, on
// either list, is deleted to the free list's head. A paged page-file
// section's pages give back their page file blocks, whether out or in a
// frame's backing store.
func (sys *System) releaseSection(s *GlobalSection) {
	g := sys.Sections

	sys.writeBackSection(s)

	for i, pfn := range s.Frames {
		if pfn == 0 {
			continue
		}

		if frame, e, ok := sys.globalFrame(s, i); ok {
			sys.forgetPageFileBlock(frame, e.Backing)
		}

		delete(g.byFrame, pfn)
		sys.mem.FreePage(pfn)
	}

	for _, vbn := range s.blocks {
		if vbn != 0 {
			_ = sys.FreePageFile(paging.Run{First: vbn, Count: 1})
		}
	}

	if s.hasPageTable() {
		for i := range s.Frames {
			g.pageTable[s.gpt+uint32(i)] = sectionPage{}
		}
	}

	if s.File != nil {
		sys.dropFile(s.File)
	}

	g.account(s, len(s.Frames), -1)
	s.File, s.Frames, s.blocks = nil, nil, nil

	g.sections = slices.DeleteFunc(g.sections, func(x *GlobalSection) bool { return x == s })
	g.deleted = slices.DeleteFunc(g.deleted, func(x *GlobalSection) bool { return x == s })
	g.pending = slices.DeleteFunc(g.pending, func(x *GlobalSection) bool { return x == s })
}

// createSection makes a new page-file section of pages pages, returning
// it or the status that stops it: SS$_GSDFULL, SS$_GPTFULL, or
// SS$_EXGBLPAGFIL past the limits (checkLimits; GBLPAGFIL is taken for
// the whole section at once, and the process's paging file quota isn't
// charged: the 5.0 manual names PGFLQUOTA only for copy-on-reference
// pages), SS$_INSFMEM when physical memory runs out. With paging off, its
// pages are zeroed physical pages allocated now; with paging on, global
// demand-zero pages with global page table entries, and no frames until
// they're touched (globalpages.go). The sections waiting on the pending
// list are released first (maybeDeleteSection).
func (sys *System) createSection(s *GlobalSection, pages int) (*GlobalSection, uint32) {
	g := sys.Sections
	sys.ReleasePendingSections()

	if st := g.checkLimits(pages, true); st != ssNormal {
		return nil, st
	}

	if sys.mem.Paging() != nil {
		if !g.allocPageTable(s, pages) {
			return nil, ssGptFull
		}

		g.newSerial(s)

		s.paged = true
		s.Frames = make([]uint32, pages)
		s.blocks = make([]uint32, pages)

		g.account(s, pages, 1)
		g.sections = append(g.sections, s)

		return s, ssNormal
	}

	g.newSerial(s)

	for i := range pages {
		pfn, ok := sys.mem.AllocateFrame(globalClaim(s, i))
		if !ok {
			for _, f := range s.Frames {
				sys.mem.FreePage(f)
			}

			return nil, ssInsfMem
		}

		// AllocateFrame hands out a cleared frame in either paging mode;
		// clearing it here as well costs little.
		var zero [pageSize]byte
		if err := sys.mem.StorePhysical(pfn*pageSize, zero[:]); err != nil {
			return nil, ssInsfMem
		}

		s.Frames = append(s.Frames, pfn)
	}

	for i, pfn := range s.Frames {
		g.byFrame[pfn] = sectionPage{s, i}
	}

	g.account(s, pages, 1)
	g.sections = append(g.sections, s)

	return s, ssNormal
}

// createFileSection makes a new section of pages pages of s.File, from
// block s.VBN, none read in yet (filesec.go), returning the status that
// stops it: SS$_GSDFULL or SS$_GPTFULL past the limits (or the global
// page table's largest index). The sections waiting on the pending list
// are released first (maybeDeleteSection).
func (sys *System) createFileSection(s *GlobalSection, pages int) uint32 {
	g := sys.Sections
	sys.ReleasePendingSections()

	if st := g.checkLimits(pages, false); st != ssNormal {
		return st
	}

	if !g.allocPageTable(s, pages) {
		return ssGptFull
	}

	g.newSerial(s)

	s.Frames = make([]uint32, pages)
	s.Dirty = make([]bool, pages)

	g.account(s, pages, 1)
	g.sections = append(g.sections, s)
	sys.holdFile(s.File)

	return ssNormal
}

// sectionArgs are the arguments $CRMPSC and $MGBLSC share.
type sectionArgs struct {
	inadr, retadr uint32
	mode          registers.AccessMode
	flags         uint32
	name          string
	ident         sectionIdent
	relpag        uint32

	// pfc is $CRMPSC's page fault cluster argument (cluster.go); $MGBLSC
	// has none.
	pfc uint32
}

// readSectionArgs reads the arguments $CRMPSC and $MGBLSC share: inadr,
// retadr, acmode (maximized with the caller's), flags, gsdnam (1 to 43
// characters, SS$_IVLOGNAM otherwise; SS$_ACCVIO if it can't be read),
// ident (a quadword: match control, SS$_IVSECIDCTL if not one of the
// three, and version), and relpag.
func (env *Environment) readSectionArgs(argv []uint32) (sectionArgs, uint32) {
	a := sectionArgs{
		inadr:  optArg(argv, 0),
		retadr: optArg(argv, 1),
		mode:   max(registers.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod()),
		flags:  optArg(argv, 3),
		relpag: optArg(argv, 6),
	}

	name, st := sectionName(env, optArg(argv, 4))
	if st != 0 {
		return a, st
	}

	a.name = name

	ident, st := env.readIdent(optArg(argv, 5))
	if st != 0 {
		return a, st
	}

	a.ident = ident

	return a, 0
}

// sectionName reads a global section name argument.
func sectionName(env *Environment, desc uint32) (string, uint32) {
	if desc == 0 {
		return "", ssIvLogNam
	}

	name, ok, err := strGet(env, desc, maxSectionNameLength)
	if err != nil {
		return "", ssAccVio
	}

	if !ok || name == "" {
		return "", ssIvLogNam
	}

	return name, 0
}

// readIdent reads a version ident argument at addr; none (0) matches all
// versions and creates version 0.
func (env *Environment) readIdent(addr uint32) (sectionIdent, uint32) {
	if addr == 0 {
		return sectionIdent{control: secMatAll}, 0
	}

	control, err1 := env.mem.LoadLongword(env.cpu, addr)
	version, err2 := env.mem.LoadLongword(env.cpu, addr+4)

	if err1 != nil || err2 != nil {
		return sectionIdent{}, ssAccVio
	}

	if control != secMatAll && control != secMatEqu && control != secMatLeq {
		return sectionIdent{}, ssIvSecIdCtl
	}

	return sectionIdent{control: control, version: version}, 0
}

// serviceSysCrmpsc is SYS$CRMPSC, create and map section:
//
//	SYS$CRMPSC [inadr] ,[retadr] ,[acmode] ,[flags] ,[gsdnam] ,[ident]
//	           ,[relpag] ,[chan] ,[pagcnt] ,[vbn] ,[prot] ,[pfc]
//
// It makes three kinds of section:
//
//   - A *page-file* global section: flags SEC$M_GBL and SEC$M_PAGFIL
//     (with SEC$M_DZRO, implied anyway), of pagcnt pages (SS$_ENDOFFILE
//     for 0, as VMS 7.3 says), zeroed. Its pages count against GBLPAGFIL
//     (SS$_EXGBLPAGFIL past it), not the paging file quota.
//   - A *file* global section: SEC$M_GBL without SEC$M_PAGFIL, of the
//     file accessed on channel chan (filesec.go), from block vbn, pagcnt
//     pages or the rest of the file.
//   - A *private* file section: no SEC$M_GBL; the same, mapped by this
//     process only, with no name. It returns SS$_NORMAL.
//
// Optional flags: SEC$M_WRT (writable pages; a file section's modified
// pages are written back), SEC$M_CRF (copy on reference, file sections
// only: each page a private copy, never written back), SEC$M_SYSGBL
// (needs SYSGBL), SEC$M_PERM (needs PRMGBL), SEC$M_EXPREG.
//
// For a global section, if one named gsdnam whose ident matches already
// exists in the scope, it's mapped (as by $MGBLSC, SEC$M_WRT asking for
// write access) and the service returns SS$_NORMAL; if not, it's created
// with ident's version and protection mask prot, mapped, and the service
// returns SS$_CREATED. The mapping is mapSection's; with no inadr (and
// no SEC$M_EXPREG) a global section is created but not mapped (the
// manual's case for a permanent one). If the mapping fails, a section
// just created goes again (unless permanent).
//
// Other statuses: SS$_IVSECFLG for an unknown flag, SEC$M_PERM or
// SEC$M_SYSGBL without SEC$M_GBL, SEC$M_CRF with SEC$M_PAGFIL, or a
// file section with no channel; SS$_UNSUPPORTED for a section mapped by
// PFN; fileSectionChannel's and fileSectionSize's (SS$_NOPRIV,
// SS$_NOTFILEDEV, SS$_NOWRT, SS$_ENDOFFILE); SS$_NOPRIV; SS$_GSDFULL,
// SS$_GPTFULL, SS$_EXGBLPAGFIL, SS$_INSFMEM; readSectionArgs's and mapRange's. Any
// failure before a page is mapped writes -1 to both longwords of retadr
// (failRetadr).
//
// The statuses for pagcnt 0 and for a file section with no channel are
// VMS 7.3's (testdata/mp/probe3). *Unconfirmed:* which rule gives the
// second SS$_IVSECFLG: probe 3 tried only SEC$M_GBL!SEC$M_WRT!SEC$M_EXPREG
// with channel 0, and govax takes the missing channel as the reason.
func serviceSysCrmpsc(env *Environment, argv []uint32) (uint32, error) {
	known := secGBL | secCRF | secDZRO | secWRT | secPERM | secSYSGBL | secPFNMAP | secEXPREG | secPAGFIL
	flags := optArg(argv, 3)
	chanNumber := optArg(argv, 7) & 0xFFFF

	switch {
	case flags&^known != 0:
		return env.failRetadr(argv, ssIvSecFlg)
	case flags&(secPERM|secSYSGBL|secPAGFIL) != 0 && flags&secGBL == 0:
		return env.failRetadr(argv, ssIvSecFlg)
	case flags&secPAGFIL != 0 && flags&secCRF != 0:
		return env.failRetadr(argv, ssIvSecFlg)
	case flags&secPAGFIL == 0 && chanNumber == 0:
		return env.failRetadr(argv, ssIvSecFlg)
	case flags&secPFNMAP != 0:
		return env.failRetadr(argv, ssUnsupport)
	}

	if flags&secGBL == 0 {
		return env.createPrivateSection(argv)
	}

	a, st := env.readSectionArgs(argv)
	if st != 0 {
		return env.failRetadr(argv, st)
	}

	p := env.Process
	system := flags&secSYSGBL != 0

	if s := env.Sections.find(a.name, system, p.UICGroup(), a.ident); s != nil {
		r, none, st := env.mapRange(a, uint32(max(len(s.Frames)-int(a.relpag), 1)))
		if st != 0 {
			return env.failRetadr(argv, st)
		}

		if none {
			return env.failRetadr(argv, ssNormal)
		}

		return env.mapSection(s, a, r), nil
	}

	if system && !p.hasPrivilege(privSYSGBL) || flags&secPERM != 0 && !p.hasPrivilege(privPRMGBL) {
		return env.failRetadr(argv, ssNoPriv)
	}

	s := &GlobalSection{
		Name:       a.name,
		System:     system,
		Group:      p.UICGroup(),
		Owner:      p.UIC,
		Protection: uint16(optArg(argv, 10)),
		Ident:      a.ident.version,
		Writable:   flags&secWRT != 0,
		Permanent:  flags&secPERM != 0,
	}

	pagcnt := optArg(argv, 8)

	if flags&secPAGFIL != 0 {
		if pagcnt == 0 {
			return env.failRetadr(argv, ssEndOfFile)
		}

		if s, st = env.createSection(s, int(pagcnt)); st != ssNormal {
			return env.failRetadr(argv, st)
		}
	} else {
		f, st := env.fileSectionChannel(chanNumber, flags)
		if st != ssNormal {
			return env.failRetadr(argv, st)
		}

		vbn, pages, st := fileSectionSize(f, optArg(argv, 9), pagcnt)
		if st != ssNormal {
			return env.failRetadr(argv, st)
		}

		s.File, s.VBN, s.CopyOnRef, s.PFC = f, vbn, flags&secCRF != 0, optArg(argv, 11)

		if st := env.createFileSection(s, int(pages)); st != ssNormal {
			return env.failRetadr(argv, st)
		}
	}

	r, none, st := env.mapRange(a, uint32(max(len(s.Frames)-int(a.relpag), 1)))
	if st == 0 && none {
		_ = env.storeRetadr(a.retadr, nil)
		env.maybeDeleteSection(s)

		return ssCreated, nil
	}

	if st == 0 {
		st = env.mapSection(s, a, r)
	}

	if st != ssNormal {
		env.maybeDeleteSection(s)

		return st, nil
	}

	return ssCreated, nil
}

// createPrivateSection is $CRMPSC of a private section: the file
// accessed on argv's channel, from block vbn, pagcnt pages (or the rest
// of the file), mapped at inadr's pages or, with SEC$M_EXPREG, at the
// end of the region inadr names (mapProcessSection). The name, ident,
// relpag, and prot arguments aren't used.
func (env *Environment) createPrivateSection(argv []uint32) (uint32, error) {
	a := sectionArgs{
		inadr:  optArg(argv, 0),
		retadr: optArg(argv, 1),
		mode:   max(registers.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod()),
		flags:  optArg(argv, 3),
		pfc:    optArg(argv, 11),
	}

	f, st := env.fileSectionChannel(optArg(argv, 7)&0xFFFF, a.flags)
	if st != ssNormal {
		return env.failRetadr(argv, st)
	}

	vbn, pages, st := fileSectionSize(f, optArg(argv, 9), optArg(argv, 8))
	if st != ssNormal {
		return env.failRetadr(argv, st)
	}

	// A private section has to be mapped: no inadr is an unreadable one.
	if a.inadr == 0 && a.flags&secEXPREG == 0 {
		return env.failRetadr(argv, ssAccVio)
	}

	r, _, st := env.mapRange(a, pages)
	if st != 0 {
		return env.failRetadr(argv, st)
	}

	// A private file section, made, is SS$_CREATED, as a global one is:
	// VMS 7.3's status (testdata/probe49, round 2, step 7b).
	st = env.mapProcessSection(f, vbn, pages, a, r)
	if st == ssNormal {
		st = ssCreated
	}

	return st, nil
}

// serviceSysMgblsc is SYS$MGBLSC, map global section:
//
//	SYS$MGBLSC inadr ,[retadr] ,[acmode] ,[flags] ,gsdnam ,[ident] ,[relpag]
//
// It maps the section named gsdnam whose ident matches (a group
// section, or with SEC$M_SYSGBL a system one; SS$_NOSUCHSEC if there's
// none), read-only or, with SEC$M_WRT, read/write, as mapSection says.
// Flags other than SEC$M_WRT, SEC$M_SYSGBL, and SEC$M_EXPREG are
// SS$_IVSECFLG.
func serviceSysMgblsc(env *Environment, argv []uint32) (uint32, error) {
	if optArg(argv, 3)&^(secWRT|secSYSGBL|secEXPREG) != 0 {
		return env.failRetadr(argv, ssIvSecFlg)
	}

	a, st := env.readSectionArgs(argv)
	if st != 0 {
		return env.failRetadr(argv, st)
	}

	s := env.Sections.find(a.name, a.flags&secSYSGBL != 0, env.Process.UICGroup(), a.ident)
	if s == nil {
		return env.failRetadr(argv, ssNoSuchSec)
	}

	r, none, st := env.mapRange(a, uint32(max(len(s.Frames)-int(a.relpag), 1)))
	if st != 0 {
		return env.failRetadr(argv, st)
	}

	if none {
		return env.failRetadr(argv, ssAccVio)
	}

	return env.mapSection(s, a, r), nil
}

// serviceSysDgblsc is SYS$DGBLSC, delete global section:
//
//	SYS$DGBLSC [flags] ,gsdnam ,[ident]
//
// It deletes the name of the section gsdnam whose ident matches (a
// group section, or with SEC$M_SYSGBL, the only flag allowed, a system
// one): no later $MGBLSC finds it, and the section goes when its last
// mapping does (at once if nothing maps it). A system section needs
// SYSGBL, a permanent one PRMGBL (SS$_NOPRIV). SS$_NOSUCHSEC if there's
// no such section.
func serviceSysDgblsc(env *Environment, argv []uint32) (uint32, error) {
	flags := optArg(argv, 0)
	if flags&^secSYSGBL != 0 {
		return ssIvSecFlg, nil
	}

	name, st := sectionName(env, optArg(argv, 1))
	if st != 0 {
		return st, nil
	}

	ident, st := env.readIdent(optArg(argv, 2))
	if st != 0 {
		return st, nil
	}

	p, g := env.Process, env.Sections

	s := g.find(name, flags&secSYSGBL != 0, p.UICGroup(), ident)
	if s == nil {
		return ssNoSuchSec, nil
	}

	if s.System && !p.hasPrivilege(privSYSGBL) || s.Permanent && !p.hasPrivilege(privPRMGBL) {
		return ssNoPriv, nil
	}

	g.sections = slices.DeleteFunc(g.sections, func(x *GlobalSection) bool { return x == s })
	g.deleted = append(g.deleted, s)
	s.DeletePending = true
	s.Permanent = false

	env.maybeDeleteSection(s)

	return ssNormal, nil
}

// failRetadr returns status, for a $CRMPSC or $MGBLSC that fails before
// mapping a page, after writing -1 to both longwords of its retadr (if
// any), as VMS 7.3 does (testdata/mp/probe3).
func (env *Environment) failRetadr(argv []uint32, status uint32) (uint32, error) {
	_ = env.storeRetadr(optArg(argv, 1), nil)

	return status, nil
}

// mapRange is the pages a mapping of count pages may use, lowest first,
// before the section's size limits them. With SEC$M_EXPREG they're new
// pages at the end of the region inadr's first address names (P0 if
// there's no inadr): in P0, from the first page above its high-water
// mark (Environment.RegionSize[0]) up; in P1, count pages below its low
// end (expandP1). Otherwise they're the pages inadr names, whichever
// order its addresses are in; none (no inadr) reports none. An
// unreadable inadr is SS$_ACCVIO.
func (env *Environment) mapRange(a sectionArgs, count uint32) (pageRange, bool, uint32) {
	if a.flags&secEXPREG != 0 {
		if a.inadr != 0 {
			start, err := env.mem.LoadLongword(env.cpu, a.inadr)
			if err != nil {
				return pageRange{}, false, ssAccVio
			}

			if start >= p1Base {
				first, st := env.expandP1(count)

				return pageRange{first: first, last: first + (count-1)*pageSize}, false, st
			}
		}

		// Never page 0, which no mode may touch, even in a process whose
		// high-water mark hasn't moved.
		base := max((env.RegionSize[0]+pageMask)&^pageMask, pageSize)

		return pageRange{first: base, last: p1Base - pageSize}, false, 0
	}

	if a.inadr == 0 {
		return pageRange{}, true, 0
	}

	r, ok := env.readRange(a.inadr, true)
	if !ok {
		return pageRange{}, false, ssAccVio
	}

	if r.last < r.first {
		r.first, r.last = r.last, r.first
	}

	return r, false, 0
}

// userStackPages is how much of P1, below the user stack's top
// (UserStackTop), is kept for the stack: P1 expansion ($EXPREG of region
// 1, SEC$M_EXPREG) begins below it.
const userStackPages = 1024

// p1Low is P1's low end: the lowest P1 address in use, below which P1
// grows. Until the process expands P1 it's userStackPages below the user
// stack's top.
func (env *Environment) p1Low() uint32 {
	if env.RegionSize[1] == 0 {
		env.RegionSize[1] = UserStackTop - userStackPages*pageSize
	}

	return env.RegionSize[1]
}

// expandP1 adds count pages to P1, below its low end, returning the
// first (lowest) one. On VMS P1 grows down by lengthening its page table;
// govax's tables are built at their full size, so P1 can grow only as
// far down as its table reaches (the page P1LR names): beyond that it's
// SS$_VASFULL, and no pages are added.
func (env *Environment) expandP1(count uint32) (uint32, uint32) {
	low := env.p1Low()
	bottom := p1Base + env.cpu.PR(registers.P1LR)*pageSize

	if count == 0 || low < bottom || (low-bottom)/pageSize < count {
		return 0, ssVasFull
	}

	env.RegionSize[1] = low - count*pageSize

	return env.RegionSize[1], 0
}

// mapSection maps section s into the pages of r, from the lowest up,
// starting at its page a.relpag (SS$_BADPARAM if it hasn't that many
// pages): as many pages as both have, so a range larger than what's left
// of the section maps only that, and a smaller one only part of the
// section. Each page is owned by a.mode, read/write for a.mode and the
// more privileged modes with SEC$M_WRT, read-only for them without. A
// page-file section's page becomes a valid page pointing at the
// section's physical page (with paging off); a file section's, and with
// paging on a page-file section's, an invalid one holding its global page
// table index, which the pager resolves when it's touched (filesec.go,
// globalpages.go). A page already there is replaced as by $CRETVA
// (SS$_PAGOWNVIO if a more privileged mode owns it; SS$_NOPRIV for a
// system page, SS$_VASFULL beyond the page table). Mapping without the
// access the section's protection gives the process is SS$_NOPRIV; a
// page-file section may be mapped writable whether or not it was created
// so (VMS 7.3, testdata/mp/probe3); a file section created read-only
// can't be, unless it's copy on reference (SS$_NOWRT; unconfirmed).
// relpag at or past the section's end is SS$_ENDOFFILE (the same).
// Mapping past P0's high-water mark moves it, as $CRETVA does. retadr
// receives the range mapped (from storeRetadr; on an error, what was
// mapped before it).
func (env *Environment) mapSection(s *GlobalSection, a sectionArgs, r pageRange) uint32 {
	write := a.flags&secWRT != 0

	want := accessRead
	if write {
		want |= accessWrite
	}

	if !env.Process.uicAccess(s.Owner, s.Protection, want) {
		_ = env.storeRetadr(a.retadr, nil)

		return ssNoPriv
	}

	// A writable mapping of a read-only file section is SS$_NOPRIV, as
	// VMS 7.3's was (testdata/probe49, round 3, step 7f).
	if write && s.File != nil && !s.Writable && !s.CopyOnRef {
		_ = env.storeRetadr(a.retadr, nil)

		return ssNoPriv
	}

	if a.relpag >= uint32(len(s.Frames)) {
		_ = env.storeRetadr(a.retadr, nil)

		return ssEndOfFile
	}

	prot := readOnlyProtections[a.mode]
	if write {
		prot = modeProtections[a.mode]
	}

	status := uint32(ssNormal)
	done := make([]uint32, 0, len(s.Frames)-int(a.relpag))

	for i, addr := a.relpag, r.first; i < uint32(len(s.Frames)) && addr >= r.first && addr <= r.last; i, addr = i+1, addr+pageSize {
		pte := vm.GlobalPTE(s.gpt+i, prot, uint8(a.mode))
		if !s.hasPageTable() {
			pte = vm.ValidPTE(s.Frames[i], prot, uint8(a.mode))
		}

		if st := env.mapSectionPage(s, addr, pte, a.mode); st != ssNormal {
			status = st

			break
		}

		done = append(done, addr)
		env.raiseP0Mark(addr)
	}

	if !env.storeRetadr(a.retadr, done) {
		return ssAccVio
	}

	return status
}

// readOnlyProtections is the protection a read-only mapping gets for
// each access mode: read for that mode and the more privileged ones,
// nothing for the rest (KR, ER, SR, UR).
var readOnlyProtections = [4]vm.Protection{vm.ProtKR, vm.ProtER, vm.ProtSR, vm.ProtUR}

// mapSectionPage maps the page at addr to one of s's pages, giving it
// pte (mapPage).
func (env *Environment) mapSectionPage(s *GlobalSection, addr uint32, pte vm.PTE, mode registers.AccessMode) uint32 {
	// The reference is counted before the old page is given back, so
	// that mapping a page over itself doesn't delete the section.
	s.Refs++

	// A copy-on-reference section's page is charged against the paging
	// file quota as it's mapped (mapChargedPage); its fault makes the
	// process's own copy (pageInGlobal).
	if st := env.mapChargedPage(addr, pte, mode, s.CopyOnRef); st != ssNormal {
		s.Refs--

		return st
	}

	if env.sectionPages == nil {
		env.sectionPages = map[uint32]bool{}
	}

	env.sectionPages[addr] = true

	// A page-file section's pages are valid from the start with paging
	// off (when there are no working sets); any other section's pages
	// enter the working set when they're faulted in.
	if pte.Valid() {
		env.enterWorkingSet(addr, true)
	}

	return ssNormal
}

// unmapSections is image rundown's step for sections, DeleteP0's first
// (so a rundown under KeepP0 keeps them too): VMS deletes the
// image's address space when it exits, and with it every mapping of a
// section; govax keeps the rest of P0, but each page mapped to a section,
// global or private, becomes again the demand-zero page a process starts
// with (ProcessPTE), and the section loses it (releasePTE: a private
// section's modified page is written back, a global section loses the
// reference).
func (env *Environment) unmapSections() {
	pages := slices.Collect(maps.Keys(env.sectionPages))

	for _, ps := range env.procSections {
		if ps == nil {
			continue
		}

		for i, present := range ps.present {
			if present {
				pages = append(pages, ps.first+uint32(i)*pageSize)
			}
		}
	}

	if len(pages) == 0 {
		return
	}

	as := vm.CurrentAddressSpace(env.cpu)
	if env.Space != nil {
		as = env.Space.AddressSpace
	}

	slices.Sort(pages)

	for _, addr := range slices.Compact(pages) {
		pte, err := env.mem.LookupPTEIn(env.cpu, as, addr)
		if err != nil {
			continue
		}

		p1 := addr >= p1Base

		page := addr / pageSize
		if p1 {
			page = (addr - p1Base) / pageSize
		}

		if env.mem.StorePTEIn(env.cpu, as, addr, ProcessPTE(p1, page)) == nil {
			env.releasePTE(addr, pte)
		}
	}

	env.sectionPages = nil
}

func registerSectionServices(t *ServiceTable) {
	t.Register("SYS$CRMPSC", serviceSysCrmpsc)
	t.Register("SYS$MGBLSC", serviceSysMgblsc)
	t.Register("SYS$DGBLSC", serviceSysDgblsc)
}
