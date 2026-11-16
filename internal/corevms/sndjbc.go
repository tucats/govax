package corevms

import (
	"errors"
	"slices"
	"time"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// $SNDJBC and $SNDJBCW (docs/planning/PHASE-98 - batch queues.md,
// Decision 9): every queue and job operation, by function code and item
// list, as the System Services manual and VSI's pages describe them and
// VMS 7.1 answered them (testdata/bat98's BAT98Q, A.4):
//
//	SYS$SNDJBC[W] [efn] ,func ,[nullarg] ,[itmlst] ,[iosb] ,[astadr] ,[astprm]
//
// The request completes at once, so both forms behave the same: the event
// flag is cleared and set, the IOSB's first longword gets the status, its
// second 0, and an AST is queued if astadr is given. R0 is SS$_NORMAL and
// the status is in the IOSB, but for an unknown function or item code,
// which is SS$_BADPARAM in R0 and the IOSB alike.
//
// The functions are a table, sjcFunctions, a function code to its
// routine: the queue manager's and the queues' (subtask 6), and the jobs'
// (subtask 7), with ABORT_JOB and SYNCHRONIZE_JOB (subtask 8, batch.go). A function VMS has that isn't in the table (those,
// and the forms, characteristics, and printers Decision 1 leaves out) is
// JBC$_NOTSUPPORTED; so is an item VMS has that the function doesn't
// take. The queue functions read their items from sjcItems, the job
// functions from sjcJobItems: an item such as SJC$_WSDEFAULT means a
// queue's attribute to one and a job's limit to the other.

// sjcRequest is a $SNDJBC request's items, as read from its item list.
type sjcRequest struct {
	queue     string
	hasQueue  bool
	batch     bool
	start     bool // SJC$_CREATE_START
	directory string
	sets      []QueueSet
	status    uint32 // a refusal found while reading: NOTSUPPORTED, INVPARLEN

	// The job functions': the job's fields, an entry number (hasEntry),
	// hold or release (NO_HOLD), a new /AFTER, the changes ALTER_JOB
	// makes, and the output items, written when the function succeeds.
	job        JobRequest
	entry      int
	hasEntry   bool
	noHold     bool
	after      *time.Time
	jobSets    []func(*jobqueue.Job)
	outputs    []sjcOutput
	r0         uint32 // R0 when it isn't SS$_NORMAL (a file RMS couldn't open)
	priority   bool   // a PRIORITY item was given
	parameters bool   // a PARAMETER_n item was given
}

// sjcOutput is an output item: where to write it, and which.
type sjcOutput struct {
	entry itemListEntry
	code  uint16
}

// set is the request's attribute changes as one.
func (r *sjcRequest) set(q *jobqueue.Queue) {
	for _, s := range r.sets {
		s(q)
	}
}

// sjcFunction is one $SNDJBC function: it does the request and returns
// the status for the IOSB.
type sjcFunction func(env *Environment, r *sjcRequest) uint32

// sjcCode is a $SJCDEF code's value.
func sjcCode(name string) uint16 {
	v, ok := vmsdef.Symbols["SJC$_"+name]
	if !ok {
		panic("corevms: no $SJCDEF code SJC$_" + name)
	}

	return uint16(v)
}

// queueFunction makes a function of an operation on the queue the
// request names (JBC$_MISREQPAR without one).
func queueFunction(op func(env *Environment, r *sjcRequest) error) sjcFunction {
	return func(env *Environment, r *sjcRequest) uint32 {
		if !r.hasQueue {
			return jbcMisReqPar
		}

		return sjcStatus(op(env, r))
	}
}

// sjcFunctions are the functions govax has.
var sjcFunctions = map[uint16]sjcFunction{
	sjcCode("START_QUEUE_MANAGER"): func(env *Environment, r *sjcRequest) uint32 {
		return sjcStatus(env.StartQueueManager(r.directory))
	},
	sjcCode("STOP_QUEUE_MANAGER"): func(env *Environment, _ *sjcRequest) uint32 {
		return sjcStatus(env.StopQueueManager())
	},
	sjcCode("CREATE_QUEUE"): queueFunction(func(env *Environment, r *sjcRequest) error {
		return env.CreateQueue(r.queue, r.batch, r.start, r.set)
	}),
	sjcCode("START_QUEUE"): queueFunction(func(env *Environment, r *sjcRequest) error {
		return env.StartQueue(r.queue, r.set)
	}),
	sjcCode("PAUSE_QUEUE"): queueFunction(func(env *Environment, r *sjcRequest) error {
		return env.StopQueue(r.queue, StopPause)
	}),
	sjcCode("STOP_QUEUE"): queueFunction(func(env *Environment, r *sjcRequest) error {
		return env.StopQueue(r.queue, StopNext)
	}),
	sjcCode("RESET_QUEUE"): queueFunction(func(env *Environment, r *sjcRequest) error {
		return env.StopQueue(r.queue, StopReset)
	}),
	sjcCode("ALTER_QUEUE"): queueFunction(func(env *Environment, r *sjcRequest) error {
		return env.SetQueue(r.queue, r.set)
	}),
	sjcCode("DELETE_QUEUE"): queueFunction(func(env *Environment, r *sjcRequest) error {
		return env.DeleteQueue(r.queue)
	}),
	sjcCode("ENTER_FILE"): sjcEnterFile,
	sjcCode("CREATE_JOB"): sjcCreateJob,
	sjcCode("ADD_FILE"):   sjcAddFile,
	sjcCode("CLOSE_JOB"):  sjcCloseJob,
	sjcCode("ALTER_JOB"):  sjcAlterJob,
	sjcCode("DELETE_JOB"): sjcDeleteJob,
	sjcCode("ABORT_JOB"):  sjcAbortJob,
	// SYNCHRONIZE_JOB completes when its job ends: serviceSysSndjbc's
	// syncJob, not a table routine.
}

// sjcJobFunctions are the functions that read sjcJobItems.
var sjcJobFunctions = map[uint16]bool{
	sjcCode("ENTER_FILE"): true, sjcCode("CREATE_JOB"): true, sjcCode("ADD_FILE"): true,
	sjcCode("CLOSE_JOB"): true, sjcCode("ALTER_JOB"): true, sjcCode("DELETE_JOB"): true,
	sjcCode("ABORT_JOB"): true, sjcCode("SYNCHRONIZE_JOB"): true,
}

// sjcStatus is an operation's error as the IOSB's status: JBC$_NORMAL
// for none, a refusal's condition, or SS$_ABORT for a database that
// couldn't be read or written.
func sjcStatus(err error) uint32 {
	if err == nil {
		return jbcNormal
	}

	if st := QueueErrorStatus(err); st != 0 {
		return st
	}

	return ssAbort
}

// The function codes VMS has: SJC$K_MIN_FUNC to the last that isn't
// reserved (SJC$_RESERVED_FUNC_42 and after are SS$_BADPARAM).
var (
	sjcMinFunc = uint16(vmsdef.Symbols["SJC$K_MIN_FUNC"])
	sjcMaxFunc = uint16(vmsdef.Symbols["SJC$_RESERVED_FUNC_42"]) - 1
	sjcMaxItem = uint16(vmsdef.Symbols["SJC$K_MAX_ITEM"])
)

// sjcAttribute makes an item that changes a queue's attribute from a
// longword.
func sjcAttribute(apply func(q *jobqueue.Queue, v uint32)) func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
	return func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
		v, st := env.itemNumber(e, 4)
		if st != 0 {
			return st
		}

		r.sets = append(r.sets, func(q *jobqueue.Queue) { apply(q, uint32(v)) })

		return 0
	}
}

