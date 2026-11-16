package corevms

import (
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/rms"
)

// $SNDJBC's job functions (docs/planning/PHASE-98 - batch queues.md,
// subtask 7), held to round 2 of the probe, BAT98Q
// (testdata/bat98/vax/bat98b.log): ENTER_FILE, ALTER_JOB, DELETE_JOB, and
// CREATE_JOB, ADD_FILE, and CLOSE_JOB, with R0, the IOSB, and the jobs
// they left.

// jobServiceFixture is a System with a queue database, its manager
// started, a started batch queue BAT98_Q, and a volume DUA0 with
// [000000]BAT98OK.COM and BAT98WRN.COM.
func jobServiceFixture(t *testing.T) (*Environment, *arena, string) {
	t.Helper()

	env, _ := fixture()
	newMountedVolumeFixture(t, env)
	env.UseSession(rms.NewSession(env.Mounts).ForProcess(env.Logicals))

	for _, name := range []string{"BAT98OK.COM", "BAT98WRN.COM"} {
		loc, err := env.Session.Locate("DUA0:[000000]"+name, false)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := env.Session.CreateRecordFile(loc, rms.TextRecords, [][]byte{[]byte("$ EXIT")}); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(t.TempDir(), "qman.json")
	env.System.SetQueueFile(path)

	a := newArena(t, env)

	for _, step := range []struct {
		function string
		itmlst   uint32
	}{
		{"START_QUEUE_MANAGER", 0},
		{"CREATE_QUEUE", a.items(sjcItem(a, "QUEUE", "BAT98_Q"), sjcItem(a, "BATCH", nil))},
		{"START_QUEUE", a.items(sjcItem(a, "QUEUE", "BAT98_Q"))},
	} {
		if _, st := sndjbc(t, env, a, step.function, step.itmlst, true); st != jbcNormal {
			t.Fatalf("%s: %#x", step.function, st)
		}
	}

	return env, a, path
}

// outputItem is an output item of n bytes, and where its length goes.
func outputItem(a *arena, name string, n uint16) item {
	it := sjcItem(a, name, nil)
	it.buflen, it.buf, it.ret = n, a.alloc(uint32(n)), a.alloc(2)

	return it
}

func TestSndjbcJobFunctions(t *testing.T) {
	env, a, path := jobServiceFixture(t)

	const (
		normal    = 0x00040001
		noSuchQue = 0x0004803A
		noSuchJob = 0x00048042
		misReqPar = 0x0004810C
		fnf       = 0x00018292
	)

	// Case 6: a held job, its entry number and SUBMIT's message out.
	entryOut, statusOut := outputItem(a, "ENTRY_NUMBER_OUTPUT", 4), outputItem(a, "JOB_STATUS_OUTPUT", 4)

	r0, st := sndjbc(t, env, a, "ENTER_FILE", a.items(
		sjcItem(a, "QUEUE", "BAT98_Q"), sjcItem(a, "FILE_SPECIFICATION", "DUA0:[000000]BAT98OK.COM"),
		sjcItem(a, "HOLD", nil), sjcItem(a, "JOB_NAME", "QJOB"), sjcItem(a, "PARAMETER_1", "P1VAL"),
		sjcItem(a, "NOTE", "QNOTE"), sjcItem(a, "LOG_SPECIFICATION", "DUA0:[000000]QJOB.LOG"),
		sjcItem(a, "NO_LOG_SPOOL", nil), entryOut, statusOut), true)
	if r0 != ssNormal || st != normal {
		t.Fatalf("case 6: R0 %#x IOSB %#x", r0, st)
	}

	entry := a.readLong(entryOut.buf)
	if got := a.readString(statusOut.buf, 4); got != "Job " {
		t.Errorf("case 6: JOB_STATUS_OUTPUT %q", got)
	}

	job := func() *jobqueue.Job {
		d, err := jobqueue.Read(path)
		if err != nil {
			t.Fatal(err)
		}

		return d.Job(int(entry))
	}

	if j := job(); j == nil || j.Name != "QJOB" || j.Status != jobqueue.JobHolding || j.Note != "QNOTE" ||
		len(j.Parameters) != 1 || j.Parameters[0] != "P1VAL" || j.LogFile != "DUA0:[000000]QJOB.LOG" ||
		!j.NoPrinter || j.Priority != jobqueue.DefaultJobPri || j.User != "SYSTEM" {
		t.Errorf("case 6's job: %+v", j)
	}

	// Cases 10 to 12: ALTER_JOB changes the note and replaces the
	// parameters (PARAMETER_1 empty after PARAMETER_2 alone); NO_HOLD
	// releases it.
	entryItem := item{code: sjcCode("ENTRY_NUMBER"), buflen: 4, buf: a.long(entry)}

	if _, st := sndjbc(t, env, a, "ALTER_JOB", a.items(sjcItem(a, "QUEUE", "BAT98_Q"), entryItem,
		sjcItem(a, "NOTE", "CHANGED"), sjcItem(a, "PARAMETER_2", "P2VAL")), true); st != normal {
		t.Errorf("case 10: %#x", st)
	}

	if j := job(); j.Note != "CHANGED" || len(j.Parameters) != 2 || j.Parameters[0] != "" || j.Parameters[1] != "P2VAL" ||
		j.Status != jobqueue.JobHolding {
		t.Errorf("case 11's job: %+v", j)
	}

	if _, st := sndjbc(t, env, a, "ALTER_JOB", a.items(sjcItem(a, "QUEUE", "BAT98_Q"), entryItem,
		sjcItem(a, "NO_HOLD", nil)), true); st != normal {
		t.Errorf("case 12: %#x", st)
	}

	if j := job(); j.Status != jobqueue.JobPending {
		t.Errorf("case 12's job: %s", j.Status)
	}

	for _, c := range []struct {
		what     string
		function string
		itmlst   uint32
		r0, iosb uint32
	}{
		{"case 15", "ENTER_FILE", a.items(sjcItem(a, "QUEUE", "BAT98_Q")), ssNormal, misReqPar},
		{"case 16", "ENTER_FILE", a.items(sjcItem(a, "QUEUE", "NOSUCHQ98"),
			sjcItem(a, "FILE_SPECIFICATION", "DUA0:[000000]BAT98OK.COM")), ssNormal, noSuchQue},
		{"case 18", "ENTER_FILE", a.items(sjcItem(a, "QUEUE", "BAT98_Q"),
			sjcItem(a, "FILE_SPECIFICATION", "DUA0:[000000]NOSUCH98.COM")), fnf, fnf},
		{"case 19", "DELETE_JOB", a.items(sjcItem(a, "QUEUE", "BAT98_Q"),
			item{code: sjcCode("ENTRY_NUMBER"), buflen: 4, buf: a.long(999)}), ssNormal, noSuchJob},
		{"case 49", "CLOSE_JOB", 0, ssNormal, noSuchJob},
		{"the job", "DELETE_JOB", a.items(sjcItem(a, "QUEUE", "BAT98_Q"), entryItem), ssNormal, normal},
	} {
		if r0, st := sndjbc(t, env, a, c.function, c.itmlst, true); r0 != c.r0 || st != c.iosb {
			t.Errorf("%s, %s: R0 %#x IOSB %#x, want %#x %#x", c.what, c.function, r0, st, c.r0, c.iosb)
		}
	}

	// A job's ENTRY_NUMBER_OUTPUT must be a longword: JBC$_INVPARLEN.
	if _, st := sndjbc(t, env, a, "ENTER_FILE", a.items(sjcItem(a, "QUEUE", "BAT98_Q"),
		sjcItem(a, "FILE_SPECIFICATION", "DUA0:[000000]BAT98OK.COM"),
		outputItem(a, "ENTRY_NUMBER_OUTPUT", 256)), true); st != jbcInvParLen {
		t.Errorf("a 256-byte ENTRY_NUMBER_OUTPUT: %#x", st)
	}

	// Cases 44 to 48: one job of two files, by CREATE_JOB, ADD_FILE, and
	// CLOSE_JOB.
	created := outputItem(a, "ENTRY_NUMBER_OUTPUT", 4)

	for _, c := range []struct {
		function string
		itmlst   uint32
	}{
		{"CREATE_JOB", a.items(sjcItem(a, "QUEUE", "BAT98_Q"), sjcItem(a, "HOLD", nil),
			sjcItem(a, "JOB_NAME", "MULTI"), created)},
		{"ADD_FILE", a.items(sjcItem(a, "FILE_SPECIFICATION", "DUA0:[000000]BAT98OK.COM"))},
		{"ADD_FILE", a.items(sjcItem(a, "FILE_SPECIFICATION", "DUA0:[000000]BAT98WRN.COM"))},
		{"CLOSE_JOB", 0},
	} {
		if _, st := sndjbc(t, env, a, c.function, c.itmlst, true); st != normal {
			t.Errorf("%s: %#x", c.function, st)
		}
	}

	entry = a.readLong(created.buf)
	if j := job(); j == nil || j.Name != "MULTI" || len(j.Files) != 2 || j.Status != jobqueue.JobHolding {
		t.Errorf("case 48's job (entry %d): %+v", entry, j)
	}
}
