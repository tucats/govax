package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// inMode puts the CPU in mode, its previous mode the same.
func inMode(env *Environment, mode registers.AccessMode) {
	psl := env.cpu.PSL()
	psl.SetCurMod(mode)
	psl.SetPrvMod(mode)
	env.cpu.SetPSL(psl)
}

// $DCLCMH declares the handler of the caller's mode, user or supervisor,
// writing the one it replaces to prvhnd; an inner mode is SS$_IVSSRQ, a
// prvhnd it can't write SS$_ACCVIO with nothing changed; type 1 is the
// compatibility mode handler, never a change mode one; image rundown
// forgets the user-mode handler and keeps the supervisor-mode one.
func TestDclcmh(t *testing.T) {
	env, _ := fixture()
	const prvhnd = 0x3000

	inMode(env, registers.User)

	if r0, _ := serviceSysDclcmh(env, []uint32{0x1234, prvhnd, 0}); r0 != ssNormal || env.ChangeModeHandler(registers.User) != 0x1234 {
		t.Fatalf("user: R0 %08X, handler %#x", r0, env.ChangeModeHandler(registers.User))
	}

	if r0, _ := serviceSysDclcmh(env, []uint32{0x5678, prvhnd, 0}); r0 != ssNormal {
		t.Fatalf("user again: R0 %08X", r0)
	}

	if prev, _ := env.mem.LoadLongword(env.cpu, prvhnd); prev != 0x1234 {
		t.Errorf("prvhnd %#x, want the first handler, 0x1234", prev)
	}

	if r0, _ := serviceSysDclcmh(env, []uint32{0x9999, 0xFFFFFFF0, 0}); r0 != ssAccVio || env.ChangeModeHandler(registers.User) != 0x5678 {
		t.Errorf("unwritable prvhnd: R0 %08X, handler %#x; want SS$_ACCVIO, unchanged", r0, env.ChangeModeHandler(registers.User))
	}

	if r0, _ := serviceSysDclcmh(env, []uint32{0x7000, 0, 1}); r0 != ssNormal || env.ChangeModeHandler(registers.User) != 0x5678 {
		t.Errorf("type 1: R0 %08X, change mode handler %#x; want it unchanged", r0, env.ChangeModeHandler(registers.User))
	}

	inMode(env, registers.Supervisor)

	if r0, _ := serviceSysDclcmh(env, []uint32{0x2222, 0, 0}); r0 != ssNormal || env.ChangeModeHandler(registers.Supervisor) != 0x2222 {
		t.Errorf("supervisor: R0 %08X, handler %#x", r0, env.ChangeModeHandler(registers.Supervisor))
	}

	for _, mode := range []registers.AccessMode{registers.Kernel, registers.Executive} {
		inMode(env, mode)

		if r0, _ := serviceSysDclcmh(env, []uint32{0x3333, 0, 0}); r0 != ssIvSsRq {
			t.Errorf("mode %v: R0 %08X, want SS$_IVSSRQ", mode, r0)
		}
	}

	env.cancelUserChangeMode()

	if env.ChangeModeHandler(registers.User) != 0 || env.ChangeModeHandler(registers.Supervisor) != 0x2222 {
		t.Errorf("after rundown: user %#x, supervisor %#x; want 0 and 0x2222",
			env.ChangeModeHandler(registers.User), env.ChangeModeHandler(registers.Supervisor))
	}
}

// $SETSFM sets the caller's mode's bit alone, returning what it was.
func TestSetsfm(t *testing.T) {
	env, _ := fixture()
	inMode(env, registers.User)

	for i, tt := range []struct {
		enable uint32
		want   uint32
	}{{1, ssWasClrStatus}, {1, ssWasSetStatus}, {0, ssWasSetStatus}, {0, ssWasClrStatus}} {
		if r0, _ := serviceSysSetsfm(env, []uint32{tt.enable}); r0 != tt.want {
			t.Errorf("call %d: R0 %08X, want %08X", i, r0, tt.want)
		}
	}

	inMode(env, registers.Supervisor)
	_, _ = serviceSysSetsfm(env, []uint32{1})

	if env.FailureExceptions(registers.User) || !env.FailureExceptions(registers.Supervisor) {
		t.Error("supervisor's $SETSFM changed another mode's bit")
	}
}

// failureException raises SS$_SSFAIL only for an error or severe status, in
// a mode with the exceptions enabled, from a service that changes mode.
func TestFailureException_which(t *testing.T) {
	env := conditionFixture(t)
	inMode(env, registers.User)
	env.Process.failureExceptions[registers.User] = true

	for _, tt := range []struct {
		name   string
		status uint32
		want   bool
	}{
		{"SYS$ASSIGN", ssNormal, false},
		{"SYS$ASSIGN", 0x908, false}, // SS$_NOSUCHDEV, a warning
		{"SYS$FAO", ssAccVio, false},  // runs in the caller's mode
		{"SYS$GETTIM", ssAccVio, true},
	} {
		env.Process.conditions = nil

		if got := env.failureException(tt.name, 0x7FFEDE00, tt.status); got != tt.want {
			t.Errorf("%s returning %08X: signaled %v, want %v", tt.name, tt.status, got, tt.want)
		}
	}

	d := env.Process.conditions[0]
	if words, err := env.sigWords(d); err != nil || len(words) != 4 || words[0] != ssSSFail ||
		words[1] != ssAccVio || words[2] != 0x7FFEDE04 {
		t.Errorf("signal array %08X (%v), want SS$_SSFAIL, the status, the entry's RET, the PSL", words, err)
	}
}
