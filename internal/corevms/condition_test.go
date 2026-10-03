package corevms

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// Addresses the condition tests lay things out at, inside the fixture's
// 1MB of memory.
const (
	condStub   = 0x8000  // stands in for SYS$SRCHANDLER
	condSP     = 0x20000 // the stack pointer when the condition happens
	condFrameA = 0x10000 // the innermost call frame
	condFrameB = 0x11000 // its caller's frame
	condH1     = 0x5000  // frame A's handler
	condH2     = 0x5100  // frame B's handler
)

// conditionFixture returns an Environment with a SYS$SRCHANDLER stub at
// condStub and a two-frame call chain: frame A (handler condH1) called
// from frame B (handler condH2), which the console's call frame called.
// Either handler can be removed by storing 0 over it.
func conditionFixture(t *testing.T) *Environment {
	t.Helper()

	env, _ := fixture()

	saved := srchandlerAddr
	srchandlerAddr = condStub

	t.Cleanup(func() { srchandlerAddr = saved })

	if err := env.mem.StoreWord(env.cpu, condStub, xfcP1VectorWord); err != nil {
		t.Fatal(err)
	}

	putLongword(t, env, condFrameA, condH1)
	putLongword(t, env, condFrameA+12, condFrameB)
	putLongword(t, env, condFrameB, condH2)
	putLongword(t, env, condFrameB+12, consoleCallSentinel)

	env.cpu.SetGPR(vax.SP, condSP)
	env.cpu.SetGPR(vax.FP, condFrameA)
	env.cpu.SetGPR(vax.R0, 0xAAAA)
	env.cpu.SetGPR(vax.R1, 0xBBBB)

	return env
}

// longwordsAt reads n longwords starting at addr.
func longwordsAt(t *testing.T, env *Environment, addr uint32, n int) []uint32 {
	t.Helper()

	out := make([]uint32, n)

	for i := range out {
		v, err := env.mem.LoadLongword(env.cpu, addr+uint32(4*i))
		if err != nil {
			t.Fatalf("LoadLongword(%#x): %v", addr+uint32(4*i), err)
		}

		out[i] = v
	}

	return out
}

// expectHandlerCall checks that a SYS$SRCHANDLER step asked for handler
// to be called with the dispatch's argument list, and that the mechanism
// array says frame and depth.
func expectHandlerCall(t *testing.T, env *Environment, r0 uint32, err error, handler, frame uint32, depth int32) {
	t.Helper()

	call, ok := err.(*CallRequest)
	if !ok || call.Routine != handler {
		t.Fatalf("SYS$SRCHANDLER = %#x, %v; want a call of %#x", r0, err, handler)
	}

	d := env.Process.conditions[len(env.Process.conditions)-1]
	if call.ArgList != d.argList {
		t.Errorf("argument list %#x, want %#x", call.ArgList, d.argList)
	}

	mech := longwordsAt(t, env, d.mech, 3)
	if mech[1] != frame || int32(mech[2]) != depth {
		t.Errorf("mechanism frame %#x depth %d; want %#x, %d", mech[1], int32(mech[2]), frame, depth)
	}
}

func TestExceptionCondition(t *testing.T) {
	cases := []struct {
		name   string
		code   uint32
		params []uint32
		cond   uint32
		detail []uint32
	}{
		{"access violation", scbAccessViol, []uint32{0x1234, 4}, 0x0C, []uint32{4, 0x1234}},
		{"translation not valid", scbTranslationNV, []uint32{0x200, 0}, 0x0C, []uint32{0, 0x200}},
		{"reserved operand", scbReservedOp, nil, 0x454, nil},
		{"reserved addressing mode", scbReservedAddr, nil, 0x44C, nil},
		{"privileged instruction", scbPrivileged, nil, 0x43C, nil},
		{"customer instruction", scbCustomer, nil, 0x434, nil},
		{"integer divide", scbArithmetic, []uint32{2}, 0x484, nil},
		{"subscript range", scbArithmetic, []uint32{7}, 0x4AC, nil},
		{"floating underflow fault", scbArithmetic, []uint32{10}, 0x4C4, nil},
		{"unknown arithmetic type", scbArithmetic, []uint32{99}, 0x474, nil},
	}

	for _, tc := range cases {
		cond, detail, ok := exceptionCondition(tc.code, tc.params)
		if !ok || cond != tc.cond || len(detail) != len(tc.detail) {
			t.Errorf("%s: %#x %v %v; want %#x %v", tc.name, cond, detail, ok, tc.cond, tc.detail)

			continue
		}

		for i := range detail {
			if detail[i] != tc.detail[i] {
				t.Errorf("%s: detail %v, want %v", tc.name, detail, tc.detail)
			}
		}
	}

	// Machine checks, breakpoints, and the like aren't signaled.
	for _, code := range []uint32{0x04, 0x28, 0x2C, 0x40} {
		if _, _, ok := exceptionCondition(code, nil); ok {
			t.Errorf("exception %#x was given a condition", code)
		}
	}
}

