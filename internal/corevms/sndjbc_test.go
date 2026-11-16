package corevms

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/vmsdef"
)

// $SNDJBC's queue functions (docs/planning/PHASE-98 - batch queues.md,
// subtask 6), held to round 2 of the probe, BAT98Q
// (testdata/bat98/vax/bat98b.log): R0 and the IOSB of each queue case,
// and the queue each made, as $GETQUI showed it.

// sndjbc calls $SNDJBCW with function and the item list at itmlst,
// returning R0 and the IOSB's status (iosb false: no IOSB, and 0).
func sndjbc(t *testing.T, env *Environment, a *arena, function string, itmlst uint32, iosb bool) (r0, status uint32) {
	t.Helper()

	code, ok := vmsdef.Symbols["SJC$_"+function]
	if !ok {
		code = 9999
	}

	block := uint32(0)
	if iosb {
		block = a.alloc(8)
	}

	r0, err := serviceSysSndjbc(env, []uint32{0, code, 0, itmlst, block, 0, 0})
	if err != nil {
		t.Fatal(err)
	}

	if iosb {
		status, _ = env.mem.LoadLongword(env.cpu, block)
	}

	return r0, status
}

// sjcItem is an input item: a string, a longword, or nothing.
func sjcItem(a *arena, name string, value any) item {
	code := uint16(vmsdef.Symbols["SJC$_"+name])

	switch v := value.(type) {
	case string:
		return item{code: code, buflen: uint16(len(v)), buf: a.str(v)}
	case int:
		return item{code: code, buflen: 4, buf: a.long(uint32(v))}
	}

	return item{code: code}
}

