package corevms

import (
	"encoding/binary"
	"errors"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// $GET_SECURITY and $SET_SECURITY: an object's security profile (owner,
// protection, ACL) read and changed through an item list, for objects of
// several classes, files and devices among them (docs/planning/PHASE-93 -
// file protection.md, subtask 13b; A.4's "Round 3"; VSI's system services
// pages).
//
// Both take (clsnam, objnam, objhan, flags, itmlst, contxt, acmode). The
// object is a class (clsnam, any abbreviation of one) and a name (objnam)
// or, for a file, a channel it is accessed on (the longword objhan points
// at; a name beside a handle is ignored). A context (contxt) carries the
// object and an ACL position from call to call; OSS$M_RELCTX releases it.
// $SET_SECURITY's changes under OSS$M_LOCAL wait in the context until the
// call that releases it.

// The item codes ($OSSDEF).
var (
	ossAccessNames       = uint16(vmsdef.Symbols["OSS$_ACCESS_NAMES"])
	ossAccessNamesLength = uint16(vmsdef.Symbols["OSS$_ACCESS_NAMES_LENGTH"])
	ossAddEntry          = uint16(vmsdef.Symbols["OSS$_ACL_ADD_ENTRY"])
	ossDeleteEntry       = uint16(vmsdef.Symbols["OSS$_ACL_DELETE_ENTRY"])
	ossDelete            = uint16(vmsdef.Symbols["OSS$_ACL_DELETE"])
	ossDeleteAll         = uint16(vmsdef.Symbols["OSS$_ACL_DELETE_ALL"])
	ossFindEntry         = uint16(vmsdef.Symbols["OSS$_ACL_FIND_ENTRY"])
	ossFindNext          = uint16(vmsdef.Symbols["OSS$_ACL_FIND_NEXT"])
	ossFindType          = uint16(vmsdef.Symbols["OSS$_ACL_FIND_TYPE"])
	ossGrantACE          = uint16(vmsdef.Symbols["OSS$_ACL_GRANT_ACE"])
	ossACLLength         = uint16(vmsdef.Symbols["OSS$_ACL_LENGTH"])
	ossModifyEntry       = uint16(vmsdef.Symbols["OSS$_ACL_MODIFY_ENTRY"])
	ossPosition          = uint16(vmsdef.Symbols["OSS$_ACL_POSITION"])
	ossPositionTop       = uint16(vmsdef.Symbols["OSS$_ACL_POSITION_TOP"])
	ossPositionBottom    = uint16(vmsdef.Symbols["OSS$_ACL_POSITION_BOTTOM"])
	ossReadEntry         = uint16(vmsdef.Symbols["OSS$_ACL_READ_ENTRY"])
	ossRead              = uint16(vmsdef.Symbols["OSS$_ACL_READ"])
	ossNextObject        = uint16(vmsdef.Symbols["OSS$_NEXT_OBJECT"])
	ossOwner             = uint16(vmsdef.Symbols["OSS$_OWNER"])
	ossProtection        = uint16(vmsdef.Symbols["OSS$_PROTECTION"])
	ossSysProt           = uint16(vmsdef.Symbols["OSS$_SYS_PROT"])
	ossOwnProt           = uint16(vmsdef.Symbols["OSS$_OWN_PROT"])
	ossGrpProt           = uint16(vmsdef.Symbols["OSS$_GRP_PROT"])
	ossWorProt           = uint16(vmsdef.Symbols["OSS$_WOR_PROT"])
	ossClassName         = uint16(vmsdef.Symbols["OSS$_CLASS_NAME"])
	ossFirstTemplate     = uint16(vmsdef.Symbols["OSS$_FIRST_TEMPLATE"])
	ossNextTemplate      = uint16(vmsdef.Symbols["OSS$_NEXT_TEMPLATE"])
	ossObjectName        = uint16(vmsdef.Symbols["OSS$_OBJECT_NAME"])
	ossAccessClassName   = uint16(vmsdef.Symbols["OSS$_ACCESS_CLASS_NAME"])
	ossDamagedACL        = uint16(vmsdef.Symbols["OSS$_DAMAGED_ACL"])
	ossInTransition      = uint16(vmsdef.Symbols["OSS$_IN_TRANSITION"])
	ossTranquility       = uint16(vmsdef.Symbols["OSS$_TRANQUILITY_COUNT"])

	ossRelCtx = uint32(vmsdef.Symbols["OSS$M_RELCTX"])
	ossLocal  = uint32(vmsdef.Symbols["OSS$M_LOCAL"])
)

// Statuses only these services return.
var (
	ssNoClass     = vmsdef.Symbols["SS$_NOCLASS"]
	ssInvClsItm   = vmsdef.Symbols["SS$_INVCLSITM"]
	ssACLEmpty    = vmsdef.Symbols["SS$_ACLEMPTY"]
	ssNoMoreACE   = vmsdef.Symbols["SS$_NOMOREACE"]
	ssNoEntry     = vmsdef.Symbols["SS$_NOENTRY"]
	ssNoSuchObj   = vmsdef.Symbols["SS$_NOSUCHOBJECT"]
	ssNoPrivilege = vmsdef.Symbols["SS$_NOPRIV"]
)

// changingItems are $SET_SECURITY's items that change the profile, which
// need control access.
var changingItems = map[uint16]bool{
	ossOwner: true, ossProtection: true, ossAddEntry: true, ossDeleteEntry: true,
	ossDelete: true, ossDeleteAll: true, ossModifyEntry: true,
}

// positionItems move the ACL position; both services take them.
var positionItems = map[uint16]bool{
	ossFindEntry: true, ossFindNext: true, ossFindType: true, ossPosition: true,
	ossPositionTop: true, ossPositionBottom: true,
}

// getItems are $GET_SECURITY's other items.
var getItems = map[uint16]bool{
	ossAccessNames: true, ossAccessNamesLength: true, ossGrantACE: true, ossACLLength: true,
	ossReadEntry: true, ossRead: true, ossNextObject: true, ossOwner: true, ossProtection: true,
	ossSysProt: true, ossOwnProt: true, ossGrpProt: true, ossWorProt: true, ossClassName: true,
	ossFirstTemplate: true, ossNextTemplate: true, ossObjectName: true, ossAccessClassName: true,
	ossDamagedACL: true, ossInTransition: true, ossTranquility: true,
}

// categoryItems are the items for objects whose categories have more than
// four access bits (queues and the like), which none of govax's classes
// has: SS$_INVCLSITM (VMS 7.1's for a file).
var categoryItems = map[uint16]bool{ossSysProt: true, ossOwnProt: true, ossGrpProt: true, ossWorProt: true}

// securityObject is an object whose profile the services read or change:
// a file (rms.SecurityObject), a device, a volume, a logical name table,
// or a security class.
type securityObject interface {
	Profile() security.Profile
	ACL() []byte
	MayRead() bool
	MayChange() bool
	SetACL([]byte) error
	SetOwnerProtection(security.UIC, security.Protection) error
}

// securityClass is a class of object the services know.
type securityClass struct {
	name string

	// access are the names of the access bits, ACCESS_NAMES's (VMS 7.1
	// gave a file's; the others' are the Guide to System Security's).
	access []string

	// find is the object called name.
	find func(env *Environment, name string) (securityObject, string, uint32)
}

// securityClasses are the classes, in the order SECURITY_CLASS's
// NEXT_OBJECT walks them (VMS 7.1 gave FILE's next as VOLUME; the rest of
// the order is unconfirmed).
var securityClasses []*securityClass

func init() {
	securityClasses = []*securityClass{
		{name: "DEVICE", access: []string{"READ", "WRITE", "LOGICAL", "PHYSICAL", "CONTROL"}, find: findDeviceObject},
		{name: "FILE", access: []string{"READ", "WRITE", "EXECUTE", "DELETE", "CONTROL"}, find: findFileObject},
		{name: "VOLUME", access: []string{"READ", "WRITE", "CREATE", "DELETE", "CONTROL"}, find: findVolumeObject},
		{name: "LOGICAL_NAME_TABLE", access: []string{"READ", "WRITE", "CREATE", "DELETE", "CONTROL"}, find: findTableObject},
		{name: "QUEUE", access: security.QueueAccessNames, find: findQueueObject},
		{name: "SECURITY_CLASS", access: []string{"READ", "WRITE", "", "", "CONTROL"}, find: findClassObject},
	}
}

// classNamed is the class name abbreviates, in any case.
func classNamed(name string) *securityClass {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" {
		return nil
	}

	for _, c := range securityClasses {
		if strings.HasPrefix(c.name, name) {
			return c
		}
	}

	return nil
}