// TestDispatchException_layout checks what DispatchException leaves on the
// stack and in the registers.
func TestDispatchException_layout(t *testing.T) {
	env := conditionFixture(t)
	c := env.cpu
	psl := c.PSL()

	ok, err := env.DispatchException(scbAccessViol, []uint32{0x1234, 4}, 0x3000, psl)
	if !ok || err != nil {
		t.Fatalf("DispatchException = %v, %v; want dispatched", ok, err)
	}

	d := env.Process.conditions[0]

	if c.GPR(vax.PC) != condStub || c.GPR(vax.SP) != d.sp {
		t.Errorf("PC %#x SP %#x; want SYS$SRCHANDLER %#x and SP %#x", c.GPR(vax.PC), c.GPR(vax.SP), condStub, d.sp)
	}

	// From the top of the stack: the argument list, the mechanism array,
	// then the signal array, ending just below the old SP.
	got := longwordsAt(t, env, d.sp, 15)
	want := []uint32{
		2, d.sig, d.mech,
		4, 0, 0, 0xAAAA, 0xBBBB,
		5, 0x0C, 4, 0x1234, 0x3000, uint32(psl),
	}

	for i, w := range want {
		if got[i] != w {
			t.Errorf("stack longword %d = %#x, want %#x", i, got[i], w)
		}
	}

	if d.sig+4*6 != condSP {
		t.Errorf("the signal array ends at %#x, want the old SP %#x", d.sig+4*6, condSP)
	}
}

// TestDispatchException_declines checks the exceptions DispatchException
// leaves to the console, changing nothing.
func TestDispatchException_declines(t *testing.T) {
	cases := []struct {
		name  string
		code  uint32
		setup func(env *Environment) vax.PSL
	}{
		{"no condition value", 0x2C, func(env *Environment) vax.PSL { return env.cpu.PSL() }},
		{"interrupt stack", scbReservedOp, func(env *Environment) vax.PSL {
			psl := env.cpu.PSL()
			psl.SetIS(true)

			return psl
		}},
		{"no SYS$SRCHANDLER stub", scbReservedOp, func(env *Environment) vax.PSL {
			putLongword(t, env, condStub, 0)

			return env.cpu.PSL()
		}},
		{"stack not writable", scbReservedOp, func(env *Environment) vax.PSL {
			env.cpu.SetGPR(vax.SP, 0x7FFF0000)

			return env.cpu.PSL()
		}},
	}

	for _, tc := range cases {
		env := conditionFixture(t)
		psl := tc.setup(env)
		sp, pc := env.cpu.GPR(vax.SP), env.cpu.GPR(vax.PC)

		ok, err := env.DispatchException(tc.code, nil, 0x3000, psl)
		if ok || err != nil {
			t.Errorf("%s: DispatchException = %v, %v; want declined", tc.name, ok, err)
		}

		if env.cpu.GPR(vax.SP) != sp || env.cpu.GPR(vax.PC) != pc || len(env.Process.conditions) != 0 {
			t.Errorf("%s: SP %#x PC %#x, %d dispatches; want nothing changed", tc.name,
				env.cpu.GPR(vax.SP), env.cpu.GPR(vax.PC), len(env.Process.conditions))
		}
	}
}

