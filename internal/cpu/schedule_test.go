package cpu_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// fakeScheduler records each Schedule call and answers with a fixed
// budget, or an error.
type fakeScheduler struct {
	ran         []int
	preemptible []bool
	next        int
	err         error
}

func (f *fakeScheduler) Schedule(_ *cpu.Engine, ran int, preemptible bool) (int, error) {
	f.ran = append(f.ran, ran)
	f.preemptible = append(f.preemptible, preemptible)

	return f.next, f.err
}

// loopEngine returns an engine whose CPU, memory mapping off, runs a
// one-instruction loop (BRB to itself) in kernel mode at IPL 0.
func loopEngine(t *testing.T) *cpu.Engine {
	t.Helper()

	c, mem := vax.New(), vm.NewMemory(1<<16)

	const origin = 0x1000
	if err := mem.StorePhysical(origin, []byte{0x11, 0xFE}); err != nil { // BRB .
		t.Fatal(err)
	}

	var psl vax.PSL // kernel mode, IPL 0, not on the interrupt stack

	c.SetPSL(psl)
	c.SetPR(vax.IPL, 0)
	c.SetGPR(vax.PC, origin)
	c.SetGPR(vax.SP, 0x8000)

	return cpu.NewEngine(c, mem)
}

// steps runs n instructions, failing the test on an error.
func steps(t *testing.T, e *cpu.Engine, n int) {
	t.Helper()

	for range n {
		if err := e.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
	}
}

// TestSchedulerCadence checks when the hook is called and what it's
// told: before the first instruction, then each time its budget runs
// out, with the count of instructions run since.
func TestSchedulerCadence(t *testing.T) {
	e := loopEngine(t)
	f := &fakeScheduler{next: 3}
	e.SetScheduler(f, cpu.PreemptAllModes)

	steps(t, e, 7)

	if want := []int{0, 3, 3}; !slices.Equal(f.ran, want) {
		t.Errorf("ran %v, want %v", f.ran, want)
	}

	// A request mid-budget calls the hook before the next instruction,
	// with the instructions actually run.
	e.RequestReschedule()
	steps(t, e, 1)

	if want := []int{0, 3, 3, 1}; !slices.Equal(f.ran, want) {
		t.Errorf("after a request, ran %v, want %v", f.ran, want)
	}

	// A budget below 1 is taken as 1.
	f.next = 0
	steps(t, e, 5) // 2 left of the 3, then 1 each

	if want := []int{0, 3, 3, 1, 3, 1, 1}; !slices.Equal(f.ran, want) {
		t.Errorf("budget 0: ran %v, want %v", f.ran, want)
	}

	// Removing the hook stops the calls.
	e.SetScheduler(nil, cpu.PreemptAllModes)
	steps(t, e, 5)

	if len(f.ran) != 7 {
		t.Errorf("hook called %d times after removal", len(f.ran)-7)
	}

	// RequestReschedule without a hook is harmless.
	e.RequestReschedule()
	steps(t, e, 1)
}

// TestSchedulerError checks that the hook's error stops Step.
func TestSchedulerError(t *testing.T) {
	e := loopEngine(t)
	stop := errors.New("stop")
	e.SetScheduler(&fakeScheduler{err: stop}, cpu.PreemptAllModes)

	if err := e.Step(); !errors.Is(err, stop) {
		t.Errorf("Step = %v, want the hook's error", err)
	}
}

