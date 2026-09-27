package rtl

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

func TestNewEnvironmentProcess(t *testing.T) {
	env, _ := fixture()
	p := env.Process

	if p == nil {
		t.Fatal("Environment.Process = nil, want the default emulated process")
	}

	if p.PID != nominalPID || p.Username != "SYSTEM" || p.UIC != NominalUIC {
		t.Errorf("process = PID %#x user %q UIC %#x, want %#x SYSTEM %#x",
			p.PID, p.Username, p.UIC, nominalPID, NominalUIC)
	}

	if p.UICGroup() != 1 || p.UICMember() != 4 {
		t.Errorf("UIC = [%o,%o], want [1,4]", p.UICGroup(), p.UICMember())
	}

	if p.WSLimit != p.WSDefault {
		t.Errorf("WSLimit = %d, want WSDEFAULT %d", p.WSLimit, p.WSDefault)
	}

	if !(p.MinWSCount <= p.WSDefault && p.WSDefault <= p.WSQuota && p.WSQuota <= p.WSExtent) {
		t.Errorf("working-set quotas out of order: MINWSCNT %d, WSDEFAULT %d, WSQUOTA %d, WSEXTENT %d",
			p.MinWSCount, p.WSDefault, p.WSQuota, p.WSExtent)
	}
}

func TestOptArg(t *testing.T) {
	argv := []uint32{7, 8}

	if got := optArg(argv, 1); got != 8 {
		t.Errorf("optArg(argv, 1) = %d, want 8", got)
	}

	if got := optArg(argv, 2); got != 0 {
		t.Errorf("optArg(argv, 2) = %d, want 0 for an omitted trailing argument", got)
	}
}

// setCurMod puts the emulated CPU in mode, as the caller of a service.
func setCurMod(env *Environment, mode vax.AccessMode) {
	psl := env.cpu.PSL()
	psl.SetCurMod(mode)
	env.cpu.SetPSL(psl)
}

func TestServiceSysAdjstk(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	// From kernel mode, load user mode's stack pointer from newadr, then
	// adjust it by a negative word.
	env.cpu.SetPR(vax.USP, 0x7000)
	newadr := a.long(0x5000)
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 3, uint32(0xFFF8), newadr), ssNormal)

	if got := env.cpu.PR(vax.USP); got != 0x4FF8 {
		t.Errorf("USP = %#x, want 0x4FF8 (0x5000 - 8)", got)
	}

	if got := a.readLong(newadr); got != 0x4FF8 {
		t.Errorf("newadr = %#x, want the updated stack pointer 0x4FF8", got)
	}

	// newadr holding 0 adjusts the mode's current stack pointer. Only the
	// low word of adjust counts, so 0x10010 adds 0x10.
	env.cpu.SetPR(vax.SSP, 0x9000)
	newadr = a.long(0)
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 2, 0x10010, newadr), ssNormal)

	if got := env.cpu.PR(vax.SSP); got != 0x9010 {
		t.Errorf("SSP = %#x, want 0x9010", got)
	}

	if got := a.readLong(newadr); got != 0x9010 {
		t.Errorf("newadr = %#x, want 0x9010", got)
	}

	// adjust 0 and newadr holding 0 leave the stack pointer alone, but
	// still report it through newadr.
	env.cpu.SetPR(vax.ESP, 0x8000)
	newadr = a.long(0)
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 1, 0, newadr), ssNormal)

	if got, gotAdr := env.cpu.PR(vax.ESP), a.readLong(newadr); got != 0x8000 || gotAdr != 0x8000 {
		t.Errorf("ESP = %#x, newadr = %#x, want both 0x8000", got, gotAdr)
	}

	// adjust 0 with an address just loads it.
	newadr = a.long(0x6000)
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 3, 0, newadr), ssNormal)

	if got := env.cpu.PR(vax.USP); got != 0x6000 {
		t.Errorf("USP = %#x, want 0x6000", got)
	}
}

func TestServiceSysAdjstkErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	newadr := a.long(0x5000)

	// The caller's own mode (and the default, kernel) is SS$_NOPRIV.
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 0, 0, newadr), ssNoPriv)
	wantR0(t, callLNM(t, env, serviceSysAdjstk), ssNoPriv)

	// From user mode there is no less privileged mode; a more privileged
	// acmode is maximized to user.
	setCurMod(env, vax.User)
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 1, 0, newadr), ssNoPriv)
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 3, 0, newadr), ssNoPriv)

	if got := a.readLong(newadr); got != 0x5000 {
		t.Errorf("newadr = %#x after a failed call, want it untouched", got)
	}

	// From supervisor mode, adjusting user mode works, but a missing
	// newadr is an access violation.
	setCurMod(env, vax.Supervisor)
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 3, 0, 0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysAdjstk, 3, 0, newadr), ssNormal)
}

// TestPhase26ServicesRegistered checks each service docs/PHASE-26.md adds
// is reachable through SystemService, by its real P1-vector address.
func TestPhase26ServicesRegistered(t *testing.T) {
	env, _ := fixture()

	for _, name := range []string{"SYS$ADJSTK", "SYS$ADJWSL", "SYS$ALLOC", "SYS$ASCEFC", "SYS$DALLOC"} {
		addr, found := uint32(0), false

		for _, e := range vmsdef.P1VectorTable {
			if e.Name == name {
				addr, found = e.Addr, true
			}
		}

		if !found {
			t.Fatalf("%s is not in the P1 vector", name)
		}

		putArgs(t, env, 0x2000, nil)

		if _, handled, _ := env.SystemService(addr); !handled {
			t.Errorf("%s at %#x not handled", name, addr)
		}
	}
}

func TestServiceSysAdjwsl(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process
	start := p.WSLimit

	// pagcnt 0 (or omitted) reports the current limit and changes nothing.
	wsetlm := a.long(0xFFFFFFFF)
	wantR0(t, callLNM(t, env, serviceSysAdjwsl, 0, wsetlm), ssNormal)

	if got := a.readLong(wsetlm); got != start {
		t.Errorf("wsetlm = %d, want the current limit %d", got, start)
	}

	wantR0(t, callLNM(t, env, serviceSysAdjwsl), ssNormal)

	if p.WSLimit != start {
		t.Errorf("WSLimit = %d after pagcnt 0, want %d", p.WSLimit, start)
	}

	// A positive pagcnt grows the limit; a negative one shrinks it.
	wantR0(t, callLNM(t, env, serviceSysAdjwsl, 50, wsetlm), ssNormal)

	if got := a.readLong(wsetlm); got != start+50 || p.WSLimit != start+50 {
		t.Errorf("after +50: wsetlm = %d, WSLimit = %d, want %d", got, p.WSLimit, start+50)
	}

	minus30 := int32(-30)
	wantR0(t, callLNM(t, env, serviceSysAdjwsl, uint32(minus30), wsetlm), ssNormal)

	if got := a.readLong(wsetlm); got != start+20 {
		t.Errorf("after -30: wsetlm = %d, want %d", got, start+20)
	}

	// Past either end, the limit is clamped without an error.
	wantR0(t, callLNM(t, env, serviceSysAdjwsl, 1_000_000, wsetlm), ssNormal)

	if got := a.readLong(wsetlm); got != p.WSExtent {
		t.Errorf("after a huge increase: wsetlm = %d, want WSEXTENT %d", got, p.WSExtent)
	}

	minusHuge := int32(-1_000_000)
	wantR0(t, callLNM(t, env, serviceSysAdjwsl, uint32(minusHuge)), ssNormal)

	if p.WSLimit != p.MinWSCount {
		t.Errorf("after a huge decrease: WSLimit = %d, want MINWSCNT %d", p.WSLimit, p.MinWSCount)
	}
}

func TestServiceSysAdjwslAccvio(t *testing.T) {
	env, _ := fixture()
	start := env.Process.WSLimit

	// wsetlm outside the fixture's 1MB of memory can't be written; the
	// limit is left alone.
	wantR0(t, callLNM(t, env, serviceSysAdjwsl, 10, 0x7FFF0000), ssAccVio)

	if env.Process.WSLimit != start {
		t.Errorf("WSLimit = %d after SS$_ACCVIO, want it unchanged at %d", env.Process.WSLimit, start)
	}
}