// TestSrchandler_resignalThenContinue walks a search: frame A's handler
// resignals, frame B's changes the PC and saved R0 and continues.
func TestSrchandler_resignalThenContinue(t *testing.T) {
	env := conditionFixture(t)
	c := env.cpu
	psl := c.PSL()
	psl.SetZ(true)

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, psl); !ok {
		t.Fatal("not dispatched")
	}

	d := env.Process.conditions[0]

	r0, err := serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH1, condFrameA, 0)

	// Frame A's handler returns SS$_RESIGNAL, having clobbered R1.
	c.SetGPR(vax.R0, ssResignal)
	c.SetGPR(vax.R1, 0xDEAD)

	r0, err = serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH2, condFrameB, 1)

	// Frame B's handler moves the PC on, sets the Z bit's neighbor N in
	// the signal array's PSL, returns 0x77 through the mechanism array,
	// and continues.
	putLongword(t, env, d.sig+4*2, 0x3004)
	putLongword(t, env, d.sig+4*3, uint32(psl)|0x08)
	putLongword(t, env, d.mech+12, 0x77)
	c.SetGPR(vax.R0, 1)

	r0, err = serviceSysSrchandler(env, nil)
	if err != nil || r0 != 0x77 {
		t.Fatalf("continue = %#x, %v; want R0 0x77", r0, err)
	}

	if c.GPR(vax.PC) != 0x3004 || c.GPR(vax.SP) != condSP || c.GPR(vax.FP) != condFrameA {
		t.Errorf("PC %#x SP %#x FP %#x; want 0x3004, %#x, %#x", c.GPR(vax.PC), c.GPR(vax.SP), c.GPR(vax.FP), condSP, condFrameA)
	}

	if c.GPR(vax.R1) != 0xBBBB {
		t.Errorf("R1 = %#x, want the saved 0xBBBB", c.GPR(vax.R1))
	}

	if got := c.PSL(); !got.N() || !got.Z() || got.CurMod() != psl.CurMod() {
		t.Errorf("PSL %#x; want N and Z set, mode unchanged", uint32(got))
	}

	if len(env.Process.conditions) != 0 {
		t.Errorf("%d dispatches left", len(env.Process.conditions))
	}
}

// TestSrchandler_psl checks that a continue takes only the signal array
// PSL's low byte: a handler can't change the mode.
func TestSrchandler_psl(t *testing.T) {
	env := conditionFixture(t)
	c := env.cpu
	psl := c.PSL()

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, psl); !ok {
		t.Fatal("not dispatched")
	}

	d := env.Process.conditions[0]
	_, _ = serviceSysSrchandler(env, nil)

	putLongword(t, env, d.sig+4*3, 0x03C0000F) // user mode, N Z V C
	c.SetGPR(vax.R0, 1)

	if _, err := serviceSysSrchandler(env, nil); err != nil {
		t.Fatal(err)
	}

	if got := c.PSL(); uint32(got) != uint32(psl)&^0xFF|0x0F {
		t.Errorf("PSL %#x, want %#x", uint32(got), uint32(psl)&^0xFF|0x0F)
	}
}

// TestSrchandler_catchAll checks the end of a search nobody continued: a
// severe condition is reported and the image exits through SYS$EXIT.
func TestSrchandler_catchAll(t *testing.T) {
	env := conditionFixture(t)
	out := &strings.Builder{}
	env.consoleOut = out
	c := env.cpu

	putLongword(t, env, condFrameA, 0) // no handlers at all
	putLongword(t, env, condFrameB, 0)

	psl := vax.PSL(0x0000001B)
	if ok, _ := env.DispatchException(scbAccessViol, []uint32{0x1234, 4}, 0x3000, psl); !ok {
		t.Fatal("not dispatched")
	}

	d := env.Process.conditions[0]

	r0, err := serviceSysSrchandler(env, nil)

	call, ok := err.(*CallRequest)
	if !ok || call.Routine != exitEntryAddr {
		t.Fatalf("SYS$SRCHANDLER = %#x, %v; want a call of SYS$EXIT", r0, err)
	}

	if args := longwordsAt(t, env, call.ArgList, 2); args[0] != 1 || args[1] != 0x1000000C {
		t.Errorf("$EXIT's arguments %#x, want 1, 0x1000000C (ACCVIO, message inhibited)", args)
	}

	if call.ArgList >= d.sp || c.GPR(vax.SP) != call.ArgList {
		t.Errorf("$EXIT's argument list %#x, SP %#x; want both below the dispatch's %#x", call.ArgList, c.GPR(vax.SP), d.sp)
	}

	want := "%SYSTEM-F-ACCVIO, access violation, reason mask=04, virtual address=00001234, PC=00003000, PS=0000001B\n"
	if out.String() != want {
		t.Errorf("output %q\nwant   %q", out.String(), want)
	}

	if len(env.Process.conditions) != 0 {
		t.Errorf("%d dispatches left", len(env.Process.conditions))
	}
}