// TestPreemptible checks the preemption test: below IPL 3, not on the
// interrupt stack, and in a mode the setting allows.
func TestPreemptible(t *testing.T) {
	tests := []struct {
		name  string
		mode  vax.AccessMode
		ipl   uint32
		is    bool
		modes cpu.PreemptModes
		want  bool
	}{
		{"kernel at IPL 0", vax.Kernel, 0, false, cpu.PreemptAllModes, true},
		{"user at IPL 2", vax.User, 2, false, cpu.PreemptAllModes, true},
		{"IPL 3", vax.User, 3, false, cpu.PreemptAllModes, false},
		{"IPL 31", vax.Kernel, 31, false, cpu.PreemptAllModes, false},
		{"interrupt stack", vax.Kernel, 0, true, cpu.PreemptAllModes, false},
		{"user only, user", vax.User, 0, false, cpu.PreemptUserMode, true},
		{"user only, supervisor", vax.Supervisor, 0, false, cpu.PreemptUserMode, false},
		{"user only, kernel", vax.Kernel, 0, false, cpu.PreemptUserMode, false},
		{"none", vax.User, 0, false, cpu.PreemptNoModes, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := loopEngine(t)
			e.SetScheduler(&fakeScheduler{next: 1}, tt.modes)

			var psl vax.PSL

			psl.SetCurMod(tt.mode)
			psl.SetIPL(tt.ipl)
			psl.SetIS(tt.is)
			e.CPU().SetPSL(psl)

			if got := e.Preemptible(); got != tt.want {
				t.Errorf("Preemptible() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSchedulerPreemptibleArgument checks that Step passes the test's
// result to the hook.
func TestSchedulerPreemptibleArgument(t *testing.T) {
	e := loopEngine(t)
	f := &fakeScheduler{next: 1}
	e.SetScheduler(f, cpu.PreemptUserMode) // the loop runs in kernel mode

	steps(t, e, 2)

	if want := []bool{false, false}; !slices.Equal(f.preemptible, want) {
		t.Errorf("preemptible %v, want %v", f.preemptible, want)
	}
}

// TestFreezeScheduling: while scheduling is frozen the hook isn't
// called; freezes nest; the instructions run meanwhile are charged at the
// first call after the last unfreeze, and a reschedule asked for
// meanwhile happens then.
func TestFreezeScheduling(t *testing.T) {
	e := loopEngine(t)
	f := &fakeScheduler{next: 3}
	e.SetScheduler(f, cpu.PreemptAllModes)

	steps(t, e, 1) // the first call

	outer := e.FreezeScheduling()
	inner := e.FreezeScheduling()

	e.RequestReschedule()
	steps(t, e, 10)

	inner()
	inner() // a second call changes nothing
	steps(t, e, 2)

	if len(f.ran) != 1 || !e.SchedulingFrozen() {
		t.Fatalf("hook called %d times while frozen (frozen %v)", len(f.ran)-1, e.SchedulingFrozen())
	}

	outer()
	steps(t, e, 1)

	if want := []int{0, 13}; !slices.Equal(f.ran, want) {
		t.Errorf("ran %v, want %v", f.ran, want)
	}
}

// TestSwitchIfDue: the hook is called early only when it's due at this
// boundary and scheduling isn't frozen; Step then doesn't call it again.
func TestSwitchIfDue(t *testing.T) {
	e := loopEngine(t)
	f := &fakeScheduler{next: 3}
	e.SetScheduler(f, cpu.PreemptAllModes)

	if err := e.SwitchIfDue(); err != nil || len(f.ran) != 1 {
		t.Fatalf("not called when due: %v, %d calls", err, len(f.ran))
	}

	steps(t, e, 1)

	if err := e.SwitchIfDue(); err != nil || len(f.ran) != 1 {
		t.Errorf("called when not due: %v, %d calls", err, len(f.ran))
	}

	steps(t, e, 2) // the budget is spent

	unfreeze := e.FreezeScheduling()

	if err := e.SwitchIfDue(); err != nil || len(f.ran) != 1 {
		t.Errorf("called while frozen: %d calls", len(f.ran))
	}

	unfreeze()

	if err := e.SwitchIfDue(); err != nil || len(f.ran) != 2 {
		t.Errorf("not called after unfreezing: %d calls", len(f.ran))
	}

	if err := (&cpu.Engine{}).SwitchIfDue(); err != nil {
		t.Errorf("no scheduler: %v", err)
	}
}
