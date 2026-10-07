package corevms

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Process is the emulated VMS process an Environment runs images in: the
// identity and quota state system services read and update on behalf of
// "the calling process" (docs/PHASE-26.md). There is one Process per
// Environment; what every process shares is in the System (system.go).
// It holds what VMS keeps in a process's PCB and process header; what a
// job's processes share (VMS's JIB) is the Job it points to (job.go), and
// its scheduling state is the scheduler's (schedule.go). It has just the
// fields some implemented service needs, with a comment naming the VMS
// field each one stands in for. Later services that need more process state (quotas,
// privileges, a process name, ...) add it here rather than as loose
// Environment fields.
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

	// Owner is the PID of the process that created this one, if it's a
	// subprocess, or 0 for a detached process (PCB$L_OWNER).
	// SubprocessCount is how many subprocesses this process has created
	// that still exist (PCB$W_PRCCNT). Job is the job the process belongs
	// to (PCB$L_JIB): its own, for a detached process, or its owner's
	// (job.go).
	Owner           uint32
	SubprocessCount uint32
	Job             *Job

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

	// ASTLimit is the AST quota (UAF ASTLM, PCB$W_ASTCNT's starting
	// value): how many ASTs may be outstanding at once. It's reported by
	// $GETJPI (JPI$_ASTLM, and JPI$_ASTCNT as what's left of it) but not
	// enforced (docs/DEVIATIONS.md).
	ASTLimit uint32

	// BufferedIOLimit and DirectIOLimit are the BIOLM and DIOLM quotas:
	// how many buffered and direct I/O requests may be outstanding at
	// once. CPULimit is the CPULM quota, the CPU time the process may
	// use, in 10-millisecond units, 0 meaning no limit. They're recorded
	// ($CREPRC sets a new process's from its creator's; creprc.go) and
	// reported by $GETJPI, but not enforced.
	BufferedIOLimit, DirectIOLimit, CPULimit uint32

	// cpuDeducted is how much of its creator's CPU time limit the
	// process took when it was created (quotas.go's cpuLimit), to be
	// given back when it's deleted; 0 when the creator had no limit.
	cpuDeducted uint32

	// CreateFlags are the status flags ($CREPRC's stsflg, the PRC$M_
	// bits) the process was created with: $GETJPI's JPI$_CREPRC_FLAGS.
	// TerminationMailbox is the unit number of the mailbox that gets
	// the accounting message when the process is deleted (PCB$W_TMBU,
	// $CREPRC's mbxunt), 0 for none. Both are 0 for process 1.
	CreateFlags        uint32
	TerminationMailbox uint32

	// LoginTime is when the process was created, in VMS system time
	// (CTL$GQ_LOGIN): $GETJPI's JPI$_LOGINTIM, and the termination
	// message's ACC$Q_LOGIN. The process table sets it (addProcess).
	LoginTime uint64

	// Priority and BasePriority are the process's current and base
	// scheduling priorities (PCB$B_PRI, PCB$B_PRIB, as the user sees
	// them: 0-31, higher runs first). govax has one process and no
	// scheduler, so they're only reported ($GETJPI's JPI$_PRI, PRIB).
	Priority     uint32
	BasePriority uint32

	// AuthorizedPriority is the highest base priority the process may
	// set without the ALTPRI privilege (UAF PRIORITY, PCB$B_AUTHPRI):
	// $SETPRI lowers a higher request to it, and $GETJPI reports it
	// (JPI$_AUTHPRI).
	AuthorizedPriority uint32

	// The process's privilege masks (privilege.go), one bit per $PRVDEF
	// privilege: AuthorizedPrivileges, what it may enable (the UAF's,
	// PCB's AUTHPRIV); ProcessPrivileges, its permanent ones (PROCPRIV,
	// in the process header); CurrentPrivileges, the ones enabled now,
	// which services check (PCB$Q_PRIV); ImagePrivileges, the running
	// image's installed ones (always empty: govax installs nothing).
	AuthorizedPrivileges, ProcessPrivileges, CurrentPrivileges, ImagePrivileges uint64

	// LocalEventFlags are event flag clusters 0 and 1 (flags 0-63), local
	// to the process. CommonClusters are the common event flag clusters
	// $ASCEFC associated with cluster numbers 2 and 3 (flags 64-127), nil
	// when not associated (eventflags.go).
	LocalEventFlags [2]uint32
	CommonClusters  [2]*EventFlagCluster

	// ResourceWaitDisabled is set when the process has turned resource
	// wait mode off with $SETRWM (PCB$V_SSRWAIT): a service that runs out
	// of a resource, such as a write to a full mailbox, then fails at
	// once instead of waiting for it. VMS starts every process with
	// resource wait mode enabled.
	ResourceWaitDisabled bool

	// WakePending is the process's wakeup request flag (PCB$V_WAKEPEN):
	// set by $WAKE or an expiring $SCHDWK, consumed by the next $HIBER
	// (hibernate.go). It's a flag, not a count: several wakeups before a
	// $HIBER end just that one.
	WakePending bool

	// ast is the process's AST queue and per-mode AST state (the PCB's
	// AST fields; ast.go).
	ast astState

	// exitHandlers holds each access mode's declared exit control blocks
	// ($DCLEXH), by address, oldest first — the lists VMS heads at
	// CTL$GL_THEXIT and friends (exit.go). Kernel mode's is always empty.
	exitHandlers [4][]uint32

	// ExitStatus is the completion status of the last $EXIT, which VMS
	// saves in the process header.
	ExitStatus uint32

	// putmsg holds the $PUTMSG calls whose action routine is running,
	// innermost last (an action routine may call $PUTMSG itself;
	// message.go).
	putmsg []*putmsgCall

	// cmode holds the $CMKRNL/$CMEXEC calls whose routine is running,
	// innermost last (cmode.go).
	cmode []*cmodeCall

	// conditions holds the conditions being dispatched to handlers,
	// innermost last: a handler may cause a condition of its own
	// (condition.go).
	conditions []*conditionDispatch

	// exceptionVectors are each access mode's primary, secondary, and
	// last-chance exception vectors (indexed by vectorPrimary, ...): the
	// condition handlers searched before and after the call frames, 0
	// when not set (CTL$AQ_EXCVEC on VMS; condition.go).
	exceptionVectors [4][3]uint32

	// memoryLocks and workingSetLocks are the pages locked in memory
	// ($LCKPAG) and in the working set ($LKWSET), by address. Nothing
	// pages in govax, so they only decide what the locking services
	// report (pageprot.go).
	memoryLocks, workingSetLocks pageLocks
}

