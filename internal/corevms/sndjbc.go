package corevms

import (
	"github.com/tucats/govax/internal/jobqueue"
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
// routine; subtask 6 has the queue manager's and the queues'. A function
// VMS has that isn't in the table (the jobs' until subtask 7, and the
// forms, characteristics, and printers Decision 1 leaves out) is
// JBC$_NOTSUPPORTED; so is an item VMS has that the function doesn't
// take.

// sjcRequest is a $SNDJBC request's items, as read from its item list.
type sjcRequest struct {
	queue     string
	hasQueue  bool
	batch     bool
	start     bool // SJC$_CREATE_START
	directory string
	sets      []QueueSet
	status    uint32 // a refusal found while reading: NOTSUPPORTED, INVPARLEN
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

// readSjcItems reads the item list at itmlst into r: SS$_BADPARAM for a
// code past SJC$K_MAX_ITEM, or SS$_ACCVIO. An item VMS has that the queue
// functions don't take leaves JBC$_NOTSUPPORTED in r.status, and a queue
// name longer than 31 characters JBC$_INVPARLEN, as VMS 7.1 answered
// (BAT98Q's case 23).
func (env *Environment) readSjcItems(itmlst uint32, r *sjcRequest) uint32 {
	if itmlst == 0 {
		return 0
	}

	return env.walkItemList(itmlst, func(e itemListEntry) uint32 {
		if e.ItemCode == 0 || e.ItemCode > sjcMaxItem {
			return ssBadParam
		}

		read, ok := sjcItems[e.ItemCode]
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

// serviceSysSndjbc is SYS$SNDJBC and SYS$SNDJBCW.
func serviceSysSndjbc(env *Environment, argv []uint32) (uint32, error) {
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

	r0 := uint32(ssNormal)

	var r sjcRequest

	status := env.readSjcItems(itmlst, &r)

	fn, ok := sjcFunctions[uint16(function)]

	switch {
	case status != 0:
		r0 = status
	case function < uint32(sjcMinFunc) || function > uint32(sjcMaxFunc):
		r0, status = ssBadParam, ssBadParam
	case !ok:
		status = jbcNotSupported
	case r.status != 0:
		status = r.status
	default:
		status = fn(env, &r)
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
	t.Register("SYS$SNDJBC", serviceSysSndjbc)
	t.Register("SYS$SNDJBCW", serviceSysSndjbc)
}
