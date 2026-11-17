package jobqueue

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/hostlock"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// t0 is the tests' "now".
var t0 = time.Date(2026, 11, 16, 11, 24, 43, 0, time.UTC)

// code is a JBC condition's value by name.
func code(name string) uint32 { return vmsdef.Symbols[name] }

// refusedWith reports whether err is a Refusal naming name.
func refusedWith(err error, name string) bool {
	var r *Refusal

	return errors.As(err, &r) && r.Name == name
}

// testJob is a job of one file for queue, submitted by SYSTEM.
func testJob(name, queue string) *Job {
	return &Job{
		Name:     name,
		Queue:    queue,
		User:     "SYSTEM",
		Owner:    SystemUIC,
		Priority: DefaultJobPri,
		Files:    []File{{Spec: "_GOVAX$DUA1:[000000]" + name + ".COM;1", ID: FileID{Number: 23, Sequence: 1}}},
	}
}

// withQueue is a database with one started batch queue, BAT98_A.
func withQueue(t *testing.T) *Database {
	t.Helper()

	d := New()
	if _, _, err := d.CreateQueue("BAT98_A", nil); err != nil {
		t.Fatal(err)
	}

	if err := d.StartQueue("BAT98_A", "GOVAX"); err != nil {
		t.Fatal(err)
	}

	return d
}

// submit enters j, failing the test on an error.
func submit(t *testing.T, d *Database, j *Job, hold bool) *Job {
	t.Helper()

	if err := d.Submit(j, hold, t0); err != nil {
		t.Fatal(err)
	}

	return j
}

// TestRefusalNamesExist: every condition the rules can refuse with is in
// vmsdef, so a Refusal's Code is never 0.
func TestRefusalNamesExist(t *testing.T) {
	names := []string{
		"JBC$_NOSUCHQUE", "JBC$_NOSUCHJOB", "JBC$_INVPARLEN", "JBC$_INVQUENAM",
		"JBC$_INVPARVAL", "JBC$_NOQUESPACE", "JBC$_QUERESET", "JBC$_JOBABORT",
	}

	for _, row := range queueRules {
		for _, c := range row {
			names = append(names, c.refuse)
		}
	}

	for _, row := range jobRules {
		for _, c := range row {
			names = append(names, c.refuse)
		}
	}

	for _, n := range names {
		if n == "" {
			continue
		}

		if (&Refusal{Name: n}).Code() == 0 {
			t.Errorf("%s isn't in vmsdef.Symbols", n)
		}
	}
}

// TestProtectionText: a queue's code as SHOW QUEUE/FULL writes it, read
// back the same (the probe's two codes).
func TestProtectionText(t *testing.T) {
	if got := DefaultProtection.String(); got != "(S:M,O:D,G:R,W:S)" {
		t.Errorf("default protection %s", got)
	}

	for _, text := range []string{"(S:M,O:D,G:R,W:S)", "(S:M,O:R,G:D,W)", "(S:RSMD,O:RSMD,G,W)"} {
		p, err := ParseProtection(text, Protection(security.NoAccess))
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}

		if p.String() != text {
			t.Errorf("%s read back as %s", text, p)
		}
	}

	// The letters are a file's bits under a queue's names.
	if DefaultProtection.Allows(security.World) != security.Write {
		t.Errorf("W:S isn't the write bit")
	}

	p, err := ParseProtection("group=rs", DefaultProtection)
	if err != nil || p.String() != "(S:M,O:D,G:RS,W:S)" {
		t.Errorf("merged code %s, %v", p, err)
	}

	for _, bad := range []string{"", "(X:R)", "(S:R,S:M)", "(S:W)"} {
		if _, err := ParseProtection(bad, 0); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
}

