package corevms

import (
	"fmt"
	"slices"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/security"
)

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

// Credentials are what process p brings to an access check: its UIC, its
// current privileges, the rights identifiers it holds, and MAXSYSGROUP.
// Nothing keeps a copy: each check asks again, so a privilege turned off
// counts at once.
func (p *Process) Credentials() security.Credentials {
	return security.Credentials{
		UIC:         security.UIC(p.UIC),
		Privileges:  p.CurrentPrivileges,
		Rights:      slices.Clone(p.Rights),
		MaxSysGroup: p.MaxSysGroup,

		DefaultProtection: security.Protection(p.DefaultProtection),
	}
}

// Credentials are the credentials of env's process, for RMS and the DCL
// utilities (rms.Context.Credentials, rms.Session.Credentials). While SET
// DEBUG PROTECTION is on they trace each check made with them to the
// debug output, a "DEBUG(PROTECTION):" line each (security.Check).
func (env *Environment) Credentials() security.Credentials {
	creds := env.Process.Credentials()

	if env.System != nil && env.cpu != nil && env.cpu.DebugEnabled(registers.DebugProtection) {
		w := env.cpu.DebugWriter()
		creds.Trace = func(line string) { fmt.Fprintf(w, "DEBUG(PROTECTION): %s\n", line) }
	}

	return creds
}

// uicAccess reports whether env's process may have every access in want
// (accessRead, accessWrite, ...) to the object called name, owned by
// owner (a UIC) with protection mask mask.
func (env *Environment) uicAccess(name string, owner uint32, mask uint16, want uint32) bool {
	profile := security.Profile{Owner: security.UIC(owner), Protection: security.Protection(mask), Name: name}

	return security.Check(env.Credentials(), profile, security.Access(want)).Granted
}
