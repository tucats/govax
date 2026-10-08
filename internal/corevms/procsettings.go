package corevms

import (
	"fmt"
	"strings"
)

// The multiprocessing settings (Phase 43; docs/PHASE-43.md, Part A's
// "Rules for every commit" and Decisions 2, 3, and 5). Phase 43 reads
// them but nothing acts on them yet: there is no scheduler until Phase 44.

// Default values for ProcessSettings.
const (
	// DefaultProcessQuantum is how many instructions a process runs
	// before the scheduler may give the CPU to another of the same
	// priority (Decision 2): a starting value, to be tuned in Phase 44.
	DefaultProcessQuantum = 20000
)

// PreemptMode says which access modes the scheduler may take the CPU
// away from involuntarily, at an instruction boundary (Decision 3). A
// process that waits gives the CPU up whatever the mode.
type PreemptMode int

const (
	// PreemptAll is VMS's rule: a process can be preempted in any access
	// mode, when the CPU's IPL is below 3 (the IPL VMS reschedules at)
	// and it isn't on the interrupt stack.
	PreemptAll PreemptMode = iota

	// PreemptUser preempts only user-mode code, so a system service or
	// a supervisor-mode routine always finishes first: for debugging the
	// scheduler.
	PreemptUser

	// PreemptNone never preempts: processes switch only when one waits
	// (cooperative scheduling).
	PreemptNone
)

// preemptNames are the setting's values, by PreemptMode.
var preemptNames = [...]string{PreemptAll: "all", PreemptUser: "user", PreemptNone: "none"}

// String returns the setting's value for m.
func (m PreemptMode) String() string {
	if int(m) < len(preemptNames) {
		return preemptNames[m]
	}

	return fmt.Sprintf("PreemptMode(%d)", int(m))
}

// ParsePreemptMode turns the vax.process.preempt setting's text into a
// PreemptMode, ignoring case and surrounding blanks. Empty text is the
// default, PreemptAll.
func ParsePreemptMode(text string) (PreemptMode, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return PreemptAll, nil
	}

	for m, name := range preemptNames {
		if name == text {
			return PreemptMode(m), nil
		}
	}

	return PreemptAll, fmt.Errorf("vax.process.preempt is %q; it must be all, user, or none", text)
}

// ProcessSettings are the multiprocessing settings a System runs with.
type ProcessSettings struct {
	// Scheduler is vax.process.scheduler: whether the engine may run
	// processes other than process 1, and $CREPRC and LIB$SPAWN create
	// them. On by default since Phase 48's milestone passed (Decision 5
	// in docs/PHASE-43.md); the key can still turn it off.
	Scheduler bool

	// Quantum is vax.process.quantum: instructions per quantum.
	Quantum int

	// Preempt is vax.process.preempt: which modes may be preempted.
	Preempt PreemptMode
}

// DefaultProcessSettings are the settings with none of the keys set.
func DefaultProcessSettings() ProcessSettings {
	return ProcessSettings{Scheduler: true, Quantum: DefaultProcessQuantum, Preempt: PreemptAll}
}
