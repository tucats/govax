package corevms

import "github.com/tucats/govax/internal/vm"

// Read clustering (docs/PHASE-65 - sections and paging.md, subtask 6),
// with paging on.
//
// # Why read more than one page
//
// A hard fault waits for a disk read, and most of the cost of a read is
// in starting it, not in how many blocks it moves. Programs tend to touch
// the pages next to the one they just touched. So when the pager has to
// read a page, VMS's reads its neighbors with it, as many as the page's
// *cluster size* allows, in the hope that they'll be wanted soon; each
// neighbor read is put in the working set too, and touching it later
// doesn't fault. *VAX/VMS Internals and Data Structures*, 15.5.1, gives
// the rules, which govax follows:
//
//   - **The scan.** From the faulting page the pager looks at the PTEs at
//     higher addresses first, adding each page that is in the same
//     backing store as the faulting one, until the cluster is full or a
//     page doesn't match, a page table can't be read, the working set
//     list has no available entry, or no frame is free. Only if this
//     forward scan added no page does it scan toward lower addresses, by
//     the same rules.
//   - **What matches** (15.5.1.2): for a page of a private section (its
//     PTE a process section table index), a PTE holding exactly the same
//     index, since every page of a private section holds its section's;
//     for a page in the page file, a PTE holding the next block (the one
//     before, scanning down); for a global page (its PTE a global page
//     table index), the next global page table index (the one before),
//     if it's the same section's and that page is out, not in memory.
//     Only hard faults cluster: a demand-zero fault reads nothing, and a
//     soft fault takes a frame already in memory.
//   - **The size** (15.5.1.3, Table 15-1): a section's own cluster size
//     ($CRMPSC's pfc argument, kept in the private section's entry or the
//     global section) or, when that's 0, PFCDEFAULT, a SYSGEN parameter
//     (vax.sysgen.pfcdefault); the page file's is PFCDEFAULT (its own
//     cluster byte being 0), and so is a page-file section's (the 5.0
//     System Services manual: pfc isn't used for one). An image section's
//     cluster size, LINK's CLUSTER option, comes with image activation by
//     mapping (Phase 66).
//
// # govax's readings
//
// These the book doesn't settle; each is govax's choice, unconfirmed
// until the probe (testdata/pg65) says:
//
//   - The cluster's size counts the faulting page: a cluster of 16 reads
//     the page and up to 15 neighbors. A size of 0 or 1 reads the page
//     alone, and a size above 127, PFCDEFAULT's largest, is 127.
//   - "No more working set list entries" means no available entry the
//     list may fill: the cluster stops rather than take any page out
//     (paging.WorkingSet.AddAvailable), and it stops past the quota while
//     the free list is at GROWLIM or below, as the list's growth would.
//   - A page-file section's out page matches if the writer has written it
//     (its page file block, whichever block that is); a page never
//     written is a demand-zero page, which isn't read.
//   - Each page is read by itself, as the faulting page is: govax counts
//     the cluster as one read in its counts (PageCounts.Faults.Read, one
//     for the fault) and every page in PageCounts.PagesRead.
//
// The pages read with a fault aren't faults themselves: JPI$_PAGEFLTS
// counts the one fault, and so the cluster size shows in a program's
// fault counts, which is how the probe sees it.

// clusterMatch says whether the page k pages from the faulting one
// (negative below it), whose PTE is pte, is in the same backing store as
// the faulting page, to be read with it.
type clusterMatch func(k int, pte vm.PTE) bool

// readCluster reads the faulting page's neighbors with it, after a hard
// fault f the pager resolved for process env, returning how many it read.
// Each neighbor read has a valid PTE and is in the working set, as the
// faulting page is.
func (sys *System) readCluster(env *Environment, f vm.PageFault) int {
	size, match := sys.clusterRule(env, f.PTE)
	if size <= 1 {
		return 0
	}

	if n := sys.scanCluster(env, f, size, 1, match); n > 0 {
		return n
	}

	return sys.scanCluster(env, f, size, -1, match)
}

