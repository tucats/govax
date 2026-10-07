package corevms

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
)

// The System's scheduler (docs/PHASE-44.md). The rules of who runs next
// are internal/sched's; the System keeps a sched.Scheduler in step with
// its process table (every process in the table is in the scheduler,
// named by its PID) and is the engine's cpu.Scheduler hook, installed
// by the console when vax.process.scheduler is on.

// ErrNoComputableProcess is what Schedule returns when there is no
// process at all to run (every waiter is retried before that; see
// retryWaiters).
var ErrNoComputableProcess = errors.New("corevms: no process is computable")

// handle is the scheduler's name for env's process: its PID.
func handle(env *Environment) sched.Handle {
	return sched.Handle(env.Process.PID)
}

// Scheduler returns the System's scheduler, for SHOW SYSTEM, tests, and
// the services that make processes wait.
func (sys *System) Scheduler() *sched.Scheduler {
	return sys.sched
}

// SetProcessSettings replaces the multiprocessing settings, and gives
// the scheduler the new quantum.
func (sys *System) SetProcessSettings(ps ProcessSettings) {
	sys.ProcessSettings = ps
	sys.sched.SetQuantum(ps.Quantum)
}

// Modes returns the access modes m lets the scheduler preempt, as the
// engine's cpu.PreemptModes.
func (m PreemptMode) Modes() cpu.PreemptModes {
	switch m {
	case PreemptUser:
		return cpu.PreemptUserMode
	case PreemptNone:
		return cpu.PreemptNoModes
	default:
		return cpu.PreemptAllModes
	}
}

// CPUInstructions returns how many instructions env's process has
// executed while the scheduler was installed: its CPU time, counted in
// instructions (Decision 2's unit), for $GETJPI's JPI$_CPUTIM.
func (sys *System) CPUInstructions(env *Environment) uint64 {
	info, _ := sys.sched.Info(handle(env))

	return info.CPU
}

// Schedule is the engine's scheduling hook (cpu.Scheduler). It charges
// the instructions run to the current process, and when a reschedule is
// due and allowed, has the scheduler choose who runs next. A process
// that is waiting gives up the CPU even when preemption isn't allowed;
// a quantum end or a preemption that isn't allowed yet is tried again at
// the next instruction boundary, until it is. When the choice is another
// process, Schedule switches the CPU to it (switchTo).
func (sys *System) Schedule(e *cpu.Engine, ran int, preemptible bool) (int, error) {
	s := sys.sched
	s.Charge(ran)
	sys.accountTime()

	// Every process's due timers expire, and a waiting process whose
	// wait is over becomes computable now, which may preempt the
	// current one (waits.go).
	sys.pollEvents()

	if !s.RescheduleRequested() {
		return sys.budget(e), nil
	}

	// No current process in the scheduler means the last one waited (or
	// none has been chosen yet): a choice must be made, preemptible or
	// not.
	if _, running := s.Current(); running && !preemptible {
		return 1, nil
	}

	// A process starting for the first time may fail to (its image
	// can't be activated) and be deleted at once (startup.go): then the
	// choice is made again, before anything runs.
	for {
		h, ok := s.Reschedule()
		if !ok {
			// Nobody can run: idle until somebody can (idle.go).
			if h, ok = sys.idle(e); !ok {
				return 0, ErrNoComputableProcess
			}
		}

		cur := sys.Current()
		if cur != nil && handle(cur) == h {
			break
		}

		next, found := sys.FindProcess(uint32(h))
		if !found {
			return 0, fmt.Errorf("corevms: the scheduler chose process %08X, which isn't in the table", uint32(h))
		}

		if err := sys.switchTo(e, cur, next); err != nil {
			return 0, err
		}

		if !next.Deleted {
			break
		}
	}

	return sys.budget(e), nil
}