// The exception vectors of each access mode, as $SETEXV numbers them.
const (
	vectorPrimary    = 0
	vectorSecondary  = 1
	vectorLastChance = 2
)

// Default identity and quotas for the emulated process. The PID is the
// one the process table gives process 1 (proctable.go), which replaces it
// when the process is added to a System; it's nonzero, so "a process
// exists" can be told from a zero field. The username and UIC are the SYSTEM account's. The working-set
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

	// The AST quota and priority are VMS's defaults for an interactive
	// user: ASTLM 24, base priority 4 (SYSGEN DEFPRI).
	nominalASTLimit = 24
	nominalPriority = 4

	// The buffered and direct I/O quotas are nominal values in the range
	// VMS's SYSTEM account has. There is no CPU time limit.
	nominalBIOLM = 40
	nominalDIOLM = 40
)

// NewProcess returns the default emulated process: PID nominalPID, user
// SYSTEM (process name SYSTEM, account SYSTEM, terminal TTA0:, CLI DCL),
// UIC [1,4], with its working-set limit at its default, authorized for
// and holding every privilege, as the SYSTEM account is.
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

		ASTLimit:        nominalASTLimit,
		BufferedIOLimit: nominalBIOLM,
		DirectIOLimit:   nominalDIOLM,
		Priority:        nominalPriority,
		BasePriority:    nominalPriority,

		AuthorizedPriority:   nominalPriority,
		AuthorizedPrivileges: allPrivileges,
		ProcessPrivileges:    allPrivileges,
		CurrentPrivileges:    allPrivileges,

		ast: newASTState(),
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
var ssNoPriv = vmsdef.Symbols["SS$_NOPRIV"]

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
// for the state this package owns: it closes the files the image left
// open (CloseFiles), deassigns the channels the image
// assigned from user mode, then deallocates the devices it allocated in
// user mode, and disassociates its common event flag clusters (deleting
// temporary ones nobody else uses), cancels its outstanding $SETIMR
// timers and $SCHDWK wakeups, discards its queued user-mode ASTs, and
// forgets its user-mode exit handlers and any $PUTMSG or $CMKRNL left
// waiting for its routine. The console calls it when an
// image started by RUN returns or exits (and does its own logical-name
// rundown alongside).
func (env *Environment) ImageRundown() {
	env.CloseFiles()
	env.deassignUserChannels()
	env.deallocateUserDevices()
	env.disassociateClusters()
	env.cancelTimers()
	env.flushUserASTs()
	env.cancelUserExitHandlers()
	env.cancelPutmsgCalls()
	env.cancelChangeModeCalls()
	env.cancelConditions()
	env.cancelPageLocks()
	env.resetImagePrivileges()
	env.qiowWaits = nil
}

