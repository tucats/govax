package corevms

import (
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
)

// Waiting with the scheduler (docs/PHASE-44.md, subtask 4).
//
// A service that can't finish yet ($HIBER with no wakeup pending,
// $WAITFR on a clear flag, a $QIOW whose I/O hasn't completed) returns
// ErrWait: the engine backs PC up to the service's XFC instruction, so
// the call is made again later, and finishes then if its wait is over.
// Without the scheduler, "later" is the next instruction: the process
// spins on its XFC, delivering ASTs and expiring timers between tries.
//
// With the scheduler, a waiting process gives the CPU to another. The
// service says what it's waiting for before it returns ErrWait (waitOn):
// a VMS wait state (HIB, LEF, CEF, MWAIT and its resource), the priority
// boost class of the event that will end it, and a Go function that
// reports whether it has ended. SystemService hands that to the
// scheduler (enterWait), which marks the process waiting and asks the
// engine to reschedule. Each time the scheduler is about to choose
// (wakeWaiters), it tests every waiting process: one whose wait has
// ended, or that has an AST it could take, becomes computable, boosted,
// and when it next runs its XFC runs again and finishes (or, after an
// AST, waits again), exactly as before. Testing every waiter at each
// choice can't miss an event, and with a handful of processes it's cheap.

// waitCondition is what a waiting process waits for.
type waitCondition struct {
	// state is the VMS wait state ($GETJPI's JPI$_STATE, SHOW SYSTEM),
	// and resource, for StateMWAIT, which resource.
	state    sched.State
	resource sched.Resource

	// class is the priority boost the process gets when the wait ends:
	// the class of the event that ends it (internal/sched, Table 10-3).
	class sched.Class

	// over reports whether the wait has ended. nil means the service
	// can't tell from outside: the process is made computable at the
	// next choice and tries again (a yield, as the old spin did).
	over func() bool

	// ignoreASTs keeps a queued AST from ending the wait: a process
	// whose deletion waits for its subprocesses (delete.go) runs nothing
	// more of its own.
	ignoreASTs bool
}

// The boost classes govax gives the events that end each kind of wait.
// VMS's own come from whoever reports the event (a timer, an I/O
// completion, a $SETEF), which govax doesn't track; these are its
// choice, unconfirmed: a wakeup or a resource is the book's "Wake a
// Process" and "Resource Available" (3), and an event flag is an I/O
// completion's (2), the commonest reason to wait for one.
const (
	hibernateBoost = sched.ClassResourceAvailable
	eventFlagBoost = sched.ClassIOCompletion
	resourceBoost  = sched.ClassResourceAvailable
)

// waitOn records why the service now running must wait, and returns
// ErrWait for it to return. The scheduler, if installed, uses the
// record; without one it's ignored, and the process spins as before.
func (env *Environment) waitOn(state sched.State, resource sched.Resource, class sched.Class, over func() bool) error {
	env.pendingWait = &waitCondition{state: state, resource: resource, class: class, over: over}

	return ErrWait
}

// waitOnFlag is waitOn for an event flag wait: LEF for one of the
// process's own flags (0-63), CEF for a common cluster's (64-127).
func (env *Environment) waitOnFlag(efn uint32, over func() bool) error {
	state := sched.StateLEF
	if efn&0xFF >= 64 {
		state = sched.StateCEF
	}

	return env.waitOn(state, sched.ResourceNone, eventFlagBoost, over)
}

// InstallScheduler makes the System e's scheduling hook (cpu.Scheduler),
// preempting in the modes ProcessSettings.Preempt allows, and lets
// waiting services and events ask e to reschedule. The console calls it
// when vax.process.scheduler is on.
func (sys *System) InstallScheduler(e *cpu.Engine) {
	sys.engine = e
	sys.lastCharge = sys.Clock()
	e.SetScheduler(sys, sys.ProcessSettings.Preempt.Modes())
}

