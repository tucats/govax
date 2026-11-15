package securertl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// $CHECK_ACCESS: whether a named user may access an object
// (docs/planning/PHASE-93 - file protection.md, subtask 13; A.4's "Round
// 3"; VSI's system services pages).
//
// A server process asks on behalf of a user, by the user's name: the
// service finds the account in the authorization file (govax's is the
// System's table of accounts, corevms.UserAccount), and makes the check a
// file's own access would with the account's UIC, its granted identifiers,
// and its authorized privileges, as $CHKPRO's item list says what to ask
// and what to answer (corevms.CheckAccessItems).

// More statuses.
var (
	ssNoCallPriv = vmsdef.Symbols["SS$_NOCALLPRIV"]
	ssNoClass    = vmsdef.Symbols["SS$_NOCLASS"]
	rmsRNF       = vmsdef.Symbols["RMS$_RNF"]
)

// checkClasses are the object classes $CHECK_ACCESS takes by name (the
// clsnam argument), and the ACL$C_ object types they stand for.
var checkClasses = []struct {
	name   string
	objtyp uint32
}{
	{"FILE", objFile},
	{"DEVICE", objDevice},
}

// authorizationFile is what the caller must be able to read to use the
// service: SYSUAF.DAT, owned by SYSTEM, (S:RWED,O:RWED,G,W), as it comes.
var authorizationFile = security.Profile{Owner: security.SystemUIC, Protection: 0xFF00}

// checkAccess is SYS$CHECK_ACCESS [objtyp] ,[objnam] ,[usrnam] ,itmlst
// ,[contxt] ,[clsnam] ,[objpro] ,[usrpro]. As VMS 7.1's (A.4):
//
//   - a caller that may not read the authorization file is refused the
//     service, SS$_NOCALLPRIV (PROT93C without privileges; SYSTEM without
//     READALL, GRPPRV, SYSPRV, and BYPASS isn't, its UIC being a system
//     one, so the privilege needed is govax's choice: read access to
//     SYSUAF.DAT, as VSI's page says);
//   - the object's class comes from objtyp or clsnam, not both
//     (SS$_BADPARAM), at least one (SS$_INSFARG); a class name it doesn't
//     know is SS$_NOCLASS, and govax takes FILE and DEVICE;
//   - a user who isn't in the authorization file, or no name, is
//     RMS$_RNF, the record not found;
//   - a file is found by RMS's rules, with RMS's statuses (RMS$_FNF), and
//     a device by its name, a file's name being SS$_NOPRIV for a device.
//
// contxt (the authorization file kept open), objpro, and usrpro are taken
// and not used: govax reads its table each time and takes no encoded
// profiles.
func checkAccess(env *corevms.Environment, argv []uint32) (uint32, error) {
	if !security.Check(env.Credentials(), authorizationFile, security.Read).Granted {
		return ssNoCallPriv, nil
	}

	objtyp, status := checkClass(env, arg(argv, 0), arg(argv, 5))
	if status != ssNormal {
		return status, nil
	}

	name, err := librtl.ReadString(env, arg(argv, 2))
	if arg(argv, 2) != 0 && err != nil {
		return ssAccVio, nil
	}

	user, ok := env.System.User(name)
	if !ok {
		return rmsRNF, nil
	}

	objnam, err := librtl.ReadString(env, arg(argv, 1))
	if err != nil {
		return ssAccVio, nil
	}

	var profile security.Profile

	if objtyp == objDevice {
		// govax keeps no device profiles: a device is owned by [0,0]
		// with no ACL, so anyone may read, write, and so on, but not
		// control it (unconfirmed; VMS 7.1 gave PROT93C read of the
		// probe's disk, case 345).
		if !env.DeviceNamed(objnam) {
			return ssNoPriv, nil
		}
	} else {
		target, status := findTarget(env, objFile, 0, arg(argv, 1))
		if status != ssNormal {
			return status, nil
		}

		profile = target.Profile()
	}

	return env.CheckAccessItems(arg(argv, 3), user.AccountCredentials(env.Process.MaxSysGroup), profile), nil
}

// checkClass is the object's class: objtyp's longword, or clsnam's name,
// which may be any abbreviation of one, in any case (unconfirmed for
// $CHECK_ACCESS; $GET_SECURITY took them).
func checkClass(env *corevms.Environment, objtypAddr, clsnamAddr uint32) (uint32, uint32) {
	switch {
	case objtypAddr != 0 && clsnamAddr != 0:
		return 0, ssBadParam
	case objtypAddr == 0 && clsnamAddr == 0:
		return 0, ssInsfArg
	case objtypAddr != 0:
		objtyp, err := env.Memory().LoadLongword(env.CPU(), objtypAddr)
		if err != nil {
			return 0, ssAccVio
		}

		if objtyp != objFile && objtyp != objDevice {
			return 0, ssBadParam
		}

		return objtyp, ssNormal
	}

	name, err := librtl.ReadString(env, clsnamAddr)
	if err != nil {
		return 0, ssAccVio
	}

	name = strings.ToUpper(strings.TrimSpace(name))

	for _, c := range checkClasses {
		if name != "" && strings.HasPrefix(c.name, name) {
			return c.objtyp, ssNormal
		}
	}

	return 0, ssNoClass
}
