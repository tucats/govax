package corevms

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// The engine's half of a callback (the frame built, the procedure run)
// is tested end to end by internal/console's TestCallback_assembledProgram
// (testdata/asm/callback.asm). These tests drive the dispatch's half
// directly: env.Shim as the XFC calls it, with the registers set as the
// stub's frame and the procedure's RET would leave them.

const (
	callbackTestCode = 999
	callbackStubFP   = 0x8000
	callbackStubSP   = 0x7F00
	callbackStubPC   = 0x2010 // just past the stub's XFC
	callbackRoutine  = 0x3000 // the procedure's entry mask
)

// callbackFixture is an environment whose CPU is in a stub's frame, about
// to run the shim under callbackTestCode, registered with fn.
func callbackFixture(fn ShimFunc) *Environment {
	env, _ := fixture()
	env.Shims().RegisterCallback(callbackTestCode, "TEST$CALLBACK", fn)

	c := env.cpu
	c.SetGPR(registers.FP, callbackStubFP)
	c.SetGPR(registers.SP, callbackStubSP)
	c.SetGPR(registers.PC, callbackStubPC)
	c.SetGPR(registers.AP, 0x7000) // an empty argument list (memory is zero)

	return env
}

// TestCallback_resumeAndRundown: a routine's callback is a CallRequest
// with the argument list below SP; the XFC run again with the frame the
// same continues it with R0, whatever code R0 would select; and image
// rundown abandons a routine left waiting for a callback, running its
// deferred code.
func TestCallback_resumeAndRundown(t *testing.T) {
	var got []uint32

	abandoned := false

	env := callbackFixture(func(env *Environment, argv []uint32) (uint32, error) {
		returned := false

		defer func() { abandoned = !returned }()

		for i := uint32(1); i <= 3; i++ {
			r0, err := env.Callback(callbackRoutine, i, 0x55)
			if err != nil {
				return 0, err
			}

			got = append(got, r0)
		}

		returned = true

		return 0x1234, nil
	})
	c := env.cpu

	// The first callback: a plain call (not a tail call), its argument
	// list (3 longwords) below the stub's SP, and SP moved to it.
	_, handled, err := env.Shim(callbackTestCode)

	var call *CallRequest
	if !handled || !errors.As(err, &call) || call.Tail || call.Image {
		t.Fatalf("Shim = handled %v, %v; want a plain CallRequest", handled, err)
	}

	if call.Routine != callbackRoutine || call.ArgList != callbackStubSP-12 || c.GPR(registers.SP) != call.ArgList {
		t.Fatalf("CallRequest %+v, SP %#x; want routine %#x, argument list and SP %#x",
			call, c.GPR(registers.SP), callbackRoutine, callbackStubSP-12)
	}

	for i, want := range []uint32{2, 1, 0x55} {
		v, _ := env.mem.LoadLongword(c, call.ArgList+uint32(4*i))
		if v != want {
			t.Errorf("argument list longword %d = %#x, want %#x", i, v, want)
		}
	}

	// The procedure returns 7 (a code that selects some other shim, or
	// none): the XFC continues the routine, which asks for its second
	// callback with the same list (SP put back first).
	c.SetGPR(registers.R0, 7)

	if _, _, err = env.Shim(7); !errors.As(err, &call) || call.ArgList != callbackStubSP-12 {
		t.Fatalf("second Shim: %v, want the second callback", err)
	}

	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("the routine got %v, want [7]", got)
	}

	// The image ends before the second callback returns.
	env.ImageRundown()

	if !abandoned {
		t.Error("image rundown didn't abandon the routine")
	}

	if n := len(env.Process.shimCalls); n != 0 || env.Process.runningShim != nil {
		t.Errorf("%d calls still waiting (running %v), want none", n, env.Process.runningShim)
	}

	// A new call starts the routine afresh, and three returns finish it.
	c.SetGPR(registers.SP, callbackStubSP)

	got, abandoned = nil, false

	r0, _, err := env.Shim(callbackTestCode)
	for n := uint32(10); errors.As(err, &call); n++ {
		c.SetGPR(registers.R0, n)
		r0, _, err = env.Shim(n)
	}

	if err != nil || r0 != 0x1234 || abandoned || len(got) != 3 || got[2] != 12 {
		t.Errorf("Shim = %#x, %v (got %v, abandoned %v); want 0x1234 after [10 11 12]", r0, err, got, abandoned)
	}

	if c.GPR(registers.SP) != callbackStubSP {
		t.Errorf("SP %#x after the routine, want the stub's %#x", c.GPR(registers.SP), callbackStubSP)
	}
}

// TestCallback_poppedFrame: a routine waiting for a callback whose stack
// has been popped past (its frame removed some way the dispatch didn't
// see) is abandoned when the next shim is called, and the shim R0 names
// runs.
func TestCallback_poppedFrame(t *testing.T) {
	abandoned := false

	env := callbackFixture(func(env *Environment, argv []uint32) (uint32, error) {
		returned := false

		defer func() { abandoned = !returned }()

		_, err := env.Callback(callbackRoutine)
		returned = true

		return 0, err
	})

	if _, _, err := env.Shim(callbackTestCode); err == nil {
		t.Fatal("Shim: no callback")
	}

	ran := false

	env.Shims().Register(998, "TEST$PLAIN", func(*Environment, []uint32) (uint32, error) {
		ran = true

		return 1, nil
	})

	env.cpu.SetGPR(registers.SP, callbackStubSP+0x100)

	if r0, _, err := env.Shim(998); err != nil || r0 != 1 || !ran {
		t.Errorf("Shim(998) = %d, %v (ran %v); want TEST$PLAIN's 1", r0, err, ran)
	}

	if !abandoned || len(env.Process.shimCalls) != 0 {
		t.Errorf("abandoned %v, %d calls waiting; want the popped one abandoned", abandoned, len(env.Process.shimCalls))
	}
}

// TestCallback_waitAfterCallback: a routine that waits after calling
// back fails, since the XFC run again would start it over; one that
// waits before calling back is run again, as any shim is.
func TestCallback_waitAfterCallback(t *testing.T) {
	env := callbackFixture(func(env *Environment, argv []uint32) (uint32, error) {
		if _, err := env.Callback(callbackRoutine); err != nil {
			return 0, err
		}

		return 0, ErrWait
	})

	if _, _, err := env.Shim(callbackTestCode); err == nil {
		t.Fatal("Shim: no callback")
	}

	env.cpu.SetGPR(registers.R0, 1)

	if _, _, err := env.Shim(1); err == nil || !strings.Contains(err.Error(), "can't wait") {
		t.Errorf("Shim after the callback: %v, want can't wait", err)
	}

	waits := callbackFixture(func(*Environment, []uint32) (uint32, error) { return 0, ErrWait })

	if _, _, err := waits.Shim(callbackTestCode); !errors.Is(err, ErrWait) {
		t.Errorf("Shim: %v, want ErrWait", err)
	}
}

// TestCallback_outsideRoutine: Callback from a routine not registered
// with RegisterCallback is a mistake in the caller, and panics.
func TestCallback_outsideRoutine(t *testing.T) {
	env, _ := fixture()

	defer func() {
		if recover() == nil {
			t.Error("Callback outside a callback routine didn't panic")
		}
	}()

	_, _ = env.Callback(callbackRoutine)
}
