package corevms

import (
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// Global pages under the pager (docs/PHASE-65 - sections and paging.md,
// subtasks 3 and 4), with paging on.
//
// # A global page's states
//
// A page of a global file section, or of a page-file section, is in one
// of three places, which its global page table entry says (the book's
// 14.3.3; govax keeps the entry in Go, GlobalSections.pageTable, and the
// section's Frames slot for the page holds its frame):
//
//   - **valid**: the page is in one or more processes' working sets. Its
//     frame is active, and its share count (the PFN database's, VMS's
//     SHRCNT) is how many process PTEs are valid on it right now. That's
//     a different count from the section's Refs, how many process PTEs
//     map the section at all, valid or not (its lifetime).
//   - **in transition**: no working set holds it, but its frame still
//     does, on the free list or the modified list, tied to the global
//     page (not to any process's page).
//   - **out**: no frame; its contents are in its block of the section
//     file, or for a page-file section in a page file block (the
//     section's blocks slot for the page), or nowhere yet: a page-file
//     section's page never written is a global demand-zero page. The
//     Frames slot is 0.
//
// # Process PTEs
//
// A process PTE for a global page holds the global page table index
// (GPTX) until the process faults the page, and then the frame, valid.
// When the page leaves the process's working set (evictPage), the PTE
// gets its GPTX back, never a transition form (the book's 15.3.1, steps
// 6 and 7), and the process's modify bit is gathered into the frame's
// saved modify bit as its share goes (paging.Database.Unshare). The last
// share's going sends the frame to the modified list if any process
// wrote the page, to the free list if not.
//
// So every fault on a global page comes through its GPTX
// (pageInSharedGlobal): a page valid elsewhere is a soft "global valid"
// fault that adds a share; one in transition is a soft fault that takes
// the frame back off its list; one out is a hard fault that reads it from
// the file or the page file, or a demand-zero fault that gives it a frame
// of zeros.
//
// # Page-file sections
//
// A page-file section (SEC$M_PAGFIL) made with paging on has no frames
// when it's made (subtask 4; the book's 14.3.2's global demand-zero
// page). Its first fault anywhere gives a page a frame of zeros, backed
// by the page file and effectively modified, as a process's demand-zero
// page is: whenever its last share goes it waits on the modified list,
// and the writer writes it to the page file (mpw.go) with any of its
// neighbors in the section waiting there too. A reused frame leaves the
// block in the section's blocks slot, and the page's next fault reads it
// back. A permanent section's pages can so leave memory while nothing
// maps it. A page modified again gives its old block up.
//
// # Frames reused and written
//
// When a global page's frame on the free list is reused, the tie is broken
// through the global page table (BreakTie, loseGlobalFrame): the Frames
// slot goes back to 0, and no process PTE changes, since none points at
// the frame. The modified page writer writes a global frame on the
// modified list to its block of the section file, with its neighbors in
// the section on the list as one cluster (writeGlobalCluster), and moves
// it to the free list, still the page's.
//
// # Paging off
//
// With paging off, none of this happens: a global page, once read, keeps
// its frame until its section is deleted, a page-file section's frames
// are allocated when it's made and valid in each mapping process from the
// start, and the section's Dirty marks gather the modify bits.

// globalFrame returns the frame page i of global section s holds, valid
// or in transition, with its PFN database entry, if it's tied to that
// page; false with paging off, and for a page that's out.
func (sys *System) globalFrame(s *GlobalSection, i int) (paging.PFN, paging.Entry, bool) {
	db := sys.mem.Paging()
	if db == nil || !s.hasPageTable() || i < 0 || i >= len(s.Frames) || s.Frames[i] == 0 {
		return 0, paging.Entry{}, false
	}

	pfn := paging.PFN(s.Frames[i])

	e, err := db.Entry(pfn)
	if err != nil || !e.Tied || e.Type != paging.GlobalPage || e.Owner != globalClaim(s, i).Owner {
		return 0, paging.Entry{}, false
	}

	return pfn, e, true
}

// pageInSharedGlobal resolves a fault on page i of global section s with
// paging on: a page valid in some working set gets one more share (a
// global valid fault); a page in transition is taken back off its list
// (a soft fault from the free or modified list) and given its first
// share; a page that's out is read into a new frame (pageInGlobalOut),
// which the faulting process's share holds. It returns the frame, the
// fault's kind, and whether it worked.
func (sys *System) pageInSharedGlobal(s *GlobalSection, i int) (uint32, faultKind, bool) {
	db := sys.mem.Paging()

	if pfn, e, ok := sys.globalFrame(s, i); ok {
		if e.State == paging.Active {
			return uint32(pfn), faultGlobalValid, db.Share(pfn) == nil
		}

		kind := faultFreeList
		if e.State == paging.Modified {
			kind = faultModifiedList
		}

		if db.Reclaim(pfn) != nil || db.ShareFirst(pfn) != nil {
			return 0, kind, false
		}

		if kind == faultFreeList {
			sys.wakeReclaimerIfLow()
		}

		return uint32(pfn), kind, true
	}

	pfn, kind, ok := sys.pageInGlobalOut(s, i)
	if !ok {
		return 0, kind, false
	}

	if db.ShareFirst(paging.PFN(pfn)) != nil {
		sys.mem.FreePage(pfn)

		return 0, kind, false
	}

	s.Frames[i] = pfn
	sys.Sections.byFrame[pfn] = sectionPage{s, i}

	return pfn, kind, true
}

// pageInGlobalOut gives page i of global section s, which is out, a new
// frame tied to it, returning the frame and the fault's kind: a file
// section's page is read from its block of the file; a page-file
// section's from its page file block if the writer wrote it (the block
// then the frame's backing store, and the page clean), or else, never
// written, a frame of zeros, effectively modified (a demand-zero fault).
func (sys *System) pageInGlobalOut(s *GlobalSection, i int) (uint32, faultKind, bool) {
	claim := globalClaim(s, i)

	if s.File != nil {
		pfn, ok := sys.readPage(s.File, s.VBN+uint32(i), claim)

		return pfn, faultRead, ok
	}

	if vbn := s.blocks[i]; vbn != 0 {
		pfn, ok := sys.readPageFileBlock(vbn, claim)
		if ok {
			s.blocks[i] = 0
		}

		return pfn, faultRead, ok
	}

	pfn, ok := sys.mem.AllocateFrame(claim)
	if !ok {
		return 0, faultDemandZero, false
	}

	sys.effectivelyModified(pfn, claim)

	return pfn, faultDemandZero, true
}

// sharedGlobalPage returns the global section page frame pfn holds, if
// it's a page whose frame the pager shares out (a file section's, with
// paging on) and at least one process PTE is valid on it.
func (sys *System) sharedGlobalPage(pfn uint32) (sectionPage, bool) {
	sp, ok := sys.Sections.byFrame[pfn]
	if !ok {
		return sectionPage{}, false
	}

	frame, e, ok := sys.globalFrame(sp.s, sp.page)
	if !ok || frame != paging.PFN(pfn) || e.State != paging.Active || e.ShareCount == 0 {
		return sectionPage{}, false
	}

	return sp, true
}

// evictGlobalPage takes the process's global page at va, whose valid PTE
// is pte, out of its working set: the PTE gets the page's global page
// table index back, and the process's share of the frame goes, its
// modify bit gathered into the frame's saved modify bit. If that was the
// last share, the frame goes to a list (the modified list may wake the
// modified page writer). It reports false, changing nothing, if the
// frame isn't a shared global page's.
func (env *Environment) evictGlobalPage(as vm.AddressSpace, va uint32, pte vm.PTE) bool {
	sp, ok := env.sharedGlobalPage(pte.PFN())
	if !ok {
		return false
	}

	gpte := vm.GlobalPTE(sp.s.gpt+uint32(sp.page), pte.Protection(), pte.Owner())
	if env.mem.StorePTEIn(env.cpu, as, va, gpte) != nil {
		return true
	}

	env.dropStaleBlock(paging.PFN(pte.PFN()), pte.Modified())

	if state, err := env.mem.Paging().Unshare(paging.PFN(pte.PFN()), pte.Modified()); err == nil && state == paging.Modified {
		env.wakeModifiedPageWriter()
	}

	return true
}

// dropStaleBlock gives back the page file block frame pfn's backing store
// names, if modified is set: a page-file section's page read back from
// the page file and written since no longer matches its block, which the
// writer will replace (the book's 15.2.4, as for a process's page).
func (sys *System) dropStaleBlock(pfn paging.PFN, modified bool) {
	if !modified {
		return
	}

	if e, err := sys.mem.Paging().Entry(pfn); err == nil {
		sys.forgetPageFileBlock(pfn, e.Backing)
	}
}

// unshareGlobalFrame drops one process PTE's share of frame pfn, a
// shared global page's, as the PTE goes (releaseFrame), gathering its
// modify bit into the frame's saved modify bit. It reports whether the
// frame was such a page, and whether the frame went to the modified
// list.
func (sys *System) unshareGlobalFrame(pfn uint32, modified bool) (shared, toModified bool) {
	if _, ok := sys.sharedGlobalPage(pfn); !ok {
		return false, false
	}

	sys.dropStaleBlock(paging.PFN(pfn), modified)
	state, err := sys.mem.Paging().Unshare(paging.PFN(pfn), modified)

	return true, err == nil && state == paging.Modified
}

// loseGlobalFrame is BreakTie for a global page's frame being reused off
// the free list: the global page the frame holds loses it, and the page
// is out, in its block of the file, or for a page-file section's page in
// the page file block its backing store names, kept in the section's
// blocks slot (a page-file section's frame reaches the free list only
// once the writer has given it a block: until then it's effectively
// modified). No process PTE changes: each one mapping the page holds its
// GPTX.
func (sys *System) loseGlobalFrame(pfn paging.PFN, owner paging.Owner, back paging.Backing) {
	sp, ok := sys.Sections.byFrame[uint32(pfn)]
	if !ok || sp.s.serial != owner.Handle || sp.s.Frames[sp.page] != uint32(pfn) {
		return
	}

	s, i := sp.s, sp.page

	if s.paged && back.Kind == paging.PageFile {
		s.blocks[i] = back.Block
	}

	s.Frames[i] = 0
	delete(sys.Sections.byFrame, uint32(pfn))
}

// pageFileGlobal reports whether a frame, whose entry is e, is a paged
// page-file section's page whose contents go to the page file when the
// writer writes it.
func (sys *System) pageFileGlobal(e paging.Entry) bool {
	if !e.Tied || e.Type != paging.GlobalPage || e.Backing.Kind != paging.PageFile {
		return false
	}

	s := sys.Sections.bySerial(e.Owner.Handle)

	return s != nil && s.paged
}

// globalMiniCluster is miniCluster for frame pfn, a page-file section's
// page whose entry is e: the frame with its neighbors in the section,
// first the pages below it, then those above, for as long as each holds a
// frame on the modified list which match accepts and taken doesn't hold,
// up to size frames, in page order.
func (sys *System) globalMiniCluster(pfn paging.PFN, e paging.Entry, size uint32, match func(paging.Entry) bool,
	taken map[paging.PFN]bool) []paging.PFN {
	s := sys.Sections.bySerial(e.Owner.Handle)
	i := int(e.Owner.VA / pageSize)

	if s == nil || i >= len(s.Frames) || s.Frames[i] != uint32(pfn) {
		return []paging.PFN{pfn}
	}

	neighbor := func(j int) (paging.PFN, bool) {
		frame, n, ok := sys.globalFrame(s, j)

		return frame, ok && n.State == paging.Modified && !taken[frame] && match(n)
	}

	first, last := i, i
	for first > 0 && uint32(last-first+1) < size {
		if _, ok := neighbor(first - 1); !ok {
			break
		}

		first--
	}

	for last+1 < len(s.Frames) && uint32(last-first+1) < size {
		if _, ok := neighbor(last + 1); !ok {
			break
		}

		last++
	}

	cluster := make([]paging.PFN, 0, last-first+1)
	for j := first; j <= last; j++ {
		cluster = append(cluster, paging.PFN(s.Frames[j]))
	}

	return cluster
}

// globalPageModified reports whether page i of global section s, which
// holds a frame, differs from its block of the file: with paging off, by
// the section's Dirty marks; with it on, by the frame's saved modify bit
// too.
func (sys *System) globalPageModified(s *GlobalSection, i int) bool {
	if s.Dirty != nil && s.Dirty[i] {
		return true
	}

	_, e, ok := sys.globalFrame(s, i)

	return ok && e.SavedModify
}

// writableGlobalFrame returns the global section and page a frame on the
// modified list holds, if its contents go back to the section's file: a
// tied global page of a section that writes back.
func (sys *System) writableGlobalFrame(pfn paging.PFN, e paging.Entry) (*GlobalSection, int, bool) {
	if !e.Tied || e.Type != paging.GlobalPage || e.Backing.Kind != paging.GlobalSection {
		return nil, 0, false
	}

	s, i, ok := sys.Sections.globalPage(e.Backing.Index)
	if !ok || !s.writesBack() || s.serial != e.Owner.Handle || s.Frames[i] != uint32(pfn) {
		return nil, 0, false
	}

	return s, i, true
}

// writeGlobalCluster writes frame pfn, page i of global section s, and
// its neighbors in the section whose frames wait on the modified list
// too, to the section's file as one cluster of up to MPW_WRTCLUSTER
// pages (successive global page table entries of one section, the book's
// 15.5.2.3), moving each to the free list. It looks below the page first,
// then above, as miniCluster does in a process's page table. It reports
// whether it wrote pfn.
func (sys *System) writeGlobalCluster(pfn paging.PFN, s *GlobalSection, i int) bool {
	size := int(sys.writeClusterSize())

	waiting := func(j int) bool {
		_, e, ok := sys.globalFrame(s, j)

		return ok && e.State == paging.Modified
	}

	first, last := i, i
	for first > 0 && last-first+1 < size && waiting(first-1) {
		first--
	}

	for last+1 < len(s.Frames) && last-first+1 < size && waiting(last+1) {
		last++
	}

	db := sys.mem.Paging()
	written := 0

	for j := first; j <= last; j++ {
		if sys.writePage(s.File, s.VBN+uint32(j), s.Frames[j]) != nil {
			break
		}

		_ = db.Cleaned(paging.PFN(s.Frames[j]))
		written++
	}

	sys.countWrite(written)

	return written > i-first
}

// updateGlobalPage is $UPDSEC's work for page i of global section s,
// one the caller's PTE doesn't hold valid, with paging on: it follows the
// page's global page table entry. A page that's out has nothing in memory
// to write. With modifiedOnly clear (UPDFLG 0) every page with a frame is
// written, valid in another process's working set or on either list,
// modified or not: VMS can't tell which global pages were modified, and
// writes them all (16.3.4; VMS 7.1 rewrote a section's clean pages,
// testdata/pg65's U3). With it set (UPDFLG 1, the caller the only
// writer) only a modified page is written: on the modified list, or
// valid with its frame's saved modify bit set (U5: pages the caller
// modified and purged were written). A page written on the modified list
// moves to the free list. It reports whether it wrote the page, and the
// write's error.
func (sys *System) updateGlobalPage(s *GlobalSection, i int, modifiedOnly bool) (bool, error) {
	pfn, e, ok := sys.globalFrame(s, i)
	if !ok {
		return false, nil
	}

	modified := e.State == paging.Modified || e.State == paging.Active && e.SavedModify
	if modifiedOnly && !modified {
		return false, nil
	}

	if err := sys.writePage(s.File, s.VBN+uint32(i), uint32(pfn)); err != nil {
		return false, err
	}

	switch e.State {
	case paging.Modified:
		_ = sys.mem.Paging().Cleaned(pfn)
	case paging.Active:
		_ = sys.mem.Paging().ClearModified(pfn)
	}

	return true, nil
}
