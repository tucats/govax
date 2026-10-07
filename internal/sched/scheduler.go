package sched

import (
	"fmt"
	"slices"
)

// Handle names a process to the Scheduler. The caller chooses the values
// (govax uses process IDs); the Scheduler only compares them.
type Handle uint32

// process is what the Scheduler knows about one process: VMS keeps the
// same things in the software PCB (process control block).
type process struct {
	// state is the scheduling state: StateCUR, StateCOM, or a wait.
	state State

	// resource is what a StateMWAIT process waits for.
	resource Resource

	// base is the base priority, which only $SETPRI changes; priority
	// is the current priority, which boosts raise and scheduling lowers
	// back toward base.
	base, priority int

	// quantumLeft is how many more instructions the process may run
	// before its quantum ends.
	quantumLeft int

	// cpu counts the instructions the process has executed.
	cpu uint64
}

// Info is a snapshot of one process's scheduling data, for $GETJPI and
// SHOW SYSTEM.
type Info struct {
	State       State
	Resource    Resource
	Base        int
	Priority    int
	QuantumLeft int
	CPU         uint64
}

// Scheduler holds every process's scheduling state and decides which
// one runs. It isn't safe for concurrent use; govax runs every process
// on one goroutine, as one VAX CPU runs them.
type Scheduler struct {
	// processes are all the processes, by handle.
	processes map[Handle]*process

	// queues are the computable processes, one FIFO queue per current
	// priority: a process joins at the back and the head is chosen
	// first. VMS keeps the same 32 queues (section 10.1.3.1).
	queues [Priorities][]Handle

	// current is the process the CPU is running, if hasCurrent.
	current    Handle
	hasCurrent bool

	// cur is current's record, or nil, so the per-instruction Tick
	// needn't look it up.
	cur *process

	// quantum is a full quantum, in instructions.
	quantum int

	// reschedule is set when the current process should give up the CPU
	// at the next chance: it waited, was preempted, or used its quantum.
	reschedule bool
}

// New returns a Scheduler with no processes, whose quantum is quantum
// instructions (at least 1).
func New(quantum int) *Scheduler {
	return &Scheduler{processes: map[Handle]*process{}, quantum: max(quantum, 1)}
}

// Quantum returns a full quantum, in instructions.
func (s *Scheduler) Quantum() int {
	return s.quantum
}

// SetQuantum changes a full quantum to n instructions (at least 1). A
// process with more than that left of its quantum is cut to the new
// length, so the change takes effect at once (the console sets the
// quantum after the system's first process exists).
func (s *Scheduler) SetQuantum(n int) {
	s.quantum = max(n, 1)

	for _, p := range s.processes {
		p.quantumLeft = min(p.quantumLeft, s.quantum)
	}
}

// checkPriority reports an error for a priority outside 0-31.
func checkPriority(p int) error {
	if p < MinPriority || p > MaxPriority {
		return fmt.Errorf("priority %d is outside %d-%d", p, MinPriority, MaxPriority)
	}

	return nil
}

// lookup returns h's record, or an error if there's no such process.
func (s *Scheduler) lookup(h Handle) (*process, error) {
	p := s.processes[h]
	if p == nil {
		return nil, fmt.Errorf("no process %d in the scheduler", h)
	}

	return p, nil
}

// Add adds a new process with base priority base, computable, with a
// full quantum. Its current priority is boosted by class as if an event
// had made it computable: VMS boosts a created process by
// ClassProcessCreation; ClassNull adds it at its base priority. Adding a
// process may request a reschedule, as any process becoming computable
// can (see Ready).
func (s *Scheduler) Add(h Handle, base int, class Class) error {
	if err := checkPriority(base); err != nil {
		return err
	}

	if s.processes[h] != nil {
		return fmt.Errorf("process %d is already in the scheduler", h)
	}

	p := &process{base: base, priority: boosted(base, base, class.Boost()), quantumLeft: s.quantum}
	s.processes[h] = p
	s.makeComputable(h, p)

	return nil
}

// Remove forgets a deleted process. If it was the current process, there
// is none until the next Reschedule, which is requested.
func (s *Scheduler) Remove(h Handle) error {
	p, err := s.lookup(h)
	if err != nil {
		return err
	}

	s.leaveState(h, p)
	delete(s.processes, h)

	return nil
}

