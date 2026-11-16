package jobqueue

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tucats/govax/internal/security"
)

// Version is the file's format: Parse refuses a file of another, so a
// later govax that changes the format can say so rather than misread it.
const Version = 1

// MaxEntry is the highest entry number the manager gives a job, VMS's
// default (the phase doc's A.1): after it, numbering starts again at 1,
// passing over numbers still in use.
const MaxEntry = 999

// The limits a queue's and a job's fields have (the phase doc's A.4).
const (
	MaxQueueName  = 31  // a longer name is JBC$_INVPARLEN
	MaxJobLimit   = 255 // /JOB_LIMIT is 1 to 255
	MaxPriority   = 31  // /BASE_PRIORITY is 0 to 31
	MaxJobPrio    = 255 // a job's /PRIORITY is 0 to 255
	MaxParameters = 8   // P1 to P8
	MaxParameter  = 255 // the longest parameter
	MaxJobName    = 39  // /NAME's longest
	DefaultJobPri = 100 // a job's /PRIORITY when none is given
)

// Database is the whole queue database: the manager's state, every queue,
// and every job, with the counters that number entries and order
// submissions. Its methods are the rules (queue.go, job.go); the caller
// saves it after changing it (Update does both under the file's lock).
type Database struct {
	// Version is the file's format, Version.
	Version int `json:"version"`

	// Manager is the queue manager's state.
	Manager Manager `json:"manager"`

	// NextEntry is the entry number the next job is given, unless in use.
	NextEntry int `json:"next_entry"`

	// NextSequence orders submissions: each job takes the next, so jobs of
	// equal priority run in the order they were submitted even after the
	// entry numbers wrap.
	NextSequence int64 `json:"next_sequence"`

	// Queues are the queues, in order of name.
	Queues []*Queue `json:"queues"`

	// Jobs are the jobs, in order of entry number.
	Jobs []*Job `json:"jobs"`
}

// Manager is the queue manager's state (SHOW QUEUE/MANAGER).
type Manager struct {
	// State is whether the manager was last started or stopped.
	State ManagerState `json:"state"`

	// Directory is START/QUEUE/MANAGER's directory, kept and shown but
	// not used: the database is this file (Decision 3).
	Directory string `json:"directory,omitempty"`

	// Started is when the manager was last started.
	Started time.Time `json:"started,omitzero"`
}

// Queue is one batch queue.
type Queue struct {
	// Name is the queue's name, 1 to 31 letters, digits, "$", and "_".
	Name string `json:"name"`

	// State is the queue's state.
	State QueueState `json:"state"`

	// Node is the node the queue was last started on; until it is first
	// started it has none, and SHOW QUEUE names no node.
	Node string `json:"node,omitempty"`

	// Description is /DESCRIPTION's text.
	Description string `json:"description,omitempty"`

	// JobLimit is how many jobs the queue runs at once, 1 to 255.
	JobLimit int `json:"job_limit"`

	// BasePriority is the base priority (and authorized priority) its
	// jobs' processes get, 0 to 31.
	BasePriority int `json:"base_priority"`

	// Owner is the queue's owner UIC.
	Owner UIC `json:"owner"`

	// Protection is the queue's protection code.
	Protection Protection `json:"protection"`

	// ACL is the queue's access control list.
	ACL []ACE `json:"acl,omitempty"`

	// WSDefault, WSQuota, and WSExtent are the queue's working-set
	// values in pages, 0 when not given.
	WSDefault int `json:"wsdefault,omitempty"`
	WSQuota   int `json:"wsquota,omitempty"`
	WSExtent  int `json:"wsextent,omitempty"`

	// CPUDefault is the CPU limit its jobs get when they give none, and
	// CPUMaximum the most a job may ask for; 0 when not given.
	CPUDefault CPUTime `json:"cpudefault,omitzero"`
	CPUMaximum CPUTime `json:"cpumaximum,omitzero"`

	// Retain is which ended jobs the queue keeps.
	Retain Retain `json:"retain,omitzero"`
}

// NewQueue returns a batch queue named name with VMS's defaults: a job
// limit of 1, base priority 4, owner [SYSTEM], (S:M,O:D,G:R,W:S),
// stopped.
func NewQueue(name string) *Queue {
	return &Queue{
		Name:         name,
		State:        QueueStopped,
		JobLimit:     1,
		BasePriority: 4,
		Owner:        SystemUIC,
		Protection:   DefaultProtection,
	}
}