// securityContext is what a context carries between calls: the object,
// the ACL being read or changed with its position, and the changes
// OSS$M_LOCAL holds back.
type securityContext struct {
	class  *securityClass
	object securityObject
	name   string
	edit   *security.ACLEdit

	// lost is set by a find that found nothing: a read then has nothing
	// to read, and a FIND_TYPE starts at the second ACE (unconfirmed;
	// the reading of VMS 7.1's cases 385 to 388).
	lost bool

	owner      security.UIC
	protection security.Protection
	profileSet bool
}

// contextBase is the first context's value: the address VMS 7.1 gave
// the probe's contexts, in P1. govax's contexts are handles to Go state
// (Environment.securityContexts), the n-th 16 bytes on from it; nothing is
// stored there.
const contextBase = 0x7FFBEF10

// newSecurityContext keeps ctx for env's process, returning its handle.
func (env *Environment) newSecurityContext(ctx *securityContext) uint32 {
	if env.securityContexts == nil {
		env.securityContexts = map[uint32]*securityContext{}
	}

	h := uint32(contextBase)
	for env.securityContexts[h] != nil {
		h += 16
	}

	env.securityContexts[h] = ctx

	return h
}

// securityCall is one call of either service.
type securityCall struct {
	env    *Environment
	setter bool
	flags  uint32
	ctx    *securityContext
	handle uint32 // the context's handle, 0 for none
	ctxPtr uint32 // where the context longword is, 0 for none
}

