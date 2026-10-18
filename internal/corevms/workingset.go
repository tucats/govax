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
// pages are "effectively modified" from their first fault, and the page
// file is their backing store). If the
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
// # Global pages
//
// A global file section's pages enter the working set as global pages
// (counted in GPGCNT) and leave it as the process's own do, but their
// frames are shared, so a page leaving gives its PTE the global page
// table index back rather than a transition form, and its frame goes to
// a list only when no working set holds it (globalpages.go). A page-file
// section's pages are never chosen to leave yet (wsProtected).
//
// # What isn't here yet
//
// Without a page file
// (pagefile.go), the modified page writer (mpw.go) writes only a
// writable private file section's pages, to their file, so any other
// page on the modified list leaves it only by a soft fault, or by being
// deleted.
//
// # Deleting a page in transition
//
// A page can be deleted while it's out of the working set ($DELTVA,
// $CRETVA over it, image rundown's unmapping of sections, the deletion
// of its process). Its frame is still tied to it, on the free or
// modified list, and must go with it: heldFrame finds it from the PTE,
// and it's deleted as a valid page's frame is, to the head of the free
// list (paging.Database.Delete), where it's reused first. Left there
// tied, a new page at the same address would make two frames with one
// owner.

// WorkingSet returns the process's working set list, made at its first
// use from the process's limit and bounds, or nil with paging off.
func (env *Environment) WorkingSet() *paging.WorkingSet {
	db := env.mem.Paging()
	if db == nil || env.Process == nil {
		return nil
	}

	if env.ws == nil || env.wsDB != db {
		env.ws = paging.NewWorkingSet(int(env.Process.WSLimit), env.workingSetBounds())
		env.wsDB = db
		env.Process.WSLimit = uint32(env.ws.Size())
	}

	return env.ws
}

// workingSetBounds are the limits the process's working set list moves
// within: its WSQUOTA, WSEXTENT, and MINWSCNT.
func (env *Environment) workingSetBounds() paging.Bounds {
	p := env.Process

	return paging.Bounds{
		Quota:   int(p.WSQuota),
		Extent:  int(p.WSExtent),
		Minimum: int(p.MinWSCount),
	}
}

// setWorkingSetLimit makes the process's working-set limit (WSLIMIT)
// limit, kept within [MINWSCNT, WSEXTENT], and returns the limit it
// set. It's $ADJWSL's work and image exit's reset (resetWorkingSet).
//
// With paging on, the working set list changes size with it
// (paging.WorkingSet.Resize). A smaller list takes its oldest pages over
// the new limit out, into transition as any page leaving the working
// set; the list's own floor is MINWSCNT more than its locked pages, so
// with pages locked the limit may stay above the one asked for. A page
// in use by the instruction running (the CHMK or XFC that called
// $ADJWSL) can't leave yet, so the list may stay over its limit until
// the instruction ends, as when it grows past it (enterWorkingSet).
func (env *Environment) setWorkingSetLimit(limit int64) uint32 {
	p := env.Process
	limit = min(max(limit, int64(p.MinWSCount)), int64(p.WSExtent))
	p.WSLimit = uint32(limit)

	ws := env.WorkingSet()
	if ws == nil {
		return p.WSLimit
	}

	ws.SetBounds(env.workingSetBounds())

	for _, page := range ws.Resize(int(limit), env.replacementConditions()) {
		env.evictPage(page)
	}

	if ws.Over() {
		env.mem.AtNextInstruction(env.System.atInstructionBoundary)
	}

	p.WSLimit = uint32(ws.Size())

	return p.WSLimit
}