// TestCPUTimeText: limits as SHOW QUEUE/FULL writes them.
func TestCPUTimeText(t *testing.T) {
	for text, want := range map[string]CPUTime{
		"00:05:00": 30000, "00:00:03": 300, "01:00:00.50": 360050, "INFINITE": CPUInfinite,
	} {
		got, err := ParseCPUTime(text)
		if err != nil || got != want || got.String() != text {
			t.Errorf("%s: %d (%s), %v", text, got, got, err)
		}
	}

	for _, bad := range []string{"5", "00:60:00", "00:00:00.5", "x"} {
		if _, err := ParseCPUTime(bad); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
}

// TestQueueNames: the probe's INVPARLEN and INVQUENAM.
func TestQueueNames(t *testing.T) {
	d := New()

	for name, want := range map[string]string{
		strings.Repeat("Q", 32): "JBC$_INVPARLEN",
		"":                      "JBC$_INVPARLEN",
		"BAD-NAME":              "JBC$_INVQUENAM",
	} {
		if _, _, err := d.CreateQueue(name, nil); !refusedWith(err, want) {
			t.Errorf("%q: %v, want %s", name, err, want)
		}
	}

	if _, _, err := d.CreateQueue(strings.Repeat("Q", 31), nil); err != nil {
		t.Errorf("31 characters: %v", err)
	}
}

// queueStates puts a queue in each state, for TestQueueRules.
var queueStates = map[QueueState]func(*Database) error{
	QueueStopped: func(*Database) error { return nil },
	QueueStarted: func(d *Database) error { return d.StartQueue("Q", "GOVAX") },
	QueuePaused: func(d *Database) error {
		if err := d.StartQueue("Q", "GOVAX"); err != nil {
			return err
		}

		return d.PauseQueue("Q")
	},
	QueueStopping: func(d *Database) error {
		// /NEXT leaves a queue stopping only while a job runs.
		if err := d.StartQueue("Q", "GOVAX"); err != nil {
			return err
		}

		j := testJob("RUNNING", "Q")
		if err := d.Submit(j, false, t0); err != nil {
			return err
		}

		if _, err := d.StartJob(j.Entry, t0); err != nil {
			return err
		}

		return d.StopQueueNext("Q")
	},
}

// TestQueueRules runs every queue operation in every state and checks the
// outcome: the state after, or the refusal. The expectations are written
// out here, apart from queueRules, so the table is checked, not repeated.
func TestQueueRules(t *testing.T) {
	type outcome struct {
		state  QueueState
		refuse string
		gone   bool
	}

	run := func(d *Database, op QueueOp) error {
		switch op {
		case OpStartQueue:
			return d.StartQueue("Q", "GOVAX")
		case OpPauseQueue:
			return d.PauseQueue("Q")
		case OpStopQueueNext:
			return d.StopQueueNext("Q")
		case OpResetQueue:
			_, err := d.ResetQueue("Q")

			return err
		case OpDeleteQueue:
			return d.DeleteQueue("Q")
		case OpModifyQueue:
			_, created, err := d.CreateQueue("Q", func(q *Queue) { q.JobLimit = 3 })
			if created {
				t.Error("CREATE of an existing queue made a new one")
			}

			return err
		}

		return nil
	}

	S, R, P, T := QueueStopped, QueueStarted, QueuePaused, QueueStopping
	want := map[QueueOp]map[QueueState]outcome{
		OpStartQueue:    {S: {state: R}, R: {state: R, refuse: "JBC$_STARTED"}, P: {state: R}, T: {state: R}},
		OpPauseQueue:    {S: {state: S}, R: {state: P}, P: {state: P}, T: {state: P}},
		OpStopQueueNext: {S: {state: S}, R: {state: S}, P: {state: S}, T: {state: T}},
		OpResetQueue:    {S: {state: S}, R: {state: S}, P: {state: S}, T: {state: S}},
		OpDeleteQueue: {
			S: {gone: true}, R: {state: R, refuse: "JBC$_QUENOTSTOP"},
			P: {state: P, refuse: "JBC$_QUENOTSTOP"}, T: {state: T, refuse: "JBC$_QUENOTSTOP"},
		},
		OpModifyQueue: {
			S: {state: S}, R: {state: R, refuse: "JBC$_QUENOTMOD"},
			P: {state: P, refuse: "JBC$_QUENOTMOD"}, T: {state: T, refuse: "JBC$_QUENOTMOD"},
		},
	}

	for op := range numQueueOps {
		for state, setUp := range queueStates {
			d := New()
			if _, _, err := d.CreateQueue("Q", nil); err != nil {
				t.Fatal(err)
			}

			if err := setUp(d); err != nil {
				t.Fatal(err)
			}

			w := want[op][state]
			err := run(d, op)

			switch {
			case w.refuse != "" && !refusedWith(err, w.refuse):
				t.Errorf("op %d in %s: %v, want %s", op, state, err, w.refuse)
			case w.refuse == "" && err != nil:
				t.Errorf("op %d in %s: %v", op, state, err)
			}

			q := d.Queue("Q")

			switch {
			case w.gone && q != nil:
				t.Errorf("op %d in %s: the queue is still there", op, state)
			case !w.gone && q == nil:
				t.Errorf("op %d in %s: the queue is gone", op, state)
			case !w.gone && q.State != w.state:
				t.Errorf("op %d in %s: now %s, want %s", op, state, q.State, w.state)
			}

			// A modification refused changes nothing; allowed, it changes.
			if op == OpModifyQueue && q != nil && (q.JobLimit == 3) != (w.refuse == "") {
				t.Errorf("op %d in %s: job limit %d", op, state, q.JobLimit)
			}
		}
	}
}

// TestQueueRulesEveryCell: queueRules has a cell for every operation and
// state, none left at the zero value by mistake (a zero cell would move a
// queue to stopped).
func TestQueueRulesEveryCell(t *testing.T) {
	for op, row := range queueRules {
		for state, c := range row {
			if !c.set {
				t.Errorf("op %d in %s: an empty cell", op, QueueState(state))
			}
		}
	}

	for op, row := range jobRules {
		for status, c := range row {
			if c.act == 0 && c.refuse == "" {
				t.Errorf("op %d in %s: an empty cell", op, JobStatus(status))
			}
		}
	}
}

// TestCreateAndSetQueue: defaults, the qualifiers' changes, a value out
// of range refused with nothing changed, an unknown queue.
func TestCreateAndSetQueue(t *testing.T) {
	d := New()

	q, created, err := d.CreateQueue("bat98_b", nil)
	if err != nil || !created {
		t.Fatal(created, err)
	}

	if q.JobLimit != 1 || q.BasePriority != 4 || q.Owner != SystemUIC ||
		q.Protection != DefaultProtection || q.State != QueueStopped || q.Node != "" {
		t.Errorf("defaults %+v", q)
	}

	if d.Queue("BAT98_B") != q {
		t.Error("names aren't compared ignoring case")
	}

	err = d.SetQueue("BAT98_B", func(q *Queue) { q.JobLimit = 0 })
	if !refusedWith(err, "JBC$_INVPARVAL") || q.JobLimit != 1 {
		t.Errorf("job limit 0: %v, %d", err, q.JobLimit)
	}

	err = d.SetQueue("BAT98_B", func(q *Queue) {
		q.JobLimit, q.CPUDefault, q.Retain, q.Description = 3, 30000, RetainError, "<GOVAX PROBE A>"
		q.Name = "RENAMED" // not set's to change
	})
	if err != nil || q.JobLimit != 3 || q.Retain != RetainError || q.Name != "bat98_b" {
		t.Errorf("SET QUEUE: %v, %+v", err, q)
	}

	if err := d.SetQueue("NONE", nil); !refusedWith(err, "JBC$_NOSUCHQUE") {
		t.Errorf("missing queue: %v", err)
	}

	if err := d.StartQueue("BAT98_B", "GOVAX"); err != nil || q.Node != "GOVAX" {
		t.Errorf("start: %v, node %q", err, q.Node)
	}

	// SET QUEUE works on a started queue.
	if err := d.SetQueue("BAT98_B", func(q *Queue) { q.BasePriority = 3 }); err != nil {
		t.Error(err)
	}
}

// TestStateText: SHOW QUEUE's words for a started queue by its jobs.
func TestStateText(t *testing.T) {
	d := withQueue(t)
	q := d.Queue("BAT98_A")
	_ = d.SetQueue("BAT98_A", func(q *Queue) { q.JobLimit = 2 })

	if got := d.StateText(q); got != "idle" {
		t.Errorf("idle: %s", got)
	}

	a := submit(t, d, testJob("A", "BAT98_A"), false)
	_, _ = d.StartJob(a.Entry, t0)

	if got := d.StateText(q); got != "available" {
		t.Errorf("one of two: %s", got)
	}

	_ = d.StopQueueNext("BAT98_A")

	if got := d.StateText(q); got != "available, stop pending" {
		t.Errorf("stopping: %s", got)
	}

	_ = d.StartQueue("BAT98_A", "GOVAX")
	b := submit(t, d, testJob("B", "BAT98_A"), false)
	_, _ = d.StartJob(b.Entry, t0)

	if got := d.StateText(q); got != "busy" {
		t.Errorf("two of two: %s", got)
	}

	_ = d.PauseQueue("BAT98_A")

	if got := d.StateText(q); got != "paused" {
		t.Errorf("paused: %s", got)
	}
}

// jobStates puts a job in each status, for TestJobRules.
var jobStates = map[JobStatus]func(*testing.T, *Database) *Job{
	JobPending: func(t *testing.T, d *Database) *Job { return submit(t, d, testJob("J", "BAT98_A"), false) },
	JobHolding: func(t *testing.T, d *Database) *Job { return submit(t, d, testJob("J", "BAT98_A"), true) },
	JobTimed: func(t *testing.T, d *Database) *Job {
		j := testJob("J", "BAT98_A")
		j.After = t0.Add(time.Hour)

		return submit(t, d, j, false)
	},
	JobExecuting: func(t *testing.T, d *Database) *Job {
		j := submit(t, d, testJob("J", "BAT98_A"), false)
		if _, err := d.StartJob(j.Entry, t0); err != nil {
			t.Fatal(err)
		}

		return j
	},
	JobRetained: func(t *testing.T, d *Database) *Job {
		j := submit(t, d, testJob("J", "BAT98_A"), false)
		j.Retain = RetainAlways

		if _, err := d.StartJob(j.Entry, t0); err != nil {
			t.Fatal(err)
		}

		if d.Finish(j.Entry, 1, t0) == nil {
			t.Fatal("not retained")
		}

		return j
	},
}

// TestJobRules runs every job operation in every status and checks the
// outcome against expectations written out apart from jobRules.
func TestJobRules(t *testing.T) {
	type outcome struct {
		status  JobStatus
		refuse  string
		gone    bool
		ending  uint32
		requeue bool
	}

	run := func(d *Database, j *Job, op JobOp) error {
		var err error

		switch op {
		case OpHoldJob:
			err = d.HoldJob(j.Entry, t0)
		case OpReleaseJob:
			err = d.ReleaseJob(j.Entry, t0)
		case OpAlterJob:
			err = d.AlterJob(j.Entry, t0, func(j *Job) { j.Note = "CHANGED" })
		case OpDeleteJob:
			_, err = d.DeleteJob(j.Entry, "", t0)
		case OpAbortJob:
			_, err = d.AbortJob(j.Entry, "BAT98_A", t0)
		case OpRequeueJob:
			_, err = d.RequeueJob(j.Entry, "", Requeue{Hold: true}, t0)
		case OpStartJob:
			_, err = d.StartJob(j.Entry, t0)
		}

		return err
	}

	Pn, H, Tm, X, Rt := JobPending, JobHolding, JobTimed, JobExecuting, JobRetained
	abort := code("JBC$_JOBABORT")
	notExec := func(s JobStatus) outcome { return outcome{status: s, refuse: "JBC$_JOBNOTEXEC"} }
	internal := func(s JobStatus) outcome { return outcome{status: s, refuse: "JBC$_INTERNALERROR"} }
	executing := outcome{status: X, refuse: "JBC$_EXECUTING"}

	want := map[JobOp]map[JobStatus]outcome{
		OpHoldJob:    {Pn: {status: H}, H: {status: H}, Tm: {status: H}, X: executing, Rt: {status: H}},
		OpReleaseJob: {Pn: {status: Pn}, H: {status: Pn}, Tm: {status: Pn}, X: executing, Rt: {status: Pn}},
		OpAlterJob:   {Pn: {status: Pn}, H: {status: H}, Tm: {status: Tm}, X: executing, Rt: {status: Rt}},
		OpDeleteJob: {
			Pn: {gone: true}, H: {gone: true}, Tm: {gone: true},
			X: {status: X, ending: abort}, Rt: {gone: true},
		},
		OpAbortJob:   {Pn: notExec(Pn), H: notExec(H), Tm: notExec(Tm), X: {status: X, ending: abort}, Rt: notExec(Rt)},
		OpRequeueJob: {Pn: notExec(Pn), H: notExec(H), Tm: notExec(Tm), X: {status: X, requeue: true}, Rt: notExec(Rt)},
		OpStartJob: {
			Pn: {status: X}, H: internal(H), Tm: internal(Tm),
			X: internal(X), Rt: internal(Rt),
		},
	}

	for op := range numJobOps {
		for status, setUp := range jobStates {
			d := withQueue(t)
			j := setUp(t, d)

			if j.Status != status {
				t.Fatalf("set up %s: got %s", status, j.Status)
			}

			w := want[op][status]
			err := run(d, j, op)

			switch {
			case w.refuse != "" && !refusedWith(err, w.refuse):
				t.Errorf("op %d in %s: %v, want %s", op, status, err, w.refuse)
			case w.refuse == "" && err != nil:
				t.Errorf("op %d in %s: %v", op, status, err)
			}

			if w.gone {
				if d.Job(j.Entry) != nil {
					t.Errorf("op %d in %s: the job is still there", op, status)
				}

				continue
			}

			if d.Job(j.Entry) == nil {
				t.Errorf("op %d in %s: the job is gone", op, status)

				continue
			}

			if j.Status != w.status || j.Ending != w.ending || (j.Requeue != nil) != w.requeue {
				t.Errorf("op %d in %s: %s, ending %#x, requeue %v; want %+v",
					op, status, j.Status, j.Ending, j.Requeue != nil, w)
			}

			if err := d.Check(); err != nil {
				t.Errorf("op %d in %s: %v", op, status, err)
			}
		}
	}
}

// TestMissingJob: an entry that isn't there, or in another queue, is
// NOSUCHJOB.
func TestMissingJob(t *testing.T) {
	d := withQueue(t)
	_, _, _ = d.CreateQueue("BAT98_B", nil)
	j := submit(t, d, testJob("J", "BAT98_A"), false)

	if _, err := d.DeleteJob(j.Entry, "BAT98_B", t0); !refusedWith(err, "JBC$_NOSUCHJOB") {
		t.Errorf("another queue: %v", err)
	}

	if err := d.HoldJob(500, t0); !refusedWith(err, "JBC$_NOSUCHJOB") {
		t.Errorf("no such entry: %v", err)
	}

	if err := d.Submit(testJob("X", "NONE"), false, t0); !refusedWith(err, "JBC$_NOSUCHQUE") {
		t.Errorf("no such queue: %v", err)
	}

	bad := testJob("X", "BAT98_A")
	bad.Parameters = make([]string, 9)

	if err := d.Submit(bad, false, t0); !refusedWith(err, "JBC$_INVPARVAL") || bad.Entry != 0 {
		t.Errorf("nine parameters: %v, entry %d", err, bad.Entry)
	}
}

// TestAlterJob: SET ENTRY's changes, /AFTER moving a job between pending
// and timed, and what set may not change.
func TestAlterJob(t *testing.T) {
	d := withQueue(t)
	j := submit(t, d, testJob("J", "BAT98_A"), false)
	entry := j.Entry

	err := d.AlterJob(entry, t0, func(j *Job) {
		j.After = t0.Add(time.Minute)
		j.Parameters = []string{"CHANGED"}
		j.Entry = 77
	})
	if err != nil || j.Status != JobTimed || j.Entry != entry || j.Parameters[0] != "CHANGED" {
		t.Errorf("after: %v, %+v", err, j)
	}

	if err := d.AlterJob(entry, t0, func(j *Job) { j.After = time.Time{} }); err != nil || j.Status != JobPending {
		t.Errorf("no after: %v, %s", err, j.Status)
	}

	if err := d.AlterJob(entry, t0, func(j *Job) { j.Priority = 256 }); !refusedWith(err, "JBC$_INVPARVAL") || j.Priority != 100 {
		t.Errorf("priority 256: %v, %d", err, j.Priority)
	}
}

// TestEntryNumbers: numbers from 1, wrapping after 999 past numbers in
// use, and NOQUESPACE with every number taken.
func TestEntryNumbers(t *testing.T) {
	d := withQueue(t)

	if j := submit(t, d, testJob("A", "BAT98_A"), false); j.Entry != 1 {
		t.Errorf("first entry %d", j.Entry)
	}

	d.NextEntry = MaxEntry

	if j := submit(t, d, testJob("B", "BAT98_A"), false); j.Entry != MaxEntry {
		t.Errorf("last entry %d", j.Entry)
	}

	// Entry 1 is still in use: the next is 2.
	if j := submit(t, d, testJob("C", "BAT98_A"), false); j.Entry != 2 {
		t.Errorf("after wrapping %d", j.Entry)
	}

	for range MaxEntry - 3 {
		submit(t, d, testJob("F", "BAT98_A"), false)
	}

	if err := d.Submit(testJob("Z", "BAT98_A"), false, t0); !refusedWith(err, "JBC$_NOQUESPACE") {
		t.Errorf("all numbers used: %v", err)
	}
}

// TestNextJob: priority first, then the order submitted; nothing past
// the job limit, from a held or timed job, or from a queue not started.
func TestNextJob(t *testing.T) {
	d := withQueue(t)
	_ = d.SetQueue("BAT98_A", func(q *Queue) { q.JobLimit = 2 })

	low := testJob("LOW", "BAT98_A")
	low.Priority = 50
	submit(t, d, low, false)

	first := submit(t, d, testJob("FIRST", "BAT98_A"), false)
	submit(t, d, testJob("SECOND", "BAT98_A"), false)
	submit(t, d, testJob("HELD", "BAT98_A"), true)

	timed := testJob("TIMED", "BAT98_A")
	timed.After = t0.Add(time.Hour)
	timed.Priority = 200
	submit(t, d, timed, false)

	// Entry numbers wrapped: order by submission, not by entry.
	d.NextEntry = 1

	var order []string

	for {
		j := d.NextJob("BAT98_A")
		if j == nil {
			break
		}

		order = append(order, j.Name)

		if _, err := d.StartJob(j.Entry, t0); err != nil {
			t.Fatal(err)
		}
	}

	if strings.Join(order, " ") != "FIRST SECOND" {
		t.Errorf("first two: %v", order)
	}

	d.Finish(first.Entry, 1, t0)

	if j := d.NextJob("BAT98_A"); j == nil || j.Name != "LOW" {
		t.Errorf("after one ends: %v", j)
	}

	_ = d.PauseQueue("BAT98_A")

	if j := d.NextJob("BAT98_A"); j != nil {
		t.Errorf("a paused queue starts %s", j.Name)
	}
}

// TestTimedJobs: NextTime is the earliest /AFTER, and ReleaseDue makes a
// job whose time has come pending; /HOLD/AFTER is holding, and released
// before its time it is timed.
func TestTimedJobs(t *testing.T) {
	d := withQueue(t)

	if _, ok := d.NextTime(); ok {
		t.Error("a time with no timed jobs")
	}

	late := testJob("LATE", "BAT98_A")
	late.After = t0.Add(2 * time.Hour)
	submit(t, d, late, false)

	soon := testJob("SOON", "BAT98_A")
	soon.After = t0.Add(time.Hour)
	submit(t, d, soon, false)

	past := testJob("PAST", "BAT98_A")
	past.After = t0.Add(-time.Hour)

	if submit(t, d, past, false); past.Status != JobPending {
		t.Errorf("a past /AFTER: %s", past.Status)
	}

	if next, ok := d.NextTime(); !ok || !next.Equal(soon.After) {
		t.Errorf("next time %v", next)
	}

	if due := d.ReleaseDue(t0.Add(time.Hour)); len(due) != 1 || due[0] != soon || soon.Status != JobPending {
		t.Errorf("due %v", due)
	}

	held := testJob("HELD", "BAT98_A")
	held.After = t0.Add(time.Hour)

	if submit(t, d, held, true); held.Status != JobHolding {
		t.Errorf("/HOLD/AFTER: %s", held.Status)
	}

	// VMS 7.1 made a /HOLD/AFTER job pending when it was released, its
	// time to come or not (BAT98.LOG's case 132).
	if _ = d.ReleaseJob(held.Entry, t0); held.Status != JobPending {
		t.Errorf("released before its time: %s", held.Status)
	}
}

// TestFinishRetains: /RETAIN by the job, else the queue; DELETE/ENTRY's
// job always gone; the condition and its label.
func TestFinishRetains(t *testing.T) {
	type tc struct {
		name      string
		queue     Retain
		job       Retain
		status    uint32
		abort     bool
		delete    bool
		kept      bool
		onError   bool
		condition uint32
	}

	fnf := code("RMS$_FNF")

	cases := []tc{
		{name: "none", status: 1},
		{name: "none, failed", status: fnf},
		{name: "queue ERROR, success", queue: RetainError, status: 1},
		{name: "queue ERROR, error", queue: RetainError, status: fnf, kept: true, onError: true, condition: fnf},
		{name: "queue ERROR, warning", queue: RetainError, status: 0, kept: true, condition: 0},
		{name: "job ALWAYS", job: RetainAlways, status: 1, kept: true, condition: 1},
		{name: "job NEVER over queue ALWAYS", queue: RetainAlways, job: RetainNever, status: fnf},
		{name: "job ERROR over queue ALWAYS", queue: RetainAlways, job: RetainError, status: 1},
		{
			name: "STOP/QUEUE/ENTRY", queue: RetainError, status: 1, abort: true,
			kept: true, onError: true, condition: code("JBC$_JOBABORT"),
		},
		{name: "DELETE/ENTRY", queue: RetainAlways, status: 1, delete: true},
	}

	for _, c := range cases {
		d := withQueue(t)
		_ = d.SetQueue("BAT98_A", func(q *Queue) { q.Retain = c.queue })

		j := testJob("J", "BAT98_A")
		j.Retain = c.job
		submit(t, d, j, false)

		if _, err := d.StartJob(j.Entry, t0); err != nil {
			t.Fatal(err)
		}

		if c.abort {
			_, _ = d.AbortJob(j.Entry, "", t0)
		}

		if c.delete {
			_, _ = d.DeleteJob(j.Entry, "", t0)
		}

		got := d.Finish(j.Entry, c.status, t0.Add(time.Second))
		if (got != nil) != c.kept {
			t.Errorf("%s: kept %v", c.name, got != nil)

			continue
		}

		if got == nil {
			if d.Job(j.Entry) != nil {
				t.Errorf("%s: still in the database", c.name)
			}

			continue
		}

		if got.Status != JobRetained || got.Condition != c.condition || got.RetainedOnError() != c.onError ||
			got.CompletedQueue != "BAT98_A" || got.Ending != 0 {
			t.Errorf("%s: %+v", c.name, got)
		}
	}
}

// TestRequeue: STOP/QUEUE/REQUEUE's job requeued when its run ends, held
// or not, to its own queue or another, a restart, checkpointed with a
// restart value; a stopping queue stops after its last job.
func TestRequeue(t *testing.T) {
	d := withQueue(t)
	_, _, _ = d.CreateQueue("BAT98_B", nil)

	j := submit(t, d, testJob("J", "BAT98_A"), false)
	_, _ = d.StartJob(j.Entry, t0)

	if err := d.SetRestartValue(j.Entry, "STEP2"); err != nil {
		t.Fatal(err)
	}

	if _, err := d.RequeueJob(j.Entry, "", Requeue{Queue: "NONE"}, t0); !refusedWith(err, "JBC$_NOSUCHQUE") {
		t.Errorf("to a missing queue: %v", err)
	}

	if _, err := d.RequeueJob(j.Entry, "", Requeue{Queue: "bat98_b", Hold: true}, t0); err != nil {
		t.Fatal(err)
	}

	_ = d.StopQueueNext("BAT98_A")

	if got := d.Finish(j.Entry, 1, t0); got != j || j.Status != JobHolding || j.Queue != "BAT98_B" ||
		!j.Restarted || !j.Checkpointed || j.Requeue != nil {
		t.Errorf("requeued: %+v", j)
	}

	if q := d.Queue("BAT98_A"); q.State != QueueStopped {
		t.Errorf("the stopping queue is %s", q.State)
	}

	if err := d.SetRestartValue(j.Entry, "X"); !refusedWith(err, "JBC$_JOBNOTEXEC") {
		t.Errorf("SET RESTART_VALUE of a held job: %v", err)
	}
}

// TestResetAndDeleteQueue: /RESET ends executing jobs with QUERESET;
// DELETE/QUEUE takes the queue's jobs with it.
func TestResetAndDeleteQueue(t *testing.T) {
	d := withQueue(t)
	_ = d.SetQueue("BAT98_A", func(q *Queue) { q.Retain = RetainError })

	j := submit(t, d, testJob("J", "BAT98_A"), false)
	_, _ = d.StartJob(j.Entry, t0)
	waiting := submit(t, d, testJob("W", "BAT98_A"), false)

	ending, err := d.ResetQueue("BAT98_A")
	if err != nil || len(ending) != 1 || ending[0] != j {
		t.Fatalf("reset: %v, %v", ending, err)
	}

	if got := d.Finish(j.Entry, 0x2C, t0); got == nil || got.Condition != code("JBC$_QUERESET") {
		t.Errorf("reset job: %+v", got)
	}

	if err := d.DeleteQueue("BAT98_A"); err != nil {
		t.Fatal(err)
	}

	if len(d.Jobs) != 0 || d.Job(waiting.Entry) != nil {
		t.Errorf("jobs left: %v", d.Jobs)
	}
}

// TestRecover: a new session's start (the probe's reboot): every queue
// stopped, a /RESTART job requeued with its restart value, any other
// executing job ended with QUERESET; waiting jobs untouched.
func TestRecover(t *testing.T) {
	d := withQueue(t)
	_ = d.SetQueue("BAT98_A", func(q *Queue) { q.Retain, q.JobLimit = RetainError, 5 })

	restart := testJob("BOOTR", "BAT98_A")
	restart.Restart = true
	submit(t, d, restart, false)

	plain := submit(t, d, testJob("BOOTN", "BAT98_A"), false)
	deleted := submit(t, d, testJob("GONE", "BAT98_A"), false)

	for _, j := range []*Job{restart, plain, deleted} {
		if _, err := d.StartJob(j.Entry, t0); err != nil {
			t.Fatal(err)
		}
	}

	_ = d.SetRestartValue(restart.Entry, "STEP2")
	_, _ = d.DeleteJob(deleted.Entry, "", t0)
	held := submit(t, d, testJob("HELD", "BAT98_A"), true)

	requeued, ended := d.Recover(t0.Add(time.Hour), nil)

	if q := d.Queue("BAT98_A"); q.State != QueueStopped {
		t.Errorf("queue %s", q.State)
	}

	if len(requeued) != 1 || requeued[0] != restart || restart.Status != JobPending ||
		!restart.Restarted || !restart.Checkpointed || restart.RestartValue != "STEP2" {
		t.Errorf("requeued %v: %+v", requeued, restart)
	}

	if len(ended) != 2 || plain.Status != JobRetained || plain.Condition != code("JBC$_QUERESET") || !plain.RetainedOnError() {
		t.Errorf("ended %v: %+v", ended, plain)
	}

	if d.Job(deleted.Entry) != nil {
		t.Error("the deleted job is still there")
	}

	if held.Status != JobHolding {
		t.Errorf("held job %s", held.Status)
	}
}

// TestRecoverSparesLiveJobs: a session that stops and starts its manager
// again recovers only the jobs whose processes are gone; one still
// running here goes on, and its queue is stopped like the rest.
func TestRecoverSparesLiveJobs(t *testing.T) {
	d := withQueue(t)
	_ = d.SetQueue("BAT98_A", func(q *Queue) { q.JobLimit = 5 })

	live := submit(t, d, testJob("LIVE", "BAT98_A"), false)
	gone := submit(t, d, testJob("GONE", "BAT98_A"), false)

	for _, j := range []*Job{live, gone} {
		if _, err := d.StartJob(j.Entry, t0); err != nil {
			t.Fatal(err)
		}
	}

	_, ended := d.Recover(t0, func(entry int) bool { return entry == live.Entry })

	if live.Status != JobExecuting || len(ended) != 1 || ended[0] != gone {
		t.Errorf("live %s, ended %v", live.Status, ended)
	}

	if d.Job(gone.Entry) != nil {
		t.Error("the job without a process is still there (no /RETAIN)")
	}
}

// TestRequeueKeepsFile: a job requeued in its second file restarts there,
// checkpointed with its restart value; a job started afresh begins with
// its first file.
func TestRequeueKeepsFile(t *testing.T) {
	d := withQueue(t)

	j := testJob("TWO", "BAT98_A")
	j.Files = append(j.Files, File{Spec: "_GOVAX$DUA1:[000000]B.COM;1", ID: FileID{Number: 24, Sequence: 1}})
	submit(t, d, j, false)

	if _, err := d.StartJob(j.Entry, t0); err != nil {
		t.Fatal(err)
	}

	if err := d.SetJobFile(j.Entry, 1); err != nil {
		t.Fatal(err)
	}

	if err := d.SetJobFile(j.Entry, 7); err != nil || j.File != 1 {
		t.Errorf("a file past the job's: %v, file %d", err, j.File)
	}

	_ = d.SetRestartValue(j.Entry, "STEP2")

	if _, err := d.RequeueJob(j.Entry, "BAT98_A", Requeue{Hold: true}, t0); err != nil {
		t.Fatal(err)
	}

	if got := d.Finish(j.Entry, 0x10000004, t0); got != j || j.Status != JobHolding ||
		!j.Restarted || !j.Checkpointed || j.File != 1 {
		t.Fatalf("requeued: %+v", j)
	}

	if err := d.Check(); err != nil {
		t.Fatal(err)
	}

	if err := d.ReleaseJob(j.Entry, t0); err != nil {
		t.Fatal(err)
	}

	if _, err := d.StartJob(j.Entry, t0); err != nil || j.File != 1 {
		t.Errorf("restart begins with file %d: %v", j.File, err)
	}

	if err := d.SetJobFile(j.Entry+1, 0); !refusedWith(err, "JBC$_NOSUCHJOB") {
		t.Errorf("SetJobFile of no job: %v", err)
	}
}

// TestExpireRetained: a job its own /RETAIN=UNTIL kept goes at its time,
// which NextTime gives; one its queue's /RETAIN kept stays.
func TestExpireRetained(t *testing.T) {
	d := withQueue(t)
	_ = d.SetQueue("BAT98_A", func(q *Queue) { q.Retain, q.JobLimit = RetainAlways, 5 })

	timed := testJob("TIMED", "BAT98_A")
	timed.Retain, timed.RetainUntil = RetainUntil, t0.Add(time.Hour)
	submit(t, d, timed, false)

	kept := submit(t, d, testJob("KEPT", "BAT98_A"), false)

	for _, j := range []*Job{timed, kept} {
		if _, err := d.StartJob(j.Entry, t0); err != nil {
			t.Fatal(err)
		}

		d.Finish(j.Entry, 1, t0)
	}

	if next, ok := d.NextTime(); !ok || !next.Equal(timed.RetainUntil) {
		t.Errorf("next time %v, %v", next, ok)
	}

	if gone := d.ExpireRetained(t0.Add(time.Minute)); len(gone) != 0 {
		t.Errorf("early: %v", gone)
	}

	if gone := d.ExpireRetained(t0.Add(time.Hour)); len(gone) != 1 || gone[0] != timed {
		t.Errorf("expired %v", gone)
	}

	if d.Job(kept.Entry) == nil || d.Job(timed.Entry) != nil {
		t.Errorf("jobs left: %v", d.Jobs)
	}

	if _, ok := d.NextTime(); ok {
		t.Error("a next time with nothing waiting")
	}
}

// TestFileRoundTrip: Save and Load give back the same database, the file
// readable JSON of mode 0600 in VMS's forms.
func TestFileRoundTrip(t *testing.T) {
	d := withQueue(t)
	_ = d.SetQueue("BAT98_A", func(q *Queue) {
		q.CPUDefault, q.CPUMaximum, q.Retain = 30000, CPUInfinite, RetainError
		q.Owner = UIC(security.NewUIC(0o200, 1))
		q.ACL = []ACE{security.NewIdentifierACE([]uint32{uint32(security.NewUIC(0o200, 1))}, security.Read|security.Write, 0).Data}
	})

	j := testJob("J", "BAT98_A")
	j.Parameters = []string{"A", "B"}
	j.After = t0.Add(time.Hour)
	submit(t, d, j, false)
	d.StartManager("SYS$SYSROOT:[SYSEXE]", t0)

	path := filepath.Join(t.TempDir(), "qman.json")
	if err := d.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != fileMode {
		t.Errorf("mode %v, %v", info.Mode(), err)
	}

	text, _ := os.ReadFile(path)
	for _, want := range []string{
		`"owner": "[200,1]"`, `"protection": "(S:M,O:D,G:R,W:S)"`, `"cpudefault": "00:05:00"`,
		`"cpumaximum": "INFINITE"`, `"retain": "ERROR"`, `"status": "TIMED"`, `"id": "(23,1,0)"`,
		`"state": "STARTED"`,
	} {
		if !strings.Contains(string(text), want) {
			t.Errorf("the file has no %s:\n%s", want, text)
		}
	}

	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	again, _ := back.Marshal()
	if string(again) != string(text) {
		t.Errorf("read back differently:\n%s\n%s", text, again)
	}

	if back.Queue("BAT98_A").Profile().ACL[0].Access != security.Read|security.Write {
		t.Errorf("ACL %+v", back.Queue("BAT98_A").Profile().ACL)
	}
}

// TestParseErrors: what Load refuses, naming the line.
func TestParseErrors(t *testing.T) {
	good := New()
	_, _, _ = good.CreateQueue("Q", nil)
	text, _ := good.Marshal()

	cases := map[string]struct {
		old, new string
		line     int
	}{
		"bad UIC":       {`"owner": "[1,4]"`, `"owner": "[1,9]"`, -1},
		"unknown field": {`"job_limit"`, `"jobs_limit"`, 0},
		"version":       {`"version": 1`, `"version": 2`, 0},
		"bad state":     {`"state": "STOPPED"`, `"state": "ASLEEP"`, 0},
		"job limit":     {`"job_limit": 1`, `"job_limit": 0`, 0},
	}

	dir := t.TempDir()

	for name, c := range cases {
		if !strings.Contains(string(text), c.old) {
			t.Fatalf("%s: the file has no %s", name, c.old)
		}

		if c.line < 0 { // the line the change is on
			c.line = strings.Count(string(text[:strings.Index(string(text), c.old)]), "\n") + 1
		}

		path := filepath.Join(dir, "q.json")
		_ = os.WriteFile(path, []byte(strings.Replace(string(text), c.old, c.new, 1)), fileMode)

		_, err := Load(path)

		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("%s: %v", name, err)

			continue
		}

		if c.line != 0 && pe.Line != c.line {
			t.Errorf("%s: line %d, want %d (%v)", name, pe.Line, c.line, err)
		}
	}
}

