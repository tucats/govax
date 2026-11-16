package dclcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The job commands (docs/planning/PHASE-98 - batch queues.md, subtask 7,
// Decision 10): SUBMIT, SET ENTRY, DELETE/ENTRY, and SHOW ENTRY, and the
// jobs SHOW QUEUE lists. Each reads its qualifiers, calls corevms's job
// operation (the code $SNDJBC's job functions call too), and says what
// VMS 7.1 said (testdata/bat98's BAT98.LOG, its section JOBS ON A
// STOPPED QUEUE):
//
//	$ SUBMIT/QUEUE=BAT98_B/NOPRINTER BAT98$DISK:[000000]BAT98OK
//	Job BAT98OK (queue BAT98_B, entry 18) pending
//	     pending status caused by queue stopped state
//	$ SHOW ENTRY 18
//	  Entry  Jobname         Username     Blocks  Status
//	  -----  -------         --------     ------  ------
//	     18  BAT98OK         SYSTEM               Pending
//	         On stopped batch queue BAT98_B
//
// SUBMIT leaves $STATUS %X10000001 and the local symbol $ENTRY the new
// job's number; SET ENTRY and DELETE/ENTRY JBC$_NORMAL with the inhibit
// bit, %X10040001.

// SUBMIT's facility (193) and its statuses, as VMS 7.1's $STATUS showed
// them: INVQUAVAL %X10C1132C, OPENIN %X10C1109C, CREJOB %X10C1800C, and
// INVLOGFIL %X10C18324.
const submitFacility = 193 << 16

var (
	submitInvQuaVal  = submitFacility | vmsdef.LibrarySymbols["SHR$_INVQUAVAL"] | 4
	submitOpenIn     = submitFacility | vmsdef.LibrarySymbols["SHR$_OPENIN"] | 4
	submitCreJob     = uint32(submitFacility | 0x800C)
	submitInvLogFil  = uint32(submitFacility | 0x8324)
	submitDoneStatus = ssNormal | stsInhibitMsg
)

// SubmitOptions are SUBMIT's parameters and qualifiers as the command line
// gave them. A ...Given field says the qualifier was there, a No... field
// that it was negated.
type SubmitOptions struct {
	Files []string

	Queue      string
	After      string
	User       string
	Parameters []string
	Hold       bool

	LogFile    string
	LogGiven   bool
	NoLog      bool
	Name       string
	Keep       bool
	NoKeep     bool
	Printer    string
	PrinterSet bool
	NoPrinter  bool
	Notify     bool

	Priority      int
	PriorityGiven bool
	CPUTime       string
	CPUGiven      bool

	WSDefault, WSQuota, WSExtent int

	Restart  bool
	Retain   string
	NoRetain bool
	Note     string
	NoIdent  bool
	Delete   bool
}

// submitMessage is one of SUBMIT's own messages, with the conditions
// under it.
func submitMessage(status uint32, text string, causes ...uint32) error {
	for _, c := range causes {
		text += "\n" + ConditionLine(c)
	}

	return vmserrors.ConditionError{Status: status | stsInhibitMsg, Text: text}
}

// submitInvalid is SUBMIT's refusal of a qualifier's value:
//
//	%SUBMIT-F-INVQUAVAL, value 'I' invalid for /PARAMETERS qualifier
func submitInvalid(value, qualifier string, causes ...uint32) error {
	return submitMessage(submitInvQuaVal,
		"SUBMIT-F-INVQUAVAL, value '"+value+"' invalid for /"+qualifier+" qualifier", causes...)
}

// jobParameters reads /PARAMETERS: at most eight (a ninth is INVQUAVAL,
// naming it), as DCL gives them, a quoted one's case kept and an unquoted
// one uppercased (VMS 7.1 showed /PARAMETERS=lower as "LOWER").
func jobParameters(list []string) ([]string, error) {
	// A value longer than 255 characters is DCL's TKNOVF before SUBMIT sees
	// it, with its first 255 as the segment (VMS 7.1's case 83; govax's DCL
	// checks WRITE's items the same way, not every command's values).
	for _, p := range list {
		if text := dequote(p); len(text) > jobqueue.MaxParameter {
			return nil, vmserrors.NewSegment(vmserrors.CLI_TKNOVF, text[:jobqueue.MaxParameter])
		}
	}

	if len(list) > jobqueue.MaxParameters {
		return nil, submitInvalid(dequote(list[jobqueue.MaxParameters]), "PARAMETERS")
	}

	out := make([]string, len(list))
	for i, p := range list {
		out[i] = dequote(p)
	}

	return out, nil
}

// dequote is a value without the quotes DCL may have left around it: a
// quoted string's text, doubled quotes made one. The grammar has already
// uppercased an unquoted value.
func dequote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return strings.ReplaceAll(s[1:len(s)-1], `""`, `"`)
	}

	return s
}

