package jobqueue

import (
	"fmt"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// Refusal is an operation the rules refuse: the JBC condition VMS's job
// controller returns for it, by name ("JBC$_STARTED"), and for a
// malformed request a Detail saying what was wrong.
type Refusal struct {
	Name   string
	Detail string
}

// refuse makes a Refusal of the JBC condition name.
func refuse(name string) *Refusal { return &Refusal{Name: name} }

func (r *Refusal) Error() string {
	if r.Detail != "" {
		return fmt.Sprintf("jobqueue: %s (%s)", r.Name, r.Detail)
	}

	return "jobqueue: " + r.Name
}

// Code is the condition's value.
func (r *Refusal) Code() uint32 { return vmsdef.Symbols[r.Name] }

// QueueOp is an operation on a queue whose outcome depends on the queue's
// state: a row of queueRules.
type QueueOp int

// The queue operations.
const (
	// OpStartQueue is START/QUEUE.
	OpStartQueue QueueOp = iota
	// OpPauseQueue is STOP/QUEUE without /NEXT or /RESET: the queue
	// pauses, its executing jobs suspended.
	OpPauseQueue
	// OpStopQueueNext is STOP/QUEUE/NEXT: the queue starts no more jobs,
	// and stops when its executing jobs end.
	OpStopQueueNext
	// OpResetQueue is STOP/QUEUE/RESET: the queue stops at once, its
	// executing jobs ended with JBC$_QUERESET.
	OpResetQueue
	// OpDeleteQueue is DELETE/QUEUE.
	OpDeleteQueue
	// OpModifyQueue is INITIALIZE/QUEUE (CREATE_QUEUE) of a queue that
	// exists: its attributes changed.
	OpModifyQueue
	numQueueOps
)

// queueCell is one cell of queueRules: what an operation does to a queue
// in one state. refuse, when set, is the JBC condition refusing it;
// otherwise the queue goes to next, or stays as it is when keep is set
// (the operation succeeds, changing nothing).
type queueCell struct {
	refuse string
	next   QueueState
	keep   bool
	set    bool // made by a constructor below: no cell is left empty
}

// Shorthands for queueRules' cells.
func goQueue(s QueueState) queueCell    { return queueCell{next: s, set: true} }
func refuseQueue(n string) queueCell    { return queueCell{refuse: n, set: true} }
func keepQueue() queueCell              { return queueCell{keep: true, set: true} }
func queueRow(c queueCell) [4]queueCell { return [4]queueCell{c, c, c, c} }

// queueRules is what each queue operation does in each state: a row per
// operation, a column per state (stopped, started, paused, stopping).
// The cells VMS 7.1 showed (the phase doc's A.4): START of a started
// queue STARTED; STOP of a stopped queue succeeds silently; STOP/QUEUE
// pauses a started queue; DELETE of a started queue QUENOTSTOP;
// INITIALIZE of a started queue QUENOTMOD, changing nothing. The rest are
// govax's choices, unconfirmed: START of a paused queue continues it, and
// of a stopping one takes back the stop; /NEXT of a paused queue leaves
// it stopping.
var queueRules = [numQueueOps][4]queueCell{
	OpStartQueue: {
		QueueStopped:  goQueue(QueueStarted),
		QueueStarted:  refuseQueue("JBC$_STARTED"),
		QueuePaused:   goQueue(QueueStarted),
		QueueStopping: goQueue(QueueStarted),
	},
	OpPauseQueue: {
		QueueStopped:  keepQueue(),
		QueueStarted:  goQueue(QueuePaused),
		QueuePaused:   keepQueue(),
		QueueStopping: goQueue(QueuePaused),
	},
	OpStopQueueNext: {
		QueueStopped:  keepQueue(),
		QueueStarted:  goQueue(QueueStopping),
		QueuePaused:   goQueue(QueueStopping),
		QueueStopping: keepQueue(),
	},
	OpResetQueue: queueRow(goQueue(QueueStopped)),
	OpDeleteQueue: {
		QueueStopped:  keepQueue(),
		QueueStarted:  refuseQueue("JBC$_QUENOTSTOP"),
		QueuePaused:   refuseQueue("JBC$_QUENOTSTOP"),
		QueueStopping: refuseQueue("JBC$_QUENOTSTOP"),
	},
	OpModifyQueue: {
		QueueStopped:  keepQueue(),
		QueueStarted:  refuseQueue("JBC$_QUENOTMOD"),
		QueuePaused:   refuseQueue("JBC$_QUENOTMOD"),
		QueueStopping: refuseQueue("JBC$_QUENOTMOD"),
	},
}

// queue finds the queue named name, or refuses with JBC$_NOSUCHQUE.
func (d *Database) queue(name string) (*Queue, error) {
	q := d.Queue(name)
	if q == nil {
		return nil, refuse("JBC$_NOSUCHQUE")
	}

	return q, nil
}

// applyQueue looks up op's cell for q's state and, unless it refuses,
// moves q to its next state.
func applyQueue(q *Queue, op QueueOp) error {
	cell := queueRules[op][q.State]
	if cell.refuse != "" {
		return refuse(cell.refuse)
	}

	if !cell.keep {
		q.State = cell.next
	}

	return nil
}

// settle stops a stopping queue whose last executing job has ended.
func (d *Database) settle(q *Queue) {
	if q.State == QueueStopping && d.Executing(q.Name) == 0 {
		q.State = QueueStopped
	}
}

// changeQueue applies set to a copy of q and keeps the copy only if it
// passes the queue's checks, so a bad value changes nothing.
func changeQueue(q *Queue, set func(*Queue)) error {
	if set == nil {
		return nil
	}

	c := *q
	c.ACL = append([]ACE(nil), q.ACL...)

	set(&c)

	c.Name, c.State, c.Node = q.Name, q.State, q.Node // not set's to change

	if err := c.check(); err != nil {
		return &Refusal{Name: "JBC$_INVPARVAL", Detail: err.Error()}
	}

	*q = c

	return nil
}

// CreateQueue is INITIALIZE/QUEUE/BATCH: a new queue named name with
// NewQueue's defaults, changed by set (the qualifiers given), stopped.
// A queue of that name that exists is changed instead, by set, if it is
// stopped (JBC$_QUENOTMOD, changing nothing, if not), as VMS 7.1's
// CREATE_QUEUE of an existing queue succeeded. created says which it
// was. A name that isn't a queue's is JBC$_INVPARLEN or JBC$_INVQUENAM, a
// value out of range JBC$_INVPARVAL.
func (d *Database) CreateQueue(name string, set func(*Queue)) (q *Queue, created bool, err error) {
	if err := ValidQueueName(name); err != nil {
		return nil, false, err
	}

	if q := d.Queue(name); q != nil {
		if err := applyQueue(q, OpModifyQueue); err != nil {
			return q, false, err
		}

		return q, false, changeQueue(q, set)
	}

	q = NewQueue(name)
	if err := changeQueue(q, set); err != nil {
		return nil, false, err
	}

	d.Queues = append(d.Queues, q)
	d.sortQueues()

	return q, true, nil
}

// SetQueue is SET QUEUE: the queue's attributes changed by set, in any
// state.
func (d *Database) SetQueue(name string, set func(*Queue)) error {
	q, err := d.queue(name)
	if err != nil {
		return err
	}

	return changeQueue(q, set)
}

// StartQueue is START/QUEUE: the queue started (or continued) on node.
func (d *Database) StartQueue(name, node string) error {
	q, err := d.queue(name)
	if err != nil {
		return err
	}

	if err := applyQueue(q, OpStartQueue); err != nil {
		return err
	}

	q.Node = node

	return nil
}

// PauseQueue is STOP/QUEUE: the queue paused. Its executing jobs are the
// caller's to suspend.
func (d *Database) PauseQueue(name string) error {
	q, err := d.queue(name)
	if err != nil {
		return err
	}

	return applyQueue(q, OpPauseQueue)
}

// StopQueueNext is STOP/QUEUE/NEXT: the queue stops now if nothing is
// executing, or when its last executing job ends (Finish).
func (d *Database) StopQueueNext(name string) error {
	q, err := d.queue(name)
	if err != nil {
		return err
	}

	if err := applyQueue(q, OpStopQueueNext); err != nil {
		return err
	}

	d.settle(q)

	return nil
}

// ResetQueue is STOP/QUEUE/RESET: the queue stopped at once, and each
// executing job marked to end with JBC$_QUERESET. It returns those jobs,
// whose processes the caller stops; each then goes to Finish.
func (d *Database) ResetQueue(name string) ([]*Job, error) {
	q, err := d.queue(name)
	if err != nil {
		return nil, err
	}

	if err := applyQueue(q, OpResetQueue); err != nil {
		return nil, err
	}

	var ending []*Job

	for _, j := range d.QueueJobs(name) {
		if j.Status == JobExecuting {
			j.Ending = vmsdef.Symbols["JBC$_QUERESET"]
			j.Requeue = nil
			ending = append(ending, j)
		}
	}

	return ending, nil
}

// DeleteQueue is DELETE/QUEUE: a stopped queue removed with its jobs.
func (d *Database) DeleteQueue(name string) error {
	q, err := d.queue(name)
	if err != nil {
		return err
	}

	if err := applyQueue(q, OpDeleteQueue); err != nil {
		return err
	}

	kept := d.Jobs[:0]

	for _, j := range d.Jobs {
		if j.Queue != q.Name {
			kept = append(kept, j)
		}
	}

	d.Jobs = kept

	for i, other := range d.Queues {
		if other == q {
			d.Queues = append(d.Queues[:i], d.Queues[i+1:]...)

			break
		}
	}

	return nil
}

// StartManager is START/QUEUE/MANAGER: the manager's state started, with
// the directory given (kept, not used).
func (d *Database) StartManager(directory string, now time.Time) {
	d.Manager.State = ManagerStarted
	d.Manager.Started = now

	if directory != "" {
		d.Manager.Directory = directory
	}
}

// StopManager is STOP/QUEUE/MANAGER: the manager's state stopped, so the
// next session doesn't start it.
func (d *Database) StopManager() { d.Manager.State = ManagerStopped }
