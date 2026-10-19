package console

import (
	"io"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vm"
)

// Phase 67's subtask 4 (docs/PHASE-67 - address space under paging.md):
// with paging on, a process's two page tables are one table area, P0's
// growing up from its bottom and P1's down from its top, and only the
// area's pages that hold entries of pages inside the regions' lengths
// have frames.

// areaPages checks that each page of sp's table area has a frame exactly
// when it holds the entry of a page inside a region's length, and
// returns how many pages the area has and how many of them have frames.
func areaPages(t *testing.T, c *Console, label string, sp *corevms.ProcessSpace) (pages, backed int) {
	t.Helper()

	end := sp.P1BR + 0x200000*4

	for va := sp.P0BR &^ 511; va < end; va += 512 {
		need := sp.P0LR > 0 && va <= (sp.P0BR+sp.P0LR*4-1)&^511 ||
			sp.P1LR < 0x200000 && va >= (sp.P1BR+sp.P1LR*4)&^511

		_, _, pte, err := c.Mem.LookupPTE(c.CPU, va)
		if err != nil {
			t.Fatalf("%s: the S0 PTE of table page %08X: %v", label, va, err)
		}

		if pte.Valid() != need {
			t.Errorf("%s: table page %08X has a frame %v, want %v (P0LR %#x, P1LR %#x)",
				label, va, pte.Valid(), need, sp.P0LR, sp.P1LR)
		}

		if !pte.Valid() && pte != 0 {
			t.Errorf("%s: table page %08X without a frame has PTE %08X, want 0", label, va, uint32(pte))
		}

		pages++

		if pte.Valid() {
			backed++
		}
	}

	return pages, backed
}

// TestTableAreaOnDemand: process 1's table area (VMINIT's /P0=2048
// /P1=8192, 80 pages) starts with frames behind only the pages that
// hold its P0 pages' and P1's permanent part's entries; a region growing
// across a table page gives it a frame, and shrinking gives it back; and
// P0 may grow into the room P1 doesn't use, past VMINIT's /P0, up to the
// area's end, where P0 and P1 both meet SS$_VASFULL; and P1 into P0's,
// past /P1.
func TestTableAreaOnDemand(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	env := c.ProcessOne()
	sp := env.Space
	db := c.Mem.Paging()

	if db == nil {
		t.Fatal("VMINIT left paging off")
	}

	if sp.Entries != 2048+8192 {
		t.Fatalf("process 1's area has %d entries, want /P0 + /P1", sp.Entries)
	}

	// The scratch page (P0 page 0x40), then P1's 4096 pages from the user
	// stack's top up: one table page and 32.
	putRange(t, c, vaInadr, 200*512, 200*512+511)

	if pages, backed := areaPages(t, c, "at the start", sp); pages != 80 || backed != 1+32 {
		t.Errorf("at the start: %d of %d table pages backed, want 33 of 80", backed, pages)
	}

	if got := jpiLong(t, env, "JPI$_FREPTECNT"); got != sp.Entries-sp.P0LR-4096 {
		t.Errorf("FREPTECNT %d, want %d", got, sp.Entries-sp.P0LR-4096)
	}

	// P0 page 200's entry is in the area's second page.
	system := db.Counts().System

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$CRETVA of page 200 = %#x", got)
	}

	lengths(t, c, "after $CRETVA of page 200", 201, stackTopPage)

	if _, backed := areaPages(t, c, "after $CRETVA of page 200", sp); backed != 2+32 {
		t.Errorf("after $CRETVA of page 200: %d table pages backed, want 34", backed)
	}

	if got := db.Counts().System; got != system+1 {
		t.Errorf("system frames %d after $CRETVA, want %d: the table page's", got, system+1)
	}

	if got := callService(t, c, "SYS$DELTVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$DELTVA of page 200 = %#x", got)
	}

	if _, backed := areaPages(t, c, "after $DELTVA of page 200", sp); backed != 1+32 {
		t.Errorf("after $DELTVA of page 200: %d table pages backed, want 33", backed)
	}

	if got := db.Counts().System; got != system {
		t.Errorf("system frames %d after $DELTVA, want %d", got, system)
	}

	// P0's last page with P1 as it is: 10240 - 4096 - 1, past /P0.
	last := sp.Entries - 4096 - 1

	putRange(t, c, vaInadr, last*512, last*512+511)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$CRETVA of P0's last page = %#x", got)
	}

	lengths(t, c, "at the area's end", last+1, stackTopPage)

	if pages, backed := areaPages(t, c, "at the area's end", sp); backed != pages {
		t.Errorf("at the area's end: %d of %d table pages backed, want all", backed, pages)
	}

	if got := jpiLong(t, env, "JPI$_FREPTECNT"); got != 0 {
		t.Errorf("FREPTECNT %d at the area's end, want 0", got)
	}

	if err := storeLong(c, last*512, 5); err != nil {
		t.Errorf("a store to P0's last page: %v", err)
	}

	putRange(t, c, vaInadr, (last+1)*512, (last+1)*512+511)

	if got := callService(t, c, "SYS$CRETVA", vaInadr, vaRetadr, 0); got != ssVasFull {
		t.Errorf("$CRETVA past the area's end = %#x, want SS$_VASFULL", got)
	}

	if got := callService(t, c, "SYS$EXPREG", 1, vaRetadr, 0, 1); got != ssVasFull {
		t.Errorf("$EXPREG of P1 with the area full = %#x, want SS$_VASFULL", got)
	}

	// Deleting P0's last page gives every table page between back.
	putRange(t, c, vaInadr, last*512, last*512+511)

	if got := callService(t, c, "SYS$DELTVA", vaInadr, vaRetadr, 0); got != ssNormal {
		t.Fatalf("$DELTVA of P0's last page = %#x", got)
	}

	if _, backed := areaPages(t, c, "after $DELTVA of P0's last page", sp); backed != 1+32 {
		t.Errorf("after $DELTVA of P0's last page: %d table pages backed, want 33", backed)
	}

	// P1 grows down into P0's room: 9096 pages, past VMINIT's /P1, their
	// entries in the area's last 72 pages (71 and part of another).
	if got := callService(t, c, "SYS$EXPREG", 5000, vaRetadr, 0, 1); got != ssNormal {
		t.Fatalf("$EXPREG of 5000 P1 pages = %#x", got)
	}

	lengths(t, c, "after $EXPREG of P1", sp.P0LR, stackTopPage-5000)

	if _, backed := areaPages(t, c, "after $EXPREG of P1", sp); backed != 1+72 {
		t.Errorf("after $EXPREG of P1: %d table pages backed, want 73", backed)
	}

	if got := callService(t, c, "SYS$CNTREG", 5000, vaRetadr, 0, 1); got != ssNormal {
		t.Fatalf("$CNTREG of P1 = %#x", got)
	}

	if _, backed := areaPages(t, c, "after $CNTREG of P1", sp); backed != 1+32 {
		t.Errorf("after $CNTREG of P1: %d table pages backed, want 33", backed)
	}

	// Image rundown deletes P0: no P0 table page keeps a frame.
	env.DeleteP0()

	if _, backed := areaPages(t, c, "after DeleteP0", sp); backed != 32 {
		t.Errorf("after DeleteP0: %d table pages backed, want P1's 32", backed)
	}

	if err := db.Check(); err != nil {
		t.Error(err)
	}
}

