package corevms

import (
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// Working sets (docs/PHASE-63 - working sets.md, subtask 3).
//
// # What a working set is
//
// A process's working set is the set of its pages that are valid in its
// page tables right now: the ones it can touch without a page fault.
// VMS keeps it as a list with a limit (WSLIMIT, which $ADJWSL moves),
// and when a process faults on a page while its list is full, the pager
// takes some other page of the process out of the list to make room:
// the oldest, by a "modified first-in/first-out" rule (internal/paging's
// WorkingSet keeps the list and its rules). That's how a process with
// more pages than its limit keeps running in a fixed amount of memory.
//
// A page taken out keeps its frame, the physical page holding it. Its
// PTE takes the *transition* form (the valid bit clear, the frame's
// number kept), and the frame goes to the tail of one of two lists: the
// free page list if its contents are safe elsewhere (a page of a file,
// unchanged since it was read), the modified page list if they must be
// written first (a page that was written, or a demand-zero or
// copy-on-reference page, which has nowhere else to come from: such
// pages are "effectively modified" from their first fault). If the
// process touches the page again before the frame is reused, the fault
// takes the frame back off its list with no I/O: a *soft fault*. A new
// frame is always taken from the free list's head, so a frame waits on
// the list as long as it can; when it is reused, its old page's PTE goes
// back to the form that says where its contents are (BreakTie, below),
// and the next touch reads it back (a *hard fault*).
//
// With paging off (the default), none of this happens: no process has a
// working set, every page stays valid until it's deleted, and these
// functions do nothing.
//
// # What isn't here yet
//
// Global section pages enter the working set (counted as global pages)
// but are never chosen to leave it: they stay resident until Phase 65
// gives the global page table its own states. There's no page file
// (Phase 64), and no writer for a private file section's modified pages
// (subtask 7), so a page on the modified list leaves it only by a soft
// fault. The working set's limit is fixed at the process's WSLIMIT when
// the list is first made; $ADJWSL and image exit move it from subtask 4.
// And a page that's deleted, or whose process is, while it's in
// transition keeps its frame on its list until subtask 5 frees it.

// WorkingSet returns the process's working set list, made at its first
// use from the process's limit and bounds, or nil with paging off.
func (env *Environment) WorkingSet() *paging.WorkingSet {
	db := env.mem.Paging()
	if db == nil || env.Process == nil {
		return nil
	}

	if env.ws == nil || env.wsDB != db {
		p := env.Process
		env.ws = paging.NewWorkingSet(int(p.WSLimit), paging.Bounds{
			Quota:   int(p.WSQuota),
			Extent:  int(p.WSExtent),
			Minimum: int(p.MinWSCount),
		})
		env.wsDB = db
	}

	return env.ws
}

// pageSpace is the address space the process's P0 and P1 pages are in:
// its own, or, before it has one (before VMINIT), the CPU's.
func (env *Environment) pageSpace() vm.AddressSpace {
	if env.Space != nil {
		return env.Space.AddressSpace
	}

	return vm.CurrentAddressSpace(env.cpu)
}

// pagingProcess returns the process whose working set a fault's page
// enters: with paging on, a P0 or P1 page's process; nil for an S0 page,
// with paging off, or when the address space is no known process's.
func (sys *System) pagingProcess(f vm.PageFault) *Environment {
	if f.Addr >= s0Base || sys.mem.Paging() == nil {
		return nil
	}

	return sys.processForSpace(f.Space)
}

// enterWorkingSet puts the page at va in the process's working set, if
// it isn't there: the pager does this for each page it makes valid (and
// a global section's mapping, for each page it maps valid). If the list
// has no room, the page its rules choose leaves it (evictPage).
func (env *Environment) enterWorkingSet(va uint32, global bool) {
	ws := env.WorkingSet()
	if ws == nil {
		return
	}

	if _, ok := ws.Lookup(va); ok {
		return
	}

	removed, err := ws.Add(va, global, env.replacementConditions())
	if err != nil {
		return
	}

	for _, p := range removed {
		env.evictPage(p)
	}

	// With every page it holds in use by the instruction running (a Go
	// service, or the image activator, touching many pages in one call),
	// the list grows past its limit; it comes back down as soon as the
	// instruction is over (the phase doc's finding 10).
	if ws.Over() {
		env.mem.AtNextInstruction(env.System.trimWorkingSets)
	}
}

// trimWorkingSets brings each process's working set that grew past its
// limit back down to it, at an instruction boundary (enterWorkingSet),
// when no page counts as in use but the instruction-fetch window's and
// the STC's. The pages taken out go into transition as any others.
func (sys *System) trimWorkingSets() {
	db := sys.mem.Paging()

	for _, env := range sys.Processes() {
		if env.ws == nil || env.wsDB != db || !env.ws.Over() {
			continue
		}

		for _, p := range env.ws.Trim(env.replacementConditions()) {
			env.evictPage(p)
		}
	}
}

// leaveWorkingSet takes the page at va out of the process's working set,
// if it's there: its PTE is leaving it (releasePTE), so the page is
// going, not being paged out.
func (env *Environment) leaveWorkingSet(va uint32) {
	if ws := env.WorkingSet(); ws != nil {
		ws.Remove(va)
	}
}

// replacementConditions is what the working set's choice of a page to
// take out depends on outside the list (paging.Conditions): the free
// list's length and GROWLIM; for the process the CPU is running,
// TBSKIPWSL and the translation buffer (which holds no other process's
// pages); and the pages that mustn't leave now (wsProtected).
func (env *Environment) replacementConditions() paging.Conditions {
	as := env.pageSpace()

	c := paging.Conditions{
		FreePages: env.mem.Paging().FreeCount(),
		GrowLimit: int(env.Sysgen.GrowLimit),
		Protected: func(va uint32) bool { return env.wsProtected(as, va) },
	}

	if vm.CurrentAddressSpace(env.cpu) == as {
		c.TBSkip = int(env.Sysgen.TBSkipWSL)
		c.ValidInTB = env.mem.ValidInTB
	}

	return c
}

// wsProtected reports whether the page at va, in address space as, must
// stay in the working set for now: a global section's page (resident
// until Phase 65), or a page whose frame has been translated during the
// current instruction (the phase doc's Decision 6), which the
// instruction, or the Go code running inside it, may still be using. A
// page whose PTE can't be read is kept too: nothing could be done with
// it.
func (env *Environment) wsProtected(as vm.AddressSpace, va uint32) bool {
	if p, ok := env.ws.Lookup(va); ok && p.Global {
		return true
	}

	pte, err := env.mem.LookupPTEIn(env.cpu, as, va)
	if err != nil {
		return true
	}

	return pte.Valid() && env.mem.TranslatedThisInstruction(pte.PFN())
}

// evictPage finishes taking page p out of the working set: the page's
// PTE becomes a transition PTE, which keeps its frame number, and the
// frame goes to the tail of the modified list or the free list
// (paging.Database.Release), by its saved modify bit with the PTE's
// modify bit ORed in. Storing the PTE also empties the page's TB slot,
// whose cached modify bit and frame would otherwise outlive it.
//
// The frame must be the page's own: active, tied to this process's page,
// and held by nothing else. Anything else (which the pager never makes)
// leaves the PTE as it is.
func (env *Environment) evictPage(p paging.Page) {
	db := env.mem.Paging()
	as := env.pageSpace()

	pte, err := env.mem.LookupPTEIn(env.cpu, as, p.VA)
	if err != nil || !pte.Valid() {
		return
	}

	pfn := paging.PFN(pte.PFN())

	e, err := db.Entry(pfn)
	if err != nil || e.State != paging.Active || !e.Tied || e.Type != paging.ProcessPage ||
		e.Owner != (paging.Owner{Handle: env.Process.PID, VA: p.VA}) || e.RefCount != 1 {
		return
	}

	if env.mem.StorePTEIn(env.cpu, as, p.VA, vm.TransitionPTE(pte.PFN(), pte.Protection(), pte.Owner())) != nil {
		return
	}

	_, _ = db.Release(pfn, pte.Modified())
}

// softFault resolves a fault on a page in transition, frame pfn: the
// frame is taken off its list (paging.Database.Reclaim) and the page's
// PTE made valid again, pointing at it, with no I/O. The frame must still
// be tied to this page; if it isn't, the PTE is stale and the fault
// fails.
func (sys *System) softFault(f vm.PageFault, pfn uint32) (vm.PTE, bool) {
	db := sys.mem.Paging()
	claim := sys.faultClaim(f, paging.Backing{})

	e, err := db.Entry(paging.PFN(pfn))
	if err != nil || !claim.Tied || !e.Tied || e.Type != paging.ProcessPage || e.Owner != claim.Owner {
		return 0, false
	}

	if db.Reclaim(paging.PFN(pfn)) != nil {
		return 0, false
	}

	return vm.ValidPTE(pfn, f.PTE.Protection(), f.PTE.Owner()), true
}

// effectivelyModified sets the saved modify bit of frame pfn, which a
// demand-zero or copy-on-reference page was just given, as VMS does at
// such a page's first fault: its contents exist nowhere else, so it must
// go to the modified list, never the free list, whenever it leaves the
// working set (to be written to the page file, from Phase 64). An S0
// page's frame, tied to nothing, is left alone.
func (sys *System) effectivelyModified(pfn uint32, claim vm.FrameClaim) {
	if db := sys.mem.Paging(); db != nil && claim.Tied {
		_ = db.SetModified(paging.PFN(pfn))
	}
}

// BreakTie is the PFN database's TieBreaker (paging.TieBreaker): when a
// frame on the free list is reused, the page it still held, in
// transition, loses it, and its PTE goes back to the form that says where
// the page's contents are (backingPTE), so the page's next touch reads it
// in again. The PTE is changed only if it's still in transition in that
// frame. Global pages never leave a working set in this phase, so only a
// process page's tie is broken.
func (sys *System) BreakTie(pfn paging.PFN, typ paging.Type, owner paging.Owner, back paging.Backing) {
	if typ != paging.ProcessPage {
		return
	}

	env, ok := sys.FindProcess(owner.Handle)
	if !ok {
		return
	}

	as := env.pageSpace()

	pte, err := sys.mem.LookupPTEIn(sys.cpu, as, owner.VA)
	if err != nil {
		return
	}

	if frame, ok := pte.Transition(); !ok || frame != uint32(pfn) {
		return
	}

	_ = sys.mem.StorePTEIn(sys.cpu, as, owner.VA, backingPTE(back, pte.Protection(), pte.Owner()))
}

// backingPTE is the invalid PTE, with protection prot and owner mode
// owner, of a page whose contents are at back: a process section table
// index for a section file's page, a global page table index for a
// global section's, a page file block for a page in the page file, and
// for a page with no backing store the demand-zero form (whose next
// touch gives it a page of zeros, as before it was first touched).
func backingPTE(back paging.Backing, prot vm.Protection, owner uint8) vm.PTE {
	switch back.Kind {
	case paging.SectionFile:
		return vm.SectionPTE(back.Index, prot, owner)
	case paging.GlobalSection:
		return vm.GlobalPTE(back.Index, prot, owner)
	case paging.PageFile:
		return vm.PageFilePTE(back.Block, prot, owner)
	case paging.NoBacking:
	}

	var pte vm.PTE

	pte.SetProtection(prot)
	pte.SetOwner(owner)

	return pte
}

// faultIsGlobal reports whether a fault on a page whose PTE is pte maps
// the page to a global section's own frame (counted in the working set
// as a global page): a global page table index, not of a
// copy-on-reference section, whose fault gives the process a private
// copy.
func (sys *System) faultIsGlobal(pte vm.PTE) bool {
	gi, ok := pte.GlobalIndex()
	if !ok {
		return false
	}

	s, _, ok := sys.Sections.globalPage(gi)

	return ok && !s.CopyOnRef
}
