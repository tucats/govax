package corevms

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/sched"
)

// TestScheduler_followsTable checks that the scheduler knows every
// process in the table, by PID, at its base priority, and forgets a
// removed one.
func TestScheduler_followsTable(t *testing.T) {
	env, _ := fixture()
	p2 := newProcess(t, env)
	p3 := newProcess(t, env)

	s := env.Scheduler()
	want := []sched.Handle{0x301, 0x302, 0x303}

	if got := s.Handles(); !slices.Equal(got, want) {
		t.Fatalf("scheduler has %v, want %v", got, want)
	}

	info, _ := s.Info(handle(p2))
	if info.State != sched.StateCOM || info.Base != int(p2.Process.BasePriority) {
		t.Errorf("process 2: %+v", info)
	}

	env.RemoveProcess(p3)

	if got := s.Handles(); !slices.Equal(got, want[:2]) {
		t.Errorf("after removal, scheduler has %v", got)
	}
}

// TestSetProcessSettings checks that the quantum reaches the scheduler,
// and the preempt setting's modes.
func TestSetProcessSettings(t *testing.T) {
	env, _ := fixture()

	ps := DefaultProcessSettings()
	ps.Quantum = 1234
	env.SetProcessSettings(ps)

	if env.Scheduler().Quantum() != 1234 || env.ProcessSettings.Quantum != 1234 {
		t.Errorf("quantum %d, settings %+v", env.Scheduler().Quantum(), env.ProcessSettings)
	}

	modes := map[PreemptMode]cpu.PreemptModes{
		PreemptAll:  cpu.PreemptAllModes,
		PreemptUser: cpu.PreemptUserMode,
		PreemptNone: cpu.PreemptNoModes,
	}

	for m, want := range modes {
		if got := m.Modes(); got != want {
			t.Errorf("%s.Modes() = %b, want %b", m, got, want)
		}
	}
}

// TestSchedule_oneProcess checks the hook with process 1 alone: the
// first call chooses it, instructions are charged to it, and its budget
// is the rest of its quantum, a quantum end keeping it running.
func TestSchedule_oneProcess(t *testing.T) {
	env, _ := fixture()

	ps := DefaultProcessSettings()
	ps.Quantum = 100
	env.SetProcessSettings(ps)

	next, err := env.Schedule(nil, 0, true)
	if err != nil || next != 100 {
		t.Fatalf("first call: %d, %v", next, err)
	}

	if h, ok := env.Scheduler().Current(); !ok || h != handle(env) {
		t.Fatalf("current %v, %v", h, ok)
	}

	if next, err = env.Schedule(nil, 40, true); err != nil || next != 60 {
		t.Errorf("after 40: %d, %v", next, err)
	}

	// A quantum end, not preemptible here: tried again next instruction.
	if next, err = env.Schedule(nil, 60, false); err != nil || next != 1 {
		t.Errorf("quantum end, not preemptible: %d, %v", next, err)
	}

	// Preemptible: rescheduled, chosen again. The instruction run while
	// the reschedule waited counts against the fresh quantum.
	if next, err = env.Schedule(nil, 1, true); err != nil || next != 99 {
		t.Errorf("quantum end: %d, %v", next, err)
	}

	if got := env.CPUInstructions(env); got != 101 {
		t.Errorf("CPU %d instructions, want 101", got)
	}
}

// TestSchedule_switchNeedsPCB checks that the scheduler choosing
// another process switches to it, which a process without a hardware
// PCB (the fixture's processes have no stacks) can't be.
func TestSchedule_switchNeedsPCB(t *testing.T) {
	env, _ := fixture()
	p2 := newProcess(t, env)

	ps := DefaultProcessSettings()
	ps.Quantum = 10
	env.SetProcessSettings(ps)

	if _, err := env.Schedule(nil, 0, true); err != nil {
		t.Fatalf("first call: %v", err)
	}

	_, err := env.Schedule(nil, 10, true) // quantum end: process 2's turn
	if err == nil || !strings.Contains(err.Error(), "no hardware PCB") {
		t.Errorf("err %v, want no hardware PCB", err)
	}

	if h, _ := env.Scheduler().Current(); h != handle(p2) {
		t.Errorf("scheduler chose %v, want process 2", h)
	}
}

// TestSchedule_noProcess checks the hook with every process waiting.
func TestSchedule_noProcess(t *testing.T) {
	env, _ := fixture()

	if err := env.Scheduler().Wait(handle(env), sched.StateHIB, sched.ResourceNone); err != nil {
		t.Fatal(err)
	}

	if _, err := env.Schedule(nil, 0, false); !errors.Is(err, ErrNoComputableProcess) {
		t.Errorf("err %v, want ErrNoComputableProcess", err)
	}
}
