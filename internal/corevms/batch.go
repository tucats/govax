package corevms

import (
	"fmt"
	"io"
	"strings"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmsdef"
)

// Batch jobs (docs/planning/PHASE-98 - batch queues.md, subtask 8;
// Decisions 5 to 7).
//
// On VMS the job controller starts a job by creating a detached process
// running LOGINOUT, which logs the process in to the job's user, opens its
// log file as SYS$OUTPUT, and starts DCL, which runs the login procedures
// and then the job's files. govax's queue manager is Go code (queues.go),
// so the job controller is too: whenever the database changes in this
// session (a job entered or released, a queue started, a job ended, an
// /AFTER time come), settleJobs starts what can start, as the database's
// NextJob chooses it (a started queue below its job limit, its pending job
// of highest priority, then the first submitted), each through
// startBatchJob:
//
//   - the account is logged in as a non-interactive login (BeginLogin),
//     so a DISUSER account's job ends at once with LOGIN-F-DISUSER;
//   - a detached process is created by $CREPRC's code, as the job
//     controller, which may give anything (CreateRequest.jobController):
//     named BATCH_n for its entry, with PRC$M_BATCH (CREPRC_FLAGS 16, as
//     VMS 7.1's job showed), the account's UIC, SYS$INPUT NL: (its TT
//     _NLA0:), and SYS$OUTPUT and SYS$ERROR its log file, which the
//     process's startup creates as the job's user, so the user's
//     protection decides (a log it can't make ends the job with RMS$_PRV,
//     prepareImage);
//   - the process is the account's (LogIn), but its base and authorized
//     priorities are the queue's /BASE_PRIORITY, and its CPU limit the
//     job's /CPUTIME, else the queue's /CPUDEFAULT, no more than the
//     queue's /CPUMAXIMUM; the working-set values stay the account's (the
//     probe's smaller queue values didn't lower them, unconfirmed);
//   - its CLI is DCL in batch mode (CLIStartup.Batch, internal/dclshell's
//     dclbatch.go), which runs the login procedures and then the files.
//
// When the process is deleted, however it ends (its CLI logging out, a
// STOP of the process, DELETE/ENTRY, STOP/QUEUE/ENTRY or /RESET, its CPU
// time running out), completeBatchJob finishes the job in the database
// with the process's final status, or the reason it was ended
// (jobqueue's Finish: retained by /RETAIN, or removed), records the status
// for SYNCHRONIZE, deletes the log for /NOKEEP and the files for /DELETE,
// tells the user for /NOTIFY, and settles the jobs again: a queue with
// room starts its next.
//
// The processes the database's jobs run in are known only to this
// System (System.batch, by entry): a job left executing by a session that
// ended, or by an INITIALIZE/MEMORY, has none. A session's end ends its
// jobs (EndSessionJobs), the next session to run the manager recovers
// what a session that ended without doing so left (queues.go's
// recoverQueues), and a new System ends the old one's (EndOrphanedJobs):
// each requeued as a restart if /RESTART, else ended with JBC$_QUERESET.

// BatchJob is what a batch job's CLI runs (CLIStartup.Batch).
type BatchJob struct {
	// Entry is the job's entry number; Name and Queue its name and its
	// queue's.
	Entry       int
	Name, Queue string

	// Files are its command procedures, in the order they run, each named
	// as SHOW ENTRY/FULL names it ("_GOVAX$DUA1:[000000]BAT98OK.COM;1").
	Files []string

	// Parameters are P1 to P8, given to every file.
	Parameters []string

	// Restarted says this run is a restart ($RESTART "TRUE"), and
	// RestartValue is BATCH$RESTART's value then.
	Restarted    bool
	RestartValue string

	// First is the index in Files of the file the run begins with: 0, or
	// for a restart the file its last run stopped in (jobqueue's Job.File).
	First int

	// LGICMD is the account's login procedure, "" for SYS$LOGIN:LOGIN.COM;
	// Device and Directory its home, SYS$LOGIN.
	LGICMD            string
	Device, Directory string
}

