package corevms

import (
	"encoding/binary"
	"errors"

	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// The lock management services (Phase 47, subtask 7): $ENQ, $ENQW, and
// $DEQ, over the system's lock database (System.Locks, internal/lck),
// as the System Services Reference Manual describes them.
//
// A request's lock status block (LKSB) is 8 bytes: the request's final
// status in its first word, a reserved word, and the lock ID in its
// second longword; with LCK$M_VALBLK, the resource's 16-byte value block
// follows. $ENQ writes the lock ID when it queues the request, and the
// status, the value block, the event flag, and the completion AST when
// the request completes: at once, or later, when another process's $DEQ
// or conversion (or its deletion) lets the lock be granted. The lock
// manager reports that as an event for the lock, which the request's
// enqRequest (the lock's lck.Data) carries out in its owner's process:
// the LKSB is written through the owner's address space, and the owner,
// if it waits, is made computable (reportEvent), as an I/O completion
// does. A blocking AST is delivered the same way.

// The lock services' statuses.
var (
	ssSynch       = vmsdef.Symbols["SS$_SYNCH"]
	ssNotQueued   = vmsdef.Symbols["SS$_NOTQUEUED"]
	ssCvtUngrant  = vmsdef.Symbols["SS$_CVTUNGRANT"]
	ssIvLockID    = vmsdef.Symbols["SS$_IVLOCKID"]
	ssParNotGrant = vmsdef.Symbols["SS$_PARNOTGRANT"]
	ssSublocks    = vmsdef.Symbols["SS$_SUBLOCKS"]
	ssCancelGrant = vmsdef.Symbols["SS$_CANCELGRANT"]
	ssValNotValid = vmsdef.Symbols["SS$_VALNOTVALID"]
	ssIvBufLen    = vmsdef.Symbols["SS$_IVBUFLEN"]
	ssNoSysLck    = vmsdef.Symbols["SS$_NOSYSLCK"]
)

// The $ENQ and $DEQ flags.
var (
	lckValBlk    = vmsdef.Symbols["LCK$M_VALBLK"]
	lckConvert   = vmsdef.Symbols["LCK$M_CONVERT"]
	lckNoQueue   = vmsdef.Symbols["LCK$M_NOQUEUE"]
	lckSyncSts   = vmsdef.Symbols["LCK$M_SYNCSTS"]
	lckSystem    = vmsdef.Symbols["LCK$M_SYSTEM"]
	lckDeqAll    = vmsdef.Symbols["LCK$M_DEQALL"]
	lckCancel    = vmsdef.Symbols["LCK$M_CANCEL"]
	lckInvValBlk = vmsdef.Symbols["LCK$M_INVVALBLK"]
)

var privSYSLCK = privilegeBit("SYSLCK")

// lockBoost is the priority boost a granted lock gives its waiting owner:
// the book's "Resource Available" (unconfirmed: $ENQ completes through
// an event flag, which govax otherwise boosts as an I/O completion).
const lockBoost = sched.ClassResourceAvailable

// enqRequest is one $ENQ's request, kept as its lock's Data until it
// completes: how to tell its owner.
type enqRequest struct {
	env    *Environment
	lksb   uint32
	efn    uint32
	astadr uint32
	astprm uint32
	blkast uint32
	mode   uint32 // the caller's access mode, which its ASTs run in
	valblk bool

	done bool
}

// Notify carries out a lock event for the request's owner (lck.Notifier).
func (r *enqRequest) Notify(ev lck.Event) {
	switch ev.Kind {
	case lck.EventGranted:
		r.complete(ev.Lock, grantStatus(ev.Lock))
	case lck.EventAborted:
		r.complete(ev.Lock, ssAbort)
	case lck.EventCanceled:
		r.complete(ev.Lock, ssCancel)
	case lck.EventBlocking:
		if r.blkast != 0 {
			r.env.queueAST(r.blkast, r.astprm, r.mode)
			r.env.reportEvent(lockBoost)
		}
	}
}

// grantStatus is a granted lock's completion status: SS$_VALNOTVALID if
// it asked for the value block and the resource's is marked invalid.
func grantStatus(l *lck.Lock) uint32 {
	if l.ValueBlock && l.ValueNotValid {
		return ssValNotValid
	}

	return ssNormal
}

// complete finishes the request with status: the LKSB (and value
// block), the event flag, the completion AST, and the owner told.
func (r *enqRequest) complete(l *lck.Lock, status uint32) {
	if r.done {
		return
	}

	r.done = true
	r.writeLKSB(l, status)
	r.env.postFlag(r.efn, lockBoost)

	if r.astadr != 0 {
		r.env.queueAST(r.astadr, r.astprm, r.mode)
	}

	r.env.reportEvent(lockBoost)
}

// writeLKSB writes the request's status block: status (0 while pending)
// and l's ID, and l's copy of the value block if the request asked for
// it and l is granted.
func (r *enqRequest) writeLKSB(l *lck.Lock, status uint32) {
	var b [8]byte

	binary.LittleEndian.PutUint16(b[0:], uint16(status))
	binary.LittleEndian.PutUint32(b[4:], uint32(l.ID))
	_ = r.env.storeOwn(r.lksb, b[:])

	if r.valblk && l.State == lck.Granted && status != ssAbort {
		_ = r.env.storeOwn(r.lksb+8, l.Value[:])
	}
}

// lockErrorStatus is the status for a lock manager error.
func lockErrorStatus(err error) uint32 {
	switch {
	case errors.Is(err, lck.ErrBadMode):
		return ssBadParam
	case errors.Is(err, lck.ErrBadName):
		return ssIvBufLen
	case errors.Is(err, lck.ErrParentNotGranted):
		return ssParNotGrant
	case errors.Is(err, lck.ErrNotQueued):
		return ssNotQueued
	case errors.Is(err, lck.ErrNotGranted):
		return ssCvtUngrant
	case errors.Is(err, lck.ErrSublocks):
		return ssSublocks
	case errors.Is(err, lck.ErrCancelGranted):
		return ssCancelGrant
	}

	return ssIvLockID
}

// enqueue is $ENQ's work, for $ENQ and $ENQW: it returns R0's status
// and, for a request queued but not yet granted, the request, which
// completes later.
//
//	SYS$ENQ [efn] ,lkmode ,lksb ,[flags] ,[resnam] ,[parid] ,[astadr]
//	        ,[astprm] ,[blkast] ,[acmode] ,[rsdm_id] ,[nullarg]
func (env *Environment) enqueue(argv []uint32) (uint32, *enqRequest) {
	efn, lkmode, lksb, flags := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2), optArg(argv, 3)
	resnam, parid := optArg(argv, 4), optArg(argv, 5)
	caller := uint32(env.cpu.PSL().CurMod())

	req := &enqRequest{
		env: env, lksb: lksb, efn: efn,
		astadr: optArg(argv, 6), astprm: optArg(argv, 7), blkast: optArg(argv, 8),
		mode: caller, valblk: flags&lckValBlk != 0,
	}

	// The LKSB must be readable and writable: a conversion's lock ID is
	// read from it, and the status written to it.
	var lksbBytes [24]byte

	size := uint32(8)
	if req.valblk {
		size = 24
	}

	if lksb == 0 || env.mem.Load(env.cpu, lksb, lksbBytes[:size]) != nil || env.storeOwn(lksb, lksbBytes[:size]) != nil {
		return ssAccVio, nil
	}

	word, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	*word &^= 1 << bit

	owner := lck.Owner(env.Process.PID)

	var (
		l      *lck.Lock
		events []lck.Event
		err    error
	)

	if flags&lckConvert != 0 {
		var value lck.ValueBlock

		copy(value[:], lksbBytes[8:24])

		id := lck.ID(binary.LittleEndian.Uint32(lksbBytes[4:]))
		if held, ok := env.Locks.Lock(id); ok && held.Owner == owner && uint32(held.AccessMode) < caller {
			return ssIvLockID, nil // a lock of a more privileged mode
		}

		l, events, err = env.Locks.Convert(owner, id, lck.Mode(lkmode), lck.ConvertOptions{
			NoQueue: flags&lckNoQueue != 0, ValueBlock: req.valblk, Value: value,
			Blocking: req.blkast != 0, Data: req,
		})
	} else {
		name, ok, rerr := strGet(env, resnam, lck.MaxNameLength)

		switch {
		case resnam == 0 || rerr != nil:
			return ssAccVio, nil
		case !ok || name == "":
			return ssIvBufLen, nil
		}

		group := uint16(env.Process.UICGroup())
		if flags&lckSystem != 0 {
			if caller > uint32(vax.Executive) && !env.Process.hasPrivilege(privSYSLCK) {
				return ssNoSysLck, nil
			}

			group = 0
		}

		l, events, err = env.Locks.Enqueue(lck.Request{
			Owner: owner, Mode: lck.Mode(lkmode), Name: name, Group: group,
			AccessMode: uint8(max(optArg(argv, 9)&3, caller)), Parent: lck.ID(parid),
			NoQueue: flags&lckNoQueue != 0, ValueBlock: req.valblk,
			Blocking: req.blkast != 0, Data: req,
		})
	}

	if err != nil {
		st := lockErrorStatus(err)
		if st == ssNotQueued {
			binary.LittleEndian.PutUint16(lksbBytes[0:], uint16(st))
			_ = env.storeOwn(lksb, lksbBytes[:2])
		}

		return st, nil
	}

	defer lck.Deliver(events)

	if l.State != lck.Granted {
		req.writeLKSB(l, 0)

		return ssNormal, req
	}

	// Granted at once. With LCK$M_SYNCSTS that's SS$_SYNCH, and no event
	// flag or AST; otherwise the request completes as a later grant
	// would.
	if flags&lckSyncSts != 0 {
		req.done = true
		req.writeLKSB(l, grantStatus(l))

		return ssSynch, nil
	}

	req.complete(l, grantStatus(l))

	return ssNormal, nil
}