// clusterRule returns the cluster size for a hard fault on a page whose
// PTE was pte, and how its neighbors match it; a size of 0 for a page
// that doesn't cluster.
func (sys *System) clusterRule(env *Environment, pte vm.PTE) (int, clusterMatch) {
	if si, ok := pte.SectionIndex(); ok {
		if si >= uint32(len(env.procSections)) || env.procSections[si] == nil {
			return 0, nil
		}

		return sys.clusterSize(env.procSections[si].pfc), func(_ int, p vm.PTE) bool {
			idx, ok := p.SectionIndex()

			return ok && idx == si
		}
	}

	if gi, ok := pte.GlobalIndex(); ok {
		s, _, ok := sys.Sections.globalPage(gi)
		if !ok {
			return 0, nil
		}

		pfc := s.PFC
		if s.File == nil {
			pfc = 0
		}

		return sys.clusterSize(pfc), func(k int, p vm.PTE) bool {
			next, ok := p.GlobalIndex()
			if !ok || int64(next) != int64(gi)+int64(k) {
				return false
			}

			other, i, ok := sys.Sections.globalPage(next)

			return ok && other == s && sys.globalPageOut(s, i)
		}
	}

	if vbn, ok := pte.PageFileBlock(); ok && vbn != 0 {
		return sys.clusterSize(0), func(k int, p vm.PTE) bool {
			next, ok := p.PageFileBlock()

			return ok && next != 0 && int64(next) == int64(vbn)+int64(k)
		}
	}

	return 0, nil
}

// clusterSize is the cluster size for a section whose own is pfc: pfc, or
// PFCDEFAULT when it's 0; at most MaxPFCDefault.
func (sys *System) clusterSize(pfc uint32) int {
	if pfc == 0 {
		pfc = sys.Sysgen.PFCDefault
	}

	return int(min(pfc, MaxPFCDefault))
}

// globalPageOut reports whether page i of global section s is out and has
// something to read: a file section's page with no frame (a copy-on-
// reference section's pages never have one), or a page-file section's
// page the writer wrote to the page file. A page-file section's page
// never written is demand zero, and isn't.
func (sys *System) globalPageOut(s *GlobalSection, i int) bool {
	if i < 0 || i >= len(s.Frames) || s.Frames[i] != 0 {
		return false
	}

	return s.File != nil || i < len(s.blocks) && s.blocks[i] != 0
}

// scanCluster reads up to size-1 of the faulting page's neighbors in one
// direction (dir 1 up, -1 down), stopping at the first that doesn't
// match, isn't in the faulting page's region, or has no room or no frame;
// it returns how many it read.
func (sys *System) scanCluster(env *Environment, f vm.PageFault, size, dir int, match clusterMatch) int {
	db := sys.mem.Paging()
	base := int64(f.Addr &^ pageMask)
	region := f.Addr >> 30

	for k := 1; k < size; k++ {
		at := base + int64(dir*k)*pageSize
		if at < 0 || uint32(at)>>30 != region {
			return k - 1
		}

		va := uint32(at)

		pte, err := sys.mem.LookupPTEIn(sys.cpu, f.Space, va)
		if err != nil || !match(dir*k, pte) || db.FreeCount() == 0 {
			return k - 1
		}

		if !sys.readNeighbor(env, f.Space, va, pte) {
			return k - 1
		}
	}

	return size - 1
}

// readNeighbor reads the page at va, whose invalid PTE is pte, as part of
// a cluster: it takes an available working set entry (none, and it
// reads nothing), is resolved as a fault on it would be, and gets its
// valid PTE. It reports whether the page was read.
func (sys *System) readNeighbor(env *Environment, space vm.AddressSpace, va uint32, pte vm.PTE) bool {
	ws := env.WorkingSet()
	if ws == nil {
		return false
	}

	added, err := ws.AddAvailable(va, sys.faultIsGlobal(pte), env.replacementConditions())
	if err != nil || !added {
		return false
	}

	valid, _, ok := sys.resolveFault(vm.PageFault{Space: space, Addr: va, PTE: pte})
	if !ok {
		env.leaveWorkingSet(va)

		return false
	}

	if sys.mem.StorePTEIn(sys.cpu, space, va, valid) != nil {
		env.releasePTE(va, valid)

		return false
	}

	sys.PageCounts.PagesRead++

	return true
}
