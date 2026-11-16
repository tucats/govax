package corevms

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// The job operations (docs/planning/PHASE-98 - batch queues.md, subtask
// 7): entering a job in a queue (SUBMIT, $SNDJBC's ENTER_FILE and
// CREATE_JOB), changing it (SET ENTRY, ALTER_JOB), deleting it
// (DELETE/ENTRY, DELETE_JOB), and reading the jobs (SHOW ENTRY, SHOW
// QUEUE). As queueops.go's, each is made in one place for DCL and the
// service alike, its checks in VMS 7.1's order as the probe showed them
// (testdata/bat98): the queue manager running here, the job's user, its
// files, the queue, the asker's access, and last the job's state
// (internal/jobqueue's jobRules). Running a job is subtask 8's.

// maxQueuePriority is MAXQUEPRI, the highest /PRIORITY a job may have
// without OPER or ALTPRI: VMS's default, 100 (unconfirmed: the probe
// never read the VAX's SYSGEN; a /PRIORITY=200 was lowered silently but
// for PRIOSMALL, and 255 was kept for SYSTEM).
const maxQueuePriority = 100

// The job operations' statuses.
var (
	jbcNoSuchJob = vmsdef.Symbols["JBC$_NOSUCHJOB"]
	jbcNoSuchEnt = vmsdef.Symbols["JBC$_NOSUCHENT"]
	jbcNoDstQue  = vmsdef.Symbols["JBC$_NODSTQUE"]
	jbcPrioSmall = vmsdef.Symbols["JBC$_PRIOSMALL"]
	rmsPrivilege = vmsdef.Symbols["RMS$_PRV"]
	sysprvBit    = privilegeBit("SYSPRV")
)

// JobRequest is a job to enter: SUBMIT's parameters and qualifiers, or
// $SNDJBC's items.
type JobRequest struct {
	// Queue is the queue's name; Files the command procedures' specs, in
	// the order they run.
	Queue string
	Files []string

	// Name is /NAME, "" for the first file's name; User /USER, "" for the
	// process's own.
	Name, User string

	// Hold is /HOLD, After /AFTER (zero for none).
	Hold  bool
	After time.Time

	// Parameters are P1 to P8.
	Parameters []string

	// LogFile is /LOG_FILE's spec as given ("" for the default), NoLog
	// /NOLOG_FILE; Keep, NoKeep, and NoPrinter /KEEP, /NOKEEP, and
	// /NOPRINTER; Printer is /PRINTER's queue, given when PrinterGiven.
	LogFile                 string
	NoLog                   bool
	Keep, NoKeep, NoPrinter bool
	Printer                 string
	PrinterGiven            bool

	// Priority is /PRIORITY, -1 for the default.
	Priority int

	// CPUTime and the working-set values are the job's limits, 0 for
	// none.
	CPUTime                      jobqueue.CPUTime
	WSDefault, WSQuota, WSExtent int

	// Restart, Retain (and RetainUntil), Notify, Note, and Delete are the
	// rest of the job's options.
	Restart     bool
	Retain      jobqueue.Retain
	RetainUntil time.Time
	Notify      bool
	Note        string
	Delete      bool
}

// Submitted is what entering a job did: the job as entered, its queue's
// state, and whether its priority was lowered to MAXQUEPRI.
type Submitted struct {
	Job             jobqueue.Job
	QueueState      jobqueue.QueueState
	PriorityLowered bool
}

// busy reports whether the job waits on a started queue: one at its job
// limit (VMS 7.1's BAT98L4, case 212, "pending status caused by queue busy
// state").
func (s Submitted) busy() bool {
	return s.QueueState == jobqueue.QueueStarted && s.Job.Status == jobqueue.JobPending
}

