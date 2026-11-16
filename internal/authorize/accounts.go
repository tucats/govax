package authorize

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/uaf"
)

// This file is AUTHORIZE's account commands: ADD, COPY, MODIFY and
// DEFAULT, REMOVE, and RENAME, each as VMS 7.1's did them in
// testdata/uaf95 (the phase doc's A.5). Each is one change to the file
// (Store.UpdateAuthorization), whose messages are written when it has
// been kept.

// errNoChange ends a change that is refused, after its message: the
// file is left as it was.
var errNoChange = errors.New("no change")

// change runs body on the authorization file and keeps what it did,
// then writes the messages body queued. body returns errNoChange (or
// another error) to leave the file alone. A failure to keep the file is
// written as AUTHORIZE's NAOFIL, with what went wrong.
func (u *Utility) change(body func(f *uaf.File, say *sayings) error) {
	say := &sayings{}

	err := u.Store.UpdateAuthorization(func(f *uaf.File) error {
		say.reset()

		return body(f, say)
	})

	say.write(u)

	if err != nil && !errors.Is(err, errNoChange) {
		u.message("NAOFIL", "")
		fmt.Fprintf(u.Out, "-GOVAX-E-FILE, %v\n", err)
		u.changedAccounts, u.changedRights = false, false
	}
}

// sayings are messages a change writes once it is kept: in order, each
// AUTHORIZE's message or a system status under the one before.
type sayings struct {
	lines []func(u *Utility)
}

func (s *sayings) reset() { s.lines = nil }

func (s *sayings) message(ident, segment string, args ...string) {
	s.lines = append(s.lines, func(u *Utility) { u.message(ident, segment, args...) })
}

func (s *sayings) cause(code uint32) {
	s.lines = append(s.lines, func(u *Utility) { u.cause(code) })
}

func (s *sayings) write(u *Utility) {
	for _, line := range s.lines {
		line(u)
	}
}

// username reads a username parameter as AUTHORIZE takes one, for a new
// account: NAMETOOBIG over 12 characters, INVUSERNAME with a character a
// username can't have (cases 13 and 14), each leaving its own condition.
func (u *Utility) username(text string) (string, bool) {
	name := strings.ToUpper(strings.TrimSpace(text))

	switch {
	case len(name) > uaf.MaxUsername:
		u.message("NAMETOOBIG", "")
		u.status = uafStatus("NAMETOOBIG")

		return "", false
	case !uaf.ValidUsername(name):
		u.message("INVUSERNAME", "")
		u.status = uafStatus("INVUSERNAME")

		return "", false
	}

	return name, true
}

// add is ADD username [/qualifiers]: a new account, DEFAULT's record with
// the qualifiers' fields (DISUSER and all, case 9), then its UIC
// identifier and, given /ACCOUNT, a group identifier for its group if the
// group has none (case 22). A UIC another identifier has is
// RDBADDERRU's DUPIDENT, the account added anyway (case 17).
func (u *Utility) add(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	e, ok := u.readEdit(r)
	if !ok {
		return nil
	}

	name, ok := u.username(r.String("USERNAME"))
	if !ok {
		return nil
	}

	u.change(func(f *uaf.File, say *sayings) error {
		if _, exists := f.Find(name); exists {
			say.message("UAEERR", "")
			u.status = uafStatus("UAEERR")

			return errNoChange
		}

		rec := uaf.DefaultRecord()
		if def, ok := f.Find("DEFAULT"); ok {
			rec = *def
		}

		rec.Username = name
		rec.Password, rec.PasswordChanged = "", time.Time{}
		rec.LastInteractive, rec.LastNonInteractive = time.Time{}, time.Time{}
		e.apply(&rec)
		f.Accounts = append(f.Accounts, rec)

		say.message("ADDMSG", "")
		u.changedAccounts = true

		if e.cli {
			say.message("CLIWARNMSG", "")
		}

		u.addUICIdentifier(f, say, name, rec.UIC)

		if e.account != "" {
			u.addGroupIdentifier(f, say, e.account, rec.UIC)
		}

		return nil
	})

	return nil
}

