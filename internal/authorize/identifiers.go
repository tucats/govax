package authorize

import (
	"slices"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/uaf"
)

// This file is AUTHORIZE's identifier commands: ADD/IDENTIFIER,
// REMOVE/IDENTIFIER, RENAME/IDENTIFIER, MODIFY/IDENTIFIER, and GRANT and
// REVOKE/IDENTIFIER, as VMS 7.1's did them in testdata/uaf95 (the phase
// doc's A.5 and Decision 11). A failure says AUTHORIZE's message with
// the system's reason under it, and leaves that reason with the inhibit
// bit in $STATUS (GRANTERR's DUPIDENT, %X1000222C, case 90).
//
// The environmental identifiers (INTERACTIVE and the rest) aren't in the
// file: the system gives them. Their names and values can't be another
// identifier's, and they can't be removed, renamed, changed, or granted:
// such a command is refused with NOPRIV (unconfirmed).

// The /VALUE forms' range: IDENTIFIER=n takes n from 65,536 to
// 268,435,455 and sets bit 31 itself (VMS 7.1 refused %X80011234 with
// BADVALUE, showing the number as a signed longword, case 77).
const (
	minGeneralNumber = 0x10000
	maxGeneralNumber = 0x0FFFFFFF
)

// parseValue reads /VALUE's text for a command, saying BADVALUE and
// leaving %X10000000 (case 77) when it can't.
func (u *Utility) parseValue(text string) (uaf.IdentifierValue, bool) {
	v, shown, ok := valueOf(text)
	if !ok {
		u.message("BADVALUE", shown)
		u.status = stsInhibitMsg
	}

	return v, ok
}

// valueOf reads /VALUE's "IDENTIFIER=n" or "UIC=[g,m]" (a member of "*"
// the group's identifier, [g,177777], round 3's drop procedure). When it
// can't, shown is the text BADVALUE shows: an IDENTIFIER number out of
// range as a signed longword ("-2147413452", case 77).
func valueOf(text string) (v uaf.IdentifierValue, shown string, ok bool) {
	t := strings.ToUpper(strings.TrimSpace(text))

	kind, rest, cut := strings.Cut(t, "=")
	if !cut {
		kind, rest, cut = strings.Cut(t, ":")
	}

	switch {
	case cut && kind != "" && strings.HasPrefix("IDENTIFIER", kind):
		n, err := parseNumber(rest)
		if err != nil {
			return 0, rest, false
		}

		if n < minGeneralNumber || n > maxGeneralNumber {
			return 0, strconv.Itoa(int(int32(uint32(n)))), false
		}

		return uaf.IdentifierValue(n | uaf.GeneralBit), "", true
	case cut && kind != "" && strings.HasPrefix("UIC", kind):
		uicText := strings.Replace(rest, ",*]", ",177777]", 1)

		if uic, err := uaf.ParseUIC(uicText); err == nil {
			return uaf.IdentifierValue(uic), "", true
		}

		// The group's identifier has member 177777, past MaxMember.
		g, m, two := strings.Cut(strings.Trim(uicText, "[]"), ",")
		group, err := strconv.ParseUint(strings.TrimSpace(g), 8, 32)

		if !two || err != nil || strings.TrimSpace(m) != "177777" || group > uaf.MaxGroup {
			return 0, rest, false
		}

		return uaf.IdentifierValue(uaf.NewUIC(uint32(group), 0o177777)), "", true
	}

	return 0, text, false
}

// parseNumber reads a number in decimal or with DCL's %D, %O, and %X.
func parseNumber(text string) (uint64, error) {
	t, base := strings.ToUpper(strings.TrimSpace(text)), 10

	switch {
	case strings.HasPrefix(t, "%D"):
		t = t[2:]
	case strings.HasPrefix(t, "%O"):
		t, base = t[2:], 8
	case strings.HasPrefix(t, "%X"):
		t, base = t[2:], 16
	}

	return strconv.ParseUint(t, base, 32)
}

