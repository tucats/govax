package rtl

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// $GETJPI and $GETJPIW (docs/PHASE-26.md): information about the emulated
// process, read from rtl.Process. govax has one process, so a request
// either names it (no process given, PID 0, its own PID, its own name, or
// the first step of a wildcard) or names no process at all.

// Status codes $GETJPI returns.
var (
	ssNonExpr      = vmsdef.SSConstants["SS$_NONEXPR"]
	ssNoMoreProc   = vmsdef.SSConstants["SS$_NOMOREPROC"]
	jpiChain       = uint16(vmsdef.JPIConstants["JPI$_CHAIN"])
	jpiInteractive = vmsdef.JPIConstants["JPI$K_INTERACTIVE"]
	jpiLocal       = vmsdef.JPIConstants["JPI$K_LOCAL"]
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

// jpiValue is one item's data: a string, or a longword when str is false.
type jpiValue struct {
	str   bool
	text  string
	value uint32
}

func jpiString(s string) jpiValue            { return jpiValue{str: true, text: s} }
func jpiLong(v uint32) jpiValue              { return jpiValue{value: v} }
func padded(s string, width int) string      { return fmt.Sprintf("%-*s", width, s) }
func jpiPadded(s string, width int) jpiValue { return jpiString(padded(s, width)) }

// jpiItemsByName is the item-code registry, keyed by $JPIDEF name: what
// each supported item returns. Items not here are SS$_BADPARAM. The
// working-set items read rtl.Process's quota fields; JPI$_WSSIZE reports
// the current limit $ADJWSL adjusts, since govax has no real working set.
// The AST items describe Process.ast (see astModeMask and remainingASTs),
// and JPI$_STATE is always SCH$C_CUR: the process asking is, by
// definition, the one running.
var jpiItemsByName = map[string]func(env *Environment) jpiValue{
	"JPI$_ACCOUNT":    func(env *Environment) jpiValue { return jpiPadded(env.Process.Account, 8) },
	"JPI$_ASTACT":     func(env *Environment) jpiValue { return jpiLong(astModeMask(env.Process.ast.active)) },
	"JPI$_ASTCNT":     func(env *Environment) jpiValue { return jpiLong(env.remainingASTs()) },
	"JPI$_ASTEN":      func(env *Environment) jpiValue { return jpiLong(astModeMask(env.Process.ast.enabled)) },
	"JPI$_ASTLM":      func(env *Environment) jpiValue { return jpiLong(env.Process.ASTLimit) },
	"JPI$_PRI":        func(env *Environment) jpiValue { return jpiLong(env.Process.Priority) },
	"JPI$_PRIB":       func(env *Environment) jpiValue { return jpiLong(env.Process.BasePriority) },
	"JPI$_STATE":      func(env *Environment) jpiValue { return jpiLong(schStateCurrent) },
	"JPI$_CLINAME":    func(env *Environment) jpiValue { return jpiString(env.Process.CLIName) },
	"JPI$_DFWSCNT":    func(env *Environment) jpiValue { return jpiLong(env.Process.WSDefault) },
	"JPI$_EFCS":       func(env *Environment) jpiValue { return jpiLong(env.Process.LocalEventFlags[0]) },
	"JPI$_EFCU":       func(env *Environment) jpiValue { return jpiLong(env.Process.LocalEventFlags[1]) },
	"JPI$_GRP":        func(env *Environment) jpiValue { return jpiLong(env.Process.UICGroup()) },
	"JPI$_JOBTYPE":    func(env *Environment) jpiValue { return jpiLong(jpiLocal) },
	"JPI$_MASTER_PID": func(env *Environment) jpiValue { return jpiLong(env.Process.PID) },
	"JPI$_MEM":        func(env *Environment) jpiValue { return jpiLong(env.Process.UICMember()) },
	"JPI$_MODE":       func(env *Environment) jpiValue { return jpiLong(jpiInteractive) },
	"JPI$_OWNER":      func(env *Environment) jpiValue { return jpiLong(0) },
	"JPI$_PID":        func(env *Environment) jpiValue { return jpiLong(env.Process.PID) },
	"JPI$_PRCNAM":     func(env *Environment) jpiValue { return jpiString(env.Process.Name) },
	"JPI$_TERMINAL":   func(env *Environment) jpiValue { return jpiString(env.Process.Terminal) },
	"JPI$_UIC":        func(env *Environment) jpiValue { return jpiLong(env.Process.UIC) },
	"JPI$_USERNAME":   func(env *Environment) jpiValue { return jpiPadded(env.Process.Username, 12) },
	"JPI$_WSAUTH":     func(env *Environment) jpiValue { return jpiLong(env.Process.WSQuota) },
	"JPI$_WSAUTHEXT":  func(env *Environment) jpiValue { return jpiLong(env.Process.WSExtent) },
	"JPI$_WSEXTENT":   func(env *Environment) jpiValue { return jpiLong(env.Process.WSExtent) },
	"JPI$_WSQUOTA":    func(env *Environment) jpiValue { return jpiLong(env.Process.WSQuota) },
	"JPI$_WSSIZE":     func(env *Environment) jpiValue { return jpiLong(env.Process.WSLimit) },
}

// schStateCurrent is SCH$C_CUR, the state of the running process.
var schStateCurrent = vmsdef.STATEConstants["SCH$C_CUR"]

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
var jpiItems = func() map[uint16]func(*Environment) jpiValue {
	out := map[uint16]func(*Environment) jpiValue{}

	for name, fn := range jpiItemsByName {
		code, ok := vmsdef.JPIConstants[name]
		if !ok {
			panic("rtl: no $JPIDEF item code " + name)
		}

		out[uint16(code)] = fn
	}

	return out
}()

// storeJPIItem writes v into e's buffer, truncated to the buffer's length
// (a longword is stored low byte first), and its length to e's return
// length address.
func (env *Environment) storeJPIItem(e itemListEntry, v jpiValue) uint32 {
	data := v.text
	if !v.str {
		data = string([]byte{byte(v.value), byte(v.value >> 8), byte(v.value >> 16), byte(v.value >> 24)})
	}

	n, _, err := storeBuffer(env, e.BuffAddr, e.BuffLen, data)
	if err != nil {
		return ssAccVio
	}

	return env.setRetLen(e, n)
}

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

	if st := env.processTarget(pidadr, prcnam, true); st != 0 {
		return st, nil
	}

	status := env.walkItemListChain(itmlst, jpiChain, func(e itemListEntry) uint32 {
		item, ok := jpiItems[e.ItemCode]
		if !ok {
			return ssBadParam
		}

		return env.storeJPIItem(e, item(env))
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

// processTarget checks that pidadr/prcnam — the (PID by reference,
// process name by descriptor) pair many services use to pick a process —
// name this process, returning 0 if so. It writes the PID back to pidadr
// when that holds 0. A PID wins over a name; with neither, the caller is
// meant. SS$_NONEXPR for any other process, SS$_IVLOGNAM for a bad
// process name, SS$_ACCVIO for an unreadable or unwritable argument.
//
// With wildcard ($GETJPI only), a PID of -1 starts a wildcard scan and
// SS$_NOMOREPROC ends it (see jpiWildcard); other services treat -1 as
// just another PID that doesn't exist.
func (env *Environment) processTarget(pidadr, prcnam uint32, wildcard bool) uint32 {
	p := env.Process
	pid := uint32(0)

	if pidadr != 0 {
		v, err := env.mem.LoadLongword(env.cpu, pidadr)
		if err != nil {
			return ssAccVio
		}

		pid = v
	}

	writeBack := func(v uint32) uint32 {
		if err := env.mem.StoreLongword(env.cpu, pidadr, v); err != nil {
			return ssAccVio
		}

		return 0
	}

	switch {
	case wildcard && pid == jpiWildcard:
		return writeBack(jpiWildcardDone)

	case wildcard && pid == jpiWildcardDone:
		return ssNoMoreProc

	case pid != 0:
		if pid != p.PID {
			return ssNonExpr
		}

		return 0
	}

	if prcnam != 0 {
		name, ok, err := strGet(env, prcnam, maxProcessNameLength)
		if err != nil {
			return ssAccVio
		}

		if !ok || name == "" {
			return ssIvLogNam
		}

		if name != p.Name { // exactly: no abbreviation or trailing blanks
			return ssNonExpr
		}
	}

	if pidadr != 0 {
		return writeBack(p.PID)
	}

	return 0
}

func registerJPIServices(t *ServiceTable) {
	t.Register("SYS$GETJPI", serviceSysGetjpi)
	t.Register("SYS$GETJPIW", serviceSysGetjpi)
}