// serviceSysEnq is SYS$ENQ: queue a new lock or a conversion, returning
// at once (see enqueue).
func serviceSysEnq(env *Environment, argv []uint32) (uint32, error) {
	st, _ := env.enqueue(argv)

	return st, nil
}

// enqWait is a $ENQW waiting for its request: fp is the SYS$ENQW stub's
// frame pointer, the same each time its XFC runs again.
type enqWait struct {
	fp  uint32
	req *enqRequest
}

// serviceSysEnqw is SYS$ENQW: $ENQ, returning when the request has
// completed (granted, or aborted or canceled by a $DEQ). Like $QIOW it
// waits in LEF, re-executing its XFC until then; the waits are a stack,
// since an AST routine may issue its own. Its R0 is $ENQ's; the final
// status is in the LKSB.
func serviceSysEnqw(env *Environment, argv []uint32) (uint32, error) {
	fp := env.cpu.GPR(vax.FP)

	if n := len(env.enqWaits); n > 0 && env.enqWaits[n-1].fp == fp {
		req := env.enqWaits[n-1].req
		if !req.done {
			return 0, env.waitOnLock(req)
		}

		env.enqWaits = env.enqWaits[:n-1]

		return ssNormal, nil
	}

	st, pending := env.enqueue(argv)
	if pending == nil {
		return st, nil
	}

	env.enqWaits = append(env.enqWaits, enqWait{fp: fp, req: pending})

	return 0, env.waitOnLock(pending)
}