func TestSndjbcQueueFunctions(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	const (
		normal     = 0x00040001
		badParam   = 0x00000014
		noSuchQue  = 0x0004803A
		queNotStop = 0x00048182
		invParLen  = 0x00048094
	)

	queue := func(name string) uint32 { return a.items(sjcItem(a, "QUEUE", name)) }

	// No database: the manager can't run.
	if _, st := sndjbc(t, env, a, "START_QUEUE_MANAGER", 0, true); st != jbcJobQueDis {
		t.Errorf("START_QUEUE_MANAGER with no database: %#x", st)
	}

	path := filepath.Join(t.TempDir(), "qman.json")
	env.System.SetQueueFile(path)

	type step struct {
		what     string
		function string
		itmlst   uint32
		r0, iosb uint32
	}

	run := func(steps []step) {
		t.Helper()

		for _, s := range steps {
			if r0, st := sndjbc(t, env, a, s.function, s.itmlst, true); r0 != s.r0 || st != s.iosb {
				t.Errorf("%s, %s: R0 %#x IOSB %#x, want %#x %#x", s.what, s.function, r0, st, s.r0, s.iosb)
			}
		}
	}

	run([]step{
		{"start the manager", "START_QUEUE_MANAGER", 0, ssNormal, normal},
		{"case 1", "CREATE_QUEUE", a.items(
			sjcItem(a, "QUEUE", "BAT98_Q"), sjcItem(a, "BATCH", nil), sjcItem(a, "JOB_LIMIT", 2),
			sjcItem(a, "BASE_PRIORITY", 3), sjcItem(a, "OWNER_UIC", 0x00010004), sjcItem(a, "WSDEFAULT", 150),
			sjcItem(a, "QUEUE_DESCRIPTION", "BAT98 Q")), ssNormal, normal},
		{"case 2", "CREATE_QUEUE", a.items(sjcItem(a, "QUEUE", "BAT98_Q"), sjcItem(a, "BATCH", nil)), ssNormal, normal},
		{"case 3", "CREATE_QUEUE", a.items(sjcItem(a, "QUEUE", "BAT98_Q2"), sjcItem(a, "BATCH", nil)), ssNormal, normal},
		{"case 5", "START_QUEUE", queue("BAT98_Q"), ssNormal, normal},
		{"case 20", "NO_SUCH_FUNCTION", queue("BAT98_Q"), badParam, badParam},
		{"case 22", "START_QUEUE", a.items(sjcItem(a, "QUEUE", "BAT98_Q"), item{code: 9999, buflen: 4, buf: a.long(0)}),
			badParam, badParam},
		{"case 23", "CREATE_QUEUE", a.items(sjcItem(a, "QUEUE", strings.Repeat("Q", 32)), sjcItem(a, "BATCH", nil)),
			ssNormal, invParLen},
		{"case 51", "DELETE_QUEUE", queue("BAT98_Q"), ssNormal, queNotStop},
		{"case 52", "STOP_QUEUE", queue("BAT98_Q"), ssNormal, normal},
		{"case 53", "RESET_QUEUE", queue("BAT98_Q"), ssNormal, normal},
		{"case 54", "ALTER_QUEUE", a.items(sjcItem(a, "QUEUE", "BAT98_Q"), sjcItem(a, "JOB_LIMIT", 4),
			sjcItem(a, "QUEUE_DESCRIPTION", "ALTERED")), ssNormal, normal},
	})

	// $GETQUI's view after case 54 (case 55): JOB_LIMIT 4, ALTERED,
	// stopped; and case 4's owner, base priority, and working set.
	d, err := jobqueue.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	q := d.Queue("BAT98_Q")
	if q == nil || q.JobLimit != 4 || q.Description != "ALTERED" || q.State != jobqueue.QueueStopped ||
		q.BasePriority != 3 || q.Owner != jobqueue.SystemUIC || q.WSDefault != 150 {
		t.Errorf("BAT98_Q after ALTER_QUEUE: %+v", q)
	}

	run([]step{
		{"case 56", "DELETE_QUEUE", queue("BAT98_Q"), ssNormal, normal},
		{"case 57", "DELETE_QUEUE", queue("BAT98_Q"), ssNormal, noSuchQue},
		{"case 58", "DELETE_QUEUE", queue("BAT98_Q2"), ssNormal, normal},
		{"case 59", "STOP_QUEUE", queue("BAT98_Q"), ssNormal, noSuchQue},
	})

	// A queue function with no queue named is JBC$_MISREQPAR; a
	// request with no IOSB still has R0 SS$_NORMAL (case 17's form).
	if _, st := sndjbc(t, env, a, "START_QUEUE", 0, true); st != jbcMisReqPar {
		t.Errorf("START_QUEUE with no queue: %#x", st)
	}

	if r0, _ := sndjbc(t, env, a, "START_QUEUE", queue("NOSUCH"), false); r0 != ssNormal {
		t.Errorf("no IOSB: R0 %#x", r0)
	}

	// A printer queue is left out (Decision 1): JBC$_NOTSUPPORTED.
	if _, st := sndjbc(t, env, a, "CREATE_QUEUE", a.items(sjcItem(a, "QUEUE", "PRINTQ")), true); st != jbcNotSupported {
		t.Errorf("CREATE_QUEUE without BATCH: %#x", st)
	}

	// A value out of range changes nothing (JBC$_INVPARVAL).
	if _, st := sndjbc(t, env, a, "CREATE_QUEUE", a.items(sjcItem(a, "QUEUE", "BAT98_V"), sjcItem(a, "BATCH", nil),
		sjcItem(a, "JOB_LIMIT", 0)), true); st != jbcInvParVal {
		t.Errorf("JOB_LIMIT 0: %#x", st)
	}

	// The manager stopped: queue requests are JBC$_JOBQUEDIS.
	if _, st := sndjbc(t, env, a, "STOP_QUEUE_MANAGER", 0, true); st != normal {
		t.Errorf("STOP_QUEUE_MANAGER: %#x", st)
	}

	if _, st := sndjbc(t, env, a, "START_QUEUE", queue("BAT98_Q"), true); st != jbcJobQueDis {
		t.Errorf("START_QUEUE with the manager stopped: %#x", st)
	}
}
