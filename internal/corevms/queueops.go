package corevms

import (
	"strings"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/security"
)

// The queue operations (docs/planning/PHASE-98 - batch queues.md,
// subtask 6, Decisions 5 and 8): creating, starting, stopping, changing,
// and deleting a batch queue, and reading the queues for SHOW QUEUE. DCL's
// queue commands and $SNDJBC's queue functions both call these, so the
// checks are made in one place and in one order, VMS 7.1's as the probe
// showed it (testdata/bat98): the queue manager running here
// (managerHere), the queue's name, the queue's existence, the asker's
// access (OPER to create one, manage access to change one; NOOPER and
// NOPRIV), and last the queue's state (internal/jobqueue's queueRules:
// STARTED, QUENOTMOD, QUENOTSTOP). Each change is made to the database
// under its lock and saved at once (jobqueue.Update): a refusal saves
// nothing.

// QueueSet changes a queue's attributes: the qualifiers of
// INITIALIZE/QUEUE, START/QUEUE, and SET QUEUE, or $SNDJBC's items. nil
// changes nothing. A value out of range is refused (JBC$_INVPARVAL) and
// changes nothing.
type QueueSet func(*jobqueue.Queue)

// QueueStop says how STOP/QUEUE stops a queue.
type QueueStop int

// The ways STOP/QUEUE stops a queue.
const (
	// StopPause is STOP/QUEUE ($SNDJBC's PAUSE_QUEUE): the queue pauses,
	// its executing jobs suspended.
	StopPause QueueStop = iota
	// StopNext is STOP/QUEUE/NEXT (STOP_QUEUE): no more jobs started, the
	// queue stopped when its executing jobs end.
	StopNext
	// StopReset is STOP/QUEUE/RESET (RESET_QUEUE): stopped at once, its
	// executing jobs ended with JBC$_QUERESET.
	StopReset
)

// changeQueuesHere is changeQueues for an operation that needs the queue
// manager running in this session (managerHere).
func (env *Environment) changeQueuesHere(change func(*jobqueue.Database) error) error {
	if st, err := env.managerHere(); err != nil || st != 0 {
		return queueResult(st, err)
	}

	return env.System.updateJobs(change)
}

// managedQueue is the queue called name in d, if the process may manage
// it: JBC$_NOSUCHQUE or JBC$_NOPRIV otherwise.
func (env *Environment) managedQueue(d *jobqueue.Database, name string) (*jobqueue.Queue, error) {
	q := d.Queue(queueName(name))
	if q == nil {
		return nil, queueStatus(jbcNoSuchQue)
	}

	if !env.QueueAccess(q, security.QueueManage) {
		return nil, queueStatus(jbcNoPriv)
	}

	return q, nil
}

// CreateQueue is INITIALIZE/QUEUE (and $SNDJBC's CREATE_QUEUE): a queue
// named name made with set's attributes, stopped, or with start started
// at once (/START, CREATE_START). Making one needs OPER (JBC$_NOOPER),
// and only batch queues are made (batch false is JBC$_NOTSUPPORTED:
// Decision 1; VMS makes a printer queue). A queue of that name that
// exists is changed by set instead, which needs manage access to it, and
// only while it is stopped (JBC$_QUENOTMOD, changing nothing, as VMS 7.1
// answered for a started queue); batch is then not asked
// (unconfirmed).
func (env *Environment) CreateQueue(name string, batch, start bool, set QueueSet) error {
	if st, err := env.managerHere(); err != nil || st != 0 {
		return queueResult(st, err)
	}

	name = queueName(name)
	if err := jobqueue.ValidQueueName(name); err != nil {
		return err
	}

	return jobqueue.Update(env.System.queues.path, func(d *jobqueue.Database) error {
		if q := d.Queue(name); q != nil {
			if !env.QueueAccess(q, security.QueueManage) {
				return queueStatus(jbcNoPriv)
			}
		} else {
			if st := env.MayCreateQueue(); st != 0 {
				return queueStatus(st)
			}

			if !batch {
				return queueStatus(jbcNotSupported)
			}
		}

		q, _, err := d.CreateQueue(name, set)
		if err != nil {
			return err
		}

		if start {
			return d.StartQueue(q.Name, env.NodeName)
		}

		return nil
	})
}

