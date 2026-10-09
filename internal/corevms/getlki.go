package corevms

import (
	"encoding/binary"
	"math"

	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// $GETLKI and $GETLKIW (docs/PHASE-49.md, subtask 6): information about
// a lock in the lock database, from the System Services Reference
// Manual. The lock is named by its lock ID (the second longword of the
// lock status block $ENQ filled in); a lock ID of 0 or -1 asks for a
// wildcard scan, one lock per call, over every lock the caller may look
// at, ending with SS$_NOMORELOCK.
//
// Who may look at a lock:
//
//   - a caller whose access mode is the lock's or a more privileged one
//     (SS$_IVMODE): a user-mode program doesn't see RMS's
//     executive-mode record locks;
//   - for a lock on a system-wide resource (LCK$M_SYSTEM), a caller in
//     executive or kernel mode, or with SYSLCK (SS$_NOSYSLCK);
//   - for another process's lock, a caller in the same UIC group, or
//     with WORLD (SS$_NOWORLD).
//
// A wildcard scan skips the locks the caller may not look at.
//
// Every item but the lists (LKI$_BLOCKEDBY, LKI$_BLOCKING, LKI$_LOCKS)
// is a fixed-size value; a list is one 24-byte entry (LKI$C_LENGTH) per
// lock. The return length is a longword: the bytes written in bits 0 to
// 15, a list entry's size in bits 16 to 30, and bit 31 set when the
// buffer was too small (the status is still SS$_NORMAL). govax writes
// as many whole entries of a list as fit (unconfirmed). govax is no
// cluster, so the cluster system IDs are 0 and the master copy of a lock
// is the lock itself.
//
// The request completes at once, as $GETJPI's does, so $GETLKI and
// $GETLKIW behave the same: the event flag (efn, default 0) is cleared
// and then set, the IOSB gets the final status, and an AST, if astadr
// isn't 0, is queued in the caller's access mode.

// The lock information services' statuses.
var (
	ssNoMoreLock = vmsdef.Symbols["SS$_NOMORELOCK"]
	ssIvMode     = vmsdef.Symbols["SS$_IVMODE"]
	ssNoWorld    = vmsdef.Symbols["SS$_NOWORLD"]
)

// The queue codes a lock's state reports: LKI$C_GRANTED, LKI$C_CONVERT,
// and LKI$C_WAITING (-1, which as a byte is 0xFF).
var lkiQueue = map[lck.State]byte{
	lck.Granted:    byte(vmsdef.Symbols["LKI$C_GRANTED"]),
	lck.Converting: byte(vmsdef.Symbols["LKI$C_CONVERT"]),
	lck.Waiting:    byte(vmsdef.Symbols["LKI$C_WAITING"]),
}

// lkiEntryLength is LKI$C_LENGTH, the size of one lock's entry in a list
// item: LKI$L_MSTLKID, LKI$L_PID, LKI$L_MSTCSID, LKI$B_RQMODE,
// LKI$B_GRMODE, LKI$B_QUEUE, a spare byte, LKI$L_LKID, LKI$L_CSID.
var lkiEntryLength = int(vmsdef.Symbols["LKI$C_LENGTH"])

// lkiWildcardContext marks a lock ID longword as a wildcard scan's
// place: ^XFE in the high byte, with the last lock ID returned below it
// (lkiContextIndex; govax's lock IDs are small). VMS 7.3 wrote its
// context so (testdata/probe49, step 5a: FE0000E6 after lock 1A0000E6,
// whose high byte is a sequence number govax's IDs don't have).
const (
	lkiWildcardContext = 0xFE000000
	lkiContextIndex    = 0x00FFFFFF
)

// lkiItem is one $GETLKI item's value for a lock: its bytes, and for a
// list item the size of one entry.
type lkiItem func(l *lck.Lock) (data []byte, entry int)

// lkiLong is an item that is a longword.
func lkiLong(v func(l *lck.Lock) uint32) lkiItem {
	return func(l *lck.Lock) ([]byte, int) {
		return binary.LittleEndian.AppendUint32(nil, v(l)), 0
	}
}

// lkiList is a list item: an entry for each lock locks returns.
func lkiList(locks func(l *lck.Lock) []*lck.Lock) lkiItem {
	return func(l *lck.Lock) ([]byte, int) {
		var out []byte

		for _, o := range locks(l) {
			out = append(out, lkiEntry(o)...)
		}

		return out, lkiEntryLength
	}
}

// lkiEntry is lock l's entry in a list item.
func lkiEntry(l *lck.Lock) []byte {
	b := make([]byte, lkiEntryLength)
	binary.LittleEndian.PutUint32(b[vmsdef.Symbols["LKI$L_MSTLKID"]:], uint32(l.ID))
	binary.LittleEndian.PutUint32(b[vmsdef.Symbols["LKI$L_PID"]:], uint32(l.Owner))
	b[vmsdef.Symbols["LKI$B_RQMODE"]] = byte(l.Requested)
	b[vmsdef.Symbols["LKI$B_GRMODE"]] = grantedMode(l)
	b[vmsdef.Symbols["LKI$B_QUEUE"]] = lkiQueue[l.State]
	binary.LittleEndian.PutUint32(b[vmsdef.Symbols["LKI$L_LKID"]:], uint32(l.ID))

	return b
}

// grantedMode is the mode l is granted at, as $GETLKI reports it: a
// waiting lock has none, reported as NL (unconfirmed).
func grantedMode(l *lck.Lock) byte {
	if l.State == lck.Waiting {
		return byte(lck.NL)
	}

	return byte(l.Mode)
}

// lkiItems is $GETLKI's item codes. The cluster items are 0, and a
// lock's master copy and remote lock ID are the lock itself.
var lkiItems = map[uint16]lkiItem{}

func init() {
	id := func(l *lck.Lock) uint32 { return uint32(l.ID) }
	zero := func(*lck.Lock) uint32 { return 0 }
	counts := func(which int) func(*lck.Lock) uint32 {
		return func(l *lck.Lock) uint32 {
			g, c, w := l.Resource.QueueLengths()

			return uint32([]int{g, c, w}[which])
		}
	}

	longs := map[string]func(*lck.Lock) uint32{
		"LKI$_PID":        func(l *lck.Lock) uint32 { return uint32(l.Owner) },
		"LKI$_LOCKID":     id,
		"LKI$_LKID":       id,
		"LKI$_MSTLKID":    id,
		"LKI$_REMLKID":    id,
		"LKI$_CSID":       zero,
		"LKI$_MSTCSID":    zero,
		"LKI$_SYSTEM":     zero,
		"LKI$_LCKREFCNT":  func(l *lck.Lock) uint32 { return uint32(l.Sublocks()) },
		"LKI$_RSBREFCNT":  func(l *lck.Lock) uint32 { return uint32(l.Resource.Subresources()) },
		"LKI$_GRANTCOUNT": counts(0),
		"LKI$_LCKCOUNT":   counts(0),
		"LKI$_CVTCOUNT":   counts(1),
		"LKI$_WAITCOUNT":  counts(2),
		"LKI$_NAMSPACE":   namespace,
		"LKI$_PARENT": func(l *lck.Lock) uint32 {
			if l.Parent == nil {
				return 0
			}

			return uint32(l.Parent.ID)
		},
	}

	for name, v := range longs {
		lkiItems[uint16(vmsdef.Symbols[name])] = lkiLong(v)
	}

	lkiItems[uint16(vmsdef.Symbols["LKI$_STATE"])] = func(l *lck.Lock) ([]byte, int) {
		return []byte{byte(l.Requested), grantedMode(l), lkiQueue[l.State]}, 0
	}
	lkiItems[uint16(vmsdef.Symbols["LKI$_RESNAM"])] = func(l *lck.Lock) ([]byte, int) {
		return []byte(l.Resource.Name), 0
	}
	lkiItems[uint16(vmsdef.Symbols["LKI$_VALBLK"])] = func(l *lck.Lock) ([]byte, int) {
		return l.Resource.Value[:], 0
	}
	lkiItems[uint16(vmsdef.Symbols["LKI$_BLOCKING"])] = lkiList(lck.Blockers)
	lkiItems[uint16(vmsdef.Symbols["LKI$_BLOCKEDBY"])] = lkiList(lck.BlockedBy)
	lkiItems[uint16(vmsdef.Symbols["LKI$_LOCKS"])] = lkiList(func(l *lck.Lock) []*lck.Lock {
		return l.Resource.Locks()
	})
}

// namespace is LKI$_NAMSPACE: the resource's UIC group in bits 0 to 15
// (LKI$W_GROUP) unless the name is system-wide, which sets bit 31
// (LKI$V_SYSNAM); the resource's access mode in bits 16 to 23
// (LKI$B_RMOD).
func namespace(l *lck.Lock) uint32 {
	r := l.Resource
	v := uint32(r.AccessMode) << 16

	if r.Group == 0 {
		return v | uint32(vmsdef.Symbols["LKI$M_SYSNAM"])
	}

	return v | uint32(r.Group)
}

// lockAccess reports whether the caller may look at lock l, or the
// status why not (see this file's opening comment).
func (env *Environment) lockAccess(l *lck.Lock) uint32 {
	caller := uint32(env.cpu.PSL().CurMod())
	p := env.Process

	if caller > uint32(l.AccessMode) {
		return ssIvMode
	}

	if l.Resource.Group == 0 && caller > uint32(vax.Executive) && !p.hasPrivilege(privSYSLCK) {
		return ssNoSysLck
	}

	if uint32(l.Owner) != p.PID && !p.hasPrivilege(privWORLD) {
		owner, ok := env.FindProcess(uint32(l.Owner))
		if !ok || owner.Process.UICGroup() != p.UICGroup() {
			return ssNoWorld
		}
	}

	return 0
}

// serviceSysGetlki is SYS$GETLKI and SYS$GETLKIW:
//
//	SYS$GETLKI [efn] ,lkidadr ,itmlst ,[iosb] ,[astadr] ,[astprm] ,[nullarg]
//
// See this file's opening comment.
func serviceSysGetlki(env *Environment, argv []uint32) (uint32, error) {
	efn, lkidadr, itmlst, iosb := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2), optArg(argv, 3)
	astadr, astprm := optArg(argv, 4), optArg(argv, 5)

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

	status := env.lockInformation(lkidadr, itmlst)

	if iosb != 0 {
		if err := env.mem.StoreLongword(env.cpu, iosb, status); err != nil {
			return ssAccVio, nil
		}
	}

	env.postFlag(efn, sched.ClassIOCompletion)

	if astadr != 0 {
		env.queueAST(astadr, astprm, uint32(env.cpu.PSL().CurMod()))
	}

	return status, nil
}

