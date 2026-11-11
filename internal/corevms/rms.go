package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// This file connects internal/rms's service handlers to the service
// table, so a program's SYS$OPEN, SYS$GET, and the rest reach them, and
// calls the completion routines a program gives them.
//
// # Why a wrapper closure per service
//
// ServiceTable.Register wants a ServiceFunc: func(*Environment, []uint32)
// (uint32, error). internal/rms's handlers are func(*rms.Context,
// []uint32) (uint32, error) instead: the same shape with a different
// first argument, because rms can't import this package (this package
// imports rms, and Go refuses an import cycle). Each wrapper builds an
// *rms.Context from the Environment the call was made in
// (env.rmsContext, environment.go) and passes argv on unchanged.
//
// # Completion routines
//
// An RMS service's argument list is the FAB or RAB, then, optionally, an
// error and a success completion routine ($RENAME's new FAB comes after
// them). RMS calls the one the outcome chooses as an AST in the caller's
// mode, with the FAB's or RAB's address as its parameter. For an operation
// that completes during the call, "the AST routine executes prior to
// returning control from the service call, unless it is called from AST
// level or AST delivery is disabled" (the OpenVMS RMS Reference Manual,
// 2.4.3): here the AST is queued as the service finishes, and the engine
// delivers it at the next instruction boundary, inside the service's
// vector entry before its RET. An operation left RMS$_PENDING (govax's
// only one is a $PUT to a mailbox with RAB$V_ASY) has its routines
// remembered, and $WAIT, which has none of its own, declares them when it
// finishes the operation (the manual's $WAIT). An ill-formed call, one
// whose status RMS couldn't store in the block's STS (a FAB or RAB that
// isn't one, or can't be written), calls no routine (2.4.1).
func registerRMSServices(t *ServiceTable) {
	for name, fn := range rmsServices {
		t.Register(name, func(env *Environment, argv []uint32) (uint32, error) {
			ctx := env.rmsContext()
			r0, err := fn(ctx, argv)

			// SET DEBUG RMS: the control block as the service left it
			// (rms's trace.go); a service that waits is shown when it
			// completes.
			if err == nil && len(argv) > 0 && env.cpu.DebugEnabled(registers.DebugRMS) {
				fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG(RMS): %s\n", rms.Trace(ctx, name, argv[0], r0))
			}

			if err == nil && len(argv) > 0 {
				env.rmsCompletion(name, argv, r0)
			}

			return r0, err
		})
	}
}

// rmsRoutines are an RMS call's completion routines (0 for none) and the
// mode they run in.
type rmsRoutines struct {
	err, suc, mode uint32
}

// pendingRMSKey is the ImageData key of an image's operations left
// RMS$_PENDING: a map from the FAB's or RAB's address to their routines.
type pendingRMSKey struct{}

// rmsPending is RMS$_PENDING, and rmsStatusOffset where FAB$L_STS and
// RAB$L_STS are (the same place in both).
var (
	rmsPending      = vmsdef.Symbols["RMS$_PENDING"]
	rmsStatusOffset = vmsdef.Symbols["FAB$L_STS"]
)

// rmsCompletion queues the completion routine for an RMS service that
// returned r0 (see this file's comment).
func (env *Environment) rmsCompletion(name string, argv []uint32, r0 uint32) {
	block := argv[0]
	pending := env.ImageData(pendingRMSKey{}, func() any { return map[uint32]rmsRoutines{} }).(map[uint32]rmsRoutines)

	routines := rmsRoutines{err: optArg(argv, 1), suc: optArg(argv, 2), mode: uint32(env.cpu.PSL().CurMod())}

	if name == "SYS$WAIT" {
		var ok bool
		if routines, ok = pending[block]; !ok || r0 == rmsPending {
			return
		}

		delete(pending, block)
	}

	if r0 == rmsPending {
		if routines.err != 0 || routines.suc != 0 {
			pending[block] = routines
		}

		return
	}

	routine := routines.err
	if r0&1 != 0 {
		routine = routines.suc
	}

	if routine == 0 {
		return
	}

	if sts, err := env.mem.LoadLongword(env.cpu, block+rmsStatusOffset); err != nil || sts != r0 {
		return
	}

	env.queueAST(routine, block, routines.mode)
}

// rmsServices are the RMS services, by name: Phase 22's and 33's,
// Phase 49's record operations past $GET and $PUT ($FIND, $UPDATE,
// $TRUNCATE, $DELETE, $REWIND), and Phase 88's $EXTEND.
var rmsServices = map[string]func(*rms.Context, []uint32) (uint32, error){
	"SYS$CREATE":     rms.SysCreate,
	"SYS$CONNECT":    rms.SysConnect,
	"SYS$OPEN":       rms.SysOpen,
	"SYS$CLOSE":      rms.SysClose,
	"SYS$GET":        rms.SysGet,
	"SYS$PUT":        rms.SysPut,
	"SYS$PARSE":      rms.SysParse,
	"SYS$SEARCH":     rms.SysSearch,
	"SYS$DISPLAY":    rms.SysDisplay,
	"SYS$RENAME":     rms.SysRename,
	"SYS$WAIT":       rms.SysWait,
	"SYS$FLUSH":      rms.SysFlush,
	"SYS$ERASE":      rms.SysErase,
	"SYS$EXTEND":     rms.SysExtend,
	"SYS$FREE":       rms.SysFree,
	"SYS$RELEASE":    rms.SysRelease,
	"SYS$FIND":       rms.SysFind,
	"SYS$UPDATE":     rms.SysUpdate,
	"SYS$TRUNCATE":   rms.SysTruncate,
	"SYS$DELETE":     rms.SysDelete,
	"SYS$REWIND":     rms.SysRewind,
	"SYS$READ":       rms.SysRead,
	"SYS$WRITE":      rms.SysWrite,
	"SYS$DISCONNECT": rms.SysDisconnect,
}
