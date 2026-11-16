package corevms

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// Queue protection (docs/planning/PHASE-98 - batch queues.md, Decision
// 8; internal/security's queue.go has the rule). A queue is an object of
// the security class QUEUE, kept in the queue database (queues.go); its
// owner, protection code, and ACL decide who may submit to it, manage it,
// and see or change the jobs in it, and the OPER privilege overrides them
// all (ruleQueueOperator). The methods here are the one place those
// decisions are made, for DCL's queue commands, $SNDJBC, and $GETQUI
// alike, and the QUEUE class's object for $GET_SECURITY, $SET_SECURITY,
// and SET and SHOW SECURITY/CLASS=QUEUE.

// jbcNoSuchQue is the queue manager's status for a queue that isn't there.
var jbcNoSuchQue = vmsdef.Symbols["JBC$_NOSUCHQUE"]

// isOperator reports whether the process holds OPER, which gives it
// every access to every queue and job.
func (env *Environment) isOperator() bool {
	return env.holdsPrivilege(ruleQueueOperator)
}

// QueueAccess reports whether the process may have every access in want
// (security.QueueSubmit, QueueManage, ...) to queue q.
func (env *Environment) QueueAccess(q *jobqueue.Queue, want security.Access) bool {
	return env.isOperator() || security.CheckQueue(env.Credentials(), q.Profile(), want).Granted
}

// JobAccess reports whether the process may have every access in want
// (security.QueueRead to see it or SYNCHRONIZE with it, QueueDelete to
// change, delete, or stop it) to job j in queue q.
func (env *Environment) JobAccess(q *jobqueue.Queue, j *jobqueue.Job, want security.Access) bool {
	if env.isOperator() {
		return true
	}

	return security.CheckJob(env.Credentials(), q.Profile(), security.UIC(j.Owner), want).Granted
}

// QueueSecurityVisible reports whether the process may see queue q's
// owner, protection, and ACL: SHOW SECURITY, SHOW QUEUE/FULL's /OWNER and
// /PROTECTION (VMS 7.1 wrote "Access to security information denied."
// without it), and $GETQUI's items for them. Read access gives it, and
// so does control: in the probe a queue's owner whose protection gave it
// nothing, and a SYSPRV process whose system category was given nothing,
// saw both.
func (env *Environment) QueueSecurityVisible(q *jobqueue.Queue) bool {
	if env.isOperator() {
		return true
	}

	d := security.CheckQueue(env.Credentials(), q.Profile(), security.QueueRead)

	return d.Granted || d.Access&security.Control != 0
}

// QueueControl reports whether the process may change queue q's
// protection and ACL: control access (its owner, the system category, an
// ACE's CONTROL, BYPASS) or OPER. VMS 7.1 refused it to the SECURITY
// privilege, READALL, and manage access alike (the probe's case 23).
func (env *Environment) QueueControl(q *jobqueue.Queue) bool {
	return env.isOperator() || security.CheckQueue(env.Credentials(), q.Profile(), security.Control).Granted
}

// MayCreateQueue is the check INITIALIZE/QUEUE (and $SNDJBC's
// CREATE_QUEUE) makes before creating a queue that doesn't exist: 0, or
// JBC$_NOOPER. One that exists is changed, which needs manage access to
// it instead.
func (env *Environment) MayCreateQueue() uint32 {
	return env.requirePrivilege(ruleQueueOperator)
}

// readQueues reads the queue database, an empty one when the System has
// none (a test's) or its file doesn't exist yet.
func (env *Environment) readQueues() (*jobqueue.Database, error) {
	path := env.System.QueueFile()
	if path == "" {
		return jobqueue.New(), nil
	}

	return jobqueue.Read(path)
}

// changeQueues applies change to the queue database and saves it, under
// the file's lock, so that the change is made to what is there now. The
// queue manager must run in this session (managerHere): another session
// may only read the queues (Decision 4).
func (env *Environment) changeQueues(change func(*jobqueue.Database) error) error {
	return env.changeQueuesHere(change)
}

// queueStatusError is a refusal that is a condition value of its own.
type queueStatusError uint32

func (e queueStatusError) Error() string { return vmsdef.FormatMessage(uint32(e)) }

// queueStatus makes status an error.
func queueStatus(status uint32) error { return queueStatusError(status) }

// QueueErrorStatus is the condition value of an error a queue operation
// returned: a refusal's JBC$_ (or SS$_) status, or 0 for any other error
// (a host file that can't be read).
func QueueErrorStatus(err error) uint32 {
	var st queueStatusError
	if errors.As(err, &st) {
		return uint32(st)
	}

	var refusal *jobqueue.Refusal
	if errors.As(err, &refusal) {
		return refusal.Code()
	}

	return 0
}