// The small process-control services (docs/PHASE-26.md subtask 30):
// $SETPRN, $SETPRI, $FORCEX, and $DELPRC. Each names its target process
// the usual way ([pidadr] ,[prcnam], processTarget), and govax's only
// process is the caller, so "another process" is always SS$_NONEXPR.

// maxPriority is the highest scheduling priority: 0-15 are ordinary
// ("normal") priorities, 16-31 real-time ones.
const maxPriority = 31

// serviceSysSetprn is SYS$SETPRN:
//
//	SYS$SETPRN [prcnam]
//
// It gives the calling process the name prcnam (1-15 characters), which
// $GETJPI reports (JPI$_PRCNAM) and the services that take a prcnam
// argument match. With prcnam omitted the process has no name. SS$_IVLOGNAM
// for an empty or too-long name, SS$_ACCVIO if it can't be read, and
// SS$_DUPLNAM if another process in the caller's UIC group already has
// the name (names are unique within a group). Renaming a process to the
// name it has is no error.
func serviceSysSetprn(env *Environment, argv []uint32) (uint32, error) {
	prcnam := optArg(argv, 0)
	if prcnam == 0 {
		env.Process.Name = ""

		return ssNormal, nil
	}

	name, ok, err := strGet(env, prcnam, maxProcessNameLength)
	if err != nil {
		return ssAccVio, nil
	}

	if !ok || name == "" {
		return ssIvLogNam, nil
	}

	if other, found := env.FindProcessName(env.Process.UICGroup(), name); found && other != env {
		return ssDuplNam, nil
	}

	env.Process.Name = name

	return ssNormal, nil
}

// serviceSysSetpri is SYS$SETPRI:
//
//	SYS$SETPRI [pidadr] ,[prcnam] ,pri [,prvpri]
//
// It sets the target process's base priority to pri (its low five bits,
// 0-31), storing the previous base priority at prvpri if that's given.
// The process holds ALTPRI, so it may raise its priority. Its current
// priority becomes the new base, in the scheduler too
// (sched.SetBasePriority), which then reschedules if that means another
// process should run: a process that lowers itself below a computable
// one gives it the CPU at the next instruction (docs/PHASE-44.md,
// subtask 6). The book's Table 10-3 lists a boost of 2 for "Set
// Priority"; govax doesn't apply it (unconfirmed). The target is picked
// by processTarget (SS$_NONEXPR, SS$_IVLOGNAM, SS$_ACCVIO); prvpri that
// can't be written is SS$_ACCVIO, with the priority unchanged.
func serviceSysSetpri(env *Environment, argv []uint32) (uint32, error) {
	if st := env.callerTarget(optArg(argv, 0), optArg(argv, 1), false); st != 0 {
		return st, nil
	}

	p := env.Process
	pri, prvpri := optArg(argv, 2)&maxPriority, optArg(argv, 3)

	// Without ALTPRI, no higher than the authorized priority.
	if !p.hasPrivilege(privALTPRI) {
		pri = min(pri, p.AuthorizedPriority)
	}

	if prvpri != 0 {
		if err := env.mem.StoreLongword(env.cpu, prvpri, p.BasePriority); err != nil {
			return ssAccVio, nil
		}
	}

	p.BasePriority, p.Priority = pri, pri

	if err := env.sched.SetBasePriority(handle(env), int(pri)); err == nil {
		env.requestReschedule()
	}

	return ssNormal, nil
}

