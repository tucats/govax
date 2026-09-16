package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// newCHFConsole returns a runnable console with FP/AP left at zero (no call
// frame chain at all -- chf's own VAX_NOFRAMES case) and a scratch base for
// a test to build a synthetic call-frame chain in, matching image.go's own
// use of CONSOLE$SCRATCH for hand-built code sequences.
func newCHFConsole(t *testing.T) (*Console, uint32) {
	t.Helper()

	c := newRunnableConsole(t)

	base, ok := c.Symbols.Get("CONSOLE$SCRATCH")
	if !ok {
		t.Fatal("expected CONSOLE$SCRATCH to be defined")
	}

	return c, base
}

// handlerStub returns a tiny, real, callable procedure's raw bytes: an
// empty entry mask, MOVL #code,R0, RET -- exactly what Console.Call's own
// CallEntry expects at a target address (see internal/cpu/call.go's
// buildCallFrame), used here to stand in for a VMS condition handler that
// declines (an even code, SS$_CONTINUE's bit 0 clear) or continues (an odd
// code).
func handlerStub(code uint32) []byte {
	return []byte{
		0x00, 0x00, // entry mask
		0xD0, 0x8F, byte(code), byte(code >> 8), byte(code >> 16), byte(code >> 24), 0x50, // MOVL #code,R0
		0x04, // RET
	}
}

// writeFrame builds one synthetic call-frame-chain link at addr: handler at
// +0, an all-zero mask/saved-AP (no saved registers), and nextFP at +12 --
// matching the layout chf's own frame walk reads (see chf.go's doc
// comment).
func writeFrame(t *testing.T, c *Console, addr, handler, nextFP uint32) {
	t.Helper()

	if err := c.storeLong(addr, handler); err != nil {
		t.Fatalf("writeFrame(handler): %v", err)
	}

	if err := c.storeLong(addr+4, 0); err != nil {
		t.Fatalf("writeFrame(mask): %v", err)
	}

	if err := c.storeLong(addr+8, 0); err != nil {
		t.Fatalf("writeFrame(saved AP): %v", err)
	}

	if err := c.storeLong(addr+12, nextFP); err != nil {
		t.Fatalf("writeFrame(saved FP): %v", err)
	}
}

func TestHandleConsoleFault_noFramesFormatsAndHalts(t *testing.T) {
	c, _ := newCHFConsole(t)
	c.CPU.SetGPR(vax.FP, 0)
	c.CPU.SetGPR(vax.AP, 0)

	fault := &cpu.ConsoleHandlerFault{
		Fault: &cpu.Fault{Code: cpu.ExcReservedOp, Args: []uint32{0xAAAA}},
		PC:    0x1234,
		PSL:   c.CPU.PSL(),
	}

	if err := c.handleConsoleFault(fault); err != nil {
		t.Fatalf("handleConsoleFault: %v", err)
	}

	if !c.Engine.Halted() {
		t.Error("expected the machine to be halted after an unhandled console fault")
	}

	out := c.Out.(*bytes.Buffer).String()
	if !strings.Contains(out, "%VAX-E-CONHANDLER, RESOP, PC=00001234") {
		t.Errorf("output = %q, want a %%VAX-E-CONHANDLER RESOP diagnostic naming the fault PC", out)
	}

	if !strings.Contains(out, "0000AAAA") {
		t.Errorf("output = %q, want the signal argument 0000AAAA reported", out)
	}
}

func TestHandleConsoleFault_frameChainNoHandlerFormatsAndHalts(t *testing.T) {
	c, base := newCHFConsole(t)

	// A single frame with no handler (handler == 0), whose saved-FP slot is
	// zero -- chf's own "fp == 0" stop condition, walked once and giving up,
	// same observable outcome as no frames at all but exercising the walk.
	writeFrame(t, c, base, 0, 0)
	c.CPU.SetGPR(vax.FP, base)
	c.CPU.SetGPR(vax.AP, base)

	fault := &cpu.ConsoleHandlerFault{
		Fault: &cpu.Fault{Code: cpu.ExcAccessViol},
		PC:    0x5678,
		PSL:   c.CPU.PSL(),
	}

	if err := c.handleConsoleFault(fault); err != nil {
		t.Fatalf("handleConsoleFault: %v", err)
	}

	if !c.Engine.Halted() {
		t.Error("expected the machine to be halted")
	}

	out := c.Out.(*bytes.Buffer).String()
	if !strings.Contains(out, "%VAX-E-CONHANDLER, ACCVIO, PC=00005678") {
		t.Errorf("output = %q, want an ACCVIO diagnostic", out)
	}
}

