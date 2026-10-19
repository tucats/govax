package corevms

import "maps"

// The paging file quota, PGFLQUOTA (docs/PHASE-64 - page file.md,
// subtask 5; the paging plan's Decision 10, as the survey's finding 10
// changed it).
//
// On VMS every page whose contents may one day have to go to the page
// file, a demand-zero page or the private copy of a copy-on-reference
// page, is charged against its job's PGFLQUOTA when it's created, and
// the charge is given back when the page is deleted (VAX/VMS Internals
// and Data Structures, 16.2.1.1 and 16.2.2.1). The quota is pooled: every
// process in the job draws on one count (JIB$L_PGFLCNT, which $GETJPI
// reports as JPI$_PAGFILCNT, the pages left). A service that creates
// pages, $CRETVA or $EXPREG, creates them one at a time and stops with
// SS$_EXQUOTA when the quota runs out, having made the pages before it.
//
// govax charges only with paging on, so that with it off nothing
// changes. It keeps the charged pages as a set per process (the
// Environment's pageFileCharges, by page address), so that no page is
// charged twice, and the job's total (Job.PageFileCharged). A page is
// charged:
//
//   - at creation, by the services that create pages: $CRETVA, $EXPREG of
//     P0 or P1, and the heap's expansion of P0 (expandP0), each refusing
//     pages past the quota;
//   - at mapping, a copy-on-reference section's pages, private or global
//     ($CRMPSC, $MGBLSC; filesec.go's mapChargedPage, Phase 65), refused
//     past the quota the same way;
//   - at activation, a mapped image's writable sections (copy-on-reference
//     image sections) and demand-zero sections (imagesec.go), and its user
//     stack (userstack.go), whose automatic expansion charges each page it
//     creates.
//
// Every page that can hold contents only memory holds is created by one
// of these, so a fault never charges (Phase 67's subtask 6: until then
// every P0 and P1 page existed from the start, and a page no service
// created, such as one the console's DEPOSIT wrote, was charged at its
// first touch). With paging off, when images are copied into P0, nothing
// is charged. Only the system zones' control page (vmzone.go) is charged
// even past the quota, which JPI$_PAGFILCNT then shows as a negative
// count (unconfirmed).
//
// The charge is given back when the page is deleted (releasePTE: $DELTVA,
// $CRETVA over the page, image rundown), and a process's whole charge
// when the process is deleted (refundAllPageFile).
//
// Each process also notes the most pages it has had charged at once, its
// peak (Process.PageFilePeak): LOGOUT/FULL's "Peak page file size" and
// the termination message's ACC$L_PGFLPEAK.

// chargingPageFile reports whether env's pages are charged against the
// quota: with paging on, for a process with a job.
func (env *Environment) chargingPageFile() bool {
	return env.mem != nil && env.mem.Paging() != nil && env.Process != nil && env.Process.Job != nil
}

// pageFileChargeSet returns env's set of charged pages, starting it
// afresh (and giving its old charges back to the job) when the PFN
// database has changed since it was made: VMINIT built a new one, and
// every page the old set held is gone.
func (env *Environment) pageFileChargeSet() map[uint32]bool {
	db := env.mem.Paging()
	if env.pageFileCharges == nil || env.chargeDB != db {
		env.dropPageFileCharges()
		env.pageFileCharges = map[uint32]bool{}
		env.chargeDB = db
	}

	return env.pageFileCharges
}

// dropPageFileCharges gives every page env has charged back to its job.
func (env *Environment) dropPageFileCharges() {
	if env.Process != nil && env.Process.Job != nil {
		job := env.Process.Job
		job.PageFileCharged -= min(job.PageFileCharged, uint32(len(env.pageFileCharges)))
	}

	env.pageFileCharges = nil
}

// PageFileRemaining is JPI$_PAGFILCNT: how many pages env's job may still
// charge against its PGFLQUOTA, negative when forced charges took it past
// the quota.
func (env *Environment) PageFileRemaining() int64 {
	job := env.Process.Job

	return int64(job.Pooled.PGFLQUOTA) - int64(job.PageFileCharged)
}

// PageFileCharged is how many of env's pages are charged against its
// job's quota now.
func (env *Environment) PageFileCharged() uint32 {
	if !env.chargingPageFile() {
		return 0
	}

	return uint32(len(env.pageFileChargeSet()))
}

