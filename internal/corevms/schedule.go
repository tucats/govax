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

	// A waiting process whose wait is over becomes computable now, which
	// may preempt the current one (waits.go).
	sys.wakeWaiters()

	if !s.RescheduleRequested() {
		return s.QuantumLeft(), nil
	}

	// No current process in the scheduler means the last one waited (or
	// none has been chosen yet): a choice must be made, preemptible or
	// not.
	if _, running := s.Current(); running && !preemptible {
		return 1, nil
	}

	h, ok := s.Reschedule()
	if !ok {
		// Nobody can run: every waiter tries its wait again, as each
		// did before the scheduler (until subtask 5's idle loop).
		sys.retryWaiters()

		if h, ok = s.Reschedule(); !ok {
			return 0, ErrNoComputableProcess
		}
	}

	if cur := sys.Current(); cur == nil || handle(cur) != h {
		next, found := sys.FindProcess(uint32(h))
		if !found {
			return 0, fmt.Errorf("corevms: the scheduler chose process %08X, which isn't in the table", uint32(h))
		}

		if err := sys.switchTo(e, cur, next); err != nil {
			return 0, err
		}
	}

	return s.QuantumLeft(), nil
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
func (sys *System) switchTo(e *cpu.Engine, cur, next *Environment) error {
	if next.Stacks == nil || next.Stacks.PCB == 0 {
		return fmt.Errorf("corevms: process %08X has no hardware PCB to switch to", next.Process.PID)
	}

	// With no process to save (the current one was deleted), the CPU
	// isn't left on the interrupt stack, where LoadContext starts from.
	// Nothing deletes the current process yet; Phase 45's deletion will.
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

	return nil
}
