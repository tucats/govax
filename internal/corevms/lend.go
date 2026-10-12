package corevms

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/sched"
)

// Giving the CPU up between DCL commands (docs/PHASE-50, subtask 21).
//
// The command interpreter is Go, so a DCL command takes no instructions,
// and nothing but a process's own wait or quantum end would let another
// process run. Two things give the CPU up of their own accord:
//
//   - A subprocess's CLI, after each command it runs, yields from its
//     shim (Yield): the process stays computable, going to the back of
//     its priority's queue, and its shim's XFC runs again, with the next
//     command, when the scheduler next chooses it. Its DCL's WAIT waits
//     in the scheduler instead (WaitUntil).
//   - Process 1's DCL runs between machine runs, with the CPU stopped.
//     Between its commands, the console lends the CPU (LendCPU): process
//     1 yields, or waits for a WAIT's time, and the machine runs the
//     other processes until the scheduler chooses process 1 again, when
//     the console stops it, before process 1 runs anything, and goes on
//     with its next command. ReclaimCPU takes the CPU back early.

// Yield is a shim's way to give the CPU up while staying computable: it
// returns ErrWait, so the shim's XFC runs again when the process is next
// chosen, with no wait (see enterWait). Without the scheduler, the XFC
// simply runs again at the next instruction.
func (env *Environment) Yield() error {
	env.pendingWait = &waitCondition{yield: true}

	return ErrWait
}

// WaitUntil is a shim's way to wait until over reports true, in LEF,
// woken with a timer's boost. deadline, when not 0, is the system time
// (Clock) by which over is expected to be true, so the scheduler, with
// nothing to run, idles until then rather than retrying the wait at
// once.
func (env *Environment) WaitUntil(over func() bool, deadline uint64) error {
	env.waitDeadline = deadline

	return env.waitOn(sched.StateLEF, sched.ResourceNone, sched.ClassTimer, over)
}

// LendCPU makes env, the scheduler's current process, give the CPU up
// between instructions, while it runs nothing (process 1, between two of
// its DCL commands): it yields, staying computable, if over is nil, or
// else waits in LEF until over reports true, with deadline as
// WaitUntil's. The next instruction boundary chooses who runs. It
// reports false, changing nothing, when there is no scheduler or env
// isn't current.
func (sys *System) LendCPU(env *Environment, over func() bool, deadline uint64) bool {
	if sys.engine == nil {
		return false
	}

	if h, ok := sys.sched.Current(); !ok || h != handle(env) {
		return false
	}

	if over == nil {
		sys.sched.Yield()
		sys.requestReschedule()

		return true
	}

	env.waitDeadline = deadline

	// Process 1 runs nothing of its own while it lends the CPU, so an AST
	// queued to it doesn't end the wait: DCL's next run takes it.
	env.pendingWait = &waitCondition{state: sched.StateLEF, class: sched.ClassTimer, over: over, ignoreASTs: true}
	env.enterWait(true)

	return true
}

// ReclaimCPU gives env, which lent the CPU (LendCPU), the CPU back now,
// whoever holds it: a wait it lent the CPU for ends, and it becomes the
// current process, the one the CPU holds.
func (sys *System) ReclaimCPU(e *cpu.Engine, env *Environment) error {
	if env.waiting != nil {
		env.waitDeadline = 0
		env.endWait(sched.ClassNull)
	}

	if sys.Current() != env {
		return sys.SwitchCPU(e, env)
	}

	if h, ok := sys.sched.Current(); !ok || h != handle(env) {
		return sys.sched.Choose(handle(env))
	}

	return nil
}
