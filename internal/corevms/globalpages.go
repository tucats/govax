package corevms

import (
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// Global pages under the pager (docs/PHASE-65 - sections and paging.md,
// subtask 3), with paging on.
//
// # A global page's states
//
// A page of a global file section is in one of three places, which its
// global page table entry says (the book's 14.3.3; govax keeps the entry
// in Go, GlobalSections.pageTable, and the section's Frames slot for the
// page holds its frame):
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
//     file. The Frames slot is 0.
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
// the file.
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
// # What isn't here yet
//
// A page-file section's pages (SEC$M_PAGFIL) are still allocated when
// it's created and valid in each mapping process from the start: no share
// counts, never chosen to leave a working set (wsProtected), until
// subtask 4 makes them global demand-zero pages. With paging off, none of
// this happens: a global page, once read, keeps its frame until its
// section is deleted, and the section's Dirty marks gather the modify
// bits.

// globalFrame returns the frame page i of global section s holds, valid
// or in transition, with its PFN database entry, if it's tied to that
// page; false with paging off, for a page that's out, and for a
// page-file section's frame (which has no share counts yet).
func (sys *System) globalFrame(s *GlobalSection, i int) (paging.PFN, paging.Entry, bool) {
	db := sys.mem.Paging()
	if db == nil || s.File == nil || i < 0 || i >= len(s.Frames) || s.Frames[i] == 0 {
		return 0, paging.Entry{}, false
	}

	pfn := paging.PFN(s.Frames[i])

	e, err := db.Entry(pfn)
	if err != nil || !e.Tied || e.Type != paging.GlobalPage || e.Owner != globalClaim(s, i).Owner {
		return 0, paging.Entry{}, false
	}

	return pfn, e, true
}

// pageInSharedGlobal resolves a fault on page i of global file section s
// with paging on: a page valid in some working set gets one more share
// (a global valid fault); a page in transition is taken back off its list
// (a soft fault from the free or modified list) and given its first
// share; a page that's out is read from the file into a new frame, which
// the faulting process's share holds. It returns the frame, the fault's
// kind, and whether it worked.
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

	pfn, ok := sys.readPage(s.File, s.VBN+uint32(i), globalClaim(s, i))
	if !ok {
		return 0, faultRead, false
	}

	if db.ShareFirst(paging.PFN(pfn)) != nil {
		sys.mem.FreePage(pfn)

		return 0, faultRead, false
	}

	s.Frames[i] = pfn
	sys.Sections.byFrame[pfn] = sectionPage{s, i}

	return pfn, faultRead, true
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

	if state, err := env.mem.Paging().Unshare(paging.PFN(pte.PFN()), pte.Modified()); err == nil && state == paging.Modified {
		env.wakeModifiedPageWriter()
	}

	return true
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

	state, err := sys.mem.Paging().Unshare(paging.PFN(pfn), modified)

	return true, err == nil && state == paging.Modified
}

// loseGlobalFrame is BreakTie for a global page's frame being reused off
// the free list: the global page table entry the frame's backing store
// names loses it, and the page is out, in its block of the file. No
// process PTE changes: each one mapping the page holds its GPTX.
func (sys *System) loseGlobalFrame(pfn paging.PFN, owner paging.Owner, back paging.Backing) {
	if back.Kind != paging.GlobalSection {
		return
	}

	s, i, ok := sys.Sections.globalPage(back.Index)
	if !ok || s.serial != owner.Handle || s.Frames[i] != uint32(pfn) {
		return
	}

	s.Frames[i] = 0
	delete(sys.Sections.byFrame, uint32(pfn))
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
// page's global page table entry. A page valid in another process's
// working set, or on the modified list, is written (the latter then moved
// to the free list); one on the free list matches its block already, and
// one that's out has nothing in memory to write (the phase doc's finding
// 9). It reports whether it wrote the page, and the write's error.
func (sys *System) updateGlobalPage(s *GlobalSection, i int) (bool, error) {
	pfn, e, ok := sys.globalFrame(s, i)
	if !ok || e.State == paging.Free {
		return false, nil
	}

	if err := sys.writePage(s.File, s.VBN+uint32(i), uint32(pfn)); err != nil {
		return false, err
	}

	if e.State == paging.Modified {
		_ = sys.mem.Paging().Cleaned(pfn)
	} else {
		_ = sys.mem.Paging().ClearModified(pfn)
	}

	return true, nil
}