// jobTime reads a time qualifier's value (/AFTER, /RETAIN=UNTIL=) as DCL
// takes one: an absolute time, TODAY, TOMORROW, a delta from now, or a
// combination.
func (c *Context) jobTime(text, qualifier string) (time.Time, error) {
	text = strings.Trim(strings.TrimSpace(text), `"`)

	if c.ParseTime == nil {
		return time.Time{}, submitInvalid(text, qualifier)
	}

	v, ok := c.ParseTime(text)
	if !ok {
		return time.Time{}, submitInvalid(text, qualifier)
	}

	return corevms.LocalTime(v), nil
}

// jobCPUTime reads /CPUTIME: NONE (no limit given, 0), INFINITE, or a
// time, hh:mm:ss[.cc] or a delta's d-hh:mm:ss.
func jobCPUTime(text string) (jobqueue.CPUTime, error) {
	text = strings.ToUpper(strings.TrimSpace(text))

	switch {
	case text == "NONE":
		return 0, nil
	case strings.HasPrefix("INFINITE", text) && text != "":
		return jobqueue.CPUInfinite, nil
	}

	if t, err := jobqueue.ParseCPUTime(text); err == nil {
		return t, nil
	}

	parts := strings.Split(text, ":")
	for len(parts) < 3 {
		parts = append([]string{"0"}, parts...)
	}

	t, err := jobqueue.ParseCPUTime(strings.Join(parts, ":"))
	if err != nil {
		return 0, submitInvalid(text, "CPUTIME")
	}

	return t, nil
}

// logSpec is a job's log file name as SUBMIT keeps it: spec ($PARSE'd,
// SYS$LOGIN and ".LOG" its defaults), "DUA1:[BAT98LOG]S25.LOG;", or with
// a directory alone "DUA1:[BAT98LOG].LOG;" (VMS 7.1 left the name for the
// job's). A directory that isn't there is INVLOGFIL, RMS$_DNF, and
// SS$_NOSUCHFILE (the probe's case 264).
func (c *Context) logSpec(spec string) (string, error) {
	if c.ContainerSession == nil {
		return strings.ToUpper(spec), nil
	}

	p, found, err := c.ContainerSession.Parse(spec, "SYS$LOGIN:.LOG", "", false, false)
	if err != nil || !found {
		return "", submitMessage(submitInvLogFil, "SUBMIT-F-INVLOGFIL, invalid log file specification",
			vmsdef.Symbols["RMS$_DNF"], vmsdef.Symbols["SS$_NOSUCHFILE"])
	}

	return p.String(), nil
}

// jobRetain reads /RETAIN's value: ALWAYS, ERROR, or UNTIL=time.
func (c *Context) jobRetain(text string) (jobqueue.Retain, time.Time, error) {
	value := strings.ToUpper(strings.TrimSpace(text))

	key, rest, _ := strings.Cut(value, "=")

	switch {
	case value == "" || strings.HasPrefix("ALWAYS", key) && rest == "":
		return jobqueue.RetainAlways, time.Time{}, nil
	case strings.HasPrefix("ERROR", key) && rest == "":
		return jobqueue.RetainError, time.Time{}, nil
	case strings.HasPrefix("UNTIL", key) && key != "" && rest != "":
		t, err := c.jobTime(rest, "RETAIN")

		return jobqueue.RetainUntil, t, err
	}

	return 0, time.Time{}, submitInvalid(text, "RETAIN")
}

// splitFiles is SUBMIT's files: each list element split at "+" too.
func splitFiles(list []string) []string {
	var files []string

	for _, f := range list {
		for part := range strings.SplitSeq(f, "+") {
			if part = strings.TrimSpace(part); part != "" {
				files = append(files, part)
			}
		}
	}

	return files
}

