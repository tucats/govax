package securertl

import (
	"encoding/binary"
	"errors"
	"slices"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// $CHANGE_ACL: an object's ACL read and changed by an item list
// (docs/planning/PHASE-93 - file protection.md, A.4's "Round 3"; the VMS
// 5.0 System Services manual).
//
// # The position
//
// The items work in order on one ACL and share a position in it, which a
// context longword carries from call to call: its low word is the
// position, 1 the first ACE, 0 the top, and %XFFFF (from a context of -1)
// the bottom; bit 24 says an ACE was found there. An item's ACE goes in
// before the ACE at the position (so at the top for 0, after the last ACE
// for the bottom), and the position moves past it; a read starts there and
// moves past what it read. VMS 7.1 also kept bits 16 to 23 of the context
// after some items (5 after READACE, 4 after an ADDACLENT that followed a
// find), whose meaning the probe didn't settle; govax leaves them 0.

// The item codes ($ACLDEF).
var (
	aclAdd       = uint16(vmsdef.Symbols["ACL$C_ADDACLENT"])
	aclDel       = uint16(vmsdef.Symbols["ACL$C_DELACLENT"])
	aclMod       = uint16(vmsdef.Symbols["ACL$C_MODACLENT"])
	aclFind      = uint16(vmsdef.Symbols["ACL$C_FNDACLENT"])
	aclFindType  = uint16(vmsdef.Symbols["ACL$C_FNDACETYP"])
	aclDeleteACL = uint16(vmsdef.Symbols["ACL$C_DELETEACL"])
	aclRead      = uint16(vmsdef.Symbols["ACL$C_READACL"])
	aclLength    = uint16(vmsdef.Symbols["ACL$C_ACLLENGTH"])
	aclReadACE   = uint16(vmsdef.Symbols["ACL$C_READACE"])
	aclRLock     = uint16(vmsdef.Symbols["ACL$C_RLOCK_ACL"])
	aclWLock     = uint16(vmsdef.Symbols["ACL$C_WLOCK_ACL"])
	aclUnlock    = uint16(vmsdef.Symbols["ACL$C_UNLOCK_ACL"])
	aclGrantACE  = uint16(vmsdef.Symbols["ACL$C_GRANT_ACE"])
	aclNextACE   = uint16(vmsdef.Symbols["ACL$C_NEXT_ACE"])
	aclDeleteAll = uint16(vmsdef.Symbols["ACL$C_DELETE_ALL"])

	objFile   = vmsdef.Symbols["ACL$C_FILE"]
	objDevice = vmsdef.Symbols["ACL$C_DEVICE"]
)

// changingItems are the items that change the ACL, which need control
// access; the others read it.
var changingItems = []uint16{aclAdd, aclDel, aclMod, aclDeleteACL, aclDeleteAll}

// knownItems are every item $CHANGE_ACL takes.
var knownItems = []uint16{
	aclAdd, aclDel, aclMod, aclFind, aclFindType, aclDeleteACL, aclRead, aclLength,
	aclReadACE, aclRLock, aclWLock, aclUnlock, aclGrantACE, aclNextACE, aclDeleteAll,
}

const (
	contextFound  = 1 << 24 // the context's found bit
	positionMask  = 0xFFFF  // the context's position
	itemEntrySize = 12      // an item_list_3 entry
)

// aclItem is one entry of the item list: its code, buffer, and the
// buffer's bytes for an item that gives the routine something.
type aclItem struct {
	code   uint16
	length uint16
	buffer uint32
	data   []byte
	aces   [][]byte // an ADDACLENT's, MODACLENT's, FNDACLENT's, or DELACLENT's
}

// aclTarget is the object whose ACL the call works on.
type aclTarget interface {
	ACL() []byte
	MayRead() bool
	MayChange() bool
	SetACL([]byte) error
	Profile() security.Profile
}

// changeACL is SYS$CHANGE_ACL [chan] ,objtyp ,[objnam] ,itmlst ,[acmode]
// ,[iosb] ,[contxt] ,[routin] ,[orbadr]. As VMS 7.1's (A.4): no objtyp is
// SS$_INSFARG, one it doesn't know SS$_BADPARAM, neither chan nor objnam
// SS$_NOSUCHOBJECT; an item code it doesn't know SS$_BADPARAM, and an
// ACE buffer that isn't whole ACEs SS$_IVACL, before anything is done; a
// change needs control access and a read read or control (SS$_NOPRIV).
// No item writes a return length. acmode is taken and not used: govax
// doesn't protect by access mode.
func changeACL(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	if arg(argv, 1) == 0 {
		return ssInsfArg, nil
	}

	objtyp, err := mem.LoadLongword(cpu, arg(argv, 1))
	if err != nil {
		return ssAccVio, nil
	}

	if objtyp != objFile && objtyp != objDevice {
		return ssBadParam, nil
	}

	chanNumber, objnam := arg(argv, 0), arg(argv, 2)
	if chanNumber == 0 && objnam == 0 {
		return ssNoSuchObj, nil
	}

	items, status := readItems(env, arg(argv, 3))
	if status != ssNormal {
		return status, nil
	}

	target, status := findTarget(env, objtyp, chanNumber, objnam)
	if status != ssNormal {
		return status, nil
	}

	changes := slices.ContainsFunc(items, func(it aclItem) bool { return slices.Contains(changingItems, it.code) })

	switch {
	case len(items) == 0:
		return ssNormal, nil
	case changes && !target.MayChange():
		return ssNoPriv, nil
	case !changes && !target.MayRead():
		return ssNoPriv, nil
	}

	e := &aclEditor{env: env, target: target, acl: splitACL(target.ACL())}

	ctxAddr := arg(argv, 6)
	if ctxAddr != 0 {
		v, err := mem.LoadLongword(cpu, ctxAddr)
		if err != nil {
			return ssAccVio, nil
		}

		// -1 is the bottom, its found bit no find's.
		e.pos, e.found = int(v&positionMask), v&contextFound != 0 && v != 0xFFFFFFFF
	}

	status = ssNormal

	for _, it := range items {
		if status = e.do(it); status != ssNormal {
			break
		}
	}

	if e.changed {
		if err := target.SetACL(joinACL(e.acl)); err != nil {
			return ssACLFull, nil
		}
	}

	if ctxAddr != 0 {
		v := uint32(e.pos & positionMask)

		switch {
		case status == ssNoEntry, e.zeroContext:
			v = 0
		case e.found:
			v |= contextFound
		}

		if mem.StoreLongword(cpu, ctxAddr, v) != nil {
			return ssAccVio, nil
		}
	}

	return status, nil
}

// readItems reads the item list at addr, to its longword of 0, checking
// each item's code and the ACEs in its buffer.
func readItems(env *corevms.Environment, addr uint32) ([]aclItem, uint32) {
	var items []aclItem

	for addr != 0 {
		head, err := readBytes(env, addr, itemEntrySize)
		if err != nil {
			// The list's last longword, 0, may end the readable memory.
			if first, err := env.Memory().LoadLongword(env.CPU(), addr); err == nil && first == 0 {
				break
			}

			return nil, ssAccVio
		}

		le := binary.LittleEndian
		if le.Uint32(head) == 0 {
			break
		}

		it := aclItem{length: le.Uint16(head), code: le.Uint16(head[2:]), buffer: le.Uint32(head[4:])}

		if !slices.Contains(knownItems, it.code) {
			return nil, ssBadParam
		}

		if it.buffer != 0 && it.code != aclRead && it.code != aclReadACE && it.code != aclLength &&
			it.code != aclGrantACE && it.code != aclNextACE {
			if it.data, err = readBytes(env, it.buffer, int(it.length)); err != nil {
				return nil, ssAccVio
			}
		}

		switch it.code {
		case aclAdd, aclMod, aclFind, aclDel:
			if it.code == aclDel && it.buffer == 0 {
				break
			}

			aces, ok := wholeACEs(it.data)
			if !ok {
				return nil, ssIvACL
			}

			it.aces = aces
		}

		items = append(items, it)
		addr += itemEntrySize
	}

	return items, ssNormal
}

// wholeACEs splits b into ACEs, and reports whether b is whole ACEs, at
// least one: VMS 7.1 refused a buffer longer or shorter than its ACEs, and
// an ACE of size 3 (an ACE of fewer bytes than its header is refused, an
// unconfirmed bound).
func wholeACEs(b []byte) ([][]byte, bool) {
	var aces [][]byte

	for off := 0; off < len(b); {
		n := int(b[off])
		if n < aceHeaderLen || off+n > len(b) {
			return nil, false
		}

		aces = append(aces, slices.Clone(b[off:off+n]))
		off += n
	}

	return aces, len(aces) > 0
}

// splitACL is an ACL's entries.
func splitACL(raw []byte) [][]byte {
	var acl [][]byte

	for off := 0; off < len(raw); {
		n := int(raw[off])
		if n == 0 || off+n > len(raw) {
			break
		}

		acl = append(acl, slices.Clone(raw[off:off+n]))
		off += n
	}

	return acl
}

// joinACL is the entries one after another.
func joinACL(acl [][]byte) []byte { return slices.Concat(acl...) }

// findTarget is the object the call names: a file by channel or name, or a
// device.
func findTarget(env *corevms.Environment, objtyp, chanNumber, objnam uint32) (aclTarget, uint32) {
	if objtyp == objDevice {
		// govax keeps no device ACLs: a device's is empty and stays so
		// (VMS 7.1 gave the probe's disk none; changing one is
		// unconfirmed).
		return deviceTarget{}, ssNormal
	}

	if chanNumber != 0 {
		f, status := env.ChannelFile(chanNumber)
		if status != 0 {
			return nil, status
		}

		return f.SecurityObject(env.Credentials()), ssNormal
	}

	name, err := librtl.ReadString(env, objnam)
	if err != nil {
		return nil, ssAccVio
	}

	if env.Session == nil {
		return nil, ssNoSuchObj
	}

	o, err := env.Session.FindSecurityObject(name)
	if err != nil {
		var fileErr *rms.DCLFileError

		switch {
		case errors.As(err, &fileErr):
			return nil, fileErr.Status
		case errors.Is(err, rms.ErrNoPrivilege):
			return nil, ssNoPriv
		}

		return nil, ssNoSuchObj
	}

	return o, ssNormal
}

// deviceTarget is a device's ACL: empty, and unchangeable in govax.
type deviceTarget struct{}

func (deviceTarget) ACL() []byte               { return nil }
func (deviceTarget) MayRead() bool             { return true }
func (deviceTarget) MayChange() bool           { return true }
func (deviceTarget) SetACL([]byte) error       { return errors.New("securertl: no device ACLs") }
func (deviceTarget) Profile() security.Profile { return security.Profile{} }

// aclEditor works the items on the ACL.
type aclEditor struct {
	env    *corevms.Environment
	target aclTarget
	acl    [][]byte

	pos     int // 1 the first ACE, 0 the top, past the last the bottom
	found   bool
	changed bool

	// zeroContext is set by an alarm or audit ACE's addition: VMS 7.1
	// left a context of 0 after each (A.4).
	zeroContext bool
}

// current is the position as the index of an ACE, 1 for the top.
func (e *aclEditor) current() int { return max(e.pos, 1) }

// insertAt is where an ACE added at the position goes, 1 to len+1.
func (e *aclEditor) insertAt() int { return min(e.current(), len(e.acl)+1) }

// do works one item, returning its status.
func (e *aclEditor) do(it aclItem) uint32 {
	switch it.code {
	case aclAdd:
		for _, ace := range it.aces {
			e.add(ace)
		}

	case aclDel:
		return e.delete(it)

	case aclMod:
		cur := e.current()
		if cur > len(e.acl) {
			return ssNoEntry
		}

		e.acl[cur-1] = it.aces[0]
		e.pos, e.changed = cur+1, true

	case aclFind:
		return e.find(e.current(), func(ace []byte) bool { return security.MatchACE(ace, it.aces[0]) })

	case aclFindType:
		typ := byte(0)
		if len(it.data) > 1 {
			typ = it.data[1]
		}

		// The search starts after the position: VMS 7.1's FNDACETYP of
		// identifier ACEs from a context of 0 found the second ACE, both
		// being identifier ACEs.
		return e.find(e.current()+1, func(ace []byte) bool { return ace[1] == typ })

	case aclDeleteACL:
		e.acl = slices.DeleteFunc(e.acl, func(ace []byte) bool {
			return binary.LittleEndian.Uint16(ace[2:])&security.OptionProtected == 0
		})
		e.pos, e.found, e.changed = 0, false, true

	case aclDeleteAll:
		e.acl, e.pos, e.found, e.changed = nil, 0, false, true

	case aclRead:
		return e.read(it)

	case aclReadACE:
		cur := e.current()
		if cur > len(e.acl) {
			return ssNoMoreACE
		}

		return e.copyOut(it, e.acl[cur-1])

	case aclLength:
		var b [4]byte

		binary.LittleEndian.PutUint32(b[:], uint32(len(joinACL(e.acl))))

		if writeBytes(e.env, it.buffer, b[:min(int(it.length), 4)]) != nil {
			return ssAccVio
		}

	case aclGrantACE:
		d := security.Evaluate(e.env.Credentials(), e.target.Profile())
		if d.ACE < 0 || d.ACE >= len(e.acl) {
			return ssNoEntry
		}

		return e.copyOut(it, e.acl[d.ACE])

	case aclNextACE:
		e.pos++

	case aclRLock, aclWLock, aclUnlock:
		// ACL locks interlock other processes' changes; govax changes an
		// ACL in one step, so they have nothing to do.
	}

	return ssNormal
}

// add adds ace at the position, by $CHANGE_ACL's matching
// (security.MatchACE): a matching ACE above the position makes the
// addition do nothing, one at or below it is replaced. An alarm or audit
// ACE replaces a match wherever it is (VMS 7.1 dropped
// (ALARM...READ+SUCCESS) above the bottom for (ALARM...WRITE+SUCCESS)
// added there; unconfirmed elsewhere than the bottom).
func (e *aclEditor) add(ace []byte) {
	alarm := security.AlarmLike(ace)
	at := e.insertAt()

	if !alarm {
		for i, have := range e.acl {
			if i+1 < at && security.MatchACE(have, ace) {
				return
			}
		}
	}

	for i := len(e.acl) - 1; i >= 0; i-- {
		if (alarm || i+1 >= at) && security.MatchACE(e.acl[i], ace) {
			e.acl = slices.Delete(e.acl, i, i+1)
		}
	}

	at = min(e.insertAt(), at)
	e.acl = slices.Insert(e.acl, at-1, ace)
	e.pos, e.changed = at+1, true
	e.zeroContext = e.zeroContext || alarm
}

// delete is DELACLENT: the first ACE matching the item's (from the top),
// or with no buffer the ACE at the position; SS$_NOENTRY if there is none.
func (e *aclEditor) delete(it aclItem) uint32 {
	at := -1

	if it.aces == nil {
		if cur := e.current(); cur <= len(e.acl) {
			at = cur - 1
		}
	} else {
		at = slices.IndexFunc(e.acl, func(ace []byte) bool { return security.MatchACE(ace, it.aces[0]) })
	}

	if at < 0 {
		return ssNoEntry
	}

	e.acl = slices.Delete(e.acl, at, at+1)
	e.changed = true

	if at+1 < e.pos {
		e.pos--
	}

	return ssNormal
}

// find moves the position to the first ACE from start that matches,
// setting the found bit; SS$_NOENTRY, the context then 0, if none does.
func (e *aclEditor) find(start int, match func([]byte) bool) uint32 {
	for i := start - 1; i < len(e.acl); i++ {
		if i >= 0 && match(e.acl[i]) {
			e.pos, e.found = i+1, true

			return ssNormal
		}
	}

	e.pos, e.found = 0, false

	return ssNoEntry
}

// read is READACL: the whole ACEs from the position that fit in the
// buffer, the position moved past them. Past the last ACE is
// SS$_NOMOREACE; a buffer too small for the next is SS$_IVBUFLEN, with
// what fitted of it written (VMS 7.1's wrote 8 of 12 bytes).
func (e *aclEditor) read(it aclItem) uint32 {
	cur := e.current()
	if cur > len(e.acl) {
		return ssNoMoreACE
	}

	var out []byte

	next := cur
	for ; next <= len(e.acl) && len(out)+len(e.acl[next-1]) <= int(it.length); next++ {
		out = append(out, e.acl[next-1]...)
	}

	if next == cur {
		return e.copyOut(it, e.acl[cur-1])
	}

	if writeBytes(e.env, it.buffer, out) != nil {
		return ssAccVio
	}

	e.pos = next

	return ssNormal
}

// copyOut writes ace in the item's buffer: SS$_IVBUFLEN, with what fitted
// written, if it doesn't fit.
func (e *aclEditor) copyOut(it aclItem, ace []byte) uint32 {
	n := min(len(ace), int(it.length))

	if writeBytes(e.env, it.buffer, ace[:n]) != nil {
		return ssAccVio
	}

	if n < len(ace) {
		return ssIvBufLen
	}

	return ssNormal
}
