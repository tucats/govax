package corevms

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Privileges (docs/PHASE-26.md subtask 38): the process's privilege
// masks, $SETPRV, and the check the other services make.
//
// # What a privilege is
//
// Some VMS operations are too dangerous to allow every user: changing to
// kernel mode, creating a permanent mailbox, putting a name in the
// system-wide logical name table, raising a process's priority. Each is
// guarded by a *privilege* (CMKRNL, PRMMBX, SYSNAM, ALTPRI, ...), and a
// process may use it only while the privilege is enabled. There are 39;
// each is a bit in a quadword (64-bit) mask, numbered by $PRVDEF
// (PRV$V_CMKRNL is bit 0, ...), generated as vmsdef.Symbols.
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
// is authorized for, unless the process is authorized for SETPRV itself
// (which allows any) or the caller is in kernel or executive mode.
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

// The privileges services check.
var (
	privCMKRNL = privilegeBit("CMKRNL")
	privCMEXEC = privilegeBit("CMEXEC")
	privSYSNAM = privilegeBit("SYSNAM")
	privGRPNAM = privilegeBit("GRPNAM")
	privSYSPRV = privilegeBit("SYSPRV")
	privPRMCEB = privilegeBit("PRMCEB")
	privPRMMBX = privilegeBit("PRMMBX")
	privTMPMBX = privilegeBit("TMPMBX")
	privPSWAPM = privilegeBit("PSWAPM")
	privALTPRI = privilegeBit("ALTPRI")
	privSETPRV = privilegeBit("SETPRV")
	privOPER   = privilegeBit("OPER")
	privWORLD  = privilegeBit("WORLD")
	privDETACH = privilegeBit("DETACH")
	privNETMBX = privilegeBit("NETMBX")
	privNOACNT = privilegeBit("NOACNT")
)

// allPrivileges is every privilege's bit: the first PRV$K_NUMBER_OF_PRIVS
// bits of the mask.
var allPrivileges = uint64(1)<<vmsdef.Symbols["PRV$K_NUMBER_OF_PRIVS"] - 1

// ssNotAllPriv is SS$_NOTALLPRIV, $SETPRV's "done, but not every
// privilege you asked for" (a success status).
var ssNotAllPriv = vmsdef.Symbols["SS$_NOTALLPRIV"]

// hasPrivilege reports whether every privilege in mask is enabled
// (CURPRIV).
func (p *Process) hasPrivilege(mask uint64) bool {
	return p.CurrentPrivileges&mask == mask
}

// hasAnyPrivilege reports whether at least one privilege in mask is
// enabled, for the operations VMS allows with either of two privileges
// (SYSNAM or SYSPRV, ...).
func (p *Process) hasAnyPrivilege(mask uint64) bool {
	return p.CurrentPrivileges&mask != 0
}

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
// It enables (enbflg 1) or disables (0, the default) the privileges in
// the quadword mask at prvadr, in CURPRIV only (prmflg 0, the default:
// until the image exits) or in both CURPRIV and PROCPRIV (prmflg 1). The
// previous mask — CURPRIV's, or PROCPRIV's for a permanent change — is
// stored at prvprv if given. With prvadr omitted nothing changes, so a
// program can just read its privileges.
//
// A privilege can be enabled only if the process is authorized for it,
// or for SETPRV, or the caller is in kernel or executive mode; any others
// asked for are left alone and the status is SS$_NOTALLPRIV (a success).
// Disabling is always allowed. It returns SS$_NORMAL, SS$_NOTALLPRIV, or
// SS$_ACCVIO if prvadr can't be read or prvprv written (nothing changes).
func serviceSysSetprv(env *Environment, argv []uint32) (uint32, error) {
	enable := optArg(argv, 0)&0xFF != 0
	prvadr, permanent, prvprv := optArg(argv, 1), optArg(argv, 2)&0xFF != 0, optArg(argv, 3)
	p := env.Process

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

		mask = v & allPrivileges
	}

	if prvprv != 0 {
		if err := env.mem.StoreQuadword(env.cpu, prvprv, previous); err != nil {
			return ssAccVio, nil
		}
	}

	status := uint32(ssNormal)

	switch {
	case prvadr == 0:
		// Nothing to change.

	case enable:
		allowed := mask

		if env.cpu.PSL().CurMod() > vax.Executive && p.AuthorizedPrivileges&privSETPRV == 0 {
			allowed &= p.AuthorizedPrivileges

			if allowed != mask {
				status = ssNotAllPriv
			}
		}

		p.CurrentPrivileges |= allowed
		if permanent {
			p.ProcessPrivileges |= allowed
		}

	default:
		p.CurrentPrivileges &^= mask
		if permanent {
			p.ProcessPrivileges &^= mask
		}
	}

	return status, nil
}

func registerPrivilegeServices(t *ServiceTable) {
	t.Register("SYS$SETPRV", serviceSysSetprv)
}
