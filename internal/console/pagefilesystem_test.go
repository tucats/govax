package console_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
)

// Phase 64's subtask 6 (docs/PHASE-64 - page file.md): the page file
// meets the rest of the system. DISMOUNT refuses the page file's volume
// and says why, as VMS's does; the reclaimer, not only the writer's own
// threshold, writes to the page file (opening it first); and a
// subprocess that used the page file gives its blocks and its charges
// back when it goes.

// mountPageFileVolume makes a fresh 2,000-block volume, mounts it on
// device, and returns the container's path.
func mountPageFileVolume(t *testing.T, c *console.Console, device string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "page.dsk")
	if err := c.Commands().InitializeContainer(path, 2000, "PAGE", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Commands().Mount(device, path, true); err != nil {
		t.Fatal(err)
	}
}

// TestDismountPageFileVolume: a volume holding the open page file can't be
// dismounted, and DISMOUNT says so, apart from users' open files; once the
// page file is closed, as at the session's end, it can.
func TestDismountPageFileVolume(t *testing.T) {
	pageFileSettings(t)
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "200")

	c, dispatch := wsDispatcher(t)
	mountPageFileVolume(t, c, "DUA1")

	pf := c.RTL.OpenPageFile()
	if pf == nil || !pf.Created() || pf.Name() != "DUA1:[000000]PAGEFILE.SYS;1" {
		t.Fatalf("the page file on DUA1 wasn't made: %+v", pf)
	}

	const (
		cannot   = "%DISM-W-CANNOTDMT, DUA1: cannot be dismounted"
		pageFile = "%DISM-W-INSWPGFIL, 1 swap or page files installed on volume"
		userFile = "%DISM-W-USERFILES, 1 user files open on volume"
	)

	for _, tt := range []struct{ line, want string }{
		{"DISMOUNT DUA1:", cannot + "|" + pageFile},
		{"OPEN/WRITE OUT DUA1:[000000]USER.DAT", ""},
		{"DISMOUNT DUA1:", cannot + "|" + pageFile + "|" + userFile},
		{"CLOSE OUT", ""},
		{"DISMOUNT DUA1:", cannot + "|" + pageFile},
	} {
		if got := dispatch(tt.line); got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", tt.line, got, tt.want)
		}
	}

	if _, mounted := c.Commands().Mounts.Lookup("DUA1"); !mounted {
		t.Fatal("DUA1 was dismounted with the page file on it")
	}

	// The session's end closes the page file; the volume can go.
	c.EndSession()

	if got := dispatch("DISMOUNT DUA1:"); got != "" {
		t.Errorf("DISMOUNT DUA1: after the page file closed: %q", got)
	}

	if _, mounted := c.Commands().Mounts.Lookup("DUA1"); mounted {
		t.Error("DUA1 is still mounted")
	}
}

