package console

import (
	"testing"

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
