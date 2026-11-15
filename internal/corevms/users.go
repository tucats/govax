package corevms

import (
	"slices"
	"strings"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// The authorization file, as far as govax keeps one (docs/planning/
// PHASE-93 - file protection.md, subtask 13).
//
// On VMS each account is a record of SYSUAF.DAT (its UIC, its privileges,
// its quotas, ...), and the identifiers granted to it are records of
// RIGHTSLIST.DAT. govax has neither file, and no AUTHORIZE: every process
// is SYSTEM's unless something makes it another's. But $CHECK_ACCESS asks
// about a user by name, so the System keeps a table of accounts, which
// AddUser fills in as AUTHORIZE's ADD and GRANT/IDENTIFIER would, and
// which the tests use to stand for a probe's VAX. SYSTEM is always in it:
// UIC [1,4], every privilege authorized, no identifiers granted.

// UserAccount is one account: what $CHECK_ACCESS needs of its SYSUAF
// record and its rights.
type UserAccount struct {
	// Name is the username, in capitals.
	Name string

	// UIC is the account's UIC.
	UIC uint32

	// Privileges are its authorized privileges (UAF$Q_PRIV), PRV$V_ bits.
	Privileges uint64

	// Rights are the identifiers granted to it (RIGHTSLIST.DAT's holder
	// records), not the environmental ones a process gets at login.
	Rights []security.Identifier
}

// systemAccount is the SYSTEM account every System has.
var systemAccount = UserAccount{Name: nominalUsername, UIC: uint32(security.SystemUIC), Privileges: security.AllPrivileges}

// AddUser puts u in the System's authorization file, replacing an account
// of the same name (SYSTEM's too). A name that can't be a username (1 to
// 12 letters, digits, "$", and "_") is SS$_IVIDENT.
func (s *System) AddUser(u UserAccount) uint32 {
	u.Name = strings.ToUpper(strings.TrimSpace(u.Name))
	if !validUsername(u.Name) {
		return vmsdef.Symbols["SS$_IVIDENT"]
	}

	u.Rights = slices.Clone(u.Rights)

	if i := slices.IndexFunc(s.users, func(have UserAccount) bool { return have.Name == u.Name }); i >= 0 {
		s.users[i] = u
	} else {
		s.users = append(s.users, u)
	}

	return ssNormal
}

// User returns the account called name, in any case and with trailing
// blanks (VMS 7.1's $CHECK_ACCESS took "prot93b" and "PROT93B   ", the
// probe's cases 337 and 338).
func (s *System) User(name string) (UserAccount, bool) {
	name = strings.ToUpper(strings.TrimRight(name, " "))

	for _, u := range s.users {
		if u.Name == name {
			return u, true
		}
	}

	if name == systemAccount.Name {
		return systemAccount, true
	}

	return UserAccount{}, false
}

// AccountCredentials are what an access check on the account's behalf
// uses: its UIC, its authorized privileges (not the defaults a process
// would start with: VMS 7.1 granted PROT93B and PROT93C, whose SYSPRV and
// BYPASS are authorized but not default, every file, the probe's cases 289
// to 336), and the identifiers granted to it, without the environmental
// ones, which belong to a process (PROT93A was refused a file whose only
// grant was INTERACTIVE, case 322). maxSysGroup is MAXSYSGROUP.
func (u UserAccount) AccountCredentials(maxSysGroup uint32) security.Credentials {
	return security.Credentials{
		UIC:         security.UIC(u.UIC),
		Privileges:  u.Privileges,
		Rights:      slices.Clone(u.Rights),
		MaxSysGroup: maxSysGroup,
	}
}

// validUsername reports whether name has a username's form.
func validUsername(name string) bool {
	if name == "" || len(name) > 12 {
		return false
	}

	for _, ch := range name {
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9', ch == '$', ch == '_':
		default:
			return false
		}
	}

	return true
}