// sjcFlag makes an item with no value that changes a queue.
func sjcFlag(apply func(q *jobqueue.Queue)) func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
	return func(_ *Environment, _ itemListEntry, r *sjcRequest) uint32 {
		r.sets = append(r.sets, apply)

		return 0
	}
}

// signed is a longword item's value as a number: VMS's limits are
// longwords, and a value past 2**31 is taken as negative, which the
// queue's checks refuse.
func signed(v uint32) int { return int(int32(v)) }

// sjcItems are the input items the queue functions take.
var sjcItems = map[uint16]func(env *Environment, e itemListEntry, r *sjcRequest) uint32{
	sjcCode("QUEUE"): func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
		data, st := env.itemBytes(e)
		if st != 0 {
			return st
		}

		r.queue, r.hasQueue = string(data), true

		return 0
	},
	sjcCode("QUEUE_DIRECTORY"): func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
		data, st := env.itemBytes(e)
		r.directory = string(data)

		return st
	},
	sjcCode("BATCH"):        func(_ *Environment, _ itemListEntry, r *sjcRequest) uint32 { r.batch = true; return 0 },
	sjcCode("NO_BATCH"):     func(_ *Environment, _ itemListEntry, r *sjcRequest) uint32 { r.batch = false; return 0 },
	sjcCode("CREATE_START"): func(_ *Environment, _ itemListEntry, r *sjcRequest) uint32 { r.start = true; return 0 },
	sjcCode("JOB_LIMIT"):    sjcAttribute(func(q *jobqueue.Queue, v uint32) { q.JobLimit = signed(v) }),
	sjcCode("BASE_PRIORITY"): sjcAttribute(func(q *jobqueue.Queue, v uint32) {
		q.BasePriority = signed(v)
	}),
	sjcCode("OWNER_UIC"):      sjcAttribute(func(q *jobqueue.Queue, v uint32) { q.Owner = jobqueue.UIC(v) }),
	sjcCode("PROTECTION"):     sjcAttribute(func(q *jobqueue.Queue, v uint32) { q.Protection = jobqueue.Protection(v) }),
	sjcCode("WSDEFAULT"):      sjcAttribute(func(q *jobqueue.Queue, v uint32) { q.WSDefault = signed(v) }),
	sjcCode("WSQUOTA"):        sjcAttribute(func(q *jobqueue.Queue, v uint32) { q.WSQuota = signed(v) }),
	sjcCode("WSEXTENT"):       sjcAttribute(func(q *jobqueue.Queue, v uint32) { q.WSExtent = signed(v) }),
	sjcCode("NO_WSDEFAULT"):   sjcFlag(func(q *jobqueue.Queue) { q.WSDefault = 0 }),
	sjcCode("NO_WSQUOTA"):     sjcFlag(func(q *jobqueue.Queue) { q.WSQuota = 0 }),
	sjcCode("NO_WSEXTENT"):    sjcFlag(func(q *jobqueue.Queue) { q.WSExtent = 0 }),
	sjcCode("CPU_DEFAULT"):    sjcAttribute(func(q *jobqueue.Queue, v uint32) { q.CPUDefault = jobqueue.CPUTime(signed(v)) }),
	sjcCode("CPU_LIMIT"):      sjcAttribute(func(q *jobqueue.Queue, v uint32) { q.CPUMaximum = jobqueue.CPUTime(signed(v)) }),
	sjcCode("NO_CPU_DEFAULT"): sjcFlag(func(q *jobqueue.Queue) { q.CPUDefault = 0 }),
	sjcCode("NO_CPU_LIMIT"):   sjcFlag(func(q *jobqueue.Queue) { q.CPUMaximum = 0 }),
	sjcCode("RETAIN_ALL_JOBS"): sjcFlag(func(q *jobqueue.Queue) {
		q.Retain = jobqueue.RetainAlways
	}),
	sjcCode("RETAIN_ERROR_JOBS"): sjcFlag(func(q *jobqueue.Queue) { q.Retain = jobqueue.RetainError }),
	sjcCode("NO_RETAIN_JOBS"):    sjcFlag(func(q *jobqueue.Queue) { q.Retain = jobqueue.RetainDefault }),
	sjcCode("QUEUE_DESCRIPTION"): func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
		data, st := env.itemBytes(e)
		if st != 0 {
			return st
		}

		text := string(data)
		r.sets = append(r.sets, func(q *jobqueue.Queue) { q.Description = text })

		return 0
	},
	sjcCode("NO_QUEUE_DESCRIPTION"): sjcFlag(func(q *jobqueue.Queue) { q.Description = "" }),
}

