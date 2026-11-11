package corevms

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// This file connects internal/rms's service handlers to the service
// table, so a program's SYS$OPEN, SYS$GET, and the rest reach them; checks
// that each call's FAB or RAB is one; and calls the completion routines a
// program gives them.
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
// # Ill-formed calls
//
// "Every RMS service call requires an interface data structure (FAB or
// RAB) which is writable in caller's mode and not currently in use for
// another operation" (the OpenVMS RMS Reference Manual, 2.4.1). One that
// isn't is an ill-formed call: RMS$_FAB or RMS$_RAB for a block the
// caller can't write or whose FAB$B_BID or RAB$B_BID isn't a FAB's or a
// RAB's, RMS$_BLN for one whose FAB$B_BLN or RAB$B_BLN is shorter than the
// block, and RMS$_BUSY for one still in use. The status is only R0:
// RMS can't trust the block, so it stores nothing there, and calls no
// completion routine. rmsBlockFault makes the checks before the service
// runs. A block is in use while a service on it waits (a terminal $GET
// with no line yet, a record lock, a mailbox $PUT; the process may take
// an AST meanwhile, and the AST's call on the same block is the one
// refused), and while an operation on it is RMS$_PENDING, until the
// $WAIT that finishes it. Which of RMS$_BUSY and RMS$_RSA ("record stream
// currently active") VMS gives a second operation on a RAB with one
// pending, and the checks' order, are unconfirmed.
//
// # Completion routines
//
// An RMS service's argument list is the FAB or RAB, then, optionally, an
// error and a success completion routine ($RENAME's new FAB comes after
// them). RMS calls the one the outcome chooses as an AST in the caller's
// mode, with the FAB's or RAB's address as its parameter. For an operation
// that completes during the call, "the AST routine executes prior to
// returning control from the service call, unless it is called from AST
// level or AST delivery is disabled" (the manual's 2.4.3): here the AST is
// queued as the service finishes, and the engine delivers it at the next
// instruction boundary, inside the service's vector entry before its RET.
// An operation left RMS$_PENDING (govax's only one is a $PUT to a mailbox
// with RAB$V_ASY) has its routines remembered, and $WAIT, which has none
// of its own, declares them when it finishes the operation (the manual's
// $WAIT).
func registerRMSServices(t *ServiceTable) {
	for name, fn := range rmsServices {
		t.Register(name, func(env *Environment, argv []uint32) (uint32, error) {
			if sts := env.rmsBlockFault(name, argv); sts != 0 {
				return sts, nil
			}

			ctx := env.rmsContext()
			r0, err := fn(ctx, argv)

			env.noteRMSBlockUse(name, argv[0], r0, err)

			// SET DEBUG RMS: the control block as the service left it
			// (rms's trace.go); a service that waits is shown when it
			// completes.
			if err == nil && env.cpu.DebugEnabled(registers.DebugRMS) {
				fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG(RMS): %s\n", rms.Trace(ctx, name, argv[0], r0))
			}

			if err == nil {
				env.rmsCompletion(name, argv, r0)
			}

			return r0, err
		})
	}
}

// The ill-formed calls' statuses, and the blocks' identifications.
var (
	rmsInvalidFAB = vmsdef.Symbols["RMS$_FAB"]
	rmsInvalidRAB = vmsdef.Symbols["RMS$_RAB"]
	rmsInvalidBLN = vmsdef.Symbols["RMS$_BLN"]
	rmsBusy       = vmsdef.Symbols["RMS$_BUSY"]

	fabBID, fabBLN = byte(vmsdef.Symbols["FAB$C_BID"]), byte(vmsdef.Symbols["FAB$C_BLN"])
	rabBID, rabBLN = byte(vmsdef.Symbols["RAB$C_BID"]), byte(vmsdef.Symbols["RAB$C_BLN"])
)

// rmsKind is which block a service takes.
type rmsKind int

const (
	rmsTakesFAB rmsKind = iota
	rmsTakesRAB
	rmsTakesEither // $WAIT: the block of the operation it waits for
)

