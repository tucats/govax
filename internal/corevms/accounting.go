package corevms

import "github.com/tucats/govax/internal/vm"

// Paging accounting (docs/PHASE-63 - working sets.md, subtask 8).
//
// # Page faults by kind
//
// Each fault the pager resolves is counted twice: in the faulting
// process's counts and in the system's. Its kind says how it was
// resolved, which is what its cost was:
//
//   - a soft fault took a frame that was already in memory: one on the
//     free list or the modified list, still holding the page (a page that
//     had left the working set), or a global section's page that another
//     process, or this one, had already brought in (a "global valid"
//     fault, in VMS's MONITOR's words);
//   - a demand-zero fault gave the page a new frame of zeros;
//   - a hard fault read the page from a file (a section's, or an image's),
//     the only kind that waits for I/O on VMS.
//
// $GETJPI's JPI$_PAGEFLTS is the process's total, as before. With paging
// off there are no soft faults but global valid ones.
//
// # Pages in the working set
//
// With paging on, a process's pages in memory are its working set's
// (paging.WorkingSet's counts: PPGCNT, its own pages, and GPGCNT, global
// section pages; their most, WSPEAK). With paging off there's no working
// set, but every valid page is in memory, so residentPages counts the
// valid P0 and P1 PTEs, a page being global if its frame is a global
// section's. That count is the working set's size on a system that never
// pages; its peak is the most any count of it has seen (the process's
// WSPeak), so it's a lower bound: govax counts when something asks
// ($GETJPI, SHOW SYSTEM, LOGOUT, the termination message) and at image
// exit, before the image's pages go.

// FaultCounts are page faults by kind (see above).
type FaultCounts struct {
	// FreeList and ModifiedList are soft faults that took a frame back
	// off the free or the modified page list; GlobalValid are soft faults
	// on a global section page already in memory.
	FreeList, ModifiedList, GlobalValid uint64

	// DemandZero are faults that gave a page a new frame of zeros.
	DemandZero uint64

	// Read are hard faults, which read the page from a file.
	Read uint64
}

// Soft is how many of the faults were soft.
func (f FaultCounts) Soft() uint64 {
	return f.FreeList + f.ModifiedList + f.GlobalValid
}

// Total is how many faults there were.
func (f FaultCounts) Total() uint64 {
	return f.Soft() + f.DemandZero + f.Read
}

// PageCounts are the system's paging counts.
type PageCounts struct {
	// Faults are the faults on every process's P0 and P1 pages, by kind.
	Faults FaultCounts

	// Written counts the pages the modified page writer has written
	// (mpw.go), on its own or for the reclaimer, and WriteIOs the writes
	// it made, one for each cluster (MONITOR PAGE's page writes and page
	// write I/Os).
	Written, WriteIOs uint64

	// PageFileReads counts the pages read back from the page file, by
	// hard faults on page file PTEs (filesec.go's pageInFromPageFile):
	// among Faults.Read, which counts section files' reads too.
	PageFileReads uint64

	// Reclaims counts the reclaimer's runs, and Trimmed the pages it took
	// out of working sets (reclaim.go).
	Reclaims, Trimmed uint64
}

// faultKind is how the pager resolved a fault.
type faultKind int

const (
	faultFreeList faultKind = iota
	faultModifiedList
	faultGlobalValid
	faultDemandZero
	faultRead
)

// add counts one fault of kind k.
func (f *FaultCounts) add(k faultKind) {
	switch k {
	case faultFreeList:
		f.FreeList++
	case faultModifiedList:
		f.ModifiedList++
	case faultGlobalValid:
		f.GlobalValid++
	case faultDemandZero:
		f.DemandZero++
	case faultRead:
		f.Read++
	}
}

// countFault counts a resolved fault of kind k on a process's own page
// (P0 or P1): in its Process.PageFaults, $GETJPI's JPI$_PAGEFLTS, which
// LIB$INIT_TIMER and its kind report (docs/PHASE-55, subtask 5), in its
// counts by kind, and in the system's. A fault on a system (S0) page
// belongs to no process, and isn't counted. VMS counts every fault a
// working set sees, soft ones included, and so does govax with paging
// on; with it off there are no soft faults but global valid ones.
func (sys *System) countFault(f vm.PageFault, k faultKind) {
	if f.Addr >= s0Base {
		return
	}

	if env := sys.processForSpace(f.Space); env != nil {
		env.Process.PageFaults++
		env.Process.Faults.add(k)
		sys.PageCounts.Faults.add(k)
	}
}

// residentPages counts env's pages in memory, its own and global section
// pages (PPGCNT and GPGCNT), and notes their sum in its peak (WSPeak):
// with paging on, its working set's counts; with it off, its valid P0
// and P1 pages (see above).
func (sys *System) residentPages(env *Environment) (process, global int) {
	if ws := env.ws; ws != nil && env.wsDB == sys.mem.Paging() && env.wsDB != nil {
		c := ws.Counts()
		process, global = c.Process, c.Global
		env.Process.WSPeak = max(env.Process.WSPeak, uint32(c.Peak))
	} else if sys.mem.Paging() == nil {
		process, global = sys.validPages(env)
	}

	env.Process.WSPeak = max(env.Process.WSPeak, uint32(process+global))

	return process, global
}

// validPages counts the valid PTEs of env's P0 and P1 regions: those
// whose frames are global section pages, and the rest.
func (sys *System) validPages(env *Environment) (process, global int) {
	if env.Space == nil {
		return 0, 0
	}

	as := env.Space.AddressSpace
	count := func(base, pages uint32) {
		for i := range pages {
			pte, err := sys.mem.LookupPTEIn(sys.cpu, as, base+i*pageSize)
			if err != nil || !pte.Valid() {
				continue
			}

			if _, ok := sys.Sections.byFrame[pte.PFN()]; ok {
				global++
			} else {
				process++
			}
		}
	}

	count(0, env.Space.P0Pages)
	count(s0Base-env.Space.P1Pages*pageSize, env.Space.P1Pages)

	return process, global
}

// WorkingSetPeak is the most pages env's working set has been seen to
// hold (JPI$_WSPEAK), counting it now (residentPages).
func (sys *System) WorkingSetPeak(env *Environment) uint32 {
	sys.residentPages(env)

	return env.Process.WSPeak
}
