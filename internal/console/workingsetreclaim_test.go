package console_test

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/rms"
)

// Phase 63's subtask 8 (docs/PHASE-63 - working sets.md): when the free
// list falls below FREELIM, the reclaimer trims working sets that grew
// past their quotas back to WSQUOTA, and has the modified page writer
// write what it can; and the faults are counted by kind, and reported
// by $GETJPI with the working set's counts.

// The working set the test gives process 1: a default of 30 pages, a
// quota of 40, and room, through $ADJWSL, for 100.
const (
	reclaimDefault = 30
	reclaimQuota   = 40
	reclaimLimit   = 100

	// reclaimSlack is how far past its quota the test lets the working
	// set be when the reclaimer is on: trimming takes the oldest pages,
	// the program's own code, stack, and data among them, and those come
	// back by soft faults off the modified list, which take no frame from
	// the free list and so don't wake the reclaimer.
	reclaimSlack = 4
)

// reclaimSource raises its working set limit by 70 pages with $ADJWSL
// (R9 the new limit), adds an 80-page array to P0 with $EXPREG (so its
// pages are demand-zero, not the image's), and touches each of its pages
// (checkpoint 1); maps PAGES.DAT as a writable private section and adds
// 1000 to the longword at the start of each of its 64 pages (checkpoint
// 2, R7 the section's first address); touches the array again and asks
// $GETJPIW for its PPGCNT, GPGCNT, WSPEAK, and PAGEFLTS (checkpoint 3,
// R9 the status, R11 the answers' address); and returns. At each
// checkpoint it waits in a loop until the test sets R10 to the
// checkpoint's number, R6. R8 is the array's address.
const reclaimSource = `	.title	wsreclaim
	$fabdef
	$secdef
	$jpidef
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
array:	.blkl	2
fab:	$fab	fnm=<DUA0:[000000]PAGES.DAT>,fac=<get,put>,fop=ufo,shr=get
inadr:	.long	^x200,^x200
range:	.blkl	2
limit:	.long	0
jpi:	.word	4,jpi$_ppgcnt
	.long	counts,0
	.word	4,jpi$_gpgcnt
	.long	counts+4,0
	.word	4,jpi$_wspeak
	.long	counts+8,0
	.word	4,jpi$_pageflts
	.long	counts+12,0
	.long	0
counts:	.blkl	4
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	pushal	limit
	pushl	#70
	calls	#2,g^sys$adjwsl
	movl	limit,r9
	$expreg_s pagcnt=#80,retadr=array
	ok
	movl	limit,r9
	movl	array,r8
	bsbw	touch
	stop	1
	$open	fab=fab
	ok
	$crmpsc_s inadr=inadr,retadr=range,flags=#<sec$m_expreg!sec$m_wrt>,-
		chan=fab+fab$l_stv,pagcnt=#64
	ok
	movl	range,r2
	movl	#64,r3
20$:	addl2	#1000,(r2)
	addl2	#512,r2
	sobgtr	r3,20$
	movl	range,r7
	stop	2
	bsbw	touch
	$getjpiw_s itmlst=jpi
	movl	r0,r9
	moval	counts,r11
	stop	3
	movl	#1,r0
	ret
touch:	movl	r8,r2
	movl	#80,r3
10$:	movl	r3,(r2)
	addl2	#512,r2
	sobgtr	r3,10$
	rsb
fail:	brb	fail
	.end	start
`

// TestReclaimer runs reclaimSource with the reclaimer always wanted
// (FREELIM and FREEGOAL above any free list) and with it off (FREELIM
// 0), and checks the working set, the section's pages, and the counts at
// each checkpoint. The modified page writer's own threshold stays at its
// default, above anything the program sends to the modified list, so
// only the reclaimer writes; its low limit is 0, so the reclaimer writes
// every page it can.
func TestReclaimer(t *testing.T) {
	for _, on := range []bool{true, false} {
		name := "off"
		if on {
			name = "on"
		}

		t.Run(name, func(t *testing.T) { reclaimerRun(t, on) })
	}
}

