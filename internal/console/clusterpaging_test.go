package console_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
)

// Phase 65's subtask 6 (docs/PHASE-65 - sections and paging.md): read
// clustering. With paging on, a hard fault reads the faulting page's
// neighbors in the same backing store with it, up to the section's
// cluster size ($CRMPSC's pfc) or PFCDEFAULT, scanning up first and down
// only if the scan up read nothing, and stopping where the working set
// has no available entry. A neighbor read with a fault doesn't fault
// when it's touched, so a program reading a section's pages in order
// takes one read fault per cluster.

// clusterSource maps PAGES.DAT, opened read-only, as a 64-page section
// made with flags and pfc (checkpoint 1, R7 its first address), then
// reads the first longword of every page, from the first page up, or
// with down set from the last page down, counting in R9 the pages that
// don't hold their block number (checkpoint 2). At each checkpoint it
// waits in a loop until the test sets R10 to the checkpoint's number, R6.
// R9 is a failing service's status before checkpoint 2.
func clusterSource(flags string, pfc int, down bool) string {
	start, step, delta, offset := 1, 512, 1, 0
	if down {
		start, step, delta, offset = 64, -512, -1, 63*512
	}

	return fmt.Sprintf(`	.title	cluster
	$fabdef
	$secdef
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.macro	ok,?l
	movl	r0,r9
	blbs	r0,l
	brw	fail
l:
	.endm	ok
	.psect	data,noexe,wrt,page
fab:	$fab	fnm=<DUA0:[000000]PAGES.DAT>,fac=get,fop=ufo,shr=get
inadr:	.long	^x200,^x200
range:	.blkl	2
name:	.ascid	/CLUSTER/
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	clrl	r7
	$open	fab=fab
	ok
	$crmpsc_s inadr=inadr,retadr=range,gsdnam=name,flags=#<%s>,-
		chan=fab+fab$l_stv,pagcnt=#64,pfc=#%d
	ok
	movl	range,r7
	stop	1
	clrl	r9
	movl	#64,r3
	addl3	#%d,r7,r2
	movl	#%d,r4
30$:	cmpl	(r2),r4
	beql	31$
	incl	r9
31$:	addl2	#%d,r2
	addl2	#%d,r4
	sobgtr	r3,30$
	stop	2
	movl	#1,r0
	ret
fail:	brb	fail
	.end	start
`, flags, pfc, offset, start, step, delta)
}

// clusterRun is a run of a program under clusterSource's checkpoints.
type clusterRun struct {
	c       *console.Console
	env     *corevms.Environment
	section uint32

	// checkpoint runs to checkpoint n, after letting the program past the
	// one before.
	checkpoint func(n uint32)
}

// startCluster builds and runs source, with PAGES.DAT made, to
// checkpoint 1, and returns the run; R7 is the section's address.
func startCluster(t *testing.T, name, source string) *clusterRun {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	t.Cleanup(func() {
		if t.Failed() {
			t.Log(out.String())
		}
	})

	if c.Mem.Paging() == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	makePagesFile(t, c)
	exe := buildImage(t, c, name, source)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	r := &clusterRun{c: c, env: c.ProcessOne()}

	r.checkpoint = func(n uint32) {
		t.Helper()

		if n > 1 {
			c.CPU.SetGPR(10, n-1)
		}

		runUntil(t, c, 2_000_000, func() bool { return c.CPU.GPR(6) == n })
		step(t, c, 4)
		checkWorkingSets(t, c)

		if err := c.Mem.Paging().Check(); err != nil {
			t.Fatalf("checkpoint %d: %v", n, err)
		}
	}

	r.checkpoint(1)

	if r.section = c.CPU.GPR(7); r.section == 0 {
		t.Fatalf("checkpoint 1: the section wasn't mapped (status %08X)", c.CPU.GPR(9))
	}

	return r
}

// clusterSettings turns paging on with a working set big enough for the
// whole section, and PFCDEFAULT pfcDefault.
func clusterSettings(t *testing.T, pfcDefault int) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "200")
	setSetting(t, "vax.sysgen.pql_dwsquota", "200")
	setSetting(t, "vax.sysgen.pql_dwsextent", "300")
	setSetting(t, machine.PFCDefaultSetting, strconv.Itoa(pfcDefault))
}

