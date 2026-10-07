package corevms

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/sched"
)

// The System's scheduler (docs/PHASE-44.md). The rules of who runs next
// are internal/sched's; the System keeps a sched.Scheduler in step with
// its process table (every process in the table is in the scheduler,
// named by its PID) and is the engine's cpu.Scheduler hook, installed
// by the console when vax.process.scheduler is on.

// ErrNoComputableProcess is what Schedule returns when every process is
// waiting. Until the idle loop (VMS's "null process") exists, nothing
// lets a process wait in the scheduler, so it can't happen in practice.
var ErrNoComputableProcess = errors.New("corevms: no process is computable")

// ErrProcessSwitch is what Schedule returns when the scheduler chooses a
// process other than the current one, until switching processes is
// implemented (Phase 44, subtask 3).
var ErrProcessSwitch = errors.New("corevms: switching processes isn't implemented yet")

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
// the next instruction boundary, until it is.
func (sys *System) Schedule(_ *cpu.Engine, ran int, preemptible bool) (int, error) {
	s := sys.sched
	s.Charge(ran)

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
		return 0, ErrNoComputableProcess
	}

	if cur := sys.Current(); cur == nil || handle(cur) != h {
		return 0, fmt.Errorf("%w (process %08X chosen)", ErrProcessSwitch, uint32(h))
	}

	return s.QuantumLeft(), nil
}
