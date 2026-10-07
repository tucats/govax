package corevms

import (
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// System is the state one emulated VAX/VMS system has once, however many
// processes run on it (docs/PHASE-43.md): the machine (one CPU, one
// physical memory), the tables of system services and RTL shims, the
// devices and mounted volumes, the mailboxes, common event flag clusters,
// and OPCOM's state, and the system's clock, boot time, and node name.
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
	}

	sys.BootTime = sys.Clock()
	sys.removeStaleMailboxes()

	registerShims(sys.shims)
	registerServices(sys.services)

	return sys
}
