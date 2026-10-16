package corevms

import (
	"slices"

	"github.com/tucats/govax/internal/sched"
)

// The reclaimer (docs/PHASE-63 - working sets.md, subtask 8; docs/PHASE-62
// - VMS paging.md, Decision 7).
//
// # Why it's needed
//
// Every new page a process touches takes a frame from the head of the
// free page list, and pages leaving working sets give frames back only
// when their contents are safe elsewhere. If processes keep their pages,
// or their pages go to the modified list, the free list runs down, and
// once it's empty a fault fails for want of memory. On VMS the swapper
// watches the free list: when the pager leaves it with fewer than the
// SYSGEN parameter FREELIM pages, the swapper wakes and works to bring it
// back up to FREEGOAL (the book's 17.1.1). Of its four ways, govax uses
// two:
//
//  1. it writes modified pages, until the modified list is down to its low
//     limit (MPW_LOLIMIT), so their frames move to the free list (the
//     modified page writer's own run, mpw.go);
//  2. it trims working sets: each process whose working set holds more
//     pages than its WSQUOTA (one that grew past it while memory was
//     plentiful) loses its oldest pages, down to WSQUOTA (the first of
//     the book's three passes over its selection table, 17.2.2).
//
// The other two, outswapping process headers and whole processes, need a
// swapper govax doesn't have (Phase 62's "Future work"), as does the
// second pass, which shrinks a set to SWPOUTPGCNT before an outswap.
//
// Trimming moves pages to the lists, as any page leaving a working set:
// only clean pages (a file section's, unchanged since read) reach the
// free list at once; written ones go to the modified list, which the
// writer then empties if it can. So after trimming, the writer runs
// again. Without a page file (pagefile.go), a demand-zero page has
// nowhere to be written, and trimming one gains no free frame.
//
// # When it runs
//
// The frame allocator calls wakeReclaimer when an allocation leaves the
// free list below FREELIM (vm.Memory.SetFreeLimit), and so does a soft
// fault that takes a frame back off the free list (softFault): either
// way the list is shorter. (A soft fault off the modified list, like a
// page leaving a working set, leaves the free list as it was, and wakes
// nothing: so a working set the reclaimer trimmed can take its own pages
// back off the modified list, past its quota, until its next allocation.)
//
// The reclaimer doesn't run there, in the middle of a fault, but at the
// next instruction boundary (vm.Memory.AtNextInstruction), when no page
// is in use by an instruction: the faulting page has its frame by then,
// and nothing the instruction has translated can be taken from under
// it. It's synchronous, as the writer is (Decision 7): no separate
// process, and its writes are done when it returns.
//
// # Whose working sets
//
// The swapper takes processes in the order of its selection table (the
// book's Table 17-2), which puts first those that would suffer least from
// losing pages: suspended processes, then those waiting for an event
// flag or hibernating, then the rest, the computable ones last.
// reclaimOrder is that table without its subtler columns (how long a
// process has waited, whether it's doing direct I/O, its priority and
// initial quantum). Within one state the order is by PID, govax's
// choice: the book scans each state's queue from its newest entry, which
// govax's scheduler doesn't expose (unconfirmed). The process the CPU is
// running comes last of all. Trimming stops once the free list reaches
// FREEGOAL.
//
// The book doesn't say whether trimming lowers the working set's limit;
// govax's leaves the limit alone (paging.WorkingSet.Shrink), so the set
// may grow back past its quota once memory is plentiful again, by the
// growth rule (GROWLIM). With the free list low, it can't (unconfirmed).
//
// # Not running in vain
//
// With pages the writer can't write on the modified list, every wake
// would walk the whole list for nothing. So the reclaimer remembers how
// many it passed over (reclaimStuck), and its writer runs only when the
// list holds more than MPW_LOLIMIT by more than that; the count is
// forgotten when the free list is back to FREELIM (govax's rule, as the
// writer's own mpwStuck is).

// installReclaimer has the frame allocator wake the reclaimer when the
// free list falls below FREELIM. NewSystem and SetSysgen call it.
func (sys *System) installReclaimer() {
	sys.mem.SetFreeLimit(int(sys.Sysgen.FreeLimit), sys.wakeReclaimer)
}

// wakeReclaimerIfLow wakes the reclaimer if the free list holds fewer
// than FREELIM frames: softFault's check, after taking a frame off it.
func (sys *System) wakeReclaimerIfLow() {
	if db := sys.mem.Paging(); db != nil && db.FreeCount() < int(sys.Sysgen.FreeLimit) {
		sys.wakeReclaimer()
	}
}