// jobItem makes a job item that changes the job ALTER_JOB changes, and
// the job ENTER_FILE and CREATE_JOB make.
func jobItem(apply func(r *JobRequest, j *jobqueue.Job, v uint32)) func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
	return func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
		v, st := env.itemNumber(e, 4)
		if st != 0 {
			return st
		}

		apply(&r.job, nil, uint32(v))
		r.jobSets = append(r.jobSets, func(j *jobqueue.Job) { apply(nil, j, uint32(v)) })

		return 0
	}
}

// jobText makes a job item of a string.
func jobText(apply func(r *JobRequest, j *jobqueue.Job, text string)) func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
	return func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
		data, st := env.itemBytes(e)
		if st != 0 {
			return st
		}

		text := string(data)
		apply(&r.job, nil, text)
		r.jobSets = append(r.jobSets, func(j *jobqueue.Job) { apply(nil, j, text) })

		return 0
	}
}

// jobFlag makes a job item with no value.
func jobFlag(apply func(r *JobRequest, j *jobqueue.Job)) func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
	return func(_ *Environment, _ itemListEntry, r *sjcRequest) uint32 {
		apply(&r.job, nil)
		r.jobSets = append(r.jobSets, func(j *jobqueue.Job) { apply(nil, j) })

		return 0
	}
}

