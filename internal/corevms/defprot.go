package corevms

import "github.com/tucats/govax/internal/vmsdef"

// The process's default file protection (docs/planning/PHASE-93 - file
// protection.md, Decision 3).
//
// Every VMS process has a default protection code, the one a new file
// gets when nothing else gives it one: not an explicit code, not the
// previous version's, not its directory's default protection ACE. It
// starts as the system parameter RMS_FILEPROT, normally %XFA00,
// (S:RWED,O:RWED,G:RE,W); a process creates others with its own; and
// DCL's SET PROTECTION/DEFAULT, or a program's $SETDFPROT, changes it.
// The code is kept in the Process (DefaultProtection) as VMS stores
// codes: a set bit denies.

// Status values $SETDFPROT returns. It is one of RMS's services, as its
// statuses show.
var (
	rmsNormal = vmsdef.Symbols["RMS$_NORMAL"]
	rmsIAL    = vmsdef.Symbols["RMS$_IAL"]
)

// serviceSysSetdfprot is SYS$SETDFPROT:
//
//	SYS$SETDFPROT [new-def-prot-addr] ,[cur-def-prot-addr]
//
// It stores the process's default protection in the word at
// cur-def-prot-addr, if given, then makes the word at new-def-prot-addr,
// if given, the new default. Both may be the same word, which then trades
// the old code for the new. It returns RMS$_NORMAL, or RMS$_IAL, the
// manual's other status, when it can't read or write a word, changing
// nothing (unconfirmed: which status VMS gives a word it can't reach).
// The manual says there are no $SETDFPROT_S or _G macros; a program calls
// the service itself.
func serviceSysSetdfprot(env *Environment, argv []uint32) (uint32, error) {
	newAddr, curAddr := optArg(argv, 0), optArg(argv, 1)
	p := env.Process

	var code uint16

	if newAddr != 0 {
		v, err := env.mem.LoadWord(env.cpu, newAddr)
		if err != nil {
			return rmsIAL, nil
		}

		code = v
	}

	if curAddr != 0 {
		if err := env.mem.StoreWord(env.cpu, curAddr, p.DefaultProtection); err != nil {
			return rmsIAL, nil
		}
	}

	if newAddr != 0 {
		p.DefaultProtection = code
	}

	return rmsNormal, nil
}

func registerDefaultProtectionServices(t *ServiceTable) {
	t.Register("SYS$SETDFPROT", serviceSysSetdfprot)
}