// requestReschedule asks the engine to call the scheduler at the next
// instruction boundary: something has changed that may let a waiting
// process run (a $WAKE, ...), or the current one has started to wait.
// Without the scheduler it does nothing.
func (sys *System) requestReschedule() {
	if sys.engine != nil {
		sys.engine.RequestReschedule()
	}
}

// enterWait is SystemService's (and Shim's) step after a service: if it
// returned ErrWait and the scheduler is installed, the process enters
// the service's wait (see waitOn) in the scheduler, which is asked to
// reschedule. A service that waited without saying why waits for
// nothing in particular: it is retried at the next choice.
func (env *Environment) enterWait(waiting bool) {
	w := env.pendingWait
	env.pendingWait = nil

	if !waiting || env.engine == nil {
		return
	}

	if w == nil {
		w = &waitCondition{state: sched.StateLEF}
	}

	if err := env.sched.Wait(handle(env), w.state, w.resource); err != nil {
		return // not in the scheduler: nothing to wait in
	}

	if env.waiting == nil {
		env.waiters++
	}

	env.waiting = w
	env.requestReschedule()
}

// endWait makes a waiting process computable, with its wait's boost.
func (env *Environment) endWait(class sched.Class) {
	env.waiting = nil
	env.waiters--
	_, _ = env.sched.Ready(handle(env), class)
}

// pollEvents is what the scheduler does before each choice: every
// process's due timers expire (so a timer's event flag, wakeup, or AST
// happens on time whichever process it belongs to, not only when that
// process next runs), and then every waiting process is tested
// (wakeWaiters).
func (sys *System) pollEvents() {
	for _, env := range sys.procs.slots {
		if env != nil {
			env.expireTimers()
		}
	}

	if sys.wakeWaiters() {
		sys.idleSpinning = false
	}
}

// wakeWaiters tests every waiting process (see this file's opening
// comment): one whose wait is over, or that has an AST it could take,
// becomes computable. It reports whether any did.
func (sys *System) wakeWaiters() bool {
	if sys.waiters == 0 {
		return false
	}

	woke := false

	for _, env := range sys.procs.slots {
		if env == nil || env.waiting == nil {
			continue
		}

		w := env.waiting
		if w.over == nil || w.over() || (!w.ignoreASTs && env.astDeliverable()) {
			env.endWait(w.class)
			woke = true
		}
	}

	return woke
}

// retryWaiters makes every waiting process computable, without a boost,
// to try its wait again: what the idle loop falls back on when no timer
// can end a wait (idle.go). Each then spins on its XFC as it did before
// the scheduler, between instructions where CTRL/C and the run's limits
// are checked.
func (sys *System) retryWaiters() {
	for _, env := range sys.procs.slots {
		if env != nil && env.waiting != nil {
			env.endWait(sched.ClassNull)
		}
	}
}

// savedPSL is the PSL env's process will run with: the CPU's, if it is
// the process the CPU holds, or else the one saved in its hardware PCB.
// ok is false if there is no PCB to read.
func (env *Environment) savedPSL() (vax.PSL, bool) {
	if env == env.Current() {
		return env.cpu.PSL(), true
	}

	if env.Stacks == nil || env.Stacks.PCB == 0 {
		return 0, false
	}

	pcb, err := cpu.ReadPCB(env.mem, env.Stacks.PCBB)
	if err != nil {
		return 0, false
	}

	return pcb.PSL, true
}

// astDeliverable reports whether env's process has a queued AST it
// could take where it is waiting (by NextAST's rules, at its saved PSL).
// A waiting process with one becomes computable, takes the AST, and,
// when the AST routine returns to its XFC, waits again if it must, as
// on VMS (VAX/VMS Internals and Data Structures, section 10.2.2.1).
func (env *Environment) astDeliverable() bool {
	if len(env.Process.ast.queue) == 0 {
		return false
	}

	psl, ok := env.savedPSL()
	if !ok {
		return false
	}

	_, i := env.Process.deliverableAST(psl)

	return i >= 0
}