// leaveState takes a process out of whatever its state holds it in: the
// current-process slot or a computable queue. (A waiting process is in
// no queue; its state alone says it waits.)
func (s *Scheduler) leaveState(h Handle, p *process) {
	switch p.state {
	case StateCUR:
		s.hasCurrent = false
		s.cur = nil
		s.reschedule = true

	case StateCOM:
		q := s.queues[p.priority]
		if i := slices.Index(q, h); i >= 0 {
			s.queues[p.priority] = slices.Delete(q, i, i+1)
		}
	}
}

// makeComputable puts a process at the back of its priority's queue,
// and requests a reschedule if it should preempt the current process:
// when there is none, or when the newcomer's current priority is higher
// than or *equal to* the current process's. (Equal, by the book, section
// 10.2.3, step 4: the current process then goes to the back of the same
// queue, behind the newcomer.) It returns whether it requested one.
func (s *Scheduler) makeComputable(h Handle, p *process) bool {
	p.state = StateCOM
	p.resource = ResourceNone
	s.queues[p.priority] = append(s.queues[p.priority], h)

	if !s.hasCurrent || p.priority >= s.cur.priority {
		s.reschedule = true

		return true
	}

	return false
}

// Wait puts a process into a wait state (StateLEF, StateHIB, ...; for
// StateMWAIT, resource says what for). The process is usually the
// current one, which then gives up the CPU: a reschedule is requested.
// A computable process can be made to wait too ($SUSPND of another
// process), and a waiting one moved to another wait.
func (s *Scheduler) Wait(h Handle, state State, resource Resource) error {
	if !state.IsWait() {
		return fmt.Errorf("%s is not a wait state", state)
	}

	p, err := s.lookup(h)
	if err != nil {
		return err
	}

	s.leaveState(h, p)
	p.state = state
	p.resource = ResourceNone

	if state == StateMWAIT {
		p.resource = resource
	}

	return nil
}

// Ready reports that an event ended a waiting process's wait: it becomes
// computable, its priority boosted by the event's class (see boosted).
// It returns whether that requested a reschedule (the process should
// preempt the current one). A process that isn't waiting is left alone
// and Ready returns false: VMS ignores such events too (section 10.2.3).
func (s *Scheduler) Ready(h Handle, class Class) (bool, error) {
	p, err := s.lookup(h)
	if err != nil {
		return false, err
	}

	if !p.state.IsWait() {
		return false, nil
	}

	p.priority = boosted(p.base, p.priority, class.Boost())

	return s.makeComputable(h, p), nil
}

// Tick counts one instruction executed by the current process, and
// returns whether a reschedule is due: Charge(1).
func (s *Scheduler) Tick() bool {
	return s.Charge(1)
}

// Charge counts n instructions executed by the current process, and
// returns whether a reschedule is due. The engine charges instructions
// in batches, calling only when the current process's quantum may have
// run out (QuantumLeft says when) or something else needs a decision.
//
// When a quantum runs out, the process gets a fresh one, and a normal
// process is rescheduled: it goes to the back of its queue, so another
// computable process of the same priority gets a turn (round robin). If
// there is none, Reschedule chooses it again, one priority lower if it
// was above its base. A real-time process just goes on: it has no
// quantum end (section 10.1.2.1). (Instructions charged past the end of
// a quantum aren't carried into the next one.)
func (s *Scheduler) Charge(n int) bool {
	p := s.cur
	if p == nil || n <= 0 {
		return s.reschedule
	}

	p.cpu += uint64(n)
	p.quantumLeft -= n

	if p.quantumLeft <= 0 {
		p.quantumLeft = s.quantum
		if !IsRealTime(p.base) {
			s.reschedule = true
		}
	}

	return s.reschedule
}

// QuantumLeft returns how many instructions the current process has
// left of its quantum, or a full quantum if there is no current process.
func (s *Scheduler) QuantumLeft() int {
	if s.cur == nil {
		return s.quantum
	}

	return s.cur.quantumLeft
}

// RequestReschedule asks for the current process to be rescheduled at
// the next chance, as if its quantum had ended: it goes to the back of
// its queue, and the highest-priority computable process runs (VMS's
// SCH$RESCHED request, section 10.3).
func (s *Scheduler) RequestReschedule() {
	s.reschedule = true
}

// RescheduleRequested reports whether the current process should give
// up the CPU at the next chance.
func (s *Scheduler) RescheduleRequested() bool {
	return s.reschedule
}