// openSecurityCall reads the arguments common to both services and finds
// the object: from the context if there is one, else by class and name
// or handle.
func (env *Environment) openSecurityCall(argv []uint32, setter bool) (*securityCall, uint32) {
	arg := func(i int) uint32 {
		if i < len(argv) {
			return argv[i]
		}

		return 0
	}

	c := &securityCall{env: env, setter: setter, flags: arg(3), ctxPtr: arg(5)}

	if c.ctxPtr != 0 {
		v, err := env.mem.LoadLongword(env.cpu, c.ctxPtr)
		if err != nil {
			return nil, ssAccVio
		}

		if ctx := env.securityContexts[v]; v != 0 && ctx != nil {
			c.ctx, c.handle = ctx, v

			return c, ssNormal
		}
	}

	if arg(0) == 0 {
		return nil, ssNoClass
	}

	clsnam, ok, err := strGet(env, arg(0), 255)
	if err != nil || !ok {
		return nil, ssAccVio
	}

	class := classNamed(clsnam)
	if class == nil {
		return nil, ssNoClass
	}

	ctx := &securityContext{class: class}

	switch {
	case arg(2) != 0:
		h, err := env.mem.LoadLongword(env.cpu, arg(2))
		if err != nil {
			return nil, ssAccVio
		}

		// A handle of 0 is SS$_BADPARAM (VMS 7.1's, when the probe's
		// channel failed to open); only a file has handles here.
		if h == 0 || class.name != "FILE" {
			return nil, ssBadParam
		}

		f, status := env.ChannelFile(h)
		if status != 0 {
			return nil, status
		}

		ctx.object = f.SecurityObject(env.Credentials())

	case arg(1) == 0:
		return nil, ssNoSuchObj

	default:
		name, ok, err := strGet(env, arg(1), 4096)
		if err != nil || !ok {
			return nil, ssAccVio
		}

		obj, objName, status := class.find(env, name)
		if status != ssNormal {
			return nil, status
		}

		ctx.object, ctx.name = obj, objName
	}

	p := ctx.object.Profile()
	ctx.owner, ctx.protection = p.Owner, p.Protection
	c.ctx = ctx

	return c, ssNormal
}

