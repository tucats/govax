package uaf

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// This file is the names the JSON file uses for numbers VMS keeps as
// bits and longwords: a UIC, privileges, the account's flags, an
// identifier's value and attributes. Each is a Go type with its own JSON
// form, so a Record is plain Go to its users and readable text in the
// file.

// UIC is a user identification code, the group in the high word and the
// member in the low. In the file it is "[group,member]", both numbers in
// octal, as AUTHORIZE writes and reads them ("[300,8]" was UICERR on VMS
// 7.1, the phase doc's A.5).
type UIC uint32

// The largest group and member a UIC may have: VMS 7.1's AUTHORIZE took
// group 37776 and member 177776 (octal), and refused [400000,1]. Group
// 37777 and member 177777 are the wildcards.
const (
	MaxGroup  = 0o37776
	MaxMember = 0o177776
)

// NewUIC makes a UIC from its group and member.
func NewUIC(group, member uint32) UIC { return UIC(group<<16 | member&0xFFFF) }

// Group is the UIC's group number.
func (u UIC) Group() uint32 { return uint32(u) >> 16 }

// Member is the UIC's member number.
func (u UIC) Member() uint32 { return uint32(u) & 0xFFFF }

// String is the UIC as the file writes it, "[1,4]".
func (u UIC) String() string { return fmt.Sprintf("[%o,%o]", u.Group(), u.Member()) }

// ParseUIC reads "[group,member]", octal numbers each in range. Blanks
// around the numbers are allowed.
func ParseUIC(text string) (UIC, error) {
	bad := fmt.Errorf("%q is not a UIC of the form [group,member], in octal", text)

	inner, ok := strings.CutPrefix(strings.TrimSpace(text), "[")
	if !ok {
		return 0, bad
	}

	inner, ok = strings.CutSuffix(inner, "]")
	if !ok {
		return 0, bad
	}

	g, m, ok := strings.Cut(inner, ",")
	if !ok {
		return 0, bad
	}

	group, err1 := strconv.ParseUint(strings.TrimSpace(g), 8, 32)
	member, err2 := strconv.ParseUint(strings.TrimSpace(m), 8, 32)

	if err1 != nil || err2 != nil || group > MaxGroup || member > MaxMember {
		return 0, bad
	}

	return NewUIC(uint32(group), uint32(member)), nil
}

// MarshalJSON writes the UIC as its text.
func (u UIC) MarshalJSON() ([]byte, error) { return json.Marshal(u.String()) }

// UnmarshalJSON reads a UIC's text.
func (u *UIC) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	v, err := ParseUIC(s)
	if err != nil {
		return &valueError{text: s, err: err}
	}

	*u = v

	return nil
}

// Privileges is a privilege mask, bit n the $PRVDEF privilege whose
// PRV$V_ value is n. In the file it is a list of names: ["ALL"] for every
// bit, or the privileges one by one.
type Privileges uint64

// AllPrivileges is every bit: VMS's ALL is all 64, though only the first
// PRV$K_NUMBER_OF_PRIVS have names.
const AllPrivileges = Privileges(^uint64(0))

// privilegeNames are the privileges' names by bit. Where two names share
// a bit (DETACH and IMPERSONATE, ACNT and NOACNT) the file writes
// AUTHORIZE's, IMPERSONATE and ACNT (VMS 7.1's SHOW of SYSTEM, the
// probe's case 2), and reads both.
var privilegeNames = func() []string {
	older := map[string]bool{"DETACH": true, "NOACNT": true}

	var names []string

	for name, bit := range vmsdef.Symbols {
		p, ok := strings.CutPrefix(name, "PRV$V_")
		if !ok {
			continue
		}

		for int(bit) >= len(names) {
			names = append(names, "")
		}

		if names[bit] == "" || older[names[bit]] {
			names[bit] = p
		}
	}

	return names
}()

// PrivilegeBit returns the bit of the privilege called name (any case),
// and whether there is one.
func PrivilegeBit(name string) (int, bool) {
	bit, ok := vmsdef.Symbols["PRV$V_"+strings.ToUpper(name)]

	return int(bit), ok
}

// Names returns the privileges' names in bit order, or just "ALL" when
// every bit is set.
func (p Privileges) Names() []string {
	if p == AllPrivileges {
		return []string{"ALL"}
	}

	names := []string{}

	for bit, name := range privilegeNames {
		if name != "" && p&(1<<bit) != 0 {
			names = append(names, name)
		}
	}

	return names
}

// ParsePrivileges reads a list of privilege names; "ALL" is every bit.
func ParsePrivileges(names []string) (Privileges, error) {
	var p Privileges

	for _, name := range names {
		if strings.EqualFold(name, "ALL") {
			p = AllPrivileges

			continue
		}

		bit, ok := PrivilegeBit(name)
		if !ok {
			return 0, &valueError{text: name, err: fmt.Errorf("no privilege is called %q", name)}
		}

		p |= 1 << bit
	}

	return p, nil
}

// MarshalJSON writes the privileges' names.
func (p Privileges) MarshalJSON() ([]byte, error) { return json.Marshal(p.Names()) }

// UnmarshalJSON reads a list of privilege names.
func (p *Privileges) UnmarshalJSON(data []byte) error {
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return err
	}

	v, err := ParsePrivileges(names)
	*p = v

	return err
}

// Flags are an account's login flags, AUTHORIZE's /FLAGS, $GETUAI's
// UAI$_FLAGS bits.
type Flags uint32