// attributeEdit reads /ATTRIBUTES's names into the bits they set and
// clear (NO in front clearing one); an unknown name is CLI's IVKEYW.
func (u *Utility) attributeEdit(r *dcl.Result) (isSet, isClear uaf.Attributes, ok bool) {
	if !r.Present("ATTRIBUTES") {
		return 0, 0, true
	}

	s, c, ok := u.keywordBits(r.List("ATTRIBUTES"), func(name string) (uint64, bool) {
		m, ok := uaf.AttributeMask(name)

		return uint64(m), ok
	})

	return uaf.Attributes(s), uaf.Attributes(c), ok
}

// identifierName reads an identifier's name from a parameter, upper case.
func identifierName(text string) string { return strings.ToUpper(strings.TrimSpace(text)) }

// environmental reports whether name or v is an environmental
// identifier's.
func environmental(name string, v uaf.IdentifierValue) bool {
	if _, ok := uaf.EnvironmentalNamed(name, 0); ok && name != "" {
		return true
	}

	_, ok := uaf.EnvironmentalNamed("", v)

	return ok && v != 0
}

// addIdentifier is ADD/IDENTIFIER name [/VALUE=...] [/ATTRIBUTES=...]: a
// new identifier, the next general value unless /VALUE gives one
// (RDBADDMSG for a general one, RDBADDMSGU for a UIC). A name in use is
// RDBADDERRU with DUPLNAM, showing the value as [000000,000000] when none
// was given (case 78); a name that can't be one IVIDENT (case 81); a value
// in use DUPIDENT (unconfirmed).
func (u *Utility) addIdentifier(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	name := identifierName(r.String("ID_NAME"))

	var (
		value    uaf.IdentifierValue
		hasValue bool
	)

	if r.Present("VALUE") {
		v, ok := u.parseValue(r.String("VALUE"))
		if !ok {
			return nil
		}

		value, hasValue = v, true
	}

	set, _, ok := u.attributeEdit(r)
	if !ok {
		return nil
	}

	u.change(func(f *uaf.File, say *sayings) error {
		fail := func(code uint32) error {
			say.message("RDBADDERRU", "", name, identifierValue(value))
			say.cause(code)
			u.status = inhibited(code)

			return errNoChange
		}

		if !uaf.ValidIdentifierName(name) {
			return fail(ssIVIDENT)
		}

		if _, taken := f.IdentifierNamed(name); taken || environmental(name, 0) {
			return fail(ssDUPLNAM)
		}

		if !hasValue {
			value = f.NextGeneral()
		} else if _, taken := f.IdentifierWithValue(value); taken || environmental("", value) {
			return fail(ssDUPIDENT)
		}

		f.Identifiers = append(f.Identifiers, uaf.Identifier{Name: name, Value: value, Attributes: set})

		if value&uaf.GeneralBit != 0 {
			say.message("RDBADDMSG", "", name, identifierValue(value))
		} else {
			say.message("RDBADDMSGU", "", name, identifierValue(value))
		}

		u.changedRights = true

		return nil
	})

	return nil
}

// removeIdentifier is REMOVE/IDENTIFIER name: the identifier and its
// holder records, and, a UIC identifier's, every holder record of its UIC
// ($REM_IDENT's rule). Not found is RDBREMERR with NOSUCHID (case 112).
func (u *Utility) removeIdentifier(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	name := identifierName(r.String("ID_NAME"))

	u.change(func(f *uaf.File, say *sayings) error {
		fail := func(code uint32) error {
			say.message("RDBREMERR", "", name)
			say.cause(code)
			u.status = inhibited(code)

			return errNoChange
		}

		if environmental(name, 0) {
			return fail(ssNOPRIV)
		}

		id, ok := f.IdentifierNamed(name)
		if !ok {
			return fail(ssNOSUCHID)
		}

		gone := *id
		f.Identifiers = slices.DeleteFunc(f.Identifiers, func(i uaf.Identifier) bool { return i.Name == gone.Name })

		if gone.Value&uaf.GeneralBit != 0 {
			say.message("RDBREMMSG", "", gone.Name, identifierValue(gone.Value))
		} else {
			f.DropHolder(uaf.UIC(gone.Value))
			say.message("RDBREMMSGU", "", gone.Name, identifierValue(gone.Value))
		}

		u.changedRights = true

		return nil
	})

	return nil
}