// Profile is the queue's protection as internal/security's rule takes
// it: its owner, code, and ACL.
func (q *Queue) Profile() security.Profile {
	acl := make([]security.ACE, 0, len(q.ACL))
	for _, a := range q.ACL {
		acl = append(acl, security.DecodeACE(a))
	}

	return security.Profile{
		Owner:      security.UIC(q.Owner),
		Protection: security.Protection(q.Protection),
		ACL:        acl,
		Name:       q.Name,
	}
}

// Started reports whether the queue is started in VMS's sense: running,
// paused, or stopping, anything but stopped.
func (q *Queue) Started() bool { return q.State != QueueStopped }

// File is one of a job's files.
type File struct {
	// Spec is the file's full name, "_GOVAX$DUA1:[000000]BAT98OK.COM;1".
	Spec string `json:"spec"`

	// ID is the file's file ID, by which the job holds it.
	ID FileID `json:"id"`
}

// Job is one batch job.
type Job struct {
	// Entry is the job's entry number, unique among the manager's jobs.
	Entry int `json:"entry"`

	// Sequence orders it among jobs submitted (Database.NextSequence).
	Sequence int64 `json:"sequence"`

	// Name is the job's name: /NAME, or its first file's name.
	Name string `json:"name"`

	// Queue is the name of the queue it is in.
	Queue string `json:"queue"`

	// User is the username it runs as, and Owner that user's UIC: the
	// owner its queue's protection is checked against for it.
	User  string `json:"user"`
	Owner UIC    `json:"owner"`

	// Status is the job's state.
	Status JobStatus `json:"status"`

	// Files are its command procedures, run in order.
	Files []File `json:"files"`

	// Parameters are P1 to P8, given to every file.
	Parameters []string `json:"parameters,omitempty"`

	// LogFile is /LOG_FILE's name, "" for the default
	// (SYS$LOGIN:jobname.LOG); NoLog is /NOLOG_FILE.
	LogFile string `json:"log_file,omitempty"`
	NoLog   bool   `json:"no_log,omitempty"`

	// Keep is /KEEP, NoKeep /NOKEEP (the log deleted after the job);
	// NoPrinter is /NOPRINTER. With no print queues (Decision 1) the log
	// is kept unless /NOKEEP.
	Keep      bool `json:"keep,omitempty"`
	NoKeep    bool `json:"no_keep,omitempty"`
	NoPrinter bool `json:"no_printer,omitempty"`

	// After is /AFTER's time, zero when none.
	After time.Time `json:"after,omitzero"`

	// Priority is the job's place in its queue, 0 to 255 (not its
	// process's priority).
	Priority int `json:"priority"`

	// CPUTime is /CPUTIME, 0 when not given.
	CPUTime CPUTime `json:"cputime,omitzero"`

	// WSDefault, WSQuota, and WSExtent are the job's working-set values,
	// 0 when not given.
	WSDefault int `json:"wsdefault,omitempty"`
	WSQuota   int `json:"wsquota,omitempty"`
	WSExtent  int `json:"wsextent,omitempty"`

	// Restart is /RESTART: requeued, not ended, when the session running
	// it ends.
	Restart bool `json:"restart,omitempty"`

	// Retain is the job's /RETAIN, RetainDefault for its queue's.
	Retain Retain `json:"retain,omitzero"`

	// Notify is /NOTIFY, Note /NOTE's text, Delete /DELETE (the files
	// deleted after the job).
	Notify bool   `json:"notify,omitempty"`
	Note   string `json:"note,omitempty"`
	Delete bool   `json:"delete,omitempty"`

	// Submitted, Started, and Completed are when the job was submitted,
	// last started, and ended.
	Submitted time.Time `json:"submitted"`
	Started   time.Time `json:"started,omitzero"`
	Completed time.Time `json:"completed,omitzero"`

	// CompletedQueue is the queue it ended on.
	CompletedQueue string `json:"completed_queue,omitempty"`

	// Condition is how it ended: its last file's status, or the reason
	// it was ended (JBC$_JOBABORT, JBC$_QUERESET), a condition value.
	Condition uint32 `json:"condition,omitempty"`

	// Ending is the condition an executing job is being ended with, by
	// DELETE/ENTRY, STOP/QUEUE/ENTRY, or STOP/QUEUE/RESET: Finish gives
	// it as the job's status whatever the process's own was.
	Ending uint32 `json:"ending,omitempty"`

	// Deleting says DELETE/ENTRY is ending the executing job: it is
	// removed when it ends, whatever /RETAIN says.
	Deleting bool `json:"deleting,omitempty"`

	// Requeue, while the job executes, asks that it be requeued when its
	// run ends (STOP/QUEUE/REQUEUE).
	Requeue *Requeue `json:"requeue,omitempty"`

	// RestartValue is the last SET RESTART_VALUE, BATCH$RESTART in a
	// restarted run.
	RestartValue string `json:"restart_value,omitempty"`

	// Restarted says the job's next run is a restart ($RESTART "TRUE"),
	// and Checkpointed that it has a restart value to go on from.
	Restarted    bool `json:"restarted,omitempty"`
	Checkpointed bool `json:"checkpointed,omitempty"`

	// PID is the executing job's process, known only to the session
	// running it; it isn't saved.
	PID uint32 `json:"-"`
}