// either sets a field of whichever of r and j isn't nil.
func either[T any](r *JobRequest, j *jobqueue.Job, ofRequest func(*JobRequest) *T, ofJob func(*jobqueue.Job) *T, v T) {
	if r != nil {
		*ofRequest(r) = v
	}

	if j != nil {
		*ofJob(j) = v
	}
}

// parameterItem is PARAMETER_n: the job's nth parameter, the list padded
// with empty ones before it.
func parameterItem(n int) func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
	set := func(list []string, text string) []string {
		for len(list) < n {
			list = append(list, "")
		}

		list[n-1] = text

		return list
	}

	// The job's list is changed whole, by sjcAlterJob: VMS 7.1's ALTER_JOB
	// with PARAMETER_2 alone left PARAMETER_1 empty (round 2's case 11).
	return func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
		data, st := env.itemBytes(e)
		if st != 0 {
			return st
		}

		r.job.Parameters, r.parameters = set(r.job.Parameters, string(data)), true

		return 0
	}
}

// sjcJobItems are the items the job functions take.
var sjcJobItems = func() map[uint16]func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
	m := map[uint16]func(env *Environment, e itemListEntry, r *sjcRequest) uint32{
		sjcCode("QUEUE"): sjcItems[sjcCode("QUEUE")],
		sjcCode("FILE_SPECIFICATION"): func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
			data, st := env.itemBytes(e)
			r.job.Files = append(r.job.Files, string(data))

			return st
		},
		sjcCode("ENTRY_NUMBER"): func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
			v, st := env.itemNumber(e, 4)
			r.entry, r.hasEntry = signed(uint32(v)), true

			return st
		},
		sjcCode("USERNAME"): func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
			data, st := env.itemBytes(e)
			r.job.User = string(data)

			return st
		},
		sjcCode("HOLD"):    func(_ *Environment, _ itemListEntry, r *sjcRequest) uint32 { r.job.Hold = true; return 0 },
		sjcCode("NO_HOLD"): func(_ *Environment, _ itemListEntry, r *sjcRequest) uint32 { r.noHold = true; return 0 },
		sjcCode("AFTER_TIME"): func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
			v, ok := env.loadQuad(e.BuffAddr)
			if !ok {
				return ssAccVio
			}

			t := goTime(v)
			r.job.After, r.after = t, &t

			return 0
		},
		sjcCode("NO_AFTER_TIME"): func(_ *Environment, _ itemListEntry, r *sjcRequest) uint32 {
			r.after = &time.Time{}

			return 0
		},
		sjcCode("PRIORITY"): func(env *Environment, e itemListEntry, r *sjcRequest) uint32 {
			v, st := env.itemNumber(e, 4)
			r.job.Priority, r.priority = signed(uint32(v)), true

			return st
		},
		sjcCode("ENTRY_NUMBER_OUTPUT"): sjcOutputItem,
		sjcCode("JOB_STATUS_OUTPUT"):   sjcOutputItem,
		sjcCode("JOB_NAME"): jobText(func(r *JobRequest, j *jobqueue.Job, t string) {
			either(r, j, func(r *JobRequest) *string { return &r.Name }, func(j *jobqueue.Job) *string { return &j.Name }, t)
		}),
		sjcCode("NOTE"): jobText(func(r *JobRequest, j *jobqueue.Job, t string) {
			either(r, j, func(r *JobRequest) *string { return &r.Note }, func(j *jobqueue.Job) *string { return &j.Note }, t)
		}),
		sjcCode("NO_NOTE"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *string { return &r.Note }, func(j *jobqueue.Job) *string { return &j.Note }, "")
		}),
		sjcCode("LOG_SPECIFICATION"): jobText(func(r *JobRequest, j *jobqueue.Job, t string) {
			either(r, j, func(r *JobRequest) *string { return &r.LogFile }, func(j *jobqueue.Job) *string { return &j.LogFile }, t)
		}),
		sjcCode("NO_LOG_SPECIFICATION"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.NoLog }, func(j *jobqueue.Job) *bool { return &j.NoLog }, true)
		}),
		sjcCode("LOG_SPOOL"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.NoPrinter }, func(j *jobqueue.Job) *bool { return &j.NoPrinter }, false)
		}),
		sjcCode("NO_LOG_SPOOL"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.NoPrinter }, func(j *jobqueue.Job) *bool { return &j.NoPrinter }, true)
		}),
		sjcCode("LOG_DELETE"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.NoKeep }, func(j *jobqueue.Job) *bool { return &j.NoKeep }, true)
		}),
		sjcCode("NO_LOG_DELETE"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.NoKeep }, func(j *jobqueue.Job) *bool { return &j.NoKeep }, false)
		}),
		sjcCode("NOTIFY"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.Notify }, func(j *jobqueue.Job) *bool { return &j.Notify }, true)
		}),
		sjcCode("NO_NOTIFY"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.Notify }, func(j *jobqueue.Job) *bool { return &j.Notify }, false)
		}),
		sjcCode("RESTART"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.Restart }, func(j *jobqueue.Job) *bool { return &j.Restart }, true)
		}),
		sjcCode("NO_RESTART"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.Restart }, func(j *jobqueue.Job) *bool { return &j.Restart }, false)
		}),
		sjcCode("DELETE_FILE"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.Delete }, func(j *jobqueue.Job) *bool { return &j.Delete }, true)
		}),
		sjcCode("NO_DELETE_FILE"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *bool { return &r.Delete }, func(j *jobqueue.Job) *bool { return &j.Delete }, false)
		}),
		sjcCode("CPU_LIMIT"): jobItem(func(r *JobRequest, j *jobqueue.Job, v uint32) {
			either(r, j, func(r *JobRequest) *jobqueue.CPUTime { return &r.CPUTime },
				func(j *jobqueue.Job) *jobqueue.CPUTime { return &j.CPUTime }, jobqueue.CPUTime(signed(v)))
		}),
		sjcCode("NO_CPU_LIMIT"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *jobqueue.CPUTime { return &r.CPUTime },
				func(j *jobqueue.Job) *jobqueue.CPUTime { return &j.CPUTime }, 0)
		}),
		sjcCode("NO_PARAMETERS"): jobFlag(func(r *JobRequest, j *jobqueue.Job) {
			either(r, j, func(r *JobRequest) *[]string { return &r.Parameters }, func(j *jobqueue.Job) *[]string { return &j.Parameters }, nil)
		}),
	}

	for _, ws := range []struct {
		name  string
		field func(*JobRequest) *int
		job   func(*jobqueue.Job) *int
	}{
		{"WSDEFAULT", func(r *JobRequest) *int { return &r.WSDefault }, func(j *jobqueue.Job) *int { return &j.WSDefault }},
		{"WSQUOTA", func(r *JobRequest) *int { return &r.WSQuota }, func(j *jobqueue.Job) *int { return &j.WSQuota }},
		{"WSEXTENT", func(r *JobRequest) *int { return &r.WSExtent }, func(j *jobqueue.Job) *int { return &j.WSExtent }},
	} {
		m[sjcCode(ws.name)] = jobItem(func(r *JobRequest, j *jobqueue.Job, v uint32) { either(r, j, ws.field, ws.job, signed(v)) })
		m[sjcCode("NO_"+ws.name)] = jobFlag(func(r *JobRequest, j *jobqueue.Job) { either(r, j, ws.field, ws.job, 0) })
	}

	for n := 1; n <= jobqueue.MaxParameters; n++ {
		m[sjcCode("PARAMETER_"+string(rune('0'+n)))] = parameterItem(n)
	}

	return m
}()