// resetWorkingSet is image exit's working-set step (the book's 21.3,
// MMG$IMGRESET): the limit goes back to the process's default (DFWSCNT,
// SET WORKING_SET/LIMIT's value), undoing the image's $ADJWSL calls,
// with paging on or off. With paging on, the pages over the default
// leave the working set, oldest first.
func (env *Environment) resetWorkingSet() {
	if env.Process != nil {
		env.setWorkingSetLimit(int64(env.Process.WSDefault))
	}
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
		env.mem.AtNextInstruction(env.System.atInstructionBoundary)
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
// stay in the working set for now: a global page whose frame isn't
// shared out by the pager (since Phase 65's subtask 4 every global
// section's page is, so this is a safeguard), or a page whose frame has been translated
// during the current instruction (the phase doc's Decision 6), which the
// instruction, or the Go code running inside it, may still be using. A
// page whose PTE can't be read is kept too: nothing could be done with
// it.
func (env *Environment) wsProtected(as vm.AddressSpace, va uint32) bool {
	pte, err := env.mem.LookupPTEIn(env.cpu, as, va)
	if err != nil {
		return true
	}

	if p, ok := env.ws.Lookup(va); ok && p.Global {
		if _, shared := env.sharedGlobalPage(pte.PFN()); !pte.Valid() || !shared {
			return true
		}
	}

	return pte.Valid() && env.mem.TranslatedThisInstruction(pte.PFN())
}

// evictPage finishes taking page p out of the working set: the page's
// PTE becomes a transition PTE, which keeps its frame number, and the
// frame goes to the tail of the modified list or the free list
// (paging.Database.Release), by its saved modify bit with the PTE's
// modify bit ORed in. Storing the PTE also empties the page's TB slot,
// whose cached modify bit and frame would otherwise outlive it. A frame
// sent to the modified list may wake the modified page writer (mpw.go).
//
// The frame must be the page's own: active, tied to this process's page,
// and held by nothing else. Anything else (which the pager never makes)
// leaves the PTE as it is. A global page's frame is shared instead: its
// PTE gets its global page table index back, and the process's share of
// the frame goes (evictGlobalPage).
func (env *Environment) evictPage(p paging.Page) {
	db := env.mem.Paging()
	as := env.pageSpace()

	pte, err := env.mem.LookupPTEIn(env.cpu, as, p.VA)
	if err != nil || !pte.Valid() {
		return
	}

	if p.Global {
		env.evictGlobalPage(as, p.VA, pte)

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

	if state, err := db.Release(pfn, pte.Modified()); err == nil && state == paging.Modified {
		env.forgetPageFileBlock(pfn, e.Backing)
		env.wakeModifiedPageWriter()
	}
}

// forgetPageFileBlock frees the page file block of frame pfn, whose
// backing store was back, as the frame goes to the modified list: a page
// read back from the page file and modified again no longer matches its
// block, so the block is given back, and the writer allocates the page a
// new one (the book's 15.2.1.1, step 6). Freeing it here lets the writer
// gather the page into a cluster with others, in a run of blocks.
func (sys *System) forgetPageFileBlock(pfn paging.PFN, back paging.Backing) {
	if back.Kind != paging.PageFile || back.Block == 0 {
		return
	}

	_ = sys.FreePageFile(paging.Run{First: back.Block, Count: 1})
	_ = sys.mem.Paging().SetBacking(pfn, paging.Backing{Kind: paging.PageFile})
}

// releasePageFileBlock gives back the page file block the page at va of
// process pid holds, by its PTE pte, as the page is deleted (the book's
// 16.2.2.1): a page whose contents are only in the page file holds its
// block in its PTE (the page file form); a page that still has its frame
// (valid, or in transition) holds it as the frame's backing store, when
// the page was written and then faulted back, or read back from the
// page file, and not modified since. A page file page's contents are
// never written at deletion. Nothing happens with paging off, or for a
// page with no block.
func (sys *System) releasePageFileBlock(pid, va uint32, pte vm.PTE) {
	if sys.mem.Paging() == nil {
		return
	}

	if vbn, ok := pte.PageFileBlock(); ok {
		if vbn != 0 {
			_ = sys.FreePageFile(paging.Run{First: vbn, Count: 1})
		}

		return
	}

	pfn, _, ok := sys.heldFrame(pid, va, pte)
	if !ok {
		return
	}

	if e, tied := sys.tiedEntry(pid, va, pfn); tied {
		sys.forgetPageFileBlock(paging.PFN(pfn), e.Backing)
	}
}

// softFault resolves a fault on a page in transition, frame pfn: the
// frame is taken off its list (paging.Database.Reclaim) and the page's
// PTE made valid again, pointing at it, with no I/O. The fault's kind
// says which list the frame was on; a frame off the free list leaves it
// shorter, which may wake the reclaimer (reclaim.go). The frame must still be tied to this
// page; if it isn't, the PTE is stale and the fault fails.
func (sys *System) softFault(f vm.PageFault, pfn uint32) (vm.PTE, faultKind, bool) {
	db := sys.mem.Paging()
	claim := sys.faultClaim(f, paging.Backing{})

	e, err := db.Entry(paging.PFN(pfn))
	if err != nil || !claim.Tied || !e.Tied || e.Type != paging.ProcessPage || e.Owner != claim.Owner {
		return 0, faultFreeList, false
	}

	kind := faultFreeList
	if e.State == paging.Modified {
		kind = faultModifiedList
	}

	if db.Reclaim(paging.PFN(pfn)) != nil {
		return 0, kind, false
	}

	if kind == faultFreeList {
		sys.wakeReclaimerIfLow()
	}

	return vm.ValidPTE(pfn, f.PTE.Protection(), f.PTE.Owner()), kind, true
}

// heldFrame returns the frame the page at va of process pid holds, by
// its PTE pte, for a page being deleted, and whether the frame's
// contents differ from the page's backing store: a valid page's frame,
// modified if its PTE's modify bit or, for a frame tied to the page, its
// saved modify bit is set (a page soft-faulted back after being written
// has a clean PTE and the bit in the database); or, with paging on, a
// page in transition's frame, still on its list and tied to the page,
// modified by its saved modify bit. Any other PTE holds no frame. So
// does a transition PTE whose frame has gone to another page, which
// happens only when the frame was reused after the process left the
// process table (BreakTie then can't find the PTE to change).
func (sys *System) heldFrame(pid, va uint32, pte vm.PTE) (pfn uint32, modified, ok bool) {
	if pte.Valid() {
		e, tied := sys.tiedEntry(pid, va, pte.PFN())

		return pte.PFN(), pte.Modified() || (tied && e.SavedModify), true
	}

	if sys.mem.Paging() == nil {
		return 0, false, false
	}

	frame, ok := pte.Transition()
	if !ok {
		return 0, false, false
	}

	e, tied := sys.tiedEntry(pid, va, frame)
	if !tied || e.State == paging.Active {
		return 0, false, false
	}

	return frame, e.SavedModify, true
}

// tiedEntry returns frame pfn's PFN database entry if it's tied to the
// process page at va of process pid; false with paging off, or for any
// other frame.
func (sys *System) tiedEntry(pid, va, pfn uint32) (paging.Entry, bool) {
	db := sys.mem.Paging()
	if db == nil {
		return paging.Entry{}, false
	}

	e, err := db.Entry(paging.PFN(pfn))
	if err != nil || !e.Tied || e.Type != paging.ProcessPage ||
		e.Owner != (paging.Owner{Handle: pid, VA: va &^ pageMask}) {
		return paging.Entry{}, false
	}

	return e, true
}

// effectivelyModified sets the saved modify bit of frame pfn, which a
// demand-zero or copy-on-reference page was just given, as VMS does at
// such a page's first fault: its contents exist nowhere else, so it must
// go to the modified list, never the free list, whenever it leaves the
// working set, to be written to the page file (mpw.go). An S0
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
// frame. A global page's tie is broken in the global page table instead
// (loseGlobalFrame), where no process PTE points at the frame.
func (sys *System) BreakTie(pfn paging.PFN, typ paging.Type, owner paging.Owner, back paging.Backing) {
	if typ == paging.GlobalPage {
		sys.loseGlobalFrame(pfn, owner, back)

		return
	}

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
