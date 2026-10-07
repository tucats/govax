package corevms

import (
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Suspension (docs/PHASE-45.md, subtask 9): $SUSPND and $RESUME.
//
// A suspended process doesn't run until another process resumes it, even
// if what it was waiting for happens. govax models the one kind of
// suspension, which takes it out of the scheduler's choice (state SUSP):
//
//   - A computable or running process becomes SUSP at once, and computable
//     again, with no boost, when resumed.
//   - A waiting process (HIB, LEF, ...) keeps its wait (Environment.waiting)
//     but is SUSP meanwhile: wakeWaiters leaves it alone, so an event that
//     ends its wait isn't acted on until it is resumed. Resumed, it goes
//     back to its wait state, and the next scheduler choice wakes it if
//     its wait has ended.
//
// VMS's suspend lets kernel-mode ASTs through, which govax doesn't model:
// no AST is delivered to a suspended process, and the deletion of one is
// preceded by its resumption (markForDeletion), so it can run. The flags
// argument of $SUSPND (kernel-mode, supervisor-mode suspension in later
// VMS) is accepted and ignored (unconfirmed).

var ssSuspended = vmsdef.Symbols["SS$_SUSPENDED"]

// Suspended reports whether the process is suspended.
func (env *Environment) Suspended() bool { return env.suspended }

// suspend suspends env's process. It reports false if it already was.
func (env *Environment) suspend() bool {
	if env.suspended {
		return false
	}

	env.suspended = true

	// Wait puts a computable or running process into SUSP (a running one
	// gives up the CPU at the next instruction boundary). A waiting one
	// is moved to SUSP as well; its wait is kept for the resume.
	if err := env.sched.Wait(handle(env), sched.StateSUSP, sched.ResourceNone); err == nil {
		env.requestReschedule()
	}

	return true
}

// resume resumes env's process if it is suspended; it is a no-op
// otherwise. A process that was waiting goes back to that wait, and one
// that wasn't becomes computable.
func (env *Environment) resume() {
	if !env.suspended {
		return
	}

	env.suspended = false

	if w := env.waiting; w != nil {
		_ = env.sched.Wait(handle(env), w.state, w.resource)
	} else {
		_, _ = env.sched.Ready(handle(env), sched.ClassNull)
	}

	env.requestReschedule()
}

// serviceSysSuspnd is SYS$SUSPND:
//
//	SYS$SUSPND [pidadr] ,[prcnam] ,[flags]
//
// It suspends the target process (processTarget; the caller with
// neither), which then doesn't run until $RESUME. A process may suspend
// itself. Another process needs GROUP or WORLD unless it has the
// caller's UIC (mayAffect: SS$_NOPRIV). Suspending a process that is
// already suspended is SS$_SUSPENDED (unconfirmed; the process stays
// suspended once, and one $RESUME resumes it).
func serviceSysSuspnd(env *Environment, argv []uint32) (uint32, error) {
	target, st := env.processTarget(optArg(argv, 0), optArg(argv, 1), false)
	if st != 0 {
		return st, nil
	}

	if st := env.mayAffect(target); st != 0 {
		return st, nil
	}

	if !target.suspend() {
		return ssSuspended, nil
	}

	return ssNormal, nil
}

// serviceSysResume is SYS$RESUME:
//
//	SYS$RESUME [pidadr] ,[prcnam]
//
// It resumes the target process, picked as $SUSPND picks it, with the
// same privilege rules. Resuming a process that isn't suspended is
// SS$_NORMAL and does nothing.
func serviceSysResume(env *Environment, argv []uint32) (uint32, error) {
	target, st := env.processTarget(optArg(argv, 0), optArg(argv, 1), false)
	if st != 0 {
		return st, nil
	}

	if st := env.mayAffect(target); st != 0 {
		return st, nil
	}

	target.resume()

	return ssNormal, nil
}

func registerSuspendServices(t *ServiceTable) {
	t.Register("SYS$SUSPND", serviceSysSuspnd)
	t.Register("SYS$RESUME", serviceSysResume)
}