// batchRun is one executing job's process.
type batchRun struct {
	entry int
	env   *Environment

	// forced is the status its process was ended with by the System (its
	// CPU limit, SS$_EXCPUTIM), 0 when it ended by itself or was ended
	// by an operation the job's Ending records.
	forced uint32

	// file is the index of the job's file its DCL runs (SetBatchFile).
	file int
}

// SetBatchFile records that env's batch job now runs its file with index
// i, as SHOW QUEUE/FULL shows it ("File: ... (executing)").
//
// The database keeps it too (jobqueue's SetJobFile), so that a restart,
// in this session or the next, begins with that file.
func (env *Environment) SetBatchFile(i int) {
	if env.batch == nil {
		return
	}

	if env.batch.file == i {
		return
	}

	env.batch.file = i

	// StartJob recorded the run's first file; only a move past it is
	// written.
	if m := env.System.queues; m.Running() {
		entry := env.batch.entry

		_ = jobqueue.Update(m.path, func(d *jobqueue.Database) error {
			return d.SetJobFile(entry, i)
		})
	}
}

// jobEnd is a job's last end: its final status, and endSeq's count when
// it ended, which tells SYNCHRONIZE whether the end it saw came after it
// began waiting; the job's name, queue, and user, by which SYNCHRONIZE
// names it too; and whether a SYNCHRONIZE has had it.
//
// VMS 7.1 gave the status of a job that ended in 0.03 seconds to the
// SYNCHRONIZE a few commands after its SUBMIT (BAT98.LOG's case 159), but
// NOSUCHENT to a SYNCHRONIZE of a job that had ended while the one before
// it waited for another (case 216), and to a second SYNCHRONIZE of a job
// (case 236): on the VAX a job is gone a moment after it ends, and case
// 159's SYNCHRONIZE came within that moment. govax runs a job between
// process 1's commands, so its jobs end sooner; to answer as the VAX did,
// an end is kept until a SYNCHRONIZE completes, which forgets every end
// then kept (ConsumeJobEnd; govax's model of the VAX's timing,
// unconfirmed).
type jobEnd struct {
	status            uint32
	seq               uint64
	name, queue, user string
	consumed          bool
}

// ssExcpuTim is SS$_EXCPUTIM, the status a batch job whose CPU limit runs
// out ends with.
var ssExcpuTim = vmsdef.Symbols["SS$_EXCPUTIM"]

// BatchJob reports whether env's process runs a batch job, and the job's
// entry number.
func (env *Environment) BatchJob() (entry int, ok bool) {
	if env.batch == nil {
		return 0, false
	}

	return env.batch.entry, true
}

// jobController is the process the job controller's work is done from:
// process 1, the console's, which every System has while it has a CPU.
func (sys *System) jobController() *Environment {
	if len(sys.procs.slots) > 1 && sys.procs.slots[1] != nil {
		return sys.procs.slots[1]
	}

	return nil
}

// updateJobs is jobqueue.Update with the job controller's part: change,
// then settleJobs, in one update of the database; what settleJobs leaves
// to do to processes (suspending, resuming, ending them) is done after
// the database is written, so that a process ended at once can finish its
// job in its own update.
func (sys *System) updateJobs(change func(*jobqueue.Database) error) error {
	var after []func()

	err := jobqueue.Update(sys.queues.path, func(d *jobqueue.Database) error {
		if err := change(d); err != nil {
			return err
		}

		after = sys.settleJobs(d)

		return nil
	})

	for _, f := range after {
		f()
	}

	return err
}

