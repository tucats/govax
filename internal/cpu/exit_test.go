package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// p1Stub writes a P1-vector-style service stub at addr: an empty entry
// mask, then XFC with selector XFC$P1VECTOR.
func p1Stub(t *testing.T, e *Engine, addr uint32) {
	t.Helper()
	putBytes(t, e.cpu, e.mem, addr, 0x00, 0x00, 0xFC, xfcP1Vector)
}

// TestXfcServiceCallCallsProcedure: a service answering with a
// *ServiceCall has the routine called as CALLG ArgList, Routine would,
// with the XFC as the return address and R0 left alone. The routine's RET
// lands back on the XFC.
func TestXfcServiceCallCallsProcedure(t *testing.T) {
	e := newEngine()
	c := e.cpu

	const (
		stub    = 0x1000
		routine = 0x2000
		argList = 0x8F00
	)

	p1Stub(t, e, stub)
	putBytes(t, c, e.mem, routine, 0x00, 0x00, 0x04) // entry mask, RET

	c.SetGPR(vax.PC, stub+2) // at the XFC, as after CALLS to the stub
	c.SetGPR(vax.SP, 0x9000)
	c.SetGPR(vax.R0, 0x55)

	e.SetSystemServices(&fakeServices{serviceHandled: true, serviceErr: &ServiceCall{Routine: routine, ArgList: argList}})

	if err := e.Step(); err != nil {
		t.Fatal(err)
	}

	if c.GPR(vax.PC) != routine+2 || c.GPR(vax.AP) != argList {
		t.Errorf("after the XFC: PC=%#x AP=%#x, want the routine (%#x) with AP at the argument list (%#x)",
			c.GPR(vax.PC), c.GPR(vax.AP), routine+2, argList)
	}

	if c.GPR(vax.R0) != 0x55 {
		t.Errorf("R0 = %#x, want it untouched (0x55)", c.GPR(vax.R0))
	}

	if err := e.Step(); err != nil { // the routine's RET
		t.Fatal(err)
	}

	if c.GPR(vax.PC) != stub+2 {
		t.Errorf("after the routine's RET: PC=%#x, want the XFC at %#x again", c.GPR(vax.PC), stub+2)
	}
}

// TestXfcImageExitUnwindsToConsoleCall: ErrImageExit from a service two
// procedure calls deep sets R0 and returns from the console's CallEntry
// frame, discarding the frame in between: ErrConsoleCallReturned, with
// SP back where it was before the console's call.
func TestXfcImageExitUnwindsToConsoleCall(t *testing.T) {
	e := newEngine()
	c := e.cpu

	const (
		main = 0x1000
		stub = 0x1100
	)

	// main: entry mask, then CALLS #0, @#stub.
	putBytes(t, c, e.mem, main, 0x00, 0x00, 0xFB, 0x00, 0x9F, 0x00, 0x11, 0x00, 0x00)
	p1Stub(t, e, stub)

	c.SetGPR(vax.SP, 0x9000)

	if err := e.CallEntry(main); err != nil {
		t.Fatal(err)
	}

	e.SetSystemServices(&fakeServices{serviceHandled: true, serviceRC: 0x2A, serviceErr: ErrImageExit})

	if err := e.Step(); err != nil { // CALLS to the stub
		t.Fatal(err)
	}

	if err := e.Step(); !errors.Is(err, ErrConsoleCallReturned) { // the XFC
		t.Fatalf("Step at the XFC = %v, want ErrConsoleCallReturned", err)
	}

	if c.GPR(vax.R0) != 0x2A {
		t.Errorf("R0 = %#x, want the exit status 0x2A", c.GPR(vax.R0))
	}

	if c.GPR(vax.SP) != 0x9000 {
		t.Errorf("SP = %#x, want 0x9000, as before the console's call", c.GPR(vax.SP))
	}
}

// TestXfcImageExitWithoutConsoleCallHalts: with no CallEntry frame to
// return to, ErrImageExit halts the machine, leaving the status in R0.
func TestXfcImageExitWithoutConsoleCallHalts(t *testing.T) {
	e := newEngine()
	c := e.cpu

	p1Stub(t, e, 0x1000)
	c.SetGPR(vax.PC, 0x1002)
	c.SetGPR(vax.SP, 0x9000)
	c.SetGPR(vax.FP, 0)

	e.SetSystemServices(&fakeServices{serviceHandled: true, serviceRC: 0x2A, serviceErr: ErrImageExit})

	if err := e.Step(); !errors.Is(err, ErrHalted) {
		t.Fatalf("Step = %v, want ErrHalted", err)
	}

	if c.GPR(vax.R0) != 0x2A {
		t.Errorf("R0 = %#x, want the exit status 0x2A", c.GPR(vax.R0))
	}
}