// TestSrchandler_unreadableFrame checks that a frame that can't be read
// ends the walk, like the end of the chain.
func TestSrchandler_unreadableFrame(t *testing.T) {
	env := conditionFixture(t)
	env.consoleOut = &strings.Builder{}

	putLongword(t, env, condFrameA, 0)
	putLongword(t, env, condFrameA+12, 0x7FFF0000) // beyond memory

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, env.cpu.PSL()); !ok {
		t.Fatal("not dispatched")
	}

	if _, err := serviceSysSrchandler(env, nil); err == nil {
		t.Fatal("expected the catch-all's $EXIT call")
	} else if call, ok := err.(*CallRequest); !ok || call.Routine != exitEntryAddr {
		t.Errorf("SYS$SRCHANDLER = %v; want the catch-all's $EXIT call", err)
	}
}

// TestSrchandler_nested checks that a condition in a handler is
// dispatched on its own, and the first dispatch resumes after it.
func TestSrchandler_nested(t *testing.T) {
	env := conditionFixture(t)
	c := env.cpu

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, c.PSL()); !ok {
		t.Fatal("not dispatched")
	}

	outer := env.Process.conditions[0]
	_, _ = serviceSysSrchandler(env, nil) // calls condH1

	// Inside condH1 (a new frame, below the outer dispatch's stack), a
	// reserved operand; the handler's own frame has no handler, and its
	// caller is frame B.
	const handlerFrame = 0x12000

	putLongword(t, env, handlerFrame, 0)
	putLongword(t, env, handlerFrame+12, condFrameB)
	c.SetGPR(vax.FP, handlerFrame)
	c.SetGPR(vax.SP, outer.sp-0x100)

	if ok, _ := env.DispatchException(scbReservedAddr, nil, 0x5010, c.PSL()); !ok {
		t.Fatal("inner not dispatched")
	}

	r0, err := serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH2, condFrameB, 1)

	c.SetGPR(vax.R0, 1)

	if _, err := serviceSysSrchandler(env, nil); err != nil {
		t.Fatal(err)
	}

	if c.GPR(vax.PC) != 0x5010 || c.GPR(vax.SP) != outer.sp-0x100 || len(env.Process.conditions) != 1 {
		t.Fatalf("after the inner continue: PC %#x SP %#x, %d dispatches", c.GPR(vax.PC), c.GPR(vax.SP), len(env.Process.conditions))
	}

	// condH1 returns to the outer dispatch and continues.
	c.SetGPR(vax.SP, outer.sp)
	c.SetGPR(vax.FP, condFrameA)
	c.SetGPR(vax.R0, 1)

	if _, err := serviceSysSrchandler(env, nil); err != nil || c.GPR(vax.PC) != 0x3000 {
		t.Errorf("outer continue: %v, PC %#x; want PC 0x3000", err, c.GPR(vax.PC))
	}
}

// TestSrchandler_noDispatch checks that reaching SYS$SRCHANDLER with
// nothing to dispatch is an emulator error, not a guess.
func TestSrchandler_noDispatch(t *testing.T) {
	env := conditionFixture(t)

	if _, err := serviceSysSrchandler(env, nil); err == nil {
		t.Error("expected an error")
	}
}

// TestImageRundown_conditions checks that rundown forgets dispatches.
func TestImageRundown_conditions(t *testing.T) {
	env := conditionFixture(t)

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, env.cpu.PSL()); !ok {
		t.Fatal("not dispatched")
	}

	env.ImageRundown()

	if len(env.Process.conditions) != 0 {
		t.Errorf("%d dispatches left", len(env.Process.conditions))
	}
}