// budget is how many instructions may run before the scheduler is
// called again: the rest of the current process's quantum, but no later
// than the next timer of any process is due, so that a timer ends its
// process's wait on time even while another process runs (pollEvents
// expires it at that call).
func (sys *System) budget(e *cpu.Engine) int {
	n := sys.sched.QuantumLeft()

	if t, ok := sys.nextTimer(); ok && e != nil {
		n = min(n, e.InstructionsUntil(t))
	}

	return n
}

// switchTo moves the CPU from process cur to
// process next, between two instructions: what VMS's rescheduling
// interrupt does with SVPCTX and LDPCTX (VAX/VMS Internals and Data
// Structures, section 10.3).
//
//  1. cur's hardware PCB gets its state. The memory-management longwords
//     go in first (SVPCTX doesn't save them; see
//     cpu.Engine.SaveMemoryContext), then its registers, stack pointers,
//     PC, and PSL. Process 1 gets a PCB page the first time it's
//     switched out (EnsurePCB); PCBB is pointed at cur's PCB, which it
//     may not have been yet.
//  2. PCBB is pointed at next's PCB, and the CPU loads it: registers,
//     address space (with the per-process TB entries and the instruction
//     fetch window emptied), and the PC and PSL to resume at.
//  3. next becomes the System's current process, the one the system
//     service and AST hooks reach.
//  4. If cur has been deleted (delete.go), the memory it kept while the
//     CPU ran in it, its page tables, stacks, and PCB among it, is freed
//     now that the CPU has left it: what VMS does once the deleted
//     process has saved its context for the last time (VAX/VMS Internals
//     and Data Structures, section 22.2.1, steps 14 to 16). Saving its
//     context first, into a PCB about to be freed, is wasted but
//     harmless, and leaves the CPU on the interrupt stack, where loading
//     the next context starts from.
func (sys *System) switchTo(e *cpu.Engine, cur, next *Environment) error {
	if next.Stacks == nil || next.Stacks.PCB == 0 {
		return fmt.Errorf("corevms: process %08X has no hardware PCB to switch to", next.Process.PID)
	}

	// There is always a current process to save: a deleted one stays
	// current until this switch (step 4).
	if cur == nil {
		return fmt.Errorf("corevms: no current process to switch from to %08X", next.Process.PID)
	}

	pcb, err := sys.EnsurePCB(cur)
	if err != nil {
		return err
	}

	sys.cpu.SetPR(vax.PCBB, pcb)

	if err := e.SaveMemoryContext(); err != nil {
		return err
	}

	if err := e.SaveContext(); err != nil {
		return err
	}

	sys.cpu.SetPR(vax.PCBB, next.Stacks.PCBB)

	if err := e.LoadContext(); err != nil {
		return fmt.Errorf("corevms: loading process %08X: %w", next.Process.PID, err)
	}

	sys.SetCurrent(next)

	if sys.cpu.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(sys.cpu.DebugWriter(), "DEBUG(PROCESS): SWITCH %08X -> %08X, PC=%08X\n",
			cur.Process.PID, next.Process.PID, sys.cpu.GPR(vax.PC))
	}

	if cur.Deleted {
		sys.releaseMemory(cur)
	}

	// A process $CREPRC created starts the first time it runs, in its
	// own context (creprc.go).
	if next.Startup != nil {
		return sys.startProcess(e, next)
	}

	return nil
}

// SwitchCPU moves the CPU to env's process now, outside the scheduler's
// choice: the console taking the CPU back for process 1 when the user
// resumes after a run stopped in another process (docs/PHASE-44.md,
// subtask 9). The process the CPU held keeps its place in the scheduler
// and runs again when it's chosen (sched.Choose). A waiting env is
// switched to all the same (the console's commands need its context);
// the scheduler leaves it waiting and decides who runs at the next
// instruction.
func (sys *System) SwitchCPU(e *cpu.Engine, env *Environment) error {
	cur := sys.Current()
	if cur == env {
		return nil
	}

	sys.accountTime()

	if err := sys.switchTo(e, cur, env); err != nil {
		return err
	}

	if !env.Deleted {
		if err := sys.sched.Choose(handle(env)); err != nil {
			return err
		}
	}

	e.RequestReschedule()

	return nil
}