// settleJobs is the job controller's look at the database after a change
// (see this file's opening comment): jobs held until a time that has come
// are released; an executing job that is being ended or requeued has its
// process stopped, and one whose queue is paused its process suspended
// (resumed when the queue starts again); every queue then starts what it
// can. It returns what is to be done to processes once the database is
// written, and notes the manager's next /AFTER time.
func (sys *System) settleJobs(d *jobqueue.Database) []func() {
	now := goTime(sys.now())

	d.ReleaseDue(now)
	d.ExpireRetained(now)

	var after []func()

	for _, j := range d.Jobs {
		run := sys.batch[j.Entry]
		if j.Status != jobqueue.JobExecuting || run == nil {
			continue
		}

		env := run.env

		switch q := d.Queue(j.Queue); {
		case j.Ending != 0 || j.Requeue != nil:
			after = append(after, func() { sys.markForDeletion(env) })
		case q != nil && q.State == jobqueue.QueuePaused:
			after = append(after, func() { env.suspend() })
		default:
			after = append(after, env.resume)
		}
	}

	creator := sys.jobController()

	for _, q := range d.Queues {
		for j := d.NextJob(q.Name); j != nil && creator != nil; j = d.NextJob(q.Name) {
			if _, err := d.StartJob(j.Entry, now); err != nil {
				break
			}

			if st := creator.startBatchJob(q, j); st != 0 {
				d.Finish(j.Entry, st, now)
			}
		}
	}

	sys.queues.noteNext(d)

	return after
}

// startBatchJob starts j, from queue q, which the database has just made
// executing: its process created and logged in (see this file's opening
// comment). It returns 0, or the status the job ends with at once: the
// login's refusal, or $CREPRC's.
func (env *Environment) startBatchJob(q *jobqueue.Queue, j *jobqueue.Job) uint32 {
	sys := env.System

	l, err := sys.BeginLogin(j.User, false)
	if err != nil {
		if le, ok := err.(*LoginError); ok { //nolint:errorlint // BeginLogin's own
			return le.Status
		}

		return ssAbort
	}

	r := l.Record
	log := batchLog(j, r)

	// SYS$INPUT and SYS$COMMAND are the job's command stream, its first
	// file, a process-permanent file on the file's device (VMS 7.1's
	// "_SIMVAX$DUA1:"); its TT is NL: (prepareImage).
	input := "NLA0:"
	if len(j.Files) > 0 {
		input = strings.TrimPrefix(j.Files[0].Spec, "_"+sys.NodeName+"$")
	}

	child, st := env.CreateProcess(CreateRequest{
		Image: "LOGINOUT", Input: input, Output: log, Error: log,
		Name: fmt.Sprintf("BATCH_%d", j.Entry), UIC: uint32(r.UIC), Flags: prcBATCH,
		jobController: true,
	})
	if st != ssNormal {
		return st
	}

	child.LogIn(l)

	p := child.Process
	p.Name = fmt.Sprintf("BATCH_%d", j.Entry)
	p.CreateFlags = prcBATCH

	pri := uint32(q.BasePriority) & maxPriority
	p.BasePriority, p.Priority, p.AuthorizedPriority = pri, pri, pri

	if err := sys.sched.SetBasePriority(handle(child), int(pri)); err == nil {
		sys.requestReschedule()
	}

	p.CPULimit = batchCPULimit(j, q)

	files := make([]string, len(j.Files))
	for i, f := range j.Files {
		files[i] = f.Spec
	}

	child.Startup.CLI = &CLIStartup{Login: true, Batch: &BatchJob{
		Entry: j.Entry, Name: j.Name, Queue: j.Queue, Files: files, Parameters: j.Parameters,
		Restarted: j.Restarted, RestartValue: j.RestartValue, First: j.File,
		LGICMD: r.LGICMD, Device: r.Device, Directory: r.Directory,
	}}

	run := &batchRun{entry: j.Entry, env: child, file: j.File}
	child.batch = run
	child.batchLog = log

	if sys.batch == nil {
		sys.batch = map[int]*batchRun{}
	}

	sys.batch[j.Entry] = run
	j.PID = p.PID

	return 0
}