func TestSetexv(t *testing.T) {
	env := conditionFixture(t)
	setMode(env, vax.Supervisor, vax.Supervisor, condSP)

	const prvhnd = 0x6000

	putLongword(t, env, prvhnd, 0xFFFF)

	// Set supervisor's secondary vector (acmode kernel is maximized to
	// the caller's supervisor), then replace it.
	wantR0(t, callLNM(t, env, serviceSysSetexv, 1, 0x5200, 0, prvhnd), ssNormal)

	if got := env.Process.exceptionVectors[vax.Supervisor][vectorSecondary]; got != 0x5200 {
		t.Fatalf("supervisor secondary = %#x, want 0x5200", got)
	}

	if longwordsAt(t, env, prvhnd, 1)[0] != 0 {
		t.Error("prvhnd should hold the previous (empty) vector")
	}

	wantR0(t, callLNM(t, env, serviceSysSetexv, 1, 0x5300, 2, prvhnd), ssNormal)

	if longwordsAt(t, env, prvhnd, 1)[0] != 0x5200 {
		t.Error("prvhnd should hold the replaced handler 0x5200")
	}

	// Omitted address: clear the (default, primary) vector of user mode.
	wantR0(t, callLNM(t, env, serviceSysSetexv, 0, 0x5400, 3), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysSetexv, 0, 0, 3, prvhnd), ssNormal)

	if env.Process.exceptionVectors[vax.User][vectorPrimary] != 0 || longwordsAt(t, env, prvhnd, 1)[0] != 0x5400 {
		t.Error("clearing user's primary vector")
	}

	// Errors change nothing.
	wantR0(t, callLNM(t, env, serviceSysSetexv, 3, 0x5500), ssBadParam)
	wantR0(t, callLNM(t, env, serviceSysSetexv, 1, 0x5500, 2, 0x7FFF0000), ssAccVio)

	if got := env.Process.exceptionVectors[vax.Supervisor][vectorSecondary]; got != 0x5300 {
		t.Errorf("supervisor secondary = %#x after errors, want 0x5300", got)
	}
}

// TestSrchandler_vectors walks a search through all three vectors: the
// primary and secondary before the frames, the last chance after.
func TestSrchandler_vectors(t *testing.T) {
	env := conditionFixture(t)
	c := env.cpu
	mode := c.PSL().CurMod()
	v := &env.Process.exceptionVectors[mode]
	v[vectorPrimary], v[vectorSecondary], v[vectorLastChance] = 0x5200, 0x5300, 0x5400

	// Another mode's vectors are never searched.
	env.Process.exceptionVectors[(mode+1)&3] = [3]uint32{0x6200, 0x6300, 0x6400}

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, c.PSL()); !ok {
		t.Fatal("not dispatched")
	}

	steps := []struct {
		handler, frame uint32
		depth          int32
	}{
		{0x5200, condFrameA, depthPrimary},
		{0x5300, condFrameA, depthSecondary},
		{condH1, condFrameA, 0},
		{condH2, condFrameB, 1},
		{0x5400, condFrameA, depthLastChance},
	}

	for i, st := range steps {
		if i > 0 {
			c.SetGPR(vax.R0, ssResignal)
		}

		r0, err := serviceSysSrchandler(env, nil)
		expectHandlerCall(t, env, r0, err, st.handler, st.frame, st.depth)
	}

	// The last-chance handler continues.
	c.SetGPR(vax.R0, 1)

	if _, err := serviceSysSrchandler(env, nil); err != nil || c.GPR(vax.PC) != 0x3000 {
		t.Errorf("continue: %v, PC %#x", err, c.GPR(vax.PC))
	}
}

// TestImageRundown_exceptionVectors checks that rundown clears only the
// user-mode vectors.
func TestImageRundown_exceptionVectors(t *testing.T) {
	env, _ := fixture()
	env.Process.exceptionVectors[vax.User] = [3]uint32{1, 2, 3}
	env.Process.exceptionVectors[vax.Supervisor] = [3]uint32{4, 5, 6}

	env.ImageRundown()

	if env.Process.exceptionVectors[vax.User] != [3]uint32{} || env.Process.exceptionVectors[vax.Supervisor] != [3]uint32{4, 5, 6} {
		t.Errorf("vectors after rundown: %v", env.Process.exceptionVectors)
	}
}
