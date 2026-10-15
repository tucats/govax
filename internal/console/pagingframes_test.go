package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
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
}