// waitOnLock is a $ENQW's wait: LEF, as on the request's event flag,
// until the request completes.
func (env *Environment) waitOnLock(req *enqRequest) error {
	return env.waitOn(sched.StateLEF, sched.ResourceNone, lockBoost, func() bool { return req.done })
}

// serviceSysDeq is SYS$DEQ:
//
//	SYS$DEQ [lkid] ,[valblk] ,[acmode] ,[flags]
//
// It dequeues lock lkid, or with LCK$M_CANCEL cancels its queued
// conversion (or waiting request); with LCK$M_DEQALL, every lock the
// process has at the effective access mode and less privileged ones
// (lkid 0), or every sublock of lkid. The effective mode is the less
// privileged of the caller's and acmode. valblk is stored as the
// resource's value block when a PW or EX lock goes; LCK$M_INVVALBLK marks
// it invalid instead. Requests aborted or canceled, and others' locks
// granted because this one went, complete in their owners' processes.
func serviceSysDeq(env *Environment, argv []uint32) (uint32, error) {
	lkid, valblk, flags := optArg(argv, 0), optArg(argv, 1), optArg(argv, 3)
	mode := max(optArg(argv, 2)&3, uint32(env.cpu.PSL().CurMod()))
	owner := lck.Owner(env.Process.PID)

	if flags&lckDeqAll != 0 {
		events, err := env.Locks.DequeueAll(owner, uint8(mode), lck.ID(lkid))
		lck.Deliver(events)

		if err != nil {
			return lockErrorStatus(err), nil
		}

		return ssNormal, nil
	}

	l, ok := env.Locks.Lock(lck.ID(lkid))
	if lkid == 0 || !ok || l.Owner != owner || uint32(l.AccessMode) < mode {
		return ssIvLockID, nil
	}

	opts := lck.DequeueOptions{Cancel: flags&lckCancel != 0, Invalidate: flags&lckInvValBlk != 0}

	if valblk != 0 {
		var v lck.ValueBlock
		if env.mem.Load(env.cpu, valblk, v[:]) != nil {
			return ssAccVio, nil
		}

		opts.Value = &v
	}

	events, err := env.Locks.Dequeue(owner, l.ID, opts)
	lck.Deliver(events)

	if err != nil {
		return lockErrorStatus(err), nil
	}

	return ssNormal, nil
}

// registerLockServices registers $ENQ, $ENQW, and $DEQ.
func registerLockServices(t *ServiceTable) {
	t.Register("SYS$ENQ", serviceSysEnq)
	t.Register("SYS$ENQW", serviceSysEnqw)
	t.Register("SYS$DEQ", serviceSysDeq)
}
