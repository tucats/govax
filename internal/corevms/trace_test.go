package corevms

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// The trace fault as a condition (docs/PHASE-73 - trace trap.md, subtask
// 2): SS$_TBIT, its handlers run with T and TP clear, and continuing
// takes T from the signal array's PSL.

// tracedPSL is a user-mode PSL with PSL<T> set, as a program tracing
// itself has it when the trace fault comes (the fault has cleared TP).
func tracedPSL() registers.PSL {
	psl := registers.PSL(0x03C00000)
	psl.SetT(true)

	return psl
}

// TestDispatchException_traceFault: the trace fault is signaled as
// SS$_TBIT with no detail, the signal array keeps the exception's PSL
// (T set), and the handler search runs untraced.
func TestDispatchException_traceFault(t *testing.T) {
	env := conditionFixture(t)
	c := env.cpu
	psl := tracedPSL()
	c.SetPSL(psl)

	ok, err := env.DispatchException(scbTracePending, nil, 0x3002, psl)
	if !ok || err != nil {
		t.Fatalf("DispatchException = %v, %v; want dispatched", ok, err)
	}

	d := env.Process.conditions[0]
	if sig := longwordsAt(t, env, d.sig, 4); sig[0] != 3 || sig[1] != 0x464 || sig[2] != 0x3002 || sig[3] != uint32(psl) {
		t.Errorf("signal array %#x; want 3, SS$_TBIT (0x464), PC 0x3002, PSL %#x", sig, uint32(psl))
	}

	if live := c.PSL(); live.T() || live.TP() {
		t.Errorf("PSL during the search %#x; want T and TP clear", uint32(live))
	}
}

// TestSrchandler_traceContinue: a handler that continues from SS$_TBIT
// leaving the signal array alone goes on tracing (T set again, TP clear
// so the next instruction runs before the next trace); one that clears T
// in the signal array's PSL stops it; one that moves the PC is obeyed.
func TestSrchandler_traceContinue(t *testing.T) {
	cases := []struct {
		name  string
		pc    uint32 // the PC the handler leaves, 0 for unchanged
		clear bool   // the handler clears T in the signal array's PSL
		wantT bool
	}{
		{"tracing goes on", 0, false, true},
		{"tracing stops", 0, true, false},
		{"resumed elsewhere", 0x4000, false, true},
	}

	for _, tc := range cases {
		env := conditionFixture(t)
		c := env.cpu
		psl := tracedPSL()
		c.SetPSL(psl)

		if ok, _ := env.DispatchException(scbTracePending, nil, 0x3002, psl); !ok {
			t.Fatalf("%s: not dispatched", tc.name)
		}

		d := env.Process.conditions[0]

		r0, err := serviceSysSrchandler(env, nil)
		expectHandlerCall(t, env, r0, err, condH1, condFrameA, 0)

		wantPC := uint32(0x3002)
		if tc.pc != 0 {
			wantPC = tc.pc
			putLongword(t, env, d.sig+4*2, tc.pc)
		}

		if tc.clear {
			sigPSL := psl
			sigPSL.SetT(false)
			putLongword(t, env, d.sig+4*3, uint32(sigPSL))
		}

		c.SetGPR(registers.R0, 1) // SS$_CONTINUE

		if _, err := serviceSysSrchandler(env, nil); err != nil {
			t.Fatalf("%s: continue: %v", tc.name, err)
		}

		got := c.PSL()
		if got.T() != tc.wantT || got.TP() || c.GPR(registers.PC) != wantPC {
			t.Errorf("%s: PC %#x PSL %#x (T %v, TP %v); want PC %#x, T %v, TP clear",
				tc.name, c.GPR(registers.PC), uint32(got), got.T(), got.TP(), wantPC, tc.wantT)
		}
	}
}

// TestSrchandler_trapKeepsTracePending: an exception taken by a trap
// (here an arithmetic trap) in traced code keeps TP in its PSL. The
// handlers run with TP clear, and continuing brings TP back, so the
// trace of the trapping instruction comes as the program resumes.
func TestSrchandler_trapKeepsTracePending(t *testing.T) {
	env := conditionFixture(t)
	c := env.cpu
	psl := tracedPSL()
	psl.SetTP(true)
	c.SetPSL(psl)

	if ok, _ := env.DispatchException(scbArithmetic, []uint32{2}, 0x3004, psl); !ok {
		t.Fatal("not dispatched")
	}

	if live := c.PSL(); live.T() || live.TP() {
		t.Errorf("PSL during the search %#x; want T and TP clear", uint32(live))
	}

	_, _ = serviceSysSrchandler(env, nil)
	c.SetGPR(registers.R0, 1)

	if _, err := serviceSysSrchandler(env, nil); err != nil {
		t.Fatal(err)
	}

	if got := c.PSL(); !got.T() || !got.TP() {
		t.Errorf("PSL after continuing %#x; want T and TP set", uint32(got))
	}
}

// TestSrchandler_traceUnhandled: an SS$_TBIT nobody takes is a severe
// condition, so the catch-all writes its message and ends the image
// (Decision 4; how VMS ends it is the probe's question).
func TestSrchandler_traceUnhandled(t *testing.T) {
	env := conditionFixture(t)
	out := &strings.Builder{}
	env.consoleOut = out

	putLongword(t, env, condFrameA, 0) // no handlers at all
	putLongword(t, env, condFrameB, 0)

	psl := tracedPSL()
	env.cpu.SetPSL(psl)

	if ok, _ := env.DispatchException(scbTracePending, nil, 0x3002, psl); !ok {
		t.Fatal("not dispatched")
	}

	r0, err := serviceSysSrchandler(env, nil)

	call, ok := err.(*CallRequest)
	if !ok || call.Routine != exitEntryAddr {
		t.Fatalf("SYS$SRCHANDLER = %#x, %v; want a call of SYS$EXIT", r0, err)
	}

	if args := longwordsAt(t, env, call.ArgList, 2); args[1] != 0x10000464 {
		t.Errorf("$EXIT's status %#x, want 0x10000464 (TBIT, message inhibited)", args[1])
	}

	want := "%SYSTEM-F-TBIT, T-bit pending trap at PC=00003002, PS=03C00010"
	if !strings.HasPrefix(out.String(), want) {
		t.Errorf("output %q\nwant it to start %q", out.String(), want)
	}
}
