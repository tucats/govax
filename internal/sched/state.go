package sched

import "fmt"

// State is a process's scheduling state, by VMS's own numbering: the
// values of the SCH$C_* symbols, which VMS keeps in a process's
// PCB$W_STATE field and $GETJPI returns for JPI$_STATE (*VAX/VMS
// Internals and Data Structures*, Table 10-1).
type State uint16

const (
	// StateCOLPG is "collided page wait": a page fault on a page another
	// process is already reading in. govax has no paging, so it's unused.
	StateCOLPG State = 1

	// StateMWAIT is "miscellaneous wait": waiting for a system resource,
	// such as room in a full mailbox. Which resource is a Resource.
	StateMWAIT State = 2

	// StateCEF is "common event flag wait": waiting for a flag in a
	// common event flag cluster, a set of flags shared by processes.
	StateCEF State = 3

	// StatePFW is "page fault wait" (unused: no paging).
	StatePFW State = 4

	// StateLEF is "local event flag wait": waiting for one of the
	// process's own event flags ($WAITFR, $WFLOR, $WFLAND, $QIOW, ...).
	StateLEF State = 5

	// StateLEFO is StateLEF for a process swapped out of memory (unused).
	StateLEFO State = 6

	// StateHIB is "hibernate": waiting for a $WAKE ($HIBER).
	StateHIB State = 7

	// StateHIBO is StateHIB swapped out (unused).
	StateHIBO State = 8

	// StateSUSP is "suspended": stopped by $SUSPND until a $RESUME.
	StateSUSP State = 9

	// StateSUSPO is StateSUSP swapped out (unused).
	StateSUSPO State = 10

	// StateFPG is "free page wait" (unused: no paging).
	StateFPG State = 11

	// StateCOM is "computable": ready to run, waiting only for the CPU.
	StateCOM State = 12

	// StateCOMO is StateCOM swapped out (unused).
	StateCOMO State = 13

	// StateCUR is the current process: the one the CPU is running.
	StateCUR State = 14
)

// stateNames are the states' names as VMS's SHOW SYSTEM shows them.
var stateNames = [...]string{
	StateCOLPG: "COLPG",
	StateMWAIT: "MWAIT",
	StateCEF:   "CEF",
	StatePFW:   "PFW",
	StateLEF:   "LEF",
	StateLEFO:  "LEFO",
	StateHIB:   "HIB",
	StateHIBO:  "HIBO",
	StateSUSP:  "SUSP",
	StateSUSPO: "SUSPO",
	StateFPG:   "FPG",
	StateCOM:   "COM",
	StateCOMO:  "COMO",
	StateCUR:   "CUR",
}

// String returns the state's short name ("LEF", "HIB", "CUR", ...).
func (s State) String() string {
	if int(s) < len(stateNames) && stateNames[s] != "" {
		return stateNames[s]
	}

	return fmt.Sprintf("State(%d)", uint16(s))
}

// IsWait reports whether s is one of the wait states: a process in it
// can't run until some event makes it computable again.
func (s State) IsWait() bool {
	return s >= StateCOLPG && s <= StateFPG
}

// Resource names what a process in StateMWAIT is waiting for. Its values
// are the RSN$_* numbers the book lists (Table 10-2), which VMS keeps in
// the PCB$L_EFWM field during a resource wait. The book describes VMS
// 3.3; that VMS 7.3 numbers them the same is unconfirmed.
type Resource uint32

const (
	// ResourceNone is no resource: the process isn't in a resource wait.
	ResourceNone Resource = 0

	// ResourceAST (RSN$_ASTWAIT) waits for an AST to be delivered.
	ResourceAST Resource = 1

	// ResourceMailbox (RSN$_MAILBOX) waits for room in a full mailbox.
	ResourceMailbox Resource = 2

	// ResourceNonpagedPool (RSN$_NPDYNMEM) waits for nonpaged pool.
	ResourceNonpagedPool Resource = 3

	// ResourcePageFile (RSN$_PGFILE) waits for page file space.
	ResourcePageFile Resource = 4

	// ResourcePagedPool (RSN$_PGDYNMEM) waits for paged pool.
	ResourcePagedPool Resource = 5

	// ResourceBreakthrough (RSN$_BRKTHRU) waits for a broadcast message.
	ResourceBreakthrough Resource = 6

	// ResourceImageActivation (RSN$_IACLOCK) waits for the image
	// activation lock.
	ResourceImageActivation Resource = 7

	// ResourceJobQuota (RSN$_JQUOTA) waits for a pooled job quota.
	ResourceJobQuota Resource = 8

	// ResourceLockID (RSN$_LOCKID) waits for the lock ID database.
	ResourceLockID Resource = 9

	// ResourceSwapFile (RSN$_SWPFILE) waits for swap file space.
	ResourceSwapFile Resource = 10

	// ResourceModifiedPageList (RSN$_MPLEMPTY) waits for the modified
	// page list to empty.
	ResourceModifiedPageList Resource = 11

	// ResourceModifiedPageWriter (RSN$_MPWBUSY) waits for the modified
	// page writer.
	ResourceModifiedPageWriter Resource = 12
)

// resourceNames are the names SHOW SYSTEM shows in the state column for
// a process waiting on each resource, in place of "MWAIT" (the "RW"
// stands for "resource wait"). Unconfirmed against VMS 7.3's output.
var resourceNames = [...]string{
	ResourceAST:                "RWAST",
	ResourceMailbox:            "RWMBX",
	ResourceNonpagedPool:       "RWNPG",
	ResourcePageFile:           "RWPFF",
	ResourcePagedPool:          "RWPAG",
	ResourceBreakthrough:       "RWBRK",
	ResourceImageActivation:    "RWIMG",
	ResourceJobQuota:           "RWQUO",
	ResourceLockID:             "RWLCK",
	ResourceSwapFile:           "RWSWP",
	ResourceModifiedPageList:   "RWMPE",
	ResourceModifiedPageWriter: "RWMPB",
}

// String returns the resource wait's name ("RWMBX", ...), or "MWAIT" for
// a resource without one.
func (r Resource) String() string {
	if int(r) < len(resourceNames) && resourceNames[r] != "" {
		return resourceNames[r]
	}

	return "MWAIT"
}

// StateName is the state as SHOW SYSTEM names it: the resource's name for
// a resource wait, otherwise the state's.
func StateName(s State, r Resource) string {
	if s == StateMWAIT && r != ResourceNone {
		return r.String()
	}

	return s.String()
}