// sjcOutputItem records an output item, written when the function
// succeeds. ENTRY_NUMBER_OUTPUT is a longword, whose buffer must be 4
// bytes long (JBC$_INVPARLEN, VMS 7.1's answer to round 1's 256).
func sjcOutputItem(_ *Environment, e itemListEntry, r *sjcRequest) uint32 {
	if e.ItemCode == sjcCode("ENTRY_NUMBER_OUTPUT") && e.BuffLen != 4 && r.status == 0 {
		r.status = jbcInvParLen
	}

	r.outputs = append(r.outputs, sjcOutput{entry: e, code: e.ItemCode})

	return 0
}

// writeOutputs writes the job function's output items for job s: its
// entry number, and SUBMIT's message as text, cut to the buffer (VMS
// 7.1's "Job QJOB (queue BAT98_Q, entry 419) holding", round 2's case 6).
func (env *Environment) writeOutputs(r *sjcRequest, s *Submitted) uint32 {
	for _, o := range r.outputs {
		var v itemValue

		switch o.code {
		case sjcCode("ENTRY_NUMBER_OUTPUT"):
			v = itemLong(uint32(s.Job.Entry))
		default:
			v = itemString(s.Message()[0])
		}

		if st := env.storeItem(o.entry, v); st != 0 {
			return st
		}
	}

	return 0
}