// lockInformation is $GETLKI's work: find the lock (or the wildcard
// scan's next), and return its items.
func (env *Environment) lockInformation(lkidadr, itmlst uint32) uint32 {
	if lkidadr == 0 || env.Locks == nil {
		return ssNoMoreLock
	}

	lkid, err := env.mem.LoadLongword(env.cpu, lkidadr)
	if err != nil {
		return ssAccVio
	}

	var l *lck.Lock

	if lkid == 0 || lkid&^lkiContextIndex == lkiWildcardContext || lkid == math.MaxUint32 {
		// A wildcard scan: the next lock after the last one returned
		// that the caller may look at.
		after := lck.ID(0)
		if lkid != 0 && lkid != math.MaxUint32 {
			after = lck.ID(lkid & lkiContextIndex)
		}

		for _, o := range env.Locks.All() {
			if o.ID > after && env.lockAccess(o) == 0 {
				l = o

				break
			}
		}

		if l == nil {
			return ssNoMoreLock
		}

		if env.mem.StoreLongword(env.cpu, lkidadr, uint32(l.ID)|lkiWildcardContext) != nil {
			return ssAccVio
		}
	} else {
		var ok bool
		if l, ok = env.Locks.Lock(lck.ID(lkid)); !ok {
			return ssIvLockID
		}

		if st := env.lockAccess(l); st != 0 {
			return st
		}
	}

	status := env.walkItemList(itmlst, func(e itemListEntry) uint32 {
		item, ok := lkiItems[e.ItemCode]
		if !ok {
			return ssBadParam
		}

		data, entry := item(l)

		return env.storeLockItem(e, data, entry)
	})
	if status == 0 {
		status = ssNormal
	}

	return status
}

// storeLockItem writes one item's data to e's buffer, and its return
// length longword (see this file's opening comment): a list (entry not
// 0) keeps to whole entries.
func (env *Environment) storeLockItem(e itemListEntry, data []byte, entry int) uint32 {
	fits := min(len(data), int(e.BuffLen))
	if entry > 0 {
		fits -= fits % entry
	}

	if _, _, err := storeBuffer(env, e.BuffAddr, uint16(fits), string(data[:fits])); err != nil {
		return ssAccVio
	}

	if e.RetAddr == 0 {
		return 0
	}

	retlen := uint32(fits) | uint32(entry)<<16
	if fits < len(data) {
		retlen |= 1 << 31
	}

	if env.mem.StoreLongword(env.cpu, e.RetAddr, retlen) != nil {
		return ssAccVio
	}

	return 0
}