// Message is SUBMIT's report of the job, as VMS 7.1 wrote it (BAT98.LOG's
// cases 69 to 107): its name, queue, and entry, and what it is waiting
// for, a second line saying why a pending job waits on a stopped queue.
//
//	Job BAT98OK (queue BAT98_B, entry 18) pending
//	     pending status caused by queue stopped state
//	Job HELD98 (queue BAT98_B, entry 19) holding
//	Job BAT98OK (queue BAT98_B, entry 20) holding until 16-NOV-2026 11:54
func (s Submitted) Message() []string {
	j := &s.Job
	head := fmt.Sprintf("Job %s (queue %s, entry %d) ", j.Name, j.Queue, j.Entry)

	switch j.Status {
	case jobqueue.JobHolding:
		return []string{head + "holding"}
	case jobqueue.JobTimed:
		return []string{head + "holding until " + QueueTime(j.After, true)}
	case jobqueue.JobExecuting:
		return []string{head + "started on " + j.Queue}
	}

	if s.QueueState == jobqueue.QueueStopped {
		return []string{head + "pending", "     pending status caused by queue stopped state"}
	}

	if s.busy() {
		return []string{head + "pending", "     pending status caused by queue busy state"}
	}

	return []string{head + "pending"}
}

// QueueTime is t as the queue commands write a time: "16-NOV-2026
// 11:54:42.48", or with short the minutes alone, "16-NOV-2026 11:54", as
// SUBMIT and /RETAIN=UNTIL write it.
func QueueTime(t time.Time, short bool) string {
	if short {
		return strings.ToUpper(t.Format("2-Jan-2006 15:04"))
	}

	return strings.ToUpper(t.Format("2-Jan-2006 15:04:05.00"))
}

// LocalTime is the VMS time v as the queue database keeps a time (a
// moment in the host's zone, as goTime makes it): DCL's SUBMIT/AFTER
// reads a time with $BINTIM's rules and hands it on so.
func LocalTime(v uint64) time.Time { return goTime(v) }

// JobUserError is SUBMIT/USER naming an account it can't use: one that
// isn't there (Status 0), or an authorization file the process may not
// read (RMS$_PRV). DCL's SUBMIT reports it as its INVQUAVAL.
type JobUserError struct {
	User   string
	Status uint32
}

func (e *JobUserError) Error() string { return "no account " + e.User }

// jobAccount finds the account a job runs as: the process's own when
// user is "" (or, once the file is read, its own name), otherwise the account called user, whose
// UIC owns the job. Reading the authorization file needs read access to
// it with the process's authorized privileges (an account authorized for
// SYSPRV but without it enabled could, one not authorized couldn't: the
// matrix's case 12; which privilege VMS's SUBMIT turns on is
// unconfirmed). Another user's job needs CMKRNL (ruleSubmitUser), asked
// after the account is found.
func (env *Environment) jobAccount(user string) (name string, owner jobqueue.UIC, creds security.Credentials, err error) {
	p := env.Process
	user = strings.ToUpper(strings.TrimSpace(user))

	if user == "" {
		return strings.ToUpper(p.Username), jobqueue.UIC(p.UIC), env.Credentials(), nil
	}

	// /USER naming the process's own account still reads the file: VMS
	// 7.1 refused BAT98W's SUBMIT/USER=BAT98W with RMS$_PRV (the matrix's
	// case 12 as BAT98W).
	reader := env.Credentials()
	reader.Privileges |= p.AuthorizedPrivileges & sysprvBit

	if !security.Check(reader, authorizationProfile, security.Read).Granted {
		return "", 0, creds, &JobUserError{User: user, Status: rmsPrivilege}
	}

	u, ok := env.System.User(user)
	if !ok {
		return "", 0, creds, &JobUserError{User: user}
	}

	if u.Name == strings.ToUpper(p.Username) {
		return u.Name, jobqueue.UIC(p.UIC), env.Credentials(), nil
	}

	if st := env.requirePrivilege(ruleSubmitUser); st != 0 {
		return "", 0, creds, queueStatus(st)
	}

	return u.Name, jobqueue.UIC(u.UIC), u.AccountCredentials(env.Process.MaxSysGroup), nil
}

// authorizationProfile is the authorization file's security profile,
// SYSUAF.DAT's on VMS 7.1: [SYSTEM], (S:RWE,O:RWE,G,W).
var authorizationProfile = security.Profile{
	Owner:      security.SystemUIC,
	Protection: security.NoAccess.With(security.System, security.Read|security.Write|security.Execute).With(security.Owner, security.Read|security.Write|security.Execute),
	Name:       "SYSUAF.DAT;1",
}

