package rtl

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// The IO$_SETMODE functions that enable CTRL/C and CTRL/Y ASTs.
var (
	fnCtrlCAST = vmsdef.IOConstants["IO$_SETMODE"] | vmsdef.IOConstants["IO$M_CTRLCAST"]
	fnCtrlYAST = vmsdef.IOConstants["IO$_SETMODE"] | vmsdef.IOConstants["IO$M_CTRLYAST"]
)

// enableAST calls $QIO function fn (a CTRL/C or CTRL/Y AST request) on
// channel ch, for routine with param in mode.
func enableAST(t *testing.T, env *Environment, ch, fn, routine, param, mode uint32) {
	t.Helper()
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fn, p: [6]uint32{routine, param, mode}}), ssNormal)
}

// queuedASTs returns the queued ASTs' routines, parameters, and modes.
func queuedASTs(env *Environment) []astRequest {
	return append([]astRequest(nil), env.Process.ast.queue...)
}

func TestCtrlCAST(t *testing.T) {
	env, _, _, ch := qioFixture(t, "")

	// Nothing enabled: the key isn't taken.
	if env.Attention(AttentionCtrlC) {
		t.Fatal("CTRL/C taken with no AST enabled")
	}

	// Enabled from user mode, asking for kernel mode: maximized to user.
	setMode(env, vax.User, vax.User, 0x8000)
	enableAST(t, env, ch, fnCtrlCAST, 0x4000, 5, uint32(vax.Kernel))

	if !env.Attention(AttentionCtrlC) {
		t.Fatal("CTRL/C not taken with an AST enabled")
	}

	want := []astRequest{{routine: 0x4000, param: 5, mode: uint32(vax.User)}}
	if got := queuedASTs(env); len(got) != 1 || got[0] != want[0] {
		t.Errorf("queued %+v, want %+v", got, want)
	}

	// One-shot: the next CTRL/C isn't taken.
	if env.Attention(AttentionCtrlC) || env.AttentionASTs() != 0 {
		t.Error("the CTRL/C AST stayed enabled after delivery")
	}

	// A CTRL/C AST doesn't catch CTRL/Y.
	enableAST(t, env, ch, fnCtrlCAST, 0x4000, 5, 0)

	if env.Attention(AttentionCtrlY) {
		t.Error("CTRL/Y taken by a CTRL/C AST")
	}
}

func TestCtrlYAST(t *testing.T) {
	env, _, _, ch := qioFixture(t, "")

	enableAST(t, env, ch, fnCtrlYAST, 0x5000, 9, 0)

	// With no CTRL/C AST, CTRL/C acts as CTRL/Y.
	if !env.Attention(AttentionCtrlC) {
		t.Fatal("CTRL/C not taken by the CTRL/Y AST")
	}

	if got := queuedASTs(env); len(got) != 1 || got[0].routine != 0x5000 || got[0].param != 9 {
		t.Errorf("queued %+v, want the CTRL/Y AST", got)
	}

	// CTRL/Y itself, with both enabled: only the CTRL/Y AST.
	env.Process.ast.queue = nil
	enableAST(t, env, ch, fnCtrlCAST, 0x4000, 1, 0)
	enableAST(t, env, ch, fnCtrlYAST, 0x5000, 2, 0)

	if !env.Attention(AttentionCtrlY) {
		t.Fatal("CTRL/Y not taken")
	}

	if got := queuedASTs(env); len(got) != 1 || got[0].routine != 0x5000 {
		t.Errorf("queued %+v, want only the CTRL/Y AST", got)
	}

	if env.AttentionASTs() != 1 {
		t.Errorf("%d requests left, want the CTRL/C one", env.AttentionASTs())
	}
}

func TestAttentionAST_requests(t *testing.T) {
	env, _, a, ch := qioFixture(t, "")
	ch2 := assignCall(t, env, a, "TTA0", uint32(vax.User))

	// A second request on a channel replaces the first; each channel's
	// request is delivered.
	enableAST(t, env, ch, fnCtrlCAST, 0x4000, 1, 0)
	enableAST(t, env, ch, fnCtrlCAST, 0x4100, 2, 0)
	enableAST(t, env, ch2, fnCtrlCAST, 0x4200, 3, 0)

	if !env.Attention(AttentionCtrlC) {
		t.Fatal("CTRL/C not taken")
	}

	got := queuedASTs(env)
	if len(got) != 2 || got[0].routine != 0x4100 || got[1].routine != 0x4200 {
		t.Errorf("queued %+v, want the replacement request and the second channel's", got)
	}

	// p1 of 0 cancels.
	enableAST(t, env, ch, fnCtrlCAST, 0x4000, 1, 0)
	enableAST(t, env, ch, fnCtrlCAST, 0, 0, 0)

	if env.AttentionASTs() != 0 {
		t.Error("p1 = 0 didn't cancel the request")
	}

	// $CANCEL and $DASSGN cancel the channel's requests only.
	enableAST(t, env, ch, fnCtrlCAST, 0x4000, 1, 0)
	enableAST(t, env, ch2, fnCtrlYAST, 0x5000, 1, 0)
	wantR0(t, callLNM(t, env, serviceSysCancel, ch), ssNormal)

	if env.AttentionASTs() != 1 {
		t.Errorf("after $CANCEL, %d requests; want the other channel's 1", env.AttentionASTs())
	}

	wantR0(t, callLNM(t, env, serviceSysDassgn, ch2), ssNormal)

	if env.AttentionASTs() != 0 {
		t.Error("$DASSGN didn't cancel the channel's request")
	}

	// Image rundown deassigns the user-mode channel, cancelling it.
	enableAST(t, env, ch, fnCtrlYAST, 0x5000, 1, 0)
	env.ImageRundown()

	if env.AttentionASTs() != 0 {
		t.Error("image rundown kept a user channel's CTRL/Y AST")
	}
}
