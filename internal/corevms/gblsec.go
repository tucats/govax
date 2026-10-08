package corevms

import (
	"maps"
	"slices"

	"github.com/tucats/govax/internal/vax"
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
// A section's physical pages are allocated, zeroed, when it's created,
// and stay with it until it's deleted: the section owns them, not the
// processes mapping it. A mapping process's PTEs are valid from the
// start, pointing at the section's pages (VMS's point at global page
// table entries and fault the pages in; there's no paging here). So the
// code that gives back a page's physical page when its PTE is replaced
// (replacePTE, TeardownAddressSpace) asks the System first
// (releaseFrame): a page of a section only drops the section's
// reference count.
//
// File-backed sections (a $CRMPSC of a file's blocks, private or
// global) and sections mapped by PFN aren't supported: SS$_UNSUPPORTED.

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

// The SYSGEN limits on global sections, at govax's defaults: how many
// sections may exist (GBLSECTIONS; SS$_GSDFULL beyond it) and how many
// pages they may have in all (GBLPAGES; SS$_GPTFULL beyond it).
const (
	DefaultGlobalSectionLimit = 128
	DefaultGlobalPageLimit    = 4096
)

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

	// Writable is set for a section created with SEC$M_WRT, which may be
	// mapped read/write; Permanent for one created with SEC$M_PERM that
	// $DGBLSC hasn't deleted.
	Writable  bool
	Permanent bool

	// DeletePending is set once $DGBLSC has deleted the section's name:
	// it goes when its last mapping does.
	DeletePending bool

	// Frames are the section's physical pages, in order.
	Frames []uint32

	// Refs is how many process page table entries map the section.
	Refs int
}

// GlobalSections is the system's table of global sections.
type GlobalSections struct {
	// sections are the sections whose names can be found, in creation
	// order; deleted lists those $DGBLSC has deleted that still have
	// mappings.
	sections, deleted []*GlobalSection

	// byFrame finds the section a physical page belongs to.
	byFrame map[uint32]*GlobalSection

	// SectionLimit and PageLimit are GBLSECTIONS and GBLPAGES.
	SectionLimit, PageLimit int

	// pages is how many pages the sections have in all.
	pages int
}

// NewGlobalSections returns an empty table with the default limits.
func NewGlobalSections() *GlobalSections {
	return &GlobalSections{
		byFrame:      map[uint32]*GlobalSection{},
		SectionLimit: DefaultGlobalSectionLimit,
		PageLimit:    DefaultGlobalPageLimit,
	}
}

// Sections returns the sections whose names can be found, in creation
// order.
func (g *GlobalSections) Sections() []*GlobalSection {
	return slices.Clone(g.sections)
}