// submitStatus is a job function's error as R0 and the IOSB's status: a
// file RMS couldn't find or open is RMS's status in both (VMS 7.1's
// RMS$_FNF, round 2's case 18); another refusal is sjcStatus's in the
// IOSB, R0 SS$_NORMAL; a /USER account that isn't there or can't be read
// is JBC$_INVPARVAL (unconfirmed: the probe named none to $SNDJBC).
func submitStatus(err error) (r0, status uint32) {
	var file *rms.DCLFileError
	if errors.As(err, &file) {
		return file.Status, file.Status
	}

	var user *JobUserError
	if errors.As(err, &user) {
		return ssNormal, jbcInvParVal
	}

	return ssNormal, sjcStatus(err)
}

// sjcSubmit enters r.job and writes the outputs.
func (env *Environment) sjcSubmit(r *sjcRequest) (r0, status uint32) {
	if !r.priority {
		r.job.Priority = -1
	}

	r.job.Queue = r.queue

	s, err := env.SubmitJob(r.job)
	if err != nil {
		return submitStatus(err)
	}

	if st := env.writeOutputs(r, &s); st != 0 {
		return st, st
	}

	if s.PriorityLowered {
		return ssNormal, jbcPrioSmall
	}

	return ssNormal, jbcNormal
}

// sjcEnterFile is ENTER_FILE: a job of the request's files, entered at
// once. With no file it is JBC$_MISREQPAR (round 2's case 15).
func sjcEnterFile(env *Environment, r *sjcRequest) uint32 {
	if !r.hasQueue || len(r.job.Files) == 0 {
		return jbcMisReqPar
	}

	r0, status := env.sjcSubmit(r)
	r.r0 = r0

	return status
}

// sjcCreateJob is CREATE_JOB: a job opened in the process, its files to
// come by ADD_FILE, entered by CLOSE_JOB. Its entry number is the one the
// job will have (round 2's case 44 gave one at CREATE_JOB): govax gives
// it at CLOSE_JOB, and CREATE_JOB's ENTRY_NUMBER_OUTPUT is the number the
// next job is expected to take (unconfirmed).
func sjcCreateJob(env *Environment, r *sjcRequest) uint32 {
	if !r.hasQueue {
		return jbcMisReqPar
	}

	if st, err := env.managerHere(); err != nil || st != 0 {
		return sjcStatus(queueResult(st, err))
	}

	d, err := jobqueue.Read(env.System.queues.path)
	if err != nil {
		return ssAbort
	}

	if d.Queue(queueName(r.queue)) == nil {
		return jbcNoSuchQue
	}

	job := r.job
	job.Queue = r.queue

	if !r.priority {
		job.Priority = -1
	}

	env.openJob = &job

	s := Submitted{Job: jobqueue.Job{Entry: d.NextEntry, Name: job.Name, Queue: queueName(r.queue)}}
	if st := env.writeOutputs(r, &s); st != 0 {
		return st
	}

	return jbcNormal
}