// batchLog is j's log file's name, as its process opens it: NL: for
// /NOLOG_FILE; otherwise /LOG_FILE's name, with the job's name when it
// names only a directory ("DUA1:[BAT98LOG].LOG;", as SUBMIT keeps one);
// by default the job's name with type .LOG in the account's home (its
// SYS$LOGIN, which isn't defined until its DCL starts).
func batchLog(j *jobqueue.Job, r uaf.Record) string {
	if j.NoLog {
		return "NLA0:"
	}

	spec := j.LogFile
	if spec == "" {
		device := strings.TrimSpace(r.Device)
		if device != "" && !strings.HasSuffix(device, ":") {
			device += ":"
		}

		return device + strings.TrimSpace(r.Directory) + j.Name + ".LOG"
	}

	i := strings.LastIndexAny(spec, "]>:") + 1
	if rest := spec[i:]; rest == "" || rest[0] == '.' || rest[0] == ';' {
		spec = spec[:i] + j.Name + rest
	}

	return spec
}

// batchCPULimit is a job's process's CPU limit, in 10-millisecond units
// (CPULM's), 0 for none: the job's /CPUTIME, else its queue's
// /CPUDEFAULT, no more than the queue's /CPUMAXIMUM; INFINITE is none.
func batchCPULimit(j *jobqueue.Job, q *jobqueue.Queue) uint32 {
	limit := j.CPUTime
	if limit == 0 {
		limit = q.CPUDefault
	}

	if m := q.CPUMaximum; m > 0 && (limit == jobqueue.CPUInfinite || limit == 0 || limit > m) {
		limit = m
	}

	if limit <= 0 {
		return 0
	}

	return uint32(limit)
}

// checkCPULimit ends the current process if it runs a batch job and has
// used its CPU limit, with SS$_EXCPUTIM, as VMS ended BAT98R8. Other
// processes' limits are recorded but not enforced (docs/DEVIATIONS.md).
func (sys *System) checkCPULimit() {
	env := sys.Current()
	if env == nil || env.batch == nil || env.Deleted || env.deletePending || env.batch.forced != 0 {
		return
	}

	limit := uint64(env.Process.CPULimit)
	if limit == 0 || env.cpuTime/100000 < limit {
		return
	}

	env.batch.forced = ssExcpuTim
	env.Process.ExitStatus = ssExcpuTim
	sys.markForDeletion(env)
}

// completeBatchJob is the job controller's part of a batch job's
// process's deletion (see this file's opening comment).
func (sys *System) completeBatchJob(env *Environment) {
	run := env.batch
	if run == nil {
		return
	}

	env.batch = nil
	delete(sys.batch, run.entry)

	status := env.Process.ExitStatus
	if run.forced != 0 {
		status = run.forced
	}

	if sys.queues.Path() == "" {
		return
	}

	var (
		ended jobqueue.Job
		found bool
		final uint32
	)

	now := goTime(sys.now())

	_ = sys.updateJobs(func(d *jobqueue.Database) error {
		j := d.Job(run.entry)
		if j == nil || j.Status != jobqueue.JobExecuting {
			return nil
		}

		ended, found = *j, true

		final = status
		if j.Ending != 0 {
			final = j.Ending
		}

		requeued := j.Requeue != nil

		d.Finish(run.entry, status, now)

		if requeued {
			found = false // its run ends, not the job
		}

		return nil
	})

	if !found {
		return
	}

	sys.recordJobEnd(&ended, final)
	sys.afterJob(env, &ended, final)
}

// recordJobEnd notes that the job j has ended with status, for
// SYNCHRONIZE.
func (sys *System) recordJobEnd(j *jobqueue.Job, status uint32) {
	if sys.jobEnds == nil {
		sys.jobEnds = map[int]jobEnd{}
	}

	sys.endSeq++
	sys.jobEnds[j.Entry] = jobEnd{status: status, seq: sys.endSeq, name: j.Name, queue: j.Queue, user: j.User}
}

// forgetJobEnd forgets the end of an earlier job of entry number entry, a
// new job having taken the number.
func (sys *System) forgetJobEnd(entry int) { delete(sys.jobEnds, entry) }

// ConsumeJobEnd is a SYNCHRONIZE's completion: the ends kept so far are
// forgotten (see jobEnd), so a job that has ended and gone is NOSUCHENT to
// a later SYNCHRONIZE. entry is the job it had.
func (sys *System) ConsumeJobEnd(int) {
	for n, e := range sys.jobEnds {
		e.consumed = true
		sys.jobEnds[n] = e
	}
}

