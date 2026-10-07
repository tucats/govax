package cpu

import "github.com/tucats/govax/internal/vax"

// The engine's scheduling hook (docs/PHASE-44.md, "The engine's hook").
//
// With several processes on one CPU, something has to decide, between
// instructions, whether the running process should give the CPU to
// another: because its quantum (its turn) is used up, because a
// higher-priority process has become ready to run, or because it has
// started to wait. On VMS that is the scheduler, run as a software
// interrupt at IPL 3. In govax the rules are Go (internal/sched), owned
// by the system (internal/corevms), and the engine only gives them their
// chance: Step counts instructions down, and when the count runs out (or
// a reschedule has been asked for) it calls the Scheduler at the next
// instruction boundary. Most instructions pay only a decrement and a
// compare; with no Scheduler installed, only a nil test.

// Scheduler is the hook Step calls at an instruction boundary when the
// count the previous call returned has run out, or after
// RequestReschedule.
type Scheduler interface {
	// Schedule charges ran instructions (those executed since the last
	// call) to the current process, and decides whether another process
	// should run now. preemptible says whether the CPU may be taken from
	// the current process against its will at this boundary (see
	// Engine.Preemptible); a process that is waiting gives the CPU up
	// either way. To switch, Schedule saves and loads the hardware
	// context itself (Engine.SaveContext/LoadContext), so the next
	// instruction Step executes is the new process's.
	//
	// It returns how many instructions may run before the next call (at
	// least 1). An error stops the engine: Step returns it.
	Schedule(e *Engine, ran int, preemptible bool) (next int, err error)
}

// PreemptModes is a set of access modes, one bit per mode (1<<vax.Kernel
// through 1<<vax.User): the modes in which the scheduler may preempt a
// process (the vax.process.preempt setting).
type PreemptModes uint8

const (
	// PreemptAllModes preempts in any mode: VMS's own rule.
	PreemptAllModes PreemptModes = 1<<vax.Kernel | 1<<vax.Executive | 1<<vax.Supervisor | 1<<vax.User

	// PreemptUserMode preempts only user-mode code.
	PreemptUserMode PreemptModes = 1 << vax.User

	// PreemptNoModes never preempts: processes switch only when one waits.
	PreemptNoModes PreemptModes = 0
)

// schedulerIPL is the IPL VMS's scheduler interrupt runs at (IPL$_SCHED,
// 3). Code running at that IPL or above can't be rescheduled, on VMS
// because the software interrupt isn't taken until IPL drops below it.
const schedulerIPL = 3

// SetScheduler installs s as the engine's scheduling hook, preempting in
// the given modes, or removes the hook if s is nil. The first call to s
// comes before the next instruction.
func (e *Engine) SetScheduler(s Scheduler, modes PreemptModes) {
	e.scheduler = s
	e.preemptModes = modes
	e.schedBudget, e.schedLeft = 0, 0

	if s == nil {
		e.schedLeft = noSchedulerBudget
	}
}

// noSchedulerBudget is schedLeft with no scheduler installed: so large
// that SwitchIfDue's one test sends it home at once. (Step only counts
// schedLeft down when there is a scheduler.)
const noSchedulerBudget = 1 << 62

// RequestReschedule asks for the Scheduler to be called at the next
// instruction boundary, sooner than its count would: a process has just
// started to wait, or one has become ready that should run before the
// current one. Without a Scheduler it does nothing.
func (e *Engine) RequestReschedule() {
	// Shrink the budget to what has been used of it, so the next call's
	// count of instructions run stays right.
	e.schedBudget -= e.schedLeft
	e.schedLeft = 0
}

// Preemptible reports whether the current process may be preempted at
// this instruction boundary: the CPU is below the scheduler's IPL, not
// on the interrupt stack (interrupt handlers belong to no process), and
// in one of the modes the preempt setting allows.
func (e *Engine) Preemptible() bool {
	psl := e.cpu.PSL()

	return psl.IPL() < schedulerIPL && !psl.IS() && e.preemptModes&(1<<psl.CurMod()) != 0
}

// schedule calls the Scheduler: Step's slow path, when the count has run
// out.
func (e *Engine) schedule() error {
	ran := e.schedBudget - e.schedLeft

	next, err := e.scheduler.Schedule(e, ran, e.Preemptible())
	if err != nil {
		return err
	}

	next = max(next, 1)
	e.schedBudget, e.schedLeft = next, next

	return nil
}

// FreezeScheduling stops the engine calling the scheduler, so the
// process the CPU holds keeps it, whatever happens, until the function
// it returns is called (docs/PHASE-44.md, subtask 8). The debugger's STEP
// freezes scheduling so a step is one instruction of the debugged
// process, and a nested run (the console's condition-handler call) so
// the outer run finds the CPU as it left it. Freezes nest. Instructions
// run while frozen are charged, and a reschedule asked for meanwhile
// made, at the first scheduler call after the last unfreeze.
func (e *Engine) FreezeScheduling() (unfreeze func()) {
	e.schedFrozen++

	done := false

	return func() {
		if !done {
			done = true
			e.schedFrozen--
		}
	}
}

// SchedulingFrozen reports whether scheduling is frozen.
func (e *Engine) SchedulingFrozen() bool {
	return e.schedFrozen > 0
}

// SwitchIfDue gives the scheduler its turn now, if it is due at the
// coming instruction boundary (and scheduling isn't frozen), instead of
// at the start of the next Step: so a run loop that looks at the next
// instruction before running it (a breakpoint at its PC, the trace) sees
// the process that will run it. Step then has nothing left to do for the
// scheduler at this boundary. An error is the scheduler's.
func (e *Engine) SwitchIfDue() error {
	// The usual case, one test, small enough to be inlined into the run
	// loops: not due, or no scheduler at all (noSchedulerBudget).
	if e.schedLeft > 0 {
		return nil
	}

	return e.switchIfDue()
}

// switchIfDue is SwitchIfDue's rarer half, kept out of line so that
// SwitchIfDue stays small enough to inline.
//
//go:noinline
func (e *Engine) switchIfDue() error {
	if e.scheduler == nil || e.schedFrozen > 0 {
		return nil
	}

	return e.schedule()
}