// Requeue is a STOP/QUEUE/REQUEUE waiting for a job's run to end.
type Requeue struct {
	// Queue is the queue it goes to, "" for its own.
	Queue string `json:"queue,omitempty"`

	// Hold says it is to be held there.
	Hold bool `json:"hold,omitempty"`
}

// New returns an empty database: no queues, no jobs, the manager
// stopped, numbering from 1.
func New() *Database {
	return &Database{
		Version:      Version,
		NextEntry:    1,
		NextSequence: 1,
		Queues:       []*Queue{},
		Jobs:         []*Job{},
	}
}

// Queue returns the queue named name (any case), or nil.
func (d *Database) Queue(name string) *Queue {
	for _, q := range d.Queues {
		if strings.EqualFold(q.Name, name) {
			return q
		}
	}

	return nil
}

// Job returns the job with entry number entry, or nil.
func (d *Database) Job(entry int) *Job {
	for _, j := range d.Jobs {
		if j.Entry == entry {
			return j
		}
	}

	return nil
}

// QueueJobs returns the jobs in the queue named name, in order of entry
// number.
func (d *Database) QueueJobs(name string) []*Job {
	var jobs []*Job

	for _, j := range d.Jobs {
		if strings.EqualFold(j.Queue, name) {
			jobs = append(jobs, j)
		}
	}

	return jobs
}

// Executing returns how many of the queue's jobs are executing.
func (d *Database) Executing(name string) int {
	n := 0

	for _, j := range d.QueueJobs(name) {
		if j.Status == JobExecuting {
			n++
		}
	}

	return n
}

// StateText is the queue's state as SHOW QUEUE words it (the phase doc's
// A.4): "stopped", "idle" (started, nothing running), "available" (jobs
// running, room for more), "busy" (at its job limit), "paused", or
// "available, stop pending" while STOP/QUEUE/NEXT waits for its jobs.
func (d *Database) StateText(q *Queue) string {
	switch q.State {
	case QueueStopped:
		return "stopped"
	case QueuePaused:
		return "paused"
	case QueueStopping:
		// Unconfirmed: the probe saw a queue of job limit 3 with one job
		// running, "available, stop pending"; a queue at its limit is
		// taken to be "busy, stop pending".
		if d.Executing(q.Name) >= q.JobLimit {
			return "busy, stop pending"
		}

		return "available, stop pending"
	}

	switch n := d.Executing(q.Name); {
	case n == 0:
		return "idle"
	case n >= q.JobLimit:
		return "busy"
	default:
		return "available"
	}
}

// sortQueues keeps the queues in order of name, as SHOW QUEUE lists
// them.
func (d *Database) sortQueues() {
	slices.SortFunc(d.Queues, func(a, b *Queue) int { return strings.Compare(a.Name, b.Name) })
}

// sortJobs keeps the jobs in order of entry number.
func (d *Database) sortJobs() {
	slices.SortFunc(d.Jobs, func(a, b *Job) int { return a.Entry - b.Entry })
}

// ValidQueueName checks a queue's name: JBC$_INVPARLEN when empty or
// longer than 31 characters, JBC$_INVQUENAM when it has a character other
// than a letter, digit, "$", or "_" (VMS 7.1's answers, the phase doc's
// A.4). Lowercase letters are allowed: a name is compared ignoring case.
func ValidQueueName(name string) error {
	if name == "" || len(name) > MaxQueueName {
		return refuse("JBC$_INVPARLEN")
	}

	for _, ch := range name {
		ok := ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' ||
			ch == '$' || ch == '_'
		if !ok {
			return refuse("JBC$_INVQUENAM")
		}
	}

	return nil
}

