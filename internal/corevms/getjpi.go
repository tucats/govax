package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// $GETJPI and $GETJPIW (docs/PHASE-26.md): information about the emulated
// process, read from corevms.Process. govax has one process, so a request
// either names it (no process given, PID 0, its own PID, its own name, or
// the first step of a wildcard) or names no process at all.

// Status codes $GETJPI returns.
var (
	ssNonExpr      = vmsdef.Symbols["SS$_NONEXPR"]
	ssNoMoreProc   = vmsdef.Symbols["SS$_NOMOREPROC"]
	jpiChain       = uint16(vmsdef.Symbols["JPI$_CHAIN"])
	jpiInteractive = vmsdef.Symbols["JPI$K_INTERACTIVE"]
	jpiLocal       = vmsdef.Symbols["JPI$K_LOCAL"]
)

// Wildcard $GETJPI contexts, the longword at pidadr. VMS starts a
// wildcard scan at -1 and keeps its own position there between calls;
// programs loop until SS$_NOMOREPROC without interpreting it. govax's one
// process is returned for -1, and jpiWildcardDone marks the scan finished.
const (
	jpiWildcard     = 0xFFFFFFFF
	jpiWildcardDone = 0xFFFFFFFE
)

// maxProcessNameLength is the longest process name $GETJPI's prcnam
// accepts (SS$_IVLOGNAM beyond it).
const maxProcessNameLength = 15

// jpiItemsByName is the item-code registry, keyed by $JPIDEF name: what
// each supported item returns. Items not here are SS$_BADPARAM. The
// working-set items read corevms.Process's quota fields; JPI$_WSSIZE reports
// the current limit $ADJWSL adjusts, since govax has no real working set.
// The AST items describe Process.ast (see astModeMask and remainingASTs),
// and JPI$_STATE is always SCH$C_CUR: the process asking is, by
// definition, the one running.
var jpiItemsByName = map[string]func(env *Environment) itemValue{
	"JPI$_ACCOUNT":    func(env *Environment) itemValue { return itemPadded(env.Process.Account, 8) },
	"JPI$_ASTACT":     func(env *Environment) itemValue { return itemLong(astModeMask(env.Process.ast.active)) },
	"JPI$_ASTCNT":     func(env *Environment) itemValue { return itemLong(env.remainingASTs()) },
	"JPI$_ASTEN":      func(env *Environment) itemValue { return itemLong(astModeMask(env.Process.ast.enabled)) },
	"JPI$_ASTLM":      func(env *Environment) itemValue { return itemLong(env.Process.ASTLimit) },
	"JPI$_AUTHPRI":    func(env *Environment) itemValue { return itemLong(env.Process.AuthorizedPriority) },
	"JPI$_AUTHPRIV":   func(env *Environment) itemValue { return itemQuad(env.Process.AuthorizedPrivileges) },
	"JPI$_CURPRIV":    func(env *Environment) itemValue { return itemQuad(env.Process.CurrentPrivileges) },
	"JPI$_IMAGPRIV":   func(env *Environment) itemValue { return itemQuad(env.Process.ImagePrivileges) },
	"JPI$_PROCPRIV":   func(env *Environment) itemValue { return itemQuad(env.Process.ProcessPrivileges) },
	"JPI$_PRI":        func(env *Environment) itemValue { return itemLong(env.currentPriority()) },
	"JPI$_CPUTIM":     func(env *Environment) itemValue { return itemLong(uint32(env.CPUTime(env) / 100_000)) }, // 10ms units
	"JPI$_PRIB":       func(env *Environment) itemValue { return itemLong(env.Process.BasePriority) },
	"JPI$_STATE":      func(env *Environment) itemValue { return itemLong(schStateCurrent) },
	"JPI$_CLINAME":    func(env *Environment) itemValue { return itemString(env.Process.CLIName) },
	"JPI$_DFWSCNT":    func(env *Environment) itemValue { return itemLong(env.Process.WSDefault) },
	"JPI$_EFCS":       func(env *Environment) itemValue { return itemLong(env.Process.LocalEventFlags[0]) },
	"JPI$_EFCU":       func(env *Environment) itemValue { return itemLong(env.Process.LocalEventFlags[1]) },
	"JPI$_GRP":        func(env *Environment) itemValue { return itemLong(env.Process.UICGroup()) },
	"JPI$_JOBTYPE":    func(env *Environment) itemValue { return itemLong(jpiLocal) },
	"JPI$_MASTER_PID": func(env *Environment) itemValue { return itemLong(env.Process.PID) },
	"JPI$_MEM":        func(env *Environment) itemValue { return itemLong(env.Process.UICMember()) },
	"JPI$_MODE":       func(env *Environment) itemValue { return itemLong(jpiInteractive) },
	"JPI$_OWNER":      func(env *Environment) itemValue { return itemLong(0) },
	"JPI$_PID":        func(env *Environment) itemValue { return itemLong(env.Process.PID) },
	"JPI$_PRCNAM":     func(env *Environment) itemValue { return itemString(env.Process.Name) },
	"JPI$_TERMINAL":   func(env *Environment) itemValue { return itemString(env.Process.Terminal) },
	"JPI$_UIC":        func(env *Environment) itemValue { return itemLong(env.Process.UIC) },
	"JPI$_USERNAME":   func(env *Environment) itemValue { return itemPadded(env.Process.Username, 12) },
	"JPI$_WSAUTH":     func(env *Environment) itemValue { return itemLong(env.Process.WSQuota) },
	"JPI$_WSAUTHEXT":  func(env *Environment) itemValue { return itemLong(env.Process.WSExtent) },
	"JPI$_WSEXTENT":   func(env *Environment) itemValue { return itemLong(env.Process.WSExtent) },
	"JPI$_WSQUOTA":    func(env *Environment) itemValue { return itemLong(env.Process.WSQuota) },
	"JPI$_WSSIZE":     func(env *Environment) itemValue { return itemLong(env.Process.WSLimit) },
}

