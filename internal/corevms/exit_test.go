package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// exitBlock lays out an exit control block for handler with the given
// argument count, the first argument being the address of a status
// longword. It returns the block's address and the status longword's.
func exitBlock(a *arena, handler uint32, argCount byte) (desblk, statusAdr uint32) {
	desblk, statusAdr = a.alloc(20), a.alloc(4)

	putLongword(a.t, a.env, desblk+exhLink, 0xDEADBEEF)
	putLongword(a.t, a.env, desblk+exhHandler, handler)
	putLongword(a.t, a.env, desblk+exhArgList, uint32(argCount))
	putLongword(a.t, a.env, desblk+exhStatusAdr, statusAdr)
	putLongword(a.t, a.env, statusAdr, 0)

	return desblk, statusAdr
}

func TestServiceSysDclexh(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	first, _ := exitBlock(a, 0x4000, 1)
	second, _ := exitBlock(a, 0x5000, 1)

	// Kernel mode has no exit handlers.
	wantR0(t, callLNM(t, env, serviceSysDclexh, first), ssIvSsRq)

	setCurMod(env, vax.User)
	wantR0(t, callLNM(t, env, serviceSysDclexh), ssNoHandler)
	wantR0(t, callLNM(t, env, serviceSysDclexh, first), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDclexh, second), ssNormal)

	// Each block links to the one declared before it; the first to 0.
	if got := a.readLong(first + exhLink); got != 0 {
		t.Errorf("first block's link = %#x, want 0", got)
	}

	if got := a.readLong(second + exhLink); got != first {
		t.Errorf("second block's link = %#x, want the first block %#x", got, first)
	}

	if env.ExitHandlers(vax.User) != 2 || env.ExitHandlers(vax.Supervisor) != 0 {
		t.Errorf("handlers: user %d, supervisor %d; want 2 and 0", env.ExitHandlers(vax.User), env.ExitHandlers(vax.Supervisor))
	}

	wantR0(t, callLNM(t, env, serviceSysDclexh, badAddr), ssAccVio)

	if env.ExitHandlers(vax.User) != 2 {
		t.Error("an unwritable block was declared anyway")
	}
}

func TestServiceSysCanexh(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	b1, _ := exitBlock(a, 0x4000, 1)
	b2, _ := exitBlock(a, 0x5000, 1)
	b3, _ := exitBlock(a, 0x6000, 1)

	wantR0(t, callLNM(t, env, serviceSysCanexh, b1), ssIvSsRq) // kernel mode

	setCurMod(env, vax.User)

	for _, b := range []uint32{b1, b2, b3} {
		wantR0(t, callLNM(t, env, serviceSysDclexh, b), ssNormal)
	}

	// Removing the middle block relinks the newer one past it.
	wantR0(t, callLNM(t, env, serviceSysCanexh, b2), ssNormal)

	if got := a.readLong(b3 + exhLink); got != b1 {
		t.Errorf("after cancelling the middle block, the newest links to %#x, want %#x", got, b1)
	}

	wantR0(t, callLNM(t, env, serviceSysCanexh, b2), ssNoHandler) // already gone

	// The newest block has no newer one to relink.
	wantR0(t, callLNM(t, env, serviceSysCanexh, b3), ssNormal)

	if env.ExitHandlers(vax.User) != 1 {
		t.Errorf("%d handlers left, want 1", env.ExitHandlers(vax.User))
	}

	// A newer block whose link can't be written.
	env.Process.exitHandlers[vax.User] = []uint32{b1, badAddr}
	wantR0(t, callLNM(t, env, serviceSysCanexh, b1), ssAccVio)

	// No block: all of the mode's are cancelled.
	wantR0(t, callLNM(t, env, serviceSysCanexh), ssNormal)

	if env.ExitHandlers(vax.User) != 0 {
		t.Errorf("%d handlers left after cancelling all, want 0", env.ExitHandlers(vax.User))
	}
}

// exitCall calls $EXIT, returning what it asked the engine to do: call
// a handler, or end the image with a status.
func exitCall(t *testing.T, env *Environment, argv ...uint32) (call *CallRequest, status uint32, exited bool) {
	t.Helper()

	r0, err := serviceSysExit(env, argv)

	switch {
	case errors.As(err, &call):
		return call, 0, false
	case errors.Is(err, ErrExit):
		return nil, r0, true
	}

	t.Fatalf("$EXIT = %#x, %v; want a CallRequest or ErrExit", r0, err)

	return nil, 0, false
}

