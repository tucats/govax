package corevms

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// Addresses for a shim's own call frame: the stub's frame, called from
// frame A (conditionFixture's innermost frame) with return address
// signalReturn, and the stub's RET, where PC is while the shim runs.
const (
	signalFrame  = 0x12000
	signalReturn = 0x3010
	signalRET    = 0x4000
)

// signalFixture is conditionFixture with a LIB$ shim's stub frame on top
// of frame A, as the shim sees it while its XFC runs.
func signalFixture(t *testing.T) (*Environment, *strings.Builder) {
	t.Helper()

	env := conditionFixture(t)
	out := &strings.Builder{}
	env.consoleOut = out

	putLongword(t, env, signalFrame, 0)
	putLongword(t, env, signalFrame+frameMaskPSW, 0x0000000C) // the caller's N and Z
	putLongword(t, env, signalFrame+frameSavedFP, condFrameA)
	putLongword(t, env, signalFrame+frameSavedPC, signalReturn)

	env.cpu.SetGPR(registers.FP, signalFrame)
	env.cpu.SetGPR(registers.PC, signalRET)

	return env, out
}

// userWarning is a condition value of a customer facility (bit 27 set),
// severity WARNING (0), with no message text.
const userWarning = 0x08018000

func TestLibSignal_continue(t *testing.T) {
	env, _ := signalFixture(t)
	c := env.cpu
	psl := c.PSL()

	r0, err := env.Signal([]uint32{userWarning, 1, 42}, false)
	if err != nil || r0 != 0 || c.GPR(registers.PC) != condStub {
		t.Fatalf("LIB$SIGNAL = %#x, %v, PC %#x; want the jump to SYS$SRCHANDLER", r0, err, c.GPR(registers.PC))
	}

	d := env.Process.conditions[0]

	sig := longwordsAt(t, env, d.sig, 6)
	want := []uint32{5, userWarning, 1, 42, signalReturn, uint32(psl)&^0xFFFF | 0x0C}

	for i, w := range want {
		if sig[i] != w {
			t.Errorf("signal array[%d] = %#x, want %#x", i, sig[i], w)
		}
	}

	// The search starts at the caller's frame, depth 0.
	r0, err = serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH1, condFrameA, 0)

	putLongword(t, env, d.mech+12, 0x77)
	c.SetGPR(registers.R0, 1)

	r0, err = serviceSysSrchandler(env, nil)
	if err != nil || r0 != 0x77 {
		t.Fatalf("continue = %#x, %v; want R0 0x77", r0, err)
	}

	if c.GPR(registers.PC) != signalRET || c.GPR(registers.FP) != signalFrame || c.GPR(registers.SP) != condSP {
		t.Errorf("resumed at PC %#x FP %#x SP %#x; want the stub's RET %#x, FP %#x, SP %#x",
			c.GPR(registers.PC), c.GPR(registers.FP), c.GPR(registers.SP), signalRET, signalFrame, condSP)
	}
}

// TestLibSignal_catchAllContinues checks that an unhandled condition
// that isn't severe is reported, and LIB$SIGNAL returns.
func TestLibSignal_catchAllContinues(t *testing.T) {
	env, out := signalFixture(t)
	c := env.cpu

	putLongword(t, env, condFrameA, 0)
	putLongword(t, env, condFrameB, 0)

	if _, err := env.Signal([]uint32{0x870}, false); err != nil { // SS$_ENDOFFILE: W
		t.Fatal(err)
	}

	if _, err := serviceSysSrchandler(env, nil); err != nil {
		t.Fatalf("catch-all: %v", err)
	}

	if out.String() != "%SYSTEM-W-ENDOFFILE, end of file\n" {
		t.Errorf("output %q", out.String())
	}

	if c.GPR(registers.PC) != signalRET || len(env.Process.conditions) != 0 {
		t.Errorf("PC %#x, %d dispatches; want LIB$SIGNAL returning", c.GPR(registers.PC), len(env.Process.conditions))
	}
}