// exitEntryAddr is the SYS$EXIT P1-vector entry: a procedure whose XFC
// calls $EXIT with its argument list's first argument as the status.
var exitEntryAddr = p1VectorAddr("SYS$EXIT")

// serviceSysForcex is SYS$FORCEX:
//
//	SYS$FORCEX [pidadr] ,[prcnam] ,[code]
//
// It makes the target process call $EXIT with status code (0 if
// omitted), as VMS does: by queuing a user-mode AST whose routine is
// $EXIT itself — the SYS$EXIT vector entry — with code as its parameter,
// which is the first argument an AST routine gets, so $EXIT reads it as
// its status. The image then exits normally, exit handlers and all, as
// soon as a user-mode AST can be delivered: at once if the caller is in
// user mode with ASTs enabled, and not while user-mode ASTs are disabled
// or the CPU is in a more privileged mode. A forced exit already queued
// isn't queued again. The target is picked by processTarget.
func serviceSysForcex(env *Environment, argv []uint32) (uint32, error) {
	if st := env.callerTarget(optArg(argv, 0), optArg(argv, 1), false); st != 0 {
		return st, nil
	}

	for _, a := range env.Process.ast.queue {
		if a.routine == exitEntryAddr && a.mode == uint32(vax.User) {
			return ssNormal, nil
		}
	}

	env.queueAST(exitEntryAddr, optArg(argv, 2), uint32(vax.User))

	return ssNormal, nil
}

// serviceSysSetrwm is SYS$SETRWM (docs/PHASE-26.md subtask 37):
//
//	SYS$SETRWM [watflg]
//
// It sets the process's resource wait mode: watflg 0 (the default)
// enables it, so services wait for a resource they need (room in a full
// mailbox) to become available; 1 disables it, so they fail at once. It
// returns SS$_WASCLR if resource wait mode was enabled before, SS$_WASSET
// if it was disabled.
func serviceSysSetrwm(env *Environment, argv []uint32) (uint32, error) {
	status := uint32(ssWasClr)
	if env.Process.ResourceWaitDisabled {
		status = ssWasSet
	}

	env.Process.ResourceWaitDisabled = optArg(argv, 0)&1 != 0

	return status, nil
}

// serviceSysDelprc is SYS$DELPRC:
//
//	SYS$DELPRC [pidadr] ,[prcnam]
//
// It deletes the target process, which can only be the caller: so it
// doesn't return. Unlike $FORCEX, no exit handlers run: they're all
// forgotten, and the image ends (ErrExit, as $EXIT ends it) with status
// SS$_NORMAL. On VMS the process is then gone; govax has no logging out,
// so the console carries on with the same process, as after any image.
// The target is picked by processTarget (SS$_NONEXPR, SS$_IVLOGNAM,
// SS$_ACCVIO).
func serviceSysDelprc(env *Environment, argv []uint32) (uint32, error) {
	if st := env.callerTarget(optArg(argv, 0), optArg(argv, 1), false); st != 0 {
		return st, nil
	}

	p := env.Process
	p.exitHandlers = [4][]uint32{}
	p.ExitStatus = ssNormal

	return ssNormal, ErrExit
}

func registerProcessServices(t *ServiceTable) {
	t.Register("SYS$ADJSTK", serviceSysAdjstk)
	t.Register("SYS$ADJWSL", serviceSysAdjwsl)
	t.Register("SYS$SETPRN", serviceSysSetprn)
	t.Register("SYS$SETPRI", serviceSysSetpri)
	t.Register("SYS$FORCEX", serviceSysForcex)
	t.Register("SYS$DELPRC", serviceSysDelprc)
	t.Register("SYS$SETRWM", serviceSysSetrwm)
	t.Register("SYS$CREPRC", serviceSysCreprc)
}
