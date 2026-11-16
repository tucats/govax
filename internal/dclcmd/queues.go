package dclcmd

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The queue commands (docs/planning/PHASE-98 - batch queues.md, subtask
// 6, Decision 10): START and STOP/QUEUE/MANAGER, INITIALIZE/QUEUE,
// START/QUEUE, STOP/QUEUE, SET QUEUE, and DELETE/QUEUE; SHOW QUEUE is
// showqueue.go's. Each reads its qualifiers, then calls corevms's queue
// operation, the code $SNDJBC's functions call too (Decision 5), and
// says what VMS 7.1 said (testdata/bat98's BAT98.LOG):
//
//	$ INITIALIZE/QUEUE/BATCH/JOB_LIMIT=2 BAT98_B
//	%JBC-I-QUENOTMOD, modifications not made to running queue
//	$ DELETE/QUEUE BAT98_B
//	%DELETE-E-NOTDELETED, error deleting BAT98_B
//	-JBC-E-QUENOTSTOP, queue must be stopped to perform operation
//
// A command that succeeds says nothing and leaves $STATUS %X10040001,
// JBC$_NORMAL with the inhibit bit, as VMS's did.

// The queue commands' statuses.
var (
	// jbcNormalStatus is a queue command's $STATUS when it succeeds.
	jbcNormalStatus = vmsdef.Symbols["JBC$_NORMAL"] | stsInhibitMsg

	// The QUEMAN facility's INVQUAVAL, the queue commands' refusal of a
	// qualifier's value (%X10FB132C in the probe): the shared message
	// SHR$_INVQUAVAL in facility 251, severe.
	quemanInvQuaVal = 251<<16 | vmsdef.LibrarySymbols["SHR$_INVQUAVAL"] | 4

	// DELETE/QUEUE's: DELETE's facility (147), SEARCHFAIL for a queue
	// that isn't there (%X10931238) and NOTDELETED for any other refusal
	// (%X1093137A).
	deleteQueueSearchFail = deleteSearchFail
	deleteNotDeleted      = 147<<16 | vmsdef.LibrarySymbols["SHR$_NOTDELETED"] | 2
)

// QueueQualifier is one of a queue's attribute qualifiers as the command
// line gave it: its name (JOB_LIMIT), whether it was negated, and its
// value (a /PROTECTION's elements in List).
type QueueQualifier struct {
	Name    string
	Negated bool
	Value   string
	List    []string
}

// QueueOptions are INITIALIZE/QUEUE's, START/QUEUE's, and SET QUEUE's
// qualifiers: /BATCH and /START (INITIALIZE's), and the attributes.
type QueueOptions struct {
	Batch      bool
	Start      bool
	Attributes []QueueQualifier
}

// QueueAttributeQualifiers are the attribute qualifiers' names, the
// grammar's: the console reads each into a QueueQualifier.
var QueueAttributeQualifiers = []string{
	"JOB_LIMIT", "BASE_PRIORITY", "OWNER_UIC", "PROTECTION", "WSDEFAULT", "WSQUOTA", "WSEXTENT",
	"CPUDEFAULT", "CPUMAXIMUM", "RETAIN", "DESCRIPTION",
}

// setStatus makes status the command's $STATUS, when the command has one.
func (c *Context) setStatus(status uint32) {
	if c.SetStatus != nil {
		c.SetStatus(status)
	}
}

// invalidQueueValue is QUEMAN's refusal of a qualifier's value:
//
//	%QUEMAN-F-INVQUAVAL, value '0' invalid for /JOB_LIMIT qualifier
func invalidQueueValue(value, qualifier string) error {
	return vmserrors.ConditionError{
		Status: quemanInvQuaVal | stsInhibitMsg,
		Text:   "QUEMAN-F-INVQUAVAL, value '" + value + "' invalid for /" + qualifier + " qualifier",
	}
}

// queueNumber reads a numeric qualifier's value, decimal, within lo and
// hi; anything else is INVQUAVAL, as VMS 7.1's /JOB_LIMIT=0, 256, and
// /BASE_PRIORITY=99 were.
func queueNumber(q QueueQualifier, lo, hi int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(q.Value))
	if err != nil || n < lo || n > hi {
		return 0, invalidQueueValue(q.Value, q.Name)
	}

	return n, nil
}