// TestReadClustering reads a 64-page section of PAGES.DAT in order, up
// and down, for each kind of file section and several cluster sizes, and
// checks that each cluster is one read fault, every page is read once,
// and each holds its block.
func TestReadClustering(t *testing.T) {
	const (
		private = "sec$m_expreg"
		global  = "sec$m_gbl!sec$m_expreg"
		gblCRF  = "sec$m_gbl!sec$m_expreg!sec$m_wrt!sec$m_crf"
	)

	for _, tc := range []struct {
		name       string
		flags      string
		pfc        int
		pfcDefault int
		down       bool
		faults     uint64
	}{
		{"private pfc 0, PFCDEFAULT 16", private, 0, 16, false, 4},
		{"private pfc 0, PFCDEFAULT 32", private, 0, 32, false, 2},
		{"private pfc 0, PFCDEFAULT 1", private, 0, 1, false, 64},
		{"private pfc 0, PFCDEFAULT 0", private, 0, 0, false, 64},
		{"private pfc 1", private, 1, 32, false, 64},
		{"private pfc 4", private, 4, 32, false, 16},
		{"private pfc 5", private, 5, 32, false, 13},
		{"private pfc 64", private, 64, 32, false, 1},
		{"private pfc 200", private, 200, 32, false, 1},
		{"private pfc 16, down", private, 16, 32, true, 4},
		{"private pfc 5, down", private, 5, 32, true, 13},
		{"global pfc 16", global, 16, 32, false, 4},
		{"global pfc 0, PFCDEFAULT 8, down", global, 0, 8, true, 8},
		{"global copy-on-reference pfc 8", gblCRF, 8, 32, false, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clusterSettings(t, tc.pfcDefault)

			r := startCluster(t, "cluster", clusterSource(tc.flags, tc.pfc, tc.down))
			c, p := r.c, r.env.Process

			before, read := p.Faults, c.RTL.PageCounts.PagesRead

			r.checkpoint(2)

			if got := c.CPU.GPR(9); got != 0 {
				t.Errorf("%d of the section's pages read wrong", got)
			}

			if got := p.Faults.Read - before.Read; got != tc.faults {
				t.Errorf("%d read faults, want %d", got, tc.faults)
			}

			if got := p.Faults.Total() - before.Total(); got != tc.faults {
				t.Errorf("%d faults in all, want %d", got, tc.faults)
			}

			if got := c.RTL.PageCounts.PagesRead - read; got != wsPagesBlocks {
				t.Errorf("%d pages read, want %d", got, wsPagesBlocks)
			}

			// Every page is valid and in the working set, a global
			// section's held by one share, a copy-on-reference page the
			// process's own.
			db := c.Mem.Paging()

			for i := range wsPagesBlocks {
				va := r.section + uint32(i)*512

				pte := pteIn(t, c, va)
				if !pte.Valid() {
					t.Errorf("section page %d's PTE is %08X, not valid", i, uint32(pte))

					continue
				}

				if _, ok := c.RTL.WorkingSet().Lookup(va); !ok {
					t.Errorf("section page %d isn't in the working set", i)
				}

				e, err := db.Entry(paging.PFN(pte.PFN()))
				if err != nil {
					t.Fatal(err)
				}

				switch tc.flags {
				case global:
					if e.Type != paging.GlobalPage || e.ShareCount != 1 {
						t.Errorf("section page %d's frame is %+v, want a global page with one share", i, e)
					}
				case gblCRF:
					if e.Type != paging.ProcessPage || e.Backing.Kind != paging.PageFile || !e.SavedModify {
						t.Errorf("section page %d's frame is %+v, want an effectively modified private copy", i, e)
					}
				}
			}

			c.CPU.SetGPR(10, 2)
			finishImage(t, c)
		})
	}
}

// TestReadClusteringWorkingSet reads the section with a 30-page working
// set and a cluster size of 64: a cluster stops where the working set
// has no available entry, rather than take a page out, so the pages come
// in a few clusters at first and then a fault at a time, and every page
// still reads right.
func TestReadClusteringWorkingSet(t *testing.T) {
	clusterSettings(t, 32)
	setSetting(t, "vax.sysgen.pql_dwsdefault", strconv.Itoa(wsLimit))

	r := startCluster(t, "cluster", clusterSource("sec$m_expreg", 64, false))
	c, p := r.c, r.env.Process
	ws := c.RTL.WorkingSet()

	before, read := p.Faults, c.RTL.PageCounts.PagesRead

	// The first page's cluster fills the working set to its limit, and no
	// page leaves for it.
	room := ws.Size() - ws.InUse()

	c.CPU.SetGPR(10, 1)
	runUntil(t, c, 2_000_000, func() bool { return p.Faults.Read > before.Read })

	if got := ws.InUse(); got != ws.Size() {
		t.Errorf("after the first fault the working set holds %d pages, want its limit, %d", got, ws.Size())
	}

	if got, want := c.RTL.PageCounts.PagesRead-read, uint64(room); got != want {
		t.Errorf("the first fault read %d pages, want %d (the room in the working set)", got, want)
	}

	if soft := p.Faults.Soft() - before.Soft(); soft != 0 {
		t.Errorf("%d soft faults by the first cluster, want none", soft)
	}

	runUntil(t, c, 2_000_000, func() bool { return c.CPU.GPR(6) == 2 })
	step(t, c, 4)
	checkWorkingSets(t, c)

	if got := c.CPU.GPR(9); got != 0 {
		t.Errorf("%d of the section's pages read wrong", got)
	}

	if got := p.Faults.Read - before.Read; got <= 1 || got >= wsPagesBlocks {
		t.Errorf("%d read faults, want more than one and fewer than %d", got, wsPagesBlocks)
	}

	if ws.InUse() > ws.Size() {
		t.Errorf("the working set holds %d pages, past its limit, %d", ws.InUse(), ws.Size())
	}

	c.CPU.SetGPR(10, 2)
	finishImage(t, c)
}

