package corevms

import (
	"errors"
	"sync"
	"time"

	"github.com/tucats/govax/internal/hostlock"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/vmsdef"
)

// The queue manager (docs/planning/PHASE-98 - batch queues.md, Decisions
// 2 to 5). On VMS the queue manager is a detached process, QUEUE_MANAGER,
// that keeps the queue database; govax keeps the database in a host file
// of internal/jobqueue's (cmd/govax finds and checks it before the machine
// starts), and the manager is Go code here, with no process of its own.
//
// A QueueManager is the session's: the machine makes one for its queue
// file and gives it to each System it makes, so that an INITIALIZE/MEMORY,
// which makes a new System, keeps it running. The manager runs in this
// session while the session holds the manager's lock beside the file
// (jobqueue.LockManager, Decision 4): only then may a queue or job be
// changed here. Another session holding it is the batch initiator, and
// this one may only show queues.
//
// What the manager does with jobs (starting them, ending them) is
// batch.go's; here it keeps the queues, starts in a session
// (ResumeQueueManager, startHere), and recovers what the session that last
// ran it left (recoverQueues). A session's end, with jobs still executing,
// is EndSessionJobs's (batch.go).

// QueueManager is the session's queue manager: the database's host file,
// and the manager's lock while the manager runs in this session.
type QueueManager struct {
	mu   sync.Mutex
	path string
	lock *hostlock.Lock

	// next is the database's next /AFTER time, in VMS time, 0 for none,
	// as the job controller last saw it (batch.go's noteNext).
	next uint64
}

// NewQueueManager returns the manager of the queue database at path, a
// host file ("" for none: every operation but a SHOW is then
// JBC$_JOBQUEDIS). The manager isn't running until Start, or until the
// first operation that needs it finds the database saying it was started
// (lockIfStarted).
func NewQueueManager(path string) *QueueManager { return &QueueManager{path: path} }

// Path is the database's host file, "" for none.
func (m *QueueManager) Path() string {
	if m == nil {
		return ""
	}

	return m.path
}