// addUICIdentifier adds the identifier name with uic's value, a new
// account's, saying RDBADDMSGU, or RDBADDERRU with DUPIDENT when another
// identifier has the value (DUPLNAM when another has the name).
func (u *Utility) addUICIdentifier(f *uaf.File, say *sayings, name string, uic uaf.UIC) {
	value := uaf.IdentifierValue(uic)

	if _, taken := f.IdentifierWithValue(value); taken {
		say.message("RDBADDERRU", "", name, identifierValue(value))
		say.cause(ssDUPIDENT)

		return
	}

	if _, taken := f.IdentifierNamed(name); taken {
		say.message("RDBADDERRU", "", name, identifierValue(value))
		say.cause(ssDUPLNAM)

		return
	}

	f.Identifiers = append(f.Identifiers, uaf.Identifier{Name: name, Value: value})
	say.message("RDBADDMSGU", "", name, identifierValue(value))
	u.changedRights = true
}

// addGroupIdentifier adds account as the identifier of uic's group,
// [group,177777], if the group has none and the name can be an unused
// identifier's (unconfirmed: the probe's case 22 made one, CASED, and
// gave no case where it couldn't).
func (u *Utility) addGroupIdentifier(f *uaf.File, say *sayings, account string, uic uaf.UIC) {
	name := strings.ToUpper(account)
	value := uaf.IdentifierValue(uaf.NewUIC(uic.Group(), 0o177777))

	if _, has := f.IdentifierWithValue(value); has {
		return
	}

	if _, taken := f.IdentifierNamed(name); taken || !uaf.ValidIdentifierName(name) {
		return
	}

	f.Identifiers = append(f.Identifiers, uaf.Identifier{Name: name, Value: value})
	say.message("RDBADDMSGU", "", name, identifierValue(value))
	u.changedRights = true
}

// copyAccount is COPY old new [/qualifiers]: a new account, old's record
// with the qualifiers' fields. It has no password unless /PASSWORD gives
// one (DEFPWD: VMS's password hash includes the username) and no logins;
// what it holds is what its UIC holds. Its UIC gets an identifier, or if it
// is old's, SAMEUIC (cases 54 and 56). A missing old is BADUSR, leaving
// %X10000000 (case 57).
func (u *Utility) copyAccount(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	e, ok := u.readEdit(r)
	if !ok {
		return nil
	}

	oldName := strings.ToUpper(strings.TrimSpace(r.String("OLD")))

	newName, ok := u.username(r.String("NEW"))
	if !ok {
		return nil
	}

	u.change(func(f *uaf.File, say *sayings) error {
		old, ok := f.Find(oldName)
		if !ok {
			say.message("BADUSR", oldName)
			u.status = stsInhibitMsg

			return errNoChange
		}

		if _, exists := f.Find(newName); exists {
			say.message("UAEERR", "")
			u.status = uafStatus("UAEERR")

			return errNoChange
		}

		rec, oldUIC := *old, old.UIC
		rec.Username = newName
		rec.Password, rec.PasswordChanged = "", time.Time{}
		rec.LastInteractive, rec.LastNonInteractive = time.Time{}, time.Time{}
		e.apply(&rec)
		f.Accounts = append(f.Accounts, rec)

		say.message("COPMSG", "")
		u.changedAccounts = true

		if !e.password {
			say.message("DEFPWD", "")
		}

		if e.cli {
			say.message("CLIWARNMSG", "")
		}

		if rec.UIC == oldUIC {
			say.message("SAMEUIC", "")
		} else {
			u.addUICIdentifier(f, say, newName, rec.UIC)
		}

		return nil
	})

	return nil
}

// modify is MODIFY username [/qualifiers], and DEFAULT [/qualifiers]
// (MODIFY DEFAULT): every account the username matches, with % and *
// wildcards, changed, and one MDFYMSG (cases 47 and 48). A username that
// matches nothing is BADSPC (case 50), and no qualifiers change nothing
// and say nothing (case 51). A new UIC changes the value of the
// identifier named for the account, RDBMDFYMSG, or RDBMDFYERR with
// DUPIDENT when another identifier has it, the record changed anyway
// (cases 35 and 52).
func (u *Utility) modify(r *dcl.Result, pattern string) error {
	if !u.mayWrite() {
		return nil
	}

	e, ok := u.readEdit(r)
	if !ok {
		return nil
	}

	pattern = strings.ToUpper(strings.TrimSpace(pattern))

	u.change(func(f *uaf.File, say *sayings) error {
		var matched []int

		for i := range f.Accounts {
			if wildcardMatch(pattern, f.Accounts[i].Username) {
				matched = append(matched, i)
			}
		}

		if len(matched) == 0 {
			say.message("BADSPC", "")
			u.status = uafStatus("BADSPC")

			return errNoChange
		}

		if e.empty() {
			return errNoChange
		}

		if e.noPassword {
			say.message("PWDLESSMIN", "")
		}

		for _, i := range matched {
			rec := &f.Accounts[i]
			oldUIC := rec.UIC

			e.apply(rec)

			if e.hasUIC && rec.UIC != oldUIC {
				u.moveUICIdentifier(f, say, rec.Username, rec.UIC)
			}
		}

		say.message("MDFYMSG", "")
		u.changedAccounts = true

		if e.cli {
			say.message("CLIWARNMSG", "")
		}

		return nil
	})

	return nil
}