// TestReclaimerPageFile runs reclaimSource (workingsetreclaim_test.go) to
// its first checkpoint with the reclaimer always wanted and a page file on
// a volume. The writer's own threshold, MPW_HILIMIT, stays at its default,
// above anything the program sends to the modified list, so it's the
// reclaimer that opens the page file and writes the array's pages it
// trimmed there: where Phase 63's reclaimer, with no page file, left them
// on the modified list, their frames are now on the free list, backed by
// page file blocks that hold their contents.
func TestReclaimerPageFile(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	setSetting(t, "vax.sysgen.pql_dwsquota", "40")
	setSetting(t, "vax.sysgen.mpw_lolimit", "0")
	setSetting(t, "vax.sysgen.freelim", "1000000")
	setSetting(t, "vax.sysgen.freegoal", "1000000")
	setSetting(t, "vax.sysgen.paging.file", "DUA0:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "400")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	db := c.Mem.Paging()

	if db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	if c.RTL.Sysgen.MPWHiLimit < 80 {
		t.Fatalf("MPW_HILIMIT is %d; the test needs it above the array's 80 pages", c.RTL.Sysgen.MPWHiLimit)
	}

	mountPageFileVolume(t, c, "DUA0")

	exe := buildImage(t, c, "wsreclaim", reclaimSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == 1 })
	step(t, c, 4)
	checkWorkingSets(t, c)

	pf := c.RTL.PageFile()
	if pf == nil {
		t.Fatalf("the reclaimer didn't open the page file\n%s", out.String())
	}

	counts := c.RTL.PageCounts
	if counts.Reclaims == 0 || counts.Trimmed < 80-reclaimQuota {
		t.Errorf("%d reclaimer runs, %d pages trimmed", counts.Reclaims, counts.Trimmed)
	}

	array := c.CPU.GPR(8)
	buf := make([]byte, 512)
	written, modified := 0, 0

	for i := range uint32(80) {
		pfn, ok := pteIn(t, c, array+i*512).Transition()
		if !ok {
			continue
		}

		e, err := db.Entry(paging.PFN(pfn))
		if err != nil {
			t.Fatal(err)
		}

		if e.State == paging.Modified {
			modified++

			continue
		}

		if e.Backing.Kind != paging.PageFile || e.Backing.Block == 0 {
			t.Errorf("page %d's frame is on the %s list with backing %+v", i, e.State, e.Backing)

			continue
		}

		// The touch loop counts down: page i holds 80 - i.
		if err := pf.Read(e.Backing.Block, buf); err != nil {
			t.Fatal(err)
		}

		if got := binary.LittleEndian.Uint32(buf); got != 80-i {
			t.Errorf("page %d's block %d holds %d, want %d", i, e.Backing.Block, got, 80-i)
		}

		written++
	}

	if written < 80-reclaimQuota-reclaimSlack || modified != 0 {
		t.Errorf("%d of the array's pages written to the page file, %d waiting on the modified list; want at least %d and none",
			written, modified, 80-reclaimQuota-reclaimSlack)
	}

	if counts.Written < uint64(written) || pf.Map.UsedBlocks() < uint32(written) {
		t.Errorf("%d pages written, %d page file blocks in use, for %d pages", counts.Written, pf.Map.UsedBlocks(), written)
	}

	if err := pf.Map.Check(); err != nil {
		t.Fatal(err)
	}
}

// subPageFileSource adds 120 pages to P0 with $EXPREG, writes each one's
// number (counting down) in its first longword, and checks them, ending
// with SS$_NORMAL when every page held its number and status 4 (a severe
// error) when one didn't.
const subPageFileSource = `	.title	pfsub
	.psect	data,noexe,wrt
ret:	.blkl	2
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3>
	$expreg_s pagcnt=#120,retadr=ret
	blbc	r0,90$
	movl	ret,r2
	movl	#120,r3
10$:	movl	r3,(r2)
	addl2	#512,r2
	sobgtr	r3,10$
	movl	ret,r2
	movl	#120,r3
20$:	cmpl	r3,(r2)
	bneq	80$
	addl2	#512,r2
	sobgtr	r3,20$
	movl	#1,r0
	ret
80$:	movl	#4,r0
90$:	ret
	.end	start
`

// TestSubprocessPageFile: a SPAWNed subprocess runs an image whose pages
// outgrow its working set, so the writer sends them to the page file; and
// when the subprocess has logged out, its job's pool has its charges back
// and the page file holds only process 1's blocks.
func TestSubprocessPageFile(t *testing.T) {
	pageFileSettings(t)

	c, dispatch := wsDispatcher(t)
	job := c.ProcessOne().Process.Job
	charged, procs := job.PageFileCharged, len(c.RTL.Processes())

	exe := buildImage(t, c, "pfsub", subPageFileSource)
	proc := filepath.Join(t.TempDir(), "pfsub.com")

	if err := os.WriteFile(proc, []byte("$ RUN \""+exe+"\"\n$ WRITE SYS$OUTPUT $STATUS\n$ LOGOUT\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	written := c.RTL.PageCounts.Written
	got := dispatch(`SPAWN @"` + proc + `"`)

	if !strings.Contains(got, "|%X00000001|") {
		t.Fatalf("the subprocess's image didn't read its pages back:\n%s", strings.ReplaceAll(got, "|", "\n"))
	}

	if c.RTL.PageCounts.Written-written < 120-40 {
		t.Errorf("%d pages written while the subprocess ran, want at least %d", c.RTL.PageCounts.Written-written, 120-40)
	}

	if n := len(c.RTL.Processes()); n != procs {
		t.Fatalf("%d processes after the subprocess logged out, want %d", n, procs)
	}

	if job.PageFileCharged != charged {
		t.Errorf("the job has %d pages charged after the subprocess went, want %d", job.PageFileCharged, charged)
	}

	if c.RTL.PageFile() == nil {
		t.Fatal("no page file open")
	}

	checkPageFileHolders(t, c, "after the subprocess")
}