// rmsBlockKinds is each service's block; a service not here takes a FAB.
var rmsBlockKinds = map[string]rmsKind{
	"SYS$CONNECT": rmsTakesRAB, "SYS$DISCONNECT": rmsTakesRAB,
	"SYS$GET": rmsTakesRAB, "SYS$PUT": rmsTakesRAB, "SYS$FIND": rmsTakesRAB,
	"SYS$UPDATE": rmsTakesRAB, "SYS$DELETE": rmsTakesRAB, "SYS$TRUNCATE": rmsTakesRAB,
	"SYS$REWIND": rmsTakesRAB, "SYS$FLUSH": rmsTakesRAB, "SYS$FREE": rmsTakesRAB,
	"SYS$RELEASE": rmsTakesRAB, "SYS$READ": rmsTakesRAB, "SYS$WRITE": rmsTakesRAB,
	"SYS$WAIT": rmsTakesEither,
}

// rmsInUseKey is the ImageData key of the image's RMS blocks in use: a
// map from a block's address to the frame pointer of the service call
// waiting on it, or 0 for an operation pending until its $WAIT.
type rmsInUseKey struct{}

func (env *Environment) rmsInUse() map[uint32]uint32 {
	return env.ImageData(rmsInUseKey{}, func() any { return map[uint32]uint32{} }).(map[uint32]uint32)
}

// rmsBlockFault returns the status of an ill-formed call (see this file's
// comment), or 0 for a well-formed one. argv may be empty: a call with no
// block names none.
func (env *Environment) rmsBlockFault(name string, argv []uint32) uint32 {
	block := optArg(argv, 0)

	kind := rmsBlockKinds[name]
	if kind == rmsTakesEither {
		kind = rmsTakesRAB
		if bid, err := env.mem.LoadByte(env.cpu, block); err == nil && bid == fabBID {
			kind = rmsTakesFAB
		}
	}

	if sts := env.rmsBlockCheck(block, kind); sts != 0 {
		return sts
	}

	// $RENAME's fourth argument is the new name's FAB.
	if name == "SYS$RENAME" {
		if sts := env.rmsBlockCheck(optArg(argv, 3), rmsTakesFAB); sts != 0 {
			return sts
		}
	}

	if fp, ok := env.rmsInUse()[block]; ok {
		if fp == 0 && name != "SYS$WAIT" || fp != 0 && fp != env.cpu.GPR(registers.FP) {
			return rmsBusy
		}
	}

	return 0
}

// rmsBlockCheck checks that block is a FAB or a RAB, as kind says: writable
// in the caller's mode for the block's whole length (the PTEs' protection
// only, as PROBEW tests it, so no page is brought in), with the block's
// identification and a length at least the block's.
func (env *Environment) rmsBlockCheck(block uint32, kind rmsKind) uint32 {
	bid, bln, bad := fabBID, fabBLN, rmsInvalidFAB
	if kind == rmsTakesRAB {
		bid, bln, bad = rabBID, rabBLN, rmsInvalidRAB
	}

	last := block + uint32(bln) - 1
	if last < block || !env.mem.ProbeAccess(env.cpu, block, vm.AccessWrite) || !env.mem.ProbeAccess(env.cpu, last, vm.AccessWrite) {
		return bad
	}

	gotBID, err := env.mem.LoadByte(env.cpu, block)
	if err != nil || gotBID != bid {
		return bad
	}

	gotBLN, err := env.mem.LoadByte(env.cpu, block+1)
	if err != nil {
		return bad
	}

	if gotBLN < bln {
		return rmsInvalidBLN
	}

	return 0
}

// noteRMSBlockUse records whether a service on block left it in use: a
// service that waits holds it for its own frame, one left RMS$_PENDING
// holds it for its $WAIT, and one that completed, or the $WAIT that
// finished it, lets it go.
func (env *Environment) noteRMSBlockUse(name string, block, r0 uint32, err error) {
	inUse := env.rmsInUse()

	switch {
	case errors.Is(err, ErrWait):
		inUse[block] = env.cpu.GPR(registers.FP)
	case err == nil && r0 == rmsPending && name != "SYS$WAIT":
		inUse[block] = 0
	default:
		delete(inUse, block)
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

	// A status the service couldn't store (an ill-formed block found by
	// the service itself) calls no routine either.
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
