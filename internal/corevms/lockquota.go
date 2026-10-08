package corevms

import (
	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/vmsdef"
)

// The lock services' quotas (docs/PHASE-49.md, subtask 7). $ENQ uses
// two (System Services manual, $ENQ's description):
//
//   - ENQLM, the enqueue limit: how many locks the job's processes may
//     hold or wait for at once. govax's ENQLM is a pooled job quota
//     (Job.Pooled, as the Internals book's JIB$W_ENQLM is), so every
//     lock of every process in the job counts, the locks RMS takes for
//     records and files included. A new lock past it is SS$_EXENQLM (RMS
//     reports RMS$_EXENQLM); a conversion takes no more.
//   - ASTLM, the AST limit: a request asking for a completion or a
//     blocking AST when the process has none left is SS$_EXQUOTA
//     (unconfirmed: SS$_EXASTLM is the other candidate). A request's
//     completion AST counts against the quota until it's delivered
//     (remainingASTs).
//
// JPI$_ENQCNT is what's left of ENQLM, JPI$_ASTCNT of ASTLM.

// ssExEnqLm is SS$_EXENQLM: a lock request past the ENQLM quota.
var ssExEnqLm = vmsdef.Symbols["SS$_EXENQLM"]

// jobLocks is how many locks the processes of env's job hold or wait
// for.
func (env *Environment) jobLocks() uint32 {
	job := env.Process.Job
	n := uint32(0)

	for _, p := range env.Processes() {
		if p.Process.Job == job {
			n += uint32(len(env.Locks.Locks(lck.Owner(p.Process.PID))))
		}
	}

	return n
}

// remainingLocks is JPI$_ENQCNT: how many more locks the job may take.
func (env *Environment) remainingLocks() uint32 {
	if env.Locks == nil {
		return env.Process.Job.Pooled.ENQLM
	}

	limit := env.Process.Job.Pooled.ENQLM

	return limit - min(env.jobLocks(), limit)
}

// pendingLockASTs is how many of the process's lock requests still have
// a completion AST to deliver: each counts against its AST quota.
func (env *Environment) pendingLockASTs() uint32 {
	if env.Locks == nil {
		return 0
	}

	n := uint32(0)

	for _, l := range env.Locks.Locks(lck.Owner(env.Process.PID)) {
		if r, ok := l.Data.(*enqRequest); ok && !r.done && r.astadr != 0 {
			n++
		}
	}

	return n
}