// TestLibStop checks LIB$STOP's forced severity, and that continuing it
// exits the image.
func TestLibStop(t *testing.T) {
	env, out := signalFixture(t)
	c := env.cpu

	if _, err := env.Signal([]uint32{userWarning}, true); err != nil {
		t.Fatal(err)
	}

	d := env.Process.conditions[0]
	if got := longwordsAt(t, env, d.sig+4, 1)[0]; got != userWarning|severitySevere {
		t.Errorf("signaled condition %#x, want severity forced to SEVERE: %#x", got, userWarning|severitySevere)
	}

	_, _ = serviceSysSrchandler(env, nil) // calls condH1

	c.SetGPR(registers.R0, 1) // which tries to continue

	_, err := serviceSysSrchandler(env, nil)

	call, ok := err.(*CallRequest)
	if !ok || call.Routine != exitEntryAddr {
		t.Fatalf("continuing a stop = %v; want a call of SYS$EXIT", err)
	}

	if args := longwordsAt(t, env, call.ArgList, 2); args[1] != userWarning|severitySevere|stsInhibitMsg {
		t.Errorf("exit status %#x", args[1])
	}

	if out.String() != "%LIB-F-ATTCONSTO, attempt to continue from stop\n" {
		t.Errorf("output %q", out.String())
	}
}

// TestLibStop_catchAllExits checks that an unhandled stop of a
// condition that was a warning still ends the image.
func TestLibStop_catchAllExits(t *testing.T) {
	env, _ := signalFixture(t)

	putLongword(t, env, condFrameA, 0)
	putLongword(t, env, condFrameB, 0)

	if _, err := env.Signal([]uint32{0x870}, true); err != nil {
		t.Fatal(err)
	}

	if _, err := serviceSysSrchandler(env, nil); err == nil {
		t.Fatal("expected the catch-all's $EXIT call")
	} else if call, ok := err.(*CallRequest); !ok || call.Routine != exitEntryAddr {
		t.Errorf("catch-all = %v", err)
	}
}

func TestLibSignal_errors(t *testing.T) {
	env, _ := signalFixture(t)

	if _, err := env.Signal(nil, false); err == nil {
		t.Error("no condition value: expected an error")
	}

	putLongword(t, env, condStub, 0) // no SYS$SRCHANDLER stub

	if _, err := env.Signal([]uint32{userWarning}, false); err == nil {
		t.Error("no stub: expected an error")
	}
}

func TestLibEstablishRevert(t *testing.T) {
	env, _ := signalFixture(t)

	old, err := env.SetCallerHandler("LIB$ESTABLISH", 0x5555)
	if err != nil || old != condH1 {
		t.Fatalf("LIB$ESTABLISH = %#x, %v; want frame A's old handler %#x", old, err, condH1)
	}

	if got := longwordsAt(t, env, condFrameA, 1)[0]; got != 0x5555 {
		t.Errorf("frame A's handler %#x, want 0x5555", got)
	}

	old, err = env.SetCallerHandler("LIB$REVERT", 0)
	if err != nil || old != 0x5555 {
		t.Fatalf("LIB$REVERT = %#x, %v; want 0x5555", old, err)
	}

	if got := longwordsAt(t, env, condFrameA, 1)[0]; got != 0 {
		t.Errorf("frame A's handler %#x, want 0", got)
	}

	if got := longwordsAt(t, env, signalFrame, 1)[0]; got != 0 {
		t.Errorf("the stub's own frame was changed: %#x", got)
	}
}

// TestLibSignal_traced: LIB$SIGNAL called with PSL<T> set (from code
// that traces itself) calls no handler: it writes the condition's message
// and ends the image with the condition as its status, T and TP cleared,
// as VMS 7.1 does (testdata/trace73's round 2, part G).
func TestLibSignal_traced(t *testing.T) {
	env, out := signalFixture(t)
	c := env.cpu

	psl := c.PSL()
	psl.SetT(true)
	psl.SetTP(true)
	c.SetPSL(psl)

	_, err := env.Signal([]uint32{0x870}, false) // SS$_ENDOFFILE: W

	call, ok := err.(*CallRequest)
	if !ok || call.Routine != exitEntryAddr {
		t.Fatalf("traced LIB$SIGNAL = %v; want a call of SYS$EXIT", err)
	}

	if args := longwordsAt(t, env, call.ArgList, 2); args[1] != 0x870|stsInhibitMsg {
		t.Errorf("exit status %#x, want %#x", args[1], 0x870|stsInhibitMsg)
	}

	if out.String() != "%SYSTEM-W-ENDOFFILE, end of file\n" {
		t.Errorf("output %q", out.String())
	}

	if got := c.PSL(); got.T() || got.TP() || len(env.Process.conditions) != 0 {
		t.Errorf("PSL %#x, %d dispatches; want T and TP clear, none", uint32(got), len(env.Process.conditions))
	}
}