// QueueProfile is what SHOW SECURITY/CLASS=QUEUE shows of the queue
// called name: its name as the database has it, its profile, and its
// ACL's bytes. status is 0, JBC$_NOSUCHQUE, or SS$_NOPRIV when the
// process may see neither (QueueSecurityVisible). err is a database that
// can't be read.
func (env *Environment) QueueProfile(name string) (queue string, p security.Profile, status uint32, err error) {
	d, err := env.readQueues()
	if err != nil {
		return "", security.Profile{}, 0, err
	}

	q := d.Queue(queueName(name))
	if q == nil {
		return "", security.Profile{}, jbcNoSuchQue, nil
	}

	if !env.QueueSecurityVisible(q) {
		return q.Name, security.Profile{}, ssNoPrivilege, nil
	}

	return q.Name, q.Profile(), 0, nil
}

// QueueSecurity is a change to a queue's profile: its owner, protection,
// and ACL, as SET SECURITY/CLASS=QUEUE computes them from the old ones.
type QueueSecurity struct {
	Owner      security.UIC
	Protection security.Protection
	ACL        []security.ACE
}

// ChangeQueueSecurity is SET SECURITY/CLASS=QUEUE on the queue called
// name: change is given the queue's profile and changes it in place.
// The process needs control access (QueueControl), and to give the queue
// another owner, security.CanChangeOwner's rule or OPER (unconfirmed for
// queues: the probe changed no queue's owner but SYSTEM's own). The
// refusals are SS$_NOPRIV, as VMS 7.1 gave them, and JBC$_NOSUCHQUE for
// a queue that isn't there (unconfirmed). change's own error stops the
// change and is returned as it is.
func (env *Environment) ChangeQueueSecurity(name string, change func(*QueueSecurity) error) error {
	return env.changeQueues(func(d *jobqueue.Database) error {
		q := d.Queue(queueName(name))
		if q == nil {
			return queueStatus(jbcNoSuchQue)
		}

		if !env.QueueControl(q) {
			return queueStatus(ssNoPrivilege)
		}

		old := q.Profile()
		s := QueueSecurity{Owner: old.Owner, Protection: old.Protection, ACL: old.ACL}

		if err := change(&s); err != nil {
			return err
		}

		if s.Owner != old.Owner && !env.isOperator() && !security.CanChangeOwner(env.Credentials(), old, s.Owner) {
			return queueStatus(ssNoPrivilege)
		}

		return d.SetQueue(q.Name, func(c *jobqueue.Queue) {
			c.Owner = jobqueue.UIC(s.Owner)
			c.Protection = jobqueue.Protection(s.Protection)
			c.ACL = queueACL(s.ACL)
		})
	})
}

// queueACL is acl as the database keeps it, each entry's bytes.
func queueACL(acl []security.ACE) []jobqueue.ACE {
	out := make([]jobqueue.ACE, 0, len(acl))
	for _, a := range acl {
		out = append(out, jobqueue.ACE(append([]byte(nil), a.Data...)))
	}

	return out
}

// queueName is name as a queue's: blanks and a trailing colon taken off.
func queueName(name string) string {
	return strings.TrimSuffix(strings.TrimSpace(name), ":")
}

// queueObject is a queue as $GET_SECURITY's and $SET_SECURITY's object:
// a copy of the queue as the database had it when the call found it.
type queueObject struct {
	env   *Environment
	queue *jobqueue.Queue
}

// findQueueObject is the queue called name, its OBJECT_NAME its name; a
// name no queue has is JBC$_NOSUCHQUE (unconfirmed: the probe asked
// $GET_SECURITY about no queue).
func findQueueObject(env *Environment, name string) (securityObject, string, uint32) {
	d, err := env.readQueues()
	if err != nil {
		return nil, "", jbcNoSuchQue
	}

	q := d.Queue(queueName(name))
	if q == nil {
		return nil, "", jbcNoSuchQue
	}

	return queueObject{env: env, queue: q}, q.Name, ssNormal
}

func (o queueObject) Profile() security.Profile { return o.queue.Profile() }

func (o queueObject) ACL() []byte { return security.EncodeACL(o.queue.Profile().ACL) }

func (o queueObject) MayRead() bool { return o.env.QueueSecurityVisible(o.queue) }

func (o queueObject) MayChange() bool { return o.env.QueueControl(o.queue) }

func (o queueObject) SetACL(acl []byte) error {
	entries, err := security.DecodeACL(acl)
	if err != nil {
		return err
	}

	return o.env.ChangeQueueSecurity(o.queue.Name, func(s *QueueSecurity) error {
		s.ACL = entries

		return nil
	})
}

func (o queueObject) SetOwnerProtection(owner security.UIC, protection security.Protection) error {
	return o.env.ChangeQueueSecurity(o.queue.Name, func(s *QueueSecurity) error {
		s.Owner, s.Protection = owner, protection

		return nil
	})
}