// TestUpdateAndLocks: Update makes the file, keeps a change, and writes
// nothing when change fails; Read of a missing file is empty; the
// manager's lock is held by one session at a time.
func TestUpdateAndLocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile", "qman.json")

	if err := Ensure(path); err != nil {
		t.Fatal(err)
	}

	if d, err := Read(path); err != nil || len(d.Queues) != 0 {
		t.Fatalf("read of no file: %v, %v", d, err)
	}

	err := Update(path, func(d *Database) error {
		_, _, err := d.CreateQueue("SYS$BATCH", nil)

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	err = Update(path, func(d *Database) error {
		_, _, _ = d.CreateQueue("LOST", nil)

		return refuse("JBC$_NOPRIV")
	})
	if !refusedWith(err, "JBC$_NOPRIV") {
		t.Errorf("change's error: %v", err)
	}

	d, err := Read(path)
	if err != nil || d.Queue("SYS$BATCH") == nil || d.Queue("LOST") != nil {
		t.Errorf("after the updates: %v, %v", d, err)
	}

	first, err := LockManager(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := LockManager(path); !errors.Is(err, hostlock.ErrLocked) {
		t.Errorf("a second manager: %v", err)
	}

	_ = first.Close()

	second, err := LockManager(path)
	if err != nil {
		t.Errorf("after the first let go: %v", err)
	}

	_ = second.Close()
}
