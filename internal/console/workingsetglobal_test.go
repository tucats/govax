package console

import (
	"testing"

	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
)

// Phase 63's subtask 3 (docs/PHASE-63 - working sets.md), with paging
// on: the P1 vector's pages are system pages, in no working set (the
// paging program's Decision 13); a page-file global section's pages enter
// the working set when they're mapped, as global pages, and stay while
// the process's own pages come and go (they're resident until Phase 65);
// and they leave it when they're unmapped.
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

	if got := ws.Counts().Global; got != before+4 {
		t.Errorf("%d global pages in the working set after two mappings, want %d", got, before+4)
	}

	for _, va := range mapped {
		if p, in := ws.Lookup(va); !in || !p.Global {
			t.Errorf("mapped page %08X: in the working set %v, as %+v", va, in, p)
		}
	}

	// Touch 60 pages of P0, an instruction at a time, twice the working
	// set's limit: process 1's own pages leave it, the section's don't.
	for i := range uint32(60) {
		c.Mem.BeginInstruction()

		if err := storeLong(c, 0x60000+i*512, i); err != nil {
			t.Fatal(err)
		}
	}

	c.Mem.BeginInstruction() // the boundary brings a list over its limit back down

	if counts := ws.Counts(); counts.Entries != 30 || counts.Global != before+4 {
		t.Errorf("after the touches, working set counts %+v; want 30 entries, %d global pages", counts, before+4)
	}

	for _, va := range mapped {
		if _, in := ws.Lookup(va); !in || !pteAt(t, c, va).Valid() {
			t.Errorf("mapped page %08X left the working set (or its PTE is invalid)", va)
		}
	}

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