// afterJob is what a job's end does beyond the database: /NOKEEP deletes
// its log, /DELETE its files (each as the job's process could, by its
// session), and /NOTIFY tells the user at the console's terminal, in VMS
// 7.1's words (A.4):
//
//	Job BAT98R7 (queue BAT98_A, entry 52) completed
//	Job BAT98X (queue BAT98_A, entry 53) terminated with error status
//	%SYSTEM-F-ABORT, abort
func (sys *System) afterJob(env *Environment, j *jobqueue.Job, status uint32) {
	if s := env.Session; s != nil {
		if j.NoKeep && !j.NoLog && env.batchLog != "" {
			_, _ = s.Delete(strings.TrimSuffix(env.batchLog, ";") + ";")
		}

		if j.Delete {
			for _, f := range j.Files {
				_, _ = s.Delete(strings.TrimPrefix(f.Spec, "_"+sys.NodeName+"$"))
			}
		}
	}

	sys.notifyJob(j, status)
}

// notifyJob tells j's user, at the console's terminal, that j has ended
// with status, if it was submitted /NOTIFY.
func (sys *System) notifyJob(j *jobqueue.Job, status uint32) {
	var out io.Writer
	if c := sys.jobController(); c != nil {
		out = c.consoleOut
	}

	if !j.Notify || out == nil {
		return
	}

	head := fmt.Sprintf("Job %s (queue %s, entry %d) ", j.Name, j.Queue, j.Entry)

	if status&1 != 0 {
		fmt.Fprintf(out, "\r\n%scompleted\r\n", head)

		return
	}

	fmt.Fprintf(out, "\r\n%sterminated with error status\r\n%%%s\r\n", head, vmsdef.FormatMessage(status))
}

// JobEnded reports, for SYNCHRONIZE, whether the job entry has ended
// since since (a value JobEndMark returned), and its final status.
func (sys *System) JobEnded(entry int, since uint64) (status uint32, ended bool) {
	e, ok := sys.jobEnds[entry]
	if !ok || e.seq <= since {
		return 0, false
	}

	return e.status, true
}

// JobEndMark is the count JobEnded compares with: the ends recorded so
// far.
func (sys *System) JobEndMark() uint64 { return sys.endSeq }

// SyncTarget is the job SYNCHRONIZE waits for: the job entry (or, with
// entry 0, the process's own job called name, in queue, SYS$BATCH by
// default), which the process must be able to read. A job that isn't
// there is JBC$_NOSUCHENT by entry and JBC$_NOSUCHJOB by name, as VMS
// 7.1 answered (cases 232 and 233); a job the process may not read is
// JBC$_NOPRIV. It returns the job's entry, and, for a job already retained,
// the status it ended with (done true): SYNCHRONIZE waits for nothing.
func (env *Environment) SyncTarget(entry int, name, queue string) (found int, status uint32, done bool, err error) {
	d, err := env.readJobs()
	if err != nil {
		return 0, 0, false, err
	}

	var j *jobqueue.Job

	if queue == "" {
		queue = "SYS$BATCH"
	}

	if entry != 0 {
		j = d.Job(entry)
	} else {
		for _, c := range d.QueueJobs(queueName(queue)) {
			if strings.EqualFold(c.Name, strings.TrimSpace(name)) && strings.EqualFold(c.User, env.Process.Username) {
				j = c

				break
			}
		}
	}

	if j == nil {
		// A job ended and gone, not yet had by a SYNCHRONIZE.
		for n, e := range env.System.jobEnds {
			if e.consumed || entry != 0 && n != entry ||
				entry == 0 && (!strings.EqualFold(e.name, strings.TrimSpace(name)) ||
					!strings.EqualFold(e.queue, queueName(queue)) || !strings.EqualFold(e.user, env.Process.Username)) {
				continue
			}

			return n, e.status, true, nil
		}

		if entry != 0 {
			return 0, 0, false, queueStatus(jbcNoSuchEnt)
		}

		return 0, 0, false, queueStatus(jbcNoSuchJob)
	}

	if !env.JobAccess(d.Queue(j.Queue), j, security.QueueRead) {
		return 0, 0, false, queueStatus(jbcNoPriv)
	}

	if j.Status == jobqueue.JobRetained {
		return j.Entry, j.Condition, true, nil
	}

	return j.Entry, 0, false, nil
}