// Submit is SUBMIT file[,...]: a job entered in /QUEUE's queue
// (SYS$BATCH by default), its report written unless /NOIDENTIFY, and
// $ENTRY defined. The job's checks are corevms's SubmitJob's; each
// refusal is SUBMIT's message for it.
func (c *Context) Submit(o SubmitOptions, defineEntry func(string)) error {
	if c.RTL == nil {
		return noQueues()
	}

	r := corevms.JobRequest{
		Queue: o.Queue, Files: splitFiles(o.Files), Name: strings.ToUpper(o.Name), User: o.User,
		Hold: o.Hold, NoLog: o.NoLog, Keep: o.Keep, NoKeep: o.NoKeep, NoPrinter: o.NoPrinter,
		Printer: o.Printer, PrinterGiven: o.PrinterSet, Priority: -1,
		WSDefault: o.WSDefault, WSQuota: o.WSQuota, WSExtent: o.WSExtent,
		Restart: o.Restart, Notify: o.Notify, Note: o.Note, Delete: o.Delete,
	}

	if r.Queue == "" {
		r.Queue = "SYS$BATCH"
	}

	if r.NoPrinter && !r.NoKeep {
		r.Keep = true
	}

	var err error

	if r.Parameters, err = jobParameters(o.Parameters); err != nil {
		return err
	}

	if o.After != "" {
		if r.After, err = c.jobTime(o.After, "AFTER"); err != nil {
			return err
		}
	}

	if o.PriorityGiven {
		r.Priority = o.Priority
	}

	if o.CPUGiven {
		if r.CPUTime, err = jobCPUTime(o.CPUTime); err != nil {
			return err
		}
	}

	switch {
	case o.NoRetain:
		r.Retain = jobqueue.RetainNever
	case o.Retain != "":
		if r.Retain, r.RetainUntil, err = c.jobRetain(o.Retain); err != nil {
			return err
		}
	}

	switch {
	case o.LogGiven && !o.NoLog:
		if r.LogFile, err = c.logSpec(o.LogFile); err != nil {
			return err
		}
	case r.Name != "" && !o.NoLog && c.ContainerSession != nil:
		// A job named by /NAME keeps its default log's name, which is no
		// longer its file's (VMS 7.1 showed HELD98's as
		// SYS$LOGIN:HELD98.LOG expanded). Its directory is the job's to
		// find (unconfirmed: the probe's SYS$LOGIN was always there).
		if p, _, err := c.ContainerSession.Parse(r.Name, "SYS$LOGIN:.LOG", "", true, false); err == nil {
			r.LogFile = p.String()
		}
	}

	s, err := c.RTL.SubmitJob(r)
	if err != nil {
		return submitFailure(err)
	}

	if s.PriorityLowered {
		c.Printf("%%%s\n", vmsdef.FormatMessage(vmsdef.Symbols["JBC$_PRIOSMALL"]))
	}

	if !o.NoIdent {
		for _, line := range s.Message() {
			c.Printf("%s\n", line)
		}
	}

	if defineEntry != nil {
		defineEntry(strconv.Itoa(s.Job.Entry))
	}

	c.setStatus(submitDoneStatus)

	return nil
}

// submitFailure is SUBMIT's message for a refusal: INVQUAVAL for /USER's
// account, OPENIN for a file, CREJOB with the queue manager's reason.
func submitFailure(err error) error {
	var user *corevms.JobUserError
	if errors.As(err, &user) {
		if user.Status != 0 {
			return submitInvalid(strings.ToUpper(user.User), "USER", user.Status)
		}

		return submitInvalid(strings.ToUpper(user.User), "USER")
	}

	var file *rms.DCLFileError
	if errors.As(err, &file) {
		causes := []uint32{file.Status}
		if file.STV != 0 && file.STV != file.Status {
			causes = append(causes, file.STV)
		}

		return submitMessage(submitOpenIn, "SUBMIT-F-OPENIN, error opening "+file.Name+" as input", causes...)
	}

	if st := corevms.QueueErrorStatus(err); st != 0 {
		return submitMessage(submitCreJob, "SUBMIT-F-CREJOB, error creating job", st)
	}

	return err
}

// EntryOptions are SET ENTRY's qualifiers.
type EntryOptions struct {
	Hold, Release bool

	After   string
	NoAfter bool

	Parameters      []string
	ParametersGiven bool
	Name            string
	Priority        int
	PriorityGiven   bool
	Note            string
	NoteGiven       bool
	LogFile         string
	LogGiven        bool
	CPUTime         string
	CPUGiven        bool
	Requeue         string
}

