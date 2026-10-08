package corevms

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// $CREPRC's checks (docs/PHASE-45.md, subtask 4). The fixture has no
// VMINIT, so no S0 pool: a request that passes every check fails at the
// last step, building the process's memory, with SS$_NOSLOT. That makes
// it a test of each check before it, and of the clean-up after the last:
// whatever fails, nothing is left created. Creating a process for real
// is tested with a booted machine, in internal/console's creprc_test.go.

// creprcArgs returns a $CREPRC argument list (12 arguments, pidadr
// first), all omitted but those set.
func creprcArgs(set map[int]uint32) []uint32 {
	argv := make([]uint32, 12)
	for i, v := range set {
		argv[i] = v
	}

	return argv
}

// The arguments' positions.
const (
	argPidadr = iota
	argImage
	argInput
	argOutput
	argError
	argPrvadr
	argQuota
	argPrcnam
	argBaspri
	argUIC
	argMbxunt
	argStsflg
)

// quotaList returns the address of a quota list of code, value pairs,
// ended by PQL$_LISTEND.
func (a *arena) quotaList(pairs ...uint32) uint32 {
	addr := a.alloc(uint32(len(pairs)/2*5 + 1))

	at := addr
	for i := 0; i+1 < len(pairs); i += 2 {
		putByte(a.t, a.env, at, byte(pairs[i]))
		putLongword(a.t, a.env, at+1, pairs[i+1])
		at += 5
	}

	putByte(a.t, a.env, at, 0)

	return addr
}

// pql is a PQL$_ code.
func pql(name string) uint32 { return vmsdef.Symbols["PQL$_"+name] }

// unreadable is an address beyond the fixture's memory.
const unreadable = 0x00F00000

// TestCreprc_unsupported: with vax.process.scheduler off, $CREPRC is
// SS$_UNSUPPORTED, whatever its arguments.
func TestCreprc_unsupported(t *testing.T) {
	env, _ := fixture()
	env.ProcessSettings.Scheduler = false
	a := newArena(t, env)

	r0 := callLNM(t, env, serviceSysCreprc, creprcArgs(map[int]uint32{argPrcnam: a.desc("WORKER")})...)
	wantR0(t, r0, ssUnsupported)

	if _, st := env.CreateProcess(CreateRequest{}); st != ssUnsupported {
		t.Errorf("CreateProcess: %#x, want SS$_UNSUPPORTED", st)
	}
}

