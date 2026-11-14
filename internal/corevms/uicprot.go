package corevms

import "github.com/tucats/govax/internal/security"

// UIC-based protection (docs/planning/PHASE-46.md, subtask 3): the check VMS
// makes before letting a process use an object that has an owner and a
// protection mask, such as a mailbox, a global section, or a common event
// flag cluster.
//
// The rule itself is internal/security's (Phase 93), the one rule every
// protected object uses, files included: its package comment explains the
// protection mask's four categories, the privileges BYPASS, SYSPRV,
// GRPPRV, and READALL, and objects owned by [0,0]. These objects have no
// access control lists in govax, so only the owner and the mask count.

// The access kinds, as bits within a category's field: $ARMDEF's. For a
// device (a mailbox), bit 2 is physical I/O and bit 3 logical I/O.
var (
	accessRead     = uint32(security.Read)
	accessWrite    = uint32(security.Write)
	accessPhysical = uint32(security.Physical)
	accessLogical  = uint32(security.Logical)
)

// The privileges protection checks look at.
var (
	privBYPASS  = privilegeBit("BYPASS")
	privGRPPRV  = privilegeBit("GRPPRV")
	privREADALL = privilegeBit("READALL")
)

// Credentials are what process p brings to an access check: its UIC and
// its current privileges. (The rights identifiers it holds come with
// Phase 93's subtask 5.)
func (p *Process) Credentials() security.Credentials {
	return security.Credentials{
		UIC:        security.UIC(p.UIC),
		Privileges: p.CurrentPrivileges,
	}
}

// uicAccess reports whether process p may have every access in want
// (accessRead, accessWrite, ...) to an object owned by owner (a UIC)
// with protection mask mask.
func (p *Process) uicAccess(owner uint32, mask uint16, want uint32) bool {
	profile := security.Profile{Owner: security.UIC(owner), Protection: security.Protection(mask)}

	return security.Check(p.Credentials(), profile, security.Access(want)).Granted
}