// jobFiles finds the files a job runs (rms.Session.JobFiles), each named
// with the node as a job's file is ("_GOVAX$DUA1:[000000]BAT98OK.COM;1").
// A file that can't be found or read is the *rms.DCLFileError RMS gave.
func (env *Environment) jobFiles(specs []string) ([]jobqueue.File, error) {
	if env.Session == nil {
		return nil, &rms.DCLFileError{Name: strings.ToUpper(strings.Join(specs, ",")), Status: vmsdef.Symbols["RMS$_FNF"]}
	}

	var files []jobqueue.File

	for _, spec := range specs {
		found, err := env.Session.JobFiles(spec)
		if err != nil {
			return nil, err
		}

		for _, f := range found {
			files = append(files, jobqueue.File{
				Spec: "_" + env.NodeName + "$" + strings.TrimPrefix(f.Name, "_"),
				ID:   jobqueue.FileID{Number: uint32(f.ID.Num) | uint32(f.ID.Nmx)<<16, Sequence: f.ID.Seq, RVN: uint16(f.ID.Rvn)},
			})
		}
	}

	return files, nil
}

// fileName is a job's default name: its first file's name, without its
// device, directory, type, and version.
func fileName(spec string) string {
	if i := strings.LastIndexAny(spec, "]>:"); i >= 0 {
		spec = spec[i+1:]
	}

	if i := strings.IndexAny(spec, ".;"); i >= 0 {
		spec = spec[:i]
	}

	return spec
}

// SubmitJob enters a job (SUBMIT, ENTER_FILE, CLOSE_JOB): r's files in
// r's queue, as r's user. Its refusals, in the order VMS 7.1 made them:
// the manager not running here; /USER's account (*JobUserError, then
// JBC$_NOCMKRNL); a file (*rms.DCLFileError); /PRINTER (JBC$_NODSTQUE:
// govax has no output queues, Decision 7); the queue (JBC$_NOSUCHQUE);
// submit access to it, the job's user's (JBC$_NOPRIV: SUBMIT/USER's
// access was the named user's, the matrix's states E and F). A /PRIORITY
// above MAXQUEPRI without OPER or ALTPRI is lowered to it, and the job
// entered (Submitted.PriorityLowered, JBC$_PRIOSMALL).
func (env *Environment) SubmitJob(r JobRequest) (Submitted, error) {
	if st, err := env.managerHere(); err != nil || st != 0 {
		return Submitted{}, queueResult(st, err)
	}

	user, owner, creds, err := env.jobAccount(r.User)
	if err != nil {
		return Submitted{}, err
	}

	if len(r.Files) == 0 {
		return Submitted{}, queueStatus(jbcMisReqPar)
	}

	files, err := env.jobFiles(r.Files)
	if err != nil {
		return Submitted{}, err
	}

	if r.PrinterGiven {
		return Submitted{}, queueStatus(jbcNoDstQue)
	}

	j := &jobqueue.Job{
		Name: r.Name, Queue: queueName(r.Queue), User: user, Owner: owner,
		Files: files, Parameters: slices.Clone(r.Parameters),
		LogFile: r.LogFile, NoLog: r.NoLog, Keep: r.Keep, NoKeep: r.NoKeep, NoPrinter: r.NoPrinter,
		After: r.After, Priority: r.Priority, CPUTime: r.CPUTime,
		WSDefault: r.WSDefault, WSQuota: r.WSQuota, WSExtent: r.WSExtent,
		Restart: r.Restart, Retain: r.Retain, RetainUntil: r.RetainUntil,
		Notify: r.Notify, Note: r.Note, Delete: r.Delete,
	}

	if j.Name == "" {
		j.Name = fileName(files[0].Spec)
	}

	if j.Priority < 0 {
		j.Priority = jobqueue.DefaultJobPri
	}

	var s Submitted

	if j.Priority > maxQueuePriority && !env.holdsPrivilege(ruleQueuePriority) {
		j.Priority, s.PriorityLowered = maxQueuePriority, true
	}

	now := env.queueNow()

	err = env.System.updateJobs(func(d *jobqueue.Database) error {
		q := d.Queue(j.Queue)
		if q == nil {
			return queueStatus(jbcNoSuchQue)
		}

		if !env.isOperator() && !security.CheckQueue(creds, q.Profile(), security.QueueSubmit).Granted {
			return queueStatus(jbcNoPriv)
		}

		if err := d.Submit(j, r.Hold, now); err != nil {
			return err
		}

		s.QueueState = q.State

		return nil
	})
	if err != nil {
		return Submitted{}, err
	}

	s.Job = *j
	env.System.forgetJobEnd(j.Entry)

	return s, nil
}