// entryNumbers reads a list of entry numbers; one that isn't a number is
// the job manager's NOSUCHJOB (unconfirmed).
func entryNumbers(list []string) ([]int, error) {
	var entries []int

	for _, item := range list {
		n, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil {
			return nil, queueCondition(vmsdef.Symbols["JBC$_NOSUCHJOB"])
		}

		entries = append(entries, n)
	}

	return entries, nil
}

// SetEntry is SET ENTRY n[,...]: each job changed (corevms's AlterJob).
// A /PRIORITY above MAXQUEPRI is lowered with JBC$_PRIOSMALL said.
func (c *Context) SetEntry(list []string, o EntryOptions) error {
	if c.RTL == nil {
		return noQueues()
	}

	entries, err := entryNumbers(list)
	if err != nil {
		return err
	}

	change := corevms.JobChange{Hold: o.Hold, Release: o.Release, Requeue: o.Requeue, Priority: -1}

	if o.PriorityGiven {
		change.Priority = o.Priority
	}

	switch {
	case o.NoAfter:
		change.After = &time.Time{}
	case o.After != "":
		t, err := c.jobTime(o.After, "AFTER")
		if err != nil {
			return err
		}

		change.After = &t
	}

	var sets []func(*jobqueue.Job)

	if o.ParametersGiven {
		params, err := jobParameters(o.Parameters)
		if err != nil {
			return err
		}

		sets = append(sets, func(j *jobqueue.Job) { j.Parameters = params })
	}

	if o.Name != "" {
		name := strings.ToUpper(o.Name)
		sets = append(sets, func(j *jobqueue.Job) { j.Name = name })
	}

	if o.NoteGiven {
		note := o.Note
		sets = append(sets, func(j *jobqueue.Job) { j.Note = note })
	}

	if o.LogGiven {
		spec, err := c.logSpec(o.LogFile)
		if err != nil {
			return err
		}

		sets = append(sets, func(j *jobqueue.Job) { j.LogFile, j.NoLog = spec, false })
	}

	if o.CPUGiven {
		t, err := jobCPUTime(o.CPUTime)
		if err != nil {
			return err
		}

		sets = append(sets, func(j *jobqueue.Job) { j.CPUTime = t })
	}

	if len(sets) > 0 {
		change.Set = func(j *jobqueue.Job) {
			for _, s := range sets {
				s(j)
			}
		}
	}

	for _, entry := range entries {
		lowered, err := c.RTL.AlterJob(entry, "", change)
		if err != nil {
			return queueFailure(err)
		}

		if lowered {
			c.Printf("%%%s\n", vmsdef.FormatMessage(vmsdef.Symbols["JBC$_PRIOSMALL"]))
		}
	}

	c.setStatus(jbcNormalStatus)

	return nil
}

// DeleteEntry is DELETE/ENTRY=(n,...) [queue]: each job deleted, in queue
// if it is given. A refusal is DELETE's NOTDELETED naming the entry, with
// the reason under it, and the others are still deleted (unconfirmed:
// the probe's lists all succeeded).
func (c *Context) DeleteEntry(list []string, queue string) error {
	if c.RTL == nil {
		return noQueues()
	}

	var failure error

	for _, item := range list {
		item = strings.TrimSpace(item)

		entry, convErr := strconv.Atoi(item)

		st := vmsdef.Symbols["JBC$_NOSUCHJOB"]

		if convErr == nil {
			err := c.RTL.DeleteJob(entry, strings.TrimSpace(queue))
			if err == nil {
				continue
			}

			if st = corevms.QueueErrorStatus(err); st == 0 {
				return err
			}
		}

		failure = vmserrors.ConditionError{
			Status: deleteNotDeleted | stsInhibitMsg,
			Text:   "DELETE-E-NOTDELETED, error deleting " + item + "\n" + ConditionLine(st),
		}
	}

	if failure != nil {
		return failure
	}

	c.setStatus(jbcNormalStatus)

	return nil
}

// ShowEntryOptions are SHOW ENTRY's qualifiers.
type ShowEntryOptions struct {
	Full bool
	User string
}

