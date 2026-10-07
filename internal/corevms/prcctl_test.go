package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestSetprn(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	wantR0(t, callLNM(t, env, serviceSysSetprn, a.desc("WORKER_1")), ssNormal)

	if env.Process.Name != "WORKER_1" {
		t.Fatalf("name = %q, want WORKER_1", env.Process.Name)
	}

	// The new name is the one prcnam arguments now match.
	wantR0(t, env.callerTarget(0, a.desc("WORKER_1"), false), 0)
	wantR0(t, env.callerTarget(0, a.desc("SYSTEM"), false), ssNonExpr)

	// Errors leave the name alone.
	wantR0(t, callLNM(t, env, serviceSysSetprn, a.desc("")), ssIvLogNam)
	wantR0(t, callLNM(t, env, serviceSysSetprn, a.desc("SIXTEEN_CHARS_XX")), ssIvLogNam)
	wantR0(t, callLNM(t, env, serviceSysSetprn, badAddr), ssAccVio)

	if env.Process.Name != "WORKER_1" {
		t.Errorf("name = %q after errors", env.Process.Name)
	}

	// Omitted: no name.
	wantR0(t, callLNM(t, env, serviceSysSetprn), ssNormal)

	if env.Process.Name != "" {
		t.Errorf("name = %q, want none", env.Process.Name)
	}
}

func TestSetpri(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process
	start := p.BasePriority

	prvpri := a.long(0xFFFFFFFF)
	wantR0(t, callLNM(t, env, serviceSysSetpri, 0, 0, 9, prvpri), ssNormal)

	if a.readLong(prvpri) != start || p.BasePriority != 9 || p.Priority != 9 {
		t.Errorf("prvpri %d, base %d, current %d; want %d, 9, 9", a.readLong(prvpri), p.BasePriority, p.Priority, start)
	}

	// By PID (written back for 0) and by name; only the low five bits.
	pid := a.long(0)
	wantR0(t, callLNM(t, env, serviceSysSetpri, pid, 0, 0x104), ssNormal)

	if a.readLong(pid) != p.PID || p.BasePriority != 4 {
		t.Errorf("PID %#x, base %d; want %#x, 4", a.readLong(pid), p.BasePriority, p.PID)
	}

	wantR0(t, callLNM(t, env, serviceSysSetpri, 0, a.desc("SYSTEM"), 31), ssNormal)

	// Errors change nothing.
	wantR0(t, callLNM(t, env, serviceSysSetpri, a.long(0x999), 0, 1), ssNonExpr)
	wantR0(t, callLNM(t, env, serviceSysSetpri, 0, a.desc("OTHER"), 1), ssNonExpr)
	wantR0(t, callLNM(t, env, serviceSysSetpri, 0, 0, 1, badAddr), ssAccVio)

	if p.BasePriority != 31 {
		t.Errorf("base %d after errors, want 31", p.BasePriority)
	}
}

func TestForcex(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	wantR0(t, callLNM(t, env, serviceSysForcex, 0, 0, 0x2C), ssNormal)

	q := env.Process.ast.queue
	if len(q) != 1 || q[0].routine != exitEntryAddr || q[0].param != 0x2C || q[0].mode != uint32(vax.User) {
		t.Fatalf("queued %+v; want a user-mode AST to SYS$EXIT with parameter 0x2C", q)
	}

	// A second request while one is pending adds nothing.
	wantR0(t, callLNM(t, env, serviceSysForcex, 0, a.desc("SYSTEM")), ssNormal)

	if len(env.Process.ast.queue) != 1 {
		t.Errorf("%d ASTs queued, want still 1", len(env.Process.ast.queue))
	}

	wantR0(t, callLNM(t, env, serviceSysForcex, a.long(0x999)), ssNonExpr)
}

func TestDelprc(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	setMode(env, vax.User, vax.User, 0x8000)
	wantR0(t, callLNM(t, env, serviceSysDclexh, a.alloc(16)+0), ssNormal)

	// The image ends, with no exit handler called.
	r0, err := serviceSysDelprc(env, nil)
	if !errors.Is(err, ErrExit) || r0 != ssNormal {
		t.Fatalf("$DELPRC = %#x, %v; want SS$_NORMAL, ErrExit", r0, err)
	}

	if env.ExitHandlers(vax.User) != 0 {
		t.Error("an exit handler survived $DELPRC")
	}

	// Another process doesn't exist.
	r0, err = serviceSysDelprc(env, []uint32{a.long(0x999)})
	if err != nil || r0 != ssNonExpr {
		t.Errorf("$DELPRC of another = %#x, %v; want SS$_NONEXPR", r0, err)
	}
}
