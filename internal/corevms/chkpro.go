package corevms

import (
	"bytes"
	"encoding/binary"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// $CHKPRO, the system's access check given entirely by an item list, and
// the item handling $CHECK_ACCESS (internal/securertl) shares with it
// (docs/planning/PHASE-93 - file protection.md, subtask 13; A.4's "Round
// 3"; VSI's system services pages).
//
// A caller describes an object (its owner, its protection code, its ACL)
// and an accessor (a UIC, rights, privileges, access mode), asks for some
// access, and gets SS$_NORMAL or SS$_NOPRIV, with, if it asks, the ACE
// that decided, the privileges the access needed, and the names of the
// alarm and audit journals the ACL names. What the item list leaves out of
// the accessor is the calling process's own (its UIC, rights, and current
// privileges); what it leaves out of the object is zero: owner [0,0],
// protection 0 (everyone everything), no ACL. $CHKPRO compares the access
// bits with the protection code's rather than interpreting them
// (security.Compared).

// The item codes ($CHPDEF).
var (
	chpAccess     = uint16(vmsdef.Symbols["CHP$_ACCESS"])
	chpFlags      = uint16(vmsdef.Symbols["CHP$_FLAGS"])
	chpPriv       = uint16(vmsdef.Symbols["CHP$_PRIV"])
	chpACMode     = uint16(vmsdef.Symbols["CHP$_ACMODE"])
	chpRights     = uint16(vmsdef.Symbols["CHP$_RIGHTS"])
	chpAddRights  = uint16(vmsdef.Symbols["CHP$_ADDRIGHTS"])
	chpMode       = uint16(vmsdef.Symbols["CHP$_MODE"])
	chpModes      = uint16(vmsdef.Symbols["CHP$_MODES"])
	chpOwner      = uint16(vmsdef.Symbols["CHP$_OWNER"])
	chpProt       = uint16(vmsdef.Symbols["CHP$_PROT"])
	chpACL        = uint16(vmsdef.Symbols["CHP$_ACL"])
	chpAuditName  = uint16(vmsdef.Symbols["CHP$_AUDITNAME"])
	chpAlarmName  = uint16(vmsdef.Symbols["CHP$_ALARMNAME"])
	chpMatchedACE = uint16(vmsdef.Symbols["CHP$_MATCHEDACE"])
	chpPrivUsed   = uint16(vmsdef.Symbols["CHP$_PRIVUSED"])
	chpUIC        = uint16(vmsdef.Symbols["CHP$_UIC"])
	chpMaxCode    = uint16(vmsdef.Symbols["CHP$_MAX_CODE"])

	chpUseReadAll = uint32(vmsdef.Symbols["CHP$M_USEREADALL"])
)

// The limits on repeated items, the manual's.
const (
	maxACLSegments   = 20 // CHP$_ACL items
	maxRightsEntries = 11 // CHP$_ADDRIGHTS items
)

// Statuses only these services return.
var (
	ssACLFull    = vmsdef.Symbols["SS$_ACLFULL"]
	ssRightsFull = vmsdef.Symbols["SS$_RIGHTSFULL"]
	ssIvACL      = vmsdef.Symbols["SS$_IVACL"]
)

// privUsedBits are CHP$_PRIVUSED's bits for security.Used's.
var privUsedBits = []struct {
	used security.Used
	bit  uint32
}{
	{security.UsedSYSPRV, uint32(vmsdef.Symbols["CHP$M_SYSPRV"])},
	{security.UsedGRPPRV, uint32(vmsdef.Symbols["CHP$M_GRPPRV"])},
	{security.UsedBYPASS, uint32(vmsdef.Symbols["CHP$M_BYPASS"])},
	{security.UsedREADALL, uint32(vmsdef.Symbols["CHP$M_READALL"])},
}

// checkRequest is one check: who asks, for what, of what, and where the
// answers go.
type checkRequest struct {
	creds   security.Credentials
	profile security.Profile
	want    security.Access
	flags   uint32

	// acmode, mode, and modes are the access modes the check compares,
	// each -1 (or hasModes false) when not given.
	acmode, mode int
	modes        uint64
	hasModes     bool

	// outputs are the items that ask for an answer, in the list's order.
	outputs []itemListEntry

	segments, extraRights int
	rightsGiven           bool
}

// newCheckRequest is a request with env's process as the accessor and
// read as the access (each service changes what it must).
func (env *Environment) newCheckRequest() checkRequest {
	return checkRequest{creds: env.Process.Credentials(), acmode: -1, mode: -1}
}

// readCheckItems reads the item list at itmlst into r. With subject, the
// accessor's and the object's items are read ($CHKPRO's); without, they
// are passed over, the object and accessor being the caller's to name
// ($CHECK_ACCESS's). An item code CHP$ doesn't have is SS$_BADPARAM.
func (env *Environment) readCheckItems(itmlst uint32, r *checkRequest, subject bool) uint32 {
	if itmlst == 0 {
		return 0
	}

	return env.walkItemList(itmlst, func(e itemListEntry) uint32 {
		if e.ItemCode == 0 || e.ItemCode >= chpMaxCode {
			return ssBadParam
		}

		switch e.ItemCode {
		case chpMatchedACE, chpPrivUsed, chpAlarmName, chpAuditName:
			r.outputs = append(r.outputs, e)

			return 0
		case chpAccess:
			v, status := env.itemNumber(e, 4)
			r.want = security.Access(v)

			return status
		case chpFlags:
			v, status := env.itemNumber(e, 4)
			r.flags = uint32(v)

			return status
		}

		if !subject {
			return 0
		}

		return env.readSubjectItem(e, r)
	})
}

// readSubjectItem reads one of $CHKPRO's items about the accessor or the
// object.
func (env *Environment) readSubjectItem(e itemListEntry, r *checkRequest) uint32 {
	switch e.ItemCode {
	case chpPriv:
		v, status := env.itemNumber(e, 8)
		r.creds.Privileges = v

		return status
	case chpUIC:
		v, status := env.itemNumber(e, 4)
		r.creds.UIC = security.UIC(v)

		return status
	case chpOwner:
		v, status := env.itemNumber(e, 4)
		r.profile.Owner = security.UIC(v)

		return status
	case chpProt:
		// Only the first word: VMS 7.1 didn't look at a second
		// (security.Compared's comment).
		v, status := env.itemNumber(e, 2)
		r.profile.Protection = security.Protection(v)

		return status
	case chpACMode:
		v, status := env.itemNumber(e, 1)
		r.acmode = int(v & 3)

		return status
	case chpMode:
		v, status := env.itemNumber(e, 1)
		r.mode = int(v & 3)

		return status
	case chpModes:
		v, status := env.itemNumber(e, 8)
		r.modes, r.hasModes = v, true

		return status
	case chpRights, chpAddRights:
		return env.readRightsItem(e, r)
	case chpACL:
		return env.readACLItem(e, r)
	}

	// The audit's items (CHP$_AUDIT_LIST, CHP$_OBJECT_NAME, ...) and
	// classification's: govax has no auditing or mandatory classes.
	return 0
}

// readRightsItem reads CHP$_RIGHTS or CHP$_ADDRIGHTS: quadwords, each an
// identifier and its attributes, which the check ignores. CHP$_RIGHTS
// replaces the accessor's rights with its own, the first entry being the
// accessor's UIC; CHP$_ADDRIGHTS adds to the rights (the process's, if
// there was no CHP$_RIGHTS).
func (env *Environment) readRightsItem(e itemListEntry, r *checkRequest) uint32 {
	data, status := env.itemBytes(e)
	if status != 0 {
		return status
	}

	var ids []security.Identifier

	for off := 0; off+8 <= len(data); off += 8 {
		ids = append(ids, security.Identifier{
			Value:      binary.LittleEndian.Uint32(data[off:]),
			Attributes: binary.LittleEndian.Uint32(data[off+4:]),
		})
	}

	if e.ItemCode == chpAddRights {
		if r.extraRights++; r.extraRights > maxRightsEntries {
			return ssRightsFull
		}

		r.creds.Rights = append(r.creds.Rights, ids...)

		return 0
	}

	r.rightsGiven = true
	r.creds.Rights = nil

	if len(ids) > 0 {
		r.creds.UIC = security.UIC(ids[0].Value)
		r.creds.Rights = ids[1:]
	}

	return 0
}

// readACLItem reads a CHP$_ACL segment, which must be whole ACEs (VMS 7.1
// refused an ACE of size 3, the probe's case 572; govax, any ACE shorter
// than an ACE's header, unconfirmed).
func (env *Environment) readACLItem(e itemListEntry, r *checkRequest) uint32 {
	if r.segments++; r.segments > maxACLSegments {
		return ssACLFull
	}

	data, status := env.itemBytes(e)
	if status != 0 {
		return status
	}

	for off := 0; off < len(data); {
		n := int(data[off])
		if n < aceHeaderSize || off+n > len(data) {
			return ssIvACL
		}

		r.profile.ACL = append(r.profile.ACL, security.DecodeACE(data[off:off+n]))
		off += n
	}

	return 0
}

// aceHeaderSize is an ACE's header: its size, type, flags, and access.
var aceHeaderSize = int(vmsdef.Symbols["ACE$C_LENGTH"])

// itemBytes is an input item's buffer.
func (env *Environment) itemBytes(e itemListEntry) ([]byte, uint32) {
	data := make([]byte, e.BuffLen)

	for i := range data {
		b, err := env.mem.LoadByte(env.cpu, e.BuffAddr+uint32(i))
		if err != nil {
			return nil, ssAccVio
		}

		data[i] = b
	}

	return data, 0
}

// itemNumber is an input item's number of up to width bytes, low byte
// first; a buffer shorter than width gives what it holds.
func (env *Environment) itemNumber(e itemListEntry, width int) (uint64, uint32) {
	e.BuffLen = min(e.BuffLen, uint16(width))

	data, status := env.itemBytes(e)
	if status != 0 {
		return 0, status
	}

	var v uint64

	for i, b := range data {
		v |= uint64(b) << (8 * i)
	}

	return v, 0
}

// decide makes the check and writes the answers r's outputs ask for. It
// returns SS$_NORMAL or SS$_NOPRIV, or for a matched ACE too long for its
// buffer SS$_BUFFEROVF (granted) or SS$_IVBUFLEN (refused), as the manual
// has them.
func (env *Environment) decide(r *checkRequest, rules security.Rules) uint32 {
	out := security.ThirdParty(r.creds, r.profile, r.want, rules, r.flags&chpUseReadAll != 0)

	if !r.modeAllows() {
		out.Granted, out.Used = false, 0
	}

	status := uint32(ssNormal)
	if !out.Granted {
		status = ssNoPriv
	}

	for _, e := range r.outputs {
		var value []byte

		switch e.ItemCode {
		case chpMatchedACE:
			if out.ACE >= 0 {
				value = r.profile.ACL[out.ACE].Data
			}
		case chpPrivUsed:
			var bits uint32

			for _, b := range privUsedBits {
				if out.Used&b.used != 0 {
					bits |= b.bit
				}
			}

			value = binary.LittleEndian.AppendUint32(nil, bits)
		case chpAlarmName:
			value = journalName(r.profile.ACL, security.TypeAlarm, r.want, out.Granted)
		case chpAuditName:
			value = journalName(r.profile.ACL, security.TypeAudit, r.want, out.Granted)
		}

		n, short, err := storeBuffer(env, e.BuffAddr, e.BuffLen, string(value))
		if err != nil {
			return ssAccVio
		}

		if s := env.setRetLen(e, n); s != 0 {
			return s
		}

		if short && e.ItemCode == chpMatchedACE {
			status = ssBufferOvf
			if !out.Granted {
				status = ssIvBufLen
			}
		}
	}

	return status
}

// modeAllows applies the access modes: an accessor whose mode is less
// privileged (a larger number) than the object's (CHP$_MODE), or than the
// mode CHP$_MODES gives any access bit asked for, is refused. Without
// CHP$_ACMODE the modes aren't looked at (the manual).
func (r *checkRequest) modeAllows() bool {
	if r.acmode < 0 {
		return true
	}

	if r.mode >= 0 && r.acmode > r.mode {
		return false
	}

	if r.hasModes {
		for bit := range 32 {
			if r.want&(1<<bit) != 0 && r.acmode > int(r.modes>>(2*bit)&3) {
				return false
			}
		}
	}

	return true
}

// journalName is CHP$_ALARMNAME's or CHP$_AUDITNAME's answer: the journal
// name of the first ACE of type typ whose access includes any bit asked
// for and whose SUCCESS or FAILURE option fits the outcome, or nothing
// (VMS 7.1 gave "SECURITY", the ACE's name, the probe's case 269; the
// manual says only SECURITY is ever named).
func journalName(acl []security.ACE, typ byte, want security.Access, granted bool) []byte {
	outcome := security.AlarmFailure
	if granted {
		outcome = security.AlarmSuccess
	}

	for _, a := range acl {
		if a.Type != typ || a.Flags&outcome == 0 || a.Access&want == 0 || len(a.Data) <= aceHeaderSize {
			continue
		}

		name := a.Data[aceHeaderSize:]
		if i := bytes.IndexByte(name, 0); i >= 0 {
			name = name[:i]
		}

		return name
	}

	return nil
}

// serviceSysChkpro is SYS$CHKPRO:
//
//	SYS$CHKPRO itmlst ,[objpro] ,[usrpro]
//
// govax takes no encoded profiles (objpro is reserved to VMS, and govax
// has no $CREATE_USER_PROFILE): the item list says everything. With no
// item list, or an empty one, nothing is asked and the answer is
// SS$_NORMAL (the probe's case 569).
func serviceSysChkpro(env *Environment, argv []uint32) (uint32, error) {
	r := env.newCheckRequest()

	if status := env.readCheckItems(optArg(argv, 0), &r, true); status != 0 {
		return status, nil
	}

	return env.decide(&r, security.Compared), nil
}

// CheckAccessItems is $CHECK_ACCESS's check (internal/securertl): the item
// list at itmlst asks for an access (read if it doesn't say), and for the
// answers, which an account with credentials creds gets to an object with
// profile p, by a file's rules (security.Interpreted).
func (env *Environment) CheckAccessItems(itmlst uint32, creds security.Credentials, p security.Profile) uint32 {
	r := checkRequest{creds: creds, profile: p, want: security.Read, acmode: -1, mode: -1}

	if status := env.readCheckItems(itmlst, &r, false); status != 0 {
		return status
	}

	return env.decide(&r, security.Interpreted)
}

func registerCheckProtectionServices(t *ServiceTable) {
	t.Register("SYS$CHKPRO", serviceSysChkpro)
	registerSecurityServices(t)
}
