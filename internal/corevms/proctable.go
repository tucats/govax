package corevms

import (
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The process table (docs/PHASE-43.md, subtask 3).
//
// VMS finds a process from its process ID (PID) through a table of
// pointers to process control blocks, the "PCB vector", indexed by a
// small number called the process index. The PID carries that index in
// its low bits and a sequence number above it. Each slot of the table has
// its own sequence number, incremented every time a new process takes
// the slot, so a PID of a process that has been deleted doesn't match
// the slot's next occupant: a stale PID is told from a live one by
// comparing the whole PID with the one the slot's process holds.
// Slot 0 belongs to the "null process" (the idle loop), so real processes
// start at index 1, and a new process takes the lowest free index.
// (VAX/VMS Internals and Data Structures, section 20.1.3, "The PCB
// Vector", and 20.1.4, "Fabrication of Process IDs", figure 20-4.)
//
// The book's VMS has a 16-bit index and a 16-bit sequence number. The
// PIDs later VMS versions show users (the "extended PID") pack both into
// 21 bits, with as few index bits as the system's maximum process count
// needs, which is why a VMS 7.3 system's first processes have PIDs such
// as 00000201 and 00000205. govax uses 8 index bits (255 processes, more
// than S0 can hold; see docs/PHASE-43.md, subtask 6) and the remaining
// 13 for the sequence number. Every slot's sequence number starts at 2,
// so a slot's first process has sequence 3, and process 1 keeps the PID
// govax has always given it, 00000301. The widths and the starting
// sequence are govax's choice, unconfirmed against a real system.
const (
	// pidIndexBits is how many low PID bits hold the process index.
	pidIndexBits = 8
	pidIndexMask = 1<<pidIndexBits - 1

	// MaxProcesses is how many processes the table holds: indexes 1
	// through 255 (0 is the null process's).
	MaxProcesses = pidIndexMask

	// pidSequenceMask keeps a sequence number in the 13 bits above the
	// index (21 bits in all, as an extended PID's process field).
	pidSequenceMask = 1<<(21-pidIndexBits) - 1

	// initialSequence is every slot's sequence number before its first
	// process: that process gets initialSequence+1.
	initialSequence = 2
)

// Status codes of the process table.
var (
	ssDuplNam = vmsdef.Symbols["SS$_DUPLNAM"]
	ssNoSlot  = vmsdef.Symbols["SS$_NOSLOT"]
)

// processTable is the System's table of processes: the slot each one
// occupies (by index), each slot's sequence number, and which process is
// current, the one whose context the CPU holds.
type processTable struct {
	// slots[i] is the process at index i, or nil if the slot is free.
	// slots[0] is always nil: index 0 is the null process's.
	slots [MaxProcesses + 1]*Environment

	// sequence[i] is the sequence number slot i's latest process got.
	sequence [MaxProcesses + 1]uint32

	// current is the process the CPU is running (or will run when it
	// next runs): the one whose services the engine's system-service and
	// AST hooks reach. nil when there is no process.
	current *Environment
}

// newProcessTable returns an empty table, every slot's sequence number
// at initialSequence.
func newProcessTable() *processTable {
	t := &processTable{}

	for i := range t.sequence {
		t.sequence[i] = initialSequence
	}

	return t
}

// pid composes a PID from a slot's index and sequence number.
func pid(index, sequence uint32) uint32 {
	return sequence<<pidIndexBits | index
}

// addProcess gives env a slot in the table: the lowest free index, with
// that slot's next sequence number, which together make its PID
// (env.Process.PID). The first process added becomes the current one.
// A process whose name is already another process's in its UIC group
// is left with no name, since names are unique within a group (System
// Services Reference, $SETPRN and $CREPRC: SS$_DUPLNAM). It fails with
// SS$_NOSLOT, as $CREPRC does, when every slot is taken.
func (sys *System) addProcess(env *Environment) error {
	t := sys.procs

	for index := uint32(1); index <= MaxProcesses; index++ {
		if t.slots[index] != nil {
			continue
		}

		// The sequence number wraps within its field, skipping 0, so a
		// PID is never just an index.
		seq := (t.sequence[index] + 1) & pidSequenceMask
		if seq == 0 {
			seq = 1
		}

		if _, found := sys.FindProcessName(env.Process.UICGroup(), env.Process.Name); found {
			env.Process.Name = ""
		}

		t.sequence[index] = seq
		t.slots[index] = env
		env.Process.PID = pid(index, seq)

		if t.current == nil {
			t.current = env
		}

		return nil
	}

	return vmserrors.New(ssNoSlot)
}

// RemoveProcess takes env's process out of the table, freeing its slot
// for a later process (which will get a different PID: the slot's
// sequence number moves on). If it was the current process, there is
// none until SetCurrent names one. Removing a process that isn't in the
// table does nothing. The physical pages its address space was given
// (TeardownAddressSpace), and any S0 pool pages still allocated to the
// process, are freed.
func (sys *System) RemoveProcess(env *Environment) {
	t := sys.procs
	index := env.Process.PID & pidIndexMask

	if t.slots[index] != env {
		return
	}

	t.slots[index] = nil

	// A failure here (the space is the CPU's current one) leaves the
	// frames allocated, a leak rather than a corruption; FreeProcess below
	// still gives back the tables' pages.
	_ = sys.TeardownAddressSpace(env.Space)

	if sys.s0 != nil {
		sys.s0.FreeProcess(env.Process.PID)
	}

	if t.current == env {
		t.current = nil
	}
}

// Processes returns every process in the table, by index.
func (sys *System) Processes() []*Environment {
	var list []*Environment

	for _, env := range sys.procs.slots {
		if env != nil {
			list = append(list, env)
		}
	}

	return list
}

// FindProcess returns the process whose PID is pid. As VMS does, it uses
// the PID's low bits as an index into the table and then compares the
// whole PID with that slot's process's, so the PID of a deleted process
// finds nothing even after its slot is reused.
func (sys *System) FindProcess(pid uint32) (*Environment, bool) {
	index := pid & pidIndexMask

	env := sys.procs.slots[index]
	if env == nil || env.Process.PID != pid {
		return nil, false
	}

	return env, true
}

// FindProcessName returns the process named name in UIC group group.
// Process names are unique only within a group, and a service that names
// a process (a prcnam argument) only finds one in its caller's group.
// The match is exact: no abbreviation, case folding, or trailing blanks.
// An empty name names no process.
func (sys *System) FindProcessName(group uint32, name string) (*Environment, bool) {
	if name == "" {
		return nil, false
	}

	for _, env := range sys.procs.slots {
		if env != nil && env.Process.Name == name && env.Process.UICGroup() == group {
			return env, true
		}
	}

	return nil, false
}

// Current returns the current process: the one the CPU is running. nil
// before the first process is added.
func (sys *System) Current() *Environment { return sys.procs.current }

// SetCurrent makes env the current process. It only records which
// process that is; switching the CPU's context to it is the scheduler's
// work (docs/PHASE-44.md).
func (sys *System) SetCurrent(env *Environment) { sys.procs.current = env }