// canChargePage reports whether the page at va could be charged without
// taking the job past its quota: it's charged already, or there's a page
// left, or nothing is charged at all (paging off).
func (env *Environment) canChargePage(va uint32) bool {
	if !env.chargingPageFile() {
		return true
	}

	return env.pageFileChargeSet()[va&^pageMask] || env.PageFileRemaining() >= 1
}

// chargePage charges the page at va against the job's quota, if it
// isn't charged already. Unless force is set, it refuses, charging
// nothing, when the job has no page left. It reports whether the page is
// charged (always true with paging off, when nothing is charged).
func (env *Environment) chargePage(va uint32, force bool) bool {
	if !env.chargingPageFile() {
		return true
	}

	set := env.pageFileChargeSet()
	va &^= pageMask

	if set[va] {
		return true
	}

	if !force && env.PageFileRemaining() < 1 {
		return false
	}

	set[va] = true
	env.Process.Job.PageFileCharged++
	env.Process.PageFilePeak = max(env.Process.PageFilePeak, uint32(len(set)))

	return true
}

// chargePages charges count pages from first up, all or none: it
// refuses, charging nothing, when the ones not already charged don't fit
// in what's left of the job's quota.
func (env *Environment) chargePages(first, count uint32) bool {
	if !env.chargingPageFile() {
		return true
	}

	set := env.pageFileChargeSet()
	needed := int64(0)

	for i := range count {
		if !set[(first+i*pageSize)&^pageMask] {
			needed++
		}
	}

	if needed > env.PageFileRemaining() {
		return false
	}

	for i := range count {
		env.chargePage(first+i*pageSize, true)
	}

	return true
}

// refundPage gives the charge for the page at va back to the job, if it
// was charged, as the page is deleted.
func (env *Environment) refundPage(va uint32) {
	if !env.chargingPageFile() {
		return
	}

	set := env.pageFileChargeSet()
	va &^= pageMask

	if set[va] {
		delete(set, va)

		env.Process.Job.PageFileCharged--
	}
}

// refundPagesBelow gives back the charges of every page of env's below
// limit: image rundown's for P0, whose pages a service charged but
// nothing touched still have the PTE they started with, which rundown
// passes over.
func (env *Environment) refundPagesBelow(limit uint32) {
	if !env.chargingPageFile() {
		return
	}

	for va := range env.pageFileChargeSet() {
		if va < limit {
			env.refundPage(va)
		}
	}
}

// refundPagesIn gives back the charges of env's pages from low up to,
// not including, high.
func (env *Environment) refundPagesIn(low, high uint32) {
	if !env.chargingPageFile() {
		return
	}

	for va := range env.pageFileChargeSet() {
		if va >= low && va < high {
			env.refundPage(va)
		}
	}
}

// refundAllPageFile gives back every charge env holds, as the process is
// deleted: a subprocess's charges return to its job's pool.
func (env *Environment) refundAllPageFile() {
	if env.chargingPageFile() {
		env.pageFileChargeSet()
	}

	env.dropPageFileCharges()
}

// pageFileIndex is the index govax's one page file has in VMS's table of
// page and swap files: 3, as the VMS 7.1 VAX of Phase 64's probe gave
// its page file (JPI$_PAGFILLOC %X03000000, SHOW MEMORY/FILES/FULL's
// "Paging File Number"), its swap file being 1. That VAX's SWPFILCNT was
// 2, so the page files' numbers seem to follow the swap files' slots
// (unconfirmed: one VAX, one page file).
const pageFileIndex = 3

// PageFileIndex is the page file's number in VMS's table of page and
// swap files (SHOW MEMORY/FILES/FULL's "Paging File Number").
const PageFileIndex = pageFileIndex

// pageFileLocation is JPI$_PAGFILLOC: the index of the page file the
// process is assigned to in the longword's fourth byte (the VSI system
// services manual), with paging on and a page file configured, else 0.
func (env *Environment) pageFileLocation() uint32 {
	if env.mem == nil || env.mem.Paging() == nil || env.Sysgen.PageFileName == "" {
		return 0
	}

	return pageFileIndex << 24
}

// PageFileChargedAt reports whether the page at va is charged against
// env's job's paging file quota.
func (env *Environment) PageFileChargedAt(va uint32) bool {
	return env.chargingPageFile() && env.pageFileChargeSet()[va&^pageMask]
}

// ChargedPages returns the addresses of env's pages charged against its
// job's paging file quota, as a set.
func (env *Environment) ChargedPages() map[uint32]bool {
	if !env.chargingPageFile() {
		return nil
	}

	return maps.Clone(env.pageFileChargeSet())
}