// schStateCurrent is SCH$C_CUR, the state of the running process.
var schStateCurrent = vmsdef.Symbols["SCH$C_CUR"]

// astModeMask turns a per-mode flag array (AST enabled, AST active) into
// the bit vector $GETJPI reports: bit 0 for kernel mode, 1 executive, 2
// supervisor, 3 user.
func astModeMask(modes [4]bool) uint32 {
	var mask uint32

	for mode, set := range modes {
		if set {
			mask |= 1 << mode
		}
	}

	return mask
}

// remainingASTs is JPI$_ASTCNT: what's left of the AST quota. VMS charges
// an AST against the quota from the moment it's requested until it's
// delivered, so both queued ASTs and $SETIMR timers that will queue one
// count. (Nothing stops the count reaching 0: the quota isn't enforced.)
func (env *Environment) remainingASTs() uint32 {
	outstanding := uint32(len(env.Process.ast.queue))

	for _, t := range env.timers {
		if !t.wake && t.astadr != 0 {
			outstanding++
		}
	}

	return env.Process.ASTLimit - min(outstanding, env.Process.ASTLimit)
}

// jpiItems is jpiItemsByName keyed by item code.
var jpiItems = func() map[uint16]func(*Environment) itemValue {
	out := map[uint16]func(*Environment) itemValue{}

	for name, fn := range jpiItemsByName {
		code, ok := vmsdef.Symbols[name]
		if !ok {
			panic("rtl: no $JPIDEF item code " + name)
		}

		out[uint16(code)] = fn
	}

	return out
}()

// serviceSysGetjpi is SYS$GETJPI and SYS$GETJPIW:
//
//	SYS$GETJPI[W] [efn] ,[pidadr] ,[prcnam] ,itmlst ,[iosb] ,[astadr] ,[astprm]
//
// The process is picked as the manual's Table SYS-5 says: pidadr's PID if
// it's nonzero (and then prcnam is ignored), else prcnam, else the caller.
// A zero PID at pidadr is replaced by the PID found. -1 starts a wildcard
// scan, which returns this process and then SS$_NOMOREPROC.
//
// The request completes at once, so $GETJPI and $GETJPIW behave the same:
// the event flag (efn, default 0) is cleared and then set, iosb gets the
// final status, and, if astadr isn't 0, an AST is queued to call it with
// astprm in the caller's access mode. The AST usually runs as soon as the
// service returns (docs/PHASE-26.md subtask 16). A call rejected before
// the request starts (bad efn, too few arguments, no such process)
// completes nothing: no flag, IOSB status, or AST.
func serviceSysGetjpi(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 7 {
		return ssInsfArg, nil
	}

	efn, pidadr, prcnam, itmlst, iosb := argv[0], argv[1], argv[2], argv[3], argv[4]
	astadr, astprm := argv[5], argv[6]

	if env.cpu.DebugEnabled(vax.DebugProcess) {
		name := ""

		if prcnam != 0 {
			if s, ok, err := strGet(env, prcnam, 63); err == nil && ok {
				name = s
			}
		}

		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG: SYS$GETJPIW EFN=%d PRCNAM=%q\n", efn, name)
	}

	flags, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	*flags &^= 1 << bit

	if iosb != 0 {
		if env.mem.StoreLongword(env.cpu, iosb, 0) != nil || env.mem.StoreLongword(env.cpu, iosb+4, 0) != nil {
			return ssAccVio, nil
		}
	}

	if st := env.callerTarget(pidadr, prcnam, true); st != 0 {
		return st, nil
	}

	status := env.walkItemListChain(itmlst, jpiChain, func(e itemListEntry) uint32 {
		item, ok := jpiItems[e.ItemCode]
		if !ok {
			return ssBadParam
		}

		return env.storeItem(e, item(env))
	})
	if status == 0 {
		status = ssNormal
	}

	if iosb != 0 {
		if err := env.mem.StoreLongword(env.cpu, iosb, status); err != nil {
			return ssAccVio, nil
		}
	}

	*flags |= 1 << bit

	if astadr != 0 {
		env.queueAST(astadr, astprm, uint32(env.cpu.PSL().CurMod()))
	}

	return status, nil
}

