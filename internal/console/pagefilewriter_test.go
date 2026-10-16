package console_test

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// Phase 64's subtask 3 (docs/PHASE-64 - page file.md): with a page file
// configured, the modified page writer writes demand-zero pages from the
// modified list to it, in clusters, opening it at the first instruction
// boundary after it's wanted; a written page's frame goes to the free
// list with its block as its backing store; when the frame is reused the
// page's PTE takes the page file form, and the page's next touch reads
// it back from the page file; and a page read back and modified again
// gives its block up, so the file holds no more blocks than pages.

// pfPages is how many pages pfSource's array has.
const pfPages = 80

// pfSource fills each of the 80 pages of an array with its number plus
// 5001, in its first and last longwords (checkpoint 1); reads them back,
// counting in R9 the pages that don't hold their values (checkpoint 2);
// adds 100 to both longwords of each page (checkpoint 3); and checks them
// again, counting in R9 (checkpoint 4). At each checkpoint it waits in a
// loop until the test sets R10 to the checkpoint's number, R6. R8 is the
// array's address.
const pfSource = `	.title	wspf
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.macro	check,base,?l1,?l2,?l3
	moval	buf,r2
	clrl	r9
	movl	#base,r4
	movl	#80,r3
l1:	cmpl	(r2),r4
	bneq	l2
	cmpl	508(r2),r4
	beql	l3
l2:	incl	r9
l3:	incl	r4
	addl2	#512,r2
	sobgtr	r3,l1
	.endm	check
	.psect	data,noexe,wrt,page
buf:	.blkb	512*80
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	moval	buf,r8
	moval	buf,r2
	movl	#5001,r4
	movl	#80,r3
10$:	movl	r4,(r2)
	movl	r4,508(r2)
	incl	r4
	addl2	#512,r2
	sobgtr	r3,10$
	stop	1
	check	5001
	stop	2
	moval	buf,r2
	movl	#80,r3
30$:	addl2	#100,(r2)
	addl2	#100,508(r2)
	addl2	#512,r2
	sobgtr	r3,30$
	stop	3
	check	5101
	stop	4
	movl	#1,r0
	ret
	.end	start
`

