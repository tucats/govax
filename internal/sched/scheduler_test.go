package sched

import (
	"slices"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestStateCodes checks the State values against VMS's SCH$C_* symbols.
func TestStateCodes(t *testing.T) {
	for s := StateCOLPG; s <= StateCUR; s++ {
		name := "SCH$C_" + s.String()

		v, ok := vmsdef.Symbols[name]
		if !ok {
			t.Errorf("%s isn't in vmsdef.Symbols", name)

			continue
		}

		if int64(v) != int64(s) {
			t.Errorf("%s = %d, State is %d", name, v, s)
		}
	}
}

// TestStateNames checks the names, IsWait, and resource waits' names.
func TestStateNames(t *testing.T) {
	waits := []State{StateCOLPG, StateMWAIT, StateCEF, StatePFW, StateLEF, StateLEFO,
		StateHIB, StateHIBO, StateSUSP, StateSUSPO, StateFPG}

	for s := StateCOLPG; s <= StateCUR; s++ {
		if got, want := s.IsWait(), slices.Contains(waits, s); got != want {
			t.Errorf("%s.IsWait() = %v, want %v", s, got, want)
		}
	}

	if State(0).IsWait() || State(0).String() != "State(0)" {
		t.Errorf("State(0): IsWait %v, String %q", State(0).IsWait(), State(0))
	}

	tests := []struct {
		state    State
		resource Resource
		want     string
	}{
		{StateLEF, ResourceNone, "LEF"},
		{StateHIB, ResourceMailbox, "HIB"},
		{StateMWAIT, ResourceNone, "MWAIT"},
		{StateMWAIT, ResourceMailbox, "RWMBX"},
		{StateMWAIT, ResourceAST, "RWAST"},
		{StateMWAIT, Resource(99), "MWAIT"},
	}

	if ResourceNone.String() != "NONE" {
		t.Errorf("ResourceNone is %q", ResourceNone)
	}

	for _, tt := range tests {
		if got := StateName(tt.state, tt.resource); got != tt.want {
			t.Errorf("StateName(%s, %d) = %q, want %q", tt.state, tt.resource, got, tt.want)
		}
	}
}

// TestBoosted checks the boost rule's three steps and Table 10-3's
// increments.
func TestBoosted(t *testing.T) {
	tests := []struct {
		name                 string
		base, current, boost int
		want                 int
	}{
		{"increment added to base", 4, 4, 2, 6},
		{"terminal input", 4, 4, ClassTerminalInput.Boost(), 10},
		{"higher current kept", 4, 8, 3, 8},
		{"from base, not current", 4, 6, 3, 7},
		{"no increment", 4, 6, 0, 6},
		{"result 15 allowed", 9, 9, 6, 15},
		{"above 15 falls to base", 10, 10, 6, 10},
		{"base 14 gets nothing", 14, 14, 2, 14},
		{"real-time never boosted", 18, 18, 6, 18},
		{"real-time at 16", 16, 16, 0, 16},
	}

	for _, tt := range tests {
		if got := boosted(tt.base, tt.current, tt.boost); got != tt.want {
			t.Errorf("%s: boosted(%d, %d, %d) = %d, want %d",
				tt.name, tt.base, tt.current, tt.boost, got, tt.want)
		}
	}

	increments := []struct {
		class Class
		boost int
	}{
		{ClassNull, 0}, {ClassIOCompletion, 2}, {ClassResourceAvailable, 3}, {ClassTimer, 3},
		{ClassTerminalOutput, 4}, {ClassTerminalInput, 6}, {ClassProcessCreation, 6},
		{Class(-1), 0}, {classCount, 0},
	}

	for _, tt := range increments {
		if got := tt.class.Boost(); got != tt.boost {
			t.Errorf("%s.Boost() = %d, want %d", tt.class, got, tt.boost)
		}
	}
}

// newScheduler returns a Scheduler with quantum q and the given
// processes added at their base priorities, without boosts; process i
// gets handle i+1.
func newScheduler(t *testing.T, q int, bases ...int) *Scheduler {
	t.Helper()

	s := New(q)
	for i, b := range bases {
		if err := s.Add(Handle(i+1), b, ClassNull); err != nil {
			t.Fatal(err)
		}
	}

	return s
}

// run executes n instructions the way the engine will: one Tick per
// instruction, and a Reschedule whenever one is due. It returns the
// process that ran each instruction (0 for idle).
func run(s *Scheduler, n int) []Handle {
	ran := make([]Handle, 0, n)

	for range n {
		if s.RescheduleRequested() {
			s.Reschedule()
		}

		h, _ := s.Current()
		ran = append(ran, h)
		s.Tick()
	}

	return ran
}

// mustInfo returns h's Info, failing the test if there is none.
func mustInfo(t *testing.T, s *Scheduler, h Handle) Info {
	t.Helper()

	info, ok := s.Info(h)
	if !ok {
		t.Fatalf("no process %d", h)
	}

	return info
}

// TestScheduling runs small workloads and checks which process ran each
// instruction.
func TestScheduling(t *testing.T) {
	tests := []struct {
		name    string
		quantum int
		bases   []int
		n       int
		want    []Handle
	}{
		{
			name: "round robin among equals", quantum: 2, bases: []int{4, 4, 4}, n: 12,
			want: []Handle{1, 1, 2, 2, 3, 3, 1, 1, 2, 2, 3, 3},
		},
		{
			name: "highest priority runs", quantum: 2, bases: []int{4, 6, 5}, n: 6,
			want: []Handle{2, 2, 2, 2, 2, 2},
		},
		{
			name: "quantum end with no competitor keeps the CPU", quantum: 2, bases: []int{4}, n: 5,
			want: []Handle{1, 1, 1, 1, 1},
		},
		{
			name: "quantum end with a lower competitor keeps the CPU", quantum: 2, bases: []int{4, 3}, n: 5,
			want: []Handle{1, 1, 1, 1, 1},
		},
		{
			name: "real-time has no quantum end", quantum: 2, bases: []int{20, 20}, n: 6,
			want: []Handle{1, 1, 1, 1, 1, 1},
		},
		{
			name: "no process idles", quantum: 2, n: 2,
			want: []Handle{0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newScheduler(t, tt.quantum, tt.bases...)
			if got := run(s, tt.n); !slices.Equal(got, tt.want) {
				t.Errorf("ran %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDecay checks that a boosted normal process loses one priority each
// time it's chosen, down to its base and no further.
func TestDecay(t *testing.T) {
	s := New(1)
	if err := s.Add(1, 4, ClassTerminalInput); err != nil {
		t.Fatal(err)
	}

	if got := mustInfo(t, s, 1).Priority; got != 10 {
		t.Fatalf("priority after creation boost %d, want 10", got)
	}

	// Each instruction is a whole quantum, so each is a fresh choice.
	var got []int

	for range 9 {
		run(s, 1)
		got = append(got, mustInfo(t, s, 1).Priority)
	}

	want := []int{9, 8, 7, 6, 5, 4, 4, 4, 4}
	if !slices.Equal(got, want) {
		t.Errorf("priorities %v, want %v", got, want)
	}

	if info := mustInfo(t, s, 1); info.Base != 4 || info.CPU != 9 || info.State != StateCUR {
		t.Errorf("info %+v", info)
	}
}

// TestRealTimeNoDecay checks that a real-time process's priority stays
// at its base.
func TestRealTimeNoDecay(t *testing.T) {
	s := newScheduler(t, 1, 20)
	run(s, 1)

	if err := s.Wait(1, StateHIB, ResourceNone); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Ready(1, ClassTerminalInput); err != nil {
		t.Fatal(err)
	}

	run(s, 3)

	if got := mustInfo(t, s, 1).Priority; got != 20 {
		t.Errorf("priority %d, want 20", got)
	}
}

// TestWaitAndReady checks that a waiting process gives the CPU up, that
// an event makes it computable with its boost, and when that preempts.
func TestWaitAndReady(t *testing.T) {
	s := newScheduler(t, 100, 4, 4)
	run(s, 1) // process 1 is current

	if err := s.Wait(1, StateHIB, ResourceNone); err != nil {
		t.Fatal(err)
	}

	if !s.RescheduleRequested() {
		t.Fatal("waiting didn't request a reschedule")
	}

	if info := mustInfo(t, s, 1); info.State != StateHIB {
		t.Fatalf("state %s, want HIB", info.State)
	}

	if got := run(s, 2); !slices.Equal(got, []Handle{2, 2}) {
		t.Fatalf("ran %v, want process 2", got)
	}

	// A $WAKE: boost 3, so process 1 (now 7) preempts process 2 (4).
	preempt, err := s.Ready(1, ClassResourceAvailable)
	if err != nil {
		t.Fatal(err)
	}

	if !preempt {
		t.Error("a higher-priority process didn't preempt")
	}

	if got := run(s, 1); got[0] != 1 {
		t.Errorf("ran %v, want process 1", got)
	}

	// Chosen at 7, it runs at 6.
	if got := mustInfo(t, s, 1).Priority; got != 6 {
		t.Errorf("priority %d, want 6", got)
	}

	// An event for a process that isn't waiting is ignored.
	preempt, err = s.Ready(2, ClassTerminalInput)
	if err != nil || preempt {
		t.Errorf("Ready of a computable process: %v, %v", preempt, err)
	}

	if got := mustInfo(t, s, 2).Priority; got != 4 {
		t.Errorf("ignored event changed the priority to %d", got)
	}
}

// TestPreemptionTest checks when a process made computable preempts the
// current one: at a higher or equal priority, not a lower one.
func TestPreemptionTest(t *testing.T) {
	tests := []struct {
		name    string
		waiter  int // the waiting process's base priority
		class   Class
		preempt bool
	}{
		{"higher", 6, ClassNull, true},
		{"equal", 5, ClassNull, true},
		{"lower", 4, ClassNull, false},
		{"boosted above", 4, ClassIOCompletion, true},
		{"real-time over normal", 16, ClassNull, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newScheduler(t, 100, tt.waiter, 5)
			if err := s.Wait(1, StateLEF, ResourceNone); err != nil {
				t.Fatal(err)
			}

			run(s, 1) // process 2 is current

			if h, _ := s.Current(); h != 2 {
				t.Fatalf("current %d, want 2", h)
			}

			preempt, err := s.Ready(1, tt.class)
			if err != nil {
				t.Fatal(err)
			}

			if preempt != tt.preempt || s.RescheduleRequested() != tt.preempt {
				t.Errorf("preempt %v, requested %v, want %v", preempt, s.RescheduleRequested(), tt.preempt)
			}

			// An equal-priority newcomer runs first; the preempted
			// process waits behind it.
			if tt.preempt {
				if got := run(s, 1); got[0] != 1 {
					t.Errorf("ran %v, want process 1", got)
				}
			}
		})
	}
}

// TestIdleAndWake checks that with every process waiting the CPU idles,
// and an event ends the idling.
func TestIdleAndWake(t *testing.T) {
	s := newScheduler(t, 100, 4)
	run(s, 1)

	if err := s.Wait(1, StateMWAIT, ResourceMailbox); err != nil {
		t.Fatal(err)
	}

	if info := mustInfo(t, s, 1); info.Resource != ResourceMailbox || StateName(info.State, info.Resource) != "RWMBX" {
		t.Errorf("info %+v", info)
	}

	if got := run(s, 2); !slices.Equal(got, []Handle{0, 0}) {
		t.Errorf("ran %v, want idle", got)
	}

	if preempt, _ := s.Ready(1, ClassResourceAvailable); !preempt {
		t.Error("an event while idle didn't request a reschedule")
	}

	if info := mustInfo(t, s, 1); info.Resource != ResourceNone {
		t.Errorf("resource %s kept after the wait", info.Resource)
	}

	if got := run(s, 1); got[0] != 1 {
		t.Errorf("ran %v, want process 1", got)
	}
}

// TestQuantumKept checks that a process that waits keeps the rest of
// its quantum, and that quantum end gives a fresh one.
func TestQuantumKept(t *testing.T) {
	s := newScheduler(t, 5, 4, 4)
	run(s, 3) // process 1 runs 3 of its 5

	if err := s.Wait(1, StateLEF, ResourceNone); err != nil {
		t.Fatal(err)
	}

	if got := mustInfo(t, s, 1).QuantumLeft; got != 2 {
		t.Errorf("quantum left %d, want 2", got)
	}

	if got := run(s, 1); got[0] != 2 {
		t.Fatalf("ran %v, want process 2", got)
	}

	if _, err := s.Ready(1, ClassNull); err != nil {
		t.Fatal(err)
	}

	// Process 1 preempted process 2 (equal priority) at once, and has
	// two instructions left before process 2's turn.
	if got := run(s, 4); !slices.Equal(got, []Handle{1, 1, 2, 2}) {
		t.Errorf("ran %v", got)
	}

	if got := mustInfo(t, s, 1).QuantumLeft; got != 5 {
		t.Errorf("quantum left %d, want a fresh 5", got)
	}
}

// TestRemove checks deleting the current process and a computable one.
func TestRemove(t *testing.T) {
	s := newScheduler(t, 100, 4, 4, 4)
	run(s, 1)

	if err := s.Remove(3); err != nil {
		t.Fatal(err)
	}

	if s.RescheduleRequested() {
		t.Error("removing a computable process requested a reschedule")
	}

	if err := s.Remove(1); err != nil {
		t.Fatal(err)
	}

	if _, ok := s.Current(); ok || !s.RescheduleRequested() {
		t.Error("removing the current process left it current")
	}

	if got := run(s, 2); !slices.Equal(got, []Handle{2, 2}) {
		t.Errorf("ran %v, want process 2", got)
	}

	if got := s.Handles(); !slices.Equal(got, []Handle{2}) {
		t.Errorf("handles %v", got)
	}

	if _, ok := s.Info(1); ok {
		t.Error("Info of a removed process")
	}
}

// TestSetBasePriority checks $SETPRI's scheduling effects.
func TestSetBasePriority(t *testing.T) {
	s := newScheduler(t, 100, 6, 4, 4)
	run(s, 1)

	// A computable process raised above the current one preempts it.
	if err := s.SetBasePriority(2, 8); err != nil {
		t.Fatal(err)
	}

	if !s.RescheduleRequested() {
		t.Error("raising a computable process didn't preempt")
	}

	if got := run(s, 1); got[0] != 2 {
		t.Errorf("ran %v, want 2", got)
	}

	// The current process lowering itself below a computable one (1,
	// at 6) gives up the CPU; lowering to equal wouldn't.
	if err := s.SetBasePriority(2, 6); err != nil {
		t.Fatal(err)
	}

	if s.RescheduleRequested() {
		t.Error("lowering to an equal priority requested a reschedule")
	}

	if err := s.SetBasePriority(2, 5); err != nil {
		t.Fatal(err)
	}

	if !s.RescheduleRequested() {
		t.Error("lowering below a computable process didn't reschedule")
	}

	if got := run(s, 1); got[0] != 1 {
		t.Errorf("ran %v, want 1", got)
	}

	if want := []Handle{2, 3}; !slices.Equal(s.Computable(), want) {
		t.Errorf("computable %v, want %v", s.Computable(), want)
	}

	// A waiting process just takes the new priority.
	if err := s.Wait(3, StateSUSP, ResourceNone); err != nil {
		t.Fatal(err)
	}

	if err := s.SetBasePriority(3, 9); err != nil {
		t.Fatal(err)
	}

	if info := mustInfo(t, s, 3); info.Base != 9 || info.Priority != 9 || info.State != StateSUSP {
		t.Errorf("info %+v", info)
	}
}

// TestErrors checks the calls' refusals.
func TestErrors(t *testing.T) {
	s := newScheduler(t, 100, 4)

	if err := s.Add(1, 4, ClassNull); err == nil {
		t.Error("Add of an existing handle")
	}

	for _, p := range []int{-1, 32} {
		if err := s.Add(9, p, ClassNull); err == nil {
			t.Errorf("Add at priority %d", p)
		}

		if err := s.SetBasePriority(1, p); err == nil {
			t.Errorf("SetBasePriority to %d", p)
		}
	}

	if err := s.Wait(1, StateCOM, ResourceNone); err == nil {
		t.Error("Wait in COM")
	}

	if err := s.Wait(9, StateLEF, ResourceNone); err == nil {
		t.Error("Wait of an unknown process")
	}

	if _, err := s.Ready(9, ClassNull); err == nil {
		t.Error("Ready of an unknown process")
	}

	if err := s.Remove(9); err == nil {
		t.Error("Remove of an unknown process")
	}

	if err := s.SetBasePriority(9, 4); err == nil {
		t.Error("SetBasePriority of an unknown process")
	}
}

// TestQuantumSetting checks New's and SetQuantum's floor of 1.
func TestQuantumSetting(t *testing.T) {
	s := New(0)
	if s.Quantum() != 1 {
		t.Errorf("New(0) quantum %d", s.Quantum())
	}

	s.SetQuantum(300)
	if s.Quantum() != 300 {
		t.Errorf("quantum %d", s.Quantum())
	}

	// A quantum under way is cut to a shorter new length, not lengthened.
	if err := s.Add(1, 4, ClassNull); err != nil {
		t.Fatal(err)
	}

	s.SetQuantum(50)

	if got := mustInfo(t, s, 1).QuantumLeft; got != 50 {
		t.Errorf("quantum left %d, want 50", got)
	}

	s.SetQuantum(80)

	if got := mustInfo(t, s, 1).QuantumLeft; got != 50 {
		t.Errorf("quantum left %d, want 50 still", got)
	}
}

// TestFigure10_2 follows the start of the book's Figure 10-2 (section
// 10.1.2.4): a compute-bound process A at base 4, boosted to 10, loses a
// priority each time it's chosen, and an I/O-bound process B at base 4
// boosted by a wakeup preempts A when it's at or above A's priority.
func TestFigure10_2(t *testing.T) {
	const a, b = 1, 2

	s := New(2)
	if err := s.Add(a, 4, ClassTerminalInput); err != nil { // A at 10
		t.Fatal(err)
	}

	if err := s.Add(b, 4, ClassNull); err != nil {
		t.Fatal(err)
	}

	if err := s.Wait(b, StateHIB, ResourceNone); err != nil {
		t.Fatal(err)
	}

	run(s, 2) // A chosen at 10, runs at 9

	if got := mustInfo(t, s, a).Priority; got != 9 {
		t.Fatalf("A at %d, want 9", got)
	}

	run(s, 2) // A's quantum ends; chosen again, at 8

	if got := mustInfo(t, s, a).Priority; got != 8 {
		t.Fatalf("A at %d, want 8", got)
	}

	// B wakes with a boost of 6 (terminal input): 10 preempts A's 8.
	if preempt, _ := s.Ready(b, ClassTerminalInput); !preempt {
		t.Fatal("B didn't preempt A")
	}

	if got := run(s, 1); got[0] != b {
		t.Fatalf("ran %v, want B", got)
	}

	if got := mustInfo(t, s, b).Priority; got != 9 {
		t.Errorf("B at %d, want 9", got)
	}

	// A was preempted, not chosen, so it kept 8, in the computable queue.
	if info := mustInfo(t, s, a); info.Priority != 8 || info.State != StateCOM {
		t.Errorf("A %+v", info)
	}
}

// TestRequestReschedule: a requested reschedule puts the current
// process behind an equal one.
func TestRequestReschedule(t *testing.T) {
	s := newScheduler(t, 100, 4, 4)
	run(s, 1)

	s.RequestReschedule()

	if got := run(s, 1); got[0] != 2 {
		t.Errorf("ran %v, want process 2", got)
	}
}

// TestCharge checks charging instructions in batches: the CPU count,
// QuantumLeft, and a quantum end inside a batch.
func TestCharge(t *testing.T) {
	s := newScheduler(t, 10, 4, 4)

	if got := s.QuantumLeft(); got != 10 {
		t.Errorf("QuantumLeft with no current process %d, want 10", got)
	}

	if s.Charge(5) != true {
		t.Error("no reschedule pending before the first choice")
	}

	s.Reschedule()

	if s.Charge(4) || s.QuantumLeft() != 6 {
		t.Errorf("after 4: reschedule %v, quantum left %d", s.RescheduleRequested(), s.QuantumLeft())
	}

	if s.Charge(0) {
		t.Error("Charge(0) requested a reschedule")
	}

	if !s.Charge(7) {
		t.Error("a batch past the quantum's end didn't reschedule")
	}

	if info := mustInfo(t, s, 1); info.CPU != 11 || info.QuantumLeft != 10 {
		t.Errorf("info %+v", info)
	}

	if h, _ := s.Reschedule(); h != 2 {
		t.Errorf("chose %d, want 2", h)
	}
}