// JobChange is SET ENTRY's (ALTER_JOB's) change to a job: hold it,
// release it, move it to another queue (/REQUEUE), give it another
// /AFTER time (After set; a zero time is /NOAFTER), or change its other
// fields (Set); Priority is a new /PRIORITY, -1 for none.
type JobChange struct {
	Hold, Release bool
	Requeue       string
	After         *time.Time
	Priority      int
	Set           func(*jobqueue.Job)
}

// AlterJob is SET ENTRY entry (ALTER_JOB): c applied to the job, in queue
// when queue isn't "" (a job elsewhere is JBC$_NOSUCHJOB, as one that
// isn't there). The process needs delete access to the job (one's own
// through the queue's O field, another's by an ACE or the queue's manage
// access: JBC$_NOPRIV); a job executing is JBC$_EXECUTING. A /PRIORITY
// above MAXQUEPRI is lowered as SubmitJob lowers it (lowered).
func (env *Environment) AlterJob(entry int, queue string, c JobChange) (lowered bool, err error) {
	if c.Priority > maxQueuePriority && !env.holdsPrivilege(ruleQueuePriority) {
		c.Priority, lowered = maxQueuePriority, true
	}

	now := env.queueNow()

	err = env.changeQueuesHere(func(d *jobqueue.Database) error {
		j, err := env.deletableJob(d, entry, queue)
		if err != nil {
			return err
		}

		if c.Requeue != "" {
			if err := d.MoveJob(j.Entry, queueName(c.Requeue), now); err != nil {
				return err
			}
		}

		if c.Set != nil || c.After != nil || c.Priority >= 0 {
			err := d.AlterJob(j.Entry, now, func(a *jobqueue.Job) {
				if c.Set != nil {
					c.Set(a)
				}

				if c.After != nil {
					a.After = *c.After
				}

				if c.Priority >= 0 {
					a.Priority = c.Priority
				}
			})
			if err != nil {
				return err
			}
		}

		switch {
		case c.Hold:
			return d.HoldJob(j.Entry, now)
		case c.Release:
			return d.ReleaseJob(j.Entry, now)
		}

		// A change and nothing else is still refused for an executing
		// job, as VMS 7.1's SET ENTRY was (A.4).
		if j.Status == jobqueue.JobExecuting {
			return queueStatus(vmsdef.Symbols["JBC$_EXECUTING"])
		}

		return nil
	})

	return lowered, err
}

// deletableJob is the job entry in d (in queue, when queue isn't ""), if
// the process may change or delete it: JBC$_NOSUCHJOB or JBC$_NOPRIV
// otherwise.
func (env *Environment) deletableJob(d *jobqueue.Database, entry int, queue string) (*jobqueue.Job, error) {
	j := d.Job(entry)
	if j == nil || queue != "" && d.Queue(queueName(queue)) != d.Queue(j.Queue) {
		return nil, queueStatus(jbcNoSuchJob)
	}

	if !env.JobAccess(d.Queue(j.Queue), j, security.QueueDelete) {
		return nil, queueStatus(jbcNoPriv)
	}

	return j, nil
}

