package rtl

import "testing"

func TestServiceSysGettim(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	now := fakeClock(env)
	*now = 0x00A1B2C3D4E5F607

	timadr := a.quad(0)
	wantR0(t, callLNM(t, env, serviceSysGettim, timadr), ssNormal)

	if got, _ := env.loadQuad(timadr); got != *now {
		t.Errorf("$GETTIM stored %#x, want the clock's %#x", got, *now)
	}

	// The clock moves on; a second call sees it.
	*now += 5 * ms
	wantR0(t, callLNM(t, env, serviceSysGettim, timadr), ssNormal)

	if got, _ := env.loadQuad(timadr); got != *now {
		t.Errorf("second $GETTIM stored %#x, want %#x", got, *now)
	}
}

func TestServiceSysGettimAccvio(t *testing.T) {
	env, _ := fixture()

	// timadr 0, omitted, or outside memory.
	wantR0(t, callLNM(t, env, serviceSysGettim, 0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysGettim), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysGettim, 0x7FFFFFF0), ssAccVio)
}
