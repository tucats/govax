package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// condFrameC is a third frame, frame B's caller, for unwinds that need
// somewhere to return to past B.
const condFrameC = 0x13000

// unwindFixture is conditionFixture with the RET after the stub that an
// unwind returns through, frame C above frame B, and distinct return
// addresses in each frame.
func unwindFixture(t *testing.T) *Environment {
	t.Helper()

	env := conditionFixture(t)

	if err := env.mem.StoreByte(env.cpu, condStub+2, opRET); err != nil {
		t.Fatal(err)
	}

	putLongword(t, env, condFrameB+frameSavedFP, condFrameC)
	putLongword(t, env, condFrameC, 0)
	putLongword(t, env, condFrameC+frameSavedFP, consoleCallSentinel)

	putLongword(t, env, condFrameA+frameSavedPC, 0x3100) // into B's procedure
	putLongword(t, env, condFrameB+frameSavedPC, 0x3200) // into C's
	putLongword(t, env, condFrameC+frameSavedPC, consoleCallSentinel)

	return env
}

// dispatchToH2 dispatches a reserved operand and runs the search until
// frame B's handler is running, frame A's having resignaled.
func dispatchToH2(t *testing.T, env *Environment) *conditionDispatch {
	t.Helper()

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, env.cpu.PSL()); !ok {
		t.Fatal("not dispatched")
	}

	_, _ = serviceSysSrchandler(env, nil) // H1
	env.cpu.SetGPR(vax.R0, ssResignal)

	r0, err := serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH2, condFrameB, 1)

	return env.Process.conditions[0]
}

// TestUnwind_default walks a default unwind from frame B's handler: to
// B's caller, calling A's and B's handlers with SS$_UNWIND, then
// returning through RETs.
func TestUnwind_default(t *testing.T) {
	env := unwindFixture(t)
	c := env.cpu
	d := dispatchToH2(t, env)

	wantR0(t, callLNM(t, env, serviceSysUnwind), ssNormal)
	putLongword(t, env, d.mech+12, 0x99) // what B will seem to return

	// H2 returns; its answer (resignal, here) is ignored.
	c.SetGPR(vax.R0, ssResignal)

	r0, err := serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH1, condFrameA, 0)

	if sig := longwordsAt(t, env, d.sig, 2); sig[0] != 1 || sig[1] != ssUnwind {
		t.Errorf("signal array %#x, want 1, SS$_UNWIND", sig)
	}

	r0, err = serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH2, condFrameB, 1)

	r0, err = serviceSysSrchandler(env, nil)
	if err != nil || r0 != 0x99 {
		t.Fatalf("finish = %#x, %v; want R0 0x99", r0, err)
	}

	if c.GPR(vax.PC) != condStub+2 || c.GPR(vax.FP) != condFrameA || c.GPR(vax.R1) != 0xBBBB {
		t.Errorf("PC %#x FP %#x R1 %#x; want the RET %#x, frame A, R1 0xBBBB", c.GPR(vax.PC), c.GPR(vax.FP), c.GPR(vax.R1), condStub+2)
	}

	// A returns into the RET; B returns where it always would, into C.
	if got := longwordsAt(t, env, condFrameA+frameSavedPC, 1)[0]; got != condStub+2 {
		t.Errorf("frame A returns to %#x, want the RET", got)
	}

	if got := longwordsAt(t, env, condFrameB+frameSavedPC, 1)[0]; got != 0x3200 {
		t.Errorf("frame B returns to %#x, want 0x3200", got)
	}

	if len(env.Process.conditions) != 0 {
		t.Errorf("%d dispatches left", len(env.Process.conditions))
	}
}

// TestUnwind_depthAndNewPC unwinds one frame (A only) with a new PC.
func TestUnwind_depthAndNewPC(t *testing.T) {
	env := unwindFixture(t)
	c := env.cpu
	d := dispatchToH2(t, env)

	const depadr = 0x6000

	putLongword(t, env, depadr, 1)
	wantR0(t, callLNM(t, env, serviceSysUnwind, depadr, 0x3180), ssNormal)

	if len(d.unwind.frames) != 1 {
		t.Fatalf("frames %#x, want frame A only", d.unwind.frames)
	}

	c.SetGPR(vax.R0, 1)

	_, _ = serviceSysSrchandler(env, nil) // A's handler, SS$_UNWIND

	if _, err := serviceSysSrchandler(env, nil); err != nil {
		t.Fatal(err)
	}

	if got := longwordsAt(t, env, condFrameA+frameSavedPC, 1)[0]; got != 0x3180 {
		t.Errorf("frame A returns to %#x, want the new PC 0x3180", got)
	}
}

func TestUnwind_errors(t *testing.T) {
	env := unwindFixture(t)

	// No handler running.
	wantR0(t, callLNM(t, env, serviceSysUnwind), ssNoSignal)

	dispatchToH2(t, env)

	const depadr = 0x6000

	putLongword(t, env, depadr, 0)
	wantR0(t, callLNM(t, env, serviceSysUnwind, depadr), ssNormal) // depth 0: nothing

	if env.Process.conditions[0].unwind != nil {
		t.Error("depth 0 recorded an unwind")
	}

	putLongword(t, env, depadr, 3) // A, B, and C: nothing left to return to
	wantR0(t, callLNM(t, env, serviceSysUnwind, depadr), ssInsFrame)
	wantR0(t, callLNM(t, env, serviceSysUnwind, 0x7FFF0000), ssAccVio)

	putLongword(t, env, depadr, 2)
	wantR0(t, callLNM(t, env, serviceSysUnwind, depadr), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysUnwind, depadr), ssUnwinding)
}