// Check verifies the database: its version; each queue's name and fields
// in range, no two named alike; each job's entry in range and unique, its
// queue existing, its fields in range; the counters in range. Parse and
// Save call it, so a file govax writes is always one it can read.
func (d *Database) Check() error {
	if d.Version != Version {
		return fmt.Errorf("the file's format is version %d; this govax reads version %d", d.Version, Version)
	}

	if d.NextEntry < 1 || d.NextEntry > MaxEntry {
		return fmt.Errorf("next_entry %d is not from 1 to %d", d.NextEntry, MaxEntry)
	}

	if d.NextSequence < 1 {
		return fmt.Errorf("next_sequence %d is less than 1", d.NextSequence)
	}

	seen := map[string]bool{}

	for _, q := range d.Queues {
		if err := q.check(); err != nil {
			return fmt.Errorf("queue %q: %w", q.Name, err)
		}

		key := strings.ToUpper(q.Name)
		if seen[key] {
			return fmt.Errorf("queue %q is in the file twice", q.Name)
		}

		seen[key] = true
	}

	entries := map[int]bool{}

	for _, j := range d.Jobs {
		if err := j.check(); err != nil {
			return fmt.Errorf("job %d: %w", j.Entry, err)
		}

		if entries[j.Entry] {
			return fmt.Errorf("job %d is in the file twice", j.Entry)
		}

		entries[j.Entry] = true

		if d.Queue(j.Queue) == nil {
			return fmt.Errorf("job %d is in queue %q, which isn't in the file", j.Entry, j.Queue)
		}

		if j.Sequence >= d.NextSequence {
			return fmt.Errorf("job %d's sequence %d isn't below next_sequence %d", j.Entry, j.Sequence, d.NextSequence)
		}
	}

	return nil
}

// check verifies one queue's fields.
func (q *Queue) check() error {
	if err := ValidQueueName(q.Name); err != nil {
		return fmt.Errorf("not a queue name (%v)", err)
	}

	switch {
	case q.JobLimit < 1 || q.JobLimit > MaxJobLimit:
		return fmt.Errorf("job_limit %d is not from 1 to %d", q.JobLimit, MaxJobLimit)
	case q.BasePriority < 0 || q.BasePriority > MaxPriority:
		return fmt.Errorf("base_priority %d is not from 0 to %d", q.BasePriority, MaxPriority)
	case q.WSDefault < 0 || q.WSQuota < 0 || q.WSExtent < 0:
		return fmt.Errorf("a working-set value is negative")
	case q.CPUDefault < CPUInfinite || q.CPUMaximum < CPUInfinite:
		return fmt.Errorf("a CPU time is negative")
	}

	for _, a := range q.ACL {
		if err := checkACE(a); err != nil {
			return err
		}
	}

	return nil
}

// check verifies one job's fields.
func (j *Job) check() error {
	switch {
	case j.Entry < 1 || j.Entry > MaxEntry:
		return fmt.Errorf("entry %d is not from 1 to %d", j.Entry, MaxEntry)
	case j.Sequence < 1:
		return fmt.Errorf("sequence %d is less than 1", j.Sequence)
	case j.Name == "" || len(j.Name) > MaxJobName:
		return fmt.Errorf("name %q is not 1 to %d characters", j.Name, MaxJobName)
	case j.User == "":
		return fmt.Errorf("no user")
	case len(j.Files) == 0:
		return fmt.Errorf("no files")
	case len(j.Parameters) > MaxParameters:
		return fmt.Errorf("%d parameters, more than %d", len(j.Parameters), MaxParameters)
	case j.Priority < 0 || j.Priority > MaxJobPrio:
		return fmt.Errorf("priority %d is not from 0 to %d", j.Priority, MaxJobPrio)
	case j.CPUTime < CPUInfinite:
		return fmt.Errorf("a negative CPU time")
	case j.WSDefault < 0 || j.WSQuota < 0 || j.WSExtent < 0:
		return fmt.Errorf("a working-set value is negative")
	case j.Status < JobPending || j.Status > JobRetained:
		return fmt.Errorf("status %d unknown", j.Status)
	case j.Requeue != nil && j.Status != JobExecuting:
		return fmt.Errorf("a requeue for a job not executing")
	case (j.Ending != 0 || j.Deleting) && j.Status != JobExecuting:
		return fmt.Errorf("an ending for a job not executing")
	}

	for _, p := range j.Parameters {
		if len(p) > MaxParameter {
			return fmt.Errorf("a parameter longer than %d characters", MaxParameter)
		}
	}

	return nil
}
