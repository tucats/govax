package security

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// UICs as text (docs/planning/PHASE-93 - file protection.md, Decision 9
// and A.4's "Forms and their errors").
//
// A command that takes a UIC (SET SECURITY/OWNER, CREATE/DIRECTORY
// /OWNER_UIC, BACKUP/BY_OWNER) accepts it in any of these forms:
//
//	[200,1]          group and member, in octal
//	[SMITH]          a UIC identifier's name: its UIC
//	[ACCOUNTS,SMITH] a group's name and a member's name, either may be
//	                 a number
//	SMITH            an identifier's name, without brackets
//
// Angle brackets may stand for square ones. DCL has already checked the
// brackets and that there are at most two elements (the $UIC grammar
// type); what is left is the command's: numbers in range and names that
// exist. VMS 7.1 refused [1,99999], [400000,1], and [1] (a number alone
// in brackets), and a name the rights database doesn't have.

// The largest group and member numbers a UIC may have; the next ones up
// are the wildcards (WildGroup, WildMember).
const (
	MaxGroup  = WildGroup - 1
	MaxMember = WildMember - 1
)

// ErrBadUIC is ParseUIC's error for text that isn't a UIC: a number out
// of range or not octal, a number alone in brackets, or a name that can't
// be an identifier's. The commands report it with SS$_IVIDENT.
var ErrBadUIC = errors.New("security: not a UIC")

// ErrNoSuchIdentifier is ParseUIC's error for a name the rights database
// doesn't have: SS$_NOSUCHID.
type ErrNoSuchIdentifier struct{ Name string }

func (e ErrNoSuchIdentifier) Error() string {
	return fmt.Sprintf("security: no identifier %q", e.Name)
}

// IdentifierLookup returns the value of the identifier called name, as
// the rights database has it, and whether there is one. A nil lookup
// knows no names.
type IdentifierLookup func(name string) (uint32, bool)

// ParseUIC reads a UIC in any of its forms, looking names up with lookup.
func ParseUIC(text string, lookup IdentifierLookup) (UIC, error) {
	return parseUIC(text, lookup, false)
}

// ParseUICPattern is ParseUIC allowing "*" for the group or the member,
// which gives WildGroup or WildMember there: "[200,*]" names every member
// of group 200 (BACKUP/BY_OWNER, an identifier ACE).
func ParseUICPattern(text string, lookup IdentifierLookup) (UIC, error) {
	return parseUIC(text, lookup, true)
}

// parseUIC is ParseUIC, and with wild ParseUICPattern.
func parseUIC(text string, lookup IdentifierLookup, wild bool) (UIC, error) {
	s := strings.ToUpper(strings.Join(strings.Fields(text), ""))

	inner, bracketed := unbracket(s)
	if !bracketed {
		// A bare name: the identifier's value, whatever kind it is.
		return identifierValue(s, lookup)
	}

	groupText, memberText, pair := strings.Cut(inner, ",")
	if !pair {
		// [name] is a UIC identifier's UIC; a number alone isn't a UIC.
		return uicIdentifier(inner, lookup)
	}

	group, err := uicPart(groupText, MaxGroup, WildGroup, wild, lookup, func(u UIC) uint32 { return u.Group() })
	if err != nil {
		return 0, err
	}

	member, err := uicPart(memberText, MaxMember, WildMember, wild, lookup, func(u UIC) uint32 { return u.Member() })
	if err != nil {
		return 0, err
	}

	return NewUIC(group, member), nil
}

// unbracket returns what is between s's brackets, square or angle, and
// whether it had them.
func unbracket(s string) (string, bool) {
	if len(s) >= 2 && (s[0] == '[' && s[len(s)-1] == ']' || s[0] == '<' && s[len(s)-1] == '>') {
		return s[1 : len(s)-1], true
	}

	return s, false
}

// uicPart reads a group or a member: "*" when wild allows it, an octal
// number up to max, or a UIC identifier's name, whose UIC's own group or
// member (field) it stands for: [ACCOUNTS,SMITH] is SMITH's member
// number in ACCOUNTS's group (unconfirmed: the probe never wrote a UIC
// with a group's name).
func uicPart(text string, max, wildValue uint32, wild bool, lookup IdentifierLookup, field func(UIC) uint32) (uint32, error) {
	if text == "*" {
		if wild {
			return wildValue, nil
		}

		return 0, ErrBadUIC
	}

	if allDigits(text) {
		n, err := strconv.ParseUint(text, 8, 32)
		if err != nil || n > uint64(max) {
			return 0, ErrBadUIC
		}

		return uint32(n), nil
	}

	u, err := uicIdentifier(text, lookup)
	if err != nil {
		return 0, err
	}

	return field(u), nil
}

// uicIdentifier is the UIC a UIC identifier's name stands for. A general
// identifier, bit 31 set, isn't one (unconfirmed), and neither is a
// number.
func uicIdentifier(name string, lookup IdentifierLookup) (UIC, error) {
	u, err := identifierValue(name, lookup)
	if err != nil {
		return 0, err
	}

	if IsGeneral(uint32(u)) {
		return 0, ErrBadUIC
	}

	return u, nil
}

// identifierValue looks name up: ErrBadUIC if it can't be an identifier's
// name, ErrNoSuchIdentifier if the database doesn't have it.
func identifierValue(name string, lookup IdentifierLookup) (UIC, error) {
	if !validIdentifierName(name) {
		return 0, ErrBadUIC
	}

	if lookup != nil {
		if v, ok := lookup(name); ok {
			return UIC(v), nil
		}
	}

	return 0, ErrNoSuchIdentifier{Name: name}
}

// allDigits reports whether s is all decimal digits; whether they are
// octal ones is strconv.ParseUint's to say.
func allDigits(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// validIdentifierName reports whether name has an identifier name's form:
// 1 to 31 letters, digits, "$", and "_", not all digits.
func validIdentifierName(name string) bool {
	if name == "" || len(name) > 31 || allDigits(name) {
		return false
	}

	for _, c := range name {
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '$' || c == '_') {
			return false
		}
	}

	return true
}
