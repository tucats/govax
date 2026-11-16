package uaf

import (
	"fmt"
	"strings"
)

// The longest names a record's fields may have, AUTHORIZE's limits:
// a username 12 characters (longer was NAMETOOBIG on VMS 7.1, the
// probe's case 13), an owner 31, an account 8, an identifier's name 31.
const (
	MaxUsername   = 12
	MaxOwner      = 31
	MaxAccount    = 8
	MaxIdentifier = 31
)

// ValidUsername reports whether name has a username's form: 1 to 12
// capital letters, digits, "$", and "_" ("BAD-NAME"
// was INVUSERNAME on VMS 7.1, the probe's case 14).
func ValidUsername(name string) bool {
	if name == "" || len(name) > MaxUsername {
		return false
	}

	for _, ch := range name {
		if !nameChar(ch) {
			return false
		}
	}

	return true
}

// ValidIdentifierName reports whether name has an identifier name's
// form: 1 to 31 capital letters, digits, "$", and "_", not all digits.
func ValidIdentifierName(name string) bool {
	if name == "" || len(name) > MaxIdentifier {
		return false
	}

	digits := true

	for _, ch := range name {
		if !nameChar(ch) {
			return false
		}

		if ch < '0' || ch > '9' {
			digits = false
		}
	}

	return !digits
}

// nameChar reports whether ch may be in a username or identifier name.
func nameChar(ch rune) bool {
	return ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '$' || ch == '_'
}

// Check reports the first thing wrong with the file: a version it can't
// read, a username or identifier name that can't be one, one used twice,
// a field too long, a priority over 31, two identifiers with one value,
// or an identifier held twice by one UIC. Each error
// names the account or identifier, so Load can find its line.
func (f *File) Check() error {
	if f.Version != Version {
		return &valueError{text: "version", err: fmt.Errorf("version %d is not a version govax can read (%d)", f.Version, Version)}
	}

	seen := map[string]bool{}

	for i := range f.Accounts {
		r := &f.Accounts[i]

		if err := r.Check(); err != nil {
			return err
		}

		if seen[r.Username] {
			return &valueError{text: r.Username, err: fmt.Errorf("account %s is in the file twice", r.Username)}
		}

		seen[r.Username] = true
	}

	names, values := map[string]bool{}, map[IdentifierValue]bool{}

	for _, id := range f.Identifiers {
		switch {
		case !ValidIdentifierName(id.Name):
			return &valueError{text: id.Name, err: fmt.Errorf("%q can't be an identifier's name", id.Name)}
		case names[id.Name]:
			return &valueError{text: id.Name, err: fmt.Errorf("identifier %s is in the file twice", id.Name)}
		case values[id.Value]:
			return &valueError{text: id.Value.String(), err: fmt.Errorf("identifier %s's value %s is another identifier's too", id.Name, id.Value)}
		}

		names[id.Name], values[id.Value] = true, true
	}

	for _, id := range f.Identifiers {
		holders := map[UIC]bool{}

		for _, h := range id.Holders {
			switch {
			case h.UIC.Group() > MaxGroup || h.UIC.Member() > MaxMember:
				return &valueError{text: id.Value.String(), err: fmt.Errorf("identifier %s's holder %s is out of range", id.Name, h.UIC)}
			case holders[h.UIC]:
				return &valueError{text: id.Value.String(), err: fmt.Errorf("identifier %s is held by %s twice", id.Name, h.UIC)}
			}

			holders[h.UIC] = true
		}
	}

	for _, r := range f.Accounts {
		if len(r.Legacy) > 0 {
			return &valueError{text: r.Username, err: fmt.Errorf("account %s still holds identifiers itself", r.Username)}
		}
	}

	return nil
}

// Check reports the first thing wrong with the record alone.
func (r *Record) Check() error {
	bad := func(format string, args ...any) error {
		return &valueError{text: r.Username, err: fmt.Errorf("account %s: "+format, append([]any{r.Username}, args...)...)}
	}

	switch {
	case !ValidUsername(r.Username):
		return &valueError{text: r.Username, err: fmt.Errorf("%q can't be a username: 1 to %d capital letters, digits, $, and _", r.Username, MaxUsername)}
	case len(r.Owner) > MaxOwner:
		return bad("the owner is over %d characters", MaxOwner)
	case len(r.Account) > MaxAccount:
		return bad("the account name is over %d characters", MaxAccount)
	case r.UIC.Group() > MaxGroup || r.UIC.Member() > MaxMember:
		return bad("UIC %s is out of range", r.UIC)
	case r.Priority > 31 || r.QueuePriority > 255:
		return bad("a priority is out of range")
	case r.Password != "" && !strings.HasPrefix(r.Password, "$argon2id$"):
		return bad("the password is not an Argon2id hash govax made")
	}

	return nil
}