// TestPageFileWriter runs pfSource with a 30-page working set, the
// writer's thresholds at 10 and 2, clusters of up to 16 pages, and a
// host page file, and checks at each checkpoint where the array's pages
// are and what the page file holds.
func TestPageFileWriter(t *testing.T) {
	pagefile := filepath.Join(t.TempDir(), "pagefile.sys")

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	setSetting(t, "vax.sysgen.mpw_hilimit", "10")
	setSetting(t, "vax.sysgen.mpw_lolimit", "2")
	setSetting(t, machine.WriteClusterSetting, "16")
	setSetting(t, "vax.sysgen.paging.file", `"`+pagefile+`"`)
	setSetting(t, "vax.sysgen.paging.size", "1000")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	db := c.Mem.Paging()
	if db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	exe := buildImage(t, c, "wspf", pfSource)

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

	// written returns the array's pages in transition whose frames are
	// on the free list, by their page file blocks, checking that each
	// block holds the page's contents (its first and last longwords
	// value plus its page number).
	written := func(label string, array, value uint32) map[int]uint32 {
		t.Helper()

		pf := c.RTL.PageFile()
		blocks := map[int]uint32{}
		buf := make([]byte, 512)

		for i := range pfPages {
			pfn, ok := pteIn(t, c, array+uint32(i)*512).Transition()
			if !ok {
				continue
			}

			e, err := db.Entry(paging.PFN(pfn))
			if err != nil || e.State != paging.Free {
				continue
			}

			if e.Backing.Kind != paging.PageFile || e.Backing.Block == 0 {
				t.Errorf("%s: page %d's frame is on the free list with backing %+v", label, i, e.Backing)

				continue
			}

			if err := pf.Read(e.Backing.Block, buf); err != nil {
				t.Fatalf("%s: reading page file block %d: %v", label, e.Backing.Block, err)
			}

			want := value + uint32(i)
			if a, b := binary.LittleEndian.Uint32(buf), binary.LittleEndian.Uint32(buf[508:]); a != want || b != want {
				t.Errorf("%s: page %d's block %d holds %d and %d, want %d", label, i, e.Backing.Block, a, b, want)
			}

			blocks[i] = e.Backing.Block
		}

		return blocks
	}

	// Checkpoint 1: the page file is open, and the writer has written
	// most of the array's pages that left the working set, in clusters:
	// fewer writes than pages.
	checkpoint(1)

	pf := c.RTL.PageFile()
	if pf == nil {
		t.Fatalf("checkpoint 1: no page file open\n%s", out.String())
	}

	array := c.CPU.GPR(8)
	blocks := written("checkpoint 1", array, 5001)

	if len(blocks) < pfPages-wsLimit-10 {
		t.Errorf("checkpoint 1: %d of the array's pages written to the page file, want at least %d", len(blocks), pfPages-wsLimit-10)
	}

	counts := c.RTL.PageCounts
	if counts.Written < uint64(len(blocks)) || counts.WriteIOs == 0 || counts.WriteIOs*2 > counts.Written {
		t.Errorf("checkpoint 1: %d pages written in %d writes, want clusters", counts.Written, counts.WriteIOs)
	}

	if used := pf.Map.UsedBlocks(); used < uint32(len(blocks)) || used > pfPages {
		t.Errorf("checkpoint 1: %d page file blocks in use for %d pages written", used, len(blocks))
	}

	if err := pf.Map.Check(); err != nil {
		t.Fatal(err)
	}

	// A cluster's blocks follow its pages' addresses: page i+1's block is
	// page i's plus 1 for most neighbors.
	follow := 0

	for i, b := range blocks {
		if next, ok := blocks[i+1]; ok && next == b+1 {
			follow++
		}
	}

	if follow < len(blocks)/2 {
		t.Errorf("checkpoint 1: only %d of %d written pages have their neighbors in the next block", follow, len(blocks))
	}

	// Take every free frame, so each written page's frame is reused and
	// its PTE takes the page file form, with its block.
	stealFreeFrames(t, c)

	for i, b := range blocks {
		if got, ok := pteIn(t, c, array+uint32(i)*512).PageFileBlock(); !ok || got != b {
			t.Errorf("page %d's PTE after its frame was reused: block %d (%v), want %d", i, got, ok, b)
		}
	}

	// Checkpoint 2: every page reads back from the page file with its
	// values, by hard faults.
	reads := c.ProcessOne().Process.Faults.Read

	release(1)
	checkpoint(2)

	if got := c.CPU.GPR(9); got != 0 {
		t.Errorf("checkpoint 2: %d of the array's pages read back wrong", got)
	}

	if got := c.ProcessOne().Process.Faults.Read - reads; got < uint64(len(blocks)) {
		t.Errorf("checkpoint 2: %d hard faults, want at least the %d pages read from the page file", got, len(blocks))
	}

	// Checkpoint 3: every page modified again; those written have new
	// blocks, and the file holds no more blocks than there are pages.
	release(2)
	checkpoint(3)

	written("checkpoint 3", array, 5101)

	if used := pf.Map.UsedBlocks(); used > pfPages {
		t.Errorf("checkpoint 3: %d page file blocks in use for %d pages", used, pfPages)
	}

	if err := pf.Map.Check(); err != nil {
		t.Fatal(err)
	}

	// Checkpoint 4: the new values read back, after the frames are
	// reused once more.
	stealFreeFrames(t, c)
	release(3)
	checkpoint(4)

	if got := c.CPU.GPR(9); got != 0 {
		t.Errorf("checkpoint 4: %d of the array's pages read back wrong", got)
	}

	release(4)

	for range 400_000 {
		if err := c.StepMachine(); err != nil {
			if err := c.ReportStop(err); err != nil {
				t.Fatalf("at PC %08X: %v", c.CPU.GPR(registers.PC), err)
			}

			break
		}
	}

	if c.ImageActive() {
		t.Fatal("the image didn't end")
	}

	checkWorkingSets(t, c)
}

// TestPageFileWriterNoFile: with paging on and no page file configured,
// demand-zero pages stay on the modified list, as before the page file,
// and nothing is opened.
func TestPageFileWriterNoFile(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	setSetting(t, "vax.sysgen.mpw_hilimit", "10")
	setSetting(t, "vax.sysgen.mpw_lolimit", "2")
	setSetting(t, "vax.sysgen.paging.file", "")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	exe := buildImage(t, c, "wspf", pfSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == 1 })
	checkWorkingSets(t, c)

	if c.RTL.PageFile() != nil {
		t.Error("a page file is open with none configured")
	}

	if n := c.Mem.Paging().ModifiedCount(); n < pfPages-wsLimit {
		t.Errorf("%d frames on the modified list, want at least %d", n, pfPages-wsLimit)
	}
}

// stealFreeFrames takes every frame on the free list and gives each
// back, so that each is reused: the page each still held loses it, and
// its PTE goes back to its backing store's form.
func stealFreeFrames(t *testing.T, c *console.Console) {
	t.Helper()

	db := c.Mem.Paging()

	var taken []uint32

	for db.FreeCount() > 0 {
		pfn, ok := c.Mem.AllocateFrame(vm.FrameClaim{})
		if !ok {
			t.Fatal("AllocateFrame failed with frames free")
		}

		taken = append(taken, pfn)
	}

	for _, pfn := range taken {
		c.Mem.FreePage(pfn)
	}
}