// StartQueue is START/QUEUE (START_QUEUE): the queue started on this
// node, with set's attributes first (START/QUEUE takes INITIALIZE's), or
// a paused or stopping one continued. A started queue is JBC$_STARTED,
// changing nothing.
func (env *Environment) StartQueue(name string, set QueueSet) error {
	return env.changeQueuesHere(func(d *jobqueue.Database) error {
		q, err := env.managedQueue(d, name)
		if err != nil {
			return err
		}

		if err := d.StartQueue(q.Name, env.NodeName); err != nil {
			return err
		}

		return d.SetQueue(q.Name, set)
	})
}

// StopQueue is STOP/QUEUE (PAUSE_QUEUE, STOP_QUEUE, RESET_QUEUE), by how.
// A stopped queue stays stopped, silently, as VMS 7.1's did.
//
// The executing jobs a pause suspends and a reset ends are the batch
// jobs' processes, which subtask 8 starts; here the database records the
// queue's state, and a reset marks the jobs it ends (jobqueue's
// ResetQueue).
func (env *Environment) StopQueue(name string, how QueueStop) error {
	return env.changeQueuesHere(func(d *jobqueue.Database) error {
		q, err := env.managedQueue(d, name)
		if err != nil {
			return err
		}

		switch how {
		case StopNext:
			return d.StopQueueNext(q.Name)
		case StopReset:
			_, err := d.ResetQueue(q.Name)

			return err
		}

		return d.PauseQueue(q.Name)
	})
}

// SetQueue is SET QUEUE (ALTER_QUEUE): the queue's attributes changed by
// set, in any state.
func (env *Environment) SetQueue(name string, set QueueSet) error {
	return env.changeQueuesHere(func(d *jobqueue.Database) error {
		q, err := env.managedQueue(d, name)
		if err != nil {
			return err
		}

		return d.SetQueue(q.Name, set)
	})
}

// DeleteQueue is DELETE/QUEUE (DELETE_QUEUE): a stopped queue removed,
// with its jobs (JBC$_QUENOTSTOP if it isn't stopped).
func (env *Environment) DeleteQueue(name string) error {
	return env.changeQueuesHere(func(d *jobqueue.Database) error {
		q, err := env.managedQueue(d, name)
		if err != nil {
			return err
		}

		return d.DeleteQueue(q.Name)
	})
}

// QueueView is one queue as SHOW QUEUE shows it to the process: a copy of
// the queue, its state as SHOW QUEUE words it, and whether the process may
// see its owner, protection, and ACL (QueueSecurityVisible).
type QueueView struct {
	Queue     jobqueue.Queue
	State     string
	Security  bool
	JobsCount int

	// Jobs are the queue's jobs as SHOW QUEUE lists them.
	Jobs []JobView
}

// QueueViews is SHOW QUEUE's reading of the database: the queues whose
// names match pattern (VMS's wildcards, "*" and "%"; "" or "*" is every
// queue), in order of name. A pattern matching none is JBC$_NOSUCHQUE
// (unconfirmed for a wildcard: the probe's every wildcard matched). Any
// session may read the queues, the manager running here or not
// (Decision 4), but not while the database says the manager is stopped
// (JBC$_JOBQUEDIS, as VMS answers with its manager stopped).
func (env *Environment) QueueViews(pattern string) ([]QueueView, error) {
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

	pattern = strings.ToUpper(queueName(pattern))
	if pattern == "" {
		pattern = "*"
	}

	var views []QueueView

	for _, q := range d.Queues {
		if !lnm.Match(pattern, strings.ToUpper(q.Name)) {
			continue
		}

		v := QueueView{
			Queue:     *q,
			State:     d.StateText(q),
			Security:  env.QueueSecurityVisible(q),
			JobsCount: len(d.QueueJobs(q.Name)),
			Jobs:      env.queueJobViews(d, q),
		}
		v.Queue.ACL = append([]jobqueue.ACE(nil), q.ACL...)

		views = append(views, v)
	}

	if len(views) == 0 {
		return nil, queueStatus(jbcNoSuchQue)
	}

	return views, nil
}
