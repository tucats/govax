package console_test

import (
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 64's subtask 5 (docs/PHASE-64 - page file.md): the paging file
// quota. With paging on, the services that create pages charge them
// against the job's PGFLQUOTA and stop with SS$_EXQUOTA when it runs
// out, having made the pages before; deletion gives the charge back;
// image activation charges only writable sections' pages and the user
// stack's; a page is charged when it's created, never when it's touched
// (Phase 67's subtask 6); a subprocess draws on its job's pool and gives
// its charges back when it goes.

// quotaSource runs the services against the quota the test sets at each
// checkpoint. At each it waits in a loop until the test sets R10 to the
// checkpoint's number, R6; R9 is the last service's status.
//
//  1. Nothing done yet: R7 is the code's address, R8 the data's.
//  2. $EXPREG of 50 P0 pages (the test leaves 30 pages of quota).
//  3. $DELTVA of what it made.
//  4. $CRETVA of 40 pages 64 pages past them.
//  5. $EXPREG of 5 P1 pages (the test adds 5 pages to the quota).
//  6. $EXPREG of 1 more P1 page.
//  7. LIB$GET_VM of 100 pages.
const quotaSource = `	.title	pgfl
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.psect	data,noexe,wrt,page
data:	.blkb	512
ret1:	.blkl	2
ret2:	.blkl	2
ret3:	.blkl	2
cr:	.blkl	2
vm:	.blkl	1
size:	.long	512*100
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	moval	start,r7
	moval	data,r8
	stop	1
	$expreg_s pagcnt=#50,retadr=ret1
	movl	r0,r9
	stop	2
	$deltva_s inadr=ret1
	movl	r0,r9
	stop	3
	addl3	#<512*64>+1,ret1+4,cr
	addl3	#<512*40>-1,cr,cr+4
	$cretva_s inadr=cr,retadr=ret2
	movl	r0,r9
	stop	4
	$expreg_s pagcnt=#5,retadr=ret3,region=#1
	movl	r0,r9
	stop	5
	$expreg_s pagcnt=#1,retadr=ret3,region=#1
	movl	r0,r9
	stop	6
	pushal	vm
	pushal	size
	calls	#2,g^lib$get_vm
	movl	r0,r9
	stop	7
	movl	#1,r0
	ret
	.end	start
`

// TestPageFileQuotaServices runs quotaSource with paging on and checks the
// job's count, each service's status and range, and the process's peak.
func TestPageFileQuotaServices(t *testing.T) {
	pageFileSettings(t)

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	env := c.ProcessOne()

	// The console's output shows what went wrong in the image.
	t.Cleanup(func() {
		if t.Failed() {
			t.Log(out.String())
		}
	})
	job := env.Process.Job

	exe := buildImage(t, c, "pgfl", quotaSource)

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
	}

	release := func(n uint32) { c.CPU.SetGPR(10, n) }

	long := func(addr uint32) uint32 {
		t.Helper()

		v, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			t.Fatal(err)
		}

		return v
	}

	status := func(label, name string) {
		t.Helper()

		if got, want := c.CPU.GPR(9), vmsdef.Symbols[name]; got != want {
			t.Errorf("%s: status %08X, want %s (%08X)", label, got, name, want)
		}
	}

	remaining := func(label string, want int64) {
		t.Helper()

		if got := env.PageFileRemaining(); got != want {
			t.Errorf("%s: %d pages of quota left, want %d", label, got, want)
		}
	}

	// Checkpoint 1: the image's writable data page is charged, its
	// read-only code isn't (finding 12), and its user stack is, created at
	// activation.
	checkpoint(1)

	code, data := c.CPU.GPR(7), c.CPU.GPR(8)
	if env.PageFileChargedAt(code) {
		t.Errorf("the image's code page %08X is charged", code)
	}

	if !env.PageFileChargedAt(data) {
		t.Errorf("the image's data page %08X isn't charged", data)
	}

	if !env.PageFileChargedAt(c.CPU.GPR(registers.SP)) {
		t.Errorf("the user stack's page %08X isn't charged", c.CPU.GPR(registers.SP))
	}

	if job.PageFileCharged == 0 || job.PageFileCharged != env.PageFileCharged() {
		t.Errorf("the job has %d pages charged, process 1 %d", job.PageFileCharged, env.PageFileCharged())
	}

	before := job.PageFileCharged
	job.Pooled.PGFLQUOTA = before + 30

	// Checkpoint 2: $EXPREG of 50 pages makes 30 and returns SS$_EXQUOTA.
	ret1 := data + 512

	release(1)
	checkpoint(2)
	status("$EXPREG", "SS$_EXQUOTA")
	remaining("$EXPREG", 0)

	first, last := long(ret1), long(ret1+4)
	if last-first+1 != 30*512 {
		t.Errorf("$EXPREG made %08X to %08X, want 30 pages", first, last)
	}

	// Checkpoint 3: $DELTVA gives the 30 pages' charge back, though
	// nothing touched them.
	release(2)
	checkpoint(3)
	status("$DELTVA", "SS$_NORMAL")
	remaining("$DELTVA", 30)

	// Checkpoint 4: $CRETVA of 40 pages makes 30, SS$_EXQUOTA.
	release(3)
	checkpoint(4)
	status("$CRETVA", "SS$_EXQUOTA")
	remaining("$CRETVA", 0)

	if first, last := long(ret1+8), long(ret1+12); last-first+1 != 30*512 {
		t.Errorf("$CRETVA made %08X to %08X, want 30 pages", first, last)
	}

	// Checkpoint 5: 5 more pages of quota, and $EXPREG of 5 P1 pages.
	job.Pooled.PGFLQUOTA += 5

	release(4)
	checkpoint(5)
	status("$EXPREG of P1", "SS$_NORMAL")
	remaining("$EXPREG of P1", 0)

	// Checkpoint 6: one more P1 page is over the quota.
	release(5)
	checkpoint(6)
	status("$EXPREG of 1 P1 page", "SS$_EXQUOTA")

	// Checkpoint 7: the heap can't grow by 100 pages with none left.
	release(6)
	checkpoint(7)
	status("LIB$GET_VM", "LIB$_INSVIRMEM")

	peak := env.Process.PageFilePeak
	if peak < before+30 {
		t.Errorf("the process's peak is %d pages, want at least %d", peak, before+30)
	}

	// F$GETJPI shows what's left.
	if err := d.Dispatch(`WRITE SYS$OUTPUT "PAGFILCNT=", F$GETJPI("","PAGFILCNT")`); err != nil {
		t.Fatal(err)
	}

	// The system zones' control page, made at the heap's first use, is
	// charged past the quota (vmzone.go): the job is a page over.
	left := env.PageFileRemaining()
	if left != -1 {
		t.Errorf("%d pages of quota left after LIB$GET_VM, want -1", left)
	}

	if want := "PAGFILCNT=" + strconv.FormatInt(left, 10); !strings.Contains(out.String(), want) {
		t.Errorf("F$GETJPI's PAGFILCNT isn't %d:\n%s", left, out.String())
	}

	// Image rundown deletes P0, and with it P0's charges; the P1 pages
	// $EXPREG made stay charged.
	release(7)
	finishImage(t, c)

	for va := range env.ChargedPages() {
		if va < 0x40000000 {
			t.Errorf("after the image: P0 page %08X still charged", va)
		}
	}

	if job.PageFileCharged != env.PageFileCharged() {
		t.Errorf("after the image: the job has %d pages charged, process 1 %d", job.PageFileCharged, env.PageFileCharged())
	}
}