// TestCreprc_errors: each check's status, and nothing created by a
// failed call: the process table, the scheduler, the job's and the
// creator's subprocess counts, and the PID's longword are as they were.
func TestCreprc_errors(t *testing.T) {
	ssExQuota := vmsdef.Symbols["SS$_EXQUOTA"]

	for _, tt := range []struct {
		name  string
		setup func(env *Environment)
		args  func(a *arena) map[int]uint32
		want  uint32
	}{
		{"no pool", nil, func(a *arena) map[int]uint32 {
			return map[int]uint32{argPrcnam: a.desc("WORKER"), argImage: a.desc("CHILD")}
		}, ssNoSlot},
		{"no pool, detached", nil, func(a *arena) map[int]uint32 {
			return map[int]uint32{argUIC: 0x00200004}
		}, ssNoSlot},
		{"reserved flag", nil, func(*arena) map[int]uint32 {
			return map[int]uint32{argStsflg: 1 << 18}
		}, ssIvStsFlg},
		{"empty name", nil, func(a *arena) map[int]uint32 {
			return map[int]uint32{argPrcnam: a.desc("")}
		}, ssIvLogNam},
		{"long name", nil, func(a *arena) map[int]uint32 {
			return map[int]uint32{argPrcnam: a.desc("SIXTEEN_LETTERS_")}
		}, ssIvLogNam},
		{"long image", nil, func(a *arena) map[int]uint32 {
			return map[int]uint32{argImage: a.desc(strings.Repeat("X", 256))}
		}, ssIvLogNam},
		{"long error", nil, func(a *arena) map[int]uint32 {
			return map[int]uint32{argError: a.desc(strings.Repeat("X", 256))}
		}, ssIvLogNam},
		{"unreadable input", nil, func(*arena) map[int]uint32 {
			return map[int]uint32{argInput: unreadable}
		}, ssAccVio},
		{"unreadable name", nil, func(*arena) map[int]uint32 {
			return map[int]uint32{argPrcnam: unreadable}
		}, ssAccVio},
		{"unreadable privileges", nil, func(*arena) map[int]uint32 {
			return map[int]uint32{argPrvadr: unreadable}
		}, ssAccVio},
		{"unreadable quota list", nil, func(*arena) map[int]uint32 {
			return map[int]uint32{argQuota: unreadable}
		}, ssAccVio},
		{"unknown quota", nil, func(a *arena) map[int]uint32 {
			return map[int]uint32{argQuota: a.quotaList(pql("ASTLM"), 10, 15, 1)}
		}, ssIvQuotaL},
		{"unwritable PID", nil, func(*arena) map[int]uint32 {
			return map[int]uint32{argPidadr: unreadable}
		}, ssAccVio},
		{"detached, other UIC, no DETACH", func(env *Environment) {
			env.Process.CurrentPrivileges &^= privDETACH | privCMKRNL
		}, func(*arena) map[int]uint32 {
			return map[int]uint32{argUIC: 0x00200004}
		}, ssNoPriv},
		{"batch, no DETACH", func(env *Environment) {
			env.Process.CurrentPrivileges &^= privDETACH | privCMKRNL
		}, func(*arena) map[int]uint32 {
			return map[int]uint32{argStsflg: prcBATCH}
		}, ssNoPriv},
		{"network, no NETMBX", func(env *Environment) {
			env.Process.CurrentPrivileges &^= privNETMBX
		}, func(*arena) map[int]uint32 {
			return map[int]uint32{argStsflg: prcNETWRK}
		}, ssNoPriv},
		{"no swapping, no PSWAPM", func(env *Environment) {
			env.Process.CurrentPrivileges &^= privPSWAPM
		}, func(*arena) map[int]uint32 {
			return map[int]uint32{argStsflg: prcPSWAPM}
		}, ssNoPriv},
		{"no accounting, no NOACNT", func(env *Environment) {
			env.Process.CurrentPrivileges &^= privNOACNT
		}, func(*arena) map[int]uint32 {
			return map[int]uint32{argStsflg: prcNOACNT}
		}, ssNoPriv},
		{"duplicate name", func(env *Environment) {
			env.Process.Name = "WORKER"
		}, func(a *arena) map[int]uint32 {
			return map[int]uint32{argPrcnam: a.desc("WORKER")}
		}, ssDuplNam},
		{"PRCLM", func(env *Environment) {
			env.Process.Job.SubprocessCount = env.Process.Job.SubprocessLimit
		}, func(*arena) map[int]uint32 {
			return map[int]uint32{}
		}, ssExQuota},
		{"CPU time", func(env *Environment) {
			env.Process.CPULimit = 1
		}, func(*arena) map[int]uint32 {
			return map[int]uint32{}
		}, ssExQuota},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env, _ := fixture()
			env.ProcessSettings.Scheduler = true
			a := newArena(t, env)

			if tt.setup != nil {
				tt.setup(env)
			}

			job, pidadr := env.Process.Job, a.long(0xCAFEF00D)

			args := tt.args(a)
			if _, ok := args[argPidadr]; !ok {
				args[argPidadr] = pidadr
			}

			jobCount, cpu := job.SubprocessCount, env.Process.CPULimit

			r0 := callLNM(t, env, serviceSysCreprc, creprcArgs(args)...)
			wantR0(t, r0, tt.want)

			if n := len(env.Processes()); n != 1 {
				t.Errorf("%d processes after a failed $CREPRC, want 1", n)
			}

			if n := len(env.Scheduler().Handles()); n != 1 {
				t.Errorf("%d processes in the scheduler after a failed $CREPRC, want 1", n)
			}

			if job.SubprocessCount != jobCount || env.Process.SubprocessCount != 0 {
				t.Errorf("subprocess counts: job %d (was %d), creator %d; a failed $CREPRC changed them",
					job.SubprocessCount, jobCount, env.Process.SubprocessCount)
			}

			if env.Process.CPULimit != cpu {
				t.Errorf("the creator's CPU limit is %d, was %d", env.Process.CPULimit, cpu)
			}

			if v := a.readLong(pidadr); v != 0xCAFEF00D {
				t.Errorf("pidadr holds %08X after a failed $CREPRC", v)
			}
		})
	}
}