// renameIdentifier is RENAME/IDENTIFIER old new: RDBMDFYMSG naming the
// old name (unconfirmed: no probe case succeeded); old not found is
// RDBMDFYERR with NOSUCHID (case 102), and new in use DUPLNAM.
func (u *Utility) renameIdentifier(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	oldName, newName := identifierName(r.String("OLD")), identifierName(r.String("NEW"))

	u.change(func(f *uaf.File, say *sayings) error {
		return u.modifyIdentifier(f, say, oldName, func(id *uaf.Identifier) uint32 {
			if !uaf.ValidIdentifierName(newName) {
				return ssIVIDENT
			}

			if have, taken := f.IdentifierNamed(newName); taken && have.Name != id.Name || environmental(newName, 0) {
				return ssDUPLNAM
			}

			id.Name = newName

			return 0
		})
	})

	return nil
}

// modifyIdentifierCommand is MODIFY/IDENTIFIER name [/VALUE=...]
// [/ATTRIBUTES=...] [/NAME=new]. A new value of a UIC identifier moves its
// UIC's holder records ($MOD_IDENT's rule). Not found is RDBMDFYERR with
// NOSUCHID, found before /VALUE is read (cases 105 to 107); a bad value
// is then BADVALUE (unconfirmed).
func (u *Utility) modifyIdentifierCommand(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	name := identifierName(r.String("ID_NAME"))
	hasValue := r.Present("VALUE")
	valueText := r.String("VALUE")

	isSet, isClear, ok := u.attributeEdit(r)
	if !ok {
		return nil
	}

	newName := ""
	if r.Present("NAME") {
		newName = identifierName(r.String("NAME"))
	}

	u.change(func(f *uaf.File, say *sayings) error {
		return u.modifyIdentifier(f, say, name, func(id *uaf.Identifier) uint32 {
			var value uaf.IdentifierValue

			if hasValue {
				v, shown, ok := valueOf(valueText)
				if !ok {
					say.message("BADVALUE", shown)

					u.status = stsInhibitMsg

					return badValue
				}

				value = v
			}

			if newName != "" {
				if !uaf.ValidIdentifierName(newName) {
					return ssIVIDENT
				}

				if have, taken := f.IdentifierNamed(newName); taken && have.Name != id.Name || environmental(newName, 0) {
					return ssDUPLNAM
				}
			}

			if hasValue && value != id.Value {
				if _, taken := f.IdentifierWithValue(value); taken || environmental("", value) {
					return ssDUPIDENT
				}

				if id.Value&uaf.GeneralBit == 0 && value&uaf.GeneralBit == 0 {
					f.MoveHolder(uaf.UIC(id.Value), uaf.UIC(value))
				}

				id.Value = value
			}

			if newName != "" {
				id.Name = newName
			}

			id.Attributes = (id.Attributes | isSet) &^ isClear

			return 0
		})
	})

	return nil
}

// modifyIdentifier finds the identifier called name and has edit change
// it, saying RDBMDFYMSG with the name it had, or RDBMDFYERR with the
// reason edit returns.
func (u *Utility) modifyIdentifier(f *uaf.File, say *sayings, name string, edit func(*uaf.Identifier) uint32) error {
	fail := func(code uint32) error {
		say.message("RDBMDFYERR", "", name)
		say.cause(code)
		u.status = inhibited(code)

		return errNoChange
	}

	if environmental(name, 0) {
		return fail(ssNOPRIV)
	}

	id, ok := f.IdentifierNamed(name)
	if !ok {
		return fail(ssNOSUCHID)
	}

	was := id.Name

	switch code := edit(id); code {
	case 0:
	case badValue:
		return errNoChange
	default:
		return fail(code)
	}

	say.message("RDBMDFYMSG", "", was)

	u.changedRights = true

	return nil
}

