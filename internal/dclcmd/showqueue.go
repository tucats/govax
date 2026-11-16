package dclcmd

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/security"
)

// SHOW QUEUE and SHOW QUEUE/MANAGER (docs/planning/PHASE-98 - batch
// queues.md, subtask 6), laid out as VMS 7.1 laid them out
// (testdata/bat98's BAT98.LOG):
//
//	Batch queue BAT98_A, stopped
//	  <GOVAX PROBE A>
//	  /BASE_PRIORITY=3 /CPUDEFAULT=00:05:00 /CPUMAXIMUM=00:10:00 /JOB_LIMIT=3 /OWNER=[SYSTEM] /PROTECTION=(S:M,O:D,G:R,W:S)
//	  /RETAIN=ERROR /WSDEFAULT=200 /WSEXTENT=800 /WSQUOTA=400
//
// A queue that has been started names its node ("idle, on GOVAX::"), its
// description follows in angle brackets, and /FULL adds its attributes in
// order of name and its ACL, or, to a process that may not see its
// security (corevms's QueueSecurityVisible), the attributes without
// /OWNER and /PROTECTION and "Access to security information denied.".
// Queues are separated by a blank line, and each queue's jobs follow it
// (jobs.go's showQueueJobs); /BRIEF leaves out the description.

// showQueueWidth is the widest line /FULL's attributes make: VMS 7.1
// wrapped a line of 120 characters before an attribute of 13 and kept one
// of 111 (unconfirmed: the width is somewhere from 120 to 132).
const showQueueWidth = 132

// ShowQueueOptions are SHOW QUEUE's qualifiers.
type ShowQueueOptions struct {
	Full    bool
	All     bool
	Brief   bool
	Summary bool
	Batch   bool

	// ByStatus is /BY_JOB_STATUS's statuses, nil for every job.
	ByStatus map[jobqueue.JobStatus]bool
}

// ShowQueue is SHOW QUEUE [name]: the queues whose names match name
// (every queue when it is empty). $STATUS is SS$_NORMAL with the inhibit
// bit, %X10000001, as VMS 7.1 left it.
func (c *Context) ShowQueue(name string, opts ShowQueueOptions) error {
	if c.RTL == nil {
		return noQueues()
	}

	views, err := c.RTL.QueueViews(name)
	if err != nil {
		return queueFailure(err)
	}

	for i, v := range views {
		if i > 0 {
			c.Printf("\n")
		}

		c.showOneQueue(v, opts)
	}

	c.setStatus(ssNormal | stsInhibitMsg)

	return nil
}

// showOneQueue shows one queue.
func (c *Context) showOneQueue(v corevms.QueueView, opts ShowQueueOptions) {
	q := &v.Queue

	head := "Batch queue " + q.Name + ", " + v.State
	if q.Node != "" {
		head += ", on " + q.Node + "::"
	}

	c.Printf("%s\n", head)

	if q.Description != "" && !opts.Brief {
		c.Printf("  <%s>\n", q.Description)
	}

	if opts.Full {
		c.showQueueAttributes(v)
	}

	if opts.Summary {
		if v.JobsCount == 0 {
			c.Printf("\n    Job summary:  queue is empty\n")
		}

		return
	}

	c.showQueueJobs(v.Jobs, opts)
}

// showQueueAttributes is /FULL's lines: the attributes, wrapped at
// showQueueWidth with a blank left at the end of a wrapped line (as
// VMS's), then the ACL's entries, each on a line of its own indented
// nine; or for a process that may not see the queue's security, the
// attributes without /OWNER and /PROTECTION and a line saying so.
func (c *Context) showQueueAttributes(v corevms.QueueView) {
	q := &v.Queue
	names := rightsNames{c}

	var items []string

	add := func(format string, args ...any) { items = append(items, fmt.Sprintf(format, args...)) }

	add("/BASE_PRIORITY=%d", q.BasePriority)

	if q.CPUDefault != 0 {
		add("/CPUDEFAULT=%s", q.CPUDefault)
	}

	if q.CPUMaximum != 0 {
		add("/CPUMAXIMUM=%s", q.CPUMaximum)
	}

	add("/JOB_LIMIT=%d", q.JobLimit)

	if v.Security {
		add("/OWNER=%s", security.IdentifierText(uint32(q.Owner), names))
		add("/PROTECTION=%s", q.Protection)
	}

	switch q.Retain {
	case jobqueue.RetainAlways:
		add("/RETAIN=ALL")
	case jobqueue.RetainError:
		add("/RETAIN=ERROR")
	}

	if q.WSDefault != 0 {
		add("/WSDEFAULT=%d", q.WSDefault)
	}

	if q.WSExtent != 0 {
		add("/WSEXTENT=%d", q.WSExtent)
	}

	if q.WSQuota != 0 {
		add("/WSQUOTA=%d", q.WSQuota)
	}

	line := "  " + items[0]

	for _, item := range items[1:] {
		if len(line)+1+len(item) > showQueueWidth {
			c.Printf("%s \n", line)
			line = "  " + item

			continue
		}

		line += " " + item
	}

	c.Printf("%s\n", line)

	if !v.Security {
		c.Printf("  Access to security information denied.\n")

		return
	}

	for _, a := range q.Profile().ACL {
		c.Printf("         %s\n", security.FormatACE(a.Data, names, security.QueueAccessNames))
	}
}

// ShowQueueManager is SHOW QUEUE/MANAGER[/FULL]:
//
//	Queue manager SYS$QUEUE_MANAGER, running, on GOVAX::
//
// /FULL adds the master file (govax's is the host file, Decision 3) and
// the database's location, START/QUEUE/MANAGER's directory (VMS 7.1's
// default, SYS$COMMON:[SYSEXE], when none was given). A manager that
// isn't running is "stopped" (unconfirmed: the probe's VAX ran its
// manager throughout). $STATUS is %X10000001.
func (c *Context) ShowQueueManager(full bool) error {
	if c.RTL == nil {
		return noQueues()
	}

	info, err := c.RTL.QueueManagerStatus()
	if err != nil {
		return err
	}

	state := "stopped"
	if info.Running {
		state = "running"
	}

	if full {
		file := info.File
		if file == "" {
			file = "(none)"
		}

		c.Printf("Master file:  %s\n\n", file)
	}

	c.Printf("Queue manager SYS$QUEUE_MANAGER, %s, on %s::\n", state, info.Node)

	if full {
		dir := info.Directory
		if dir == "" {
			dir = "SYS$COMMON:[SYSEXE]"
		}

		c.Printf("  /ON=(*)\n  Database location:  %s\n", strings.ToUpper(dir))
	}

	c.setStatus(ssNormal | stsInhibitMsg)

	return nil
}
