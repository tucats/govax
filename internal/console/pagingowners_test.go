package console

import (
	"testing"

	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// Phase 62's subtask 5 (docs/PHASE-62 - VMS paging.md): with paging on,
// every frame the system gives a page is recorded in the PFN database as
// that page's.

// frameOwnerCounts is how many tied frames frameOwners found, by kind.
type frameOwnerCounts struct {
	process, global int
}

// frameOwners checks the PFN database of c's memory (paging must be on):
// its own rules (Check), then that each tied frame is its owner's. A
// process page's frame is owned by a process that exists, whose PTE for
// the page is valid and holds the frame; a global page's is a page of a
// global section that holds the frame at that offset.
func frameOwners(t *testing.T, c *Console) frameOwnerCounts {
	t.Helper()

	db := c.Mem.Paging()
	if db == nil {
		t.Fatal("paging is off")
	}

	if err := db.Check(); err != nil {
		t.Fatalf("the PFN database: %v", err)
	}

	var counts frameOwnerCounts

	for n := range db.Frames() {
		e, err := db.Entry(paging.PFN(n))
		if err != nil {
			t.Fatal(err)
		}

		if !e.Tied || e.State != paging.Active {
			continue
		}

		switch e.Type {
		case paging.ProcessPage:
			counts.process++

			checkProcessFrame(t, c, uint32(n), e.Owner)

		case paging.GlobalPage:
			counts.global++

			if !globalFrameFound(c, uint32(n), e.Owner) {
				t.Errorf("frame %d: no global section holds it as page %d", n, e.Owner.VA/512)
			}

		case paging.SystemPage:
			t.Errorf("frame %d: a system page tied to %+v", n, e.Owner)
		}
	}

	return counts
}

// checkProcessFrame checks that frame pfn is the frame of owner's page:
// the process exists, and its PTE for the page is valid and holds pfn.
func checkProcessFrame(t *testing.T, c *Console, pfn uint32, owner paging.Owner) {
	t.Helper()

	for _, env := range c.RTL.Processes() {
		if env.Process == nil || env.Process.PID != owner.Handle {
			continue
		}

		space := vm.CurrentAddressSpace(c.CPU)
		if env.Space != nil {
			space = env.Space.AddressSpace
		}

		pte, err := c.Mem.LookupPTEIn(c.CPU, space, owner.VA)
		if err != nil || !pte.Valid() || pte.PFN() != pfn {
			t.Errorf("frame %d: PID %08X's page %08X has PTE %08X (%v)", pfn, owner.Handle, owner.VA, uint32(pte), err)
		}

		return
	}

	t.Errorf("frame %d: tied to PID %08X, which doesn't exist", pfn, owner.Handle)
}

// globalFrameFound reports whether a global section holds frame pfn as
// its page at owner's offset.
func globalFrameFound(c *Console, pfn uint32, owner paging.Owner) bool {
	i := int(owner.VA / 512)

	for _, s := range c.RTL.Sections.Sections() {
		if i < len(s.Frames) && s.Frames[i] == pfn {
			return true
		}
	}

	return false
}

// TestPagingGlobalSectionFrames: with paging on, a page-file global
// section's pages have no frames until they're touched (Phase 65's
// subtask 4: global demand-zero pages); then each has one frame, a global
// page backed by the page file and effectively modified, however many
// mappings share it; and the frames go back to the free list with the
// section.
func TestPagingGlobalSectionFrames(t *testing.T) {
	withSettings(t, map[string]string{machine.PagingSetting: "true"})

	c := newServiceConsole(t)
	db := c.Mem.Paging()

	if db == nil {
		t.Fatal("VMINIT left paging off")
	}

	before := frameOwners(t, c)

	if got := crmpsc(t, c, "PAGED", pageFile, secPages, secPages+0x3FF, 2, 0); got != ssCreated {
		t.Fatalf("$CRMPSC = %#x, want SS$_CREATED", got)
	}

	if got := mgblsc(t, c, "PAGED", secWRT, secPages+0x1000, secPages+0x13FF, 0, 0); got != ssNormal {
		t.Fatalf("$MGBLSC = %#x", got)
	}

	if after := frameOwners(t, c); after.global != before.global {
		t.Errorf("global frames %d, then %d after mapping; want none more", before.global, after.global)
	}

	for _, va := range []uint32{secPages, secPages + 0x200, secPages + 0x1000, secPages + 0x1200} {
		c.Mem.BeginInstruction()

		if v, err := c.Mem.LoadLongword(c.CPU, va); err != nil || v != 0 {
			t.Errorf("section page at %08X holds %d (%v), want 0", va, v, err)
		}
	}

	after := frameOwners(t, c)
	if after.global != before.global+2 {
		t.Errorf("global frames %d, then %d after the touches; want 2 more", before.global, after.global)
	}

	s := section(c, "PAGED")
	if s == nil {
		t.Fatal("no section PAGED")
	}

	for i, pfn := range s.Frames {
		e, _ := db.Entry(paging.PFN(pfn))
		if e.Type != paging.GlobalPage || e.Backing.Kind != paging.PageFile || e.Owner.VA != uint32(i)*512 ||
			e.ShareCount != 2 || !e.SavedModify {
			t.Errorf("page %d's frame %d: %+v", i, pfn, e)
		}
	}

	frames := s.Frames

	for _, r := range [][2]uint32{{secPages, secPages + 0x3FF}, {secPages + 0x1000, secPages + 0x13FF}} {
		putRange(t, c, vaInadr, r[0], r[1])

		if got := callService(t, c, "SYS$DELTVA", vaInadr, vaRetadr, 0); got != ssNormal {
			t.Fatalf("$DELTVA = %#x", got)
		}
	}

	if section(c, "PAGED") != nil {
		t.Fatal("the section outlived its mappings")
	}

	// Gone by name, the section waits to be released (testdata/pg65's
	// G2), as image rundown or the next section's creation does.
	if p := c.RTL.Sections.PendingSections(); len(p) != 1 || p[0].Name != "PAGED" {
		t.Fatalf("pending %v, want PAGED", p)
	}

	c.RTL.ReleasePendingSections()

	for _, pfn := range frames {
		if e, _ := db.Entry(paging.PFN(pfn)); e.State != paging.Free || e.Tied {
			t.Errorf("frame %d after the section: %+v", pfn, e)
		}
	}

	if got := frameOwners(t, c); got.global != before.global {
		t.Errorf("global frames %d after the section, want %d", got.global, before.global)
	}
}
