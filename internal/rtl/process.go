package rtl

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

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

	// Name is the process name (PCB$T_LNAME), what $GETJPI's prcnam
	// matches. Account is the UAF account field (JIB$T_ACCOUNT), and
	// Terminal the login terminal (PCB$T_TERMINAL). CLIName is the
	// command language interpreter's name. All describe a SYSTEM login on
	// the console terminal.
	Name     string
	Account  string
	Terminal string
	CLIName  string

	// UIC is the process's user identification code (PCB$L_UIC), group
	// in the high word and member in the low word — [1,4] (SYSTEM) by
	// default. Its group names the logical-name group table and scopes
	// common event flag cluster names.
	UIC uint32

	// Working-set state (kept in the process header on VMS). WSLimit is
	// the current limit $ADJWSL adjusts, starting at WSDefault; $ADJWSL keeps
	// it within [MinWSCount, WSExtent]. None of these are enforced by
	// govax's memory model — they're kept so the services that report or
	// adjust them behave as documented. All counts are in pages.
	WSLimit    uint32
	WSDefault  uint32 // UAF WSDEFAULT: the initial WSLimit
	WSQuota    uint32 // UAF WSQUOTA
	WSExtent   uint32 // UAF WSEXTENT: the most $ADJWSL can grow WSLimit to
	MinWSCount uint32 // SYSGEN MINWSCNT: the least $ADJWSL can shrink it to

	// LocalEventFlags are event flag clusters 0 and 1 (flags 0-63), local
	// to the process. CommonClusters are the common event flag clusters
	// $ASCEFC associated with cluster numbers 2 and 3 (flags 64-127), nil
	// when not associated (eventflags.go).
	LocalEventFlags [2]uint32
	CommonClusters  [2]*EventFlagCluster
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
	nominalName     = "SYSTEM"
	nominalAccount  = "SYSTEM"
	nominalTerminal = "TTA0:"
	nominalCLIName  = "DCL"

	nominalWSDefault  = 150
	nominalWSQuota    = 256
	nominalWSExtent   = 1024
	nominalMinWSCount = 20
)

// NewProcess returns the default emulated process: PID nominalPID, user
// SYSTEM (process name SYSTEM, account SYSTEM, terminal TTA0:, CLI DCL),
// UIC [1,4], with its working-set limit at its default.
func NewProcess() *Process {
	return &Process{
		PID:        nominalPID,
		Username:   nominalUsername,
		Name:       nominalName,
		Account:    nominalAccount,
		Terminal:   nominalTerminal,
		CLIName:    nominalCLIName,
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

// Status codes the process-control services return.
var ssNoPriv = vmsdef.SSConstants["SS$_NOPRIV"]

// serviceSysAdjstk is SYS$ADJSTK: sets the saved stack pointer of an
// access mode less privileged than the caller's. The longword at newadr
// supplies the new value (or, when it is 0, the mode's current stack
// pointer does), the signed low word of adjust is added to it, and the
// result is both written back to newadr and loaded as that mode's stack
// pointer. acmode is maximized with the caller's mode, so asking for the
// caller's own mode or a more privileged one — including the default,
// kernel — is SS$_NOPRIV.
//
// The target mode is never the one executing, so its stack pointer is
// the saved copy in the KSP/ESP/SSP/USP privileged register, which is
// what gets loaded when the CPU next changes into that mode. The manual's
// SS$_ACCVIO for "a portion of the new stack segment cannot be written"
// isn't checked: govax doesn't probe the new stack, only newadr itself.
func serviceSysAdjstk(env *Environment, argv []uint32) (uint32, error) {
	acmode, adjust, newadr := optArg(argv, 0), int16(optArg(argv, 1)), optArg(argv, 2)

	curMod := uint32(env.cpu.PSL().CurMod())
	mode := max(acmode&3, curMod)

	if mode == curMod {
		return ssNoPriv, nil
	}

	if newadr == 0 { // page 0 is never accessible on VMS
		return ssAccVio, nil
	}

	value, err := env.mem.LoadLongword(env.cpu, newadr)
	if err != nil {
		return ssAccVio, nil
	}

	if value == 0 {
		value = env.cpu.PR(vax.PrivReg(mode)) // KSP/ESP/SSP/USP == mode 0/1/2/3
	}

	value += uint32(int32(adjust))

	if err := env.mem.StoreLongword(env.cpu, newadr, value); err != nil {
		return ssAccVio, nil
	}

	env.cpu.SetPR(vax.PrivReg(mode), value)

	return ssNormal, nil
}

// serviceSysAdjwsl is SYS$ADJWSL: adds the signed pagcnt to the process's
// working-set limit and returns the result through wsetlm. A limit pushed
// past WSEXTENT or below MINWSCNT is quietly clamped there, as the manual
// says ("no error condition is returned"). With pagcnt 0 (or omitted)
// nothing changes and the current limit is returned. wsetlm is optional.
//
// The limit is recorded in env.Process but not enforced: govax's memory
// model has no working set.
func serviceSysAdjwsl(env *Environment, argv []uint32) (uint32, error) {
	pagcnt, wsetlm := int32(optArg(argv, 0)), optArg(argv, 1)
	p := env.Process

	limit := min(max(int64(p.WSLimit)+int64(pagcnt), int64(p.MinWSCount)), int64(p.WSExtent))
	if pagcnt == 0 {
		limit = int64(p.WSLimit)
	}

	if wsetlm != 0 {
		if err := env.mem.StoreLongword(env.cpu, wsetlm, uint32(limit)); err != nil {
			return ssAccVio, nil
		}
	}

	p.WSLimit = uint32(limit)

	return ssNormal, nil
}

// ImageRundown does the per-image cleanup VMS does when an image exits,
// for the state this package owns: it deassigns the channels the image
// assigned from user mode, then deallocates the devices it allocated in
// user mode, and disassociates its common event flag clusters (deleting
// temporary ones nobody else uses). The console calls it when an image
// started by RUN returns (and does its own logical-name rundown
// alongside).
func (env *Environment) ImageRundown() {
	env.deassignUserChannels()
	env.deallocateUserDevices()
	env.disassociateClusters()
}

func registerProcessServices(t *ServiceTable) {
	t.Register("SYS$ADJSTK", serviceSysAdjstk)
	t.Register("SYS$ADJWSL", serviceSysAdjwsl)
}