// Reschedule chooses the process to run, makes it current, and returns
// it. It returns false when no process is computable: the CPU idles
// (VMS's "null process") until an event makes one computable.
//
// It does VMS's two scheduling steps (section 10.3):
//
//  1. If there is a current process (it was preempted, or its quantum
//     ended), it goes to the back of its priority's queue.
//  2. The head of the highest non-empty queue is taken out and becomes
//     current. If it's a normal process above its base priority, its
//     priority first drops by one, so it runs at the priority of the
//     queue it will go back to.
//
// The result can be the same process as before; then the caller has no
// context to switch.
func (s *Scheduler) Reschedule() (Handle, bool) {
	s.reschedule = false

	if s.hasCurrent {
		s.cur.state = StateCOM
		s.queues[s.cur.priority] = append(s.queues[s.cur.priority], s.current)
		s.hasCurrent = false
		s.cur = nil
	}

	for pri := MaxPriority; pri >= MinPriority; pri-- {
		q := s.queues[pri]
		if len(q) == 0 {
			continue
		}

		h := q[0]
		s.queues[pri] = q[1:]
		p := s.processes[h]

		if !IsRealTime(p.base) && p.priority > p.base {
			p.priority--
		}

		p.state = StateCUR
		s.current, s.hasCurrent, s.cur = h, true, p

		return h, true
	}

	return 0, false
}

// Choose makes h the current process now, outside the usual choice: for
// the console taking the CPU back for process 1 after a run stopped in
// another (docs/PHASE-44.md, subtask 9). The process that was current
// goes to the back of its queue, keeping its priority and the rest of its
// quantum. A computable h becomes current, its priority unchanged. A
// waiting h stays waiting, and then there is no current process and a
// reschedule is requested, so the next choice decides who runs.
func (s *Scheduler) Choose(h Handle) error {
	p, err := s.lookup(h)
	if err != nil {
		return err
	}

	if s.hasCurrent {
		if s.current == h {
			return nil
		}

		s.cur.state = StateCOM
		s.queues[s.cur.priority] = append(s.queues[s.cur.priority], s.current)
		s.hasCurrent, s.cur = false, nil
	}

	if p.state != StateCOM {
		s.reschedule = true

		return nil
	}

	s.leaveState(h, p)
	p.state = StateCUR
	s.current, s.hasCurrent, s.cur = h, true, p

	return nil
}

// Current returns the current process, and false if there is none.
func (s *Scheduler) Current() (Handle, bool) {
	if !s.hasCurrent {
		return 0, false
	}

	return s.current, true
}

// SetBasePriority changes a process's base priority, as $SETPRI does.
// Its current priority becomes the new base. A computable process moves
// to its new priority's queue (at the back). A reschedule is requested if
// the change means another process should run: the changed process is
// computable and now preempts the current one, or it is current and a
// computable process now has a higher priority than it.
//
// (Table 10-3 lists a boost of 2 for "Set Priority" too; how $SETPRI
// combines it with the new base is settled in Phase 44's subtask 6.)
func (s *Scheduler) SetBasePriority(h Handle, base int) error {
	if err := checkPriority(base); err != nil {
		return err
	}

	p, err := s.lookup(h)
	if err != nil {
		return err
	}

	switch p.state {
	case StateCOM:
		s.leaveState(h, p)
		p.base, p.priority = base, base
		s.makeComputable(h, p)

	case StateCUR:
		p.base, p.priority = base, base
		if s.highestComputable() > base {
			s.reschedule = true
		}

	default:
		p.base, p.priority = base, base
	}

	return nil
}

// highestComputable returns the highest priority with a computable
// process, or -1 if none is computable.
func (s *Scheduler) highestComputable() int {
	for pri := MaxPriority; pri >= MinPriority; pri-- {
		if len(s.queues[pri]) > 0 {
			return pri
		}
	}

	return -1
}

// Info returns a snapshot of a process's scheduling data, and false if
// there is no such process.
func (s *Scheduler) Info(h Handle) (Info, bool) {
	p := s.processes[h]
	if p == nil {
		return Info{}, false
	}

	return Info{
		State:       p.state,
		Resource:    p.resource,
		Base:        p.base,
		Priority:    p.priority,
		QuantumLeft: p.quantumLeft,
		CPU:         p.cpu,
	}, true
}

// Handles returns every process's handle, in increasing order.
func (s *Scheduler) Handles() []Handle {
	hs := make([]Handle, 0, len(s.processes))
	for h := range s.processes {
		hs = append(hs, h)
	}

	slices.Sort(hs)

	return hs
}

// Computable returns the computable processes in the order Reschedule
// would consider them (highest priority first, each queue front to
// back), not counting the current process.
func (s *Scheduler) Computable() []Handle {
	var hs []Handle

	for pri := MaxPriority; pri >= MinPriority; pri-- {
		hs = append(hs, s.queues[pri]...)
	}

	return hs
}
