package corevms

import (
	"slices"

	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// The modified page writer (docs/PHASE-63 - working sets.md, subtask 7;
// docs/PHASE-64 - page file.md, subtask 3; docs/PHASE-62 - VMS
// paging.md, Decision 7).
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
// longest. With its writes done at once, a run may leave the list a
// little below MPW_LOLIMIT, by up to one cluster.
//
// # Where it writes
//
// A modified page goes back to one of two places:
//
//   - a page of a writable private section mapped from a file ($CRMPSC
//     with SEC$M_WRT, without SEC$M_CRF) goes to its own block of the
//     file, the frame's backing store address;
//   - every other page, a demand-zero page or a private copy of a
//     copy-on-reference page, goes to the page file (pagefile.go), to a
//     block the writer allocates when it writes the page. The block
//     becomes the frame's backing store address, and the PTE's when the
//     frame is reused (the page file form), so the page's next fault
//     reads it back (filesec.go's pageInFromPageFile).
//
// # Clusters
//
// The writer writes pages in clusters, one write each, of up to
// MPW_WRTCLUSTER pages (the book's 15.5.2.2 and 15.5.2.3). A cluster
// starts with the list's first frame, then takes its neighbors in the
// process's page table, first toward lower addresses, then higher, for
// as long as they're pages in transition whose frames wait on the
// modified list bound for the same place (a run of virtually contiguous
// pages, a "minicluster"). Lower first, because a program is likelier to
// touch the higher addresses next. A section's cluster is that one run:
// its pages, of one section, are contiguous in the file too. For the
// page file, the writer first allocates a run of blocks for the cluster;
// when one minicluster ends before the run is full, the next page on the
// list bound for the page file, of any process, starts another. Blocks
// the cluster doesn't fill are given back.
//
// # Opening the page file
//
// The page file is opened the first time the writer needs it, but not
// from inside a fault (pagefile.go): the writer, woken in the middle of
// one, asks for the next instruction boundary (atInstructionBoundary),
// which opens the file and runs it again. Until then, and with no page
// file at all, a page bound for it can't be written.
//
// # Pages it can't write
//
// A page bound for the page file when there's none, or when it's full,
// stays on the modified list until it's faulted back or deleted.
// Passing over such pages costs a walk of the list, so the writer
// remembers how many it couldn't write (System.mpwStuck) and wakes again
// only when the list is over MPW_HILIMIT by more than that: the pages it
// can't write don't count toward the threshold. The count is forgotten
// once the list is back down to MPW_HILIMIT, and when the page file is
// opened. (govax's own rule: VMS's writer waits for page file space.)
//
// # Writing from inside a fault
//
// The writer can run in the middle of anything that faults, including an
// RMS operation copying into a user buffer. Its writes go straight
// through a file's block I/O (rms.ACPFile.WritePage, or the page file's,
// which never extends the file), as releasePTE's write-back does, never
// through RMS's record path; ods2 keeps no locks for a write to wait on
// (docs/PHASE-64 - page file.md, "The ods2 audit").

// wakeModifiedPageWriter runs the modified page writer if the modified
// list is over MPW_HILIMIT by more than the pages it couldn't write the
// last time it ran. evictPage calls it after each frame it sends to the
// modified list. If the page file is wanted but isn't open yet, it asks
// for the next instruction boundary to open it.
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

	if sys.pageFileWanted() {
		sys.mem.AtNextInstruction(sys.atInstructionBoundary)
	}

	if n > hi+sys.mpwStuck {
		sys.mpwStuck = sys.writeModifiedPages(int(sys.Sysgen.MPWLoLimit))
	}
}

// pageFileWanted reports whether the page file is configured but hasn't
// been opened yet (pagefile.go).
func (sys *System) pageFileWanted() bool {
	return sys.pageFileState == pageFileUnopened && sys.Sysgen.Paging && sys.Sysgen.PageFileName != ""
}