// edit is the context's ACL edit, made from the object's ACL at its
// first use.
func (ctx *securityContext) acl() *security.ACLEdit {
	if ctx.edit == nil {
		ctx.edit = security.NewACLEdit(ctx.object.ACL())
	}

	return ctx.edit
}

// close ends the call: the context kept (its handle stored) or released,
// with the changes it holds applied, as the flags say.
func (c *securityCall) close(status uint32) uint32 {
	release := c.flags&ossRelCtx != 0
	local := c.flags&ossLocal != 0 && c.ctxPtr != 0 && !release

	if c.setter && !local {
		if st := c.apply(); st != ssNormal && status&1 != 0 {
			status = st
		}
	}

	if c.ctxPtr == 0 {
		return status
	}

	v := c.handle

	switch {
	case release:
		if c.handle != 0 {
			delete(c.env.securityContexts, c.handle)
		}

		v = 0
	case v == 0:
		v = c.env.newSecurityContext(c.ctx)
	}

	if c.env.mem.StoreLongword(c.env.cpu, c.ctxPtr, v) != nil {
		return ssAccVio
	}

	return status
}

// apply writes the context's changes to the object.
func (c *securityCall) apply() uint32 {
	ctx := c.ctx

	if ctx.profileSet {
		ctx.profileSet = false

		if err := ctx.object.SetOwnerProtection(ctx.owner, ctx.protection); err != nil {
			return securityError(err)
		}
	}

	if ctx.edit != nil && ctx.edit.Changed {
		ctx.edit.Changed = false

		if err := ctx.object.SetACL(ctx.edit.Bytes()); err != nil {
			return securityError(err)
		}
	}

	return ssNormal
}

// securityError is the status for an error changing an object.
func securityError(err error) uint32 {
	if errors.Is(err, rms.ErrNoPrivilege) {
		return ssNoPrivilege
	}

	if st := QueueErrorStatus(err); st != 0 {
		return st
	}

	return ssACLFull
}

// secItem is one entry of the item list, with its buffer's bytes for an
// item that gives the service something.
type secItem struct {
	itemListEntry
	data []byte
	aces [][]byte
}

// readSecurityItems reads the item list, checking each item before any is
// worked: a code the service doesn't take is SS$_ACCVIO (VMS 7.1's answer
// to item 99, whose buffer was 0; unconfirmed for other buffers), an ACE
// buffer that isn't whole ACEs SS$_IVACL, an OWNER or PROTECTION of the
// wrong size SS$_IVBUFLEN.
func (env *Environment) readSecurityItems(itmlst uint32, setter bool) ([]secItem, uint32) {
	var items []secItem

	if itmlst == 0 {
		return nil, ssNormal
	}

	status := env.walkItemList(itmlst, func(e itemListEntry) uint32 {
		code := e.ItemCode

		known := positionItems[code] || (setter && changingItems[code]) || (!setter && getItems[code]) ||
			(setter && categoryItems[code])
		if !known {
			return ssAccVio
		}

		it := secItem{itemListEntry: e}

		switch {
		case setter && code == ossOwner && e.BuffLen != 4,
			setter && code == ossProtection && e.BuffLen != 2:
			return ssIvBufLen
		}

		if setter || code == ossFindEntry || code == ossFindType || code == ossPosition {
			if e.BuffAddr != 0 && e.BuffLen != 0 {
				data, status := env.itemBytes(e)
				if status != 0 {
					return status
				}

				it.data = data
			}
		}

		switch code {
		case ossAddEntry, ossModifyEntry, ossFindEntry, ossDeleteEntry:
			if code == ossDeleteEntry && it.data == nil {
				break
			}

			aces, ok := wholeACEs(it.data)
			if !ok {
				return ssIvACL
			}

			it.aces = aces
		}

		items = append(items, it)

		return 0
	})

	if status != 0 {
		return nil, status
	}

	return items, ssNormal
}

// wholeACEs splits b into ACEs, and reports whether b is whole ACEs, at
// least one, each at least an ACE's header long.
func wholeACEs(b []byte) ([][]byte, bool) {
	var aces [][]byte

	for off := 0; off < len(b); {
		n := int(b[off])
		if n < 4 || off+n > len(b) {
			return nil, false
		}

		aces = append(aces, b[off:off+n])
		off += n
	}

	return aces, len(aces) > 0
}