// TestChf_handlerContinuesResumesAtOriginalFaultPC checks the "handled"
// path end-to-end: a single installed handler that returns an odd R0
// (SS$_CONTINUE's bit 0 set) is treated as having handled the exception,
// the machine is NOT halted, and PC/R0/R1 are reloaded from the mechanism/
// signal arrays -- which, since this handler never touches those stack
// slots itself, means PC ends up back at the original fault PC (not
// wherever the nested handler call itself left it) and R0/R1 end up back
// at their pre-fault values, exactly VMS's own "resume as if nothing
// happened" continue semantics.
func TestChf_handlerContinuesResumesAtOriginalFaultPC(t *testing.T) {
	c, base := newCHFConsole(t)

	handlerAddr := base + 0x40
	if err := c.storeBytes(handlerAddr, handlerStub(1)); err != nil { // odd: continue
		t.Fatalf("storeBytes(handler): %v", err)
	}

	writeFrame(t, c, base, handlerAddr, 0)
	c.CPU.SetGPR(vax.FP, base)
	c.CPU.SetGPR(vax.AP, base)
	c.CPU.SetGPR(vax.R0, 0x1111)
	c.CPU.SetGPR(vax.R1, 0x2222)

	fault := &cpu.ConsoleHandlerFault{
		Fault: &cpu.Fault{Code: cpu.ExcReservedOp},
		PC:    0x9000,
		PSL:   c.CPU.PSL(),
		R0:    0x1111,
		R1:    0x2222,
	}

	if err := c.handleConsoleFault(fault); err != nil {
		t.Fatalf("handleConsoleFault: %v", err)
	}

	if c.Engine.Halted() {
		t.Error("expected the machine NOT to be halted -- the handler continued")
	}

	if got := c.CPU.GPR(vax.PC); got != 0x9000 {
		t.Errorf("PC = %#x, want the original fault PC 0x9000 restored", got)
	}

	if got := c.CPU.GPR(vax.R0); got != 0x1111 {
		t.Errorf("R0 = %#x, want the original 0x1111 restored", got)
	}

	if got := c.CPU.GPR(vax.R1); got != 0x2222 {
		t.Errorf("R1 = %#x, want the original 0x2222 restored", got)
	}
}

// TestChf_handlerDeclinesRestoresRegistersAndKeepsSearching checks that a
// declining handler (even R0, SS$_CONTINUE's bit 0 clear) doesn't stop the
// search or leak its own register writes into the next frame's attempt --
// this also exercises the fix to interrupt.c's own `rc && 0x00000001` typo
// (see invokeHandler's doc comment): with the original `&&`, this handler's
// nonzero-but-even return code (2) would have been wrongly treated as
// "continue".
func TestChf_handlerDeclinesRestoresRegistersAndKeepsSearching(t *testing.T) {
	c, base := newCHFConsole(t)

	handlerAddr := base + 0x40
	// This handler clobbers R0/R1 before declining (MOVL #2,R0 leaves R0=2;
	// R1 is left untouched by the stub, so the meaningful check is that R0
	// -- which the stub DOES write -- comes back to its pre-call value).
	if err := c.storeBytes(handlerAddr, handlerStub(2)); err != nil { // even: decline
		t.Fatalf("storeBytes(handler): %v", err)
	}

	writeFrame(t, c, base, handlerAddr, 0) // declines, then fp=0 ends the search
	c.CPU.SetGPR(vax.FP, base)
	c.CPU.SetGPR(vax.AP, base)
	c.CPU.SetGPR(vax.R0, 0x3333)

	fault := &cpu.ConsoleHandlerFault{
		Fault: &cpu.Fault{Code: cpu.ExcReservedOp},
		PC:    0x9100,
		PSL:   c.CPU.PSL(),
		R0:    0x3333,
	}

	if err := c.handleConsoleFault(fault); err != nil {
		t.Fatalf("handleConsoleFault: %v", err)
	}

	if !c.Engine.Halted() {
		t.Error("expected the machine to be halted -- the only handler declined")
	}

	if got := c.CPU.GPR(vax.R0); got != 0x3333 {
		t.Errorf("R0 = %#x, want the pre-call 0x3333 restored after the handler declined", got)
	}
}
