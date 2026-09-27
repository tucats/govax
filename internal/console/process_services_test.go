package console

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/cpu"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
)

// TestProcessServices_assembledProgram is docs/PHASE-26.md's acceptance
// test for its first batch of services: testdata/asm/process_services.asm,
// assembled and run as real VAX code, calls $ADJSTK, $ADJWSL, $ALLOC, and
// $ASCEFC (with $SETEF/$READEF on the associated cluster) through their
// real P1-vector addresses. The program checks each result itself and
// leaves 1 in R0 only if all of them worked; this test then checks the
// emulated process and device state they left behind.
func TestProcessServices_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	tta0 := c.DefineDevice("TTA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassTT})

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "process_services.asm"))
	if err != nil {
		t.Fatalf("Assemble(process_services.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("process_services.asm has no entry address")
	}

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running process_services.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("process_services.asm didn't finish within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every service call worked)", got)
	}

	if got := c.CPU.PR(vax.USP); got != 0x4FF8 {
		t.Errorf("USP = %#x, want 0x4FF8 from $ADJSTK", got)
	}

	p := c.RTL.Process
	if p.WSLimit != p.WSDefault+10 {
		t.Errorf("WSLimit = %d, want WSDEFAULT+10 from $ADJWSL", p.WSLimit)
	}

	if !tta0.Allocated() || tta0.PID != p.PID {
		t.Errorf("TTA0 allocated=%v PID=%#x, want allocated to %#x", tta0.Allocated(), tta0.PID, p.PID)
	}

	cl := p.CommonClusters[0]
	if cl == nil || cl.Name != "CLUSTER" || cl.Flags != 2 {
		t.Errorf("common cluster 2 = %+v, want CLUSTER with flag 65 set", cl)
	}
}

// TestEventFlagWait_timerInterrupt is docs/PHASE-26.md subtask 9's
// acceptance test: testdata/asm/wait_timer.asm waits in $WAITFR for an
// event flag that only its interval-timer interrupt handler sets. The
// wait re-executes the service's XFC each step, the timer interrupt is
// taken in between, and its handler's $SETEF ends the wait.
func TestEventFlagWait_timerInterrupt(t *testing.T) {
	c := newBootableConsole(t)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "wait_timer.asm"))
	if err != nil {
		t.Fatalf("Assemble(wait_timer.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("wait_timer.asm has no entry address")
	}

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running wait_timer.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("wait_timer.asm didn't finish within 100,000 steps (the wait never ended)")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (the timer interrupt ended the wait)", got)
	}
}

// TestEventFlagWait_blocksUntilSet: a $WAITFR on a flag nothing sets keeps
// the program in the wait (PC on the service's XFC) for as many steps as
// it's given; setting the flag from outside then lets it finish.
func TestEventFlagWait_blocksUntilSet(t *testing.T) {
	c := newBootableConsole(t)

	addr, _, err := c.Assemble(asmFixturePath(t, "wait_timer.asm"))
	if err != nil {
		t.Fatalf("Assemble(wait_timer.asm): %v", err)
	}

	// Keep the timer from ever interrupting: ICCS is cleared after every
	// step, so nothing asynchronous can set the flag.
	if err := c.Engine.CallEntry(addr); err != nil {
		t.Fatal(err)
	}

	waitfr := uint32(0)

	for i := 0; i < 5_000; i++ {
		if err := c.Engine.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}

		c.CPU.SetPR(vax.ICCS, 0)

		if pc := c.CPU.GPR(vax.PC); pc >= 0x7FFE0000 {
			waitfr = pc
		}
	}

	if waitfr == 0 || c.CPU.GPR(vax.PC) != waitfr {
		t.Fatalf("PC = %#x after 5,000 steps, want the program parked in $WAITFR's P1-vector stub", c.CPU.GPR(vax.PC))
	}

	c.RTL.Process.LocalEventFlags[0] |= 1 << 3

	runErr, hitCap := continueBounded(c, 1_000)
	if runErr != nil || hitCap {
		t.Fatalf("after setting flag 3: err=%v hitCap=%v, want the program to finish", runErr, hitCap)
	}

	// The program notices no tick ran, so it reports failure -- which
	// confirms the wait really was ended from outside.
	if got := c.CPU.GPR(vax.R0); got != 0 {
		t.Errorf("R0 = %d, want 0 (no timer tick ended this wait)", got)
	}
}

// continueBounded steps an already started console call to completion,
// like callBounded without the CallEntry.
func continueBounded(c *Console, maxSteps int) (err error, hitCap bool) {
	for i := 0; i < maxSteps; i++ {
		err := c.Engine.Step()
		if err == nil {
			continue
		}

		if errors.Is(err, cpu.ErrConsoleCallReturned) || errors.Is(err, cpu.ErrHalted) {
			return nil, false
		}

		return err, false
	}

	return nil, true
}
