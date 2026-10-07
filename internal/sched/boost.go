package sched

import "fmt"

// Class is a priority increment class: the kind of event that ended a
// process's wait, which decides how much its priority is boosted. The
// system routine reporting an event names its class, and the scheduler
// looks the boost up in a small table (*VAX/VMS Internals and Data
// Structures*, section 10.2.4 and Table 10-3, where the classes are the
// PRI$_* symbols).
//
// The idea: a process that waited for something, especially for a person
// at a terminal, gets the CPU sooner when it can go on, so interactive
// programs feel responsive while compute-bound ones run in the gaps.
type Class int

const (
	// ClassNull (PRI$_NULL) gives no boost: a page read completing, a
	// quantum end, and other events without one.
	ClassNull Class = iota

	// ClassIOCompletion (PRI$_IOCOM) is direct I/O, nonterminal buffered
	// I/O, or an update-section write completing: a boost of 2.
	ClassIOCompletion

	// ClassResourceAvailable (PRI$_RESAVL; also PRI$_TIMER, the same
	// class) is a resource becoming available, a $WAKE, a $RESUME, a
	// deletion, or a timer expiring: a boost of 3.
	ClassResourceAvailable

	// ClassTerminalOutput (PRI$_TOCOM) is terminal output completing: a
	// boost of 4.
	ClassTerminalOutput

	// ClassTerminalInput (PRI$_TICOM) is terminal input completing, and
	// also process creation: a boost of 6.
	ClassTerminalInput

	// classCount is how many classes there are.
	classCount
)

// ClassTimer is the class of a timer request expiring ($SETIMR,
// $SCHDWK): the same as ClassResourceAvailable (PRI$_TIMER).
const ClassTimer = ClassResourceAvailable

// ClassProcessCreation is the class a newly created process is made
// computable with: the same as ClassTerminalInput.
const ClassProcessCreation = ClassTerminalInput

// boosts are each class's priority increment (Table 10-3).
var boosts = [classCount]int{
	ClassNull:              0,
	ClassIOCompletion:      2,
	ClassResourceAvailable: 3,
	ClassTerminalOutput:    4,
	ClassTerminalInput:     6,
}

// classNames are the classes' PRI$_ names.
var classNames = [classCount]string{
	ClassNull:              "PRI$_NULL",
	ClassIOCompletion:      "PRI$_IOCOM",
	ClassResourceAvailable: "PRI$_RESAVL",
	ClassTerminalOutput:    "PRI$_TOCOM",
	ClassTerminalInput:     "PRI$_TICOM",
}

// Boost returns the class's priority increment, or 0 for an unknown
// class.
func (c Class) Boost() int {
	if c >= 0 && c < classCount {
		return boosts[c]
	}

	return 0
}

// String returns the class's PRI$_ name.
func (c Class) String() string {
	if c >= 0 && c < classCount {
		return classNames[c]
	}

	return fmt.Sprintf("Class(%d)", int(c))
}

// The priority ranges (section 10.1.2).
const (
	// MinPriority and MaxPriority bound every priority.
	MinPriority = 0
	MaxPriority = 31

	// MaxNormalPriority is the highest normal priority; 16 and up are
	// real-time.
	MaxNormalPriority = 15

	// Priorities is how many priorities there are, so how many
	// computable queues.
	Priorities = MaxPriority + 1
)

// IsRealTime reports whether a base priority is in the real-time range.
// A real-time process's priority never changes by itself: no boosts, no
// decay, and no quantum end.
func IsRealTime(base int) bool {
	return base > MaxNormalPriority
}

// boosted is the current priority a process with base priority base and
// current priority current gets when an event with increment inc ends
// its wait, by the book's three steps (section 10.2.4):
//
//  1. the increment is added to the base priority;
//  2. if the current priority is already higher, it is kept instead;
//  3. if the result is above 15, the base priority is used.
//
// Step 3 does two jobs at once: a real-time process (base above 15)
// always comes out at its base priority, and a boost can't lift a normal
// process into the real-time range. (So a normal process at base 14 or
// 15 gets no boost at all.)
func boosted(base, current, inc int) int {
	p := base + inc
	if current > p {
		p = current
	}

	if p > MaxNormalPriority {
		p = base
	}

	return p
}
