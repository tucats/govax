package console_test

import (
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
)

// Phase 45's subtask 6: a process $CREPRC created is deleted when its
// image exits, and everything it was given comes back.

// runChild has process 1 create a subprocess running exe, at process 1's
// priority, and runs the machine until the subprocess has been deleted
// and process 1 runs again. It returns the subprocess.
func runChild(t *testing.T, c *console.Console, exe string) *corevms.Environment {
	t.Helper()

	one := c.RTL

	child, st := one.CreateProcess(corevms.CreateRequest{Image: exe, BasePriority: one.Process.BasePriority})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	c.Engine.RequestReschedule()
	runUntil(t, c, 100000, func() bool { return child.Deleted && c.RTL.Current() == one })

	return child
}

// memoryInUse is what the processes on c's machine hold: the S0 pool's
// pages in use, and the physical pages mapped.
func memoryInUse(c *console.Console) (pool uint32, frames int) {
	p := c.RTL.S0Pool()
	base, limit := p.Range()

	return (limit-base)/512 - p.FreePages(), c.Mem.MappedPages()
}

// TestDeleteProcess_imageExit: a subprocess whose image returns is
// deleted. It leaves the process table, the scheduler, and its job at
// once, while the CPU is still in its context; its page tables, stacks,
// and PCB go as soon as the CPU has left it, to process 1.
func TestDeleteProcess_imageExit(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	exe := buildChildImage(t, c)
	one := c.RTL

	child, st := one.CreateProcess(corevms.CreateRequest{Image: exe, BasePriority: one.Process.BasePriority})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	c.Engine.RequestReschedule()
	runUntil(t, c, 100000, func() bool { return child.Deleted })

	if _, found := one.FindProcess(child.Process.PID); found {
		t.Error("the deleted child is still in the process table")
	}

	if one.Process.SubprocessCount != 0 || one.Process.Job.SubprocessCount != 0 {
		t.Errorf("subprocess counts: process 1's %d, the job's %d; want 0",
			one.Process.SubprocessCount, one.Process.Job.SubprocessCount)
	}

	if c.RTL.Current() != child || !child.Space.Owned() {
		t.Errorf("current process %08X, the child's page tables kept: %v; want the child, still with them",
			c.RTL.Current().Process.PID, child.Space.Owned())
	}

	step(t, c, 1)

	if c.RTL.Current() != one || c.CPU.GPR(vax.PC) != codeAddr {
		t.Errorf("process %08X runs at %08X; want process 1 at %08X",
			c.RTL.Current().Process.PID, c.CPU.GPR(vax.PC), codeAddr)
	}

	if child.Space.Owned() {
		t.Error("the child's page tables outlived the switch to process 1")
	}

	for _, a := range one.S0Pool().Allocations() {
		if a.PID == child.Process.PID {
			t.Errorf("the child's %s is still allocated", a.Purpose)
		}
	}
}

// TestDeleteProcess_hundred: 100 subprocesses, one after another, each
// created, run, and deleted when its image returns, leave the S0 pool
// and physical memory as they found them. (The first one is run before
// counting: process 1 gets a PCB page the first time it's switched out,
// and keeps it.)
func TestDeleteProcess_hundred(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	exe := buildChildImage(t, c)

	runChild(t, c, exe)

	pool, frames := memoryInUse(c)

	for i := range 100 {
		child := runChild(t, c, exe)

		if child.Process.ExitStatus != 3 {
			t.Fatalf("child %d: status %08X, want 3", i, child.Process.ExitStatus)
		}
	}

	if p, f := memoryInUse(c); p != pool || f != frames {
		t.Errorf("after 100 processes: %d pool pages and %d frames in use; want %d and %d", p, f, pool, frames)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes in the table, want 1", n)
	}
}

// writerSource is a child image that creates a file on DUA0, writes a
// record to it, and returns without closing it.
const writerSource = `	.title	writer
	.psect	data,noexe,wrt
fab:	$fab	fnm=<DUA0:[000000]LEFTOPEN.DAT>,rfm=var,rat=cr,fac=put
rab:	$rab	fab=fab,rbf=rec,rsz=reclen
rec:	.ascii	/written, never closed/
reclen = .-rec
	.psect	code,exe,nowrt
	.entry	start,^m<>
	$create	fab=fab
	blbc	r0,done
	$connect rab=rab
	blbc	r0,done
	$put	rab=rab
done:	ret
	.end	start
`

// TestDeleteProcess_closesFiles: deleting a process closes the files it
// left open, as RMS's rundown does, so the record it wrote is in the
// file on the volume.
func TestDeleteProcess_closesFiles(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 400, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	child := runChild(t, c, buildImage(t, c, "writer", writerSource))
	if child.Process.ExitStatus&1 == 0 {
		t.Fatalf("the writer failed: status %08X", child.Process.ExitStatus)
	}

	lines, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]LEFTOPEN.DAT"}, rms.TextRecords)
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}

	if len(lines) != 1 || string(lines[0]) != "written, never closed" {
		t.Errorf("the file holds %q; want the one record", lines)
	}
}