// AbortJob is STOP/QUEUE/ENTRY (ABORT_JOB): the executing job entry, in
// queue, ended with JBC$_JOBABORT, its process stopped; a job not
// executing is JBC$_JOBNOTEXEC before any protection check, as VMS 7.1
// answered (subtask 5's finding), and one in another queue JBC$_NOSUCHJOB.
// The process needs delete access to the job.
func (env *Environment) AbortJob(entry int, queue string) error {
	now := env.queueNow()

	if st, err := env.managerHere(); err != nil || st != 0 {
		return queueResult(st, err)
	}

	return env.System.updateJobs(func(d *jobqueue.Database) error {
		j := d.Job(entry)
		if j == nil || queue != "" && d.Queue(queueName(queue)) != d.Queue(j.Queue) {
			return queueStatus(jbcNoSuchJob)
		}

		if j.Status != jobqueue.JobExecuting {
			return queueStatus(vmsdef.Symbols["JBC$_JOBNOTEXEC"])
		}

		if _, err := env.deletableJob(d, entry, queue); err != nil {
			return err
		}

		_, err := d.AbortJob(entry, "", now)

		return err
	})
}

// RequeueJob is STOP/QUEUE/REQUEUE[=to]/ENTRY=entry queue[/HOLD]
// (the DCL Dictionary's STOP/QUEUE/REQUEUE): the executing job's process
// stopped, and the job requeued when it has gone, to the queue to names
// ("" for its own), held if hold says, its next run a restart ($RESTART
// "TRUE", BATCH$RESTART its last SET RESTART_VALUE), whether or not it
// was submitted /RESTART, as VMS 7.1 requeued BAT98X2. Its checks are
// AbortJob's, with JBC$_NOSUCHQUE for a to that isn't there; the process
// needs delete access to the job.
func (env *Environment) RequeueJob(entry int, queue, to string, hold bool) error {
	now := env.queueNow()

	if st, err := env.managerHere(); err != nil || st != 0 {
		return queueResult(st, err)
	}

	return env.System.updateJobs(func(d *jobqueue.Database) error {
		j := d.Job(entry)
		if j == nil || queue != "" && d.Queue(queueName(queue)) != d.Queue(j.Queue) {
			return queueStatus(jbcNoSuchJob)
		}

		if j.Status != jobqueue.JobExecuting {
			return queueStatus(vmsdef.Symbols["JBC$_JOBNOTEXEC"])
		}

		if _, err := env.deletableJob(d, entry, queue); err != nil {
			return err
		}

		r := jobqueue.Requeue{Hold: hold}
		if to != "" {
			r.Queue = queueName(to)
		}

		_, err := d.RequeueJob(entry, "", r, now)

		return err
	})
}

// SetRestartValue is SET RESTART_VALUE=value in a batch job: the value
// the job's next run, if it is a restart, finds in BATCH$RESTART, kept in
// the database (and shown by SHOW ENTRY/FULL as /RESTART_VALUE). The
// running job's own BATCH$RESTART isn't changed: VMS 7.1's SHOW SYMBOL
// BATCH$RESTART after it was UNDSYM (BAT98X1's log). Outside a batch job
// it does nothing (the DCL Dictionary: "meaningful only in command
// procedures"; unconfirmed that VMS says nothing).
func (env *Environment) SetRestartValue(value string) error {
	if env.batch == nil {
		return nil
	}

	if st, err := env.managerHere(); err != nil || st != 0 {
		return queueResult(st, err)
	}

	entry := env.batch.entry

	return jobqueue.Update(env.System.queues.path, func(d *jobqueue.Database) error {
		return d.SetRestartValue(entry, value)
	})
}