// Running reports whether the manager runs in this session.
func (m *QueueManager) Running() bool {
	if m == nil {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.lock != nil
}

// Close lets the manager's lock go, as the session ends; the database
// still says the manager was started, so the next session runs it
// (Decision 11).
func (m *QueueManager) Close() {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_ = m.lock.Close()
	m.lock = nil
}

// takeLock takes the manager's lock if this session doesn't hold it:
// took says it was taken now; status is 0, or JBC$_QMANNOTSTARTED when
// another session holds it.
func (m *QueueManager) takeLock() (took bool, status uint32, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.lock != nil {
		return false, 0, nil
	}

	lock, err := jobqueue.LockManager(m.path)

	switch {
	case errors.Is(err, hostlock.ErrLocked):
		return false, jbcQmanNotStarted, nil
	case err != nil:
		return false, 0, err
	}

	m.lock = lock

	return true, 0, nil
}

// startHere takes the manager's lock for this session and, when it is
// taken now, recovers what the session that last ran the manager left
// (recoverQueues). It returns 0 or JBC$_QMANNOTSTARTED.
func (sys *System) startHere() (uint32, error) {
	took, st, err := sys.queues.takeLock()
	if err != nil || st != 0 || !took {
		return st, err
	}

	return 0, sys.recoverQueues()
}

// recoverQueues is the manager's start in a session (Decision 11, held to
// the probe's reboot): every queue stopped, and every job the database
// says is executing, but that has no process in this System, ended as a
// system's stop ends it (jobqueue's Recover): requeued if /RESTART (or
// STOP/QUEUE/REQUEUE had asked), else ended with JBC$_QUERESET. VMS's
// manager says nothing of it at boot, and neither does govax's.
func (sys *System) recoverQueues() error {
	now := goTime(sys.now())

	return sys.updateJobs(func(d *jobqueue.Database) error {
		d.Recover(now, func(entry int) bool { return sys.batch[entry] != nil })

		return nil
	})
}

// EndOrphanedJobs is a new System's look at a manager that runs here
// already (the machine's INITIALIZE/MEMORY makes a new System and keeps
// the session's manager): the old System's batch processes are gone, so
// each job that was executing is ended as at a session's end
// (jobqueue's EndRuns); the queues keep their states, and start what
// they can once the new System has its first process.
func (sys *System) EndOrphanedJobs() error {
	if !sys.queues.Running() {
		return nil
	}

	now := goTime(sys.now())

	return sys.updateJobs(func(d *jobqueue.Database) error {
		d.EndRuns(now, func(entry int) bool { return sys.batch[entry] != nil })

		return nil
	})
}

// ResumeQueueManager is the manager's start as the session starts
// (Decision 11, after STARTUP.COM): when the database says the manager was
// started, and no other session runs it, it runs here from now on, and
// what the last session left is recovered; otherwise nothing is done
// (with another session running it, this one may only show queues).
func (env *Environment) ResumeQueueManager() error {
	_, err := env.managerHere()

	return err
}

// The queue manager's statuses.
var (
	jbcNormal         = vmsdef.Symbols["JBC$_NORMAL"]
	jbcJobQueDis      = vmsdef.Symbols["JBC$_JOBQUEDIS"]
	jbcQmanNotStarted = vmsdef.Symbols["JBC$_QMANNOTSTARTED"]
	jbcNoPriv         = vmsdef.Symbols["JBC$_NOPRIV"]
	jbcNotSupported   = vmsdef.Symbols["JBC$_NOTSUPPORTED"]
	jbcMisReqPar      = vmsdef.Symbols["JBC$_MISREQPAR"]
	jbcInvParLen      = vmsdef.Symbols["JBC$_INVPARLEN"]
	jbcInvParVal      = vmsdef.Symbols["JBC$_INVPARVAL"]
)

// SetQueueManager gives the System the session's queue manager.
func (s *System) SetQueueManager(m *QueueManager) { s.queues = m }

// QueueManager is the System's queue manager, nil when it has none.
func (s *System) QueueManager() *QueueManager { return s.queues }

// SetQueueFile gives the System a queue manager of its own for path, a
// host file; "" is none. The tests use it; the machine gives each System
// the session's manager instead (SetQueueManager).
func (s *System) SetQueueFile(path string) {
	if s.queues != nil && s.queues.Path() == path {
		return
	}

	s.queues.Close()
	s.queues = NewQueueManager(path)
}

// QueueFile is the queue database's host path, "" when there is none.
func (s *System) QueueFile() string { return s.queues.Path() }

// queueNow is the System's time as the database keeps times.
func (env *Environment) queueNow() time.Time { return goTime(env.System.now()) }

// managerHere is the check every operation that changes the database
// makes first: 0 when the manager runs in this session, or the refusal.
// With no database, or the database saying the manager was stopped, it
// is JBC$_JOBQUEDIS, as VMS refuses a request while its queue manager is
// stopped; with another session running it, JBC$_QMANNOTSTARTED
// (Decision 4). A database that says the manager was started, in a
// session not yet running it, starts it here (startHere): VMS starts its
// manager at boot, and so does govax's session (ResumeQueueManager), but
// the session that held it may have ended since.
func (env *Environment) managerHere() (uint32, error) {
	m := env.System.queues
	if m.Path() == "" {
		return jbcJobQueDis, nil
	}

	if m.Running() {
		return 0, nil
	}

	d, err := jobqueue.Read(m.path)
	if err != nil {
		return 0, err
	}

	if d.Manager.State != jobqueue.ManagerStarted {
		return jbcJobQueDis, nil
	}

	return env.System.startHere()
}

// QueueManagerInfo is what SHOW QUEUE/MANAGER shows.
type QueueManagerInfo struct {
	// Running says the manager is started: running in this session or
	// another. Here says it runs in this one.
	Running, Here bool

	// Directory is START/QUEUE/MANAGER's directory, kept and shown.
	Directory string

	// File is the database's host file.
	File string

	// Node is the node the manager runs on.
	Node string
}

// QueueManagerStatus is SHOW QUEUE/MANAGER's: the manager's state as the
// database has it. Anyone may ask.
func (env *Environment) QueueManagerStatus() (QueueManagerInfo, error) {
	m := env.System.queues

	info := QueueManagerInfo{File: m.Path(), Node: env.NodeName, Here: m.Running()}
	if info.File == "" {
		return info, nil
	}

	d, err := jobqueue.Read(info.File)
	if err != nil {
		return info, err
	}

	info.Running = d.Manager.State == jobqueue.ManagerStarted
	info.Directory = d.Manager.Directory

	return info, nil
}

// StartQueueManager is START/QUEUE/MANAGER (and $SNDJBC's
// START_QUEUE_MANAGER): the manager started in this session, the database
// made if it doesn't exist, directory recorded (Decision 3: the database
// stays the host file). It needs OPER and SYSNAM (ruleQueueManager). A
// manager already running here is started again, silently (unconfirmed:
// the probe never started a running manager; VMS's boot starts it, and a
// site's startup that says START/QUEUE/MANAGER again must not fail).
// With no database it is JBC$_JOBQUEDIS; with another session running
// the manager, JBC$_QMANNOTSTARTED.
func (env *Environment) StartQueueManager(directory string) error {
	if st := env.requirePrivilege(ruleQueueManager); st != 0 {
		return queueStatus(st)
	}

	m := env.System.queues
	if m.Path() == "" {
		return queueStatus(jbcJobQueDis)
	}

	if st, err := env.System.startHere(); err != nil || st != 0 {
		return queueResult(st, err)
	}

	now := env.queueNow()

	return jobqueue.Update(m.path, func(d *jobqueue.Database) error {
		d.StartManager(directory, now)

		return nil
	})
}

// StopQueueManager is STOP/QUEUE/MANAGER (and $SNDJBC's
// STOP_QUEUE_MANAGER): the database says the manager stopped, so no
// session runs it until START/QUEUE/MANAGER, and this session lets its
// lock go. It needs OPER and SYSNAM. A manager not running here is
// JBC$_JOBQUEDIS, or JBC$_QMANNOTSTARTED when another session runs it
// (unconfirmed: the probe never stopped its VAX's manager).
func (env *Environment) StopQueueManager() error {
	if st := env.requirePrivilege(ruleQueueManager); st != 0 {
		return queueStatus(st)
	}

	if st, err := env.managerHere(); err != nil || st != 0 {
		return queueResult(st, err)
	}

	m := env.System.queues

	err := jobqueue.Update(m.path, func(d *jobqueue.Database) error {
		d.StopManager()

		return nil
	})

	m.Close()

	return err
}

// queueResult is a refusal's status or err as one error: nil for neither.
func queueResult(status uint32, err error) error {
	if err != nil {
		return err
	}

	if status != 0 {
		return queueStatus(status)
	}

	return nil
}