// finishImage runs the machine until the image ends.
func finishImage(t *testing.T, c *console.Console) {
	t.Helper()

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
}

// TestPageFileQuotaSubprocess: a subprocess's pages, charged as they're
// created (touching them charges nothing more), come from its job's
// pool, and go back to it when the subprocess is deleted. Its peak is its
// own.
func TestPageFileQuotaSubprocess(t *testing.T) {
	pageFileSettings(t)

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	sys := c.RTL.System
	job := c.ProcessOne().Process.Job
	before := job.PageFileCharged

	env, err := corevms.NewSubprocess(c.ProcessOne(), nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if env.Space, err = sys.BuildAddressSpace(env.Process.PID, 128, 64); err != nil {
		t.Fatal(err)
	}

	// With paging on a P0 page exists only once something creates it,
	// and it's charged then.
	if err := env.EnsurePages(0x200, 20*512); err != nil {
		t.Fatal(err)
	}

	if got := job.PageFileCharged - before; got != 20 {
		t.Errorf("after creation: the job has %d more pages charged, want the subprocess's 20", got)
	}

	// Touching the pages, each a demand-zero fault, charges nothing more;
	// and a fault on a page nothing created is an access violation, not a
	// charge.
	for i := range uint32(20) {
		c.Mem.BeginInstruction()

		if err := c.Mem.StoreLongwordIn(c.CPU, env.Space.AddressSpace, 0x200+i*512, i); err != nil {
			t.Fatal(err)
		}
	}

	c.Mem.BeginInstruction()

	if err := c.Mem.StoreLongwordIn(c.CPU, env.Space.AddressSpace, 0x200+20*512, 0); err == nil {
		t.Error("a store to a page nothing created succeeded")
	}

	if got := job.PageFileCharged - before; got != 20 {
		t.Errorf("after the touches: the job has %d more pages charged, want 20", got)
	}

	if env.Process.PageFilePeak != 20 {
		t.Errorf("the subprocess's peak is %d, want 20", env.Process.PageFilePeak)
	}

	if got := strconv.FormatInt(env.PageFileRemaining(), 10); got != strconv.FormatInt(c.ProcessOne().PageFileRemaining(), 10) {
		t.Errorf("the subprocess has %s pages left, process 1 %d", got, c.ProcessOne().PageFileRemaining())
	}

	sys.RemoveProcess(env)

	if job.PageFileCharged != before {
		t.Errorf("after the subprocess went: the job has %d pages charged, want %d", job.PageFileCharged, before)
	}
}

// TestPageFileQuotaPagingOff: with paging off, nothing is charged and
// $EXPREG makes every page.
func TestPageFileQuotaPagingOff(t *testing.T) {
	setSetting(t, machine.PagingSetting, "false")

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	env := c.ProcessOne()

	if err := c.Mem.StoreLongwordIn(c.CPU, env.Space.AddressSpace, 0x10000, 1); err != nil {
		t.Fatal(err)
	}

	if env.Process.Job.PageFileCharged != 0 || env.PageFileCharged() != 0 {
		t.Errorf("%d pages charged with paging off", env.Process.Job.PageFileCharged)
	}

	if got, want := env.PageFileRemaining(), int64(env.Process.Job.Pooled.PGFLQUOTA); got != want {
		t.Errorf("%d pages left with paging off, want the quota, %d", got, want)
	}
}

// TestPageFileFullNotice: a page file too small for the pages the writer
// wants to write is reported full on the console, once, and the pages
// it can't take stay on the modified list.
func TestPageFileFullNotice(t *testing.T) {
	pageFileSettings(t)
	setSetting(t, "vax.sysgen.paging.size", "20")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	exe := buildImage(t, c, "wspf", pfSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == 1 })
	checkWorkingSets(t, c)

	pf := c.RTL.PageFile()
	if pf == nil {
		t.Fatalf("no page file open\n%s", out.String())
	}

	if pf.Map.UsedBlocks() < pf.Size()-2 {
		t.Errorf("%d of the page file's %d blocks in use, want it all but full", pf.Map.UsedBlocks(), pf.Size())
	}

	if n := strings.Count(out.String(), "%GOVAX-W-PAGEFILEFULL"); n != 1 {
		t.Errorf("the page file was reported full %d times, want once:\n%s", n, out.String())
	}

	if c.Mem.Paging().ModifiedCount() == 0 {
		t.Error("no pages on the modified list with the page file full")
	}
}

// TestMemoryExhaustedNotice: a fault that finds no free frame tells the
// console, once until a fault succeeds again.
func TestMemoryExhaustedNotice(t *testing.T) {
	pageFileSettings(t)

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	db := c.Mem.Paging()

	if err := c.ProcessOne().EnsurePages(0x30000, 4); err != nil {
		t.Fatal(err)
	}

	space := c.ProcessOne().Space.AddressSpace

	var taken []uint32

	for db.FreeCount() > 0 {
		pfn, ok := c.Mem.AllocateFrame(vm.FrameClaim{})
		if !ok {
			t.Fatal("AllocateFrame failed with frames free")
		}

		taken = append(taken, pfn)
	}

	for range 2 {
		c.Mem.BeginInstruction()

		if err := c.Mem.StoreLongwordIn(c.CPU, space, 0x30000, 1); err == nil {
			t.Fatal("a store to a new page succeeded with no frame free")
		}
	}

	if n := strings.Count(out.String(), "%GOVAX-F-MEMEXHAUST"); n != 1 {
		t.Errorf("memory exhaustion reported %d times, want once:\n%s", n, out.String())
	}

	for _, pfn := range taken {
		c.Mem.FreePage(pfn)
	}
}