// TestUnwind_vectoredDefault checks that a vectored handler's default
// unwind does nothing.
func TestUnwind_vectoredDefault(t *testing.T) {
	env := unwindFixture(t)
	env.Process.exceptionVectors[env.cpu.PSL().CurMod()][vectorPrimary] = 0x5200

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, env.cpu.PSL()); !ok {
		t.Fatal("not dispatched")
	}

	_, _ = serviceSysSrchandler(env, nil) // the primary vector's handler

	wantR0(t, callLNM(t, env, serviceSysUnwind), ssNormal)

	if env.Process.conditions[0].unwind != nil {
		t.Error("a vectored handler's default unwind recorded one")
	}
}

// TestUnwind_libSignal checks that unwinding a LIB$SIGNAL removes the
// stub's frame too, uncounted.
func TestUnwind_libSignal(t *testing.T) {
	env, _ := signalFixture(t)

	if err := env.mem.StoreByte(env.cpu, condStub+2, opRET); err != nil {
		t.Fatal(err)
	}

	if _, err := env.Signal([]uint32{userWarning}, false); err != nil {
		t.Fatal(err)
	}

	_, _ = serviceSysSrchandler(env, nil) // H1, frame A, depth 0

	wantR0(t, callLNM(t, env, serviceSysUnwind), ssNormal)

	d := env.Process.conditions[0]
	if f := d.unwind.frames; len(f) != 2 || f[0] != signalFrame || f[1] != condFrameA {
		t.Fatalf("frames %#x, want the stub's frame and frame A", f)
	}

	env.cpu.SetGPR(vax.R0, 1)

	// A's handler is called with SS$_UNWIND at depth 0 (the stub's frame
	// has no handler).
	r0, err := serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH1, condFrameA, 0)

	if _, err := serviceSysSrchandler(env, nil); err != nil {
		t.Fatal(err)
	}

	if env.cpu.GPR(vax.FP) != signalFrame {
		t.Errorf("FP %#x, want the stub's frame first", env.cpu.GPR(vax.FP))
	}
}

// TestLibSigToRet runs LIB$SIG_TO_RET as frame A's handler: A returns
// the condition value to B.
func TestLibSigToRet(t *testing.T) {
	env := unwindFixture(t)
	c := env.cpu

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, c.PSL()); !ok {
		t.Fatal("not dispatched")
	}

	d := env.Process.conditions[0]
	_, _ = serviceSysSrchandler(env, nil) // "H1" stands in for LIB$SIG_TO_RET

	if r0, err := env.ConditionToReturn(d.sig, d.mech); err != nil || r0 != ssNormal {
		t.Fatalf("LIB$SIG_TO_RET = %#x, %v", r0, err)
	}

	if len(d.unwind.frames) != 1 {
		t.Fatalf("frames %#x, want frame A", d.unwind.frames)
	}

	_, _ = serviceSysSrchandler(env, nil) // A's handler again, with SS$_UNWIND

	// Called with SS$_UNWIND, it leaves the saved R0 alone.
	if r0, _ := env.ConditionToReturn(d.sig, d.mech); r0 != ssNormal {
		t.Errorf("LIB$SIG_TO_RET during the unwind = %#x", r0)
	}

	r0, err := serviceSysSrchandler(env, nil)
	if err != nil || r0 != 0x454 {
		t.Errorf("finish = %#x, %v; want R0 SS$_ROPRAND", r0, err)
	}

	if r0, _ := env.ConditionToReturn(d.sig, d.mech); r0 != ssNoSignal {
		t.Errorf("with no condition = %#x, want SS$_NOSIGNAL", r0)
	}
}

// TestUnwind_discardsOuterDispatch checks that an unwind past the frames
// of an outer dispatch's handler ends that dispatch too.
func TestUnwind_discardsOuterDispatch(t *testing.T) {
	env := unwindFixture(t)
	c := env.cpu

	// The stack below the frames, as it really is: SP moves down as
	// procedures are called.
	c.SetGPR(vax.SP, 0xF000)

	outer := dispatchToH2(t, env) // H2 running for the outer condition

	// In H2 (frame 0x12000, called from B... its frame chain continues at
	// B), a second condition; H2's frame has no handler, so the search
	// reaches B's handler, which unwinds by default to C.
	const h2Frame = 0xE000

	putLongword(t, env, h2Frame, 0)
	putLongword(t, env, h2Frame+frameSavedFP, condFrameB)
	putLongword(t, env, h2Frame+frameSavedPC, 0x5150)
	c.SetGPR(vax.FP, h2Frame)
	c.SetGPR(vax.SP, outer.sp-0x100)

	if ok, _ := env.DispatchException(scbReservedAddr, nil, 0x5140, c.PSL()); !ok {
		t.Fatal("inner not dispatched")
	}

	r0, err := serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH2, condFrameB, 1)

	wantR0(t, callLNM(t, env, serviceSysUnwind), ssNormal) // to C

	c.SetGPR(vax.R0, 1)

	// B's handler with SS$_UNWIND (H2's own frame has none), then the
	// finish.
	r0, err = serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH2, condFrameB, 1)

	if _, err := serviceSysSrchandler(env, nil); err != nil {
		t.Fatal(err)
	}

	if len(env.Process.conditions) != 0 {
		t.Errorf("%d dispatches left; the outer one's stack is gone too", len(env.Process.conditions))
	}
}