// moveUICIdentifier gives the UIC identifier called name the value of
// uic, an account's new UIC, and its holder records go with it, as
// $MOD_IDENT moves them.
func (u *Utility) moveUICIdentifier(f *uaf.File, say *sayings, name string, uic uaf.UIC) {
	id, ok := f.IdentifierNamed(name)
	if !ok || id.Value&uaf.GeneralBit != 0 {
		return
	}

	if _, taken := f.IdentifierWithValue(uaf.IdentifierValue(uic)); taken {
		say.message("RDBMDFYERR", "", id.Name)
		say.cause(ssDUPIDENT)

		return
	}

	f.MoveHolder(uaf.UIC(id.Value), uic)
	id.Value = uaf.IdentifierValue(uic)
	say.message("RDBMDFYMSG", "", id.Name)
	u.changedRights = true
}

// remove is REMOVE username: the account, and the UIC identifier named
// for it (RDBREMMSGU, case 65) with its holder records and those of its
// UIC ($REM_IDENT's rule). A missing account is BADUSR, leaving
// SS$_NORMAL (case 67).
func (u *Utility) remove(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	name := strings.ToUpper(strings.TrimSpace(r.String("USERNAME")))

	u.change(func(f *uaf.File, say *sayings) error {
		if !f.Remove(name) {
			say.message("BADUSR", name)

			return errNoChange
		}

		say.message("REMMSG", "")
		u.changedAccounts = true

		id, ok := f.IdentifierNamed(name)
		if !ok || id.Value&uaf.GeneralBit != 0 {
			return nil
		}

		gone := *id
		f.Identifiers = slices.DeleteFunc(f.Identifiers, func(i uaf.Identifier) bool { return i.Name == gone.Name })
		f.DropHolder(uaf.UIC(gone.Value))

		say.message("RDBREMMSGU", "", gone.Name, identifierValue(gone.Value))
		u.changedRights = true

		return nil
	})

	return nil
}

// rename is RENAME old new: the account's username, its password gone
// (DEFPWD), and the identifier whose value is its UIC renamed new,
// RDBMDFYMSG naming the identifier as it was. VMS 7.1 found that
// identifier by value, not name: renaming an account that shared another
// account's UIC renamed the other's identifier (case 114). A missing old
// is BADUSR and a new name in use UAEERR, both leaving SS$_NORMAL (cases
// 63 and 64).
func (u *Utility) rename(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	oldName := strings.ToUpper(strings.TrimSpace(r.String("OLD")))

	newName, ok := u.username(r.String("NEW"))
	if !ok {
		return nil
	}

	u.change(func(f *uaf.File, say *sayings) error {
		rec, ok := f.Find(oldName)
		if !ok {
			say.message("BADUSR", oldName)

			return errNoChange
		}

		if _, exists := f.Find(newName); exists {
			say.message("UAEERR", "")

			return errNoChange
		}

		rec.Username = newName
		rec.Password, rec.PasswordChanged = "", time.Time{}

		say.message("RENMSG", "")
		say.message("DEFPWD", "")
		u.changedAccounts = true

		id, ok := f.IdentifierWithValue(uaf.IdentifierValue(rec.UIC))
		if !ok {
			return nil
		}

		if _, taken := f.IdentifierNamed(newName); taken {
			say.message("RDBMDFYERR", "", id.Name)
			say.cause(ssDUPLNAM)

			return nil
		}

		was := id.Name
		id.Name = newName

		say.message("RDBMDFYMSG", "", was)
		u.changedRights = true

		return nil
	})

	return nil
}

// wildcardMatch reports whether name matches pattern, in which "%" is
// any one character and "*" any run of them.
func wildcardMatch(pattern, name string) bool {
	if pattern == "" {
		return name == ""
	}

	switch pattern[0] {
	case '*':
		for i := 0; i <= len(name); i++ {
			if wildcardMatch(pattern[1:], name[i:]) {
				return true
			}
		}

		return false
	case '%':
		return name != "" && wildcardMatch(pattern[1:], name[1:])
	}

	return name != "" && pattern[0] == name[0] && wildcardMatch(pattern[1:], name[1:])
}