// TestTableAreaNewProcess: with paging on, a new process's table area
// has frames behind only P1's permanent part's pages; its pages grow it
// as process 1's do; and teardown gives every frame back, the area's
// pages left without, until the pool hands them out again and
// AllocateS0 backs them.
func TestTableAreaNewProcess(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	sys := c.RTL.System
	db := c.Mem.Paging()

	env, err := corevms.NewEnvironment(sys, c.Logicals, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	pid := env.Process.PID

	if env.Space, err = sys.BuildAddressSpace(pid, 64, 8192); err != nil {
		t.Fatal(err)
	}

	sp := env.Space

	// One page for P0's 64 entries and 64 for P1's 8192: P1's top 4096
	// pages exist, their entries in the area's last 32 pages.
	if pages, backed := areaPages(t, c, "a new space", sp); pages != 65 || backed != 32 {
		t.Errorf("a new space: %d of %d table pages backed, want 32 of 65", backed, pages)
	}

	if sp.P0BR != sp.Table || sp.P1BR+0x200000*4 != sp.Table+65*512 {
		t.Errorf("the area at %08X: P0BR %08X, P1BR %08X", sp.Table, sp.P0BR, sp.P1BR)
	}

	// The P1 vector is mapped, as before.
	vec := sp.P1BR + (0x7FFEDE00-0x40000000)/512*4
	if pte, err := c.Mem.LoadLongword(c.CPU, vec); err != nil || !vm.PTE(pte).Valid() {
		t.Errorf("the P1 vector's PTE %08X: %v", pte, err)
	}

	// P0 may grow to the area's 8256 entries less P1's 4096.
	if err := env.EnsurePages(0x200, 3*512); err != nil {
		t.Fatal(err)
	}

	if _, backed := areaPages(t, c, "three P0 pages", sp); backed != 1+32 {
		t.Errorf("three P0 pages: %d table pages backed, want 33", backed)
	}

	if err := env.EnsurePages((64+4096-1)*512, 512); err != nil {
		t.Errorf("P0's last page, past its table's first 64 entries: %v", err)
	}

	if err := env.EnsurePages((64+4096)*512, 512); err == nil {
		t.Error("a P0 page past the area's end was created")
	}

	if pages, backed := areaPages(t, c, "P0 at the area's end", sp); backed != pages {
		t.Errorf("P0 at the area's end: %d of %d table pages backed, want all", backed, pages)
	}

	free := db.Counts().Free
	table := sp.Table

	if err := sys.TeardownAddressSpace(sp); err != nil {
		t.Fatal(err)
	}

	// The 65 table pages' frames, and the P0 pages' charged but never
	// touched, which had none.
	if got := db.Counts().Free; got != free+65 {
		t.Errorf("free frames %d after teardown, want %d", got, free+65)
	}

	for va := table; va < table+65*512; va += 512 {
		if _, _, pte, err := c.Mem.LookupPTE(c.CPU, va); err != nil || pte != 0 {
			t.Errorf("table page %08X after teardown: PTE %08X (%v), want 0", va, uint32(pte), err)
		}
	}

	// The pool hands the pages out again, each with a cleared frame.
	addr, err := sys.AllocateS0(65, pid, "test")
	if err != nil {
		t.Fatal(err)
	}

	if addr != table {
		t.Fatalf("AllocateS0 gave %08X, want the old table area's %08X", addr, table)
	}

	for va := addr; va < addr+65*512; va += 512 {
		if v, err := c.Mem.LoadLongword(c.CPU, va+508); err != nil || v != 0 {
			t.Errorf("pool page %08X: %08X (%v), want a cleared page", va, v, err)
		}
	}

	if err := db.Check(); err != nil {
		t.Error(err)
	}
}