func reclaimerRun(t *testing.T, on bool) {
	freeLimit := "0"
	if on {
		freeLimit = "1000000"
	}

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	setSetting(t, "vax.sysgen.pql_dwsquota", "40")
	setSetting(t, "vax.sysgen.mpw_lolimit", "0")
	setSetting(t, "vax.sysgen.freelim", freeLimit)
	setSetting(t, "vax.sysgen.freegoal", "1000000")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	db := c.Mem.Paging()
	if db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	env := c.ProcessOne()
	p := env.Process

	if p.WSDefault != reclaimDefault || p.WSQuota != reclaimQuota {
		t.Fatalf("DFWSCNT %d, WSQUOTA %d; want %d and %d", p.WSDefault, p.WSQuota, reclaimDefault, reclaimQuota)
	}

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.Commands().InitializeContainer(path, 400, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Commands().Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	pagesFile := rms.FileLocation{Name: "DUA0:[000000]PAGES.DAT"}

	blocks := make([][]byte, wsPagesBlocks)
	for i := range blocks {
		blocks[i] = make([]byte, 512)
		binary.LittleEndian.PutUint32(blocks[i], uint32(i+1))
	}

	if _, err := c.ContainerSession.CreateRecordFile(pagesFile, rms.ImageBlocks, blocks); err != nil {
		t.Fatal(err)
	}

	exe := buildImage(t, c, "wsreclaim", reclaimSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	checkpoint := func(n uint32) {
		t.Helper()

		runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == n })
		step(t, c, 4)
		checkWorkingSets(t, c)
	}

	release := func(n uint32) { c.CPU.SetGPR(10, n) }

	ws := env.WorkingSet()
	counts := &c.ProcessOne().PageCounts

	// Checkpoint 1: the limit is 100, but with the reclaimer on the set
	// holds no more than its quota, the array's other pages waiting on the
	// modified list (the writer can't write demand-zero pages). With it
	// off, the whole array is in the set.
	checkpoint(1)

	array := c.CPU.GPR(8)

	if got := c.CPU.GPR(9); got != reclaimLimit || ws.Size() != reclaimLimit {
		t.Fatalf("checkpoint 1: wsetlm %d, list size %d; want %d", got, ws.Size(), reclaimLimit)
	}

	inTransition := len(pagesInTransition(t, c, array, 80, paging.Modified))

	if on {
		if ws.InUse() > reclaimQuota+reclaimSlack || inTransition < 80-reclaimQuota-reclaimSlack {
			t.Errorf("checkpoint 1: %d pages in the set, %d of the array's in transition; want at most %d and at least %d",
				ws.InUse(), inTransition, reclaimQuota+reclaimSlack, 80-reclaimQuota-reclaimSlack)
		}

		if counts.Reclaims == 0 || counts.Trimmed < 80-reclaimQuota {
			t.Errorf("checkpoint 1: %d reclaimer runs, %d pages trimmed", counts.Reclaims, counts.Trimmed)
		}
	} else if ws.InUse() < 80 || inTransition != 0 || counts.Reclaims != 0 {
		t.Errorf("checkpoint 1: %d pages in the set, %d of the array's in transition, %d reclaimer runs; want at least 80, 0, 0",
			ws.InUse(), inTransition, counts.Reclaims)
	}

	// Checkpoint 2: with the reclaimer on, the section's pages it trimmed
	// were written to the file, and their frames are on the free list;
	// none waits on the modified list. With it off, nothing was written.
	release(1)
	checkpoint(2)

	section := c.CPU.GPR(7)
	if section == 0 {
		t.Fatalf("checkpoint 2: the section wasn't mapped (status %08X)", c.CPU.GPR(9))
	}

	free := pagesInTransition(t, c, section, wsPagesBlocks, paging.Free)

	if on {
		if len(free) < wsPagesBlocks-reclaimQuota-reclaimSlack || counts.Written < uint64(len(free)) {
			t.Errorf("checkpoint 2: %d section pages on the free list, %d pages written; want at least %d",
				len(free), counts.Written, wsPagesBlocks-reclaimQuota-reclaimSlack)
		}

		// Every section page out of the set is on the free list: none
		// waits on the modified list.
		valid := 0
		
		for i := range uint32(wsPagesBlocks) {
			if pteIn(t, c, section+i*512).Valid() {
				valid++
			}
		}

		if valid+len(free) != wsPagesBlocks {
			t.Errorf("checkpoint 2: %d section pages in the set, %d on the free list; the rest wait on the modified list",
				valid, len(free))
		}

		got, _, err := c.ContainerSession.ReadRecordFile(pagesFile, rms.ImageBlocks)
		if err != nil {
			t.Fatal(err)
		}

		for _, va := range free {
			i := (va - section) / 512
			if v := binary.LittleEndian.Uint32(got[i]); v != i+1001 {
				t.Errorf("checkpoint 2: page %d's frame is on the free list, but its block holds %d", i, v)
			}
		}
	} else if counts.Written != 0 || counts.Reclaims != 0 {
		t.Errorf("checkpoint 2: %d pages written, %d reclaimer runs; want none", counts.Written, counts.Reclaims)
	}

	// Checkpoint 3: the process's faults by kind: the array's pages, new
	// at first (demand-zero), and the section's, read from the file; with
	// the reclaimer on, the array's second sweep took its pages back off
	// the modified list (soft faults). $GETJPIW's answers are the working
	// set's counts and the process's faults.
	release(2)
	checkpoint(3)

	f := p.Faults

	if f.DemandZero < 80 || f.Read < wsPagesBlocks || f.Total() != uint64(p.PageFaults) {
		t.Errorf("checkpoint 3: faults %+v, %d in all; want at least 80 demand-zero and %d read, %d in all",
			f, f.Total(), wsPagesBlocks, p.PageFaults)
	}

	if on && f.ModifiedList < 80-reclaimQuota-reclaimSlack {
		t.Errorf("checkpoint 3: %d soft faults off the modified list, want at least %d", f.ModifiedList, 80-reclaimQuota-reclaimSlack)
	}

	if got := c.CPU.GPR(9); got != 1 {
		t.Fatalf("checkpoint 3: $GETJPIW returned %08X", got)
	}

	answers := make([]byte, 16)
	if err := c.Mem.Load(c.CPU, c.CPU.GPR(11), answers); err != nil {
		t.Fatal(err)
	}

	jpi := func(i int) uint32 { return binary.LittleEndian.Uint32(answers[4*i:]) }
	wc := ws.Counts()

	if jpi(0) != uint32(wc.Process) || jpi(1) != uint32(wc.Global) || jpi(2) != uint32(wc.Peak) || jpi(3) != p.PageFaults {
		t.Errorf("checkpoint 3: $GETJPIW PPGCNT %d, GPGCNT %d, WSPEAK %d, PAGEFLTS %d; want %d, %d, %d, %d",
			jpi(0), jpi(1), jpi(2), jpi(3), wc.Process, wc.Global, wc.Peak, p.PageFaults)
	}

	if on && wc.Peak > reclaimLimit || !on && wc.Peak < 80 {
		t.Errorf("checkpoint 3: peak %d", wc.Peak)
	}

	release(3)
}