// ExecutingJobs is how many batch jobs this session runs now: those whose
// processes its System has.
func (sys *System) ExecutingJobs() int { return len(sys.batch) }

// EndSessionJobs is what a session's end does with the batch jobs it runs
// (Decision 11): each is ended as VMS's shutdown ends a job (the probe's
// reboot), requeued and checkpointed if /RESTART, else ended with
// JBC$_QUERESET and retained as /RETAIN says; nothing new is started, and
// the session lets the manager's lock go, so another session may run it.
// Their processes go with the session. It returns how many were requeued
// and how many ended.
func (sys *System) EndSessionJobs() (requeued, ended int, err error) {
	m := sys.queues
	if !m.Running() || len(sys.batch) == 0 {
		return 0, 0, nil
	}

	now := goTime(sys.now())

	err = jobqueue.Update(m.path, func(d *jobqueue.Database) error {
		r, e := d.EndRuns(now, func(entry int) bool { return sys.batch[entry] == nil })
		requeued, ended = len(r), len(e)

		return nil
	})

	for entry, run := range sys.batch {
		run.env.batch = nil
		delete(sys.batch, entry)
	}

	m.Close()

	return requeued, ended, err
}

// noteNext keeps the database's next /AFTER time for the scheduler
// (nextTimer, pollQueueTimer), 0 when no job waits for a time.
func (m *QueueManager) noteNext(d *jobqueue.Database) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.next = 0

	if t, ok := d.NextTime(); ok {
		m.next = vmsTime(t)
	}
}

// nextJobTime is the manager's next /AFTER time, in VMS time, and whether
// there is one, while the manager runs here.
func (m *QueueManager) nextJobTime() (uint64, bool) {
	if m == nil {
		return 0, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.next, m.lock != nil && m.next != 0
}

// pollQueueTimer is the scheduler's look at the manager (pollEvents): when
// a job's /AFTER time has come, the jobs are settled, and the jobs it
// released start where their queues have room.
func (sys *System) pollQueueTimer() {
	next, ok := sys.queues.nextJobTime()
	if !ok || sys.now() < next {
		return
	}

	_ = sys.updateJobs(func(*jobqueue.Database) error { return nil })
}

// PollJobs is the job controller's look at the time when the scheduler
// isn't running (process 1's DCL waiting with no other process to lend
// the CPU to): a job held until a time that has come is released.
func (sys *System) PollJobs() { sys.pollQueueTimer() }

// syncWait is a $SNDJBC SYNCHRONIZE_JOB not yet complete: the job it
// waits for, the end count when it began, and where its completion goes.
type syncWait struct {
	env                  *Environment
	entry                int
	since                uint64
	efn, iosb            uint32
	astadr, astprm, mode uint32
	done                 bool
}

// pollSyncWaits completes each SYNCHRONIZE_JOB whose job has ended: its
// IOSB gets the job's status, its event flag is set, and its AST queued.
func (sys *System) pollSyncWaits() {
	if len(sys.syncWaits) == 0 {
		return
	}

	kept := sys.syncWaits[:0]

	for _, w := range sys.syncWaits {
		status, ended := sys.JobEnded(w.entry, w.since)
		if w.env.Deleted {
			continue
		}

		if !ended {
			kept = append(kept, w)

			continue
		}

		w.done = true
		sys.ConsumeJobEnd(w.entry)
		w.env.completeSync(w, status)
	}

	sys.syncWaits = kept
}

// completeSync is a SYNCHRONIZE_JOB's completion in its process.
func (env *Environment) completeSync(w *syncWait, status uint32) {
	if w.iosb != 0 {
		var buf [8]byte
		buf[0], buf[1], buf[2], buf[3] = byte(status), byte(status>>8), byte(status>>16), byte(status>>24)
		_ = env.storeOwn(w.iosb, buf[:])
	}

	env.postFlag(w.efn, sched.ClassIOCompletion)

	if w.astadr != 0 {
		env.queueAST(w.astadr, w.astprm, w.mode)
	}
}