// TestCreprc_quotas: the three steps that make a new process's quotas:
// the default, the list's last value for a quota, the minimum, and the
// creator's own as a ceiling, unless a detached process is created with
// DETACH. Pooled quotas are the job's: only a detached process sets
// them.
func TestCreprc_quotas(t *testing.T) {
	env, _ := fixture()
	creator := env.Process
	creator.ASTLimit = 50

	items := []QuotaItem{
		{pql("ASTLM"), 100}, // above the creator's 50
		{pql("BIOLM"), 1},   // below the minimum, 4
		{pql("DIOLM"), 9},
		{pql("DIOLM"), 12}, // the last DIOLM wins
		{pql("PRCLM"), 3},
	}

	values, deducted, st := resolveQuotas(creator, items, false, false)
	if st != 0 || deducted != 0 {
		t.Fatalf("resolveQuotas: status %#x, %d deducted", st, deducted)
	}

	p := NewProcess()
	p.Job = newJob(0x999)
	applyQuotas(p, values, false)

	for _, tt := range []struct {
		name      string
		got, want uint32
	}{
		{"ASTLM", p.ASTLimit, 50},
		{"BIOLM", p.BufferedIOLimit, 4},
		{"DIOLM", p.DirectIOLimit, 12},
		{"WSDEFAULT (default, raised to the minimum)", p.WSDefault, 512},
		{"WSQUOTA (default, raised to the minimum)", p.WSQuota, 1024},
		{"working set", p.WSLimit, 512},
		{"PRCLM (pooled, a subprocess's is ignored)", p.Job.SubprocessLimit, nominalPRCLM},
		{"CPULM (the creator has none)", p.CPULimit, 0},
	} {
		if tt.got != tt.want {
			t.Errorf("subprocess's %s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}

	// A detached process created with DETACH gets what it asks for, and
	// its job the pooled values.
	values, _, _ = resolveQuotas(creator, items, true, true)
	d := NewProcess()
	d.Job = newJob(0x999)
	applyQuotas(d, values, true)

	if d.ASTLimit != 100 || d.Job.SubprocessLimit != 3 || d.Job.Pooled.FILLM != 16 {
		t.Errorf("detached: ASTLM %d, PRCLM %d, FILLM %d; want 100, 3, 16 (the default)",
			d.ASTLimit, d.Job.SubprocessLimit, d.Job.Pooled.FILLM)
	}

	// The working-set default is held to the quota.
	values, _, _ = resolveQuotas(creator, []QuotaItem{{pql("WSDEFAULT"), 3000}, {pql("WSQUOTA"), 2000}}, true, true)
	w := NewProcess()
	w.Job = newJob(0x999)
	applyQuotas(w, values, true)

	if w.WSDefault != 2000 || w.WSLimit != 2000 {
		t.Errorf("WSDEFAULT 3000 over WSQUOTA 2000: default %d, limit %d; want 2000", w.WSDefault, w.WSLimit)
	}
}

// TestCreprc_cpuLimit: the CPU time limit's own rules.
func TestCreprc_cpuLimit(t *testing.T) {
	ssExQuota := vmsdef.Symbols["SS$_EXQUOTA"]

	for _, tt := range []struct {
		name                          string
		have, asked                   uint32
		given, detached, independent  bool
		wantLimit, wantTaken, wantErr uint32
	}{
		{"subprocess, unlimited creator, none asked", 0, 0, false, false, false, 0, 0, 0},
		{"subprocess, unlimited creator, 500 asked", 0, 500, true, false, false, 500, 0, 0},
		{"subprocess, none asked: half", 1000, 0, false, false, false, 500, 500, 0},
		{"subprocess, 0 asked: half", 1000, 0, true, false, false, 500, 500, 0},
		{"subprocess, 300 asked", 1000, 300, true, false, false, 300, 300, 0},
		{"subprocess, more than the creator has", 1000, 5000, true, false, false, 0, 0, ssExQuota},
		{"subprocess, a creator with 1", 1, 0, false, false, false, 0, 0, ssExQuota},
		{"detached, none asked: unlimited", 1000, 0, false, true, true, 0, 0, 0},
		{"detached with DETACH, 5000 asked", 1000, 5000, true, true, true, 5000, 0, 0},
		{"detached without DETACH, none asked: half", 1000, 0, false, true, false, 500, 500, 0},
	} {
		creator := NewProcess()
		creator.CPULimit = tt.have

		limit, taken, st := cpuLimit(creator, tt.asked, tt.given, tt.detached, tt.independent)
		if limit != tt.wantLimit || taken != tt.wantTaken || st != tt.wantErr {
			t.Errorf("%s: limit %d, taken %d, status %#x; want %d, %d, %#x",
				tt.name, limit, taken, st, tt.wantLimit, tt.wantTaken, tt.wantErr)
		}
	}
}