// DeleteJob is DELETE/ENTRY entry (DELETE_JOB): the job removed, in queue
// when queue isn't "" (a job elsewhere is JBC$_NOSUCHJOB), with delete
// access to it (JBC$_NOPRIV). An executing job is marked to end with
// JBC$_JOBABORT and go (subtask 8 stops its process).
//
// A /NOTIFY job deleted before it ran tells its user so, as VMS 7.1 told
// BAT98.LOG's case 99's (terminal.log): "terminated with error status",
// JBC$_JOBDELETE.
func (env *Environment) DeleteJob(entry int, queue string) error {
	now := env.queueNow()

	var gone *jobqueue.Job

	err := env.changeQueuesHere(func(d *jobqueue.Database) error {
		j, err := env.deletableJob(d, entry, queue)
		if err != nil {
			return err
		}

		was := *j

		running, err := d.DeleteJob(j.Entry, "", now)
		if err == nil && running == nil {
			gone = &was
		}

		return err
	})

	if err == nil && gone != nil && gone.Notify {
		env.System.notifyJob(gone, vmsdef.Symbols["JBC$_JOBDELETE"])
	}

	return err
}

// JobView is one job as SHOW QUEUE and SHOW ENTRY show it to the process:
// a copy of the job, whether the process may see it (read access, else
// "no privilege"), and its queue's state as "On stopped batch queue"
// words it.
//
// For an executing job, Running is the index of the file its process
// runs (SHOW QUEUE/FULL's "(executing)"), -1 when no process here runs it;
// Paused says its queue is paused, so it is suspended; Busy that its
// queue is started and at its job limit (a pending job's "(queue busy)").
type JobView struct {
	Job        jobqueue.Job
	Visible    bool
	Own        bool
	QueueState string
	Stopped    bool
	Running    int
	Paused     bool
	Busy       bool
}

// jobView is j as the process may see it.
func (env *Environment) jobView(d *jobqueue.Database, j *jobqueue.Job) JobView {
	q := d.Queue(j.Queue)

	v := JobView{
		Job:        *j,
		Visible:    env.JobAccess(q, j, security.QueueRead),
		Own:        strings.EqualFold(j.User, env.Process.Username),
		QueueState: d.StateText(q),
		Stopped:    q.State == jobqueue.QueueStopped,
		Running:    -1,
		Paused:     q.State == jobqueue.QueuePaused,
		Busy:       q.State == jobqueue.QueueStarted && d.Executing(q.Name) >= q.JobLimit,
	}

	if run := env.System.batch[j.Entry]; run != nil && j.Status == jobqueue.JobExecuting {
		v.Running = run.file
	}
	v.Job.Files = slices.Clone(j.Files)
	v.Job.Parameters = slices.Clone(j.Parameters)

	return v
}

// readJobs is the database for SHOW ENTRY: JBC$_JOBQUEDIS when there is
// none or its manager is stopped, as QueueViews.
func (env *Environment) readJobs() (*jobqueue.Database, error) {
	m := env.System.queues
	if m.Path() == "" {
		return nil, queueStatus(jbcJobQueDis)
	}

	d, err := jobqueue.Read(m.path)
	if err != nil {
		return nil, err
	}

	if d.Manager.State != jobqueue.ManagerStarted {
		return nil, queueStatus(jbcJobQueDis)
	}

	return d, nil
}

// EntryViews is SHOW ENTRY's reading of the jobs, in order of entry
// number: those entries names, or with none, user's jobs (the process's
// own when user is ""), in every queue. Entries none of which is there
// are JBC$_NOSUCHENT (VMS 7.1's SHOW ENTRY 99999); a user with no jobs
// shows nothing (unconfirmed).
func (env *Environment) EntryViews(entries []int, user string) ([]JobView, error) {
	d, err := env.readJobs()
	if err != nil {
		return nil, err
	}

	if user == "" {
		user = env.Process.Username
	}

	var views []JobView

	for _, j := range d.Jobs {
		if len(entries) > 0 && !slices.Contains(entries, j.Entry) ||
			len(entries) == 0 && !strings.EqualFold(j.User, strings.TrimSpace(user)) {
			continue
		}

		views = append(views, env.jobView(d, j))
	}

	if len(views) == 0 && len(entries) > 0 {
		return nil, queueStatus(jbcNoSuchEnt)
	}

	return views, nil
}

// queueJobViews are the jobs in q as SHOW QUEUE lists them
// (jobqueue.Database.Order).
func (env *Environment) queueJobViews(d *jobqueue.Database, q *jobqueue.Queue) []JobView {
	var views []JobView

	for _, j := range d.Order(q.Name) {
		views = append(views, env.jobView(d, j))
	}

	return views
}
