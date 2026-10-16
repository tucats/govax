package corevms

import (
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// The modified page writer (docs/PHASE-63 - working sets.md, subtask 7;
// docs/PHASE-62 - VMS paging.md, Decision 7).
//
// # What it does
//
// A page that leaves a working set after being written can't simply be
// forgotten: its contents exist only in its frame. So its frame goes to
// the modified page list (workingset.go's evictPage), where it waits to
// be written to its backing store. On VMS a system process, the modified
// page writer, does the writing: it wakes when the list holds more than
// MPW_HILIMIT pages and writes until it holds MPW_LOLIMIT (the book's
// 15.5.2 and 17.2). Each frame written moves to the tail of the free list, still
// holding the page, so a touch before the frame is reused is still a
// soft fault; when the frame is reused, the page's PTE goes back to the
// form that says where it was written (BreakTie), and the next touch
// reads it from there.
//
// govax's writer is synchronous (Decision 7): it runs inside the
// operation that took the list over MPW_HILIMIT, and its writes complete
// at once. It takes the frames head first, those that have waited
// longest.
//
// # What it can write
//
// In this phase the only backing store a modified page can be written to
// is its own file: a page of a writable private section mapped from a
// file ($CRMPSC with SEC$M_WRT, without SEC$M_CRF), whose block is the
// frame's backing store address. Every other modified page, a
// demand-zero page or a copy-on-reference copy, belongs in the page file,
// which comes with Phase 64; until then the writer passes it over, and it
// stays on the list until it's faulted back or deleted.
//
// Passing over those pages costs a walk of the list, so the writer
// remembers how many it couldn't write (System.mpwStuck) and wakes again
// only when the list is over MPW_HILIMIT by more than that: the pages it
// can't write don't count toward the threshold. The count is forgotten
// once the list is back down to MPW_HILIMIT. (govax's own rule: VMS
// always has a page file to write them to.)
//
// # Writing from inside a fault
//
// The writer can run in the middle of anything that faults, including an
// RMS operation copying into a user buffer. Its writes go straight
// through the section file's block I/O (rms.ACPFile.WritePage), as
// releasePTE's write-back does, never through RMS's record path; ods2
// keeps no locks for a write to wait on.

// wakeModifiedPageWriter runs the modified page writer if the modified
// list is over MPW_HILIMIT by more than the pages it couldn't write the
// last time it ran. evictPage calls it after each frame it sends to the
// modified list.
func (sys *System) wakeModifiedPageWriter() {
	db := sys.mem.Paging()
	if db == nil {
		return
	}

	n, hi := db.ModifiedCount(), int(sys.Sysgen.MPWHiLimit)
	if n <= hi {
		sys.mpwStuck = 0

		return
	}

	if n > hi+sys.mpwStuck {
		sys.mpwStuck = sys.writeModifiedPages(int(sys.Sysgen.MPWLoLimit))
	}
}

// writeModifiedPages is the writer's run: it writes the frames on the
// modified list, head first, until the list holds no more than goal, and
// moves each one written to the free list (paging.Database.Cleaned). It
// returns how many frames it passed over because it can't write them
// (writableSectionPage).
func (sys *System) writeModifiedPages(goal int) int {
	db := sys.mem.Paging()
	stuck := 0

	for _, pfn := range db.ModifiedFrames() {
		if db.ModifiedCount() <= goal {
			break
		}

		if !sys.writeModifiedFrame(pfn) {
			stuck++
		}
	}

	return stuck
}

// writeModifiedFrame writes frame pfn, on the modified list, to its
// section file and moves it to the free list, reporting whether it could.
func (sys *System) writeModifiedFrame(pfn paging.PFN) bool {
	db := sys.mem.Paging()

	e, err := db.Entry(pfn)
	if err != nil || e.State != paging.Modified {
		return false
	}

	ps, ok := sys.writableSectionPage(e)
	if !ok || sys.writePage(ps.file, e.Backing.Block, uint32(pfn)) != nil {
		return false
	}

	sys.PageCounts.Written++

	return db.Cleaned(pfn) == nil
}

// writableSectionPage returns the private section a frame's page belongs
// to, if the frame's contents go back to that section's file: a tied
// process page whose backing store is a block of a section that writes
// back, which its process still maps at the page's address.
func (sys *System) writableSectionPage(e paging.Entry) (*processSection, bool) {
	if !e.Tied || e.Type != paging.ProcessPage || e.Backing.Kind != paging.SectionFile {
		return nil, false
	}

	env, ok := sys.FindProcess(e.Owner.Handle)
	if !ok || e.Backing.Index >= uint32(len(env.procSections)) {
		return nil, false
	}

	ps := env.procSections[e.Backing.Index]
	if ps == nil || !ps.writeBack || e.Owner.VA < ps.first {
		return nil, false
	}

	i := (e.Owner.VA - ps.first) / pageSize
	if i >= uint32(len(ps.present)) || !ps.present[i] || ps.vbn+i != e.Backing.Block {
		return nil, false
	}

	return ps, true
}

// modifiedTransitionPage returns the private section the page at addr,
// whose PTE is pte, belongs to, if the page is out of the working set
// (in transition, with paging on), its frame waits on the modified list,
// and the section writes back: a page $UPDSEC writes from the list. nil
// for any other page.
func (env *Environment) modifiedTransitionPage(addr uint32, pte vm.PTE) *processSection {
	if _, ok := pte.Transition(); !ok || env.mem.Paging() == nil || env.Process == nil {
		return nil
	}

	if _, modified, ok := env.heldFrame(env.Process.PID, addr, pte); !ok || !modified {
		return nil
	}

	if ps, _, _ := env.processSectionAt(addr); ps != nil && ps.writeBack {
		return ps
	}

	return nil
}

// savedModify reports whether frame pfn, tied to the process's page at
// addr, has its saved modify bit set; false with paging off, or for a
// frame tied to anything else.
func (env *Environment) savedModify(addr, pfn uint32) bool {
	if env.Process == nil {
		return false
	}

	e, ok := env.tiedEntry(env.Process.PID, addr, pfn)

	return ok && e.SavedModify
}
