package dclshell

import (
	"strings"
	"time"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// SYNCHRONIZE (docs/planning/PHASE-98 - batch queues.md, subtask 8): DCL
// waits for a batch job to end, and the job's final status is the
// command's, as VMS 7.1's was in the probe (A.4):
//
//	SYNCHRONIZE [job-name] [/ENTRY=n] [/QUEUE=queue-name]
//
// The job is named by its entry number, or by its name among the
// process's own jobs in /QUEUE's queue (SYS$BATCH by default). A job that
// isn't there is JBC$_NOSUCHENT by number (a job already gone too) and
// JBC$_NOSUCHJOB by name; another user's job needs read access to it.
// Once the job ends, $STATUS is its status with STS$M_INHIB_MSG set
// (%X10000001, %X10018292), and a status that isn't a success has its
// message shown, as VMS 7.1 showed %RMS-E-FNF and %JBC-F-JOBABORT. A job
// already retained gives its status at once. Like WAIT (dclwait.go), the
// other processes run meanwhile, and a Ctrl/Y or Ctrl/C ends the wait.

// syncPoll is how long the host waits between looks at the job when no
// process can run.
const syncPoll = 20 * time.Millisecond

// synchronizeCommand is SYNCHRONIZE.
func (c *Interpreter) synchronizeCommand(_ int64, r *dcl.Result) error {
	if c.RTL == nil {
		return c.jobCondition(vmsdef.Symbols["JBC$_JOBQUEDIS"])
	}

	name := strings.ToUpper(strings.TrimSpace(r.String("JOB_NAME")))
	entry := 0

	if r.Present("ENTRY") {
		entry = int(r.Int("ENTRY"))
	}

	since := c.RTL.JobEndMark()

	found, status, done, err := c.RTL.SyncTarget(entry, name, strings.TrimSpace(r.String("QUEUE")))
	if err != nil {
		if st := corevms.QueueErrorStatus(err); st != 0 {
			return c.jobCondition(st)
		}

		return err
	}

	for !done {
		if status, done = c.RTL.JobEnded(found, since); done {
			break
		}

		if c.syncInterrupted(func() bool {
			_, ended := c.RTL.JobEnded(found, since)

			return ended
		}) {
			c.keepStatus()

			return nil
		}
	}

	c.RTL.ConsumeJobEnd(found)

	status |= stsInhibitMsg

	if status&1 != 0 {
		c.SetCommandStatus(status, nil)

		return nil
	}

	return c.jobCondition(status)
}

// jobCondition is a queue command's failure with condition value code:
// its message shown, $STATUS code with STS$M_INHIB_MSG set.
func (c *Interpreter) jobCondition(code uint32) error {
	code |= stsInhibitMsg

	return vmserrors.ConditionError{Status: code, Text: c.statusText(code &^ stsInhibitMsg)}
}

// syncInterrupted waits, once, for ended to report true, the other
// processes running meanwhile: a subprocess's DCL waits in the scheduler,
// process 1's lends the CPU (dcllend.go). It reports whether a Ctrl/Y or
// Ctrl/C has ended the wait.
func (c *Interpreter) syncInterrupted(ended func() bool) bool {
	if cli := c.subCLI; cli != nil {
		over := func() bool { return ended() || cli.interrupted }
		cli.handBack(cliYield{kind: yieldTimer, over: over, deadline: c.machineDeadline(syncPoll)})

		return cli.interrupted
	}

	attention := func() bool { return c.machine.Engine != nil && isAttentionKey(c.machine.Engine.PendingAttention()) }

	c.pollHostJobs()

	if attention() {
		return true
	}

	// When nothing else runs, the host waits a little, the quantum
	// clock moving on with it as far as the next timer (an /AFTER time
	// among them), and the job controller looks at the time.
	if ran, _ := c.lendCPU(func() bool { return ended() || attention() }, 0); !ran {
		if c.machine.Engine == nil {
			time.Sleep(syncPoll)
		} else {
			c.RTL.HostIdle(c.machine.Engine, func() { time.Sleep(syncPoll) })
		}

		c.RTL.PollJobs() // HostIdle's look, or the first without an engine
	}

	return attention()
}