// FlagNames are the flags by bit, as $GETUAI's UAI$_FLAGS numbers them
// (the System Services Reference Manual's $GETUAI entry). SHOW lists the
// set ones in this order (VMS 7.1's "DisCtlY DefCLI LockPwd DisUser
// DisWelcome DisNewMail Captive", the probe's case 26). Only DISUSER
// does anything in govax yet (it refuses a login); the rest are kept and
// shown. PWD_EXPIRED and PWD2_EXPIRED are AUTHORIZE's flags too, though
// govax has no password expiration.
var FlagNames = []string{
	"DISCTLY", "DEFCLI", "LOCKPWD", "RESTRICTED", "DISUSER", "DISWELCOME",
	"DISNEWMAIL", "DISMAIL", "GENPWD", "PWD_EXPIRED", "PWD2_EXPIRED", "AUDIT",
	"DISREPORT", "DISRECONNECT", "AUTOLOGIN", "DISFORCE_PWD_CHANGE", "CAPTIVE",
	"DISIMAGE", "DISPWDDIC", "DISPWDHIS",
}

// The flags govax looks at.
var (
	FlagDISUSER = flagNamed("DISUSER")
)

// flagNamed returns the flag called name, which must be in FlagNames.
func flagNamed(name string) Flags {
	bit := slices.Index(FlagNames, name)
	if bit < 0 {
		panic("uaf: no flag " + name)
	}

	return 1 << bit
}

// FlagBit returns the bit of the flag called name (any case), and
// whether there is one.
func FlagBit(name string) (int, bool) {
	bit := slices.Index(FlagNames, strings.ToUpper(name))

	return bit, bit >= 0
}

// Names returns the set flags' names in bit order.
func (f Flags) Names() []string {
	names := []string{}

	for bit, name := range FlagNames {
		if f&(1<<bit) != 0 {
			names = append(names, name)
		}
	}

	return names
}

// MarshalJSON writes the flags' names.
func (f Flags) MarshalJSON() ([]byte, error) { return json.Marshal(f.Names()) }

// UnmarshalJSON reads a list of flag names.
func (f *Flags) UnmarshalJSON(data []byte) error {
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return err
	}

	*f = 0

	for _, name := range names {
		bit, ok := FlagBit(name)
		if !ok {
			return &valueError{text: name, err: fmt.Errorf("no flag is called %q", name)}
		}

		*f |= 1 << bit
	}

	return nil
}

// IdentifierValue is an identifier's 32-bit value: a general identifier
// has bit 31 set ("%X80010001"); a UIC identifier is a UIC, bit 31 clear.
// In the file it is "%X" and eight hexadecimal digits.
type IdentifierValue uint32

// GeneralBit marks a general identifier.
const GeneralBit = 0x80000000

// FirstGeneral is the value AUTHORIZE's ADD/IDENTIFIER gives the first
// identifier it makes, and the next ones count up from it (the probe's
// case 76 gave PROJX %X80010000, the next %X80010001).
const FirstGeneral = 0x80010000

// String is the value as the file writes it.
func (v IdentifierValue) String() string { return fmt.Sprintf("%%X%08X", uint32(v)) }

// ParseIdentifierValue reads "%Xhhhhhhhh" (any case, up to 8 digits).
func ParseIdentifierValue(text string) (IdentifierValue, error) {
	t := strings.TrimSpace(text)
	if len(t) < 3 || !strings.EqualFold(t[:2], "%X") {
		return 0, fmt.Errorf("%q is not an identifier value of the form %%X80010001", text)
	}

	n, err := strconv.ParseUint(t[2:], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not an identifier value of the form %%X80010001", text)
	}

	return IdentifierValue(n), nil
}

// MarshalJSON writes the value's text.
func (v IdentifierValue) MarshalJSON() ([]byte, error) { return json.Marshal(v.String()) }

// UnmarshalJSON reads a value's text.
func (v *IdentifierValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	n, err := ParseIdentifierValue(s)
	if err != nil {
		return &valueError{text: s, err: err}
	}

	*v = n

	return nil
}

// Attributes are an identifier's attributes, or a holder's: the KGB$M_
// bits (RESOURCE, DYNAMIC, NOACCESS, SUBSYSTEM, IMPERSONATE,
// HOLDER_HIDDEN, NAME_HIDDEN).
type Attributes uint32

// attributeNames are the attributes $KGBDEF names, by mask.
var attributeNames = func() map[uint32]string {
	names := map[uint32]string{}

	for name, v := range vmsdef.Symbols {
		if a, ok := strings.CutPrefix(name, "KGB$M_"); ok {
			names[v] = a
		}
	}

	return names
}()

// AttributeMask returns the mask of the attribute called name (any case),
// and whether there is one.
func AttributeMask(name string) (Attributes, bool) {
	v, ok := vmsdef.Symbols["KGB$M_"+strings.ToUpper(name)]

	return Attributes(v), ok
}

// Names returns the set attributes' names, lowest bit first.
func (a Attributes) Names() []string {
	names := []string{}

	for bit := range 32 {
		if name, ok := attributeNames[1<<bit]; ok && a&(1<<bit) != 0 {
			names = append(names, name)
		}
	}

	return names
}

// MarshalJSON writes the attributes' names.
func (a Attributes) MarshalJSON() ([]byte, error) { return json.Marshal(a.Names()) }

// UnmarshalJSON reads a list of attribute names.
func (a *Attributes) UnmarshalJSON(data []byte) error {
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return err
	}

	*a = 0

	for _, name := range names {
		m, ok := AttributeMask(name)
		if !ok {
			return &valueError{text: name, err: fmt.Errorf("no identifier attribute is called %q", name)}
		}

		*a |= m
	}

	return nil
}

// valueError is a value in the file a type couldn't read: its text, so
// Load can find the line it's on, and why.
type valueError struct {
	text string
	err  error
}

func (e *valueError) Error() string { return e.err.Error() }

func (e *valueError) Unwrap() error { return e.err }