// wakeReclaimer asks for the reclaimer to run at the next instruction
// boundary (atInstructionBoundary).
func (sys *System) wakeReclaimer() {
	sys.mem.AtNextInstruction(sys.atInstructionBoundary)
}

// atInstructionBoundary is the work the pager leaves for the next
// instruction boundary: working sets left over their limits come back
// down (trimWorkingSets), the page file is opened if the modified page
// writer wants it (openWantedPageFile), and, if the free list is below
// FREELIM, the reclaimer runs. Both share the one routine vm.Memory keeps for the
// boundary.
func (sys *System) atInstructionBoundary() {
	sys.trimWorkingSets()
	sys.openWantedPageFile(sys.Sysgen.MPWHiLimit)
	sys.reclaim()
}

// reclaim is the reclaimer's run (see above): if the free list holds
// fewer than FREELIM frames, it writes modified pages, trims working sets
// past their quotas, and writes again, stopping when the free list holds
// FREEGOAL frames.
func (sys *System) reclaim() {
	db := sys.mem.Paging()
	if db == nil {
		return
	}

	limit, goal := int(sys.Sysgen.FreeLimit), int(sys.Sysgen.FreeGoal)
	if db.FreeCount() >= limit {
		sys.reclaimStuck = 0

		return
	}

	sys.PageCounts.Reclaims++

	sys.reclaimWrite()

	for _, env := range sys.reclaimOrder() {
		if db.FreeCount() >= goal {
			break
		}

		sys.trimToQuota(env)
	}

	if db.FreeCount() < goal {
		sys.reclaimWrite()
	}
}

// reclaimWrite is the reclaimer's use of the modified page writer: it
// writes the pages it can until the modified list is down to MPW_LOLIMIT,
// unless the list is no longer than that by more than the pages it
// couldn't write the last time.
func (sys *System) reclaimWrite() {
	sys.openWantedPageFile(sys.Sysgen.MPWLoLimit)

	lo := int(sys.Sysgen.MPWLoLimit)
	if sys.mem.Paging().ModifiedCount() > lo+sys.reclaimStuck {
		sys.reclaimStuck = sys.writeModifiedPages(lo)
	}
}

// trimToQuota takes env's oldest pages out of its working set until it
// holds no more than its WSQUOTA (paging.WorkingSet.Shrink), each into
// transition as any page leaving a working set (evictPage).
func (sys *System) trimToQuota(env *Environment) {
	ws := env.ws
	if ws == nil || ws.InUse() <= int(env.Process.WSQuota) {
		return
	}

	removed := ws.Shrink(int(env.Process.WSQuota), env.replacementConditions())
	sys.PageCounts.Trimmed += uint64(len(removed))

	for _, p := range removed {
		env.evictPage(p)
	}
}

// reclaimOrder lists the processes whose working sets the reclaimer may
// trim, in the order it tries them (see above): by their scheduling
// states' places in the swapper's selection table (reclaimRank), then by
// PID, with the process the CPU holds last.
func (sys *System) reclaimOrder() []*Environment {
	db := sys.mem.Paging()

	var list []*Environment

	for _, env := range sys.Processes() {
		if env.ws != nil && env.wsDB == db {
			list = append(list, env)
		}
	}

	rank := func(env *Environment) int {
		if env == sys.Current() {
			return len(reclaimRank) + 1
		}

		if info, ok := sys.sched.Info(handle(env)); ok && sys.engine != nil {
			if r, ok := reclaimRank[info.State]; ok {
				return r
			}
		}

		return len(reclaimRank)
	}

	slices.SortStableFunc(list, func(a, b *Environment) int {
		if d := rank(a) - rank(b); d != 0 {
			return d
		}

		return int(a.Process.PID) - int(b.Process.PID)
	})

	return list
}

// reclaimRank is each scheduling state's place in the swapper's
// selection table (the book's Table 17-2), the states it takes pages from
// first ranked lowest. A state not here (a computable one) comes after
// them.
var reclaimRank = map[sched.State]int{
	sched.StateSUSP:  0,
	sched.StateLEF:   1,
	sched.StateHIB:   2,
	sched.StateCEF:   3,
	sched.StateFPG:   4,
	sched.StateCOLPG: 5,
	sched.StateMWAIT: 6,
	sched.StatePFW:   7,
}
