package rtl

// Process is the emulated VMS process an Environment runs images in: the
// identity and quota state system services read and update on behalf of
// "the calling process" (docs/PHASE-26.md). govax has exactly one process
// per Environment, so there is no process table, no PCB/JIB split, and no
// scheduling state — just the fields some implemented service needs, with
// a comment naming the VMS field each one stands in for. Later services
// that need more process state (quotas, privileges, a process name, ...)
// add it here rather than as loose Environment fields.
//
// A Process is rebuilt along with its Environment on INIT/VMINIT/ZERO,
// the emulated equivalent of logging in again.
type Process struct {
	// PID is the process ID (PCB$L_EPID). Stamped into a device's owner
	// by $ASSIGN and $ALLOC.
	PID uint32

	// Username is the account name (JIB$T_USERNAME), SYSTEM by default.
	Username string

	// UIC is the process's user identification code (PCB$L_UIC), group
	// in the high word and member in the low word — [1,4] (SYSTEM) by
	// default. Its group names the logical-name group table and scopes
	// common event flag cluster names.
	UIC uint32

	// Working-set state (kept in the process header on VMS). WSLimit is the
	// current limit $ADJWSL adjusts, starting at WSDefault; $ADJWSL keeps
	// it within [MinWSCount, WSExtent]. None of these are enforced by
	// govax's memory model — they're kept so the services that report or
	// adjust them behave as documented. All counts are in pages.
	WSLimit    uint32
	WSDefault  uint32 // UAF WSDEFAULT: the initial WSLimit
	WSQuota    uint32 // UAF WSQUOTA
	WSExtent   uint32 // UAF WSEXTENT: the most $ADJWSL can grow WSLimit to
	MinWSCount uint32 // SYSGEN MINWSCNT: the least $ADJWSL can shrink it to
}

// Default identity and quotas for the emulated process. The PID is
// arbitrary (nonzero, so "a process exists" can be told from a zero
// field). The username and UIC are the SYSTEM account's. The working-set
// numbers are nominal values in the range VMS 5 used for SYSTEM, not
// taken from any particular system's UAF or SYSGEN parameters.
const (
	nominalPID      = 0x00000301
	NominalUIC      = 0x00010004 // [1,4]
	nominalUsername = "SYSTEM"

	nominalWSDefault  = 150
	nominalWSQuota    = 256
	nominalWSExtent   = 1024
	nominalMinWSCount = 20
)

// NewProcess returns the default emulated process: PID nominalPID, user
// SYSTEM, UIC [1,4], with its working-set limit at its default.
func NewProcess() *Process {
	return &Process{
		PID:        nominalPID,
		Username:   nominalUsername,
		UIC:        NominalUIC,
		WSLimit:    nominalWSDefault,
		WSDefault:  nominalWSDefault,
		WSQuota:    nominalWSQuota,
		WSExtent:   nominalWSExtent,
		MinWSCount: nominalMinWSCount,
	}
}

// UICGroup returns the group half of the process's UIC.
func (p *Process) UICGroup() uint32 { return p.UIC >> 16 }

// UICMember returns the member half of the process's UIC.
func (p *Process) UICMember() uint32 { return p.UIC & 0xFFFF }

// optArg returns argv[i], or 0 when the argument list is too short to
// hold it — the VMS convention for an omitted trailing optional argument,
// which reads the same as one passed as 0.
func optArg(argv []uint32, i int) uint32 {
	if i < len(argv) {
		return argv[i]
	}

	return 0
}