// sjcAddFile is ADD_FILE: the request's files added to the open job
// (JBC$_NOSUCHJOB with none open).
func sjcAddFile(env *Environment, r *sjcRequest) uint32 {
	if env.openJob == nil {
		return jbcNoSuchJob
	}

	if len(r.job.Files) == 0 {
		return jbcMisReqPar
	}

	env.openJob.Files = append(env.openJob.Files, r.job.Files...)

	return jbcNormal
}

// sjcCloseJob is CLOSE_JOB: the open job entered (JBC$_NOSUCHJOB with
// none open, round 2's case 49).
func sjcCloseJob(env *Environment, r *sjcRequest) uint32 {
	job := env.openJob
	if job == nil {
		return jbcNoSuchJob
	}

	env.openJob = nil

	open := &sjcRequest{job: *job, outputs: r.outputs}

	s, err := env.SubmitJob(open.job)
	if err != nil {
		r0, status := submitStatus(err)
		r.r0 = r0

		return status
	}

	if st := env.writeOutputs(open, &s); st != 0 {
		return st
	}

	return jbcNormal
}

// sjcAlterJob is ALTER_JOB: the job ENTRY_NUMBER names (in QUEUE, if
// given) changed by the request's job items, held by HOLD, released by
// NO_HOLD (round 2's cases 10 and 12).
func sjcAlterJob(env *Environment, r *sjcRequest) uint32 {
	if !r.hasEntry {
		return jbcMisReqPar
	}

	c := JobChange{Hold: r.job.Hold, Release: r.noHold, After: r.after, Priority: -1}

	if r.priority {
		c.Priority = r.job.Priority
	}

	if r.parameters {
		params := r.job.Parameters
		r.jobSets = append(r.jobSets, func(j *jobqueue.Job) { j.Parameters = slices.Clone(params) })
	}

	if len(r.jobSets) > 0 {
		c.Set = func(j *jobqueue.Job) {
			for _, set := range r.jobSets {
				set(j)
			}
		}
	}

	queue := ""
	if r.hasQueue {
		queue = r.queue
	}

	lowered, err := env.AlterJob(r.entry, queue, c)
	if err == nil && lowered {
		return jbcPrioSmall
	}

	return sjcStatus(err)
}

// sjcDeleteJob is DELETE_JOB: the job ENTRY_NUMBER names (in QUEUE, if
// given) deleted (round 2's cases 19, 42, 43, 50).
func sjcDeleteJob(env *Environment, r *sjcRequest) uint32 {
	if !r.hasEntry {
		return jbcMisReqPar
	}

	queue := ""
	if r.hasQueue {
		queue = r.queue
	}

	return sjcStatus(env.DeleteJob(r.entry, queue))
}

// sjcAbortJob is ABORT_JOB: the executing job ENTRY_NUMBER names, in
// QUEUE, ended with JBC$_JOBABORT (AbortJob).
func sjcAbortJob(env *Environment, r *sjcRequest) uint32 {
	if !r.hasEntry || !r.hasQueue {
		return jbcMisReqPar
	}

	return sjcStatus(env.AbortJob(r.entry, r.queue))
}

// syncJob is SYNCHRONIZE_JOB: it completes when the job ENTRY_NUMBER (or
// the process's job JOB_NAME in QUEUE) ends, its IOSB the job's status
// (round 2's case 13: SS$_NORMAL), its event flag set and its AST queued
// then. It returns 0 when the request waits, or the status to complete it
// with now: a job that isn't there (JBC$_NOSUCHENT, JBC$_NOSUCHJOB), or one
// already ended. w is the wait, for $SNDJBCW to wait on.
func (env *Environment) syncJob(r *sjcRequest, efn, iosb, astadr, astprm uint32) (status uint32, w *syncWait) {
	since := env.System.JobEndMark()

	entry := 0
	if r.hasEntry {
		entry = r.entry
	}

	queue := ""
	if r.hasQueue {
		queue = r.queue
	}

	found, st, done, err := env.SyncTarget(entry, r.job.Name, queue)

	switch {
	case err != nil:
		return sjcStatus(err), nil
	case done:
		env.System.ConsumeJobEnd(found)

		return st, nil
	}

	w = &syncWait{env: env, entry: found, since: since, efn: efn, iosb: iosb,
		astadr: astadr, astprm: astprm, mode: uint32(env.cpu.PSL().CurMod())}
	env.System.syncWaits = append(env.System.syncWaits, w)

	return 0, w
}