// The headings of SHOW ENTRY's and SHOW QUEUE's job lists.
const (
	entryHeading = "  Entry  Jobname         Username     Blocks  Status\n" +
		"  -----  -------         --------     ------  ------\n"
	queueJobHeading = "  Entry  Jobname         Username             Status\n" +
		"  -----  -------         --------             ------\n"
)

// ShowEntry is SHOW ENTRY [n,...]: the jobs named, or the user's, each
// with its queue, and with /FULL its options and files, a blank line
// between them. An entry that isn't there is a blank line and NOSUCHENT,
// as VMS 7.1 wrote it.
func (c *Context) ShowEntry(list []string, o ShowEntryOptions) error {
	if c.RTL == nil {
		return noQueues()
	}

	var entries []int

	for _, item := range list {
		n, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil {
			continue
		}

		entries = append(entries, n)
	}

	if len(list) > 0 && len(entries) == 0 {
		entries = []int{0}
	}

	views, err := c.RTL.EntryViews(entries, o.User)
	if err != nil {
		if corevms.QueueErrorStatus(err) == vmsdef.Symbols["JBC$_NOSUCHENT"] {
			c.Printf("\n")
		}

		return queueFailure(err)
	}

	if len(views) > 0 {
		c.Printf("%s", entryHeading)
	}

	for i, v := range views {
		if i > 0 {
			c.Printf("\n")
		}

		c.showJob(v, o.Full, true)
	}

	c.setStatus(ssNormal | stsInhibitMsg)

	return nil
}

// jobStatusText is a job's status as SHOW ENTRY and SHOW QUEUE word it;
// full adds why a pending job waits on a stopped queue.
func jobStatusText(v corevms.JobView, full bool) string {
	j := &v.Job

	switch j.Status {
	case jobqueue.JobHolding:
		return "Holding"
	case jobqueue.JobTimed:
		return "Holding until " + corevms.QueueTime(j.After, false)
	case jobqueue.JobExecuting:
		return "Executing"
	case jobqueue.JobRetained:
		if j.RetainedOnError() {
			return "Retained on error"
		}

		return "Retained on completion"
	}

	if full && v.Stopped {
		return "Pending (queue stopped)"
	}

	return "Pending"
}

// showJob writes one job's lines: its row, and if the process may see it,
// a retained job's condition, its queue (SHOW ENTRY's, entry), and with
// full its options, files, and completion. A job it may not see is a row
// of "no privilege" and nothing more.
func (c *Context) showJob(v corevms.JobView, full, entry bool) {
	j := &v.Job

	if !v.Visible {
		c.Printf("%7d  %-37s%s\n", j.Entry, "no privilege", jobStatusText(v, full))

		return
	}

	c.Printf("%7d  %-15s %-20s %s\n", j.Entry, j.Name, j.User, jobStatusText(v, full))

	if j.Status == jobqueue.JobRetained && j.RetainedOnError() {
		c.Printf("       %s\n", vmsdef.FormatMessage(j.Condition))
	}

	if entry {
		c.Printf("         On %s batch queue %s\n", v.QueueState, j.Queue)
	}

	if !full {
		return
	}

	c.writeWrapped("         Submitted "+corevms.QueueTime(j.Submitted, false), jobOptions(j))

	for i, f := range j.Files {
		line := "         File: " + f.Spec
		if j.Delete && i == len(j.Files)-1 {
			line += " /DELETE"
		}

		c.Printf("%s\n", line)
	}

	if j.Status == jobqueue.JobRetained {
		c.Printf("         Completed %s on queue %s\n", corevms.QueueTime(j.Completed, false), j.CompletedQueue)
	}
}

// writeWrapped writes head and items, a blank between each, wrapped at
// showQueueWidth with a blank left at a wrapped line's end and the next
// line indented nine, as VMS 7.1 wrapped SHOW ENTRY/FULL's options (the
// probe's case 140).
func (c *Context) writeWrapped(head string, items []string) {
	line := head

	for _, item := range items {
		if len(line)+1+len(item) > showQueueWidth {
			c.Printf("%s \n", line)
			line = "         " + item

			continue
		}

		line += " " + item
	}

	c.Printf("%s\n", line)
}

