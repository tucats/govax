package corevms

import (
	"slices"
	"strings"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmsdef"
)

// The accounts (docs/planning/PHASE-93 - file protection.md, subtask 13;
// since Phase 95 the authorization file's, authorization.go).
//
// $CHECK_ACCESS asks about a user by name, and $IDTOASC names a UIC by
// its account's username. A UserAccount is what they need of an account:
// its name, UIC, authorized privileges, and the identifiers it holds.
// AddUser puts one in the authorization file, as AUTHORIZE's ADD and
// GRANT/IDENTIFIER would, which is how the tests stand for a probe's VAX.
// SYSTEM is always found: if the file has lost it, uaf.SystemRecord's.

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

// AddUser puts u in the System's authorization file: a new account is
// the DEFAULT record's (without DISUSER) with u's name, UIC, authorized
// privileges, and identifiers; an account of the same name, SYSTEM's too,
// gets u's in place of its own. As AUTHORIZE's ADD does, it adds a UIC
// identifier named for the account, unless the name or the UIC already
// has one. A name that can't be a username (1 to 12
// letters, digits, "$", and "_") is SS$_IVIDENT. An identifier the file
// doesn't name is held by its value.
func (s *System) AddUser(u UserAccount) uint32 {
	name := strings.ToUpper(strings.TrimSpace(u.Name))
	if !uaf.ValidUsername(name) {
		return vmsdef.Symbols["SS$_IVIDENT"]
	}

	err := s.UpdateAuthorization(func(f *uaf.File) error {
		r, ok := f.Find(name)
		if !ok {
			def := uaf.DefaultRecord()
			if have, ok := f.Find("DEFAULT"); ok {
				def = *have
			}

			def.Username, def.Flags = name, def.Flags&^uaf.FlagDISUSER
			def.Identifiers = slices.Clone(def.Identifiers)
			f.Accounts = append(f.Accounts, def)
			r = &f.Accounts[len(f.Accounts)-1]
		}

		r.UIC, r.Privileges = uaf.UIC(u.UIC), uaf.Privileges(u.Privileges)
		r.Identifiers = []uaf.Held{}

		for _, id := range u.Rights {
			held := uaf.Held{Name: uaf.IdentifierValue(id.Value).String(), Attributes: uaf.Attributes(id.Attributes)}
			if named, ok := f.IdentifierWithValue(uaf.IdentifierValue(id.Value)); ok {
				held.Name = named.Name
			}

			r.Identifiers = append(r.Identifiers, held)
		}

		_, named := f.IdentifierNamed(name)
		_, valued := f.IdentifierWithValue(uaf.IdentifierValue(u.UIC))

		if !named && !valued {
			f.Identifiers = append(f.Identifiers, uaf.Identifier{Name: name, Value: uaf.IdentifierValue(u.UIC)})
		}

		return nil
	})
	if err != nil {
		return vmsdef.Symbols["SS$_IVIDENT"]
	}

	return ssNormal
}

// User returns the account called name, in any case and with trailing
// blanks (VMS 7.1's $CHECK_ACCESS took "prot93b" and "PROT93B   ", the
// probe's cases 337 and 338).
func (s *System) User(name string) (UserAccount, bool) {
	name = strings.ToUpper(strings.TrimRight(name, " "))

	if f, err := s.Authorization(); err == nil {
		if r, ok := f.Find(name); ok {
			return userAccount(f, r), true
		}
	}

	if sys := uaf.SystemRecord(); name == sys.Username {
		return userAccount(uaf.Default(), &sys), true
	}

	return UserAccount{}, false
}

// userAccount is the UserAccount of f's record r: the identifiers it
// holds by value, any f can't name left out.
func userAccount(f *uaf.File, r *uaf.Record) UserAccount {
	u := UserAccount{Name: r.Username, UIC: uint32(r.UIC), Privileges: uint64(r.Privileges)}

	for _, h := range r.Identifiers {
		if v, ok := f.HeldValue(h); ok {
			u.Rights = append(u.Rights, security.Identifier{Value: uint32(v), Attributes: uint32(h.Attributes)})
		}
	}

	return u
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