// openWantedPageFile opens the page file if it's wanted and the modified
// list holds more than limit pages, so that a writer is due to write
// some: at an instruction boundary (pagefile.go). Once it's open, it
// forgets the pages the writer and the reclaimer couldn't write without
// it and runs the writer again.
func (sys *System) openWantedPageFile(limit uint32) {
	db := sys.mem.Paging()
	if db == nil || !sys.pageFileWanted() || db.ModifiedCount() <= int(limit) {
		return
	}

	if sys.OpenPageFile() == nil {
		return
	}

	sys.mpwStuck, sys.reclaimStuck = 0, 0
	sys.wakeModifiedPageWriter()
}

// writeModifiedPages is the writer's run: it writes the frames on the
// modified list, head first, in clusters, until the list holds no more
// than goal, and moves each one written to the free list
// (paging.Database.Cleaned). It returns how many frames it passed over
// because it can't write them.
func (sys *System) writeModifiedPages(goal int) int {
	db := sys.mem.Paging()
	frames := db.ModifiedFrames()
	stuck := 0
	pageFileFull := false

	for i, pfn := range frames {
		if db.ModifiedCount() <= goal {
			break
		}

		// A frame an earlier cluster took is no longer on the list.
		e, err := db.Entry(pfn)
		if err != nil || e.State != paging.Modified {
			continue
		}

		written := false

		switch ps, ok := sys.writableSectionPage(e); {
		case ok:
			written = sys.writeSectionCluster(pfn, e, ps)
		case sys.pageFileBound(e) && !pageFileFull:
			written = sys.writePageFileCluster(frames[i:])
			pageFileFull = !written
		}

		if !written {
			stuck++
		}
	}

	return stuck
}

// writeSectionCluster writes frame pfn, whose entry is e, and its
// neighbors bound for the same private section ps, to the section's file
// as one cluster, moving each to the free list. It reports whether it
// wrote pfn.
func (sys *System) writeSectionCluster(pfn paging.PFN, e paging.Entry, ps *processSection) bool {
	same := func(n paging.Entry) bool {
		other, ok := sys.writableSectionPage(n)

		return ok && other == ps
	}

	cluster := sys.miniCluster(pfn, e, sys.writeClusterSize(), same, nil)
	db := sys.mem.Paging()
	written := 0

	for _, c := range cluster {
		n, _ := db.Entry(c)
		if sys.writePage(ps.file, n.Backing.Block, uint32(c)) != nil {
			break
		}

		_ = db.Cleaned(c)
		written++
	}

	sys.countWrite(written)

	return written > indexOf(cluster, pfn)
}

// indexOf is the place of pfn in cluster, or len(cluster) if it isn't
// there.
func indexOf(cluster []paging.PFN, pfn paging.PFN) int {
	if i := slices.Index(cluster, pfn); i >= 0 {
		return i
	}

	return len(cluster)
}

// writePageFileCluster writes a cluster to the page file, starting with
// frames[0], the first frame on the modified list bound for it: it
// allocates a run of blocks, fills it with miniclusters around that
// frame and the frames after it on the list bound for the page file,
// writes each page to its block, and moves each to the free list with
// the block as its backing store. Blocks left over are given back. It
// reports false, writing nothing, when there's no page file or no block
// free.
func (sys *System) writePageFileCluster(frames []paging.PFN) bool {
	pf := sys.PageFile()
	if pf == nil {
		return false
	}

	db := sys.mem.Paging()

	// Ask for no more blocks than there are pages to fill them.
	want := uint32(0)

	for _, pfn := range frames {
		if want == sys.writeClusterSize() {
			break
		}

		if e, err := db.Entry(pfn); err == nil && e.State == paging.Modified && sys.pageFileBound(e) {
			want++
		}
	}

	run, ok := sys.AllocatePageFile(want)
	if !ok {
		return false
	}

	taken := map[paging.PFN]bool{}

	var cluster []paging.PFN

	for _, pfn := range frames {
		room := run.Count - uint32(len(cluster))
		if room == 0 {
			break
		}

		e, err := db.Entry(pfn)
		if err != nil || e.State != paging.Modified || taken[pfn] || !sys.pageFileBound(e) {
			continue
		}

		for _, c := range sys.miniCluster(pfn, e, room, sys.pageFileBound, taken) {
			taken[c] = true
			cluster = append(cluster, c)
		}
	}

	written := uint32(0)
	buf := make([]byte, pageSize)

	for _, c := range cluster {
		vbn := run.First + written
		if sys.mem.LoadPhysical(uint32(c)*pageSize, buf) != nil || pf.Write(vbn, buf) != nil {
			break
		}

		// A page that still has a block from an earlier write (which
		// evictPage normally frees) gives it up for its new one.
		if e, _ := db.Entry(c); e.Backing.Block != 0 {
			_ = sys.FreePageFile(paging.Run{First: e.Backing.Block, Count: 1})
		}

		_ = db.SetBacking(c, pageFileBacking(vbn))
		_ = db.Cleaned(c)
		written++
	}

	if written < run.Count {
		_ = sys.FreePageFile(paging.Run{First: run.First + written, Count: run.Count - written})
	}

	sys.countWrite(int(written))

	return written > 0
}