// queueSet reads the attribute qualifiers into a change to a queue, each
// value checked as QUEMAN checks it before anything is asked of the queue
// manager. The working-set values, CPU times, and the owner's and
// protection's forms are refused the same way (unconfirmed: the probe
// gave none of them a bad value).
func (c *Context) queueSet(attrs []QueueQualifier) (corevms.QueueSet, error) {
	var sets []corevms.QueueSet

	add := func(s corevms.QueueSet) { sets = append(sets, s) }

	for _, q := range attrs {
		s, err := c.queueAttribute(q)
		if err != nil {
			return nil, err
		}

		add(s)
	}

	if len(sets) == 0 {
		return nil, nil
	}

	return func(q *jobqueue.Queue) {
		for _, s := range sets {
			s(q)
		}
	}, nil
}

// queueAttribute reads one attribute qualifier.
func (c *Context) queueAttribute(q QueueQualifier) (corevms.QueueSet, error) {
	workingSet := func(set func(*jobqueue.Queue, int)) (corevms.QueueSet, error) {
		if q.Negated {
			return func(j *jobqueue.Queue) { set(j, 0) }, nil
		}

		n, err := queueNumber(q, 0, 1<<30)
		if err != nil {
			return nil, err
		}

		return func(j *jobqueue.Queue) { set(j, n) }, nil
	}

	cpuTime := func(set func(*jobqueue.Queue, jobqueue.CPUTime)) (corevms.QueueSet, error) {
		if q.Negated {
			return func(j *jobqueue.Queue) { set(j, 0) }, nil
		}

		t, err := jobqueue.ParseCPUTime(q.Value)
		if err != nil {
			return nil, invalidQueueValue(q.Value, q.Name)
		}

		return func(j *jobqueue.Queue) { set(j, t) }, nil
	}

	switch q.Name {
	case "JOB_LIMIT":
		n, err := queueNumber(q, 1, jobqueue.MaxJobLimit)
		if err != nil {
			return nil, err
		}

		return func(j *jobqueue.Queue) { j.JobLimit = n }, nil
	case "BASE_PRIORITY":
		n, err := queueNumber(q, 0, jobqueue.MaxPriority)
		if err != nil {
			return nil, err
		}

		return func(j *jobqueue.Queue) { j.BasePriority = n }, nil
	case "OWNER_UIC":
		uic, err := security.ParseUIC(q.Value, c.identifierLookup())
		if err != nil {
			return nil, invalidQueueValue(q.Value, q.Name)
		}

		return func(j *jobqueue.Queue) { j.Owner = jobqueue.UIC(uic) }, nil
	case "PROTECTION":
		return queueProtection(q)
	case "WSDEFAULT":
		return workingSet(func(j *jobqueue.Queue, n int) { j.WSDefault = n })
	case "WSQUOTA":
		return workingSet(func(j *jobqueue.Queue, n int) { j.WSQuota = n })
	case "WSEXTENT":
		return workingSet(func(j *jobqueue.Queue, n int) { j.WSExtent = n })
	case "CPUDEFAULT":
		return cpuTime(func(j *jobqueue.Queue, t jobqueue.CPUTime) { j.CPUDefault = t })
	case "CPUMAXIMUM":
		return cpuTime(func(j *jobqueue.Queue, t jobqueue.CPUTime) { j.CPUMaximum = t })
	case "RETAIN":
		return queueRetain(q)
	case "DESCRIPTION":
		text := q.Value
		if q.Negated {
			text = ""
		}

		return func(j *jobqueue.Queue) { j.Description = text }, nil
	}

	return nil, nil
}

// queueProtection reads /PROTECTION: the categories it names set, the
// others kept (a new queue's being the default's); with no value, the
// default protection.
func queueProtection(q QueueQualifier) (corevms.QueueSet, error) {
	code := codeText(q.List)
	if code == "" {
		return func(j *jobqueue.Queue) { j.Protection = jobqueue.DefaultProtection }, nil
	}

	if _, err := jobqueue.ParseProtection(code, 0); err != nil {
		return nil, invalidQueueValue(code, q.Name)
	}

	return func(j *jobqueue.Queue) {
		j.Protection, _ = jobqueue.ParseProtection(code, j.Protection)
	}, nil
}

// queueRetain reads a queue's /RETAIN: ALL (the default) or ERROR;
// /NORETAIN keeps no job.
func queueRetain(q QueueQualifier) (corevms.QueueSet, error) {
	if q.Negated {
		return func(j *jobqueue.Queue) { j.Retain = jobqueue.RetainDefault }, nil
	}

	value := strings.ToUpper(strings.TrimSpace(q.Value))

	switch {
	case value == "" || strings.HasPrefix("ALL", value):
		return func(j *jobqueue.Queue) { j.Retain = jobqueue.RetainAlways }, nil
	case strings.HasPrefix("ERROR", value):
		return func(j *jobqueue.Queue) { j.Retain = jobqueue.RetainError }, nil
	}

	return nil, invalidQueueValue(q.Value, q.Name)
}

