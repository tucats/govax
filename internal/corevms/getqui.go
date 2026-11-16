package corevms

import (
	"slices"
	"strings"
	"time"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// $GETQUI and $GETQUIW (docs/planning/PHASE-98 - batch queues.md,
// subtask 9, Decision 9): information about the queue manager, the
// queues, the jobs in them, and the jobs' files, by function code and
// item list, as the System Services manual and VSI's pages describe it
// and VMS 7.1 answered it (testdata/bat98's BAT98Q, A.4):
//
//	SYS$GETQUI[W] [efn] ,func ,[context] ,[itmlst] ,[iosb] ,[astadr] ,[astprm]
//
// Like $SNDJBC, it completes at once: the event flag cleared and set, the
// IOSB the status, an AST queued; R0 SS$_NORMAL but for an unknown function
// or item code (SS$_BADPARAM in R0 and the IOSB alike, round 2's case 21).
// The context argument is VMS 5.0's nullarg, and is ignored: each access
// mode of a process has one context (quiContext), so an image's and DCL's
// F$GETQUI's (supervisor mode) don't disturb each other.
//
// A request names what it asks about with its input items (SEARCH_NAME,
// SEARCH_NUMBER, SEARCH_USERNAME, SEARCH_JOB_NAME, SEARCH_FLAGS) and gets
// back the output items it lists. The functions (quiFunctions):
//
//   - DISPLAY_QUEUE: a queue by name, or with a wildcard (or the WILDCARD
//     flag) each matching queue in turn, in order of name, then
//     JBC$_NOMOREQUE, after which the next call starts again (round 2's
//     cases 27 to 30). The queue found is the context DISPLAY_JOB walks.
//     THIS_JOB is the caller's own batch job's queue.
//   - DISPLAY_JOB: the queue context's jobs in turn, in SHOW QUEUE's
//     order, the caller's own unless ALL_JOBS, then JBC$_NOMOREJOB, after
//     which the context has no jobs (JBC$_NOSUCHJOB, cases 33 to 36), and
//     JBC$_NOQUECTX with no queue context. THIS_JOB is the caller's job.
//   - DISPLAY_ENTRY: a job by entry number (no context kept: DISPLAY_FILE
//     after one is JBC$_NOJOBCTX, case 8), or with no number or the
//     WILDCARD flag each of a user's jobs in turn, by entry number, then
//     JBC$_NOMOREENT.
//   - DISPLAY_FILE: the job context's files in turn, then
//     JBC$_NOMOREFILE; THIS_JOB is the file the caller's batch job runs.
//   - DISPLAY_MANAGER: the queue manager, SYS$QUEUE_MANAGER.
//   - TRANSLATE_QUEUE: a queue's name through logical names.
//   - CANCEL_OPERATION: the context forgotten.
//
// FREEZE_CONTEXT returns the object a wildcard call last found again,
// without moving on; the job state flags (EXECUTING_JOBS, HOLDING_JOBS,
// PENDING_JOBS, RETAINED_JOBS, TIMED_RELEASE_JOBS) select jobs; and the
// queue kind flags (BATCH, and PRINTER, SERVER, TERMINAL, SYMBIONT, which
// govax's batch queues never are, Decision 1) select queues.
//
// Each output item has its size (quiItems: a byte for PRIORITY, JOB_LIMIT,
// and BASE_PRIORITY, a word for PROTECTION, a quadword for a time, a
// string its length), truncated to the buffer. An item the object has no
// value for (a form's, a printer's, a queue's asked of a file) is returned
// with length 0, as VMS 7.1 returned LOG_QUEUE (case 7). Anyone may see a
// queue's attributes; a job without read access is JOB_INACCESSIBLE, and
// only the DCL Dictionary's few items are returned for it; a queue's owner
// and protection without QueueSecurityVisible are 0 and no access (the
// probe's matrix: F$GETQUI's "[0,0]"). DISPLAY_CHARACTERISTIC,
// DISPLAY_FORM, and DISPLAY_QMAN are JBC$_NOTSUPPORTED (Decision 1).

// QueueNormal is a $GETQUI request's status when it succeeds,
// JBC$_NORMAL.
var QueueNormal = jbcNormal

// QueueQuery is one $GETQUI request's input items (and F$GETQUI's
// arguments).
type QueueQuery struct {
	// Function is the QUI$_ function code.
	Function uint16

	// Name is SEARCH_NAME (a queue's or the manager's name, with
	// wildcards; HasName says it was given), Number SEARCH_NUMBER (an
	// entry; HasNumber), User SEARCH_USERNAME, JobName SEARCH_JOB_NAME,
	// and Flags SEARCH_FLAGS (QUI$M_SEARCH_ bits).
	Name      string
	HasName   bool
	Number    int
	HasNumber bool
	User      string
	JobName   string
	Flags     uint32
}

// QueueInfo is what a $GETQUI request found: each output item's value, by
// item code. An item it has no value for has none.
type QueueInfo struct {
	values map[uint16]itemValue
}

// Item is the value of the output item with code, as $GETQUI returns it
// (a number low byte first), and whether there is one.
func (i QueueInfo) Item(code uint16) (string, bool) {
	v, ok := i.values[code]

	return v.data, ok
}

// quiContext is one access mode's $GETQUI context: the queue a wildcard
// search last found (or the queue a DISPLAY_QUEUE found, for DISPLAY_JOB),
// the jobs a DISPLAY_JOB or DISPLAY_ENTRY walk has returned, and the job
// whose files DISPLAY_FILE walks.
type quiContext struct {
	// The queue context: pattern is the wildcard search's, "" for none;
	// queue the queue found last ("" for no queue context).
	pattern string
	queue   string

	// The job context: seen are the entries DISPLAY_JOB has returned in
	// the queue context, job the entry it (or a wildcard DISPLAY_ENTRY)
	// found last, 0 for none; jobsDone says the walk ended (NOMOREJOB).
	seen     []int
	job      int
	jobsDone bool

	// entryWalk says a wildcard DISPLAY_ENTRY walk is in progress;
	// lastEntry is its last entry.
	entryWalk bool
	lastEntry int

	// file is how many of the job context's files DISPLAY_FILE has
	// returned; filesDone says the walk ended.
	file      int
	filesDone bool

	// manager says a wildcard DISPLAY_MANAGER returned the manager.
	manager bool
}

// quiCode is a $QUIDEF code's value.
func quiCode(name string) uint16 {
	v, ok := vmsdef.Symbols["QUI$_"+name]
	if !ok {
		panic("corevms: no $QUIDEF code QUI$_" + name)
	}

	return uint16(v)
}

// quiMask is a $QUIDEF mask's value.
func quiMask(name string) uint32 {
	v, ok := vmsdef.Symbols["QUI$M_"+name]
	if !ok {
		panic("corevms: no $QUIDEF mask QUI$M_" + name)
	}

	return v
}

// The queue manager's statuses $GETQUI returns.
var (
	jbcNoMoreQue   = vmsdef.Symbols["JBC$_NOMOREQUE"]
	jbcNoMoreJob   = vmsdef.Symbols["JBC$_NOMOREJOB"]
	jbcNoMoreEnt   = vmsdef.Symbols["JBC$_NOMOREENT"]
	jbcNoMoreFile  = vmsdef.Symbols["JBC$_NOMOREFILE"]
	jbcNoMoreQmgr  = vmsdef.Symbols["JBC$_NOMOREQMGR"]
	jbcNoSuchFile  = vmsdef.Symbols["JBC$_NOSUCHFILE"]
	jbcNoSuchQmgr  = vmsdef.Symbols["JBC$_NOSUCHQMGR"]
	jbcNoQueCtx    = vmsdef.Symbols["JBC$_NOQUECTX"]
	jbcNoJobCtx    = vmsdef.Symbols["JBC$_NOJOBCTX"]
	quiMinFunc     = uint16(vmsdef.Symbols["QUI$K_MIN_FUNC"])
	quiMaxFunc     = uint16(vmsdef.Symbols["QUI$K_MAX_FUNC"])
	quiMaxItem     = uint16(vmsdef.Symbols["QUI$K_MAX_ITEM"])
	queueManagerID = "SYS$QUEUE_MANAGER"
)

// The search flags.
var (
	quiWildcard = quiMask("SEARCH_WILDCARD")
	quiAllJobs  = quiMask("SEARCH_ALL_JOBS")
	quiThisJob  = quiMask("SEARCH_THIS_JOB")
	quiFreeze   = quiMask("SEARCH_FREEZE_CONTEXT")
	quiBatch    = quiMask("SEARCH_BATCH")
	quiOutput   = quiMask("SEARCH_PRINTER") | quiMask("SEARCH_SERVER") | quiMask("SEARCH_TERMINAL") |
		quiMask("SEARCH_SYMBIONT") | quiMask("SEARCH_GENERIC")

	// quiJobStates are the flags that select jobs by state, each with the
	// state it selects.
	quiJobStates = map[uint32]jobqueue.JobStatus{
		quiMask("SEARCH_EXECUTING_JOBS"):     jobqueue.JobExecuting,
		quiMask("SEARCH_HOLDING_JOBS"):       jobqueue.JobHolding,
		quiMask("SEARCH_PENDING_JOBS"):       jobqueue.JobPending,
		quiMask("SEARCH_RETAINED_JOBS"):      jobqueue.JobRetained,
		quiMask("SEARCH_TIMED_RELEASE_JOBS"): jobqueue.JobTimed,
	}
)

// quiObject is the object a request found, and what its items need: the
// database, and the queue, job (with its queue), or file (with its job).
type quiObject struct {
	env   *Environment
	d     *jobqueue.Database
	queue *jobqueue.Queue
	job   *jobqueue.Job
	file  int // the file's index in job.Files, -1 for no file

	// intervening is how many jobs a DISPLAY_JOB passed over to find job.
	intervening int
}

// quiFunction is one $GETQUI function: it finds the object the request
// asks about, in the context ctx, or returns the status to fail with.
type quiFunction func(env *Environment, ctx *quiContext, d *jobqueue.Database, q QueueQuery) (quiObject, uint32)

// quiFunctions are the functions govax has, by code; CANCEL_OPERATION and
// DISPLAY_MANAGER are QueueInformation's own.
var quiFunctions = map[uint16]quiFunction{
	quiCode("DISPLAY_QUEUE"):   quiDisplayQueue,
	quiCode("DISPLAY_JOB"):     quiDisplayJob,
	quiCode("DISPLAY_ENTRY"):   quiDisplayEntry,
	quiCode("DISPLAY_FILE"):    quiDisplayFile,
	quiCode("TRANSLATE_QUEUE"): quiTranslateQueue,
}

// quiContextFor is the $GETQUI context of access mode mode.
func (env *Environment) quiContextFor(mode registers.AccessMode) *quiContext {
	if env.quiContexts[mode&3] == nil {
		env.quiContexts[mode&3] = &quiContext{}
	}

	return env.quiContexts[mode&3]
}

// QueueInformation is $GETQUI's work for DCL's F$GETQUI: q done in
// supervisor mode's context. It returns what was found, or the status:
// JBC$_NORMAL, or the refusal (JBC$_NOSUCHQUE, JBC$_NOMOREQUE, ...).
func (env *Environment) QueueInformation(q QueueQuery) (QueueInfo, uint32) {
	return env.queueInformation(env.quiContextFor(dclMode), q)
}

// queueInformation does request q in context ctx.
func (env *Environment) queueInformation(ctx *quiContext, q QueueQuery) (QueueInfo, uint32) {
	switch q.Function {
	case quiCode("CANCEL_OPERATION"):
		*ctx = quiContext{}

		return QueueInfo{}, jbcNormal
	case quiCode("DISPLAY_MANAGER"):
		return env.quiDisplayManager(ctx, q)
	}

	fn, ok := quiFunctions[q.Function]
	if !ok {
		return QueueInfo{}, jbcNotSupported
	}

	d, err := env.readJobs()
	if err != nil {
		if st := QueueErrorStatus(err); st != 0 {
			return QueueInfo{}, st
		}

		return QueueInfo{}, ssAbort
	}

	obj, st := fn(env, ctx, d, q)
	if st != 0 {
		return QueueInfo{}, st
	}

	obj.env, obj.d = env, d

	return obj.info(), jbcNormal
}

// thisJob is the caller's own batch job, if it is one and the database has
// it: JBC$_NOSUCHJOB otherwise (unconfirmed for a process that isn't a
// batch job: the probe asked only from one).
func (env *Environment) thisJob(d *jobqueue.Database) (*jobqueue.Job, uint32) {
	if env.batch == nil {
		return nil, jbcNoSuchJob
	}

	j := d.Job(env.batch.entry)
	if j == nil {
		return nil, jbcNoSuchJob
	}

	return j, 0
}

// isWildcard reports whether a name has VMS's wildcards in it.
func isWildcard(name string) bool { return strings.ContainsAny(name, "*%") }

// queueSelected reports whether a queue is one the request's kind flags
// select: a batch queue, unless only output queues are asked for.
func queueSelected(flags uint32) bool {
	return flags&quiOutput == 0 || flags&quiBatch != 0
}

// quiDisplayQueue is DISPLAY_QUEUE.
func quiDisplayQueue(env *Environment, ctx *quiContext, d *jobqueue.Database, q QueueQuery) (quiObject, uint32) {
	if q.Flags&quiThisJob != 0 {
		j, st := env.thisJob(d)
		if st != 0 {
			return quiObject{}, st
		}

		ctx.enterQueue(j.Queue, "")

		return quiObject{queue: d.Queue(j.Queue), file: -1}, 0
	}

	name := strings.ToUpper(queueName(q.Name))

	if q.Flags&quiWildcard == 0 && !isWildcard(name) {
		if !q.HasName || name == "" {
			return quiObject{}, jbcMisReqPar
		}

		queue := d.Queue(name)
		if queue == nil || !queueSelected(q.Flags) {
			return quiObject{}, jbcNoSuchQue
		}

		ctx.enterQueue(queue.Name, "")

		return quiObject{queue: queue, file: -1}, 0
	}

	// A wildcard search: a new one when the pattern changes (or none is
	// in progress), the queue it found last again with FREEZE_CONTEXT.
	if name == "" {
		name = ctx.pattern
	}

	if name == "" {
		return quiObject{}, jbcMisReqPar
	}

	if name != ctx.pattern {
		*ctx = quiContext{pattern: name}
	}

	if q.Flags&quiFreeze != 0 && ctx.queue != "" {
		if queue := d.Queue(ctx.queue); queue != nil {
			return quiObject{queue: queue, file: -1}, 0
		}
	}

	for _, queue := range d.Queues {
		if strings.ToUpper(queue.Name) > strings.ToUpper(ctx.queue) &&
			lnm.Match(name, strings.ToUpper(queue.Name)) && queueSelected(q.Flags) {
			ctx.enterQueue(queue.Name, name)

			return quiObject{queue: queue, file: -1}, 0
		}
	}

	*ctx = quiContext{}

	return quiObject{}, jbcNoMoreQue
}

// enterQueue makes queue the context's queue, found by a search of
// pattern ("" for none): a new job context in it.
func (ctx *quiContext) enterQueue(queue, pattern string) {
	*ctx = quiContext{pattern: pattern, queue: queue}
}

// jobSelected reports whether job j is one a DISPLAY_JOB or DISPLAY_ENTRY
// request selects: by its state flags, and its user unless ALL_JOBS.
func (env *Environment) jobSelected(j *jobqueue.Job, q QueueQuery, user string) bool {
	if q.Flags&quiAllJobs == 0 && !strings.EqualFold(j.User, user) {
		return false
	}

	if q.JobName != "" && !lnm.Match(strings.ToUpper(q.JobName), strings.ToUpper(j.Name)) {
		return false
	}

	states := false

	for mask, status := range quiJobStates {
		if q.Flags&mask != 0 {
			states = true

			if j.Status == status {
				return true
			}
		}
	}

	return !states
}

// quiDisplayJob is DISPLAY_JOB.
func quiDisplayJob(env *Environment, ctx *quiContext, d *jobqueue.Database, q QueueQuery) (quiObject, uint32) {
	if q.Flags&quiThisJob != 0 {
		j, st := env.thisJob(d)
		if st != 0 {
			return quiObject{}, st
		}

		ctx.job, ctx.file, ctx.filesDone = j.Entry, 0, false

		return quiObject{queue: d.Queue(j.Queue), job: j, file: -1}, 0
	}

	if ctx.queue == "" {
		return quiObject{}, jbcNoQueCtx
	}

	queue := d.Queue(ctx.queue)
	if queue == nil || ctx.jobsDone {
		return quiObject{}, jbcNoSuchJob
	}

	if q.Flags&quiFreeze != 0 && ctx.job != 0 {
		if j := d.Job(ctx.job); j != nil {
			return quiObject{queue: queue, job: j, file: -1}, 0
		}
	}

	intervening := 0

	for _, j := range d.Order(queue.Name) {
		if slices.Contains(ctx.seen, j.Entry) {
			continue
		}

		if !env.jobSelected(j, q, env.Process.Username) {
			if j.Status == jobqueue.JobPending {
				intervening++
			}

			continue
		}

		ctx.seen = append(ctx.seen, j.Entry)
		ctx.job, ctx.file, ctx.filesDone = j.Entry, 0, false

		return quiObject{queue: queue, job: j, file: -1, intervening: intervening}, 0
	}

	ctx.job, ctx.jobsDone = 0, true

	return quiObject{}, jbcNoMoreJob
}

// quiDisplayEntry is DISPLAY_ENTRY.
func quiDisplayEntry(env *Environment, ctx *quiContext, d *jobqueue.Database, q QueueQuery) (quiObject, uint32) {
	queues := strings.ToUpper(queueName(q.Name))

	inQueue := func(j *jobqueue.Job) bool {
		return queues == "" || lnm.Match(queues, strings.ToUpper(j.Queue))
	}

	if q.HasNumber && q.Flags&quiWildcard == 0 {
		j := d.Job(q.Number)
		if j == nil || !inQueue(j) {
			return quiObject{}, jbcNoSuchEnt
		}

		return quiObject{queue: d.Queue(j.Queue), job: j, file: -1}, 0
	}

	if q.Flags&quiFreeze != 0 && ctx.entryWalk && ctx.job != 0 {
		if j := d.Job(ctx.job); j != nil {
			return quiObject{queue: d.Queue(j.Queue), job: j, file: -1}, 0
		}
	}

	if !ctx.entryWalk {
		*ctx = quiContext{entryWalk: true}
	}

	user := env.Process.Username
	if q.User != "" {
		user = q.User
	}

	// A user's jobs: SEARCH_USERNAME's, or the caller's own; ALL_JOBS
	// isn't a DISPLAY_ENTRY flag, so it is taken out.
	sel := q
	sel.Flags &^= quiAllJobs

	for _, j := range d.Jobs {
		if j.Entry <= ctx.lastEntry || !inQueue(j) || !env.jobSelected(j, sel, user) {
			continue
		}

		if q.HasNumber && j.Entry != q.Number {
			continue
		}

		ctx.lastEntry, ctx.job, ctx.file, ctx.filesDone = j.Entry, j.Entry, 0, false

		return quiObject{queue: d.Queue(j.Queue), job: j, file: -1}, 0
	}

	*ctx = quiContext{}

	return quiObject{}, jbcNoMoreEnt
}

// quiDisplayFile is DISPLAY_FILE.
func quiDisplayFile(env *Environment, ctx *quiContext, d *jobqueue.Database, q QueueQuery) (quiObject, uint32) {
	if q.Flags&quiThisJob != 0 {
		j, st := env.thisJob(d)
		if st != 0 {
			return quiObject{}, st
		}

		return quiObject{queue: d.Queue(j.Queue), job: j, file: min(env.batch.file, len(j.Files)-1)}, 0
	}

	if ctx.job == 0 {
		return quiObject{}, jbcNoJobCtx
	}

	j := d.Job(ctx.job)
	if j == nil || ctx.filesDone {
		return quiObject{}, jbcNoSuchFile
	}

	if q.Flags&quiFreeze != 0 && ctx.file > 0 {
		return quiObject{queue: d.Queue(j.Queue), job: j, file: ctx.file - 1}, 0
	}

	if ctx.file >= len(j.Files) {
		ctx.filesDone = true

		return quiObject{}, jbcNoMoreFile
	}

	ctx.file++

	return quiObject{queue: d.Queue(j.Queue), job: j, file: ctx.file - 1}, 0
}

// quiTranslateQueue is TRANSLATE_QUEUE: the name through the process's
// logical names (LNM$FILE_DEV, as a device's), to the queue it names.
// A name that names no queue is JBC$_NOSUCHQUE (unconfirmed: the probe
// translated only SYS$BATCH, a queue's own name, case 41).
func quiTranslateQueue(env *Environment, _ *quiContext, d *jobqueue.Database, q QueueQuery) (quiObject, uint32) {
	name := strings.ToUpper(queueName(q.Name))
	if !q.HasName || name == "" {
		return quiObject{}, jbcMisReqPar
	}

	for range lnm.MaxDepth {
		e, err := env.TranslateName("LNM$FILE_DEV", name, lnm.User, 0)
		if err != nil || e == nil || len(e.Equivalences) == 0 {
			break
		}

		name = strings.ToUpper(queueName(e.Equivalences[0].Value))
	}

	queue := d.Queue(name)
	if queue == nil {
		return quiObject{}, jbcNoSuchQue
	}

	return quiObject{queue: queue, file: -1}, 0
}

// quiDisplayManager is DISPLAY_MANAGER: the one queue manager, by name
// (wildcards allowed; a wildcard search gives it once, then
// JBC$_NOMOREQMGR). It answers with the manager stopped too, as its
// status says.
func (env *Environment) quiDisplayManager(ctx *quiContext, q QueueQuery) (QueueInfo, uint32) {
	name := strings.ToUpper(strings.TrimSpace(q.Name))
	if !q.HasName || name == "" {
		return QueueInfo{}, jbcMisReqPar
	}

	wild := q.Flags&quiWildcard != 0 || isWildcard(name)

	if !lnm.Match(name, queueManagerID) {
		if wild {
			return QueueInfo{}, jbcNoMoreQmgr
		}

		return QueueInfo{}, jbcNoSuchQmgr
	}

	if wild {
		if ctx.manager && q.Flags&quiFreeze == 0 {
			ctx.manager = false

			return QueueInfo{}, jbcNoMoreQmgr
		}

		ctx.manager = true
	}

	info, err := env.QueueManagerStatus()
	if err != nil {
		return QueueInfo{}, ssAbort
	}

	status := quiMask("MANAGER_STOPPED")
	if info.Running {
		status = quiMask("MANAGER_RUNNING")
	}

	return QueueInfo{values: map[uint16]itemValue{
		quiCode("MANAGER_NAME"):    itemString(queueManagerID),
		quiCode("MANAGER_STATUS"):  itemLong(status),
		quiCode("MANAGER_NODES"):   itemString(env.NodeName),
		quiCode("QUEUE_DIRECTORY"): itemString(info.Directory),
		quiCode("SCSNODE_NAME"):    itemString(env.NodeName),
	}}, jbcNormal
}

// info is every item the object has a value for.
func (o quiObject) info() QueueInfo {
	values := map[uint16]itemValue{}

	var table map[uint16]func(quiObject) itemValue

	switch {
	case o.file >= 0:
		table = quiFileItems
	case o.job != nil:
		table = quiJobItems

		if !o.env.JobAccess(o.queue, o.job, security.QueueRead) {
			table = quiInaccessibleItems
		}
	case o.queue != nil:
		table = quiQueueItems
	}

	for code, value := range table {
		values[code] = value(o)
	}

	return QueueInfo{values: values}
}

// cpuTicks is a CPU time limit as $GETQUI returns it, in 10-millisecond
// units, no limit (INFINITE) being 0.
func cpuTicks(t jobqueue.CPUTime) itemValue {
	if t == jobqueue.CPUInfinite {
		return itemLong(0)
	}

	return itemLong(uint32(t))
}

// queueStatus is a queue's QUEUE_STATUS: its state's bits (the bits for
// idle, available, and busy are unconfirmed: the probe's $GETQUI asked only
// about stopped queues, QUI$M_QUEUE_STOPPED, case 4).
func (o quiObject) queueStatus() uint32 {
	q, running := o.queue, o.d.Executing(o.queue.Name)

	switch q.State {
	case jobqueue.QueueStopped:
		return quiMask("QUEUE_STOPPED")
	case jobqueue.QueuePaused:
		return quiMask("QUEUE_PAUSED")
	}

	var st uint32

	switch {
	case running == 0:
		st = quiMask("QUEUE_IDLE")
	case running >= q.JobLimit:
		st = quiMask("QUEUE_BUSY")
	default:
		st = quiMask("QUEUE_AVAILABLE")
	}

	if q.State == jobqueue.QueueStopping {
		st |= quiMask("QUEUE_STOP_PENDING")
	}

	return st
}

// queueFlags is a queue's QUEUE_FLAGS: a batch queue's, which VMS 7.1 gave
// GENERIC_SELECTION and SWAP by default (case 4's %X000A0401, with
// WSDEFAULT), and a bit for each attribute it has.
func (o quiObject) queueFlags() uint32 {
	q := o.queue
	flags := quiMask("QUEUE_BATCH") | quiMask("QUEUE_GENERIC_SELECTION") | quiMask("QUEUE_SWAP")

	for _, f := range []struct {
		set  bool
		name string
	}{
		{q.CPUDefault != 0, "QUEUE_CPU_DEFAULT"},
		{q.CPUMaximum != 0, "QUEUE_CPU_LIMIT"},
		{q.WSDefault != 0, "QUEUE_WSDEFAULT"},
		{q.WSQuota != 0, "QUEUE_WSQUOTA"},
		{q.WSExtent != 0, "QUEUE_WSEXTENT"},
		{q.Retain == jobqueue.RetainAlways, "QUEUE_RETAIN_ALL"},
		{q.Retain == jobqueue.RetainError, "QUEUE_RETAIN_ERROR"},
		{len(q.ACL) > 0, "QUEUE_ACL_SPECIFIED"},
		{!o.env.QueueSecurityVisible(q), "SECURITY_INACCESSIBLE"},
	} {
		if f.set {
			flags |= quiMask(f.name)
		}
	}

	return flags
}

// countJobs is how many of the queue's jobs are in state s.
func (o quiObject) countJobs(s jobqueue.JobStatus) itemValue {
	n := 0

	for _, j := range o.d.QueueJobs(o.queue.Name) {
		if j.Status == s {
			n++
		}
	}

	return itemLong(uint32(n))
}

// quiQueueItems are a queue's items.
var quiQueueItems = map[uint16]func(quiObject) itemValue{
	quiCode("QUEUE_NAME"):        func(o quiObject) itemValue { return itemString(o.queue.Name) },
	quiCode("QUEUE_DESCRIPTION"): func(o quiObject) itemValue { return itemString(o.queue.Description) },
	quiCode("QUEUE_STATUS"):      func(o quiObject) itemValue { return itemLong(o.queueStatus()) },
	quiCode("QUEUE_FLAGS"):       func(o quiObject) itemValue { return itemLong(o.queueFlags()) },
	quiCode("JOB_LIMIT"):         func(o quiObject) itemValue { return itemByte(uint8(o.queue.JobLimit)) },
	quiCode("BASE_PRIORITY"):     func(o quiObject) itemValue { return itemByte(uint8(o.queue.BasePriority)) },
	quiCode("OWNER_UIC"): func(o quiObject) itemValue {
		if !o.env.QueueSecurityVisible(o.queue) {
			return itemLong(0)
		}

		return itemLong(uint32(o.queue.Owner))
	},
	quiCode("PROTECTION"): func(o quiObject) itemValue {
		if !o.env.QueueSecurityVisible(o.queue) {
			return itemWord(0xFFFF)
		}

		return itemWord(uint16(o.queue.Protection))
	},
	quiCode("WSDEFAULT"):                func(o quiObject) itemValue { return itemLong(uint32(o.queue.WSDefault)) },
	quiCode("WSQUOTA"):                  func(o quiObject) itemValue { return itemLong(uint32(o.queue.WSQuota)) },
	quiCode("WSEXTENT"):                 func(o quiObject) itemValue { return itemLong(uint32(o.queue.WSExtent)) },
	quiCode("CPU_DEFAULT"):              func(o quiObject) itemValue { return cpuTicks(o.queue.CPUDefault) },
	quiCode("CPU_LIMIT"):                func(o quiObject) itemValue { return cpuTicks(o.queue.CPUMaximum) },
	quiCode("EXECUTING_JOB_COUNT"):      func(o quiObject) itemValue { return o.countJobs(jobqueue.JobExecuting) },
	quiCode("HOLDING_JOB_COUNT"):        func(o quiObject) itemValue { return o.countJobs(jobqueue.JobHolding) },
	quiCode("PENDING_JOB_COUNT"):        func(o quiObject) itemValue { return o.countJobs(jobqueue.JobPending) },
	quiCode("TIMED_RELEASE_JOB_COUNT"):  func(o quiObject) itemValue { return o.countJobs(jobqueue.JobTimed) },
	quiCode("RETAINED_JOB_COUNT"):       func(o quiObject) itemValue { return o.countJobs(jobqueue.JobRetained) },
	quiCode("PENDING_JOB_BLOCK_COUNT"):  func(quiObject) itemValue { return itemLong(0) },
	quiCode("SCSNODE_NAME"):             func(o quiObject) itemValue { return itemString(o.queue.Node) },
	quiCode("DEVICE_NAME"):              func(quiObject) itemValue { return itemString("") },
	quiCode("ASSIGNED_QUEUE_NAME"):      func(quiObject) itemValue { return itemString("") },
	quiCode("QUEUE_DIRECTORY"):          func(o quiObject) itemValue { return itemString(o.d.Manager.Directory) },
	quiCode("JOB_SIZE_MAXIMUM"):         func(quiObject) itemValue { return itemLong(0) },
	quiCode("JOB_SIZE_MINIMUM"):         func(quiObject) itemValue { return itemLong(0) },
	quiCode("GENERIC_TARGET"):           func(quiObject) itemValue { return itemString("") },
	quiCode("CHECKPOINT_FREQUENCY"):     func(quiObject) itemValue { return itemLong(0) },
	quiCode("INTERVENING_JOBS"):         func(quiObject) itemValue { return itemLong(0) },
	quiCode("INTERVENING_BLOCKS"):       func(quiObject) itemValue { return itemLong(0) },
	quiCode("QUEUE_SUPPORT"):            func(quiObject) itemValue { return itemLong(0) },
	quiCode("DEFAULT_FORM_NAME"):        func(quiObject) itemValue { return itemString("") },
	quiCode("LIBRARY_SPECIFICATION"):    func(quiObject) itemValue { return itemString("") },
	quiCode("PROCESSOR"):                func(quiObject) itemValue { return itemString("") },
	quiCode("FORM_NAME"):                func(quiObject) itemValue { return itemString("") },
}

// jobStatus is a job's JOB_STATUS: its state's bit, SUSPENDED for an
// executing job on a paused queue, ABORTING for one being ended, and
// INACCESSIBLE without read access.
func (o quiObject) jobStatus() uint32 {
	j := o.job

	var st uint32

	switch j.Status {
	case jobqueue.JobPending:
		st = quiMask("JOB_PENDING")
	case jobqueue.JobHolding:
		st = quiMask("JOB_HOLDING")
	case jobqueue.JobTimed:
		st = quiMask("JOB_TIMED_RELEASE")
	case jobqueue.JobExecuting:
		st = quiMask("JOB_EXECUTING")

		if o.queue != nil && o.queue.State == jobqueue.QueuePaused {
			st |= quiMask("JOB_SUSPENDED")
		}

		if j.Ending != 0 || j.Deleting {
			st |= quiMask("JOB_ABORTING")
		}
	case jobqueue.JobRetained:
		st = quiMask("JOB_RETAINED")
	}

	if j.Requeue != nil {
		st |= quiMask("JOB_REQUEUE")
	}

	if !o.env.JobAccess(o.queue, j, security.QueueRead) {
		st |= quiMask("JOB_INACCESSIBLE")
	}

	return st
}

// jobFlags is a job's JOB_FLAGS: a bit for each option it was given.
// VMS 7.1's held job of $SNDJBC with no LOG_SPOOL had none (case 7), and
// a SUBMIT/NOPRINTER/KEEP job none (BAT98J's view).
func (o quiObject) jobFlags() uint32 {
	j := o.job

	var flags uint32

	for _, f := range []struct {
		set  bool
		name string
	}{
		{j.CPUTime != 0, "JOB_CPU_LIMIT"},
		{j.NoKeep, "JOB_LOG_DELETE"},
		{j.NoLog, "JOB_LOG_NULL"},
		{!j.NoPrinter && !j.NoLog, "JOB_LOG_SPOOL"},
		{j.Notify, "JOB_NOTIFY"},
		{j.Restart, "JOB_RESTART"},
		{j.WSDefault != 0, "JOB_WSDEFAULT"},
		{j.WSQuota != 0, "JOB_WSQUOTA"},
		{j.WSExtent != 0, "JOB_WSEXTENT"},
		{j.Retain == jobqueue.RetainError, "JOB_ERROR_RETENTION"},
		{j.Retain == jobqueue.RetainAlways || j.Retain == jobqueue.RetainUntil, "JOB_RETENTION"},
	} {
		if f.set {
			flags |= quiMask(f.name)
		}
	}

	return flags
}

// pendingReason is a pending job's PENDING_JOB_REASON: its queue not
// started, or at its job limit.
func (o quiObject) pendingReason() uint32 {
	if o.job.Status != jobqueue.JobPending || o.queue == nil {
		return 0
	}

	if o.queue.State != jobqueue.QueueStarted {
		return quiMask("PEND_QUEUE_STATE")
	}

	if o.d.Executing(o.queue.Name) >= o.queue.JobLimit {
		return quiMask("PEND_QUEUE_BUSY")
	}

	return 0
}

// jobPID is an executing job's process's PID, 0 for none here.
func (o quiObject) jobPID() uint32 {
	if run := o.env.System.batch[o.job.Entry]; run != nil && run.env != nil && o.job.Status == jobqueue.JobExecuting {
		return run.env.Process.PID
	}

	return 0
}

// jobTime is a time as $GETQUI returns it, a quadword, or nothing (length
// 0) for none.
func jobTime(t time.Time) itemValue {
	if t.IsZero() {
		return itemString("")
	}

	return itemQuad(vmsTime(t))
}

// parameter makes PARAMETER_n's item.
func parameter(n int) func(quiObject) itemValue {
	return func(o quiObject) itemValue {
		if n > len(o.job.Parameters) {
			return itemString("")
		}

		return itemString(o.job.Parameters[n-1])
	}
}

// quiJobItems are a job's items.
var quiJobItems = map[uint16]func(quiObject) itemValue{
	quiCode("ENTRY_NUMBER"): func(o quiObject) itemValue { return itemLong(uint32(o.job.Entry)) },
	quiCode("JOB_NAME"):     func(o quiObject) itemValue { return itemString(o.job.Name) },
	quiCode("JOB_STATUS"):   func(o quiObject) itemValue { return itemLong(o.jobStatus()) },
	quiCode("JOB_FLAGS"):    func(o quiObject) itemValue { return itemLong(o.jobFlags()) },
	quiCode("USERNAME"):     func(o quiObject) itemValue { return itemString(o.job.User) },
	quiCode("ACCOUNT_NAME"): func(o quiObject) itemValue { return itemString(o.job.Account) },
	quiCode("UIC"):          func(o quiObject) itemValue { return itemLong(uint32(o.job.Owner)) },
	// AFTER_TIME is the submission time for a job with no /AFTER (VMS
	// 7.1's held job, case 7, and BAT98J's).
	quiCode("AFTER_TIME"): func(o quiObject) itemValue {
		if o.job.After.IsZero() {
			return itemQuad(vmsTime(o.job.Submitted))
		}

		return itemQuad(vmsTime(o.job.After))
	},
	quiCode("SUBMISSION_TIME"): func(o quiObject) itemValue { return itemQuad(vmsTime(o.job.Submitted)) },
	quiCode("JOB_COMPLETION_TIME"): func(o quiObject) itemValue { return jobTime(o.job.Completed) },
	quiCode("JOB_RETENTION_TIME"):  func(o quiObject) itemValue { return jobTime(o.job.RetainUntil) },
	quiCode("JOB_COMPLETION_QUEUE"): func(o quiObject) itemValue { return itemString(o.job.CompletedQueue) },
	quiCode("NOTE"):                 func(o quiObject) itemValue { return itemString(o.job.Note) },
	quiCode("LOG_SPECIFICATION"):    func(o quiObject) itemValue { return itemString(o.job.LogFile) },
	quiCode("LOG_QUEUE"):            func(quiObject) itemValue { return itemString("") },
	quiCode("PRIORITY"):             func(o quiObject) itemValue { return itemByte(uint8(o.job.Priority)) },
	quiCode("QUEUE_NAME"):           func(o quiObject) itemValue { return itemString(o.job.Queue) },
	quiCode("FILE_COUNT"):           func(o quiObject) itemValue { return itemLong(uint32(len(o.job.Files))) },
	quiCode("CPU_LIMIT"):            func(o quiObject) itemValue { return cpuTicks(o.job.CPUTime) },
	quiCode("PENDING_JOB_REASON"):   func(o quiObject) itemValue { return itemLong(o.pendingReason()) },
	quiCode("CONDITION_VECTOR"): func(o quiObject) itemValue {
		return itemString(littleEndian(uint64(o.job.Condition), 4) + strings.Repeat("\x00", 8))
	},
	quiCode("JOB_PID"):         func(o quiObject) itemValue { return itemLong(o.jobPID()) },
	quiCode("WSDEFAULT"):       func(o quiObject) itemValue { return itemLong(uint32(o.job.WSDefault)) },
	quiCode("WSQUOTA"):         func(o quiObject) itemValue { return itemLong(uint32(o.job.WSQuota)) },
	quiCode("WSEXTENT"):        func(o quiObject) itemValue { return itemLong(uint32(o.job.WSExtent)) },
	quiCode("JOB_SIZE"):        func(quiObject) itemValue { return itemLong(0) },
	quiCode("COMPLETED_BLOCKS"): func(quiObject) itemValue { return itemLong(0) },
	quiCode("CHECKPOINT_DATA"): func(o quiObject) itemValue { return itemString(o.job.RestartValue) },
	quiCode("CLI"):             func(quiObject) itemValue { return itemString("") },
	quiCode("INTERVENING_JOBS"): func(o quiObject) itemValue {
		return itemLong(uint32(o.intervening))
	},
	quiCode("INTERVENING_BLOCKS"): func(quiObject) itemValue { return itemLong(0) },
	quiCode("RESTART_QUEUE_NAME"): func(o quiObject) itemValue {
		if o.job.Restart {
			return itemString(o.job.Queue)
		}

		return itemString("")
	},
	quiCode("REQUEUE_QUEUE_NAME"): func(o quiObject) itemValue {
		if o.job.Requeue != nil {
			return itemString(o.job.Requeue.Queue)
		}

		return itemString("")
	},
}

// quiInaccessibleItems are the items of a job the caller has no read
// access to (the DCL Dictionary's JOB_INACCESSIBLE).
var quiInaccessibleItems = func() map[uint16]func(quiObject) itemValue {
	m := map[uint16]func(quiObject) itemValue{}

	for _, name := range []string{
		"AFTER_TIME", "COMPLETED_BLOCKS", "ENTRY_NUMBER", "INTERVENING_BLOCKS",
		"INTERVENING_JOBS", "JOB_SIZE", "JOB_STATUS",
	} {
		m[quiCode(name)] = quiJobItems[quiCode(name)]
	}

	return m
}()

func init() {
	for n := 1; n <= jobqueue.MaxParameters; n++ {
		quiJobItems[quiCode("PARAMETER_"+string(rune('0'+n)))] = parameter(n)
	}
}

// quiFileItems are a job's file's items: its name as the job has it
// ("_GOVAX$DUA1:[000000]BAT98J.COM;1", as VMS 7.1's "_SIMVAX$DUA1:..."),
// DELETE for /DELETE, EXECUTING for the file the job's process runs.
var quiFileItems = map[uint16]func(quiObject) itemValue{
	quiCode("FILE_SPECIFICATION"): func(o quiObject) itemValue { return itemString(o.job.Files[o.file].Spec) },
	quiCode("FILE_FLAGS"): func(o quiObject) itemValue {
		if o.job.Delete {
			return itemLong(quiMask("FILE_DELETE"))
		}

		return itemLong(0)
	},
	quiCode("FILE_STATUS"): func(o quiObject) itemValue {
		if run := o.env.System.batch[o.job.Entry]; run != nil && run.file == o.file && o.job.Status == jobqueue.JobExecuting {
			return itemLong(quiMask("FILE_EXECUTING"))
		}

		return itemLong(0)
	},
	quiCode("FILE_COPIES"):      func(quiObject) itemValue { return itemLong(1) },
	quiCode("FILE_COPIES_DONE"): func(quiObject) itemValue { return itemLong(0) },
	quiCode("FIRST_PAGE"):       func(quiObject) itemValue { return itemLong(0) },
	quiCode("LAST_PAGE"):        func(quiObject) itemValue { return itemLong(0) },
}

// quiInputs are the input items, each read into the request.
var quiInputs = map[uint16]func(env *Environment, e itemListEntry, q *QueueQuery) uint32{
	quiCode("SEARCH_NAME"): func(env *Environment, e itemListEntry, q *QueueQuery) uint32 {
		data, st := env.itemBytes(e)
		q.Name, q.HasName = string(data), true

		return st
	},
	quiCode("SEARCH_NUMBER"): func(env *Environment, e itemListEntry, q *QueueQuery) uint32 {
		v, st := env.itemNumber(e, 4)
		q.Number, q.HasNumber = signed(uint32(v)), true

		return st
	},
	quiCode("SEARCH_FLAGS"): func(env *Environment, e itemListEntry, q *QueueQuery) uint32 {
		v, st := env.itemNumber(e, 4)
		q.Flags = uint32(v)

		return st
	},
	quiCode("SEARCH_USERNAME"): func(env *Environment, e itemListEntry, q *QueueQuery) uint32 {
		data, st := env.itemBytes(e)
		q.User = strings.TrimSpace(string(data))

		return st
	},
	quiCode("SEARCH_JOB_NAME"): func(env *Environment, e itemListEntry, q *QueueQuery) uint32 {
		data, st := env.itemBytes(e)
		q.JobName = strings.TrimSpace(string(data))

		return st
	},
}

// serviceSysGetqui is SYS$GETQUI and SYS$GETQUIW (wait): every function
// completes at once, so both are the same.
func serviceSysGetqui(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return ssInsfArg, nil
	}

	efn, function, itmlst := argv[0], argv[1], optArg(argv, 3)
	iosb, astadr, astprm := optArg(argv, 4), optArg(argv, 5), optArg(argv, 6)

	flags, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	*flags &^= 1 << bit

	if iosb != 0 && !env.storeQuad(iosb, 0) {
		return ssAccVio, nil
	}

	q := QueueQuery{Function: uint16(function)}

	var outputs []itemListEntry

	status := uint32(0)
	if itmlst != 0 {
		status = env.walkItemList(itmlst, func(e itemListEntry) uint32 {
			if e.ItemCode == 0 || e.ItemCode > quiMaxItem {
				return ssBadParam
			}

			if read, ok := quiInputs[e.ItemCode]; ok {
				return read(env, e, &q)
			}

			outputs = append(outputs, e)

			return 0
		})
	}

	r0 := uint32(ssNormal)

	switch {
	case status != 0:
		r0 = status
	case function < uint32(quiMinFunc) || function > uint32(quiMaxFunc):
		r0, status = ssBadParam, ssBadParam
	default:
		var info QueueInfo

		info, status = env.queueInformation(env.quiContextFor(env.cpu.PSL().CurMod()), q)

		if status == jbcNormal {
			for _, e := range outputs {
				v := info.values[e.ItemCode]
				if st := env.storeItem(e, v); st != 0 {
					return st, nil
				}
			}
		}
	}

	if iosb != 0 {
		if err := env.mem.StoreLongword(env.cpu, iosb, status); err != nil {
			return ssAccVio, nil
		}
	}

	env.postFlag(efn, sched.ClassIOCompletion)

	if astadr != 0 {
		env.queueAST(astadr, astprm, uint32(env.cpu.PSL().CurMod()))
	}

	return r0, nil
}
