package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/paging"
)

// Phase 66's subtask 5 (docs/PHASE-66 - mapped image activation.md):
// image reset. With paging on, an image's rundown deletes P0 as VMS's
// MMG$IMGRESET does: every page goes back to its fresh form, the frames
// the image's pages held are untied, its sections go with their last
// pages, and so does the hold on the image file, and the working set
// returns to its default. That needs no code of its own beyond
// DeleteP0; these tests check it on each way an image ends: by itself,
// abandoned (stopped and never resumed, then run down by the next RUN),
// and under the debugger, whose session keeps the image's pages until
// the next activation (KeepP0, ResetICBList).

// checkImageReset checks that process 1 holds nothing of an image: no
// process sections, every P0 page in its fresh form, no frame tied to a
// P0 page of the process, a working set of its default size with no P0
// page in it, the PFN database and working sets consistent, and the
// image's volume free to be dismounted (no file of it held), which it
// does, so it's a test's last check.
func checkImageReset(t *testing.T, c *console.Console, volume string) {
	t.Helper()

	env := c.ProcessOne()

	if got := env.ProcessSections(); got != 0 {
		t.Errorf("%d process sections after the reset, want none", got)
	}

	space := env.Space.AddressSpace
	for i := range space.P0LR {
		va := i * 512

		pte, err := c.Mem.LookupPTEIn(c.CPU, space, va)
		if err != nil {
			t.Fatalf("PTE of %08X: %v", va, err)
		}

		if pte != corevms.ProcessPTE(false, i) {
			t.Errorf("P0 page %08X has PTE %08X after the reset, want its fresh form", va, uint32(pte))

			break
		}
	}

	db := c.Mem.Paging()
	pid := env.Process.PID

	for pfn := range db.Frames() {
		e, err := db.Entry(paging.PFN(pfn))
		if err != nil {
			t.Fatal(err)
		}

		if e.Owner.Handle == pid && e.Owner.VA < 0x40000000 && e.Type == paging.ProcessPage {
			t.Errorf("frame %d is still tied to P0 page %08X after the reset: %+v", pfn, e.Owner.VA, e)
		}
	}

	ws := env.WorkingSet()
	if got, want := ws.Size(), int(env.Process.WSDefault); got != want {
		t.Errorf("working set limit %d after the reset, want WSDEFAULT's %d", got, want)
	}

	for _, p := range ws.Pages() {
		if p.VA < 0x40000000 {
			t.Errorf("P0 page %08X is still in the working set after the reset", p.VA)
		}
	}

	checkWorkingSets(t, c)

	if err := c.Mounts.Dismount(volume); err != nil {
		t.Errorf("Dismount after the reset: %v", err)
	}
}

// TestMappedImageReset: an image that runs to its end leaves nothing
// behind, and the page file quota it was charged is all back.
func TestMappedImageReset(t *testing.T) {
	c, d, name, _ := mappedImageConsole(t, "")

	// The harness's own code (scheduledConsole's spin loop) is a P0 page
	// the first RUN's reset deletes; the count starts after it.
	c.ResetICBList()
	before := c.ProcessOne().PageFileRemaining()

	startMappedImage(t, c, d, name)
	toCheckpoint2(t, c)
	finishMappedImage(t, c)

	checkImageReset(t, c, "DUA0")

	if got := c.ProcessOne().PageFileRemaining(); got != before {
		t.Errorf("page file quota remaining %d after the image, want %d as before it", got, before)
	}
}

// TestMappedImageResetAbandoned: an image stopped in the middle (here at
// checkpoint 2, its array's pages valid, in transition, and on the
// modified list) and never resumed is run down when the next RUN starts
// (EndAbandonedImage), with the same result as an image that exits.
func TestMappedImageResetAbandoned(t *testing.T) {
	c, d, name, _ := mappedImageConsole(t, "")

	startMappedImage(t, c, d, name)
	toCheckpoint2(t, c)

	if c.ProcessOne().ProcessSections() == 0 {
		t.Fatal("no process sections while the image runs")
	}

	c.EndAbandonedImage()

	if c.ImageActive() {
		t.Fatal("the image is still active after its rundown")
	}

	checkImageReset(t, c, "DUA0")
}

// TestMappedImageResetDebugger: an image that ends under the debugger
// keeps its pages, its sections, and so its file, for the session to
// examine (KeepP0); the volume can't be dismounted meanwhile. The next
// RUN deletes them before it activates its image (ResetICBList), so the
// same image runs again from its file, with only its own sections, and
// its end leaves nothing behind.
func TestMappedImageResetDebugger(t *testing.T) {
	c, d, name, out := mappedImageConsole(t, "")
	env := c.ProcessOne()

	if err := d.Dispatch("RUN/DEBUG " + name); err != nil {
		t.Fatalf("RUN/DEBUG %s: %v", name, err)
	}

	if !c.InDebugger() || !c.ImageActive() {
		t.Fatalf("RUN/DEBUG: session %v, image active %v; want both", c.InDebugger(), c.ImageActive())
	}

	runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == 1 })
	step(t, c, 4)

	sections := env.ProcessSections()
	if sections == 0 {
		t.Fatal("no process sections while the image runs")
	}

	finishMappedImage(t, c)

	if !c.InDebugger() {
		t.Fatal("the debugger session ended with the image")
	}

	if got := env.ProcessSections(); got != sections {
		t.Errorf("%d process sections after the image's end under the debugger, want its %d kept", got, sections)
	}

	if err := c.Mounts.Dismount("DUA0"); err == nil {
		t.Fatal("the volume of the image the debugger keeps was dismounted")
	}

	// Leave the session, then run the image again, to its end.
	if err := d.Dispatch("EXIT"); err != nil {
		t.Fatalf("EXIT: %v", err)
	}

	startMappedImage(t, c, d, name)

	if got := env.ProcessSections(); got > sections {
		t.Errorf("%d process sections in the second run, want at most the image's %d", got, sections)
	}

	finishMappedImage(t, c)

	if n := strings.Count(out.String(), "mapped hello"); n != 2 {
		t.Errorf("the image said hello %d times, want 2:\n%s", n, out.String())
	}

	checkImageReset(t, c, "DUA0")
}