// badValue is an edit's answer when it has said BADVALUE itself.
const badValue = ^uint32(0)

// holderUIC finds the holder GRANT and REVOKE name: a UIC identifier's
// name, whose value is the UIC. Not one is GRANTUSR with NOSUCHID (case
// 92); for REVOKE, REVOKEERR (unconfirmed).
func holderUIC(f *uaf.File, holder string) (uaf.UIC, bool) {
	id, ok := f.IdentifierNamed(holder)
	if !ok || id.Value&uaf.GeneralBit != 0 {
		return 0, false
	}

	return uaf.UIC(id.Value), true
}

// grant is GRANT/IDENTIFIER name holder [/ATTRIBUTES=...]: a holder
// record of the holder's UIC, with those of the attributes the identifier
// has ($ADD_HOLDER's rule), GRANTMSG. An identifier not found is GRANTERR
// with NOSUCHID (case 93), one the UIC holds already DUPIDENT (case 90).
func (u *Utility) grant(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	name, holder := identifierName(r.String("ID_NAME")), identifierName(r.String("HOLDER"))

	set, _, ok := u.attributeEdit(r)
	if !ok {
		return nil
	}

	u.change(func(f *uaf.File, say *sayings) error {
		fail := func(ident string, code uint32) error {
			if ident == "GRANTUSR" {
				say.message(ident, "", holder, name)
			} else {
				say.message(ident, "", name, holder)
			}

			say.cause(code)
			u.status = inhibited(code)

			return errNoChange
		}

		if environmental(name, 0) {
			return fail("GRANTERR", ssNOPRIV)
		}

		id, ok := f.IdentifierNamed(name)
		if !ok {
			return fail("GRANTERR", ssNOSUCHID)
		}

		uic, ok := holderUIC(f, holder)
		if !ok {
			return fail("GRANTUSR", ssNOSUCHID)
		}

		if slices.ContainsFunc(id.Holders, func(h uaf.Holder) bool { return h.UIC == uic }) {
			return fail("GRANTERR", ssDUPIDENT)
		}

		id.Holders = append(id.Holders, uaf.Holder{UIC: uic, Attributes: set & id.Attributes})

		say.message("GRANTMSG", "", name, holder)

		u.changedRights = true

		return nil
	})

	return nil
}

// revoke is REVOKE/IDENTIFIER name holder: the holder record goes,
// REVOKEMSG. One the UIC doesn't hold is REVOKEERR with NOSUCHID (case
// 100), as is an identifier or holder not found (unconfirmed).
func (u *Utility) revoke(r *dcl.Result) error {
	if !u.mayWrite() {
		return nil
	}

	name, holder := identifierName(r.String("ID_NAME")), identifierName(r.String("HOLDER"))

	u.change(func(f *uaf.File, say *sayings) error {
		fail := func(code uint32) error {
			say.message("REVOKEERR", "", name, holder)
			say.cause(code)
			u.status = inhibited(code)

			return errNoChange
		}

		id, ok := f.IdentifierNamed(name)
		uic, held := holderUIC(f, holder)

		if !ok || !held {
			return fail(ssNOSUCHID)
		}

		i := slices.IndexFunc(id.Holders, func(h uaf.Holder) bool { return h.UIC == uic })
		if i < 0 {
			return fail(ssNOSUCHID)
		}

		id.Holders = slices.Delete(id.Holders, i, i+1)

		say.message("REVOKEMSG", "", name, holder)
		
		u.changedRights = true

		return nil
	})

	return nil
}
