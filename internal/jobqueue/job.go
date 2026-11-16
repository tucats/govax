package jobqueue

import (
	"slices"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// JobOp is an operation on a job whose outcome depends on the job's
// state: a row of jobRules.
type JobOp int

// The job operations.
const (
	// OpHoldJob is SET ENTRY/HOLD.
	OpHoldJob JobOp = iota
	// OpReleaseJob is SET ENTRY/RELEASE.
	OpReleaseJob
	// OpAlterJob is SET ENTRY's other changes (/AFTER, /PARAMETERS,
	// /NAME, /PRIORITY, /LOG_FILE, ...).
	OpAlterJob
	// OpDeleteJob is DELETE/ENTRY.
	OpDeleteJob
	// OpAbortJob is STOP/QUEUE/ENTRY.
	OpAbortJob
	// OpRequeueJob is STOP/QUEUE/REQUEUE/ENTRY.
	OpRequeueJob
	// OpStartJob is the manager starting a job's process.
	OpStartJob
	numJobOps
)

// jobAct is what an allowed cell of jobRules does.
type jobAct int

const (
	// actTo moves the job to the cell's next status. (0 is no action, so
	// an empty cell can be told from a filled one.)
	actTo jobAct = iota + 1
	// actByTime moves it to JobTimed if its /AFTER time is still to come,
	// otherwise JobPending.
	actByTime
	// actKeep leaves its status as it is.
	actKeep
	// actRemove takes it out of the database.
	actRemove
	// actEnd marks an executing job to end with JBC$_JOBABORT; its
	// process is the caller's to stop. With remove set it is removed when
	// it ends (DELETE/ENTRY), not retained.
	actEnd
	// actRequeue marks an executing job to be requeued when its run ends.
	actRequeue
)

// jobCell is one cell of jobRules: refuse, when set, is the JBC condition
// refusing the operation; otherwise act says what it does.
type jobCell struct {
	refuse string
	act    jobAct
	next   JobStatus
	remove bool
}

// Shorthands for jobRules' cells.
func toJob(s JobStatus) jobCell  { return jobCell{act: actTo, next: s} }
func refuseJob(n string) jobCell { return jobCell{refuse: n} }

var (
	byTime    = jobCell{act: actByTime}
	keepJob   = jobCell{act: actKeep}
	removeJob = jobCell{act: actRemove}
)

// jobRules is what each job operation does in each state: a row per
// operation, a column per status (pending, holding, timed, executing,
// retained). What VMS 7.1 showed (the phase doc's A.4): SET ENTRY of an
// executing job is EXECUTING; STOP/QUEUE/ENTRY of a job not executing
// JOBNOTEXEC; DELETE/ENTRY and STOP/QUEUE/ENTRY of an executing job end
// it with JOBABORT, the deleted one gone afterwards, the stopped one
// retained if its /RETAIN says (BAT98R11 and BAT98R10); a /HOLD/AFTER job
// is holding. govax's choices, unconfirmed: /RELEASE of a timed job
// starts it now, of a retained job requeues it (the DCL Dictionary's
// "held or retained"), and /HOLD of a retained job requeues it held;
// /REQUEUE of a job not executing is JOBNOTEXEC.
var jobRules = [numJobOps][5]jobCell{
	OpHoldJob: {
		JobPending:   toJob(JobHolding),
		JobHolding:   keepJob,
		JobTimed:     toJob(JobHolding),
		JobExecuting: refuseJob("JBC$_EXECUTING"),
		JobRetained:  toJob(JobHolding),
	},
	OpReleaseJob: {
		JobPending:   keepJob,
		JobHolding:   byTime,
		JobTimed:     toJob(JobPending),
		JobExecuting: refuseJob("JBC$_EXECUTING"),
		JobRetained:  byTime,
	},
	OpAlterJob: {
		JobPending:   keepJob,
		JobHolding:   keepJob,
		JobTimed:     keepJob,
		JobExecuting: refuseJob("JBC$_EXECUTING"),
		JobRetained:  keepJob,
	},
	OpDeleteJob: {
		JobPending:   removeJob,
		JobHolding:   removeJob,
		JobTimed:     removeJob,
		JobExecuting: {act: actEnd, remove: true},
		JobRetained:  removeJob,
	},
	OpAbortJob: {
		JobPending:   refuseJob("JBC$_JOBNOTEXEC"),
		JobHolding:   refuseJob("JBC$_JOBNOTEXEC"),
		JobTimed:     refuseJob("JBC$_JOBNOTEXEC"),
		JobExecuting: {act: actEnd},
		JobRetained:  refuseJob("JBC$_JOBNOTEXEC"),
	},
	OpRequeueJob: {
		JobPending:   refuseJob("JBC$_JOBNOTEXEC"),
		JobHolding:   refuseJob("JBC$_JOBNOTEXEC"),
		JobTimed:     refuseJob("JBC$_JOBNOTEXEC"),
		JobExecuting: {act: actRequeue},
		JobRetained:  refuseJob("JBC$_JOBNOTEXEC"),
	},
	OpStartJob: {
		JobPending:   toJob(JobExecuting),
		JobHolding:   refuseJob("JBC$_INTERNALERROR"),
		JobTimed:     refuseJob("JBC$_INTERNALERROR"),
		JobExecuting: refuseJob("JBC$_INTERNALERROR"),
		JobRetained:  refuseJob("JBC$_INTERNALERROR"),
	},
}

// timeStatus is the status a job that isn't held takes: JobTimed while its
// /AFTER time is to come, JobPending once it has come (or with none).
func (j *Job) timeStatus(now time.Time) JobStatus {
	if !j.After.IsZero() && j.After.After(now) {
		return JobTimed
	}

	return JobPending
}

// job finds the job with entry number entry, in queue (any queue when
// queue is ""), or refuses with JBC$_NOSUCHJOB, as VMS 7.1's SET ENTRY and
// DELETE/ENTRY did (SHOW ENTRY and SYNCHRONIZE say NOSUCHENT: the
// caller's to change).
func (d *Database) job(entry int, queue string) (*Job, error) {
	j := d.Job(entry)
	if j == nil || queue != "" && d.Queue(queue) != d.Queue(j.Queue) {
		return nil, refuse("JBC$_NOSUCHJOB")
	}

	return j, nil
}

// applyJob looks up op's cell for j's status and does what it says. It
// reports whether j was removed.
func (d *Database) applyJob(j *Job, op JobOp, now time.Time) (removed bool, err error) {
	cell := jobRules[op][j.Status]
	if cell.refuse != "" {
		return false, refuse(cell.refuse)
	}

	switch cell.act {
	case actTo:
		j.Status = cell.next
	case actByTime:
		j.Status = j.timeStatus(now)
	case actKeep:
	case actRemove:
		d.removeJob(j)

		return true, nil
	case actEnd:
		j.Ending = vmsdef.Symbols["JBC$_JOBABORT"]
		j.Requeue = nil
		j.Deleting = cell.remove
	case actRequeue:
		// The caller fills in j.Requeue.
	}

	// A job requeued from retention starts afresh.
	if j.Status != JobRetained && j.Status != JobExecuting && !j.Completed.IsZero() {
		j.Completed, j.CompletedQueue, j.Condition = time.Time{}, "", 0
	}

	return false, nil
}

// removeJob takes j out of the database.
func (d *Database) removeJob(j *Job) {
	if i := slices.Index(d.Jobs, j); i >= 0 {
		d.Jobs = slices.Delete(d.Jobs, i, i+1)
	}
}

// nextEntry is the entry number the next job gets: NextEntry, or the next
// after it not in use, wrapping from MaxEntry to 1. With every number in
// use it refuses with JBC$_NOQUESPACE.
func (d *Database) nextEntry() (int, error) {
	used := make(map[int]bool, len(d.Jobs))
	for _, j := range d.Jobs {
		used[j.Entry] = true
	}

	n := d.NextEntry
	for range MaxEntry {
		if !used[n] {
			d.NextEntry = n%MaxEntry + 1

			return n, nil
		}

		n = n%MaxEntry + 1
	}

	return 0, refuse("JBC$_NOQUESPACE")
}

// Submit is SUBMIT (ENTER_FILE): j entered in its queue, j.Queue, given
// an entry number and a sequence, submitted now, and held (hold), held
// until its /AFTER time, or pending. A queue that doesn't exist is
// JBC$_NOSUCHQUE; a job whose fields are out of range JBC$_INVPARVAL.
// j's fields are the caller's; Submit sets Entry, Sequence, Queue (to the
// queue's own spelling), Submitted, and Status.
func (d *Database) Submit(j *Job, hold bool, now time.Time) error {
	q, err := d.queue(j.Queue)
	if err != nil {
		return err
	}

	saved := *j

	j.Queue = q.Name
	j.Submitted = now
	j.Sequence = d.NextSequence

	if hold {
		j.Status = JobHolding
	} else {
		j.Status = j.timeStatus(now)
	}

	j.Entry = 1 // a placeholder for the check, which needs one in range

	if err := j.check(); err != nil {
		*j = saved

		return &Refusal{Name: "JBC$_INVPARVAL", Detail: err.Error()}
	}

	entry, err := d.nextEntry()
	if err != nil {
		*j = saved

		return err
	}

	j.Entry = entry
	d.NextSequence++
	d.Jobs = append(d.Jobs, j)
	d.sortJobs()

	return nil
}

// HoldJob is SET ENTRY/HOLD.
func (d *Database) HoldJob(entry int, now time.Time) error {
	j, err := d.job(entry, "")
	if err != nil {
		return err
	}

	_, err = d.applyJob(j, OpHoldJob, now)

	return err
}

// ReleaseJob is SET ENTRY/RELEASE.
func (d *Database) ReleaseJob(entry int, now time.Time) error {
	j, err := d.job(entry, "")
	if err != nil {
		return err
	}

	_, err = d.applyJob(j, OpReleaseJob, now)

	return err
}

// AlterJob is SET ENTRY's other changes: set applied to a copy of the job,
// kept if it passes the job's checks (JBC$_INVPARVAL if not). A pending or
// timed job's status follows its /AFTER time afterwards.
func (d *Database) AlterJob(entry int, now time.Time, set func(*Job)) error {
	j, err := d.job(entry, "")
	if err != nil {
		return err
	}

	if _, err := d.applyJob(j, OpAlterJob, now); err != nil {
		return err
	}

	c := *j
	c.Files = slices.Clone(j.Files)
	c.Parameters = slices.Clone(j.Parameters)

	set(&c)

	// Which job it is and where it stands aren't set's to change.
	c.Entry, c.Sequence, c.Queue, c.Status, c.Submitted = j.Entry, j.Sequence, j.Queue, j.Status, j.Submitted

	if err := c.check(); err != nil {
		return &Refusal{Name: "JBC$_INVPARVAL", Detail: err.Error()}
	}

	if c.Status == JobPending || c.Status == JobTimed {
		c.Status = c.timeStatus(now)
	}

	*j = c

	return nil
}

// DeleteJob is DELETE/ENTRY of entry, in queue when queue isn't "" (a job
// in another queue is JBC$_NOSUCHJOB). A job not executing is removed at
// once, and DeleteJob returns nil. An executing job is marked to end with
// JBC$_JOBABORT and be removed when it ends: DeleteJob returns it, and its
// process is the caller's to stop, then Finish's.
func (d *Database) DeleteJob(entry int, queue string, now time.Time) (*Job, error) {
	j, err := d.job(entry, queue)
	if err != nil {
		return nil, err
	}

	removed, err := d.applyJob(j, OpDeleteJob, now)
	if err != nil || removed {
		return nil, err
	}

	return j, nil
}

// AbortJob is STOP/QUEUE/ENTRY of entry in queue: an executing job marked
// to end with JBC$_JOBABORT; any other JBC$_JOBNOTEXEC. Its process is the
// caller's to stop, then Finish's.
func (d *Database) AbortJob(entry int, queue string, now time.Time) (*Job, error) {
	j, err := d.job(entry, queue)
	if err != nil {
		return nil, err
	}

	if _, err := d.applyJob(j, OpAbortJob, now); err != nil {
		return nil, err
	}

	return j, nil
}

// RequeueJob is STOP/QUEUE/REQUEUE/ENTRY of entry in queue: an executing
// job marked to be requeued, to r's queue (its own when r.Queue is "") and
// held if r.Hold, when its run ends. A queue r names that doesn't exist is
// JBC$_NOSUCHQUE. Its process is the caller's to stop, then Finish's.
func (d *Database) RequeueJob(entry int, queue string, r Requeue, now time.Time) (*Job, error) {
	j, err := d.job(entry, queue)
	if err != nil {
		return nil, err
	}

	if r.Queue != "" {
		q, err := d.queue(r.Queue)
		if err != nil {
			return nil, err
		}

		r.Queue = q.Name
	}

	if _, err := d.applyJob(j, OpRequeueJob, now); err != nil {
		return nil, err
	}

	j.Requeue = &r

	return j, nil
}

// SetRestartValue is SET RESTART_VALUE in an executing job: the value its
// next run, if it is restarted, finds in BATCH$RESTART.
func (d *Database) SetRestartValue(entry int, value string) error {
	j, err := d.job(entry, "")
	if err != nil {
		return err
	}

	if j.Status != JobExecuting {
		return refuse("JBC$_JOBNOTEXEC")
	}

	j.RestartValue = value

	return nil
}

// NextJob returns the job queue should start next, or nil: none unless
// the queue is started (not paused or stopping) and runs fewer jobs than
// its limit; then the pending job of highest priority, and of those the
// first submitted (the phase doc's A.4).
func (d *Database) NextJob(queue string) *Job {
	q := d.Queue(queue)
	if q == nil || q.State != QueueStarted || d.Executing(q.Name) >= q.JobLimit {
		return nil
	}

	var best *Job

	for _, j := range d.QueueJobs(q.Name) {
		if j.Status != JobPending {
			continue
		}

		if best == nil || j.Priority > best.Priority ||
			j.Priority == best.Priority && j.Sequence < best.Sequence {
			best = j
		}
	}

	return best
}

// StartJob is the manager starting the pending job entry: it is executing,
// started now. The process is the caller's to create, its PID kept in
// j.PID. $RESTART in the run is j.Restarted.
func (d *Database) StartJob(entry int, now time.Time) (*Job, error) {
	j, err := d.job(entry, "")
	if err != nil {
		return nil, err
	}

	if _, err := d.applyJob(j, OpStartJob, now); err != nil {
		return nil, err
	}

	j.Started = now

	return j, nil
}

// ReleaseDue makes every job held until a time that has come (now or
// before) pending, and returns them.
func (d *Database) ReleaseDue(now time.Time) []*Job {
	var due []*Job

	for _, j := range d.Jobs {
		if j.Status == JobTimed && !j.After.After(now) {
			j.Status = JobPending
			due = append(due, j)
		}
	}

	return due
}

// NextTime is the earliest time a job is held until, the manager's next
// timer; ok is false when no job waits for a time.
func (d *Database) NextTime() (t time.Time, ok bool) {
	for _, j := range d.Jobs {
		if j.Status == JobTimed && (!ok || j.After.Before(t)) {
			t, ok = j.After, true
		}
	}

	return t, ok
}

// RetainedOnError reports whether a retained job is "Retained on error"
// rather than "on completion": its condition's severity error or severe.
// A warning (BAT98R12's 0, its process stopped) is "on completion",
// though /RETAIN=ERROR kept it.
func (j *Job) RetainedOnError() bool {
	sev := j.Condition & 7

	return sev == 2 || sev == 4
}

// keeps reports whether a job ending with condition cond is retained, by
// the job's /RETAIN, else its queue's: ALWAYS keeps every job, ERROR one
// whose condition isn't a success (its low bit clear: BAT98R12's warning
// was kept under the queue's /RETAIN=ERROR).
func keeps(j *Job, q *Queue, cond uint32) bool {
	mode := j.Retain
	if mode == RetainDefault && q != nil {
		mode = q.Retain
	}

	switch mode {
	case RetainAlways:
		return true
	case RetainError:
		return cond&1 == 0
	}

	return false
}

// Finish is an executing job's process ending with status status. The
// job is requeued if STOP/QUEUE/REQUEUE asked (pending or held, a restart,
// checkpointed if it has a restart value); otherwise its condition is the
// reason it was ended (Ending) or status, and it is retained if /RETAIN
// says, or removed (always, if DELETE/ENTRY ended it). Finish returns the
// job if it is still in the database, nil if removed; a job no longer
// there (its queue deleted) is nothing to do. A stopping queue whose last
// job this was stops.
func (d *Database) Finish(entry int, status uint32, now time.Time) *Job {
	j := d.Job(entry)
	if j == nil || j.Status != JobExecuting {
		return nil
	}

	q := d.Queue(j.Queue)

	defer func() {
		if q != nil {
			d.settle(q)
		}
	}()

	j.PID = 0

	if r := j.Requeue; r != nil {
		j.Requeue, j.Ending = nil, 0

		if r.Queue != "" && d.Queue(r.Queue) != nil {
			j.Queue = d.Queue(r.Queue).Name
		}

		if r.Hold {
			j.Status = JobHolding
		} else {
			j.Status = j.timeStatus(now)
		}

		j.Restarted, j.Checkpointed = true, j.RestartValue != ""

		return j
	}

	cond := status
	if j.Ending != 0 {
		cond = j.Ending
	}

	deleting := j.Deleting
	j.Ending, j.Deleting = 0, false

	if deleting || !keeps(j, q, cond) {
		d.removeJob(j)

		return nil
	}

	j.Status = JobRetained
	j.Condition = cond
	j.Completed = now
	j.CompletedQueue = j.Queue

	return j
}

// Recover is what a session starting the manager does with what the last
// session left (Decision 11, held to the probe's reboot, A.4): every queue
// stopped, as VMS's non-autostart queues come back; a job left executing
// requeued, pending, a restart (checkpointed if it has a restart value),
// if it was submitted /RESTART or STOP/QUEUE/REQUEUE had asked; any other
// ended with JBC$_QUERESET (or the JBC$_JOBABORT it was already being
// ended with), retained as /RETAIN says, or removed if DELETE/ENTRY had
// asked. It returns the jobs
// requeued and the jobs ended, in that order of interest to a report.
func (d *Database) Recover(now time.Time) (requeued, ended []*Job) {
	for _, q := range d.Queues {
		q.State = QueueStopped
	}

	reset := vmsdef.Symbols["JBC$_QUERESET"]

	for _, j := range slices.Clone(d.Jobs) {
		if j.Status != JobExecuting {
			continue
		}

		if j.Restart && j.Requeue == nil && j.Ending == 0 {
			j.Requeue = &Requeue{}
		}

		if j.Requeue != nil {
			requeued = append(requeued, d.Finish(j.Entry, 0, now))

			continue
		}

		if j.Ending == 0 {
			j.Ending = reset
		}

		ended = append(ended, j)
		d.Finish(j.Entry, reset, now)
	}

	return requeued, ended
}
