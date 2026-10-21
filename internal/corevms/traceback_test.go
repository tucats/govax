package corevms

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// tracebackFixture dispatches a reserved operand from the condition
// fixture's frames, with frame A's handler being called, and then makes
// the condition cond (as a handler may change it). It returns the
// dispatch and the traceback handler's arguments as the dispatcher
// passes them: the signal and mechanism arrays.
func tracebackFixture(t *testing.T, cond uint32) (*Environment, *strings.Builder, *conditionDispatch, []uint32) {
	t.Helper()

	env := conditionFixture(t)
	out := &strings.Builder{}
	env.consoleOut = out

	if ok, _ := env.DispatchException(scbReservedOp, nil, 0x3000, env.cpu.PSL()); !ok {
		t.Fatal("not dispatched")
	}

	r0, err := serviceSysSrchandler(env, nil)
	expectHandlerCall(t, env, r0, err, condH1, condFrameA, 0)

	d := env.Process.conditions[0]
	putLongword(t, env, d.sig+4, cond)

	return env, out, d, []uint32{d.sig, d.mech}
}

// TestTraceback_warningAndError checks that a warning and an error get
// their message and are continued.
func TestTraceback_warningAndError(t *testing.T) {
	for _, tc := range []struct {
		cond uint32
		want string
	}{
		{0x870, "%SYSTEM-W-ENDOFFILE, end of file\n"},
		{0x1827A, "%RMS-E-EOF, end of file detected\n"}, // RMS$_EOF
	} {
		env, out, _, argv := tracebackFixture(t, tc.cond)

		r0, err := shimTraceback(env, argv)
		if err != nil || r0 != ssContinue {
			t.Errorf("%#x: %#x, %v; want SS$_CONTINUE", tc.cond, r0, err)
		}

		if out.String() != tc.want {
			t.Errorf("%#x: output %q, want %q", tc.cond, out.String(), tc.want)
		}

		// The dispatcher continues the program, and the catch-all never
		// shows the message again.
		env.cpu.SetGPR(registers.R0, r0)

		if _, err := serviceSysSrchandler(env, nil); err != nil || env.cpu.GPR(registers.PC) != 0x3000 {
			t.Errorf("%#x: continue: %v, PC %#x", tc.cond, err, env.cpu.GPR(registers.PC))
		}
	}
}

// TestTraceback_severe checks that a severe condition gets its message
// and ends the image, by a tail call of $EXIT with the condition marked
// STS$M_INHIB_MSG, the dispatch over.
func TestTraceback_severe(t *testing.T) {
	env, out, _, argv := tracebackFixture(t, 0x0C) // SS$_ACCVIO with no detail: its message's arguments are missing

	_, err := shimTraceback(env, argv)

	call, ok := err.(*CallRequest)
	if !ok || call.Routine != exitEntryAddr || !call.Tail {
		t.Fatalf("shimTraceback: %v; want a tail call of SYS$EXIT", err)
	}

	if args := longwordsAt(t, env, call.ArgList, 2); args[0] != 1 || args[1] != 0x1000000C {
		t.Errorf("$EXIT's arguments %#x, want 1, 0x1000000C", args)
	}

	if !strings.HasPrefix(out.String(), "%SYSTEM-F-ACCVIO,") {
		t.Errorf("output %q, want the ACCVIO message", out.String())
	}

	if len(env.Process.conditions) != 0 {
		t.Errorf("%d dispatches left", len(env.Process.conditions))
	}
}

// TestTraceback_resignals checks the conditions the handler passes on:
// success, informational, SS$_DEBUG, any while the image exits, and a
// call that isn't the dispatcher's. None shows a message.
func TestTraceback_resignals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cond    uint32
		exiting bool
		argv    func([]uint32) []uint32
	}{
		{name: "success", cond: 0x01},
		{name: "informational", cond: 0x0B},
		{name: "SS$_DEBUG", cond: ssDebug},
		{name: "exiting", cond: 0x870, exiting: true},
		{name: "not the dispatcher's", cond: 0x870, argv: func(a []uint32) []uint32 { return []uint32{a[0], a[0]} }},
	} {
		env, out, _, argv := tracebackFixture(t, tc.cond)
		env.Process.exiting = tc.exiting

		if tc.argv != nil {
			argv = tc.argv(argv)
		}

		if r0, err := shimTraceback(env, argv); err != nil || r0 != ssResignal {
			t.Errorf("%s: %#x, %v; want SS$_RESIGNAL", tc.name, r0, err)
		}

		if out.Len() != 0 {
			t.Errorf("%s: output %q, want none", tc.name, out.String())
		}
	}
}

// TestTraceback_inhibited checks that a condition marked STS$M_INHIB_MSG
// has no message, but is still taken (testdata/tb70's S5).
func TestTraceback_inhibited(t *testing.T) {
	env, out, _, argv := tracebackFixture(t, 0x10000870)

	if r0, err := shimTraceback(env, argv); err != nil || r0 != ssContinue {
		t.Errorf("%#x, %v; want SS$_CONTINUE", r0, err)
	}

	if out.Len() != 0 {
		t.Errorf("output %q, want none", out.String())
	}
}

// TestExit_exiting checks that $EXIT marks the image exiting, and image
// rundown clears it.
func TestExit_exiting(t *testing.T) {
	env, _ := fixture()

	if _, err := serviceSysExit(env, []uint32{1}); err != ErrExit {
		t.Fatalf("$EXIT: %v", err)
	}

	if !env.Process.exiting {
		t.Error("not exiting after $EXIT")
	}

	env.ImageRundown()

	if env.Process.exiting {
		t.Error("still exiting after rundown")
	}
}