// processTarget finds the process that pidadr/prcnam — the (PID by
// reference, process name by descriptor) pair many services use to pick
// a process — name, in the system's process table (proctable.go). A PID
// wins over a name; with neither, the caller is meant. A name is looked
// for in the caller's UIC group only, since process names are unique
// only within a group. When the longword at pidadr holds 0, the target's
// PID is written back to it. It returns the target, or a status: SS$_NONEXPR
// for a process that doesn't exist, SS$_IVLOGNAM for a bad process name,
// SS$_ACCVIO for an unreadable or unwritable argument.
//
// With wildcard ($GETJPI only), a PID of -1 starts a wildcard scan and
// SS$_NOMOREPROC ends it (see jpiWildcard); other services treat -1 as
// just another PID that doesn't exist.
func (env *Environment) processTarget(pidadr, prcnam uint32, wildcard bool) (*Environment, uint32) {
	pid := uint32(0)

	if pidadr != 0 {
		v, err := env.mem.LoadLongword(env.cpu, pidadr)
		if err != nil {
			return nil, ssAccVio
		}

		pid = v
	}

	writeBack := func(target *Environment, v uint32) (*Environment, uint32) {
		if err := env.mem.StoreLongword(env.cpu, pidadr, v); err != nil {
			return nil, ssAccVio
		}

		return target, 0
	}

	switch {
	case wildcard && pid == jpiWildcard:
		return writeBack(env, jpiWildcardDone)

	case wildcard && pid == jpiWildcardDone:
		return nil, ssNoMoreProc

	case pid != 0:
		target, found := env.FindProcess(pid)
		if !found {
			return nil, ssNonExpr
		}

		return target, 0
	}

	target := env

	if prcnam != 0 {
		name, ok, err := strGet(env, prcnam, maxProcessNameLength)
		if err != nil {
			return nil, ssAccVio
		}

		if !ok || name == "" {
			return nil, ssIvLogNam
		}

		found := false
		if target, found = env.FindProcessName(env.Process.UICGroup(), name); !found {
			return nil, ssNonExpr
		}
	}

	if pidadr != 0 {
		return writeBack(target, target.Process.PID)
	}

	return target, 0
}

// callerTarget is processTarget for the services that so far act only on
// the calling process ($GETJPI, $SETPRI, $FORCEX, $DELPRC, $SCHDWK,
// $CANWAK; $WAKE reaches any process since docs/PHASE-44.md, subtask 4): naming any other process is SS$_NONEXPR, as it was
// when govax had one process. Acting on another process — its event
// flags, ASTs, timers, and deletion, which must reach it even while it
// isn't current — arrives with process creation (docs/PHASE-45.md).
func (env *Environment) callerTarget(pidadr, prcnam uint32, wildcard bool) uint32 {
	target, st := env.processTarget(pidadr, prcnam, wildcard)
	if st != 0 {
		return st
	}

	if target != env {
		return ssNonExpr
	}

	return 0
}

func registerJPIServices(t *ServiceTable) {
	t.Register("SYS$GETJPI", serviceSysGetjpi)
	t.Register("SYS$GETJPIW", serviceSysGetjpi)
}

// currentPriority is the process's current priority as $GETJPI's
// JPI$_PRI reports it: the scheduler's, which includes any boost from the
// event that ended its last wait, as VMS's does (docs/PHASE-44.md).
func (env *Environment) currentPriority() uint32 {
	if info, ok := env.sched.Info(handle(env)); ok {
		return uint32(info.Priority)
	}

	return env.Process.Priority
}