// jobOptions are a job's options as SHOW ENTRY/FULL lists them after its
// submission time, in order of name as VMS 7.1 listed them: /CPU, /KEEP,
// /LOG (or /NOLOG), /NOTE, /NOTIFY, /PARAM, /NOPRINT, /PRIORITY,
// /RESTART, /RETAIN, and the working-set values.
func jobOptions(j *jobqueue.Job) []string {
	var items []string

	add := func(format string, args ...any) { items = append(items, fmt.Sprintf(format, args...)) }

	if j.CPUTime != 0 {
		add("/CPU=%s", j.CPUTime)
	}

	if j.Keep && !j.NoKeep {
		add("/KEEP")
	}

	switch {
	case j.NoLog:
		add("/NOLOG")
	case j.LogFile != "":
		add("/LOG=%s", j.LogFile)
	}

	if j.Note != "" {
		add("/NOTE=%q", j.Note)
	}

	if j.Notify {
		add("/NOTIFY")
	}

	if len(j.Parameters) > 0 {
		quoted := make([]string, len(j.Parameters))
		for i, p := range j.Parameters {
			quoted[i] = `"` + p + `"`
		}

		add("/PARAM=(%s)", strings.Join(quoted, ","))
	}

	if j.NoPrinter {
		add("/NOPRINT")
	}

	add("/PRIORITY=%d", j.Priority)

	if j.Restart {
		add("/RESTART=%s", j.Queue)
	}

	switch j.Retain {
	case jobqueue.RetainAlways:
		add("/RETAIN=ALWAYS")
	case jobqueue.RetainError:
		add("/RETAIN=ERROR")
	case jobqueue.RetainUntil:
		add("/RETAIN=UNTIL=%q", corevms.QueueTime(j.RetainUntil, true))
	}

	if j.WSDefault != 0 {
		add("/WSDEFAULT=%d", j.WSDefault)
	}

	if j.WSExtent != 0 {
		add("/WSEXTENT=%d", j.WSExtent)
	}

	if j.WSQuota != 0 {
		add("/WSQUOTA=%d", j.WSQuota)
	}

	return items
}

// jobStatusNames are /BY_JOB_STATUS's keywords and the statuses each
// selects.
var jobStatusNames = map[string]jobqueue.JobStatus{
	"EXECUTING":     jobqueue.JobExecuting,
	"HOLDING":       jobqueue.JobHolding,
	"PENDING":       jobqueue.JobPending,
	"RETAINED":      jobqueue.JobRetained,
	"TIMED_RELEASE": jobqueue.JobTimed,
}

// showQueueJobs writes a queue's jobs under SHOW QUEUE: without /ALL_JOBS
// only the process's own, a run of others between two of them counted
// ("(2 intervening jobs)", VMS 7.1's case 107); with it every job, one the
// process may not read as "no privilege". /BY_JOB_STATUS keeps the
// statuses it names. With /FULL each job has its options and files, a
// blank line between jobs. No job shown is no heading.
func (c *Context) showQueueJobs(jobs []corevms.JobView, opts ShowQueueOptions) {
	var shown []corevms.JobView

	for _, v := range jobs {
		if opts.ByStatus != nil && !opts.ByStatus[v.Job.Status] {
			continue
		}

		shown = append(shown, v)
	}

	type row struct {
		job     corevms.JobView
		hidden  int
		isCount bool
	}

	var rows []row

	pending := 0

	for _, v := range shown {
		if !opts.All && !v.Own {
			pending++

			continue
		}

		if pending > 0 && len(rows) > 0 {
			rows = append(rows, row{hidden: pending, isCount: true})
		}

		pending = 0

		rows = append(rows, row{job: v})
	}

	if len(rows) == 0 {
		return
	}

	c.Printf("\n%s", queueJobHeading)

	for i, r := range rows {
		if r.isCount {
			c.Printf("         (%d intervening jobs)\n", r.hidden)

			continue
		}

		if opts.Full && i > 0 {
			c.Printf("\n")
		}

		c.showJob(r.job, opts.Full, false)
	}
}

// JobStatusSelection reads /BY_JOB_STATUS's keywords, each an
// abbreviation of one of jobStatusNames'; another is IVKEYW.
func JobStatusSelection(list []string) (map[jobqueue.JobStatus]bool, error) {
	by := map[jobqueue.JobStatus]bool{}

	for _, item := range list {
		word := strings.ToUpper(strings.TrimSpace(item))
		found := false

		for name, status := range jobStatusNames {
			if word != "" && strings.HasPrefix(name, word) {
				by[status], found = true, true
			}
		}

		if !found {
			return nil, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, word)
		}
	}

	return by, nil
}