func TestServiceSysExit(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	b1, s1 := exitBlock(a, 0x4000, 1)
	b2, s2 := exitBlock(a, 0x5000, 0) // no arguments: no status stored

	setCurMod(env, vax.User)

	for _, b := range []uint32{b1, b2} {
		wantR0(t, callLNM(t, env, serviceSysDclexh, b), ssNormal)
	}

	// Newest first, each called as CALLG desblk+8, handler.
	call, _, _ := exitCall(t, env, 0x2C)
	if call == nil || call.Routine != 0x5000 || call.ArgList != b2+exhArgList {
		t.Fatalf("first $EXIT call = %+v, want the second block's handler 0x5000 with its argument list", call)
	}

	if got := a.readLong(s2); got != 0 {
		t.Errorf("a handler with no arguments had the status stored: %#x", got)
	}

	// The handler returns to $EXIT's XFC: the next call moves on.
	call, _, _ = exitCall(t, env, 0x2C)
	if call == nil || call.Routine != 0x4000 || call.ArgList != b1+exhArgList {
		t.Fatalf("second $EXIT call = %+v, want the first block's handler 0x4000", call)
	}

	if got := a.readLong(s1); got != 0x2C {
		t.Errorf("status passed to the handler = %#x, want 0x2C", got)
	}

	// No handlers left: the image ends with the status.
	if _, status, exited := exitCall(t, env, 0x2C); !exited || status != 0x2C {
		t.Errorf("last $EXIT: exited=%v status=%#x, want an exit with 0x2C", exited, status)
	}

	if env.Process.ExitStatus != 0x2C {
		t.Errorf("ExitStatus = %#x, want 0x2C", env.Process.ExitStatus)
	}

	// With no argument list at all, the status is SS$_NORMAL.
	if _, status, exited := exitCall(t, env); !exited || status != ssNormal {
		t.Errorf("$EXIT with no arguments: exited=%v status=%#x, want SS$_NORMAL", exited, status)
	}
}

func TestServiceSysExit_skipsUnreadableBlock(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	b1, _ := exitBlock(a, 0x4000, 1)

	setCurMod(env, vax.User)
	env.Process.exitHandlers[vax.User] = []uint32{b1, badAddr}

	if call, _, _ := exitCall(t, env, 1); call == nil || call.Routine != 0x4000 {
		t.Errorf("$EXIT = %+v, want the readable block's handler, the unreadable one skipped", call)
	}
}

func TestServiceSysExit_onlyCallersMode(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	b1, _ := exitBlock(a, 0x4000, 1)

	setCurMod(env, vax.User)
	wantR0(t, callLNM(t, env, serviceSysDclexh, b1), ssNormal)

	// From kernel mode there are no handlers: the image ends at once,
	// and the user-mode handler is still declared.
	setCurMod(env, vax.Kernel)

	if _, status, exited := exitCall(t, env, 3); !exited || status != 3 {
		t.Errorf("kernel $EXIT: exited=%v status=%d, want an immediate exit with 3", exited, status)
	}

	if env.ExitHandlers(vax.User) != 1 {
		t.Error("a kernel-mode $EXIT consumed a user-mode handler")
	}
}

func TestImageRundown_forgetsUserExitHandlers(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	b1, _ := exitBlock(a, 0x4000, 1)
	b2, _ := exitBlock(a, 0x5000, 1)

	setCurMod(env, vax.User)
	wantR0(t, callLNM(t, env, serviceSysDclexh, b1), ssNormal)
	setCurMod(env, vax.Supervisor)
	wantR0(t, callLNM(t, env, serviceSysDclexh, b2), ssNormal)

	env.ImageRundown()

	if env.ExitHandlers(vax.User) != 0 || env.ExitHandlers(vax.Supervisor) != 1 {
		t.Errorf("after rundown: user %d, supervisor %d; want 0 and 1", env.ExitHandlers(vax.User), env.ExitHandlers(vax.Supervisor))
	}
}
