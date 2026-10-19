package console

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// freePTEs is how many more pages process 1's table area can describe:
// FREPTECNT, its entries less both regions' pages.
func freePTEs(c *Console) uint32 {
	sp := c.ProcessOne().Space

	return sp.Entries - sp.P0LR - (0x200000 - sp.P1LR)
}

// TestCreationNeedsRoom (Phase 67's subtask 9): with paging on, a P1
// creation the table area can't hold creates nothing, as VMS 7.1's did
// (testdata/pg67's A12 and A13). A user-mode reference further below
// P1's low end than FREPTECNT pages is the length violation it was, P1
// as long as before (where creating page by page would have spent the
// room P0 needs: testdata/lib59's references to 0x55555555); $CRETVA of
// more P1 pages than FREPTECNT is SS$_VASFULL with retadr -1. In P0 the
// same $CRETVA makes its pages one at a time, to the area's end, then
// SS$_VASFULL; and the paging file quota stops a creation partway in
// either.
func TestCreationNeedsRoom(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	env := c.ProcessOne()
	top := uint32(corevms.UserStackTop)

	if err := storeLong(c, vaInadr, 0); err != nil { // creates the scratch pages
		t.Fatal(err)
	}

	p0 := c.CPU.PR(registers.P0LR)
	free := freePTEs(c)
	charged := env.Process.Job.PageFileCharged

	restore := inMode(c, registers.User)

	if m := faultMask(t, c, top-(free+10)*512, vm.AccessRead); m != vm.MaskLength {
		t.Errorf("a user-mode read FREPTECNT+10 pages below the stack: mask %d, want 1", m)
	}

	if m := faultMask(t, c, 0x55555555, vm.AccessRead); m != vm.MaskLength {
		t.Errorf("a user-mode read of 0x55555555: mask %d, want 1", m)
	}

	restore()

	lengths(t, c, "after the reads far below P1", p0, stackTopPage)

	if got := env.Process.Job.PageFileCharged; got != charged {
		t.Errorf("%d pages charged after the reads, %d before", got, charged)
	}

	// $CRETVA below P1's low end, one page more than the area holds.
	low := top - (free+1)*512
	putRange(t, c, vaInadr, low, top-1)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 0); got != vmsdef.Symbols["SS$_VASFULL"] {
		t.Errorf("$CRETVA of FREPTECNT+1 P1 pages = %#x, want SS$_VASFULL", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != 0xFFFFFFFF || b != 0xFFFFFFFF {
		t.Errorf("retadr %#x-%#x, want nothing created", a, b)
	}

	lengths(t, c, "after the refused P1 $CRETVA", p0, stackTopPage)

	if got := freePTEs(c); got != free {
		t.Errorf("FREPTECNT %d, want %d as before", got, free)
	}

	// The same from P0's end: every page the area holds is made, then
	// SS$_VASFULL.
	end := p0 * 512
	putRange(t, c, vaInadr, end, end+(free+1)*512-1)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 0); got != vmsdef.Symbols["SS$_VASFULL"] {
		t.Errorf("$CRETVA of FREPTECNT+1 P0 pages = %#x, want SS$_VASFULL", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != end || b != end+free*512-1 {
		t.Errorf("retadr %#x-%#x, want the %d pages the area holds, %#x-%#x", a, b, free, end, end+free*512-1)
	}

	lengths(t, c, "after the P0 $CRETVA", p0+free, stackTopPage)

	putRange(t, c, vaInadr, end, end+free*512-1)

	if got := callService(t, c, "SYS$DELTVA", vaInadr); got != ssNormal {
		t.Fatalf("$DELTVA of them = %#x", got)
	}

	lengths(t, c, "after they're deleted", p0, stackTopPage)

	// Within the room the quota is met page by page: three of ten made.
	job := env.Process.Job
	job.Pooled.PGFLQUOTA = job.PageFileCharged + 3

	putRange(t, c, vaInadr, end, end+10*512-1)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 0); got != vmsdef.Symbols["SS$_EXQUOTA"] {
		t.Errorf("$CRETVA of 10 pages with 3 of quota = %#x, want SS$_EXQUOTA", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != end || b != end+3*512-1 {
		t.Errorf("retadr %#x-%#x, want the three pages made, %#x-%#x", a, b, end, end+3*512-1)
	}

	lengths(t, c, "after the quota's three", p0+3, stackTopPage)
}