// position works an item that moves the ACL position, and reports whether
// it was one. A find that finds nothing is notFound's status.
func (c *securityCall) position(it secItem, notFound uint32) (bool, uint32) {
	ctx := c.ctx
	e := ctx.acl()

	switch it.ItemCode {
	case ossPositionTop:
		e.Top()
		ctx.lost = false
	case ossPositionBottom:
		e.Bottom()
		ctx.lost = false
	case ossPosition:
		var v uint32
		if len(it.data) >= 4 {
			v = binary.LittleEndian.Uint32(it.data)
		}

		e.Pos, e.Found, ctx.lost = int(v), false, false
	case ossFindNext:
		// The next ACE (unconfirmed: VMS 7.1's case 381, a FIND_NEXT
		// after the first ACE's read, left nothing to read).
		e.Pos, e.Found = e.Current()+1, false
	case ossFindEntry:
		// From the top, whatever the position (VMS 7.1 found an ACE above
		// the position, cases 383 and 384).
		found := e.FindMatching(1, it.aces[0])
		ctx.lost = !found

		if !found {
			return true, notFound
		}
	case ossFindType:
		// A byte, or an ACE's header with the type in its second.
		typ := byte(0)

		switch len(it.data) {
		case 0:
		case 1:
			typ = it.data[0]
		default:
			typ = it.data[1]
		}

		start := e.Current()
		if ctx.lost {
			start = 2
		}

		found := e.FindType(start, typ)
		ctx.lost = !found

		if !found {
			return true, notFound
		}
	default:
		return false, ssNormal
	}

	return true, ssNormal
}

// serviceSysGetSecurity is SYS$GET_SECURITY [clsnam] ,[objnam] ,[objhan]
// ,[flags] ,[itmlst] ,[contxt] ,[acmode]: the items of an object's
// profile, as VMS 7.1 answered them (A.4): an OWNER longword, a
// PROTECTION word, ACL_LENGTH, ACL_READ (the whole ACL, SS$_IVBUFLEN if it
// doesn't fit; through a context, the whole ACEs that fit, then
// SS$_NOMOREACE), ACL_READ_ENTRY, the names, and the rest; only the text
// items, ACCESS_NAMES, DAMAGED_ACL, and IN_TRANSITION write a return
// length. The profile needs read or control access (SS$_NOPRIV). A file
// with no ACL asked for ACL_READ is SS$_ACLEMPTY, the other items filled
// in. acmode is taken and not used.
func serviceSysGetSecurity(env *Environment, argv []uint32) (uint32, error) {
	c, status := env.openSecurityCall(argv, false)
	if status != ssNormal {
		return status, nil
	}

	items, status := env.readSecurityItems(arg(argv, 4), false)
	if status != ssNormal {
		return status, nil
	}

	if len(items) > 0 && !c.ctx.object.MayRead() {
		return ssNoPrivilege, nil
	}

	status = ssNormal

	for _, it := range items {
		st := c.getItem(it)
		if st == ssACLEmpty {
			status = st

			continue
		}

		if st != ssNormal {
			status = st

			break
		}
	}

	return c.close(status), nil
}

// arg is argument i of a call, 0 if the call had fewer.
func arg(argv []uint32, i int) uint32 {
	if i < len(argv) {
		return argv[i]
	}

	return 0
}

