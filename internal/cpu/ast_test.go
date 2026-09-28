package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// astFake is fakeServices plus a one-shot ASTSource: it hands out call
// once, the next time the engine asks.
type astFake struct {
	fakeServices

	call    ASTCall
	pending bool
	asked   int
}

func (f *astFake) NextAST() (ASTCall, bool, error) {
	f.asked++

	if !f.pending {
		return ASTCall{}, false, nil
	}

	f.pending = false

	return f.call, true, nil
}

// TestDeliverASTCallsLikeCALLG: one Step with an AST due calls the
// routine as CALLG would (its entry mask's registers saved, AP at the
// argument list) and then runs its first instruction. Here that's RET,
// which returns to ReturnPC with SP back at the argument list: CALLG
// frames don't pop their arguments, the RTL's AST exit does.
func TestDeliverASTCallsLikeCALLG(t *testing.T) {
	e := newEngine()
	c := e.cpu

	const (
		mainPC   = 0x1000
		routine  = 0x2000
		argList  = 0x8F00
		returnPC = 0x3000
	)

	putBytes(t, c, e.mem, mainPC, 0x01)             // NOP, never reached this step
	putBytes(t, c, e.mem, routine, 0x04, 0x00, 0x04) // entry mask ^M<R2>, then RET

	c.SetGPR(vax.PC, mainPC)
	c.SetGPR(vax.SP, argList)
	c.SetGPR(vax.R2, 0x2222)

	fake := &astFake{call: ASTCall{Routine: routine, ArgList: argList, ReturnPC: returnPC}, pending: true}
	e.SetSystemServices(fake)

	// Stop inside the routine to look at the frame: deliver, but don't
	// run the RET yet. deliverAST alone is the delivery half of Step.
	if err := e.deliverAST(); err != nil {
		t.Fatal(err)
	}

	if c.GPR(vax.PC) != routine+2 || c.GPR(vax.AP) != argList {
		t.Errorf("in the routine: PC=%#x AP=%#x, want %#x and the argument list %#x", c.GPR(vax.PC), c.GPR(vax.AP), routine+2, argList)
	}

	// The frame's saved PC (FP+16) is the AST exit, and R2 (from the
	// entry mask) is saved at FP+20.
	fp := c.GPR(vax.FP)
	if got, _ := e.mem.LoadLongword(c, fp+16); got != returnPC {
		t.Errorf("frame's saved PC = %#x, want ReturnPC %#x", got, returnPC)
	}

	if got, _ := e.mem.LoadLongword(c, fp+20); got != 0x2222 {
		t.Errorf("frame's saved R2 = %#x, want 0x2222", got)
	}

	// Now the RET.
	if err := e.Step(); err != nil {
		t.Fatal(err)
	}

	if c.GPR(vax.PC) != returnPC || c.GPR(vax.SP) != argList {
		t.Errorf("after RET: PC=%#x SP=%#x, want ReturnPC %#x, SP at the argument list %#x", c.GPR(vax.PC), c.GPR(vax.SP), returnPC, argList)
	}
}

// TestStepDeliversAST: Step asks for an AST at every instruction
// boundary and, given one, runs the routine's first instruction in the
// same step.
func TestStepDeliversAST(t *testing.T) {
	e := newEngine()
	c := e.cpu

	putBytes(t, c, e.mem, 0x1000, 0x01, 0x01)       // NOP; NOP
	putBytes(t, c, e.mem, 0x2000, 0x00, 0x00, 0x01) // entry mask ^M<>, then NOP
	c.SetGPR(vax.PC, 0x1000)
	c.SetGPR(vax.SP, 0x8F00)

	fake := &astFake{}
	e.SetSystemServices(fake)

	if err := e.Step(); err != nil {
		t.Fatal(err)
	}

	if c.GPR(vax.PC) != 0x1001 || fake.asked != 1 {
		t.Fatalf("no AST due: PC=%#x asked=%d, want 0x1001 and one question", c.GPR(vax.PC), fake.asked)
	}

	fake.call = ASTCall{Routine: 0x2000, ArgList: 0x8F00, ReturnPC: 0x3000}
	fake.pending = true

	if err := e.Step(); err != nil {
		t.Fatal(err)
	}

	if c.GPR(vax.PC) != 0x2003 {
		t.Errorf("PC = %#x, want 0x2003: the routine's NOP executed", c.GPR(vax.PC))
	}
}

// TestDeliverASTBadRoutine: an AST routine whose entry mask can't be
// read faults as the routine is given control, with the routine's
// address as the faulting PC.
func TestDeliverASTBadRoutine(t *testing.T) {
	e := newEngine()
	c := e.cpu

	c.SetGPR(vax.PC, 0x1000)
	c.SetGPR(vax.SP, 0x8F00)
	e.SetSystemServices(&astFake{call: ASTCall{Routine: 0x7FFFFFF0, ArgList: 0x8F00, ReturnPC: 0x3000}, pending: true})

	_ = e.Step() // the fault's delivery through the SCB isn't under test

	h := e.FaultHistory()
	if len(h) == 0 || h[len(h)-1].PC != 0x7FFFFFF0 {
		t.Fatalf("fault history = %+v, want a fault at the routine's address", h)
	}
}

// TestNoASTSourceNoQuestion: services without the optional ASTSource
// half (the plain fake) are never asked for ASTs.
func TestNoASTSourceNoQuestion(t *testing.T) {
	e := newEngine()
	e.SetSystemServices(&fakeServices{})

	if e.astSource != nil {
		t.Error("astSource set for services that don't implement ASTSource")
	}
}
