package machine

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// RunningProcess is the process the engine's hooks reach: the System's current
// process, the one whose context the CPU holds (docs/PHASE-44.md).
// That's process 1 unless the scheduler has switched to another;
// console commands keep using process 1 (or their own subprocess's DCL
// its process), whichever runs. nil before the first INIT.
func (m *Machine) RunningProcess() *corevms.Environment {
	return m.EngineHooks.Running()
}

// RunningProcessOne reports whether the CPU is running process 1, the
// console's (and the debugger's: docs/PHASE-43.md, Decision 11) process.
// The debugger's breakpoints, tracepoints, and watchpoints act only then
// (docs/PHASE-44.md, subtask 8): a P0 address names process 1's memory,
// and another process reaching the same address is somewhere else.
func (m *Machine) RunningProcessOne() bool {
	return m.RunningProcess() == m.processOne
}

// ProcessNote is what a stop message adds when the CPU holds a process
// other than process 1, naming it (" in process 00000302"), and "" when
// it holds process 1 (docs/PHASE-44.md, subtask 9).
func (m *Machine) ProcessNote() string {
	if env := m.RunningProcess(); env != m.processOne {
		return fmt.Sprintf(" in process %08X", env.Process.PID)
	}

	return ""
}

// ReturnToProcessOne gives the CPU back to process 1 if a run stopped in
// another process (docs/PHASE-44.md, subtask 9). After such a stop the
// CPU stays in that process, so EXAMINE and SHOW REGISTERS see it; the
// next run (GO, STEP, CALL, RUN) and the debugger's EXIT come back here,
// since the console's runs and the debugger's breakpoints are process
// 1's (docs/PHASE-43.md, Decision 11). The other process keeps its place
// in the scheduler. While scheduling is frozen (a nested run, inside
// another process's) it does nothing.
func (m *Machine) ReturnToProcessOne() error {
	if m.processOne == nil || m.Engine == nil || m.Engine.SchedulingFrozen() || m.RunningProcess() == m.processOne {
		return nil
	}

	return m.processOne.SwitchCPU(m.Engine, m.processOne)
}

// ShowProcessContext moves the CPU to env's process, for the debugger's
// SET PROCESS: its registers and memory are then what EXAMINE and SHOW
// REGISTERS see, as after a run that stopped in it. The next run gives
// the CPU back to process 1 (ReturnToProcessOne). Not while a nested run
// has scheduling frozen.
func (m *Machine) ShowProcessContext(env *corevms.Environment) error {
	if m.processOne == nil || m.Engine == nil || env == m.RunningProcess() {
		return nil
	}

	if m.Engine.SchedulingFrozen() {
		return fmt.Errorf("console: can't change process during a nested run")
	}

	return m.processOne.SwitchCPU(m.Engine, env)
}

// ProcessPC is env's PC: the CPU's if the CPU holds env's process, and
// otherwise the one saved in its hardware PCB when it last lost the CPU
// (0 if it has never had it).
func (m *Machine) ProcessPC(env *corevms.Environment) uint32 {
	if env == m.RunningProcess() {
		return m.CPU.GPR(vax.PC)
	}

	if env.Stacks == nil || env.Stacks.PCBB == 0 {
		return 0
	}

	pcb, err := cpu.ReadPCB(m.Mem, env.Stacks.PCBB)
	if err != nil {
		return 0
	}

	return pcb.PC
}