// getItem answers one of $GET_SECURITY's items.
func (c *securityCall) getItem(it secItem) uint32 {
	if moved, st := c.position(it, ssNoMoreACE); moved {
		return st
	}

	ctx, env := c.ctx, c.env
	class := ctx.class
	p := ctx.object.Profile()

	switch it.ItemCode {
	case ossOwner:
		return c.put(it, longBytes(uint32(p.Owner)), false)

	case ossProtection:
		return c.put(it, wordBytes(uint16(p.Protection)), false)

	case ossSysProt, ossOwnProt, ossGrpProt, ossWorProt, ossTranquility:
		return ssInvClsItm

	case ossNextObject, ossFirstTemplate, ossNextTemplate:
		if class.name != "SECURITY_CLASS" {
			return ssInvClsItm
		}

		return c.putText(it, classText(it.ItemCode, ctx.name))

	case ossACLLength:
		return c.put(it, longBytes(uint32(len(ctx.object.ACL()))), false)

	case ossRead:
		return c.readACL(it)

	case ossReadEntry:
		e := ctx.acl()

		cur := e.Current()
		if ctx.lost || cur > len(e.ACL) {
			return ssNoMoreACE
		}

		ace := e.ACL[cur-1]
		if st := c.put(it, ace, false); st != ssNormal {
			return st
		}

		if len(ace) > int(it.BuffLen) {
			return ssIvBufLen
		}

		e.Pos, e.Found = cur+1, false

	case ossGrantACE:
		raw := ctx.object.ACL()

		d := security.Evaluate(env.Credentials(), p)
		acl := security.SplitACL(raw)

		if d.ACE < 0 || d.ACE >= len(acl) {
			return ssNoEntry
		}

		return c.put(it, acl[d.ACE], false)

	case ossClassName, ossAccessClassName:
		return c.putText(it, class.name)

	case ossObjectName:
		return c.putText(it, ctx.name)

	case ossAccessNamesLength:
		return c.put(it, longBytes(uint32(len(accessNames(class, 0)))), false)

	case ossAccessNames:
		return c.put(it, accessNames(class, it.BuffAddr), true)

	case ossDamagedACL, ossInTransition:
		return c.put(it, []byte{0}, true)
	}

	return ssNormal
}

// readACL is ACL_READ: with no context the whole ACL (SS$_IVBUFLEN, with
// what fitted written, if it doesn't fit; SS$_ACLEMPTY if there is none),
// through a context the whole ACEs from the position that fit.
func (c *securityCall) readACL(it secItem) uint32 {
	if c.ctxPtr == 0 {
		raw := c.ctx.object.ACL()
		if len(raw) == 0 {
			// SS$_ACLEMPTY is a file's answer; a device with no ACL was
			// SS$_NORMAL (VMS 7.1's cases 370 and 411).
			if c.ctx.class.name == "FILE" {
				return ssACLEmpty
			}

			return ssNormal
		}

		if st := c.put(it, raw, false); st != ssNormal {
			return st
		}

		if len(raw) > int(it.BuffLen) {
			return ssIvBufLen
		}

		return ssNormal
	}

	out, none, short := c.ctx.acl().ReadFitting(int(it.BuffLen))

	switch {
	case none:
		return ssNoMoreACE
	case short:
		_ = c.put(it, out, false)

		return ssIvBufLen
	}

	return c.put(it, out, false)
}

// put writes b in the item's buffer, as much as fits, and with retlen its
// return length.
func (c *securityCall) put(it secItem, b []byte, retlen bool) uint32 {
	n := min(len(b), int(it.BuffLen))

	for i := 0; i < n; i++ {
		if c.env.mem.StoreByte(c.env.cpu, it.BuffAddr+uint32(i), b[i]) != nil {
			return ssAccVio
		}
	}

	if retlen && it.RetAddr != 0 {
		if c.env.mem.StoreWord(c.env.cpu, it.RetAddr, uint16(n)) != nil {
			return ssAccVio
		}
	}

	return ssNormal
}

// putText writes a text item with its return length.
func (c *securityCall) putText(it secItem, s string) uint32 { return c.put(it, []byte(s), true) }


// accessNames is ACCESS_NAMES's buffer for a buffer at addr: 32 quadword
// descriptors, one per access bit, then the names they describe. VMS 7.1's
// descriptors pointed at names in system space; govax's point at the
// copies after them in the buffer (unconfirmed where they should point).
func accessNames(class *securityClass, addr uint32) []byte {
	const table = 32 * 8

	b := make([]byte, table)
	off := table

	var text []byte

	for i, name := range class.access {
		if name == "" {
			continue
		}

		binary.LittleEndian.PutUint16(b[i*8:], uint16(len(name)))
		binary.LittleEndian.PutUint32(b[i*8+4:], addr+uint32(off+len(text)))
		text = append(text, name...)
	}

	return append(b, text...)
}

