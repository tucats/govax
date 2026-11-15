package corevms

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// Privileges (docs/planning/PHASE-26.md subtask 38): the process's privilege
// masks and $SETPRV. The checks the other services make go through the
// gate, privrules.go.
//
// # What a privilege is
//
// Some VMS operations are too dangerous to allow every user: changing to
// kernel mode, creating a permanent mailbox, putting a name in the
// system-wide logical name table, raising a process's priority. Each is
// guarded by a *privilege* (CMKRNL, PRMMBX, SYSNAM, ALTPRI, ...), and a
// process may use it only while the privilege is enabled. There are 39;
// each is a bit in a quadword (64-bit) mask, numbered by $PRVDEF
// (PRV$V_CMKRNL is bit 0, ...), generated as vmsdef.Symbols. The other 25
// bits name nothing, but a mask keeps them: ALL is every bit.
//
// A process has four such masks:
//
//   - AUTHPRIV, the privileges it is *authorized* to enable (from its
//     user's authorization record). It never changes.
//   - PROCPRIV, its *permanent* privileges: enabled whenever no image is
//     running, and after one exits.
//   - IMAGPRIV, the privileges of the running image, if it was installed
//     with some. govax has no installed images, so this is always empty.
//   - CURPRIV, the privileges *enabled right now*: the ones every
//     service checks.
//
// A process starts with all three of AUTHPRIV, PROCPRIV, and CURPRIV
// alike. $SETPRV enables or disables privileges in CURPRIV (temporarily:
// until the image exits, when PROCPRIV is copied back) or in both CURPRIV
// and PROCPRIV (permanently). It can only enable privileges the process
// is authorized for, unless the process holds SETPRV (which allows any)
// or the caller is in kernel or executive mode.
//
// govax's process is the SYSTEM account's, which is authorized for, and
// starts with, every privilege: so programs that don't use $SETPRV see
// no change from before privileges existed.

// privilegeBit returns the mask bit of the privilege called name (the
// part after PRV$V_), panicking for a name $PRVDEF doesn't have: they
// are package-level constants below, so a misspelling fails at once.
func privilegeBit(name string) uint64 {
	n, ok := vmsdef.Symbols["PRV$V_"+name]
	if !ok {
		panic("rtl: no $PRVDEF privilege " + name)
	}

	return 1 << n
}

// privSETPRV is the privilege that lets $SETPRV enable any privilege.
// The other privileges' bits are the rules' (privrules.go).
var privSETPRV = privilegeBit("SETPRV")

// allPrivileges is every bit of a mask. There are 39 privileges, but VMS
// keeps all 64 bits: AUTHORIZE's /PRIVILEGES=ALL is FFFFFFFFFFFFFFFF, and
// $SETPRV of an all-ones mask sets every bit (testdata/priv94, the phase
// doc's A.4), so govax does too.
var allPrivileges = security.AllPrivileges

// ssNotAllPriv is SS$_NOTALLPRIV, $SETPRV's "done, but not every
// privilege you asked for" (a success status).
var ssNotAllPriv = vmsdef.Symbols["SS$_NOTALLPRIV"]

// resetImagePrivileges is image rundown's privilege step: the privileges
// the image enabled temporarily go, as the permanent ones are copied
// back to the current mask.
func (env *Environment) resetImagePrivileges() {
	env.Process.CurrentPrivileges = env.Process.ProcessPrivileges
}

// serviceSysSetprv is SYS$SETPRV:
//
//	SYS$SETPRV [enbflg] ,[prvadr] ,[prmflg] ,[prvprv]
//
// It enables (enbflg's low bit set) or disables (clear, the default) the
// privileges in the quadword mask at prvadr, in CURPRIV only (prmflg 0,
// the default: until the image exits) or in both CURPRIV and PROCPRIV
// (prmflg 1); any other prmflg is SS$_IVSTSFLG, with nothing changed or
// stored. The previous mask — CURPRIV's, or PROCPRIV's for a permanent
// change — is stored at prvprv if given. With prvadr omitted nothing
// changes, so a program can just read its privileges.
//
// A privilege can be enabled only if the process is authorized for it,
// or holds SETPRV now, or the caller is in kernel or executive mode; any
// others asked for are left alone and the status is SS$_NOTALLPRIV (a
// success). Disabling is always allowed. It returns SS$_NORMAL,
// SS$_NOTALLPRIV, SS$_IVSTSFLG, or SS$_ACCVIO if prvadr can't be read or
// prvprv written (nothing changes). VMS 7.1's answers to each
// combination are testdata/priv94's part S (the phase doc's A.4).
func serviceSysSetprv(env *Environment, argv []uint32) (uint32, error) {
	enable := optArg(argv, 0)&1 != 0
	prvadr, prmflg, prvprv := optArg(argv, 1), optArg(argv, 2), optArg(argv, 3)
	p := env.Process

	if prmflg > 1 {
		return ssIvStsFlg, nil
	}

	permanent := prmflg == 1

	previous := p.CurrentPrivileges
	if permanent {
		previous = p.ProcessPrivileges
	}

	var mask uint64

	if prvadr != 0 {
		v, err := env.mem.LoadQuadword(env.cpu, prvadr)
		if err != nil {
			return ssAccVio, nil
		}

		mask = v
	}

	if prvprv != 0 {
		if err := env.mem.StoreQuadword(env.cpu, prvprv, previous); err != nil {
			return ssAccVio, nil
		}
	}

	if prvadr == 0 {
		return ssNormal, nil
	}

	return env.setPrivileges(mask, enable, permanent, env.callerMode()), nil
}

// setPrivileges is $SETPRV's change: it enables (or, without enable,
// disables) the privileges in mask, in the current privileges and, with
// permanent, the process's permanent ones too, for a caller in mode.
// Privileges the process isn't authorized for are enabled only by the
// gate's rule (SETPRV held now, or a caller in an inner mode); otherwise they're left out and the
// status is SS$_NOTALLPRIV, a success. Otherwise SS$_NORMAL.
func (env *Environment) setPrivileges(mask uint64, enable, permanent bool, mode registers.AccessMode) uint32 {
	p := env.Process

	if !enable {
		p.CurrentPrivileges &^= mask
		if permanent {
			p.ProcessPrivileges &^= mask
		}

		return ssNormal
	}

	status := uint32(ssNormal)
	allowed := mask

	if mask&^p.AuthorizedPrivileges != 0 && !env.privilegeGranted(ruleSetprvUnauthorized, mode) {
		allowed &= p.AuthorizedPrivileges
		status = ssNotAllPriv
	}

	p.CurrentPrivileges |= allowed
	if permanent {
		p.ProcessPrivileges |= allowed
	}

	return status
}

func registerPrivilegeServices(t *ServiceTable) {
	t.Register("SYS$SETPRV", serviceSysSetprv)
}