// readSjcItems reads the item list at itmlst into r: SS$_BADPARAM for a
// code past SJC$K_MAX_ITEM, or SS$_ACCVIO. An item VMS has that the queue
// functions don't take leaves JBC$_NOTSUPPORTED in r.status, and a queue
// name longer than 31 characters JBC$_INVPARLEN, as VMS 7.1 answered
// (BAT98Q's case 23).
func (env *Environment) readSjcItems(itmlst uint32, function uint32, r *sjcRequest) uint32 {
	if itmlst == 0 {
		return 0
	}

	items := sjcItems
	if sjcJobFunctions[uint16(function)] {
		items = sjcJobItems
	}

	return env.walkItemList(itmlst, func(e itemListEntry) uint32 {
		if e.ItemCode == 0 || e.ItemCode > sjcMaxItem {
			return ssBadParam
		}

		read, ok := items[e.ItemCode]
		if !ok {
			if r.status == 0 {
				r.status = jbcNotSupported
			}

			return 0
		}

		if e.ItemCode == sjcCode("QUEUE") && e.BuffLen > jobqueue.MaxQueueName && r.status == 0 {
			r.status = jbcInvParLen
		}

		return read(env, e, r)
	})
}

// serviceSysSndjbc is SYS$SNDJBC and SYS$SNDJBCW (wait): every function
// completes at once but SYNCHRONIZE_JOB, which completes when its job ends
// (syncJob), and which $SNDJBCW waits for.
func serviceSysSndjbc(env *Environment, argv []uint32, wait bool) (uint32, error) {
	if len(argv) < 2 {
		return ssInsfArg, nil
	}

	// $SNDJBCW's XFC run again while its SYNCHRONIZE_JOB waits.
	if w := env.pendingSync; w != nil {
		if !w.done && !env.Deleted {
			return 0, env.waitOnFlag(w.efn, func() bool { return w.done })
		}

		env.pendingSync = nil

		return ssNormal, nil
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

	r0 := uint32(ssNormal)

	// A job $SNDJBC makes doesn't spool its log unless LOG_SPOOL says so:
	// VMS 7.1's held job of ENTER_FILE had no JOB_LOG_SPOOL flag ($GETQUI's
	// JOB_FLAGS 0, round 2's case 7), where SUBMIT's default /PRINTER asks
	// for it.
	var r sjcRequest

	r.job.NoPrinter = true

	status := env.readSjcItems(itmlst, function, &r)

	fn, ok := sjcFunctions[uint16(function)]

	switch {
	case status != 0:
		r0 = status
	case function < uint32(sjcMinFunc) || function > uint32(sjcMaxFunc):
		r0, status = ssBadParam, ssBadParam
	case uint16(function) == sjcCode("SYNCHRONIZE_JOB") && r.status == 0:
		var w *syncWait

		if status, w = env.syncJob(&r, efn, iosb, astadr, astprm); w != nil {
			if wait {
				env.pendingSync = w

				return 0, env.waitOnFlag(efn, func() bool { return w.done })
			}

			return r0, nil
		}
	case !ok:
		status = jbcNotSupported
	case r.status != 0:
		status = r.status
	default:
		status = fn(env, &r)

		if r.r0 != 0 {
			r0 = r.r0
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

func registerQueueServices(t *ServiceTable) {
	t.Register("SYS$SNDJBC", func(env *Environment, argv []uint32) (uint32, error) { return serviceSysSndjbc(env, argv, false) })
	t.Register("SYS$SNDJBCW", func(env *Environment, argv []uint32) (uint32, error) { return serviceSysSndjbc(env, argv, true) })
	t.Register("SYS$GETQUI", serviceSysGetqui)
	t.Register("SYS$GETQUIW", serviceSysGetqui)
}
