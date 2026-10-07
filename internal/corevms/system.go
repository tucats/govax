package corevms

import (
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// System is the state one emulated VAX/VMS system has once, however many
// processes run on it (docs/PHASE-43.md): the machine (one CPU, one
// physical memory), the tables of system services and RTL shims, the
// devices and mounted volumes, the mailboxes, common event flag clusters,
// and OPCOM's state, the system's clock, boot time, and node name, and
// the process table.
//
// On VMS this is what lives in system space (S0) and is the same in every
// process: the I/O database, the mailbox and event-flag-cluster lists,
// the system's time. A process's own state (its channels, open files,
// event flags, ASTs, heap, ...) is in its Environment, and every
// Environment points to the one System it runs on.
//
// The console builds a System on each INIT, VMINIT, and ZERO, since they
// make a new machine; the device and mount tables it's given are the
// console's own, which outlive the machine.
type System struct {
	mem *vm.Memory
	cpu *vax.CPU

	// shims and services are the LIB$/CRTL shim and SYS$ service
	// registries the XFC$SHIM and XFC$P1VECTOR selectors dispatch
	// through (shim.go, service.go). They're the same for every process,
	// so they're built once per System.
	shims    *ShimTable
	services *ServiceTable

	// Devices is Phase 09's device table (internal/io), injected rather
	// than owned here: the console and every process see the same table,
	// and it outlives an INIT/VMINIT/ZERO.
	Devices *iodev.DeviceTable

	// Mounts is docs/PHASE-22.md's device-name -> mounted-ODS-2-volume
	// table (internal/rms.MountTable), injected the same way Devices is:
	// it's owned by internal/console's Console (constructed once,
	// alongside Devices — a MOUNT command's effect must still be visible
	// after a later VMInit/Zero rebuilds this System from scratch).
	Mounts *rms.MountTable

	// EventFlagClusters is the system-wide table of common event flag
	// clusters $ASCEFC creates and associates (eventflags.go). A process's
	// own event flags and associations are in its Process.
	EventFlagClusters *CommonEventFlags

	// Mailboxes are the mailboxes $CREMBX has created (mailbox.go). Like
	// common event flag clusters they're in system memory on VMS, so
	// INIT/VMINIT/ZERO start with none.
	Mailboxes *MailboxTable

	// Operator is OPCOM's state: the console's operator classes and the
	// outstanding operator requests (operator.go).
	Operator *operatorState

	// Clock returns the current system time in VMS format (100ns units
	// since 17-Nov-1858), what $SETIMR's timers run on. NewSystem sets it
	// to the host clock; the console replaces it with its Engine's
	// SystemTime, the time base the interval clock also uses (timers.go).
	Clock func() uint64

	// BootTime is when the system "booted", in VMS time: $GETSYI's
	// SYI$_BOOTTIME. NewSystem sets it to Clock's time; the console
	// resets it after rebinding Clock, so it's the time of the INIT,
	// VMINIT, or ZERO that built this System.
	BootTime uint64

	// NodeName is the system's node name ($GETSYI's SYI$_NODENAME).
	NodeName string

	// ProcessSettings are the multiprocessing settings
	// (procsettings.go). NewSystem sets the defaults; the console
	// replaces them with the vax.process.* settings
	// (SetProcessSettings, which also gives the scheduler its quantum).
	ProcessSettings ProcessSettings

	// sched is the scheduler (schedule.go): every process in procs is
	// in it, by PID. It only decides anything while the engine has the
	// System installed as its scheduling hook.
	sched *sched.Scheduler

	// engine is the engine the System is the scheduling hook of
	// (InstallScheduler), or nil when the scheduler isn't in use; then
	// waiting services spin as they always have (waits.go).
	engine *cpu.Engine

	// waiters counts the processes waiting in the scheduler (those with
	// Environment.waiting set), so the scheduler skips testing them
	// when there are none.
	waiters int

	// procs is the process table: every process's Environment by its
	// PID's index, and which one is current (proctable.go).
	procs *processTable

	// s0 is the pool of S0 pages new processes' page tables, stacks, and
	// PCBs come from (s0pool.go); nil until the console's VMINIT.
	s0 *S0Pool

	// sharedP1 is the P1 pages every process maps onto the same physical
	// pages: the P1 vector's (ShareP1, addrspace.go).
	sharedP1 []sharedPage
}

// NewSystem returns a new System driving cpu and mem, sharing devices and
// mounts with whatever else (the console) also uses them. Mailbox devices
// left in the device table by a previous System are removed: their
// messages were in that machine's memory.
func NewSystem(cpu *vax.CPU, mem *vm.Memory, devices *iodev.DeviceTable, mounts *rms.MountTable) *System {
	sys := &System{
		mem:      mem,
		cpu:      cpu,
		shims:    NewShimTable(),
		services: NewServiceTable(),
		Devices:  devices,
		Mounts:   mounts,

		EventFlagClusters: NewCommonEventFlags(),
		Mailboxes:         NewMailboxTable(),
		Operator:          newOperatorState(),
		Clock:             wallClock,
		NodeName:          nominalNodeName,
		ProcessSettings:   DefaultProcessSettings(),
		procs:             newProcessTable(),
		sched:             sched.New(DefaultProcessQuantum),
	}

	sys.BootTime = sys.Clock()
	sys.removeStaleMailboxes()

	registerShims(sys.shims)
	registerServices(sys.services)

	return sys
}