// queueDone is a queue command's ending: its failure shown as a JBC
// message, or $STATUS JBC$_NORMAL.
func (c *Context) queueDone(err error) error {
	if err != nil {
		return queueFailure(err)
	}

	c.setStatus(jbcNormalStatus)

	return nil
}

// noQueues is a command's failure when govax has no RTL to ask.
func noQueues() error { return queueCondition(vmsdef.Symbols["JBC$_JOBQUEDIS"]) }

// StartQueueManager is START/QUEUE/MANAGER [directory].
func (c *Context) StartQueueManager(directory string) error {
	if c.RTL == nil {
		return noQueues()
	}

	return c.queueDone(c.RTL.StartQueueManager(strings.TrimSpace(directory)))
}

// StopQueueManager is STOP/QUEUE/MANAGER[/CLUSTER].
func (c *Context) StopQueueManager() error {
	if c.RTL == nil {
		return noQueues()
	}

	return c.queueDone(c.RTL.StopQueueManager())
}

// InitializeQueue is INITIALIZE/QUEUE name: a batch queue made, or one
// that exists changed (corevms's CreateQueue).
func (c *Context) InitializeQueue(name string, opts QueueOptions) error {
	if c.RTL == nil {
		return noQueues()
	}

	set, err := c.queueSet(opts.Attributes)
	if err != nil {
		return err
	}

	return c.queueDone(c.RTL.CreateQueue(name, opts.Batch, opts.Start, set))
}

// StartQueue is START/QUEUE name, with INITIALIZE's attributes.
func (c *Context) StartQueue(name string, opts QueueOptions) error {
	if c.RTL == nil {
		return noQueues()
	}

	set, err := c.queueSet(opts.Attributes)
	if err != nil {
		return err
	}

	return c.queueDone(c.RTL.StartQueue(name, set))
}

// StopQueue is STOP/QUEUE name: paused, or with /NEXT stopped after its
// executing jobs, or with /RESET stopped at once.
func (c *Context) StopQueue(name string, next, reset bool) error {
	if c.RTL == nil {
		return noQueues()
	}

	how := corevms.StopPause

	switch {
	case reset:
		how = corevms.StopReset
	case next:
		how = corevms.StopNext
	}

	return c.queueDone(c.RTL.StopQueue(name, how))
}

// AbortJob is STOP/QUEUE/ENTRY=entry name: the executing job ended with
// JBC$_JOBABORT (corevms's AbortJob).
func (c *Context) AbortJob(entry int, name string) error {
	if c.RTL == nil {
		return noQueues()
	}

	return c.queueDone(c.RTL.AbortJob(entry, name))
}

// SetQueue is SET QUEUE name: the queue's attributes changed.
func (c *Context) SetQueue(name string, opts QueueOptions) error {
	if c.RTL == nil {
		return noQueues()
	}

	set, err := c.queueSet(opts.Attributes)
	if err != nil {
		return err
	}

	return c.queueDone(c.RTL.SetQueue(name, set))
}

// DeleteQueue is DELETE/QUEUE name: a stopped queue and its jobs gone.
// A refusal is DELETE's own message with the reason under it, as VMS
// 7.1's was: SEARCHFAIL for a queue that isn't there, NOTDELETED for the
// rest.
func (c *Context) DeleteQueue(name string) error {
	name = strings.ToUpper(strings.TrimSpace(name))

	st := vmsdef.Symbols["JBC$_JOBQUEDIS"]

	if c.RTL != nil {
		err := c.RTL.DeleteQueue(name)
		if err == nil {
			c.setStatus(jbcNormalStatus)

			return nil
		}

		if st = corevms.QueueErrorStatus(err); st == 0 {
			return err
		}
	}

	if st&^7 == vmsdef.Symbols["JBC$_NOSUCHQUE"]&^7 {
		return vmserrors.ConditionError{
			Status: deleteQueueSearchFail | stsInhibitMsg,
			Text:   "DELETE-W-SEARCHFAIL, error searching for " + name + "\n" + ConditionLine(st),
		}
	}

	return vmserrors.ConditionError{
		Status: deleteNotDeleted | stsInhibitMsg,
		Text:   "DELETE-E-NOTDELETED, error deleting " + name + "\n" + ConditionLine(st),
	}
}