// count is how many sections exist, deleted ones still mapped included.
func (g *GlobalSections) count() int {
	return len(g.sections) + len(g.deleted)
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

// releaseFrame gives back the physical page pfn, which a page table
// entry no longer points at: to the free list, unless it's a global
// section's page, when the section loses a reference instead (and is
// deleted if that was its last, unless it's permanent).
func (sys *System) releaseFrame(pfn uint32) {
	s := sys.Sections.byFrame[pfn]
	if s == nil {
		sys.mem.FreePage(pfn)

		return
	}

	s.Refs--
	sys.maybeDeleteSection(s)
}

// maybeDeleteSection deletes s if nothing maps it and nothing keeps it:
// a temporary section, or one $DGBLSC has deleted. Its physical pages
// go back to the free list (cleared).
func (sys *System) maybeDeleteSection(s *GlobalSection) {
	g := sys.Sections
	if s.Refs > 0 || s.Permanent {
		return
	}

	for _, pfn := range s.Frames {
		delete(g.byFrame, pfn)
		sys.mem.FreePage(pfn)
	}

	g.pages -= len(s.Frames)
	s.Frames = nil

	g.sections = slices.DeleteFunc(g.sections, func(x *GlobalSection) bool { return x == s })
	g.deleted = slices.DeleteFunc(g.deleted, func(x *GlobalSection) bool { return x == s })
}

// createSection makes a new section of pages zeroed physical pages,
// returning it or the status that stops it: SS$_GSDFULL or SS$_GPTFULL
// past the limits, SS$_INSFMEM when physical memory runs out.
func (sys *System) createSection(s *GlobalSection, pages int) (*GlobalSection, uint32) {
	g := sys.Sections

	if g.count() >= g.SectionLimit {
		return nil, ssGsdFull
	}

	if g.pages+pages > g.PageLimit {
		return nil, ssGptFull
	}

	for range pages {
		pfn, ok := sys.mem.AllocatePage()
		if !ok {
			for _, f := range s.Frames {
				sys.mem.FreePage(f)
			}

			return nil, ssInsfMem
		}

		// A page never used reads as zeros, and FreePage clears a page
		// it gives back; clearing it here as well costs little.
		var zero [pageSize]byte
		if err := sys.mem.StorePhysical(pfn*pageSize, zero[:]); err != nil {
			return nil, ssInsfMem
		}

		s.Frames = append(s.Frames, pfn)
	}

	for _, pfn := range s.Frames {
		g.byFrame[pfn] = s
	}

	g.pages += pages
	g.sections = append(g.sections, s)

	return s, ssNormal
}

// sectionArgs are the arguments $CRMPSC and $MGBLSC share.
type sectionArgs struct {
	inadr, retadr uint32
	mode          vax.AccessMode
	flags         uint32
	name          string
	ident         sectionIdent
	relpag        uint32
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
		mode:   max(vax.AccessMode(optArg(argv, 2)&3), env.cpu.PSL().CurMod()),
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
// govax supports page-file global sections: flags SEC$M_GBL and
// SEC$M_PAGFIL (with SEC$M_DZRO, implied anyway), and optionally
// SEC$M_WRT, SEC$M_SYSGBL (needs SYSGBL), SEC$M_PERM (needs PRMGBL), and
// SEC$M_EXPREG. If a section named gsdnam whose ident matches already
// exists in the scope, it's mapped (as by $MGBLSC, SEC$M_WRT asking for
// write access) and the service returns SS$_NORMAL; if not, a section of
// pagcnt pages (SS$_ILLPAGCNT for 0) is created, with ident's version
// and protection mask prot, mapped, and the service returns SS$_CREATED.
// The mapping is mapSection's. If it fails, a section just created goes
// again (unless permanent).
//
// Other statuses: SS$_IVSECFLG for an unknown flag, SEC$M_PERM or
// SEC$M_SYSGBL without SEC$M_GBL, or SEC$M_CRF with SEC$M_PAGFIL;
// SS$_UNSUPPORTED for a section of a file (no SEC$M_PAGFIL) or by PFN;
// SS$_NOPRIV; SS$_GSDFULL, SS$_GPTFULL, SS$_INSFMEM; readSectionArgs's.
func serviceSysCrmpsc(env *Environment, argv []uint32) (uint32, error) {
	known := secGBL | secCRF | secDZRO | secWRT | secPERM | secSYSGBL | secPFNMAP | secEXPREG | secPAGFIL
	flags := optArg(argv, 3)

	switch {
	case flags&^known != 0:
		return ssIvSecFlg, nil
	case flags&(secPERM|secSYSGBL|secPAGFIL) != 0 && flags&secGBL == 0:
		return ssIvSecFlg, nil
	case flags&secPAGFIL != 0 && flags&secCRF != 0:
		return ssIvSecFlg, nil
	case flags&secPFNMAP != 0, flags&secPAGFIL == 0:
		return ssUnsupport, nil
	}

	a, st := env.readSectionArgs(argv)
	if st != 0 {
		return st, nil
	}

	r, st := env.mapRange(a)
	if st != 0 {
		return st, nil
	}

	p := env.Process
	system := flags&secSYSGBL != 0

	if s := env.Sections.find(a.name, system, p.UICGroup(), a.ident); s != nil {
		return env.mapSection(s, a, r), nil
	}

	pagcnt := optArg(argv, 8)
	if pagcnt == 0 {
		return ssIllPagCnt, nil
	}

	if system && !p.hasPrivilege(privSYSGBL) || flags&secPERM != 0 && !p.hasPrivilege(privPRMGBL) {
		return ssNoPriv, nil
	}

	s, st := env.createSection(&GlobalSection{
		Name:       a.name,
		System:     system,
		Group:      p.UICGroup(),
		Owner:      p.UIC,
		Protection: uint16(optArg(argv, 10)),
		Ident:      a.ident.version,
		Writable:   flags&secWRT != 0,
		Permanent:  flags&secPERM != 0,
	}, int(pagcnt))
	if st != ssNormal {
		return st, nil
	}

	if st := env.mapSection(s, a, r); st != ssNormal {
		env.maybeDeleteSection(s)

		return st, nil
	}

	return ssCreated, nil
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
		return ssIvSecFlg, nil
	}

	a, st := env.readSectionArgs(argv)
	if st != 0 {
		return st, nil
	}

	r, st := env.mapRange(a)
	if st != 0 {
		return st, nil
	}

	s := env.Sections.find(a.name, a.flags&secSYSGBL != 0, env.Process.UICGroup(), a.ident)
	if s == nil {
		return ssNoSuchSec, nil
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

// mapRange is the pages a mapping may use, lowest first, before the
// section's size limits them: with SEC$M_EXPREG, every page from the
// first page above P0's high-water mark (Environment.RegionSize[0]) up
// (inadr, if given, must name P0: expanding P1 isn't supported); else
// the pages inadr names, whichever order its addresses are in. An
// unreadable inadr is SS$_ACCVIO.
func (env *Environment) mapRange(a sectionArgs) (pageRange, uint32) {
	if a.flags&secEXPREG != 0 {
		if a.inadr != 0 {
			start, err := env.mem.LoadLongword(env.cpu, a.inadr)
			if err != nil {
				return pageRange{}, ssAccVio
			}

			if start >= p1Base {
				return pageRange{}, ssUnsupport
			}
		}

		// Never page 0, which no mode may touch, even in a process whose
		// high-water mark hasn't moved.
		base := max((env.RegionSize[0]+pageMask)&^pageMask, pageSize)

		return pageRange{first: base, last: p1Base - pageSize}, 0
	}

	r, ok := env.readRange(a.inadr, true)
	if !ok {
		return pageRange{}, ssAccVio
	}

	if r.last < r.first {
		r.first, r.last = r.last, r.first
	}

	return r, 0
}

// mapSection maps section s into the pages of r, from the lowest up,
// starting at its page a.relpag (SS$_BADPARAM if it hasn't that many
// pages): as many pages as both have, so a range larger than what's left
// of the section maps only that, and a smaller one only part of the
// section. Each page becomes a valid page pointing at the section's
// physical page, owned by a.mode, read/write for a.mode and the more
// privileged modes with SEC$M_WRT, read-only for them without. A page
// already there is replaced as by $CRETVA (SS$_PAGOWNVIO if a more
// privileged mode owns it; SS$_NOPRIV for a system page, SS$_VASFULL
// beyond the page table). Mapping writable a section not created
// writable, or without the access its protection gives the process, is
// SS$_NOPRIV. Mapping past P0's high-water mark moves it, as $CRETVA
// does. retadr receives the range mapped (from storeRetadr; on an error,
// what was mapped before it).
func (env *Environment) mapSection(s *GlobalSection, a sectionArgs, r pageRange) uint32 {
	write := a.flags&secWRT != 0

	want := accessRead
	if write {
		want |= accessWrite
	}

	if write && !s.Writable || !env.Process.uicAccess(s.Owner, s.Protection, want) {
		return ssNoPriv
	}

	if a.relpag >= uint32(len(s.Frames)) {
		return ssBadParam
	}

	frames := s.Frames[a.relpag:]

	prot := readOnlyProtections[a.mode]
	if write {
		prot = modeProtections[a.mode]
	}

	status := uint32(ssNormal)
	done := make([]uint32, 0, len(frames))

	for i, addr := 0, r.first; i < len(frames) && addr >= r.first && addr <= r.last; i, addr = i+1, addr+pageSize {
		if st := env.mapSectionPage(s, addr, frames[i], prot, a.mode); st != ssNormal {
			status = st

			break
		}

		done = append(done, addr)

		if addr < p1Base && addr+pageSize > env.RegionSize[0] {
			env.RegionSize[0] = addr + pageSize
		}
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

// mapSectionPage maps the page at addr onto s's physical page pfn.
func (env *Environment) mapSectionPage(s *GlobalSection, addr, pfn uint32, prot vm.Protection, mode vax.AccessMode) uint32 {
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

	var pte vm.PTE

	pte.SetValid(true)
	pte.SetPFN(pfn)
	pte.SetProtection(prot)
	pte.SetOwner(uint8(mode))

	// The reference is counted before the old page is given back, so
	// that mapping a page over itself doesn't delete the section.
	s.Refs++

	if !env.replacePTE(addr, old, pte) {
		s.Refs--

		return ssVasFull
	}

	if env.sectionPages == nil {
		env.sectionPages = map[uint32]bool{}
	}

	env.sectionPages[addr] = true

	return ssNormal
}

// unmapSections is image rundown's step for global sections: VMS
// deletes the image's address space when it exits, and with it every
// mapping of a section; govax keeps the rest of P0, but each page mapped
// to a section becomes again the demand-zero page a process starts with
// (ProcessPTE), and the section loses the reference.
func (env *Environment) unmapSections() {
	if len(env.sectionPages) == 0 {
		return
	}

	as := vm.CurrentAddressSpace(env.cpu)
	if env.Space != nil {
		as = env.Space.AddressSpace
	}

	for _, addr := range slices.Sorted(maps.Keys(env.sectionPages)) {
		pte, err := env.mem.LookupPTEIn(env.cpu, as, addr)
		if err != nil {
			continue
		}

		p1 := addr >= p1Base

		page := addr / pageSize
		if p1 {
			page = (addr - p1Base) / pageSize
		}

		if env.mem.StorePTEIn(env.cpu, as, addr, ProcessPTE(p1, page)) == nil && pte.Valid() {
			env.releaseFrame(pte.PFN())
		}
	}

	env.sectionPages = nil
}

func registerSectionServices(t *ServiceTable) {
	t.Register("SYS$CRMPSC", serviceSysCrmpsc)
	t.Register("SYS$MGBLSC", serviceSysMgblsc)
	t.Register("SYS$DGBLSC", serviceSysDgblsc)
}
