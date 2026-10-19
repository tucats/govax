package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
)

// pagedImageSource touches eight pages of its own data, creates four
// more with $EXPREG, writes one, deletes them with $DELTVA, and says so.
const pagedImageSource = `	.title	paged
	.psect	data,noexe,wrt
buf:	.blkb	512*8
range:	.long	0,0
msg:	.ascid	/paged image done/
	.psect	code,exe,nowrt
	.entry	start,^m<>
	movab	buf,r2
	movl	#8,r3
10$:	movl	r3,(r2)
	addl2	#512,r2
	sobgtr	r3,10$
	$expreg_s pagcnt=#4,retadr=range
	blbc	r0,90$
	movl	range,r2
	movl	#1,(r2)
	$deltva_s inadr=range
	blbc	r0,90$
	pushaq	msg
	calls	#1,g^lib$put_output
	movl	#1,r0
90$:	ret
	.end	start
`

// TestPagingFrames: with vax.sysgen.paging.enable on, VMINIT gives the
// memory the PFN database (docs/PHASE-62 - VMS paging.md, subtask 4), a
// program runs as it does without it, and the database is sound
// afterward, its count of claimed frames the memory's.
func TestPagingFrames(t *testing.T) {
	setSetting(t, "vax.sysgen.paging.enable", "true")

	c, out := scheduledConsole(t, longQuantum, brbSelf)

	db := c.Mem.Paging()
	if !c.Paging() || db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	if err := db.Check(); err != nil {
		t.Fatalf("after vax.init: %v", err)
	}

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	runImage(t, d, buildImage(t, c, "paged", pagedImageSource))

	if !strings.Contains(out.String(), "paged image done") {
		t.Errorf("the image's output:\n%s", out.String())
	}

	if err := db.Check(); err != nil {
		t.Fatalf("after the image: %v", err)
	}

	counts := db.Counts()
	if got := c.Mem.MappedPages(); got != counts.Active()-1 {
		t.Errorf("MappedPages = %d, the database's active frames but frame 0 = %d", got, counts.Active()-1)
	}

	if counts.Free+counts.Active() != db.Frames() || counts.Modified != 0 {
		t.Errorf("counts %+v over %d frames", counts, db.Frames())
	}

	// Subtask 5: the image's pages were process 1's, each frame recorded
	// as its page's, and they went with the image: since Phase 67's
	// subtask 3 its user stack does too, so no frame is left tied to a
	// process page.
	if process, _ := console.FrameOwners(t, c); process != 0 {
		t.Errorf("%d frames still tied to process pages after the image", process)
	}
}

// TestPagingFileSections runs Phase 49's file section programs with
// paging on (docs/PHASE-62 - VMS paging.md, subtask 5): a global section
// two processes share, and private and copy-on-reference sections left
// for image rundown. Each still ends as it should, and afterward no
// frame is left tied to a global page; since Phase 67's subtask 3 image
// rundown takes the user stack too, so none is left tied to a process
// page either.
func TestPagingFileSections(t *testing.T) {
	setSetting(t, "vax.sysgen.paging.enable", "true")

	for _, image := range []string{"SECPARENT", "RUNDOWN"} {
		t.Run(image, func(t *testing.T) {
			c := sectionMachine(t, longQuantum, "secchild", "secparent", "rundown")
			if c.Mem.Paging() == nil {
				t.Fatal("VMINIT left paging off")
			}

			if err := c.Run("DUA0:[000000]"+image+".EXE", machine.RunOptions{}); err != nil {
				t.Fatalf("RUN: %v", err)
			}

			if got := c.CPU.GPR(registers.R0); got != 999 {
				t.Fatalf("R0 = %d, want 999 (another value is the step that failed)", got)
			}

			if process, global := console.FrameOwners(t, c); process != 0 || global != 0 {
				t.Errorf("tied frames: %d process, %d global; want none", process, global)
			}
		})
	}
}
