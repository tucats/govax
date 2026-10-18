package console

import (
	"testing"

	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
)

// Phase 63's subtask 3 (docs/PHASE-63 - working sets.md), with paging
// on: the P1 vector's pages are system pages, in no working set (the
// paging program's Decision 13). Since Phase 65's subtask 4, a page-file
// global section's pages are global demand-zero pages: they enter the
// working set when they're first touched, as global pages, not when
// they're mapped; they leave it as the process's own pages come and go,
// their PTEs holding their global page table indexes again, and come
// back with their contents; and they leave it when they're unmapped.
func TestWorkingSetGlobalPages(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true", "vax.sysgen.pql_dwsdefault": "30"})

	c := newServiceConsole(t)
	db := c.Mem.Paging()

	if db == nil {
		t.Fatal("VMINIT left paging off")
	}

	ws := c.RTL.WorkingSet()
	if ws.Size() != 30 {
		t.Fatalf("process 1's working set limit %d, want 30", ws.Size())
	}

	vector, ok := c.Symbols.Get("SYS$CRMPSC")
	if !ok {
		t.Fatal("no SYS$CRMPSC in the P1 vector")
	}

	pte := pteAt(t, c, vector)
	if e, err := db.Entry(paging.PFN(pte.PFN())); !pte.Valid() || err != nil || e.Type != paging.SystemPage || e.Tied {
		t.Errorf("the P1 vector's page %08X: PTE %08X, frame %+v (%v); want a valid system page", vector, uint32(pte), e, err)
	}

	if _, in := ws.Lookup(vector); in {
		t.Error("the P1 vector's page is in process 1's working set")
	}

	before := ws.Counts().Global

	if got := crmpsc(t, c, "PAGED", pageFile, secPages, secPages+0x3FF, 2, 0); got != ssCreated {
		t.Fatalf("$CRMPSC = %#x, want SS$_CREATED", got)
	}

	if got := mgblsc(t, c, "PAGED", secWRT, secPages+0x1000, secPages+0x13FF, 0, 0); got != ssNormal {
		t.Fatalf("$MGBLSC = %#x", got)
	}

	mapped := []uint32{secPages, secPages + 0x200, secPages + 0x1000, secPages + 0x1200}

	if got := ws.Counts().Global; got != before {
		t.Errorf("%d global pages in the working set after two mappings, want %d (none touched)", got, before)
	}

	for _, va := range mapped {
		if _, global := pteAt(t, c, va).GlobalIndex(); !global {
			t.Errorf("mapped page %08X: PTE %08X, want its global page table index", va, uint32(pteAt(t, c, va)))
		}
	}

	// Each page written through the first mapping and read through the
	// second: four global pages in the working set.
	for i, va := range mapped[:2] {
		c.Mem.BeginInstruction()

		if err := storeLong(c, va, uint32(i+100)); err != nil {
			t.Fatal(err)
		}
	}

	for i, va := range mapped[2:] {
		c.Mem.BeginInstruction()

		if v, err := c.Mem.LoadLongword(c.CPU, va); err != nil || v != uint32(i+100) {
			t.Errorf("section page %d through the second mapping holds %d (%v), want %d", i, v, err, i+100)
		}
	}

	c.Mem.BeginInstruction()

	for _, va := range mapped {
		if p, in := ws.Lookup(va); !in || !p.Global {
			t.Errorf("touched page %08X: in the working set %v, as %+v", va, in, p)
		}
	}

	if got := ws.Counts().Global; got != before+4 {
		t.Errorf("%d global pages in the working set after the touches, want %d", got, before+4)
	}

	// Touch 60 pages of P0, an instruction at a time, twice the working
	// set's limit: process 1's own pages leave it, and so do the
	// section's, each PTE holding its global page table index again.
	for i := range uint32(60) {
		c.Mem.BeginInstruction()

		if err := storeLong(c, 0x60000+i*512, i); err != nil {
			t.Fatal(err)
		}
	}

	c.Mem.BeginInstruction() // the boundary brings a list over its limit back down

	if counts := ws.Counts(); counts.Entries != 30 || counts.Global != before {
		t.Errorf("after the touches, working set counts %+v; want 30 entries, %d global pages", counts, before)
	}

	for _, va := range mapped {
		if _, global := pteAt(t, c, va).GlobalIndex(); !global {
			t.Errorf("mapped page %08X after the touches: PTE %08X, want its global page table index", va, uint32(pteAt(t, c, va)))
		}
	}

	// The section's pages, never written to a page file (there's none),
	// wait on the modified list, and come back with their contents.
	for i, va := range mapped {
		c.Mem.BeginInstruction()

		if v, err := c.Mem.LoadLongword(c.CPU, va); err != nil || v != uint32(i%2+100) {
			t.Errorf("section page %d at %08X holds %d (%v), want %d", i%2, va, v, err, i%2+100)
		}
	}

	c.Mem.BeginInstruction()

	if _, ok := pteAt(t, c, 0x60000).Transition(); !ok {
		t.Errorf("the first page touched has PTE %08X, want it in transition", uint32(pteAt(t, c, 0x60000)))
	}

	if err := db.Check(); err != nil {
		t.Fatal(err)
	}

	for _, r := range [][2]uint32{{secPages, secPages + 0x3FF}, {secPages + 0x1000, secPages + 0x13FF}} {
		putRange(t, c, vaInadr, r[0], r[1])

		if got := callService(t, c, "SYS$DELTVA", vaInadr, vaRetadr, 0); got != ssNormal {
			t.Fatalf("$DELTVA = %#x", got)
		}
	}

	if got := ws.Counts().Global; got != before {
		t.Errorf("%d global pages in the working set after the deletions, want %d", got, before)
	}

	if err := ws.Check(); err != nil {
		t.Fatal(err)
	}
}