// pageFileBound reports whether a frame's contents go to the page file
// when the writer writes it: a tied process page whose backing store is
// the page file (or, for a frame claimed before demand-zero pages had
// one, nothing), of a process that still exists.
func (sys *System) pageFileBound(e paging.Entry) bool {
	if !e.Tied || e.Type != paging.ProcessPage {
		return false
	}

	if e.Backing.Kind != paging.PageFile && e.Backing.Kind != paging.NoBacking {
		return false
	}

	_, ok := sys.FindProcess(e.Owner.Handle)

	return ok
}

// miniCluster returns frame pfn, whose entry is e, with its neighbors in
// its process's page table: first the pages below it, then those above,
// for as long as each is in transition in a frame on the modified list
// tied to it, which match accepts and taken doesn't hold, up to size
// frames in all. They're in the order of their addresses, lowest first.
// A page never crosses from P0 to P1.
func (sys *System) miniCluster(pfn paging.PFN, e paging.Entry, size uint32, match func(paging.Entry) bool,
	taken map[paging.PFN]bool) []paging.PFN {
	env, ok := sys.FindProcess(e.Owner.Handle)
	if !ok || size <= 1 {
		return []paging.PFN{pfn}
	}

	as := env.pageSpace()
	low, high := uint32(0), uint32(p1Base)

	if e.Owner.VA >= p1Base {
		low, high = p1Base, s0Base
	}

	neighbor := func(va uint32) (paging.PFN, bool) {
		pte, err := sys.mem.LookupPTEIn(sys.cpu, as, va)
		if err != nil {
			return 0, false
		}

		frame, ok := pte.Transition()
		if !ok || taken[paging.PFN(frame)] {
			return 0, false
		}

		n, err := sys.mem.Paging().Entry(paging.PFN(frame))
		if err != nil || n.State != paging.Modified || !n.Tied || n.Type != paging.ProcessPage ||
			n.Owner != (paging.Owner{Handle: e.Owner.Handle, VA: va}) || !match(n) {
			return 0, false
		}

		return paging.PFN(frame), true
	}

	var below []paging.PFN

	for va := e.Owner.VA; uint32(len(below))+1 < size && va > low; {
		va -= pageSize

		frame, ok := neighbor(va)
		if !ok {
			break
		}

		below = append(below, frame)
	}

	slices.Reverse(below)
	cluster := append(below, pfn)

	for va := e.Owner.VA + pageSize; uint32(len(cluster)) < size && va < high; va += pageSize {
		frame, ok := neighbor(va)
		if !ok {
			break
		}

		cluster = append(cluster, frame)
	}

	return cluster
}

// writeClusterSize is MPW_WRTCLUSTER, or its default if it's unset.
func (sys *System) writeClusterSize() uint32 {
	if n := sys.Sysgen.MPWWriteCluster; n != 0 {
		return n
	}

	return DefaultMPWWriteCluster
}

// countWrite counts one of the writer's writes, of pages pages, in the
// system's counts; a write of no pages isn't one.
func (sys *System) countWrite(pages int) {
	if pages > 0 {
		sys.PageCounts.Written += uint64(pages)
		sys.PageCounts.WriteIOs++
	}
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