// pageFileClusterSource fills each of 64 demand-zero pages ($EXPREG, R7
// the first) with its number, from 1, in its first longword, with a
// 30-page working set, so that most leave it and go to the page file
// (checkpoint 1; some still wait on the modified list, below the
// writer's goal); grows the working set limit by 100 pages with $ADJWSL
// (checkpoint 2); and reads every page back in order, counting in R9 the
// pages that don't hold their number (checkpoint 3).
const pageFileClusterSource = `	.title	pfclust
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.macro	ok,?l
	movl	r0,r9
	blbs	r0,l
	brw	fail
l:
	.endm	ok
	.psect	data,noexe,wrt,page
range:	.blkl	2
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	clrl	r7
	$expreg_s pagcnt=#64,retadr=range
	ok
	movl	range,r7
	movl	r7,r2
	movl	#1,r4
10$:	movl	r4,(r2)
	addl2	#512,r2
	aoblss	#65,r4,10$
	stop	1
	$adjwsl_s pagcnt=#100
	ok
	stop	2
	movl	r7,r2
	clrl	r9
	movl	#1,r4
30$:	cmpl	(r2),r4
	beql	31$
	incl	r9
31$:	addl2	#512,r2
	aoblss	#65,r4,30$
	stop	3
	movl	#1,r0
	ret
fail:	brb	fail
	.end	start
`

// TestReadClusteringPageFile writes 64 pages to the page file, reuses
// their frames, and reads them back in order with room in the working
// set: pages in successive page file blocks come back as one cluster, so
// there are fewer read faults than pages read from the page file.
func TestReadClusteringPageFile(t *testing.T) {
	pageFileSettings(t)
	setSetting(t, "vax.sysgen.pql_dwsquota", "200")
	setSetting(t, "vax.sysgen.pql_dwsextent", "300")
	setSetting(t, machine.PFCDefaultSetting, "16")

	r := startCluster(t, "pfclust", pageFileClusterSource)
	c, p := r.c, r.env.Process

	r.checkpoint(2)

	// Each page out of the working set and written has its frame reused,
	// leaving its PTE a page file block.
	stealFreeFrames(t, c)

	// The read faults to expect: reading in order, a page in the page
	// file faults, and its cluster takes the pages after it whose blocks
	// follow its block, up to PFCDEFAULT pages in all.
	const pfcDefault = 16

	inPageFile, want := 0, uint64(0)

	for i := 0; i < wsPagesBlocks; {
		vbn, ok := pteIn(t, c, r.section+uint32(i)*512).PageFileBlock()
		if !ok || vbn == 0 {
			i++

			continue
		}

		want++
		inPageFile++
		i++

		for n := 1; n < pfcDefault && i < wsPagesBlocks; n++ {
			next, ok := pteIn(t, c, r.section+uint32(i)*512).PageFileBlock()
			if !ok || next != vbn+uint32(n) {
				break
			}

			inPageFile++
			i++
		}
	}

	if inPageFile < pfcDefault || want == uint64(inPageFile) {
		t.Fatalf("%d pages in the page file, %d runs of successive blocks; want at least %d pages, some in runs",
			inPageFile, want, pfcDefault)
	}

	before, reads := p.Faults, c.RTL.PageCounts.PageFileReads

	r.checkpoint(3)

	if got := c.CPU.GPR(9); got != 0 {
		t.Errorf("%d pages read back wrong", got)
	}

	faults, pages := p.Faults.Read-before.Read, c.RTL.PageCounts.PageFileReads-reads

	// A fault whose scan up reads nothing scans down, and may so read a
	// page of the image's below the array, if its block came just before.
	if pages < uint64(inPageFile) {
		t.Errorf("%d pages read from the page file, want at least %d", pages, inPageFile)
	}

	if faults != want {
		t.Errorf("%d read faults for %d pages in the page file, want %d", faults, pages, want)
	}

	checkPageFileHolders(t, c, "after the pages were read back")

	c.CPU.SetGPR(10, 3)
	finishImage(t, c)
}