// classText is a SECURITY_CLASS object's NEXT_OBJECT (the class after it)
// or its templates' names (DEFAULT, the only template govax has).
func classText(code uint16, name string) string {
	if code != ossNextObject {
		if code == ossFirstTemplate {
			return "DEFAULT"
		}

		return ""
	}

	for i, c := range securityClasses {
		if c.name == name && i+1 < len(securityClasses) {
			return securityClasses[i+1].name
		}
	}

	return ""
}

// serviceSysSetSecurity is SYS$SET_SECURITY [clsnam] ,[objnam] ,[objhan]
// ,[flags] ,[itmlst] ,[contxt] ,[acmode]: an object's owner, protection,
// and ACL changed by an item list, as VMS 7.1 changed them (A.4): OWNER a
// longword and PROTECTION a word (any other size SS$_IVBUFLEN), the ACL
// items by $CHANGE_ACL's matching, at a position the position items set.
// A change needs control access, and a new owner one the process may give
// (SS$_NOPRIV). Each call's changes are made at its end, or with
// OSS$M_LOCAL and a context at the call that releases the context.
func serviceSysSetSecurity(env *Environment, argv []uint32) (uint32, error) {
	c, status := env.openSecurityCall(argv, true)
	if status != ssNormal {
		return status, nil
	}

	items, status := env.readSecurityItems(arg(argv, 4), true)
	if status != ssNormal {
		return status, nil
	}

	for _, it := range items {
		if changingItems[it.ItemCode] && !c.ctx.object.MayChange() {
			return ssNoPrivilege, nil
		}
	}

	status = ssNormal

	for _, it := range items {
		if status = c.setItem(it); status != ssNormal {
			break
		}
	}

	return c.close(status), nil
}

// setItem works one of $SET_SECURITY's items.
func (c *securityCall) setItem(it secItem) uint32 {
	if moved, st := c.position(it, ssNoEntry); moved {
		return st
	}

	ctx := c.ctx

	switch it.ItemCode {
	case ossSysProt, ossOwnProt, ossGrpProt, ossWorProt:
		return ssInvClsItm

	case ossOwner:
		ctx.owner, ctx.profileSet = security.UIC(binary.LittleEndian.Uint32(it.data)), true

	case ossProtection:
		ctx.protection, ctx.profileSet = security.Protection(binary.LittleEndian.Uint16(it.data)), true

	case ossAddEntry:
		e := ctx.acl()
		for _, ace := range it.aces {
			e.Add(ace)
		}

	case ossDeleteEntry:
		var want []byte
		if it.aces != nil {
			want = it.aces[0]
		}

		if !ctx.acl().Delete(want) {
			return ssNoEntry
		}

	case ossModifyEntry:
		if !ctx.acl().Modify(it.aces[0]) {
			return ssNoEntry
		}

	case ossDelete:
		ctx.acl().DeleteUnprotected()

	case ossDeleteAll:
		ctx.acl().DeleteAll()
	}

	return ssNormal
}

// registerSecurityServices installs $GET_SECURITY and $SET_SECURITY.
func registerSecurityServices(t *ServiceTable) {
	t.Register("SYS$GET_SECURITY", serviceSysGetSecurity)
	t.Register("SYS$SET_SECURITY", serviceSysSetSecurity)
}

// findFileObject is the file name names, by $CHANGE_ACL's rules
// (rms.Session.FindSecurityObject). A file's OBJECT_NAME is empty, as VMS
// 7.1's was.
func findFileObject(env *Environment, name string) (securityObject, string, uint32) {
	if env.Session == nil {
		return nil, "", ssNoSuchObj
	}

	o, err := env.Session.FindSecurityObject(name)
	if err != nil {
		var fileErr *rms.DCLFileError

		switch {
		case errors.As(err, &fileErr):
			return nil, "", fileErr.Status
		case errors.Is(err, rms.ErrNoPrivilege):
			return nil, "", ssNoPrivilege
		}

		return nil, "", ssNoSuchObj
	}

	return o, "", ssNormal
}
